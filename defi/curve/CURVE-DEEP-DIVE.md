# Curve Deep Dive

Source under this folder (`.git` stripped):

| folder | what | language / compiler |
|---|---|---|
| `curve-contract/` | classic pools (2020-21). Read `contracts/pools/3pool/StableSwap3Pool.vy` (DAI/USDC/USDT, 847 lines) and `contracts/pool-templates/base/SwapTemplateBase.vy` (the template all plain pools were generated from) | Vyper 0.1.0b16 – 0.2.12 |
| `stableswap-ng/` | current generation ("NG", 2023+). `contracts/main/CurveStableSwapNG.vy` (plain pool + LP token in one contract), `CurveStableSwapMetaNG.vy`, `CurveStableSwapNGMath.vy`, `CurveStableSwapNGViews.vy`, `CurveStableSwapFactoryNG.vy`, `MetaZapNG.vy`, `LiquidityGauge.vy` (gauge v6) | Vyper 0.3.10 |
| `curve-dao-contracts/` | tokenomics: `ERC20CRV.vy`, `VotingEscrow.vy`, `GaugeController.vy`, `Minter.vy`, `FeeDistributor.vy`, `gauges/LiquidityGauge*.vy` | Vyper 0.2.4 – 0.2.7 |

All paths below are relative to `defi/curve/`. Line numbers were verified with `grep -n` against these exact files.

Reading order if you are new: §0 (why) → §1 (classic pool, the whole AMM in 850 lines) → §3 (DAO) → §2 (NG, which is §1 plus engineering) → §4–6.

---

## 0. Why StableSwap

### 0.1 The two extremes

An AMM is a function `F(x_0, …, x_{n-1}) = const` over the pool's reserves. A trade moves the reserves along the level set; the marginal price is the slope.

* **Constant sum** `Σ x_i = D`. Price is always exactly 1:1. Zero slippage, but a pool can be fully drained of one coin: anyone can buy the whole USDC balance for USDT even when USDC trades at 1.02 elsewhere. Useless once the peg wobbles.
* **Constant product** (Uniswap v2) `Π x_i = (D/n)^n`. Never drainable (price → ∞ as a reserve → 0), but for two coins that should trade at 1:1 it is wasteful: a 10 % of-pool trade costs ~9 % slippage (table below).

### 0.2 The amplified invariant

Curve's idea (Egorov 2019): take the constant-sum invariant, multiply it by an amplification `χ`, and add the constant-product invariant so the sum term dominates near balance and the product term dominates when the pool is skewed:

```
χ · D^(n-1) · Σx_i  +  Πx_i  =  χ · D^n  +  (D/n)^n
```

To make `χ` self-adjust, Curve sets `χ = A · Πx_i / (D/n)^n` — it equals `A` when the pool is balanced and decays to 0 as the pool becomes imbalanced. Substituting and simplifying gives the StableSwap invariant you will see in every docstring:

```
A · n^n · Σx_i  +  D  =  A · D · n^n  +  D^(n+1) / (n^n · Πx_i)          (1)
```

* `A → 0`: the terms with `A` vanish, leaving `D = D^(n+1)/(n^n Πx)` → `Πx = (D/n)^n` — constant product.
* `A → ∞`: divide (1) by `A n^n`: `Σx = D` — constant sum.
* `D` is "the total amount of coins the pool would hold if it were perfectly balanced". It is *the* quantity everything else is derived from: LP token value is `D / totalSupply` (`get_virtual_price`).

**A-parameter quirk you must know.** In the code `Ann = amp * N_COINS` (`curve-contract/contracts/pools/3pool/StableSwap3Pool.vy:202`, `stableswap-ng/contracts/main/CurveStableSwapNG.vy:1099`), i.e. the contract's `A` already absorbs a factor `n^(n-1)` relative to the whitepaper's `A`. The `A` you read on-chain (3pool: 2000) is the contract value. In NG the stored value is additionally multiplied by `A_PRECISION = 100` (`CurveStableSwapNG.vy:176`) so `A()` returns `_A()/100` (`:1775`) and `A_precise()` returns the raw value (`:1781`).

### 0.3 Numbers: slippage for a 2-coin pool

Computed with the exact integer algorithms below (`get_D`/`get_y`), pool = 1,000,000 / 1,000,000, trade 100,000 of coin 0 → coin 1, no fee:

| curve | coins out | price impact |
|---|---|---|
| constant sum | 100,000.00 | 0 % |
| Curve A=1 | 95,227.30 | 4.77 % |
| Curve A=10 | 99,091.73 | 0.91 % |
| Curve A=100 | 99,900.11 | 0.10 % |
| Curve A=1000 | 99,989.91 | 0.01 % |
| Uniswap v2 (xy=k) | 90,909.09 | 9.09 % |

Same trade on an already-skewed pool 1,500,000 / 500,000 (we are buying the scarce coin):

| curve | coins out | price impact |
|---|---|---|
| Curve A=10 | 83,865.66 | 16.1 % |
| Curve A=100 | 97,804.48 | 2.2 % |
| Curve A=1000 | 99,771.84 | 0.23 % |
| Uniswap v2 | 31,250.00 | 68.8 % |

Two lessons: (a) at A=100 Curve is ~90× more capital-efficient than xy=k for pegged pairs; (b) a high `A` keeps the price near 1 even when the pool is 75/25 — which is exactly what you do *not* want if one coin is actually depegging, because LPs end up holding the bad coin. That is why `A` is a governance parameter that can be ramped (§1.2) and why NG adds a *dynamic fee* that rises with imbalance (§2.2).

---

## 1. Classic pool: `StableSwap3Pool.vy`

File: `curve-contract/contracts/pools/3pool/StableSwap3Pool.vy` (all `:line` refs in this section point there unless stated). This is the live mainnet 3pool (`0xbEbc44…`). It is Vyper 0.2.4, 847 lines, and contains the entire AMM: no libraries, no delegatecall, no proxies. The LP token (3CRV) is a separate minimal ERC20 whose `mint`/`burnFrom` only the pool may call (`:7-10`).

### 1.1 Storage and constants

```vyper
N_COINS: constant(int128) = 3                                   # :76
FEE_DENOMINATOR: constant(uint256) = 10 ** 10                   # :78  fees are 1e10-scaled: 4000000 = 0.04 %
PRECISION: constant(uint256) = 10 ** 18                         # :80
PRECISION_MUL: constant(uint256[N_COINS]) = [1, 1e12, 1e12]     # :81  DAI 18d, USDC 6d, USDT 6d
RATES: constant(uint256[N_COINS]) = [1e18, 1e30, 1e30]          # :82  PRECISION_MUL * 1e18
FEE_INDEX: constant(int128) = 2                                 # :83  USDT *may* turn on a transfer fee
MAX_ADMIN_FEE = 10 * 10**9   (100 %)  MAX_FEE = 5 * 10**9 (50 %) # :85-86
MAX_A = 10**6   MAX_A_CHANGE = 10                                # :87-88
ADMIN_ACTIONS_DELAY = 3 days   MIN_RAMP_TIME = 1 day             # :90-91
```

State (`:93-114`):

| var | meaning |
|---|---|
| `coins[3]`, `balances[3]` | token addresses; **pool-accounted** balances (not `balanceOf`). The difference `balanceOf(self) - balances[i]` is the admin's accrued fee (`admin_balances`, `:804`). |
| `fee`, `admin_fee` | swap fee and the *share of the fee* that goes to the DAO (both 1e10-scaled). 3pool: fee 0.04 %, admin_fee 50 %. |
| `owner`, `token` | DAO ownership proxy; LP token |
| `initial_A / future_A / initial_A_time / future_A_time` | the A-ramp (§1.2) |
| `admin_actions_deadline`, `future_fee`, `future_admin_fee`, `transfer_ownership_deadline`, `future_owner` | 3-day timelocked param changes (`:734-799`) |
| `is_killed`, `kill_deadline` | emergency stop, only usable within 60 days of deploy (`:112-114`, `:838-846`). Killed pool: `exchange`, `add_liquidity`, imbalance/one-coin withdrawals revert; proportional `remove_liquidity` still works so LPs can always exit. |

Note `token` is *not* `public` and `balances` are pool-accounted: the pool never trusts `balanceOf` except at explicit points (USDT fee-on-transfer, admin fee sweep).

### 1.2 `_A()` — the amplification ramp (`:149-166`)

```vyper
def _A() -> uint256:
    t1: uint256 = self.future_A_time
    A1: uint256 = self.future_A
    if block.timestamp < t1:
        A0: uint256 = self.initial_A
        t0: uint256 = self.initial_A_time
        if A1 > A0:
            return A0 + (A1 - A0) * (block.timestamp - t0) / (t1 - t0)
        else:
            return A0 - (A0 - A1) * (block.timestamp - t0) / (t1 - t0)
    else:
        return A1
```

`A` is interpolated **linearly in time** between `(initial_A, initial_A_time)` and `(future_A, future_A_time)`. Why linear and slow: changing `A` changes `D` for the same balances, hence changes `get_virtual_price` and the spot price. A step change would create an instant arbitrage against LPs (see §5). `ramp_A` (`:702-717`) enforces: only owner; at least `MIN_RAMP_TIME` since the previous ramp *started*; the ramp must last ≥ 1 day; `0 < future_A < MAX_A`; and `future_A` within a factor of `MAX_A_CHANGE = 10` of the current value (`:709-711`). `stop_ramp_A` (`:720-731`) freezes `A` at its current interpolated value by setting both endpoints to it with `future_A_time = now`.

### 1.3 `_xp()` / `_xp_mem()` — normalising to 18 decimals (`:177-190`)

```vyper
def _xp() -> uint256[N_COINS]:
    result: uint256[N_COINS] = RATES
    for i in range(N_COINS):
        result[i] = result[i] * self.balances[i] / LENDING_PRECISION   # RATES[i] * bal / 1e18
    return result
```

Every math function works in "xp" units: all balances scaled to 18 decimals. `RATES[i] = 10^(36 - decimals_i)` so `xp_i = balance_i · 10^(18 - decimals_i)`. `_xp_mem` is the pure variant taking balances as an argument (used when the function already loaded `self.balances` into memory to save SLOADs). In lending/oracle pools (e.g. `pools/compound`, `pools/steth`) the rate is not constant — it includes the cToken / stETH exchange rate — so "xp" also means "priced in the underlying". That generalises into NG's `_stored_rates()` (§2.1).

### 1.4 `get_D()` — solving the invariant for D by Newton's method (`:195-219`)

