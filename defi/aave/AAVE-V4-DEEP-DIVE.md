# Aave v4 Deep Dive (Hub & Spoke)

> Source read for this document: `aave/v4-aave` (repo `aave/aave-v4`), Solidity `0.8.28`.
> Everything below is derived from that source tree. Where the code did not let me
> settle a question, the text says so explicitly rather than guessing.
>
> Companion documents in this folder:
> - `aave/AAVE-DEEP-DIVE.md` — Aave v3.6 (`Pool`, logic libraries, aTokens, liquidation, rate strategy).
> - `aave/AAVE-V1-V2-DEEP-DIVE.md` — Aave v1 and v2, and the road to v3.
>
> This document assumes you have read at least the v3 one, because section 7 is a
> diff against it rather than a re-explanation.

---

## 0. The big idea in plain English

### 0.1 The problem v4 is solving

In Aave v3 (see `aave/AAVE-DEEP-DIVE.md`), a "market" is one `Pool` contract holding
every reserve. If the DAO wants a different risk regime — an RWA market, a market with
a different oracle, a market with different liquidation rules — it must deploy a whole
new `Pool` with its own liquidity. USDC supplied to the "Core" market cannot be borrowed
by someone in the "RWA" market. Liquidity is fragmented per market, and a new market
starts at zero depth. Worse, changing a risk parameter in v3 (say lowering a liquidation
threshold) hits **every open position instantly**, which can liquidate users who did
nothing wrong.

### 0.2 The v4 answer

Split the system in two layers:

- **Hub** — owns liquidity and interest accounting for an asset. It is dumb on purpose:
  it knows assets, shares, an index, caps, and which Spokes are allowed to touch it. It
  has **no idea what a user is**. It is immutable (`src/hub/instances/HubInstance.sol:9`).
- **Spoke** — owns users, collateral, oracles, health factors, and liquidation rules.
  It is upgradeable. Users only ever talk to Spokes; Spokes talk to Hubs.

One Hub can serve many Spokes. Adding a new product (a new market) means deploying a
Spoke and registering it on the existing Hub. **No liquidity migration.** The new Spoke
immediately draws from the same deep USDC pool as everything else. Removing a product
means setting its caps to zero and deactivating it; the liquidity never moved.

```
   Suppliers                Borrowers            Suppliers
       |                        |                    |
       v                        v                    v
  +----------+           +-------------+       +-----------+
  | Spoke A  |           |   Spoke B   |       |  Spoke C  |     <- users, collateral,
  | (core)   |           |  (RWA mkt)  |       | (ERC4626) |        oracle, HF, liquidation
  +----------+           +-------------+       +-----------+        UPGRADEABLE
       |  add/remove           | draw/restore        | add/remove
       |  draw/restore         |                     |
       +-----------+-----------+---------------------+
                   |
                   v
            +--------------------------------+
            |            HUB                 |   <- holds ALL the ERC20 tokens,
            |  assetId 0: USDC               |      shares, drawnIndex, caps,
            |  assetId 1: WETH               |      deficit, fees.  IMMUTABLE.
            |  assetId 2: ...                |      Knows spokes, not users.
            +--------------------------------+
                   |
                   v  sweep/reclaim (optional, governance-controlled)
            +--------------------------+
            | Reinvestment Controller  |
            +--------------------------+
```

Note the token custody: **the Hub holds the underlying ERC20s**. In `Spoke.supply` the
transfer goes straight from the user to the Hub address, never resting in the Spoke:

```solidity
// src/spoke/Spoke.sol:234
IERC20(reserve.underlying).safeTransferFrom(msg.sender, address(reserve.hub), amount);
uint256 suppliedShares = reserve.hub.add(reserve.assetId, amount);
```

### 0.3 Risk premiums: the second big idea

In v3 every borrower of USDC pays the same rate, whether they posted ETH or a thin
long-tail token as collateral. v4 prices that difference.

- Every reserve gets a **Collateral Risk** `CR` in BPS, 0 (pristine) to `1000_00`
  (`MAX_ALLOWED_COLLATERAL_RISK`, `src/spoke/Spoke.sol:70`). Note this maximum is
  1000.00%, not 100% — the premium can be a multiple of the base rate.
- A borrower's **User Risk Premium** `RP_u` is the collateral-value-weighted average of
  `CR` over just enough of their collateral (cheapest-risk first) to cover their debt.
- Their debt then accrues at the base drawn rate **plus** `base × RP_u`.

So an ETH-backed borrow is cheaper than an exotic-backed borrow of the same asset, out
of the same liquidity pool. The mechanism is "premium shares", explained in §2.4.

### 0.4 The vocabulary shift

The Hub deliberately does not say "supply" or "borrow", because it does not know about
users. Learn this mapping, everything else follows:

| User action (Spoke) | Hub call | Meaning at the Hub |
|---|---|---|
| `supply` | `add` | liquidity came in, mint added-shares to the calling Spoke |
| `withdraw` | `remove` | liquidity goes out, burn added-shares |
| `borrow` | `draw` | liquidity goes out as debt, mint drawn-shares |
| `repay` | `restore` | liquidity comes back, burn drawn-shares (+ premium) |
| liquidation w/ bad debt | `reportDeficit` | debt written off, recorded as deficit |
| — | `refreshPremium` | recalibrate premium bookkeeping, no value change |

---

## 1. Contract map & roles

### 1.1 Who calls whom

```
 user / EOA
   |
   |  (direct)                        (delegated)
   |                                      |
   v                                      v
 Spoke  <---------------------  PositionManager (Taker/Giver/Config/SignatureGateway/
   |     onlyPositionManager                     NativeTokenGateway)
   |
   |  add / remove / draw / restore / reportDeficit / refreshPremium / payFeeShares
   v
  Hub  ------> AssetInterestRateStrategy.calculateInterestRate()   (per Hub, per assetId)
   |
   +---------> ERC20 underlying (safeTransfer / balanceOf)
   |
   +---------> Reinvestment Controller (sweep / reclaim)   [optional, governance]

 Spoke also calls:
   AaveOracle.getReservePrice(reserveId)         (oracle is SPOKE-specific)
   LiquidationLogic (external library, delegatecall-linked)

 Governance path:
   AccessManagerEnumerable (OZ AccessManager)
        ^ authority for both Hub and Spoke (`restricted` modifier)
        |
   HubConfigurator / SpokeConfigurator  <---- AaveV4ConfigEngine <---- AaveV4Payload
```

Key files:

| Concern | Contract | Path |
|---|---|---|
| Liquidity + accounting | `Hub` (abstract), `HubInstance` | `src/hub/Hub.sol`, `src/hub/instances/HubInstance.sol` |
| Hub storage | `HubStorage` | `src/hub/HubStorage.sol` |
| Index/share math | `AssetLogic`, `SharesMath`, `Premium` | `src/hub/libraries/` |
| Rates | `AssetInterestRateStrategy` | `src/hub/AssetInterestRateStrategy.sol` |
| Users, collateral, HF | `Spoke` (abstract), `SpokeInstance` | `src/spoke/Spoke.sol` |
| Spoke storage | `SpokeStorage` | `src/spoke/SpokeStorage.sol` |
| Liquidations | `LiquidationLogic` | `src/spoke/libraries/LiquidationLogic.sol` |
| Position bitmap | `PositionStatusMap` | `src/spoke/libraries/PositionStatusMap.sol` |
| Premium per user | `UserPositionUtils` | `src/spoke/libraries/UserPositionUtils.sol` |
| ERC4626 wrapper | `TokenizationSpoke` | `src/spoke/TokenizationSpoke.sol` |
| Fee sink | `TreasurySpoke` | `src/spoke/TreasurySpoke.sol` |
| Oracle | `AaveOracle` | `src/spoke/AaveOracle.sol` |
| Access control | `AccessManagerEnumerable` | `src/access/AccessManagerEnumerable.sol` |
| Governance UX | `AaveV4ConfigEngine`, `AaveV4Payload` | `src/config-engine/` |
| Delegation | `PositionManagerBase` + 5 managers | `src/position-manager/` |

### 1.2 Hub storage

`src/hub/HubStorage.sol:11-28`:

```solidity
uint256 internal _assetCount;
mapping(uint256 assetId => IHub.Asset) internal _assets;
mapping(uint256 assetId => mapping(address spoke => IHub.SpokeData)) internal _spokes;
mapping(uint256 assetId => EnumerableSet.AddressSet) internal _assetToSpokes;
mapping(address underlying => uint256 assetId) internal _underlyingToAssetId;
uint256[50] private __gap;
```

