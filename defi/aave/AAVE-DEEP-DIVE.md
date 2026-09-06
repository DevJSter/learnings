# Aave v3 Deep Dive

> Source: `aave/aave-v3-origin` (cloned from `aave-dao/aave-v3-origin`, `main` branch).
> All paths below are relative to `aave/aave-v3-origin/`. Line numbers were verified against the clone with `grep -n`.
>
> **Version note.** `package.json` says `3.6.0`, but the code is the *post-3.6 main branch* with the
> v3.7 changes already merged (`PoolInstance.POOL_REVISION = 11`, `src/contracts/instances/PoolInstance.sol:15`;
> `docs/3.7/Aave-v3.7-changelog.md`). Concretely, in this tree:
> - **Isolation mode, siloed borrowing and the debt ceiling are gone** (`IsolationModeLogic.sol` deleted; bits 61/62 and 212-251 of the reserve config are "unoccupied holes", `ReserveConfiguration.sol:22,30`).
> - **PriceOracleSentinel / sequencer grace period is gone** (`docs/3.7/sentinel-removal.md`).
> - **`dropReserve` is gone** (`docs/3.7/drop-reserve-removal.md`).
> - **A new `isolated` flag on eMode categories** replaces isolation mode (`DataTypes.sol:147`, `docs/3.7/isolated-emode.md`).
> - **Stable-rate debt was removed in 3.2**, **`unbacked`/Portals in 3.4**, and there is exactly one debt token per reserve: the `VariableDebtToken`.
>
> When this doc says "Aave v3" it means the code in this tree. Where a behaviour is version-specific I say which version introduced it.

---

## 0. Lending protocol mental model

A lending protocol is a **pooled, over-collateralized, algorithmic-rate money market**. Every concept in Aave's code maps to one of these words.

### 0.1 Pooled liquidity
Suppliers do not lend to a specific borrower. They deposit an asset (say USDC) into a shared pool and receive a claim token (`aUSDC`). Borrowers draw from that same pool. Interest paid by borrowers flows to the pool and is shared pro-rata by all suppliers. There is no order book, no matching, no maturity: positions are open-ended and can be changed at any block.

### 0.2 Over-collateralization
Because borrowers are anonymous and cannot be sued, every loan must be backed by *more* collateral than debt, valued by an oracle. Two ratios matter, both set per asset by governance:

| Parameter | Meaning | Where used |
|---|---|---|
| **LTV** (loan-to-value) | Max debt you may *open* against 1 unit of collateral value. 80% LTV → $100 of ETH lets you borrow up to $80. | `validateHFAndLtv` on borrow |
| **Liquidation threshold (LT)** | Debt/collateral ratio at which the position *becomes liquidatable*. Always ≥ LTV, e.g. 82.5%. | `calculateUserAccountData` → health factor |
| **Liquidation bonus (LB)** | Discount the liquidator gets on collateral, e.g. 105% = 5% bonus. | `_calculateAvailableCollateralToLiquidate` |

The gap between LTV and LT is a safety buffer: you cannot borrow right up to the point where you are instantly liquidatable.

### 0.3 Health factor
```
HF = Σ(collateral_i_in_USD × LT_i) / Σ(debt_j_in_USD)
```
HF ≥ 1 → healthy. HF < 1 → anyone may liquidate. Aave stores it as an 18-decimal number; `1e18` is the boundary (`ValidationLogic.HEALTH_FACTOR_LIQUIDATION_THRESHOLD`, `src/contracts/protocol/libraries/logic/ValidationLogic.sol:32`).

### 0.4 Utilization-driven interest rates
There is no negotiation. The borrow rate is a *function of utilization* `U = totalDebt / (availableLiquidity + totalDebt)`. Low U → cheap borrowing to attract borrowers; U above an "optimal" kink → the rate rises steeply to attract suppliers and push borrowers to repay, keeping liquidity available for withdrawals. The supply rate is the borrow rate × U × (1 − reserve factor). The reserve factor is the protocol's cut.

### 0.5 Liquidation as the enforcement mechanism
Nobody can force a borrower to repay. Instead, when HF < 1, a third party (liquidator) repays some of the borrower's debt and receives the borrower's collateral at a discount (the bonus). This is what keeps suppliers whole: collateral is sold *before* it is worth less than the debt.

### 0.6 Bad debt
If collateral crashes faster than liquidators act, a position can end with zero collateral and non-zero debt. That debt will never be repaid. Since v3.3 Aave *recognises* it immediately as a reserve **deficit** (`ReserveData.deficit`), stops it accruing interest by burning the debt tokens, and lets a safety module (Umbrella) burn aTokens to cover it (`eliminateReserveDeficit`).

### 0.7 Worked example (numbers reused in §7)
Assume ETH: LTV 80%, LT 82.5%, LB 105%, protocol liquidation fee 10% of the bonus. USDC: price $1.

1. Alice supplies **10 ETH @ $2,000** = $20,000 collateral. She may borrow up to $16,000.
2. She borrows **15,000 USDC**. HF = 20,000 × 0.825 / 15,000 = **1.10**.
3. ETH drops to **$1,780**. Collateral = $17,800. HF = 17,800 × 0.825 / 15,000 = **0.979 < 1** → liquidatable.
4. Because HF is still above 0.95 and the position is above $2,000, only **50%** of her debt can be closed in one call (§3.8). Bob repays 7,500 USDC and receives 7,500 / 1,780 × 1.05 = **4.4242 ETH**; 10% of the 0.2107 ETH bonus (0.0211 ETH) goes to the treasury, so Bob nets 4.4031 ETH.
5. Alice is left with 5.5758 ETH ($9,925) and $7,500 debt → HF = **1.09**. She survived, but paid ~$210 in bonus.

Everything below is how the code implements exactly this.

---

## 1. Architecture & contract map

### 1.1 The cast

| Contract | Role | File |
|---|---|---|
| `PoolAddressesProvider` | Registry of the market's addresses **and** admin of the proxies for Pool/Configurator. Owned by governance. | `src/contracts/protocol/configuration/PoolAddressesProvider.sol` |
| `Pool` (behind a proxy) | The *only* user entry point: supply/withdraw/borrow/repay/liquidate/flashLoan. Holds all reserve + user state. Delegates all logic to libraries. | `src/contracts/protocol/pool/Pool.sol` |
| `PoolStorage` | Storage layout inherited by `Pool`. | `src/contracts/protocol/pool/PoolStorage.sol` |
| `PoolConfigurator` (proxy) | Admin API. The only caller of Pool's `onlyPoolConfigurator` functions. | `src/contracts/protocol/pool/PoolConfigurator.sol` |
| `ACLManager` | OpenZeppelin `AccessControl` role registry: POOL_ADMIN, RISK_ADMIN, EMERGENCY_ADMIN, ASSET_LISTING_ADMIN, FLASH_BORROWER, BRIDGE. | `src/contracts/protocol/configuration/ACLManager.sol` |
| `AaveOracle` | Price source per asset (Chainlink `latestAnswer`) in a *base currency* (USD, 8 decimals on mainnet). | `src/contracts/misc/AaveOracle.sol` |
| `DefaultReserveInterestRateStrategyV2` | One immutable rate contract shared by all reserves (since 3.4). Stores kink params per reserve. | `src/contracts/misc/DefaultReserveInterestRateStrategyV2.sol` |
| `AToken` (one proxy per reserve) | Interest-bearing receipt. **Also the vault**: the underlying ERC20 sits in the aToken contract, not in the Pool. | `src/contracts/protocol/tokenization/AToken.sol` |
| `VariableDebtToken` (one proxy per reserve) | Non-transferable debt receipt. | `src/contracts/protocol/tokenization/VariableDebtToken.sol` |
| `RewardsController` / `EmissionManager` | Liquidity-mining accounting, driven by `handleAction` hooks from the tokens. | `src/contracts/rewards/` |
| Treasury (`Collector`) | Receives reserve-factor cut and liquidation fees as aTokens. Address is an immutable on each aToken (`AToken.sol:32`). | `src/contracts/treasury/` |
| Logic libraries | `SupplyLogic`, `BorrowLogic`, `LiquidationLogic`, `FlashLoanLogic`, `PoolLogic` are **externally linked libraries** (deployed once, called via `DELEGATECALL`). `ReserveLogic`, `ValidationLogic`, `GenericLogic`, `ConfiguratorLogic` are internal (inlined). | `src/contracts/protocol/libraries/logic/` |

### 1.2 Who calls whom

```
                         governance (owner)
                                │
                    ┌───────────▼─────────────┐
                    │  PoolAddressesProvider  │  getPool(), getPriceOracle(), getACLManager()...
                    │  (creates + admins the  │  setAddress('UMBRELLA', ...)
                    │   Pool & Configurator   │
                    │   proxies)              │
                    └───┬───────────┬─────────┘
                        │           │
   ACL roles ──► ACLManager   AaveOracle ◄── Chainlink aggregators
                        │           ▲
        admins ─► PoolConfigurator  │ getAssetPrice()
                        │ onlyPoolConfigurator
                        ▼
   users ───────────► Pool (proxy → PoolInstance impl)
                        │  DELEGATECALL into linked libs:
                        │  SupplyLogic / BorrowLogic / LiquidationLogic / FlashLoanLogic / PoolLogic
                        │  (which inline ReserveLogic / ValidationLogic / GenericLogic)
                        │
      ┌─────────────────┼──────────────────────────┬──────────────────────────┐
      ▼                 ▼                          ▼                          ▼
  AToken (per asset) VariableDebtToken (per asset) InterestRateStrategyV2   RewardsController
  holds underlying   mint/burn onlyPool            calculateInterestRates()  handleAction() from tokens
  mint/burn onlyPool
  transfer → Pool.finalizeTransfer() (callback for HF check)
```

The `Pool` is the *state* and the *router*. Tokens are *dumb*: they only mint/burn when the Pool tells them to (`onlyPool`, `IncentivizedERC20.sol:45-48`) and they hold the actual ERC20 balances.

### 1.3 Why external libraries, and how linking works
`Pool.sol` imports the logic libraries and calls e.g. `SupplyLogic.executeSupply(_reserves, _eModeCategories, _usersConfig[onBehalfOf], params)` (`Pool.sol:124-137`). `executeSupply` is declared `external` in a `library` (`SupplyLogic.sol:40-45`). Solidity compiles a call to an *external library function* as a `DELEGATECALL` to the library's deployed address, passing storage pointers (`mapping(...) storage`) as slot references. The Pool's bytecode contains a placeholder `__$...$__` that the deployer fills in with the library address (Foundry `--libraries`).

