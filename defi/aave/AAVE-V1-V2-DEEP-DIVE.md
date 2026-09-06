# Aave v1 & v2 Deep Dive (the road to v3)

> Sibling documents in this folder:
> - `aave/AAVE-DEEP-DIVE.md` — Aave v3.6 in depth (`aave-v3-origin`).
> - `aave/AAVE-V4-DEEP-DIVE.md` — Aave v4 Hub & Spoke architecture.
>
> This document covers the two generations that came before, from the code in
> `aave/v1-aave-protocol` (Solidity 0.5) and `aave/v2-protocol` (Solidity 0.6.12).
> Every claim is anchored to a `path:line` you can open yourself.

---

## 0. Why read old versions

You are not reading v1 and v2 to deploy them. You are reading them because **v3 is a
sequence of answers, and v1/v2 are the questions**. Almost every "weird" thing in v3 —
scaled balances, the reserve factor, the bitmap config, the absence of stable rate — is
a scar from a specific v1 or v2 design that hurt.

**v1 (Jan 2020).** A lending pool that works. Funds and state live in one giant contract
(`LendingPoolCore`, 1775 lines). aTokens rebase, tracking a **per-user index**. Debt is
a plain struct field, not a token. Borrowing costs a one-time **origination fee**.
Liquidations run through a `delegatecall` into a separate manager. It was correct, and
it was extremely expensive to use.

**v2 (Dec 2020).** The great refactor. Funds move out of the core and into each aToken.
State becomes `DataTypes.ReserveData`, logic becomes linked libraries. Debt becomes
**real ERC20-shaped tokens** (`StableDebtToken`, `VariableDebtToken`), which unlocks
credit delegation and makes debt visible to the outside world. Interest math swaps an
exact loop for a **3-term binomial approximation**. A **reserve factor** finally sends a
slice of interest to the treasury. Flash loans get batching and the (later regretted)
ability to end as a debt position.

**v3 (Mar 2022) and beyond.** Risk controls: isolation mode, eMode, supply/borrow caps,
siloed borrowing, a price-oracle sentinel, and heavy gas work. Then v3.1–v3.6 (2024–2026)
removed the stable rate entirely, reworked rounding via `TokenMath`, added virtual
balances and explicit deficit accounting. See `aave/AAVE-DEEP-DIVE.md`.

```
Jan 2020        Dec 2020         Mar 2022          2024 ────────────── 2026
   v1              v2               v3.0           v3.1  v3.2  v3.3  v3.5  v3.6      v4
   │               │                 │               │     │     │     │     │        │
Core holds     aToken holds      risk params    flashloan  stable  liq.  Token  deficit  Hub &
funds+state    funds; debt       (isolation,    fee split  rate    close  Math   /Umbrella Spoke
per-user idx   tokenized;        eMode, caps)              REMOVED factor
rebasing       scaled balance                                      change
```

---

## 1. Aave v1 (2020)

### 1.1 Architecture: logic and money in different places (but money in *one* place)

```
                        ┌───────────────────────────────┐
                        │ LendingPoolAddressesProvider  │  registry of every component
                        │  (v1/…/configuration/…)       │  behind upgradeable proxies
                        └───────────────┬───────────────┘
                                        │ getX()
      user ──deposit/borrow/repay──▶ ┌──▼──────────────┐
                                     │   LendingPool   │  logic + validation only
                                     │  (1007 lines)   │
                                     └──┬───┬───┬──────┘
                     updateStateOnX()   │   │   │  delegatecall("liquidationCall")
                     transferToUser()   │   │   └──────────────────┐
                                        │   │                      ▼
                  ┌─────────────────────▼┐  │        ┌───────────────────────────────┐
                  │   LendingPoolCore    │  │        │ LendingPoolLiquidationManager │
                  │  ★ HOLDS ALL FUNDS ★ │  │        │ (runs in LendingPool storage!)│
                  │  ★ HOLDS ALL STATE ★ │  │        └───────────────────────────────┘
                  │      1775 lines      │  │
                  └──────────┬───────────┘  │ views (health factor)
                             │              ▼
                    mintOnDeposit()   ┌──────────────────────┐
                             │        │ LendingPoolDataProvider│
                        ┌────▼─────┐  └──────────────────────┘
                        │  AToken  │  rebasing, per-user index, interest redirection
                        └────┬─────┘
                             │ redeem() calls back into pool.redeemUnderlying()
                             └──────────────▶ LendingPool
```

Three structural facts define v1:

1. **`LendingPoolCore` custodies every asset of every reserve.** One contract, one
   balance, all markets. Its fallback only accepts ETH from contracts
   (`v1-aave-protocol/contracts/lendingpool/LendingPoolCore.sol:385`).
2. **aTokens rebase off a per-user index**, not a scaled balance
   (`.../tokenization/AToken.sol:338`).
3. **Liquidation is a `delegatecall`** from `LendingPool` into
   `LendingPoolLiquidationManager`, so the manager executes *inside `LendingPool`'s
   storage* (`.../lendingpool/LendingPool.sol:815`).

State lives in exactly two mappings in the core
(`.../lendingpool/LendingPoolCore.sol:75`):

```solidity
mapping(address => CoreLibrary.ReserveData) internal reserves;
mapping(address => mapping(address => CoreLibrary.UserReserveData)) internal usersReserveData;
```

### 1.2 The data structures (`CoreLibrary`)

`UserReserveData` (`.../libraries/CoreLibrary.sol:19`) — note that everything is a full
`uint256`; there is no bit packing anywhere in v1:

| Field | Meaning |
|---|---|
| `principalBorrowBalance` | Debt as of the **last user interaction**, not now. |
| `lastVariableBorrowCumulativeIndex` | The variable index snapshot **for this user**. Set to `0` when borrowing stable. |
| `originationFee` | Accrued one-time borrow fee, owed on top of principal. |
| `stableBorrowRate` | The rate frozen at borrow time. `0` means the loan is variable. |
| `lastUpdateTimestamp` | When this user's position last accrued. |
| `useAsCollateral` | Per-reserve collateral flag (a `bool`, not a bit). |

`ReserveData` (`.../libraries/CoreLibrary.sol:33`) carries `lastLiquidityCumulativeIndex`,
`currentLiquidityRate`, `totalBorrowsStable`, `totalBorrowsVariable`,
`currentVariableBorrowRate`, `currentStableBorrowRate`, `currentAverageStableBorrowRate`,
`lastVariableBorrowCumulativeIndex`, plus config (`baseLTVasCollateral`,
`liquidationThreshold`, `liquidationBonus`, `decimals`) and five separate `bool` flags.