Assets are addressed by a **numeric `assetId`**, not by token address. `getAssetId(underlying)`
maps back (`src/hub/Hub.sol:496`).

The `Asset` struct (`src/hub/interfaces/IHub.sol:29-56`) is slot-packed; the `//` comments
in the source mark slot boundaries:

```solidity
struct Asset {
  uint120 liquidity;        // idle underlying actually sitting in the Hub
  uint120 realizedFees;     // accrued protocol fees not yet minted as shares
  uint8   decimals;
  //
  uint120 addedShares;      // total supply-side shares across all spokes
  uint120 swept;            // liquidity sent to the reinvestment controller
  //
  int200  premiumOffsetRay; // signed! see §2.4
  //
  uint120 drawnShares;      // total debt shares
  uint120 premiumShares;    // total virtual premium shares
  uint16  liquidityFee;     // protocol cut of interest, BPS
  //
  uint120 drawnIndex;       // RAY, monotonically increasing
  uint96  drawnRate;        // RAY per year
  uint40  lastUpdateTimestamp;
  //
  address underlying;  address irStrategy;  address reinvestmentController;  address feeReceiver;
  //
  uint200 deficitRay;       // bad debt, in asset units * RAY
}
```

`SpokeData` (`src/hub/interfaces/IHub.sol:77-91`) mirrors the per-spoke slice plus its config:

```solidity
struct SpokeData {
  uint120 drawnShares; uint120 premiumShares;
  int200  premiumOffsetRay;
  uint120 addedShares;
  uint40  addCap;                 // in WHOLE assets, not scaled by decimals
  uint40  drawCap;                // ditto
  uint24  riskPremiumThreshold;   // max premiumShares/drawnShares ratio, BPS
  bool    active;                 // false = spoke can do nothing
  bool    halted;                 // true = no actions that move liquidity now
  uint200 deficitRay;
}
```

`active` vs `halted` matters: `reportDeficit` and `eliminateDeficit` only require `active`
and deliberately tolerate `halted` (`src/hub/Hub.sol:875-895`), so a halted Spoke can still
be cleaned up in an emergency. Every other action requires `!halted`.

Caps are stored in **whole tokens** (`uint40`), and expanded at check time:
`addCap * 10**decimals` (`src/hub/Hub.sol:825-829`). `MAX_ALLOWED_SPOKE_CAP = type(uint40).max`
means "no cap" (`src/hub/Hub.sol:38`).

### 1.3 Spoke storage

`src/spoke/SpokeStorage.sol:10-39`:

```solidity
uint256 internal _reserveCount;
ISpoke.LiquidationConfig internal _liquidationConfig;
mapping(uint256 reserveId => ISpoke.Reserve) internal _reserves;
mapping(address hub => mapping(uint256 assetId => uint256 reserveId)) internal _hubAssetIdToReserveId;
mapping(uint256 reserveId => mapping(uint32 dynamicConfigKey => ISpoke.DynamicReserveConfig)) internal _dynamicConfig;
mapping(address user => ISpoke.PositionStatus) internal _positionStatus;
mapping(address user => mapping(uint256 reserveId => ISpoke.UserPosition)) internal _userPositions;
mapping(address positionManager => ISpoke.PositionManagerConfig) internal _positionManager;
uint256[50] private __gap;
```

**`reserveId` vs `assetId`.** A `reserveId` is Spoke-local; an `assetId` is Hub-local. The
`Reserve` struct (`src/spoke/interfaces/ISpoke.sol:44-53`) carries `hub` **per reserve**, so
one Spoke can source different reserves from *different Hubs*. That is unusual and worth
internalising: the Spoke is not bound to a single Hub.

`UserPosition` (`src/spoke/interfaces/ISpoke.sol:95-103`) is the whole per-user state:

```solidity
struct UserPosition {
  uint120 drawnShares; uint120 premiumShares;
  int200  premiumOffsetRay;
  uint120 suppliedShares;
  uint32  dynamicConfigKey;   // snapshot of the risk config this position is bound to
}
```

Notice: **no aToken, no debt token.** A supply position is a `uint120` in a mapping. This is
the single biggest structural change from v3 and it is covered in §7.

`PositionStatus` holds the bitmap plus the cached risk premium
(`src/spoke/interfaces/ISpoke.sol:116-121`):

```solidity
struct PositionStatus { mapping(uint256 bucket => uint256) map; uint24 riskPremium; }
```

`PositionStatusMap` packs **2 bits per reserve** — bit 0 borrowing, bit 1 collateral — 128
reserves per word (`src/spoke/libraries/PositionStatusMap.sol:22-51`). Same idea as v3's
`UserConfiguration`, but bucketed so the reserve count is unbounded. Iteration helpers
`next`, `nextBorrowing`, `nextCollateral` walk set bits using `LibBit`, returning
`NOT_FOUND = type(uint256).max` when done.

`MAX_USER_RESERVES_LIMIT` is an **immutable** set in the constructor
(`src/spoke/Spoke.sol:59, 96-102`). It caps collateral reserves and borrow reserves
*separately*; it is enforced on `borrow` (`src/spoke/Spoke.sol:288-294`) and on enabling
collateral.

Reserve flags are packed into a `ReserveFlags` user-defined type via `ReserveFlagsMap`:
`paused`, `frozen`, `borrowable`, `receiveSharesEnabled` (`src/spoke/interfaces/ISpoke.sol:61-67`).

---

## 2. Accounting model

### 2.1 Two independent share systems

The Hub runs **two separate** conversions, and confusing them is the main way to misread
this codebase.

**Debt side — an index, like v3.** `drawnIndex` starts at `RAY`
(`src/hub/Hub.sol:76`) and only grows. Conversions are plain ray math
(`src/hub/libraries/AssetLogic.sol:25-54`):

```
drawnAssets = drawnShares * drawnIndex / RAY
drawnShares = drawnAssets * RAY / drawnIndex
```

**Supply side — ERC4626-style shares, not an index.** `toAddedShares*` /
`toAddedAssets*` (`src/hub/libraries/AssetLogic.sol:99-128`) route through `SharesMath`
against `totalAddedAssets()`:

```solidity
// src/hub/libraries/SharesMath.sol:13-14, 17-28
uint256 internal constant VIRTUAL_ASSETS = 1e6;
uint256 internal constant VIRTUAL_SHARES = 1e6;

function toSharesDown(uint256 assets, uint256 totalAssets, uint256 totalShares) internal pure returns (uint256) {
  return assets.mulDiv(totalShares + VIRTUAL_SHARES, totalAssets + VIRTUAL_ASSETS, Math.Rounding.Floor);
}
```

The virtual offset of 1e6 is the standard OZ-style defence against the inflation /
donation attack on an empty vault: the first depositor cannot make one share worth an
arbitrary amount, because the denominator never starts at zero. This is v4's structural
answer to the same problem v3 solved with `virtualUnderlyingBalance`.

And `totalAddedAssets` is where supplier yield actually comes from
(`src/hub/libraries/AssetLogic.sol:79-96`):

```solidity
uint256 aggregatedOwedRay = _calculateAggregatedOwedRay({
  drawnShares: asset.drawnShares, premiumShares: asset.premiumShares,
  premiumOffsetRay: asset.premiumOffsetRay, deficitRay: asset.deficitRay,
  drawnIndex: drawnIndex
});
return asset.liquidity + asset.swept + aggregatedOwedRay.fromRayUp()
       - asset.realizedFees - asset.getUnrealizedFees(drawnIndex);
```

Read that as: **total supplier claim = idle + reinvested + everything owed − protocol fees.**
Because `aggregatedOwedRay` includes `deficitRay` (`src/hub/libraries/AssetLogic.sol:229-242`),
bad debt is *not* immediately subtracted from suppliers. It sits as a recorded hole that
someone must fill; see §4.5.

As debt accrues, `aggregatedOwedRay` rises, `totalAddedAssets` rises, and every added-share
becomes worth more. That is the yield. There is no `liquidityIndex`.

### 2.2 Accrual

```solidity
// src/hub/libraries/AssetLogic.sol:141-150
function accrue(IHub.Asset storage asset) internal {
  if (asset.lastUpdateTimestamp == block.timestamp) return;
  uint256 drawnIndex = asset.getDrawnIndex();
  asset.realizedFees += asset.getUnrealizedFees(drawnIndex).toUint120();
  asset.drawnIndex = drawnIndex.toUint120();
  asset.lastUpdateTimestamp = block.timestamp.toUint40();
}
```

and