Reason: the EIP-170 24 KB contract size limit. The Pool alone cannot hold all this logic. Splitting into libraries keeps the Pool small, and each library can be upgraded by redeploying it and relinking the Pool implementation. The Pool exposes the linked addresses via `getSupplyLogic()` etc. (`Pool.sol:918-940`).

Storage is *not* duplicated: because it is `DELEGATECALL`, the library code runs in the Pool's storage context. That is why every `execute*` takes the storage mappings as parameters.

### 1.4 Storage: `PoolStorage`

`src/contracts/protocol/pool/PoolStorage.sol:21-56`
```solidity
mapping(address => DataTypes.ReserveData) internal _reserves;            // asset => reserve state
mapping(address => DataTypes.UserConfigurationMap) internal _usersConfig; // user => 2-bit-per-reserve bitmap
mapping(uint256 => address) internal _reservesList;                       // reserveId => asset
mapping(uint8 => DataTypes.EModeCategory) internal _eModeCategories;      // eModeId => config
mapping(address => uint8) internal _usersEModeCategory;                   // user => eModeId (0 = none)
uint256 internal __DEPRECATED_bridgeProtocolFee;
uint128 internal _flashLoanPremium;                                       // bps, all of it → treasury since 3.4
uint128 internal __DEPRECATED_flashLoanPremiumToProtocol;
uint64  internal __DEPRECATED_maxStableRateBorrowSizePercent;
uint16  internal _reservesCount;
mapping(address user => mapping(address permittedPositionManager => bool)) internal _positionManager;
```
Note the deprecated slots are kept to preserve the layout across proxy upgrades.

### 1.5 `DataTypes.ReserveData` field by field (`src/contracts/protocol/libraries/types/DataTypes.sol:42-79`)

| Field | Type | Meaning |
|---|---|---|
| `configuration` | `ReserveConfigurationMap` | The 256-bit packed config (§1.6). |
| `liquidityIndex` | uint128 ray | Cumulative supply-side interest multiplier since listing. Starts at `1e27`. `aToken.balanceOf = scaled × liquidityIndex`. |
| `currentLiquidityRate` | uint128 ray | Current APR paid to suppliers, set by the rate strategy at the last interaction. |
| `variableBorrowIndex` | uint128 ray | Cumulative borrow-side multiplier. `vToken.balanceOf = scaled × variableBorrowIndex`. |
| `currentVariableBorrowRate` | uint128 ray | Current borrow APR. |
| `deficit` | uint128 | Bad debt in underlying units recognised but not yet covered (3.3+). Reuses the old stable-rate slot. |
| `lastUpdateTimestamp` | uint40 | When indexes were last accrued. |
| `id` | uint16 | Index in `_reservesList`; also the bit position in user bitmaps. Max 128 reserves (`ReserveConfiguration.MAX_RESERVES_COUNT`, `ReserveConfiguration.sol:64`). |
| `liquidationGracePeriodUntil` | uint40 | Liquidations blocked until this timestamp (set on un-pause, ≤ 4h). |
| `aTokenAddress`, `variableDebtTokenAddress` | address | Per-reserve token proxies. |
| `__deprecatedStableDebtTokenAddress`, `__deprecatedInterestRateStrategyAddress` | | Kept for layout. Strategy is now `Pool.RESERVE_INTEREST_RATE_STRATEGY` (immutable, `Pool.sol:43`). |
| `accruedToTreasury` | uint128 *scaled* | Reserve-factor income owed to treasury, not yet minted as aTokens. |
| `virtualUnderlyingBalance` | uint128 | The protocol's *own* count of underlying it believes is in the aToken (3.1+). Used for utilization instead of `balanceOf` so donations/rebases cannot move rates. |
| `__deprecatedIsolationModeTotalDebt`, `__deprecatedVirtualUnderlyingBalance` | | Layout padding. |

`ReserveDataLegacy` (`DataTypes.sol:9-40`) is what the public `getReserveData()` returns for backwards compatibility (`Pool.sol:438-460`); note it hard-codes `unbacked = 0`, `isolationModeTotalDebt = 0` and returns a `MOCK_STABLE_DEBT` address.

### 1.6 `ReserveConfigurationMap` bit layout

One `uint256` per reserve. Masks and bit positions in `src/contracts/protocol/libraries/configuration/ReserveConfiguration.sol:13-53`. Comment map in `DataTypes.sol:82-102`.

```
bit   0-15   LTV                       (bps, 8000 = 80%)          LTV_MASK
bit  16-31   Liquidation threshold     (bps)                      LIQUIDATION_THRESHOLD_MASK
bit  32-47   Liquidation bonus         (bps, 10500 = +5%)         LIQUIDATION_BONUS_MASK
bit  48-55   Decimals of underlying                               DECIMALS_MASK
bit  56      active                                               ACTIVE_MASK
bit  57      frozen   (no new supply/borrow; repay/withdraw ok)   FROZEN_MASK
bit  58      borrowing enabled                                    BORROWING_MASK
bit  59      [hole] pre-3.2 stable rate enabled
bit  60      paused   (nothing works, incl. transfers)            PAUSED_MASK
bit  61-62   [hole] pre-3.7 borrowableInIsolation / siloedBorrowing
bit  63      flashloan enabled                                    FLASHLOAN_ENABLED_MASK
bit  64-79   reserve factor            (bps)                      RESERVE_FACTOR_MASK
bit  80-115  borrow cap (whole tokens, 0 = none)                  BORROW_CAP_MASK
bit 116-151  supply cap (whole tokens, 0 = none)                  SUPPLY_CAP_MASK
bit 152-167  liquidation protocol fee  (bps of the bonus)         LIQUIDATION_PROTOCOL_FEE_MASK
bit 168-175  [hole] pre-3.2 eMode category
bit 176-211  [hole] pre-3.4 unbacked mint cap
bit 212-251  [hole] pre-3.7 debt ceiling
bit 252      virtual accounting active (always 1 since 3.4)       VIRTUAL_ACC_ACTIVE_MASK
bit 253-255  unused
```
Accessors are pure bit ops on a `memory` copy, e.g. `getLtv` (`ReserveConfiguration.sol:82-84`) `return self.data & LTV_MASK;` and `setLtv` (`:71-75`) `self.data = (self.data & ~LTV_MASK) | ltv;`. Since 3.3 masks are "read-optimised" (the mask selects the field rather than clearing it). `getFlags` (`:403-414`) returns (active, frozen, borrowingEnabled, paused) in one read; `getParams` (`:425-437`) returns (ltv, lt, lb, decimals, reserveFactor).

### 1.7 `UserConfigurationMap`: 2 bits per reserve (`UserConfiguration.sol`)

```
bit 2·id     = user is BORROWING reserve id
bit 2·id + 1 = user is USING reserve id AS COLLATERAL
```
`BORROWING_MASK = 0x5555…` (every even bit), `COLLATERAL_MASK = 0xAAAA…` (every odd bit) (`UserConfiguration.sol:17-20`).
- `setBorrowing` / `setUsingAsCollateral` (`:28-72`) flip one bit; the collateral setter also emits `ReserveUsedAsCollateralEnabled/Disabled`.
- `isBorrowingAny`, `isUsingAsCollateralAny`, `isEmpty` (`:124-157`) are one `AND`.
- `isUsingAsCollateralOne` / `isBorrowingOne` (`:112-139`) use the power-of-two trick `n & (n-1) == 0`.
- `getNextFlags(data)` (`:168-172`) returns `(data >> 2, data&1, data&2)`, which lets loops shift through the bitmap and **exit early when it hits zero**. That is why `calculateUserAccountData` and `_burnBadDebt` iterate `while (cached != 0)`.

Consequence: a user's whole position is enumerable in O(#reserves used), and "does this user borrow anything?" is O(1).

### 1.8 eMode configuration (`DataTypes.sol:141-151`, `EModeConfiguration.sol`)
```solidity
struct EModeCategory {
  uint16 ltv; uint16 liquidationThreshold; uint16 liquidationBonus;
  uint128 collateralBitmap;   // bit id = asset gets eMode LTV/LT/LB when used as collateral
  bool isolated;              // 3.7: assets NOT in collateralBitmap get LTV 0
  string label;
  uint128 borrowableBitmap;   // bit id = asset may be borrowed while in this eMode
  uint128 ltvzeroBitmap;      // 3.6: asset in collateralBitmap but forced LTV 0 (ltv0 rules)
}
```
`EModeConfiguration.isReserveEnabledOnBitmap(bitmap, id)` is `(bitmap >> id) & 1` (`EModeConfiguration.sol:44-52`). Category `0` means "no eMode" and can never be configured (`Pool.sol:657`).

---

## 2. Interest math (the heart)

### 2.1 Fixed-point units
- **wad** = 1e18, **ray** = 1e27 (`WadRayMath.sol:20-26`). Indexes and rates are rays; token amounts are in the asset's own decimals.
- **PercentageMath**: `PERCENTAGE_FACTOR = 1e4`, so `8000` = 80.00% (`PercentageMath.sol:13`).
- Default `rayMul`/`rayDiv`/`percentMul`/`percentDiv` round **half-up** (`WadRayMath.sol:64-72,104-112`). Since 3.5 there are explicit `Floor`/`Ceil` variants (`rayMulFloor`, `rayMulCeil`, `rayDivFloor`, `rayDivCeil`, `percentMulFloor/Ceil`, `percentDivFloor/Ceil`) and every balance-affecting path uses a deliberate direction (§2.4).

### 2.2 Linear vs compounded interest (`MathUtils.sol`)
`calculateLinearInterest(rate, lastUpdate)` (`:23-34`):
```
1 + rate × Δt / SECONDS_PER_YEAR         (all in ray)
```
`calculateCompoundedInterest(rate, lastUpdate, now)` (`:50-85`) wants `(1+r/n)^n ≈ e^(rate·Δt/year)` but exponentiation is expensive, so it uses the **3-term binomial/Taylor approximation**:
```
x = rate × Δt / SECONDS_PER_YEAR
(1+x)^n ≈ 1 + x + x²/2 + x³/6
code:  RAY + x + x.rayMul(x/2 + x.rayMul(x/6))
```
The comment at `:40-44` explains the choice: slightly *under*-charges borrowers and *under*-pays suppliers versus true `e^x`, but is cheap and can never overflow for realistic inputs (`unchecked`, `:79-84`). Numerically, at 3.5556% for one year the code gives `1.0361951` vs true `e^x = 1.0361952`: an error of 7×10⁻⁸.

