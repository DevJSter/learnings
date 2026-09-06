# Aave v3 — Complete Reference: everything outside `protocol/`

This document covers **every Solidity file** in `aave/aave-v3-origin/src/` *except*
`src/contracts/protocol/**` and `src/contracts/interfaces/**`, which are covered by the
sibling document [`V3-PROTOCOL-COMPLETE-REFERENCE.md`](V3-PROTOCOL-COMPLETE-REFERENCE.md).
For the conceptual walkthrough (why lending works this way, index math derivations,
worked liquidation examples) read [`AAVE-DEEP-DIVE.md`](AAVE-DEEP-DIVE.md) first.

That is **159 files / 13,438 lines**: the rewards system, the interest rate strategy, the
oracle, the ERC-4626 aToken wrapper, the governance config engine, every data provider,
the treasury, the token instances, the proxy machinery, the flash-loan base contracts, the
vendored dependencies, the mocks, and the whole deployment pipeline.

Every `file:line` below was checked with `grep -n` against these exact files. Every function
selector was computed with `cast sig`. Paths are relative to `aave/aave-v3-origin/`.

---

## ⚠️ Version note: this tree is v3.7 code, not v3.6

`package.json` says `"version": "3.6.0"` and `CHANGELOG.md` stops at `## 3.6.0`, but the
**code in this tree is v3.7**. The changeset for 3.7 has not been released yet, so the
version bump has not happened. Proof, all from this tree:

| Evidence | Location |
|---|---|
| `POOL_REVISION = 11` (3.7 bumps 10 → 11) | `src/contracts/instances/PoolInstance.sol:15` |
| `CONFIGURATOR_REVISION = 8` (3.7 bumps 7 → 8) | `src/contracts/instances/PoolConfiguratorInstance.sol:12` |
| Config engine libraries called **internally**, not by `delegatecall` | `AaveV3ConfigEngine.sol:77` calls `ListingEngine.executeCustomAssetListing(...)` directly |
| `EModeCategoryCreation.isolated` / `EModeCategoryUpdate.isolated` exist | `IAaveV3ConfigEngine.sol:200`, `:220` |
| `setAssetLtvzeroInEMode` wired in the engine | `EModeEngine.sol:109` |
| `AaveV3MiscProcedure` deploys no `PriceOracleSentinel` | `AaveV3MiscProcedure.sol:9-15` |
| `AaveV3LibrariesBatch1` deploys `BorrowLogic`, not `ConfiguratorLogic` | `AaveV3LibrariesBatch1.sol:16` |
| `docs/3.7/Aave-v3.7-changelog.md` exists and describes exactly this code | `docs/3.7/` |

The full 3.7 delta is in `docs/3.7/Aave-v3.7-changelog.md`; §19 of this document summarises
the periphery-facing half of it. Where a feature was added or removed in a specific release
I label it inline, e.g. **[3.7]** or **[removed in 3.7]**.

---

## Table of contents

