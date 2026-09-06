# Uniswap V2 — Complete Reference

Every contract, every function, every revert string in `uni/v2-core` and
`uni/v2-periphery`. **35 Solidity files**: 12 in core, 23 in periphery. Nothing
is skipped — trivial interfaces get short entries, but they get entries.

This is a *reference*, meant to be read cover to cover once and then grepped
forever after. For the narrative version (why an AMM works at all, how V2
compares to V3/V4) see [`UNISWAP-DEEP-DIVE.md`](UNISWAP-DEEP-DIVE.md).

Every `file:line` here was verified with `grep -n` against these exact files.
Paths are relative to `uni/`.

---

## Table of contents

**Part I — Core** (`v2-core/`, Solidity `=0.5.16`)

- [1. UniswapV2Factory](#1-uniswapv2factory)
  - [1.1 State, events, constructor](#11-state-events-constructor)
  - [1.2 `allPairsLength()`](#12-allpairslength)
  - [1.3 `createPair(address,address)`](#13-createpairaddressaddress)
  - [1.4 `setFeeTo(address)`](#14-setfeetoaddress)
  - [1.5 `setFeeToSetter(address)`](#15-setfeetosetteraddress)
- [2. UniswapV2Pair](#2-uniswapv2pair)
  - [2.1 Storage layout and slot packing](#21-storage-layout-and-slot-packing)
  - [2.2 The `lock` modifier](#22-the-lock-modifier)
  - [2.3 `getReserves()`](#23-getreserves)
  - [2.4 `_safeTransfer(address,address,uint)`](#24-_safetransferaddressaddressuint)
  - [2.5 `constructor()` and `initialize(address,address)`](#25-constructor-and-initializeaddressaddress)
  - [2.6 `_update(...)` — reserves and the TWAP oracle](#26-_update--reserves-and-the-twap-oracle)
  - [2.7 `_mintFee(uint112,uint112)` — the protocol fee](#27-_mintfeeuint112uint112--the-protocol-fee)
  - [2.8 `mint(address)`](#28-mintaddress)
  - [2.9 `burn(address)`](#29-burnaddress)
  - [2.10 `swap(uint,uint,address,bytes)`](#210-swapuintuintaddressbytes)
  - [2.11 `skim(address)`](#211-skimaddress)
  - [2.12 `sync()`](#212-sync)
- [3. UniswapV2ERC20](#3-uniswapv2erc20)
  - [3.1 State and the EIP-712 domain](#31-state-and-the-eip-712-domain)
  - [3.2 `_mint` / `_burn` / `_approve` / `_transfer`](#32-_mint--_burn--_approve--_transfer)
  - [3.3 `approve` / `transfer` / `transferFrom`](#33-approve--transfer--transferfrom)
  - [3.4 `permit(...)` — EIP-2612](#34-permit--eip-2612)
- [4. Core libraries](#4-core-libraries)
  - [4.1 `Math`](#41-math)
  - [4.2 `SafeMath`](#42-safemath)
  - [4.3 `UQ112x112`](#43-uq112x112)
- [5. Core interfaces](#5-core-interfaces)
- [6. Core test helper](#6-core-test-helper)

**Part II — Periphery** (`v2-periphery/`, Solidity `=0.6.6`)

- [7. UniswapV2Library](#7-uniswapv2library)
- [8. UniswapV2Router02](#8-uniswapv2router02)
  - [8.1 State, modifier, constructor, receive](#81-state-modifier-constructor-receive)
  - [8.2 Add liquidity](#82-add-liquidity)
  - [8.3 Remove liquidity](#83-remove-liquidity)
  - [8.4 Remove liquidity (fee-on-transfer)](#84-remove-liquidity-fee-on-transfer)
  - [8.5 `_swap` and the six swap functions](#85-_swap-and-the-six-swap-functions)
  - [8.6 Fee-on-transfer swaps](#86-fee-on-transfer-swaps)
  - [8.7 Library passthroughs](#87-library-passthroughs)
- [9. UniswapV2Router01 and the getAmountIn bug](#9-uniswapv2router01-and-the-getamountin-bug)
- [10. UniswapV2OracleLibrary](#10-uniswapv2oraclelibrary)
- [11. UniswapV2LiquidityMathLibrary](#11-uniswapv2liquiditymathlibrary)
- [12. UniswapV2Migrator](#12-uniswapv2migrator)
- [13. Example contracts](#13-example-contracts)
  - [13.1 ExampleOracleSimple](#131-exampleoraclesimple)
  - [13.2 ExampleSlidingWindowOracle](#132-exampleslidingwindoworacle)
  - [13.3 ExampleFlashSwap](#133-exampleflashswap)
  - [13.4 ExampleSwapToPrice](#134-exampleswaptoprice)
  - [13.5 ExampleComputeLiquidityValue](#135-examplecomputeliquidityvalue)
- [14. Periphery interfaces](#14-periphery-interfaces)
- [15. Periphery test helpers](#15-periphery-test-helpers)

**Part III — Tables and recipes**

- [16. Use-case cookbook](#16-use-case-cookbook)
- [17. Selector / ABI tables](#17-selector--abi-tables)
- [18. Storage layout tables](#18-storage-layout-tables)
- [19. Events reference](#19-events-reference)
- [20. Revert-string reference](#20-revert-string-reference)
- [21. File inventory](#21-file-inventory)

---

## The whole system in one diagram

```
                        ┌──────────────────────────────────────┐
   deploys pairs        │        UniswapV2Factory              │
   via CREATE2  ────────│  getPair[t0][t1] -> pair             │
                        │  allPairs[]  feeTo  feeToSetter      │
                        └───────────────┬──────────────────────┘
                                        │ create2 + initialize()
                                        v
   ┌────────────────────────────────────────────────────────────────┐
   │                       UniswapV2Pair  (is UniswapV2ERC20)       │
   │  reserve0/reserve1/blockTimestampLast   <- 1 slot              │
   │  price0CumulativeLast, price1CumulativeLast, kLast             │
   │  mint() burn() swap() skim() sync()   [all `lock`]             │
   │  HOLDS THE TOKENS. Trusts nobody. Checks balances itself.      │
   └───────▲──────────────────────────────────────┬────────────────-┘
           │ low-level calls                      │ optimistic transfer
           │ (tokens must ALREADY be sent)        │ + uniswapV2Call()
           │                                      v
   ┌───────┴────────────────────┐        ┌──────────────────────────┐
   │   UniswapV2Router02        │        │  IUniswapV2Callee        │
   │  deadline + slippage       │        │  (flash-swap borrower,   │
   │  WETH wrapping, multi-hop  │        │   e.g. ExampleFlashSwap) │
   │  uses UniswapV2Library     │        └──────────────────────────┘
   └────────────────────────────┘
           uses (pure/view, no state)
                  │
                  v
   UniswapV2Library ─ pairFor (CREATE2 address, no call)
                    ─ quote / getAmountOut / getAmountIn
                    ─ getAmountsOut / getAmountsIn (multi-hop)
```

**The one rule that explains the whole design:** the Pair never trusts its
caller. It measures its own `balanceOf` before and after and enforces the
invariant. That is why the Router must transfer tokens to the Pair *before*
calling `mint` or `swap`, and why anyone can call `swap` directly if they know
what they are doing.

---

# Part I — Core

## 1. UniswapV2Factory

**File:** `v2-core/contracts/UniswapV2Factory.sol` (49 lines)
**Inheritance:** `IUniswapV2Factory`
**Purpose:** a registry and deployer. It creates one `UniswapV2Pair` per unordered
token pair at a deterministic address, and holds the two governance addresses
(`feeTo`, `feeToSetter`) that every Pair reads to decide whether the protocol
fee is on.

The Factory holds no tokens and can never move user funds. Its only power is
switching the protocol fee destination.

### 1.1 State, events, constructor

```solidity
address public feeTo;                                              // :7
address public feeToSetter;                                        // :8
mapping(address => mapping(address => address)) public getPair;     // :10
address[] public allPairs;                                          // :11
event PairCreated(address indexed token0, address indexed token1, address pair, uint); // :13
```

| Slot | Type | Name | Notes |
|---|---|---|---|
| 0 | `address` | `feeTo` | zero = protocol fee OFF (the default, and the value on mainnet for V2's entire life) |
| 1 | `address` | `feeToSetter` | may set both `feeTo` and its own successor |
| 2 | `mapping` | `getPair` | populated in BOTH directions, see `:34-35` |
| 3 | `address[]` | `allPairs` | append-only; `allPairs.length` is the pair count |

**`constructor(address _feeToSetter)`** — `v2-core/contracts/UniswapV2Factory.sol:15-17`.
Sets `feeToSetter = _feeToSetter`. Note `feeTo` is left at zero, so the protocol
fee starts off. No validation: passing `address(0)` permanently bricks fee
governance (nobody can ever call `setFeeTo`).

### 1.2 `allPairsLength()`

```
function allPairsLength() external view returns (uint)     v2-core/contracts/UniswapV2Factory.sol:19
```

- **Purpose:** number of pairs ever created.
- **Params:** none.
- **Checks:** none.
- **State writes:** none.
- **External calls:** none.
- **Returns:** `allPairs.length`. **Events:** none.
- **Callers:** off-chain indexers enumerating all pairs via `allPairs(i)` for `i < allPairsLength()`.
- **Gotcha:** the auto-generated `allPairs(uint)` getter is the companion; there is no way to remove a pair, so the index is stable forever.

### 1.3 `createPair(address,address)`

```
function createPair(address tokenA, address tokenB) external returns (address pair)
                                                          v2-core/contracts/UniswapV2Factory.sol:23
```

- **Purpose:** deploy the `UniswapV2Pair` for an unordered token pair, at an address that is a pure function of `(factory, token0, token1)`.
- **Params:** `tokenA`, `tokenB` — any two distinct non-zero ERC-20 addresses, in either order.
- **Checks:**
  - `:24` `require(tokenA != tokenB, 'UniswapV2: IDENTICAL_ADDRESSES')`
  - `:26` `require(token0 != address(0), 'UniswapV2: ZERO_ADDRESS')` — only `token0` is checked because after sorting `token0 < token1`, so `token0 != 0` implies `token1 != 0`.
  - `:27` `require(getPair[token0][token1] == address(0), 'UniswapV2: PAIR_EXISTS')` — the comment "single check is sufficient" holds because the mapping is written in both directions at `:34-35`.
- **State writes:** `getPair[token0][token1]`, `getPair[token1][token0]`, `allPairs.push(pair)`.
- **External calls:** `IUniswapV2Pair(pair).initialize(token0, token1)` at `:33`.
- **Returns:** the new pair address. **Emits:** `PairCreated(token0, token1, pair, allPairs.length)` at `:37`.
- **Who may call:** anyone. Pair creation is permissionless.
- **Callers in repo:** `UniswapV2Router01.sol:40` and `UniswapV2Router02.sol:43`, both inside `_addLiquidity`, which auto-creates the pair if it does not exist.

The deployment itself:

```solidity
bytes memory bytecode = type(UniswapV2Pair).creationCode;          // :28
bytes32 salt = keccak256(abi.encodePacked(token0, token1));        // :29
assembly {
    pair := create2(0, add(bytecode, 32), mload(bytecode), salt)   // :31
}
IUniswapV2Pair(pair).initialize(token0, token1);                   // :33
```

**Why CREATE2 and why `initialize` instead of constructor args.** The CREATE2
address is `keccak256(0xff ++ factory ++ salt ++ keccak256(creationCode))[12:]`.
For this to be computable off-chain without any RPC call, `keccak256(creationCode)`
must be a *constant* — which means the creation code cannot contain constructor
arguments. So the tokens are set afterwards by `initialize`, and the constructor
only records `factory = msg.sender` (`UniswapV2Pair.sol:61-63`).

That constant is the famous **init code hash**, hardcoded in the periphery at
`v2-periphery/contracts/libraries/UniswapV2Library.sol:24`:

```
0x96e8ac4277198ff8b6f785478aa9a39f403cb768dd02cbee326c3e7da348845f
```

- **Gotchas:**
  - The `assembly` `create2` return value is *not* checked for zero. If the pair already existed at that address the deployment would fail and `pair` would be `address(0)` — but the `PAIR_EXISTS` check at `:27` makes that unreachable through this function.
  - Because the salt is only `(token0, token1)`, there is exactly one pair per token pair per factory. Different fee tiers (a V3 idea) are impossible here.
  - Any fork that changes `UniswapV2Pair.sol` by even one byte gets a different init code hash, and the hardcoded value in `UniswapV2Library` must be recomputed. This is the single most common bug when forking Uniswap V2.

### 1.4 `setFeeTo(address)`

```
function setFeeTo(address _feeTo) external          v2-core/contracts/UniswapV2Factory.sol:40
```

- **Purpose:** turn the protocol fee on (non-zero) or off (zero) and choose its recipient.
- **Checks:** `:41` `require(msg.sender == feeToSetter, 'UniswapV2: FORBIDDEN')`.
- **State writes:** `feeTo = _feeTo`. **Events:** none (deliberately — no event on a governance change is a real observability gap).
- **Effect elsewhere:** every `UniswapV2Pair._mintFee` reads `IUniswapV2Factory(factory).feeTo()` (`UniswapV2Pair.sol:90`) on every `mint` and `burn`. Flipping this one address changes fee behaviour for every pair at once.

### 1.5 `setFeeToSetter(address)`

```
function setFeeToSetter(address _feeToSetter) external   v2-core/contracts/UniswapV2Factory.sol:45
```

- **Checks:** `:46` `require(msg.sender == feeToSetter, 'UniswapV2: FORBIDDEN')`.
- **State writes:** `feeToSetter = _feeToSetter`.
- **Gotcha:** single-step ownership transfer with no zero-check. Setting it to a wrong or zero address permanently loses fee governance for every pair.

---

## 2. UniswapV2Pair

**File:** `v2-core/contracts/UniswapV2Pair.sol` (201 lines)
**Inheritance:** `IUniswapV2Pair`, `UniswapV2ERC20`
**Purpose:** the AMM itself. Holds both tokens, enforces `x*y=k`, mints and burns
LP shares, and maintains the price accumulators.

Its three mutating entry points (`mint`, `burn`, `swap`) are all documented in
the source as *"this low-level function should be called from a contract which
performs important safety checks"* (`:109`, `:133`, `:158`). They have no
slippage protection and no deadline — that is the Router's job. They are safe
for the *pool*, not for a careless caller.

### 2.1 Storage layout and slot packing

```solidity
uint public constant MINIMUM_LIQUIDITY = 10**3;                              // :15
bytes4 private constant SELECTOR = bytes4(keccak256(bytes('transfer(address,uint256)'))); // :16

address public factory;        // :18
address public token0;         // :19
address public token1;         // :20

uint112 private reserve0;           // :22  ┐
uint112 private reserve1;           // :23  ├─ ONE storage slot
uint32  private blockTimestampLast; // :24  ┘

uint public price0CumulativeLast;   // :26
uint public price1CumulativeLast;   // :27
uint public kLast;                  // :28
uint private unlocked = 1;          // :30
```

Slots 0–4 belong to the inherited `UniswapV2ERC20` (see §18 for the full table).
The Pair's own storage starts after them.

`112 + 112 + 32 = 256` bits exactly. This is the single most important gas
optimisation in the contract: `getReserves()` is one `SLOAD`, and `_update`
writing all three is one `SSTORE`. Every constraint that follows — the `uint112`
overflow check at `:74`, the `uint32` timestamp wrapping at `:75`, the
`UQ112x112` fixed-point format — exists to make this packing work.

`MINIMUM_LIQUIDITY = 1000` is burned on the first mint to make the
first-depositor share-inflation attack unprofitable (see §2.8).

`SELECTOR` is precomputed `transfer(address,uint256)` = `0xa9059cbb`, used by
`_safeTransfer` to support non-standard ERC-20s.

### 2.2 The `lock` modifier

```solidity
uint private unlocked = 1;                                  // :30
modifier lock() {                                           // :31-36
    require(unlocked == 1, 'UniswapV2: LOCKED');
    unlocked = 0;
    _;
    unlocked = 1;
}
```

Applied to `mint` (`:110`), `burn` (`:134`), `swap` (`:159`), `skim` (`:190`),
`sync` (`:198`). It uses `1`/`0` rather than a `bool` — in 0.5.16 both cost the
same, but the pattern matters because `swap` hands control to arbitrary code via
`uniswapV2Call` (`:172`). Without this, a flash-swap borrower could re-enter
`swap` and observe or exploit a half-updated pool.

Note the lock guards *state-changing* functions only. `getReserves()` is
un-guarded and can be read mid-callback while balances have already moved but
reserves have not — the read-only reentrancy shape. In V2 this is not
exploitable against the pool, but integrators pricing off `getReserves()` inside
a callback can be fooled.

### 2.3 `getReserves()`

```
function getReserves() public view returns (uint112 _reserve0, uint112 _reserve1, uint32 _blockTimestampLast)
                                                              v2-core/contracts/UniswapV2Pair.sol:38
```

- **Purpose:** read the packed slot. This is the canonical way to get the pool price: `price of token0 in token1 = reserve1 / reserve0`.
- **Checks / writes / calls:** none.
- **Returns:** the three packed fields. **Events:** none.
- **Callers:** everywhere — `mint:111`, `burn:135`, `swap:161`, `UniswapV2Library.getReserves:31`, `UniswapV2OracleLibrary.currentCumulativePrices:24`, `Router02._swapSupportingFeeOnTransferTokens:329`.
- **Gotcha:** these are the *reserves*, not `balanceOf`. They differ whenever someone has sent tokens to the pair without calling a function (donation), between the transfer and the `mint`/`swap` that consumes it, or with a rebasing token. `skim`/`sync` reconcile them.

### 2.4 `_safeTransfer(address,address,uint)`

```
function _safeTransfer(address token, address to, uint value) private
                                                              v2-core/contracts/UniswapV2Pair.sol:44
```

```solidity
(bool success, bytes memory data) = token.call(abi.encodeWithSelector(SELECTOR, to, value));
require(success && (data.length == 0 || abi.decode(data, (bool))), 'UniswapV2: TRANSFER_FAILED');
```

- **Purpose:** transfer out an ERC-20 that may or may not return a bool.
- **Checks:** the call must succeed, AND either return nothing (USDT-style) or decode to `true`.
- **Callers:** `burn:148-149`, `swap:170-171`, `skim:193-194`.
- **Gotchas:**
  - Uses raw `.call`, so a token with no code returns `success = true` with empty data and the require *passes*. Creating a pair against a non-contract address produces a pool where transfers silently no-op. The Factory does not check `codesize`.
  - Only `transfer` is wrapped, never `transferFrom` — the Pair never pulls tokens, it only measures what it already has.

### 2.5 `constructor()` and `initialize(address,address)`

**`constructor()`** — `v2-core/contracts/UniswapV2Pair.sol:61-63`. Sets
`factory = msg.sender`. Takes no arguments so that the creation code hash is
constant (see §1.3).

**`initialize(address _token0, address _token1)`** — `:66-70`.

- **Purpose:** one-time setting of the two token addresses, called by the Factory immediately after `create2`.
- **Checks:** `:67` `require(msg.sender == factory, 'UniswapV2: FORBIDDEN')`. The source comment calls this "sufficient check" — it is, because the Factory only ever calls it once, at `UniswapV2Factory.sol:33`, immediately after deployment, within the same transaction.
- **State writes:** `token0`, `token1`.
- **Gotcha:** there is no `initialized` flag. The safety rests entirely on the Factory calling it exactly once. A forked factory that calls `initialize` twice would re-point a live pool at different tokens.

### 2.6 `_update(...)` — reserves and the TWAP oracle

```
function _update(uint balance0, uint balance1, uint112 _reserve0, uint112 _reserve1) private
                                                              v2-core/contracts/UniswapV2Pair.sol:73
```

```solidity
require(balance0 <= uint112(-1) && balance1 <= uint112(-1), 'UniswapV2: OVERFLOW');   // :74
uint32 blockTimestamp = uint32(block.timestamp % 2**32);                              // :75
uint32 timeElapsed = blockTimestamp - blockTimestampLast; // overflow is desired       // :76
if (timeElapsed > 0 && _reserve0 != 0 && _reserve1 != 0) {
    // * never overflows, and + overflow is desired
    price0CumulativeLast += uint(UQ112x112.encode(_reserve1).uqdiv(_reserve0)) * timeElapsed;  // :79
    price1CumulativeLast += uint(UQ112x112.encode(_reserve0).uqdiv(_reserve1)) * timeElapsed;  // :80
}
reserve0 = uint112(balance0);                                                         // :82
reserve1 = uint112(balance1);                                                         // :83
blockTimestampLast = blockTimestamp;                                                  // :84
emit Sync(reserve0, reserve1);                                                        // :85
```

- **Purpose:** write the new reserves and, once per block, advance the price accumulators.
- **Params:** the *new* balances to store, and the *old* reserves used to price the elapsed interval.
- **Checks:** `:74` overflow guard — balances must fit in `uint112`.
- **State writes:** `price0CumulativeLast`, `price1CumulativeLast`, `reserve0`, `reserve1`, `blockTimestampLast`.
- **Emits:** `Sync(reserve0, reserve1)`.
- **Callers:** `mint:128`, `burn:153`, `swap:185`, `sync:199`.
- **The three deliberate overflows:**
  1. `block.timestamp % 2**32` (`:75`) — wraps in Feb 2106. Harmless because only *differences* are used.
  2. `blockTimestamp - blockTimestampLast` (`:76`) — `uint32` subtraction wraps correctly across the 2106 boundary, so `timeElapsed` stays right.
  3. `price0CumulativeLast +=` (`:79`) — `uint256` accumulator is allowed to wrap. Consumers compute `(cum_now - cum_then)`, which is correct under wrapping as long as less than `2^256` accumulates between reads.
- **Key detail — the accumulators use the OLD reserves.** `_update` prices the interval that just *ended* using the reserves that were in effect during it. This is what makes the TWAP an integral of price over time rather than a sample.
- **Key detail — once per block.** `timeElapsed > 0` means the first trade in a block advances the accumulator using the previous block's price, and later trades in the same block do not. **This is what makes the accumulator manipulation-resistant:** an attacker who moves the price and restores it within one block contributes nothing to the accumulator. To move a TWAP they must hold the manipulated price across a block boundary and eat the arbitrage.
- **Gotcha:** if a pair's balance would exceed `2^112 - 1` (~5.19e33), `_update` reverts and the pool is bricked for `mint`/`swap`/`sync` until someone calls `skim` (`:190`) to remove the excess. Anyone can trigger this by donating an enormous amount of a token with huge supply.

### 2.7 `_mintFee(uint112,uint112)` — the protocol fee

```
function _mintFee(uint112 _reserve0, uint112 _reserve1) private returns (bool feeOn)
                                                              v2-core/contracts/UniswapV2Pair.sol:89
```

```solidity
address feeTo = IUniswapV2Factory(factory).feeTo();     // :90  external call
feeOn = feeTo != address(0);                            // :91
uint _kLast = kLast;                                    // :92
if (feeOn) {
    if (_kLast != 0) {
        uint rootK     = Math.sqrt(uint(_reserve0).mul(_reserve1));   // :95
        uint rootKLast = Math.sqrt(_kLast);                           // :96
        if (rootK > rootKLast) {
            uint numerator   = totalSupply.mul(rootK.sub(rootKLast)); // :98
            uint denominator = rootK.mul(5).add(rootKLast);           // :99
            uint liquidity = numerator / denominator;                 // :100
            if (liquidity > 0) _mint(feeTo, liquidity);               // :101
        }
    }
} else if (_kLast != 0) {
    kLast = 0;                                                        // :105
}
```

- **Purpose:** if the fee switch is on, mint `feeTo` new LP tokens worth 1/6 of the growth in `sqrt(k)` since the last liquidity event.
- **Params:** the reserves *before* this operation.
- **Checks:** none that revert. Every branch is conditional.
- **State writes:** `totalSupply` and `balanceOf[feeTo]` via `_mint`; or `kLast = 0` when the fee is turned off.
- **External calls:** `IUniswapV2Factory(factory).feeTo()` — one `STATICCALL` on every `mint` and `burn`.
- **Returns:** `feeOn`, which the caller uses to decide whether to refresh `kLast`.
- **Callers:** `mint:117`, `burn:142`. Never on `swap` — fees accrue into `k` continuously and are only *crystallised* at liquidity events.

**The 1/6 formula, derived.** LPs own `sqrt(k)` per share. Let `s1 = sqrt(k)` now
and `s0 = sqrt(kLast)`. Growth in `sqrt(k)` is entirely fee income. The protocol
should take a fraction `phi = 1/6` of it. Minting `L` new tokens against existing
supply `S` dilutes everyone to `S/(S+L)`, so we need:

```
value_to_feeTo = L/(S+L) * s1  =  phi * (s1 - s0)
```

Solving for `L` with `phi = 1/6`:

```
L = S * (s1 - s0) / (s1*(1/phi - 1) + s0) = S * (s1 - s0) / (5*s1 + s0)
```

which is exactly `numerator / denominator` at `:98-100`. Change the `5` to
`(1/phi - 1)` for any other fraction.

**Why 1/6 of `sqrt(k)` growth equals 1/6 of fees, i.e. 0.05% of volume:** the
total swap fee is 0.30%; `sqrt(k)` growth captures all of it; one sixth of 0.30%
is 0.05%. That is the "0.30% to LPs, or 0.25% to LPs + 0.05% to protocol"
description you see in the docs.

- **Gotchas:**
  - `kLast` is only refreshed at `mint:129` and `burn:154`, and only `if (feeOn)`. So when the fee is off, `kLast` stays stale at whatever it was.
  - The `else if (_kLast != 0) kLast = 0` branch (`:104-105`) is essential: when the fee is switched off, `kLast` must be zeroed so that if it is later switched back on, the protocol does not retroactively claim a share of fees earned during the off period.
  - `Math.sqrt` truncates, so the fee rounds slightly in the LPs' favour.
  - This adds an external call to the factory on every liquidity event even when the fee is off — a known gas cost of the design.

### 2.8 `mint(address)`

```
function mint(address to) external lock returns (uint liquidity)
                                                             v2-core/contracts/UniswapV2Pair.sol:110
```

- **Purpose:** convert tokens *already sitting in the contract* into LP shares.
- **Params:** `to` — who receives the LP tokens.
- **Preconditions the Pair does NOT check:** the caller must have already transferred both tokens in. The Pair discovers how much by differencing balance against reserve.

**Line by line:**

```solidity
(uint112 _reserve0, uint112 _reserve1,) = getReserves();               // :111
uint balance0 = IERC20(token0).balanceOf(address(this));               // :112  external call
uint balance1 = IERC20(token1).balanceOf(address(this));               // :113  external call
uint amount0 = balance0.sub(_reserve0);                                // :114
uint amount1 = balance1.sub(_reserve1);                                // :115

bool feeOn = _mintFee(_reserve0, _reserve1);                           // :117
uint _totalSupply = totalSupply; // must be read AFTER _mintFee        // :118
if (_totalSupply == 0) {
    liquidity = Math.sqrt(amount0.mul(amount1)).sub(MINIMUM_LIQUIDITY);// :120
   _mint(address(0), MINIMUM_LIQUIDITY);                               // :121
} else {
    liquidity = Math.min(amount0.mul(_totalSupply) / _reserve0,
                         amount1.mul(_totalSupply) / _reserve1);       // :123
}
require(liquidity > 0, 'UniswapV2: INSUFFICIENT_LIQUIDITY_MINTED');    // :125
_mint(to, liquidity);                                                  // :126
_update(balance0, balance1, _reserve0, _reserve1);                     // :128
if (feeOn) kLast = uint(reserve0).mul(reserve1);                       // :129
emit Mint(msg.sender, amount0, amount1);                               // :130
```

- **Checks:** `:114-115` implicit — `SafeMath.sub` reverts with `ds-math-sub-underflow` if balance < reserve. `:125` `INSUFFICIENT_LIQUIDITY_MINTED`.
- **State writes:** `totalSupply`, `balanceOf[to]`, `balanceOf[address(0)]` (first mint only), `reserve0/1`, `blockTimestampLast`, accumulators, possibly `kLast`.
- **External calls:** two `balanceOf`, plus `factory.feeTo()` inside `_mintFee`.
- **Emits:** `Transfer` (from `_mint`), possibly a second `Transfer` for the fee, `Sync`, `Mint`.
- **Callers:** `Router01.sol:72`, `:94`; `Router02.sol:75`, `:97`.

**First mint — `sqrt(amount0 * amount1)`.** The geometric mean makes the initial
share count independent of the units of either token, so LP token value does not
depend on which token you call "first". `MINIMUM_LIQUIDITY = 1000` shares are
minted to `address(0)` — permanently unrecoverable.

**Why the burn matters.** Without it, an attacker could mint exactly 1 wei of LP
(by depositing 1 wei of each token), then *donate* a large amount directly to the
pair. Now 1 LP share owns the whole pool. The next depositor's
`amount * totalSupply / reserve` rounds to zero and they get nothing. Burning
1000 shares means the attacker must donate enough to make each of 1000 shares
valuable, which costs at least 1000× more than the profit.

**Subsequent mints — `min` of the two ratios.** You get shares proportional to
the *scarcer* of your two contributions. Anything you over-contribute is simply
absorbed by the pool and given to existing LPs. This is why the Router computes
optimal amounts with `quote` first (`Router02.sol:49`, `:54`) — the Pair itself
will happily eat your excess.

- **Gotchas:**
  - `_totalSupply` MUST be cached after `_mintFee` (the source comment at `:118` says so) because `_mintFee` can increase it.
  - Both branches round *down*, favouring the pool.
  - Sending tokens and calling `mint` must be atomic. Between your transfer and your `mint`, anyone can call `mint(theirAddress)` and claim your deposit.

### 2.9 `burn(address)`

```
function burn(address to) external lock returns (uint amount0, uint amount1)
                                                             v2-core/contracts/UniswapV2Pair.sol:134
```

- **Purpose:** redeem LP tokens *already sent to the contract* for a pro-rata slice of both reserves.

```solidity
uint liquidity = balanceOf[address(this)];                             // :140
bool feeOn = _mintFee(_reserve0, _reserve1);                           // :142
uint _totalSupply = totalSupply;                                       // :143
amount0 = liquidity.mul(balance0) / _totalSupply;   // uses BALANCES    // :144
amount1 = liquidity.mul(balance1) / _totalSupply;                      // :145
require(amount0 > 0 && amount1 > 0, 'UniswapV2: INSUFFICIENT_LIQUIDITY_BURNED'); // :146
_burn(address(this), liquidity);                                       // :147
_safeTransfer(_token0, to, amount0);                                   // :148
_safeTransfer(_token1, to, amount1);                                   // :149
balance0 = IERC20(_token0).balanceOf(address(this));                   // :150  re-read
balance1 = IERC20(_token1).balanceOf(address(this));                   // :151  re-read
_update(balance0, balance1, _reserve0, _reserve1);                     // :153
if (feeOn) kLast = uint(reserve0).mul(reserve1);                       // :154
emit Burn(msg.sender, amount0, amount1, to);                           // :155
```

- **Checks:** `:146` `INSUFFICIENT_LIQUIDITY_BURNED` — burning a dust amount that rounds to zero on either side reverts.
- **State writes:** `balanceOf[address(this)]`, `totalSupply`, reserves, accumulators, possibly `kLast`.
- **External calls:** 2 `balanceOf` before, 2 `_safeTransfer`, 2 `balanceOf` after, plus `feeTo()`.
- **Emits:** `Transfer` (burn), `Sync`, `Burn`.
- **Callers:** `Router01.sol:110`, `Router02.sol:114`.
- **Why balances, not reserves (`:144-145`, and the source comment says "using balances ensures pro-rata distribution").** If someone donated tokens to the pair, those donated tokens belong to LPs. Using `balance` distributes them; using `reserve` would strand them until `skim`.
- **Why balances are re-read at `:150-151`.** With a fee-on-transfer token, the pair sends `amount0` but its balance drops by more than `amount0`. Re-reading makes the reserves reflect what the pair *actually* holds.
- **Gotcha:** LP tokens must be transferred to the pair first. `Router02.sol:113` does `transferFrom(msg.sender, pair, liquidity)` immediately before calling `burn`.

### 2.10 `swap(uint,uint,address,bytes)`

```
function swap(uint amount0Out, uint amount1Out, address to, bytes calldata data) external lock
                                                             v2-core/contracts/UniswapV2Pair.sol:159
```

The most important function in the contract. Note it takes **outputs**, not
inputs: you tell the pool what you want, send what you owe, and the pool checks
`k` at the end.

```solidity
require(amount0Out > 0 || amount1Out > 0, 'UniswapV2: INSUFFICIENT_OUTPUT_AMOUNT');   // :160
(uint112 _reserve0, uint112 _reserve1,) = getReserves();                              // :161
require(amount0Out < _reserve0 && amount1Out < _reserve1, 'UniswapV2: INSUFFICIENT_LIQUIDITY'); // :162

uint balance0; uint balance1;
{ // scope for _token{0,1}, avoids stack too deep errors
address _token0 = token0;
address _token1 = token1;
require(to != _token0 && to != _token1, 'UniswapV2: INVALID_TO');                     // :169
if (amount0Out > 0) _safeTransfer(_token0, to, amount0Out);  // OPTIMISTIC             // :170
if (amount1Out > 0) _safeTransfer(_token1, to, amount1Out);                            // :171
if (data.length > 0) IUniswapV2Callee(to).uniswapV2Call(msg.sender, amount0Out, amount1Out, data); // :172
balance0 = IERC20(_token0).balanceOf(address(this));                                  // :173
balance1 = IERC20(_token1).balanceOf(address(this));                                  // :174
}
uint amount0In = balance0 > _reserve0 - amount0Out ? balance0 - (_reserve0 - amount0Out) : 0;  // :176
uint amount1In = balance1 > _reserve1 - amount1Out ? balance1 - (_reserve1 - amount1Out) : 0;  // :177
require(amount0In > 0 || amount1In > 0, 'UniswapV2: INSUFFICIENT_INPUT_AMOUNT');       // :178
{
uint balance0Adjusted = balance0.mul(1000).sub(amount0In.mul(3));                      // :180
uint balance1Adjusted = balance1.mul(1000).sub(amount1In.mul(3));                      // :181
require(balance0Adjusted.mul(balance1Adjusted) >= uint(_reserve0).mul(_reserve1).mul(1000**2), 'UniswapV2: K'); // :182
}
_update(balance0, balance1, _reserve0, _reserve1);                                     // :185
emit Swap(msg.sender, amount0In, amount1In, amount0Out, amount1Out, to);               // :186
```

- **Params:** `amount0Out`/`amount1Out` — exactly what you want out (one is normally zero). `to` — recipient, and also the callback target. `data` — if non-empty, triggers the flash-swap callback.
- **Checks, in order:**
  - `:160` at least one output requested.
  - `:162` outputs are strictly less than reserves (strictly — you can never drain a pool completely).
  - `:169` `to` is neither token. **This prevents `_safeTransfer(token0, token0, x)`, which for some tokens would be a self-transfer that leaves the balance unchanged and lets the k-check pass with no real input.**
  - `:178` at least one input actually arrived.
  - `:182` the k-check.
- **State writes:** reserves, timestamp, accumulators (all via `_update`).
- **External calls, in order:** `transfer` out (up to 2), then `uniswapV2Call` on `to` if `data` is non-empty, then 2 `balanceOf`.
- **Emits:** `Sync`, `Swap`.
- **Callers:** `Router01.sol:176`, `Router02.sol:219` (`_swap`), `Router02.sol:336` (`_swapSupportingFeeOnTransferTokens`).

**The k-check, derived.** The fee is 0.30% of the input. Rather than deducting it,
V2 checks the invariant against *fee-adjusted* balances. `balance0Adjusted =
balance0*1000 - amount0In*3` means "the balance as if only 99.7% of the input had
been added". The check is:

```
(b0*1000 - 3*in0) * (b1*1000 - 3*in1)  >=  r0 * r1 * 1000^2
```

Dividing both sides by `1000^2`: `(b0 - 0.003*in0) * (b1 - 0.003*in1) >= r0*r1`.
So `k` computed on fee-discounted balances must not decrease. The 0.3% that was
*not* discounted stays in the pool permanently, growing real `k` and therefore
every LP's claim. **The fee is never transferred anywhere. It is just the gap
between the two versions of `k`.**

Multiplying by 1000 first keeps everything in integers with no precision loss.

**The `amountIn` computation at `:176-177`** looks convoluted but is simply
"balance now, minus what the balance *should* be after the output, if positive".
The ternary avoids an underflow revert when nothing came in on that side.

**Flash swaps.** The transfer at `:170-171` happens *before* any payment is
verified. If `data` is non-empty, `to` gets a callback at `:172` and can do
anything — including trading elsewhere and returning the proceeds. As long as the
balances satisfy the k-check when the callback returns, the swap stands. Two ways
to pay: send back the *other* token (a normal swap, paid late), or send back the
*same* token plus the 0.3% fee (a flash loan). Both are just the k-check.

The `lock` modifier is what makes handing control to `to` safe: the borrower
cannot re-enter `swap`, `mint`, `burn`, `skim`, or `sync`.

- **Gotchas:**
  - `msg.sender` is passed to the callback (`:172`) so the borrower knows who initiated. `ExampleFlashSwap.sol:55` uses it to return profit.
  - Because the pool checks its own balances, a fee-on-transfer token silently reduces the input. The plain Router path computes amounts with `getAmountsOut` and will fail the k-check; `Router02`'s `SupportingFeeOnTransferTokens` variants (`:321`) measure the actual delivered amount instead.
  - Nothing stops you from swapping both directions at once. The k-check handles it.
  - Anyone can call `swap` directly. If you send tokens and request too little output, the surplus is simply donated to LPs — no revert.

### 2.11 `skim(address)`

```
function skim(address to) external lock            v2-core/contracts/UniswapV2Pair.sol:190
```

```solidity
_safeTransfer(_token0, to, IERC20(_token0).balanceOf(address(this)).sub(reserve0));   // :193
_safeTransfer(_token1, to, IERC20(_token1).balanceOf(address(this)).sub(reserve1));   // :194
```

- **Purpose:** send `balance - reserve` (the un-accounted surplus) to `to`.
- **Checks:** none explicit; `SafeMath.sub` reverts if reserve somehow exceeds balance.
- **State writes:** none — reserves are untouched. Only balances change.
- **Who may call:** anyone. **Callers in repo:** none; it is a recovery tool.
- **Use cases:** (1) recover tokens accidentally sent to a pair, (2) un-brick a pool whose balance exceeded `uint112` and now reverts in `_update:74`, (3) collect the surplus of a positively-rebasing token.
- **Gotcha:** `skim` is a race. Any surplus is claimable by whoever calls first, so tokens mistakenly sent to a pair are usually gone to a bot.

### 2.12 `sync()`

```
function sync() external lock                      v2-core/contracts/UniswapV2Pair.sol:198
```

```solidity
_update(IERC20(token0).balanceOf(address(this)), IERC20(token1).balanceOf(address(this)), reserve0, reserve1);
```

- **Purpose:** force reserves to equal current balances — the opposite choice from `skim`. Donated tokens become LP property, and the pool price moves.
- **State writes:** all of `_update`'s.
- **Emits:** `Sync`.
- **Who may call:** anyone.
- **Use cases:** after a donation you want to gift to LPs; after a *negative* rebase, where balance dropped below reserve and every quote is now wrong.
- **Gotcha:** `sync` changes the price with no trade and no fee. On a pool holding a rebasing token, calling `sync` at the right moment is a real arbitrage vector.

---

## 3. UniswapV2ERC20

**File:** `v2-core/contracts/UniswapV2ERC20.sol` (94 lines)
**Inheritance:** `IUniswapV2ERC20`; inherited *by* `UniswapV2Pair`
**Purpose:** the LP token. A minimal ERC-20 with EIP-2612 `permit`, so liquidity
can be removed in one transaction instead of approve-then-call.

Because `UniswapV2Pair is UniswapV2ERC20`, every pair IS an ERC-20 named
"Uniswap V2" / "UNI-V2". There is no separate LP token contract.

### 3.1 State and the EIP-712 domain

```solidity
string public constant name = 'Uniswap V2';       // :9
string public constant symbol = 'UNI-V2';         // :10
uint8 public constant decimals = 18;              // :11
uint public totalSupply;                          // :12   slot 0
mapping(address => uint) public balanceOf;        // :13   slot 1
mapping(address => mapping(address => uint)) public allowance;  // :14  slot 2
bytes32 public DOMAIN_SEPARATOR;                  // :16   slot 3
bytes32 public constant PERMIT_TYPEHASH = 0x6e71edae12b1b97f4d1f60370fef10105fa2faae0126114a169c64845d6126c9; // :18
mapping(address => uint) public nonces;           // :19   slot 4
```

Constants take no storage. `PERMIT_TYPEHASH` is verifiably
`keccak256("Permit(address owner,address spender,uint256 value,uint256 nonce,uint256 deadline)")`
— I recomputed it and it matches the literal at `:18` exactly.

**`constructor()`** — `:24-38`:

```solidity
uint chainId;
assembly { chainId := chainid }                   // :26-28
DOMAIN_SEPARATOR = keccak256(abi.encode(
    keccak256('EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)'),
    keccak256(bytes(name)),      // 'Uniswap V2'
    keccak256(bytes('1')),       // version
    chainId,
    address(this)
));
```

- **Why assembly:** Solidity 0.5.16 predates `block.chainid`, so `chainid` is read with inline assembly.
- **Gotcha — the chain-split bug.** `DOMAIN_SEPARATOR` is computed once at deploy and stored. If the chain hard-forks into two chains with different chain IDs, the stored separator is wrong on the new chain, and permits signed for one chain are replayable on the other. Modern implementations recompute it when `block.chainid` changes. V2 does not.
- Because it is computed in the constructor and `address(this)` differs per pair, every pair has a distinct domain separator — permits are not replayable across pairs.

### 3.2 `_mint` / `_burn` / `_approve` / `_transfer`

| Function | Line | Visibility | Writes | Emits |
|---|---|---|---|---|
| `_mint(address to, uint value)` | `:40-44` | `internal` | `totalSupply +=`, `balanceOf[to] +=` | `Transfer(address(0), to, value)` |
| `_burn(address from, uint value)` | `:46-50` | `internal` | `balanceOf[from] -=`, `totalSupply -=` | `Transfer(from, address(0), value)` |
| `_approve(address owner, address spender, uint value)` | `:52-55` | `private` | `allowance[owner][spender] = value` | `Approval` |
| `_transfer(address from, address to, uint value)` | `:57-61` | `private` | both balances | `Transfer` |

All arithmetic goes through `SafeMath`, so underflow reverts with
`ds-math-sub-underflow` rather than a named error — a mildly confusing UX when a
transfer exceeds balance.

`_mint` and `_burn` are `internal` precisely so `UniswapV2Pair` can call them
(`Pair:121`, `:126`, `:101`, `:147`). `_approve` and `_transfer` are `private`
because the Pair has no business calling them.

### 3.3 `approve` / `transfer` / `transferFrom`

**`approve(address spender, uint value) external returns (bool)`** — `:63-66`.
Standard. No zero-first requirement, so the classic ERC-20 approval race exists.

**`transfer(address to, uint value) external returns (bool)`** — `:68-71`.
Standard. Always returns `true` (reverts otherwise).

**`transferFrom(address from, address to, uint value) external returns (bool)`** — `:73-79`:

```solidity
if (allowance[from][msg.sender] != uint(-1)) {
    allowance[from][msg.sender] = allowance[from][msg.sender].sub(value);
}
_transfer(from, to, value);
```

- **Gotcha:** `uint(-1)` (max uint256) is treated as *infinite* approval and is never decremented — a gas optimisation that also means a max approval can never be partially consumed.
- **Callers:** `Router01.sol:109` and `Router02.sol:113` use it to pull LP tokens from the user into the pair before `burn`.

### 3.4 `permit(...)` — EIP-2612

```
function permit(address owner, address spender, uint value, uint deadline, uint8 v, bytes32 r, bytes32 s) external
                                                             v2-core/contracts/UniswapV2ERC20.sol:81
```

```solidity
require(deadline >= block.timestamp, 'UniswapV2: EXPIRED');            // :82
bytes32 digest = keccak256(abi.encodePacked(
    '\x19\x01', DOMAIN_SEPARATOR,
    keccak256(abi.encode(PERMIT_TYPEHASH, owner, spender, value, nonces[owner]++, deadline))
));                                                                     // :83-89
address recoveredAddress = ecrecover(digest, v, r, s);                  // :90
require(recoveredAddress != address(0) && recoveredAddress == owner, 'UniswapV2: INVALID_SIGNATURE'); // :91
_approve(owner, spender, value);                                        // :92
```

- **Purpose:** set an allowance from an off-chain signature, so removing liquidity is one transaction.
- **Checks:** `:82` deadline; `:91` signature recovers to `owner` and is not the zero address (`ecrecover` returns zero on malformed input, hence the explicit check).
- **State writes:** `nonces[owner]++` (inside the hash, `:87`), then `allowance[owner][spender]`.
- **Emits:** `Approval`.
- **Callers:** `Router01.sol:149`, `:163`; `Router02.sol:153`, `:167`, `:204`.
- **Gotchas:**
  - The nonce increments *inside the digest computation* at `:87`, which is correct but easy to misread. Each signature is single-use.
  - No EIP-1271 support: contract wallets cannot use `permit` here.
  - No `s`-value malleability check. A signature `(v,r,s)` and its counterpart `(v', r, n-s)` both recover to `owner`. Since the nonce is consumed either way, the practical impact is limited to off-chain systems that key on signature bytes.
  - `permit` is `external` and callable by anyone holding the signature — that is the point, but it also means the "spender" front-running the permit is normal, not an attack.

---

## 4. Core libraries

### 4.1 `Math`

**File:** `v2-core/contracts/libraries/Math.sol` (23 lines)

**`min(uint x, uint y) internal pure returns (uint z)`** — `:6-8`. `z = x < y ? x : y`.
Used at `Pair:123` to pick the limiting side of a deposit.

**`sqrt(uint y) internal pure returns (uint z)`** — `:11-22`. Babylonian method:

```solidity
if (y > 3) {
    z = y;
    uint x = y / 2 + 1;
    while (x < z) { z = x; x = (y / x + x) / 2; }
} else if (y != 0) {
    z = 1;
}
```

- Converges quadratically; ~7 iterations for values near `2^256`.
- Returns `floor(sqrt(y))`; `sqrt(0) = 0`, and `sqrt(1..3) = 1`.
- **Callers:** `Pair:120` (first mint) and `Pair:95-96` (`_mintFee`).
- **Gotcha:** the loop is unbounded in principle. It always terminates for `uint256` inputs, but forks that alter it should be careful.

### 4.2 `SafeMath`

**File:** `v2-core/contracts/libraries/SafeMath.sol` (17 lines). Identical copy at
`v2-periphery/contracts/libraries/SafeMath.sol` (pragma `=0.6.6` instead of `=0.5.16`).

| Function | Line | Reverts with |
|---|---|---|
| `add(uint x, uint y)` | `:6-8` | `ds-math-add-overflow` |
| `sub(uint x, uint y)` | `:10-12` | `ds-math-sub-underflow` |
| `mul(uint x, uint y)` | `:14-16` | `ds-math-mul-overflow` |

Only three operations — division cannot overflow and is used bare. Note there is
no `div`, so all division in V2 is unchecked native `/` (safe: Solidity reverts
on division by zero).

**Gotcha:** these are the errors users actually see when they try to burn more LP
than they hold, or when a pair's math overflows. They are not user-friendly and
have no Uniswap prefix, which makes debugging harder.

### 4.3 `UQ112x112`

**File:** `v2-core/contracts/libraries/UQ112x112.sol` (20 lines)

A binary fixed-point format: a `uint224` holding a `uint112` integer part and 112
fractional bits. Range `[0, 2^112 - 1]`, resolution `1 / 2^112`.

```solidity
uint224 constant Q112 = 2**112;                                       // :9
function encode(uint112 y) internal pure returns (uint224 z) {
    z = uint224(y) * Q112; // never overflows                         // :13
}
function uqdiv(uint224 x, uint112 y) internal pure returns (uint224 z) {
    z = x / uint224(y);                                               // :18
}
```

- `encode` cannot overflow: `uint112` max × `2^112` = `2^224 - 2^112` < `uint224` max.
- `uqdiv` has no zero check — `_update` guards it with `_reserve0 != 0 && _reserve1 != 0` at `Pair:77`.
- **Why 224 bits:** the product `UQ112x112 × uint32 timeElapsed` must fit in `uint256`. `224 + 32 = 256`. The entire fixed-point choice falls out of the reserve packing.
- **Callers:** `Pair:79-80` only.
- **Precision:** for a pair like USDC (6 decimals) / WETH (18 decimals), the raw ratio is tiny and the 112 fractional bits carry it fine. The format is *relative*, so consumers must adjust for decimals themselves.

---

## 5. Core interfaces

Five interface files, no logic. Listed for completeness.

| File | Lines | Contents |
|---|---|---|
| `v2-core/contracts/interfaces/IERC20.sol` | 17 | Full ERC-20 including `name`/`symbol`/`decimals` as `view`. Used by the Pair to call `balanceOf` on the underlying tokens. |
| `v2-core/contracts/interfaces/IUniswapV2Callee.sol` | 5 | One function: `uniswapV2Call(address sender, uint amount0, uint amount1, bytes calldata data)` — the flash-swap callback, invoked at `Pair:172`. Selector `0x10d1e85c`. |
| `v2-core/contracts/interfaces/IUniswapV2ERC20.sol` | 23 | The LP-token ABI. Note `name`/`symbol`/`decimals` are `pure` here (they are constants) whereas `IERC20` declares them `view` — this mismatch is why the Pair cannot inherit both. |
| `v2-core/contracts/interfaces/IUniswapV2Factory.sol` | 17 | `PairCreated` event, `feeTo`, `feeToSetter`, `getPair`, `allPairs`, `allPairsLength`, `createPair`, `setFeeTo`, `setFeeToSetter`. |
| `v2-core/contracts/interfaces/IUniswapV2Pair.sol` | 52 | The complete pair ABI: all of `IUniswapV2ERC20` (`:4-22`) plus the four AMM events (`:24-34`) and the AMM functions (`:36-51`). This is the interface the periphery imports. |

**Gotcha on `IUniswapV2Pair`:** it declares `initialize(address,address)` at `:51`,
so anyone holding the interface can *attempt* to call it. The Pair's
`msg.sender == factory` check at `Pair:67` is the only thing stopping it.

## 6. Core test helper

**`v2-core/contracts/test/ERC20.sol`** (9 lines). A 3-line contract:
`contract ERC20 is UniswapV2ERC20 { constructor(uint _totalSupply) public { _mint(msg.sender, _totalSupply); } }`.
Used by the core test suite to create a token with permit support. Not deployed
in production.

---

# Part II — Periphery

The periphery is what users and integrators actually call. Everything here is
*optional*: the core works without it. The periphery's job is to add the three
things the core deliberately omits — **deadlines**, **slippage limits**, and
**ETH/WETH wrapping** — plus the multi-hop routing loop.

The periphery is also completely permissionless and stateless. `UniswapV2Router02`
has exactly two storage-free `immutable` values (`factory`, `WETH`) and no owner,
no upgrade path, no pause. It can never hold funds between transactions.

## 7. UniswapV2Library

**File:** `v2-periphery/contracts/libraries/UniswapV2Library.sol` (82 lines)
**Type:** `library`, all functions `internal` — inlined into callers, no `DELEGATECALL`.
**Purpose:** the pure math and address derivation that the Router (and everyone
else) needs. Nothing here touches state.

### 7.1 `sortTokens(address,address)`

```
function sortTokens(address tokenA, address tokenB) internal pure returns (address token0, address token1)
                                                v2-periphery/contracts/libraries/UniswapV2Library.sol:11
```

- **Purpose:** impose the canonical ordering `token0 < token1` used by the Factory's salt.
- **Checks:** `:12` `IDENTICAL_ADDRESSES`; `:14` `ZERO_ADDRESS` (only `token0`, same reasoning as the Factory).
- **Returns:** the two addresses in ascending numeric order.
- **Callers:** `pairFor:19`, `getReserves:30`, `Router01:111`, `Router02:115`, `Router01/02 _swap:172/215`, `_swapSupportingFeeOnTransferTokens:324`, `ExampleSlidingWindowOracle:117`.
- **Why it matters:** the entire system's "which side is token0" question is answered by raw address comparison. `amount0Out`/`amount1Out`, `reserve0`/`reserve1`, and `price0CumulativeLast` are all indexed this way, so every caller must sort before interpreting.

### 7.2 `pairFor(address,address,address)`

```
function pairFor(address factory, address tokenA, address tokenB) internal pure returns (address pair)
                                                v2-periphery/contracts/libraries/UniswapV2Library.sol:18
```

```solidity
(address token0, address token1) = sortTokens(tokenA, tokenB);
pair = address(uint(keccak256(abi.encodePacked(
        hex'ff',
        factory,
        keccak256(abi.encodePacked(token0, token1)),
        hex'96e8ac4277198ff8b6f785478aa9a39f403cb768dd02cbee326c3e7da348845f' // init code hash  :24
    ))));
```

- **Purpose:** compute a pair's address with **zero external calls** — the single biggest gas saving in the periphery.
- **Checks:** inherits `sortTokens`'s two.
- **Returns:** the CREATE2 address, whether or not a contract exists there.
- **Callers:** everywhere. `getReserves:31`, `Router02:72,93,112,151,165,202,218,219,234,248,278,295,325,335,347,371,393`, `ExampleOracleSimple:28`, `ExampleSlidingWindowOracle:70,108`, `ExampleFlashSwap:35`.
- **Gotchas:**
  - **The init code hash at `:24` is the #1 forking bug.** It is `keccak256(type(UniswapV2Pair).creationCode)`. Change one byte of `UniswapV2Pair.sol` — even a comment that affects metadata, or a different compiler version — and every `pairFor` result is wrong. Forks must recompute it.
  - It returns an address even for a pair that does not exist. Calls to it then fail in confusing ways (a call to a non-contract returns success with empty returndata, so `getReserves` would revert on decoding). `_addLiquidity` guards against this by calling `factory.getPair` first (`Router02:42`).
  - `ExampleFlashSwap:35` uses this in reverse as an *authentication* check: `assert(msg.sender == pairFor(factory, token0, token1))` proves the caller is a genuine pair. Every flash-swap callback must do this or anyone can call it directly.

### 7.3 `getReserves(address,address,address)`

```
function getReserves(address factory, address tokenA, address tokenB) internal view returns (uint reserveA, uint reserveB)
                                                v2-periphery/contracts/libraries/UniswapV2Library.sol:29
```

- **Purpose:** fetch reserves and return them ordered to match the caller's `(tokenA, tokenB)` argument order rather than the pair's internal sort order.
- **External calls:** one `IUniswapV2Pair(...).getReserves()`.
- **Callers:** `_addLiquidity` (`Router02:45`), `getAmountsOut:67`, `getAmountsIn:78`, `UniswapV2LiquidityMathLibrary:51`, `ExampleSwapToPrice:45`.
- **Gotcha:** the third return value (`blockTimestampLast`) is discarded. Code needing the timestamp must call the pair directly, as `UniswapV2OracleLibrary:24` does.

### 7.4 `quote(uint,uint,uint)`

```
function quote(uint amountA, uint reserveA, uint reserveB) internal pure returns (uint amountB)
                                                v2-periphery/contracts/libraries/UniswapV2Library.sol:36
```

`amountB = amountA.mul(reserveB) / reserveA;` — `:39`

- **Purpose:** "if I deposit `amountA`, how much B must I deposit to keep the ratio?" **This is for liquidity provision, not swaps.** No fee, no price impact.
- **Checks:** `:37` `INSUFFICIENT_AMOUNT`; `:38` `INSUFFICIENT_LIQUIDITY`.
- **Callers:** `_addLiquidity` at `Router01:46,51` and `Router02:49,54`; exposed publicly as `Router02.quote:403`.
- **Gotcha:** using `quote` to price a *trade* is a classic beginner error — it ignores both the 0.3% fee and the price impact, and it is trivially manipulable since it reads spot reserves.

### 7.5 `getAmountOut(uint,uint,uint)`

```
function getAmountOut(uint amountIn, uint reserveIn, uint reserveOut) internal pure returns (uint amountOut)
                                                v2-periphery/contracts/libraries/UniswapV2Library.sol:43
```

```solidity
uint amountInWithFee = amountIn.mul(997);                      // :46
uint numerator   = amountInWithFee.mul(reserveOut);            // :47
uint denominator = reserveIn.mul(1000).add(amountInWithFee);   // :48
amountOut = numerator / denominator;                           // :49
```

**Derivation.** With fee `f = 0.003`, the effective input is `dx' = dx(1-f)`.
Constant product requires:

```
(x + dx')(y - dy) = xy
=> dy = y*dx' / (x + dx')
      = y * 997*dx / (1000*x + 997*dx)
```

which is exactly `:47-49`. Multiplying through by 1000 keeps integer precision.

- **Checks:** `:44` `INSUFFICIENT_INPUT_AMOUNT`; `:45` `INSUFFICIENT_LIQUIDITY`.
- **Callers:** `getAmountsOut:68`, `Router02._swapSupportingFeeOnTransferTokens:332`, `Router02.getAmountOut:407` (public), `UniswapV2LiquidityMathLibrary:64,68`.
- **Gotchas:**
  - Rounds **down** — the trader gets slightly less, the pool keeps the dust. This is required for the pool's k-check at `Pair:182` to pass.
  - Returns 0 for tiny inputs against large reserves, and `Pair.swap` then reverts with `INSUFFICIENT_OUTPUT_AMOUNT`.
  - `amountOut` is always strictly less than `reserveOut` — the asymptote guarantees a pool can never be drained.

### 7.6 `getAmountIn(uint,uint,uint)`

```
function getAmountIn(uint amountOut, uint reserveIn, uint reserveOut) internal pure returns (uint amountIn)
                                                v2-periphery/contracts/libraries/UniswapV2Library.sol:53
```

```solidity
uint numerator   = reserveIn.mul(amountOut).mul(1000);         // :56
uint denominator = reserveOut.sub(amountOut).mul(997);         // :57
amountIn = (numerator / denominator).add(1);                   // :58
```

**Derivation.** Invert the previous equation for `dx` given `dy`:

```
dy = y*997*dx / (1000x + 997dx)
=> dx = 1000 * x * dy / (997 * (y - dy))
```

then **add 1** at `:58`.

- **Checks:** `:54` `INSUFFICIENT_OUTPUT_AMOUNT`; `:55` `INSUFFICIENT_LIQUIDITY`.
- **Callers:** `getAmountsIn:79`, `Router02.getAmountIn:417` (public), `ExampleFlashSwap:51,61` (via `getAmountsIn`).
- **The `+1` is critical.** Integer division rounds down, which here would round the *required input* down and make the k-check fail by one wei. The `+1` guarantees the swap succeeds. It is a ceiling-division idiom and always rounds against the trader, in the pool's favour.
- **Gotcha:** `reserveOut.sub(amountOut)` reverts with `ds-math-sub-underflow` if you ask for more output than the pool has. Asking for *exactly* `reserveOut` gives division by zero.

### 7.7 `getAmountsOut(address,uint,address[])`

```
function getAmountsOut(address factory, uint amountIn, address[] memory path) internal view returns (uint[] memory amounts)
                                                v2-periphery/contracts/libraries/UniswapV2Library.sol:62
```

```solidity
require(path.length >= 2, 'UniswapV2Library: INVALID_PATH');   // :63
amounts = new uint[](path.length);
amounts[0] = amountIn;                                         // :65
for (uint i; i < path.length - 1; i++) {                       // :66
    (uint reserveIn, uint reserveOut) = getReserves(factory, path[i], path[i + 1]);
    amounts[i + 1] = getAmountOut(amounts[i], reserveIn, reserveOut);
}
```

- **Purpose:** chain `getAmountOut` forward along a path. `amounts[i]` is the amount of `path[i]` at each hop.
- **Direction:** forward, index 0 → last.
- **External calls:** one `getReserves` per hop.
- **Callers:** `Router02:231` (`swapExactTokensForTokens`), `:261`, `:292`; `Router01:186,211,238`; `Router02.getAmountsOut:427`.
- **Gotcha:** compounding rounding-down across hops means a 3-hop route loses slightly more than 3 × 0.3%. Also, all reserves are read *before* any swap executes, so the quoted amounts are only valid if nothing changes in between — which is exactly why `amountOutMin` exists.

### 7.8 `getAmountsIn(address,uint,address[])`

```
function getAmountsIn(address factory, uint amountOut, address[] memory path) internal view returns (uint[] memory amounts)
                                                v2-periphery/contracts/libraries/UniswapV2Library.sol:73
```

Same shape but **backwards**: `amounts[last] = amountOut` (`:76`) and the loop
runs `i = path.length - 1` down to `1` (`:77`), calling `getAmountIn`.

- **Callers:** `Router02:245,275,310`; `Router01:198,224,253`; `ExampleFlashSwap:51,61`.
- **Gotcha:** each hop's `+1` accumulates, so a long exact-output path requires slightly more input than the ideal. This is correct and intentional.

---

## 8. UniswapV2Router02

**File:** `v2-periphery/contracts/UniswapV2Router02.sol` (446 lines)
**Inheritance:** `IUniswapV2Router02` (which extends `IUniswapV2Router01`)
**Purpose:** the canonical user entry point. 24 external/public functions.

Deployed at `0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D` on mainnet — the single
most-called contract in DeFi history.

### 8.1 State, modifier, constructor, receive

```solidity
address public immutable override factory;      // :15
address public immutable override WETH;         // :16

modifier ensure(uint deadline) {                // :18-21
    require(deadline >= block.timestamp, 'UniswapV2Router: EXPIRED');
    _;
}

constructor(address _factory, address _WETH) public { factory = _factory; WETH = _WETH; }  // :23

receive() external payable {
    assert(msg.sender == WETH);                 // :29
}
```

- **`immutable`** means both are baked into bytecode — no `SLOAD`, and no possibility of an admin changing them. The Router has **no owner and no storage**.
- **`ensure(deadline)`** protects against a transaction sitting in the mempool and executing hours later at a stale price. Applied to every state-changing function *except* the four `WithPermit` variants (`:141`, `:156`, `:193`) — those pass the deadline through to the inner call, which does have `ensure`, and to `permit` itself, which checks its own deadline at `UniswapV2ERC20.sol:82`.
- **`receive()`** uses `assert`, not `require`. In 0.6.6 `assert` consumes all remaining gas on failure. This is deliberate: receiving ETH from anything other than WETH means something is deeply wrong. It exists so that `IWETH(WETH).withdraw()` (at `:138`, `:190`, `:281`, `:298`, `:398`) can send ETH back.
- **Gotcha:** because `receive` rejects everything but WETH, you cannot accidentally donate ETH to the Router. Any ETH that does end up there is stuck forever — there is no rescue function.

### 8.2 Add liquidity

#### `_addLiquidity(...)` — `:33-60` (`internal virtual`)

- **Purpose:** compute the optimal `(amountA, amountB)` respecting the pool's current ratio and the caller's maximums and minimums.
- **Params:** `tokenA`, `tokenB`, `amountADesired`/`amountBDesired` (maximums), `amountAMin`/`amountBMin` (slippage floors).
- **Logic:**
  - `:42-44` if the pair does not exist, create it via `factory.createPair`.
  - `:46-47` if reserves are zero (first liquidity), accept both desired amounts as-is — **the first LP sets the price**.
  - `:49` otherwise compute `amountBOptimal = quote(amountADesired, reserveA, reserveB)`.
  - `:50-52` if `amountBOptimal <= amountBDesired`, A is the binding side. Check `amountBOptimal >= amountBMin`, use `(amountADesired, amountBOptimal)`.
  - `:54-57` else B is binding. Compute `amountAOptimal`, `assert(amountAOptimal <= amountADesired)` (`:55` — must hold mathematically), check `amountAOptimal >= amountAMin`, use `(amountAOptimal, amountBDesired)`.
- **Reverts:** `INSUFFICIENT_B_AMOUNT` (`:51`), `INSUFFICIENT_A_AMOUNT` (`:56`).
- **External calls:** `factory.getPair`, possibly `factory.createPair`, then `getReserves`.
- **Gotcha:** the `assert` at `:55` is an invariant check, not input validation — if `amountBOptimal > amountBDesired` then by the symmetry of `quote`, `amountAOptimal <= amountADesired` necessarily. An `assert` failure here would mean a broken pool.

#### `addLiquidity(...)` — `:61-76` (`external`, `ensure`)

```solidity
(amountA, amountB) = _addLiquidity(tokenA, tokenB, amountADesired, amountBDesired, amountAMin, amountBMin);
address pair = UniswapV2Library.pairFor(factory, tokenA, tokenB);         // :72
TransferHelper.safeTransferFrom(tokenA, msg.sender, pair, amountA);       // :73
TransferHelper.safeTransferFrom(tokenB, msg.sender, pair, amountB);       // :74
liquidity = IUniswapV2Pair(pair).mint(to);                                // :75
```

- **Returns:** `(amountA, amountB, liquidity)`.
- **External calls:** `_addLiquidity`'s, then 2 `transferFrom` **direct to the pair** (never via the Router), then `pair.mint(to)`.
- **Note:** tokens go straight from the user to the pair. The Router never custodies them, which is why it needs no rescue function.
- **Prerequisite:** the user must have approved the **Router** for both tokens.
- **Emits (from the pair):** `Transfer`, `Sync`, `Mint`.

#### `addLiquidityETH(...)` — `:77-100` (`external payable`, `ensure`)

Same as above with `WETH` substituted for `tokenB` and `msg.value` for
`amountBDesired` (`:89`). Then:

```solidity
TransferHelper.safeTransferFrom(token, msg.sender, pair, amountToken);  // :94
IWETH(WETH).deposit{value: amountETH}();                                // :95
assert(IWETH(WETH).transfer(pair, amountETH));                          // :96
liquidity = IUniswapV2Pair(pair).mint(to);                              // :97
if (msg.value > amountETH) TransferHelper.safeTransferETH(msg.sender, msg.value - amountETH);  // :99
```

- **Key detail:** the ETH refund at `:99` goes to `msg.sender`, **not** to `to`. Send exactly what you intend.
- **Gotcha:** `assert` on the WETH transfer at `:96` — WETH9 always returns true, so this can only fail if WETH is broken.

### 8.3 Remove liquidity

#### `removeLiquidity(...)` — `:103-119` (`public virtual`, `ensure`)

```solidity
address pair = UniswapV2Library.pairFor(factory, tokenA, tokenB);
IUniswapV2Pair(pair).transferFrom(msg.sender, pair, liquidity);   // :113  LP -> pair
(uint amount0, uint amount1) = IUniswapV2Pair(pair).burn(to);     // :114
(address token0,) = UniswapV2Library.sortTokens(tokenA, tokenB);  // :115
(amountA, amountB) = tokenA == token0 ? (amount0, amount1) : (amount1, amount0);  // :116
require(amountA >= amountAMin, 'UniswapV2Router: INSUFFICIENT_A_AMOUNT');  // :117
require(amountB >= amountBMin, 'UniswapV2Router: INSUFFICIENT_B_AMOUNT');  // :118
```

- **The two-step:** transfer LP tokens *to the pair*, then call `burn`. The pair reads `balanceOf[address(this)]` at `Pair:140`.
- **The sort at `:115-116`** un-permutes the pair's `(amount0, amount1)` back into the caller's `(tokenA, tokenB)` order.
- **`public`, not `external`,** because `removeLiquidityETH:128` and `:180` call it internally.
- **Prerequisite:** user approved the Router for the **LP token**.

#### `removeLiquidityETH(...)` — `:120-140` (`public virtual`, `ensure`)

Calls `removeLiquidity(token, WETH, ..., address(this), deadline)` — note the
recipient is **the Router itself** (`:134`), which then unwraps:

```solidity
TransferHelper.safeTransfer(token, to, amountToken);   // :137
IWETH(WETH).withdraw(amountETH);                        // :138  triggers receive()
TransferHelper.safeTransferETH(to, amountETH);          // :139
```

This is the one place the Router briefly holds funds — within a single transaction.

#### `removeLiquidityWithPermit(...)` — `:141-155` and `removeLiquidityETHWithPermit(...)` — `:156-169`

```solidity
uint value = approveMax ? uint(-1) : liquidity;                       // :152 / :166
IUniswapV2Pair(pair).permit(msg.sender, address(this), value, deadline, v, r, s);  // :153 / :167
```

then delegate to the non-permit version.

- **Purpose:** remove liquidity in one transaction, no prior `approve`.
- **`approveMax`:** if true, the signature authorises infinite allowance (which `transferFrom` then never decrements — see §3.3). Convenient but leaves a standing infinite approval.
- **No `ensure` modifier** on these — the deadline is enforced by `permit` (`UniswapV2ERC20.sol:82`) and by the inner `removeLiquidity`.

### 8.4 Remove liquidity (fee-on-transfer)

#### `removeLiquidityETHSupportingFeeOnTransferTokens(...)` — `:172-192` (`public virtual`, `ensure`)

The only difference from `removeLiquidityETH`:

```solidity
TransferHelper.safeTransfer(token, to, IERC20(token).balanceOf(address(this)));  // :189
```

It sends **whatever the Router actually received** rather than the `amountToken`
the pair reported. With a fee-on-transfer token those differ, and the plain
version would try to send more than it has and revert.

- Note the return signature drops `amountToken` entirely (`:179` returns only `amountETH`) — because the true token amount is unknowable in advance.

#### `removeLiquidityETHWithPermitSupportingFeeOnTransferTokens(...)` — `:193-208`

Permit wrapper around the above. Same `approveMax` pattern.

### 8.5 `_swap` and the six swap functions

#### `_swap(uint[] memory amounts, address[] memory path, address _to)` — `:212-223` (`internal virtual`)

The multi-hop engine. Precondition stated at `:211`: *"requires the initial
amount to have already been sent to the first pair"*.

```solidity
for (uint i; i < path.length - 1; i++) {
    (address input, address output) = (path[i], path[i + 1]);
    (address token0,) = UniswapV2Library.sortTokens(input, output);
    uint amountOut = amounts[i + 1];
    (uint amount0Out, uint amount1Out) = input == token0 ? (uint(0), amountOut) : (amountOut, uint(0));  // :217
    address to = i < path.length - 2 ? UniswapV2Library.pairFor(factory, output, path[i + 2]) : _to;     // :218
    IUniswapV2Pair(UniswapV2Library.pairFor(factory, input, output)).swap(amount0Out, amount1Out, to, new bytes(0));
}
```

**The chaining trick at `:218`** is the elegant part: each hop sends its output
*directly to the next pair*, not back to the Router. Only the final hop sends to
`_to`. An N-hop swap therefore costs N transfers, not 2N.

```
user --USDC--> [pair1] --WETH--> [pair2] --DAI--> user
                  ^                  ^
             swap() sends        swap() sends
             output to pair2     output to _to
```

`new bytes(0)` as the last argument means **empty `data`, so no flash-swap
callback** at `Pair:172`.

#### The six exact-in / exact-out functions

| Function | Line | In | Out | Pre-flight |
|---|---|---|---|---|
| `swapExactTokensForTokens` | `:224` | exact ERC20 | ≥ min ERC20 | `getAmountsOut`, check last ≥ `amountOutMin` (`:232`) |
| `swapTokensForExactTokens` | `:238` | ≤ max ERC20 | exact ERC20 | `getAmountsIn`, check first ≤ `amountInMax` (`:246`) |
| `swapExactETHForTokens` | `:252` | exact ETH (`msg.value`) | ≥ min ERC20 | `path[0] == WETH` (`:260`), then `getAmountsOut` |
| `swapTokensForExactETH` | `:267` | ≤ max ERC20 | exact ETH | `path[last] == WETH` (`:274`), `getAmountsIn` |
| `swapExactTokensForETH` | `:284` | exact ERC20 | ≥ min ETH | `path[last] == WETH` (`:291`), `getAmountsOut` |
| `swapETHForExactTokens` | `:301` | ≤ `msg.value` ETH | exact ERC20 | `path[0] == WETH` (`:309`), `getAmountsIn` |

All six share the same skeleton:

1. `ensure(deadline)`.
2. Compute the full `amounts[]` array with `getAmountsOut` or `getAmountsIn`.
3. Check the slippage bound — `INSUFFICIENT_OUTPUT_AMOUNT` or `EXCESSIVE_INPUT_AMOUNT`.
4. Move `amounts[0]` of `path[0]` to `pairFor(path[0], path[1])` — via `safeTransferFrom` for tokens, or `WETH.deposit` + `WETH.transfer` for ETH.
5. `_swap(amounts, path, recipient)`.
6. For ETH-out variants: `_swap` to `address(this)`, then `WETH.withdraw` + `safeTransferETH`.
7. For ETH-in exact-output: refund `msg.value - amounts[0]` to `msg.sender` (`:316`).

**Returns:** all six return `uint[] memory amounts` — the full per-hop breakdown.

**Gotchas:**
- ETH-out variants route through the Router (`:280`, `:297`) so it can unwrap. Everything else goes pair-to-pair.
- `swapETHForExactTokens` refunds to `msg.sender`, not `to` (`:316`).
- The `INVALID_PATH` requires only check the WETH *endpoint*. Nothing validates that intermediate hops have liquidity — a bad path reverts deeper, inside `getReserves` or the pair.
- Passing a `path` whose pairs do not exist makes `pairFor` return an address with no code; the resulting call reverts obscurely.

### 8.6 Fee-on-transfer swaps

#### `_swapSupportingFeeOnTransferTokens(address[] memory path, address _to)` — `:321-338`

The key difference: **amounts are computed hop by hop from actual balances**,
because you cannot predict them in advance.

```solidity
(uint reserve0, uint reserve1,) = pair.getReserves();
(uint reserveInput, uint reserveOutput) = input == token0 ? (reserve0, reserve1) : (reserve1, reserve0);
amountInput = IERC20(input).balanceOf(address(pair)).sub(reserveInput);   // :331  MEASURE
amountOutput = UniswapV2Library.getAmountOut(amountInput, reserveInput, reserveOutput);  // :332
```

Line `:331` is the whole idea: ask the pair how much it *actually received*
(`balanceOf - reserve`) instead of trusting a pre-computed number.

#### The three public variants

| Function | Line | Notes |
|---|---|---|
| `swapExactTokensForTokensSupportingFeeOnTransferTokens` | `:339` | Measures `balanceOf(to)` before (`:349`) and after, requires the delta ≥ `amountOutMin` (`:351-354`) |
| `swapExactETHForTokensSupportingFeeOnTransferTokens` | `:356` | `payable`; wraps `msg.value` (`:370`), same before/after check |
| `swapExactTokensForETHSupportingFeeOnTransferTokens` | `:379` | Swaps to `address(this)`, reads `IERC20(WETH).balanceOf(address(this))` (`:396`), unwraps and forwards |

- **All three return nothing** — the amounts are unknowable in advance, so there is no `amounts[]` to return.
- **Only exact-input variants exist.** Exact-output is impossible with fee-on-transfer tokens: you cannot solve for the required input when the transfer function eats an unknown cut.
- **Gotcha — the balance-delta check at `:349`/`:352` is the real slippage guard.** If `to` is a contract whose balance changes for other reasons during the swap, the check is unreliable.
- **Gotcha:** these also work fine for normal tokens, just with a bit more gas. Aggregators often use them universally to avoid classifying tokens.

### 8.7 Library passthroughs

Five `public pure`/`view` wrappers so off-chain callers can use the library
without deploying anything:

| Function | Line | Delegates to |
|---|---|---|
| `quote(uint,uint,uint)` | `:403` | `UniswapV2Library.quote` |
| `getAmountOut(uint,uint,uint)` | `:407` | `UniswapV2Library.getAmountOut` |
| `getAmountIn(uint,uint,uint)` | `:417` | `UniswapV2Library.getAmountIn` ✅ |
| `getAmountsOut(uint,address[])` | `:427` | `UniswapV2Library.getAmountsOut` |
| `getAmountsIn(uint,address[])` | `:437` | `UniswapV2Library.getAmountsIn` |

All are `virtual override`, so a fork can subclass Router02 and change them.

---

## 9. UniswapV2Router01 and the getAmountIn bug

**File:** `v2-periphery/contracts/UniswapV2Router01.sol` (280 lines)
**Status:** deprecated. Documented here because it is still deployed, still
holds allowances, and because its bug is instructive.

### 9.1 What Router01 has

Structurally identical to Router02 for its 16 functions: `_addLiquidity:30`,
`addLiquidity:58`, `addLiquidityETH:74`, `removeLiquidity:99`,
`removeLiquidityETH:116`, `removeLiquidityWithPermit:137`,
`removeLiquidityETHWithPermit:152`, `_swap:169`, and the same six swaps at
`:179`, `:191`, `:203`, `:217`, `:231`, `:245`, plus five library passthroughs at
`:261-279`.

### 9.2 The bug

```solidity
function getAmountIn(uint amountOut, uint reserveIn, uint reserveOut) public pure override returns (uint amountIn) {
    return UniswapV2Library.getAmountOut(amountOut, reserveIn, reserveOut);   // :270  <-- WRONG
}
```

`UniswapV2Router01.sol:270` calls **`getAmountOut`** from a function named
`getAmountIn`. Compare `Router02.sol:424`, which correctly calls
`UniswapV2Library.getAmountIn`.

- **Impact:** anyone calling `Router01.getAmountIn(...)` as a quoting helper gets a
  badly wrong number (it computes the output for a given input, not the input for
  a desired output). The two differ both in formula and in rounding direction.
- **Why it was not catastrophic:** the *swap* functions do not use this method.
  `swapTokensForExactTokens:198` calls `UniswapV2Library.getAmountsIn` directly,
  which uses the correct library `getAmountIn:53`. So actual swaps were always
  right; only the public view helper lied.
- **Lesson:** a thin passthrough is exactly where a copy-paste error hides, because
  it looks too trivial to test.

### 9.3 Router01 vs Router02

| | Router01 | Router02 |
|---|---|---|
| `getAmountIn` helper | **broken** (`:270`) | correct (`:424`) |
| Fee-on-transfer swaps | none | 3 functions (`:339`, `:356`, `:379`) |
| Fee-on-transfer liquidity removal | none | 2 functions (`:172`, `:193`) |
| `virtual` on functions | no | yes — subclassable |
| `_addLiquidity` / `_swap` visibility | `private` (`:30`, `:169`) | `internal virtual` (`:33`, `:212`) |
| ETH dust refund in `addLiquidityETH` | yes (`:95`) | yes (`:99`) |
| Function count | 16 | 24 |

**Migration note:** Router01 and Router02 are separate deployments with separate
allowances. A user who approved Router01 must approve Router02 again.

---

## 10. UniswapV2OracleLibrary

**File:** `v2-periphery/contracts/libraries/UniswapV2OracleLibrary.sol` (35 lines)
**Purpose:** read a pair's price accumulators *as of right now*, even if nobody
has traded this block.

### 10.1 `currentBlockTimestamp()` — `:11-13`

`return uint32(block.timestamp % 2 ** 32);` — mirrors `Pair:75` exactly so the
two agree on what "now" means.

### 10.2 `currentCumulativePrices(address pair)` — `:16-34`

```solidity
blockTimestamp = currentBlockTimestamp();
price0Cumulative = IUniswapV2Pair(pair).price0CumulativeLast();
price1Cumulative = IUniswapV2Pair(pair).price1CumulativeLast();
(uint112 reserve0, uint112 reserve1, uint32 blockTimestampLast) = IUniswapV2Pair(pair).getReserves();
if (blockTimestampLast != blockTimestamp) {
    uint32 timeElapsed = blockTimestamp - blockTimestampLast;     // :27  overflow desired
    price0Cumulative += uint(FixedPoint.fraction(reserve1, reserve0)._x) * timeElapsed;  // :30
    price1Cumulative += uint(FixedPoint.fraction(reserve0, reserve1)._x) * timeElapsed;  // :32
}
```

- **Purpose:** the "counterfactual" — extend the stored accumulator to the current timestamp using current reserves, exactly as `_update` would have.
- **Returns:** `(price0Cumulative, price1Cumulative, blockTimestamp)`.
- **External calls:** 3 view calls to the pair.
- **Callers:** `ExampleOracleSimple:42`, `ExampleSlidingWindowOracle:84,116`.
- **Why it exists:** without it, a consumer would have to call `pair.sync()` (a state-changing transaction) before reading, or accept an accumulator that is stale by up to one block.
- **Gotcha:** `FixedPoint.fraction` comes from `@uniswap/lib`, an external npm dependency not vendored in this repo. It produces the same `UQ112x112` encoding as `UQ112x112.encode(...).uqdiv(...)` at `Pair:79-80`.
- **Security:** this returns *cumulative* values. Reading it twice and dividing by the elapsed time gives a TWAP. Reading it once tells you nothing useful.

---

## 11. UniswapV2LiquidityMathLibrary

**File:** `v2-periphery/contracts/libraries/UniswapV2LiquidityMathLibrary.sol` (139 lines)
**Purpose:** value an LP position correctly, including the pending protocol fee,
and compute the trade that moves a pool to a given price. This is the library you
want if you are building a lending protocol that accepts LP tokens as collateral.

### 11.1 `computeProfitMaximizingTrade(...)` — `:17-40`

```solidity
aToB = FullMath.mulDiv(reserveA, truePriceTokenB, reserveB) < truePriceTokenA;   // :23
uint256 invariant = reserveA.mul(reserveB);
uint256 leftSide = Babylonian.sqrt(FullMath.mulDiv(
        invariant.mul(1000),
        aToB ? truePriceTokenA : truePriceTokenB,
        (aToB ? truePriceTokenB : truePriceTokenA).mul(997)));   // :27-33
uint256 rightSide = (aToB ? reserveA.mul(1000) : reserveB.mul(1000)) / 997;      // :34
if (leftSide < rightSide) return (false, 0);
amountIn = leftSide.sub(rightSide);                                              // :39
```

**Derivation.** To move the pool to price `p = truePriceA/truePriceB`, you need
post-trade reserves satisfying `x'y' = k` and `y'/x' = p`, giving
`x' = sqrt(k/p)`. Accounting for the 0.3% fee (the input is scaled by 1000/997)
gives `amountIn = sqrt(k * 1000 * pA / (997 * pB)) - x*1000/997`.

- **Returns:** `(aToB, amountIn)` — direction and size. `(false, 0)` when already at or past the target.
- **Callers:** `getReservesAfterArbitrage:56`, `ExampleSwapToPrice:46`.
- **Gotcha:** `FullMath.mulDiv` and `Babylonian.sqrt` come from `@uniswap/lib`. `mulDiv` computes `a*b/c` with a 512-bit intermediate, avoiding overflow that plain `a.mul(b)/c` would hit.

### 11.2 `getReservesAfterArbitrage(...)` — `:43-72`

Fetches current reserves, computes the profit-maximizing trade, and applies it
in memory to return the post-arbitrage reserves.

- **Checks:** `:53` `'UniswapV2ArbitrageLibrary: ZERO_PAIR_RESERVES'`.
- **Purpose:** answer "what would this pool look like if the market were efficient?" — the manipulation-resistant baseline for valuing LP tokens.

### 11.3 `computeLiquidityValue(...)` — `:75-95`

```solidity
if (feeOn && kLast > 0) {
    uint rootK = Babylonian.sqrt(reservesA.mul(reservesB));
    uint rootKLast = Babylonian.sqrt(kLast);
    if (rootK > rootKLast) {
        uint feeLiquidity = FullMath.mulDiv(totalSupply, rootK.sub(rootKLast), rootK.mul(5).add(rootKLast));
        totalSupply = totalSupply.add(feeLiquidity);      // :91
    }
}
return (reservesA.mul(liquidityAmount) / totalSupply, reservesB.mul(liquidityAmount) / totalSupply);  // :94
```

This **replicates `Pair._mintFee`'s formula** (`Pair:98-100`) in a view context.
Without it, your valuation would be too high, because the next `mint` or `burn`
will dilute you by `feeLiquidity`.

### 11.4 `getLiquidityValue(...)` — `:100-112`

Reads live reserves, `feeTo`, `kLast`, `totalSupply` and calls
`computeLiquidityValue`.

- **The source's own warning at `:98-99`:** *"note this is subject to manipulation, e.g. sandwich attacks"*. Spot reserves can be flash-loaned in either direction, so an attacker can make an LP position appear worth much more or less within one transaction.

### 11.5 `getLiquidityValueAfterArbitrageToPrice(...)` — `:116-138`

The safe version. Takes an externally-supplied true price, computes the reserves
after arbitrage to that price, and values the position there.

- **Checks:** `:133` `require(totalSupply >= liquidityAmount && liquidityAmount > 0, 'ComputeLiquidityValue: LIQUIDITY_AMOUNT')` — the comment notes this also implicitly checks `totalSupply > 0`.
- **Why it is manipulation-resistant:** an attacker who skews the reserves also creates an arbitrage opportunity, and this function prices the position *after* that arbitrage is taken. The result depends on the true price you supply, not on the current reserves. **Your oracle for `truePriceTokenA/B` is now the trust assumption** — use a Chainlink feed or a long TWAP, never spot.

---

## 12. UniswapV2Migrator

**File:** `v2-periphery/contracts/UniswapV2Migrator.sol` (49 lines)
**Purpose:** move liquidity from a Uniswap **V1** exchange to a V2 pair in one
transaction. A historical artifact from the 2020 migration.

```solidity
IUniswapV1Factory immutable factoryV1;   // :12
IUniswapV2Router01 immutable router;      // :13
receive() external payable {}             // :22  accepts ETH from anyone
```

**`migrate(address token, uint amountTokenMin, uint amountETHMin, address to, uint deadline)`** — `:24-48`:

1. `:28` look up the V1 exchange for `token`.
2. `:29-30` pull the user's *entire* V1 LP balance: `require(exchangeV1.transferFrom(msg.sender, address(this), liquidityV1), 'TRANSFER_FROM_FAILED')`.
3. `:31` `exchangeV1.removeLiquidity(liquidityV1, 1, 1, uint(-1))` — **min amounts of 1 and an infinite deadline**, i.e. no slippage protection on the V1 side.
4. `:32` approve the Router for the recovered tokens.
5. `:33-40` `router.addLiquidityETH{value: amountETHV1}(...)` with the user's real min amounts.
6. `:41-47` refund whichever side was not fully consumed, to `msg.sender`.

- **Gotchas:**
  - `receive()` at `:22` accepts ETH from **anyone** — the source comment explains why (checking the V1 factory would cost too much gas). Any ETH sent here outside a migration is permanently stuck.
  - It migrates the user's *whole* V1 position (`:29` reads `balanceOf`), not a specified amount.
  - `:42` resets the token allowance to 0 after a partial spend — "be a good blockchain citizen".
  - The `else if` at `:44` is safe because `addLiquidityETH` always fully consumes at least one side (see `_addLiquidity:46-58`).
  - Slippage on the V1 withdrawal is unprotected (`1, 1`), but V1 `removeLiquidity` is proportional and cannot be sandwiched meaningfully.

---

## 13. Example contracts

Five contracts in `v2-periphery/contracts/examples/`. These are *reference
implementations* — not deployed as protocol infrastructure — and they are the
best teaching material in the repo.

### 13.1 ExampleOracleSimple

**File:** `examples/ExampleOracleSimple.sol` (67 lines)
**Use case:** "I want a manipulation-resistant price for one pair, updated daily."

```solidity
uint public constant PERIOD = 24 hours;     // :15
IUniswapV2Pair immutable pair;              // :17
uint public price0CumulativeLast;           // :21
uint32 public blockTimestampLast;           // :23
FixedPoint.uq112x112 public price0Average;  // :24
```

**`constructor(address factory, address tokenA, address tokenB)`** — `:27-38`.
Resolves the pair via `pairFor:28`, snapshots both accumulators (`:32-33`) and
the timestamp, and requires non-zero reserves: `:37` `'ExampleOracleSimple: NO_RESERVES'`.

**`update()`** — `:40-56` (`external`, anyone may call):
- `:42` read counterfactual cumulatives.
- `:46` `require(timeElapsed >= PERIOD, 'ExampleOracleSimple: PERIOD_NOT_ELAPSED')`.
- `:50-51` `priceAverage = (cumNow - cumLast) / timeElapsed` — **this is the TWAP**, one division.
- `:53-55` store the new snapshot.

**`consult(address token, uint amountIn) external view returns (uint amountOut)`** — `:59-65`:
- `:61` if `token == token0`, use `price0Average`; `:63` else require `token == token1` (`'ExampleOracleSimple: INVALID_TOKEN'`).
- Returns `priceAverage.mul(amountIn).decode144()`.

**Gotchas (the header comment at `:10-11` states them):**
- Returns **0** before the first successful `update()`. A consumer that does not check will read a zero price.
- The window is "at least `PERIOD`, possibly much longer" — if nobody calls `update` for a week, you get a week-long average, which may be badly stale.
- Someone must pay gas to call `update()`. There is no incentive built in.

### 13.2 ExampleSlidingWindowOracle

**File:** `examples/ExampleSlidingWindowOracle.sol` (125 lines)
**Use case:** the same, but with a rolling window and a fixed lookback, and as a
**singleton serving all pairs** (`:13-14` comment) rather than one deployment per pair.

```solidity
struct Observation { uint timestamp; uint price0Cumulative; uint price1Cumulative; }  // :19-23
uint public immutable windowSize;    // :27   e.g. 24 hours
uint8 public immutable granularity;  // :35   e.g. 24 buckets
uint public immutable periodSize;    // :37   = windowSize / granularity
mapping(address => Observation[]) public pairObservations;   // :40
```

**`constructor(address factory_, uint windowSize_, uint8 granularity_)`** — `:42-51`:
- `:43` `require(granularity_ > 1, 'SlidingWindowOracle: GRANULARITY')`.
- `:44-47` `require((periodSize = windowSize_ / granularity_) * granularity_ == windowSize_, 'SlidingWindowOracle: WINDOW_NOT_EVENLY_DIVISIBLE')` — note the assignment inside the require.

**`observationIndexOf(uint timestamp) public view returns (uint8)`** — `:54-57`.
`(timestamp / periodSize) % granularity` — a **ring buffer keyed on absolute
time**, so every observer independently agrees which bucket "now" belongs to.

**`getFirstObservationInWindow(address pair) private view`** — `:60-65`. The
oldest bucket is `(currentIndex + 1) % granularity` — the one about to be
overwritten.

**`update(address tokenA, address tokenB) external`** — `:69-89`:
- `:73-75` lazily push empty `Observation`s until the array has `granularity` entries.
- `:82-83` only write if `timeElapsed > periodSize` — one write per bucket per cycle.

**`computeAmountOut(...) private pure`** — `:93-102`. The same division as the simple oracle.

**`consult(address tokenIn, uint amountIn, address tokenOut) external view`** — `:107-123`:
- `:112` `require(timeElapsed <= windowSize, 'SlidingWindowOracle: MISSING_HISTORICAL_OBSERVATION')` — reverts if `update` was not called often enough.
- `:114` `require(timeElapsed >= windowSize - periodSize * 2, 'SlidingWindowOracle: UNEXPECTED_TIME_ELAPSED')` — the source calls this "should never happen".
- `:119-122` sort to pick which cumulative to use.

**Trade-off vs. the simple oracle:** finer granularity means a tighter, fresher
window but requires more frequent `update` calls. The effective averaging window
is `[windowSize - 2*periodSize, windowSize]` (`:30-34`), not exactly `windowSize`.

### 13.3 ExampleFlashSwap

**File:** `examples/ExampleFlashSwap.sol` (67 lines)
**Use case:** arbitrage V2 against V1 **with zero capital**. The canonical
`IUniswapV2Callee` implementation.

**`uniswapV2Call(address sender, uint amount0, uint amount1, bytes calldata data)`** — `:28-66`.

The security check first, at `:33-35`:

```solidity
address token0 = IUniswapV2Pair(msg.sender).token0();
address token1 = IUniswapV2Pair(msg.sender).token1();
assert(msg.sender == UniswapV2Library.pairFor(factory, token0, token1));  // :35
```

**This line is the entire security model of a flash-swap borrower.** Without it,
anyone could call `uniswapV2Call` directly with fabricated arguments and drain
whatever the contract holds. Any integration implementing this callback must do
the same.

Then the strategy (`:47-65`), token→ETH direction:
1. `:48` decode the caller's slippage parameter from `data`.
2. `:49-50` approve V1 and sell the borrowed tokens for ETH.
3. `:51` `amountRequired = UniswapV2Library.getAmountsIn(factory, amountToken, path)[0]` — what V2 must be repaid.
4. `:52` `assert(amountReceived > amountRequired)` — revert (unwinding everything) if unprofitable.
5. `:53-54` wrap exactly `amountRequired` and send it back to `msg.sender` (the pair).
6. `:55` forward the profit to `sender` — **the original initiator**, which is why `Pair:172` passes `msg.sender` into the callback.

The ETH→token branch (`:57-65`) mirrors it.

**Gotchas:**
- `:36` `assert(amount0 == 0 || amount1 == 0)` — unidirectional only.
- Repayment goes to `msg.sender` (the pair), not a stored address. Correct, and it composes.
- Uses `assert` rather than `require` throughout, burning all gas on failure. Modern code would use `require`.
- Nothing is left in the contract between transactions, so it needs no access control on `receive()` (`:25`).

### 13.4 ExampleSwapToPrice

**File:** `examples/ExampleSwapToPrice.sol` (77 lines)
**Use case:** "move this pool to the true market price and keep the profit."

**`swapToPrice(address tokenA, address tokenB, uint truePriceTokenA, uint truePriceTokenB, uint maxSpendTokenA, uint maxSpendTokenB, address to, uint deadline)`** — `:27-76`:

- `:38` `require(truePriceTokenA != 0 && truePriceTokenB != 0, "ExampleSwapToPrice: ZERO_PRICE")`.
- `:40` `require(maxSpendTokenA != 0 || maxSpendTokenB != 0, "ExampleSwapToPrice: ZERO_SPEND")`.
- `:46` compute the trade with `UniswapV2LiquidityMathLibrary.computeProfitMaximizingTrade`.
- `:52` `require(amountIn > 0, 'ExampleSwapToPrice: ZERO_AMOUNT_IN')`.
- `:56-58` cap `amountIn` at the caller's max spend.
- `:62-63` pull the input token and approve the Router.
- `:69-75` `router.swapExactTokensForTokens(amountIn, 0, path, to, deadline)`.

**Gotcha — `amountOutMin` is hardcoded to `0` at `:71`**, with the comment *"we can
skip computing this number because the math is tested"*. That is only true if the
reserves do not change between the computation at `:45` and the swap at `:69` —
which within a single transaction they cannot, unless a hook or callback
intervenes. **Do not copy this pattern into a multi-step transaction.**

### 13.5 ExampleComputeLiquidityValue

**File:** `examples/ExampleComputeLiquidityValue.sol` (90 lines)
**Use case:** an on-chain wrapper exposing `UniswapV2LiquidityMathLibrary` as
`external view` functions, so an off-chain caller or another contract can use
them without linking the library.

| Function | Line | Delegates to |
|---|---|---|
| `getReservesAfterArbitrage(...)` | `:15` | library `:43` |
| `getLiquidityValue(...)` | `:31` | library `:100` |
| `getLiquidityValueAfterArbitrageToPrice(...)` | `:48` | library `:116` |
| `getGasCostOfGetLiquidityValueAfterArbitrageToPrice(...)` | `:69` | measures `gasleft()` before and after (`:78`, `:87`) |

The last one is a benchmarking helper (`:68` comment: *"test function to measure
the gas cost of the above function"*), useful because the arbitrage computation
involves a `sqrt` and a 512-bit `mulDiv`.

---

## 14. Periphery interfaces

| File | Lines | Contents |
|---|---|---|
| `interfaces/IUniswapV2Router01.sol` | 95 | The 16-function Router01 ABI: `factory`/`WETH` (both `pure`), 2 add-liquidity, 4 remove-liquidity, 6 swaps, 5 library helpers |
| `interfaces/IUniswapV2Router02.sol` | 44 | `is IUniswapV2Router01` plus the 5 fee-on-transfer functions (`:6`, `:14`, `:24`, `:31`, `:37`) |
| `interfaces/IWETH.sol` | 7 | Just `deposit()`, `transfer(address,uint)`, `withdraw(uint)` — the minimal surface the Router needs. Notably **no `approve`**, because the Router transfers WETH directly to pairs |
| `interfaces/IERC20.sol` | 17 | Same as the core copy, duplicated so the periphery does not depend on core's file layout |
| `interfaces/IUniswapV2Migrator.sol` | 5 | One function: `migrate(address,uint,uint,address,uint)` |
| `interfaces/V1/IUniswapV1Exchange.sol` | 9 | The five V1 methods needed: `balanceOf`, `transferFrom`, `removeLiquidity`, `tokenToEthSwapInput`, `ethToTokenSwapInput` |
| `interfaces/V1/IUniswapV1Factory.sol` | 5 | One function: `getExchange(address)` |

**Gotcha:** `IUniswapV2Router01` declares `factory()` and `WETH()` as `pure`
(`:4-5`), but the implementations are `immutable` and thus `view`-like. Solidity
permits this because `immutable` reads compile to constants.

## 15. Periphery test helpers

Four contracts in `v2-periphery/contracts/test/`. Not production code, but two of
them are load-bearing for understanding the test suite.

### 15.1 `DeflatingERC20.sol` (97 lines)

A token that **burns 1% on every transfer** — the fixture that proves the
fee-on-transfer Router functions work.

```solidity
function _transfer(address from, address to, uint value) private {
    uint burnAmount = value / 100;                 // :58   1% burn
    _burn(from, burnAmount);
    uint transferAmount = value.sub(burnAmount);
    balanceOf[from] = balanceOf[from].sub(transferAmount);
    balanceOf[to] = balanceOf[to].add(transferAmount);
    emit Transfer(from, to, transferAmount);       // :63   logs the NET amount
}
```

The recipient receives 99% while the sender is debited 100%. Note the `Transfer`
event logs the net amount, which is what a well-behaved fee-on-transfer token
should do — many real ones do not, which breaks naive indexers.

Otherwise a full ERC-20 with permit (`:84`), mirroring `UniswapV2ERC20`.

### 15.2 `ERC20.sol` (94 lines)

A plain test token, "Test Token" / "TT", 18 decimals, with permit. Functionally
identical to `UniswapV2ERC20` but standalone (does not inherit it) so it can use
pragma `=0.6.6`.

### 15.3 `RouterEventEmitter.sol` (97 lines)

A test harness that `delegatecall`s each of the six Router swap functions and
emits the returned `amounts[]` array as an event:

```solidity
event Amounts(uint[] amounts);                                        // :6
(bool success, bytes memory returnData) = router.delegatecall(abi.encodeWithSelector(
    IUniswapV2Router01(router).swapExactTokensForTokens.selector, amountIn, amountOutMin, path, to, deadline));  // :18-20
assert(success);
emit Amounts(abi.decode(returnData, (uint[])));                       // :22
```

**Why `delegatecall`:** return values from an external call are not visible to a
test framework watching events. By `delegatecall`ing, the harness runs the Router
code in its own context and can emit the result. It works because the Router has
no storage — `factory` and `WETH` are `immutable`, baked into the *Router's*
bytecode, so they resolve correctly even under `delegatecall`. **This is an
accidental proof that the Router is stateless.**

Covers `swapExactTokensForTokens:10`, `swapTokensForExactTokens:25`,
`swapExactETHForTokens:40`, `swapTokensForExactETH:54`, `swapExactTokensForETH:69`,
`swapETHForExactTokens:84`.

### 15.4 `WETH9.sol` (755 lines)

The canonical Wrapped Ether contract, vendored for tests. Most of the file is the
GPL license text; the contract is `:18`–~`:75`.

```solidity
string public name = "Wrapped Ether";      // :19
string public symbol = "WETH";             // :20
uint8  public decimals = 18;               // :21
event Deposit(address indexed dst, uint wad);      // :25
event Withdrawal(address indexed src, uint wad);   // :26
function deposit() public payable                  // :34
function withdraw(uint wad) public                 // :38
function totalSupply() public view returns (uint)  // :45   returns address(this).balance
```

Notable: `totalSupply()` at `:45` returns the contract's ETH balance rather than
a stored counter, so it is always exactly backed. WETH9 has no `permit`, which is
why the Router must use `deposit` + `transfer` rather than a signature flow.

---

# Part III — Tables and recipes

## 16. Use-case cookbook

"I want to do X" → the exact function, and the full internal call chain. Every
line number here is verified.

### 16.1 Create a new pair

```
factory.createPair(tokenA, tokenB)                       UniswapV2Factory.sol:23
 |-- sort, 3 requires                                    :24-27
 |-- create2(0, bytecode, salt=keccak(t0,t1))            :31
 |-- pair.initialize(token0, token1)                     :33 -> UniswapV2Pair.sol:66
 |-- getPair[t0][t1] = getPair[t1][t0] = pair            :34-35
 |-- allPairs.push(pair)                                 :36
 `-- emit PairCreated                                    :37
```

Usually you skip this: `_addLiquidity` creates the pair for you (`Router02.sol:42-44`).

### 16.2 Add liquidity to a new or existing pair

**Prerequisite:** `approve(router, amount)` on both tokens.

```
router.addLiquidity(tokenA,tokenB,aDes,bDes,aMin,bMin,to,deadline)   Router02.sol:61
 |-- ensure(deadline)                                                :18
 |-- _addLiquidity(...)                                              :33
 |    |-- factory.getPair -> if 0, factory.createPair                :42-44
 |    |-- UniswapV2Library.getReserves                               :45  -> Library:29
 |    |-- if reserves == 0: use both desired amounts                 :46-47
 |    `-- else: quote() both ways, pick the binding side             :49-58 -> Library:36
 |-- pairFor(factory, tokenA, tokenB)                                :72  -> Library:18
 |-- TransferHelper.safeTransferFrom(tokenA, msg.sender, pair, amtA) :73
 |-- TransferHelper.safeTransferFrom(tokenB, msg.sender, pair, amtB) :74
 `-- pair.mint(to)                                                   :75  -> Pair:110
      |-- balanceOf x2, amount = balance - reserve                   Pair:112-115
      |-- _mintFee(r0, r1)                                           Pair:117 -> Pair:89
      |-- first mint:  sqrt(a0*a1) - 1000, burn 1000 to address(0)   Pair:120-121
      |-- later mints: min(a0*S/r0, a1*S/r1)                         Pair:123
      |-- _mint(to, liquidity)                                       Pair:126
      |-- _update(...)                                               Pair:128 -> Pair:73
      `-- emit Mint                                                  Pair:130
```

**With ETH:** `addLiquidityETH` (`:77`) — send ETH as `msg.value`, get dust
refunded to `msg.sender` at `:99`.

### 16.3 Remove liquidity

**Prerequisite:** `approve(router, liquidity)` on the **pair** (the LP token).

```
router.removeLiquidity(tokenA,tokenB,liq,aMin,bMin,to,deadline)      Router02.sol:103
 |-- ensure(deadline)
 |-- pair.transferFrom(msg.sender, pair, liquidity)                  :113  LP -> pair
 |-- pair.burn(to)                                                   :114 -> Pair:134
 |    |-- liquidity = balanceOf[address(this)]                       Pair:140
 |    |-- _mintFee                                                   Pair:142
 |    |-- amountN = liquidity * balanceN / totalSupply               Pair:144-145
 |    |-- _burn, 2x _safeTransfer                                    Pair:147-149
 |    |-- re-read balances, _update                                  Pair:150-153
 |    `-- emit Burn                                                  Pair:155
 |-- sortTokens to un-permute amount0/amount1                        :115-116
 `-- 2 slippage requires                                             :117-118
```

**One transaction, no prior approve:** `removeLiquidityWithPermit` (`:141`) — sign
an EIP-2612 permit off-chain; it calls `pair.permit` at `:153` then the above.

**Fee-on-transfer token:** `removeLiquidityETHSupportingFeeOnTransferTokens`
(`:172`), which sends the Router's actual balance at `:189`.

### 16.4 Swap exact input for as much output as possible

```
router.swapExactTokensForTokens(amountIn, amountOutMin, path, to, deadline)   Router02.sol:224
 |-- ensure(deadline)
 |-- amounts = UniswapV2Library.getAmountsOut(factory, amountIn, path)        :231 -> Library:62
 |    `-- per hop: getReserves + getAmountOut(997/1000 formula)               Library:67-68
 |-- require(amounts[last] >= amountOutMin, 'INSUFFICIENT_OUTPUT_AMOUNT')     :232
 |-- safeTransferFrom(path[0], msg.sender, pairFor(path[0],path[1]), amounts[0])  :233
 `-- _swap(amounts, path, to)                                                :236 -> :212
      `-- per hop: pair.swap(a0Out, a1Out, nextPairOrTo, "")                 :219 -> Pair:159
```

**ETH in:** `swapExactETHForTokens` (`:252`) — `path[0]` must be WETH.
**ETH out:** `swapExactTokensForETH` (`:284`) — `path[last]` must be WETH; swaps to
the Router, then `WETH.withdraw` + `safeTransferETH` (`:298-299`).

### 16.5 Swap for an exact output, spending as little as possible

```
router.swapTokensForExactTokens(amountOut, amountInMax, path, to, deadline)   Router02.sol:238
 |-- amounts = getAmountsIn(factory, amountOut, path)                         :245 -> Library:73
 |    `-- BACKWARDS per hop: getAmountIn with the +1 ceiling                  Library:79 -> :53
 |-- require(amounts[0] <= amountInMax, 'EXCESSIVE_INPUT_AMOUNT')             :246
 |-- safeTransferFrom(path[0], msg.sender, firstPair, amounts[0])             :247
 `-- _swap(amounts, path, to)                                                 :250
```

**ETH in:** `swapETHForExactTokens` (`:301`) — refunds `msg.value - amounts[0]` to
`msg.sender` at `:316`.

### 16.6 Multi-hop swap

Identical to the above — just pass a longer `path`:
`[USDC, WETH, DAI]` routes USDC→WETH→DAI. The loop in `_swap` (`:213`) handles any
length, and `:218` sends each hop's output straight to the next pair. Cost is one
`swap` call per hop, not per hop round-trip.

`getAmountsOut` requires `path.length >= 2` (`Library:63`). There is no upper
bound other than gas.

### 16.7 Flash swap (borrow with zero capital)

**You must write a contract implementing `IUniswapV2Callee`.**

```
yourContract.startArb()
 `-- pair.swap(amount0Out, amount1Out, address(this), abi.encode(yourParams))  -> Pair:159
      |-- _safeTransfer(token, you, amountOut)   TOKENS ARRIVE FIRST           Pair:170-171
      |-- IUniswapV2Callee(you).uniswapV2Call(msg.sender, a0, a1, data)        Pair:172
      |    `-- YOUR CODE RUNS HERE
      |         |-- assert(msg.sender == pairFor(factory, token0, token1))  <- MANDATORY
      |         |-- do whatever (trade elsewhere, liquidate, ...)
      |         `-- transfer repayment back to msg.sender (the pair)
      |-- balanceOf x2                                                         Pair:173-174
      |-- amountIn = balance - (reserve - amountOut)                           Pair:176-177
      `-- require(adjusted k-check)                                            Pair:182
```

Reference implementation: `examples/ExampleFlashSwap.sol:28`. Two ways to repay:
send the *other* token (a normal swap paid late) or the *same* token plus 0.3%
(a true flash loan). Compute the repayment with
`UniswapV2Library.getAmountsIn(...)[0]` as at `ExampleFlashSwap.sol:51`.

### 16.8 Swap a fee-on-transfer token

```
router.swapExactTokensForTokensSupportingFeeOnTransferTokens(amountIn, amountOutMin, path, to, deadline)
                                                                    Router02.sol:339
 |-- safeTransferFrom(path[0], msg.sender, firstPair, amountIn)      :346
 |-- balanceBefore = IERC20(path[last]).balanceOf(to)                :349
 |-- _swapSupportingFeeOnTransferTokens(path, to)                    :350 -> :321
 |    `-- per hop: amountInput = pair.balanceOf(input) - reserveIn   :331   MEASURED
 |                 amountOutput = getAmountOut(measured, ...)        :332
 `-- require(balanceOf(to) - balanceBefore >= amountOutMin)          :351-354
```

Returns nothing. Only exact-input variants exist — exact-output is unsolvable.

### 16.9 Read a manipulation-resistant price (TWAP)

**Do not use `getReserves()` for pricing.** It is spot and flash-loanable.

Minimal recipe (see `examples/ExampleOracleSimple.sol`):

```
t0: (cum0_a, cum1_a, ts_a) = UniswapV2OracleLibrary.currentCumulativePrices(pair)   Library:16
    ... wait at least one period, ideally 30+ minutes ...
t1: (cum0_b, cum1_b, ts_b) = currentCumulativePrices(pair)
    price0Average = (cum0_b - cum0_a) / (ts_b - ts_a)      // UQ112x112
    amountOut     = price0Average.mul(amountIn).decode144()
```

Ready-made: `ExampleOracleSimple.update():40` + `consult():59` for a fixed 24h
window; `ExampleSlidingWindowOracle.update():69` + `consult():107` for a rolling
window across many pairs.

**Why it resists manipulation:** `_update` only advances the accumulator on the
first trade of each block (`Pair:77`). Moving the price and reverting it inside
one block contributes nothing.

### 16.10 Value an LP position (e.g. as collateral)

**Naive and wrong:** `reserveN * liquidity / totalSupply`. It ignores the pending
protocol fee, and reserves are flash-loanable.

**Correct:**

```
UniswapV2LiquidityMathLibrary.getLiquidityValueAfterArbitrageToPrice(
    factory, tokenA, tokenB, truePriceA, truePriceB, liquidityAmount)     Library:116
 |-- read feeTo, kLast, totalSupply                                       :127-130
 |-- require(totalSupply >= liquidityAmount && liquidityAmount > 0)       :133
 |-- getReservesAfterArbitrage(...)                                       :135 -> :43
 |    `-- computeProfitMaximizingTrade + apply it in memory               :56-71
 `-- computeLiquidityValue(...)                                           :137 -> :75
      `-- replicate _mintFee dilution before dividing                     :83-93
```

Supply `truePriceA/B` from a Chainlink feed or a long TWAP. The quick version,
`getLiquidityValue:100`, carries the source's own sandwich warning at `:98-99`.

### 16.11 Arbitrage a pool back to the true price

```
ExampleSwapToPrice.swapToPrice(tokenA, tokenB, truePriceA, truePriceB,
                               maxSpendA, maxSpendB, to, deadline)     ExampleSwapToPrice.sol:27
 |-- 2 requires on prices and spend                                    :38, :40
 |-- computeProfitMaximizingTrade(...)                                 :46 -> LiquidityMathLibrary:17
 |-- require(amountIn > 0)                                             :52
 |-- cap at maxSpend                                                   :56-58
 |-- pull tokens + approve router                                      :62-63
 `-- router.swapExactTokensForTokens(amountIn, 0, path, to, deadline)  :69
```

Note `amountOutMin = 0` at `:71` — acceptable only because the computation and
the swap are in the same transaction.

### 16.12 Migrate a Uniswap V1 position to V2

```
migrator.migrate(token, amountTokenMin, amountETHMin, to, deadline)    UniswapV2Migrator.sol:24
 |-- exchangeV1 = factoryV1.getExchange(token)                         :28
 |-- pull the user's ENTIRE V1 LP balance                              :29-30
 |-- exchangeV1.removeLiquidity(liq, 1, 1, uint(-1))                   :31
 |-- safeApprove(token, router, amountTokenV1)                         :32
 |-- router.addLiquidityETH{value: amountETHV1}(...)                   :33-40
 `-- refund the unused side to msg.sender                              :41-47
```

### 16.13 Compute a pair address off-chain

```solidity
address pair = address(uint(keccak256(abi.encodePacked(
    hex'ff', FACTORY, keccak256(abi.encodePacked(token0, token1)),
    hex'96e8ac4277198ff8b6f785478aa9a39f403cb768dd02cbee326c3e7da348845f'))));
```

`UniswapV2Library.pairFor:18`. `token0 < token1` must hold. **On a fork, the init
code hash differs** — recompute it as `keccak256(type(UniswapV2Pair).creationCode)`.

### 16.14 Recover tokens sent to a pair by mistake

`pair.skim(yourAddress)` (`Pair:190`) sends `balance - reserve` to any address you
name. It is a public race — bots watch for this. The alternative, `pair.sync()`
(`Pair:198`), gifts the surplus to LPs instead.

---

## 17. Selector / ABI tables

Computed with keccak256 and verified: `PERMIT_TYPEHASH` derived here matches the
hardcoded literal at `UniswapV2ERC20.sol:18` exactly.

### 17.1 UniswapV2Pair (ERC-20 surface, from UniswapV2ERC20)

| Function | Selector | Visibility | Mutability |
|---|---|---|---|
| `name()` | `0x06fdde03` | public | constant→pure |
| `symbol()` | `0x95d89b41` | public | constant→pure |
| `decimals()` | `0x313ce567` | public | constant→pure |
| `totalSupply()` | `0x18160ddd` | public | view |
| `balanceOf(address)` | `0x70a08231` | public | view |
| `allowance(address,address)` | `0xdd62ed3e` | public | view |
| `DOMAIN_SEPARATOR()` | `0x3644e515` | public | view |
| `PERMIT_TYPEHASH()` | `0x30adf81f` | public | pure |
| `nonces(address)` | `0x7ecebe00` | public | view |
| `approve(address,uint256)` | `0x095ea7b3` | external | nonpayable |
| `transfer(address,uint256)` | `0xa9059cbb` | external | nonpayable |
| `transferFrom(address,address,uint256)` | `0x23b872dd` | external | nonpayable |
| `permit(address,address,uint256,uint256,uint8,bytes32,bytes32)` | `0xd505accf` | external | nonpayable |

### 17.2 UniswapV2Pair (AMM surface)

| Function | Selector | Visibility | Mutability | Line |
|---|---|---|---|---|
| `MINIMUM_LIQUIDITY()` | `0xba9a7a56` | public | constant | `:15` |
| `factory()` | `0xc45a0155` | public | view | `:18` |
| `token0()` | `0x0dfe1681` | public | view | `:19` |
| `token1()` | `0xd21220a7` | public | view | `:20` |
| `getReserves()` | `0x0902f1ac` | public | view | `:38` |
| `price0CumulativeLast()` | `0x5909c0d5` | public | view | `:26` |
| `price1CumulativeLast()` | `0x5a3d5493` | public | view | `:27` |
| `kLast()` | `0x7464fc3d` | public | view | `:28` |
| `mint(address)` | `0x6a627842` | external | nonpayable `lock` | `:110` |
| `burn(address)` | `0x89afcb44` | external | nonpayable `lock` | `:134` |
| `swap(uint256,uint256,address,bytes)` | `0x022c0d9f` | external | nonpayable `lock` | `:159` |
| `skim(address)` | `0xbc25cf77` | external | nonpayable `lock` | `:190` |
| `sync()` | `0xfff6cae9` | external | nonpayable `lock` | `:198` |
| `initialize(address,address)` | `0x485cc955` | external | nonpayable, factory-only | `:66` |

Internal/private (no selector): `_safeTransfer` `:44`, `_update` `:73`,
`_mintFee` `:89`; from `UniswapV2ERC20`: `_mint` `:40`, `_burn` `:46`,
`_approve` `:52`, `_transfer` `:57`.

### 17.3 UniswapV2Factory

| Function | Selector | Visibility | Mutability | Line |
|---|---|---|---|---|
| `feeTo()` | `0x017e7e58` | public | view | `:7` |
| `feeToSetter()` | `0x094b7415` | public | view | `:8` |
| `getPair(address,address)` | `0xe6a43905` | public | view | `:10` |
| `allPairs(uint256)` | `0x1e3dd18b` | public | view | `:11` |
| `allPairsLength()` | `0x574f2ba3` | external | view | `:19` |
| `createPair(address,address)` | `0xc9c65396` | external | nonpayable | `:23` |
| `setFeeTo(address)` | `0xf46901ed` | external | nonpayable, `feeToSetter` only | `:40` |
| `setFeeToSetter(address)` | `0xa2e74af6` | external | nonpayable, `feeToSetter` only | `:45` |

### 17.4 Callback

| Function | Selector | Implemented by |
|---|---|---|
| `uniswapV2Call(address,uint256,uint256,bytes)` | `0x10d1e85c` | any flash-swap borrower; e.g. `ExampleFlashSwap.sol:28` |

### 17.5 UniswapV2Router02 (and Router01 where shared)

| Function | Selector | Mutability | R02 line | R01 line |
|---|---|---|---|---|
| `factory()` | `0xc45a0155` | view | `:15` | `:12` |
| `WETH()` | `0xad5c4648` | view | `:16` | `:13` |
| `addLiquidity(address,address,uint256,uint256,uint256,uint256,address,uint256)` | `0xe8e33700` | nonpayable | `:61` | `:58` |
| `addLiquidityETH(address,uint256,uint256,uint256,address,uint256)` | `0xf305d719` | **payable** | `:77` | `:74` |
| `removeLiquidity(address,address,uint256,uint256,uint256,address,uint256)` | `0xbaa2abde` | nonpayable | `:103` | `:99` |
| `removeLiquidityETH(address,uint256,uint256,uint256,address,uint256)` | `0x02751cec` | nonpayable | `:120` | `:116` |
| `removeLiquidityWithPermit(address,address,uint256,uint256,uint256,address,uint256,bool,uint8,bytes32,bytes32)` | `0x2195995c` | nonpayable | `:141` | `:137` |
| `removeLiquidityETHWithPermit(address,uint256,uint256,uint256,address,uint256,bool,uint8,bytes32,bytes32)` | `0xded9382a` | nonpayable | `:156` | `:152` |
| `swapExactTokensForTokens(uint256,uint256,address[],address,uint256)` | `0x38ed1739` | nonpayable | `:224` | `:179` |
| `swapTokensForExactTokens(uint256,uint256,address[],address,uint256)` | `0x8803dbee` | nonpayable | `:238` | `:191` |
| `swapExactETHForTokens(uint256,address[],address,uint256)` | `0x7ff36ab5` | **payable** | `:252` | `:203` |
| `swapTokensForExactETH(uint256,uint256,address[],address,uint256)` | `0x4a25d94a` | nonpayable | `:267` | `:217` |
| `swapExactTokensForETH(uint256,uint256,address[],address,uint256)` | `0x18cbafe5` | nonpayable | `:284` | `:231` |
| `swapETHForExactTokens(uint256,address[],address,uint256)` | `0xfb3bdb41` | **payable** | `:301` | `:245` |
| `quote(uint256,uint256,uint256)` | `0xad615dec` | pure | `:403` | `:261` |
| `getAmountOut(uint256,uint256,uint256)` | `0x054d50d4` | pure | `:407` | `:265` |
| `getAmountIn(uint256,uint256,uint256)` | `0x85f8c259` | pure | `:417` | `:269` ⚠ **buggy** |
| `getAmountsOut(uint256,address[])` | `0xd06ca61f` | view | `:427` | `:273` |
| `getAmountsIn(uint256,address[])` | `0x1f00ca74` | view | `:437` | `:277` |

**Router02 only:**

| Function | Selector | Mutability | Line |
|---|---|---|---|
| `removeLiquidityETHSupportingFeeOnTransferTokens(address,uint256,uint256,uint256,address,uint256)` | `0xaf2979eb` | nonpayable | `:172` |
| `removeLiquidityETHWithPermitSupportingFeeOnTransferTokens(address,uint256,uint256,uint256,address,uint256,bool,uint8,bytes32,bytes32)` | `0x5b0d5984` | nonpayable | `:193` |
| `swapExactTokensForTokensSupportingFeeOnTransferTokens(uint256,uint256,address[],address,uint256)` | `0x5c11d795` | nonpayable | `:339` |
| `swapExactETHForTokensSupportingFeeOnTransferTokens(uint256,address[],address,uint256)` | `0xb6f9de95` | **payable** | `:356` |
| `swapExactTokensForETHSupportingFeeOnTransferTokens(uint256,uint256,address[],address,uint256)` | `0x791ac947` | nonpayable | `:379` |

Plus `receive()` at `:28` (payable, WETH-only) and internal `_addLiquidity:33`,
`_swap:212`, `_swapSupportingFeeOnTransferTokens:321`.

### 17.6 Other

| Contract | Function | Selector |
|---|---|---|
| `UniswapV2Migrator` | `migrate(address,uint256,uint256,address,uint256)` | `0xb7df1d25` |
| `IWETH` | `deposit()` | `0xd0e30db0` |
| `IWETH` | `withdraw(uint256)` | `0x2e1a7d4d` |
| `IWETH` | `transfer(address,uint256)` | `0xa9059cbb` |

---

## 18. Storage layout tables

### 18.1 UniswapV2Pair (including inherited UniswapV2ERC20)

`UniswapV2Pair is UniswapV2ERC20`, so the parent's slots come first.

| Slot | Offset | Type | Name | Declared at |
|---|---|---|---|---|
| 0 | 0 | `uint256` | `totalSupply` | `UniswapV2ERC20.sol:12` |
| 1 | 0 | `mapping(address=>uint256)` | `balanceOf` | `UniswapV2ERC20.sol:13` |
| 2 | 0 | `mapping(address=>mapping(address=>uint256))` | `allowance` | `UniswapV2ERC20.sol:14` |
| 3 | 0 | `bytes32` | `DOMAIN_SEPARATOR` | `UniswapV2ERC20.sol:16` |
| 4 | 0 | `mapping(address=>uint256)` | `nonces` | `UniswapV2ERC20.sol:19` |
| 5 | 0 | `address` | `factory` | `UniswapV2Pair.sol:18` |
| 6 | 0 | `address` | `token0` | `UniswapV2Pair.sol:19` |
| 7 | 0 | `address` | `token1` | `UniswapV2Pair.sol:20` |
| **8** | **0** | `uint112` | `reserve0` | `UniswapV2Pair.sol:22` |
| **8** | **14** | `uint112` | `reserve1` | `UniswapV2Pair.sol:23` |
| **8** | **28** | `uint32` | `blockTimestampLast` | `UniswapV2Pair.sol:24` |
| 9 | 0 | `uint256` | `price0CumulativeLast` | `UniswapV2Pair.sol:26` |
| 10 | 0 | `uint256` | `price1CumulativeLast` | `UniswapV2Pair.sol:27` |
| 11 | 0 | `uint256` | `kLast` | `UniswapV2Pair.sol:28` |
| 12 | 0 | `uint256` | `unlocked` | `UniswapV2Pair.sol:30` |

**Slot 8 is the famous one** — `112+112+32 = 256` bits exactly. One `SLOAD` for
`getReserves()`, one `SSTORE` for `_update`. Offsets are in bytes from the low
end (`reserve0` occupies bytes 0–13, `reserve1` bytes 14–27, the timestamp bytes
28–31).

Constants (`name`, `symbol`, `decimals`, `PERMIT_TYPEHASH`, `MINIMUM_LIQUIDITY`,
`SELECTOR`) live in bytecode and use no storage.

Reading slot 8 directly:

```
raw = eth_getStorageAt(pair, 8)
blockTimestampLast = raw >> 224
reserve1           = (raw >> 112) & (2**112 - 1)
reserve0           = raw & (2**112 - 1)
```

### 18.2 UniswapV2Factory

| Slot | Type | Name | Line |
|---|---|---|---|
| 0 | `address` | `feeTo` | `:7` |
| 1 | `address` | `feeToSetter` | `:8` |
| 2 | `mapping(address=>mapping(address=>address))` | `getPair` | `:10` |
| 3 | `address[]` | `allPairs` | `:11` |

`allPairs` stores its length in slot 3; elements live at `keccak256(3) + i`.

### 18.3 UniswapV2Router01 / Router02

**No storage slots at all.** `factory` and `WETH` are `immutable` (`Router02:15-16`,
`Router01:12-13`) and compiled into the bytecode. This is why
`RouterEventEmitter` can safely `delegatecall` the Router (§15.3), and why the
Router needs no rescue function.

### 18.4 UniswapV2Migrator

**No storage slots.** Both `factoryV1` and `router` are `immutable` (`:12-13`).

### 18.5 Example contracts

| Contract | Slot | Type | Name |
|---|---|---|---|
| `ExampleOracleSimple` | 0 | `uint256` | `price0CumulativeLast` (`:21`) |
| | 1 | `uint256` | `price1CumulativeLast` (`:22`) |
| | 2 (offset 0) | `uint32` | `blockTimestampLast` (`:23`) |
| | 3 | `uint224` | `price0Average` (`:24`) |
| | 4 | `uint224` | `price1Average` (`:25`) |
| `ExampleSlidingWindowOracle` | 0 | `mapping(address=>Observation[])` | `pairObservations` (`:40`) |
| `ExampleSwapToPrice` | — | — | none (both fields `immutable`, `:16-17`) |
| `ExampleComputeLiquidityValue` | — | — | none (`factory` is `immutable`, `:8`) |
| `ExampleFlashSwap` | — | — | none (all three `immutable`, `:13-15`) |

`ExampleOracleSimple`'s `pair`, `token0`, `token1` (`:17-19`) are `immutable`.
Note slot 2 holds only `blockTimestampLast` — a `uint32` followed by a `uint224`
does not pack, because `32 + 224 = 256` bits would fit but Solidity places each
`struct`-typed `FixedPoint.uq112x112` in its own slot.

---

## 19. Events reference

### 19.1 UniswapV2Pair / UniswapV2ERC20

| Event | topic0 | Emitted at | Meaning |
|---|---|---|---|
| `Mint(address indexed sender, uint amount0, uint amount1)` | `0x4c209b5f...21c4f` | `Pair:130` | liquidity added; `sender` is the caller (usually the Router), **not the LP** |
| `Burn(address indexed sender, uint amount0, uint amount1, address indexed to)` | `0xdccd412f...36496` | `Pair:155` | liquidity removed; `to` received the tokens |
| `Swap(address indexed sender, uint amount0In, uint amount1In, uint amount0Out, uint amount1Out, address indexed to)` | `0xd78ad95f...9d822` | `Pair:186` | a trade; all four amounts are logged |
| `Sync(uint112 reserve0, uint112 reserve1)` | `0x1c411e9a...bbad1` | `Pair:85` (inside `_update`) | new reserves, emitted on **every** state change |
| `Transfer(address indexed from, address indexed to, uint value)` | `0xddf252ad...23b3ef` | `ERC20:43,49,60` | LP token movement; `from == 0` is a mint, `to == 0` a burn |
| `Approval(address indexed owner, address indexed spender, uint value)` | `0x8c5be1e5...c3b925` | `ERC20:54` | LP allowance set, including via `permit` |

Full topic0 values:

```
Mint         0x4c209b5fc8ad50758f13e2e1088ba56a560dff690a1c6fef26394f4c03821c4f
Burn         0xdccd412f0b1252819cb1fd330b93224ca42612892bb3f4f789976e6d81936496
Swap         0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822
Sync         0x1c411e9a96e071241c2f21f7726b17ae89e3cab4c78be50e062b03a9fffbbad1
Transfer     0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef
Approval     0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925
PairCreated  0x0d3648bd0f6ba80134a33ba9275ac585d9d315f0ad8355cddefde31afa28d0e9
```

**How an indexer uses these:**

- **`Sync` is the one you want for price history.** It fires on every `mint`, `burn`, `swap` and `sync`, always carrying the post-state reserves. Price = `reserve1/reserve0` adjusted for decimals. You do not need to replay swaps.
- **`Swap` gives volume and direction.** Exactly one of `amount0In`/`amount1In` is non-zero for a normal trade; both non-zero means someone swapped both ways at once. The 0.3% fee is `amountIn * 3 / 1000` and is not logged separately.
- **`sender` in `Mint`/`Burn`/`Swap` is the immediate caller** — usually the Router or an aggregator, so it is useless for attributing activity to end users. Use the transaction's `from`, or `to` for the recipient.
- **`Transfer` from the pair address itself** identifies LP token flows. `from == address(0)` and `value == 1000` in a pair's first-ever event is the `MINIMUM_LIQUIDITY` burn (`Pair:121`).
- **Ordering within one `mint`:** `Transfer` (fee, if on) → `Transfer` (to LP) → `Sync` → `Mint`. Within a `swap`: `Sync` → `Swap`.

### 19.2 UniswapV2Factory

| Event | topic0 | Emitted at |
|---|---|---|
| `PairCreated(address indexed token0, address indexed token1, address pair, uint)` | `0x0d3648bd...28d0e9` | `Factory:37` |

The unnamed fourth field is `allPairs.length` *after* the push — i.e. the 1-based
index of this pair. Subscribe to this to discover every pair ever created;
`token0`/`token1` are indexed so you can filter by token.

### 19.3 WETH9 and test helpers

| Event | Contract | Line |
|---|---|---|
| `Deposit(address indexed dst, uint wad)` | `WETH9` | `:25` |
| `Withdrawal(address indexed src, uint wad)` | `WETH9` | `:26` |
| `Amounts(uint[] amounts)` | `RouterEventEmitter` | `:6` |

**Gotcha — no events on governance.** `setFeeTo` (`Factory:40`) and
`setFeeToSetter` (`:45`) emit nothing. There is no on-chain log when the protocol
fee is switched; you must diff storage or watch for the extra `Transfer` to
`feeTo` on the next `mint`/`burn`.

---

## 20. Revert-string reference

Every `require` string in both repos, with where it fires and what causes it.

### 20.1 Core — `UniswapV2:` prefix

| String | Where | Cause |
|---|---|---|
| `UniswapV2: IDENTICAL_ADDRESSES` | `Factory:24` | `createPair(x, x)` |
| `UniswapV2: ZERO_ADDRESS` | `Factory:26` | either token is `address(0)` |
| `UniswapV2: PAIR_EXISTS` | `Factory:27` | pair already created for this token pair |
| `UniswapV2: FORBIDDEN` | `Factory:41`, `Factory:46` | caller is not `feeToSetter` |
| `UniswapV2: FORBIDDEN` | `Pair:67` | `initialize` called by anyone but the factory |
| `UniswapV2: LOCKED` | `Pair:32` | reentrancy into `mint`/`burn`/`swap`/`skim`/`sync` |
| `UniswapV2: TRANSFER_FAILED` | `Pair:46` | the token's `transfer` reverted or returned `false` |
| `UniswapV2: OVERFLOW` | `Pair:74` | a balance exceeds `2^112 - 1`; pool is bricked until `skim` |
| `UniswapV2: INSUFFICIENT_LIQUIDITY_MINTED` | `Pair:125` | deposit rounds to zero LP tokens (too small, or unbalanced against a huge pool) |
| `UniswapV2: INSUFFICIENT_LIQUIDITY_BURNED` | `Pair:146` | burning LP that redeems to zero on either side |
| `UniswapV2: INSUFFICIENT_OUTPUT_AMOUNT` | `Pair:160` | `swap` called with both outputs zero |
| `UniswapV2: INSUFFICIENT_LIQUIDITY` | `Pair:162` | requested output ≥ the corresponding reserve |
| `UniswapV2: INVALID_TO` | `Pair:169` | `to` is `token0` or `token1` |
| `UniswapV2: INSUFFICIENT_INPUT_AMOUNT` | `Pair:178` | no tokens actually arrived before `swap` |
| `UniswapV2: K` | `Pair:182` | **the invariant check failed** — not enough input for the requested output |
| `UniswapV2: EXPIRED` | `ERC20:82` | `permit` deadline passed |
| `UniswapV2: INVALID_SIGNATURE` | `ERC20:91` | signature does not recover to `owner` |

### 20.2 Core — `ds-math-` prefix (SafeMath)

| String | Where | Common cause |
|---|---|---|
| `ds-math-add-overflow` | `SafeMath:7` | `totalSupply` or a balance overflowing `uint256` (practically unreachable) |
| `ds-math-sub-underflow` | `SafeMath:11` | **transferring/burning more than you hold**; also `mint` when balance < reserve; also `skim` if reserve > balance |
| `ds-math-mul-overflow` | `SafeMath:15` | the k-check product overflowing, or an absurd liquidity amount |

`ds-math-sub-underflow` is by far the most common confusing revert users hit —
it is what an over-large `transferFrom` on the LP token produces.

### 20.3 Periphery — `UniswapV2Router:` prefix

| String | Where | Cause |
|---|---|---|
| `UniswapV2Router: EXPIRED` | `Router02:19`, `Router01:16` | `deadline < block.timestamp` |
| `UniswapV2Router: INSUFFICIENT_A_AMOUNT` | `Router02:56,117`; `Router01:53,113` | optimal A below `amountAMin`, or withdrawal returned less than `amountAMin` |
| `UniswapV2Router: INSUFFICIENT_B_AMOUNT` | `Router02:51,118`; `Router01:48,114` | same for token B |
| `UniswapV2Router: INSUFFICIENT_OUTPUT_AMOUNT` | `Router02:232,262,293,353,376,397`; `Router01:187,212,239` | **slippage** — the swap would return less than `amountOutMin` |
| `UniswapV2Router: EXCESSIVE_INPUT_AMOUNT` | `Router02:246,276,311`; `Router01:199,225,254` | **slippage** — an exact-output swap needs more than `amountInMax` (or `msg.value`) |
| `UniswapV2Router: INVALID_PATH` | `Router02:260,274,291,309,368,391`; `Router01:210,223,237,252` | an ETH function whose path does not start/end with WETH |

### 20.4 Periphery — `UniswapV2Library:` prefix

| String | Where | Cause |
|---|---|---|
| `UniswapV2Library: IDENTICAL_ADDRESSES` | `Library:12` | `sortTokens(x, x)` |
| `UniswapV2Library: ZERO_ADDRESS` | `Library:14` | a token is `address(0)` |
| `UniswapV2Library: INSUFFICIENT_AMOUNT` | `Library:37` | `quote` with `amountA == 0` |
| `UniswapV2Library: INSUFFICIENT_LIQUIDITY` | `Library:38,45,55` | a reserve is zero — usually **the pair does not exist** or has never been funded |
| `UniswapV2Library: INSUFFICIENT_INPUT_AMOUNT` | `Library:44` | `getAmountOut` with `amountIn == 0` |
| `UniswapV2Library: INSUFFICIENT_OUTPUT_AMOUNT` | `Library:54` | `getAmountIn` with `amountOut == 0` |
| `UniswapV2Library: INVALID_PATH` | `Library:63,74` | `path.length < 2` |

### 20.5 Periphery — everything else

| String | Where | Cause |
|---|---|---|
| `UniswapV2ArbitrageLibrary: ZERO_PAIR_RESERVES` | `LiquidityMathLibrary:53` | arbitrage math on an empty pool |
| `ComputeLiquidityValue: LIQUIDITY_AMOUNT` | `LiquidityMathLibrary:133` | `liquidityAmount` is zero or exceeds `totalSupply` |
| `TRANSFER_FROM_FAILED` | `Migrator:30` | V1 LP `transferFrom` returned false |
| `ExampleOracleSimple: NO_RESERVES` | `ExampleOracleSimple:37` | constructing an oracle on an unfunded pair |
| `ExampleOracleSimple: PERIOD_NOT_ELAPSED` | `ExampleOracleSimple:46` | `update()` called before 24h elapsed |
| `ExampleOracleSimple: INVALID_TOKEN` | `ExampleOracleSimple:63` | `consult` with a token not in the pair |
| `SlidingWindowOracle: GRANULARITY` | `ExampleSlidingWindowOracle:43` | `granularity <= 1` |
| `SlidingWindowOracle: WINDOW_NOT_EVENLY_DIVISIBLE` | `ExampleSlidingWindowOracle:46` | `windowSize % granularity != 0` |
| `SlidingWindowOracle: MISSING_HISTORICAL_OBSERVATION` | `ExampleSlidingWindowOracle:112` | nobody called `update()` recently enough |
| `SlidingWindowOracle: UNEXPECTED_TIME_ELAPSED` | `ExampleSlidingWindowOracle:114` | "should never happen" |
| `ExampleSwapToPrice: ZERO_PRICE` | `ExampleSwapToPrice:38` | either true-price component is zero |
| `ExampleSwapToPrice: ZERO_SPEND` | `ExampleSwapToPrice:40` | both max-spend values are zero |
| `ExampleSwapToPrice: ZERO_AMOUNT_IN` | `ExampleSwapToPrice:52` | pool is already at (or past) the target price |

### 20.6 Failures with NO message

These revert without a reason string, which makes them harder to diagnose:

- **`assert` failures** consume all gas: `Router02:29` (`receive` from non-WETH), `:55` (`_addLiquidity` invariant), `:96`/`:264`/`:313`/`:371` (WETH transfer returned false), and every `assert` in `ExampleFlashSwap` (`:35`, `:36`, `:43`, `:52`, `:54`, `:56`, `:62`, `:63`, `:64`).
- **Calling a pair that does not exist.** `pairFor` returns an address regardless; the subsequent `getReserves()` call to an address with no code returns empty data and reverts during ABI decoding, with no message.
- **`TransferHelper` failures** come from `@uniswap/lib` (not vendored here) and carry their own strings such as `TransferHelper::safeTransferFrom: transferFrom failed`.

---

## 21. File inventory

All 35 Solidity files, every one covered above.

### v2-core (12 files, 527 lines)

| # | File | Lines | Section |
|---|---|---|---|
| 1 | `contracts/UniswapV2Factory.sol` | 49 | [§1](#1-uniswapv2factory) |
| 2 | `contracts/UniswapV2Pair.sol` | 201 | [§2](#2-uniswapv2pair) |
| 3 | `contracts/UniswapV2ERC20.sol` | 94 | [§3](#3-uniswapv2erc20) |
| 4 | `contracts/libraries/Math.sol` | 23 | [§4.1](#41-math) |
| 5 | `contracts/libraries/SafeMath.sol` | 17 | [§4.2](#42-safemath) |
| 6 | `contracts/libraries/UQ112x112.sol` | 20 | [§4.3](#43-uq112x112) |
| 7 | `contracts/interfaces/IERC20.sol` | 17 | [§5](#5-core-interfaces) |
| 8 | `contracts/interfaces/IUniswapV2Callee.sol` | 5 | [§5](#5-core-interfaces) |
| 9 | `contracts/interfaces/IUniswapV2ERC20.sol` | 23 | [§5](#5-core-interfaces) |
| 10 | `contracts/interfaces/IUniswapV2Factory.sol` | 17 | [§5](#5-core-interfaces) |
| 11 | `contracts/interfaces/IUniswapV2Pair.sol` | 52 | [§5](#5-core-interfaces) |
| 12 | `contracts/test/ERC20.sol` | 9 | [§6](#6-core-test-helper) |

### v2-periphery (23 files, 2699 lines)

| # | File | Lines | Section |
|---|---|---|---|
| 13 | `contracts/UniswapV2Router02.sol` | 446 | [§8](#8-uniswapv2router02) |
| 14 | `contracts/UniswapV2Router01.sol` | 280 | [§9](#9-uniswapv2router01-and-the-getamountin-bug) |
| 15 | `contracts/UniswapV2Migrator.sol` | 49 | [§12](#12-uniswapv2migrator) |
| 16 | `contracts/libraries/UniswapV2Library.sol` | 82 | [§7](#7-uniswapv2library) |
| 17 | `contracts/libraries/UniswapV2OracleLibrary.sol` | 35 | [§10](#10-uniswapv2oraclelibrary) |
| 18 | `contracts/libraries/UniswapV2LiquidityMathLibrary.sol` | 139 | [§11](#11-uniswapv2liquiditymathlibrary) |
| 19 | `contracts/libraries/SafeMath.sol` | 17 | [§4.2](#42-safemath) |
| 20 | `contracts/examples/ExampleOracleSimple.sol` | 67 | [§13.1](#131-exampleoraclesimple) |
| 21 | `contracts/examples/ExampleSlidingWindowOracle.sol` | 125 | [§13.2](#132-exampleslidingwindoworacle) |
| 22 | `contracts/examples/ExampleFlashSwap.sol` | 67 | [§13.3](#133-exampleflashswap) |
| 23 | `contracts/examples/ExampleSwapToPrice.sol` | 77 | [§13.4](#134-exampleswaptoprice) |
| 24 | `contracts/examples/ExampleComputeLiquidityValue.sol` | 90 | [§13.5](#135-examplecomputeliquidityvalue) |
| 25 | `contracts/interfaces/IUniswapV2Router01.sol` | 95 | [§14](#14-periphery-interfaces) |
| 26 | `contracts/interfaces/IUniswapV2Router02.sol` | 44 | [§14](#14-periphery-interfaces) |
| 27 | `contracts/interfaces/IERC20.sol` | 17 | [§14](#14-periphery-interfaces) |
| 28 | `contracts/interfaces/IWETH.sol` | 7 | [§14](#14-periphery-interfaces) |
| 29 | `contracts/interfaces/IUniswapV2Migrator.sol` | 5 | [§14](#14-periphery-interfaces) |
| 30 | `contracts/interfaces/V1/IUniswapV1Exchange.sol` | 9 | [§14](#14-periphery-interfaces) |
| 31 | `contracts/interfaces/V1/IUniswapV1Factory.sol` | 5 | [§14](#14-periphery-interfaces) |
| 32 | `contracts/test/DeflatingERC20.sol` | 97 | [§15.1](#151-deflatingerc20sol-97-lines) |
| 33 | `contracts/test/ERC20.sol` | 94 | [§15.2](#152-erc20sol-94-lines) |
| 34 | `contracts/test/RouterEventEmitter.sol` | 97 | [§15.3](#153-routereventemittersol-97-lines) |
| 35 | `contracts/test/WETH9.sol` | 755 | [§15.4](#154-weth9sol-755-lines) |

### External dependencies (not vendored here)

Imported from npm, so not in this tree but referenced by the code:

| Package | Used for | Imported at |
|---|---|---|
| `@uniswap/lib/contracts/libraries/TransferHelper.sol` | `safeTransfer`, `safeTransferFrom`, `safeApprove`, `safeTransferETH` | `Router01:4`, `Router02:4`, `Migrator:3`, `ExampleSwapToPrice:5` |
| `@uniswap/lib/contracts/libraries/FixedPoint.sol` | `uq112x112` type, `fraction`, `mul`, `decode144` | `UniswapV2OracleLibrary:4`, `ExampleOracleSimple:5`, `ExampleSlidingWindowOracle:5` |
| `@uniswap/lib/contracts/libraries/Babylonian.sol` | `sqrt` (the 0.6.x port of core's `Math.sqrt`) | `UniswapV2LiquidityMathLibrary:5`, `ExampleSwapToPrice:4` |
| `@uniswap/lib/contracts/libraries/FullMath.sol` | `mulDiv` — `a*b/c` with 512-bit intermediate | `UniswapV2LiquidityMathLibrary:6` |
| `@uniswap/v2-core/contracts/interfaces/*` | the core interfaces, consumed by the periphery | `Router01:3`, `Router02:3`, `UniswapV2Library:3`, and others |

---

## Closing notes

Three facts worth carrying away:

1. **The Pair trusts nothing.** Every entry point measures `balanceOf` itself and
   enforces the invariant afterwards. That single decision produces the
   "transfer first, then call" calling convention, makes flash swaps free, and
   is why the Router can be stateless and ownerless.

2. **The whole design is downstream of one storage slot.** `reserve0`,
   `reserve1` and `blockTimestampLast` packed into slot 8 forces `uint112`
   reserves, which forces the `uint32` wrapping timestamp, which forces the
   `UQ112x112` fixed-point format, which shapes the oracle.

3. **Rounding always favours the pool.** `getAmountOut` floors, `getAmountIn`
   adds one, `mint` takes the `min`, `sqrt` truncates. Every one of these is
   load-bearing; reversing any single one opens a wei-by-wei drain.

For the conceptual treatment and the comparison against V3 and V4, see
[`UNISWAP-DEEP-DIVE.md`](UNISWAP-DEEP-DIVE.md).
