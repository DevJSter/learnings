# Aave v1 & v2 — Complete Contract & Function Reference

A mechanical, file-by-file, function-by-function reference for the two Aave
generations cloned in this folder:

| Version | Path | Solidity | .sol files | Contract LOC |
|---|---|---|---|---|
| **v1** (Jan 2020) | `aave/v1-aave-protocol/contracts/` | `^0.5.0` | 73 | 7,356 |
| **v2** (Dec 2020) | `aave/v2-protocol/contracts/` | `0.6.12` | 121 | 15,170 |

Every `path:line` below was verified with `grep -n` against these exact files.
**Every citation is written as a full path from `aave/`** so v1 and v2 can never
be confused.

**Companion documents.** `aave/AAVE-V1-V2-DEEP-DIVE.md` explains *why* these
designs look the way they do — the architecture story, the rate models, the
rebasing aToken, the migration narrative, with end-to-end traces. This file is
the opposite: it assumes you want the exhaustive surface, one entry per
function. For v3 and v4 see `aave/AAVE-DEEP-DIVE.md`,
`aave/V3-PROTOCOL-COMPLETE-REFERENCE.md`, `aave/AAVE-V4-DEEP-DIVE.md` and
`aave/V4-COMPLETE-REFERENCE.md`.

---

## Table of contents