**Why suppliers get linear and borrowers compounded.** Each borrower's debt compounds continuously. The supply side is only credited with what borrowers actually owe, so paying suppliers *linear* interest on the rate computed at the last interaction guarantees `Σ aToken balances ≤ Σ debt + cash` between interactions: the protocol never promises suppliers more than has accrued on the debt side. Whenever anyone interacts, both indexes are refreshed and the rate is recomputed, so in practice supply interest is piecewise-linear with frequent re-anchoring, which is effectively compound.

### 2.3 Index accrual: `ReserveLogic.updateState`

Every state-changing action starts with the same three lines (see `SupplyLogic.sol:46-49`):
```solidity
DataTypes.ReserveData storage reserve = reservesData[params.asset];
DataTypes.ReserveCache memory reserveCache = reserve.cache();   // snapshot storage → memory
reserve.updateState(reserveCache);                              // accrue interest to "now"
```
`cache()` (`ReserveLogic.sol:251-274`) copies config, both indexes (as `curr` and `next`), both rates, token addresses, timestamp and `scaledTotalSupply()` of the debt token into a `ReserveCache` struct (`DataTypes.sol:159-173`) to avoid repeated SLOADs.

`updateState` (`ReserveLogic.sol:85-101`):
1. If `lastUpdateTimestamp == block.timestamp` → return (already accrued this block).
2. `_updateIndexes` (`:211-243`):
   - if `currLiquidityRate != 0`: `nextLiquidityIndex = linearInterest(rate, ts).rayMul(currLiquidityIndex)` and store.
   - if `currScaledVariableDebt != 0`: `nextVariableBorrowIndex = compoundedInterest(rate, ts).rayMul(currVariableBorrowIndex)` and store. (Guarded on *debt*, not rate, because a base rate can be non-zero with no borrowers; `:229-232`.)
3. `_accrueToTreasury` (`:183-204`):
   ```solidity
   totalDebtAccrued = currScaledVariableDebt.rayMulFloor(nextVariableBorrowIndex - currVariableBorrowIndex);
   amountToMint     = totalDebtAccrued.percentMul(reserveFactor);
   reserve.accruedToTreasury += amountToMint.getATokenMintScaledAmount(nextLiquidityIndex);
   ```
   i.e. "interest that accrued on all debt since last update × reserve factor", stored **scaled** by the new liquidity index so it keeps earning like any other aToken. It is materialised later by `mintToTreasury` (§3.11).
4. Write `lastUpdateTimestamp = now`.

`getNormalizedIncome` / `getNormalizedDebt` (`ReserveLogic.sol:39-78`) compute the *same* `next` index as a **view**, which is what `aToken.balanceOf` uses (§4.1) so balances are live without any transaction.

### 2.4 Scaled balances and rounding (`TokenMath.sol`)

Tokens store a **scaled** balance `s`. The visible balance is `s × index`. Minting `amount` at index `i` stores `amount / i`. Because the index only grows, everyone's visible balance grows without any storage write. This is the entire trick behind "rebasing" aTokens.

3.5 made every conversion direction explicit, always **in favour of the protocol** (like ERC-4626):

| Operation | Function (`TokenMath.sol`) | Rounding | Why |
|---|---|---|---|
| Supply → mint aToken | `getATokenMintScaledAmount` (`:24-29`) | `rayDivFloor` | User gets ≤ what they paid. |
| Withdraw → burn aToken | `getATokenBurnScaledAmount` (`:38-43`) | `rayDivCeil` | Burn ≥ the amount released. |
| aToken transfer | `getATokenTransferScaledAmount` (`:52-57`) | `rayDivCeil` | Recipient gets ≥ requested. |
| aToken `balanceOf` | `getATokenBalance` (`:66-71`) | `rayMulFloor` | Never over-report claims. |
| Borrow → mint vToken | `getVTokenMintScaledAmount` (`:80-85`) | `rayDivCeil` | Never under-record debt. |
| Repay → burn vToken | `getVTokenBurnScaledAmount` (`:94-99`) | `rayDivFloor` | Never over-forgive. |
| vToken `balanceOf` | `getVTokenBalance` (`:108-113`) | `rayMulCeil` | Never under-report debt. |

Note the asymmetry: the same `rayDivCeil` that is "bad" for a withdrawer is "good" for the protocol; the naming in `TokenMath` encodes intent, not just arithmetic.

`ScaledBalanceTokenBase._mintScaled` (`ScaledBalanceTokenBase.sol:69-92`) and `_burnScaled` (`:105-134`) now take the already-scaled amount and a *function pointer* `getTokenBalance` (`TokenMath.getATokenBalance` or `getVTokenBalance`) so a single implementation serves both tokens with the right rounding. They also update `_userState[user].additionalData = index` — the user's "last index" — and emit `Mint`/`Burn` with the interest accrued since the user's last interaction (`balanceIncrease`), which is how the UI shows "interest earned".

### 2.5 Numeric example: indexes over one year
Take the rates from §2.6 at 80% utilization: borrow 3.5556%, supply 2.56%.
- `liquidityIndex`: 1.0 → 1.0 × (1 + 0.0256) = **1.0256**.
- `variableBorrowIndex`: 1.0 → binomial(0.035556) = **1.0361951**.
- Alice supplied 10 ETH at index 1.0 → `scaled = 10e18`. After a year `balanceOf = rayMulFloor(10e18, 1.0256e27) = 10.256 ETH`.
- Bob borrowed 5,000 USDC at index 1.0 → `scaled = 5000e6`. After a year `balanceOf = rayMulCeil(5000e6, 1.0361951e27) = 5,180.98 USDC`.
No storage on either token changed during that year.

### 2.6 Rate model: `DefaultReserveInterestRateStrategyV2.calculateInterestRates`

Called from `ReserveLogic.updateInterestRatesAndVirtualBalance` (`ReserveLogic.sol:130-175`) at the *end* of every action, after balances changed:
```solidity
uint256 totalVariableDebt = reserveCache.nextScaledVariableDebt.getVTokenBalance(reserveCache.nextVariableBorrowIndex);
(nextLiquidityRate, nextVariableRate) = IReserveInterestRateStrategy(strategy).calculateInterestRates(
  CalculateInterestRatesParams({ unbacked: reserve.deficit, liquidityAdded, liquidityTaken,
    totalDebt: totalVariableDebt, reserveFactor, reserve, usingVirtualBalance: true,
    virtualUnderlyingBalance: reserve.virtualUnderlyingBalance }));
reserve.currentLiquidityRate = ...; reserve.currentVariableBorrowRate = ...;
if (liquidityAdded > 0)  reserve.virtualUnderlyingBalance += liquidityAdded;
if (liquidityTaken > 0)  reserve.virtualUnderlyingBalance -= liquidityTaken;
```
Params per reserve (stored in bps in the strategy, `IDefaultInterestRateStrategyV2.sol:24-29`): `optimalUsageRatio`, `baseVariableBorrowRate`, `variableRateSlope1`, `variableRateSlope2`. Setter guards (`DefaultReserveInterestRateStrategyV2.sol:177-208`): optimal in [1%, 99%], slope1 ≤ slope2, base+slope1+slope2 ≤ 1000%.

`calculateInterestRates` (`:124-170`):
```
if totalDebt == 0: return (0, base)
availableLiquidity = virtualUnderlyingBalance + liquidityAdded - liquidityTaken
U      = totalDebt / (availableLiquidity + totalDebt)                  // borrowUsageRatio
U_s    = totalDebt / (availableLiquidity + totalDebt + deficit)        // supplyUsageRatio (deficit dilutes suppliers)
if U > optimal:
    excess     = (U - optimal) / (1 - optimal)
    borrowRate = base + slope1 + slope2 × excess
else:
    borrowRate = base + slope1 × U / optimal
liquidityRate  = borrowRate × U_s × (1 - reserveFactor)
```
Worked numbers, USDC-like params: optimal 90%, base 0, slope1 4%, slope2 60%, reserve factor 10%:

| Utilization | Borrow APR | Supply APR |
|---|---|---|
| 50% | 2.22% | 1.00% |
| 80% | 3.56% | 2.56% |
| 90% (kink) | 4.00% | 3.24% |
| 95% | 34.00% | 29.07% |
| 100% | 64.00% | 57.60% |

Past the kink the rate explodes on purpose: at 100% utilization no one can withdraw, so the protocol makes borrowing punishingly expensive and supplying very attractive until U falls back.

Note `liquidityAdded/liquidityTaken` are passed *before* `virtualUnderlyingBalance` is updated so the strategy sees post-action liquidity; the virtual balance is then adjusted on lines 160-165.

---

## 3. User actions, traced Pool → logic library → tokens

Every action follows the same skeleton (documented in `FlashLoanLogic.sol:64-66`): **cache → updateState → validate → change state → updateInterestRates**. Flash loans deliberately reorder it (§3.9).

### 3.1 `supply(asset, amount, onBehalfOf, referralCode)`

`Pool.supply` (`Pool.sol:118-138`) builds `ExecuteSupplyParams` and calls `SupplyLogic.executeSupply` (`SupplyLogic.sol:40-92`):

