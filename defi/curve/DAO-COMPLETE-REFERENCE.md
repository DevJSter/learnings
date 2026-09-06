# Curve DAO — Complete Contract Reference

Exhaustive, function-by-function reference for **every** Vyper file in
`curve-dao-contracts/contracts/` — 68 files, 19,366 lines.

This is the *reference*. The conceptual walkthrough lives in
[`CURVE-DEEP-DIVE.md`](CURVE-DEEP-DIVE.md) (§3 covers the DAO). Read that to
understand the system; read this to have seen all of it. Every `file:line`
below was verified with `grep -n` against the tree in this folder. Paths are
relative to `curve/curve-dao-contracts/`.

---

## Table of contents

| § | Contract / group | File | Ver | Lines |
|---|---|---|---|---|
| [1](#1-erc20crvvy--the-crv-token) | **ERC20CRV** — the CRV token | `contracts/ERC20CRV.vy` | 0.2.4 | 374 |
| [2](#2-votingescrowvy--vecrv) | **VotingEscrow** — veCRV | `contracts/VotingEscrow.vy` | 0.2.4 | 671 |
| [3](#3-gaugecontrollervy) | **GaugeController** | `contracts/GaugeController.vy` | 0.2.4 | 596 |
| [4](#4-the-gauge-family) | **Gauges** ×7 | `contracts/gauges/*.vy` | 0.2.4–0.3.1 | 4,372 |
| [5](#5-mintervy) | **Minter** | `contracts/Minter.vy` | 0.2.4 | 99 |
| [6](#6-feedistributorvy) | **FeeDistributor** | `contracts/FeeDistributor.vy` | 0.2.7 | 466 |
| [7](#7-the-proxy-admin-layer) | **Proxies** ×4 | `PoolProxy`, `CryptoPoolProxy`, `PoolProxySidechain`, `GaugeProxy` | 0.2.7–0.2.8 | 1,534 |
| [8](#8-vesting) | **Vesting** ×3 | `contracts/vests/*.vy` | 0.2.4 | 623 |
| [9](#9-streamers) | **Streamers** ×3 | `contracts/streamers/*.vy` | 0.2.12–0.2.16 | 482 |
| [10](#10-sidechain-root-gauges--wrappers) | **Sidechain gauges & wrappers** ×9 | `gauges/sidechain/`, `gauges/wrappers/` | 0.2.8–0.2.16 | 2,062 |
| [11](#11-the-burner-family) | **Burners** ×30 | `contracts/burners/**` | 0.2.7–0.3.7 | 6,441 |
| [12](#12-bridging) | **Bridging** ×3 | `contracts/bridging/*.vy` | 0.3.0 | 236 |
| [13](#13-crvinfovy) | **CRVInfo** | `contracts/CRVInfo.vy` | 0.3.7 | 100 |
| [14](#14-testing-helpers) | **Testing helpers** ×3 | `contracts/testing/*.vy` | — | 981 |
| [15](#15-the-flywheel-end-to-end) | The flywheel, end to end | — | — | — |
| [16](#16-use-case-index) | Use-case index | — | — | — |
| [17](#17-events-reference) | Events reference | — | — | — |
| [18](#18-revert-string-decoder) | Revert-string decoder | — | — | — |
| [19](#19-storage-layout-tables) | Storage layout tables | — | — | — |
| [20](#20-selector-tables) | Selector tables | — | — | — |

**Conventions used throughout.** `WEEK = 604800`. All "fixed point" values are
`1e18`-scaled unless stated. `@nonreentrant('lock')` is Vyper's storage-flag
mutex — see [§18](#18-revert-string-decoder) for the July-2023 compiler caveat.
Curve's ownership pattern is two-step everywhere: `commit_*` stores a
`future_*`, then `apply_*`/`accept_*` promotes it. Which of the two names is
used is inconsistent across contracts and is called out per contract.

---

## 1. `ERC20CRV.vy` — the CRV token

`contracts/ERC20CRV.vy`, Vyper 0.2.4, 374 lines, `implements: ERC20`.

A plain ERC-20 with one twist: **supply is not fixed and not arbitrary**. A
single `minter` address may mint, but only up to a hard ceiling that grows
along a published piecewise-linear schedule. The contract is the *rate limiter*
on emissions; the GaugeController decides who gets them.

### 1.1 Constants and the schedule (`:50-67`)

| Constant | Line | Value | Meaning |
|---|---|---|---|
| `YEAR` | `:50` | `86400*365` | 31,536,000 s |
| `INITIAL_SUPPLY` | `:62` | `1_303_030_303` | premined whole tokens (43%) |
| `INITIAL_RATE` | `:63` | `274_815_283e18 / YEAR` = `8714335457889396245` | wei per second, year 0 |
| `RATE_REDUCTION_TIME` | `:64` | `YEAR` | epoch length |
| `RATE_REDUCTION_COEFFICIENT` | `:65` | `1189207115002721024` | 2^(1/4)·1e18 |
| `RATE_DENOMINATOR` | `:66` | `1e18` | |
| `INFLATION_DELAY` | `:67` | `86400` | 1 day before mining may start |

Each epoch the rate is divided by 2^(1/4), so it **halves every four years**:

```
rate_n = INITIAL_RATE / 2^(n/4)
```

Computed from the real constants (integer division as on-chain):

| Epoch | rate (wei/s) | CRV emitted that year | Supply at epoch start | Annual inflation |
|---:|---:|---:|---:|---:|
| 0 | 8714335457889396245 | 274,815,283 | 1,303,030,303 | 21.09% |
| 1 | 7327853447857530670 | 231,091,186 | 1,577,845,586 | 14.65% |
| 2 | 6161965695807970181 | 194,323,750 | 1,808,936,772 | 10.74% |
| 3 | 5181574864521283150 | 163,406,145 | 2,003,260,523 | 8.16% |
| 4 | 4357167728944698747 | 137,407,642 | 2,166,666,667 | 6.34% |
| 5 | 3663926723928765860 | 115,545,593 | 2,304,074,309 | 5.01% |
| 6 | 3080982847903985532 | 97,161,875 | 2,419,619,902 | 4.02% |
| 7 | 2590787432260641946 | 81,703,072 | 2,516,781,777 | 3.25% |
| 8 | 2178583864472349685 | 68,703,821 | 2,598,484,850 | 2.64% |
| 9 | 1831963361964383192 | 57,772,797 | 2,667,188,670 | 2.17% |
| 10 | 1540491423951992986 | 48,580,938 | 2,724,961,467 | 1.78% |

Summing the geometric series to exhaustion gives 1,727,272,728 CRV ever
emitted, for an **asymptotic total supply of 3,030,303,030 CRV** and a premine
share of exactly 0.43 — which is what the comment on `:63` claims.

Storage: `name/symbol/decimals` (`:38-40`), `balanceOf` (`:42`), `allowances`
(`:43`, private), `total_supply` (`:44`, private — exposed via the explicit
`totalSupply()` at `:252`), `minter` (`:46`), `admin` (`:47`), `mining_epoch`
(`:70`), `start_epoch_time` (`:71`), `rate` (`:72`), `start_epoch_supply`
(`:74`, private).

### 1.2 `__init__(_name, _symbol, _decimals)` — `:78`

Mints `INITIAL_SUPPLY * 10**decimals` to the deployer (`:89-90`), sets
`admin = msg.sender` (`:91`), logs `Transfer(0x0, deployer, init_supply)`
(`:92`).

The clever bit is `:94`:

```python
self.start_epoch_time = block.timestamp + INFLATION_DELAY - RATE_REDUCTION_TIME
self.mining_epoch = -1
self.rate = 0
```

`start_epoch_time` is set one *year minus one day* in the past, `mining_epoch`
to −1 and `rate` to 0. So the first call to `_update_mining_parameters` becomes
legal exactly `INFLATION_DELAY` (1 day) after deployment, and it moves the
system into epoch 0 at the initial rate. Until then `rate == 0`, so
`_available_supply()` equals the premine and `mint` can only fail.

### 1.3 `_update_mining_parameters()` `@internal` — `:101`

Advances one epoch. Order matters:

1. Caches `_rate` and `_start_epoch_supply` (`:106-107`).
2. `start_epoch_time += RATE_REDUCTION_TIME`; `mining_epoch += 1` (`:109-110`).
3. If `_rate == 0` (first ever call) → `_rate = INITIAL_RATE` (`:112-113`),
   and `start_epoch_supply` is left at the premine.
4. Otherwise credit the whole finished epoch's emission to
   `start_epoch_supply` (`:115-116`) *then* reduce the rate (`:117`):
   `_rate = _rate * 1e18 / RATE_REDUCTION_COEFFICIENT`.
5. Writes `self.rate` (`:119`), logs `UpdateMiningParameters` (`:121`).

**Gotcha:** the double integer division in step 4 (multiply by `RATE_DENOMINATOR`,
divide by the coefficient) rounds the rate *down* every epoch. The comment at
`mintable_in_timeframe:219` calls this out explicitly — "double-division with
rounding made rate a bit less => good". Rounding against the emitters is the
safe direction.

**Gotcha 2:** `update_mining_parameters` is permissionless and the docstring at
`:129` warns "Total supply becomes slightly larger if this function is called
late". Because `start_epoch_time` advances by exactly one `RATE_REDUCTION_TIME`
per call rather than jumping to now, a late call leaves the old (higher) rate
applying for the extra elapsed time.

### 1.4 The epoch accessors

| Function | Line | Access | Behaviour |
|---|---|---|---|
| `update_mining_parameters()` | `:125` | any | `assert block.timestamp >= start_epoch_time + RATE_REDUCTION_TIME  # dev: too soon!` (`:131`), then `_update_mining_parameters()` |
| `start_epoch_time_write() -> uint256` | `:136` | any | Rolls the epoch if due (`:143-145`), returns current `start_epoch_time` |
| `future_epoch_time_write() -> uint256` | `:151` | any | Same, but returns `start_epoch_time + RATE_REDUCTION_TIME` |

`future_epoch_time_write` is the one every gauge calls in its constructor and
in `_checkpoint` (e.g. `LiquidityGaugeV5.vy:170`, `:291`) — the gauge needs to
know *when the rate will next change* so it can split its integral at that
boundary.

### 1.5 Supply views

**`_available_supply() -> uint256` `@internal @view` — `:167`**

```python
return self.start_epoch_supply + (block.timestamp - self.start_epoch_time) * self.rate
```

The ceiling: everything credited in completed epochs plus linear accrual so far
this epoch. `available_supply()` (`:173`) is the external wrapper.

**`mintable_in_timeframe(start, end) -> uint256` `@external @view` — `:182`**

How much CRV *may* be minted in `[start, end]`. Walks epochs **backwards**.

- `assert start <= end  # dev: start > end` (`:189`).
- If `end` is past the current epoch's end, step one epoch forward in memory
  first (`:195-197`) — this lets you query one epoch into the future without
  having called `update_mining_parameters`.
- `assert end <= current_epoch_time + RATE_REDUCTION_TIME  # dev: too far in future` (`:199`) — you may look at most one un-started epoch ahead.
- Loop `range(999)` (`:201`) clamping `[current_start, current_end]` to the
  epoch window and accumulating `current_rate * (current_end - current_start)`
  (`:213`), then stepping the epoch *back* and multiplying the rate back up
  (`:218-219`).
- `assert current_rate <= INITIAL_RATE  # This should never happen` (`:220`) is
  a paranoia rail against the multiply-back overshooting past epoch 0.

The comment on `:201` — *"Curve will not work in 1000 years. Darn!"* — is the
loop bound.

### 1.6 Admin

| Function | Line | Guard | Notes |
|---|---|---|---|
| `set_minter(_minter)` | `:226` | `msg.sender == admin` (`:232`), `minter == ZERO_ADDRESS` (`:233`) | **One-shot.** Once the Minter is wired it can never be changed. This is the single most important immutability guarantee in the DAO. |
| `set_admin(_admin)` | `:239` | `msg.sender == admin` (`:245`) | One-step, no `future_admin`. After setup the admin can *only* rename the token. |
| `set_name(_name, _symbol)` | `:365` | `msg.sender == admin`, `"Only admin is allowed to change name"` (`:372`) | Cosmetic only |

### 1.7 ERC-20 surface

`totalSupply()` `:252`, `allowance()` `:261`, `transfer()` `:272`,
`transferFrom()` `:289`, `approve()` `:308`, `mint()` `:325`, `burn()` `:350`.

**`approve(_spender, _value)` `:308`** carries the 2020-era race-condition
guard at `:318`:

```python
assert _value == 0 or self.allowances[msg.sender][_spender] == 0
```

You must zero an allowance before setting a new non-zero one. Many integrations
trip over this.

**`mint(_to, _value) -> bool` `:325`** — the only inflation path.

1. `assert msg.sender == self.minter  # dev: minter only` (`:333`)
2. `assert _to != ZERO_ADDRESS  # dev: zero address` (`:334`)
3. Rolls the epoch if due (`:336-337`) — so minting never needs a separate
   keeper call
4. `assert _total_supply <= self._available_supply()  # dev: exceeds allowable mint amount` (`:340`) — **the ceiling check**
5. Writes `total_supply` and `balanceOf[_to]`, logs `Transfer(0x0, _to, _value)`

**`burn(_value) -> bool` `:350`** — permissionless self-burn; reduces
`total_supply` (`:358`). Note burning does **not** raise the mint ceiling,
because `_available_supply()` is computed from `start_epoch_supply` and elapsed
time, not from `total_supply`. Burned CRV is gone.

---

## 2. `VotingEscrow.vy` — veCRV

`contracts/VotingEscrow.vy`, Vyper 0.2.4, 671 lines.

Lock CRV for up to 4 years; receive a **non-transferable, linearly decaying**
balance called veCRV. This is the original vote-escrow contract that the entire
"ve" design space was copied from.

### 2.1 The model

A lock is `(amount, end)`. Voting power at time `t`:

```
veCRV(t) = amount * (end - t) / MAXTIME        for t < end,  else 0
```

That is a straight line hitting zero at `end`. The contract stores it as a
**bias/slope pair** rather than recomputing:

```
slope = amount / MAXTIME          (constant, the decay rate)
bias  = slope * (end - now)       (the value right now)
```

so `veCRV(t) = bias - slope*(t - now)`. Locking 1000 CRV for 4 years gives 1000
veCRV; for 2 years, 500; for 1 year, 250.

Summing a straight line over all users is itself a straight line — **until a
lock expires**, at which point the total's slope must drop by that user's
slope. That is the entire difficulty of this contract, and it is solved by
`slope_changes`.

### 2.2 Types, constants, storage

```python
struct Point:                     # :25
    bias: int128
    slope: int128                 # - dweight / dt
    ts: uint256
    blk: uint256                  # block

struct LockedBalance:             # :34
    amount: int128
    end: uint256
```

| Constant | Line | Value |
|---|---|---|
| `WEEK` | `:84` | `7*86400` — all future times rounded down to weeks |
| `MAXTIME` | `:85` | `4*365*86400` = 126,144,000 s |
| `MULTIPLIER` | `:86` | `1e18` |
| `DEPOSIT_FOR_TYPE / CREATE_LOCK_TYPE / INCREASE_LOCK_AMOUNT / INCREASE_UNLOCK_TIME` | `:55-58` | `0 / 1 / 2 / 3` — the `type` field in the `Deposit` event |

| Storage | Line | Meaning |
|---|---|---|
| `token` | `:88` | CRV |
| `supply` | `:89` | total CRV locked (real tokens, not veCRV) |
| `locked[addr]` | `:91` | `LockedBalance` |
| `epoch` | `:93` | index of the newest global point |
| `point_history[epoch]` | `:94` | global `Point` history |
| `user_point_history[addr][uepoch]` | `:95` | per-user `Point` history |
| `user_point_epoch[addr]` | `:96` | per-user epoch counter |
| `slope_changes[ts]` | `:97` | signed slope delta scheduled at week boundary `ts` |
| `controller`, `transfersEnabled` | `:100-101` | Aragon compatibility shims |
| `name/symbol/version/decimals` | `:103-106` | |
| `smart_wallet_checker`, `future_smart_wallet_checker` | `:110-111` | contract whitelist |
| `admin`, `future_admin` | `:113-114` | |

The array dimensions on `:94-95` (`Point[1e29]`, `Point[1e9]`) are Vyper's way
of declaring a practically-unbounded mapping; no storage is pre-allocated.

### 2.3 `__init__(token_addr, _name, _symbol, _version)` — `:118`

Sets `admin`/`controller` to the deployer, seeds `point_history[0]` with the
current block and timestamp (`:128-129`), reads `decimals` from CRV and
`assert _decimals <= 255` (`:134`).

### 2.4 Why non-transferable, and `assert_not_contract`

There is **no** `transfer`, `transferFrom` or `approve` in this contract. veCRV
cannot move. If it could, the lock would be tokenizable and the whole
commitment mechanism would collapse into a liquid governance token.

To stop wrappers rebuilding transferability, `assert_not_contract(addr)`
`@internal` (`:185`) rejects any caller that is not `tx.origin` unless a
`SmartWalletChecker` approves it:

```python
if addr != tx.origin:
    checker: address = self.smart_wallet_checker
    if checker != ZERO_ADDRESS:
        if SmartWalletChecker(checker).check(addr):
            return
    raise "Smart contract depositors not allowed"
```

Called from `create_lock` (`:418`), `increase_amount` (`:438`) and
`increase_unlock_time` (`:455`) — but **deliberately not** from `deposit_for`
(`:393`), because topping up someone else's existing lock cannot create a new
tokenized position.

The whitelist is itself two-step: `commit_smart_wallet_checker(addr)` (`:166`)
and `apply_smart_wallet_checker()` (`:176`), both `assert msg.sender == self.admin`.

Ownership: `commit_transfer_ownership(addr)` (`:143`) / `apply_transfer_ownership()`
(`:154`). Note `apply_` is guarded by `msg.sender == self.admin` (`:158`), not
by the *future* admin — the current admin promotes, the future admin does not
accept. `changeController(_newController)` (`:666`) is a dead Aragon shim.

### 2.5 `_checkpoint(addr, old_locked, new_locked)` `@internal` — `:234`

The heart of the contract. Called by `_deposit_for` (`:374`), `withdraw`
(`:488`) and the public `checkpoint()` (`:388`, with `addr = ZERO_ADDRESS`).

**Phase 1 — user slopes (`:247-265`), skipped when `addr == 0x0`.**

```python
if old_locked.end > block.timestamp and old_locked.amount > 0:
    u_old.slope = old_locked.amount / MAXTIME
    u_old.bias  = u_old.slope * convert(old_locked.end - block.timestamp, int128)
if new_locked.end > block.timestamp and new_locked.amount > 0:
    u_new.slope = new_locked.amount / MAXTIME
    u_new.bias  = u_new.slope * convert(new_locked.end - block.timestamp, int128)
```

Expired or empty locks give a zero `Point`. Then it reads the currently
scheduled slope changes at both ends (`:260-265`), reusing `old_dslope` when
the two ends coincide.

**Phase 2 — replay history week by week (`:267-308`).**

`block_slope` (`:277`) is `1e18 * Δblock / Δtime` since the last point: an
estimate of blocks-per-second used to stamp an approximate block number onto
each synthesised historical point, because `balanceOfAt`/`totalSupplyAt` are
queried by block but the maths is in time.

```python
t_i: uint256 = (last_checkpoint / WEEK) * WEEK
for i in range(255):
    t_i += WEEK
    d_slope: int128 = 0
    if t_i > block.timestamp:
        t_i = block.timestamp
    else:
        d_slope = self.slope_changes[t_i]
    last_point.bias -= last_point.slope * convert(t_i - last_checkpoint, int128)
    last_point.slope += d_slope
    if last_point.bias < 0: last_point.bias = 0
    if last_point.slope < 0: last_point.slope = 0
    ...
    _epoch += 1
    if t_i == block.timestamp:
        last_point.blk = block.number
        break
    else:
        self.point_history[_epoch] = last_point
```

One iteration per week since the last checkpoint: decay the bias across the
week, then apply that week's scheduled slope change. 255 iterations ≈ **4.9
years**; the comment at `:284` accepts that if nobody touches the contract for
five years the vote weight breaks (withdrawals still work). The clamps at
`:294-297` are defensive; `bias < 0` genuinely happens from integer truncation.

Note the loop writes each intermediate week to `point_history` but the final
point is written once after the loop at `:322` — that is why `_epoch` is
incremented inside and assigned to `self.epoch` at `:308`.

**Phase 3 — apply the user's delta (`:311-319`).**

```python
last_point.slope += (u_new.slope - u_old.slope)
last_point.bias  += (u_new.bias  - u_old.bias)
```

**Phase 4 — reschedule slope changes (`:324-347`).**

```python
if old_locked.end > block.timestamp:
    old_dslope += u_old.slope                 # cancel the old drop
    if new_locked.end == old_locked.end:
        old_dslope -= u_new.slope             # re-add at the same date
    self.slope_changes[old_locked.end] = old_dslope

if new_locked.end > block.timestamp:
    if new_locked.end > old_locked.end:
        new_dslope -= u_new.slope             # schedule the new drop
        self.slope_changes[new_locked.end] = new_dslope
```

Slope changes are stored **negative** (the total slope falls when a lock
expires). Cancelling an old schedule therefore *adds* the slope back. The
`new_locked.end == old_locked.end` branch handles a pure `increase_amount`,
where the date does not move and both operations collapse into one write.

Finally the user's own point is appended (`:342-347`) with
`user_point_epoch[addr] += 1`.

### 2.6 `_deposit_for(_addr, _value, unlock_time, locked_balance, type)` `@internal` — `:351`

1. `self.supply = supply_before + _value` (`:362`).
2. Mutate the lock in memory: `_locked.amount += _value`, and `_locked.end =
   unlock_time` **only if `unlock_time != 0`** (`:365-367`), which is how the
   four entry points share one routine.
3. Write `self.locked[_addr]` (`:368`).
4. `self._checkpoint(_addr, old_locked, _locked)` (`:374`).
5. `assert ERC20(self.token).transferFrom(_addr, self, _value)` **after** the
   checkpoint (`:376-377`).
6. `log Deposit(...)`, `log Supply(...)` (`:379-380`).

Note the transfer pulls from `_addr`, not `msg.sender`. For `deposit_for` this
means **the beneficiary must have approved this contract**, not the caller.

### 2.7 The four entry points

All are `@external @nonreentrant('lock')`.

| Function | Line | Asserts | `type` |
|---|---|---|---|
| `deposit_for(_addr, _value)` | `:393` | `_value > 0  # dev: need non-zero value` (`:403`); `_locked.amount > 0, "No existing lock found"` (`:404`); `_locked.end > block.timestamp, "Cannot add to expired lock. Withdraw"` (`:405`) | `0` |
| `create_lock(_value, _unlock_time)` | `:412` | `assert_not_contract`; `_value > 0` (`:422`); `_locked.amount == 0, "Withdraw old tokens first"` (`:423`); `unlock_time > block.timestamp, "Can only lock until time in the future"` (`:424`); `unlock_time <= block.timestamp + MAXTIME, "Voting lock can be 4 years max"` (`:425`) | `1` |
| `increase_amount(_value)` | `:432` | `assert_not_contract`; `_value > 0` (`:441`); `"No existing lock found"` (`:442`); `"Cannot add to expired lock. Withdraw"` (`:443`) | `2` |
| `increase_unlock_time(_unlock_time)` | `:450` | `assert_not_contract`; `_locked.end > block.timestamp, "Lock expired"` (`:459`); `_locked.amount > 0, "Nothing is locked"` (`:460`); `unlock_time > _locked.end, "Can only increase lock duration"` (`:461`); `unlock_time <= block.timestamp + MAXTIME, "Voting lock can be 4 years max"` (`:462`) | `3` |

`create_lock` and `increase_unlock_time` both round the target **down** to a
week boundary: `unlock_time = (_unlock_time / WEEK) * WEEK` (`:419`, `:457`).
This is what makes `slope_changes` a sparse weekly map rather than a
per-second one. Consequence: asking for exactly 4 years usually yields slightly
under 4 years of power, because the rounding moves `end` backwards by up to a
week while `MAXTIME` in the denominator is unchanged.

**`withdraw()` `:469`** — `assert block.timestamp >= _locked.end, "The lock
didn't expire"` (`:475`), zeroes the lock, decrements `supply`, checkpoints
(`:488`), then transfers (`:490`). There is no early exit and no penalty: the
only way out is to wait.

**`checkpoint()` `:384`** — permissionless global-history advance with
`ZERO_ADDRESS`. `FeeDistributor._checkpoint_total_supply` calls it (`:197`).

### 2.8 Balance and supply queries

| Function | Line | Kind |
|---|---|---|
| `get_last_user_slope(addr) -> int128` | `:200` | view; used by `GaugeController.vote_for_gauge_weights` (`:492`) |
| `user_point_history__ts(_addr, _idx) -> uint256` | `:212` | view; used by every gauge's `kick` |
| `locked__end(_addr) -> uint256` | `:224` | view; used by `GaugeController` (`:493`) |
| `balanceOf(addr, _t = block.timestamp)` | `:525` | view |
| `balanceOfAt(addr, _block)` | `:546` | view |
| `totalSupply(t = block.timestamp)` | `:626` | view |
| `totalSupplyAt(_block)` | `:639` | view |
| `find_block_epoch(_block, max_epoch)` `@internal` | `:502` | binary search |
| `supply_at(point, t)` `@internal` | `:597` | week-walk |

**`balanceOf` `:525`** takes the user's latest point and decays it linearly:

```python
last_point.bias -= last_point.slope * convert(_t - last_point.ts, int128)
if last_point.bias < 0: last_point.bias = 0
```

Because a *user's* line has no slope changes of its own, no loop is needed.

**`balanceOfAt(addr, _block)` `:546`** is the MiniMe interface and does real
work. `assert _block <= block.number` (`:556`). Then:

1. Binary-search the user's point history by **block** (`:559-568`, inlined
   rather than calling `find_block_epoch` because "Vyper cannot pass by
   reference yet", `:554`).
2. Binary-search the *global* history for the same block via `find_block_epoch`
   (`:573`).
3. **Interpolate a timestamp for `_block`** (`:575-586`):

```python
if _epoch < max_epoch:
    point_1 = self.point_history[_epoch + 1]
    d_block = point_1.blk - point_0.blk
    d_t     = point_1.ts  - point_0.ts
else:
    d_block = block.number    - point_0.blk
    d_t     = block.timestamp - point_0.ts
block_time = point_0.ts
if d_block != 0:
    block_time += d_t * (_block - point_0.blk) / d_block
```

Linear interpolation between the two bracketing global points converts a block
number into an approximate timestamp; the user's bias is then decayed to that
time (`:588`). This is why every `Point` carries **both** `ts` and `blk`.

**`supply_at(point, t)` `@internal @view` `:597`** replays the weekly loop of
`_checkpoint` read-only — same `range(255)`, same `slope_changes` application
(`:606-617`) — and clamps negatives (`:619-620`).

**`totalSupply(t)` `:626`** = `supply_at(point_history[epoch], t)`.

**`totalSupplyAt(_block)` `:639`** finds the bracketing epoch, computes a `dt`
offset by block interpolation (`:651-657`), then calls `supply_at(point,
point.ts + dt)`.

### 2.9 Worked example

Alice locks 1000 CRV for 2 years; Bob locks 400 CRV for 1 year, same block,
`t = 0`.

```
MAXTIME = 126,144,000 s
Alice: end = 62,899,200 (day 728, week-rounded)  slope = 7,927,447,995,941   bias = 498.63 veCRV
Bob:   end = 31,449,600 (day 364, week-rounded)  slope = 3,170,979,198,376   bias =  99.73 veCRV

slope_changes[62,899,200] = −7,927,447,995,941
slope_changes[31,449,600] = −3,170,979,198,376
```

The idealised values are 500 and 100; the shortfall is the week-rounding of
`end`.

| Day | Alice | Bob | Total |
|---:|---:|---:|---:|
| 0 | 498.63 | 99.73 | 598.36 |
| 180 | 375.34 | 50.41 | 425.75 |
| 364 | 249.32 | 0.00 | 249.32 |
| 365 | 248.63 | 0.00 | 248.63 |
| 500 | 156.16 | 0.00 | 156.16 |
| 729 | 0.00 | 0.00 | 0.00 |

At day 364 the global point's slope drops by Bob's slope. The *total* line is
piecewise linear with a kink at each expiry — exactly what `slope_changes`
encodes, and why `supply_at` must walk weeks instead of using one subtraction.

---

## 3. `GaugeController.vy`

`contracts/GaugeController.vy`, Vyper 0.2.4, 596 lines.

Decides **what fraction of the weekly CRV emission each gauge receives**, by
veCRV vote. It never touches tokens.

### 3.1 The model

Every quantity here is a bias/slope pair in *week buckets*, mirroring
`VotingEscrow` — because a vote is backed by veCRV, and veCRV decays. A user
voting 40% of their power onto a gauge contributes a line that decays at
`0.40 × (their ve slope)` and hits zero when their lock ends.

Three levels of aggregation, each with its own history and its own catch-up
routine:

```
points_weight[gauge][t]  ── per-gauge bias/slope       ← _get_weight
points_sum[type][t]      ── sum over gauges of a type  ← _get_sum
points_total[t]          ── Σ_type (sum_type × type_weight)  ← _get_total
points_type_weight[type][t] ── DAO-set multiplier per type ← _get_type_weight
```

and the payout share is

```
relative_weight(gauge, t) = 1e18 × type_weight[type][t] × weight[gauge][t] / total[t]
```

### 3.2 Constants, types, storage

| Constant | Line | Value |
|---|---|---|
| `WEEK` | `:11` | `604800` |
| `WEIGHT_VOTE_DELAY` | `:14` | `10 * 86400` — 10 days between votes **per gauge** |
| `MULTIPLIER` | `:66` | `1e18` |

```python
struct Point:          # :17
    bias: uint256
    slope: uint256

struct VotedSlope:     # :21
    slope: uint256
    power: uint256     # bps, 0..10000
    end: uint256
```

Note these are **unsigned** here, unlike `VotingEscrow`'s `int128` — the
controller only ever handles non-negative aggregates.

| Storage | Line | Meaning |
|---|---|---|
| `admin`, `future_admin` | `:68-69` | |
| `token`, `voting_escrow` | `:71-72` | CRV, veCRV |
| `n_gauge_types`, `n_gauges` | `:76-77` | counters |
| `gauge_type_names[int128]` | `:78` | |
| `gauges[1e9]` | `:81` | enumeration array |
| `gauge_types_[addr]` | `:85` | **stored +1** so 0 means "not registered" |
| `vote_user_slopes[user][gauge]` | `:87` | `VotedSlope` |
| `vote_user_power[user]` | `:88` | total bps used, ≤ 10000 |
| `last_user_vote[user][gauge]` | `:89` | cooldown timestamp |
| `points_weight[gauge][t]`, `changes_weight[gauge][t]`, `time_weight[gauge]` | `:97-99` | per-gauge |
| `points_sum[type][t]`, `changes_sum[type][t]`, `time_sum[1e9]` | `:101-103` | per-type |
| `points_total[t]`, `time_total` | `:105-106` | global |
| `points_type_weight[type][t]`, `time_type_weight[1e9]` | `:108-109` | type multipliers |

The `+1` offset on `gauge_types_` (`:83-85`) is why `gauge_types(_addr)`
(`:153`) asserts `gauge_type != 0` (`:160`) and returns `gauge_type - 1`
(`:162`). `Minter._mint_for` relies on this reverting for unregistered gauges
(`Minter.vy:44`).

`__init__(_token, _voting_escrow)` `:113` asserts both non-zero (`:119-120`)
and sets `time_total = block.timestamp / WEEK * WEEK` (`:125`).

Ownership: `commit_transfer_ownership` `:129` / `apply_transfer_ownership`
`:140`, both `assert msg.sender == self.admin  # dev: admin only`.

### 3.3 The four catch-up routines

All four have the same shape: walk forward in `WEEK` steps from the last
recorded time, filling in history, stopping once past `block.timestamp`. All
are `@internal` and **state-changing** — they memoise.

**`_get_type_weight(gauge_type) -> uint256` `:166`.** Type weights do not
decay; the loop just copies the last value forward (`:176-182`). Returns 0 if
never initialised (`:185`).

**`_get_sum(gauge_type) -> uint256` `:189`.** Decays a bias/slope pair week by
week (`:199-213`):

```python
d_bias: uint256 = pt.slope * WEEK
if pt.bias > d_bias:
    pt.bias -= d_bias
    d_slope: uint256 = self.changes_sum[gauge_type][t]
    pt.slope -= d_slope
else:
    pt.bias = 0
    pt.slope = 0
```

The `else` branch is the unsigned-arithmetic guard: rather than underflow, a
line that would go negative is pinned to zero **and its slope is zeroed too**.

**`_get_weight(gauge_addr) -> uint256` `:259`.** Identical to `_get_sum` but
keyed by gauge address (`:269-283`).

**`_get_total() -> uint256` `:220`.** The composite.

```python
t: uint256 = self.time_total
if t > block.timestamp:
    t -= WEEK                      # :228-230
for gauge_type in range(100):      # :233-237  refresh every type first
    if gauge_type == _n_gauge_types: break
    self._get_sum(gauge_type)
    self._get_type_weight(gauge_type)
for i in range(500):               # :239-254
    if t > block.timestamp: break
    t += WEEK
    pt = 0
    for gauge_type in range(100):
        if gauge_type == _n_gauge_types: break
        pt += self.points_sum[gauge_type][t].bias * self.points_type_weight[gauge_type][t]
    self.points_total[t] = pt
    if t > block.timestamp:
        self.time_total = t
```

The rewind at `:228-230` matters: `time_total` is normally *one week in the
future*, and the loop must recompute that future bucket because a new vote may
have changed it. Bounds are `range(100)` types and `range(500)` weeks — about
9.6 years of unchecked history.

### 3.4 Relative weight

**`_gauge_relative_weight(addr, time)` `@internal @view` `:347`** — pure read,
no catch-up:

```python
t: uint256 = time / WEEK * WEEK
_total_weight: uint256 = self.points_total[t]
if _total_weight > 0:
    gauge_type: int128 = self.gauge_types_[addr] - 1
    _type_weight: uint256 = self.points_type_weight[gauge_type][t]
    _gauge_weight: uint256 = self.points_weight[addr][t].bias
    return MULTIPLIER * _type_weight * _gauge_weight / _total_weight
else:
    return 0
```

Returns `1e18`-scaled. If the requested week was never checkpointed it reads
zero and returns 0 — hence the write variant.

**`gauge_relative_weight(addr, time = block.timestamp)` `@external @view`
`:371`** — the view wrapper. This is what gauges call inside their integral
loop (`LiquidityGaugeV5.vy:308`) — a view, so the *gauge* must have ensured the
history exists first, which it does by calling `checkpoint_gauge` at `:302`.

**`gauge_relative_weight_write(addr, time = block.timestamp)` `@external`
`:384`** — `_get_weight(addr)`, `_get_total()`, then the same read. Callable by
anyone; idempotent.

**`checkpoint()` `:328`** = `_get_total()`. **`checkpoint_gauge(addr)` `:336`**
= `_get_weight(addr)` then `_get_total()`.

### 3.5 Admin: types and gauges

**`add_type(_name, weight = 0)` `:422`** — `assert msg.sender == self.admin`
(`:428`). Assigns `type_id = n_gauge_types`, stores the name, increments. Only
calls `_change_type_weight` and logs `AddType` **if `weight != 0`** (`:432-434`)
— a type added with weight 0 emits no event, which is a genuine indexing trap.

**`_change_type_weight(type_id, weight)` `@internal` `:401`** — refreshes old
values, then

```python
_total_weight = _total_weight + old_sum * weight - old_sum * old_weight   # :412
self.points_total[next_time]              = _total_weight
self.points_type_weight[type_id][next_time] = weight
self.time_total                            = next_time
self.time_type_weight[type_id]             = next_time
```

with `next_time = (block.timestamp + WEEK) / WEEK * WEEK` (`:410`) — **changes
always land at the next week boundary**, never mid-week. Logs `NewTypeWeight`.
`change_type_weight(type_id, weight)` `:438` is the admin-guarded wrapper.

**`add_gauge(addr, gauge_type, weight = 0)` `:290`**

```python
assert msg.sender == self.admin                                  # :297
assert (gauge_type >= 0) and (gauge_type < self.n_gauge_types)   # :298
assert self.gauge_types_[addr] == 0  # dev: cannot add the same gauge twice   # :299
```

Appends to `gauges[n]`, sets `gauge_types_[addr] = gauge_type + 1` (`:305`). If
`weight > 0` it folds the weight into sum and total at `next_time`
(`:308-318`). Always sets `time_weight[addr] = next_time` (`:322`) and seeds
`time_sum[gauge_type]` if unset (`:320-321`). Logs `NewGauge`.

**`_change_gauge_weight(addr, weight)` `@internal` `:449`** / **`change_gauge_weight`
`:474`** — admin override of a gauge's weight; recomputes sum and total by
delta (`:462-467`). Logs `NewGaugeWeight`. There is no "remove gauge"; the DAO
sets weight to 0 and/or the gauge is killed at the gauge contract.

### 3.6 `vote_for_gauge_weights(_gauge_addr, _user_weight)` — `:485`

The only user-facing write. `_user_weight` is **bps**: 0..10000.

**Reads and checks (`:491-501`):**

```python
slope    = convert(VotingEscrow(escrow).get_last_user_slope(msg.sender), uint256)
lock_end = VotingEscrow(escrow).locked__end(msg.sender)
next_time = (block.timestamp + WEEK) / WEEK * WEEK
assert lock_end > next_time, "Your token lock expires too soon"
assert (_user_weight >= 0) and (_user_weight <= 10000), "You used all your voting power"
assert block.timestamp >= self.last_user_vote[msg.sender][_gauge_addr] + WEIGHT_VOTE_DELAY, "Cannot vote so often"
gauge_type = self.gauge_types_[_gauge_addr] - 1
assert gauge_type >= 0, "Gauge not added"
```

The cooldown is **per (user, gauge)** (`:498`, `:551`), not global — you can
vote on many gauges in one block, but not re-vote the same gauge for 10 days.

**Old and new slopes (`:503-514`):**

```python
old_slope = self.vote_user_slopes[msg.sender][_gauge_addr]
old_dt = 0
if old_slope.end > next_time:
    old_dt = old_slope.end - next_time
old_bias = old_slope.slope * old_dt
new_slope = VotedSlope({slope: slope * _user_weight / 10000, end: lock_end, power: _user_weight})
new_dt = lock_end - next_time     # dev: raises when expired
new_bias = new_slope.slope * new_dt
```

The user's vote line is their veCRV slope scaled by the bps fraction, measured
from `next_time` (not now) to their lock end.

**Power budget (`:517-520`):**

```python
power_used = self.vote_user_power[msg.sender]
power_used = power_used + new_slope.power - old_slope.power
self.vote_user_power[msg.sender] = power_used
assert (power_used >= 0) and (power_used <= 10000), 'Used too much power'
```

Voting 0 on a gauge is how you free the budget back up.

**Bookkeeping (`:525-544`):**

```python
self.points_weight[_gauge_addr][next_time].bias = max(old_weight_bias + new_bias, old_bias) - old_bias
self.points_sum[gauge_type][next_time].bias     = max(old_sum_bias + new_bias, old_bias) - old_bias
if old_slope.end > next_time:
    self.points_weight[_gauge_addr][next_time].slope = max(old_weight_slope + new_slope.slope, old_slope.slope) - old_slope.slope
    self.points_sum[gauge_type][next_time].slope     = max(old_sum_slope + new_slope.slope, old_slope.slope) - old_slope.slope
else:
    self.points_weight[_gauge_addr][next_time].slope += new_slope.slope
    self.points_sum[gauge_type][next_time].slope     += new_slope.slope
if old_slope.end > block.timestamp:
    self.changes_weight[_gauge_addr][old_slope.end] -= old_slope.slope
    self.changes_sum[gauge_type][old_slope.end]     -= old_slope.slope
self.changes_weight[_gauge_addr][new_slope.end] += new_slope.slope
self.changes_sum[gauge_type][new_slope.end]     += new_slope.slope
```

The `max(a + new, old) - old` idiom is unsigned-safe subtraction: it computes
`a + new − old` but floors at 0 instead of underflowing. Then `_get_total()`
(`:546`), store the new `VotedSlope` (`:548`), stamp the cooldown (`:551`), log
`VoteForGauge` (`:553`).

Note `changes_*` here are stored **positive** (added at `:543-544`, subtracted
in `_get_sum:206`/`_get_weight:276`), the opposite sign convention to
`VotingEscrow.slope_changes`.

### 3.7 Views

| Function | Line | Returns |
|---|---|---|
| `gauge_types(_addr) -> int128` | `:153` | type id, reverts if unregistered |
| `get_gauge_weight(addr)` | `:558` | `points_weight[addr][time_weight[addr]].bias` |
| `get_type_weight(type_id)` | `:569` | latest type weight |
| `get_total_weight()` | `:580` | `points_total[time_total]` |
| `get_weights_sum_per_type(type_id)` | `:590` | `points_sum[type_id][time_sum[type_id]].bias` |

### 3.8 From vote to CRV

Putting it together: in week `w`, a gauge with relative weight `r` (1e18-scaled)
receives

```
CRV_for_gauge = rate × WEEK × r / 1e18
```

where `rate` comes from `ERC20CRV.rate()`. Since `Σ_gauges r = 1e18` by
construction of `points_total`, the whole weekly emission is partitioned.
Inside the gauge that budget is then divided among stakers by *working balance*
— see §4.

---

## 4. The gauge family

Seven contracts in `contracts/gauges/`, 4,372 lines, spanning Vyper 0.2.4 →
0.3.1 and three years of iteration.

| File | Ver | Lines | CRV? | Extra rewards | Boost source | ERC-20? |
|---|---|---:|---|---|---|---|
| `LiquidityGauge.vy` | 0.2.4 | 356 | yes | — | `veCRV.balanceOf` | no |
| `LiquidityGaugeReward.vy` | 0.2.4 | 442 | yes | 1 token, Synthetix staking | `veCRV.balanceOf` | no |
| `LiquidityGaugeV2.vy` | 0.2.8 | 758 | yes | 8 tokens, sig-based pull | `veCRV.balanceOf` | yes |
| `LiquidityGaugeV3.vy` | 0.2.12 | 806 | yes | 8 tokens, sig-based + claim throttle | `veCRV.balanceOf` | yes |
| `LiquidityGaugeV4.vy` | 0.2.16 | 705 | yes | 8 tokens, **push** (`deposit_reward_token`) | **veBoost proxy** | yes |
| `LiquidityGaugeV5.vy` | 0.3.1 | 819 | yes | 8 tokens, push | veBoost proxy | yes + `permit` |
| `RewardsOnlyGauge.vy` | 0.2.12 | 486 | **no** | 8 tokens, sig-based | — | yes |

### 4.1 The CRV integral — the idea

A gauge must answer "how much CRV does each staker deserve?" without touching
every staker's storage as time passes. The standard accumulator trick:

```
integrate_inv_supply(T) = ∫₀ᵀ rate(t) · w(t) / working_supply(t)  dt      × 1e18
```

a single global number: CRV per unit of working balance, ever. Then per user:

```
integrate_fraction[u] += working_balance[u] × (integrate_inv_supply − integrate_inv_supply_of[u]) / 1e18
integrate_inv_supply_of[u] = integrate_inv_supply
```

`integrate_fraction[u]` is the user's **lifetime CRV entitlement**. The Minter
subtracts what it has already paid. This is the same shape as Uniswap V3's
`feeGrowthInside` and Aave's `RewardsDistributor` index.

The integral cannot be a single multiplication because two things change on
week boundaries — the gauge's `relative_weight` and (once a year) the inflation
`rate` — so `_checkpoint` integrates **week by week**.

### 4.2 The boost — `_update_liquidity_limit`

`LiquidityGauge.vy:125`, `V2:177`, `V3:181`, `V4:183`, `V5:191`.

```python
lim: uint256 = l * TOKENLESS_PRODUCTION / 100
if voting_total > 0:
    lim += L * voting_balance / voting_total * (100 - TOKENLESS_PRODUCTION) / 100
lim = min(l, lim)
old_bal: uint256 = self.working_balances[addr]
self.working_balances[addr] = lim
_working_supply: uint256 = self.working_supply + lim - old_bal
self.working_supply = _working_supply
```

With `TOKENLESS_PRODUCTION = 40` (`LiquidityGauge.vy:56`, `V5:92`):

```
working_balance = min( l ,  0.40·l + 0.60·L·(veUser/veTotal) )
```

- `l` = the user's LP balance, `L` = the gauge's `totalSupply`.
- With no veCRV you earn on 40% of your deposit.
- The cap `min(l, …)` means the **maximum boost is 1/0.4 = 2.5×**.
- Full boost needs `veUser/veTotal ≥ l/L` — your share of all veCRV must match
  your share of this gauge.

Worked, with `l = 1000`, `L = 100,000` (so `l/L = 1%`):

| `veUser/veTotal` | working_balance | boost |
|---:|---:|---:|
| 0 | 400.00 | 1.000× |
| 0.1% | 460.00 | 1.150× |
| **1%** | **1000.00** | **2.500×** |
| 5% | 1000.00 | 2.500× (capped) |

Because `working_supply` is the denominator of the integral, boosting yourself
dilutes everyone else — the emission is fixed by the GaugeController, the
gauge only decides its split.

**Version differences.** V1 and V2/V3 read `ERC20(voting_escrow).balanceOf(addr)`
and `.totalSupply()` directly (`LiquidityGauge.vy:136-137`). V1 additionally
gates the boost behind `BOOST_WARMUP = 2 weeks` (`:57`, `:140`):

```python
if (voting_total > 0) and (block.timestamp > self.period_timestamp[0] + BOOST_WARMUP):
```

so for the gauge's first fortnight everyone is unboosted. V4 and V5 replace the
direct read with `VotingEscrowBoost(VEBOOST_PROXY).adjusted_balance_of(addr)`
(`V4:194`, `V5:201`) — this is the delegable-boost proxy, letting a veCRV
holder lend boost to another address without moving the lock.

### 4.3 `_checkpoint(addr)` — the integral, line by line

`LiquidityGauge.vy:153`, `V2:258`, `V3:296`, `V4:271`, `V5:279`. Structure is
identical across versions; V5 shown (`:279-343`).

**Step 1 — detect a rate change (`:284-293`).**

```python
_period: int128 = self.period
_period_time: uint256 = self.period_timestamp[_period]
_integrate_inv_supply: uint256 = self.integrate_inv_supply[_period]
rate: uint256 = self.inflation_rate
new_rate: uint256 = rate
prev_future_epoch: uint256 = self.future_epoch_time
if prev_future_epoch >= _period_time:
    self.future_epoch_time = CRV20(CRV).future_epoch_time_write()
    new_rate = CRV20(CRV).rate()
    self.inflation_rate = new_rate
```

If the CRV epoch boundary falls at or after our last checkpoint, we may cross
it during this integration, so fetch both the *new* rate and the *next*
boundary. `future_epoch_time_write` is state-changing — this is where a gauge
checkpoint can roll the CRV epoch for the whole protocol.

**Step 2 — killed gauges (`:295-297`).**

```python
if self.is_killed:
    rate = 0
```

Note only the local `rate` is zeroed, not `new_rate`. A killed gauge stops
accruing but keeps its bookkeeping consistent.

**Step 3 — the weekly loop (`:300-332`).**

```python
if block.timestamp > _period_time:
    _working_supply: uint256 = self.working_supply
    Controller(GAUGE_CONTROLLER).checkpoint_gauge(self)
    prev_week_time: uint256 = _period_time
    week_time: uint256 = min((_period_time + WEEK) / WEEK * WEEK, block.timestamp)

    for i in range(500):
        dt: uint256 = week_time - prev_week_time
        w: uint256 = Controller(GAUGE_CONTROLLER).gauge_relative_weight(self, prev_week_time / WEEK * WEEK)

        if _working_supply > 0:
            if prev_future_epoch >= prev_week_time and prev_future_epoch < week_time:
                _integrate_inv_supply += rate * w * (prev_future_epoch - prev_week_time) / _working_supply
                rate = new_rate
                _integrate_inv_supply += rate * w * (week_time - prev_future_epoch) / _working_supply
            else:
                _integrate_inv_supply += rate * w * dt / _working_supply

        if week_time == block.timestamp:
            break
        prev_week_time = week_time
        week_time = min(week_time + WEEK, block.timestamp)
```

Points to notice:

- `checkpoint_gauge` is called **first** so the subsequent `gauge_relative_weight`
  *view* calls find populated history.
- The relative weight is sampled at the **week bucket start**
  (`prev_week_time / WEEK * WEEK`), so weight is piecewise-constant per week.
- The `prev_future_epoch` branch splits a week that straddles a CRV epoch
  boundary into two integrals at two rates.
- **The bug-shaped comment at `:315-316`:** *"If more than one epoch is crossed
  - the gauge gets less, but that'd mean it wasn't called for more than 1
  year."* Only one rate change is handled per checkpoint. A gauge untouched for
  over a year under-pays. The 500-iteration bound (≈9.6 years) is the other
  cliff.
- `_working_supply == 0` weeks are skipped entirely — emission allocated to an
  empty gauge is simply never minted.
- The precision note at `:322-327` argues worst-case loss is ~1e-9.

**Step 4 — commit (`:334-343`).**

```python
_period += 1
self.period = _period
self.period_timestamp[_period] = block.timestamp
self.integrate_inv_supply[_period] = _integrate_inv_supply

_working_balance: uint256 = self.working_balances[addr]
self.integrate_fraction[addr] += _working_balance * (_integrate_inv_supply - self.integrate_inv_supply_of[addr]) / 10 ** 18
self.integrate_inv_supply_of[addr] = _integrate_inv_supply
self.integrate_checkpoint_of[addr] = block.timestamp
```

Every checkpoint appends a new period, even if nothing changed.

**Critical ordering invariant.** `_checkpoint(addr)` uses the user's **current**
`working_balance`, so it must run *before* balances change, and
`_update_liquidity_limit` must run *after*. Every caller obeys this — e.g.
`V5.deposit` (`:459` then `:472`), `V5.withdraw` (`:488` then `:501`).

### 4.4 `LiquidityGauge.vy` (v1) — full function list

| Function | Line | Access | Notes |
|---|---|---|---|
| `__init__(lp_addr, _minter, _admin)` | `:100` | — | Reads `crv_token`/`controller` **from the Minter** (`:113-116`), `voting_escrow` from the Controller (`:117`); seeds `period_timestamp[0]`, `inflation_rate`, `future_epoch_time` (`:118-120`) |
| `_update_liquidity_limit(addr, l, L)` | `:125` | internal | With `BOOST_WARMUP` gate |
| `_checkpoint(addr)` | `:153` | internal | Calls `Controller.checkpoint_gauge` at `:170` (outside the `if`, unlike later versions) |
| `user_checkpoint(addr) -> bool` | `:223` | `msg.sender == addr or == minter  # dev: unauthorized` (`:229`) | checkpoint + limit |
| `claimable_tokens(addr) -> uint256` | `:236` | any, **not a view** | `_checkpoint` then `integrate_fraction[addr] - Minter.minted(addr, self)` (`:243`). Docstring `:239`: *"should be manually changed to view in the ABI"* |
| `kick(addr)` | `:247` | any | See below |
| `set_approve_deposit(addr, can_deposit)` | `:268` | any | v1/v2/v3 only |
| `deposit(_value, addr = msg.sender)` | `:279` | `@nonreentrant('lock')` | `assert self.approved_to_deposit[msg.sender][addr], "Not approved"` when depositing for another (`:286`) |
| `withdraw(_value)` | `:305` | `@nonreentrant('lock')` | |
| `integrate_checkpoint() -> uint256` | `:326` | view | `period_timestamp[period]` |
| `kill_me()` | `:331` | `msg.sender == self.admin` (`:332`) | **Toggles** `is_killed` |
| `commit_transfer_ownership(addr)` | `:337` | admin | |
| `apply_transfer_ownership()` | `:348` | admin (`:352`) | |

**`kick(addr)` `:247`** — the anti-freeloader. A user's boost is fixed at their
last checkpoint; if their lock expires, their `working_balance` stays
inflated until they interact. `kick` lets anyone force a re-limit:

```python
t_last: uint256 = self.integrate_checkpoint_of[addr]
t_ve: uint256 = VotingEscrow(_voting_escrow).user_point_history__ts(
    addr, VotingEscrow(_voting_escrow).user_point_epoch(addr))
_balance: uint256 = self.balanceOf[addr]

assert ERC20(self.voting_escrow).balanceOf(addr) == 0 or t_ve > t_last  # dev: kick not allowed
assert self.working_balances[addr] > _balance * TOKENLESS_PRODUCTION / 100  # dev: kick not needed
```

First assert: only if their veCRV is now zero, **or** they had a ve event after
their last gauge checkpoint. Second: only if they are actually still boosted.
Then checkpoint and re-limit (`:263-264`). Identical in every boosted version
(`V2:417`, `V3:478`, `V4:422`, `V5:430`).

### 4.5 `LiquidityGaugeReward.vy` — the Synthetix bolt-on

Vyper 0.2.4, 442 lines. v1 plus **one** extra reward token staked into a
Synthetix-style `StakingRewards` contract.

Constructor `:114` takes `_reward_contract` and `_rewarded_token`.

**`_checkpoint_rewards(addr, claim_rewards)` `@internal` `:173`** — the
balance-diff pattern that all later versions refine: snapshot
`rewarded_token.balanceOf(self)`, optionally call the reward contract's
`getReward()`, diff, fold into `reward_integral` per unit of `totalSupply`,
then credit the user via `reward_integral_for[addr]`.

**`_checkpoint(addr, claim_rewards)` `:195`** — v1's integral plus the reward
checkpoint.

| Function | Line |
|---|---|
| `user_checkpoint(addr)` | `:267` |
| `claimable_tokens(addr)` | `:280` |
| `claimable_reward(addr)` `@view` | `:292` |
| `kick(addr)` | `:311` |
| `set_approve_deposit(addr, can_deposit)` | `:332` |
| `deposit(_value, addr = msg.sender)` | `:343` |
| `withdraw(_value, claim_rewards = True)` | `:370` |
| `claim_rewards(addr = msg.sender)` | `:393` |
| `integrate_checkpoint()` | `:403` |
| `kill_me()` | `:408` |
| `commit_transfer_ownership(addr)` / `apply_transfer_ownership()` | `:414` / `:425` |
| `toggle_external_rewards_claim(val)` | `:436` |

`toggle_external_rewards_claim` is the escape hatch: if the external staking
contract breaks, the admin disables claiming so deposits/withdrawals keep
working.

### 4.6 `LiquidityGaugeV2.vy` — ERC-20 + 8 rewards by selector

Vyper 0.2.8, 758 lines. Two big changes: the gauge **is itself an ERC-20**
(deposit receipts become transferable), and external rewards generalise to 8
tokens with an arbitrary staking contract driven by raw selectors.

New storage (`:113-122`): `reward_contract`, `reward_tokens[8]`,
`reward_sigs: bytes32`, `reward_integral[token]`,
`reward_integral_for[token][user]`.

**`_checkpoint_rewards(_addr, _total_supply)` `@internal` `:205`.** Returns
immediately if `_total_supply == 0` (`:210-211`). Snapshots all 8 balances,
then:

```python
raw_call(self.reward_contract, slice(self.reward_sigs, 8, 4))  # dev: bad claim sig
```

Bytes 8..12 of `reward_sigs` are the **claim** selector; 0..4 deposit, 4..8
withdraw. After the call, `dI = 1e18 * (balanceAfter - balanceBefore) / _total_supply`
(`:227`) and rewards are transferred to the user immediately via `raw_call`
(`:245-256`) — V2 has no "accrue without transferring" mode.

**`set_rewards(_reward_contract, _sigs, _reward_tokens[8])` `:649`,
`@nonreentrant('lock')`, admin only (`:663`).** The most interesting function in
the file. To migrate away from an existing contract it checkpoints, withdraws
the entire `total_supply` using the stored withdraw selector, and revokes
approval (`:668-676`). To install a new one it does a **live test round-trip**
(`:678-708`):

```python
assert _reward_contract.is_contract  # dev: not a contract
...
assert total_supply != 0  # dev: zero total supply
ERC20(lp_token).approve(_reward_contract, MAX_UINT256)
raw_call(_reward_contract, concat(deposit_sig, convert(total_supply, bytes32)))  # dev: failed deposit
assert ERC20(lp_token).balanceOf(self) == 0
raw_call(_reward_contract, concat(withdraw_sig, convert(total_supply, bytes32)))  # dev: failed withdraw
assert ERC20(lp_token).balanceOf(self) == total_supply
raw_call(_reward_contract, concat(deposit_sig, convert(total_supply, bytes32)))
```

Deposit everything, assert the balance went to zero, withdraw everything,
assert it came back, then deposit for real. A wrong selector cannot be
installed. `assert convert(withdraw_sig, uint256) == 0  # dev: withdraw without
deposit` (`:710`) forbids a withdraw-only configuration. Finally
`assert i != 0  # dev: no reward token` (`:719`).

ERC-20 surface: `allowance` `:525`, `_transfer` `:536`, `transfer` `:560`,
`transferFrom` `:574`, `approve` `:592`, `increaseAllowance` `:612`,
`decreaseAllowance` `:630`. `_transfer` checkpoints **both** parties and
re-limits both (`:536-556`) — transferring a gauge position moves the boost
basis with it.

Others: `decimals()` `:161`, `integrate_checkpoint()` `:172`,
`user_checkpoint` `:328`, `claimable_tokens` `:341`,
`claimable_reward(_addr,_token)` `:353` (`@nonreentrant`, not a view — it
claims to compute), `claim_rewards(_addr)` `:378`,
`claim_historic_rewards(_reward_tokens[8], _addr)` `:388` (drains tokens no
longer in the active list), `kick` `:417`, `set_approve_deposit` `:438`,
`deposit(_value,_addr)` `:449`, `withdraw(_value)` `:489`,
`set_killed(_is_killed)` `:726` (now a **setter**, not a toggle),
`commit_transfer_ownership` `:738`, `accept_transfer_ownership` `:750` —
renamed from `apply_` and now guarded by the *future* admin.

### 4.7 `LiquidityGaugeV3.vy` — throttling and packed claim data

Vyper 0.2.12, 806 lines. V2 plus three refinements.

**1. Claim throttle.** `CLAIM_FREQUENCY: constant(uint256) = 3600` (`:74`). The
reward contract is only polled once an hour (`:224`):

```python
if _total_supply != 0 and reward_data != 0 and block.timestamp > shift(reward_data, -160) + CLAIM_FREQUENCY:
```

`reward_data: uint256` (`:112`) packs `[uint96 last_claim][uint160 contract]`,
rewritten at `:238`. Exposed as `reward_contract()` `:391` and `last_claim()`
`:401`.

**2. Packed per-user claim data.** `claim_data[user][token]: uint256` (`:128`)
holds `[uint128 claimable][uint128 claimed]`. This decouples *accruing* from
*transferring*: `_checkpoint_rewards(_user, _total_supply, _claim, _receiver)`
(`:209`) takes a `_claim` flag, and when false it accumulates into the high 128
bits (`:293-294`) instead of transferring. That is what makes
`deposit(..., _claim_rewards = False)` cheap. `claimed_reward` `:411`,
`claimable_reward` `:423` (a true `@view`), `claimable_reward_write` `:438`.

**3. Reward receivers.** `rewards_receiver[user]` (`:119`) with setter
`set_rewards_receiver(_receiver)` `:453`; resolution logic at `:249-255`.

Everything else mirrors V2: `_checkpoint` `:296`, `user_checkpoint` `:366`,
`claimable_tokens` `:379`, `claim_rewards(_addr, _receiver)` `:464`, `kick`
`:478`, `deposit(_value,_addr,_claim_rewards)` `:500`,
`withdraw(_value,_claim_rewards)` `:540`, `_transfer` `:577`, `transfer` `:601`,
`transferFrom` `:615`, `approve` `:633`, `increaseAllowance` `:653`,
`decreaseAllowance` `:671`, `set_rewards` `:690`, `set_killed` `:774`,
`commit_transfer_ownership` `:786`, `accept_transfer_ownership` `:798`.
`set_approve_deposit` is **gone** — anyone may deposit for anyone.

### 4.8 `LiquidityGaugeV4.vy` — push rewards, hardcoded addresses

Vyper 0.2.16, 705 lines. The model inverts: instead of the gauge *pulling* from
a staking contract, a designated distributor **pushes** tokens in.

```python
struct Reward:                # V5 :76, same in V4
    token: address
    distributor: address
    period_finish: uint256
    rate: uint256
    last_update: uint256
    integral: uint256
```

`reward_count` (`:125`), `reward_tokens[8]`, `reward_data[token]: Reward`.

**`add_reward(_reward_token, _distributor)` `:614`** — `assert msg.sender ==
self.admin  # dev: only owner`, `reward_count < MAX_REWARDS`, and
`self.reward_data[_reward_token].distributor == ZERO_ADDRESS` (no re-add).

**`set_reward_distributor(_reward_token, _distributor)` `:630`** — callable by
the current distributor **or** the admin; both old and new must be non-zero.

**`deposit_reward_token(_reward_token, _amount)` `:642`, `@nonreentrant("lock")`**
— `assert msg.sender == self.reward_data[_reward_token].distributor`, checkpoint
globally, `transferFrom` via `raw_call`, then the Synthetix rate roll:

```python
period_finish: uint256 = self.reward_data[_reward_token].period_finish
if block.timestamp >= period_finish:
    self.reward_data[_reward_token].rate = _amount / WEEK
else:
    remaining: uint256 = period_finish - block.timestamp
    leftover: uint256 = remaining * self.reward_data[_reward_token].rate
    self.reward_data[_reward_token].rate = (_amount + leftover) / WEEK
self.reward_data[_reward_token].last_update = block.timestamp
self.reward_data[_reward_token].period_finish = block.timestamp + WEEK
```

Topping up mid-period stretches the remainder over a fresh week.

`_checkpoint_rewards` (`:210`) becomes purely arithmetic — no `raw_call` to a
foreign staking contract:

```python
last_update: uint256 = min(block.timestamp, self.reward_data[token].period_finish)
duration: uint256 = last_update - self.reward_data[token].last_update
if duration != 0:
    self.reward_data[token].last_update = last_update
    if _total_supply != 0:
        integral += duration * self.reward_data[token].rate * 10**18 / _total_supply
        self.reward_data[token].integral = integral
```

Also new: `VEBOOST_PROXY` for boost delegation (`:194`), and the constructor
`__init__(_lp_token, _admin)` `:146` drops the `_minter` argument — Minter, CRV,
VotingEscrow, GaugeController and the veBoost proxy are all **hardcoded
constants**. Remaining: `decimals` `:167`, `integrate_checkpoint` `:178`,
`_checkpoint` `:271`, `user_checkpoint` `:339`, `claimable_tokens` `:352`,
`claimed_reward` `:364`, `claimable_reward` `:376`, `set_rewards_receiver`
`:397`, `claim_rewards` `:408`, `kick` `:422`, `deposit` `:443`, `withdraw`
`:474`, `_transfer` `:502`, `transfer` `:526`, `transferFrom` `:540`, `approve`
`:558`, `increaseAllowance` `:578`, `decreaseAllowance` `:596`, `set_killed`
`:673`, `commit_transfer_ownership` `:685`, `accept_transfer_ownership` `:697`.

### 4.9 `LiquidityGaugeV5.vy` — immutables and EIP-2612

Vyper 0.3.1, 819 lines. V4 plus modern Vyper.

Hardcoded mainnet constants (`:95-99`):

| Constant | Address |
|---|---|
| `MINTER` | `0xd061D61a4d941c39E5453435B6345Dc261C2fcE0` |
| `CRV` | `0xD533a949740bb3306d119CC777fa900bA034cd52` |
| `VOTING_ESCROW` | `0x5f3b5DfEb7B28CDbD7FAba78963EE202a494e2A2` |
| `GAUGE_CONTROLLER` | `0x2F50D538606Fa9EDD2B11E2446BEb18C9D5846bB` |
| `VEBOOST_PROXY` | `0x8E0c00ed546602fD9927DF742bbAbF726D5B0d16` |

`NAME`, `SYMBOL`, `DOMAIN_SEPARATOR`, `LP_TOKEN` are `immutable` (`:102-106`),
set in `__init__` (`:159-181`) — name is built as `concat("Curve.fi ",
lp_symbol, " Gauge Deposit")` (`:173`) and symbol as `concat(lp_symbol,
"-gauge")` (`:176`). Being immutables, they are exposed through explicit
getters: `name()` `:768`, `symbol()` `:777`, `decimals()` `:786` (returns a
literal `18`), `lp_token()` `:797`, `version()` `:806` (`"v5.0.0"`, `:89`),
`DOMAIN_SEPARATOR()` `:815`.

**`permit(_owner, _spender, _value, _deadline, _v, _r, _s) -> bool` `:586`** —
EIP-2612, with **ERC-1271 support** for contract wallets (`:292-297`):

```python
if _owner.is_contract:
    sig: Bytes[65] = concat(_abi_encode(_r, _s), slice(convert(_v, bytes32), 31, 1))
    assert ERC1271(_owner).isValidSignature(digest, sig) == ERC1271_MAGIC_VAL
else:
    assert ecrecover(digest, convert(_v, uint256), convert(_r, uint256), convert(_s, uint256)) == _owner
```

`assert _owner != ZERO_ADDRESS` (`:281`), `assert block.timestamp <= _deadline`
(`:282`), nonce bumped at `:300`. `ERC1271_MAGIC_VAL` `:86`, `EIP712_TYPEHASH`
`:87`, `PERMIT_TYPEHASH` `:88`.

One subtle V5-only detail: `_update_liquidity_limit` (`:191`) drops V4's
`BOOST_WARMUP`-style gating entirely and `_checkpoint` moves the
`working_supply` read *inside* the `if block.timestamp > _period_time` block
(`:301`).

### 4.10 `RewardsOnlyGauge.vy` — sidechain rewards, no CRV

Vyper 0.2.12, 486 lines. Everything about CRV is deleted: no `integrate_*`, no
`working_balances`, no boost, no GaugeController. It is V3's reward half plus
an ERC-20.

`__init__(_admin, _lp_token)` `:77`. `_checkpoint_rewards(_user,
_total_supply, _claim, _receiver)` `:104` is V3's, including the
`CLAIM_FREQUENCY` throttle and packed `claim_data`. Views: `decimals` `:94`,
`reward_contract` `:176`, `last_claim` `:186`, `claimed_reward` `:196`,
`claimable_reward` `:208`, `claimable_reward_write` `:223`. Actions:
`set_rewards_receiver` `:238`, `claim_rewards` `:249`, `deposit` `:264`,
`withdraw` `:290`, `_transfer` `:314`, `transfer` `:332`, `transferFrom` `:346`,
`approve` `:364`, `increaseAllowance` `:384`, `decreaseAllowance` `:402`,
`set_rewards(_reward_contract, _claim_sig, _reward_tokens[8])` `:421`,
`commit_transfer_ownership` `:466`, `accept_transfer_ownership` `:478`.

This is what a sidechain LP stakes into; CRV reaches the chain separately via a
root gauge (§10) and a streamer (§9).

---

## 5. `Minter.vy`

`contracts/Minter.vy`, Vyper 0.2.4, 99 lines. The only address CRV will mint
for.

Storage: `token` (`:26`), `controller` (`:27`),
`minted[user][gauge]` (`:30`), `allowed_to_mint_for[minter][user]` (`:33`).
`__init__(_token, _controller)` `:37`.

### `_mint_for(gauge_addr, _for)` `@internal` — `:43`

```python
assert GaugeController(self.controller).gauge_types(gauge_addr) >= 0  # dev: gauge is not added

LiquidityGauge(gauge_addr).user_checkpoint(_for)
total_mint: uint256 = LiquidityGauge(gauge_addr).integrate_fraction(_for)
to_mint: uint256 = total_mint - self.minted[_for][gauge_addr]

if to_mint != 0:
    MERC20(self.token).mint(_for, to_mint)
    self.minted[_for][gauge_addr] = total_mint
    log Minted(_for, gauge_addr, total_mint)
```

Four lines carry the whole design:

1. **Registration check.** `gauge_types` reverts for an unregistered gauge
   (`GaugeController.vy:160`), so a fake gauge cannot mint. The `>= 0` is
   almost decorative — the revert does the work.
2. **Force a checkpoint.** `user_checkpoint(_for)` brings
   `integrate_fraction[_for]` up to now. The gauge allows this because the
   caller is the Minter (`LiquidityGauge.vy:229`).
3. **Idempotence.** `minted[_for][gauge]` is a high-water mark. `integrate_fraction`
   only ever grows, so `to_mint` is the un-paid remainder. Calling `mint` twice
   in a block yields zero the second time; there is no reentrancy value in the
   accounting itself, though every entry point is `@nonreentrant('lock')`.
4. **Event caveat.** `log Minted(_for, gauge_addr, total_mint)` emits the
   *cumulative* total, not `to_mint`. Indexers that sum `Minted.minted` will
   overcount badly.

### Entry points

| Function | Line | Guard |
|---|---|---|
| `mint(gauge_addr)` | `:59` | `@nonreentrant('lock')`; mints for `msg.sender` |
| `mint_many(gauge_addrs: address[8])` | `:69` | `@nonreentrant('lock')`; loops, breaks at first `ZERO_ADDRESS` (`:75-76`) |
| `mint_for(gauge_addr, _for)` | `:82` | `@nonreentrant('lock')`; **silently does nothing** if not approved (`:89-90`) |
| `toggle_approve_mint(minting_user)` | `:94` | any; flips `allowed_to_mint_for[minting_user][msg.sender]` (`:99`) |

`mint_for` uses `if` rather than `assert` (`:89`) — an unauthorised call
succeeds and mints nothing. Wrappers that call it and assume tokens arrived
will silently under-deliver.

---

## 6. `FeeDistributor.vy`

`contracts/FeeDistributor.vy`, Vyper 0.2.7, 466 lines.

Pays trading fees (historically 3CRV, later crvUSD) to veCRV holders. The
economics: 50% of every Curve swap fee is the "admin fee", which the burner
chain (§11) converts into `token` and pushes here; this contract splits it
across weeks and pays each holder in proportion to their veCRV **at each week
boundary**.

### 6.1 Storage and constants

| Name | Line | Meaning |
|---|---|---|
| `WEEK` | `:46` | `7*86400` |
| `TOKEN_CHECKPOINT_DEADLINE` | `:47` | `86400` — throttle for permissionless token checkpoints |
| `start_time` | `:49` | week-rounded distribution start |
| `time_cursor` | `:50` | next week needing a `ve_supply` snapshot |
| `time_cursor_of[user]` | `:51` | next week to pay this user |
| `user_epoch_of[user]` | `:52` | cached ve epoch cursor |
| `last_token_time` | `:54` | last `_checkpoint_token` |
| `tokens_per_week[t]` | `:55` | fees allocated to week `t` |
| `voting_escrow`, `token` | `:57-58` | |
| `total_received`, `token_last_balance` | `:59-60` | |
| `ve_supply[t]` | `:62` | veCRV total supply at week boundary `t` |
| `admin`, `future_admin` | `:64-65` | |
| `can_checkpoint_token` | `:66` | permissionless-checkpoint flag |
| `emergency_return` | `:67` | kill/recover destination |
| `is_killed` | `:68` | |

`__init__(_voting_escrow, _start_time, _token, _admin, _emergency_return)` `:72`
rounds `_start_time` down to a week and seeds `start_time`, `last_token_time`
and `time_cursor` to it (`:88-91`).

The contract redeclares `struct Point` (`:39`) to match `VotingEscrow`'s,
because it reads `point_history`/`user_point_history` directly.

### 6.2 `_checkpoint_token()` `@internal` — `:99`

Splits newly-arrived tokens across the weeks they arrived over.

```python
token_balance: uint256 = ERC20(self.token).balanceOf(self)
to_distribute: uint256 = token_balance - self.token_last_balance
self.token_last_balance = token_balance

t: uint256 = self.last_token_time
since_last: uint256 = block.timestamp - t
self.last_token_time = block.timestamp
this_week: uint256 = t / WEEK * WEEK

for i in range(20):
    next_week = this_week + WEEK
    if block.timestamp < next_week:
        ... self.tokens_per_week[this_week] += to_distribute * (block.timestamp - t) / since_last
        break
    else:
        ... self.tokens_per_week[this_week] += to_distribute * (next_week - t) / since_last
    t = next_week
    this_week = next_week
```

Balance-diff accounting: anything that appeared since the last checkpoint is
spread **pro-rata over elapsed time**, not dumped into the current week. The
`since_last == 0` branches (`:113`, `:119`) avoid division by zero in the
same-timestamp case. `range(20)` caps the catch-up at 20 weeks — leave this
uncalled for longer and allocation is wrong.

**`checkpoint_token()` `@external` `:130`** —

```python
assert (msg.sender == self.admin) or\
       (self.can_checkpoint_token and (block.timestamp > self.last_token_time + TOKEN_CHECKPOINT_DEADLINE))
```

Admin always; anyone else only once `can_checkpoint_token` is on and a day has
passed.

### 6.3 `_checkpoint_total_supply()` `@internal` — `:193`

Snapshots veCRV total supply at each week boundary.

```python
VotingEscrow(ve).checkpoint()          # :197  force ve history current
for i in range(20):
    if t > rounded_timestamp: break
    epoch: uint256 = self._find_timestamp_epoch(ve, t)
    pt: Point = VotingEscrow(ve).point_history(epoch)
    dt: int128 = 0
    if t > pt.ts:
        dt = convert(t - pt.ts, int128)
    self.ve_supply[t] = convert(max(pt.bias - pt.slope * dt, 0), uint256)
    t += WEEK
self.time_cursor = t
```

**Note the approximation:** it decays the bracketing global point linearly to
`t` **without** applying `slope_changes` in between. `VotingEscrow.totalSupply`
does apply them (`supply_at:612`). So `ve_supply[t]` can differ slightly from
the true total. It is consistent between numerator and denominator only if the
user's own decay is computed the same way — which it is (`_claim:279`). The
`range(20)` bound again caps catch-up at 20 weeks.

`_find_timestamp_epoch(ve, _timestamp)` `:144` and
`_find_timestamp_user_epoch(ve, user, _timestamp, max_user_epoch)` `:161` are
binary searches over ve history; both use `(_min + _max + 2) / 2` (`:150`,
`:167`) rather than the usual `+1`, an upper-biased midpoint.

**`checkpoint_total_supply()` `@external` `:217`** — permissionless, unthrottled.

**`ve_for_at(_user, _timestamp) -> uint256` `@view @external` `:178`** — the
public helper: find the user's ve epoch at that time and decay it.

### 6.4 `_claim(addr, ve, _last_token_time)` `@internal` — `:228`

Walks the user forward week by week, at most 50 weeks per call.

Setup (`:233-259`): `max_user_epoch == 0` → return 0 (no lock, no fees,
`:236-238`). First-ever claim does a binary search from `start_time`
(`:243`) and sets `week_cursor` to the week **after** the user's first point
(`:253`): `(user_point.ts + WEEK - 1) / WEEK * WEEK`. Returns 0 if the cursor
has caught up to `_last_token_time` (`:255-256`).

The loop (`:263-285`):

```python
for i in range(50):
    if week_cursor >= _last_token_time: break

    if week_cursor >= user_point.ts and user_epoch <= max_user_epoch:
        user_epoch += 1
        old_user_point = user_point
        if user_epoch > max_user_epoch:
            user_point = empty(Point)
        else:
            user_point = VotingEscrow(ve).user_point_history(addr, user_epoch)
    else:
        dt: int128 = convert(week_cursor - old_user_point.ts, int128)
        balance_of: uint256 = convert(max(old_user_point.bias - dt * old_user_point.slope, 0), uint256)
        if balance_of == 0 and user_epoch > max_user_epoch:
            break
        if balance_of > 0:
            to_distribute += balance_of * self.tokens_per_week[week_cursor] / self.ve_supply[week_cursor]
        week_cursor += WEEK
```

The two-armed loop alternates: advance the ve-point cursor until it is ahead of
the week cursor, then pay that week. The payout line is the whole economics:

```
user_week_fees = veCRV(user, week) × tokens_per_week[week] / ve_supply[week]
```

Then `user_epoch = min(max_user_epoch, user_epoch - 1)` (`:287`) and cursors are
persisted (`:288-289`).

**The 50-week cap is user-visible.** The docstring on `claim` (`:301-305`)
explains: if the emitted `Claimed(addr, amount, claim_epoch, max_epoch)` has
`claim_epoch < max_epoch`, call again. A holder with a long, busy ve history
must claim repeatedly.

### 6.5 External surface

| Function | Line | Notes |
|---|---|---|
| `claim(_addr = msg.sender) -> uint256` | `:298` | `@nonreentrant('lock')`; `assert not self.is_killed` (`:309`); auto-checkpoints supply (`:311-312`) and token (`:316-318`) if allowed; floors `last_token_time` to the week (`:320`); transfers and decrements `token_last_balance` (`:325-326`) |
| `claim_many(_receivers: address[20]) -> bool` | `:333` | `@nonreentrant('lock')`; stops at first `ZERO_ADDRESS` (`:360-361`); one `token_last_balance` decrement for the batch (`:368-369`) |
| `burn(_coin) -> bool` | `:375` | `assert _coin == self.token` (`:381`), `assert not self.is_killed`; pulls the **caller's entire balance** (`:384-386`) then maybe checkpoints. Named `burn` to fit the burner interface (§11) — this is the terminus of the fee chain |
| `commit_admin(_addr)` | `:394` | `msg.sender == self.admin  # dev: access denied` |
| `apply_admin()` | `:405` | `msg.sender == self.admin`, `future_admin != ZERO_ADDRESS` |
| `toggle_allow_checkpoint_token()` | `:417` | admin; flips `can_checkpoint_token`, logs `ToggleAllowCheckpointToken` |
| `kill_me()` | `:428` | admin; sets `is_killed = True` **irreversibly** and sweeps the whole `token` balance to `emergency_return` (`:436-439`) |
| `recover_balance(_coin) -> bool` | `:443` | admin; `assert _coin != self.token` (`:451`); `raw_call` transfer to `emergency_return`, tolerating non-standard ERC-20s (`:454-464`) |

---