```solidity
// src/hub/libraries/AssetLogic.sol:153-165
function getDrawnIndex(IHub.Asset storage asset) internal view returns (uint256) {
  uint256 previousIndex = asset.drawnIndex;
  uint40 lastUpdateTimestamp = asset.lastUpdateTimestamp;
  if (lastUpdateTimestamp == block.timestamp || (asset.drawnShares == 0 && asset.premiumShares == 0))
    return previousIndex;
  return previousIndex.rayMulUp(MathUtils.calculateLinearInterest(asset.drawnRate, lastUpdateTimestamp));
}
```

**A real difference from v3 worth pausing on.** v3 used `calculateCompoundedInterest`, a
third-order binomial approximation of continuous compounding, for the borrow index. v4 has
**no such function at all** — I grepped `src/` for `calculateCompoundedInterest` and it does
not exist. v4 applies *simple* interest over the elapsed interval and multiplies it into the
index. Because `accrue()` runs on every single Hub interaction, the index still compounds,
but **per interaction rather than continuously**. Consequence: a totally idle asset accrues
slightly less than v3 would have; a busy asset accrues slightly more than simple interest.

Every state-changing Hub function starts with `asset.accrue()` and ends with
`asset.updateDrawnRate(assetId)` — accrue at the *old* rate, then reprice. That ordering is
correct and is worth checking in each function below.

`updateDrawnRate` emits `UpdateAsset` on every call (`src/hub/libraries/AssetLogic.sol:132-138`),
so the event stream is chatty but complete.

### 2.3 The protocol fee

```solidity
// src/hub/libraries/AssetLogic.sol:187-226 (abridged)
uint256 aggregatedOwedRayAfter  = _calculateAggregatedOwedRay(..., drawnIndex);
uint256 aggregatedOwedRayBefore = _calculateAggregatedOwedRay(..., previousIndex);
return (aggregatedOwedRayAfter.fromRayUp() - aggregatedOwedRayBefore.fromRayUp())
         .percentMulDown(liquidityFee);
```

The fee is `liquidityFee` BPS of the *growth in total owed* (drawn + premium + deficit)
between the two indexes. It accumulates in `realizedFees`, which is excluded from
`totalAddedAssets`, so it is invisible to suppliers from the moment it accrues.

`mintFeeShares` then converts that balance into real added-shares for the fee receiver
Spoke (`src/hub/Hub.sol:765-783`):

```solidity
uint256 fees = asset.realizedFees;
uint120 shares = asset.toAddedSharesDown(fees).toUint120();
if (shares == 0) return 0;                       // no-op below one share
require(feeReceiverSpoke.active, SpokeNotActive());
asset.addedShares += shares; feeReceiverSpoke.addedShares += shares; asset.realizedFees = 0;
```

The fee receiver is itself a Spoke — normally a `TreasurySpoke`
(`src/spoke/TreasurySpoke.sol:15`) — registered automatically at `addAsset` time with
max add cap and **zero draw cap** (`src/hub/Hub.sol:688-701`). The treasury can hold and
withdraw but can never borrow.

### 2.4 Premium shares — the trick to understand

The problem: a user's extra risk-based interest must accrue continuously, but the Hub only
has one index. Solution: give the user **extra virtual debt shares** whose growth *is* the
premium, and subtract a fixed baseline so they do not add principal.

```solidity
// src/hub/libraries/Premium.sol:17-23
function calculatePremiumRay(uint256 premiumShares, int256 premiumOffsetRay, uint256 drawnIndex)
  internal pure returns (uint256) {
  return ((premiumShares * drawnIndex).toInt256() - premiumOffsetRay).toUint256();
}
```

`premiumDebt = premiumShares × drawnIndex − premiumOffset`. At the instant premium is
(re)set, the offset is chosen so this equals the *already accrued* premium, i.e. the
premium shares contribute **zero principal** at t=0 and only their subsequent growth is
new premium.

Setting them (`src/spoke/libraries/UserPositionUtils.sol:54-81`):

```solidity
uint256 premiumDebtRay = Premium.calculatePremiumRay({premiumShares: oldPremiumShares, ...});
uint256 newPremiumShares = (userPosition.drawnShares - drawnSharesTaken).percentMulUp(riskPremium);
int256  newPremiumOffsetRay = (newPremiumShares * drawnIndex).signedSub(premiumDebtRay - restoredPremiumRay);
return IHubBase.PremiumDelta({
  sharesDelta:   newPremiumShares.signedSub(oldPremiumShares),
  offsetRayDelta: newPremiumOffsetRay - oldPremiumOffsetRay,
  restoredPremiumRay: restoredPremiumRay
});
```

So **`premiumShares = drawnShares × RP_u`** (BPS). If `RP_u = 20_00` (20%), you carry 20%
extra virtual shares, and your total debt grows at `base × 1.20`. That is exactly the doc's
`ΔD_premium = R_base × RP_u × D_base`. Note `offsetRayDelta` is a *signed* delta, which is
why `premiumOffsetRay` is `int200` everywhere.

The Hub does not trust the Spoke's arithmetic. `_validateApplyPremiumDelta`
(`src/hub/Hub.sol:933-960`) recomputes the premium before and after and enforces
conservation of value:

```solidity
require(premiumRayAfter + premiumDelta.restoredPremiumRay == premiumRayBefore, InvalidPremiumChange());
```

In words: *you may reshape premium shares and offset however you like, but the premium debt
you remove must exactly equal the premium you claim to have repaid.* This runs for both the
asset aggregate and the spoke aggregate (`src/hub/Hub.sol:734-762`), and is followed by the
threshold cap:

```solidity
// src/hub/Hub.sol:756-761
require(riskPremiumThreshold == MAX_RISK_PREMIUM_THRESHOLD ||
        spoke.premiumShares <= spoke.drawnShares.percentMulUp(riskPremiumThreshold),
        InvalidPremiumChange());
```

That is the Hub's defence against a compromised or buggy Spoke inflating premium: a Spoke
can never report more than `riskPremiumThreshold` BPS of premium relative to its drawn
shares. This is the single most important trust boundary in v4.

### 2.5 The four Hub invariants and where they live

The overview doc (`docs/overview.md`) lists four. Mapping them to code:

| Invariant | Enforcement |
|---|---|
| 1. Total drawn shares == Σ spoke drawn shares | Structural: every write touches both, adjacent lines — e.g. `src/hub/Hub.sol:259-261` (`draw`), `:283-285` (`restore`) |
| 2. Hub added assets ≥ Σ spoke added assets | Rounding discipline: shares out round **up** (`toAddedSharesUp` on `remove`, `src/hub/Hub.sol:233`), shares in round **down** (`toAddedSharesDown` on `add`, `:208`) |
| 3. Hub added shares == Σ spoke added shares | Structural, same pattern (`:209-210`); transfers move both sides in `_transferShares` (`:721-729`) |
| 4. Share price and drawn index never decrease | `drawnIndex` only ever multiplied by `≥ RAY` (`AssetLogic.sol:161-164`); share price protected by rounding + `VIRTUAL_*` |

Invariant 4 is the one users depend on and the one Certora formally verified (see the
`audits/` PDFs listed in §8).

I did not find a single explicit `require` that asserts these as global statements — they are
maintained structurally rather than checked. That is normal for gas reasons, and it is why the
formal-verification reports exist.

### 2.6 Interest rate strategy

`src/hub/AssetInterestRateStrategy.sol:102-137`. A classic two-slope kinked curve, per
`assetId`, and the strategy is Hub-bound (`HUB` immutable, `setInterestRateData` is
`require(HUB == msg.sender)`, `:43`).

```solidity
uint256 usageRatioRay = drawn.rayDivUp(liquidity + drawn + swept);
if (usageRatioRay <= optimalUsageRatioRay) {
  rate += rateGrowthBeforeOptimal * U / U_opt;
} else {
  rate += rateGrowthBeforeOptimal + rateGrowthAfterOptimal * (U - U_opt) / (RAY - U_opt);
}
```

Two details that are easy to miss:

1. **`swept` is in the denominator.** Sending liquidity to a reinvestment strategy does not
   raise utilisation, so it does not raise borrow rates. This is the "interest rate neutral"
   promise in the docs, and it is enforced right here.
2. **`deficit` is accepted but ignored** — the parameter is literally commented out in the
   signature (`:106`). Bad debt does not push rates up.

Guards: `optimalUsageRatio ∈ [1_00, 99_00]`, and the sum of base + both slopes must be
≤ `MAX_ALLOWED_DRAWN_RATE = 1000_00` i.e. 1000% APR (`:18-24, 45-54`).

**Premium debt is deliberately excluded from the rate calculation** — `getDrawnRate` passes
only `asset.drawn(drawnIndex)` (`src/hub/libraries/AssetLogic.sol:170-183`). Premium is a
transfer from risky borrowers to suppliers, not a utilisation signal.

