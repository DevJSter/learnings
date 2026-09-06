# Uniswap V3 Periphery — Complete Reference

Every contract, every library, every function in `uni/v3-periphery/contracts`.
77 Solidity files, all enumerated. Line numbers verified with `grep -n` against
this tree; open the files beside this document.

The core (Pool, Factory, tick/price math) lives in a sibling document,
[`V3-CORE-COMPLETE-REFERENCE.md`](V3-CORE-COMPLETE-REFERENCE.md); the conceptual
walkthrough of why any of this exists is in
[`UNISWAP-DEEP-DIVE.md`](UNISWAP-DEEP-DIVE.md) §2. This file is the exhaustive
one: it assumes you know what a tick is and tells you what every line does.

---

## Table of contents

- [0. What "periphery" means](#0-what-periphery-means)
- [1. `base/` — the mixin layer](#1-base--the-mixin-layer)
- [2. `libraries/` — pure helpers](#2-libraries--pure-helpers)
- [3. `NonfungiblePositionManager`](#3-nonfungiblepositionmanager)
- [4. `SwapRouter`](#4-swaprouter)
- [5. `lens/` — read-only helpers](#5-lens--read-only-helpers)
- [6. `V3Migrator`](#6-v3migrator)
- [7. The on-chain SVG pipeline](#7-the-on-chain-svg-pipeline)
- [8. `examples/PairFlash`](#8-examplespairflash)
- [9. `interfaces/`](#9-interfaces)
- [10. `test/` helpers](#10-test-helpers)
- [11. ABI / selector tables](#11-abi--selector-tables)
- [12. Storage layout tables](#12-storage-layout-tables)
- [13. Events reference](#13-events-reference)
- [14. Revert-string decoder](#14-revert-string-decoder)
- [15. Use cases → call chains](#15-use-cases--call-chains)
- [16. Gotchas, collected](#16-gotchas-collected)

---

## 0. What "periphery" means

The V3 core is deliberately hostile to humans. `UniswapV3Pool.mint` does not pull
your tokens — it calls you back and demands them. It does not know what a
"position NFT" is; it stores positions under `keccak256(owner, tickLower,
tickUpper)`, so one address can only ever have *one* position per range. It takes
`liquidity` (an abstract `L`), not "I have 1000 USDC and 0.5 ETH". It has no
deadline check, no slippage check, no multi-hop, no ETH support.

The periphery supplies all of that. Nothing here is privileged: every contract is
a stateless-ish convenience wrapper that anyone could rewrite. That is the point.
Core is immutable and minimal; periphery is where the product lives, and Uniswap
has shipped several periphery generations against the same pools.

Three contracts matter in production:

| Contract | Job |
|---|---|
| `NonfungiblePositionManager` | Owns pool positions on your behalf, wraps each in an ERC-721 |
| `SwapRouter` | Multi-hop swaps, exact-in and exact-out, WETH handling, slippage, deadlines |
| `QuoterV2` | Simulates a swap off-chain and returns the amounts |

Everything else supports those three.

### 0.1 The dependency graph

```
                        ┌─────────────────────────┐
                        │   UniswapV3Factory      │  (core)
                        └───────────┬─────────────┘
                                    │ CREATE2, deterministic
                        ┌───────────▼─────────────┐
                        │   UniswapV3Pool         │  (core)
                        │  mint/burn/collect/swap │
                        └──▲────────────────▲─────┘
        mint + callback    │                │  swap + callback
                           │                │
     ┌─────────────────────┴───┐   ┌────────┴──────────────┐
     │ NonfungiblePositionMgr  │   │      SwapRouter       │
     │  (ERC721 over positions)│   │   (paths, slippage)   │
     └───────────┬─────────────┘   └────────┬──────────────┘
                 │                          │
                 │  both inherit ───────────┤
                 ▼                          ▼
     ┌───────────────────────────────────────────────────┐
     │ base/  PeripheryImmutableState  (factory, WETH9)  │
     │        PeripheryPayments        (pay, unwrap)     │
     │        PeripheryValidation      (checkDeadline)   │
     │        Multicall                (batching)        │
     │        SelfPermit               (gasless approve) │
     └───────────────────────────────────────────────────┘
                 │
                 ▼
     ┌───────────────────────────────────────────────────┐
     │ libraries/ PoolAddress, CallbackValidation, Path, │
     │            LiquidityAmounts, PositionKey, ...     │
     └───────────────────────────────────────────────────┘

     lens/  Quoter, QuoterV2, TickLens, InterfaceMulticall  (off-chain reads)
     NFTDescriptor + NFTSVG  →  tokenURI                    (on-chain art)
     V3Migrator                                             (V2 → V3)
```

### 0.2 File inventory (all 77)

Every `.sol` in the repo, with where it is covered here.

**Top level (4)**

| File | Lines | Section |
|---|---|---|
| `NonfungiblePositionManager.sol` | 400 | [§3](#3-nonfungiblepositionmanager) |
| `SwapRouter.sol` | 244 | [§4](#4-swaprouter) |
| `V3Migrator.sol` | 99 | [§6](#6-v3migrator) |
| `NonfungibleTokenPositionDescriptor.sol` | 124 | [§7.1](#71-nonfungibletokenpositiondescriptor) |

**`base/` (10)**

| File | Lines | Section |
|---|---|---|
| `BlockTimestamp.sol` | 12 | [§1.1](#11-blocktimestamp) |
| `ERC721Permit.sol` | 86 | [§1.9](#19-erc721permit) |
| `LiquidityManagement.sol` | 90 | [§1.10](#110-liquiditymanagement) |
| `Multicall.sol` | 28 | [§1.4](#14-multicall) |
| `PeripheryImmutableState.sol` | 18 | [§1.3](#13-peripheryimmutablestate) |
| `PeripheryPayments.sol` | 70 | [§1.6](#16-peripherypayments) |
| `PeripheryPaymentsWithFee.sol` | 55 | [§1.7](#17-peripherypaymentswithfee) |
| `PeripheryValidation.sol` | 11 | [§1.2](#12-peripheryvalidation) |
| `PoolInitializer.sol` | 32 | [§1.8](#18-poolinitializer) |
| `SelfPermit.sol` | 63 | [§1.5](#15-selfpermit) |

**`libraries/` (16)**

| File | Lines | Section |
|---|---|---|
| `BytesLib.sol` | 101 | [§2.5](#25-byteslib) |
| `CallbackValidation.sol` | 36 | [§2.2](#22-callbackvalidation) |
| `ChainId.sol` | 13 | [§2.7](#27-chainid) |
| `HexStrings.sol` | 29 | [§2.13](#213-hexstrings) |
| `LiquidityAmounts.sol` | 137 | [§2.8](#28-liquidityamounts) |
| `NFTDescriptor.sol` | 477 | [§7.2](#72-nftdescriptor) |
| `NFTSVG.sol` | 406 | [§7.3](#73-nftsvg) |
| `OracleLibrary.sol` | 180 | [§2.11](#211-oraclelibrary) |
| `Path.sol` | 69 | [§2.4](#24-path) |
| `PoolAddress.sol` | 48 | [§2.1](#21-pooladdress) |
| `PoolTicksCounter.sol` | 96 | [§2.12](#212-poolticjscounter) |
| `PositionKey.sol` | 13 | [§2.3](#23-positionkey) |
| `PositionValue.sol` | 167 | [§2.10](#210-positionvalue) |
| `SqrtPriceMathPartial.sol` | 62 | [§2.9](#29-sqrtpricemathpartial) |
| `TokenRatioSortOrder.sol` | 12 | [§2.14](#214-tokenratiosortorder) |
| `TransferHelper.sol` | 60 | [§2.6](#26-transferhelper) |

**`lens/` (4)**

| File | Lines | Section |
|---|---|---|
| `Quoter.sol` | 170 | [§5.1](#51-quoter) |
| `QuoterV2.sol` | 273 | [§5.2](#52-quoterv2) |
| `TickLens.sol` | 42 | [§5.3](#53-ticklens) |
| `UniswapInterfaceMulticall.sol` | 39 | [§5.4](#54-uniswapinterfacemulticall) |

**`examples/` (1)** — `PairFlash.sol` (149), [§8](#8-examplespairflash).

**`interfaces/` (18)** — all listed in [§9](#9-interfaces):
`IERC20Metadata`, `IERC721Permit`, `IMulticall`, `INonfungiblePositionManager`,
`INonfungibleTokenPositionDescriptor`, `IPeripheryImmutableState`,
`IPeripheryPayments`, `IPeripheryPaymentsWithFee`, `IPoolInitializer`, `IQuoter`,
`IQuoterV2`, `ISelfPermit`, `ISwapRouter`, `ITickLens`, `IV3Migrator`, and
`external/`: `IERC1271`, `IERC20PermitAllowed`, `IWETH9`.

**`test/` (24)** — all listed in [§10](#10-test-helpers).

### 0.3 Three patterns you will see everywhere

**(a) Compute the pool address, never ask the factory.** Every periphery contract
derives the pool address with `PoolAddress.computeAddress` (CREATE2 preimage)
instead of `factory.getPool()`. That is one `keccak256` instead of an external
`SLOAD`, and it works for pools that do not exist yet.

**(b) The callback is the authentication.** Core pools transfer tokens *out*
first, then call `msg.sender` back and check balances. So the periphery must
implement `uniswapV3MintCallback` / `uniswapV3SwapCallback` as public functions —
which means anyone can call them directly. The only defence is
`CallbackValidation.verifyCallback`: recompute the pool address from the pool key
in the callback data and `require(msg.sender == pool)`. Get this wrong and your
contract is drained by anyone who calls the callback with a payer of their
choosing. This is *the* most-copied-badly pattern in DeFi.

**(c) Every entry point is `payable`, so `Multicall` works.** `multicall`
`delegatecall`s into `address(this)`, so `msg.value` is visible to every sub-call.
Marking entry points `payable` lets them be batched. The consequence: `msg.value`
is *not* consumed per sub-call, so batching two ETH-spending calls in one
multicall lets both see the same ETH. Uniswap handles this with `refundETH` at the
end of the batch, but a naive fork can double-spend `msg.value`.

---

## 1. `base/` — the mixin layer

### 1.1 `BlockTimestamp`

`base/BlockTimestamp.sol` — abstract, 12 lines. Exists solely so tests can freeze
time.

#### `_blockTimestamp() internal view virtual returns (uint256)` — `base/BlockTimestamp.sol:9`

Returns `block.timestamp`. Overridden in `test/MockTimeNonfungiblePositionManager.sol:16`
and `test/MockTimeSwapRouter.sol:12` to return a settable value.

- **Checks / writes / calls / events:** none.
- **Callers:** `PeripheryValidation.checkDeadline` (`:8`), `ERC721Permit.permit` (`:63`).
- **Gotcha:** because it is `virtual`, a fork can make deadlines meaningless by
  overriding it. Verify the deployed bytecode, not the repo.

### 1.2 `PeripheryValidation`

`base/PeripheryValidation.sol` — abstract, 11 lines, `is BlockTimestamp`.

#### `modifier checkDeadline(uint256 deadline)` — `base/PeripheryValidation.sol:7`

```solidity
modifier checkDeadline(uint256 deadline) {
    require(_blockTimestamp() <= deadline, 'Transaction too old');
    _;
}
```

- **Revert:** `'Transaction too old'`.
- **Applied to:** `NonfungiblePositionManager.mint` (`:132`), `increaseLiquidity`
  (`:202`), `decreaseLiquidity` (`:262`); `SwapRouter.exactInputSingle` (`:119`),
  `exactInput` (`:136`), `exactOutputSingle` (`:207`), `exactOutput` (`:228`).
- **Not applied to:** `NonfungiblePositionManager.collect` and `burn`. Collecting
  fees at a stale time is harmless — there is no price exposure.
- **Gotcha:** a deadline of `type(uint256).max` disables MEV protection entirely.

### 1.3 `PeripheryImmutableState`

`base/PeripheryImmutableState.sol` — abstract, 18 lines, `is IPeripheryImmutableState`.

| Variable | Type | Line | Notes |
|---|---|---|---|
| `factory` | `address public immutable` | `:10` | V3 factory, used by `PoolAddress` |
| `WETH9` | `address public immutable` | `:12` | Wrapped native token |

Both are `immutable`, so they live in bytecode, not storage — reads are free.

#### `constructor(address _factory, address _WETH9)` — `base/PeripheryImmutableState.sol:14`

Assigns both immutables. No validation: a zero factory produces a contract whose
every pool address is garbage, and whose `verifyCallback` therefore always fails.

### 1.4 `Multicall`

`base/Multicall.sol` — abstract, 28 lines, `is IMulticall`.

#### `multicall(bytes[] calldata data) public payable returns (bytes[] memory results)` — `base/Multicall.sol:11`

Batches N calls to *this same contract* in one transaction.

```solidity
function multicall(bytes[] calldata data) public payable override returns (bytes[] memory results) {
    results = new bytes[](data.length);
    for (uint256 i = 0; i < data.length; i++) {
        (bool success, bytes memory result) = address(this).delegatecall(data[i]);
        if (!success) {
            if (result.length < 68) revert();
            assembly { result := add(result, 0x04) }
            revert(abi.decode(result, (string)));
        }
        results[i] = result;
    }
}
```

- **Parameters:** `data` — ABI-encoded calldata for each sub-call.
- **Checks:** none up front; each sub-call's own requires apply.
- **State writes:** whatever the sub-calls write.
- **External calls:** `delegatecall` to `address(this)` — so `msg.sender` and
  `msg.value` are preserved for every sub-call.
- **Returns:** the raw return data of each call.
- **Error bubbling:** on failure it strips the 4-byte `Error(string)` selector
  (`result := add(result, 0x04)`) and re-reverts with the decoded string, so the
  caller sees `'Too little received'` rather than opaque bytes. Revert data
  shorter than 68 bytes (custom errors, empty reverts, panics) is flattened into a
  bare `revert()` and the reason is lost.
- **Gotchas:**
  - `msg.value` is visible to *every* sub-call. Two `exactInputSingle` calls that
    each spend `msg.value` in one batch will both see the full amount. Always end
    an ETH batch with `refundETH`.
  - A no-value `multicall` is still `payable`, so ETH sent with a batch that never
    spends it is stuck unless the batch calls `refundETH`.

Test double: `test/TestMulticall.sol`.

### 1.5 `SelfPermit`

`base/SelfPermit.sol` — abstract, 63 lines, `is ISelfPermit`. Designed to be the
*first* element of a `multicall` so an EOA can approve and act in one transaction.

#### `selfPermit(address token, uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s) public payable` — `base/SelfPermit.sol:16`

Calls EIP-2612 `IERC20Permit(token).permit(msg.sender, address(this), value, deadline, v, r, s)`.
The spender is hard-coded to `address(this)`, so a signature harvested from the
mempool cannot be redirected to another spender.

- **Gotcha:** reverts if someone front-runs the same permit (nonce consumed).

#### `selfPermitIfNecessary(...) external payable` — `base/SelfPermit.sol:28`

Calls `selfPermit` only `if (IERC20(token).allowance(msg.sender, address(this)) < value)`.
This is the front-running fix: if an attacker already submitted your permit, the
allowance is set and this is a no-op instead of a revert.

#### `selfPermitAllowed(address token, uint256 nonce, uint256 expiry, uint8 v, bytes32 r, bytes32 s) public payable` — `base/SelfPermit.sol:40`

The DAI/CHAI permit variant (`IERC20PermitAllowed`, `interfaces/external/IERC20PermitAllowed.sol:17`):
no `value`, instead a boolean `allowed` which is passed as `true`, granting
**infinite** allowance.

#### `selfPermitAllowedIfNecessary(...) external payable` — `base/SelfPermit.sol:52`

Same guard, but the condition is `allowance < type(uint256).max` — because
`selfPermitAllowed` always grants infinite, anything less means the permit has not
run.

- **Inherited by:** `NonfungiblePositionManager` (`:31`), `SwapRouter` (`:28`),
  `V3Migrator` (`:20`).

### 1.6 `PeripheryPayments`

`base/PeripheryPayments.sol` — abstract, 70 lines, `is IPeripheryPayments, PeripheryImmutableState`.
The ETH/WETH/ERC-20 payment layer.

#### `receive() external payable` — `base/PeripheryPayments.sol:14`

```solidity
receive() external payable { require(msg.sender == WETH9, 'Not WETH9'); }
```

Only WETH9 may push raw ETH in (during `withdraw`). Users send ETH as `msg.value`
on a `payable` function, never by a bare transfer.

- **Revert:** `'Not WETH9'`.

#### `unwrapWETH9(uint256 amountMinimum, address recipient) public payable` — `base/PeripheryPayments.sol:19`

Unwraps the router's **entire** WETH9 balance and forwards it as ETH.

- **Parameters:** `amountMinimum` — slippage floor; `recipient` — who gets the ETH.
- **Checks:** `require(balanceWETH9 >= amountMinimum, 'Insufficient WETH9')`.
- **External calls:** `IWETH9.withdraw(balanceWETH9)` then
  `TransferHelper.safeTransferETH(recipient, balanceWETH9)`.
- **Gotcha:** it sweeps the balance, not `amountMinimum`. Any WETH left in the
  router by an earlier failed interaction is paid out to whoever calls this next.
  The router is *not* meant to hold funds between transactions; treat any residual
  balance as up for grabs.

#### `sweepToken(address token, uint256 amountMinimum, address recipient) public payable` — `base/PeripheryPayments.sol:30`

The ERC-20 twin of `unwrapWETH9`. `require(balanceToken >= amountMinimum, 'Insufficient token')`,
then transfers the full balance. Used to collect the output of a swap whose
`recipient` was `address(0)` (i.e. the router itself).

#### `refundETH() external payable` — `base/PeripheryPayments.sol:44`

`if (address(this).balance > 0) TransferHelper.safeTransferETH(msg.sender, address(this).balance);`
Returns leftover ETH to `msg.sender`. Must be the last call in any ETH multicall.

- **Gotcha:** sends the whole contract balance to `msg.sender`.

#### `pay(address token, address payer, address recipient, uint256 value) internal` — `base/PeripheryPayments.sol:52`

The three-way payment router used by both callbacks.

```solidity
if (token == WETH9 && address(this).balance >= value) {
    IWETH9(WETH9).deposit{value: value}();      // wrap only what is needed
    IWETH9(WETH9).transfer(recipient, value);
} else if (payer == address(this)) {
    TransferHelper.safeTransfer(token, recipient, value);   // intermediate hop
} else {
    TransferHelper.safeTransferFrom(token, payer, recipient, value);  // pull
}
```

Branch 1 = user sent ETH, wrap it. Branch 2 = the router already holds the token
(multi-hop intermediate). Branch 3 = pull from the user's approval.

- **Gotcha:** branch 1 triggers whenever `token == WETH9` **and** the contract has
  enough ETH — even if the caller intended to pay with their WETH allowance. In a
  multicall carrying `msg.value`, a WETH-input swap silently consumes the ETH
  instead of the WETH.

### 1.7 `PeripheryPaymentsWithFee`

`base/PeripheryPaymentsWithFee.sol` — abstract, 55 lines, `is PeripheryPayments, IPeripheryPaymentsWithFee`.
Adds an integrator fee cut. Inherited by `SwapRouter` (`:26`) but **not** by
`NonfungiblePositionManager`.

#### `unwrapWETH9WithFee(uint256 amountMinimum, address recipient, uint256 feeBips, address feeRecipient) public payable` — `base/PeripheryPaymentsWithFee.sol:17`

- **Checks:** `require(feeBips > 0 && feeBips <= 100)` (bare, no message) — the
  cap is **100 bips = 1%**; `require(balanceWETH9 >= amountMinimum, 'Insufficient WETH9')`.
- **Math:** `feeAmount = balanceWETH9 * feeBips / 10_000`, fee to `feeRecipient`,
  remainder to `recipient`.

#### `sweepTokenWithFee(address token, uint256 amountMinimum, address recipient, uint256 feeBips, address feeRecipient) public payable` — `base/PeripheryPaymentsWithFee.sol:37`

Identical shape for ERC-20s: same `feeBips` bounds, same `'Insufficient token'`
check, `safeTransfer` to fee recipient then to recipient.

- **Gotcha (both):** `feeBips == 0` **reverts**. To take no fee, call the plain
  `unwrapWETH9` / `sweepToken`.

### 1.8 `PoolInitializer`

`base/PoolInitializer.sol` — abstract, 32 lines, `is IPoolInitializer, PeripheryImmutableState`.

#### `createAndInitializePoolIfNecessary(address token0, address token1, uint24 fee, uint160 sqrtPriceX96) external payable returns (address pool)` — `base/PoolInitializer.sol:13`

Idempotent "make sure this pool exists and has a price".

- **Checks:** `require(token0 < token1)` (bare) — you must pre-sort.
- **Logic:** three cases.
  1. `factory.getPool(...) == 0` → `factory.createPool(...)` then `pool.initialize(sqrtPriceX96)`.
  2. Pool exists but `slot0().sqrtPriceX96 == 0` (created, never initialized) →
     `initialize(sqrtPriceX96)`.
  3. Pool exists and is initialized → do nothing, return it. **Your
     `sqrtPriceX96` is ignored.**
- **External calls:** `IUniswapV3Factory.getPool`, `.createPool`, `IUniswapV3Pool.initialize`, `.slot0`.
- **Gotcha:** case 3 is silent. Calling this before a mint does not guarantee the
  pool is at the price you asked for — check `slot0` yourself, or rely on
  `amount0Min`/`amount1Min` in the subsequent mint.
- **Inherited by:** `NonfungiblePositionManager` (`:28`), `V3Migrator` (`:20`).

### 1.9 `ERC721Permit`

`base/ERC721Permit.sol` — abstract, 86 lines, `is BlockTimestamp, ERC721, IERC721Permit`.
EIP-712 signature approvals for NFTs.

| Variable | Type | Line |
|---|---|---|
| `nameHash` | `bytes32 private immutable` | `:19` |
| `versionHash` | `bytes32 private immutable` | `:22` |
| `PERMIT_TYPEHASH` | `bytes32 public constant` | `:51` |

`PERMIT_TYPEHASH = 0x49ecf333e5b8c95c40fdafc95c1ad136e8914a8fb55e9dc8bb01eaa83a2df9ad`
= `keccak256("Permit(address spender,uint256 tokenId,uint256 nonce,uint256 deadline)")`.

#### `_getAndIncrementNonce(uint256 tokenId) internal virtual returns (uint256)` — `base/ERC721Permit.sol:16`

Abstract. Implemented by `NonfungiblePositionManager` (`:384`) as
`_positions[tokenId].nonce++`, packing the nonce into the position struct instead
of a separate mapping.

#### `constructor(string name_, string symbol_, string version_)` — `base/ERC721Permit.sol:25`

Stores `keccak256(bytes(name_))` and `keccak256(bytes(version_))` as immutables so
`DOMAIN_SEPARATOR()` can be recomputed cheaply per call.

#### `DOMAIN_SEPARATOR() public view returns (bytes32)` — `base/ERC721Permit.sol:35`

Recomputed on **every call** using `ChainId.get()` rather than cached at
construction. That is the fix for the post-fork replay bug: if the chain forks,
the separator changes automatically and old signatures die.

#### `permit(address spender, uint256 tokenId, uint256 deadline, uint8 v, bytes32 r, bytes32 s) external payable` — `base/ERC721Permit.sol:55`

Approves `spender` for `tokenId` from an off-chain signature.

1. `require(_blockTimestamp() <= deadline, 'Permit expired')`.
2. Build the EIP-712 digest over `(PERMIT_TYPEHASH, spender, tokenId, nonce++, deadline)`.
3. `owner = ownerOf(tokenId)` (reverts inside ERC721 for a nonexistent token).
4. `require(spender != owner, 'ERC721Permit: approval to current owner')`.
5. **Branch on contract vs EOA:**
   - `Address.isContract(owner)` → ERC-1271:
     `require(IERC1271(owner).isValidSignature(digest, abi.encodePacked(r, s, v)) == 0x1626ba7e, 'Unauthorized')`.
   - otherwise → `ecrecover`, `require(recoveredAddress != address(0), 'Invalid signature')`,
     `require(recoveredAddress == owner, 'Unauthorized')`.
6. `_approve(spender, tokenId)` — which the position manager overrides to write
   `_positions[tokenId].operator`.

- **State writes:** nonce increment (step 2, via `_getAndIncrementNonce`), operator (step 6).
- **Event:** `Approval(owner, spender, tokenId)` from the overridden `_approve` (`NonfungiblePositionManager.sol:398`).
- **Gotcha:** the nonce is consumed at digest-build time, *before* signature
  verification.
- **Gotcha:** the signature is `abi.encodePacked(r, s, v)` for ERC-1271, which is
  the 65-byte layout — note `r,s,v`, not `v,r,s`.
- **Test double:** `test/TestPositionNFTOwner.sol` implements `IERC1271`.

### 1.10 `LiquidityManagement`

`base/LiquidityManagement.sol` — abstract, 90 lines,
`is IUniswapV3MintCallback, PeripheryImmutableState, PeripheryPayments`.
The bridge between "I have X and Y tokens" and core's abstract `liquidity`.

```solidity
struct MintCallbackData { PoolAddress.PoolKey poolKey; address payer; }   // :19
struct AddLiquidityParams {                                              // :37
    address token0; address token1; uint24 fee; address recipient;
    int24 tickLower; int24 tickUpper;
    uint256 amount0Desired; uint256 amount1Desired;
    uint256 amount0Min; uint256 amount1Min;
}
```

#### `uniswapV3MintCallback(uint256 amount0Owed, uint256 amount1Owed, bytes calldata data) external` — `base/LiquidityManagement.sol:25`

Called **by the pool**, mid-`mint`, demanding payment.

```solidity
MintCallbackData memory decoded = abi.decode(data, (MintCallbackData));
CallbackValidation.verifyCallback(factory, decoded.poolKey);
if (amount0Owed > 0) pay(decoded.poolKey.token0, decoded.payer, msg.sender, amount0Owed);
if (amount1Owed > 0) pay(decoded.poolKey.token1, decoded.payer, msg.sender, amount1Owed);
```

- **Access control:** none declared — but `verifyCallback` recomputes the pool
  address from `decoded.poolKey` and requires `msg.sender` to equal it. Without
  that line, anyone could call this with `payer = victim` and drain every
  allowance granted to the position manager. **This single line is the security of
  the whole contract.**
- **Payer:** always `msg.sender` of the outer `addLiquidity` (`:85`), so a user
  can only ever spend their own approval.

#### `addLiquidity(AddLiquidityParams memory params) internal returns (uint128 liquidity, uint256 amount0, uint256 amount1, IUniswapV3Pool pool)` — `base/LiquidityManagement.sol:51`

1. Build the `PoolKey`, `pool = PoolAddress.computeAddress(factory, poolKey)` (`:63`).
   No existence check — a nonexistent pool just makes the next call revert.
2. Read `slot0().sqrtPriceX96`; convert both ticks to sqrt prices via
   `TickMath.getSqrtRatioAtTick` (`:68-69`).
3. `liquidity = LiquidityAmounts.getLiquidityForAmounts(...)` (`:71`) — the
   largest `L` fundable by both desired amounts. See [§2.8](#28-liquidityamounts).
4. `pool.mint(recipient, tickLower, tickUpper, liquidity, abi.encode(MintCallbackData{poolKey, payer: msg.sender}))`
   (`:80`). The pool re-enters `uniswapV3MintCallback` here and pulls the tokens.
5. `require(amount0 >= params.amount0Min && amount1 >= params.amount1Min, 'Price slippage check')` (`:88`).

- **The slippage check is inverted from what people expect.** For a swap you check
  that outputs are *large enough*. Here you check that the amounts actually
  deposited are *at least* the minimums — because if the price moved, the pool
  takes a *different ratio* than you planned. Set `amountMin` as a fraction of
  `amountDesired`.
- **Returns the pool** so callers can immediately read `pool.positions(...)`
  without recomputing the address.

---

## 2. `libraries/` — pure helpers

### 2.1 `PoolAddress`

`libraries/PoolAddress.sol` — 48 lines.

| Constant | Value | Line |
|---|---|---|
| `POOL_INIT_CODE_HASH` | `0xe34f199b19b2b4f47f68442619d555527d244f78a3297ea89325f843f87b8b54` | `:6` |

```solidity
struct PoolKey { address token0; address token1; uint24 fee; }   // :9
```

#### `getPoolKey(address tokenA, address tokenB, uint24 fee) internal pure returns (PoolKey memory)` — `libraries/PoolAddress.sol:20`

Sorts: `if (tokenA > tokenB) (tokenA, tokenB) = (tokenB, tokenA)`.

#### `computeAddress(address factory, PoolKey memory key) internal pure returns (address pool)` — `libraries/PoolAddress.sol:33`

```solidity
require(key.token0 < key.token1);
pool = address(uint256(keccak256(abi.encodePacked(
    hex'ff', factory,
    keccak256(abi.encode(key.token0, key.token1, key.fee)),   // salt
    POOL_INIT_CODE_HASH
))));
```

The standard CREATE2 preimage. Note the salt uses `abi.encode` (padded, 96 bytes)
while the outer hash uses `abi.encodePacked` — mixing these up is the classic
"my computed address is wrong" bug.

- **Gotcha:** `POOL_INIT_CODE_HASH` is a *compile-time constant of the pool
  bytecode*. Any chain whose V3 deployment used a different compiler version or
  optimizer setting has a different hash, and this library silently returns wrong
  addresses. Forks (PancakeSwap V3, etc.) always change it. Always verify against
  a known pool on the target chain.
- **Gotcha:** the returned address is a pure function; the pool may not exist.
  Every caller relies on a subsequent call reverting.
- **Test double:** `test/PoolAddressTest.sol`.

### 2.2 `CallbackValidation`

`libraries/CallbackValidation.sol` — 36 lines. The security keystone.

#### `verifyCallback(address factory, address tokenA, address tokenB, uint24 fee) internal view returns (IUniswapV3Pool pool)` — `libraries/CallbackValidation.sol:15`

Sugar: `verifyCallback(factory, PoolAddress.getPoolKey(tokenA, tokenB, fee))`.

#### `verifyCallback(address factory, PoolAddress.PoolKey memory poolKey) internal view returns (IUniswapV3Pool pool)` — `libraries/CallbackValidation.sol:28`

```solidity
pool = IUniswapV3Pool(PoolAddress.computeAddress(factory, poolKey));
require(msg.sender == address(pool));
```

Two lines. The bare `require` (no message) at `:34` is what stands between the
router's users and anyone who calls `uniswapV3SwapCallback` directly.

- **Why it works:** an attacker can pass any `poolKey`, but the computed address
  must equal `msg.sender`. To satisfy that they would have to *be* a legitimate
  CREATE2-derived pool, which requires the factory to have deployed them.
- **Callers:** `LiquidityManagement:31`, `SwapRouter:65`, `Quoter:45`,
  `QuoterV2:48`, `PairFlash:52`.
- **Test double:** `test/TestCallbackValidation.sol`.

### 2.3 `PositionKey`

`libraries/PositionKey.sol` — 13 lines.

#### `compute(address owner, int24 tickLower, int24 tickUpper) internal pure returns (bytes32)` — `libraries/PositionKey.sol:6`

`keccak256(abi.encodePacked(owner, tickLower, tickUpper))` — the key under which
core stores positions.

- **The critical consequence:** the position manager passes
  `owner = address(this)`, so **every NFT for the same tick range shares one core
  position**. Two users' NFTs on WETH/USDC 0.3% [200000, 201000] are one entry in
  the pool. The manager must therefore track each NFT's share of the aggregate
  itself — that is what `feeGrowthInside*LastX128` and `tokensOwed*` in the
  `Position` struct are for. See [§3.14](#314-the-fee-reconciliation-invariant).

### 2.4 `Path`

`libraries/Path.sol` — 69 lines, `using BytesLib for bytes`. Encodes multi-hop routes.

| Constant | Value | Line |
|---|---|---|
| `ADDR_SIZE` | 20 | `:11` |
| `FEE_SIZE` | 3 | `:13` |
| `NEXT_OFFSET` | 23 | `:16` |
| `POP_OFFSET` | 43 | `:18` |
| `MULTIPLE_POOLS_MIN_LENGTH` | 66 | `:20` |

Layout — tokens and fees tightly packed, no ABI padding:

```
1 pool  (43 bytes):  [tokenA 20][fee 3][tokenB 20]
2 pools (66 bytes):  [tokenA 20][fee 3][tokenB 20][fee 3][tokenC 20]
3 pools (89 bytes):  ... + [fee 3][tokenD 20]
                     └── each extra hop adds NEXT_OFFSET = 23 bytes
```

Build with `abi.encodePacked(tokenA, fee1, tokenB, fee2, tokenC)`.

#### `hasMultiplePools(bytes memory path) internal pure returns (bool)` — `libraries/Path.sol:25`

`path.length >= 66`.

#### `numPools(bytes memory path) internal pure returns (uint256)` — `libraries/Path.sol:32`

`(path.length - 20) / 23`. Used only by QuoterV2 to size its result arrays.

#### `decodeFirstPool(bytes memory path) internal pure returns (address tokenA, address tokenB, uint24 fee)` — `libraries/Path.sol:42`

Reads `toAddress(0)`, `toUint24(20)`, `toAddress(23)`. Note the **return order is
`(tokenA, tokenB, fee)` but the encoding order is `token, fee, token`** — a
frequent source of confusion when reading call sites.

#### `getFirstPool(bytes memory path) internal pure returns (bytes memory)` — `libraries/Path.sol:59`

`path.slice(0, 43)` — the first hop only, to pass as callback data.

#### `skipToken(bytes memory path) internal pure returns (bytes memory)` — `libraries/Path.sol:66`

`path.slice(23, path.length - 23)` — drops the leading `[token][fee]`, leaving a
path that starts at the next token.

- **Gotcha:** none of these validate length. A malformed path reverts inside
  `BytesLib` with `'slice_outOfBounds'` / `'toAddress_outOfBounds'`.
- **Gotcha:** for **exact-output** the path is encoded **backwards**: output token
  first. `SwapRouter.exactOutputSingle` builds `abi.encodePacked(tokenOut, fee, tokenIn)`
  (`SwapRouter.sol:215`), and `exactOutputInternal` destructures it as
  `(tokenOut, tokenIn, fee)` (`:178`).
- **Test double:** `test/PathTest.sol`.

### 2.5 `BytesLib`

`libraries/BytesLib.sol` — 101 lines. Gonçalo Sá's library, trimmed to three
functions. All are assembly.

#### `slice(bytes memory _bytes, uint256 _start, uint256 _length) internal pure returns (bytes memory)` — `libraries/BytesLib.sol:12`

- **Checks:** `require(_length + 31 >= _length, 'slice_overflow')`,
  `require(_start + _length >= _start, 'slice_overflow')`,
  `require(_bytes.length >= _start + _length, 'slice_outOfBounds')`.
- Copies word-by-word into fresh memory, then aligns the free-memory pointer to a
  32-byte boundary. The zero-length case returns an empty `bytes` and still bumps
  the pointer.

#### `toAddress(bytes memory _bytes, uint256 _start) internal pure returns (address)` — `libraries/BytesLib.sol:78`

`require(_start + 20 >= _start, 'toAddress_overflow')`,
`require(_bytes.length >= _start + 20, 'toAddress_outOfBounds')`, then
`mload(add(_bytes, add(0x20, _start)))` shifted right by 96 bits.

#### `toUint24(bytes memory _bytes, uint256 _start) internal pure returns (uint24)` — `libraries/BytesLib.sol:90`

Same shape with `'toUint24_overflow'` / `'toUint24_outOfBounds'` and a 3-byte load.

- **Note:** the `_start + N >= _start` overflow checks are dead code under 0.8.x
  but this file is pinned `>=0.5.0 <0.8.0`, where arithmetic wraps.

### 2.6 `TransferHelper`

`libraries/TransferHelper.sol` — 60 lines. Handles tokens that return nothing
instead of `true` (USDT, BNB and friends).

The shape is identical in all three ERC-20 functions: low-level `call` with an
encoded selector, then
`require(success && (data.length == 0 || abi.decode(data, (bool))), '<CODE>')`.
Empty return data counts as success.

| Function | Line | Selector called | Revert |
|---|---|---|---|
| `safeTransferFrom(address,address,address,uint256)` | `:13` | `transferFrom` | `'STF'` |
| `safeTransfer(address,address,uint256)` | `:29` | `transfer` | `'ST'` |
| `safeApprove(address,address,uint256)` | `:43` | `approve` | `'SA'` |
| `safeTransferETH(address,uint256)` | `:56` | — (`to.call{value:}("")`) | `'STE'` |

- **Gotcha:** `safeTransferETH` forwards **all remaining gas**. A recipient
  contract can consume it or re-enter. Used in `unwrapWETH9`, `refundETH`, and
  the `WithFee` variants; none of those hold state that reentrancy would corrupt,
  but a fork that adds state must add a guard.
- **Gotcha:** `safeApprove` does not do the USDT "reset to zero first" dance.
  `V3Migrator` (`:50-51`, `:74`, `:87`) works around this by only approving from a
  known-zero state and resetting to zero afterwards.

### 2.7 `ChainId`

`libraries/ChainId.sol` — 13 lines.

#### `get() internal pure returns (uint256 chainId)` — `libraries/ChainId.sol:8`

`assembly { chainId := chainid() }`. Marked `pure` while reading an opcode — legal
because the compiler cannot see inside assembly, and a lie that lets
`DOMAIN_SEPARATOR()` stay `view`.

- **Callers:** `ERC721Permit.DOMAIN_SEPARATOR` (`:43`),
  `NonfungibleTokenPositionDescriptor.tokenURI` (`:65`).

### 2.8 `LiquidityAmounts`

`libraries/LiquidityAmounts.sol` — 137 lines. Converts between token amounts and
the abstract `L`. This is the math that makes V3 usable.

**The derivation.** Inside a range `[Pa, Pb]` a position of liquidity `L` holds

```
amount0 = L * (1/√P − 1/√Pb)      (token0 is held when price is below Pb)
amount1 = L * (√P − √Pa)          (token1 is held when price is above Pa)
```

Solving each for `L` gives the two "ForAmount" functions; substituting `P` gives
the two "ForLiquidity" functions.

#### `toUint128(uint256 x) private pure returns (uint128 y)` — `libraries/LiquidityAmounts.sol:13`

`require((y = uint128(x)) == x)` — bare revert on overflow.

#### `getLiquidityForAmount0(uint160 sqrtRatioAX96, uint160 sqrtRatioBX96, uint256 amount0) internal pure returns (uint128)` — `:23`

`L = amount0 * (√Pa·√Pb) / (√Pb − √Pa)`. In code, `intermediate = mulDiv(√Pa, √Pb, Q96)`
then `mulDiv(amount0, intermediate, √Pb − √Pa)`. Sorts its inputs first.

#### `getLiquidityForAmount1(uint160 sqrtRatioAX96, uint160 sqrtRatioBX96, uint256 amount1) internal pure returns (uint128)` — `:39`

`L = amount1 * Q96 / (√Pb − √Pa)`. Simpler because `amount1` is linear in `√P`.

#### `getLiquidityForAmounts(uint160 sqrtRatioX96, uint160 sqrtRatioAX96, uint160 sqrtRatioBX96, uint256 amount0, uint256 amount1) internal pure returns (uint128 liquidity)` — `:56`

Three cases on where the current price sits:

```
P ≤ Pa   (entirely below range)  → position is 100% token0 → getLiquidityForAmount0(Pa, Pb, amount0)
Pa < P < Pb (in range)           → min( getLiquidityForAmount0(P, Pb, amount0),
                                        getLiquidityForAmount1(Pa, P, amount1) )
P ≥ Pb   (entirely above range)  → position is 100% token1 → getLiquidityForAmount1(Pa, Pb, amount1)
```

The `min` in the middle case is why you often deposit less than one of your
desired amounts: `L` is capped by whichever token you have relatively less of.

- **Caller:** `LiquidityManagement.addLiquidity:71`.

#### `getAmount0ForLiquidity(uint160 sqrtRatioAX96, uint160 sqrtRatioBX96, uint128 liquidity) internal pure returns (uint256)` — `:82`

`mulDiv(L << 96, √Pb − √Pa, √Pb) / √Pa`. Rounds **down** — unlike core's
`SqrtPriceMath`, this library has no `roundUp` flag because it is only used for
display and valuation, never for settlement.

#### `getAmount1ForLiquidity(uint160 sqrtRatioAX96, uint160 sqrtRatioBX96, uint128 liquidity) internal pure returns (uint256)` — `:102`

`mulDiv(L, √Pb − √Pa, Q96)`.

#### `getAmountsForLiquidity(uint160 sqrtRatioX96, uint160 sqrtRatioAX96, uint160 sqrtRatioBX96, uint128 liquidity) internal pure returns (uint256 amount0, uint256 amount1)` — `:120`

Mirror of `getLiquidityForAmounts`: below range → all token0; in range → both
(split at `P`); above range → all token1.

- **Caller:** `PositionValue.principal:47`.
- **Test double:** `test/LiquidityAmountsTest.sol`.

### 2.9 `SqrtPriceMathPartial`

`libraries/SqrtPriceMathPartial.sol` — 62 lines. Two functions copied verbatim
from core's `SqrtPriceMath`, re-exported so periphery need not link the whole
library (which would blow the size limit).

#### `getAmount0Delta(uint160 sqrtRatioAX96, uint160 sqrtRatioBX96, uint128 liquidity, bool roundUp) internal pure returns (uint256 amount0)` — `:20`

`L * (√Pb − √Pa) / (√Pb · √Pa)`, with `require(sqrtRatioAX96 > 0)` (bare) to avoid
division by zero. `roundUp` selects `mulDivRoundingUp` + `divRoundingUp` versus
plain `mulDiv` + `/`.

#### `getAmount1Delta(uint160 sqrtRatioAX96, uint160 sqrtRatioBX96, uint128 liquidity, bool roundUp) internal pure returns (uint256 amount1)` — `:49`

`L * (√Pb − √Pa) / Q96`, same rounding switch.

- **Note:** nothing in this repo imports it — it exists for downstream
  integrators. See `V3-CORE-COMPLETE-REFERENCE.md` for how core uses the originals
  and why rounding direction is a security property.

### 2.10 `PositionValue`

`libraries/PositionValue.sol` — 167 lines. "What is this NFT worth right now?"
without sending a transaction.

#### `total(INonfungiblePositionManager positionManager, uint256 tokenId, uint160 sqrtRatioX96) internal view returns (uint256 amount0, uint256 amount1)` — `:22`

`principal(...) + fees(...)`, component-wise.

#### `principal(INonfungiblePositionManager positionManager, uint256 tokenId, uint160 sqrtRatioX96) internal view returns (uint256, uint256)` — `:39`

Reads ticks and liquidity from `positions(tokenId)`, then
`LiquidityAmounts.getAmountsForLiquidity(sqrtRatioX96, tickLower→√P, tickUpper→√P, liquidity)`.

- **Gotcha:** `sqrtRatioX96` is a **caller-supplied parameter**, not read from the
  pool. That is intentional (you may want to value at a hypothetical price) and
  dangerous (passing a manipulated spot price gives a manipulated valuation). Any
  protocol using this as an oracle must pass a TWAP-derived price, not `slot0`.

#### `fees(INonfungiblePositionManager positionManager, uint256 tokenId) internal view returns (uint256 amount0, uint256 amount1)` — `:73`

Destructures all 12 return values of `positions(tokenId)` into a `FeeParams`
struct (`:55`) — the struct exists purely to dodge stack-too-deep — and delegates
to `_fees`.

#### `_fees(INonfungiblePositionManager positionManager, FeeParams memory feeParams) private view returns (uint256 amount0, uint256 amount1)` — `:111`

```
amount = mulDiv(poolFeeGrowthInside − positionFeeGrowthInsideLast, liquidity, Q128) + tokensOwed
```

That is exactly the reconciliation the position manager performs on-chain
([§3.14](#314-the-fee-reconciliation-invariant)), done as a view.

#### `_getFeeGrowthInside(IUniswapV3Pool pool, int24 tickLower, int24 tickUpper) private view returns (uint256, uint256)` — `:145`

Reimplements core's `Tick.getFeeGrowthInside` from public getters, because core
does not expose it externally. Three cases on `tickCurrent`:

```
tickCurrent <  tickLower :  inside = lowerOutside − upperOutside
tickLower ≤ tickCurrent < tickUpper : inside = global − lowerOutside − upperOutside
tickCurrent ≥ tickUpper :  inside = upperOutside − lowerOutside
```

All arithmetic is deliberately allowed to underflow (the file is pinned
`>=0.6.8 <0.8.0`); only differences are meaningful. Under 0.8.x this code would
revert — a real hazard when porting.

- **Test double:** `test/PositionValueTest.sol`.

### 2.11 `OracleLibrary`

`libraries/OracleLibrary.sol` — 180 lines. Safe consumption of the pool's
observation ring buffer.

#### `consult(address pool, uint32 secondsAgo) internal view returns (int24 arithmeticMeanTick, uint128 harmonicMeanLiquidity)` — `:16`

The main TWAP entry point.

1. `require(secondsAgo != 0, 'BP')`.
2. `pool.observe([secondsAgo, 0])` → cumulative tick and seconds-per-liquidity at
   both ends.
3. `arithmeticMeanTick = tickCumulativesDelta / secondsAgo`, then
   **`if (tickCumulativesDelta < 0 && delta % secondsAgo != 0) arithmeticMeanTick--`**
   (`:36`). Solidity truncates toward zero; this forces round-toward-negative-infinity
   so the result is consistent for negative ticks. Omitting it produces a
   one-tick (~0.01%) bias that a determined arbitrageur can farm.
4. `harmonicMeanLiquidity = (secondsAgo * type(uint160).max) / (secondsPerLiquidityDelta << 32)`.
   Multiplying rather than shifting keeps the result inside `uint128`.

Because the mean is over **ticks** (log-price), the implied price is a **geometric**
mean, not arithmetic.

#### `getQuoteAtTick(int24 tick, uint128 baseAmount, address baseToken, address quoteToken) internal pure returns (uint256 quoteAmount)` — `:49`

Converts a tick into "how much quoteToken for `baseAmount` of baseToken".

- Branches on precision: if `sqrtRatioX96 <= type(uint128).max` it squares to
  `ratioX192` and uses `1 << 192`; otherwise it pre-shifts to `ratioX128` to avoid
  overflow.
- Direction: `baseToken < quoteToken` selects multiply-by-ratio versus
  divide-by-ratio.

#### `getOldestObservationSecondsAgo(address pool) internal view returns (uint32 secondsAgo)` — `:74`

`require(observationCardinality > 0, 'NI')`. Reads observation
`(observationIndex + 1) % cardinality` — the oldest slot in the ring. If it is not
`initialized` (cardinality is mid-growth) it falls back to index 0.

- **Use this before `consult`.** Asking for a longer window than the buffer holds
  makes `observe` revert with core's `OLD`.

#### `getBlockStartingTickAndLiquidity(address pool) internal view returns (int24, uint128)` — `:93`

The tick as of the start of the current block — manipulation-resistant within one
block.

- `require(observationCardinality > 1, 'NEO')`.
- If the newest observation's timestamp `!= block.timestamp`, nothing moved the
  price this block, so `slot0.tick` and `pool.liquidity()` are already correct.
- Otherwise it steps back one slot, `require(prevInitialized, 'ONI')`, and derives
  the tick from the cumulative difference.

#### `getWeightedArithmeticMeanTick(WeightedTickData[] memory weightedTickData) internal pure returns (int24)` — `:140`

`Σ(tick·weight) / Σweight`, with the same round-toward-negative-infinity
correction (`:159`). `struct WeightedTickData { int24 tick; uint128 weight; }` (`:129`).
Used to combine several fee tiers of the same pair, weighted by liquidity.

- **Gotcha:** no guard on `denominator == 0` — an empty array panics.

#### `getChainedPrice(address[] memory tokens, int24[] memory ticks) internal pure returns (int256 syntheticTick)` — `:168`

`require(tokens.length - 1 == ticks.length, 'DL')`, then accumulates each tick
with `+` or `−` depending on the sort order of the adjacent token pair, so
intermediate tokens cancel. Gives a synthetic A→C tick from A→B and B→C.

- **Test double:** `test/OracleTest.sol`; mocks `test/MockObservable.sol`, `test/MockObservations.sol`.

### 2.12 `PoolTicksCounter`

`libraries/PoolTicksCounter.sol` — 96 lines. Counts initialized ticks a swap
crossed, so QuoterV2 can report gas realistically.

#### `countInitializedTicksCrossed(IUniswapV3Pool self, int24 tickBefore, int24 tickAfter) internal view returns (uint32 initializedTicksCrossed)` — `:11`

1. Compute `(wordPos, bitPos)` for both ticks: `wordPos = (tick / tickSpacing) >> 8`,
   `bitPos = (tick / tickSpacing) % 256` (`:25-29`).
2. Decide whether the endpoints themselves count (`:35-45`). An initialized
   endpoint is only crossed if the swap moves *away* from it:
   - `tickAfterInitialized` requires the bit set, `tickAfter % tickSpacing == 0`,
     **and** `tickBefore > tickAfter` (swapping down).
   - `tickBeforeInitialized` requires the bit set, alignment, **and**
     `tickBefore < tickAfter` (swapping up).
3. Order the endpoints low→high (`:47-57`).
4. Walk the bitmap words, masking off bits outside the range, summing popcounts
   (`:62-75`). First mask is `type(uint256).max << bitPosLower`; the final word is
   additionally masked with `type(uint256).max >> (255 - bitPosHigher)`.
5. Subtract the endpoints that should not count (`:77-83`).

#### `countOneBits(uint256 x) private pure returns (uint16)` — `:88`

Kernighan's popcount: `while (x != 0) { bits++; x &= (x - 1); }`.

- **Gotcha:** `tick / tickSpacing` truncates toward zero, so for negative ticks the
  word/bit derivation differs from core's `TickBitmap.position`, which floors.
  This is acceptable for a gas *estimate* but means the count can be off by one
  near negative boundaries. Do not use it for settlement.
- **Test double:** `test/PoolTicksCounterTest.sol`.

### 2.13 `HexStrings`

`libraries/HexStrings.sol` — 29 lines. MIT, adapted from OpenZeppelin.

| Constant | Value | Line |
|---|---|---|
| `ALPHABET` | `bytes16 '0123456789abcdef'` | `:5` |

#### `toHexString(uint256 value, uint256 length) internal pure returns (string memory)` — `:9`

Fixed-length hex **with** `0x`. Buffer is `2*length + 2`; fills backwards from the
end; `require(value == 0, 'Strings: hex length insufficient')` catches a value too
large for the requested width.

#### `toHexStringNoPrefix(uint256 value, uint256 length) internal pure returns (string memory)` — `:21`

Same without `0x` and **without** the sufficiency check — used for colour hex
where truncation is intended.

- **Callers:** `NFTDescriptor.addressToString:405` (via `toHexString(20)`),
  `NFTDescriptor.tokenToColorHex:462` (via `toHexStringNoPrefix(3)`).

### 2.14 `TokenRatioSortOrder`

`libraries/TokenRatioSortOrder.sol` — 12 lines, six constants, no functions.

| Constant | Value |
|---|---|
| `NUMERATOR_MOST` | 300 |
| `NUMERATOR_MORE` | 200 |
| `NUMERATOR` | 100 |
| `DENOMINATOR_MOST` | −300 |
| `DENOMINATOR_MORE` | −200 |
| `DENOMINATOR` | −100 |

Priority ladder for deciding which token is the price *denominator* in the NFT
title, so it reads "ETH/USDC = 3000" rather than "0.000333". Consumed by
`NonfungibleTokenPositionDescriptor.tokenRatioPriority` (`:103`).

---

## 3. `NonfungiblePositionManager`

`NonfungiblePositionManager.sol` — 400 lines. Solidity `=0.7.6`, `abicoder v2`.

### 3.1 Inheritance linearization

```solidity
contract NonfungiblePositionManager is
    INonfungiblePositionManager,   // :24
    Multicall,                     // :25
    ERC721Permit,                  // :26  → BlockTimestamp, ERC721 (OZ), IERC721Permit
    PeripheryImmutableState,       // :27
    PoolInitializer,               // :28
    LiquidityManagement,           // :29  → IUniswapV3MintCallback, PeripheryPayments
    PeripheryValidation,           // :30  → BlockTimestamp
    SelfPermit                     // :31
```

C3 linearization, most-derived last:

```
NonfungiblePositionManager
 └─ SelfPermit
     └─ PeripheryValidation
         └─ LiquidityManagement
             └─ PeripheryPayments ── IPeripheryPayments
                 └─ PoolInitializer
                     └─ PeripheryImmutableState ── IPeripheryImmutableState
                         └─ ERC721Permit
                             └─ ERC721 (OpenZeppelin) ── IERC721Metadata, IERC721Enumerable
                                 └─ BlockTimestamp
                                     └─ Multicall ── IMulticall
                                         └─ INonfungiblePositionManager
```

`BlockTimestamp` is reached through both `ERC721Permit` and `PeripheryValidation`
— the diamond that makes the explicit ordering necessary.

The contract inherits the whole of OpenZeppelin's ERC721 (transfers, approvals,
enumeration, `safeTransferFrom` with receiver checks). Only `tokenURI`,
`baseURI`, `getApproved` and `_approve` are overridden.

### 3.2 Storage layout

```solidity
struct Position {                       // :34
    uint96  nonce;                      // :36   ┐ slot n
    address operator;                   // :38   ┘ (96 + 160 = 256 bits, packed)
    uint80  poolId;                     // :40   ┐
    int24   tickLower;                  // :42   │ slot n+1
    int24   tickUpper;                  // :43   │ (80 + 24 + 24 + 128 = 256, packed)
    uint128 liquidity;                  // :45   ┘
    uint256 feeGrowthInside0LastX128;   // :47     slot n+2
    uint256 feeGrowthInside1LastX128;   // :48     slot n+3
    uint128 tokensOwed0;                // :50   ┐ slot n+4
    uint128 tokensOwed1;                // :51   ┘ (packed)
}
```

Five slots per position, carefully packed. The `poolId` indirection is the reason:
storing `(token0, token1, fee)` per position would cost two more slots each.

| Variable | Type | Line | Purpose |
|---|---|---|---|
| `_poolIds` | `mapping(address => uint80) private` | `:55` | pool address → compact id |
| `_poolIdToPoolKey` | `mapping(uint80 => PoolAddress.PoolKey) private` | `:58` | id → key |
| `_positions` | `mapping(uint256 => Position) private` | `:61` | tokenId → position |
| `_nextId` | `uint176 private = 1` | `:64` | next token id, skips 0 |
| `_nextPoolId` | `uint80 private = 1` | `:66` | next pool id, skips 0 |
| `_tokenDescriptor` | `address private immutable` | `:69` | SVG renderer |

`_nextId` and `_nextPoolId` pack into one slot (176 + 80 = 256). Both start at 1
so that `poolId == 0` reliably means "no such position" — the check in `positions`
(`:100`) and the implicit one in `cachePoolKey` (`:121`).

### 3.3 `constructor`

`constructor(address _factory, address _WETH9, address _tokenDescriptor_)` — `NonfungiblePositionManager.sol:71`

```solidity
ERC721Permit('Uniswap V3 Positions NFT-V1', 'UNI-V3-POS', '1')
PeripheryImmutableState(_factory, _WETH9)
```

Sets `_tokenDescriptor = _tokenDescriptor_` (`:76`).

- **Gotcha:** `_tokenDescriptor` is `immutable`, so the art contract can never be
  swapped. Mainnet did upgrade descriptors — by deploying a new descriptor and
  upgrading the proxy that fronts it, not by changing this field.

### 3.4 `positions`

`positions(uint256 tokenId) external view returns (uint96 nonce, address operator, address token0, address token1, uint24 fee, int24 tickLower, int24 tickUpper, uint128 liquidity, uint256 feeGrowthInside0LastX128, uint256 feeGrowthInside1LastX128, uint128 tokensOwed0, uint128 tokensOwed1)` — `:80`

Twelve return values: the packed `Position` plus the pool key expanded from `poolId`.

- **Checks:** `require(position.poolId != 0, 'Invalid token ID')` (`:100`).
- **Gotcha:** `tokensOwed0/1` are the values *as of the last write*. Fees accrued
  since are not included. To see live fees use `PositionValue.fees` or simulate
  `collect` with `staticcall`.
- **Gas test double:** `test/NonfungiblePositionManagerPositionsGasTest.sol`.

### 3.5 `cachePoolKey`

`cachePoolKey(address pool, PoolAddress.PoolKey memory poolKey) private returns (uint80 poolId)` — `:119`

```solidity
poolId = _poolIds[pool];
if (poolId == 0) {
    _poolIds[pool] = (poolId = _nextPoolId++);
    _poolIdToPoolKey[poolId] = poolKey;
}
```

Idempotent. First mint into a pool costs the extra SSTOREs; every later mint into
the same pool is a single SLOAD.

### 3.6 `mint`

`mint(MintParams calldata params) external payable checkDeadline(params.deadline) returns (uint256 tokenId, uint128 liquidity, uint256 amount0, uint256 amount1)` — `:128`

```solidity
struct MintParams {                      // interfaces/INonfungiblePositionManager.sol:78
    address token0; address token1; uint24 fee;
    int24 tickLower; int24 tickUpper;
    uint256 amount0Desired; uint256 amount1Desired;
    uint256 amount0Min; uint256 amount1Min;
    address recipient; uint256 deadline;
}
```

| Parameter | Meaning |
|---|---|
| `token0`, `token1` | **Must be sorted** (`token0 < token1`); enforced downstream by `PoolAddress.computeAddress` |
| `fee` | Pool fee tier in hundredths of a bip (500 / 3000 / 10000) |
| `tickLower`, `tickUpper` | Range bounds, **must be multiples of the pool's `tickSpacing`** (enforced by core) |
| `amount0Desired`, `amount1Desired` | Upper bounds on what you will spend |
| `amount0Min`, `amount1Min` | Slippage floors on what actually gets deposited |
| `recipient` | Who receives the NFT (not necessarily the payer) |
| `deadline` | Expiry |

**Flow:**

```
mint
 ├─ checkDeadline(params.deadline)                          PeripheryValidation.sol:7
 ├─ addLiquidity({... recipient: address(this) ...})        LiquidityManagement.sol:51
 │    ├─ PoolAddress.computeAddress                         PoolAddress.sol:33
 │    ├─ pool.slot0()                              → sqrtPriceX96
 │    ├─ TickMath.getSqrtRatioAtTick × 2
 │    ├─ LiquidityAmounts.getLiquidityForAmounts   → liquidity
 │    ├─ pool.mint(address(this), tl, tu, liquidity, data)   ◄── CORE
 │    │    └─ ↩ uniswapV3MintCallback              LiquidityManagement.sol:25
 │    │         ├─ CallbackValidation.verifyCallback
 │    │         └─ pay(token, msg.sender, pool, owed) × 2
 │    └─ require(amount0 >= min && amount1 >= min, 'Price slippage check')
 ├─ _mint(params.recipient, tokenId = _nextId++)             OZ ERC721      :156
 ├─ pool.positions(PositionKey.compute(address(this),tl,tu)) → feeGrowthInside*  :159
 ├─ cachePoolKey(pool, poolKey)                    → poolId  :162
 ├─ _positions[tokenId] = Position{...}                      :168
 └─ emit IncreaseLiquidity(tokenId, liquidity, amount0, amount1)   :181
```

- **Note the recipient split:** `addLiquidity` is called with
  `recipient: address(this)` (`:146`) — the *pool* position is owned by the
  manager. `params.recipient` only receives the ERC-721.
- **Note the ordering:** `_mint` (`:156`) happens *before* `_positions[tokenId]`
  is written (`:168`). `_mint` on a contract recipient triggers
  `onERC721Received`, so the recipient sees a token whose `positions()` call would
  revert with `'Invalid token ID'`.
- **`feeGrowthInside*` is read *after* the pool mint** (`:159`) so the snapshot
  already includes this position's contribution. Starting `tokensOwed` at 0 is
  therefore correct.
- **Returns:** `(tokenId, liquidity, amount0, amount1)`.
- **Event:** `IncreaseLiquidity(tokenId, liquidity, amount0, amount1)`.
- **Access control:** none — anyone may mint to anyone.
- **Gotcha:** minting into an *uninitialized* pool reverts inside core. Batch
  `createAndInitializePoolIfNecessary` first via `multicall`.
- **Gotcha:** no refund of unused ETH. If `msg.value` exceeded what was spent,
  append `refundETH` to the multicall.

### 3.7 `isAuthorizedForToken`

`modifier isAuthorizedForToken(uint256 tokenId)` — `:184`

`require(_isApprovedOrOwner(msg.sender, tokenId), 'Not approved')`. OZ's
`_isApprovedOrOwner` accepts owner, the per-token operator, or an
`isApprovedForAll` operator.

- **Applied to:** `decreaseLiquidity` (`:261`), `collect` (`:313`), `burn` (`:377`).
- **Not applied to `increaseLiquidity`** — anyone may donate liquidity to your
  position. It is a gift, not a risk: the extra liquidity is owned by the NFT.

### 3.8 `tokenURI` and `baseURI`

#### `tokenURI(uint256 tokenId) public view override(ERC721, IERC721Metadata) returns (string memory)` — `:189`

`require(_exists(tokenId))` (bare), then delegates to
`INonfungibleTokenPositionDescriptor(_tokenDescriptor).tokenURI(this, tokenId)`.

#### `baseURI() public pure override returns (string memory)` — `:195`

Empty body — returns `""`. The comment says it plainly: "save bytecode by removing
implementation of unused method".

### 3.9 `increaseLiquidity`

`increaseLiquidity(IncreaseLiquidityParams calldata params) external payable checkDeadline(params.deadline) returns (uint128 liquidity, uint256 amount0, uint256 amount1)` — `:198`

```solidity
struct IncreaseLiquidityParams {  // interfaces/INonfungiblePositionManager.sol:110
    uint256 tokenId;
    uint256 amount0Desired; uint256 amount1Desired;
    uint256 amount0Min; uint256 amount1Min;
    uint256 deadline;
}
```

1. Load `position` (storage pointer) and `poolKey` from `poolId` (`:209-211`).
2. `addLiquidity(...)` with the position's existing ticks — same callback dance.
3. Re-read `pool.positions(positionKey)` (`:232`) — now updated by the mint.
4. **Reconcile fees before changing liquidity** (`:234-247`):
   ```solidity
   position.tokensOwed0 += uint128(FullMath.mulDiv(
       feeGrowthInside0LastX128 - position.feeGrowthInside0LastX128,
       position.liquidity,          // ← the OLD liquidity
       FixedPoint128.Q128));
   ```
   Using the old liquidity is essential: those fees were earned by the old size.
5. Update the snapshots and `position.liquidity += liquidity` (`:249-251`).
6. `emit IncreaseLiquidity(params.tokenId, liquidity, amount0, amount1)`.

- **Access control:** none. See [§3.7](#37-isauthorizedfortoken).
- **Gotcha:** the `uint128()` casts at `:234` and `:241` are **unchecked**. If a
  position accrued more than `2^128 − 1` of fee units the value silently wraps.
- **Gotcha:** reverts with a panic if `tokenId` does not exist — `poolKey` would be
  the zero key and `computeAddress` requires `token0 < token1`.

### 3.10 `decreaseLiquidity`

`decreaseLiquidity(DecreaseLiquidityParams calldata params) external payable isAuthorizedForToken(params.tokenId) checkDeadline(params.deadline) returns (uint256 amount0, uint256 amount1)` — `:257`

```solidity
struct DecreaseLiquidityParams {  // interfaces/INonfungiblePositionManager.sol:138
    uint256 tokenId; uint128 liquidity;
    uint256 amount0Min; uint256 amount1Min; uint256 deadline;
}
```

1. `require(params.liquidity > 0)` (bare, `:265`).
2. `require(positionLiquidity >= params.liquidity)` (bare, `:269`).
3. `pool.burn(tickLower, tickUpper, params.liquidity)` (`:273`) — **core's `burn`
   moves tokens into the position's `tokensOwed` inside the pool; it transfers
   nothing.**
4. `require(amount0 >= params.amount0Min && amount1 >= params.amount1Min, 'Price slippage check')` (`:275`).
5. Reconcile: `tokensOwed += uint128(amountN) + feeDelta·positionLiquidity/Q128`
   (`:281-298`) — note it adds **both** the withdrawn principal and the accrued
   fees, and uses the pre-burn `positionLiquidity` for the fee term.
6. `position.liquidity = positionLiquidity - params.liquidity` (`:303`).
7. `emit DecreaseLiquidity(params.tokenId, params.liquidity, amount0, amount1)`.

- **The headline gotcha:** *this function transfers nothing to you.* It converts
  liquidity into a credit. You must call `collect` afterwards. Front-ends always
  batch `decreaseLiquidity` + `collect` (+ `burn`) in one `multicall`.

### 3.11 `collect`

`collect(CollectParams calldata params) external payable isAuthorizedForToken(params.tokenId) returns (uint256 amount0, uint256 amount1)` — `:309`

```solidity
struct CollectParams {  // interfaces/INonfungiblePositionManager.sol:159
    uint256 tokenId; address recipient;
    uint128 amount0Max; uint128 amount1Max;
}
```

Note: **no `checkDeadline`**.

1. `require(params.amount0Max > 0 || params.amount1Max > 0)` (bare, `:316`).
2. `recipient = params.recipient == address(0) ? address(this) : params.recipient` (`:318`)
   — collecting to the manager itself, so a later `sweepToken`/`unwrapWETH9` in the
   same multicall can unwrap WETH into ETH.
3. **The zero-burn trick** (`:329-351`): if `position.liquidity > 0`, call
   `pool.burn(tickLower, tickUpper, 0)`.

   ```solidity
   if (position.liquidity > 0) {
       pool.burn(position.tickLower, position.tickUpper, 0);
       (, uint256 fg0, uint256 fg1, , ) = pool.positions(PositionKey.compute(...));
       tokensOwed0 += uint128(FullMath.mulDiv(fg0 - position.feeGrowthInside0LastX128,
                                              position.liquidity, FixedPoint128.Q128));
       ...
   }
   ```

   **Why:** core only refreshes a position's `feeGrowthInside` snapshot inside
   `_modifyPosition`, which `collect` does not call. Burning zero liquidity is a
   no-op that still runs `_updatePosition`, pushing all accrued fees into the
   pool's own `tokensOwed`. Without it you would collect only fees accrued up to
   the last mint/burn. This is the single most-asked-about line in the periphery.
4. Clamp: `amountNCollect = min(params.amountNMax, tokensOwedN)` (`:354-358`).
5. `pool.collect(recipient, tickLower, tickUpper, amount0Collect, amount1Collect)` (`:361`)
   — this is where tokens actually move.
6. **Subtract the requested amount, not the received amount** (`:371`):
   ```solidity
   (position.tokensOwed0, position.tokensOwed1) =
       (tokensOwed0 - amount0Collect, tokensOwed1 - amount1Collect);
   ```
   The comment at `:369-370` explains: core may return a few wei less due to
   rounding; subtracting the full expected amount lets `tokensOwed` reach exactly
   zero so `burn` can succeed. The cost is those few wei are stranded in the pool
   forever.
7. `emit Collect(params.tokenId, recipient, amount0Collect, amount1Collect)` —
   **the event reports the clamped request, not `(amount0, amount1)` actually
   received.** Indexers that trust this event over-count by the rounding dust.

- **Returns:** `(amount0, amount1)` — the *actual* amounts from core.

### 3.12 `burn`

`burn(uint256 tokenId) external payable isAuthorizedForToken(tokenId)` — `:377`

```solidity
Position storage position = _positions[tokenId];
require(position.liquidity == 0 && position.tokensOwed0 == 0 && position.tokensOwed1 == 0, 'Not cleared');
delete _positions[tokenId];
_burn(tokenId);
```

- **Revert:** `'Not cleared'`.
- The full close sequence is therefore three calls, normally batched:
  `decreaseLiquidity(all)` → `collect(max, max)` → `burn`.
- `delete` refunds gas for the five slots.

### 3.13 `_getAndIncrementNonce`, `getApproved`, `_approve`

#### `_getAndIncrementNonce(uint256 tokenId) internal override returns (uint256)` — `:384`

`return uint256(_positions[tokenId].nonce++);` — post-increment, so the *old*
value is used in the digest. Packing the nonce into the position struct saves a
whole mapping.

#### `getApproved(uint256 tokenId) public view override(ERC721, IERC721) returns (address)` — `:389`

`require(_exists(tokenId), 'ERC721: approved query for nonexistent token')`, then
`_positions[tokenId].operator`.

#### `_approve(address to, uint256 tokenId) internal override(ERC721)` — `:396`

```solidity
_positions[tokenId].operator = to;
emit Approval(ownerOf(tokenId), to, tokenId);
```

Both overrides exist for one reason: to keep the approval in the position struct's
already-paid-for slot (packed beside `nonce`) rather than in OZ's separate
`_tokenApprovals` mapping. A cold SSTORE saved on every approve.

- **Gotcha:** OZ's internal `_tokenApprovals` still exists but is dead. Anything
  reading it directly (assembly, storage proofs, some indexers) sees zeros.

### 3.14 The fee-reconciliation invariant

The one thing to internalise about this contract.

Core stores **one** position per `(manager, tickLower, tickUpper)`
([§2.3](#23-positionkey)). All NFTs on the same range share it. Core tracks
`feeGrowthInside0X128` — cumulative fees per unit of liquidity earned while the
price was inside that range, an ever-increasing number that overflows on purpose.

Each NFT stores its own snapshot. Fees owed to one NFT since its last touch:

```
owed = (feeGrowthInside_now − feeGrowthInside_snapshot) × liquidity / 2^128
```

Every function that changes `liquidity` must settle first, at the **old**
liquidity, then update the snapshot:

| Function | Line | Liquidity used for the fee term | Then |
|---|---|---|---|
| `mint` | `:159` | — (new position) | snapshot = current, owed = 0 |
| `increaseLiquidity` | `:234-247` | `position.liquidity` (old) | snapshot updated, `liquidity +=` |
| `decreaseLiquidity` | `:281-298` | `positionLiquidity` (pre-burn) | snapshot updated, `liquidity −=` |
| `collect` | `:334-347` | `position.liquidity` (unchanged) | snapshot updated, `liquidity` untouched |

Miss any one of these and fees are misallocated between NFTs sharing the range —
which is a theft, not a rounding error. This is the pattern every "V3 position
wrapper" gets wrong; the same shape appears in Curve gauges and Aave rewards
(see the cross-cutting concepts in [`../README.md`](../README.md)).

---

## 4. `SwapRouter`

`SwapRouter.sol` — 244 lines.

```solidity
contract SwapRouter is
    ISwapRouter,                 // :23  → IUniswapV3SwapCallback
    PeripheryImmutableState,     // :24
    PeripheryValidation,         // :25  → BlockTimestamp
    PeripheryPaymentsWithFee,    // :26  → PeripheryPayments
    Multicall,                   // :27
    SelfPermit                   // :28
```

Stateless between transactions. It should never hold a balance; anything it holds
is claimable by the next caller via `sweepToken` / `unwrapWETH9`.

### 4.1 Storage and `getPool`

| Variable | Type | Line |
|---|---|---|
| `DEFAULT_AMOUNT_IN_CACHED` | `uint256 private constant = type(uint256).max` | `:35` |
| `amountInCached` | `uint256 private = DEFAULT_AMOUNT_IN_CACHED` | `:38` |

`amountInCached` is a pre-EIP-1153 transient variable: written inside a nested
callback, read by the outer frame, reset before returning. Initialised to
`type(uint256).max` (not 0) so the slot is never cleared — writing non-zero over
non-zero is far cheaper than the zero↔non-zero transitions.

```solidity
struct SwapCallbackData { bytes path; address payer; }   // :51
```

#### `getPool(address tokenA, address tokenB, uint24 fee) private view returns (IUniswapV3Pool)` — `:43`

`IUniswapV3Pool(PoolAddress.computeAddress(factory, PoolAddress.getPoolKey(tokenA, tokenB, fee)))`.

### 4.2 `uniswapV3SwapCallback`

`uniswapV3SwapCallback(int256 amount0Delta, int256 amount1Delta, bytes calldata _data) external override` — `:57`

Called by the pool mid-swap. Deltas are from the **pool's** perspective: positive
means the pool must receive that token.

```solidity
require(amount0Delta > 0 || amount1Delta > 0);   // no zero-liquidity swaps
SwapCallbackData memory data = abi.decode(_data, (SwapCallbackData));
(address tokenIn, address tokenOut, uint24 fee) = data.path.decodeFirstPool();
CallbackValidation.verifyCallback(factory, tokenIn, tokenOut, fee);

(bool isExactInput, uint256 amountToPay) =
    amount0Delta > 0
        ? (tokenIn < tokenOut, uint256(amount0Delta))
        : (tokenOut < tokenIn, uint256(amount1Delta));

if (isExactInput) {
    pay(tokenIn, data.payer, msg.sender, amountToPay);
} else {
    if (data.path.hasMultiplePools()) {
        data.path = data.path.skipToken();
        exactOutputInternal(amountToPay, msg.sender, 0, data);   // ← recursion
    } else {
        amountInCached = amountToPay;
        tokenIn = tokenOut;            // reversed for exact-output
        pay(tokenIn, data.payer, msg.sender, amountToPay);
    }
}
```

- **`isExactInput` derivation:** whichever delta is positive tells you which token
  the pool wants; comparing it against the path's `tokenIn` ordering tells you
  whether the swap was specified by its input or its output.
- **The exact-output recursion (`:77`)** is the heart of multi-hop exact-output:
  the callback for hop N triggers the swap for hop N−1, paying the current pool
  from the previous pool's output. `recipient` is `msg.sender` — the pool that is
  currently calling us.
- **The `tokenIn = tokenOut` swap (`:80`)** looks like a bug and is not: for
  exact-output the path is reversed, so `decodeFirstPool` returned them the wrong
  way round for payment purposes.
- **Access control:** none, guarded entirely by `verifyCallback` (`:65`).

### 4.3 `exactInputInternal`

`exactInputInternal(uint256 amountIn, address recipient, uint160 sqrtPriceLimitX96, SwapCallbackData memory data) private returns (uint256 amountOut)` — `:87`

```solidity
if (recipient == address(0)) recipient = address(this);
(address tokenIn, address tokenOut, uint24 fee) = data.path.decodeFirstPool();
bool zeroForOne = tokenIn < tokenOut;
(int256 amount0, int256 amount1) = getPool(tokenIn, tokenOut, fee).swap(
    recipient, zeroForOne, amountIn.toInt256(),
    sqrtPriceLimitX96 == 0
        ? (zeroForOne ? TickMath.MIN_SQRT_RATIO + 1 : TickMath.MAX_SQRT_RATIO - 1)
        : sqrtPriceLimitX96,
    abi.encode(data));
return uint256(-(zeroForOne ? amount1 : amount0));
```

- **Positive `amountSpecified` = exact input** in core's convention.
- **`sqrtPriceLimitX96 == 0`** is translated to the extreme tick ±1, i.e. "no
  limit". Core requires strict inequality against `MIN`/`MAX_SQRT_RATIO`, hence
  the ±1.
- **The output is the negated delta**: core returns negative for tokens leaving
  the pool.

### 4.4 `exactInputSingle`

`exactInputSingle(ExactInputSingleParams calldata params) external payable checkDeadline(params.deadline) returns (uint256 amountOut)` — `:115`

```solidity
struct ExactInputSingleParams {  // interfaces/ISwapRouter.sol:10
    address tokenIn; address tokenOut; uint24 fee;
    address recipient; uint256 deadline;
    uint256 amountIn; uint256 amountOutMinimum; uint160 sqrtPriceLimitX96;
}
```

One call to `exactInputInternal` with `path = abi.encodePacked(tokenIn, fee, tokenOut)`
and `payer = msg.sender`, then
`require(amountOut >= params.amountOutMinimum, 'Too little received')` (`:128`).

- **Gotcha:** if you set a real `sqrtPriceLimitX96`, the swap can *partially fill*
  and still succeed as long as the output clears `amountOutMinimum`.

### 4.5 `exactInput`

`exactInput(ExactInputParams memory params) external payable checkDeadline(params.deadline) returns (uint256 amountOut)` — `:132`

```solidity
struct ExactInputParams {  // interfaces/ISwapRouter.sol:26
    bytes path; address recipient; uint256 deadline;
    uint256 amountIn; uint256 amountOutMinimum;
}
```

Note `memory`, not `calldata` — the function mutates `params.path` as it walks.

```solidity
address payer = msg.sender;                     // msg.sender pays hop 1
while (true) {
    bool hasMultiplePools = params.path.hasMultiplePools();
    params.amountIn = exactInputInternal(
        params.amountIn,
        hasMultiplePools ? address(this) : params.recipient,  // custody mid-route
        0,
        SwapCallbackData({ path: params.path.getFirstPool(), payer: payer }));
    if (hasMultiplePools) {
        payer = address(this);                  // router pays hop 2+
        params.path = params.path.skipToken();
    } else { amountOut = params.amountIn; break; }
}
require(amountOut >= params.amountOutMinimum, 'Too little received');
```

Forward iteration, no recursion. Intermediate outputs land on the router, and
`pay`'s `payer == address(this)` branch (`PeripheryPayments.sol:62`) forwards them
without an allowance.

- `sqrtPriceLimitX96` is hard-coded to 0 per hop; only the final
  `amountOutMinimum` protects you, so intermediate hops can be sandwiched
  arbitrarily as long as the end result clears the floor.

**Trace, 3-hop A→B→C→D:**

```
exactInput(path = A·f1·B·f2·C·f3·D, amountIn = 100)
 │ payer = user
 ├─ hop1: exactInputInternal(100, recipient=router, path=A·f1·B, payer=user)
 │        pool(A,B).swap → ↩ callback → pay(A, user, pool(A,B), 100)
 │        → 98 B to router
 │ payer = router;  path = B·f2·C·f3·D
 ├─ hop2: exactInputInternal(98, recipient=router, path=B·f2·C, payer=router)
 │        pool(B,C).swap → ↩ callback → pay(B, router, pool(B,C), 98)   [safeTransfer]
 │        → 96 C to router
 │ path = C·f3·D
 ├─ hop3: exactInputInternal(96, recipient=USER, path=C·f3·D, payer=router)
 │        pool(C,D).swap → ↩ callback → pay(C, router, pool(C,D), 96)
 │        → 95 D straight to the user
 └─ require(95 >= amountOutMinimum, 'Too little received')
```

### 4.6 `exactOutputInternal`

`exactOutputInternal(uint256 amountOut, address recipient, uint160 sqrtPriceLimitX96, SwapCallbackData memory data) private returns (uint256 amountIn)` — `:169`

```solidity
(address tokenOut, address tokenIn, uint24 fee) = data.path.decodeFirstPool();  // REVERSED
bool zeroForOne = tokenIn < tokenOut;
(int256 amount0Delta, int256 amount1Delta) = getPool(tokenIn, tokenOut, fee).swap(
    recipient, zeroForOne, -amountOut.toInt256(),   // NEGATIVE = exact output
    sqrtPriceLimitX96 == 0 ? (...) : sqrtPriceLimitX96, abi.encode(data));
(amountIn, amountOutReceived) = zeroForOne
    ? (uint256(amount0Delta), uint256(-amount1Delta))
    : (uint256(amount1Delta), uint256(-amount0Delta));
if (sqrtPriceLimitX96 == 0) require(amountOutReceived == amountOut);
```

- **Negative `amountSpecified` = exact output.**
- The destructuring at `:178` is `(tokenOut, tokenIn, fee)` because exact-output
  paths are written backwards.
- `:199` — with no price limit, a partial fill is impossible, so require exactness
  (bare revert). With a limit, partial fills are tolerated.

### 4.7 `exactOutputSingle`

`exactOutputSingle(ExactOutputSingleParams calldata params) external payable checkDeadline(params.deadline) returns (uint256 amountIn)` — `:203`

```solidity
struct ExactOutputSingleParams {  // interfaces/ISwapRouter.sol:39
    address tokenIn; address tokenOut; uint24 fee;
    address recipient; uint256 deadline;
    uint256 amountOut; uint256 amountInMaximum; uint160 sqrtPriceLimitX96;
}
```

Path is `abi.encodePacked(tokenOut, fee, tokenIn)` — reversed (`:215`). Then
`require(amountIn <= params.amountInMaximum, 'Too much requested')` (`:218`) and
`amountInCached = DEFAULT_AMOUNT_IN_CACHED` (`:220`).

- The comment at `:219` is candid: the reset is unnecessary for the single-hop
  path (the return value is used directly) but is done anyway for uniformity.

### 4.8 `exactOutput`

`exactOutput(ExactOutputParams calldata params) external payable checkDeadline(params.deadline) returns (uint256 amountIn)` — `:224`

```solidity
struct ExactOutputParams {  // interfaces/ISwapRouter.sol:55
    bytes path; address recipient; uint256 deadline;
    uint256 amountOut; uint256 amountInMaximum;
}
```

```solidity
exactOutputInternal(params.amountOut, params.recipient, 0,
                    SwapCallbackData({path: params.path, payer: msg.sender}));
amountIn = amountInCached;
require(amountIn <= params.amountInMaximum, 'Too much requested');
amountInCached = DEFAULT_AMOUNT_IN_CACHED;
```

**No loop.** One call kicks off a recursion that unwinds through the callbacks.

### 4.9 Why exact-output runs backwards

You know the output but not the input, so you cannot start at the first pool — you
do not know how much to put in. So you start at the **last** pool, ask it for the
exact output, and let *its* callback demand payment, which is the exact output of
the second-to-last pool, and so on.

**Trace, exact-output A→B→C, want 100 C** (path encoded `C·f2·B·f1·A`):

```
exactOutput(path = C·f2·B·f1·A, amountOut = 100)
 └─ exactOutputInternal(100, recipient=user, path=C·f2·B·f1·A, payer=user)
     └─ pool(B,C).swap(user, ..., -100, data)          → sends 100 C to user
         └─ ↩ uniswapV3SwapCallback(amount1Delta = +102 B)
             │ isExactInput = false; hasMultiplePools = true
             │ path = path.skipToken() → B·f1·A
             └─ exactOutputInternal(102, recipient=pool(B,C), path=B·f1·A, payer=user)
                 └─ pool(A,B).swap(pool(B,C), ..., -102, data)  → sends 102 B to pool(B,C)
                     └─ ↩ uniswapV3SwapCallback(amount0Delta = +105 A)
                         │ hasMultiplePools = false  → base case
                         ├─ amountInCached = 105      ◄── the answer escapes here
                         ├─ tokenIn = tokenOut        (reversal fix)
                         └─ pay(A, user, pool(A,B), 105)
     ── unwind ──
 amountIn = amountInCached = 105
 require(105 <= amountInMaximum, 'Too much requested')
 amountInCached = type(uint256).max
```

The recursion depth equals the number of hops, so a pathological path can hit the
1024-frame limit; gas runs out long before.

`amountInCached` exists because the innermost frame computes the answer but its
return value is discarded by the pool that called it. A storage slot is the only
channel back to the outer frame. In V4 this is a transient-storage variable; in
0.7.6 it is a regular slot.

---

## 5. `lens/` — read-only helpers

### 5.1 `Quoter`

`lens/Quoter.sol` — 170 lines, `is IQuoter, IUniswapV3SwapCallback, PeripheryImmutableState`.

The trick: **actually perform the swap, then revert**, encoding the answer in the
revert data. That way the quote uses the identical code path as a real swap —
tick crossings, fee tiers, rounding — with no duplicated math to drift out of
sync.

| Variable | Type | Line |
|---|---|---|
| `amountOutCached` | `uint256 private` | `:25` |

#### `getPool(address,address,uint24) private view` — `:29`

Same as the router's.

#### `uniswapV3SwapCallback(int256 amount0Delta, int256 amount1Delta, bytes memory path) external view override` — `:38`

Note **`view`** — it can never pay, only revert.

```solidity
require(amount0Delta > 0 || amount1Delta > 0);
(address tokenIn, address tokenOut, uint24 fee) = path.decodeFirstPool();
CallbackValidation.verifyCallback(factory, tokenIn, tokenOut, fee);
...
if (isExactInput) {
    assembly { let ptr := mload(0x40); mstore(ptr, amountReceived); revert(ptr, 32) }
} else {
    if (amountOutCached != 0) require(amountReceived == amountOutCached);
    assembly { let ptr := mload(0x40); mstore(ptr, amountToPay); revert(ptr, 32) }
}
```

The revert payload is a bare 32-byte word — **not** an ABI-encoded `Error(string)`.
That is what makes it distinguishable from a genuine revert.

#### `parseRevertReason(bytes memory reason) private pure returns (uint256)` — `:69`

```solidity
if (reason.length != 32) {
    if (reason.length < 68) revert('Unexpected error');
    assembly { reason := add(reason, 0x04) }
    revert(abi.decode(reason, (string)));
}
return abi.decode(reason, (uint256));
```

Exactly 32 bytes → our quote. Otherwise it is a real error: ≥68 bytes gets
re-thrown as its decoded string, anything else becomes `'Unexpected error'`.

- **Gotcha:** a genuine revert whose payload happens to be exactly 32 bytes would
  be misread as a quote. Custom errors (4 bytes) and panics (36 bytes) both fall
  into `'Unexpected error'`.

#### `quoteExactInputSingle(address tokenIn, address tokenOut, uint24 fee, uint256 amountIn, uint160 sqrtPriceLimitX96) public returns (uint256 amountOut)` — `:81`

`try pool.swap(...) {} catch (bytes memory reason) { return parseRevertReason(reason); }`.
The success branch is empty because success is impossible.

- **Not `view`** — `try/catch` needs a real call, and `amountOutCached` is written
  in the exact-output variants.

#### `quoteExactInput(bytes memory path, uint256 amountIn) external returns (uint256 amountOut)` — `:106`

Loops forward over hops, feeding each output into the next input.

#### `quoteExactOutputSingle(address tokenIn, address tokenOut, uint24 fee, uint256 amountOut, uint160 sqrtPriceLimitX96) public returns (uint256 amountIn)` — `:125`

Sets `amountOutCached = amountOut` when there is no price limit (`:135`) so the
callback can assert the full output was achievable, and `delete`s it in the catch
(`:147`).

#### `quoteExactOutput(bytes memory path, uint256 amountOut) external returns (uint256 amountIn)` — `:153`

Loops backwards over the reversed path.

- **Gotcha (whole contract):** the header comment (`:18-19`) says it: *"These
  functions are not gas efficient and should not be called on chain."* Each hop is
  a full swap plus a revert. Calling a quoter on-chain also gives you a **spot**
  price with no manipulation resistance — it is not an oracle.

### 5.2 `QuoterV2`

`lens/QuoterV2.sol` — 273 lines. Same trick, richer output:
`(amount, sqrtPriceX96After, initializedTicksCrossed, gasEstimate)`.

Why each extra field matters: `sqrtPriceX96After` shows the price impact so a
router can compare split routes; `initializedTicksCrossed` predicts the real gas
of the swap (each crossing is an SSTORE); `gasEstimate` is measured, not modelled.

#### `uniswapV3SwapCallback(int256, int256, bytes memory) external view override` — `:41`

Additionally reads `pool.slot0()` (`:56`) and reverts with **96 bytes**:
`amount`, `sqrtPriceX96After`, `tickAfter` (`:59-75`).

#### `parseRevertReason(bytes memory reason) private pure returns (uint256 amount, uint160 sqrtPriceX96After, int24 tickAfter)` — `:80`

`reason.length != 96` → same real-error handling as V1.

#### `handleRevert(bytes memory reason, IUniswapV3Pool pool, uint256 gasEstimate) private view returns (uint256, uint160, uint32, uint256)` — `:99`

Reads `tickBefore` from `slot0` **after** the reverted swap (state was rolled
back, so this is the pre-swap tick), parses the reason for `tickAfter`, and calls
`pool.countInitializedTicksCrossed(tickBefore, tickAfter)`
([§2.12](#212-poolticjscounter)).

#### `quoteExactInputSingle(QuoteExactInputSingleParams memory params) public returns (uint256 amountOut, uint160 sqrtPriceX96After, uint32 initializedTicksCrossed, uint256 gasEstimate)` — `:123`

```solidity
struct QuoteExactInputSingleParams {   // interfaces/IQuoterV2.sol:27
    address tokenIn; address tokenOut; uint256 amountIn; uint24 fee; uint160 sqrtPriceLimitX96;
}
```

Brackets the call with `gasleft()` (`:136`, `:148`) so `gasEstimate` is measured.

- **Note the field order differs from V1's positional arguments:** `amountIn`
  comes *before* `fee` here. Mechanical translations between the two quoters
  silently swap them.

#### `quoteExactInput(bytes memory path, uint256 amountIn) public returns (uint256 amountOut, uint160[] memory sqrtPriceX96AfterList, uint32[] memory initializedTicksCrossedList, uint256 gasEstimate)` — `:153`

Allocates both arrays with `path.numPools()` (`:163-164`) and fills them per hop,
accumulating `gasEstimate`.

#### `quoteExactOutputSingle(QuoteExactOutputSingleParams memory params) public returns (...)` — `:197`

```solidity
struct QuoteExactOutputSingleParams {  // interfaces/IQuoterV2.sol:71
    address tokenIn; address tokenOut; uint256 amount; uint24 fee; uint160 sqrtPriceLimitX96;
}
```

Note the field is `amount`, not `amountOut`.

#### `quoteExactOutput(bytes memory path, uint256 amountOut) public returns (...)` — `:230`

Backwards walk over the reversed path.

- **Gotcha:** `gasEstimate` measures the *reverted* swap, which skips the final
  SSTOREs the real swap performs. Treat it as a lower bound.

### 5.3 `TickLens`

`lens/TickLens.sol` — 42 lines, `is ITickLens`.

#### `getPopulatedTicksInWord(address pool, int16 tickBitmapIndex) public view returns (PopulatedTick[] memory populatedTicks)` — `:12`

```solidity
struct PopulatedTick { int24 tick; int128 liquidityNet; uint128 liquidityGross; }  // interfaces/ITickLens.sol:10
```

Two passes over the 256-bit word: count set bits to size the array, then fill it.
Tick reconstruction (`:32`):
`populatedTick = ((int24(tickBitmapIndex) << 8) + int24(i)) * tickSpacing`.

Fills **backwards** (`populatedTicks[--numberOfPopulatedTicks]`, `:34`), so results
come out in descending tick order.

- **Use:** front-ends draw the liquidity-depth chart by scanning a range of words.
- **Gotcha:** one word covers 256 *spacings*. For `tickSpacing = 60` that is 15,360
  ticks ≈ a 4.6× price range — but for `tickSpacing = 1` it is only 256 ticks
  ≈ 2.6%. Scanning a wide range on a 1-spacing pool takes many calls.
- **Test double:** `test/TickLensTest.sol`.

### 5.4 `UniswapInterfaceMulticall`

`lens/UniswapInterfaceMulticall.sol` — 39 lines. MIT. A standalone Multicall2 fork;
nothing else in the repo uses it. Note this is a *different* contract from
`base/Multicall` — it batches calls to **other** contracts, not to itself.

```solidity
struct Call   { address target; uint256 gasLimit; bytes callData; }   // :7
struct Result { bool success; uint256 gasUsed; bytes returnData; }    // :13
```

| Function | Line | Notes |
|---|---|---|
| `getCurrentBlockTimestamp() public view returns (uint256)` | `:19` | |
| `getEthBalance(address addr) public view returns (uint256)` | `:23` | |
| `multicall(Call[] memory calls) public returns (uint256 blockNumber, Result[] memory returnData)` | `:27` | per-call `gasLimit`, records `gasUsed`, **never reverts on sub-call failure** |

The per-call gas limit and the success flag are what the interface needs: one
failing token among 200 must not kill the batch.

- **Gotcha:** not `view`, so it must be `eth_call`ed rather than read as a getter.
- **Gotcha:** no 63/64 check. If `gasLimit` exceeds what is available the sub-call
  gets only 63/64 of the remainder and may fail spuriously.

---

## 6. `V3Migrator`

`V3Migrator.sol` — 99 lines, `is IV3Migrator, PeripheryImmutableState, PoolInitializer, Multicall, SelfPermit`.

| Variable | Type | Line |
|---|---|---|
| `nonfungiblePositionManager` | `address public immutable` | `:23` |

#### `constructor(address _factory, address _WETH9, address _nonfungiblePositionManager)` — `:25`

#### `receive() external payable` — `:33`

`require(msg.sender == WETH9, 'Not WETH9')` — same guard as `PeripheryPayments`,
duplicated because this contract does not inherit it.

#### `migrate(MigrateParams calldata params) external override` — `:37`

```solidity
struct MigrateParams {   // interfaces/IV3Migrator.sol:12
    address pair;                // the V2 pair
    uint256 liquidityToMigrate;  // expected to be balanceOf(msg.sender)
    uint8   percentageToMigrate; // numerator over 100
    address token0; address token1; uint24 fee;
    int24   tickLower; int24 tickUpper;
    uint256 amount0Min; uint256 amount1Min;   // must be discounted by percentageToMigrate
    address recipient; uint256 deadline; bool refundAsETH;
}
```

Burn a V2 LP position, mint a V3 one, refund the remainder — atomically.

1. `require(params.percentageToMigrate > 0, 'Percentage too small')` (`:38`),
   `require(params.percentageToMigrate <= 100, 'Percentage too large')` (`:39`).
2. `IUniswapV2Pair(pair).transferFrom(msg.sender, pair, liquidityToMigrate)` (`:42`)
   — LP tokens go **to the pair itself**, the V2 burn convention.
3. `(amount0V2, amount1V2) = IUniswapV2Pair(pair).burn(address(this))` (`:43`).
4. `amountNV2ToMigrate = amountNV2 * percentageToMigrate / 100` (`:46-47`).
5. `TransferHelper.safeApprove(tokenN, nonfungiblePositionManager, amountNV2ToMigrate)` (`:50-51`).
6. `INonfungiblePositionManager.mint(MintParams{...})` (`:55`) — returns
   `(_, _, amount0V3, amount1V3)`.
7. Refund loop per token (`:72-97`):
   - `if (amountNV3 < amountNV2ToMigrate)` → `safeApprove(tokenN, npm, 0)` to clear
     the dangling allowance.
   - `refundN = amountNV2 − amountNV3`, sent as ETH if
     `params.refundAsETH && tokenN == WETH9` (unwrap then `safeTransferETH`),
     else `safeTransfer`.

**Flow:**

```
user ── approve LP ──► V3Migrator.migrate
                        │
                        ├─ V2Pair.transferFrom(user → pair, liquidity)
                        ├─ V2Pair.burn(migrator)          → amount0V2, amount1V2
                        ├─ safeApprove(token0/1, NPM, toMigrate)
                        ├─ NPM.mint(...)                  → V3 NFT to recipient
                        │    └─ pulls exactly amount0V3/amount1V3 from migrator
                        └─ refund (amountV2 − amountV3) to msg.sender
                             (as ETH if refundAsETH && token == WETH9)
```

- **Gotcha:** the `percentageToMigrate < 100` case leaves the remainder as a
  refund, so the "percentage" controls how much of the *V2 position* becomes a V3
  position — the rest comes back as loose tokens, not V2 LP.
- **Gotcha:** `amount0Min`/`amount1Min` must already be discounted by
  `percentageToMigrate`; the contract does not do it for you. The interface comment
  at `interfaces/IV3Migrator.sol:21-22` says so.
- **Gotcha:** an out-of-range V3 target needs only one token. The other token's
  entire amount is refunded, and `amountNMin` should be 0 — the interface comment
  (`:29-30`) notes this enforces that the position stays out of range.
- **Gotcha:** the refund goes to `msg.sender`, never to `params.recipient`.
- **Gotcha:** `migrate` has **no `checkDeadline`** of its own; the deadline is
  passed through to `NPM.mint`, which does check it.

---

## 7. The on-chain SVG pipeline

Every V3 position NFT renders entirely on-chain: no IPFS, no server. `tokenURI`
returns a `data:application/json;base64,...` URI whose `image` field is itself a
`data:image/svg+xml;base64,...`. Three contracts, ~1000 lines, and the hardest
part is not the graphics — it is printing a Q64.96 fixed-point number as a decimal
string in Solidity 0.7.

```
NonfungiblePositionManager.tokenURI(tokenId)                     :189
 └─ NonfungibleTokenPositionDescriptor.tokenURI(this, tokenId)   :48
     ├─ positionManager.positions(tokenId)     → ticks, tokens, fee
     ├─ PoolAddress.computeAddress             → pool
     ├─ flipRatio(token0, token1, chainId)     → which token is the denominator
     ├─ pool.slot0()                           → current tick
     └─ NFTDescriptor.constructTokenURI(params)                  :44
         ├─ generateName            → "Uniswap - 0.3% - USDC/WETH - 1234<>5678"
         ├─ generateDescriptionPartOne / PartTwo  → prose + addresses + disclaimer
         ├─ generateSVGImage                                      :409
         │   └─ NFTSVG.generateSVG(svgParams)                     :46
         │       ├─ generateSVGDefs           gradients, masks, blur   :76
         │       ├─ generateSVGBorderText     scrolling token addresses :161
         │       ├─ generateSVGCardMantle     "USDC/WETH" + "0.3%"     :194
         │       ├─ generageSvgCurve          the curve, by range width:213
         │       ├─ generateSVGPositionData…  ID / min tick / max tick :306
         │       └─ generateSVGRareSparkle    the sparkle, if rare     :386
         └─ Base64.encode ×2  →  data:application/json;base64,…
```

### 7.1 `NonfungibleTokenPositionDescriptor`

`NonfungibleTokenPositionDescriptor.sol` — 124 lines, `is INonfungibleTokenPositionDescriptor`.

Hard-coded mainnet addresses (`:19-23`): `DAI`, `USDC`, `USDT`, `TBTC`, `WBTC`.

| Variable | Type | Line |
|---|---|---|
| `WETH9` | `address public immutable` | `:25` |
| `nativeCurrencyLabelBytes` | `bytes32 public immutable` | `:27` |

#### `constructor(address _WETH9, bytes32 _nativeCurrencyLabelBytes)` — `:29`

The label is a `bytes32` (null-terminated) rather than a string so it can be
`immutable` — "ETH" on mainnet, "MATIC" on Polygon.

#### `nativeCurrencyLabel() public view returns (string memory)` — `:35`

Scans for the null terminator, copies to a right-sized `bytes`.

#### `tokenURI(INonfungiblePositionManager positionManager, uint256 tokenId) external view returns (string memory)` — `:48`

Assembles `ConstructTokenURIParams`: reads the position, computes the pool,
decides `flipRatio`, reads the current tick, substitutes `nativeCurrencyLabel()`
for WETH's symbol, and reads decimals from both tokens.

- Symbols come from `SafeERC20Namer.tokenSymbol` (`@uniswap/lib`), which tolerates
  `bytes32` symbols and missing implementations.

#### `flipRatio(address token0, address token1, uint256 chainId) public view returns (bool)` — `:95`

`tokenRatioPriority(token0, chainId) > tokenRatioPriority(token1, chainId)`.

#### `tokenRatioPriority(address token, uint256 chainId) public view returns (int256)` — `:103`

| Token | Priority | Meaning |
|---|---|---|
| `WETH9` | `DENOMINATOR` (−100) | prefer as denominator |
| `USDC` (chainId 1) | `NUMERATOR_MOST` (300) | strongest numerator |
| `USDT` (chainId 1) | `NUMERATOR_MORE` (200) | |
| `DAI` (chainId 1) | `NUMERATOR` (100) | |
| `TBTC` (chainId 1) | `DENOMINATOR_MORE` (−200) | |
| `WBTC` (chainId 1) | `DENOMINATOR_MOST` (−300) | strongest denominator |
| anything else | 0 | |

So WBTC/USDC renders as "USDC per WBTC" (a big number) rather than the reciprocal.

- **Gotcha:** the stablecoin ladder is gated on `chainId == 1`. On every other
  chain only the WETH rule applies, so an L2 USDC pair may render inverted.

### 7.2 `NFTDescriptor`

`libraries/NFTDescriptor.sol` — 477 lines. A **public** library, so it is deployed
separately and linked (the position descriptor exceeds the size limit otherwise).

| Constant | Value | Line |
|---|---|---|
| `sqrt10X128` | `1076067327063303206878105757264492625226` | `:25` |

`√10` in Q128 — used to handle odd decimal differences.

```solidity
struct ConstructTokenURIParams {   // :27
    uint256 tokenId;
    address quoteTokenAddress; address baseTokenAddress;
    string  quoteTokenSymbol;   string  baseTokenSymbol;
    uint8   quoteTokenDecimals; uint8   baseTokenDecimals;
    bool    flipRatio;
    int24   tickLower; int24 tickUpper; int24 tickCurrent; int24 tickSpacing;
    uint24  fee; address poolAddress;
}
struct DecimalStringParams {       // :189
    uint256 sigfigs; uint8 bufferLength; uint8 sigfigIndex; uint8 decimalIndex;
    uint8 zerosStartIndex; uint8 zerosEndIndex; bool isLessThanOne; bool isPercent;
}
```

#### `constructTokenURI(ConstructTokenURIParams memory params) public pure returns (string memory)` — `:44`

Builds name, two description halves, and the Base64 SVG, then wraps them in JSON
and Base64s the whole thing. Two nested Base64 encodings — the image inside the
JSON, then the JSON itself.

#### `escapeQuotes(string memory symbol) internal pure returns (string memory)` — `:85`

Counts `"` characters, and if any exist allocates a longer buffer and prefixes
each with `\`. **This is a security function, not a cosmetic one**: a token whose
symbol is `", "image":"evil` would otherwise break out of the JSON string and
inject arbitrary metadata. The disclaimer added at `:150` ("token symbols may be
imitated") covers the remaining social-engineering surface.

#### `generateDescriptionPartOne(string quoteTokenSymbol, string baseTokenSymbol, string poolAddress) private pure returns (string memory)` — `:107`

#### `generateDescriptionPartTwo(string tokenId, string baseTokenSymbol, string quoteTokenAddress, string baseTokenAddress, string feeTier) private pure returns (string memory)` — `:129`

Split in two only to dodge stack-too-deep. Part two ends with the
`⚠️ DISCLAIMER: Due diligence is imperative...` warning (`:150`).

#### `generateName(ConstructTokenURIParams memory params, string memory feeTier) private pure returns (string memory)` — `:155`

`'Uniswap - ' + feeTier + ' - ' + quote + '/' + base + ' - ' + lowPrice + '<>' + highPrice`.
When `flipRatio` is set, `tickLower` and `tickUpper` swap roles (`:170`, `:178`)
because inverting a price reverses the ordering of the bounds.

#### `generateDecimalString(DecimalStringParams memory params) private pure returns (string memory)` — `:208`

The string-building core. Allocates `bufferLength`, optionally writes `%` at the
end and `0.` at the start, fills the zero run, then walks `sigfigs` backwards from
`sigfigIndex`, inserting `.` when it reaches `decimalIndex`.

#### `tickToDecimalString(int24 tick, int24 tickSpacing, uint8 baseTokenDecimals, uint8 quoteTokenDecimals, bool flipRatio) internal pure returns (string memory)` — `:233`

Returns the literal `'MIN'` / `'MAX'` when the tick equals the spacing-aligned
extreme (`:239-243`), swapping the two when `flipRatio`. Otherwise converts to
`sqrtRatioX96`, inverts it as `(1 << 192) / sqrtRatioX96` if flipping (`:248`),
and defers to `fixedPointToDecimalString`.

#### `sigfigsRounded(uint256 value, uint8 digits) private pure returns (uint256, bool)` — `:253`

Rounds to 5 significant figures. Divides down to 6 digits, rounds on the last, and
handles the `99999 → 100000` carry by dividing again and returning
`extraDigit = true`.

#### `adjustForDecimalPrecision(uint160 sqrtRatioX96, uint8 baseTokenDecimals, uint8 quoteTokenDecimals) private pure returns (uint256)` — `:271`

Corrects for differing token decimals. Because the input is a **square root**, a
decimal difference of `d` needs a factor of `10^(d/2)`. Integer division handles
even `d`; for odd `d` it multiplies or divides by `sqrt10X128` (`:277-279`,
`:283-285`). Differences above 18 are ignored entirely (`:288`).

#### `abs(int256 x) private pure returns (uint256)` — `:294`

#### `fixedPointToDecimalString(uint160 sqrtRatioX96, uint8 baseTokenDecimals, uint8 quoteTokenDecimals) internal pure returns (string memory)` — `:300`

The hard one. Squares the adjusted sqrt price to get the price, then scales for
precision: `10^44` when the price is below 1 (`:311`), `10^5` otherwise (`:314`) —
one extra digit in each case for the rounding step. Counts digits, rounds to 5
sigfigs, then picks one of three layouts:

| Case | Line | Layout |
|---|---|---|
| price < 1 | `:334-338` | `0.` + leading zeros + 5 sigfigs; buffer `7 + (43 − digits)` |
| digits ≥ 9 | `:339-344` | integer only, trailing zeros; buffer `digits − 4` |
| otherwise | `:345-349` | 5 sigfigs around a decimal point; buffer 6 |

#### `feeToPercentString(uint24 fee) internal pure returns (string memory)` — `:361`

`0 → '0%'`, `500 → '0.05%'`, `3000 → '0.3%'`, `10000 → '1%'`. Counts total digits
and significant digits (trailing zeros excluded, `:366-375`), then branches on
`digits >= 5` (`:380`) to place the decimal point and zero runs.

#### `addressToString(address addr) internal pure returns (string memory)` — `:405`

`uint256(addr).toHexString(20)`.

#### `generateSVGImage(ConstructTokenURIParams memory params) internal pure returns (string memory svg)` — `:409`

Derives the visual identity from the token addresses:

- `color0..color3` = `tokenToColorHex(quote/base, 136 or 0)` — 3-byte hex slices of
  each address, so every pair has a deterministic palette.
- `x1,y1,x2,y2,x3,y3` = `scale(getCircleCoord(address, offset, tokenId), 0, 255, …)`
  — three blur-circle positions, mapped into the card's coordinate ranges
  (`16..274` horizontally, `100..484` vertically).
- `overRange` = `overRange(tickLower, tickUpper, tickCurrent)`.

#### `overRange(int24 tickLower, int24 tickUpper, int24 tickCurrent) private pure returns (int8)` — `:438`

`−1` below range, `+1` above, `0` in range.

#### `scale(uint256 n, uint256 inMn, uint256 inMx, uint256 outMn, uint256 outMx) private pure returns (string memory)` — `:452`

Linear remap, returned as a decimal string.

#### `tokenToColorHex(uint256 token, uint256 offset) internal pure returns (string memory)` — `:462`

`(token >> offset).toHexStringNoPrefix(3)` — 6 hex chars, a CSS colour.

#### `getCircleCoord(uint256 tokenAddress, uint256 offset, uint256 tokenId) internal pure returns (uint256)` — `:466`

`(sliceTokenHex(tokenAddress, offset) * tokenId) % 255` — token id enters here, so
two positions on the same pair still look different.

#### `sliceTokenHex(uint256 token, uint256 offset) internal pure returns (uint256)` — `:474`

`uint256(uint8(token >> offset))` — one byte.

- **Test double:** `test/NFTDescriptorTest.sol`, `test/Base64Test.sol`.

### 7.3 `NFTSVG`

`libraries/NFTSVG.sol` — 406 lines. Emits the SVG.

Eight hard-coded Bézier paths, `curve1`..`curve8` (`:13-20`), from nearly straight
to sharply bent.

```solidity
struct SVGParams { ... }   // :22 — 22 fields: tokens, symbols, feeTier, ticks,
                           //       overRange, tokenId, 4 colours, 6 coordinates
```

#### `generateSVG(SVGParams memory params) internal pure returns (string memory svg)` — `:46`

Concatenates six generators plus `'</svg>'`. Carries a signed provenance comment
(`:47-52`) from the artist.

#### `generateSVGDefs(SVGParams memory params) private pure returns (string memory svg)` — `:76`

The `<defs>`: three radial-gradient blur circles at the derived coordinates, the
fade masks (`fade-up`, `fade-down`, `none`), the `text-path-a` used by the
scrolling border, and the clip paths.

#### `generateSVGBorderText(string quoteToken, string baseToken, string quoteTokenSymbol, string baseTokenSymbol) private pure returns (string memory svg)` — `:161`

Four `<textPath>` elements at start offsets `−100%`, `0%`, `50%`, `−50%`, each with
a 30-second `<animate>` on `startOffset`. Four copies at staggered offsets produce
a seamless loop — as one scrolls off, another is already entering.

#### `generateSVGCardMantle(string quoteTokenSymbol, string baseTokenSymbol, string feeTier) private pure returns (string memory svg)` — `:194`

The large `QUOTE/BASE` heading and the fee tier beneath, under a `fade-symbol`
mask, plus the rounded border rect.

#### `generageSvgCurve(int24 tickLower, int24 tickUpper, int24 tickSpacing, int8 overRange) private pure returns (string memory svg)` — `:213`

(The typo `generage` is in the deployed source.) Draws the curve twice — a thick
dark stroke then a thin white one, giving an outline — and applies `#fade-up` /
`#fade-down` / `#none` depending on `overRange`.

#### `getCurve(int24 tickLower, int24 tickUpper, int24 tickSpacing) internal pure returns (string memory curve)` — `:244`

`tickRange = (tickUpper − tickLower) / tickSpacing`, then thresholds
`4, 8, 16, 32, 64, 128, 256` select `curve1`..`curve8`. A tight range gets a
straight line; a wide one gets a deep curve. The picture encodes the position.

#### `generateSVGCurveCircle(int8 overRange) internal pure returns (string memory svg)` — `:269`

In range → two small dots at both ends of the curve. Out of range → one dot with a
24px halo at the end the price has passed.

#### `generateSVGPositionDataAndLocationCurve(string tokenId, int24 tickLower, int24 tickUpper) private pure returns (string memory svg)` — `:306`

Three rounded label boxes (`ID`, `Min Tick`, `Max Tick`) whose widths are computed
from the string lengths (`7 * (strLength + 4)` px, `:322`, `:328`, `:334`), plus a
mini-curve with a dot positioned by `rangeLocation`.

#### `tickToString(int24 tick) private pure returns (string memory)` — `:352`

Handles the sign manually since `uint256(negative)` would wrap.

#### `rangeLocation(int24 tickLower, int24 tickUpper) internal pure returns (string memory, string memory)` — `:361`

Maps the range midpoint to `(x, y)` on the mini-curve through ten buckets from
`< −125_000` to `≥ 125_000`. A visual "where on the price spectrum is this".

#### `generateSVGRareSparkle(uint256 tokenId, address poolAddress) private pure returns (string memory svg)` — `:386`

If `isRare`, emits a rotating sparkle (a 10-second infinite `<animateTransform>`).
Otherwise the empty string.

#### `isRare(uint256 tokenId, address poolAddress) internal pure returns (bool)` — `:402`

```solidity
bytes32 h = keccak256(abi.encodePacked(tokenId, poolAddress));
return uint256(h) < type(uint256).max / (1 + BitMath.mostSignificantBit(tokenId) * 2);
```

The threshold shrinks as `tokenId` grows: `mostSignificantBit` is ~1 for early
ids and ~20 by token 1,000,000, so the probability falls from ~1/3 to ~1/41.
**Early minters are likelier to be rare** — an on-chain, verifiable, purely
cosmetic scarcity gradient.

---

## 8. `examples/PairFlash`

`examples/PairFlash.sol` — 149 lines, `is IUniswapV3FlashCallback, PeripheryPayments`.
Not deployed; a reference implementation of a flash-loan arbitrage.

| Variable | Type | Line |
|---|---|---|
| `swapRouter` | `ISwapRouter public immutable` | `:21` |

```solidity
struct FlashCallbackData {   // :32
    uint256 amount0; uint256 amount1; address payer;
    PoolAddress.PoolKey poolKey; uint24 poolFee2; uint24 poolFee3;
}
struct FlashParams {         // :112
    address token0; address token1; uint24 fee1;
    uint256 amount0; uint256 amount1; uint24 fee2; uint24 fee3;
}
```

#### `constructor(ISwapRouter _swapRouter, address _factory, address _WETH9)` — `:23`

#### `initFlash(FlashParams memory params) external` — `:124`

Computes the pool from `(token0, token1, fee1)` and calls
`pool.flash(address(this), amount0, amount1, abi.encode(FlashCallbackData{...}))`.

#### `uniswapV3FlashCallback(uint256 fee0, uint256 fee1, bytes calldata data) external override` — `:46`

1. Decode, `CallbackValidation.verifyCallback(factory, decoded.poolKey)` (`:52`).
2. Compute the repayment floors: `amountNMin = decoded.amountN + feeN` (`:59-60`).
3. Approve the router and `exactInputSingle` token1→token0 in the `fee2` pool with
   `amountOutMinimum: amount0Min` (`:63-76`) — **the profitability check is the
   slippage check**. If the arb is not profitable the swap reverts.
4. Same in reverse through the `fee3` pool.
5. `pay(...)` the borrowed amount plus fee back to `msg.sender` (the pool).
6. Any surplus goes to `decoded.payer`.

The lesson: you do not need an explicit `require(profit > 0)`. Set
`amountOutMinimum` to the repayment obligation and the swap enforces it.

- **Gotcha:** `initFlash` has no access control, so anyone can trigger an arb — but
  `payer = msg.sender` means the caller takes the profit and, if it reverts, pays
  the gas. That is the correct incentive.

---