Given balances `xp` and `amp`, we need `D` such that (1) holds. Rearrange (1) with `Ann = A·n^n` (code: `amp * N_COINS`, see quirk in §0.2) as a root-finding problem:

```
f(D) = D^(n+1) / (n^n · Πx)  +  (Ann - 1)·D  -  Ann·S  = 0         S = Σx
```

Let `D_P = D^(n+1) / (n^n Πx)`. Then `f'(D) = (n+1)·D_P/D + (Ann - 1)`, and one Newton step `D ← D - f/f'` simplifies to:

```
D_new = (Ann·S + n·D_P) · D  /  ((Ann - 1)·D + (n + 1)·D_P)
```

That is literally line 210:

```vyper
def get_D(xp: uint256[N_COINS], amp: uint256) -> uint256:
    S: uint256 = 0
    for _x in xp:
        S += _x
    if S == 0:
        return 0
    Dprev: uint256 = 0
    D: uint256 = S                                   # initial guess: constant-sum D
    Ann: uint256 = amp * N_COINS
    for _i in range(255):
        D_P: uint256 = D
        for _x in xp:
            D_P = D_P * D / (_x * N_COINS)           # D_P = D^(n+1) / (n^n Πx), computed without overflow
        Dprev = D
        D = (Ann * S + D_P * N_COINS) * D / ((Ann - 1) * D + (N_COINS + 1) * D_P)   # :210
        if D > Dprev:
            if D - Dprev <= 1:
                break
        else:
            if Dprev - D <= 1:
                break
    return D
```

Details worth noticing:

* `D_P` is built multiplicatively (`D_P = D_P * D / (x*n)` per coin) so intermediate values stay inside uint256 even for 8 coins with 1e26-scale balances. A zero balance divides by zero → the pool bricks for everything except proportional withdrawal (comment on `:205`).
* Initial guess `D = S` is an upper bound (constant-sum ≥ true D), and `f` is convex, so Newton descends monotonically; convergence is quadratic and in practice takes 3–8 iterations (our 3pool example below: 3).
* "Precision of 1" termination: stop when two iterates differ by ≤ 1 wei.
* `range(255)`: Vyper requires a compile-time loop bound; 255 is a generous cap, not a real expectation. The classic code *returns* whatever it has after 255 iterations; NG `raise`s instead (`CurveStableSwapNG.vy:1127`).

`get_D_mem(balances, amp)` (`:223-224`) = `get_D(_xp_mem(balances), amp)`.

### 1.5 `get_y()` — solving for one balance given the others (`:356-398`)

A swap changes `x_i` and asks: what must `x_j` become for `D` to stay unchanged? Fix every balance except `y = x_j`. Let `S' = Σ_{k≠j} x_k`, `P' = Π_{k≠j} x_k`. Equation (1) becomes

```
Ann·(S' + y) + D = Ann·D + D^(n+1) / (n^n · P' · y)
```

Multiply by `y`, divide by `Ann`, collect terms:

```
y² + (S' + D/Ann − D)·y − D^(n+1)/(n^n · P' · Ann) = 0
y² + (b − D)·y − c = 0          with  b = S' + D/Ann,   c = D^(n+1) / (n^n · P' · Ann)
```

Newton on `g(y) = y² + (b−D)y − c`, `g'(y) = 2y + b − D`, gives `y ← y − g/g'`, which simplifies to

```
y_new = (y² + c) / (2y + b − D)
```

Code:

```vyper
def get_y(i: int128, j: int128, x: uint256, xp_: uint256[N_COINS]) -> uint256:
    assert i != j ; assert j >= 0 ; assert j < N_COINS ; ...
    amp: uint256 = self._A()
    D: uint256 = self.get_D(xp_, amp)               # D is *preserved* by the swap: compute it from pre-swap balances
    c: uint256 = D
    S_: uint256 = 0
    Ann: uint256 = amp * N_COINS
    _x: uint256 = 0
    for _i in range(N_COINS):
        if _i == i:  _x = x                         # the coin being sold uses the *new* balance x
        elif _i != j:  _x = xp_[_i]
        else:  continue                             # skip j — that is the unknown
        S_ += _x
        c = c * D / (_x * N_COINS)                  # accumulates D^n / (n^(n-1) P')
    c = c * D / (Ann * N_COINS)                     # → D^(n+1) / (n^n P' Ann)
    b: uint256 = S_ + D / Ann                       # "- D" is applied inside the loop
    y_prev: uint256 = 0
    y: uint256 = D
    for _i in range(255):
        y_prev = y
        y = (y*y + c) / (2 * y + b - D)             # :389
        if y > y_prev:
            if y - y_prev <= 1: break
        else:
            if y_prev - y <= 1: break
    return y
```

The output `y` is the *new* balance of coin `j`; the trader receives `xp[j] − y` (minus fee). Starting from `y = D` (again an upper bound) the iteration descends monotonically.

`get_y_D(A_, i, xp, D)` (`:584-626`) is the same quadratic solved for a different question: "given a *smaller* target `D` (because LP tokens are being burned), what would `x_i` have to be if only coin `i` leaves the pool?" Identical `b`, `c`, Newton step (`:617`); the only difference is that `D` is an *input* rather than computed from `xp`. Used by single-coin withdrawal (§1.9).

### 1.6 `get_virtual_price()` (`:229-238`)

```vyper
def get_virtual_price() -> uint256:
    D: uint256 = self.get_D(self._xp(), self._A())
    token_supply: uint256 = self.token.totalSupply()
    return D * PRECISION / token_supply
```

`D / totalSupply`: how many "balanced dollars" one LP token represents. It starts at 1e18 on the first deposit (`mint_amount = D1`, `:342`) and only *increases* thereafter, because every operation that changes the composition — swap, imbalanced deposit, imbalanced withdrawal — charges a fee that stays in the pool, and the fee is precisely the amount by which `D` per share grows. The only thing that moves it discontinuously is a change of `A` (same balances, different `D`): a ramp up on an imbalanced pool raises it, a ramp down lowers it. (Ramps are slow for exactly this reason — see §1.2.)

Because it is monotone and cannot be pushed down by trading, `get_virtual_price` became the standard way for *other* protocols to price Curve LP tokens: `LP price = virtual_price × min(underlying prices)`. That is also what makes the **read-only reentrancy** bug (§5.1) so dangerous: during a callback in the middle of `remove_liquidity` the value is transiently wrong, and this function has no reentrancy guard (`:227-229` — there is no `@nonreentrant` on it; NG fixed this: `CurveStableSwapNG.vy:1739`).

### 1.7 `exchange()` — the swap (`:429-493`)

```vyper
@external
@nonreentrant('lock')
def exchange(i: int128, j: int128, dx: uint256, min_dy: uint256):
    assert not self.is_killed
    rates: uint256[N_COINS] = RATES
    old_balances: uint256[N_COINS] = self.balances
    xp: uint256[N_COINS] = self._xp_mem(old_balances)

    dx_w_fee: uint256 = dx
    input_coin: address = self.coins[i]
    if i == FEE_INDEX:                                   # USDT might charge a transfer fee one day:
        dx_w_fee = ERC20(input_coin).balanceOf(self)     #   measure balance before…
    _response: Bytes[32] = raw_call(input_coin, concat(method_id("transferFrom(address,address,uint256)"), ...), max_outsize=32)
    if len(_response) > 0:
        assert convert(_response, bool)                  # "safeTransferFrom": accept no-return-value ERC20s
    if i == FEE_INDEX:
        dx_w_fee = ERC20(input_coin).balanceOf(self) - dx_w_fee   # …and after; use what actually arrived

    x: uint256 = xp[i] + dx_w_fee * rates[i] / PRECISION # new normalised balance of i
    y: uint256 = self.get_y(i, j, x, xp)                 # required new balance of j
    dy: uint256 = xp[j] - y - 1                          # :465  "-1 just in case there were some rounding errors"
    dy_fee: uint256 = dy * self.fee / FEE_DENOMINATOR    # :466
    dy = (dy - dy_fee) * PRECISION / rates[j]            # back to token units
    assert dy >= min_dy, "Exchange resulted in fewer coins than expected"

    dy_admin_fee: uint256 = dy_fee * self.admin_fee / FEE_DENOMINATOR      # :472
    dy_admin_fee = dy_admin_fee * PRECISION / rates[j]                     # :473

    self.balances[i] = old_balances[i] + dx_w_fee
    self.balances[j] = old_balances[j] - dy - dy_admin_fee                 # :478
    raw_call(self.coins[j], concat(method_id("transfer(address,uint256)"), ...))   # send dy to msg.sender
    log TokenExchange(msg.sender, i, dx, j, dy)
```

Step by step:

1. **Snapshot** balances → `xp` (pre-swap), so `get_y` computes `D` from the pre-swap state.
2. **Pull tokens** with a raw `transferFrom` that tolerates ERC20s returning nothing (USDT). For the `FEE_INDEX` coin the *received* amount is measured by balance diff.
3. **Solve**: `x = xp[i] + dx` → `y = get_y(...)` → `dy = xp[j] − y − 1`.
4. **Fee**: `dy_fee = dy · fee`. The trader gets `dy − dy_fee`. Of `dy_fee`, `admin_fee` share (50 %) is *not* added to `balances[j]` (`:478` subtracts `dy_admin_fee`), so it sits in the contract as `balanceOf − balances` = admin-claimable; the other 50 % stays in `balances[j]` and accrues to LPs via `D`.
5. **Write balances, push tokens, emit.** All state writes happen before the outgoing transfer; the whole function is under `@nonreentrant('lock')` (`:430`).

There is no `receiver` parameter and no return value in this generation (NG adds both).

`get_dy(i, j, dx)` (`:402-411`) is the read-only twin: same maths, no transfers, returns `dy − fee` in token units. `get_dy_underlying` (`:416-426`) exists only for interface compatibility with lending pools here (uses `PRECISION_MUL` instead of `RATES`, which is identical for 3pool).

### 1.8 `add_liquidity()` (`:268-351`)

