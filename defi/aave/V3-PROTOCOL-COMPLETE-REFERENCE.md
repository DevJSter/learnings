# Aave v3 Core Protocol — Complete Reference

An exhaustive, function-by-function reference for **every contract and every
function** under `aave/aave-v3-origin/src/contracts/protocol/`, plus the
interfaces in `src/contracts/interfaces/` that define the protocol's external
surface.

Companion documents (do not duplicate; cite them):

- `aave/AAVE-DEEP-DIVE.md` — the conceptual walkthrough. Read that first if you
  want to *understand* Aave; read this one to have *seen all of it*.
- `aave/V3-PERIPHERY-COMPLETE-REFERENCE.md` — the non-protocol contracts
  (`misc/`, `helpers/`, `extensions/`, `rewards/`, `treasury/`, `instances/`,
  `deployments/`). Anything under those directories is out of scope here and is
  marked **[periphery]** where it is referenced.

Every `file:line` below was verified with `grep -n` against this exact tree.
Paths are relative to `aave/aave-v3-origin/`.

---

## ⚠️ Version: this tree is v3.7, not v3.6

`package.json:3` says `"version": "3.6.0"`, but that is a **stale changeset
version** — the `.changeset/` directory holds only `README.md` and `config.json`,
so the 3.7 release has not been versioned yet. The *code* is v3.7. Evidence:

| Check | Result | Implication |
|---|---|---|
| `src/contracts/instances/PoolInstance.sol:15` | `POOL_REVISION = 11` | 3.7 bumped 10 → 11 |
| `src/contracts/instances/PoolConfiguratorInstance.sol:12` | `CONFIGURATOR_REVISION = 8` | 3.7 bumped 7 → 8 |
| `grep -r priceOracleSentinel src/contracts/protocol` | 0 hits | 3.7 deleted the sentinel |
| `grep -r IsolationModeLogic` | 0 hits | 3.7 deleted the library |
| `grep -r EModeLogic` | 0 hits | 3.6 merged it into `SupplyLogic` |
| `grep -r setDebtCeiling` | 0 hits | 3.7 removed isolation mode |
| `grep -r dropReserve src/contracts/protocol` | 4 hits, **all in comments** | 3.7 removed it; comments point at `docs/3.7/drop-reserve-removal.md` |
| `DataTypes.sol:38-39` | `// DEPRECATED on v3.7.0` on `isolationModeTotalDebt` | explicit 3.7 marker |
| `Errors.sol:88` | `MustNotLeaveDust` present | added in 3.7 |
| `PercentageMath.sol:94` | `percentDivFloor` present | added in 3.7 |

So: **read `docs/3.7/Aave-v3.7-changelog.md` as the changelog for the code you
are looking at.** Throughout this document, features are tagged with the version
that introduced them, e.g. **[3.7]**.

### What v3.7 removed (and why you will see holes)

Isolation mode, siloed borrowing, the price-oracle sentinel and `dropReserve`
are all gone. They left permanent artefacts:

- **Bit holes in the reserve configuration word** (see §4.1). Bits 61, 62 and
  212–251 are unoccupied because removing a feature cannot reclaim storage bits
  without breaking every deployed reserve's config.
- **Deprecated struct fields** kept as `__deprecated*` placeholders so the
  storage layout of upgradeable contracts never shifts (see §3.1).
- **Legacy `reserveAddress != address(0)` guards** in four places
  (`GenericLogic.sol:92`, `LiquidationLogic.sol:652`, `Pool.sol:535`,
  `PoolLogic.sol:47`) that defend against gaps `dropReserve` could once leave in
  `_reservesList`. They are dead weight now but are cheap and safe.

This "deprecate in place, never re-pack" discipline is the single most important
thing to understand about reading Aave storage.

---

## Table of contents