> **Read this carefully:** v1 stores `totalBorrowsStable` / `totalBorrowsVariable` as
> **absolute amounts**, so every borrow and repay must add/subtract from a running total.
> v2 replaces this with a scaled total supply on the debt token, which is
> self-accruing. This one change removes a whole class of accounting drift.

**Index accrual** (`.../libraries/CoreLibrary.sol:111`). Suppliers accrue **linearly**,
variable borrowers **compound** — the asymmetry that survives into v3:

```solidity
function updateCumulativeIndexes(ReserveData storage _self) internal {
    uint256 totalBorrows = getTotalBorrows(_self);
    if (totalBorrows > 0) {
        uint256 cumulatedLiquidityInterest = calculateLinearInterest(
            _self.currentLiquidityRate, _self.lastUpdateTimestamp);
        _self.lastLiquidityCumulativeIndex = cumulatedLiquidityInterest.rayMul(
            _self.lastLiquidityCumulativeIndex);
        uint256 cumulatedVariableBorrowInterest = calculateCompoundedInterest(
            _self.currentVariableBorrowRate, _self.lastUpdateTimestamp);
        _self.lastVariableBorrowCumulativeIndex = cumulatedVariableBorrowInterest.rayMul(
            _self.lastVariableBorrowCumulativeIndex);
    }
}
```

**Compounding in v1 is a real exponentiation.** `calculateCompoundedInterest`
(`.../libraries/CoreLibrary.sol:413`) computes `(1 + r/yr)^Δt` with `rayPow`, a
square-and-multiply loop (`.../libraries/WadRayMath.sol:72`) that costs `O(log Δt)`
`rayMul`s. v2 replaces this with three terms of a binomial series — see §2.4. This is
one of the single largest gas differences between the two versions.

**`cumulateToLiquidityIndex`** (`.../libraries/CoreLibrary.sol:142`) injects a one-off
amount into the index, spreading it over all depositors instantly. v1 uses it for flash
loan fees; v2 keeps the function verbatim.

**`getCompoundedBorrowBalance`** (`.../libraries/CoreLibrary.sol:254`) is where the
per-user index shows its face:

```solidity
if (_self.stableBorrowRate > 0) {
    cumulatedInterest = calculateCompoundedInterest(_self.stableBorrowRate, _self.lastUpdateTimestamp);
} else {
    cumulatedInterest = calculateCompoundedInterest(
            _reserve.currentVariableBorrowRate, _reserve.lastUpdateTimestamp)
        .rayMul(_reserve.lastVariableBorrowCumulativeIndex)
        .rayDiv(_self.lastVariableBorrowCumulativeIndex);   // ← ratio of indexes
}
compoundedBalance = principalBorrowBalanceRay.rayMul(cumulatedInterest).rayToWad();
```

A **variable** borrower's debt is `principal × (reserveIndexNow / userIndexAtBorrow)`. A
**stable** borrower ignores the reserve index entirely and compounds their own frozen
rate from their own timestamp. Two completely different accrual paths in one struct.

And a detail worth remembering, at `.../libraries/CoreLibrary.sol:283`:

```solidity
if (compoundedBalance == _self.principalBorrowBalance) {
    if (_self.lastUpdateTimestamp != block.timestamp) {
        return _self.principalBorrowBalance.add(1 wei);  // no interest-free loans
    }
}
```

Rounding could otherwise let a tiny loan accrue exactly zero interest, so v1 charges a
symbolic wei. v3.5+ generalises this instinct into the `TokenMath` rounding rules.

### 1.3 The interest rate model, and the missing reserve factor

`calculateInterestRates`
(`.../lendingpool/DefaultReserveInterestRateStrategy.sol:108`) is the kinked model you
already know: below `OPTIMAL_UTILIZATION_RATE` the variable rate rises along
`variableRateSlope1`, above it along `slope2` scaled by how far into the excess band
utilisation sits. The stable rate starts from an **external oracle** value
(`ILendingRateOracle.getMarketBorrowRate`, line 129) — an off-chain view of what that
asset costs in the real world — then adds the same slopes.

The supply rate is the weighted average borrow rate times utilisation
(`.../DefaultReserveInterestRateStrategy.sol:157`):

```solidity
currentLiquidityRate = getOverallBorrowRateInternal(
    _totalBorrowsStable, _totalBorrowsVariable,
    currentVariableBorrowRate, _averageStableBorrowRate
).rayMul(utilizationRate);
```

with the weighted average at line 175:

```
overallBorrowRate = (totalVariable × variableRate + totalStable × avgStableRate) / totalBorrows
```

> **The thing that is not there:** no `reserveFactor`. In v1, **100% of borrower interest
> goes to suppliers.** The protocol's only revenue is the origination fee and the flash
> loan protocol cut. v2 adds `.percentMul(PERCENTAGE_FACTOR - reserveFactor)` to this
> exact line and creates the treasury.

Rates are re-derived after every balance-changing action by
`updateReserveInterestRatesAndTimestampInternal`
(`.../lendingpool/LendingPoolCore.sol:1703`), which recomputes all three rates and stamps
`lastUpdateTimestamp`.

### 1.4 Stable rate borrowing: the idea and the flaw

A stable-rate loan freezes the borrower's rate at borrow time
(`.../lendingpool/LendingPoolCore.sol:1325`):

```solidity
if (_rateMode == CoreLibrary.InterestRateMode.STABLE) {
    user.stableBorrowRate = reserve.currentStableBorrowRate;
    user.lastVariableBorrowCumulativeIndex = 0;
} else if (_rateMode == CoreLibrary.InterestRateMode.VARIABLE) {
    user.stableBorrowRate = 0;
    user.lastVariableBorrowCumulativeIndex = reserve.lastVariableBorrowCumulativeIndex;
}
```

The reserve then tracks `currentAverageStableBorrowRate` as a liquidity-weighted average,
updated on every stable borrow (`.../libraries/CoreLibrary.sol:303`) and repay (line 331).

The economic problem: **a fixed rate is a free option written by the protocol.** If market
rates rise, the borrower keeps paying the old low rate while suppliers demand more. Two
guards exist:

- **Borrow-time limits** (`.../lendingpool/LendingPool.sol:469`): stable borrowing must be
  enabled, the user must not be borrowing the same asset they mostly collateralise
  (`isUserAllowedToBorrowAtStable`), and the loan is capped at
  `getMaxStableRateBorrowSizePercent()` of available liquidity.