```vyper
def add_liquidity(amounts: uint256[N_COINS], min_mint_amount: uint256):
    assert not self.is_killed
    fees: uint256[N_COINS] = empty(uint256[N_COINS])
    _fee: uint256 = self.fee * N_COINS / (4 * (N_COINS - 1))          # :274  imbalance fee
    _admin_fee: uint256 = self.admin_fee
    amp: uint256 = self._A()
    token_supply: uint256 = self.token.totalSupply()
    D0: uint256 = 0
    old_balances: uint256[N_COINS] = self.balances
    if token_supply > 0:
        D0 = self.get_D_mem(old_balances, amp)
    new_balances: uint256[N_COINS] = old_balances
    for i in range(N_COINS):
        in_amount: uint256 = amounts[i]
        if token_supply == 0:
            assert in_amount > 0                                       # :289 initial deposit requires all coins
        ... transferFrom (with FEE_INDEX balance-diff, :295/:312) ...
        new_balances[i] = old_balances[i] + in_amount
    D1: uint256 = self.get_D_mem(new_balances, amp)
    assert D1 > D0
    D2: uint256 = D1
    if token_supply > 0:
        for i in range(N_COINS):
            ideal_balance: uint256 = D1 * old_balances[i] / D0          # what balance i *would* be if the deposit were proportional
            difference = |ideal_balance - new_balances[i]|
            fees[i] = _fee * difference / FEE_DENOMINATOR
            self.balances[i] = new_balances[i] - (fees[i] * _admin_fee / FEE_DENOMINATOR)
            new_balances[i] -= fees[i]
        D2 = self.get_D_mem(new_balances, amp)                          # D after fees
    else:
        self.balances = new_balances
    mint_amount: uint256 = 0
    if token_supply == 0:
        mint_amount = D1                                               # first LP gets D tokens → virtual price = 1.0
    else:
        mint_amount = token_supply * (D2 - D0) / D0                     # :344
    assert mint_amount >= min_mint_amount, "Slippage screwed you"
    self.token.mint(msg.sender, mint_amount)
    log AddLiquidity(msg.sender, amounts, fees, D1, token_supply + mint_amount)
```

**Why the first deposit needs every coin (`:289`)**: with a zero balance `get_D` divides by zero (`:205`), and more fundamentally the pool's price would be undefined. The first depositor receives exactly `D1` LP tokens so that virtual price starts at 1e18.

**Mint formula** `supply · (D2 − D0)/D0`: LP share = fraction by which the deposit grew `D` (after fees). A perfectly proportional deposit grows `D` by exactly the deposit's value; an imbalanced one grows it less (you are implicitly doing a swap) *and* pays the imbalance fee.

**Imbalance fee `fee · n / (4(n−1))`** (`:274`): a deposit of only coin `i` is economically half a swap (you push the pool off balance but nobody pulls it back). The fee is charged on `|new − ideal|` for *every* coin, so a proportional deposit (difference = 0 everywhere) pays nothing. The factor:

| n | `n/(4(n−1))` | so imbalance fee = |
|---|---|---|
| 2 | 0.5 | ½ · swap fee (one-sided deposit ≈ half a swap) |
| 3 | 0.375 | |
| 4 | 0.333 | |
| 8 | 0.286 | |

The `n/(n−1)` part compensates for the fact that when you deposit one coin, *all* `n` balances deviate from ideal (the other `n−1` are now "too small"), so the sum of `differences` overcounts by roughly `n/(n−1)`... and the `/4` makes the 2-coin case exactly half a swap. It is a heuristic, not an exact swap-equivalence; NG keeps the same constant (`CurveStableSwapNG.vy:626-629`).

Admin share of these fees is again kept out of `balances[i]` (`:333`).

### 1.9 The three withdrawals

**`remove_liquidity(_amount, min_amounts)` (`:496-525`) — proportional, no fee, works even when killed.**

```vyper
for i in range(N_COINS):
    value: uint256 = self.balances[i] * _amount / total_supply
    assert value >= min_amounts[i]
    self.balances[i] -= value
    ...transfer value...
self.token.burnFrom(msg.sender, _amount)
```

You get `share` of every balance. No `get_D`, no fee (the composition doesn't change). Note `burnFrom` is called *after* the transfers — harmless for ERC20s, but see §5.1 for the ETH-pool variant.

**`remove_liquidity_imbalance(amounts, max_burn_amount)` (`:527-580`)** — "I want exactly these amounts; burn whatever LP that costs." Mirror image of `add_liquidity`: `D0` → subtract amounts → `D1` → imbalance fee on `|ideal − new|` → `D2`; `token_amount = (D0 − D2)·supply/D0 + 1` (`:557-559`, the `+1` rounds against the caller "to make it unfavorable for the attacker"). Burn, then transfer.

**`remove_liquidity_one_coin(_token_amount, i, min_amount)` (`:668-699`)** — burn LP, receive only coin `i`. The maths is in `_calc_withdraw_one_coin` (`:628-660`):

```vyper
D0: uint256 = self.get_D(xp, amp)
D1: uint256 = D0 - _token_amount * D0 / total_supply     # :642 target D after burning that share
new_y: uint256 = self.get_y_D(amp, i, xp, D1)            # balance of i that alone brings D to D1
dy_0: uint256 = (xp[i] - new_y) / precisions[i]          # fee-less amount

for j in range(N_COINS):                                  # charge the imbalance fee on every coin's deviation
    if j == i:
        dx_expected = xp[j] * D1 / D0 - new_y             # i moves *more* than a proportional withdrawal would
    else:
        dx_expected = xp[j] - xp[j] * D1 / D0             # others move *less* (not at all)
    xp_reduced[j] -= _fee * dx_expected / FEE_DENOMINATOR

dy: uint256 = xp_reduced[i] - self.get_y_D(amp, i, xp_reduced, D1)   # re-solve on fee-reduced balances
dy = (dy - 1) / precisions[i]
return dy, dy_0 - dy                                      # (amount out, fee paid)
```

Two `get_y_D` solves: the first gives the fee-less answer, the second re-solves after shaving the imbalance fee off every balance. The caller then removes `dy + admin's share of the fee` from `balances[i]` (`:679`), burns, transfers.

### 1.10 Admin plumbing

* `admin_balances(i)` (`:804`) = `balanceOf(self) − balances[i]` — the fee accrual is *implicit*. `withdraw_admin_fees` (`:809-828`) sweeps that difference to the owner; `donate_admin_fees` (`:831-834`) instead sets `balances = balanceOf`, gifting it to LPs.
* `commit_new_fee` / `apply_new_fee` / `revert_new_parameters` (`:734-768`): 3-day timelock, `fee ≤ 50 %`, `admin_fee ≤ 100 %`.
* `commit_transfer_ownership` / `apply_transfer_ownership` (`:771-799`): same pattern.
* `calc_token_amount(amounts, deposit)` (`:243-265`): `D` delta ratio **without fees** — the docstring says it's "needed to prevent front-running, not for precise calculations", i.e. use it to compute `min_mint_amount`, not to display an exact quote. (NG's Views version includes fees.)

### 1.11 End-to-end trace: sell 1,000 USDC for USDT

Pool state: 50,000,000 DAI / 40,000,000 USDC / 30,000,000 USDT (USDT is scarce), `A = 2000`, `fee = 4000000` (0.04 %), `admin_fee = 5000000000` (50 %). All numbers below are from running the contract's integer algorithms.

```
user ──exchange(1, 2, 1_000_000_000, min_dy)──▶ 3pool
                                                 │ old_balances = [50e24, 40e12, 30e12]
                                                 │ xp = RATES*bal/1e18 = [50e24, 40e24, 30e24]        (§1.3)
                                                 │ USDC.transferFrom(user, pool, 1e9)
                                                 │ x = xp[1] + 1e9 * 1e30 / 1e18 = 40,001,000e18
                                                 │ get_y(1, 2, x, xp):
                                                 │    get_D(xp, 2000)  →  D = 119,998,667.39e18   (3 Newton iterations; note D < S = 120M because pool is imbalanced)
                                                 │    Ann = 6000 ; S' = 50e24 + 40.001e24 ; b = S' + D/Ann = 90,020,999.78e18
                                                 │    c = D^4 / (27 · 50e24 · 40.001e24 · 6000)
                                                 │    Newton from y=D: 9 iterations → y = 29,999,000.1777e18
                                                 │ dy     = xp[2] − y − 1 = 999.8223e18       (≈ 0.018 % worse than 1:1 because USDT is the scarce coin)
                                                 │ dy_fee = dy · 0.0004      = 0.39993e18
                                                 │ dy     = (dy − dy_fee)·1e18/1e30 = 999,422,408   (= 999.422408 USDT, 6 decimals)
                                                 │ assert dy ≥ min_dy
                                                 │ dy_admin_fee = 0.39993e18 · 0.5 → /1e12 = 199,964   (0.199964 USDT to DAO)
                                                 │ balances[1] = 40,001,000e6
                                                 │ balances[2] = 30,000,000e6 − 999,422,408 − 199,964 = 29,999,000,377,628
                                                 │      (the other 0.199964 USDT of fee stays inside balances[2] → LPs)
                                                 │ USDT.transfer(user, 999,422,408)
                                                 ▼ log TokenExchange(user, 1, 1e9, 2, 999422408)
```

Sanity: the trader's total cost vs 1:1 is 0.5776 USDT = 0.018 % curve slippage + 0.04 % fee. Compare Uniswap v2 with the same 40M/30M reserves: `dy = 30M − 40M·30M/(40.001M) ≈ 749.98` before fees — a 25 % loss, because xy=k prices the pool at 30/40 = 0.75.

---

## 2. StableSwap-NG (`stableswap-ng/contracts/main/`)

NG ("next generation", late 2023) is the same maths re-engineered. `CurveStableSwapNG.vy` (all `:line` refs in §2.1–2.3 point there) is 1,890 lines because it also *is* the LP token (ERC20 + EIP-2612 `permit`, `:1541-1685`) and carries two oracles.

### 2.1 What changed vs classic

| classic | NG | where |
|---|---|---|
| fixed `N_COINS` per compiled pool | `DynArray[…, MAX_COINS=8]`, `N_COINS` immutable set in constructor | `:152-157`, `:239` |
| one implementation per pool | pools are **blueprints** deployed by the factory (`create_from_blueprint`) | §2.5 |
| separate LP token | pool *is* the ERC20 | `:213-232`, `:1557-1685` |
| `RATES` constant | `rate_multipliers` immutable + live `_stored_rates()` for oracle / ERC4626 coins | `:433-470` |
| trusts `self.balances` | `stored_balances[]` + `admin_balances[]`; `_balances()` = stored − admin (or `balanceOf − admin` for rebasing) | `:473-499` |
| flat fee | `_dynamic_fee` that scales with imbalance | `:887-901` |
| `exchange` only | `exchange` + `exchange_received` (no approvals) + `_receiver` param + return value | `:504-567` |
| no oracle | EMA price oracle per coin vs coin 0 + EMA `D` (TVL) oracle | `:1313-1466` |
| admin fee implicit | explicit `admin_balances`, `withdraw_admin_fees` callable by anyone → `factory.fee_receiver()` | `:875-881`, `:988-1005` |
| `get_dy` etc. in-pool | view math delegated to a shared **Views** contract; metapools delegate `get_D/get_y` to a **Math** contract | §2.4 |
| 3-day-timelocked fee change by owner | `set_new_fee` by `factory.admin()` (DAO), immediate | `:1862-1875` |
| `is_killed` | removed — LPs always have `remove_liquidity`; `get_D` `raise`s if Newton does not converge, docstring says "pool is borked and LPs can withdraw via `remove_liquidity`" | `:1124-1127` |