---

## 3. Hub external functions, traced

Every one of these follows the same skeleton, so I will state it once:

```
asset.accrue()  ->  _validateX(...)  ->  mutate shares/liquidity  ->  asset.updateDrawnRate(assetId)  ->  emit
```

`add`, `restore` and `reclaim` additionally use a **transfer-then-verify** pattern: the caller
must have already sent the tokens, and the Hub checks its own balance. There is no
`transferFrom` from the Spoke.

### 3.1 `add(assetId, amount) -> shares` — `src/hub/Hub.sol:200`

- **Caller**: an active, non-halted Spoke (`_validateAdd`, `:814-830`), plus add-cap check.
- **Precondition**: tokens already at the Hub.
- **Check**: `balance >= asset.liquidity + amount`, else `InsufficientTransferred`.
- **Writes**: `asset.addedShares +=`, `spoke.addedShares +=`, `asset.liquidity =`.
- **Rounding**: `toAddedSharesDown` — user gets the fewer shares. `require(shares > 0)`.
- **Emits**: `Add`.

The balance check rather than a `transferFrom` is what the interface means by "Extra
untracked underlying liquidity in the Hub can be skimmed into the Hub's liquidity accounting
through this action" (`src/hub/interfaces/IHubBase.sol:95`). Donated tokens are claimable by
whichever Spoke calls `add` next — deliberate, and the reason `TreasurySpoke.supplySkimmed`
exists (`src/spoke/TreasurySpoke.sol:31`).

### 3.2 `remove(assetId, amount, to) -> shares` — `:224`

`_validateRemove` (`:831-838`) blocks `to == address(this)`. Requires `amount <= liquidity`
(swept funds are *not* available — that is the reinvestment liquidity risk). Rounds
`toAddedSharesUp` (user burns more shares). Transfers out, emits `Remove`.

### 3.3 `draw(assetId, amount, to) -> drawnShares` — `:249`

`_validateDraw` (`:840-858`) enforces the draw cap against **total owed including deficit**:

```solidity
uint256 owed = _getSpokeDrawn(asset, spoke) + _getSpokePremium(asset, spoke);
require(drawCap == MAX_ALLOWED_SPOKE_CAP ||
        drawCap * 10**decimals >= owed + amount + uint256(spoke.deficitRay).fromRayUp(), ...);
```

A Spoke that has generated bad debt has its remaining borrowing capacity reduced by that bad
debt. Nice touch. Rounds `toDrawnSharesUp` (against the borrower). Emits `Draw`.

### 3.4 `restore(assetId, drawnAmount, premiumDelta) -> drawnShares` — `:274`

Repayment. `_validateRestore` (`:860-873`) checks you are not repaying more drawn or more
premium than exists. Then:

```solidity
uint120 drawnShares = asset.toDrawnSharesDown(drawnAmount).toUint120();   // rounds against repayer
asset.drawnShares -= drawnShares; spoke.drawnShares -= drawnShares;
_applyPremiumDelta(asset, spoke, premiumDelta);
uint256 premiumAmount = premiumDelta.restoredPremiumRay.fromRayUp();
uint256 liquidity = asset.liquidity + drawnAmount + premiumAmount;
require(IERC20(asset.underlying).balanceOf(address(this)) >= liquidity, InsufficientTransferred(...));
```

Note the doc comment at `src/hub/interfaces/IHubBase.sol:119`: *"Interest is always paid off
first from premium, then from drawn."* The ordering is implemented Spoke-side in
`calculateRestoreAmount` (`src/spoke/libraries/UserPositionUtils.sol:89-106`).

### 3.5 `reportDeficit(assetId, drawnAmount, premiumDelta) -> (shares, amount)` — `:304`

Write-off. Same share burn as `restore`, but **no tokens arrive**; instead:

```solidity
uint256 deficitAmountRay = uint256(drawnShares) * asset.drawnIndex + premiumDelta.restoredPremiumRay;
asset.deficitRay += deficitAmountRay.toUint200();
spoke.deficitRay += deficitAmountRay.toUint200();
```

Because `deficitRay` stays inside `_calculateAggregatedOwedRay`, `totalAddedAssets` does not
drop. Suppliers are not immediately marked down; the hole is explicit and must be filled.
Requires `active` but **tolerates `halted`** (`:875-887`).

### 3.6 `eliminateDeficit(assetId, amount, spoke) -> (shares, amount)` — `:333`

`restricted` **and** Spoke-only. The caller Spoke burns its own added shares to erase another
Spoke's deficit:

```solidity
uint256 deficitAmountRay = (amount < deficitRay.fromRayUp()) ? amount.toRay() : deficitRay;
uint120 shares = asset.toAddedSharesUp(deficitToEliminate).toUint120();
asset.addedShares -= shares; callerSpoke.addedShares -= shares;
asset.deficitRay -= ...; coveredSpoke.deficitRay -= ...;
```

In practice the caller is the treasury/Umbrella-style Spoke spending protocol-owned
liquidity. Note it burns shares without moving tokens — the tokens were already gone; this
just reassigns the loss from "unattributed hole" to "the covering Spoke's balance".

### 3.7 `refreshPremium(assetId, premiumDelta)` — `:362`

Pure bookkeeping. Explicitly `require(premiumDelta.restoredPremiumRay == 0)` — no value may
move. Used when a user's `RP_u` changes.

### 3.8 `payFeeShares` / `transferShares` — `:377`, `:392`

`payFeeShares` moves the caller Spoke's added shares to the asset's `feeReceiver` (used by
liquidation to pay the protocol fee, §4.4). `transferShares` is the general form to an
arbitrary Spoke, with the receiver's add-cap enforced (`:902-918`). Both require both sides
active and un-halted.

### 3.9 `sweep` / `reclaim` — `:406`, `:427`

Only callable by `asset.reinvestmentController` (`:920-931`; the zero-address default means
"disabled"). `sweep` moves `liquidity -> swept` and transfers out; `reclaim` reverses it with
the balance check. `updateAssetConfig` refuses to unset the controller while `swept != 0`
(`:126-129`).

### 3.10 Admin: `addAsset`, `updateAssetConfig`, `addSpoke`, `updateSpokeConfig`, `setInterestRateData`, `mintFeeShares`

All `restricted` (AccessManager). `addAsset` (`:47`) requires decimals in `[6, 18]`, refuses
duplicate underlyings, initialises `drawnIndex = RAY`, and auto-registers the fee receiver as
a Spoke. `updateAssetConfig` (`:115`) mints outstanding fee shares *before* switching fee
receiver, and requires `irData` to be empty unless the strategy actually changes.

---

## 4. Spoke external functions, traced

All user actions carry `nonReentrant` (transient-storage reentrancy guard,
`src/spoke/Spoke.sol:39`) and `onlyPositionManager(onBehalfOf)`
(`:90-93`) — which passes trivially when `msg.sender == onBehalfOf`.

### 4.1 `supply(reserveId, amount, onBehalfOf)` — `src/spoke/Spoke.sol:225`

```solidity
_validateSupply(reserve.flags);                                   // !paused && !frozen
IERC20(reserve.underlying).safeTransferFrom(msg.sender, address(reserve.hub), amount);
uint256 suppliedShares = reserve.hub.add(reserve.assetId, amount);
userPosition.suppliedShares += suppliedShares.toUint120();
emit Supply(...);
```

Six lines. **No health-factor check, no risk-premium refresh** — supplying only reduces risk,
and per `docs/overview.md` the premium is deliberately not refreshed on risk-reducing actions
(a user with improved collateral must call `updateUserRiskPremium` to benefit). Also note
supply does **not** auto-enable collateral; that is a separate `setUsingAsCollateral` call.
That is a behavioural change from v3, where the first supply of an asset auto-enabled it.

### 4.2 `withdraw` — `:244`

```solidity
uint256 withdrawnAmount = MathUtils.min(amount, hub.previewRemoveByShares(assetId, userPosition.suppliedShares));
uint256 withdrawnShares = hub.remove(assetId, withdrawnAmount, msg.sender);
userPosition.suppliedShares -= withdrawnShares.toUint120();
if (_positionStatus[onBehalfOf].isUsingAsCollateral(reserveId)) {
  uint256 newRiskPremium = _refreshAndValidateUserAccountData(onBehalfOf).riskPremium;
  _notifyRiskPremiumUpdate(onBehalfOf, newRiskPremium);
}
```

`type(uint256).max` works as "withdraw everything" via the `min`. The HF check and premium
refresh happen **only if the reserve was collateral** — cheap path otherwise. Output goes to
`msg.sender`, not `onBehalfOf`: a position manager receives the funds and is responsible for
forwarding.