- [Part 0 — File inventories](#part-0--file-inventories)
  - [0.1 v1 file inventory (73 files)](#01-v1-file-inventory-73-files)
  - [0.2 v2 file inventory (121 files)](#02-v2-file-inventory-121-files)
- [Part 1 — Aave v1](#part-1--aave-v1)
  - [1.1 Architecture and the delegatecall storage contract](#11-architecture-and-the-delegatecall-storage-contract)
  - [1.2 `CoreLibrary`](#12-corelibrary)
  - [1.3 `WadRayMath` (v1)](#13-wadraymath-v1)
  - [1.4 `LendingPool`](#14-lendingpool)
  - [1.5 `LendingPoolCore`](#15-lendingpoolcore)
  - [1.6 `LendingPoolDataProvider`](#16-lendingpooldataprovider)
  - [1.7 `LendingPoolLiquidationManager`](#17-lendingpoolliquidationmanager)
  - [1.8 `DefaultReserveInterestRateStrategy` (v1)](#18-defaultreserveinterestratestrategy-v1)
  - [1.9 `AToken` (v1) — the rebasing token with interest redirection](#19-atoken-v1--the-rebasing-token-with-interest-redirection)
  - [1.10 `LendingPoolConfigurator` (v1)](#110-lendingpoolconfigurator-v1)
  - [1.11 Configuration, fees, flashloan, misc, mocks](#111-configuration-fees-flashloan-misc-mocks)
  - [1.12 v1 revert-string table](#112-v1-revert-string-table)
  - [1.13 v1 events reference](#113-v1-events-reference)
  - [1.14 v1 storage layouts](#114-v1-storage-layouts)
  - [1.15 v1 ABI / selector tables](#115-v1-abi--selector-tables)
  - [1.16 v1 use cases](#116-v1-use-cases)
- [Part 2 — Aave v2](#part-2--aave-v2)
  - [2.1 Architecture](#21-architecture)
  - [2.2 `DataTypes` and `LendingPoolStorage`](#22-datatypes-and-lendingpoolstorage)
  - [2.3 Math libraries](#23-math-libraries)
  - [2.4 `ReserveConfiguration` — the bitmap](#24-reserveconfiguration--the-bitmap)
  - [2.5 `UserConfiguration` — 2 bits per reserve](#25-userconfiguration--2-bits-per-reserve)
  - [2.6 `ReserveLogic`](#26-reservelogic)
  - [2.7 `GenericLogic`](#27-genericlogic)
  - [2.8 `ValidationLogic`](#28-validationlogic)
  - [2.9 `Helpers`](#29-helpers)
  - [2.10 `LendingPool`](#210-lendingpool)
  - [2.11 `LendingPoolCollateralManager`](#211-lendingpoolcollateralmanager)
  - [2.12 `DefaultReserveInterestRateStrategy` (v2)](#212-defaultreserveinterestratestrategy-v2)
  - [2.13 Tokenization](#213-tokenization)
  - [2.14 `LendingPoolConfigurator` (v2)](#214-lendingpoolconfigurator-v2)
  - [2.15 Configuration and upgradeability](#215-configuration-and-upgradeability)
  - [2.16 `misc/` — oracle, gateway, data providers](#216-misc--oracle-gateway-data-providers)
  - [2.17 `adapters/` — flash-loan-powered position management](#217-adapters--flash-loan-powered-position-management)
  - [2.18 `flashloan/`, `deployments/`, `dependencies/`, `mocks/`](#218-flashloan-deployments-dependencies-mocks)
  - [2.19 The complete `Errors.sol` table](#219-the-complete-errorssol-table)
  - [2.20 v2 events reference](#220-v2-events-reference)
  - [2.21 v2 storage layouts](#221-v2-storage-layouts)
  - [2.22 v2 ABI / selector tables](#222-v2-abi--selector-tables)
  - [2.23 v2 use cases](#223-v2-use-cases)
- [Part 3 — v1 → v2 migration table](#part-3--v1--v2-migration-table)

---

# Part 0 — File inventories

Nothing in either repository is skipped. Files whose content is a vendored
dependency, a test mock, or a one-line interface get a short entry; everything
that carries protocol logic gets a full section later.

## 0.1 v1 file inventory (73 files)

### Core protocol (`aave/v1-aave-protocol/contracts/lendingpool/`)

| File | Lines | Purpose | Section |
|---|---:|---|---|
| `LendingPool.sol` | 1007 | User-facing entry point. Holds no funds; validates and forwards to `LendingPoolCore`. | [1.4](#14-lendingpool) |
| `LendingPoolCore.sol` | 1775 | **Holds every reserve's funds** and all reserve/user state. The largest contract in v1. | [1.5](#15-lendingpoolcore) |
| `LendingPoolDataProvider.sol` | 475 | Read-only aggregation: global account data, health factor, collateral checks. | [1.6](#16-lendingpooldataprovider) |
| `LendingPoolLiquidationManager.sol` | 355 | Liquidation logic, invoked by `LendingPool` via `delegatecall`. | [1.7](#17-lendingpoolliquidationmanager) |
| `LendingPoolConfigurator.sol` | 449 | Admin surface for listing and configuring reserves. | [1.10](#110-lendingpoolconfigurator-v1) |
| `DefaultReserveInterestRateStrategy.sol` | 199 | Kinked rate curve plus the weighted overall borrow rate. | [1.8](#18-defaultreserveinterestratestrategy-v1) |

### Libraries (`aave/v1-aave-protocol/contracts/libraries/`)

| File | Lines | Purpose | Section |
|---|---:|---|---|
| `CoreLibrary.sol` | 439 | `ReserveData` / `UserReserveData` structs plus index and rate math. | [1.2](#12-corelibrary) |
| `WadRayMath.sol` | 85 | Wad (1e18) and ray (1e27) fixed-point arithmetic, including `rayPow`. | [1.3](#13-wadraymath-v1) |
| `EthAddressLib.sol` | 11 | Returns the `0xEeee…EEeE` pseudo-address used to mean native ETH. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `openzeppelin-upgradeability/Proxy.sol` | 71 | Abstract delegatecall fallback. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `openzeppelin-upgradeability/BaseUpgradeabilityProxy.sol` | 64 | Implementation slot storage plus `_upgradeTo`. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `openzeppelin-upgradeability/UpgradeabilityProxy.sol` | 27 | Constructor-initialised upgradeable proxy. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `openzeppelin-upgradeability/BaseAdminUpgradeabilityProxy.sol` | 121 | Adds an admin able to upgrade, with the `ifAdmin` routing. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `openzeppelin-upgradeability/AdminUpgradeabilityProxy.sol` | 24 | Concrete admin proxy. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `openzeppelin-upgradeability/InitializableUpgradeabilityProxy.sol` | 28 | Proxy initialised after deployment rather than in the constructor. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `openzeppelin-upgradeability/InitializableAdminUpgradeabilityProxy.sol` | 27 | The proxy Aave actually deploys for every component. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `openzeppelin-upgradeability/Initializable.sol` | 62 | Single-shot `initializer` modifier. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `openzeppelin-upgradeability/VersionedInitializable.sol` | 70 | Revision-numbered initializer allowing re-initialisation on upgrade. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |

### Tokenization, configuration, fees, flashloan, misc

| File | Lines | Purpose | Section |
|---|---:|---|---|
| `tokenization/AToken.sol` | 674 | Rebasing interest-bearing token with the interest-redirection feature. | [1.9](#19-atoken-v1--the-rebasing-token-with-interest-redirection) |
| `configuration/LendingPoolAddressesProvider.sol` | 238 | Registry of every component address; deploys and upgrades their proxies. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `configuration/LendingPoolParametersProvider.sol` | 53 | Three global parameters: max stable-rate loan %, rebalance delta, flash-loan fees. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `configuration/AddressStorage.sol` | 14 | `bytes32 => address` key-value store backing the provider. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `configuration/UintStorage.sol` | 14 | `bytes32 => uint256` key-value store backing the parameters provider. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `fees/FeeProvider.sol` | 51 | Computes the loan origination fee. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `fees/TokenDistributor.sol` | 162 | Splits collected fees between receivers, burning the LEND share. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `flashloan/base/FlashLoanReceiverBase.sol` | 51 | Base class for flash-loan receivers; repayment helper. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `flashloan/interfaces/IFlashLoanReceiver.sol` | 12 | Declares `executeOperation(address,uint256,uint256,bytes)`. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `misc/ChainlinkProxyPriceProvider.sol` | 108 | Chainlink aggregation with a fallback oracle; prices denominated in ETH. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `misc/WalletBalanceProvider.sol` | 76 | Batch balance reads for front-ends. | [1.11](#111-configuration-fees-flashloan-misc-mocks) |
| `misc/IERC20DetailedBytes.sol` | 7 | ERC20 variant whose `symbol()` returns `bytes32` (for MKR-style tokens). | [1.11](#111-configuration-fees-flashloan-misc-mocks) |

### Interfaces (`aave/v1-aave-protocol/contracts/interfaces/`)

| File | Lines | Declares |
|---|---:|---|
| `ILendingPoolAddressesProvider.sol` | 43 | Every getter/setter of the address registry. |
| `IReserveInterestRateStrategy.sol` | 29 | `calculateInterestRates`, plus base-rate getters. |
| `IKyberNetworkProxyInterface.sol` | 21 | Kyber swap interface (unused by the core flow). |
| `IPriceOracle.sol` | 17 | `getAssetPrice` / `setAssetPrice` (mock-oriented). |
| `ILendingRateOracle.sol` | 18 | `getMarketBorrowRate` — the off-chain stable-rate anchor. |
| `IFeeProvider.sol` | 11 | `calculateLoanOriginationFee`, `getLoanOriginationFeePercentage`. |
| `IPriceOracleGetter.sol` | 11 | `getAssetPrice(address)`. |
| `IChainlinkAggregator.sol` | 11 | `latestAnswer` and its events. |

### Mocks and test helpers (35 files)

All under `aave/v1-aave-protocol/contracts/mocks/`. They carry no protocol logic
and exist only for the test suite:

- `flashloan/MockFlashLoanReceiver.sol` (50) — a receiver that can be told to fail, used to prove the flash-loan balance check reverts.
- `oracle/CLAggregators/MockAggregatorBase.sol` (15) plus 14 per-asset subclasses (`MockAggregatorBAT/DAI/KNC/LEND/LINK/MANA/MKR/REP/SUSD/TUSD/USDC/USDT/WBTC/ZRX.sol`, 6 lines each) — hardcoded Chainlink answers.
- `oracle/GenericOracleI.sol` (19), `oracle/LendingRateOracle.sol` (26), `oracle/PriceOracle.sol` (30) — settable oracles for tests.
- `tokens/MintableERC20.sol` (19) plus 13 per-asset subclasses (`MockBAT/DAI/KNC/LEND/LINK/MANA/MKR/REP/SUSD/TUSD/USDC/USDT/WBTC/ZRX.sol`, 11–12 lines each) — freely mintable test tokens with per-asset decimals.
- `upgradeability/MockLendingPoolCore.sol` (46) — a `LendingPoolCore` with a bumped revision, used to test that `VersionedInitializable` permits exactly one re-initialisation per revision.

## 0.2 v2 file inventory (121 files)

### Core protocol (`aave/v2-protocol/contracts/protocol/`)

| File | Lines | Purpose | Section |
|---|---:|---|---|
| `lendingpool/LendingPool.sol` | 946 | The single entry point. Holds no funds and no reserve math; delegates to libraries. | [2.10](#210-lendingpool) |
| `lendingpool/LendingPoolStorage.sol` | 32 | The storage layout `LendingPool` and `LendingPoolCollateralManager` must share. | [2.2](#22-datatypes-and-lendingpoolstorage) |
| `lendingpool/LendingPoolCollateralManager.sol` | 317 | Liquidation logic, `delegatecall`ed from `LendingPool`. | [2.11](#211-lendingpoolcollateralmanager) |
| `lendingpool/LendingPoolConfigurator.sol` | 487 | Admin surface: list reserves, deploy token proxies, set risk parameters. | [2.14](#214-lendingpoolconfigurator-v2) |
| `lendingpool/DefaultReserveInterestRateStrategy.sol` | 260 | Kinked rate curve; applies the reserve factor to the supply rate. | [2.12](#212-defaultreserveinterestratestrategy-v2) |
| `libraries/logic/ReserveLogic.sol` | 373 | Index accrual, treasury minting, rate refresh. | [2.6](#26-reservelogic) |
| `libraries/logic/GenericLogic.sol` | 275 | Health factor, account aggregation, collateral withdrawal checks. | [2.7](#27-genericlogic) |
| `libraries/logic/ValidationLogic.sol` | 469 | Every precondition for every user action. | [2.8](#28-validationlogic) |
| `libraries/configuration/ReserveConfiguration.sol` | 366 | The packed reserve configuration bitmap. | [2.4](#24-reserveconfiguration--the-bitmap) |
| `libraries/configuration/UserConfiguration.sol` | 111 | Two bits per reserve: borrowing, collateral. | [2.5](#25-userconfiguration--2-bits-per-reserve) |
| `libraries/math/WadRayMath.sol` | 135 | Wad/ray arithmetic with half-rounding. | [2.3](#23-math-libraries) |
| `libraries/math/PercentageMath.sol` | 54 | Basis-point arithmetic. | [2.3](#23-math-libraries) |
| `libraries/math/MathUtils.sol` | 84 | Linear interest and the binomial compounded-interest approximation. | [2.3](#23-math-libraries) |
| `libraries/types/DataTypes.sol` | 49 | `ReserveData`, the two configuration maps, `InterestRateMode`. | [2.2](#22-datatypes-and-lendingpoolstorage) |
| `libraries/helpers/Errors.sol` | 119 | All 80 numeric error codes. | [2.19](#219-the-complete-errorssol-table) |
| `libraries/helpers/Helpers.sol` | 39 | `getUserCurrentDebt` in storage and memory flavours. | [2.9](#29-helpers) |
| `libraries/aave-upgradeability/VersionedInitializable.sol` | 77 | Revision-gated initializer. | [2.15](#215-configuration-and-upgradeability) |
| `libraries/aave-upgradeability/BaseImmutableAdminUpgradeabilityProxy.sol` | 80 | Proxy whose admin is an immutable, saving an SLOAD per call. | [2.15](#215-configuration-and-upgradeability) |
| `libraries/aave-upgradeability/InitializableImmutableAdminUpgradeabilityProxy.sol` | 23 | The proxy used for aTokens and debt tokens. | [2.15](#215-configuration-and-upgradeability) |
| `configuration/LendingPoolAddressesProvider.sol` | 215 | Per-market registry; owns every proxy. | [2.15](#215-configuration-and-upgradeability) |
| `configuration/LendingPoolAddressesProviderRegistry.sol` | 89 | Registry of markets (each market is one addresses provider). | [2.15](#215-configuration-and-upgradeability) |

### Tokenization (`aave/v2-protocol/contracts/protocol/tokenization/`)

| File | Lines | Purpose | Section |
|---|---:|---|---|
| `AToken.sol` | 406 | Interest-bearing collateral token; **holds the reserve's underlying**. | [2.13](#213-tokenization) |
| `StableDebtToken.sol` | 435 | Non-transferable stable-rate debt with weighted-average rate bookkeeping. | [2.13](#213-tokenization) |
| `VariableDebtToken.sol` | 209 | Non-transferable variable-rate debt, scaled by the borrow index. | [2.13](#213-tokenization) |
| `IncentivizedERC20.sol` | 255 | ERC20 base that pings the incentives controller on every balance change. | [2.13](#213-tokenization) |
| `base/DebtTokenBase.sol` | 137 | Credit delegation allowances; disables all ERC20 transfer paths. | [2.13](#213-tokenization) |
| `DelegationAwareAToken.sol` | 30 | aToken variant that can delegate the underlying's voting power. | [2.13](#213-tokenization) |

### `misc/` (18 files)

| File | Lines | Purpose |
|---|---:|---|
| `AaveOracle.sol` | 127 | Chainlink sources plus fallback oracle; `getAssetPrice` in the base currency. |
| `WETHGateway.sol` | 189 | Wraps/unwraps ETH around deposit, withdraw, repay and borrow. |
| `AaveProtocolDataProvider.sol` | 180 | Canonical read API for reserve and user data. |
| `UiPoolDataProvider.sol` | 399 | Front-end aggregation (original). |
| `UiPoolDataProviderV2.sol` | 224 | Front-end aggregation (v2 markets). |
| `UiPoolDataProviderV2V3.sol` | 241 | Front-end aggregation compatible with both v2 and v3 markets. |
| `UiIncentiveDataProviderV2.sol` | 287 | Incentives aggregation for v2 markets. |
| `UiIncentiveDataProviderV2V3.sol` | 397 | Incentives aggregation for v2 and v3 markets. |
| `WalletBalanceProvider.sol` | 111 | Batch balance reads. |
| `interfaces/IAaveOracle.sol` | 24 | Oracle interface. |
| `interfaces/IERC20DetailedBytes.sol` | 11 | `bytes32` symbol variant. |
| `interfaces/IUiPoolDataProvider.sol` | 110 | Structs and signatures for the UI provider. |
| `interfaces/IUiPoolDataProviderV2.sol` | 81 | Same, v2 flavour. |
| `interfaces/IUiPoolDataProviderV3.sol` | 111 | Same, v3 flavour. |
| `interfaces/IUiIncentiveDataProviderV2.sol` | 57 | Incentives structs, v2. |
| `interfaces/IUiIncentiveDataProviderV3.sol` | 74 | Incentives structs, v3. |
| `interfaces/IWETH.sol` | 16 | `deposit`/`withdraw`/`approve`. |
| `interfaces/IWETHGateway.sol` | 30 | Gateway interface. |
| `interfaces/IUniswapV2Router01.sol` | 161 | Router surface used by the adapters. |
| `interfaces/IUniswapV2Router02.sol` | 51 | Router02 additions. |

### `adapters/` (8 files)

| File | Lines | Purpose | Section |
|---|---:|---|---|
| `BaseUniswapAdapter.sol` | 566 | Shared swap, pricing and aToken-pull logic for all Uniswap adapters. | [2.17](#217-adapters--flash-loan-powered-position-management) |
| `UniswapLiquiditySwapAdapter.sol` | 283 | Swap one collateral for another, optionally inside a flash loan. | [2.17](#217-adapters--flash-loan-powered-position-management) |
| `UniswapRepayAdapter.sol` | 266 | Repay debt using collateral. | [2.17](#217-adapters--flash-loan-powered-position-management) |
| `FlashLiquidationAdapter.sol` | 184 | Liquidate with no capital, funded by a flash loan. | [2.17](#217-adapters--flash-loan-powered-position-management) |
| `BaseParaSwapAdapter.sol` | 122 | ParaSwap equivalent of the Uniswap base. | [2.17](#217-adapters--flash-loan-powered-position-management) |
| `BaseParaSwapSellAdapter.sol` | 109 | Exact-in ParaSwap sell helper. | [2.17](#217-adapters--flash-loan-powered-position-management) |
| `ParaSwapLiquiditySwapAdapter.sol` | 210 | Collateral swap routed through ParaSwap. | [2.17](#217-adapters--flash-loan-powered-position-management) |
| `interfaces/IBaseUniswapAdapter.sol` | 90 | `PermitSignature`, `AmountCalc` and the adapter surface. | [2.17](#217-adapters--flash-loan-powered-position-management) |

### `interfaces/` (25 files)

`ILendingPool.sol` (410) is the full pool ABI including every event.
`IAaveIncentivesController.sol` (148) declares `handleAction` and the claim
surface. `IAToken.sol` (107), `IStableDebtToken.sol` (133),
`IVariableDebtToken.sol` (62), `IScaledBalanceToken.sol` (26),
`ICreditDelegationToken.sol` (28), `IInitializableAToken.sol` (55) and
`IInitializableDebtToken.sol` (51) describe the token layer.
`ILendingPoolAddressesProvider.sol` (60),
`ILendingPoolAddressesProviderRegistry.sol` (26),
`ILendingPoolConfigurator.sol` (179) and
`ILendingPoolCollateralManager.sol` (60) cover configuration.
`IPriceOracle.sol` (17), `IPriceOracleGetter.sol` (16),
`ILendingRateOracle.sol` (19), `IChainlinkAggregator.sol` (18) and
`IReserveInterestRateStrategy.sol` (47) cover pricing and rates.
`IERC20WithPermit.sol` (16), `IDelegationToken.sol` (11),
`IExchangeAdapter.sol` (23), `IUniswapExchange.sol` (21),
`IUniswapV2Router02.sol` (30), `IParaSwapAugustus.sol` (7) and
`IParaSwapAugustusRegistry.sol` (7) cover integrations.

### `flashloan/`, `deployments/`, `dependencies/`, `mocks/`

| File | Lines | Purpose |
|---|---:|---|
| `flashloan/base/FlashLoanReceiverBase.sol` | 22 | Stores `ADDRESSES_PROVIDER` and `LENDING_POOL`. |
| `flashloan/interfaces/IFlashLoanReceiver.sol` | 25 | Multi-asset `executeOperation(address[],uint256[],uint256[],address,bytes)`. |
| `deployments/ATokensAndRatesHelper.sol` | 86 | Batch-deploys tokens and rate strategies during market setup. |
| `deployments/StableAndVariableTokensHelper.sol` | 47 | Batch-deploys the two debt token implementations. |
| `deployments/StringLib.sol` | 8 | `concat` for building token names. |
| `dependencies/openzeppelin/contracts/*` | 9 files, 936 | Vendored `ERC20`, `IERC20`, `IERC20Detailed`, `SafeERC20`, `SafeMath`, `Address`, `Context`, `Ownable`, `ReentrancyGuard`. |
| `dependencies/openzeppelin/upgradeability/*` | 8 files, 465 | Vendored proxy set, mirroring v1's. |
| `mocks/dependencies/weth/WETH9.sol` | 758 | The canonical WETH9 source, for tests. |
| `mocks/flashloan/MockFlashLoanReceiver.sol` | 84 | Configurable-failure receiver. |
| `mocks/attacks/SefldestructTransfer.sol` | 8 | Force-sends ETH via `selfdestruct` to test balance assumptions. |
| `mocks/oracle/*` | 6 files, 124 | Settable price and lending-rate oracles, aggregator mocks. |
| `mocks/swap/*` | 4 files, 199 | Mock Uniswap router and ParaSwap Augustus, registry and transfer proxy. |
| `mocks/tokens/*` | 3 files, 74 | `MintableERC20`, `MintableDelegationERC20`, `WETH9Mocked`. |
| `mocks/upgradeability/*` | 3 files, 32 | Revision-bumped `MockAToken`, `MockStableDebtToken`, `MockVariableDebtToken`. |

---

# Part 1 — Aave v1

## 1.1 Architecture and the delegatecall storage contract

```
                            ┌──────────────────────────────────┐
                            │  LendingPoolAddressesProvider    │
                            │  bytes32 => address registry,    │
                            │  owns every component's proxy    │
                            └───────────────┬──────────────────┘
                                            │ getX()
   user ──deposit/borrow/repay/…──►  ┌──────┴───────┐
                                     │ LendingPool  │  logic + validation, holds NOTHING
                                     └──┬────┬───┬──┘
                     updateStateOn*()   │    │   │  delegatecall("liquidationCall(...)")
                     transferTo*()      │    │   └────────────► LendingPoolLiquidationManager
                                        │    │                   (must share LendingPool's
                                        │    │                    storage prefix — see below)
                                        │    │ calculateUserGlobalData()
                                        │    └──────────────────► LendingPoolDataProvider
                                        ▼                              │ getAssetPrice()
                              ┌──────────────────┐                     ▼
                              │ LendingPoolCore  │◄──────── ChainlinkProxyPriceProvider
                              │  **HOLDS ALL     │
                              │    FUNDS**       │ calculateInterestRates()
                              │  reserves[]      │────────► DefaultReserveInterestRateStrategy
                              │  usersReserveData│                     │ getMarketBorrowRate()
                              └────────┬─────────┘                     ▼
                                       │ mintOnDeposit / burnOnLiquidation   LendingRateOracle
                                       ▼
                                    AToken  ──redeem()──► LendingPool.redeemUnderlying()
```

Two structural facts define v1 and both were reversed in v2:

1. **`LendingPoolCore` custodies every asset.** `transferToReserve` pulls
   underlying into the Core; `transferToUser` pays out of it
   (`aave/v1-aave-protocol/contracts/lendingpool/LendingPoolCore.sol:397`,
   `:472`). aTokens hold nothing.
2. **`LendingPool` is a thin validator.** It reads via
   `LendingPoolDataProvider`, mutates via `LendingPoolCore.updateStateOn*`, and
   moves value via `LendingPoolCore.transferTo*`.

### Why the `delegatecall` into `LendingPoolLiquidationManager` is safe

`LendingPool.liquidationCall`
(`aave/v1-aave-protocol/contracts/lendingpool/LendingPool.sol:805`) does not
call the manager — it `delegatecall`s it, so the manager executes against
`LendingPool`'s storage. That only works because the two contracts declare an
identical storage prefix, with identical inheritance order:

```solidity
// LendingPool.sol:27-36
contract LendingPool is ReentrancyGuard, VersionedInitializable {
    LendingPoolAddressesProvider public addressesProvider;   // slot n+0
    LendingPoolCore public core;                             // slot n+1
    LendingPoolDataProvider public dataProvider;             // slot n+2
    LendingPoolParametersProvider public parametersProvider; // slot n+3
    IFeeProvider feeProvider;                                // slot n+4
```

```solidity
// LendingPoolLiquidationManager.sol:23-33
contract LendingPoolLiquidationManager is ReentrancyGuard, VersionedInitializable {
    LendingPoolAddressesProvider public addressesProvider;   // slot n+0
    LendingPoolCore core;                                    // slot n+1
    LendingPoolDataProvider dataProvider;                    // slot n+2
    LendingPoolParametersProvider parametersProvider;        // slot n+3
    IFeeProvider feeProvider;                                // slot n+4
    address ethereumAddress;                                 // slot n+5  (extra, harmless)
```

The manager's variables are never written, only read, so the trailing extra slot
is inert. Note `LendingPoolLiquidationManager.getRevision()` returns `0`
(`:111-113`) precisely because it is never initialised as a proxy itself; it only
exists to be delegated into.

## 1.2 `CoreLibrary`

`aave/v1-aave-protocol/contracts/libraries/CoreLibrary.sol` — 439 lines. Defines
the two state structs and every piece of index/rate math. `using CoreLibrary for
CoreLibrary.ReserveData` is applied in `LendingPoolCore.sol:29-30`.

### `enum InterestRateMode` — `:15`

`{NONE, STABLE, VARIABLE}` — so `1` means stable and `2` means variable. Callers
pass a raw `uint256` and it is cast at `LendingPool.sol:410`.

### `struct UserReserveData` — `:19-31`

| Field | Type | Meaning |
|---|---|---|
| `principalBorrowBalance` | `uint256` | Debt as of the user's last interaction, in token units. **Not** scaled by an index. |
| `lastVariableBorrowCumulativeIndex` | `uint256` (ray) | The reserve's variable index snapshot when the user last acted. Zero for stable borrowers. |
| `originationFee` | `uint256` | Accumulated unpaid origination fees. |
| `stableBorrowRate` | `uint256` (ray) | The rate locked in at borrow time. Zero for variable borrowers. |
| `lastUpdateTimestamp` | `uint40` | When the user last acted, used for stable-rate compounding. |
| `useAsCollateral` | `bool` | Whether this deposit backs borrows. |

The v1 signature is that **debt is per-user principal plus a per-user index or
rate**, not a scaled balance. `stableBorrowRate > 0` is the discriminator for
which mode a user is in (`getUserCurrentBorrowRateMode`,
`LendingPoolCore.sol:926`).

### `struct ReserveData` — `:33-79`

| Field | Type | Meaning |
|---|---|---|
| `lastLiquidityCumulativeIndex` | `uint256` (ray) | Supply index; grows **linearly**. |
| `currentLiquidityRate` | `uint256` (ray) | Current supply APR. |
| `totalBorrowsStable` | `uint256` | Stable-rate principal outstanding. |
| `totalBorrowsVariable` | `uint256` | Variable-rate principal outstanding. |
| `currentVariableBorrowRate` | `uint256` (ray) | Current variable APR. |
| `currentStableBorrowRate` | `uint256` (ray) | Rate a *new* stable borrower would lock in. |
| `currentAverageStableBorrowRate` | `uint256` (ray) | Weighted average across all existing stable loans. |
| `lastVariableBorrowCumulativeIndex` | `uint256` (ray) | Variable borrow index; **compounds**. |
| `baseLTVasCollateral` | `uint256` | LTV in whole percent (0–100), not bps. |
| `liquidationThreshold` | `uint256` | Threshold in whole percent. |
| `liquidationBonus` | `uint256` | Bonus in whole percent (e.g. `105` = 5% bonus). |
| `decimals` | `uint256` | Underlying decimals. |
| `aTokenAddress` | `address` | Overlying token. |
| `interestRateStrategyAddress` | `address` | Rate model. |
| `lastUpdateTimestamp` | `uint40` | Last index update. |
| `borrowingEnabled` / `usageAsCollateralEnabled` / `isStableBorrowRateEnabled` / `isActive` / `isFreezed` | `bool` | Flags, each a full storage slot — v2 packs all of these into one word. |

Everything is a full `uint256`: a `ReserveData` costs roughly 20 storage slots.
This is the single biggest reason v1 was gas-expensive, and directly motivated
v2's `ReserveConfigurationMap` bitmap ([2.4](#24-reserveconfiguration--the-bitmap)).

### `getNormalizedIncome(ReserveData storage) internal view returns (uint256)` — `:89`

- **Purpose.** Current supply index including interest accrued since the last write.
- **Returns.** `calculateLinearInterest(currentLiquidityRate, lastUpdateTimestamp) × lastLiquidityCumulativeIndex`, in ray.
- **Callers.** `LendingPoolCore.getReserveNormalizedIncome` (`:621`), which the aToken calls on every balance read (`AToken.sol:466`, `:531`).
- **Gotcha.** A pure view — it never writes the index, so an aToken balance can grow between two calls with no transaction in between.

### `updateCumulativeIndexes(ReserveData storage)` — `:111`

- **Purpose.** Advance both indexes to `block.timestamp`.
- **Checks.** `if (totalBorrows > 0)` — with no debt, no interest accrues and both indexes are left untouched.
- **State writes.** `lastLiquidityCumulativeIndex` via linear interest, `lastVariableBorrowCumulativeIndex` via compounded interest.
- **Gotcha.** It does **not** write `lastUpdateTimestamp`; that happens in `LendingPoolCore.updateReserveInterestRatesAndTimestampInternal` (`:1703`). Every mutator therefore calls `updateCumulativeIndexes()` first and the rate/timestamp refresh last, and the order matters: reversing them would accrue interest over a zero interval.

```solidity
// CoreLibrary.sol:111-131
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

**Suppliers earn linear, borrowers pay compound.** The difference is retained by
the protocol as a rounding-level surplus. v2 keeps exactly this asymmetry
([2.6](#26-reservelogic)).

### `cumulateToLiquidityIndex(ReserveData storage, uint256 _totalLiquidity, uint256 _amount)` — `:142`

- **Purpose.** Distribute a one-off income (the flash-loan fee) to all suppliers at once.
- **Math.** `index *= (1 + amount/totalLiquidity)`.
- **Callers.** `LendingPoolCore.updateStateOnFlashLoan` (`:150`).
- **Gotcha.** `_totalLiquidity` must be the pre-fee total, which is why `LendingPool.flashLoan` snapshots `availableLiquidityBefore` and passes it down (`LendingPool.sol:851`, `LendingPoolCore.sol:161`).

### `init(ReserveData storage, address _aTokenAddress, uint256 _decimals, address _interestRateStrategyAddress) external` — `:163`

- **Checks.** `require(_self.aTokenAddress == address(0), "Reserve has already been initialized")`.
- **State writes.** Sets both indexes to `1e27` if zero, stores the aToken, decimals and strategy, sets `isActive = true`, `isFreezed = false`.
- **Note.** Declared `external` on a library, so it is a real `DELEGATECALL` to a deployed library rather than inlined — the same linking arrangement v2 uses for its logic libraries.

### `enableBorrowing(ReserveData storage, bool) external` — `:194` / `disableBorrowing(ReserveData storage) external` — `:206`

`enableBorrowing` reverts with `"Reserve is already enabled"` if borrowing is on;
it also sets `isStableBorrowRateEnabled`. `disableBorrowing` is unconditional.

### `enableAsCollateral(ReserveData storage, uint256 _baseLTVasCollateral, uint256 _liquidationThreshold, uint256 _liquidationBonus) external` — `:217` / `disableAsCollateral` — `:242`

Reverts with `"Reserve is already enabled as collateral"` when already enabled.
Also initialises `lastLiquidityCumulativeIndex` to ray if it is still zero, which
covers a reserve enabled as collateral before being enabled for borrowing.

### `getCompoundedBorrowBalance(UserReserveData storage, ReserveData storage) internal view returns (uint256)` — `:254`

This is the heart of v1 debt accounting.

- **Returns `0`** when `principalBorrowBalance == 0`.
- **Stable branch** (`_self.stableBorrowRate > 0`): compound the user's own fixed rate from the user's own `lastUpdateTimestamp`.
- **Variable branch**: `compoundedInterest(reserveRate, reserveTimestamp) × reserveIndex / userIndex` — the classic index-ratio, but computed live rather than from a stored index.

```solidity
// CoreLibrary.sol:275-286
} else {
    //variable interest
    cumulatedInterest = calculateCompoundedInterest(
        _reserve.currentVariableBorrowRate,
        _reserve.lastUpdateTimestamp
    )
        .rayMul(_reserve.lastVariableBorrowCumulativeIndex)
        .rayDiv(_self.lastVariableBorrowCumulativeIndex);
}
compoundedBalance = principalBorrowBalanceRay.rayMul(cumulatedInterest).rayToWad();
```

- **The 1 wei rule** (`:288-296`): if rounding produced no growth but time has passed, return `principal + 1 wei`. The comment says it plainly — *"no interest cumulation because of the rounding - we add 1 wei as symbolic cumulated interest to avoid interest free loans."* Without it, dust-sized borrows would be free.

### `increaseTotalBorrowsStableAndUpdateAverageRate(ReserveData storage, uint256 _amount, uint256 _rate) internal` — `:303`

Weighted average update:
`avg' = (amount·rate + prevTotal·avg) / (prevTotal + amount)`.

### `decreaseTotalBorrowsStableAndUpdateAverageRate(ReserveData storage, uint256 _amount, uint256 _rate) internal` — `:331`

- **Checks.** `require(_reserve.totalBorrowsStable >= _amount, "Invalid amount to decrease")`, then `require(weightedPreviousTotalBorrows >= weightedLastBorrow, "The amounts to subtract don't match")`.
- **Edge case.** If `totalBorrowsStable` reaches zero the average rate is zeroed and the function returns early.
- **Gotcha.** The second `require` is a real failure mode: accumulated rounding can make the removed weight exceed the stored aggregate. v2 replaced the revert with a silent clamp to zero (`StableDebtToken.sol:220-222`).

### `increaseTotalBorrowsVariable` — `:370` / `decreaseTotalBorrowsVariable` — `:379`

Plain add/subtract. The decrease reverts with *"The amount that is being
subtracted from the variable total borrows is incorrect"*.

### `calculateLinearInterest(uint256 _rate, uint40 _lastUpdateTimestamp) internal view returns (uint256)` — `:394`

`1 + rate × Δt / SECONDS_PER_YEAR`, all in ray. `SECONDS_PER_YEAR = 365 days`
(`:17`).

### `calculateCompoundedInterest(uint256 _rate, uint40 _lastUpdateTimestamp) internal view returns (uint256)` — `:413`

```solidity
uint256 ratePerSecond = _rate.div(SECONDS_PER_YEAR);
return ratePerSecond.add(WadRayMath.ray()).rayPow(timeDifference);
```

**True exponentiation** via `rayPow` — an O(log n) square-and-multiply
(`WadRayMath.sol:72`). This is exact but expensive, and it is exactly what v2
replaced with a three-term binomial approximation to save gas
([2.3](#23-math-libraries)).

### `getTotalBorrows(ReserveData storage) internal view returns (uint256)` — `:431`

`totalBorrowsStable + totalBorrowsVariable`.

## 1.3 `WadRayMath` (v1)

`aave/v1-aave-protocol/contracts/libraries/WadRayMath.sol` — 85 lines.

| Function | Line | Behaviour |
|---|---:|---|
| `ray()` | `:22` | `1e27` |
| `wad()` | `:25` | `1e18` |
| `halfRay()` | `:29` | `0.5e27` |
| `halfWad()` | `:33` | `0.5e18` |
| `wadMul(a,b)` | `:37` | `(a·b + halfWAD) / WAD` — rounds half up |
| `wadDiv(a,b)` | `:41` | `(a·WAD + b/2) / b` |
| `rayMul(a,b)` | `:47` | `(a·b + halfRAY) / RAY` |
| `rayDiv(a,b)` | `:51` | `(a·RAY + b/2) / b` |
| `rayToWad(a)` | `:57` | `(a + 0.5e9) / 1e9` |
| `wadToRay(a)` | `:63` | `a · 1e9` |
| `rayPow(x,n)` | `:72` | Square-and-multiply exponentiation in ray |

Constants at `:14-20`: `WAD = 1e18`, `RAY = 1e27`, `WAD_RAY_RATIO = 1e9`.
Everything rounds half-up, so v1 has no systematic protocol-favouring rounding
direction — a design v3 later reversed deliberately with its `TokenMath` helpers.

---

## 1.4 `LendingPool`

`aave/v1-aave-protocol/contracts/lendingpool/LendingPool.sol` — 1007 lines.
`contract LendingPool is ReentrancyGuard, VersionedInitializable` (`:27`).

### State, constants, modifiers

| Item | Line | Notes |
|---|---:|---|
| `addressesProvider` | `:32` | `public` |
| `core` | `:33` | `public LendingPoolCore` |
| `dataProvider` | `:34` | `public LendingPoolDataProvider` |
| `parametersProvider` | `:35` | `public LendingPoolParametersProvider` |
| `feeProvider` | `:36` | internal `IFeeProvider` |
| `UINT_MAX_VALUE` | `:269` | `uint256(-1)`, the "repay everything" sentinel |
| `LENDINGPOOL_REVISION` | `:271` | `0x3` |
| `onlyOverlyingAToken(address)` | `:232` | `msg.sender` must be the reserve's aToken. Revert: *"The caller of this function can only be the aToken contract of this reserve"* |
| `onlyActiveReserve(address)` | `:244` | delegates to `requireReserveActiveInternal` (`:990`) |
| `onlyUnfreezedReserve(address)` | `:254` | delegates to `requireReserveNotFreezedInternal` (`:997`) |
| `onlyAmountGreaterThanZero(uint256)` | `:264` | delegates to `requireAmountGreaterThanZeroInternal` (`:1004`) |

The three modifiers delegate to internal functions purely to keep bytecode below
the 24 KB limit — a modifier body is inlined at every use site, an internal
function call is not.

### `getRevision() internal pure returns (uint256)` — `:273`
Returns `LENDINGPOOL_REVISION` (`0x3`), consumed by `VersionedInitializable`.

### `initialize(LendingPoolAddressesProvider _addressesProvider) public initializer` — `:282`
- **State writes.** Caches `core`, `dataProvider`, `parametersProvider`, `feeProvider` from the provider.
- **Access.** `initializer` — once per revision.
- **Gotcha.** These are cached, not read live. Changing the Core address in the provider requires re-initialising the pool at a new revision.

### `deposit(address _reserve, uint256 _amount, uint16 _referralCode) external payable` — `:299`
Modifiers: `nonReentrant`, `onlyActiveReserve`, `onlyUnfreezedReserve`, `onlyAmountGreaterThanZero`.

1. Resolve `aToken = core.getReserveATokenAddress(_reserve)`.
2. `isFirstDeposit = aToken.balanceOf(msg.sender) == 0`.
3. `core.updateStateOnDeposit(...)` — accrues indexes, refreshes rates, and if first deposit flips the collateral flag on.
4. `aToken.mintOnDeposit(msg.sender, _amount)`.
5. `core.transferToReserve.value(msg.value)(_reserve, msg.sender, _amount)` — pulls the underlying (or forwards ETH).
6. `emit Deposit(_reserve, msg.sender, _amount, _referralCode, block.timestamp)` (`:46`).

- **Gotcha.** Minting happens *before* the transfer in. Safe only because `mintOnDeposit` is `onlyLendingPool` and the transfer reverts on failure, unwinding everything.
- **Gotcha.** Deposits are always credited to `msg.sender`; there is no `onBehalfOf`. v2 added one.

### `redeemUnderlying(address _reserve, address payable _user, uint256 _amount, uint256 _aTokenBalanceAfterRedeem) external` — `:331`
Modifiers: `nonReentrant`, `onlyOverlyingAToken`, `onlyActiveReserve`, `onlyAmountGreaterThanZero`.

- **Access.** **Only the aToken may call this.** Users call `AToken.redeem` (`:218`), which burns and then calls back here.
- **Checks.** `require(currentAvailableLiquidity >= _amount, "There is not enough liquidity available to redeem")`.
- **State.** `core.updateStateOnRedeem(...)`; clears the collateral flag when `_aTokenBalanceAfterRedeem == 0`.
- **Transfer.** `core.transferToUser`.
- **Event.** `RedeemUnderlying` (`:61`).

### `borrow(address _reserve, uint256 _amount, uint256 _interestRateMode, uint16 _referralCode) external` — `:388`
Modifiers: `nonReentrant`, `onlyActiveReserve`, `onlyUnfreezedReserve`, `onlyAmountGreaterThanZero`. Uses `BorrowLocalVars` (`:362`) to dodge stack-too-deep.

Checks in order:

| # | Check | Revert string | Line |
|---|---|---|---:|
| 1 | `core.isReserveBorrowingEnabled` | `"Reserve is not enabled for borrowing"` | `:404` |
| 2 | mode is `STABLE` or `VARIABLE` | `"Invalid interest rate mode selected"` | `:409` |
| 3 | `availableLiquidity >= _amount` | `"There is not enough liquidity available in the reserve"` | `:420` |
| 4 | `userCollateralBalanceETH > 0` | `"The collateral balance is 0"` | `:434` |
| 5 | `!healthFactorBelowThreshold` | `"The borrower can already be liquidated so he cannot borrow more"` | `:438` |
| 6 | `borrowFee > 0` | `"The amount to borrow is too small"` | `:444` |
| 7 | `amountOfCollateralNeededETH <= userCollateralBalanceETH` | `"There is not enough collateral to cover a new borrow"` | `:457` |
| 8 | stable only: `core.isUserAllowedToBorrowAtStable` | `"User cannot borrow the selected amount with a stable rate"` | `:473` |
| 9 | stable only: `_amount <= maxLoanSizeStable` | `"User is trying to borrow too much liquidity at a stable rate"` | `:483` |

Then `core.updateStateOnBorrow(...)` returns `(finalUserBorrowRate,
borrowBalanceIncrease)`, `core.transferToUser` pays out, and `Borrow` (`:80`) is
emitted with nine fields.

- **Gotcha.** Check 6 means a borrow so small that `amount × 0.0025` rounds to zero is rejected outright.
- **Gotcha.** Check 9 reads `parametersProvider.getMaxStableRateBorrowSizePercent()` and applies it to *available* liquidity, not total.

### `repay(address _reserve, uint256 _amount, address payable _onBehalfOf) external payable` — `:533`
Modifiers: `nonReentrant`, `onlyActiveReserve`, `onlyAmountGreaterThanZero`. Uses `RepayLocalVars` (`:522`).

1. Read `(principal, compounded, balanceIncrease)` and `originationFee`.
2. `require(compoundedBorrowBalance > 0, "The user does not have any borrow pending")`.
3. `require(_amount != UINT_MAX_VALUE || msg.sender == _onBehalfOf, "To repay on behalf of an user an explicit amount to repay is needed.")`.
4. `paybackAmount = compounded + originationFee`, capped by `_amount`.
5. `require(!isETH || msg.value >= paybackAmount, "Invalid msg.value sent for the repayment")`.
6. **Fee-only branch** (`:572`): when `paybackAmount <= originationFee`, the whole payment goes to the `TokenDistributor` via `transferToFeeCollectionAddress`, the `Repay` event reports `0` principal, and the function **returns early**.
7. Otherwise `updateStateOnRepay`, then the fee transfer (if any), then `transferToReserve` for the principal.
8. `emit Repay(...)` (`:102`).

- **Gotcha.** Fees are paid **before** principal. A partial repayment always clears the origination fee first.
- **Gotcha.** In the ETH path, `msg.value.sub(vars.originationFee)` is forwarded and `transferToReserve` refunds the excess (`LendingPoolCore.sol:486`).

### `swapBorrowRateMode(address _reserve) external` — `:648`
Modifiers: `nonReentrant`, `onlyActiveReserve`, `onlyUnfreezedReserve`.

- **Checks.** `require(compoundedBorrowBalance > 0, "User does not have a borrow in progress on this reserve")`; when switching variable→stable, `require(core.isUserAllowedToBorrowAtStable(...), "User cannot borrow the selected amount at stable")`.
- **Gotcha.** The stable check applies **only** in the variable→stable direction. Going stable→variable is unconditional.
- **Event.** `Swap` (`:121`).

### `rebalanceStableBorrowRate(address _reserve, address _user) external` — `:709`
Modifiers: `nonReentrant`, `onlyActiveReserve`. **Permissionless** — anyone may rebalance anyone.

- **Checks.** Non-zero balance; `require(core.getUserCurrentBorrowRateMode(...) == STABLE, "The user borrow is variable and cannot be rebalanced")`.
- **Trigger condition** (`:742-745`): rebalance if `userRate < liquidityRate` (the user could earn more by re-depositing than they pay — a self-arbitrage) **or** `userRate > reserveStableRate × (1 + rebalanceDownRateDelta)` (the user is overpaying).
- **Fallthrough.** `revert("Interest rate rebalance conditions were not met")` (`:764`).

### `setUserUseReserveAsCollateral(address _reserve, bool _useAsCollateral) external` — `:772`
- **Checks.** `require(underlyingBalance > 0, "User does not have any liquidity deposited")`; `require(dataProvider.balanceDecreaseAllowed(...), "User deposit is already being used as collateral")`.
- **Gotcha.** The second revert string is misleading — the real meaning is *"disabling this collateral would make you liquidatable"*. Note it is evaluated even when enabling.
- **Events.** `ReserveUsedAsCollateralEnabled` (`:135`) / `Disabled` (`:142`).

### `liquidationCall(address _collateral, address _reserve, address _user, uint256 _purchaseAmount, bool _receiveAToken) external payable` — `:805`
Modifiers: `nonReentrant`, `onlyActiveReserve(_reserve)`, `onlyActiveReserve(_collateral)`.

- **Mechanism.** `delegatecall` into the liquidation manager, `require(success, "Liquidation call failed")`, then decode `(uint256 returnCode, string returnMessage)` and, if non-zero, `revert("Liquidation failed: " + returnMessage)`.
- **Gotcha.** The manager returns error *codes* rather than reverting, so the pool can prefix the message. Every failure therefore costs a full execution.

### `flashLoan(address _receiver, address _reserve, uint256 _amount, bytes memory _params) public` — `:843`
Modifiers: `nonReentrant`, `onlyActiveReserve`, `onlyAmountGreaterThanZero`.

1. Snapshot `availableLiquidityBefore` by reading the Core's balance directly (the comment at `:850` says this avoids a Core call to save gas).
2. `require(availableLiquidityBefore >= _amount, "There is not enough liquidity available to borrow")`.
3. Fees from `parametersProvider.getFlashLoanFeesInBips()`: `amountFee = amount × totalFeeBips / 10000`, `protocolFee = amountFee × protocolFeeBips / 10000`.
4. `require(amountFee > 0 && protocolFee > 0, "The requested amount is too small for a flashLoan.")`.
5. `core.transferToUser` → `receiver.executeOperation(_reserve, _amount, amountFee, _params)`.
6. `require(availableLiquidityAfter == availableLiquidityBefore.add(amountFee), "The actual balance of the protocol is inconsistent")` — note **equality**, not `>=`.
7. `core.updateStateOnFlashLoan(...)` splits `amountFee - protocolFee` to suppliers and `protocolFee` to the distributor.
8. `emit FlashLoan` (`:169`).

- **Gotcha.** v1 flash loans are **single-asset** and cannot end as debt. v2 added both ([2.10](#210-lendingpool)).
- **Gotcha.** The strict equality in step 6 means over-repaying reverts.

### View passthroughs

| Function | Line | Returns |
|---|---:|---|
| `getReserveConfigurationData` | `:908` | LTV, threshold, bonus, strategy, flags |
| `getReserveData` | `:925` | Liquidity, borrows, rates, indexes, aToken, timestamp |
| `getUserAccountData` | `:947` | Aggregated collateral/debt/fees/LTV/threshold/HF |
| `getUserReserveData` | `:964` | Per-reserve user position |
| `getReserves` | `:983` | The reserve list |

### Internal helpers

- `requireReserveActiveInternal` — `:990` — *"Action requires an active reserve"*
- `requireReserveNotFreezedInternal` — `:997` — *"Action requires an unfreezed reserve"*
- `requireAmountGreaterThanZeroInternal` — `:1004` — *"Amount must be greater than 0"*

## 1.5 `LendingPoolCore`

`aave/v1-aave-protocol/contracts/lendingpool/LendingPoolCore.sol` — 1775 lines,
the largest contract in either version. `contract LendingPoolCore is
VersionedInitializable` (`:26`).

### State

| Item | Line | Notes |
|---|---:|---|
| `lendingPoolAddress` | `:52` | `public address` — cached from the provider |
| `addressesProvider` | — | inherited slot, set in `initialize` |
| `reserves` | `:75` | `mapping(address => CoreLibrary.ReserveData) internal` |
| `usersReserveData` | `:76` | `mapping(address => mapping(address => CoreLibrary.UserReserveData)) internal` — **user first, then reserve** |
| `reservesList` | `:78` | `address[] public` |
| `CORE_REVISION` | `:80` | `0x6` |

Note the index order of `usersReserveData`: `usersReserveData[_user][_reserve]`.
Getting it backwards is the classic v1 integration bug.

### Modifiers

- `onlyLendingPool` — `:59` — *"The caller must be a lending pool contract"*
- `onlyLendingPoolConfigurator` — `:67` — *"The caller must be a lending pool configurator contract"*

### `initialize(LendingPoolAddressesProvider) public initializer` — `:94`
Stores the provider and calls `refreshConfigInternal()` (`:1759`), which caches
`lendingPoolAddress`.

### `function() external payable` — `:385`
```solidity
require(msg.sender.isContract(), "Only contracts can send ether to the Lending pool core");
```
- **Purpose.** Accept ETH repayments and flash-loan returns while rejecting accidental EOA sends.
- **Gotcha.** `isContract` is code-size based, so a contract in its constructor cannot repay ETH here.

### State mutators — all `onlyLendingPool`

Every one follows the same shape: accrue indexes → mutate → refresh rates and
timestamp.

#### `updateStateOnDeposit(address _reserve, address _user, uint256 _amount, bool _isFirstDeposit) external` — `:107`
`updateCumulativeIndexes()` → `updateReserveInterestRatesAndTimestampInternal(_reserve, _amount, 0)` → if first deposit, `setUserUseReserveAsCollateral(..., true)`.

#### `updateStateOnRedeem(address _reserve, address _user, uint256 _amountRedeemed, bool _userRedeemedEverything) external` — `:129`
Mirror image; clears the collateral flag on a full redeem.

#### `updateStateOnFlashLoan(address _reserve, uint256 _availableLiquidityBefore, uint256 _income, uint256 _protocolFee) external` — `:150`
1. `transferFlashLoanProtocolFeeInternal` (`:1744`) sends the protocol cut to the `TokenDistributor`.
2. `updateCumulativeIndexes()`.
3. `cumulateToLiquidityIndex(totalLiquidityBefore, _income)` — the supplier cut, distributed by bumping the index.
4. `updateReserveInterestRatesAndTimestampInternal(_reserve, _income, 0)`.

#### `updateStateOnBorrow(...) external returns (uint256, uint256)` — `:181`
Reads the user's balances, calls `updateReserveStateOnBorrowInternal` (`:1281`)
and `updateUserStateOnBorrowInternal` (`:1314`), refreshes rates, and returns
`(getUserCurrentBorrowRate(...), balanceIncrease)`.

`updateUserStateOnBorrowInternal` is where the mode discriminator is written:

```solidity
// LendingPoolCore.sol:1327-1339
if (_rateMode == CoreLibrary.InterestRateMode.STABLE) {
    user.stableBorrowRate = reserve.currentStableBorrowRate;
    user.lastVariableBorrowCumulativeIndex = 0;
} else if (_rateMode == CoreLibrary.InterestRateMode.VARIABLE) {
    user.stableBorrowRate = 0;
    user.lastVariableBorrowCumulativeIndex = reserve.lastVariableBorrowCumulativeIndex;
} else {
    revert("Invalid borrow rate mode");
}
user.principalBorrowBalance = user.principalBorrowBalance.add(_amountBorrowed).add(_balanceIncrease);
user.originationFee = user.originationFee.add(_fee);
```

The accrued interest is **capitalised into the principal**. A v1 user therefore
has exactly one debt position per reserve, and switching modes rewrites it.

#### `updateStateOnRepay(...) external` — `:227`
`updateReserveStateOnRepayInternal` (`:1357`) + `updateUserStateOnRepayInternal` (`:1396`) + rate refresh with `_paybackAmountMinusFees` as liquidity added.

#### `updateStateOnSwapRate(...) external returns (InterestRateMode, uint256)` — `:262`
`updateReserveStateOnSwapRateInternal` (`:1434`) moves the principal between the
stable and variable totals; `updateUserStateOnSwapRateInternal` (`:1478`) flips
the user's discriminator and returns the new mode.

#### `updateStateOnLiquidation(...) external` — `:302`
Nine parameters. Calls `updatePrincipalReserveStateOnLiquidationInternal`
(`:1517`), `updateCollateralReserveStateOnLiquidationInternal` (`:1561`),
`updateUserStateOnLiquidationInternal` (`:1577`), then refreshes rates on the
principal reserve and — **only when `!_liquidatorReceivesAToken`** — on the
collateral reserve too, since aTokens changing hands does not move underlying.

#### `updateStateOnRebalance(address _reserve, address _user, uint256 _balanceIncrease) external returns (uint256)` — `:351`
Returns the user's new `stableBorrowRate`.

#### `setUserUseReserveAsCollateral(address _reserve, address _user, bool _useAsCollateral) public onlyLendingPool` — `:370`
The only `public` mutator, because `updateStateOnDeposit` and
`updateStateOnRedeem` call it internally.

### Value movement — all `onlyLendingPool`

| Function | Line | Behaviour |
|---|---:|---|
| `transferToUser(address,address payable,uint256)` | `:397` | `safeTransfer` for ERC20; for ETH a `call.value(...).gas(50000)` with `require(result, "Transfer of ETH failed")` |
| `transferToFeeCollectionAddress(address,address,uint256,address) payable` | `:418` | Pushes fees to the distributor. ERC20 path requires `msg.value == 0`; ETH path requires `msg.value >= _amount` |
| `liquidateFee(address,uint256,address) payable` | `:446` | Same, but from the Core's own balance. Requires `msg.value == 0` — *"Fee liquidation does not require any transfer of value"* |
| `transferToReserve(address,address payable,uint256) payable` | `:473` | Pulls funds in. ERC20 requires `msg.value == 0`; ETH requires `msg.value >= _amount` and **refunds the excess** |

**The 50,000 gas stipend** on every ETH send is the notable constraint: a
contract with an expensive `receive()` cannot be a v1 ETH recipient.

### Read surface

`getUserBasicReserveData` `:505` · `isUserAllowedToBorrowAtStable` `:535` ·
`getUserUnderlyingAssetBalance` `:558` ·
`getReserveInterestRateStrategyAddress` `:573` · `getReserveATokenAddress`
`:584` · `getReserveAvailableLiquidity` `:594` · `getReserveTotalLiquidity`
`:610` · `getReserveNormalizedIncome` `:621` · `getReserveTotalBorrows` `:631` ·
`getReserveTotalBorrowsStable` `:640` · `getReserveTotalBorrowsVariable` `:651` ·
`getReserveLiquidationThreshold` `:662` · `getReserveLiquidationBonus` `:673` ·
`getReserveCurrentVariableBorrowRate` `:684` ·
`getReserveCurrentStableBorrowRate` `:701` ·
`getReserveCurrentAverageStableBorrowRate` `:719` ·
`getReserveCurrentLiquidityRate` `:733` · `getReserveLiquidityCumulativeIndex`
`:743` · `getReserveVariableBorrowsCumulativeIndex` `:753` ·
`getReserveConfiguration` `:772` · `getReserveDecimals` `:796` ·
`isReserveBorrowingEnabled` `:806` · `isReserveUsageAsCollateralEnabled` `:817` ·
`getReserveIsStableBorrowRateEnabled` `:827` · `getReserveIsActive` `:837` ·
`getReserveIsFreezed` `:848` · `getReserveLastUpdate` `:859` ·
`getReserveUtilizationRate` `:870` · `getReserves` `:887` ·
`isUserUseReserveAsCollateralEnabled` `:896` · `getUserOriginationFee` `:910` ·
`getUserCurrentBorrowRateMode` `:926` · `getUserCurrentBorrowRate` `:949` ·
`getUserCurrentStableBorrowRate` `:972` · `getUserBorrowBalances` `:988` ·
`getUserVariableBorrowCumulativeIndex` `:1013` · `getUserLastUpdate` `:1029`.

Two worth calling out:

- **`getReserveCurrentStableBorrowRate` (`:701`)** falls back to the
  `LendingRateOracle` market rate when the stored rate is zero, so an unused
  reserve still quotes a sane stable rate.
- **`getUserBorrowBalances` (`:988`)** returns
  `(principal, compounded, compounded - principal)`. That third value,
  `balanceIncrease`, is threaded through every mutator as the interest to
  capitalise.

### Admin surface — all `onlyLendingPoolConfigurator`

`refreshConfiguration` `:1041` · `initReserve` `:1052` ·
`removeLastAddedReserve` `:1069` · `setReserveInterestRateStrategyAddress`
`:1100` · `enableBorrowingOnReserve` `:1113` · `disableBorrowingOnReserve`
`:1125` · `enableReserveAsCollateral` `:1133` · `disableReserveAsCollateral`
`:1150` · `enableReserveStableBorrowRate` `:1158` ·
`disableReserveStableBorrowRate` `:1167` · `activateReserve` `:1176` ·
`deactivateReserve` `:1191` · `freezeReserve` `:1201` · `unfreezeReserve`
`:1210` · `setReserveBaseLTVasCollateral` `:1220` ·
`setReserveLiquidationThreshold` `:1233` · `setReserveLiquidationBonus` `:1246` ·
`setReserveDecimals` `:1259`.

`deactivateReserve` (`:1191`) requires the reserve's total liquidity to be zero —
a reserve with users cannot be switched off, only frozen.

### `updateReserveInterestRatesAndTimestampInternal(address _reserve, uint256 _liquidityAdded, uint256 _liquidityTaken) internal` — `:1703`

Calls the strategy with `(availableLiquidity + added − taken, totalBorrowsStable,
totalBorrowsVariable, currentAverageStableBorrowRate)`, stores the three returned
rates, writes `lastUpdateTimestamp`, and emits `ReserveUpdated` (`:43`). Every
mutator ends here — this is the single point at which a v1 reserve's clock
advances.

---

## 1.6 `LendingPoolDataProvider`

`aave/v1-aave-protocol/contracts/lendingpool/LendingPoolDataProvider.sol` — 475
lines. `contract LendingPoolDataProvider is VersionedInitializable` (`:21`).
Pure reads; it never writes state.

| Item | Line |
|---|---:|
| `HEALTH_FACTOR_LIQUIDATION_THRESHOLD = 1e18` | `:32` |
| `DATA_PROVIDER_REVISION = 0x1` | `:34` |
| `getRevision()` | `:36` |
| `initialize(LendingPoolAddressesProvider)` | `:40` |
| `struct UserGlobalDataLocalVars` | `:48` |
| `struct balanceDecreaseAllowedLocalVars` | `:158` |

### `calculateUserGlobalData(address _user) public view returns (uint256 totalLiquidityBalanceETH, uint256 totalCollateralBalanceETH, uint256 totalBorrowBalanceETH, uint256 totalFeesETH, uint256 currentLtv, uint256 currentLiquidationThreshold, uint256 healthFactor, bool healthFactorBelowThreshold)` — `:70`

The v1 health-factor engine. It loops over **every** listed reserve:

```solidity
// LendingPoolDataProvider.sol:86-101 (excerpt)
address[] memory reserves = core.getReserves();
for (uint256 i = 0; i < reserves.length; i++) {
    vars.currentReserve = reserves[i];
    ( vars.compoundedLiquidityBalance, vars.compoundedBorrowBalance,
      vars.originationFee, vars.userUsesReserveAsCollateral
    ) = core.getUserBasicReserveData(vars.currentReserve, _user);
    if (vars.compoundedLiquidityBalance == 0 && vars.compoundedBorrowBalance == 0) continue;
```

- Collateral counts only when `usageAsCollateralEnabled && userUsesReserveAsCollateral`.
- `currentLtv` and `currentLiquidationThreshold` are accumulated as
  ETH-value-weighted sums and divided by total collateral at the end, producing
  weighted averages.
- **Origination fees are part of the debt** for health purposes — the v1-only
  `totalFeesETH` term.
- **Gotcha.** The loop is unbounded over the reserve list and makes an oracle
  call per reserve. This is exactly why v2 introduced the `UserConfigurationMap`
  bitmap so it could skip untouched reserves in O(1) per reserve.

### `balanceDecreaseAllowed(address _reserve, address _user, uint256 _amount) external view returns (bool)` — `:180`

Answers *"may this user lose `_amount` of this collateral?"*. Returns `true`
immediately when the reserve is not collateral-enabled or the user is not using
it. Otherwise it recomputes the post-decrease collateral and liquidation
threshold and returns whether the resulting health factor stays above `1e18`.

### `calculateCollateralNeededInETH(address _reserve, uint256 _amount, uint256 _fee, uint256 _userCurrentBorrowBalanceTH, uint256 _userCurrentFeesETH, uint256 _userCurrentLtv) external view returns (uint256)` — `:258`

```
collateralNeeded = (existingDebtETH + existingFeesETH + (amount + fee)·price/1eDecimals) × 100 / ltv
```

The `× 100 / ltv` is because v1 LTV is whole percent. Note the parameter name
typo `_userCurrentBorrowBalanceTH` — preserved here so a `grep` finds it.

### `calculateAvailableBorrowsETHInternal(uint256 collateralBalanceETH, uint256 borrowBalanceETH, uint256 totalFeesETH, uint256 ltv) internal view returns (uint256)` — `:296`

`collateral × ltv / 100`, minus existing debt and fees, minus the origination fee
on the remainder. Returns `0` when already at the limit.

### `calculateHealthFactorFromBalancesInternal(uint256 collateralBalanceETH, uint256 borrowBalanceETH, uint256 totalFeesETH, uint256 liquidationThreshold) internal pure returns (uint256)` — `:322`

```solidity
if (borrowBalanceETH == 0) return uint256(-1);
return (collateralBalanceETH.mul(liquidationThreshold).div(100))
         .wadDiv(borrowBalanceETH.add(totalFeesETH));
```

No debt means `type(uint256).max`. Fees sit in the denominator.

### Remaining views

`getHealthFactorLiquidationThreshold()` `:339` · `getReserveConfigurationData`
`:346` · `getReserveData` `:371` · `getUserAccountData` `:405` ·
`getUserReserveData` `:438`.

## 1.7 `LendingPoolLiquidationManager`

`aave/v1-aave-protocol/contracts/lendingpool/LendingPoolLiquidationManager.sol`
— 355 lines. Executed **only** via `delegatecall` from `LendingPool`
([1.1](#11-architecture-and-the-delegatecall-storage-contract)).

| Item | Line | Notes |
|---|---:|---|
| `LIQUIDATION_CLOSE_FACTOR_PERCENT = 50` | `:35` | Half the debt per call, hardcoded |
| `enum LiquidationErrors` | `:79` | `{NO_ERROR, NO_COLLATERAL_AVAILABLE, COLLATERAL_CANNOT_BE_LIQUIDATED, CURRRENCY_NOT_BORROWED, HEALTH_FACTOR_ABOVE_THRESHOLD, NOT_ENOUGH_LIQUIDITY}` — note the triple-R typo, carried into v2 |
| `struct LiquidationCallLocalVars` | `:88` | |
| `struct AvailableCollateralToLiquidateLocalVars` | `:295` | |
| `getRevision()` | `:111` | returns `0` |

### `liquidationCall(address _collateral, address _reserve, address _user, uint256 _purchaseAmount, bool _receiveAToken) external payable returns (uint256, string memory)` — `:124`

Returns `(code, message)` rather than reverting, so `LendingPool` can prefix the
message.

| Guard | Returned error | Line |
|---|---|---:|
| Health factor not below threshold | `HEALTH_FACTOR_ABOVE_THRESHOLD` | `:136` |
| User holds none of this collateral | `NO_COLLATERAL_AVAILABLE` | `:145` |
| Collateral not enabled (globally or by the user) | `COLLATERAL_CANNOT_BE_LIQUIDATED` | `:157` |
| User has no debt in this currency | `CURRRENCY_NOT_BORROWED` | `:170` |
| `!_receiveAToken` and reserve lacks the collateral | `NOT_ENOUGH_LIQUIDITY` | `:219` |

Then:

1. `maxPrincipalAmountToLiquidate = compoundedBorrow × 50 / 100`; the actual amount is `min(_purchaseAmount, max)`.
2. `calculateAvailableCollateralToLiquidate` for the debt.
3. **If an origination fee is outstanding**, a *second* call computes the collateral needed to also seize the fee, out of whatever collateral remains.
4. If the collateral is insufficient, `actualAmountToLiquidate` is reduced to `principalAmountNeeded`.
5. `core.updateStateOnLiquidation(...)` with all nine arguments.
6. Either `collateralAtoken.transferOnLiquidation` (aToken path) or `burnOnLiquidation` + `core.transferToUser` (underlying path).
7. `core.transferToReserve.value(msg.value)` pulls the repayment from the liquidator.
8. Fee seizure, if any, burns more aTokens and calls `core.liquidateFee`, emitting `OriginationFeeLiquidated` (`:44` in `LendingPool.sol:194`).
9. `emit LiquidationCall(...)`.

- **Gotcha.** The fee seizure is a *second* collateral grab on top of the 50% close factor, so a liquidation can take more collateral than the headline bonus suggests. v2 dropped origination fees entirely and this whole branch with them.

### `calculateAvailableCollateralToLiquidate(address _collateral, address _principal, uint256 _purchaseAmount, uint256 _userCollateralBalance) internal view returns (uint256 collateralAmount, uint256 principalAmountNeeded)` — `:314`

```
maxAmountCollateralToLiquidate = principalPrice × purchaseAmount / collateralPrice × liquidationBonus / 100
```

If that exceeds the user's balance, the collateral is capped and the principal is
back-solved. Otherwise the requested amount stands.

- **Gotcha.** Token decimals are **not** normalised here — v1 prices are per whole token in ETH and the multiplication assumes matching scales. v2's `_calculateAvailableCollateralToLiquidate` fixed this by dividing by `10**decimals` explicitly.

## 1.8 `DefaultReserveInterestRateStrategy` (v1)

`aave/v1-aave-protocol/contracts/lendingpool/DefaultReserveInterestRateStrategy.sol`
— 199 lines.

| Constant | Line | Value |
|---|---:|---|
| `OPTIMAL_UTILIZATION_RATE` | `:28` | `0.8e27` |
| `EXCESS_UTILIZATION_RATE` | `:36` | `0.2e27` |

Immutables: `baseVariableBorrowRate`, `variableRateSlope1`, `variableRateSlope2`,
`stableRateSlope1`, `stableRateSlope2`, exposed by getters at `:79`, `:83`,
`:87`, `:91`, `:95`.

### `calculateInterestRates(address _reserve, uint256 _availableLiquidity, uint256 _totalBorrowsStable, uint256 _totalBorrowsVariable, uint256 _averageStableBorrowRate) external view returns (uint256 currentLiquidityRate, uint256 currentStableBorrowRate, uint256 currentVariableBorrowRate)` — `:108`

1. `U = totalBorrows / (availableLiquidity + totalBorrows)`, or `0` when both are zero.
2. Base stable rate comes from `ILendingRateOracle.getMarketBorrowRate(_reserve)` — an **off-chain anchor**, unique to v1/v2.
3. Above the kink: both rates add their full slope1 plus `slope2 × (U − U*)/(1 − U*)`. Below: `slope1 × U/U*`.
4. `currentLiquidityRate = overallBorrowRate × U`.

### `getOverallBorrowRateInternal(uint256 _totalBorrowsStable, uint256 _totalBorrowsVariable, uint256 _currentVariableBorrowRate, uint256 _currentAverageStableBorrowRate) internal pure returns (uint256)` — `:175`

```
overall = (variableTotal·variableRate + stableTotal·avgStableRate) / totalBorrows
```

Returns `0` when there is no debt.

- **The v1 gap.** `currentLiquidityRate = overallBorrowRate × U` with **no reserve factor**. v1 took no protocol cut of interest at all — its revenue was the origination fee and the flash-loan protocol fee. v2 multiplies by `(1 − reserveFactor)` ([2.12](#212-defaultreserveinterestratestrategy-v2)).

## 1.9 `AToken` (v1) — the rebasing token with interest redirection

`aave/v1-aave-protocol/contracts/tokenization/AToken.sol` — 674 lines.
`contract AToken is ERC20, ERC20Detailed` (`:18`).

### State

| Field | Line | Meaning |
|---|---:|---|
| `userIndexes` | `:125` | Each user's snapshot of the reserve's normalized income |
| `interestRedirectionAddresses` | `:126` | Who receives this user's interest |
| `redirectedBalances` | `:127` | Total balance redirected **to** this address |
| `interestRedirectionAllowances` | `:128` | Who may redirect on this user's behalf |

### Modifiers

- `onlyLendingPool` — `:135` — *"The caller of this function must be a lending pool"*
- `whenTransferAllowed(address _from, uint256 _amount)` — `:143` — requires `isTransferAllowed`

### `balanceOf(address _user) public view returns (uint256)` — `:338`

The most subtle function in v1:

```solidity
// AToken.sol:352-372
if (interestRedirectionAddresses[_user] == address(0)) {
    return calculateCumulatedBalanceInternal(
        _user, currentPrincipalBalance.add(redirectedBalance)
    ).sub(redirectedBalance);
} else {
    return currentPrincipalBalance.add(
        calculateCumulatedBalanceInternal(_user, redirectedBalance).sub(redirectedBalance)
    );
}
```

- **Not redirecting:** your principal *and* everything redirected to you accrue; the redirected principal is then subtracted so you keep only the interest on it.
- **Redirecting:** your own principal stops accruing for you; only the balance others redirected to you earns.

This is why `redirectedBalances` is a *balance*, not an interest figure — it is
the notional on which someone else's interest is computed.

### `calculateCumulatedBalanceInternal(address _user, uint256 _balance) internal view returns (uint256)` — `:522`

```solidity
return _balance.wadToRay()
    .rayMul(core.getReserveNormalizedIncome(underlyingAssetAddress))
    .rayDiv(userIndexes[_user])
    .rayToWad();
```

The scaled-balance idea in embryo — except the index snapshot lives in a side
mapping instead of being folded into the stored balance, which is precisely what
v2 changed.

### `cumulateBalanceInternal(address _user) internal returns (uint256, uint256, uint256, uint256)` — `:452`

Materialises accrued interest by **actually minting** it, then refreshes
`userIndexes[_user]`. Returns `(previousPrincipal, newPrincipal, balanceIncrease,
newIndex)`. Every mutating path calls this first.

### `updateRedirectedBalanceOfRedirectionAddressInternal(address _user, uint256 _balanceToAdd, uint256 _balanceToRemove) internal` — `:479`

Returns immediately if the user is not redirecting. Otherwise it compounds the
redirection target's balance, adjusts `redirectedBalances`, and — if the target
is *itself* redirecting — propagates the balance increase one more hop. Emits
`RedirectedBalanceUpdated` (`:110`).

- **Gotcha.** Only **one** hop is propagated. A three-deep redirection chain does not fully update in a single transaction.

### User-facing functions

| Function | Line | Behaviour |
|---|---:|---|
| `redirectInterestStream(address _to)` | `:179` | Redirect your own interest. Reverts *"Interest stream can only be redirected to a different address"* / *"Interest stream can only be redirected if there is a valid balance"* |
| `redirectInterestStreamOf(address _from, address _to)` | `:191` | Redirect someone else's, if allowed. Reverts *"Caller is not allowed to redirect the interest of the user"* |
| `allowInterestRedirectionTo(address _to)` | `:205` | Grant that permission. Emits `InterestRedirectionAllowanceChanged` (`:118`) |
| `redeem(uint256 _amount)` | `:218` | Burn and call `pool.redeemUnderlying`. `_amount == UINT_MAX_VALUE` redeems everything |
| `mintOnDeposit(address _account, uint256 _amount)` | `:271` | `onlyLendingPool` |
| `burnOnLiquidation(address _account, uint256 _value)` | `:297` | `onlyLendingPool` |
| `transferOnLiquidation(address _from, address _to, uint256 _value)` | `:325` | `onlyLendingPool`; bypasses `whenTransferAllowed` |
| `_transfer(address,address,uint256)` | `:167` | Overrides ERC20; gated by `whenTransferAllowed` |
| `principalBalanceOf(address)` | `:381` | `super.balanceOf` — the stored, non-accruing figure |
| `totalSupply()` | `:392` | Principal supply scaled by normalized income |
| `isTransferAllowed(address,uint256)` | `:413` | `pool.balanceDecreaseAllowed(...)` |
| `getUserIndex(address)` | `:422` | |
| `getInterestRedirectionAddress(address)` | `:432` | |
| `getRedirectedBalance(address)` | `:442` | |
| `executeTransferInternal(address,address,uint256)` | `:540` | Compounds both sides, moves redirected balances, transfers |
| `redirectInterestStreamInternal(address,address)` | `:598` | Shared body of both redirect entry points |
| `resetDataOnZeroBalanceInternal(address)` | `:657` | Clears index and redirection when a balance hits zero |

- **Gotcha.** `redeem` is the user entry point and it calls back into
  `LendingPool.redeemUnderlying`, which is `onlyOverlyingAToken`. The
  circular trust between pool and token is a v1 signature; v2 inverted it so the
  pool drives the token.

## 1.10 `LendingPoolConfigurator` (v1)

`aave/v1-aave-protocol/contracts/lendingpool/LendingPoolConfigurator.sol` — 449
lines. `CONFIGURATOR_REVISION = 0x3` (`:157`). Every mutator carries
`onlyLendingPoolManager` (`:149`) — *"The caller must be a lending pool
manager"*.

| Function | Line | Effect |
|---|---:|---|
| `initialize(LendingPoolAddressesProvider)` | `:163` | |
| `initReserve(...)` | `:173` | Deploys an `AToken` then calls `initReserveWithData` |
| `initReserveWithData(...)` | `:201` | Registers an existing aToken. Emits `ReserveInitialized` (`:26`) |
| `removeLastAddedReserve(address)` | `:235` | Emits `ReserveRemoved` (`:36`) |
| `enableBorrowingOnReserve(address,bool)` | `:246` | `BorrowingEnabledOnReserve` (`:45`) |
| `disableBorrowingOnReserve(address)` | `:259` | `BorrowingDisabledOnReserve` (`:51`) |
| `enableReserveAsCollateral(address,uint256,uint256,uint256)` | `:273` | `ReserveEnabledAsCollateral` (`:60`) |
| `disableReserveAsCollateral(address)` | `:298` | `ReserveDisabledAsCollateral` (`:71`) |
| `enableReserveStableBorrowRate(address)` | `:309` | `StableRateEnabledOnReserve` (`:77`) |
| `disableReserveStableBorrowRate(address)` | `:320` | `StableRateDisabledOnReserve` (`:83`) |
| `activateReserve(address)` | `:331` | `ReserveActivated` (`:89`) |
| `deactivateReserve(address)` | `:342` | `ReserveDeactivated` (`:95`) |
| `freezeReserve(address)` | `:354` | `ReserveFreezed` (`:101`) |
| `unfreezeReserve(address)` | `:365` | `ReserveUnfreezed` (`:107`) |
| `setReserveBaseLTVasCollateral(address,uint256)` | `:377` | `ReserveBaseLtvChanged` (`:114`) |
| `setReserveLiquidationThreshold(address,uint256)` | `:391` | `ReserveLiquidationThresholdChanged` (`:121`) |
| `setReserveLiquidationBonus(address,uint256)` | `:404` | `ReserveLiquidationBonusChanged` (`:128`) |
| `setReserveDecimals(address,uint256)` | `:420` | `ReserveDecimalsChanged` (`:135`) |
| `setReserveInterestRateStrategyAddress(address,address)` | `:433` | `ReserveInterestRateStrategyChanged` (`:143`) |
| `refreshLendingPoolCoreConfiguration()` | `:444` | Re-caches the pool address inside the Core |

**One admin key.** There is a single `LENDING_POOL_MANAGER` role and no
timelock, emergency admin or risk admin. v2 split off an emergency admin; v3
added a full `ACLManager`.

## 1.11 Configuration, fees, flashloan, misc, mocks

### `LendingPoolAddressesProvider` — `configuration/LendingPoolAddressesProvider.sol`, 238 lines

Fourteen `bytes32` ids at `:33-46`: `LENDING_POOL`, `LENDING_POOL_CORE`,
`LENDING_POOL_CONFIGURATOR`, `PARAMETERS_PROVIDER`, `LENDING_POOL_MANAGER`,
`LIQUIDATION_MANAGER`, `FLASHLOAN_PROVIDER`, `DATA_PROVIDER`,
`ETHEREUM_ADDRESS`, `PRICE_ORACLE`, `LENDING_RATE_ORACLE`, `FEE_PROVIDER`,
`WALLET_BALANCE_PROVIDER`, `TOKEN_DISTRIBUTOR`.

Getters and `onlyOwner` setters at `:53`–`:211`. The pattern splits in two:
`setXImpl` routes through `updateImplInternal` (`:222`), which deploys or
upgrades an `InitializableAdminUpgradeabilityProxy` and calls
`initialize(address)` on it; plain `setX` (price oracle, lending rate oracle,
token distributor, manager) writes the address directly with no proxy.

- **Gotcha.** `setLendingPoolLiquidationManager` (`:168`) is a direct set, not a
  proxy — correct, since the manager is only ever `delegatecall`ed.

### `LendingPoolParametersProvider` — `configuration/LendingPoolParametersProvider.sol`, 53 lines

Four hardcoded `private constant`s:

| Constant | Line | Value | Meaning |
|---|---:|---|---|
| `MAX_STABLE_RATE_BORROW_SIZE_PERCENT` | `:15` | `25` | A stable borrow may take at most 25% of available liquidity |
| `REBALANCE_DOWN_RATE_DELTA` | `:16` | `1e27/5` | The 20% band above the reserve stable rate that triggers a rebalance-down |
| `FLASHLOAN_FEE_TOTAL` | `:17` | `35` | 0.35% total flash-loan fee (bips) |
| `FLASHLOAN_FEE_PROTOCOL` | `:18` | `3000` | 30% of that fee goes to the protocol |

Exposed by `getMaxStableRateBorrowSizePercent` (`:35`),
`getRebalanceDownRateDelta` (`:43`), `getFlashLoanFeesInBips` (`:50`).

- **Gotcha.** All four are `constant` and every getter is `pure`. Changing any of
  them requires deploying a new implementation — there is no setter, despite the
  contract being proxied.

### `AddressStorage` (`:14`) / `UintStorage` (`:14`)

Minimal `bytes32 => address` and `bytes32 => uint256` stores with
`getAddress`/`_setAddress` and `getUint`/`_setUint`.

### `FeeProvider` — `fees/FeeProvider.sol`, 51 lines

`originationFeePercentage` is set in `initialize` (`:30`) to `0.0025 * 1e18`.
`calculateLoanOriginationFee(address _user, uint256 _amount)` (`:39`) returns
`_amount.wadMul(originationFeePercentage)`.
`getLoanOriginationFeePercentage()` at `:46`. `FEE_PROVIDER_REVISION = 0x1` (`:20`).

> **Documentation bug, preserved in the source.** The NatSpec at
> `aave/v1-aave-protocol/contracts/fees/FeeProvider.sol:30` says *"origination
> fee is set as default as 25 basis points of the loan amount (0.0025%)"*. 25
> basis points is **0.25%**, and `0.0025 * 1e18` is indeed 0.25%. The code is
> right; the parenthetical is wrong by a factor of 100.

- **Gotcha.** The `_user` parameter is unused — the NatSpec says it exists so a
  future version could give stakers a discount. It never shipped.

### `TokenDistributor` — `fees/TokenDistributor.sol`, 162 lines

Receives every protocol fee and splits it by configured percentages.

| Item | Line | Notes |
|---|---:|---|
| `IMPLEMENTATION_REVISION` | `:27` | `0x4` |
| `MAX_UINT` / `MAX_UINT_MINUS_ONE` | `:30`, `:33` | Sentinels |
| `MIN_CONVERSION_RATE` | `:36` | `1` |
| `KYBER_ETH_MOCK_ADDRESS` | `:39` | Kyber's native-ETH pseudo-address |
| `DISTRIBUTION_BASE` | `:45` | `10000` |
| `tokenToBurn` / `recipientBurn` | `:51`, `:54` | The LEND burn leg |
| `initialize(...)` | `:60` | |
| `distribute(IERC20[])` | `:75` | Distribute full balances |
| `distributeWithAmounts(IERC20[],uint256[])` | `:91` | Distribute specific amounts |
| `distributeWithPercentages(IERC20[],uint256[])` | `:100` | Distribute a percentage of each balance |
| `internalSetTokenDistribution(...)` | `:117` | Emits `DistributionUpdated` (`:24`) |
| `internalDistributeTokenWithAmount(...)` | `:127` | Emits `Distributed` (`:25`) per receiver |
| `getDistribution()` | `:152` | |

- **Gotcha.** Anyone may call `distribute` — it is permissionless because the
  receivers and percentages are fixed by the owner.

### `ChainlinkProxyPriceProvider` — `misc/ChainlinkProxyPriceProvider.sol`, 108 lines

`setAssetSources` (`:37`, `onlyOwner`) · `setFallbackOracle` (`:44`,
`onlyOwner`) · `internalSetAssetsSources` (`:51`, emits `AssetSourceUpdated`
`:18`) · `internalSetFallbackOracle` (`:61`, emits `FallbackOracleUpdated`
`:19`) · `getAssetPrice` (`:68`) · `getAssetsPrices` (`:89`) ·
`getSourceOfAsset` (`:100`) · `getFallbackOracle` (`:106`).

`getAssetPrice` returns `1 ether` for the ETH pseudo-address, otherwise
`latestAnswer()`; a zero or missing answer falls through to the fallback oracle.

- **Gotcha.** `latestAnswer()` carries **no staleness check** — no `updatedAt`,
  no heartbeat. Prices are denominated in ETH, not USD.

### `WalletBalanceProvider` — `misc/WalletBalanceProvider.sol`, 76 lines

`balanceOf` (`:29`), `batchBalanceOf` (`:44`), `getUserWalletBalances` (`:65`).
Front-end convenience only.

### `IERC20DetailedBytes` — `misc/IERC20DetailedBytes.sol`, 7 lines

Declares `symbol()` returning `bytes32`, for tokens like MKR that predate the
string convention.

### `flashloan/`

- `IFlashLoanReceiver.sol` (12) — `executeOperation(address _reserve, uint256 _amount, uint256 _fee, bytes calldata _params)`.
- `FlashLoanReceiverBase.sol` (51) — stores the addresses provider, exposes `transferFundsBackInternal` / `transferInternal` and `getBalanceInternal`, and handles the ETH branch.

### Upgradeability set (`libraries/openzeppelin-upgradeability/`)

`Proxy.sol` (71) provides the assembly `delegatecall` fallback.
`BaseUpgradeabilityProxy.sol` (64) stores the implementation at
`keccak256("org.zeppelinos.proxy.implementation")` and emits `Upgraded`.
`UpgradeabilityProxy.sol` (27) sets it in the constructor;
`InitializableUpgradeabilityProxy.sol` (28) sets it post-deployment.
`BaseAdminUpgradeabilityProxy.sol` (121) adds the admin slot and the `ifAdmin`
router, so admin calls never fall through to the implementation.
`AdminUpgradeabilityProxy.sol` (24) and
`InitializableAdminUpgradeabilityProxy.sol` (27) are the concrete forms; the
latter is what the addresses provider deploys.

`Initializable.sol` (62) gives a one-shot `initializer`.
**`VersionedInitializable.sol` (70)** is the important one: it stores
`lastInitializedRevision` and its `initializer` modifier (`:32`) requires
`isConstructor() || revision > lastInitializedRevision`. Each contract's
`getRevision()` (`:56`) returns a bumped constant on upgrade, so a fresh
implementation may run `initialize` exactly once. `isConstructor()` (`:61`)
checks `extcodesize(address()) == 0`.

## 1.12 v1 revert-string table

v1 has no error library — every failure is an inline string. Complete list, with
the file and line where it is raised.

| Message | Where |
|---|---|
| `"The caller must be a lending pool contract"` | `LendingPoolCore.sol:59` |
| `"The caller must be a lending pool configurator contract"` | `LendingPoolCore.sol:67` |
| `"Only contracts can send ether to the Lending pool core"` | `LendingPoolCore.sol:387` |
| `"Transfer of ETH failed"` | `LendingPoolCore.sol:405`, `:436`, `:461`, `:492` |
| `"User is sending ETH along with the ERC20 transfer. Check the value attribute of the transaction"` | `LendingPoolCore.sol:427` |
| `"The amount and the value sent to deposit do not match"` | `LendingPoolCore.sol:433`, `:481` |
| `"Fee liquidation does not require any transfer of value"` | `LendingPoolCore.sol:453` |
| `"User is sending ETH along with the ERC20 transfer."` | `LendingPoolCore.sol:478` |
| `"Invalid borrow rate mode"` | `LendingPoolCore.sol:1338` |
| `"The caller of this function can only be the aToken contract of this reserve"` | `LendingPool.sol:235` |
| `"Action requires an active reserve"` | `LendingPool.sol:991` |
| `"Action requires an unfreezed reserve"` | `LendingPool.sol:998` |
| `"Amount must be greater than 0"` | `LendingPool.sol:1005` |
| `"There is not enough liquidity available to redeem"` | `LendingPool.sol:346` |
| `"Reserve is not enabled for borrowing"` | `LendingPool.sol:404` |
| `"Invalid interest rate mode selected"` | `LendingPool.sol:409` |
| `"There is not enough liquidity available in the reserve"` | `LendingPool.sol:420` |
| `"The collateral balance is 0"` | `LendingPool.sol:434` |
| `"The borrower can already be liquidated so he cannot borrow more"` | `LendingPool.sol:438` |
| `"The amount to borrow is too small"` | `LendingPool.sol:444` |
| `"There is not enough collateral to cover a new borrow"` | `LendingPool.sol:457` |
| `"User cannot borrow the selected amount with a stable rate"` | `LendingPool.sol:473` |
| `"User is trying to borrow too much liquidity at a stable rate"` | `LendingPool.sol:483` |
| `"The user does not have any borrow pending"` | `LendingPool.sol:552` |
| `"To repay on behalf of an user an explicit amount to repay is needed."` | `LendingPool.sol:556` |
| `"Invalid msg.value sent for the repayment"` | `LendingPool.sol:568` |
| `"User does not have a borrow in progress on this reserve"` | `LendingPool.sol:659` |
| `"User cannot borrow the selected amount at stable"` | `LendingPool.sol:677` |
| `"User does not have any borrow for this reserve"` | `LendingPool.sol:720` |
| `"The user borrow is variable and cannot be rebalanced"` | `LendingPool.sol:725` |
| `"Interest rate rebalance conditions were not met"` | `LendingPool.sol:764` |
| `"User does not have any liquidity deposited"` | `LendingPool.sol:780` |
| `"User deposit is already being used as collateral"` | `LendingPool.sol:784` |
| `"Liquidation call failed"` | `LendingPool.sol:825` |
| `"Liquidation failed: " + message` | `LendingPool.sol:831` |
| `"There is not enough liquidity available to borrow"` | `LendingPool.sol:857` |
| `"The requested amount is too small for a flashLoan."` | `LendingPool.sol:869` |
| `"The actual balance of the protocol is inconsistent"` | `LendingPool.sol:890` |
| `"Reserve has already been initialized"` | `CoreLibrary.sol:170` |
| `"Reserve is already enabled"` | `CoreLibrary.sol:195` |
| `"Reserve is already enabled as collateral"` | `CoreLibrary.sol:223` |
| `"Invalid amount to decrease"` | `CoreLibrary.sol:336` |
| `"The amounts to subtract don't match"` | `CoreLibrary.sol:357` |
| `"The amount that is being subtracted from the variable total borrows is incorrect"` | `CoreLibrary.sol:381` |
| `"The caller of this function must be a lending pool"` | `AToken.sol:138` |
| `"Transfer cannot be allowed."` | `AToken.sol:145` |
| `"Interest stream can only be redirected to a different address"` | `AToken.sol:611` |
| `"Interest stream can only be redirected if there is a valid balance"` | `AToken.sol:617` |
| `"Caller is not allowed to redirect the interest of the user"` | `AToken.sol:196` |
| `"Amount to redeem needs to be > 0"` | `AToken.sol:228` |
| `"User cannot redeem more than the available balance"` | `AToken.sol:238` |
| `"Transfer cannot be allowed."` | `AToken.sol:241` |
| `"The caller must be a lending pool manager"` | `LendingPoolConfigurator.sol:152` |

Liquidation failures are **return codes**, not reverts — see the
`LiquidationErrors` enum at `LendingPoolLiquidationManager.sol:79` and the
message strings paired with them at `:137`, `:146`, `:158`, `:171`, `:221`.

## 1.13 v1 events reference

| Event | Declared | Emitted when |
|---|---|---|
| `Deposit(reserve, user, amount, referral, timestamp)` | `LendingPool.sol:46` | `deposit` succeeds |
| `RedeemUnderlying(reserve, user, amount, timestamp)` | `LendingPool.sol:61` | `redeemUnderlying` succeeds |
| `Borrow(reserve, user, amount, borrowRateMode, borrowRate, originationFee, borrowBalanceIncrease, referral, timestamp)` | `LendingPool.sol:80` | `borrow` succeeds |
| `Repay(reserve, user, repayer, amountMinusFees, fees, borrowBalanceIncrease, timestamp)` | `LendingPool.sol:102` | `repay`, both branches |
| `Swap(reserve, user, newRateMode, newRate, borrowBalanceIncrease, timestamp)` | `LendingPool.sol:121` | `swapBorrowRateMode` |
| `ReserveUsedAsCollateralEnabled(reserve, user)` | `LendingPool.sol:135` | Collateral flag on |
| `ReserveUsedAsCollateralDisabled(reserve, user)` | `LendingPool.sol:142` | Collateral flag off |
| `RebalanceStableBorrowRate(reserve, user, newStableRate, borrowBalanceIncrease, timestamp)` | `LendingPool.sol:152` | `rebalanceStableBorrowRate` |
| `FlashLoan(target, reserve, amount, totalFee, protocolFee, timestamp)` | `LendingPool.sol:169` | `flashLoan` |
| `OriginationFeeLiquidated(collateral, reserve, user, feeLiquidated, liquidatedCollateralForFee, timestamp)` | `LendingPool.sol:194` | Liquidation that also seizes fees |
| `LiquidationCall(collateral, reserve, user, purchaseAmount, liquidatedCollateralAmount, accruedBorrowInterest, liquidator, receiveAToken, timestamp)` | `LendingPool.sol:215` | Liquidation |
| `ReserveUpdated(reserve, liquidityRate, stableBorrowRate, variableBorrowRate, liquidityIndex, variableBorrowIndex)` | `LendingPoolCore.sol:43` | Every rate/timestamp refresh |
| `Redeem(from, value, fromBalanceIncrease, fromIndex)` | `AToken.sol:30` | `AToken.redeem` |
| `MintOnDeposit(from, value, fromBalanceIncrease, fromIndex)` | `AToken.sol:44` | `mintOnDeposit` |
| `BurnOnLiquidation(from, value, fromBalanceIncrease, fromIndex)` | `AToken.sol:59` | `burnOnLiquidation` |
| `BalanceTransfer(from, to, value, fromBalanceIncrease, toBalanceIncrease, fromIndex, toIndex)` | `AToken.sol:76` | aToken transfer |
| `InterestStreamRedirected(from, to, redirectedBalance, fromBalanceIncrease, fromIndex)` | `AToken.sol:94` | Redirection set or reset |
| `RedirectedBalanceUpdated(targetAddress, targetBalanceIncrease, targetIndex, redirectedBalanceAdded, redirectedBalanceRemoved)` | `AToken.sol:110` | Redirected notional changes |
| `InterestRedirectionAllowanceChanged(from, to)` | `AToken.sol:118` | `allowInterestRedirectionTo` |
| `DistributionUpdated(receivers, percentages)` | `TokenDistributor.sol:24` | Distribution reconfigured |
| `Distributed(receiver, percentage, amount)` | `TokenDistributor.sol:25` | Per-receiver payout |
| `AssetSourceUpdated(asset, source)` | `ChainlinkProxyPriceProvider.sol:18` | Oracle source set |
| `FallbackOracleUpdated(fallbackOracle)` | `ChainlinkProxyPriceProvider.sol:19` | Fallback set |

Plus the 19 configurator events listed in [1.10](#110-lendingpoolconfigurator-v1).

- **Indexer gotcha.** v1 events carry an explicit `timestamp` field. They mostly
  do **not** use `indexed`, so filtering by user requires scanning. v2 fixed both.

## 1.14 v1 storage layouts

### `LendingPool` (proxied)

| Slot | Source | Type | Name |
|---:|---|---|---|
| 0 | `ReentrancyGuard` | `uint256` | `_guardCounter` |
| 1 | `VersionedInitializable` | `uint256` | `lastInitializedRevision` |
| 2 | `LendingPool.sol:32` | `address` | `addressesProvider` |
| 3 | `LendingPool.sol:33` | `address` | `core` |
| 4 | `LendingPool.sol:34` | `address` | `dataProvider` |
| 5 | `LendingPool.sol:35` | `address` | `parametersProvider` |
| 6 | `LendingPool.sol:36` | `address` | `feeProvider` |

`LendingPoolLiquidationManager` reproduces slots 0–6 exactly and adds
`ethereumAddress` at slot 7 — the invariant that makes the `delegatecall` sound.

### `LendingPoolCore` (proxied)

| Slot | Source | Type | Name |
|---:|---|---|---|
| 0 | `VersionedInitializable` | `uint256` | `lastInitializedRevision` |
| 1 | `LendingPoolCore.sol:52` | `address` | `lendingPoolAddress` |
| 2 | inherited field | `address` | `addressesProvider` |
| 3 | `LendingPoolCore.sol:75` | mapping | `reserves` |
| 4 | `LendingPoolCore.sol:76` | mapping | `usersReserveData` |
| 5 | `LendingPoolCore.sol:78` | `address[]` | `reservesList` |

Each `ReserveData` occupies roughly 20 slots; each `UserReserveData` roughly 6.

### `AToken`

Slots 0–4 come from OpenZeppelin `ERC20` and `ERC20Detailed` (balances,
allowances, total supply, name, symbol, decimals), followed by
`addressesProvider`, `core`, `pool`, `dataProvider`, `underlyingAssetAddress`,
then the four redirection mappings at `AToken.sol:125-128`.

- **Gotcha.** v1 aTokens are **not** proxied — `AToken` has no
  `VersionedInitializable`. Upgrading one means deploying a new token and
  re-listing the reserve.

## 1.15 v1 ABI / selector tables

### `LendingPool`

| Signature | Mutability | Access |
|---|---|---|
| `initialize(address)` | non-payable | `initializer` |
| `deposit(address,uint256,uint16)` | payable | anyone |
| `redeemUnderlying(address,address,uint256,uint256)` | non-payable | aToken only |
| `borrow(address,uint256,uint256,uint16)` | non-payable | anyone |
| `repay(address,uint256,address)` | payable | anyone |
| `swapBorrowRateMode(address)` | non-payable | anyone |
| `rebalanceStableBorrowRate(address,address)` | non-payable | anyone |
| `setUserUseReserveAsCollateral(address,bool)` | non-payable | anyone |
| `liquidationCall(address,address,address,uint256,bool)` | payable | anyone |
| `flashLoan(address,address,uint256,bytes)` | non-payable | anyone |
| `getReserveConfigurationData(address)` | view | anyone |
| `getReserveData(address)` | view | anyone |
| `getUserAccountData(address)` | view | anyone |
| `getUserReserveData(address,address)` | view | anyone |
| `getReserves()` | view | anyone |

### `AToken`

`redeem(uint256)` · `redirectInterestStream(address)` ·
`redirectInterestStreamOf(address,address)` ·
`allowInterestRedirectionTo(address)` · `mintOnDeposit(address,uint256)` * ·
`burnOnLiquidation(address,uint256)` * ·
`transferOnLiquidation(address,address,uint256)` * · `balanceOf(address)` ·
`principalBalanceOf(address)` · `totalSupply()` ·
`isTransferAllowed(address,uint256)` · `getUserIndex(address)` ·
`getInterestRedirectionAddress(address)` · `getRedirectedBalance(address)` ·
plus the full ERC20 surface. Entries marked * are `onlyLendingPool`.

### `LendingPoolCore`

All 8 `updateStateOn*` mutators and `setUserUseReserveAsCollateral` are
`onlyLendingPool`. The 4 `transferTo*`/`liquidateFee` functions are
`onlyLendingPool`. The 19 configuration setters are
`onlyLendingPoolConfigurator`. The ~35 getters listed in
[1.5](#15-lendingpoolcore) are open.

## 1.16 v1 use cases

| Goal | Call | Internal chain |
|---|---|---|
| Supply an ERC20 | `LendingPool.deposit(asset, amt, 0)` | `core.updateStateOnDeposit` → `aToken.mintOnDeposit` → `core.transferToReserve` |
| Supply ETH | same, `asset = 0xEeee…`, with `msg.value` | as above; `transferToReserve` refunds excess |
| Withdraw | `AToken.redeem(amt)` | `cumulateBalanceInternal` → `_burn` → `pool.redeemUnderlying` → `core.updateStateOnRedeem` → `core.transferToUser` |
| Withdraw everything | `AToken.redeem(uint256(-1))` | as above with the full balance |
| Borrow variable | `LendingPool.borrow(asset, amt, 2, 0)` | 9 checks → `core.updateStateOnBorrow` → `core.transferToUser` |
| Borrow stable | `LendingPool.borrow(asset, amt, 1, 0)` | as above, plus the two stable-rate limits |
| Repay | `LendingPool.repay(asset, amt, self)` | fee first, then principal |
| Repay everything | `repay(asset, uint256(-1), self)` | `paybackAmount = compounded + fee` |
| Repay for someone else | `repay(asset, explicitAmt, other)` | the `UINT_MAX_VALUE` guard forbids max-repay on behalf |
| Switch rate mode | `swapBorrowRateMode(asset)` | `core.updateStateOnSwapRate` |
| Rebalance someone's stable rate | `rebalanceStableBorrowRate(asset, user)` | two trigger conditions or revert |
| Toggle collateral | `setUserUseReserveAsCollateral(asset, bool)` | `dataProvider.balanceDecreaseAllowed` gate |
| Liquidate, take underlying | `liquidationCall(coll, debt, user, amt, false)` | delegatecall → `burnOnLiquidation` + `transferToUser` |
| Liquidate, take aTokens | `liquidationCall(coll, debt, user, amt, true)` | delegatecall → `transferOnLiquidation` |
| Flash loan | `flashLoan(receiver, asset, amt, params)` | `transferToUser` → `executeOperation` → strict equality balance check |
| Redirect your interest | `AToken.redirectInterestStream(to)` | `redirectInterestStreamInternal` |
| Let someone redirect for you | `AToken.allowInterestRedirectionTo(who)` | then they call `redirectInterestStreamOf` |
| List a reserve | `LendingPoolConfigurator.initReserve(...)` | deploys the aToken, then `core.initReserve` |
| Freeze a reserve | `LendingPoolConfigurator.freezeReserve(asset)` | deposits and new borrows blocked; repay and redeem still work |

---