- **`rebalanceStableBorrowRate`** (`.../lendingpool/LendingPool.sol:709`), callable by
  **anyone**, which resets a user's rate when either the loan has become abusive or unfair
  (line 741):

```solidity
if (userCurrentStableRate < liquidityRate ||
    userCurrentStableRate > rebalanceDownRateThreshold) {
    uint256 newStableRate = core.updateStateOnRebalance(_reserve, _user, borrowBalanceIncrease);
    ...
}
revert("Interest rate rebalance conditions were not met");
```

The first branch is the important one: if your stable borrow rate has fallen **below the
supply rate**, you could borrow and redeposit the same asset at a profit. Rebalancing
closes that loop.

Neither guard was ever fully sufficient. Stable-rate debt kept creating rate-inversion
and solvency edge cases, and Aave finally deleted the whole mechanism in **v3.2**.

### 1.5 The user-facing functions, traced

Every one of these follows the same skeleton:
**validate in `LendingPool` → mutate in `LendingPoolCore` → move tokens → emit.**

#### `deposit(reserve, amount, referralCode)` — `.../lendingpool/LendingPool.sol:299`

- **Checks:** `nonReentrant`, reserve active, not frozen, amount > 0 (modifiers, lines 302–305).
- **Calls:** `core.updateStateOnDeposit` (line 311) → `updateCumulativeIndexes()` then
  `updateReserveInterestRatesAndTimestampInternal(_reserve, _amount, 0)`
  (`.../LendingPoolCore.sol:107`), auto-enabling collateral on first deposit.
- **Then:** `aToken.mintOnDeposit(msg.sender, _amount)` (line 314), then
  `core.transferToReserve` pulls the funds **into the core** (line 317).
- **Emits:** `Deposit`.

Note the ordering: **aTokens are minted before the underlying arrives.** Safe only
because the core, not the aToken, is the custodian and the pull is unconditional.

#### `redeemUnderlying(...)` — `.../lendingpool/LendingPool.sol:331`

The **aToken calls the pool**, not the other way around — guarded by
`onlyOverlyingAToken` (line 339). The user's entry point is `AToken.redeem`
(`.../tokenization/AToken.sol:218`), which burns first and then asks the pool to release
funds at line 255. The pool checks available liquidity (line 344), calls
`core.updateStateOnRedeem`, and transfers.

#### `borrow(reserve, amount, interestRateMode, referralCode)` — `.../lendingpool/LendingPool.sol:388`

1. Reserve is borrowing-enabled and the rate mode is valid (lines 404–410).
2. Enough liquidity (line 418).
3. `dataProvider.calculateUserGlobalData(msg.sender)` (line 432) → collateral, debt, fees,
   LTV, liquidation threshold, and `healthFactorBelowThreshold`.
4. Collateral must exist (line 434) and the user must not already be liquidatable (line 436).
5. **Origination fee** `feeProvider.calculateLoanOriginationFee` (line 442), which must be
   non-zero — so dust borrows revert (line 444).
6. `calculateCollateralNeededInETH` including existing fees; must be ≤ collateral (line 455).
7. Stable-mode extra checks (lines 469–485).
8. `core.updateStateOnBorrow` (line 488) → reserve totals, user state, rates.
9. `core.transferToUser` (line 497), emit `Borrow`.

`updateStateOnBorrow` (`.../LendingPoolCore.sol:181`) reads the user's current balances,
then calls `updateReserveStateOnBorrowInternal` (line 1281) and
`updateUserStateOnBorrowInternal` (line 1314). The latter **capitalises accrued interest
into principal**:

```solidity
user.principalBorrowBalance = user.principalBorrowBalance.add(_amountBorrowed).add(_balanceIncrease);
user.originationFee = user.originationFee.add(_fee);
user.lastUpdateTimestamp = uint40(block.timestamp);
```

#### `repay(reserve, amount, onBehalfOf)` — `.../lendingpool/LendingPool.sol:533`

The fee makes this messier than v2's repay. Payback defaults to
`compoundedBorrowBalance + originationFee` (line 560). **Fees are paid first**: if the
payment is ≤ the fee it all goes to the `TokenDistributor` and the loan is untouched
(lines 572–599). Otherwise the fee is split off (line 602), state is updated (line 604),
the fee is forwarded (line 615) and the remainder goes to the reserve (line 626).

#### `swapBorrowRateMode(reserve)` — line 648
Requires an open borrow; swapping *to* stable re-runs `isUserAllowedToBorrowAtStable`
(line 675). Delegates to `core.updateStateOnSwapRate`
(`.../LendingPoolCore.sol:262`).

#### `setUserUseReserveAsCollateral(reserve, useAsCollateral)` — line 772
Requires a balance and `dataProvider.balanceDecreaseAllowed(...)` for the **full**
balance (line 782) — i.e. you may only disable collateral if you would still be healthy
with none of it counted.

#### `liquidationCall(collateral, reserve, user, purchaseAmount, receiveAToken)` — line 805

```solidity
address liquidationManager = addressesProvider.getLendingPoolLiquidationManager();
(bool success, bytes memory result) = liquidationManager.delegatecall(
    abi.encodeWithSignature("liquidationCall(address,address,address,uint256,bool)", ...));
require(success, "Liquidation call failed");
(uint256 returnCode, string memory returnMessage) = abi.decode(result, (uint256, string));
if (returnCode != 0) revert(string(abi.encodePacked("Liquidation failed: ", returnMessage)));
```

This existed purely to dodge the **24KB contract size limit**; `LendingPool` was already
near it. The manager returns `(code, message)` instead of reverting so the pool can
prefix the error — a pre-custom-errors idiom.

Inside the manager (`.../lendingpool/LendingPoolLiquidationManager.sol:124`):

1. Health factor must be below threshold (line 134).
2. User must actually hold that collateral (line 145) with the flag on (line 155).
3. User must actually owe the debt asset (line 168).
4. **Close factor 50%** — `LIQUIDATION_CLOSE_FACTOR_PERCENT = 50` (line 35), applied at line 179.
5. `calculateAvailableCollateralToLiquidate` (line 314) prices both legs and applies the bonus:

```solidity
vars.maxAmountCollateralToLiquidate = vars.principalCurrencyPrice
    .mul(_purchaseAmount).div(vars.collateralPrice)
    .mul(vars.liquidationBonus).div(100);
```

6. **The origination fee is liquidated too** (lines 198–208, 256–278) against whatever
   collateral remains — a whole code path that disappears in v2.
7. Settlement: `transferOnLiquidation` if `receiveAToken`, else `burnOnLiquidation` plus
   `core.transferToUser` (lines 244–251).