### 4.3 `borrow` — `:274`

```solidity
uint256 drawnShares = hub.draw(reserve.assetId, amount, msg.sender);
userPosition.drawnShares += drawnShares.toUint120();
if (!positionStatus.isBorrowing(reserveId)) {
  require(MAX_USER_RESERVES_LIMIT == MAX_ALLOWED_USER_RESERVES_LIMIT ||
          positionStatus.borrowCount(_reserveCount) < MAX_USER_RESERVES_LIMIT, MaximumUserReservesExceeded());
  positionStatus.setBorrowing(reserveId, true);
}
uint256 newRiskPremium = _refreshAndValidateUserAccountData(onBehalfOf).riskPremium;
_notifyRiskPremiumUpdate(onBehalfOf, newRiskPremium);
```

The order is draw-first, check-after — safe because `_refreshAndValidateUserAccountData`
reverts the whole transaction if HF < 1. `_refreshAndValidateUserAccountData` (`:686`) calls
`_processUserAccountData(user, true)`, which **rebinds every collateral's `dynamicConfigKey`
to the latest** as a side effect, then requires HF ≥ 1. This is the "automatic rebinding and
hard safety guard" from the docs: you get the newest risk parameters, and if you cannot
survive them, you cannot borrow.

### 4.4 `repay` — `:305`

```solidity
uint256 drawnIndex = reserve.hub.getAssetDrawnIndex(reserve.assetId);
(uint256 drawnDebtRestored, uint256 premiumDebtRayRestored) = userPosition.calculateRestoreAmount(drawnIndex, amount);
uint256 restoredShares = drawnDebtRestored.rayDivDown(drawnIndex);
IHubBase.PremiumDelta memory premiumDelta = userPosition.calculatePremiumDelta({...});
uint256 totalDebtRestored = drawnDebtRestored + premiumDebtRayRestored.fromRayUp();
IERC20(reserve.underlying).safeTransferFrom(msg.sender, address(reserve.hub), totalDebtRestored);
reserve.hub.restore(reserve.assetId, drawnDebtRestored, premiumDelta);
userPosition.applyPremiumDelta(premiumDelta);
userPosition.drawnShares -= restoredShares.toUint120();
if (userPosition.drawnShares == 0) positionStatus.setBorrowing(reserveId, false);
```

`calculateRestoreAmount` (`src/spoke/libraries/UserPositionUtils.sol:89-106`) implements
premium-first: if `amount < premiumDebt`, everything goes to premium and `drawnDebt` untouched.
No HF check — repaying cannot hurt. No premium refresh either (risk-reducing).

### 4.5 `liquidationCall(collateralReserveId, debtReserveId, user, debtToCover, receiveShares)` — `:347`

The Spoke computes account data, hands everything to the external library, then branches:

```solidity
bool isUserInDeficit = LiquidationLogic.liquidateUser({...});
if (isUserInDeficit) {
  LiquidationLogic.notifyReportDeficit(_reserves, _userPositions, _positionStatus, _reserveCount, user);
} else {
  uint256 newRiskPremium = _calculateUserAccountData(user).riskPremium;
  _notifyRiskPremiumUpdate(user, newRiskPremium);
}
```

Full walk in §5.

### 4.6 `setUsingAsCollateral` — `:391`

Early-returns if unchanged. On **enable**, only `_refreshDynamicConfig` for that one reserve.
On **disable**, full `_refreshAndValidateUserAccountData` (rebinding all keys) plus premium
refresh — because dropping collateral is risk-increasing. Exactly matches the docs.

### 4.7 `updateUserRiskPremium` / `updateUserDynamicConfig` — `:415`, `:424`

Permissioned by an interesting fallback:

```solidity
if (!_isPositionManager({user: onBehalfOf, manager: msg.sender})) { _checkCanCall(msg.sender, msg.data); }
```

Either you are the user / their manager, **or** you hold the AccessManager role. That is the
Governor's power to force-refresh a stale position. The difference between the two: risk
premium only recomputes `RP_u`; dynamic config also rebinds `dynamicConfigKey` snapshots and
therefore validates HF.

### 4.8 Position-manager plumbing — `:433-467`

`setUserPositionManager` (self), `setUserPositionManagersWithSig` (EIP-712 batch, verified via
`IntentConsumer`), `renouncePositionManagerRole` (the manager gives up its own approval).
Approval is two-sided: the user must approve **and** governance must have set
`_positionManager[pm].active = true` via `updatePositionManager` (`:219`).

```solidity
// src/spoke/Spoke.sol:909-913
function _isPositionManager(address user, address manager) internal view returns (bool) {
  ... // user == manager, or (config.active && config.approval[user])
}
```

`permitReserve` (`:469`) wraps `IERC20Permit.permit` in `try/catch{}` — swallowing failures so
a front-run permit cannot brick a multicall.

### 4.9 `_processUserAccountData` — the heart — `:706`

One pass over the user's bitmap computing collateral, debt, HF, and risk premium.

```solidity
uint256 assetPrice = IAaveOracle(ORACLE).getReservePrice(reserveId);

if (collateral) {
  uint256 collateralFactor = _dynamicConfig[reserveId][
    refreshConfig ? (userPosition.dynamicConfigKey = reserve.dynamicConfigKey)  // <-- assignment inside index!
                  : userPosition.dynamicConfigKey
  ].collateralFactor;
  ...
  uint256 userCollateralValue = reserve.hub.previewRemoveByShares(reserve.assetId, suppliedShares)
                                     .toValue({decimals: assetDecimals, price: assetPrice});
  accountData.totalCollateralValue += userCollateralValue;
  collateralInfo.add(accountData.activeCollateralCount, reserve.collateralRisk, userCollateralValue);
  accountData.avgCollateralFactor += collateralFactor * userCollateralValue;
}
```

That `(userPosition.dynamicConfigKey = reserve.dynamicConfigKey)` inside the mapping index is
the rebinding side effect — terse, and the reason `_calculateUserAccountData` is a non-view
function that merely *promises* not to write (`:634` has a `// SAFETY:` comment and `_castToView`
at `:936` to launder it back to `view` for external getters).

Health factor:

```solidity
accountData.healthFactor = Math.mulDiv(accountData.avgCollateralFactor.bpsToWad(),
                                       WadRayMath.RAY, accountData.totalDebtValueRay, Math.Rounding.Floor);
```

At that point `avgCollateralFactor` is still the CF-weighted *sum*, so this is
`Σ(collateral_i × CF_i) / totalDebt` — the same shape as v3's HF, with CF playing the role of
the liquidation threshold. **v4 has one collateral factor, not v3's separate LTV and liquidation
threshold.** If there is no debt, HF is `type(uint256).max`.

Then the risk-premium algorithm — sort by risk ascending, consume collateral until debt is
covered, take the weighted average:

```solidity
collateralInfo.sortByKey();   // ASC by collateralRisk
uint256 totalDebtValue = accountData.totalDebtValueRay.fromRayUp();
uint256 debtValueLeftToCover = totalDebtValue;
for (uint256 index = 0; index < collateralInfo.length(); ++index) {
  if (debtValueLeftToCover == 0) break;
  (uint256 collateralRisk, uint256 userCollateralValue) = collateralInfo.uncheckedAt(index);
  userCollateralValue = userCollateralValue.min(debtValueLeftToCover);
  accountData.riskPremium += userCollateralValue * collateralRisk;
  debtValueLeftToCover = debtValueLeftToCover.uncheckedSub(userCollateralValue);
}
if (debtValueLeftToCover < totalDebtValue) {
  accountData.riskPremium = accountData.riskPremium.divUp(totalDebtValue.uncheckedSub(debtValueLeftToCover));
}
```

Cheapest-risk-first is user-friendly: posting extra safe collateral genuinely lowers your rate,
and posting a small risky asset alongside a large safe one barely moves it.

`_notifyRiskPremiumUpdate` (`:822`) then loops the user's borrowed reserves and pushes a
`refreshPremium` into each relevant Hub. Early-returns when premium is and was zero.

### 4.10 `TokenizationSpoke` — the ERC4626 face

`src/spoke/TokenizationSpoke.sol:18`. Not a `Spoke` subclass at all — it inherits
`ERC20Upgradeable`, not `Spoke`. It is a **supply-only** Spoke: one Hub, one asset, fixed at
construction (`:47-54`), no borrowing, no collateral, no liquidation.