**Asset types** (`:11-27`, constructor `:239-353`): `0` plain ERC20, `1` oracle-rated (wstETH, sDAI-style `convertToAssets` via arbitrary method id), `2` rebasing (stETH), `3` ERC4626. The constructor packs `method_id << 224 | oracle_addr` into `rate_oracles[i]` (`:314`), and for ERC4626 precomputes `call_amount = 10**decimals` and a `scale_factor` for the underlying (`:319-323`). `pool_contains_rebasing_tokens = 2 in asset_types` (`:263`) switches the whole balance-accounting strategy.

**`_stored_rates()`** (`:433-470`):

```vyper
rates: DynArray[uint256, MAX_COINS] = rate_multipliers          # 10**(36 - decimals)
for i in range(N_COINS_128, bound=MAX_COINS_128):
    if asset_types[i] == 1 and not rate_oracles[i] == 0:
        oracle_response: Bytes[32] = raw_call(convert(rate_oracles[i] % 2**160, address), _abi_encode(rate_oracles[i] & ORACLE_BIT_MASK), max_outsize=32, is_static_call=True)
        rates[i] = unsafe_div(rates[i] * convert(oracle_response, uint256), PRECISION)     # oracle must return 1e18-scaled
    elif asset_types[i] == 3:
        rates[i] = unsafe_div(rates[i] * ERC4626(coins[i]).convertToAssets(call_amount[i]) * scale_factor[i], PRECISION)
return rates
```

So `xp_i = balance_i · rate_i / 1e18` now means "balance expressed in 18-decimal *underlying* units". A wstETH/ETH pool with rate 1.15 treats 1 wstETH as 1.15 ETH and the invariant sees a balanced pool at 1:1.15 — that is how a pegged-but-appreciating asset is handled without the pool "seeing" a depeg.

**`_transfer_in(coin_idx, dx, sender, expect_optimistic_transfer)`** (`:358-395`):

```vyper
_dx: uint256 = ERC20(coins[coin_idx]).balanceOf(self)
if expect_optimistic_transfer:
    _dx = _dx - self.stored_balances[coin_idx]             # :378 what arrived since last accounting
    assert _dx >= dx                                       # :379
else:
    assert dx > 0
    assert ERC20(coins[coin_idx]).transferFrom(sender, self, dx, default_return_value=True)
    _dx = ERC20(coins[coin_idx]).balanceOf(self) - _dx     # balance diff → fee-on-transfer safe for *all* coins
self.stored_balances[coin_idx] += _dx
return _dx
```

`default_return_value=True` is Vyper's built-in "treat no return data as success" — replaces the raw_call dance of the classic pool. Every deposit is measured by balance diff, so `stored_balances` is exact even for fee-on-transfer tokens.

**`exchange_received`** (`:534-567`) — the aggregator path. Caller transfers coin `i` to the pool *first* (in the same tx), then calls this; `_transfer_in` with `expect_optimistic_transfer=True` measures the surplus over `stored_balances[i]` and uses it as `dx`. Saves an `approve` + `transferFrom` (~20-40k gas) and lets routers chain pools by sending output straight to the next pool. Disabled for rebasing pools (`:556`) because there `stored_balances` cannot be trusted (a rebase looks like a deposit). There is no `exchange_extended` in this codebase (that was a crypto-pool feature).

**`_transfer_out`** (`:398-430`): non-rebasing pools decrement `stored_balances` and transfer; rebasing pools read `balanceOf` before, transfer, then set `stored_balances = before − amount` (so an in-between rebase is absorbed).

**`_balances()`** (`:473-499`) returns what belongs to LPs: `stored_balances[i] − admin_balances[i]`, or `balanceOf − admin_balances[i]` for rebasing pools ("LPs keep all rebases and admin only claims swap fees … immune to slashing events").

### 2.2 The dynamic fee (`:887-901`)

```vyper
def _dynamic_fee(xpi: uint256, xpj: uint256, _fee: uint256) -> uint256:
    _offpeg_fee_multiplier: uint256 = self.offpeg_fee_multiplier
    if _offpeg_fee_multiplier <= FEE_DENOMINATOR:
        return _fee                                              # multiplier ≤ 1 → flat fee
    xps2: uint256 = (xpi + xpj) ** 2                             # :893
    return unsafe_div(
        unsafe_mul(_offpeg_fee_multiplier, _fee),
        unsafe_add(unsafe_sub(_offpeg_fee_multiplier, FEE_DENOMINATOR) * 4 * xpi * xpj / xps2, FEE_DENOMINATOR)
    )
```

In real numbers with `m = offpeg_fee_multiplier / 1e10`:

```
fee_dyn = m · fee / ( (m − 1) · 4·x_i·x_j/(x_i + x_j)²  +  1 )
```

`4 x_i x_j / (x_i+x_j)²` is 1 when `x_i = x_j` and → 0 as one coin dominates. So the fee is `fee` at balance and rises smoothly to `m · fee` at extreme imbalance. Example `m = 2`, balances 3:1 → `4·3/16 = 0.75` → `fee_dyn = 2/(1.75) · fee = 1.14 · fee`. In `__exchange` the arguments are the *midpoints* `(xp_i + x)/2, (xp_j + y)/2` (`:918`), i.e. the average state during the trade. The same fee is applied per-coin in `add_liquidity`/`remove_liquidity_imbalance`/`_calc_withdraw_one_coin` with `xs = rates·(old+new)` vs `ys = (D0+D1)/n` (`:634-642`, `:776-780`, `:1265-1282`). `set_new_fee` bounds `m · fee ≤ MAX_FEE` (`:1871`). Purpose: when a stablecoin depegs, LPs are compensated more for absorbing it and arbitrageurs are slowed — the classic pools' flat 0.04 % was too cheap during the March 2023 USDC depeg.

### 2.3 Swap path, oracles, admin fees

**`exchange` → `_exchange` → `__exchange`** (`:504-531`, `:943-985`, `:904-940`):

```
_exchange(sender, i, j, _dx, _min_dy, receiver, optimistic)
   rates = _stored_rates(); old_balances = _balances(); xp = _xp_mem(rates, old_balances)
   dx = _transfer_in(i, _dx, sender, optimistic)
   x  = xp[i] + dx·rates[i]/1e18
   dy = __exchange(x, xp, rates, i, j)
        amp = _A(); D = get_D(xp, amp); y = get_y(i, j, x, xp, amp, D)
        dy = xp[j] − y − 1
        dy_fee = dy · _dynamic_fee((xp[i]+x)/2, (xp[j]+y)/2, fee) / 1e10
        dy = (dy − dy_fee)·1e18/rates[j]
        admin_balances[j] += (dy_fee·admin_fee/1e10)·1e18/rates[j]          # explicit, :925-928
        xp[i] = x; xp[j] = y; upkeep_oracles(xp, amp, D)                     # :933-937
   assert dy ≥ _min_dy
   _transfer_out(j, dy, receiver)
   log TokenExchange
```

