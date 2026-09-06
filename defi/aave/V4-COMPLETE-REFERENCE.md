# Aave V4 — Complete Reference

Every contract, every function, every custom error, every storage slot in
`aave/v4-aave/src`, excluding the vendored `src/dependencies/**` (listed in §13).

**119 in-scope Solidity files, 16,560 lines.** Every `path:line` below was checked
with `grep -n` against these exact files. Paths are relative to `aave/v4-aave/`.

Companion documents, which this one does not duplicate:

- [`aave/AAVE-V4-DEEP-DIVE.md`](AAVE-V4-DEEP-DIVE.md) — why hub-and-spoke, the accounting model explained conceptually, end-to-end traces with numbers.
- [`aave/AAVE-DEEP-DIVE.md`](AAVE-DEEP-DIVE.md) — Aave v3.6.
- [`aave/AAVE-V1-V2-DEEP-DIVE.md`](AAVE-V1-V2-DEEP-DIVE.md) — Aave v1 and v2.

---

## Contents

- [§0 Three claims, verified against the code](#0-three-claims-verified-against-the-code)
- [§1 Architecture in one page](#1-architecture-in-one-page)
- [§2 File inventory (all 119)](#2-file-inventory-all-119)
- [§3 Math libraries](#3-math-libraries)
- [§4 Hub layer](#4-hub-layer)
- [§5 Spoke layer](#5-spoke-layer)
- [§6 LiquidationLogic](#6-liquidationlogic)
- [§7 TokenizationSpoke and TreasurySpoke](#7-tokenizationspoke-and-treasuryspoke)
- [§8 Oracle and interest rate strategy](#8-oracle-and-interest-rate-strategy)
- [§9 Position managers](#9-position-managers)
- [§10 Access control](#10-access-control)
- [§11 Config engine and governance](#11-config-engine-and-governance)
- [§12 Deployments](#12-deployments)
- [§13 Utils, interfaces, dependencies](#13-utils-interfaces-dependencies)
- [§14 Storage layout tables](#14-storage-layout-tables)
- [§15 Selector / ABI tables](#15-selector--abi-tables)
- [§16 Complete custom-error table](#16-complete-custom-error-table)
- [§17 Complete events reference](#17-complete-events-reference)
- [§18 Use-case index](#18-use-case-index)
- [§19 v3 → v4 API migration table](#19-v3--v4-api-migration-table)
- [§20 Gas snapshots and audits](#20-gas-snapshots-and-audits)

---

## §0 Three claims, verified against the code

The deep-dive author reported three findings. All three are **confirmed**. Here is
the line that settles each.

**Claim 1 — supply uses ERC-4626-style shares against `totalAddedAssets()` with a
1e6 virtual offset; debt uses a ray `drawnIndex`; there is no `liquidityIndex`.**

Confirmed. `SharesMath.sol:13-14` declares the offsets:

```solidity
uint256 internal constant VIRTUAL_ASSETS = 1e6;
uint256 internal constant VIRTUAL_SHARES = 1e6;
```

and every supply-side conversion routes through them (`SharesMath.sol:17-62`).
The supply denominator is `AssetLogic.totalAddedAssets` (`AssetLogic.sol:79-97`),
called by `toAddedSharesDown/Up` and `toAddedAssetsDown/Up`
(`AssetLogic.sol:99-130`). The debt side is a separate ray index:
`AssetLogic.getDrawnIndex` (`:153-165`) and `toDrawnShares*/toDrawnAssets*`
(`:25-55`). `grep -rn 'liquidityIndex' src/` returns nothing.

**Claim 2 — `premiumShares = drawnShares × riskPremium` plus a signed offset chosen
so the shares contribute zero principal when set; each Spoke has a
premium-to-drawn ratio cap.**

Confirmed, and the mechanism is exactly as described.
`UserPositionUtils.calculatePremiumDelta` (`:54-81`) computes

```solidity
uint256 newPremiumShares = (userPosition.drawnShares - drawnSharesTaken).percentMulUp(riskPremium);
int256 newPremiumOffsetRay = (newPremiumShares * drawnIndex).signedSub(premiumDebtRay - restoredPremiumRay);
```

The offset is set to exactly `newPremiumShares × drawnIndex` minus the premium debt
that should remain, so `Premium.calculatePremiumRay` (`Premium.sol:17-24`), which
returns `premiumShares × drawnIndex − premiumOffsetRay`, yields the *unchanged*
premium at that instant. Only later index growth adds premium. The Hub enforces
value conservation on every premium edit in `_validateApplyPremiumDelta`
(`Hub.sol:933-959`):

```solidity
require(premiumRayAfter + premiumDelta.restoredPremiumRay == premiumRayBefore, InvalidPremiumChange());
```

and caps the ratio per Spoke in `_applyPremiumDelta` (`Hub.sol:757-762`):

```solidity
require(riskPremiumThreshold == MAX_RISK_PREMIUM_THRESHOLD ||
        spoke.premiumShares <= spoke.drawnShares.percentMulUp(riskPremiumThreshold),
        InvalidPremiumChange());
```

**Claim 3 — there is no `calculateCompoundedInterest`; interest is simple per
interval and compounds per interaction.**

Confirmed. `grep -rn 'calculateCompoundedInterest\|Compounded' src/` returns
nothing. `MathUtils` (`src/libraries/math/MathUtils.sol`) exposes only
`calculateLinearInterest` (`:20-31`), which returns `RAY + rate·Δt/SECONDS_PER_YEAR`.
`AssetLogic.getDrawnIndex` (`:153-165`) multiplies that into the stored index:

```solidity
return previousIndex.rayMulUp(MathUtils.calculateLinearInterest(asset.drawnRate, lastUpdateTimestamp));
```

So the index compounds once per `accrue()`, i.e. once per interacting transaction
per asset, not continuously. A long-idle asset accrues strictly simple interest
over the idle span.

---

## §1 Architecture in one page

```
                         ┌──────────────────────────────────────┐
   users / EOAs ───────► │  Spoke  (upgradeable, per market)    │
   position managers ──► │  risk config, user positions,        │
                         │  oracle, health factor, liquidation  │
                         └───────────────┬──────────────────────┘
                                         │  add / remove / draw / restore
                                         │  reportDeficit / refreshPremium
                                         │  payFeeShares / transferShares
                                         ▼
                         ┌──────────────────────────────────────┐
   TokenizationSpoke ──► │  Hub  (immutable, per liquidity set) │ ◄── TreasurySpoke
   (ERC-4626 vault)      │  holds ALL underlying tokens          │     (fee receiver)
                         │  shares, drawnIndex, caps, deficit    │
                         └───────┬───────────────────┬──────────┘
                                 │                   │
                    IRS ◄────────┘                   └───────► reinvestment
            AssetInterestRateStrategy                          controller
                                                               (sweep/reclaim)

   governance:  AaveV4Payload ──delegatecall──► AaveV4ConfigEngine
                    ──► HubConfigurator / SpokeConfigurator ──► Hub / Spoke
                    all gated by AccessManagerEnumerable (OZ AccessManager)
```

**Who holds the tokens.** Only the Hub. `Spoke.supply` transfers the underlying
*directly to the Hub address* and then calls `Hub.add`
(`Spoke.sol:234-235`). The Spoke never custodies user funds. This is the single
biggest structural difference from v3, where each aToken held its own reserve.

**Vocabulary.** The Hub speaks in liquidity terms, the Spoke in user terms:

| Spoke calls | Hub function | Meaning |
|---|---|---|
| `supply` | `add` | liquidity enters the Hub, Spoke gains added shares |
| `withdraw` | `remove` | liquidity leaves, Spoke burns added shares |
| `borrow` | `draw` | liquidity leaves as debt, Spoke gains drawn shares |
| `repay` | `restore` | liquidity returns, Spoke burns drawn shares |
| (liquidation, bad debt) | `reportDeficit` | debt written off into `deficitRay` |
| (risk change) | `refreshPremium` | premium shares re-based, no value change |

**Two accounting systems, side by side.**

| | Supply side | Debt side |
|---|---|---|
| unit | `addedShares` | `drawnShares` + `premiumShares` |
| conversion | ERC-4626 shares vs `totalAddedAssets()` | ray `drawnIndex` |
| manipulation defense | 1e6 virtual assets/shares | index starts at RAY, monotonic |
| where | `SharesMath.sol` | `AssetLogic.sol:25-55`, `Premium.sol` |
| yield source | `totalAddedAssets` grows as debt accrues | rate from IRS |

Supplier yield is *emergent*: `totalAddedAssets` (`AssetLogic.sol:79-97`) equals
`liquidity + swept + aggregatedOwed − realizedFees − unrealizedFees`. When debt
accrues, `aggregatedOwed` rises, so the same number of shares is worth more.

---

## §2 File inventory (all 119)

Total in-scope files: 119

| File | Lines | Purpose |
|---|---:|---|
| `src/access/AccessManagerEnumerable.sol` | 397 | Extension of AccessManager that tracks role members and their function selectors using EnumerableSet. |
| `src/access/interfaces/IAccessManagerEnumerable.sol` | 222 | Interface for AccessManagerEnumerable extension. |
| `src/config-engine/AaveV4ConfigEngine.sol` | 129 | Implementation of IAaveV4ConfigEngine. Delegates to external library contracts for |
| `src/config-engine/AaveV4Payload.sol` | 438 | Abstract base payload contract for Aave V4 governance proposals. |
| `src/config-engine/interfaces/IAaveV4ConfigEngine.sol` | 441 | Interface for the Aave V4 Config Engine, defining all structs and engine method signatures. |
| `src/config-engine/libraries/AccessManagerEngine.sol` | 88 | Library containing access manager logic for AaveV4ConfigEngine. |
| `src/config-engine/libraries/EngineFlags.sol` | 46 | Sentinel values for partial updates in config engine structs. |
| `src/config-engine/libraries/HubEngine.sol` | 353 | Library containing hub configurator logic for AaveV4ConfigEngine. |
| `src/config-engine/libraries/PositionManagerEngine.sol` | 38 | Library containing position manager logic for AaveV4ConfigEngine. |
| `src/config-engine/libraries/SpokeEngine.sol` | 228 | Library containing Spoke configurator logic for AaveV4ConfigEngine. |
| `src/config-engine/libraries/TokenizationSpokeDeployer.sol` | 121 | Library for deterministic CREATE2 deployment and address pre-computation of TokenizationSpoke proxies |
| `src/deployments/batches/AaveV4AuthorityBatch.sol` | 25 | Deploys the AccessManagerEnumerable contract and creates a batch report. |
| `src/deployments/batches/AaveV4ConfiguratorBatch.sol` | 45 | Deploys the HubConfigurator and SpokeConfigurator contracts, producing a batch report. |
| `src/deployments/batches/AaveV4GatewayBatch.sol` | 54 | Deploys the NativeTokenGateway and SignatureGateway contracts, producing a batch report. |
| `src/deployments/batches/AaveV4HubInstanceBatch.sol` | 47 | Deploys a Hub (proxy + implementation) and its InterestRateStrategy, producing a batch report. |
| `src/deployments/batches/AaveV4PositionManagerBatch.sol` | 34 | Deploys the GiverPositionManager, TakerPositionManager, and ConfigPositionManager contracts, producing a ba... |
| `src/deployments/batches/AaveV4SpokeInstanceBatch.sol` | 56 | Deploys a Spoke (proxy + implementation) and its AaveOracle, producing a batch report. |
| `src/deployments/batches/AaveV4TokenizationSpokeBatch.sol` | 51 | Deploys a TokenizationSpoke instance (proxy + implementation), producing a batch report. |
| `src/deployments/batches/AaveV4TreasurySpokeBatch.sol` | 25 | Deploys the TreasurySpoke contract, producing a batch report. |
| `src/deployments/libraries/BatchReports.sol` | 65 | Report structs returned by each deployment batch, including deployed contract addresses. |
| `src/deployments/libraries/OrchestrationReports.sol` | 42 | Aggregated deployment reports produced by the orchestration layer. |
| `src/deployments/orchestration/AaveV4DeployBase.sol` | 177 | Static deploy helpers that instantiate each deployment batch. |
| `src/deployments/orchestration/AaveV4DeployOrchestration.sol` | 517 | Main orchestrator that deploys all Aave V4 contracts in order and configures roles. |
| `src/deployments/procedures/AaveV4DeployProcedureBase.sol` | 11 | Base contract for all Aave V4 deployment procedures, providing access to Foundry cheat codes. |
| `src/deployments/procedures/deploy/AaveV4AccessManagerEnumerableDeployProcedure.sol` | 24 | Deploys the AccessManagerEnumerable contract for access control. |
| `src/deployments/procedures/deploy/hub/AaveV4HubConfiguratorDeployProcedure.sol` | 24 | Deploys the HubConfigurator contract for configuring the Hub. |
| `src/deployments/procedures/deploy/hub/AaveV4HubDeployProcedure.sol` | 36 | Deploys an upgradeable Hub instance behind a transparent proxy. |
| `src/deployments/procedures/deploy/hub/AaveV4InterestRateStrategyDeployProcedure.sol` | 24 | Deploys the AssetInterestRateStrategy contract for the Hub. |
| `src/deployments/procedures/deploy/position-manager/AaveV4ConfigPositionManagerDeployProcedure.sol` | 24 | Deploys the ConfigPositionManager contract. |
| `src/deployments/procedures/deploy/position-manager/AaveV4GiverPositionManagerDeployProcedure.sol` | 24 | Deploys the GiverPositionManager contract. |
| `src/deployments/procedures/deploy/position-manager/AaveV4NativeTokenGatewayDeployProcedure.sol` | 33 | Deploys the NativeTokenGateway contract. |
| `src/deployments/procedures/deploy/position-manager/AaveV4SignatureGatewayDeployProcedure.sol` | 24 | Deploys the SignatureGateway contract. |
| `src/deployments/procedures/deploy/position-manager/AaveV4TakerPositionManagerDeployProcedure.sol` | 24 | Deploys the TakerPositionManager contract. |
| `src/deployments/procedures/deploy/spoke/AaveV4AaveOracleDeployProcedure.sol` | 18 | Deploys the AaveOracle contract. |
| `src/deployments/procedures/deploy/spoke/AaveV4SpokeConfiguratorDeployProcedure.sol` | 23 | Deploys the SpokeConfigurator contract for configuring Spoke instances. |
| `src/deployments/procedures/deploy/spoke/AaveV4SpokeDeployProcedure.sol` | 58 | Deploys an upgradeable Spoke instance behind a transparent proxy. |
| `src/deployments/procedures/deploy/spoke/AaveV4TokenizationSpokeDeployProcedure.sol` | 71 | Deploys an upgradeable TokenizationSpoke instance behind a transparent proxy. |
| `src/deployments/procedures/deploy/spoke/AaveV4TreasurySpokeDeployProcedure.sol` | 30 | Deploys the TreasurySpoke contract behind a transparent proxy. |
| `src/deployments/procedures/roles/AaveV4AccessManagerRolesProcedure.sol` | 76 | Procedures for labelling protocol roles and managing the default admin role on the AccessManager. |
| `src/deployments/procedures/roles/AaveV4HubConfiguratorRolesProcedure.sol` | 55 | Procedures for granting and setting up HubConfigurator roles on the AccessManager. |
| `src/deployments/procedures/roles/AaveV4HubRolesProcedure.sol` | 76 | Procedures for granting and setting up Hub roles on the AccessManager. |
| `src/deployments/procedures/roles/AaveV4SpokeConfiguratorRolesProcedure.sol` | 56 | Procedures for granting and setting up SpokeConfigurator roles on the AccessManager. |
| `src/deployments/procedures/roles/AaveV4SpokeRolesProcedure.sol` | 66 | Procedures for granting and setting up spoke roles on the AccessManager. |
| `src/deployments/utils/Logger.sol` | 204 | JSON output report and console logging for deployment scripts. |
| `src/deployments/utils/MetadataLogger.sol` | 86 | Extends Logger with structured custom JSON report formatting for full deployment outputs. |
| `src/deployments/utils/interfaces/IHubInstance.sol` | 16 | Hub instance interface exposing the initializer and revision. |
| `src/deployments/utils/interfaces/ISpokeInstance.sol` | 16 | Spoke instance interface exposing the initializer and revision. |
| `src/deployments/utils/interfaces/ITokenizationSpokeInstance.sol` | 17 | TokenizationSpoke instance interface exposing the initializer and revision. |
| `src/deployments/utils/libraries/BytecodeHelper.sol` | 23 | Library for loading contract bytecode. |
| `src/deployments/utils/libraries/Create2Utils.sol` | 85 | Deterministic deployment helpers using the Safe Singleton Factory. |
| `src/deployments/utils/libraries/DeployConstants.sol` | 12 | Protocol constants used by the deployment engine. |
| `src/deployments/utils/libraries/InputUtils.sol` | 65 | Deployment input struct and validation helpers. |
| `src/deployments/utils/libraries/Roles.sol` | 174 | Defines the different roles used by the protocol and their target selectors. |
| `src/hub/AssetInterestRateStrategy.sol` | 138 | Manages the optimal-usage-based interest rate strategy for an asset. |
| `src/hub/Hub.sol` | 960 | A liquidity hub that manages assets and spokes. |
| `src/hub/HubConfigurator.sol` | 326 | Handles administrative functions on the Hub. |
| `src/hub/HubStorage.sol` | 29 | Storage layout for the Hub contract. |
| `src/hub/instances/HubInstance.sol` | 24 | Implementation contract for the Hub. |
| `src/hub/interfaces/IAssetInterestRateStrategy.sol` | 96 | Interface of the optimal-usage-based asset interest rate strategy. |
| `src/hub/interfaces/IBasicInterestRateStrategy.sol` | 31 | Basic interface for any interest rate strategy. |
| `src/hub/interfaces/IHub.sol` | 423 | Full interface for the Hub. |
| `src/hub/interfaces/IHubBase.sol` | 336 | Minimal interface for Hub. |
| `src/hub/interfaces/IHubConfigurator.sol` | 222 | Interface for HubConfigurator. |
| `src/hub/libraries/AssetLogic.sol` | 243 | Implements the base logic and share price conversions for asset data. |
| `src/hub/libraries/Premium.sol` | 24 | Implements the premium calculations. |
| `src/hub/libraries/SharesMath.sol` | 63 | Implements the logic to convert between assets and shares. |
| `src/interfaces/IExtSload.sol` | 15 | Minimal interface to easily access storage of source contract externally. See https://eips.ethereum.org/EIP... |
| `src/interfaces/IIntentConsumer.sol` | 15 | Minimal interface for IntentConsumer. |
| `src/interfaces/IMulticall.sol` | 12 | Minimal interface for Multicall. |
| `src/interfaces/INoncesKeyed.sol` | 19 | Thrown when nonce being consumed does not match `currentNonce` for `account`. |
| `src/interfaces/IRescuable.sol` | 25 | Interface for Rescuable. |
| `src/libraries/math/MathUtils.sol` | 128 | Calculates the interest accumulated using a linear interest rate formula. |
| `src/libraries/math/PercentageMath.sol` | 82 | Provides functions to perform percentage calculations with explicit rounding. |
| `src/libraries/math/WadRayMath.sol` | 216 | Provides utility functions to work with WAD and RAY units with explicit rounding. |
| `src/position-manager/ConfigPositionManager.sol` | 472 | Position manager to handle position configuration actions on behalf of users. |
| `src/position-manager/GiverPositionManager.sol` | 71 | Position manager to handle supply and repay actions on behalf of users. |
| `src/position-manager/NativeTokenGateway.sol` | 167 | Gateway to interact with a spoke using the native coin of a chain. |
| `src/position-manager/PositionManagerBase.sol` | 125 | Base implementation for position manager common functionalities. |
| `src/position-manager/PositionManagerIntentBase.sol` | 20 | Extension of PositionManagerBase powered with intents consumption functionality. |
| `src/position-manager/SignatureGateway.sol` | 209 | Gateway to consume EIP-712 typed intents for Spoke actions on behalf of a user. |
| `src/position-manager/TakerPositionManager.sol` | 342 | Position manager to handle withdraw and borrow actions on behalf of users. |
| `src/position-manager/interfaces/IConfigPositionManager.sol` | 288 | Interface for position manager handling user configuration actions on behalf of an user. |
| `src/position-manager/interfaces/IGiverPositionManager.sol` | 76 | Interface for position manager handling supply and repay actions on behalf of users. |
| `src/position-manager/interfaces/INativeTokenGateway.sol` | 86 | Abstracts actions to the protocol involving the native token. |
| `src/position-manager/interfaces/INativeWrapper.sol` | 16 | Minimal interface for interacting with a wrapped native token contract. |
| `src/position-manager/interfaces/IPositionManagerBase.sol` | 81 | Base interface for position managers. |
| `src/position-manager/interfaces/IPositionManagerIntentBase.sol` | 10 | Interface to extend PositionManagerBase with intent consuming capabilities. |
| `src/position-manager/interfaces/ISignatureGateway.sol` | 218 | Minimal interface for protocol actions involving signed intents. |
| `src/position-manager/interfaces/ITakerPositionManager.sol` | 235 | Interface for position manager handling withdraw permit and borrow permit actions on behalf of users. |
| `src/position-manager/libraries/ConfigPermissionsMap.sol` | 130 | Implements the bitmap logic to handle the ConfigPermissions configuration. |
| `src/position-manager/libraries/EIP712Hash.sol` | 274 | Helper methods to hash EIP712 typed data structs. |
| `src/spoke/AaveOracle.sol` | 89 | Provides reserve prices. |
| `src/spoke/Spoke.sol` | 947 | Handles risk configuration & borrowing strategy for reserves and user positions. |
| `src/spoke/SpokeConfigurator.sol` | 298 | Handles administrative functions on the Spoke. |
| `src/spoke/SpokeStorage.sol` | 40 | Storage layout for the Spoke contract. |
| `src/spoke/TokenizationSpoke.sol` | 446 | ERC4626 compliant wrapper to tokenize one listed asset of the connected Hub. |
| `src/spoke/TreasurySpoke.sol` | 94 | Spoke contract used as a treasury where accumulated fees are treated as supplied assets. |
| `src/spoke/instances/SpokeInstance.sol` | 33 | Implementation contract for the Spoke. |
| `src/spoke/instances/TokenizationSpokeInstance.sol` | 30 | Implementation contract for the TokenizationSpoke. |
| `src/spoke/instances/TreasurySpokeInstance.sol` | 23 | Implementation contract for the TreasurySpoke. |
| `src/spoke/interfaces/IAaveOracle.sol` | 68 | Interface for the Aave Oracle. |
| `src/spoke/interfaces/IPriceFeed.sol` | 16 | Defines the minimal functions needed to work with the AaveOracle contract. |
| `src/spoke/interfaces/IPriceOracle.sol` | 23 | Basic interface for any price oracle. |
| `src/spoke/interfaces/ISpoke.sol` | 787 | Full interface for Spoke. |
| `src/spoke/interfaces/ISpokeConfigurator.sol` | 211 | Interface for the SpokeConfigurator. |
| `src/spoke/interfaces/ITokenizationSpoke.sol` | 192 | Intent data to deposit assets into the TokenizationSpoke. |
| `src/spoke/interfaces/ITreasurySpoke.sol` | 65 | Interface for the TreasurySpoke. |
| `src/spoke/libraries/EIP712Hash.sol` | 133 | Helper methods to hash EIP712 typed data structs. |
| `src/spoke/libraries/KeyValueList.sol` | 100 | Library to pack key-value pairs in a list. |
| `src/spoke/libraries/LiquidationLogic.sol` | 827 | Implements the logic for liquidations. |
| `src/spoke/libraries/PositionStatusMap.sol` | 252 | Implements the bitmap logic to handle the user configuration. |
| `src/spoke/libraries/ReserveFlagsMap.sol` | 109 | Implements the bitmap logic to handle the Reserve flags configuration. |
| `src/spoke/libraries/SpokeUtils.sol` | 41 | Provides utility functions for the Spoke contract. |
| `src/spoke/libraries/UserPositionUtils.sol` | 165 | Implements debt calculations for user positions. |
| `src/utils/ExtSload.sol` | 36 | This allows the source contract to make its state available to external contracts. |
| `src/utils/IntentConsumer.sol` | 41 | Base contract to consume EIP712-signed intents with keyed-nonces. |
| `src/utils/Multicall.sol` | 27 | This contract allows for batching multiple calls into a single call. |
| `src/utils/NoncesKeyed.sol` | 67 | Provides tracking nonces for addresses. Supports keyed nonces, where nonces will only increment for each key. |
| `src/utils/Rescuable.sol` | 39 | Contract that allows for the rescue of tokens and native assets. |

---

## §3 Math libraries

### 3.1 `src/libraries/math/WadRayMath.sol` (216 lines)

Fixed-point helpers where **every operation names its rounding direction**. That
is the design point: v3's `WadRayMath` rounded half-up implicitly, v4 forces the
caller to choose, and the choice is always "in the protocol's favour".

Constants (`:8-11`): `WAD_DECIMALS = 18`, `WAD = 1e18`, `RAY = 1e27`,
`PERCENTAGE_FACTOR = 1e4`.

| Function | Line | Returns | Reverts when |
|---|---:|---|---|
| `wadMulDown(a,b)` | 16 | `floor(a·b/WAD)` | `a·b` overflows |
| `wadMulUp(a,b)` | 30 | `ceil(a·b/WAD)` | `a·b` overflows |
| `wadDivDown(a,b)` | 45 | `floor(a·WAD/b)` | `b==0` or `a·WAD` overflows |
| `wadDivUp(a,b)` | 59 | `ceil(a·WAD/b)` | same |
| `rayMulDown(a,b)` | 74 | `floor(a·b/RAY)` | `a·b` overflows |
| `rayMulUp(a,b)` | 88 | `ceil(a·b/RAY)` | `a·b` overflows |
| `rayDivDown(a,b)` | 103 | `floor(a·RAY/b)` | `b==0` or overflow |
| `rayDivUp(a,b)` | 117 | `ceil(a·RAY/b)` | same |
| `toWad(a)` | 132 | `a·WAD` | overflow (checked via `div(b,WAD)==a`) |
| `toRay(a)` | 146 | `a·RAY` | overflow |
| `fromWadDown(a)` | 159 | `a/WAD` | never |
| `fromRayUp(a)` | 167 | `ceil(a/RAY)` | never |
| `bpsToWad(a)` | 177 | `a·1e14` | overflow |
| `bpsToRay(a)` | 191 | `a·1e23` | overflow |
| `roundRayUp(a)` | 205 | `ceil(a/RAY)·RAY` | overflow |

All are hand-written Yul with an explicit pre-multiplication overflow guard of the
form `if a > type(uint256).max / b revert`. There is no `unchecked` escape hatch and
no error message — these are bare `revert(0,0)`, so a failure surfaces as an empty
revert, not a named error. That is worth knowing when debugging.

`roundRayUp` (`:205`) is unusual and matters in liquidations: it rounds a
ray-scaled value up to a whole asset unit *and keeps it in ray*, used at
`LiquidationLogic.sol:650` and `:751` so that a premium repayment always covers a
whole token.

### 3.2 `src/libraries/math/PercentageMath.sol` (82 lines)

`PERCENTAGE_FACTOR = 1e4` (`:10`), i.e. basis points, 100_00 = 100.00%.

| Function | Line | Returns |
|---|---:|---|
| `percentMulDown(value, pct)` | 15 | `floor(value·pct/1e4)` |
| `percentMulUp(value, pct)` | 32 | `ceil(value·pct/1e4)` |
| `percentDivDown(value, pct)` | 48 | `floor(value·1e4/pct)` |
| `percentDivUp(value, pct)` | 65 | `ceil(value·1e4/pct)` |
| `fromBpsDown(value)` | 79 | `value/1e4` |

Same Yul overflow-guard pattern, same bare reverts.

### 3.3 `src/libraries/math/MathUtils.sol` (128 lines)

Constants (`:11-13`): `RAY = 1e27`, `SECONDS_PER_YEAR = 365 days` (leap years
ignored, stated in the comment).

**`calculateLinearInterest(uint96 rate, uint40 lastUpdateTimestamp) → uint256`** (`:20-31`)

```solidity
assembly ('memory-safe') {
  if gt(lastUpdateTimestamp, timestamp()) { revert(0, 0) }
  result := sub(timestamp(), lastUpdateTimestamp)
  result := add(div(mul(rate, result), SECONDS_PER_YEAR), RAY)
}
```

Returns `RAY + rate·Δt / SECONDS_PER_YEAR`. This is the **only** interest formula
in v4. There is no compounding function anywhere in `src/` (§0, claim 3).
Compounding happens because the result is multiplied into the stored index on each
`accrue()`.

Remaining helpers, all pure and all used to keep the hot paths cheap:

| Function | Line | Behaviour | Note |
|---|---:|---|---|
| `min(a,b)` | 34 | branchless minimum | `xor`/`lt` trick |
| `zeroFloorSub(a,b)` | 41 | `a>b ? a-b : 0` | saturating |
| `add(uint256,int256)` | 49 | signed add | reverts on underflow |
| `uncheckedAdd(a,b)` | 56 | wrapping add | used for loop counters |
| `signedSub(a,b)` | 63 | `int256(a) - int256(b)` | via `SafeCast.toInt256` |
| `uncheckedSub(a,b)` | 69 | wrapping sub | used after an explicit `<=` check |
| `uncheckedExp(a,b)` | 77 | wrapping `a**b` | used as `10**decimals` |
| `divUp(a,b)` | 86 | `ceil(a/b)` | reverts if `b==0` |
| `mulDivDown(a,b,c)` | 98 | `floor(a·b/c)` | 256-bit intermediate only |
| `mulDivUp(a,b,c)` | 114 | `ceil(a·b/c)` | 256-bit intermediate only |

`mulDivDown`/`mulDivUp` here are **not** full 512-bit like OpenZeppelin's
`Math.mulDiv`; they revert if `a·b` overflows 256 bits. Where a genuine 512-bit
product is needed (liquidation collateral maths), the code calls OZ `Math.mulDiv`
instead — see `LiquidationLogic.sol:635`, `:713`, `:807`.

### 3.4 `src/hub/libraries/SharesMath.sol` (63 lines)

The ERC-4626 virtual-offset defense against share-price (donation) manipulation.

`VIRTUAL_ASSETS = VIRTUAL_SHARES = 1e6` (`:13-14`).

| Function | Line | Formula |
|---|---:|---|
| `toSharesDown(assets, totalAssets, totalShares)` | 17 | `floor(assets · (S+1e6) / (A+1e6))` |
| `toAssetsDown(shares, totalAssets, totalShares)` | 31 | `floor(shares · (A+1e6) / (S+1e6))` |
| `toSharesUp(...)` | 45 | `ceil(assets · (S+1e6) / (A+1e6))` |
| `toAssetsUp(...)` | 55 | `ceil(shares · (A+1e6) / (S+1e6))` |

All four delegate to OZ `Math.mulDiv` with an explicit `Math.Rounding`, so they are
full 512-bit safe.

**Why 1e6 on both sides.** An empty asset starts at a 1:1 rate (`1e6/1e6`). An
attacker who deposits 1 wei and donates a large amount to inflate the price per
share must overcome the virtual 1e6 shares, which makes the classic first-depositor
round-to-zero attack cost 1e6× more. Note the Hub *additionally* refuses zero-share
deposits at `Hub.sol:172` (`require(shares > 0, InvalidShares())`).

**A donation caveat worth stating.** `totalAddedAssets` is computed from *tracked*
accounting fields (`asset.liquidity`, `asset.swept`, owed, fees), not from
`balanceOf(hub)`. A plain ERC-20 transfer to the Hub therefore does **not** move the
share price by itself. It sits untracked until someone skims it — `Hub.add`
(`:169-170`) and `Hub.reclaim` (`:435-436`) both only require that the balance is at
*least* the expected liquidity, so surplus balance can be absorbed by a later `add`.
`IHub.sol:95` and `:335` document this as intended skimming behaviour, and
`TreasurySpoke.supplySkimmed` (`TreasurySpoke.sol:31-37`) is the sanctioned way to
claim it.

### 3.5 `src/hub/libraries/Premium.sol` (24 lines)

One function.

**`calculatePremiumRay(uint256 premiumShares, int256 premiumOffsetRay, uint256 drawnIndex) → uint256`** (`:17-24`)

```solidity
return ((premiumShares * drawnIndex).toInt256() - premiumOffsetRay).toUint256();
```

Premium debt in ray = `premiumShares · drawnIndex − premiumOffsetRay`. The final
`toUint256()` reverts if the result is negative, which is the invariant that premium
debt can never go below zero. Callers: `AssetLogic.premium` (`:62`),
`AssetLogic._calculateAggregatedOwedRay` (`:229`), `Hub.getAssetPremiumRay` (`:536`),
`Hub._getSpokePremiumRay` (`:801`), `Hub._validateApplyPremiumDelta` (`:933`),
`UserPositionUtils._calculatePremiumRay` (`:154`).

### 3.6 `src/hub/libraries/AssetLogic.sol` (243 lines)

The Hub's accounting core. Everything is `internal` and operates on
`IHub.Asset storage`.

**Debt conversions** (`:25-55`) — note the asymmetry, which is the protocol's
rounding policy in miniature:

| Function | Line | Rounding | Used for |
|---|---:|---|---|
| `toDrawnAssetsUp(shares)` | 25 | up | what a borrower owes |
| `toDrawnAssetsDown(shares)` | 33 | down | previews |
| `toDrawnSharesUp(assets)` | 41 | up | `draw` — borrower gets more shares |
| `toDrawnSharesDown(assets)` | 49 | down | `restore` — repayer burns fewer shares |

Borrowers always round against themselves in both directions.

**`drawn(asset, drawnIndex)`** (`:57-59`) → `asset.drawnShares.rayMulUp(drawnIndex)`.

**`premium(asset, drawnIndex)`** (`:62-72`) → `Premium.calculatePremiumRay(...).fromRayUp()`.

**`totalOwed(asset, drawnIndex)`** (`:74-76`) → `drawn + premium`.

**`totalAddedAssets(asset)`** (`:79-97`) — the supply-side denominator:

```solidity
uint256 aggregatedOwedRay = _calculateAggregatedOwedRay({
  drawnShares: asset.drawnShares, premiumShares: asset.premiumShares,
  premiumOffsetRay: asset.premiumOffsetRay, deficitRay: asset.deficitRay,
  drawnIndex: drawnIndex });
return asset.liquidity + asset.swept + aggregatedOwedRay.fromRayUp()
       - asset.realizedFees - asset.getUnrealizedFees(drawnIndex);
```

Read that as: everything the Hub has (idle + invested) plus everything owed to it
(including written-off deficit) minus the protocol's cut. **Deficit is included**,
which is deliberate: writing off bad debt via `reportDeficit` moves value from
`drawn` into `deficitRay` without changing `totalAddedAssets`, so suppliers do not
take an instant loss at the moment of the write-off. They take it only if the
deficit is never eliminated and they try to withdraw against absent liquidity.

**Supply conversions** (`:99-130`): `toAddedAssetsUp/Down`, `toAddedSharesUp/Down`,
each forwarding to the corresponding `SharesMath` function with
`asset.totalAddedAssets()` and `asset.addedShares`.

**`updateDrawnRate(asset, assetId)`** (`:132-138`) — recomputes the rate from the IRS
using the *stored* index (accrual must already have happened) and emits
`IHub.UpdateAsset`. Called at the end of every mutating Hub function.

**`accrue(asset)`** (`:141-150`):

```solidity
if (asset.lastUpdateTimestamp == block.timestamp) return;
uint256 drawnIndex = asset.getDrawnIndex();
asset.realizedFees += asset.getUnrealizedFees(drawnIndex).toUint120();
asset.drawnIndex = drawnIndex.toUint120();
asset.lastUpdateTimestamp = block.timestamp.toUint40();
```

Order matters: fees are realized against the *old* index before the index moves.

**`getDrawnIndex(asset)`** (`:153-165`) — pure view of the current index. Short-circuits
to the stored value if the timestamp is current **or if there is no debt at all**
(`asset.drawnShares == 0 && asset.premiumShares == 0`). The second condition means an
asset with zero borrows does not drift its index, so idle time is not "lost" interest.

**`getDrawnRate(asset, assetId, drawnIndex)`** (`:170-184`) — calls
`IBasicInterestRateStrategy.calculateInterestRate` with `liquidity`, `drawn`,
`deficit` and `swept`. The doc comment at `:167-169` states premium debt is *not*
an input to the rate: risk premium is priced per borrower, not into the base curve.

**`getUnrealizedFees(asset, drawnIndex)`** (`:187-226`) — the protocol's cut of index
growth. Computes aggregated owed at the new and old index and takes
`liquidityFee` bps of the difference, rounding down. Returns 0 early if the index
did not move (`:191-193`) or the fee is zero (`:196-199`).

**`_calculateAggregatedOwedRay(...)`** (`:229-242`) —
`drawnShares·drawnIndex + premiumRay + deficitRay`, all in ray.

---

## §4 Hub layer

### 4.1 `src/hub/HubStorage.sol` (29 lines)

```
slot 0   uint256 _assetCount                                                 :13
slot 1   mapping(uint256 assetId => IHub.Asset) _assets                      :16
slot 2   mapping(uint256 => mapping(address => IHub.SpokeData)) _spokes      :19
slot 3   mapping(uint256 => EnumerableSet.AddressSet) _assetToSpokes         :22
slot 4   mapping(address underlying => uint256 assetId) _underlyingToAssetId :25
slot 5-54  uint256[50] __gap                                                 :28
```

`abstract contract HubStorage` (`:11`). Note the Hub is *upgradeable by proxy but
declared immutable in intent* — `HubInstance` is deployed behind a proxy for
initialization, and the docs describe the Hub as immutable. The 50-slot gap exists
regardless.

**`_underlyingToAssetId` maps to 0 by default**, which is why `isUnderlyingListed`
(`Hub.sol:446-448`) checks `_assets[id].underlying == underlying` rather than
`id != 0` — asset 0 is a real asset.

### 4.2 `src/hub/interfaces/IHub.sol` — the `Asset` struct (`:29-56`)

Packed into 8 slots. Field order in the source is the packing order:

| Slot | Fields | Meaning |
|---:|---|---|
| 0 | `uint120 liquidity`, `uint120 realizedFees`, `uint8 decimals` | idle tokens; fees accrued not yet minted; underlying decimals |
| 1 | `uint120 addedShares`, `uint120 swept` | total supply shares; amount out with the reinvestment controller |
| 2 | `int200 premiumOffsetRay` | signed offset for the premium formula |
| 3 | `uint120 drawnShares`, `uint120 premiumShares`, `uint16 liquidityFee` | debt shares; premium shares; protocol fee in bps |
| 4 | `uint120 drawnIndex`, `uint96 drawnRate`, `uint40 lastUpdateTimestamp` | ray index; ray rate; accrual stamp |
| 5 | `address underlying` | |
| 6 | `address irStrategy` | |
| 7 | `address reinvestmentController` | zero disables sweep/reclaim |
| 8 | `address feeReceiver` | a Spoke address |
| 9 | `uint200 deficitRay` | written-off bad debt |

`uint120` for balances caps a single asset's tracked amount at ~1.3e36, comfortable
for 18-decimal tokens. The casts are `SafeCast`, so an overflow reverts rather than
wrapping.

**`AssetConfig`** (`:59-64`): `feeReceiver`, `liquidityFee`, `irStrategy`,
`reinvestmentController` — the writable subset.

**`SpokeData`** (`:77-91`), 4 slots:

| Slot | Fields |
|---:|---|
| 0 | `uint120 drawnShares`, `uint120 premiumShares` |
| 1 | `int200 premiumOffsetRay` |
| 2 | `uint120 addedShares`, `uint40 addCap`, `uint40 drawCap`, `uint24 riskPremiumThreshold`, `bool active`, `bool halted` |
| 3 | `uint200 deficitRay` |

Caps are stored as **whole assets, not scaled by decimals** (`:71-72`), and compared
after multiplying by `10**decimals` (`Hub.sol:825`, `:854`, `:914`). A cap of
`type(uint40).max` means no cap. Same sentinel convention for
`riskPremiumThreshold` with `type(uint24).max`.

**`SpokeConfig`** (`:94-100`): `addCap`, `drawCap`, `riskPremiumThreshold`,
`active`, `halted`.

`active` vs `halted` (`:74-75`): `active = false` blocks *every* action;
`halted = true` blocks only the actions that instantly move liquidity. Check
`_validateReportDeficit` (`Hub.sol:875-887`) — it requires `active` but **not**
`!halted`, so a halted Spoke can still write off bad debt. Same for
`_validateEliminateDeficit` (`:889-895`) and `_validatePayFeeShares` (`:897-900`).

### 4.3 `src/hub/interfaces/IHubBase.sol` — `PremiumDelta` (`:12-16`)

```solidity
struct PremiumDelta {
  int256 sharesDelta;        // change in premium shares
  int256 offsetRayDelta;     // change in premium offset (ray)
  uint256 restoredPremiumRay; // premium actually repaid (ray)
}
```

Every premium mutation travels as this triple, and the Hub checks
`premiumAfter + restored == premiumBefore` (`Hub.sol:954-957`).

### 4.4 `src/hub/Hub.sol` (960 lines) — constants and modifiers

| Constant | Line | Value |
|---|---:|---|
| `MAX_ALLOWED_UNDERLYING_DECIMALS` | 32 | 18 |
| `MIN_ALLOWED_UNDERLYING_DECIMALS` | 35 | 6 |
| `MAX_ALLOWED_SPOKE_CAP` | 38 | `type(uint40).max` |
| `MAX_RISK_PREMIUM_THRESHOLD` | 41 | `type(uint24).max` |

Inheritance (`:21`): `Hub is IHub, HubStorage, AccessManagedUpgradeable`.
Access control is the OZ `restricted` modifier from `AccessManagedUpgradeable`,
which asks the `AccessManager` authority whether `msg.sender` may call this
selector on this target. There are no role constants in the Hub itself — see §10.

`initialize(address authority)` is `external virtual` (`:44`) and implemented in
`HubInstance` (`instances/HubInstance.sol`).

**Uniform shape of every mutating function.** Read one and you have read them all:

```
1. resolve  Asset storage asset = _assets[assetId];
            SpokeData storage spoke = _spokes[assetId][msg.sender];
2. accrue   asset.accrue();               // index + realized fees move first
3. validate _validateXxx(...);            // caps, active/halted, amounts
4. convert  shares <-> assets with the correct rounding
5. mutate   asset.* and spoke.* in lockstep (invariant 1 and 3)
6. rate     asset.updateDrawnRate(assetId);  // recompute + emit UpdateAsset
7. move     IERC20 transfer, or balance check for pull-style
8. emit     the action event
```

Steps 2 and 6 bracket every state change, which is what keeps the index and rate
consistent without a separate keeper.

#### Governance functions (all `restricted`)

**`addAsset(address underlying, uint8 decimals, address feeReceiver, address irStrategy, bytes irData) → uint256`** (`:47-112`)

- **Checks**: non-zero `underlying`/`feeReceiver`/`irStrategy` → `InvalidAddress` (`:54-57`); `6 ≤ decimals ≤ 18` → `InvalidAssetDecimals` (`:58-61`); `!isUnderlyingListed` → `UnderlyingAlreadyListed` (`:62`).
- **External calls**: `IBasicInterestRateStrategy(irStrategy).setInterestRateData(assetId, irData)` (`:67`), then `calculateInterestRate` with all-zero state (`:68-74`).
- **Writes**: `_assetCount++`, `_underlyingToAssetId[underlying]`, the whole `_assets[assetId]` struct with `drawnIndex = RAY` and `liquidityFee = 0` and `reinvestmentController = address(0)` (`:78-96`); then `_addFeeReceiver` (`:97`).
- **Emits**: `AddAsset`, `UpdateAssetConfig`, `UpdateAsset` (`:99-110`).
- **Returns**: the new `assetId`.
- **Note**: the fee receiver is registered as a Spoke with `addCap = MAX`, `drawCap = 0`, `active = true` (`_addFeeReceiver`, `:688-701`). The treasury can receive but never borrow.

**`updateAssetConfig(uint256 assetId, AssetConfig config, bytes irData)`** (`:115-155`)

- **Checks**: `assetId < _assetCount` → `AssetNotListed`; `liquidityFee ≤ 100_00` → `InvalidLiquidityFee` (`:124`); non-zero `feeReceiver`/`irStrategy` → `InvalidAddress` (`:125`); `reinvestmentController != 0 || asset.swept == 0` → `InvalidReinvestmentController` (`:126-129`) — you cannot unset a controller while it still holds funds.
- **Fee-receiver rotation** (`:134-143`): mints outstanding fees to the *old* receiver first (`_mintFeeShares`), zeroes the old receiver's caps while preserving its `active`/`halted`, then registers the new receiver.
- **IRS swap** (`:145-150`): if the strategy address changes, it is set and configured with `irData`; if it does not change, `irData` **must be empty** → `InvalidInterestRateStrategy` (`:149`).
- Ends with `asset.updateDrawnRate(assetId)` and `UpdateAssetConfig`.

**`addSpoke(uint256 assetId, address spoke, SpokeConfig config)`** (`:158-167`) — `AssetNotListed`, `InvalidAddress`, then `_addSpoke` (reverts `SpokeAlreadyListed` via `EnumerableSet.add` returning false, `:706`) and `_updateSpokeConfig`.

**`updateSpokeConfig(uint256 assetId, address spoke, SpokeConfig config)`** (`:170-178`) — `AssetNotListed`, `SpokeNotListed`, then `_updateSpokeConfig`.

**`setInterestRateData(uint256 assetId, bytes irData)`** (`:181-187`) — accrue, forward to the current strategy, re-derive the rate.

**`mintFeeShares(uint256 assetId) → uint256`** (`:190-197`) — accrue, `_mintFeeShares`, re-derive rate. Returns shares minted; **no-op returning 0** if accrued fees are worth less than one share (`:767-770`). Reverts `SpokeNotActive` if the fee receiver Spoke is inactive (`:774`).

#### Spoke-facing liquidity functions (callable by any registered Spoke, no `restricted`)

Access control here is *implicit*: `_spokes[assetId][msg.sender]` is a zero struct
for an unregistered caller, so `spoke.active` is false and `_validateXxx` reverts
`SpokeNotActive`. There is no separate registry check.

**`add(uint256 assetId, uint256 amount) → uint256 shares`** (`:200-221`)

- `_validateAdd` (`:814-829`): `amount > 0` → `InvalidAmount`; `active` → `SpokeNotActive`; `!halted` → `SpokeHalted`; add cap → `AddCapExceeded(addCap)`.
- **Pull-style, not transferFrom**: the Spoke must have already sent the tokens. The Hub verifies `balanceOf(this) >= asset.liquidity + amount` → `InsufficientTransferred(needed)` (`:208-209`).
- `shares = toAddedSharesDown(amount)` rounded **down** (caller gets no free share), then `require(shares > 0, InvalidShares())` (`:211`).
- Writes `asset.addedShares += shares`, `spoke.addedShares += shares`, `asset.liquidity = liquidity` (`:212-214`).
- Emits `Add(assetId, msg.sender, shares, amount)`.

**`remove(uint256 assetId, uint256 amount, address to) → uint256 shares`** (`:224-246`)

- `_validateRemove` (`:831-836`): `to != address(this)` → `InvalidAddress`; `amount > 0`; active; not halted.
- `amount <= asset.liquidity` → `InsufficientLiquidity(liquidity)` (`:232`).
- `shares = toAddedSharesUp(amount)` rounded **up** (withdrawer burns more shares).
- Writes, then `safeTransfer(to, amount)` **after** `updateDrawnRate` (`:239-241`).
- Emits `Remove`.

**`draw(uint256 assetId, uint256 amount, address to) → uint256 drawnShares`** (`:249-271`)

- `_validateDraw` (`:840-858`): `to != address(this)`; `amount > 0`; active; not halted; draw cap checked against **owed + amount + deficit** (`:851-856`), so a Spoke carrying bad debt has correspondingly less borrowing room.
- `amount <= liquidity` → `InsufficientLiquidity`.
- `drawnShares = toDrawnSharesUp(amount)` rounded **up**.
- Transfers out, emits `Draw`.

**`restore(uint256 assetId, uint256 drawnAmount, PremiumDelta premiumDelta) → uint256`** (`:274-301`)

- `_validateRestore` (`:860-873`): `drawnAmount > 0 || restoredPremiumRay > 0` → `InvalidAmount`; active; not halted; `drawnAmount <= spokeDrawn` → `SurplusDrawnRestored(drawn)`; `premiumAmountRay <= spokePremiumRay` → `SurplusPremiumRayRestored(premiumRay)`.
- `drawnShares = toDrawnSharesDown(drawnAmount)` rounded **down**.
- `_applyPremiumDelta` (`:283`) mutates asset and spoke premium and enforces conservation + threshold.
- Pull-style balance check for `liquidity + drawnAmount + premiumAmount` → `InsufficientTransferred` (`:285-288`).
- Emits `Restore(assetId, spoke, drawnShares, premiumDelta, drawnAmount, premiumAmount)`.

**`reportDeficit(uint256 assetId, uint256 drawnAmount, PremiumDelta premiumDelta) → (uint256, uint256)`** (`:304-330`)

- `_validateReportDeficit` (`:875-887`): amounts, `active` (**not** `!halted`), `SurplusDrawnDeficitReported`, `SurplusPremiumRayDeficitReported`.
- Burns drawn shares from asset and spoke, applies the premium delta, then **adds** the value to `asset.deficitRay` and `spoke.deficitRay` (`:320-323`):
  ```solidity
  uint256 deficitAmountRay = uint256(drawnShares) * asset.drawnIndex + premiumDelta.restoredPremiumRay;
  ```
- No tokens move. Value is reclassified from "owed" to "written off", and both are inside `totalAddedAssets`, so the share price is unchanged at this instant.
- Returns `(drawnShares, deficitAmount)`.

**`refreshPremium(uint256 assetId, PremiumDelta premiumDelta)`** (`:362-374`) — `active` only; **requires `restoredPremiumRay == 0`** (`:369`) so no repayment can be smuggled through a refresh. Applies the delta, re-derives the rate, emits `RefreshPremium`.

**`payFeeShares(uint256 assetId, uint256 shares)`** (`:377-389`) — moves added shares from the caller Spoke to the asset's `feeReceiver`. Used by liquidations to route the liquidation fee (`LiquidationLogic.sol:481`). Emits `TransferShares`.

**`transferShares(uint256 assetId, uint256 shares, address toSpoke)`** (`:392-403`) — Spoke-to-Spoke share movement; `_validateTransferShares` (`:902-918`) requires both active, both not halted, `shares > 0`, and the **receiver's** add cap to accommodate the new total → `AddCapExceeded`.

**`eliminateDeficit(uint256 assetId, uint256 amount, address spoke) → (uint256, uint256)`** (`:333-359`) — `restricted`. The caller Spoke burns its own added shares to erase another Spoke's deficit:

```solidity
uint256 deficitAmountRay = (amount < deficitRay.fromRayUp()) ? amount.toRay() : deficitRay;
...
uint120 shares = asset.toAddedSharesUp(deficitToEliminate);
asset.addedShares -= shares;  callerSpoke.addedShares -= shares;
asset.deficitRay -= deficitAmountRay;  coveredSpoke.deficitRay -= deficitAmountRay;
```

This is where a backstop (Umbrella-style) actually pays. Emits `EliminateDeficit`.

#### Reinvestment

**`sweep(uint256 assetId, uint256 amount)`** (`:406-424`) — caller must equal `asset.reinvestmentController` → `OnlyReinvestmentController` (`:922`); moves `amount` from `liquidity` to `swept` and transfers the tokens out. Because `swept` is still inside `totalAddedAssets`, suppliers keep their claim on it.

**`reclaim(uint256 assetId, uint256 amount)`** (`:427-443`) — the mirror. Pull-style: the controller must have sent the tokens first; `InsufficientTransferred` otherwise. Any *extra* balance is absorbed into `liquidity` (`IHub.sol:335` documents this skim).

**Loss-bearing note.** Nothing in `Hub.sol` forces the controller to return what it took. `swept` is only reduced by `reclaim`. If a strategy loses money and the controller never reclaims, `totalAddedAssets` still counts the swept amount, so suppliers' share price stays nominally intact while the liquidity is simply absent. The overview doc says the Governor absorbs strategy losses; that is a policy statement, not a code-enforced guarantee.

#### View functions

Previews mirror the mutating rounding exactly, which is the point of having eight of them:

| View | Line | Delegates to | Rounding |
|---|---:|---|---|
| `previewAddByAssets` | 456 | `toAddedSharesDown` | down |
| `previewAddByShares` | 461 | `toAddedAssetsUp` | up |
| `previewRemoveByAssets` | 466 | `toAddedSharesUp` | up |
| `previewRemoveByShares` | 471 | `toAddedAssetsDown` | down |
| `previewDrawByAssets` | 476 | `toDrawnSharesUp` | up |
| `previewDrawByShares` | 481 | `toDrawnAssetsDown` | down |
| `previewRestoreByAssets` | 486 | `toDrawnSharesDown` | down |
| `previewRestoreByShares` | 491 | `toDrawnAssetsUp` | up |

Asset views: `isUnderlyingListed` (`:446`), `getAssetCount` (`:451`), `getAssetId`
(`:496`, reverts `AssetNotListed`), `getAssetUnderlyingAndDecimals` (`:502`),
`getAssetDrawnIndex` (`:508`), `getAddedAssets` (`:513`), `getAddedShares` (`:518`),
`getAssetOwed` (`:523`), `getAssetTotalOwed` (`:530`), `getAssetPremiumRay` (`:536`),
`getAssetDrawnShares` (`:547`), `getAssetPremiumData` (`:552`), `getAssetLiquidity`
(`:558`), `getAssetDeficitRay` (`:563`), `getAsset` (`:568`), `getAssetConfig` (`:573`),
`getAssetAccruedFees` (`:585`, realized + unrealized), `getAssetSwept` (`:591`),
`getAssetDrawnRate` (`:596`).

Spoke views: `getSpokeCount` (`:602`), `getSpokeAddedAssets` (`:607`),
`getSpokeAddedShares` (`:612`), `getSpokeOwed` (`:617`), `getSpokeTotalOwed` (`:624`),
`getSpokePremiumRay` (`:631`), `getSpokeDrawnShares` (`:638`), `getSpokePremiumData`
(`:643`), `getSpokeDeficitRay` (`:652`), `isSpokeListed` (`:657`), `getSpokeAddress`
(`:662`), `getSpoke` (`:667`), `getSpokeConfig` (`:672`).

`getAsset` (`:568`) returns the raw struct, so `drawnIndex`, `drawnRate` and
`lastUpdateTimestamp` can be stale — the interface says so at `IHub.sol:349`. Use
`getAssetDrawnIndex` / `getAssetDrawnRate` for live values.

#### Internal helpers

`_addFeeReceiver` (`:688`), `_addSpoke` (`:705`), `_updateSpokeConfig` (`:710`),
`_transferShares` (`:721`), `_applyPremiumDelta` (`:734`), `_mintFeeShares` (`:765`),
`_getSpokeDrawn` (`:785`), `_getSpokePremium` (`:793`), `_getSpokePremiumRay` (`:801`),
the eight `_validate*` functions (`:814-930`), and `_validateApplyPremiumDelta` (`:933`).

#### The four accounting invariants, mapped to code

The overview doc states four invariants. None is a runtime `require`; each is
maintained structurally by paired writes and rounding direction.

| Invariant | Maintained by |
|---|---|
| 1. total drawn shares == Σ spoke drawn shares | every `asset.drawnShares ±= x` is adjacent to `spoke.drawnShares ±= x` — `Hub.sol:260-261`, `:285-286`, `:316-317` |
| 2. Hub added assets ≥ Σ spoke added assets | `toAddedAssetsDown` for reads (`:607`) vs `toAddedSharesUp` on removal (`:234`) — the protocol rounds each Spoke's claim down |
| 3. Hub added shares == Σ spoke added shares | paired writes at `:212-213`, `:234-235`, `:338-339` (`_mintFeeShares` `:776-777`), and `_transferShares` (`:726-727`) moves without changing the total |
| 4. share price and drawn index never decrease | `getDrawnIndex` multiplies by `RAY + something ≥ 0` (`:157-164`); the share price cannot fall because every removal burns shares rounded **up** against the remover |

Confirming these hold globally is what the Certora reports in `audits/` are for
(`2026-03-09_Hub-Fomal-Verification_Certora.pdf`); this document does not reproduce
their proofs.

### 4.5 `src/hub/HubConfigurator.sol` (326 lines)

`contract HubConfigurator is AccessManaged, IHubConfigurator` (`:14`). A thin,
role-gated facade in front of `Hub`. It exists so governance can hand out narrow
permissions (raise a cap) without granting the Hub's full `restricted` surface.
Every function is `restricted` and takes the target `hub` address as its first
argument, so one configurator serves many Hubs.

| Function | Line | Effect |
|---|---:|---|
| `addAsset` | 24 | list an asset (decimals read from the token) |
| `addAssetWithDecimals` | 45 | list with explicit decimals |
| `updateLiquidityFee` | 61 | fee bps only |
| `updateFeeReceiver` | 70 | receiver only |
| `updateFeeConfig` | 82 | both |
| `updateInterestRateStrategy` | 96 | swap the IRS |
| `updateReinvestmentController` | 109 | set/unset the controller |
| `resetAssetCaps` | 121 | zero caps across all spokes of an asset |
| `deactivateAsset` | 135 | `active = false` for all spokes |
| `haltAsset` | 147 | `halted = true` for all spokes |
| `addSpoke` | 159 | register one spoke on one asset |
| `addSpokeToAssets` | 169 | register one spoke across many assets |
| `updateSpokeActive` | 183 | |
| `updateSpokeHalted` | 196 | |
| `updateSpokeAddCap` | 209 | |
| `updateSpokeDrawCap` | 222 | |
| `updateSpokeRiskPremiumThreshold` | 235 | |
| `updateSpokeCaps` | 248 | both caps at once |
| `deactivateSpoke` | 259 | across all assets |
| `haltSpoke` | 272 | across all assets |
| `resetSpokeCaps` | 285 | across all assets |
| `updateInterestRateData` | 299 | reconfigure the current IRS |

Internals: `_updateSpokeCaps` (`:308`), `_updateLiquidityFee` (`:321`).
Errors: `InvalidAddress` and `MismatchedConfigs` (`IHubConfigurator.sol:11`, `:14`).

The "across all assets/spokes" variants are the emergency levers: one call to
`haltAsset` stops instant-liquidity actions on every Spoke for that asset while
still permitting deficit reporting.

### 4.6 `src/hub/instances/HubInstance.sol` (24 lines)

`HUB_REVISION = 1`. Constructor calls `_disableInitializers()`.
`initialize(address authority)` is `reinitializer(HUB_REVISION)`, requires a
non-zero authority, and calls `__AccessManaged_init`.

### 4.7 `src/hub/AssetInterestRateStrategy.sol` (138 lines)

See §8.2.

---

## §5 Spoke layer

### 5.1 `src/spoke/SpokeStorage.sol` (40 lines)

```
slot 0   uint256 _reserveCount                                                       :12
slot 1   ISpoke.LiquidationConfig _liquidationConfig                                  :15
slot 2   mapping(uint256 reserveId => ISpoke.Reserve) _reserves                       :18
slot 3   mapping(address hub => mapping(uint256 assetId => uint256)) _hubAssetIdToReserveId :21
slot 4   mapping(uint256 => mapping(uint32 => ISpoke.DynamicReserveConfig)) _dynamicConfig  :25
slot 5   mapping(address user => ISpoke.PositionStatus) _positionStatus               :29
slot 6   mapping(address => mapping(uint256 => ISpoke.UserPosition)) _userPositions   :32
slot 7   mapping(address => ISpoke.PositionManagerConfig) _positionManager            :36
slot 8-57  uint256[50] __gap                                                          :39
```

`LiquidationConfig` is `uint128 + uint64 + uint16` so it fits one slot.

### 5.2 `src/spoke/interfaces/ISpoke.sol` (787 lines) — types

`type ReserveFlags is uint8` (`:10`) — a user-defined value type, so flags cannot be
confused with a raw uint8 at the type level.

**`Reserve`** (`:44-53`), 2 slots:

| Slot | Fields |
|---:|---|
| 0 | `address underlying` |
| 1 | `IHubBase hub` (address), `uint16 assetId`, `uint8 decimals`, `uint24 collateralRisk`, `ReserveFlags flags` (uint8), `uint32 dynamicConfigKey` |

Slot 1 packs 160+16+8+24+8+32 = 248 bits. A reserve is `(hub, assetId)` plus this
Spoke's own risk view of it. **`reserveId` is Spoke-local and independent of the
Hub's `assetId`** — two reserves on one Spoke may point at the same Hub asset with
different risk parameters only if they are different `(hub, assetId)` pairs, since
`addReserve` rejects duplicates (`Spoke.sol:130`).

**`ReserveConfig`** (`:61-67`): `collateralRisk`, `paused`, `frozen`, `borrowable`,
`receiveSharesEnabled`.

**`DynamicReserveConfig`** (`:73-77`): `uint16 collateralFactor`,
`uint32 maxLiquidationBonus`, `uint16 liquidationFee` — all bps.
`maxLiquidationBonus` is expressed with 100_00 meaning *zero* bonus (`:71`).

**Why "dynamic".** Each reserve keeps a monotonically increasing
`dynamicConfigKey`, and each user position stores the key it was last refreshed to.
Governance can publish a **new** config (`addDynamicReserveConfig`) that applies to
new activity while existing positions keep their old terms until refreshed. This is
v4's answer to "changing the collateral factor instantly liquidates people".
`_validateUpdateDynamicReserveConfig` (`Spoke.sol:852-860`) forbids setting a
historical key's `collateralFactor` to zero so liquidations against old keys can
still proceed.

**`LiquidationConfig`** (`:83-87`): `uint128 targetHealthFactor`,
`uint64 healthFactorForMaxBonus`, `uint16 liquidationBonusFactor`.

**`UserPosition`** (`:95-103`), 3 slots: `uint120 drawnShares`,
`uint120 premiumShares` | `int200 premiumOffsetRay` | `uint120 suppliedShares`,
`uint32 dynamicConfigKey`.

**`PositionManagerConfig`** (`:108-111`): `mapping(address user => bool) approval`
plus `bool active`. Two-sided: the Spoke admin activates a manager globally, and
each user approves it individually.

**`PositionStatus`** (`:116-119`): `mapping(uint256 bucket => uint256) map` plus
`uint24 riskPremium`.

**`UserAccountData`** (`:129-137`): `riskPremium`, `avgCollateralFactor`,
`healthFactor`, `totalCollateralValue`, `totalDebtValueRay`,
`activeCollateralCount`, `borrowCount`.

**Value units.** `SpokeUtils.toValue` (`SpokeUtils.sol:34-40`) returns
`amount · price · 10**(18 − decimals)`, and with `ORACLE_DECIMALS = 8`
(`SpokeUtils.sol:13`) this makes **1e26 == 1 USD**. That is why
`DUST_LIQUIDATION_THRESHOLD = 1000e26` (`LiquidationLogic.sol:185`) reads as
$1,000.

### 5.3 `src/spoke/libraries/ReserveFlagsMap.sol` (109 lines)

| Flag | Mask | Line | Blocks |
|---|---|---:|---|
| `paused` | `0x01` | 11 | everything, including liquidation |
| `frozen` | `0x02` | 13 | new activity: supply, borrow, enabling collateral |
| `borrowable` | `0x04` | 15 | (inverted) borrow requires it set |
| `receiveSharesEnabled` | `0x08` | 17 | liquidator taking shares instead of assets |

`create` (`:25`), `setPaused` (`:43`), `setFrozen` (`:51`), `setBorrowable` (`:59`),
`setReceiveSharesEnabled` (`:67`), getters `paused` (`:80`), `frozen` (`:87`),
`borrowable`, `receiveSharesEnabled`, and private `_setStatus` at the end.

### 5.4 `src/spoke/libraries/PositionStatusMap.sol` (252 lines)

**Two bits per reserve**, packed 128 reserves per word:

```
bit index for reserveId r inside its bucket:
  borrowing  -> (r % 128) * 2
  collateral -> (r % 128) * 2 + 1
bucket = r >> 7                                    (:195 bucketId)
reserveId = (bitId >> 1) + (bucket << 7)           (:203 fromBitId)
```

Masks (`:16-19`): `BORROWING_MASK = 0x5555...` (every even bit),
`COLLATERAL_MASK = 0xAAAA...` (every odd bit). `NOT_FOUND = type(uint256).max` (`:14`).

| Function | Line | Purpose |
|---|---:|---|
| `setBorrowing` | 22 | set/clear the even bit |
| `setUsingAsCollateral` | 38 | set/clear the odd bit |
| `isUsingAsCollateralOrBorrowing` | 54 | `(word >> shift) & 3 != 0` |
| `isBorrowing` | 64 | `& 1` |
| `isUsingAsCollateral` | 74 | `& 2` (shifted) |
| `collateralCount` | 86 | popcount across buckets, masked to `reserveCount` |
| `borrowCount` | 103 | same for borrow bits |
| `next` | 125 | previous set bit (either kind) strictly below `fromReserveId` |
| `nextBorrowing` | 152 | previous borrow bit |
| `nextCollateral` | 172 | previous collateral bit |
| `getBucketWord` | 187 | raw word |
| `bucketId` / `fromBitId` | 195 / 203 | index arithmetic |
| `isolateBorrowing` / `...Until` | 210 / 217 | mask helpers |
| `isolateUntil` | 228 | mask helpers |
| `isolateCollateral` / `...Until` | 236 / 243 | mask helpers |

The iterators walk **backwards** using Solady `LibBit.fls` (find-last-set), starting
from `_reserveCount` and descending, returning `NOT_FOUND` when exhausted. That is
why every loop in `Spoke.sol` is written as
`while ((reserveId = positionStatus.nextX(reserveId)) != NOT_FOUND)`.

The `...Until` variants mask off bits at or above `reserveCount` so that stale bits
from a since-shrunk reserve list cannot be read. The comments call these "dirty bits"
(`:84`, `:120`).

### 5.5 `src/spoke/libraries/KeyValueList.sol` (100 lines)

A packed `(key, value)` list used once, for sorting collateral by risk during
risk-premium computation.

Layout (`:24-28`): `KEY_BITS = 32` in the high bits, `VALUE_BITS = 224` in the low
bits, `KEY_SHIFT = 224`. The trick is at `pack` (`:69`):

```solidity
return ((MAX_KEY - key) << KEY_SHIFT) | value;
```

The key is stored **inverted**. Sorting the packed words in *descending* order
(`sortByKey` → `Arrays.sort(gtComparator)`, `:63`) therefore yields ascending key
and, within equal keys, descending value — exactly what the risk-premium loop wants
(cheapest-risk collateral first, largest chunk first). Uninitialized entries unpack
to `(0,0)` (`:90-93`) and, because a zero word is the smallest, land at the end.

`init` (`:31`), `length` (`:36`), `add` (`:42`, reverts `MaxDataSizeExceeded`),
`get` (`:49`), `uncheckedAt` (`:55`), `unpackKey` (`:75`), `unpackValue` (`:83`),
`unpack` (`:90`), `gtComparator` (`:97`).

### 5.6 `src/spoke/libraries/UserPositionUtils.sol` (165 lines)

`struct DebtComponents { drawnShares; premiumDebtRay; drawnIndex; }` (`:26-30`) —
one struct so a position's debt can be read with a single Hub call.

**`applyPremiumDelta(userPosition, premiumDelta)`** (`:35-45`) — adds `sharesDelta`
and `offsetRayDelta` to the user position. Always called *after* the Hub accepted
the same delta, so user and Hub stay in lockstep.

**`calculatePremiumDelta(userPosition, drawnSharesTaken, drawnIndex, riskPremium, restoredPremiumRay)`** (`:54-81`) — the heart of the premium system, quoted in §0.

**`calculateRestoreAmount(userPosition, drawnIndex, amount)`** (`:89-106`) — splits a
repayment. **Premium is always repaid first**:

```solidity
if (amount >= drawnDebt + premiumDebt) return (drawnDebt, premiumDebtRay);
if (amount < premiumDebt)              return (0, amount.toRay());
return (amount - premiumDebt, premiumDebtRay);
```

**`getDebt(userPosition, hub, assetId)`** (`:114-120`) and the index-taking overload
(`:127-133`), **`getDebtComponents`** (`:136-148`), **`_calculatePremiumRay`** (`:154-164`).

### 5.7 `src/spoke/libraries/SpokeUtils.sol` (41 lines)

`ORACLE_DECIMALS = 8` (`:13`). `get(reserves, reserveId)` (`:19-27`) is the
existence-checked accessor used everywhere — reverts `ReserveNotListed` if
`reserve.hub == address(0)`. `toValue(amount, decimals, price)` (`:34-40`) as
described above.

### 5.8 `src/spoke/Spoke.sol` (947 lines)

Inheritance (`:33-41`): `ISpoke, SpokeStorage, AccessManagedUpgradeable,
IntentConsumer, ExtSload, Multicall, ReentrancyGuardTransient`.

**Immutables and constants**

| Name | Line | Value |
|---|---:|---|
| `SET_USER_POSITION_MANAGERS_TYPEHASH` | 55 | from `EIP712Hash` |
| `MAX_USER_RESERVES_LIMIT` | 59 | immutable, constructor arg |
| `ORACLE` | 62 | immutable, constructor arg |
| `ORACLE_DECIMALS` | 65 | 8 |
| `MAX_ALLOWED_ASSET_ID` | 68 | `type(uint16).max` |
| `MAX_ALLOWED_COLLATERAL_RISK` | 71 | `1000_00` (i.e. 1000.00%) |
| `MAX_ALLOWED_DYNAMIC_CONFIG_KEY` | 74 | `type(uint32).max` |
| `MAX_ALLOWED_USER_RESERVES_LIMIT` | 77 | `type(uint16).max` (sentinel for "no limit") |
| `HEALTH_FACTOR_LIQUIDATION_THRESHOLD` | 81 | `1e18` |
| `DUST_LIQUIDATION_THRESHOLD` | 86 | `1000e26` ($1,000) |

**Modifier `onlyPositionManager(address onBehalfOf)`** (`:90-93`) —
`_isPositionManager(user, msg.sender)`, which returns true when `user == manager`
(`:910`), so ordinary self-service calls pass through the same gate.

**Constructor** (`:98-103`) — requires `IAaveOracle(oracle_).decimals() == 8`
→ `InvalidOracleDecimals`, and `maxUserReservesLimit_ > 0` →
`InvalidMaxUserReservesLimit`.

#### Admin functions (`restricted`)

**`updateLiquidationConfig(LiquidationConfig config)`** (`:109-118`) — requires
`targetHealthFactor >= 1e18`, `liquidationBonusFactor <= 100_00`, and
`healthFactorForMaxBonus < 1e18` → `InvalidLiquidationConfig`.

**`addReserve(address hub, uint256 assetId, address priceSource, ReserveConfig config, DynamicReserveConfig dynamicConfig) → uint256`** (`:121-165`)

Checks: non-zero hub; `assetId <= type(uint16).max` → `InvalidAssetId`; not already
listed → `ReserveExists`; `_validateReserveConfig`; `_validateDynamicReserveConfig`;
underlying non-zero → `AssetNotListed`; `decimals <= 18` → `InvalidAssetDecimals`.
Calls `IHubBase(hub).getAssetUnderlyingAndDecimals` (`:137`) and
`_updateReservePriceSource` → `IAaveOracle.setReserveSource` (`:673`).
Writes `_reserves[reserveId]` with `dynamicConfigKey = 0` and stores the config.
Emits `AddReserve`, `UpdateReserveConfig`, `AddDynamicReserveConfig`.

**`updateReserveConfig`** (`:168-182`), **`updateReservePriceSource`** (`:185-188`),
**`addDynamicReserveConfig`** (`:191-204`, increments the key, reverts
`MaximumDynamicConfigKeyReached`), **`updateDynamicReserveConfig`** (`:207-216`),
**`updatePositionManager(address, bool active)`** (`:219-222`).

**`_validateDynamicReserveConfig`** (`:921-930`) enforces the solvency relation that
makes liquidation always possible:

```solidity
require(config.collateralFactor < 100_00 &&
        config.maxLiquidationBonus >= 100_00 &&
        config.maxLiquidationBonus.percentMulUp(config.collateralFactor) < 100_00,
        InvalidCollateralFactorAndMaxLiquidationBonus());
require(config.liquidationFee <= 100_00, InvalidLiquidationFee());
```

`bonus × collateralFactor < 1` guarantees that seizing collateral worth
`debt × bonus` never exceeds the borrowing capacity that debt consumed — the
"liquidation penalty" denominator at `LiquidationLogic.sol:799-812` can never be
zero or negative.

#### User actions

**`supply(uint256 reserveId, uint256 amount, address onBehalfOf) → (uint256, uint256)`** (`:225-241`)

`nonReentrant`, `onlyPositionManager(onBehalfOf)`. `_validateSupply` (`:862-865`):
not paused → `ReservePaused`, not frozen → `ReserveFrozen`. Then:

```solidity
IERC20(reserve.underlying).safeTransferFrom(msg.sender, address(reserve.hub), amount);
uint256 suppliedShares = reserve.hub.add(reserve.assetId, amount);
userPosition.suppliedShares += suppliedShares.toUint120();
```

Tokens go **straight to the Hub**. Emits `Supply(reserveId, msg.sender, onBehalfOf, suppliedShares, amount)`.
No health-factor check — supplying can only help.

**`withdraw(uint256 reserveId, uint256 amount, address onBehalfOf) → (uint256, uint256)`** (`:244-271`)

`_validateWithdraw` (`:867-869`): not paused only — you may withdraw from a frozen
reserve. Caps the amount at the user's full position via
`hub.previewRemoveByShares(assetId, suppliedShares)` (`:255-258`), so
`type(uint256).max` works as "withdraw all". Calls `hub.remove(..., msg.sender)` —
**funds go to `msg.sender`, not `onBehalfOf`**, which is what makes a position
manager able to route the proceeds. If the reserve was collateral, runs
`_refreshAndValidateUserAccountData` (which reverts `HealthFactorBelowThreshold`)
and then `_notifyRiskPremiumUpdate` (`:263-266`).

**`borrow(uint256 reserveId, uint256 amount, address onBehalfOf) → (uint256, uint256)`** (`:274-302`)

`_validateBorrow` (`:871-876`): not paused, not frozen, `borrowable` →
`ReserveNotBorrowable`. Draws from the Hub to `msg.sender`. If this is a new borrow
reserve for the user, enforces `MAX_USER_RESERVES_LIMIT` on `borrowCount` →
`MaximumUserReservesExceeded` (`:288-292`) and sets the borrow bit. Always ends with
`_refreshAndValidateUserAccountData` + `_notifyRiskPremiumUpdate`, so the health
factor is checked *after* the debt exists.

**`repay(uint256 reserveId, uint256 amount, address onBehalfOf) → (uint256, uint256)`** (`:305-344`)

`_validateRepay` (`:878-880`): not paused. Splits the amount with
`calculateRestoreAmount` (premium first), computes `restoredShares` with
`rayDivDown`, builds the `PremiumDelta` at the user's *current* `riskPremium`,
transfers `totalDebtRestored` to the Hub, calls `hub.restore`, applies the delta to
the user position, and clears the borrow bit when `drawnShares` hits zero.
Emits `Repay(..., restoredShares, totalDebtRestored, premiumDelta)`.

**`liquidationCall(uint256 collateralReserveId, uint256 debtReserveId, address user, uint256 debtToCover, bool receiveShares)`** (`:347-388`)

`nonReentrant`, no position-manager gate — anyone may liquidate. Computes
`UserAccountData` first, packs `LiquidateUserParams`, delegates to
`LiquidationLogic.liquidateUser` (an **external library**, so this is a
`delegatecall` into linked code), then branches:

```solidity
if (isUserInDeficit) LiquidationLogic.notifyReportDeficit(...);   // write off ALL debt reserves
else                 _notifyRiskPremiumUpdate(user, _calculateUserAccountData(user).riskPremium);
```

**`setUsingAsCollateral(uint256 reserveId, bool usingAsCollateral, address onBehalfOf)`** (`:391-412`)

Short-circuits if unchanged (`:398-400`). `_validateSetUsingAsCollateral` (`:882-898`):
not paused; when enabling, not frozen and `collateralCount < MAX_USER_RESERVES_LIMIT`.
Enabling refreshes that reserve's dynamic config key (`_refreshDynamicConfig`, `:815-818`);
disabling runs the full health check.

**`updateUserRiskPremium(address onBehalfOf)`** (`:415-421`) and
**`updateUserDynamicConfig(address onBehalfOf)`** (`:424-430`) — callable by an
approved position manager, or by anyone holding the role, via the explicit fallback
`_checkCanCall(msg.sender, msg.data)` (`:417`, `:426`). The first recomputes the
premium at the user's *existing* config keys; the second refreshes the keys to the
latest published config first — i.e. it is the function that migrates a user onto
new risk parameters, and it reverts if that would put them below the threshold.

**`setUserPositionManager(address, bool)`** (`:433-435`),
**`setUserPositionManagersWithSig(SetUserPositionManagers, bytes)`** (`:438-457`,
EIP-712 via `_verifyAndConsumeIntent`), **`renouncePositionManagerRole(address)`**
(`:460-466`, a manager dropping its own approval), **`permitReserve(...)`**
(`:469-492`, a `try/catch`-wrapped ERC-2612 permit so a front-run permit does not
brick a multicall).

#### The account-data engine

**`_processUserAccountData(address user, bool refreshConfig) → UserAccountData`** (`:706-813`)

One backward pass over the user's set bits, then two derived quantities.

```
reserveId = _reserveCount
loop: (reserveId, borrowing, collateral) = positionStatus.next(reserveId)   :719
      break on NOT_FOUND
      price = IAaveOracle(ORACLE).getReservePrice(reserveId)                :725
      if collateral and collateralFactor > 0 and suppliedShares > 0:
          value = hub.previewRemoveByShares(...).toValue(decimals, price)   :738-741
          totalCollateralValue += value
          collateralInfo.add(idx, reserve.collateralRisk, value)            :743-747
          avgCollateralFactor += collateralFactor * value                   :748
          activeCollateralCount++
      if borrowing:
          debtRay = drawnShares*drawnIndex + premiumDebtRay                 :759-760
          totalDebtValueRay += debtRay.toValue(decimals, price)             :761-764
          borrowCount++
```

`refreshConfig == true` writes `userPosition.dynamicConfigKey = reserve.dynamicConfigKey`
inline inside the index expression (`:731`) — that assignment is the only state
change in this function, which is why the read-only path is safe to cast to `view`.

**Health factor** (`:773-778`):

```solidity
healthFactor = mulDiv(avgCollateralFactor.bpsToWad(), RAY, totalDebtValueRay, Floor)
```

At this point `avgCollateralFactor` is still the *sum* of `collateralFactor × value`
(bps × Value). Converting bps→WAD and dividing by a ray-scaled debt yields a
WAD-scaled health factor. With no debt it is `type(uint256).max` (`:780`).
Only after that is `avgCollateralFactor` normalized into a true average (`:783-786`).

**Risk premium** (`:788-810`) — the part that is genuinely new:

```solidity
collateralInfo.sortByKey();                      // ascending collateralRisk
uint256 totalDebtValue = totalDebtValueRay.fromRayUp();
uint256 debtValueLeftToCover = totalDebtValue;
for each (collateralRisk, userCollateralValue) in collateralInfo:
    if debtValueLeftToCover == 0 break;
    userCollateralValue = min(userCollateralValue, debtValueLeftToCover);
    riskPremium += userCollateralValue * collateralRisk;
    debtValueLeftToCover -= userCollateralValue;
if (debtValueLeftToCover < totalDebtValue)
    riskPremium = riskPremium.divUp(totalDebtValue - debtValueLeftToCover);
```

The user's debt is notionally backed by their **safest collateral first**. The
resulting premium is the value-weighted average `collateralRisk` of just the
collateral needed to cover the debt. Post excellent collateral and your premium
falls; post the same debt against volatile collateral and it rises. Excess
collateral beyond the debt is ignored, so over-collateralizing with a risky asset
does not penalize you.

**`_refreshAndValidateUserAccountData`** (`:686-696`) — `_processUserAccountData(user, true)`,
emits `RefreshAllUserDynamicConfig`, requires `healthFactor >= 1e18` →
`HealthFactorBelowThreshold`.

**`_calculateUserAccountData`** (`:699-701`) — the non-refreshing variant.

**`getUserAccountData(address)`** (`:633-636`) uses the `_castToView` assembly trick
(`:936-946`) to expose the same code as a `view`, safe only because `refreshConfig`
is `false`. The comment at `:634` says so explicitly.

**`_notifyRiskPremiumUpdate(address user, uint256 newRiskPremium)`** (`:822-849`)

Skips entirely if the premium is zero and was already zero (`:824-826`). Otherwise
stores the new premium and walks every borrowed reserve, computing a `PremiumDelta`
with `drawnSharesTaken: 0` and `restoredPremiumRay: 0` and pushing it to the Hub via
`refreshPremium`. Each iteration is a Hub call, which is why the gas snapshot shows
`updateUserRiskPremium: 2 borrows` at 210k versus 158k for one borrow.

#### Spoke views

`getLiquidationConfig` (`:495`), `getReserveCount` (`:500`), `getReserveSuppliedAssets`
(`:505`), `getReserveSuppliedShares` (`:511`), `getReserveDebt` (`:517`),
`getReserveTotalDebt` (`:523`), `getReserveId` (`:529`), `getReserve` (`:536`),
`getReserveConfig` (`:541`), `getDynamicReserveConfig` (`:554`),
`getUserReserveStatus` (`:563`), `getUserSuppliedAssets` (`:573`),
`getUserSuppliedShares` (`:583`), `getUserDebt` (`:589`), `getUserTotalDebt` (`:600`),
`getUserPremiumDebtRay` (`:611`), `getUserPosition` (`:619`),
`getUserLastRiskPremium` (`:628`), `getUserAccountData` (`:633`),
`getLiquidationBonus` (`:639`), `isPositionManagerActive` (`:657`),
`isPositionManager` (`:662`), `getLiquidationLogic` (`:667`, returns the linked
library address).

### 5.9 `src/spoke/SpokeConfigurator.sol` (298 lines)

`AccessManaged`, same facade pattern as `HubConfigurator`, `spoke` as first arg.

Price/liquidation: `updateReservePriceSource` (`:23`),
`updateLiquidationTargetHealthFactor` (`:32`), `updateHealthFactorForMaxBonus` (`:43`),
`updateLiquidationBonusFactor` (`:54`), `updateLiquidationConfig` (`:65`).

Reserves: `addReserve` (`:73`), `updatePaused` (`:85`), `updateFrozen` (`:93`),
`updateBorrowable` (`:101`), `updateReceiveSharesEnabled` (`:109`),
`updateCollateralRisk` (`:121`).

Dynamic config, each in add/update pairs so a parameter can be published as a new
key or edited in place: `addCollateralFactor` (`:133`) / `updateCollateralFactor`
(`:148`), `addMaxLiquidationBonus` (`:164`) / `updateMaxLiquidationBonus` (`:179`),
`addLiquidationFee` (`:195`) / `updateLiquidationFee` (`:210`),
`addDynamicReserveConfig` (`:226`) / `updateDynamicReserveConfig` (`:235`).

Emergency: `pauseAllReserves` (`:245`), `freezeAllReserves` (`:256`),
`pauseReserve` (`:267`), `freezeReserve` (`:275`).
Plus `updatePositionManager` (`:283`) and internal
`_getReserveLastDynamicConfigKey` (`:292`).

### 5.10 `src/spoke/instances/SpokeInstance.sol` (33 lines)

`SPOKE_REVISION = 1`. Constructor takes `(oracle_, maxUserReservesLimit_)` and
disables initializers. `initialize(authority)` emits `SetSpokeImmutables`, inits
access control, and **defaults `targetHealthFactor` to 1e18 if unset**, emitting
`UpdateLiquidationConfig`. The doc comment at `:14` warns that on upgrade the new
oracle must support the existing reserves — the oracle is immutable per
implementation, so replacing it is an implementation swap.

---

## §6 LiquidationLogic

`src/spoke/libraries/LiquidationLogic.sol` (827 lines). An **external** library, so
`Spoke` reaches it by `delegatecall` and its address is queryable via
`Spoke.getLiquidationLogic()` (`Spoke.sol:667-669`). It runs in the Spoke's storage
context, which is why it takes storage mappings as parameters.

Constants: `HEALTH_FACTOR_LIQUIDATION_THRESHOLD = 1e18` (`:182`),
`DUST_LIQUIDATION_THRESHOLD = 1000e26` (`:185`) — $1,000 in Value units.

Ten parameter structs (`:32-179`) exist purely to dodge stack-too-deep; they carry
no logic. The interesting ones are `LiquidationAmounts` (`:174-179`, the four
outputs) and `DebtComponents` from `UserPositionUtils`.

### 6.1 Entry points

**`liquidateUser(reserves, userPositions, positionStatus, dynamicConfig, params) → bool`** (`:194-250`)

Resolves both reserves, reads the **user's stored** `dynamicConfigKey` for the
collateral reserve (`:207-209`) — so liquidation uses the terms the position was
opened under, not the latest ones — packs `ExecuteLiquidationParams` and calls
`_executeLiquidation`. Returns "this liquidation left the user in deficit".

**`notifyReportDeficit(reserves, userPositions, positionStatus, reserveCount, user)`** (`:260-303`)

Called by `Spoke.liquidationCall` when the return value is true. Zeroes
`riskPremium` (`:268`) then walks **every** borrowed reserve and writes the whole
debt off:

```solidity
premiumDelta = userPosition.calculatePremiumDelta({
  drawnSharesTaken: debtComponents.drawnShares,   // all of it
  drawnIndex: ..., riskPremium: 0,
  restoredPremiumRay: debtComponents.premiumDebtRay });
hub.reportDeficit(assetId, drawnShares.rayMulUp(drawnIndex), premiumDelta);
userPosition.drawnShares -= debtComponents.drawnShares;
positionStatus.setBorrowing(reserveId, false);
```

Emits `ISpoke.ReportDeficit` per reserve and one `UpdateUserRiskPremium(user, 0)`.
The user's position is left completely clean; the loss sits in the Hub's
`deficitRay` awaiting `eliminateDeficit`.

**`calculateLiquidationBonus(healthFactorForMaxBonus, liquidationBonusFactor, healthFactor, maxLiquidationBonus) → uint256`** (`:312-333`) — `public pure`, also exposed through `Spoke.getLiquidationBonus`.

```
if healthFactor <= healthFactorForMaxBonus: return maxLiquidationBonus
minBonus = (maxBonus - 100_00) * liquidationBonusFactor / 100_00 + 100_00
return minBonus + (maxBonus - minBonus) * (1e18 - HF) / (1e18 - HFforMaxBonus)
```

A **linear ramp**: the bonus is minimal at HF just under 1 and grows to the maximum
as HF falls to `healthFactorForMaxBonus`. This is the mechanism that stops a
position at HF 0.999 from paying a full 5-10% penalty. The denominator is safe
because `updateLiquidationConfig` enforces `healthFactorForMaxBonus < 1e18`
(`Spoke.sol:113`).

### 6.2 `_executeLiquidation` (`:342-446`)

```
1. read suppliedShares and DebtComponents
2. _validateLiquidationCall
3. _calculateLiquidationAmounts  -> LiquidationAmounts
4. _liquidateCollateral          -> amountRemoved, isCollateralPositionEmpty
5. _liquidateDebt                -> amountRestored, premiumDelta, isDebtPositionEmpty
6. emit ISpoke.LiquidationCall
7. return _evaluateDeficit(...)
```

**`_validateLiquidationCall` (`:533-559`)** — in order:

| Check | Error |
|---|---|
| `user != liquidator` | `SelfLiquidation` |
| `debtToCover > 0` | `InvalidDebtToCover` |
| neither reserve paused | `ReservePaused` |
| `suppliedShares > 0` | `ReserveNotSupplied` |
| `drawnShares > 0` | `ReserveNotBorrowed` |
| `healthFactor < 1e18` | `HealthFactorNotBelowThreshold` |
| `collateralFactor > 0 && isUsingAsCollateral` | `ReserveNotEnabledAsCollateral` |
| if `receiveShares`: collateral not frozen and `receiveSharesEnabled` | `CannotReceiveShares` |

The comment at `:541-542` records a useful invariant: a user has active debt iff
they have drawn shares, because premium is always repaid first and only exists
alongside drawn shares.

**`_liquidateCollateral` (`:450-489`)** — reduces the user's `suppliedShares`, then
either credits `sharesToLiquidator` to the liquidator's own position on this Spoke
(`receiveShares == true`, `:466`) or calls `hub.remove` to send underlying
(`:470-475`). The difference `sharesToLiquidate − sharesToLiquidator` is the
protocol's liquidation fee and goes to the treasury with
`hub.payFeeShares(assetId, feeShares)` (`:479-482`).

**`_liquidateDebt` (`:493-529`)** — builds the premium delta at the user's current
`riskPremium`, pulls `drawnAmount + premium` from the **liquidator** to the Hub
(`:507-511`), calls `hub.restore`, applies the delta, decrements drawn shares, and
clears the borrow bit when it reaches zero.

**`_evaluateDeficit` (`:816-826`)**

```solidity
if (!isCollateralPositionEmpty || activeCollateralCount > 1) return false;
return !isDebtPositionEmpty || borrowCount > 1;
```

Deficit is declared only when the liquidated collateral reserve was the user's
**last** collateral *and* debt still remains anywhere. That is the correct condition:
no collateral left, debt outstanding.

### 6.3 The sizing calculation

**`_calculateDebtToTargetHealthFactor` (`:795-813`)** — the classic close-factor
formula, restated for v4:

```
liquidationPenalty = bpsToWad(liquidationBonus) * collateralFactor / 100_00
debtRay = totalDebtValueRay * debtAssetUnit * (targetHF - HF)
          / ((targetHF - liquidationPenalty) * toWad(debtAssetPrice))     [ceil]
```

Repaying this much debt moves the position's health factor to
`targetHealthFactor`. The denominator is positive because
`_validateDynamicReserveConfig` guarantees
`liquidationBonus × collateralFactor < 100_00` (§5.8).

**`_calculateDebtToLiquidate` (`:735-791`)**

1. Compute `debtRayToTarget`.
2. Take premium first: `premiumDebtRayToLiquidate = roundRayUp(debtRayToTarget).min(premiumDebtRay)` (`:751`), then clamp to `debtToCover` (`:753-755`).
3. Only if premium was fully consumed *and* more is needed, compute drawn shares as `min(sharesToTarget, sharesToCover, drawnShares)` (`:757-773`).
4. **Dust guard** (`:775-788`): if the remaining debt would be worth less than $1,000 and not all drawn shares were taken, override and liquidate everything — `targetHealthFactor` is deliberately bypassed.

**`_calculateCollateralToLiquidate` (`:707-729`)** — converts debt-to-liquidate into
collateral shares, applying the bonus, via OZ `Math.mulDiv` with `Floor`:

```
collateral = debtRay * debtPrice * collateralUnit * bonus
             / (debtUnit * collateralPrice * 100_00 * RAY)
shares = hub.previewAddByAssets(assetId, collateral)
```

**`_calculateLiquidationAmounts` (`:563-703`)** — the orchestrator, and the
messiest 140 lines in the codebase. Its job is stated in the comment at `:576-579`:
enforce that a liquidation either clears all debt, clears all collateral, or leaves
at least $1,000 of both.

- Computes the bonus, the debt to liquidate, the collateral for it.
- `leavesCollateralDust` (`:612-623`): would the remaining collateral be under $1,000?
- If collateral is insufficient **or** dust would remain (`:626-629`), pin
  `collateralSharesToLiquidate = suppliedShares` and back-solve the debt from the
  collateral (`:635-646`), then re-split it premium-first (`:648-680`), with two
  rounding rescues: cap drawn shares at the user's total (`:659-661`) and
  recompute-and-cap collateral (`:665-678`).
- **`MustNotLeaveDust` guard** (`:684-688`): the liquidator's stated `debtToCover`
  must be at least the computed amount, otherwise revert. A liquidator cannot opt
  into a partial liquidation that would strand dust.
- Fee split (`:690-694`):
  ```solidity
  collateralSharesToLiquidator = collateralSharesToLiquidate
    - collateralSharesToLiquidate.mulDivUp(
        liquidationFee * (liquidationBonus - 100_00),
        liquidationBonus * 100_00);
  ```
  The fee is taken as a percentage **of the bonus only**, never of principal. With
  `liquidationBonus == 100_00` (no bonus) the fee is exactly zero.

---

## §7 TokenizationSpoke and TreasurySpoke

### 7.1 `src/spoke/TokenizationSpoke.sol` (446 lines)

`abstract contract TokenizationSpoke is ITokenizationSpoke, ERC20Upgradeable, IntentConsumer` (`:18`).
A full ERC-4626 vault over exactly one Hub asset. It registers with the Hub as a
Spoke, so from the Hub's perspective it is just another supplier.

**Is it rebasing? No.** Settled by the code: it inherits `ERC20Upgradeable` and
mints/burns real share balances (`_mint` at `:393`, `_burn` at `:411`). `balanceOf`
is the plain ERC-20 balance. Value accrues because `convertToAssets`
(`:262-264`) → `previewRedeem` → `HUB.previewRemoveByShares` returns more assets per
share over time. This is the v4 equivalent of v3's `StataTokenV2`, promoted into the
core protocol.

Immutables (`:36-43`): `MAX_ALLOWED_SPOKE_CAP`, `HUB`, `ASSET_ID`, `ASSET`,
`DECIMALS`, `ASSET_UNITS`. The constructor (`:48-54`) resolves the asset by calling
`HUB.getAssetId(underlying_)`, which **reverts if the asset is not listed** — so a
vault cannot exist for an unlisted asset.

Typehash constants (`:24-36`): `PERMIT_NONCE_NAMESPACE = 0`, `PERMIT_TYPEHASH`,
`DEPOSIT_TYPEHASH`, `MINT_TYPEHASH`, `WITHDRAW_TYPEHASH`, `REDEEM_TYPEHASH`.

| Function | Line | Notes |
|---|---:|---|
| `deposit(assets, receiver)` | 68 | → `_executeDeposit` |
| `mint(shares, receiver)` | 73 | → `_executeMint` |
| `withdraw(assets, receiver, owner)` | 78 | → `_executeWithdraw` |
| `redeem(shares, receiver, owner)` | 87 | → `_executeRedeem` |
| `depositWithSig` | 96 | EIP-712, signer = `params.depositor` |
| `mintWithSig` | 116 | |
| `withdrawWithSig` | 132 | signer = `params.owner`; caller forced to `owner` (`:145`) |
| `redeemWithSig` | 153 | same |
| `depositWithPermit` | 174 | `try/catch` permit then deposit |
| `permit` | 197 | EIP-2612 over the **share** token |
| `usePermitNonce` | 224 | burn a nonce to cancel a signature |
| `renounceAllowance(owner)` | 229 | a spender dropping its own allowance |

Previews (`:237-254`) map one-to-one onto Hub previews, so vault rounding is Hub
rounding: `previewDeposit`→`previewAddByAssets`, `previewMint`→`previewAddByShares`,
`previewWithdraw`→`previewRemoveByAssets`, `previewRedeem`→`previewRemoveByShares`.

`maxDeposit` (`:267-278`) returns 0 when the Spoke is inactive or halted, `max` when
uncapped, else `addCap·ASSET_UNITS − previewMint(totalSupply())` via
`zeroFloorSub`. `maxWithdraw`/`maxRedeem` (`:290-301`) are bounded by
`_maxRemovableAssets` (`:435-441`), which is the Hub's **available liquidity** — so
the vault correctly reports zero withdrawable when the Hub is fully drawn.

Internal workflow: `_deposit` (`:386-396`) = `_pullAndDepositAssets` → `_mint` →
`_afterDeposit` hook → `Deposit` event; `_withdraw` (`:400-414`) = allowance spend →
`_beforeWithdraw` hook → `_burn` → `_removeAndPushAssets` → `Withdraw` event.
`_pullAndDepositAssets` (`:418-421`) transfers **to the Hub** then calls `HUB.add`.
The `_afterDeposit`/`_beforeWithdraw` hooks (`:430-433`) are empty and `virtual`,
the designed extension point for a yield-bearing variant.

`_domainNameAndVersion` returns `('Tokenization Spoke', '1')` (`:443-445`).

### 7.2 `src/spoke/TreasurySpoke.sol` (94 lines)

`abstract contract TreasurySpoke is ITreasurySpoke, Ownable2StepUpgradeable` (`:15`).
The default `feeReceiver`. Not a lending market — it is a single-owner wallet that
happens to be registered as a Spoke, so protocol fees minted as added shares land
somewhere that can withdraw them.

`supply` (`:22`), `supplySkimmed` (`:31`), `withdraw` (`:40`, caps at the position so
`max` means "all"), `transfer` (`:58`, arbitrary ERC-20 rescue), `getSuppliedAssets`
(`:63`), `getSuppliedShares` (`:70`), internal `_supply` (`:78-93`).

`supplySkimmed` is the sanctioned way to claim untracked tokens sitting on the Hub:
it skips the `transferFrom` and calls `HUB.add` directly, so the Hub's balance check
passes using the stray balance (§3.4).

### 7.3 Instances

`TokenizationSpokeInstance.sol` (30 lines) — `SPOKE_REVISION = 1`, constructor
`(hub_, underlying_)`, `initialize(shareName, shareSymbol)`.
`TreasurySpokeInstance.sol` (23 lines) — `initialize(owner)` running
`__Ownable_init` + `__Ownable2Step_init`.

---

## §8 Oracle and interest rate strategy

### 8.1 `src/spoke/AaveOracle.sol` (89 lines)

**Oracles are Spoke-specific**, stated at `:11`, because `_sources` is keyed by
`reserveId` and `reserveId` is Spoke-local. Two Spokes cannot share one oracle.

Immutables: `DECIMALS` (`:14`), `DEPLOYER` (`:17`). Storage: `address public spoke`
(`:20`), `mapping(uint256 => IPriceFeed) _sources` (`:23`).

**`setSpoke(address spoke_)`** (`:34-41`) — one-shot binding. Requires
`msg.sender == DEPLOYER` → `OnlyDeployer`; non-zero → `InvalidAddress`;
`spoke == address(0)` → `SpokeAlreadySet`; and a **reciprocity check**
`ISpoke(spoke_).ORACLE() == address(this)` → `OracleMismatch`. Emits `SetSpoke`.

**`setReserveSource(uint256 reserveId, address source)`** (`:44-51`) — only the bound
Spoke → `OnlySpoke`. Requires `source.decimals() == DECIMALS` →
`InvalidSourceDecimals(reserveId)`, then **immediately reads the price**
(`_getSourcePrice`, `:49`) so a dead feed is rejected at configuration time rather
than at liquidation time.

Views: `decimals` (`:54`), `getReservePrice` (`:59`), `getReservesPrices` (`:64`),
`getReserveSource` (`:75`). Internal `_getSourcePrice` (`:80-88`) requires a set
source → `InvalidSource(reserveId)` and `price > 0` → `InvalidPrice(reserveId)`.

There is **no fallback oracle and no staleness check** — no `updatedAt` is read.
`latestAnswer()` is trusted as long as it is positive. Feed liveness is an
operational assumption.

### 8.2 `src/hub/AssetInterestRateStrategy.sol` (138 lines)

**Hub-specific**, keyed by `assetId` (`:13`). Immutable `HUB` (`:27`).

Constants: `MAX_ALLOWED_DRAWN_RATE = 1000_00` (1000%, `:18`),
`MIN_OPTIMAL_RATIO = 1_00`, `MAX_OPTIMAL_RATIO = 99_00` (`:21-24`).

**`InterestRateData`** (`IAssetInterestRateStrategy.sol:15-20`), all bps, one slot:
`uint16 optimalUsageRatio`, `uint32 baseDrawnRate`, `uint32 rateGrowthBeforeOptimal`,
`uint32 rateGrowthAfterOptimal`.

**`setInterestRateData(uint256 assetId, bytes data)`** (`:42-66`) — `OnlyHub`;
decodes the struct; requires `1_00 ≤ optimalUsageRatio ≤ 99_00` →
`InvalidOptimalUsageRatio`; requires the three rate components to sum to at most
1000_00 → `InvalidMaxDrawnRate`. Emits `UpdateInterestRateData`.

**`calculateInterestRate(assetId, liquidity, drawn, deficit, swept)`** (`:102-137`)

```solidity
require(rateData.optimalUsageRatio > 0, InterestRateDataNotSet(assetId));
uint256 rate = baseDrawnRate.bpsToRay();
if (drawn == 0) return rate;
uint256 U = drawn.rayDivUp(liquidity + drawn + swept);
uint256 Uopt = optimalUsageRatio.bpsToRay();
if (U <= Uopt) rate += growthBefore.bpsToRay().rayMulUp(U).rayDivUp(Uopt);
else           rate += growthBefore.bpsToRay()
                     + growthAfter.bpsToRay().rayMulUp(U - Uopt).rayDivUp(RAY - Uopt);
```

The familiar two-slope kink. Two details that differ from v3:

- **`deficit` is accepted and ignored** — the parameter name is commented out at
  `:106`. Written-off bad debt does not push the rate up.
- **`swept` is in the denominator** (`:117`), so reinvested liquidity still counts as
  available. Sweeping funds to a strategy does not artificially spike the borrow rate.

Utilization rounds **up** (`rayDivUp`) and both slope terms round up, so the rate is
biased toward suppliers.

Views: `getInterestRateData` (`:69`), `getOptimalUsageRatio` (`:74`),
`getBaseDrawnRate` (`:79`), `getRateGrowthBeforeOptimal` (`:84`),
`getRateGrowthAfterOptimal` (`:89`), `getMaxDrawnRate` (`:94`, the sum).

**There is no supply rate.** v3 computed `liquidityRate = borrowRate · U · (1 − reserveFactor)`.
v4 has no such value anywhere; supplier yield is whatever `totalAddedAssets` growth
produces, which is the same quantity arrived at from the other direction.

---

## §9 Position managers

A **position manager** is a contract a user authorizes to act on their Spoke
position. Authorization is two-sided and both sides must hold:

1. Spoke admin marks the manager globally active — `Spoke.updatePositionManager` (`Spoke.sol:219-222`), `restricted`.
2. The user approves it — `Spoke.setUserPositionManager` (`:433`) or `setUserPositionManagersWithSig` (`:438`).

`Spoke._isPositionManager` (`:909-913`) checks both, and short-circuits `true` when
`user == manager`. Managers can also drop their own approval unilaterally
(`Spoke.renouncePositionManagerRole`, `:460`).

### 9.1 `src/position-manager/PositionManagerBase.sol` (125 lines)

`Ownable2Step, Rescuable, Multicall`. Storage: `mapping(address spoke => bool) _registeredSpokes` (`:20`).
Modifier `onlyRegisteredSpoke` → `SpokeNotRegistered` (`:23-26`).

| Function | Line | Notes |
|---|---:|---|
| `registerSpoke(spoke, registered)` | 33 | `onlyOwner`, non-zero, emits `RegisterSpoke` |
| `setSelfAsUserPositionManagerWithSig(...)` | 40 | forwards a user signature to the Spoke, wrapped in `try/catch` so a front-run does not revert a batch |
| `permitReserveUnderlying(...)` | 64 | `try/catch` ERC-2612 permit on the reserve's underlying |
| `renouncePositionManagerRole(spoke, user)` | 89 | `onlyOwner` |
| `multicall(bytes[])` | 97 | gated on `_multicallEnabled()` → `UnsupportedAction` |
| `isSpokeRegistered` | 105 | |

`_rescueGuardian()` returns `owner()` (`:122-124`), wiring `Rescuable`.

### 9.2 `src/position-manager/GiverPositionManager.sol` (71 lines)

Actions that can only **help** a position, so they need no user approval beyond
Spoke registration.

**`supplyOnBehalfOf(spoke, reserveId, amount, onBehalfOf)`** (`:20-39`) — pulls from
`msg.sender`, `forceApprove`s the Spoke, calls `ISpoke.supply`. Emits
`SupplyOnBehalfOf`.

**`repayOnBehalfOf(spoke, reserveId, amount, onBehalfOf)`** (`:42-66`) — rejects
`type(uint256).max` → `RepayOnBehalfMaxUintNotAllowed` (`:48`, so a caller cannot
accidentally approve unbounded pull), caps at `getUserTotalDebt`, pulls exactly that,
repays. Emits `RepayOnBehalfOf`. Multicall enabled (`:68-70`).

### 9.3 `src/position-manager/TakerPositionManager.sol` (342 lines)

Actions that **remove value**, so they carry per-`(spoke, reserveId, owner, spender)`
allowances — v4's credit delegation, generalized to withdrawals as well.

Storage (`:26-31`): `_withdrawAllowances` and `_borrowAllowances`, both
quadruple-nested mappings.

Typehashes: `WITHDRAW_PERMIT_TYPEHASH` (`:20`), `BORROW_PERMIT_TYPEHASH` (`:23`).

| Function | Line |
|---|---:|
| `approveWithdraw` | 38 |
| `approveWithdrawWithSig` | 54 |
| `approveBorrow` | 76 |
| `approveBorrowWithSig` | 92 |
| `renounceWithdrawAllowance` | 114 |
| `renounceBorrowAllowance` | 139 |
| `withdrawOnBehalfOf` | 164 |
| `borrowOnBehalfOf` | 223 |
| `withdrawAllowance` (view) | 274 |
| `borrowAllowance` (view) | 285 |

**The allowance-accounting subtlety** (`:190-207`) is worth reading in full, because
it is a real consequence of share-based accounting. Decrementing the allowance by the
requested `amount` would be wrong twice over: Hub rounding means the position moves
by slightly more or less than `amount`, and `Spoke.withdraw` caps at the full
position so `withdrawnAmount` can be smaller. The fix measures the position before
and after and consumes the delta:

```solidity
uint256 suppliedAssetsAfter = ISpoke(spoke).getUserSuppliedAssets(reserveId, onBehalfOf);
_updateWithdrawAllowance({..., newAllowance:
  currentAllowance.zeroFloorSub(suppliedAssetsBefore - suppliedAssetsAfter)});
```

`type(uint256).max` is an infinite allowance and skips the two extra `getUserSuppliedAssets`
calls entirely (`:180-182`, `:190`). Proceeds are forwarded to `msg.sender` (`:208`),
because `Spoke.withdraw` sends to its own caller (the manager).

`_domainNameAndVersion` (`:339`) gives this contract its own EIP-712 domain, distinct
from the Spoke's.

### 9.4 `src/position-manager/ConfigPositionManager.sol` (472 lines)

Delegates **configuration** rights, not value. Permissions are a 3-bit bitmap
(`libraries/ConfigPermissionsMap.sol`):

| Permission | Mask | Line |
|---|---|---:|
| `canSetUsingAsCollateral` | `0x1` | 16 |
| `canUpdateUserRiskPremium` | `0x2` | 18 |
| `canUpdateUserDynamicConfig` | `0x4` | 20 |
| `GLOBAL_PERMISSIONS_MASK` | `0x7` | 22 |

Library functions: `setGlobalPermissions` (`:27`), `setCanSetUsingAsCollateral`
(`:35`), `setCanUpdateUserRiskPremium` (`:49`), `setCanUpdateUserDynamicConfig`
(`:63`), `getConfigPermissionValues` (`:76`), the three getters (`:90`, `:97`, `:104`),
`eq` (`:112`), private `_setStatus`/`_getStatus` (`:117-129`).

Four typehashes (`:21-35`), four direct setters (`:45`, `:59`, `:73`, `:87`), four
`WithSig` variants (`:101`, `:121`, `:141`, `:161`), four renounce functions
(`:181`, `:201`, `:221`, `:241`), three acting functions —
`setUsingAsCollateralOnBehalfOf` (`:261`), `updateUserRiskPremiumOnBehalfOf` (`:290`),
`updateUserDynamicConfigOnBehalfOf` (`:306`) — a view `getConfigPermissions` (`:322`),
and internals (`:337-463`). Error: `DelegateeNotAllowed`.

The use case: a delegatee that can refresh your dynamic config onto new governance
parameters, or toggle collateral, without ever being able to move your funds.

### 9.5 `src/position-manager/SignatureGateway.sol` (209 lines)

A **gasless relay**. It holds no allowances of its own; it verifies a user's EIP-712
signature and then calls the Spoke as an approved position manager. Seven typehashes
(`:21-43`) and seven functions:

| Function | Line | Typehash |
|---|---:|---|
| `supplyWithSig` | 49 | `Supply(address spoke,uint256 reserveId,uint256 amount,address onBehalfOf,uint256 nonce,uint256 deadline)` |
| `withdrawWithSig` | 72 | `Withdraw(...)` same shape |
| `borrowWithSig` | 99 | `Borrow(...)` |
| `repayWithSig` | 126 | `Repay(...)` |
| `setUsingAsCollateralWithSig` | 154 | `SetUsingAsCollateral(address spoke,uint256 reserveId,bool useAsCollateral,address onBehalfOf,uint256 nonce,uint256 deadline)` |
| `updateUserRiskPremiumWithSig` | 171 | `UpdateUserRiskPremium(address spoke,address onBehalfOf,uint256 nonce,uint256 deadline)` |
| `updateUserDynamicConfigWithSig` | 187 | `UpdateUserDynamicConfig(...)` |

### 9.6 `src/position-manager/NativeTokenGateway.sol` (167 lines)

ETH ↔ WETH wrapper. Immutable `NATIVE_TOKEN_WRAPPER` (`:19`).

```solidity
receive() external payable { require(msg.sender == NATIVE_TOKEN_WRAPPER, UnsupportedAction()); }
fallback() external payable { revert UnsupportedAction(); }
```

(`:33-40`) — only the wrapper may push ETH in, so a stray send cannot be stranded.

`supplyNative` (`:43`), `supplyAsCollateralNative` (`:53`), `withdrawNative` (`:71`),
`borrowNative` (`:91`), `repayNative` (`:111`), internal `_supplyNative` (`:144`),
`_validateParams` (`:158`, → `NotNativeWrappedAsset`). Every payable entry requires
`msg.value == amount` → `NativeAmountMismatch`.

**Multicall is deliberately disabled** (`_multicallEnabled() => false`, `:164`), with
the reason in the comment at `:163`: `msg.value` is visible to every delegatecall in
a batch, so a multicall could spend the same ETH twice.

### 9.7 `src/position-manager/PositionManagerIntentBase.sol` (20 lines)

`PositionManagerBase + IntentConsumer`. Nothing but the combination.

---

## §10 Access control

### 10.1 `src/access/AccessManagerEnumerable.sol` (397 lines)

`contract AccessManagerEnumerable is AccessManager, IAccessManagerEnumerable` (`:13`).
Standard OZ `AccessManager` semantics — roles are `uint64`, permissions are
`(target, selector) → roleId`, grants carry execution delays — plus **enumeration**,
which vanilla `AccessManager` lacks entirely (it is event-sourced only).

Tracked sets (`:36-46`): roles, admin roles, role members, role→targets,
(target,selector)→role, role→target→selectors, and role labels.
`ADMIN_ROLE` (0) is excluded from tracking (`:11`).

Enumeration surface — every one is a view over an `EnumerableSet`:
`getRole` (`:70`), `getRoleCount` (`:75`), `getRoles` (`:80`), `isRole` (`:90`),
`getAdminRole` (`:95`), `getAdminRoleCount` (`:100`), `getAdminRoles` (`:105`),
`isAdminRole` (`:115`), `getRoleMember` (`:120`), `getRoleMemberCount` (`:125`),
`getRoleMembers` (`:130`), `getRoleOfAdminRole` (`:139`), `getRoleOfAdminRoleCount`
(`:144`), `getRolesOfAdminRole` (`:149`), `getRoleTarget` (`:163`),
`getRoleTargetCount` (`:168`), `getRoleTargets` (`:173`), `getRoleTargetSelector`
(`:182`), `getRoleTargetSelectorCount` (`:191`), `getRoleTargetSelectors` (`:199`),
`getRoleOfTargetSelector` (`:217`).

Labels are a first-class, enforced-unique registry rather than the base contract's
event-only labels: `labelRole` (`:61`), `getRoleLabel` (`:222`), `getRoleLabelCount`
(`:227`), `getRoleLabels` (`:232`), `isLabelAssigned` (`:237`), `isRoleLabeled`
(`:242`), `getLabelOfRole` (`:247`), `getRoleOfLabel` (`:254`). Errors:
`AccessManagerUnlabeledRole`, `AccessManagerUnregisteredLabel`,
`AccessManagerRoleAlreadyLabeled`, `AccessManagerLabelAlreadyUsed`.

Tracking is implemented by overriding four base hooks — `_setRoleAdmin` (`:260`),
`_setRoleGuardian` (`:270`), `_grantRole` (`:277`), `_revokeRole` (`:294`),
`_setTargetFunctionRole` (`:305`) — each calling a `_track*` helper (`:317-397`).
Because the overrides sit on internal hooks, no permission logic changes; only
bookkeeping is added.

### 10.2 Role catalogue — `src/deployments/utils/libraries/Roles.sol` (174 lines)

| Role | ID | Line |
|---|---:|---:|
| `ACCESS_MANAGER_ADMIN_ROLE` | 0 | 44 |
| `HUB_DOMAIN_ADMIN_ROLE` | 100 | 47 |
| `HUB_CONFIGURATOR_ROLE` | 101 | 48 |
| `HUB_FEE_MINTER_ROLE` | 102 | 49 |
| `HUB_DEFICIT_ELIMINATOR_ROLE` | 103 | 50 |
| `HUB_CONFIGURATOR_DOMAIN_ADMIN_ROLE` | 200 | 53 |
| `SPOKE_DOMAIN_ADMIN_ROLE` | 300 | 56 |
| `SPOKE_CONFIGURATOR_ROLE` | 301 | 57 |
| `SPOKE_USER_POSITION_UPDATER_ROLE` | 302 | 58 |
| `SPOKE_CONFIGURATOR_DOMAIN_ADMIN_ROLE` | 400 | 61 |

Selector bundles: `getHubConfiguratorRoleSelectors` (`:66`),
`getHubFeeMinterRoleSelectors` (`:77`), `getHubDeficitEliminatorRoleSelectors`
(`:84`), `getHubConfiguratorDomainAdminRoleSelectors` (`:93`),
`getSpokePositionUpdaterRoleSelectors` (`:123`), `getSpokeConfiguratorRoleSelectors`
(`:131`), `getSpokeConfiguratorDomainAdminRoleSelectors` (`:146`).

Two roles worth calling out. `HUB_DEFICIT_ELIMINATOR_ROLE` gates
`Hub.eliminateDeficit`, i.e. who may burn a Spoke's shares to absorb bad debt —
that is the backstop key. `SPOKE_USER_POSITION_UPDATER_ROLE` is what
`Spoke.updateUserRiskPremium`/`updateUserDynamicConfig` fall back to
(`Spoke.sol:417`, `:426`) when the caller is not the user's own position manager: a
keeper role that can migrate users onto new risk parameters.

---

## §11 Config engine and governance

### 11.1 `src/config-engine/AaveV4Payload.sol` (438 lines)

An abstract governance-proposal base using the template-method pattern. A concrete
payload overrides only the getters it needs; every getter defaults to an empty array.

`execute()` (`:28-37`) runs, in order: `_preExecute()`, `_executeHubActions()`,
`_executeSpokeActions()`, `_executeAccessManagerActions()`,
`_executePositionManagerActions()`, `_postExecute()`.

Overridable getters (all `public view virtual`, returning empty by default):
`hubAssetListings` (`:39`), `hubAssetConfigUpdates` (`:50`),
`hubSpokeToAssetsAdditions` (`:61`), `hubSpokeConfigUpdates` (`:72`),
`hubAssetHalts` (`:83`), `hubAssetDeactivations` (`:89`), `hubAssetCapsResets`
(`:100`), `hubSpokeDeactivations` (`:111`), `hubSpokeCapsResets` (`:122`),
`spokeReserveListings` (`:133`), `spokeReserveConfigUpdates` (`:144`),
`spokeLiquidationConfigUpdates` (`:155`), `spokeDynamicReserveConfigAdditions`
(`:166`), `spokeDynamicReserveConfigUpdates` (`:177`), `spokePositionManagerUpdates`
(`:188`), `accessManagerRoleMemberships` (`:199`), `accessManagerRoleUpdates`
(`:210`), `accessManagerTargetFunctionRoleUpdates` (`:221`),
`accessManagerTargetAdminDelayUpdates` (`:232`), `positionManagerSpokeRegistrations`
(`:243`), `positionManagerRoleRenouncements` (`:254`).

Hooks `_preExecute` (`:434`) and `_postExecute` (`:437`) are empty.

`_delegateCallEngine(bytes data)` (`:429-432`) — the payload **delegatecalls** the
engine, so the engine's library calls execute with the payload's identity and thus
the payload's roles. Error `InvalidConfigEngine` (`:17`).

### 11.2 `src/config-engine/AaveV4ConfigEngine.sol` (129 lines)

A pure dispatcher. Each of its 21 functions forwards one array to the matching
external library. Hub actions (`:16-59`), Spoke actions (`:61-95`), position-manager
actions (`:97-109`), access-manager actions (`:111-128`).

### 11.3 Engine libraries

**`HubEngine.sol` (353 lines)** — `executeHubAssetListings` (`:30`),
`executeHubAssetConfigUpdates` (`:54`), `executeHubSpokeToAssetsAdditions` (`:99`),
`executeHubSpokeConfigUpdates` (`:126`), `executeHubAssetHalts` (`:165`),
`executeHubAssetDeactivations` (`:175`), `executeHubAssetCapsResets` (`:187`),
`executeHubSpokeDeactivations` (`:199`), `executeHubSpokeCapsResets` (`:213`).
Internals: `_deployAndRegisterTokenizationSpoke` (`:225`) — listing an asset can
deploy its ERC-4626 vault in the same proposal — `_mergeInterestRateData` (`:265`),
`_updateInterestRateStrategy` (`:304`), `_updateSpokeCaps` (`:332`).
Errors: `InvalidIrDataWithNewStrategy` (`:20`), `InvalidTokenizationSpokeConfig` (`:24`).

**`SpokeEngine.sol` (228 lines)** — `executeSpokeReserveListings` (`:18`),
`executeSpokeReserveConfigUpdates` (`:37`), `executeSpokeLiquidationConfigUpdates`
(`:98`), `executeSpokeDynamicReserveConfigAdditions` (`:141`),
`executeSpokeDynamicReserveConfigUpdates` (`:163`),
`executeSpokePositionManagerUpdates` (`:206`), `_resolveReserveId` (`:220`).

**`AccessManagerEngine.sol` (88 lines)** — `executeRoleMemberships` (`:14`),
`executeRoleUpdates` (`:38`), `executeTargetFunctionRoleUpdates` (`:62`),
`executeTargetAdminDelayUpdates` (`:77`).

**`PositionManagerEngine.sol` (38 lines)** —
`executePositionManagerSpokeRegistrations` (`:13`),
`executePositionManagerRoleRenouncements` (`:27`).

**`TokenizationSpokeDeployer.sol` (121 lines)** — deterministic CREATE2 deployment
and pre-computation: `deploy` (`:25`), `computeImplementationAddress` (`:56`),
`computeProxyAddress` (`:72`), `_computeImplementationAddress` (`:90`),
`_computeImplementationSalt` (`:104`), `_computeProxySalt` (`:113`).
Error `InvalidProxyAdminOwner` (`:14`). Pre-computation matters because a proposal
must reference the vault address before it exists.

**`EngineFlags.sol` (46 lines)** — sentinel values for "leave this field alone" in
partial updates: `KEEP_CURRENT = type(uint256).max - 652` (`:13`),
`KEEP_CURRENT_ADDRESS = address(type(uint160).max)` (`:15`),
`KEEP_CURRENT_UINT64 = type(uint64).max - 46` (`:18`),
`KEEP_CURRENT_UINT32 = type(uint32).max - 23` (`:21`),
`KEEP_CURRENT_UINT16 = type(uint16).max - 61` (`:24`), plus `ENABLED`/`DISABLED`
(`:27-29`) and `toBool` (`:35`, reverts `InvalidBoolValue`) / `fromBool` (`:43`).
The odd offsets keep the sentinels clear of plausible real values.

**Worked example — listing an asset.** A payload overrides `hubAssetListings()` to
return one `AssetListing`, and `spokeReserveListings()` to return one
`ReserveListing`. On `execute()`:

```
AaveV4Payload.execute
 ├─ _executeHubActions            AaveV4Payload.sol:264
 │   └─ delegatecall AaveV4ConfigEngine.executeHubAssetListings   :16
 │       └─ HubEngine.executeHubAssetListings                     :30
 │           ├─ HubConfigurator.addAsset / addAssetWithDecimals   HubConfigurator.sol:24/45
 │           │   └─ Hub.addAsset  (restricted)                    Hub.sol:47
 │           └─ _deployAndRegisterTokenizationSpoke (optional)    HubEngine.sol:225
 │               └─ TokenizationSpokeDeployer.deploy              :25
 └─ _executeSpokeActions          AaveV4Payload.sol:327
     └─ SpokeEngine.executeSpokeReserveListings                   :18
         └─ SpokeConfigurator.addReserve                          SpokeConfigurator.sol:73
             └─ Spoke.addReserve  (restricted)                    Spoke.sol:121
                 └─ AaveOracle.setReserveSource                   AaveOracle.sol:44
```

**Adding a Spoke to a live Hub** is the same shape but shorter: override
`hubSpokeToAssetsAdditions()` → `HubEngine.executeHubSpokeToAssetsAdditions`
(`:99`) → `HubConfigurator.addSpokeToAssets` (`:169`) → `Hub.addSpoke` (`:158`) per
asset. **No liquidity migrates.** That is the whole point of the architecture: a new
market is a registry entry plus caps.

---

## §12 Deployments

### 12.1 `src/deployments/orchestration/AaveV4DeployOrchestration.sol` (517 lines)

`deployAaveV4(logger, deployer, deployInputs, hubBytecode, spokeBytecode)` (`:30-150`)
runs a fixed order. Everything is CREATE2 with a salt derived from the deployer and
an input salt (`_deriveSalt`, `:501`; `_deriveChildSalt`, `:510`), so addresses are
predictable across chains.

| # | Step | Line | Produces |
|---:|---|---:|---|
| 1 | Authority batch | 43 | `AccessManagerEnumerable`, admin = deployer initially |
| 2 | Label all roles | 53 | via `AaveV4AccessManagerRolesProcedure` |
| 3 | Configurator batch | 56 | `HubConfigurator`, `SpokeConfigurator` |
| 4 | Configurator roles | 64 | |
| 5 | Treasury spoke batch | 67 | one `TreasurySpoke` shared by all Hubs |
| 6 | Validate label uniqueness | 74 | duplicate labels would collide CREATE2 salts |
| 7 | Hub batches | 78 | per label: Hub proxy + impl + `AssetInterestRateStrategy` |
| 8 | Spoke batches | 88 | per label: `AaveOracle`, `Spoke` proxy + impl |
| 9 | Gateway batch (optional) | 98 | `NativeTokenGateway`, `SignatureGateway` |
| 10 | Position manager batch (optional) | 110 | Giver, Taker, Config managers |
| 11 | Grant roles | 118 | hub and spoke admins |
| 12 | Hand over admin | 136 | replace the deployer with the real admin |

Step 12 is the one that matters for trust: `replaceDefaultAdminRole` (`:141`) adds
the intended admin and removes the deployer in one call. Until then the deployer is
root.

Helpers: `_deployHubs` (`:152`), `_deployHub` (`:177`), `_deploySpokes` (`:201`),
`_deploySpoke` (`:231`), the six batch wrappers (`:259-381`), `_setupConfiguratorRoles`
(`:382`), `_setupSpokeRoles` (`:399`), `_setupHubRoles` (`:411`), `_grantHubRoles`
(`:420`), `_grantSpokeRoles` (`:448`), `_logHubReport` (`:476`), `_logSpokeReport`
(`:487`).

### 12.2 Supporting deployment files

Batches (`src/deployments/batches/`): `AaveV4AuthorityBatch.sol` (25),
`AaveV4ConfiguratorBatch.sol` (45), `AaveV4GatewayBatch.sol` (54),
`AaveV4HubInstanceBatch.sol` (47), `AaveV4PositionManagerBatch.sol` (34),
`AaveV4SpokeInstanceBatch.sol` (56), `AaveV4TokenizationSpokeBatch.sol` (51),
`AaveV4TreasurySpokeBatch.sol` (25).

Deploy procedures (`procedures/deploy/`): one per contract —
`AaveV4AccessManagerEnumerableDeployProcedure.sol`, hub-side
`AaveV4HubDeployProcedure.sol` / `AaveV4HubConfiguratorDeployProcedure.sol` /
`AaveV4InterestRateStrategyDeployProcedure.sol`, spoke-side
`AaveV4SpokeDeployProcedure.sol` / `AaveV4SpokeConfiguratorDeployProcedure.sol` /
`AaveV4AaveOracleDeployProcedure.sol` / `AaveV4TokenizationSpokeDeployProcedure.sol` /
`AaveV4TreasurySpokeDeployProcedure.sol`, and position-manager-side
`AaveV4ConfigPositionManagerDeployProcedure.sol`,
`AaveV4GiverPositionManagerDeployProcedure.sol`,
`AaveV4TakerPositionManagerDeployProcedure.sol`,
`AaveV4SignatureGatewayDeployProcedure.sol`,
`AaveV4NativeTokenGatewayDeployProcedure.sol`. Base:
`AaveV4DeployProcedureBase.sol` (11).

Role procedures (`procedures/roles/`): `AaveV4AccessManagerRolesProcedure.sol` (76),
`AaveV4HubRolesProcedure.sol` (76), `AaveV4HubConfiguratorRolesProcedure.sol` (55),
`AaveV4SpokeRolesProcedure.sol` (66), `AaveV4SpokeConfiguratorRolesProcedure.sol` (56).

Utils: `Create2Utils.sol` (85, errors `MissingCreate2Factory`,
`Create2AddressDerivationFailure`, `FailedCreate2FactoryCall`,
`ContractAlreadyDeployed`), `BytecodeHelper.sol` (23), `InputUtils.sol` (65),
`DeployConstants.sol` (12), `Roles.sol` (174), `Logger.sol` (204),
`MetadataLogger.sol` (86), and interfaces `IHubInstance.sol`, `ISpokeInstance.sol`,
`ITokenizationSpokeInstance.sol`.

Reports: `OrchestrationReports.sol` (42), `BatchReports.sol` (65).

---

## §13 Utils, interfaces, dependencies

### 13.1 `src/utils/`

| File | Lines | What |
|---|---:|---|
| `Multicall.sol` | 27 | `multicall(bytes[])` self-`delegatecall` loop, bubbles the first revert verbatim in assembly (`:17-21`) |
| `ExtSload.sol` | 36 | `extSload(bytes32)` (`:11`) and `extSloads(bytes32[])` (`:18`) — raw storage reads for off-chain indexers; `extSloads` writes its own ABI encoding in assembly and `return`s directly |
| `NoncesKeyed.sol` | 67 | ERC-4337-style keyed nonces in ERC-7201 namespaced storage |
| `IntentConsumer.sol` | 41 | EIP-712 verification + nonce consumption |
| `Rescuable.sol` | 39 | guardian-gated token rescue |

**`NoncesKeyed`** — storage at a fixed slot (`:18-20`):

```
0x474d4a5585c1bae3dbeb574bb96408c7174aadd8ab635de4ab498e2723195f00
= keccak256(abi.encode(uint256(keccak256("aave-v4.storage.NoncesKeyed")) - 1)) & ~bytes32(0xff)
```

A nonce is `(uint192 key << 64) | uint64 nonce` (`_pack`, `:52`). Different keys
advance independently, so a user can have several signature streams in flight and
cancel one without invalidating the others. `useNonce(key)` (`:23`) burns the next
nonce — that is how a user cancels a pending signature.
`_useCheckedNonce` (`:45`) reverts `InvalidAccountNonce(owner, current)` on mismatch.

**`IntentConsumer._verifyAndConsumeIntent`** (`:26-40`) — deadline check →
`InvalidSignature`; `_hashTypedData`; `SignatureChecker.isValidSignatureNowCalldata`
so **ERC-1271 smart-contract wallets are supported**; then the checked nonce.
Domain is Solady `EIP712` with `address(this)` as verifying contract, no salt
(`:13`).

### 13.2 `src/interfaces/`

`IMulticall.sol` (12), `IExtSload.sol` (15), `IIntentConsumer.sol` (15, error
`InvalidSignature`), `INoncesKeyed.sol` (19, error `InvalidAccountNonce`),
`IRescuable.sol` (25, error `OnlyRescueGuardian`).

Interfaces elsewhere in scope: hub `IHub.sol` (423), `IHubBase.sol` (336),
`IHubConfigurator.sol` (222), `IBasicInterestRateStrategy.sol` (31),
`IAssetInterestRateStrategy.sol` (96); spoke `ISpoke.sol` (787),
`ISpokeConfigurator.sol` (211), `ITokenizationSpoke.sol` (192),
`ITreasurySpoke.sol` (65), `IAaveOracle.sol` (68), `IPriceOracle.sol` (23),
`IPriceFeed.sol` (16); position-manager `IPositionManagerBase.sol` (81),
`IPositionManagerIntentBase.sol` (10), `IGiverPositionManager.sol` (76),
`ITakerPositionManager.sol` (235), `IConfigPositionManager.sol` (288),
`ISignatureGateway.sol` (218), `INativeTokenGateway.sol` (86),
`INativeWrapper.sol` (16); access `IAccessManagerEnumerable.sol` (222);
config-engine `IAaveV4ConfigEngine.sol` (441).

`IPriceFeed.sol` (16 lines) is the whole oracle contract v4 depends on:
`latestAnswer()` and `decimals()`. No `latestRoundData`, no round IDs, no timestamps.

### 13.3 `src/dependencies/` — vendored, out of scope

Not documented function-by-function. All are vendored copies, unmodified in
interface, of upstream libraries.

| Directory | Source | Files used behaviourally |
|---|---|---|
| `openzeppelin/` | OpenZeppelin Contracts | `AccessManager`, `AccessManaged`, `Math` (512-bit `mulDiv`), `SafeCast`, `SafeERC20`, `EnumerableSet`, `Arrays` (sort), `ECDSA`, `SignatureChecker`, `ReentrancyGuardTransient`, `ERC1967Proxy`, `TransparentUpgradeableProxy`, `ProxyAdmin`, `Ownable2Step`, `IERC4626`, `IERC20Permit`, `TransientSlot`, `Bytes`, `Comparators`, `Hashes`, `SlotDerivation`, `StorageSlot`, `Time`, `LowLevelCall`, `Panic`, `Errors` |
| `openzeppelin-upgradeable/` | OpenZeppelin Upgradeable | `AccessManagedUpgradeable`, `ERC20Upgradeable`, `Initializable`, `Ownable2StepUpgradeable`, `OwnableUpgradeable`, `ContextUpgradeable` |
| `solady/` | Solady | `EIP712` (domain separator caching), `LibBit` (`fls`, `popCount` — the position bitmap iterators) |
| `weth/` | canonical WETH9 | test/deploy fixture for `NativeTokenGateway` |

---

## §14 Storage layout tables

### Hub (`HubStorage.sol`)

| Slot | Type | Name | Line |
|---:|---|---|---:|
| 0 | `uint256` | `_assetCount` | 13 |
| 1 | `mapping(uint256 => Asset)` | `_assets` | 16 |
| 2 | `mapping(uint256 => mapping(address => SpokeData))` | `_spokes` | 19 |
| 3 | `mapping(uint256 => EnumerableSet.AddressSet)` | `_assetToSpokes` | 22 |
| 4 | `mapping(address => uint256)` | `_underlyingToAssetId` | 25 |
| 5–54 | `uint256[50]` | `__gap` | 28 |

`IHub.Asset` sub-layout (10 slots per asset, `IHub.sol:29-56`):

| Off | Bits | Fields |
|---:|---|---|
| +0 | 120/120/8 | `liquidity`, `realizedFees`, `decimals` |
| +1 | 120/120 | `addedShares`, `swept` |
| +2 | 200 | `premiumOffsetRay` (int200) |
| +3 | 120/120/16 | `drawnShares`, `premiumShares`, `liquidityFee` |
| +4 | 120/96/40 | `drawnIndex`, `drawnRate`, `lastUpdateTimestamp` |
| +5 | 160 | `underlying` |
| +6 | 160 | `irStrategy` |
| +7 | 160 | `reinvestmentController` |
| +8 | 160 | `feeReceiver` |
| +9 | 200 | `deficitRay` |

`IHub.SpokeData` sub-layout (4 slots, `IHub.sol:77-91`):

| Off | Bits | Fields |
|---:|---|---|
| +0 | 120/120 | `drawnShares`, `premiumShares` |
| +1 | 200 | `premiumOffsetRay` (int200) |
| +2 | 120/40/40/24/8/8 | `addedShares`, `addCap`, `drawCap`, `riskPremiumThreshold`, `active`, `halted` |
| +3 | 200 | `deficitRay` |

### Spoke (`SpokeStorage.sol`)

| Slot | Type | Name | Line |
|---:|---|---|---:|
| 0 | `uint256` | `_reserveCount` | 12 |
| 1 | `ISpoke.LiquidationConfig` | `_liquidationConfig` | 15 |
| 2 | `mapping(uint256 => Reserve)` | `_reserves` | 18 |
| 3 | `mapping(address => mapping(uint256 => uint256))` | `_hubAssetIdToReserveId` | 21 |
| 4 | `mapping(uint256 => mapping(uint32 => DynamicReserveConfig))` | `_dynamicConfig` | 25 |
| 5 | `mapping(address => PositionStatus)` | `_positionStatus` | 29 |
| 6 | `mapping(address => mapping(uint256 => UserPosition))` | `_userPositions` | 32 |
| 7 | `mapping(address => PositionManagerConfig)` | `_positionManager` | 36 |
| 8–57 | `uint256[50]` | `__gap` | 39 |

`ISpoke.Reserve` (2 slots): `underlying` | `hub`,`assetId`,`decimals`,`collateralRisk`,`flags`,`dynamicConfigKey`.
`ISpoke.UserPosition` (3 slots): `drawnShares`,`premiumShares` | `premiumOffsetRay` | `suppliedShares`,`dynamicConfigKey`.
`ISpoke.LiquidationConfig` (1 slot): `targetHealthFactor` (128), `healthFactorForMaxBonus` (64), `liquidationBonusFactor` (16).

### Namespaced storage

| Contract | Slot | Derivation |
|---|---|---|
| `NoncesKeyed` | `0x474d4a5585c1bae3dbeb574bb96408c7174aadd8ab635de4ab498e2723195f00` | `keccak256(abi.encode(uint256(keccak256("aave-v4.storage.NoncesKeyed")) - 1)) & ~bytes32(uint256(0xff))` — `NoncesKeyed.sol:18-20` |

`ERC20Upgradeable`, `AccessManagedUpgradeable`, `Initializable` and
`Ownable2StepUpgradeable` each use their own upstream ERC-7201 namespace, so the
Spoke's `__gap` only has to protect `SpokeStorage`.

### Other stateful contracts

| Contract | Storage |
|---|---|
| `AaveOracle` | `address public spoke` (slot 0), `mapping(uint256 => IPriceFeed) _sources` (slot 1); `DECIMALS`, `DEPLOYER` immutable |
| `AssetInterestRateStrategy` | `mapping(uint256 => InterestRateData) _interestRateData` (slot 0); `HUB` immutable |
| `PositionManagerBase` | `mapping(address => bool) _registeredSpokes` |
| `TakerPositionManager` | `_withdrawAllowances`, `_borrowAllowances` (4-level maps) |
| `AccessManagerEnumerable` | OZ `AccessManager` storage plus seven `EnumerableSet`s and the label maps |
| `TokenizationSpoke` | `ERC20Upgradeable` namespaced storage only; `HUB`/`ASSET_ID`/`ASSET`/`DECIMALS`/`ASSET_UNITS`/`MAX_ALLOWED_SPOKE_CAP` immutable |
| `TreasurySpoke` | `Ownable2StepUpgradeable` namespaced storage only |

---

## §15 Selector / ABI tables

Computed with `cast sig` against the signatures in this tree.

### Hub — mutating

| Signature | Selector | Gate |
|---|---|---|
| `addAsset(address,uint8,address,address,bytes)` | `0x1e83287e` | `restricted` |
| `updateAssetConfig(uint256,(address,uint16,address,address),bytes)` | `0x24e4c1af` | `restricted` |
| `addSpoke(uint256,address,(uint40,uint40,uint24,bool,bool))` | `0xc25d82fe` | `restricted` |
| `updateSpokeConfig(uint256,address,(uint40,uint40,uint24,bool,bool))` | `0xa2763d29` | `restricted` |
| `setInterestRateData(uint256,bytes)` | `0xa467cc59` | `restricted` |
| `mintFeeShares(uint256)` | `0x033a0695` | `restricted` (`HUB_FEE_MINTER_ROLE`) |
| `eliminateDeficit(uint256,uint256,address)` | `0xbe105280` | `restricted` (`HUB_DEFICIT_ELIMINATOR_ROLE`) |
| `add(uint256,uint256)` | `0x771602f7` | registered active spoke |
| `remove(uint256,uint256,address)` | `0xe840427d` | registered active spoke |
| `draw(uint256,uint256,address)` | `0xa436458d` | registered active spoke |
| `restore(uint256,uint256,(int256,int256,uint256))` | `0x2a5b3803` | registered active spoke |
| `reportDeficit(uint256,uint256,(int256,int256,uint256))` | `0xcc0e1c1c` | active spoke (halted allowed) |
| `refreshPremium(uint256,(int256,int256,uint256))` | `0x341f7dcf` | active spoke |
| `payFeeShares(uint256,uint256)` | `0x83e4bcb7` | active spoke |
| `transferShares(uint256,uint256,address)` | `0x87a7dc77` | active spoke |
| `sweep(uint256,uint256)` | `0x066dd830` | `reinvestmentController` only |
| `reclaim(uint256,uint256)` | `0x7333a3b4` | `reinvestmentController` only |

### Spoke

| Signature | Selector | Gate |
|---|---|---|
| `supply(uint256,uint256,address)` | `0x852a56a5` | `onlyPositionManager` |
| `withdraw(uint256,uint256,address)` | `0x0ad58d2f` | `onlyPositionManager` |
| `borrow(uint256,uint256,address)` | `0xd6bda0c0` | `onlyPositionManager` |
| `repay(uint256,uint256,address)` | `0xb1e8f8ef` | `onlyPositionManager` |
| `liquidationCall(uint256,uint256,address,uint256,bool)` | `0xc2fa746c` | open |
| `setUsingAsCollateral(uint256,bool,address)` | `0x9e35c533` | `onlyPositionManager` |
| `updateUserRiskPremium(address)` | `0x91c46d09` | manager **or** `SPOKE_USER_POSITION_UPDATER_ROLE` |
| `updateUserDynamicConfig(address)` | `0x826002e2` | manager **or** role |
| `setUserPositionManager(address,bool)` | `0x8874e104` | self |
| `renouncePositionManagerRole(address)` | `0xfea149a6` | the manager itself |
| `permitReserve(uint256,address,uint256,uint256,uint8,bytes32,bytes32)` | `0x2bccdfd5` | open (`try/catch`) |
| `addReserve(address,uint256,address,(uint24,bool,bool,bool,bool),(uint16,uint32,uint16))` | `0x4de8d2ba` | `restricted` |
| `updateReserveConfig(uint256,(uint24,bool,bool,bool,bool))` | `0xa0f5b9ab` | `restricted` |
| `updatePositionManager(address,bool)` | `0x9ca9c134` | `restricted` |
| `getUserAccountData(address)` | `0xbf92857c` | view |
| `multicall(bytes[])` | `0xac9650d8` | open |
| `extSload(bytes32)` | `0xaaaf97ab` | view |

`TokenizationSpoke` exposes the standard ERC-4626 and ERC-20 selectors plus the
`*WithSig` variants; `TreasurySpoke` exposes `supply`, `supplySkimmed`, `withdraw`,
`transfer` and two getters, all `onlyOwner`.

---

## §16 Complete custom-error table

95 custom errors are declared in scope. Selectors from `cast sig`.

### Hub (`IHub.sol`)

| Error | Selector | Thrown at | Cause |
|---|---|---|---|
| `UnderlyingAlreadyListed()` | `0x603c058b` | `Hub.sol:62` | listing a token twice |
| `AssetNotListed()` | `0xb77e1e0f` | `:118`, `:171`, `:182`, `:191`, `:407`, `:428`, `:497` | `assetId >= _assetCount` |
| `AddCapExceeded(uint256)` | `0xde3fc6ae` | `:823-828`, `:912-917` | spoke add cap or transfer target cap |
| `InsufficientLiquidity(uint256)` | `0xc730333f` | `:232`, `:257`, `:414` | not enough idle liquidity |
| `InsufficientTransferred(uint256)` | `0x80561eeb` | `:209`, `:288`, `:436` | pull-style balance check failed |
| `DrawCapExceeded(uint256)` | `0x3ad30dd0` | `:852-857` | spoke draw cap (incl. deficit) |
| `SurplusDrawnRestored(uint256)` | `0x4bd9e476` | `:871` | repaying more drawn than owed |
| `SurplusPremiumRayRestored(uint256)` | `0xdc868246` | `:872` | repaying more premium than owed |
| `InvalidPremiumChange()` | `0xa664e075` | `:369`, `:758-762`, `:954-957` | premium not conserved, threshold exceeded, or repayment inside a refresh |
| `SurplusDrawnDeficitReported(uint256)` | `0x04ddd91f` | `:885` | writing off more than owed |
| `SurplusPremiumRayDeficitReported(uint256)` | `0x37f3f9b7` | `:886` | same, premium |
| `SpokeNotActive()` | `0xe86fa032` | `:367`, `:774`, `:820`, `:834`, `:848`, `:867`, `:882`, `:893`, `:898`, `:908` | caller not a registered active spoke |
| `SpokeHalted()` | `0x9db9b355` | `:821`, `:835`, `:849`, `:868`, `:909` | spoke halted (liquidity actions only) |
| `InvalidReinvestmentController()` | `0x91eaafa1` | `:126-129` | unsetting a controller that still holds `swept` |
| `OnlyReinvestmentController()` | `0x3c6b7746` | `:922`, `:928` | sweep/reclaim by the wrong caller |
| `SpokeAlreadyListed()` | `0x04c94583` | `:706` | `EnumerableSet.add` returned false |
| `SpokeNotListed()` | `0xaa1b05f0` | `:172` | updating an unregistered spoke |
| `InvalidAmount()` | `0x2c5211c6` | `:819`, `:833`, `:847`, `:866`, `:881`, `:894`, `:923`, `:929` | zero amount |
| `InvalidShares()` | `0x6edcc523` | `:211`, `:899`, `:910` | zero shares (add rounded to nothing) |
| `InvalidAddress()` | `0xe6c4247b` | `:54-57`, `:125`, `:160`, `:832`, `:846` | zero address, or `to == hub` |
| `InvalidLiquidityFee()` | `0xe15e46cb` | `:124` | fee > 100_00 bps |
| `InvalidAssetDecimals()` | `0xe2364765` | `:58-61` | decimals outside [6, 18] |
| `InvalidInterestRateStrategy()` | `0x38ad7932` | `:149` | non-empty `irData` without a strategy change |

### Spoke (`ISpoke.sol`)

| Error | Selector | Thrown at | Cause |
|---|---|---|---|
| `AssetNotListed()` | `0xb77e1e0f` | `Spoke.sol:138` | Hub returned a zero underlying |
| `ReserveExists()` | `0xe71301f8` | `:130` | `(hub, assetId)` already listed |
| `InvalidAssetId()` | `0xfafca5a0` | `:129` | `assetId > type(uint16).max` |
| `InvalidAssetDecimals()` | `0xe2364765` | `:139` | decimals > 18 |
| `ReserveNotListed()` | `0x2e5d6bb4` | `SpokeUtils.sol:24`; `Spoke.sol:186`, `:195`, `:212`, `:480` | unknown `reserveId` |
| `ReserveNotBorrowable()` | `0xaac43c92` | `Spoke.sol:874` | borrowable flag clear |
| `ReservePaused()` | `0xd37f5f1c` | `:863`, `:868`, `:872`, `:879`, `:887`; `LiquidationLogic.sol:536-539` | paused flag set |
| `ReserveFrozen()` | `0x6d305815` | `Spoke.sol:864`, `:873`, `:890` | frozen flag set |
| `HealthFactorBelowThreshold()` | `0x851aedc1` | `:691-694` | HF < 1e18 after the action |
| `ReserveNotEnabledAsCollateral()` | `0x95aaa7c8` | `LiquidationLogic.sol:548-551` | zero collateral factor or bit unset |
| `ReserveNotSupplied()` | `0xe2fadb09` | `:540` | no collateral shares to seize |
| `ReserveNotBorrowed()` | `0x7b55ea4a` | `:543` | no drawn shares to repay |
| `Unauthorized()` | `0x82b42900` | `Spoke.sol:91` | caller is not an approved active position manager |
| `DynamicConfigKeyUninitialized()` | `0x17e5b997` | `:857` | updating a never-added config key |
| `InvalidAddress()` | `0xe6c4247b` | `:128`, `:672` | zero hub or price source |
| `InvalidOracleDecimals()` | `0x38bebeba` | `:99` | oracle decimals ≠ 8 |
| `InvalidMaxUserReservesLimit()` | `0xbdf3f563` | `:100` | zero limit |
| `InvalidCollateralRisk()` | `0xc0534e95` | `:916` | risk > 1000_00 |
| `InvalidLiquidationConfig()` | `0xa3f02e7f` | `:110-115` | target HF, bonus factor or max-bonus HF out of range |
| `InvalidLiquidationFee()` | `0x16535458` | `:929` | fee > 100_00 |
| `InvalidCollateralFactorAndMaxLiquidationBonus()` | `0xcf8e83b1` | `:922-928` | `bonus × CF >= 100_00`, or CF ≥ 100_00, or bonus < 100_00 |
| `InvalidCollateralFactor()` | `0xbc8b2b40` | `:858` | new dynamic config with zero collateral factor |
| `SelfLiquidation()` | `0x44511af1` | `LiquidationLogic.sol:534` | liquidator == user |
| `HealthFactorNotBelowThreshold()` | `0x930bb771` | `:544-547` | position is healthy |
| `MustNotLeaveDust()` | `0xb629b0e4` | `:684-688` | `debtToCover` too small to avoid dust |
| `InvalidDebtToCover()` | `0x411dcff5` | `:535` | zero `debtToCover` |
| `CannotReceiveShares()` | `0x861a96d6` | `:552-557` | `receiveShares` on a frozen or shares-disabled reserve |
| `MaximumDynamicConfigKeyReached()` | `0x8affb4f4` | `Spoke.sol:197` | key would exceed `uint32` |
| `MaximumUserReservesExceeded()` | `0x0f04df18` | `:288-292`, `:892-896` | too many borrow or collateral reserves |

### Everything else

| Error | Selector | Declared in | Cause |
|---|---|---|---|
| `InterestRateDataNotSet(uint256)` | `0x0d183c0b` | `IBasicInterestRateStrategy.sol:10` | rate params never configured |
| `InvalidAddress()` | `0xe6c4247b` | `IAssetInterestRateStrategy.sol:39`, `IHubConfigurator.sol:11`, `ISpokeConfigurator.sol:11`, `IPositionManagerBase.sol:16`, `IAaveOracle.sol:38` | zero address |
| `OnlyHub()` | `0x2c53e398` | `IAssetInterestRateStrategy.sol:42` | IRS called by a non-Hub |
| `InvalidMaxDrawnRate()` | `0x126383a8` | `:45` | rate components sum > 1000_00 |
| `InvalidOptimalUsageRatio()` | `0x35a8aee7` | `:48` | outside [1_00, 99_00] |
| `MismatchedConfigs()` | `0x8f7b3d64` | `IHubConfigurator.sol:14` | array length mismatch |
| `OnlyDeployer()` | `0x618bbdd5` | `IAaveOracle.sol:20` | `setSpoke` by a non-deployer |
| `SpokeAlreadySet()` | `0x307ef04a` | `:23` | oracle already bound |
| `InvalidSourceDecimals(uint256)` | `0xbbff7655` | `:27` | feed decimals ≠ oracle decimals |
| `InvalidSource(uint256)` | `0x0246b0be` | `:31` | no feed configured |
| `InvalidPrice(uint256)` | `0xf9632e86` | `:35` | `latestAnswer() <= 0` |
| `OracleMismatch()` | `0x80e24704` | `:41` | Spoke's `ORACLE()` is a different address |
| `OnlySpoke()` | `0x564cbba0` | `IPriceOracle.sol:10` | `setReserveSource` by a non-Spoke |
| `InvalidSignature()` | `0x8baa579f` | `IIntentConsumer.sol:11` | expired deadline or bad signature |
| `InvalidAccountNonce(address,uint256)` | `0x752d88c0` | `INoncesKeyed.sol:6` | nonce mismatch (replay) |
| `OnlyRescueGuardian()` | `0x3a026269` | `IRescuable.sol:9` | rescue by a non-guardian |
| `InvalidAmount()` | `0x2c5211c6` | `IPositionManagerBase.sol:19` | zero amount |
| `UnsupportedAction()` | `0x25e9714f` | `:22` | multicall disabled, or ETH sent to a gateway fallback |
| `SpokeNotRegistered()` | `0x94918724` | `:25` | spoke not registered on the manager |
| `InsufficientWithdrawAllowance(uint256,uint256)` | `0xa0edc624` | `ITakerPositionManager.sol:110` | allowance < amount |
| `InsufficientBorrowAllowance(uint256,uint256)` | `0x31577909` | `:113` | allowance < amount |
| `DelegateeNotAllowed()` | `0xfada00e8` | `IConfigPositionManager.sol:135` | permission bit not granted |
| `RepayOnBehalfMaxUintNotAllowed()` | `0x61306f9d` | `IGiverPositionManager.sol:43` | `type(uint256).max` as repay amount |
| `NotNativeWrappedAsset()` | `0x75996463` | `INativeTokenGateway.sol:12` | reserve underlying ≠ wrapper |
| `NativeAmountMismatch()` | `0x677606af` | `:15` | `msg.value != amount` |
| `MaxDataSizeExceeded()` | `0x249148e2` | `KeyValueList.sol:17` | key ≥ 2³²−1 or value ≥ 2²²⁴−1 |
| `AccessManagerUnlabeledRole(uint64)` | `0xc0a6919f` | `IAccessManagerEnumerable.sol:11` | reading a label that was never set |
| `AccessManagerUnregisteredLabel(string)` | `0x9b0e2ce8` | `:14` | resolving an unknown label |
| `AccessManagerRoleAlreadyLabeled(uint64)` | `0x98bb4e7f` | `:17` | relabelling a role |
| `AccessManagerLabelAlreadyUsed(string,uint64)` | `0xf0d37523` | `:20` | label collision |
| `InvalidConfigEngine()` | `0xa93d7028` | `AaveV4Payload.sol:17` | zero engine address |
| `InvalidBoolValue(uint256)` | `0xb998bad5` | `EngineFlags.sol:9` | flag neither 0 nor 1 |
| `InvalidIrDataWithNewStrategy()` | `0x5d3c1bf2` | `HubEngine.sol:20` | partial IR data with a strategy swap |
| `InvalidTokenizationSpokeConfig()` | `0x59322750` | `:24` | inconsistent vault config in a listing |
| `InvalidProxyAdminOwner()` | `0x664ccbe0` | `TokenizationSpokeDeployer.sol:14` | zero proxy admin owner |
| `MissingCreate2Factory()`, `Create2AddressDerivationFailure()`, `FailedCreate2FactoryCall()`, `ContractAlreadyDeployed()` | — | `Create2Utils.sol:13-16` | deployment-time only |

**Not custom errors:** every `WadRayMath`, `PercentageMath` and `MathUtils` failure
is a bare `revert(0,0)` with empty return data. If you see an unexplained empty
revert from v4, suspect a fixed-point overflow or a division by zero in one of those
three libraries.

---

## §17 Complete events reference

51 events. `Hub` events are the canonical liquidity ledger; `Spoke` events are the
user-facing ledger; the two are linked by `(assetId, spoke)` versus `(reserveId, user)`.

### Hub — `IHubBase.sol`

| Event | Line | Indexed | Fires from |
|---|---:|---|---|
| `Add(assetId, spoke, shares, amount)` | 23 | assetId, spoke | `Hub.add:218` |
| `Remove(assetId, spoke, shares, amount)` | 30 | assetId, spoke | `Hub.remove:243` |
| `Draw(assetId, spoke, drawnShares, drawnAmount)` | 37 | assetId, spoke | `Hub.draw:268` |
| `Restore(assetId, spoke, drawnShares, premiumDelta, drawnAmount, premiumAmount)` | 51 | assetId, spoke | `Hub.restore:298` |
| `RefreshPremium(assetId, spoke, premiumDelta)` | 64 | assetId, spoke | `Hub.refreshPremium:373` |
| `ReportDeficit(assetId, spoke, drawnShares, premiumDelta, deficitAmountRay)` | 72 | assetId, spoke | `Hub.reportDeficit:327` |
| `TransferShares(assetId, sender, receiver, shares)` | 85 | all three | `Hub.payFeeShares:388`, `Hub.transferShares:402` |

### Hub — `IHub.sol`

| Event | Line | Fires from |
|---|---:|---|
| `AddAsset(assetId, underlying, decimals)` | 106 | `Hub.addAsset:99` |
| `UpdateAsset(assetId, drawnIndex, drawnRate, accruedFees)` | 113 | `AssetLogic.updateDrawnRate:137`, `Hub.addAsset:110` |
| `UpdateAssetConfig(assetId, config)` | 123 | `Hub.addAsset:100`, `Hub.updateAssetConfig:154` |
| `AddSpoke(assetId, spoke)` | 128 | `Hub._addSpoke:707` |
| `UpdateSpokeConfig(assetId, spoke, config)` | 134 | `Hub._updateSpokeConfig:717` |
| `MintFeeShares(assetId, feeReceiver, shares, assets)` | 141 | `Hub._mintFeeShares:779` |
| `Sweep(assetId, reinvestmentController, amount)` | 152 | `Hub.sweep:423` |
| `Reclaim(assetId, reinvestmentController, amount)` | 158 | `Hub.reclaim:442` |
| `EliminateDeficit(assetId, callerSpoke, coveredSpoke, shares, deficitAmountRay)` | 166 | `Hub.eliminateDeficit:356` |

`UpdateAsset` fires on **every** mutating Hub call, because `updateDrawnRate` ends
each one. An indexer can rebuild the index/rate time series from it alone.

### Spoke — `ISpoke.sol`

| Event | Line | Fires from |
|---|---:|---|
| `SetSpokeImmutables(oracle, maxUserReservesLimit)` | 142 | `SpokeInstance.initialize` |
| `UpdateLiquidationConfig(config)` | 146 | `Spoke.updateLiquidationConfig:117`, `SpokeInstance.initialize` |
| `AddReserve(reserveId, assetId, hub)` | 152 | `Spoke.addReserve:160` |
| `UpdateReserveConfig(reserveId, config)` | 157 | `Spoke.addReserve:161`, `updateReserveConfig:181` |
| `UpdateReservePriceSource(reserveId, priceSource)` | 162 | `Spoke._updateReservePriceSource:674` |
| `AddDynamicReserveConfig(reserveId, key, config)` | 170 | `Spoke.addReserve:162`, `addDynamicReserveConfig:202` |
| `UpdateDynamicReserveConfig(reserveId, key, config)` | 180 | `Spoke.updateDynamicReserveConfig:215` |
| `UpdatePositionManager(positionManager, active)` | 189 | `Spoke.updatePositionManager:221` |
| `Supply(reserveId, caller, user, suppliedShares, suppliedAmount)` | 197 | `Spoke.supply:238` |
| `Withdraw(reserveId, caller, user, withdrawnShares, withdrawnAmount)` | 211 | `Spoke.withdraw:268` |
| `Borrow(reserveId, caller, user, drawnShares, drawnAmount)` | 225 | `Spoke.borrow:299` |
| `Repay(reserveId, caller, user, drawnShares, totalAmountRepaid, premiumDelta)` | 240 | `Spoke.repay:341` |
| `LiquidationCall(...)` | 261 | `LiquidationLogic._executeLiquidation:425` |
| `ReportDeficit(reserveId, user, drawnShares, premiumDelta)` | 280 | `LiquidationLogic.notifyReportDeficit:299` |
| `SetUsingAsCollateral(reserveId, caller, user, usingAsCollateral)` | 292 | `Spoke.setUsingAsCollateral:411` |
| `UpdateUserRiskPremium(user, riskPremium)` | 302 | `Spoke._notifyRiskPremiumUpdate:848`, `LiquidationLogic:302` |
| `RefreshAllUserDynamicConfig(user)` | 306 | `Spoke._refreshAndValidateUserAccountData:690` |
| `RefreshSingleUserDynamicConfig(user, reserveId)` | 311 | `Spoke._refreshDynamicConfig:817` |
| `SetUserPositionManager(user, positionManager, approve)` | 317 | `Spoke._setUserPositionManager:680`, `renouncePositionManagerRole:465` |
| `RefreshPremiumDebt(reserveId, user, premiumDelta)` | 323 | `Spoke._notifyRiskPremiumUpdate:845` |

### Peripheral

| Event | Declared | Fires from |
|---|---|---|
| `UpdateInterestRateData(hub, assetId, optimalUsageRatio, baseDrawnRate, growthBefore, growthAfter)` | `IAssetInterestRateStrategy.sol:29` | `AssetInterestRateStrategy.setInterestRateData:58` |
| `UpdateReserveSource(reserveId, source)` | `IAaveOracle.sol:13` | `AaveOracle.setReserveSource:50` |
| `SetSpoke(spoke)` | `:17` | `AaveOracle.setSpoke:40` |
| `SetTokenizationSpokeImmutables(hub, assetId)` | `ITokenizationSpoke.sol:70` | `TokenizationSpokeInstance.initialize` |
| `RegisterSpoke(spoke, registered)` | `IPositionManagerBase.sol:13` | `PositionManagerBase.registerSpoke:36` |
| `SupplyOnBehalfOf(...)`, `RepayOnBehalfOf(...)` | `IGiverPositionManager.sol:17`, `:33` | `GiverPositionManager:36`, `:63` |
| `WithdrawApproval(...)`, `BorrowApproval(...)`, `WithdrawOnBehalfOf(...)`, `BorrowOnBehalfOf(...)` | `ITakerPositionManager.sol:55`, `:69`, `:84`, `:100` | `TakerPositionManager` |
| `UpdateConfigPermissions(...)`, `SetUsingAsCollateralOnBehalfOf(...)`, `UpdateUserRiskPremiumOnBehalfOf(...)`, `UpdateUserDynamicConfigOnBehalfOf(...)` | `IConfigPositionManager.sol:92`, `:106`, `:118`, `:128` | `ConfigPositionManager` |

Plus standard ERC-20 `Transfer`/`Approval` and ERC-4626 `Deposit`/`Withdraw` from
`TokenizationSpoke`, and OZ `AccessManager` events from `AccessManagerEnumerable`.

---
