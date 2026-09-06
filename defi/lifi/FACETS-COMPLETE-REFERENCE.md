# LI.FI Facets — Complete Reference

Every facet in `lifi/contracts/src/Facets/`, function by function. 42 files,
11,214 lines of Solidity. Nothing here is skipped: every contract, every
function, every parameter, every custom error, every external call into a bridge.

This is the **reference** layer. For the narrative explanation of how the Diamond
works and how a cross-chain route executes end to end, read
[`LIFI-DEEP-DIVE.md`](LIFI-DEEP-DIVE.md) first. For the shared libraries,
helpers and periphery contracts that every facet leans on, see
[`LIBRARIES-PERIPHERY-COMPLETE-REFERENCE.md`](LIBRARIES-PERIPHERY-COMPLETE-REFERENCE.md).

Every `file:line` below was verified with `grep -n` against the cloned source.
Paths are relative to `lifi/contracts/`.

---

## Table of contents

- [1. Facet inventory](#1-facet-inventory)
- [2. The common pattern](#2-the-common-pattern)
  - [2.1 `ILiFi.BridgeData` field by field](#21-ilifibridgedata-field-by-field)
  - [2.2 The two entry points and `_startBridge`](#22-the-two-entry-points-and-_startbridge)
  - [2.3 The modifier stack](#23-the-modifier-stack)
  - [2.4 Asset movement: `depositAsset` and `maxApproveERC20`](#24-asset-movement-depositasset-and-maxapproveerc20)
  - [2.5 The ASCII call flow](#25-the-ascii-call-flow)
  - [2.6 Namespaced diamond storage in a facet](#26-namespaced-diamond-storage-in-a-facet)
  - [2.7 EIP-712 signature gating](#27-eip-712-signature-gating)
- [3. Infrastructure facets](#3-infrastructure-facets)
  - [3.1 DiamondCutFacet](#31-diamondcutfacet)
  - [3.2 DiamondLoupeFacet](#32-diamondloupefacet)
  - [3.3 OwnershipFacet](#33-ownershipfacet)
  - [3.4 AccessManagerFacet](#34-accessmanagerfacet)
  - [3.5 WhitelistManagerFacet](#35-whitelistmanagerfacet)
  - [3.6 PeripheryRegistryFacet](#36-peripheryregistryfacet)
  - [3.7 WithdrawFacet](#37-withdrawfacet)
  - [3.8 EmergencyPauseFacet](#38-emergencypausefacet)
  - [3.9 CalldataVerificationFacet](#39-calldataverificationfacet)
- [4. Same-chain swaps: GenericSwapFacetV3](#4-same-chain-swaps-genericswapfacetv3)
- [5. Canonical rollup bridges](#5-canonical-rollup-bridges)
  - [5.1 ArbitrumBridgeFacet](#51-arbitrumbridgefacet)
  - [5.2 OptimismBridgeFacet](#52-optimismbridgefacet)
  - [5.3 PolygonBridgeFacet](#53-polygonbridgefacet)
  - [5.4 GnosisBridgeFacet](#54-gnosisbridgefacet)
  - [5.5 OmniBridgeFacet](#55-omnibridgefacet)
  - [5.6 MegaETHBridgeFacet](#56-megaethbridgefacet)
- [6. The Across family](#6-the-across-family)
  - [6.1 AcrossFacet (v2, legacy)](#61-acrossfacet-v2-legacy)
  - [6.2 AcrossFacetV4](#62-acrossfacetv4)
  - [6.3 AcrossFacetPackedV4](#63-acrossfacetpackedv4)
  - [6.4 AcrossV4SwapFacet](#64-acrossv4swapfacet)
- [7. Messaging and liquidity-network bridges](#7-messaging-and-liquidity-network-bridges)
  - [7.1 StargateFacetV2](#71-stargatefacetv2)
  - [7.2 AllBridgeFacet](#72-allbridgefacet)
  - [7.3 GlacisFacet](#73-glacisfacet)
  - [7.4 SquidFacet](#74-squidfacet)
  - [7.5 SymbiosisFacet](#75-symbiosisfacet)
  - [7.6 ThorSwapFacet](#76-thorswapfacet)
  - [7.7 FraxFacet](#77-fraxfacet)
  - [7.8 SupersetFacet](#78-supersetfacet)
- [8. Circle CCTP and burn/mint facets](#8-circle-cctp-and-burnmint-facets)
  - [8.1 CelerCircleBridgeFacet](#81-celercirclebridgefacet)
  - [8.2 PolymerCCTPFacet](#82-polymercctpfacet)
  - [8.3 PaxosTransitFacet](#83-paxostransitfacet)
- [9. Intent and solver facets](#9-intent-and-solver-facets)
  - [9.1 DeBridgeDlnFacet](#91-debridgedlnfacet)
  - [9.2 MayanFacet](#92-mayanfacet)
  - [9.3 RelayDepositoryFacet](#93-relaydepositoryfacet)
  - [9.4 LiFiIntentEscrowFacetV2](#94-lifiintentescrowfacetv2)
  - [9.5 EcoFacet](#95-ecofacet)
  - [9.6 NEARIntentsFacet](#96-nearintentsfacet)
  - [9.7 GardenFacet](#97-gardenfacet)
  - [9.8 ChainflipFacet](#98-chainflipfacet)
  - [9.9 LayerSwapFacet](#99-layerswapfacet)
  - [9.10 UnitFacet](#910-unitfacet)
- [10. Utility: GasZipFacet](#10-utility-gaszipfacet)
- [11. Cross-facet tables](#11-cross-facet-tables)
- [12. Use-case index](#12-use-case-index)

---

## 1. Facet inventory

42 facets. "Doc" links to `docs/<name>.md` in the repo.

| # | Facet | Lines | Category | Integrates | Doc |
|---|---|---|---|---|---|
| 1 | `DiamondCutFacet` | 26 | infrastructure | EIP-2535 | yes |
| 2 | `DiamondLoupeFacet` | 89 | infrastructure | EIP-2535 / EIP-165 | yes |
| 3 | `OwnershipFacet` | 92 | infrastructure | EIP-173 (2-step) | no |
| 4 | `AccessManagerFacet` | 52 | infrastructure | LibAccess | yes |
| 5 | `WhitelistManagerFacet` | 122 | infrastructure | LibAllowList | yes |
| 6 | `PeripheryRegistryFacet` | 57 | infrastructure | name→address registry | yes |
| 7 | `WithdrawFacet` | 91 | infrastructure | stuck-fund recovery | yes |
| 8 | `EmergencyPauseFacet` | 246 | infrastructure | facet removal / pause | yes |
| 9 | `CalldataVerificationFacet` | 366 | infrastructure | calldata decoding | yes |
| 10 | `GenericSwapFacetV3` | 609 | same-chain swap | any whitelisted DEX | yes |
| 11 | `ArbitrumBridgeFacet` | 157 | canonical rollup | Arbitrum Inbox / Gateway Router | yes |
| 12 | `OptimismBridgeFacet` | 210 | canonical rollup | OP Standard Bridge | yes |
| 13 | `PolygonBridgeFacet` | 114 | canonical rollup | Polygon PoS RootChainManager | yes |
| 14 | `GnosisBridgeFacet` | 121 | canonical rollup | Gnosis xDAI Bridge | yes |
| 15 | `OmniBridgeFacet` | 106 | canonical rollup | Gnosis OmniBridge | yes |
| 16 | `MegaETHBridgeFacet` | 218 | canonical rollup | MegaETH Standard Bridge | yes |
| 17 | `AcrossFacet` | 142 | liquidity network | Across SpokePool (v2) | yes |
| 18 | `AcrossFacetV4` | 271 | liquidity network | Across SpokePool V4 | yes |
| 19 | `AcrossFacetPackedV4` | 463 | liquidity network | Across SpokePool V4 (packed) | yes |
| 20 | `AcrossV4SwapFacet` | 838 | liquidity network | Across SpokePoolPeriphery | yes |
| 21 | `StargateFacetV2` | 165 | messaging | Stargate V2 / LayerZero OFT | yes |
| 22 | `AllBridgeFacet` | 322 | liquidity network | Allbridge Core | yes |
| 23 | `GlacisFacet` | 171 | messaging | Glacis Airlift | yes |
| 24 | `SquidFacet` | 217 | messaging | Squid Router (Axelar) | yes |
| 25 | `SymbiosisFacet` | 504 | liquidity network | Symbiosis MetaRouter | yes |
| 26 | `ThorSwapFacet` | 126 | liquidity network | THORChain Router | yes |
| 27 | `FraxFacet` | 564 | messaging | Frax Ferry / Tempo (LZ) | yes |
| 28 | `SupersetFacet` | 537 | messaging | Superset OFT (LayerZero) | yes |
| 29 | `CelerCircleBridgeFacet` | 137 | CCTP | Circle CCTP via Celer proxy | yes |
| 30 | `PolymerCCTPFacet` | 574 | CCTP | Circle CCTP v2 TokenMessenger | yes |
| 31 | `PaxosTransitFacet` | 208 | burn/mint | Paxos Transit Station | yes |
| 32 | `DeBridgeDlnFacet` | 284 | intent | deBridge DLN | yes |
| 33 | `MayanFacet` | 652 | intent | Mayan (Wormhole/Swift) | yes |
| 34 | `RelayDepositoryFacet` | 145 | intent | Relay Depository | yes |
| 35 | `LiFiIntentEscrowFacetV2` | 272 | intent | LI.FI's own escrow (OIF) | yes |
| 36 | `EcoFacet` | 477 | intent | Eco Portal | yes |
| 37 | `NEARIntentsFacet` | 339 | intent | NEAR Intents | yes |
| 38 | `GardenFacet` | 194 | intent | Garden Finance (HTLC) | yes |
| 39 | `ChainflipFacet` | 246 | intent | Chainflip Vault | yes |
| 40 | `LayerSwapFacet` | 297 | intent | LayerSwap | yes |
| 41 | `UnitFacet` | 237 | intent | Unit (Hyperliquid) | yes |
| 42 | `GasZipFacet` | 156 | utility | Gas.zip refuel | yes |

Only `OwnershipFacet` has no doc file.

---

## 2. The common pattern

Almost every bridge facet is the same 120-line shape. Learn it once here and the
30 bridge sections below become short.

### 2.1 `ILiFi.BridgeData` field by field

`src/Interfaces/ILiFi.sol:10-21`. This struct is the protocol's lingua franca:
every bridge facet takes it as its first argument, and the off-chain API builds
it.

```solidity
struct BridgeData {
    bytes32 transactionId;
    string bridge;
    string integrator;
    address referrer;
    address sendingAssetId;
    address receiver;
    uint256 minAmount;
    uint256 destinationChainId;
    bool hasSourceSwaps;
    bool hasDestinationCall;
}
```

| Field | Meaning | Who reads it |
|---|---|---|
| `transactionId` | Client-generated UUID that ties the source tx, the bridge message and the destination tx together. Not verified on-chain. | LI.FI's indexer, to show route status |
| `bridge` | Human-readable bridge name (`"across"`, `"stargate"`). Purely informational, emitted in the event. | analytics |
| `integrator` | Which partner/frontend originated the route. Drives fee attribution off-chain. | analytics, fee reporting |
| `referrer` | Optional referral address. Informational at this layer. | analytics |
| `sendingAssetId` | Token being bridged **after** any source swap. `address(0)` means native. | every facet, for deposit + approve |
| `receiver` | Who receives funds on the destination chain. For destination calls this is usually the destination `Receiver` contract, not the end user. | the bridge call |
| `minAmount` | Amount to bridge. **Mutated in place** by the swap entry point to the realized swap output. | the bridge call |
| `destinationChainId` | EVM chain id, or a LI.FI sentinel for non-EVM chains (see `LiFiData.sol`). | validation + chain-id mapping |
| `hasSourceSwaps` | Must match which entry point was called. Enforced by modifiers. | `containsSourceSwaps` / `doesNotContainSourceSwaps` |
| `hasDestinationCall` | Whether a destination-chain call is attached. Enforced by `doesNotContainDestinationCalls` on facets that cannot do it. | validation |

The two booleans exist so that a wallet simulating the calldata can check that
the *declared* intent matches the *actual* function selector. That is exactly
what `CalldataVerificationFacet` (§3.9) does.

**Events** (`ILiFi.sol:25-64`):

| Event | Line | Emitted where | Purpose |
|---|---|---|---|
| `LiFiTransferStarted(BridgeData)` | `:25` | end of every `_startBridge` | source-chain leg began |
| `LiFiTransferCompleted(bytes32 indexed, address, address, uint256, uint256)` | `:27-33` | destination `Executor`/`Receiver` | destination leg succeeded |
| `LiFiTransferRecovered(bytes32 indexed, address, address, uint256, uint256)` | `:35-41` | destination `Receiver` catch-block | destination swap failed, raw tokens forwarded |
| `LiFiGenericSwapCompleted(...)` | `:43-52` | `GenericSwapFacetV3` | same-chain swap, no bridge |
| `BridgeToNonEVMChain(bytes32 indexed, uint256 indexed, bytes)` | `:55-59` | non-EVM facets | receiver is not 20 bytes |
| `BridgeToNonEVMChainBytes32(bytes32 indexed, uint256 indexed, bytes32)` | `:60-64` | non-EVM facets | receiver fits in 32 bytes |
| `LiFiSwappedGeneric(...)` | `:67-75` | nothing (deprecated) | kept in the ABI so historic logs still decode |

### 2.2 The two entry points and `_startBridge`

Every bridge facet exposes exactly two externals plus one internal. Using
`AcrossFacet` as the canonical example:

**`startBridgeTokensViaAcross(BridgeData, AcrossData)`** — `src/Facets/AcrossFacet.sol:57-74`.
User already holds the token the bridge wants. The facet pulls it and bridges.

```solidity
function startBridgeTokensViaAcross(
    ILiFi.BridgeData memory _bridgeData,
    AcrossData calldata _acrossData
)
    external payable nonReentrant
    refundExcessNative(payable(msg.sender))
    validateBridgeData(_bridgeData)
    doesNotContainSourceSwaps(_bridgeData)
    doesNotContainDestinationCalls(_bridgeData)
{
    LibAsset.depositAsset(_bridgeData.sendingAssetId, _bridgeData.minAmount);
    _startBridge(_bridgeData, _acrossData);
}
```

**`swapAndStartBridgeTokensViaAcross(BridgeData, SwapData[], AcrossData)`** — `:80-100`.
User holds something else. The facet runs the swaps, then bridges the *realized*
output:

```solidity
_bridgeData.minAmount = _depositAndSwap(
    _bridgeData.transactionId,
    _bridgeData.minAmount,
    _swapData,
    payable(msg.sender)
);
_startBridge(_bridgeData, _acrossData);
```

The single most important line in the whole codebase is that assignment.
`_bridgeData` is `memory`, so overwriting `minAmount` with the swap's actual
output means the bridge always receives what the swap actually produced, and the
`minAmount` the caller passed in becomes the **slippage floor** enforced inside
`_depositAndSwap` (`src/Helpers/SwapperV2.sol:120-124`, which reverts
`CumulativeSlippageTooHigh(_minAmount, newBalance)`).

**`_startBridge(BridgeData, XData)`** — `:107-141`. Internal, does three things
and nothing else:
1. branch on native vs ERC20,
2. for ERC20, `LibAsset.maxApproveERC20(token, bridgeAddress, amount)`,
3. call the bridge, then `emit LiFiTransferStarted(_bridgeData)`.

### 2.3 The modifier stack

From `src/Helpers/Validatable.sol` and `src/Helpers/ReentrancyGuard.sol`:

| Modifier | Source | Reverts with | Checks |
|---|---|---|---|
| `nonReentrant` | `ReentrancyGuard.sol:30-36` | `ReentrancyError()` | diamond-storage flag at `keccak256("com.lifi.reentrancyguard")` |
| `validateBridgeData` | `Validatable.sol:14-25` | `InvalidReceiver()`, `InvalidAmount()`, `CannotBridgeToSameNetwork()` | receiver non-zero, `minAmount != 0`, `destinationChainId != block.chainid` |
| `noNativeAsset` | `:27-32` | `NativeAssetNotSupported()` | `sendingAssetId` is not `address(0)` |
| `onlyAllowSourceToken` | `:34-42` | `InvalidSendingToken()` | asset equals a facet-fixed token (e.g. USDC) |
| `onlyAllowDestinationChain` | `:44-52` | `InvalidDestinationChain()` | destination equals a facet-fixed chain |
| `containsSourceSwaps` | `:54-59` | `InformationMismatch()` | `hasSourceSwaps == true` |
| `doesNotContainSourceSwaps` | `:61-66` | `InformationMismatch()` | `hasSourceSwaps == false` |
| `doesNotContainDestinationCalls` | `:68-75` | `InformationMismatch()` | `hasDestinationCall == false` |
| `refundExcessNative` | `SwapperV2.sol:67` | — | snapshots native balance, returns the surplus to the caller after the body |

The `nonReentrant` guard is notable: it lives in **diamond storage**, not a
per-facet variable, so it is shared across every facet in the Diamond. Two
different facets cannot re-enter each other.

### 2.4 Asset movement: `depositAsset` and `maxApproveERC20`

`src/Libraries/LibAsset.sol`:

- `getOwnBalance(address)` `:31` — native or ERC20 balance of the Diamond.
- `transferAsset` `:45`, `transferNativeAsset` `:61`, `transferERC20` `:76`, `transferFromERC20` `:95`.
- `depositAsset(assetId, amount)` `:118` — for ERC20, `transferFromERC20(assetId, msg.sender, address(this), amount)`; for native, asserts `msg.value == amount`. This is the *only* place user funds enter the Diamond in the no-swap path.
- `depositAssets(SwapData[] calldata)` `:132` — same, for every swap leg that has `requiresDeposit`.
- `maxApproveERC20(IERC20, spender, amount)` `:149` — approves `type(uint256).max` if the current allowance is below `amount`. Saves gas across repeated bridges; the trade-off is a permanent max allowance from the Diamond to each bridge, which is why `WithdrawFacet` and `EmergencyPauseFacet` exist.
- `isNativeAsset(address)` `:190` — `assetId == address(0)`.
- `isContract(address)` `:200` — used by `LibSwap.swap` to reject EOA call targets.

`NULL_ADDRESS` is `address(0)` (`:23`) and doubles as the native-asset sentinel.

### 2.5 The ASCII call flow

```
                    user tx (EOA or smart wallet)
                             |
                             v
              LiFiDiamond.fallback()  --- delegatecall by selector --->  <Bridge>Facet
                             |
        +--------------------+--------------------+
        |                                         |
  no source swap                            with source swap
        |                                         |
        v                                         v
 LibAsset.depositAsset                _depositAndSwap(txId, minAmount, swaps, refundTo)
 (pull ERC20 / check msg.value)                    |
        |                              LibAsset.depositAssets(swaps)
        |                                          |
        |                              _executeSwaps  --(noLeftovers)-->
        |                                  |  for each swap:
        |                                  |  LibSwap.swap()
        |                                  |    - LibAllowList check (callTo+selector)
        |                                  |    - approve approveTo
        |                                  |    - CALL the DEX
        |                                  |    - emit AssetSwapped
        |                                          |
        |                              newBalance >= minAmount ? else
        |                                CumulativeSlippageTooHigh
        |                                          |
        +--------------------+---------------------+
                             v
                    _startBridge(bridgeData, xData)
                             |
                 native? --> bridge.deposit{value: amount}(...)
                 erc20?  --> LibAsset.maxApproveERC20(token, bridge, amount)
                             bridge.deposit(...)
                             |
                    emit LiFiTransferStarted(bridgeData)
                             |
                             v
              === chain boundary: relayer / message / attestation ===
                             |
                             v
          destination Receiver (ReceiverAcrossV4 / ReceiverStargateV2 / ...)
                             |
                    Executor.swapAndCompleteBridgeTokens
                             |
              success -> emit LiFiTransferCompleted
              failure -> send raw tokens, emit LiFiTransferRecovered
```

### 2.6 Namespaced diamond storage in a facet

Facets that need their own state (chain-id maps, config) declare a namespaced
struct so they cannot collide with another facet's slots. The idiom, from
`src/Facets/AllBridgeFacet.sol:33-34` and `:315-321`:

```solidity
bytes32 internal constant NAMESPACE = keccak256("com.lifi.facets.allbridge");

function getStorage() private pure returns (Storage storage s) {
    bytes32 namespace = NAMESPACE;
    // solhint-disable-next-line no-inline-assembly
    assembly {
        s.slot := namespace
    }
}
```

Facets using this pattern and their namespace strings:

| Facet | Namespace constant | Line |
|---|---|---|
| `ReentrancyGuard` (shared) | `com.lifi.reentrancyguard` | `Helpers/ReentrancyGuard.sol:11` |
| `AllBridgeFacet` | `com.lifi.facets.allbridge` | `:33` |
| `DeBridgeDlnFacet` | `com.lifi.facets.debridgedln` | `:28` |
| `EmergencyPauseFacet` | `com.lifi.facets.emergencyPauseFacet` | `:33` |
| `FraxFacet` | `com.lifi.facets.frax` | `:42` |
| `LayerSwapFacet` | `com.lifi.facets.layerswap` | `:28` |
| `MayanFacet` | `com.lifi.facets.mayan` | `:43` |
| `MegaETHBridgeFacet` | `com.lifi.facets.megaeth` | `:21` |
| `NEARIntentsFacet` | `com.lifi.facets.nearintents` | `:31` |
| `OptimismBridgeFacet` | `com.lifi.facets.optimism` | `:26` |
| `OwnershipFacet` | `com.lifi.facets.ownership` | `:16` |
| `PeripheryRegistryFacet` | `com.lifi.facets.periphery_registry` | `:13` |
| `PolymerCCTPFacet` | `com.lifi.facets.polymer_cctp` | `:81` |
| `SupersetFacet` | `com.lifi.facets.superset` | `:62` |
| `SymbiosisFacet` | `com.lifi.facets.symbiosis` | `:50` |
| `UnitFacet` | `com.lifi.facets.unit` | `:27` |

(The exact strings are verified per-facet in each section below.)

### 2.7 EIP-712 signature gating

Seven facets require a LI.FI backend signature on the quote before they will
bridge. They all follow the same shape: a `PAYLOAD_TYPEHASH` constant, a
`_domainSeparator()` built from `EIP712_DOMAIN_TYPEHASH`, `NAME_HASH`,
`VERSION_HASH`, `block.chainid` and `address(this)`, a `_verifySignature` that
`ecrecover`s and compares against a stored signer, plus a replay-protection
mapping and a deadline.

| Facet | Typehash constant | Verify fn | Replay guard | Deadline error |
|---|---|---|---|---|
| `AcrossV4SwapFacet` | `ACROSS_V4_SWAP_PAYLOAD_TYPEHASH` `:65` | `_verifySignatureIfRequired` `:274` | — | — |
| `EcoFacet` | `ECO_PAYLOAD_TYPEHASH` `:46` | `_verifySignature` `:420` | `IntentAlreadyFunded` `:33` | `SignatureExpired` `:35` |
| `LayerSwapFacet` | `LAYERSWAP_PAYLOAD_TYPEHASH` `:32` | `_verifySignature` `:234` | `RequestAlreadyProcessed` `:78` | `SignatureExpired` `:77` |
| `NEARIntentsFacet` | `NEARINTENTS_PAYLOAD_TYPEHASH` `:35` | `_verifySignature` `:274` | `QuoteAlreadyConsumed` `:93` | `QuoteExpired` `:96` |
| `SymbiosisFacet` | `SYMBIOSIS_PAYLOAD_TYPEHASH` `:54` | `_verifyOnchainSwapV3Signature` `:445` | `TransactionAlreadyProcessed` `:90` | `SignatureExpired` `:88` |
| `UnitFacet` | `UNIT_PAYLOAD_TYPEHASH` `:30` | `_verifySignature` `:174` | `TransactionAlreadyProcessed` `:61` | `SignatureExpired` `:57` |

Why: these bridges take a destination address or a fee that the contract cannot
validate on-chain (a Solana pubkey, a Hyperliquid account, an off-chain quote).
The signature makes LI.FI's backend the attesting party, and the replay guard
stops the same signed quote being reused.

---

## 3. Infrastructure facets

Nine facets that manage the Diamond itself rather than moving user funds. These
are the ones an auditor reads first, because they define who can change
everything else.

### 3.1 DiamondCutFacet

`src/Facets/DiamondCutFacet.sol` — 26 lines, version 1.0.0. Doc: `docs/DiamondCutFacet.md`.

The upgrade entry point of EIP-2535. It is the smallest facet in the repo and
the most dangerous: whoever can call it controls every other facet.

**`diamondCut(LibDiamond.FacetCut[] calldata _diamondCut, address _init, bytes calldata _calldata)`** — `:18-25`, external.

- **Purpose**: add, replace or remove any number of function selectors, then optionally `delegatecall` an initializer.
- **Parameters**:
  - `_diamondCut` — array of `{facetAddress, action, functionSelectors}`. `action` is `Add | Replace | Remove` (`LibDiamond.FacetCutAction`). For `Remove`, `facetAddress` must be `address(0)`.
  - `_init` — contract to `delegatecall` after the cut, for one-time storage setup. `address(0)` skips it.
  - `_calldata` — the encoded call for `_init`.
- **Checks**: `LibDiamond.enforceIsContractOwner()` at `:23`. Reverts `OnlyContractOwner()` for anyone else. All structural validation (adding a selector that already exists, removing an immutable function, `_init` not being a contract) lives inside `LibDiamond.diamondCut`.
- **State writes**: none directly; `LibDiamond.diamondCut` writes `selectorToFacetAndPosition`, `facetFunctionSelectors` and `facetAddresses` in diamond storage.
- **External calls**: `delegatecall` to `_init` if non-zero.
- **Events**: `DiamondCut(FacetCut[], address, bytes)` emitted by `LibDiamond`.
- **Access control**: contract owner only. In production the owner is `LiFiTimelockController` (`src/Security/LiFiTimelockController.sol`), so a cut is a two-step timelocked operation.
- **Gotchas**: there is no pause check here. `EmergencyPauseFacet` explicitly refuses to remove the facet holding `diamondCut.selector` (`EmergencyPauseFacet.sol:80-81` and `EmergencyPauseFacet.sol:164-165`) because doing so would make the Diamond permanently immutable.

### 3.2 DiamondLoupeFacet

`src/Facets/DiamondLoupeFacet.sol` — 89 lines, version 1.0.0. Doc: `docs/DiamondLoupeFacet.md`.

Read-only introspection required by EIP-2535, plus EIP-165. Block explorers and
LI.FI's own deployment tooling call these constantly.

| Function | Line | Returns | Notes |
|---|---|---|---|
| `facets()` | `:24-38` | `Facet[] {address facetAddress; bytes4[] functionSelectors;}` | Loops `ds.facetAddresses` and copies each facet's selector array. O(facets × selectors); on mainnet with ~40 facets this is a large but view-only call. |
| `facetFunctionSelectors(address _facet)` | `:43-55` | `bytes4[]` | Direct read of `ds.facetFunctionSelectors[_facet].functionSelectors`. Empty array means the facet is not registered. |
| `facetAddresses()` | `:59-67` | `address[]` | Direct read of `ds.facetAddresses`. |
| `facetAddress(bytes4 _functionSelector)` | `:73-80` | `address` | `ds.selectorToFacetAndPosition[sel].facetAddress`. Returns `address(0)` when unknown — the same lookup the Diamond's `fallback` performs before reverting `FunctionDoesNotExist()`. |
| `supportsInterface(bytes4 _interfaceId)` | `:83-88` | `bool` | Reads `ds.supportedInterfaces`, populated during deployment. |

All five are `external view override`, no access control, no state writes.

**Gotcha**: while the Diamond is paused, every selector including these points at
`EmergencyPauseFacet`, whose `fallback` reverts `DiamondIsPaused()`. Tooling that
assumes the loupe always answers will break during an incident. `LibDiamondLoupe`
provides internal equivalents so `EmergencyPauseFacet` can still read the facet
list while paused (`EmergencyPauseFacet.sol:159-161` and `EmergencyPauseFacet.sol:200`).

### 3.3 OwnershipFacet

`src/Facets/OwnershipFacet.sol` — 92 lines, version 1.0.0. **No doc file.**

EIP-173 ownership with a two-step handover, so a typo in an address cannot brick
the Diamond.

- **Namespace**: `keccak256("com.lifi.facets.ownership")` `:16-17`.
- **Storage**: `struct Storage { address newOwner; }` `:21-23`. The *actual* owner lives in `LibDiamond`'s own storage, not here; this slot only holds the pending owner.
- **Errors**: `NoNullOwner()` `:27`, `NewOwnerMustNotBeSelf()` `:28`, `NoPendingOwnershipTransfer()` `:29`, `NotPendingOwner()` `:30`.
- **Events**: `OwnershipTransferRequested(address indexed _from, address indexed _to)` `:34-37`; `OwnershipTransferred` comes from `IERC173`.

**`transferOwnership(address _newOwner)`** — `:43-54`, external.
Owner-only (`LibDiamond.enforceIsContractOwner()` `:44`). Reverts `NoNullOwner()`
if `_newOwner == address(0)` `:47`, and `NewOwnerMustNotBeSelf()` if it equals the
current owner `:49-50`. Writes `s.newOwner`, emits `OwnershipTransferRequested`.
Note it does **not** change the real owner yet.

**`cancelOwnershipTransfer()`** — `:57-64`, external. Owner-only. Reverts
`NoPendingOwnershipTransfer()` if `s.newOwner` is zero `:61-62`. Clears `s.newOwner`.
Emits nothing.

**`confirmOwnershipTransfer()`** — `:67-74`, external. Callable only by the pending
owner: reverts `NotPendingOwner()` if `msg.sender != s.newOwner` `:70`. Emits
`OwnershipTransferred(oldOwner, pendingOwner)`, calls
`LibDiamond.setContractOwner(_pendingOwner)` `:72`, then clears `s.newOwner`.

**`owner()`** — `:78-80`, external view. Returns `LibDiamond.contractOwner()`.

**`getStorage()`** — `:85-91`, private pure. The namespaced-slot assembly.

**Gotcha**: `cancelOwnershipTransfer` is owner-only, so a pending owner cannot
withdraw their own candidacy; only the current owner can revoke it.

### 3.4 AccessManagerFacet

`src/Facets/AccessManagerFacet.sol` — 52 lines, version 1.0.0. Doc: `docs/AccessManagerFacet.md`.

Per-selector, per-address permissions. This is how LI.FI grants a backend wallet
the right to call one specific admin function without making it the owner.

- **Events**: `ExecutionAllowed(address indexed account, bytes4 indexed method)` `:15`, `ExecutionDenied(...)` `:16`.

**`setCanExecute(bytes4 _selector, address _executor, bool _canExecute)`** — `:24-41`, external.

- **Checks**: reverts `CannotAuthoriseSelf()` if `_executor == address(this)` `:29-31` — note this check runs *before* the owner check. Then `LibDiamond.enforceIsContractOwner()` `:32`.
- **State writes**: `LibAccess.addAccess(_selector, _executor)` or `removeAccess`, which set `LibAccess.accessStorage().execAccess[selector][executor]`.
- **Events**: `ExecutionAllowed` or `ExecutionDenied`.
- **Access control**: owner only.
- **Gotcha**: granting the Diamond permission over itself would let any facet call any permissioned function via `address(this)`, hence the explicit guard.

**`addressCanExecuteMethod(bytes4 _selector, address _executor)`** — `:46-51`,
external view. Direct read of the mapping. No access control.

The consuming side is `LibAccess.enforceAccessControl()`, used by
`WithdrawFacet` (`WithdrawFacet.sol:43`, `:71`) and `WhitelistManagerFacet` (`WhitelistManagerFacet.sol:24`, `:36`).

### 3.5 WhitelistManagerFacet

`src/Facets/WhitelistManagerFacet.sol` — 122 lines, version 1.1.0. Doc: `docs/WhitelistManagerFacet.md`.

The security boundary for the entire aggregator. Because `LibSwap.swap` performs
an arbitrary `call` with attacker-supplied calldata, the only thing preventing
"call `USDC.transferFrom(victim, attacker, all)`" is this allowlist. Version 1.1.0
moved from whitelisting *contracts* to whitelisting **(contract, selector) pairs**,
which is much tighter.

**`setContractSelectorWhitelist(address _contract, bytes4 _selector, bool _whitelisted)`** — `:18-27`, external.
Owner **or** an address granted this selector via `AccessManagerFacet`
(`:23-25`). Delegates to `_setContractSelectorWhitelist`.

**`batchSetContractSelectorWhitelist(address[] calldata _contracts, bytes4[] calldata _selectors, bool _whitelisted)`** — `:30-51`, external.
Same access control. Reverts `InvalidConfig()` if the two arrays differ in length
`:38-40`. Loops with `unchecked { ++i; }`. Note the single `_whitelisted` flag
applies to the whole batch.

**`isContractSelectorWhitelisted(address _contract, bytes4 _selector)`** — `:54-59`, external view. Reads `LibAllowList.contractSelectorIsAllowed`.

**`getWhitelistedSelectorsForContract(address _contract)`** — `:62-66`, external view.

**`getAllContractSelectorPairs()`** — `:69-89`, external view. Returns parallel
arrays: `contracts[]` from `LibAllowList.getAllowedContracts()` and a
`bytes4[][]` of each one's selectors. O(n·m) — an off-chain-only call.

**`_setContractSelectorWhitelist(address _contract, bytes4 _selector, bool _whitelisted)`** — `:94-121`, internal.
The single funnel for all state changes.
- Reverts `CannotAuthoriseSelf()` if `_contract == address(this)` `:99-101`. Whitelisting the Diamond itself would let a "swap" re-enter any facet with arbitrary calldata.
- Reads the current status and **returns early without an event** if nothing changes `:107-109`.
- Calls `LibAllowList.addAllowedContractSelector` or `removeAllowedContractSelector`.
- Emits `ContractSelectorWhitelistChanged(_contract, _selector, _whitelisted)` (declared in `IWhitelistManagerFacet`).

**Gotcha worth internalising**: `SwapperV2._executeSwaps` documents that when
`approveTo != callTo`, the `approveTo` address must be whitelisted with the
sentinel selector `0xffffffff` (`GenericSwapFacetV3.sol:24` names it
`APPROVE_TO_ONLY_SELECTOR`). Without that rule, an attacker could get an
allowance granted to an address that was never vetted.

### 3.6 PeripheryRegistryFacet

`src/Facets/PeripheryRegistryFacet.sol` — 57 lines, version 1.0.0. Doc: `docs/PeripheryRegistryFacet.md`.

A `string => address` map so facets and off-chain code can find
`"Executor"`, `"ERC20Proxy"`, `"FeeCollector"` and friends by name.

- **Namespace**: `keccak256("com.lifi.facets.periphery_registry")` `:13-14`.
- **Storage**: `mapping(string => address) contracts` `:19`.
- **Event**: `PeripheryContractRegistered(string name, address contractAddress)` `:24`.

**`registerPeripheryContract(string calldata _name, address _contractAddress)`** — `:31-39`, external, owner-only (`:35`). Writes the mapping and emits. No zero-address check, no "is a contract" check.

**`getPeripheryContract(string calldata _name)`** — `:43-47`, external view. Returns `address(0)` for unknown names.

**`getStorage()`** — `:50-56`, private pure.

**Gotcha**: names are raw strings, so `"Executor"` and `"executor"` are different
keys, and a typo during registration silently yields `address(0)` at read time.

### 3.7 WithdrawFacet

`src/Facets/WithdrawFacet.sol` — 91 lines, version 1.0.0. Doc: `docs/WithdrawFacet.md`.

Recovery for funds that end up stuck in the Diamond — dust from swaps, tokens
sent by mistake, or a bridge refund that arrived at the Diamond instead of the
user.

- **Error**: `WithdrawFailed()` `:17`.
- **Event**: `LogWithdraw(address indexed _assetAddress, address _to, uint256 amount)` `:21-25`.

**`executeCallAndWithdraw(address payable _callTo, bytes calldata _callData, address _assetAddress, address _to, uint256 _amount)`** — `:35-59`, external.

- **Access**: owner, or an `AccessManagerFacet`-granted address (`:42-44`).
- **Checks**: `LibAsset.isContract(_callTo)` else `NotAContract()` `:48-49`.
- **Behaviour**: raw `_callTo.call(_callData)` `:52`. If it succeeds, `_withdrawAsset`; if not, revert `WithdrawFailed()` `:57`.
- **Use case**: a bridge refund that must first be *claimed* by calling the bridge, then swept. One transaction does both.
- **Gotcha**: this is an arbitrary call from the Diamond with **no allowlist check**. It is owner-gated precisely because it bypasses `LibAllowList` entirely.

**`withdraw(address _assetAddress, address _to, uint256 _amount)`** — `:65-74`, external. Same access control, straight to `_withdrawAsset`.

**`_withdrawAsset(address _assetAddress, address _to, uint256 _amount)`** — `:82-90`, internal. If `_to` is zero, defaults to `msg.sender` `:87`. Calls `LibAsset.transferAsset` (handles native vs ERC20), emits `LogWithdraw`.

### 3.8 EmergencyPauseFacet

`src/Facets/EmergencyPauseFacet.sol` — 246 lines, version 1.0.1. Doc: `docs/EmergencyPauseFacet.md`.

The incident-response tool. Its trick is that a **non-multisig hot wallet** can
act instantly, while only the multisig owner can undo the damage. That asymmetry
is the whole design.

- **Immutables**: `pauserWallet` `:31` (public), `_emergencyPauseFacetAddress` `:36` (its own deployed address, captured in the constructor `:55` — necessary because `address(this)` inside a `delegatecall` is the Diamond, not the facet).
- **Namespace**: `keccak256("com.lifi.facets.emergencyPauseFacet")` `:33-34`.
- **Storage**: `struct Storage { IDiamondLoupe.Facet[] facets; }` `:38-40` — the snapshot taken at pause time so the state can be restored.
- **Errors**: `FacetIsNotRegistered()` `:26`, `NoFacetToPause()` `:27`, plus `UnAuthorized()`, `InvalidCallData()`, `DiamondIsPaused()` from `GenericErrors`.
- **Events**: `EmergencyFacetRemoved(address indexed facetAddress, address indexed msgSender)` `:18-21`, `EmergencyPaused(address indexed msgSender)` `:22`, `EmergencyUnpaused(address indexed msgSender)` `:23`.
- **Modifier**: `OnlyPauserWalletOrOwner` `:43-49` — `msg.sender` must be `pauserWallet` or `LibDiamond.contractOwner()`, else `UnAuthorized()`.

**`removeFacet(address _facetAddress)`** — `:63-87`, external, `OnlyPauserWalletOrOwner`.
Surgical response: kill one compromised integration, leave the rest running.
- Reverts `InvalidCallData()` if the target is this facet `:67-68` (you cannot remove your own kill switch).
- Reads the facet's selectors from diamond storage `:71-74`; reverts `FacetIsNotRegistered()` if empty `:77`.
- Reverts `InvalidCallData()` if `functionSelectors[0] == DiamondCutFacet.diamondCut.selector` `:80-81` — protecting upgradeability.
- Calls `LibDiamond.removeFunctions(address(0), functionSelectors)` `:84`, emits `EmergencyFacetRemoved`.

**`pauseDiamond()`** — `:98-127`, external, `OnlyPauserWalletOrOwner`.
Total shutdown. Rather than removing selectors, it **repoints every one of them at
this facet**, whose `fallback` reverts `DiamondIsPaused()` — so callers get a
meaningful error instead of `FunctionDoesNotExist()`.
- `_getAllFacetFunctionSelectorsToBeRemoved()` returns every facet except itself `:103`.
- Reverts `NoFacetToPause()` if that list is empty `:106`.
- For each facet: `LibDiamond.replaceFunctions(_emergencyPauseFacetAddress, facets[i].functionSelectors)` `:112-115`, then `s.facets.push(facets[i])` `:118` to record how to restore it.
- Emits `EmergencyPaused`.
- **Gotcha, acknowledged in the source** `:94-97`: this loop can run out of gas if there are too many facets. LI.FI mitigates with a forked-mainnet test rather than a code change.

**`unpauseDiamond(address[] calldata _blacklist)`** — `:132-190`, external, **owner only** `:134`.
- Replays the snapshot: for each stored facet, `LibDiamond.replaceFunctions(facetAddress, functionSelectors)` `:141-144`.
- Then processes the blacklist: for each address, fetch its current selectors via `LibDiamondLoupe.facetFunctionSelectors` `:159-161`, skip if it is the DiamondCutFacet `:164-165`, otherwise build a `FacetCut` with `action: Remove` and `facetAddress: address(0)` and call `LibDiamond.diamondCut` `:171-178`.
- `delete s.facets` `:187`, emit `EmergencyUnpaused`.
- **Why restore-then-remove instead of just skipping?** The comment at `:153-156` explains it: skipping would leave those selectors pointing at `EmergencyPauseFacet` (so they would revert `DiamondIsPaused()` forever), and checking membership per facet inside the main loop would be expensive.
- **Gotcha**: `currentSelectors[0]` is read without a length check at `:164`; a blacklisted address with no selectors would revert on array bounds.

**`_getAllFacetFunctionSelectorsToBeRemoved()`** — `:194-227`, internal view. Copies `LibDiamondLoupe.facets()` minus the entry whose `facetAddress == _emergencyPauseFacetAddress`. Sizes the result `allFacets.length - 1`, which underflows if this facet is not registered.

**`getStorage()`** — `:230-236`, private pure.

**`fallback()`** — `:240-242`, external payable. Reverts `DiamondIsPaused()`. This is the destination of every selector while paused.

**`receive()`** — `:245`, external payable, empty. Added only to silence a compiler warning after the fallback was introduced.

### 3.9 CalldataVerificationFacet

`src/Facets/CalldataVerificationFacet.sol` — 366 lines, version 2.0.0. Doc: `docs/CalldataVerificationFacet.md`.

Pure functions that decode LI.FI calldata so a wallet, a security scanner or a
clear-signing device can show the user what they are actually signing. Nothing
here touches state; the facet exists so that verification logic lives at the same
address as the thing being verified.

**`extractBridgeData(bytes calldata data)`** — `:21-25`, external pure. Delegates to `_extractBridgeData`.

**`extractSwapData(bytes calldata data)`** — `:30-34`, external pure. Delegates to `_extractSwapData`.

**`extractData(bytes calldata data)`** — `:40-54`, external pure. Returns both; only decodes swap data when `bridgeData.hasSourceSwaps` is true `:51-53`.

**`extractMainParameters(bytes calldata data)`** — `:65-100`, external pure.
Returns `(bridge, sendingAssetId, receiver, amount, destinationChainId, hasSourceSwaps, hasDestinationCall)`.
The important nuance is at `:82-89`: when there are source swaps, the
user-facing "what am I spending" is `swapData[0].sendingAssetId` and
`swapData[0].fromAmount`, **not** the bridge data's fields (which describe the
post-swap asset). This is exactly the number a wallet must show.

**`extractNonEVMAddress(bytes calldata data)`** — `:111-145`, external pure.
Hand-rolled calldata surgery to pull a `bytes32` non-EVM receiver out of the
bridge-specific struct.
- The struct's ABI head offset sits at `0x64` when there are source swaps (selector + bridgeData head + swapData head) and at `0x44` when there are not `:124-132`.
- The receiver is the first field of that struct, so it lives at `offset + 0x24` in the `bytes memory` copy (`+0x20` for the length prefix, `+0x04` for the selector) `:143`.
- **Bounds check** `:139-140`: `if (callData.length < 0x24 || offset > callData.length - 0x24) revert InvalidCallData();`. Written as a subtraction guard specifically so an attacker-supplied `offset` near `2^256` reverts cleanly rather than panicking on overflow. The comment at `:117-122` explains why: `offset` is read straight from calldata and is not covered by the `BridgeData` decode.
- **Documented limitation** `:103-108`: only works for facets whose bridge struct is dynamically encoded and starts with the `bytes32` receiver (Mayan, NEARIntents, AcrossV4). For Chainflip, Eco, DeBridgeDln, Garden, Glacis, AllBridge and PolymerCCTP the result is undefined or the call reverts.

**`extractGenericSwapParameters(bytes calldata data)`** — `:154-210`, **public** pure (the only public function here).
- Reverts `InvalidCallData()` if `data.length <= 484` `:175-177`; the comment at `:167-174` itemises the 484 bytes.
- Branches on selector: the three single-swap `GenericSwapFacetV3` selectors decode a single `LibSwap.SwapData` `:182-197`; everything else decodes a `SwapData[]` `:198-204`.
- Derives `sendingAssetId`/`amount` from `swapData[0]` and `receivingAssetId` from the **last** swap `:207-209`, which is correct for multi-hop.

**`validateCalldata(bytes calldata data, string calldata bridge, address sendingAssetId, address receiver, uint256 amount, uint256 destinationChainId, bool hasSourceSwaps, bool hasDestinationCall)`** — `:225-258`, external pure.
Returns a single bool. Each field can be waived with a sentinel: the empty string
for `bridge`, `0xFFfFfFffFFfffFFfFFfFFFFFffFFFffffFfFFFfF` for the two addresses,
`type(uint256).max` for the two numbers. The two booleans are always compared.
Strings are compared by `keccak256(abi.encodePacked(...))` `:237-242`.

**`validateDestinationCalldata(bytes calldata data, bytes calldata callTo, bytes calldata dstCalldata)`** — `:265-317`, external pure.
Confirms that the destination-chain payload embedded in the calldata matches what
the caller expects.
- Handles exactly two selectors: `StargateFacetV2.startBridgeTokensViaStargate` `:275-291` and `swapAndStartBridgeTokensViaStargate` `:292-313`.
- For each it decodes the `StargateData`, then checks `keccak256(dstCalldata) == keccak256(stargateDataV2.sendParams.composeMsg)` **and** that `callTo` matches `sendParams.to`.
- **Returns `false` for every other bridge** `:316`. This is the honest answer, but it means a wallet cannot verify destination calls for Across, Mayan, Squid and the rest through this function. Worth knowing before you rely on it.

**`_extractBridgeData(bytes calldata data)`** — `:324-328`, internal pure. `abi.decode(data[4:], (ILiFi.BridgeData))`. Works because `BridgeData` is always the first parameter of every bridge function.

**`_extractSwapData(bytes calldata data)`** — `:333-340`, internal pure. `abi.decode(data[4:], (ILiFi.BridgeData, LibSwap.SwapData[]))`, discarding the first.

**`_compareBytesToBytes32CallTo(bytes memory callTo, bytes32 callToBytes32)`** — `:342-365`, private pure.
`require(callTo.length >= 20, "Invalid callTo length; expected at least 20 bytes")`
`:347-350` — the **only string-based require left in the facets**; everything else
uses custom errors. Loads the first word of `callTo` as an address `:354-356`
(note this reads 32 bytes and truncates, so it takes the *high* 20 bytes), then
truncates `callToBytes32` to an address `:360-362` and compares. There is a
`TODO(EXSC-626)` at `:359` to migrate this to `LibBytes.toAddressUnchecked`.

---

## 4. Same-chain swaps: GenericSwapFacetV3

`src/Facets/GenericSwapFacetV3.sol` — 609 lines, version 2.0.0. Doc: `docs/GenericSwapFacetV3.md`.

No bridge involved. The user swaps token A for token B on one chain, routed
through any whitelisted DEX. It exists as a separate facet from `SwapperV2`
because a same-chain swap is the highest-volume, most gas-sensitive operation
LI.FI performs, so this facet **duplicates** the swap logic in a flattened,
loop-free form rather than reusing the generic helpers.

- **Inherits**: `ILiFi` only. No `ReentrancyGuard`, no `Validatable`, no `SwapperV2`.
- **Immutable**: `address public immutable NATIVE_ADDRESS` `:19` — set in the constructor `:28-30`. This is the *label* used in events for native, which may differ from `address(0)`.
- **Constant**: `bytes4 private constant APPROVE_TO_ONLY_SELECTOR = 0xffffffff` `:24` — the sentinel selector that marks an address as "allowance target only, never call target" in `LibAllowList`.
- **Errors used**: `ContractCallNotAllowed()`, `CumulativeSlippageTooHigh(uint256,uint256)`, `NativeAssetTransferFailed()` — all from `GenericErrors`.

**Why six functions instead of one.** The matrix is {single, multiple} × {ERC20→ERC20,
ERC20→native, native→ERC20}. Each combination is written out separately so that
the hot path contains no branches, no array iteration where a single swap
suffices, and no memory expansion. `native→native` is absent because it is
meaningless.

### 4.1 The six external entry points

Every one takes the same first five parameters:
`(bytes32 _transactionId, string calldata _integrator, string calldata _referrer, address payable _receiver, uint256 _minAmountOut, ...)`.

| Function | Line | Payable | Swap data | Ends by |
|---|---|---|---|---|
| `swapTokensSingleV3ERC20ToERC20` | `:43-90` | no | `SwapData` | `transferERC20` |
| `swapTokensSingleV3ERC20ToNative` | `:99-144` | no | `SwapData` | raw `.call{value}` |
| `swapTokensSingleV3NativeToERC20` | `:153-223` | **yes** | `SwapData` | `transferERC20` |
| `swapTokensMultipleV3ERC20ToNative` | `:234-252` | no | `SwapData[]` | `_transferNativeTokensAndEmitEvent` |
| `swapTokensMultipleV3ERC20ToERC20` | `:261-279` | no | `SwapData[]` | `_transferERC20TokensAndEmitEvent` |
| `swapTokensMultipleV3NativeToERC20` | `:288-305` | **yes** | `SwapData[]` | `_transferERC20TokensAndEmitEvent` |

**`swapTokensSingleV3ERC20ToERC20`** — `:43-90`.
1. `_depositAndSwapERC20Single(_swapData, _receiver)` `:51` — pulls, checks allowlist, approves, calls the DEX, refunds leftover input.
2. Reads `amountReceived = IERC20(receivingAssetId).balanceOf(address(this))` `:57-59`. Note: **the entire contract balance**, not the delta.
3. Reverts `CumulativeSlippageTooHigh(_minAmountOut, amountReceived)` if short `:62-63`.
4. `LibAsset.transferERC20(receivingAssetId, _receiver, amountReceived)` `:66`.
5. Emits **both** `LibSwap.AssetSwapped` `:70-78` and `ILiFi.LiFiGenericSwapCompleted` `:80-89`. The comment at `:68` says both are required for tracking: the first is the per-hop record, the second is the user-level summary.

**`swapTokensSingleV3ERC20ToNative`** — `:99-144`. Identical shape, but
`amountReceived = address(this).balance` `:110` and delivery is a raw
`_receiver.call{value: amountReceived}("")` `:118` with `NativeAssetTransferFailed()`
on failure `:119`.

**`swapTokensSingleV3NativeToERC20`** — `:153-223`. The one that does not use
`_depositAndSwapERC20Single`, because there is nothing to pull — the native
arrives as `msg.value`.
- Allowlist check inline `:163-168`: `LibAllowList.contractSelectorIsAllowed(callTo, bytes4(_swapData.callData[:4]))` else `ContractCallNotAllowed()`.
- `callTo.call{value: msg.value}(_swapData.callData)` `:172-174`; on failure `LibUtil.revertWith(res)` `:176` bubbles the DEX's own revert reason rather than masking it.
- `_returnPositiveSlippageNative(_receiver)` `:179` refunds unspent native *before* measuring the output.
- Then the usual balance read, slippage check, transfer and two events.
- **Note there is no `approveTo` check here** — correct, since native needs no allowance.
- **Known issue, flagged in-source** `:195-201` (`TODO(EXSC-850)`): the events emit the caller-declared `_swapData.fromAmount` as the native input, which is unrelated to `msg.value` and is therefore "decoration" that gets priced downstream as real volume. The team's own note says to emit the native actually consumed on the next version bump.

**The three multiple-swap variants** are thin: deposit (if ERC20), `_executeSwaps`,
then the matching transfer-and-emit helper. `swapTokensMultipleV3NativeToERC20`
`:288-305` skips the deposit step entirely since `msg.value` is already present.

### 4.2 Private helpers

**`_depositMultipleERC20Tokens(SwapData[] calldata _swapData)`** — `:308-338`.
Loops every swap and, **only when `currentSwap.requiresDeposit` is true** `:324`,
pulls `fromAmount` of `sendingAssetId` from `msg.sender`. Skipping the pull is how
intermediate hops avoid re-pulling tokens the contract already holds.
- **Known issue, flagged in-source** `:318-323` (`TODO(EXSC-850)`): `requiresDeposit` is caller-supplied, so a caller can skip the pull while the event helpers still emit `fromAmount` unconditionally — the same "unbacked amount" spoof as the native path, ERC20-denominated. It is an event-accuracy problem, not a fund-loss problem, because the swap itself would fail without real tokens.

**`_depositAndSwapERC20Single(SwapData calldata _swapData, address _receiver)`** — `:340-393`.
The single-swap workhorse, in order:
1. `LibAsset.transferFromERC20(sendingAssetId, msg.sender, address(this), fromAmount)` `:347-352`.
2. Allowlist the (callTo, selector) pair `:358-363` else `ContractCallNotAllowed()`.
3. **If `approveTo != callTo`**, require `approveTo` to be whitelisted against `APPROVE_TO_ONLY_SELECTOR` `:366-372` else `ContractCallNotAllowed()`. This closes the allowance-leak hole: without it, a caller could name any address as the approval target.
4. `LibAsset.maxApproveERC20(IERC20(sendingAssetId), approveTo, fromAmount)` `:379-383`. The comment `:374-378` documents that this uses Solady's `safeApproveWithRetry`, which tries `approve(spender, max)` first and only zeroes-then-retries on failure — the USDT compatibility dance.
5. `callTo.call(callData)` `:387`; `LibUtil.revertWith(res)` on failure `:389`.
6. `_returnPositiveSlippageERC20(sendingAssetId, _receiver)` `:392`.

**`_executeSwaps(SwapData[] calldata _swapData, bytes32 _transactionId, address _receiver)`** — `:399-504`.
The multi-hop loop. Per iteration:
- Allowlist the (callTo, selector) pair `:419-426`, and the `approveTo` sentinel if it differs `:429-437`.
- **Native branch** `:439-453`: `callTo.call{value: fromAmount}(callData)`, then `_returnPositiveSlippageNative` **only if `sendingAssetId != receivingAssetId`**.
- **ERC20 branch** `:454-480`: `maxApproveERC20`, `callTo.call(callData)`, then `_returnPositiveSlippageERC20` under the same condition.
- The `sendingAssetId != receivingAssetId` guard `:452`, `:478` distinguishes a *swap* from a *fee collection* or wrap, where input and output assets are the same. Refunding after a fee collection would return the whole amount before the real swap ran. The comment at `:449-451` says exactly this.
- Emits `LibSwap.AssetSwapped` per hop `:488-498`, reading the receiving balance live.
- **Documented inaccuracy** `:483-487`: if the contract already held a balance of the receiving asset, the emitted `toAmount` is `swapOutput + existingBalance`. Accepted for gas.
- **Documented limitation** `:395-398`: this function does not work when two swaps share the same `sendingAssetId`, because the positive-slippage refund after the first swap sweeps the tokens the second swap needed. The suggested workaround is the original `GenericSwapFacet.swapTokensGeneric`.

**`_transferERC20TokensAndEmitEvent(...)`** — `:506-537`. Takes `finalAssetId` from
the **last** swap `:515-516`, reads the full contract balance `:517`, enforces
`_minAmountOut` `:520-521`, transfers, emits `LiFiGenericSwapCompleted` using
`_swapData[0]` for the input side.

**`_transferNativeTokensAndEmitEvent(...)`** — `:539-571`. Same with
`address(this).balance` and a raw call.

**`_returnPositiveSlippageERC20(address sendingAssetId, address receiver)`** — `:574-596`.
Refunds leftover input tokens. The threshold is `> 1`, not `> 0` `:588`. The
comment `:584-587` explains: rebasing tokens can leave 1 wei that is not
transferable, which would revert the whole transaction. LI.FI accepts 1 wei of
dust stranded per transaction instead.

**`_returnPositiveSlippageNative(address receiver)`** — `:599-608`. Refunds the
entire native balance if non-zero, reverting `NativeAssetTransferFailed()` on
failure. Threshold is `> 0` here since native has no rebasing quirk.

### 4.3 Call flow

```
swapTokensSingleV3ERC20ToERC20(txId, integrator, referrer, receiver, minOut, swapData)
  |
  +-- _depositAndSwapERC20Single(swapData, receiver)
  |     |-- LibAsset.transferFromERC20(sendingAsset, msg.sender, diamond, fromAmount)
  |     |-- LibAllowList.contractSelectorIsAllowed(callTo, selector)   -> else ContractCallNotAllowed
  |     |-- if approveTo != callTo:
  |     |     LibAllowList.contractSelectorIsAllowed(approveTo, 0xffffffff) -> else ContractCallNotAllowed
  |     |-- LibAsset.maxApproveERC20(sendingAsset, approveTo, fromAmount)
  |     |-- callTo.call(callData)                    <-- the DEX
  |     |     failure -> LibUtil.revertWith(res)     (bubbles the DEX revert)
  |     +-- _returnPositiveSlippageERC20(sendingAsset, receiver)   (if balance > 1)
  |
  |-- amountReceived = IERC20(receivingAsset).balanceOf(diamond)
  |-- amountReceived < minOut ? revert CumulativeSlippageTooHigh
  |-- LibAsset.transferERC20(receivingAsset, receiver, amountReceived)
  |-- emit LibSwap.AssetSwapped
  +-- emit ILiFi.LiFiGenericSwapCompleted
```

**When the router picks this facet**: any route where source and destination
chain are the same.

---

## 5. Canonical rollup bridges

Six facets that wrap a chain's *official* bridge. These are the slowest routes
(minutes to deposit, up to 7 days to withdraw through a fraud-proof window) but
the most trust-minimised: security equals the rollup's own security, with no
relayer, no liquidity provider and no attestation service in the path.

All six share the standard two-entry-point shape from §2. None supports
destination calls (`doesNotContainDestinationCalls` on every entry point), which
makes sense — a canonical deposit delivers tokens to an address, nothing more.

### 5.1 ArbitrumBridgeFacet

`src/Facets/ArbitrumBridgeFacet.sol` — 157 lines, version 1.0.0. Doc: `docs/ArbitrumBridgeFacet.md`.

**Integrates**: Arbitrum's `Inbox` (for native ETH, via retryable tickets) and
`GatewayRouter` (for ERC20). **Trust**: Arbitrum's rollup security only.

- **Immutables**: `IGatewayRouter private immutable gatewayRouter` `:27`, `IGatewayRouter private immutable inbox` `:31`. Set in the constructor `:49-52`.
- **`struct ArbitrumData`** `:38-42`:
  - `maxSubmissionCost` — gas deducted from the user's L2 balance to cover the base submission fee for the retryable ticket.
  - `maxGas` — max gas for L2 execution.
  - `maxGasPrice` — the bid for L2 gas.

**`startBridgeTokensViaArbitrumBridge(BridgeData, ArbitrumData)`** — `:59-81`,
external payable, `nonReentrant refundExcessNative(msg.sender) doesNotContainSourceSwaps doesNotContainDestinationCalls validateBridgeData`.
Computes `cost = maxSubmissionCost + maxGas * maxGasPrice` `:71-73`, deposits the
asset, calls `_startBridge`. The user must send `minAmount + cost` as `msg.value`
for native, or just `cost` for ERC20; `refundExcessNative` returns any surplus.

**`swapAndStartBridgeTokensViaArbitrumBridge(BridgeData, SwapData[], ArbitrumData)`** — `:87-113`.
Same, but calls the **five-argument** `_depositAndSwap` overload
(`SwapperV2.sol:135`) passing `cost` as `_nativeReserve` `:104-110`. That reserve
is the reason the overload exists: without it, the positive-slippage refund would
sweep the native the bridge fee needs.

**`_startBridge(BridgeData, ArbitrumData, uint256 _cost)`** — `:121-156`, private.
- **Native** `:126-138`: `inbox.unsafeCreateRetryableTicket{value: minAmount + cost}(receiver, minAmount /*l2CallValue*/, maxSubmissionCost, receiver /*excessFeeRefundAddress*/, receiver /*callValueRefundAddress*/, maxGas, maxGasPrice, "")`. Both refund addresses are the end user, so failed-ticket funds are recoverable by them and not stranded in the Diamond. The empty `data` argument means no L2 call.
- **ERC20** `:139-153`: approve `gatewayRouter.getGateway(sendingAssetId)` — note the gateway is looked up per token, not a fixed address — then `gatewayRouter.outboundTransfer{value: _cost}(token, receiver, minAmount, maxGas, maxGasPrice, abi.encode(maxSubmissionCost, ""))`. The last argument is Arbitrum's packed extra-data format.
- Emits `LiFiTransferStarted`.

**Gotcha**: `unsafeCreateRetryableTicket` is the "unsafe" variant because it does
not enforce that the caller's address is aliased. Here it is fine because the
refund addresses are explicit.

### 5.2 OptimismBridgeFacet

`src/Facets/OptimismBridgeFacet.sol` — 210 lines, version 1.0.0. Doc: `docs/OptimismBridgeFacet.md`.

**Integrates**: the OP Stack `L1StandardBridge`, plus a registry of per-token
custom bridges (Synthetix and friends run their own).

- **Namespace**: `keccak256("com.lifi.facets.optimism")` `:26-27`.
- **`struct Storage`** `:31-35`: `mapping(address => IL1StandardBridge) bridges` (per-token override), `IL1StandardBridge standardBridge` (the default), `bool initialized`.
- **`struct Config`** `:37-40`: `{address assetId; address bridge;}`.
- **`struct OptimismData`** `:42-46`: `assetIdOnL2` (the token's address on L2), `l2Gas` (uint32 gas limit for the L2 message), `isSynthetix` (selects a different bridge method).
- **Events**: `OptimismInitialized(Config[])` `:50`, `OptimismBridgeRegistered(address indexed assetId, address bridge)` `:51`.

**`initOptimism(Config[] calldata configs, IL1StandardBridge standardBridge)`** — `:57-82`, external, owner-only `:61`.
Reverts `AlreadyInitialized()` if `s.initialized` `:65-67`, and `InvalidConfig()`
for any zero bridge address `:70-72`. Populates the map, sets the default, flips
`initialized`, emits.

**`registerOptimismBridge(address assetId, address bridge)`** — `:89-103`, external, owner-only.
Reverts `NotInitialized()` `:94` or `InvalidConfig()` for a zero address `:96-98`.
Adds or overwrites one mapping entry.

**`startBridgeTokensViaOptimismBridge(BridgeData, OptimismData)`** — `:108-125`, and
**`swapAndStartBridgeTokensViaOptimismBridge(BridgeData, SwapData[], OptimismData)`** — `:131-151`.
The standard pair; the swap variant uses the **four-argument** `_depositAndSwap`
(no native reserve) because OP deposits carry no L1-paid fee.

**`_startBridge(BridgeData, OptimismData)`** — `:158-200`, private.
- Resolves the bridge: `s.bridges[sendingAssetId]`, falling back to `s.standardBridge` when zero `:163-170`.
- **Native** `:172-177`: `bridge.depositETHTo{value: minAmount}(receiver, l2Gas, "")`.
- **ERC20** `:178-197`: approve, then either `bridge.depositTo(receiver, minAmount)` when `isSynthetix` `:186`, or `bridge.depositERC20To(l1Token, l2Token, receiver, amount, l2Gas, "")` `:188-195`.
- Emits `LiFiTransferStarted`.

**Gotcha**: unlike MegaETH below, this facet does **not** check `assetIdOnL2` for
zero, and `_startBridge` does not re-check `initialized`. A misconfigured L2
address would deposit into a token mapping that does not exist.

### 5.3 PolygonBridgeFacet

`src/Facets/PolygonBridgeFacet.sol` — 114 lines, version 1.0.0. Doc: `docs/PolygonBridgeFacet.md`.

**Integrates**: Polygon PoS `RootChainManager` and its `ERC20Predicate`. Note the
approval goes to the **predicate**, not the manager — a common source of confusion.

- **Immutables**: `IRootChainManager private immutable rootChainManager` `:20`, `address private immutable erc20Predicate` `:24`.
- **No bridge-specific data struct.** Both entry points take only `BridgeData` (plus swaps).

**`startBridgeTokensViaPolygonBridge(BridgeData)`** — `:40-56`.
**`swapAndStartBridgeTokensViaPolygonBridge(BridgeData, SwapData[])`** — `:61-80`.

**`_startBridge(BridgeData)`** — `:86-113`, private.
- **Native** `:89-92`: `rootChainManager.depositEtherFor{value: minAmount}(receiver)`.
- **ERC20** `:93-109`: reads `childToken = rootChainManager.rootToChildToken(sendingAssetId)` `:94-96` (assigned but **never used** — dead code, `:87` declares it), approves `erc20Predicate` `:98-102`, then `rootChainManager.depositFor(receiver, rootToken, abi.encode(minAmount))` `:105-109`. Polygon's ABI takes the amount as encoded `depositData` rather than a plain uint.
- Emits `LiFiTransferStarted`.

### 5.4 GnosisBridgeFacet

`src/Facets/GnosisBridgeFacet.sol` — 121 lines, version 2.0.0. Doc: `docs/GnosisBridgeFacet.md`.

**Integrates**: the Gnosis xDAI bridge router. The narrowest facet in the repo:
it bridges **only DAI or USDS**, **only to Gnosis Chain**, and **never native**.

- **Constants**: `DAI = 0x6B17...1d0F` `:20`, `USDS = 0xdC03...384F` `:22`, `GNOSIS_CHAIN_ID = 100` `:25`.
- **Immutable**: `IGnosisBridgeRouter private immutable GNOSIS_BRIDGE_ROUTER` `:28`. The constructor `:34-39` reverts `InvalidConfig()` on a zero address — one of the few facets that validates its own constructor argument.

**`startBridgeTokensViaGnosisBridge(BridgeData)`** — `:45-66`. Note it is **not
payable** `:48-49` (native is impossible here). Modifiers include
`onlyAllowDestinationChain(_bridgeData, GNOSIS_CHAIN_ID)` `:53`. Body reverts
`InvalidSendingToken()` unless the asset is DAI or USDS `:55-60`.

**`swapAndStartBridgeTokensViaGnosisBridge(BridgeData, SwapData[])`** — `:71-100`.
Payable (the swap leg may need native). Its token check is stricter `:84-92`: the
asset must be DAI or USDS **and**, if there are swaps, must equal
`_swapData[_swapData.length - 1].receivingAssetId`. That second clause guards
against a mismatch between what the swap produces and what the bridge is told to
send.

**`_startBridge(BridgeData)`** — `:106-120`, private. Approve the router, call
`GNOSIS_BRIDGE_ROUTER.relayTokens(token, receiver, amount)` `:113-117`, emit.

### 5.5 OmniBridgeFacet

`src/Facets/OmniBridgeFacet.sol` — 106 lines, version 1.0.0. Doc: `docs/OmniBridgeFacet.md`.

**Integrates**: Gnosis OmniBridge, which uses two separate contracts — one for
ERC20s and one that wraps native ETH on the way in.

- **Immutables**: `IOmniBridge private immutable foreignOmniBridge` `:20`, `IOmniBridge private immutable wethOmniBridge` `:24`.
- **No bridge-specific data struct.**

**`startBridgeTokensViaOmniBridge(BridgeData)`** — `:40-56`.
**`swapAndStartBridgeTokensViaOmniBridge(BridgeData, SwapData[])`** — `:61-80`.

**`_startBridge(BridgeData)`** — `:86-105`, private.
- **Native** `:87-90`: `wethOmniBridge.wrapAndRelayTokens{value: minAmount}(receiver)` — the bridge wraps to WETH itself.
- **ERC20** `:91-101`: approve `foreignOmniBridge`, then `relayTokens(token, receiver, amount)`.
- Emits `LiFiTransferStarted`.

### 5.6 MegaETHBridgeFacet

`src/Facets/MegaETHBridgeFacet.sol` — 218 lines, version 1.0.0. Doc: `docs/MegaETHBridgeFacet.md`.

**Integrates**: MegaETH's OP-Stack-style `L1StandardBridge`. Structurally a
copy of `OptimismBridgeFacet` with the rough edges filed off — reading them side
by side is a good exercise in how a codebase improves.

- **Namespace**: `keccak256("com.lifi.facets.megaeth")` `:21`. **Constant**: `bytes internal constant EMPTY_BYTES = ""` `:22`.
- **`struct Storage`** `:26-30`: `bridges` map, `defaultBridge`, `initialized`.
- **`struct Config`** `:32-35`, **`struct MegaETHData`** `:40-44`: `assetIdOnL2`, `l2Gas`, `requiresDepositTo` (the same Synthetix escape hatch, renamed to describe the behaviour rather than the token).
- **Events**: `MegaETHInitialized(Config[])` `:48`, `MegaETHBridgeRegistered(address indexed, address)` `:49`.

**`initMegaETH(Config[] calldata _configs, IL1StandardBridge _defaultBridge)`** — `:56-84`, owner-only.
Same as Optimism's, **plus** an explicit zero-check on `_defaultBridge` `:77-79`
that Optimism lacks.

**`registerMegaETHBridge(address _assetId, address _bridge)`** — `:91-108`, owner-only. Identical logic to Optimism's.

**`startBridgeTokensViaMegaETHBridge(BridgeData, MegaETHData)`** — `:113-130`.
**`swapAndStartBridgeTokensViaMegaETHBridge(BridgeData, SwapData[], MegaETHData)`** — `:136-156`.

**`_startBridge(BridgeData, MegaETHData)`** — `:163-208`, private. Three
improvements over the Optimism version:
1. Re-checks `if (!s.initialized) revert NotInitialized();` `:168` at bridge time.
2. Validates `assetIdOnL2` is non-zero before `depositERC20To`, reverting `InvalidCallData()` `:193-195`.
3. Uses the named `EMPTY_BYTES` constant instead of a literal.

Otherwise identical: resolve bridge with fallback `:169-172`, native →
`depositETHTo` `:175-179`, ERC20 → approve then `depositTo` or `depositERC20To`
`:189-204`, emit.

### 5.7 Rollup facet comparison

| | Arbitrum | Optimism | Polygon | Gnosis | Omni | MegaETH |
|---|---|---|---|---|---|---|
| Bridge-specific struct | yes (3 fields) | yes (3) | none | none | none | yes (3) |
| Native supported | yes | yes | yes | **no** | yes | yes |
| L1 fee paid by caller | yes (`cost`) | no | no | no | no | no |
| Uses `_nativeReserve` overload | **yes** | no | no | no | no | no |
| Per-token bridge registry | no (router lookup) | yes | no | no | 2 fixed | yes |
| Restricted token set | no | no | no | **DAI/USDS** | no | no |
| Restricted destination | no | no | no | **chain 100** | no | no |
| Diamond storage | no | yes | no | no | no | yes |
| Init required | no | **yes** | no | no | no | **yes** |

---

## 6. The Across family

Four facets integrate Across, more than any other bridge. That is because Across
is LI.FI's highest-volume route and each facet trades a different amount of
safety for gas.

**How Across works.** It is an intents/relayer bridge. You *deposit* into a
`SpokePool` on the source chain, declaring the output token, output amount,
destination chain and a fill deadline. A relayer sees the deposit event, fills
the order on the destination chain **from their own capital**, and later claims
reimbursement from Across's shared liquidity pool after an optimistic-oracle
challenge window. The user gets funds in seconds; the settlement risk sits with
the relayer and the Across DAO, not the user.

| Facet | Version | Deployed as | Validation | Gas |
|---|---|---|---|---|
| `AcrossFacet` | 2.0.0 | diamond facet | full | baseline |
| `AcrossFacetV4` | 1.0.0 | diamond facet | full | baseline |
| `AcrossFacetPackedV4` | 1.0.0 | **standalone** | almost none | lowest |
| `AcrossV4SwapFacet` | 1.0.0 | diamond facet | full + EIP-712 | highest |

### 6.1 AcrossFacet (v2, legacy)

`src/Facets/AcrossFacet.sol` — 142 lines, version 2.0.0. Doc: `docs/AcrossFacet.md`.

The original integration, kept for chains still on the v2 SpokePool. Fully
walked through in §2.2 as the canonical example.

- **Immutables**: `IAcrossSpokePool private immutable spokePool` `:23`, `address private immutable wrappedNative` `:27`.
- **`struct AcrossData`** `:35-40`: `int64 relayerFeePct` (18-decimal percentage), `uint32 quoteTimestamp`, `bytes message`, `uint256 maxCount` (front-running protection — the quote is only valid while the pool's deposit count is below this).
- **Functions**: `startBridgeTokensViaAcross` `:57-74`, `swapAndStartBridgeTokensViaAcross` `:80-100`, `_startBridge` `:107-141`.
- Both entry points carry `doesNotContainDestinationCalls`, so the v2 facet cannot attach a destination call despite `AcrossData` having a `message` field.
- `_startBridge` calls `spokePool.deposit(recipient, originToken, amount, destinationChainId, relayerFeePct, quoteTimestamp, message, maxCount)` `:112-137` — addresses are plain `address`, EVM-only.

### 6.2 AcrossFacetV4

`src/Facets/AcrossFacetV4.sol` — 271 lines, version 1.0.0. Doc: `docs/AcrossFacetV4.md`.

The current full-validation integration. Two things changed versus v2: every
address is `bytes32` (so Solana and other non-EVM chains fit), and destination
calls are supported.

- **Inherits**: `ILiFi, ReentrancyGuard, SwapperV2, Validatable, LiFiData`.
- **Immutables**: `IAcrossSpokePoolV4 public immutable SPOKEPOOL` `:29`, `bytes32 public immutable WRAPPED_NATIVE` `:32` (note: `bytes32`, not `address`).
- **Constants**: `ACROSS_CHAIN_ID_SOLANA = 34268394551451` `:35`, `MULTIPLIER_BASE = 1e18` `:38`.
- **Constructor** `:80-89`: reverts `InvalidConfig()` if either argument is zero.

**`struct AcrossV4Data`** `:61-73` — eleven fields, each documented at `:42-60`:

| Field | Type | Meaning |
|---|---|---|
| `receiverAddress` | `bytes32` | destination recipient. For destination calls this is LI.FI's `ReceiverAcrossV4`, not the user. |
| `refundAddress` | `bytes32` | the `depositor` on Across; receives a refund if the fill never happens. |
| `sendingAssetId` | `bytes32` | input token, as bytes32. |
| `receivingAssetId` | `bytes32` | output token on the destination chain. |
| `outputAmount` | `uint256` | exactly what the relayer must deliver. The difference from `inputAmount` is the relayer's fee. |
| `outputAmountMultiplier` | `uint128` | used only in the swap path to rescale `outputAmount` after the swap; see below. |
| `exclusiveRelayer` | `bytes32` | a relayer with first refusal, or zero. |
| `quoteTimestamp` | `uint32` | which Across fee quote this used. |
| `fillDeadline` | `uint32` | destination-chain timestamp after which the order expires and the refund path opens. |
| `exclusivityParameter` | `uint32` | tri-modal, documented at `:52-59`: `0` = no exclusivity; below `MAX_EXCLUSIVITY_PERIOD_SECONDS` = an offset added to `block.timestamp`; otherwise an absolute timestamp. |
| `message` | `bytes` | destination payload. Non-empty means a destination call. |

**`startBridgeTokensViaAcrossV4(BridgeData, AcrossV4Data)`** — `:96-112`, external payable.
Modifiers: `nonReentrant refundExcessNative(msg.sender) validateBridgeData doesNotContainSourceSwaps`. **No `doesNotContainDestinationCalls`** — destination calls are allowed, and the consistency check moved into `_startBridge`.

**`swapAndStartBridgeTokensViaAcrossV4(BridgeData, SwapData[], AcrossV4Data)`** — `:118-150`.
After `_depositAndSwap`, it rescales the output:

```solidity
AcrossV4Data memory modifiedAcrossData = _acrossData;
modifiedAcrossData.outputAmount =
    (_bridgeData.minAmount * _acrossData.outputAmountMultiplier) /
    MULTIPLIER_BASE;
```
`:144-147`. The comment `:137-143` gives the formula the backend must use:
`multiplier = multiplierPercentage * 1e18 * 10^(outputDecimals - inputDecimals)`.
Dividing by `1e18` leaves room to scale in either direction, so 6→18 decimals and
18→6 both work. **The comment also warns explicitly**: "we intentionally do not
verify the outputAmount any further. Only use LI.FI backend-generated calldata to
avoid potential loss of funds." A hand-crafted multiplier can set `outputAmount`
to near zero and hand the difference to the relayer.

**`_startBridge(BridgeData, AcrossV4Data memory)`** — `:157-247`, internal. Note the
parameter is `memory`, not `calldata`, precisely so the swap path can pass its
modified copy.

Validation, in order:
1. `if (_acrossData.message.length > 0 != _bridgeData.hasDestinationCall) revert InformationMismatch();` `:162-164`. The declared flag must match reality.
2. `destinationChainId = _getAcrossChainId(_bridgeData.destinationChainId)` `:167-169`.
3. **Non-EVM branch** `:172-184`: if `_bridgeData.receiver == NON_EVM_ADDRESS` (the sentinel `0x11f111f111f111F111f111f111F111f111f111F1` from `LiFiData.sol:9-10`), require `receiverAddress != 0` else `InvalidNonEVMReceiver()`, and emit `BridgeToNonEVMChainBytes32(transactionId, destinationChainId, receiverAddress)`. Nothing more can be validated — a Solana pubkey is opaque to the EVM.
4. **EVM branch** `:185-199`: unless there is a destination call, require `_convertAddressToBytes32(_bridgeData.receiver) == _acrossData.receiverAddress` else `InvalidReceiver()`. The exemption exists because with a destination call the Across recipient is LI.FI's Receiver contract while `bridgeData.receiver` is the end user. Then require `receiverAddress != 0`.
5. `if (_acrossData.refundAddress == bytes32(0)) revert InvalidCallData();` `:202-204` — an explicit guard against burning the refund path.

Then the deposit. **Native** `:209-222`: `SPOKEPOOL.deposit{value: minAmount}(refundAddress /*depositor*/, receiverAddress /*recipient*/, WRAPPED_NATIVE /*inputToken*/, receivingAssetId /*outputToken*/, minAmount, outputAmount, destinationChainId, exclusiveRelayer, quoteTimestamp, fillDeadline, exclusivityParameter, message)`. **ERC20** `:230-243`: same, after `maxApproveERC20`, with `_acrossData.sendingAssetId` as `inputToken`. Then `emit LiFiTransferStarted` `:246`.

**`_getAcrossChainId(uint256)`** — `:252-261`, internal pure. Maps
`LIFI_CHAIN_ID_SOLANA` (`1151111081099710`) to `ACROSS_CHAIN_ID_SOLANA`
(`34268394551451`); everything else passes through. Solana is currently the only
special case.

**`_convertAddressToBytes32(address)`** — `:265-270`, internal pure.
`bytes32(uint256(uint160(_address)))`. Carries `TODO(EXSC-626)` to move to `LibBytes.toBytes32`.

**Destination side**: `src/Periphery/ReceiverAcrossV4.sol` receives the fill,
authenticates the SpokePool, and hands the tokens to `Executor`. See the
periphery reference.

### 6.3 AcrossFacetPackedV4

`src/Facets/AcrossFacetPackedV4.sol` — 463 lines, version 1.0.0. Doc: `docs/AcrossFacetPackedV4.md`.

The gas-optimised twin. **It is not a diamond facet in the usual sense** — it
inherits `TransferrableOwnership` and holds its own token approvals, and the
comment at `:82` says `setApprovalForBridge` is "only meant to be called outside
of the context of the diamond". It is deployed standalone and called directly.

The trick: ABI encoding wastes bytes on 32-byte word alignment and dynamic
offsets. This facet reads its arguments **straight out of `msg.data` at fixed
byte offsets**, so a `uint32` costs 4 bytes instead of 32. On an L2 where
calldata is the dominant cost, that is a large saving.

- **Immutables**: `IAcrossSpokePoolV4 public immutable SPOKEPOOL` `:24`, `bytes32 public immutable WRAPPED_NATIVE` `:27`.
- **Events**: `LiFiAcrossTransfer(bytes8 _transactionId)` `:31` — note `bytes8`, a truncated transaction id, and **not** the standard `LiFiTransferStarted`. Indexers must handle both. `CallExecutedAndFundsWithdrawn()` `:32`.
- **Errors**: `WithdrawFailed()` `:36`, `InvalidInputAmount()` `:38`, `InvalidCalldataLength()` `:39`.
- **`struct PackedParameters`** `:43-55`: bundles the eleven parameters. The comment `:41-42` explains it exists purely to dodge stack-too-deep.
- **Constructor** `:63-78`: reverts `InvalidConfig()` if spoke pool, wrapped native or owner is zero.

**`setApprovalForBridge(address[] calldata tokensToApprove)`** — `:85-96`, external, `onlyOwner`.
Pre-approves `type(uint256).max` of each token to the SpokePool `:90-94`. Because
this contract holds standing approvals, the bridging functions skip the approve
step entirely — that is part of the gas saving.

#### The native packed calldata layout

`startBridgeTokensViaAcrossV4NativePacked()` — `:119-137`, external payable, **no arguments in the signature**. Layout documented at `:99-111` and read at `:121-133`:

| Bytes | Field | Type | Read at |
|---|---|---|---|
| `[0:4]` | function selector | `bytes4` | — |
| `[4:12]` | `transactionId` | `bytes8` | `:136` |
| `[12:44]` | `depositor` (refund address) | `bytes32` | `:122` |
| `[44:76]` | `receiver` | `bytes32` | `:123` |
| `[76:108]` | `receivingAssetId` | `bytes32` | `:125` |
| `[108:140]` | `outputAmount` | `uint256` | `:127` |
| `[140:148]` | `destinationChainId` | `uint64` | `:128` |
| `[148:180]` | `exclusiveRelayer` | `bytes32` | `:129` |
| `[180:184]` | `quoteTimestamp` | `uint32` | `:130` |
| `[184:188]` | `fillDeadline` | `uint32` | `:131` |
| `[188:192]` | `exclusivityParameter` | `uint32` | `:132` |
| `[192:]` | `message` | `bytes` | `:133` |

Two values are **not** in calldata at all: `inputToken` is hardwired to
`WRAPPED_NATIVE` `:124` and `inputAmount` is hardwired to `msg.value` `:126`. The
comment at `:116-118` calls this out explicitly.

#### The ERC20 packed calldata layout

`startBridgeTokensViaAcrossV4ERC20Packed()` — `:187-216`, external. Layout documented at `:168-182`:

| Bytes | Field | Type |
|---|---|---|
| `[0:4]` | function selector | `bytes4` |
| `[4:12]` | `transactionId` | `bytes8` |
| `[12:44]` | `depositor` | `bytes32` |
| `[44:76]` | `receiver` | `bytes32` |
| `[76:108]` | `sendingAssetId` | `bytes32` |
| `[108:140]` | `receivingAssetId` | `bytes32` |
| `[140:156]` | `inputAmount` | **`uint128`** (16 bytes) |
| `[156:188]` | `outputAmount` | `uint256` |
| `[188:196]` | `destinationChainId` | `uint64` |
| `[196:228]` | `exclusiveRelayer` | `bytes32` |
| `[228:232]` | `quoteTimestamp` | `uint32` |
| `[232:236]` | `fillDeadline` | `uint32` |
| `[236:240]` | `exclusivityParameter` | `uint32` |
| `[240:]` | `message` | `bytes` |

`inputAmount` is deliberately `uint128` — 16 bytes saved, and no real token
supply approaches `2^128`. The body pulls tokens with
`LibAsset.transferFromERC20(address(uint160(uint256(sendingAssetId))), msg.sender, address(this), inputAmount)` `:192-198`,
then deposits.

**`startBridgeTokensViaAcrossV4NativeMin(PackedParameters calldata)`** — `:145-165`, external payable.
**`startBridgeTokensViaAcrossV4ERC20Min(PackedParameters calldata, bytes32 sendingAssetId, uint256 inputAmount)`** — `:226-257`, external.
The "Min" variants use normal ABI encoding but keep the no-validation, no-approve
design. They exist for callers who cannot easily produce packed calldata.

**`encode_startBridgeTokensViaAcrossV4NativePacked(PackedParameters calldata)`** — `:261-281`, external pure. Builds the packed bytes with `bytes.concat` in the exact order above.

**`encode_startBridgeTokensViaAcrossV4ERC20Packed(PackedParameters calldata, bytes32 sendingAssetId, uint256 inputAmount)`** — `:302-338`, external pure.
Reverts `InvalidInputAmount()` if `inputAmount > type(uint128).max` `:307-309` —
the one real validation in the whole contract. Splits the concatenation into
`part1`/`part2`/`part3` `:312-334` to avoid stack-too-deep, then joins them with
the message.

**`decode_startBridgeTokensViaAcrossV4NativePacked(bytes calldata data)`** — `:342-384`, external pure.
Reverts `InvalidCalldataLength()` if `data.length < 192` `:353-355`. Reconstructs
both `BridgeData` and `AcrossFacetV4.AcrossV4Data` so tooling can verify packed
calldata. The **EVM-vs-non-EVM heuristic** at `:361-368`: if the first 12 bytes of
the receiver field are zero it is treated as an EVM address, otherwise
`bridgeData.receiver` is set to `NON_EVM_ADDRESS`. `acrossData.sendingAssetId` is
set to `bytes32(0)` `:374` since native has no input token in calldata.

**`decode_startBridgeTokensViaAcrossV4ERC20Packed(bytes calldata data)`** — `:388-433`, external pure.
Minimum length 240 `:399-401`. Same heuristic `:410-417`. Populates
`bridgeData.sendingAssetId` `:405-407` and `minAmount` `:418`.

**`executeCallAndWithdraw(address _callTo, bytes calldata _callData, address _assetAddress, address _to, uint256 _amount)`** — `:441-462`, external, `onlyOwner`.
Arbitrary call then sweep, mirroring `WithdrawFacet`. Necessary because this
contract holds its own balances and approvals outside the Diamond. Emits
`CallExecutedAndFundsWithdrawn()` or reverts `WithdrawFailed()`.

**The security trade-off, stated plainly.** The doc comments at `:112-115`,
`:141-144`, `:183-186` all repeat: *"This packed implementation prioritizes gas
optimization over runtime validation. The depositor parameter (refund address) is
not validated to be non-zero. Callers must ensure valid parameters to avoid
potential loss of funds. For full validation, use the non-packed AcrossFacetV4
implementation."* There is no `nonReentrant`, no `validateBridgeData`, no
receiver/refund consistency check, and no `hasDestinationCall` cross-check. This
is safe only because the calldata comes from LI.FI's backend.

### 6.4 AcrossV4SwapFacet

`src/Facets/AcrossV4SwapFacet.sol` — 838 lines, version 1.0.0, the largest facet in the repo. Doc: `docs/AcrossV4SwapFacet.md`.

Integrates Across's `SpokePoolPeriphery`, which can perform a swap on the source
chain *inside* the Across deposit, and also supports "sponsored" deposits routed
through CCTP or LayerZero OFT. This facet therefore has five different bridge
paths rather than one.

- **Immutable**: `ISpokePoolPeriphery public immutable SPOKE_POOL_PERIPHERY` `:43`.
- **Constants**: `ACROSS_CHAIN_ID_SOLANA = 34268394551451` `:58`, `MULTIPLIER_BASE = 1e18` `:61`, `ACROSS_V4_SWAP_PAYLOAD_TYPEHASH` `:65`, `EIP712_DOMAIN_TYPEHASH` `:69`, `NAME_HASH` `:73`, `VERSION_HASH = keccak256(bytes("1"))` `:75`.
- **`struct AcrossV4SwapFacetData`** `:94` — the union of parameters for all five paths.

**Entry points**: `startBridgeTokensViaAcrossV4Swap` `:140-172`, `swapAndStartBridgeTokensViaAcrossV4Swap` `:173-226`.

**`_startBridge`** — `:227-273`, internal. Dispatches to one of five private paths:

| Path | Line | What it does |
|---|---|---|
| `_callSpokePoolDeposit` | `:317-390` | plain V4 deposit through the periphery |
| `_callSpokePoolPeripherySwapAndBridge` | `:391-485` | source-chain swap executed by Across's periphery, then deposit |
| `_callSponsoredOftDeposit` | `:486-564` | LayerZero OFT sponsored transfer |
| `_callSponsoredCctpDepositForBurn` | `:565-640` | Circle CCTP sponsored burn |
| `_depositToSpokePool` | `:641-663` | the shared low-level deposit |

**Supporting internals**:
- `_verifySignatureIfRequired` `:274-316` — EIP-712 check, only when the chosen path needs a LI.FI attestation.
- `_toAcrossChainId(uint256)` `:664-678` — same Solana mapping as V4.
- `_validateReceiverAndEmitNonEvmEvent(...)` `:679-706` — the non-EVM sentinel handling, factored out.
- `_validateAmount(...)` `:707-718`, `_validateDestinationChainId(...)` `:719-737`.
- `_chainIdToCctpDomainId(uint256)` `:738-771` — EVM chain id to Circle domain (a `pure` switch, so adding a chain needs a redeploy).
- `_chainIdToLzEid(uint256)` `:772-816` — EVM chain id to LayerZero endpoint id, likewise `pure`.
- `_domainSeparator()` `:817-831`, `_convertAddressToBytes32(address)` `:832-838`.

**When the router picks each Across facet**: `AcrossFacetPackedV4` for a plain
token bridge on an L2 where calldata dominates gas; `AcrossFacetV4` for anything
needing validation or a destination call; `AcrossV4SwapFacet` when Across itself
should perform the source swap or when the route is sponsored; `AcrossFacet` only
on chains still running the v2 SpokePool.

---

## 7. Messaging and liquidity-network bridges

Eight facets that route through a general-purpose messaging layer (LayerZero,
Axelar, Wormhole) or a liquidity network with its own pools.

### 7.1 StargateFacetV2

`src/Facets/StargateFacetV2.sol` — 165 lines, version 1.0.1. Doc: `docs/StargateFacetV2.md`.

**Integrates**: Stargate V2, which is LayerZero's OFT (Omnichain Fungible Token)
standard plus unified liquidity pools. **Trust**: LayerZero's DVN set plus
Stargate's pools. Supports destination calls via `composeMsg`.

- **`using SafeTransferLib for address`** `:19` — Solady, not OpenZeppelin.
- **Immutable**: `ITokenMessaging public immutable tokenMessaging` `:23`. Note the facet does not store pool addresses; it *looks them up* per asset.
- **`struct StargateData`** `:30-35`: `uint16 assetId` (Stargate's own token id), `IStargate.SendParam sendParams` (destination endpoint, recipient as bytes32, amounts, `composeMsg`, `oftCmd`), `IStargate.MessagingFee fee` (`nativeFee` + `lzTokenFee`), `address payable refundAddress`.
- **Error**: `InvalidAssetId(uint16 invalidAssetId)` `:38`.

**`startBridgeTokensViaStargate(BridgeData calldata, StargateData calldata)`** — `:51-67`. Note `BridgeData` is **`calldata`** here, unusual among the facets — possible because this path never mutates it.

**`swapAndStartBridgeTokensViaStargate(BridgeData memory, SwapData[], StargateData)`** — `:73-94`. Uses the `_nativeReserve` overload with `_stargateData.fee.nativeFee` `:85-91`, so the LayerZero fee survives the swap-leftover sweep.

**`_startBridge(BridgeData memory, StargateData memory)`** — `:101-164`, private.
1. **Two-part consistency check** `:106-111`: `composeMsg.length > 0` must equal `hasDestinationCall`, **and** a destination call must not also set `oftCmd` (the two are mutually exclusive in Stargate's ABI). Either violation reverts `InformationMismatch()`.
2. **Receiver match** `:115-119`: without a destination call, `_bridgeData.receiver` must equal `address(uint160(uint256(sendParams.to)))`, else `InformationMismatch()`.
3. **Pool lookup** `:122-126`: `routerAddress = tokenMessaging.stargateImpls(assetId)`; reverts `InvalidAssetId(assetId)` if zero. This indirection means new Stargate pools work without redeploying the facet.
4. **Approval, done by hand** `:136-150` rather than via `LibAsset.maxApproveERC20`: read the current allowance, and if insufficient, zero it first when non-zero (`safeApprove(routerAddress, 0)` `:146`) before setting `type(uint256).max`. This is the USDT dance written out explicitly.
5. `_stargateData.sendParams.amountLD = _bridgeData.minAmount` `:154` — overwrite the amount with the realized post-swap value.
6. `IStargate(routerAddress).sendToken{value: msgValue}(sendParams, fee, refundAddress)` `:157-161`, where `msgValue = fee.nativeFee` plus `minAmount` when bridging native `:129-133`.
7. `emit LiFiTransferStarted` `:163`.

**Destination side**: `src/Periphery/ReceiverStargateV2.sol` implements
`lzCompose`, which LayerZero calls with the `composeMsg`; it forwards to
`Executor`. This is the only bridge `CalldataVerificationFacet.validateDestinationCalldata`
can verify (§3.9).

### 7.2 AllBridgeFacet

`src/Facets/AllBridgeFacet.sol` — 322 lines, version 2.2.0. Doc: `docs/AllBridgeFacet.md`.

**Integrates**: Allbridge Core, a stablecoin liquidity network with its own chain
numbering and a choice of messenger protocol.

- **Namespace**: `keccak256("com.lifi.facets.allbridge")` `:33-34`. **Immutable**: `IAllBridge private immutable ALLBRIDGE` `:40`. **Error**: `UnsupportedAllBridgeChainId()` `:36`.
- **`struct AllBridgeData`** `:49-56`: `bytes32 recipient`, `uint256 fees`, `bytes32 receiveToken`, `uint256 nonce`, `IAllBridge.MessengerProtocol messenger` (an enum — Allbridge lets you pick the attestation layer), `bool payFeeWithSendingAsset`.
- **`struct ChainIdConfig`** `:61-64`, **`struct Storage`** `:66-71`: `mapping(uint256 => uint256) allBridgeChainIds` plus `bool chainMappingsInitialized`. The comment `:67-68` notes Allbridge chain ids start at 1, so a stored `0` unambiguously means "unmapped" and no offset trick is needed.
- **Events**: `AllBridgeChainMappingsInitialized(ChainIdConfig[])` `:75`, `ChainIdToAllBridgeChainIdSet(uint256 indexed, uint256)` `:77-81`, `ChainIdToAllBridgeChainIdUnset(uint256 indexed)` `:82`.

**Chain-mapping admin** (all owner-only):
- `initAllBridge(ChainIdConfig[] calldata)` `:98-119` — reverts `InvalidConfig()` on an empty array `:99` or any zero id `:108`. Re-initialisation overwrites the given entries and leaves others alone `:94`.
- `setChainIdToAllBridgeChainId(ChainIdConfig[] calldata)` `:125-149` — requires `chainMappingsInitialized` else `NotInitialized()` `:132-134`, same zero checks, emits per entry.
- `unsetChainIdToAllBridgeChainId(uint256 _chainId)` `:153-163` — `delete`s the entry, emits.
- `getChainIdToAllBridgeChainId(uint256)` `:168-172`, external view.

**`startBridgeTokensViaAllBridge`** `:176-193`, **`swapAndStartBridgeTokensViaAllBridge`** `:199-219`. Both carry `doesNotContainDestinationCalls`.

**`_startBridge(BridgeData memory, AllBridgeData calldata)`** — `:224-296`, internal.
- The comment at `:228-229` is candid: "we do not validate `_allBridgeData.fees` here due to gas optimization reasons; our backend ensures that the fees are correct."
- `_getAllBridgeChainId` `:232-234`.
- Non-EVM branch `:237-248` / EVM receiver match `:249-257`, the standard pattern.
- `maxApproveERC20(sendingAssetId, ALLBRIDGE, minAmount)` `:260-264`.
- **Two fee modes** `:267-293`: `payFeeWithSendingAsset` calls `ALLBRIDGE.swapAndBridge(...)` with `fees` as the last argument and no `msg.value`; otherwise it calls the same function with `{value: fees}` and passes `0` as the fee argument. Same call, fee denominated differently.
- Emits `LiFiTransferStarted`.

**`_getAllBridgeChainId(uint256)`** — `:303-312`, internal view. Reverts `UnsupportedAllBridgeChainId()` when the mapping is empty — unlike Across's pure passthrough, an unmapped chain fails loudly.

**`getStorage()`** — `:315-321`, private pure.

### 7.3 GlacisFacet

`src/Facets/GlacisFacet.sol` — 171 lines, version 1.2.0. Doc: `docs/GlacisFacet.md`.

**Integrates**: Glacis Airlift, an aggregator over native token bridges (used for
tokens like USDT and LBTC that have their own canonical cross-chain mechanism).

- **Immutable**: `IGlacisAirlift public immutable AIRLIFT` `:28`; constructor reverts `InvalidConfig()` on zero `:46-51`.
- **`struct GlacisData`** `:36-41`: `bytes32 receiverAddress`, `address refundAddress`, `uint256 nativeFee`, `bytes32 outputToken`. The comment `:35` notes `bytes32(0)` for `outputToken` selects Glacis's default routing.

**`startBridgeTokensViaGlacis`** `:60-78` — modifiers include **`noNativeAsset`** `:71`; Glacis handles ERC20 only. **`swapAndStartBridgeTokensViaGlacis`** `:84-106` uses the `_nativeReserve` overload with `_glacisData.nativeFee` `:98-104`.

**`_startBridge(BridgeData memory, GlacisData calldata)`** — `:113-170`, internal.
Non-EVM sentinel handling `:118-130`, EVM receiver match `:131-141`, then
`refundAddress != address(0)` else `InvalidCallData()` `:143-145`, then approve
and call:

```solidity
AIRLIFT.send{ value: _glacisData.nativeFee }(
    _bridgeData.sendingAssetId,
    _bridgeData.minAmount,
    _glacisData.receiverAddress,
    _bridgeData.destinationChainId,
    _glacisData.refundAddress,
    _glacisData.outputToken
);
```
`:160-167`. The comment `:156-158` explains the sixth parameter was added in 1.2.0 to enable multibridge routing while staying backwards compatible.

### 7.4 SquidFacet

`src/Facets/SquidFacet.sol` — 217 lines, version 1.0.0. Doc: `docs/SquidFacet.md`.

**Integrates**: Squid Router, built on Axelar. Squid can run its own multicalls
on the source chain, the destination chain, or both — which is why this facet has
a route-type enum.

- **`enum RouteType { BridgeCall, CallBridge, CallBridgeCall }`** `:25-29`.
- **`struct SquidData`** `:42-52`: `routeType`, `string destinationChain` (Axelar uses chain *names*), `string destinationAddress` (a string, to allow non-EVM), `string bridgedTokenSymbol`, `address depositAssetId`, `ISquidMulticall.Call[] sourceCalls`, `bytes payload`, `uint256 fee`, `bool enableExpress`.
- **`struct BridgeContext`** `:55-59`: `{bridgeData, squidData, msgValue}`, introduced purely to defeat stack-too-deep `:54`.
- **Error**: `InvalidRouteType()` `:62`. **Immutable**: `ISquidRouter private immutable squidRouter` `:68`.

**`startBridgeTokensViaSquid`** `:81-98`. Note it deposits **`_squidData.depositAssetId`**, not `_bridgeData.sendingAssetId` `:92-95` — because Squid may swap on the source chain, the token the user provides can differ from the token that gets bridged.

**`swapAndStartBridgeTokensViaSquid`** `:104-126` uses the `_nativeReserve` overload with `_squidData.fee` `:117-123`.

**`_startBridge(BridgeData memory, SquidData calldata)`** — `:133-164`, internal.
Builds the context `:137-141`, approves `depositAssetId` if non-native `:144-150`,
then dispatches on `routeType` `:153-161` with `InvalidRouteType()` as the
fallthrough, and emits.

**`_bridgeCall(BridgeContext memory)`** — `:166-176`, internal. `squidRouter.bridgeCall(bridgedTokenSymbol, minAmount, destinationChain, destinationAddress, payload, receiver, enableExpress)`. Bridge, then call on destination.

**`_callBridge(BridgeContext memory)`** — `:178-189`, private. `squidRouter.callBridge(...)` with `sourceCalls`. Note native is passed as Axelar's sentinel `0xEeee...EEeE` `:181` rather than `address(0)`, and the receiver is hex-stringified with `LibBytes.toHexString(uint160(receiver), 20)` `:187` because Axelar wants a string address.

**`_callBridgeCall(BridgeContext memory)`** — `:191-205`, private. Both source calls and destination payload.

**`_calculateMsgValue(BridgeData memory, SquidData calldata)`** — `:207-216`, private pure. `fee`, plus `minAmount` when `sendingAssetId` is native.

**Gotcha**: `_calculateMsgValue` keys off `_bridgeData.sendingAssetId` while the
approval branch keys off `_squidData.depositAssetId`. If a caller sets these
inconsistently the value forwarded and the token approved disagree.

### 7.5 SymbiosisFacet

`src/Facets/SymbiosisFacet.sol` — 504 lines, version 2.0.0. Doc: `docs/SymbiosisFacet.md`.

**Integrates**: Symbiosis MetaRouter (cross-chain swaps through Symbiosis's own
pools) **and**, in 2.0.0, an `OnchainSwapV3` router for the syBTC → Bitcoin path.

- **Namespace**: `keccak256("com.lifi.facets.symbiosis")` `:50`. **Typehash**: `SYMBIOSIS_PAYLOAD_TYPEHASH` `:54`.
- **`struct Storage`** `:76`. **Errors**: `OnchainSwapV3NotSupported()` `:84`, `InvalidSignature()` `:86`, `SignatureExpired()` `:88`, `TransactionAlreadyProcessed()` `:90`, `OnchainSwapV3FeeMismatch()` `:92`.
- **`struct SymbiosisData`** `:113-130` — fifteen fields, documented at `:96-112`. The first nine describe the MetaRouter path (two swap calldatas, two DEX routers, `approvedTokens`, `callTo`, `callData`); the last six describe the OnchainSwapV3 path (`viaOnchainSwapV3`, `dex`, `dexgateway`, `onchainSwapData`, `fee`, `deadline`, `signature`).

**Constructor** `:140-170` takes five addresses. Two validations worth noting:
- MetaRouter, gateway and backend signer must all be non-zero `:147-151`.
- The OnchainSwapV3 router and its gateway must be **both** set or **both** zero `:156-159`, because "a router with a zero gateway would approve address(0) for ERC20 inputs, silently breaking the route".
- They are deliberately allowed to be zero `:161-164` so that one facet source deploys to every chain; chains without the syBTC path simply revert `OnchainSwapV3NotSupported`.

**`startBridgeTokensViaSymbiosis`** `:177-208`, **`swapAndStartBridgeTokensViaSymbiosis`** `:209-281`.

**`_startBridge`** — `:282-291`, internal. A two-way dispatch on `viaOnchainSwapV3`.

**`_startBridgeViaMetaRouter`** — `:296-337`, private. The important check is first:

```solidity
if (
    _symbiosisData.approvedTokens.length == 0 ||
    _symbiosisData.approvedTokens[0] != _bridgeData.sendingAssetId
) revert InformationMismatch();
```
`:304-307`. The comment `:300-303` explains why: the MetaRouter pulls
`approvedTokens[0]` from the Diamond using a standing gateway allowance, so
without this pin a caller could redirect the pull to **any other token the
Diamond holds a residual balance of**. This is one of the sharpest lessons in the
codebase about what "max approval" costs you.

Then: native amount or `maxApproveERC20` to `symbiosisGateway` `:309-320`, and
`symbiosisMetaRouter.metaRoute{value: nativeAssetAmount}(MetaRouteTransaction(...))` `:322-334`.

**`_startBridgeViaOnchainSwapV3`** — `:349-421`, private. The doc comment `:339-346`
is a good summary of the threat model: `dex`, `dexgateway` and `onchainSwapData`
are caller-supplied and forwarded into a trusted router, so they are gated by an
EIP-712 backend signature, each transaction id is single-use, and a wrong
`viaOnchainSwapV3` flag "can only revert here, never misdirect funds". Replay
protection is written **before** the external call, following checks-effects-interactions `:359-360`.

**`_validateAndVerifyOnchainSwapV3`** — `:422-444`, private. Runs in the entry
point *before* any deposit or swap, so the quote is checked once against the
amount the backend signed. The comment `:353-357` credits this ordering to audit
finding #3.

**`_verifyOnchainSwapV3Signature`** — `:445-480`, private. **`_domainSeparator`** — `:481-496`, private view. **`getStorage`** — `:497-503`.

### 7.6 ThorSwapFacet

`src/Facets/ThorSwapFacet.sol` — 126 lines, version 1.2.1. Doc: `docs/ThorSwapFacet.md`.

**Integrates**: THORChain's router. THORChain is a native-asset cross-chain AMM;
the destination action is encoded in a **memo string**, not calldata.

- **Immutable**: `address private immutable thorchainRouter` `:21`. **Constant**: `DEPRECATED_RUNE = 0x3155BA85D5F96b2d030a4966AF206230e46849cb` `:23-24`. **Error**: `DeprecatedToken()` `:36`.
- **`struct ThorSwapData`** `:30-34`: `address vault` (THORChain's current asgard vault — it rotates, so it is a parameter), `string memo` (THORChain's instruction language, e.g. `SWAP:BTC.BTC:addr:minOut`), `uint256 expiration`.

**`startBridgeTokensViaThorSwap`** `:46-63`, **`swapAndStartBridgeTokensViaThorSwap`** `:69-89`. Both forbid destination calls.

**`_startBridge`** — `:94-125`, internal.
- `if (block.chainid == 1 && sendingAssetId == DEPRECATED_RUNE) revert DeprecatedToken();` `:98-102`. ERC20 RUNE was migrated to native RUNE; bridging the old token would burn it.
- Approve if not native `:107-113`.
- `IThorSwap(thorchainRouter).depositWithExpiry{value: isNative ? minAmount : 0}(vault, sendingAssetId, minAmount, memo, expiration)` `:114-122`.
- Emits `LiFiTransferStarted`.

**Gotcha**: the `vault` address is fully caller-supplied and unvalidated. A wrong
vault sends funds to an address THORChain does not watch. The `expiration`
parameter is the only on-chain protection, and it only limits *when*, not *where*.

### 7.7 FraxFacet

`src/Facets/FraxFacet.sol` — 564 lines, version 1.0.0. Doc: `docs/FraxFacet.md`.

**Integrates**: Frax HopV2 (hub on Fraxtal, spoke elsewhere) over LayerZero OFT.
Its distinguishing feature is handling **Tempo**, a chain whose LayerZero
`EndpointV2Alt` refuses native `msg.value` and charges the messaging fee in an
ERC20 gas token instead.

- **Namespace**: `keccak256("com.lifi.facets.frax")` `:42`.
- **Immutables**: `IFraxHopV2 public immutable FRAX_HOP` `:48` (also the approval target), `address public immutable FRAX_TIP_FEE_MANAGER` `:55`, `address public immutable FRAX_PATH_USD` `:59`. The comment `:50-54` explains the design: `FRAX_TIP_FEE_MANAGER` is non-zero **only on Tempo**, and that single immutable is what lets one source file serve every chain.
- **`struct ChainIdConfig`** `:66-69`, **`struct Storage`** `:73-76` (`mapping(uint256 => uint32) lzEids` + `chainMappingsInitialized`; the comment `:71-72` notes EID 0 is safe as "unset" because LayerZero v1 starts at 101 and v2 at 30000).
- **`struct FraxData`** `:91-97`: `address oft`, `uint32 dstEid`, `uint256 nativeFee`, `address refundRecipient`, `bytes32 nonEVMReceiver`. The comment `:87-90` states the invariant crisply: `nonEVMReceiver` is required when the receiver is the sentinel and **must be `bytes32(0)` for EVM destinations**, "so the two receiver fields can never disagree about where the funds go".
- **Events**: `FraxChainMappingsInitialized` `:102`, `FraxChainIdToEidSet(uint256 indexed, uint32)` `:105`, `FraxChainIdToEidUnset(uint256 indexed)` `:108`.

**Admin**: `initFrax` `:136-160`, `setFraxChainIdToEid` `:162-188`, `unsetFraxChainIdToEid(uint256[] calldata)` `:190-208` (note: batch, unlike AllBridge's single), `getFraxChainIdToEid` `:210-220`.

**`startBridgeTokensViaFrax`** `:222-262`, **`swapAndStartBridgeTokensViaFrax`** `:264-328`.

**`_validateFraxData`** — `:330-394`, internal. Enforces `oft.token() == sendingAssetId` and `FRAX_HOP.approvedOft(oft)` before any deposit or swap `:409-410`, plus the EID cross-check and the receiver-sentinel binding.

**`_isNonEVMDestination`** — `:396-403`, internal.

**`_startBridge`** — `:405-477`, internal. The interesting part is dust:
1. `flooredAmount = FRAX_HOP.removeDust(oft, minAmount)` `:415-418`; reverts `InvalidCallData()` if it floors to zero `:419-421`. OFT transfers quantise to a "shared decimals" granularity, and HopV2 only pulls the floored amount.
2. Approve exactly `flooredAmount` `:423-427`.
3. Build `recipient` from either `nonEVMReceiver` or `LibBytes.toBytes32(receiver)` `:432-435`, emitting `BridgeToNonEVMChainBytes32` when applicable `:437-443`.
4. **Fee branch** `:445-461`: `FRAX_TIP_FEE_MANAGER == address(0)` → `FRAX_HOP.sendOFT{value: nativeFee}(oft, dstEid, recipient, flooredAmount, 0, "")`; otherwise `_sendViaTempo(...)`.
5. **Return the dust** `:464-471`: `dust = minAmount - flooredAmount`, transferred to `refundRecipient`. The comment `:463` is emphatic: "never leave it in the diamond".
6. `_bridgeData.minAmount = flooredAmount` `:474` **before** emitting, so downstream accounting matches what actually arrives.

**`_sendViaTempo(FraxData calldata, address, bytes32, uint256)`** — `:489-556`, internal.
Documented at `:479-484`: resolves the fee token as
`FRAX_TIP_FEE_MANAGER.userTokens(diamond)` falling back to `FRAX_PATH_USD` `:495-500`,
pulls `nativeFee` of *that ERC20* from the caller, approves HopV2, calls `sendOFT`
with `msg.value == 0`, and sweeps any unpulled remainder to `refundRecipient`. The
fee token must differ from the bridged asset or it reverts `InformationMismatch`.

**`_getStorage`** — `:557-563`, private pure.

### 7.8 SupersetFacet

`src/Facets/SupersetFacet.sol` — 537 lines, version 1.1.0. Doc: `docs/SupersetFacet.md`.

**Integrates**: Superset OFT over LayerZero.

- **Namespace**: `keccak256("com.lifi.facets.superset")` `:62`.
- **Modifier**: `validateBridgeDataSuperset(BridgeData memory)` `:40-52` — a facet-local replacement for the standard `validateBridgeData`.
- **Constants**: `ARBITRUM_CHAIN_ID = 42161` `:53`, `OMNI_TOKEN_ID_BYTES = 32` `:56`, `FEE_BYTES = 3` `:59` — the last two describe a packed byte layout used by `_omniPathToLocalAddressPath`.
- **`struct ChainIdConfig`** `:80-83`, **`struct Storage`** `:87`, **`struct SupersetData`** `:124`.
- **Error**: `InsufficientNativeValue()` `:143`. **Events**: `SupersetChainMappingsInitialized` `:148`, `ChainIdToEidSet(uint256 indexed, uint32)` `:151`.

**Admin**: `initSuperset` `:170-196`, `setChainIdToEid` `:198-219`, `getChainIdToEid` `:221-231`.

**Entry points**: `startBridgeTokensViaSuperset` `:233-262`, `swapAndStartBridgeTokensViaSuperset` `:264-324`.

**`_validateSupersetData`** — `:326-413`, internal. **`_startBridge`** — `:415-495`, internal.

**`_omniPathToLocalAddressPath(...)`** — `:497-528`, internal. Decodes Superset's
"omni path" encoding — a packed sequence of 32-byte token ids and 3-byte fees —
into local addresses, using the two byte-width constants above. This is the same
kind of hand-rolled packing as `AcrossFacetPackedV4`, applied to a route
description rather than a function call.

**`_getStorage`** — `:530-536`, private pure.

### 7.9 Comparison

| | Stargate V2 | AllBridge | Glacis | Squid | Symbiosis | ThorSwap | Frax | Superset |
|---|---|---|---|---|---|---|---|---|
| Underlying layer | LayerZero | own + choice of messenger | native bridges | Axelar | own pools | THORChain | LayerZero OFT | LayerZero OFT |
| Destination call | **yes** (`composeMsg`) | no | no | **yes** (`payload`) | via `callData` | via memo | no | — |
| Non-EVM support | via bytes32 | **yes** | **yes** | via strings | **yes** (BTC) | **yes** | **yes** | — |
| Chain-id mapping | LayerZero EIDs in `sendParams` | diamond storage | passthrough | chain *names* | — | — | diamond storage | diamond storage |
| Fee paid in | native | native **or** sending asset | native | native | native | native | native **or** TIP20 | native |
| Diamond storage | no | yes | no | no | yes | no | yes | yes |
| EIP-712 gate | no | no | no | no | **yes** (V3 path only) | no | no | no |

---

## 8. Circle CCTP and burn/mint facets

Three facets whose bridge does not lock-and-mint or use a liquidity pool. Circle's
Cross-Chain Transfer Protocol **burns** USDC on the source chain and **mints**
native USDC on the destination after Circle's attestation service signs the burn.
There is no wrapped asset and no LP; the trust assumption is Circle itself.

### 8.1 CelerCircleBridgeFacet

`src/Facets/CelerCircleBridgeFacet.sol` — 137 lines, version 2.0.0. Doc: `docs/CelerCircleBridgeFacet.md`.

**Integrates**: Circle CCTP through Celer's `CircleBridgeProxy`, which adds a
relayer that completes the mint so the user does not have to submit a second
transaction.

- **Immutables**: `ICircleBridgeProxy public immutable CIRCLE_BRIDGE_PROXY` `:27`, `address public immutable USDC` `:29`. Constructor reverts `InvalidConfig()` on either zero `:45-51`.
- **`struct CelerCircleData`** `:35-38`: `uint256 maxFee` (max fee payable on the destination domain in burn-token units; `0` means no limit), `uint32 minFinalityThreshold` (documented at `:34`: `1000` = fast path, `2000` = standard).

**`initCelerCircleBridge()`** — `:56-62`, external, owner-only `:57`.
Sets `IERC20(USDC).approve(CIRCLE_BRIDGE_PROXY, type(uint256).max)` `:61`. Two
comments justify the design: `:54` argues a max approval is safe "since the
diamond is designed to not hold any funds", and `:55` notes there is no
initialization flag because re-running is harmless. `:60` explains plain `approve`
is fine here because the only token is USDC, which is standards-compliant.

**`startBridgeTokensViaCelerCircleBridge(BridgeData calldata, CelerCircleData calldata)`** — `:69-82`.
**Not payable** `:73-74`. Modifiers include `onlyAllowSourceToken(_bridgeData, USDC)` `:78`. Deposits `USDC` explicitly rather than `_bridgeData.sendingAssetId` `:80`.

**`swapAndStartBridgeTokensViaCelerCircleBridge(BridgeData memory, SwapData[], CelerCircleData)`** — `:88-109`. Payable, same token restriction.

**`_startBridge(BridgeData memory, CelerCircleData calldata)`** — `:116-136`, private.
- `if (_bridgeData.destinationChainId > type(uint64).max) revert InvalidCallData();` `:120-122` — the proxy takes a `uint64` chain id, so an out-of-range value must fail rather than silently truncate.
- `CIRCLE_BRIDGE_PROXY.depositForBurn(minAmount, uint64(destinationChainId), bytes32(uint256(uint160(receiver))), USDC, maxFee, minFinalityThreshold)` `:125-133`.
- Emits `LiFiTransferStarted`.

Note this facet passes a **chain id**, not a Circle domain id — the Celer proxy
does the mapping. `PolymerCCTPFacet` talks to Circle directly and therefore has to
maintain the domain mapping itself.

### 8.2 PolymerCCTPFacet

`src/Facets/PolymerCCTPFacet.sol` — 574 lines, version 3.1.0. Doc: `docs/PolymerCCTPFacet.md`.

**Integrates**: Circle's `TokenMessenger` (CCTP v2) directly, with Polymer as the
off-chain relayer. This is the most carefully commented facet in the repo and
worth reading in full — it is a masterclass in the edge cases of moving a token
to chains with different address formats.

- **Constants**:
  - `UNRESTRICTED_DESTINATION_CALLER = bytes32(0)` `:58` — anyone may complete the mint.
  - `HYPERCORE_CCTP_FORWARDER` `:65-66` — Circle's `CctpForwarder` on HyperEVM, pre-encoded as bytes32. The comment `:60-64` explains HyperCore flows mint to this contract, which alone may execute the message and then deposits into HyperCore for the encoded receiver.
  - `STELLAR_CCTP_FORWARDER` `:78-79` — a raw 32-byte Stellar contract id. The comment `:68-77` explains why this exists at all: **a Stellar account can never be a CCTP mintRecipient**, because CCTP stores only the raw 32 bytes without the strkey type prefix, so the protocol assumes the recipient is a contract and USDC minted to a bare account is unrecoverable. Rotating the forwarder requires a facet upgrade.
  - `NAMESPACE = keccak256("com.lifi.facets.polymercctp")` `:81-82`.
- **Immutables**: `ITokenMessenger public immutable TOKEN_MESSENGER` `:85`, `address public immutable USDC` `:87`, `address payable public immutable POLYMER_FEE_RECEIVER` `:89`. Constructor reverts `InvalidConfig()` if any is zero `:172-188`.
- **`struct PolymerCCTPData`** `:91-116`, each field documented inline:

| Field | Purpose |
|---|---|
| `polymerTokenFee` | USDC fee taken by the facet, sent to `POLYMER_FEE_RECEIVER`. May be zero. |
| `maxCCTPFee` | Circle's max destination fee, in burn-token units. |
| `nonEVMReceiver` | The recipient emitted in `BridgeToNonEVMChainBytes32`. For Solana this is the **owner wallet, not a token account** — the relayer derives the ATA from it. |
| `solanaReceiverATA` | For Solana only: the receiver's USDC Associated Token Account, used as the CCTP `mintRecipient`. |
| `minFinalityThreshold` | 1000 fast / 2000 standard. |
| `refundRecipient` | Consumed only by the swap entry point; `msg.sender` may be a relayer or the Permit2Proxy, so refunds need an explicit address. |
| `hookData` | `CctpForwarder` hook payload. Required for HyperCore and Stellar; **must be empty otherwise**. |

- **`struct ChainIdConfig`** `:118-121`, **`struct Storage`** `:123-127`. Note the comment at `:124`: the mapping stores `domainId + 1` so that **domain 0 (Ethereum) stays distinguishable from an unset entry** — the opposite trick from AllBridge, which could rely on ids starting at 1.
- **Events**: `PolymerCCTPFeeSent(uint256 bridgeAmount, uint256 polymerFee, uint32 minFinalityThreshold)` `:136-140` (the comment `:132` says Polymer's off-chain component keys off this), `PolymerCCTPChainMappingsInitialized` `:142`, `ChainIdToDomainIdSet(uint256 indexed, uint32)` `:144`, `ChainIdToDomainIdUnset(uint256 indexed)` `:146`.
- **Modifier `validatePolymerData`** `:151-164`: a slimmed-down `validateBridgeData` that checks only `minAmount != 0` and `destinationChainId != block.chainid`, skipping the receiver check because `_startBridge` does a far more specific one. The comment `:155` acknowledges this bends checks-effects-interactions and argues it is acceptable since only USDC is ever touched.

**Admin**: `initPolymerCCTP(ChainIdConfig[])` `:194-219` (owner-only; also sets the max USDC approval `:200`), `setChainIdToDomainId` `:220-245`, `unsetChainIdToDomainId(uint256)` `:246-260`, `getChainIdToDomainId(uint256)` `:261-270`.

**Entry points**: `startBridgeTokensViaPolymerCCTP` `:271-296`, `swapAndStartBridgeTokensViaPolymerCCTP` `:297-341`.

**`_startBridge(BridgeData memory, PolymerCCTPData calldata)`** — `:342-556`, internal.
This is a **corridor dispatch**: three mutually exclusive arms based on the
destination, each with its own receiver rules.

**Arm 1 — HyperCore** `:349-369`. Requires a real EVM receiver (not the sentinel)
and `hookData.length >= 52`. The hook layout is `magic (24) + version (4) +
payload length (4)`, then the recipient address at `[32:52]`. The facet checks
`address(bytes20(hookData[32:52])) == _bridgeData.receiver` else
`InvalidReceiver()` `:364-369`, because otherwise "calldata could redirect funds
unnoticed".

**Arm 2 — Stellar** `:370-398`. Requires the sentinel receiver, a non-zero
`nonEVMReceiver`, and `hookData.length > 32`. The comment `:377-379` explains that
`length == 32` would encode a zero-length strkey the forwarder cannot credit,
leaving the USDC "burned but stuck". A second check `:393-398` rejects a hook whose
declared payload length `uint32(bytes4(hookData[28:32]))` disagrees with
`hookData.length - 32`, so a truncated or padded hook cannot reach the forwarder.
The version bytes `[24:28]` are deliberately **not** validated on-chain `:379-382`,
delegating that to Circle's forwarder.

**Arm 3 — everything else** `:399-430`. `hookData` must be empty `:407-409`. Then
the receiver *kind* must match the destination *kind*: the sentinel is valid only
for Solana `:413-416`, and a real EVM address must never target Solana `:427-428`.
The comment `:404-406` spells out both failure modes this prevents — minting to
the low 20 bytes of `nonEVMReceiver` on an EVM chain while events show the
sentinel, or burning a zero-padded EVM address to a non-EVM domain where it is
unclaimable.

Then the burn:
- `LibAsset.transferERC20(USDC, POLYMER_FEE_RECEIVER, polymerTokenFee)` `:432-436`.
- `bridgeAmount = minAmount - polymerTokenFee` `:442-443`. The comment `:438-440` admits the underflow guard is deliberately omitted for gas, relying on backend-generated calldata.
- `domainId = _chainIdToDomainId(destinationChainId)` `:447`.
- **EVM receiver** `:450-481`: HyperCore uses `depositForBurnWithHook(amount, domain, HYPERCORE_CCTP_FORWARDER /*mintRecipient*/, USDC, HYPERCORE_CCTP_FORWARDER /*destinationCaller*/, maxFee, finality, hookData)` `:460-469` — minting to the forwarder and restricting execution to it. Everything else uses plain `depositForBurn(...)` with the receiver as `mintRecipient` and `UNRESTRICTED_DESTINATION_CALLER` `:471-480`. This branch is checked first "for gas ops since it will likely be triggered more often" `:449`.
- **Stellar** `:482-501`: `depositForBurnWithHook` to `STELLAR_CCTP_FORWARDER`, then `emit BridgeToNonEVMChainBytes32`.
- **Solana** `:502-521`: requires `solanaReceiverATA != bytes32(0)` else `InvalidConfig()` `:509-511`, then `depositForBurn` with the **ATA** as `mintRecipient` while the event carries the **owner wallet**. The comment `:504-508` explains the split precisely.

**`_chainIdToDomainId(uint256)`** — `:557-566`, internal view. Reads the mapping and subtracts the `+1` offset.
**`getStorage()`** — `:567-573`, private pure.

### 8.3 PaxosTransitFacet

`src/Facets/PaxosTransitFacet.sol` — 208 lines, version 1.0.0. Doc: `docs/PaxosTransitFacet.md`.

**Integrates**: Paxos Transit Station, which moves Paxos-issued stablecoins
(USDP, PYUSD) cross-chain against a **Paxos-signed quote**. The quote is the
authority here: it fixes the amount, the receiver and the destination asset, and
the facet's job is to make sure the on-chain data agrees with it.

- **Immutable**: `IPaxosTransit public immutable TRANSIT_STATION` `:21`; constructor reverts `InvalidConfig()` on zero `:48-53`.
- **Constant**: `LIFI_DISTRIBUTOR_CODE = 0x4c49464900...00` `:24-25` — the left-adjusted bytes32 encoding of the ASCII string `"LIFI"`. Paxos uses it for volume attribution.
- **`struct PaxosTransitData`** `:37-42`: `IPaxosTransit.Quote quote`, `bytes signature` (Paxos's EIP-712 signature over the quote), `uint256 nativeFee` (forwarded to pay the LayerZero messaging fee), `address refundRecipient`. The comment `:32-36` warns that `refundRecipient` "must accept plain native transfers: a refundRecipient that rejects them reverts the whole bridge (self-inflicted)".

**`startBridgeTokensViaPaxosTransit(BridgeData memory, PaxosTransitData calldata)`** — `:60-95`.
Modifiers include `refundExcessNative(payable(_paxosData.refundRecipient))` `:67` —
note it refunds to the **named recipient**, not `msg.sender`, unlike almost every
other facet. Also `noNativeAsset` `:71`. Three body checks:
1. `refundRecipient != address(0)` else `InvalidCallData()` `:76-78`. The comment `:73-75` explains this is a fail-fast: without it the zero-address transfer would only revert "when LZ fee drift actually leaves an excess — a data-dependent late revert".
2. `_bridgeData.minAmount == _paxosData.quote.offerAmount` else `InformationMismatch()` `:81-83`.
3. `_paxosData.nativeFee <= msg.value` else `InvalidCallData()` `:86-88`, so "the station's LayerZero fee must be paid from msg.value, never from diamond balance".

**`swapAndStartBridgeTokensViaPaxosTransit(BridgeData memory, SwapData[], PaxosTransitData)`** — `:101-166`.
- Same `refundRecipient` zero-check `:118-120`, justified at `:115-117`: `msg.sender` may be a relayer or the Permit2Proxy, so user value must go to an explicit address.
- Same `minAmount == offerAmount` check `:127-129`.
- **Last-swap-asset check** `:136-142`: if there are swaps, `_swapData[last].receivingAssetId` must equal `_bridgeData.sendingAssetId`, else `InformationMismatch()`. The comment `:131-135` gives the reason: `_depositAndSwap` measures the received amount in the last swap's receiving asset, while the slippage floor, the positive-slippage refund and `submitOrder` all act on `sendingAssetId`; a mismatch would apply those checks to the wrong token.
- `nativeFee` is deliberately **not** checked against `msg.value` here `:144-146`, because the fee may be funded by an ERC20→native pre-swap whose output the native reserve keeps in the Diamond.
- Calls `_depositAndSwap` with `offerAmount` as the floor and `nativeFee` as the reserve `:149-155`.
- **Refunds positive slippage before bridging** `:157-163`: `if (receivedAmount > offerAmount)` transfer the excess to `refundRecipient`. Because the quote locks an exact amount, anything above it must not be bridged.

**`_startBridge(BridgeData memory, PaxosTransitData calldata)`** — `:173-207`, internal.
The quote cross-check `:187-193` requires all three:
`sendingAssetId == quote.route.offerAsset`, `receiver == quote.receiver`, and
`quote.distributorCode == LIFI_DISTRIBUTOR_CODE`.
The comment `:182-186` is explicit about what is *not* checked: `quote.route.destEID`
and `quote.route.wantAsset` are **not** cross-checked against
`_bridgeData.destinationChainId`, because "funds always follow the Paxos-signed
quote" — the same trust model as `AcrossFacetV4`'s `outputAmount`.
`_bridgeData.destinationChainId` is for analytics only.

Then approve `TRANSIT_STATION` `:195-199`, call
`TRANSIT_STATION.submitOrder{value: nativeFee}(quote, signature)` `:201-204`, emit.

### 8.4 CCTP facet comparison

| | CelerCircle | PolymerCCTP | PaxosTransit |
|---|---|---|---|
| Talks to | Celer proxy | Circle `TokenMessenger` directly | Paxos Transit Station |
| Token | USDC only (`onlyAllowSourceToken`) | USDC only (hardcoded) | Paxos stablecoins, per quote |
| Chain id → domain | done by the proxy | **facet's own mapping**, stored `+1` | LayerZero EID inside the quote |
| Non-EVM | no | **Solana, Stellar, HyperCore** | via quote |
| Fee | Circle's `maxFee` | Circle's `maxFee` + a USDC `polymerTokenFee` | native LayerZero fee |
| Authority for the route | calldata | calldata + corridor guards | **Paxos EIP-712 signature** |
| Standing max approval | yes (`initCelerCircleBridge`) | yes (`initPolymerCCTP`) | no, per-call |

---

## 9. Intent and solver facets

Ten facets where the source chain does not send tokens anywhere at all. Instead
it **escrows funds and publishes an order**, and a competing solver fills the
order on the destination chain from their own inventory, then claims the escrow.
The user gets destination funds in seconds; the settlement risk sits with the
solver.

The recurring architectural consequence: the *real* routing instructions live in
an off-chain order or a signed quote, so the on-chain facet's main job is
verifying that the calldata agrees with that order as far as it possibly can.
Several of these facets say plainly in comments that they cannot verify all of it.

### 9.1 DeBridgeDlnFacet

`src/Facets/DeBridgeDlnFacet.sol` — 284 lines, version 1.1.0. Doc: `docs/DeBridgeDlnFacet.md`.

**Integrates**: deBridge Liquidity Network (DLN), a pure order book. You create an
order specifying give/take tokens and amounts; a taker fills it.

- **Namespace**: `keccak256("com.lifi.facets.debridgedln")` `:28-29`. **Constant**: `REFERRAL_CODE = 30729` `:30`. **Immutable**: `IDlnSource public immutable DLN_SOURCE` `:32`.
- **`struct DeBridgeDlnData`** `:41-46`: `bytes receivingAssetId`, `bytes receiver`, `bytes orderAuthorityDst`, `uint256 minAmountOut`. Note the first three are **`bytes`**, not `address` — deBridge supports non-EVM destinations natively.
- **`struct Storage`** `:48-51`, **`struct ChainIdConfig`** `:53-56`.
- **Errors**: `UnknownDeBridgeChain()` `:60`, `EmptyNonEVMAddress()` `:61`, `EmptyOrderAuthorityDst()` `:62`.
- **Events**: `DeBridgeInitialized(ChainIdConfig[])` `:66`, `DlnOrderCreated(bytes32 indexed orderId)` `:68`, `DeBridgeChainIdSet(uint256 indexed, uint256)` `:70`.
- **Modifier `onlyValidDeBridgeDlnData`** `:74-82`: rejects an empty `receiver` (`EmptyNonEVMAddress`) or empty `orderAuthorityDst` (`EmptyOrderAuthorityDst`). The latter matters because that address controls cancellation and refund routing.

**`initDeBridgeDln(ChainIdConfig[] calldata)`** — `:98-113`, owner-only.
**`startBridgeTokensViaDeBridgeDln`** `:120-147`, **`swapAndStartBridgeTokensViaDeBridgeDln`** `:148-178`.
**`setDeBridgeChainId`** `:249-266`, **`getDeBridgeChainId`** `:267-276`, **`getStorage`** `:277-283`.

**`_startBridge(BridgeData memory, DeBridgeDlnData calldata, uint256 _fee)`** — `:179-241`, internal.
Builds `IDlnSource.OrderCreation` `:184-203`:

| Field | Source | Note |
|---|---|---|
| `giveTokenAddress` / `giveAmount` | `sendingAssetId` / `minAmount` | what the user escrows |
| `takeTokenAddress` / `takeAmount` | `receivingAssetId` / `minAmountOut` | what the taker must deliver |
| `takeChainId` | `getDeBridgeChainId(destinationChainId)` | mapped |
| `receiverDst` | `_deBridgeData.receiver` | raw bytes |
| `givePatchAuthoritySrc` | **`msg.sender`** `:196` | the comment `:194-195` notes that even if the caller is the Permit2Proxy, the user retains control from the destination via `orderAuthorityAddressDst` |
| `orderAuthorityAddressDst` | `_deBridgeData.orderAuthorityDst` | can cancel and name the refund beneficiary |
| `allowedTakerDst` / `externalCall` / `allowedCancelBeneficiarySrc` | `""` | empty; the comment `:200-201` explains an empty cancel-beneficiary delegates refund routing to the destination authority |

- **ERC20** `:206-219`: approve, `createOrder{value: _fee}(...)`.
- **Native** `:220-228`: `giveAmount` is **reduced by the fee** (`orderCreation.giveAmount = orderCreation.giveAmount - _fee` `:221`) and the whole `minAmount` is sent as value. The fee comes out of the bridged amount rather than on top.
- Emits `DlnOrderCreated(orderId)` `:230`, then `BridgeToNonEVMChain` if applicable `:232-238` (the `bytes` variant, not `bytes32`), then `LiFiTransferStarted`.

### 9.2 MayanFacet

`src/Facets/MayanFacet.sol` — 652 lines, version 2.0.0. Doc: `docs/MayanFacet.md`.

**Integrates**: Mayan (Wormhole-based, with a Swift order type). Its distinguishing
feature is that the routing is inside an **opaque `protocolData` blob**, and this
facet *parses that blob* to check the receiver — a genuinely unusual defensive
measure.

- **Namespace**: `keccak256("com.lifi.facets.mayan")` `:43`.
- **Constants**: `MAYAN_HYPERCORE_DEPOSITOR = 0x5603...4e33` `:48-49` (the comment `:45-47` explains HyperCore Swift v2 orders set this as `destAddr` while the real receiver lives in `customPayload[0:20]`, and that rotating it requires a facet upgrade); `MAYAN_HYPEREVM_DEST_CHAIN_ID = 47` `:54`.
- **Immutable**: `IMayan public immutable MAYAN` `:56`.
- **`struct MayanData`** `:77-87`, documented at `:58-76`. The subtle field is `mayanAmountIn`: the exact input amount the Mayan quote and `protocolData` were generated for. On the swap entry point the realized output is **bound down** to this committed amount and the positive slippage refunded, so the amount handed to Mayan always matches the opaque blob its quote was built for.
- **Errors**: `InvalidReceiver(address expected, address actual)` `:90` (a facet-local error shadowing the generic one, with useful arguments), `ProtocolDataTooShort()` `:91`.

**`startBridgeTokensViaMayan`** `:107-146`, **`swapAndStartBridgeTokensViaMayan`** `:147-233`.

**`_startBridge(BridgeData memory, MayanData memory)`** — `:234-326`, internal.
1. **Receiver verification by parsing the blob** `:239-264`. Non-EVM: require `nonEVMReceiver != 0`, then `_parseReceiver(protocolData, destinationChainId)` and require the two match, else `InvalidNonEVMReceiver()`. EVM: parse, truncate to an address, require it equals `_bridgeData.receiver`, else `InvalidReceiver(expected, actual)`.
2. **ERC20 path** `:268-293`: first `_readInputAmount(protocolData) == minAmount` else `InvalidAmount()` `:274-279`. The comment `:269-273` explains why this binding matters: Mayan's `forwardERC20` has **no token-return step**, so a mismatch would strand the remainder in Mayan's Forwarder. Then approve and `MAYAN.forwardERC20(token, amount, emptyPermitParams, mayanProtocol, protocolData)`.
3. **Native with a swap** `:294-309`: requires `minMiddleAmount != 0` else `InvalidAmount()` — the comment `:295-296` says it is "the only slippage guard on the native input → middleToken source swap; at zero the swap can be fully sandwiched". Then `MAYAN.swapAndForwardEth{value: minAmount}(...)`.
4. **Native without a swap** `:310-315`: `MAYAN.forwardEth{value: minAmount}(mayanProtocol, protocolData)`.
5. Non-EVM event `:317-323`, then `LiFiTransferStarted`.

**`_parseReceiver(bytes memory protocolData, uint256 destinationChainId)`** — `:335-460`, internal pure.
A selector switch over Mayan's protocol encodings, with fixed byte offsets per
order type. For `LIFI_CHAIN_ID_HYPERCORE` it prefers a Swift v2 `customPayload`
receiver `:339-341` and otherwise falls through to the `destAddr` switch.

**`_parseHypercoreReceiver(...)`** — `:461-566`, internal pure. The HyperCore-specific
path, cross-checking `MAYAN_HYPERCORE_DEPOSITOR` and `MAYAN_HYPEREVM_DEST_CHAIN_ID`.

**`_normalizeAmount(...)`** — `:567-581`, **`_readInputAmount(bytes memory)`** — `:582-603`,
**`_replaceInputAmount(...)`** — `:604-652`, all internal pure. The last one rewrites
the amount encoded inside `protocolData` so the swap path can commit the realized
output into the blob.

### 9.3 RelayDepositoryFacet

`src/Facets/RelayDepositoryFacet.sol` — 145 lines, version 1.0.0. Doc: `docs/RelayDepositoryFacet.md`.

**Integrates**: Relay's depository. The simplest intent facet: deposit funds
against an `orderId` and let Relay's off-chain system do the rest.

- **Immutable**: `address public immutable RELAY_DEPOSITORY` `:28`; constructor reverts `InvalidCallData()` on zero `:43-48` (note: `InvalidCallData`, not the usual `InvalidConfig`).
- **`struct RelayDepositoryData`** `:35-38`: `bytes32 orderId`, `address depositorAddress`.

**`startBridgeTokensViaRelayDepository`** `:55-72`, **`swapAndStartBridgeTokensViaRelayDepository`** `:78-98`.

**`_startBridge`** — `:105-144`, internal.
- `depositorAddress != address(0)` else `InvalidCallData()` `:110-112`. The comment `:109` gives the real reason: otherwise the deposit could be "accidentally credited to msg.sender (our diamond)".
- **The honest warning** `:114-117`: *"We cannot validate / guarantee that the off-chain-data associated with the provided orderId corresponds to the _bridgeData (e.g. receiver, destinationChain)"*, and if the deposit exceeds the amount associated with the order, the overpayment is forwarded to whatever receiver the off-chain order names. This is the intent-facet trust model stated as plainly as it gets.
- Native → `depositNative{value: minAmount}(depositorAddress, orderId)` `:121-126`; ERC20 → approve then `depositErc20(depositorAddress, token, amount, orderId)` `:129-140`.
- Emits `LiFiTransferStarted`.

### 9.4 LiFiIntentEscrowFacetV2

`src/Facets/LiFiIntentEscrowFacetV2.sol` — 272 lines, version 1.0.0. Doc: `docs/LiFiIntentEscrowFacetV2.md`.

**Integrates**: LI.FI's **own** intent escrow (an Open Intents Framework "input
settler"). This is the one bridge where LI.FI is the protocol, not just the router.

- **Immutable**: `address public immutable LIFI_INTENT_ESCROW_SETTLER_V2` `:51`. **Constant**: `MULTIPLIER_BASE = 1e18` `:55`. **Error**: `InvalidDepositAndRefundAddress()` `:46`.
- **Modifier `validateBridgeDataLiFiIntentEscrowV2`** `:35-42`: checks receiver and `minAmount` but **deliberately omits the same-network guard** — the comment `:32-33` says same-chain intents are supported. This is the only facet where source and destination chain may be equal.
- **`struct LiFiIntentEscrowDataV2`** `:72-89`, thirteen fields mapping onto an OIF `StandardOrder`. Each carries an inline comment naming its `StandardOrder` destination. Two deserve attention:
  - `dstCallReceiver` `:59`: when there is destination calldata this becomes the on-chain fund recipient and **must be a `ReceiverOIF` deployment**, but on-chain "it is only checked to be non-zero, not verified to be an instance of `ReceiverOIF`. If it is any other address that accepts an OIF callback, funds may be lost."
  - `outputAmountMultiplier` `:69`: same 1e18-based scaling as `AcrossFacetV4`, folding the quoted price ratio and decimal difference into one factor, with the same "use only LI.FI backend-generated calldata" warning.

**`startBridgeTokensViaLiFiIntentEscrowV2`** `:104-135` — `noNativeAsset` `:110`, ERC20 only.
**`swapAndStartBridgeTokensViaLiFiIntentEscrowV2`** `:136-179`.

**`_startBridge(BridgeData memory, LiFiIntentEscrowDataV2 calldata, uint256 _effectiveOutputAmount)`** — `:180-271`, internal.
- Destination-call flag must match `dstCallSwapData.length > 0` `:187-189`.
- `_effectiveOutputAmount != 0` else `InvalidAmount()` `:190`.
- **Canonical recipient** `:192-211`: `recipient` starts as `_lifiIntentData.recipient`, must be non-zero. For a non-EVM destination, destination calls are rejected (`ReceiverOIF` is an EVM contract) `:196-197` and the non-EVM event is emitted. For EVM, `recipient` must equal `bridgeData.receiver` bytes32-encoded `:208-210`.
- Approve the settler `:216-220`.
- **The recipient swap** `:222-238`: when there is destination calldata, `recipient` is **replaced** by `dstCallReceiver` and the output call is `abi.encode(transactionId, dstCallSwapData, recipient)`. The comment `:225-227` repeats the warning: the `bridgeData.receiver` guard above "does not protect funds on this path".

### 9.5 EcoFacet

`src/Facets/EcoFacet.sol` — 477 lines, version 2.0.0. Doc: `docs/EcoFacet.md`.

**Integrates**: Eco Portal, an intent protocol where you publish a *route* (a list
of calls to execute on the destination) plus a *reward* (what the solver gets).

- **Immutable**: `IEcoPortal public immutable PORTAL` `:39`, plus `BACKEND_SIGNER`.
- **Constants**: `ECO_CHAIN_ID_TRON = 728126428` `:42`, `ECO_CHAIN_ID_SOLANA = 1399811149` `:43`, `ECO_PAYLOAD_TYPEHASH` `:46`, `NATIVE_REWARD_AMOUNT = 0` `:51`, `ALLOW_PARTIAL_FILL = false` `:52`, and four Solana-specific layout constants `:53-56`: `SOLANA_ENCODED_ROUTE_LENGTH = 319`, `SOLANA_RECEIVER_OFFSET = 251`, `SOLANA_RECEIVER_END = 283`, `SOLANA_ADDRESS_MIN_LENGTH = 32`, `SOLANA_ADDRESS_MAX_LENGTH = 44`.
- **Errors**: `IntentAlreadyFunded()` `:33`, `SignatureExpired()` `:35`.
- **Structs**: `Route` `:69-80`, `Call` `:82-94`, `EcoData` `:96-105` (`nonEVMReceiver`, `prover`, `rewardDeadline`, `encodedRoute`, `solanaATA`, `refundRecipient`, `deadline`, `signature`).

**`startBridgeTokensViaEco`** `:125-154`, **`swapAndStartBridgeTokensViaEco`** `:155-195`.

**`_buildReward(...)`** — `:196-217`, internal. Assembles the solver's reward from the bridged token and amount, with zero native.

**`_startBridge`** — `:218-281`, internal.
- Chain-id mapping `:230-240`: Tron and Solana get their Eco-specific ids; anything else must fit in `uint64` else `InvalidConfig()`.
- `intentHash = _getIntentHash(destination, encodedRoute, reward)` `:242-246`.
- **Idempotency check** `:248-250`: `PORTAL.getRewardStatus(intentHash) != Status.Initial` reverts `IntentAlreadyFunded()`. Publishing the same intent twice would double-fund the solver.
- Approve and `PORTAL.publishAndFund(destination, encodedRoute, reward, ALLOW_PARTIAL_FILL)` `:252-262`.

**Validation helpers**: `_validateEcoData` `:282-334`, `_decodeRouteReceiver` `:335-364`
(reads the receiver out of `encodedRoute` at the fixed Solana offsets),
`_validateTronReceiver` `:365-374`, `_validateSolanaReceiver` `:375-404`,
`_getIntentHash` `:405-419`, `_verifySignature` `:420-462`, `_domainSeparator` `:463-477`.

### 9.6 NEARIntentsFacet

`src/Facets/NEARIntentsFacet.sol` — 339 lines, version 1.0.0. Doc: `docs/NEARIntentsFacet.md`.

**Integrates**: NEAR Intents via the 1Click API. The mechanism is almost
comically simple on-chain: **transfer tokens to a deposit address** the API gave
you, and emit an event NEAR's infrastructure watches. All the safety comes from
the EIP-712 signature over the quote.

- **Namespace**: `keccak256("com.lifi.facets.nearintents")` `:31`. **Typehash**: `NEARINTENTS_PAYLOAD_TYPEHASH = 0x26e3f312...903e` `:35-36`. **Immutable**: `address internal immutable BACKEND_SIGNER` `:39`.
- **`struct NEARIntentsData`** `:51-59`: `nonEVMReceiver`, `depositAddress`, `quoteId`, `deadline`, `minAmountOut`, `refundRecipient`, `signature`.
- **`struct Storage`** `:64-67`: only `mapping(bytes32 => bool) consumedQuoteIds`.
- **Event `NEARIntentsBridgeStarted(bytes32 indexed transactionId, bytes32 indexed quoteId, address indexed depositAddress, address sendingAssetId, uint256 amount, uint256 deadline, uint256 minAmountOut)`** `:80-88`. The comment `:72` states it is "required by NEAR off-chain infrastructure to track deposits and initiate intent settlement" — this event *is* the bridge message.
- **Errors**: `QuoteAlreadyConsumed()` `:93`, `QuoteExpired()` `:96`, `InvalidSignature()` `:99`.
- **Modifier `onlyValidQuote`** `:117-150`: deadline, replay and signature checks in one place.

**`startBridgeTokensViaNEARIntents`** `:151-179`, **`swapAndStartBridgeTokensViaNEARIntents`** `:180-220`, **`isQuoteConsumed(bytes32)`** `:221-231` external view, **`_startBridge`** `:232-273`, **`_verifySignature`** `:274-312`, **`_domainSeparator`** `:313-331`, **`getStorage`** `:332-338`.

### 9.7 GardenFacet

`src/Facets/GardenFacet.sol` — 194 lines, version 1.0.0. Doc: `docs/GardenFacet.md`.

**Integrates**: Garden Finance, which uses **hash time-locked contracts** — the
classic atomic-swap primitive, mostly used here for Bitcoin.

- **Constant**: `NATIVE_TOKEN_ADDRESS = 0xEeee...EEeE` `:32-33`, Garden's native sentinel. **Immutable**: `IGardenRegistry REGISTRY`, constructor reverts `InvalidConfig()` on zero `:39-42`.
- **`struct GardenData`** `:53-59`: `redeemer` (the solver who can claim with the secret), `refundAddress` (who reclaims after the timelock), `timelock` (**in blocks**, per `:48`), `secretHash` (SHA-256 of the secret), `nonEvmReceiver`.
- The struct comment `:51-52` is the standard intent caveat: "Transfer details (destination chain, receiver) are encoded in an off-chain order. There is no on-chain guarantee that emitted params match the actual transfer details."
- **Errors**: `AssetNotSupported()` `:64`, `InvalidGardenData()` `:66`.

**`startBridgeTokensViaGarden`** `:73-90`, **`swapAndStartBridgeTokensViaGarden`** `:96-116`.

**`_startBridge`** — `:123-193`, internal.
- Reject zero `redeemer`, zero `timelock` or zero `secretHash` → `InvalidGardenData()` `:132-136`; zero `refundAddress` → `InvalidReceiver()` `:139-141`.
- **Registry lookup** `:145-153`: `htlcAddress = REGISTRY.htlcs(assetForGarden)` where native maps to Garden's sentinel; zero means `AssetNotSupported()`. Each asset has its own HTLC contract.
- Native → `garden.initiateOnBehalf{value: minAmount}(refundAddress, redeemer, timelock, minAmount, secretHash)` `:160-166`; ERC20 → approve the HTLC then the same call with no value `:169-181`.
- Non-EVM event `:184-190`, then `LiFiTransferStarted`.

**How the atomic swap completes**: the solver fills on the destination and
reveals the secret; revealing it lets them claim the source escrow. If they never
fill, the timelock expires and `refundAddress` reclaims. No trusted third party —
but the user must be online to complete their side.

### 9.8 ChainflipFacet

`src/Facets/ChainflipFacet.sol` — 246 lines, version 1.0.1. Doc: `docs/ChainflipFacet.md`.

**Integrates**: Chainflip's Vault. Chainflip has its own validator network and
supports native Bitcoin and Solana.

- **Immutable**: `IChainflipVault public immutable CHAINFLIP_VAULT` `:32`; constructor reverts `InvalidConfig()` on zero `:62-67`.
- **Hardcoded chain ids** `:33-38`: `CHAIN_ID_ETHEREUM = 1`, `CHAIN_ID_ARBITRUM = 42161`, and Chainflip's own `CHAINFLIP_ID_ETHEREUM = 1`, `CHAINFLIP_ID_ARBITRUM = 4`, `CHAINFLIP_ID_SOLANA = 5`, `CHAINFLIP_ID_BITCOIN = 3`. Unlike AllBridge or Frax, the mapping is **compiled in**, so adding a chain needs a redeploy.
- **`struct ChainflipData`** `:49-56`: `bytes nonEVMReceiver`, `uint32 dstToken`, `address dstCallReceiver`, `LibSwap.SwapData[] dstCallSwapData`, `uint256 gasAmount`, `bytes cfParameters`.
- **Errors**: `EmptyNonEvmAddress()` `:27`, `UnsupportedChainflipChainId()` `:28`.

**`startBridgeTokensViaChainflip`** `:74-95`, **`swapAndStartBridgeTokensViaChainflip`** `:96-122`.

**`_startBridge`** — `:123-230`, internal.
- `dstChain = _getChainflipChainId(destinationChainId)` `:127`.
- **Address encoding** `:132-156`: non-EVM uses the raw `nonEVMReceiver` bytes (rejecting empty with `EmptyNonEvmAddress()`) and emits `BridgeToNonEVMChain`. For EVM, the encoded address is **`dstCallReceiver` when there is a destination call, otherwise `bridgeData.receiver`** `:147-149` — the same "receiver contract vs end user" split as elsewhere, made explicit.
- Approve once for both branches `:159-165`.
- **With destination call** `:168-199`: requires non-empty `dstCallSwapData` else `InformationMismatch()`, builds `message = abi.encode(transactionId, dstCallSwapData, bridgeData.receiver)`, then `xCallNative{value:}` or `xCallToken(...)`.
- **Without** `:200-...`: requires *empty* `dstCallSwapData` else `InformationMismatch()`, then the plain `xSwapNative`/`xSwapToken` variants.

**`_getChainflipChainId(uint256)`** — `:231-245`, internal pure. Reverts `UnsupportedChainflipChainId()` for anything not in the compiled list.

### 9.9 LayerSwapFacet

`src/Facets/LayerSwapFacet.sol` — 297 lines, version 1.0.0. Doc: `docs/LayerSwapFacet.md`.

**Integrates**: LayerSwap's Depository. Like NEAR Intents, the on-chain action is
a deposit against an off-chain swap id, gated by a backend signature.

- **Namespace**: `keccak256("com.lifi.facets.layerswap")` `:28-29`. **Typehash**: `LAYERSWAP_PAYLOAD_TYPEHASH = 0x3368de29...8335` `:32-33`, whose preimage is spelled out at `:31` and covers ten fields including `requestId`, `depositoryReceiver`, `refundRecipient` and `deadline`.
- **Immutables**: `address public immutable LAYERSWAP_DEPOSITORY` `:38`, `address internal immutable BACKEND_SIGNER` `:41`.
- **`struct Storage`** `:45-48`: `mapping(bytes32 => bool) usedRequestIds`.
- **`struct LayerSwapData`** `:66-73`: `requestId` (from `POST /api/v2/swaps`), `depositoryReceiver` (a whitelisted address the depository forwards to **on the source chain**, distinct from `bridgeData.receiver` which is the destination recipient — the comment `:53-57` is careful about this), `refundRecipient`, `nonEVMReceiver`, `signature`, `deadline`.
- **Errors**: `SignatureExpired()` `:77`, `RequestAlreadyProcessed()` `:78`.

**`startBridgeTokensViaLayerSwap`** `:100-125`, **`swapAndStartBridgeTokensViaLayerSwap`** `:126-170`, **`_validateLayerSwapData`** `:171-184`, **`_startBridge`** `:185-233`, **`_verifySignature`** `:234-274`, **`_domainSeparator`** `:275-290`, **`getStorage`** `:291-296`.

### 9.10 UnitFacet

`src/Facets/UnitFacet.sol` — 237 lines, version 1.0.1. Doc: `docs/UnitFacet.md`.

**Integrates**: Unit (Hyperliquid's bridge). The smallest signed-quote facet:
transfer to a `depositAddress` the backend signed for.

- **Namespace**: `keccak256("com.lifi.facets.unit")` `:27`. **Typehash**: `UNIT_PAYLOAD_TYPEHASH = 0xe40c93b7...4cba` `:30-31`, preimage at `:29` covering `transactionId, minAmount, receiver, depositAddress, destinationChainId, sendingAssetId, deadline`.
- **Immutable**: `address internal immutable BACKEND_SIGNER` `:34`.
- **`struct Storage`** `:38-41`: `mapping(bytes32 => bool) usedTransactionIds`.
- **`struct UnitData`** `:47-51`: `depositAddress`, `signature`, `deadline`. That is the entire bridge-specific payload.
- **Errors**: `InvalidSignature()` `:55`, `SignatureExpired()` `:57`, `UnsupportedChain()` `:59`, `TransactionAlreadyProcessed()` `:61`.

**A warning worth repeating**, from the NatSpec at `:76-81`: Unit enforces
**minimum deposit amounts**, and "amounts below the minimum threshold may result
in irrecoverable fund loss". The minimums are validated by the backend inside the
signed payload, not on-chain.

**`startBridgeTokensViaUnit`** `:84-115`, **`swapAndStartBridgeTokensViaUnit`** `:116-146`, **`_startBridge`** `:147-173`, **`_verifySignature`** `:174-214`, **`_domainSeparator`** `:215-230`, **`getStorage`** `:231-236`.

### 9.11 Intent facet comparison

| | DeBridge DLN | Mayan | Relay | LiFiIntentV2 | Eco | NEAR | Garden | Chainflip | LayerSwap | Unit |
|---|---|---|---|---|---|---|---|---|---|---|
| Settlement | order book | Wormhole/Swift | depository | OIF escrow | portal intent | deposit + event | HTLC | validator network | depository | deposit |
| Signature gate | no | no | no | no | **yes** | **yes** | no | no | **yes** | **yes** |
| Replay guard | — | — | — | `nonce` in order | reward status | `consumedQuoteIds` | `secretHash` | — | `usedRequestIds` | `usedTransactionIds` |
| Destination call | no | no | no | **yes** | via route | no | no | **yes** | no | no |
| Non-EVM | **yes** (`bytes`) | **yes** | no | **yes** | Tron, Solana | **yes** | **yes** (BTC) | **yes** (BTC, SOL) | **yes** | Hyperliquid |
| Chain-id mapping | diamond storage | — | — | — | compiled constants | — | — | **compiled constants** | — | — |
| Native supported | **yes** | **yes** | **yes** | no | no | — | **yes** | **yes** | — | — |
| Parses opaque data | no | **yes** (`protocolData`) | no | no | **yes** (`encodedRoute`) | no | no | no | no | no |

---

## 10. Utility: GasZipFacet

`src/Facets/GasZipFacet.sol` — 156 lines, version 2.0.5. Doc: `docs/GasZipFacet.md`.

**Integrates**: Gas.zip. Not a bridge in the usual sense — it solves the
"I just bridged to a new chain and have no gas to move my tokens" problem by
depositing native on the source chain and having Gas.zip deliver small amounts of
native gas on up to 16 destination chains.

- **Immutable**: `IGasZip public immutable GAS_ZIP_ROUTER` `:33`. **Constant**: `MAX_CHAINID_LENGTH_ALLOWED = 16` `:34`.
- **Errors**: `OnlyNativeAllowed()` `:29`, `TooManyChainIds()` `:30`.
- Uses `IGasZip.GasZipData` (defined in the interface, not the facet): `destinationChains` (a packed uint256) and `receiverAddress` (bytes32).

**`startBridgeTokensViaGasZip(BridgeData memory, IGasZip.GasZipData calldata)`** — `:48-67`.
Modifiers are only `nonReentrant doesNotContainSourceSwaps doesNotContainDestinationCalls` — **no `validateBridgeData`**, because `_startBridge` does its own. Body:
- `if (!LibAsset.isNativeAsset(sendingAssetId)) revert OnlyNativeAllowed();` `:59-60`.
- `if (msg.value != _bridgeData.minAmount) revert InvalidAmount();` `:63` — exact match, no surplus tolerated on this path.

**`swapAndStartBridgeTokensViaGasZip(BridgeData memory, SwapData[], GasZipData)`** — `:74-103`.
Requires the **last swap's output to be native** else `InvalidCallData()` `:87-91`,
since Gas.zip only accepts native. This is the "swap my leftover USDC into gas on
five chains" path.

**`_startBridge`** — `:108-137`, internal.
- `receiverAddress != bytes32(0)` else `InvalidCallData()` `:113-114`.
- **Right-padded receiver check** `:117-122`: for EVM destinations, `receiverAddress` must equal `bytes32(bytes20(uint160(receiver)))`. The comment `:121` is worth noting: Gas.zip expects the address **right-padded**, so the code uses `bytes20` rather than the usual `uint256` cast, which would left-pad. Getting this backwards would silently send gas to the wrong address.
- `destinationChainId != block.chainid` else `CannotBridgeToSameNetwork()` `:126-127`.
- `GAS_ZIP_ROUTER.deposit{value: minAmount}(destinationChains, receiverAddress)` `:131-134`, then `emit LiFiTransferStarted`.

**`getDestinationChainsValue(uint8[] calldata _chainIds)`** — `:142-155`, external pure.
A helper that packs up to 16 Gas.zip-specific chain ids into one `uint256`:

```solidity
for (uint256 i; i < length; ++i) {
    destinationChains = (destinationChains << 16) | uint256(_chainIds[i]);
}
```
`:149-154`. Each id occupies 16 bits, so 16 ids fill the word — hence
`MAX_CHAINID_LENGTH_ALLOWED`, enforced with `TooManyChainIds()` `:147`. Note the
input is `uint8` while the shift is 16 bits, so the high byte of each slot is
always zero; the format reserves room. These are **Gas.zip's own chain ids**, not
EVM chain ids `:140-141`.

**When the router picks this facet**: as the tail of a route, or standalone when
the user asks to "refuel" several chains at once.

---

## 11. Cross-facet tables

### 11.1 Every custom error declared in a facet

43 facet-local errors. Errors from `src/Errors/GenericErrors.sol` (45 of them, listed at `GenericErrors.sol:5-45`) are shared and not repeated here.

| Error | Facet | Line | Cause |
|---|---|---|---|
| `UnsupportedAllBridgeChainId()` | AllBridge | `:36` | destination not in the chain-id map |
| `EmptyNonEvmAddress()` | Chainflip | `:27` | non-EVM receiver bytes are empty |
| `UnsupportedChainflipChainId()` | Chainflip | `:28` | destination not in the compiled list |
| `IntentAlreadyFunded()` | Eco | `:33` | `PORTAL.getRewardStatus(intentHash) != Initial` |
| `SignatureExpired()` | Eco | `:35` | backend signature past its deadline |
| `InsufficientNativeValue()` | Superset | `:143` | `msg.value` below the required fee |
| `AssetNotSupported()` | Garden | `:64` | no HTLC registered for the asset |
| `InvalidGardenData()` | Garden | `:66` | zero redeemer, timelock or secretHash |
| `OnlyNativeAllowed()` | GasZip | `:29` | non-native asset on the direct path |
| `TooManyChainIds()` | GasZip | `:30` | more than 16 destination chains |
| `WithdrawFailed()` | AcrossFacetPackedV4 | `:36` | the pre-withdraw call reverted |
| `InvalidInputAmount()` | AcrossFacetPackedV4 | `:38` | `inputAmount > type(uint128).max` when encoding |
| `InvalidCalldataLength()` | AcrossFacetPackedV4 | `:39` | packed calldata shorter than 192 / 240 bytes |
| `QuoteAlreadyConsumed()` | NEARIntents | `:93` | `quoteId` already used |
| `QuoteExpired()` | NEARIntents | `:96` | quote past its deadline |
| `InvalidSignature()` | NEARIntents | `:99` | recovered signer is not `BACKEND_SIGNER` |
| `NoNullOwner()` | Ownership | `:27` | transferring ownership to zero |
| `NewOwnerMustNotBeSelf()` | Ownership | `:28` | new owner equals current owner |
| `NoPendingOwnershipTransfer()` | Ownership | `:29` | cancelling with nothing pending |
| `NotPendingOwner()` | Ownership | `:30` | confirm called by the wrong address |
| `FacetIsNotRegistered()` | EmergencyPause | `:26` | facet has no selectors in the Diamond |
| `NoFacetToPause()` | EmergencyPause | `:27` | nothing but this facet is registered |
| `SignatureExpired()` | LayerSwap | `:77` | signature past its deadline |
| `RequestAlreadyProcessed()` | LayerSwap | `:78` | `requestId` already used |
| `OnchainSwapV3NotSupported()` | Symbiosis | `:84` | router not configured on this chain |
| `InvalidSignature()` | Symbiosis | `:86` | bad backend signature |
| `SignatureExpired()` | Symbiosis | `:88` | past deadline |
| `TransactionAlreadyProcessed()` | Symbiosis | `:90` | transaction id replayed |
| `OnchainSwapV3FeeMismatch()` | Symbiosis | `:92` | quoted fee differs from the router's live `fee()` |
| `WithdrawFailed()` | Withdraw | `:17` | the pre-withdraw call reverted |
| `InvalidSignature()` | Unit | `:55` | bad backend signature |
| `SignatureExpired()` | Unit | `:57` | past deadline |
| `UnsupportedChain()` | Unit | `:59` | destination not supported |
| `TransactionAlreadyProcessed()` | Unit | `:61` | transaction id replayed |
| `InvalidReceiver(address expected, address actual)` | Mayan | `:90` | parsed `protocolData` receiver differs from `bridgeData.receiver` |
| `ProtocolDataTooShort()` | Mayan | `:91` | blob too short to parse |
| `UnknownDeBridgeChain()` | DeBridgeDln | `:60` | destination not in the map |
| `EmptyNonEVMAddress()` | DeBridgeDln | `:61` | empty `receiver` bytes |
| `EmptyOrderAuthorityDst()` | DeBridgeDln | `:62` | empty destination order authority |
| `InvalidDepositAndRefundAddress()` | LiFiIntentEscrowV2 | `:46` | zero deposit/refund address |
| `InvalidRouteType()` | Squid | `:62` | `routeType` outside the enum |
| `DeprecatedToken()` | ThorSwap | `:36` | bridging ERC20 RUNE on mainnet |
| `InvalidAssetId(uint16 invalidAssetId)` | StargateV2 | `:38` | `tokenMessaging.stargateImpls(assetId)` returned zero |

Note the deliberate duplication: `SignatureExpired`, `InvalidSignature` and
`TransactionAlreadyProcessed` are each declared separately in three or four
facets rather than shared. Same name, same selector, different declaration site.

**The one string-based revert left in the facets** is
`"Invalid callTo length; expected at least 20 bytes"` in
`CalldataVerificationFacet.sol:349`.

### 11.2 Events declared in facets

Beyond `ILiFi`'s six shared events (§2.1), fourteen facets declare their own:

| Facet | Count | Events |
|---|---|---|
| `AccessManagerFacet` | 2 | `ExecutionAllowed`, `ExecutionDenied` |
| `AcrossFacetPackedV4` | 2 | `LiFiAcrossTransfer(bytes8)`, `CallExecutedAndFundsWithdrawn()` |
| `AllBridgeFacet` | 3 | `AllBridgeChainMappingsInitialized`, `ChainIdToAllBridgeChainIdSet`, `ChainIdToAllBridgeChainIdUnset` |
| `MegaETHBridgeFacet` | 2 | `MegaETHInitialized`, `MegaETHBridgeRegistered` |
| `OwnershipFacet` | 1 | `OwnershipTransferRequested` |
| `NEARIntentsFacet` | 1 | `NEARIntentsBridgeStarted` |
| `SupersetFacet` | 2 | `SupersetChainMappingsInitialized`, `ChainIdToEidSet` |
| `EmergencyPauseFacet` | 3 | `EmergencyFacetRemoved`, `EmergencyPaused`, `EmergencyUnpaused` |
| `WithdrawFacet` | 1 | `LogWithdraw` |
| `PeripheryRegistryFacet` | 1 | `PeripheryContractRegistered` |
| `OptimismBridgeFacet` | 2 | `OptimismInitialized`, `OptimismBridgeRegistered` |
| `PolymerCCTPFacet` | 4 | `PolymerCCTPFeeSent`, `PolymerCCTPChainMappingsInitialized`, `ChainIdToDomainIdSet`, `ChainIdToDomainIdUnset` |
| `FraxFacet` | 3 | `FraxChainMappingsInitialized`, `FraxChainIdToEidSet`, `FraxChainIdToEidUnset` |
| `DeBridgeDlnFacet` | 3 | `DeBridgeInitialized`, `DlnOrderCreated`, `DeBridgeChainIdSet` |

`WhitelistManagerFacet` emits `ContractSelectorWhitelistChanged`, declared in
`IWhitelistManagerFacet` rather than the facet.

**Two facets do not emit `LiFiTransferStarted`**: `AcrossFacetPackedV4` emits
`LiFiAcrossTransfer(bytes8)` instead, and `GenericSwapFacetV3` emits
`LiFiGenericSwapCompleted` (no bridge involved). Any indexer must handle both.

### 11.3 Which facets accept native

| Accepts native as the bridged asset | Payable but native-fee only | Never native |
|---|---|---|
| Arbitrum, Optimism, Polygon, Omni, MegaETH, AcrossFacet, AcrossFacetV4, AcrossFacetPackedV4, StargateV2, Squid, Symbiosis, ThorSwap, DeBridgeDln, Mayan, Relay, Garden, Chainflip, GasZip | AllBridge, CelerCircle (swap path), Frax, Superset, Paxos, PolymerCCTP | Gnosis (`not payable`), Glacis (`noNativeAsset`), LiFiIntentEscrowV2 (`noNativeAsset`), Paxos (`noNativeAsset`) |

### 11.4 Facets holding diamond storage

Sixteen namespaces, verified per facet: `com.lifi.reentrancyguard` (shared, in
`ReentrancyGuard`), plus `com.lifi.facets.` + `allbridge` (`AllBridgeFacet.sol:33`),
`debridgedln` (`:28`), `emergencyPauseFacet` (`:33`), `frax` (`:42`),
`layerswap` (`:28`), `mayan` (`:43`), `megaeth` (`:21`), `nearintents` (`:31`),
`optimism` (`:26`), `ownership` (`:16`), `periphery_registry` (`:13`),
`polymercctp` (`:81`), `superset` (`:62`), `symbiosis` (`:50`), `unit` (`:27`).

### 11.5 Facets requiring an init call before use

| Facet | Init function | Reverts if skipped |
|---|---|---|
| `OptimismBridgeFacet` | `initOptimism` | `registerOptimismBridge` → `NotInitialized()`; `_startBridge` silently uses zero addresses |
| `MegaETHBridgeFacet` | `initMegaETH` | `NotInitialized()` in `_startBridge` too |
| `AllBridgeFacet` | `initAllBridge` | `UnsupportedAllBridgeChainId()` |
| `DeBridgeDlnFacet` | `initDeBridgeDln` | `UnknownDeBridgeChain()` |
| `PolymerCCTPFacet` | `initPolymerCCTP` | unmapped domain |
| `FraxFacet` | `initFrax` | unmapped EID |
| `SupersetFacet` | `initSuperset` | unmapped EID |
| `CelerCircleBridgeFacet` | `initCelerCircleBridge` | the `depositForBurn` pull fails with no allowance |
| `AcrossFacetPackedV4` | `setApprovalForBridge` | the SpokePool pull fails |

---

## 12. Use-case index

"I want to…" → the facet and function, with the internal chain.

**1. Swap USDC for WETH on Arbitrum, no bridge.**
`GenericSwapFacetV3.swapTokensSingleV3ERC20ToERC20` →
`_depositAndSwapERC20Single` → allowlist → `maxApproveERC20` → DEX call →
`_returnPositiveSlippageERC20` → `transferERC20` → `LiFiGenericSwapCompleted`.

**2. Swap ETH for a token on one chain.** `GenericSwapFacetV3.swapTokensSingleV3NativeToERC20` (payable). For a multi-hop route use the `swapTokensMultipleV3*` triple.

**3. Bridge USDC Arbitrum → Base, fastest, no destination call.**
`AcrossFacetV4.startBridgeTokensViaAcrossV4` → `_startBridge` →
`SPOKEPOOL.deposit(...)`. A relayer fills on Base within seconds.

**4. Same route, minimum gas.** `AcrossFacetPackedV4.startBridgeTokensViaAcrossV4ERC20Packed` with calldata built by `encode_startBridgeTokensViaAcrossV4ERC20Packed`. No validation, no `nonReentrant`, standing approvals.

**5. Swap then bridge in one transaction.** Any `swapAndStartBridgeTokensVia<X>`. The internal shape is always `_depositAndSwap(...)` → assign `minAmount` → `_startBridge`.

**6. Bridge and run a swap on the destination.**
`AcrossFacetV4` with a non-empty `message` and `hasDestinationCall = true`, or
`StargateFacetV2` with `sendParams.composeMsg`, or `ChainflipFacet` /
`LiFiIntentEscrowFacetV2` with `dstCallSwapData`. In every case
`bridgeData.receiver` becomes LI.FI's Receiver contract, and the end user's
address travels inside the message. The destination flow is
Receiver → `Executor.swapAndCompleteBridgeTokens` → `LiFiTransferCompleted`, or on
failure raw tokens plus `LiFiTransferRecovered`.

**7. Move native USDC with no wrapped asset.** `PolymerCCTPFacet.startBridgeTokensViaPolymerCCTP` (Circle CCTP burn/mint), or `CelerCircleBridgeFacet` where a Celer relayer completes the mint for you.

**8. Deposit ETH to an L2 with maximum trust-minimisation.** The canonical bridge for that chain: `ArbitrumBridgeFacet`, `OptimismBridgeFacet`, `PolygonBridgeFacet`, `MegaETHBridgeFacet`. Slow, but security equals the rollup's.

**9. Bridge DAI to Gnosis Chain.** `GnosisBridgeFacet.startBridgeTokensViaGnosisBridge` — DAI or USDS only, chain 100 only.

**10. Bridge to Solana.** Set `bridgeData.receiver = NON_EVM_ADDRESS` (`0x11f1…11F1`) and put the pubkey in the facet's own field: `AcrossFacetV4.receiverAddress`, `PolymerCCTPFacet.nonEVMReceiver` + `solanaReceiverATA`, `MayanFacet.nonEVMReceiver`, `ChainflipFacet.nonEVMReceiver`, `EcoFacet` via the encoded route. Watch for `BridgeToNonEVMChain` / `BridgeToNonEVMChainBytes32`.

**11. Bridge to Bitcoin.** `GardenFacet` (HTLC), `ChainflipFacet`, `ThorSwapFacet` (memo), or `SymbiosisFacet` with `viaOnchainSwapV3 = true`.

**12. Get gas on five chains at once.** `GasZipFacet.startBridgeTokensViaGasZip`, packing the chain list with `getDestinationChainsValue`. To fund it from an ERC20, use `swapAndStartBridgeTokensViaGasZip` whose last swap must output native.

**13. Let a user pay with a Permit2 signature instead of an approval.** The user signs for `Permit2Proxy` (see the periphery reference), which calls the Diamond. Inside the facet, `msg.sender` is then the proxy — which is exactly why `PaxosTransitFacet`, `MayanFacet`, `LayerSwapFacet`, `NEARIntentsFacet` and `PolymerCCTPFacet` all take an explicit `refundRecipient` instead of refunding `msg.sender`.

**14. Add a new bridge integration.** Follow `docs/AddingANewBridge.md`: `bun codegen facet` scaffolds the facet, doc, deploy script, config and test. Implement `startBridgeTokensVia<X>`, `swapAndStartBridgeTokensVia<X>` and `_startBridge`, then register it with a diamond cut.

**15. Add a new facet to a live Diamond.** `DiamondCutFacet.diamondCut(FacetCut[], _init, _calldata)` with `action: Add`, executed by the owner (in production, the timelock). Verify afterwards with `DiamondLoupeFacet.facets()`.

**16. Whitelist a new DEX.** `WhitelistManagerFacet.batchSetContractSelectorWhitelist(contracts, selectors, true)`. If the DEX uses a separate spender, also whitelist that address against the `0xffffffff` sentinel.

**17. Respond to an incident.** `EmergencyPauseFacet.removeFacet(addr)` to kill one integration, or `pauseDiamond()` to redirect every selector to `DiamondIsPaused()`. Both callable by the hot pauser wallet. Recovery is `unpauseDiamond(blacklist)`, owner-only.

**18. Recover tokens stuck in the Diamond.** `WithdrawFacet.withdraw(asset, to, amount)`, or `executeCallAndWithdraw` when a bridge refund must be claimed first.

**19. Show a user what they are signing.** `CalldataVerificationFacet.extractMainParameters(data)` for the human-readable summary, `validateCalldata(...)` to assert expectations, `extractNonEVMAddress(data)` for a non-EVM receiver. Destination-call verification works only for Stargate V2.

---

## Reading this alongside the code

Every function above cites a `file:line` verified with `grep -n` against
`lifi/contracts/` as cloned. To follow along:

```bash
cd lifi/contracts
sed -n '107,141p' src/Facets/AcrossFacet.sol          # the canonical _startBridge
sed -n '340,393p' src/Facets/GenericSwapFacetV3.sol   # the allowlist + approve + call
sed -n '342,430p' src/Facets/PolymerCCTPFacet.sol     # the corridor dispatch
grep -n 'function _startBridge' src/Facets/*.sol      # every bridge's business logic
```

For the Diamond internals, `LibSwap`, `LibAsset`, `LibAllowList`, the `Executor`,
the Receiver contracts and `LiFiDEXAggregator`, continue to
[`LIBRARIES-PERIPHERY-COMPLETE-REFERENCE.md`](LIBRARIES-PERIPHERY-COMPLETE-REFERENCE.md).
For the narrative version of how a route executes end to end, see
[`LIFI-DEEP-DIVE.md`](LIFI-DEEP-DIVE.md).