```
Pool.supply
 └─ SupplyLogic.executeSupply(_reserves, _eModeCategories, _usersConfig[onBehalfOf], params)
     ├─ reserve.cache(); reserve.updateState(cache)                       // accrue indexes           :46-49
     ├─ scaledAmount = amount.getATokenMintScaledAmount(nextLiquidityIndex) // floor                  :50
     ├─ ValidationLogic.validateSupply(cache, reserve, scaledAmount, onBehalfOf)                     :52
     │    • scaledAmount != 0; active; !paused; !frozen; onBehalfOf != aToken         (ValidationLogic.sol:45-51)
     │    • supply cap: (scaledTotalSupply + scaledAmount + accruedToTreasury) × index ≤ cap × 10^dec (:53-63)
     ├─ reserve.updateInterestRatesAndVirtualBalance(cache, asset, liquidityAdded=amount, 0, strategy)  :54-60
     ├─ IERC20(asset).safeTransferFrom(user, aToken, amount)               // underlying goes to the aToken :62
     ├─ isFirstSupply = IAToken(aToken).mint(user, onBehalfOf, scaledAmount, nextLiquidityIndex)     :65-70
     ├─ if isFirstSupply && ValidationLogic.validateUseAsCollateral(...)   // LTV != 0 in user's eMode :72-83
     │      userConfig.setUsingAsCollateral(reserve.id, asset, onBehalfOf, true)
     └─ emit Supply(asset, user, onBehalfOf, amount, referralCode)                                    :85-91
```
Details worth noticing:
- The supply cap check is done in *scaled* terms then scaled up once (3.5 change) to avoid double rounding.
- `isFirstSupply` is `scaledBalance == 0` before the mint (`ScaledBalanceTokenBase.sol:91`). Only a *first* supply auto-enables collateral; subsequent supplies leave the flag alone.
- `validateUseAsCollateral` (`ValidationLogic.sol:509-516`) is simply `getUserReserveLtv(...) != 0`: an asset with LTV 0 (frozen, or ltv0 in the user's eMode, or outside an `isolated` eMode's bitmap) is **not** auto-enabled. This is the "ltv0 rule".
- `supplyWithPermit` (`Pool.sol:141-176`) wraps a `try permit() {} catch {}` around the same call; the try/catch prevents griefing by front-running the permit.
- `deposit` (`Pool.sol:799-819`) is the legacy alias.

### 3.2 `withdraw(asset, amount, to)` → `SupplyLogic.executeWithdraw` (`SupplyLogic.sol:106-176`)

```
├─ cache, require(to != aToken), updateState                                                    :113-118
├─ scaledUserBalance = aToken.scaledBalanceOf(user)                                              :120
├─ if amount == uint256.max: withdraw everything (scaled = all, amount = scaled×index floor)     :124-127
│  else:                     scaled = amount.getATokenBurnScaledAmount(index)  // ceil            :129-133
├─ validateWithdraw: scaled != 0, scaled ≤ scaledUserBalance, active, !paused   (ValidationLogic.sol:72-83)
├─ updateInterestRatesAndVirtualBalance(liquidityTaken = amountToWithdraw)                       :138-144
├─ zeroBalanceAfterBurn = aToken.burn(from=user, receiverOfUnderlying=to, amount, scaled, index) :147-153
│      (AToken.burn burns scaled then safeTransfer(underlying, to, amount))
├─ if userConfig.isUsingAsCollateral(reserve.id):                                                :155-171
│     if zeroBalanceAfterBurn: setUsingAsCollateral(false)
│     if userConfig.isBorrowingAny(): ValidationLogic.validateHFAndLtvzero(...)  // HF ≥ 1 after
└─ emit Withdraw; return amountToWithdraw
```
The health check runs only if the asset *was* collateral and the user *has* debt: withdrawing non-collateral or having no debt can never make you liquidatable. `validateHFAndLtvzero` (`ValidationLogic.sol:398-430`) additionally enforces: if the user holds any ltv0 collateral, the asset being withdrawn must *be* an ltv0 asset ("withdraw ltv0 collateral first", so you cannot leave a position backed only by LTV-0 collateral).

### 3.3 `borrow(asset, amount, interestRateMode, referralCode, onBehalfOf)` → `BorrowLogic.executeBorrow` (`BorrowLogic.sol:41-114`)

```
├─ cache, updateState                                                                            :48-51
├─ amountScaled = amount.getVTokenMintScaledAmount(nextVariableBorrowIndex)   // ceil            :53-55
├─ ValidationLogic.validateBorrow(reservesData, eModeCategories, {cache, asset, amountScaled, mode, userEMode})
│    (ValidationLogic.sol:103-157)
│    • amountScaled != 0; active; !paused; !frozen
│    • if user in eMode: asset must be in eModeCategory.borrowableBitmap   (else: reserve.borrowingEnabled)
│    • aToken.totalSupply() ≥ amount        // "cannot borrow more than exists" soft inflation guard
│    • mode == VARIABLE                      // stable removed in 3.2
│    • borrow cap: (currScaledDebt + amountScaled) × index ≤ cap × 10^dec
├─ nextScaledVariableDebt = vToken.mint(user, onBehalfOf, amount, amountScaled, index)           :69-76
│      (if user != onBehalfOf → consumes credit delegation allowance, §4.2)
├─ if !userConfig.isBorrowing(id): setBorrowing(id, true)                                        :78-81
├─ updateInterestRatesAndVirtualBalance(liquidityTaken = releaseUnderlying ? amount : 0)         :83-89
├─ if releaseUnderlying: aToken.transferUnderlyingTo(user, amount)                               :91-93
├─ ValidationLogic.validateHFAndLtv(..., onBehalfOf, userEMode, oracle)    // AFTER the mint (3.5) :95-103
│    (ValidationLogic.sol:346-385)
│    • currentLtv != 0                                   → LtvValidationFailed
│    • healthFactor ≥ 1e18                               → HealthFactorLowerThanLiquidationThreshold
│    • collateral ≥ debt.percentDivCeil(avgLtv)          → CollateralCannotCoverNewBorrow
└─ emit Borrow(asset, user, onBehalfOf, amount, VARIABLE, currentVariableBorrowRate, referral)
```
Two separate checks after the borrow: the **HF** check (LT-based) and the **LTV** check (`collateral × LTV ≥ debt`). The LTV check is the stricter one and is what actually limits new borrowing; the HF check is a belt-and-braces guard. `releaseUnderlying = false` is used by flash loans that are converted into debt (§3.9).

### 3.4 `repay` / `repayWithPermit` / `repayWithATokens` → `BorrowLogic.executeRepay` (`BorrowLogic.sol:126-223`)

```
├─ cache, updateState                                                                            :133-135
├─ userDebtScaled = vToken.scaledBalanceOf(onBehalfOf); userDebt = scaled × index (ceil)         :137-139
├─ validateRepay (ValidationLogic.sol:167-190):
│    amount != 0; mode == VARIABLE; (amount == max ⇒ user == onBehalfOf); active; !paused; debtScaled != 0
├─ paybackAmount = amount
│  if useATokens && amount == max: paybackAmount = user's full aToken balance                    :151-156
│  if paybackAmount > userDebt: paybackAmount = userDebt          // cap at debt                 :158-160
├─ (noMoreDebt, nextScaledDebt) = vToken.burn(onBehalfOf, paybackAmount.getVTokenBurnScaledAmount(index), index) :162-169
├─ updateInterestRatesAndVirtualBalance(liquidityAdded = useATokens ? 0 : paybackAmount)         :171-177
├─ if noMoreDebt: setBorrowing(id, false)                                                        :179-181
├─ if useATokens:                                                                                :184-209
│     aToken.burn(from=user, receiverOfUnderlying=aToken, paybackAmount, scaledBurn(ceil), index) // burns aTokens, underlying stays put
│     if collateral flag && zeroBalanceAfterBurn: clear flag
│     if isBorrowingAny: validateHealthFactor(user)      // 3.5: cannot self-repay into liquidation
│  else:
│     IERC20(asset).safeTransferFrom(user, aToken, paybackAmount)                                :211
└─ emit Repay(asset, onBehalfOf, user, paybackAmount, useATokens)
```
`repayWithATokens` (`Pool.sol:304-327`) forces `onBehalfOf = msg.sender`: it burns your aUSDC and your vUSDC simultaneously, no ERC20 transfer needed, no virtual-balance change (`liquidityAdded = 0` because the underlying never left the aToken).

### 3.5 `setUserUseReserveAsCollateral(asset, useAsCollateral)` → `SupplyLogic.executeUseReserveAsCollateral` (`SupplyLogic.sol:240-289`)
- Enable: require `scaledBalanceOf != 0` and `validateUseAsCollateral` (LTV ≠ 0 in your eMode; `UserHasAssetWithZeroLtv` otherwise), then set the bit.
- Disable: clear the bit, then `validateHFAndLtvzero` so you cannot drop collateral you need.
- Noop if already in the requested state (`:256`).
- `setUserUseReserveAsCollateralOnBehalfOf` (`Pool.sol:859-875`) is the same, gated by `onlyPositionManager(onBehalfOf)` (3.4 position managers, §3.12).

### 3.6 `GenericLogic.calculateUserAccountData` — the health-factor engine (`GenericLogic.sol:65-183`)

Inputs: the user's bitmap, address, oracle, eMode. Output: `(totalCollateralBase, totalDebtBase, avgLtv, avgLiquidationThreshold, healthFactor, hasZeroLtvCollateral)`.

```solidity
if (params.userConfig.isEmpty()) return (0, 0, 0, 0, type(uint256).max, false);        // :71-73
if (userEMode != 0) { eModeLiqThreshold = cat.liquidationThreshold; eModeCollateralBitmap = cat.collateralBitmap; } // :77-80
while (cachedUserConfig != 0) {                                                            // :86
  (cachedUserConfig, isBorrowed, isEnabledAsCollateral) = getNextFlags(cachedUserConfig);  // :87-88
  if (isEnabledAsCollateral || isBorrowed) {
    asset = reservesList[i]; price = oracle.getAssetPrice(asset); unit = 10**decimals;     // :90-103
    if (isEnabledAsCollateral) {
      bal$ = scaledBalance × normalizedIncome (floor) × price / unit;                      // _getUserBalanceInBaseCurrency :242-257
      totalCollateral += bal$;
      ltv = ValidationLogic.getUserReserveLtv(reserve, eModeCat, userEMode);               // :115-119
      if (ltv == 0) hasZeroLtvCollateral = true; else avgLtv += bal$ × ltv;                // :120-124
      lt  = (in eMode && asset in eModeCollateralBitmap) ? eModeLiqThreshold : reserve LT; // :126-133
      avgLiquidationThreshold += bal$ × lt;                                                // :135-137
    }
    if (isBorrowed) totalDebt += mulDivCeil(scaledDebt × normalizedDebt (ceil), price, unit); // :140-146, :219-230
  }
  ++i;
}
healthFactor = totalDebt == 0 ? max : avgLiquidationThreshold.wadDiv(totalDebt) / 100_00;  // :162-164
avgLtv /= totalCollateral; avgLiquidationThreshold /= totalCollateral;                     // :166-173
```
Points:
- Collateral value is rounded **down**, debt value **up** (`mulDivCeil`, `MathUtils.sol:100-115`), so any non-zero debt is ≥ 1 wei of base currency and cannot vanish from HF (3.5).
- `avgLiquidationThreshold` before division is `Σ bal$ × LT` with 8+4 decimals; `wadDiv` by `totalDebt` (8 dec) then `/1e4` yields an 18-decimal HF (comment `:156-161`).
- `getUserReserveLtv` (`ValidationLogic.sol:524-549`) is the single source of truth for "effective LTV of asset X for user U":
  1. in eMode and asset in `collateralBitmap` → `ltvzeroBitmap` bit ? 0 : eMode LTV;
  2. in eMode and category `isolated` → 0 (3.7);
  3. else reserve LTV.
  Every auto-collateral decision and the `hasZeroLtvCollateral` rule flow from this one function.