#### `flashLoan(receiver, reserve, amount, params)` — line 843

Single-asset only. Measures the core's balance before (line 851), computes
`amountFee` and the `protocolFee` slice (lines 863–866), sends funds, calls
`receiver.executeOperation` (line 881), then requires the balance to be **exactly**
`before + amountFee` (line 888). `updateStateOnFlashLoan`
(`.../LendingPoolCore.sol:150`) forwards the protocol slice to the `TokenDistributor` and
spreads the rest to depositors via `cumulateToLiquidityIndex`.

### 1.6 The v1 AToken: rebasing on a per-user index, and interest redirection

`balanceOf` (`.../tokenization/AToken.sol:338`) is not `scaled × index`. It is
`principal × (indexNow / userIndex)`, via `calculateCumulatedBalanceInternal` (line 522):

```solidity
return _balance.wadToRay()
    .rayMul(core.getReserveNormalizedIncome(underlyingAssetAddress))
    .rayDiv(userIndexes[_user])
    .rayToWad();
```

`cumulateBalanceInternal` (line 452) **materialises** accrued interest by minting the
difference and re-stamping the user's index:

```solidity
uint256 previousPrincipalBalance = super.balanceOf(_user);
uint256 balanceIncrease = balanceOf(_user).sub(previousPrincipalBalance);
_mint(_user, balanceIncrease);
uint256 index = userIndexes[_user] = core.getReserveNormalizedIncome(underlyingAssetAddress);
```

So a v1 aToken's stored ERC20 balance drifts upward on every interaction. **Two users
with identical deposits at identical times can hold different stored balances** simply
because one of them transacted more often. Contrast v2/v3, where the stored value is
scaled and *never* changes from accrual alone.

**Interest redirection** is v1's most distinctive feature and exists nowhere else in
Aave. `redirectInterestStream(to)` (line 179) — or `redirectInterestStreamOf` (line 191)
after `allowInterestRedirectionTo` (line 205) — assigns your yield to another address
while you keep your principal. It is implemented with a parallel
`redirectedBalances` mapping, and `balanceOf` forks on it (line 351):

- **Not redirecting:** interest accrues on `principal + redirectedBalance`, then the
  redirected part is subtracted back out (lines 355–359).
- **Redirecting:** only the balance others redirected *to you* earns for you; your own
  principal's yield goes elsewhere (lines 365–371).

`updateRedirectedBalanceOfRedirectionAddressInternal` (line 479) keeps this consistent on
every mint, burn and transfer, and even handles **chained redirection** (line 501).

Elegant, and a maintenance burden: every balance-changing path had to remember to call
it, and it made the token's accounting genuinely hard to reason about. v2 dropped it.

Other pool-only entry points: `mintOnDeposit` (line 271), `burnOnLiquidation` (line 297),
`transferOnLiquidation` (line 325), all gated by `onlyLendingPool` (line 135). Transfers
are gated by `whenTransferAllowed` (line 143) → `isTransferAllowed` (line 413) →
`dataProvider.balanceDecreaseAllowed`, so you cannot transfer collateral you need.

### 1.7 The origination fee and the TokenDistributor

`FeeProvider.initialize` (`.../fees/FeeProvider.sol:29`) sets:

```solidity
originationFeePercentage = 0.0025 * 1e18;   // 25 bps of the borrowed amount
```

`calculateLoanOriginationFee` (line 40) is `amount.wadMul(originationFeePercentage)`.
The fee is charged **once, at borrow time**, accumulated in `user.originationFee`, and
routed to the `TokenDistributor` (`.../fees/TokenDistributor.sol`), which split it toward
LEND/AAVE buyback-and-burn.

The comment at `FeeProvider.sol:30` says "0.0025%" — the code means **0.25%**. A doc bug
frozen in the repo forever.

v2 kept a flash-loan premium but dropped the borrow origination fee. v3 replaced protocol
revenue entirely with the reserve factor and liquidation protocol fee.

### 1.8 Why v1 was replaced

| Problem | Consequence |
|---|---|
| `LendingPoolCore` is one 1775-line monolith holding all funds and state | Enormous gas per action; every op touches many full-word slots |
| No bit packing (`ReserveData` is ~19 separate `uint256`/`bool` slots) | Cold `SLOAD`s dominate the cost of every call |
| Debt is a struct field, not a token | Debt is invisible to wallets/integrators; no credit delegation possible |
| Rebasing aTokens with a **per-user** index | Every DeFi integration must special-case them; balances differ between equal depositors |
| `rayPow` exact compounding | `O(log Δt)` multiplications on the hot path |
| Origination fee in the position | Extra state, extra branches, an entire fee-liquidation code path |
| Interest redirection | Fragile invariants across every mint/burn/transfer |
| No reserve factor | Protocol earns nothing from interest |
| `calculateUserGlobalData` loops **all** reserves (`.../LendingPoolDataProvider.sol:91`) | Health factor cost grows with the market's asset count, not the user's position |

---

## 2. Aave v2 (December 2020)

### 2.1 Architecture: money moves to the tokens, logic moves to libraries

```
                 ┌──────────────────────────────┐
                 │ LendingPoolAddressesProvider │
                 └───────────────┬──────────────┘
                                 │
   user ──▶ ┌─────────────────────▼──────────────────────┐
            │              LendingPool                   │  storage: _reserves, _usersConfig,
            │  (logic only — holds NO funds, 946 lines)  │           _reservesList
            └───┬──────────┬──────────┬──────────┬───────┘
   linked libs  │          │          │          │ delegatecall
   (DELEGATECALL│ at the   │          │          ▼
    EVM level,  │ library  │      ┌───▼──────────────────────────────┐
    but source- │ level)   │      │ LendingPoolCollateralManager     │
    inlined)    │          │      │ (runs in LendingPool storage)    │
                ▼          ▼      └──────────────────────────────────┘
        ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
        │ ReserveLogic │ │ValidationLogic│ │ GenericLogic │
        │ index math   │ │ all requires │ │ health factor│
        └──────────────┘ └──────────────┘ └──────────────┘
                │
                │ per reserve, three tokens:
                ▼
   ┌────────────────┐  ┌────────────────────┐  ┌──────────────────┐
   │     AToken     │  │ VariableDebtToken  │  │ StableDebtToken  │
   │ ★ HOLDS FUNDS ★│  │ scaled, non-transf.│  │ non-transferable │
   └────────────────┘  └────────────────────┘  └──────────────────┘
```

Three inversions relative to v1:

1. **Each aToken custodies its own underlying.** `transferUnderlyingTo`
   (`v2-protocol/contracts/protocol/tokenization/AToken.sol:308`) is how the pool
   releases funds; deposits `safeTransferFrom(msg.sender, aToken, amount)`
   (`.../lendingpool/LendingPool.sol:119`). A bug in one market can no longer drain another.
2. **Logic is libraries.** `ReserveLogic`, `ValidationLogic`, `GenericLogic` are linked
   at deploy time, which keeps `LendingPool` under the size limit without a manual
   `delegatecall` — except liquidation, which still needs one
   (`.../lendingpool/LendingPool.sol:436`).
3. **Debt is tokenized**, which is what makes credit delegation expressible.

`DataTypes.ReserveData` (`.../libraries/types/DataTypes.sol:6`) is now aggressively
packed: `liquidityIndex`, `variableBorrowIndex` and all three rates are `uint128`,
`lastUpdateTimestamp` is `uint40`, and `id` is `uint8`. Config collapses into a **single
`uint256` bitmap**.

### 2.2 The configuration bitmaps

`ReserveConfigurationMap` (`.../libraries/types/DataTypes.sol:30`), with masks and shifts
at `.../libraries/configuration/ReserveConfiguration.sol:13-31`:

```
 bit  0-15 : LTV                       (bps, max 65535)
 bit 16-31 : liquidation threshold     (bps)
 bit 32-47 : liquidation bonus         (bps)
 bit 48-55 : decimals
 bit 56    : isActive
 bit 57    : isFrozen
 bit 58    : borrowingEnabled
 bit 59    : stableRateBorrowingEnabled
 bit 60-63 : reserved
 bit 64-79 : reserve factor            (bps)
 bit 80-255: unused  ← v3 fills this with caps, eMode, isolation ceiling, flags…
```

`UserConfigurationMap` (line 43) is one `uint256` giving each reserve **two bits**:
`isUsingAsCollateral` and `isBorrowing`. That is why `MAX_NUMBER_RESERVES` is 128, and why
`calculateUserAccountData` can skip untouched reserves cheaply.

### 2.3 Debt tokenization

**`VariableDebtToken`** is the simple one (`.../tokenization/VariableDebtToken.sol:95`):

```solidity
if (user != onBehalfOf) { _decreaseBorrowAllowance(onBehalfOf, user, amount); }
uint256 previousBalance = super.balanceOf(onBehalfOf);
uint256 amountScaled = amount.rayDiv(index);
require(amountScaled != 0, Errors.CT_INVALID_MINT_AMOUNT);
_mint(onBehalfOf, amountScaled);
```

Stored balance is `debt / variableBorrowIndex`; `balanceOf` multiplies back by the live
index. Interest accrual costs **zero storage writes** — the index moves, every balance
moves with it. `burn` (line 124) is the mirror image.

**Credit delegation** lives in `DebtTokenBase`: `_borrowAllowances`
(`.../tokenization/base/DebtTokenBase.sol:23`), `approveDelegation` (line 40), and
`_decreaseBorrowAllowance` (line 121). A delegator's collateral backs a delegatee's
borrow, with the debt token as the allowance ledger. This is simply impossible in v1,
where debt is a struct field.

**`StableDebtToken`** carries the real complexity. `balanceOf`
(`.../tokenization/StableDebtToken.sol:106`) compounds each user's own frozen rate from
their own timestamp:

```solidity
uint256 cumulatedInterest = MathUtils.calculateCompoundedInterest(stableRate, _timestamps[account]);
return accountBalance.rayMul(cumulatedInterest);
```

`mint` (line 136) must maintain **two** weighted averages at once. First the user's own
blended rate (line 156):

```
newUserRate = (oldUserRate × currentBalance + newRate × amount) / (currentBalance + amount)
```

then the reserve-wide average (line 168):

```
newAvgRate = (oldAvgRate × previousSupply + newRate × amount) / nextSupply
```

`_calculateBalanceIncrease` (line 264) capitalises accrued interest before either update,
and `_mint(onBehalfOf, amount.add(balanceIncrease), ...)` at line 174 writes principal
**and** interest into storage. Every stable-rate action is a storage write, which is
exactly what the variable token avoids.

The comment at line 205 is an honest admission of the design's cost:

> "Since the total supply and each single user debt accrue separately, there might be
> accumulation errors so that the last borrower repaying might actually try to repay more
> than the available debt supply."

Two independently-compounding aggregates can disagree. v2 handles it by clamping to zero
(line 209). v3.2 solved it by deleting stable debt.

### 2.4 Interest math: the binomial approximation

`MathUtils.calculateLinearInterest` (`.../libraries/math/MathUtils.sol:21`) is trivial:
`1 + rate × Δt / yr`.

`calculateCompoundedInterest` (line 45) is the interesting one — a **three-term binomial
expansion** of `(1 + r)^n`:

```solidity
uint256 expMinusOne = exp - 1;
uint256 expMinusTwo = exp > 2 ? exp - 2 : 0;
uint256 ratePerSecond = rate / SECONDS_PER_YEAR;
uint256 basePowerTwo = ratePerSecond.rayMul(ratePerSecond);
uint256 basePowerThree = basePowerTwo.rayMul(ratePerSecond);
uint256 secondTerm = exp.mul(expMinusOne).mul(basePowerTwo) / 2;
uint256 thirdTerm = exp.mul(expMinusOne).mul(expMinusTwo).mul(basePowerThree) / 6;
return WadRayMath.ray().add(ratePerSecond.mul(exp)).add(secondTerm).add(thirdTerm);
```

This is `1 + nx + C(n,2)x² + C(n,3)x³`, truncated. Since `x = rate/31536000` is around
`1e-9` for a 3% APR, the fourth term is negligible. It **always slightly underestimates**
true compounding, so the error favours borrowers by a rounding-dust amount — an
acceptable, deliberate trade for constant gas instead of v1's `rayPow` loop.

### 2.5 `ReserveLogic`: the accrual engine

**`updateState`** (`.../libraries/logic/ReserveLogic.sol:110`) is called at the top of
essentially every state-changing pool function:

```solidity
uint256 scaledVariableDebt = IVariableDebtToken(reserve.variableDebtTokenAddress).scaledTotalSupply();
uint256 previousVariableBorrowIndex = reserve.variableBorrowIndex;
uint256 previousLiquidityIndex = reserve.liquidityIndex;
uint40 lastUpdatedTimestamp = reserve.lastUpdateTimestamp;
(uint256 newLiquidityIndex, uint256 newVariableBorrowIndex) = _updateIndexes(...);
_mintToTreasury(...);
```

