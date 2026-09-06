# LI.FI — Libraries, Helpers, Periphery & Diamond Core: Complete Reference

Exhaustive, function-by-function reference for **everything under
`lifi/contracts/src/` except `src/Facets/`**. The facets have their own document,
[`lifi/FACETS-COMPLETE-REFERENCE.md`](FACETS-COMPLETE-REFERENCE.md); the
conceptual walkthrough is [`lifi/LIFI-DEEP-DIVE.md`](LIFI-DEEP-DIVE.md). This
file is the one to read if you want to have *seen every line that matters* in the
plumbing.

Scope: **97 Solidity files, 9,367 lines** — 1 diamond proxy, 9 libraries,
6 helpers, 63 interfaces, 1 error file, 1 security contract, 16 periphery
contracts. Every `path:line` below was verified with `grep -n` against this
checkout. All paths are relative to `lifi/contracts/`.

---

## Table of contents

- [0. How to read this](#0-how-to-read-this)
- [1. File inventory](#1-file-inventory)
- [2. The Diamond (EIP-2535)](#2-the-diamond-eip-2535)
  - [2.1 `LiFiDiamond.sol`](#21-lifidiamondsol)
  - [2.2 `LibDiamond.sol`](#22-libdiamondsol)
  - [2.3 `LibDiamondLoupe.sol`](#23-libdiamondloupesol)
  - [2.4 A note on `LibStorage`](#24-a-note-on-libstorage)
- [3. Namespaced storage: the slot table](#3-namespaced-storage-the-slot-table)
- [4. The two permission systems](#4-the-two-permission-systems)
  - [4.1 `LibAllowList.sol` — what may be called](#41-liballowlistsol--what-may-be-called)
  - [4.2 `LibAccess.sol` — who may call](#42-libaccesssol--who-may-call)
- [5. Asset and swap primitives](#5-asset-and-swap-primitives)
  - [5.1 `LibAsset.sol`](#51-libassetsol)
  - [5.2 `LibSwap.sol`](#52-libswapsol)
  - [5.3 `LibBytes.sol`](#53-libbytessol)
  - [5.4 `LibUtil.sol`](#54-libutilsol)
  - [5.5 `OFTComposeMsgCodec.sol`](#55-oftcomposemsgcodecsol)
- [6. Helpers](#6-helpers)
  - [6.1 `SwapperV2.sol`](#61-swapperv2sol)
  - [6.2 `Validatable.sol`](#62-validatablesol)
  - [6.3 `ReentrancyGuard.sol`](#63-reentrancyguardsol)
  - [6.4 `TransferrableOwnership.sol`](#64-transferrableownershipsol)
  - [6.5 `WithdrawablePeriphery.sol`](#65-withdrawableperipherysol)
  - [6.6 `LiFiData.sol`](#66-lifidatasol)
- [7. Interfaces](#7-interfaces)
  - [7.1 `ILiFi.sol` — the protocol's vocabulary](#71-ilifisol--the-protocols-vocabulary)
  - [7.2 Full interface catalogue](#72-full-interface-catalogue)
- [8. Errors](#8-errors)
- [9. Security: `LiFiTimelockController.sol`](#9-security-lifitimelockcontrollersol)
- [10. Periphery: execution core](#10-periphery-execution-core)
  - [10.1 `Executor.sol`](#101-executorsol)
  - [10.2 `ERC20Proxy.sol`](#102-erc20proxysol)
- [11. Periphery: the Receivers](#11-periphery-the-receivers)
  - [11.1 `ReceiverAcrossV3.sol`](#111-receiveracrossv3sol)
  - [11.2 `ReceiverAcrossV4.sol`](#112-receiveracrossv4sol)
  - [11.3 `ReceiverStargateV2.sol`](#113-receiverstargatev2sol)
  - [11.4 `ReceiverChainflip.sol`](#114-receiverchainflipsol)
  - [11.5 `ReceiverOIF.sol`](#115-receiveroifsol)
  - [11.6 Receiver comparison](#116-receiver-comparison)
- [12. Periphery: fees](#12-periphery-fees)
  - [12.1 `FeeCollector.sol`](#121-feecollectorsol)
  - [12.2 `FeeForwarder.sol`](#122-feeforwardersol)
- [13. Periphery: wrappers and liquid staking](#13-periphery-wrappers-and-liquid-staking)
  - [13.1 `TokenWrapper.sol`](#131-tokenwrappersol)
  - [13.2 `LidoWrapper.sol`](#132-lidowrappersol)
- [14. Periphery: `OutputValidator.sol`](#14-periphery-outputvalidatorsol)
- [15. Periphery: `GasZipPeriphery.sol`](#15-periphery-gaszipperipherysol)
- [16. Periphery: `Permit2Proxy.sol`](#16-periphery-permit2proxysol)
- [17. Periphery: `Patcher.sol`](#17-periphery-patchersol)
- [18. Periphery: `LiFiDEXAggregator.sol`](#18-periphery-lifidexaggregatorsol)
- [19. Cross-cutting tables](#19-cross-cutting-tables)
- [20. Use-case index](#20-use-case-index)
- [21. Security notes](#21-security-notes)

---

## 0. How to read this

Every function gets the same nine-part treatment:

1. **Signature** — exact, with visibility and `file:line`
2. **Purpose** — one line
3. **Parameters** — each one explained
4. **Checks** — every `require`/`revert`, with the exact error
5. **State writes** — what storage changes
6. **External calls** — in order, with untrusted ones marked
7. **Returns / events**
8. **Callers** — who invokes it, and who is permitted to
9. **Gotchas** — the thing that will bite you

Trivial functions get a compressed form. Nothing is skipped.

**Two conventions that run through the whole codebase:**

- **`address(0)` means native.** `LibAsset.NULL_ADDRESS` (`src/Libraries/LibAsset.sol:23`)
  is the asset id for ETH/BNB/AVAX/etc. Every asset-handling function branches on
  `isNativeAsset()`. Note that two *other* native sentinels appear in the tree for
  external-protocol compatibility: `0xEeee…EEeE` in `ReceiverChainflip`
  (`src/Periphery/ReceiverChainflip.sol:29-30`) and `LiFiDEXAggregator`
  (`src/Periphery/LiFiDEXAggregator.sol:19`). Do not mix them up.
- **Balance-difference accounting.** The protocol almost never trusts a return
  value from an external contract. It reads its own balance before, makes the
  call, reads after, and uses the delta. This is what makes arbitrary-DEX calls
  survivable, and it is why fee-on-transfer tokens mostly work.

---

## 1. File inventory

### Diamond core, libraries, helpers, security

| Path | Lines | Purpose |
|---|---:|---|
| `src/LiFiDiamond.sol` | 72 | EIP-2535 proxy: constructor bootstraps `diamondCut`, fallback delegatecalls by selector |
| `src/Libraries/LibDiamond.sol` | 364 | Diamond storage, cut logic, ownership, selector bookkeeping |
| `src/Libraries/LibDiamondLoupe.sol` | 56 | Internal (non-`delegatecall`) loupe reads for facets that need them |
| `src/Libraries/LibAllowList.sol` | 355 | Dual-model allowlist: which contract+selector pairs may be called |
| `src/Libraries/LibAccess.sol` | 65 | Per-selector caller permissions inside the diamond |
| `src/Libraries/LibAsset.sol` | 210 | Native/ERC20 transfer, deposit, approval helpers |
| `src/Libraries/LibSwap.sol` | 109 | `SwapData` struct + the generic "call a whitelisted DEX" primitive |
| `src/Libraries/LibBytes.sol` | 155 | `slice`, `toAddress`, `toHexString`, bytes32↔address casts |
| `src/Libraries/LibUtil.sol` | 34 | `getRevertMsg`, `isZeroAddress`, `revertWith` |
| `src/Libraries/OFTComposeMsgCodec.sol` | 98 | LayerZero OFT compose-message decoder (vendored) |
| `src/Helpers/SwapperV2.sol` | 357 | Deposit → swap → slippage check → refund leftovers |
| `src/Helpers/Validatable.sol` | 76 | Six modifiers validating `BridgeData` |
| `src/Helpers/ReentrancyGuard.sol` | 52 | Diamond-storage reentrancy lock |
| `src/Helpers/TransferrableOwnership.sol` | 58 | Two-step ownership for periphery |
| `src/Helpers/WithdrawablePeriphery.sol` | 42 | `withdrawToken` escape hatch, inherited by most periphery |
| `src/Helpers/LiFiData.sol` | 23 | Non-EVM chain-id constants and `NON_EVM_ADDRESS` |
| `src/Errors/GenericErrors.sol` | 45 | 40 shared custom errors |
| `src/Security/LiFiTimelockController.sol` | 83 | OZ `TimelockController` + emergency unpause bypass |

### Periphery

| Path | Lines | Purpose |
|---|---:|---|
| `src/Periphery/Executor.sol` | 264 | Destination-chain arbitrary execution + swaps |
| `src/Periphery/ERC20Proxy.sol` | 63 | Holds user approvals so the Executor never does |
| `src/Periphery/ReceiverAcrossV3.sol` | 131 | Across V3 destination handler |
| `src/Periphery/ReceiverAcrossV4.sol` | 127 | Across V4 destination handler |
| `src/Periphery/ReceiverStargateV2.sol` | 246 | Stargate V2 / LayerZero `lzCompose` handler |
| `src/Periphery/ReceiverChainflip.sol` | 180 | Chainflip `cfReceive` handler |
| `src/Periphery/ReceiverOIF.sol` | 134 | Open Intent Framework `outputFilled` handler |
| `src/Periphery/FeeCollector.sol` | 185 | Integrator + LI.FI fee ledger |
| `src/Periphery/FeeForwarder.sol` | 127 | Stateless fee fan-out |
| `src/Periphery/TokenWrapper.sol` | 112 | WETH wrap/unwrap, optionally via a decimal converter |
| `src/Periphery/LidoWrapper.sol` | 112 | stETH ↔ wstETH on L2 |
| `src/Periphery/OutputValidator.sol` | 150 | Skims positive slippage to a validation wallet |
| `src/Periphery/GasZipPeriphery.sol` | 161 | Swap-to-native then deposit into gas.zip |
| `src/Periphery/Permit2Proxy.sol` | 389 | Permit2 / EIP-2612 gasless entry to the diamond |
| `src/Periphery/Patcher.sol` | 459 | Runtime calldata patching with dynamically fetched values |
| `src/Periphery/LiFiDEXAggregator.sol` | 1820 | Byte-stream route processor across 11 DEX families |

### Interfaces (63 files)

Catalogued in full in [§7.2](#72-full-interface-catalogue).

---

## 2. The Diamond (EIP-2535)

The LI.FI Diamond is one address that answers every selector by `delegatecall`ing
into a facet. All facets share the diamond's storage, which is why every piece of
shared state in this codebase lives at a hand-picked namespaced slot rather than
in a declared state variable.

```
                       user tx: swapAndStartBridgeTokensViaAcrossV4(...)
                                            |
                                            v
                                 +----------------------+
                                 |    LiFiDiamond       |   <-- holds ALL storage
                                 |  fallback() :32      |
                                 +----------------------+
                                            |
              ds.selectorToFacetAndPosition[msg.sig].facetAddress
                                            |
                                   delegatecall (:55)
                                            v
                                 +----------------------+
                                 |   AcrossFacetV4      |   <-- code only, no storage
                                 +----------------------+
                                            |
                        reads/writes the DIAMOND's storage via
                        LibAllowList / LibAccess / ReentrancyGuard
```

### 2.1 `LiFiDiamond.sol`

**Contract** `LiFiDiamond` (`src/LiFiDiamond.sol:13`), Solidity `^0.8.17`,
version tag `1.0.0`. No inheritance. **No declared state variables** — all state
is reached through namespaced slots.

#### `constructor(address _contractOwner, address _diamondCutFacet) payable` — `:14`

- **Purpose.** Bootstrap the diamond with exactly one function: `diamondCut`.
- **Parameters.** `_contractOwner` becomes `ds.contractOwner`; `_diamondCutFacet`
  is the facet that will serve `diamondCut`.
- **Checks.** None directly, but `LibDiamond.addFunctions` → `addFacet` →
  `enforceHasContractCode` reverts `FacetContainsNoCode()` if `_diamondCutFacet`
  has no code.
- **State writes.** `ds.contractOwner`; then via `diamondCut`:
  `ds.selectorToFacetAndPosition[IDiamondCut.diamondCut.selector]`,
  `ds.facetFunctionSelectors[_diamondCutFacet]`, `ds.facetAddresses`.
- **Calls.** `LibDiamond.setContractOwner` (`:15`), `LibDiamond.diamondCut` (`:26`)
  with `_init = address(0)` and empty calldata.
- **Events.** `OwnershipTransferred(address(0), _contractOwner)` and
  `DiamondCut([...], address(0), "")`.
- **Gotcha.** The constructor is `payable` purely as a gas micro-optimisation
  (removes the compiler's `callvalue` check). Nothing consumes sent ETH.

#### `fallback() external payable` — `:32`

The heart of the proxy. Worth reading line by line:

```solidity
address facet = ds.selectorToFacetAndPosition[msg.sig].facetAddress;   // :43
if (facet == address(0)) revert LibDiamond.FunctionDoesNotExist();     // :45-47
assembly {
    calldatacopy(0, 0, calldatasize())                                  // :53
    let result := delegatecall(gas(), facet, 0, calldatasize(), 0, 0)   // :55
    returndatacopy(0, 0, returndatasize())                              // :57
    switch result
    case 0 { revert(0, returndatasize()) }                              // :61
    default { return(0, returndatasize()) }                             // :64
}
```

- **Lookup.** `msg.sig` is the first four calldata bytes. One `SLOAD` from the
  diamond-storage struct.
- **Unknown selector** reverts `FunctionDoesNotExist()` (`0xa9ad62f8`). This is
  also what a *paused* diamond looks like from outside: `EmergencyPauseFacet`
  pauses by **removing** selectors, so a paused diamond answers every call with
  this error, not `DiamondIsPaused()`.
- **The assembly.** Copies calldata to memory offset 0 (scratch space — safe
  because we never return to Solidity), forwards **all** remaining gas,
  `delegatecall`s, then copies returndata to offset 0 and either `revert`s or
  `return`s it verbatim. Bubbling raw returndata is what lets facet custom errors
  reach the caller intact.
- **Gotcha.** `delegatecall` means the facet's `address(this)` is the diamond and
  its `msg.sender` is the original caller. A facet that declares state variables
  would collide with diamond storage — hence the namespaced-slot discipline.

#### `receive() external payable` — `:71`

Empty. Lets the diamond accept plain ETH (refunds from WETH `withdraw`, bridge
refunds). Without it, a bare-value transfer would hit `fallback()` with
`msg.sig == 0x00000000` and revert `FunctionDoesNotExist()`.

### 2.2 `LibDiamond.sol`

**Library** `LibDiamond` (`src/Libraries/LibDiamond.sol:11`), version `1.0.0`.

#### Storage

```solidity
bytes32 internal constant DIAMOND_STORAGE_POSITION =
    keccak256("diamond.standard.diamond.storage");   // :12-13
```

= **`0xc8fcad8db84d3cc18b4c41d551ea0ee66dd599cde068d998e57d5e09332c131c`**

```solidity
struct DiamondStorage {                              // :39-52
    mapping(bytes4 => FacetAddressAndPosition) selectorToFacetAndPosition;  // slot+0
    mapping(address => FacetFunctionSelectors) facetFunctionSelectors;      // slot+1
    address[] facetAddresses;                                               // slot+2
    mapping(bytes4 => bool) supportedInterfaces;                            // slot+3 (ERC-165)
    address contractOwner;                                                  // slot+4
}
```

`FacetAddressAndPosition` (`:29-32`) packs `address facetAddress` + `uint96
functionSelectorPosition` into **one** slot (160 + 96 = 256 bits) — the reason
the position is `uint96` and not `uint256`.

`FacetFunctionSelectors` (`:34-37`) is `bytes4[] functionSelectors` plus
`uint256 facetAddressPosition` (index into `facetAddresses`).

`enum FacetCutAction { Add, Replace, Remove }` (`:54-58`) → `0, 1, 2`.
`struct FacetCut { address facetAddress; FacetCutAction action; bytes4[] functionSelectors; }` (`:61-65`).

#### Errors (`:16-26`)

| Selector | Error | Thrown at | Cause |
|---|---|---|---|
| `0xe548e6b5` | `IncorrectFacetCutAction()` | `:126` | `action` outside 0–2 (unreachable via ABI) |
| `0x7bc55950` | `NoSelectorsInFace()` | `:141`, `:179`, `:218` | empty `functionSelectors` array (note the typo in the name — it is `Face`, not `Facet`) |
| `0xa023275d` | `FunctionAlreadyExists()` | `:164`, `:202` | Add of an already-registered selector; Replace onto the same facet |
| `0xc68ec83a` | `FacetAddressIsZero()` | `:145`, `:183` | Add/Replace with `facetAddress == 0` |
| `0x79c9df22` | `FacetAddressIsNotZero()` | `:223` | Remove with a non-zero `facetAddress` |
| `0xe3500600` | `FacetContainsNoCode()` | `:361` | target has `extcodesize == 0` |
| `0xa9ad62f8` | `FunctionDoesNotExist()` | `:273`, and the diamond fallback | removing/ calling an unregistered selector |
| `0xc3c5ec37` | `FunctionIsImmutable()` | `:277` | removing a function defined directly on the diamond |
| `0x98116860` | `InitZeroButCalldataNotEmpty()` | `:332` | `_init == 0` but `_calldata` non-empty |
| `0x42200566` | `CalldataEmptyButInitNotZero()` | `:336` | `_init != 0` but `_calldata` empty |
| `0xc53ebed5` | `InitReverted()` | `:348` | init `delegatecall` failed with no returndata |

#### `diamondStorage() internal pure returns (DiamondStorage storage ds)` — `:67`

Assigns `ds.slot := DIAMOND_STORAGE_POSITION` in assembly (`:74-76`). Pure by the
usual storage-pointer trick: it computes an address, it does not read.

#### `setContractOwner(address _newOwner) internal` — `:86`

Writes `ds.contractOwner`, emits `OwnershipTransferred(previousOwner, _newOwner)`
(`:79-82`, `:90`). **No access control of its own** — every caller must gate it.
Callers: the diamond constructor (`src/LiFiDiamond.sol:15`) and `OwnershipFacet`.

#### `contractOwner() internal view returns (address)` — `:93`
Single `SLOAD`.

#### `enforceIsContractOwner() internal view` — `:97`
Reverts `OnlyContractOwner()` (`0x277d76f8`, from `GenericErrors`) unless
`msg.sender == ds.contractOwner`. This is the gate on every privileged facet
function that is not delegated to `LibAccess`.

#### `diamondCut(FacetCut[] memory _diamondCut, address _init, bytes memory _calldata) internal` — `:103`

- **Purpose.** Apply a batch of add/replace/remove operations, then optionally run
  an initializer by `delegatecall`.
- **Flow.** Loops the array (`:108-131`), dispatching on `action` to
  `addFunctions` / `replaceFunctions` / `removeFunctions`, else reverts
  `IncorrectFacetCutAction()`. Then emits `DiamondCut` (`:132`) and calls
  `initializeDiamondCut` (`:133`).
- **Ordering gotcha.** The event fires **before** the initializer runs. If the
  init reverts the whole tx reverts, so the ordering is only observable in traces.
- **No access control here either.** `DiamondCutFacet` supplies
  `enforceIsContractOwner()`. In production the owner is the
  `LiFiTimelockController` (§9).

#### `addFunctions(address _facetAddress, bytes4[] memory _functionSelectors) internal` — `:136`

- **Checks.** empty array → `NoSelectorsInFace()`; zero facet → `FacetAddressIsZero()`;
  per selector, already-registered → `FunctionAlreadyExists()`.
- **First-selector detection** (`:147-153`): `selectorPosition` is the current
  length of the facet's selector array; if `0`, the facet is new and `addFacet`
  runs (which is where `enforceHasContractCode` fires).
- **Loop** (`:154-171`) calls `addFunction` per selector, incrementing
  `selectorPosition` unchecked.
- **Gotcha.** "Length is 0 ⇒ new facet" is subtly wrong if a facet is fully
  removed and re-added — but `removeFunction` also pops the facet address, so the
  two stay consistent.

#### `replaceFunctions(...) internal` — `:174`

Same shape as `addFunctions`, but per selector it reverts `FunctionAlreadyExists()`
when the selector already points at *this* facet (`:201-203`), then
`removeFunction(old)` + `addFunction(new)` (`:204-205`). Replacing a
**non-existent** selector reverts `FunctionDoesNotExist()` from inside
`removeFunction` (`:273`).

#### `removeFunctions(address _facetAddress, bytes4[] memory _functionSelectors) internal` — `:213`

- **The counter-intuitive check** (`:222-224`):

```solidity
// if function does not exist then do nothing and return
if (!LibUtil.isZeroAddress(_facetAddress)) {
    revert FacetAddressIsNotZero();
}
```

  Removal requires `_facetAddress == address(0)`. This is the EIP-2535 reference
  convention: on a Remove cut the facet address field is unused and must be zero.
  The comment above it is stale and describes different behaviour than the code.
- Each selector is resolved to its *current* facet from storage (`:231-233`) and
  passed to `removeFunction`.

#### `addFacet(DiamondStorage storage ds, address _facetAddress) internal` — `:241`
`enforceHasContractCode`, then record `facetAddressPosition = facetAddresses.length`
and push. Two writes.

#### `addFunction(ds, bytes4 _selector, uint96 _selectorPosition, address _facetAddress) internal` — `:252`
Three writes: the selector's position, the push onto the facet's selector array,
and the selector→facet mapping.

#### `removeFunction(ds, address _facetAddress, bytes4 _selector) internal` — `:267`

The most intricate function in the library — a swap-and-pop on two levels.

- **Checks.** zero facet → `FunctionDoesNotExist()` (`:272-274`);
  `_facetAddress == address(this)` → `FunctionIsImmutable()` (`:276-278`). The
  second guards functions defined directly on `LiFiDiamond` itself.
- **Level 1 — selector array** (`:280-301`). Read the selector's position and the
  last index. If not last, move the last selector into the hole and fix *its*
  recorded position. `pop()`, then `delete ds.selectorToFacetAndPosition[_selector]`.
- **Level 2 — facet array** (`:304-323`). Guarded by `if (lastSelectorPosition == 0)`,
  i.e. the array had exactly one element, so this was the facet's last selector.
  Same swap-and-pop over `facetAddresses`, then delete the position record.
- **Gotcha.** Both levels reorder arrays. Never cache a loupe index across a cut.
  This is also why `DiamondLoupeFacet.facets()` output ordering is unstable.

#### `initializeDiamondCut(address _init, bytes memory _calldata) internal` — `:326`

- `_init == 0`: `_calldata` must be empty, else `InitZeroButCalldataNotEmpty()`.
- `_init != 0`: `_calldata` must be non-empty, else `CalldataEmptyButInitNotZero()`;
  `enforceHasContractCode(_init)` unless `_init == address(this)`;
  then **`_init.delegatecall(_calldata)`** (`:342`) — untrusted, runs in the
  diamond's storage context with full power. On failure it bubbles the revert
  string via `revert(string(error))` (`:346`), else `InitReverted()`.
- **Gotcha.** `revert(string(error))` re-wraps the bubbled bytes as an `Error(string)`,
  which mangles custom errors from the initializer. Read raw traces when an init fails.
- **Security.** This is the single most dangerous call in the codebase: an
  arbitrary `delegatecall` from the diamond. It is why the owner is a timelock.

#### `enforceHasContractCode(address _contract) internal view` — `:354`
`extcodesize` check, reverts `FacetContainsNoCode()`. Note the standard caveat:
during a constructor `extcodesize` is 0, and a self-destructed contract passes as
an EOA.

#### Operational notes from the docs

- **`docs/DiamondCutRecomputation.md`** — cut calldata is *not* a pure function of
  the repo: `buildDiamondCut` reads the live diamond through the loupe to decide
  Add vs Replace vs Remove. Verifying a pending Safe proposal therefore requires
  pinning `DIAMOND_STATE_BLOCK` and overriding facet/diamond addresses, else the
  script reverts `DiamondStateNotPinned`.
- **`docs/FacetRemovalReconciliation.md`** — deprecating a facet in the repo does
  **not** remove it from the ~71 deployed diamonds; selectors stay callable
  forever until an explicit removal cut. Once source is deleted the build artifact
  is gone, so the on-chain loupe becomes the only selector source.
- **`docs/DeferredDiamondCleanupQueue.md`** — the scheduling layer that parks such
  removals and drains them opportunistically. Status in-repo: partially built.

### 2.3 `LibDiamondLoupe.sol`

**Library** (`src/Libraries/LibDiamondLoupe.sol:9`), version `1.0.0`. Exists so a
facet can read the loupe **without** an external call back into the diamond.
Four functions, all `internal view`, all thin reads of `LibDiamond.diamondStorage()`:

| Function | Line | Returns |
|---|---|---|
| `facets()` | `:10` | `IDiamondLoupe.Facet[]` — loops `facetAddresses`, copying each selector array |
| `facetFunctionSelectors(address _facet)` | `:30` | that facet's `bytes4[]` |
| `facetAddresses()` | `:39` | the whole `address[]` |
| `facetAddress(bytes4 _functionSelector)` | `:48` | owning facet, or `address(0)` |

**Gotcha.** `facets()` is O(facets × selectors) and copies every array to memory.
On a diamond with 40+ facets this is an off-chain-only call.

### 2.4 A note on `LibStorage`

`docs/LibStorage.md` documents an AppStorage-pattern shared-storage layout, but
**there is no `src/Libraries/LibStorage.sol` in this checkout** — the directory
contains exactly nine files (see [§1](#1-file-inventory)). The pattern it
describes is realised instead by each library owning its own namespaced struct,
enumerated next.

---

## 3. Namespaced storage: the slot table

Every piece of diamond state lives at a `keccak256`-derived constant slot, chosen
so that structs cannot collide. These are **not** hashed-then-offset the way
ERC-7201 does it; the constant *is* the struct's base slot, and fields sit at
`base + n`.

| Namespace string | Slot | Struct | Declared at |
|---|---|---|---|
| `diamond.standard.diamond.storage` | `0xc8fcad8db84d3cc18b4c41d551ea0ee66dd599cde068d998e57d5e09332c131c` | `LibDiamond.DiamondStorage` (5 fields) | `src/Libraries/LibDiamond.sol:12` |
| `com.lifi.library.access.management` | `0xdf05114fe8fad5d7cd2d71c5651effc2a4c21f13ee8b4a462e2a3bd4e140c73e` | `LibAccess.AccessStorage` (1 field) | `src/Libraries/LibAccess.sol:12` |
| `com.lifi.library.allow.list` | `0x7a8ac5d3b7183f220a0602439da45ea337311d699902d1ed11a3725a714e7f1e` | `LibAllowList.AllowListStorage` (10 fields) | `src/Libraries/LibAllowList.sol:28` |
| `com.lifi.reentrancyguard` | `0xa65bb2f450488ab0858c00edc14abc5297769bf42adb48cfb77752890e8b697b` | `ReentrancyGuard.ReentrancyStorage` (1 field) | `src/Helpers/ReentrancyGuard.sol:11` |

Reproduce any of these with `cast keccak "com.lifi.library.allow.list"`.

**The rule this imposes on facets:** a facet may not declare state variables.
Anything it needs must go through one of these libraries, or a new namespace. The
`AllowListStorage` struct in particular is **append-only** — its first six fields
are kept purely for storage-layout compatibility with already-deployed diamonds
(`src/Libraries/LibAllowList.sol:32-51`), and reordering them would corrupt every
live deployment.

---

## 4. The two permission systems

They are independent and answer different questions.

```
   LibAllowList  ->  "may the diamond CALL this (contract, selector)?"     outbound
   LibAccess     ->  "may this CALLER invoke this selector on the diamond?" inbound
```

A swap needs the first. `WithdrawFacet.executeCallAndWithdraw` needs the second.
Neither implies the other.

### 4.1 `LibAllowList.sol` — what may be called

**Library** (`src/Libraries/LibAllowList.sol:25`), version `2.0.0`. This is the
single control that makes "call an arbitrary contract with arbitrary calldata"
safe. Without it, `LibSwap.swap` would be a universal fund drain.

#### The dual model

V1 (deployed, still read by old facets) asked two independent questions: "is this
contract allowed?" and "is this selector allowed?". That is weaker than it looks
— allowing `transferFrom` for *any* whitelisted DEX allows it for *all* of them.
V2 stores the **pair**. Both are maintained simultaneously, with reference counting
to keep the V1 arrays consistent.

#### Storage — `AllowListStorage` (`:31-63`)

| # | Field | Model | Meaning |
|---:|---|---|---|
| 0 | `mapping(address => bool) contractAllowList` | V1 | legacy per-contract flag |
| 1 | `mapping(bytes4 => bool) selectorAllowList` | V1 | legacy per-selector flag |
| 2 | `address[] contracts` | V1 | iterable contract list |
| 3 | `mapping(address => uint256) contractToIndex` | V1 | **1-based** index into `contracts` |
| 4 | `mapping(bytes4 => uint256) selectorToIndex` | V1 | 1-based index into `selectors` |
| 5 | `bytes4[] selectors` | V1 | iterable selector list |
| 6 | `mapping(address => mapping(bytes4 => bool)) contractSelectorAllowList` | **V2 — source of truth** | the pair |
| 7 | `mapping(bytes4 => uint256) selectorReferenceCount` | sync | how many contracts use this selector |
| 8 | `mapping(address => bytes4[]) whitelistedSelectorsByContract` | V2 | iterable; **its length is the contract's implicit refcount** |
| 9 | `mapping(address => mapping(bytes4 => uint256)) selectorIndices` | V2 | 1-based index into field 8 |
| 10 | `bool migrated` | one-shot | migration completion flag |

**1-based indices everywhere.** `0` means "absent", so a plain `mapping` default
distinguishes missing from index-0.

#### The `0xffffffff` approve-only selector

Documented at `:16-23`. Some DEXes take the approval on one address (a
`Permit2`-style spender or a vault) and the swap call on another (the router).
Whitelisting the spender with the sentinel selector `0xffffffff` makes
`contractIsAllowed(spender)` true — satisfying V1 consumers — **without**
authorising any call to it in the V2 model, because `0xffffffff` is not a real
selector any function will ever have. `SwapperV2` enforces exactly this
(`src/Helpers/SwapperV2.sol:25`, `:209-214`).

#### `addAllowedContractSelector(address _contract, bytes4 _selector) internal` — `:70`

- **Checks.** `_contract == 0 || _selector == 0` → `InvalidCallData()` (`:74-75`).
  Idempotent: returns silently if the pair is already allowed (`:79`).
- **State writes, in order** (`:82-100`):
  1. `contractSelectorAllowList[c][s] = true` — source of truth.
  2. If `whitelistedSelectorsByContract[c].length == 0`, call `_addAllowedContract(c)`
     — this is where `InvalidContract()` can fire.
  3. `++selectorReferenceCount[s]`; if it becomes 1, `_addAllowedSelector(s)`.
  4. Push onto `whitelistedSelectorsByContract[c]` and record the 1-based index.
- **Callers.** `WhitelistManagerFacet.sol:112`.
- **Gotcha.** Step 2 runs *before* the push in step 4, so the length check is
  correct. Reordering them would break the refcount.

#### `removeAllowedContractSelector(address _contract, bytes4 _selector) internal` — `:108`

Mirror image. Idempotent (`:114`). Order matters and is commented as such
(`:119-131`): delete from the source of truth, remove from the iterable list
**first** so the length is fresh, then drop the contract if the list is now
empty, then decrement the global refcount and drop the selector if it hits zero.
Callers: `WhitelistManagerFacet.sol:114`.

#### `contractSelectorIsAllowed(address, bytes4) internal view returns (bool)` — `:140`

**The function that matters at runtime.** One `SLOAD`. Called from
`SwapperV2.sol:205`, `:211`, `:252`, `:258`, `GenericSwapFacetV3.sol:164`, `:359`,
`:368`, `:420`, `:431`, and `WhitelistManagerFacet.sol:58`, `:103`.

#### `getWhitelistedSelectorsForContract(address) internal view returns (bytes4[])` — `:151`
Backend-sync getter. Caller: `WhitelistManagerFacet.sol:65`, `:82`.

#### Backward-compatibility reads

| Function | Line | Note |
|---|---|---|
| `contractIsAllowed(address)` | `:168` | V1 boolean. **Not granular** — true if *any* selector (or `0xffffffff`) is whitelisted for it |
| `selectorIsAllowed(bytes4)` | `:179` | V1 boolean, global across all contracts |
| `getAllowedContracts()` | `:186` | whole `contracts` array; caller `WhitelistManagerFacet.sol:75` |
| `getAllowedSelectors()` | `:192` | whole `selectors` array |

#### Private sync helpers

- **`_addAllowedContract(address)` — `:200`.** `LibAsset.isContract` check →
  `InvalidContract()` (`:202`). Sets the V1 bool, then pushes to `contracts` with
  a 1-based index unless already present.
- **`_removeAllowedContract(address)` — `:219`.** Deletes the V1 bool
  **first**, deliberately (comment at `:222-231`), so an admin can repair a stale
  V1 flag by add-then-remove even when the V2 index is 0. Then swap-and-pop.
- **`_addAllowedSelector(bytes4)` — `:258`** / **`_removeAllowedSelector(bytes4)` — `:276`.**
  Same pattern for the global selector list; the `delete` before the index check
  (`:285`) exists because the migration's selector cleanup relied on an off-chain
  list and can leave stale V1 flags.
- **`_removeSelectorFromIterableList(address, bytes4)` — `:313`.** Swap-and-pop on
  `whitelistedSelectorsByContract[c]`, fixing the moved element's index.
- **`_getStorage()` — `:345`.** Note this one is `internal`, not `private`, unlike
  its siblings.

**Security summary.** Whitelisting a `(contract, selector)` pair is granting the
diamond's entire token balance to that pair. `transferFrom` on a token, or any
`execute`-style multicall router, must never be whitelisted.

### 4.2 `LibAccess.sol` — who may call

**Library** (`src/Libraries/LibAccess.sol:10`), version `1.0.0`. 65 lines.

Storage: `struct AccessStorage { mapping(bytes4 => mapping(address => bool)) execAccess; }`
(`:16-18`) at the slot in [§3](#3-namespaced-storage-the-slot-table).

Events: `AccessGranted(address indexed account, bytes4 indexed method)` (`:21`,
topic0 `0xcdd2f8ab…`), `AccessRevoked(...)` (`:22`, topic0 `0x4e2965fe…`).

| Function | Line | Behaviour |
|---|---|---|
| `accessStorage()` | `:25` | slot pointer |
| `addAccess(bytes4 selector, address executor)` | `:40` | reverts `CannotAuthoriseSelf()` (`0xa9cefcae`) if `executor == address(this)`; sets flag; emits `AccessGranted` |
| `removeAccess(bytes4 selector, address executor)` | `:52` | clears flag; emits `AccessRevoked`. No self-check needed |
| `enforceAccessControl()` | `:60` | reverts `UnAuthorized()` (`0xbe245983`) unless `execAccess[msg.sig][msg.sender]` |

**`CannotAuthoriseSelf` matters.** Authorising the diamond itself would let any
facet's internal `address(this)` call pass the gate, turning an arbitrary-call
primitive into privilege escalation.

**Callers of `enforceAccessControl()`:** `src/Facets/WithdrawFacet.sol` and
`src/Facets/WhitelistManagerFacet.sol` (grep across `src/` finds only these two,
plus the definition). Both also accept the contract owner as an alternative.

**Gotcha.** `enforceAccessControl` keys on `msg.sig`, so it only works in a
function reached through the diamond's fallback. It is meaningless in an internal
helper called from a differently-named entry point.

---

## 5. Asset and swap primitives

### 5.1 `LibAsset.sol`

**Library** (`src/Libraries/LibAsset.sol:17`), version `2.1.3`. Uses
`solady/utils/SafeTransferLib` for every actual movement.

**Constants.**
- `address internal constant NULL_ADDRESS = address(0)` (`:23`) — the native-asset id.
- `bytes3 internal constant DELEGATION_DESIGNATOR = 0xef0100` (`:26`) — the EIP-7702
  delegation prefix. Declared for documentation; `isContract` uses the length, not the prefix.

#### `getOwnBalance(address assetId) internal view returns (uint256)` — `:31`
Native → `address(this).balance`; ERC20 → `assetId.balanceOf(address(this))`.
The workhorse behind every balance-difference computation in the codebase.

#### `transferAsset(address assetId, address payable recipient, uint256 amount) internal` — `:45`
Dispatches to `transferNativeAsset` or `transferERC20`.

#### `transferNativeAsset(address payable recipient, uint256 amount) internal` — `:61`
Reverts `InvalidReceiver()` (`0x1e4ec46b`) on zero recipient (`:66`), then
`recipient.safeTransferETH(amount)`. Solady forwards all gas and reverts
`ETHTransferFailed` on failure. **Gotcha:** all-gas forwarding means the recipient
can re-enter — see `FeeForwarder` (§12.2) for a contract that had to reason about
exactly this.

#### `transferERC20(address assetId, address recipient, uint256 amount) internal` — `:76`
Zero-recipient check, then `safeTransfer`. Handles non-compliant tokens that
return nothing.

#### `transferFromERC20(address assetId, address from, address recipient, uint256 amount) internal` — `:95`
Reverts `NullAddrIsNotAnERC20Token()` (`0xd1bebf0c`) if `assetId` is native
(`:102-104`) and `InvalidReceiver()` on zero recipient, then `safeTransferFrom`.
Callers include `ERC20Proxy.sol:61`, `FeeForwarder.sol:68`, `Permit2Proxy.sol:114`,
`Patcher.sol:279`, `OutputValidator.sol:136`.

#### `depositAsset(address assetId, uint256 amount) internal` — `:118`
- `amount == 0` → `InvalidAmount()` (`0x2c5211c6`).
- Native: requires `msg.value >= amount`, else `InvalidAmount()`. **Note it is
  `>=`, not `==`** — excess `msg.value` is not rejected here; the caller's
  `refundExcessNative` modifier is what returns it.
- ERC20: `safeTransferFrom(msg.sender, address(this), amount)`.

#### `depositAssets(LibSwap.SwapData[] calldata swaps) internal` — `:132`
Loops and calls `depositAsset` for entries with `requiresDeposit == true`. That
flag is what distinguishes "pull this from the user" (first hop) from "we already
hold it" (later hops).

#### `maxApproveERC20(IERC20 assetId, address spender, uint256 amount) internal` — `:149`
Thin wrapper: `approveERC20(assetId, spender, amount, type(uint256).max)`.

#### `approveERC20(IERC20 assetId, address spender, uint256 requiredAllowance, uint256 setAllowanceTo) internal` — `:163`
- Native → no-op return (`:169-171`).
- Zero spender → `NullAddrIsNotAValidSpender()` (`0x63ba9bff`).
- **Only if** the current allowance is insufficient, call
  `safeApproveWithRetry(spender, setAllowanceTo)` (`:182-184`). Solady's retry
  handles USDT-style tokens that revert on a non-zero→non-zero approve by zeroing
  first.
- **Gotcha.** The default is an *infinite* approval that is never revoked. The
  diamond is designed to hold no balance between transactions, so the residual
  approval is only as dangerous as the whitelist that guards the spender.

#### `isNativeAsset(address assetId) internal pure returns (bool)` — `:190`
`assetId == address(0)`.

#### `isContract(address account) internal view returns (bool)` — `:200`

```solidity
uint256 size;
assembly { size := extcodesize(account) }
return size > 23;                                  // :208
```

**Not** the usual `size > 0`. An EIP-7702 delegated EOA has exactly 23 bytes of
code (`0xef0100` + 20-byte address), and must still be treated as an EOA. The
documented limitation (`:197-198`) is that a self-destructed contract is
indistinguishable from an EOA. Callers: `LibSwap.sol:53`, `LibAllowList.sol:202`,
`TokenWrapper.sol:47`, `:53`.

### 5.2 `LibSwap.sol`

**Library** (`src/Libraries/LibSwap.sol:14`), version `1.1.0`. 109 lines, one
function — and it is the most powerful primitive in the protocol.

#### `struct SwapData` (`:23-31`)

| Field | Type | Meaning |
|---|---|---|
| `callTo` | `address` | contract to call (the DEX router) |
| `approveTo` | `address` | who gets the token approval; may differ from `callTo` |
| `sendingAssetId` | `address` | input token, `address(0)` for native |
| `receivingAssetId` | `address` | expected output token |
| `fromAmount` | `uint256` | exact input amount |
| `callData` | `bytes` | the encoded call |
| `requiresDeposit` | `bool` | pull from `msg.sender` first? |

#### `event AssetSwapped(...)` (`:41-49`)
Seven non-indexed fields; topic0 `0x7bfdfdb5e3a3776976e53cb0607060f54c5312701c8cba1155cc4d5394440b38`.
Nothing is indexed, so indexers must decode data rather than filter on topics.

#### `swap(bytes32 transactionId, SwapData calldata _swap) internal` — `:51`

1. **`callTo` must be a contract** (`:53`) → `InvalidContract()` (`0x6eefed20`).
   Uses `isContract`, so a 7702-delegated EOA is rejected.
2. **`fromAmount != 0`** (`:57`) → `NoSwapFromZeroBalance()` (`0xe46e079c`).
3. **Native value** (`:60-62`): `nativeValue = isNativeAsset(sendingAssetId) ? fromAmount : 0`.
4. **Snapshot** `initialReceivingAssetBalance` (`:65-67`).
5. **Approve** (`:70-76`) only when `nativeValue == 0`: `maxApproveERC20(sendingAssetId, approveTo, fromAmount)`.
6. **The call** (`:86-88`): `_swap.callTo.call{value: nativeValue}(_swap.callData)`
   — **fully untrusted**, arbitrary target and arbitrary calldata. On failure,
   `LibUtil.revertWith(res)` bubbles the raw returndata.
7. **Post-balance** and `emit AssetSwapped` (`:94-107`).

**The removed check.** Lines `:78-82` document that a
`initialSendingAssetBalance >= fromAmount` check used to exist and was deleted to
support rebasing and fee-taking tokens. The reasoning: if the funds are not there
the DEX call reverts anyway, just with a less legible message.

**The event's `toAmount` quirk** (`:103-105`):

```solidity
newBalance > initialReceivingAssetBalance
    ? newBalance - initialReceivingAssetBalance
    : newBalance
```

If the balance did **not** increase, the event reports the absolute post-balance
rather than a delta. Indexers must not assume this field is always a delta.

**Why this is safe.** Only because every caller checks `LibAllowList` first.
`swap` itself performs no authorisation. Callers, all three of them:
`Executor.sol:229`, `SwapperV2.sol:217`, `SwapperV2.sol:264`.

### 5.3 `LibBytes.sol`

**Library** (`src/Libraries/LibBytes.sol:5`), version `1.1.0`.

Errors: `SliceOverflow()` `0x47aaf07a`, `SliceOutOfBounds()` `0x3b99b53d`,
`AddressOutOfBounds()` `0x8f95a28a`, `HexLengthInsufficient()` `0x2194895a`,
`NotAnAddress(bytes32)` `0x479ef3f7`. Constant `_SYMBOLS = "0123456789abcdef"` (`:15`).

| Function | Line | Notes |
|---|---|---|
| `slice(bytes memory, uint256 _start, uint256 _length)` | `:19` | Classic `BytesLib` assembly copy. Guards `_length + 31 < _length` (overflow) and `_bytes.length < _start + _length`. Handles the `lengthmod == 0` case with the `mul(0x20, iszero(lengthmod))` trick (`:50-53`), and returns a zero-length array without touching memory beyond the free pointer (`:80-87`) |
| `toAddress(bytes memory, uint256 _start)` | `:93` | Bounds-checked (`:97`), then `div(mload(...), 0x1000000000000000000000000)` to take the high 20 bytes |
| `toHexString(uint256 value, uint256 length)` | `:115` | OpenZeppelin `Strings` port; reverts `HexLengthInsufficient()` if the value did not fit |
| `toBytes32(address)` | `:133` | Left-zero-pad. Always lossless |
| `toAddress(bytes32 _value)` | `:141` | **Checked** downcast: reverts `NotAnAddress(_value)` if any of the top 96 bits are set |
| `toAddressUnchecked(bytes32 _value)` | `:150` | Truncating downcast. Only where dropping high bits is intentional |

The checked/unchecked `bytes32 → address` pair was added in v1.1.0 for non-EVM
address handling. `ReceiverOIF.sol:93-94` carries a TODO to migrate to the
checked variant.

### 5.4 `LibUtil.sol`

**Library** (`src/Libraries/LibUtil.sol:8`), version `1.0.0`. Three functions.

- **`getRevertMsg(bytes memory _res) internal pure returns (string memory)` — `:11`.**
  Returns `"Transaction reverted silently"` if `_res.length < 68`, else strips the
  4-byte selector and ABI-decodes a string. Only valid for `Error(string)`
  reverts; a custom error decodes to garbage or reverts.
- **`isZeroAddress(address addr) internal pure returns (bool)` — `:23`.** Used
  throughout `LibDiamond` and `Patcher`.
- **`revertWith(bytes memory data) internal pure` — `:27`.** Assembly
  `revert(dataPtr, dataSize)`, re-throwing captured returndata **verbatim**. This
  is what preserves a DEX's custom error through `LibSwap.swap`. Callers:
  `LibSwap.sol:90`, `Patcher.sol:357`, `GasZipPeriphery.sol:106`, `Permit2Proxy.sol:109`.
  Marked `pure` while reverting — legal, and a common idiom.

### 5.5 `OFTComposeMsgCodec.sol`

**Library** (`src/Libraries/OFTComposeMsgCodec.sol:10`), version `1.0.0`.
Vendored verbatim from LayerZero v2 with only the pragma changed (`:4-8`).

Byte layout of an OFT compose message:

```
 offset  0        8         12                      44                      76        end
         |--------|---------|-----------------------|-----------------------|---------|
          nonce     srcEid    amountLD                composeFrom             composeMsg
          uint64    uint32    uint256                 bytes32                 bytes
          8 bytes   4 bytes   32 bytes                32 bytes                variable
```

Offsets are the private constants at `:12-15`.

| Function | Line | Returns |
|---|---|---|
| `encode(uint64, uint32, uint256, bytes)` | `:25` | `abi.encodePacked` of the above |
| `nonce(bytes calldata)` | `:39` | `[0:8]` |
| `srcEid(bytes calldata)` | `:48` | `[8:12]` |
| `amountLD(bytes calldata)` | `:57` | `[12:44]` — the amount actually delivered |
| `composeFrom(bytes calldata)` | `:66` | `[44:76]` |
| `composeMsg(bytes calldata)` | `:75` | `[76:]` — the LI.FI payload |
| `addressToBytes32(address)` / `bytes32ToAddress(bytes32)` | `:86` / `:95` | unchecked casts |

Only `ReceiverStargateV2` uses it (`src/Periphery/ReceiverStargateV2.sol:121`, `:131`).
Using `amountLD` from the message rather than a balance read is deliberate: the
receiver may already hold dust of the same token.

---

## 6. Helpers

### 6.1 `SwapperV2.sol`

**Contract** `SwapperV2 is ILiFi` (`src/Helpers/SwapperV2.sol:14`), version
`1.2.0`. Inherited by **32 facets** (`grep -l _depositAndSwap src/Facets/ | wc -l`).
This is the subtlest contract in the repository and the one most worth reading
slowly.

Constant: `bytes4 internal constant APPROVE_TO_ONLY_SELECTOR = 0xffffffff` (`:25`).
Struct `ReserveData { bytes32 transactionId; address payable leftoverReceiver; uint256 nativeReserve; }`
(`:18-22`) — exists purely to dodge stack-too-deep.

#### Modifiers

| Modifier | Line | Behaviour |
|---|---|---|
| `noLeftovers(swaps, leftoverReceiver, initialBalances)` | `:34` | runs body, then `_refundLeftovers(..., 0)` |
| `noLeftoversReserve(swaps, leftoverReceiver, initialBalances, nativeReserve)` | `:49` | same, with a native reserve withheld |
| `refundExcessNative(address payable _refundReceiver)` | `:67` | snapshots `address(this).balance - msg.value` **before** the body, and after it refunds any increase |

`refundExcessNative` deserves a note. The pre-balance subtracts `msg.value`
because at modifier-entry the value has already been credited. Any *increase*
over that baseline after the body — a bridge refunding unused native fee, say —
goes to the refund receiver. A pre-existing stray balance is untouched.

#### `_depositAndSwap(bytes32, uint256 _minAmount, SwapData[] calldata, address payable) internal returns (uint256)` — `:89`

The four-argument overload. Sequence:

1. `numSwaps == 0` → `NoSwapDataProvided()` (`0x0503c3ed`) (`:97-99`).
2. `finalTokenId` = the **last** swap's `receivingAssetId` (`:101`).
3. `initialBalance` of that token; if native, subtract `msg.value` (`:102-106`) so
   the user's own ETH is not counted as swap output.
4. `initialBalances = _fetchBalances(_swaps)` (`:108`) — per-hop snapshots.
5. `LibAsset.depositAssets(_swaps)` (`:110`) — pull everything flagged.
6. `_executeSwaps(...)` (`:112-117`) — runs the hops and, on exit, refunds leftovers.
7. `newBalance = getOwnBalance(finalTokenId) - initialBalance` (`:119-120`).
8. `newBalance < _minAmount` → **`CumulativeSlippageTooHigh(_minAmount, newBalance)`**
   (`0x275c273c`) (`:122-124`).

**This is the protocol's only end-to-end slippage guarantee.** Per-hop minimums
live inside each DEX's calldata, but the cumulative check here is what the user is
actually protected by.

#### `_depositAndSwap(..., uint256 _nativeReserve) internal returns (uint256)` — `:135`

Five-argument overload, for bridges that need native gas for a destination fee.
Identical through step 6, but builds a `ReserveData` (`:159-163`), calls the
reserve-aware `_executeSwaps`, and then at `:170-172` **subtracts the reserve from
the measured output** before the slippage check. Without that subtraction, native
set aside for the bridge fee would be counted as user output.

#### `_executeSwaps(bytes32, SwapData[] calldata, address payable, uint256[] memory) internal` — `:190`

Carries `noLeftovers`. Per hop, the whitelist check (`:204-215`):

```solidity
if (
    !LibAllowList.contractSelectorIsAllowed(callTo, bytes4(currentSwap.callData[:4])) ||
    (!LibAsset.isNativeAsset(currentSwap.sendingAssetId) &&
        approveTo != callTo &&
        !LibAllowList.contractSelectorIsAllowed(approveTo, APPROVE_TO_ONLY_SELECTOR))
) revert ContractCallNotAllowed();
```

Read it as two independent requirements:
- **The call** — `(callTo, selector)` must be whitelisted. `bytes4(callData[:4])`
  reverts on calldata shorter than 4 bytes, which is a free length check.
- **The approval** — only when sending an ERC20 *and* `approveTo != callTo`, the
  spender must additionally be whitelisted under `0xffffffff`. This closes the
  allowance-leak hole: without it, any whitelisted router could name an arbitrary
  `approveTo` and walk away with an infinite approval.

Then `LibSwap.swap(_transactionId, currentSwap)` (`:217`).

#### `_executeSwaps(ReserveData memory, SwapData[] calldata, uint256[] memory) internal` — `:230`
Byte-identical logic under `noLeftoversReserve` (`:236-241`), reading the
transaction id from the struct (`:264`).

#### `_fetchBalances(SwapData[] calldata) internal view returns (uint256[] memory)` — `:274`
One entry per hop, each the current balance of that hop's `receivingAssetId`,
minus `msg.value` for native (`:284-286`).

#### `_refundLeftovers(SwapData[] calldata, address payable, uint256[] memory, uint256) private` — `:297`

The most intricate function here. One loop, two jobs (`:314-355`).

**Job A — intermediate outputs** (`:316-334`). Only for non-final hops
(`i < numSwaps - 1 && numSwaps != 1`) whose `receivingAssetId != finalAsset`.
Leftover is `getOwnBalance(curAsset) - _initialBalances[i]`, reduced by the native
reserve if applicable, and swept if positive. This catches a middle-hop token that
a later hop did not fully consume.

**Job B — unspent inputs** (`:337-354`). For every hop's `sendingAssetId`, sweep
the **entire current balance** (minus reserve) — but *only if it is not the final
asset* (`:345-348`). The final asset must survive to be bridged.

Three things to notice:

- Job B uses the **absolute balance**, not a delta. The diamond is designed to
  hold nothing between transactions, so anything present is either unspent input
  or pre-existing dust; both go to the leftover receiver.
- The `inputAsset != finalAsset` guard is what stops the refund from stealing the
  swap output in an A→B→A route.
- The native reserve is honoured on both paths, so a withheld bridge fee is never
  refunded to the user by mistake.

### 6.2 `Validatable.sol`

**Contract** (`src/Helpers/Validatable.sol:13`), version `1.0.0`. Six modifiers,
no state. Every bridge facet composes several.

| Modifier | Line | Rejects | Error |
|---|---|---|---|
| `validateBridgeData(BridgeData)` | `:14` | zero `receiver`; `minAmount == 0`; `destinationChainId == block.chainid` | `InvalidReceiver()` / `InvalidAmount()` / `CannotBridgeToSameNetwork()` |
| `noNativeAsset(BridgeData)` | `:27` | native `sendingAssetId` | `NativeAssetNotSupported()` |
| `onlyAllowSourceToken(BridgeData, address _token)` | `:34` | `sendingAssetId != _token` | `InvalidSendingToken()` |
| `onlyAllowDestinationChain(BridgeData, uint256 _chainId)` | `:44` | wrong destination | `InvalidDestinationChain()` |
| `containsSourceSwaps(BridgeData)` | `:54` | `hasSourceSwaps == false` | `InformationMismatch()` |
| `doesNotContainSourceSwaps(BridgeData)` | `:61` | `hasSourceSwaps == true` | `InformationMismatch()` |
| `doesNotContainDestinationCalls(BridgeData)` | `:68` | `hasDestinationCall == true` | `InformationMismatch()` |

**Why the `hasSourceSwaps` flags exist.** They are *not* redundant with the
presence of swap data. They are the bridge between what the user signed / what the
API published and what the contract does, and they are the fields
`CalldataVerificationFacet` and off-chain monitoring key on. A mismatch means the
calldata was rearranged, hence `InformationMismatch()`.

**Gotcha.** `validateBridgeData` does **not** check `sendingAssetId`,
`transactionId`, or the string fields. `receiver == address(0)` is rejected, but
`NON_EVM_ADDRESS` (§6.6) is a legitimate receiver on non-EVM routes.

### 6.3 `ReentrancyGuard.sol`

**Abstract contract** (`src/Helpers/ReentrancyGuard.sol:8`), version `1.0.0`.

Diamond-storage based rather than a state variable, for the reason given in §2:
a facet cannot own storage. Namespace `com.lifi.reentrancyguard` → slot
`0xa65bb2f4…` (`:11`).

```solidity
struct ReentrancyStorage { uint256 status; }        // :15-17
uint256 private constant _NOT_ENTERED = 0;          // :25
uint256 private constant _ENTERED = 1;              // :26

modifier nonReentrant() {                           // :30
    ReentrancyStorage storage s = reentrancyStorage();
    if (s.status == _ENTERED) revert ReentrancyError();
    s.status = _ENTERED;
    _;
    s.status = _NOT_ENTERED;
}
```

Declares its own `error ReentrancyError()` (`:21`) which shadows the identical one
in `GenericErrors` — same selector `0x29f745a7`, so it is indistinguishable
on-chain.

**Gotcha.** `_NOT_ENTERED` is `0`, not `1`. That is the cheap-to-deploy choice but
the expensive-to-run one: every guarded call pays a zero→non-zero `SSTORE`
(20,000 gas) and gets a refund on the way out, rather than the 100-gas warm write
OpenZeppelin's `1`/`2` scheme achieves. Since the slot is shared across all
facets, the first guarded call in a transaction always pays full price.

**Inheritors:** 29 facets plus `Periphery/Executor.sol` (grep list in §19).

### 6.4 `TransferrableOwnership.sol`

**Contract** `TransferrableOwnership is IERC173` (`src/Helpers/TransferrableOwnership.sol:8`),
version `1.0.0`. The periphery's ownership model — note the periphery does **not**
use `LibDiamond`'s owner, because periphery contracts are standalone.

State: `address public owner` (`:9`), `address public pendingOwner` (`:10`).

Errors: `UnAuthorized()` `0xbe245983`, `NoNullOwner()` `0x1beca374`,
`NewOwnerMustNotBeSelf()` `0xbf1ea9fb`, `NoPendingOwnershipTransfer()` `0x75cdea12`,
`NotPendingOwner()` `0x1853971c`.

Event: `OwnershipTransferRequested(address indexed _from, address indexed _to)` (`:20`),
plus `OwnershipTransferred` inherited from `IERC173`.

| Function | Line | Behaviour |
|---|---|---|
| `constructor(address initialOwner)` | `:25` | sets `owner`. **No zero check** — several inheritors add their own |
| `modifier onlyOwner()` | `:29` | `UnAuthorized()` |
| `transferOwnership(address _newOwner) external onlyOwner` | `:36` | rejects zero (`NoNullOwner`) and self (`NewOwnerMustNotBeSelf`); sets `pendingOwner`; emits request |
| `cancelOwnershipTransfer() external onlyOwner` | `:44` | `NoPendingOwnershipTransfer()` if none; clears `pendingOwner` |
| `confirmOwnershipTransfer() external` | `:51` | callable **only by `pendingOwner`** (`NotPendingOwner()`); emits `OwnershipTransferred`, moves `owner`, clears `pendingOwner` |

Two-step transfer means a typo'd address cannot brick the contract — the new
owner must prove control by transacting.

### 6.5 `WithdrawablePeriphery.sol`

**Abstract contract** `WithdrawablePeriphery is TransferrableOwnership`
(`src/Helpers/WithdrawablePeriphery.sol:16`), version `1.0.0`.

Event: `TokensWithdrawn(address assetId, address payable receiver, uint256 amount)` (`:19`).

#### `withdrawToken(address assetId, address payable receiver, uint256 amount) external onlyOwner` — `:27`
Native → raw `receiver.call{value: amount}("")`, reverting `ExternalCallFailed()`
(`0x350c20f1`) on failure; ERC20 → `safeTransfer`. Emits `TokensWithdrawn`.

**The escape hatch.** Every periphery contract is designed to hold no balance
between transactions, but tokens do get stranded — a failed destination swap, a
forced transfer, dust. This is the recovery path, and its existence is why most
periphery contracts are `onlyOwner`-recoverable rather than trustless.

The `TODO(EXSC-241)` at `:9-14` is worth reading: routing through
`LibAsset.transferAsset` and adding a zero-amount check is deferred because
bumping this contract forces redeploys of every inheritor and drifts the repo from
deployed bytecode.

**Inheritors:** `Executor`, `ERC20Proxy`, all five Receivers, `FeeForwarder`,
`TokenWrapper`, `LidoWrapper`, `OutputValidator`, `GasZipPeriphery`,
`Permit2Proxy`, `LiFiDEXAggregator`. (`FeeCollector` inherits
`TransferrableOwnership` directly and implements its own withdrawals.)

### 6.6 `LiFiData.sol`

**Contract** (`src/Helpers/LiFiData.sol:8`), version `1.0.2`. Pure constants, no
functions.

```solidity
address internal constant NON_EVM_ADDRESS =
    0x11f111f111f111F111f111f111F111f111f111F1;      // :9-10
```

Placed in `BridgeData.receiver` to signal "the real recipient is in the
bridge-specific data, in a non-EVM format". The actual address is then emitted in
`BridgeToNonEVMChain` / `BridgeToNonEVMChainBytes32` (§7.1).

LI.FI-invented chain ids for non-EVM chains (`:13-22`), since these chains have no
EVM `chainId`:

| Constant | Value |
|---|---:|
| `LIFI_CHAIN_ID_APTOS` | 9271000000000010 |
| `LIFI_CHAIN_ID_BCH` | 20000000000002 |
| `LIFI_CHAIN_ID_BTC` | 20000000000001 |
| `LIFI_CHAIN_ID_DGE` | 20000000000004 |
| `LIFI_CHAIN_ID_LTC` | 20000000000003 |
| `LIFI_CHAIN_ID_SOLANA` | 1151111081099710 |
| `LIFI_CHAIN_ID_STELLAR` | 1201081091099710 |
| `LIFI_CHAIN_ID_SUI` | 9270000000000000 |
| `LIFI_CHAIN_ID_TRON` | 1885080386571452 |
| `LIFI_CHAIN_ID_HYPERCORE` | 1337 |

**Gotcha.** `LIFI_CHAIN_ID_HYPERCORE = 1337` collides with the conventional local
Ganache/Hardhat chain id. Harmless in production, confusing in tests.

---

## 7. Interfaces

### 7.1 `ILiFi.sol` — the protocol's vocabulary

**Interface** (`src/Interfaces/ILiFi.sol:7`), version `1.0.1`. Inherited by
`SwapperV2`, `Executor`, every Receiver, and every bridge facet. It defines the
one struct and the events that the entire LI.FI backend indexes on.

#### `struct BridgeData` (`:10-21`)

| # | Field | Type | Meaning and constraints |
|---:|---|---|---|
| 0 | `transactionId` | `bytes32` | LI.FI-generated correlation id. The **only** link between the source-chain and destination-chain events. Not validated on-chain — uniqueness is the backend's job |
| 1 | `bridge` | `string` | human-readable bridge name, e.g. `"across"`. Emitted only |
| 2 | `integrator` | `string` | partner identifier for attribution and fees |
| 3 | `referrer` | `address` | referral attribution; `address(0)` if none |
| 4 | `sendingAssetId` | `address` | asset leaving the source chain **after** any source swap; `address(0)` = native |
| 5 | `receiver` | `address` | destination recipient. Must be non-zero (`Validatable:15`). `LiFiData.NON_EVM_ADDRESS` for non-EVM targets |
| 6 | `minAmount` | `uint256` | with source swaps: the minimum swap output, enforced by `CumulativeSlippageTooHigh`. Without: the exact amount to bridge. Must be non-zero |
| 7 | `destinationChainId` | `uint256` | target chain; must differ from `block.chainid` |
| 8 | `hasSourceSwaps` | `bool` | must match the entry point actually used |
| 9 | `hasDestinationCall` | `bool` | whether a destination payload is attached |

**`minAmount` is mutated in place.** Facets that swap first overwrite
`bridgeData.minAmount` with the realised swap output before bridging, so the
value in the emitted event is the *actual* amount bridged, not the user's floor.

#### Events

| Event | Line | topic0 | Emitted by |
|---|---|---|---|
| `LiFiTransferStarted(BridgeData)` | `:25` | `0xcba69f43792f9f399347222505213b55af8e0b0b54b893085c2e27ecbe1644f1` | every bridge facet, source chain |
| `LiFiTransferCompleted(bytes32 indexed, address, address, uint256, uint256)` | `:27` | `0xb8c86983f929c6b770461983d1bbde1870408120f07123e9c12d49f35a0b4c4b` | `Executor._processSwaps` (`Executor.sol:204`) |
| `LiFiTransferRecovered(bytes32 indexed, address, address, uint256, uint256)` | `:35` | `0x1fbfa988fd46deed0de12c94c7b5dcb537d51b804246d0083f245f7a8997d170` | Receivers, when the destination swap failed and raw tokens were forwarded |
| `LiFiGenericSwapCompleted(bytes32 indexed, string, string, address, address, address, uint256, uint256)` | `:43` | `0x38eee76fd911eabac79da7af16053e809be0e12c8637f156e77e1af309b99537` | `GenericSwapFacetV3` (same-chain swaps) |
| `BridgeToNonEVMChain(bytes32 indexed, uint256 indexed, bytes)` | `:55` | — | facets bridging to non-EVM, address as `bytes` |
| `BridgeToNonEVMChainBytes32(bytes32 indexed, uint256 indexed, bytes32)` | `:60` | — | newer variant, address as `bytes32` |
| `LiFiSwappedGeneric(...)` | `:67` | — | **deprecated**; kept in the ABI only so historic logs still decode |

Only `transactionId` (and `destinationChainId` on the non-EVM events) is indexed.
`LiFiTransferStarted` indexes **nothing** — the whole struct is data — so tracking
a user's transfers means decoding every log from the diamond, not filtering topics.

**The status lifecycle an indexer sees:**

```
  source chain                              destination chain
  ------------                              -----------------
  LiFiTransferStarted(txId, ...)
        |                                    (bridge delivers funds)
        |                                            |
        |                        +-------------------+-------------------+
        |                        |                                       |
        v                        v                                       v
     PENDING            LiFiTransferCompleted(txId)          LiFiTransferRecovered(txId)
                        destination swap succeeded           swap failed, raw tokens sent
```

### 7.2 Full interface catalogue

63 interface files. Grouped by role; each is a pure ABI declaration with no logic
unless noted.

**Diamond / standards (6)**

| File | Lines | Contents |
|---|---:|---|
| `IDiamondCut.sol` | 27 | `diamondCut(FacetCut[],address,bytes)` + `DiamondCut` event. Imports `LibDiamond` for the struct |
| `IDiamondLoupe.sol` | 41 | `struct Facet`, `facets()`, `facetFunctionSelectors()`, `facetAddresses()`, `facetAddress()` |
| `IERC165.sol` | 17 | `supportsInterface(bytes4)` |
| `IERC173.sol` | 23 | `owner()`, `transferOwnership(address)`, `OwnershipTransferred` event. ERC-165 id `0x7f5828d0` |
| `ILiFi.sol` | 76 | §7.1 |
| `IWhitelistManagerFacet.sol` | 80 | Granular allowlist facet ABI; documents the `0xffffffff` approve-only convention (`:29-33`) |

**LI.FI internal (2)**

| File | Lines | Contents |
|---|---:|---|
| `IExecutor.sol` | 21 | `swapAndCompleteBridgeTokens(bytes32, SwapData[], address, address payable) external payable` — the single function all Receivers call |
| `IERC20Proxy.sol` | 14 | `transferFrom(address,address,address,uint256)` |

**Across / intents (7)**

`IAcrossSpokePool.sol` (34), `IAcrossSpokePoolV4.sol` (61),
`ISpokePoolPeriphery.sol` (57), `ISponsoredCCTPSrcPeriphery.sol` (47),
`ISponsoredOFTSrcPeriphery.sol` (57), `IOriginSettler.sol` (18),
`IOpenIntentFramework.sol` (49).

**Bridges (21)**

`IAllBridge.sol` (43), `IChainflip.sol` (72), `ICircleBridgeProxy.sol` (31),
`IConnextHandler.sol` (71), `IDlnSource.sol` (52), `IEcoPortal.sol` (39),
`IFraxHopV2.sol` (97), `IGarden.sol` (33), `IGasZip.sol` (21),
`IGatewayRouter.sol` (57, Arbitrum), `IGlacisAirlift.sol` (77),
`IGnosisBridgeRouter.sol` (18), `IL1StandardBridge.sol` (44, Optimism),
`ILayerSwapDepository.sol` (27), `IMayan.sol` (47), `IOmniBridge.sol` (21),
`IOmniTokenAddressBook.sol` (17), `IPaxosTransit.sol` (50),
`IRelayDepository.sol` (39), `IRootChainManager.sol` (33, Polygon PoS),
`ISquidMulticall.sol` (22) + `ISquidRouter.sol` (40), `IStargate.sol` (124),
`ISymbiosisMetaRouter.sol` (37) + `IOnchainSwapV3.sol` (31),
`IThorSwap.sol` (16), `ITokenMessenger.sol` (80, Circle CCTP v2),
`IXDaiBridge.sol` (13) + `IXDaiBridgeL2.sol` (12),
`ISupersetPoolManager.sol` (20) + `ISupersetHubPoolManager.sol` (48) +
`ISupersetSpokePoolManager.sol` (53).

**DEX interfaces used by `LiFiDEXAggregator` (18)**

`IAlgebraFactory.sol` (20), `IAlgebraPool.sol` (76), `IAlgebraQuoter.sol` (51),
`IAlgebraRouter.sol` (19), `IHyperswapV3Factory.sol` (19),
`IHyperswapV3QuoterV2.sol` (37), `IiZiSwapPool.sol` (37),
`ISyncSwapPool.sol` (21), `ISyncSwapVault.sol` (14),
`IVelodromeV2Pool.sol` (50), `IVelodromeV2PoolCallee.sol` (14),
`IVelodromeV2PoolFactory.sol` (13), `IVelodromeV2Router.sol` (35),
`KatanaV3/IKatanaV3AggregateRouter.sol` (12),
`KatanaV3/IKatanaV3Governance.sol` (9), `KatanaV3/IKatanaV3Pool.sol` (19).

Note that `IUniswapV2Pair`, `IUniswapV3Pool`, `IWETH`, `IPool` (Trident),
`ITridentCLPool`, `ICurve`, `ICurveLegacy` and `IBentoBoxMinimal` are **not** in
`src/Interfaces/` — they are declared inline at the bottom of
`src/Periphery/LiFiDEXAggregator.sol` (`:1279-1616`).

---

## 8. Errors

`src/Errors/GenericErrors.sol` (45 lines, version `1.0.4`) declares 40 file-level
custom errors — file-level, so any contract can `import { X } from`. Complete table
with computed selectors:

| Selector | Error | Typical cause |
|---|---|---|
| `0x0dc149f0` | `AlreadyInitialized()` | re-running an initializer |
| `0xa9cefcae` | `CannotAuthoriseSelf()` | `LibAccess.addAccess(_, address(this))` |
| `0x4ac09ad3` | `CannotBridgeToSameNetwork()` | `destinationChainId == block.chainid` |
| `0x94539804` | `ContractCallNotAllowed()` | `(callTo, selector)` or `approveTo` not whitelisted |
| `0x275c273c` | `CumulativeSlippageTooHigh(uint256 minAmount, uint256 receivedAmount)` | end-to-end swap output below `minAmount` |
| `0x1ab7da6b` | `DeadlineExpired()` | past a signed deadline |
| `0x0149422e` | `DiamondIsPaused()` | pause guard (note: a pause-by-removal shows as `FunctionDoesNotExist`) |
| `0xb12d13eb` | `ETHTransferFailed()` | native send rejected |
| `0x350c20f1` | `ExternalCallFailed()` | `WithdrawablePeriphery.withdrawToken` native leg |
| `0xa9ad62f8` | `FunctionDoesNotExist()` | unknown selector on the diamond |
| `0x50dc905c` | `InformationMismatch()` | `hasSourceSwaps` / `hasDestinationCall` inconsistent with the entry point |
| `0xcf479181` | `InsufficientBalance(uint256 required, uint256 balance)` | `FeeForwarder.forwardNativeFees` overspend |
| `0x2c5211c6` | `InvalidAmount()` | zero amount, or `msg.value < amount` |
| `0x1c49f4d1` | `InvalidCallData()` | zero pool/recipient, bad mode byte, zero allowlist args |
| `0x35be3ac8` | `InvalidConfig()` | zero constructor parameter |
| `0x6eefed20` | `InvalidContract()` | `callTo` / whitelist target has ≤23 bytes of code |
| `0xb86ac1ef` | `InvalidDestinationChain()` | `onlyAllowDestinationChain` mismatch |
| `0xd7a2b022` | `InvalidFallbackAddress()` | bad refund address |
| `0x58b05100` | `InvalidNonEVMReceiver()` | empty non-EVM receiver |
| `0x1e4ec46b` | `InvalidReceiver()` | zero receiver |
| `0x7d6f2013` | `InvalidSendingToken()` | `onlyAllowSourceToken` mismatch |
| `0x8baa579f` | `InvalidSignature()` | signature recovery failed |
| `0x5ded5997` | `NativeAssetNotSupported()` | `noNativeAsset` on a token-only bridge |
| `0x5a046737` | `NativeAssetTransferFailed()` | native send failed |
| `0x0503c3ed` | `NoSwapDataProvided()` | empty `SwapData[]` |
| `0xe46e079c` | `NoSwapFromZeroBalance()` | `SwapData.fromAmount == 0` |
| `0x09ee12d5` | `NotAContract()` | code-size check |
| `0x87138d5c` | `NotInitialized()` | use before init |
| `0x21f74345` | `NoTransferToNullAddress()` | zero-address transfer |
| `0xd1bebf0c` | `NullAddrIsNotAnERC20Token()` | `transferFromERC20` with `address(0)` |
| `0x63ba9bff` | `NullAddrIsNotAValidSpender()` | approving `address(0)` |
| `0x277d76f8` | `OnlyContractOwner()` | `LibDiamond.enforceIsContractOwner` |
| `0xb897c401` | `RecoveryAddressCannotBeZero()` | zero recovery address |
| `0x29f745a7` | `ReentrancyError()` | `nonReentrant` re-entry |
| `0x3dd1b305` | `TokenNotSupported()` | unsupported token for a bridge |
| `0x7939f424` | `TransferFromFailed()` | pull failed |
| `0xbe245983` | `UnAuthorized()` | wrong caller: `onlyOwner`, `onlySpokepool`, `LibAccess`, ERC20Proxy |
| `0xa5dab5fe` | `UnsupportedChainId(uint256 chainId)` | chain not supported |
| `0x750b219c` | `WithdrawFailed()` | withdrawal failed |
| `0x1f2a2005` | `ZeroAmount()` | zero amount |

There is a `TODO` at `:10` noting that `EcoFacet`, `UnitFacet` and
`NEARIntentsFacet` still use their own deadline reverts rather than
`DeadlineExpired()`.

**Duplicate-selector warnings.** `FunctionDoesNotExist()` is declared both here
and in `LibDiamond` (`:22`); `ReentrancyError()` both here and in
`ReentrancyGuard` (`:21`); `UnAuthorized()` both here and in
`TransferrableOwnership` (`:13`). Same selectors, so on-chain they are
indistinguishable — do not try to infer the source contract from the error alone.

Per-contract errors are listed with each contract, and gathered in [§19](#19-cross-cutting-tables).

---

## 9. Security: `LiFiTimelockController.sol`

**Contract** `LiFiTimelockController is TimelockController`
(`src/Security/LiFiTimelockController.sol:20`), version `1.0.1`. Extends
OpenZeppelin's `TimelockController`.

Also declares `interface EmergencyPause { function unpauseDiamond(address[] calldata _blacklist) external; }`
(`:14-18`).

State: `address public diamond` (`:22`).
Event: `DiamondAddressUpdated(address indexed diamond)` (`:26`).

#### `constructor(uint256 _minDelay, address[] _proposers, address[] _executors, address _cancellerWallet, address _admin, address _diamond)` — `:34`

- Passes `_minDelay, _proposers, _executors, _admin` up to `TimelockController` (`:41`).
- Then validates **all six** — `_minDelay != 0`, both arrays non-empty, and three
  non-zero addresses — reverting `InvalidConfig()` (`:43-50`).
- Sets `diamond`, grants `CANCELLER_ROLE` to `_cancellerWallet` (`:55`), emits
  `DiamondAddressUpdated`.
- **Gotcha.** Validation runs *after* the parent constructor, so an invalid config
  still executes OZ's role setup before reverting. Harmless (the whole deployment
  reverts) but surprising in traces.

#### `setDiamondAddress(address _diamond) external onlyRole(TIMELOCK_ADMIN_ROLE)` — `:63`
Updates the controlled diamond and emits. **No zero check here**, unlike the
constructor.

#### `unpauseDiamond(address[] calldata _blacklist) external onlyRole(TIMELOCK_ADMIN_ROLE)` — `:77`

The deliberate exception to the timelock. Calls
`EmergencyPause(diamond).unpauseDiamond(_blacklist)` **directly**, bypassing
`minDelay` entirely (`:81`).

The safety argument is spelled out at `:70-76`: unpausing can only *re-add
previously registered* facet→selector mappings, minus a blacklist. It cannot
introduce new code. So the worst a compromised admin achieves is restoring facets
that were already live, or refusing to restore some — neither is a path to
arbitrary execution. Meanwhile a delay here would leave the protocol frozen for
`minDelay` after a false-positive pause.

**The governance shape in production:**

```
   Pauser wallet ---------> EmergencyPauseFacet.pauseDiamond()      [instant, removes facets]
                                        |
   LI.FI MultiSig SAFE (TIMELOCK_ADMIN) |
        |                               v
        +--> LiFiTimelockController.unpauseDiamond()  [instant, bypasses minDelay]
        |
        +--> schedule(diamondCut) --> wait minDelay --> execute()   [normal upgrades]
        |
   Canceller wallet --> cancel()                       [kills a pending proposal]
```

Because the timelock is the diamond's `contractOwner`, and `LibDiamond.diamondCut`
can `delegatecall` an arbitrary initializer (§2.2), the `minDelay` is the primary
protection users have against a malicious or compromised upgrade.

---

## 10. Periphery: execution core

### 10.1 `Executor.sol`

**Contract** `Executor is ILiFi, ReentrancyGuard, ERC1155Holder, ERC721Holder, WithdrawablePeriphery`
(`src/Periphery/Executor.sol:20-26`), version `2.1.0`.

This is the destination-chain workhorse: it receives bridged funds and performs
arbitrary swaps/calls with them. `ERC1155Holder` and `ERC721Holder` are inherited
so a destination call can mint or transfer an NFT to the Executor mid-route
without the transfer reverting on the receiver hook.

State: `IERC20Proxy public erc20Proxy` (`:30`). Event: `ERC20ProxySet(address indexed proxy)` (`:33`).

#### `modifier noLeftovers(SwapData[] calldata _swaps, address payable _leftoverReceiver)` — `:38`

Structurally different from `SwapperV2`'s. It **branches on the swap count**:

```solidity
if (numSwaps != 1) {
    uint256[] memory initialBalances = _fetchBalances(_swaps);
    address finalAsset = _swaps[numSwaps - 1].receivingAssetId;
    _;                                                     // :48
    for (uint256 i = 0; i < numSwaps - 1; ) { ... }         // :50-66
} else {
    _;                                                      // :68
}
```

Single-hop routes skip snapshotting entirely (a real gas saving on the
destination chain). Multi-hop routes sweep every non-final intermediate output
whose balance grew. Unlike `SwapperV2._refundLeftovers`, this one does **not**
sweep unspent *inputs* — that job is done differently, by the `_transferredAssetId`
reconciliation in `_processSwaps`.

#### `constructor(address _erc20Proxy, address _owner)` — `:75`
Sets `erc20Proxy`, emits `ERC20ProxySet`. Carries a `TODO` at `:79` noting the
missing zero-address validation.

#### `swapAndCompleteBridgeTokens(bytes32 _transactionId, SwapData[] calldata _swapData, address _transferredAssetId, address payable _receiver) external payable nonReentrant` — `:91`

- **The Receivers' entry point.** Delegates to `_processSwaps(..., 0, true)` — the
  `true` meaning "pull via allowance", the `0` meaning "amount comes from the
  allowance, not a parameter".
- **Permissionless.** Anyone may call it. Safe because it only ever moves funds it
  can pull from `msg.sender`'s allowance and forwards results to `_receiver`.

#### `swapAndExecute(bytes32, SwapData[] calldata, address _transferredAssetId, address payable _receiver, uint256 _amount) external payable nonReentrant` — `:113`
Same, but `_depositAllowance = false`, so funds are pulled through the
**ERC20Proxy** for the explicit `_amount`. This is the same-chain / pre-approved
path.

#### `_processSwaps(bytes32, SwapData[] calldata, address _transferredAssetId, address payable _receiver, uint256 _amount, bool _depositAllowance) private` — `:139`

The core. Step by step:

1. **Final-asset snapshot** (`:149-157`). `finalAssetId` is the last hop's output;
   its starting balance is recorded, minus `msg.value` when native.
2. **Input snapshot and pull** (`:159-179`):
   - ERC20 + `_depositAllowance`: read `IERC20(_transferredAssetId).allowance(msg.sender, address(this))`
     and `LibAsset.depositAsset(_transferredAssetId, allowance)` — it pulls the
     **entire allowance**, not a stated amount. That is why Receivers approve
     exactly `amount` and reset to 0 afterwards.
   - ERC20 + `!_depositAllowance`: `erc20Proxy.transferFrom(..., _amount)`.
   - Native: `startingBalance = balance - msg.value`.
3. **`_executeSwaps`** (`:181`).
4. **Refund unspent input** (`:183-190`): if the transferred asset's balance is
   still above the starting balance, send the surplus to `_receiver`. A
   partially-consuming swap therefore returns the remainder to the user, not to
   the Executor.
5. **Deliver output** (`:192-202`): send `finalAssetPostSwapBalance - finalAssetStartingBalance`
   of `finalAssetId` to `_receiver`.
6. **`emit LiFiTransferCompleted`** (`:204-210`).

**A real gotcha in the event.** The `amount` field is
`finalAssetPostSwapBalance` — the contract's **absolute post-swap balance**, not
the delta actually transferred (`:208`). If the Executor is holding stray dust of
the output token, the event over-reports. Indexers should trust the token transfer,
not this number.

#### `_executeSwaps(bytes32, SwapData[] calldata, address payable _leftoverReceiver) private noLeftovers(...)` — `:217`

Loops the hops. The one check (`:224-226`):

```solidity
if (_swapData[i].callTo == address(erc20Proxy)) revert UnAuthorized();
```

**This is essential.** The ERC20Proxy authorises the Executor to move *any user's*
approved tokens. Without this guard, a route could name the proxy as `callTo` and
hand it `transferFrom(victimToken, victim, attacker, all)`. Note that the Executor
does **not** consult `LibAllowList` — it is a standalone contract with no access to
diamond storage — so this single check plus "the Executor holds nothing between
transactions" is its whole security model.

#### `_fetchBalances(SwapData[] calldata) private view returns (uint256[] memory)` — `:239`
Per-hop `receivingAssetId` balances, minus `msg.value` for native.

#### `receive() external payable` — `:263`
Required for native output from destination swaps and for WETH unwrapping.

### 10.2 `ERC20Proxy.sol`

**Contract** `ERC20Proxy is WithdrawablePeriphery` (`src/Periphery/ERC20Proxy.sol:12`),
version `1.2.0`. 63 lines that carry a lot of weight.

**Why it exists.** Users approve the *proxy*, not the Executor. The Executor is
the contract that makes arbitrary calls, so it is the one most likely to be
exploited or replaced. Keeping approvals on a minimal, near-immutable contract
means an Executor bug cannot reach user allowances, and the Executor can be
redeployed without users re-approving.

State: `mapping(address => bool) public authorizedCallers` (`:14`).
Event: `AuthorizationChanged(address indexed caller, bool authorized)` (`:17`).

#### `constructor(address _owner, address _executorAddress)` — `:26`
`_owner == address(0)` → `InvalidConfig()`. If `_executorAddress != 0`, authorise
it immediately and emit.

The comment at `:19-25` explains the CREATE3 ordering: the Executor is deployed
*after* the proxy (its constructor needs the proxy address), but its CREATE3
address is known in advance, so it is pre-authorised here. Otherwise the
post-deploy `setAuthorizedCaller` would need to come from the owner
(the refund wallet), which the deploy wallet cannot do.

#### `setAuthorizedCaller(address caller, bool authorized) external onlyOwner` — `:40`
Flag write plus event.

#### `transferFrom(address tokenAddress, address from, address to, uint256 amount) external` — `:53`
`if (!authorizedCallers[msg.sender]) revert UnAuthorized();` then
`LibAsset.transferFromERC20(...)`. Deliberately **not** ERC20-compliant despite
the name — it takes the token as its first argument.

**Trust model.** An authorised caller can move *any* token from *any* address that
has approved this proxy, in any amount. `authorizedCallers` is therefore the most
security-critical mapping in the periphery. The `Executor` guard at
`Executor.sol:224` is the other half of that story.

---

## 11. Periphery: the Receivers

Five contracts, one job: be the address a bridge delivers to on the destination
chain, authenticate the caller, decode the LI.FI payload, and hand off to the
Executor — while guaranteeing that **the user gets something even if the swap
fails**.

```
  bridge delivers tokens + message
              |
              v
      +-----------------+   onlyX modifier: is msg.sender the real bridge?
      |    Receiver     |
      +-----------------+
              |  abi.decode(message) -> (transactionId, SwapData[], receiver)
              |  approve(EXECUTOR, amount)
              v
      +-----------------+  try  ------> swaps succeed -> LiFiTransferCompleted
      |    Executor     |
      +-----------------+  catch ------> send raw bridged token to receiver
              |                          -> LiFiTransferRecovered
              v
        approve(EXECUTOR, 0)
```

Every one of them decodes the identical payload shape:

```solidity
(bytes32 transactionId, LibSwap.SwapData[] memory swapData, address receiver)
    = abi.decode(message, (bytes32, LibSwap.SwapData[], address));
```

### 11.1 `ReceiverAcrossV3.sol`

**Contract** `ReceiverAcrossV3 is ILiFi, WithdrawablePeriphery`
(`src/Periphery/ReceiverAcrossV3.sol:18`), version `1.1.0`.

Immutables: `IExecutor public immutable executor` (`:23`), `address public immutable spokepool` (`:25`).
Modifier `onlySpokepool()` (`:28`) → `UnAuthorized()`.
Constructor (`:36`) has **no zero-address validation** (`TODO` at `:41`).

#### `handleV3AcrossMessage(address tokenSent, uint256 amount, address /*relayer*/, bytes memory message) external onlySpokepool` — `:55`
The callback Across's SpokePool makes on fill. Decodes and calls the private helper.
The third parameter (relayer) is deliberately unnamed and unused.

#### `_swapAndCompleteBridgeTokens(bytes32, SwapData[] memory, address assetId, address payable receiver, uint256 amount) private` — `:88`
1. `assetId.safeApproveWithRetry(address(executor), amount)` (`:95`).
2. `try executor.swapAndCompleteBridgeTokens(...)` (`:96-102`).
3. `catch`: `assetId.safeTransfer(receiver, amount)` and emit `LiFiTransferRecovered` (`:113-121`).
4. **Always** `assetId.safeApprove(address(executor), 0)` (`:125`).

**No native handling** — Across always delivers wrapped native, as documented at `:81`.
Carries a `TODO(EXSC-241)` about routing through `LibAsset` for Tron USDT support.

`receive()` at `:130`.

### 11.2 `ReceiverAcrossV4.sol`

**Contract** (`src/Periphery/ReceiverAcrossV4.sol:16`), version `1.0.0`.
Functionally the V3 contract with three changes:

1. Immutables renamed to SCREAMING_CASE: `EXECUTOR` (`:20`), `SPOKEPOOL` (`:21`).
2. Constructor **validates** all three addresses → `InvalidConfig()` (`:37-41`).
3. Recovery uses `LibAsset.transferERC20` (`:109`) instead of raw
   `safeTransfer` — the migration the V3 TODO describes.

The entry point is still called `handleV3AcrossMessage` (`:57`); the comment at
`:52` notes Across never renamed it for V4.

### 11.3 `ReceiverStargateV2.sol`

**Contract** `ReceiverStargateV2 is ILiFi, WithdrawablePeriphery, ILayerZeroComposer`
(`src/Periphery/ReceiverStargateV2.sol:40-44`), version `1.1.0`. The most defensive
of the five.

Declares `interface IPool { function token() external view returns (address); }` (`:16-18`)
and `interface ILayerZeroComposer` (`:20-34`) inline.

Immutables: `executor` (`:49`), `tokenMessaging` (`:51`), `endpointV2` (`:53`),
`recoverGas` (`:55`). Modifier `onlyEndpointV2()` (`:58`).

#### `lzCompose(address _from, bytes32, bytes calldata _message, address, bytes calldata) external payable onlyEndpointV2` — `:101`

**Two-layer authentication**, which is what makes this contract interesting:

1. `msg.sender` must be the LayerZero endpoint (modifier).
2. `if (tokenMessaging.assetIds(_from) == 0) revert UnAuthorized();` (`:110`) —
   `_from` must be a **registered Stargate pool**. The endpoint will deliver a
   compose message from *any* OApp, so without this a fake OApp could drive the
   Receiver.

Then `bridgedAssetId = IPool(_from).token()` (`:113`), decode
`OFTComposeMsgCodec.composeMsg(_message)` (`:121`), and use
`OFTComposeMsgCodec.amountLD(_message)` (`:131`) as the amount.

**The known limitation, documented at `:81-92`.** `lzCompose` is permissionless
and replayable with identical parameters, so a frontrunner can call it with
*just enough* gas to pass the `recoverGas` check but not enough for the swap,
forcing the recovery path. The user then gets raw bridged tokens instead of the
swap output. The team deliberately does not pin `_executor` because production
recovery has required manual re-execution. Integrators are told to monitor
`LiFiTransferRecovered`.

#### `_swapAndCompleteBridgeTokens(...) private` — `:144`

Caches `gasleft()` (`:151`) and branches four ways:

| Case | Condition | Behaviour |
|---|---|---|
| 1a | native, `gasleft() < recoverGas` | `safeTransferETH(receiver, amount)`, emit recovered, `return` |
| 1b | native, enough gas | `try executor.swapAndCompleteBridgeTokens{value: amount, gas: cacheGasLeft - recoverGas}(...)`; on catch, refund + emit |
| 2a | ERC20, `gasleft() < recoverGas` | `safeApprove(executor, 0)` first (`:190`), then `safeTransfer`, emit, `return` |
| 2b | ERC20, enough gas | `safeIncreaseAllowance` (`:215`), gas-capped `try`, catch → transfer + emit; **then `safeApprove(executor, 0)`** (`:239`) |

**The gas reservation is the point.** By calling with `gas: cacheGasLeft - recoverGas`
it guarantees that even if the swap consumes everything it was given, `recoverGas`
remains to execute the fallback transfer. Without it, an out-of-gas swap would
revert the whole `lzCompose` and the funds would sit in the Receiver.

Note the EIP-150 subtlety: `call` forwards at most 63/64 of remaining gas, so the
child actually gets `min(cacheGasLeft - recoverGas, 63/64 × available)` — the
reserve is in practice slightly larger than requested, which is the safe direction.

### 11.4 `ReceiverChainflip.sol`

**Contract** (`src/Periphery/ReceiverChainflip.sol:16`), version `1.0.1`.

Immutables `executor` (`:24`), `chainflipVault` (`:27`). Constant
`CHAINFLIP_NATIVE_ADDRESS = 0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE` (`:29-30`).
Modifier `onlyChainflipVault()` (`:36`). Constructor validates all three
addresses (`:54-60`).

#### `cfReceive(uint32, bytes calldata, bytes calldata message, address token, uint256 amount) external payable onlyChainflipVault` — `:74`
Source chain and source address are accepted and ignored.

#### `_swapAndCompleteBridgeTokens(...) private` — `:108`
First **normalises the native sentinel** (`:116-118`):

```solidity
address actualAssetId = assetId == CHAINFLIP_NATIVE_ADDRESS
    ? LibAsset.NULL_ADDRESS
    : assetId;
```

Chainflip uses `0xEeee…`, LI.FI uses `address(0)`. Every downstream call would
misbehave without this translation.

Then two branches. ERC20 (`:121-151`): approve-with-retry, `try`, on success
`return` **early** — note this skips the `safeApprove(0)` at `:151`, so a
successful swap leaves the approval set to `amount`. It is harmless in practice
(the Executor pulls the whole allowance), but it is an inconsistency with the
Across receivers. Native (`:152-173`): `try` with `value: amount`, catch →
`receiver.safeTransferETH`.

### 11.5 `ReceiverOIF.sol`

**Contract** `ReceiverOIF is ILiFi, WithdrawablePeriphery, IOutputCallback`
(`src/Periphery/ReceiverOIF.sol:31`), version `1.0.0`. Declares
`interface IOutputCallback { function outputFilled(bytes32 token, uint256 amount, bytes calldata executionData) external; }` (`:15-24`).

Immutables `EXECUTOR` (`:33`), `OUTPUT_SETTLER` (`:34`). Modifier
`onlyTrustedOutputSettler()` (`:37`). Constructor validates all three (`:50-52`).

#### `outputFilled(bytes32 token, uint256 amount, bytes calldata executionData) external onlyTrustedOutputSettler` — `:75`

**Read the header comment at `:59-71` — this contract is different in kind.** The
settler restriction proves the token and amount were genuinely delivered, but it
does **not** validate the `executionData`. The contract explicitly states that
supplied `SwapData` must be self-contained, with its own slippage protection and
its own revert-on-failure, and lists the ways an integrator can lose funds:

- not pulling the approved token (the contract does not verify the input left),
- not reverting on a failed swap (tokens are abandoned and collectable by anyone),
- no slippage bounds on the swap.

Only one on-chain guard: `receiver == address(0)` → `InvalidReceiver()` (`:87`),
which blocks the fill so the user is refunded on the source chain.

`address(uint160(uint256(token)))` (`:94`) is an unchecked `bytes32 → address`
truncation, with `TODO(EXSC-626)` to move to `LibBytes.toAddress`.

#### `_swapAndCompleteBridgeTokens(...) private` — `:108`
**No `try/catch` at all** (`:123-128`) — a failed swap reverts the entire fill,
which is the intended behaviour for an intent: the solver is simply not paid and
the user is made whole on the source chain. Native is signalled by
`token == bytes32(0)`, delivered before the callback, and forwarded as
`value: amount`.

### 11.6 Receiver comparison

| | AcrossV3 | AcrossV4 | StargateV2 | Chainflip | OIF |
|---|---|---|---|---|---|
| Entry point | `handleV3AcrossMessage` | `handleV3AcrossMessage` | `lzCompose` | `cfReceive` | `outputFilled` |
| Caller check | `spokepool` | `SPOKEPOOL` | `endpointV2` **+ registered Stargate pool** | `chainflipVault` | `OUTPUT_SETTLER` |
| Constructor validation | none | all three | none | all three | all three |
| Native supported | no | no | yes | yes (via `0xEeee…`) | yes (via `bytes32(0)`) |
| Failure behaviour | `try/catch` → raw transfer | `try/catch` → raw transfer | gas-reserved `try/catch` → raw transfer | `try/catch` → raw transfer | **reverts** |
| Gas reservation | no | no | yes (`recoverGas`) | no | no |
| Approval reset to 0 | always | always | always (both paths) | only on failure | n/a |
| Emits `LiFiTransferRecovered` | yes | yes | yes | yes | never |

---

## 12. Periphery: fees

Two contracts with opposite designs: one is a ledger that custodies fees until
withdrawal, the other is a pass-through that keeps nothing.

### 12.1 `FeeCollector.sol`

**Contract** `FeeCollector is TransferrableOwnership` (`src/Periphery/FeeCollector.sol:12`),
version `1.0.1`. Note it inherits `TransferrableOwnership` **directly**, not
`WithdrawablePeriphery` — it has its own withdrawal semantics, and an owner-level
sweep would let LI.FI take integrator balances.

State (`:16-18`):
```solidity
mapping(address => mapping(address => uint256)) private _balances;  // integrator => token => amount
mapping(address => uint256) private _lifiBalances;                  // token => amount
```

Errors: `TransferFailure()` `0xf7e6817a`, `NotEnoughNativeForFees()` `0x840a2adf`.
(`TransferFailure` is declared but never thrown — dead code kept for ABI stability.)

Events: `FeesCollected(address indexed _token, address indexed _integrator, uint256 _integratorFee, uint256 _lifiFee)` (`:25`),
`FeesWithdrawn(address indexed _token, address indexed _to, uint256 _amount)` (`:31`),
`LiFiFeesWithdrawn(address indexed _token, address indexed _to, uint256 _amount)` (`:36`).

| Function | Line | Selector | Behaviour |
|---|---|---|---|
| `collectTokenFees(address tokenAddress, uint256 integratorFee, uint256 lifiFee, address integratorAddress)` | `:54` | `0xeedd56e1` | `depositAsset(token, integratorFee + lifiFee)` pulls from `msg.sender`, then credits both ledgers. Permissionless |
| `collectNativeFees(uint256 integratorFee, uint256 lifiFee, address integratorAddress) payable` | `:75` | `0xe0cbc5f2` | `msg.value < integratorFee + lifiFee` → `NotEnoughNativeForFees()`. Credits both, then **refunds the remainder to `msg.sender`** (`:84-89`) so nothing is stranded |
| `withdrawIntegratorFees(address tokenAddress)` | `:100` | `0xbd0b380b` | Sweeps `_balances[msg.sender][token]`. Returns silently on zero. **Effects before interaction** (`:105-106`) |
| `batchWithdrawIntegratorFees(address[] memory tokenAddresses)` | `:112` | `0xe5d64766` | Loop of the above, skipping zero balances |
| `withdrawLifiFees(address tokenAddress) onlyOwner` | `:136` | `0x461ad4f5` | Same for the LI.FI ledger |
| `batchWithdrawLifiFees(address[] memory tokenAddresses) onlyOwner` | `:148` | `0x64bc5be1` | Loop. **Does not skip zero balances** — unlike its integrator counterpart it writes and emits regardless (`:154-161`), producing zero-value events |
| `getTokenBalance(address integratorAddress, address tokenAddress) view` | `:171` | `0xc489744b` | ledger read |
| `getLifiTokenBalance(address tokenAddress) view` | `:180` | `0x0fe97f70` | ledger read |

**Key property: an integrator can only ever withdraw its own balance**, keyed on
`msg.sender`. There is no admin override. The owner's power is limited to
`_lifiBalances`.

**Gotcha.** Because `collectTokenFees` is permissionless and credits whatever
`integratorAddress` is passed, anyone can credit fees to anyone. That is harmless
(you are giving away your own tokens) but means the ledger is not an authorisation
record.

**Usage.** The collector is invoked as a `LibSwap.SwapData` step inside a route —
`callTo = FeeCollector`, `callData = collectTokenFees(...)` — which is why
`LibSwap`'s doc comment (`:11-13`) mentions fee collection explicitly.

### 12.2 `FeeForwarder.sol`

**Contract** `FeeForwarder is WithdrawablePeriphery` (`src/Periphery/FeeForwarder.sol:16`),
version `2.0.0`. The modern replacement: no ledger, no custody.

The contract header (`:11-14`) states the design intent — a normal invocation
forwards everything and leaves no net balance; anything that accumulates anyway is
never paid out as a refund and is owner-recoverable only.

`struct FeeDistribution { address recipient; uint256 amount; }` (`:22-25`).
Event `FeesForwarded(address indexed token, FeeDistribution[] distributions)` (`:32`).
Constructor rejects a zero owner (`:42`).

#### `forwardERC20Fees(address _token, FeeDistribution[] calldata _distributions) external` — `:52`
Loops, calling `LibAsset.transferFromERC20(_token, msg.sender, recipient, amount)`
per entry (`:68-73`), then emits. Tokens move **directly from the caller** to each
recipient; the contract never holds them. Deliberately unvalidated for gas:
no length check, no balance check, no allowance check, no zero-amount skip
(`:56-66`) — a bad input simply reverts inside the transfer.

#### `forwardNativeFees(FeeDistribution[] calldata _distributions) external payable` — `:86`

More interesting, because native transfers forward all gas and recipients can
re-enter. The sequence (`:92-125`):

1. Accumulate `totalDistributed` **unchecked** and send each amount as it goes.
2. **After** the loop: `if (totalDistributed > msg.value) revert InsufficientBalance(totalDistributed, msg.value);`
3. Refund `msg.value - totalDistributed` to `msg.sender` if non-zero.

The comment at `:110-113` explains why the check is post-hoc rather than
pre-computed: recipients receive with all gas forwarded and **can call back into
this function**. Checking against the contract balance would let a nested call
draw on a stray balance or on a parent call's still-undistributed funds. Binding
each invocation to its own `msg.value` makes reentrancy safe without a lock.

The unchecked accumulation (`:100-102`) is safe because a real overflow would
require summing amounts that no balance could cover — the transfers would revert
long before.

**Gotcha.** Transfers happen *before* the total is validated, so a partial payout
followed by revert is possible in-trace, though the revert undoes it. An empty
array is legal and just emits an event (`:84`).

---

## 13. Periphery: wrappers and liquid staking

### 13.1 `TokenWrapper.sol`

**Contract** `TokenWrapper is WithdrawablePeriphery` (`src/Periphery/TokenWrapper.sol:27`),
version `1.2.1`. Wraps/unwraps native, optionally through a decimal converter.

Immutables (`:28-32`): `WRAPPED_TOKEN`, `CONVERTER`, `USE_CONVERTER` (private),
`SWAP_RATIO_MULTIPLIER` (private); constant `BASE_DENOMINATOR = 1 ether`.

#### `constructor(address _wrappedToken, address _converter, address _owner)` — `:40`
- Zero `_wrappedToken` or `_owner` → `InvalidConfig()`; non-contract `_wrappedToken`
  → `InvalidContract()` (`:45-47`).
- `USE_CONVERTER = _converter != address(0)` (`:50`). When set, validates it is a
  contract and grants it an **infinite approval once** at construction (`:52-60`) —
  the gas optimisation that makes `withdraw()` cheap.
- Immutables must be assigned unconditionally, hence the ternaries at `:63-66`:
  `CONVERTER` falls back to `_wrappedToken`, and `SWAP_RATIO_MULTIPLIER` is
  `10 ** decimals(_wrappedToken)` with a converter, else `1 ether`.

#### `deposit() external payable` — `:74` — selector `0xd0e30db0`
`WETH(CONVERTER).deposit{value: msg.value}()`, then if a converter is in use scale
`amount = (msg.value * SWAP_RATIO_MULTIPLIER) / BASE_DENOMINATOR` (`:79`), then
`safeTransfer` to `msg.sender`. Without a converter the ratio is 1e18/1e18 = 1.

#### `withdraw() external` — `:88` — selector `0x3ccfd60b`
Pulls the caller's **entire balance** of `WRAPPED_TOKEN` (`:93-99`) —
justified at `:89-92` as a gas optimisation valid because in LI.FI routes the
allowance is always near-max. Then `WETH(CONVERTER).withdraw(amount)`, inverse
scaling (`:104`), `safeTransferETH` to `msg.sender`.

`receive()` at `:111` — required to receive native from `withdraw`.

**Two documented assumptions** (`:24-25`): the native token has 18 decimals, and
the converter charges **no fee** and only converts decimals. A fee-charging
converter would silently under-deliver, since the returned amount is computed by
ratio rather than measured.

### 13.2 `LidoWrapper.sol`

**Contract** `LidoWrapper is WithdrawablePeriphery` (`src/Periphery/LidoWrapper.sol:26`),
version `1.0.0`. This is the reference's main contact with liquid staking, so the
mechanics are worth stating properly.

#### The stETH / wstETH model

Lido issues **stETH** when you deposit ETH. stETH is a **rebasing** token: your
balance number grows every day as staking rewards accrue, while 1 stETH stays
pegged to ~1 ETH. Internally Lido tracks *shares*; `balanceOf` returns
`shares × totalPooledEther / totalShares`.

That rebasing is poison for DeFi integrations. An AMM pool, a lending market, or
a bridge that records "user holds 100 stETH" is wrong tomorrow. So Lido also
issues **wstETH**: a wrapper whose balance is the raw share count and never
rebases, while its *price* in stETH rises over time. Same economic exposure, two
different accounting shapes:

| | stETH | wstETH |
|---|---|---|
| Balance | grows daily (rebases) | constant |
| Value per unit | ~1 ETH, fixed | rises with rewards |
| Represents | pooled ether | shares |
| DeFi-friendly | poorly | yes |

Aggregators need to move between them because a route may source stETH but the
destination pool only lists wstETH (or vice versa). Hence this contract, used as
a `LibSwap.SwapData` step.

#### The naming inversion

**The single most important thing in this file.** On Lido's **L2** stETH
contract, `wrap` and `unwrap` mean the opposite of what you expect
(`:23`, `:64`, `:77`, `:104`). The L2 stETH token is the one holding the methods,
so from its perspective:

- `IStETH.unwrap(amount)` → consumes stETH, returns **wstETH**
- `IStETH.wrap(amount)` → consumes wstETH, returns **stETH**

The interface declaration at `:10-18` even carries `@notice` comments that read
backwards relative to the names, which is faithful to the upstream contract.

State: `ETH_CHAIN_ID = 1` (`:27`), `IStETH public immutable ST_ETH` (`:30`),
`address public immutable WST_ETH_ADDRESS` (`:33`).
Error: `ContractNotYetReadyForMainnet()` `0x8bb9ffc7`.

#### `constructor(address _stETHAddress, address _wstETHAddress, address _owner)` — `:41`
Validates all three non-zero → `InvalidConfig()` (`:46-50`). Then
**`if (block.chainid == ETH_CHAIN_ID) revert ContractNotYetReadyForMainnet();`**
(`:56-57`) — mainnet's Lido has a different wrap/unwrap surface, so deployment is
blocked there. Finally grants `ST_ETH` an infinite approval over this contract's
wstETH (`:60`).

#### `wrapStETHToWstETH(uint256 _amount) external returns (uint256 wrappedAmount)` — `:67` — selector `0x24dd6483`
1. Pull `_amount` stETH from the caller (`:71-75`).
2. Read the contract's **full** stETH balance (`:79-81`).
3. `ST_ETH.unwrap(stETHBalance)` — inverted naming, yields wstETH (`:82`).
4. `transfer(msg.sender, wrappedAmount)` (`:85`).

Using the full balance is justified at `:77-78` ("designed to not hold funds"),
and it is also what makes the contract robust to stETH's classic
**1-wei transfer shortfall**: because stETH converts amounts to shares and back,
`transferFrom(x)` can deliver `x - 1`. Pulling `_amount` but wrapping
`balanceOf(this)` sidesteps the off-by-one entirely.

#### `unwrapWstETHToStETH(uint256 _amount) external returns (uint256 unwrappedAmount)` — `:94` — selector `0xa816ca92`
Pull `_amount` wstETH (`:98-102`), `ST_ETH.wrap(_amount)` → stETH (`:105`),
transfer out (`:108`). Note this direction uses `_amount`, not the full balance,
because wstETH is not rebasing and has no rounding quirk.

**Neither function emits an event** (`:87`, `:110`) — the Lido contracts already
do. **Neither uses `SafeERC20`** — raw `IERC20.transfer`/`transferFrom`, which is
safe only because stETH and wstETH both return proper booleans.

**Documented risk** (`:24`): tokens sent directly to this contract can be swept by
anyone, since both functions operate on full balances.

---

## 14. Periphery: `OutputValidator.sol`

**Contract** `OutputValidator is WithdrawablePeriphery` (`src/Periphery/OutputValidator.sol:20`),
version `1.0.0`. Skims **positive slippage** — output above what the user was
quoted — to a validation wallet.

Event `OutputValidated(address indexed token, address indexed validationWallet, uint256 excessAmount)` (`:27`).
Constructor rejects a zero owner with `InvalidCallData()` (`:35`) — note it uses
`InvalidCallData`, not the `InvalidConfig` its siblings use.

The header (`:13-18`) is the important part: output is measured as the **caller's
full balance**, the same full-balance pattern `GenericSwapFacetV3` uses. It is
therefore only safe as a whitelisted step inside an atomic route.

#### `validateNativeOutput(uint256 expectedAmount, address validationWalletAddress) external payable` — `:48` — selector `0x5d865df2`

`outputAmount = msg.sender.balance + msg.value` (`:58`). Three branches:

| Condition | Action |
|---|---|
| `outputAmount > expectedAmount` and `excess >= msg.value` | forward **all** `msg.value` to the validation wallet (skipping a zero-value call), emit |
| `outputAmount > expectedAmount` and `excess < msg.value` | forward `excess`, emit, refund `msg.value - excess` to `msg.sender` |
| `outputAmount <= expectedAmount` | refund all `msg.value` if non-zero; otherwise do nothing |

The cap is explicit at `:42-45`: native payouts can never exceed `msg.value`, so
pre-existing native held by the caller counts toward `outputAmount` but can never
be transferred out by this contract. The intended integration forwards only the
excess as `msg.value` with `expectedAmount == 0`.

#### `validateERC20Output(address tokenAddress, uint256 expectedAmount, address validationWalletAddress) external` — `:118` — selector `0x27444dab`

`outputAmount = IERC20(tokenAddress).balanceOf(msg.sender)` (`:128`). If it
exceeds `expectedAmount`, `LibAsset.transferFromERC20(token, msg.sender, validationWallet, excess)`
(`:136-141`) and emit. **Requires an approval from the caller to this contract**
(`:127`) — the ERC20 path pulls, whereas the native path is pushed.

**Gotcha.** No zero-address validation on `validationWalletAddress` in either
function; `LibAsset` catches it (`InvalidReceiver()` for native,
`transferFromERC20`'s own check for ERC20). And no access control at all — anyone
may call these, which is only safe because they can move nothing the caller has
not already sent or approved.

---

## 15. Periphery: `GasZipPeriphery.sol`

**Contract** `GasZipPeriphery is ILiFi, WithdrawablePeriphery`
(`src/Periphery/GasZipPeriphery.sol:18`), version `1.0.2`. Buys destination-chain
gas via gas.zip, as a route step.

Immutables `GAS_ZIP_ROUTER` (`:22`), `LIFI_DIAMOND` (`:23`); constants
`MAX_CHAINID_LENGTH_ALLOWED = 16` (`:24`), `APPROVE_TO_ONLY_SELECTOR = 0xffffffff` (`:26`).
Errors: `TooManyChainIds()` `0x1ee194c6`, `SwapOutputMustBeNative()` `0x3e457d68`.
Constructor validates all three addresses (`:38-44`).

#### `depositToGasZipERC20(LibSwap.SwapData calldata _swapData, IGasZip.GasZipData calldata _gasZipData) public` — `:54`

1. `_swapData.receivingAssetId != address(0)` → `SwapOutputMustBeNative()` (`:58-60`).
2. **Whitelist check against the diamond** (`:62-83`). This contract is standalone
   and cannot read diamond storage, so it calls back into the diamond's
   `IWhitelistManagerFacet.isContractSelectorWhitelisted` — reproducing exactly the
   `SwapperV2` rule, including the `0xffffffff` approve-only clause.
3. `LibAsset.depositAsset` pulls the ERC20 from the diamond (`:86`).
4. `maxApproveERC20` to `approveTo` (`:89-93`).
5. Snapshot `address(this).balance`, execute the raw call (`:102-107`), bubble via
   `LibUtil.revertWith` on failure.
6. `swapOutputAmount = address(this).balance - preSwapBal` (`:109`).
7. Tail-call `depositToGasZipNative(_gasZipData, swapOutputAmount)` (`:112`).

The slippage note at `:98-100` is worth keeping in mind: per-swap protection lives
in the DEX calldata, and the cumulative check happens back in
`SwapperV2._depositAndSwap`.

#### `depositToGasZipNative(IGasZip.GasZipData calldata _gasZipData, uint256 _amount) public payable` — `:119`

`receiverAddress == bytes32(0)` → `InvalidCallData()` (`:124-125`). Calls
`GAS_ZIP_ROUTER.deposit{value: _amount}(destinationChains, receiverAddress)`
(`:128-131`), then **sweeps its entire remaining native balance to `msg.sender`**
(`:135-138`), described at `:133-134` as a backend money-flow requirement.

The receiver is `bytes32` to accommodate Solana. `IGasZip.sol:14-16` warns that
**EVM addresses must be left-aligned** (trailing zeros), not right-aligned — the
opposite of the usual `bytes32(uint256(uint160(addr)))` padding. Getting this
wrong sends gas to a wrong address on every destination chain.

#### `getDestinationChainsValue(uint8[] calldata _chainIds) external pure returns (uint256)` — `:144`

Packs gas.zip-specific chain ids into one `uint256`, 16 bits each:

```
destinationChains = (destinationChains << 16) | chainId    // :153-155
```

`length > 16` → `TooManyChainIds()` (`:149`). 16 slots × 16 bits = 256 bits
exactly. The ids are gas.zip's own numbering, **not** EVM chain ids (`:142-143`).
Note each id is a `uint8` shifted into a 16-bit field, so the upper byte of every
slot is always zero.

`receive()` at `:160` — required to receive native from the ERC20→native swap.

---

## 16. Periphery: `Permit2Proxy.sol`

**Contract** `Permit2Proxy is WithdrawablePeriphery` (`src/Periphery/Permit2Proxy.sol:16`),
version `1.0.4`. Lets a user authorise a diamond call by **signature** instead of a
prior `approve` transaction.

Immutables and constants (`:19-29`):
```solidity
address public immutable LIFI_DIAMOND;
ISignatureTransfer public immutable PERMIT2;
string public constant WITNESS_TYPE_STRING =
    "LiFiCall witness)LiFiCall(address diamondAddress,bytes32 diamondCalldataHash)TokenPermissions(address token,uint256 amount)";
bytes32 public constant WITNESS_TYPEHASH =
    keccak256("LiFiCall(address diamondAddress,bytes32 diamondCalldataHash)");
bytes32 public immutable PERMIT_WITH_WITNESS_TYPEHASH;
```

`struct LiFiCall { address diamondAddress; bytes32 diamondCalldataHash; }` (`:35-38`).
Error `CallToDiamondFailed(bytes)` `0x0e971f12`.

**The witness type string is EIP-712 sensitive.** Its shape —
`"LiFiCall witness)"` followed by the referenced struct definitions in alphabetical
order — is exactly what Permit2's `PermitHash` expects when it concatenates the
stub with the caller-supplied suffix. Changing whitespace or ordering changes the
digest and invalidates every signature.

The constructor (`:46`) builds `PERMIT_WITH_WITNESS_TYPEHASH` by hashing
`PermitHash._PERMIT_TRANSFER_FROM_WITNESS_TYPEHASH_STUB` concatenated with
`WITNESS_TYPE_STRING` (`:54-59`). **No zero-address validation.**

#### `callDiamondWithEIP2612Signature(address tokenAddress, uint256 amount, uint256 deadline, uint8 v, bytes32 r, bytes32 s, bytes calldata diamondCalldata) public payable returns (bytes memory)` — `:77`

For tokens implementing EIP-2612 natively.

1. `try ERC20Permit(tokenAddress).permit(msg.sender, address(this), amount, deadline, v, r, s)` (`:87-96`).
   **`owner` is hardcoded to `msg.sender`** — the signer must be the caller, which
   is what stops a frontrunner from replaying the permit with different calldata.
2. **Both catch blocks swallow the failure if the allowance is already sufficient**
   (`:97-111`). This defends against the classic permit-frontrunning griefing
   attack: an attacker submits your permit first, so your transaction's `permit`
   reverts as "nonce used" — but the approval it granted is still there, so the
   call proceeds. Only if the allowance is genuinely insufficient does it rethrow
   (as a string via `revert(reason)`, or raw via `LibUtil.revertWith`).
3. Pull the tokens (`:114-119`), `maxApproveERC20` to the diamond (`:122`),
   `_executeCalldata` (`:125`).

#### `callDiamondWithPermit2(bytes calldata _diamondCalldata, ISignatureTransfer.PermitTransferFrom calldata _permit, bytes calldata _signature) external payable returns (bytes memory)` — `:136`

`PERMIT2.permitTransferFrom(_permit, {to: address(this), requestedAmount: _permit.permitted.amount}, msg.sender, _signature)` (`:141-149`).

**The signature covers the token, the amount, the spender, a nonce and a deadline —
but *not* the calldata.** Safety comes entirely from passing `msg.sender` as the
owner (`:147`): only the signer can submit it, so nobody else can pair the
signature with different calldata. This is stated at `:130-132`.

#### `callDiamondWithPermit2Witness(bytes calldata _diamondCalldata, address _signer, ISignatureTransfer.PermitTransferFrom calldata _permit, bytes calldata _signature) external payable returns (bytes memory)` — `:168`

**The genuinely gasless path.** Builds `LiFiCall(LIFI_DIAMOND, keccak256(_diamondCalldata))`
(`:174-177`), hashes it with `WITNESS_TYPEHASH` (`:179`), and calls
`PERMIT2.permitWitnessTransferFrom(..., _signer, witness, WITNESS_TYPE_STRING, _signature)` (`:181-191`).

Here `_signer` is an arbitrary address, so **a relayer can submit on the user's
behalf**. That is safe precisely because the witness binds the signature to both
the diamond address and the exact calldata hash: a relayer that alters one byte of
the calldata produces a different witness and Permit2 rejects the signature.
Replay is prevented by Permit2's own nonce bitmap.

#### `getPermit2MsgHash(bytes calldata _diamondCalldata, address _assetId, uint256 _amount, uint256 _nonce, uint256 _deadline) external view returns (bytes32)` — `:209`
Off-chain helper that reconstructs the EIP-712 digest a wallet must sign.
Composes `_getTokenPermissionsHash` (`:246`), `_getWitnessHash` (`:259`) and
`_getPermitWitnessTransferFromHash` (`:265`), the last of which does the
`keccak256("\x19\x01" ‖ domainSeparator ‖ dataHash)` assembly (`:284-285`).

#### `_executeCalldata(bytes memory diamondCalldata) internal returns (bytes memory)` — `:288`
`LIFI_DIAMOND.call{value: msg.value}(diamondCalldata)`; on failure reverts
`CallToDiamondFailed(data)`, wrapping rather than bubbling the diamond's error.

#### Permit2 nonce helpers

Adapted from flood-protocol's `Permit2NonceFinder` (`:303-305`). Permit2 uses an
**unordered** nonce bitmap: `nonce >> 8` selects a 256-bit word, `nonce & 0xff`
selects the bit.

| Function | Line | Behaviour |
|---|---|---|
| `nextNonce(address owner) external view` | `:313` | first free nonce from 0 |
| `nextNonceAfter(address owner, uint256 start) external view` | `:323` | advances past `start`, rolling to the next word when `pos == 255` (`:329-336`) |
| `_nextNonce(address, uint248 word, uint8 pos) internal view` | `:344` | skips full words (`bitmap == type(uint256).max`), shifts past `pos`, then scans for the first zero bit |
| `_nonceFromWordAndPos(uint248 word, uint8 pos) internal pure` | `:377` | `(word << 8) | pos` |

**Gotcha.** `_nextNonce`'s `while (true)` has no upper bound; with every word
full it would loop until out of gas. Only reachable after 2^248 nonces, so
theoretical.

`receive()` at `:388` — required for native refunds from the diamond.

---

## 17. Periphery: `Patcher.sol`

**Contract** `Patcher` (`src/Periphery/Patcher.sol:17`), version `1.0.1`. **No
inheritance, no owner, no access control.**

**The problem it solves.** A route is built off-chain, but some amounts are not
knowable until execution: the exact output of a preceding swap, a balance after a
rebase, a bridge fee quoted at fill time. Rather than re-encode the calldata,
`Patcher` fetches the value at execution time with a `staticcall` and overwrites
32-byte words at given offsets before making the call.

Errors (`:21-36`): `FailedToGetDynamicValue()` `0x97113939`,
`MismatchedArrayLengths()` `0x568efce2`, `InvalidPatchOffset()` `0xaaf90328`,
`CallExecutionFailed()` `0x6b3b6576`, `ZeroAddress()` `0xd92e233d`,
`InvalidReturnDataLength()` `0x3ad0505d`.

Events: `PatchExecuted(address indexed caller, address indexed finalTarget, uint256 value, bool success, uint256 returnDataLength)` (`:46`),
`TokensDeposited(address indexed caller, address indexed tokenAddress, uint256 amount, address indexed finalTarget)` (`:59`).

### The four external functions

| Function | Line | Deposits? | Values |
|---|---|---|---|
| `executeWithDynamicPatches(address valueSource, bytes calldata valueGetter, address finalTarget, uint256 value, bytes calldata data, uint256[] calldata offsets, bool delegateCall)` | `:78` | no | one |
| `depositAndExecuteWithDynamicPatches(address tokenAddress, ...same...)` | `:128` | yes | one |
| `depositAndExecuteWithMultiplePatches(address tokenAddress, address[] valueSources, bytes[] valueGetters, ..., uint256[][] offsetGroups, bool)` | `:183` | yes | many |
| `executeWithMultiplePatches(address[] valueSources, bytes[] valueGetters, ..., uint256[][] offsetGroups, bool)` | `:228` | no | many |

All four are `payable`, all four emit `PatchExecuted`, and the deposit variants
reset the approval to zero afterwards (`:151`, `:206`).

### Internals

**`_getDynamicValue(address valueSource, bytes calldata valueGetter) internal view returns (uint256)` — `:298`**
`valueSource.staticcall(valueGetter)`; failure → `FailedToGetDynamicValue()`;
**return data must be exactly 32 bytes** → `InvalidReturnDataLength()` (`:308`).
`staticcall` means the value source cannot mutate state or re-enter.

**`_applyPatch(bytes memory patchedData, uint256 offset, uint256 dynamicValue) internal pure` — `:317`**

```solidity
if (offset + 32 > patchedData.length) revert InvalidPatchOffset();   // :322
assembly {
    let position := add(add(patchedData, 32), offset)                // :326
    mstore(position, dynamicValue)                                   // :329
}
```

The bounds check is the whole safety argument for the assembly: `add(patchedData, 32)`
skips the length word, and the check guarantees the 32-byte write lands inside the
array. `offset + 32` cannot overflow in practice because `patchedData.length` is
bounded by calldata size.

**`_applyPatches` — `:450`**, **`_processPatches` — `:427`**: loops over offsets and
over (source, getter, offsets) triples respectively.

**`_executeCall(address finalTarget, uint256 value, bytes memory patchedData, bool delegateCall) internal returns (bool, bytes memory)` — `:340`**
`delegatecall` or `call{value: value}`; on failure bubbles via `LibUtil.revertWith`
or reverts `CallExecutionFailed()`.

**`_depositAndApprove(address tokenAddress, address finalTarget) private` — `:271`**
Reads `IERC20(tokenAddress).balanceOf(msg.sender)` and pulls **the entire
balance** (`:276-284`), then max-approves `finalTarget` and emits `TokensDeposited`.

**`_executeWithDynamicPatches` — `:365`** validates non-zero `valueSource` and
`finalTarget` → `ZeroAddress()`, and non-empty `offsets` → `InvalidPatchOffset()`.
**`_executeWithMultiplePatches` — `:394`** additionally requires all three arrays
to be the same length → `MismatchedArrayLengths()`, and checks each source and
offset group. Note both loop with `uint8 i` (`:411`, `:433`), silently capping the
number of value sources at 255 — beyond that the loop wraps and never terminates.

### The security argument, and its limits

The contract's own header (`:12-15`) and the deposit functions' comments
(`:109-117`, `:164-172`) are unusually candid, and should be read as normative:

- **No refunds.** Excess tokens or ETH stay in the contract and **can be stolen by
  anyone**.
- **The deposit variants transfer the caller's entire token balance**, not just
  what is needed.
- **Frontrunning.** Because targets are not whitelisted and there is no access
  control, any token approved to this contract can be taken by an attacker who
  frontruns with their own `depositAndExecute…` call naming a malicious target.

So why is `delegatecall` acceptable here? Because the contract holds no persistent
state worth corrupting — no owner, no mappings, no configuration. A `delegatecall`
executes in the context of a contract whose storage is empty. The danger is
entirely in the *approvals* users grant it, not in the contract's own state.

**The correct usage is transient**: the diamond calls `Patcher` as a whitelisted
route step within a single atomic transaction, so no approval outlives the
transaction. Granting a standing approval to `Patcher` from an EOA is unsafe, and
the contract says so.

---

## 18. Periphery: `LiFiDEXAggregator.sol`

**Contract** `LiFiDEXAggregator is WithdrawablePeriphery`
(`src/Periphery/LiFiDEXAggregator.sol:62`), version `1.12.0`. 1,820 lines — the
largest file in the repository. Forked from Sushi's `RouteProcessor4`
(`:56-59`) and extended with seven more DEX families.

**What it is.** A byte-code interpreter for swap routes. The off-chain router
compiles a path — possibly splitting across several pools and several protocols —
into a compact byte string, and this contract walks it. Compared with encoding a
`SwapData[]` array, the stream format saves substantial calldata, which is the
dominant cost on L2s.

### 18.1 Constants

File-level (`:19-54`):

| Constant | Value | Meaning |
|---|---|---|
| `NATIVE_ADDRESS` | `0xEeee…EEeE` | native sentinel **in this file only** — not `LibAsset.NULL_ADDRESS` |
| `IMPOSSIBLE_POOL_ADDRESS` | `0x…0001` | sentinel meaning "no callback expected" |
| `INTERNAL_INPUT_SOURCE` | `0x…0000` | "funds are already at the pool" |
| `LOCKED` / `NOT_LOCKED` | 2 / 1 | reentrancy states |
| `PAUSED` / `NOT_PAUSED` | 2 / 1 | pause states |
| `MIN_SQRT_RATIO` | `4295128739` | Uniswap V3 lower price bound |
| `MAX_SQRT_RATIO` | `1461446703485210103287273052203988822378723970342` | upper bound |
| `IZUMI_LEFT_MOST_PT` / `IZUMI_RIGHT_MOST_PT` | `-800000` / `800000` | iZiSwap point bounds |
| `DIRECTION_TOKEN0_TO_TOKEN1` | 1 | direction flag |
| `CALLBACK_ENABLED` | 1 | Velodrome flashloan-callback flag |
| `KATANA_V3_SWAP_EXACT_IN` | `hex"00"` | Katana router command byte |

Pool types (`:41-51`): `UNIV2=0`, `UNIV3=1`, `WRAP_NATIVE=2`, `BENTO_BRIDGE=3`,
`TRIDENT=4`, `CURVE=5`, `VELODROME_V2=6`, `ALGEBRA=7`, `IZUMI_V3=8`,
`SYNCSWAP=9`, `KATANA_V3=10`.

### 18.2 State, modifiers, admin

```solidity
IBentoBoxMinimal public immutable BENTO_BOX;      // :94
mapping(address => bool) public priviledgedUsers; // :95   [sic]
address private lastCalledPool;                   // :96
uint8 private unlocked = NOT_LOCKED;              // :98
uint8 private paused = NOT_PAUSED;                // :99
```

`lastCalledPool` is the callback-authentication mechanism (§18.6).

- **`modifier lock()` — `:100`.** Checks *both* `unlocked` and `paused`, so pausing
  and reentrancy produce distinct errors: `RouteProcessorLocked()` / `RouteProcessorPaused()`.
- **`modifier onlyOwnerOrPriviledgedUser()` — `:108`.** → `CallerNotOwnerOrPriviledged()`.
- **`constructor(address _bentoBox, address[] memory priviledgedUserList, address _owner)` — `:114`.**
  Zero owner → `InvalidConfig()`; sets `lastCalledPool = IMPOSSIBLE_POOL_ADDRESS` (`:123`).
- **`setPriviledge(address user, bool priviledge) external onlyOwner` — `:130`.**
- **`pause()` — `:134`** / **`resume()` — `:138`**, both `onlyOwnerOrPriviledgedUser`.
- **`receive() external payable` — `:143`**, for native unwrapping.

### 18.3 Entry points

#### `processRoute(address tokenIn, uint256 amountIn, address tokenOut, uint256 amountOutMin, address to, bytes memory route) external payable lock returns (uint256 amountOut)` — `:151`
Thin `lock`ed wrapper over `processRouteInternal`.

#### `transferValueAndprocessRoute(address payable transferValueTo, uint256 amountValueTransfer, ...same...) external payable lock` — `:178`
Sends `amountValueTransfer` native to `transferValueTo` **first** (`:188`), then
processes the route. Used to pay a bridge/protocol fee out of the same transaction.
(The lowercase `p` in the name is in the source.)

#### `processRouteInternal(...) private returns (uint256 amountOut)` — `:206`

The interpreter, and the security envelope around it:

1. **Snapshot** `balanceInInitial` (of `msg.sender`) and `balanceOutInitial` (of `to`)
   (`:214-219`). Native input snapshots as `0`.
2. **Interpret** (`:222-241`): create a stream, then `while (stream.isNotEmpty())`
   read one command byte and dispatch:

   | Code | Handler | Meaning |
   |---:|---|---|
   | 1 | `processMyERC20` | spend this contract's balance of a token |
   | 2 | `processUserERC20` | spend `amountIn` pulled from `msg.sender` |
   | 3 | `processNative` | spend this contract's native balance |
   | 4 | `processOnePool` | funds already sit at the pool |
   | 5 | `processInsideBento` | spend a BentoBox balance |
   | 6 | `applyPermit` | run an EIP-2612 permit for `tokenIn` |
   | else | — | `UnknownCommandCode()` |

   `realAmountIn` is captured from step 0 only (`:229`, `:234`), for the event.
3. **Input check** (`:243-250`): `balanceInFinal + amountIn < balanceInInitial` →
   `MinimalInputBalanceViolation`. In words: the route may not have taken more than
   `amountIn` from the sender.
4. **Output check** (`:252-258`): `balanceOutFinal < balanceOutInitial + amountOutMin`
   → `MinimalOutputBalanceViolation`. The slippage guarantee, measured on the
   **recipient's** balance.
5. **`emit Route(...)`** (`:262-270`).

**These two balance checks are the entire safety model.** Every individual pool
call is unvalidated; what is guaranteed is only that the sender lost at most
`amountIn` and the recipient gained at least `amountOutMin`.

**The documented weakness** (`:1197-1202`): if `to` is a contract with
side-effects — a hook or callback recipient that can move its own balance — the
balance-difference check is no longer meaningful. The source says it plainly:
"never trust balance-based slippage protection for callback recipients."

### 18.4 Route byte layout

The stream is read strictly left to right by `InputStream` (§18.7). There are no
lengths or tags — the reader's position is the only state, so a malformed route
misparses rather than reverting cleanly.

```
route := command+

command :=
   0x01 <token:20>  <distribution>          # processMyERC20
 | 0x02 <token:20>  <distribution>          # processUserERC20
 | 0x03             <distribution>          # processNative
 | 0x04 <token:20>  <swap>                  # processOnePool  (no share list)
 | 0x05 <token:20>  <distribution>          # processInsideBento
 | 0x06 <value:32> <deadline:32> <v:1> <r:32> <s:32>   # applyPermit

distribution := <num:1> ( <share:2> <swap> ) * num

swap := <poolType:1> <poolTypeSpecificData>
```

Per-pool-type payloads:

| Pool type | Byte layout after the type byte |
|---|---|
| `0` UniV2 | `pool:20`, `direction:1`, `to:20`, `fee:3` (in 1e-6) |
| `1` UniV3 | `pool:20`, `direction:1`, `recipient:20` |
| `2` WrapNative | `directionAndFake:1`, `to:20`, then `wrapToken:20` **only if wrapping** |
| `3` BentoBridge | `direction:1`, `to:20` |
| `4` Trident | `pool:20`, `swapData:bytes` (length-prefixed) |
| `5` Curve | `pool:20`, `poolType:1`, `fromIndex:1`, `toIndex:1`, `to:20`, `tokenOut:20` |
| `6` VelodromeV2 | `pool:20`, `direction:1`, `to:20`, `callback:1` |
| `7` Algebra | `pool:20`, `direction:1`, `recipient:20`, `supportsFeeOnTransfer:1` |
| `8` iZiSwap | `pool:20`, `direction:1`, `recipient:20` |
| `9` SyncSwap | `pool:20`, `to:20`, `withdrawMode:1`, `isV1Pool:1`, then `vault:20` **only if V1** |
| `10` KatanaV3 | `pool:20`, `direction:1`, `recipient:20` |

**Note the two variable-shape encodings.** `WRAP_NATIVE` reads a further 20 bytes
only when the low bit of `directionAndFake` is set (`:423-425`), and `SYNCSWAP`
reads a vault address only when `isV1Pool` (`:849-851`). An encoder that gets
either wrong desynchronises the whole remaining stream.

**Share arithmetic** (`distributeAndSwap`, `:354`):

```solidity
uint8 num = stream.readUint8();
for (uint256 i = 0; i < num; ++i) {
    uint16 share = stream.readUint16();
    uint256 amount = (amountTotal * share) / type(uint16).max;   // :364-365
    amountTotal -= amount;
    swap(stream, from, tokenIn, amount);
}
```

Shares are 16-bit fractions of 65535. Because `amountTotal` is decremented as it
goes, the split is of the *remaining* amount, and the conventional encoding gives
the last leg `share = 65535` so it takes everything left. The whole loop is
`unchecked` (`:361`).

### 18.5 Source handlers

| Function | Line | Source of funds |
|---|---|---|
| `applyPermit(address tokenIn, uint256 stream)` | `:276` | reads `value`, `deadline`, `v`, `r`, `s` and calls `safePermit(msg.sender, address(this), …)` |
| `processNative(uint256 stream)` | `:295` | `address(this).balance`, all of it |
| `processMyERC20(uint256 stream)` | `:305` | `balanceOf(this)` **minus 1 wei** (`:310-312`) |
| `processUserERC20(uint256 stream, uint256 amountTotal)` | `:320` | `msg.sender`, exactly `amountTotal` |
| `processOnePool(uint256 stream)` | `:332` | already at the pool; calls `swap(..., 0)` |
| `processInsideBento(uint256 stream)` | `:340` | BentoBox balance **minus 1 wei** |

**The "slot undrain protection" 1-wei subtraction** (`:311`, `:344`) is a gas
optimisation, not a safety measure: leaving one wei keeps the storage slot
non-zero, so the next user pays a 5,000-gas warm write instead of a 20,000-gas
cold one.

**`processOnePool` passes `amountIn = 0`.** The warning at `:328-330` matters:
some Uniswap V3 forks require a non-zero amount for pricing, so this optimisation
is unsafe for V3-style pools.

#### `swap(uint256 stream, address from, address tokenIn, uint256 amountIn) private` — `:377`
Reads the pool-type byte and dispatches to one of eleven handlers (`:384-406`),
else `UnknownPoolType()`.

### 18.6 Pool handlers

#### `wrapNative(...)` — `:414`
`directionAndFake` is a bit field: **bit 0** = wrap (1) vs unwrap (0), **bit 1** =
"fake" (skip the actual WETH call, because a previous step already did it).
Wrapping: optional `deposit{value: amountIn}()`, then transfer out if `to` is not
this contract. Unwrapping: optional pull + `withdraw`, then `safeTransferETH`.

#### `bentoBridge(...)` — `:450`
Into Bento: transfer the token to the BentoBox from wherever, or — for
`INTERNAL_INPUT_SOURCE` — derive the amount as
`balanceOf(BENTO_BOX) + strategyData.balance - totals.elastic` (`:472-475`),
i.e. the un-accounted surplus. Then `BENTO_BOX.deposit`. Out of Bento:
`transfer` then `withdraw`.

#### `swapUniV2(...)` — `:492`

Notable because it computes the output itself rather than trusting the pool:

```solidity
(uint256 r0, uint256 r1, ) = IUniswapV2Pair(pool).getReserves();
if (r0 == 0 || r1 == 0) revert WrongPoolReserves();            // :509
amountIn = IERC20(tokenIn).balanceOf(pool) - reserveIn;        // :514
uint256 amountInWithFee = amountIn * (1_000_000 - fee);
uint256 amountOut = (amountInWithFee * reserveOut) /
                    (reserveIn * 1_000_000 + amountInWithFee); // :516-518
```

Two things: the effective input is re-derived from the pool's **actual balance**
minus its recorded reserve, which makes fee-on-transfer tokens work; and the fee
is a per-pool parameter in millionths rather than the hardcoded 0.3%, so forks
with different fees are supported.

#### `swapTrident(...)` — `:531`
Transfers via BentoBox if needed, then `IPool(pool).swap(swapData)` with an opaque
blob from the stream.

#### `swapUniV3(...)` — `:552`
Validates pool and recipient are non-zero and the pool is not the sentinel
(`:562-566`). Sets `lastCalledPool = pool` (`:575`), calls
`IUniswapV3Pool.swap(recipient, direction, int256(amountIn), direction ? MIN_SQRT_RATIO+1 : MAX_SQRT_RATIO-1, abi.encode(tokenIn))`,
then asserts `lastCalledPool == IMPOSSIBLE_POOL_ADDRESS` (`:583-584`) — proof the
callback ran exactly once.

Note the price limit is set to the extreme, so there is **no per-pool slippage
protection**; only the route-level output check applies.

#### `uniswapV3SwapCallback(int256 amount0Delta, int256 amount1Delta, bytes calldata data) public` — `:596`

The callback-authentication pattern, in full:

```solidity
if (msg.sender != lastCalledPool) revert UniswapV3SwapCallbackUnknownSource();  // :601
int256 amount = amount0Delta > 0 ? amount0Delta : amount1Delta;
if (amount <= 0) revert UniswapV3SwapCallbackNotPositiveAmount();               // :604
lastCalledPool = IMPOSSIBLE_POOL_ADDRESS;                                       // :606
address tokenIn = abi.decode(data, (address));
IERC20(tokenIn).safeTransfer(msg.sender, uint256(amount));                      // :608
```

`lastCalledPool` is set immediately before the swap and cleared here, so the
callback is authenticated by *transient expectation* rather than by a factory
lookup. This is cheaper than computing a CREATE2 pool address and works uniformly
across forks — at the cost of requiring that the value be set on every path that
can trigger a callback, and cleared on every return.

**Fifteen fork aliases** delegate to this one function verbatim, differing only in
name because each fork chose its own callback selector:
`algebraSwapCallback` (`:620`), `pancakeV3SwapCallback` (`:635`),
`ramsesV2SwapCallback` (`:650`), `xeiV3SwapCallback` (`:665`),
`dragonswapV2SwapCallback` (`:680`), `agniSwapCallback` (`:697`),
`fusionXV3SwapCallback` (`:712`), `vvsV3SwapCallback` (`:727`),
`supV3SwapCallback` (`:742`), `zebraV3SwapCallback` (`:757`),
`hyperswapV3SwapCallback` (`:1000`), `laminarV3SwapCallback` (`:1015`),
`xswapCallback` (`:1032`), `rabbitSwapV3SwapCallback` (`:1049`),
`enosysdexV3SwapCallback` (`:1066`).

#### `swapIzumiV3(...)` — `:771` and its callbacks
Validates inputs including `amountIn > type(uint128).max` → `InvalidCallData()`
(`:784`). Calls `swapX2Y` or `swapY2X` with the extreme point bound, then asserts
the sentinel (`:816-818`). `_handleIzumiV3SwapCallback(uint256 amountToPay, bytes calldata data) private` (`:939`)
mirrors the UniV3 pattern with its own three errors; `swapX2YCallback` (`:968`)
and `swapY2XCallback` (`:983`) pick the correct amount from the two arguments.

#### `swapSyncSwap(...)` — `:828`
Reads a `withdrawMode` byte (0 = vault decides, 1 = raw ETH, 2 = WETH) and
rejects `> 2` (`:847`). For V1 pools the tokens go to a **vault** and
`ISyncSwapVault.deposit(tokenIn, pool)` credits the pool (`:862-864`); for V2 they
go straight to the pool. Then `ISyncSwapPool.swap(abi.encode(tokenIn, to, withdrawMode), from, address(0), "")`.

#### `swapKatanaV3(...)` — `:877`
The only handler that goes through a **router** rather than the pool. It reads the
pool's `governance()`, asks it for `getRouter()` (`:894-895`), reads `fee()` and
the output token, transfers the input **to the router**, builds a V3 path
`abi.encodePacked(tokenIn, fee, tokenOut)` (`:913`), and calls
`execute(hex"00", inputs)` with `payerIsUser = false` (`:917-931`). The callback
lives in the router, so `lastCalledPool` is not used here.

#### `swapCurve(...)` — `:1079`
Two Curve ABIs. `poolType == 0` uses `ICurve.exchange` which **returns** the
output; anything else uses `ICurveLegacy.exchange` which returns nothing, so the
handler brackets the call with balance reads (`:1116-1123`). Native input is sent
as `value`. Approvals go through `Approve.approveSafe` (§18.7). Output is
forwarded to `to` when that is not this contract (`:1127-1133`).

#### `swapVelodromeV2(...)` — `:1142`
For `INTERNAL_INPUT_SOURCE`, re-derives `amountIn` from the pool's balance minus
its reserve (`:1155-1163`). Asks the pool `getAmountOut(amountIn, tokenIn)`
(`:1172-1175`) — Velodrome's stable/volatile curve has no simple closed form worth
reimplementing — then calls `swap` with a callback payload only if the flag byte
is set. The long comment at `:1186-1202` enumerates the safety properties it
relies on and the callback caveat quoted in §18.3.

#### `swapAlgebra(...)` — `:1219`
Reads a `supportsFeeOnTransfer` flag and picks between
`swapSupportingFeeOnInputTokens` (`:1254-1261`) and plain `swap` (`:1263-1269`).
Same `lastCalledPool` bracket, asserting with `AlgebraSwapUnexpected()`.

### 18.7 Embedded libraries and interfaces

Everything below `:1275` is support code declared inline.

**`library InputStream` — `:1618`.** The stream is a two-word memory structure:
word 0 is the current position, word 1 the end (`createStream`, `:1622-1632`).
`isNotEmpty()` (`:1637`) compares them. Each reader advances the position **first**
and then `mload`s, which is why a `readUint8` yields the byte at the *new*
position's low end:

```solidity
function readUint8(uint256 stream) internal pure returns (uint8 res) {
    assembly {
        let pos := mload(stream)
        pos := add(pos, 1)     // advance
        res := mload(pos)      // load 32 bytes; implicit truncation to uint8
        mstore(stream, pos)
    }
}
```

Readers: `readUint8` `:1650`, `readUint16` `:1662`, `readUint24` `:1674`,
`readUint32` `:1686`, `readUint` `:1698`, `readBytes32` `:1710`,
`readAddress` `:1722`, `readBytes` `:1734` (length-prefixed, returns a pointer
into the stream without copying).

**No bounds checking anywhere.** Reading past the end silently returns whatever
memory follows. Safety rests entirely on the two balance checks in
`processRouteInternal`.

**`library Approve` — `:1746`.** `approveStable` (`:1753`) does a raw `call` to
`approve` and accepts either an empty return or `true`. `approveSafe` (`:1772`)
tries once, and on failure zeroes the allowance and retries — the USDT pattern.

**`struct Rebase` / `struct StrategyData` / `library RebaseLibrary`** (`:1784-1820`) —
BentoBox share↔amount conversion (`toBase`, `toElastic`).

**Inline interfaces** (`:1279-1616`): `IBentoBoxMinimal`, `IPool` (Trident, with
its `Swap` event and `TokenAmount` struct), `ITridentCLPool`, `IUniswapV2Pair`
(full ERC20 + pair surface), `IUniswapV3Pool`, `IWETH`.

### 18.8 Aggregator error table

| Selector | Error | Where |
|---|---|---|
| `0x963b34a5` | `MinimalOutputBalanceViolation(uint256)` | `:256` |
| `0x583af586` | `MinimalInputBalanceViolation(uint256,uint256)` | `:247` |
| `0x18e914e9` | `RouteProcessorLocked()` | `:101` |
| `0xf8c06269` | `RouteProcessorPaused()` | `:102` |
| `0xde34a2ae` | `CallerNotOwnerOrPriviledged()` | `:110` |
| `0xb926a1f0` | `UnknownCommandCode()` | `:238` |
| `0xbfa9e1b5` | `UnknownPoolType()` | `:406` |
| `0x38cfb4e2` | `UniswapV3SwapUnexpected()` | `:584` |
| `0xf00b64c1` | `UniswapV3SwapCallbackUnknownSource()` | `:602` |
| `0x6ef7645e` | `UniswapV3SwapCallbackNotPositiveAmount()` | `:604` |
| `0xdf683a98` | `WrongPoolReserves()` | `:509`, `:1158` |
| `0xef7e5398` | `AlgebraSwapUnexpected()` | `:1273` |
| `0xc2284f37` | `IzumiV3SwapUnexpected()` | `:817` |
| `0xe6151f96` | `IzumiV3SwapCallbackUnknownSource()` | `:944` |
| `0x28ecba32` | `IzumiV3SwapCallbackNotPositiveAmount()` | `:948` |

Plus `InvalidCallData()` and `InvalidConfig()` from `GenericErrors`.

Event: `Route(address indexed from, address to, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOutMin, uint256 amountOut)` (`:68-76`).

---

## 19. Cross-cutting tables

### 19.1 Namespaced storage slots

See [§3](#3-namespaced-storage-the-slot-table). Four namespaces:
`diamond.standard.diamond.storage`, `com.lifi.library.access.management`,
`com.lifi.library.allow.list`, `com.lifi.reentrancyguard`.

### 19.2 Periphery contracts with conventional storage

These are standalone contracts, so they use ordinary slots.

| Contract | Slot 0 | Slot 1 | Slot 2+ | Immutables (not in storage) |
|---|---|---|---|---|
| `TransferrableOwnership` | `owner` | `pendingOwner` | — | — |
| `Executor` | `owner` | `pendingOwner` | `erc20Proxy` | — |
| `ERC20Proxy` | `owner` | `pendingOwner` | `authorizedCallers` | — |
| `FeeCollector` | `owner` | `pendingOwner` | `_balances`, `_lifiBalances` | — |
| `FeeForwarder` | `owner` | `pendingOwner` | — | — |
| `TokenWrapper` | `owner` | `pendingOwner` | — | `WRAPPED_TOKEN`, `CONVERTER`, `USE_CONVERTER`, `SWAP_RATIO_MULTIPLIER` |
| `LidoWrapper` | `owner` | `pendingOwner` | — | `ST_ETH`, `WST_ETH_ADDRESS` |
| `OutputValidator` | `owner` | `pendingOwner` | — | — |
| `GasZipPeriphery` | `owner` | `pendingOwner` | — | `GAS_ZIP_ROUTER`, `LIFI_DIAMOND` |
| `Permit2Proxy` | `owner` | `pendingOwner` | — | `LIFI_DIAMOND`, `PERMIT2`, `PERMIT_WITH_WITNESS_TYPEHASH` |
| `Patcher` | — | — | — | none (**stateless**) |
| `LiFiDEXAggregator` | `owner` | `pendingOwner` | `priviledgedUsers`, `lastCalledPool`+`unlocked`+`paused` (packed) | `BENTO_BOX` |
| Receivers (all five) | `owner` | `pendingOwner` | — | executor + bridge address (+ `recoverGas` for Stargate) |
| `LiFiTimelockController` | (OZ `AccessControl` layout) | | `diamond` | — |

In `LiFiDEXAggregator`, `lastCalledPool` (20 bytes) + `unlocked` (1) + `paused` (1)
share a single slot — 22 of 32 bytes used.

### 19.3 Complete event reference

| Event | Declared | Emitted by | Indexed fields |
|---|---|---|---|
| `DiamondCut(FacetCut[],address,bytes)` | `LibDiamond.sol:84` | `LibDiamond.diamondCut:132` | none |
| `OwnershipTransferred(address,address)` | `LibDiamond.sol:79`, `IERC173.sol:10` | `setContractOwner:90`, `confirmOwnershipTransfer:54` | both |
| `OwnershipTransferRequested(address,address)` | `TransferrableOwnership.sol:20` | `transferOwnership:40` | both |
| `AccessGranted(address,bytes4)` | `LibAccess.sol:21` | `addAccess:46` | both |
| `AccessRevoked(address,bytes4)` | `LibAccess.sol:22` | `removeAccess:55` | both |
| `AssetSwapped(bytes32,address,address,address,uint256,uint256,uint256)` | `LibSwap.sol:41` | `LibSwap.swap:97` | **none** |
| `LiFiTransferStarted(BridgeData)` | `ILiFi.sol:25` | every bridge facet | none |
| `LiFiTransferCompleted(bytes32,address,address,uint256,uint256)` | `ILiFi.sol:27` | `Executor:204` | `transactionId` |
| `LiFiTransferRecovered(bytes32,address,address,uint256,uint256)` | `ILiFi.sol:35` | `ReceiverAcrossV3:115`, `ReceiverAcrossV4:111`, `ReceiverStargateV2:159/179/204/230`, `ReceiverChainflip:143/165` | `transactionId` |
| `LiFiGenericSwapCompleted(...)` | `ILiFi.sol:43` | `GenericSwapFacetV3` | `transactionId` |
| `BridgeToNonEVMChain(bytes32,uint256,bytes)` | `ILiFi.sol:55` | non-EVM bridge facets | both |
| `BridgeToNonEVMChainBytes32(bytes32,uint256,bytes32)` | `ILiFi.sol:60` | non-EVM bridge facets | both |
| `TokensWithdrawn(address,address,uint256)` | `WithdrawablePeriphery.sol:19` | `withdrawToken:40` | none |
| `ERC20ProxySet(address)` | `Executor.sol:33` | constructor `:81` | yes |
| `AuthorizationChanged(address,bool)` | `ERC20Proxy.sol:17` | constructor `:33`, `setAuthorizedCaller:45` | `caller` |
| `FeesCollected(address,address,uint256,uint256)` | `FeeCollector.sol:25` | `:63`, `:90` | token, integrator |
| `FeesWithdrawn(address,address,uint256)` | `FeeCollector.sol:31` | `:107`, `:126` | token, to |
| `LiFiFeesWithdrawn(address,address,uint256)` | `FeeCollector.sol:36` | `:143`, `:161` | token, to |
| `FeesForwarded(address,FeeDistribution[])` | `FeeForwarder.sol:32` | `:76`, `:125` | token |
| `OutputValidated(address,address,uint256)` | `OutputValidator.sol:27` | `:75`, `:89`, `:143` | token, wallet |
| `PatchExecuted(address,address,uint256,bool,uint256)` | `Patcher.sol:46` | `:97`, `:153`, `:208`, `:247` | caller, finalTarget |
| `TokensDeposited(address,address,uint256,address)` | `Patcher.sol:59` | `_depositAndApprove:290` | caller, token, finalTarget |
| `Route(address,address,address,address,uint256,uint256,uint256)` | `LiFiDEXAggregator.sol:68` | `processRouteInternal:262` | from, tokenIn, tokenOut |
| `DiamondAddressUpdated(address)` | `LiFiTimelockController.sol:26` | `:57`, `:67` | yes |
| `ContractSelectorWhitelistChanged(address,bytes4,bool)` | `IWhitelistManagerFacet.sol:13` | `WhitelistManagerFacet` | all three |

### 19.4 Per-contract error tables

`GenericErrors` in [§8](#8-errors). Contract-local errors:

| Contract | Errors (selector) |
|---|---|
| `LibDiamond` | `IncorrectFacetCutAction` `0xe548e6b5`, `NoSelectorsInFace` `0x7bc55950`, `FunctionAlreadyExists` `0xa023275d`, `FacetAddressIsZero` `0xc68ec83a`, `FacetAddressIsNotZero` `0x79c9df22`, `FacetContainsNoCode` `0xe3500600`, `FunctionDoesNotExist` `0xa9ad62f8`, `FunctionIsImmutable` `0xc3c5ec37`, `InitZeroButCalldataNotEmpty` `0x98116860`, `CalldataEmptyButInitNotZero` `0x42200566`, `InitReverted` `0xc53ebed5` |
| `LibBytes` | `SliceOverflow` `0x47aaf07a`, `SliceOutOfBounds` `0x3b99b53d`, `AddressOutOfBounds` `0x8f95a28a`, `HexLengthInsufficient` `0x2194895a`, `NotAnAddress(bytes32)` `0x479ef3f7` |
| `TransferrableOwnership` | `UnAuthorized` `0xbe245983`, `NoNullOwner` `0x1beca374`, `NewOwnerMustNotBeSelf` `0xbf1ea9fb`, `NoPendingOwnershipTransfer` `0x75cdea12`, `NotPendingOwner` `0x1853971c` |
| `ReentrancyGuard` | `ReentrancyError` `0x29f745a7` |
| `FeeCollector` | `TransferFailure` `0xf7e6817a` (unused), `NotEnoughNativeForFees` `0x840a2adf` |
| `LidoWrapper` | `ContractNotYetReadyForMainnet` `0x8bb9ffc7` |
| `GasZipPeriphery` | `TooManyChainIds` `0x1ee194c6`, `SwapOutputMustBeNative` `0x3e457d68` |
| `Permit2Proxy` | `CallToDiamondFailed(bytes)` `0x0e971f12` |
| `Patcher` | `FailedToGetDynamicValue` `0x97113939`, `MismatchedArrayLengths` `0x568efce2`, `InvalidPatchOffset` `0xaaf90328`, `CallExecutionFailed` `0x6b3b6576`, `ZeroAddress` `0xd92e233d`, `InvalidReturnDataLength` `0x3ad0505d` |
| `LiFiDEXAggregator` | see [§18.8](#188-aggregator-error-table) |

### 19.5 Selector tables for the periphery

| Contract | Function | Selector |
|---|---|---|
| `ERC20Proxy` | `setAuthorizedCaller(address,bool)` | `0x454bbd29` |
| | `transferFrom(address,address,address,uint256)` | `0x15dacbea` |
| `FeeCollector` | `collectTokenFees(address,uint256,uint256,address)` | `0xeedd56e1` |
| | `collectNativeFees(uint256,uint256,address)` | `0xe0cbc5f2` |
| | `withdrawIntegratorFees(address)` | `0xbd0b380b` |
| | `batchWithdrawIntegratorFees(address[])` | `0xe5d64766` |
| | `withdrawLifiFees(address)` | `0x461ad4f5` |
| | `batchWithdrawLifiFees(address[])` | `0x64bc5be1` |
| | `getTokenBalance(address,address)` | `0xc489744b` |
| | `getLifiTokenBalance(address)` | `0x0fe97f70` |
| `TokenWrapper` | `deposit()` | `0xd0e30db0` |
| | `withdraw()` | `0x3ccfd60b` |
| `LidoWrapper` | `wrapStETHToWstETH(uint256)` | `0x24dd6483` |
| | `unwrapWstETHToStETH(uint256)` | `0xa816ca92` |
| `OutputValidator` | `validateNativeOutput(uint256,address)` | `0x5d865df2` |
| | `validateERC20Output(address,uint256,address)` | `0x27444dab` |

Functions taking `LibSwap.SwapData[]` or other structs (`Executor`, the Receivers,
`Permit2Proxy`, `Patcher`, `LiFiDEXAggregator`) have tuple-typed signatures; build
those selectors from the compiled ABI rather than by hand.

### 19.6 `ReentrancyGuard` inheritors

`Periphery/Executor.sol` plus 29 facets: `AcrossFacet`, `AcrossFacetV4`,
`AcrossV4SwapFacet`, `AllBridgeFacet`, `ArbitrumBridgeFacet`, `ChainflipFacet`,
`DeBridgeDlnFacet`, `EcoFacet`, `GardenFacet`, `GasZipFacet`, `GlacisFacet`,
`GnosisBridgeFacet`, `LiFiIntentEscrowFacetV2`, `MayanFacet`, `MegaETHBridgeFacet`,
`NEARIntentsFacet`, `OmniBridgeFacet`, `OptimismBridgeFacet`, `PaxosTransitFacet`,
`PolygonBridgeFacet`, `PolymerCCTPFacet`, `RelayDepositoryFacet`, `SquidFacet`,
`StargateFacetV2`, `SupersetFacet`, `SymbiosisFacet`, `ThorSwapFacet`, `UnitFacet`.