```solidity
// :418-427
function _pullAndDepositAssets(address from, uint256 amount) internal virtual {
  IERC20(ASSET).safeTransferFrom(from, address(HUB), amount);
  HUB.add(ASSET_ID, amount);
}
function _removeAndPushAssets(address to, uint256 amount) internal virtual {
  HUB.remove(ASSET_ID, amount, to);
}
```

**Is it non-rebasing? Yes.** The share token is a plain ERC20 whose balance never changes on
its own; `convertToAssets(shares)` grows as the Hub's share price grows (`:261-263`,
`totalAssets()` at `:304` is `previewRedeem(totalSupply())`). This is the v4 equivalent of v3's
`StataTokenV2`, except it is a first-class citizen rather than a wrapper over a rebasing aToken
— because **v4 has no rebasing token to wrap**. Aave's aToken concept is simply gone: the
`Spoke` tracks `suppliedShares` in a mapping, and if you want a transferable ERC20 receipt you
use a `TokenizationSpoke`.

It also carries a full EIP-712 surface (`depositWithSig`, `mintWithSig`, `withdrawWithSig`,
`redeemWithSig`, `permit`, `depositWithPermit`) and `maxDeposit`/`maxMint` that read the Hub's
spoke config so ERC4626 max-functions respect caps and halts (`:267-289`).

### 4.11 Position managers — `src/position-manager/`

Five contracts, all on `PositionManagerBase` (`Ownable2Step`, `Rescuable`, `Multicall`, plus a
`_registeredSpokes` allowlist):

| Contract | Purpose |
|---|---|
| `GiverPositionManager` | `supplyOnBehalfOf` / repay — the *safe* direction, needs no user approval beyond token allowance |
| `TakerPositionManager` | `withdrawOnBehalfOf` / `borrowOnBehalfOf` with per-reserve **allowances** (`approveWithdraw`, `approveBorrow`, and `...WithSig`) — the v4 replacement for v3's credit delegation |
| `ConfigPositionManager` | `setUsingAsCollateralOnBehalfOf`, `updateUserRiskPremiumOnBehalfOf`, `updateUserDynamicConfigOnBehalfOf`, with a granular permission bitmap (`ConfigPermissionsMap`) |
| `SignatureGateway` | every Spoke action from a signature: `supplyWithSig`, `withdrawWithSig`, `borrowWithSig`, `repayWithSig`, ... — gasless UX |
| `NativeTokenGateway` | ETH ⟷ WETH wrapping: `supplyNative`, `borrowNative`, `repayNative`, ... |

Why they exist: the Spoke's `onlyPositionManager` hook is a *generic* delegation primitive, so
features that were hardcoded in v3 (credit delegation on the debt token, the WETH gateway,
permit flows) become pluggable contracts that governance activates individually. Note
`TakerPositionManager` keeps its own allowance accounting (`:274-334`) — the Spoke only knows
"this manager is approved for this user", so the *granularity* lives in the manager.

---

## 5. Liquidation engine — `src/spoke/libraries/LiquidationLogic.sol`

An `external` library (delegatecall-linked; `Spoke.getLiquidationLogic()` at `:667` exposes its
address), so its code is deployed once and shared.

Constants: `HEALTH_FACTOR_LIQUIDATION_THRESHOLD = 1e18` (`:182`),
`DUST_LIQUIDATION_THRESHOLD = 1000e26` (`:185`) — 1000 USD in the internal value unit where
`1e26 == 1 USD`.

### 5.1 Value units

`toValue` (`src/spoke/libraries/SpokeUtils.sol:34-40`):

```solidity
return amount * price * MathUtils.uncheckedExp(10, WadRayMath.WAD_DECIMALS - decimals);
```

With `ORACLE_DECIMALS = 8` (`:13`), a value is `amount_normalised_to_18dp × price_8dp`, so
1 USD = `1e18 × 1e8 = 1e26`. That is where `1000e26` comes from.

### 5.2 Dutch-auction bonus — `:312`

```solidity
if (healthFactor <= healthFactorForMaxBonus) return maxLiquidationBonus;
uint256 minLiquidationBonus = (maxLiquidationBonus - PERCENTAGE_FACTOR).percentMulDown(liquidationBonusFactor)
                              + PERCENTAGE_FACTOR;
return minLiquidationBonus + (maxLiquidationBonus - minLiquidationBonus)
       .mulDivDown(HEALTH_FACTOR_LIQUIDATION_THRESHOLD - healthFactor,
                   HEALTH_FACTOR_LIQUIDATION_THRESHOLD - healthFactorForMaxBonus);
```

Linear from `minLB` at HF = 1.0 down to `maxLB` at HF ≤ `healthFactorForMaxBonus`. A position
that just crossed the line is liquidated cheaply for the borrower; a deeply unhealthy one pays
the full bonus. v3's flat bonus is gone.

### 5.3 How much debt — `:735` and `:795`

Target-HF sizing replaces v3's close factor:

```solidity
uint256 liquidationPenalty = params.liquidationBonus.bpsToWad().percentMulUp(params.collateralFactor);
return Math.mulDiv(params.totalDebtValueRay,
                   params.debtAssetUnit * (params.targetHealthFactor - params.healthFactor),
                   (params.targetHealthFactor - liquidationPenalty) * params.debtAssetPrice.toWad(),
                   Math.Rounding.Ceil);
```

Standard "repay just enough to reach HF_target" algebra. The denominator is safe because
`liquidationBonus × collateralFactor < 100%` is enforced in `_validateDynamicReserveConfig`
(`src/spoke/Spoke.sol:921`).

Then `_calculateDebtToLiquidate` (`:735`) applies, in order: premium debt first, capped by
`debtToCover`; then drawn shares up to `min(toTarget, toCover, all)`; then the dust rule:

```solidity
bool leavesDebtDust = (drawnSharesToLiquidate < params.drawnShares) &&
  debtRayRemaining.toValue({...}) < DUST_LIQUIDATION_THRESHOLD.toRay();
if (leavesDebtDust) { drawnSharesToLiquidate = params.drawnShares; premiumDebtRayToLiquidate = params.premiumDebtRay; }
```

If finishing would leave under $1000 of debt, the target HF is **bypassed** and the whole
position is repaid. This is why v4 does not need a "100% close factor below HF 0.95" special
case like v3.3.

### 5.4 Execution — `:342`

`_executeLiquidation` → `_validateLiquidationCall` (`:533`) → `_calculateLiquidationAmounts`
(`:563`) → `_liquidateCollateral` (`:450`) → `_liquidateDebt` (`:493`) → emit → `_evaluateDeficit`.

Validation blocks: self-liquidation, zero `debtToCover`, either reserve `paused`, no supplied
shares, no drawn shares, HF ≥ 1, collateral not enabled or `CF == 0`, and — if `receiveShares`
— the collateral reserve being `frozen` or not `receiveSharesEnabled`. Frozen reserves *can*
still be liquidated for underlying.

`_liquidateCollateral` splits the seized shares:

```solidity
if (params.sharesToLiquidator > 0) {
  if (params.receiveShares) liquidatorPosition.suppliedShares += params.sharesToLiquidator.toUint120();
  else { ...; params.hub.remove(params.assetId, amountToLiquidator, params.liquidator); }
}
uint256 feeShares = params.sharesToLiquidate - params.sharesToLiquidator;
if (feeShares > 0) params.hub.payFeeShares(params.assetId, feeShares);
```

`receiveShares: true` credits the liquidator a supply position **inside the same Spoke**
without touching Hub liquidity — that is what makes liquidation possible when the pool is
fully utilised. The protocol fee goes to the fee receiver as shares (so it keeps earning),
via `payFeeShares`.

`_liquidateDebt` pulls `drawn + premium` from the liquidator straight to the Hub and calls
`restore`.

### 5.5 Deficit — `:816`

```solidity
function _evaluateDeficit(bool isCollateralPositionEmpty, bool isDebtPositionEmpty,
                          uint256 activeCollateralCount, uint256 borrowCount) internal pure returns (bool) {
  if (!isCollateralPositionEmpty || activeCollateralCount > 1) return false;
  return !isDebtPositionEmpty || borrowCount > 1;
}
```

Deficit only when **all** collateral is gone (this was the last one) **and** debt remains
(here or elsewhere). Then `notifyReportDeficit` (`:260`) zeroes `riskPremium`, walks every
borrowed reserve, and calls `hub.reportDeficit` for each — clearing the user's entire debt
book in one transaction and emitting `UpdateUserRiskPremium(user, 0)`.

### 5.6 Worked liquidation

Setup: WETH collateral (CF 80%, `maxLiquidationBonus` 105_00, `liquidationFee` 10_00),
USDC debt, `targetHealthFactor` 1.05e18, `healthFactorForMaxBonus` 0.95e18,
`liquidationBonusFactor` 80_00.