- [0. File inventory — all 159 files](#0-file-inventory)
- [1. Rewards: `RewardsDistributor`, `RewardsController`, `EmissionManager`, transfer strategies](#1-rewards)
  - [1.1 The accounting model, and a proof of pro-rata correctness](#11-the-accounting-model)
  - [1.2 `RewardsDataTypes` — every struct, every field](#12-rewardsdatatypes)
  - [1.3 `RewardsDistributor` — every function](#13-rewardsdistributor)
  - [1.4 `RewardsController` — every function](#14-rewardscontroller)
  - [1.5 `EmissionManager` — every function](#15-emissionmanager)
  - [1.6 Transfer strategies — every function](#16-transfer-strategies)
  - [1.7 Rewards interfaces](#17-rewards-interfaces)
- [2. `DefaultReserveInterestRateStrategyV2`](#2-defaultreserveinterestratestrategyv2)
- [3. `AaveOracle`](#3-aaveoracle)
- [4. The stata-token stack (ERC-4626 aToken wrapper)](#4-the-stata-token-stack)
- [5. The v3 config engine](#5-the-v3-config-engine)
- [6. Helpers and data providers](#6-helpers-and-data-providers)
- [7. Treasury: `Collector`](#7-treasury-collector)
- [8. `instances/` — the deployable concrete contracts](#8-instances)
- [9. Upgradeability: proxies and `VersionedInitializable`](#9-upgradeability)
- [10. Flash loan base contracts](#10-flash-loan-base-contracts)
- [11. Vendored dependencies](#11-vendored-dependencies)
- [12. Mocks](#12-mocks)
- [13. Deployments — how a whole market is built](#13-deployments)
- [14. Selector / ABI tables](#14-selector--abi-tables)
- [15. Storage layouts](#15-storage-layouts)
- [16. Events reference](#16-events-reference)
- [17. Revert reason table](#17-revert-reason-table)
- [18. Use-case index](#18-use-case-index)
- [19. What changed per release (3.1 → 3.7), periphery only](#19-what-changed-per-release)

---

<a name="0-file-inventory"></a>

## 0. File inventory — all 159 files

Nothing in scope is omitted. Files marked **†** get a full function-by-function treatment
below; the rest get a description sized to what they actually do.

### `src/contracts/rewards/` — 14 files

| File | Lines | Purpose |
|---|---:|---|
| `rewards/RewardsDistributor.sol` **†** | 539 | Abstract multi-asset, multi-reward index accounting |
| `rewards/RewardsController.sol` **†** | 355 | Concrete controller: claims, transfer strategies, oracles |
| `rewards/EmissionManager.sol` **†** | 109 | Per-reward admin layer in front of the controller |
| `rewards/libraries/RewardsDataTypes.sol` **†** | 54 | The four structs the whole system is built on |
| `rewards/transfer-strategies/TransferStrategyBase.sol` **†** | 66 | Abstract strategy + emergency withdrawal |
| `rewards/transfer-strategies/PullRewardsTransferStrategy.sol` **†** | 49 | `transferFrom` out of a vault |
| `rewards/transfer-strategies/StakedTokenTransferStrategy.sol` **†** | 73 | Stakes into stkAAVE on the user's behalf |
| `rewards/interfaces/IRewardsController.sol` | 203 | Controller interface + 4 events |
| `rewards/interfaces/IRewardsDistributor.sol` | 177 | Distributor interface + `AssetConfigUpdated`, `Accrued` |
| `rewards/interfaces/IEmissionManager.sol` | 118 | Emission manager interface + `EmissionAdminUpdated` |
| `rewards/interfaces/ITransferStrategyBase.sol` | 41 | Strategy interface, `EmergencyWithdrawal`, 2 custom errors |
| `rewards/interfaces/IPullRewardsTransferStrategy.sol` | 15 | Adds `getRewardsVault()` |
| `rewards/interfaces/IStakedTokenTransferStrategy.sol` | 30 | Adds `renewApproval`/`dropApproval`/getters |
| `rewards/interfaces/IStakedToken.sol` | 14 | Minimal stkAAVE interface (`stake`, `STAKED_TOKEN`) |

### `src/contracts/misc/` — 10 files

| File | Lines | Purpose |
|---|---:|---|
| `misc/DefaultReserveInterestRateStrategyV2.sol` **†** | 234 | The kinked interest rate model |
| `misc/AaveOracle.sol` **†** | 147 | Chainlink aggregation with a fallback oracle |
| `misc/EmptyImplementation.sol` **†** | 8 | A contract with no code, for proxies deployed "off" |
| `misc/aave-upgradeability/VersionedInitializable.sol` **†** | 86 | The revision-gated `initializer` modifier |
| `misc/aave-upgradeability/BaseImmutableAdminUpgradeabilityProxy.sol` **†** | 85 | Admin in an immutable, not storage |
| `misc/aave-upgradeability/InitializableImmutableAdminUpgradeabilityProxy.sol` **†** | 29 | The above + `initialize` |
| `misc/flashloan/base/FlashLoanReceiverBase.sol` **†** | 21 | Base for multi-asset flash loan receivers |
| `misc/flashloan/base/FlashLoanSimpleReceiverBase.sol` **†** | 21 | Base for single-asset receivers |
| `misc/flashloan/interfaces/IFlashLoanReceiver.sol` **†** | 36 | `executeOperation(address[],uint256[],uint256[],address,bytes)` |
| `misc/flashloan/interfaces/IFlashLoanSimpleReceiver.sol` **†** | 36 | `executeOperation(address,uint256,uint256,address,bytes)` |

### `src/contracts/helpers/` — 13 files

| File | Lines | Purpose |
|---|---:|---|
| `helpers/LiquidationDataProvider.sol` **†** | 445 | Off-chain liquidation sizing, mirrors `LiquidationLogic` |
| `helpers/L2Encoder.sol` **†** | 296 | Packs L2Pool calldata into `bytes32` words |
| `helpers/UiIncentiveDataProviderV3.sol` **†** | 294 | Everything a UI needs about incentives |
| `helpers/AaveProtocolDataProvider.sol` **†** | 288 | The canonical read-only view of a market |
| `helpers/UiPoolDataProviderV3.sol` **†** | 264 | One call → the whole market page |
| `helpers/WrappedTokenGatewayV3.sol` **†** | 204 | Native ETH in/out of a WETH reserve |
| `helpers/WalletBalanceProvider.sol` **†** | 100 | Batch balance reads. Not used by the protocol |
| `helpers/interfaces/ILiquidationDataProvider.sol` | 122 | 6 structs + the provider's interface |
| `helpers/interfaces/IUiPoolDataProviderV3.sol` | 95 | `AggregatedReserveData`, `UserReserveData`, `BaseCurrencyInfo`, `Emode` |
| `helpers/interfaces/IUiIncentiveDataProviderV3.sol` | 73 | Incentive data structs |
| `helpers/interfaces/IWrappedTokenGatewayV3.sol` | 28 | Gateway interface |
| `helpers/interfaces/IWETH.sol` | 12 | `deposit`/`withdraw`/`approve` |
| `helpers/interfaces/IERC20DetailedBytes.sol` | 12 | For tokens whose `name`/`symbol` are `bytes32` (MKR) |

### `src/contracts/extensions/stata-token/` — 8 files

| File | Lines | Purpose |
|---|---:|---|
| `ERC4626StataTokenUpgradeable.sol` **†** | 311 | The 4626 vault half of the wrapper |
| `ERC20AaveLMUpgradeable.sol` **†** | 309 | The liquidity-mining passthrough half |
| `StataTokenV2.sol` **†** | 120 | Glues both halves + pausable + rescuable |
| `StataTokenFactory.sol` **†** | 90 | One stata token per underlying, deterministic |
| `interfaces/IERC20AaveLM.sol` | 107 | LM interface, 2 structs, 3 custom errors |
| `interfaces/IERC4626StataToken.sol` | 83 | 4626 extension interface, 3 custom errors |
| `interfaces/IStataTokenFactory.sol` | 55 | Factory interface + `StataTokenCreated` |
| `interfaces/IStataTokenV2.sol` | 22 | The union interface |
| `interfaces/IAToken.sol` | 18 | Local minimal aToken view (`POOL`, `UNDERLYING_ASSET_ADDRESS`, `scaledTotalSupply`) |

### `src/contracts/extensions/v3-config-engine/` — 11 files

| File | Lines | Purpose |
|---|---:|---|
| `IAaveV3ConfigEngine.sol` **†** | 337 | 15 structs — the entire declarative config language |
| `AaveV3Payload.sol` **†** | 183 | Template-method base for governance payloads |
| `AaveV3ConfigEngine.sol` **†** | 131 | Stateless facade over the seven engine libraries |
| `EngineFlags.sol` **†** | 34 | `KEEP_CURRENT` sentinels and bool↔flag conversion |
| `libraries/EModeEngine.sol` **†** | 203 | eMode create / update / per-asset flags |
| `libraries/ListingEngine.sol` **†** | 146 | Full asset listing, orchestrates the other engines |
| `libraries/RateEngine.sol` **†** | 95 | Rate strategy params with keep-current merging |
| `libraries/CollateralEngine.sol` **†** | 88 | LTV / LT / bonus / protocol fee |
| `libraries/BorrowEngine.sol` **†** | 56 | Borrowing enabled, reserve factor, flashloanable |
| `libraries/PriceFeedEngine.sol` **†** | 37 | Price feed validation and assignment |
| `libraries/CapsEngine.sol` **†** | 31 | Supply and borrow caps |

### `src/contracts/treasury/` — 2 files

| File | Lines | Purpose |
|---|---:|---|
| `treasury/Collector.sol` **†** | 339 | DAO treasury with Sablier-style linear streams |
| `treasury/ICollector.sol` | 216 | Interface, `Stream` struct, 12 custom errors, 3 events |

### `src/contracts/instances/` — 7 files

| File | Lines | Revision | Purpose |
|---|---:|---:|---|
| `instances/ATokenInstance.sol` **†** | 61 | 5 | Deployable `AToken` |
| `instances/ATokenWithDelegationInstance.sol` **†** | 61 | 5 | Deployable `ATokenWithDelegation` (AAVE token) |
| `instances/VariableDebtTokenInstance.sol` **†** | 50 | 5 | Deployable `VariableDebtToken` |
| `instances/VariableDebtTokenMainnetInstanceGHO.sol` **†** | 53 | 6 | GHO variant with a no-op legacy discount hook |
| `instances/PoolInstance.sol` **†** | 36 | 11 | Deployable `Pool` |
| `instances/L2PoolInstance.sol` **†** | 19 | 11 | `L2Pool` + `PoolInstance` |
| `instances/PoolConfiguratorInstance.sol` **†** | 23 | 8 | Deployable `PoolConfigurator` |

### `src/contracts/dependencies/` — 18 files

| File | Lines | Purpose |
|---|---:|---|
| `dependencies/weth/WETH9.sol` | 754 | The canonical WETH, vendored for tests/deploys |
| `dependencies/openzeppelin/contracts/Address.sol` | 220 | `isContract`, `functionCall`, `sendValue` |
| `dependencies/openzeppelin/contracts/AccessControl.sol` | 216 | Role-based access, used by `Collector` |
| `dependencies/openzeppelin/upgradeability/BaseAdminUpgradeabilityProxy.sol` | 125 | Storage-slot admin proxy |
| `dependencies/gnosis/contracts/GPv2SafeERC20.sol` **†** | 115 | Assembly `safeTransfer`/`safeTransferFrom` |
| `dependencies/openzeppelin/contracts/IAccessControl.sol` | 91 | AccessControl interface |
| `dependencies/openzeppelin/upgradeability/Proxy.sol` | 81 | The raw `delegatecall` fallback |
| `dependencies/openzeppelin/contracts/Ownable.sol` | 69 | Single-owner access |
| `dependencies/openzeppelin/contracts/Strings.sol` | 67 | `toString`, `toHexString` |
| `dependencies/openzeppelin/upgradeability/BaseUpgradeabilityProxy.sol` | 66 | `_implementation`/`_upgradeTo` + EIP-1967 slot |
| `dependencies/chainlink/AggregatorInterface.sol` **†** | 49 | `latestAnswer`, `decimals`, round data |
| `dependencies/openzeppelin/upgradeability/InitializableAdminUpgradeabilityProxy.sol` | 38 | Admin proxy + initializer |
| `dependencies/openzeppelin/upgradeability/AdminUpgradeabilityProxy.sol` | 36 | Constructor-set admin |
| `dependencies/openzeppelin/upgradeability/InitializableUpgradeabilityProxy.sol` | 29 | `initialize(logic, data)` |
| `dependencies/openzeppelin/contracts/ERC165.sol` | 28 | Interface detection |
| `dependencies/openzeppelin/upgradeability/UpgradeabilityProxy.sol` | 28 | Constructor-set implementation |
| `dependencies/openzeppelin/contracts/IERC165.sol` | 24 | `supportsInterface` |
| `dependencies/openzeppelin/contracts/Context.sol` | 23 | `_msgSender`, `_msgData` |

### `src/contracts/mocks/` — 22 files

| File | Lines | Purpose |
|---|---:|---|
| `mocks/helpers/MockReserveConfiguration.sol` | 133 | Exposes every `ReserveConfiguration` getter/setter for tests |
| `mocks/upgradeability/MockInitializableImplementation.sol` | 128 | V1/V2/V3 implementations to test revision gating |
| `mocks/testnet-helpers/TestnetERC20.sol` | 102 | Faucet-mintable ERC20 with permit |
| `mocks/tokens/MintableERC20.sol` | 96 | `mint()` open to anyone |
| `mocks/flashloan/MockFlashLoanReceiver.sol` | 78 | Multi-asset receiver, can be told to fail |
| `mocks/ATokenMock.sol` | 72 | Calls `handleAction` so reward events can be asserted |
| `mocks/flashloan/MockSimpleFlashLoanReceiver.sol` | 71 | Single-asset receiver, can be told to fail |
| `mocks/testnet-helpers/Faucet.sol` | 70 | Rate-limited testnet minting |
| `mocks/tokens/MockAToken.sol` | 55 | aToken with a bumped revision, for upgrade tests |
| `mocks/tests/FlashloanAttacker.sol` | 52 | Reentrancy probe against the flash loan path |
| `mocks/helpers/MockPeripheryContract.sol` | 42 | V1/V2 periphery for `PoolAddressesProvider` tests |
| `mocks/tokens/MockScaledToken.sol` | 37 | Bare `IScaledBalanceToken` for reward tests |
| `mocks/tokens/MintableDelegationERC20.sol` | 37 | Mintable + `delegate()` |
| `mocks/MockBadTransferStrategy.sol` | 33 | `performTransfer` returns `false` → `'TRANSFER_ERROR'` |
| `mocks/testnet-helpers/IFaucet.sol` | 32 | Faucet interface |
| `mocks/oracle/PriceOracle.sol` | 32 | Settable prices, used as the fallback oracle in tests |
| `mocks/helpers/MockPool.sol` | 28 | `MockPoolInherited`: settable `MAX_NUMBER_RESERVES` |
| `mocks/oracle/CLAggregators/MockAggregator.sol` | 25 | Fixed `latestAnswer` |
| `mocks/flashloan/MockFlashLoanReceiverWithoutMint.sol` | 23 | Receiver that cannot repay |
| `mocks/helpers/MockL2Pool.sol` | 23 | `L2PoolInstance` with a bumped revision |
| `mocks/flashloan/MockSimpleFlashLoanReceiverWithoutMint.sol` | 22 | Simple receiver that cannot repay |
| `mocks/WETH9Mock.sol` / `mocks/tokens/WETH9Mocked.sol` | 20 / 19 | WETH with a public `mint` |
| `mocks/tokens/MockDebtTokens.sol` | 16 | `MockVariableDebtToken` with a bumped revision |
| `mocks/helpers/MockIncentivesController.sol` | 8 | `handleAction` no-op |
| `mocks/upgradeability/MockAToken.sol` / `MockVariableDebtToken.sol` | 17 / 16 | Revision-bumped upgrade targets |
| `mocks/tests/MockReserveInterestRateStrategy.sol` | 26 | Returns hardcoded rates |

### `src/deployments/` — 34 files

| File | Lines | Purpose |
|---|---:|---|
| `projects/aave-v3-batched/AaveV3BatchOrchestration.sol` **†** | 317 | The full market deployment, top to bottom |
| `contracts/procedures/AaveV3SetupProcedure.sol` **†** | 209 | Provider, ACL, ownership handover |
| `contracts/utilities/MetadataReporter.sol` | 156 | Writes the deployment JSON report |
| `interfaces/IMarketReportTypes.sol` **†** | 152 | `MarketReport`, `Roles`, `MarketConfig`, and the per-stage reports |
| `contracts/utilities/FfiUtils.sol` | 97 | Reads pre-deployed library addresses from `.env` |
| `contracts/utilities/DeployUtils.sol` | 74 | Deploy from build artifacts, CREATE2 address math |
| `projects/aave-v3-batched/batches/AaveV3SetupBatch.sol` **†** | 68 | Holds the market report during deployment |
| `contracts/procedures/AaveV3TreasuryProcedure.sol` **†** | 62 | Collector proxy + dust bin |
| `contracts/utilities/Create2Utils.sol` **†** | 51 | Deterministic library deployment |
| `projects/aave-v3-batched/batches/AaveV3PeripheryBatch.sol` | 49 | Oracle + treasury + incentives |
| `contracts/utilities/MarketReportUtils.sol` | 45 | `MarketReport` → typed `ContractsReport` |
| `contracts/procedures/AaveV3HelpersProcedureTwo.sol` **†** | 42 | Stata token factory + impl |
| `contracts/procedures/AaveV3GettersProcedureTwo.sol` **†** | 41 | Gateway, L2Encoder, data provider |
| `contracts/procedures/AaveV3PoolProcedure.sol` **†** | 40 | Pool + configurator impls |
| `contracts/procedures/AaveV3L2PoolProcedure.sol` **†** | 40 | L2 variant |
| `contracts/procedures/AaveV3GettersProcedureOne.sol` **†** | 39 | Wallet balance + UI providers |
| `contracts/procedures/AaveV3HelpersProcedureOne.sol` **†** | 36 | Config engine |
| `projects/aave-v3-batched/batches/AaveV3HelpersBatchOne.sol` | 35 | Wraps helpers procedure one |
| `interfaces/IMetadataReporter.sol` | 16 | Report-writing interface |
| `contracts/procedures/AaveV3TokensProcedure.sol` **†** | 32 | aToken + vToken impls |
| `projects/aave-v3-libraries/AaveV3LibrariesBatch2.sol` **†** | 29 | FlashLoan/Liquidation/Pool/Supply logic |
| `projects/aave-v3-batched/batches/AaveV3GettersBatchTwo.sol` | 28 | Wraps getters procedure two |
| `contracts/procedures/AaveV3OracleProcedure.sol` **†** | 27 | `AaveOracle` |
| `projects/aave-v3-batched/batches/AaveV3GettersBatchOne.sol` | 22 | Wraps getters procedure one |
| `contracts/procedures/AaveV3MiscProcedure.sol` **†** | 20 | Default interest rate strategy |
| `projects/aave-v3-libraries/AaveV3LibrariesBatch1.sol` **†** | 19 | `BorrowLogic` |
| `projects/aave-v3-batched/batches/AaveV3PoolBatch.sol` | 19 | Wraps pool procedure |
| `inputs/MarketInput.sol` / `inputs/DefaultMarketInput.sol` **†** | 18 / 30 | The parameters of a market |
| `projects/aave-v3-batched/batches/AaveV3MiscBatch.sol` | 18 | Wraps misc procedure |
| `projects/aave-v3-batched/batches/AaveV3L2PoolBatch.sol` | 17 | Wraps L2 pool procedure |
| `projects/aave-v3-batched/batches/AaveV3HelpersBatchTwo.sol` | 17 | Wraps helpers procedure two |
| `projects/aave-v3-batched/batches/AaveV3TokensBatch.sol` | 16 | Wraps tokens procedure |
| `contracts/procedures/AaveV3IncentiveProcedure.sol` **†** | 14 | `EmissionManager` + `RewardsController` impl |
| `contracts/procedures/AaveV3PoolConfigProcedure.sol` **†** | 12 | `PoolConfiguratorInstance` |
| `contracts/procedures/AaveV3DefaultRateStrategyProcedure.sol` **†** | 11 | Rate strategy |
| `contracts/LibraryReportStorage.sol` / `contracts/MarketReportStorage.sol` | 12 / 8 | Storage for the two reports |
| `interfaces/IErrors.sol` | 11 | 6 deployment custom errors |
| `interfaces/ILibraryReportStorage.sol` / `IMarketReportStorage.sol` / `IPoolReport.sol` | 8 / 8 / 8 | Report getters |

---

<a name="1-rewards"></a>

## 1. Rewards: `RewardsDistributor`, `RewardsController`, `EmissionManager`, transfer strategies

Aave's liquidity mining pays a *stream* of one or more reward tokens to the holders of an
incentivized asset — an aToken or a variable debt token. It never iterates over holders.

The layering is three deep, and each layer has exactly one job:

```
   governance / risk council
            |
            v
  EmissionManager  (Ownable)                 rewards/EmissionManager.sol
    - one "emission admin" per REWARD token
    - the only address the controller will listen to
            |  calls
            v
  RewardsController  (upgradeable)           rewards/RewardsController.sol
    - claim entrypoints, claimer allowlist
    - transfer strategy + price oracle per reward
            |  inherits
            v
  RewardsDistributor (abstract)              rewards/RewardsDistributor.sol
    - ALL the index math. No token ever moves here.
            |
            +-- reads scaledBalanceOf / scaledTotalSupply from the aToken / vToken
            |
            v  (only at claim time)
  ITransferStrategyBase                      rewards/transfer-strategies/*
    - PullRewards:   transferFrom(vault -> user)
    - StakedToken:   STAKE_CONTRACT.stake(user, amount)
```

And the write path from the other direction — every balance change in a token calls in:

```
AToken.mint / burn / _transfer   (protocol/tokenization)
   |
   +-> IncentivizedERC20._transfer / ScaledBalanceTokenBase._mintScaled
         |
         v
     RewardsController.handleAction(user, scaledTotalSupply, scaledUserBalance)
         |                                       RewardsController.sol:109
         v
     RewardsDistributor._updateData(msg.sender=asset, user, userBalance, totalSupply)
         |                                       RewardsDistributor.sol:345
         +-- for each reward configured on this asset:
              _updateRewardData(...)   -> moves the ASSET index forward   :284
              _updateUserData(...)     -> banks the user's accrual        :315
              emit Accrued(...)                                           :380
```

Two things worth internalising before the function walk:

1. **`handleAction`'s `msg.sender` *is* the asset.** There is no `asset` parameter
   (`RewardsController.sol:109-111`). Any contract can call `handleAction` and it will
   update the distribution keyed on *itself*. That is safe only because a distribution
   for an unconfigured asset has `availableRewardsCount == 0` and `_updateData` returns
   immediately (`RewardsDistributor.sol:357-359`).
2. **Balances are always *scaled*.** The controller reads
   `getScaledUserBalanceAndSupply` (`RewardsController.sol:197-199`), not `balanceOf`. If
   it used the rebasing balance, a user's reward share would grow with the liquidity index
   even when they did nothing, and the sum of shares would not be 1.

<a name="11-the-accounting-model"></a>

### 1.1 The accounting model, and a proof of pro-rata correctness

The distribution keeps one number per `(asset, reward)`: an **index**, in units of
"reward tokens per one whole unit of asset, scaled by `assetUnit = 10**assetDecimals`".

`_getAssetIndex` (`RewardsDistributor.sol:489-517`) advances it:

```solidity
uint256 currentTimestamp = block.timestamp > distributionEnd ? distributionEnd : block.timestamp;
uint256 timeDelta = currentTimestamp - lastUpdateTimestamp;
uint256 firstTerm = emissionPerSecond * timeDelta * assetUnit;
assembly { firstTerm := div(firstTerm, totalSupply) }
return (oldIndex, (firstTerm + oldIndex));
```

so

```
                         emissionPerSecond * Δt * assetUnit
index(t2) = index(t1) + ------------------------------------
                                  totalSupply
```

and `_getRewards` (`:469-480`) converts an index *delta* back into tokens for one user:

```
                userBalance * (index_now - index_user)
rewards(user) = -------------------------------------
                             assetUnit
```

**Proof that a constant holder gets exactly their share.** Let a user hold a constant
scaled balance `b` while the scaled total supply is a constant `S`, for the whole emission
window `[t0, t1]`, with `e = emissionPerSecond`. Suppose `handleAction` fires `n` times at
`t0 = τ0 < τ1 < … < τn = t1`. Each firing adds `e·(τ_k − τ_{k−1})·u / S` to the index
(`u = assetUnit`), and each also runs `_updateUserData`, which banks
`b·(index_k − index_{k−1})/u` into `accrued` and sets `usersData[user].index = index_k`
(`:325-333`). Summing the banked amounts telescopes:

```
Σ_k  b·(index_k − index_{k−1})/u  =  b·(index_n − index_0)/u
                                  =  b·( Σ_k e·(τ_k − τ_{k−1})·u / S )/u
                                  =  (b/S) · e · (t1 − t0)
```

The number of firings drops out entirely, which is the whole point: the result is
`(b/S) × emissionPerSecond × duration` regardless of how often anyone touched the
contract. The only loss is integer truncation, twice per step, both flooring — so a user
can never extract more than their share, and the residue stays in the contract.

**Where the truncation lives, and why it is `assembly`.** Both divisions are done in raw
`assembly { div(...) }` (`:476-478` and `:513-515`) rather than Solidity `/`. In Solidity
0.8 the checked `/` inserts a division-by-zero guard; here both denominators are already
known non-zero (`totalSupply == 0` returns early at `:501`, and `assetUnit = 10**decimals`
is never 0), so the guard is dead weight. `div` by zero in EVM yields 0 rather than
reverting, which is why the early return matters.

**Precision trap.** `assetUnit` is `10**decimals` of the *asset*, not of the reward. For a
6-decimal asset like aUSDC, `firstTerm = e·Δt·1e6/S`. If `S` is large and `Δt` small,
`firstTerm` truncates to 0 and the index does not move at all — `_updateRewardData` then
takes the `else` branch (`:299-301`) and only writes `lastUpdateTimestamp`, silently
discarding that sliver of emission. High-decimal assets and infrequent updates make this
negligible; a 6-decimal asset with a huge supply and per-block `handleAction` calls is the
worst case.

**Overflow guard.** The index is stored in a `uint104` (`RewardsDataTypes.sol:33`).
`_updateRewardData` explicitly checks `newIndex <= type(uint104).max` and reverts
`'INDEX_OVERFLOW'` (`RewardsDistributor.sol:292`) before the downcast. `_updateUserData`
does *not* re-check, and says so in a comment (`:326-328`) — the user index can only ever
be assigned a value that already passed the asset-level check.

<a name="12-rewardsdatatypes"></a>

### 1.2 `RewardsDataTypes` — every struct, every field

`src/contracts/rewards/libraries/RewardsDataTypes.sol` (54 lines). A pure struct library;
no functions.

**`RewardsConfigInput`** (`:8-16`) — the input to `configureAssets`.

| Field | Type | Meaning |
|---|---|---|
| `emissionPerSecond` | `uint88` | Reward tokens emitted per second across *all* holders of `asset` |
| `totalSupply` | `uint256` | Ignored on input — `RewardsController.configureAssets` overwrites it with the live `scaledTotalSupply()` (`RewardsController.sol:81`) |
| `distributionEnd` | `uint32` | Unix time after which the index stops moving |
| `asset` | `address` | The incentivized aToken or variable debt token |
| `reward` | `address` | The ERC20 paid out |
| `transferStrategy` | `ITransferStrategyBase` | How to actually deliver `reward` |
| `rewardOracle` | `AggregatorInterface` | Price feed, **for UIs only**; never used in accounting |

**`UserAssetBalance`** (`:18-22`) — a transient `(asset, userBalance, totalSupply)` triple,
built by `_getUserAssetBalances` so the balances are read once and reused.

**`UserData`** (`:24-29`) — packed into one slot:

| Field | Type | Meaning |
|---|---|---|
| `index` | `uint104` | The asset index at the user's last interaction |
| `accrued` | `uint128` | Rewards already banked but not yet claimed |

**`RewardData`** (`:31-42`) — the per-`(asset, reward)` distribution. The first four fields
pack into one 256-bit slot (104 + 88 + 32 + 32 = 256 exactly):

| Field | Type | Bits | Meaning |
|---|---|---:|---|
| `index` | `uint104` | 104 | Cumulative rewards per `assetUnit` of asset |
| `emissionPerSecond` | `uint88` | 88 | Current emission rate |
| `lastUpdateTimestamp` | `uint32` | 32 | When the index last moved |
| `distributionEnd` | `uint32` | 32 | Emission cutoff |
| `usersData` | `mapping` | — | Second slot |

`lastUpdateTimestamp != 0` is the sentinel for "this distribution exists" — used at
`RewardsDistributor.sol:193` and `:238`.

**`AssetData`** (`:44-53`):

| Field | Type | Meaning |
|---|---|---|
| `rewards` | `mapping(address => RewardData)` | Per-reward distributions |
| `availableRewards` | `mapping(uint128 => address)` | Index → reward address, so `_updateData` can iterate |
| `availableRewardsCount` | `uint128` | Length of the above. **Rewards are never removed**, only zeroed |
| `decimals` | `uint8` | Cached `IERC20Metadata(asset).decimals()`; `!= 0` means "asset registered" |

<a name="13-rewardsdistributor"></a>

### 1.3 `RewardsDistributor` — every function

`src/contracts/rewards/RewardsDistributor.sol` (539 lines).
`abstract contract RewardsDistributor is IRewardsDistributor`.

**State** (`:19-33`):

| Slot | Declaration | Notes |
|---|---|---|
| — | `address public immutable EMISSION_MANAGER` (`:19`) | Bytecode, not storage |
| 0 | `address internal _emissionManager` (`:21`) | **Deprecated.** Kept only so the storage layout of already-deployed proxies does not shift |
| 1 | `mapping(address => AssetData) internal _assets` (`:24`) | |
| 2 | `mapping(address => bool) internal _isRewardEnabled` (`:27`) | |
| 3 | `address[] internal _rewardsList` (`:30`) | Global, across all assets |
| 4 | `address[] internal _assetsList` (`:33`) | |

**`modifier onlyEmissionManager()`** — `:35-38`. `require(msg.sender == EMISSION_MANAGER, 'ONLY_EMISSION_MANAGER')`.

**`constructor(address emissionManager)`** — `:40-42`. Sets the immutable. Note this is
the *manager contract*, not an EOA, and it can never be changed; replacing it means
deploying a new `RewardsController` implementation.

---

#### `getRewardsData(address asset, address reward) → (uint256,uint256,uint256,uint256)`
`external view` — `:45-55`

Returns `(index, emissionPerSecond, lastUpdateTimestamp, distributionEnd)` verbatim from
storage. **Stale by design**: it does not project the index forward to `block.timestamp`.
Use `getAssetIndex` for that.

#### `getAssetIndex(address asset, address reward) → (uint256 oldIndex, uint256 newIndex)`
`external view` — `:58-69`

Calls `_getAssetIndex` with the *live* `scaledTotalSupply()` and returns both the stored
index and what it would become right now. `ERC20AaveLMUpgradeable.getCurrentRewardsIndex`
(`extensions/stata-token/ERC20AaveLMUpgradeable.sol:119`) consumes the second value; that
is how the stata token stays in sync without writing.

- **External call:** `IScaledBalanceToken(asset).scaledTotalSupply()`.

#### `getDistributionEnd(address asset, address reward) → uint256`
`external view` — `:72-77`. Plain storage read.

#### `getRewardsByAsset(address asset) → address[]`
`external view` — `:80-88`

Materialises `availableRewards[0..availableRewardsCount)` into memory. `refreshRewardTokens`
in the stata token calls this (`ERC20AaveLMUpgradeable.sol:88`).

#### `getRewardsList() → address[]`
`external view` — `:91-93`. The global reward list. Append-only.

#### `getUserAssetIndex(address user, address asset, address reward) → uint256`
`external view` — `:96-102`.

#### `getUserAccruedRewards(address user, address reward) → uint256`
`external view` — `:105-115`

Sums `accrued` over **every asset ever registered** (`_assetsList`). Only counts *banked*
rewards, not pending ones. `O(_assetsList.length)` — this grows forever and will eventually
be too expensive for an on-chain caller.

#### `getUserRewards(address[] assets, address user, address reward) → uint256`
`external view` — `:118-124`. Banked + pending, for one reward, over the given assets.

#### `getAllUserRewards(address[] assets, address user) → (address[] rewardsList, uint256[] unclaimedAmounts)`
`external view` — `:127-159`

Banked + pending for **every** reward in `_rewardsList`, over the given assets. Note the
loop nesting: `rewardsList[r] = _rewardsList[r]` is re-assigned inside the inner loop on
every asset (`:146`) — harmless but wasteful. Skips `_getPendingRewards` when the user's
balance is 0 (`:152-154`), because a zero balance can accrue nothing.

#### `setDistributionEnd(address asset, address reward, uint32 newDistributionEnd)`
`external onlyEmissionManager` — `:162-179`

- **Writes:** `_assets[asset].rewards[reward].distributionEnd`.
- **Emits:** `AssetConfigUpdated` with old == new emission and the *stored* index.
- **Gotcha:** it does **not** call `_updateRewardData` first. Extending
  `distributionEnd` after it has already passed retroactively re-opens the window: the next
  `_getAssetIndex` computes `timeDelta = min(now, newEnd) − lastUpdateTimestamp`, and
  `lastUpdateTimestamp` is still back before the original end. Emissions for the dead
  period are paid. Shortening it is fine. The emission manager must call something that
  triggers an update first if that is not wanted.

#### `setEmissionPerSecond(address asset, address[] rewards, uint88[] newEmissionsPerSecond)`
`external onlyEmissionManager` — `:182-216`

- **Checks:** `rewards.length == newEmissionsPerSecond.length` → `'INVALID_INPUT'` (`:187`);
  per reward, `decimals != 0 && lastUpdateTimestamp != 0` → `'DISTRIBUTION_DOES_NOT_EXIST'` (`:192-195`).
- **Order matters:** `_updateRewardData` runs *before* the rate is changed (`:197-204`), so
  the old rate is settled up to now and the new rate applies only going forward. Contrast
  with `setDistributionEnd` above, which does not do this.
- **Emits:** `AssetConfigUpdated` per reward.

#### `_configureAssets(RewardsDataTypes.RewardsConfigInput[] rewardsInput)`
`internal` — `:222-274`

The registration path. Per entry:

1. If `_assets[asset].decimals == 0`, push `asset` onto `_assetsList` (`:224-227`).
2. Cache `decimals` from `IERC20Metadata(asset).decimals()` (`:229-231`). A revert here
   means the asset is not a valid ERC20-metadata token.
3. If `rewardConfig.lastUpdateTimestamp == 0`, append `reward` to this asset's
   `availableRewards` and bump `availableRewardsCount` (`:238-243`).
4. If `!_isRewardEnabled[reward]`, set it and push onto the global `_rewardsList` (`:246-249`).
5. `_updateRewardData` with the supplied `totalSupply` (`:252-256`) — settles the old rate.
6. Write the new `emissionPerSecond` and `distributionEnd` (`:261-262`).
7. `emit AssetConfigUpdated`.

**Gotcha:** step 3 makes rewards permanent per asset. Re-configuring the same
`(asset, reward)` pair does not duplicate it, but there is no removal path — a retired
reward stays in `availableRewards` forever and every future `_updateData` still loops over
it. That loop cost is paid by every mint, burn and transfer of the aToken.

#### `_updateRewardData(RewardData storage rewardData, uint256 totalSupply, uint256 assetUnit) → (uint256 newIndex, bool indexUpdated)`
`internal` — `:284-304`

- Computes `(oldIndex, newIndex)` via `_getAssetIndex`.
- If they differ: `require(newIndex <= type(uint104).max, 'INDEX_OVERFLOW')` (`:292`), then
  write `index` and `lastUpdateTimestamp` **adjacently** — the comment at `:295` notes this
  saves one `SSTORE` because both live in the same slot.
- If they are equal, only `lastUpdateTimestamp` is written (`:300`). This still costs an
  `SSTORE` and is what makes the truncation-to-zero case lossy.

#### `_updateUserData(RewardData storage rewardData, address user, uint256 userBalance, uint256 newAssetIndex, uint256 assetUnit) → (uint256 rewardsAccrued, bool dataUpdated)`
`internal` — `:315-336`

- `dataUpdated = (userIndex != newAssetIndex)`; assigned inside the `if` condition (`:325`).
- On update: write the user index, then, **only if `userBalance != 0`**, compute
  `_getRewards` and `+=` it into `accrued` as a `uint128` (`:329-333`).
- **Gotcha:** the user index is advanced even for a zero balance. That is correct and
  necessary — otherwise a user who exits and later re-enters would be credited for the
  whole gap.

#### `_updateData(address asset, address user, uint256 userBalance, uint256 totalSupply)`
`internal` — `:345-384`

Loops every reward on `asset`, calling `_updateRewardData` then `_updateUserData`, and
emits `Accrued` if either moved. Early-returns when `availableRewardsCount == 0` (`:357-359`)
— the guard that makes the permissionless `handleAction` harmless.

**Documented quirk:** the `Accrued` event takes `assetIndex` and `userIndex` as separate
fields, but this call site passes `newAssetIndex` for **both** (`:380`). The user's *old*
index is therefore not observable from the event. Indexers wanting the delta must track
state themselves.

The `unchecked` block (`:360-383`) covers only the loop counter and index math that was
already bounds-checked.

#### `_updateDataMultiple(address user, UserAssetBalance[] userAssetBalances)`
`internal` — `:391-403`. Fan-out over a pre-fetched balance list.

#### `_getUserReward(address user, address reward, UserAssetBalance[] userAssetBalances) → uint256 unclaimedRewards`
`internal view` — `:412-432`. Banked + pending, skipping the pending term on zero balances.

#### `_getPendingRewards(address user, address reward, UserAssetBalance userAssetBalance) → uint256`
`internal view` — `:441-459`. Projects the index forward with `_getAssetIndex` and diffs
against the user's stored index. Read-only twin of `_updateUserData`.

#### `_getRewards(uint256 userBalance, uint256 reserveIndex, uint256 userIndex, uint256 assetUnit) → uint256`
`internal pure` — `:469-480`

```solidity
uint256 result = userBalance * (reserveIndex - userIndex);
assembly { result := div(result, assetUnit) }
```

The multiplication is checked (0.8 semantics) so it cannot silently overflow; the division
is unchecked assembly.

#### `_getAssetIndex(RewardData storage rewardData, uint256 totalSupply, uint256 assetUnit) → (uint256 oldIndex, uint256 newIndex)`
`internal view` — `:489-517`

Four early-return conditions, all returning `(oldIndex, oldIndex)` (`:499-506`):
`emissionPerSecond == 0`, `totalSupply == 0`, `lastUpdateTimestamp == block.timestamp`,
or `lastUpdateTimestamp >= distributionEnd`. Then the clamped-`Δt` formula from §1.1.

#### `_getUserAssetBalances(address[] assets, address user) → UserAssetBalance[]`
`internal view virtual` — `:525-528`. Declared abstract here; implemented in
`RewardsController`.

#### `getAssetDecimals(address asset) → uint8` / `getEmissionManager() → address`
`external view` — `:531-533`, `:536-538`.

<a name="14-rewardscontroller"></a>

### 1.4 `RewardsController` — every function

`src/contracts/rewards/RewardsController.sol` (355 lines).
`contract RewardsController is RewardsDistributor, VersionedInitializable, IRewardsController`.

**State** (`:21-37`), continuing after the distributor's slots:

| Declaration | Purpose |
|---|---|
| `uint256 public constant REVISION = 1` (`:21`) | Bytecode constant |
| `mapping(address => address) internal _authorizedClaimers` (`:25`) | user → the one address allowed to claim for them |
| `mapping(address => ITransferStrategyBase) internal _transferStrategy` (`:30`) | reward → delivery mechanism |
| `mapping(address => AggregatorInterface) internal _rewardOracle` (`:37`) | reward → price feed, UI only |

**`modifier onlyAuthorizedClaimers(address claimer, address user)`** — `:39-42`.
`require(_authorizedClaimers[user] == claimer, 'CLAIMER_UNAUTHORIZED')`.

#### `constructor(address emissionManager)` — `:44`. Forwards to `RewardsDistributor`.

#### `initialize(address) external initializer` — `:50`

Takes an address and **ignores it**. The parameter exists purely because
`PoolAddressesProvider._updateImpl` always calls `initialize(address)` on the new
implementation (comment at `:47-49`). The body is empty; all real state comes from the
constructor immutable and later admin calls.

#### `getRevision() → uint256` — `internal pure override`, `:61-63`. Returns `REVISION` (1).

#### `getClaimer(address user) → address` — `:53-55`.
#### `getRewardOracle(address reward) → address` — `:66-68`.
#### `getTransferStrategy(address reward) → address` — `:71-73`.

#### `configureAssets(RewardsDataTypes.RewardsConfigInput[] config)`
`external onlyEmissionManager` — `:76-90`

Per entry, before delegating to `_configureAssets`:
1. Overwrite `config[i].totalSupply` with the live `scaledTotalSupply()` (`:81`) — the
   caller's value is discarded, so a stale or malicious supply cannot skew the index.
2. `_installTransferStrategy` (`:84`).
3. `_setRewardOracle` (`:87`).

#### `setTransferStrategy(address reward, ITransferStrategyBase transferStrategy)`
`external onlyEmissionManager` — `:93-98`.

#### `setRewardOracle(address reward, AggregatorInterface rewardOracle)`
`external onlyEmissionManager` — `:101-106`.

#### `handleAction(address user, uint256 totalSupply, uint256 userBalance)`
`external` — `:109-111`

**No access control.** `_updateData(msg.sender, user, userBalance, totalSupply)`. See the
note at the top of §1: safety comes from `availableRewardsCount == 0` short-circuiting.
An attacker calling this from their own contract updates a distribution that does not
exist, spending their own gas to do nothing.

#### `claimRewards(address[] assets, uint256 amount, address to, address reward) → uint256`
`external` — `:114-122`. `require(to != address(0), 'INVALID_TO_ADDRESS')`. Claims for
`msg.sender`, sends to `to`.

#### `claimRewardsOnBehalf(address[] assets, uint256 amount, address user, address to, address reward) → uint256`
`external onlyAuthorizedClaimers(msg.sender, user)` — `:125-135`. Two zero-address checks.

#### `claimRewardsToSelf(address[] assets, uint256 amount, address reward) → uint256`
`external` — `:138-144`. `claimer = user = to = msg.sender`. No zero check needed.

#### `claimAllRewards(address[] assets, address to) → (address[], uint256[])`
`external` — `:147-153`.

#### `claimAllRewardsOnBehalf(address[] assets, address user, address to) → (address[], uint256[])`
`external onlyAuthorizedClaimers` — `:156-169`.

#### `claimAllRewardsToSelf(address[] assets) → (address[], uint256[])`
`external` — `:172-176`.

#### `setClaimer(address user, address caller)`
`external onlyEmissionManager` — `:179-182`. Emits `ClaimerSet`. This is a *governance*
power: the DAO decides who may claim on a user's behalf. It cannot redirect the
destination — `claimRewardsOnBehalf` still takes `to` from the claimer, so an authorised
claimer *can* send a user's rewards anywhere. That is a real trust grant.

#### `_getUserAssetBalances(address[] assets, address user) → UserAssetBalance[]`
`internal view override` — `:190-202`

One `getScaledUserBalanceAndSupply(user)` call per asset, packing both values at once.

#### `_claimRewards(address[] assets, uint256 amount, address claimer, address user, address to, address reward) → uint256`
`internal` — `:214-250`

1. `amount == 0` → return 0 (`:222-224`).
2. `_updateDataMultiple` to bank everything pending (`:227`).
3. Walk `assets`, accumulating `accrued` into `totalRewards` and zeroing each as it goes.
   When the running total would exceed `amount`, keep the difference on the *last* asset
   and `break` (`:232-239`).
4. `totalRewards == 0` → return 0 (`:242-244`).
5. `_transferRewards(to, reward, totalRewards)` then `emit RewardsClaimed` (`:246-247`).

**Gotcha:** the partial-claim bookkeeping at `:235-237` leaves the remainder attributed to
whichever asset the loop stopped on, not spread proportionally. Total is conserved; the
per-asset split is not meaningful after a partial claim.

#### `_claimAllRewards(address[] assets, address claimer, address user, address to) → (address[], uint256[])`
`internal` — `:262-292`

Zeroes `accrued` for every `(asset, reward)` pair and sums per reward, then transfers once
per reward. **`_transferRewards` is called even when `claimedAmounts[i] == 0`** (`:288`) —
a zero-value `transferFrom`. Most ERC20s allow it; a token that reverts on zero-value
transfers would brick `claimAllRewards` for everyone the moment it is registered as a
reward. `RewardsClaimed` is likewise emitted with amount 0.

Cost is `O(assets.length × _rewardsList.length)`.

#### `_transferRewards(address to, address reward, uint256 amount)`
`internal` — `:300-306`

```solidity
bool success = transferStrategy.performTransfer(to, reward, amount);
require(success == true, 'TRANSFER_ERROR');
```

The doc comment above it says "using delegatecall" — that is **wrong**; it is a plain
external call. `MockBadTransferStrategy` exists to exercise the `false` path.

#### `_isContract(address account) → bool` — `internal view`, `:313-324`. `extcodesize > 0`.

#### `_installTransferStrategy(address reward, ITransferStrategyBase transferStrategy)`
`internal` — `:331-341`. `'STRATEGY_CAN_NOT_BE_ZERO'`, `'STRATEGY_MUST_BE_CONTRACT'`,
then store and `emit TransferStrategyInstalled`.

#### `_setRewardOracle(address reward, AggregatorInterface rewardOracle)`
`internal` — `:350-354`

`require(rewardOracle.latestAnswer() > 0, 'ORACLE_MUST_RETURN_PRICE')`. A liveness probe,
nothing more — the returned price is never used by the protocol. Emits `RewardOracleUpdated`.

<a name="15-emissionmanager"></a>

### 1.5 `EmissionManager` — every function

`src/contracts/rewards/EmissionManager.sol` (109 lines). `contract EmissionManager is Ownable, IEmissionManager`.

The point of this contract: the DAO owns it, but delegates *per-reward-token* control to
whoever funds that reward. An external protocol incentivizing its own token on Aave gets
`setEmissionAdmin(theirToken, theirMultisig)` and can then tune only their own emissions.

**State:** `mapping(address => address) internal _emissionAdmins` (`:18`),
`IRewardsController internal _rewardsController` (`:20`).

**`modifier onlyEmissionAdmin(address reward)`** — `:25-28`. `'ONLY_EMISSION_ADMIN'`.

| Function | Line | Access | Forwards to |
|---|---:|---|---|
| `constructor(address owner)` | 34-36 | — | `transferOwnership(owner)` |
| `configureAssets(RewardsConfigInput[])` | 39-44 | per-entry `_emissionAdmins[reward] == msg.sender` (`:41`) | `controller.configureAssets` |
| `setTransferStrategy(address,ITransferStrategyBase)` | 47-52 | `onlyEmissionAdmin` | `controller.setTransferStrategy` |
| `setRewardOracle(address,AggregatorInterface)` | 55-60 | `onlyEmissionAdmin` | `controller.setRewardOracle` |
| `setDistributionEnd(address,address,uint32)` | 63-69 | `onlyEmissionAdmin(reward)` | `controller.setDistributionEnd` |
| `setEmissionPerSecond(address,address[],uint88[])` | 72-81 | per-entry admin check (`:78`) | `controller.setEmissionPerSecond` |
| `setClaimer(address,address)` | 84-86 | `onlyOwner` | `controller.setClaimer` |
| `setEmissionAdmin(address,address)` | 89-93 | `onlyOwner` | emits `EmissionAdminUpdated` |
| `setRewardsController(address)` | 96-98 | `onlyOwner` | sets `_rewardsController` |
| `getRewardsController()` | 101-103 | view | |
| `getEmissionAdmin(address)` | 106-108 | view | |

**Gotcha:** `configureAssets` checks the admin of `config[i].reward`, but a listing also
names an `asset`. An emission admin for reward token R can therefore configure R against
**any** asset in the market, including one they have nothing to do with. The scoping is
per-reward, never per-asset.

**Bootstrap ordering.** `_rewardsController` starts as `address(0)` and is set by
`setRewardsController` *after* the controller proxy exists. The deployment does this in
`AaveV3SetupProcedure.sol:129-130`, immediately followed by transferring ownership of the
manager to the pool admin (`:131`).

<a name="16-transfer-strategies"></a>

### 1.6 Transfer strategies — every function

The controller never holds reward tokens. It delegates delivery to a strategy so that the
funding model is pluggable.

#### `TransferStrategyBase` — `rewards/transfer-strategies/TransferStrategyBase.sol` (66 lines)

`abstract contract TransferStrategyBase is ITransferStrategyBase`.

**State:** `address internal immutable INCENTIVES_CONTROLLER` (`:15`),
`address internal immutable REWARDS_ADMIN` (`:16`). Both immutable — a strategy is
disposable, not upgradeable.

- **`constructor(address incentivesController, address rewardsAdmin)`** — `:18-21`.
- **`modifier onlyIncentivesController()`** — `:26-29`. `require(..., CallerNotIncentivesController())`
  — a **custom error**, not a string, unlike everything else in the rewards package.
- **`modifier onlyRewardsAdmin()`** — `:34-37`. `OnlyRewardsAdmin()`.
- **`getIncentivesController() → address`** — `:40-42`.
- **`getRewardsAdmin() → address`** — `:45-47`.
- **`performTransfer(address,address,uint256) → bool`** — `:50-54`, `external virtual`, unimplemented.
- **`emergencyWithdrawal(address token, address to, uint256 amount)`** — `:57-65`,
  `onlyRewardsAdmin`. `GPv2SafeERC20.safeTransfer`, emits `EmergencyWithdrawal`. Lets the
  admin recover anything stuck in the strategy.

#### `PullRewardsTransferStrategy` — `PullRewardsTransferStrategy.sol` (49 lines)

**State:** `address internal immutable REWARDS_VAULT` (`:19`).

- **`constructor(address incentivesController, address rewardsAdmin, address rewardsVault)`** — `:21-27`.
- **`performTransfer(address to, address reward, uint256 amount) → bool`** — `:30-43`,
  `onlyIncentivesController`. Body is one line:
  `IERC20(reward).safeTransferFrom(REWARDS_VAULT, to, amount)` (`:40`), then `return true`.
  **Setup requirement:** the vault must `approve` *this strategy contract* (not the
  controller) for the reward token. Allowance running out makes every claim revert.
- **`getRewardsVault() → address`** — `:46-48`.

#### `StakedTokenTransferStrategy` — `StakedTokenTransferStrategy.sol` (73 lines)

Pays in stkAAVE: it holds AAVE and stakes it *for* the user, so the reward arrives already
staked and cooling-down.

**State:** `IStakedToken internal immutable STAKE_CONTRACT` (`:20`),
`address internal immutable UNDERLYING_TOKEN` (`:21`).

- **`constructor(address incentivesController, address rewardsAdmin, IStakedToken stakeToken)`** — `:23-33`.
  Reads `STAKED_TOKEN()` off the stake contract, then does the
  **approve-to-0-then-approve-to-max** dance (`:31-32`) for tokens (USDT-style) that reject
  a non-zero→non-zero allowance change.
- **`performTransfer(address to, address reward, uint256 amount) → bool`** — `:36-51`,
  `onlyIncentivesController`.
  `require(reward == address(STAKE_CONTRACT), 'REWARD_TOKEN_NOT_STAKE_CONTRACT')` (`:46`),
  then `STAKE_CONTRACT.stake(to, amount)` (`:48`). The strategy must be pre-funded with the
  underlying AAVE; nothing here pulls it in.
- **`renewApproval()`** — `:54-57`, `onlyRewardsAdmin`. Re-runs 0-then-max.
- **`dropApproval()`** — `:60-62`, `onlyRewardsAdmin`. Sets allowance to 0. The kill switch.
- **`getStakeContract() → address`** — `:65-67`; **`getUnderlyingToken() → address`** — `:70-72`.

<a name="17-rewards-interfaces"></a>

### 1.7 Rewards interfaces

| File | Contents |
|---|---|
| `interfaces/IRewardsDistributor.sol` (177) | `AssetConfigUpdated` (`:20-28`), `Accrued` (`:39-46`), and every distributor view/setter |
| `interfaces/IRewardsController.sol` (203) | `ClaimerSet` (`:20`), `RewardsClaimed` (`:30-36`), `TransferStrategyInstalled` (`:43`), `RewardOracleUpdated` (`:50`) |
| `interfaces/IEmissionManager.sol` (118) | `EmissionAdminUpdated` (`:21`) + the manager surface |
| `interfaces/ITransferStrategyBase.sol` (41) | `EmergencyWithdrawal` (`:8`), errors `CallerNotIncentivesController()`, `OnlyRewardsAdmin()` |
| `interfaces/IPullRewardsTransferStrategy.sol` (15) | `getRewardsVault()` |
| `interfaces/IStakedTokenTransferStrategy.sol` (30) | `renewApproval`, `dropApproval`, `getStakeContract`, `getUnderlyingToken` |
| `interfaces/IStakedToken.sol` (14) | `stake(address,uint256)`, `redeem`, `cooldown`, `STAKED_TOKEN()` |

---

<a name="2-defaultreserveinterestratestrategyv2"></a>

## 2. `DefaultReserveInterestRateStrategyV2`

`src/contracts/misc/DefaultReserveInterestRateStrategyV2.sol` (234 lines).
`contract DefaultReserveInterestRateStrategyV2 is IDefaultInterestRateStrategyV2`.

One deployed instance serves **every reserve in one pool**, keyed by underlying address.
The header comment (`:16-18`) is explicit that an instance cannot be shared across pools:
it caches the addresses provider and indexes rate data by underlying, so two pools would
collide.

**State:**

| Declaration | Line | Notes |
|---|---:|---|
| `IPoolAddressesProvider public immutable ADDRESSES_PROVIDER` | 34 | |
| `uint256 public constant MAX_BORROW_RATE = 1000_00` | 37 | 1000% in bps |
| `uint256 public constant MIN_OPTIMAL_POINT = 1_00` | 40 | 1% |
| `uint256 public constant MAX_OPTIMAL_POINT = 99_00` | 43 | 99% |
| `mapping(address => InterestRateData) internal _interestRateData` | 46 | underlying → params |

**`struct InterestRateData`** (`interfaces/IDefaultInterestRateStrategyV2.sol:24-29`) —
packs into **one storage slot**, 16 + 32 + 32 + 32 = 112 bits:

| Field | Type | Unit |
|---|---|---|
| `optimalUsageRatio` | `uint16` | bps |
| `baseVariableBorrowRate` | `uint32` | bps |
| `variableRateSlope1` | `uint32` | bps |
| `variableRateSlope2` | `uint32` | bps |

`InterestRateDataRay` (`:40-45`) is the same four fields widened to `uint256` in ray. The
conversion is `_bpsToRay(n) = n * 1e23` (`:231-233`): bps → ray is `×1e27/1e4 = ×1e23`.

**`struct CalcInterestRatesLocalVars`** (`:24-31`) — six scratch fields, purely to dodge
stack-too-deep.

**`modifier onlyPoolConfigurator()`** — `:48-54`. Reads
`ADDRESSES_PROVIDER.getPoolConfigurator()` **live** on every call, so a configurator
upgrade takes effect immediately. Reverts `Errors.CallerNotPoolConfigurator()`.

**`constructor(address provider)`** — `:60-63`. `require(provider != address(0), Errors.InvalidAddressesProvider())`.

---

### The model

```
                        borrow APR
                             ^
                             |                                  /
   base + slope1 + slope2 ---|                                 /
                             |                                /
                             |                               /   <- slope2, steep
                             |                              /
          base + slope1  ----|.........................../
                             |                      ..../
                             |            ..........              <- slope1, gentle
                    base  ---|...........
                             +-----------------------------------> utilization U
                             0                       U_optimal   1
```

#### `calculateInterestRates(DataTypes.CalculateInterestRatesParams params) → (uint256 liquidityRate, uint256 variableBorrowRate)`
`external view virtual override` — `:124-170`

**Input struct** (`protocol/libraries/types/DataTypes.sol:309-319`):

| Field | Meaning |
|---|---|
| `unbacked` | Portal-bridged supply with no underlying yet; widens the *supply* denominator only |
| `liquidityAdded` | Underlying arriving in this tx |
| `liquidityTaken` | Underlying leaving in this tx |
| `totalDebt` | Scaled debt × index, i.e. real debt |
| `reserveFactor` | DAO cut, bps |
| `reserve` | The underlying, used as the key into `_interestRateData` |
| `usingVirtualBalance` | **DEPRECATED in 3.4**, ignored here |
| `virtualUnderlyingBalance` | The reserve's accounted liquidity |

**Step by step:**

1. Load and rayify the params (`:127`).
2. Seed `currentLiquidityRate = 0`, `currentVariableBorrowRate = baseVariableBorrowRate` (`:131-132`).
3. **`totalDebt == 0` → return `(0, base)` immediately** (`:145-147`). No debt, no supply
   yield, and the borrow rate sits at the base.
4. Otherwise (`:135-144`):
   ```
   availableLiquidity        = virtualUnderlyingBalance + liquidityAdded - liquidityTaken
   availableLiquidityPlusDebt= availableLiquidity + totalDebt
   borrowUsageRatio          = totalDebt.rayDiv(availableLiquidityPlusDebt)
   supplyUsageRatio          = totalDebt.rayDiv(availableLiquidityPlusDebt + unbacked)
   ```
   Two different utilizations. `borrowUsageRatio` prices borrowing; `supplyUsageRatio` is
   `≤` it and dilutes supplier yield across unbacked (bridged) supply.
5. **Above the kink** (`:149-156`):
   ```
   excess = (U_borrow - U_opt).rayDiv(RAY - U_opt)
   borrowRate = base + slope1 + slope2.rayMul(excess)
   ```
   `excess` runs 0 → 1 across the top segment, so the rate ends at exactly
   `base + slope1 + slope2` at 100%.
6. **Below the kink** (`:157-162`):
   ```
   borrowRate = base + slope1.rayMul(U_borrow).rayDiv(U_opt)
   ```
   Linear from `base` to `base + slope1`.
7. **Supply rate** (`:164-167`):
   ```
   liquidityRate = borrowRate.rayMul(U_supply).percentMul(100_00 - reserveFactor)
   ```
   Borrowers pay on debt; suppliers earn on *total* supply, hence the `×U` — and then the
   DAO takes `reserveFactor`.

**Worked table.** Typical stablecoin params: `optimalUsageRatio = 90_00`,
`baseVariableBorrowRate = 0`, `slope1 = 6_50`, `slope2 = 60_00`, `reserveFactor = 10_00`,
`unbacked = 0`. Computed by evaluating the code above exactly:

| Utilization | Borrow APR | Supply APR |
|---:|---:|---:|
| 0% | 0.0000% | 0.0000% |
| 25% | 1.8056% | 0.4062% |
| 50% | 3.6111% | 1.6250% |
| 75% | 5.4167% | 3.6562% |
| 90% (kink) | 6.5000% | 5.2650% |
| 95% | 36.5000% | 31.2075% |
| 99% | 60.5000% | 53.9055% |
| 100% | 66.5000% | 59.8500% |

The cliff at the kink is the entire design: crossing 90% multiplies the borrow rate 5.6×
within five points of utilization, which is what forces repayment and restores withdrawable
liquidity.

These are **APRs in ray, per second × seconds-per-year**. The protocol compounds the borrow
side (`MathUtils.calculateCompoundedInterest`) and accrues the supply side linearly
(`calculateLinearInterest`) — see `AAVE-DEEP-DIVE.md` §2.2.

#### `setInterestRateParams(address reserve, bytes calldata rateData)`
`external onlyPoolConfigurator` — `:66-71`. ABI-decodes into `InterestRateData` and
forwards. This is the `IReserveInterestRateStrategy` shape the configurator uses generically.

#### `setInterestRateParams(address reserve, InterestRateData calldata rateData)`
`external onlyPoolConfigurator` — `:74-79`. Typed overload, same target. Two selectors,
one behaviour: `0xfd81bb12` (typed) and the bytes variant.

#### `_setInterestRateParams(address reserve, InterestRateData memory rateData)`
`internal` — `:177-208`

Four validations, then one `SSTORE` and an event:

| Check | Line | Error |
|---|---:|---|
| `reserve != address(0)` | 178 | `Errors.ZeroAddressNotValid()` |
| `MIN_OPTIMAL_POINT <= optimalUsageRatio <= MAX_OPTIMAL_POINT` | 180-184 | `Errors.InvalidOptimalUsageRatio()` |
| `variableRateSlope1 <= variableRateSlope2` | 186-189 | `Errors.Slope2MustBeGteSlope1()` |
| `base + slope1 + slope2 <= MAX_BORROW_RATE` | 192-198 | `Errors.InvalidMaxRate()` |

The third check enforces that the curve is convex — a gentler second slope would remove the
incentive cliff. The sum in the fourth is widened to `uint256` first (`:193-195`) so it
cannot overflow the `uint32` fields.

Emits `RateDataUpdate(reserve, optimalUsageRatio, baseVariableBorrowRate, slope1, slope2)` (`:201-207`).

#### View getters

| Function | Line | Returns |
|---|---:|---|
| `getInterestRateData(address)` | 82-84 | `InterestRateDataRay` (ray) |
| `getInterestRateDataBps(address)` | 87-89 | `InterestRateData` (bps, raw storage) |
| `getOptimalUsageRatio(address)` | 92-94 | ray |
| `getVariableRateSlope1(address)` | 97-99 | ray |
| `getVariableRateSlope2(address)` | 102-104 | ray |
| `getBaseVariableBorrowRate(address)` | 107-109 | ray |
| `getMaxVariableBorrowRate(address)` | 112-121 | ray, `base + slope1 + slope2` |

`RateEngine` calls `getInterestRateDataBps` when merging `KEEP_CURRENT` values
(`extensions/v3-config-engine/libraries/RateEngine.sol:135-136`).

#### `_rayifyRateData` / `_bpsToRay`
`internal pure` — `:218-228`, `:231-233`.

**Gotcha:** an unconfigured reserve returns all zeros rather than reverting. Utilization
would then produce a 0% borrow rate — free borrowing. The configurator always calls
`setInterestRateParams` during `initReserve`, so this is unreachable in practice, but it is
why listing must go through the configurator.

---

<a name="3-aaveoracle"></a>

## 3. `AaveOracle`

`src/contracts/misc/AaveOracle.sol` (147 lines). `contract AaveOracle is IAaveOracle`.

Prices everything in one **base currency** — USD with `BASE_CURRENCY_UNIT = 1e8` on most
markets, or WETH with `1e18` on older ones. Every health-factor calculation in
`GenericLogic` funnels through `getAssetPrice`.

**State:**

| Declaration | Line | Notes |
|---|---:|---|
| `IPoolAddressesProvider public immutable ADDRESSES_PROVIDER` | 20 | |
| `mapping(address => AggregatorInterface) private assetsSources` | 23 | `private`, no direct read |
| `IPriceOracleGetter private _fallbackOracle` | 25 | |
| `address public immutable override BASE_CURRENCY` | 26 | `0x0` means USD |
| `uint256 public immutable override BASE_CURRENCY_UNIT` | 27 | e.g. `1e8` |

**`modifier onlyAssetListingOrPoolAdmins()`** — `:32-35`, delegating to
`_onlyAssetListingOrPoolAdmins()` (`:140-146`), which reads the ACL manager live and
requires `isAssetListingAdmin(msg.sender) || isPoolAdmin(msg.sender)`, else
`Errors.CallerNotAssetListingOrPoolAdmin()`. Split into a function to shrink bytecode at
each use site.

#### `constructor(IPoolAddressesProvider provider, address[] assets, address[] sources, address fallbackOracle, address baseCurrency, uint256 baseCurrencyUnit)`
`:47-61`

Sets the provider, the fallback, the sources, then both immutables, and emits
`BaseCurrencySet` (`:60`). Base currency and unit can **never** be changed — switching a
market's numeraire means a new oracle.

#### `getAssetPrice(address asset) → uint256`
`public view override` — `:101-117`

Three branches, in order:

1. `asset == BASE_CURRENCY` → return `BASE_CURRENCY_UNIT` (`:104-105`). Price of the
   numeraire in itself is 1.
2. No source configured → `_fallbackOracle.getAssetPrice(asset)` (`:106-107`).
3. Otherwise `source.latestAnswer()`; if `> 0` cast to `uint256` and return, else fall
   through to the fallback (`:109-115`).

**What this does *not* check — and it matters:**

- **No staleness check.** `latestAnswer()` returns the last answer no matter how old.
  There is no `updatedAt` comparison, no heartbeat check. A frozen Chainlink feed keeps
  serving its last value indefinitely and the protocol will happily liquidate against it.
  `latestRoundData()` exists on the interface (`dependencies/chainlink/AggregatorInterface.sol:23-32`)
  and is deliberately not used.
- **No sanity bounds.** Any positive answer is accepted.
- **Zero or negative → fallback.** If the fallback is `address(0)`, the call to it reverts
  with no data. Every pool action that prices this asset then reverts. Failure is loud, but
  it is a full denial of service for the market.

Aave's answer to this is the **price oracle sentinel** on L2s — which **was removed in
3.7** (`docs/3.7/sentinel-removal.md`); grace-period protection now lives in
`PoolConfigurator.setLiquidationGracePeriod`.

#### `getAssetsPrices(address[] assets) → uint256[]`
`external view` — `:120-128`. Loops `getAssetPrice`.

#### `setAssetSources(address[] assets, address[] sources)`
`external onlyAssetListingOrPoolAdmins` — `:64-69` → `_setAssetsSources` (`:83-89`).
`require(assets.length == sources.length, Errors.InconsistentParamsLength())`, then one
`AssetSourceUpdated` per asset. Setting a source to `address(0)` reverts nothing — it just
routes that asset to the fallback.

#### `setFallbackOracle(address fallbackOracle)`
`external onlyAssetListingOrPoolAdmins` — `:72-76` → `_setFallbackOracle` (`:95-98`).
Emits `FallbackOracleUpdated`. No zero check.

#### `getSourceOfAsset(address) → address` — `:131-133`; `getFallbackOracle() → address` — `:136-138`.

**Decimal contract.** Every feed must report in `BASE_CURRENCY_UNIT` decimals. Nothing here
enforces it — enforcement lives in the config engine, where `PriceFeedEngine` requires
`decimals() == 8` (`extensions/v3-config-engine/libraries/PriceFeedEngine.sol:27-30`). A
feed added by direct `setAssetSources` bypasses that check entirely.

---

<a name="4-the-stata-token-stack"></a>

## 4. The stata-token stack (ERC-4626 aToken wrapper)

**The problem.** An aToken rebases: `balanceOf` grows every block because it is
`scaledBalance × liquidityIndex`. That breaks every integration that assumes balances only
change on transfer — AMM pools, ERC-4626 vault composition, accounting systems,
cross-chain bridges.

**The fix.** `StataTokenV2` ("stata" = *static* aToken, `wa`-prefixed on-chain, e.g.
`waEthUSDC`) is a vault whose share count is fixed and whose *exchange rate* rises. It holds
aTokens; you hold shares. It is an ERC-4626 vault where the exchange rate is literally the
pool's liquidity index.

Four files, three layers:

```
StataTokenV2                                        StataTokenV2.sol (120)
  is ERC20PermitUpgradeable                          -- name/symbol/permit
   , ERC20AaveLMUpgradeable                          -- liquidity-mining passthrough (309)
   , ERC4626StataTokenUpgradeable                    -- the vault itself (311)
   , PausableUpgradeable                             -- emergency admin can freeze
   , Rescuable                                       -- ACL admin can pull stuck tokens
       ^
       | deployed behind a TransparentUpgradeableProxy by
       |
StataTokenFactory                                   StataTokenFactory.sol (90)
  one stata token per underlying, CREATE2-deterministic
```

Both mixins use **ERC-7201 namespaced storage** so that mixing them into one contract
cannot collide. Each computes its slot as
`keccak256(abi.encode(uint256(keccak256("<namespace>")) - 1)) & ~bytes32(uint256(0xff))`.

### 4.1 `ERC4626StataTokenUpgradeable` — the vault half

`src/contracts/extensions/stata-token/ERC4626StataTokenUpgradeable.sol` (311 lines).
`abstract contract ERC4626StataTokenUpgradeable is ERC4626Upgradeable, IERC4626StataToken`.

**Namespaced storage** (`:25-42`):

```solidity
/// @custom:storage-location erc7201:aave-dao.storage.ERC4626StataToken
struct ERC4626StataTokenStorage { IERC20 _aToken; }
bytes32 private constant ERC4626StataTokenStorageLocation =
  0x55029d3f54709e547ed74b2fc842d93107ab1490ab7555dd9dd0bf6451101900;
```

**Immutables / constants:** `uint256 public constant RAY = 1e27` (`:44`),
`IPool public immutable POOL` (`:46`),
`IPoolAddressesProvider public immutable POOL_ADDRESSES_PROVIDER` (`:47`).

#### The exchange rate — `_rate()`
`internal view` — `:308-310`

```solidity
return POOL.getReserveNormalizedIncome(asset());
```

That is it. The vault's entire valuation is the pool's liquidity index, projected to
`block.timestamp`. No internal accounting, no snapshot, nothing to manipulate.

- `_convertToShares(assets, rounding)` — `:292-298` — `assets.mulDiv(RAY, _rate(), rounding)`
- `_convertToAssets(shares, rounding)` — `:300-306` — `shares.mulDiv(_rate(), RAY, rounding)`

Rounding comes from OpenZeppelin's `ERC4626Upgradeable`, which always rounds against the
caller: shares down on deposit, assets down on redeem, shares up on withdraw. Combined with
an index that only ever increases, there is **no donation/inflation attack surface** — a
donated aToken does not change `_rate()`, it just becomes rescuable dust (see `maxRescue`).

#### `constructor(IPool pool)` — `:49-52`. Caches `POOL` and `pool.ADDRESSES_PROVIDER()`.

#### `__ERC4626StataToken_init(address newAToken)` / `__ERC4626StataToken_init_unchained`
`internal onlyInitializing` — `:54-74`

- `require`-equivalent: `IAToken(newAToken).POOL() != address(POOL)` → revert
  `PoolAddressMismatch(poolOfAToken)` (`:63-64`). Prevents wrapping an aToken from a
  different market.
- Reads `UNDERLYING_ASSET_ADDRESS()`, stores `_aToken`, and
  `SafeERC20.forceApprove(aTokenUnderlying, address(POOL), type(uint256).max)` (`:71`) so
  `POOL.deposit` works without per-call approvals.

#### `depositATokens(uint256 assets, address receiver) → uint256 shares`
`external` — `:77-88`

Deposit **aTokens** rather than underlying, skipping the pool entirely.

```solidity
uint256 actualUserBalance = IERC20(aToken()).balanceOf(_msgSender());
if (assets > actualUserBalance) { assets = actualUserBalance; }
```

The clamp at `:80-82` is deliberate (comment at `:78`): the caller's aToken balance grows
between signing and mining, so a UI passes a slightly-too-large amount and lets the contract
take what is actually there. **Consequence:** this function silently deposits *less* than
asked. Never treat the argument as exact; use the returned `shares`.

Calls `_deposit(..., depositToAave = false)`.

#### `depositWithPermit(uint256 assets, address receiver, uint256 deadline, SignatureParams sig, bool depositToAave) → uint256`
`external` — `:91-122`

- Picks the token to permit: `asset()` if `depositToAave`, else `aToken()` (`:98`).
- The `permit` is wrapped in `try/catch {}` (`:100-110`) — a permit that was already
  front-run does not brick the deposit; the pre-existing allowance is used.
- Same balance clamp (`:114-117`), applied to the underlying too "to make it consistent".

#### `redeemATokens(uint256 shares, address receiver, address owner) → uint256 assets`
`external` — `:125-134`. `previewRedeem` then `_withdraw(..., withdrawFromAave = false)`.
Exit in aTokens, which never touches the pool and so **works even when the reserve has no
free liquidity**.

#### `_deposit(address caller, address receiver, uint256 assets, uint256 shares, bool depositToAave)`
`internal virtual` — `:213-242`

1. `if (shares == 0) revert StaticATokenInvalidZeroShares()` (`:220-222`).
2. If `depositToAave`: `safeTransferFrom(underlying, caller, this, assets)` then
   `POOL.deposit(cachedAsset, assets, address(this), 0)` (`:231-234`).
   Else: `safeTransferFrom(_aToken, caller, this, assets)` (`:236-237`).
3. `_mint(receiver, shares)`; `emit Deposit`.

The transfer-before-mint ordering is justified in a long comment (`:223-229`): with an
ERC-777-style asset, `transferFrom` can reenter *before* the transfer, and at that moment
neither the assets nor the shares have moved — a consistent state.

`_deposit(caller, receiver, assets, shares)` (`:244-251`) is the 4-arg OZ override,
hardcoding `depositToAave = true`. So plain `deposit`/`mint` take underlying.

#### `_withdraw(address caller, address receiver, address owner, uint256 assets, uint256 shares, bool withdrawFromAave)`
`internal virtual` — `:253-280`

`_spendAllowance` if `caller != owner` (`:261-263`), `_burn(owner, shares)`, then either
`POOL.withdraw(asset(), assets, receiver)` or `safeTransfer(_aToken, receiver, assets)`.
Burn-before-transfer, again for reentrancy consistency (`:265-270`). The 5-arg OZ override
at `:282-290` hardcodes `withdrawFromAave = true`.

#### `totalAssets() → uint256`
`public view override` — `:155-157`

```solidity
return _convertToAssets(totalSupply(), Math.Rounding.Floor);
```

**Not** `aToken.balanceOf(address(this))`. Deliberately: the vault reports what it *owes*,
not what it *holds*. Donated aTokens are excluded, which is what keeps the exchange rate
donation-proof and makes `maxRescue` computable.

#### `maxRedeem(address owner) → uint256`
`public view override` — `:160-179`

- Reserve inactive or paused → 0 (`:161-169`).
- Otherwise `min(balanceOf(owner), convertToShares(POOL.getVirtualUnderlyingBalance(asset())))`
  (`:172-178`). Bounded by real withdrawable liquidity, so a 4626 aggregator sees the truth.

#### `maxDeposit(address) → uint256`
`public view override` — `:182-203`

- Inactive, paused **or frozen** → 0 (`:186-191`). Note `maxRedeem` does not check frozen —
  freezing stops entry, not exit.
- `supplyCap == 0` → `type(uint256).max` (`:197`).
- Else remaining headroom (`:200-202`):
  ```solidity
  uint256 currentSupply = (IAToken(reserveData.aTokenAddress).scaledTotalSupply() +
    reserveData.accruedToTreasury).mulDiv(_rate(), RAY, Math.Rounding.Ceil);
  return currentSupply >= supplyCap ? 0 : supplyCap - currentSupply;
  ```
  Includes `accruedToTreasury` and rounds **up**, matching how `ValidationLogic` computes
  the cap so the preview cannot promise a deposit the pool would reject.

#### `maxMint(address) → uint256` — `:143-147`. `maxDeposit` converted to shares, preserving the `uint256.max` sentinel.
#### `maxWithdraw(address owner) → uint256` — `:150-152`. `convertToAssets(maxRedeem(owner))`.

#### `latestAnswer() → int256`
`external view` — `:206-211`

```solidity
uint256 p = IAaveOracle(POOL_ADDRESSES_PROVIDER.getPriceOracle()).getAssetPrice(asset());
return int256(p.mulDiv(_rate(), RAY, Math.Rounding.Floor));
```

A **Chainlink-shaped price feed for the stata token itself**: underlying price × index. This
is what lets a stata token be listed as collateral in another lending market. It inherits
every weakness of `AaveOracle.getAssetPrice` (§3), including no staleness check.

#### `aToken() → address` — `public view`, `:137-140`.

### 4.2 `ERC20AaveLMUpgradeable` — the liquidity-mining half

`src/contracts/extensions/stata-token/ERC20AaveLMUpgradeable.sol` (309 lines).

**The problem it solves:** while the vault holds aTokens, *it* is the address the
`RewardsController` credits. Individual share holders would get nothing. So the vault runs
its own miniature copy of the same index algorithm, one level down.

**Namespaced storage** (`:21-37`):

```solidity
/// @custom:storage-location erc7201:aave-dao.storage.ERC20AaveLM
struct ERC20AaveLMStorage {
  address _referenceAsset;                                    // the aToken
  address[] _rewardTokens;
  mapping(address reward => RewardIndexCache cache) _startIndex;
  mapping(address user => mapping(address reward => UserRewardsData)) _userRewardsData;
}
bytes32 private constant ERC20AaveLMStorageLocation =
  0x4fad66563f105be0bff96185c9058c4934b504d3ba15ca31e86294f0b01fd200;
```

`IRewardsController public immutable INCENTIVES_CONTROLLER` (`:39`); the constructor
(`:41-46`) reverts `ZeroIncentivesControllerIsForbidden()` on a zero address.

**The `_startIndex` trick.** When a reward is first registered, the vault snapshots the
controller's *current* index as `lastUpdatedIndex` (`:301-305`). A user whose
`rewardsIndexOnLastInteraction == 0` is then measured from that start index, not from zero
(`:243-245`). Without it the first holder would appear to be owed every reward ever emitted
to the asset since the beginning of time.

| Function | Line | Purpose |
|---|---:|---|
| `__ERC20AaveLM_init(address referenceAsset_)` | 48-59 | Stores the reference asset, then `refreshRewardTokens()` |
| `claimRewardsOnBehalf(address,address,address[])` | 62-73 | Caller must be the user or the controller-authorised claimer, else `InvalidClaimer(msgSender)` |
| `claimRewards(address receiver, address[] rewards)` | 76-78 | For `msg.sender` |
| `claimRewardsToSelf(address[] rewards)` | 81-83 | |
| `refreshRewardTokens()` | 86-92 | Pulls `getRewardsByAsset` and registers any new ones. **Permissionless** — anyone can sync |
| `collectAndUpdateRewards(address reward)` | 95-105 | Claims the vault's own rewards from the controller into the vault |
| `isRegisteredRewardToken(address)` | 108-111 | |
| `getCurrentRewardsIndex(address reward)` | 114-121 | The `nextIndex` half of `controller.getAssetIndex` |
| `getTotalClaimableRewards(address reward)` | 124-134 | Vault balance + what the controller still owes the vault |
| `getClaimableRewards(address user, address reward)` | 137-139 | |
| `getUnclaimedRewards(address user, address reward)` | 142-145 | Banked only |
| `getReferenceAsset()` | 148-151 | |
| `rewardTokens()` | 154-157 | |

#### `_update(address from, address to, uint256 amount)`
`internal virtual override` — `:164-177`

The hook every share transfer, mint and burn goes through. For **each** registered reward it
reads the current index and calls `_updateUser` for `from` and `to` (skipping zero
addresses, and skipping `to` when `from == to`), then `super._update`.

**Cost warning:** this is `O(rewardTokens.length)` external calls to the controller on
*every* share transfer. Each `getCurrentRewardsIndex` is a full `getAssetIndex` call. Three
reward tokens make a plain ERC20 transfer meaningfully expensive.

#### `_updateUser(address user, uint256 currentRewardsIndex, address rewardToken)`
`internal` — `:185-198`. Banks pending into `unclaimedRewards` if balance > 0, then always
writes `rewardsIndexOnLastInteraction`.

#### `_getPendingRewards(uint256 balance, uint256 rewardsIndexOnLastInteraction, uint256 currentRewardsIndex)`
`internal view` — `:207-216`. `balance * Δindex / 10**decimals()`. Note it uses the
**vault's** decimals, which `ERC4626Upgradeable` set equal to the underlying's — the same
`assetUnit` the controller uses, so the two index scales agree.

#### `_getClaimableRewards(address user, address reward, uint256 balance, uint256 currentRewardsIndex)`
`internal view` — `:226-248`. Reverts `RewardNotInitialized(reward)` if unregistered
(`:234-236`); applies the `_startIndex` fallback.

#### `_claimRewardsOnBehalf(address onBehalfOf, address receiver, address[] rewards)`
`internal virtual` — `:256-293`

Per reward: skip `address(0)` (`:262-264`); compute the user's entitlement; if the vault's
balance is short, `collectAndUpdateRewards` to top up (`:276-278`); if **still** short, pay
what exists and carry the shortfall in `unclaimedRewards` (`:280-283`). Only then write
state and `safeTransfer`. Graceful degradation rather than a revert.

#### `_registerRewardToken(address reward)`
`internal` — `:299-308`. Idempotent; pushes, snapshots the start index, emits
`RewardTokenRegistered`.

### 4.3 `StataTokenV2` — the assembly

`src/contracts/extensions/stata-token/StataTokenV2.sol` (120 lines).

- **`constructor(IPool pool, IRewardsController rewardsController)`** — `:31-36`. Wires both
  parent constructors and calls `_disableInitializers()`, so the implementation itself can
  never be initialised.
- **`modifier onlyPauseGuardian()`** — `:38-41`. `canPause(_msgSender())` else
  `OnlyPauseGuardian(_msgSender())`.
- **`initialize(address aToken, string staticATokenName, string staticATokenSymbol)`** —
  `:43-53`, `external initializer`. Chains `__ERC20_init`, `__ERC20Permit_init`,
  `__ERC20AaveLM_init`, `__ERC4626StataToken_init`, `__Pausable_init`.
- **`setPaused(bool paused)`** — `:56-59`, `onlyPauseGuardian`.
- **`canPause(address actor) → bool`** — `:80-82`. `IACLManager(...).isEmergencyAdmin(actor)`.
- **`whoCanRescue() → address`** — `:62-64`. `POOL_ADDRESSES_PROVIDER.getACLAdmin()`.
- **`maxRescue(address asset) → uint256`** — `:67-77`. For the aToken:
  `balance − ceil(convertToAssets(totalSupply))`, i.e. exactly the donated surplus, never
  user backing. For anything else, `type(uint256).max`.
- **`nonces(address)`** — `:85-89`, disambiguates the diamond inheritance.
- **`decimals()`** — `:92-101`, resolves to `ERC4626Upgradeable.decimals()` (the
  underlying's).
- **`_claimRewardsOnBehalf(...)`** — `:103-109`, adds `whenNotPaused`.
- **`_update(address,address,uint256)`** — `:113-119`, `whenNotPaused`, then
  `ERC20AaveLMUpgradeable._update`. The comment at `:111-112` explains why `whenNotPaused`
  is placed here rather than using `ERC20PausableUpgradeable`: the LM mixin already
  overrides `_update`, and stacking both overrides cleanly is not possible.

**Pause semantics:** pausing blocks transfers, deposits, withdrawals *and* reward claims —
everything routes through `_update` or `_claimRewardsOnBehalf`. It is a hard freeze.

### 4.4 `StataTokenFactory`

`src/contracts/extensions/stata-token/StataTokenFactory.sol` (90 lines).
`contract StataTokenFactory is Initializable, IStataTokenFactory`.

**Immutables:** `POOL` (`:20`), `INITIAL_OWNER` (`:23`), `TRANSPARENT_PROXY_FACTORY` (`:26`),
`STATA_TOKEN_IMPL` (`:29`). **State:** `_underlyingToStataToken` (`:31`), `_stataTokens` (`:32`).

#### `createStataTokens(address[] underlyings) → address[]`
`external` — `:50-79`. **Permissionless.**

Per underlying, if not already created:
1. `POOL.getReserveAToken(underlyings[i])`; `address(0)` → revert
   `NotListedUnderlying(aTokenAddress)` (`:55-56`). Note the error reports the *aToken*
   address (always zero here), not the underlying — a small reporting bug.
2. Name/symbol are derived: `"Wrapped " + aToken.name()` and `"w" + aToken.symbol()`
   (`:57`, `:64-65`), giving e.g. `waEthUSDC`.
3. `TRANSPARENT_PROXY_FACTORY.createDeterministic(STATA_TOKEN_IMPL, INITIAL_OWNER, initCalldata, salt)`
   with **`salt = bytes32(uint256(uint160(underlying)))`** (`:67`). The address is a pure
   function of the underlying, so it is predictable before deployment and can only ever be
   claimed once.
4. Record both mappings, `emit StataTokenCreated`.

Already-created entries are returned from cache (`:74-76`), so the call is idempotent.

- **`initialize()`** — `:47`, empty; the constructor `_disableInitializers()` at `:40`.
- **`getStataTokens() → address[]`** — `:82-84`; **`getStataToken(address) → address`** — `:87-89`.

---

<a name="5-the-v3-config-engine"></a>

## 5. The v3 config engine

**The problem.** Listing an asset on Aave means ~10 separate `PoolConfigurator` calls in the
right order with the right units. Governance payloads that did this by hand were long,
error-prone and hard to review.

**The fix.** A declarative layer: a payload *declares* the desired configuration as structs,
and the engine translates it into configurator calls. A reviewer reads a table of numbers
instead of a transaction script.

```
  DAO proposal executor
        |  delegatecall
        v
  MyListingPayload is AaveV3Payload            extensions/v3-config-engine/AaveV3Payload.sol
    - overrides newListings() / capsUpdates() / ...
    - execute() collects them and delegatecalls the engine
        |  delegatecall  (payload's permissions are used)
        v
  AaveV3ConfigEngine                            AaveV3ConfigEngine.sol
    - stateless facade, holds only immutables
        |  internal library calls  [3.7: was delegatecall pre-3.7]
        v
  ListingEngine / CapsEngine / BorrowEngine / CollateralEngine
  RateEngine / PriceFeedEngine / EModeEngine
        |  external calls
        v
  PoolConfigurator  +  AaveOracle
```

**Why delegatecall at the payload→engine boundary.** The engine has no permissions of its
own. The payload does (governance granted it `POOL_ADMIN`). `delegatecall` makes the
configurator see `msg.sender == payload`. That is why both `AaveV3ConfigEngine` and
`AaveV3Payload` carry an all-caps warning that they **must be stateless** (`AaveV3ConfigEngine.sol:15`,
`AaveV3Payload.sol:11`) — a `delegatecall`ed contract writing storage would write into the
*payload's* slots.

**[3.7]** The engine→library boundary is now plain internal library calls
(`AaveV3ConfigEngine.sol:77` etc.); before 3.7 each engine library was separately deployed
and `delegatecall`ed, and `IAaveV3ConfigEngine` carried an `EngineLibraries` struct and
per-engine address getters. Both are gone.

### 5.1 `EngineFlags` — the sentinel system

`src/contracts/extensions/v3-config-engine/EngineFlags.sol` (34 lines).

| Constant | Line | Value | Meaning |
|---|---:|---|---|
| `KEEP_CURRENT` | 138 | `type(uint256).max - 42` | Leave this numeric field unchanged |
| `KEEP_CURRENT_STRING` | 142 | `'KEEP_CURRENT_STRING'` | Leave this string unchanged |
| `KEEP_CURRENT_ADDRESS` | 146-147 | `0x…0050` | Leave this address unchanged |
| `ENABLED` | 150 | `1` | true |
| `DISABLED` | 153 | `0` | false |

- **`toBool(uint256 flag) → bool`** — `:156-159`.
  `require(flag == 0 || flag == 1, 'INVALID_CONVERSION_TO_BOOL')`. So a `KEEP_CURRENT`
  that reaches `toBool` reverts loudly rather than being read as `true`.
- **`fromBool(bool) → uint256`** — `:162-164`.

Booleans are carried as `uint256` purely so `KEEP_CURRENT` has somewhere to live — a `bool`
has no spare value. The comments (`:137`, `:141`, `:145`) each admit the design is a
"strong assumption" that these magic values are never legitimate.

### 5.2 `IAaveV3ConfigEngine` — the config language

`src/contracts/extensions/v3-config-engine/IAaveV3ConfigEngine.sol` (337 lines). 15 structs.

**`Listing`** (`:69-83`) — everything about a new asset in one struct:

| Field | Type | Notes |
|---|---|---|
| `asset` | `address` | |
| `assetSymbol` | `string` | Used to build the aToken/vToken names |
| `priceFeed` | `address` | Must be a live 8-decimal feed |
| `rateStrategyParams` | `InterestRateInputData` | Mandatory even if not borrowable |
| `enabledToBorrow` | `uint256` | `ENABLED`/`DISABLED` |
| `flashloanable` | `uint256` | Independent of borrowing |
| `ltv` | `uint256` | bps; ignored unless `liqThreshold > 0` |
| `liqThreshold` | `uint256` | bps; **`0` means "not collateral"** |
| `liqBonus` | `uint256` | bps *above* 100%, e.g. `5_00` = 5% bonus |
| `reserveFactor` | `uint256` | bps |
| `supplyCap` / `borrowCap` | `uint256` | Whole tokens, not wei |
| `liqProtocolFee` | `uint256` | bps of the bonus taken by the DAO |

**[3.7 removed]** `borrowableInIsolation`, `withSiloedBorrowing`, `debtCeiling`.

**`InterestRateInputData`** (`:26-30`) — the four rate params as `uint256` so they can carry
`KEEP_CURRENT`.

**`PoolContext`** (`:41-43`) — `networkName`, `networkAbbreviation`; feeds the token naming.

Update structs: **`CapsUpdate`** (`:113-117`), **`PriceFeedUpdate`** (`:127-130`),
**`CollateralUpdate`** (`:143-149`), **`BorrowUpdate`** (`:161-166`),
**`AssetEModeUpdate`** (`:178-184`, with `ltvzero` **[3.7]**),
**`EModeCategoryUpdate`** (`:194-201`, with `isolated` **[3.7]**),
**`EModeCategoryCreation`** (`:214-222`, with `borrowables`, `collaterals`, `isolated`),
**`RateStrategyUpdate`** (`:236-239`).

Plumbing structs: **`Basic`** (`:12-15`), **`EngineConstants`** (`:17-24`),
**`TokenImplementations`** (`:95-98`), **`ListingWithCustomImpl`** (`:100-103`),
**`RepackedListings`** (`:85-93`).

### 5.3 `AaveV3ConfigEngine`

`src/contracts/extensions/v3-config-engine/AaveV3ConfigEngine.sol` (131 lines).

**Immutables** (`:25-32`): `POOL`, `POOL_CONFIGURATOR`, `ORACLE`, `ATOKEN_IMPL`,
`VTOKEN_IMPL`, `REWARDS_CONTROLLER`, `COLLECTOR`, `DEFAULT_INTEREST_RATE_STRATEGY`.

**`constructor(address aTokenImpl, address vTokenImpl, EngineConstants engineConstants)`** —
`:34-55`. Two `require`s: `'ONLY_NONZERO_ENGINE_CONSTANTS'` over the six engine constants
(`:35-43`) and `'ONLY_NONZERO_TOKEN_IMPLS'` (`:45`).

| Function | Line | Delegates to | Selector |
|---|---:|---|---|
| `listAssets(PoolContext, Listing[])` | 58-70 | wraps each `Listing` with the default impls, then `listAssetsCustom` | `0x3149a503` |
| `listAssetsCustom(PoolContext, ListingWithCustomImpl[])` | 73-78 | `ListingEngine.executeCustomAssetListing` | `0xc3211013` |
| `updateCaps(CapsUpdate[])` | 81-83 | `CapsEngine.executeCapsUpdate` | `0x55caa163` |
| `updatePriceFeeds(PriceFeedUpdate[])` | 86-88 | `PriceFeedEngine.executePriceFeedsUpdate` | `0x927c4003` |
| `updateCollateralSide(CollateralUpdate[])` | 91-93 | `CollateralEngine.executeCollateralSide` | `0x02da32ae` |
| `updateBorrowSide(BorrowUpdate[])` | 96-98 | `BorrowEngine.executeBorrowSide` | `0x104116c3` |
| `updateRateStrategies(RateStrategyUpdate[])` | 101-103 | `RateEngine.executeRateStrategiesUpdate` | `0xb79421eb` |
| `createEModeCategories(EModeCategoryCreation[])` | 106-108 | `EModeEngine.executeEModeCategoriesCreate` | `0xf725ed56` |
| `updateEModeCategories(EModeCategoryUpdate[])` | 111-113 | `EModeEngine.executeEModeCategoriesUpdate` | `0x4b56247b` |
| `updateAssetsEMode(AssetEModeUpdate[])` | 116-118 | `EModeEngine.executeAssetsEModeUpdate` | `0x963ec016` |

`listAssets` requires `listings.length != 0` → `'AT_LEAST_ONE_ASSET_REQUIRED'` (`:59`).
`_getEngineConstants()` (`:120-129`) repacks the immutables into a memory struct for the
libraries.

### 5.4 `AaveV3Payload` — the template method

`src/contracts/extensions/v3-config-engine/AaveV3Payload.sol` (183 lines). `abstract`.

`IEngine public immutable CONFIG_ENGINE` (`:30`), set in the constructor (`:32-34`).

#### `execute()`
`external` — `:42-121`

1. `_preExecute()` (`:37`, overridable hook).
2. Call all ten `public view virtual` getters (`:45-54`). Unoverridden ones return empty
   arrays — the default bodies are literally `{}` (`:131-178`).
3. For each non-empty array, `address(CONFIG_ENGINE).functionDelegateCall(abi.encodeWithSelector(...))`.
4. `_postExecute()` (`:40`).

**The order is fixed and load-bearing** (`:56-118`):

```
listings -> listingsCustom -> eModeCategories -> assetsEModes -> newEmodes
         -> borrows -> collaterals -> rates -> priceFeeds -> caps
```

eMode categories are configured before assets are assigned to them; borrow config precedes
collateral config; caps come last so they apply to a fully-configured reserve.

**Overridable surface** — a payload implements only what it needs:

| Getter | Line | Returns |
|---|---:|---|
| `newListings()` | 131 | `Listing[]` |
| `newListingsCustom()` | 134-139 | `ListingWithCustomImpl[]` |
| `capsUpdates()` | 142 | `CapsUpdate[]` |
| `collateralsUpdates()` | 145 | `CollateralUpdate[]` |
| `borrowsUpdates()` | 148 | `BorrowUpdate[]` |
| `priceFeedsUpdates()` | 151 | `PriceFeedUpdate[]` |
| `eModeCategoryCreations()` | 154-159 | `EModeCategoryCreation[]` |
| `eModeCategoriesUpdates()` | 162-167 | `EModeCategoryUpdate[]` |
| `assetsEModeUpdates()` | 170 | `AssetEModeUpdate[]` |
| `rateStrategiesUpdates()` | 173-178 | `RateStrategyUpdate[]` |
| `getPoolContext()` | 182 | **abstract** — every payload must supply it |

`_bpsToRay(uint256) → uint256` — `:126-128`, `(amount * RAY) / 10_000`.

### 5.5 `ListingEngine`

`src/contracts/extensions/v3-config-engine/libraries/ListingEngine.sol` (146 lines).

#### `executeCustomAssetListing(PoolContext, EngineConstants, ListingWithCustomImpl[])`
`internal` — `:15-39`

```
require(listings.length != 0)                    'AT_LEAST_ONE_ASSET_REQUIRED'   :20
_repackListing(listings)                                                          :22
PriceFeedEngine.executePriceFeedsUpdate(...)     <- price BEFORE listing          :24
_initAssets(...)                                 <- PoolConfigurator.initReserves :26
CapsEngine.executeCapsUpdate(...)                                                 :34
BorrowEngine.executeBorrowSide(...)                                               :36
CollateralEngine.executeCollateralSide(...)                                       :38
```

Price feeds go first because `initReserves` can trigger reads that need a price; caps and
risk parameters go after because they target an existing reserve.

#### `_repackListing(ListingWithCustomImpl[]) → RepackedListings`
`internal pure` — `:41-109`

One pass, fanning each `Listing` out into the six per-engine update arrays plus the rate
array. `require(listings[i].base.asset != address(0), 'INVALID_ASSET')` (`:59`). The rate
params are narrowed with `SafeCast` (`:88-95`): `toUint16` for the optimal point, `toUint32`
for the three rates. A payload passing `KEEP_CURRENT` into a *listing's* rate params reverts
here — correctly, since a new reserve has nothing to keep.

#### `_initAssets(PoolContext, IPoolConfigurator, address[] ids, Basic[] basics, InterestRateData[] rates)`
`internal` — `:112-145`

Builds `ConfiguratorInputTypes.InitReserveInput[]` and makes **one** batched
`poolConfigurator.initReserves(initReserveInputs)` call (`:144`). The naming convention
(`:128-140`):

| Field | Template | Example |
|---|---|---|
| `aTokenName` | `'Aave ' + networkName + ' ' + assetSymbol` | `Aave Ethereum USDC` |
| `aTokenSymbol` | `'a' + networkAbbreviation + assetSymbol` | `aEthUSDC` |
| `variableDebtTokenName` | `'Aave ' + networkName + ' Variable Debt ' + assetSymbol` | `Aave Ethereum Variable Debt USDC` |
| `variableDebtTokenSymbol` | `'variableDebt' + networkAbbreviation + assetSymbol` | `variableDebtEthUSDC` |

`interestRateData` is `abi.encode(rates[i])` (`:126`) — the bytes form the configurator
forwards to the strategy.

### 5.6 `CapsEngine`

`libraries/CapsEngine.sol` (31 lines). `executeCapsUpdate` (`:8-15`) requires a non-empty
array (`'AT_LEAST_ONE_UPDATE_REQUIRED'`, `:12`); `_configureCaps` (`:17-30`) calls
`setSupplyCap` / `setBorrowCap` only for fields that are not `KEEP_CURRENT` (`:22-28`).

### 5.7 `BorrowEngine`

`libraries/BorrowEngine.sol` (56 lines). `executeBorrowSide` (`:12-19`) → `_configBorrowSide`
(`:21-55`). Per update:

- `enabledToBorrow`: if set, `setReserveBorrowing`; **else read the live flag back into the
  struct** (`:32-35`). That write-back matters — `ListingEngine` reuses the struct.
- **`require((reserveFactor > 0 && reserveFactor <= 100_00) || reserveFactor == KEEP_CURRENT, 'INVALID_RESERVE_FACTOR')`**
  (`:38-42`). A **zero reserve factor is rejected**: the DAO must always take a cut.
- `flashloanable`: `setReserveFlashLoaning` if set (`:48-53`).

### 5.8 `CollateralEngine`

`libraries/CollateralEngine.sol` (88 lines). `executeCollateralSide` (`:14-21`) →
`_configCollateralSide` (`:23-87`).

- The whole block is gated on `liqThreshold != 0` (`:29`) — threshold zero means "not
  collateral", so LTV and bonus are meaningless.
- Two booleans classify the update (`:30-36`): `notAllKeepCurrent` and
  `atLeastOneKeepCurrent`. Only when **both** hold does it read current values back
  (`:38-62`) — the read is skipped when every field is supplied, saving gas.
- **`liqBonus` round-trip:** the engine's convention is "bonus above 100%", the protocol
  stores "100% + bonus". Reading back therefore subtracts `100_00` (`:60`, with the comment
  at `:59`) and writing adds it (`:77`).
- **The critical invariant** (`:66-69`):
  ```solidity
  require(updates[i].liqThreshold.percentMul(100_00 + updates[i].liqBonus) <= 100_00,
          'INVALID_LT_LB_RATIO');
  ```
  `LT × LB ≤ 100%`. Violating it means a position liquidated exactly at its threshold hands
  the liquidator more collateral than the position holds — instant bad debt. This single
  line is the most important check in the engine.
- `liqProtocolFee`: `require(< 100_00, 'INVALID_LIQ_PROTOCOL_FEE')` (`:83`), then
  `setLiquidationProtocolFee`.

### 5.9 `RateEngine`

`libraries/RateEngine.sol` (95 lines). `executeRateStrategiesUpdate` (`:100-116`) →
`_unpackRatesUpdate` (`:169-182`) → `_configRateStrategies` (`:118-167`).

If any of the four params is `KEEP_CURRENT`, one `getInterestRateDataBps(asset)` call fetches
all four current values and fills the gaps (`:133-153`). Then `SafeCast`-narrowed and
re-encoded into `poolConfigurator.setReserveInterestRateData(asset, abi.encode(...))`
(`:155-165`).

### 5.10 `PriceFeedEngine`

`libraries/PriceFeedEngine.sol` (37 lines). `executePriceFeedsUpdate` (`:8-15`) →
`_setPriceFeeds` (`:17-36`). Three validations per feed:

| Check | Line | Error |
|---|---:|---|
| `priceFeed != address(0)` | 22 | `'PRICE_FEED_ALWAYS_REQUIRED'` |
| `latestAnswer() > 0` | 23-26 | `'FEED_SHOULD_RETURN_POSITIVE_PRICE'` |
| `decimals() == 8` | 27-30 | `'FEED_MUST_USE_8_DECIMALS'` |

Then one batched `oracle.setAssetSources(assets, sources)` (`:35`). The 8-decimal
requirement is the check `AaveOracle` itself lacks (§3).

### 5.11 `EModeEngine`

`libraries/EModeEngine.sol` (203 lines). Declares `error NoAvailableEmodeCategory()` (`:15`).

#### `executeEModeCategoriesCreate(EngineConstants, EModeCategoryCreation[])`
`internal` — `:26-62`

Per creation: reject a `KEEP_CURRENT_STRING` label (`:31-35`, `'INVALID_LABEL'`), allocate
an id with `_findFirstUnusedEmodeCategory`, call `setEModeCategory(id, ltv, lt, 100_00 + liqBonus, label, isolated)`
(`:37-46`), then loop `setAssetCollateralInEMode` over `collaterals` (`:47-53`) and
`setAssetBorrowableInEMode` over `borrowables` (`:54-60`).

Note this path has **no `INVALID_LT_LB_RATIO` check** — unlike `_configEModeCategories`
below and `CollateralEngine`. A creation with a bad LT×LB pair is not caught by the engine;
`PoolConfigurator` must catch it.

#### `_findFirstUnusedEmodeCategory(IPool pool) → uint8`
`private view` — `:196-202`

Scans ids `1..255` for the first with `liquidationThreshold == 0`, reverting
`NoAvailableEmodeCategory()` if full. **Id 0 is skipped deliberately** (`:197`) — it is the
reserved "no eMode" default. Categories cannot be deleted, so ids are only ever reused if a
category's threshold is zeroed.

#### `executeEModeCategoriesUpdate` / `_configEModeCategories`
`:64-71`, `:118-191`

Same keep-current merging pattern as `CollateralEngine`, extended to `label` (compared by
`keccak256(abi.encode(...))`, `:128-129`) and `isolated`. Guards
`require(cfg.liquidationThreshold != 0, 'INVALID_UPDATE')` (`:142`) so updates cannot create
categories. Enforces `INVALID_LT_LB_RATIO` (`:174-177`).

#### `executeAssetsEModeUpdate` / `_configAssetsEMode`
`:17-24`, `:73-116`

Per asset: verify the category exists (`:79-82`), then conditionally
`setAssetCollateralInEMode` (`:83-89`), `setAssetBorrowableInEMode` (`:90-96`) and
**`setAssetLtvzeroInEMode`** (`:97-114`) **[3.7]**.

The `ltvzero` branch has a special case worth noting (`:98-107`): disabling an *already
disabled* asset would revert in the configurator, so the engine reads
`getEModeCategoryLtvzeroBitmap` and `continue`s if the bit is already clear. The comment
(`:99-101`) explains this exists so a payload can write `DISABLED` explicitly in an initial
setup PR rather than being forced to use `KEEP_CURRENT`.

---

<a name="6-helpers-and-data-providers"></a>

## 6. Helpers and data providers

Seven contracts. None of them is called by the protocol — they are read-only lenses plus one
gateway and one calldata packer. Front-ends, liquidation bots and indexers live here.

### 6.1 `AaveProtocolDataProvider`

`src/contracts/helpers/AaveProtocolDataProvider.sol` (288 lines).
`contract AaveProtocolDataProvider is IPoolDataProvider`.

The canonical read-only view of a market. Registered on the addresses provider
(`setPoolDataProvider`) so integrators can find it from the provider alone.

**State:** `address constant MKR = 0x9f8F…79A2` (`:25`),
`address constant ETH = 0xEeee…EEeE` (`:26`),
`IPoolAddressesProvider public immutable ADDRESSES_PROVIDER` (`:29`),
`IPool public immutable POOL` (`:32`).

**`constructor(IPoolAddressesProvider addressesProvider)`** — `:38-48`.
`require(pool != address(0), Errors.ZeroAddressNotValid())`. The comment at `:44-46`
explains why `POOL` is a plain immutable rather than read live: there is a circular
reference between the provider and the pool, and the pool address never changes in practice.

| Function | Line | Returns / notes |
|---|---:|---|
| `getAllReservesTokens()` | 51-71 | `TokenData[]`. **Hardcodes `'MKR'` and `'ETH'`** (`:55-63`) because those two tokens return `bytes32` from `symbol()` and would revert a `string` decode |
| `getAllATokens()` | 72-85 | `TokenData[]` of aToken symbols |
| `getReserveConfigurationData(address)` | 86-118 | 10 values. `stableBorrowRateEnabled` is **hardcoded `false`** (`:111`, deprecated 3.2). `usageAsCollateralEnabled = liquidationThreshold != 0` (`:117`) |
| `getReserveCaps(address)` | 119-125 | `(borrowCap, supplyCap)` |
| `getPaused(address)` | 126-129 | |
| `getSiloedBorrowing(address)` | 131-133 | **`pure`, returns `false`** — **[3.7]** hardcoded for backward compatibility |
| `getLiquidationProtocolFee(address)` | 136-139 | |
| `getUnbackedMintCap(address)` | 141-143 | **`pure`, returns 0** — deprecated |
| `getDebtCeiling(address)` | 146-148 | **`pure`, returns 0** — **[3.7]** isolation mode removed |
| `getDebtCeilingDecimals()` | 151-153 | **`pure`, returns 2** — **[3.7]** kept so old integrations do not divide by zero |
| `getReserveData(address)` | 156-193 | 12 values, five of them permanently zero: `unbacked` (3.4), and four stable-debt fields (3.2). See below |
| `getATokenTotalSupply(address)` | 197-200 | |
| `getTotalDebt(address)` | 203-206 | |
| `getUserReserveData(address,address)` | 209-241 | 9 values; four stable-debt ones zeroed at `:236`. `usageAsCollateralEnabled` from the user config bitmap (`:240`) |
| `getReserveTokensAddresses(address)` | 244-259 | `(aToken, address(0), variableDebtToken)` — the middle slot was the stable debt token |
| `getInterestRateStrategyAddress(address)` | 261-266 | **Ignores its argument**, returns `POOL.RESERVE_INTEREST_RATE_STRATEGY()` — one strategy per pool since 3.2 |
| `getFlashLoanEnabled(address)` | 268-273 | |
| `getIsVirtualAccActive(address)` | 275-277 | **`pure`, returns `true`** — virtual accounting is now unconditional |
| `getVirtualUnderlyingBalance(address)` | 280-282 | |
| `getReserveDeficit(address)` | 285-287 | Bad debt recorded against the reserve (3.3+) |

**Reading `getReserveData` correctly.** The tuple keeps its v2-era shape, so positions 0, 3,
7 and 8 are permanent zeros (`:180-192`). New integrations should read
`POOL.getReserveData(asset)` directly; this exists so a decade of deployed integrations keep
compiling.

**A subtlety in `totalAToken`.** It returns `IERC20Metadata(aToken).totalSupply()` (`:182`),
the *rebased* supply, while `accruedToTreasuryScaled` (`:181`) is **scaled**. Mixing them
without multiplying the latter by the liquidity index is a classic integration bug.

### 6.2 `UiPoolDataProviderV3`

`src/contracts/helpers/UiPoolDataProviderV3.sol` (264 lines).

One call returns everything a market page needs. Not gas-bounded — `eth_call` only.

**State:** `AggregatorInterface public immutable networkBaseTokenPriceInUsdProxyAggregator`
(`:25`), `marketReferenceCurrencyPriceInUsdProxyAggregator` (`:26`),
`uint256 public constant ETH_CURRENCY_UNIT = 1 ether` (`:27`),
`address public constant MKR_ADDRESS` (`:28`). The two aggregators exist so a UI can show
USD values even when the market's base currency is ETH.

| Function | Line | Purpose |
|---|---:|---|
| `getReservesList(provider)` | 38-43 | Passthrough |
| `getReservesData(provider)` | 45-173 | `(AggregatedReserveData[], BaseCurrencyInfo)` — the whole market |
| `getEModes(provider)` | 175-217 | Every eMode category |
| `getUserReservesData(provider, user)` | 219-251 | `(UserReserveData[], uint8 eModeId)` |
| `bytes32ToString(bytes32)` | 253-263 | `public pure`, for MKR's `bytes32` symbol |

**`AggregatedReserveData`** (`interfaces/IUiPoolDataProviderV3.sol:8-55`) — 40 fields,
explicitly versioned in the source comments: the base block, then `// v3 only` (`:39`),
`// v3.1 virtualUnderlyingBalance` (`:51-52`), `// v3.3 deficit` (`:53-54`).
**[3.7]** `isSiloedBorrowing` and `debtCeiling`/`debtCeilingDecimals` are now hardcoded to
`false`/`0`.

**`getEModes` is heuristic, and says so.** It scans ids 1..255 and **breaks after 3
consecutive misses** (`:210-211`, comment: *"assumes there will never be a gap > 2 when
setting eModes"*). A market with categories at 1, 2 and 9 would silently report only the
first two. It also wraps `getEModeCategoryLtvzeroBitmap` and `getIsEModeCategoryIsolated`
in `try/catch` (`:184-190`) so the same provider bytecode works against pre-3.7 pools that
lack those functions.

**`getUserReservesData`** reads *scaled* balances (`:238-240`, `:246-248`) and only fetches
debt when the user config bitmap says they are borrowing (`:244`), saving a call per
non-borrowed reserve. Passing `user = address(0)` returns an empty array (`:231-233`).

### 6.3 `UiIncentiveDataProviderV3`

`src/contracts/helpers/UiIncentiveDataProviderV3.sol` (294 lines).

| Function | Line |
|---|---:|
| `getFullReservesIncentiveData(provider, user)` | 17-27 |
| `getReservesIncentivesData(provider)` | 29-33 |
| `_getReservesIncentivesData(provider)` | 35-160 |
| `getUserReservesIncentivesData(provider, user)` | 162-167 |
| `_getUserReservesIncentivesData(provider, user)` | 169-… |

For each reserve it reads the incentives controller **off the token itself**
(`IncentivizedERC20(...).getIncentivesController()`, `:66-68` for aTokens and `:104-106`
for vTokens) rather than from the addresses provider — so a token wired to a different
controller is still reported correctly. Then per reward it collects `getRewardsData`,
`getAssetDecimals` (as `precision`), the reward's decimals, symbol, and the price feed's
`latestAnswer`/`decimals`.

**[3.7] bug fix recorded in the changelog:** vToken incentive *user* data previously used
`aTokenIncentiveController` instead of `vTokenIncentiveController`. Fixed in this tree.

### 6.4 `LiquidationDataProvider`

`src/contracts/helpers/LiquidationDataProvider.sol` (445 lines).

A read-only mirror of `LiquidationLogic`, so a bot can size a liquidation exactly before
sending it. Its value depends entirely on staying bit-identical to the core; **[3.7]**
aligned its rounding to match (`percentMulFloor`, `percentDivFloor`, `percentMulCeil`,
`MathUtils.mulDivCeil`).

**Structs** (`interfaces/ILiquidationDataProvider.sol`): `UserPositionFullInfo` (`:10-17`),
`CollateralFullInfo` (`:19-25`), `DebtFullInfo` (`:27-33`), `LiquidationInfo` (`:35-43`),
plus two local-var structs (`:45-51`, `:53-62`).

| Function | Line | Purpose |
|---|---:|---|
| `getUserPositionFullInfo(user)` | 49-64 | Wraps `POOL.getUserAccountData` |
| `getCollateralFullInfo(user, asset)` | 66-72 | aToken, balance, base-currency value, price, unit |
| `getDebtFullInfo(user, asset)` | 74-80 | Same for debt |
| `getLiquidationInfo(user, collateral, debt)` | 82-88 | Delegates with `debtLiquidationAmount = type(uint256).max` |
| `getLiquidationInfo(user, collateral, debt, amount)` | 91-186 | The real one |
| `_adjustAmountsForGoodLeftovers(...)` | 192-278 | Enforces the no-dust rule |
| `_getAvailableCollateralAndDebtToLiquidate(...)` | 280-320 | Bonus + protocol fee |
| `_getMaxDebtToLiquidate(...)` | 322-348 | Close factor |
| `_getLiquidationBonus(...)` | 350-374 | eMode-aware |
| `_isCollateralEnabledForUser(...)` | 376-383 | |
| `_canLiquidateThisHealthFactor(uint256)` | 385-387 | `private pure`. **[3.7]** sentinel logic removed |
| `_isReserveReadyForLiquidations(...)` | 389-400 | active, not paused |
| `_getCollateralFullInfo(...)` / `_getDebtFullInfo(...)` | 402-421, 423-… | |

`getLiquidationInfo` returns an all-zero `LiquidationInfo` (rather than reverting) on every
"not liquidatable" path — no debt (`:108-110`), healthy factor (`:112-114`), reserve not
ready (`:117-129`), collateral not enabled (`:131-133`). A bot treats zeros as "skip".

The last step is the useful one (`:174-184`): `amountToPassToLiquidationCall` is set to
`type(uint256).max` when the computation shows the whole debt or the whole collateral is
being consumed, and to the exact figure otherwise. Passing the sentinel is what lets the
core close the position cleanly without leaving dust.

### 6.5 `WrappedTokenGatewayV3`

`src/contracts/helpers/WrappedTokenGatewayV3.sol` (204 lines).
`contract WrappedTokenGatewayV3 is IWrappedTokenGatewayV3, Ownable`.

Wraps and unwraps native ETH around a WETH reserve. **Immutables:** `IWETH public immutable WETH`
(`:24`), `IPool public immutable POOL` (`:25`).

**`constructor(address weth, address owner, IPool pool)`** — `:32-37`. Transfers ownership
and grants the pool an infinite WETH allowance (`:36`).

**Every function's first parameter is an unnamed `address`** — the legacy `pool` argument,
kept for ABI compatibility and ignored.

| Function | Line | Flow |
|---|---:|---|
| `depositETH(address, address onBehalfOf, uint16 referralCode)` | 45-48 | `payable`. `WETH.deposit{value: msg.value}()` → `POOL.deposit(WETH, msg.value, onBehalfOf, code)` |
| `withdrawETH(address, uint256 amount, address to)` | 55-70 | `amount == max` → user's full aWETH balance (`:61-63`); `aWETH.transferFrom(msg.sender, this, amt)`; `POOL.withdraw`; `WETH.withdraw`; `_safeTransferETH` |
| `repayETH(address, uint256 amount, address onBehalfOf)` | 77-96 | `payable`. Clamps to the actual debt (`:82-84`); `require(msg.value >= paybackAmount, 'msg.value is less than repayment amount')` (`:85`); wraps, repays, **refunds the dust** (`:95`) |
| `borrowETH(address, uint256 amount, uint16 referralCode)` | 103-113 | `POOL.borrow(WETH, amount, VARIABLE, code, msg.sender)` then unwrap and send. **Requires the user to have called `approveDelegation` on the WETH debt token first** |
| `withdrawETHWithPermit(address, uint256 amount, address to, uint256 deadline, uint8 v, bytes32 r, bytes32 s)` | 124-151 | Same as `withdrawETH` with a `try/catch`-wrapped permit (`:142-144`). Permits `amount`, not `amountToWithdraw` — deliberate, per the comment at `:141` |
| `emergencyTokenTransfer(address,address,uint256)` | 170-172 | `onlyOwner` |
| `emergencyEtherTransfer(address,uint256)` | 180-182 | `onlyOwner` |
| `getWETHAddress()` | 187-189 | |
| `receive()` | 194-196 | `require(msg.sender == address(WETH), 'Receive not allowed')` |
| `fallback()` | 201-203 | `revert('Fallback not allowed')` |

**`_safeTransferETH(address to, uint256 value)`** — `:158-161`, `internal`. Raw `call` with
all remaining gas, `require(success, 'ETH_TRANSFER_FAILED')`. Forwarding all gas (not 2300)
means the recipient can run arbitrary code; the gateway holds nothing between calls, so
there is nothing to reenter for.

**The permanent trap:** `withdrawETH` requires the user to `approve` the *gateway* on their
aWETH first. Users who send aWETH directly to the contract instead lose it — which is what
`emergencyTokenTransfer` exists to undo.

### 6.6 `WalletBalanceProvider`

`src/contracts/helpers/WalletBalanceProvider.sol` (100 lines). The header states plainly:
**"THIS CONTRACT IS NOT USED WITHIN THE AAVE PROTOCOL"** (`:17-18`). It exists to cut RPC
round-trips for the backend.

`address constant MOCK_ETH_ADDRESS = 0xEeee…EEeE` (`:26`).

| Function | Line | Notes |
|---|---:|---|
| `balanceOf(address user, address token)` | 34-42 | Mock address → `user.balance`; a contract → `IERC20.balanceOf`; an EOA → `revert('INVALID_TOKEN')` |
| `batchBalanceOf(address[] users, address[] tokens)` | 50-63 | Flattened `users × tokens` grid, row-major |
| `getUserWalletBalances(address provider, address user)` | 68-99 | Every reserve plus ETH appended; **inactive reserves report 0** rather than the real balance (`:88-93`) |

### 6.7 Helper interfaces

| File | Lines | Contents |
|---|---:|---|
| `interfaces/ILiquidationDataProvider.sol` | 122 | 6 structs + the provider surface |
| `interfaces/IUiPoolDataProviderV3.sol` | 95 | `AggregatedReserveData` (40 fields), `UserReserveData`, `BaseCurrencyInfo`, `Emode` |
| `interfaces/IUiIncentiveDataProviderV3.sol` | 73 | `AggregatedReserveIncentiveData`, `IncentiveData`, `RewardInfo`, and the user variants |
| `interfaces/IWrappedTokenGatewayV3.sol` | 28 | The gateway surface |
| `interfaces/IWETH.sol` | 12 | `deposit()`, `withdraw(uint256)`, `approve`, `transferFrom` |
| `interfaces/IERC20DetailedBytes.sol` | 12 | `name()`/`symbol()` returning `bytes32` — for MKR |

### 6.8 `L2Encoder`

`src/contracts/helpers/L2Encoder.sol` (296 lines).

The **encode** half of the L2 calldata compression scheme; `CalldataLogic` in the protocol
is the decode half. On an optimistic rollup, L1 calldata bytes dominate transaction cost, so
`L2Pool` accepts a packed `bytes32` instead of an ABI-encoded argument list.

**State:** `IPool public immutable POOL` (`:16`). Every function is `view` because it must
resolve `asset → reserve.id` through `POOL.getReserveData(asset)`.

**The packing scheme.** Fields are laid out little-endian by bit offset via
`add(x, shl(offset, y))`:

```
encodeSupplyParams                                        :46-48
 bits   0..15    assetId        (uint16)
 bits  16..143   amount         (uint128)
 bits 144..159   referralCode   (uint16)
 -> 1 word replaces (address,uint256,address,uint16) = 4 words

encodeSupplyWithPermitParams                              :83-91   -> 3 words
 bits   0..15    assetId
 bits  16..143   amount
 bits 144..159   referralCode
 bits 160..191   deadline       (uint32)
 bits 192..199   permitV        (uint8)
 + permitR, permitS returned as separate words (unpackable)

encodeWithdrawParams                                      :110-112
 bits   0..15    assetId
 bits  16..143   amount         (uint128, max -> uint128.max sentinel)

encodeBorrowParams                                        :138-146
 bits   0..15    assetId
 bits  16..143   amount
 bits 144..151   interestRateMode (uint8)
 bits 152..167   referralCode     (uint16)

encodeRepayParams                                         :171-173
 bits   0..15    assetId
 bits  16..143   amount           (max -> uint128.max)
 bits 144..151   interestRateMode

encodeRepayWithPermitParams                               :209-219  -> 3 words
 bits   0..15    assetId
 bits  16..143   amount
 bits 144..151   interestRateMode
 bits 152..183   deadline
 bits 184..191   permitV

encodeSetUserUseReserveAsCollateral                       :253-255
 bits   0..15    assetId
 bit    16       useAsCollateral (bool)

encodeLiquidationCall                                     :290-293  -> 2 words
 word1: bits 0..15 collateralAssetId | 16..31 debtAssetId | 32..191 user (address)
 word2: bits 0..127 debtToCover (uint128, max -> uint128.max) | bit 128 receiveAToken
```

**The `uint256.max` sentinel translation.** `withdraw`, `repay` and `liquidationCall` accept
`type(uint256).max` meaning "everything". A `uint128` field cannot hold it, so the encoder
maps it to `type(uint128).max` (`:107`, `:167`, `:204`, `:283-285`) and `CalldataLogic`
expands it back. **Consequence:** a genuine amount of exactly `type(uint128).max` wei is
indistinguishable from "max" — irrelevant at real token scales, but it is a real edge.

**No `onBehalfOf`.** Every doc comment says so (`:28`, `:54`, `:98`, …): the L2 compact
calls always use `msg.sender`. Acting on behalf of another address requires the full
uncompressed `Pool` entrypoint.

**Overflow behaviour.** `SafeCast` `.toUint128()` / `.toUint32()` / `.toUint8()` revert on
truncation, so a too-large amount fails at encode time rather than silently wrapping. But
the packing itself uses `add`, not `or` — safe only because `SafeCast` guarantees the fields
do not overlap.

`encodeRepayWithATokensParams` (`:232-238`) simply forwards to `encodeRepayParams` — same
layout, different `L2Pool` selector.

---

<a name="7-treasury-collector"></a>

## 7. Treasury: `Collector`

`src/contracts/treasury/Collector.sol` (339 lines).
`contract Collector is AccessControlUpgradeable, ReentrancyGuardUpgradeable, ICollector`.

Where `mintToTreasury` sends the reserve-factor cut. It holds aTokens and can pay them out
directly or as Sablier-style linear streams (grants, contributor comp, service retainers).

**Constants:** `ETH_MOCK_ADDRESS = 0xEeee…EEeE` (`:30`),
`bytes32 public constant FUNDS_ADMIN_ROLE = 'FUNDS_ADMIN'` (`:33`).

**Storage, and a genuinely nasty history** (`:35-53`):

```solidity
// Reserved storage space to account for deprecated inherited storage
// 0 was lastInitializedRevision
// 1-50 were the ____gap
// 51 was the reentrancy guard _status
// 52 was the _fundsAdmin
// On some networks the layout was shifted by 1 due to `initializing` being on slot 1
uint256[53] private ______gap;
uint256 private _nextStreamId;
mapping(uint256 => Stream) private _streams;
```

The comment at `:41-42` is the important one: **the same contract had different storage
layouts on different chains**, and realigning them required a manual shift in an upgrade
proposal. This is the canonical example of why storage gaps and `VersionedInitializable`
exist.

**Modifiers:** `onlyFundsAdmin()` (`:59-64`, `OnlyFundsAdmin()`),
`onlyAdminOrRecipient(uint256 streamId)` (`:70-76`, `OnlyFundsAdminOrRecipient()`),
`streamExists(uint256 streamId)` (`:80-83`, `StreamDoesNotExist()`).

**`constructor()`** — `:85-87`, `_disableInitializers()`.

#### `initialize(uint256 nextStreamId, address admin)`
`external virtual initializer` — `:95-105`

`__AccessControl_init`, `__ReentrancyGuard_init`, grants `DEFAULT_ADMIN_ROLE` and
`FUNDS_ADMIN_ROLE` to `admin`, and sets `_nextStreamId` only if non-zero (`:102-104`) — so
an upgrade can preserve the existing counter.

#### `deltaOf(uint256 streamId) → uint256`
`public view streamExists` — `:152-157`. Elapsed streaming seconds, clamped to
`[0, stopTime − startTime]`.

#### `balanceOf(uint256 streamId, address who) → uint256`
`public view streamExists` — `:166-192`

```
recipientBalance = delta * ratePerSecond
if (deposit > remainingBalance)                       // there have been withdrawals
    recipientBalance -= (deposit - remainingBalance)
who == recipient -> recipientBalance
who == sender    -> remainingBalance - recipientBalance
otherwise        -> 0
```

#### `approve(IERC20 token, address recipient, uint256 amount)` — `:197-199`, `onlyFundsAdmin`, `forceApprove`.
#### `transfer(IERC20 token, address recipient, uint256 amount)` — `:202-…`, `onlyFundsAdmin`.

#### `createStream(address recipient, uint256 deposit, address tokenAddress, uint256 startTime, uint256 stopTime) → uint256 streamId`
`external onlyFundsAdmin` — `:235-287`

Six input checks (`:241-246`), then two arithmetic ones:

| Check | Line | Error |
|---|---:|---|
| `recipient != address(0)` | 241 | `InvalidZeroAddress()` |
| `recipient != address(this)` | 242 | `InvalidRecipient()` |
| `recipient != msg.sender` | 243 | `InvalidRecipient()` |
| `deposit != 0` | 244 | `InvalidZeroAmount()` |
| `startTime >= block.timestamp` | 245 | `InvalidStartTime()` |
| `stopTime > startTime` | 246 | `InvalidStopTime()` |
| `deposit >= duration` | 252 | `DepositSmallerTimeDelta()` — otherwise `ratePerSecond` would floor to 0 |
| `deposit % duration == 0` | 255 | `DepositNotMultipleTimeDelta()` — "avoids dealing with remainders" |

`ratePerSecond = deposit / duration` (`:257`) is then exact. The stream is stored with
`sender: address(this)` (`:266`) — the Collector always streams its own funds. Emits
`CreateStream`.

**Note:** nothing checks that the Collector actually *holds* `deposit` of `tokenAddress`.
An over-committed stream simply fails at withdrawal time.

#### `withdrawFromStream(uint256 streamId, uint256 amount) → bool`
`external nonReentrant streamExists onlyAdminOrRecipient` — `:296-319`

`amount != 0` → `InvalidZeroAmount()`; `balance >= amount` → else `BalanceExceeded()`;
decrement `remainingBalance`; **`delete _streams[streamId]` when it hits zero** (`:314`);
`safeTransfer`; emit. Deleting frees the slot and gas-refunds, but also makes the stream id
permanently unresolvable — an indexer must capture the event.

#### `cancelStream(uint256 streamId) → bool`
`external nonReentrant streamExists onlyAdminOrRecipient` — `:321-…`

Computes both balances, `delete`s the stream **first**, then pays the recipient what they
had earned. The sender's share simply stays in the Collector (it never left). Emits
`CancelStream`. Checks-effects-interactions plus `nonReentrant`.

#### `receive() external payable {}` — the last line. The comment says it is needed to
receive ETH from the **Aave v1 ecosystem reserve**.

`ICollector.sol` (216 lines) declares the `Stream` struct, 12 custom errors and the three
stream events.

---

<a name="8-instances"></a>

## 8. `instances/` — the deployable concrete contracts

`src/contracts/protocol/` holds *abstract* or *base* contracts with no `getRevision()`.
`instances/` supplies the missing piece: a thin subclass that pins a revision constant and
an `initialize`. **The revision is the upgrade gate** — `VersionedInitializable` only lets
`initialize` run if `getRevision() > lastInitializedRevision` (§9), so bumping this number
is what makes an upgrade re-initialisable.

| Contract | File | Revision | Line of constant |
|---|---|---:|---:|
| `ATokenInstance` | `ATokenInstance.sol` | 5 | `:19` |
| `ATokenWithDelegationInstance` | `ATokenWithDelegationInstance.sol` | 5 | `:19` |
| `VariableDebtTokenInstance` | `VariableDebtTokenInstance.sol` | 5 | `:13` |
| `VariableDebtTokenMainnetInstanceGHO` | `VariableDebtTokenMainnetInstanceGHO.sol` | 6 | `:13` |
| `PoolInstance` | `PoolInstance.sol` | **11** | `:15` |
| `L2PoolInstance` | `L2PoolInstance.sol` | 11 (inherited) | — |
| `PoolConfiguratorInstance` | `PoolConfiguratorInstance.sol` | **8** | `:12` |

- **`ATokenInstance`** (61) — `constructor(IPool, address rewardsController, address treasury)` (`:21`),
  `getRevision()` (`:28-30`), `initialize(...)` (`:33`) with the full token metadata.
- **`ATokenWithDelegationInstance`** (61) — identical shape over `ATokenWithDelegation`. Used
  only for the AAVE reserve, where aAAVE must carry governance power.
- **`VariableDebtTokenInstance`** (50) — `constructor(IPool pool, address rewardsController)` (`:15`).
- **`VariableDebtTokenMainnetInstanceGHO`** (53) — revision **6**, one higher than the
  standard vToken, so the GHO reserve can be upgraded independently. Its only extra member
  is a **no-op**:
  ```solidity
  // @note deprecated discount hook being called by stkAAVE, not used since v3.4
  function updateDiscountDistribution(address, address, uint256, uint256, uint256) external {}
  ```
  (`:52`). stkAAVE still calls this selector; without the stub the call would revert and
  break staking. A deliberate tombstone.
- **`PoolInstance`** (36) — `initialize(IPoolAddressesProvider provider) external virtual initializer`
  (`:29`), `getRevision()` (`:33-35`). The comment at `:24-25` notes the proxy invokes it.
- **`L2PoolInstance`** (19) — `contract L2PoolInstance is L2Pool, PoolInstance` (`:14`).
  Pure multiple inheritance: `L2Pool` adds the compact-calldata entrypoints, `PoolInstance`
  supplies the revision.
- **`PoolConfiguratorInstance`** (23) — `initialize(IPoolAddressesProvider provider) public virtual override initializer`
  (`:19`).

**Reading the revisions as history:** the pool is on its 11th implementation, the
configurator its 8th, the tokens their 5th. **[3.7]** bumped pool 10 → 11 and configurator
7 → 8.

---

<a name="9-upgradeability"></a>

## 9. Upgradeability: proxies and `VersionedInitializable`

### 9.1 `VersionedInitializable`

`src/contracts/misc/aave-upgradeability/VersionedInitializable.sol` (86 lines). `abstract`.

**State:** `uint256 private lastInitializedRevision = 0` (`:29`), `bool private initializing` (`:34`),
`uint256[50] private ______gap` (`:85`).

**`constructor()`** — `:21-24`:

```solidity
constructor() {
  // break the initialize
  lastInitializedRevision = getRevision();
}
```

Sets the implementation's *own* storage so that calling `initialize` directly on the
implementation can never succeed. The proxy's storage is untouched by this, since the
constructor runs in the implementation's context.

**`modifier initializer()`** — `:39-57`:

```solidity
uint256 revision = getRevision();
require(initializing || isConstructor() || revision > lastInitializedRevision,
        'Contract instance has already been initialized');
bool isTopLevelCall = !initializing;
if (isTopLevelCall) { initializing = true; lastInitializedRevision = revision; }
_;
if (isTopLevelCall) { initializing = false; }
```

The `revision > lastInitializedRevision` clause is what makes this different from OZ's
`Initializable`: an upgrade with a **strictly higher** revision may re-run `initialize`,
which is how new storage fields get populated on an existing proxy. Re-deploying the *same*
revision cannot.

- **`getRevision() → uint256`** — `:64`, `internal pure virtual`. Supplied by the instances.
- **`isConstructor() → bool`** — `:70-82`, `private view`. `extcodesize(address()) == 0`.

**Upgrade recipe:** change the implementation, bump the revision constant in the instance,
deploy, then `upgradeToAndCall(newImpl, abi.encodeCall(initialize, ...))`.

### 9.2 `BaseImmutableAdminUpgradeabilityProxy`

`src/contracts/misc/aave-upgradeability/BaseImmutableAdminUpgradeabilityProxy.sol` (85 lines).
`contract BaseImmutableAdminUpgradeabilityProxy is BaseUpgradeabilityProxy`.

**State:** `address internal immutable _admin` (`:103` in the concatenated read; the file's
own line 22). The admin lives in **bytecode**, not the EIP-1967 admin slot — one fewer
`SLOAD` on every single call through the proxy. The trade-off is that the admin can never be
changed; in Aave it is always the `PoolAddressesProvider`, which is itself `Ownable`, so
admin rotation happens one level up.

**`modifier ifAdmin()`** — the transparent-proxy pattern:

```solidity
modifier ifAdmin() {
  if (msg.sender == _admin) { _; } else { _fallback(); }
}
```

Non-admins calling `admin()` or `implementation()` are silently forwarded to the
implementation instead of getting the proxy's answer. That is what stops a selector clash
between proxy and implementation from being exploitable.

| Function | Access | Behaviour |
|---|---|---|
| `admin() → address` | `ifAdmin` | Returns `_admin` |
| `implementation() → address` | `ifAdmin` | Returns `_implementation()` |
| `upgradeTo(address newImplementation)` | `ifAdmin` | `_upgradeTo` |
| `upgradeToAndCall(address newImplementation, bytes data)` | `ifAdmin payable` | `_upgradeTo` then `delegatecall(data)`, `require(success)` |
| `_willFallback()` | `internal virtual override` | `require(msg.sender != _admin, 'Cannot call fallback function from the proxy admin')` |

**The bare `require(success)`** in `upgradeToAndCall` swallows the revert reason from the
initializer — a failed upgrade gives no diagnostic.

### 9.3 `InitializableImmutableAdminUpgradeabilityProxy`

Same directory, 29 lines.
`contract InitializableImmutableAdminUpgradeabilityProxy is BaseImmutableAdminUpgradeabilityProxy, InitializableUpgradeabilityProxy`.

`constructor(address admin)` (`:22`) forwards to the base. `_willFallback()` (`:26-28`)
resolves the `override(BaseImmutableAdminUpgradeabilityProxy, Proxy)` diamond explicitly in
favour of the admin-guard version.

**This is the proxy Aave actually deploys** for aTokens, debt tokens, the pool, the
configurator and the rewards controller — `ConfiguratorLogic` instantiates it during
`initReserve`.

### 9.4 `EmptyImplementation`

`src/contracts/misc/EmptyImplementation.sol` (8 lines). `contract EmptyImplementation {}`.

A proxy pointed here accepts calls and does nothing. Used for the **dust bin**
(`AaveV3TreasuryProcedure.sol:35-42`) — an address that must exist and be upgradeable later
but has no behaviour today. Introduced in **[3.4]** to separate listing dust from real
treasury income.

### 9.5 Vendored OpenZeppelin proxy chain

```
Proxy (81)                                   dependencies/openzeppelin/upgradeability/
 |  fallback() -> _delegate(_implementation())
 |  _willFallback() hook
 +- BaseUpgradeabilityProxy (66)
 |    IMPLEMENTATION_SLOT = keccak256('eip1967.proxy.implementation') - 1
 |    _implementation() / _upgradeTo() / _setImplementation()
 |    +- UpgradeabilityProxy (28)              constructor(logic, data)
 |    +- InitializableUpgradeabilityProxy (29) initialize(logic, data)
 |    +- BaseAdminUpgradeabilityProxy (125)    admin in the EIP-1967 ADMIN_SLOT
 |         +- AdminUpgradeabilityProxy (36)
 |         +- InitializableAdminUpgradeabilityProxy (38)
```

`BaseAdminUpgradeabilityProxy` is the storage-slot variant Aave *does not* use for its own
proxies; it appears because `PoolAddressesProvider` and some deployment paths reference it.
It carries `changeAdmin` with `require(newAdmin != address(0), 'Cannot change the admin of a proxy to the zero address')`.

---

<a name="10-flash-loan-base-contracts"></a>

## 10. Flash loan base contracts

Four files in `src/contracts/misc/flashloan/`. Nothing here is deployed by Aave — they are
the contracts *you* inherit when writing a flash-loan receiver.

### `IFlashLoanReceiver` — `interfaces/IFlashLoanReceiver.sol` (36 lines)

```solidity
function executeOperation(
  address[] calldata assets,
  uint256[] calldata amounts,
  uint256[] calldata premiums,
  address initiator,
  bytes calldata params
) external returns (bool);
```
(`:25-32`) plus `ADDRESSES_PROVIDER()` (`:33`) and `POOL()` (`:35`).

### `IFlashLoanSimpleReceiver` — `interfaces/IFlashLoanSimpleReceiver.sol` (36 lines)

Same shape with scalars: `executeOperation(address asset, uint256 amount, uint256 premium, address initiator, bytes params)` (`:25-32`).

### `FlashLoanReceiverBase` / `FlashLoanSimpleReceiverBase` — `base/` (21 lines each)

Identical bodies:

```solidity
IPoolAddressesProvider public immutable override ADDRESSES_PROVIDER;
IPool public immutable override POOL;
constructor(IPoolAddressesProvider provider) {
  ADDRESSES_PROVIDER = provider;
  POOL = IPool(provider.getPool());
}
```

**The four rules of writing a receiver**, none of which the base enforces:

1. **Return `true`.** `FlashLoanLogic` checks the return value; `false` reverts the loan.
2. **Approve the `POOL`, not the aToken.** The pull is
   `safeTransferFrom(receiver, aToken, amount + premium)` executed *by the Pool's
   delegatecalled library*, so `msg.sender` on the token is the Pool.
   `AAVE-DEEP-DIVE.md` §3.9 walks the exact call.
3. **Gate `executeOperation` on `msg.sender == address(POOL)`.** The base does *not* do this.
   A receiver without the check can be invoked by anyone with fabricated `premiums`, and
   whatever the receiver does with the "borrowed" funds happens with no loan behind it. This
   is the single most common flash-loan integration bug.
4. **Check `initiator`** if the receiver holds funds of its own. `initiator` is whoever
   called `Pool.flashLoan`, and it is the only evidence of who actually asked.

`POOL` is cached at construction, so a receiver survives a pool *implementation* upgrade
(the proxy address is stable) but not a pool *address* change.

---

<a name="11-vendored-dependencies"></a>

## 11. Vendored dependencies

18 files. Aave vendors rather than imports these so that a specific, audited version is
pinned into the build.

### `GPv2SafeERC20` — `dependencies/gnosis/contracts/GPv2SafeERC20.sol` (115 lines)

Gnosis Protocol's assembly-optimised `SafeERC20`. Used by every contract in this scope that
moves tokens: the transfer strategies, the gateway, `WalletBalanceProvider`.

`safeTransfer(IERC20 token, address to, uint256 value)` (`:12-29`) hand-builds the calldata
at the free-memory pointer, `call`s, bubbles the revert with
`returndatacopy`/`revert`, then `require(getLastTransferResult(token), 'GPv2: failed transfer')`.
`safeTransferFrom` (`:33-…`) is the same with `'GPv2: failed transferFrom'`.

`getLastTransferResult` is the interesting part: it inspects `returndatasize()` and treats
**0 bytes as success** (USDT and friends return nothing) and 32 bytes as a boolean. Anything
else fails. Cheaper than OZ's `SafeERC20` because it never allocates memory through Solidity.

### `AggregatorInterface` — `dependencies/chainlink/AggregatorInterface.sol` (49 lines)

`decimals()`, `description()`, `getRoundData(uint80)`, `latestRoundData()`, `latestAnswer()`,
`latestTimestamp()`, `latestRound()`, `getAnswer(uint256)`, `getTimestamp(uint256)`,
`aggregator()`, plus `AnswerUpdated` (`:46`) and `NewRound` (`:48`).

Aave calls exactly two of these: `latestAnswer()` (`AaveOracle.sol:109`,
`RewardsController.sol:351`, `PriceFeedEngine.sol:24`) and `decimals()`
(`PriceFeedEngine.sol:28`). `latestRoundData` — the one that carries `updatedAt` — is
declared and never used, which is the staleness gap noted in §3.

### `WETH9` — `dependencies/weth/WETH9.sol` (754 lines)

The canonical 2015 WETH, vendored for local deployments and tests. Events `Approval`,
`Transfer`, `Deposit` (`:25`), `Withdrawal` (`:26`).

### OpenZeppelin, `dependencies/openzeppelin/`

| File | Lines | Used by |
|---|---:|---|
| `contracts/Address.sol` | 220 | `AaveV3Payload.functionDelegateCall`, `WalletBalanceProvider.isContract` |
| `contracts/AccessControl.sol` | 216 | `Collector` (via the upgradeable variant) |
| `contracts/Ownable.sol` | 69 | `EmissionManager`, `WrappedTokenGatewayV3` |
| `contracts/Strings.sol` | 67 | Error formatting |
| `contracts/Context.sol` | 23 | `_msgSender` |
| `contracts/ERC165.sol` / `IERC165.sol` | 28 / 24 | Interface detection |
| `contracts/IAccessControl.sol` | 91 | |
| `upgradeability/*` (7 files) | 403 | The proxy chain in §9.5 |

Note that the *modern* OZ contracts (`ERC4626Upgradeable`, `ERC20PermitUpgradeable`,
`SafeERC20`, `SafeCast`, `Math`) are **not** vendored — they come from the
`openzeppelin-contracts` / `openzeppelin-contracts-upgradeable` remappings in `lib/`. Only
the legacy 0.6-era contracts Aave forked live under `dependencies/`.

---

<a name="12-mocks"></a>

## 12. Mocks

22 files under `src/contracts/mocks/`, never deployed to production. Worth knowing because
they document intended behaviour and each one usually exists to pin a specific edge case.

| Mock | Lines | What it pins down |
|---|---:|---|
| `helpers/MockReserveConfiguration.sol` | 133 | Exposes every `ReserveConfiguration` getter/setter so the bitmap can be fuzzed directly |
| `upgradeability/MockInitializableImplementation.sol` | 128 | V1/V2/V3 implementations with ascending revisions — the `VersionedInitializable` gate |
| `testnet-helpers/TestnetERC20.sol` | 102 | Faucet-mintable ERC20 with permit |
| `tokens/MintableERC20.sol` | 96 | Open `mint()` |
| `flashloan/MockFlashLoanReceiver.sol` | 78 | Settable `setFailExecutionTransfer` / `setAmountToApprove`; emits `ExecutedWithSuccess` / `ExecutedWithFail` |
| `ATokenMock.sol` | 72 | Redeclares `AssetConfigUpdated` and `Accrued` locally (comment at `:12`: *"hack to be able to test event from Distribution manager properly"*) and calls `handleActionOnAic` |
| `flashloan/MockSimpleFlashLoanReceiver.sol` | 71 | Single-asset twin |
| `testnet-helpers/Faucet.sol` | 70 | Rate-limited minting |
| `tokens/MockAToken.sol` | 55 | Bumped revision, for upgrade tests |
| `tests/FlashloanAttacker.sol` | 52 | Attempts reentrancy through the flash-loan path |
| `helpers/MockPeripheryContract.sol` | 42 | `MockPeripheryContractV1`/`V2` for `setAddressAsProxy` tests |
| `tokens/MockScaledToken.sol` | 37 | Bare `IScaledBalanceToken` for reward accounting tests |
| `tokens/MintableDelegationERC20.sol` | 37 | Mintable + `delegate()` |
| `MockBadTransferStrategy.sol` | 33 | `performTransfer` returns `false` → exercises `'TRANSFER_ERROR'` |
| `testnet-helpers/IFaucet.sol` | 32 | |
| `oracle/PriceOracle.sol` | 32 | Settable prices; the **fallback oracle** in tests |
| `helpers/MockPool.sol` | 28 | `MockPoolInherited` with settable `MAX_NUMBER_RESERVES`, to test the reserve-count limit. **[3.7]** its `dropReserve` override was removed |
| `oracle/CLAggregators/MockAggregator.sol` | 25 | Fixed `latestAnswer` |
| `flashloan/MockFlashLoanReceiverWithoutMint.sol` | 23 | Cannot repay → the revert path |
| `helpers/MockL2Pool.sol` | 23 | `L2PoolInstance` with a bumped revision |
| `flashloan/MockSimpleFlashLoanReceiverWithoutMint.sol` | 22 | |
| `WETH9Mock.sol` / `tokens/WETH9Mocked.sol` | 20 / 19 | WETH with public `mint` |
| `tokens/MockDebtTokens.sol` | 16 | `MockVariableDebtToken`, bumped revision |
| `upgradeability/MockAToken.sol` / `MockVariableDebtToken.sol` | 17 / 16 | Upgrade targets |
| `helpers/MockIncentivesController.sol` | 8 | `handleAction` no-op — proves the protocol works with incentives disabled |
| `tests/MockReserveInterestRateStrategy.sol` | 26 | Hardcoded rates, so tests can pin utilization |

**[3.7]** `SequencerOracle` was deleted along with the price oracle sentinel.

---