1. [File inventory](#1-file-inventory)
2. [Architecture in one page](#2-architecture-in-one-page)
3. [Types: `DataTypes` and `ConfiguratorInputTypes`](#3-types)
   - [3.1 `ReserveData` / `ReserveDataLegacy`](#31-reservedata)
   - [3.2 `ReserveCache`](#32-reservecache)
   - [3.3 eMode structs](#33-emode-structs)
   - [3.4 Execution parameter structs](#34-execution-parameter-structs)
   - [3.5 `ConfiguratorInputTypes`](#35-configuratorinputtypes)
4. [Configuration bitmaps](#4-configuration-bitmaps)
   - [4.1 `ReserveConfiguration` — the 256-bit word](#41-reserveconfiguration)
   - [4.2 `UserConfiguration` — two bits per reserve](#42-userconfiguration)
   - [4.3 `EModeConfiguration`](#43-emodeconfiguration)
5. [Math libraries](#5-math-libraries)
   - [5.1 `WadRayMath`](#51-wadraymath)
   - [5.2 `PercentageMath`](#52-percentagemath)
   - [5.3 `MathUtils`](#53-mathutils)
   - [5.4 `TokenMath`](#54-tokenmath)
6. [`ReserveLogic` — index accrual](#6-reservelogic)
7. [`ValidationLogic`](#7-validationlogic)
8. [`GenericLogic` — health factor](#8-genericlogic)
9. [`SupplyLogic`](#9-supplylogic)
10. [`BorrowLogic`](#10-borrowlogic)
11. [`LiquidationLogic`](#11-liquidationlogic)
12. [`FlashLoanLogic`](#12-flashloanlogic)
13. [`PoolLogic`](#13-poollogic)
14. [`ConfiguratorLogic`](#14-configuratorlogic)
15. [`CalldataLogic` and `L2Pool`](#15-calldatalogic-and-l2pool)
16. [`Pool`](#16-pool)
17. [`PoolConfigurator`](#17-poolconfigurator)
18. [Tokenization](#18-tokenization)
19. [Configuration contracts](#19-configuration-contracts)
20. [Interfaces](#20-interfaces)
21. [Reference tables](#21-reference-tables)
22. [Use-case index](#22-use-case-index)

---

## 1. File inventory

37 Solidity files under `src/contracts/protocol/`, 22 under
`src/contracts/interfaces/`. Every one is covered below; the section column says
where.

### `protocol/pool/` (4 files)

| File | Lines | § |
|---|---|---|
| `pool/Pool.sol` | 941 | [16](#16-pool) |
| `pool/PoolConfigurator.sol` | 622 | [17](#17-poolconfigurator) |
| `pool/PoolStorage.sol` | 57 | [16.1](#161-poolstorage) |
| `pool/L2Pool.sol` | 101 | [15](#15-calldatalogic-and-l2pool) |

### `protocol/libraries/logic/` (10 files)

| File | Lines | § |
|---|---|---|
| `logic/LiquidationLogic.sol` | 679 | [11](#11-liquidationlogic) |
| `logic/ValidationLogic.sol` | 550 | [7](#7-validationlogic) |
| `logic/SupplyLogic.sol` | 337 | [9](#9-supplylogic) |
| `logic/ReserveLogic.sol` | 275 | [6](#6-reservelogic) |
| `logic/GenericLogic.sol` | 258 | [8](#8-genericlogic) |
| `logic/FlashLoanLogic.sol` | 253 | [12](#12-flashloanlogic) |
| `logic/CalldataLogic.sol` | 233 | [15](#15-calldatalogic-and-l2pool) |
| `logic/BorrowLogic.sol` | 224 | [10](#10-borrowlogic) |
| `logic/PoolLogic.sol` | 194 | [13](#13-poollogic) |
| `logic/ConfiguratorLogic.sol` | 191 | [14](#14-configuratorlogic) |

> `EModeLogic.sol` and `IsolationModeLogic.sol` **do not exist** in this tree.
> eMode entry moved into `SupplyLogic.executeSetUserEMode` **[3.6]**; isolation
> mode was deleted **[3.7]**.

### `protocol/libraries/math/`, `configuration/`, `types/`, `helpers/` (9 files)

| File | Lines | § |
|---|---|---|
| `configuration/ReserveConfiguration.sol` | 455 | [4.1](#41-reserveconfiguration) |
| `libraries/types/DataTypes.sol` | 328 | [3](#3-types) |
| `configuration/UserConfiguration.sol` | 194 | [4.2](#42-userconfiguration) |
| `math/WadRayMath.sol` | 167 | [5.1](#51-wadraymath) |
| `math/PercentageMath.sol` | 120 | [5.2](#52-percentagemath) |
| `math/MathUtils.sol` | 116 | [5.3](#53-mathutils) |
| `helpers/TokenMath.sol` | 114 | [5.4](#54-tokenmath) |
| `helpers/Errors.sol` | 95 | [21.4](#214-complete-errors-table) |
| `configuration/EModeConfiguration.sol` | 53 | [4.3](#43-emodeconfiguration) |
| `libraries/types/ConfiguratorInputTypes.sol` | 32 | [3.5](#35-configuratorinputtypes) |

### `protocol/tokenization/` (10 files)

| File | Lines | § |
|---|---|---|
| `tokenization/delegation/BaseDelegation.sol` | 441 | [18.7](#187-basedelegation) |
| `tokenization/AToken.sol` | 338 | [18.3](#183-atoken) |
| `tokenization/base/IncentivizedERC20.sol` | 324 | [18.1](#181-incentivizederc20) |
| `tokenization/VariableDebtToken.sol` | 196 | [18.5](#185-variabledebttoken) |
| `tokenization/base/ScaledBalanceTokenBase.sol` | 135 | [18.2](#182-scaledbalancetokenbase) |
| `tokenization/ATokenWithDelegation.sol` | 129 | [18.6](#186-atokenwithdelegation) |
| `tokenization/base/DebtTokenBase.sol` | 121 | [18.4](#184-debttokenbase) |
| `tokenization/delegation/interfaces/IBaseDelegation.sol` | 111 | [18.7](#187-basedelegation) |
| `tokenization/base/EIP712Base.sol` | 70 | [18.0](#180-eip712base) |
| `tokenization/base/MintableIncentivizedERC20.sol` | 64 | [18.2](#182-scaledbalancetokenbase) |
| `tokenization/base/DelegationMode.sol` | 9 | [18.6](#186-atokenwithdelegation) |

### `protocol/configuration/` (3 files)

| File | Lines | § |
|---|---|---|
| `configuration/PoolAddressesProvider.sol` | 209 | [19.1](#191-pooladdressesprovider) |
| `configuration/ACLManager.sol` | 133 | [19.2](#192-aclmanager) |
| `configuration/PoolAddressesProviderRegistry.sol` | 104 | [19.3](#193-pooladdressesproviderregistry) |

### `src/contracts/interfaces/` (22 files)

Covered in [§20](#20-interfaces).

---

## 2. Architecture in one page

Aave v3 is a **thin entry-point contract that delegates to linked libraries**.
`Pool` holds all the storage and all the access control; it contains almost no
business logic. Each user action is forwarded to a logic library that operates
directly on `Pool`'s storage.

```
                        ┌────────────────────────────┐
                        │   PoolAddressesProvider    │  registry of everything
                        │  (getPool, getACLManager,  │  owns the proxies
                        │   getPriceOracle, ...)     │
                        └──────────┬─────────────────┘
                                   │ resolves
        ┌──────────────────────────┼───────────────────────────┐
        │                          │                           │
   ┌────▼─────┐            ┌───────▼────────┐          ┌───────▼──────┐
   │ACLManager│            │ Pool (proxy)   │          │ PoolConfig-  │
   │ roles    │◄───checks──┤ PoolStorage    ├─────────►│ urator       │
   └──────────┘            │ _reserves      │  admin   │ (proxy)      │
                           │ _usersConfig   │  calls   └──────┬───────┘
                           │ _reservesList  │                 │
                           │ _eModeCategories│                │ delegates to
                           └───┬────────────┘                 ▼
                               │ delegatecall-linked   ┌──────────────────┐
                               │ (external libraries)  │ ConfiguratorLogic│
        ┌──────────────────────┼──────────────────┐    └──────────────────┘
        ▼          ▼           ▼         ▼        ▼
   SupplyLogic BorrowLogic Liquidation FlashLoan PoolLogic
        │          │        Logic       Logic       │
        └────┬─────┴──────────┬───────────┬─────────┘
             │ all of them use│           │
             ▼                ▼           ▼
       ReserveLogic     ValidationLogic  GenericLogic
       (index accrual)  (all requires)   (health factor)
             │                                │
             │ reads/writes                   │ reads prices
             ▼                                ▼
   ┌──────────────────┐                 ┌───────────────┐
   │ AToken (proxy)   │                 │ AaveOracle    │ [periphery]
   │ VariableDebtToken│                 │ (Chainlink)   │
   │  (proxy)         │                 └───────────────┘
   │  ─ hold funds    │
   │  ─ scaled bal.   │
   └──────────────────┘
```

**The three invariants that explain every design decision:**

1. **Balances are stored scaled.** A user's aToken balance is
   `scaledBalance × liquidityIndex`. Interest accrues by moving one global
   index, never by touching user storage. See [§5.4](#54-tokenmath) and
   [§18.2](#182-scaledbalancetokenbase).
2. **Every rounding goes the protocol's way.** `TokenMath` **[3.5]** fixes each
   operation's direction: mint down, burn up, debt up. See
   [§5.4](#54-tokenmath).
3. **`updateState` before you read, `updateInterestRates` after you write.**
   Every logic function follows this sandwich. See [§6](#6-reservelogic).

### The library-linking mechanism

`SupplyLogic`, `BorrowLogic`, `LiquidationLogic`, `FlashLoanLogic` and
`PoolLogic` declare their entry points `external`, which makes the compiler emit
them as **separately deployed libraries** that `Pool` reaches by `delegatecall`.
That is why they can write `Pool`'s storage: `delegatecall` runs their code in
`Pool`'s storage context.

`ReserveLogic`, `ValidationLogic`, `GenericLogic` and all of `math/`,
`configuration/`, `helpers/` are `internal`, so they are **inlined** into
whatever calls them and cost no extra call.

**[3.7]** `ConfiguratorLogic` changed from `external` to `internal` — it is no
longer deployed separately, so a v3.7 deployment links one fewer library.

Consequence for deployers: the five external libraries must be deployed first
and their addresses supplied at `Pool` compile/link time. This is what
`AaveV3LibrariesBatch1` **[periphery]** does.

---

## 3. Types

`libraries/types/DataTypes.sol` (328 lines) is a pure struct library — no
functions. It is the vocabulary the whole protocol speaks.

### 3.1 `ReserveData`

`DataTypes.sol:42-79`. One of these per listed asset, in `Pool`'s `_reserves`
mapping. **Field order is storage layout** — it can never be reordered, only
deprecated in place.

| # | Field | Type | Unit | Meaning |
|---|---|---|---|---|
| 0 | `configuration` | `ReserveConfigurationMap` | bitmap | the 256-bit risk-parameter word, [§4.1](#41-reserveconfiguration) |
| 1 | `liquidityIndex` | `uint128` | ray | cumulative supply interest. Starts at `1e27`, only grows. Multiply a scaled aToken balance by this to get the real balance |
| 2 | `currentLiquidityRate` | `uint128` | ray/yr | current APR paid to suppliers |
| 3 | `variableBorrowIndex` | `uint128` | ray | cumulative borrow interest. Starts at `1e27`, only grows, always ≥ `liquidityIndex` |
| 4 | `currentVariableBorrowRate` | `uint128` | ray/yr | current APR charged to borrowers |
| 5 | `deficit` | `uint128` | underlying | **[3.3]** bad debt recognised on this reserve. Reuses the old `stableBorrowRate` slot (`:53-55`) |
| 6 | `lastUpdateTimestamp` | `uint40` | seconds | when the indexes were last accrued |
| 7 | `id` | `uint16` | index | position in `_reservesList`; also the bit position in every bitmap |
| 8 | `liquidationGracePeriodUntil` | `uint40` | timestamp | **[3.1]** liquidations blocked until this time |
| 9 | `aTokenAddress` | `address` | — | the aToken proxy; **holds the underlying** |
| 10 | `__deprecatedStableDebtTokenAddress` | `address` | — | **removed [3.2]** |
| 11 | `variableDebtTokenAddress` | `address` | — | the debt token proxy |
| 12 | `__deprecatedInterestRateStrategyAddress` | `address` | — | **removed [3.4]**; use `Pool.RESERVE_INTEREST_RATE_STRATEGY` |
| 13 | `accruedToTreasury` | `uint128` | scaled | treasury's share, in scaled aToken units, not yet minted |
| 14 | `virtualUnderlyingBalance` | `uint128` | underlying | **[3.4]** the protocol's own accounting of held underlying. Occupies the old `unbacked` slot (`:72`) |
| 15 | `__deprecatedIsolationModeTotalDebt` | `uint128` | — | **removed [3.7]** |
| 16 | `__deprecatedVirtualUnderlyingBalance` | `uint128` | — | **removed [3.4]**, moved to field 14 to share a slot with `accruedToTreasury` |

**Why `virtualUnderlyingBalance` exists.** Utilization must not be computable
from `IERC20(asset).balanceOf(aToken)`, because anyone can donate tokens to the
aToken and move the interest rate. `virtualUnderlyingBalance` is incremented and
decremented only by protocol actions (`ReserveLogic.sol:161,164`), so a donation
changes nothing. This is Aave's answer to the same donation problem Uniswap
solves with `MINIMUM_LIQUIDITY`.

**`ReserveDataLegacy`** (`DataTypes.sol:9-40`) is the *old* shape, returned by
`Pool.getReserveData` so that pre-3.4 integrations keep compiling. It omits
`virtualUnderlyingBalance` and still exposes `currentStableBorrowRate` and
`isolationModeTotalDebt` as zeros. See [§16.9](#169-view-functions).

### 3.2 `ReserveCache`

`DataTypes.sol:159-173`. A memory struct built once per action by
`ReserveLogic.cache` and threaded through the whole call. It exists purely to
avoid re-reading storage and re-calling `scaledTotalSupply()`.

The `curr*`/`next*` pairs are the key idea: `curr` is the value as stored when
the action began, `next` is the value after accrual. `_updateIndexes` writes
`next*`; everything downstream reads `next*`.

| Field | Meaning |
|---|---|
| `currScaledVariableDebt` / `nextScaledVariableDebt` | `IVariableDebtToken.scaledTotalSupply()` before / after this action's mint or burn |
| `currLiquidityIndex` / `nextLiquidityIndex` | supply index before / after accrual |
| `currVariableBorrowIndex` / `nextVariableBorrowIndex` | borrow index before / after accrual |
| `currLiquidityRate` / `currVariableBorrowRate` | the rates that applied over the elapsed period |
| `reserveFactor` | cached from the config word |
| `reserveConfiguration` | the whole config word, copied to memory |
| `aTokenAddress`, `variableDebtTokenAddress` | cached token addresses |
| `reserveLastUpdateTimestamp` | when accrual last ran |

### 3.3 eMode structs

**`EModeCategory`** (`DataTypes.sol:141-151`) — the live struct, in
`_eModeCategories`:

| Field | Type | Meaning |
|---|---|---|
| `ltv`, `liquidationThreshold`, `liquidationBonus` | `uint16` | bps risk params that *replace* the reserve's own when the user is in this category |
| `collateralBitmap` | `uint128` | **[3.2]** bit `i` set ⇒ reserve `i` may be collateral in this category |
| `isolated` | `bool` | **[3.7]** if true, assets **not** in `collateralBitmap` get ltv0 rules |
| `label` | `string` | **deprecated [3.6]** — do not build UI on it |
| `borrowableBitmap` | `uint128` | **[3.2]** bit `i` set ⇒ reserve `i` may be borrowed in this category |
| `ltvzeroBitmap` | `uint128` | **[3.6]** bit `i` set ⇒ reserve `i` is treated as ltv0 *inside this category only* |

`ltvzeroBitmap` is the whole point of the **[3.6]** eMode rework. Before it, an
asset with `ltv = 0` in the base reserve config was ltv0 everywhere, so you could
not offboard an asset outside eMode while keeping it usable inside one. Now
LTV, liquidation threshold and borrowability are fully decoupled between
`eMode = 0` and `eMode ≠ 0`.

**`EModeCategoryBaseConfiguration`** (`:133-139`) is the setter-side struct
(`ltv`, `lt`, `lb`, `isolated`, `label`). **`CollateralConfig`** (`:127-131`) is
the trimmed triple returned by health-factor code.
**`EModeCategoryLegacy`** (`:117-125`) is the pre-3.2 shape kept for old
integrations, including a `priceSource` field that has been dead since 3.2.

### 3.4 Execution parameter structs

Every logic-library entry point takes one of these instead of a long argument
list. That is not style: `Pool` is at the edge of the stack-depth limit, and a
struct is one stack slot.

| Struct | Line | Consumed by |
|---|---|---|
| `ExecuteSupplyParams` | `:187` | `SupplyLogic.executeSupply` |
| `ExecuteWithdrawParams` | `:222` | `SupplyLogic.executeWithdraw` |
| `ExecuteBorrowParams` | `:197` | `BorrowLogic.executeBorrow` |
| `ExecuteRepayParams` | `:210` | `BorrowLogic.executeRepay` |
| `ExecuteLiquidationCallParams` | `:175` | `LiquidationLogic.executeLiquidationCall` |
| `ExecuteEliminateDeficitParams` | `:232` | `LiquidationLogic.executeEliminateDeficit` |
| `FinalizeTransferParams` | `:239` | `SupplyLogic.executeFinalizeTransfer` |
| `FlashloanParams` | `:249` | `FlashLoanLogic.executeFlashLoan` |
| `FlashloanSimpleParams` | `:265` | `FlashLoanLogic.executeFlashLoanSimple` |
| `FlashLoanRepaymentParams` | `:276` | `FlashLoanLogic._handleFlashLoanRepayment` |
| `CalculateUserAccountDataParams` | `:286` | `GenericLogic.calculateUserAccountData` |
| `ValidateBorrowParams` | `:293` | `ValidationLogic.validateBorrow` |
| `ValidateLiquidationCallParams` | `:301` | `ValidationLogic.validateLiquidationCall` |
| `CalculateInterestRatesParams` | `:309` | the rate strategy **[periphery]** |
| `InitReserveParams` | `:321` | `PoolLogic.executeInitReserve` |

Two fields worth flagging:

- `ExecuteBorrowParams.releaseUnderlying` (`:205`) — `true` for a normal borrow
  (send tokens to the user), `false` when a flash loan opens debt (the user
  already holds the tokens).
- `FlashloanParams.isAuthorizedFlashBorrower` (`:262`) — set from
  `ACLManager.isFlashBorrower`; authorised borrowers pay zero premium.
- `CalculateInterestRatesParams.unbacked` (`:310`) — despite the name, since
  **[3.3]** `ReserveLogic` passes `reserve.deficit` here (`ReserveLogic.sol:146`).
  The field name is a fossil.

### 3.5 `ConfiguratorInputTypes`

`libraries/types/ConfiguratorInputTypes.sol` (32 lines), three structs, no
functions:

- `InitReserveInput` (`:5-15`) — everything needed to list an asset: both token
  implementations, the underlying, names/symbols for both tokens, `params` for
  the token initializers, and `interestRateData` forwarded to the rate strategy.
- `UpdateATokenInput` (`:17-23`) and `UpdateDebtTokenInput` (`:25-31`) — proxy
  upgrade payloads: `asset`, new `name`/`symbol`, new `implementation`, `params`.

---

## 4. Configuration bitmaps

### 4.1 `ReserveConfiguration`

`libraries/configuration/ReserveConfiguration.sol` (455 lines). Packs every risk
parameter of a reserve into **one 256-bit word**, so reading a reserve's whole
risk profile is one `SLOAD`.

#### The complete bit map

```
 bit  255 ...  253  252  251 ......... 212  211 ..... 176  175 ... 168  167 .. 152
      ┌─────────┬────┬────────────────────┬──────────────┬───────────┬──────────┐
      │ unused  │VIRT│  HOLE (debtCeiling)│ HOLE(unbacked│HOLE(eMode │ liq.protoc│
      │  (3)    │ACC │   [removed 3.7]    │ MintCap)[3.4]│Cat.)[3.2] │ ol fee(16)│
      └─────────┴────┴────────────────────┴──────────────┴───────────┴──────────┘

 bit  151 ......... 116  115 .......... 80  79 ..... 64  63   62   61   60
      ┌──────────────────┬─────────────────┬───────────┬────┬────┬────┬────┐
      │  supply cap (36) │  borrow cap (36)│ res.factor│FLASH│HOLE│HOLE│PAUSE│
      │                  │                 │   (16)    │LOAN│silo│isol│  D  │
      └──────────────────┴─────────────────┴───────────┴────┴────┴────┴────┘

 bit   59   58    57    56  55 ..... 48  47 ....... 32  31 ....... 16  15 ..... 0
      ┌────┬─────┬─────┬────┬──────────┬───────────────┬──────────────┬─────────┐
      │HOLE│BORRO│FROZE│ACTI│ decimals │ liq.bonus (16)│ liq.thresh(16)│ LTV (16)│
      │stbl│WING │  N  │ VE │   (8)    │               │              │         │
      └────┴─────┴─────┴────┴──────────┴───────────────┴──────────────┴─────────┘
```

#### Field table

| Bits | Field | Mask constant (`:line`) | Start-bit constant | Getter / Setter | Max valid |
|---|---|---|---|---|---|
| 0–15 | LTV (bps) | `LTV_MASK` `:13` | — (0) | `getLtv` `:82` / `setLtv` `:71` | `MAX_VALID_LTV = 65535` `:55` |
| 16–31 | Liquidation threshold (bps) | `LIQUIDATION_THRESHOLD_MASK` `:14` | `16` `:35` | `getLiquidationThreshold` `:107` / `setLiquidationThreshold` `:91` | `65535` `:56` |
| 32–47 | Liquidation bonus (bps, `>10000`) | `LIQUIDATION_BONUS_MASK` `:15` | `32` `:36` | `getLiquidationBonus` `:134` / `setLiquidationBonus` `:118` | `65535` `:57` |
| 48–55 | Underlying decimals | `DECIMALS_MASK` `:16` | `48` `:37` | `getDecimals` `:159` / `setDecimals` `:145` | `255` `:58` |
| 56 | Active | `ACTIVE_MASK` `:17` | `56` `:38` | `getActive` `:181` / `setActive` `:170` | — |
| 57 | Frozen | `FROZEN_MASK` `:18` | `57` `:39` | `getFrozen` `:201` / `setFrozen` `:190` | — |
| 58 | Borrowing enabled | `BORROWING_MASK` `:19` | `58` `:40` | `getBorrowingEnabled` `:244` / `setBorrowingEnabled` `:230` | — |
| **59** | **HOLE** — was `stableRateBorrowingEnabled` | — (`:20` comment) | — | — | **removed [3.2]** |
| 60 | Paused | `PAUSED_MASK` `:21` | `60` `:41` | `getPaused` `:221` / `setPaused` `:210` | — |
| **61** | **HOLE** — was `borrowableInIsolation` | — (`:22` comment) | — | — | **removed [3.7]** |
| **62** | **HOLE** — was `siloedBorrowing` | — (`:22` comment) | — | — | **removed [3.7]** |
| 63 | Flashloan enabled | `FLASHLOAN_ENABLED_MASK` `:23` | `63` `:44` | `getFlashLoanEnabled` `:377` / `setFlashLoanEnabled` `:363` | — |
| 64–79 | Reserve factor (bps) | `RESERVE_FACTOR_MASK` `:24` | `64` `:45` | `getReserveFactor` `:271` / `setReserveFactor` `:255` | `65535` `:59` |
| 80–115 | Borrow cap (whole tokens, 0 = none) | `BORROW_CAP_MASK` `:25` | `80` `:46` | `getBorrowCap` `:296` / `setBorrowCap` `:282` | `68719476735` (2³⁶−1) `:60` |
| 116–151 | Supply cap (whole tokens, 0 = none) | `SUPPLY_CAP_MASK` `:26` | `116` `:47` | `getSupplyCap` `:321` / `setSupplyCap` `:307` | `68719476735` `:61` |
| 152–167 | Liquidation protocol fee (bps) | `LIQUIDATION_PROTOCOL_FEE_MASK` `:27` | `152` `:48` | `getLiquidationProtocolFee` `:351` / `setLiquidationProtocolFee` `:332` | `65535` `:62` |
| **168–175** | **HOLE** — was `eModeCategory` | — (`:28`) | — | — | **removed [3.2]** |
| **176–211** | **HOLE** — was `unbackedMintCap` | — (`:29`) | — | — | **removed [3.4]** |
| **212–251** | **HOLE** — was `debtCeiling` | — (`:30`) | — | — | **removed [3.7]** |
| 252 | Virtual accounting active | `VIRTUAL_ACC_ACTIVE_MASK` `:32` | `252` `:53` | — / `setVirtualAccActive` `:389` | **deprecated [3.4]** — always set, no getter |
| 253–255 | unused | — | — | — | — |

`MAX_RESERVES_COUNT = 128` (`:64`) is `public`, so it is readable on-chain. 128
is not arbitrary: `UserConfigurationMap` spends 2 bits per reserve in a
`uint256`, so 128 reserves is exactly full.

#### Batch getters

Three functions read several fields from one `dataLocal` copy, saving repeated
memory reads:

- **`getFlags`** (`:403-414`) → `(active, frozen, borrowingEnabled, paused)`.
- **`getParams`** (`:425-437`) → `(ltv, liquidationThreshold, liquidationBonus, decimals, reserveFactor)`.
- **`getCaps`** (`:445-454`) → `(borrowCap, supplyCap)`.

Every setter is `internal pure` on a **`memory`** struct — it mutates a copy.
The caller must write it back with `Pool.setConfiguration`. Every setter
validates its range and reverts with the matching `Errors.Invalid*`; e.g.
`setLtv` (`:71-75`) does `require(ltv <= MAX_VALID_LTV, Errors.InvalidLtv())`.

`setVirtualAccActive` (`:389-393`) has **no getter** — **[3.4]** made virtual
accounting universal, so the bit is written for backward compatibility with
integrations that still read it directly, and never read by the protocol.

### 4.2 `UserConfiguration`

`libraries/configuration/UserConfiguration.sol` (194 lines). One `uint256` per
user, **2 bits per reserve**, indexed by `ReserveData.id`.

> **⚠️ The NatSpec is backwards.** `DataTypes.sol:109-111` says "The first bit
> indicates if an asset is used as collateral, the second whether an asset is
> borrowed." The code says the opposite:
> `setBorrowing` uses `bit = 1 << (reserveIndex << 1)` (`:36`) — the **even**
> bit — and `setUsingAsCollateral` uses `bit = 1 << ((reserveIndex << 1) + 1)`
> (`:63`) — the **odd** bit. So for reserve `i`: **bit `2i` = borrowing, bit
> `2i+1` = collateral.** Trust the code.

```
 bit:  ... │  5  │  4  │  3  │  2  │  1  │  0  │
           │ col │ bor │ col │ bor │ col │ bor │
           │  #2 │  #2 │  #1 │  #1 │  #0 │  #0 │
```

| Function | Line | Purpose |
|---|---|---|
| `setBorrowing(self, reserveIndex, borrowing)` | `:28-43` | set/clear the even bit. `storage` — writes directly |
| `setUsingAsCollateral(self, reserveIndex, asset, user, usingAsCollateral)` | `:53-72` | set/clear the odd bit **and emit** `ReserveUsedAsCollateralEnabled`/`Disabled` (`:66`, `:69`) |
| `isBorrowing(self, reserveIndex)` | `:80` | test even bit |
| `isUsingAsCollateral(self, reserveIndex)` | `:96` | test odd bit |
| `isUsingAsCollateralOne(self)` | `:112` | exactly one collateral bit set |
| `isUsingAsCollateralAny(self)` | `:124` | any collateral bit set |
| `isBorrowingOne(self)` | `:136` | exactly one borrow bit set |
| `isBorrowingAny(self)` | `:146` | any borrow bit set |
| `isEmpty(self)` | `:155` | the whole word is zero |
| `getNextFlags(uint256 data)` | `:168` | pop the low pair, return `(remaining, isBorrowing, isCollateral)` — the iteration primitive used by `GenericLogic` |
| `_getFirstAssetIdByMask(self, mask)` | `:179` | index of the lowest set bit under a mask |

Both setters `require(reserveIndex < MAX_RESERVES_COUNT, Errors.InvalidReserveIndex())`
(`:34`, `:61`) and run in `unchecked` blocks — the shift cannot overflow given
that bound.

`COLLATERAL_MASK` is `0xAAAA...AA` (`:20`, the odd bits) — confirming the layout.
`isUsingAsCollateralOne` uses the standard `x & (x-1) == 0 && x != 0`
power-of-two test against that mask.

**Why a bitmap at all:** the health-factor loop
([§8](#8-genericlogic)) must visit only the reserves a user actually touches. A
128-entry array scan would cost a fortune; one word plus `getNextFlags` costs
almost nothing.

### 4.3 `EModeConfiguration`

`libraries/configuration/EModeConfiguration.sol` (53 lines), two functions,
operating on the `uint128` bitmaps inside `EModeCategory`.

- **`setReserveBitmapBit(bitmap, reserveIndex, enabled)`** (`:21-36`) — returns a
  new bitmap with bit `reserveIndex` set or cleared. `require(reserveIndex <
  MAX_RESERVES_COUNT, Errors.InvalidReserveIndex())` (`:27`).
- **`isReserveEnabledOnBitmap(bitmap, reserveIndex)`** (`:44-52`) — `(bitmap >>
  reserveIndex) & 1 != 0`, same bound check (`:49`).

Both are `pure` and take/return the bitmap by value; the caller writes it back.
Used for `collateralBitmap`, `borrowableBitmap` and `ltvzeroBitmap`.

> **Latent bound mismatch.** `MAX_RESERVES_COUNT` is 128 and the bitmaps are
> `uint128`, so index 127 is the last valid bit — consistent. But `uint128(1 <<
> reserveIndex)` at `:29` computes the shift in `uint256` and then narrows; the
> `require` above it is what keeps that safe. The `forge-lint` disable comment on
> `:28` acknowledges the pattern is only safe because of the preceding check.

---

## 5. Math libraries

Three fixed-point scales are in play:

| Name | Factor | Used for |
|---|---|---|
| **bps** (`PERCENTAGE_FACTOR`) | `1e4` | LTV, thresholds, bonuses, reserve factor, close factor |
| **wad** | `1e18` | health factor, generic 18-dec quantities |
| **ray** | `1e27` | indexes and interest rates — everything that compounds |

Ray exists because indexes multiply thousands of times over a reserve's life;
27 digits keeps the accumulated rounding error irrelevant.

### 5.1 `WadRayMath`

`libraries/math/WadRayMath.sol` (167 lines). All functions `internal pure`, all
implemented in Yul for gas. The header (`:10-11`) states the rule: **default
operations round half-up**, and `Floor`/`Ceil` variants exist where the
direction must be pinned.

| Function | Line | Rounding | Formula |
|---|---|---|---|
| `wadMul(a,b)` | `:35` | half-up | `(a*b + 0.5e18) / 1e18` |
| `wadDiv(a,b)` | `:53` | half-up | `(a*1e18 + b/2) / b` |
| `rayMul(a,b)` | `:64` | half-up | `(a*b + 0.5e27) / 1e27` |
| `rayMulFloor(a,b)` | `:74` | down | `(a*b) / 1e27` |
| `rayMulCeil(a,b)` | `:85` | up | `(a*b)/1e27 + (mod != 0)` |
| `rayDiv(a,b)` | `:104` | half-up | `(a*1e27 + b/2) / b` |
| `rayDivCeil(a,b)` | `:114` | up | `(a*1e27)/b + (mod != 0)` |
| `rayDivFloor(a,b)` | `:125` | down | `(a*1e27) / b` |
| `rayToWad(a)` | `:141` | half-up | `a / 1e9`, `+1` if remainder ≥ `5e8` |
| `wadToRay(a)` | `:157` | exact | `a * 1e9`, reverts on overflow |

**The half-up trick.** `rayMul` is `(a*b + HALF_RAY) / RAY`. Adding half the
divisor before an integer division is the standard round-half-up: if the true
fractional part is ≥ 0.5, adding 0.5 carries into the integer part.

**The ceil trick.** `rayMulCeil` (`:92-93`) computes
`div(product, RAY)` then adds `iszero(iszero(mod(product, RAY)))`. In Yul,
`iszero(iszero(x))` is the idiomatic "x != 0 ? 1 : 0", so this is
"add one if the division was not exact".

**Overflow guards.** Every function reverts *before* multiplying, by checking
that `a` is small enough. `rayMul` (`:67`) checks
`a <= (type(uint256).max - HALF_RAY) / b`, i.e. the product plus the half-add
must fit. The `Floor`/`Ceil` variants (`:77`, `:88`) drop `HALF_RAY` from the
bound because they do not add it. `rayDivCeil`/`rayDivFloor` (`:117`, `:128`)
instead check `a <= type(uint256).max / RAY` because they scale up first. All
revert with bare `revert(0,0)` — **no error data**, so a revert here surfaces as
an empty revert, which is a real debugging trap.

`wadDiv` and `rayDiv` also `iszero(b)`-check, so division by zero reverts
empty rather than panicking.

### 5.2 `PercentageMath`

`libraries/math/PercentageMath.sol` (120 lines). `PERCENTAGE_FACTOR = 1e4`
(`:13`), `HALF_PERCENTAGE_FACTOR = 0.5e4` (`:16`).

| Function | Line | Rounding | Formula |
|---|---|---|---|
| `percentMul(v,p)` | `:25` | half-up | `(v*p + 0.5e4) / 1e4` |
| `percentMulCeil(v,p)` | `:41` | up | `(v*p)/1e4 + (mod != 0)` |
| `percentMulFloor(v,p)` | `:59` | down | `(v*p) / 1e4` |
| `percentDiv(v,p)` | `:80` | half-up | `(v*1e4 + p/2) / p` |
| `percentDivFloor(v,p)` | `:94` | down | **[3.7]** `(v*1e4) / p` |
| `percentDivCeil(v,p)` | `:107` | up | `(v*1e4)/p + (mod != 0)` |

`percentDivFloor` was **added in [3.7]** specifically so
`LiquidationLogic._calculateAvailableCollateralToLiquidate` could pin every
rounding direction — see [§11.5](#115-_calculateavailablecollateraltoliquidate).

Same guard-then-compute Yul pattern and same bare `revert(0,0)` as `WadRayMath`.

### 5.3 `MathUtils`

`libraries/math/MathUtils.sol` (116 lines). `SECONDS_PER_YEAR = 365 days`
(`:15`) — leap years ignored, which is a deliberate, documented simplification.

#### `calculateLinearInterest(rate, lastUpdateTimestamp)` — `:23-34`

```solidity
uint256 result = rate * (block.timestamp - uint256(lastUpdateTimestamp));
unchecked { result = result / SECONDS_PER_YEAR; }
return WadRayMath.RAY + result;
```

Returns `1 + r·Δt/yr` in ray. **Suppliers earn simple interest.**

#### `calculateCompoundedInterest(rate, lastUpdateTimestamp, currentTimestamp)` — `:50-85`

Returns `(1 + r/yr)^Δt` approximated by the **first three terms of the binomial
expansion**. With `x = r·Δt/yr`:

```
(1+x/n)^n  ≈  1 + x + x²/2 + x³/6
```

which the code writes as a Horner-style nest (`:83`):

```solidity
return WadRayMath.RAY + x + x.rayMul(x / 2 + x.rayMul(x / 6));
```

Expanding: `RAY + x + x·(x/2 + x·(x/6))` = `1 + x + x²/2 + x³/6`. That is the
Taylor series of `eˣ` truncated after the cubic term.

**Error bound.** The exact value is `eˣ = Σ xⁿ/n!`; the first omitted term is
`x⁴/24`. For a reserve at 10% APR accruing over a full year, `x = 0.1` and the
error is `0.1⁴/24 ≈ 4.2e-6`, i.e. **~0.4 basis points per year** — and in
practice `updateState` runs on nearly every interaction, so `Δt` is minutes and
the error is unmeasurable. The truncation always **under**estimates (every
omitted term is positive), which the NatSpec (`:42-43`) states plainly: it
"slightly underpays liquidity providers and undercharges borrowers". Erring
downward on debt is the *unsafe* direction in principle, but it is bounded, tiny,
and paid for by an enormous gas saving versus real exponentiation.

The whole computation is `unchecked` (`:79`). The long comment (`:62-77`)
justifies it: `rate` fits in 128 bits, `exp` in 40 bits, and timestamps overflow
`uint40` only in ~100 years, so the product cannot wrap in practice. It also
honestly notes the approximation is garbage at absurd inputs (100,000% APR over
10,000 years) but cannot overflow there either.

An overload `calculateCompoundedInterest(rate, lastUpdateTimestamp)` (`:93-98`)
defaults `currentTimestamp` to `block.timestamp`. Note the three-argument form is
`pure` and the two-argument form is `view` — that is what lets
`getNormalizedDebt` be a view.

#### `mulDivCeil(a, b, c)` — `:100-115`

`ceil(a*b/c)`. Reverts if `c == 0` (`:103`) or if `a*b` would overflow (`:108`).
**[3.7]** uses it in `LiquidationDataProvider` **[periphery]** and in the
liquidation dust check so off-chain quotes match on-chain rounding exactly.

### 5.4 `TokenMath`

`libraries/helpers/TokenMath.sol` (114 lines). Added in **[3.5]**, authored by
BGD Labs. This library is small and is the most important security artefact in
the protocol: it fixes, once, the rounding direction of every conversion between
*scaled* and *actual* balances.

The header (`:10-12`) states the rule: rounding is **ERC-4626 aligned**, meaning
always **in favour of the protocol**.

| Function | Line | Direction | Why |
|---|---|---|---|
| `getATokenMintScaledAmount(amount, idx)` | `:24-29` | `rayDivFloor` (down) | minting fewer shares than paid for ⇒ user cannot mint value from rounding |
| `getATokenBurnScaledAmount(amount, idx)` | `:38-43` | `rayDivCeil` (up) | burn at least enough shares for the underlying withdrawn |
| `getATokenTransferScaledAmount(amount, idx)` | `:52-57` | `rayDivCeil` (up) | recipient gets ≥ requested; sender pays the rounding |
| `getATokenBalance(scaled, idx)` | `:66-71` | `rayMulFloor` (down) | never report more balance than backed |
| `getVTokenMintScaledAmount(amount, idx)` | `:80-85` | `rayDivCeil` (up) | debt shares round **up** ⇒ never under-record debt |
| `getVTokenBurnScaledAmount(amount, idx)` | `:94-99` | `rayDivFloor` (down) | burn fewer debt shares ⇒ never forgive debt by rounding |
| `getVTokenBalance(scaled, idx)` | `:108-113` | `rayMulCeil` (up) | never under-report debt |

**Read the table as one sentence:** *supply rounds against the supplier, debt
rounds against the borrower.* Every wei of rounding error accrues to the
protocol's solvency buffer.

Before **[3.5]** these were plain `rayDiv`/`rayMul` (half-up), which meant half
of all roundings went the user's way. That is the classic "1 wei" attack surface:
repeat a half-up-rounding operation enough times and you extract value. Pinning
every direction closes it.

---

## 6. `ReserveLogic`

`libraries/logic/ReserveLogic.sol` (275 lines). **`internal`, so inlined** —
this is not a deployed library. It owns interest accrual: the two indexes, the
treasury cut, and the rate refresh.

### The sandwich

Every state-changing action in the protocol has this shape:

```
DataTypes.ReserveCache memory cache = reserve.cache();   // snapshot
reserve.updateState(cache);                              // accrue to NOW
        ... validate ...                                 // uses cache
        ... mint/burn tokens ...                         // uses cache.next* indexes
reserve.updateInterestRatesAndVirtualBalance(cache, ...); // new rates + balance
```

Skip `updateState` and you charge interest at a stale index. Skip
`updateInterestRatesAndVirtualBalance` and the rate does not react to the
liquidity you just moved. Every logic library follows this exactly.

### 6.1 `cache(reserve)` — `:251-274`

**Internal view.** Builds the `ReserveCache`.

- **Reads:** the whole `configuration` word, both indexes, both rates, both token
  addresses, `lastUpdateTimestamp`.
- **External call:** exactly one —
  `IVariableDebtToken(variableDebtTokenAddress).scaledTotalSupply()` (`:269-271`).
  This is the only reason `cache` is not `pure`.
- Sets `curr* == next*` initially (`:258-260`, `:269`); `_updateIndexes` and the
  mint/burn steps advance the `next*` copies.

### 6.2 `updateState(reserve, reserveCache)` — `:85-101`

**Internal.** The accrual entry point.

1. **Early exit** (`:91-93`): if `reserveCache.reserveLastUpdateTimestamp ==
   block.timestamp`, return immediately — already accrued this block. This is
   what makes multiple actions in one transaction cheap.
2. `_updateIndexes(reserve, reserveCache)` (`:95`).
3. `_accrueToTreasury(reserve, reserveCache)` (`:96`) — **must run after**
   `_updateIndexes`, because it needs both the old and new borrow index.
4. Writes `reserve.lastUpdateTimestamp = uint40(block.timestamp)` (`:99`) and
   mirrors it into the cache (`:100`).

### 6.3 `_updateIndexes(reserve, reserveCache)` — `:211-243`

**Internal.** Advances both indexes. Two independent guarded blocks:

**Supply index** (`:218-227`), guarded by `currLiquidityRate != 0`:

```solidity
uint256 cumulatedLiquidityInterest = MathUtils.calculateLinearInterest(
  reserveCache.currLiquidityRate, reserveCache.reserveLastUpdateTimestamp);
reserveCache.nextLiquidityIndex = cumulatedLiquidityInterest.rayMul(
  reserveCache.currLiquidityIndex);
reserve.liquidityIndex = reserveCache.nextLiquidityIndex.toUint128();
```

**Linear**, per [§5.3](#53-mathutils). The comment (`:215-217`) explains the
guard: with a 100% reserve factor `currentLiquidityRate` is 0 and the index must
not move.

**Borrow index** (`:233-242`), guarded by **`currScaledVariableDebt != 0`**:

```solidity
uint256 cumulatedVariableBorrowInterest = MathUtils.calculateCompoundedInterest(
  reserveCache.currVariableBorrowRate, reserveCache.reserveLastUpdateTimestamp);
reserveCache.nextVariableBorrowIndex = cumulatedVariableBorrowInterest.rayMul(
  reserveCache.currVariableBorrowIndex);
reserve.variableBorrowIndex = reserveCache.nextVariableBorrowIndex.toUint128();
```

**Compounded.** The comment (`:230-232`) is worth reading: the guard is on
*debt*, not on *rate*, because a reserve can have a non-zero base rate with zero
borrows, and the index must not move then.

**Why linear for supply and compound for debt.** Debt genuinely compounds — the
borrower owes interest on interest. Supply interest is *paid out of* what
borrowers owe, and is distributed by the index; using linear accrual on the
supply side guarantees the protocol never promises suppliers more than borrowers
were charged. The gap between the compounded borrow index and the linear supply
index, minus the reserve factor, is the protocol's safety margin.

### 6.4 `_accrueToTreasury(reserve, reserveCache)` — `:183-204`

**Internal.** Takes the reserve factor's cut of the interest just accrued.

1. **Early exit** (`:187-189`) if `reserveFactor == 0`.
2. Interest accrued this period (`:193-195`):
   ```solidity
   uint256 totalDebtAccrued = reserveCache.currScaledVariableDebt.rayMulFloor(
     reserveCache.nextVariableBorrowIndex - reserveCache.currVariableBorrowIndex);
   ```
   `scaledDebt × Δindex` is exactly the underlying interest. **`rayMulFloor`** —
   the comment (`:192`) says it: "Rounding down to undermint to the treasury and
   keep the invariant healthy." The treasury is the one party that loses the
   rounding.
3. `amountToMint = totalDebtAccrued.percentMul(reserveFactor)` (`:197`).
4. If non-zero, accumulate **scaled** (`:200-202`):
   ```solidity
   reserve.accruedToTreasury += amountToMint
     .getATokenMintScaledAmount(reserveCache.nextLiquidityIndex).toUint128();
   ```

It accrues *scaled* and stays there. No aTokens are minted until someone calls
`Pool.mintToTreasury` ([§13.4](#134-executeminttotreasury)) — so the treasury's
claim grows with the index like any supplier's.

### 6.5 `updateInterestRatesAndVirtualBalance(...)` — `:130-175`

**Internal.** Recomputes rates and moves the virtual balance.

Parameters: `reserve`, `reserveCache`, `reserveAddress`, `liquidityAdded`,
`liquidityTaken`, `interestRateStrategyAddress`.

1. Current total debt (`:138-140`):
   `reserveCache.nextScaledVariableDebt.getVTokenBalance(nextVariableBorrowIndex)`
   — **ceil**, per [§5.4](#54-tokenmath).
2. **External call** (`:142-155`) to
   `IReserveInterestRateStrategy.calculateInterestRates` **[periphery]**, passing
   a `CalculateInterestRatesParams` with:
   - `unbacked: reserve.deficit` (`:146`) — the fossil field name, see [§3.4](#34-execution-parameter-structs)
   - `totalDebt`, `reserveFactor`, `reserve`
   - `usingVirtualBalance: true` (`:152`) — hardcoded since **[3.4]**
   - `virtualUnderlyingBalance: reserve.virtualUnderlyingBalance` (`:153`)
3. Store both new rates as `uint128` (`:157-158`).
4. **Move the virtual balance** (`:160-165`):
   ```solidity
   if (liquidityAdded > 0) reserve.virtualUnderlyingBalance += liquidityAdded.toUint128();
   if (liquidityTaken > 0) reserve.virtualUnderlyingBalance -= liquidityTaken.toUint128();
   ```
   The subtraction is checked arithmetic, so an attempt to take more than the
   protocol accounts for reverts with a panic — the last line of defence for
   solvency.
5. **Emits** `IPool.ReserveDataUpdated` (`:167-174`) with a hardcoded `0` for the
   dead stable-rate slot.

### 6.6 `getNormalizedIncome(reserve)` — `:39-54`

**Internal view.** The *current* supply index without writing storage.

```solidity
if (timestamp == block.timestamp) return reserve.liquidityIndex;   // :45-47
return MathUtils.calculateLinearInterest(reserve.currentLiquidityRate, timestamp)
         .rayMul(reserve.liquidityIndex);                          // :49-52
```

This is what `AToken.balanceOf` multiplies by. Same-block short-circuit means a
read after a write in one transaction is free and consistent.

### 6.7 `getNormalizedDebt(reserve)` — `:63-78`

**Internal view.** Identical shape, but `calculateCompoundedInterest` on
`currentVariableBorrowRate` (`:74-76`). Backs `VariableDebtToken.balanceOf`.

### 6.8 `init(reserve, aTokenAddress, variableDebtTokenAddress)` — `:109-120`

**Internal.** Called once per reserve from `PoolLogic.executeInitReserve`.

- `require(reserve.aTokenAddress == address(0), Errors.ReserveAlreadyInitialized())` (`:114`)
- Sets **both indexes to `RAY`** (`:116-117`) — indexes start at exactly 1.0.
- Stores both token addresses (`:118-119`).

Note it does **not** set `lastUpdateTimestamp`; it stays 0 until the first
`updateState`, at which point the elapsed time is enormous but both rates are 0,
so both guarded blocks in `_updateIndexes` are skipped and nothing accrues.

---

## 7. `ValidationLogic`

`libraries/logic/ValidationLogic.sol` (550 lines). **`internal`, inlined.** Every
`require` in the protocol lives here. One public constant:

```solidity
uint256 public constant HEALTH_FACTOR_LIQUIDATION_THRESHOLD = 1e18;   // :32
```

HF is a wad; `1e18` is exactly 1.0. Below it, you are liquidatable.

**[3.7] removed from this library:** `MINIMUM_HEALTH_FACTOR_LIQUIDATION_THRESHOLD`,
`validateAutomaticUseAsCollateral`, `validateDropReserve`, the
`priceOracleSentinel` parameter, and all siloed-borrowing checks.

### 7.1 `validateSupply(reserveCache, reserve, scaledAmount, onBehalfOf)` — `:39-64`

**Internal view.** Checks, in order:

| # | Check | Line | Error |
|---|---|---|---|
| 1 | `scaledAmount != 0` | `:45` | `InvalidAmount` |
| 2 | reserve active | `:48` | `ReserveInactive` |
| 3 | not paused | `:49` | `ReservePaused` |
| 4 | not frozen | `:50` | `ReserveFrozen` |
| 5 | `onBehalfOf != aToken` | `:51` | `SupplyToAToken` |
| 6 | supply cap | `:54-63` | `SupplyCapExceeded` |

The supply-cap check (`:57-61`) is worth reading closely:

```solidity
supplyCap == 0 ||
  ((IAToken(aTokenAddress).scaledTotalSupply() + scaledAmount +
    uint256(reserve.accruedToTreasury)).getATokenBalance(nextLiquidityIndex))
  <= supplyCap * (10 ** decimals)
```

It sums scaled supply **plus the pending treasury accrual** before converting to
underlying. Including `accruedToTreasury` means the cap counts value the treasury
is owed but has not minted — otherwise the cap could be silently exceeded the
moment `mintToTreasury` runs. `supplyCap == 0` means no cap, and the cap is in
**whole tokens**, hence `* 10**decimals`.

Check 5 (`SupplyToAToken`) blocks supplying *to* the aToken address, which would
corrupt the aToken's own balance accounting.

### 7.2 `validateWithdraw(reserveCache, scaledAmount, scaledUserBalance)` — `:72-83`

**Internal pure.** `scaledAmount != 0` (`:77`, `InvalidAmount`);
`scaledAmount <= scaledUserBalance` (`:78`, `NotEnoughAvailableUserBalance`);
active (`:81`); not paused (`:82`). **Frozen is deliberately not checked** — a
frozen reserve must still allow exit.

### 7.3 `validateBorrow(reservesData, eModeCategories, params)` — `:103-157`

**Internal view.** Uses `ValidateBorrowLocalVars` (`:85-95`) to dodge stack
depth.

| # | Check | Line | Error |
|---|---|---|---|
| 1 | `amountScaled != 0` | `:108` | `InvalidAmount` |
| 2 | active | `:118` | `ReserveInactive` |
| 3 | not paused | `:119` | `ReservePaused` |
| 4 | not frozen | `:120` | `ReserveFrozen` |
| 5 | **borrowable in this mode** | `:121-131` | `NotBorrowableInEMode` / `BorrowingNotEnabled` |
| 6 | pool has the liquidity | `:132-135` | `InvalidAmount` |
| 7 | rate mode is `VARIABLE` | `:138-141` | `InvalidInterestRateModeSelected` |
| 8 | borrow cap | `:149-156` | `BorrowCapExceeded` |

**Check 5 is the [3.6] decoupling in one branch** (`:121-131`):

```solidity
if (params.userEModeCategory != 0) {
  require(EModeConfiguration.isReserveEnabledOnBitmap(
      eModeCategories[params.userEModeCategory].borrowableBitmap,
      reservesData[params.asset].id), Errors.NotBorrowableInEMode());
} else {
  require(vars.borrowingEnabled, Errors.BorrowingNotEnabled());
}
```

Inside an eMode, *only* the category's `borrowableBitmap` matters — the reserve's
own `borrowingEnabled` flag is ignored entirely. That is what makes
"borrowable only in eMode" configurable. Before 3.6 both had to be true.

Check 7 is a fossil worth noting: since **[3.2]** removed stable-rate borrowing,
the only legal `InterestRateMode` is `VARIABLE` (= 2). `NONE` (0) and
`__DEPRECATED` (1) both revert.

### 7.4 `validateRepay(user, reserveCache, amountSent, interestRateMode, onBehalfOf, debtScaled)` — `:167-190`

**Internal pure.**

| # | Check | Line | Error |
|---|---|---|---|
| 1 | `amountSent != 0` | `:175` | `InvalidAmount` |
| 2 | mode is `VARIABLE` | `:176-179` | `InvalidInterestRateModeSelected` |
| 3 | `amountSent != max \|\| user == onBehalfOf` | `:180-183` | `NoExplicitAmountToRepayOnBehalf` |
| 4 | active | `:186` | `ReserveInactive` |
| 5 | not paused | `:187` | `ReservePaused` |
| 6 | `debtScaled != 0` | `:189` | `NoDebtOfSelectedType` |

Check 3 is a safety rail: `type(uint256).max` means "repay everything", and you
may only say that about **your own** debt. Repaying someone else's requires an
explicit number, so a third party cannot be surprised by an unbounded pull.

### 7.5 `validateSetUseReserveAsCollateral(reserveConfig)` — `:196-202`

**Internal pure.** Only active (`:200`) and not paused (`:201`).

### 7.6 `validateFlashloan(reservesData, assets, amounts)` — `:210-222`

**Internal view.** `assets.length == amounts.length` (`:215`), then an **O(n²)
duplicate scan** (`:216-221`) rejecting any repeated asset, delegating each entry
to `validateFlashloanSimple`. Duplicates are banned because the premium and
repayment accounting assume one entry per reserve.

### 7.7 `validateFlashloanSimple(reserve, amount)` — `:228-237`

**Internal view.** Not paused (`:233`), active (`:234`), **flashloan enabled**
(`:235`, `FlashloanDisabled`), and `aToken.totalSupply() >= amount` (`:236`).

### 7.8 `validateLiquidationCall(borrowerConfig, collateralReserve, debtReserve, params)` — `:253-292`

**Internal view.**

| # | Check | Line | Error |
|---|---|---|---|
| 1 | `borrower != liquidator` | `:261` | `SelfLiquidation` |
| 2 | both reserves active | `:272` | `ReserveInactive` |
| 3 | neither paused | `:273` | `ReservePaused` |
| 4 | **grace period expired on both** | `:275-279` | `LiquidationGraceSentinelCheckFailed` |
| 5 | `healthFactor < 1e18` | `:281-284` | `HealthFactorNotBelowThreshold` |
| 6 | borrower actually uses this collateral | `:287-290` | `CollateralCannotBeLiquidated` |
| 7 | `totalDebt != 0` | `:291` | `SpecifiedCurrencyNotBorrowedByUser` |

Check 1 is **[3.3]**: self-liquidation used to let a user realise the liquidation
bonus against themselves, producing a deficit while keeping the bonus as
collateral. Check 4 is the **[3.1]** grace period — after unpausing a reserve,
liquidations stay blocked for a window so users can react to prices that moved
while they could not act.

### 7.9 `validateHealthFactor(...)` — `:305-333`

**Internal view.** Calls `GenericLogic.calculateUserAccountData`, requires
`healthFactor >= HEALTH_FACTOR_LIQUIDATION_THRESHOLD` (`:327-330`,
`HealthFactorLowerThanLiquidationThreshold`), returns
`(healthFactor, hasZeroLtvCollateral)`.

### 7.10 `validateHFAndLtv(...)` — `:347-385`

**Internal view.** The **borrow-time** check. Three requires:

1. `currentLtv != 0` (`:374`, `LtvValidationFailed`).
2. `healthFactor >= 1e18` (`:376-379`).
3. **The LTV headroom check** (`:381-384`):
   ```solidity
   require(userCollateralInBaseCurrency >= userDebtInBaseCurrency.percentDivCeil(currentLtv),
           Errors.CollateralCannotCoverNewBorrow());
   ```

Requirement 3 is *stricter* than requirement 2. HF uses the **liquidation
threshold**; this uses the **LTV**, which is always lower. The gap between LTV
and LT is the buffer that stops you borrowing right up to the liquidation line.
`percentDivCeil` **[3.5+]** rounds the required collateral **up**, against the
user. The NatSpec (`:337-338`) is candid that a sophisticated user can work
around it (borrow, then withdraw) — it is an accident guard, not an invariant.

### 7.11 `validateHFAndLtvzero(...)` — `:398-430`

**Internal view.** The **withdraw/transfer-time** check, and the enforcement
point for **ltv0 rules**.

It calls `validateHealthFactor` (`:408-416`), then:

```solidity
if (hasZeroLtvCollateral) {
  require(getUserReserveLtv(reservesData[asset], eModeCategories[userEModeCategory],
                            userEModeCategory) == 0, Errors.LtvValidationFailed());
}
```

Read it as: *if any of your collateral is ltv0, the asset you are removing must
itself be the ltv0 one.* The comment (`:418-419`) says it plainly — a
multi-collateral position must withdraw its ltv0 assets **first**. Otherwise you
could withdraw the good collateral and leave the protocol holding only the asset
governance has flagged as being offboarded.

### 7.12 `validateTransfer(reserve)` — `:436-438`

**Internal view.** One check: not paused (`:437`).

### 7.13 `validateSetUserEMode(...)` — `:448-498`

**Internal view.** Guards entering/leaving an eMode.

1. `categoryId == 0 || eModeCategory.liquidationThreshold != 0` (`:457-460`,
   `InconsistentEModeCategory`) — a category with LT 0 does not exist.
2. **Early exit if `userConfig.isEmpty()`** (`:463-465`) — with no positions,
   any mode is safe.
3. Otherwise iterate the user's config with `getNextFlags` (`:475-496`) and for
   each touched reserve:
   - if **borrowed**: it must be borrowable in the *target* state (`:480-487`) —
     the category's `borrowableBitmap` if entering an eMode, else the reserve's
     own `borrowingEnabled`. Reverts `InvalidDebtInEmode(reserve, categoryId)`.
   - if **collateral**: `getUserReserveLtv(...) != 0` in the target state
     (`:489-494`). Reverts `InvalidCollateralInEmode(reserve, categoryId)`.

Note both errors carry **parameters** (`Errors.sol:92-93`) — rare in this
codebase and genuinely useful, because they name the offending reserve.

The check runs symmetrically for entering *and* leaving (including leaving to
category 0), so you can never strand a position in a state where its own assets
are not valid.

### 7.14 `validateUseAsCollateral(reservesData, eModeCategories, asset, categoryId)` — `:509-516`

**Internal view returns bool.** One line: `getUserReserveLtv(...) != 0`. **[3.7]**
simplified this (it no longer takes `reservesList`) and deleted its sibling
`validateAutomaticUseAsCollateral`, because **[3.6]** stopped auto-enabling
collateral on transfers and liquidations.

### 7.15 `getUserReserveLtv(reserveData, eModeCategoryData, categoryId)` — `:524-549`

**Internal view.** *The* function that resolves which LTV applies to a user in a
reserve. This single function encodes the entire **[3.6]** eMode model:

```solidity
if (categoryId != 0 && isReserveEnabledOnBitmap(collateralBitmap, reserveData.id)) {
  if (isReserveEnabledOnBitmap(ltvzeroBitmap, reserveData.id)) return 0;   // :539
  else return eModeCategoryData.ltv;                                        // :541
}
if (categoryId != 0 && eModeCategoryData.isolated) return 0;                // :545-547
return reserveData.configuration.getLtv();                                  // :548
```

Decision table:

| In eMode? | In `collateralBitmap`? | In `ltvzeroBitmap`? | Category `isolated`? | Result |
|---|---|---|---|---|
| no (`0`) | — | — | — | reserve's own LTV |
| yes | yes | yes | — | **0** (ltv0 inside this eMode) |
| yes | yes | no | — | the **category's** LTV |
| yes | no | — | yes **[3.7]** | **0** |
| yes | no | — | no | reserve's own LTV |

The fourth row is the **[3.7]** `isolated` flag: in an isolated category, assets
outside the collateral bitmap are forced to ltv0 rather than falling back to
their own LTV.

Returning `0` does not merely zero the borrowing power — it sets
`hasZeroLtvCollateral` in `GenericLogic`, which activates the withdraw-first
rule in [§7.11](#711-validatehfandltvzero).

---

## 8. `GenericLogic`

`libraries/logic/GenericLogic.sol` (258 lines). **`internal`, inlined.** Four
functions; one of them is the health-factor engine that every other check
ultimately calls.

### 8.1 `calculateUserAccountData(reservesData, reservesList, eModeCategories, params)` — `:65-183`

**Internal view.** Returns six values:

```
(totalCollateralInBaseCurrency, totalDebtInBaseCurrency,
 avgLtv, avgLiquidationThreshold, healthFactor, hasZeroLtvCollateral)
```

Uses `CalculateUserAccountDataVars` (`:30-48`) — 17 fields, purely to survive
stack depth.

**Step 1 — trivial case** (`:71-73`): empty user config ⇒
`(0,0,0,0, type(uint256).max, false)`. Infinite HF, no work.

**Step 2 — cache eMode params** (`:77-80`): if in an eMode, read the category's
`liquidationThreshold` and `collateralBitmap` **once**, outside the loop.

**Step 3 — the loop** (`:86-154`). Iterate the user's config bitmap:

```solidity
while (vars.unsafe_cachedUserConfig != 0) {
  (vars.unsafe_cachedUserConfig, isBorrowed, isEnabledAsCollateral) =
    UserConfiguration.getNextFlags(vars.unsafe_cachedUserConfig);
  if (isEnabledAsCollateral || isBorrowed) { ... }
  unchecked { ++vars.i; }
}
```

The loop consumes two bits per iteration and **terminates when the remaining
word is zero** — so a user with one position in reserve #3 does 4 iterations,
not 128. The variable is named `unsafe_cachedUserConfig` (`:34`, `:82`) because
it is destructively shifted; the comment at `:470` in `ValidationLogic` makes the
same warning. Do not reuse it.

For each touched reserve (`:90-148`):

1. `vars.currentReserveAddress = reservesList[vars.i]` (`:90`).
2. **The legacy gap guard** (`:92-93`): `if (currentReserveAddress != address(0))`
   — dead since **[3.7]** removed `dropReserve`, kept because it is nearly free.
3. `assetUnit = 10 ** decimals` (`:98`, `unchecked`).
4. **One oracle call** (`:101-103`): `IPriceOracleGetter(oracle).getAssetPrice(asset)`.
   Fetched once and reused for both the collateral and debt legs.
5. **If collateral** (`:105-138`):
   - `userBalanceInBaseCurrency = _getUserBalanceInBaseCurrency(...)` (`:106-111`)
   - accumulate `totalCollateralInBaseCurrency` (`:113`)
   - `vars.ltv = ValidationLogic.getUserReserveLtv(...)` (`:115-119`) — the [§7.15](#715-getuserreserveltv) resolver
   - **if `ltv == 0`, set `hasZeroLtvCollateral = true` and do not add to
     `avgLtv`** (`:120-124`). The asset still counts as collateral for HF, but
     contributes zero borrowing power.
   - pick the liquidation threshold (`:126-133`): the **category's** LT if in an
     eMode *and* this reserve is in `collateralBitmap`, else the reserve's own.
   - accumulate `avgLiquidationThreshold += balance * threshold` (`:135-137`)
6. **If borrowed** (`:140-147`): accumulate
   `totalDebtInBaseCurrency += _getUserDebtInBaseCurrency(...)`.

Note the collateral and debt legs are independent `if`s, not `else if` — a user
can supply and borrow the same asset.

**Step 4 — health factor** (`:162-164`):

```solidity
vars.healthFactor = (vars.totalDebtInBaseCurrency == 0)
  ? type(uint256).max
  : vars.avgLiquidationThreshold.wadDiv(vars.totalDebtInBaseCurrency) / 100_00;
```

At this point `avgLiquidationThreshold` still holds the **unnormalised sum**
`Σ(collateralᵢ × LTᵢ)`. The comment block (`:156-161`) does the dimensional
analysis: base currency has 8 decimals, the percentage 2, so the sum carries 10;
`wadDiv` adds 18 and removes 8; dividing by `100_00` removes the last 2, leaving
18. So:

```
HF = Σ(collateralᵢ × LTᵢ) / totalDebt        (wad)
```

**No debt ⇒ HF = `type(uint256).max`**, which is why a user with no borrows can
always withdraw.

**Step 5 — normalise the averages** (`:166-173`): divide both accumulators by
`totalCollateralInBaseCurrency` (guarding against zero) to turn the weighted sums
into weighted averages. This happens **after** the HF computation, which is why
HF uses the raw sum.

**Cost.** One oracle call, one `scaledBalanceOf`, and one `getNormalized*` per
touched reserve. This function is the single largest gas consumer in the
protocol, which is why `supply` skips it entirely and `borrow`/`withdraw` call it
exactly once.

### 8.2 `calculateAvailableBorrows(totalCollateral, totalDebt, ltv)` — `:193-206`

**Internal pure.**

```solidity
uint256 availableBorrows = totalCollateralInBaseCurrency.percentMulFloor(ltv);   // :198
if (availableBorrows <= totalDebtInBaseCurrency) return 0;                        // :200-202
return availableBorrows - totalDebtInBaseCurrency;                                // :204
```

`percentMulFloor` **[3.5]** rounds **down**, so the quoted headroom is never
optimistic. Returns 0 rather than underflowing when already over-borrowed. Used
only by `Pool.getUserAccountData` — it is a UI helper, not a protocol check.

### 8.3 `_getUserDebtInBaseCurrency(user, reserve, assetPrice, assetUnit)` — `:219-230`

**Private view.**

```solidity
uint256 userTotalDebt = IScaledBalanceToken(reserve.variableDebtTokenAddress)
  .scaledBalanceOf(user).getVTokenBalance(reserve.getNormalizedDebt());
return MathUtils.mulDivCeil(userTotalDebt, assetPrice, assetUnit);
```

Two ceilings: `getVTokenBalance` rounds the debt **up** ([§5.4](#54-tokenmath)),
and `mulDivCeil` rounds the base-currency conversion **up**. Debt is always
over-stated by at most a wei, never under-stated.

It reads `scaledBalanceOf` + `getNormalizedDebt` rather than `balanceOf` — the
NatSpec (`:210-212`) says it is cheaper, and it also lets the price multiply
happen in the same expression.

### 8.4 `_getUserBalanceInBaseCurrency(user, reserve, assetPrice, assetUnit)` — `:242-257`

**Private view.**

```solidity
uint256 balance = (IScaledBalanceToken(reserve.aTokenAddress).scaledBalanceOf(user)
    .getATokenBalance(reserve.getNormalizedIncome())) * assetPrice;
unchecked { return balance / assetUnit; }
```

Mirror image of the debt version: `getATokenBalance` floors, and the division
floors. Collateral is always **under**-stated. Together with §8.3, every rounding
in the health factor pushes HF **down** — toward liquidation, never away from it.

---

## 9. `SupplyLogic`

`libraries/logic/SupplyLogic.sol` (337 lines). **`external` — a deployed,
delegatecall-linked library.** Five functions. **[3.6]** absorbed
`executeSetUserEMode` from the deleted `EModeLogic`.

### 9.1 `executeSupply(reservesData, eModeCategories, userConfig, params)` — `:40-92`

**External.** The single most-called function in the protocol.

**Order of operations** (the order matters enormously):

```
1  reserve.cache()                                          :47
2  reserve.updateState(reserveCache)                        :49   ← accrue FIRST
3  scaledAmount = amount.getATokenMintScaledAmount(nextIdx) :50   ← floor
4  ValidationLogic.validateSupply(...)                      :52
5  reserve.updateInterestRatesAndVirtualBalance(+amount, 0) :54-60
6  IERC20(asset).safeTransferFrom(user → aToken, amount)    :62   ← pull funds
7  isFirstSupply = IAToken.mint(...)                        :65-70
8  if (isFirstSupply && validateUseAsCollateral) setUsingAsCollateral(true)  :72-83
9  emit IPool.Supply(...)                                   :85-91
```

- **Step 3 rounds down** (`getATokenMintScaledAmount`). The comment at `:64` is
  explicit: "As aToken.mint rounds down the minted shares, we ensure an
  equivalent of <= params.amount shares is minted." You never get more shares
  than you paid for.
- **Step 5 happens before step 6.** Rates are updated using the *intended*
  amount, then the transfer executes. Because `virtualUnderlyingBalance` is the
  protocol's own number, this ordering is safe — the actual token balance is
  irrelevant to the accounting.
- **Step 8 is the auto-collateral rule.** It fires **only on first supply**
  (`isFirstSupply`, returned by `AToken.mint` when the previous scaled balance was
  zero) and only if `validateUseAsCollateral` says the asset has non-zero LTV in
  the user's current eMode. **[3.6]** kept this for `supply` while removing it
  from transfers and liquidations.
- **No health-factor check.** Supplying can only improve HF, so the expensive
  `calculateUserAccountData` is skipped entirely.

**Events:** `IPool.Supply` (`:85`), plus `ReserveUsedAsCollateralEnabled` from
inside `setUsingAsCollateral`, plus `ReserveDataUpdated` from step 5.

**Callers:** `Pool.supply`, `Pool.supplyWithPermit`, `Pool.deposit`.

### 9.2 `executeWithdraw(reservesData, reservesList, eModeCategories, userConfig, params)` — `:106-176`

**External returns `uint256`** (the actual amount withdrawn).

```
1  require(params.to != aTokenAddress, Errors.WithdrawToAToken())   :116
2  reserve.updateState(reserveCache)                                :118
3  scaledUserBalance = aToken.scaledBalanceOf(user)                 :120
4  resolve amount (max vs explicit)                                 :124-134
5  ValidationLogic.validateWithdraw(...)                            :136
6  updateInterestRatesAndVirtualBalance(0, amountToWithdraw)        :138-144
7  zeroBalanceAfterBurn = aToken.burn(...)                          :147-153
8  collateral bookkeeping + HF check                                :155-171
9  emit IPool.Withdraw                                              :173
```

**Step 4 — the `type(uint256).max` path** (`:124-134`) is subtle and important:

```solidity
if (params.amount == type(uint256).max) {
  scaledAmountToWithdraw = scaledUserBalance;                                  // exact
  amountToWithdraw = scaledUserBalance.getATokenBalance(nextLiquidityIndex);   // floor
} else {
  scaledAmountToWithdraw = params.amount.getATokenBurnScaledAmount(nextIdx);   // ceil
  amountToWithdraw = params.amount;
}
```

For "withdraw everything", it burns the **exact scaled balance** and derives the
underlying by flooring — guaranteeing the position closes to exactly zero with no
dust. For an explicit amount, it rounds the burned shares **up**, so the user
always pays at least enough shares (comment at `:146`).

**Step 8** (`:155-171`) only does work if the reserve was collateral:
- if the burn emptied the balance, clear the collateral bit (`:156-158`);
- **only if the user is borrowing anything** (`:159`), run
  `validateHFAndLtvzero` (`:160-169`) — which enforces both HF ≥ 1 and the
  ltv0-withdraw-first rule ([§7.11](#711-validatehfandltvzero)).

A user with no debt skips the oracle loop entirely — withdrawal is cheap.

### 9.3 `executeFinalizeTransfer(reservesData, reservesList, eModeCategories, usersConfig, params)` — `:188-222`

**External.** Called by `Pool.finalizeTransfer`, which `AToken._transfer` calls
after moving balances. This is the hook that keeps collateral flags and health
factors correct across aToken transfers.

```
1  ValidationLogic.validateTransfer(reserve)     :197   ← not paused
2  if (from != to && scaledAmount != 0):         :201
3    if fromConfig.isUsingAsCollateral(id):      :204
4      if (scaledBalanceFromBefore == scaledAmount) clear collateral bit   :205-207
5      if (fromConfig.isBorrowingAny()) validateHFAndLtvzero(from)         :208-219
```

**[3.6] changed this function's signature**: `scaledBalanceToBefore` was removed,
because the receiver side no longer does anything. Before 3.6 the recipient would
have the asset auto-enabled as collateral; now nothing happens to `to` at all.
The gas saving is ~25k (~18%) per transfer, and the rationale is in
`docs/3.6/Aave-v3.6-features.md` — on-chain analysis showed the auto-enable was
almost always unintentional.

Step 4 compares **scaled** values, so "did the sender send everything" is exact
regardless of index drift.

Only the **sender** is health-checked. The recipient's HF can only improve.

### 9.4 `executeUseReserveAsCollateral(...)` — `:240-289`

**External.** Toggling the collateral flag manually.

```
1  ValidationLogic.validateSetUseReserveAsCollateral(config)   :254
2  if (useAsCollateral == current) return;                     :256  ← no-op guard
3a enabling:
     require(aToken.scaledBalanceOf(user) != 0, UnderlyingBalanceZero)  :260-263
     require(validateUseAsCollateral(...), UserHasAssetWithZeroLtv)     :265-273
     setUsingAsCollateral(true)                                          :275
3b disabling:
     setUsingAsCollateral(false)                                         :277
     validateHFAndLtvzero(...)                                           :278-287
```

The idempotence guard at `:256` means calling it twice costs almost nothing and
emits nothing.

Enabling requires an actual balance and a non-zero LTV — you cannot mark an
offboarded (ltv0) asset as collateral. Disabling is unconditional but must leave
you solvent, and must respect the ltv0 withdraw-first rule.

`Errors.UserHasAssetWithZeroLtv` was **renamed in [3.7]** from
`UserInIsolationModeOrLtvZero`, since isolation mode no longer exists.

### 9.5 `executeSetUserEMode(...)` — `:304-336`

**External.** Moved here from `EModeLogic` in **[3.6]**.

```
1  if (usersEModeCategory[user] == categoryId) return;   :314  ← no-op guard
2  ValidationLogic.validateSetUserEMode(...)             :316-322
3  usersEModeCategory[user] = categoryId;                :324   ← write BEFORE checking
4  ValidationLogic.validateHealthFactor(...)             :326-334
5  emit IPool.UserEModeSet(user, categoryId)             :335
```

**Step 3 before step 4 is deliberate.** The new category is written first so that
`validateHealthFactor` recomputes the health factor **under the new eMode's
parameters**. If the switch would push HF below 1, the whole transaction reverts
and the write is rolled back. Checking before writing would test the old
parameters and be useless.

`validateSetUserEMode` (step 2) checks that every borrowed asset remains
borrowable and every collateral remains collateral in the target mode; step 4
then checks the *numbers* still work.

---

## 10. `BorrowLogic`

`libraries/logic/BorrowLogic.sol` (224 lines). **`external`, deployed library.**
Two functions. **[3.7]** removed the isolation-mode debt-ceiling updates that
used to bracket both.

### 10.1 `executeBorrow(reservesData, reservesList, eModeCategories, userConfig, params)` — `:41-114`

**External.**

```
1  reserve.cache(); reserve.updateState(reserveCache)              :49-51
2  amountScaled = amount.getVTokenMintScaledAmount(nextBorrowIdx)  :53-55  ← CEIL
3  ValidationLogic.validateBorrow(...)                             :57-67
4  nextScaledVariableDebt = IVariableDebtToken.mint(...)           :69-76
5  if (!userConfig.isBorrowing(id)) userConfig.setBorrowing(id,true) :78-81
6  updateInterestRatesAndVirtualBalance(0, releaseUnderlying?amount:0) :83-89
7  if (releaseUnderlying) aToken.transferUnderlyingTo(user, amount)   :91-93
8  ValidationLogic.validateHFAndLtv(...)  ← on onBehalfOf            :95-103
9  emit IPool.Borrow(...)                                            :105-113
```

**Step 2 rounds up** — debt shares always round against the borrower
([§5.4](#54-tokenmath)).

**Step 6/7 and `releaseUnderlying`.** When a flash loan opens debt
(`FlashLoanLogic`, mode 2), the receiver already holds the tokens, so
`releaseUnderlying` is `false`: no transfer happens **and** `liquidityTaken` is
passed as `0`, because the virtual balance was already decremented when the flash
loan paid out. Getting this wrong would double-count the outflow.

**Step 8 checks `params.onBehalfOf`, not `params.user`.** In a credit-delegated
borrow the debt lands on `onBehalfOf`, so it is *their* health factor that must
survive. `params.user` merely receives the tokens.

Note step 8 runs **after** the transfer in step 7 — the standard
"do it, then prove it was allowed" pattern. It is safe because a revert unwinds
the transfer.

The `Borrow` event (`:105-113`) hardcodes `InterestRateMode.VARIABLE` and reports
`reserve.currentVariableBorrowRate` — read *after* step 6, so it is the new rate.

### 10.2 `executeRepay(reservesData, reservesList, eModeCategories, onBehalfOfConfig, params)` — `:126-223`

**External returns `uint256`** (the amount actually repaid).

```
1  cache + updateState                                        :134-135
2  userDebtScaled = vToken.scaledBalanceOf(onBehalfOf)        :137-138
   userDebt = userDebtScaled.getVTokenBalance(nextIdx)        :139  ← CEIL
3  ValidationLogic.validateRepay(...)                          :141-148
4  resolve paybackAmount                                       :150-160
5  (noMoreDebt, nextScaledVariableDebt) = vToken.burn(...)     :162-169
6  updateInterestRatesAndVirtualBalance(useATokens?0:payback,0) :171-177
7  if (noMoreDebt) clear the borrowing bit                     :179-181
8a useATokens  → aToken.burn(...) + collateral/HF bookkeeping  :184-209
8b otherwise   → safeTransferFrom(user → aToken, payback)      :211
9  emit IPool.Repay(...)                                       :214-220
```

**Step 4 — resolving the amount** (`:150-160`) has two clamps:

```solidity
uint256 paybackAmount = params.amount;
if (params.useATokens && params.amount == type(uint256).max) {
  paybackAmount = IAToken(aTokenAddress).scaledBalanceOf(params.user)
                    .getATokenBalance(nextLiquidityIndex);      // :153-155
}
if (paybackAmount > userDebt) paybackAmount = userDebt;          // :158-160
```

The first clamp handles "repay everything with my aTokens": it uses the user's
**aToken balance**, because that is the ceiling on what they can pay with. The
comment (`:152`) explains the motive — repaying with aTokens should not leave
interest dust. The second clamp caps at the actual debt, so overpaying is
impossible and the excess is simply never pulled.

**Step 5 burns `getVTokenBurnScaledAmount` (floor)** (`:167`) — burning *fewer*
debt shares than the payment strictly buys, so rounding never forgives debt.

**Step 6's `liquidityAdded`** is `0` when repaying with aTokens (`:174`): no new
underlying enters the protocol, the aToken supply just shrinks. Passing
`paybackAmount` there would inflate the virtual balance out of thin air.

**Step 8a — the aToken repayment path** (`:184-209`) is the interesting one. It
burns the user's aTokens with `receiverOfUnderlying: aTokenAddress` (`:188`) —
i.e. the underlying stays where it is, only the accounting moves. Then, because
the user's collateral just shrank:
- if their aToken balance hit zero, clear the collateral bit (`:194-196`);
- if they still borrow anything, re-check the health factor (`:198-208`).

The comment at `:183` notes that aToken repayment always repays on behalf of
oneself, which is why `params.user` is used for the aToken burn while
`params.onBehalfOf` was used for the debt burn.

**Step 8b** is the normal path: pull underlying straight to the aToken.

---

## 11. `LiquidationLogic`

`libraries/logic/LiquidationLogic.sol` (679 lines). **`external`, deployed
library.** The largest and most intricate file in the protocol.

### 11.0 Constants

| Constant | Line | Value | Meaning |
|---|---|---|---|
| `DEFAULT_LIQUIDATION_CLOSE_FACTOR` | `:43` | `0.5e4` (50%) | `internal`. Default share of debt liquidatable in one call |
| `CLOSE_FACTOR_HF_THRESHOLD` | `:49` | `0.95e18` | `public`. **[3.3]** At or below HF 0.95, the close factor becomes 100% |
| `MIN_BASE_MAX_CLOSE_FACTOR_THRESHOLD` | `:56` | `2000e8` ($2,000) | `public`. **[3.3]** Positions smaller than this are always 100% liquidatable |
| `MIN_LEFTOVER_BASE` | `:64` | `1000e8` ($1,000) | `public`. **[3.3]** Minimum value that may be *left behind* |

The `e8` suffixes assume a **USD-denominated oracle with 8 decimals** — the
NatSpec at `:54` warns these must be adjusted for a non-USD pool. That is a real
deployment footgun.

`MIN_LEFTOVER_BASE` is derived as half of `MIN_BASE_MAX_CLOSE_FACTOR_THRESHOLD`
(`:64`), and the comment (`:61-62`) explains why: liquidating a position of
`n+1` at 50% leaves `n/2`, which must still be worth liquidating. The whole
mechanism exists so liquidators cannot "optimise gas by leaving some wei".

### 11.1 `executeLiquidationCall(reservesData, reservesList, usersConfig, eModeCategories, params)` — `:166-460`

**External.** Uses `LiquidationCallLocalVars` (`:134-153`), 18 fields.
**[3.7]** added `borrowerScaledCollateralBalance` (`:146`) as a single-SLOAD
reuse for the new consumption check.

#### Phase 1 — accrue and measure (`:175-211`)

```solidity
vars.debtReserveCache = debtReserve.cache();
debtReserve.updateState(vars.debtReserveCache);
// caching of the collateral happens after debtReserveCache is updated  :180-181
vars.collateralReserveCache = collateralReserve.cache();
collateralReserve.updateState(vars.collateralReserveCache);
```

**Order matters.** The comment (`:180-182`) says it: debt is cached and accrued
*first*, then collateral. If the debt and collateral asset are the same reserve,
caching collateral first would capture a stale `scaledTotalSupply`.

Then `GenericLogic.calculateUserAccountData` (`:192-202`) gives
`totalCollateralInBaseCurrency`, `totalDebtInBaseCurrency` and `healthFactor`,
and three per-reserve balances are read (`:204-211`), including the new scaled
collateral balance.

#### Phase 2 — validate (`:213-224`)

`ValidationLogic.validateLiquidationCall` — see [§7.8](#78-validateliquidationcall).

#### Phase 3 — pick the liquidation bonus (`:226-239`)

```solidity
if (borrowerEModeCategory != 0 &&
    isReserveEnabledOnBitmap(eModeCategories[...].collateralBitmap, collateralReserve.id))
  vars.liquidationBonus = eModeCategories[borrowerEModeCategory].liquidationBonus;
else
  vars.liquidationBonus = collateralReserveCache.reserveConfiguration.getLiquidationBonus();
```

Same pattern as the LT selection in `GenericLogic` — the eMode bonus applies only
if this specific collateral is in the category's bitmap.

#### Phase 4 — the close factor (`:258-282`)

```solidity
uint256 maxLiquidatableDebt = vars.borrowerReserveDebt;        // default: 100%
if (borrowerReserveCollateralInBaseCurrency >= MIN_BASE_MAX_CLOSE_FACTOR_THRESHOLD &&
    borrowerReserveDebtInBaseCurrency     >= MIN_BASE_MAX_CLOSE_FACTOR_THRESHOLD &&
    healthFactor > CLOSE_FACTOR_HF_THRESHOLD) {
  uint256 totalDefaultLiquidatableDebtInBaseCurrency =
    totalDebtInBaseCurrency.percentMul(DEFAULT_LIQUIDATION_CLOSE_FACTOR);
  if (borrowerReserveDebtInBaseCurrency > totalDefaultLiquidatableDebtInBaseCurrency)
    maxLiquidatableDebt =
      (totalDefaultLiquidatableDebtInBaseCurrency * debtAssetUnit) / debtAssetPrice;
}
vars.actualDebtToLiquidate = min(params.debtToCover, maxLiquidatableDebt);
```

**Read the default carefully: it is 100%, not 50%.** The 50% close factor is the
*exception*, applied only when **all three** conditions hold:

| Condition | Rationale |
|---|---|
| collateral ≥ $2,000 | tiny positions get fully cleared, not left as dust |
| debt ≥ $2,000 | same |
| HF > 0.95 | **[3.3]** deeply underwater positions get fully cleared immediately |

And even then, the 50% is measured against the user's **total** debt across all
reserves, not this reserve's debt — so if this reserve is less than half the
total debt, it is fully liquidatable anyway.

`percentMul` here is deliberately still the **half-up** variant; the **[3.7]**
changelog notes this explicitly while the *collateral* math moved to explicit
directions.

#### Phase 5 — compute collateral (`:284-297`)

Calls `_calculateAvailableCollateralToLiquidate`, which **returns a possibly
reduced `actualDebtToLiquidate`** — see [§11.5](#115-_calculateavailablecollateraltoliquidate).

#### Phase 6 — the dust check **[3.7]** (`:303-325`)

If the liquidation would leave *both* some debt and some collateral behind, both
leftovers must exceed `MIN_LEFTOVER_BASE`:

```solidity
bool isDebtMoreThanLeftoverThreshold = MathUtils.mulDivCeil(
  borrowerReserveDebt - actualDebtToLiquidate, debtAssetPrice, debtAssetUnit) >= MIN_LEFTOVER_BASE;
bool isCollateralMoreThanLeftoverThreshold = ((borrowerCollateralBalance -
  actualCollateralToLiquidate - liquidationProtocolFeeAmount) * collateralAssetPrice) /
  collateralAssetUnit >= MIN_LEFTOVER_BASE;
require(isDebtMoreThanLeftoverThreshold && isCollateralMoreThanLeftoverThreshold,
        Errors.MustNotLeaveDust());
```

The comment (`:299-302`) states the rule: you must either liquidate all the debt,
or all the collateral, or leave more than `MIN_LEFTOVER_BASE` of both. Debt
leftover is measured with `mulDivCeil` (over-state what remains) and collateral
leftover with floor division (under-state it) — both directions make the check
*harder* to pass, i.e. more likely to force a clean liquidation.

#### Phase 7 — `hasNoCollateralLeft` **[3.7]** (`:341-377`)

This block is new in 3.7 and its 12-line comment (`:327-340`) is the best
explanation in the codebase of a subtle rounding problem. Summary:

The two token operations (transfer to liquidator, transfer of the protocol fee)
each independently `rayDivCeil` their scaled amount. **The sum of two
ceil-rounded values can exceed the balance** even when the unscaled arithmetic
predicts a leftover. Pre-3.7 the check compared *base-currency values*, which
could disagree with what the token operations actually did, stranding debt.

The fix computes consumption in **scaled** units, exactly matching the burns:

```solidity
uint256 scaledCollateralConsumed =
  actualCollateralToLiquidate.getATokenBurnScaledAmount(nextLiquidityIndex) +
  liquidationProtocolFeeAmount.getATokenTransferScaledAmount(nextLiquidityIndex);
if (scaledCollateralConsumed > borrowerScaledCollateralBalance)
  scaledCollateralConsumed = borrowerScaledCollateralBalance;          // :351-353
bool reserveFullyConsumed = scaledCollateralConsumed == borrowerScaledCollateralBalance;
uint256 consumedInBaseCurrency = (scaledCollateralConsumed
  .getATokenBalance(nextLiquidityIndex) * collateralAssetPrice) / collateralAssetUnit;
hasNoCollateralLeft = consumedInBaseCurrency == vars.totalCollateralInBaseCurrency;
if (reserveFullyConsumed || hasNoCollateralLeft)
  borrowerConfig.setUsingAsCollateral(..., false);                      // :369-376
```

Two distinct flags:
- **`reserveFullyConsumed`** — *this* collateral reserve is emptied ⇒ clear its
  collateral bit.
- **`hasNoCollateralLeft`** — the user's *entire* collateral across all reserves
  is gone ⇒ any remaining debt is bad debt.

The comment (`:335-337`) also notes the benign case: for an 18-decimal token, a
few wei of genuine leftover rounds to $0 in base currency, so
`consumedInBaseCurrency` still equals the total and the flag is still correct.

#### Phase 8 — burn debt (`:378-388`)

`_burnDebtTokens`, passing `hasNoCollateralLeft` — see [§11.4](#114-_burndebttokens).

#### Phase 9 — move collateral (`:390-411`)

Two paths:

- **`receiveAToken == true`** (`:391-399`):
  `AToken.transferOnLiquidation(borrower → liquidator, ...)`. **[3.6]** removed
  the `_liquidateATokens` helper and the auto-enable of collateral for the
  liquidator — the liquidator receives aTokens but they are **not** marked as
  their collateral (saves ~25k gas).
- **`receiveAToken == false`** (`:400-411`): `_burnCollateralATokens`, preceded by
  a manual cache fixup:
  ```solidity
  if (params.collateralAsset == params.debtAsset)
    vars.collateralReserveCache.nextScaledVariableDebt =
      vars.debtReserveCache.nextScaledVariableDebt;
  ```
  When collateral and debt are the same asset, the debt burn in phase 8 already
  changed the scaled debt; without this copy (`:404-408`) the collateral reserve's
  rate update would use a stale number.

#### Phase 10 — protocol fee (`:413-435`)

If non-zero, transfer the fee to `AToken.RESERVE_TREASURY_ADDRESS()` (`:430`),
with a **1-wei safety cap** (`:419-427`): re-read the borrower's scaled balance
and clamp, because phase 9 may have consumed slightly more than predicted. The
comment at `:421` names the reason: "To avoid trying to send more aTokens than
available on balance, due to 1 wei imprecision."

#### Phase 11 — bad debt (`:437-442`)

```solidity
if (hasNoCollateralLeft && borrowerConfig.isBorrowingAny())
  _burnBadDebt(reservesData, reservesList, borrowerConfig, params);
```

The comment (`:438-439`) gives the gas rationale: each extra debt asset costs
~75k gas, so zero-collateral positions are only touched when there is something
to clear.

#### Phase 12 — collect payment and emit (`:444-459`)

```solidity
IERC20(params.debtAsset).safeTransferFrom(
  params.liquidator, vars.debtReserveCache.aTokenAddress, vars.actualDebtToLiquidate);
emit IPool.LiquidationCall(...);
```

The liquidator pays **last**. Everything before it is accounting; a revert
anywhere unwinds it all.

### 11.2 `executeEliminateDeficit(reservesData, userConfig, params)` — `:76-132`

**External returns `uint256`.** **[3.3]**, for the Umbrella safety module. Burns
the caller's aTokens to write off recognised bad debt.

| # | Check | Line | Error |
|---|---|---|---|
| 1 | `amount != 0` | `:81` | `InvalidAmount` |
| 2 | `reserve.deficit != 0` | `:86` | `ReserveNotInDeficit` |
| 3 | **caller has no debt** | `:87` | `UserCannotHaveDebt` |
| 4 | reserve active | `:92` | `ReserveInactive` |
| 5 | caller has the balance | `:104` | `NotEnoughAvailableUserBalance` |

Then: clamp to the deficit (`:96-98`), clear the collateral bit if fully consumed
(`:106-109`), burn with `receiverOfUnderlying: aTokenAddress` so the underlying
stays put (`:111-117`), reduce `reserve.deficit` (`:119`), refresh rates
(`:121-127`), emit `DeficitCovered` (`:129`).

Check 3 matters: the caller's aTokens vanish with no compensation, so allowing a
user with debt to call it would let them wreck their own health factor. Access
control is on `Pool` (`onlyUmbrella`, [§16.6](#166-admin-and-umbrella-functions)).

### 11.3 `_burnCollateralATokens(collateralReserve, params, vars)` — `:469-492`

**Internal.** Rates first (`:474-480`, with `liquidityTaken =
actualCollateralToLiquidate`), then
`AToken.burn(from: borrower, receiverOfUnderlying: liquidator, ...)` with
`getATokenBurnScaledAmount` (ceil, `:487-489`).

### 11.4 `_burnDebtTokens(...)` — `:506-553`

**Internal.** Nine parameters.

```solidity
bool noMoreDebt = true;
if (borrowerReserveDebt != 0) {                                            // :521
  uint256 burnAmount = hasNoCollateralLeft ? borrowerReserveDebt : actualDebtToLiquidate;  // :522
  (noMoreDebt, debtReserveCache.nextScaledVariableDebt) =
    IVariableDebtToken(...).burn({ from: borrower,
      scaledAmount: burnAmount.getVTokenBurnScaledAmount(nextVariableBorrowIndex),
      index: nextVariableBorrowIndex });
}
uint256 outstandingDebt = borrowerReserveDebt - actualDebtToLiquidate;      // :536
if (hasNoCollateralLeft && outstandingDebt != 0) {
  debtReserve.deficit += outstandingDebt.toUint128();                       // :538
  emit IPool.DeficitCreated(borrower, debtAsset, outstandingDebt);          // :539
}
if (noMoreDebt) borrowerConfig.setBorrowing(debtReserve.id, false);         // :542-544
debtReserve.updateInterestRatesAndVirtualBalance(
  debtReserveCache, debtAsset, actualDebtToLiquidate, 0, ...);              // :546-552
```

**The `hasNoCollateralLeft` branch at `:522` is bad-debt recognition.** When the
borrower has nothing left, the *entire* remaining debt is burned — not just what
the liquidator paid for — and the shortfall is booked as `reserve.deficit`.
The debt stops accruing interest against a position that can never repay, and the
loss becomes an explicit protocol liability that Umbrella can later clear via
`executeEliminateDeficit`.

**Why `liquidityAdded = actualDebtToLiquidate` and not `burnAmount`** (`:549`):
only the amount the liquidator actually pays enters the pool. The extra burned
in the bad-debt case is written off, not funded.

The guard at `:521` with its comment (`:518-520`) is a pre-3.1 compatibility
shim: positions exist where the `isBorrowing` flag was left set with zero debt,
and burning zero would revert inside `_burnScaled`.

### 11.5 `_calculateAvailableCollateralToLiquidate(...)` — `:583-625`

**Internal pure.** Returns `(collateralAmount, debtAmountNeeded,
liquidationProtocolFee)`. **[3.7]** rewrote every rounding call here to be
explicit, and **removed** the old 4th return value
(`collateralToLiquidateInBaseCurrency`).

**Step 1 — base collateral** (`:599-601`):

```
baseCollateral = (debtAssetPrice × debtToCover × collateralAssetUnit)
                 / (collateralAssetPrice × debtAssetUnit)
```

Equal *value* of collateral, no bonus yet. Plain integer division ⇒ floor.

**Step 2 — apply the bonus** (`:603`):

```solidity
maxCollateralToLiquidate = baseCollateral.percentMulFloor(liquidationBonus);
```

`liquidationBonus` is bps **above** 100%: `10500` = 5% bonus. **`percentMulFloor`
[3.7]** — the liquidator's reward rounds **down**.

**Step 3 — clamp to the borrower's balance** (`:605-612`):

```solidity
if (maxCollateralToLiquidate > borrowerCollateralBalance) {
  collateralAmount = borrowerCollateralBalance;
  debtAmountNeeded = ((collateralAssetPrice * collateralAmount * debtAssetUnit) /
    (debtAssetPrice * collateralAssetUnit)).percentDivCeil(liquidationBonus);
} else {
  collateralAmount = maxCollateralToLiquidate;
  debtAmountNeeded = debtToCover;
}
```

**This is where `actualDebtToLiquidate` can shrink.** If the borrower does not
have enough collateral to pay the bonus, the debt repaid is reduced to what the
available collateral supports. `percentDivCeil` **[3.7]** rounds the required
debt **up** — the liquidator pays at least the fair amount.

**Step 4 — protocol fee** (`:614-623`):

```solidity
if (liquidationProtocolFeePercentage != 0) {
  bonusCollateral = collateralAmount - collateralAmount.percentDivFloor(liquidationBonus);
  liquidationProtocolFee = bonusCollateral.percentMulCeil(liquidationProtocolFeePercentage);
  collateralAmount -= liquidationProtocolFee;
}
```

`collateralAmount.percentDivFloor(liquidationBonus)` backs out the pre-bonus
amount, so `bonusCollateral` is **just the bonus portion**. The protocol takes
its cut of the *bonus only*, never of the principal. `percentDivFloor`
**[3.7 new function]** and `percentMulCeil` both push the fee **up** — toward the
protocol, away from the liquidator.

Every rounding in this function: bonus down, required debt up, fee up. The
liquidator absorbs all four.

### 11.6 `_burnBadDebt(reservesData, reservesList, borrowerConfig, params)` — `:636-678`

**Internal.** Walks every reserve the borrower still borrows and writes it all
off.

```solidity
uint256 unsafe_cachedBorrowerConfig = borrowerConfig.data;
while (unsafe_cachedBorrowerConfig != 0) {
  (unsafe_cachedBorrowerConfig, isBorrowed, ) = UserConfiguration.getNextFlags(...);
  if (isBorrowed) {
    address reserveAddress = reservesList[i];
    if (reserveAddress != address(0)) {          // legacy gap guard, :652-653
      DataTypes.ReserveCache memory reserveCache = reservesData[reserveAddress].cache();
      if (reserveCache.reserveConfiguration.getActive()) {         // :655
        reservesData[reserveAddress].updateState(reserveCache);
        _burnDebtTokens(..., borrowerReserveDebt: <full debt>,
                        actualDebtToLiquidate: 0, hasNoCollateralLeft: true, ...);
      }
    }
  }
  unchecked { ++i; }
}
```

Calling `_burnDebtTokens` with `actualDebtToLiquidate = 0` and
`hasNoCollateralLeft = true` means `outstandingDebt == borrowerReserveDebt`, so
**the entire debt of every other reserve becomes deficit** and emits its own
`DeficitCreated`.

The `getActive()` guard at `:655` means debt in an **inactive** reserve is
silently skipped — it stays on the books rather than becoming deficit.

---

## 12. `FlashLoanLogic`

`libraries/logic/FlashLoanLogic.sol` (253 lines). **`external`, deployed
library.** Three functions. Uses `FlashLoanLocalVars` (`:36-42`).

### The inverted action flow

Both entry points carry the same comment (`:64-66`, `:171-173`):

> The usual action flow (cache → updateState → validation → changeState →
> updateRates) is altered to (validation → user payload → cache → updateState →
> changeState → updateRates) for flashloans. This is done to protect against
> reentrance and rate manipulation within the user specified payload.

This is the key security property. A flash loan hands control to arbitrary code
in the middle of the operation. If the reserve were cached *before* that call,
the attacker's payload could change reserve state (by supplying, borrowing, or
taking another flash loan) and the stale cache would then be written back,
corrupting indexes. Deferring the cache until **after** `executeOperation`
returns means the accounting is always built on post-payload reality.

### 12.1 `executeFlashLoan(reservesData, reservesList, eModeCategories, userConfig, params)` — `:57-155`

**External.** Multi-asset flash loan.

```
1  ValidationLogic.validateFlashloan(assets, amounts)                    :68
2  flashloanPremium = isAuthorizedFlashBorrower ? 0 : params.flashLoanPremium  :75
3  for each asset:                                                       :77-90
     totalPremiums[i] = (mode == NONE) ? amount.percentMulCeil(premium) : 0
     reservesData[asset].virtualUnderlyingBalance -= amount
     IAToken(aToken).transferUnderlyingTo(receiver, amount)
4  require(receiver.executeOperation(...), InvalidFlashloanExecutorReturn) :92-101
5  for each asset:                                                       :103-154
     mode == NONE → _handleFlashLoanRepayment(...)
     else         → BorrowLogic.executeBorrow(releaseUnderlying: false)
                    + emit FlashLoan(premium = 0)
```

**Step 2 — the fee waiver** (`:75`): addresses with the `FLASH_BORROWER` role
(`ACLManager.isFlashBorrower`, checked in `Pool.flashLoan`) pay **zero premium**.

**Step 3 decrements `virtualUnderlyingBalance` directly** (`:84`), bypassing
`updateInterestRatesAndVirtualBalance`. Rates are deliberately *not* refreshed
here — doing so would let the payload observe and exploit a manipulated rate.

**Step 3's premium is `percentMulCeil`** (`:81`) — the fee rounds **up**.

**Step 4** requires the receiver's `executeOperation` to return `true`
(`:92-101`). A receiver that returns `false` or reverts kills the transaction.

**Step 5's two modes:**

| `interestRateModes[i]` | Behaviour |
|---|---|
| `0` (`NONE`) | Repay: pull `amount + premium` from the receiver |
| `2` (`VARIABLE`) | **Do not repay** — open a variable debt position instead |

The debt path (`:125-142`) calls `BorrowLogic.executeBorrow` with
`releaseUnderlying: false` (`:138`) because the receiver already holds the
tokens, and looks up the oracle and the user's eMode live (`:139-140`). The
`FlashLoan` event for this path reports **premium 0** (`:150`) — the comment at
`:143` says it: no premium is paid when taking the loan as debt.

Mode `1` was stable-rate debt and no longer exists; `ValidationLogic.validateBorrow`
rejects it.

### 12.2 `executeFlashLoanSimple(reserve, params)` — `:167-207`

**External.** Single-asset, cheaper, and deliberately less capable. The NatSpec
(`:160`) states both restrictions: it **does not waive the fee** for authorised
borrowers and **does not allow taking on debt** — both to save gas.

```
1  ValidationLogic.validateFlashloanSimple(reserve, amount)          :175
2  totalPremium = amount.percentMulCeil(params.flashLoanPremium)     :178
3  reserve.virtualUnderlyingBalance -= amount                        :180
4  IAToken.transferUnderlyingTo(receiver, amount)                    :182
5  require(receiver.executeOperation(...), ...)                      :184-193
6  _handleFlashLoanRepayment(...)                                    :195-206
```

Note the receiver interface differs: `IFlashLoanSimpleReceiver.executeOperation`
takes scalars, not arrays (`IFlashLoanSimpleReceiver.sol:25-32`).

### 12.3 `_handleFlashLoanRepayment(reserve, params)` — `:215-252`

**Internal.**

```solidity
uint256 amountPlusPremium = params.amount + params.totalPremium;      // :219
DataTypes.ReserveCache memory reserveCache = reserve.cache();          // :221  ← cache NOW
reserve.updateState(reserveCache);                                     // :222
reserve.accruedToTreasury += params.totalPremium
  .getATokenMintScaledAmount(reserveCache.nextLiquidityIndex).toUint128();  // :224-227
reserve.updateInterestRatesAndVirtualBalance(
  reserveCache, params.asset, amountPlusPremium, 0, ...);              // :229-235
IERC20(params.asset).safeTransferFrom(
  params.receiverAddress, reserveCache.aTokenAddress, amountPlusPremium);  // :237-241
emit IPool.FlashLoan(...);                                             // :243-251
```

**The entire premium goes to the treasury** (`:224`) — **[3.4]** removed the
LP split, which is why `PoolStorage` still carries
`__DEPRECATED_flashLoanPremiumToProtocol` (`PoolStorage.sol:46`) with the comment
"From v3.4 all flashloan premium is paid to treasury."

`liquidityAdded = amountPlusPremium` (`:232`) restores the virtual balance that
step 3 removed **and** adds the premium — so utilization drops slightly and the
pool ends richer.

The receiver must have approved **the Pool** (not the aToken) for
`amountPlusPremium`, because the `safeTransferFrom` executes inside `Pool`'s
context via `delegatecall`.

---

## 13. `PoolLogic`

`libraries/logic/PoolLogic.sol` (194 lines). **`external`, deployed library.**
Six functions — the housekeeping that does not belong to any user action.
**[3.7]** removed `executeDropReserve` and `executeResetIsolationModeTotalDebt`.

### 13.1 `executeInitReserve(reservesData, reservesList, params)` — `:34-59`

**External returns `bool`** (`true` if appended at the end, `false` if it filled
a gap).

```solidity
require(Address.isContract(params.asset), Errors.NotContract());         // :39
reservesData[params.asset].init(aTokenAddress, variableDebtAddress);     // :40
bool reserveAlreadyAdded = reservesData[params.asset].id != 0
                        || reservesList[0] == params.asset;              // :42-43
require(!reserveAlreadyAdded, Errors.ReserveAlreadyAdded());             // :44
for (uint16 i = 0; i < params.reservesCount; i++) {
  if (reservesList[i] == address(0)) {          // legacy gap fill, :47-48
    reservesData[params.asset].id = i; reservesList[i] = params.asset; return false;
  }
}
require(params.reservesCount < params.maxNumberReserves, Errors.NoMoreReservesAllowed());  // :55
reservesData[params.asset].id = params.reservesCount;
reservesList[params.reservesCount] = params.asset;
return true;
```

The duplicate test at `:42-43` is a classic "id 0 is ambiguous" workaround: a
brand-new reserve and the reserve at index 0 both have `id == 0`, so index 0 is
checked by address.

The gap-filling loop is **[3.7] dead code** — without `dropReserve` no gaps can
form — but it is retained so a pool that *did* drop a reserve before upgrading
still reuses the slot.

### 13.2 `executeSyncIndexesState(reserve)` — `:65-69`

**External.** `cache()` then `updateState()`. Forces accrual with no user action.
Used by the configurator before changing parameters, so the change applies from
"now" rather than retroactively.

### 13.3 `executeSyncRatesState(reserve, asset, interestRateStrategyAddress)` — `:77-91`

**External.** `cache()` then `updateInterestRatesAndVirtualBalance(..., 0, 0, ...)`
— recompute rates with no liquidity movement. Used after the rate strategy or its
parameters change.

Note it does **not** call `updateState` first, so it must be preceded by
`syncIndexesState` if indexes are stale. `PoolConfigurator` does exactly that.

### 13.4 `executeMintToTreasury(reservesData, assets)` — `:108-133`

**External.** Converts accrued scaled amounts into real aTokens.

```solidity
for each asset:
  if (!reserve.configuration.getActive()) continue;       // :118-120
  uint256 accruedToTreasury = reserve.accruedToTreasury;
  if (accruedToTreasury != 0) {
    reserve.accruedToTreasury = 0;                        // :125  ← zero BEFORE minting
    uint256 normalizedIncome = reserve.getNormalizedIncome();
    uint256 amountToMint = accruedToTreasury.getATokenBalance(normalizedIncome);
    IAToken(reserve.aTokenAddress).mintToTreasury(accruedToTreasury, normalizedIncome);
    emit IPool.MintedToTreasury(assetAddress, amountToMint);
  }
```

**Permissionless** — anyone may call `Pool.mintToTreasury`. There is nothing to
steal: the mint always goes to `AToken.RESERVE_TREASURY_ADDRESS()`.

The comment at `:117` notes the `getActive()` check covers **both** inactive and
non-existent reserves, since an unlisted asset has an all-zero config word.

Zeroing before minting (`:125`) is standard checks-effects-interactions.

### 13.5 `executeRescueTokens(token, to, amount)` — `:99-101`

**External.** One line: `IERC20(token).safeTransfer(to, amount)`. Recovers tokens
mistakenly sent to the `Pool` contract itself. Because `Pool` never legitimately
holds tokens (they live in aTokens), anything here is a mistake. Guarded by
`onlyPoolAdmin` at the `Pool` level.

### 13.6 `executeSetLiquidationGracePeriod(reservesData, asset, until)` — `:141-147`

**External.** One line: `reservesData[asset].liquidationGracePeriodUntil = until`.
**[3.1]**. Range validation happens in `PoolConfigurator`
([§17.6](#176-pause-freeze-and-grace-periods)).

### 13.7 `executeGetUserAccountData(reservesData, reservesList, eModeCategories, params)` — `:162-193`

**External view.** Returns the six-tuple behind `Pool.getUserAccountData`:
`GenericLogic.calculateUserAccountData` (`:179-186`) plus
`GenericLogic.calculateAvailableBorrows` (`:188-192`). Note the ordering
difference: `calculateUserAccountData` returns `ltv` before
`currentLiquidationThreshold`, and this function swaps them into the documented
external order.

---

## 14. `ConfiguratorLogic`

`libraries/logic/ConfiguratorLogic.sol` (191 lines). **[3.7] changed every
function from `external` to `internal`**, so this library is now **inlined into
`PoolConfigurator` and no longer deployed separately**. The 3.7 changelog also
removed `IPoolConfigurator.getConfiguratorLogic()`. If you are following an old
deployment guide, this is a step that no longer exists.

### 14.1 `executeInitReserve(pool, input)` — `:30-90`

**Internal.** Lists a new asset. This is the only place aTokens and debt tokens
are created.

```
1  decimals = IERC20Metadata(underlyingAsset).decimals()      :35
   require(decimals > 5, Errors.InvalidDecimals())            :36
2  aTokenProxy = _initTokenWithProxy(aTokenImpl, initCall)    :38-49
3  vDebtProxy = _initTokenWithProxy(vDebtImpl, initCall)      :51-62
4  pool.initReserve(asset, aTokenProxy, vDebtProxy)           :64
5  build a fresh config word:                                 :66-73
     setDecimals(decimals); setActive(true);
     setPaused(false); setFrozen(false); setVirtualAccActive();
6  pool.setConfiguration(asset, currentConfig)                :75
7  IReserveInterestRateStrategy.setInterestRateParams(...)    :77-81
8  emit ReserveInitialized(...)                               :83-89
```

**`decimals > 5`** (`:36`) is a real constraint: with fewer than 6 decimals the
supply/borrow caps (expressed in whole tokens) and the rounding in `TokenMath`
lose too much precision. USDC (6) is the practical minimum.

The comment at `:34` states an explicit trust assumption: *"It is an assumption
that the asset listed is non-malicious, and the external call doesn't create
re-entrancies."* Listing an asset is a governance action, and a malicious
`decimals()` implementation could reenter here. That is accepted, not defended.

Note step 5 creates a config with **everything else zero** — LTV 0, LT 0, no
borrowing, no caps. A freshly listed reserve is inert until
`configureReserveAsCollateral` and friends are called, which is why listing
payloads are always multi-step.

Step 7 reads `pool.RESERVE_INTEREST_RATE_STRATEGY()` — **[3.4]** made the
strategy a single pool-wide immutable rather than per-reserve.

The `ReserveInitialized` event (`:83-89`) still carries `address(0)` in the
stable-debt-token position — a permanent fossil of **[3.2]**.

### 14.2 `executeUpdateAToken(cachedPool, input)` — `:98-119`

**Internal.** Reads the existing proxy via `cachedPool.getReserveAToken` (`:102`)
and the decimals from the live config (`:104`), re-encodes
`IInitializableAToken.initialize` (`:106-114`), calls
`_upgradeTokenImplementation` (`:116`), emits `ATokenUpgraded` (`:118`).

Re-reading decimals from the pool config rather than the token means an upgrade
can never accidentally change them.

### 14.3 `executeUpdateVariableDebtToken(cachedPool, input)` — `:127-152`

**Internal.** Identical shape for the debt token; emits
`VariableDebtTokenUpgraded` (`:147-151`).

### 14.4 `_initTokenWithProxy(implementation, initParams)` — `:160-171`

**Internal returns `address`.**

```solidity
InitializableImmutableAdminUpgradeabilityProxy proxy =
  new InitializableImmutableAdminUpgradeabilityProxy(address(this));
proxy.initialize(implementation, initParams);
return address(proxy);
```

`address(this)` is the **`PoolConfigurator`**, which therefore becomes the
proxy's permanent admin. "Immutable admin" means the admin is baked into the
proxy's bytecode at construction — it cannot be transferred. Every aToken and
debt token in the market is upgradeable **only** through `PoolConfigurator`,
which is itself behind a proxy owned by governance.

### 14.5 `_upgradeTokenImplementation(proxyAddress, implementation, initParams)` — `:180-190`

**Internal.** `proxy.upgradeToAndCall(implementation, initParams)` — swap the
implementation and run the initializer atomically.

---

## 15. `CalldataLogic` and `L2Pool`

On a rollup, the dominant cost is **calldata**, not execution. `L2Pool` exposes
every user action with its arguments packed into one or two `bytes32` words, and
`CalldataLogic` unpacks them in assembly.

The big win is the asset: instead of a 20-byte address, callers pass a **2-byte
`assetId`**, and `CalldataLogic` looks it up in `_reservesList`. A `supply` call
drops from 4 words of arguments to 1.

### 15.1 `CalldataLogic` — the encodings

`libraries/logic/CalldataLogic.sol` (233 lines). All functions `internal view`
(they read `reservesList`). Seven decoders.

**`decodeSupplyParams(reservesList, args)` — `:18-32`**

| Bits | Width | Field |
|---|---|---|
| 0–15 | 16 | `assetId` → `reservesList[assetId]` |
| 16–143 | 128 | `amount` |
| 144–159 | 16 | `referralCode` |

```solidity
assembly {
  assetId      := and(args, 0xFFFF)
  amount       := and(shr(16, args), 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF)
  referralCode := and(shr(144, args), 0xFFFF)
}
```

**`decodeSupplyWithPermitParams(reservesList, args)` — `:44-58`** — reuses
`decodeSupplyParams` (`:55`) and adds:

| Bits | Width | Field |
|---|---|---|
| 160–191 | 32 | `deadline` |
| 192–199 | 8 | `permitV` |

`r` and `s` stay as separate `bytes32` arguments — they are incompressible.

**`decodeWithdrawParams(reservesList, args)` — `:67-79`** — `assetId` (0–15) and
`amount` (16–143). Note `type(uint256).max` cannot be expressed in 128 bits, so
"withdraw everything" on L2 is `amount = type(uint128).max`; the `Pool` treats
only exact `type(uint256).max` as "all", so L2 users must pass a real amount.

**`decodeBorrowParams(reservesList, args)` — `:92-109`**

| Bits | Width | Field |
|---|---|---|
| 0–15 | 16 | `assetId` |
| 16–143 | 128 | `amount` |
| 144–151 | 8 | `interestRateMode` |
| 152–167 | 16 | `referralCode` |

**`decodeRepayParams(reservesList, args)` — `:119-138`** — `assetId`, `amount`,
`interestRateMode` at bits 144–151.

**`decodeRepayWithPermitParams(reservesList, args)` — `:150-168`** — reuses
`decodeRepayParams`, adds `deadline` at bits 152–183 and `permitV` at 184–191.

**`decodeSetUserUseReserveAsCollateralParams(reservesList, args)` — `:177-188`** —
`assetId` (0–15) and a single bit `useAsCollateral` at bit 16.

**`decodeLiquidationCallParams(reservesList, args1, args2)` — `:201-232`** — the
only two-word decoder:

`args1`:
| Bits | Width | Field |
|---|---|---|
| 0–15 | 16 | `collateralAssetId` |
| 16–31 | 16 | `debtAssetId` |
| 32–191 | 160 | `user` (full address) |

`args2`:
| Bits | Width | Field |
|---|---|---|
| 0–127 | 128 | `debtToCover` |
| 128 | 1 | `receiveAToken` |

The borrower's address cannot be compressed — there is no registry of users — so
it takes a full 160 bits.

**The 128-bit amount ceiling** is the one real constraint of the L2 interface. It
is ~3.4e38, which is comfortable for any realistic token amount, but it does mean
the `type(uint256).max` sentinel is unavailable on the packed path.

### 15.2 `L2Pool`

`pool/L2Pool.sol` (101 lines). `abstract contract L2Pool is Pool, IL2Pool`
(`:14`). Nine functions, each a two-liner: decode, then call the inherited
`Pool` function.

| Function | Line | Decoder | Forwards to |
|---|---|---|---|
| `supply(bytes32)` | `:16-23` | `decodeSupplyParams` | `supply(asset, amount, _msgSender(), referralCode)` |
| `supplyWithPermit(bytes32,bytes32,bytes32)` | `:26-31` | `decodeSupplyWithPermitParams` | `supplyWithPermit(...)` |
| `withdraw(bytes32)` | `:34-38` | `decodeWithdrawParams` | `withdraw(asset, amount, _msgSender())` |
| `borrow(bytes32)` | `:41-46` | `decodeBorrowParams` | `borrow(...)` |
| `repay(bytes32)` | `:49-56` | `decodeRepayParams` | `repay(...)` |
| `repayWithPermit(bytes32,bytes32,bytes32)` | `:59-69` | `decodeRepayWithPermitParams` | `repayWithPermit(...)` |
| `repayWithATokens(bytes32)` | `:72-79` | `decodeRepayParams` | `repayWithATokens(...)` |
| `setUserUseReserveAsCollateral(bytes32)` | `:82-88` | `decodeSetUserUseReserveAsCollateralParams` | `setUserUseReserveAsCollateral(...)` |
| `liquidationCall(bytes32,bytes32)` | `:91-100` | `decodeLiquidationCallParams` | `liquidationCall(...)` |

**Every one hardcodes `_msgSender()`** as `onBehalfOf`/`to`. The packed interface
deliberately offers no way to act for another address — that is what saves the
address from the calldata. Third-party actions must use the full `Pool` ABI.

Because these are `external` overloads with different signatures
(`supply(bytes32)` vs `supply(address,uint256,address,uint16)`), both coexist on
the deployed L2 pool and integrators can pick either.

Note `flashLoan` has **no packed variant** — its arrays cannot be compressed
usefully.

---

## 16. `Pool`

`pool/Pool.sol` (941 lines).

```solidity
abstract contract Pool is VersionedInitializable, PoolStorage, IPool, Multicall   // :38
```

`abstract` — the deployed contract is `PoolInstance` **[periphery]**, which
supplies `initialize` and `POOL_REVISION` (= **11**, i.e. v3.7). `Multicall`
lets users batch several pool calls into one transaction, which is now the
official replacement for several removed auto-behaviours (e.g. enabling
collateral after a transfer).

### 16.1 `PoolStorage`

`pool/PoolStorage.sol` (57 lines). Declares the layout; `Pool` inherits it. Order
is immutable across upgrades.

| Slot | Variable | Line | Notes |
|---|---|---|---|
| 0 | `mapping(address => ReserveData) _reserves` | `:21` | per-asset state |
| 1 | `mapping(address => UserConfigurationMap) _usersConfig` | `:24` | per-user bitmap |
| 2 | `mapping(uint256 => address) _reservesList` | `:28` | id → asset. A mapping, not an array, "for gas savings reasons" (`:27`) |
| 3 | `mapping(uint8 => EModeCategory) _eModeCategories` | `:32` | |
| 4 | `mapping(address => uint8) _usersEModeCategory` | `:35` | |
| 5 | `uint256 __DEPRECATED_bridgeProtocolFee` | `:38` | dead |
| 6 (packed) | `uint128 _flashLoanPremium` | `:42` | **[3.4]** all of it goes to treasury |
| 6 (packed) | `uint128 __DEPRECATED_flashLoanPremiumToProtocol` | `:46` | dead |
| 7 (packed) | `uint64 __DEPRECATED_maxStableRateBorrowSizePercent` | `:49` | dead **[3.2]** |
| 7 (packed) | `uint16 _reservesCount` | `:52` | high-water mark of reserve ids |
| 8 | `mapping(address user => mapping(address => bool)) _positionManager` | `:55-56` | **[3.4]** |

**No storage gap.** Aave relies on strict append-only discipline instead: new
variables go at the end, removed ones become `__DEPRECATED_`. `_positionManager`
being last is exactly that pattern.

### 16.2 Immutables, constants and modifiers

| Member | Line | Notes |
|---|---|---|
| `IPoolAddressesProvider public immutable ADDRESSES_PROVIDER` | `:41` | set in constructor, so it survives upgrades (it lives in bytecode, not storage) |
| `address public immutable RESERVE_INTEREST_RATE_STRATEGY` | `:43` | **[3.4]** one strategy for the whole pool |
| `bytes32 public constant UMBRELLA = 'UMBRELLA'` | `:46` | the id looked up in the addresses provider |

| Modifier | Line | Enforces | Error |
|---|---|---|---|
| `onlyPoolConfigurator` | `:51-54` → `_onlyPoolConfigurator` `:80-85` | `msg.sender == ADDRESSES_PROVIDER.getPoolConfigurator()` | `CallerNotPoolConfigurator` |
| `onlyPoolAdmin` | `:59-62` → `_onlyPoolAdmin` `:87-92` | `IACLManager.isPoolAdmin(msg.sender)` | `CallerNotPoolAdmin` |
| `onlyPositionManager(onBehalfOf)` | `:67-70` → `_onlyPositionManager` `:94-96` | `_positionManager[onBehalfOf][msg.sender]` | `CallerNotPositionManager` |
| `onlyUmbrella` | `:75-78` | `ADDRESSES_PROVIDER.getAddress(UMBRELLA) == msg.sender` | `CallerNotUmbrella` |

The `_only*` helpers are `internal view virtual` so test/mock subclasses can
override them.

**`constructor(provider, interestRateStrategy)`** — `:102-106`. Sets both
immutables; `require(interestRateStrategy != address(0), ZeroAddressNotValid())`
(`:104`).

**`initialize(provider)`** — `:115`, `external virtual`, unimplemented here.

### 16.3 User actions

Every one is a thin forwarder. The pattern: build a params struct, pass the
relevant storage mappings by reference, call the library.

| Function | Line | Forwards to |
|---|---|---|
| `supply(asset, amount, onBehalfOf, referralCode)` | `:118-138` | `SupplyLogic.executeSupply` |
| `supplyWithPermit(..., deadline, v, r, s)` | `:141-176` | permit, then `SupplyLogic.executeSupply` |
| `deposit(asset, amount, onBehalfOf, referralCode)` | `:799` | alias of `supply` (legacy v2 name) |
| `withdraw(asset, amount, to)` | `:179-200` | `SupplyLogic.executeWithdraw` |
| `borrow(asset, amount, rateMode, referralCode, onBehalfOf)` | `:203-228` | `BorrowLogic.executeBorrow` (`releaseUnderlying: true`) |
| `repay(asset, amount, rateMode, onBehalfOf)` | `:231` | `BorrowLogic.executeRepay` (`useATokens: false`) |
| `repayWithPermit(...)` | `:258` | permit, then `executeRepay` |
| `repayWithATokens(asset, amount, rateMode)` | `:304` | `executeRepay` (`useATokens: true`, `onBehalfOf: _msgSender()`) |
| `setUserUseReserveAsCollateral(asset, useAsCollateral)` | `:330` | `SupplyLogic.executeUseReserveAsCollateral` |
| `setUserEMode(categoryId)` | `:754-765` | `SupplyLogic.executeSetUserEMode` |
| `liquidationCall(collateral, debt, borrower, debtToCover, receiveAToken)` | `:348` | `LiquidationLogic.executeLiquidationCall` |
| `flashLoan(...)` | `:375` | `FlashLoanLogic.executeFlashLoan` |
| `flashLoanSimple(...)` | `:412` | `FlashLoanLogic.executeFlashLoanSimple` |
| `mintToTreasury(assets)` | `:433-436` | `PoolLogic.executeMintToTreasury` — **permissionless** |

**The `try/catch` around `permit`** (`:151-161`, and again for repay):

```solidity
try IERC20WithPermit(asset).permit(_msgSender(), address(this), amount,
                                   deadline, permitV, permitR, permitS) {} catch {}
```

The failure is **swallowed**. This defends against permit front-running: an
attacker can extract your signature from the mempool and submit the `permit`
alone, which would make your `supplyWithPermit` revert on a reused nonce. By
ignoring the failure, the supply proceeds using the allowance the attacker just
created for you. If there is genuinely no allowance, the subsequent
`safeTransferFrom` reverts anyway.

**Who is charged vs. who is credited.** Note the asymmetry in the params:
`user: _msgSender()` always pays or receives tokens, while `onBehalfOf` owns the
resulting position. `withdraw` uses `_usersConfig[_msgSender()]` — you can only
withdraw your own. `borrow` uses `_usersConfig[onBehalfOf]` — credit delegation.

### 16.4 eMode configuration and views

Setters, all `onlyPoolConfigurator`:

| Function | Line | Writes |
|---|---|---|
| `configureEModeCategory(id, category)` | `:652-664` | ltv, lt, lb, isolated, label |
| `configureEModeCategoryCollateralBitmap(id, bitmap)` | `:666-674` | `collateralBitmap` |
| `configureEModeCategoryBorrowableBitmap(id, bitmap)` | `:676-684` | `borrowableBitmap` |
| `configureEModeCategoryLtvzeroBitmap(id, bitmap)` | `:686-694` | **[3.6]** `ltvzeroBitmap` |
| `configureEModeCategoryIsolated(id, isolated)` | `:696-703` | **[3.7]** `isolated` |

Views: `getEModeCategoryData` (`:705`, returns the legacy struct),
`getEModeCategoryCollateralConfig` (`:720`), `getEModeCategoryLabel` (`:729`,
**deprecated [3.6]**), `getEModeCategoryCollateralBitmap` (`:734`),
`getEModeCategoryBorrowableBitmap` (`:739`),
`getEModeCategoryLtvzeroBitmap` (`:744`, **[3.6]**),
`getIsEModeCategoryIsolated` (`:749`, **[3.7]**),
`getUserEMode(user)` (`:768`).

### 16.5 Position managers **[3.4]**

A position manager is an address a user authorises to manage *non-custodial*
aspects of their position — it can toggle collateral and switch eMode, but it can
**never** move funds.

| Function | Line | Access | Behaviour |
|---|---|---|---|
| `approvePositionManager(positionManager, approve)` | `:840-849` | anyone, for themselves | idempotent (`:841` early return); emits `PositionManagerApproved` or `PositionManagerRevoked` |
| `renouncePositionManagerRole(user)` | `:852-857` | the manager | lets a manager drop its own authority; emits `PositionManagerRevoked` |
| `setUserUseReserveAsCollateralOnBehalfOf(asset, useAsCollateral, onBehalfOf)` | `:859-876` | `onlyPositionManager(onBehalfOf)` | `SupplyLogic.executeUseReserveAsCollateral` |
| `setUserEModeOnBehalfOf(categoryId, onBehalfOf)` | `:878-893` | `onlyPositionManager(onBehalfOf)` | `SupplyLogic.executeSetUserEMode` |
| `isApprovedPositionManager(user, positionManager)` | `:895-901` | view | reads `_positionManager` |

Both `approvePositionManager` and `renouncePositionManagerRole` return early when
the state already matches, so no spurious events.

The delegated actions still run the full health-factor validation, so a
misbehaving manager can at worst leave the position exactly as safe as the rules
require.

### 16.6 Admin and Umbrella functions

| Function | Line | Access | Forwards to |
|---|---|---|---|
| `initReserve(asset, aToken, vDebtToken)` | `:602` | `onlyPoolConfigurator` | `PoolLogic.executeInitReserve` |
| `setConfiguration(asset, config)` | `:635-643` | `onlyPoolConfigurator` | writes `_reserves[asset].configuration` |
| `syncIndexesState(asset)` | `:625-628` | `onlyPoolConfigurator` | `PoolLogic.executeSyncIndexesState` |
| `syncRatesState(asset)` | `:630-633` | `onlyPoolConfigurator` | `PoolLogic.executeSyncRatesState` |
| `updateFlashloanPremium(premium)` | `:645-650` | `onlyPoolConfigurator` | writes `_flashLoanPremium` |
| `setLiquidationGracePeriod(asset, until)` | `:780-787` | `onlyPoolConfigurator` | `PoolLogic.executeSetLiquidationGracePeriod` |
| `rescueTokens(token, to, amount)` | `:789-797` | `onlyPoolAdmin` | `PoolLogic.executeRescueTokens` |
| `eliminateReserveDeficit(asset, amount)` | `:822-838` | **`onlyUmbrella`** | `LiquidationLogic.executeEliminateDeficit` |
| `finalizeTransfer(...)` | `:576` | aToken only | `SupplyLogic.executeFinalizeTransfer` |

**[3.6]** changed `finalizeTransfer`'s signature to
`finalizeTransfer(address asset, address from, address to, uint256 scaledAmount,
uint256 scaledBalanceFromBefore)` — the `scaledBalanceToBefore` parameter is
gone, because the receiver side no longer does anything.

### 16.7 Library address getters

`getFlashLoanLogic` (`:918`), `getBorrowLogic` (`:923`), `getLiquidationLogic`
(`:928`), `getPoolLogic` (`:933`), `getSupplyLogic` (`:938`) — all `external
pure`, returning the linked library addresses baked into the bytecode. They exist
so tooling can verify which library builds a deployment is actually linked
against.

**[3.7] removed `getEModeLogic()`** (the library is gone) and
`getConfiguratorLogic()` (now inlined).

### 16.8 `getReservesList()` — `:529-548`

Worth its own entry for the assembly:

```solidity
uint256 reservesListCount = _reservesCount;
uint256 droppedReservesCount = 0;
address[] memory reservesList = new address[](reservesListCount);
for (uint256 i = 0; i < reservesListCount; i++) {
  if (_reservesList[i] != address(0)) reservesList[i - droppedReservesCount] = _reservesList[i];
  else droppedReservesCount++;
}
assembly { mstore(reservesList, sub(reservesListCount, droppedReservesCount)) }   // :544-546
return reservesList;
```

The array is allocated at full length, gaps are compacted out, and then the
**length word is overwritten in place** — the cheapest possible way to shrink a
memory array in Solidity. Another `dropReserve` fossil: with no gaps possible,
`droppedReservesCount` is always 0 in a fresh v3.7 pool.

### 16.9 View functions

| Function | Line | Returns |
|---|---|---|
| `getReserveData(asset)` | `:438-461` | `ReserveDataLegacy` — the **old** struct shape |
| `getVirtualUnderlyingBalance(asset)` | `:463-468` | `uint128` |
| `getUserAccountData(user)` | `:470-499` | the six-tuple via `PoolLogic.executeGetUserAccountData` |
| `getConfiguration(asset)` | `:501-506` | `ReserveConfigurationMap` |
| `getUserConfiguration(user)` | `:508-513` | `UserConfigurationMap` |
| `getReserveNormalizedIncome(asset)` | `:515-520` | current supply index |
| `getReserveNormalizedVariableDebt(asset)` | `:522-527` | current borrow index |
| `getReservesCount()` | `:551-553` | `_reservesCount` |
| `getReserveAddressById(id)` | `:556-559` | `_reservesList[id]` |
| `getReserveDeficit(asset)` | `:903-906` | **[3.3]** `reserve.deficit` |
| `getReserveAToken(asset)` | `:908-911` | aToken address |
| `getReserveVariableDebtToken(asset)` | `:913-916` | debt token address |
| `FLASHLOAN_PREMIUM_TOTAL()` | `:561-564` | `_flashLoanPremium` |
| `FLASHLOAN_PREMIUM_TO_PROTOCOL()` | `:566-569` | **deprecated [3.4]** |
| `MAX_NUMBER_RESERVES()` | `:571-574` | `ReserveConfiguration.MAX_RESERVES_COUNT` = 128 |

`getReserveData` returning `ReserveDataLegacy` is the single most important
backward-compatibility decision in the codebase: integrations written against
v3.0 still decode the response correctly, receiving zeros in the deprecated
slots. New integrations should use the targeted getters
(`getReserveAToken`, `getVirtualUnderlyingBalance`, `getReserveDeficit`) instead.

---

## 17. `PoolConfigurator`

`pool/PoolConfigurator.sol` (622 lines).
`abstract contract PoolConfigurator is VersionedInitializable, IPoolConfigurator`
(`:24`). The deployed contract is `PoolConfiguratorInstance` **[periphery]** with
`CONFIGURATOR_REVISION = 8` (v3.7).

### 17.1 Storage, constants, modifiers

| Member | Line | Notes |
|---|---|---|
| `IPoolAddressesProvider internal _addressesProvider` | `:28` | |
| `IPool internal _pool` | `:29` | cached at `initialize` |
| `mapping(address => uint256) internal _pendingLtv` | `:31` | LTV parked while a reserve is frozen or ltv0 |
| `uint40 public constant MAX_GRACE_PERIOD = 4 hours` | `:33` | ceiling on the post-unpause liquidation grace |

| Modifier | Line | Role checked (via `ACLManager`) | Error |
|---|---|---|---|
| `onlyPoolAdmin` | `:38-41` / `_onlyPoolAdmin` `:584` | `isPoolAdmin` | `CallerNotPoolAdmin` |
| `onlyEmergencyOrPoolAdmin` | `:46-49` / `_onlyPoolOrEmergencyAdmin` `:589` | `isEmergencyAdmin \|\| isPoolAdmin` | `CallerNotPoolOrEmergencyAdmin` |
| `onlyAssetListingOrPoolAdmins` | `:54-57` / `:597` | `isAssetListingAdmin \|\| isPoolAdmin` | `CallerNotAssetListingOrPoolAdmin` |
| `onlyRiskOrPoolAdmins` | `:62-65` / `:605` | `isRiskAdmin \|\| isPoolAdmin` | `CallerNotRiskOrPoolAdmin` |
| `onlyRiskOrPoolOrEmergencyAdmins` | `:70-73` / `:613` | `isRiskAdmin \|\| isPoolAdmin \|\| isEmergencyAdmin` | `CallerNotRiskOrPoolOrEmergencyAdmin` |

### 17.2 Listing and upgrades

| Function | Line | Access | Behaviour |
|---|---|---|---|
| `initReserves(InitReserveInput[])` | `:78-94` | `onlyAssetListingOrPoolAdmins` | loops `ConfiguratorLogic.executeInitReserve` |
| `updateAToken(UpdateATokenInput)` | `:96-101` | `onlyPoolAdmin` | `ConfiguratorLogic.executeUpdateAToken` |
| `updateVariableDebtToken(UpdateDebtTokenInput)` | `:103-108` | `onlyPoolAdmin` | `ConfiguratorLogic.executeUpdateVariableDebtToken` |

**[3.7]** these call `ConfiguratorLogic` as an **inlined internal** library, not
by `delegatecall`.

### 17.3 `configureReserveAsCollateral(asset, ltv, liquidationThreshold, liquidationBonus)` — `:118-171`

**`onlyRiskOrPoolAdmins`.** The most safety-critical setter, with three
interlocking requirements:

```solidity
require(ltv <= liquidationThreshold, Errors.InvalidReserveParams());          // :127
if (liquidationThreshold != 0) {
  require(liquidationBonus > PercentageMath.PERCENTAGE_FACTOR, ...);          // :134
  require(liquidationThreshold.percentMul(liquidationBonus)
          <= PercentageMath.PERCENTAGE_FACTOR, ...);                          // :138-141
} else {
  require(liquidationBonus == 0, Errors.InvalidReserveParams());              // :143
  _checkNoSuppliers(asset);                                                    // :147
}
```

1. **`ltv <= liquidationThreshold`** (`:127`) — otherwise a loan taken at max LTV
   would be instantly liquidatable (comment `:124-126`).
2. **`liquidationBonus > 100%`** (`:134`) — a bonus at or below par would give
   the liquidator less collateral than the debt they repay, so nobody would
   liquidate (comment `:132-133`).
3. **`liquidationThreshold × liquidationBonus <= 100%`** (`:138-141`) — the deep
   one. It guarantees that at the moment a position becomes liquidatable there is
   still enough collateral to pay the bonus. With LT 80% and bonus 105%, the
   product is 84% ≤ 100%: fine. With LT 95% and bonus 110% the product is 104.5%,
   which is rejected, because liquidating such a position would create bad debt
   immediately.

Disabling collateral (`liquidationThreshold == 0`) requires zero bonus and
**no suppliers** (`:147`), so existing positions cannot be stranded.

**The frozen/pending-LTV interaction** (`:152-163`):

```solidity
if (currentConfig.getFrozen()) {
  _pendingLtv[asset] = ltv;  newLtv = 0;  emit PendingLtvChanged(asset, ltv);
} else {
  if (_pendingLtv[asset] != 0) { delete _pendingLtv[asset]; emit PendingLtvChanged(asset, 0); }
  currentConfig.setLtv(ltv);
}
```

Setting an LTV on a **frozen** reserve does not apply it — it is parked in
`_pendingLtv` and the live LTV stays 0. Unfreezing restores it. This stops an
admin from accidentally re-enabling borrowing power against an asset that
governance has frozen.

### 17.4 Simple flag and parameter setters

| Function | Line | Access | Validation |
|---|---|---|---|
| `setReserveBorrowing(asset, enabled)` | `:110-115` | `onlyRiskOrPoolAdmins` | — |
| `setReserveFlashLoaning(asset, enabled)` | `:174-183` | `onlyRiskOrPoolAdmins` | — |
| `setReserveActive(asset, active)` | `:186-192` | `onlyPoolAdmin` | deactivating calls `_checkNoSuppliers` (`:187`) |
| `setReserveFactor(asset, newReserveFactor)` | `:273-288` | `onlyRiskOrPoolAdmins` | `<= 100%`; **syncs indexes before and rates after** |
| `setBorrowCap(asset, newBorrowCap)` | `:291-301` | `onlyRiskOrPoolAdmins` | range enforced by the setter |
| `setSupplyCap(asset, newSupplyCap)` | `:303-313` | `onlyRiskOrPoolAdmins` | same |
| `setLiquidationProtocolFee(asset, newFee)` | `:315-326` | `onlyRiskOrPoolAdmins` | `<= 100%` |
| `setReserveInterestRateData(asset, rateData)` | `:457-471` | `onlyRiskOrPoolAdmins` | sync indexes → set params → sync rates |
| `updateFlashloanPremium(newFlashloanPremium)` | `:490-500` | `onlyPoolAdmin` | `<= 100%` |
| `getPendingLtv(asset)` | `:503-505` | view | |

**The sync sandwich** in `setReserveFactor` (`:277`, `:284`) and
`setReserveInterestRateData` (`:461`, `:470`):

```solidity
_pool.syncIndexesState(asset);      // accrue at the OLD parameters
... change the parameter ...
_pool.syncRatesState(asset);        // recompute rates at the NEW parameters
```

Without the first call, changing the reserve factor would retroactively re-split
interest that had already accrued. Without the second, the new parameters would
not take effect until the next user action.

### 17.5 eMode configuration

**`setEModeCategory(categoryId, ltv, liquidationThreshold, liquidationBonus, isolated, label)`** — `:328-372`.
`onlyRiskOrPoolAdmins`. **[3.7]** added the `isolated` parameter. Validates
`ltv != 0` (`:336`), `liquidationThreshold != 0` (`:337`),
`ltv <= liquidationThreshold` (`:342`), plus the same bonus and
threshold×bonus ≤ 100% rules as the reserve version. Emits `EModeCategoryAdded`
and `EModeCategoryIsolationChanged`.

**`setAssetCollateralInEMode(asset, categoryId, allowed)`** — `:376-405`.
`onlyRiskOrPoolAdmins`. Reads the bitmap, checks the asset is listed
(`:381`, `AssetNotListed`), and when enabling requires the reserve is **not
frozen** (`:390`, `ReserveFrozen`). Writes back via
`_pool.configureEModeCategoryCollateralBitmap`, emits
`AssetCollateralInEModeChanged`. **[3.6]** reworked this to handle the decoupled
model — enabling an asset as eMode collateral no longer requires it to be
collateral outside eMode.

**`setAssetBorrowableInEMode(asset, categoryId, borrowable)`** — `:409-423`. Same
shape for `borrowableBitmap`; emits `AssetBorrowableInEModeChanged`.

**`setAssetLtvzeroInEMode(asset, categoryId, ltvzero)`** — `:427-444`. **[3.6]**.
Requires the asset to already be **eMode collateral** in that category
(`Errors.MustBeEmodeCollateral`), and when clearing the flag requires the reserve
is not frozen. Delegates to `_setEmodeLtvZero`.

**`setEModeCategoryIsolated(categoryId, isolated)`** — `:448-453`. **[3.7]**.
Sets the flag independently; emits `EModeCategoryIsolationChanged`.

### 17.6 Pause, freeze and grace periods

**`setReservePause(asset, paused, gracePeriod)`** — `:240-258`, `public`,
`onlyEmergencyOrPoolAdmin`. When **unpausing** with a non-zero grace period:

```solidity
require(gracePeriod <= MAX_GRACE_PERIOD, Errors.InvalidGracePeriod());   // :246
uint40 until = uint40(block.timestamp) + gracePeriod;
_pool.setLiquidationGracePeriod(asset, until);
emit LiquidationGracePeriodChanged(asset, until);
```

**[3.1]**. After an emergency pause, prices may have moved a long way while users
could not act. The grace period blocks liquidations for up to 4 hours so
positions can be repaired first.

**`setReservePause(asset, paused)`** — `:260-262`, overload with grace 0.

**`disableLiquidationGracePeriod(asset)`** — `:265-271`,
`onlyEmergencyOrPoolAdmin`. Sets the grace to 0, re-enabling liquidations
immediately.

**`setPoolPause(paused, gracePeriod)`** — `:474-482`, `public`,
`onlyEmergencyOrPoolAdmin`. Loops every reserve calling `setReservePause`. Note
this is a single transaction touching up to 128 reserves; on a large market it is
gas-heavy but it is the emergency lever.
**`setPoolPause(paused)`** — `:485-487`, overload with grace 0.

**`setReserveFreeze(asset, freeze)`** — `:195-227`,
`onlyRiskOrPoolOrEmergencyAdmins`. **[3.6] made this much more powerful:**

```solidity
require(freeze != currentConfig.getFrozen(), Errors.InvalidFreezeState());   // :202
currentConfig.setFrozen(freeze);
if (freeze) {
  _setReserveLtvzero(asset, true, currentConfig);                            // :207
  for (uint256 j = 1; j <= type(uint8).max; j++) {                           // :213
    collateralEnabledBitmap = _pool.getEModeCategoryCollateralBitmap(uint8(j));
    if (EModeConfiguration.isReserveEnabledOnBitmap(collateralEnabledBitmap, reserveData.id)) {
      ltvzeroBitmap = _pool.getEModeCategoryLtvzeroBitmap(uint8(j));
      _setEmodeLtvZero(ltvzeroBitmap, asset, reserveData.id, uint8(j), true);
    }
  }
}
```

Freezing now **also sets ltv0 outside eMode and in every eMode where the asset is
collateral**. The loop scans all 255 categories; the comment (`:210-211`) is
candid about the cost: "worst case 2 * 255 SLOADs + 255 SSTOREs, which should be
around ~6M gas", while noting real assets are in very few categories.

Crucially, **unfreezing does not automatically restore LTV.** The 3.6 docs spell
out the recovery sequence: `setReserveFreeze(false)`, then `setReserveLtvzero(...,
false)`, then `setAssetLtvzeroInEMode(..., false)` per category. This is
deliberate — it lets an emergency admin lift the freeze without immediately
restoring borrowing power.

**`setReserveLtvzero(asset, ltvZero)`** — `:230-238`,
`onlyRiskOrPoolOrEmergencyAdmins`. **[3.6]**. Applies ltv0 outside eMode without
freezing.

### 17.7 Internal helpers

**`_setReserveLtvzero(asset, ltvZero, currentConfig)`** — `:507-534`. Parks or
restores the LTV via `_pendingLtv`:

- Setting: if LTV is already 0, return (`:515`). Otherwise save it to
  `_pendingLtv` and zero the live LTV (`:516-518`).
- Clearing: **requires the reserve is not frozen** (`:521`, comment at `:520`
  says "ltvzero can only be removed on non frozen reserves"). If there is nothing
  pending, or the live LTV is already non-zero, return (`:523`). Otherwise
  restore and delete the pending entry (`:524-525`).

Emits `PendingLtvChanged` and `CollateralConfigurationChanged` (`:527-533`).

**`_setEmodeLtvZero(ltvzeroBitmap, reserve, reserveId, eModeCategoryId, ltvzero)`** — `:536-546`.
Flips one bit, writes the bitmap through `_pool.configureEModeCategoryLtvzeroBitmap`,
emits `AssetLtvzeroInEModeChanged` **[3.6]**.

**`_checkAssetIsCollateral(asset)`** — `:548-564`, view returns bool.

**`_checkNoSuppliers(asset)`** — `:566-575`. Reverts `ReserveLiquidityNotZero`
unless the aToken's scaled supply and `accruedToTreasury` are both effectively
zero. Guards deactivation and collateral-disabling.

**`_checkNoBorrowers(asset)`** — `:577-582`. Reverts `ReserveDebtNotZero` unless
the debt token's total supply is zero.

---

## 18. Tokenization

Two token types, one shared base stack:

```
                      IncentivizedERC20            (ERC20 + rewards hook)
                             │
                    MintableIncentivizedERC20      (adds _mint / _burn)
                             │
                    ScaledBalanceTokenBase         (scaled ↔ actual conversion)
                        │            │
                    AToken       VariableDebtToken  (+ DebtTokenBase)
                        │
              ATokenWithDelegation (+ BaseDelegation)
```

**The invariant that defines both:** the ERC-20 `balanceOf` storage slot holds a
**scaled** balance; the public `balanceOf` multiplies it by the reserve's current
index at call time. Interest accrues to everyone by moving one number.

### 18.0 `EIP712Base`

`tokenization/base/EIP712Base.sol` (70 lines). `abstract`.

| Member | Line | Notes |
|---|---|---|
| `bytes public constant EIP712_REVISION = bytes('1')` | `:10` | |
| `bytes32 internal constant EIP712_DOMAIN` | `:11` | the domain typehash |
| `uint256 internal immutable _chainId` | `:18` | captured at construction |
| `DOMAIN_SEPARATOR()` | `:32` | returns the cached separator, or recomputes if `block.chainid != _chainId` |
| `nonces(address owner)` | `:44` | per-owner signature nonce |
| `_calculateDomainSeparator()` | `:52` | |
| `_EIP712BaseId()` | `:69` | `virtual`, overridden per token |

The chain-id check is the standard fork protection: a chain split changes
`block.chainid`, and the separator is recomputed rather than reused, so old
signatures do not replay on the fork.

### 18.1 `IncentivizedERC20`

`tokenization/base/IncentivizedERC20.sol` (324 lines). `abstract`.

**The packed user state** (`:55-61`) is the key design:

```solidity
struct UserState {
  uint120 balance;          // :56   ← SCALED balance
  uint128 additionalData;   // :58   ← the index at the user's last interaction
}
mapping(address => UserState) internal _userState;   // :61
```

`uint120 + uint128 = 248` bits, so a user's balance **and** their last-seen index
share **one storage slot**. Every mint, burn and transfer touches exactly one
slot. `additionalData` is what `getPreviousIndex` returns and what lets the token
compute "interest accrued since you last touched this" for event purposes.

| Member | Line | Notes |
|---|---|---|
| `IPoolAddressesProvider internal immutable _addressesProvider` | `:72` | |
| `IPool public immutable POOL` | `:73` | |
| `modifier onlyPoolAdmin()` | `:36` | via `ACLManager` |
| `modifier onlyPool()` | `:45-47` | `require(_msgSender() == address(POOL), Errors.CallerMustBePool())` |
| `name/symbol/decimals` | `:104/:109/:114` | stored, settable by `_setName` etc. (`:305/:313/:321`) |
| `totalSupply()` | `:119` | **scaled** total; overridden in both children |
| `balanceOf(account)` | `:124-126` | **scaled**; overridden in both children |
| `transfer / approve / allowance / transferFrom` | `:137/:152/:144/:158` | standard ERC-20 surface |
| `renounceAllowance(owner)` | `:178-180` | **[3.6]** sets `_allowances[owner][msg.sender] = 0` |
| `increaseAllowance / decreaseAllowance` | `:190/:208` | **deprecated [3.6]** — modern OpenZeppelin dropped them |
| `_spendAllowance(owner, spender, amount, correctedAmount)` | `:235-260` | see below |
| `_transfer(sender, recipient, amount)` | `:266-286` | moves scaled balances, then fires rewards hooks |
| `_approve(owner, spender, amount, emitEvent)` | `:288` | |

**`_spendAllowance`** (`:235-260`) has four notable behaviours:

```solidity
uint256 currentAllowance = _allowances[owner][spender];
if (currentAllowance < amount) revert ERC20InsufficientAllowance(spender, currentAllowance, amount);
if (currentAllowance == type(uint256).max) return;                     // :244-246
uint256 consumption = currentAllowance >= correctedAmount ? correctedAmount : currentAllowance;
_approve({owner: owner, spender: spender, amount: currentAllowance - consumption,
          emitEvent: false});                                          // :251-256
```

1. It reverts with the **OpenZeppelin-style custom error**
   `ERC20InsufficientAllowance`, not an `Errors.*` code.
2. **Infinite allowance is not consumed** (`:244-246`) — **[3.6]**, matching
   modern OZ.
3. It takes **two** amounts: `amount` is what must be *authorised*,
   `correctedAmount` is what is actually *consumed*. See
   [§18.3](#183-atoken) for why they differ.
4. `emitEvent: false` (`:255`) — **[3.6]** stopped emitting `Approval` on spend,
   saving ~2k gas for heavy `transferFrom` users like the ParaSwap adapters.

**`_transfer`** (`:266-286`) moves the scaled balances and then calls
`REWARDS_CONTROLLER.handleAction` for sender and recipient (`:275-282`), passing
each one's **old** balance and the current total supply — the rewards accounting
needs the pre-transfer state. The recipient hook is skipped on a self-transfer.

### 18.2 `MintableIncentivizedERC20` and `ScaledBalanceTokenBase`

**`MintableIncentivizedERC20`** (`base/MintableIncentivizedERC20.sol`, 64 lines)
adds just `_mint(account, uint120 amount)` (`:36`) and
`_burn(account, uint120 amount)` (`:53`), each updating `_totalSupply` and the
packed balance and firing the rewards hook.

**`ScaledBalanceTokenBase`** (`base/ScaledBalanceTokenBase.sol`, 135 lines) is
where scaled accounting lives.

| Function | Line | Returns |
|---|---|---|
| `scaledBalanceOf(user)` | `:39-41` | `super.balanceOf(user)` — the raw stored value |
| `getScaledUserBalanceAndSupply(user)` | `:44-48` | both, in one call |
| `scaledTotalSupply()` | `:51-53` | `super.totalSupply()` |
| `getPreviousIndex(user)` | `:56-58` | `_userState[user].additionalData` |

**`_mintScaled(caller, onBehalfOf, amountScaled, index, getTokenBalance)`** — `:69-92`:

```solidity
require(amountScaled != 0, Errors.InvalidMintAmount());                        // :76
uint256 scaledBalance = super.balanceOf(onBehalfOf);
uint256 nextBalance    = getTokenBalance(amountScaled + scaledBalance, index);
uint256 previousBalance = getTokenBalance(scaledBalance, _userState[onBehalfOf].additionalData);
uint256 balanceIncrease = getTokenBalance(scaledBalance, index) - previousBalance;
_userState[onBehalfOf].additionalData = index.toUint128();                      // :83
_mint(onBehalfOf, amountScaled.toUint120());                                    // :85
uint256 amountToMint = nextBalance - previousBalance;
emit Transfer(address(0), onBehalfOf, amountToMint);
emit Mint(caller, onBehalfOf, amountToMint, balanceIncrease, index);
return (scaledBalance == 0);                                                    // :91
```

**The `getTokenBalance` parameter is a function pointer** (`:74`) — the caller
passes `TokenMath.getATokenBalance` or `TokenMath.getVTokenBalance`, so the same
code serves both tokens with opposite rounding. That is an unusual and elegant
use of Solidity's internal function types.

The emitted `Transfer` amount is `nextBalance - previousBalance`, which includes
**both** the new deposit and the interest accrued since the user's last touch.
An off-chain indexer summing `Transfer` events therefore sees interest as
transfers from the zero address. `balanceIncrease` reports the interest portion
separately.

The return value `scaledBalance == 0` is the `isFirstSupply` flag that
`SupplyLogic` uses to auto-enable collateral.

**`_burnScaled(user, target, amountScaled, index, getTokenBalance)`** — `:105-134`:
mirror image, with one twist documented at `:96-97` — **a burn can emit a `Mint`
event**:

```solidity
if (nextBalance > previousBalance) {                     // :123
  emit Transfer(address(0), user, nextBalance - previousBalance);
  emit Mint(user, user, ...);
} else {
  emit Transfer(user, address(0), previousBalance - nextBalance);
  emit Burn(user, target, ...);
}
```

If you withdraw less than the interest you accrued since your last interaction,
your balance still went **up**, so a `Mint` is the honest event. Indexers that
assume `burn ⇒ Burn` get this wrong.

Returns `scaledBalance - amountScaled == 0`, the `zeroBalanceAfterBurn` flag.

### 18.3 `AToken`

`tokenization/AToken.sol` (338 lines).

| Member | Line | Notes |
|---|---|---|
| `PERMIT_TYPEHASH` | `:29` | EIP-2612 |
| `address public immutable TREASURY` | `:32` | |
| `constructor` | `:43` | |
| `initialize(...)` | `:53` | proxy initializer |

| Function | Line | Access | Notes |
|---|---|---|---|
| `mint(caller, onBehalfOf, scaledAmount, index)` | `:63-78` | `onlyPool` | `_mintScaled` with `TokenMath.getATokenBalance`; returns `isFirstSupply` |
| `burn(from, receiverOfUnderlying, amount, scaledAmount, index)` | `:80-100` | `onlyPool` | `_burnScaled`, then transfers underlying **unless** the receiver is the aToken itself |
| `mintToTreasury(scaledAmount, index)` | `:102-114` | `onlyPool` | early-returns on zero; mints to `TREASURY` |
| `transferOnLiquidation(from, to, amount, scaledAmount, index)` | `:116-131` | `onlyPool` | internal `_transfer` overload, no HF check on `to` |
| `balanceOf(user)` | `:133-139` | view | `scaledBalance.getATokenBalance(POOL.getReserveNormalizedIncome(...))` |
| `totalSupply()` | `:141-144` | view | scaled total × normalized income |
| `RESERVE_TREASURY_ADDRESS()` | `:146-149` | view | |
| `UNDERLYING_ASSET_ADDRESS()` | `:151-154` | view | |
| `transferUnderlyingTo(target, amount)` | `:156-159` | `onlyPool` | how borrowers and flash-loan receivers get paid |
| `permit(...)` | `:161-185` | — | EIP-2612 |
| `transferFrom(sender, recipient, amount)` | `:187-242` | — | see below |
| `_transfer(from, to, amount)` | `:244-276` | internal | the validated path |
| `_transfer(sender, recipient, scaledAmount, index, ...)` | `:278-311` | internal | the raw path |
| `DOMAIN_SEPARATOR()` / `nonces(owner)` | `:316` / `:324` | view | resolve the multiple-inheritance ambiguity |
| `rescueTokens(token, to, amount)` | `:334-337` | `onlyPoolAdmin` | cannot rescue the underlying (`Errors.UnderlyingCannotBeRescued`) |

**`transferFrom` and the `correctedAmount` problem** (`:187-242`). This carries
the longest comment block in the protocol (`:213-238`), and it is worth
understanding because it is a genuinely subtle scaled-token bug class.

The problem: `transferFrom(amount)` converts to scaled with `rayDivCeil`, so the
sender's balance can drop by slightly **more** than `amount`. If the allowance
were reduced by `amount`, a spender could repeatedly move slightly more value
than they were authorised for.

The fix: compute `amount_out` — the *actual* balance decrease — by simulating the
transfer, and pass it as `correctedAmount` to `_spendAllowance`. So:

- **`amount`** is what must be authorised (the `< amount` revert check).
- **`amount_out`** is what is actually consumed, capped at the current allowance.

The comment's compatibility note (`:229-233`) is the practical consequence: with
an allowance of exactly 100 and `amount = 100`, the call succeeds even if
`amount_out` is 101 — the consumption is capped at 100. With an allowance of 101,
101 is consumed. This keeps exact-allowance integrations working while removing
the drift.

**`_transfer(from, to, amount)`** (`:244-276`) is the validated path used by
`transfer` and `transferFrom`:

```solidity
uint256 index = POOL.getReserveNormalizedIncome(underlyingAsset);
uint256 scaledAmount = uint256(amount).getATokenTransferScaledAmount(index);   // ceil
_transfer({... scaledAmount: scaledAmount.toUint120(), index: index ...});
POOL.finalizeTransfer({asset: underlyingAsset, from: from, to: to,
                       scaledAmount: scaledAmount,
                       scaledBalanceFromBefore: <sender's pre-transfer scaled balance>});
```

**Every user-initiated aToken transfer calls back into `Pool.finalizeTransfer`**,
which runs `SupplyLogic.executeFinalizeTransfer` — the collateral-flag and
health-factor enforcement. That is why aTokens cannot be freely moved out of an
unhealthy position. `transferOnLiquidation` deliberately uses the *raw* overload
and skips this, because the liquidation logic has already done the accounting.

### 18.4 `DebtTokenBase`

`tokenization/base/DebtTokenBase.sol` (121 lines). `abstract`. Implements
**credit delegation**.

| Member | Line | Notes |
|---|---|---|
| `DELEGATION_WITH_SIG_TYPEHASH` | `:27` | |
| `approveDelegation(delegatee, amount)` | `:40-42` | caller authorises `delegatee` to borrow against their collateral |
| `renounceDelegation(delegator)` | `:45-47` | **[3.6]** a delegatee drops its own allowance |
| `delegationWithSig(delegator, delegatee, value, deadline, v, r, s)` | `:50-76` | EIP-712 gasless delegation |
| `borrowAllowance(fromUser, toUser)` | `:78-84` | view |
| `_approveDelegation(delegator, delegatee, amount)` | `:91-101` | emits `BorrowAllowanceDelegated` |
| `_decreaseBorrowAllowance(delegator, delegatee, amount, correctedAmount)` | `:103-120` | **[3.6] no longer emits** `BorrowAllowanceDelegated` |

**[3.6]** removed the event from `_decreaseBorrowAllowance` to match
OpenZeppelin's ERC-20, where `Approval` fires only on explicit approval. The
event now means "someone granted delegation", never "someone used it".

`renounceDelegation` **[3.6]** solves a real ecosystem problem: because debt is
rebasing, integrations over-approve, leaving large stale allowances. Previously
only the delegator could reduce them; now the delegatee can burn its own.

### 18.5 `VariableDebtToken`

`tokenization/VariableDebtToken.sol` (196 lines).
`abstract contract VariableDebtToken is DebtTokenBase, ScaledBalanceTokenBase, IVariableDebtToken` (`:22`).

| Function | Line | Notes |
|---|---|---|
| `initialize(...)` | `:59` | proxy initializer |
| `balanceOf(user)` | `:69-75` | `scaledBalance.getVTokenBalance(POOL.getReserveNormalizedVariableDebt(...))` — **ceil** |
| `mint(user, onBehalfOf, amount, amountScaled, index)` | `:77-130` | `onlyPool`. If `user != onBehalfOf`, decreases the borrow allowance with the same `correctedAmount` technique as `AToken.transferFrom` (comment `:104-...`). Returns the new `scaledTotalSupply` |
| `burn(from, scaledAmount, index)` | `:132-148` | `onlyPool`. Returns `(noMoreDebt, scaledTotalSupply)` |
| `totalSupply()` | `:150-154` | scaled total × normalized debt |
| `UNDERLYING_ASSET_ADDRESS()` | `:193` | view |

**Debt is non-transferable, and the ERC-20 surface is stubbed out to prove it:**

| Function | Line | Behaviour |
|---|---|---|
| `transfer(address,uint256)` | `:164` | reverts `OperationNotSupported` |
| `allowance(address,address)` | `:168` | reverts |
| `approve(address,uint256)` | `:172` | reverts |
| `transferFrom(address,address,uint256)` | `:176` | reverts |
| `renounceAllowance(address)` | `:180` | reverts |
| `increaseAllowance(address,uint256)` | `:184` | reverts |
| `decreaseAllowance(address,uint256)` | `:188` | reverts |

They exist rather than being simply absent so that a contract expecting an ERC-20
gets a clear revert instead of a missing-selector failure. Note
`renounceAllowance` reverts here while `renounceDelegation` (inherited from
`DebtTokenBase`) is the working equivalent — the 3.6 changelog mentions
`VariableDebtToken.renounceAllowance` existing, and it does: as a reverting stub.

### 18.6 `ATokenWithDelegation`

`tokenization/ATokenWithDelegation.sol` (129 lines).
`abstract contract ATokenWithDelegation is AToken, BaseDelegation` (`:18`).
Used **only for the AAVE aToken** (`:14`), so that supplying AAVE does not cost
you your governance power.

| Function | Line | Purpose |
|---|---|---|
| `_getDomainSeparator()` | `:40-42` | bridges `AToken`'s EIP-712 to `BaseDelegation` |
| `_getDelegationState(user)` | `:44-53` | reads the packed delegation state |
| `_getBalance(user)` | `:55-57` | the **scaled** balance is the voting weight |
| `_incrementNonces(user)` | `:59-64` | |
| `_setDelegationState(user, state)` | `:66-74` | |
| `_transfer(from, to, amount)` | `:86-113` | updates delegation balances, then `super._transfer` |
| `_mint(account, amount)` | `:115-123` | delegation-aware |
| `_burn(account, amount)` | `:125-129` | delegation-aware |

**Voting power is the scaled balance, not the actual balance** (`:55-57`). That
is the crucial trick: scaled balances do not change as interest accrues, so
delegation bookkeeping does not need to be touched every time the index moves.
Governance weight is proportional to scaled balance, which is proportional to
actual balance by the same index for everyone.

The NatSpec at `:79` notes the amount is divided by the index inside `_transfer`
to perform the scaling.

`DelegationMode` (`base/DelegationMode.sol`, 9 lines) is the enum:
`NO_DELEGATION`, `VOTING_DELEGATED`, `PROPOSITION_DELEGATED`,
`FULL_POWER_DELEGATED`.

### 18.7 `BaseDelegation`

`tokenization/delegation/BaseDelegation.sol` (441 lines) and its interface
`delegation/interfaces/IBaseDelegation.sol` (111 lines). Implements Aave's
two-power governance delegation: **voting power** and **proposition power** can be
delegated independently, which is what the four-value `DelegationMode` enum
encodes in two bits.

The contract is abstract over its storage access — `_getDelegationState`,
`_setDelegationState`, `_getBalance`, `_incrementNonces` and
`_getDomainSeparator` are all supplied by `ATokenWithDelegation`. That separation
is why the same delegation logic can back both the AAVE token and its aToken.

Public surface (from `IBaseDelegation.sol`): `delegate`, `delegateByType`,
`getDelegateeByType`, `getPowersCurrent`, `metaDelegate`, `metaDelegateByType`,
plus the `DelegateChanged` / `DelegatedPowerChanged` events.

**[3.4]** reworked this file substantially; `docs/3.4/appendix/BaseDelegation.diff`
and `ATokenWithDelegation.diff` in the repo record the exact changes.

---