User: 10 WETH, 12,000 USDC debt (ignore premium for clarity).

- ETH at 2000 → collateral 20,000; `HF = 20,000 × 0.80 / 12,000 = 1.333`. Healthy.
- ETH falls to 1700 → collateral 17,000; `HF = 17,000 × 0.80 / 12,000 = 1.133`. Still healthy.
- ETH falls to 1450 → collateral 14,500; `HF = 14,500 × 0.80 / 12,000 = 0.9667`. **Liquidatable.**

Bonus: HF 0.9667 > 0.95, so interpolate.
`minLB = (10500 − 10000) × 0.80 + 10000 = 10400` (4%).
`lb = 10400 + (10500 − 10400) × (1.0 − 0.9667)/(1.0 − 0.95) = 10400 + 100 × 0.667 ≈ 10467` (≈4.67%).

Debt to target: `penalty = 1.0467 × 0.80 = 0.8374`.
`debt = 12,000 × (1.05 − 0.9667) / (1.05 − 0.8374) = 12,000 × 0.0833 / 0.2126 ≈ 4,702 USDC`.

Collateral seized: `4,702 × 1.0467 / 1450 ≈ 3.394 WETH`.
Of that, the fee is 10% of the *bonus* portion: bonus ≈ `4,702 × 0.0467 ≈ 220` → fee ≈ `22`
→ ≈ 0.0151 WETH to the fee receiver, ≈ 3.379 WETH to the liquidator.

After: debt `12,000 − 4,702 = 7,298`; collateral `10 − 3.394 = 6.606 WETH = 9,579`.
`HF = 9,579 × 0.80 / 7,298 = 1.0500`. Target hit.

Remaining debt is far above $1000, so no dust bypass; collateral remains, so no deficit.

---

## 6. Governance, config engine, deployment

`AccessManagerEnumerable` (`src/access/AccessManagerEnumerable.sol:13`) extends OpenZeppelin's
`AccessManager` with enumeration of roles, members, targets and selectors. Both `Hub` and
`Spoke` are `AccessManagedUpgradeable`, so every `restricted` function is gated by
*(target contract, function selector) → roleId*, with OZ's built-in **execution delays** and
role admin delays. This is a real upgrade over v3's `ACLManager`, which had a fixed set of
named roles (`POOL_ADMIN`, `RISK_ADMIN`, …) and no per-selector granularity or delays.

Configurators are thin `AccessManaged` facades that call the Hub/Spoke with friendlier,
narrower functions — `HubConfigurator` has `haltSpoke`, `resetSpokeCaps`, `deactivateAsset`,
`updateSpokeAddCap`, … (`src/hub/HubConfigurator.sol`); `SpokeConfigurator` has
`pauseAllReserves`, `addCollateralFactor`, `updateLiquidationFee`, … Each fine-grained
selector can be assigned to a different role with a different delay, which is the point:
an emergency guardian can hold only `haltSpoke`/`pauseAllReserves` with zero delay, while
`addCollateralFactor` sits behind a long-delay governance role.

`AaveV4Payload` (`src/config-engine/AaveV4Payload.sol:10`) is the DAO-proposal base. You
override the hooks you need and `execute()` (`:28`) runs them in phases:
`_executeHubActions` → `_executeSpokeActions` → `_executeAccessManagerActions` →
`_executePositionManagerActions`, each `delegatecall`ing `AaveV4ConfigEngine` (`:429`).
Hooks include `hubAssetListings`, `hubSpokeToAssetsAdditions`, `spokeReserveListings`,
`spokeDynamicReserveConfigAdditions`, `accessManagerTargetAdminDelayUpdates`, and
`positionManagerSpokeRegistrations`.

### 6.1 Dynamic risk configuration

`DynamicReserveConfig` = `{collateralFactor, maxLiquidationBonus, liquidationFee}`
(`src/spoke/interfaces/ISpoke.sol:73-77`). It lives in
`_dynamicConfig[reserveId][dynamicConfigKey]`, keyed by an incrementing `uint32`.

- `addDynamicReserveConfig` (`src/spoke/Spoke.sol:191`) creates a **new** key. Existing
  positions keep pointing at their old key and are unaffected.
- `updateDynamicReserveConfig` (`:207`) edits an existing key, and *does* affect positions
  bound to it — the escape hatch when an old config is dangerous.
- A position's key is rebound to the latest only on risk-increasing actions (`borrow`,
  `withdraw`, disabling collateral) via the assignment inside `_processUserAccountData`, and
  the rebinding must leave HF ≥ 1 or the action reverts.

This is the direct fix for v3's biggest governance pain: tightening a parameter no longer
instantly liquidates existing users.

### 6.2 Listing an asset and adding a Spoke

```
Hub side:                                       Spoke side:
  HubConfigurator.addAsset(hub, underlying,       SpokeConfigurator.addReserve(spoke, hub, assetId,
      feeReceiver, irStrategy, irData)                priceSource, ReserveConfig, DynamicReserveConfig)
    -> Hub.addAsset            [:47]                -> Spoke.addReserve                 [:121]
       assetId = _assetCount++                         reserveId = _reserveCount++
       drawnIndex = RAY                                _hubAssetIdToReserveId[hub][assetId] = reserveId
       registers feeReceiver as a spoke                dynamicConfigKey = 0
  HubConfigurator.addSpoke(hub, assetId, spoke,     AaveOracle.setReserveSource(reserveId, feed)
      SpokeConfig{addCap, drawCap, riskPremiumThreshold, active, halted})
    -> Hub.addSpoke            [:158]
```

Adding a Spoke to a **live** Hub is just `Hub.addSpoke` plus caps. No liquidity moves, no
migration, existing users unaffected. That is the whole thesis of the architecture.

---

## 7. v3 vs v4

| Dimension | Aave v3.6 (`aave/AAVE-DEEP-DIVE.md`) | Aave v4 |
|---|---|---|
| Liquidity model | One `Pool` per market; each market siloed | One `Hub` per asset set; many `Spoke`s share it |
| Token custody | Each aToken holds its own underlying | The **Hub** holds all underlying |
| Supply receipt | Rebasing `aToken` (ERC20) | `suppliedShares` in a mapping; optional ERC4626 `TokenizationSpoke` |
| Debt receipt | Non-transferable `VariableDebtToken` | `drawnShares` in `UserPosition`; no token |
| Supply accounting | `liquidityIndex`, linear accrual | ERC4626 shares vs `totalAddedAssets`, virtual offset 1e6 |
| Debt accounting | `variableBorrowIndex`, binomial compounding | `drawnIndex`, **linear per interval**, compounds per interaction |
| Rate model | Kinked, per reserve | Kinked, per `assetId`, `swept` in denominator, `deficit` ignored |
| Risk pricing | Same rate for all borrowers | + Risk Premium from collateral quality (premium shares) |
| Risk params | Separate LTV **and** liquidation threshold | Single `collateralFactor` |
| Param changes | Global, immediate, affects all positions | Versioned `dynamicConfigKey`; old positions keep old params |
| Liquidation size | 50% close factor (100% below HF 0.95) | Repay to `targetHealthFactor`; dust rule forces full repay |
| Liquidation bonus | Flat per reserve | Dutch auction, linear in HF |
| Liquidator payout | Underlying or aToken | Underlying or Hub shares (`receiveShares`) |
| Bad debt | `deficit` + `eliminateReserveDeficit` (Umbrella) | `deficitRay` per asset & spoke + `eliminateDeficit` by any authorised Spoke |
| Access control | `ACLManager`, fixed named roles | OZ `AccessManager` per-selector roles **with delays** |
| Upgradeability | `Pool` behind a proxy | **Hub immutable**, Spokes upgradeable |
| Extensibility | New market = new Pool = new liquidity | New Spoke on an existing Hub, zero migration |
| E-Mode | `eModeCategory` bitmaps in the Pool | Not present under that name; per-Spoke config plays the role (see below) |
| Delegation | `approveDelegation` on the debt token | Generic position managers |
| Native ETH | `WrappedTokenGatewayV3` | `NativeTokenGateway` position manager |

### What a v3 integrator must relearn

1. **There is no aToken.** Do not expect `balanceOf` to grow. Read `getUserSuppliedAssets`
   (`src/spoke/Spoke.sol:573`) or hold a `TokenizationSpoke` share token.
2. **Address by id, not by token.** Hub calls take `assetId`; Spoke calls take `reserveId`.
   Resolve with `getAssetId` / `getReserveId` (`src/spoke/Spoke.sol:529`).
