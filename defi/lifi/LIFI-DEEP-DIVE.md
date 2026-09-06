# LI.FI Deep Dive

A code-level walkthrough of the `lifinance/contracts` repo (cloned at `lifi/contracts`, `.git` removed).
Every `path:line` below is relative to `lifi/contracts/` and was checked against the cloned source.
Read this with the files open. The goal: after this you can trace any LI.FI transaction from the user's
wallet, through the Diamond, into a bridge, across chains, through the destination Receiver + Executor,
and into the final receiver, and you can say exactly which line does what.

```
lifi/contracts/src
├── LiFiDiamond.sol              the single proxy every user talks to
├── Libraries/                   LibDiamond, LibAsset, LibSwap, LibAllowList, LibAccess, LibBytes, ...
├── Helpers/                     SwapperV2, Validatable, ReentrancyGuard, LiFiData, ownership helpers
├── Facets/                      one facet per bridge + admin facets (cut/loupe/owner/pause/whitelist/...)
├── Periphery/                   standalone contracts: Executor, Receiver*, Permit2Proxy, FeeCollector, DEX aggregator, ...
├── Interfaces/                  ILiFi (BridgeData + events) and one interface per external protocol
├── Errors/GenericErrors.sol     all custom errors
└── Security/LiFiTimelockController.sol
```

---

## 0. Mental model

### 0.1 What an aggregator is

LI.FI is a *bridge and DEX aggregator*. Nothing on-chain decides a route. The route is computed off-chain by
the LI.FI API (`/quote`), which returns a fully-populated `transactionRequest` (`docs/LiFiIntentEscrowFacetV2.md`
shows the shape: `{ to: <diamond>, data: 0x..., value }`). The contracts are an *execution engine* that runs
that pre-computed route atomically on each chain. `README.md` ("How It Works") describes it as:

> A single transaction can contain multiple steps (e.g. AAVE on Polygon -> DAI on Polygon using Paraswap ->
> DAI on Avalanche using Stargate -> SPELL on Avalanche using Paraswap)

### 0.2 The three legs

```
SOURCE CHAIN                                   DESTINATION CHAIN
┌────────────────────────────────────────┐     ┌─────────────────────────────────────────┐
│ user ──► LiFiDiamond (facet)           │     │ bridge delivers tokens + message        │
│   leg 1: source swaps (LibSwap,        │     │   ──► Receiver<Bridge> (periphery)      │
│          whitelisted DEX calls)        │ ══► │   leg 3: destination swaps via Executor │
│   leg 2: hand tokens to bridge         │     │   ──► final receiver (user wallet)      │
│          contract + emit event         │     │   fallback: raw bridged tokens to user  │
└────────────────────────────────────────┘     └─────────────────────────────────────────┘
```