- eMode LT/LB apply **only** to assets in the category's `collateralBitmap`; other collateral keeps its normal LT (liquid eModes, 3.2). Since 3.6, an asset can be collateral or borrowable *exclusively* inside an eMode.

`Pool.getUserAccountData` (`Pool.sol:470-498`) → `PoolLogic.executeGetUserAccountData` (`PoolLogic.sol:162-193`) adds `availableBorrowsBase = collateral.percentMulFloor(ltv) - debt` (`GenericLogic.calculateAvailableBorrows`, `:193-206`).

### 3.7 Risk modes and caps — what each is for and where enforced

| Feature | Purpose | Enforced in |
|---|---|---|
| **Supply cap** | Limit protocol exposure to an asset (oracle/liquidity risk). | `validateSupply` (`ValidationLogic.sol:53-63`) |
| **Borrow cap** | Limit total debt in an asset (avoid utilization spikes, bank-run risk). | `validateBorrow` (`:149-156`) |
| **Frozen** | Off-boarding: no new supply/borrow, but withdraw/repay/liquidate still work. Freezing also sets LTV to 0 (moves it to `_pendingLtv`) and flags the asset ltv0 in every eMode it is collateral in (`PoolConfigurator.setReserveFreeze`, `PoolConfigurator.sol:195-227`). | `validateSupply`, `validateBorrow` |
| **Paused** | Emergency: *nothing* works, including aToken transfers (`validateTransfer`, `:436-438`). Un-pausing can set a liquidation grace period ≤ 4h (`PoolConfigurator.sol:245-251`). | every validate* |
| **eMode** (`setUserEMode`) | Higher LTV/LT for correlated assets (stETH/ETH, stables). User opts in per account. `SupplyLogic.executeSetUserEMode` (`SupplyLogic.sol:304-336`) → `validateSetUserEMode` (`ValidationLogic.sol:448-498`): every borrowed asset must be borrowable in the target category, every collateral must keep LTV ≠ 0, then HF re-validated. | validateBorrow (borrowable bitmap), calculateUserAccountData (LT), liquidation (LB) |
| **ltv0 bitmap** (3.6) | Per-eMode "this asset gives no borrowing power" without freezing. | `getUserReserveLtv` |
| **isolated eMode** (3.7) | Replaces old isolation mode: in an isolated category, anything outside `collateralBitmap` has LTV 0, so you cannot borrow against unrelated collateral. Entry is blocked if you hold such collateral enabled. | `getUserReserveLtv` |
| **Liquidation grace period** (3.1) | After an un-pause, block liquidations for ≤ 4 h so users can top up. | `validateLiquidationCall` (`:275-279`) |
| **Isolation mode / debt ceiling / siloed borrowing** | **Removed in 3.7** (`docs/3.7/mode-removal.md`). Old data providers return `0/false`. | — |

### 3.8 `liquidationCall(collateralAsset, debtAsset, borrower, debtToCover, receiveAToken)` → `LiquidationLogic.executeLiquidationCall` (`LiquidationLogic.sol:166-460`)

Constants (`:43-64`):
- `DEFAULT_LIQUIDATION_CLOSE_FACTOR = 50%`
- `CLOSE_FACTOR_HF_THRESHOLD = 0.95e18` — at or below this HF the *whole* debt may be closed
- `MIN_BASE_MAX_CLOSE_FACTOR_THRESHOLD = 2000e8` ($2,000): positions smaller than this may be closed 100% regardless of HF
- `MIN_LEFTOVER_BASE = 1000e8`: you may not leave dust below this on *both* sides

Step by step:
```
1  cache + updateState on debt reserve, THEN on collateral reserve                          :178-183
2  (totalCollateral$, totalDebt$, _, _, healthFactor) = calculateUserAccountData(borrower)   :185-202
3  borrowerCollateralBalance = scaledBalance × liquidityIndex (floor)                         :204-208
   borrowerReserveDebt       = scaledDebt × borrowIndex (ceil)                                :209-211
4  validateLiquidationCall (ValidationLogic.sol:253-292):
     borrower != liquidator (3.4); both reserves active & !paused; no grace period active;
     healthFactor < 1e18; borrower has collateralAsset flagged as collateral; reserve debt != 0
5  liquidationBonus = (borrower in eMode && collateral in that eMode's bitmap) ? eMode LB : reserve LB :226-239
6  prices, units; reserveDebt$ (ceil), reserveCollateral$ (floor)                             :240-256
7  CLOSE FACTOR:                                                                              :258-278
     maxLiquidatableDebt = borrowerReserveDebt                       // default 100%
     if reserveCollateral$ ≥ 2000$ && reserveDebt$ ≥ 2000$ && HF > 0.95:
        totalDefault$ = totalDebt$ × 50%        // NOTE: 50% of the user's TOTAL debt (3.3), not of this reserve
        if reserveDebt$ > totalDefault$: maxLiquidatableDebt = totalDefault$ / debtPrice
   actualDebtToLiquidate = min(debtToCover, maxLiquidatableDebt)                              :280-282
8  (actualCollateral, actualDebt, protocolFee) = _calculateAvailableCollateralToLiquidate(...) :284-297
9  DUST RULE: if some debt AND some collateral remain, both leftovers must be ≥ MIN_LEFTOVER_BASE
   else revert MustNotLeaveDust                                                               :299-325
10 hasNoCollateralLeft: compute scaled collateral consumed (ceil, capped to balance);
   consumed$ == totalCollateral$ ⇒ true. Clear collateral flag if reserve fully consumed or no collateral left  :341-377
11 _burnDebtTokens(...)                                                                        :378-388
12 if receiveAToken: aToken.transferOnLiquidation(borrower → liquidator, actualCollateral, scaledCeil, index) :390-399
   else: _burnCollateralATokens → updateInterestRates(liquidityTaken) + aToken.burn(borrower, to=liquidator) :400-411
13 protocol fee: aToken.transferOnLiquidation(borrower → TREASURY, fee, scaledCeil)             :413-435
14 if hasNoCollateralLeft && borrower still borrows anything: _burnBadDebt(...)                :440-442
15 IERC20(debtAsset).safeTransferFrom(liquidator, debtAToken, actualDebtToLiquidate)           :445-449
16 emit LiquidationCall
```

`_calculateAvailableCollateralToLiquidate` (`:583-625`), all rounding explicit since 3.7:
```
baseCollateral  = debtPrice × debtToCover × collateralUnit / (collateralPrice × debtUnit)   // collateral worth exactly the debt
maxCollateral   = baseCollateral.percentMulFloor(liquidationBonus)                          // + bonus
if maxCollateral > borrowerCollateralBalance:                                               // not enough collateral
    collateralAmount = borrowerCollateralBalance
    debtAmountNeeded = (collateralValue in debt units).percentDivCeil(bonus)                // scale the debt down
else:
    collateralAmount = maxCollateral; debtAmountNeeded = debtToCover
if protocolFee% != 0:
    bonusCollateral  = collateralAmount - collateralAmount.percentDivFloor(bonus)           // the bonus part only
    protocolFee      = bonusCollateral.percentMulCeil(protocolFee%)
    collateralAmount -= protocolFee
return (collateralAmount, debtAmountNeeded, protocolFee)
```
The protocol fee is a share of the *bonus*, not of the collateral, so the liquidator always keeps ≥ the debt's worth.

`_burnDebtTokens` (`:506-553`):
- burns `hasNoCollateralLeft ? entireReserveDebt : actualDebtToLiquidate` (floor scaled);
- if `hasNoCollateralLeft && outstandingDebt != 0`: `debtReserve.deficit += outstandingDebt`, emit `DeficitCreated` — **bad debt recognised**;
- clears the borrowing bit if no more debt; updates rates with `liquidityAdded = actualDebtToLiquidate`.

`_burnBadDebt` (`:636-678`) loops the borrower's bitmap and calls `_burnDebtTokens` with `hasNoCollateralLeft = true` and `actualDebtToLiquidate = 0` for every *other* borrowed reserve, so the whole position's remaining debt becomes deficit in one liquidation.

`eliminateReserveDeficit` (`Pool.sol:822-837`, `onlyUmbrella`) → `executeEliminateDeficit` (`LiquidationLogic.sol:76-132`): the Umbrella safety module (address registered as `'UMBRELLA'` in the addresses provider, `Pool.sol:46,76`) burns its own aTokens of that asset (`aToken.burn(..., receiverOfUnderlying = aToken)` so no underlying moves) and `reserve.deficit -= amount`. Umbrella must hold no debt. Returns the amount actually covered (3.5).

Numeric walk-through (from §0.7, extended). Threshold $2,000, HF thresholds as coded:

| Step | State | Close factor | Liquidator repays | Seizes | Left |
|---|---|---|---|---|---|
| ETH $1,780 | 10 ETH / 15,000 USDC, HF 0.979 | 50% (HF > 0.95, size > $2k) | 7,500 USDC | 4.4242 ETH (0.0211 to treasury) | 5.5758 ETH, 7,500 USDC, HF 1.09 |
| ETH $1,500 | 5.5758 ETH / 7,500 USDC, HF 0.920 | 100% (HF ≤ 0.95) | 7,500 USDC | 5.25 ETH (0.025 to treasury) | 0.3258 ETH ($489), 0 debt — allowed, since debt is fully cleared |
| Instead ETH $1,400 | 5.5759 ETH / 7,500 USDC, HF 0.859 | 100% | 7,434.53 USDC (all collateral) | 5.5759 ETH | 0 ETH, **65.47 USDC → `deficit`**, vTokens burned |