Differences from classic: `get_D` is computed once and passed into `get_y` (`:1009-1076` now takes `_amp, _D`); `admin_fee` is a *constant* 50 % (`:171`); the admin share is tracked explicitly; and the oracle is updated with the *post-swap state prices* (not the trade's execution price — resistant to single-trade manipulation).

**Spot price `_get_p`** (`:1313-1337`). Differentiate invariant (1) implicitly: `∂F/∂x_k = Ann + Dr/x_k` where `Dr = D^(n+1)/(n^n Πx)`. The marginal price of coin `k` in units of coin 0 is the ratio of partials:

```
p_k = (Ann + Dr/x_k) / (Ann + Dr/x_0)  =  (Ann·x_0 + Dr·x_0/x_k) / (Ann·x_0 + Dr)
```

which is line 1334: `p.append(10**18 * (xp0_A + Dr * xp[0] / xp[i]) / (xp0_A + Dr))` with `xp0_A = ANN·xp[0]/A_PRECISION` and `Dr = D/n^n · Π(D/x_i)`. Prices are for coins 1…n−1 relative to coin 0 (`get_p(i)` returns the price of `coins[i+1]`, `:1427-1442`).

**EMA oracle** (`upkeep_oracles` `:1340-1390`, `_calc_moving_average` `:1393-1412`): storage packs `(last_spot, ema)` into one slot per coin (`last_prices_packed`, `:200`) and `(last_D, ema_D)` (`:201`). On every state-changing call:

```
alpha   = exp(−(now − ma_last_time)·1e18 / ma_exp_time)        # :1403
ema_new = last_spot·(1 − alpha) + ema_old·alpha
```

`ma_exp_time` is `T / ln 2`, so after `T` seconds `alpha = ½`: the window is a *half-life*. Spot is capped at `2e18` before being stored (`:1361`) so a manipulated block can move the EMA by at most a bounded step. `price_oracle(i)` (`:1445`) and `D_oracle()` (`:1456`) are `@view @nonreentrant` (`:1444`, `:1455`) — they *compute* the EMA as of `block.timestamp` from the stored pair, so reads are fresh without a write. `D_ma_time` defaults to 62324 s (= 12 h / ln 2, `:297`). `remove_liquidity` (proportional) does not call `upkeep_oracles`: price is unchanged, and `D` is scaled down proportionally by hand (`:843-858`).

**Admin fees**: `_withdraw_admin_fees` (`:988-1005`) sends every `admin_balances[i]` to `factory.fee_receiver()` and zeroes them; it is callable by anyone (`withdraw_admin_fees`, `:875`) and runs automatically at the end of `remove_liquidity` unless `_claim_admin_fees=False` (`:867-869`). This is how the DAO's half of every fee reaches the FeeDistributor (§3.6).

**`add_liquidity` / `remove_*`** (`:570-872`) follow §1.8–1.9 exactly, with (a) `_transfer_in` per coin, (b) dynamic fee per coin, (c) `admin_balances[i] += fee_i · 50 %` instead of skewing `balances`, (d) LP mint/burn inline (`self.balanceOf[_receiver] += mint_amount`), (e) `upkeep_oracles` at the end, (f) a `_receiver`. `remove_liquidity_imbalance` transfers out *before* computing fees (`:749-754`) — safe because the function is `@nonreentrant` and the burn amount is checked against `_max_burn_amount` after.

**`ramp_A`** (`:1825-1845`): same rules as classic, with `A_PRECISION` scaling; `MAX_A_CHANGE = 10` enforced at `:1835-1837`.

### 2.4 Math and Views contracts — why split

`CurveStableSwapNGMath.vy` (269 lines) exposes `get_y` (`:18`), `get_D` (`:90`), `get_y_D` (`:143`), `exp` (`:203`) as `@external @pure` with `N_COINS` as an argument. The plain pool inlines its own copies (`CurveStableSwapNG.vy:1009-1184`) because internal calls are cheaper; the **metapool** instead calls `math.get_D(...)` / `math.get_y(...)` (`CurveStableSwapMetaNG.vy:1117-1118`, `math` immutable at `:216`) to stay under the 24 KB contract-size limit — metapools carry the extra `exchange_underlying` logic.

`CurveStableSwapNGViews.vy` (704 lines) is a stateless helper deployed once per chain and registered in the factory (`set_views_implementation`, `CurveStableSwapFactoryNG.vy:813`). The pool's `get_dy` (`CurveStableSwapNG.vy:1702`), `get_dx` (`:1688`), `calc_token_amount` (`:1760`), `dynamic_fee` (`:1811`) are one-liners that forward to `StableSwapViews(factory.views_implementation()).x(..., self)`. The Views contract re-reads `stored_rates()` / `get_balances()` / `A()` / `fee()` / `offpeg_fee_multiplier()` from the pool (`_get_rates_balances_xp`, `Views:690-704`) and re-runs the maths (`Views:59-88` for `get_dy`, including the dynamic fee at `:82`). Benefits: pool bytecode stays small; quoting logic (e.g. `get_dx`, `:349-378` — inverting the swap by solving `get_y(j, i, y_target)` with the fee grossed-up) can be upgraded by the DAO without touching pools; integrators get `calc_token_amount` **with** fees (`:395-492`) unlike classic.

### 2.5 Metapools (`CurveStableSwapMetaNG.vy`)

A metapool is a 2-coin pool `[TOKEN, BASE_LP]` — e.g. `[LUSD, 3CRV]` — that lets a new stablecoin trade against the whole base pool's liquidity without fragmenting it (`:8-13`). Key differences:

* **The rate of coin 1 is the base pool's virtual price** (`_stored_rates`, `:528-562`, line `:535`: `rates = [rate_multiplier, StableSwap(BASE_POOL).get_virtual_price()]`). So `xp[1]` = 3CRV balance × (DAI-equivalent per 3CRV). As the base pool earns fees, `xp[1]` grows and the metapool automatically sees 3CRV as worth more. (This is the coupling exploited by read-only reentrancy, §5.1.)
* `BASE_POOL_IS_NG` is detected at construction by probing `D_ma_time()` (`:340`); it decides which `add_liquidity` ABI to use in `_meta_add_liquidity` (`:1196-1224`).
* **`exchange_underlying(i, j, dx, min_dy)`** (`:659-751`) uses "underlying indices": 0 = TOKEN, 1..n = base coins. Index arithmetic at `:691-703` (`base_i = i − MAX_METAPOOL_COIN_INDEX`, `MAX_METAPOOL_COIN_INDEX = 1`, `:203`). Three cases:

```
case A  i == 0, j > 0   (LUSD → USDC)
   _transfer_in(meta_i=0, …)                                  pull LUSD
   x = xp[0] + dx·rate0 ; dy = __exchange(x, xp, rates, 0, 1)  meta swap LUSD→3CRV (dy is 3CRV)
   stored_balances[1] -= dy
   BASE_POOL.remove_liquidity_one_coin(dy, base_j, 0)          burn 3CRV for USDC   (:730-733)
   dy = USDC balance diff ; assert dy ≥ min_dy ; transfer

case B  i > 0, j == 0   (USDC → LUSD)
   _transfer_in(meta_i=1, base_i, …, is_base_pool_swap=False)  pulls USDC, then *inside _transfer_in*
       → _meta_add_liquidity(dx, base_i): BASE_POOL.add_liquidity([0,dx,0], 0)  → 3CRV minted to metapool  (:465-478, :1196)
       returns the 3CRV amount, stored_balances[1] += it
   x = xp[1] + dx3crv·virtual_price ; dy = __exchange(x, xp, rates, 1, 0) ; transfer LUSD

case C  i > 0, j > 0   (USDC → USDT)
   _transfer_in(…, is_base_pool_swap=True) just pulls USDC, no LP minted (:461-463)
   BASE_POOL.exchange(base_i, base_j, dx, min_dy)                (:743-746)  — pure passthrough; docstring says use the base pool directly
```

`_transfer_in` (`:423-489`) is where the magic lives: when the input is a base coin and the swap crosses into the metapool, it deposits into the base pool and returns *LP tokens received* as the effective `dx`.

* **`MetaZapNG.vy`** does the deposit/withdraw side of the same trick for LPs: `add_liquidity(pool, [LUSD, DAI, USDC, USDT], min)` (`:174-249`) pulls all four, `base_pool.add_liquidity([DAI,USDC,USDT])` → 3CRV, then `metapool.add_liquidity([LUSD, 3CRV], min, receiver)` (`:229-248`); `remove_liquidity*` (`:272`, `:327`, `:370`) do the reverse. Approvals are set once and cached (`is_approved`, `:101-111`).

### 2.6 Factory (`CurveStableSwapFactoryNG.vy`)

Permissionless deployer + registry. `deploy_plain_pool(name, symbol, coins, A, fee, offpeg_mult, ma_exp_time, impl_idx, asset_types, method_ids, oracles)` (`:458-568`):

1. validate: ≥2 coins, arrays same length, `fee ≤ 1 %`, `offpeg·fee ≤ MAX_FEE`, ≤18 decimals, no duplicates (`:495-518`).
2. `rate_multipliers[i] = 10**(36 − decimals)` (`:511`).
3. `pool = create_from_blueprint(implementation, …constructor args…, code_offset=3)` (`:522-536`) — EIP-5202 blueprint: the implementation address holds *initcode*, so each pool is a full independent contract (not a proxy) with its own immutables. No `delegatecall`, no upgradeability.
4. registry writes: `pool_list`, `pool_data[pool]` (struct `PoolArray`, `:10-17`), and the `markets[coinA ^ coinB]` index (`:555-560`) that `find_pool_for_coins` (`:124`) uses.

`deploy_metapool` (`:571-691`) requires the base pool to have been whitelisted by the DAO via `add_base_pool` (`:715-758`: no rebasing coins, no native ETH) and forbids pairing a base coin with its own LP (`:613`). `deploy_gauge(pool)` (`:694-711`) blueprints a `LiquidityGauge` (§3.4) for the pool — anyone can, but it only earns CRV once the DAO adds it to the `GaugeController`. Implementations (pool / metapool / math / views / gauge) are swappable by the admin (`:761-822`) — affects *future* deployments only.

### 2.7 Trace: `exchange_received` (aggregator swap USDC → USDT in a plain NG pool)

```
Router tx:
  USDC.transfer(pool, 1_000e6)                       # step 1: tokens land first; pool.stored_balances[1] still old
  pool.exchange_received(1, 2, 1_000e6, min_dy, router)
     _exchange(router, 1, 2, 1e9, min, router, optimistic=True)
        rates = _stored_rates() = [1e18, 1e30, 1e30]  (all asset_type 0 → constant)
        old_balances = _balances() = stored − admin
        xp = _xp_mem(rates, old_balances)
        dx = _transfer_in(1, 1e9, router, True):
              _dx = USDC.balanceOf(pool) − stored_balances[1] = 1e9   (:378)
              assert 1e9 ≥ 1e9                                        (:379)
              stored_balances[1] += 1e9
        x  = xp[1] + 1e9·1e30/1e18
        dy = __exchange(x, xp, rates, 1, 2)
              D = get_D(xp, amp) ; y = get_y(1,2,x,xp,amp,D) ; dy = xp[2] − y − 1
              fee = _dynamic_fee((xp[1]+x)/2, (xp[2]+y)/2, base_fee)       # imbalance-aware
              dy = (dy − dy·fee/1e10)·1e18/1e30 ; admin_balances[2] += 50 % of fee
              upkeep_oracles({xp with x, y}, amp, D)                        # EMA of state price & D updated
        assert dy ≥ min_dy
        _transfer_out(2, dy, router): stored_balances[2] −= dy ; USDT.transfer(router, dy)
        return dy
```

No `approve`, no `transferFrom`. If someone front-runs by calling `exchange_received` before the router in the same block, *they* take the router's USDC — which is why this is only for atomic (same-tx) use, as the docstring warns (`:540-546`).

### 2.8 Trace: metapool `exchange_underlying(2, 0)` — USDC → LUSD via LUSD/3CRV

```
user.exchange_underlying(i=2, j=0, 1_000e6, min_dy)
   rates = [1e30 (LUSD 18d → 1e18), 3pool.get_virtual_price() ≈ 1.03e18]
   xp = [LUSD_bal·1e18/1e18, 3CRV_bal·1.03]                      # 3CRV counted at its DAI value
   i=2 → base_i = 1 (USDC), meta_i = 1 ; j=0 → output_coin = LUSD
   dx_w_fee = _transfer_in(1, 1, 1e9, user, False, is_base_pool_swap=False)
        USDC.transferFrom(user, metapool, 1e9)
        _meta_add_liquidity(1e9, 1): 3pool.add_liquidity([0, 1e9, 0], 0)  → metapool receives ≈ 970.9e18 3CRV (1000 / 1.03, minus 3pool imbalance fee)
        stored_balances[1] += 970.9e18 ; return 970.9e18
   x  = xp[1] + 970.9e18 · 1.03e18 / 1e18  ≈ xp[1] + 1000e18        # back in DAI-value terms
   dy = __exchange(x, xp, rates, 1, 0)     → math.get_D / math.get_y on the 2-coin pool → LUSD out (minus metapool fee)
   assert dy ≥ min_dy ; LUSD.transfer(user, dy)
   log TokenExchangeUnderlying(user, 2, 1e9, 0, dy)
```

The user paid two fees (3pool deposit imbalance fee + metapool swap fee), which is the price of borrowing the base pool's depth.

---

## 3. Curve DAO: staking and tokenomics (`curve-dao-contracts/contracts/`)

The AMM above is the product. The DAO layer answers: *who owns the fees, and how is liquidity attracted?* Five contracts:

```
                 ┌──────────────┐  mintable_in_timeframe / rate           ┌──────────────┐
                 │  ERC20CRV    │◀────────────────────────────────────────│   Minter     │
                 │  (inflation) │  mint(user, amount)                     │              │
                 └──────────────┘                                         └──────┬───────┘
                                                                                 │ user_checkpoint / integrate_fraction
   LP token ──deposit──▶ ┌──────────────┐  gauge_relative_weight(gauge, t) ┌──────▼───────┐
                         │ LiquidityGauge│◀────────────────────────────────│GaugeController│
                         │  (per pool)   │  checkpoint_gauge               │ (vote weights)│
                         └──────┬───────┘                                  └──────▲───────┘
                                │ balanceOf / totalSupply (boost)                 │ get_last_user_slope / locked__end
                         ┌──────▼───────┐                                         │
   CRV ──create_lock──▶  │ VotingEscrow │─────────────────────────────────────────┘
                         │   (veCRV)    │◀──── user_point_history / point_history ─── FeeDistributor ◀── 3CRV / crvUSD admin fees
                         └──────────────┘                                              (weekly pro-rata to veCRV)
```

### 3.1 `ERC20CRV.vy` — the emission schedule

Constants (`:50-67`):

```vyper
INITIAL_SUPPLY = 1_303_030_303                                  # 43 % premine (30 % shareholders, 3 % employees, 5 % reserve, 5 % early users)
INITIAL_RATE = 274_815_283 * 10**18 / YEAR                      # ≈ 8.714 CRV / second in year 1
RATE_REDUCTION_TIME = YEAR
RATE_REDUCTION_COEFFICIENT = 1189207115002721024                # 2^(1/4) · 1e18  → rate falls 15.9 % per year
```

`_update_mining_parameters` (`:101-122`) advances one epoch: `start_epoch_supply += rate · YEAR`, `rate = rate · 1e18 / 2^(1/4)` (`:117`). It is called lazily by whoever touches the token first after an epoch boundary (`mint` `:338-339`, `update_mining_parameters` `:125`, `start_epoch_time_write` `:136`, `future_epoch_time_write` `:151` — the gauges call the last one). Resulting schedule:

| epoch (from 13 Aug 2020) | rate CRV/s | CRV/year |
|---|---|---|
| 0 | 8.714 | 274.8 M |
| 1 | 7.328 | 231.1 M |
| 2 | 6.162 | 194.3 M |
| 3 | 5.182 | 163.4 M |
| 4 | 4.357 | 137.4 M |
| 5 | 3.664 | 115.5 M |
| 6 (Aug 2026 →) | 3.081 | 97.2 M |

Halving every 4 years; asymptote 3.03 B total. `_available_supply()` (`:167`) = `start_epoch_supply + (now − start_epoch_time)·rate` is the hard cap `mint` checks against (`:340`) — the Minter can never mint faster than the schedule regardless of gauge accounting bugs. `mintable_in_timeframe(start, end)` (`:182-223`) integrates the piecewise-constant rate over `[start, end]`, walking backwards through epochs (`:213`); gauges do *not* use it (they read `rate()` directly), but it is the reference definition. `mint` is `minter`-only (`:334`); `set_minter` can be called once (`:226-236`).

### 3.2 `VotingEscrow.vy` — veCRV

**Model.** Lock `amount` CRV until `end` (rounded *down* to a whole week, `:419`, max 4 years `:425`). Voting power is

```
bias(t) = slope · (end − t)      slope = amount / MAXTIME          (:251-255)
```

so 1 CRV locked 4 years = 1 veCRV, 1 year = 0.25 veCRV, decaying linearly to 0 at `end`. Non-transferable: there is no `transfer` at all — `balanceOf`/`totalSupply` exist only for Aragon/MiniMe compatibility ("not real balanceOf and supply", `:492-494`). Contracts are blocked unless whitelisted via `SmartWalletChecker` (`assert_not_contract`, `:185-195`) — this is what makes Convex/Yearn "vote-locker" wrappers require a DAO vote.

**Storage** (`:88-97`):

```vyper
struct Point:  bias: int128 ; slope: int128 ; ts: uint256 ; blk: uint256     # :25-29
struct LockedBalance:  amount: int128 ; end: uint256                          # :34-36
locked: HashMap[address, LockedBalance]
epoch: uint256
point_history: Point[…]                        # global: epoch → Point
user_point_history: HashMap[address, Point[…]] # per user: user_epoch → Point
user_point_epoch: HashMap[address, uint256]
slope_changes: HashMap[uint256, int128]        # week-timestamp → Δslope to apply when locks end
```

The global total is *also* a single `(bias, slope)` line, because a sum of lines is a line — until some lock expires and its slope must drop out. Those future drops are pre-scheduled in `slope_changes[end]`. That is the whole trick.

**`_checkpoint(addr, old_locked, new_locked)`** (`:234-348`) — the only function that writes points:

1. Compute the user's old and new `(slope, bias)` from the two `LockedBalance`s (`:248-255`); read the scheduled `slope_changes` at the old/new `end` (`:260-266`).
2. Load the last global point and **replay week by week** up to now (`:279-303`): for each week boundary `t_i`, `bias −= slope·Δt` (`:292`), then `slope += slope_changes[t_i]` (the locks that ended that week). Each intermediate week gets its own `point_history[epoch]` so historical queries are exact. `blk` is extrapolated from the observed block/time slope (`:275-277`) for `balanceOfAt(block)`.
3. Apply the user's delta to the global point (`:314-315`) and store it (`:325`).
4. **Schedule** slope changes (`:328-338`): cancel the old lock's scheduled drop (`old_dslope += u_old.slope`), schedule the new one (`new_dslope −= u_new.slope` at `new_locked.end`).
5. Append the user's new point (`:342-348`).

Entry points, all funnelling through `_deposit_for` (`:351-381`) → `_checkpoint` → `transferFrom`: `create_lock(value, unlock_time)` (`:412-429`, requires no existing lock), `increase_amount` (`:432-447`), `increase_unlock_time` (`:450-466`, only later), `deposit_for(addr, value)` (`:393-409`, anyone may top up someone else's *existing* lock — this is how Convex tops up), `withdraw()` (`:469-499`, only after `end`; checkpoints with `new_locked = 0`), and a public `checkpoint()` (`:384-390`) to fill history.

**Reads**: `balanceOf(addr, t)` (`:525-543`) takes the user's last point and decays it: `bias − slope·(t − ts)`, floored at 0 (`:538`). `totalSupply(t)` (`:626-635`) → `supply_at(point, t)` (`:597-623`) replays `slope_changes` forward from the last global point *without writing* (`:613`). `balanceOfAt`/`totalSupplyAt` (`:546-594`, `:639-663`) binary-search `point_history` by block and interpolate time.

### 3.3 `GaugeController.vy` — where emissions go

**Purpose**: every week, split the CRV inflation among gauges according to veCRV votes. Weights are themselves *decaying lines* (a vote is worth the voter's veCRV, which decays), so the controller re-uses the bias/slope/slope_changes machinery, now in three nested layers: per gauge (`points_weight`, `changes_weight`), per gauge-type sum (`points_sum`, `changes_sum`), and total (`points_total`), plus a governance-set multiplier per type (`points_type_weight`). Storage `:66-110`.

**`vote_for_gauge_weights(gauge, user_weight)`** (`:485-555`), `user_weight` in bps (0–10000):

```vyper
slope: uint256 = convert(VotingEscrow(escrow).get_last_user_slope(msg.sender), uint256)   # user's veCRV slope
lock_end: uint256 = VotingEscrow(escrow).locked__end(msg.sender)
next_time: uint256 = (block.timestamp + WEEK) / WEEK * WEEK                                # votes take effect next week
assert lock_end > next_time
assert block.timestamp >= self.last_user_vote[msg.sender][_gauge_addr] + WEIGHT_VOTE_DELAY   # :498  10-day cooldown per gauge
new_slope = VotedSlope({slope: slope * _user_weight / 10000, end: lock_end, power: _user_weight})   # :509
new_bias: uint256 = new_slope.slope * (lock_end - next_time)
power_used = power_used + new_slope.power - old_slope.power ; assert power_used <= 10000            # :520
# remove old vote's (bias, slope) from gauge & type-sum at next_time, add new; cancel old scheduled slope change, schedule new
self.points_weight[_gauge_addr][next_time].bias = max(old_weight_bias + new_bias, old_bias) - old_bias
...
self.changes_weight[_gauge_addr][new_slope.end] += new_slope.slope
self.changes_sum[gauge_type][new_slope.end] += new_slope.slope
self._get_total()
```

So a vote contributes `veCRV_slope · fraction` as a line that decays to 0 exactly when the voter's lock ends. You can split your power across gauges (sum ≤ 100 %), and may only re-vote a given gauge every 10 days (`WEIGHT_VOTE_DELAY`, `:14`) — the anti-flip-flop rule that later made "bribe" markets a thing.

**Filling history**: `_get_weight(gauge)` (`:259-287`), `_get_sum(type)` (`:189-217`), `_get_type_weight(type)` (`:166-186`), `_get_total()` (`:220-256`) each walk forward week by week from the last stored point, decaying bias by `slope·WEEK` and applying `changes_*[t]`, storing a value per week. `_get_total` multiplies each type's sum by its type weight (`:250`). `checkpoint_gauge(addr)` (`:336-343`) = `_get_weight(addr)` + `_get_total()`; gauges call it on every user checkpoint.

**The number gauges consume**: `gauge_relative_weight(addr, t)` (`:371`, internal `:347-368`):

```
relative = 1e18 · type_weight[t] · gauge_weight[t] / total_weight[t]           (:363)     t rounded down to week
```

A gauge earning `relative = 0.02e18` receives 2 % of that week's CRV inflation. Weights are read at the week's *start* (`t / WEEK * WEEK`), so mid-week votes never change the current week.

### 3.4 `LiquidityGauge` — staking LP tokens for CRV

Two versions are in this folder: the original `gauges/LiquidityGauge.vy` (356 lines, 2020, all refs `LG:`) and the NG factory gauge `../stableswap-ng/contracts/main/LiquidityGauge.vy` ("LiquidityGaugeV6", 864 lines, refs `NG:`). The CRV accounting is byte-for-byte the same idea; V6 adds ERC20-ness of the staked position, up to 8 extra reward tokens, permit, `kick`, and veBoost delegation.

**What "staking" means here.** You transfer LP tokens to the gauge (`deposit`, `LG:279-302` / `NG:407-435`). The gauge does not do anything with them — no rehypothecation, they just sit there. In return the gauge *measures* your share of its total over time and lets the Minter mint you CRV proportional to `∫ rate(t) · w_gauge(t) · your_working_balance(t) / working_supply(t) dt`. Everything below is about computing that integral cheaply.

**Storage** (`LG:60-98`):

```vyper
balanceOf / totalSupply                          # raw LP deposits
working_balances / working_supply                # boost-adjusted (§ boost below)
period / period_timestamp[]                      # checkpoint counter and times
integrate_inv_supply[period]                     # 1e18 · ∫ rate(t)·w(t) / working_supply(t) dt  from 0 to that checkpoint   (global)
integrate_inv_supply_of[user]                    # value of the above at the user's last checkpoint
integrate_fraction[user]                         # ∫ working_balance · rate·w / working_supply dt  = CRV *earned* so far (units: CRV wei)
inflation_rate / future_epoch_time               # cached CRV rate & when it next changes
```

The integral is separable: `∫ b_u(t)·r(t)/S(t) dt` where the user's `b_u` is piecewise constant (it changes only when they deposit/withdraw/re-boost). So between two user actions, `earned += b_u · (I_now − I_last)` where `I = ∫ r/S dt` is a *global* running sum. Same pattern as Synthetix `StakingRewards` and every "MasterChef", with the twist that `r(t)·w(t)` (CRV rate × gauge relative weight) changes weekly.

**`_checkpoint(addr)`** (`LG:153-220`):

```vyper
if prev_future_epoch >= _period_time:                                   # CRV rate changed since last checkpoint?
    self.future_epoch_time = CRV20(_token).future_epoch_time_write()
    new_rate = CRV20(_token).rate(); self.inflation_rate = new_rate
Controller(_controller).checkpoint_gauge(self)                          # LG:170  make sure weights for all weeks up to now exist
...
if block.timestamp > _period_time:
    prev_week_time = _period_time
    week_time = min((_period_time + WEEK) / WEEK * WEEK, block.timestamp)
    for i in range(500):                                                # walk week boundaries since last checkpoint
        dt = week_time - prev_week_time
        w = Controller(_controller).gauge_relative_weight(self, prev_week_time / WEEK * WEEK)   # LG:185  weight of *that* week
        if _working_supply > 0:
            if prev_future_epoch >= prev_week_time and prev_future_epoch < week_time:           # CRV epoch boundary inside this week: split
                _integrate_inv_supply += rate * w * (prev_future_epoch - prev_week_time) / _working_supply
                rate = new_rate
                _integrate_inv_supply += rate * w * (week_time - prev_future_epoch) / _working_supply
            else:
                _integrate_inv_supply += rate * w * dt / _working_supply                         # LG:198
        if week_time == block.timestamp: break
        prev_week_time = week_time ; week_time = min(week_time + WEEK, block.timestamp)
_period += 1 ; self.period = _period ; self.period_timestamp[_period] = block.timestamp
self.integrate_inv_supply[_period] = _integrate_inv_supply
self.integrate_fraction[addr] += _working_balance * (_integrate_inv_supply - self.integrate_inv_supply_of[addr]) / 10 ** 18   # LG:217
self.integrate_inv_supply_of[addr] = _integrate_inv_supply
self.integrate_checkpoint_of[addr] = block.timestamp
```

Units: `rate` is CRV-wei/s, `w` is 1e18-scaled, `working_supply` is LP-wei, so `_integrate_inv_supply` is "1e18 × CRV per LP token"; dividing by 1e18 at `:217` yields CRV-wei. Note the user's `_working_balance` used is the *pre-action* one (read at `LG:174` before any balance change), which is why every mutating function calls `_checkpoint` **first**, then changes balances, then `_update_liquidity_limit`.

**Boost — `_update_liquidity_limit(addr, l, L)`** (`LG:125-150`, `NG:350-374`):

```vyper
voting_balance = VotingEscrow.balanceOf(addr)      # NG: VotingEscrowBoost(VEBOOST_PROXY).adjusted_balance_of(addr)  — allows delegated boost
voting_total   = VotingEscrow.totalSupply()
lim = l * TOKENLESS_PRODUCTION / 100                                        # 40 % of your LP counts unconditionally   (:139)
if voting_total > 0:
    lim += L * voting_balance / voting_total * (100 - TOKENLESS_PRODUCTION) / 100   # + 60 % of *pool* liquidity × your veCRV share  (:141)
lim = min(l, lim)                                                            # never more than your actual deposit         (:143)
working_balances[addr] = lim ; working_supply += lim − old
```

`working_balance = min(l, 0.4·l + 0.6·L·ve_u/ve_total)`. With zero veCRV you earn on 40 % of your deposit; the maximum (`working = l`) requires `ve_u / ve_total ≥ l / L` — you need the same share of *all veCRV* as your share of *this gauge*. Hence "2.5× boost" = `1 / 0.4`. The classic version also had a 2-week `BOOST_WARMUP` after launch (`LG:140`). Because `working_balances` is only recomputed when *you* act, a user whose veCRV decayed keeps a stale boost until someone calls `kick(addr)` (`LG:247-265`, `NG:642`), allowed only if their veCRV is 0 or they have had a veCRV event since their last gauge checkpoint.

**Extra rewards (V6)**: `add_reward(token, distributor)` (`NG:717`), `deposit_reward_token(token, amount, epoch)` (`NG:681-714`) streams `amount/epoch` per second; `_checkpoint_rewards` (`NG:299-347`) maintains a parallel `reward_data[token].integral += duration·rate·1e18/totalSupply` and per-user `reward_integral_for` — same separable-integral pattern but on *raw* balances (no boost) and with time-based rather than week-based accrual. `claim_rewards` (`NG:467`).

**Views**: `claimable_tokens(addr)` (`LG:236-244`, `NG:801-808`) is *not* `@view` — it runs `_checkpoint` and returns `integrate_fraction − minter.minted`; call it with `eth_call`. `integrate_checkpoint()` = last checkpoint time.

### 3.5 `Minter.vy` — turning the integral into tokens (`:43-56`)

```vyper
def _mint_for(gauge_addr: address, _for: address):
    assert GaugeController(self.controller).gauge_types(gauge_addr) >= 0          # only DAO-approved gauges
    LiquidityGauge(gauge_addr).user_checkpoint(_for)                              # :46  bring integrate_fraction up to now
    total_mint: uint256 = LiquidityGauge(gauge_addr).integrate_fraction(_for)
    to_mint: uint256 = total_mint - self.minted[_for][gauge_addr]                 # :48  what has not been minted yet
    if to_mint != 0:
        MERC20(self.token).mint(_for, to_mint)                                    # :51
        self.minted[_for][gauge_addr] = total_mint
```

`mint(gauge)` (`:59`), `mint_many(gauge[8])` (`:69`), `mint_for(gauge, user)` (`:82`, needs `toggle_approve_mint`, `:94`). The gauge is the source of truth for *earned*; the Minter only remembers *paid* per (user, gauge). Because `ERC20CRV.mint` enforces `total_supply ≤ available_supply()` (`ERC20CRV:340`), the sum of all gauges' integrals can never exceed the schedule — gauge weights sum to ≤ 1 by construction of `gauge_relative_weight`.

### 3.6 `FeeDistributor.vy` — the other half of every swap fee

Admin fees (50 % of all swap fees, §1.7/§2.3) are collected, converted to 3CRV (today: crvUSD in the newer distributor — same code shape) and sent to this contract. It distributes them **pro-rata to veCRV balance, per week**.

* `_checkpoint_token()` (`:99-127`): `to_distribute = balanceOf − token_last_balance`; spread it over the weeks since `last_token_time` proportionally to time elapsed (`:122`), into `tokens_per_week[week]`.
* `_checkpoint_total_supply()` (`:193-214`): for each week boundary up to now, ask the VotingEscrow for the global point and record `ve_supply[week] = bias − slope·dt` (`:210`).
* `ve_for_at(user, ts)` (`:178-190`): user's veCRV at a timestamp from their point history.
* `_claim(addr, ve, last_token_time)` (`:228-295`): walk the user's weeks from `time_cursor_of[addr]`, advancing through their `user_point_history` as needed, and add `balance_of · tokens_per_week[week] / ve_supply[week]` (`:283`). Capped at 50 weeks per call (`:266`) — call again if `claim_epoch < max_epoch`.
* `claim(addr)` (`:298-330`): checkpoints supply (and token if allowed, `:314`), then `_claim`, then `transfer`.

So: the DAO's fee share is not "revenue to a treasury" — it flows to whoever has CRV locked, weighted by lock length. That is the *yield* on veCRV, alongside boost and votes.

### 3.7 The flywheel

```
      provide liquidity ─▶ LP token ─▶ deposit in gauge ─▶ earn CRV  (rate × gauge weight × boosted share)
             ▲                                                │
             │ deeper pools, better prices, more volume        ▼
        fees to LPs (50 %)                              lock CRV → veCRV (up to 4 y)
             ▲                                                │
             │                                       ┌────────┼──────────────┐
             │                                       ▼        ▼              ▼
   swap fees (admin 50 %) ──▶ FeeDistributor ──▶ fee share   up-to-2.5× boost   vote gauge weights
                                                                                  │
                                                                                  ▼
                                                     more CRV flows to the pools veCRV holders care about
                                                     (→ protocols acquire veCRV / bribe voters: "Curve wars")
```

### 3.8 Trace: deposit 3pool LP in the gauge, wait a week, `Minter.mint`

Assume epoch-6 rate `r = 3.081e18` CRV/s, the 3pool gauge's relative weight for the week `w = 0.02e18` (2 %), gauge `working_supply = 10,000,000e18` after our deposit, user deposits `l = 100,000e18` 3CRV, has **no** veCRV. Times: `t0` = deposit (assume exactly a week boundary for simplicity), `t1 = t0 + 604800`.

```
t0  user.approve(gauge) ; gauge.deposit(100_000e18)                           LG:279
      _checkpoint(user)                                                        LG:153
         future_epoch check (no change) ; controller.checkpoint_gauge(gauge)   fills points_weight/points_total up to this week
         block.timestamp == period_timestamp → no integral advance
         integrate_fraction[user] += working_balances[user](=0) · … = 0
         integrate_inv_supply_of[user] = I0
      balanceOf[user] = 100_000e18 ; totalSupply = 10_000_000e18
      _update_liquidity_limit(user, 100_000e18, 10_000_000e18)                 LG:125
         voting_balance = 0 → lim = 0.4 · 100_000e18 = 40_000e18 ; working_supply += 40_000e18
      3CRV.transferFrom(user, gauge, 100_000e18)

t1  user.mint(gauge)  → Minter._mint_for(gauge, user)                          Minter:43
      gauge.user_checkpoint(user)                                              LG:223
         _checkpoint(user):
            controller.checkpoint_gauge(gauge)
            loop: one full week, w = gauge_relative_weight(gauge, t0) = 0.02e18
               ΔI = r · w · dt / working_supply
                  = 3.081e18 · 0.02e18 · 604800 / 10_000_000e18
                  = 3.727e15                          (1e18-scaled CRV per working-LP)
            I1 = I0 + 3.727e15
            integrate_fraction[user] += 40_000e18 · 3.727e15 / 1e18 = 149.1e18   → 149.1 CRV     LG:217
         _update_liquidity_limit(user, …)  (unchanged)
      total_mint = 149.1e18 ; to_mint = 149.1e18 − minted[user][gauge](=0)
      CRV.mint(user, 149.1e18)      (ERC20CRV:325; checks ≤ available_supply)
      minted[user][gauge] = 149.1e18
```

Check: the gauge as a whole received `3.081 · 604800 · 0.02 = 37,268` CRV this week; the user's working share is `40k / 10M = 0.4 %` → 149.1 CRV ✓. Had the user held ≥ 1 % of all veCRV (their `l/L` share), `working_balance = 100,000e18` and they would get 372.7 CRV — the 2.5× boost.

---

## 4. Curve vs Uniswap v2 / v3

| | Curve StableSwap | Uniswap v2 | Uniswap v3 |
|---|---|---|---|
| invariant | `A n^n Σx + D = A D n^n + D^(n+1)/(n^n Πx)` (1); one tunable `A` | `x·y = k` | `x·y = k` per tick range (`L² `), positions are ranges |
| coins per pool | 2–8 (NG) | 2 | 2 |
| price curve | flat near 1:1, steepens off-peg; solved numerically (Newton, §1.4–1.5) | hyperbola, closed form | piecewise hyperbola; closed form per tick, loop across ticks |
| capital efficiency for pegged pairs | ~50–100× v2 at A≈100 (§0.3) | 1× | comparable to Curve *if* LPs choose tight ranges (e.g. 0.999–1.001), but LPs must manage ranges; Curve's `A` does it for everyone |
| assets it suits | pegged / correlated (stables, LSTs, wrapped BTC) | anything | anything |
| fee | flat (0.04 %) or dynamic-by-imbalance (NG); 50 % to veCRV | 0.30 % flat, optional 1/6 to protocol | 0.01/0.05/0.30/1 % tiers, protocol fee switch |
| LP token | fungible ERC20 (`D/supply` = virtual price) | fungible ERC20 (`√k`-share) | NFT per position |
| oracle | NG: EMA of *state price* + EMA of `D` (§2.3); classic: `get_virtual_price` only | cumulative price (TWAP) | tick-cumulative TWAP + liquidity-cumulative |
| deposits | any combination, imbalance fee (§1.8) | must be proportional (or router swaps) | any amounts within the chosen range |
| liquidity incentives | protocol-level: gauges + veCRV (§3) | none in-protocol | none in-protocol |
| gas per swap | higher (2× Newton loops, 8-coin loops) | very low | medium (tick crossing) |

The deepest difference: Uniswap sells *price discovery*, Curve sells *depth at a known price*. Curve's `A` encodes a belief ("these things are worth the same"); when that belief breaks (UST, the March-2023 USDC weekend) Curve LPs are the ones who absorb the bad asset — hence the dynamic fee and the ability to ramp `A` down.

---

## 5. Security notes

### 5.1 Read-only reentrancy via `get_virtual_price` (ETH pools)

In pools that hold native ETH (e.g. `curve-contract/contracts/pools/steth/StableSwapSTETH.vy`, Vyper 0.2.8), `remove_liquidity` (`:477`) burns LP first (`:491`) then loops over coins, doing `self.balances[i] -= value` and, for ETH, `raw_call(msg.sender, b"", value=value)` (`:499`) — *before* `balances[1]` (stETH) has been decremented. A contract receiving that ETH can call `get_virtual_price()` (`:251`, **no** `@nonreentrant`): `totalSupply` is already reduced, `balances[0]` reduced, `balances[1]` not yet → `D / supply` is transiently **inflated**. Any protocol pricing the LP token off that view (lending markets accepting steCRV as collateral) could be tricked into over-lending. The attacker's own state is consistent; the *victim* is the third-party reader — hence "read-only". Mitigations: NG marks `get_virtual_price`, `totalSupply`, `price_oracle`, `D_oracle` as `@view @nonreentrant('lock')` (`CurveStableSwapNG.vy:1739`, `:1728`, `:1444`, `:1455`) so a reentrant read reverts; integrators of classic pools call a cheap state-changing function (e.g. `withdraw_admin_fees` or `remove_liquidity(0, …)`) first to trip the lock. NG also refuses native ETH entirely (factory `add_base_pool` rejects `0xEeee…`, `CurveStableSwapFactoryNG.vy:751`).

### 5.2 `A` ramp manipulation

Changing `A` re-prices the pool without any trade (§1.6). If `A` could jump, an attacker who knows a ramp is coming could position on one side and arbitrage LPs. Hence: linear ramp over ≥ 1 day, factor ≤ 10, ≥ 1 day between ramps, owner = DAO (`:702-717`). Even a slow ramp on a very imbalanced pool leaks value; `stop_ramp_A` is the emergency brake. Related: `MAX_A = 1e6` because at astronomically high `A` the Newton loops lose precision / the pool behaves as constant-sum and can be drained of the scarce coin at ~1:1.

### 5.3 The Vyper `@nonreentrant` compiler bug (30 July 2023)

Vyper **0.2.15, 0.2.16 and 0.3.0** emitted broken reentrancy locks when the same key was used on functions of different mutability. Affected Curve pools were ETH pools compiled with those versions: alETH/ETH, msETH/ETH, pETH/ETH and CRV/ETH (≈ $70 M drained via reentrant `add_liquidity` → `remove_liquidity`, using the read-only-reentrancy shape above but *writing* this time). None of the pools in this folder use those versions (`grep "@version"` over `curve-contract/contracts/pools/*/*.vy`: 0.1.0b16 → 0.2.12), and NG is 0.3.10. Lesson for you as a reader: `@nonreentrant('lock')` is only as good as the compiler; check the pragma.

### 5.4 Other things to keep in mind

* **Admin fee / fee receiver**: NG `_withdraw_admin_fees` sends to `factory.fee_receiver()`; a malicious factory admin could redirect fees, but not LP funds. The 50 % `admin_fee` is a constant in NG (`:171`), so the DAO cannot raise it on existing pools.
* **Oracle manipulation**: `get_p` (spot) is trivially movable within a block; only `price_oracle`/`D_oracle` (EMAs) are meant for consumption, and even those can be drifted over a window by sustained capital — use the window (`ma_exp_time`) as a cost-of-attack parameter. `last_price` is capped at 2e18 (`:1361`).
* **Rate oracles (asset type 1)**: `_stored_rates` trusts an external `raw_call` result unconditionally (`:449-458`); the header warns "Oracles may be controlled externally by an EOA". A lying rate oracle == a lying price.
* **Rebasing tokens (type 2)**: balances are read live; positive rebases go to LPs, negative (slashing) hit LPs; `exchange_received` disabled; admin fees immune (`:473-499` docstring).
* **First deposit / donation**: first LP receives exactly `D1` tokens (`:342` classic / `:660` NG) so virtual price starts at 1; there is no ERC4626-style share inflation attack because `D` — not `balanceOf` — is the numeraire, and donated tokens outside `stored_balances` are simply admin-claimable (classic) or ignored (NG stored accounting). The NG docstring still flags ERC4626 underlyings as a donation risk (`:22-24`).
* **`min_dy` / `min_mint_amount` / `max_burn_amount`** are the only slippage protection; every mutating function has one. Front-running a large `remove_liquidity_one_coin` is the classic sandwich.
* **Killed pools** (classic): `remove_liquidity` always works; everything else reverts (`:271`, `:432`, `:531`, `:675`).

---

## 6. Exercises

1. **Reproduce the trace in §1.11** in Python: port `get_D` (`curve-contract/contracts/pools/3pool/StableSwap3Pool.vy:195-219`) and `get_y` (`:356-398`) verbatim with integer division, then confirm `dy = 999,422,408` for the given state. Then change `A` to 200 and 20000 and watch `dy` move.
2. **Derive the Newton step** at `:210` yourself starting from equation (1), and the one at `:389` from `y² + (b−D)y − c = 0`. Explain why `c` is built with `N_COINS` multiplications inside the loop plus one more at `:383`.
3. **Explain why `remove_liquidity_one_coin` calls `get_y_D` twice** (`:646` and `:657`). What would go wrong if you charged the fee on `dy_0` directly instead?
4. **Dynamic fee**: in `stableswap-ng/contracts/main/CurveStableSwapNG.vy:887-901`, compute `fee_dyn / fee` for `m = 5` at balance ratios 1:1, 2:1, 10:1, 100:1. Then find where the *same* function is applied in `add_liquidity` (`:634-642`) and explain what `xs` and `ys` represent there.
5. **Oracle**: starting from `_calc_moving_average` (`:1393-1412`), show that with `ma_exp_time = 866` a spot price that jumps and stays has its EMA halfway there after ~600 s. Why does `price_oracle` (`:1445`) not need a transaction to be up to date?
6. **Metapool**: trace `exchange_underlying(1, 3, …)` (DAI → USDT) through `CurveStableSwapMetaNG.vy:659-751` and `_transfer_in` (`:423-489`). Which of the three cases in §2.5 is it, and how many external calls does it make?
7. **veCRV**: with `VotingEscrow.vy:234-348` open, walk through what `slope_changes` looks like after Alice locks 1000 CRV for 2 years and Bob locks 400 CRV for 1 year in the same week; then compute `totalSupply(t)` (`:626`) at `t = now + 1 year + 1 day` by hand via `supply_at` (`:597-623`).
8. **Gauge**: in `curve-dao-contracts/contracts/gauges/LiquidityGauge.vy:153-220`, explain what happens to a user who deposited and never touched the gauge for 3 years (hint: the `range(500)` loop, `prev_future_epoch` handling at `:190-196`, and the comment "the gauge gets less"). Then find the equivalent in `stableswap-ng/contracts/main/LiquidityGauge.vy:225-296` and list what V6 changed.