3. **Supply does not enable collateral.** Call `setUsingAsCollateral` explicitly.
4. **Your borrow rate depends on your collateral mix**, and it only refreshes on certain
   actions. If you improve your collateral, call `updateUserRiskPremium` to get the benefit.
5. **`withdraw`/`borrow` output goes to `msg.sender`**, not `onBehalfOf`.
6. **Liquidations are not 50%.** Size them from `targetHealthFactor`, and watch the dust rule.
7. **Two vocabularies.** User-facing supply/borrow at the Spoke, add/draw at the Hub.
8. **Cross-chain**: I found no cross-chain messaging in this repo. The hub-and-spoke naming is
   about contract topology within a chain, not bridging. Nothing in `src/` imports a bridge or
   messaging interface. Treat "cross-chain Aave v4" as unconfirmed by this source.

**On E-Mode:** I could not find an E-Mode equivalent by name in v4 (no `eMode` identifiers in
`src/spoke/`). The architectural replacement appears to be *deploy a Spoke per risk regime* —
a correlated-assets Spoke with high collateral factors and low collateral risk, drawing from
the same Hub. That reading is consistent with the docs' framing but the code does not state
it, so treat it as my inference rather than a documented fact.

---

## 8. Security notes

**Trust boundaries.** The Hub trusts Spokes to be honest about *users*, but not about
*value*. A registered Spoke can: add/remove its own shares, draw up to `drawCap`, and report
deficit up to what it owes. It cannot: touch another Spoke's shares (every mutation is keyed
on `msg.sender`), create premium out of thin air (`_validateApplyPremiumDelta` conserves
value, `src/hub/Hub.sol:933`), exceed `riskPremiumThreshold` (`:756`), or take liquidity
beyond `asset.liquidity`. Conversely Spokes fully trust their Hub, the interest-rate strategy,
and their price feeds — `docs/overview.md:79` says so explicitly and notes the reentrancy
guards are defence in depth, since v4 "does not support tokens with callbacks".

That last point is a real integration constraint: **ERC777 / callback tokens and, by
implication, fee-on-transfer tokens are out of scope.** The `add`/`restore` balance checks
would misbehave with fee-on-transfer since the Hub credits `amount` and only checks
`balance >= liquidity + amount`.

**Upgradeable Spokes.** The Hub is immutable but Spokes are not. A malicious Spoke upgrade
can lie to its own users about health factors and liquidate them, and can draw up to its
`drawCap`. Your exposure as a supplier to Spoke A is therefore bounded by every *other*
Spoke's `drawCap` on the shared asset — caps are the containment mechanism, and they matter
much more here than in v3.

**Share manipulation / donation.** `VIRTUAL_ASSETS = VIRTUAL_SHARES = 1e6`
(`src/hub/libraries/SharesMath.sol:13-14`) blunts first-depositor inflation. Direct token
donation to the Hub is not a share-price attack — an untracked balance is not counted in
`totalAddedAssets` until some Spoke calls `add`, at which point that Spoke *claims* it. So
donations are a gift to the next caller, not a way to skew the price. Rounding is
systematically against the user (`toAddedSharesDown` in, `toAddedSharesUp` out;
`toDrawnSharesUp` on draw, `toDrawnSharesDown` on restore).

**Reinvestment.** `sweep` moves real tokens out to the controller and they stay in
`totalAddedAssets` (`AssetLogic.sol:91`). If the strategy loses money, that value is
*already counted* as belonging to suppliers. The docs say the Governor absorbs losses; the
code does not enforce that anywhere I could find. Suppliers to a reinvestment-enabled asset
therefore carry strategy risk that is only mitigated by governance policy, plus withdrawal
liquidity risk because `remove` can only draw on `asset.liquidity`, not `swept`.

**Per-Spoke oracles.** `ORACLE` is immutable per Spoke (`src/spoke/Spoke.sol:62, 96-102`),
and `AaveOracle.getReservePrice` reverts on a non-positive price (`src/spoke/AaveOracle.sol:80`).
Two Spokes on the same Hub asset can use *different* price feeds. A bad feed on one Spoke
lets that Spoke's users over-borrow, and the loss lands on the shared Hub asset as deficit —
i.e. **oracle risk is shared across Spokes even though oracle configuration is not.** Caps
are again the containment.

**Deficit socialisation.** Until someone calls `eliminateDeficit`, a deficit is carried inside
`totalAddedAssets`, so supplier share price is *not* marked down. Suppliers are effectively
holding an IOU. If it is never eliminated, the last withdrawers are the ones who cannot exit
— a slow-motion bank run rather than an instant haircut. Also note `deficitRay` is excluded
from the rate calculation, so bad debt does not signal through rates.

**AccessManager delays.** OZ `AccessManager` role-grant and target-admin delays are the
timelock. A zero-delay role on a powerful selector silently removes governance protection.
`AccessManagerEnumerable` at least makes the full role/target/selector graph enumerable
on-chain, which is exactly what you want for monitoring.

**Audit surface.** `audits/`:

- `2025-10-20_Aave-V4_Blackthorn.pdf`
- `2025-11-06_Aave-V4_TrailOfBits.pdf`
- `2026-01-28_Aave-V4_ChainSecurity.pdf`
- `2026-02-10_TokenizationSpoke_ChainSecurity.pdf`
- `2026-02-24_Aave-V4_Blackthorn.pdf`
- `2026-03-09_Hub-Fomal-Verification_Certora.pdf`
- `2026-03-09_Libraries-Fomal-Verification_Certora.pdf`
- `2026-03-09_Spoke-Fomal-Verification_Certora.pdf`
- `2026-03-23_Aave-V4_ChainSecurity.pdf`
- `2026-04-13_TokenizationSpoke-Fomal-Verification_Certora.pdf`

Three separate Certora formal-verification runs (Hub, Spoke, Libraries) plus one for
`TokenizationSpoke` — consistent with §2.5, where the core invariants are maintained
structurally rather than by runtime asserts. I have not read the PDF contents; this is a
listing, not a summary of findings.

---

## 9. Exercises to trace yourself

1. **Follow one dollar of interest.** Start at `src/hub/libraries/AssetLogic.sol:141`
   (`accrue`). Show why a supplier's balance grows without any write to their position: track
   `drawnIndex` → `_calculateAggregatedOwedRay` (`:229`) → `totalAddedAssets` (`:79`) →
   `toAddedAssetsDown` (`:107`).

2. **Prove premium conserves value.** Take `calculatePremiumDelta`
   (`src/spoke/libraries/UserPositionUtils.sol:54`) with `drawnSharesTaken = 0`,
   `restoredPremiumRay = 0`, and a new `riskPremium`. Substitute into
   `_validateApplyPremiumDelta` (`src/hub/Hub.sol:933`) and show
   `premiumRayAfter == premiumRayBefore` algebraically.

3. **Find the rebinding side effect.** In `_processUserAccountData`
   (`src/spoke/Spoke.sol:706`), locate the assignment inside a mapping index. Then explain why
   `_calculateUserAccountData` (`:699`) is not `view` and what `_castToView` (`:936`) is for.

4. **Rate neutrality of sweeping.** In `AssetInterestRateStrategy.calculateInterestRate`
   (`src/hub/AssetInterestRateStrategy.sol:102`), show that moving X from `liquidity` to
   `swept` leaves `usageRatioRay` unchanged. Then find in `Hub.sweep` (`:406`) why a supplier
   might still fail to withdraw.

5. **Walk a dust liquidation.** In `_calculateDebtToLiquidate`
   (`src/spoke/libraries/LiquidationLogic.sol:735`), construct inputs where `leavesDebtDust`
   is true and show the target HF is exceeded. What stops this being abused to force
   full liquidation of a barely-unhealthy whale? (Hint: `DUST_LIQUIDATION_THRESHOLD` is
   absolute, not proportional — `:185`.)

6. **Deficit truth table.** Enumerate all four boolean combinations in `_evaluateDeficit`
   (`:816`) crossed with `activeCollateralCount`/`borrowCount` of 1 vs >1. For each, say
   whether `notifyReportDeficit` (`:260`) runs.

7. **Cap containment.** Assume Spoke B is compromised. Using `_validateDraw`
   (`src/hub/Hub.sol:840`) and `_validateAdd` (`:814`), write the exact expression bounding
   how much of asset `i` Spoke B can remove from the shared pool. Confirm `deficitRay` is
   inside that bound.

8. **Build the smallest Spoke.** Read `TokenizationSpoke` (`src/spoke/TokenizationSpoke.sol:18`)
   and list every Hub function it calls. Then explain why it does not inherit `Spoke` and what
   it gives up by not doing so.
