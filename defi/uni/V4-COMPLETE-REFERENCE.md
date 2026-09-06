# Uniswap V4 Core — Complete Reference

Exhaustive, function-by-function documentation of every Solidity file in
`uni/v4-core/src/`. **84 files**: 46 core (7 root contracts, 24 libraries, 7
types, 8 interfaces) and 38 test helpers.

Every `file:line` below was read and verified with `grep -n` / `sed -n` against
this tree. Every selector was recomputed with `cast sig` rather than copied.
Every transient-storage slot constant was recomputed with keccak256 and matched
against the source.

Paths are relative to `uni/v4-core/`. Solidity `0.8.26`.

For the *conceptual* treatment of V4 (why singleton, why flash accounting, how it
compares to V2/V3), read [`UNISWAP-DEEP-DIVE.md`](UNISWAP-DEEP-DIVE.md) §3. This
document assumes you want completeness instead.

---

## Table of contents

- [0. File inventory](#0-file-inventory)
- [1. Architecture and the unlock lifecycle](#1-architecture-and-the-unlock-lifecycle)
- [2. The type layer](#2-the-type-layer)
  - [2.1 `Currency`](#21-currency--typescurrencysol)
  - [2.2 `PoolKey`](#22-poolkey--typespoolkeysol)
  - [2.3 `PoolId`](#23-poolid--typespoolidsol)
  - [2.4 `PoolOperation`](#24-pooloperation--typespooloperationsol)
  - [2.5 `BalanceDelta`](#25-balancedelta--typesbalancedeltasol)
  - [2.6 `BeforeSwapDelta`](#26-beforeswapdelta--typesbeforeswapdeltasol)
  - [2.7 `Slot0`](#27-slot0--typesslot0sol)
- [3. Flash accounting: the transient-storage layer](#3-flash-accounting-the-transient-storage-layer)
- [4. `PoolManager`](#4-poolmanager)
- [5. `Pool` library](#5-pool-library)
- [6. Hooks](#6-hooks)
- [7. Math libraries](#7-math-libraries)
- [8. Fees](#8-fees)
- [9. ERC-6909 claims](#9-erc-6909-claims)
- [10. State reading: Extsload, Exttload, StateLibrary](#10-state-reading-extsload-exttload-statelibrary)
- [11. Utility contracts and libraries](#11-utility-contracts-and-libraries)
- [12. Interfaces](#12-interfaces)
- [13. Test helpers](#13-test-helpers)
- [14. Reference tables](#14-reference-tables)
- [15. Use cases](#15-use-cases)
- [16. Writing your first hook](#16-writing-your-first-hook)
- [17. V3 to V4 migration map](#17-v3-to-v4-migration-map)
- [18. Gotchas and security notes](#18-gotchas-and-security-notes)

---

## 0. File inventory

Every `.sol` file under `src/`, grouped. Line counts are exact.

### Root contracts (7)

| File | Lines | What it is |
|---|---|---|
| `src/PoolManager.sol` | 395 | The singleton. Holds every pool's state and every token. |
| `src/ProtocolFees.sol` | 71 | Abstract base: protocol fee accrual, controller, collection. |
| `src/ERC6909.sol` | 90 | Solmate-derived multi-token implementation (claim tokens). |
| `src/ERC6909Claims.sol` | 23 | Adds `_burnFrom` with allowance/operator checks. |
| `src/Extsload.sol` | 64 | Exposes raw `sload` of arbitrary slots. |
| `src/Exttload.sol` | 40 | Exposes raw `tload` of arbitrary transient slots. |
| `src/NoDelegateCall.sol` | 33 | Modifier blocking `delegatecall` into the manager. |

### Types (7)

| File | Lines | Type | Underlying |
|---|---|---|---|
| `src/types/Currency.sol` | 119 | `Currency` | `address` |
| `src/types/PoolKey.sol` | 22 | `PoolKey` | struct (5 words) |
| `src/types/PoolId.sol` | 17 | `PoolId` | `bytes32` |
| `src/types/PoolOperation.sol` | 26 | `ModifyLiquidityParams`, `SwapParams` | structs |
| `src/types/BalanceDelta.sol` | 72 | `BalanceDelta` | `int256` (2×`int128`) |
| `src/types/BeforeSwapDelta.sol` | 38 | `BeforeSwapDelta` | `int256` (2×`int128`) |
| `src/types/Slot0.sol` | 94 | `Slot0` | `bytes32` (packed) |

### Libraries (24)

| File | Lines | Role |
|---|---|---|
| `src/libraries/Pool.sol` | 613 | The AMM itself: initialize, swap, modifyLiquidity, donate. |
| `src/libraries/Hooks.sol` | 340 | Permission bits, hook dispatch, return-delta handling. |
| `src/libraries/StateLibrary.sol` | 349 | Off-chain/external decoding of `Pool.State` via `extsload`. |
| `src/libraries/TickMath.sol` | 238 | tick ↔ `sqrtPriceX96`. |
| `src/libraries/SqrtPriceMath.sol` | 289 | Price/liquidity → token amount deltas. |
| `src/libraries/SwapMath.sol` | 108 | One swap step within a tick range. |
| `src/libraries/TickBitmap.sol` | 122 | Initialized-tick bitmap and next-tick search. |
| `src/libraries/Position.sol` | 103 | Position key derivation and fee crediting. |
| `src/libraries/FullMath.sol` | 117 | 512-bit `mulDiv`. |
| `src/libraries/BitMath.sol` | 49 | MSB / LSB. |
| `src/libraries/LiquidityMath.sol` | 19 | `uint128 + int128` with overflow check. |
| `src/libraries/SafeCast.sol` | 60 | Checked downcasts. |
| `src/libraries/UnsafeMath.sol` | 28 | Unchecked `divRoundingUp`, `simpleMulDiv`. |
| `src/libraries/FixedPoint96.sol` | 10 | `Q96` constant. |
| `src/libraries/FixedPoint128.sol` | 8 | `Q128` constant. |
| `src/libraries/Lock.sol` | 27 | The unlock flag (transient). |
| `src/libraries/CurrencyDelta.sol` | 41 | Per-(address,currency) delta map (transient). |
| `src/libraries/NonzeroDeltaCount.sol` | 34 | Count of outstanding debts (transient). |
| `src/libraries/CurrencyReserves.sol` | 38 | Synced currency + reserve snapshot (transient). |
| `src/libraries/TransientStateLibrary.sol` | 49 | External reads of all of the above. |
| `src/libraries/LPFeeLibrary.sol` | 79 | LP fee validity, dynamic-fee and override flags. |
| `src/libraries/ProtocolFeeLibrary.sol` | 47 | Two-direction 12-bit fee packing, swap-fee composition. |
| `src/libraries/CustomRevert.sol` | 120 | Cheap custom-error reverts + ERC-7751 bubbling. |
| `src/libraries/ParseBytes.sol` | 29 | Reads selector/delta/fee out of hook return data. |

### Interfaces (8)

| File | Lines | Declares |
|---|---|---|
| `src/interfaces/IPoolManager.sol` | 217 | 9 errors, 4 events, 13 functions. |
| `src/interfaces/IHooks.sol` | 152 | The 10 hook callbacks. |
| `src/interfaces/IProtocolFees.sol` | 52 | 3 errors, 2 events, 5 functions. |
| `src/interfaces/IExtsload.sol` | ~21 | 3 overloads of `extsload`. |
| `src/interfaces/IExttload.sol` | ~15 | 2 overloads of `exttload`. |
| `src/interfaces/callback/IUnlockCallback.sol` | 10 | `unlockCallback(bytes)`. |
| `src/interfaces/external/IERC6909Claims.sol` | 66 | ERC-6909 surface. |
| `src/interfaces/external/IERC20Minimal.sol` | 48 | Minimal ERC-20. |

### Test helpers (38)

Not deployed in production, but they are the canonical usage examples and the
best reading material after the core. Full table in [§13](#13-test-helpers).

### Docs and audits shipped in the repo

`docs/whitepaper/whitepaper-v4.pdf` (plus a Chinese translation and the LaTeX
source), `docs/security/Known_Effects_of_Hook_Permissions.pdf`, and five audit
PDFs under `docs/security/audits/`: Trail of Bits, Certora (draft), ABDK (draft),
Spearbit (draft), OpenZeppelin.

---

## 1. Architecture and the unlock lifecycle

### 1.1 The one-sentence version

V3 gave every pool its own contract that held its own tokens and settled every
action with immediate transfers. V4 puts **every pool in one contract**
(`PoolManager`), which **holds every token**, and replaces per-action transfers
with **signed deltas in transient storage** that must net to zero before the
transaction may end.

### 1.2 Inheritance

```
PoolManager
 ├── IPoolManager      (which is IProtocolFees, IERC6909Claims, IExtsload, IExttload)
 ├── ProtocolFees      (abstract; is IProtocolFees, Owned)
 ├── NoDelegateCall    (abstract)
 ├── ERC6909Claims     (abstract; is ERC6909, which is IERC6909Claims)
 ├── Extsload          (abstract; is IExtsload)
 └── Exttload          (abstract; is IExttload)
```

`PoolManager.sol:80`. `Owned` comes from solmate (`lib/solmate`), giving `owner`,
`transferOwnership(address)` and the `onlyOwner` modifier.

### 1.3 The unlock lifecycle

Nothing that moves value can be called while the manager is locked. The entry
point is `unlock`, which hands control straight back to the caller:

```
    caller (a router)                         PoolManager
         |                                         |
         |-- unlock(data) ------------------------>|  PoolManager.sol:104
         |                                         |  require !isUnlocked        :105
         |                                         |  Lock.unlock()  [tstore 1]  :107
         |<-- unlockCallback(data) ----------------|  :110  (calls back into YOU)
         |                                         |
         |   ... inside the callback you may now call ...
         |-- swap / modifyLiquidity / donate ----->|  each mutates deltas
         |-- sync(currency) ---------------------->|  snapshot reserves
         |-- (transfer tokens in) ---------------->|  plain ERC20 transfer, no manager call
         |-- settle() ---------------------------->|  credit yourself the difference
         |-- take(currency, to, amount) ---------->|  debit yourself, send tokens out
         |                                         |
         |-- return bytes ------------------------>|
         |                                         |  require NonzeroDeltaCount == 0  :112
         |                                         |    else revert CurrencyNotSettled
         |                                         |  Lock.lock()   [tstore 0]        :113
         |<-- result ------------------------------|
```

The rule the whole design rests on: **every operation writes a delta; the
transaction reverts unless every delta is zero at the end.** A negative delta
means you owe the manager; a positive delta means the manager owes you.

Sign convention, from the manager's point of view of *your* account:

| Delta | Meaning | How you clear it |
|---|---|---|
| negative | you owe tokens | `sync` → transfer in → `settle()`, or `burn` claims |
| positive | you are owed tokens | `take()`, or `mint` claims, or `clear()` to forfeit |

### 1.4 Storage map

Persistent storage of `PoolManager`, in declaration order across the inheritance
linearisation:

| Slot | Declared in | Variable |
|---|---|---|
| 0 | solmate `Owned` | `address owner` |
| 1 | `ProtocolFees.sol:21` | `mapping(Currency => uint256) protocolFeesAccrued` |
| 2 | `ProtocolFees.sol:24` | `address protocolFeeController` |
| 3 | `ERC6909.sol:15` | `mapping(address => mapping(address => bool)) isOperator` |
| 4 | `ERC6909.sol:17` | `mapping(address => mapping(uint256 => uint256)) balanceOf` |
| 5 | `ERC6909.sol:19` | `mapping(address => mapping(address => mapping(uint256 => uint256))) allowance` |
| 6 | `PoolManager.sol:93` | `mapping(PoolId => Pool.State) _pools` |

Slot 6 is asserted independently by `StateLibrary.POOLS_SLOT`
(`StateLibrary.sol:11`), which is what makes external state reads work. If the
inheritance order ever changed, that constant would have to change with it.

`NoDelegateCall` contributes `address private immutable original`
(`NoDelegateCall.sol:14`) — immutables live in bytecode, not storage, so they do
not consume a slot.

### 1.5 Transient storage map

All four slots are `keccak256(name) - 1`. I recomputed each one and they match
the source exactly:

| Slot | Constant | Holds |
|---|---|---|
| `0xc090fc4683624cfc3884e9d8de5eca132f2d0ec062aff75d43c0465d5ceeab23` | `Lock.IS_UNLOCKED_SLOT` (`Lock.sol:8`) | 1 while unlocked |
| `0x7d4b3164c6e45b97e7d87b7125a44c5828d005af88f9d751cfd78729c5d99a0b` | `NonzeroDeltaCount.NONZERO_DELTA_COUNT_SLOT` (`NonzeroDeltaCount.sol:9`) | count of nonzero deltas |
| `0x1e0745a7db1623981f0b2a5d4232364c00787266eb75ad546f190e6cebe9bd95` | `CurrencyReserves.RESERVES_OF_SLOT` (`CurrencyReserves.sol:11`) | balance snapshot at `sync` |
| `0x27e098c505d44ec3574004bca052aabf76bd35004c182099d8c575fb238593b9` | `CurrencyReserves.CURRENCY_SLOT` (`CurrencyReserves.sol:13`) | currently synced currency |

Plus one dynamic family: the per-account delta at
`keccak256(abi.encode(target, currency))`, derived in `CurrencyDelta._computeSlot`
(`CurrencyDelta.sol:10-16`) by writing `target` to scratch `0x00` and `currency`
to scratch `0x20` and hashing 64 bytes.

Note this uses the EVM scratch space (`0x00`–`0x40`) rather than the free memory
pointer, which is why the function is `memory-safe` despite writing to memory.

---

## 2. The type layer

V4 leans hard on user-defined value types (UDVTs). Each is a single EVM word with
a library attached `global`, so the operations look like methods but compile to
inline assembly with no allocation.

### 2.1 `Currency` — `types/Currency.sol`

```solidity
type Currency is address;                                          // :7
using {greaterThan as >, lessThan as <, greaterThanOrEqualTo as >=, equals as ==}
    for Currency global;                                           // :9
using CurrencyLibrary for Currency global;                         // :10
```

Native ETH is `Currency.wrap(address(0))`. This is the single change that lets V4
support raw ETH without WETH: a currency is either the zero address (native) or
an ERC-20.

#### Free functions (operators)

| Function | Line | Operator | Body |
|---|---|---|---|
| `equals(Currency,Currency)` | `:12` | `==` | `unwrap(a) == unwrap(b)` |
| `greaterThan(Currency,Currency)` | `:16` | `>` | `unwrap(a) > unwrap(b)` |
| `lessThan(Currency,Currency)` | `:20` | `<` | `unwrap(a) < unwrap(b)` |
| `greaterThanOrEqualTo(Currency,Currency)` | `:24` | `>=` | `unwrap(a) >= unwrap(b)` |

All `pure`. Address comparison is what enforces `currency0 < currency1` in
`PoolManager.initialize` (`PoolManager.sol:121`).

#### `CurrencyLibrary` (`:30-118`)

**Errors**

| Error | Line | Meaning |
|---|---|---|
| `NativeTransferFailed()` | `:32` | ETH `call` returned false. ERC-7751 context wrapper. |
| `ERC20TransferFailed()` | `:35` | ERC-20 `transfer` reverted or returned non-`true`. |

**Constant**

- `ADDRESS_ZERO = Currency.wrap(address(0))` (`:38`) — `public constant`, so it
  is also exposed as a getter on any contract that inherits the library's user.

---

**`transfer(Currency currency, address to, uint256 amount) internal`** — `:40-88`

Sends `amount` of `currency` from `address(this)` to `to`.

1. **Native branch** (`:45-53`): `call(gas(), to, amount, 0,0,0,0)`. Forwards
   *all* remaining gas, so a receiving contract may run arbitrary code. On
   failure, `CustomRevert.bubbleUpAndRevertWith(to, bytes4(0), NativeTransferFailed.selector)`.
2. **ERC-20 branch** (`:54-87`): hand-rolled `transfer(address,uint256)` call
   written at the free memory pointer. Success is
   `(returndata is exactly 1 AND len > 31) OR len == 0` — this accepts both
   standard-compliant tokens and the USDT-style tokens that return nothing. The
   three `mstore(...,0)` at `:77-79` scrub the scratch data afterwards.

State writes: none. External calls: one, to `to` (native) or the token (ERC-20).
Callers: `PoolManager.take` (`:295`), `ProtocolFees.collectProtocolFees` (`:56`).

> **Gotcha.** The native branch forwards all gas and happens *inside* `take`,
> after the delta has already been debited. A hostile recipient can re-enter, but
> the manager is still unlocked at that point, so re-entrancy is *expected* and
> safe — the final `NonzeroDeltaCount == 0` check is what actually protects the
> invariant, not a mutex.

---

**`balanceOfSelf(Currency) internal view returns (uint256)`** — `:90-96`

`address(this).balance` for native, `IERC20Minimal.balanceOf(address(this))`
otherwise. This is the primitive behind `sync`/`settle` balance-difference
accounting.

**`balanceOf(Currency, address owner) internal view returns (uint256)`** — `:98-104`

Same, for an arbitrary owner.

**`isAddressZero(Currency) internal pure returns (bool)`** — `:106-108`

The native-currency test, used at `PoolManager.sol:281`, `:353`,
`ProtocolFees.sol:49`, `TransientStateLibrary.sol:19`.

**`toId(Currency) internal pure returns (uint256)`** — `:110-112`

`uint160(unwrap(currency))`. The ERC-6909 token id for a currency.

**`fromId(uint256) internal pure returns (Currency)`** — `:116-118`

`Currency.wrap(address(uint160(id)))`. **The comment at `:114-115` is important:
the upper 12 bytes are silently discarded, so `fromId` and `toId` are not
inverses.** `PoolManager.mint` (`:324`) and `burn` (`:333`) both round-trip a
user-supplied `id` through `fromId`, meaning ids that differ only above 160 bits
collapse onto the same currency.

---

### 2.2 `PoolKey` — `types/PoolKey.sol`

```solidity
struct PoolKey {
    Currency currency0;   // :13  lower address
    Currency currency1;   // :15  higher address
    uint24 fee;           // :17  ≤ 1_000_000, or exactly 0x800000 for dynamic
    int24 tickSpacing;    // :19
    IHooks hooks;         // :21
}
using PoolIdLibrary for PoolKey global;   // :8
```

Five words. **Pool keys are never stored on chain** — the manager stores state
under `keccak256(key)` and the caller must re-supply the key on every call. That
is why `Initialize` (`IPoolManager.sol:60`) emits every field: indexers
reconstruct keys from logs.

The `hooks` address is *part of the key*. The same token pair, fee and tick
spacing with a different hook is a completely different pool.

### 2.3 `PoolId` — `types/PoolId.sol`

```solidity
type PoolId is bytes32;                                            // :6

function toId(PoolKey memory poolKey) internal pure returns (PoolId poolId) {
    assembly ("memory-safe") {
        poolId := keccak256(poolKey, 0xa0)                         // :14
    }
}
```

`0xa0` = 160 bytes = 5 words, hashing the struct's memory directly. The natspec
at `:10` promises this equals `keccak256(abi.encode(poolKey))`, which holds
because every field is a value type occupying exactly one word with no padding
ambiguity.

### 2.4 `PoolOperation` — `types/PoolOperation.sol`

Two calldata structs, split out of `IPoolManager` so hooks can import them
without importing the manager.

**`ModifyLiquidityParams`** (`:8-16`)

| Field | Type | Meaning |
|---|---|---|
| `tickLower` | `int24` | inclusive lower bound |
| `tickUpper` | `int24` | exclusive upper bound |
| `liquidityDelta` | `int256` | `> 0` add, `< 0` remove, `0` poke (collect fees) |
| `salt` | `bytes32` | lets one owner hold several distinct positions on one range |

`salt` is new in V4. In V3 a position was keyed by `(owner, tickLower, tickUpper)`
only, which is why the NFT position manager had to be the sole owner of every
position and track sub-ownership itself.

**`SwapParams`** (`:19-26`)

| Field | Type | Meaning |
|---|---|---|
| `zeroForOne` | `bool` | direction |
| `amountSpecified` | `int256` | **negative = exact input, positive = exact output** |
| `sqrtPriceLimitX96` | `uint160` | stop price |

> **Sign trap.** V3's `amountSpecified` was positive for exact-input. V4 inverted
> it: negative is exact-input. Read `SwapMath.sol:61` (`bool exactIn = amountRemaining < 0`)
> if you ever doubt it.

### 2.5 `BalanceDelta` — `types/BalanceDelta.sol`

```solidity
type BalanceDelta is int256;    // upper 128 bits = amount0, lower 128 = amount1
using {add as +, sub as -, eq as ==, neq as !=} for BalanceDelta global;  // :10
using BalanceDeltaLibrary for BalanceDelta global;                        // :11
```

**`toBalanceDelta(int128 _amount0, int128 _amount1)`** — `:14-18`

```solidity
balanceDelta := or(shl(128, _amount0), and(sub(shl(128, 1), 1), _amount1))
```

`shl(128, amount0)` puts amount0 up top; `and(2^128 - 1, amount1)` masks amount1
to its low 128 bits (necessary because a negative `int128` is sign-extended
across the full word before masking).

**`add` / `sub`** — `:20-32`, `:34-46`

Both unpack with `sar(128, x)` for the high half (arithmetic shift preserves
sign) and `signextend(15, x)` for the low half (sign-extends from byte 15, i.e.
bit 127), add or subtract as full `int256`, then repack through
`toBalanceDelta(res0.toInt128(), res1.toInt128())`.

**That final `toInt128` is the overflow check.** `SafeCast.toInt128`
(`SafeCast.sol:40-43`) reverts `SafeCastOverflow()` if the value does not fit.
Doing the arithmetic at `int256` width first and narrowing after is what makes
per-half overflow detectable; a naive `int256` addition of two packed values
would silently carry from the low half into the high half.

**`eq` / `neq`** — `:48-54`. Raw `int256` comparison.

**`BalanceDeltaLibrary`** — `:57-72`

| Member | Line | Notes |
|---|---|---|
| `ZERO_DELTA` | `:59` | `BalanceDelta.wrap(0)`, `public constant` |
| `amount0(BalanceDelta) → int128` | `:61-65` | `sar(128, d)` |
| `amount1(BalanceDelta) → int128` | `:67-71` | `signextend(15, d)` |

### 2.6 `BeforeSwapDelta` — `types/BeforeSwapDelta.sol`

```solidity
type BeforeSwapDelta is int256;
// upper 128 = delta in the SPECIFIED token, lower 128 = delta in the UNSPECIFIED token
```

Same packing as `BalanceDelta`, different semantics. It is indexed by the swap's
*specified*/*unspecified* roles rather than by token0/token1, because a hook that
charges a fee wants to talk about "the token the user named" without caring which
side of the pair that is.

Which token is "specified" depends on both direction and exactness. The mapping
back to token0/token1 happens once, in `Hooks.afterSwap` (`Hooks.sol:307-309`):

```solidity
hookDelta = (params.amountSpecified < 0 == params.zeroForOne)
    ? toBalanceDelta(hookDeltaSpecified, hookDeltaUnspecified)
    : toBalanceDelta(hookDeltaUnspecified, hookDeltaSpecified);
```

`amountSpecified < 0 == zeroForOne` is true exactly when the specified token is
token0 (exact-input zeroForOne: you name token0; exact-output oneForZero: you
name token0 as output).

| Function | Line | Body |
|---|---|---|
| `toBeforeSwapDelta(int128,int128)` | `:9-16` | free function, same `or(shl(128,·), and(mask,·))` |
| `BeforeSwapDeltaLibrary.ZERO_DELTA` | `:21` | `public constant` |
| `getSpecifiedDelta(BeforeSwapDelta)` | `:25-29` | `sar(128, delta)` |
| `getUnspecifiedDelta(BeforeSwapDelta)` | `:33-37` | `signextend(15, delta)` |

Note `BeforeSwapDeltaLibrary` is **not** attached `global` — callers must
`using BeforeSwapDeltaLibrary for BeforeSwapDelta` explicitly, as `Hooks.sol:23`
does.

### 2.7 `Slot0` — `types/Slot0.sol`

One word holding four fields. The layout comment is at `:9`:

```
 24 bits empty | 24 bits lpFee | 12 bits protocolFee 1→0 | 12 bits protocolFee 0→1 | 24 bits tick | 160 bits sqrtPriceX96
[255      232] [231       208] [207                 196] [195                 184] [183      160] [159               0]
```

| Constant | Line | Value |
|---|---|---|
| `MASK_160_BITS` | `:33` | `0x00FF..FF` (40 hex digits) |
| `MASK_24_BITS` | `:34` | `0xFFFFFF` |
| `TICK_OFFSET` | `:36` | `160` |
| `PROTOCOL_FEE_OFFSET` | `:37` | `184` |
| `LP_FEE_OFFSET` | `:38` | `208` |

**Getters** (all `internal pure`)

| Function | Line | Assembly |
|---|---|---|
| `sqrtPriceX96` | `:41-45` | `and(MASK_160_BITS, packed)` |
| `tick` | `:47-51` | `signextend(2, shr(160, packed))` — byte 2 = bit 23, the `int24` sign bit |
| `protocolFee` | `:53-57` | `and(MASK_24_BITS, shr(184, packed))` |
| `lpFee` | `:59-63` | `and(MASK_24_BITS, shr(208, packed))` |

**Setters** (all `internal pure`, returning a new `Slot0`)

| Function | Line | Pattern |
|---|---|---|
| `setSqrtPriceX96` | `:66-70` | `or(and(not(mask), packed), and(mask, value))` |
| `setTick` | `:72-76` | same, with `shl(TICK_OFFSET, ...)` |
| `setProtocolFee` | `:78-86` | same, at `PROTOCOL_FEE_OFFSET` |
| `setLpFee` | `:88-93` | same, at `LP_FEE_OFFSET` |

Each is clear-then-set, so setters compose safely in any order — as at
`Pool.sol:106` (`setSqrtPriceX96(...).setTick(...).setLpFee(...)`) and
`Pool.sol:439`.

The top 24 bits are unused. `protocolFee` is read as a 24-bit field and then
split into two 12-bit halves by `ProtocolFeeLibrary` ([§8.2](#82-protocolfeelibrary--librariesprotocolfeelibrarysol-47-lines)).

---

## 3. Flash accounting: the transient-storage layer

Five files implement the entire settlement system. All of them predate Solidity's
`transient` keyword and so hand-roll `tstore`/`tload` in assembly; three carry a
`TODO` saying they can be deleted once the keyword lands.

### 3.1 `Lock` — `libraries/Lock.sol`

| Member | Line | Detail |
|---|---|---|
| `IS_UNLOCKED_SLOT` | `:8` | `0xc090fc46…ab23` = `keccak256("Unlocked") - 1` ✓ verified |
| `unlock()` | `:10-15` | `tstore(slot, true)` |
| `lock()` | `:17-21` | `tstore(slot, false)` |
| `isUnlocked() → bool` | `:23-27` | `tload(slot)` |

Only `PoolManager.unlock` calls `unlock`/`lock` (`PoolManager.sol:107`, `:113`).
`isUnlocked` is read by the `onlyWhenUnlocked` modifier (`:97`) and by
`PoolManager._isUnlocked` (`:392-394`), which `ProtocolFees` declares abstract at
`ProtocolFees.sol:60`.

Because the flag lives in transient storage it is reset by the EVM at the end of
the transaction, so a reverted-and-caught inner call can never leave the manager
stuck unlocked across transactions.

### 3.2 `CurrencyDelta` — `libraries/CurrencyDelta.sol`

The per-account ledger. A transient mapping implemented by hand.

**`_computeSlot(address target, Currency currency) → bytes32`** — `:10-16`

```solidity
mstore(0,  and(target,   0xffff…ffff))    // scratch space
mstore(32, and(currency, 0xffff…ffff))
hashSlot := keccak256(0, 64)
```

Equivalent to `keccak256(abi.encode(target, currency))`. Both operands are masked
to 160 bits so a dirty upper word cannot forge a different slot.

**`getDelta(Currency, address target) → int256`** — `:18-23`. A `tload`.

**`applyDelta(Currency, address target, int128 delta) → (int256 previous, int256 next)`** — `:28-41`

`tload`, add in checked `int256` arithmetic (`next = previous + delta` at `:37` is
*outside* the assembly block, so Solidity's overflow check applies), `tstore`.
Returning both the old and new value is what lets the caller maintain the
nonzero count without a second read.

### 3.3 `NonzeroDeltaCount` — `libraries/NonzeroDeltaCount.sol`

| Member | Line | Detail |
|---|---|---|
| `NONZERO_DELTA_COUNT_SLOT` | `:9` | `0x7d4b3164…9a0b` = `keccak256("NonzeroDeltaCount") - 1` ✓ verified |
| `read() → uint256` | `:12-16` | `tload` |
| `increment()` | `:18-24` | `tstore(slot, tload(slot) + 1)`, unchecked |
| `decrement()` | `:28-34` | `tstore(slot, tload(slot) - 1)`, unchecked |

`decrement` can underflow — the natspec at `:26-27` says so explicitly and
argues correctness from the call sites: `_accountDelta` only decrements on a
`next == 0` transition that must have been preceded by an increment.

### 3.4 `CurrencyReserves` — `libraries/CurrencyReserves.sol`

Backs the `sync` → transfer → `settle` pattern.

| Member | Line | Detail |
|---|---|---|
| `RESERVES_OF_SLOT` | `:11` | `0x1e0745a7…bd95` = `keccak256("ReservesOf") - 1` ✓ verified |
| `CURRENCY_SLOT` | `:13` | `0x27e098c5…93b9` = `keccak256("Currency") - 1` ✓ verified |
| `getSyncedCurrency() → Currency` | `:15-19` | `tload(CURRENCY_SLOT)` |
| `resetCurrency()` | `:21-25` | `tstore(CURRENCY_SLOT, 0)` |
| `syncCurrencyAndReserves(Currency, uint256)` | `:27-32` | writes both slots together |
| `getSyncedReserves() → uint256` | `:34-38` | `tload(RESERVES_OF_SLOT)` |

The comment at `PoolManager.sol:357` (in `_settle`) — "Reserves are guaranteed to
be set because currency and reserves are always set together" — is only true
because `syncCurrencyAndReserves` writes both in one call and `resetCurrency`
clears only the currency. A zero currency therefore means "unsynced", and stale
reserves are unreachable.

### 3.5 `TransientStateLibrary` — `libraries/TransientStateLibrary.sol`

The read-only mirror, for external contracts and off-chain callers. Every
function goes through `manager.exttload(...)`.

| Function | Line | Returns |
|---|---|---|
| `getSyncedReserves(IPoolManager) → uint256` | `:18-21` | 0 if nothing is synced, else the snapshot |
| `getSyncedCurrency(IPoolManager) → Currency` | `:23-25` | the synced currency |
| `getNonzeroDeltaCount(IPoolManager) → uint256` | `:28-30` | outstanding debts |
| `currencyDelta(IPoolManager, address target, Currency) → int256` | `:35-43` | one account's delta |
| `isUnlocked(IPoolManager) → bool` | `:46-48` | the lock flag |

`currencyDelta` re-derives the slot with the same scratch-space hash as
`CurrencyDelta._computeSlot` (`:37-41`) rather than importing it, because the
library function is `internal` and would be inlined into the caller anyway.

> **Gotcha in `getSyncedReserves`.** It returns 0 when the synced currency is the
> zero address. But the zero address is also *native ETH*, which is a legitimate
> thing to sync (`PoolManager.sync` resets rather than snapshots for native, at
> `PoolManager.sol:283`). So "0" conflates "nothing synced" and "native synced" —
> which is fine only because native settlement uses `msg.value` and never reads
> the reserve.

---

## 4. `PoolManager`

`src/PoolManager.sol`, 395 lines. The `4`-shaped ASCII art at `:30-76` is 47
lines of the file.

### 4.0 Declarations

| Item | Line | Detail |
|---|---|---|
| `MAX_TICK_SPACING` | `:89` | `private constant` = `TickMath.MAX_TICK_SPACING` = `type(int16).max` = 32767 |
| `MIN_TICK_SPACING` | `:91` | `private constant` = `TickMath.MIN_TICK_SPACING` = 1 |
| `_pools` | `:93` | `mapping(PoolId => Pool.State) internal`, storage slot 6 |
| `onlyWhenUnlocked` | `:96-99` | reverts `ManagerLocked()` if `!Lock.isUnlocked()` |
| `constructor(address initialOwner)` | `:101` | forwards to `ProtocolFees(initialOwner)` → solmate `Owned` |

`using` directives at `:81-87`: `SafeCast for *`, `Pool for *`, `Hooks for IHooks`,
`CurrencyDelta for Currency`, `LPFeeLibrary for uint24`, `CurrencyReserves for Currency`,
`CustomRevert for bytes4`.

---

### 4.1 `unlock(bytes calldata data) → bytes memory result`

`:104-114` · `external` · selector `0x48c89491` · no access control

The only door in. Anyone may call it.

| Step | Line | Detail |
|---|---|---|
| check | `:105` | `if (Lock.isUnlocked()) revert AlreadyUnlocked()` — no nesting |
| transient write | `:107` | `Lock.unlock()` |
| external call | `:110` | `IUnlockCallback(msg.sender).unlockCallback(data)` |
| check | `:112` | `if (NonzeroDeltaCount.read() != 0) revert CurrencyNotSettled()` |
| transient write | `:113` | `Lock.lock()` |

Returns whatever the callback returned. Emits nothing.

**`msg.sender` must implement `IUnlockCallback`.** An EOA cannot call `unlock`
directly — the callback would revert.

**Not `noDelegateCall`.** Unlike `initialize`/`modifyLiquidity`/`swap`/`donate`,
`unlock` carries no such modifier. It does not need one: everything it can reach
that touches pool state is itself guarded.

---

### 4.2 `initialize(PoolKey memory key, uint160 sqrtPriceX96) → int24 tick`

`:117-142` · `external noDelegateCall` · selector `0x6276cbbe`

Creates a pool. **Permissionless and callable while locked** — it moves no value.

Checks, in order:

| Line | Check | Revert |
|---|---|---|
| `:119` | `key.tickSpacing > 32767` | `TickSpacingTooLarge(int24)` |
| `:120` | `key.tickSpacing < 1` | `TickSpacingTooSmall(int24)` |
| `:121` | `key.currency0 >= key.currency1` | `CurrenciesOutOfOrderOrEqual(address,address)` |
| `:126` | `!key.hooks.isValidHookAddress(key.fee)` | `Hooks.HookAddressNotValid(address)` |
| `:128` | `key.fee.getInitialLPFee()` | `LPFeeTooLarge(uint24)` if `fee > 1e6` and not the dynamic flag |

Then:

1. `key.hooks.beforeInitialize(key, sqrtPriceX96)` (`:130`) — hook call #1.
2. `id = key.toId()` (`:132`).
3. `tick = _pools[id].initialize(sqrtPriceX96, lpFee)` (`:134`) — reverts
   `PoolAlreadyInitialized()` if the pool exists; reverts `InvalidSqrtPrice` from
   `TickMath.getTickAtSqrtPrice` if the price is out of range.
4. `emit Initialize(...)` (`:139`).
5. `key.hooks.afterInitialize(key, sqrtPriceX96, tick)` (`:141`) — hook call #2.

The comment at `:136-138` explains the event-before-hook ordering: it guarantees
logs appear in causal order even when a hook emits its own.

**Storage written:** `_pools[id].slot0` only.

**A dynamic-fee pool starts at fee 0** (`LPFeeLibrary.getInitialLPFee`, `:51-56`).
If you want a nonzero starting fee you must call `updateDynamicLPFee` from your
`afterInitialize` hook — which is exactly why that hook exists.

---

### 4.3 `modifyLiquidity(PoolKey memory key, ModifyLiquidityParams memory params, bytes calldata hookData) → (BalanceDelta callerDelta, BalanceDelta feesAccrued)`

`:145-184` · `external onlyWhenUnlocked noDelegateCall` · selector `0x5a6bcfda`

Adds, removes, or pokes a position. One function for all three; `liquidityDelta`
decides.

```
modifyLiquidity
 |-- _getPool(id).checkPoolInitialized()                       :154
 |-- hooks.beforeModifyLiquidity(...)                          :156
 |     -> beforeAddLiquidity  if liquidityDelta > 0
 |     -> beforeRemoveLiquidity if liquidityDelta <= 0
 |-- Pool.modifyLiquidity(...)  -> (principalDelta, feesAccrued)  :159
 |     |-- checkTicks
 |     |-- updateTick(lower) / updateTick(upper)
 |     |-- tickBitmap.flipTick if flipped
 |     |-- getFeeGrowthInside -> Position.update -> feesOwed
 |     |-- clearTick if removing and flipped
 |     `-- SqrtPriceMath.getAmount{0,1}Delta by position-vs-price case
 |-- callerDelta = principalDelta + feesAccrued                 :171
 |-- emit ModifyLiquidity                                       :175
 |-- hooks.afterModifyLiquidity(...) -> (callerDelta, hookDelta) :178
 |-- if hookDelta != 0: _accountPoolBalanceDelta(key, hookDelta, address(hooks))  :181
 `-- _accountPoolBalanceDelta(key, callerDelta, msg.sender)     :183
```

**The position owner is `msg.sender`** (`:161`) — that is, the *router*, not the
end user. Combined with `params.salt`, a router can hold many independent
positions and manage sub-ownership itself. This is why V4 needs no NFT in core.

`params.liquidityDelta.toInt128()` at `:164` reverts `SafeCastOverflow()` for a
delta outside `int128`.

**Returns:** `callerDelta` is the *total* (principal + fees, minus any hook
delta); `feesAccrued` is the fee component alone, so callers can distinguish
"what I put in" from "what I earned". Note the hook is handed both (`:178`).

---

### 4.4 `swap(PoolKey memory key, SwapParams memory params, bytes calldata hookData) → BalanceDelta swapDelta`

`:187-227` · `external onlyWhenUnlocked noDelegateCall` · selector `0xf3cd914c`

| Step | Line | Detail |
|---|---|---|
| check | `:193` | `amountSpecified == 0` → `SwapAmountCannotBeZero()` |
| check | `:196` | `checkPoolInitialized()` → `PoolNotInitialized()` |
| hook | `:202` | `beforeSwap` → `(amountToSwap, beforeSwapDelta, lpFeeOverride)` |
| core | `:206` | `_swap(...)` with the *possibly adjusted* `amountToSwap` |
| hook | `:221` | `afterSwap` → `(swapDelta, hookDelta)` |
| deltas | `:224` | hook's delta charged to `address(key.hooks)` |
| deltas | `:226` | caller's delta charged to `msg.sender` |

The input currency passed to `_swap` at `:216` is
`params.zeroForOne ? key.currency0 : key.currency1` — the protocol fee is always
taken on the input side.

**`_swap(Pool.State storage, PoolId, Pool.SwapParams memory, Currency inputCurrency) → BalanceDelta`** — `:230-253` · `internal`

Exists solely to dodge stack-too-deep (comment at `:205`). It calls
`pool.swap(params)` (`:235`), routes `amountToProtocol` to
`_updateProtocolFees(inputCurrency, ...)` (`:238`) if nonzero, emits `Swap`
(`:241-250`) with the post-swap `sqrtPriceX96`, `liquidity`, `tick` and the
composed `swapFee`, and returns the delta.

Note the emitted `swapFee` is the **total** (LP + protocol), not the LP fee — see
`Pool.sol:307`.

---

### 4.5 `donate(PoolKey memory key, uint256 amount0, uint256 amount1, bytes calldata hookData) → BalanceDelta delta`

`:256-276` · `external onlyWhenUnlocked noDelegateCall` · selector `0x234266d7`

Gives tokens to the currently in-range LPs by bumping `feeGrowthGlobal` without a
swap. Order: `checkPoolInitialized` (`:264`) → `beforeDonate` (`:266`) →
`Pool.donate` (`:268`) → account delta (`:270`) → `emit Donate` (`:273`) →
`afterDonate` (`:275`).

Reverts `NoLiquidityToReceiveFees()` (`Pool.sol:468`) if `liquidity == 0`.

> **The tick caveat.** `IPoolManager.sol:152-155` warns that donate credits LPs at
> `slot0.tick`, which after a `zeroForOne` swap that lands exactly on a tick
> boundary is `tickNext - 1` while the price sits at `tickNext` (`Pool.sol:431`,
> and the explanatory comment at `:409-412`). A donor who cares must check both
> tick and price.

---

### 4.6 `sync(Currency currency)`

`:279-288` · `external` · selector `0xa5841194` · **no `onlyWhenUnlocked`**

Snapshots the manager's balance so a later `settle` can measure the difference.

- Native (`:281-283`): calls `CurrencyReserves.resetCurrency()` — it *clears* the
  currency slot rather than storing zero-as-a-currency, because native settlement
  reads `msg.value`, not a balance difference.
- ERC-20 (`:284-287`): `syncCurrencyAndReserves(currency, currency.balanceOfSelf())`.

**Callable while locked**, and that is deliberate: `IPoolManager.sol:170-171`
notes integrators should call `sync` before sending native funds too.

---

### 4.7 `take(Currency currency, address to, uint256 amount)`

`:291-297` · `external onlyWhenUnlocked` · selector `0x0b0d9c09`

Pull tokens out, going into debt for them.

```solidity
_accountDelta(currency, -(amount.toInt128()), msg.sender);   // :294
currency.transfer(to, amount);                               // :295
```

Debit first, transfer second. The `unchecked` at `:292` is safe because
`toInt128(uint256)` (`SafeCast.sol:56-59`) already rejects anything `≥ 2^127`, so
negation cannot overflow.

---

### 4.8 `settle() payable → uint256` and `settleFor(address recipient) payable → uint256`

`:300-302`, `:305-307` · `external payable onlyWhenUnlocked` · selectors
`0x11da60b4`, `0x3dd45adb`

Both delegate to `_settle`. `settle()` credits `msg.sender`; `settleFor(recipient)`
credits someone else — used when a router pays on behalf of a hook, or when a
hook's debt is covered by the swapper.

**`_settle(address recipient) → uint256 paid`** — `:349-365` · `internal`

```solidity
Currency currency = CurrencyReserves.getSyncedCurrency();     // :350
if (currency.isAddressZero()) {
    paid = msg.value;                                         // :354  native
} else {
    if (msg.value > 0) NonzeroNativeValue.selector.revertWith();   // :356
    uint256 reservesBefore = CurrencyReserves.getSyncedReserves(); // :358
    uint256 reservesNow = currency.balanceOfSelf();                // :359
    paid = reservesNow - reservesBefore;                           // :360
    CurrencyReserves.resetCurrency();                              // :361
}
_accountDelta(currency, paid.toInt128(), recipient);          // :364
```

Three things worth noticing:

1. **Un-synced settle is a native settle.** If you never called `sync`, the
   currency slot is zero and `_settle` takes the `msg.value` branch. Forgetting
   `sync` before an ERC-20 settle therefore does not revert — it credits you
   `msg.value` (usually 0) of native currency, and your real debt stays open
   until the final check catches it.
2. `paid = reservesNow - reservesBefore` (`:360`) reverts on underflow, which is
   what stops you settling against a balance that went *down*.
3. `resetCurrency()` (`:361`) means one `sync` serves exactly one `settle`.

---

### 4.9 `clear(Currency currency, uint256 amount)`

`:310-319` · `external onlyWhenUnlocked` · selector `0x80f0b44c`

Forfeit a positive delta rather than `take` it — for dust too small to be worth a
transfer.

```solidity
int256 current = currency.getDelta(msg.sender);   // :311
int128 amountDelta = amount.toInt128();           // :313
if (amountDelta != current) MustClearExactPositiveDelta.selector.revertWith();  // :314
_accountDelta(currency, -(amountDelta), msg.sender);   // :317
```

The equality check means you must clear the **entire** delta, exactly. Because
the input is `uint256`, only positive deltas can be cleared — you cannot use this
to wipe a debt. The forfeited tokens stay in the manager and are effectively a
donation to nobody in particular (they become claimable by the next `sync`-based
accounting or by protocol fee collection only insofar as they inflate balances).

---

### 4.10 `mint(address to, uint256 id, uint256 amount)` and `burn(address from, uint256 id, uint256 amount)`

`:322-329`, `:332-336` · `external onlyWhenUnlocked` · selectors `0x156e29f6`,
`0xf5298aca`

Convert between a delta and an ERC-6909 claim token.

`mint`: `_accountDelta(currency, -(amount.toInt128()), msg.sender)` then
`_mint(to, currency.toId(), amount)` — you take on debt and receive claims.

`burn`: `_accountDelta(currency, amount.toInt128(), msg.sender)` then
`_burnFrom(from, currency.toId(), amount)` — you give up claims and are credited.

Both round the caller-supplied `id` through `CurrencyLibrary.fromId` (`:324`,
`:333`), truncating to 160 bits.

`_burnFrom` (`ERC6909Claims.sol:13-22`) enforces operator/allowance if
`from != msg.sender`.

**Why claims exist:** settling in claim tokens avoids an ERC-20 transfer
entirely. A router that will trade the same token again next block can hold
claims instead of moving tokens in and out. It is the manager's internal balance
sheet, exposed as a standard token.

---

### 4.11 `updateDynamicLPFee(PoolKey memory key, uint24 newDynamicLPFee)`

`:339-346` · `external` · selector `0x52759651` · **no `onlyWhenUnlocked`**

```solidity
if (!key.fee.isDynamicFee() || msg.sender != address(key.hooks)) {
    UnauthorizedDynamicLPFeeUpdate.selector.revertWith();     // :341
}
newDynamicLPFee.validate();                                   // :343
_pools[key.toId()].setLPFee(newDynamicLPFee);                 // :345
```

Two conditions, both required: the pool's `fee` field must be exactly
`0x800000`, and the caller must be the pool's own hook. Reverts
`LPFeeTooLarge(uint24)` above `1_000_000`, and `PoolNotInitialized()` from
`Pool.setLPFee` (`Pool.sol:116`) if the pool does not exist.

---

### 4.12 Internal accounting

**`_accountDelta(Currency currency, int128 delta, address target)`** — `:368-378`

The heart of the system.

```solidity
if (delta == 0) return;                                       // :369
(int256 previous, int256 next) = currency.applyDelta(target, delta);  // :371
if (next == 0)          NonzeroDeltaCount.decrement();        // :373-374
else if (previous == 0) NonzeroDeltaCount.increment();        // :375-376
```

The count tracks *how many* (address, currency) pairs are nonzero, not the sum.
Transitions only: 0→nonzero increments, nonzero→0 decrements, nonzero→nonzero
does nothing. The early return on zero delta is what keeps the count honest when
a swap produces no movement on one side.

**`_accountPoolBalanceDelta(PoolKey memory key, BalanceDelta delta, address target)`** — `:381-384`

Splits a `BalanceDelta` and applies both halves.

**`_getPool(PoolId) → Pool.State storage`** — `:387-389` · `internal view override`

Implements the abstract declared at `ProtocolFees.sol:64`, giving the fee logic
access to `_pools`.

**`_isUnlocked() → bool`** — `:392-394` · `internal view override`

Implements `ProtocolFees.sol:60`.

> Note both overrides are `view` while the abstract declarations
> (`ProtocolFees.sol:60`, `:64`) are non-view. Solidity permits tightening
> mutability in an override.

---

## 5. `Pool` library

`src/libraries/Pool.sol`, 613 lines. This is V3's `UniswapV3Pool` with the
contract shell removed: no ERC-20, no oracle, no `msg.sender`, no token
transfers. Pure state transitions on a `State` struct.

### 5.1 Errors (11)

| Error | Line | Thrown when |
|---|---|---|
| `TicksMisordered(int24,int24)` | `:33` | `tickLower >= tickUpper` |
| `TickLowerOutOfBounds(int24)` | `:37` | `tickLower < MIN_TICK` |
| `TickUpperOutOfBounds(int24)` | `:41` | `tickUpper > MAX_TICK` |
| `TickLiquidityOverflow(int24)` | `:44` | `liquidityGross` exceeds `maxLiquidityPerTick` |
| `PoolAlreadyInitialized()` | `:47` | `initialize` on a live pool |
| `PoolNotInitialized()` | `:50` | any op on a pool with `sqrtPriceX96 == 0` |
| `PriceLimitAlreadyExceeded(uint160,uint160)` | `:55` | limit is on the wrong side of the current price |
| `PriceLimitOutOfBounds(uint160)` | `:59` | limit outside `(MIN_SQRT_PRICE, MAX_SQRT_PRICE)` |
| `NoLiquidityToReceiveFees()` | `:62` | `donate` with zero in-range liquidity |
| `InvalidFeeForExactOut()` | `:65` | exact-output swap at a 100% swap fee |

### 5.2 Structs

**`TickInfo`** (`:68-77`)

| Field | Type | Meaning |
|---|---|---|
| `liquidityGross` | `uint128` | total liquidity referencing this tick (for flip detection) |
| `liquidityNet` | `int128` | liquidity added when crossing left→right |
| `feeGrowthOutside0X128` | `uint256` | fee growth on the far side of this tick |
| `feeGrowthOutside1X128` | `uint256` | same, token1 |

`liquidityGross` and `liquidityNet` share one slot, which `updateTick` exploits
(see below).

**`State`** (`:83-91`)

| Offset | Field | Type |
|---|---|---|
| 0 | `slot0` | `Slot0` |
| 1 | `feeGrowthGlobal0X128` | `uint256` |
| 2 | `feeGrowthGlobal1X128` | `uint256` |
| 3 | `liquidity` | `uint128` |
| 4 | `ticks` | `mapping(int24 => TickInfo)` |
| 5 | `tickBitmap` | `mapping(int16 => uint256)` |
| 6 | `positions` | `mapping(bytes32 => Position.State)` |

These offsets are hard-coded in `StateLibrary` (`:14-28`) — changing the struct
silently breaks every external reader.

The natspec at `:80-82` carries a live warning: **`feeGrowthGlobal` can be
artificially inflated.** In a pool with a single position, that LP can `donate`
to itself and collect, ratcheting the global counter. Do not use it as a
cross-pool metric.

**`ModifyLiquidityParams`** (`:120-132`): `owner`, `tickLower`, `tickUpper`,
`liquidityDelta` (`int128` here, narrowed from the interface's `int256`),
`tickSpacing`, `salt`.

**`ModifyLiquidityState`** (`:134-139`): scratch for `flippedLower`,
`liquidityGrossAfterLower`, `flippedUpper`, `liquidityGrossAfterUpper`.

**`SwapResult`** (`:241-248`): `sqrtPriceX96`, `tick`, `liquidity` — the
post-swap snapshot returned for the event.

**`StepComputations`** (`:250-267`): per-iteration scratch — `sqrtPriceStartX96`,
`tickNext`, `initialized`, `sqrtPriceNextX96`, `amountIn`, `amountOut`,
`feeAmount`, `feeGrowthGlobalX128`.

**`SwapParams`** (`:269-275`): `amountSpecified`, `tickSpacing`, `zeroForOne`,
`sqrtPriceLimitX96`, `lpFeeOverride`. Distinct from the public
`PoolOperation.SwapParams` — this one carries the hook's fee override.

### 5.3 `checkTicks(int24 tickLower, int24 tickUpper)` — `:94-98` · `private pure`

Three checks in order: misordered, lower bound, upper bound. Called once, from
`modifyLiquidity` (`:153`).

### 5.4 `initialize(State storage, uint160 sqrtPriceX96, uint24 lpFee) → int24 tick` — `:100-107`

Reverts `PoolAlreadyInitialized()` if `slot0.sqrtPriceX96() != 0` (`:101`),
derives the tick via `TickMath.getTickAtSqrtPrice` (`:103`, which itself reverts
`InvalidSqrtPrice` out of range), and writes a fresh `Slot0` (`:106`). Protocol
fee is left at 0 — the comment at `:105` notes it needs no explicit set.

### 5.5 `setProtocolFee` — `:109-112` and `setLPFee` — `:115-118`

Both `checkPoolInitialized()` then a single `slot0` setter. `setLPFee`'s natspec
(`:114`) says only dynamic-fee pools may call it; that restriction is enforced by
the caller (`PoolManager.sol:340`), not here.

### 5.6 `modifyLiquidity(State storage, ModifyLiquidityParams memory) → (BalanceDelta delta, BalanceDelta feeDelta)` — `:146-238`

**Phase 1 — ticks** (`:159-181`, only if `liquidityDelta != 0`)

`updateTick` for lower and upper (`:160-162`). If adding (`liquidityDelta >= 0`,
using `>=` because it is cheaper than `>` and equivalent here per the comment at
`:164`), both `liquidityGrossAfter` values are checked against
`tickSpacingToMaxLiquidityPerTick(tickSpacing)` (`:166-172`), reverting
`TickLiquidityOverflow(tick)`. Any tick that flipped gets its bitmap bit toggled
(`:175-180`).

**Phase 2 — fees** (`:183-193`)

`getFeeGrowthInside(self, tickLower, tickUpper)` (`:184-185`), then
`self.positions.get(owner, tickLower, tickUpper, salt).update(...)` (`:187-189`),
which returns the fees owed. Packed into `feeDelta` (`:192`).

**Phase 3 — cleanup** (`:196-203`)

When removing, any tick that flipped to uninitialized is `delete`d via
`clearTick`. This is the gas refund path and also stops stale
`feeGrowthOutside` values from being resurrected with wrong semantics.

**Phase 4 — amounts** (`:206-237`)

Three cases against the current tick:

| Case | Line | Delta | Why |
|---|---|---|---|
| `tick < tickLower` | `:209-217` | all token0 | range is entirely above the price |
| `tickLower ≤ tick < tickUpper` | `:218-226` | both tokens, **and `self.liquidity` is updated** | range straddles the price |
| `tick ≥ tickUpper` | `:227-236` | all token1 | range is entirely below the price |

Only the in-range case touches `self.liquidity` (`:226`) — out-of-range liquidity
is not active and must not count toward the pool's swap depth.

The comments at `:210-211` and `:228-229` give the intuition: liquidity enters
range by crossing from the side where the *other* token is needed.

**Poke.** `liquidityDelta == 0` skips phases 1, 3 and 4 entirely, running only
the fee update — `delta` stays zero and only `feeDelta` is returned. That is how
you collect fees without changing your position. `Position.update` reverts
`CannotUpdateEmptyPosition()` if you poke a position that has no liquidity
(`Position.sol:86`).

### 5.7 `swap(State storage, SwapParams memory) → (BalanceDelta swapDelta, uint256 amountToProtocol, uint24 swapFee, SwapResult memory result)` — `:279-463`

**Setup** (`:283-341`)

1. `protocolFee` selected by direction (`:286-287`): `getZeroForOneFee` or
   `getOneForZeroFee` of the packed 24-bit field.
2. `swapFee` composed (`:302-308`): if the hook returned an override *and* the
   override flag is set, use it (validated); else use the stored `lpFee`. Then
   `swapFee = protocolFee == 0 ? lpFee : calculateSwapFee(protocolFee, lpFee)`.
3. **100% fee guard** (`:311-316`): a `swapFee` of `MAX_SWAP_FEE` makes exact-output
   impossible (the input would be entirely consumed by fee), so `InvalidFeeForExactOut()`.
4. Zero-amount early return (`:320`) — returns a zero delta but still reports
   `swapFee`.
5. **Price-limit validation** (`:322-338`), direction-dependent:
   - `zeroForOne`: limit must be `< current` and `> MIN_SQRT_PRICE`
   - `oneForZero`: limit must be `> current` and `< MAX_SQRT_PRICE`

   The comment at `:326-327` notes swaps never occur *at* `MIN_TICK`.

**The loop** (`:344-437`)

```
while (amountSpecifiedRemaining != 0 && sqrtPriceX96 != sqrtPriceLimitX96) {
    1. tickNext, initialized = tickBitmap.nextInitializedTickWithinOneWord(...)   :347
    2. clamp tickNext into [MIN_TICK, MAX_TICK]                                   :351-356
    3. sqrtPriceNextX96 = TickMath.getSqrtPriceAtTick(tickNext)                   :359
    4. (sqrtPriceX96, amountIn, amountOut, feeAmount) =
           SwapMath.computeSwapStep(current, target, liquidity, remaining, swapFee)  :362
    5. update amountSpecifiedRemaining / amountCalculated by exactness            :371-382
    6. carve protocol fee out of feeAmount                                        :385-398
    7. feeGrowthGlobalX128 += feeAmount * Q128 / liquidity   (if liquidity > 0)   :401-407
    8. if we hit the target price: cross the tick, flip liquidityNet sign if      :413-432
       zeroForOne, update liquidity, set tick (tickNext - 1 if zeroForOne)
       else if the price moved: recompute tick from price                         :433-436
}
```

Step 2's clamping is needed because the bitmap has no idea the tick range is
bounded (comment at `:350`).

**Step 6 in detail** (`:385-398`). Protocol fee is a fraction of the *total* input
including the LP fee, so `step.amountIn + step.feeAmount` is reconstructed. The
`swapFee == protocolFee` shortcut (`:391-392`) handles a zero LP fee, where the
whole fee is the protocol's. The division rounds down, which the comment at
`:390` says is deliberate: **rounding favours LPs over the protocol.**

**Step 7** uses `UnsafeMath.simpleMulDiv` rather than `FullMath.mulDiv`, justified
at `:403`: token supply is bounded by `uint128`, so `feeAmount * Q128` cannot
overflow `uint256`.

**Step 8's tick pre-decrement** (`:431`) is the single most confusing line in V4:

```solidity
result.tick = zeroForOne ? step.tickNext - 1 : step.tickNext;
```

After a leftward cross, `slot0.tick` is one less than
`getTickAtSqrtPrice(slot0.sqrtPriceX96)`. The comment at `:409-412` says this is
harmless for swaps but that `donate` must check both.

**Teardown** (`:439-462`)

`slot0` gets the new tick and price (`:439`); `liquidity` written only if changed
(`:442`); the direction's `feeGrowthGlobal` written (`:445-449`). Then the delta
is packed (`:451-462`) with the same specified/unspecified rotation seen in
`Hooks.afterSwap`:

```solidity
if (zeroForOne != (params.amountSpecified < 0)) {
    swapDelta = toBalanceDelta(amountCalculated.toInt128(),
                               (params.amountSpecified - amountSpecifiedRemaining).toInt128());
} else {
    swapDelta = toBalanceDelta((params.amountSpecified - amountSpecifiedRemaining).toInt128(),
                               amountCalculated.toInt128());
}
```

`params.amountSpecified - amountSpecifiedRemaining` is "how much of the specified
side was actually used", which for a partial fill (price limit hit) is less than
requested.

### 5.8 `donate(State storage, uint256 amount0, uint256 amount1) → BalanceDelta` — `:466-480`

Reverts `NoLiquidityToReceiveFees()` if `liquidity == 0` (`:468`). Returns a
*negative* delta (`:471`) — the donor owes both amounts. Bumps
`feeGrowthGlobal{0,1}X128` by `amount * Q128 / liquidity` (`:474`, `:477`), each
guarded by a nonzero check so a one-sided donation costs one SSTORE instead of two.

### 5.9 `getFeeGrowthInside(State storage, int24 tickLower, int24 tickUpper) → (uint256, uint256)` — `:488-511` · `internal view`

The classic three-case subtraction, all `unchecked` because **underflow is the
intended behaviour** — these counters wrap and only differences are meaningful.

| Current tick | Formula |
|---|---|
| `< tickLower` | `lower.outside - upper.outside` |
| `≥ tickUpper` | `upper.outside - lower.outside` |
| in range | `global - lower.outside - upper.outside` |

### 5.10 `updateTick(State storage, int24 tick, int128 liquidityDelta, bool upper) → (bool flipped, uint128 liquidityGrossAfter)` — `:520-558`

`liquidityGrossAfter = LiquidityMath.addDelta(before, delta)` (`:529`);
`flipped = (after == 0) != (before == 0)` (`:531`).

On first initialization (`:533-539`), if `tick <= slot0.tick()` the tick inherits
the current globals — the convention that "all growth before initialization
happened below the tick" (`:534`).

`liquidityNet` moves *opposite* for an upper tick (`:543`).

The write (`:544-557`) is hand-packed assembly doing a **single `sstore`** of both
`liquidityGross` (low 128 bits) and `liquidityNet` (high 128), because they share
slot 0 of `TickInfo`. No `signextend` is needed on `liquidityNet` since shifting
left discards the high bits anyway (comment at `:553`).

### 5.11 `tickSpacingToMaxLiquidityPerTick(int24 tickSpacing) → uint128` — `:565-582` · `internal pure`

`type(uint128).max / numTicks`, where `numTicks` counts usable ticks at that
spacing. The Solidity equivalent is spelled out in the comment at `:566-571`; the
assembly (`:575-581`) computes `minTick` with a floor-division correction
(`sub(sdiv(...), slt(smod(...), 0))`) because EVM `sdiv` truncates toward zero.

Guaranteeing `Σ liquidityGross ≤ uint128.max` is what makes the packed tick write
safe.

### 5.12 `checkPoolInitialized(State storage)` — `:585-587` · `internal view`

`if (slot0.sqrtPriceX96() == 0) revert PoolNotInitialized()`. Called at
`PoolManager.sol:154`, `:196`, `:264`, and from `Pool.setProtocolFee`/`setLPFee`.

### 5.13 `clearTick(State storage, int24 tick)` — `:592-594`

`delete self.ticks[tick]`.

### 5.14 `crossTick(State storage, int24 tick, uint256 feeGrowthGlobal0X128, uint256 feeGrowthGlobal1X128) → int128 liquidityNet` — `:602-612`

Flips both `feeGrowthOutside` values to `global - outside` (`:608-609`) and
returns `liquidityNet`. `unchecked` for the same wrap-around reason as
`getFeeGrowthInside`.

---

## 6. Hooks

### 6.1 The permission bits

`libraries/Hooks.sol:27-47`. A hook's **address** encodes its permissions in the
low 14 bits. `ALL_HOOK_MASK = (1 << 14) - 1 = 0x3FFF` (`:27`).

| Bit | Value | Constant | Line |
|---|---|---|---|
| 13 | `0x2000` | `BEFORE_INITIALIZE_FLAG` | `:29` |
| 12 | `0x1000` | `AFTER_INITIALIZE_FLAG` | `:30` |
| 11 | `0x0800` | `BEFORE_ADD_LIQUIDITY_FLAG` | `:32` |
| 10 | `0x0400` | `AFTER_ADD_LIQUIDITY_FLAG` | `:33` |
| 9 | `0x0200` | `BEFORE_REMOVE_LIQUIDITY_FLAG` | `:35` |
| 8 | `0x0100` | `AFTER_REMOVE_LIQUIDITY_FLAG` | `:36` |
| 7 | `0x0080` | `BEFORE_SWAP_FLAG` | `:38` |
| 6 | `0x0040` | `AFTER_SWAP_FLAG` | `:39` |
| 5 | `0x0020` | `BEFORE_DONATE_FLAG` | `:41` |
| 4 | `0x0010` | `AFTER_DONATE_FLAG` | `:42` |
| 3 | `0x0008` | `BEFORE_SWAP_RETURNS_DELTA_FLAG` | `:44` |
| 2 | `0x0004` | `AFTER_SWAP_RETURNS_DELTA_FLAG` | `:45` |
| 1 | `0x0002` | `AFTER_ADD_LIQUIDITY_RETURNS_DELTA_FLAG` | `:46` |
| 0 | `0x0001` | `AFTER_REMOVE_LIQUIDITY_RETURNS_DELTA_FLAG` | `:47` |

The file header (`:15-18`) gives the worked example: an address ending `0x2400`
has bits 13 and 10 set → `beforeInitialize` and `afterAddLiquidity`.

**This is why hook addresses must be mined.** You brute-force a CREATE2 salt
until the resulting address has exactly the bits you want. Get it wrong and
either your hook is never called or `isValidHookAddress` rejects the pool.

**`Permissions` struct** (`:49-64`) — the same 14 flags as named booleans, for
constructor-time self-checks.

### 6.2 Errors

| Error | Line | Cause |
|---|---|---|
| `HookAddressNotValid(address)` | `:68` | address bits do not match declared permissions, or fail `isValidHookAddress` |
| `InvalidHookResponse()` | `:71` | hook returned the wrong selector or wrong-length data |
| `HookCallFailed()` | `:74` | the hook reverted; ERC-7751 context |
| `HookDeltaExceedsSwapAmount()` | `:77` | `beforeSwap` delta flipped exactIn↔exactOut |

### 6.3 `validateHookPermissions(IHooks self, Permissions memory permissions)` — `:83-103` · `internal pure`

Compares all 14 declared booleans against the address bits and reverts
`HookAddressNotValid(address)` on any mismatch. Meant for hook constructors
(natspec `:79-82`); the manager never calls it.

### 6.4 `isValidHookAddress(IHooks self, uint24 fee) → bool` — `:109-127` · `internal pure`

Called by `PoolManager.initialize` (`:126`). Two rule families:

**Return-delta flags require their action flag** (`:111-120`): four checks, each
returning false if a `RETURNS_DELTA` bit is set without the corresponding action
bit. You cannot promise a delta from a callback you never receive.

**At least one flag, or dynamic fee** (`:124-126`):

```solidity
return address(self) == address(0)
    ? !fee.isDynamicFee()
    : (uint160(address(self)) & ALL_HOOK_MASK > 0 || fee.isDynamicFee());
```

- No hook (`address(0)`) → the fee may not be dynamic (nobody could ever update it).
- A hook → it must have at least one permission bit, **or** the pool must be
  dynamic-fee. The second disjunct is what allows a hook whose only job is
  calling `updateDynamicLPFee`, with no callbacks at all.

### 6.5 `callHook(IHooks self, bytes memory data) → bytes memory result` — `:131-155` · `internal`

The raw dispatcher.

1. `call(gas(), self, 0, add(data,0x20), mload(data), 0, 0)` (`:134`) — all gas, no
   value, return data ignored at call time.
2. On failure (`:137`): `CustomRevert.bubbleUpAndRevertWith(address(self), bytes4(data), HookCallFailed.selector)`
   — wraps the hook's own revert in ERC-7751 `WrappedError` so the reason
   survives.
3. Copies return data into a fresh `bytes` (`:140-149`).
4. **Selector echo check** (`:152-154`): `result.length >= 32` and
   `result.parseSelector() == data.parseSelector()`. The hook must return the
   selector of the function that was called, else `InvalidHookResponse()`.

That echo is the anti-confusion measure: a contract that happens to have a
fallback returning garbage cannot be mistaken for a working hook.

### 6.6 `callHookWithReturnDelta(IHooks self, bytes memory data, bool parseReturn) → int256` — `:159-168` · `internal`

Calls `callHook`, then:
- `!parseReturn` → return 0 (the hook lacks the RETURNS_DELTA bit, so whatever it
  returned is ignored).
- else require `result.length == 64` exactly (`:166`) — `bytes4` + `int256` — and
  return `result.parseReturnDelta()`.

### 6.7 `noSelfCall(IHooks self)` modifier — `:171-175`

```solidity
if (msg.sender != address(self)) { _; }
```

**If the hook itself is the caller, the body is skipped entirely.** A hook that
calls `swap` on its own pool does not re-enter its own callbacks. Applied to
`beforeInitialize`, `afterInitialize`, `beforeModifyLiquidity`, `beforeDonate`,
`afterDonate`. The three functions with return values —`afterModifyLiquidity`
(`:217`), `beforeSwap` (`:253`), `afterSwap` (`:293`) — implement the same check
inline with an early `return`, because a modifier that skips the body cannot
supply return values.

### 6.8 The ten dispatchers

| Function | Line | Flag(s) | Return |
|---|---|---|---|
| `beforeInitialize` | `:178-182` | `BEFORE_INITIALIZE` | — |
| `afterInitialize` | `:185-192` | `AFTER_INITIALIZE` | — |
| `beforeModifyLiquidity` | `:195-206` | `BEFORE_ADD_LIQUIDITY` / `BEFORE_REMOVE_LIQUIDITY` | — |
| `afterModifyLiquidity` | `:209-245` | `AFTER_ADD_LIQUIDITY` / `AFTER_REMOVE_LIQUIDITY` (+ RETURNS_DELTA) | `(callerDelta, hookDelta)` |
| `beforeSwap` | `:248-282` | `BEFORE_SWAP` (+ RETURNS_DELTA) | `(amountToSwap, hookReturn, lpFeeOverride)` |
| `afterSwap` | `:285-315` | `AFTER_SWAP` (+ RETURNS_DELTA) | `(swapDelta, hookDelta)` |
| `beforeDonate` | `:318-325` | `BEFORE_DONATE` | — |
| `afterDonate` | `:328-335` | `AFTER_DONATE` | — |

**`beforeModifyLiquidity`** (`:195-206`) routes on sign: `liquidityDelta > 0` →
add, `<= 0` → remove. **A poke (delta 0) fires the *remove* hook**, which is
easy to miss.

**`afterModifyLiquidity`** (`:209-245`): computes `hookDelta` via
`callHookWithReturnDelta`, then `callerDelta = callerDelta - hookDelta` (`:230`,
`:242`). Whatever the hook takes, the caller pays.

**`beforeSwap`** (`:248-282`) is the richest:

```solidity
amountToSwap = params.amountSpecified;                                    // :252
if (msg.sender == address(self)) return (amountToSwap, ZERO_DELTA, 0);    // :253
if (self.hasPermission(BEFORE_SWAP_FLAG)) {
    bytes memory result = callHook(self, abi.encodeCall(IHooks.beforeSwap, ...));  // :256
    if (result.length != 96) InvalidHookResponse.selector.revertWith();   // :259
    if (key.fee.isDynamicFee()) lpFeeOverride = result.parseFee();        // :263
    if (self.hasPermission(BEFORE_SWAP_RETURNS_DELTA_FLAG)) {
        hookReturn = BeforeSwapDelta.wrap(result.parseReturnDelta());     // :267
        int128 hookDeltaSpecified = hookReturn.getSpecifiedDelta();       // :270
        if (hookDeltaSpecified != 0) {
            bool exactInput = amountToSwap < 0;
            amountToSwap += hookDeltaSpecified;                           // :275
            if (exactInput ? amountToSwap > 0 : amountToSwap < 0) {
                HookDeltaExceedsSwapAmount.selector.revertWith();         // :277
            }
        }
    }
}
```

96 bytes = `bytes4` + `int256` + `uint24`. The fee is read whenever the pool is
dynamic (`:263`) — independent of the RETURNS_DELTA bit. The specified delta
*shrinks the swap*, and the sign check (`:276`) stops a hook turning an
exact-input swap into an exact-output one. The unspecified half is untouched here
and carried forward to `afterSwap` (comment at `:269`).

**`afterSwap`** (`:285-315`): starts from the `beforeSwap` deltas (`:295-296`),
adds the `afterSwap` return into `hookDeltaUnspecified` (`:299-302`), then rotates
specified/unspecified into token0/token1 (`:307-309`) and subtracts from
`swapDelta` (`:312`).

Note the asymmetry: `beforeSwap` may move both the specified and unspecified
side; `afterSwap` may only move the unspecified side (its return is a single
`int128`, per `IHooks.sol:121`).

### 6.9 `hasPermission(IHooks self, uint160 flag) → bool` — `:337-339`

`uint160(address(self)) & flag != 0`. Cheap enough to call repeatedly, which the
library does.

### 6.10 The hook call sequence

```
swap()
  |
  +-- beforeSwap  ─ may: revert · shrink amount · take a delta · override the LP fee
  |
  +-- Pool.swap   ─ the actual AMM math, on the possibly-reduced amount
  |
  +-- emit Swap
  |
  +-- afterSwap   ─ may: revert · take a delta in the unspecified token
  |
  +-- _accountPoolBalanceDelta(hookDelta,  address(hooks))    <- hook's own ledger
  +-- _accountPoolBalanceDelta(swapDelta,  msg.sender)        <- caller's ledger
```

Crucially, the hook's delta lands on the **hook's** account. The hook must settle
it itself before `unlock` returns, or the whole transaction reverts with
`CurrencyNotSettled()`. A hook that takes a fee must therefore also `take` or
`mint` it.

---

## 7. Math libraries

### 7.1 `TickMath` — `libraries/TickMath.sol` (238 lines)

Converts between ticks and `sqrtPriceX96`, where price = `1.0001^tick` and the
stored value is `sqrt(price) · 2^96`.

**Errors:** `InvalidTick(int24)` (`:14`), `InvalidSqrtPrice(uint160)` (`:16`).

**Constants**

| Constant | Line | Value |
|---|---|---|
| `MIN_TICK` | `:20` | `-887272` |
| `MAX_TICK` | `:23` | `887272` |
| `MIN_TICK_SPACING` | `:26` | `1` |
| `MAX_TICK_SPACING` | `:28` | `type(int16).max` = `32767` |
| `MIN_SQRT_PRICE` | `:31` | `4295128739` |
| `MAX_SQRT_PRICE` | `:33` | `1461446703485210103287273052203988822378723970342` |
| `MAX_SQRT_PRICE_MINUS_MIN_SQRT_PRICE_MINUS_ONE` | `:35-36` | the difference, precomputed for a one-comparison range check |

`MIN_TICK`/`MAX_TICK` are `±log_1.0001(2^128)` rounded inward — the range over
which a `uint160` Q64.96 price is representable.

**Functions**

| Function | Line | Notes |
|---|---|---|
| `maxUsableTick(int24 tickSpacing) → int24` | `:39-43` | `(MAX_TICK / tickSpacing) * tickSpacing` |
| `minUsableTick(int24 tickSpacing) → int24` | `:46-50` | `(MIN_TICK / tickSpacing) * tickSpacing` |
| `getSqrtPriceAtTick(int24 tick) → uint160` | `:57-119` | binary decomposition, 19 magic constants |
| `getTickAtSqrtPrice(uint160) → int24` | `:121-237` | MSB + log₂ refinement, then a two-candidate check |

`getSqrtPriceAtTick` computes `1.0001^(tick/2)` by multiplying precomputed
`1.0001^(2^i / 2)` constants for each set bit of `|tick|`, then inverts for
negative ticks. `getTickAtSqrtPrice` computes `log_√1.0001(x)` and — because the
approximation has bounded error — derives `tickLow` and `tickHi` (comments at
`:227-231`) and returns `tickHi` only if `getSqrtPriceAtTick(tickHi)` still fits
under the input.

Both are `internal pure` and both revert on out-of-range input.

### 7.2 `SqrtPriceMath` — `libraries/SqrtPriceMath.sol` (289 lines)

The reserve math. All amounts derive from two identities:

```
amount0 = L · (√Pb − √Pa) / (√Pa · √Pb)
amount1 = L · (√Pb − √Pa)
```

**Errors:** `InvalidPriceOrLiquidity()` (`:15`), `InvalidPrice()` (`:16`),
`NotEnoughLiquidity()` (`:17`), `PriceOverflow()` (`:18`). All four are thrown
from hand-written assembly with hard-coded selectors rather than via
`CustomRevert`, for gas.

| Function | Line | Purpose |
|---|---|---|
| `getNextSqrtPriceFromAmount0RoundingUp(uint160,uint128,uint256,bool add)` | `:31-75` | new price after a token0 change |
| `getNextSqrtPriceFromAmount1RoundingDown(uint160,uint128,uint256,bool add)` | `:86-120` | new price after a token1 change |
| `getNextSqrtPriceFromInput(uint160,uint128,uint256 amountIn,bool zeroForOne)` | `:129-150` | dispatch by direction |
| `getNextSqrtPriceFromOutput(uint160,uint128,uint256 amountOut,bool zeroForOne)` | `:158-179` | dispatch by direction |
| `getAmount0Delta(uint160,uint160,uint128,bool roundUp)` | `:188-211` | unsigned token0 amount |
| `absDiff(uint160,uint160)` | `:214-225` | branchless \|a−b\| |
| `getAmount1Delta(uint160,uint160,uint128,bool roundUp)` | `:234-254` | unsigned token1 amount |
| `getAmount0Delta(uint160,uint160,int128)` | `:261-271` | **signed** helper |
| `getAmount1Delta(uint160,uint160,int128)` | `:278-288` | **signed** helper |

`getAmount0Delta` sorts its price arguments (`:194`), rejects a zero price
(`:197-202`), and computes
`L·2^96 · (√Pb−√Pa) / √Pb / √Pa` with the rounding mode threaded through both
divisions (`:207-209`).

`getAmount1Delta` (`:234-254`) is `mulDiv(L, |Δ√P|, Q96)` plus a branchless
round-up: `add(amount1, and(gt(mulmod(...), 0), roundUp))` (`:252`) — add one iff
there was a remainder *and* rounding up was requested.

`absDiff` (`:214-225`) is a neat branchless trick: `mask = sar(255, a-b)` is all
ones when `a < b`, and `res = mask ^ (mask + diff)` yields the absolute value
either way. The derivation is spelled out at `:220-222`.

**The signed helpers encode V4's rounding policy** (`:261-271`, `:278-288`):

```solidity
return liquidity < 0
    ? getAmount0Delta(a, b, uint128(-liquidity), false).toInt256()   // removing: round DOWN, positive
    : -getAmount0Delta(a, b, uint128(liquidity), true).toInt256();   // adding: round UP, negative
```

Adding liquidity rounds **up** and yields a **negative** delta (you owe more);
removing rounds **down** and yields a **positive** delta (you receive less).
Both directions favour the pool. Reversing either is a slow drain.

### 7.3 `SwapMath` — `libraries/SwapMath.sol` (108 lines)

**`MAX_SWAP_FEE = 1e6`** (`:12`) — 100% in pips. The natspec at `:11` stresses
this is the *total* fee, LP plus protocol.

**`getSqrtPriceTarget(bool zeroForOne, uint160 sqrtPriceNextX96, uint160 sqrtPriceLimitX96) → uint160`** — `:20-37`

Branchless `max` or `min` depending on direction. The XOR-swap idiom at `:33-35`:
`nextOrLimit` is a 0/1 selector, `symDiff` is `a ^ b`, and
`xor(limit, mul(symDiff, sel))` picks one of the two operands without a jump.

**`computeSwapStep(uint160 sqrtPriceCurrentX96, uint160 sqrtPriceTargetX96, uint128 liquidity, int256 amountRemaining, uint24 feePips) → (uint160 sqrtPriceNextX96, uint256 amountIn, uint256 amountOut, uint256 feeAmount)`** — `:51-107`

One step of the swap loop, entirely `unchecked`.

`zeroForOne = sqrtPriceCurrentX96 >= sqrtPriceTargetX96` (`:60`) — direction is
*inferred* from the prices, not passed in. `exactIn = amountRemaining < 0` (`:61`).

**Exact-input branch** (`:63-86`):

1. `amountRemainingLessFee = mulDiv(-amountRemaining, 1e6 - fee, 1e6)` (`:64-65`).
2. `amountIn` = the amount needed to reach the target (`:66-68`), rounded **up**.
3. If the fee-adjusted remainder covers it (`:69-74`): we reach the target.
   `feeAmount = mulDivRoundingUp(amountIn, fee, 1e6 - fee)` — note the
   denominator is `1e6 - fee`, because `amountIn` is already net of fee. The
   `fee == MAX_SWAP_FEE` special case at `:72-73` avoids a division by zero.
4. Otherwise (`:75-83`): the input is exhausted first. `amountIn` becomes the
   whole fee-adjusted remainder and `feeAmount` is **everything left over**
   (`:82`) — which is what guarantees `amountIn + feeAmount` exactly equals the
   user's input, with no dust stranded.
5. `amountOut` computed from the achieved price, rounded **down** (`:84-86`).

**Exact-output branch** (`:87-105`): mirror image. `amountOut` capped by the
remaining output (`:95-98`), `amountIn` rounded **up** (`:100-102`), and
`feeAmount = mulDivRoundingUp(amountIn, fee, 1e6 - fee)` unconditionally — the
comment at `:103` notes `feePips` cannot be `MAX_SWAP_FEE` here because
`Pool.swap` already rejected that combination (`Pool.sol:311-316`).

### 7.4 `TickBitmap` — `libraries/TickBitmap.sol` (122 lines)

A `mapping(int16 wordPos => uint256)` where bit `bitPos` of word `wordPos` marks
tick `(wordPos·256 + bitPos)·tickSpacing` as initialized.

**Error:** `TickMisaligned(int24,int24)` (`:13`).

**`compress(int24 tick, int24 tickSpacing) → int24`** — `:16-29`

`tick / tickSpacing`, rounding toward **negative infinity**. EVM `sdiv`
truncates toward zero, so the correction `sub(sdiv(...), slt(smod(...), 0))`
subtracts one when the remainder is negative.

**`position(int24 tick) → (int16 wordPos, uint8 bitPos)`** — `:35-41`

`wordPos = sar(8, tick)` (arithmetic, so negative ticks map to negative words),
`bitPos = tick & 0xff`.

**`flipTick(mapping storage self, int24 tick, int24 tickSpacing)`** — `:47-75`

Reverts `TickMisaligned` if `tick % tickSpacing != 0` (`:57-63`), then computes
the storage slot as `keccak256(abi.encode(wordPos, self.slot))` inline (`:67-70`)
and XORs the bit (`:73`). Doing the slot derivation by hand saves the compiler's
generic mapping-access overhead.

**`nextInitializedTickWithinOneWord(mapping storage self, int24 tick, int24 tickSpacing, bool lte) → (int24 next, bool initialized)`** — `:85-121`

Searches within one 256-bit word.

- `lte = true` (zeroForOne, searching down, `:94-105`): mask keeps bits at or
  right of `bitPos`; `mostSignificantBit(masked)` finds the nearest.
- `lte = false` (searching up, `:106-118`): starts from `++compressed` because
  the current tick does not matter (comment at `:107`); mask keeps bits at or
  left of `bitPos`; `leastSignificantBit(masked)`.

If no bit is set, it returns the word boundary with `initialized = false` — the
caller loops again from there. This is why `Pool.swap` may iterate several times
without crossing anything.

### 7.5 `BitMath` — `libraries/BitMath.sol` (49 lines)

`mostSignificantBit(uint256) → uint8` (`:12-25`) and
`leastSignificantBit(uint256) → uint8` (`:31-48`). Both are branchless binary
searches over 8 halving steps. Both assume `x != 0`; the callers guarantee it via
the `initialized` check.

### 7.6 `FullMath` — `libraries/FullMath.sol` (117 lines)

**`mulDiv(uint256 a, uint256 b, uint256 denominator) → uint256`** — `:14-107`

Full 512-bit `a·b/denominator` without intermediate overflow, using Remco
Bloemen's algorithm: compute the 512-bit product as two words via `mulmod`,
divide out the power of two, then multiply by the modular inverse of the odd part
(computed by Newton iteration, doubling correct bits each step). Reverts on
overflow or division by zero.

**`mulDivRoundingUp(uint256, uint256, uint256) → uint256`** — `:109-116`

`mulDiv` plus one if `mulmod(a,b,denominator) > 0`.

### 7.7 `LiquidityMath` — `libraries/LiquidityMath.sol` (19 lines)

**`addDelta(uint128 x, int128 y) → uint128 z`** — `:10-19`

```solidity
z := add(and(x, 0xffff…ffff), signextend(15, y))
if shr(128, z) { mstore(0, 0x93dafdf1); revert(0x1c, 0x04) }   // SafeCastOverflow()
```

One add and one shift catches both overflow and underflow: a negative result
wraps into the high bits, so `shr(128, z) != 0` either way. Note it reverts with
`SafeCast`'s selector even though it is a different library.

### 7.8 `SafeCast` — `libraries/SafeCast.sol` (60 lines)

**Error:** `SafeCastOverflow()` (`:11`), selector `0x93dafdf1`.

| Function | Line | Check |
|---|---|---|
| `toUint160(uint256) → uint160` | `:16-19` | round-trip equality |
| `toUint128(uint256) → uint128` | `:24-27` | round-trip equality |
| `toUint128(int128) → uint128` | `:32-35` | `x < 0` rejected |
| `toInt128(int256) → int128` | `:40-43` | round-trip equality |
| `toInt256(uint256) → int256` | `:48-51` | result must be `≥ 0` |
| `toInt128(uint256) → int128` | `:56-59` | `x >= 2^127` rejected |

`PoolManager` does `using SafeCast for *` (`:81`), so these attach to every
numeric type in scope.

### 7.9 `UnsafeMath` — `libraries/UnsafeMath.sol` (28 lines)

`divRoundingUp(uint256,uint256)` (`:12-16`) and
`simpleMulDiv(uint256,uint256,uint256)` (`:24-28`). Neither checks overflow;
both return 0 on division by zero rather than reverting, which the natspec flags
at `:8` and `:19` as the caller's problem.

### 7.10 `FixedPoint96` / `FixedPoint128`

`FixedPoint96.RESOLUTION = 96` and `Q96 = 2^96` (`FixedPoint96.sol:8-9`);
`FixedPoint128.Q128 = 2^128` (`FixedPoint128.sol:7`). Prices are Q64.96; fee
growth is Q128.128.

---

## 8. Fees

Two independent fees ride on every swap. The **protocol fee** is taken from the
input first; the **LP fee** is taken from what remains.

### 8.1 `LPFeeLibrary` — `libraries/LPFeeLibrary.sol` (79 lines)

**Error:** `LPFeeTooLarge(uint24)` (`:12`).

| Constant | Line | Value | Meaning |
|---|---|---|---|
| `DYNAMIC_FEE_FLAG` | `:15` | `0x800000` | in `PoolKey.fee`: this pool has a dynamic fee |
| `OVERRIDE_FEE_FLAG` | `:19` | `0x400000` | in a `beforeSwap` return: override this swap's fee |
| `REMOVE_OVERRIDE_MASK` | `:22` | `0xBFFFFF` | clears bit 22 |
| `MAX_LP_FEE` | `:25` | `1_000_000` | 100% in pips |

| Function | Line | Behaviour |
|---|---|---|
| `isDynamicFee(uint24) → bool` | `:30-32` | `self == 0x800000` **exactly** |
| `isValid(uint24) → bool` | `:37-39` | `self <= 1_000_000` |
| `validate(uint24)` | `:43-45` | reverts `LPFeeTooLarge` |
| `getInitialLPFee(uint24) → uint24` | `:51-56` | dynamic → 0, else validate and return |
| `isOverride(uint24) → bool` | `:61-63` | bit 22 set |
| `removeOverrideFlag(uint24) → uint24` | `:68-70` | `self & 0xBFFFFF` |
| `removeOverrideFlagAndValidate(uint24) → uint24` | `:75-78` | both |

`isDynamicFee` requires exact equality, not just the high bit — `0x800001` is
**not** a dynamic-fee marker, it is an invalid static fee that
`getInitialLPFee` will reject as `> 1e6`.

A dynamic pool starts at fee 0 (`:52-53`, and the natspec at `:48` points you at
`updateDynamicLPFee` in `afterInitialize`).

### 8.2 `ProtocolFeeLibrary` — `libraries/ProtocolFeeLibrary.sol` (47 lines)

The 24-bit `protocolFee` is two 12-bit halves: **low 12 bits = 0→1 direction,
high 12 bits = 1→0**.

| Constant | Line | Value |
|---|---|---|
| `MAX_PROTOCOL_FEE` | `:8` | `1000` (0.1%) |
| `FEE_0_THRESHOLD` | `:11` | `1001` |
| `FEE_1_THRESHOLD` | `:12` | `1001 << 12` |
| `PIPS_DENOMINATOR` | `:15` | `1_000_000` |

The comment at `:7` warns that raising `MAX_PROTOCOL_FEE` could overflow
`Pool.swap`.

| Function | Line | Behaviour |
|---|---|---|
| `getZeroForOneFee(uint24) → uint16` | `:17-19` | `self & 0xfff` |
| `getOneForZeroFee(uint24) → uint16` | `:21-23` | `self >> 12` |
| `isValidProtocolFee(uint24) → bool` | `:25-32` | both halves `≤ 1000`, via two masked comparisons |
| `calculateSwapFee(uint16 self, uint24 lpFee) → uint24` | `:38-46` | composition |

**`calculateSwapFee`** implements
`protocolFee + lpFee·(1e6 − protocolFee)/1e6`, rearranged (`:39`) to
`protocolFee + lpFee − protocolFee·lpFee/1e6`. Because the protocol takes its cut
first, the LP fee applies only to what is left, and the total is always below the
naive sum.

### 8.3 `ProtocolFees` contract — `src/ProtocolFees.sol` (71 lines)

`abstract contract ProtocolFees is IProtocolFees, Owned` (`:15`).

**Storage:** `protocolFeesAccrued` (`:21`, slot 1),
`protocolFeeController` (`:24`, slot 2).

**Constructor** `:26` — forwards `initialOwner` to solmate `Owned`.

| Function | Line | Access | Notes |
|---|---|---|---|
| `setProtocolFeeController(address)` | `:29-32` | `onlyOwner` | emits `ProtocolFeeControllerUpdated` |
| `setProtocolFee(PoolKey,uint24)` | `:35-41` | controller only | validates, writes `slot0`, emits `ProtocolFeeUpdated` |
| `collectProtocolFees(address,Currency,uint256)` | `:44-57` | controller only | `amount == 0` means "all" |
| `_updateProtocolFees(Currency,uint256)` | `:66-70` | internal | `unchecked` accrual |

`setProtocolFee` reverts `InvalidCaller()` (`:36`) or
`ProtocolFeeTooLarge(uint24)` (`:37`), and `PoolNotInitialized()` via
`Pool.setProtocolFee`.

**`collectProtocolFees` has a subtle guard** (`:49-52`):

```solidity
if (!currency.isAddressZero() && CurrencyReserves.getSyncedCurrency() == currency) {
    ProtocolFeeCurrencySynced.selector.revertWith();
}
```

Collecting a currency that is currently synced would transfer tokens out between
the `sync` snapshot and the `settle` measurement, corrupting someone's balance
difference. Native is exempt because native settlement uses `msg.value`.

**Two abstract hooks** the manager must implement: `_isUnlocked()` (`:60`) and
`_getPool(PoolId)` (`:64`).

---

## 9. ERC-6909 claims

### 9.1 `ERC6909` — `src/ERC6909.sol` (90 lines)

Solmate's implementation, copied at commit `4b47a19` (`:8`) and modified.
`abstract contract ERC6909 is IERC6909Claims`.

**Storage** — slots 3, 4, 5 of `PoolManager`:

```solidity
mapping(address owner => mapping(address operator => bool)) public isOperator;                       // :15
mapping(address owner => mapping(uint256 id => uint256)) public balanceOf;                           // :17
mapping(address owner => mapping(address spender => mapping(uint256 id => uint256))) public allowance; // :19
```

| Function | Line | Notes |
|---|---|---|
| `transfer(address,uint256,uint256) → bool` | `:25-33` | no zero-address check |
| `transferFrom(address,address,uint256,uint256) → bool` | `:35-48` | operator bypasses allowance; `type(uint256).max` is infinite |
| `approve(address,uint256,uint256) → bool` | `:50-56` | |
| `setOperator(address,bool) → bool` | `:58-64` | blanket approval for all ids |
| `supportsInterface(bytes4) → bool` | `:70-73` | `0x01ffc9a7` (ERC-165), `0x0f632fb3` (ERC-6909) |
| `_mint(address,uint256,uint256)` | `:79-83` | internal |
| `_burn(address,uint256,uint256)` | `:85-89` | internal |

Underflow on an insufficient balance is caught by Solidity's checked arithmetic
(`:26`, `:41`), not by an explicit require.

### 9.2 `ERC6909Claims` — `src/ERC6909Claims.sol` (23 lines)

Adds exactly one function.

**`_burnFrom(address from, uint256 id, uint256 amount) internal`** — `:13-22`

If `from != msg.sender` and the sender is not an operator, decrement the
allowance (unless infinite), then `_burn`. This is what `PoolManager.burn`
(`:335`) calls, and it is why burning someone else's claims requires approval.

### 9.3 Why claims exist

A claim token is a receipt for currency held inside the manager. Instead of

```
take(USDC, me, 1000)   →  ERC-20 transfer out
... next transaction ...
sync(USDC); transfer 1000 in; settle()   →  ERC-20 transfer in
```

you can `mint` claims and `burn` them later, skipping both transfers. For a
market maker doing many round trips this removes the dominant gas cost. The
tokens themselves never leave the manager; only the internal ledger changes.

---

## 10. State reading: Extsload, Exttload, StateLibrary

Because all pools share one contract, V4 has no per-pool getters. Instead the
manager exposes raw storage and a library decodes it.

### 10.1 `Extsload` — `src/Extsload.sol` (64 lines)

Three overloads, all `external view`, all pure assembly that `return`s directly
without going through Solidity's ABI encoder.

| Overload | Line | Selector |
|---|---|---|
| `extsload(bytes32 slot) → bytes32` | `:10-15` | `0x1e2eaeaf` |
| `extsload(bytes32 startSlot, uint256 nSlots) → bytes32[]` | `:18-39` | `0x35fd631a` |
| `extsload(bytes32[] calldata slots) → bytes32[]` | `:42-63` | `0xdbd035ff` |

The array versions hand-build the ABI response: offset `0x20`, then length, then
the words (`:25-27`, `:47-49`).

> **Note:** both loops are do-while shaped (`for {} 1 {}` with the break at the
> end), so calling the range overload with `nSlots = 0` still reads one slot. Not
> a safety problem, but it is not a no-op either.

### 10.2 `Exttload` — `src/Exttload.sol` (40 lines)

Same idea for transient storage: `exttload(bytes32)` (`:10-15`, `0xf135baaa`) and
`exttload(bytes32[])` (`:18-39`, `0x9bf6645f`). There is no range overload,
because transient slots here are hash-derived and never contiguous.

### 10.3 `StateLibrary` — `libraries/StateLibrary.sol` (349 lines)

Knows the layout and does the arithmetic.

**Layout constants**

| Constant | Line | Value |
|---|---|---|
| `POOLS_SLOT` | `:11` | `6` |
| `FEE_GROWTH_GLOBAL0_OFFSET` | `:14` | `1` (global1 is 2, noted at `:16`) |
| `LIQUIDITY_OFFSET` | `:19` | `3` |
| `TICKS_OFFSET` | `:22` | `4` |
| `TICK_BITMAP_OFFSET` | `:25` | `5` |
| `POSITIONS_OFFSET` | `:28` | `6` |

**Slot derivation helpers**

| Function | Line | Formula |
|---|---|---|
| `_getPoolStateSlot(PoolId)` | `:324-326` | `keccak256(abi.encodePacked(poolId, POOLS_SLOT))` |
| `_getTickInfoSlot(PoolId,int24)` | `:328-337` | `keccak256(abi.encodePacked(int256(tick), stateSlot + 4))` |
| `_getPositionInfoSlot(PoolId,bytes32)` | `:339-348` | `keccak256(abi.encodePacked(positionId, stateSlot + 6))` |

**Getters**

| Function | Line | Reads |
|---|---|---|
| `getSlot0(IPoolManager,PoolId)` | `:40-63` | 1 slot → `(sqrtPriceX96, tick, protocolFee, lpFee)` |
| `getTickInfo(IPoolManager,PoolId,int24)` | `:76-97` | 3 slots → full `TickInfo` |
| `getTickLiquidity(IPoolManager,PoolId,int24)` | `:108-120` | 1 slot → gross + net |
| `getTickFeeGrowthOutside(IPoolManager,PoolId,int24)` | `:131-144` | 2 slots, offset by 1 |
| `getFeeGrowthGlobals(IPoolManager,PoolId)` | `:157-174` | 2 slots |
| `getLiquidity(IPoolManager,PoolId)` | `:183-191` | 1 slot |
| `getTickBitmap(IPoolManager,PoolId,int16)` | `:201-216` | 1 slot |
| `getPositionInfo(…,address,int24,int24,bytes32)` | `:230-242` | derives the key, then delegates |
| `getPositionInfo(…,bytes32 positionId)` | `:254-269` | 3 slots |
| `getPositionLiquidity(…,bytes32)` | `:279-286` | 1 slot |
| `getFeeGrowthInside(…,int24,int24)` | `:298-322` | composes 5 reads |

`getSlot0` (`:53-62`) repeats the `Slot0` bit layout by hand, with the shifts
160/184/208 matching `Slot0Library`. The ASCII diagram at `:50-52` is a useful
cross-check.

`getFeeGrowthInside` (`:298-322`) duplicates `Pool.getFeeGrowthInside`'s
three-case logic, and its natspec (`:290`) flags why it exists: the value cached
in `Position.State` goes stale, so an external reader that wants *current*
uncollected fees must recompute.

> Every one of these getters is a **plain `view` call on live storage**, so it is
> exactly as manipulable as spot price. Do not use `getSlot0().sqrtPriceX96` as an
> oracle.

---

## 11. Utility contracts and libraries

### 11.1 `CustomRevert` — `libraries/CustomRevert.sol` (120 lines)

Reverting with `revert MyError(x)` makes Solidity allocate memory and ABI-encode.
This library writes the selector and arguments straight to memory and reverts,
which is measurably cheaper at the thousands of call sites V4 has. The usage
convention is in the natspec at `:6-7`: `using CustomRevert for bytes4;` then
`MyError.selector.revertWith(arg)`.

The disclaimer at `:8` matters: these functions **may clobber the free memory
pointer**, which is fine only because the call context exits immediately.

| Overload | Line | Payload |
|---|---|---|
| `revertWith(bytes4)` | `:14-19` | selector only |
| `revertWith(bytes4,address)` | `:22-28` | masked address |
| `revertWith(bytes4,int24)` | `:31-37` | sign-extended |
| `revertWith(bytes4,uint160)` | `:40-46` | masked |
| `revertWith(bytes4,int24,int24)` | `:49-57` | two ticks |
| `revertWith(bytes4,uint160,uint160)` | `:60-68` | two prices |
| `revertWith(bytes4,address,address)` | `:71-79` | two addresses |

**`bubbleUpAndRevertWith(address revertingContract, bytes4 revertingFunctionSelector, bytes4 additionalContext)`** — `:83-119`

Implements **ERC-7751**: wraps a failed sub-call's revert data in
`WrappedError(address target, bytes4 selector, bytes reason, bytes details)`
(`:11`, selector `0x90bfb865`), so tooling can see both *what* failed and *why*.
Used by `Currency.transfer` (`:52`, `:83`) and `Hooks.callHook` (`:137`).

The natspec at `:82` is candid: **"this method can be vulnerable to revert data
bombs."** A malicious hook can return megabytes of revert data, and
`returndatacopy` at `:109` will copy all of it, burning the caller's gas.

### 11.2 `ParseBytes` — `libraries/ParseBytes.sol` (29 lines)

Reads fields out of hook return data without `abi.decode`'s bounds checks.

| Function | Line | Offset | Reads |
|---|---|---|---|
| `parseSelector(bytes) → bytes4` | `:9-14` | `+0x20` | first word |
| `parseFee(bytes) → uint24` | `:16-21` | `+0x60` | third word |
| `parseReturnDelta(bytes) → int256` | `:23-28` | `+0x40` | second word |

Safe only because `Hooks` checks the length first — 32 for a plain selector, 64
for selector+delta, 96 for selector+delta+fee (`Hooks.sol:152`, `:166`, `:259`).

### 11.3 `NoDelegateCall` — `src/NoDelegateCall.sol` (33 lines)

**Error:** `DelegateCallNotAllowed()` (`:11`).

Stores `address private immutable original = address(this)` in the constructor
(`:16-20`) and compares at runtime (`:24-26`). The comment at `:22-23` explains
why the check lives in a `private` function rather than inline in the modifier:
modifiers are copied into every function they decorate, and an immutable is
inlined as literal bytes, so inlining would duplicate the 20-byte address at
every site.

Applied to `initialize`, `modifyLiquidity`, `swap`, `donate`. Deliberately **not**
applied to `unlock`, `sync`, `take`, `settle`, `mint`, `burn`.

---

## 12. Interfaces

### 12.1 `IPoolManager` — `interfaces/IPoolManager.sol` (217 lines)

`interface IPoolManager is IProtocolFees, IERC6909Claims, IExtsload, IExttload` (`:16`).

**Errors (9)** — `:18-49`: `CurrencyNotSettled`, `PoolNotInitialized`,
`AlreadyUnlocked`, `ManagerLocked`, `TickSpacingTooLarge(int24)`,
`TickSpacingTooSmall(int24)`, `CurrenciesOutOfOrderOrEqual(address,address)`,
`UnauthorizedDynamicLPFeeUpdate`, `SwapAmountCannotBeZero`, `NonzeroNativeValue`,
`MustClearExactPositiveDelta`.

> `PoolNotInitialized` is declared here (`:21`) *and* in `Pool` (`Pool.sol:50`).
> Same name, same selector `0x486aa307`; the one actually thrown comes from
> `Pool.checkPoolInitialized`.

**Events (4)** — `Initialize` (`:60`), `ModifyLiquidity` (`:78`), `Swap` (`:91`),
`Donate` (`:107`).

**Functions (13)** — `unlock`, `initialize`, `modifyLiquidity`, `swap`, `donate`,
`sync`, `take`, `settle`, `settleFor`, `clear`, `mint`, `burn`,
`updateDynamicLPFee`.

### 12.2 `IHooks` — `interfaces/IHooks.sol` (152 lines)

The ten callbacks. Every one returns its own selector as the first value.

| Callback | Line | Extra return |
|---|---|---|
| `beforeInitialize(address,PoolKey,uint160)` | `:21` | — |
| `afterInitialize(address,PoolKey,uint160,int24)` | `:29-31` | — |
| `beforeAddLiquidity(address,PoolKey,ModifyLiquidityParams,bytes)` | `:39-44` | — |
| `afterAddLiquidity(address,PoolKey,ModifyLiquidityParams,BalanceDelta,BalanceDelta,bytes)` | `:55-62` | `BalanceDelta` |
| `beforeRemoveLiquidity(...)` | `:70-75` | — |
| `afterRemoveLiquidity(...)` | `:86-93` | `BalanceDelta` |
| `beforeSwap(address,PoolKey,SwapParams,bytes)` | `:103-105` | `BeforeSwapDelta`, `uint24` |
| `afterSwap(address,PoolKey,SwapParams,BalanceDelta,bytes)` | `:115-121` | `int128` |
| `beforeDonate(address,PoolKey,uint256,uint256,bytes)` | `:130-136` | — |
| `afterDonate(address,PoolKey,uint256,uint256,bytes)` | `:145-151` | — |

`sender` is the address that called the *manager* (the router), not the end user.
The natspec at `:14` states hooks "should only be callable by the v4
PoolManager" — that is **advice, not enforcement**. Nothing in core stops anyone
from calling your hook directly; you must add the check yourself (see
`FeeTakingHook`'s `onlyPoolManager` modifier, `test/FeeTakingHook.sol:25-28`).

The `beforeSwap` fee-override contract is spelled out at `:102`: three conditions
— dynamic pool, bit `0x400000` set, value `≤ 1e6`.

### 12.3 Remaining interfaces

- **`IProtocolFees`** (52 lines): 3 errors, 2 events, 5 functions. Covered in §8.3.
- **`IExtsload`** / **`IExttload`**: the raw-read overloads. §10.
- **`IUnlockCallback`** (10 lines): a single `unlockCallback(bytes) → bytes`
  (`:9`), selector `0x91dd7346`.
- **`IERC6909Claims`** (66 lines): events `OperatorSet` (`:10`), `Approval`
  (`:12`), `Transfer` (`:14`); views `balanceOf`, `allowance`, `isOperator`;
  mutators `transfer`, `transferFrom`, `approve`, `setOperator`.
- **`IERC20Minimal`** (48 lines): `balanceOf`, `transfer`, `allowance`, `approve`,
  `transferFrom`, plus `Transfer`/`Approval` events. V4 calls only `balanceOf`
  and `transfer` — it never calls `transferFrom`, because tokens arrive by the
  caller sending them before `settle`.

---

## 13. Test helpers

38 contracts under `src/test/`. They ship in `src/` (not `test/`) so downstream
projects can import them. They are also the best worked examples in the repo.

### 13.1 Routers — how to actually call the manager

| File | Lines | What it demonstrates |
|---|---|---|
| `PoolTestBase.sol` | 31 | Abstract base: holds `manager`, `_fetchBalances` reads user balance, pool balance and delta together (`:22-30`). |
| `PoolSwapTest.sol` | 117 | The canonical swap router. `swap()` wraps `manager.unlock` (`:41`); `unlockCallback` asserts deltas start at zero (`:56-57`), swaps (`:59`), then settles/takes. `TestSettings` (`:30-33`) toggles claims vs transfers. |
| `PoolModifyLiquidityTest.sol` | 96 | Add/remove liquidity through `unlock`. |
| `PoolModifyLiquidityTestNoChecks.sol` | 74 | Same without the delta assertions, for fuzzing invalid states. |
| `PoolDonateTest.sol` | 68 | `donate` round trip. |
| `PoolTakeTest.sol` | 61 | `take` in isolation. |
| `PoolClaimsTest.sol` | 50 | Settling with ERC-6909 `mint`/`burn` instead of transfers. |
| `SwapRouterNoChecks.sol` | 49 | Minimal swap router, no assertions. |
| `PoolEmptyUnlockTest.sol` | 25 | `unlock` with an empty callback — proves the lock/unlock cycle in isolation. |
| `PoolNestedActionsTest.sol` | 236 | Re-entrant action sequences; proves `AlreadyUnlocked` and that nested calls share one delta ledger. |
| `ActionsRouter.sol` | 170 | Generic action-list executor: encode a list of actions and run them in one `unlock`. The most flexible harness. |

Read `PoolSwapTest.unlockCallback` (`:48-116`) before writing your own router. Its
per-direction, per-exactness assertions (`:64-96`) encode exactly what sign each
delta should have.

### 13.2 Example hooks

| File | Lines | Demonstrates |
|---|---|---|
| `BaseTestHooks.sol` | 109 | `IHooks` implementation where every callback reverts `HookNotImplemented`. Inherit and override only what you need. |
| `EmptyTestHooks.sol` | 119 | All ten callbacks implemented as no-ops returning their selector. |
| `MockHooks.sol` | 140 | Records calls and lets tests set arbitrary return values, including wrong selectors. |
| `FeeTakingHook.sol` | 91 | `afterSwap` skims 1.23% of the unspecified token (`:30-31`) and `mint`s itself claims. The clean template for a fee hook. |
| `LPFeeTakingHook.sol` | 65 | Takes 5.43% of LP fees in `afterAddLiquidity`/`afterRemoveLiquidity`. |
| `DeltaReturningHook.sol` | 100 | Returns configurable deltas from `beforeSwap`/`afterSwap`; the test bed for the return-delta machinery. |
| `CustomCurveHook.sol` | 71 | **Replaces the AMM entirely.** `beforeSwap` returns a delta that consumes the whole swap amount, so `Pool.swap` runs on zero and the hook's own pricing applies. This is how you build a non-`x·y=k` curve on V4. |
| `DynamicFeesTestHook.sol` | 40 | Calls `updateDynamicLPFee` to change the stored fee. |
| `DynamicReturnFeeTestHook.sol` | 39 | Returns a per-swap fee override from `beforeSwap` with the `0x400000` flag. |
| `SkipCallsTestHook.sol` | 193 | Calls back into the manager from inside a hook — exercises the `noSelfCall` modifier. |

### 13.3 Test tokens and infrastructure

| File | Lines | Purpose |
|---|---|---|
| `TestERC20.sol` | 55 | Standard `IERC20Minimal` token. |
| `TestInvalidERC20.sol` | 56 | Returns `false` instead of reverting — proves `Currency.transfer`'s return-data handling. |
| `NativeERC20.sol` | 50 | Simulates a chain where native balance is also an ERC-20 (Celo-style). |
| `MockERC6909Claims.sol` | 22 | Exposes `_mint`/`_burnFrom` for direct testing. |
| `MockContract.sol` | 57 | Counts calls by selector and optionally proxies onward. |
| `EmptyRevertContract.sol` | 9 | Reverts with no data — tests `bubbleUpAndRevertWith` on empty returndata. |
| `ProxyPoolManager.sol` | 222 | A `PoolManager` that `delegatecall`s a real one, used to prove `NoDelegateCall` actually fires. |
| `ProtocolFeesImplementation.sol` | 38 | Concrete subclass of the abstract `ProtocolFees`. |
| `NoDelegateCallTest.sol` | 32 | Positive and negative cases for the modifier. |
| `Fuzzers.sol` | 184 | `StdUtils` helpers that bound fuzzed liquidity params into valid ranges. |
| `CurrencyTest.sol` | 30 | Wraps `CurrencyLibrary` internals for direct testing. |
| `HooksTest.sol` | 73 | Wraps `Hooks` library internals. |
| `LiquidityMathTest.sol` | 10 | Wraps `addDelta`. |
| `TickMathTest.sol` | 42 | Wraps `TickMath` with gas measurement. |

### 13.4 Echidna property tests

| File | Lines | Properties |
|---|---|---|
| `SqrtPriceMathEchidnaTest.sol` | 198 | Price/amount monotonicity and rounding invariants. |
| `TickMathEchidnaTest.sol` | 22 | `getTickAtSqrtPrice(getSqrtPriceAtTick(t)) == t`. |
| `TickOverflowSafetyEchidnaTest.sol` | 89 | `liquidityGross`/`liquidityNet` cannot overflow across tick updates. |

Config is `echidna.config.yml` at the repo root.

---

## 14. Reference tables

### 14.1 `PoolManager` external ABI

Selectors recomputed with `cast sig` using fully-resolved types
(`PoolKey` → `(address,address,uint24,int24,address)`,
`ModifyLiquidityParams` → `(int24,int24,int256,bytes32)`,
`SwapParams` → `(bool,int256,uint160)`, `Currency` → `address`).

| Selector | Signature | Guards |
|---|---|---|
| `0x48c89491` | `unlock(bytes)` | not already unlocked |
| `0x6276cbbe` | `initialize((address,address,uint24,int24,address),uint160)` | `noDelegateCall` |
| `0x5a6bcfda` | `modifyLiquidity((address,address,uint24,int24,address),(int24,int24,int256,bytes32),bytes)` | `onlyWhenUnlocked`, `noDelegateCall` |
| `0xf3cd914c` | `swap((address,address,uint24,int24,address),(bool,int256,uint160),bytes)` | `onlyWhenUnlocked`, `noDelegateCall` |
| `0x234266d7` | `donate((address,address,uint24,int24,address),uint256,uint256,bytes)` | `onlyWhenUnlocked`, `noDelegateCall` |
| `0xa5841194` | `sync(address)` | none |
| `0x0b0d9c09` | `take(address,address,uint256)` | `onlyWhenUnlocked` |
| `0x11da60b4` | `settle()` | `onlyWhenUnlocked`, `payable` |
| `0x3dd45adb` | `settleFor(address)` | `onlyWhenUnlocked`, `payable` |
| `0x80f0b44c` | `clear(address,uint256)` | `onlyWhenUnlocked` |
| `0x156e29f6` | `mint(address,uint256,uint256)` | `onlyWhenUnlocked` |
| `0xf5298aca` | `burn(address,uint256,uint256)` | `onlyWhenUnlocked` |
| `0x52759651` | `updateDynamicLPFee((address,address,uint24,int24,address),uint24)` | dynamic pool + caller is the hook |
| `0x7e87ce7d` | `setProtocolFee((address,address,uint24,int24,address),uint24)` | controller |
| `0x2d771389` | `setProtocolFeeController(address)` | `onlyOwner` |
| `0x8161b874` | `collectProtocolFees(address,address,uint256)` | controller |
| `0x97e8cd4e` | `protocolFeesAccrued(address)` | view |
| `0xf02de3b2` | `protocolFeeController()` | view |
| `0x1e2eaeaf` | `extsload(bytes32)` | view |
| `0x35fd631a` | `extsload(bytes32,uint256)` | view |
| `0xdbd035ff` | `extsload(bytes32[])` | view |
| `0xf135baaa` | `exttload(bytes32)` | view |
| `0x9bf6645f` | `exttload(bytes32[])` | view |
| `0x8da5cb5b` | `owner()` | view (solmate `Owned`) |
| `0xf2fde38b` | `transferOwnership(address)` | `onlyOwner` |

**ERC-6909 surface**

| Selector | Signature |
|---|---|
| `0x00fdd58e` | `balanceOf(address,uint256)` |
| `0x598af9e7` | `allowance(address,address,uint256)` |
| `0xb6363cf2` | `isOperator(address,address)` |
| `0x095bcdb6` | `transfer(address,uint256,uint256)` |
| `0xfe99049a` | `transferFrom(address,address,uint256,uint256)` |
| `0x426a8493` | `approve(address,uint256,uint256)` |
| `0x558a7297` | `setOperator(address,bool)` |
| `0x01ffc9a7` | `supportsInterface(bytes4)` |

**Callbacks you may need to implement**

| Selector | Signature | Implemented by |
|---|---|---|
| `0x91dd7346` | `unlockCallback(bytes)` | anyone calling `unlock` |
| `0xdc98354e` | `beforeInitialize(address,PoolKey,uint160)` | hooks |
| `0x6fe7e6eb` | `afterInitialize(address,PoolKey,uint160,int24)` | hooks |
| `0x259982e5` | `beforeAddLiquidity(address,PoolKey,ModifyLiquidityParams,bytes)` | hooks |
| `0x9f063efc` | `afterAddLiquidity(address,PoolKey,ModifyLiquidityParams,int256,int256,bytes)` | hooks |
| `0x21d0ee70` | `beforeRemoveLiquidity(address,PoolKey,ModifyLiquidityParams,bytes)` | hooks |
| `0x6c2bbe7e` | `afterRemoveLiquidity(address,PoolKey,ModifyLiquidityParams,int256,int256,bytes)` | hooks |
| `0x575e24b4` | `beforeSwap(address,PoolKey,SwapParams,bytes)` | hooks |
| `0xb47b2fb1` | `afterSwap(address,PoolKey,SwapParams,int256,bytes)` | hooks |
| `0xb6a8b0fa` | `beforeDonate(address,PoolKey,uint256,uint256,bytes)` | hooks |
| `0xe1b4af69` | `afterDonate(address,PoolKey,uint256,uint256,bytes)` | hooks |

### 14.2 Events

| Topic0 | Event | Declared |
|---|---|---|
| `0x7c67cd8e…4e82` | `Initialize(PoolId,Currency,Currency,uint24,int24,IHooks,uint160,int24)` | `IPoolManager.sol:60` |
| `0xd97f8255…634c` | `ModifyLiquidity(PoolId,address,int24,int24,int256,bytes32)` | `IPoolManager.sol:78` |
| `0xe5db5196…f46c` | `Swap(PoolId,address,int128,int128,uint160,uint128,int24,uint24)` | `IPoolManager.sol:91` |
| `0x49e03fde…f33a` | `Donate(PoolId,address,uint256,uint256)` | `IPoolManager.sol:107` |
| `0xb4bd8ef5…8acc` | `ProtocolFeeControllerUpdated(address)` | `IProtocolFees.sol:20` |
| `0x06236e70…c862` | `ProtocolFeeUpdated(PoolId,uint24)` | `IProtocolFees.sol:23` |
| `0xceb576d9…a267` | `OperatorSet(address,address,bool)` | `IERC6909Claims.sol:10` |
| `0xb3fd5071…e9a7` | `Approval(address,address,uint256,uint256)` | `IERC6909Claims.sol:12` |
| `0x1b3d7edb…8859` | `Transfer(address,address,address,uint256,uint256)` | `IERC6909Claims.sol:14` |

`Initialize` is the only place a `PoolKey` is ever published, since keys are not
stored. Indexers must capture it or they cannot reconstruct pool identities.

### 14.3 Complete custom-error table

All 41 errors in the non-test sources, with verified selectors.

| Selector | Error | Declared in | Thrown when |
|---|---|---|---|
| `0x5212cba1` | `CurrencyNotSettled()` | `IPoolManager.sol:18` | deltas nonzero at end of `unlock` |
| `0x486aa307` | `PoolNotInitialized()` | `IPoolManager.sol:21`, `Pool.sol:50` | pool has no price |
| `0x5090d6c6` | `AlreadyUnlocked()` | `IPoolManager.sol:24` | nested `unlock` |
| `0x54e3ca0d` | `ManagerLocked()` | `IPoolManager.sol:27` | value-moving call outside `unlock` |
| `0xb70024f8` | `TickSpacingTooLarge(int24)` | `IPoolManager.sol:30` | `tickSpacing > 32767` |
| `0xe9e90588` | `TickSpacingTooSmall(int24)` | `IPoolManager.sol:33` | `tickSpacing < 1` |
| `0x6e6c9830` | `CurrenciesOutOfOrderOrEqual(address,address)` | `IPoolManager.sol:36` | `currency0 >= currency1` |
| `0x30d21641` | `UnauthorizedDynamicLPFeeUpdate()` | `IPoolManager.sol:40` | non-dynamic pool, or caller is not the hook |
| `0xbe8b8507` | `SwapAmountCannotBeZero()` | `IPoolManager.sol:43` | `amountSpecified == 0` |
| `0xb0ec849e` | `NonzeroNativeValue()` | `IPoolManager.sol:46` | `msg.value > 0` on an ERC-20 settle |
| `0xbda73abf` | `MustClearExactPositiveDelta()` | `IPoolManager.sol:49` | `clear` amount ≠ exact delta |
| `0xa7abe2f7` | `ProtocolFeeTooLarge(uint24)` | `IProtocolFees.sol:11` | either half > 1000 |
| `0x48f5c3ed` | `InvalidCaller()` | `IProtocolFees.sol:14` | not the fee controller |
| `0xc79e5948` | `ProtocolFeeCurrencySynced()` | `IProtocolFees.sol:17` | collecting a currently synced currency |
| `0xc4433ed5` | `TicksMisordered(int24,int24)` | `Pool.sol:33` | `tickLower >= tickUpper` |
| `0xd5e2f7ab` | `TickLowerOutOfBounds(int24)` | `Pool.sol:37` | below `MIN_TICK` |
| `0x1ad777f8` | `TickUpperOutOfBounds(int24)` | `Pool.sol:41` | above `MAX_TICK` |
| `0xb8e3c385` | `TickLiquidityOverflow(int24)` | `Pool.sol:44` | tick over `maxLiquidityPerTick` |
| `0x7983c051` | `PoolAlreadyInitialized()` | `Pool.sol:47` | re-initializing |
| `0x7c9c6e8f` | `PriceLimitAlreadyExceeded(uint160,uint160)` | `Pool.sol:55` | limit on wrong side of price |
| `0x9e4d7cc7` | `PriceLimitOutOfBounds(uint160)` | `Pool.sol:59` | limit outside min/max |
| `0xa74f97ab` | `NoLiquidityToReceiveFees()` | `Pool.sol:62` | `donate` with zero liquidity |
| `0x96206246` | `InvalidFeeForExactOut()` | `Pool.sol:65` | exact-out at 100% swap fee |
| `0xe65af6a0` | `HookAddressNotValid(address)` | `Hooks.sol:68` | bad permission bits |
| `0x1e048e1d` | `InvalidHookResponse()` | `Hooks.sol:71` | wrong selector or return length |
| `0xa9e35b2f` | `HookCallFailed()` | `Hooks.sol:74` | hook reverted (ERC-7751 context) |
| `0xfa0b71d6` | `HookDeltaExceedsSwapAmount()` | `Hooks.sol:77` | delta flipped exactIn↔exactOut |
| `0x14002113` | `LPFeeTooLarge(uint24)` | `LPFeeLibrary.sol:12` | fee > 1e6 |
| `0xaefeb924` | `CannotUpdateEmptyPosition()` | `Position.sol:16` | poke on zero-liquidity position |
| `0x93dafdf1` | `SafeCastOverflow()` | `SafeCast.sol:11` | any checked cast fails; also `LiquidityMath.addDelta` |
| `0x00bfc921` | `InvalidPrice()` | `SqrtPriceMath.sol:16` | zero sqrt price |
| `0x4f2461b8` | `InvalidPriceOrLiquidity()` | `SqrtPriceMath.sol:15` | zero price or liquidity |
| `0x4323a555` | `NotEnoughLiquidity()` | `SqrtPriceMath.sol:17` | price would go negative |
| `0xf5c787f1` | `PriceOverflow()` | `SqrtPriceMath.sol:18` | product overflow |
| `0xd4d8f3e6` | `TickMisaligned(int24,int24)` | `TickBitmap.sol:13` | tick not a multiple of spacing |
| `0x8b86327a` | `InvalidTick(int24)` | `TickMath.sol:14` | \|tick\| > `MAX_TICK` |
| `0x61487524` | `InvalidSqrtPrice(uint160)` | `TickMath.sol:16` | price outside min/max |
| `0x0d89438e` | `DelegateCallNotAllowed()` | `NoDelegateCall.sol:11` | `delegatecall` into a guarded function |
| `0xf4b3b1bc` | `NativeTransferFailed()` | `Currency.sol:32` | ETH send failed |
| `0xf27f64e4` | `ERC20TransferFailed()` | `Currency.sol:35` | ERC-20 transfer failed |
| `0x90bfb865` | `WrappedError(address,bytes4,bytes,bytes)` | `CustomRevert.sol:11` | ERC-7751 wrapper around the three above |

### 14.4 Storage layout

**Persistent** (`PoolManager`)

| Slot | Variable | Type |
|---|---|---|
| 0 | `owner` | `address` |
| 1 | `protocolFeesAccrued` | `mapping(Currency => uint256)` |
| 2 | `protocolFeeController` | `address` |
| 3 | `isOperator` | nested mapping |
| 4 | `balanceOf` | nested mapping |
| 5 | `allowance` | nested mapping |
| 6 | `_pools` | `mapping(PoolId => Pool.State)` |

**`Pool.State`** at `base = keccak256(poolId ‖ 6)`

| Offset | Field |
|---|---|
| `base + 0` | `slot0` (packed) |
| `base + 1` | `feeGrowthGlobal0X128` |
| `base + 2` | `feeGrowthGlobal1X128` |
| `base + 3` | `liquidity` |
| `base + 4` | `ticks` mapping root |
| `base + 5` | `tickBitmap` mapping root |
| `base + 6` | `positions` mapping root |

- `ticks[tick]` → `keccak256(int256(tick) ‖ (base+4))`, 3 words
- `tickBitmap[wordPos]` → `keccak256(int256(wordPos) ‖ (base+5))`, 1 word
- `positions[key]` → `keccak256(key ‖ (base+6))`, 3 words, where
  `key = keccak256(abi.encodePacked(owner, tickLower, tickUpper, salt))`
  (`Position.calculatePositionKey`, `Position.sol:48-67`, hashing 58 bytes)

**Transient**

| Slot | Derivation | Holds |
|---|---|---|
| `0xc090fc46…ab23` | `keccak256("Unlocked") − 1` | unlock flag |
| `0x7d4b3164…9a0b` | `keccak256("NonzeroDeltaCount") − 1` | outstanding delta count |
| `0x1e0745a7…bd95` | `keccak256("ReservesOf") − 1` | reserve snapshot |
| `0x27e098c5…93b9` | `keccak256("Currency") − 1` | synced currency |
| dynamic | `keccak256(target ‖ currency)` | that account's delta |

---

## 15. Use cases

Each entry gives the exact call chain. Everything except `initialize` must happen
inside an `unlock` callback.

### 15.1 Initialize a pool

```
PoolManager.initialize(key, sqrtPriceX96)                        PoolManager.sol:117
 ├── tickSpacing bounds                                                        :119-120
 ├── currency ordering                                                         :121
 ├── Hooks.isValidHookAddress(key.hooks, key.fee)                  Hooks.sol:109
 ├── LPFeeLibrary.getInitialLPFee(key.fee)                    LPFeeLibrary.sol:51
 ├── Hooks.beforeInitialize                                       Hooks.sol:178
 ├── Pool.initialize → TickMath.getTickAtSqrtPrice              Pool.sol:100
 ├── emit Initialize                                             PoolManager.sol:139
 └── Hooks.afterInitialize                                          Hooks.sol:185
```

No `unlock` needed. Anyone may initialize any pool at any price.

### 15.2 Add liquidity

```
YourRouter.addLiquidity(...)
 └── manager.unlock(encoded)                                     PoolManager.sol:104
      └── your unlockCallback
           ├── manager.modifyLiquidity(key, {tickLower, tickUpper, +L, salt}, hookData)   :145
           │    ├── Hooks.beforeAddLiquidity                        Hooks.sol:201
           │    ├── Pool.modifyLiquidity                             Pool.sol:146
           │    │    ├── checkTicks                                        :94
           │    │    ├── updateTick ×2  (+ maxLiquidityPerTick check)      :520
           │    │    ├── tickBitmap.flipTick if flipped           TickBitmap.sol:47
           │    │    ├── getFeeGrowthInside                            Pool.sol:488
           │    │    ├── Position.update                          Position.sol:76
           │    │    └── SqrtPriceMath.getAmount{0,1}Delta (signed, rounds UP)
           │    ├── Hooks.afterAddLiquidity                         Hooks.sol:221
           │    └── _accountPoolBalanceDelta  (negative → you owe)         :183
           ├── manager.sync(currency0)                                       :279
           ├── currency0.transfer(address(manager), amount0)   ← plain ERC-20
           ├── manager.settle()                                             :300
           └── (repeat for currency1)
```

For native currency: skip `sync`, call `manager.settle{value: amount}()`.

### 15.3 Swap, exact input

```
manager.unlock(...)
 └── unlockCallback
      ├── manager.swap(key, {zeroForOne: true, amountSpecified: -1000e6, sqrtPriceLimitX96}, hookData)
      │    ├── Hooks.beforeSwap                                    Hooks.sol:248
      │    ├── Pool.swap  (the while loop)                          Pool.sol:279
      │    │    ├── TickBitmap.nextInitializedTickWithinOneWord           :347
      │    │    ├── SwapMath.computeSwapStep                     SwapMath.sol:51
      │    │    ├── protocol fee carve-out                             Pool.sol:385
      │    │    ├── feeGrowthGlobal update                                  :401
      │    │    └── Pool.crossTick when a boundary is reached              :602
      │    ├── _updateProtocolFees                             PoolManager.sol:238
      │    ├── emit Swap                                                    :241
      │    ├── Hooks.afterSwap                                    Hooks.sol:285
      │    ├── _accountPoolBalanceDelta(hookDelta, hooks)      PoolManager.sol:224
      │    └── _accountPoolBalanceDelta(swapDelta, msg.sender)             :226
      ├── manager.sync(currency0); transfer in; manager.settle()
      └── manager.take(currency1, recipient, uint256(int256(delta.amount1())))
```

**Exact output**: pass a positive `amountSpecified`. Everything else is identical;
`SwapMath` takes its other branch.

### 15.4 Multi-hop swap with no intermediate transfers

Call `swap` twice inside one `unlockCallback`. The intermediate currency's delta
goes positive from hop 1 and negative from hop 2, netting to zero — so
`NonzeroDeltaCount` returns to its prior value and **no transfer of the
intermediate token ever happens**. Only the first input and last output move.
This is V4's headline gas win over V3's per-pool callbacks.

### 15.5 Settle with claim tokens instead of transfers

- Pay a debt: `manager.burn(from, currency.toId(), amount)` (`:332`)
- Receive a credit: `manager.mint(to, currency.toId(), amount)` (`:322`)

`PoolClaimsTest.sol` is the worked example.

### 15.6 Collect fees without changing a position

`modifyLiquidity` with `liquidityDelta = 0`. Returns `feesAccrued`; `delta` is
zero for principal. Reverts `CannotUpdateEmptyPosition()` if the position has no
liquidity. **Note this fires the `beforeRemoveLiquidity`/`afterRemoveLiquidity`
hooks**, not the add ones (`Hooks.sol:203`).

### 15.7 Donate to in-range LPs

`manager.donate(key, amount0, amount1, hookData)`, then settle the negative delta.
Requires nonzero in-range liquidity.

### 15.8 Set a dynamic fee

1. Create the pool with `fee = LPFeeLibrary.DYNAMIC_FEE_FLAG` (`0x800000`).
2. The pool starts at fee 0.
3. From your hook, either:
   - call `manager.updateDynamicLPFee(key, newFee)` at any time (`:339`), or
   - return `newFee | LPFeeLibrary.OVERRIDE_FEE_FLAG` from `beforeSwap` to set it
     for that swap only (`Hooks.sol:263`, `Pool.sol:303`).

### 15.9 Read pool state from another contract

```solidity
using StateLibrary for IPoolManager;

(uint160 sqrtPriceX96, int24 tick, uint24 protocolFee, uint24 lpFee) = manager.getSlot0(poolId);
uint128 liquidity = manager.getLiquidity(poolId);
(uint128 liq, uint256 fg0, uint256 fg1) = manager.getPositionInfo(poolId, owner, tickLower, tickUpper, salt);
```

For live (not cached) uncollected fees, use `StateLibrary.getFeeGrowthInside`
(`:298`) and compare against the position's stored `feeGrowthInside*LastX128`.

### 15.10 Read transient state mid-transaction

```solidity
using TransientStateLibrary for IPoolManager;

int256 myDelta = manager.currencyDelta(address(this), currency);
uint256 outstanding = manager.getNonzeroDeltaCount();
```

Useful inside a hook that needs to know what it currently owes.

### 15.11 Mine a hook address

Compute the low-14-bit mask you need from §6.1, then brute-force a CREATE2 salt
until `address & 0x3FFF == mask`. Verify with
`Hooks.validateHookPermissions(IHooks(address(this)), permissions)` in your
constructor so a bad deployment fails immediately rather than at pool creation.

---

## 16. Writing your first hook

### 16.1 The contract

```solidity
contract MyHook is BaseTestHooks {          // reverts on every callback you don't override
    IPoolManager immutable manager;

    constructor(IPoolManager _manager) {
        manager = _manager;
        Hooks.validateHookPermissions(IHooks(address(this)), Hooks.Permissions({
            beforeInitialize: false, afterInitialize: false,
            beforeAddLiquidity: false, afterAddLiquidity: false,
            beforeRemoveLiquidity: false, afterRemoveLiquidity: false,
            beforeSwap: false, afterSwap: true,
            beforeDonate: false, afterDonate: false,
            beforeSwapReturnDelta: false, afterSwapReturnDelta: true,
            afterAddLiquidityReturnDelta: false, afterRemoveLiquidityReturnDelta: false
        }));
    }

    modifier onlyPoolManager() { require(msg.sender == address(manager)); _; }
```

`BaseTestHooks` (`test/BaseTestHooks.sol`) reverts `HookNotImplemented` on every
callback, so you physically cannot forget one.

### 16.2 The address

The permissions above need bits 6 (`AFTER_SWAP_FLAG`) and 2
(`AFTER_SWAP_RETURNS_DELTA_FLAG`): mask `0x0044`. Mine a CREATE2 salt until your
address ends in those bits. `isValidHookAddress` (`Hooks.sol:109`) will reject the
pool at `initialize` if the RETURNS_DELTA bit is set without its action bit.

### 16.3 The callback

Modelled on `test/FeeTakingHook.sol:34-`:

```solidity
    function afterSwap(address, PoolKey calldata key, SwapParams calldata params,
                       BalanceDelta delta, bytes calldata)
        external override onlyPoolManager returns (bytes4, int128)
    {
        bool specifiedTokenIs0 = (params.amountSpecified < 0 == params.zeroForOne);
        (Currency feeCurrency, int128 swapAmount) =
            specifiedTokenIs0 ? (key.currency1, delta.amount1()) : (key.currency0, delta.amount0());
        if (swapAmount < 0) swapAmount = -swapAmount;

        uint256 feeAmount = uint128(swapAmount) * 123 / 10000;   // 1.23%
        manager.mint(address(this), feeCurrency.toId(), feeAmount);   // settle our own delta

        return (IHooks.afterSwap.selector, int128(int256(feeAmount)));
    }
}
```

Three non-negotiable rules:

1. **Return your own selector.** `Hooks.callHook` (`:152`) checks it; anything
   else is `InvalidHookResponse()`.
2. **Settle your own delta.** The returned `int128` becomes a delta charged to
   *your* address (`PoolManager.sol:224`). You must clear it with `take`, `mint`,
   `settle` or `clear` before `unlock` returns, or the whole transaction reverts
   `CurrencyNotSettled()`. Here `mint` does it.
3. **Return-length discipline.** `afterSwap` must return exactly
   `(bytes4, int128)` → 64 bytes. `beforeSwap` must return exactly
   `(bytes4, BeforeSwapDelta, uint24)` → 96 bytes (`Hooks.sol:259`).

### 16.4 What a hook can and cannot do

**Can:** revert any action (censor); shrink a swap via `beforeSwap`'s specified
delta; take value from either side; override the LP fee per swap on a dynamic
pool; replace the curve entirely (`CustomCurveHook`); call back into the manager
(its own callbacks are skipped by `noSelfCall`).

**Cannot:** change the pool's price directly; touch another pool's state without
going through the manager; escape the delta invariant; be swapped out — the hook
address is part of the `PoolKey`, so it is immutable for that pool's lifetime.

---

## 17. V3 to V4 migration map

| V3 concept | V4 equivalent |
|---|---|
| `UniswapV3Factory.createPool` deploys a contract | `PoolManager.initialize` writes a struct. No deployment. |
| Pool address identifies a pool | `PoolId = keccak256(PoolKey)`; the key must be passed on every call |
| Each pool holds its own tokens | `PoolManager` holds everything |
| `uniswapV3SwapCallback` per swap | one `unlockCallback` per transaction, many operations inside |
| Transfers on every action | transient deltas, netted, settled once |
| `slot0()` getter | `StateLibrary.getSlot0` via `extsload` |
| `positions(bytes32)` getter | `StateLibrary.getPositionInfo` |
| `observe()` / oracle in core | **removed** — write an oracle hook |
| `NonfungiblePositionManager` NFT | not in core; `salt` lets routers hold many positions |
| Fee tiers fixed by the factory | any fee at creation; `0x800000` for dynamic |
| `flash()` | not needed — `take` before `settle` is a flash loan |
| WETH required | native ETH is `Currency(address(0))` |
| Protocol fee: fraction of LP fee (`1/4`..`1/10`) | independent fee, up to 0.1% per direction, taken first |
| `tickSpacing` per fee tier | free per pool, 1..32767 |
| `int256 amountSpecified > 0` = exact input | **`< 0` = exact input** |
| No extensibility | 14 hook permission bits |

Porting checklist:

1. Flip the sign convention on `amountSpecified`.
2. Wrap everything in `unlock` + `unlockCallback`.
3. Replace transfer-based settlement with `sync`/`settle`/`take`.
4. Replace pool getters with `StateLibrary`.
5. Stop passing pool addresses; pass `PoolKey`.
6. Decide whether you need an NFT at all.

---

## 18. Gotchas and security notes

**The delta invariant is the only real guard.** There is no reentrancy mutex on
individual operations — `Lock` prevents *nested `unlock` calls*, nothing else.
Hooks, token transfers and native sends all hand control to untrusted code while
the manager is unlocked. Safety comes entirely from
`NonzeroDeltaCount.read() != 0 → revert` (`PoolManager.sol:112`).

**`settle` without `sync` silently becomes a native settle.** `_settle` reads the
synced currency, finds zero, and takes the `msg.value` branch (`:353-354`). Your
ERC-20 debt stays open and you fail later with `CurrencyNotSettled()` — a
confusing error a long way from the actual bug.

**One `sync` serves one `settle`.** `_settle` calls `resetCurrency()` (`:361`).
Two settles after one sync: the second takes the native branch.

**`sync` is callable while locked and by anyone.** It only writes transient state,
so cross-transaction abuse is impossible, but within a transaction an untrusted
hook can call `sync` and invalidate your snapshot. Sync immediately before you
transfer.

**Hook addresses are trust.** The hook is in the `PoolKey`, so "USDC/ETH 0.05%"
with hook A and with hook B are different pools with different risk. A hook can
revert every swap, take 100% via return deltas, set a 100% dynamic fee, or be an
upgradeable proxy. **Always resolve the full key, never just the token pair.**

**`isValidHookAddress` permits `address(0)`** for static-fee pools (`Hooks.sol:124`).
Absence of a hook is the only genuinely trustless configuration.

**Rounding always favours the pool.** `SqrtPriceMath`'s signed helpers round
liquidity additions up and removals down (`:267-269`, `:284-286`);
`SwapMath.computeSwapStep` rounds `amountIn` up and `amountOut` down. If you
re-implement any of this off-chain, copy the rounding exactly or you will produce
quotes that revert.

**Protocol-fee rounding favours LPs** (`Pool.sol:390-393`), the opposite
direction from user-facing rounding. Deliberate.

**`feeGrowthGlobal` is manipulable.** A single-position pool can `donate` to
itself and inflate it arbitrarily (`Pool.sol:80-82`, repeated at
`StateLibrary.sol:153-155`). Never compare it across pools or use it as a volume
proxy.

**The tick can lag the price by one.** After a `zeroForOne` swap that lands on a
boundary, `slot0.tick == tickNext - 1` while the price sits at `tickNext`
(`Pool.sol:431`, comment at `:409-412`). `donate` credits by tick, so a donor may
pay the wrong LPs. Check both.

**`CurrencyLibrary.fromId` truncates.** Upper 12 bytes are dropped
(`Currency.sol:114-115`), so distinct `id`s collapse onto one currency in `mint`
and `burn`.

**`clear` is irreversible.** It destroys a positive delta with no compensation
(`PoolManager.sol:310-319`), and demands the exact amount. Only for dust.

**ERC-7751 bubbling can be a revert bomb.** `bubbleUpAndRevertWith` copies all of
a hook's revert data (`CustomRevert.sol:109`); the natspec warns about it at
`:82`. A malicious hook can force enormous gas consumption on failure.

**Native transfers forward all gas.** `Currency.transfer`'s native branch
(`Currency.sol:48`) uses `call` with no stipend, so recipients can run arbitrary
code inside `take`.

**`collectProtocolFees` refuses a synced currency** (`ProtocolFees.sol:49-52`) to
avoid corrupting a pending balance-difference settle. If it reverts unexpectedly,
something earlier in your transaction synced that currency.

**Do not use `getSlot0` as an oracle.** It is spot price, movable within one
transaction with a flash loan. V4 core ships **no** oracle — that was a
deliberate removal. Build one as a hook, and TWAP it.

**`Extsload`'s range overload reads at least one slot** even with `nSlots = 0`,
because the loops are do-while shaped (`Extsload.sol:31-36`).

**Poking fires the remove-liquidity hooks.** `liquidityDelta == 0` takes the
`<= 0` branch (`Hooks.sol:203`).

**A hook is not called on its own actions.** `noSelfCall` (`Hooks.sol:171-175`)
skips the callback when `msg.sender == address(hook)`. If your hook's security
depends on its own callback firing, that assumption breaks whenever the hook
itself initiates the action.