**`_updateIndexes`** (line 334): liquidity index grows linearly, variable index compounds,
both are capped to `uint128`, and the timestamp is stamped (line 370). Note the guard at
line 357 — the variable index only moves if variable debt actually exists, because the
liquidity rate might be coming entirely from stable loans.

**`_mintToTreasury`** (line 274) is v2's new revenue mechanism, and it is subtle. It
computes total debt accrued across both debt types since the last update…

```solidity
vars.totalDebtAccrued = vars.currentVariableDebt.add(vars.currentStableDebt)
    .sub(vars.previousVariableDebt).sub(vars.previousStableDebt);
vars.amountToMint = vars.totalDebtAccrued.percentMul(vars.reserveFactor);
if (vars.amountToMint != 0) {
    IAToken(reserve.aTokenAddress).mintToTreasury(vars.amountToMint, newLiquidityIndex);
}
```

…and mints that share **as aTokens to the treasury**. No underlying moves. The treasury
simply becomes a supplier whose claim was created out of interest that suppliers did not
receive — which is consistent because the liquidity rate was already reduced by the same
reserve factor (§2.6).

**`updateInterestRates`** (line 198) reads live debt totals and calls the strategy, then
writes all three rates. **`cumulateToLiquidityIndex`** (line 143) is v1's function
verbatim, now used for flash loan premiums.

### 2.6 The v2 rate strategy

`calculateInterestRates`
(`.../lendingpool/DefaultReserveInterestRateStrategy.sol:165`) is structurally identical
to v1's kinked model — same optimal-utilisation kink, same slopes, same lending-rate
oracle for the stable base. The one line that matters is 216:

```solidity
vars.currentLiquidityRate = _getOverallBorrowRate(
        totalStableDebt, totalVariableDebt,
        vars.currentVariableBorrowRate, averageStableBorrowRate)
    .rayMul(vars.utilizationRate)
    .percentMul(PercentageMath.PERCENTAGE_FACTOR.sub(reserveFactor));  // ← v2's addition
```

`supplyRate = weightedBorrowRate × utilisation × (1 − reserveFactor)`. Suppliers get the
rest; the difference is exactly what `_mintToTreasury` claims.

### 2.7 The pool functions, traced

#### `deposit(asset, amount, onBehalfOf, referralCode)` — `.../lendingpool/LendingPool.sol:104`

```
validateDeposit  →  updateState  →  updateInterestRates  →  transferFrom(user → aToken)
                 →  aToken.mint(onBehalfOf, amount, liquidityIndex)
                 →  if first deposit: userConfig.setUsingAsCollateral(reserve.id, true)
```

Compare to v1: no `LendingPoolCore` hop, funds go straight to the aToken, and the
collateral flag is a **bit** rather than a struct field.

#### `withdraw(asset, amount, to)` — line 142
`type(uint256).max` withdraws everything (line 155). `validateWithdraw`
(`.../libraries/logic/ValidationLogic.sol:60`) runs the full health-factor check.
Clearing the balance clears the collateral bit (line 174). `aToken.burn` transfers the
underlying out (`.../tokenization/AToken.sol:120`).

#### `borrow(...)` — line 201 → `_executeBorrow` — line 855
Prices the request in ETH (line 861), runs `validateBorrow`
(`.../libraries/logic/ValidationLogic.sol:120`), then `updateState`, then mints the
appropriate debt token — `StableDebtToken.mint` with the current stable rate, or
`VariableDebtToken.mint` with the index (lines 886–902). Sets the borrowing bit on first
borrow (line 904). `releaseUnderlying` is `false` when called from `flashLoan`, which is
how a flash loan converts into a debt position without moving funds twice (line 915).

#### `repay(asset, amount, rateMode, onBehalfOf)` — line 236
Reads both debts (line 244), clamps the payback (line 260), `updateState`, burns the
right debt token (lines 266–274), refreshes rates, clears the borrowing bit if all debt
is gone (line 279), pulls funds to the aToken (line 283) and calls `handleRepayment`
(line 285) — an empty hook in v2 (`.../tokenization/AToken.sol:323`) that later
implementations use.

#### `liquidationCall(...)` — line 425
Still a `delegatecall`, now into `LendingPoolCollateralManager` (line 436), still
returning `(code, message)`. Inside
(`.../lendingpool/LendingPoolCollateralManager.sol:81`):

- Close factor is now **bps**: `LIQUIDATION_CLOSE_FACTOR_PERCENT = 5000` (line 39).
- `_calculateAvailableCollateralToLiquidate` (line 272) uses `percentMul(liquidationBonus)`
  (line 300) instead of v1's `.mul(bonus).div(100)`.
- Debt burning prefers **variable first**, falling back to stable (lines 165–184) —
  a rule that only exists because two debt types exist.
- `receiveAToken` path sets the liquidator's collateral bit if this is their first
  aToken of that reserve (lines 193–200).
- **No origination-fee liquidation.** That entire v1 branch is gone.

#### `flashLoan(...)` — line 483
Now **multi-asset**. Transfers every requested asset (line 501), calls
`executeOperation` once with all arrays (line 510), then per asset (line 514) branches on
`modes[i]`:

- **mode 0 (NONE):** `updateState`, `cumulateToLiquidityIndex(totalSupply, premium)`,
  `updateInterestRates`, then pull `amount + premium` back (lines 521–538).
- **mode 1/2 (STABLE/VARIABLE):** `_executeBorrow(..., releaseUnderlying: false)` — the
  funds simply stay with the receiver and become a debt position for `onBehalfOf`
  (lines 539–553).

Mode 1/2 is powerful and was the enabling mechanism for the adapters below. It also
widened the attack surface (any flash loan can now end as debt against a delegator's
credit line), and v3 restricts it further.

#### `finalizeTransfer(...)` — line 739
Called **by the aToken** during `_transfer` (`.../tokenization/AToken.sol:387`). Verifies
the caller is the aToken (line 747), runs `validateTransfer` (health factor), then fixes
the collateral bits on both sides (lines 760–772). This is why v2 aTokens are freely
transferable while still being safe collateral.

### 2.8 Health factor in v2

`GenericLogic.calculateUserAccountData`
(`.../libraries/logic/GenericLogic.sol:150`) loops reserves but consults the user's bitmap
(lines 188, 202) to skip untouched ones. Collateral is weighted twice — once by LTV, once
by liquidation threshold (lines 196–199) — then normalised (line 216) and reduced to:

```solidity
function calculateHealthFactorFromBalances(uint256 totalCollateralInETH, uint256 totalDebtInETH,
    uint256 liquidationThreshold) internal pure returns (uint256) {
    if (totalDebtInETH == 0) return uint256(-1);
    return (totalCollateralInETH.percentMul(liquidationThreshold)).wadDiv(totalDebtInETH);
}
```
(`.../libraries/logic/GenericLogic.sol:242`)

Note `totalFeesETH` is gone — v1's health factor had to add origination fees to the debt
side (`v1-aave-protocol/.../LendingPoolDataProvider.sol:331`); v2 has no such fee.

Every guard lives in `ValidationLogic`: `validateDeposit` (line 41), `validateWithdraw`
(60), `validateBorrow` (120), `validateRepay` (223), `validateSwapRateMode` (259),
`validateRebalanceStableBorrowRate` (303), `validateSetUseReserveAsCollateral` (344),
`validateFlashloan` (379), `validateLiquidationCall` (392), `validateTransfer` (446).
Concentrating them in one library is what makes v3's risk features tractable to add.

### 2.9 Incentives

`IncentivizedERC20` calls `_getIncentivesController().handleAction(user, totalSupply, oldBalance)`
on every mint, burn and transfer
(`.../tokenization/IncentivizedERC20.sol:187, 189, 206, 222`). The controller reconstructs
each user's accrued rewards from the *old* balance and total supply. This is the direct
ancestor of v3's `RewardsController`, and it is why balance-changing paths must always
pass pre-change values.

### 2.10 The adapters: flash loans as a user feature

`contracts/adapters/` turns flash loans into one-click position management. The pattern
is always the same, e.g. `UniswapRepayAdapter.executeOperation`
(`.../adapters/UniswapRepayAdapter.sol:51`):

```solidity
require(msg.sender == address(LENDING_POOL), 'CALLER_MUST_BE_LENDING_POOL');
RepayParams memory decodedParams = _decodeParams(params);
_swapAndRepay(decodedParams.collateralAsset, assets[0], amounts[0], ...);
```

"Repay my USDC debt using my ETH collateral" becomes: flash-borrow USDC → repay the
user's debt → pull the user's aETH (they approved the adapter) → withdraw ETH → swap ETH
to USDC on Uniswap → return the flash loan. The user never needed the USDC.

Siblings: `UniswapLiquiditySwapAdapter` (swap collateral in place),
`FlashLiquidationAdapter` (liquidate without capital),
`ParaSwapLiquiditySwapAdapter` / `BaseParaSwapSellAdapter` (same, via ParaSwap).
`BaseUniswapAdapter` (566 lines) holds the shared pricing and swap plumbing.

### 2.11 Why v2 was replaced

| Problem | v3's answer |
|---|---|
| Risk is one number per asset (LTV/threshold) | **eMode** for correlated assets; **isolation mode** for risky ones |
| No size limits on a market | **supply caps** and **borrow caps** |
| A long-tail asset can be borrowed against anything | **siloed borrowing** |
| Stable rate keeps inverting against supply rate | stable rate **removed** in v3.2 |
| Still expensive (packed, but many external calls) | large gas rework; `L2Pool` + `CalldataLogic` for calldata-compressed L2 calls |
| One admin role | **ACLManager** with risk/emergency/pool admins |
| No cross-chain liquidity story | **Portal** |
| L2 sequencer downtime breaks liquidations | **price oracle sentinel** grace period |
| Bad debt has nowhere to live | explicit `deficit` accounting + Umbrella (v3.5/3.6) |

---

## 3. v1 → v2 → v3 in tables

### 3.1 Structural comparison

| Dimension | v1 | v2 | v3 (see `aave/AAVE-DEEP-DIVE.md`) |
|---|---|---|---|
| Fund custody | `LendingPoolCore` (all assets, one contract) | each `AToken` holds its own underlying | each `AToken`, plus `virtualUnderlyingBalance` accounting |
| State location | `LendingPoolCore` mappings | `LendingPool` (`_reserves`, `_usersConfig`) | `PoolStorage` |
| Logic location | inline in Pool + Core | linked libraries (`ReserveLogic`, `ValidationLogic`, `GenericLogic`) | logic libraries per action (`SupplyLogic`, `BorrowLogic`, `LiquidationLogic`, …) |
| aToken model | rebasing, **per-user index** | rebasing, **scaled balance** | scaled balance + `TokenMath` rounding rules |
| Debt model | struct fields | `StableDebtToken` + `VariableDebtToken` | `VariableDebtToken` only (stable removed in 3.2) |
| Credit delegation | ✗ | ✓ (`_borrowAllowances`) | ✓ + position manager (3.5+) |
| Reserve config | ~19 separate slots | 1 `uint256` bitmap (bits 0–79) | 1 `uint256` bitmap (bits 0–255, caps/eMode/isolation) |
| User config | `bool` per reserve struct | 2 bits per reserve | 2 bits per reserve |
| Compounding | `rayPow` exact loop | 3-term binomial | 3-term binomial |
| Protocol revenue | origination fee (25 bps) + flash protocol fee | **reserve factor** → treasury aTokens | reserve factor + liquidation protocol fee |
| Liquidation close factor | 50 (percent) | 5000 (bps) | 50% or 100% below an HF threshold (3.3) |
| Liquidation dispatch | `delegatecall` → `LendingPoolLiquidationManager` | `delegatecall` → `LendingPoolCollateralManager` | direct `LiquidationLogic` library call |
| Flash loans | single asset, fee to LPs + TokenDistributor | multi-asset, modes 0/1/2 | multi + `flashLoanSimple`, `flashLoanEnabled` per reserve |
| Interest redirection | ✓ (unique to v1) | ✗ | ✗ |
| Risk features | none | none | isolation, eMode, caps, siloed, sentinel |

### 3.2 What v3.0 added, and what came later

**Added in v3.0 (Mar 2022):** isolation mode (debt ceiling per collateral), eMode
(correlated-asset LTV boosts), supply/borrow caps, siloed borrowing, Portal
(cross-chain), price-oracle sentinel (L2 grace periods), ACLManager roles, `L2Pool` with
calldata compression, and a large gas reduction pass.

**Later:** v3.1 (flash loan fee split, oracle hardening), **v3.2 (stable rate removed;
eMode reworked into collateral/borrowable bitmaps)**, v3.3 (liquidation close-factor
rules, deficit accounting), v3.5 (`TokenMath` rounding overhaul), v3.6 (see
`aave/aave-v3-origin/CHANGELOG.md` and `docs/3.6/`).