* Leg 1 and leg 2 are one atomic transaction (a facet's `swapAndStartBridgeTokensVia<Bridge>`).
* Leg 3 is a *separate* transaction on the destination chain, executed by whoever the bridge uses to
  deliver (a relayer, a LayerZero executor, a solver). It is best-effort: if the destination swap fails, the
  user still gets the bridged token (§5).

### 0.3 Why bridges differ (and why there are ~45 facets)

| Family | Mechanism | Example facets in this repo |
|---|---|---|
| Canonical rollup bridges | lock on L1, mint on L2 (or vice versa); slow but trust-minimal | `ArbitrumBridgeFacet`, `OptimismBridgeFacet`, `PolygonBridgeFacet`, `GnosisBridgeFacet`, `OmniBridgeFacet`, `MegaETHBridgeFacet` |
| Burn/mint stablecoin (Circle CCTP) | burn USDC on source, Circle attests, mint on destination | `CelerCircleBridgeFacet`, `PolymerCCTPFacet` |
| Liquidity networks / OFTs | pool-to-pool or LayerZero OFT transfer with a message | `StargateFacetV2`, `AllBridgeFacet`, `GlacisFacet` |
| Intents / solvers | user deposits an *order*; a relayer/solver fronts liquidity on destination and is repaid after proof | `AcrossFacetV4`, `DeBridgeDlnFacet`, `MayanFacet`, `RelayDepositoryFacet`, `EcoFacet`, `LiFiIntentEscrowFacetV2`, `NEARIntentsFacet`, `GardenFacet` |
| Gas refuel | deposit native, get tiny native amounts on N chains | `GasZipFacet` |

A facet is a thin adapter: it validates inputs, optionally swaps, then makes *one* external call into the
bridge's own contract. The bridge does the cross-chain part; LI.FI never runs its own validators.

### 0.4 What LI.FI custodies

Nothing, by design. The Diamond is a pass-through: tokens enter via `transferFrom`/`msg.value`, get swapped,
and leave to the bridge contract in the same transaction. Many facets say this explicitly, e.g.
`src/Facets/CelerCircleBridgeFacet.sol:54`:

> It is safe to set a max approval since the diamond is designed to not hold any funds

Because of this, leftover dust in the Diamond is treated as sweepable (`WithdrawFacet`, §1.8) and periphery
contracts inherit `WithdrawablePeriphery` for the same reason.

### 0.5 Where trust lives

| Trust point | What it can do | Where enforced |
|---|---|---|
| The bridge protocol | lose / delay / mis-deliver funds after leg 2 | nothing LI.FI can do; choice made off-chain by the API |
| Diamond owner (multisig via timelock) | add/replace/remove any facet = arbitrary code behind the Diamond address | `LibDiamond.enforceIsContractOwner` `src/Libraries/LibDiamond.sol:97`, timelock §1.5 |
| PauserWallet (a hot EOA) | remove a facet / pause the whole Diamond (cannot add code) | `src/Facets/EmergencyPauseFacet.sol:43` |
| Whitelist | which DEX contract+selector pairs the Diamond may call with user funds | `LibAllowList` §1.8, checked in `SwapperV2._executeSwaps` |
| LI.FI API | generates calldata; several facets *only* validate what the API produces (see `AcrossFacetV4.sol:142`, `AcrossV4SwapFacet` backend signature §4.1) | comments in facets |
| User approvals | anything approved to the Diamond can be spent by whitelisted calls | this is why the whitelist exists (§8) |

---

## 1. The Diamond (EIP-2535)

### 1.1 `LiFiDiamond.sol`: one address, many facets

`src/LiFiDiamond.sol` is 72 lines and has three parts.

**Constructor** (`src/LiFiDiamond.sol:14-27`): sets the owner and installs exactly one function, `diamondCut`,
from the `DiamondCutFacet`. Every other facet is added later by calling that function.

```solidity
// src/LiFiDiamond.sol:14-26
constructor(address _contractOwner, address _diamondCutFacet) payable {
    LibDiamond.setContractOwner(_contractOwner);
    LibDiamond.FacetCut[] memory cut = new LibDiamond.FacetCut[](1);
    bytes4[] memory functionSelectors = new bytes4[](1);
    functionSelectors[0] = IDiamondCut.diamondCut.selector;
    cut[0] = LibDiamond.FacetCut({ facetAddress: _diamondCutFacet, action: LibDiamond.FacetCutAction.Add, functionSelectors: functionSelectors });
    LibDiamond.diamondCut(cut, address(0), "");
}
```

**Fallback** (`src/LiFiDiamond.sol:32-67`): the dispatcher. Every call that is not `receive()` lands here.
It looks up `msg.sig` (first 4 bytes of calldata) in diamond storage and `delegatecall`s the facet:

```solidity
// src/LiFiDiamond.sol:32-66
fallback() external payable {
    LibDiamond.DiamondStorage storage ds;
    bytes32 position = LibDiamond.DIAMOND_STORAGE_POSITION;
    assembly { ds.slot := position }
    address facet = ds.selectorToFacetAndPosition[msg.sig].facetAddress;
    if (facet == address(0)) { revert LibDiamond.FunctionDoesNotExist(); }
    assembly {
        calldatacopy(0, 0, calldatasize())
        let result := delegatecall(gas(), facet, 0, calldatasize(), 0, 0)
        returndatacopy(0, 0, returndatasize())
        switch result
        case 0 { revert(0, returndatasize()) }
        default { return(0, returndatasize()) }
    }
}
```

`delegatecall` means: run the facet's *code* with the Diamond's *storage, balance, address and msg.sender*.
So `address(this)` inside any facet is the Diamond, token approvals are granted to the Diamond, and
`msg.sender` is still the user. That is what makes facets composable and also why storage must be namespaced.

**receive()** (`src/LiFiDiamond.sol:71`): lets the Diamond accept plain ETH (needed for WETH unwrap refunds
from DEXs).

### 1.2 Diamond storage and namespaced storage

Solidity lays out state variables from slot 0. If two facets each declared `uint256 x;` they would collide
at slot 0 under delegatecall. The Diamond pattern avoids this by never using declared state; every logical
storage struct is pinned to a *hashed slot*:

```solidity
// src/Libraries/LibDiamond.sol:12-13, 67-77
bytes32 internal constant DIAMOND_STORAGE_POSITION = keccak256("diamond.standard.diamond.storage");
function diamondStorage() internal pure returns (DiamondStorage storage ds) {
    bytes32 position = DIAMOND_STORAGE_POSITION;
    assembly { ds.slot := position }
}
```

Every facet/library that needs state does the same thing with its own string ("AppStorage" pattern,
`docs/LibStorage.md`):

| Namespace constant | File |
|---|---|
| `keccak256("diamond.standard.diamond.storage")` | `src/Libraries/LibDiamond.sol:13` |
| `keccak256("com.lifi.facets.ownership")` | `src/Facets/OwnershipFacet.sol:17` |
| `keccak256("com.lifi.reentrancyguard")` | `src/Helpers/ReentrancyGuard.sol:11` |
| `keccak256("com.lifi.library.allow.list")` | `src/Libraries/LibAllowList.sol:29` |
| `keccak256("com.lifi.library.access.management")` | `src/Libraries/LibAccess.sol:13` |
| `keccak256("com.lifi.facets.periphery_registry")` | `src/Facets/PeripheryRegistryFacet.sol:14` |
| `keccak256("com.lifi.facets.emergencyPauseFacet")` | `src/Facets/EmergencyPauseFacet.sol:34` |
| `keccak256("com.lifi.facets.debridgedln")` | `src/Facets/DeBridgeDlnFacet.sol:29` |
| `keccak256("com.lifi.facets.polymercctp")` | `src/Facets/PolymerCCTPFacet.sol:82` |

Two consequences you will see everywhere:

* Facets prefer `immutable` for per-deployment config (e.g. `SPOKEPOOL`, `USDC`). Immutables live in the
  facet's *bytecode*, so they work under delegatecall without touching Diamond storage.
* Anything that must be mutable (chain-id maps, pause state) uses a `getStorage()` helper with a namespace.
  A storage collision would require two namespaces to hash equal, which is not a realistic risk.

The `DiamondStorage` struct itself (`src/Libraries/LibDiamond.sol:39-52`):

```solidity
struct DiamondStorage {
    mapping(bytes4 => FacetAddressAndPosition) selectorToFacetAndPosition; // selector -> (facet, index in facet's selector list)
    mapping(address => FacetFunctionSelectors) facetFunctionSelectors;    // facet -> (selector list, index in facetAddresses)
    address[] facetAddresses;
    mapping(bytes4 => bool) supportedInterfaces;                          // ERC-165
    address contractOwner;
}
```

The two mappings are cross-indexed so that adding and removing selectors is O(1) (swap-and-pop, §1.3).

### 1.3 `LibDiamond`: how a cut works

`LibDiamond.diamondCut` (`src/Libraries/LibDiamond.sol:103-134`) loops over `FacetCut[]` and dispatches on
`action` (`Add=0, Replace=1, Remove=2`, `:54-59`), emits `DiamondCut`, then runs an optional initializer.

**addFunctions** (`:136-172`):
1. reverts if no selectors or facet is `address(0)`;
2. if the facet has no selectors registered yet, `addFacet` (`:241-250`) which does `extcodesize` check
   (`enforceHasContractCode`, `:354-363`) and pushes into `facetAddresses`;
3. for each selector: revert `FunctionAlreadyExists` if it is already mapped, else `addFunction` (`:252-265`)
   writes both mappings.

**replaceFunctions** (`:174-211`): same, but for each selector it first `removeFunction`s the old facet's
entry; reverts if the "new" facet is the same as the old one.

**removeFunctions** (`:213-239`): requires `facetAddress == address(0)` (EIP-2535 convention for removes)
and calls `removeFunction` per selector.

**removeFunction** (`:267-324`) is the swap-and-pop:

```solidity
// src/Libraries/LibDiamond.sol:276-301 (abridged)
if (_facetAddress == address(this)) { revert FunctionIsImmutable(); }
uint256 selectorPosition = ds.selectorToFacetAndPosition[_selector].functionSelectorPosition;
uint256 lastSelectorPosition = ds.facetFunctionSelectors[_facetAddress].functionSelectors.length - 1;
if (selectorPosition != lastSelectorPosition) {
    bytes4 lastSelector = ds.facetFunctionSelectors[_facetAddress].functionSelectors[lastSelectorPosition];
    ds.facetFunctionSelectors[_facetAddress].functionSelectors[selectorPosition] = lastSelector;
    ds.selectorToFacetAndPosition[lastSelector].functionSelectorPosition = uint96(selectorPosition);
}
ds.facetFunctionSelectors[_facetAddress].functionSelectors.pop();
delete ds.selectorToFacetAndPosition[_selector];
```

and if that was the facet's last selector (`:304-323`) the facet address is swap-and-popped out of
`facetAddresses` too.

**initializeDiamondCut** (`:326-352`): if `_init != 0`, `delegatecall(_init, _calldata)`. This is how a
cut can atomically run a one-time setup in Diamond storage (e.g. an `init<Bridge>` function). It bubbles the
revert string, else `InitReverted`. Zero-address with non-empty calldata (or the reverse) is rejected.

### 1.4 The three core facets

* `DiamondCutFacet.diamondCut` (`src/Facets/DiamondCutFacet.sol:18-25`): `enforceIsContractOwner()` then
  `LibDiamond.diamondCut(...)`. This is the *only* upgrade path.
* `DiamondLoupeFacet` (`src/Facets/DiamondLoupeFacet.sol`): read-only views `facets()`,
  `facetFunctionSelectors(addr)`, `facetAddresses()`, `facetAddress(selector)`, `supportsInterface`. Tooling
  (and `EmergencyPauseFacet`) use these. `LibDiamondLoupe` (`src/Libraries/LibDiamondLoupe.sol`) is the same
  logic as internal functions so facets can read without an external self-call.
* `OwnershipFacet` (`src/Facets/OwnershipFacet.sol`): 2-step transfer. `transferOwnership` (`:43-54`) only
  stores `newOwner` in its own namespace; `confirmOwnershipTransfer` (`:67-74`) must be called *by the new
  owner* and then calls `LibDiamond.setContractOwner`. `cancelOwnershipTransfer` (`:57-64`). A typo'd owner
  address therefore cannot brick the Diamond.

### 1.5 Who may cut: owner, timelock, pauser

`LibDiamond.enforceIsContractOwner` (`src/Libraries/LibDiamond.sol:97-100`) is the gate on `diamondCut`,
`setCanExecute`, `registerPeripheryContract`, `unpauseDiamond`, and the `init*` functions of facets.

In production the owner-side flow is wrapped by `LiFiTimelockController` (`src/Security/LiFiTimelockController.sol`),
a thin subclass of OpenZeppelin `TimelockController`:

* constructor (`:34-58`) validates non-zero delay/roles, stores `diamond`, grants `CANCELLER_ROLE` to a
  canceller wallet; the *admin* is the LI.FI multisig SAFE (`:32`).
* `docs/LiFiTimelockController.md` states the production `minDelay` is 3 hours. A facet upgrade is
  `schedule(diamondCut...)` → wait → `execute(...)`; `docs/DeferredDiamondCleanupQueue.md` describes the
  Safe-proposal tooling around it (`propose-to-safe.ts --timelock`).
* `unpauseDiamond` (`:77-82`) is the one deliberate bypass: admin role, no delay, because unpausing can only
  *re-activate previously registered* facets or blacklist some; it cannot add new code (`:70-73`).

### 1.6 `EmergencyPauseFacet`: fast stop without a multisig

`src/Facets/EmergencyPauseFacet.sol`. Modifier `OnlyPauserWalletOrOwner` (`:43-49`): the `pauserWallet` is an
`immutable` EOA set at construction (`:53-56`) so it can react within seconds.

**removeFacet(address)** (`:63-87`): reads the facet's selectors from diamond storage, refuses to remove
itself (`:67-68`) or the `DiamondCutFacet` (`:79-81`, checks `functionSelectors[0] == diamondCut.selector`),
then `LibDiamond.removeFunctions(address(0), selectors)`.

**pauseDiamond()** (`:98-127`): for every facet except itself, `LibDiamond.replaceFunctions(self, selectors)`
– every selector is *re-pointed to the EmergencyPauseFacet*, whose `fallback` (`:240-242`) reverts with
`DiamondIsPaused()`. The old (facet, selectors) pairs are saved in namespaced storage (`s.facets.push`) so
they can be restored. Note the doc comment about gas: with many facets this loop is large, hence a forked
mainnet test (`:94-97`).

**unpauseDiamond(address[] blacklist)** (`:132-190`): owner-only. Replays `replaceFunctions` for every saved
facet, then for each blacklisted facet builds a `Remove` cut (never for `DiamondCutFacet`, `:163-165`) and
executes it, then `delete s.facets`.

The whole thing is "pause = redirect every selector to a reverting facet", which is elegant because it
needs no `paused` flag in every facet.

### 1.7 Facet removal hygiene (docs only)

`docs/FacetRemovalReconciliation.md` and `docs/DeferredDiamondCleanupQueue.md` are design docs for a real
operational problem: deprecating a facet in the *repo* does not remove it from 71 production Diamonds, so
its selectors stay callable forever. The proposed fix parks a removal task per (facet, network) and drains
it into the next Safe proposal for that network. Worth reading to see how the loupe (`facetFunctionSelectors`)
is the only trustworthy selector source after source deletion.

### 1.8 Access, whitelist, registry, withdraw

**LibAccess + AccessManagerFacet.** `LibAccess.AccessStorage.execAccess[selector][account]`
(`src/Libraries/LibAccess.sol:16-18`). `enforceAccessControl()` (`:60-64`) checks `execAccess[msg.sig][msg.sender]`.
Owner grants/revokes via `AccessManagerFacet.setCanExecute` (`src/Facets/AccessManagerFacet.sol:24-41`),
which refuses to authorise the Diamond itself. Used as a "if not owner then must have per-selector access"
pattern, e.g. `WhitelistManagerFacet.sol:23-25` and `WithdrawFacet.sol:42-44`.

**LibAllowList + WhitelistManagerFacet.** This is the security core of the swap path. Storage
(`src/Libraries/LibAllowList.sol:31-63`) is a *dual model*: a legacy global `contractAllowList` /
`selectorAllowList` kept for old deployed facets, and the source of truth
`contractSelectorAllowList[contract][selector]` (`:53`). The primary API:

* `addAllowedContractSelector(c, s)` (`:70-101`): sets the pair, and via reference counting keeps the legacy
  arrays in sync (`_addAllowedContract` requires `LibAsset.isContract`, `:202-203`).
* `removeAllowedContractSelector(c, s)` (`:108-133`): mirror image.
* `contractSelectorIsAllowed(c, s)` (`:140-145`): the runtime check every swap goes through.

Special selector `0xffffffff` = `APPROVE_TO_ONLY_SELECTOR` (`:16-23`, `src/Helpers/SwapperV2.sol:25`):
some DEXs need approval to contract A while you call router B (e.g. Permit2-style or vault-based DEXs). The
pair `(A, 0xffffffff)` says "A may be an `approveTo` target but nothing may be *called* on A". `SwapperV2`
enforces this at `src/Helpers/SwapperV2.sol:204-215`.

`WhitelistManagerFacet` (`src/Facets/WhitelistManagerFacet.sol`): `setContractSelectorWhitelist` (`:18-27`)
and `batchSetContractSelectorWhitelist` (`:30-51`), gated by owner or `LibAccess`; all writes funnel through
`_setContractSelectorWhitelist` (`:94-121`), which refuses the Diamond itself and skips no-op changes. Views
`isContractSelectorWhitelisted`, `getWhitelistedSelectorsForContract`, `getAllContractSelectorPairs` are what
the backend syncs against.

**PeripheryRegistryFacet** (`src/Facets/PeripheryRegistryFacet.sol`): `mapping(string => address)`, owner sets
`registerPeripheryContract(name, addr)` (`:31-39`). Pure discovery ("where is the Executor on this chain").

**WithdrawFacet** (`src/Facets/WithdrawFacet.sol`): `withdraw(asset, to, amount)` (`:65-74`) and
`executeCallAndWithdraw(callTo, data, asset, to, amount)` (`:35-59`, arbitrary call *then* withdraw, e.g.
to unwrap first). Both owner-or-access-controlled. Justified by "the Diamond holds nothing except accidents".

---

## 2. Shared plumbing

### 2.1 `ILiFi.BridgeData` and the event trail

`src/Interfaces/ILiFi.sol:10-21`:

| Field | Meaning | Who consumes it |
|---|---|---|
| `bytes32 transactionId` | route id generated by the API | indexed key of every event; backend status tracking |
| `string bridge` | tool name ("across", "stargateV2", ...) | analytics; `CalldataVerificationFacet.validateCalldata` |
| `string integrator` | partner name | analytics / fee attribution |
| `address referrer` | referral | analytics |
| `address sendingAssetId` | token that goes *into the bridge* (after source swaps); `address(0)` = native | `depositAsset`, approvals, native/ERC20 branch in `_startBridge` |
| `address receiver` | final recipient on destination, or `NON_EVM_ADDRESS` sentinel | receiver cross-checks in `_startBridge` |
| `uint256 minAmount` | amount handed to the bridge; on swap paths it is the *min* swap output and is overwritten with the real output | `depositAsset`, slippage floor, bridge amount |
| `uint256 destinationChainId` | LI.FI chain id (EVM id, or a made-up id for non-EVM, `src/Helpers/LiFiData.sol:13-22`) | mapped to bridge-specific ids |
| `bool hasSourceSwaps` | must match whether `_swapData` is passed | `Validatable` modifiers |
| `bool hasDestinationCall` | must match whether a destination message is set | `_startBridge` checks |

`NON_EVM_ADDRESS = 0x11f111f111f111F111f111f111F111f111f111F1` (`src/Helpers/LiFiData.sol:9-10`): when the
destination is Solana/Bitcoin/etc., `receiver` holds this sentinel and the real bytes32/bytes receiver lives
in the bridge-specific struct; the facet emits `BridgeToNonEVMChain(Bytes32)` (`ILiFi.sol:55-64`).

Events (`ILiFi.sol:25-52`):

* `LiFiTransferStarted(BridgeData)` – emitted by every `_startBridge` on the source chain.
* `LiFiTransferCompleted(transactionId, receivingAssetId, receiver, amount, timestamp)` – emitted by
  `Executor._processSwaps` on the destination when the destination swap succeeded.
* `LiFiTransferRecovered(...)` – emitted by a Receiver when the destination swap failed and raw bridged
  tokens were forwarded instead.
* `LiFiGenericSwapCompleted(...)` – same-chain swaps (§3).
* `LibSwap.AssetSwapped(...)` (`src/Libraries/LibSwap.sol:41-49`) – one per swap step.

The backend joins these on `transactionId` to show "pending / done / partial (recovered)".

### 2.2 `LibAsset`: native vs ERC20

`src/Libraries/LibAsset.sol`. Convention: `NULL_ADDRESS = address(0)` means the native asset (`:23`),
`isNativeAsset` (`:190-192`).

* `getOwnBalance(asset)` (`:31-36`): `address(this).balance` or `balanceOf(this)`.
* `transferAsset` (`:45-55`) → `transferNativeAsset` (`:61-70`, solady `safeTransferETH`) or `transferERC20`
  (`:76-88`, solady `safeTransfer`, handles non-returning tokens like USDT).
* `transferFromERC20(asset, from, to, amount)` (`:95-113`): reverts for native, zero recipient.
* `depositAsset(asset, amount)` (`:118-130`): pull `amount` from `msg.sender` into `address(this)`; for
  native it only checks `msg.value >= amount` (the ETH is already here).
* `depositAssets(SwapData[])` (`:132-142`): loops swaps and deposits those with `requiresDeposit == true`.
  This flag lets a multi-step route pull several *different* input tokens from the user, and skip pulling
  for intermediate steps.
* `maxApproveERC20(token, spender, required)` (`:149-155`) → `approveERC20(..., type(uint256).max)`
  (`:163-185`): only re-approves if `allowance < required`, using solady `safeApproveWithRetry` (approve → on
  failure approve(0) then approve again, for USDT-style tokens). Max approvals are considered safe because
  the Diamond keeps no balance (§0.4).
* `isContract` (`:200-209`): `extcodesize > 23` so EIP-7702 delegated EOAs (exactly 23 bytes) are *not*
  treated as contracts.

### 2.3 `LibSwap`: the generic "call a DEX" step

`SwapData` (`src/Libraries/LibSwap.sol:23-31`):

```solidity
struct SwapData {
    address callTo;          // DEX/aggregator/fee-collector to call
    address approveTo;       // who gets the ERC20 allowance (usually == callTo)
    address sendingAssetId;  // input token (0 = native)
    address receivingAssetId;// expected output token (0 = native)
    uint256 fromAmount;      // input amount (native value if native)
    bytes callData;          // full ABI-encoded call incl. selector
    bool requiresDeposit;    // pull fromAmount from user first?
}
```

`LibSwap.swap(transactionId, swap)` (`:51-108`), step by step:

1. `callTo` must be a contract (`:53`); `fromAmount != 0` (`:56-57`).
2. `nativeValue = fromAmount` if input is native, else 0 (`:60-62`).
3. Record `initialReceivingAssetBalance` (`:65-67`) purely for the event.
4. If ERC20: `maxApproveERC20(sendingAssetId, approveTo, fromAmount)` (`:70-76`).
5. `callTo.call{value: nativeValue}(callData)`; on failure bubble the exact revert bytes via
   `LibUtil.revertWith` (`:86-91`).
6. Emit `AssetSwapped` with `toAmount = newBalance - initial` (`:94-107`).

Note what it does **not** do: it does not check the whitelist and does not verify output. Both are the
caller's job (`SwapperV2._executeSwaps`, `GenericSwapFacetV3`, `Executor`). The comment at `:78-82` explains
that a pre-check `balance >= fromAmount` was removed for rebasing/fee-on-transfer tokens.

### 2.4 `SwapperV2`: deposit, swap, sweep leftovers

`src/Helpers/SwapperV2.sol`, inherited by every bridge facet that supports source swaps.

**`_depositAndSwap(txId, minAmount, swaps, leftoverReceiver)`** (`:89-127`) returns the *actual* output of
the last swap:

```solidity
// src/Helpers/SwapperV2.sol:101-126
address finalTokenId = _swaps[numSwaps - 1].receivingAssetId;
uint256 initialBalance = LibAsset.getOwnBalance(finalTokenId);
if (LibAsset.isNativeAsset(finalTokenId)) { initialBalance -= msg.value; }   // msg.value is already in balance
uint256[] memory initialBalances = _fetchBalances(_swaps);
LibAsset.depositAssets(_swaps);                                               // pull inputs (requiresDeposit)
_executeSwaps(_transactionId, _swaps, _leftoverReceiver, initialBalances);    // whitelist check + LibSwap.swap each
uint256 newBalance = LibAsset.getOwnBalance(finalTokenId) - initialBalance;
if (newBalance < _minAmount) { revert CumulativeSlippageTooHigh(_minAmount, newBalance); }
return newBalance;
```

The overload with `_nativeReserve` (`:135-179`) is used when the *bridge* needs native for its messaging fee
(Stargate `fee.nativeFee`, deBridge `globalFixedNativeFee`). It keeps that much native from being swept
and subtracts it from the measured output if the final asset is native (`:170-172`).

**`_executeSwaps`** (`:190-222` and the reserve variant `:230-269`) is where the **whitelist is enforced**:

```solidity
// src/Helpers/SwapperV2.sol:204-215
if (
    !LibAllowList.contractSelectorIsAllowed(callTo, bytes4(currentSwap.callData[:4])) ||
    (!LibAsset.isNativeAsset(currentSwap.sendingAssetId) &&
        approveTo != callTo &&
        !LibAllowList.contractSelectorIsAllowed(approveTo, APPROVE_TO_ONLY_SELECTOR))
) revert ContractCallNotAllowed();
LibSwap.swap(_transactionId, currentSwap);
```

i.e. `(callTo, selector)` must be whitelisted, and if `approveTo != callTo` then `approveTo` must be an
approve-only whitelisted target. The function is wrapped in `noLeftovers` / `noLeftoversReserve`
(`:34-62`), which run `_refundLeftovers` **after** all swaps.

**`_fetchBalances`** (`:274-290`): snapshot of `getOwnBalance(receivingAssetId)` for each step (minus
`msg.value` for native) so leftovers can be measured as deltas.

**`_refundLeftovers`** (`:297-356`), what exactly goes back to `_leftoverReceiver` (the user, `msg.sender`):

* For every non-final step whose `receivingAssetId != finalAsset`: any *increase* over the snapshot
  (`:316-333`). This returns intermediate tokens that a later step did not fully consume.
* For every step's `sendingAssetId`, if it is not the final asset: the *entire current balance*
  (`:337-354`). This returns unspent input (positive slippage on the input side, or over-deposit).
* Native is reduced by `_nativeReserve` in both cases so the bridge fee survives.
* The final asset is never swept: it is what gets bridged.

Worked example: route USDC → WETH → (bridge WETH). Swap 1 USDC→WETH via DEX A consumes all USDC, swap 2 is a
fee-collection step WETH→WETH. `_refundLeftovers` sees step 1's `receivingAssetId == finalAsset` (skip),
step 1 input USDC balance 0 (skip), step 2 input WETH == finalAsset (skip). Nothing is refunded; the whole WETH
balance delta is `newBalance`, and `_startBridge` bridges it.

**`refundExcessNative(receiver)`** (`:67-79`): snapshot `balance - msg.value` before, and after the function
send back `finalBalance - initialBalance`. Because the Diamond holds nothing, "excess" is whatever of
`msg.value` was not consumed (over-paid bridge fees, native positive slippage).

### 2.5 `Validatable` modifiers

`src/Helpers/Validatable.sol`:

| Modifier | Check | Line |
|---|---|---|
| `validateBridgeData` | receiver != 0, minAmount != 0, destinationChainId != block.chainid | `:14-25` |
| `noNativeAsset` | sendingAssetId != 0 | `:27-32` |
| `onlyAllowSourceToken(bd, token)` | sendingAssetId == token (single-asset bridges like CCTP/USDC) | `:34-42` |
| `onlyAllowDestinationChain(bd, id)` | destinationChainId == id (canonical L1↔L2 bridges) | `:44-52` |
| `containsSourceSwaps` / `doesNotContainSourceSwaps` | `hasSourceSwaps` flag must match the entry point | `:54-66` |
| `doesNotContainDestinationCalls` | `hasDestinationCall` must be false (bridge cannot carry a message) | `:68-75` |

These make the *flags in BridgeData honest*, which matters because wallets/security tools decode BridgeData
to show users what will happen (§6, `CalldataVerificationFacet`).

### 2.6 `ReentrancyGuard`

`src/Helpers/ReentrancyGuard.sol:30-36`: a classic 0/1 status flag but stored at
`keccak256("com.lifi.reentrancyguard")` (`:11`), so it is *shared by all facets* under the Diamond: a
re-entrant call into a different facet is also blocked. Note the Diamond calls arbitrary whitelisted DEXs with
user funds, so this guard plus the whitelist are the two lines of defence against a malicious `callTo`.

---

## 3. Same-chain swaps: `GenericSwapFacetV3`

`src/Facets/GenericSwapFacetV3.sol` is the *gas-optimised* same-chain path. It does **not** inherit
`SwapperV2`; it re-implements a leaner version with direct transfers to the receiver. `NATIVE_ADDRESS` is an
immutable (`:19`, set per chain, used only in events).

Six entry points, two families:

**Single swap** (one `SwapData`):

* `swapTokensSingleV3ERC20ToERC20` (`:43-90`): `_depositAndSwapERC20Single` → read *full* balance of
  `receivingAssetId` (`:57-59`) → `minAmountOut` check → `transferERC20(receivingAssetId, receiver, all)`
  → emit `AssetSwapped` + `LiFiGenericSwapCompleted`.
* `swapTokensSingleV3ERC20ToNative` (`:99-144`): same but sends `address(this).balance` via low-level call
  (`:110-119`).
* `swapTokensSingleV3NativeToERC20` (`:153-223`): whitelist check inline (`:163-168`), `callTo.call{value:
  msg.value}` (`:172-174`), then `_returnPositiveSlippageNative` (`:179`) and full ERC20 balance to receiver.

`_depositAndSwapERC20Single` (`:340-393`): `transferFromERC20(user → diamond, fromAmount)`; whitelist check
on `(callTo, selector)` and approve-only check on `approveTo` (`:358-372`); `maxApproveERC20`; `callTo.call`;
`_returnPositiveSlippageERC20(sendingAssetId, receiver)`.

**Multiple swaps** (`SwapData[]`):

* `swapTokensMultipleV3ERC20ToNative` (`:234-252`), `...ERC20ToERC20` (`:261-279`), `...NativeToERC20`
  (`:288-305`): each is `_depositMultipleERC20Tokens` (skipped for native-in) → `_executeSwaps` →
  `_transfer{ERC20,Native}TokensAndEmitEvent`.
* `_depositMultipleERC20Tokens` (`:308-338`): pulls every step with `requiresDeposit` via `transferFromERC20`.
* `_executeSwaps` (`:399-504`): per step, whitelist + approve-only checks (`:419-437`), native or ERC20 call,
  and after each *real* swap (`sendingAssetId != receivingAssetId`, so not after fee-collection steps)
  returns leftover input to the receiver (`:452-453`, `:478-479`). Emits `AssetSwapped` with the contract's
  full balance as `toAmount` (documented imprecision `:483-487`).
* `_transferERC20TokensAndEmitEvent` (`:506-537`) / `_transferNativeTokensAndEmitEvent` (`:539-571`):
  final `minAmountOut` check on the full balance, then send everything to `receiver`.

**Positive slippage helpers**: `_returnPositiveSlippageERC20` (`:574-596`) only refunds if balance `> 1`
(1 wei dust intentionally left for rebasing-token rounding, `:584-587`); `_returnPositiveSlippageNative`
(`:599-608`) sends the whole native balance.

Differences vs the `SwapperV2` path:

| | `SwapperV2` (bridge facets) | `GenericSwapFacetV3` |
|---|---|---|
| Output measured as | delta vs pre-swap snapshot | *full* contract balance (assumes Diamond is empty) |
| Leftover handling | after all swaps, per-asset deltas | after each swap, whole input balance |
| Final transfer | none (bridge pulls it) | to `receiver` |
| Native reserve | supported | n/a |
| Reentrancy guard | facets add `nonReentrant` | none (no external state; funds leave in the same call) |

Two `TODO(EXSC-850)` comments (`:195-201`, `:318-323`) are honest about a reporting weakness: the emitted
`fromAmount` is caller-declared and may not equal what was actually spent.

---

## 4. Bridge facets

### 4.0 The pattern

`docs/AddingANewBridge.md` says every bridge is generated from the same template (`bun codegen facet`).
The skeleton is always:

```solidity
contract FooFacet is ILiFi, ReentrancyGuard, SwapperV2, Validatable {
    IFoo public immutable FOO;                       // bridge contract on this chain
    struct FooData { ... }                           // bridge-specific params

    function startBridgeTokensViaFoo(BridgeData memory bd, FooData calldata fd)
        external payable nonReentrant refundExcessNative(payable(msg.sender))
        validateBridgeData(bd) doesNotContainSourceSwaps(bd) [doesNotContainDestinationCalls(bd)]
    {
        LibAsset.depositAsset(bd.sendingAssetId, bd.minAmount);   // pull exactly minAmount
        _startBridge(bd, fd);
    }

    function swapAndStartBridgeTokensViaFoo(BridgeData memory bd, LibSwap.SwapData[] calldata swaps, FooData calldata fd)
        external payable nonReentrant refundExcessNative(payable(msg.sender))
        containsSourceSwaps(bd) validateBridgeData(bd)
    {
        bd.minAmount = _depositAndSwap(bd.transactionId, bd.minAmount, swaps, payable(msg.sender));
        _startBridge(bd, fd);
    }

    function _startBridge(BridgeData memory bd, FooData memory fd) internal {
        // 1. flag/receiver consistency checks
        // 2. native branch: FOO.deposit{value: bd.minAmount}(...)
        //    ERC20 branch:  LibAsset.maxApproveERC20(token, FOO, bd.minAmount); FOO.deposit(...)
        emit LiFiTransferStarted(bd);
    }
}
```

Modifier order matters and is deliberate: `nonReentrant` outermost, `refundExcessNative` wraps everything so
the post-body refund runs after the bridge call, then data validation.

### 4.1 Across (intents): `AcrossFacetV4`, `AcrossFacetPackedV4`, `AcrossV4SwapFacet`

Across is an intent bridge: the user *deposits* on the source SpokePool with an `outputAmount` they are
willing to receive; a relayer *fills* on the destination out of its own funds (fast), and is repaid from the
source deposit after UMA's optimistic verification. LI.FI just has to call `SpokePool.deposit` correctly.

**`AcrossFacetV4`** (`src/Facets/AcrossFacetV4.sol`).

`AcrossV4Data` (`:61-73`): `receiverAddress` (bytes32, our Receiver contract if there is a destination call),
`refundAddress` (= Across `depositor`; gets refunded if never filled), `sendingAssetId`/`receivingAssetId`
(bytes32 because V4 supports Solana), `outputAmount`, `outputAmountMultiplier`, `exclusiveRelayer`,
`quoteTimestamp`, `fillDeadline`, `exclusivityParameter`, `message`.

`swapAndStartBridgeTokensViaAcrossV4` (`:118-150`) has one extra step after `_depositAndSwap`: because the
real swap output differs from the quoted one, it rescales the output amount:

```solidity
// src/Facets/AcrossFacetV4.sol:144-147
AcrossV4Data memory modifiedAcrossData = _acrossData;
modifiedAcrossData.outputAmount = (_bridgeData.minAmount * _acrossData.outputAmountMultiplier) / MULTIPLIER_BASE;
```

where the multiplier is `pricePct * 1e18 * 10^(outDecimals - inDecimals)` computed by the backend (`:137-143`).
The comment says outright: "Only use LI.FI backend-generated calldata to avoid potential loss of funds".

`_startBridge` (`:157-247`):
1. `message.length > 0` must equal `hasDestinationCall` (`:162-164`).
2. map Solana's LI.FI chain id to Across's id (`_getAcrossChainId`, `:252-261`).
3. receiver checks (`:172-199`): non-EVM → `receiverAddress != 0` and emit `BridgeToNonEVMChainBytes32`;
   EVM → if *no* destination call, `receiverAddress` must equal `bridgeData.receiver` (if there *is* one,
   `receiverAddress` is our `ReceiverAcrossV4`, so they legitimately differ).
4. `refundAddress != 0` (`:202-204`).
5. The external call (`:207-244`): native → `SPOKEPOOL.deposit{value: minAmount}(refund, receiver,
   WRAPPED_NATIVE, receivingAssetId, minAmount, outputAmount, chainId, exclusiveRelayer, quoteTimestamp,
   fillDeadline, exclusivityParameter, message)`; ERC20 → `maxApproveERC20(token, SPOKEPOOL)` then the same
   `deposit` with `sendingAssetId`. Interface: `src/Interfaces/IAcrossSpokePoolV4.sol:47-60`.
6. `emit LiFiTransferStarted`.

Destination data: the `message` bytes are `abi.encode(transactionId, SwapData[], finalReceiver)`; the
SpokePool passes them to `ReceiverAcrossV4.handleV3AcrossMessage` on fill (§5). Storage: none (all immutables).
Trust: Across relayers + UMA; LI.FI backend for `outputAmount`.

**`AcrossFacetPackedV4`** (`src/Facets/AcrossFacetPackedV4.sol`) is a gas trick: no ABI encoding, no
`BridgeData`, no validation. It is *not* delegatecalled through the Diamond; it is a standalone contract with
`TransferrableOwnership`. `startBridgeTokensViaAcrossV4NativePacked()` (`:119-137`) takes **no parameters**
and slices `msg.data` at fixed offsets:

```solidity
// src/Facets/AcrossFacetPackedV4.sol:121-134
SPOKEPOOL.deposit{ value: msg.value }(
    bytes32(msg.data[12:44]),               // depositor
    bytes32(msg.data[44:76]),               // recipient
    WRAPPED_NATIVE,                          // inputToken (hard-wired)
    bytes32(msg.data[76:108]),              // outputToken
    msg.value,                               // inputAmount (hard-wired)
    uint256(bytes32(msg.data[108:140])),    // outputAmount
    uint64(bytes8(msg.data[140:148])),      // destinationChainId
    ...
    msg.data[192:msg.data.length]           // message
);
emit LiFiAcrossTransfer(bytes8(msg.data[4:12]));   // only 8 bytes of tx id
```

The ERC20 version (`:187-216`) pulls tokens with `transferFromERC20` first and reads `inputAmount` as a
`uint128` from `[140:156]`. `encode_*`/`decode_*` helpers (`:261-433`) exist so tooling and
`CalldataVerificationFacet`-style checkers can still recover a `BridgeData`. `setApprovalForBridge`
(`:85-96`) pre-approves the SpokePool because there is no per-call `maxApprove`. The doc header (`:15-18`)
is explicit that a zero `depositor` would lose the refund.

**`AcrossV4SwapFacet`** (`src/Facets/AcrossV4SwapFacet.sol`) targets Across's *Swap API* contracts
(`SpokePool`, `SpokePoolPeriphery.swapAndBridge`, sponsored OFT/CCTP peripheries) with *opaque* calldata
from the backend. Since the final receiver is buried in that calldata and cannot be checked against
`BridgeData.receiver`, the facet requires an **EIP-712 signature from LI.FI's backend signer** over
`(transactionId, minAmount, receiver, destinationChainId, sendingAssetId, swapApiTarget, keccak(callData))`
for the two unsigned targets (`_verifySignatureIfRequired`, `:274-316`, typehash `:63-65`, signer `:78`).
This is a different trust model: the backend key becomes a signing oracle for the route. Positive slippage
is handled by either patching the encoded amount (unsigned paths) or refunding surplus (sponsored, signed
quotes) (`:227-238`).

### 4.2 Stargate V2 (LayerZero OFT): `StargateFacetV2` + `ReceiverStargateV2`

`src/Facets/StargateFacetV2.sol`. `StargateData` (`:30-35`): `assetId` (Stargate's uint16 id for the token),
`sendParams` (LayerZero `SendParam`: `dstEid`, `to`, `amountLD`, `minAmountLD`, `extraOptions`, `composeMsg`,
`oftCmd`; `src/Interfaces/IStargate.sol:23-31`), `fee` (`nativeFee`, `lzTokenFee`), `refundAddress`.

`swapAndStartBridgeTokensViaStargate` (`:73-94`) uses the `_nativeReserve` overload with
`_stargateData.fee.nativeFee` so the LayerZero fee is not swept back.

`_startBridge` (`:101-164`):
1. `composeMsg.length > 0` ⇔ `hasDestinationCall`, and no `oftCmd` when composing (`:106-111`).
2. Without a destination call, `sendParams.to` must equal `receiver` (`:116-119`).
3. Resolve the pool: `tokenMessaging.stargateImpls(assetId)` (`:122-126`).
4. `msgValue = nativeFee (+ minAmount if native)` (`:129-133`); ERC20 → manual approve-to-max with USDT
   reset (`:137-150`).
5. `sendParams.amountLD = minAmount` (`:154`), then `IStargate(pool).sendToken{value: msgValue}(sendParams,
   fee, refundAddress)` (`:157-161`).

Destination: Stargate delivers tokens to `sendParams.to` (our `ReceiverStargateV2`) and, because
`composeMsg` is set, the LayerZero Endpoint later calls `lzCompose` on it with the composed message (§5).
Trust: LayerZero DVNs + Stargate pools. Storage: none.

### 4.3 CCTP (burn/mint): `CelerCircleBridgeFacet` and `PolymerCCTPFacet`

Circle's CCTP: `TokenMessenger.depositForBurn` burns USDC and emits a message; Circle's attestation service
signs it; anyone submits `receiveMessage` on the destination to mint. No liquidity, no relayer capital, but
only USDC and only to Circle-supported domains.

**`CelerCircleBridgeFacet`** (`src/Facets/CelerCircleBridgeFacet.sol`) goes through Celer's
`CircleBridgeProxy` (which relays the attestation for you). Only USDC: `onlyAllowSourceToken(bd, USDC)`
(`:78`, `:100`), no destination calls (`:76`, `:98`). One-time `initCelerCircleBridge` (`:56-62`) sets a max
approval to the proxy. `_startBridge` (`:116-136`) checks the chain id fits `uint64` and calls:

```solidity
// src/Facets/CelerCircleBridgeFacet.sol:125-133
CIRCLE_BRIDGE_PROXY.depositForBurn(
    _bridgeData.minAmount, uint64(_bridgeData.destinationChainId),
    bytes32(uint256(uint160(_bridgeData.receiver))), USDC,
    _celerCircleData.maxFee, _celerCircleData.minFinalityThreshold);   // 1000 fast, 2000 standard
```

**`PolymerCCTPFacet`** (`src/Facets/PolymerCCTPFacet.sol`) calls Circle's `TokenMessenger` directly
(`depositForBurn` `:471,513`, `depositForBurnWithHook` `:460,486`) with a Polymer relayer completing the
mint. It has real storage: a `chainId → CCTP domainId` map (`Storage`, `:123-126`, set by owner via
`initPolymerCCTP` `:194` / `setChainIdToDomainId` `:220`). It takes a `polymerTokenFee` in USDC
(`:427-431`) before burning `minAmount - fee`. Its `_startBridge` (`:342+`) is a good example of *corridor
dispatch*: for HyperCore and Stellar the mint recipient is a pinned Circle *forwarder* contract and the true
receiver travels in `hookData`, with byte-level validation that the hook's receiver equals
`bridgeData.receiver` (`:363-368`) or that the strkey length header is consistent (`:390-397`).

### 4.4 Order/solver bridges: `DeBridgeDlnFacet`, `MayanFacet`, `RelayDepositoryFacet`

**deBridge DLN** (`src/Facets/DeBridgeDlnFacet.sol`): you create an *order* ("I give X of token A on chain 1,
I want ≥ Y of token B on chain 2 to receiver R"); a taker fills on chain 2 and claims the give-side. Storage:
`chainId → deBridgeChainId` map + `initialized` (`:48-51`), set by `initDeBridgeDln` (`:98-113`). Fee: DLN
charges a fixed native fee read live from `DLN_SOURCE.globalFixedNativeFee()` (`:140`, `:162`) and reserved
via the `_nativeReserve` overload (`:164-170`). `_startBridge` (`:179-241`) builds `OrderCreation` with
`givePatchAuthoritySrc = msg.sender` and `orderAuthorityAddressDst` (who can cancel and pick the refund
beneficiary) and calls `DLN_SOURCE.createOrder{value: fee | minAmount}(...)` (`:214-227`). Emits
`DlnOrderCreated(orderId)` so the backend can track the order. Receiver/asset are raw `bytes` because DLN
supports Solana.

**Mayan** (`src/Facets/MayanFacet.sol`): Wormhole-based swaps; the bridge-specific `protocolData` is an
opaque Mayan call. The interesting engineering is `_parseReceiver` (`:335+`): it *decodes the receiver out
of the opaque calldata per selector* to compare against `BridgeData.receiver` (`:239-263`), and for ERC20
binds the amount encoded in `protocolData` to `minAmount` (`:271-279`) so nothing strands in Mayan's
forwarder. Calls: `MAYAN.forwardERC20` (`:287`), `swapAndForwardEth` (`:301`), `forwardEth` (`:311`).

**Relay** (`src/Facets/RelayDepositoryFacet.sol`): the simplest solver model. You just deposit into a
`RelayDepository` tagged with an off-chain `orderId` (`depositNative` `:121-126`, `depositErc20` `:135-140`);
Relay's solver reads the order off-chain and pays out. The facet cannot verify anything about the order
(`:16-18`, `:114-117`): this is pure trust in the Relay backend plus LI.FI's calldata.

### 4.5 Gas refuel: `GasZipFacet`

`src/Facets/GasZipFacet.sol`. Only native can be deposited (`:59-63`); the swap path requires the last swap
to output native (`:87-91`). `_startBridge` (`:108-137`) validates the packed bytes32 receiver (right-padded
for EVM, `:117-122`) and calls `GAS_ZIP_ROUTER.deposit{value}(destinationChains, receiverAddress)`
(`:131-134`). `destinationChains` packs up to 16 Gas.zip chain ids, 16 bits each
(`getDestinationChainsValue`, `:142-155`). `GasZipPeriphery` (§6) is the same thing packaged as a *swap
step* so it can be combined with any other bridge in one route.

### 4.6 LI.FI's own intents: `LiFiIntentEscrowFacetV2`

`src/Facets/LiFiIntentEscrowFacetV2.sol` deposits into an **Open Intents Framework (OIF)** *Input Settler*
(escrow). Solvers fill on the output chain; an output oracle proves the fill to the input oracle; the
escrow releases the input to the solver (`docs/LiFiIntentEscrowFacetV2.md` diagram). Highlights:

* Custom validator that *allows same-chain* intents (`:35-42`).
* No `outputAmount` field; committed output = `inputAmount * outputAmountMultiplier / 1e18` on both paths
  (`:120-122`, `:166-168`), and positive slippage funds the intent rather than being refunded (`:162-168`).
* `_startBridge` (`:180-271`): builds one `MandateOutput` (oracle, settler, chainId, token, amount,
  recipient, `callbackData`, context) and one `[tokenId, amount]` input, then `IOriginSettler.open(StandardOrder)`
  (`:257-268`). If `dstCallSwapData` is non-empty the recipient becomes `dstCallReceiver` (a `ReceiverOIF`)
  and `callbackData = abi.encode(txId, swaps, recipient)` (`:223-238`). Trust note in code (`:225-227`):
  `dstCallReceiver` is only checked non-zero, not verified to be a real `ReceiverOIF`.

### 4.7 Summary table

| Facet | External call | Destination payload | Trust | Diamond storage |
|---|---|---|---|---|
| AcrossFacetV4 | `SpokePool.deposit` | `message` → `handleV3AcrossMessage` | Across relayers/UMA, backend multiplier | none |
| AcrossFacetPackedV4 | `SpokePool.deposit` (standalone) | `message` | caller must pass valid params | n/a (not a diamond facet) |
| AcrossV4SwapFacet | SpokePool / Periphery / sponsored | opaque | **backend EIP-712 signer** | none |
| StargateFacetV2 | `IStargate.sendToken` | `composeMsg` → `lzCompose` | LayerZero + Stargate | none |
| CelerCircleBridgeFacet | `CircleBridgeProxy.depositForBurn` | none | Circle + Celer relay | none |
| PolymerCCTPFacet | `TokenMessenger.depositForBurn[WithHook]` | hookData for forwarders | Circle + Polymer relay | chainId→domain map |
| DeBridgeDlnFacet | `DlnSource.createOrder` | none (externalCall empty) | DLN takers | chainId map |
| MayanFacet | `forwardERC20/forwardEth/swapAndForwardEth` | inside `protocolData` | Wormhole + Mayan | none |
| RelayDepositoryFacet | `depositNative/depositErc20` | off-chain orderId | Relay backend | none |
| GasZipFacet | `GasZip.deposit` | none | Gas.zip | none |
| LiFiIntentEscrowFacetV2 | `IOriginSettler.open` | `callbackData` → `ReceiverOIF.outputFilled` | OIF oracles/solvers | none |

---

## 5. Destination side: Receivers and the Executor

### 5.1 `Executor`

`src/Periphery/Executor.sol`. A standalone (non-diamond) contract that runs `SwapData[]` and then forwards
everything to the final receiver. Two entry points:

* `swapAndCompleteBridgeTokens(txId, swaps, transferredAsset, receiver)` (`:91-105`) — cross-chain path,
  `_depositAllowance = true`: the caller (a Receiver contract) has *approved* the bridged amount; the
  Executor reads `allowance(msg.sender, this)` and pulls exactly that (`:161-166`).
* `swapAndExecute(txId, swaps, transferredAsset, receiver, amount)` (`:113-128`) — same-chain "arbitrary
  execution" path, `_depositAllowance = false`: pulls `amount` from `msg.sender` **via `ERC20Proxy`**
  (`:168-173`).

`_processSwaps` (`:139-211`):
1. snapshot final-asset and transferred-asset balances (minus `msg.value` for native);
2. deposit (see above);
3. `_executeSwaps` (`:217-234`): for each step, **refuse `callTo == erc20Proxy`** (`:224-226`) then
   `LibSwap.swap`; wrapped by `noLeftovers` (`:38-70`) which returns intermediate-token deltas to the receiver;
4. any leftover of the *transferred* asset → receiver (`:183-190`);
5. the final-asset delta → receiver (`:192-202`);
6. `emit LiFiTransferCompleted` (`:204-210`).

Note the Executor does **not** consult `LibAllowList` (it has no diamond storage). Its safety model is
different: it never holds funds between transactions and only ever has *transient* approvals from a
Receiver, so an arbitrary `callTo` can only touch what was bridged in that very transaction.

### 5.2 Why `ERC20Proxy` exists

`src/Periphery/ERC20Proxy.sol`. Users of the same-chain `swapAndExecute` flow approve **the proxy**, not
the Executor. Only `authorizedCallers` (the Executor, `:26-35`, `:40-46`) may call `transferFrom`
(`:53-62`). Combine that with the Executor's ban on `callTo == erc20Proxy`: the Executor executes arbitrary
user-supplied calls, so if user approvals were granted to the Executor directly, any `SwapData` could encode
`token.transferFrom(victim, attacker, x)` and drain other users. By putting approvals on a proxy that only the
Executor can drive *through one hard-coded code path* (`_processSwaps`, `:168-173`), arbitrary calldata can
never reach those approvals.

### 5.3 The Receivers: authenticate the bridge, try the swap, never strand funds

Every Receiver has the same shape: an `immutable EXECUTOR`, an `immutable` bridge address used in an
`only<Bridge>` modifier, a decode of `(bytes32 txId, SwapData[] swaps, address receiver)`, approve the
Executor, `try executor.swapAndCompleteBridgeTokens(...) {} catch { send raw tokens to receiver; emit
LiFiTransferRecovered }`, reset approval.

**`ReceiverAcrossV4`** (`src/Periphery/ReceiverAcrossV4.sol`): `handleV3AcrossMessage(tokenSent, amount,
relayer, message)` (`:57-78`) `onlySpokepool`. Since Across always delivers wrapped native, there is no
native branch (`:83`). Core:

```solidity
// src/Periphery/ReceiverAcrossV4.sol:97-121
assetId.safeApproveWithRetry(address(EXECUTOR), amount);
try EXECUTOR.swapAndCompleteBridgeTokens(_transactionId, _swapData, assetId, receiver) {
} catch {
    LibAsset.transferERC20(assetId, receiver, amount);
    emit LiFiTransferRecovered(_transactionId, assetId, receiver, amount, block.timestamp);
}
assetId.safeApprove(address(EXECUTOR), 0);
```

**`ReceiverStargateV2`** (`src/Periphery/ReceiverStargateV2.sol`): `lzCompose(from, guid, message, executor,
extraData)` (`:101-133`) `onlyEndpointV2`, and additionally checks `tokenMessaging.assetIds(_from) != 0`
(`:110`) so only a real Stargate pool can be the composer. It decodes with `OFTComposeMsgCodec.composeMsg`
/ `amountLD` (`src/Libraries/OFTComposeMsgCodec.sol:57-79`) and handles both native and ERC20. It adds a
**gas reserve**: `recoverGas` is immutable (`:55`); if `gasleft() < recoverGas` it skips the swap entirely
(`:155-167`, `:192-212`), and otherwise calls the Executor with `gas: cacheGasLeft - recoverGas`
(`:171-175`, `:216-219`) so the `catch` branch always has enough gas to do the fallback transfer. The long
comment at `:81-92` explains the residual risk: `lzCompose` is permissionless, so a griefer can front-run it
with too little gas and force the recovery path (user gets raw tokens instead of the swap).

**`ReceiverChainflip`** (`src/Periphery/ReceiverChainflip.sol`): `cfReceive` (`:74-96`) `onlyChainflipVault`;
maps Chainflip's `0xEeee...` native sentinel to `address(0)` (`:29-30`, `:116-118`).

**`ReceiverOIF`** (`src/Periphery/ReceiverOIF.sol`): `outputFilled(token, amount, executionData)` (`:75-98`)
`onlyTrustedOutputSettler`. Differs: **no try/catch** (`:108-129`). The header (`:59-70`) explains why:
a revert here blocks the fill, so the user is refunded on the source chain by the intent system; whereas a
silently failed swap would abandon tokens. So for OIF, "revert" *is* the safe fallback.

**Why the destination call must never hold funds hostage.** The bridge has already moved value; there is no
undo. If the Receiver reverted on a bad swap (Across/Stargate), the relayer's fill would fail and the tokens
would sit in limbo or the relayer would refuse to fill. Hence: authenticate the caller, attempt the swap with
bounded gas, and on any failure hand the raw bridged asset to the user and emit `LiFiTransferRecovered` so the
backend can show "delivered, swap skipped".

---

## 6. Periphery

### 6.1 `Permit2Proxy`: gasless and one-signature flows

`src/Periphery/Permit2Proxy.sol`. Three ways to get tokens into the Diamond without a separate `approve` tx:

* `callDiamondWithEIP2612Signature(token, amount, deadline, v, r, s, diamondCalldata)` (`:77-126`):
  `token.permit(msg.sender, this, ...)` (falls through if allowance already suffices, `:97-111`),
  `transferFrom(msg.sender → proxy)`, `maxApprove(proxy → diamond)`, `_executeCalldata`. **Must be sent by
  the signer** (`msg.sender` is hard-coded as owner, `:89`) so nobody can pair a stolen permit with different
  calldata.
* `callDiamondWithPermit2(diamondCalldata, permit, signature)` (`:136-159`): Uniswap Permit2
  `permitTransferFrom` with `owner = msg.sender` (`:147`), same reasoning.
* `callDiamondWithPermit2Witness(diamondCalldata, signer, permit, signature)` (`:168-201`): fully gasless.
  The signature covers a *witness* `LiFiCall(diamondAddress, keccak256(diamondCalldata))`
  (`WITNESS_TYPE_STRING` `:22-24`, `:174-179`), so a relayer can submit it but cannot change the calldata or
  target. `getPermit2MsgHash` (`:209-242`) reproduces the exact EIP-712 digest for the frontend.
* `_executeCalldata` (`:288-301`): `LIFI_DIAMOND.call{value: msg.value}(diamondCalldata)`, bubbles reverts.
  Because the proxy is `msg.sender` to the Diamond, `refundExcessNative(msg.sender)` refunds *the proxy*,
  which is why it has a `receive()` (`:388`) and why some facets carry an explicit `refundRecipient`
  (`PolymerCCTPFacet.sol:109-113`).
* Nonce helpers `nextNonce` / `nextNonceAfter` (`:313-385`) walk Permit2's bitmap nonces.

### 6.2 Fees: `FeeCollector` and `FeeForwarder`

Fees are just another whitelisted `SwapData` step (`sendingAssetId == receivingAssetId`, which is why
`GenericSwapFacetV3` skips positive-slippage refunds for such steps, `:449-453`).

`FeeCollector` (`src/Periphery/FeeCollector.sol`): `collectTokenFees(token, integratorFee, lifiFee,
integrator)` (`:54-69`) pulls both, credits `_balances[integrator][token]` and `_lifiBalances[token]`;
`collectNativeFees` (`:75-96`) same with `msg.value`, refunding overpay. Withdrawals: integrators pull their
own (`withdrawIntegratorFees` `:100-108`, batch `:112-132`); LI.FI's share is owner-only
(`withdrawLifiFees` `:136-144`).

`FeeForwarder` (`src/Periphery/FeeForwarder.sol`): stateless push to N recipients. `forwardERC20Fees`
(`:52-77`) is `transferFrom(msg.sender → recipient)` per entry; `forwardNativeFees` (`:86-126`) pays out
and *refunds unspent msg.value*, with a re-entrancy-aware guard that an invocation may never spend more than
its own `msg.value` (`:110-115`).

### 6.3 `TokenWrapper` (WETH) and `LidoWrapper` (stETH/wstETH)

`TokenWrapper` (`src/Periphery/TokenWrapper.sol`): `deposit()` (`:74-83`) wraps `msg.value` and sends the
wrapped token to the caller; `withdraw()` (`:88-108`) pulls the caller's *entire* wrapped balance and returns
native. Optional `CONVERTER` for chains whose wrapped native has non-18 decimals (`:52-66`). Used as a
whitelisted swap step when a route needs WETH↔ETH.

`LidoWrapper` (`src/Periphery/LidoWrapper.sol`) is the liquid-staking touch point, and worth understanding:

* **Liquid staking** (Lido): you deposit ETH, Lido stakes it with validators, and you receive **stETH**, a
  *rebasing* token whose balance grows daily as rewards accrue (your `balanceOf` literally increases).
  Rebasing is hostile to DeFi plumbing that snapshots balances, that bridges fixed amounts, or that assumes
  `transfer(x)` moves exactly `x` shares. So Lido also issues **wstETH**, a non-rebasing wrapper: a fixed
  number of wstETH represents a growing amount of stETH (the exchange rate rises instead of the balance).
* Bridges and L2s carry wstETH (fixed supply, standard ERC20 semantics). Users often want stETH on the
  other side (or hold stETH and want to bridge). An aggregator therefore needs an atomic wrap/unwrap step.
* On L2s Lido's stETH contract exposes `wrap`/`unwrap` **with reversed naming** (`:23`, `:10-18`). The
  wrapper hides that: `wrapStETHToWstETH(amount)` (`:67-88`) pulls stETH, calls `ST_ETH.unwrap(fullBalance)`
  and returns wstETH; `unwrapWstETHToStETH(amount)` (`:94-111`) pulls wstETH, calls `ST_ETH.wrap(amount)`.
  Constructor refuses mainnet (`:56-57`, the L1 contracts differ) and pre-approves stETH to pull wstETH
  (`:60`). Using "full balance" (`:79-82`) is safe only because the contract never holds funds; the doc
  warns direct transfers get MEV-swept (`:24`).
* In a LI.FI route this is just another whitelisted `SwapData` with `sendingAssetId = stETH`,
  `receivingAssetId = wstETH` (or the reverse), letting `SwapperV2` measure the result by balance delta,
  which works because wstETH does not rebase.

### 6.4 `Patcher`: amounts unknown at signing time

`src/Periphery/Patcher.sol`. Problem: the destination-side swap calldata was built *before* the bridge, but
the exact amount that arrives is unknown (fees, slippage). Solution: at execution time, `staticcall` a
getter (e.g. `balanceOf(receiver)`), then overwrite 32-byte slots in the prepared calldata at known offsets,
then call the target.

* `executeWithDynamicPatches(valueSource, valueGetter, finalTarget, value, data, offsets, delegateCall)`
  (`:78-106`) → `_getDynamicValue` (`:298-311`, return data must be exactly 32 bytes) → `_applyPatch`
  (`:317-331`, `mstore` at `data + 32 + offset`, bounds-checked) → `_executeCall` (`:340-362`, call or
  delegatecall, bubbles revert).
* `depositAndExecuteWithDynamicPatches` (`:128-161`) additionally pulls the caller's **entire** balance
  (`_depositAndApprove`, `:271-291`) and resets approval after.
* Multi-value variants (`:183-256`).

The file header (`:12-15`) and `docs/Patcher.md` are blunt: no refunds, no target whitelist, standing
approvals to it can be stolen by front-runners. It is meant to be used inside an atomic route by the
Executor, not approved by end users.

### 6.5 `OutputValidator`

`src/Periphery/OutputValidator.sol`. A swap step that skims *positive slippage* above `expectedAmount` into a
LI.FI `validationWallet`. `validateERC20Output(token, expected, wallet)` (`:118-149`) measures
`balanceOf(msg.sender)` (the Diamond) and `transferFrom`s the excess; `validateNativeOutput(expected, wallet)`
(`:48-112`) uses `msg.sender.balance + msg.value` and returns any non-excess `msg.value`. Relies on the
"Diamond holds nothing" invariant (`:13-18`).

### 6.6 `GasZipPeriphery`

`src/Periphery/GasZipPeriphery.sol`: `depositToGasZipERC20(swapData, gasZipData)` (`:54-113`) runs *one*
ERC20→native swap and deposits the output to Gas.zip; note it re-checks the Diamond's whitelist by
calling `IWhitelistManagerFacet(LIFI_DIAMOND).isContractSelectorWhitelisted` (`:62-83`) since it has no
diamond storage of its own. `depositToGasZipNative` (`:119-139`) deposits and returns leftover native to
`msg.sender`.

### 6.7 `LiFiDEXAggregator` (RouteProcessor)

`src/Periphery/LiFiDEXAggregator.sol`, a fork of SushiSwap's `RouteProcessor4` (`:57-59`). Instead of many
ABI calls, a whole multi-hop, multi-split route is a compact **byte stream** interpreted on-chain. The
Diamond whitelists `(aggregator, processRoute.selector)` and passes the stream in `SwapData.callData`.

Entry: `processRoute(tokenIn, amountIn, tokenOut, amountOutMin, to, route)` (`:151-168`, `lock`ed and
pausable `:100-106`) → `processRouteInternal` (`:206-271`):

1. snapshot `tokenIn` balance of `msg.sender` and `tokenOut` balance of `to`;
2. interpret commands until the stream is empty (`:225-240`):

| code | function | reads | meaning |
|---|---|---|---|
| 1 | `processMyERC20` (`:305-314`) | token | swap tokens already in the aggregator (minus 1 wei "slot undrain") |
| 2 | `processUserERC20` (`:320-323`) | token | swap tokens pulled from `msg.sender` |
| 3 | `processNative` (`:295-300`) | – | swap the aggregator's native balance |
| 4 | `processOnePool` (`:332-335`) | token | tokens are already sitting in the pool (UniV2 style) |
| 5 | `processInsideBento` (`:340-347`) | token | BentoBox shares |
| 6 | `applyPermit` (`:276-291`) | value, deadline, v, r, s | ERC-2612 permit for `tokenIn` |

3. `distributeAndSwap` (`:354-370`): reads `num` sub-routes, each with a `uint16 share` out of 65535, and
   calls `swap` per share; `swap` (`:377-407`) reads a `poolType` byte (`:41-51`) and dispatches.
4. After the loop, enforce `msg.sender` did not lose more than `amountIn` of `tokenIn` (`:243-250`) and
   `to` gained at least `amountOutMin` of `tokenOut` (`:252-258`); emit `Route`.

Two pool handlers show the two DEX interaction styles:

*UniswapV2* `swapUniV2` (`:492-524`): stream `[pool, direction, to, fee]`. Transfer input **to the pool
first** (`:503-506`), then compute `amountIn` as `balanceOf(pool) - reserveIn` (`:514`, so fee-on-transfer
tokens are handled), apply the constant-product formula with `fee` in ppm (`:516-518`) and call
`pool.swap(amount0Out, amount1Out, to, "")` with no callback.

*UniswapV3* `swapUniV3` (`:552-585`): stream `[pool, direction, recipient]`. Records `lastCalledPool`
(`:575`) and calls `pool.swap(recipient, zeroForOne, int256(amountIn), priceLimit, abi.encode(tokenIn))`.
The pool then calls back `uniswapV3SwapCallback` (`:596-609`), which checks `msg.sender == lastCalledPool`,
resets it, and pays the pool the positive delta. Every V3-fork callback name is implemented
(`algebraSwapCallback`, `pancakeV3SwapCallback`, ... `:620-1066`) delegating to the same logic.

*Curve* `swapCurve` (`:1079-1136`): stream `[pool, poolType, i, j, to, tokenOut]`; approve and call
`exchange(i, j, amountIn, 0)`; for legacy pools measure output by balance delta because they return nothing.

`InputStream` (`:1618+`) is a tiny assembly cursor library: `createStream` stores `(pos, end)` in memory,
`readUint8/16/24/32`, `readUint`, `readBytes32`, `readAddress`, `readBytes` advance `pos` and `mload`.

### 6.8 `CalldataVerificationFacet`

`src/Facets/CalldataVerificationFacet.sol` exists so wallets, hardware-wallet clear-signing, and integrators
can decode what a LI.FI transaction will do *without* trusting the API:

* `extractBridgeData(data)` (`:21-25`) = `abi.decode(data[4:], (BridgeData))` (`_extractBridgeData`,
  `:324-328`): works because every facet's first parameter is `BridgeData`.
* `extractSwapData` (`:30-34`) decodes `(BridgeData, SwapData[])` — the layout of every
  `swapAndStartBridgeTokensVia*`.
* `extractMainParameters` (`:65-100`): if `hasSourceSwaps`, the *user-facing* input is `swapData[0]`
  (token and amount), else `BridgeData`.
* `extractGenericSwapParameters` (`:154-210`): same-chain calls, single vs multi by selector.
* `validateCalldata(data, bridge, sendingAssetId, receiver, amount, destinationChainId, hasSourceSwaps,
  hasDestinationCall)` (`:225-258`): boolean AND of all fields with "ignore" sentinels (`""`, `0xFFfF..F`,
  `type(uint256).max`).
* `validateDestinationCalldata(data, callTo, dstCalldata)` (`:265-317`): for Stargate V2 checks the
  composed message and `sendParams.to` match what the user expects.
* `extractNonEVMAddress` (`:111-145`): manual offset walk with an explicit bounds check (`:139-140`).

---

## 7. End-to-end traces

### 7.1 1000 USDC on Arbitrum → ETH on Base via Across, with a destination swap

Off-chain: the API quotes `USDC(Arb) → USDC(Base) via Across, then USDC → ETH on Base via a DEX`, and
returns calldata for `swapAndStartBridgeTokensViaAcrossV4`? No: no source swap is needed here, so it returns
`startBridgeTokensViaAcrossV4(bridgeData, acrossData)` with `hasSourceSwaps=false`,
`hasDestinationCall=true`, `acrossData.receiverAddress = ReceiverAcrossV4(Base)`,
`acrossData.message = abi.encode(txId, [SwapData{callTo: DEX, USDC→0x0 native ...}], userWallet)`. (If the
user started from, say, ARB, it would be the `swapAndStart...` variant and leg 1 would run first.)

```
ARBITRUM
 user ──approve(USDC, diamond)  (or Permit2Proxy §6.1)
 user ──► LiFiDiamond.fallback                                   src/LiFiDiamond.sol:32
        ├─ selector → AcrossFacetV4                             ds.selectorToFacetAndPosition[msg.sig]
        └─ delegatecall AcrossFacetV4.startBridgeTokensViaAcrossV4   src/Facets/AcrossFacetV4.sol:96
             modifiers: nonReentrant, refundExcessNative(user), validateBridgeData, doesNotContainSourceSwaps
             LibAsset.depositAsset(USDC, 1000e6)               LibAsset.sol:118  → USDC.transferFrom(user, diamond)
             _startBridge                                       AcrossFacetV4.sol:157
               message.length>0 == hasDestinationCall           :162
               receiver is EVM; hasDestinationCall → skip receiver equality  :190-194
               refundAddress != 0                               :202
               LibAsset.maxApproveERC20(USDC, SPOKEPOOL)        :225
               SPOKEPOOL.deposit(refund, ReceiverAcrossV4(Base), USDC, USDC(Base), 1000e6, outputAmount, 8453, ..., message)  :230
               emit LiFiTransferStarted(bridgeData)             :246
             refundExcessNative: nothing to refund
 events: LiFiTransferStarted, Across FundsDeposited
                                    ║  Across relayer sees the deposit, fills on Base out of its own USDC,
                                    ║  later repaid from the Arbitrum deposit after UMA verification
BASE
 relayer ──► SpokePool.fill...   transfers `outputAmount` USDC to ReceiverAcrossV4, then
   SpokePool ──► ReceiverAcrossV4.handleV3AcrossMessage(USDC, amount, relayer, message)  ReceiverAcrossV4.sol:57
       onlySpokepool                                            :24
       abi.decode(message) → (txId, swaps, userWallet)           :64-68
       _swapAndCompleteBridgeTokens                             :90
         USDC.safeApproveWithRetry(EXECUTOR, amount)             :97
         try EXECUTOR.swapAndCompleteBridgeTokens(txId, swaps, USDC, userWallet)   :99
              Executor._processSwaps(_depositAllowance=true)     Executor.sol:139
                allowance = USDC.allowance(receiver, executor); depositAsset(USDC, allowance)  :161-166
                _executeSwaps → LibSwap.swap(txId, swaps[0])      :217, LibSwap.sol:51
                    maxApprove(USDC → DEX); DEX.call(callData)   (USDC → ETH lands in Executor; receive() :263)
                leftover USDC → userWallet                       :183-190
                ETH delta → userWallet                           :192-202
                emit LiFiTransferCompleted                       :204
         catch → USDC.transfer(userWallet, amount); emit LiFiTransferRecovered   :107-117
         USDC.safeApprove(EXECUTOR, 0)                           :121
```

Event trail the backend reads: `LiFiTransferStarted(txId)` on Arbitrum, then either
`LiFiTransferCompleted(txId, USDC, user, ethAmount)` or `LiFiTransferRecovered(txId, USDC, user, usdcAmount)`
on Base.

### 7.2 Same-chain: 1 ETH → USDC via `GenericSwapFacetV3`

```
user ──► LiFiDiamond{value: 1 ETH}.swapTokensSingleV3NativeToERC20(txId, "integrator", "ref", user, minOut, swapData)
   fallback → delegatecall GenericSwapFacetV3                   GenericSwapFacetV3.sol:153
   LibAllowList.contractSelectorIsAllowed(DEX, selector)        :163-168   (else ContractCallNotAllowed)
   DEX.call{value: msg.value}(swapData.callData)                :172-174   (USDC arrives in diamond)
   _returnPositiveSlippageNative(user)                          :179       (any unspent ETH back)
   amountReceived = USDC.balanceOf(diamond)                     :183-185
   require amountReceived >= minOut                             :188-189
   USDC.transfer(user, amountReceived)                          :192
   emit AssetSwapped, LiFiGenericSwapCompleted                  :203-222
```

With a fee: the API would instead use `swapTokensMultipleV3NativeToERC20` with `swapData[0]` =
`FeeCollector.collectNativeFees` (native→native step) and `swapData[1]` = the DEX call.

### 7.3 Adding a new facet with `diamondCut`

```
LI.FI SC team: deploy FooFacet (immutables baked in), run DeployFooFacet/UpdateFooFacet scripts
proposer ──► LiFiTimelockController.schedule(target=Diamond, data=diamondCut([...Add FooFacet selectors...], init, initCalldata), delay ≥ 3h)
    ... 3h ...
executor ──► LiFiTimelockController.execute(...)
   timelock (the Diamond owner) ──► LiFiDiamond.diamondCut(cuts, _init, _calldata)
       fallback → delegatecall DiamondCutFacet.diamondCut       DiamondCutFacet.sol:18
         LibDiamond.enforceIsContractOwner()                    LibDiamond.sol:97
         LibDiamond.diamondCut                                  :103
           action == Add → addFunctions(FooFacet, selectors)    :136
             addFacet → enforceHasContractCode → facetAddresses.push   :241-250
             per selector: FunctionAlreadyExists? → addFunction :159-166
           emit DiamondCut(cuts, init, calldata)                :132
           initializeDiamondCut(init, calldata) → delegatecall  :326-352  (e.g. initFoo(chainIdConfigs))
owner ──► WhitelistManagerFacet.batchSetContractSelectorWhitelist(newDEXs, selectors, true)   (if new DEX routes)
owner ──► PeripheryRegistryFacet.registerPeripheryContract("ReceiverFoo", addr)               (if a receiver exists)
```

Emergency reverse path: `pauserWallet ──► EmergencyPauseFacet.removeFacet(FooFacet)` (seconds, no delay) or
`pauseDiamond()`; recovery via `timelock.unpauseDiamond([FooFacet])` (admin, no delay).

---

## 8. Security notes

**Arbitrary-call risk is the whole game.** The Diamond executes `callTo.call(callData)` with user funds
and user *approvals* attached to the Diamond's address. If `callTo`/selector were unrestricted, anyone could
encode `USDC.transferFrom(victim, attacker, amount)` and drain every wallet that ever approved the Diamond.
That is exactly what `LibAllowList` prevents (`SwapperV2.sol:204-215`, `GenericSwapFacetV3.sol:358-372`,
`GasZipPeriphery.sol:71-83`). Public LI.FI post-mortems (not part of this repo) describe two incidents of
this class: March 2022, where a swap path in an early facet allowed arbitrary calldata to whitelisted-less
targets, and July 2024, where a newly deployed GasZip facet called `LibSwap.swap` without the allow-list
check. Both drained approvals, not the Diamond's own balance. The current repo shows the consequences: the
whitelist became `(contract, selector)`-granular (`LibAllowList` v2.0.0), `GasZipFacet` is at v2.0.5 and
only swaps through `SwapperV2._depositAndSwap`, and every audit for GasZip/WhitelistManager since is listed in
`audit/reports/`.

**Approval residue.** `maxApproveERC20` leaves infinite allowances from the Diamond to DEXs and bridges.
Safe only while the Diamond's balance is ~0 at rest (§0.4). Anything that changes that invariant (a facet that
holds funds, a stuck transfer) turns every whitelisted spender into a risk. Periphery contracts reset
approvals to 0 after use for the same reason (`ReceiverAcrossV4.sol:121`, `Patcher.sol:263-265`).

**Upgrade trust.** The owner can `Replace` any selector with malicious code. Mitigations: timelock with a
3h delay (`docs/LiFiTimelockController.md`), Safe multisig as timelock admin, canceller role, and a
non-multisig pauser that can only *remove* code. A user interacting with the Diamond trusts that pipeline.

**Fee-on-transfer and rebasing tokens.** `LibSwap.swap` intentionally dropped its input-balance check
(`LibSwap.sol:78-82`); `GenericSwapFacetV3` tolerates 1 wei dust (`:584-587`); `LiFiDEXAggregator` measures
UniV2 input as pool delta (`:514`). stETH-style rebasing is handled by wrapping (§6.3). Balance-delta
accounting is the reason these mostly work, but "full balance" paths assume no pre-existing balance.

**Destination-call griefing.** Permissionless `lzCompose` with low gas forces the recovery path
(`ReceiverStargateV2.sol:81-92`). Users get bridged tokens but not the swap. Mitigated by `recoverGas`
reservation; not fully preventable on-chain.

**Calldata patching.** `Patcher` has no target whitelist and no refunds (`Patcher.sol:12-15`,
`docs/Patcher.md`). Anyone who approves it directly can be front-run.

**Opaque calldata paths.** `AcrossV4SwapFacet`, `RelayDepositoryFacet`, `MayanFacet` cannot fully validate
the destination receiver against `BridgeData.receiver`. The mitigations differ: backend EIP-712 signature
(Across Swap), per-selector receiver parsing (Mayan), or an explicit "we cannot guarantee" warning (Relay,
`RelayDepositoryFacet.sol:16-18`). Events could misreport what actually happens on those paths.

**Chain-id mapping.** Several facets keep an owner-set `chainId → protocolId` map (deBridge, PolymerCCTP).
A wrong mapping burns/locks funds to the wrong domain; that is why `PolymerCCTPFacet` validates hook
receivers byte-by-byte and why `AcrossV4SwapFacet` re-derives CCTP domain / LayerZero eid on-chain
(`:738-816`).

**Reentrancy.** Shared namespaced guard (`ReentrancyGuard.sol`) across facets; periphery contracts each have
their own. `FeeForwarder.forwardNativeFees` documents a subtle nested-call scenario (`:110-115`).

---

## 9. Exercises to trace yourself

1. **Selector routing.** Compute `bytes4(keccak256("startBridgeTokensViaAcrossV4((bytes32,string,string,address,address,address,uint256,uint256,bool,bool),(bytes32,bytes32,bytes32,bytes32,uint256,uint128,bytes32,uint32,uint32,uint32,bytes))"))` with `cast sig` and follow it through `src/LiFiDiamond.sol:43` into `src/Libraries/LibDiamond.sol:42`. Then read `addFunction` (`:252-265`) and explain what `functionSelectorPosition` is for.

2. **Leftover accounting.** Build a 3-step `SwapData[]` in your head: DAI→USDC (DEX A), USDC→WETH (DEX B) that only consumes 90% of the USDC, then WETH fee step. Walk `src/Helpers/SwapperV2.sol:297-356` and list exactly which transfers `_refundLeftovers` makes and to whom. Then repeat with `_nativeReserve = 0.01 ETH` and a final native asset.

3. **Whitelist edge case.** Explain why `(approveTo, 0xffffffff)` is needed by reading `src/Libraries/LibAllowList.sol:16-23` and `src/Helpers/SwapperV2.sol:204-215`. What would go wrong if `approveTo` were whitelisted with a *real* selector instead?

4. **Pause mechanics.** Read `src/Facets/EmergencyPauseFacet.sol:98-127`. After `pauseDiamond()`, what does `DiamondLoupeFacet.facets()` return, and why does `unpauseDiamond` (`:132-190`) use `replaceFunctions` rather than `addFunctions`?

5. **Destination fallback.** Simulate a revert inside the DEX call during `Executor._executeSwaps` (`src/Periphery/Executor.sol:217-234`) when invoked from `ReceiverAcrossV4` (`src/Periphery/ReceiverAcrossV4.sol:98-118`). Which contract ends up holding the USDC, what event fires, and what is the Executor's USDC allowance afterwards?

6. **ERC20Proxy invariant.** Try to construct a `SwapData` that would let a same-chain `swapAndExecute` caller drain another user's approval to `ERC20Proxy`, then find the exact line that stops it (`src/Periphery/Executor.sol:224-226`). What if a DEX router itself could be made to call `ERC20Proxy.transferFrom`?

7. **Route bytes.** Hand-encode a `LiFiDEXAggregator` route for "user USDC → 60% UniV2 pool P1, 40% UniV3 pool P2, both to `to`". Use the reads in `processUserERC20` (`:320-323`), `distributeAndSwap` (`:354-370`), `swapUniV2` (`:498-501`) and `swapUniV3` (`:558-560`). Then explain how `uniswapV3SwapCallback` (`:596-609`) knows which pool is legit.

8. **Calldata verification.** Take any real `swapAndStartBridgeTokensViaStargate` calldata (from a block explorer) and reproduce what `validateDestinationCalldata` (`src/Facets/CalldataVerificationFacet.sol:265-317`) checks. Why can it only do this for Stargate?

9. **Intent math.** For `LiFiIntentEscrowFacetV2.swapAndStartBridgeTokensViaLiFiIntentEscrowV2` (`:136-171`) with `minAmount = 990 USDC`, realized swap output `1005 USDC`, `outputAmountMultiplier = 0.997e18 * 10^(18-6)`, compute the committed `MandateOutput.amount` and explain why the positive slippage is *not* refunded here but *is* refunded in `AcrossV4SwapFacet` sponsored paths.