In the last row `maxCollateral > balance`, so `debtAmountNeeded` is scaled down and `hasNoCollateralLeft` is true, so `_burnDebtTokens` burns the whole 7,500 debt and records 65.47 as deficit.

### 3.9 `flashLoan` / `flashLoanSimple` → `FlashLoanLogic` (`FlashLoanLogic.sol`)

Flash loans invert the order: **validate → send funds → user callback → cache/updateState → pull repayment → updateRates** (`:64-66`). The reason: the callback is untrusted, so indexes/rates are computed only *after* it, and the user's payload cannot see or exploit a half-updated reserve.

`executeFlashLoanSimple` (`:167-207`):
```
validateFlashloanSimple: !paused, active, flashLoanEnabled, aToken.totalSupply() ≥ amount   (ValidationLogic.sol:228-237)
totalPremium = amount.percentMulCeil(flashLoanPremium)                                       :178
reserve.virtualUnderlyingBalance -= amount                                                    :180
aToken.transferUnderlyingTo(receiver, amount)                                                 :182
require(receiver.executeOperation(asset, amount, premium, initiator, params) == true)        :184-193
_handleFlashLoanRepayment:                                                                    :215-252
   cache + updateState
   reserve.accruedToTreasury += premium.getATokenMintScaledAmount(index)   // ALL premium → treasury since 3.4
   updateInterestRatesAndVirtualBalance(liquidityAdded = amount + premium)
   IERC20(asset).safeTransferFrom(receiver, aToken, amount + premium)      // receiver must have approved the Pool
   emit FlashLoan(..., InterestRateMode.NONE, premium, referral)
```
`executeFlashLoan` (`:57-155`) is the multi-asset variant with two extras:
- `isAuthorizedFlashBorrower` (ACL `FLASH_BORROWER_ROLE`) → premium 0 (`:75`).
- Per asset, `interestRateModes[i]`: `NONE` = repay now; `VARIABLE` = **do not repay, open a borrow instead** via `BorrowLogic.executeBorrow` with `releaseUnderlying = false` (`:122-153`). The user already has the tokens, so this is "borrow without transfer" and requires collateral like any borrow. No premium in that path.

`FLASHLOAN_PREMIUM_TO_PROTOCOL()` now always returns `100_00` (`Pool.sol:566-568`); the only tunable is `_flashLoanPremium` (`updateFlashloanPremium`, `PoolConfigurator.sol:490-500`).

Receiver contract: implement `IFlashLoanSimpleReceiver.executeOperation` (`src/contracts/misc/flashloan/interfaces/IFlashLoanSimpleReceiver.sol:25-32`), approve the *Pool*... careful: the pull is `safeTransferFrom(receiver, aToken)` executed by the Pool's delegatecalled library, so the receiver approves the **Pool** address. `FlashLoanSimpleReceiverBase` (`base/FlashLoanSimpleReceiverBase.sol`) just stores `POOL`.

Reentrancy: there is no reentrancy guard on the Pool. Safety comes from ordering (state is not read until after the callback) plus virtual accounting (the flash-borrowed amount is already subtracted from `virtualUnderlyingBalance` before the callback, so the callback cannot borrow/withdraw "phantom" liquidity) plus `flashLoanEnabled` per asset.

### 3.10 aToken transfers → `Pool.finalizeTransfer` → `SupplyLogic.executeFinalizeTransfer` (`SupplyLogic.sol:188-222`)
When `AToken._transfer` runs (§4.1) it calls back `POOL.finalizeTransfer(asset, from, to, scaledAmount, scaledBalanceFromBefore)` (`Pool.sol:576-599`, `require(msg.sender == aToken)`):
- `validateTransfer`: reserve not paused.
- If sender had it as collateral: clear the flag if they sent their whole scaled balance; if they borrow anything, `validateHFAndLtvzero(from)`.
- **The receiver is not auto-enabled as collateral** (3.6 change; saves ~25k gas). They must call `setUserUseReserveAsCollateral` (or use multicall).

### 3.11 Housekeeping
- `mintToTreasury(assets[])` (`Pool.sol:433-435` → `PoolLogic.executeMintToTreasury`, `PoolLogic.sol:108-133`): for each active reserve, take `accruedToTreasury` (scaled), zero it, and `aToken.mintToTreasury(scaled, normalizedIncome)`. Permissionless.
- `rescueTokens` (`Pool.sol:789-795`, `onlyPoolAdmin`): transfer tokens stuck in the Pool contract (the Pool should never hold tokens).
- `syncIndexesState` / `syncRatesState` (`Pool.sol:625-632`, `onlyPoolConfigurator`): accrue indexes / recompute rates with zero flows. The configurator calls them around reserve-factor and rate-parameter changes (`PoolConfigurator.sol:279,287,461,470`) so old parameters apply up to "now" and new ones from "now".
- `getReserveNormalizedIncome` / `getReserveNormalizedVariableDebt` (`Pool.sol:515-526`): view versions of the next index (§2.3).
- `getReservesList` (`Pool.sol:529-548`): still skips `address(0)` gaps left by the removed `dropReserve`.

### 3.12 3.4–3.7 additions present in this tree
- **Multicall** (3.4): `Pool is ... Multicall` (`Pool.sol:38`), OZ's `multicall(bytes[])` → e.g. `[supply, setUserEMode, borrow]` in one tx. (`Pool.sol:4`; the Pool uses `_msgSender()` throughout so a meta-tx context could be added.)
- **Position managers** (3.4): `approvePositionManager(manager, bool)`, `renouncePositionManagerRole(user)`, `isApprovedPositionManager` (`Pool.sol:840-900`), gating `setUserUseReserveAsCollateralOnBehalfOf` and `setUserEModeOnBehalfOf` (`Pool.sol:859-892`).
- **`eliminateReserveDeficit`** (3.3, returns amount since 3.5) and `getReserveDeficit` (`Pool.sol:903-905`).
- **Dedicated getters** `getReserveAToken`, `getReserveVariableDebtToken`, `getVirtualUnderlyingBalance`, `getLiquidationGracePeriod`, eMode bitmap getters, `getIsEModeCategoryIsolated` (3.7) (`Pool.sol:463-467, 705-751, 773-777, 908-915`).
- **`renounceAllowance`** on aTokens and **`renounceDelegation`** on debt tokens (3.6) (`IncentivizedERC20.sol:178-180`, `DebtTokenBase.sol:45-47`).
- **Deterministic liquidation rounding** and scaled-balance-based `hasNoCollateralLeft` (3.7) (`LiquidationLogic.sol:327-377`, `docs/3.7/liquidation-rounding.md`).
- **L2Pool** (`src/contracts/protocol/pool/L2Pool.sol`): same functions taking a single `bytes32 args` (asset id 16 bits | amount 128 bits | referral 16 bits) decoded by `CalldataLogic` (`CalldataLogic.sol:18-32`) to save calldata gas on rollups.

---

## 4. Tokens

Inheritance: `IncentivizedERC20` (plain ERC20 w/ `handleAction` hooks) → `MintableIncentivizedERC20` (`_mint/_burn`) → `ScaledBalanceTokenBase` (`_mintScaled/_burnScaled`) → `AToken` / `VariableDebtToken` (+ `DebtTokenBase` for delegation) → `*Instance` (revision + `initialize`).

### 4.1 `AToken` (`src/contracts/protocol/tokenization/AToken.sol`)
- Storage per user: `UserState { uint120 balance; DelegationMode delegationMode; uint128 additionalData }` (`IncentivizedERC20.sol:55-61`). `balance` is the **scaled** balance; `additionalData` is the index at the user's last interaction.
- Immutables: `POOL`, `REWARDS_CONTROLLER` (3.4), `TREASURY` (3.4) (`IncentivizedERC20.sol:73-78`, `AToken.sol:32`). One implementation is shared by all reserves via `InitializableImmutableAdminUpgradeabilityProxy`; `initialize` sets name/symbol/decimals/underlying (`ATokenInstance.sol:33-50`).
- `balanceOf(user)` (`AToken.sol:133-138`): `scaled.getATokenBalance(POOL.getReserveNormalizedIncome(underlying))` (floor). `totalSupply` likewise (`:141-143`).
- `mint` / `burn` / `mintToTreasury` / `transferOnLiquidation` / `transferUnderlyingTo` are all `onlyPool` (`:63-130, 156-158`). `burn` also transfers the underlying out unless `receiverOfUnderlying == address(this)` (repay-with-aTokens, deficit elimination) (`:95-97`).
- `_transfer(from, to, uint120 amount)` (`:244-267`): compute `scaledAmount = amount.getATokenTransferScaledAmount(index)` (ceil), do the scaled transfer, then **`POOL.finalizeTransfer(...)`** so the Pool can enforce HF on the sender (§3.10). The inner `_transfer(sender, recipient, amount, scaledAmount, index)` (`:278-310`) emits `Mint` events for the interest each side accrued since their last interaction, updates both `additionalData`, then emits `Transfer(amount)` + `BalanceTransfer(scaledAmount, index)`.
- `transferFrom` (`:187-236`): allowance consumed = the sender's *actual* balance decrease (simulated: `balance(scaled) − balance(scaled − scaledAmount)`), capped at the allowance, and no `Approval` event, no consumption for `uint256.max` (3.5/3.6; `IncentivizedERC20._spendAllowance`, `:235-258`).
- `permit` (`:161-184`): EIP-2612 with `ECDSA.recover`.
- Rewards hook: `MintableIncentivizedERC20._mint/_burn` (`:36-63`) and `IncentivizedERC20._transfer` (`:266-279`) call `REWARDS_CONTROLLER.handleAction(user, oldTotalSupply, oldUserBalance)` with the **pre-change scaled** values, which is exactly what an index-based distributor needs (§5).
- `ATokenWithDelegation` (`ATokenWithDelegation.sol`): aAAVE only. Overrides `_mint/_burn/_transfer` to move voting/proposition power (`BaseDelegation._delegationChangeOnTransfer`) on the **scaled** amounts, storing delegated balances in 72-bit fields and `delegationMode` in the spare byte of `UserState`.

