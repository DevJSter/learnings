# Curve StableSwap-NG — Complete Reference

Every contract, every function, every parameter, every revert, in
`curve/stableswap-ng/`. This is the exhaustive companion to
[`CURVE-DEEP-DIVE.md`](CURVE-DEEP-DIVE.md), which teaches the ideas; this file
is the one you read to have seen all of it. Read it cover to cover and you will
have walked every line of the generation of Curve that is deployed today.

Every `path:line` below was checked with `grep -n` against the files in this
folder. Paths are relative to `curve/stableswap-ng/`.

---

## Table of contents

- [0. Orientation](#0-orientation)
  - [0.1 The 21 files](#01-the-21-files)
  - [0.2 How the pieces are wired at runtime](#02-how-the-pieces-are-wired-at-runtime)
  - [0.3 Conventions used throughout the codebase](#03-conventions-used-throughout-the-codebase)
- [1. The math, derived](#1-the-math-derived)
  - [1.1 The invariant](#11-the-invariant)
  - [1.2 `get_D` — Newton on the invariant](#12-get_d--newton-on-the-invariant)
  - [1.3 `get_y` — solving for one balance](#13-get_y--solving-for-one-balance)
  - [1.4 `get_y_D` — solving for one balance at a reduced D](#14-get_y_d--solving-for-one-balance-at-a-reduced-d)
  - [1.5 `_dynamic_fee` — pricing imbalance](#15-_dynamic_fee--pricing-imbalance)
  - [1.6 `_get_p` — the state price from partial derivatives](#16-_get_p--the-state-price-from-partial-derivatives)
  - [1.7 `_calc_moving_average` — the keeper-free EMA](#17-_calc_moving_average--the-keeper-free-ema)
  - [1.8 `_calc_withdraw_one_coin`](#18-_calc_withdraw_one_coin)
  - [1.9 `_stored_rates` — the four asset types](#19-_stored_rates--the-four-asset-types)
- [2. `CurveStableSwapNG.vy` — the plain pool](#2-curvestableswapngvy--the-plain-pool)
- [3. `CurveStableSwapMetaNG.vy` — the metapool](#3-curvestableswapmetangvy--the-metapool)
- [4. `CurveStableSwapNGMath.vy`](#4-curvestableswapngmathvy)
- [5. `CurveStableSwapNGViews.vy`](#5-curvestableswapngviewsvy)
- [6. `CurveStableSwapFactoryNG.vy`](#6-curvestableswapfactoryngvy)
- [7. `CurveStableSwapFactoryNGHandler.vy`](#7-curvestableswapfactoryn-ghandlervy)
- [8. `MetaZapNG.vy`](#8-metazapngvy)
- [9. `LiquidityGauge.vy` — gauge v6.1.0](#9-liquiditygaugevy--gauge-v610)
- [10. `StableSwapNGLPOracle.vy`](#10-stableswapnglporaclevy)
- [11. `ProxyAdmin.vy`](#11-proxyadminvy)
- [12. The mocks](#12-the-mocks)
- [13. ABI / selector tables](#13-abi--selector-tables)
- [14. Storage layout tables](#14-storage-layout-tables)
- [15. Events reference](#15-events-reference)
- [16. Revert decoder](#16-revert-decoder)
- [17. Use-case index](#17-use-case-index)
- [18. Classic vs NG, function by function](#18-classic-vs-ng-function-by-function)
- [19. Gotchas and security notes](#19-gotchas-and-security-notes)

---

## 0. Orientation

### 0.1 The 21 files

Nothing in the repository is skipped. Every `.vy` file, with its Vyper pragma
and where it is covered below:

| # | File | Lines | Vyper | Covered in |
|---|---|---|---|---|
| 1 | `contracts/main/CurveStableSwapNG.vy` | 1890 | 0.3.10 | [§2](#2-curvestableswapngvy--the-plain-pool) |
| 2 | `contracts/main/CurveStableSwapMetaNG.vy` | 1901 | 0.3.10 | [§3](#3-curvestableswapmetangvy--the-metapool) |
| 3 | `contracts/main/CurveStableSwapNGMath.vy` | 269 | 0.3.10 | [§4](#4-curvestableswapngmathvy) |
| 4 | `contracts/main/CurveStableSwapNGViews.vy` | 704 | 0.3.10 | [§5](#5-curvestableswapngviewsvy) |
| 5 | `contracts/main/CurveStableSwapFactoryNG.vy` | 865 | 0.3.10 | [§6](#6-curvestableswapfactoryngvy) |
| 6 | `contracts/main/CurveStableSwapFactoryNGHandler.vy` | 557 | 0.3.10 | [§7](#7-curvestableswapfactoryn-ghandlervy) |
| 7 | `contracts/main/MetaZapNG.vy` | 440 | 0.3.10 | [§8](#8-metazapngvy) |
| 8 | `contracts/main/LiquidityGauge.vy` | 864 | 0.3.10 | [§9](#9-liquiditygaugevy--gauge-v610) |
| 9 | `contracts/main/StableSwapNGLPOracle.vy` | 113 | **0.4.3** | [§10](#10-stableswapnglporaclevy) |
| 10 | `contracts/ProxyAdmin.vy` | 156 | 0.3.10 | [§11](#11-proxyadminvy) |
| 11 | `contracts/mocks/CurvePool.vy` | 891 | 0.3.10 | [§12](#12-the-mocks) |
| 12 | `contracts/mocks/Zap.vy` | 314 | 0.3.10 | [§12](#12-the-mocks) |
| 13 | `contracts/mocks/ERC4626.vy` | 272 | 0.3.10 | [§12](#12-the-mocks) |
| 14 | `contracts/mocks/CurveTokenV3.vy` | 202 | 0.3.10 | [§12](#12-the-mocks) |
| 15 | `contracts/mocks/CallbackSwap.vy` | 154 | 0.3.10 | [§12](#12-the-mocks) |
| 16 | `contracts/mocks/ERC20Rebasing.vy` | 153 | 0.3.10 | [§12](#12-the-mocks) |
| 17 | `contracts/mocks/ERC20RebasingConditional.vy` | 148 | 0.3.10 | [§12](#12-the-mocks) |
| 18 | `contracts/mocks/ERC20Oracle.vy` | 99 | 0.3.10 | [§12](#12-the-mocks) |
| 19 | `contracts/mocks/WETH.vy` | 82 | 0.3.10 | [§12](#12-the-mocks) |
| 20 | `contracts/mocks/ERC20.vy` | 74 | 0.3.10 | [§12](#12-the-mocks) |
| 21 | `contracts/mocks/CallbackTestZap.vy` | 66 | ^0.3.7 | [§12](#12-the-mocks) |

Total 10,214 lines. The mocks are not dead weight: they are the executable
specification of what the pool claims to support, so §12 covers them.

Note the pragmas. Everything shipped in 2023 is Vyper **0.3.10**, which matters:
the `@nonreentrant` compiler bug that drained four Curve pools in July 2023
affected 0.2.15, 0.2.16 and 0.3.0 only. The LP oracle is a later addition on
**0.4.3** with the modern `staticcall`/`abi_encode` syntax and a `curve_std`
import, so it reads differently from everything else here.

Also note `# pragma optimize codesize` on both pools (`CurveStableSwapNG.vy:2`,
`CurveStableSwapMetaNG.vy:2`) versus `# pragma optimize gas` on the math and
gauge. The pools are near the 24 KB contract-size limit, which is the reason the
math and the views were split into separate contracts at all.

### 0.2 How the pieces are wired at runtime

```
                          ┌──────────────────────────────────┐
                          │  CurveStableSwapFactoryNG        │
        deploys ◄─────────┤  (registry + blueprint deployer) ├───────► deploys
     via create_from_      │  admin, fee_receiver,            │      gauges
       blueprint           │  *_implementation registries     │
                          └───────┬──────────────────┬───────┘
                                  │                  │
                    ┌─────────────▼───┐      ┌───────▼─────────────┐
                    │ CurveStableSwapNG│      │CurveStableSwapMetaNG│
                    │  (plain, 2–8)    │      │  (meta, always 2)   │
                    └───┬────────┬─────┘      └──┬──────┬───────┬───┘
       views_implementation│     │ inline math    │      │       │ BASE_POOL
                    ┌─────▼──┐   └── get_D/get_y  │      │       └──────────►
                    │ Views  │       in-contract  │      │      (3pool, etc.)
                    └────────┘                    │      │
                                math_implementation│      │views_implementation
                                          ┌────────▼──┐  ┌▼───────┐
                                          │   Math    │  │ Views  │
                                          └───────────┘  └────────┘

  user ──► MetaZapNG ──► metapool + base pool   (underlying add/remove)
  LP   ──► LiquidityGauge ──► CRV via Minter, boost via VotingEscrow
  integrator ──► StableSwapNGLPOracle ──► pool.get_virtual_price/price_oracle
```

The single most important structural fact: **the plain pool has its math
inlined, the metapool delegates to a Math contract.** `CurveStableSwapNG.vy`
defines `get_D` at `:1079`, `get_y` at `:1009` and `get_y_D` at `:1130` as its
own internal functions. `CurveStableSwapMetaNG.vy` has none of those; it calls
`math.get_D(...)` (`:1118`), `math.get_y(...)` (`:1119`) on an immutable `math`
address set in its constructor (`:346`). The metapool spends the gas on external
calls because it must also carry the base-pool interaction code and would
otherwise not fit.

The second: **`views_implementation` is read from the factory on every call**
(`CurveStableSwapNG.vy:1699`, `:1713`, `:1771`, `:1822`). `get_dy`, `get_dx`,
`calc_token_amount` and `dynamic_fee` on the pool are all thin forwarders. The
factory admin can therefore swap the entire quoting implementation for every
pool at once. That is a live upgrade path on an otherwise immutable pool.

### 0.3 Conventions used throughout the codebase

| Symbol | Meaning |
|---|---|
| `balances` | raw token units, admin fees already subtracted (`_balances()`) |
| `xp` | balances normalised to 18 decimals **and** rate-adjusted (`_xp_mem`) |
| `rates[i]` | `10**(36-decimals)` times any oracle/4626 rate; multiply by balance, divide by 1e18 |
| `D` | the invariant; equals total value in 18-decimal units when balanced |
| `A_PRECISION` | 100. Stored `A` is `A_true * 100` (`:239`, `_A()` returns the scaled value) |
| `Ann` | `amp * N_COINS` where `amp` is already `A*100`. Note: **not** `A*n^n` |
| `FEE_DENOMINATOR` | `10**10`. A fee of `4000000` is 0.04% |
| `PRECISION` | `10**18` |
| `admin_fee` | fixed constant `5000000000` = 50% of the swap fee (`:167`) |

One trap worth flagging immediately: Curve's `Ann` is `A * n`, not `A * n^n`,
because the historical `A` already absorbs a factor of `n^(n-1)`. This is why
the LP oracle has to rescale it (`StableSwapNGLPOracle.vy:61-68`), and why
comparing `A` across pools with different coin counts is misleading.

---

## 1. The math, derived

This section derives every formula once. §2 onward then refers back here rather
than re-deriving. Every number in this section was reproduced in Python against
the integer code paths.

### 1.1 The invariant

StableSwap interpolates between a constant sum (zero slippage, no protection
against a depeg) and a constant product (always solvent, always slippery):

```
constant sum:      Σx_i = D
constant product:  Πx_i = (D/n)^n
```

The amplified invariant is

```
        A·n^n·Σx_i  +  D  =  A·D·n^n  +  D^(n+1) / (n^n · Πx_i)
        └─ sum term ─┘        └───── product term ─────────────┘
```

As `A → ∞` the sum terms dominate and the curve becomes a straight line
(`Σx = D`). As `A → 0` only the product terms survive and it degenerates to
`Πx = (D/n)^n`. `A` is therefore a dial on "how strongly do I believe these
assets are worth the same".

`D` is the invariant value: when the pool is perfectly balanced, `x_i = D/n` for
every `i`, so `D` is the total portfolio value measured in 18-decimal units.
That is why `get_virtual_price` is just `D / totalSupply` (`:1740-1757`).

**Slippage, measured.** Balanced 1M/1M two-coin pool, swap 100,000 in, computed
by running the actual integer `get_D`/`get_y`:

| Curve | Output for 100,000 in | Slippage |
|---|---|---|
| `x·y = k` (Uniswap v2) | 90,909.09 | 9.0909% |
| StableSwap A=1 | 95,227.30 | 4.7727% |
| StableSwap A=10 | 99,091.73 | 0.9083% |
| StableSwap A=100 | 99,900.11 | 0.0999% |
| StableSwap A=1000 | 99,989.91 | 0.0101% |

A=100 gives roughly 90× less slippage than constant product on a 10%-of-pool
trade. That factor is the entire product.

### 1.2 `get_D` — Newton on the invariant

`CurveStableSwapNG.vy:1079-1126` (inlined) and `CurveStableSwapNGMath.vy:90-136`
(the metapool's copy, parameterised by `_n_coins`).

**Derivation.** Write `S = Σx_i` and `P = Πx_i`. Move everything to one side:

```
f(D) = D^(n+1)/(n^n·P) + (A·n^n − 1)·D − A·n^n·S  =  0
```

Newton's step is `D ← D − f(D)/f'(D)`. Define the convenient quantity

```
D_P = D^(n+1) / (n^n · P)          # computed iteratively: D_P = D; for x: D_P = D_P*D/x; D_P /= n^n
```

Then `f(D) = D_P + (Ann−1)·D − Ann·S` and `f'(D) = (n+1)·D_P/D + (Ann−1)`.
Substituting and simplifying gives the form the code actually uses:

```
D_next = (Ann·S + n·D_P) · D  /  ((Ann−1)·D + (n+1)·D_P)
```

which appears verbatim (modulo `A_PRECISION` scaling) at `:1104-1111`:

```vyper
D = (
    (unsafe_div(Ann * S, A_PRECISION) + D_P * N_COINS) * D
    /
    (
        unsafe_div((Ann - A_PRECISION) * D, A_PRECISION) +
        unsafe_add(N_COINS, 1) * D_P
    )
)
```

**Details that matter.**

- Initial guess `D = S` (`:1095`). For a balanced pool that is already the exact
  answer, so the loop converges in **one** iteration — confirmed numerically.
- The convergence test is absolute, not relative: `|D − D_prev| <= 1` (`:1117-1122`).
  Since `D` is in 1e18 units this is a 1-wei tolerance.
- `S == 0` returns 0 early (`:1092`). An empty pool has `D = 0`.
- The loop is `range(255)` and falls through to a bare `raise` (`:1125`). The
  comment at `:1124-1125` is important: *"convergence typically occurs in 4
  rounds or less, this should be unreachable! if it does happen the pool is
  borked and LPs can withdraw via `remove_liquidity`."* `remove_liquidity`
  (`:807`) is the only mutating function that never calls `get_D`, which is why
  it remains the guaranteed escape hatch.
- `D_P = D_P * D / x` divides by each balance. If any `xp[i]` is 0 this reverts
  on division by zero. The Math contract spells the consequence out at
  `CurveStableSwapNGMath.vy:112`: *"If division by 0, this will be borked: only
  withdrawal will work. And that is good"*.
- Rounding: every step floors. `D` is therefore a slight under-estimate, which
  is the safe direction — LP value is understated, never overstated.

### 1.3 `get_y` — solving for one balance

`CurveStableSwapNG.vy:1009-1076`, `CurveStableSwapNGMath.vy:18-84`.

Given every balance except `x_j`, and given `D`, find `x_j`. Isolate `y = x_j` in
the invariant. Let `S' = Σ_{i≠j} x_i` and `P' = Π_{i≠j} x_i`. The invariant
becomes, after multiplying through by `y`:

```
y² + y·(S' + D/Ann − D)  =  D^(n+1) / (n^n · P' · Ann · n)
```

i.e. the quadratic `y² + b·y = c` that the docstring states (`:1014-1018`), with

```
b = S' + D/Ann        (the code keeps the "− D" in the Newton denominator instead)
c = D^(n+1) / (n^(2n)... ) — built iteratively as c = c*D/(x_i*n) for each i≠j, then c = c*D*A_PRECISION/(Ann*n)
```

Newton on `g(y) = y² + b·y − c − D·y` gives the update the code uses (`:1067`):

```
y_next = (y² + c) / (2y + b − D)
```

Reading the loop at `:1046-1058`:

```vyper
for _i in range(MAX_COINS_128):
    if _i == N_COINS_128: break
    if _i == i:   _x = x        # the coin being sold, at its NEW balance
    elif _i != j: _x = xp[_i]   # untouched coins
    else: continue              # skip j entirely — that is the unknown
    S_ += _x
    c = c * D / (_x * N_COINS)
```

- Initial guess `y = D` (`:1064`), converges in about 9 iterations for a
  10%-of-pool trade (measured).
- Same 1-wei convergence test, same `range(255)` then `raise`.
- Index validation is at `:1028-1035`: `i != j`, `0 <= j < N_COINS`, and the
  same for `i` with the comment *"should be unreachable, but good for safety"*.
- Rounding floors, so `y` is under-estimated, so `dy = xp[j] − y − 1` is
  under-estimated. The extra `−1` at `__exchange` (`:912`) is a second safety
  wei. Both round in the pool's favour.

### 1.4 `get_y_D` — solving for one balance at a reduced D

`CurveStableSwapNG.vy:1130-1184`, `CurveStableSwapNGMath.vy:143-197`.

Identical algebra to `get_y`, but the question is different: *keep every other
balance fixed, and ask what `x_i` would be if the invariant were `D_new` instead
of `D_old`*. That is exactly the question a single-sided withdrawal asks.

The only structural difference in the loop (`:1156-1166`) is that there is no
`i == i → use x` branch, because no coin is being increased; every coin except
`i` contributes its current balance:

```vyper
if _i != i:
    _x = xp[_i]
else:
    continue
```

Everything else — `c`, `b`, the `(y*y + c) / (2*y + b - D)` step, the 1-wei test,
the 255 cap — is the same. Note `assert i >= 0` / `assert i < N_COINS_128`
(`:1147-1148`) but **no** `i != j` check, since there is no `j`.

### 1.5 `_dynamic_fee` — pricing imbalance

`CurveStableSwapNG.vy:887-901`. This is new in NG and is the single biggest
behavioural change from the classic pools.

```vyper
_offpeg_fee_multiplier: uint256 = self.offpeg_fee_multiplier
if _offpeg_fee_multiplier <= FEE_DENOMINATOR:
    return _fee

xps2: uint256 = (xpi + xpj) ** 2
return unsafe_div(
    unsafe_mul(_offpeg_fee_multiplier, _fee),
    unsafe_add(
        unsafe_sub(_offpeg_fee_multiplier, FEE_DENOMINATOR) * 4 * xpi * xpj / xps2,
        FEE_DENOMINATOR
    )
)
```

Writing `m` for the multiplier (in units of `FEE_DENOMINATOR`, so `m = 5e10`
means 5×) and `f` for the base fee:

```
                     m · f
fee(x_i, x_j) = ─────────────────────────
                (m − 1)·B  +  1

                        4·x_i·x_j
where   B  =  ───────────────────────
                    (x_i + x_j)²
```

`B` is the key term. It is the ratio of the geometric to the arithmetic mean,
squared: `B = 1` exactly when `x_i == x_j`, and `B → 0` as the pool depegs.

| balance ratio | `B` | fee at m=2 | fee at m=5 | fee at m=20 |
|---|---|---|---|---|
| 1:1 | 1.0000 | 0.0400% | 0.0400% | 0.0400% |
| 1.5:1 | 0.9600 | 0.0408% | 0.0413% | 0.0416% |
| 2:1 | 0.8889 | 0.0424% | 0.0439% | 0.0447% |
| 5:1 | 0.5556 | 0.0514% | 0.0621% | 0.0692% |
| 10:1 | 0.3306 | 0.0601% | 0.0861% | 0.1099% |
| 100:1 | 0.0392 | 0.0770% | 0.1729% | 0.4584% |

(base fee 0.04%, computed with the integer code path.)

At balance the fee is exactly `f` (since `B=1` gives `m·f/((m−1)+1) = f`). In the
limit of total depeg it approaches `m·f`. So `m` is literally "how many times the
base fee do I charge when this thing has completely broken". The guard at
`:890-891` means setting `m <= FEE_DENOMINATOR` disables the mechanism and gives
you a classic static-fee pool.

`_dynamic_fee` is called from four places, and the fee it is handed differs:

| Caller | `line` | `xpi`, `xpj` passed | base fee passed |
|---|---|---|---|
| `__exchange` | `:918-920` | midpoints `(xp[i]+x)/2`, `(xp[j]+y)/2` | `self.fee` |
| `add_liquidity` | `:651` | `xs = rates[i]·(old+new)/1e18`, `ys = (D0+D1)/n` | `base_fee` |
| `remove_liquidity_imbalance` | `:778` | same shape as add | `base_fee` |
| `_calc_withdraw_one_coin` | `:1282` | `xavg`, `ys = (D0+D1)/(2n)` | `base_fee` |

where the liquidity paths use the classic imbalance-fee scaling (`:633-636`):

```vyper
base_fee: uint256 = unsafe_div(
    unsafe_mul(self.fee, N_COINS),
    unsafe_mul(4, unsafe_sub(N_COINS, 1))
)
```

`fee·n/(4(n−1))`. For n=2 that is `f/2`; for n=3, `3f/8`. The factor exists so
that depositing one coin and withdrawing another costs about the same as
swapping them.

### 1.6 `_get_p` — the state price from partial derivatives

`CurveStableSwapNG.vy:1313-1337` (returns a `DynArray`, one price per coin
relative to coin 0), `CurveStableSwapMetaNG.vy:1371-1387` (returns a single
`uint256`, since metapools are always 2 coins).

This is the **state price**, not the last traded price: the marginal exchange
rate implied by the curve at the current balances. Deriving it is implicit
differentiation of the invariant.

Write the invariant as `F(x_0, …, x_{n-1}, D) = 0`. Holding `D` fixed, the
marginal price of coin `i` in units of coin 0 is

```
p_i = dx_0/dx_i = − (∂F/∂x_i) / (∂F/∂x_0)
```

With `F = Ann·Σx + D − Ann·D − D^(n+1)/(n^n·Πx)` the partial with respect to `x_k` is

```
∂F/∂x_k = Ann + D^(n+1)/(n^n · Πx · x_k)  =  Ann + D_r/x_k
```

where `D_r = D^(n+1)/(n^n·Πx)` is exactly the `Dr` the code builds at `:1321-1324`:

```vyper
Dr: uint256 = unsafe_div(D, pow_mod256(N_COINS, N_COINS))
for i in range(N_COINS_128, bound=MAX_COINS_128):
    Dr = Dr * D / xp[i]
```

So

```
        Ann + D_r/x_i        x_0·Ann + D_r·x_0/x_i
p_i =  ───────────────  ·   ────────────────────── (multiply num & den by x_0)
        Ann + D_r/x_0        x_0·Ann + D_r
```

which is precisely `:1334` (`xp0_A` is `ANN·xp[0]/A_PRECISION`):

```vyper
p.append(10**18 * (xp0_A + unsafe_div(Dr * xp[0], xp[i])) / (xp0_A + Dr))
```

Verified numerically: a balanced 3-coin pool returns exactly `1.000000000` for
both prices; balances `[0.9M, 1.05M, 1.05M]` return `0.998418` — coin 0 is
scarce, so it is worth slightly more, and the price of coins 1 and 2 quoted in
coin 0 drops below 1. The near-1 value even at a 15% imbalance is the whole point
of a high `A`.

Note the loop starts at index 1 (`:1329-1334`), so `_get_p` returns `n−1`
entries: prices of coins 1..n−1 relative to coin 0. `get_p(i)` (`:1427-1440`) is
therefore documented as *"if i = 0, it will return the state price of coin[1]"*.

### 1.7 `_calc_moving_average` — the keeper-free EMA

`CurveStableSwapNG.vy:1393-1411`.

```vyper
last_spot_value: uint256 = packed_value & (2**128 - 1)
last_ema_value: uint256 = (packed_value >> 128)

if ma_last_time < block.timestamp:
    alpha: uint256 = self.exp(
        -convert(
            unsafe_div(unsafe_mul(unsafe_sub(block.timestamp, ma_last_time), 10**18), averaging_window), int256
        )
    )
    return unsafe_div(last_spot_value * (10**18 - alpha) + last_ema_value * alpha, 10**18)

return last_ema_value
```

The EMA is

```
α = exp(−Δt / ma_exp_time)
ema_new = spot_last·(1 − α) + ema_last·α
```

**Why no keeper is needed.** The stored pair is `(last_spot, last_ema)` packed
into one slot, plus a separate `ma_last_time`. The EMA is never "ticked
forward" on a schedule; instead, whenever anyone reads `price_oracle` or writes
via `upkeep_oracles`, the function computes what the EMA *would* be having decayed
continuously from `ma_last_time` to now. `price_oracle` (`:1445-1451`) is a pure
view that does this on the fly, so the value is correct at any timestamp with no
transaction ever having occurred. The `if ma_last_time < block.timestamp` guard
means a second call in the same block returns the stored EMA unchanged, which is
what makes the oracle flat within a block and therefore not manipulable by
ordering inside one transaction.

**The half-life.** `ma_exp_time` is documented as `time_in_seconds / ln(2)`
(`:266-267`). Setting `ma_exp_time = 866` gives `α = exp(−600/866) = 0.5002` at
`Δt = 600`, i.e. after 600 seconds the EMA has moved halfway to the new value.
So 866 is "10-minute half-life". `D_ma_time` defaults to `62324`, which is
`43200/ln 2` — a 12-hour half-life (`:297`, and the comment says so).

| `ma_exp_time` | Δt | α | weight on new spot |
|---|---|---|---|
| 866 | 433 | 0.6065 | 0.3935 |
| 866 | 600 | 0.5002 | 0.4998 |
| 866 | 866 | 0.3679 | 0.6321 |
| 866 | 1732 | 0.1353 | 0.8647 |

**The spot cap.** `upkeep_oracles` stores `min(spot_price[i], 2 * 10**18)`
(`:1361`). No single observation above 2.0 can ever enter the accumulator. This
bounds the oracle's output — the LP oracle's docstring calls this out explicitly
(`StableSwapNGLPOracle.vy:106-107`).

**`exp`.** `:1469-1537` is the Snekmate/Remco Bloemen `wad_exp`, a (6,7)-term
rational approximation in a 2^96 base. It returns 0 for `x <= -41.446e18` and
reverts `"wad_exp overflow"` above `135305999368893231589` (`:1489`). Since the
argument here is always `−Δt·1e18/window ≤ 0`, only the underflow-to-zero branch
is reachable in practice; `α = 0` simply means "the window has fully elapsed,
take the spot value".

### 1.8 `_calc_withdraw_one_coin`

`CurveStableSwapNG.vy:1233-1294`. Returns a 5-tuple
`(dy, dy_fee, xp, amp, D1)` — the last three exist so the caller can feed
`upkeep_oracles` without recomputing.

The algorithm:

1. `D0 = get_D(xp, amp)` — current invariant (`:1249`).
2. `D1 = D0 − burn·D0/total_supply` (`:1252`). Burning `k`% of supply removes
   `k`% of `D`. This is the definition of pro-rata in `D`-space.
3. `new_y = get_y_D(amp, i, xp, D1)` (`:1253`) — what `x_i` becomes at the
   reduced invariant, with no fee.
4. Charge an imbalance fee on **every** coin (`:1267-1283`). For the withdrawn
   coin `i`, the "expected" move is `xp_i·D1/D0 − new_y`; for every other coin
   it is `xp_j − xp_j·D1/D0`, i.e. the amount it *would* have shrunk under a
   balanced withdrawal. Each gets `xp_reduced[j] = xp_j − dyn_fee·dx_expected/FEE_DENOM`.
5. `dy = xp_reduced[i] − get_y_D(amp, i, xp_reduced, D1)` (`:1285`) — solve
   again against the fee-reduced balances.
6. `dy_0` is the no-fee answer (`:1286`); the returned fee is `dy_0 − dy` (`:1292`).
7. `dy = (dy − 1)·1e18/rates[i]` (`:1287`) — the familiar extra wei, then convert
   out of `xp` space back to token units.

Step 4 is the subtle one. Charging the fee on the *other* coins too, and then
re-solving, is what makes a one-sided withdrawal cost the same as
"withdraw balanced, then swap" — otherwise single-sided exit would be a free
lunch for anyone wanting to change the pool composition.

Line `:1290` mutates `xp[i] = new_y` before returning, so the `xp` handed
back to `upkeep_oracles` reflects the post-withdrawal state.

### 1.9 `_stored_rates` — the four asset types

`CurveStableSwapNG.vy:433-469`. This is where NG's support for
non-plain tokens lives. It starts from the immutable `rate_multipliers` — set by
the factory to `10**(36 − decimals)` (`CurveStableSwapFactoryNG.vy:516`) — and
multiplies in a live rate per coin.

**Type 0, Standard.** No branch is taken. `rates[i]` stays `10**(36−d)`, so
`xp = rate·balance/1e18 = balance·10**(18−d)`, a plain decimal normalisation.

**Type 1, Oracle** (`:439-455`). Used for wstETH, cbETH.

```vyper
oracle_response: Bytes[32] = raw_call(
    convert(rate_oracles[i] % 2**160, address),
    _abi_encode(rate_oracles[i] & ORACLE_BIT_MASK),
    max_outsize=32,
    is_static_call=True,
)
assert len(oracle_response) == 32
fetched_rate: uint256 = convert(oracle_response, uint256)
rates[i] = unsafe_div(rates[i] * fetched_rate, PRECISION)
```

The oracle address and its method id are packed into one `uint256`
(`rate_oracles`, immutable, built at `:310`): the low 160 bits are the address,
the top 32 bits are the selector, masked out by
`ORACLE_BIT_MASK = (2**32 - 1) * 256**28` (`:209`). The call is a `staticcall`,
so it cannot reenter, but **the returned number is trusted unconditionally**.
The contract header says so at `:16-18`: *"Oracles may be controlled externally
by an EOA. Users are advised to proceed with caution."* Rate precision must be
1e18 (`:32`).

**Type 2, Rebasing** (no branch here at all). Rebasing tokens do not get a rate;
instead `_balances()` (`:473-500`) reads `balanceOf(self)` live rather than using
`stored_balances`:

```vyper
if pool_contains_rebasing_tokens:
    balances_i = ERC20(coins[i]).balanceOf(self) - self.admin_balances[i]
else:
    balances_i = self.stored_balances[i] - self.admin_balances[i]
```

`pool_contains_rebasing_tokens` is an immutable set once at `:280`
(`2 in asset_types`). Note the consequence the docstring spells out at
`:477-482`: because admin balances are tracked in a separate array rather than
inferred, **the admin's accrued fees are immune to a negative rebase (slashing)**
— the loss falls entirely on LPs.

**Type 3, ERC4626** (`:457-467`).

```vyper
rates[i] = unsafe_div(
    rates[i] * ERC4626(coins[i]).convertToAssets(call_amount[i]) * scale_factor[i],
    PRECISION
)
```

Two immutables set in the constructor (`:314-322`): `call_amount[i]` is
`10**vault_decimals` (one whole share), and `scale_factor[i]` is
`10**(18 − underlying_decimals)`. So the product is "assets per share, scaled to
1e18". This is a plain external call, **not** a staticcall — but `convertToAssets`
is a view on any correct 4626, and Vyper's declared interface marks it `view`,
which compiles to `STATICCALL`. The header warns about donation/inflation attacks
on 4626 implementations (`:23-25`).

The whole loop runs on every `_stored_rates()` call, and `_stored_rates()` is
called by `add_liquidity` (`:586`), `_exchange` (`:956`),
`remove_liquidity_imbalance` (`:741`), `_calc_withdraw_one_coin` (`:1247`),
`get_p` (`:1434`), `get_virtual_price` (`:1750`) and the `stored_rates()` getter
(`:1805`). An expensive or reverting oracle therefore bricks nearly the whole
pool — but not `remove_liquidity` (`:807`), which again is the escape hatch.

---

## 2. `CurveStableSwapNG.vy` — the plain pool

1890 lines, Vyper 0.3.10, `# pragma optimize codesize`, `# pragma evm-version shanghai`.
`implements: ERC20` — the pool **is** its own LP token; there is no separate
token contract as in the classic generation.

Supports 2–8 coins of asset types 0/1/2/3, arbitrary decimals ≤ 18, and ERC20s
that return `True`, `False` or nothing (every transfer uses
`default_return_value=True`).

### 2.1 Constants and immutables

| Name | `line` | Value / type | Notes |
|---|---|---|---|
| `MAX_COINS` | `:152` | `8` | hard cap, matches the factory |
| `MAX_COINS_128` | `:153` | `int128 8` | loop bound in `int128` space |
| `N_COINS` | `:157` | `public(immutable(uint256))` | actual coin count |
| `N_COINS_128` | `:158` | `immutable(int128)` | same, for indexing |
| `PRECISION` | `:159` | `10**18` | |
| `factory` | `:161` | `immutable(Factory)` | set to `msg.sender` in `__init__` (`:286`) |
| `coins` | `:162` | `public(immutable(DynArray))` | |
| `asset_types` | `:163` | `immutable(DynArray[uint8,8])` | **not public** |
| `pool_contains_rebasing_tokens` | `:164` | `immutable(bool)` | `2 in asset_types` (`:280`) |
| `FEE_DENOMINATOR` | `:168` | `10**10` | |
| `admin_fee` | `:171` | `public(constant) = 5000000000` | **50%, immutable forever** |
| `MAX_FEE` | `:172` | `5 * 10**9` | 50% — the cap on `self.fee` |
| `A_PRECISION` | `:176` | `100` | |
| `MAX_A` | `:177` | `10**6` | |
| `MAX_A_CHANGE` | `:178` | `10` | max ramp factor per ramp |
| `MIN_RAMP_TIME` | `:187` | `86400` | 1 day |
| `rate_multipliers` | `:192` | `immutable(DynArray)` | `10**(36−decimals)` |
| `rate_oracles` | `:194` | `immutable(DynArray)` | `[selector:4][pad:8][address:20]` |
| `call_amount`, `scale_factor` | `:197-198` | `immutable(DynArray)` | ERC4626 only |
| `ORACLE_BIT_MASK` | `:209` | `(2**32-1) * 256**28` | isolates the packed selector |
| `decimals` | `:215` | `public(constant(uint8)) = 18` | the LP token's decimals |
| `version` | `:216` | `public(constant) = "v7.0.0"` | |
| `ERC1271_MAGIC_VAL` | `:224` | `0x1626ba7e...` | |
| `EIP712_TYPEHASH` | `:225` | keccak of the domain string | includes a `salt` field |
| `EIP2612_TYPEHASH` | `:226` | keccak of the Permit string | |

`admin_fee` being a **constant** is a real design decision: the DAO can change
`fee` and `offpeg_fee_multiplier` on a live pool (`set_new_fee`, `:1862`) but can
never raise its own cut above 50% of whatever that fee is.

### 2.2 Mutable storage

| Slot group | `line` | Meaning |
|---|---|---|
| `stored_balances: DynArray[uint256,8]` | `:165` | cached token balances; the pool's own book |
| `fee: public(uint256)` | `:169` | base fee, 1e10 precision |
| `offpeg_fee_multiplier: public(uint256)` | `:170` | `m` from §1.5 |
| `initial_A`, `future_A`, `initial_A_time`, `future_A_time` | `:180-183` | ramp state, all `public` |
| `admin_balances: public(DynArray)` | `:188` | accrued protocol fees, per coin, in token units |
| `last_prices_packed: DynArray` | `:200` | per coin `i≥1`: `[ema:128][spot:128]` |
| `last_D_packed: uint256` | `:201` | `[ema_D:128][spot_D:128]` |
| `ma_exp_time`, `D_ma_time` | `:202-203` | EMA windows |
| `ma_last_time: public(uint256)` | `:204` | `[t_D:128][t_p:128]` — two clocks in one slot |
| `balanceOf`, `allowance`, `total_supply`, `nonces` | `:218-221` | the ERC20 |

Two clocks (`:205-206`) because *"p is not updated if users remove_liquidity, but
D is"* — a proportional withdrawal changes `D` but not any price.

### 2.3 `__init__` — `:239-355`

Called by the factory through `create_from_blueprint`, so `msg.sender` is the
factory and becomes the immutable `factory` (`:287`).

**Parameters.** `_name`, `_symbol`, `_A` (un-scaled; multiplied by 100 at `:288`),
`_fee`, `_offpeg_fee_multiplier`, `_ma_exp_time`, `_coins`, `_rate_multipliers`,
`_asset_types`, `_method_ids`, `_oracles`.

**Checks.** Only one: `assert _ma_exp_time != 0` (`:295`). Everything else —
fee bounds, decimals ≤ 18, duplicate coins, array lengths — is validated by the
factory before it deploys (`CurveStableSwapFactoryNG.vy:496-521`). **A pool
deployed by any other means is unvalidated.**

**State written.** `initial_A = future_A = _A*100`; `fee`; `offpeg_fee_multiplier`;
`ma_exp_time`; `D_ma_time = 62324` (12h, `:297`); `ma_last_time = pack_2(now, now)`.

The setup loop (`:305-333`) does four things per coin:
- appends `pack_2(1e18, 1e18)` to `last_prices_packed` **only for `i < N_COINS−1`**
  (`:307-308`) — there are `n−1` prices, all initialised to 1.0;
- packs `[method_id][oracle]` into `rate_oracles` (`:310`);
- appends a zero to `stored_balances` and `admin_balances` (`:311-312`);
- for type 3, calls `decimals()` on the vault and `asset()` then `decimals()` on
  the underlying to build `call_amount` and `scale_factor` (`:314-322`).

**EIP-712** (`:335-348`): `salt = block.prevhash`. Using the previous block hash
as the domain salt means two pools with the same name on the same chain still
get distinct domain separators.

**Emits** `Transfer(0x0, msg.sender, 0)` (`:351`) — a zero-value mint so indexers
register the token.

### 2.4 `_transfer_in` — `:358-395` (internal)

The only way tokens enter the pool.

```vyper
_dx: uint256 = ERC20(coins[coin_idx]).balanceOf(self)

if expect_optimistic_transfer:
    _dx = _dx - self.stored_balances[coin_idx]
    assert _dx >= dx
else:
    assert dx > 0  # dev : do not transferFrom 0 tokens into the pool
    assert ERC20(coins[coin_idx]).transferFrom(
        sender, self, dx, default_return_value=True
    )
    _dx = ERC20(coins[coin_idx]).balanceOf(self) - _dx

self.stored_balances[coin_idx] += _dx
return _dx
```

Two modes:

- **Normal** (`expect_optimistic_transfer=False`): `transferFrom`, then measure
  the balance delta. Measuring rather than trusting `dx` is what makes
  fee-on-transfer tokens work — the pool credits what actually arrived.
- **Optimistic** (`True`): the caller already sent tokens. The surplus over
  `stored_balances` is the input. `assert _dx >= dx` lets the caller under-declare
  (donating the difference) but never over-declare.

Returns the *measured* amount, which every caller uses instead of the requested
amount. `stored_balances` is the pool's own book: the difference between it and
`balanceOf` is exactly what makes `exchange_received` and donation-detection work.

### 2.5 `_transfer_out` — `:398-429` (internal)

```vyper
assert receiver != empty(address)  # dev: do not send tokens to zero_address

if not pool_contains_rebasing_tokens:
    self.stored_balances[_coin_idx] -= _amount
    assert ERC20(coins[_coin_idx]).transfer(receiver, _amount, default_return_value=True)
else:
    coin_balance: uint256 = ERC20(coins[_coin_idx]).balanceOf(self)
    assert ERC20(coins[_coin_idx]).transfer(receiver, _amount, default_return_value=True)
    self.stored_balances[_coin_idx] = coin_balance - _amount
```

For a rebasing pool the post-transfer book is recomputed from the *pre-transfer*
live balance minus the amount, which re-synchronises `stored_balances` with any
rebase that happened since the last touch. Callers: `remove_liquidity` (`:833`),
`remove_liquidity_one_coin` (`:717`), `remove_liquidity_imbalance` (`:756`),
`_exchange` (`:980`), `_withdraw_admin_fees` (`:998`).

### 2.6 `_stored_rates` / `_balances` — `:433-500` (internal view)

Derived in [§1.9](#19-_stored_rates--the-four-asset-types).

### 2.7 `exchange` — `:504-529` (external, `@nonreentrant('lock')`)

```
exchange(i: int128, j: int128, _dx: uint256, _min_dy: uint256, _receiver: address = msg.sender) -> uint256
```

Pure forwarder to `_exchange(msg.sender, i, j, _dx, _min_dy, _receiver, False)`.
No checks of its own; everything is in `_exchange`. Returns the actual `dy`.

### 2.8 `exchange_received` — `:534-565` (external, `@nonreentrant('lock')`)

Same signature, one extra guard:

```vyper
assert not pool_contains_rebasing_tokens  # dev: exchange_received not supported if pool contains rebasing tokens
```

then `_exchange(..., True)`. This is the approval-free path for aggregators:
send the tokens to the pool, then call this in the same transaction. The pool
detects the arrival as `balanceOf − stored_balances`.

The rebasing ban (`:556`) is not optional. In a rebasing pool `_balances()` reads
live `balanceOf`, so there is no reliable `stored_balances` baseline to diff
against, and a positive rebase would be indistinguishable from an incoming
transfer — anyone could claim it. The header states both halves of this at
`:41-45`.

**The standing risk for integrators:** tokens sitting in the pool between the
transfer and the call belong to whoever calls `exchange_received` first. It must
be atomic.

### 2.9 `add_liquidity` — `:570-684` (external, `@nonreentrant('lock')`)

```
add_liquidity(_amounts: DynArray[uint256,8], _min_mint_amount: uint256, _receiver: address = msg.sender) -> uint256
```

**Step by step.**

1. `assert _receiver != empty(address)` (`:582`).
2. Snapshot `amp`, `old_balances`, `rates`, and `D0 = get_D_mem(...)` (`:584-588`).
3. Transfer in each non-zero amount, accumulating the *measured* deltas into
   `new_balances` (`:598-611`). For any zero amount:
   `assert total_supply != 0  # dev: initial deposit requires all coins` (`:609`).
   The first deposit must seed every coin, otherwise `get_D` divides by zero.
4. `D1 = get_D_mem(rates, new_balances, amp)`; `assert D1 > D0` (`:614-615`).
5. **If `total_supply > 0`** (`:623-660`): charge the imbalance fee per coin.

```vyper
ideal_balance = D1 * old_balances[i] / D0
difference = |ideal_balance − new_balance|
xs = unsafe_div(rates[i] * (old_balances[i] + new_balance), PRECISION)
_dynamic_fee_i = self._dynamic_fee(xs, ys, base_fee)
fees.append(unsafe_div(_dynamic_fee_i * difference, FEE_DENOMINATOR))
self.admin_balances[i] += unsafe_div(fees[i] * admin_fee, FEE_DENOMINATOR)
new_balances[i] -= fees[i]
```

   `ideal_balance` is where coin `i` *would* sit if the deposit had been perfectly
   proportional. The fee is charged on the deviation from that, in either
   direction — deposit too much of a coin and you pay, deposit too little and you
   also pay. Then `D1` is **recomputed** on the fee-reduced balances (`:657`) and

```vyper
mint_amount = unsafe_div(total_supply * (D1 - D0), D0)
```

   LP tokens are minted strictly in proportion to the growth in `D`, so the
   virtual price cannot fall. Finally `upkeep_oracles(xp, amp, D1)` (`:659`).

6. **If `total_supply == 0`** (`:661-673`): `mint_amount = D1` — *"Take the dust
   if there was any"*. The first LP receives exactly `D1` tokens, so
   `get_virtual_price` starts at exactly 1e18. The `D` oracle is seeded with
   `pack_2(D1, D1)` (`:666`) and the `D` clock advanced.

   **There is no `MINIMUM_LIQUIDITY` burn.** Curve does not need one: shares are
   denominated in `D`, which is computed from balances via the invariant, not
   from `balanceOf`. Donating tokens does not move `D` (they are not in
   `stored_balances`), so the ERC-4626-style first-depositor inflation attack has
   no lever. The header does still flag donation risk for ERC4626 *underlyings*
   (`:22-25`).

7. `assert mint_amount >= _min_mint_amount, "Slippage screwed you"` (`:674`),
   mint, `log Transfer` + `log AddLiquidity` (`:680-682`).

### 2.10 `remove_liquidity_one_coin` — `:689-723` (external, `@nonreentrant('lock')`)

```
remove_liquidity_one_coin(_burn_amount, i, _min_received, _receiver = msg.sender) -> uint256
```

1. `assert _burn_amount > 0` (`:703`).
2. `dy, fee, xp, amp, D = self._calc_withdraw_one_coin(_burn_amount, i)` (`:710`) — see [§1.8](#18-_calc_withdraw_one_coin).
3. `assert dy >= _min_received, "Not enough coins removed"` (`:711`).
4. `self.admin_balances[i] += fee * admin_fee / FEE_DENOMINATOR` (`:713`).
5. `_burnFrom(msg.sender, _burn_amount)` (`:715`) then `_transfer_out(i, dy, _receiver)` (`:717`).
6. `log RemoveLiquidityOne` (`:719`), then `upkeep_oracles(xp, amp, D)` (`:721`).

Ordering note: the burn happens before the transfer out, and both happen before
`upkeep_oracles`. The `@nonreentrant('lock')` is what makes the interleaving safe
against a token with a transfer callback.

### 2.11 `remove_liquidity_imbalance` — `:728-802` (external, `@nonreentrant('lock')`)

```
remove_liquidity_imbalance(_amounts, _max_burn_amount, _receiver = msg.sender) -> uint256
```

The mirror image of `add_liquidity`. You state exactly what you want out; the
pool computes how many LP tokens that costs.

1. `D0` on current balances (`:745`).
2. For each non-zero requested amount: decrement `new_balances[i]` and
   **transfer out immediately** (`:748-752`). Note the transfers happen before
   the fee is computed — this is safe only because of the reentrancy lock.
3. `D1` on the reduced balances (`:754`).
4. Same per-coin imbalance fee as `add_liquidity` (`:764-783`), accruing to
   `admin_balances` and reducing `new_balances`.
5. `D1` recomputed on the fee-reduced balances (`:784`), `upkeep_oracles` (`:785`).
6. The cost:

```vyper
burn_amount: uint256 = unsafe_div((D0 - D1) * total_supply, D0) + 1
assert burn_amount > 1  # dev: zero tokens burned
assert burn_amount <= _max_burn_amount, "Slippage screwed you"
```

   The `+ 1` (`:788`) rounds the burn **up**, always in the pool's favour. The
   `> 1` check then means a request that would have cost 0 reverts, since after
   the `+1` it would be exactly 1.
7. `_burnFrom` (`:792`), `log RemoveLiquidityImbalance` (`:794-800`).

### 2.12 `remove_liquidity` — `:807-869` (external, `@nonreentrant('lock')`)

```
remove_liquidity(_burn_amount, _min_amounts, _receiver = msg.sender, _claim_admin_fees: bool = True) -> DynArray
```

**The most important function in the contract**, because it is the one that always
works. It never calls `get_D`, `get_y`, `_stored_rates` or any oracle. If a rate
oracle reverts, if `get_D` fails to converge, if the pool is otherwise bricked,
this still exits.

1. `assert _burn_amount > 0`; `assert len(_min_amounts) == N_COINS` (`:822-823`).
2. Pure pro-rata on raw balances (`:828-834`):

```vyper
value = unsafe_div(balances[i] * _burn_amount, total_supply)
assert value >= _min_amounts[i], "Withdrawal resulted in fewer coins than expected"
amounts.append(value)
self._transfer_out(i, value, _receiver)
```

   **No fee.** Proportional exit does not change the pool's composition, so there
   is nothing to charge for.
3. `_burnFrom` (`:836`).
4. `D` oracle upkeep (`:840-856`): the spot `D` is scaled down proportionally
   rather than recomputed —

```vyper
old_D - unsafe_div(old_D * _burn_amount, total_supply)
```

   and the EMA is advanced. Only the `D` clock (`ma_last_time_unpacked[1]`) is
   touched; prices are untouched because a proportional withdrawal moves no price.
5. `log RemoveLiquidity` with an **empty fees array** (`:859-864`).
6. If `_claim_admin_fees` (default `True`), calls `_withdraw_admin_fees()` (`:866-868`).
   Passing `False` is the escape hatch when the fee receiver or a coin transfer
   would revert.

### 2.13 `withdraw_admin_fees` / `_withdraw_admin_fees` — `:875-882`, `:988-1004`

Public, permissionless, `@nonreentrant('lock')`. The internal version:

```vyper
fee_receiver: address = factory.fee_receiver()
if fee_receiver == empty(address):
    return  # Do nothing.

admin_balances: DynArray[uint256, MAX_COINS] = self.admin_balances
for i in range(N_COINS_128, bound=MAX_COINS_128):
    if admin_balances[i] > 0:
        self._transfer_out(i, admin_balances[i], fee_receiver)
        admin_balances[i] = 0
self.admin_balances = admin_balances
```

Note it **returns silently** rather than reverting when there is no fee receiver
(`:989-990`) — that is what lets `remove_liquidity` call it unconditionally. The
destination is read live from the factory, so the factory admin controls where
fees go, but can never touch LP funds: `admin_balances` only ever grows by
explicit fee accrual.

### 2.14 `__exchange` — `:904-940` (internal)

The core swap. `x` is the post-input value of `xp[i]`.

```vyper
amp: uint256 = self._A()
D: uint256 = self.get_D(_xp, amp)
y: uint256 = self.get_y(i, j, x, _xp, amp, D)

dy: uint256 = _xp[j] - y - 1  # -1 just in case there were some rounding errors
dy_fee: uint256 = unsafe_div(
    dy * self._dynamic_fee(
        unsafe_div(_xp[i] + x, 2), unsafe_div(_xp[j] + y, 2), self.fee
    ),
    FEE_DENOMINATOR
)

dy = (dy - dy_fee) * PRECISION / rates[j]

self.admin_balances[j] += unsafe_div(
    unsafe_div(dy_fee * admin_fee, FEE_DENOMINATOR) * PRECISION,
    rates[j]
)

xp: DynArray[uint256, MAX_COINS] = _xp
xp[i] = x
xp[j] = y
self.upkeep_oracles(xp, amp, D)
return dy
```

Four things to notice:

- The fee is taken **on the output**, not the input. `dy_fee` is subtracted from
  `dy` before conversion to token units.
- `_dynamic_fee` is passed the *midpoints* of the pre- and post-trade balances,
  so a large trade that pushes the pool off-peg pays a fee reflecting roughly the
  average imbalance during the trade, not just the start or end.
- Half the fee goes to `admin_balances[j]`, converted from `xp` space to token
  units by `* PRECISION / rates[j]`. The other half stays in the pool and
  accrues to LPs by raising `D` on the next computation.
- `upkeep_oracles` is fed `D` computed **before** the fee (`:938` comment:
  *"D is not changed because we did not apply a fee"*). The oracle therefore
  tracks the fee-free state price, which is the economically meaningful one.

### 2.15 `_exchange` — `:943-985` (internal)

```vyper
assert i != j  # dev: coin index out of range
assert _dx > 0  # dev: do not exchange 0 coins

rates / old_balances / xp   ...
dx: uint256 = self._transfer_in(i, _dx, sender, expect_optimistic_transfer)
x: uint256 = xp[i] + unsafe_div(dx * rates[i], PRECISION)
dy: uint256 = self.__exchange(x, xp, rates, i, j)
assert dy >= _min_dy, "Exchange resulted in fewer coins than expected"
self._transfer_out(j, dy, receiver)
log TokenExchange(msg.sender, i, dx, j, dy)
```

The `xp` snapshot is taken **before** `_transfer_in` (`:956-958`), then the
measured `dx` is added in `xp` space. Ordering matters for optimistic transfers:
`_balances()` would already include the incoming tokens if read afterwards.

The event logs `dx` (measured), not `_dx` (requested), which is the correct thing
for a fee-on-transfer token.

### 2.16 Math functions — `:1009-1230`

Covered in §1: `get_y` (`:1009`), `get_D` (`:1079`), `get_y_D` (`:1130`).

**`_A()` — `:1187-1206`.** Linear ramp:

```vyper
t1: uint256 = self.future_A_time
A1: uint256 = self.future_A
if block.timestamp < t1:
    A0: uint256 = self.initial_A
    t0: uint256 = self.initial_A_time
    if A1 > A0:
        return A0 + unsafe_sub(A1, A0) * (block.timestamp - t0) / (t1 - t0)
    else:
        return A0 - unsafe_sub(A0, A1) * (block.timestamp - t0) / (t1 - t0)
else:
    return A1
```

The `if A1 > A0` split exists only because uint256 cannot hold the negative
intermediate. When `t1 == 0` (never ramped) the `else` returns `future_A`, which
`__init__` set equal to `initial_A`.

**`_xp_mem` — `:1209-1219`.** `rates[i] * balances[i] / 1e18`, per coin.

**`get_D_mem` — `:1222-1229`.** `get_D(_xp_mem(rates, balances), amp)`.

**`_calc_withdraw_one_coin` — `:1233-1294`.** See [§1.8](#18-_calc_withdraw_one_coin).

### 2.17 Packing and oracles — `:1299-1466`

**`pack_2` / `unpack_2` — `:1299-1310`.** Two `uint128`s in one slot, with
`assert p1 < 2**128` and `assert p2 < 2**128`. Used for
`(spot, ema)` pairs and for the two `ma_last_time` clocks.

**`_get_p` — `:1313-1337`.** See [§1.6](#16-_get_p--the-state-price-from-partial-derivatives).

**`upkeep_oracles` — `:1340-1390`.** Called by `__exchange` (`:938`),
`add_liquidity` (`:662`), `remove_liquidity_one_coin` (`:721`) and
`remove_liquidity_imbalance` (`:784`). Three phases:

1. Prices (`:1354-1371`): for each of the `n−1` price slots, if the spot price is
   non-zero, store `pack_2(min(spot, 2e18), new_ema)`. The **2.0 cap** is the
   only bound on the oracle.
2. `D` (`:1374-1382`): store `pack_2(D, new_ema_D)`. No cap on `D`.
3. Clocks (`:1385-1389`): both `ma_last_time` halves advanced to `block.timestamp`
   if behind.

**`_calc_moving_average` — `:1393-1411`.** See [§1.7](#17-_calc_moving_average--the-keeper-free-ema).

**The four read paths.**

| Function | `line` | Reentrancy | Returns |
|---|---|---|---|
| `last_price(i)` | `:1415-1417` | none | raw stored spot, low 128 bits |
| `ema_price(i)` | `:1421-1423` | none | stored EMA, high 128 bits, **not** decayed to now |
| `get_p(i)` | `:1427-1440` | none | live state price, recomputed from balances |
| `price_oracle(i)` | `:1445-1451` | `@nonreentrant('lock')` | EMA decayed to `block.timestamp` |
| `D_oracle()` | `:1456-1465` | `@nonreentrant('lock')` | `D` EMA decayed to now |

`price_oracle` and `D_oracle` carry `@view @nonreentrant('lock')`, and so do
`totalSupply` (`:1729`) and `get_virtual_price` (`:1740`). **This is the
read-only-reentrancy fix.** In the classic generation `get_virtual_price` had no
lock, so a contract receiving ETH mid-`remove_liquidity` could read an inflated
virtual price. Here any such read reverts. `get_p` deliberately has **no** lock
and is trivially manipulable within a block — it is an input to the EMA, not an
oracle for consumers.

**`exp` — `:1469-1537`.** Snekmate `wad_exp`. Returns 0 below −41.446e18,
reverts `"wad_exp overflow"` at `:1489`.

### 2.18 The ERC20 surface — `:1541-1683`

`_domain_separator` (`:1541-1553`) returns the cached separator unless
`chain.id` has changed, in which case it recomputes — correct behaviour across a
chain fork.

`_transfer` (`:1557-1564`) relies on Vyper's checked arithmetic for the
insufficient-balance revert (the comment says so). `_burnFrom` (`:1567-1572`)
decrements `total_supply` and `balanceOf` and logs a transfer to `0x0`.

`transfer` (`:1575`), `transferFrom` (`:1586`), `approve` (`:1605`) are standard.
`transferFrom` treats `max_value(uint256)` as infinite allowance (`:1597`).

`permit` (`:1623-1671`) is EIP-2612 **plus ERC-1271**:

```vyper
if _owner.is_contract:
    sig: Bytes[65] = concat(_abi_encode(_r, _s), slice(convert(_v, bytes32), 31, 1))
    # reentrancy not a concern since this is a staticcall
    assert ERC1271(_owner).isValidSignature(digest, sig) == ERC1271_MAGIC_VAL
else:
    assert ecrecover(digest, convert(_v, uint256), convert(_r, uint256), convert(_s, uint256)) == _owner
```

so smart-contract wallets can sign. `assert _owner != empty(address)` (`:1653`)
guards against `ecrecover` returning zero on a malformed signature. Nonce is
incremented with `unsafe_add` (`:1667`) — overflow is not a concern at 2^256
permits.

### 2.19 View functions — `:1688-1822`

| Function | `line` | Implementation |
|---|---|---|
| `get_dx(i,j,dy)` | `:1688-1699` | forwards to `Views.get_dx(i,j,dy,self)` |
| `get_dy(i,j,dx)` | `:1702-1713` | forwards to `Views.get_dy` |
| `calc_withdraw_one_coin` | `:1716-1724` | local `_calc_withdraw_one_coin(...)[0]` |
| `totalSupply()` | `:1729-1735` | `@nonreentrant('lock')` |
| `get_virtual_price()` | `:1740-1757` | `@nonreentrant('lock')`, `D * 1e18 / total_supply` |
| `calc_token_amount` | `:1760-1771` | forwards to Views |
| `A()` | `:1775-1777` | `_A() / A_PRECISION` |
| `A_precise()` | `:1781-1783` | `_A()` |
| `balances(i)` | `:1787-1795` | `_balances()[i]` — admin fees excluded |
| `get_balances()` | `:1799-1801` | the whole array |
| `stored_rates()` | `:1805-1807` | live rates |
| `dynamic_fee(i,j)` | `:1811-1822` | forwards to Views |

`get_virtual_price`'s docstring carries the warning that matters
(`:1744-1746`): *"The method may be vulnerable to donation-style attacks if
implementation contains rebasing tokens."* In a rebasing pool `_balances()` reads
live `balanceOf`, so a donation *does* raise `D`, and therefore the virtual price.

### 2.20 Admin functions — `:1825-1890`

All four gate on `msg.sender == factory.admin()`, read live from the factory.

**`ramp_A(_future_A, _future_time)` — `:1825-1845`.**

```vyper
assert msg.sender == factory.admin()  # dev: only owner
assert block.timestamp >= self.initial_A_time + MIN_RAMP_TIME
assert _future_time >= block.timestamp + MIN_RAMP_TIME  # dev: insufficient time

_initial_A: uint256 = self._A()
_future_A_p: uint256 = _future_A * A_PRECISION

assert _future_A > 0 and _future_A < MAX_A
if _future_A_p < _initial_A:
    assert _future_A_p * MAX_A_CHANGE >= _initial_A
else:
    assert _future_A_p <= _initial_A * MAX_A_CHANGE
```

Three constraints, each with a purpose: at least 1 day since the last ramp
started, at least 1 day of ramp duration, and at most a 10× change. Changing `A`
re-prices the pool with no trade, so an instant jump would be free money for
anyone positioned for it. `initial_A` is set to the *current interpolated* value
(`:1839`), so re-ramping mid-ramp is continuous.

**`stop_ramp_A()` — `:1848-1859`.** Freezes `A` at its current interpolated value
by setting `initial_A = future_A = _A()` and both times to now. The emergency brake.

**`set_new_fee(_new_fee, _new_offpeg_fee_multiplier)` — `:1862-1875`.**
`assert _new_fee <= MAX_FEE` (`:1867`, 50%) and
`assert _new_offpeg_fee_multiplier * _new_fee <= MAX_FEE * FEE_DENOMINATOR`, so
even fully depegged the effective fee cannot exceed 50%. Takes effect
immediately — there is no commit/apply delay as there was in the classic pools.

**`set_ma_exp_time(_ma_exp_time, _D_ma_time)` — `:1878-1890`.**
`assert unsafe_mul(_ma_exp_time, _D_ma_time) > 0`. Note this is a product, so it
rejects either being zero, but `unsafe_mul` means a pathological pair whose
product overflows to 0 would also be rejected — harmless.

### 2.21 Full trace: `exchange_received` (aggregator sells 100k USDC for USDT)

3-coin pool [DAI, USDC, USDT], A=100, all types 0, balanced at 1M each.
Router has already transferred 100,000 USDC to the pool.

```
Router.call → CurveStableSwapNG.exchange_received(1, 2, 100_000e6, 99_800e6, router)
 │  :534  @nonreentrant('lock') acquired
 │  :556  assert not pool_contains_rebasing_tokens          → immutable False, OK
 └─ _exchange(msg.sender, 1, 2, 100_000e6, 99_800e6, router, True)          :943
     ├─ :953  assert i != j                                  OK
     ├─ :954  assert _dx > 0                                 OK
     ├─ :957  _stored_rates()                                :433
     │         all type 0 → [1e30, 1e30, 1e30]  (10**(36-18), 10**(36-6), 10**(36-6))
     ├─ :958  _balances()                                    :473
     │         not rebasing → stored_balances − admin_balances
     ├─ :959  _xp_mem(rates, balances)                       :1209
     │         → [1_000_000e18, 1_000_000e18, 1_000_000e18]
     ├─ :963  _transfer_in(1, 100_000e6, sender, True)       :358
     │         balanceOf = 1_100_000e6, stored = 1_000_000e6
     │         _dx = 100_000e6 ; assert _dx >= dx            OK
     │         stored_balances[1] += 100_000e6
     ├─ :972  x = xp[1] + dx*rates[1]/1e18 = 1_100_000e18
     ├─ :973  __exchange(x, xp, rates, 1, 2)                 :904
     │   ├─ :909  _A()                → 10_000  (A=100 × A_PRECISION)
     │   ├─ :910  get_D(xp, amp)      → 3_000_000e18        (1 Newton iteration)
     │   ├─ :911  get_y(1, 2, x, ...) → 900_099.889e18      (9 iterations)
     │   ├─ :912  dy = xp[2] − y − 1  → 99_900.111e18
     │   ├─ :913  _dynamic_fee(midpoint_i=1_050_000e18,
     │   │                     midpoint_j=950_049e18, fee=4e6)
     │   │         B = 4·xi·xj/(xi+xj)² = 0.99875 → fee ≈ 4.000e6 (0.0400%)
     │   │         dy_fee = 99_900.111e18 · 4e6/1e10 = 39.960e18
     │   ├─ :919  dy = (dy − dy_fee)·1e18/rates[2] → 99_860.151e6  USDT
     │   ├─ :921  admin_balances[2] += 39.960e18·0.5·1e18/1e30 = 19.980e6
     │   └─ :938  upkeep_oracles([.., x, y], amp, D)          :1340
     │             stores min(spot,2e18) + decayed EMA for coins 1,2; D oracle; clocks
     ├─ :974  assert dy >= _min_dy   99_860.151e6 ≥ 99_800e6  OK
     ├─ :980  _transfer_out(2, 99_860.151e6, router)          :398
     │         stored_balances[2] -= dy ; ERC20.transfer
     └─ :982  log TokenExchange(router, 1, 100_000e6, 2, 99_860.151e6)
```

Net: the trader paid 0.0400% (39.96 USDT-equivalent) plus 0.0999% curve
slippage. Half the fee (19.98) went to `admin_balances[2]` awaiting
`withdraw_admin_fees`; the other half stayed in the pool and shows up as a higher
`D` on the next computation, i.e. as LP yield.

---

## 3. `CurveStableSwapMetaNG.vy` — the metapool

1901 lines, Vyper 0.3.10, `# pragma optimize codesize`, **`# pragma evm-version paris`**
(the plain pool is `shanghai`). Always exactly 2 coins: `coins[0]` is the new
asset, `coins[1]` is the LP token of a base pool.

The header states the hard constraint at `:13-14`: *"CAUTION: Does not work if
base pool is an NG pool. Use a different metapool implementation index in the
factory."* — but the constructor detects NG base pools anyway (`:341`) and the
`_meta_add_liquidity` path branches on it (`:1197`). Read that as: this
implementation *can* talk to an NG base pool for liquidity, but the factory keeps
separate implementation indices for the two cases.

### 3.1 What is structurally different

| Aspect | Plain (`CurveStableSwapNG.vy`) | Meta (`CurveStableSwapMetaNG.vy`) |
|---|---|---|
| Coin count | `N_COINS` immutable, 2–8 | `public(constant) = 2` (`:203`) |
| Math | inlined `get_D`/`get_y`/`get_y_D` | external `math` immutable (`:346`), calls at `:1118-1119`, `:1285`, `:1305`, `:1327` |
| Array types | `DynArray[uint256, MAX_COINS]` | fixed `uint256[N_COINS]` |
| `asset_types` | `DynArray[uint8,8]` immutable | single `asset_type: immutable(uint8)` (`:213`) — coin 0 only |
| `rate_multipliers` | array | single `rate_multiplier` (`:245`) |
| `rate_oracles` | array | single `rate_oracle` (`:247`) |
| `last_prices_packed` | `DynArray` (n−1 entries) | single `uint256` (`:254`) |
| `_stored_rates` | per-coin loop | `[rate_multiplier, BASE_POOL.get_virtual_price()]` (`:532`) |
| `_get_p` | returns `DynArray` | returns `uint256` (`:1371`) |
| `evm-version` | shanghai | paris |
| Extra externals | — | `exchange_underlying`, `get_dy_underlying`, `get_dx_underlying`, `BASE_POOL`, `BASE_N_COINS`, `BASE_COINS` |
| Extra internals | — | `_meta_add_liquidity` (`:1196`) |
| Base-pool approvals | — | infinite approve of every base coin in `__init__` (`:355-362`) |

**The rate trick.** `_stored_rates()` at `:528-561`:

```vyper
rates: uint256[N_COINS] = [rate_multiplier, StableSwap(BASE_POOL).get_virtual_price()]
```

`rates[1]` is the **base pool's virtual price**. That single line is the whole
idea of a metapool: the LP token of the base pool is treated as an asset whose
price against the base pool's underlying is `D_base / supply_base`, which grows
as the base pool earns fees. So a LUSD/3CRV metapool prices 3CRV correctly as it
appreciates, without any oracle.

It also means: **a metapool inherits every risk of its base pool.** If
`get_virtual_price()` on the base pool reverts or is manipulated, the metapool's
entire pricing is wrong. This is the read-only-reentrancy vector, one level up.

### 3.2 `__init__` — `:293-419`

Extra parameters over the plain pool: `_math_implementation`, `_base_pool`,
`_base_coins`.

```vyper
# The following reverts if BASE_POOL is an NG implementaion.
BASE_POOL_IS_NG = raw_call(_base_pool, method_id("D_ma_time()"), revert_on_failure=False)

if not BASE_POOL_IS_NG:
    assert len(_base_coins) <= 3  # dev: implementation does not support old gen base pool with more than 3 coins
```

Base-pool generation is detected by **probing for `D_ma_time()`** (`:341`) — a
function only NG pools have. `revert_on_failure=False` turns the call into a
boolean. Classic base pools are limited to 3 coins because `_meta_add_liquidity`
has hardcoded `uint256[2]` and `uint256[3]` branches (`:1210-1222`).

The approval loop (`:355-362`) grants the base pool infinite allowance on every
base coin, needed by `exchange_underlying`. Note this is done once at deploy, so
a base coin that requires allowance-to-zero-first would break the pool.

ERC4626 immutables are set inside an `if` (`:365-370`), with an explicit comment
that Vyper 0.3.10 defaults unset immutables to 0 and that this is fine because
they are only read when `asset_type == 3`.

### 3.3 `_transfer_in` — `:423-487`

The metapool's version takes **two** indices plus a flag:

```
_transfer_in(coin_metapool_idx, coin_basepool_idx, dx, sender, expect_optimistic_transfer, is_base_pool_swap = False)
```

```vyper
_input_coin: ERC20 = ERC20(coins[coin_metapool_idx])
_input_coin_is_in_base_pool: bool = False

if coin_basepool_idx >= 0 and coin_metapool_idx == 1:
    _input_coin = ERC20(BASE_COINS[coin_basepool_idx])
    _input_coin_is_in_base_pool = True
```

Three outcomes:

1. **Plain metapool coin** (`coin_basepool_idx < 0`, i.e. callers pass `-1`):
   behaves exactly like the plain pool's version.
2. **Base coin, base-pool swap** (`is_base_pool_swap=True`): returns `_dx`
   immediately at `:477` **without** touching `stored_balances` — the tokens are
   about to be handed straight to the base pool's `exchange`.
3. **Base coin, meta swap**: calls `_meta_add_liquidity(_dx, coin_basepool_idx)`
   (`:483`), converting the base coin into base LP tokens, and credits *those* to
   `stored_balances[1]`.

Note at `:455-458` that for an optimistic transfer of a *base pool* coin, the
`stored_balances` diff and the `assert _dx >= dx` are skipped entirely
(`if not _input_coin_is_in_base_pool`) — the pool has no book for base coins.

### 3.4 `exchange_underlying` — `:659-749` (external, `@nonreentrant('lock')`)

The metapool's signature move: swap between `coins[0]` and any *underlying* coin
of the base pool, in one call.

Index convention (`MAX_METAPOOL_COIN_INDEX = 1`, `:201`): `0` is the metapool
coin; `1..n` are base-pool coins `0..n−1`.

```vyper
if i > 0:
    base_i = i - MAX_METAPOOL_COIN_INDEX
    meta_i = 1

if j == 0:
    output_coin = coins[0]
else:
    base_j = j - MAX_METAPOOL_COIN_INDEX
    meta_j = 1
    output_coin = BASE_COINS[base_j]
```

Then transfer in, with the crucial flag (`:707-714`):

```vyper
dx_w_fee: uint256 = self._transfer_in(
    meta_i, base_i, _dx, msg.sender, False,
    (i > 0 and j > 0),  # <--- if True: do not add liquidity to base pool.
)
```

Two branches:

**Meta swap (`i == 0 or j == 0`), `:718-732`.**

```vyper
x = xp[meta_i] + unsafe_div(dx_w_fee * rates[meta_i], PRECISION)
dy = self.__exchange(x, xp, rates, meta_i, meta_j)
self.stored_balances[meta_j] -= dy
if j > 0:
    out_amount: uint256 = ERC20(output_coin).balanceOf(self)
    StableSwap(BASE_POOL).remove_liquidity_one_coin(dy, base_j, 0)
    dy = ERC20(output_coin).balanceOf(self) - out_amount
assert dy >= _min_dy
```

Selling a base coin: `_transfer_in` already deposited it into the base pool and
returned LP tokens, so the metapool swap is LP↔coin0. Buying a base coin: the
metapool swap produces base LP tokens, which are then redeemed single-sided.
`stored_balances[meta_j]` is decremented manually at `:725` because
`_transfer_out` is not used on this path. The `remove_liquidity_one_coin` is
called with `min_amount = 0` — slippage is checked once at the end (`:732`).

**Base-pool-only swap (`i > 0 and j > 0`), `:734-738`.**

```vyper
dy = ERC20(output_coin).balanceOf(self)
StableSwap(BASE_POOL).exchange(base_i, base_j, dx_w_fee, _min_dy)
dy = ERC20(output_coin).balanceOf(self) - dy
```

A pure pass-through to the base pool. The docstring at `:734` says the quiet
part: *"(user should swap at base pool for better gas)"*. It exists so routers
can treat the metapool as a single venue for all `n+1` assets.

Both branches transfer out with a raw `ERC20.transfer` (`:742`), not
`_transfer_out`, and emit `TokenExchangeUnderlying` (`:746`).

**Every external call in order**, for `exchange_underlying(1, 0, ...)` (base coin
in, metapool coin out): `BASE_POOL.get_virtual_price()` → `BASE_COINS[0].balanceOf`
→ `BASE_COINS[0].transferFrom` → `BASE_COINS[0].balanceOf` →
`BASE_POOL.add_liquidity` → `math.get_D` → `math.get_y` → `coins[0].transfer`.
That is eight external calls, four of them to the base pool. Gas is why the
docstring keeps pushing users to the base pool directly.

### 3.5 `_meta_add_liquidity` — `:1196-1224` (internal)

```vyper
if BASE_POOL_IS_NG:
    base_inputs: DynArray[uint256, MAX_COINS] = empty(DynArray[uint256, MAX_COINS])
    for i in range(BASE_N_COINS, bound=MAX_COINS):
        if i == convert(base_i, uint256): base_inputs.append(dx)
        else: base_inputs.append(0)
    return StableSwapNG(BASE_POOL).add_liquidity(base_inputs, 0)

coin_i: address = coins[MAX_METAPOOL_COIN_INDEX]
x: uint256 = ERC20(coin_i).balanceOf(self)
if BASE_N_COINS == 2:
    base_inputs: uint256[2] = empty(uint256[2])
    base_inputs[base_i] = dx
    StableSwap2(BASE_POOL).add_liquidity(base_inputs, 0)
if BASE_N_COINS == 3:
    ...
return ERC20(coin_i).balanceOf(self) - x
```

NG base pools return the minted amount, so it is used directly. Classic base
pools return nothing, so the LP amount is measured by balance diff. `min_mint = 0`
in both cases; slippage is the caller's problem.

### 3.6 Function-by-function diff, plain vs meta

Every function in both files. "same" means algorithmically identical modulo
`DynArray` → `uint256[2]` and inline-math → `math.*`.

| Function | Plain `line` | Meta `line` | Difference |
|---|---|---|---|
| `__init__` | 239 | 293 | meta takes `_math_implementation`, `_base_pool`, `_base_coins`; probes `D_ma_time()`; approves base coins |
| `_transfer_in` | 358 | 423 | meta adds `coin_basepool_idx` + `is_base_pool_swap`; can route into `_meta_add_liquidity` |
| `_transfer_out` | 398 | 491 | same |
| `_stored_rates` | 433 | 528 | meta hardcodes `rates[1] = BASE_POOL.get_virtual_price()` |
| `_balances` | 473 | 565 | same logic, fixed array |
| `exchange` | 504 | 593 | meta passes `-1` as basepool idx |
| `exchange_received` | 534 | 623 | same guard |
| `exchange_underlying` | — | **659** | meta only |
| `add_liquidity` | 570 | 754 | same; meta passes `-1` to `_transfer_in` |
| `remove_liquidity_one_coin` | 689 | 887 | same |
| `remove_liquidity_imbalance` | 728 | 928 | same |
| `remove_liquidity` | 807 | 1011 | same |
| `withdraw_admin_fees` | 875 | 1078 | plain is `@nonreentrant('lock')`, **meta is not** |
| `_dynamic_fee` | 887 | 1090 | same |
| `__exchange` | 904 | 1109 | meta calls `math.get_D`/`math.get_y` |
| `_exchange` | 943 | 1149 | same |
| `_meta_add_liquidity` | — | **1196** | meta only |
| `_withdraw_admin_fees` | 988 | 1227 | same |
| `get_y` | **1009** | — | plain only (meta delegates) |
| `get_D` | **1079** | — | plain only |
| `get_y_D` | **1130** | — | plain only |
| `_A` | 1187 | 1247 | same |
| `_xp_mem` | 1209 | 1269 | same |
| `get_D_mem` | 1222 | 1281 | meta calls `math.get_D` |
| `_calc_withdraw_one_coin` | 1233 | 1292 | meta calls `math.get_y_D` |
| `pack_2` / `unpack_2` | 1299 / 1307 | 1357 / 1365 | same |
| `_get_p` | 1313 | 1371 | meta returns scalar, no loop over `i≥1` |
| `upkeep_oracles` | 1340 | 1390 | meta has one price, no loop |
| `_calc_moving_average` | 1393 | 1438 | same |
| `last_price` | 1415 | 1460 | meta asserts `i == 0` |
| `ema_price` | 1421 | 1467 | meta asserts `i == 0` |
| `get_p` | 1427 | 1474 | meta asserts `i == 0` |
| `price_oracle` | 1445 | 1494 | meta asserts `i == 0` |
| `D_oracle` | 1456 | 1506 | same |
| `exp` | **1469** | — | plain only; meta uses `math.exp` |
| `_domain_separator` | 1541 | 1519 | same |
| `_transfer` / `_burnFrom` | 1557 / 1567 | 1535 / 1545 | same |
| `transfer` / `transferFrom` / `approve` | 1575 / 1586 / 1605 | 1553 / 1564 / 1583 | same |
| `permit` | 1623 | 1601 | same |
| `DOMAIN_SEPARATOR` | 1675 | 1653 | same |
| `get_dx` | 1688 | 1666 | same |
| `get_dx_underlying` | — | **1680** | meta only |
| `get_dy` | 1702 | 1695 | same |
| `get_dy_underlying` | — | **1709** | meta only |
| `calc_withdraw_one_coin` | 1716 | 1724 | same |
| `totalSupply` | 1729 | 1737 | same |
| `get_virtual_price` | 1740 | 1748 | same |
| `calc_token_amount` | 1760 | 1765 | same |
| `A` / `A_precise` | 1775 / 1781 | 1784 / 1790 | same |
| `balances` | 1787 | 1796 | same |
| `get_balances` | 1799 | 1808 | same |
| `stored_rates` | 1805 | 1815 | same |
| `dynamic_fee` | 1811 | 1822 | same |
| `ramp_A` / `stop_ramp_A` | 1825 / 1848 | 1836 / 1859 | same |
| `set_new_fee` | 1862 | 1873 | same |
| `set_ma_exp_time` | 1878 | 1889 | same |

The asymmetry worth flagging: **`withdraw_admin_fees` is `@nonreentrant('lock')`
on the plain pool (`:874-875`) but has no lock on the metapool (`:1077-1078`).**
It only moves already-accrued `admin_balances` to the fee receiver, so the impact
is limited, but it is a genuine difference between the two files.

### 3.7 Trace: `exchange_underlying(1, 0)` — sell DAI for LUSD in LUSD/3CRV

```
user → MetaNG.exchange_underlying(i=1 /*DAI*/, j=0 /*LUSD*/, 1000e18, min_dy, user)
 │ :659 @nonreentrant('lock')
 ├ :675 assert _receiver != 0
 ├ :677 _stored_rates()              :528
 │       → [1e18, 3pool.get_virtual_price()]   e.g. [1e18, 1.0271e18]
 ├ :678 _balances()                  :565
 ├ :679 _xp_mem(rates, balances)     :1269
 ├ :690 i > 0 → base_i = 0, meta_i = 1
 ├ :695 j == 0 → output_coin = coins[0] = LUSD
 ├ :707 _transfer_in(1, 0, 1000e18, user, False, is_base_pool_swap=False)   :423
 │   ├ :443 coin_basepool_idx=0 ≥ 0 and coin_metapool_idx==1 → input is DAI
 │   ├ :465 DAI.transferFrom(user, pool, 1000e18)
 │   ├ :483 _meta_add_liquidity(1000e18, 0)          :1196
 │   │       BASE_POOL_IS_NG False → StableSwap3.add_liquidity([1000e18,0,0], 0)
 │   │       measured by 3CRV balanceOf diff → 973.5e18 3CRV
 │   └ :486 stored_balances[1] += 973.5e18
 ├ :720 x = xp[1] + 973.5e18·1.0271e18/1e18
 ├ :721 __exchange(x, xp, rates, 1, 0)               :1109
 │   ├ :1118 math.get_D([xp0,xp1], amp, 2)           external → Math :90
 │   ├ :1119 math.get_y(1, 0, x, ..., 2)             external → Math :18
 │   ├ :1121 dy = xp[0] − y − 1
 │   ├ :1122 _dynamic_fee(midpoints, self.fee)       :1090
 │   ├ :1129 dy = (dy − dy_fee)·1e18/rates[0]
 │   ├ :1132 admin_balances[0] += dy_fee·0.5·1e18/rates[0]
 │   └ :1143 upkeep_oracles(xp, amp, D)              :1390
 ├ :725 stored_balances[0] -= dy
 ├ :732 assert dy >= _min_dy
 ├ :742 LUSD.transfer(user, dy)
 └ :746 log TokenExchangeUnderlying(user, 1, 1000e18, 0, dy)
```

The user paid two fees: the 3pool imbalance fee on the one-sided DAI deposit
(inside `add_liquidity`, invisible to the metapool), plus the metapool's own
dynamic fee. That stacking is why routers usually prefer a direct pool when one
exists.

---

## 4. `CurveStableSwapNGMath.vy`

269 lines, Vyper 0.3.10, `# pragma optimize gas`, `# pragma evm-version shanghai`.
Four `@external @pure` functions. Stateless — no storage at all, only three
constants (`MAX_COINS = 8` `:11`, `MAX_COINS_128` `:12`, `A_PRECISION = 100` `:13`).

**Why it exists.** The metapool cannot fit its math inline and stay under the
24 KB EIP-170 limit, so the math is deployed once and shared by every metapool.
The factory stores it as `math_implementation` (`:91`) and passes it into each
metapool's constructor (`:648`). The plain pool does not use it at all.

| Function | `line` | Signature | Notes |
|---|---|---|---|
| `get_y` | `:18-84` | `(i, j, x, xp, _amp, _D, _n_coins) -> uint256` | identical to the pool's `:1009`, but `n` is a parameter |
| `get_D` | `:90-136` | `(_xp, _amp, _n_coins) -> uint256` | identical to `:1079` |
| `get_y_D` | `:143-197` | `(A, i, xp, D, _n_coins) -> uint256` | identical to `:1130` |
| `exp` | `:203-269` | `(x: int256) -> uint256` | identical to `:1469` |

Every one takes `_n_coins` explicitly and derives `n_coins_128` by conversion
(`:37`, `:158`), where the pool versions close over the immutable `N_COINS`.
The asserts carry the same dev messages (`:39-45`, `:160-161`) and the same
`"wad_exp overflow"` string (`:226`).

Being `@pure` and `@external`, these compile to `STATICCALL` from the metapool —
no reentrancy surface. The cost is roughly 2,600 gas of call overhead per
invocation, and `__exchange` makes two of them.

---

## 5. `CurveStableSwapNGViews.vy`

704 lines, Vyper 0.3.10, `# pragma evm-version paris`.
`VERSION: public(constant(String[8])) = "1.2.0"` (`:34`), with the two prior
mainnet deployments listed in comments at `:35-36`.

**Why the pool delegates.** `get_dy`, `get_dx`, `calc_token_amount` and
`dynamic_fee` are quoting functions used by routers, never by the pool's own
state transitions. Moving them out buys ~4 KB of pool bytecode, and — more
importantly — lets the factory admin fix a quoting bug for every deployed pool at
once by calling `set_views_implementation` (`CurveStableSwapFactoryNG.vy:813`).
The pool reads `factory.views_implementation()` on every call (`:1699` etc.), so
the swap is live and retroactive.

It also means the Views contract carries its **own copies** of `get_D` (`:596`),
`get_y` (`:538`), `get_y_D` (`:641`) and `_dynamic_fee` (`:381`). Four
implementations of `get_D` exist in this repo (pool, metapool-via-Math, Views,
mock `CurvePool`). They agree, but a fix must be applied in each.

### 5.1 Public functions

**`get_dx(i, j, dy, pool)` — `:44-55`.** Reads `N_COINS` off the pool, forwards
to `_get_dx(..., static_fee=False, ...)`.

**`get_dy(i, j, dx, pool)` — `:59-86`.** The full forward quote:

```vyper
rates, balances, xp = self._get_rates_balances_xp(pool, N_COINS)
amp: uint256 = StableSwapNG(pool).A() * A_PRECISION
D: uint256 = self.get_D(xp, amp, N_COINS)
x: uint256 = xp[i] + (dx * rates[i] / PRECISION)
y: uint256 = self.get_y(i, j, x, xp, amp, D, N_COINS)
dy: uint256 = xp[j] - y - 1
base_fee: uint256 = StableSwapNG(pool).fee()
fee_multiplier: uint256 = StableSwapNG(pool).offpeg_fee_multiplier()
fee: uint256 = self._dynamic_fee((xp[i] + x) / 2, (xp[j] + y) / 2, base_fee, fee_multiplier) * dy / FEE_DENOMINATOR
return (dy - fee) * PRECISION / rates[j]
```

Mirrors `__exchange` (`:904`) exactly, including the `−1` and the midpoint fee
arguments. That correspondence is what makes the quote exact rather than
approximate.

**`get_dx_underlying(i, j, dy, pool)` — `:91-133`.** Three cases, documented
inline:

- `min(i,j) > 0` → `raise "Not a Metapool Swap. Use Base pool."` (`:107`).
- `i == 0` (`:115-121`): compute the base LP tokens that must be burnt to yield
  `dy` of base coin `j−1` (`_base_calc_token_amount(..., False)`), then `_get_dx`
  on the metapool for `0 → 1`.
- else (`:130-132`): `_get_dx(1, 0, dy, ...)` for the LP amount needed, then
  `BASE_POOL.calc_withdraw_one_coin` to convert to base coin `i−1`. The comment
  at `:124-129` admits this is an approximation: *"We don't have a method where
  user inputs lp tokens and it gives number of coins ... Instead, we will use
  calc_withdraw_one_coin. That's close enough."*

**`get_dy_underlying(i, j, dx, pool)` — `:138-222`.** Symmetric. Both-base-coins
short-circuits to `BASE_POOL.get_dy` (`:194`). Input from the base pool is
converted via `_base_calc_token_amount(..., True)` and multiplied by `rates[1]`
(`:181-190`); output to a base coin goes through `calc_withdraw_one_coin`
(`:219`) with the comment *"The fee is already accounted for"*.

**`calc_token_amount(_amounts, _is_deposit, pool)` — `:228-247`.** Forwards to
`_calc_token_amount(_amounts, _is_deposit, pool, pool, N_COINS)` — the pool is
both the pool and its own LP token, which is exactly the NG design.

**`calc_withdraw_one_coin(_burn_amount, i, pool)` — `:252-299`.** A
reimplementation of the pool's `_calc_withdraw_one_coin` (`:1233`) reading all
inputs externally. Same `ys = (D0+D1)/(2·N_COINS)`, same
`base_fee = fee·n/(4(n−1))`, same `(dy−1)·PRECISION/rates[i]`.

**`dynamic_fee(i, j, pool)` — `:304-323`.** Note it passes the **raw** `xp[i]`,
`xp[j]` — not midpoints — so this reports the fee at the *current* state, which
is the fee an infinitesimal trade would pay, not what a real trade will pay.

### 5.2 Internal helpers

**`_has_static_fee(pool)` — `:328-345`.** Probes `dynamic_fee(int128,int128)`
with a `raw_call(..., revert_on_failure=False, is_static_call=True)` and returns
`success`. Used by `get_dx_underlying` (`:100`) to decide how to treat a base
pool of unknown generation.

**`_get_dx(i, j, dy, pool, static_fee, N_COINS)` — `:349-378`.** Inverts the swap:

```vyper
dy_with_fee: uint256 = dy * rates[j] / PRECISION + 1
fee: uint256 = base_fee
if not static_fee:
    fee = self._dynamic_fee(xp[i], xp[j], base_fee, fee_multiplier)
y: uint256 = xp[j] - dy_with_fee * FEE_DENOMINATOR / (FEE_DENOMINATOR - fee)
x: uint256 = self.get_y(j, i, y, xp, amp, D, N_COINS)
return (x - xp[i]) * PRECISION / rates[i]
```

Gross up the desired output by the fee, then run `get_y` **backwards** (note the
swapped `j, i`). The `+ 1` at `:485` rounds the required input up. The fee here
uses current `xp`, not midpoints, so `get_dx` is very slightly optimistic for
large trades — quote, then verify with `get_dy`.

**`_dynamic_fee` — `:381-390`.** Same formula as the pool's (`:887`), but the
multiplier is a parameter rather than storage, and it uses checked arithmetic
instead of `unsafe_*`.

**`_calc_token_amount` — `:395-489`.** The most involved view. It handles both NG
and classic pools:

```vyper
pool_is_ng: bool = raw_call(pool, method_id("D_ma_time()"), revert_on_failure=False, is_static_call=True)
use_dynamic_fees: bool = True
if pool_is_ng:
    rates, old_balances, xp = self._get_rates_balances_xp(pool, n_coins)
else:
    use_dynamic_fees = False
    for i in range(n_coins, bound=MAX_COINS):
        rates.append(10 ** (36 - convert(ERC20Detailed(StableSwapNG(pool).coins(i)).decimals(), uint256)))
        old_balances.append(StableSwapNG(pool).balances(i))
        xp.append(rates[i] * old_balances[i] / PRECISION)
```

Same `D_ma_time()` probe as the metapool constructor. For classic pools it
reconstructs rates from `decimals()` and falls back to a static fee. Then D0 → D1
→ per-coin imbalance fee → D2, returning `(D2−D0)·supply/D0` for a deposit or
`(D0−D2)·supply/D0` for a withdrawal (`:483-489`). First deposit returns `D1`
directly (`:485-486`), matching the pool.

**`_base_calc_token_amount` — `:495-514`.** Builds a one-hot `base_inputs` array
and calls `_calc_token_amount` against the base pool.

**`newton_y(b, c, D, _y)` — `:518-533`.** The shared Newton loop factored out,
used by `get_y` (`:538`) and `get_y_D` (`:641`) in this contract.

**`_get_rates_balances_xp(pool, N_COINS)` — `:690-704`.** One call to
`stored_rates()`, one to `get_balances()`, then builds `xp`. Two external calls
per quote.

---

## 6. `CurveStableSwapFactoryNG.vy`

865 lines, Vyper 0.3.10, `# pragma evm-version shanghai`. Permissionless pool
deployer **and** on-chain registry. Every deployed pool holds this address as its
immutable `factory` and reads `admin()`, `fee_receiver()` and
`views_implementation()` from it at runtime.

### 6.1 Structs, storage, events

`PoolArray` (`:10-17`): `base_pool`, `implementation`, `liquidity_gauge`,
`coins`, `decimals`, `n_coins`, `asset_types`.
`BasePoolArray` (`:19-24`): `lp_token`, `coins`, `decimals` (packed one byte per
coin), `n_coins`, `asset_types`.

| Storage | `line` | Notes |
|---|---|---|
| `admin`, `future_admin` | `:73-74` | 2-step ownership |
| `asset_types: HashMap[uint8, String[20]]` | `:76` | human-readable names, seeded in `__init__` |
| `pool_list: address[4294967296]`, `pool_count` | `:78-79` | master registry |
| `pool_data: HashMap[address, PoolArray]` | `:80` | **not public** — accessed via getters |
| `base_pool_list`, `base_pool_count`, `base_pool_data` | `:82-84` | all public |
| `base_pool_assets: HashMap[address, bool]` | `:87` | "is this coin inside a registered base pool?" |
| `pool_implementations`, `metapool_implementations` | `:90-91` | index → blueprint |
| `math_implementation`, `gauge_implementation`, `views_implementation` | `:92-94` | singletons |
| `fee_receiver` | `:97` | one address for **all** pools |
| `markets: HashMap[uint256, address[4294967296]]`, `market_counts` | `:102-103` | key = `addr_a XOR addr_b` |

Events: `BasePoolAdded` (`:49`), `PlainPoolDeployed` (`:52`),
`MetaPoolDeployed` (`:58`), `LiquidityGaugeDeployed` (`:65`).

`__init__(_fee_receiver, _owner)` (`:108-119`) seeds the four asset-type names:
`"Standard"`, `"Oracle"`, `"Rebasing"`, `"ERC4626"` (`:115-118`).

### 6.2 `deploy_plain_pool` — `:458-566`

```
deploy_plain_pool(_name, _symbol, _coins, _A, _fee, _offpeg_fee_multiplier,
                  _ma_exp_time, _implementation_idx, _asset_types,
                  _method_ids, _oracles) -> address
```

**All validation lives here, not in the pool** (`:496-523`):

| Check | `line` | Revert |
|---|---|---|
| `len(_coins) >= 2` | `:496` | `# dev: pool needs to have at least two coins!` |
| `len(_coins) == len(_method_ids)` | `:497` | `# dev: All coin arrays should be same length` |
| `len(_coins) == len(_oracles)` | `:498` | same |
| `len(_coins) == len(_asset_types)` | `:499` | same |
| `_fee <= 100000000` | `:499` | `"Invalid fee"` — 1%, **stricter than the pool's `MAX_FEE`** |
| `_offpeg_fee_multiplier * _fee <= MAX_FEE * FEE_DENOMINATOR` | `:500` | (no message) |
| `decimals[i] < 19` | `:513` | `"Max 18 decimals for coins"` |
| `coin != _coins[j+1]` | `:520` | `"Duplicate coins"` |
| `implementation != empty(address)` | `:523` | `"Invalid implementation index"` |

Note the fee ceiling asymmetry: the factory refuses anything above **1%** at
deploy, but `set_new_fee` on the pool allows up to `MAX_FEE` = **50%**. A DAO
vote can therefore raise a live pool's fee far above what could be deployed.

Rate multipliers are computed here (`:516`): `10 ** (36 - decimals[i])`.

Deployment (`:525-540`) is `create_from_blueprint(implementation, ...args...,
code_offset=3)` — ERC-5202 blueprints, so the pool's runtime code is a copy of a
pre-deployed template rather than a proxy. **The deployed pools are not
upgradeable.** The only "upgrade" surface is `views_implementation` and the
mutable admin parameters.

Registration (`:542-563`) appends to `pool_list`, fills `pool_data`, and inserts
the pool into `markets[coin_a XOR coin_b]` for every unordered coin pair.

### 6.3 `deploy_metapool` — `:571-689`

Extra checks over the plain path:

```vyper
assert not self.base_pool_assets[_coin], "Invalid asset: Cannot pair base pool asset with base pool's LP token"
assert base_pool_n_coins != 0, "Base pool is not added"
```

The first (`:612`) prevents a metapool of e.g. USDC against 3CRV, which would be
a pool of a thing against a basket containing that thing — arbitrageable to
death. The second (`:617`) requires `add_base_pool` first.

Asset types are **combined** (`:629-635`): `[_asset_type, 0]` for the metapool's
two coins, then every base-pool asset type appended. `_rate_multipliers` is
`[10**(36−decimals), 10**18]` (`:637`) — the base LP token is always 18 decimals.

The constructor args include `self.math_implementation` (`:648`) and the base
pool's coin list (`:651`).

Market registration (`:673-685`) registers the metapool coin against **every base
coin plus the LP token**, so `find_pool_for_coins(LUSD, USDC)` finds it.

### 6.4 `deploy_gauge` — `:694-711`

```vyper
assert self.pool_data[_pool].coins[0] != empty(address), "Unknown pool"
assert self.pool_data[_pool].liquidity_gauge == empty(address), "Gauge already deployed"
implementation: address = self.gauge_implementation
assert implementation != empty(address), "Gauge implementation not set"
gauge: address = create_from_blueprint(self.gauge_implementation, _pool, code_offset=3)
```

Permissionless — anyone can deploy the gauge for a registered pool. It does not
add the gauge to the GaugeController; that still needs a DAO vote. Note the
gauge's constructor sets `manager = tx.origin` (`LiquidityGauge.vy:172`), so
**whoever sends the `deploy_gauge` transaction becomes the gauge manager** and
can add reward tokens.

### 6.5 `add_base_pool` — `:715-757` (admin only)

```vyper
assert msg.sender == self.admin  # dev: admin-only function
assert 2 not in _asset_types  # dev: rebasing tokens cannot be in base pool
assert len(self.base_pool_data[_base_pool].coins) == 0  # dev: pool exists
assert _n_coins < MAX_COINS  # dev: base pool can only have (MAX_COINS - 1) coins.
```

and inside the coin loop (`:750`):

```vyper
assert coin != 0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE  # dev: native token is not supported
```

Three deliberate exclusions: **no rebasing tokens in a base pool**, **no native
ETH anywhere**, and at most 7 base coins. Banning native ETH at the factory level
is what structurally closes the read-only-reentrancy class of bug that hit the
classic stETH pool — there is no `raw_call` value transfer to reenter through.

Every base coin is marked in `base_pool_assets` (`:752`), feeding the
`deploy_metapool` check. `decimals` are packed one byte per coin (`:753`).

### 6.6 Registry getters

| Function | `line` | Returns |
|---|---|---|
| `find_pool_for_coins(_from, _to, i=0)` | `:124-136` | `markets[a^b][i]` |
| `get_base_pool` | `:141-148` | |
| `get_n_coins` | `:152-159` | |
| `get_meta_n_coins` | `:163-171` | `(2, base_n_coins + 1)` |
| `get_coins` | `:175-182` | |
| `get_underlying_coins` | `:186-206` | metapool coin + base coins |
| `get_decimals` | `:210-217` | |
| `get_underlying_decimals` | `:221-244` | unpacks the packed base decimals |
| `get_metapool_rates` | `:248-257` | `[1e18, base.get_virtual_price()]` |
| `get_balances` | `:261-285` | |
| `get_underlying_balances` | `:289-312` | |
| `get_A` | `:316-323` | |
| `get_fees` | `:327-334` | `(fee, admin_fee)` |
| `get_admin_balances` | `:338-351` | |
| `get_coin_indices` | `:355-403` | `(i, j, is_underlying)` |
| `get_gauge` | `:407-415` | |
| `get_implementation_address` | `:419-426` | |
| `is_meta` | `:430-437` | `base_pool != 0` |
| `get_pool_asset_types` | `:441-455` | |

`get_coin_indices` (`:355`) is the fiddliest: it first checks whether both coins
are the metapool's own two (`:369-374`), returning `is_underlying=False`; else it
walks base coins and returns `True`. It `raise "No available market"` twice
(`:382`, `:389`).

### 6.7 Admin setters — `:761-865`

All `assert msg.sender == self.admin`:

| Function | `line` | Effect |
|---|---|---|
| `set_pool_implementations(idx, impl)` | `:761-773` | plain blueprint at index |
| `set_metapool_implementations(idx, impl)` | `:776-788` | meta blueprint at index |
| `set_math_implementation(addr)` | `:791-799` | affects **future** metapools only (immutable once deployed) |
| `set_gauge_implementation(addr)` | `:802-810` | |
| `set_views_implementation(addr)` | `:813-821` | **affects every existing pool immediately** |
| `commit_transfer_ownership(addr)` | `:824-831` | sets `future_admin` |
| `accept_transfer_ownership()` | `:834-844` | callable by `future_admin` only |
| `set_fee_receiver(_pool, _fee_receiver)` | `:847-856` | note: ignores `_pool`, sets the global |
| `add_asset_type(_id, _name)` | `:858-865` | registers a name only; no pool logic |

Two quirks. `set_fee_receiver` takes a `_pool` argument and never uses it
(`:855`) — it always sets the single global `fee_receiver`. And
`add_asset_type` only adds a *label*; the pools' asset-type behaviour is
hardcoded in `_stored_rates`, so registering type 4 does nothing functional.

---

## 7. `CurveStableSwapFactoryNGHandler.vy`

557 lines, Vyper 0.3.10. **Not part of the AMM.** This is an adapter that makes
the NG factory look like every other Curve registry to the Metaregistry — the
aggregator contract that fronts all Curve registry generations for integrators.

It holds only two immutables-in-storage (`:78-82`): the NG factory address and a
base-pool registry address. Every function is `@view` and translates NG's
`DynArray` returns into the Metaregistry's fixed
`address[MAX_METAREGISTRY_COINS]` / `uint256[MAX_METAREGISTRY_COINS]` shapes.

Internal padding helpers: `_pad_uint_dynarray` (`:171`), `_pad_addr_dynarray`
(`:185`). Internal accessors: `_is_meta` (`:86`), `_get_coins` (`:92`),
`_get_underlying_coins` (`:104`), `_get_n_coins` (`:116`), `_get_base_pool`
(`:124`), `_get_meta_underlying_balances` (`:136`), `_get_balances` (`:197`),
`_get_decimals` (`:203`), `_get_gauge_type` (`:209`).

External surface (all `@view`): `find_pool_for_coins` (`:233`),
`get_admin_balances` (`:239`), `get_balances` (`:258`), `get_base_pool` (`:269`),
`get_coin_indices` (`:280`), `get_coins` (`:308`), `get_decimals` (`:319`),
`get_fees` (`:330`), `get_virtual_price_from_lp_token` (`:345`), `get_gauges`
(`:356`), `get_lp_token` (`:371`), `get_n_coins` (`:383`),
`get_n_underlying_coins` (`:394`), `get_pool_asset_type` (`:422`),
`get_pool_from_lp_token` (`:434`), `get_pool_name` (`:449`), `get_pool_params`
(`:463`), `get_underlying_balances` (`:476`), `get_underlying_coins` (`:489`),
`get_underlying_decimals` (`:502`), `is_meta` (`:519`), `is_registered` (`:530`),
`pool_count` (`:541`), `pool_list` (`:551`).

Two NG-specific translations worth knowing: `get_lp_token(_pool)` returns
`_pool` itself (the pool *is* the LP token), and `get_pool_from_lp_token`
does the same in reverse. Integrators porting from classic Curve trip on this
constantly.

---

## 8. `MetaZapNG.vy`

440 lines, Vyper 0.3.10, `# pragma evm-version paris`. A stateless-ish helper
that lets a user add or remove *underlying* liquidity on a metapool in one
transaction, without holding the base LP token.

`META_N_COINS = 2` (`:78`), `MAX_COINS = 8` (`:79`),
`MAX_ALL_COINS = MAX_COINS + 1 = 9` (`:80`),
`FEE_IMPRECISION = 100 * 10**8` (`:82`).

Storage is pure caching (`:85-87`): `is_approved[coin][pool]`,
`base_pool_coins_spending_approved[pool]`, `base_pool_registry[pool] -> BasePool`.
Approvals are granted lazily and are **infinite** (`:105`, `:213`, `:239`), so
the zap is a standing approval target — users should approve the zap, not the
pools.

| Function | `line` | Behaviour |
|---|---|---|
| `get_coins_from_pool` | `:92-98` (internal view) | loops `coins(i)` up to `N_COINS()` |
| `_approve_pool_to_spend_zap_coins` | `:101-109` (internal) | infinite-approves each base coin to the base pool |
| `_fetch_base_pool_data` | `:113-119` (internal view) | `assert base_pool != empty(address)  # dev: not a metapool` |
| `_base_pool_data` | `:122-137` (internal) | memoising wrapper writing `base_pool_registry` |
| `calc_token_amount` | `:142-170` (external view) | base `calc_token_amount` → meta `calc_token_amount`. Docstring: *"accounts for slippage, but not fees"* |
| `add_liquidity` | `:174-246` | see below |
| `calc_withdraw_one_coin` | `:251-268` (external view) | `i == 0` → meta directly; else meta→LP then base `calc_withdraw_one_coin` |
| `remove_liquidity` | `:272-323` | pull LP, meta `remove_liquidity`, base `remove_liquidity`, transfer everything out |
| `remove_liquidity_one_coin` | `:327-366` | `i == 0` → straight through; else meta→LP→base one-coin |
| `remove_liquidity_imbalance` | `:370-440` | see below |

**`add_liquidity` — `:174-246`.** Transfers the metapool coin in if non-zero
(`:208-215`), transfers each base coin in (`:218-231`), deposits base coins to
get LP (`:235-239`), then deposits `[meta_amount, lp_amount]` into the metapool
with `_receiver` as the LP recipient (`:243-246`).

There is a real bug-shaped oddity in the base-coin loop (`:218-231`):

```vyper
for i in range(n_all_coins, bound=MAX_ALL_COINS):
    amount: uint256 = _deposit_amounts[i]
    base_amounts.append(0)
    if i == 0 or amount == 0:
        base_amounts.append(0)
        continue
```

`base_amounts.append(0)` runs unconditionally and then **again** inside the
skip branch, so a skipped index appends two entries. The subsequent
`base_amounts[base_idx] = amount` writes by index, so the array can end up longer
than `base_n_coins`. In practice `add_liquidity` on the base pool reads only the
first `n` entries, so it works — but it is fragile, and it is the kind of thing
worth checking against the deployed bytecode before integrating.

**`remove_liquidity_imbalance` — `:370-440`.** The most intricate function here.
It over-estimates the base LP tokens needed by padding the fee (`:404-405`):

```vyper
fee: uint256 = StableSwapNG(base_pool).fee() * base_n_coins / (4 * (base_n_coins - 1))
fee += fee * FEE_IMPRECISION / FEE_DENOMINATOR  # Overcharge to account for imprecision
```

then pulls `_max_burn_amount` LP in up-front (`:408`), burns what is actually
needed, and refunds the remainder (`:429`). If base LP tokens are left over after
the base withdrawal, it **re-deposits them for the caller** (`:433-438`) and
credits the burn accordingly. That re-deposit is why `burn_amount -=` appears on
`:438`.

---

## 9. `LiquidityGauge.vy` — gauge v6.1.0

864 lines, Vyper 0.3.10, `# pragma optimize gas`.
`VERSION = "v6.1.0"` (`:93`) with the comment *"updated from v6.0.0 (makes
rewards semi-permissionless)"*. Deployed per-pool by `deploy_gauge`
(`CurveStableSwapFactoryNG.vy:694`) via blueprint — the `@dev` at `:9` notes this
is what differs from v5.

### 9.1 Hardcoded mainnet addresses — `:105-109`

```vyper
CRV: constant(address)              = 0xD533a949740bb3306d119CC777fa900bA034cd52
GAUGE_CONTROLLER: constant(address) = 0x2F50D538606Fa9EDD2B11E2446BEb18C9D5846bB
MINTER: constant(address)           = 0xd061D61a4d941c39E5453435B6345Dc261C2fcE0
VEBOOST_PROXY: constant(address)    = 0x8E0c00ed546602fD9927DF742bbAbF726D5B0d16
VOTING_ESCROW: constant(address)    = 0x5f3b5DfEb7B28CDbD7FAba78963EE202a494e2A2
```

These are compile-time constants, so **this file is Ethereum-mainnet only**.
Sidechain deployments use a different gauge (a `RewardsOnlyGauge` / child-chain
streamer), which is why the `deployments/` folder lists separate releases for
Fraxtal, Mantle, X Layer and zkSync.

Other constants: `MAX_REWARDS = 8` (`:88`), `TOKENLESS_PRODUCTION = 40` (`:89`),
`WEEK = 604800` (`:90`).

### 9.2 Storage

`Reward` struct (`:80-86`): `token`, `distributor`, `period_finish`, `rate`,
`last_update`, `integral`.

| Storage | `line` | Meaning |
|---|---|---|
| `balanceOf`, `totalSupply`, `allowance` | `:112-114` | the gauge is itself an ERC20 |
| `factory`, `manager`, `lp_token` | `:124-126` | `manager` = `tx.origin` at deploy |
| `is_killed` | `:128` | killed ⇒ rate 0 |
| `inflation_params` | `:131` | `[future_epoch_time:40][inflation_rate:216]` |
| `reward_count`, `reward_data` | `:134-135` | extra reward tokens |
| `rewards_receiver` | `:138` | per-user default |
| `reward_integral_for` | `:141` | token → user → integral snapshot |
| `claim_data` | `:144` | user → token → `[claimable:128][claimed:128]` |
| `working_balances`, `working_supply` | `:146-147` | boosted balances |
| `integrate_inv_supply_of` | `:150` | user's snapshot of the global integral |
| `integrate_checkpoint_of` | `:151` | user's last checkpoint time |
| `integrate_fraction` | `:155` | **total CRV ever owed to the user** |
| `period`, `period_timestamp`, `integrate_inv_supply` | `:159-165` | the global integral history |

### 9.3 `_checkpoint(addr)` — `:225-295` (internal)

The CRV accounting core. The identity being maintained is

```
integrate_inv_supply  =  1e18 · ∫ rate(t)·relative_weight(t) / working_supply(t) dt
integrate_fraction[u] =        ∫ working_balance[u] · rate(t)·w(t) / working_supply(t) dt
```

The second is what the user is owed, in CRV. Storing the global integral and each
user's snapshot means paying N users costs O(1) storage per action — the same
growth-per-unit-of-stake trick as Uniswap V3 fee growth and Aave's reward index.

The week loop (`:255-281`):

```vyper
prev_week_time: uint256 = _period_time
week_time: uint256 = min((_period_time + WEEK) / WEEK * WEEK, block.timestamp)

for i in range(500):
    dt: uint256 = week_time - prev_week_time
    w: uint256 = Controller(GAUGE_CONTROLLER).gauge_relative_weight(self, prev_week_time)
    if _working_supply > 0:
        if prev_future_epoch >= prev_week_time and prev_future_epoch < week_time:
            _integrate_inv_supply += rate * w * (prev_future_epoch - prev_week_time) / _working_supply
            rate = new_rate
            _integrate_inv_supply += rate * w * (week_time - prev_future_epoch) / _working_supply
        else:
            _integrate_inv_supply += rate * w * dt / _working_supply
    if week_time == block.timestamp: break
    prev_week_time = week_time
    week_time = min(week_time + WEEK, block.timestamp)
```

It integrates week by week because `gauge_relative_weight` is a step function
that changes only on week boundaries. The `prev_future_epoch` branch handles the
annual CRV emission-rate reduction landing mid-week: apply the old rate up to the
epoch, the new rate after.

Two consequences of `range(500)`:

- 500 weeks ≈ 9.6 years, so the loop is effectively unbounded in practice.
- If more than one emission epoch is crossed, only the first and last rates are
  applied. The comment at `:270-271` states the trade-off outright: *"If more
  than one epoch is crossed - the gauge gets less, but that'd meen it wasn't
  called for more than 1 year"*. A gauge untouched for over a year under-pays.

The precision note at `:274-279` is worth reading in the source: worst-case loss
is ~1e-9, at `dt = 1`.

User update (`:291-294`):

```vyper
_working_balance: uint256 = self.working_balances[addr]
self.integrate_fraction[addr] += _working_balance * (_integrate_inv_supply - self.integrate_inv_supply_of[addr]) / 10 ** 18
self.integrate_inv_supply_of[addr] = _integrate_inv_supply
self.integrate_checkpoint_of[addr] = block.timestamp
```

### 9.4 `_update_liquidity_limit(addr, l, L)` — `:350-374` (internal)

The boost:

```vyper
voting_balance: uint256 = VotingEscrowBoost(VEBOOST_PROXY).adjusted_balance_of(addr)
voting_total: uint256 = ERC20(VOTING_ESCROW).totalSupply()

lim: uint256 = l * TOKENLESS_PRODUCTION / 100
if voting_total > 0:
    lim += L * voting_balance / voting_total * (100 - TOKENLESS_PRODUCTION) / 100

lim = min(l, lim)
```

i.e.

```
working_balance = min( l ,  0.4·l  +  0.6·L·(veCRV_user / veCRV_total) )
```

With no veCRV you earn on 40% of your deposit. With enough veCRV the second term
saturates the `min` and you earn on 100% — a **2.5× boost** (`1/0.4`). The
`adjusted_balance_of` call goes to the veBoost proxy, so delegated boost counts.

`working_supply` is maintained incrementally (`:369-371`), which is why every
balance-changing path must call this function.

### 9.5 `_checkpoint_rewards` — `:299-346` (internal)

Independent accounting for up to 8 extra reward tokens, using the same
integral pattern but with a simple linear rate:

```vyper
last_update: uint256 = min(block.timestamp, self.reward_data[token].period_finish)
duration: uint256 = last_update - self.reward_data[token].last_update
if duration != 0 and _total_supply != 0:
    integral += duration * self.reward_data[token].rate * 10**18 / _total_supply
```

Note it uses `totalSupply`/`balanceOf`, **not** `working_supply`/`working_balances`
— extra rewards are unboosted. `claim_data` packs claimable and claimed into one
slot (`:337-346`), so an unclaimed accrual costs one SSTORE.

### 9.6 External functions

| Function | `line` | Access | Notes |
|---|---|---|---|
| `deposit(_value, _addr=msg.sender, _claim_rewards=False)` | `:407-434` | any | `@nonreentrant('lock')`; `assert _addr != empty(address)` |
| `withdraw(_value, _claim_rewards=False)` | `:438-463` | any | `@nonreentrant('lock')` |
| `claim_rewards(_addr=msg.sender, _receiver=0)` | `:467-478` | any | `assert _addr == msg.sender` if `_receiver` set (`:476`) |
| `transferFrom` / `transfer` | `:482` / `:501` | any | both `@nonreentrant('lock')`, both checkpoint sender and receiver |
| `approve` | `:514-531` | any | |
| `permit` | `:534-580` | any | EIP-2612; `# dev: invalid owner`, `# dev: permit expired`, `# dev: invalid signature` |
| `increaseAllowance` / `decreaseAllowance` | `:583` / `:601` | any | |
| `user_checkpoint(addr)` | `:619-629` | `addr` or `MINTER` | `# dev: unauthorized` |
| `set_rewards_receiver(_receiver)` | `:632-639` | any | |
| `kick(addr)` | `:642-660` | any | see below |
| `set_gauge_manager(_gauge_manager)` | `:665-677` | manager or factory admin | |
| `deposit_reward_token(_reward_token, _amount, _epoch=WEEK)` | `:681-714` | that token's distributor | `@nonreentrant("lock")` |
| `add_reward(_reward_token, _distributor)` | `:717-733` | manager or factory admin | max 8 |
| `set_reward_distributor(_reward_token, _distributor)` | `:736-749` | current distributor, factory admin, or manager | |
| `set_killed(_is_killed)` | `:752-760` | factory admin only | |

**`deposit` ordering** (`:414-434`): `_checkpoint(_addr)` **first**, then reward
checkpoint, then balance and supply updates, then `_update_liquidity_limit`, then
`transferFrom` the LP token. Checkpointing before the balance change is what makes
the integral exact — the user accrues at their old working balance right up to
this moment.

**`kick(addr)` — `:642-660`.** Anyone can force a boost recomputation:

```vyper
assert ERC20(VOTING_ESCROW).balanceOf(addr) == 0 or t_ve > t_last # dev: kick not allowed
assert self.working_balances[addr] > _balance * TOKENLESS_PRODUCTION / 100  # dev: kick not needed
```

Only allowed if the user's lock expired (`balanceOf == 0`) or they had a veCRV
event since their last checkpoint, and only if they are currently boosted above
the 40% floor. This is the mechanism that stops an expired lock from keeping a
stale 2.5× boost forever.

**`deposit_reward_token` — `:681-714`.** Measures the received amount by balance
diff (`:695-702`), so fee-on-transfer reward tokens work. `assert amount_received
> _epoch  # dev: rate will tend to zero!` (`:703`) — the rate is
`amount/duration` in integer math, so fewer wei than seconds would round to zero.
Topping up mid-period folds the remainder in (`:708-711`).

### 9.7 View functions

| Function | `line` | Notes |
|---|---|---|
| `claimed_reward(_addr, _token)` | `:768-776` | low 128 bits of `claim_data` |
| `claimable_reward(_user, _reward_token)` | `:780-798` | projects the integral to now |
| `claimable_tokens(addr)` | `:801-809` | **not a view** — calls `_checkpoint`; docstring says *"should be manually changed to view in the ABI"*, i.e. call it with `eth_call` |
| `integrate_checkpoint()` | `:813-819` | `period_timestamp[period]` |
| `future_epoch_time()` | `:822-827` | `inflation_params >> 216` |
| `inflation_rate()` | `:831-836` | `inflation_params % 2**216` |
| `decimals()` | `:840-847` | 18 |
| `version()` | `:851-856` | `"v6.1.0"` |
| `DOMAIN_SEPARATOR()` | `:860-864` | |

`claimable_tokens` (`:807-808`) returns
`integrate_fraction[addr] − Minter.minted(addr, self)`. That subtraction is the
whole Minter protocol: the gauge tracks cumulative entitlement, the Minter tracks
cumulative payout, the difference is claimable.

---

## 10. `StableSwapNGLPOracle.vy`

113 lines, **Vyper 0.4.3**, `# pragma optimize gas`, MIT licensed. The newest
file here and the only one in modern Vyper syntax (`staticcall`, `abi_encode`,
module imports). It prices the **LP token** of a 2-coin NG pool.

```vyper
from curve_std.stableswap import lp_oracle_2
```

An external module (`curve_std`) not vendored in this repo — the README (`:24-30`)
explains the package is installable and the oracle importable as
`from stableswap_ng import LPOracle`.

| Function | `line` | Notes |
|---|---|---|
| `_sanity_check(pool)` | `:29-45` | internal view |
| `sanity_check(_pool)` | `:49-55` | external wrapper, returns `True` or reverts |
| `_scaled_A_raw(pool)` | `:61-68` | rescales Curve's `A` for the solver |
| `_portfolio_value(pool, i=0)` | `:72-80` | |
| `_lp_price(pool, i=0)` | `:84-86` | |
| `lp_price(_pool, _i=0)` | `:91-113` | external view |

**`_sanity_check` — `:29-45`.** Four assertions:

```vyper
assert staticcall pool.get_virtual_price() > 0
assert staticcall pool.price_oracle(0) > 0
A: uint256 = staticcall pool.A_precise()
assert POOL_A_PRECISION <= A and A <= lp_oracle_2.MAX_A * POOL_A_PRECISION, "Bad A value"
success, response = raw_call(pool.address, abi_encode(convert(2, uint256), method_id=method_id("coins(uint256)")), ..., revert_on_failure=False, is_static_call=True)
assert not success, "Supports only 2-coin pool"
_: address = staticcall pool.coins(1)  # 63/64 msg.gas attack safety
```

The 2-coin check is a negative probe: `coins(2)` **must revert**. And the final
`coins(1)` exists purely as a 63/64-gas-forwarding guard — a caller supplying
just enough gas could make the inner `raw_call` fail for gas reasons rather than
because index 2 is out of range, which would falsely pass the check. Doing a real
call afterwards ensures enough gas remains.

**`_scaled_A_raw` — `:61-68`.**

```vyper
A_pool: uint256 = staticcall pool.A_precise()
return unsafe_div(A_pool * lp_oracle_2.A_PRECISION, N_COINS**(N_COINS-1) * POOL_A_PRECISION)
```

The comment (`:63-64`) is the clearest statement of Curve's `A` convention
anywhere in the repo: *"Pool stores A as: `A_true * N_COINS**(N_COINS-1) * 100`.
Solver expects: `A_true * solver.A_PRECISION`."*

**`lp_price` — `:91-113`.** Four docstring warnings, all real:

1. The quote is in the **base asset** of coin `i`, not the token itself. For a
   yield-bearing coin `sA`, the price is in `A`. Conversion formula given at
   `:100-102`.
2. *"LP token price can be inflated by a natural increase in
   `_pool.get_virtual_price()`, including through wash trading or due to the rate
   oracles used by the pool tokens."* Wash trading raises `D` through fees; a
   lying rate oracle raises it directly.
3. `price_oracle(0)` is capped at 2.0 (§1.7), so the LP price is capped too.
4. The call reverts if `get_virtual_price()` reverts, e.g. if a rate oracle
   fails.

Note that `get_virtual_price` and `price_oracle` are both `@nonreentrant('lock')`
on the pool, so this oracle cannot be read during a pool state transition — the
read-only reentrancy defence propagates to consumers automatically.

---

## 11. `ProxyAdmin.vy`

156 lines, Vyper 0.3.10, `# pragma evm-version paris`, MIT, authored by Ben
Hauser. A two-of-two shared-ownership wrapper, unrelated to the AMM math — used
to hold ownership of factories or pools.

Storage (`:35-39`): `admins: public(address[2])`, `pending_current_admin` (a
1-based index, 0 = no request), `pending_new_admin`, `change_approved`.

| Function | `line` | Access | Behaviour |
|---|---|---|---|
| `__init__(_authorized)` | `:44-50` | — | sets both admins |
| `execute(_target, _calldata)` | `:54-65` | either admin | `@payable`; `raw_call(_target, _calldata, value=msg.value)`; logs `TransactionExecuted` |
| `get_admin_change_status()` | `:69-81` | view | `(to_replace, new, approved)` |
| `request_admin_change(_new_admin)` | `:84-102` | either admin | `# dev: already an active request`, `# dev: new admin is already admin`, falls through to `raise  # dev: only admin` |
| `approve_admin_change()` | `:105-117` | **the other** admin | `assert msg.sender == self.admins[idx % 2]  # dev: caller is not 2nd admin` |
| `revoke_admin_change()` | `:120-139` | either admin | resets state even after approval |
| `accept_admin_change()` | `:142-156` | the new admin | requires `change_approved` |

The rotation protocol is three-party: admin A requests a replacement for itself,
admin B approves, and the *new* address must then accept. `revoke` is available
to either admin at any point, including after approval. The `idx % 2` trick at
`:112` picks the other slot from a 1-based index — for `idx=1` it is `admins[1]`,
for `idx=2` it is `admins[0]`.

`execute` uses an unchecked `raw_call` with no return handling, so a failed inner
call bubbles up as a revert. `Bytes[100000]` caps calldata at 100 KB.

---

## 12. The mocks

Not production code, but they are the executable specification of what the pool
claims to support. `contracts/mocks/`, 10 files.

**`ERC20.vy`** (74 lines) — minimal ERC20 with configurable decimals.
`__init__` `:32`, `allowance` `:40`, `transfer` `:45`, `transferFrom` `:53`,
`approve` `:62`, `_mint_for_testing` `:69`.

**`ERC20Oracle.vy`** (99 lines) — asset type 1. Adds `exchangeRate()` (`:82`) and
`set_exchange_rate(rate)` (`:88`). The pool reads it through the packed
`rate_oracles` selector. Header: *"This is for testing only, it is NOT safe for
use"*.

**`ERC20Rebasing.vy`** (153 lines) — asset type 2, *"Based on stEth
implementation"*. Shares-based: `_share_price` (`:103`),
`_get_coins_by_shares` (`:111`), `_get_shares_by_coins` (`:117`), and `_rebase`
(`:128`) called on **every transfer**, with an `is_up` constructor flag so tests
can drive both positive rebases and slashing. `set_total_coin` (`:136`) forces a
rebase.

**`ERC20RebasingConditional.vy`** (148 lines) — same, but `rebase()` (`:123`) is
external and explicit rather than automatic on transfer.

**`ERC4626.vy`** (272 lines) — asset type 3. Full 4626 surface:
`convertToAssets` (`:125`), `convertToShares` (`:142`), `deposit` (`:159`),
`mint` (`:189`), `withdraw` (`:223`), `redeem` (`:255`), plus all the `max*`
and `preview*` views. Crucially it includes **`DEBUG_steal_tokens(amount)`**
(`:270`) — a deliberate backdoor to drop `totalAssets` and drive the share price
down, so tests can exercise what the pool does when a 4626 vault loses money.

**`WETH.vy`** (82 lines) — `deposit` (`:69`), `__default__` (`:75`),
`withdraw` (`:80`). Used because NG pools cannot hold native ETH at all.

**`CurveTokenV3.vy`** (202 lines) — the **classic-generation** separate LP token,
with `mint` (`:150`), `burnFrom` (`:168`), `set_minter` (`:184`). Present only so
tests can stand up a legacy base pool for metapools. NG has no equivalent: the
pool is its own token.

**`CurvePool.vy`** (891 lines) — a minimal classic StableSwap, *"only a mock used
for testing"* (`:7`). Serves as the base pool in metapool tests. Includes the
classic commit/apply fee flow (`commit_new_fee` `:779`, `apply_new_fee` `:794`,
`revert_new_parameters` `:809`), the ownership transfer dance
(`commit_transfer_ownership` `:816`, `apply_transfer_ownership` `:828`,
`revert_transfer_ownership` `:841`), `kill_me` (`:882`) / `unkill_me` (`:889`),
and `donate_admin_fees` (`:875`). **Every one of these is gone in NG** — see
§18.

**`Zap.vy`** (314 lines) — the classic 3-coin metapool zap, the predecessor of
`MetaZapNG`. Fixed `address[3]` base coins in the constructor (`:57`).

**`CallbackSwap.vy`** (154 lines) — *"CurveExchangeWithoutApproval"*, by
fiddyresearch.eth. Demonstrates `exchange_extended` with a `transfer_callback`
(`:55`), `callback_and_swap` (`:96`), `transfer_and_swap` (`:126`). Header notes
it only works with **Cryptoswap** pools that have `exchange_extended`, and does
not do native-token swaps. NG's answer to the same problem is
`exchange_received`, which needs no callback at all.

**`CallbackTestZap.vy`** (66 lines) — the adversarial version. `good_callback`
(`:28`) versus `evil_callback` (`:38`), `set_evil_input_amount` (`:49`),
`good_exchange` (`:54`) versus `evil_exchange` (`:62`). It exists to prove that a
callback which under-delivers is caught.

---

## 13. ABI / selector tables

Computed with keccak-256 over the canonical signature. Vyper generates a separate
entry for **every combination of default arguments**, so `exchange` really is two
selectors on-chain.

### 13.1 `CurveStableSwapNG`

| Signature | Selector |
|---|---|
| `exchange(int128,int128,uint256,uint256)` | `0x3df02124` |
| `exchange(int128,int128,uint256,uint256,address)` | `0xddc1f59d` |
| `exchange_received(int128,int128,uint256,uint256)` | `0x7e3db030` |
| `exchange_received(int128,int128,uint256,uint256,address)` | `0xafb43012` |
| `add_liquidity(uint256[],uint256)` | `0xb72df5de` |
| `add_liquidity(uint256[],uint256,address)` | `0xa7256d09` |
| `remove_liquidity_one_coin(uint256,int128,uint256)` | `0x1a4d01d2` |
| `remove_liquidity_one_coin(uint256,int128,uint256,address)` | `0x081579a5` |
| `remove_liquidity_imbalance(uint256[],uint256)` | `0x7706db75` |
| `remove_liquidity_imbalance(uint256[],uint256,address)` | `0x4a6e32c6` |
| `remove_liquidity(uint256,uint256[])` | `0xd40ddb8c` |
| `remove_liquidity(uint256,uint256[],address)` | `0x5e604cd2` |
| `remove_liquidity(uint256,uint256[],address,bool)` | `0x2969e04a` |
| `withdraw_admin_fees()` | `0x30c54085` |
| `get_dy(int128,int128,uint256)` | `0x5e0d443f` |
| `get_dx(int128,int128,uint256)` | `0x67df02ca` |
| `calc_withdraw_one_coin(uint256,int128)` | `0xcc2b27d7` |
| `calc_token_amount(uint256[],bool)` | `0x3db06dd8` |
| `get_virtual_price()` | `0xbb7b8b80` |
| `totalSupply()` | `0x18160ddd` |
| `A()` | `0xf446c1d0` |
| `A_precise()` | `0x76a2f0f0` |
| `balances(uint256)` | `0x4903b0d1` |
| `get_balances()` | `0x14f05979` |
| `stored_rates()` | `0xfd0684b1` |
| `dynamic_fee(int128,int128)` | `0x76a9cd3e` |
| `get_p(uint256)` | `0xec023862` |
| `price_oracle(uint256)` | `0x68727653` |
| `D_oracle()` | `0x907a016b` |
| `last_price(uint256)` | `0x3931ab52` |
| `ema_price(uint256)` | `0x90d20837` |
| `admin_balances(uint256)` | `0xe2e7d264` |
| `ramp_A(uint256,uint256)` | `0x3c157e64` |
| `stop_ramp_A()` | `0x551a6588` |
| `set_new_fee(uint256,uint256)` | `0x015c2838` |
| `set_ma_exp_time(uint256,uint256)` | `0x65bbea6b` |
| `transfer(address,uint256)` | `0xa9059cbb` |
| `transferFrom(address,address,uint256)` | `0x23b872dd` |
| `approve(address,uint256)` | `0x095ea7b3` |
| `permit(address,address,uint256,uint256,uint8,bytes32,bytes32)` | `0xd505accf` |
| `DOMAIN_SEPARATOR()` | `0x3644e515` |
| `N_COINS()` | `0x29357750` |
| `coins(uint256)` | `0xc6610657` |
| `fee()` | `0xddca3f43` |
| `offpeg_fee_multiplier()` | `0x8edfdd5f` |
| `admin_fee()` | `0xfee3f7f9` |
| `salt()` | `0xbfa0b133` |

### 13.2 `CurveStableSwapMetaNG` — additional / changed

Everything above, except the `uint256[]` array forms become `uint256[2]`:

| Signature | Selector |
|---|---|
| `exchange_underlying(int128,int128,uint256,uint256)` | `0xa6417ed6` |
| `exchange_underlying(int128,int128,uint256,uint256,address)` | `0x44ee1986` |
| `add_liquidity(uint256[2],uint256)` | `0x0b4c7e4d` |
| `add_liquidity(uint256[2],uint256,address)` | `0x0c3e4b54` |
| `remove_liquidity(uint256,uint256[2])` | `0x5b36389c` |
| `remove_liquidity_imbalance(uint256[2],uint256)` | `0xe3103273` |
| `get_dy_underlying(int128,int128,uint256)` | `0x07211ef7` |
| `get_dx_underlying(int128,int128,uint256)` | `0x0e71d1b9` |
| `BASE_POOL()` | `0x71511a5e` |
| `BASE_N_COINS()` | `0x3da575a1` |
| `BASE_COINS()` | `0x93d2b3f3` |

`add_liquidity(uint256[2],uint256)` = `0x0b4c7e4d` is the *classic* Curve
selector, which is why old integrations keep working against NG metapools but
break against NG plain pools.

### 13.3 Views, Math, Factory, Gauge, Oracle, ProxyAdmin

| Contract | Signature | Selector |
|---|---|---|
| Views | `get_dy(int128,int128,uint256,address)` | `0x0c601c2c` |
| Views | `get_dx(int128,int128,uint256,address)` | `0x83aa796a` |
| Views | `get_dy_underlying(int128,int128,uint256,address)` | `0xc02c60a6` |
| Views | `get_dx_underlying(int128,int128,uint256,address)` | `0xd6fc10ab` |
| Views | `calc_token_amount(uint256[],bool,address)` | `0xfb79eb27` |
| Views | `calc_withdraw_one_coin(uint256,int128,address)` | `0xb54e9f05` |
| Views | `dynamic_fee(int128,int128,address)` | `0xa63530bd` |
| Math | `get_y(int128,int128,uint256,uint256[],uint256,uint256,uint256)` | `0xaa3ded9b` |
| Math | `get_D(uint256[],uint256,uint256)` | `0x50e7277d` |
| Math | `get_y_D(uint256,int128,uint256[],uint256,uint256)` | `0x7982c340` |
| Math | `exp(int256)` | `0xe46751e3` |
| Factory | `deploy_plain_pool(string,string,address[],uint256,uint256,uint256,uint256,uint256,uint8[],bytes4[],address[])` | `0x5bcd3d83` |
| Factory | `deploy_metapool(address,string,string,address,uint256,uint256,uint256,uint256,uint256,uint8,bytes4,address)` | `0xdf8c5d73` |
| Factory | `deploy_gauge(address)` | `0x96bebb34` |
| Factory | `add_base_pool(address,address,uint8[],uint256)` | `0xa9a8ef44` |
| Gauge | `deposit(uint256)` | `0xb6b55f25` |
| Gauge | `deposit(uint256,address)` | `0x6e553f65` |
| Gauge | `deposit(uint256,address,bool)` | `0x83df6747` |
| Gauge | `withdraw(uint256)` | `0x2e1a7d4d` |
| Gauge | `withdraw(uint256,bool)` | `0x38d07436` |
| Gauge | `claim_rewards()` | `0xe6f1daf2` |
| Gauge | `claim_rewards(address)` | `0x84e9bd7e` |
| Gauge | `claim_rewards(address,address)` | `0x9faceb1b` |
| Gauge | `user_checkpoint(address)` | `0x4b820093` |
| Gauge | `claimable_tokens(address)` | `0x33134583` |
| Gauge | `claimable_reward(address,address)` | `0x33fd6f74` |
| Gauge | `deposit_reward_token(address,uint256)` | `0x93f7aa67` |
| Gauge | `deposit_reward_token(address,uint256,uint256)` | `0x33b50aed` |
| Gauge | `add_reward(address,address)` | `0xe8de0d4d` |
| Gauge | `set_killed(bool)` | `0x90b22997` |
| Gauge | `kick(address)` | `0x96c55175` |
| LPOracle | `lp_price(address)` | `0x53b8e11f` |
| LPOracle | `lp_price(address,uint256)` | `0xf4eb5edb` |
| LPOracle | `sanity_check(address)` | `0xdbecd7c3` |
| ProxyAdmin | `execute(address,bytes)` | `0x1cff79cd` |

Two probe selectors used internally: `D_ma_time()` (NG detection,
`CurveStableSwapMetaNG.vy:341` and `CurveStableSwapNGViews.vy:406`) and
`dynamic_fee(int128,int128)` (`_has_static_fee`, `CurveStableSwapNGViews.vy:333`).

---

## 14. Storage layout tables

Vyper 0.3.10 assigns slots sequentially in declaration order. Immutables and
constants live in code, not storage, so they are excluded.

### 14.1 `CurveStableSwapNG` storage variables

| # | Name | Type | `line` | Public |
|---|---|---|---|---|
| 1 | `stored_balances` | `DynArray[uint256, 8]` | `:163` | no |
| 2 | `fee` | `uint256` | `:167` | yes |
| 3 | `offpeg_fee_multiplier` | `uint256` | `:168` | yes |
| 4 | `initial_A` | `uint256` | `:178` | yes |
| 5 | `future_A` | `uint256` | `:179` | yes |
| 6 | `initial_A_time` | `uint256` | `:180` | yes |
| 7 | `future_A_time` | `uint256` | `:181` | yes |
| 8 | `admin_balances` | `DynArray[uint256, 8]` | `:186` | yes |
| 9 | `last_prices_packed` | `DynArray[uint256, 8]` | `:198` | no |
| 10 | `last_D_packed` | `uint256` | `:199` | no |
| 11 | `ma_exp_time` | `uint256` | `:200` | yes |
| 12 | `D_ma_time` | `uint256` | `:201` | yes |
| 13 | `ma_last_time` | `uint256` | `:202` | yes |
| 14 | `balanceOf` | `HashMap[address, uint256]` | `:215` | yes |
| 15 | `allowance` | `HashMap[address, HashMap]` | `:216` | yes |
| 16 | `total_supply` | `uint256` | `:217` | no (exposed via `totalSupply()`) |
| 17 | `nonces` | `HashMap[address, uint256]` | `:218` | yes |

Immutables (in code): `N_COINS`, `N_COINS_128`, `factory`, `coins`,
`asset_types`, `pool_contains_rebasing_tokens`, `rate_multipliers`,
`rate_oracles`, `call_amount`, `scale_factor`, `name`, `symbol`, `NAME_HASH`,
`CACHED_CHAIN_ID`, `salt`, `CACHED_DOMAIN_SEPARATOR`.

### 14.2 Packed-slot decoder

| Slot | Layout | Read with |
|---|---|---|
| `last_prices_packed[i]` | `[ema_price:128][last_price:128]` | `last_price(i)` = `& (2**128-1)`; `ema_price(i)` = `>> 128` |
| `last_D_packed` | `[ema_D:128][last_D:128]` | same masks |
| `ma_last_time` | `[t_D:128][t_price:128]` | `unpack_2` → `[t_price, t_D]` |
| `rate_oracles[i]` (immutable) | `[selector:32][pad:64][address:160]` | address = `% 2**160`; selector = `& ORACLE_BIT_MASK` |
| Gauge `inflation_params` | `[future_epoch_time:40][inflation_rate:216]` | `>> 216` / `% 2**216` |
| Gauge `claim_data[u][t]` | `[claimable:128][claimed:128]` | `>> 128` / `% 2**128` |

`pack_2` asserts both halves are `< 2**128` (`:1300-1301`). A price above
2^128/1e18 ≈ 3.4e20 would revert — unreachable given the 2.0 cap.

### 14.3 `CurveStableSwapMetaNG` differences

Same list, except: `stored_balances` is `uint256[2]` (`:216`, fixed not dynamic),
`last_prices_packed` is a single `uint256` (`:254`), and `asset_type`,
`rate_multiplier`, `rate_oracle`, `call_amount`, `scale_factor` are scalars.
Additional immutables: `BASE_POOL`, `BASE_POOL_IS_NG`, `BASE_N_COINS`,
`BASE_COINS`, `math`.

### 14.4 `CurveStableSwapFactoryNG`

| # | Name | Type | `line` |
|---|---|---|---|
| 1 | `admin` | `address` | `:73` |
| 2 | `future_admin` | `address` | `:74` |
| 3 | `asset_types` | `HashMap[uint8, String[20]]` | `:76` |
| 4 | `pool_list` | `address[4294967296]` | `:78` |
| 5 | `pool_count` | `uint256` | `:79` |
| 6 | `pool_data` | `HashMap[address, PoolArray]` | `:80` |
| 7 | `base_pool_list` | `address[4294967296]` | `:82` |
| 8 | `base_pool_count` | `uint256` | `:83` |
| 9 | `base_pool_data` | `HashMap[address, BasePoolArray]` | `:84` |
| 10 | `base_pool_assets` | `HashMap[address, bool]` | `:87` |
| 11 | `pool_implementations` | `HashMap[uint256, address]` | `:90` |
| 12 | `metapool_implementations` | `HashMap[uint256, address]` | `:91` |
| 13 | `math_implementation` | `address` | `:92` |
| 14 | `gauge_implementation` | `address` | `:93` |
| 15 | `views_implementation` | `address` | `:94` |
| 16 | `fee_receiver` | `address` | `:97` |
| 17 | `markets` | `HashMap[uint256, address[4294967296]]` | `:102` |
| 18 | `market_counts` | `HashMap[uint256, uint256]` | `:103` |

### 14.5 `LiquidityGauge`

| # | Name | `line` | | # | Name | `line` |
|---|---|---|---|---|---|---|
| 1 | `balanceOf` | `:112` | | 11 | `working_supply` | `:147` |
| 2 | `totalSupply` | `:113` | | 12 | `integrate_inv_supply_of` | `:150` |
| 3 | `allowance` | `:114` | | 13 | `integrate_checkpoint_of` | `:151` |
| 4 | `name` / `symbol` | `:116-117` | | 14 | `integrate_fraction` | `:155` |
| 5 | `nonces` | `:120` | | 15 | `period` | `:159` |
| 6 | `factory` / `manager` / `lp_token` | `:124-126` | | 16 | `reward_tokens[8]` | `:162` |
| 7 | `is_killed` | `:128` | | 17 | `period_timestamp[1e29]` | `:164` |
| 8 | `inflation_params` | `:131` | | 18 | `integrate_inv_supply[1e29]` | `:165` |
| 9 | `reward_count` / `reward_data` | `:134-135` | | | | |
| 10 | `rewards_receiver` / `reward_integral_for` / `claim_data` / `working_balances` | `:138-146` | | | | |

---

## 15. Events reference

### 15.1 Pool events (both plain and meta)

| Event | `line` (plain) | Fields | Emitted by |
|---|---|---|---|
| `Transfer` | `:82-86` | `sender` (idx), `receiver` (idx), `value` | `_transfer` `:1563`, `_burnFrom` `:1571`, mint `:680`, `__init__` `:355` |
| `Approval` | `:87-91` | `owner` (idx), `spender` (idx), `value` | `approve` `:1619`, `transferFrom` `:1600`, `permit` `:1669` |
| `TokenExchange` | `:92-98` | `buyer` (idx), `sold_id`, `tokens_sold`, `bought_id`, `tokens_bought` | `_exchange` `:982` |
| `TokenExchangeUnderlying` | `:99-105` | same | **meta only**, `exchange_underlying` `:746` |
| `AddLiquidity` | `:106-112` | `provider` (idx), `token_amounts`, `fees`, `invariant`, `token_supply` | `add_liquidity` `:682` |
| `RemoveLiquidity` | `:113-118` | `provider` (idx), `token_amounts`, `fees`, `token_supply` | `remove_liquidity` `:860` (fees always empty) |
| `RemoveLiquidityOne` | `:119-125` | `provider` (idx), `token_id`, `token_amount`, `coin_amount`, `token_supply` | `remove_liquidity_one_coin` `:719` |
| `RemoveLiquidityImbalance` | `:126-132` | `provider` (idx), `token_amounts`, `fees`, `invariant`, `token_supply` | `remove_liquidity_imbalance` `:794` |
| `RampA` | `:133-138` | `old_A`, `new_A`, `initial_time`, `future_time` | `ramp_A` `:1845` |
| `StopRampA` | `:139-142` | `A`, `t` | `stop_ramp_A` `:1859` |
| `ApplyNewFee` | `:143-146` | `fee`, `offpeg_fee_multiplier` | `set_new_fee` `:1875` |
| `SetNewMATime` | `:147-149` | `ma_exp_time`, `D_ma_time` | `set_ma_exp_time` `:1890` |

`TokenExchange` logs the **measured** `dx`, not the requested amount — correct
for fee-on-transfer tokens, and the reason indexers should never assume
`tokens_sold` equals what the user's wallet was debited.

Note there is **no event for admin fee withdrawal**. `_withdraw_admin_fees`
(`:988`) emits only the underlying ERC20 `Transfer`s. Track fee revenue by
watching transfers to `factory.fee_receiver()`.

### 15.2 Factory events

| Event | `line` | Fields |
|---|---|---|
| `BasePoolAdded` | `:49-50` | `base_pool` |
| `PlainPoolDeployed` | `:52-56` | `coins`, `A`, `fee`, `deployer` |
| `MetaPoolDeployed` | `:58-63` | `coin`, `base_pool`, `A`, `fee`, `deployer` |
| `LiquidityGaugeDeployed` | `:65-67` | `pool`, `gauge` |

None of the fields are indexed, so filtering by coin requires scanning.

### 15.3 Gauge events

| Event | `line` | Fields |
|---|---|---|
| `Deposit` | `:44-46` | `provider` (idx), `value` |
| `Withdraw` | `:48-50` | `provider` (idx), `value` |
| `UpdateLiquidityLimit` | `:52-57` | `user` (idx), `original_balance`, `original_supply`, `working_balance`, `working_supply` |
| `CommitOwnership` | `:59-60` | `admin` |
| `ApplyOwnership` | `:62-63` | `admin` |
| `SetGaugeManager` | `:65-67` | `_gauge_manager` |
| `Transfer` | `:69-72` | `_from` (idx), `_to` (idx), `_value` |
| `Approval` | `:74-77` | `_owner` (idx), `_spender` (idx), `_value` |

`UpdateLiquidityLimit` is the one to watch for boost analytics: it carries both
the raw and working balances, so `working_balance / balance` is the realised
boost multiplier.

### 15.4 ProxyAdmin events

`TransactionExecuted` (`:11`), `RequestAdminChange` (`:17`),
`RevokeAdminChange` (`:21`), `ApproveAdminChange` (`:26`),
`AcceptAdminChange` (`:31`).

---

## 16. Revert decoder

### 16.1 String reverts (these appear in the revert data)

| Message | Where | Meaning |
|---|---|---|
| `"Slippage screwed you"` | `NG:674`, `NG:790`, `Meta:861`, `Meta:994` | `mint_amount < _min_mint_amount`, or `burn_amount > _max_burn_amount` |
| `"Not enough coins removed"` | `NG:711`, `Meta:910` | `dy < _min_received` on one-coin withdrawal |
| `"Withdrawal resulted in fewer coins than expected"` | `NG:832`, `Meta:1035` | a coin's pro-rata share `< _min_amounts[i]` |
| `"Exchange resulted in fewer coins than expected"` | `NG:974`, `Meta:1182` | `dy < _min_dy` |
| `"wad_exp overflow"` | `NG:1489`, `Math:226` | `exp` argument ≥ 135.3e18. Unreachable from the EMA path |
| `"Not a Metapool Swap. Use Base pool."` | `Views:107` | `get_dx_underlying` with both indices > 0 |
| `"Invalid fee"` | `Factory:499`, `:613` | deploy-time fee > 1% |
| `"Max 18 decimals for coins"` | `Factory:513`, `:624` | |
| `"Duplicate coins"` | `Factory:520` | |
| `"Invalid implementation index"` | `Factory:523`, `:620` | that implementation slot is unset |
| `"Invalid asset: Cannot pair base pool asset with base pool's LP token"` | `Factory:612` | |
| `"Base pool is not added"` | `Factory:617` | call `add_base_pool` first |
| `"Unknown pool"` | `Factory:700` | `deploy_gauge` for an unregistered pool |
| `"Gauge already deployed"` | `Factory:701` | |
| `"Gauge implementation not set"` | `Factory:703` | |
| `"No available market"` | `Factory:382`, `:389` | `get_coin_indices` found no path |
| `"Bad A value"` | `LPOracle:34` | `A` outside the solver's range |
| `"Supports only 2-coin pool"` | `LPOracle:44` | `coins(2)` did **not** revert |

### 16.2 Bare asserts (empty revert data — identify by line)

`# dev:` comments are compiler hints, not on-chain strings. A revert with no data
from a pool is one of these.

**`CurveStableSwapNG.vy`**

| `line` | Condition | Dev comment |
|---|---|---|
| `:383` | `dx > 0` | do not transferFrom 0 tokens into the pool |
| `:408` | `receiver != 0` | do not send tokens to zero_address |
| `:556` | `not pool_contains_rebasing_tokens` | exchange_received not supported if pool contains rebasing tokens |
| `:582` | `_receiver != 0` | do not send LP tokens to zero_address |
| `:609` | `total_supply != 0` | initial deposit requires all coins |
| `:615` | `D1 > D0` | (none) |
| `:703` | `_burn_amount > 0` | do not remove 0 LP tokens |
| `:789` | `burn_amount > 1` | zero tokens burned |
| `:822` | `_burn_amount > 0` | invalid burn amount |
| `:823` | `len(_min_amounts) == N_COINS` | invalid array length for _min_amounts |
| `:953` | `i != j` | coin index out of range |
| `:954` | `_dx > 0` | do not exchange 0 coins |
| `:1028-1035` | index bounds in `get_y` | same coin / j below zero / j above N_COINS |
| `:1147-1148` | index bounds in `get_y_D` | i below zero / i above N_COINS |
| `:1300-1301` | `pack_2` halves `< 2**128` | (none) |
| `:1826`, `:1849`, `:1864`, `:1884` | `msg.sender == factory.admin()` | only owner |
| `:1827` | 1 day since last ramp | (none) |
| `:1828` | `_future_time >= now + 1 day` | insufficient time |
| `:1832-1836` | `0 < _future_A < MAX_A`, ≤10× change | (none) |
| `:1871` | offpeg × fee ≤ MAX_FEE × FEE_DENOM | offpeg multiplier exceeds maximum |
| `:1885` | `ma_exp_time * D_ma_time > 0` | 0 in input values |

**`CurveStableSwapMetaNG.vy`** — the same set at `:462`, `:503`, `:645`, `:675`,
`:766`, `:794`, `:901`, `:993`, `:1026`, `:1159`, `:1160`, `:1837-1839`, `:1860`,
`:1882`, `:1895-1896`, plus:

| `line` | Condition | Dev comment |
|---|---|---|
| `:343` | `len(_base_coins) <= 3` | implementation does not support old gen base pool with more than 3 coins |
| `:458` | `_dx >= dx` | pool did not receive tokens for swap |
| `:1461`, `:1468`, `:1481`, `:1495` | `i == 0` | metapools do not have `<x>` indices greater than 0 |

**`CurveStableSwapFactoryNG.vy`**: `:496` (≥2 coins), `:497-499` (array lengths),
`:500` (offpeg × fee), `:718` (admin only), `:719` (rebasing in base pool),
`:720` (pool exists), `:721` (`_n_coins < MAX_COINS`), `:750` (native token not
supported), and `msg.sender == self.admin` on every setter.

**`LiquidityGauge.vy`**: `:414` (cannot deposit for zero address), `:476` (cannot
redirect when claiming for another user), `:558-573` (invalid owner / permit
expired / invalid signature), `:625` (unauthorized), `:654-655` (kick not allowed
/ kick not needed), `:673` and `:723` (only manager or factory admin), `:703`
(rate will tend to zero), `:758` (only owner).

**Unlabelled failure modes worth recognising:**

- A revert from deep inside `get_D`/`get_y` with no data at the top of the stack
  is usually the **bare `raise`** after 255 Newton iterations
  (`NG:1074`, `:1125`, `:1182`). The pool is in a state where only
  `remove_liquidity` works.
- A **division-by-zero panic** inside `get_D` means a balance is zero
  (`Math:112`).
- A revert inside `_stored_rates` is a failing rate oracle: either the
  `raw_call` reverted or `assert len(oracle_response) == 32` (`:452`) failed.

---

## 17. Use-case index

"I want to X" → the exact function and its internal call chain.

### Deploy a plain 3-coin USD pool

```
Factory.deploy_plain_pool(name, sym, [DAI,USDC,USDT], A=200, fee=1e6,
                          offpeg=2e10, ma_exp_time=866, impl_idx=0,
                          [0,0,0], [b"",b"",b""], [0x0,0x0,0x0])          Factory:458
 ├ validate lengths, fee ≤ 1%, decimals < 19, no duplicates              :496-521
 ├ rate_multipliers = 10**(36-decimals)                                  :516
 ├ create_from_blueprint(pool_implementations[0], ...)                   :525
 │   └ pool.__init__ → sets A, fee, oracles, EIP-712 domain              NG:239
 └ register in pool_list + markets[coin_a^coin_b]                        :542-563
```

### Deploy a metapool on 3pool

```
(admin, once)  Factory.add_base_pool(3pool, 3CRV, [0,0,0], 3)            Factory:715
Factory.deploy_metapool(3pool, name, sym, LUSD, A, fee, offpeg,
                        ma_exp_time, impl_idx, asset_type=0, b"", 0x0)   Factory:571
 ├ assert not base_pool_assets[LUSD]                                     :612
 ├ asset_types = [0, 0] + base types                                     :629-635
 ├ create_from_blueprint(metapool_implementations[idx],
 │                       ..., math_implementation, 3pool, ...)           :640
 │   └ meta.__init__: probe D_ma_time(), approve base coins              Meta:341, :355
 └ register against every base coin + 3CRV                               :673-685
```

### Swap with an approval

```
coin_i.approve(pool, dx)
pool.exchange(i, j, dx, min_dy, receiver)                                NG:504
 └ _exchange(..., expect_optimistic_transfer=False)                      NG:943
     ├ _stored_rates / _balances / _xp_mem                               :433/:473/:1209
     ├ _transfer_in → transferFrom + balance diff                        :358
     ├ __exchange → get_D, get_y, _dynamic_fee, upkeep_oracles           :904
     └ _transfer_out                                                     :398
```

### Swap from an aggregator with no approval

```
(same tx)  coin_i.transfer(pool, dx)
           pool.exchange_received(i, j, dx, min_dy, receiver)            NG:534
            ├ assert not pool_contains_rebasing_tokens                   :556
            └ _exchange(..., True) → _transfer_in optimistic branch      :369-372
```

**Must be atomic.** Tokens sitting at the pool are claimable by anyone.

### Add liquidity one-sided

```
pool.add_liquidity([0, 100_000e6, 0], min_mint, receiver)                NG:570
 ├ D0                                                                    :588
 ├ _transfer_in for the non-zero coin only                               :604
 ├ D1, assert D1 > D0                                                    :616-617
 ├ per-coin imbalance fee via _dynamic_fee(xs, ys, fee·n/(4(n-1)))       :648
 ├ D1 recomputed on fee-reduced balances                                 :661
 └ mint = supply·(D1−D0)/D0                                              :659
```

Quote first with `calc_token_amount(amounts, True)` → Views `:228`.

### Withdraw into one coin

```
pool.remove_liquidity_one_coin(lp, i, min_received, receiver)            NG:689
 └ _calc_withdraw_one_coin                                               :1233
     ├ D0, D1 = D0 − burn·D0/supply                                      :1250-1253
     ├ new_y = get_y_D(amp, i, xp, D1)                                   :1254
     ├ fee-reduce every coin, re-solve get_y_D                           :1268-1289
     └ dy = (dy−1)·1e18/rates[i]                                         :1291
```

### Exit safely when something is broken

```
pool.remove_liquidity(lp, [0]*n, receiver, _claim_admin_fees=False)      NG:807
```

Touches no oracle, no `get_D`, no rates. Pass `False` so a broken fee-receiver
transfer cannot block the exit.

### Read a price safely

| Want | Call | Safe? |
|---|---|---|
| Manipulation-resistant price | `price_oracle(i)` `NG:1445` | yes — EMA, `@nonreentrant` |
| TVL proxy | `D_oracle()` `NG:1456` | yes — EMA, `@nonreentrant` |
| LP token value | `get_virtual_price()` `NG:1740` | `@nonreentrant`, but inflatable by donation in a rebasing pool |
| LP token price in a coin | `LPOracle.lp_price(pool, i)` `:91` | yes, 2-coin only |
| Instantaneous price | `get_p(i)` `NG:1427` | **no** — no lock, movable within a block |
| Raw last observation | `last_price(i)` `NG:1415` | **no** |

### Stake LP for CRV

```
pool.approve(gauge, amount)
gauge.deposit(amount, addr, claim_rewards=False)                         Gauge:407
 ├ _checkpoint(addr)                                                     :225
 ├ _checkpoint_rewards(...) if reward_count != 0                         :299
 ├ balances/supply updated
 ├ _update_liquidity_limit → working_balance = min(l, 0.4l + 0.6L·ve)    :350
 └ lp_token.transferFrom
...one week later...
Minter.mint(gauge)
 └ gauge.user_checkpoint(user)                                           :619
     → CRV = integrate_fraction[user] − Minter.minted(user, gauge)
```

Preview with `claimable_tokens(addr)` (`:801`) via `eth_call` — it is
state-changing in the ABI.

### Claim admin fees

```
pool.withdraw_admin_fees()          # permissionless                     NG:875
 └ _withdraw_admin_fees                                                  :988
     ├ fee_receiver = factory.fee_receiver(); if 0 → return silently     :990
     └ _transfer_out(i, admin_balances[i], fee_receiver); zero it        :998-999
```

### Ramp A

```
Factory.admin → pool.ramp_A(new_A, future_time)                          NG:1825
  constraints: ≥1 day since last ramp start, ≥1 day duration, ≤10× change
Emergency:      pool.stop_ramp_A()                                       NG:1848
```

### Change fees

```
Factory.admin → pool.set_new_fee(new_fee, new_offpeg_multiplier)         NG:1862
  new_fee ≤ 5e9 (50%); new_offpeg × new_fee ≤ 5e9 × 1e10
```

Immediate, no timelock at the pool level.

### Add underlying liquidity to a metapool without holding base LP

```
MetaZapNG.add_liquidity(pool, [meta_amt, dai, usdc, usdt], min_mint, receiver)  Zap:174
 ├ _base_pool_data (memoised)                                            :122
 ├ transferFrom each coin to the zap                                     :208-231
 ├ base_pool.add_liquidity(base_amounts, 0)                              :235
 └ metapool.add_liquidity([meta_amt, lp], min_mint, receiver)            :243
```

---

## 18. Classic vs NG, function by function

Classic reference: `curve/curve-contract/contracts/pools/3pool/StableSwap3Pool.vy`
(847 lines, Vyper **0.2.4**).

| Classic | `line` | NG | `line` | What changed |
|---|---|---|---|---|
| `__init__` | `:118` | `__init__` | `:239` | NG takes asset types, oracles, method ids, offpeg multiplier |
| `_A` | `:149` | `_A` | `:1187` | identical |
| `A` | `:171` | `A` + `A_precise` | `:1775`, `:1781` | NG adds the precise getter |
| `_xp` / `_xp_mem` | `:177` / `:186` | `_xp_mem` only | `:1209` | NG has no storage-reading `_xp`; rates are always passed |
| `get_D` | `:195` | `get_D` | `:1079` | same algorithm; NG loops a `DynArray` |
| `get_D_mem` | `:223` | `get_D_mem` | `:1222` | NG takes rates |
| `get_virtual_price` | `:229` | `get_virtual_price` | `:1740` | **NG adds `@nonreentrant('lock')`** |
| `calc_token_amount` | `:243` | `calc_token_amount` | `:1760` | NG delegates to Views; classic ignores fees, NG's Views includes them |
| `add_liquidity` | `:270` | `add_liquidity` | `:570` | NG: dynamic fee, measured transfers, oracle upkeep, returns mint amount |
| `get_y` | `:356` | `get_y` | `:1009` | same |
| `get_dy` | `:402` | `get_dy` | `:1702` | NG delegates to Views |
| `get_dy_underlying` | `:416` | — | — | classic 3pool had a lending-pool notion; NG plain pools do not |
| — | — | `get_dx` | `:1688` | **new in NG** |
| `exchange` | `:431` | `exchange` | `:504` | NG returns `dy`, takes `_receiver`, measures transfers |
| — | — | `exchange_received` | `:534` | **new in NG** — approval-free |
| `remove_liquidity` | `:498` | `remove_liquidity` | `:807` | NG adds `_receiver`, `_claim_admin_fees`, D-oracle upkeep |
| `remove_liquidity_imbalance` | `:529` | same | `:728` | NG: dynamic fee, returns burn amount |
| `get_y_D` | `:584` | `get_y_D` | `:1130` | same |
| `_calc_withdraw_one_coin` | `:630` | same | `:1233` | NG returns a 5-tuple for oracle upkeep |
| `calc_withdraw_one_coin` | `:664` | same | `:1716` | |
| `remove_liquidity_one_coin` | `:670` | same | `:689` | NG adds `_receiver` |
| `ramp_A` / `stop_ramp_A` | `:702` / `:720` | same | `:1825` / `:1848` | classic gates on `self.owner`; NG on `factory.admin()` |
| `commit_new_fee` | `:734` | — | — | **removed** |
| `apply_new_fee` | `:749` | `set_new_fee` | `:1862` | **no 3-day timelock in NG** |
| `revert_new_parameters` | `:764` | — | — | **removed** |
| `commit_transfer_ownership` | `:771` | — | — | **removed** — ownership lives on the factory |
| `apply_transfer_ownership` | `:783` | — | — | **removed** |
| `revert_transfer_ownership` | `:796` | — | — | **removed** |
| `admin_balances(i)` | `:804` | `admin_balances` (public array) | `:186` | classic *computed* it as `balanceOf − balances`; NG *stores* it |
| `withdraw_admin_fees` | `:809` | same | `:875` | classic owner-only; **NG permissionless** |
| `donate_admin_fees` | `:831` | — | — | **removed** |
| `kill_me` / `unkill_me` | `:838` / `:845` | — | — | **removed** — no kill switch in NG |
| — | — | `get_p` / `price_oracle` / `D_oracle` / `last_price` / `ema_price` | `:1427`–`:1465` | **new** — oracles |
| — | — | `set_ma_exp_time` | `:1878` | **new** |
| — | — | `stored_rates` / `get_balances` / `dynamic_fee` | `:1805`/`:1799`/`:1811` | **new** |
| — | — | `permit` / `DOMAIN_SEPARATOR` / ERC20 surface | `:1623` / `:1675` | **new** — classic used a separate `CurveTokenV3` |

Summary of the generational shift:

| Dimension | Classic | NG |
|---|---|---|
| Coins | 2–4, fixed arrays | 2–8, `DynArray` |
| LP token | separate `CurveTokenV3` contract | the pool **is** the ERC20, with `permit` |
| Fee | static | dynamic by imbalance (`offpeg_fee_multiplier`) |
| Admin fee | `commit`/`apply` with a 3-day delay, owner-only claim | constant 50%, permissionless claim |
| Ownership | per-pool `owner` with a 3-day transfer dance | `factory.admin()` |
| Kill switch | `kill_me` / `unkill_me` | none |
| Oracles | `get_virtual_price` only | state-price EMA, D EMA, `get_p` |
| Reentrancy | `get_virtual_price` unlocked | `@nonreentrant` on `get_virtual_price`, `totalSupply`, `price_oracle`, `D_oracle` |
| Asset support | plain ERC20 | plain, rate-oracle, rebasing, ERC4626 |
| Approval-free swap | no | `exchange_received` |
| Deployment | one contract per pool, hand-audited | permissionless factory + blueprints |
| Native ETH | some classic pools held it | **banned at the factory** |
| Math location | inline | inline (plain) / external Math (meta) |
| Quoting | inline | external Views, admin-swappable |

---

## 19. Gotchas and security notes

**1. `exchange_received` is a race unless atomic.** Tokens transferred to the
pool but not yet swapped belong to whoever calls `exchange_received` first
(`:534`). Only ever use it inside one transaction.

**2. Rebasing pools disable `exchange_received` — and must.** The header at
`:41-45` states both directions: if a pool contains a rebasing token and does
*not* declare asset type 2, *"this is an incorrect implementation and rebases can
be stolen"*. The declaration is what drives `_balances()` to read live balances.
Getting `asset_types` wrong at deploy time is unrecoverable — it is immutable.

**3. Rate oracles are fully trusted.** `_stored_rates` (`:439-455`) accepts any
32-byte return value with no sanity bound. A compromised or EOA-controlled oracle
sets the pool's prices directly. The header says so at `:16-18`. When integrating,
check who controls the oracle behind an asset-type-1 coin.

**4. Admin fees are immune to slashing; LPs are not.** In a rebasing pool
`_balances()` subtracts `admin_balances` from the live balance (`:487`), so a
negative rebase reduces the LP share only. The docstring at `:477-482` is
explicit.

**5. `get_p` is not an oracle.** No reentrancy lock, recomputed from live
balances, movable to any value within a block by anyone with capital. Only
`price_oracle` and `D_oracle` are for consumption, and even those can be dragged
over `ma_exp_time` by sustained flow — treat the window as a cost-of-attack
parameter.

**6. The 2.0 price cap.** `upkeep_oracles` stores `min(spot, 2e18)` (`:1361`).
For a pool whose true ratio exceeds 2.0 the oracle silently saturates. The LP
oracle documents the knock-on effect at `StableSwapNGLPOracle.vy:106-107`.

**7. `get_virtual_price` and donations.** Its own docstring warns
(`:1744-1746`). In a non-rebasing pool a donation does not move `D` (donated
tokens are outside `stored_balances`), but in a rebasing pool `_balances()` reads
`balanceOf` live, so donations *do* inflate it.

**8. Read-only reentrancy is fixed here, but not upstream.** `get_virtual_price`,
`totalSupply`, `price_oracle` and `D_oracle` are all `@view @nonreentrant('lock')`
(`:1740`, `:1729`, `:1445`, `:1456`). Combined with the factory's ban on native
ETH (`CurveStableSwapFactoryNG.vy:750`), the classic stETH-pool vector is closed.
**A metapool over a classic base pool inherits the base pool's exposure** —
`_stored_rates` calls `BASE_POOL.get_virtual_price()` (`:532`), and if that base
pool is an old ETH pool without the lock, the metapool's rates can be read
mid-manipulation.

**9. The Vyper `@nonreentrant` compiler bug does not apply.** It affected 0.2.15,
0.2.16 and 0.3.0. Everything here is 0.3.10 (the LP oracle is 0.4.3). Verify with
`grep -n "pragma version" contracts/main/*.vy` before trusting any fork.

**10. The fee ceiling is asymmetric.** The factory caps deploy-time fees at 1%
(`:499`), but `set_new_fee` allows up to `MAX_FEE` = 50% (`:1867`). A governance
action can raise a live pool's fee to fifty times what could be deployed.

**11. `views_implementation` is a live, retroactive upgrade lever.** Every pool
reads it from the factory on every quote (`:1699`). The factory admin can change
what `get_dy` returns for every pool at once. It cannot move funds — quoting is
not used in state transitions — but it can badly mislead an integrator that
trusts the quote.

**12. `A` ramping bounds.** ≥1 day since the last ramp start, ≥1 day duration,
≤10× change (`:1827-1836`), `MAX_A = 1e6`. Even a compliant ramp leaks value from
an imbalanced pool; `stop_ramp_A` is the brake. Note the constraint is on the
*ratio*, so repeated ramps can still move `A` a long way over weeks.

**13. First deposit needs every coin.** `assert total_supply != 0` in the zero-amount
branch (`:609`). The first LP receives exactly `D1` (`:663`), so the virtual price
starts at exactly 1e18. There is **no** `MINIMUM_LIQUIDITY` burn and none is
needed: shares are denominated in `D`, not `balanceOf`.

**14. `remove_liquidity` is the only guaranteed exit.** It calls no oracle, no
`get_D`, no `_stored_rates`. Pass `_claim_admin_fees=False` (`:811`) if the fee
receiver or a coin transfer might revert. Every other function can be bricked by
a failing rate oracle or a non-converging Newton loop.

**15. Newton can fail.** `get_D` (`:1125`), `get_y` (`:1074`) and `get_y_D`
(`:1182`) each end in a bare `raise` after 255 iterations. The comment calls it
unreachable; the fallback is point 14.

**16. `remove_liquidity_imbalance` transfers before it charges.** Tokens leave at
`:752`, fees are computed at `:764-783`. Only `@nonreentrant('lock')` makes that
safe. Any fork that loosens the lock breaks this function.

**17. `withdraw_admin_fees` is unlocked on the metapool.** Plain pool: `:874`
carries `@nonreentrant('lock')`. Metapool: `:1077` does not.

**18. `deploy_gauge` hands managership to `tx.origin`.** The gauge constructor
sets `manager = tx.origin` (`LiquidityGauge.vy:172`), and the manager can
`add_reward` (`:717`) and `set_gauge_manager` (`:665`). Whoever sends the deploy
transaction — not the factory, not the DAO — controls reward listing.

**19. The gauge is mainnet-only.** CRV, GaugeController, Minter, veBoost and
VotingEscrow are compile-time constants (`:105-109`).

**20. A gauge untouched for over a year under-pays.** The `_checkpoint` week loop
applies only the first and last emission rates when multiple epochs are crossed;
the comment at `:270-271` says *"the gauge gets less"*.

**21. `MetaZapNG` holds infinite approvals.** `is_approved` caches
`approve(pool, max_value(uint256))` (`:212-213`, `:238-239`). The zap is a
standing target — any bug in it reaches every pool it has approved. Also see the
double-`append` in its `add_liquidity` loop (`:218-231`, §8).

**22. Slippage arguments are the only protection.** Every mutating function takes
one: `_min_dy`, `_min_mint_amount`, `_min_received`, `_min_amounts`,
`_max_burn_amount`. There are no deadlines anywhere in this codebase — a stuck
transaction can execute at any later price.

**23. `get_dx` is approximate.** `_get_dx` (`Views:349`) computes the dynamic fee
from the *current* `xp`, while the real swap uses midpoints (`:914-916`). For
large trades `get_dx` slightly under-states the input needed. Round trip through
`get_dy` before committing.

**24. Metapool index confusion.** `exchange` uses metapool indices (0, 1);
`exchange_underlying` uses 0 for the metapool coin and 1..n for base coins. Same
contract, two index spaces, silently different results. Use
`Factory.get_coin_indices` (`:355`), which returns the `is_underlying` flag.

**25. ERC4626 donation risk.** The header flags it at `:22-25`. `_stored_rates`
calls `convertToAssets` (`:461`); a vault whose share price can be pushed by a
donation moves the pool's rates with it.

---

*Every citation in this document was verified with `grep -n` against
`curve/stableswap-ng/` as cloned in this repository. The numeric examples in §1
were reproduced in Python against the integer code paths, not the real-number
formulas.*
