# Uniswap V3 Core — Complete Reference

Every contract, every function, every parameter, every revert string, every
storage slot in `uni/v3-core/contracts`. Written to be read cover to cover.

The conceptual walkthrough lives in [`UNISWAP-DEEP-DIVE.md`](UNISWAP-DEEP-DIVE.md).
This file is the exhaustive companion: it does not skip a file, and it derives the
math rather than pointing at it.

**Verification.** Every `file:line` was checked with `grep -n` against this tree.
Every storage layout and bit offset was produced by compiling the tree with
solc 0.7.6 (optimizer on, 800 runs, `bytecodeHash: none` — the settings in
`hardhat.config.ts:56-66`) and reading `forge inspect ... storageLayout`. Every
selector was computed with `cast sig`. The compiled `UniswapV3Pool` creation code
hashes to

```
0xe34f199b19b2b4f47f68442619d555527d244f78a3297ea89325f843f87b8b54
```

which is the canonical mainnet `POOL_INIT_CODE_HASH`. This tree is the deployed
protocol, byte for byte.

---

## Table of contents

- [0. File inventory (all 62 files)](#0-file-inventory)
- [1. Architecture: the deployment triangle](#1-architecture)
- [2. `UniswapV3Factory`](#2-uniswapv3factory)
- [3. `UniswapV3PoolDeployer`](#3-uniswapv3pooldeployer)
- [4. `NoDelegateCall`](#4-nodelegatecall)
- [5. `UniswapV3Pool`](#5-uniswapv3pool)
  - [5.1 Immutables, storage, `slot0` bit layout](#51-immutables-storage-slot0)
  - [5.2 Modifiers](#52-modifiers)
  - [5.3 Constructor and private helpers](#53-constructor-and-private-helpers)
  - [5.4 `initialize`](#54-initialize)
  - [5.5 `_modifyPosition` / `_updatePosition`](#55-modifyposition)
  - [5.6 `mint`](#56-mint)
  - [5.7 `burn`](#57-burn)
  - [5.8 `collect`](#58-collect)
  - [5.9 `swap`](#59-swap)
  - [5.10 `flash`](#510-flash)
  - [5.11 Oracle views](#511-oracle-views)
  - [5.12 Owner actions](#512-owner-actions)
- [6. Libraries](#6-libraries)
  - [6.1 `TickMath`](#61-tickmath)
  - [6.2 `FullMath`](#62-fullmath)
  - [6.3 `SqrtPriceMath`](#63-sqrtpricemath)
  - [6.4 `SwapMath`](#64-swapmath)
  - [6.5 `TickBitmap`](#65-tickbitmap)
  - [6.6 `BitMath`](#66-bitmath)
  - [6.7 `Tick`](#67-tick)
  - [6.8 `Position`](#68-position)
  - [6.9 `Oracle`](#69-oracle)
  - [6.10 `LiquidityMath`](#610-liquiditymath)
  - [6.11 `SafeCast`](#611-safecast)
  - [6.12 `LowGasSafeMath`](#612-lowgassafemath)
  - [6.13 `UnsafeMath`](#613-unsafemath)
  - [6.14 `TransferHelper`](#614-transferhelper)
  - [6.15 `FixedPoint96` / `FixedPoint128`](#615-fixedpoint)
- [7. Interfaces](#7-interfaces)
- [8. Test contracts](#8-test-contracts)
- [9. ABI / selector tables](#9-abi--selector-tables)
- [10. Storage layout tables](#10-storage-layout-tables)
- [11. Events reference](#11-events-reference)
- [12. Revert-string decoder](#12-revert-string-decoder)
- [13. Use cases and call chains](#13-use-cases-and-call-chains)

---

<a name="0-file-inventory"></a>
## 0. File inventory

62 `.sol` files. Nothing below is skipped.

**Root contracts (4)**

| File | Lines | What it is |
|---|---|---|
| `UniswapV3Pool.sol` | 869 | The AMM. All liquidity, swapping, fees, oracle. |
| `UniswapV3Factory.sol` | 73 | Deploys pools, owns the fee-tier registry and protocol-fee switch. |
| `UniswapV3PoolDeployer.sol` | 38 | The CREATE2 helper that feeds constructor args through storage. |
| `NoDelegateCall.sol` | 27 | Modifier that blocks `delegatecall` into a pool. |

**Libraries (16)**

| File | Lines | Role |
|---|---|---|
| `libraries/TickMath.sol` | 205 | tick ↔ `sqrtPriceX96` conversion. |
| `libraries/FullMath.sol` | 124 | 512-bit `mulDiv`, the foundation of every other calculation. |
| `libraries/SqrtPriceMath.sol` | 227 | Token amounts ↔ price movement for a given liquidity. |
| `libraries/SwapMath.sol` | 98 | One step of a swap within a single tick range. |
| `libraries/Oracle.sol` | 325 | The observation ring buffer (TWAP). |
| `libraries/Tick.sol` | 185 | Per-tick state, fee-growth-outside bookkeeping, crossing. |
| `libraries/TickBitmap.sol` | 78 | Packed "is this tick initialized" bitmap and next-tick search. |
| `libraries/Position.sol` | 88 | Per-position liquidity and fees owed. |
| `libraries/BitMath.sol` | 94 | `mostSignificantBit` / `leastSignificantBit`. |
| `libraries/LowGasSafeMath.sol` | 46 | Cheap checked add/sub/mul. |
| `libraries/SafeCast.sol` | 28 | Checked downcasts. |
| `libraries/LiquidityMath.sol` | 16 | `uint128 + int128` with over/underflow checks. |
| `libraries/UnsafeMath.sol` | 17 | `divRoundingUp` with no zero check. |
| `libraries/TransferHelper.sol` | 23 | ERC-20 `transfer` that tolerates non-standard returns. |
| `libraries/FixedPoint96.sol` | 10 | `RESOLUTION = 96`, `Q96`. |
| `libraries/FixedPoint128.sol` | 8 | `Q128`. |

**Interfaces (13)**

| File | Lines | Role |
|---|---|---|
| `interfaces/IUniswapV3Pool.sol` | 24 | Union of the six pool interfaces below. |
| `interfaces/pool/IUniswapV3PoolImmutables.sol` | 35 | `factory`, `token0`, `token1`, `fee`, `tickSpacing`, `maxLiquidityPerTick`. |
| `interfaces/pool/IUniswapV3PoolState.sol` | 116 | `slot0`, fee growth, `liquidity`, `ticks`, `tickBitmap`, `positions`, `observations`. |
| `interfaces/pool/IUniswapV3PoolDerivedState.sol` | 40 | `observe`, `snapshotCumulativesInside`. |
| `interfaces/pool/IUniswapV3PoolActions.sol` | 103 | `initialize`, `mint`, `collect`, `burn`, `swap`, `flash`, `increaseObservationCardinalityNext`. |
| `interfaces/pool/IUniswapV3PoolOwnerActions.sol` | 23 | `setFeeProtocol`, `collectProtocol`. |
| `interfaces/pool/IUniswapV3PoolEvents.sol` | 121 | All nine pool events. |
| `interfaces/IUniswapV3Factory.sol` | 78 | Factory functions and events. |
| `interfaces/IUniswapV3PoolDeployer.sol` | 26 | The single `parameters()` getter. |
| `interfaces/IERC20Minimal.sol` | 52 | The ERC-20 subset the pool actually uses. |
| `interfaces/callback/IUniswapV3MintCallback.sol` | 18 | `uniswapV3MintCallback`. |
| `interfaces/callback/IUniswapV3SwapCallback.sol` | 21 | `uniswapV3SwapCallback`. |
| `interfaces/callback/IUniswapV3FlashCallback.sol` | 18 | `uniswapV3FlashCallback`. |

**Test contracts (29)** — covered in [§8](#8-test-contracts). They are not deployed,
but they are the executable specification: the Echidna files state the invariants
the math must satisfy.

`BitMathEchidnaTest`, `BitMathTest`, `FullMathEchidnaTest`, `FullMathTest`,
`LiquidityMathTest`, `LowGasSafeMathEchidnaTest`, `MockTimeUniswapV3Pool`,
`MockTimeUniswapV3PoolDeployer`, `NoDelegateCallTest`, `OracleEchidnaTest`,
`OracleTest`, `SqrtPriceMathEchidnaTest`, `SqrtPriceMathTest`,
`SwapMathEchidnaTest`, `SwapMathTest`, `TestERC20`, `TestUniswapV3Callee`,
`TestUniswapV3ReentrantCallee`, `TestUniswapV3Router`, `TestUniswapV3SwapPay`,
`TickBitmapEchidnaTest`, `TickBitmapTest`, `TickEchidnaTest`,
`TickMathEchidnaTest`, `TickMathTest`, `TickOverflowSafetyEchidnaTest`,
`TickTest`, `UniswapV3PoolSwapTest`, `UnsafeMathEchidnaTest`.

---

<a name="1-architecture"></a>
## 1. Architecture: the deployment triangle

Three contracts and one inheritance chain explain how a pool comes to exist at a
predictable address with immutable parameters and no constructor arguments.

```
                        ┌──────────────────────────┐
                        │   UniswapV3Factory       │
                        │   is UniswapV3PoolDeployer│
                        │   is NoDelegateCall       │
                        ├──────────────────────────┤
   createPool(A,B,fee)  │ owner                    │
   ───────────────────► │ feeAmountTickSpacing     │
                        │ getPool[t0][t1][fee]     │
                        └───────────┬──────────────┘
                                    │ deploy(...)  (internal, inherited)
                                    ▼
                        ┌──────────────────────────┐
                        │ 1. parameters = {...}    │  SSTORE 3 slots
                        │ 2. new UniswapV3Pool{    │
                        │      salt: keccak(t0,t1,f)│  CREATE2
                        │    }()                   │
                        │ 3. delete parameters     │  refund
                        └───────────┬──────────────┘
                                    │ constructor runs during CREATE2
                                    ▼
                        ┌──────────────────────────┐
                        │      UniswapV3Pool       │
                        │  constructor() {         │
                        │    ... = IUniswapV3Pool  │
                        │      Deployer(msg.sender)│  ← calls BACK into factory
                        │      .parameters();      │
                        │  }                       │
                        └──────────────────────────┘
```

**Why the callback instead of constructor arguments?** A CREATE2 address is
`keccak256(0xff ++ deployer ++ salt ++ keccak256(initCode))`. If the pool took
constructor arguments they would be appended to `initCode`, so the init-code hash
would differ per pool and the address could not be computed off-chain from
`(factory, token0, token1, fee)` alone. By passing parameters through a storage
slot that the pool reads back during its own construction, `initCode` is a
constant, its hash is the fixed `0xe34f…8b54`, and any integrator can derive a
pool address with pure arithmetic and no external call. `delete parameters`
(`UniswapV3PoolDeployer.sol:36`) then refunds most of the gas.

**The three-layer split of the pool interface.** `IUniswapV3Pool.sol:15-22`
inherits six interfaces sorted by mutability — immutables, state, derived state,
actions, owner actions, events. This is not decoration: integrators typically
import only `IUniswapV3PoolState` or only the events, which keeps their own
bytecode small.

**Contract sizes.** Compiled at 800 runs, `UniswapV3Factory` is **24,535 bytes**
against the EIP-170 limit of 24,576 — a margin of **41 bytes**. `UniswapV3Pool`
is 22,142 bytes. That is why `Tick`, `Position`, `Oracle`, `TickBitmap`,
`SwapMath` and the rest are `internal` libraries inlined into the pool rather
than separately deployed and linked: a linked library would add a `DELEGATECALL`
per call and, more importantly, the factory embeds the pool's entire creation
code and cannot afford anything larger.

---

<a name="2-uniswapv3factory"></a>
## 2. `UniswapV3Factory`

`UniswapV3Factory.sol:13` — `contract UniswapV3Factory is IUniswapV3Factory, UniswapV3PoolDeployer, NoDelegateCall`

The registry and the only privileged contract in core. It owns nothing and
custodies nothing: its powers are (a) creating pools, (b) enabling new fee tiers,
(c) transferring its own ownership. The protocol-fee switch lives on each pool
and is gated by `onlyFactoryOwner`, so the factory owner reaches it indirectly.

### State variables

| Name | Type | Slot | Line | Notes |
|---|---|---|---|---|
| `parameters` | `Parameters` | 0–2 | inherited from `UniswapV3PoolDeployer.sol:18` | transient; non-zero only during `createPool` |
| `owner` | `address` | 3 | `:15` | can enable fee tiers and set pool protocol fees |
| `feeAmountTickSpacing` | `mapping(uint24 => int24)` | 4 | `:18` | fee (hundredths of a bip) → tick spacing; `0` means "not enabled" |
| `getPool` | `mapping(address => mapping(address => mapping(uint24 => address)))` | 5 | `:20` | populated in **both** token orders |

### `constructor()`

`UniswapV3Factory.sol:22-32` · non-payable

1. **State writes.** `owner = msg.sender` (`:23`); seeds three fee tiers:
   `500 → 10`, `3000 → 60`, `10000 → 200` (`:26-31`).
2. **Events.** `OwnerChanged(address(0), msg.sender)` then one `FeeAmountEnabled`
   per tier.
3. **Gotcha.** The 1 bip tier (`100 → 1`) is *not* here. It was added later on
   mainnet by calling `enableFeeAmount(100, 1)`, which is exactly the mechanism
   this design exists to support.

The tick spacings encode a deliberate trade-off. Spacing `s` means the minimum
distance between usable prices is `1.0001^s − 1`: 0.10% for `s=10`, 0.60% for
`s=60`, 2.02% for `s=200`. Wider spacing means fewer initialized ticks to cross
per swap (cheaper gas) but coarser LP range control — appropriate for volatile
pairs where a 2% granularity is irrelevant.

### `createPool(address tokenA, address tokenB, uint24 fee) → address pool`

`UniswapV3Factory.sol:35-51` · `external override noDelegateCall`

Deploys a pool. Permissionless — anyone may create any pool on any enabled tier.

| Parameter | Meaning |
|---|---|
| `tokenA`, `tokenB` | the two tokens in **either** order; sorted internally |
| `fee` | swap fee in hundredths of a bip (`3000` = 0.30%) |

**Checks, in order**

| Line | Check | Why |
|---|---|---|
| `:40` | `require(tokenA != tokenB)` | a pool of a token against itself has no meaning |
| `:42` | `require(token0 != address(0))` | since `token0` is the *smaller* address, this rejects the zero address in either position |
| `:44` | `require(tickSpacing != 0)` | the fee tier must have been enabled |
| `:45` | `require(getPool[token0][token1][fee] == address(0))` | one pool per `(pair, fee)` |

Note that all four are bare `require`s with no message — deliberate bytecode
savings in a contract sitting 41 bytes under the size limit.

**Sorting** (`:41`): `(token0, token1) = tokenA < tokenB ? (tokenA, tokenB) : (tokenB, tokenA)`.
Address comparison as `uint160`. This canonical ordering is what makes the CREATE2
salt deterministic, and it is why `token0`/`token1` — not "base"/"quote" — are the
protocol's vocabulary. Price is always `token1` per `token0`.

**State writes.** `getPool[token0][token1][fee] = pool` and
`getPool[token1][token0][fee] = pool` (`:47-49`). The comment at `:48` records the
reasoning: writing the reverse mapping costs one extra `SSTORE` once at creation,
but saves every future caller from sorting the pair. A read-optimising trade.

**External calls.** `deploy(...)` (`:46`) → `CREATE2` → the new pool's constructor
calls back into `parameters()`.

**Event.** `PoolCreated(token0, token1, fee, tickSpacing, pool)` (`:50`).

**`noDelegateCall`** prevents a contract from `delegatecall`ing `createPool` and
thereby deploying a pool whose `deployer` (and hence CREATE2 address) is the
caller rather than the canonical factory.

### `setOwner(address _owner)`

`UniswapV3Factory.sol:54-58` · `external override`

- **Access control.** `require(msg.sender == owner)` (`:55`).
- **State write.** `owner = _owner` (`:57`).
- **Event.** `OwnerChanged(owner, _owner)` — emitted at `:56`, *before* the write,
  so the old value is still readable for the first argument.
- **Gotcha.** Single-step transfer with no zero-address check. Passing
  `address(0)` permanently renounces the ability to enable fee tiers and to set
  protocol fees. Existing pools continue to work forever regardless.

### `enableFeeAmount(uint24 fee, int24 tickSpacing)`

`UniswapV3Factory.sol:61-72` · `public override`

| Line | Check | Why |
|---|---|---|
| `:62` | `msg.sender == owner` | governance only |
| `:63` | `fee < 1000000` | a fee is a fraction of `1e6`; 100% would make `1e6 - feePips == 0` and divide by zero in `SwapMath.sol:95` |
| `:67` | `tickSpacing > 0 && tickSpacing < 16384` | see below |
| `:68` | `feeAmountTickSpacing[fee] == 0` | tiers are **append-only**; an existing tier can never be changed |

**The 16384 bound** (comment at `:64-66`) is the subtle one. In
`TickBitmap.nextInitializedTickWithinOneWord` (`TickBitmap.sol:60-75`) the result
is `compressed_something * tickSpacing`, computed in `int24`. A word spans 256
compressed ticks, so the multiplication can reach roughly `256 × tickSpacing`
beyond the current tick. With `tickSpacing ≥ 16384` that product can exceed
`int24`'s range (±8,388,607) and wrap, returning a garbage tick. 16384 ticks is
already a >5× price move, so the cap costs nothing real.

Making tiers append-only is what lets integrators cache `tickSpacing` per fee
forever, and prevents governance from silently re-spacing a live pool (which
would strand every existing position on now-unusable ticks).

---

<a name="3-uniswapv3pooldeployer"></a>
## 3. `UniswapV3PoolDeployer`

`UniswapV3PoolDeployer.sol:8` — `contract UniswapV3PoolDeployer is IUniswapV3PoolDeployer`

### `struct Parameters` (`:9-15`) and storage

Compiler-verified layout (`forge inspect ... storageLayout`):

| Field | Type | Slot | Byte offset | Bits |
|---|---|---|---|---|
| `factory` | `address` | 0 | 0 | 0–159 |
| `token0` | `address` | 1 | 0 | 0–159 |
| `token1` | `address` | 2 | 0 | 0–159 |
| `fee` | `uint24` | 2 | 20 | 160–183 |
| `tickSpacing` | `int24` | 2 | 23 | 184–207 |

Three slots, not five: `token1`, `fee` and `tickSpacing` share slot 2 (20 + 3 + 3
= 26 of 32 bytes). `parameters` is `public`, so the generated getter is the
`parameters()` the pool calls back into.

### `deploy(address factory, address token0, address token1, uint24 fee, int24 tickSpacing) → address pool`

`UniswapV3PoolDeployer.sol:27-37` · `internal`

```solidity
parameters = Parameters({factory: factory, token0: token0, token1: token1, fee: fee, tickSpacing: tickSpacing});
pool = address(new UniswapV3Pool{salt: keccak256(abi.encode(token0, token1, fee))}());
delete parameters;
```

1. **Write** the three slots (`:34`).
2. **CREATE2** with `salt = keccak256(abi.encode(token0, token1, fee))` (`:35`).
   `abi.encode` — not `encodePacked` — so each field is padded to 32 bytes and the
   salt preimage is a fixed 96 bytes with no ambiguity.
3. **Delete** the slots (`:36`), refunding gas.

**Address derivation.** Any integrator can compute a pool address off-chain:

```
pool = address(uint160(uint256(keccak256(abi.encodePacked(
    hex'ff',
    factory,                                        // 0x1F98431c8aD98523631AE4a59f267346ea31F984
    keccak256(abi.encode(token0, token1, fee)),     // salt
    hex'e34f199b19b2b4f47f68442619d555527d244f78a3297ea89325f843f87b8b54'
)))));
```

This is the entire reason for the parameters-through-storage dance. The v3
periphery does exactly this in its `PoolAddress` library.

**Gotcha — reentrancy during construction.** Between the `SSTORE` and the
`delete`, `parameters` is populated and publicly readable. A malicious ERC-20
cannot exploit this because the pool constructor makes no token calls, but any
contract inheriting `UniswapV3PoolDeployer` must not expose a reentrant path
while `parameters` is set. The factory's `noDelegateCall` and the absence of any
external call in `deploy` other than the `CREATE2` itself close this off.

---

<a name="4-nodelegatecall"></a>
## 4. `NoDelegateCall`

`NoDelegateCall.sol:6` — `abstract contract NoDelegateCall`

### `address private immutable original` (`:8`)

Set to `address(this)` in the constructor (`:13`). Because it is `immutable`, the
value is computed in init code and **inlined into the deployed bytecode** — it is
not a storage read. Under `delegatecall`, the executing bytecode is the pool's
(so `original` is the pool's address) but `address(this)` is the caller's. The two
differ exactly when the call is a `delegatecall`.

### `function checkNotDelegateCall() private view` (`:18-20`)

`require(address(this) == original)`. Bare require, no message.

### `modifier noDelegateCall()` (`:23-26`)

Calls the private function, then `_`.

**Why a private function instead of inlining the check into the modifier**
(comment at `:16-17`): a modifier's body is copied into *every* function that
uses it. Inlining would duplicate the 20-byte immutable address at each site.
Routing through one private function means the address appears once and each use
site pays only a `JUMP`. In a contract 41 bytes from the size limit at the
factory level, this matters.

**What it protects.** The pool holds real tokens and determines payment by
comparing `balanceOf(address(this))` before and after a callback. If someone could
`delegatecall` `swap` from their own contract, the balance checks would read
*their* balances while the accounting mutated *their* storage — a trivially
exploitable mismatch. Applied to: `createPool`, `swap`, `flash`,
`increaseObservationCardinalityNext`, `snapshotCumulativesInside`, `observe`, and
(indirectly, via `_modifyPosition`) `mint` and `burn`.

**Not applied to**: `initialize`, `collect`, `setFeeProtocol`, `collectProtocol`.
`initialize` is called before the pool has funds. `collect` and the owner actions
only move already-accounted balances and are additionally `lock`ed.

---

<a name="5-uniswapv3pool"></a>
## 5. `UniswapV3Pool`

`UniswapV3Pool.sol:30` — `contract UniswapV3Pool is IUniswapV3Pool, NoDelegateCall`

869 lines that hold every token, price, position and fee. The only mutable
contract in a deployed V3 system; it is not upgradeable and has no admin beyond
the two `onlyFactoryOwner` protocol-fee functions.

<a name="51-immutables-storage-slot0"></a>
### 5.1 Immutables, storage, and the `slot0` bit layout

**Immutables** (`:42-54`) — inlined into bytecode, zero storage cost:

| Name | Type | Line | Set from |
|---|---|---|---|
| `factory` | `address` | `:42` | `parameters()` |
| `token0` | `address` | `:44` | `parameters()` |
| `token1` | `address` | `:46` | `parameters()` |
| `fee` | `uint24` | `:48` | `parameters()` |
| `tickSpacing` | `int24` | `:51` | `parameters()` |
| `maxLiquidityPerTick` | `uint128` | `:54` | `Tick.tickSpacingToMaxLiquidityPerTick(tickSpacing)` |

**Storage** — compiler-verified:

| Slot | Name | Type | Line |
|---|---|---|---|
| 0 | `slot0` | `Slot0` | `:74` |
| 1 | `feeGrowthGlobal0X128` | `uint256` | `:77` |
| 2 | `feeGrowthGlobal1X128` | `uint256` | `:79` |
| 3 | `protocolFees` | `ProtocolFees` | `:87` |
| 4 | `liquidity` | `uint128` | `:90` |
| 5 | `ticks` | `mapping(int24 => Tick.Info)` | `:93` |
| 6 | `tickBitmap` | `mapping(int16 => uint256)` | `:95` |
| 7 | `positions` | `mapping(bytes32 => Position.Info)` | `:97` |
| 8 … 65542 | `observations` | `Oracle.Observation[65535]` | `:99` |

**`struct Slot0` (`:56-72`) — the hot slot, bit by bit**

Everything read at the start of a swap lives in one 256-bit word, so a swap's
first `SLOAD` is a single cold read (2100 gas) instead of five.

```
 bit 255                                                                    bit 0
 ┌────────┬────────┬────────┬────────┬────────┬────────┬─────┬──────────────────┐
 │ unused │unlocked│feeProto│obsCard │obsCard │obsIndex│tick │  sqrtPriceX96    │
 │  8 bit │  bool  │  col   │  Next  │inality │        │     │                  │
 └────────┴────────┴────────┴────────┴────────┴────────┴─────┴──────────────────┘
  248-255  240-247  232-239  216-231  200-215  184-199 160-183      0-159
```

| Field | Type | Bits | Byte offset | Meaning |
|---|---|---|---|---|
| `sqrtPriceX96` | `uint160` | 0–159 | 0 | `√(token1/token0)` as Q64.96 |
| `tick` | `int24` | 160–183 | 20 | current tick; `≤ log_1.0001(price)` |
| `observationIndex` | `uint16` | 184–199 | 23 | most recently written observation |
| `observationCardinality` | `uint16` | 200–215 | 25 | populated observation slots |
| `observationCardinalityNext` | `uint16` | 216–231 | 27 | target after next write |
| `feeProtocol` | `uint8` | 232–239 | 29 | two 4-bit denominators packed |
| `unlocked` | `bool` | 240–247 | 30 | reentrancy flag |
| — | — | 248–255 | 31 | unused |

`feeProtocol` is itself two nibbles: `feeProtocol % 16` for token0, `feeProtocol >> 4`
for token1 (`:621`, `:821`, `:827`, packed at `:843`).

**`struct ProtocolFees` (`:82-85`)** — slot 3: `token0` in bits 0–127, `token1`
in bits 128–255. Both `uint128`, so the pair is one slot.

**Why `slot0.sqrtPriceX96 == 0` means "uninitialized".** There is no separate
`initialized` flag. `initialize` requires `sqrtPriceX96 == 0` (`:272`) and the
`lock` modifier requires `unlocked` (`:105`), which is only set true by
`initialize` (`:285`). So before `initialize`, every `lock`ed function reverts
with `'LOK'`. Two guards, one slot, zero extra storage.

<a name="52-modifiers"></a>
### 5.2 Modifiers

#### `lock()` — `:104-109`

```solidity
modifier lock() {
    require(slot0.unlocked, 'LOK');
    slot0.unlocked = false;
    _;
    slot0.unlocked = true;
}
```

Reentrancy guard **and** initialization guard in one. The doc comment (`:101-103`)
states the reason it is mandatory: the pool determines whether it was paid by
comparing `balanceOf` before and after an external callback. Any reentrant call
that moved tokens between the two reads would corrupt that inference.

Applied to: `mint` (`:463`), `burn` (`:521`), `collect` (`:496`), `flash` (`:796`),
`increaseObservationCardinalityNext` (`:258`), `setFeeProtocol` (`:837`),
`collectProtocol` (`:852`).

**`swap` does not use it** — it inlines the same logic (`:607`, `:615`, `:787`) so
that it can read the whole of `slot0` into memory once at `:605` and reuse that
memory copy throughout, rather than re-`SLOAD`ing inside a modifier.

#### `onlyFactoryOwner()` — `:112-115`

`require(msg.sender == IUniswapV3Factory(factory).owner())`. An external call on
every use, but the two functions it guards are rare governance actions.

<a name="53-constructor-and-private-helpers"></a>
### 5.3 Constructor and private helpers

#### `constructor()` — `:117-123`

Reads all five parameters back from `msg.sender` (the deployer) in one call
(`:119`), then derives `maxLiquidityPerTick` (`:122`). No arguments, which is what
fixes the init-code hash. `tickSpacing` is copied through a local because
`immutable`s cannot be read inside the constructor.

#### `checkTicks(int24 tickLower, int24 tickUpper) private pure` — `:126-130`

| Line | Check | Revert |
|---|---|---|
| `:127` | `tickLower < tickUpper` | `'TLU'` — Tick Lower/Upper |
| `:128` | `tickLower >= TickMath.MIN_TICK` | `'TLM'` — Tick Lower Minimum |
| `:129` | `tickUpper <= TickMath.MAX_TICK` | `'TUM'` — Tick Upper Maximum |

Note it does **not** check `tick % tickSpacing == 0`. That is enforced later and
indirectly, by `TickBitmap.flipTick` (`TickBitmap.sol:28`).

#### `_blockTimestamp() internal view virtual → uint32` — `:133-135`

`uint32(block.timestamp)`; the comment says "truncation is desired". `virtual`
purely so `MockTimeUniswapV3Pool` can override it (`test/MockTimeUniswapV3Pool.sol:23`).
Truncation wraps in February 2106; the oracle's `lte` comparator
(`Oracle.sol:128-140`) is explicitly written to survive one wrap.

#### `balance0()` / `balance1() private view → uint256` — `:140-145`, `:150-155`

```solidity
(bool success, bytes memory data) =
    token0.staticcall(abi.encodeWithSelector(IERC20Minimal.balanceOf.selector, address(this)));
require(success && data.length >= 32);
return abi.decode(data, (uint256));
```

A raw `staticcall` rather than `IERC20Minimal(token0).balanceOf(...)`. The doc
comment (`:138-139`) gives the reason: a high-level call would emit an
`EXTCODESIZE` check *in addition to* the `RETURNDATASIZE` check, and the
`data.length >= 32` test already subsumes it — a non-contract returns no data and
fails. Saves ~100 gas on a call made up to four times per swap.

`data.length >= 32` (not `== 32`) tolerates tokens that return extra padding.

<a name="54-initialize"></a>
### 5.4 `initialize(uint160 sqrtPriceX96)`

`UniswapV3Pool.sol:271-289` · `external override` · **no `lock`** (comment `:270`:
"not locked because it initializes unlocked")

Sets the starting price. Permissionless — whoever calls first picks the price.

| Step | Line | Detail |
|---|---|---|
| Check | `:272` | `require(slot0.sqrtPriceX96 == 0, 'AI')` — Already Initialized |
| Derive | `:274` | `tick = TickMath.getTickAtSqrtRatio(sqrtPriceX96)`; reverts `'R'` if out of range |
| Oracle | `:276` | `observations.initialize(_blockTimestamp())` → returns `(1, 1)` |
| Write | `:278-286` | the whole `Slot0` in one `SSTORE`, with `unlocked: true` |
| Event | `:288` | `Initialize(sqrtPriceX96, tick)` |

`feeProtocol` starts at `0` — the protocol fee is off by default on every pool.

**Security note.** Because anyone can initialize at any price, a pool can be
created at an absurd price. Nobody is harmed directly: the first LP to mint is
the one who would supply mispriced liquidity, so front-ends check the price
against an oracle before minting. Arbitrage corrects the price at the first LP's
expense, not the protocol's.

<a name="55-modifyposition"></a>
### 5.5 `_modifyPosition` and `_updatePosition`

Every liquidity change — `mint` and `burn` — funnels through these two.

#### `_modifyPosition(ModifyPositionParams params) private noDelegateCall → (Position.Info storage position, int256 amount0, int256 amount1)`

`UniswapV3Pool.sol:306-372`

`struct ModifyPositionParams` (`:291-299`): `owner`, `tickLower`, `tickUpper`,
`liquidityDelta` (signed).

Returns amounts **owed to the pool**, negative when the pool owes the recipient.
This sign convention is why `mint` casts the results straight to `uint256`
(`:475-476`) while `burn` negates first (`:532-533`).

1. `checkTicks` (`:315`).
2. `Slot0 memory _slot0 = slot0` (`:317`) — one `SLOAD`, reused throughout.
3. `_updatePosition(...)` (`:319-325`) — all the tick and fee bookkeeping.
4. If `liquidityDelta != 0`, one of three geometric cases:

```
     price ──────────────────────────────────────────────────────────►
                 tickLower                    tickUpper
    ─────────────────┬───────────────────────────┬─────────────────────
     case A          │        case B             │        case C
   tick < lower      │   lower ≤ tick < upper    │    tick ≥ upper
   ── all token0 ──  │  ── both tokens ──        │  ── all token1 ──
     amount0 only    │  amount0 + amount1        │     amount1 only
      (:328-335)     │      (:336-361)           │      (:362-370)
```

**Case A — `_slot0.tick < params.tickLower` (`:328-335`).** The range sits entirely
above the current price, so it is 100% token0. The comment (`:329-330`) explains
why: the range can only become active by the price rising into it, at which point
the position sells token0 for token1. `amount0 = getAmount0Delta(√P(lower), √P(upper), ΔL)`.
`liquidity` is untouched — out-of-range liquidity is not active.

**Case B — in range (`:336-361`).** The only branch that touches global state:
- writes an oracle observation **first** (`:341-348`), using `liquidityBefore`,
  because the observation must record the liquidity that was in force over the
  elapsed interval, not the new value;
- `amount0` from the current price up to `√P(upper)` (`:350-354`);
- `amount1` from `√P(lower)` up to the current price (`:355-359`);
- `liquidity = LiquidityMath.addDelta(liquidityBefore, liquidityDelta)` (`:361`).

**Case C — `tick ≥ tickUpper` (`:362-370`).** Entirely below the price, 100% token1.

#### `_updatePosition(address owner, int24 tickLower, int24 tickUpper, int128 liquidityDelta, int24 tick) private → Position.Info storage`

`UniswapV3Pool.sol:379-453`

1. `positions.get(owner, tickLower, tickUpper)` (`:386`) — hashes the triple.
2. Cache both `feeGrowthGlobal` values (`:388-389`).
3. If `liquidityDelta != 0` (`:394-437`):
   - `observations.observeSingle(..., secondsAgo = 0, ...)` (`:396-404`) to get the
     *counterfactual* accumulators as of now, needed to seed newly initialized ticks;
   - `ticks.update(tickLower, ..., upper = false, ...)` (`:406-417`);
   - `ticks.update(tickUpper, ..., upper = true, ...)` (`:418-429`);
   - flip the bitmap bit for any tick whose initialized state changed (`:431-436`).
     **This is where tick spacing is enforced** — `flipTick` requires
     `tick % tickSpacing == 0`.
4. `ticks.getFeeGrowthInside(...)` (`:439-440`).
5. `position.update(liquidityDelta, feeGrowthInside0, feeGrowthInside1)` (`:442`).
6. If removing liquidity, `ticks.clear` any tick that flipped to uninitialized
   (`:445-452`) — a gas refund.

**Ordering gotcha.** The ticks are updated *before* `getFeeGrowthInside` is read.
A freshly initialized tick has its `feeGrowthOutside` seeded inside
`Tick.update` (`Tick.sol:132-142`) by the convention "all growth before
initialization happened below the tick", and `getFeeGrowthInside` depends on that
seeding already being in place.

<a name="56-mint"></a>
### 5.6 `mint(address recipient, int24 tickLower, int24 tickUpper, uint128 amount, bytes data)`

`UniswapV3Pool.sol:457-487` · `external override lock` · returns `(uint256 amount0, uint256 amount1)`

Adds `amount` of liquidity to `recipient`'s position and pulls payment from
`msg.sender` via callback.

| Parameter | Meaning |
|---|---|
| `recipient` | who owns the position (may differ from the payer) |
| `tickLower`, `tickUpper` | the range; both must be multiples of `tickSpacing` |
| `amount` | liquidity `L` to add — **not** a token amount |
| `data` | opaque bytes forwarded to the callback |

**Flow**

```
mint
 ├─ require(amount > 0)                                          :464
 ├─ _modifyPosition({owner: recipient, …, +int128(amount)})      :465-473
 │   ├─ checkTicks                                               :315
 │   ├─ _updatePosition → Tick.update ×2, flipTick, Position.update
 │   └─ SqrtPriceMath.getAmount{0,1}Delta  (rounding UP)
 ├─ balance0Before = balance0()   (only if amount0 > 0)          :480
 ├─ balance1Before = balance1()   (only if amount1 > 0)          :481
 ├─ ►► IUniswapV3MintCallback(msg.sender).uniswapV3MintCallback  :482   ⚠ untrusted
 ├─ require(balance0Before + amount0 <= balance0(), 'M0')        :483
 ├─ require(balance1Before + amount1 <= balance1(), 'M1')        :484
 └─ emit Mint(msg.sender, recipient, …)                          :486
```

- **Checks.** `amount > 0` (`:464`, bare require); the two balance assertions.
- **State writes.** Via `_modifyPosition`: `ticks`, `tickBitmap`, `positions`,
  possibly `liquidity` and `observations`.
- **External calls.** `balanceOf` ×(0–2), then the callback, then `balanceOf`
  ×(0–2) again.
- **Returns.** `(amount0, amount1)` actually owed, rounded **up** — the pool
  always collects at least the exact requirement.
- **Event.** `Mint(sender, owner, tickLower, tickUpper, amount, amount0, amount1)`.

**Gotchas**

- The pool never calls `transferFrom`. It transfers nothing and simply *checks*
  it was paid. The caller must be a contract implementing the callback; an EOA
  cannot mint directly. This is why `NonfungiblePositionManager` exists.
- `<=` rather than `==` (`:483`) means overpayment is accepted and silently
  donated to the pool.
- Because the balance is re-read after the callback, a **fee-on-transfer token
  can never satisfy the check** — the pool receives less than `amount0`. V3
  simply does not support such tokens.
- `amount` is `uint128` liquidity. Converting a desired token amount into `L` is
  the periphery's job (`LiquidityAmounts`).

<a name="57-burn"></a>
### 5.7 `burn(int24 tickLower, int24 tickUpper, uint128 amount)`

`UniswapV3Pool.sol:517-543` · `external override lock` · returns `(uint256 amount0, uint256 amount1)`

Removes liquidity from **`msg.sender`'s own** position. There is no `owner`
parameter, so a position can only ever be burned by its owner.

| Step | Line | Detail |
|---|---|---|
| Modify | `:522-530` | `_modifyPosition` with `liquidityDelta = -int256(amount).toInt128()` |
| Negate | `:532-533` | returned amounts are negative (pool owes); flip to positive |
| Credit | `:535-540` | `position.tokensOwed0 += uint128(amount0)`, same for 1 |
| Event | `:542` | `Burn(owner, tickLower, tickUpper, amount, amount0, amount1)` |

**`burn` transfers nothing.** It converts liquidity into a *credit* recorded in
`tokensOwed`; `collect` moves the tokens. The split exists so that removing
liquidity and withdrawing can be separate transactions, and so that accumulated
fees and withdrawn principal go out through one code path.

**The "poke".** `burn(tickLower, tickUpper, 0)` changes no liquidity but still
runs `_updatePosition` → `Position.update`, which credits fees earned since the
last touch into `tokensOwed`. This is the canonical way to realise fees without
withdrawing. `Position.update` explicitly permits it only for non-empty positions
(`Position.sol:54`, `'NP'`).

**Rounding.** Removal uses `roundUp = false` (`SqrtPriceMath.sol:208`, `:224`), so
the LP receives the floor. Mint rounds up, burn rounds down — every rounding
error accrues to the pool.

<a name="58-collect"></a>
### 5.8 `collect(address recipient, int24 tickLower, int24 tickUpper, uint128 amount0Requested, uint128 amount1Requested)`

`UniswapV3Pool.sol:490-513` · `external override lock` · returns `(uint128 amount0, uint128 amount1)`

Transfers previously-accrued `tokensOwed` to `recipient`. The only pool function
that pays out without a callback.

| Step | Line | Detail |
|---|---|---|
| Load | `:498` | `positions.get(msg.sender, …)` — always the caller's own position |
| Clamp | `:500-501` | `amount = requested > owed ? owed : requested` — never reverts on over-request |
| Pay | `:503-510` | decrement `tokensOwed`, then `TransferHelper.safeTransfer` |
| Event | `:512` | `Collect(owner, recipient, tickLower, tickUpper, amount0, amount1)` |

The comment at `:497` explains why `checkTicks` is skipped: an invalid tick range
can never have accumulated non-zero `tokensOwed`, so the clamp yields zero and the
call is a harmless no-op.

Passing `type(uint128).max` for both requests is the idiomatic "collect
everything".

**Ordering.** `tokensOwed` is decremented *before* the transfer (`:504` then
`:505`) — checks-effects-interactions, and the `lock` modifier covers the rest.

<a name="59-swap"></a>
### 5.9 `swap(address recipient, bool zeroForOne, int256 amountSpecified, uint160 sqrtPriceLimitX96, bytes data)`

`UniswapV3Pool.sol:596-788` · `external override noDelegateCall` · returns `(int256 amount0, int256 amount1)`

The heart of the protocol.

| Parameter | Meaning |
|---|---|
| `recipient` | who receives the output |
| `zeroForOne` | `true` = selling token0 for token1 (price **falls**); `false` = the reverse |
| `amountSpecified` | **positive** = exact input; **negative** = exact output |
| `sqrtPriceLimitX96` | the worst price to accept; the swap stops here even if unfilled |
| `data` | forwarded to the callback |

Returns signed deltas from the **pool's** perspective: positive = the pool
received, negative = the pool paid.

#### Entry checks

| Line | Check | Revert |
|---|---|---|
| `:603` | `amountSpecified != 0` | `'AS'` |
| `:607` | `slot0Start.unlocked` | `'LOK'` |
| `:608-613` | direction-consistent price limit | `'SPL'` |

The limit check (`:609-611`) requires, for `zeroForOne`, that
`sqrtPriceLimitX96 < slot0Start.sqrtPriceX96 && sqrtPriceLimitX96 > MIN_SQRT_RATIO`
— strictly *below* the current price (you are pushing the price down) and strictly
*above* the absolute floor. Mirrored for the other direction. Then `:615` takes
the lock manually.

#### Working memory

`SwapCache` (`:545-558`, built `:617-625`) — fixed for the whole swap:
`feeProtocol` (the correct nibble for this direction, `:621`), `liquidityStart`,
`blockTimestamp`, plus lazily-computed `tickCumulative` /
`secondsPerLiquidityCumulativeX128` guarded by `computedLatestObservation`.

`SwapState` (`:561-576`, built `:629-638`) — mutated each iteration:
`amountSpecifiedRemaining`, `amountCalculated`, `sqrtPriceX96`, `tick`,
`feeGrowthGlobalX128` (input token only), `protocolFee`, `liquidity`.

`StepComputations` (`:578-593`) — scratch space for one iteration.

#### The loop — `:641-730`

```
while (amountSpecifiedRemaining != 0 && sqrtPriceX96 != sqrtPriceLimitX96)
│
├─ step.sqrtPriceStartX96 = state.sqrtPriceX96                            :644
│
├─ (tickNext, initialized) = tickBitmap.nextInitializedTickWithinOneWord( :646-650
│        state.tick, tickSpacing, zeroForOne)
│      └── searches at most 256 compressed ticks; may return an
│          UNinitialized boundary, in which case the loop simply
│          iterates again from there
│
├─ clamp tickNext into [MIN_TICK, MAX_TICK]                               :653-657
│      (the bitmap knows nothing about the global bounds)
│
├─ step.sqrtPriceNextX96 = TickMath.getSqrtRatioAtTick(tickNext)          :660
│
├─ SwapMath.computeSwapStep(                                              :663-671
│        current  = state.sqrtPriceX96,
│        target   = min/max(sqrtPriceNextX96, sqrtPriceLimitX96),  ← whichever binds
│        liquidity= state.liquidity,
│        remaining= state.amountSpecifiedRemaining,
│        feePips  = fee)
│    → (sqrtPriceX96, amountIn, amountOut, feeAmount)
│
├─ book the amounts                                                       :673-679
│    exactIn : remaining -= (amountIn + feeAmount);  calculated -= amountOut
│    exactOut: remaining += amountOut;               calculated += (amountIn + feeAmount)
│
├─ protocol cut: delta = feeAmount / feeProtocol; feeAmount -= delta       :682-686
│                protocolFee += delta            (integer division; 0 when off)
│
├─ if (liquidity > 0)                                                     :689-690
│      feeGrowthGlobalX128 += mulDiv(feeAmount, Q128, liquidity)
│      └── when liquidity == 0 the fee is silently kept by the pool
│
└─ tick transition                                                        :692-729
   ├─ if sqrtPriceX96 == sqrtPriceNextX96      → we landed exactly on tickNext
   │   ├─ if initialized:
   │   │    ├─ lazily compute the observation once   (:698-708)
   │   │    ├─ liquidityNet = ticks.cross(tickNext, …) (:709-717)
   │   │    ├─ if zeroForOne: liquidityNet = -liquidityNet (:720)
   │   │    └─ liquidity = addDelta(liquidity, liquidityNet) (:722)
   │   └─ tick = zeroForOne ? tickNext - 1 : tickNext         (:725)
   ├─ else if sqrtPriceX96 != sqrtPriceStartX96 → we stopped mid-range
   │        tick = getTickAtSqrtRatio(sqrtPriceX96)           (:728)
   └─ else → price did not move at all; tick unchanged
```

**The `tickNext - 1` asymmetry (`:725`).** A tick's range is
`[getSqrtRatioAtTick(t), getSqrtRatioAtTick(t+1))` — closed on the left. Moving
right (`zeroForOne == false`) and landing exactly on `tickNext` puts the price at
the *start* of tick `tickNext`, so the tick is `tickNext`. Moving left and landing
on the same boundary puts the price at the *end* of tick `tickNext - 1`, which is
the tick that contains it. Consequence: after a leftward cross, `slot0.tick` is
`tickNext - 1` while `slot0.sqrtPriceX96` is exactly `getSqrtRatioAtTick(tickNext)`.
Code that recomputes the tick from the price and compares to `slot0.tick` will see
an off-by-one. This is correct, not a bug, and V4 carries the same comment.

**Why cross before flipping the sign.** `Tick.cross` returns `liquidityNet`, the
amount added when crossing **left to right** (`Tick.sol:167`). Going right to
left, the same boundary removes what it would have added, hence the negation at
`:720`. The comment notes it is safe because `liquidityNet` can never be
`type(int128).min` — `Tick.update` builds it through `SafeCast.toInt128`.

**Lazy observation (`:696-708`).** The counterfactual accumulator values are only
needed if an initialized tick is actually crossed, and they are the same for every
crossing in the transaction. Computing them once, on first need, keeps a swap that
crosses nothing from paying for the oracle at all.

**Protocol fee arithmetic (`:683`).** `delta = feeAmount / feeProtocol` where
`feeProtocol ∈ {0} ∪ [4,10]` — a *denominator*, so the protocol takes 1/4 to 1/10
of the swap fee. Integer division truncates in the LPs' favour.

#### Settlement — `:733-788`

1. **Oracle and price** (`:733-752`). If the tick moved, write an observation and
   store price, tick, index and cardinality together (`:743-748`). If only the
   price moved within the tick, write just `sqrtPriceX96` (`:751`) — one `SSTORE`
   instead of a struct write.
2. **Liquidity** (`:755`), written only if it changed.
3. **Fee growth and protocol fees** (`:759-765`), on the input side only. The
   comment at `:758` accepts overflow: `protocolFees` is `uint128` and governance
   is expected to withdraw before it saturates.
4. **Output amounts** (`:767-769`):
   ```solidity
   (amount0, amount1) = zeroForOne == exactInput
       ? (amountSpecified - state.amountSpecifiedRemaining, state.amountCalculated)
       : (state.amountCalculated, amountSpecified - state.amountSpecifiedRemaining);
   ```
   `amountSpecified - remaining` is the specified token's actual amount (less than
   requested if the price limit bound); `amountCalculated` is the other side.
5. **Transfers and payment** (`:772-784`). Output goes out **first** (`:773` or
   `:779`), then the callback, then the balance check. This is what makes a V3
   swap a flash swap: the recipient has the output in hand while
   `uniswapV3SwapCallback` runs, and only has to deliver the input before it
   returns.

| Line | Check | Revert |
|---|---|---|
| `:777` | `balance0Before + uint256(amount0) <= balance0()` | `'IIA'` — Insufficient Input Amount |
| `:783` | `balance1Before + uint256(amount1) <= balance1()` | `'IIA'` |

6. **Event and unlock** (`:786-787`).

**`'IIA'` is absent from `checkTicks`-style short names but is the single most
common V3 revert in the wild** — it means a router's callback failed to pay.

**Integrator warning.** `uniswapV3SwapCallback` is called on `msg.sender` with no
authentication *by the pool*. The callback implementation must verify that
`msg.sender` is a genuine Uniswap pool (recompute the CREATE2 address), or anyone
can call it directly and drain the approved tokens. This is the most frequently
exploited V3 integration mistake.

<a name="510-flash"></a>
### 5.10 `flash(address recipient, uint256 amount0, uint256 amount1, bytes data)`

`UniswapV3Pool.sol:791-834` · `external override lock noDelegateCall`

Lends both tokens with no collateral, requiring repayment plus the pool's fee tier
before the call returns.

| Step | Line | Detail |
|---|---|---|
| Check | `:798` | `require(_liquidity > 0, 'L')` — no liquidity, nothing to lend |
| Fees | `:800-801` | `fee0 = mulDivRoundingUp(amount0, fee, 1e6)`, same for 1 |
| Snapshot | `:802-803` | both balances |
| Send | `:805-806` | transfer out (skipped when zero) |
| Callback | `:808` | `uniswapV3FlashCallback(fee0, fee1, data)` ⚠ untrusted |
| Verify | `:813-814` | `balanceBefore + fee <= balanceAfter`, else `'F0'` / `'F1'` |
| Account | `:817-831` | split actual payment between protocol and LPs |
| Event | `:833` | `Flash(sender, recipient, amount0, amount1, paid0, paid1)` |

**`paid` versus `fee`.** The checks require *at least* the fee, but the accounting
at `:817-818` uses `paid = balanceAfter - balanceBefore`, the amount actually
received. Overpayment is distributed to LPs rather than stranded — which makes
`flash(0, 0, …)` with a voluntary payment a clean way to donate fees to a pool.

**No `mulDiv` overflow guard needed on the fee** because `fee < 1e6` is enforced
at `UniswapV3Factory.sol:63`.

**Protocol split (`:820-831`)** mirrors the swap path but reads
`slot0.feeProtocol` fresh from storage rather than from a cache.

**Gotcha.** `flash` does not check that the pool holds `amount0`/`amount1`; a
request larger than the balance simply reverts inside `safeTransfer` with `'TF'`.

<a name="511-oracle-views"></a>
### 5.11 Oracle views

#### `observe(uint32[] calldata secondsAgos)` — `:236-252`

`external view override noDelegateCall` → `(int56[] tickCumulatives, uint160[] secondsPerLiquidityCumulativeX128s)`

A thin forward to `Oracle.observe` (`:244-251`) passing current `tick`, `index`,
`liquidity` and `cardinality`. `secondsAgos = [3600, 0]` is the canonical TWAP
request: the arithmetic-mean tick over the last hour is
`(tickCumulatives[1] - tickCumulatives[0]) / 3600`, which corresponds to the
**geometric** mean price because ticks are logarithmic.

Reverts `'OLD'` (from `Oracle.sol:226`) if the window predates the oldest stored
observation.

#### `snapshotCumulativesInside(int24 tickLower, int24 tickUpper)` — `:158-233`

`external view override noDelegateCall` → `(int56 tickCumulativeInside, uint160 secondsPerLiquidityInsideX128, uint32 secondsInside)`

Per-range accumulator snapshots, using the same "outside" trick as fee growth.

| Step | Line | Detail |
|---|---|---|
| Check ticks | `:169` | `checkTicks` |
| Load | `:178-198` | both ticks' `tickCumulativeOutside`, `secondsPerLiquidityOutsideX128`, `secondsOutside`; bare `require(initialized)` on each (`:188`, `:197`) |
| Below range | `:202-207` | `tick < tickLower`: plain differences lower − upper |
| Inside range | `:208-225` | `observeSingle(secondsAgo = 0)` for the live totals, then subtract **both** outsides |
| Above range | `:226-231` | differences upper − lower |

**These values are only meaningful as differences between two calls.** Each is
seeded whenever a tick is initialized and flipped on every cross, so an absolute
reading is arbitrary. Take a snapshot, wait, take another, subtract.
`secondsInside` between two snapshots is how long the price spent inside the
range — the basis for range-liquidity mining.

#### `increaseObservationCardinalityNext(uint16 observationCardinalityNext)` — `:255-267`

`external override lock noDelegateCall`

Pays now for oracle capacity later.

| Step | Line | Detail |
|---|---|---|
| Read old | `:261` | kept for the event |
| Grow | `:262-263` | `observations.grow(old, next)` — writes `blockTimestamp = 1` into each new slot |
| Store | `:264` | `slot0.observationCardinalityNext = new` |
| Event | `:265-266` | only if the value actually changed |

The pre-writing in `Oracle.grow` (`Oracle.sol:118`) is the whole point: it turns
each future observation write from a 20,000-gas cold `SSTORE` into a ~2,900-gas
warm one, moving the cost from swappers to whoever wants the deeper oracle.
Cardinality can only ever rise; `grow` is a no-op if `next <= current`.

<a name="512-owner-actions"></a>
### 5.12 Owner actions

#### `setFeeProtocol(uint8 feeProtocol0, uint8 feeProtocol1)` — `:837-845`

`external override lock onlyFactoryOwner`

- **Check** (`:838-841`): each value must be `0` (off) or in `[4, 10]`. So the
  protocol may take between 1/10 and 1/4 of swap fees, or nothing. There is no
  1/2 or 1/3.
- **Write** (`:843`): `slot0.feeProtocol = feeProtocol0 + (feeProtocol1 << 4)` —
  two nibbles in one byte.
- **Event** (`:844`): `SetFeeProtocol` with old and new for both tokens.

The protocol fee is a cut of the *LP fee*, not an extra charge on the trader.
Turning it on does not change any quoted price.

#### `collectProtocol(address recipient, uint128 amount0Requested, uint128 amount1Requested)` — `:848-868`

`external override lock onlyFactoryOwner` → `(uint128 amount0, uint128 amount1)`

- **Clamp** (`:853-854`) to the accrued balance, like `collect`.
- **The leave-one-wei trick** (`:857`, `:862`):
  ```solidity
  if (amount0 == protocolFees.token0) amount0--; // ensure that the slot is not cleared, for gas savings
  ```
  Zeroing a storage slot triggers a refund but makes the *next* write a cold
  20,000-gas `SSTORE`. Since protocol fees accrue continuously, leaving 1 wei
  keeps the slot warm and saves far more than the refund is worth.
- **Transfer** (`:859`, `:864`) and emit `CollectProtocol` (`:867`).

---

<a name="6-libraries"></a>
## 6. Libraries

All sixteen are `internal` and inlined into the pool — no `DELEGATECALL`, no
linking, no separate deployment.

<a name="61-tickmath"></a>
### 6.1 `TickMath` — `libraries/TickMath.sol` (205 lines)

The bijection between ticks and prices. `pragma solidity >=0.5.0 <0.8.0` — it
relies on 0.7-style unchecked arithmetic throughout.

#### Constants

| Name | Value | Line | Derivation |
|---|---|---|---|
| `MIN_TICK` | `-887272` | `:9` | `floor(log_1.0001(2^-128))` |
| `MAX_TICK` | `887272` | `:11` | `-MIN_TICK` |
| `MIN_SQRT_RATIO` | `4295128739` | `:14` | `getSqrtRatioAtTick(MIN_TICK)` |
| `MAX_SQRT_RATIO` | `1461446703485210103287273052203988822378723970342` | `:16` | `getSqrtRatioAtTick(MAX_TICK)` |

The tick range covers prices from `2^-128` to `2^128`, a ratio of `2^256` — the
widest range expressible while keeping `sqrtPriceX96` inside `uint160`.

Both ratio bounds are defined as *outputs of the algorithm*, not as
independently-computed ideals. Simulating `getSqrtRatioAtTick` exactly confirms
this: the exact real value of `√(1.0001^-887272)·2^96` is `4295128738.5…`, and the
function's final round-up yields `4295128739`. `MAX_SQRT_RATIO` likewise differs
from a naive high-precision computation, because it carries the accumulated
truncation of twenty fixed-point multiplications. Defining the constants as
"whatever the function returns" is what keeps the two directions consistent.

#### `getSqrtRatioAtTick(int24 tick) internal pure → uint160 sqrtPriceX96`

`:23-54`. Computes `√(1.0001^tick) · 2^96`.

| Line | Check | Revert |
|---|---|---|
| `:25` | `absTick <= uint256(MAX_TICK)` | `'T'` |

**The algorithm.** Write `absTick` in binary: `absTick = Σ bᵢ·2ⁱ`. Then

```
√(1.0001^-absTick) = ∏ over set bits i of  √(1.0001)^(-2ⁱ)
```

so twenty precomputed constants — one per bit of a 20-bit magnitude — reduce the
exponentiation to at most twenty multiplications. Each constant is
`√(1.0001)^(-2ⁱ) · 2^128`, i.e. a Q128.128 fixed-point number:

| Line | Bit tested | Constant | Represents |
|---|---|---|---|
| `:27` | `0x1` | `0xfffcb933bd6fad37aa2d162d1a594001` | `1.0001^(-1/2)` |
| `:28` | `0x2` | `0xfff97272373d413259a46990580e213a` | `1.0001^(-1)` |
| `:29` | `0x4` | `0xfff2e50f5f656932ef12357cf3c7fdcc` | `1.0001^(-2)` |
| `:30` | `0x8` | `0xffe5caca7e10e4e61c3624eaa0941cd0` | `1.0001^(-4)` |
| `:31` | `0x10` | `0xffcb9843d60f6159c9db58835c926644` | `1.0001^(-8)` |
| `:32` | `0x20` | `0xff973b41fa98c081472e6896dfb254c0` | `1.0001^(-16)` |
| `:33` | `0x40` | `0xff2ea16466c96a3843ec78b326b52861` | `1.0001^(-32)` |
| `:34` | `0x80` | `0xfe5dee046a99a2a811c461f1969c3053` | `1.0001^(-64)` |
| `:35` | `0x100` | `0xfcbe86c7900a88aedcffc83b479aa3a4` | `1.0001^(-128)` |
| `:36` | `0x200` | `0xf987a7253ac413176f2b074cf7815e54` | `1.0001^(-256)` |
| `:37` | `0x400` | `0xf3392b0822b70005940c7a398e4b70f3` | `1.0001^(-512)` |
| `:38` | `0x800` | `0xe7159475a2c29b7443b29c7fa6e889d9` | `1.0001^(-1024)` |
| `:39` | `0x1000` | `0xd097f3bdfd2022b8845ad8f792aa5825` | `1.0001^(-2048)` |
| `:40` | `0x2000` | `0xa9f746462d870fdf8a65dc1f90e061e5` | `1.0001^(-4096)` |
| `:41` | `0x4000` | `0x70d869a156d2a1b890bb3df62baf32f7` | `1.0001^(-8192)` |
| `:42` | `0x8000` | `0x31be135f97d08fd981231505542fcfa6` | `1.0001^(-16384)` |
| `:43` | `0x10000` | `0x9aa508b5b7a84e1c677de54f3e99bc9` | `1.0001^(-32768)` |
| `:44` | `0x20000` | `0x5d6af8dedb81196699c329225ee604` | `1.0001^(-65536)` |
| `:45` | `0x40000` | `0x2216e584f5fa1ea926041bedfe98` | `1.0001^(-131072)` |
| `:46` | `0x80000` | `0x48a170391f7dc42444e8fa2` | `1.0001^(-262144)` |

Verified: each constant equals `ceil(1.0001^(-b/2) · 2^128)` for its bit value `b`
(exact for the first and last, +1 ulp for the rest — the constants are rounded up).

Notice the constants shrink from ~`2^128` down to ~`2^92`: each is a smaller
fraction, and beyond `0x10000` they no longer need the full 128 bits.

Mechanics:
- `:27` seeds `ratio` with either the bit-0 constant or `1.0` in Q128.128
  (`0x100000000000000000000000000000000`), avoiding a wasted multiply.
- `:28-46` each do `ratio = (ratio * C) >> 128`, keeping Q128.128 format.
- `:48` — **for positive ticks, invert**: `ratio = type(uint256).max / ratio`.
  Since `ratio ≈ 2^128 / √(1.0001^absTick)`, dividing `2^256 − 1` by it gives
  `≈ 2^128 · √(1.0001^absTick)`. Using `uint256.max` rather than `2^256` is both a
  necessity (`2^256` is not representable) and a slight downward bias.
- `:53` — convert Q128.128 → Q64.96 and round up:
  ```solidity
  sqrtPriceX96 = uint160((ratio >> 32) + (ratio % (1 << 32) == 0 ? 0 : 1));
  ```

**Why round up here** (comment `:52`): so that
`getTickAtSqrtRatio(getSqrtRatioAtTick(t)) == t` for every `t`. Rounding down
could place the result fractionally below the true boundary, landing in tick
`t − 1` and breaking the round trip that the swap loop depends on.

**Precision.** Twenty truncating shifts, each losing at most 1 ulp at Q128.128,
bound the relative error near `2^-128` — far below the Q64.96 output resolution.
`TickMathEchidnaTest.sol:8-15` fuzzes exactly the monotonicity and round-trip
properties.

#### `getTickAtSqrtRatio(uint160 sqrtPriceX96) internal pure → int24 tick`

`:61-204`. The inverse: the greatest tick whose ratio is `<= sqrtPriceX96`.

| Line | Check | Revert |
|---|---|---|
| `:63` | `sqrtPriceX96 >= MIN_SQRT_RATIO && sqrtPriceX96 < MAX_SQRT_RATIO` | `'R'` |

The upper bound is strict (comment `:62`): the price can never *reach* the max
tick's ratio, only approach it.

**Step 1 — widen to Q128.128** (`:64`): `ratio = uint256(sqrtPriceX96) << 32`.

**Step 2 — most significant bit** (`:69-107`). Eight unrolled assembly blocks
implement a branchless binary search:

```solidity
let f := shl(7, gt(r, 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF))  // f = 128 if r >= 2^128 else 0
msb := or(msb, f)
r := shr(f, r)
```

Each block tests whether the remaining value needs more than `2^k` bits, records
`k` into `msb`, and shifts it away. Widths 128, 64, 32, 16, 8, 4, 2, 1 recover all
eight bits of `msb` in eight steps with no jumps. Written as raw assembly rather
than calling `BitMath.mostSignificantBit` because the pool needs the shifted
remainder `r`, which `BitMath` discards.

**Step 3 — normalise** (`:109-110`): shift `ratio` so `r` lands in `[2^127, 2^128)`,
i.e. a Q1.127 mantissa in `[1, 2)`.

**Step 4 — integer part of the log** (`:112`):
`log_2 = (int256(msb) - 128) << 64` — the exponent, as a Q64.64 number. `-128`
because `ratio` is Q128.128, so `msb == 128` corresponds to a value of exactly 1.

**Step 5 — fractional bits** (`:114-196`). Fourteen identical blocks:

```solidity
r := shr(127, mul(r, r))     // square the Q1.127 mantissa -> Q2.127
let f := shr(128, r)         // f = 1 iff the square reached 2.0
log_2 := or(log_2, shl(63 - i, f))
r := shr(f, r)               // halve if it did, restoring [1,2)
```

This is binary logarithm digit by digit. If `m ∈ [1,2)` and `m² ≥ 2` then the next
bit of `log₂(m)` is 1 and we continue with `m²/2`; otherwise the bit is 0 and we
continue with `m²`. Fourteen iterations give 14 fractional bits, written at
positions 63 down to 50 (`:117` down to `:195`).

**Step 6 — change of base** (`:198`):
```solidity
int256 log_sqrt10001 = log_2 * 255738958999603826347141; // 128.128 number
```
We want `log_{√1.0001}(ratio) = log₂(ratio) / log₂(√1.0001)`. The multiplier is
`2^64 / log₂(√1.0001)` — computing that exactly gives `255738959000112582270869`,
slightly above the constant used; the constant is tuned down so the error stays
one-sided and is absorbed by the bounds below. Q64.64 × Q64.64 = Q128.128.

**Step 7 — bracket and disambiguate** (`:200-203`):
```solidity
int24 tickLow = int24((log_sqrt10001 - 3402992956809132418596140100660247210) >> 128);
int24 tickHi  = int24((log_sqrt10001 + 291339464771989622907027621153398088495) >> 128);
tick = tickLow == tickHi ? tickLow : getSqrtRatioAtTick(tickHi) <= sqrtPriceX96 ? tickHi : tickLow;
```
The two magic numbers are `0.01000049749154292` and `0.8561697375276566` in
Q128.128 — the proven lower and upper error bounds of everything above. If both
shifts land on the same integer, the answer is certain and costs nothing more. If
they straddle a boundary, one call to `getSqrtRatioAtTick` decides. The asymmetry
(a tiny margin below, a large one above) reflects the one-sided bias of the
truncating log and the tuned multiplier.

**Gas.** The common path avoids the `getSqrtRatioAtTick` call entirely, which is
why the bounds are as tight as they are provable.

<a name="62-fullmath"></a>
### 6.2 `FullMath` — `libraries/FullMath.sol` (124 lines)

`floor(a·b/denominator)` at full 512-bit intermediate precision. Credited in the
source (`:13`) to Remco Bloemen. Every price and fee calculation in V3 rests on it.

#### `mulDiv(uint256 a, uint256 b, uint256 denominator) internal pure → uint256 result`

`:14-106`

**Step 1 — the 512-bit product** (`:26-30`):
```solidity
let mm := mulmod(a, b, not(0))          // a*b mod (2^256 - 1)
prod0 := mul(a, b)                       // a*b mod 2^256
prod1 := sub(sub(mm, prod0), lt(mm, prod0))
```
`not(0)` is `2^256 − 1`. Knowing `a·b` modulo both `2^256` and `2^256 − 1`
determines it uniquely below `2^512` by the Chinese Remainder Theorem; the third
line is the closed-form reconstruction of the high limb, with `lt(mm, prod0)`
supplying the borrow.

**Step 2 — the easy case** (`:33-39`): if `prod1 == 0` the product fits in 256
bits; `require(denominator > 0)` then a plain `div`.

**Step 3 — bound the result** (`:43`): `require(denominator > prod1)` guarantees
`a·b/denominator < 2^256` and simultaneously rules out `denominator == 0`.

**Step 4 — make the division exact** (`:51-59`):
```solidity
remainder := mulmod(a, b, denominator)
prod1 := sub(prod1, gt(remainder, prod0))
prod0 := sub(prod0, remainder)
```
Subtracting the remainder from the 512-bit value leaves an exact multiple of
`denominator`. Exact divisibility is what lets the next steps replace division
with multiplication.

**Step 5 — factor out powers of two** (`:64-80`):
```solidity
uint256 twos = -denominator & denominator;   // lowest set bit
denominator := div(denominator, twos)        // now odd
prod0 := div(prod0, twos)
twos := add(div(sub(0, twos), twos), 1)      // = 2^256 / twos
prod0 |= prod1 * twos;
```
`-x & x` isolates the lowest set bit — the largest power of two dividing
`denominator`. Dividing both sides by it leaves an **odd** denominator, which is
the precondition for a modular inverse to exist. The `2^256 / twos` computation
(`:78`) is written as `(0 - twos)/twos + 1` because `2^256` itself is not
representable; if `twos == 1` this correctly yields 1. The final `|=` folds the
high limb's surviving bits into `prod0`.

**Step 6 — modular inverse by Newton–Raphson** (`:87-96`):
```solidity
uint256 inv = (3 * denominator) ^ 2;   // correct mod 2^4
inv *= 2 - denominator * inv;          // mod 2^8
inv *= 2 - denominator * inv;          // mod 2^16
inv *= 2 - denominator * inv;          // mod 2^32
inv *= 2 - denominator * inv;          // mod 2^64
inv *= 2 - denominator * inv;          // mod 2^128
inv *= 2 - denominator * inv;          // mod 2^256
```
The seed `(3d) XOR 2` is a known identity giving an inverse correct to 4 bits for
odd `d`. The iteration `x ← x(2 − dx)` is Newton's method for `1/d`; by Hensel's
lemma it doubles the number of correct bits each step in the 2-adic setting, so
4 → 8 → 16 → 32 → 64 → 128 → 256 in six multiplications. `^` here is Solidity's
**XOR**, not exponentiation.

**Step 7** (`:104`): `result = prod0 * inv`. Since the division is exact,
multiplying by the inverse mod `2^256` *is* the quotient.

**Cost:** no `DIV` opcodes in the hard path — a handful of `MUL`s instead.

#### `mulDivRoundingUp(uint256 a, uint256 b, uint256 denominator) internal pure → uint256`

`:113-123`. Calls `mulDiv`, then if `mulmod(a, b, denominator) > 0` (a non-zero
remainder) requires `result < type(uint256).max` and increments. Overflow-safe
ceiling division.

`FullMathEchidnaTest.sol:7-66` fuzzes the invariants: the rounded-up result minus
the rounded-down result is 0 or 1, and `mulDiv` inverts multiplication.

<a name="63-sqrtpricemath"></a>
### 6.3 `SqrtPriceMath` — `libraries/SqrtPriceMath.sol` (227 lines)

Converts between token amounts and price movement. **The rounding direction of
every function here is a security property**, not a stylistic choice.

#### The underlying geometry

A position with liquidity `L` between `√Pa` and `√Pb` holds real reserves

```
x  =  L · (1/√P − 1/√Pb)        (token0, spent as price rises)
y  =  L · (√P  − √Pa)           (token1, spent as price falls)
```

Differencing between two prices gives the two amount formulas:

```
Δx  =  L · (√Pb − √Pa) / (√Pa · √Pb)          ← getAmount0Delta
Δy  =  L · (√Pb − √Pa)                        ← getAmount1Delta
```

`Δy` is linear in `√P`, `Δx` is not — which is why token1 gets the cheap formula
and token0 the expensive one.

#### `getAmount0Delta(uint160 sqrtRatioAX96, uint160 sqrtRatioBX96, uint128 liquidity, bool roundUp) internal pure → uint256 amount0`

`:153-173`

- **Sort** (`:159`) so `A ≤ B`; callers may pass either order.
- `numerator1 = uint256(liquidity) << 96` (`:161`) — `L` in Q96.
- `numerator2 = sqrtRatioBX96 - sqrtRatioAX96` (`:162`).
- **Check** `require(sqrtRatioAX96 > 0)` (`:164`) — division by the lower price.
- **Round up** (`:168-171`): `divRoundingUp(mulDivRoundingUp(n1, n2, √B), √A)` —
  both divisions round up.
- **Round down** (`:172`): `mulDiv(n1, n2, √B) / √A` — both truncate.

The `2^96` in `numerator1` cancels one of the two Q96 price factors, leaving a
plain token amount.

#### `getAmount1Delta(uint160 sqrtRatioAX96, uint160 sqrtRatioBX96, uint128 liquidity, bool roundUp) internal pure → uint256 amount1`

`:182-194`. Sort, then `mulDiv{RoundingUp}(liquidity, √B − √A, Q96)`. One
operation; no zero check is needed because `Q96` is a constant.

#### The signed overloads

`getAmount0Delta(…, int128 liquidity)` — `:201-210`
`getAmount1Delta(…, int128 liquidity)` — `:217-226`

```solidity
liquidity < 0
    ? -getAmount0Delta(a, b, uint128(-liquidity), false).toInt256()   // removing: round DOWN
    : getAmount0Delta(a, b, uint128(liquidity), true).toInt256()      // adding:   round UP
```

**This is the single most important rounding rule in V3.** Adding liquidity
rounds the required deposit **up**; removing liquidity rounds the payout
**down**. Every fractional wei is retained by the pool, so the invariant "the pool
is never short" holds under arbitrary rounding. `_modifyPosition` calls exactly
these overloads (`:331`, `:350`, `:355`, `:365`), which is how `mint` and `burn`
inherit the behaviour without any explicit flag.

#### `getNextSqrtPriceFromAmount0RoundingUp(uint160 sqrtPX96, uint128 liquidity, uint256 amount, bool add) internal pure → uint160`

`:28-56`. From `x = L/√P` (virtual reserves), adding `Δx` gives

```
√P'  =  L · √P / (L ± Δx · √P)
```

- **Short-circuit** `amount == 0` (`:35`) — the comment notes the general formula
  would not be guaranteed to return exactly `sqrtPX96`.
- **`add == true`** (`:38-47`): if `amount * sqrtPX96` does not overflow (checked
  by the division identity at `:40`) and the denominator does not wrap, use
  `mulDivRoundingUp(numerator1, sqrtPX96, denominator)`. Otherwise fall back to
  the algebraically equivalent but lower-precision
  `divRoundingUp(numerator1, numerator1 / sqrtPX96 + amount)` (`:47`), which
  divides first and so cannot overflow.
- **`add == false`** (`:49-54`): `require` that the product does not overflow
  **and** `numerator1 > product` (`:52`) — otherwise the denominator would
  underflow, which physically means removing more token0 than the range contains.

Always rounds **up** (comment `:18-20`): on exact output (price rising) it must
move far enough to actually produce the requested amount; on exact input (price
falling) it must move *less*, so as not to over-deliver.

#### `getNextSqrtPriceFromAmount1RoundingDown(uint160 sqrtPX96, uint128 liquidity, uint256 amount, bool add) internal pure → uint160`

`:68-97`. From `y = L·√P`: `√P' = √P ± Δy/L`.

- **`add`** (`:76-84`): quotient rounded **down**, then added. Uses a plain shift
  when `amount <= type(uint160).max` (`:79-81`) and falls back to `mulDiv`
  otherwise — avoiding the expensive path for virtually all real inputs.
- **`!add`** (`:86-95`): quotient rounded **up** (so the difference rounds down),
  with `require(sqrtPX96 > quotient)` (`:93`) preventing underflow.

The comment at `:74` states the rule compactly: "if we're adding (subtracting),
rounding down requires rounding the quotient down (up)".

#### `getNextSqrtPriceFromInput` / `getNextSqrtPriceFromOutput`

`:106-120` and `:129-143`. Both `require(sqrtPX96 > 0)` and
`require(liquidity > 0)`, then dispatch:

| Function | `zeroForOne == true` | `zeroForOne == false` |
|---|---|---|
| `FromInput` | `Amount0RoundingUp(…, add=true)` | `Amount1RoundingDown(…, add=true)` |
| `FromOutput` | `Amount1RoundingDown(…, add=false)` | `Amount0RoundingUp(…, add=false)` |

Selling token0 adds token0 to the pool and removes token1; the table is just that
statement made mechanical.

<a name="64-swapmath"></a>
### 6.4 `SwapMath` — `libraries/SwapMath.sol` (98 lines)

#### `computeSwapStep(uint160 sqrtRatioCurrentX96, uint160 sqrtRatioTargetX96, uint128 liquidity, int256 amountRemaining, uint24 feePips) internal pure → (uint160 sqrtRatioNextX96, uint256 amountIn, uint256 amountOut, uint256 feeAmount)`

`:21-97`. One iteration of the swap loop: how far the price moves within a single
range, and at what cost.

**Direction and mode are inferred, not passed** (`:37-38`):
```solidity
bool zeroForOne = sqrtRatioCurrentX96 >= sqrtRatioTargetX96;   // target below => price falling
bool exactIn    = amountRemaining >= 0;
```

**Phase 1 — can we reach the target?**

*Exact input* (`:40-52`):
```solidity
uint256 amountRemainingLessFee = FullMath.mulDiv(uint256(amountRemaining), 1e6 - feePips, 1e6);
amountIn = zeroForOne ? getAmount0Delta(target, current, liquidity, true)
                      : getAmount1Delta(current, target, liquidity, true);
if (amountRemainingLessFee >= amountIn) sqrtRatioNextX96 = sqrtRatioTargetX96;   // reach it
else sqrtRatioNextX96 = getNextSqrtPriceFromInput(current, liquidity, amountRemainingLessFee, zeroForOne);
```
The fee is removed from the budget **before** any price math — the fee never
participates in the curve.

*Exact output* (`:53-65`): symmetric, computing `amountOut` needed to reach the
target (rounded **down**) and comparing against `-amountRemaining`.

**Phase 2 — the four branches** (`:67-84`). `max` records whether the target was
reached:

| `zeroForOne` | `exactIn` | `max` | `amountIn` | `amountOut` |
|---|---|---|---|---|
| true | true | true | reuse from phase 1 | `getAmount1Delta(next, current, L, false)` |
| true | true | false | `getAmount0Delta(next, current, L, true)` | `getAmount1Delta(next, current, L, false)` |
| true | false | true | `getAmount0Delta(next, current, L, true)` | reuse from phase 1 |
| true | false | false | `getAmount0Delta(next, current, L, true)` | `getAmount1Delta(next, current, L, false)` |

(and the mirror image for `zeroForOne == false`, `:77-84`.) The `max && exactIn`
and `max && !exactIn` guards reuse the already-computed value instead of
recomputing it — correctness *and* gas, since recomputation could differ by a wei
of rounding.

Note the constant rounding discipline: `amountIn` always `roundUp = true`,
`amountOut` always `roundUp = false`.

**Phase 3 — clamp the output** (`:87-89`):
```solidity
if (!exactIn && amountOut > uint256(-amountRemaining)) amountOut = uint256(-amountRemaining);
```
An exact-output swap must never deliver more than requested; rounding could
otherwise overshoot by a wei.

**Phase 4 — the fee** (`:91-96`):
```solidity
if (exactIn && sqrtRatioNextX96 != sqrtRatioTargetX96) {
    feeAmount = uint256(amountRemaining) - amountIn;      // consume the entire remainder
} else {
    feeAmount = FullMath.mulDivRoundingUp(amountIn, feePips, 1e6 - feePips);
}
```

Two genuinely different formulas:

- **Did not reach the target on exact input** — the swap is ending here, so every
  remaining wei becomes fee. This guarantees `amountIn + feeAmount == amountRemaining`
  exactly, with no dust stranded, and is why the doc comment at `:11` can promise
  the fee plus input never exceeds the specified amount.
- **Otherwise** — the fee is grossed *up* from `amountIn`:
  `fee = amountIn · f / (1e6 − f)`, so that `fee / (amountIn + fee) = f / 1e6`.
  For `feePips = 3000` and `amountIn = 1,000,000`, this yields `3010`, and
  `3010 / 1,003,010 = 0.3001%` — the fee is 0.30% of the **total paid**, not of
  the amount that reached the curve, with the excess over exactly 0.30% coming
  from the ceiling. Rounding up always favours LPs.

`SwapMathEchidnaTest.sol:7-51` fuzzes the invariants, including that
`amountIn + feeAmount <= amountRemaining` on exact input.

<a name="65-tickbitmap"></a>
### 6.5 `TickBitmap` — `libraries/TickBitmap.sol` (78 lines)

Answers "where is the next initialized tick?" without scanning storage. Only
*usable* ticks (multiples of `tickSpacing`) are tracked, so the index is the
**compressed** tick `tick / tickSpacing`.

```
compressed = tick / tickSpacing          (floored toward -infinity)
wordPos    = compressed >> 8             int16  — the mapping key
bitPos     = compressed % 256            uint8  — the bit inside the word

mapping(int16 => uint256) tickBitmap;    one 256-bit word = 256 usable ticks
```

#### `position(int24 tick) private pure → (int16 wordPos, uint8 bitPos)`

`:14-17`. `wordPos = int16(tick >> 8)`; `bitPos = uint8(tick % 256)`. The
arithmetic right-shift floors toward negative infinity for negative ticks, which
is exactly what a bitmap index needs. `tick % 256` in Solidity is
sign-following, but the `uint8` cast reinterprets the two's-complement bits and
lands on the right bit regardless.

#### `flipTick(mapping(int16 => uint256) storage self, int24 tick, int24 tickSpacing) internal`

`:23-32`

- **Check** (`:28`): `require(tick % tickSpacing == 0)` — **the only place tick
  spacing is enforced in the entire protocol.** `checkTicks` in the pool validates
  bounds and ordering but never spacing; an unaligned `mint` reverts here.
- **Write** (`:31`): `self[wordPos] ^= 1 << bitPos`. XOR, so the same call both
  sets and clears — the caller (`_updatePosition`, `:431-436`) only calls it when
  `Tick.update` reported a flip.

#### `nextInitializedTickWithinOneWord(mapping(int16 => uint256) storage self, int24 tick, int24 tickSpacing, bool lte) internal view → (int24 next, bool initialized)`

`:42-77`

**The flooring fix** (`:48-49`):
```solidity
int24 compressed = tick / tickSpacing;
if (tick < 0 && tick % tickSpacing != 0) compressed--; // round towards negative infinity
```
Solidity truncates integer division toward zero, so `-5 / 60 == 0`, but tick `-5`
belongs to compressed slot `-1`. Without this correction, negative ticks would map
into the wrong word and the search would skip initialized ticks.

**Searching left, `lte == true`** (`:51-62`):
```solidity
uint256 mask = (1 << bitPos) - 1 + (1 << bitPos);   // all bits at or below bitPos
uint256 masked = self[wordPos] & mask;
initialized = masked != 0;
next = initialized
    ? (compressed - int24(bitPos - BitMath.mostSignificantBit(masked))) * tickSpacing
    : (compressed - int24(bitPos)) * tickSpacing;
```
The mask is written as `(1 << bitPos) - 1 + (1 << bitPos)` rather than
`(1 << (bitPos + 1)) - 1` because `bitPos` is a `uint8` and `bitPos + 1` would
overflow to 0 when `bitPos == 255`. The `mostSignificantBit` of the masked word is
the nearest set bit at or below the current position.

**Searching right, `lte == false`** (`:63-76`):
```solidity
(int16 wordPos, uint8 bitPos) = position(compressed + 1);   // start from the NEXT tick
uint256 mask = ~((1 << bitPos) - 1);                        // all bits at or above bitPos
```
`compressed + 1` implements the strict "greater than" in the doc comment: the
current tick's own state is irrelevant when moving right. `leastSignificantBit`
finds the nearest set bit above.

**Returning uninitialized boundaries.** When the word is empty in the searched
direction the function returns the word's last tick with `initialized == false`.
The swap loop handles this by simply iterating again from there (`:641`), so a
long empty stretch costs one loop iteration per 256 usable ticks rather than one
per tick.

`TickBitmapEchidnaTest.sol:23-46` fuzzes that the returned tick is within one word
and that no initialized tick was skipped.

<a name="66-bitmath"></a>
### 6.6 `BitMath` — `libraries/BitMath.sol` (94 lines)

#### `mostSignificantBit(uint256 x) internal pure → uint8 r`

`:13-45`. `require(x > 0)` (`:14`), then seven `if` blocks halving the search
space (128, 64, 32, 16, 8, 4, 2) plus a final `if (x >= 0x2) r += 1` (`:44`).
Each block shifts `x` right and adds to `r`. Guarantees
`2^r <= x < 2^(r+1)` (`:9-10`).

#### `leastSignificantBit(uint256 x) internal pure → uint8 r`

`:53-93`. `require(x > 0)` (`:54`), then starts at `r = 255` and *subtracts*.
Each block asks whether any bit survives in the low half (`x & type(uintN).max > 0`);
if so the answer is in that half so `r` decreases, otherwise `x` is shifted down.
The inverted structure avoids needing to isolate the lowest bit first.

Both are straight-line and branch-predictable, ~100 gas. Fuzzed by
`BitMathEchidnaTest.sol:7-17`.

<a name="67-tick"></a>
### 6.7 `Tick` — `libraries/Tick.sol` (185 lines)

#### `struct Info` (`:17-37`) — compiler-verified layout, 4 slots

| Field | Type | Slot | Bits | Meaning |
|---|---|---|---|---|
| `liquidityGross` | `uint128` | 0 | 0–127 | total liquidity referencing this tick, from either side |
| `liquidityNet` | `int128` | 0 | 128–255 | liquidity added when crossing left→right |
| `feeGrowthOutside0X128` | `uint256` | 1 | 0–255 | token0 fee growth on the far side |
| `feeGrowthOutside1X128` | `uint256` | 2 | 0–255 | token1 fee growth on the far side |
| `tickCumulativeOutside` | `int56` | 3 | 0–55 | tick·seconds on the far side |
| `secondsPerLiquidityOutsideX128` | `uint160` | 3 | 56–215 | seconds/liquidity on the far side |
| `secondsOutside` | `uint32` | 3 | 216–247 | seconds on the far side |
| `initialized` | `bool` | 3 | 248–255 | `liquidityGross != 0` |

`initialized` is redundant with `liquidityGross != 0`; the comment at `:34-35`
explains it is stored anyway so that crossing a newly initialized tick does not
pay for a fresh `SSTORE` on slot 3.

**`liquidityGross` versus `liquidityNet`.** `Gross` counts every position touching
the tick and decides whether the tick exists at all. `Net` is the signed change
applied when the price crosses — `+ΔL` at a position's lower tick, `−ΔL` at its
upper. A tick that is one position's upper and another's lower can have
`liquidityNet == 0` while `liquidityGross > 0`, and must stay initialized.

#### `tickSpacingToMaxLiquidityPerTick(int24 tickSpacing) internal pure → uint128`

`:44-49`. `type(uint128).max / numTicks`, where `numTicks` counts the usable ticks
in `[MIN_TICK, MAX_TICK]`. Computed once in the pool constructor (`:122`).

| `tickSpacing` | min/max usable tick | `numTicks` | `maxLiquidityPerTick` |
|---|---|---|---|
| 1 | ∓887272 | 1,774,545 | 191757530477355301479181766273477 |
| 10 | ∓887270 | 177,455 | 1917569901783203986719870431555990 |
| 60 | ∓887220 | 29,575 | 11505743598341114571880798222544994 |
| 200 | ∓887200 | 8,873 | 38350317471085141830651933667504588 |

The bound guarantees that even if *every* usable tick carried the maximum, the
pool's total `liquidity` could not overflow `uint128`.

#### `getFeeGrowthInside(...) internal view → (uint256 feeGrowthInside0X128, uint256 feeGrowthInside1X128)`

`:60-95`. The fee-accounting trick, and worth understanding precisely.

**The invariant.** `feeGrowthOutside` of a tick means *fee growth accumulated on
the side of that tick away from the current price*. It is meaningless in absolute
terms — it depends on when the tick was initialized — but differences of it over
time are exact.

```
 feeGrowthGlobal (all fees ever, per unit L)
 ├──────────────── below ────────────────┬──── inside ────┬──────── above ────────┤
                                    tickLower         tickUpper
                                              ▲
                                          current tick
```

```solidity
if (tickCurrent >= tickLower) below = lower.feeGrowthOutside;          // :74-76
else                          below = global - lower.feeGrowthOutside;  // :78-79

if (tickCurrent <  tickUpper) above = upper.feeGrowthOutside;          // :85-87
else                          above = global - upper.feeGrowthOutside;  // :89-90

inside = global - below - above;                                       // :93-94
```

**Why the two cases.** "Outside" is relative to where the price *is now*. When the
price is above `tickLower`, "outside `tickLower`" already means "below it", so the
stored value is used directly. When the price is below `tickLower`, "outside" means
"above", so the below-portion is the complement.

**Why it is correct across crossings.** `cross` (below) replaces
`feeGrowthOutside` with `global − feeGrowthOutside` every time the price passes
the tick, which is exactly the operation that flips the meaning of "outside" from
one side to the other. The invariant is therefore maintained inductively, and
`inside` always measures precisely the fees earned while the price was between the
two ticks.

**Deliberate underflow.** All three subtractions are expected to wrap. In
Solidity 0.7 arithmetic is unchecked by default, and the wrap is not merely
tolerated but required — see `Position.update`.

#### `update(...) internal returns (bool flipped)`

`:110-150`

| Line | Step |
|---|---|
| `:125-126` | `liquidityGrossAfter = addDelta(liquidityGrossBefore, liquidityDelta)` |
| `:128` | `require(liquidityGrossAfter <= maxLiquidity, 'LO')` |
| `:130` | `flipped = (after == 0) != (before == 0)` |
| `:132-142` | if newly initialized, seed the outside values |
| `:144` | store `liquidityGross` |
| `:147-149` | `liquidityNet` — **subtract** for an upper tick, **add** for a lower |

**The initialization convention** (`:132-142`, comment `:133`): "by convention, we
assume that all growth before a tick was initialized happened *below* the tick."
So if `tick <= tickCurrent`, the accumulators are seeded with the current global
values; otherwise they stay zero. Either way, `getFeeGrowthInside` yields a
consistent `inside` from that point forward. The absolute number is meaningless;
only its evolution matters.

The sign rule at `:147-149` follows from the comment at `:146`: crossing a lower
tick left-to-right *adds* the position's liquidity; crossing an upper tick
left-to-right *removes* it.

#### `clear(mapping(int24 => Tick.Info) storage self, int24 tick) internal`

`:155-157`. `delete self[tick]` — four slots zeroed for a gas refund, called from
`_updatePosition` (`:447`, `:450`) only when liquidity is being removed and the
tick flipped.

#### `cross(...) internal returns (int128 liquidityNet)`

`:168-184`. Five mirror operations plus a read:
```solidity
info.feeGrowthOutside0X128 = feeGrowthGlobal0X128 - info.feeGrowthOutside0X128;
info.feeGrowthOutside1X128 = feeGrowthGlobal1X128 - info.feeGrowthOutside1X128;
info.secondsPerLiquidityOutsideX128 = secondsPerLiquidityCumulativeX128 - info.secondsPerLiquidityOutsideX128;
info.tickCumulativeOutside = tickCumulative - info.tickCumulativeOutside;
info.secondsOutside = time - info.secondsOutside;
liquidityNet = info.liquidityNet;
```
`x ← global − x` is an involution with respect to the moving `global`: applying it
on the way out and again on the way back restores the correct relative meaning.
All five wrap deliberately. `cross` does **not** apply the sign flip for leftward
movement — the caller does (`UniswapV3Pool.sol:720`).

`TickOverflowSafetyEchidnaTest.sol` fuzzes that these deliberate overflows never
corrupt the `inside` computation.

<a name="68-position"></a>
### 6.8 `Position` — `libraries/Position.sol` (88 lines)

#### `struct Info` (`:13-22`) — 4 slots

| Field | Type | Slot | Bits |
|---|---|---|---|
| `liquidity` | `uint128` | 0 | 0–127 |
| `feeGrowthInside0LastX128` | `uint256` | 1 | 0–255 |
| `feeGrowthInside1LastX128` | `uint256` | 2 | 0–255 |
| `tokensOwed0` | `uint128` | 3 | 0–127 |
| `tokensOwed1` | `uint128` | 3 | 128–255 |

#### `get(mapping(bytes32 => Info) storage self, address owner, int24 tickLower, int24 tickUpper) internal view → Info storage`

`:30-37`. `self[keccak256(abi.encodePacked(owner, tickLower, tickUpper))]`.

`encodePacked` on `(address, int24, int24)` gives a fixed 26 bytes with no
ambiguity, since every component is fixed-width. A position is identified purely
by this triple — the pool has no notion of an NFT. Two mints to the same owner and
range merge into one position, which is why `NonfungiblePositionManager` mints
each NFT position under its own `owner` key.

#### `update(Info storage self, int128 liquidityDelta, uint256 feeGrowthInside0X128, uint256 feeGrowthInside1X128) internal`

`:44-87`

| Line | Step |
|---|---|
| `:50` | copy to memory once |
| `:53-58` | `liquidityDelta == 0` requires existing liquidity, else `'NP'` |
| `:61-76` | `tokensOwed = mulDiv(feeGrowthInside - feeGrowthInsideLast, liquidity, Q128)` |
| `:79` | write new liquidity (only if it changed) |
| `:80-81` | snapshot the new `feeGrowthInsideLast` |
| `:82-86` | accumulate `tokensOwed` |

**The core formula.** Fees are tracked as growth *per unit of liquidity*, so a
position's earnings are `(growth_now − growth_at_last_touch) × its liquidity`.
This is what makes fee accounting O(1) per position regardless of how many swaps
occurred: no per-swap iteration over positions, just two numbers per position.

**`'NP'` — no poke on an empty position** (`:54`). A zero-liquidity update would
be a no-op that still costs gas and could be used to spam events.

**Fees are computed on the OLD liquidity** (`_self.liquidity` at `:65` and `:73`),
before `liquidityNext` is written at `:79`. Correct: the fees accrued while the
position had its previous size.

**Deliberate overflow** (comment `:83`): "overflow is acceptable, have to withdraw
before you hit type(uint128).max fees". Both the `feeGrowthInside` subtraction
(which relies on wrapping) and the `tokensOwed` accumulation are unchecked. A
position would need `2^128` wei of uncollected fees to wrap `tokensOwed` — not
reachable for real tokens.

<a name="69-oracle"></a>
### 6.9 `Oracle` — `libraries/Oracle.sol` (325 lines)

A ring buffer of up to 65,535 observations, each exactly one storage slot.

#### `struct Observation` (`:12-21`) — exactly 32 bytes

| Field | Type | Bits | Meaning |
|---|---|---|---|
| `blockTimestamp` | `uint32` | 0–31 | truncated `block.timestamp` |
| `tickCumulative` | `int56` | 32–87 | `Σ tick · Δt` since pool init |
| `secondsPerLiquidityCumulativeX128` | `uint160` | 88–247 | `Σ Δt / max(1, L)` as Q128.128 |
| `initialized` | `bool` | 248–255 | slot has been written |

Fitting in one slot is why the array can be 65,535 entries without the write cost
becoming prohibitive. `int56` holds `tick · seconds`: with `|tick| < 2^20` and
`t < 2^32`, the product needs 52 bits — 56 is the next packing-friendly size.

#### `transform(Observation last, uint32 blockTimestamp, int24 tick, uint128 liquidity) private pure → Observation`

`:30-45`
```solidity
uint32 delta = blockTimestamp - last.blockTimestamp;
tickCumulative: last.tickCumulative + int56(tick) * delta,
secondsPerLiquidityCumulativeX128: last.secondsPerLiquidityCumulativeX128
    + ((uint160(delta) << 128) / (liquidity > 0 ? liquidity : 1)),
```
`max(1, liquidity)` (`:42`) avoids division by zero when the pool has no in-range
liquidity. `delta` is `uint32` subtraction, which wraps correctly across the 2106
boundary.

**`secondsPerLiquidityCumulative` semantics.** Its difference over a window,
multiplied by a position's liquidity, gives the "liquidity-seconds" that position
contributed — the basis for staking rewards proportional to time-weighted
liquidity.

#### `initialize(Observation[65535] storage self, uint32 time) internal → (uint16 cardinality, uint16 cardinalityNext)`

`:52-63`. Writes slot 0 with zero accumulators and returns `(1, 1)`. Called once,
from `UniswapV3Pool.initialize` (`:276`).

#### `write(...) internal → (uint16 indexUpdated, uint16 cardinalityUpdated)`

`:78-101`

| Line | Step |
|---|---|
| `:90` | if an observation already exists for this timestamp, return unchanged — **at most one write per block** |
| `:93-97` | grow cardinality, but only when `index == cardinality - 1` (wrapping point) |
| `:99` | `indexUpdated = (index + 1) % cardinalityUpdated` |
| `:100` | `self[indexUpdated] = transform(last, …)` |

The cardinality bump is deferred to the wrap point (comment `:66-68`) to preserve
ordering: expanding mid-array would leave a gap of uninitialized slots inside the
chronological sequence, breaking binary search.

**One write per block** means the oracle records the tick at the *start* of the
first swap in a block — the price *before* any manipulation within that block.

#### `grow(Observation[65535] storage self, uint16 current, uint16 next) internal → uint16`

`:108-120`. `require(current > 0, 'I')` (`:113`); no-op if `next <= current`
(`:115`); otherwise writes `blockTimestamp = 1` into each new slot (`:118`). The
comment (`:116-117`) explains: the data is never read because `initialized` stays
false, but the non-zero write converts a future 20,000-gas cold `SSTORE` into a
~2,900-gas warm one. `1` rather than any other value because it is the cheapest
non-zero.

#### `lte(uint32 time, uint32 a, uint32 b) private pure → bool`

`:128-140`. Overflow-safe chronological comparison. If both `a` and `b` are `<= time`
no adjustment is needed (`:134`); otherwise the one that appears to be in the
future is treated as pre-wrap by adding `2^32` (`:136-137`). Safe for zero or one
overflow — enough for ~136 years.

#### `binarySearch(...) private view → (Observation beforeOrAt, Observation atOrAfter)`

`:153-184`. Searches the ring buffer over the *virtual* index range
`[index+1, index+cardinality]` (`:160-161`), taking `% cardinality` on each access
(`:166`, `:174`) so the wrap is invisible to the search. Uninitialized slots are
skipped upward (`:169-172`). Terminates when `target` lies between two adjacent
observations (`:179`).

#### `getSurroundingObservations(...) private view → (Observation beforeOrAt, Observation atOrAfter)`

`:198-230`. Three fast paths before the binary search:
- target at or after the newest observation (`:211-219`): if equal, return it; else
  `transform` the newest forward to the target — the **counterfactual** value;
- else check the oldest and `require(lte(time, beforeOrAt.blockTimestamp, target), 'OLD')`
  (`:226`) — the requested window predates stored history;
- else binary search (`:229`).

The `self[0]` fallback at `:223` handles a buffer that has not yet wrapped.

#### `observeSingle(...) internal view → (int56 tickCumulative, uint160 secondsPerLiquidityCumulativeX128)`

`:245-287`

- `secondsAgo == 0` (`:254-258`): return the newest, transformed to now if stale.
  This is the "current accumulator" path used by `_updatePosition` (`:396`) and the
  swap loop (`:699`).
- Otherwise locate the bracketing pair and interpolate **linearly** (`:271-286`):
  ```
  result = before + (after - before) * targetDelta / observationTimeDelta
  ```
  Linear interpolation of a cumulative is exact if the tick was constant between
  the two observations, and an approximation otherwise. Since an observation is
  written whenever the tick changes, the approximation is tight in practice.

#### `observe(...) internal view → (int56[] , uint160[])`

`:300-324`. `require(cardinality > 0, 'I')` (`:309`), then loops `observeSingle`
over the array (`:313-323`). Any element out of range reverts the whole call with
`'OLD'`.

`OracleEchidnaTest.sol:73-116` asserts the structural invariants: index always
below cardinality, cardinality never above `cardinalityNext`, observation 0 always
readable once initialized, and time-weighted averages always in range.

<a name="610-liquiditymath"></a>
### 6.10 `LiquidityMath` — `libraries/LiquidityMath.sol` (16 lines)

#### `addDelta(uint128 x, int128 y) internal pure → uint128 z`

`:10-16`
```solidity
if (y < 0) require((z = x - uint128(-y)) < x, 'LS');
else       require((z = x + uint128(y)) >= x, 'LA');
```
`'LS'` = Liquidity Subtraction underflow, `'LA'` = Liquidity Addition overflow.
Checks the result rather than the operands — cheaper, and exact in wrapping
arithmetic. Note `< x` (strict) in the subtraction branch: `y < 0` guarantees a
strict decrease, so equality would itself indicate a wrap.

Used at `UniswapV3Pool.sol:361` and `:722`, and in `Tick.update` (`Tick.sol:126`).

<a name="611-safecast"></a>
### 6.11 `SafeCast` — `libraries/SafeCast.sol` (28 lines)

| Function | Line | Check |
|---|---|---|
| `toUint160(uint256 y) → uint160 z` | `:10-12` | `require((z = uint160(y)) == y)` |
| `toInt128(int256 y) → int128 z` | `:17-19` | `require((z = int128(y)) == y)` |
| `toInt256(uint256 y) → int256 z` | `:24-27` | `require(y < 2**255)` |

The cast-and-compare idiom is the cheapest possible range check. All three are
bare `require`s. Used pervasively; `toInt128` in particular is what guarantees
`liquidityNet` can never be `type(int128).min`, which the swap loop's negation at
`:720` relies on.

<a name="612-lowgassafemath"></a>
### 6.12 `LowGasSafeMath` — `libraries/LowGasSafeMath.sol` (46 lines)

| Function | Line | Check |
|---|---|---|
| `add(uint256, uint256)` | `:11-13` | `require((z = x + y) >= x)` |
| `sub(uint256, uint256)` | `:19-21` | `require((z = x - y) <= x)` |
| `mul(uint256, uint256)` | `:27-29` | `require(x == 0 \|\| (z = x * y) / x == y)` |
| `add(int256, int256)` | `:35-37` | `require((z = x + y) >= x == (y >= 0))` |
| `sub(int256, int256)` | `:43-45` | `require((z = x - y) <= x == (y >= 0))` |

The signed variants encode "adding a non-negative number must not decrease the
result, and adding a negative one must not increase it" as a single boolean
comparison — cheaper than OpenZeppelin's branching equivalent, hence the name.
`pragma solidity >=0.7.0` (`:2`), since the trick assumes wrapping arithmetic.

<a name="613-unsafemath"></a>
### 6.13 `UnsafeMath` — `libraries/UnsafeMath.sol` (17 lines)

#### `divRoundingUp(uint256 x, uint256 y) internal pure → uint256 z`

`:12-16`
```solidity
assembly { z := add(div(x, y), gt(mod(x, y), 0)) }
```
Branchless ceiling division. "Unsafe" because division by zero is not checked —
the doc comment (`:8`) states it "must be checked externally". Every call site
does: `SqrtPriceMath.sol:164` requires `sqrtRatioAX96 > 0` before `:168`, and
`:47` and `:89` divide by `liquidity` after `getNextSqrtPriceFrom*` required it
non-zero.

Note EVM `div` returns 0 for a zero divisor rather than trapping, so an unchecked
call would silently produce a wrong answer instead of reverting — which is exactly
why the call sites carry the checks.

<a name="614-transferhelper"></a>
### 6.14 `TransferHelper` — `libraries/TransferHelper.sol` (23 lines)

#### `safeTransfer(address token, address to, uint256 value) internal`

`:14-22`
```solidity
(bool success, bytes memory data) = token.call(abi.encodeWithSelector(IERC20Minimal.transfer.selector, to, value));
require(success && (data.length == 0 || abi.decode(data, (bool))), 'TF');
```
Handles the three real-world ERC-20 return conventions: returns `true`, returns
nothing (USDT), or returns `false`. Only the third is rejected. `'TF'` = Transfer
Failed.

Note there is **no `safeTransferFrom`** in V3 core — the pool never pulls tokens.
It only ever pushes, and relies on callbacks plus balance checks for incoming
funds. That asymmetry is the core's entire payment design.

<a name="615-fixedpoint"></a>
### 6.15 `FixedPoint96` and `FixedPoint128`

`FixedPoint96.sol:7-10`: `RESOLUTION = 96`, `Q96 = 0x1000000000000000000000000` (`2^96`).
`FixedPoint128.sol:6-8`: `Q128 = 0x100000000000000000000000000000000` (`2^128`).

Two conventions coexist deliberately. **Prices** are Q64.96 (`sqrtPriceX96`):
96 fractional bits leave 64 integer bits, and `√price` fits `uint160`, which packs
into `slot0` alongside everything else. **Fee growth** is Q128.128: fees per unit
of liquidity are tiny numbers needing maximum fractional precision, and the value
occupies a full `uint256` slot anyway, so there is no packing pressure.

---

<a name="7-interfaces"></a>
## 7. Interfaces

Thirteen files. The pool's is split six ways so integrators import only what they
need.

### `IUniswapV3Pool` — `interfaces/IUniswapV3Pool.sol:15-22`

Inherits, in order: `IUniswapV3PoolImmutables`, `IUniswapV3PoolState`,
`IUniswapV3PoolDerivedState`, `IUniswapV3PoolActions`,
`IUniswapV3PoolOwnerActions`, `IUniswapV3PoolEvents`. Adds nothing itself. The doc
comment (`:14`) explains the split is to keep the interface digestible.

### `IUniswapV3PoolImmutables` — `interfaces/pool/IUniswapV3PoolImmutables.sol`

| Function | Line | Returns |
|---|---|---|
| `factory()` | `:9` | `address` |
| `token0()` | `:13` | `address` |
| `token1()` | `:17` | `address` |
| `fee()` | `:21` | `uint24` |
| `tickSpacing()` | `:28` | `int24` |
| `maxLiquidityPerTick()` | `:34` | `uint128` |

The comment at `:31-32` is worth reading: `maxLiquidityPerTick` exists both to stop
`uint128` overflow and to prevent out-of-range liquidity from being used to block
in-range liquidity.

### `IUniswapV3PoolState` — `interfaces/pool/IUniswapV3PoolState.sol`

| Function | Line | Returns |
|---|---|---|
| `slot0()` | `:21` | `(uint160 sqrtPriceX96, int24 tick, uint16 observationIndex, uint16 observationCardinality, uint16 observationCardinalityNext, uint8 feeProtocol, bool unlocked)` |
| `feeGrowthGlobal0X128()` | `:36` | `uint256` |
| `feeGrowthGlobal1X128()` | `:40` | `uint256` |
| `protocolFees()` | `:44` | `(uint128 token0, uint128 token1)` |
| `liquidity()` | `:48` | `uint128` |
| `ticks(int24)` | `:64` | the seven `Tick.Info` fields |
| `tickBitmap(int16)` | `:79` | `uint256` |
| `positions(bytes32)` | `:88` | the five `Position.Info` fields |
| `observations(uint256)` | `:107` | the four `Observation` fields |

All are auto-generated public getters, so a struct is returned as a flattened
tuple — the generated ABI has no struct type.

### `IUniswapV3PoolDerivedState` — `interfaces/pool/IUniswapV3PoolDerivedState.sol`

`observe(uint32[])` (`:18`) and `snapshotCumulativesInside(int24, int24)` (`:32`).
"Derived" because neither is stored: both are computed from `observations` plus
the current state.

### `IUniswapV3PoolActions` — `interfaces/pool/IUniswapV3PoolActions.sol`

`initialize` (`:10`), `mint` (`:23`), `collect` (`:43`), `burn` (`:59`),
`swap` (`:75`), `flash` (`:91`), `increaseObservationCardinalityNext` (`:102`).

### `IUniswapV3PoolOwnerActions` — `interfaces/pool/IUniswapV3PoolOwnerActions.sol`

`setFeeProtocol(uint8, uint8)` (`:10`) and
`collectProtocol(address, uint128, uint128)` (`:18`).

### `IUniswapV3PoolEvents` — `interfaces/pool/IUniswapV3PoolEvents.sol`

All nine events; see [§11](#11-events-reference).

### `IUniswapV3Factory` — `interfaces/IUniswapV3Factory.sol`

Events `OwnerChanged` (`:10`), `PoolCreated` (`:18`), `FeeAmountEnabled` (`:29`);
functions `owner()` (`:34`), `feeAmountTickSpacing(uint24)` (`:40`),
`getPool(address,address,uint24)` (`:48`), `createPool` (`:62`),
`setOwner` (`:71`), `enableFeeAmount` (`:77`).

### `IUniswapV3PoolDeployer` — `interfaces/IUniswapV3PoolDeployer.sol`

One function, `parameters()` (`:16`), returning
`(address factory, address token0, address token1, uint24 fee, int24 tickSpacing)`.
The doc comment (`:5-7`) states the design rationale: constructor arguments are
avoided so the init code hash stays constant.

### `IERC20Minimal` — `interfaces/IERC20Minimal.sol`

`balanceOf` (`:10`), `transfer` (`:16`), `allowance` (`:22`), `approve` (`:28`),
`transferFrom` (`:35`); events `Transfer` (`:45`), `Approval` (`:51`).

Deliberately minimal: no `totalSupply`, `name`, `symbol` or `decimals`. The pool
actually calls only `balanceOf` (via `staticcall`) and `transfer` (via
`TransferHelper`); the rest exist for test contracts and integrators.

### The three callbacks

| Interface | Function | Line | Called from |
|---|---|---|---|
| `IUniswapV3MintCallback` | `uniswapV3MintCallback(uint256 amount0Owed, uint256 amount1Owed, bytes data)` | `:13` | `UniswapV3Pool.sol:482` |
| `IUniswapV3SwapCallback` | `uniswapV3SwapCallback(int256 amount0Delta, int256 amount1Delta, bytes data)` | `:16` | `UniswapV3Pool.sol:776`, `:782` |
| `IUniswapV3FlashCallback` | `uniswapV3FlashCallback(uint256 fee0, uint256 fee1, bytes data)` | `:13` | `UniswapV3Pool.sol:808` |

Mint and flash callbacks receive **unsigned** amounts owed. The swap callback
receives **signed deltas**: positive means the caller owes the pool, negative
means the pool already sent that much. An implementation must pay only the
positive side and ignore the negative one.

---

<a name="8-test-contracts"></a>
## 8. Test contracts

Not deployed, but they are the executable specification. The `*EchidnaTest`
files state the properties the math must satisfy under fuzzing; the `*Test` files
are thin wrappers exposing `internal` library functions plus `getGasCostOf*`
helpers that measure a call by differencing `gasleft()`.

### Harnesses that expose libraries

| Contract | File | Exposes |
|---|---|---|
| `BitMathTest` | `test/BitMathTest.sol:6` | `mostSignificantBit` `:7`, `leastSignificantBit` `:17`, + gas helpers |
| `FullMathTest` | `test/FullMathTest.sol:6` | `mulDiv` `:7`, `mulDivRoundingUp` `:15` |
| `LiquidityMathTest` | `test/LiquidityMathTest.sol:6` | `addDelta` `:7` |
| `SqrtPriceMathTest` | `test/SqrtPriceMathTest.sol:6` | all four `getNextSqrtPriceFrom*` / `getAmount*Delta` `:7-76` |
| `SwapMathTest` | `test/SwapMathTest.sol:6` | `computeSwapStep` `:7` |
| `TickTest` | `test/TickTest.sol:7` | `tickSpacingToMaxLiquidityPerTick` `:12`, `setTick` `:16`, `getFeeGrowthInside` `:20`, `update` `:30`, `clear` `:57`, `cross` `:61` |
| `TickBitmapTest` | `test/TickBitmapTest.sol:6` | `flipTick` `:11`, `nextInitializedTickWithinOneWord` `:21`, `isInitialized` `:36` |
| `TickMathTest` | `test/TickMathTest.sol:6` | `getSqrtRatioAtTick` `:7`, `getTickAtSqrtRatio` `:17`, the two ratio constants `:27`, `:31` |
| `OracleTest` | `test/OracleTest.sol:7` | `initialize` `:25`, `advanceTime` `:33`, `update` `:44`, `batchUpdate` `:51`, `grow` `:82`, `observe` `:86` |

### Echidna invariant suites

| Contract | File | Key properties |
|---|---|---|
| `BitMathEchidnaTest` | `:6` | `2^msb <= x < 2^(msb+1)`; lsb isolates the lowest set bit |
| `FullMathEchidnaTest` | `:6` | `mulDivRoundingUp − mulDiv ∈ {0,1}`; `mulDiv` inverts multiplication `:7-66` |
| `LowGasSafeMathEchidnaTest` | `:6` | signed and unsigned add/sub/mul match checked arithmetic `:7-35` |
| `SqrtPriceMathEchidnaTest` | `:8` | price always moves in the expected direction; amounts are monotonic in liquidity; in-range and out-of-range mint invariants `:196-231` |
| `SwapMathEchidnaTest` | `:6` | `amountIn + feeAmount <= amountRemaining` on exact input; the price never passes the target `:7-51` |
| `TickEchidnaTest` | `:6` | `maxLiquidityPerTick × numTicks` cannot overflow `uint128` `:7` |
| `TickBitmapEchidnaTest` | `:6` | the returned tick is within one word and no initialized tick is skipped `:23-46` |
| `TickMathEchidnaTest` | `:6` | round-tripping and monotonicity of both directions `:8-21` |
| `TickOverflowSafetyEchidnaTest` | `:6` | the deliberate `feeGrowthOutside` wraps never corrupt `getFeeGrowthInside` `:25-110` |
| `OracleEchidnaTest` | `:7` | index < cardinality `:73`; always initialized `:77`; cardinality ≤ next `:82`; observation 0 always readable `:86`; TWAPs always in range `:113` |
| `UnsafeMathEchidnaTest` | `:6` | `divRoundingUp` equals ceiling division `:7` |

### Mocks and callers

**`MockTimeUniswapV3Pool`** — `test/MockTimeUniswapV3Pool.sol:7`, extends the real
pool. Holds `time` seeded to `1601906400` (`:9`), overrides `_blockTimestamp()`
(`:23`) and adds `advanceTime` (`:19`) plus direct setters for both
`feeGrowthGlobal` values (`:11`, `:15`). This is the only reason
`_blockTimestamp` is `virtual`.

**`MockTimeUniswapV3PoolDeployer`** — `:8`, deploys the mock pool with a public
`deploy` (`:21`) and a `PoolDeployed` event (`:19`).

**`TestUniswapV3Callee`** — `:15`, implements all three callbacks. Provides the
six canonical swap shapes: `swapExact0For1` `:18`, `swap0ForExact1` `:27`,
`swapExact1For0` `:36`, `swap1ForExact0` `:45`, `swapToLowerSqrtPrice` `:54`,
`swapToHigherSqrtPrice` `:62`; plus `mint` `:91` and `flash` `:119`. The callbacks
(`:72`, `:103`, `:130`) emit events and pay from a stored payer. This contract is
the reference implementation of a V3 integrator.

**`TestUniswapV3ReentrantCallee`** — `:10`. Its callback (`:17-55`) attempts to
reenter `swap`, `mint`, `collect`, `burn`, `flash` and `collectProtocol`, and
asserts each reverts with exactly `'LOK'` (`:11`, `:26`). It ends with
`require(false, 'Unable to reenter')` (`:54`) so the test passes only if every one
of the six was blocked. This is the reentrancy guard's proof.

**`TestUniswapV3Router`** — `:11`, multi-hop routing across two pools
(`swapForExact0Multi` `:15`, `swapForExact1Multi` `:33`) with a callback (`:52`)
that recursively initiates the next hop — the pattern v3-periphery's `SwapRouter`
generalises.

**`TestUniswapV3SwapPay`** — `:9`, a swap where the callback pays explicitly
specified amounts (`:10`, `:28`), used to test underpayment and overpayment.

**`UniswapV3PoolSwapTest`** — `:9`, `getSwapResult` (`:13`) returns the resulting
state for assertions.

**`NoDelegateCallTest`** — `:6`, proves the modifier works: `canBeDelegateCalled`
(`:7`) versus `cannotBeDelegateCalled` (`:11`), plus `callsIntoNoDelegateCallFunction`
(`:27`) showing that an internal call into a `noDelegateCall` private function
(`:31`) still succeeds.

**`TestERC20`** — `:6`, a minimal mintable token (`mint` `:14`).

---

<a name="9-abi--selector-tables"></a>
## 9. ABI / selector tables

Computed with `cast sig`.

### `UniswapV3Pool`

| Selector | Signature | Mutability | Access |
|---|---|---|---|
| `0xc45a0155` | `factory()` | view | public |
| `0x0dfe1681` | `token0()` | view | public |
| `0xd21220a7` | `token1()` | view | public |
| `0xddca3f43` | `fee()` | view | public |
| `0xd0c93a7c` | `tickSpacing()` | view | public |
| `0x70cf754a` | `maxLiquidityPerTick()` | view | public |
| `0x3850c7bd` | `slot0()` | view | public |
| `0xf3058399` | `feeGrowthGlobal0X128()` | view | public |
| `0x46141319` | `feeGrowthGlobal1X128()` | view | public |
| `0x1ad8b03b` | `protocolFees()` | view | public |
| `0x1a686502` | `liquidity()` | view | public |
| `0xf30dba93` | `ticks(int24)` | view | public |
| `0x5339c296` | `tickBitmap(int16)` | view | public |
| `0x514ea4bf` | `positions(bytes32)` | view | public |
| `0x252c09d7` | `observations(uint256)` | view | public |
| `0x883bdbfd` | `observe(uint32[])` | view | public |
| `0xa38807f2` | `snapshotCumulativesInside(int24,int24)` | view | public |
| `0xf637731d` | `initialize(uint160)` | nonpayable | public, once |
| `0x3c8a7d8d` | `mint(address,int24,int24,uint128,bytes)` | nonpayable | public |
| `0xa34123a7` | `burn(int24,int24,uint128)` | nonpayable | position owner |
| `0x4f1eb3d8` | `collect(address,int24,int24,uint128,uint128)` | nonpayable | position owner |
| `0x128acb08` | `swap(address,bool,int256,uint160,bytes)` | nonpayable | public |
| `0x490e6cbc` | `flash(address,uint256,uint256,bytes)` | nonpayable | public |
| `0x32148f67` | `increaseObservationCardinalityNext(uint16)` | nonpayable | public |
| `0x8206a4d1` | `setFeeProtocol(uint8,uint8)` | nonpayable | **factory owner** |
| `0x85b66729` | `collectProtocol(address,uint128,uint128)` | nonpayable | **factory owner** |

### `UniswapV3Factory`

| Selector | Signature | Mutability | Access |
|---|---|---|---|
| `0x8da5cb5b` | `owner()` | view | public |
| `0x22afcccb` | `feeAmountTickSpacing(uint24)` | view | public |
| `0x1698ee82` | `getPool(address,address,uint24)` | view | public |
| `0x89035730` | `parameters()` | view | public (inherited) |
| `0xa1671295` | `createPool(address,address,uint24)` | nonpayable | public |
| `0x13af4035` | `setOwner(address)` | nonpayable | **owner** |
| `0x8a7c195f` | `enableFeeAmount(uint24,int24)` | nonpayable | **owner** |

### Callbacks (implemented by integrators, called by the pool)

| Selector | Signature |
|---|---|
| `0xd3487997` | `uniswapV3MintCallback(uint256,uint256,bytes)` |
| `0xfa461e33` | `uniswapV3SwapCallback(int256,int256,bytes)` |
| `0xe9cbafb0` | `uniswapV3FlashCallback(uint256,uint256,bytes)` |

### `IERC20Minimal` (called by the pool)

| Selector | Signature | Used at |
|---|---|---|
| `0x70a08231` | `balanceOf(address)` | `UniswapV3Pool.sol:142`, `:152` |
| `0xa9059cbb` | `transfer(address,uint256)` | `TransferHelper.sol:20` |
| `0xdd62ed3e` | `allowance(address,address)` | not used by core |
| `0x095ea7b3` | `approve(address,uint256)` | not used by core |
| `0x23b872dd` | `transferFrom(address,address,uint256)` | **never** — the pool does not pull |

---

<a name="10-storage-layout-tables"></a>
## 10. Storage layout tables

Produced by `forge inspect <contract> storageLayout` at solc 0.7.6, optimizer on,
800 runs.

### `UniswapV3Pool`

| Slot | Offset | Bytes | Name | Type |
|---|---|---|---|---|
| 0 | 0 | 32 | `slot0` | `struct Slot0` |
| 1 | 0 | 32 | `feeGrowthGlobal0X128` | `uint256` |
| 2 | 0 | 32 | `feeGrowthGlobal1X128` | `uint256` |
| 3 | 0 | 32 | `protocolFees` | `struct ProtocolFees` |
| 4 | 0 | 16 | `liquidity` | `uint128` |
| 5 | 0 | 32 | `ticks` | `mapping(int24 => Tick.Info)` |
| 6 | 0 | 32 | `tickBitmap` | `mapping(int16 => uint256)` |
| 7 | 0 | 32 | `positions` | `mapping(bytes32 => Position.Info)` |
| 8 | 0 | 2,097,120 | `observations` | `Oracle.Observation[65535]` |

The immutables (`factory`, `token0`, `token1`, `fee`, `tickSpacing`,
`maxLiquidityPerTick`) occupy **no** storage. `observations` spans slots 8 through
65,542 at exactly one slot per entry.

Computing a mapping slot:
- `ticks[t]` → `keccak256(abi.encode(int256(t), uint256(5)))`, 4 consecutive slots
- `tickBitmap[w]` → `keccak256(abi.encode(int256(w), uint256(6)))`, 1 slot
- `positions[k]` → `keccak256(abi.encode(k, uint256(7)))`, 4 consecutive slots
- `observations[i]` → slot `8 + i`

### `UniswapV3Factory`

| Slot | Offset | Bytes | Name | Type |
|---|---|---|---|---|
| 0 | 0 | 20 | `parameters.factory` | `address` |
| 1 | 0 | 20 | `parameters.token0` | `address` |
| 2 | 0 | 20 | `parameters.token1` | `address` |
| 2 | 20 | 3 | `parameters.fee` | `uint24` |
| 2 | 23 | 3 | `parameters.tickSpacing` | `int24` |
| 3 | 0 | 20 | `owner` | `address` |
| 4 | 0 | 32 | `feeAmountTickSpacing` | `mapping(uint24 => int24)` |
| 5 | 0 | 32 | `getPool` | nested mapping |

`parameters` inherits from `UniswapV3PoolDeployer` and therefore comes **first**,
occupying slots 0–2 (96 bytes). `owner` lands at slot 3.

### Packed struct bit layouts

**`Slot0`** (pool slot 0)

| Field | Type | Byte offset | Bits |
|---|---|---|---|
| `sqrtPriceX96` | `uint160` | 0 | 0–159 |
| `tick` | `int24` | 20 | 160–183 |
| `observationIndex` | `uint16` | 23 | 184–199 |
| `observationCardinality` | `uint16` | 25 | 200–215 |
| `observationCardinalityNext` | `uint16` | 27 | 216–231 |
| `feeProtocol` | `uint8` | 29 | 232–239 |
| `unlocked` | `bool` | 30 | 240–247 |

**`ProtocolFees`** (pool slot 3): `token0` `uint128` bits 0–127, `token1`
`uint128` bits 128–255.

**`Tick.Info`** (4 slots)

| Field | Type | Slot | Byte offset | Bits |
|---|---|---|---|---|
| `liquidityGross` | `uint128` | 0 | 0 | 0–127 |
| `liquidityNet` | `int128` | 0 | 16 | 128–255 |
| `feeGrowthOutside0X128` | `uint256` | 1 | 0 | 0–255 |
| `feeGrowthOutside1X128` | `uint256` | 2 | 0 | 0–255 |
| `tickCumulativeOutside` | `int56` | 3 | 0 | 0–55 |
| `secondsPerLiquidityOutsideX128` | `uint160` | 3 | 7 | 56–215 |
| `secondsOutside` | `uint32` | 3 | 27 | 216–247 |
| `initialized` | `bool` | 3 | 31 | 248–255 |

**`Position.Info`** (4 slots)

| Field | Type | Slot | Byte offset | Bits |
|---|---|---|---|---|
| `liquidity` | `uint128` | 0 | 0 | 0–127 |
| `feeGrowthInside0LastX128` | `uint256` | 1 | 0 | 0–255 |
| `feeGrowthInside1LastX128` | `uint256` | 2 | 0 | 0–255 |
| `tokensOwed0` | `uint128` | 3 | 0 | 0–127 |
| `tokensOwed1` | `uint128` | 3 | 16 | 128–255 |

**`Oracle.Observation`** (exactly 1 slot)

| Field | Type | Byte offset | Bits |
|---|---|---|---|
| `blockTimestamp` | `uint32` | 0 | 0–31 |
| `tickCumulative` | `int56` | 4 | 32–87 |
| `secondsPerLiquidityCumulativeX128` | `uint160` | 11 | 88–247 |
| `initialized` | `bool` | 31 | 248–255 |

---

<a name="11-events-reference"></a>
## 11. Events reference

### Pool — `interfaces/pool/IUniswapV3PoolEvents.sol`

| Event | Line | Signature | Emitted at |
|---|---|---|---|
| `Initialize` | `:11` | `(uint160 sqrtPriceX96, int24 tick)` | `UniswapV3Pool.sol:288` |
| `Mint` | `:21` | `(address sender, address indexed owner, int24 indexed tickLower, int24 indexed tickUpper, uint128 amount, uint256 amount0, uint256 amount1)` | `:486` |
| `Collect` | `:38` | `(address indexed owner, address recipient, int24 indexed tickLower, int24 indexed tickUpper, uint128 amount0, uint128 amount1)` | `:512` |
| `Burn` | `:55` | `(address indexed owner, int24 indexed tickLower, int24 indexed tickUpper, uint128 amount, uint256 amount0, uint256 amount1)` | `:542` |
| `Swap` | `:72` | `(address indexed sender, address indexed recipient, int256 amount0, int256 amount1, uint160 sqrtPriceX96, uint128 liquidity, int24 tick)` | `:786` |
| `Flash` | `:89` | `(address indexed sender, address indexed recipient, uint256 amount0, uint256 amount1, uint256 paid0, uint256 paid1)` | `:833` |
| `IncreaseObservationCardinalityNext` | `:103` | `(uint16 observationCardinalityNextOld, uint16 observationCardinalityNextNew)` | `:266` |
| `SetFeeProtocol` | `:113` | `(uint8 feeProtocol0Old, uint8 feeProtocol1Old, uint8 feeProtocol0New, uint8 feeProtocol1New)` | `:844` |
| `CollectProtocol` | `:120` | `(address indexed sender, address indexed recipient, uint128 amount0, uint128 amount1)` | `:867` |

Indexing notes: `Mint` indexes `owner` and both ticks but leaves `sender`
unindexed — you can filter by position but must decode data to learn who paid.
`Swap` indexes `sender` and `recipient` but not the tick, so a price-range filter
requires decoding. `Burn` has no `sender` field at all, because the owner is
always the sender.

### Factory — `interfaces/IUniswapV3Factory.sol`

| Event | Line | Signature | Emitted at |
|---|---|---|---|
| `OwnerChanged` | `:10` | `(address indexed oldOwner, address indexed newOwner)` | `UniswapV3Factory.sol:24`, `:56` |
| `PoolCreated` | `:18` | `(address indexed token0, address indexed token1, uint24 indexed fee, int24 tickSpacing, address pool)` | `:50` |
| `FeeAmountEnabled` | `:29` | `(uint24 indexed fee, int24 indexed tickSpacing)` | `:27`, `:29`, `:31`, `:71` |

`PoolCreated` uses all three indexed slots on `(token0, token1, fee)` — the exact
key of `getPool` — so indexers can filter for a specific pool without decoding.

---

<a name="12-revert-string-decoder"></a>
## 12. Revert-string decoder

V3 uses short strings to save bytecode. The complete table:

| String | Meaning | Raised at | Cause |
|---|---|---|---|
| `LOK` | **Lok**ed | `UniswapV3Pool.sol:105`, `:607` | Reentrancy, **or** the pool was never initialized |
| `AI` | **A**lready **I**nitialized | `:272` | `initialize` called twice |
| `TLU` | **T**ick **L**ower > **U**pper | `:127` | `tickLower >= tickUpper` |
| `TLM` | **T**ick **L**ower **M**in | `:128` | `tickLower < MIN_TICK` |
| `TUM` | **T**ick **U**pper **M**ax | `:129` | `tickUpper > MAX_TICK` |
| `M0` | **M**int **0** | `:483` | Mint callback underpaid token0 |
| `M1` | **M**int **1** | `:484` | Mint callback underpaid token1 |
| `AS` | **A**mount **S**pecified | `:603` | `swap` called with `amountSpecified == 0` |
| `SPL` | **S**qrt **P**rice **L**imit | `:612` | Limit on the wrong side of the current price, or beyond MIN/MAX |
| `IIA` | **I**nsufficient **I**nput **A**mount | `:777`, `:783` | Swap callback underpaid |
| `L` | **L**iquidity | `:798` | `flash` on a pool with zero liquidity |
| `F0` | **F**lash **0** | `:813` | Flash callback did not repay token0 + fee |
| `F1` | **F**lash **1** | `:814` | Flash callback did not repay token1 + fee |
| `LO` | **L**iquidity **O**verflow | `Tick.sol:128` | Tick would exceed `maxLiquidityPerTick` |
| `LS` | **L**iquidity **S**ub | `LiquidityMath.sol:12` | Removing more liquidity than exists |
| `LA` | **L**iquidity **A**dd | `LiquidityMath.sol:14` | Liquidity addition overflows `uint128` |
| `NP` | **N**o **P**osition | `Position.sol:54` | Poked a zero-liquidity position |
| `T` | **T**ick | `TickMath.sol:25` | `\|tick\| > MAX_TICK` |
| `R` | **R**atio | `TickMath.sol:63` | `sqrtPriceX96` outside `[MIN_SQRT_RATIO, MAX_SQRT_RATIO)` |
| `OLD` | **OLD** | `Oracle.sol:226` | TWAP window predates the oldest observation |
| `I` | **I**nitialized | `Oracle.sol:113`, `:309` | Oracle used before initialization |
| `TF` | **T**ransfer **F**ailed | `TransferHelper.sol:21` | ERC-20 `transfer` returned false or reverted |

**Reverts with no message** (bare `require`, saving the string entirely):
`UniswapV3Factory.sol:40`, `:42`, `:44`, `:45` (createPool validation), `:55`,
`:62`, `:63`, `:67`, `:68` (owner actions); `UniswapV3Pool.sol:113`
(`onlyFactoryOwner`), `:143`, `:153` (balance calls), `:188`, `:197`
(uninitialized ticks in `snapshotCumulativesInside`), `:464` (`amount > 0`),
`:838` (fee protocol range); `NoDelegateCall.sol:19`; `SafeCast.sol:11`, `:18`,
`:25`; `LowGasSafeMath.sol` throughout; `FullMath.sol:34`, `:43`, `:120`;
`SqrtPriceMath.sol:52`, `:93`, `:112-113`, `:135-136`, `:164`;
`TickBitmap.sol:28`; `BitMath.sol:14`, `:54`.

The two most-seen in production are `LOK` (usually "this pool was never
initialized", not reentrancy) and `IIA` (a router's callback failed to pay).

---

<a name="13-use-cases-and-call-chains"></a>
## 13. Use cases and call chains

### The tick-crossing state machine

What happens to a tick's stored data as the price moves across it:

```
                      feeGrowthOutside means "growth BELOW this tick"
                    ┌──────────────────────────────────────────────┐
                    │                                              │
                    ▼                                              │
       ┌────────────────────────┐                                  │
       │  price is ABOVE tick   │                                  │
       │  (tickCurrent >= t)    │                                  │
       │                        │                                  │
       │ getFeeGrowthInside     │                                  │
       │   uses outside AS-IS   │                                  │
       │   for the "below" part │                                  │
       └───────────┬────────────┘                                  │
                   │                                               │
   swap moves price DOWN across t          swap moves price UP across t
   Tick.cross:                             Tick.cross:
     outside := global - outside             outside := global - outside
     (Tick.sol:178-182)                      (Tick.sol:178-182)
   Pool negates liquidityNet               Pool applies liquidityNet as-is
     (UniswapV3Pool.sol:720)                 (UniswapV3Pool.sol:722)
   tick := tickNext - 1                    tick := tickNext
     (UniswapV3Pool.sol:725)                 (UniswapV3Pool.sol:725)
                   │                                               │
                   ▼                                               │
       ┌────────────────────────┐                                  │
       │  price is BELOW tick   │                                  │
       │  (tickCurrent < t)     │                                  │
       │                        │──────────────────────────────────┘
       │ getFeeGrowthInside     │
       │   uses global-outside  │
       │   for the "below" part │
       └────────────────────────┘
                    ▲
                    │
       feeGrowthOutside now means "growth ABOVE this tick"

   Lifecycle:  uninitialized ──Tick.update, liquidityGross 0→+──► initialized
                              (seeds outside if t <= tickCurrent, Tick.sol:132-142)
                              (flipTick sets the bitmap bit,  UniswapV3Pool.sol:431)

               initialized ──Tick.update, liquidityGross +→0──► uninitialized
                              (flipTick clears the bit,  UniswapV3Pool.sol:434)
                              (Tick.clear deletes 4 slots, UniswapV3Pool.sol:447)
```

The invariant that makes this work: `x ← global − x` is self-inverse with respect
to a *fixed* `global`, and since `global` only grows, applying it on each crossing
keeps `feeGrowthOutside` measuring the correct side while the *difference*
`getFeeGrowthInside` computes stays exact.

### Use case: create a pool

```
Caller
 └─ UniswapV3Factory.createPool(tokenA, tokenB, 3000)          Factory:35
     ├─ sort → (token0, token1)                                Factory:41
     ├─ tickSpacing = feeAmountTickSpacing[3000] = 60           Factory:43
     ├─ UniswapV3PoolDeployer.deploy(...)                       Deployer:27
     │   ├─ parameters = {...}                    (3 SSTOREs)   Deployer:34
     │   ├─ new UniswapV3Pool{salt: keccak(t0,t1,fee)}()        Deployer:35
     │   │   └─ UniswapV3Pool.constructor()                     Pool:117
     │   │       ├─ IUniswapV3PoolDeployer(msg.sender).parameters()   Pool:119
     │   │       └─ Tick.tickSpacingToMaxLiquidityPerTick(60)   Tick:44
     │   └─ delete parameters                     (refund)      Deployer:36
     ├─ getPool[t0][t1][fee] = pool                             Factory:47
     ├─ getPool[t1][t0][fee] = pool                             Factory:49
     └─ emit PoolCreated                                        Factory:50

Then, separately (anyone):
 └─ UniswapV3Pool.initialize(sqrtPriceX96)                      Pool:271
     ├─ TickMath.getTickAtSqrtRatio                             TickMath:61
     ├─ Oracle.initialize → (1, 1)                              Oracle:52
     └─ emit Initialize                                         Pool:288
```

### Use case: provide liquidity in a range

```
Integrator contract (must implement IUniswapV3MintCallback)
 └─ pool.mint(recipient, tickLower, tickUpper, L, data)         Pool:457
     ├─ require(L > 0)                                          Pool:464
     ├─ _modifyPosition({recipient, ticks, +L})                 Pool:306
     │   ├─ checkTicks               → TLU / TLM / TUM          Pool:126
     │   ├─ _updatePosition                                     Pool:379
     │   │   ├─ Position.get(owner, lower, upper)               Position:30
     │   │   ├─ Oracle.observeSingle(secondsAgo=0)              Oracle:245
     │   │   ├─ Tick.update(lower, …, upper=false) → LO         Tick:110
     │   │   ├─ Tick.update(upper, …, upper=true)  → LO         Tick:110
     │   │   ├─ TickBitmap.flipTick ×(0-2)  → spacing check     TickBitmap:23
     │   │   ├─ Tick.getFeeGrowthInside                         Tick:60
     │   │   └─ Position.update              → NP               Position:44
     │   └─ case A / B / C:                                     Pool:328/336/362
     │       ├─ [B only] Oracle.write                           Oracle:78
     │       ├─ SqrtPriceMath.getAmount0Delta(…, int128) ROUND UP    SqrtPriceMath:201
     │       ├─ SqrtPriceMath.getAmount1Delta(…, int128) ROUND UP    SqrtPriceMath:217
     │       └─ [B only] liquidity = LiquidityMath.addDelta     Pool:361
     ├─ balance0() / balance1()                                 Pool:480-481
     ├─ ►► uniswapV3MintCallback(amount0, amount1, data)        Pool:482
     ├─ require(before + amount <= after)  → M0 / M1            Pool:483-484
     └─ emit Mint                                               Pool:486
```

### Use case: swap with a price limit

```
Router (must implement IUniswapV3SwapCallback)
 └─ pool.swap(recipient, zeroForOne, amountSpecified, limit, data)   Pool:596
     ├─ require(amountSpecified != 0)          → AS              Pool:603
     ├─ require(unlocked)                      → LOK             Pool:607
     ├─ require(limit on correct side)         → SPL             Pool:608
     ├─ slot0.unlocked = false                                   Pool:615
     │
     ├─ LOOP while remaining != 0 && price != limit              Pool:641
     │   ├─ TickBitmap.nextInitializedTickWithinOneWord          TickBitmap:42
     │   │   └─ BitMath.mostSignificantBit / leastSignificantBit BitMath:13/53
     │   ├─ TickMath.getSqrtRatioAtTick(tickNext)                TickMath:23
     │   ├─ SwapMath.computeSwapStep                             SwapMath:21
     │   │   ├─ SqrtPriceMath.getAmount{0,1}Delta                SqrtPriceMath:153/182
     │   │   ├─ SqrtPriceMath.getNextSqrtPriceFrom{Input,Output} SqrtPriceMath:106/129
     │   │   └─ FullMath.mulDiv / mulDivRoundingUp               FullMath:14/113
     │   ├─ protocol cut = feeAmount / feeProtocol               Pool:683
     │   ├─ feeGrowthGlobalX128 += mulDiv(fee, Q128, liquidity)  Pool:690
     │   └─ if landed on tickNext && initialized:
     │       ├─ Oracle.observeSingle  (once, lazily)             Pool:699
     │       ├─ Tick.cross → liquidityNet                        Tick:168
     │       └─ liquidity = LiquidityMath.addDelta(±net)         Pool:722
     │
     ├─ if tick changed: Oracle.write + store slot0              Pool:733-748
     ├─ else: store sqrtPriceX96 only                            Pool:751
     ├─ store liquidity / feeGrowthGlobal / protocolFees         Pool:755-765
     ├─ compute (amount0, amount1)                               Pool:767
     ├─ TransferHelper.safeTransfer(output) → TF                 Pool:773/779
     ├─ ►► uniswapV3SwapCallback(amount0, amount1, data)         Pool:776/782
     ├─ require(balanceBefore + in <= balanceAfter) → IIA        Pool:777/783
     ├─ emit Swap                                                Pool:786
     └─ slot0.unlocked = true                                    Pool:787
```

### Use case: collect fees without withdrawing

```
Position owner
 ├─ pool.burn(tickLower, tickUpper, 0)      ← the "poke"        Pool:517
 │   └─ _modifyPosition(liquidityDelta = 0)                     Pool:306
 │       └─ _updatePosition → Position.update                   Position:44
 │           └─ tokensOwed += (insideNow - insideLast) * L / Q128
 │              (liquidityDelta == 0 requires L > 0 → NP)       Position:54
 └─ pool.collect(recipient, lower, upper, MAX, MAX)             Pool:490
     ├─ clamp to tokensOwed                                     Pool:500-501
     ├─ tokensOwed -= amount                                    Pool:504/508
     └─ TransferHelper.safeTransfer                             Pool:505/509
```

### Use case: take a flash loan

```
Borrower (must implement IUniswapV3FlashCallback)
 └─ pool.flash(recipient, amount0, amount1, data)               Pool:791
     ├─ require(liquidity > 0)                  → L             Pool:798
     ├─ fee0 = mulDivRoundingUp(amount0, fee, 1e6)              Pool:800
     ├─ snapshot balances                                       Pool:802-803
     ├─ TransferHelper.safeTransfer out                         Pool:805-806
     ├─ ►► uniswapV3FlashCallback(fee0, fee1, data)             Pool:808
     ├─ require(before + fee <= after)          → F0 / F1       Pool:813-814
     ├─ paid = after - before   (overpayment goes to LPs)       Pool:817-818
     ├─ protocolFees += paid / feeProtocol                      Pool:822-823
     └─ feeGrowthGlobal += mulDiv(paid - protocolCut, Q128, L)  Pool:824
```

### Use case: read a TWAP

```
Consumer
 ├─ [once, in advance] pool.increaseObservationCardinalityNext(N)  Pool:255
 │   └─ Oracle.grow — pre-pays the SSTOREs                          Oracle:108
 │
 └─ pool.observe([3600, 0])                                         Pool:236
     └─ Oracle.observe                                              Oracle:300
         └─ Oracle.observeSingle ×2                                 Oracle:245
             ├─ secondsAgo == 0 → newest, transformed to now        Oracle:254
             └─ else getSurroundingObservations → binarySearch      Oracle:198/153
                 └─ require(target >= oldest)         → OLD         Oracle:226

  arithmeticMeanTick = (tickCumulatives[1] - tickCumulatives[0]) / 3600
  price              = 1.0001 ^ arithmeticMeanTick     (a GEOMETRIC mean price)
```

### Use case: turn on the protocol fee

```
Factory owner
 └─ pool.setFeeProtocol(feeProtocol0, feeProtocol1)              Pool:837
     ├─ onlyFactoryOwner → IUniswapV3Factory(factory).owner()    Pool:113
     ├─ require each is 0 or in [4, 10]                          Pool:838
     └─ slot0.feeProtocol = f0 + (f1 << 4)                       Pool:843

 └─ pool.collectProtocol(recipient, MAX, MAX)                    Pool:848
     ├─ clamp to protocolFees                                    Pool:853-854
     ├─ leave 1 wei to keep the slot warm                        Pool:857/862
     └─ TransferHelper.safeTransfer                              Pool:859/864
```

### Use case: derive a pool address off-chain

```
salt         = keccak256(abi.encode(token0, token1, fee))
initCodeHash = 0xe34f199b19b2b4f47f68442619d555527d244f78a3297ea89325f843f87b8b54
pool         = address(uint160(uint256(keccak256(
                   abi.encodePacked(hex'ff', factory, salt, initCodeHash)))))
```

`token0 < token1` must hold. The hash is fixed for all time because
`UniswapV3Pool` takes no constructor arguments — see [§3](#3-uniswapv3pooldeployer).

---

## Appendix: reproducing the verification

```bash
cd uni/v3-core

# selectors
cast sig "swap(address,bool,int256,uint160,bytes)"      # 0x128acb08

# storage layouts and the init code hash (foundry.toml must set
# solc 0.7.6, optimizer on, 800 runs, bytecode_hash = "none")
forge inspect UniswapV3Pool storageLayout
forge inspect UniswapV3Pool bytecode | cast keccak
#   → 0xe34f199b19b2b4f47f68442619d555527d244f78a3297ea89325f843f87b8b54

# every revert string in the codebase (22 of them).
# NOTE: grepping for `require(` misses 'SPL', whose string sits on its own
# line inside a multi-line require (UniswapV3Pool.sol:612). Match the literals:
grep -rhoE "'[A-Z][A-Z0-9]{0,3}'" contracts --include=*.sol \
  | grep -v '/test/' | sort -u
```

All 22 are catalogued in [§12](#12-revert-string-decoder); the table above is
complete against this command.