### 4.2 `VariableDebtToken` (`VariableDebtToken.sol`) + `DebtTokenBase`
- `balanceOf = scaled.getVTokenBalance(POOL.getReserveNormalizedVariableDebt(underlying))` (ceil) (`:69-74`).
- `mint(user, onBehalfOf, amount, scaledAmount, index)` (`:77-129`): if `user != onBehalfOf`, decrease `_borrowAllowances[onBehalfOf][user]` by the *actual* debt increase (simulated with ceil rounding) — this is **credit delegation**: Alice calls `vUSDC.approveDelegation(Bob, 1000)` (`DebtTokenBase.sol:40-42`, or `delegationWithSig`, `:50-75`), then Bob calls `Pool.borrow(USDC, 1000, 2, 0, onBehalfOf = Alice)`. Alice's collateral backs it, Alice owns the debt, Bob receives the tokens. `borrowAllowance(from, to)` (`:78-83`) is the view; `renounceDelegation` (3.6) zeroes it.
- `burn(from, scaledAmount, index)` returns `(noMoreDebt, scaledTotalSupply)` (`:132-147`).
- All ERC20 transfer/approve functions `revert OperationNotSupported()` (`:164-190`): debt is non-transferable by design.

### 4.3 `StataTokenV2` — ERC-4626 wrapper (`src/contracts/extensions/stata-token/`)
Problem: aTokens rebase (balance grows), which breaks many integrations (DEX pools, bridges). `StataTokenV2` is a non-rebasing vault whose *price* grows instead.
- `_rate() = POOL.getReserveNormalizedIncome(asset)` (`ERC4626StataTokenUpgradeable.sol:308-310`).
- `_convertToShares(assets) = assets × RAY / rate`, `_convertToAssets(shares) = shares × rate / RAY` with explicit rounding (`:292-306`).
- `deposit` (underlying) → `POOL.deposit(asset, ..., address(this))`; `depositATokens` (aToken directly) (`:77-88, 213-242`). `withdraw` → `POOL.withdraw`; `redeemATokens` → transfer aTokens out (`:125-134, 253-280`).
- `maxDeposit` respects supply cap/frozen; `maxRedeem` respects paused and available `virtualUnderlyingBalance` (`:150-203`).
- `latestAnswer()` exposes `price(asset) × rate` so it can be used as a Chainlink-style feed (`:206-211`).
- `ERC20AaveLMUpgradeable` tracks and forwards liquidity-mining rewards to share holders by snapshotting the RewardsController index in `_update` (`ERC20AaveLMUpgradeable.sol:164-198`).

### 4.4 `WrappedTokenGatewayV3` (`src/contracts/helpers/WrappedTokenGatewayV3.sol`)
Native ETH helper: `depositETH` wraps to WETH and `POOL.deposit(WETH, msg.value, onBehalfOf)` (`:45-48`); `withdrawETH` pulls aWETH via `transferFrom`, `POOL.withdraw` to itself, unwraps, sends ETH (`:55-70`); `borrowETH` requires the user to have `approveDelegation`'d the gateway on vWETH, then `POOL.borrow(..., onBehalfOf = msg.sender)` and unwraps (`:103-113`); `repayETH` wraps and repays, refunding dust (`:77-96`). The gateway gives the Pool an infinite WETH approval in its constructor (`:36`).

---

## 5. Rewards (liquidity mining)

`RewardsController` (`src/contracts/rewards/RewardsController.sol`) is `RewardsDistributor` + claiming + transfer strategies. Storage (`RewardsDataTypes.sol:24-53`): `_assets[aOrVToken].rewards[rewardToken] = RewardData { uint104 index; uint88 emissionPerSecond; uint32 lastUpdateTimestamp; uint32 distributionEnd; mapping(user => UserData{uint104 index; uint128 accrued}) }`.

Classic "reward-per-token index" (Synthetix-style) but on **scaled** balances:
```
assetIndex_new = assetIndex_old + emissionPerSecond × Δt × 10^decimals / scaledTotalSupply    (_getAssetIndex, RewardsDistributor.sol:489-517)
userAccrued   += userScaledBalance × (assetIndex_new − userIndex) / 10^decimals              (_getRewards, :469-480; _updateUserData :315-336)
```
Flow:
1. `EmissionManager` (owner-set per-reward "emission admin") calls `RewardsController.configureAssets([{asset, reward, emissionPerSecond, distributionEnd, transferStrategy, rewardOracle}])` (`EmissionManager.sol:39-44` → `RewardsController.sol:76-90`). It snapshots `scaledTotalSupply`, installs the transfer strategy and requires the reward oracle to return a price (`:331-354`).
2. On **every** mint/burn/transfer the token calls `handleAction(user, oldTotalSupply, oldUserBalance)` (`RewardsController.sol:109-111` → `_updateData`, `RewardsDistributor.sol:345-384`), which brings the asset index to now and settles the user's accrued rewards **before** their balance changes. Because both arguments are pre-change values, the accounting is exact.
3. `claimRewards(assets[], amount, to, reward)` / `claimAllRewards` (`:114-176`) first `_updateDataMultiple` with current balances, then zero `accrued` and call `transferStrategy.performTransfer(to, reward, amount)` (`:300-306`).
4. Transfer strategies: `PullRewardsTransferStrategy` does `safeTransferFrom(REWARDS_VAULT, to, amount)` (`PullRewardsTransferStrategy.sol:30-41`); `StakedTokenTransferStrategy` stakes into stkAAVE for the user (`StakedTokenTransferStrategy.sol:36-48`).
5. `setClaimer(user, claimer)` lets contracts that cannot claim (e.g. a vault holding aTokens) delegate claiming (`RewardsController.sol:179-182`).

Rewards can target either aTokens (incentivise supplying) or vTokens (incentivise borrowing) because both expose `getScaledUserBalanceAndSupply`.

---

## 6. Governance / admin

### 6.1 Roles (`ACLManager.sol:15-20`) and who can do what in `PoolConfigurator`
| Role | Typical holder | Can call |
|---|---|---|
| `DEFAULT_ADMIN_ROLE` | Governance executor (set from `ACL_ADMIN` in provider) | grant/revoke roles |
| `POOL_ADMIN` | Governance executor | everything: `initReserves`, `updateAToken/VariableDebtToken` (`:96-107`), `setReserveActive` (`:186-192`), `updateFlashloanPremium` (`:490-500`), `Pool.rescueTokens` |
| `RISK_ADMIN` | Risk stewards / Chaos Labs | `configureReserveAsCollateral` (`:118-171`), caps (`:291-312`), `setReserveFactor` (`:273-288`), `setReserveBorrowing`, `setReserveFlashLoaning`, `setLiquidationProtocolFee`, eMode create/bitmaps (`:328-424`), `setReserveInterestRateData` (`:457-471`) |
| `EMERGENCY_ADMIN` | Guardian multisig | `setReservePause`/`setPoolPause` (+ grace period) (`:240-262, 474-487`), `setReserveFreeze` (`:195-227`), `setReserveLtvzero` (`:230-237`), `setAssetLtvzeroInEMode`, `setEModeCategoryIsolated` (`:427-454`), `disableLiquidationGracePeriod` |
| `ASSET_LISTING_ADMIN` | Listing steward | `initReserves` (`:78-93`), `AaveOracle.setAssetSources` |
| `FLASH_BORROWER` | Whitelisted contracts | premium-free `flashLoan` |

### 6.2 Listing a reserve: `initReserves` → `ConfiguratorLogic.executeInitReserve` (`ConfiguratorLogic.sol:30-90`)
1. `decimals = IERC20Metadata(asset).decimals()`, require > 5 (3.1: low-decimal assets are inflation-attack prone).
2. Deploy an `InitializableImmutableAdminUpgradeabilityProxy` (admin = the configurator) for the aToken and one for the vToken, each initialised against the shared implementation (`_initTokenWithProxy`, `:160-171`).
3. `pool.initReserve(asset, aTokenProxy, vTokenProxy)` → `PoolLogic.executeInitReserve` (`PoolLogic.sol:34-59`): sets both indexes to `RAY`, assigns `id` (reuses a gap if any, else `_reservesCount++`, max 128).
4. Write initial config: decimals, active, not paused, not frozen, virtual accounting bit.
5. `strategy.setInterestRateParams(asset, rateData)`.
Everything else (LTV/LT/LB, caps, borrowing, reserve factor) is set by subsequent risk-admin calls. `configureReserveAsCollateral` enforces `ltv ≤ lt`, `lb > 100%`, `lt × lb ≤ 100%` (there is always enough collateral to pay the bonus at the moment of liquidation) (`PoolConfigurator.sol:127-141`).

### 6.3 Upgradeability
- Pool and Configurator are proxies whose admin is the `PoolAddressesProvider`; `setPoolImpl`/`setPoolConfiguratorImpl` (`PoolAddressesProvider.sol:80-96`) call `upgradeToAndCall(impl, initialize(provider))`. `VersionedInitializable` (`src/contracts/misc/aave-upgradeability/VersionedInitializable.sol:39-57`) only lets `initialize` run again when `getRevision()` increases, and the constructor bricks initialisation on the implementation itself (3.4).
- aToken/vToken proxies are admin'd by the Configurator (`updateAToken`, `ConfiguratorLogic.sol:98-119`).
- The `AaveV3ConfigEngine` (`src/contracts/extensions/v3-config-engine/`) is the payload helper governance uses to batch listings/updates; since 3.7 its sub-engines are inlined libraries, not delegatecalls.

---

## 7. End-to-end traces with numbers

Setup: ETH reserve `id = 0`, LTV 8000, LT 8250, LB 10500, decimals 18, liquidation fee 1000 (10%). USDC reserve `id = 1`, decimals 6, rate params from §2.6, reserve factor 1000 (10%). Oracle: ETH `2000e8`, USDC `1e8`. Both indexes start at `1e27`.

### (a) Alice supplies 10 ETH
`Pool.supply(WETH, 10e18, alice, 0)`
1. `cache`, `updateState`: no time passed → indexes stay `1e27`.
2. `scaledAmount = rayDivFloor(10e18, 1e27) = 10e18`.
3. `validateSupply` passes (no cap).
4. Rate update: ETH has no debt → `(0, base)`; `virtualUnderlyingBalance = 10e18`.
5. `WETH.transferFrom(alice, aWETH, 10e18)`.
6. `aWETH.mint(alice, alice, 10e18, 1e27)`: `_userState[alice].balance = 10e18`, `additionalData = 1e27`, `handleAction(alice, 0, 0)`. Returns `true` (first supply).
7. `getUserReserveLtv` = 8000 ≠ 0 → `_usersConfig[alice].data |= 1 << 1` (bit 1 = collateral for id 0) → `0b10`.
State: `aWETH.balanceOf(alice) = 10e18`. `getUserAccountData`: collateral `20000e8`, LTV 8000, LT 8250, HF `max`.