For any of these, read `aave/AAVE-DEEP-DIVE.md`. For the Hub-and-Spoke redesign that
replaces the whole reserve model, read `aave/AAVE-V4-DEEP-DIVE.md`.

---

## 4. Security notes specific to v1 and v2

**1. v1's per-user index is a rounding surface.** `calculateCumulatedBalanceInternal`
(`v1/.../AToken.sol:522`) does `rayMul` then `rayDiv` per user, and materialises the
result by minting. Every interaction re-rounds. `getCompoundedBorrowBalance` explicitly
adds `1 wei` when rounding would produce zero interest
(`v1/.../CoreLibrary.sol:283`) — evidence the team hit this in practice. v3.5's
`TokenMath` exists to make all such rounding deliberately favour the protocol.

**2. The `delegatecall` liquidation pattern.** Both versions run the liquidation manager
inside the pool's storage (`v1/.../LendingPool.sol:815`,
`v2/.../LendingPool.sol:436`). The manager's storage layout **must** match the pool's
exactly — v2 enforces this by having `LendingPoolCollateralManager` inherit the same
`LendingPoolStorage`. Get the layout wrong on an upgrade and you corrupt the pool. v3
removed the pattern entirely.

**3. v1's core is a single point of failure.** All assets of all reserves sit in one
contract. v2's per-aToken custody is a genuine blast-radius reduction, and a large part
of why the refactor was worth doing.

**4. Stable rate manipulation.** A stable rate below the supply rate is a money loop.
v1's `rebalanceStableBorrowRate` (`v1/.../LendingPool.sol:741`) and the borrow-time
caps are mitigations, not fixes. The `avgStableRate` in v2's `StableDebtToken` is a
weighted average that can be shifted by a large borrow at an extreme rate, and the
"accumulation errors" comment at `v2/.../StableDebtToken.sol:205` is a live admission
that two independent accruals can disagree. Removed in v3.2.

**5. Flash loan modes 1 and 2 (v2).** `flashLoan` can end by opening debt on
`onBehalfOf` (`v2/.../LendingPool.sol:539`). Combined with credit delegation, the
allowance check in `_decreaseBorrowAllowance` is the only thing standing between a flash
loan and someone else's collateral. Delegate credit only to contracts you have read.

**6. Oracle dependence.** Both versions take a single price per asset from
`IPriceOracleGetter` inside the health-factor loop
(`v1/.../LendingPoolDataProvider.sol:114`, `v2/.../GenericLogic.sol:186`). No
staleness check, no circuit breaker, no sequencer check. Everything downstream —
borrow limits, liquidation eligibility, liquidation size — is only as good as that feed.
v3 adds the sentinel.

**7. First-deposit / empty-reserve edges.** Indexes start at `1e27`
(`v1/.../CoreLibrary.sol:171`). v1's `updateCumulativeIndexes` skips accrual entirely
when `totalBorrows == 0`, and v2's `_updateIndexes` skips when `currentLiquidityRate == 0`
(`v2/.../ReserveLogic.sol:347`). Both mean a reserve can sit with a static index for
long stretches. v3's virtual balances address the donation/inflation variants of this.

**8. `mintToTreasury` deliberately skips the zero check.** The comment at
`v2/.../AToken.sol:174` says a rounding-to-zero treasury mint is accepted as a tiny loss
rather than reverting a user's transaction. Correct call — but it is an explicit,
documented asymmetry, and worth knowing when auditing treasury accounting.

**9. Rebasing aTokens break naive integrations.** In both versions `balanceOf` grows with
no `Transfer` event. Any integrator caching a balance, or any accounting that assumes
`balanceOf` is a stored value, is wrong. This is precisely the motivation for v3's
`StataTokenV2` ERC4626 wrapper.

---

## 5. Exercises: trace these yourself

1. **Follow one wei of interest to the treasury in v2.** Start at
   `v2-protocol/contracts/protocol/libraries/logic/ReserveLogic.sol:110`, walk into
   `_updateIndexes` (line 334) and then `_mintToTreasury` (line 274). Then open
   `.../DefaultReserveInterestRateStrategy.sol:216` and prove that the aTokens minted to
   the treasury exactly equal the interest suppliers did *not* receive.

2. **Prove v1 and v2 aTokens are different animals.** Compare
   `v1-aave-protocol/contracts/tokenization/AToken.sol:338` with
   `v2-protocol/contracts/protocol/tokenization/AToken.sol:208`. Then answer: in each
   version, does a user's *stored* ERC20 balance change when nobody touches their
   position? Confirm with `cumulateBalanceInternal` (v1 line 452) and `mint` (v2 line 144).

3. **Trace interest redirection through a transfer.** Open
   `v1-aave-protocol/contracts/tokenization/AToken.sol:540` (`executeTransferInternal`)
   and `:479`. Work out what happens when A redirects to B, B redirects to C, and A
   transfers half their balance to D.

4. **Measure the compounding change.** Compare
   `v1-aave-protocol/contracts/libraries/CoreLibrary.sol:413` (with `WadRayMath.sol:72`)
   against `v2-protocol/contracts/protocol/libraries/math/MathUtils.sol:45`. For a 5% APR
   over 30 days, compute both results and the relative error. Which direction does the
   error favour?

5. **Follow the origination fee end to end.** `v1/.../FeeProvider.sol:40` →
   `v1/.../LendingPool.sol:442` → `v1/.../LendingPoolCore.sol:1342` →
   repayment at `v1/.../LendingPool.sol:572` → liquidation at
   `v1/.../LendingPoolLiquidationManager.sol:256`. Then confirm no equivalent exists
   anywhere in `v2-protocol`.

6. **Both liquidations, side by side.** Read
   `v1/.../LendingPoolLiquidationManager.sol:124` and
   `v2/.../LendingPoolCollateralManager.sol:81`. List every difference in ordering,
   units (percent vs bps), and the debt-burn priority rule in v2 at lines 165–184.

7. **Make credit delegation dangerous on paper.** Read
   `v2/.../tokenization/base/DebtTokenBase.sol:40` and `:121`, then
   `v2/.../LendingPool.sol:539`. Write out the exact sequence by which a delegatee
   converts a flash loan into permanent debt for a delegator, and identify the single
   check that bounds it.

8. **Count the storage reads.** For a `deposit` of an already-supplied asset, list every
   `SLOAD`/`SSTORE` in v1 (`LendingPool.sol:299` → `LendingPoolCore.sol:107`) versus v2
   (`LendingPool.sol:104` → `ReserveLogic.sol:110`). The bitmap in
   `DataTypes.sol:30` should account for most of the gap.