### (b) Alice borrows 5,000 USDC (assume 100,000 USDC was supplied by others, so `virtualUnderlyingBalance = 100000e6`)
`Pool.borrow(USDC, 5000e6, 2, 0, alice)`
1. `amountScaled = rayDivCeil(5000e6, 1e27) = 5000e6`.
2. `validateBorrow`: active, borrowing enabled, `aUSDC.totalSupply() = 100000e6 ≥ 5000e6`, mode VARIABLE, no cap.
3. `vUSDC.mint(alice, alice, 5000e6, 5000e6, 1e27)`: scaled balance 5000e6; `nextScaledVariableDebt = 5000e6`.
4. `_usersConfig[alice].data |= 1 << 2` (bit 2 = borrowing id 1) → `0b110`.
5. Rates: `totalDebt = 5000e6`, `available = 100000e6 − 5000e6 = 95000e6`, `U = 5%` → borrow `0.2222%`, supply `0.2222% × 5% × 90% = 0.01%`. `virtualUnderlyingBalance = 95000e6`.
6. `aUSDC.transferUnderlyingTo(alice, 5000e6)`.
7. `validateHFAndLtv`: loop bitmap `0b110`: id 0 collateral `20000e8`, ltv 8000, lt 8250; id 1 debt `mulDivCeil(5000e6, 1e8, 1e6) = 5000e8`. HF = `(20000e8×8250).wadDiv(5000e8)/1e4 = 3.3e18`. `20000e8 ≥ 5000e8.percentDivCeil(8000) = 6250e8` ✓.

### (c) One year passes, USDC utilization has been 80% the whole time (rates 3.5556% / 2.56%)
Nobody touched the reserve; on the next interaction `updateState` does:
- `liquidityIndex = rayMul(1e27 + 0.0256e27, 1e27) = 1.0256e27`
- `variableBorrowIndex = 1.0361951e27` (binomial)
- `_accrueToTreasury`: debt accrued = `scaledDebt × 0.0361951e27`; 10% of that, divided by `1.0256e27`, added to `accruedToTreasury`.
Alice's debt is now `rayMulCeil(5000e6, 1.0361951e27) = 5180.98 USDC` even though `vUSDC` storage never changed. `getReserveNormalizedVariableDebt` returns the same index as a view, so `vUSDC.balanceOf(alice)` already shows it before any tx.

### (d) ETH falls; two liquidations (Alice's debt was 15,000 USDC in this variant, see §0.7)
- ETH `1780e8`: HF 0.979. Bob calls `liquidationCall(WETH, USDC, alice, type(uint256).max, false)`.
  - `reserveCollateral$ = 17800e8 ≥ 2000e8`, `reserveDebt$ = 15000e8 ≥ 2000e8`, HF > 0.95 → `maxLiquidatableDebt = 50% × 15000 = 7500 USDC`.
  - `baseCollateral = 1e8 × 7500e6 × 1e18 / (1780e8 × 1e6) = 4.2135e18`; `× 10500 floor = 4.4242e18` ≤ balance.
  - fee: `bonusCollateral = 4.4242 − 4.4242/1.05 = 0.2107`; `protocolFee = 0.02107` (ceil); liquidator collateral `4.4031e18`.
  - Dust rule: leftover debt `7500e8 ≥ 1000e8`, leftover collateral `5.5758 × 1780 = 9925e8 ≥ 1000e8` ✓.
  - `_burnDebtTokens`: burn `rayDivFloor(7500e6, idx)`; not bad debt; rates updated with `liquidityAdded = 7500e6`.
  - `_burnCollateralATokens`: `virtualUnderlyingBalance −= 4.4031e18`; `aWETH.burn(alice → bob, 4.4031e18)`.
  - `aWETH.transferOnLiquidation(alice → TREASURY, 0.02107e18)`.
  - `USDC.transferFrom(bob, aUSDC, 7500e6)`.
- ETH `1500e8`: HF 0.920 ≤ 0.95 → close factor 100%. Bob repays 7,500 and takes 5.25 ETH (5.225 net). Alice keeps 0.3258 ETH with zero debt: her borrowing bit is cleared by `noMoreDebt`, `MustNotLeaveDust` does not apply because `actualDebtToLiquidate == borrowerReserveDebt`.

---

## 8. Security notes

1. **Oracle dependence.** Every HF, LTV check and liquidation price comes from `AaveOracle.getAssetPrice` → Chainlink `latestAnswer()` with a fallback if ≤ 0 (`AaveOracle.sol:101-117`). There is no staleness check in the oracle contract itself: that is delegated to governance choosing feeds (and, since 3.7, no on-chain sequencer sentinel). A manipulated or stale price is the top systemic risk; caps, LT buffers and eMode design are the mitigations.
2. **Rounding / "1 wei" attacks.** With half-up rounding an attacker could repeatedly round in their favour (e.g. withdraw slightly more than deposited, or leave 1 wei of un-liquidatable debt). 3.5's `TokenMath` fixes the direction of every conversion so the protocol never loses on rounding, and 3.7 makes liquidation math deterministic with floor/ceil. The `transferFrom` allowance simulation (`AToken.sol:196-233`) exists purely because of this.
3. **Virtual accounting** (3.1) means utilization, rates and withdrawable liquidity depend on `virtualUnderlyingBalance`, not `ERC20.balanceOf(aToken)`. Donating tokens to an aToken cannot move rates; a rebasing/fee-on-transfer underlying is still unsupported. The extra guard `aToken.totalSupply() ≥ amount` on borrow/flash (`ValidationLogic.sol:132-135, 236`) is a soft inflation defence.
4. **Flash-loan reentrancy** is handled by ordering (validate → callback → accounting), by subtracting from the virtual balance before the callback, and by the per-asset `flashLoanEnabled` flag. There is deliberately no global reentrancy lock, so composability (e.g. flash → supply → borrow in the callback) works.
5. **Why isolation/siloed were removed (3.7).** The `isolated` eMode flag gives the same risk isolation with a single bit in `getUserReserveLtv`, without per-reserve debt-ceiling accounting on every borrow/repay/liquidation. Less code, less gas, fewer edge cases.
6. **Bad debt: deficit vs socialisation.** Aave does not socialise losses across suppliers (indexes never go down). Bad debt is written to `reserve.deficit`, stops accruing (vTokens burned), dilutes the *supply* rate via `supplyUsageRatio` (`DefaultReserveInterestRateStrategyV2.sol:142-144`), and is expected to be covered by Umbrella (stakers' slashable aTokens) via `eliminateReserveDeficit`.
7. **Liquidation incentives and dust.** 50% close factor on the *total* position, 100% for small or deeply underwater positions, and the `MIN_LEFTOVER_BASE` rule together ensure positions stay economically liquidatable and cannot be gamed into un-liquidatable dust (3.3).
8. **Admin keys.** All admin paths go through the `PoolAddressesProvider` owner (governance executor behind a timelock) and ACL roles. Emergency admins can only pause/freeze/ltv0 — actions that reduce risk — and un-pausing comes with a ≤ 4h liquidation grace period. Upgrades of Pool, Configurator and token implementations are governance-only; libraries are relinked by deploying a new Pool implementation.
9. **Self-liquidation forbidden** (`ValidationLogic.sol:261`) since 3.4 to avoid fee/deficit accounting games.
10. **aToken receiver not auto-collateralised** (3.6). Integrations receiving aTokens must explicitly enable collateral; otherwise those funds do not back borrows.

---

## 9. Exercises to trace yourself

1. **Follow a `supply` end-to-end in the debugger of your head.** Start at `src/contracts/protocol/pool/Pool.sol:118`, go to `SupplyLogic.sol:40`, `ReserveLogic.sol:251` (cache), `:85` (updateState), `ValidationLogic.sol:39`, `ReserveLogic.sol:130`, `AToken.sol:63`, `ScaledBalanceTokenBase.sol:69`, `MintableIncentivizedERC20.sol:36`, `RewardsController.sol:109`. Write down every SSTORE.
2. **Prove `Σ aToken balances ≤ Σ debt + virtual balance`** stays true across `updateState` by reading `ReserveLogic.sol:211-243` and `:183-204`. Where does the reserve factor go, and why is it stored scaled by `nextLiquidityIndex` rather than in underlying?
3. **Rounding audit.** For each call site of `rayDivCeil` / `rayDivFloor` / `rayMulCeil` / `rayMulFloor` in `LiquidationLogic.sol:166-460`, state who benefits from the rounding direction and why it cannot create dust. Compare with `docs/3.7/liquidation-rounding.md`.
4. **Compute the close factor by hand** for a user with $3k GHO, $3k USDC, $3k DAI debt and $9k ETH collateral at HF 0.97: how much USDC can one `liquidationCall` repay? (Read `LiquidationLogic.sol:258-282`; the answer changed in 3.3.)
5. **Credit delegation.** Trace `Pool.borrow(..., onBehalfOf = alice)` from `BorrowLogic.sol:69-76` into `VariableDebtToken.sol:84-120` and `DebtTokenBase.sol:103-120`. Why is the allowance decreased by the *simulated* debt increase rather than by `amount`?
6. **eMode entry.** Read `ValidationLogic.sol:448-498` and `:524-549`. Construct a user for whom `setUserEMode(1)` reverts with `InvalidCollateralInEmode` in an `isolated` category but succeeds in a non-isolated one with identical bitmaps.
7. **Flash loan into debt.** In `FlashLoanLogic.sol:103-154`, `interestRateModes[i] = 2` calls `executeBorrow` with `releaseUnderlying = false`. Which storage differs after the tx compared with a plain `borrow` of the same amount? (Hint: `virtualUnderlyingBalance` was already decremented at `:84`.)
8. **Rewards exactness.** Using `RewardsDistributor.sol:489-517` and `:315-336`, show that a user who holds 10% of `scaledTotalSupply` for the entire emission period receives exactly 10% of `emissionPerSecond × duration` regardless of how many times `handleAction` fires.
