# Uniswap V1 Deep Dive

**Source:** `uni/v1-contracts/contracts/uniswap_exchange.vy` (496 lines) and
`uni/v1-contracts/contracts/uniswap_factory.vy` (46 lines), written in Vyper, deployed November 2018.

This chapter covers **every function in both contracts**. V2, V3 and V4 are covered separately in
[`uni/UNISWAP-DEEP-DIVE.md`](./UNISWAP-DEEP-DIVE.md) — read this one first, then that one, and the
design decisions in V2 will read as direct answers to the limitations you find here.

All numeric examples in this document are taken from, or verified against, the repository's own
test constants in `uni/v1-contracts/tests/constants.py`.

---

## 0. Context: what V1 actually is

### The idea

In 2018 the on-chain trading options were order books (Ethfinex, 0x) that needed market makers and
lots of gas. Uniswap V1 replaced the order book with a formula. Anyone can deposit two assets into
a pot; the pot quotes a price derived only from how much of each asset it holds. No orders, no
matching, no off-chain infrastructure.

Three design choices define V1, and all three changed in V2:

1. **Every market is paired against ETH.** There is no HAY/DEN pool. There is a HAY/ETH pool and
   a DEN/ETH pool, and HAY→DEN routes through ETH.
2. **One contract per token.** A factory deploys a fresh `uniswap_exchange` clone for each ERC20.
3. **Written in Vyper**, not Solidity, and the LP token is baked *into* the exchange contract
   rather than being a separate ERC20.

### Why pair everything against ETH

With `N` tokens, arbitrary pairing needs up to `N*(N-1)/2` pools, and liquidity gets shredded
across all of them. Hub-and-spoke with ETH at the centre needs exactly `N` pools, and every token
is reachable from every other token in two hops. ETH was the obvious hub in 2018: it is the gas
token, it needs no ERC20 approval, and it had by far the deepest liquidity.

```
        Order-book world              Uniswap V1 hub-and-spoke
                                              HAY
     HAY ---- DEN                              |
      |  \  /  |                               |
      |   \/   |                    DEN ----- ETH ----- USDC
      |   /\   |                               |
     USDC ---- WBTC                            |
                                              WBTC
   N*(N-1)/2 markets                      N markets, 2 hops max
```

The cost is real and it is paid on every token→token trade:

- **Two pools, two fees.** HAY→DEN pays 0.3% in the HAY pool and 0.3% again in the DEN pool, so
  ~0.599% round trip, plus two lots of slippage.
- **ETH price exposure in the middle.** The intermediate ETH exists only inside one transaction,
  so there is no price risk in practice, but it does mean the ETH legs of both pools move.
- **LPs are forced to hold ETH.** You cannot provide HAY/DEN liquidity. Every LP is half-exposed
  to ETH, whether they want that exposure or not. This is the single biggest reason V2 allowed
  arbitrary ERC20/ERC20 pairs.

### The invariant: x·y = k

Let `x` = the reserve of the asset you are selling into the pool, `y` = the reserve of the asset
you are buying. The pool's rule is that the product of reserves must not decrease:

```
x * y = k
```

Sell `Δx` into the pool, receive `Δy` such that `(x + Δx)(y − Δy) = k`. Solving for `Δy`:

```
Δy = (Δx * y) / (x + Δx)
```

Two consequences fall straight out of that formula and they are the whole economic story of an AMM:

- **The marginal price is `y/x`.** For an infinitesimal trade, `Δy/Δx → y/x`. So the pool's quoted
  price is just the ratio of what it holds.
- **Price moves against you as you trade.** The denominator grows with `Δx`. Buying more of `y`
  makes each additional unit more expensive. The pool can never be drained: to take all of `y`
  you would need infinite `x`.

The 0.3% fee is applied by shrinking the input before it enters the formula. With
`γ = 997/1000`:

```
Δy = (γ·Δx · y) / (x + γ·Δx)
```

The fee is *not* skimmed off to a separate account. It stays in the pool, which means `k`
actually increases slightly on every trade, and that growth is what LPs earn.

### Worked example (real numbers from the test suite)

The test fixtures create a HAY/ETH pool with `ETH_RESERVE = 5 ETH` and `HAY_RESERVE = 10 HAY`
(`uni/v1-contracts/tests/constants.py:7-8`). Spot price: 10/5 = **2 HAY per ETH**.

Sell exactly 1 ETH into it:

| Step | Value |
|---|---|
| Input `Δx` | 1 ETH |
| Input after fee `γ·Δx` | 0.997 ETH |
| `Δy = (0.997 × 10) / (5 + 0.997)` | 9.97 / 5.997 = **1.662497915624478906 HAY** |
| Without any fee it would be | 10 / 6 = 1.666666666666666666 HAY |
| Difference (the fee, kept by LPs) | 0.00416875104218776 HAY |

That output, to the wei, is the constant `HAY_BOUGHT = 1662497915624478906` in
`uni/v1-contracts/tests/constants.py:14`.

Check the invariant afterwards:

```
before:  5 × 10                         = 50.000000000000000000 (×10^36)
after:   6 × 8.337502084375521094       = 50.025012506253126564 (×10^36)
```

`k` grew by 0.05%. That growth is distributed to LPs automatically, because every LP token now
redeems for a slightly larger slice of a slightly larger pot. There is no "claim fees" function in
V1 — and there is none in V2 either. Fees are simply never removed.

Note also that the effective price paid was 1 ETH / 1.6625 HAY = 0.6015 ETH per HAY, versus the
0.5 ETH per HAY spot price before the trade. That gap is slippage plus fee, and it is why every
V1 entry point takes a `min_tokens` or `max_tokens` guard.

---

## 1. `uniswap_factory.vy` — every function

46 lines. It is a registry plus a deployer, and it holds no funds.

### Storage and events

```vyper
NewExchange: event({token: indexed(address), exchange: indexed(address)})

exchangeTemplate: public(address)
tokenCount: public(uint256)
token_to_exchange: address[address]
exchange_to_token: address[address]
id_to_token: address[uint256]
```
`uni/v1-contracts/contracts/uniswap_factory.vy:4-10`

| Field | Purpose |
|---|---|
| `exchangeTemplate` | The one deployed `uniswap_exchange` whose code every clone runs. Public. |
| `tokenCount` | Number of exchanges created. Doubles as the id counter. Public. |
| `token_to_exchange` | The forward index. **Not public** — read via `getExchange`. |
| `exchange_to_token` | The reverse index, so a contract can prove an address is a real exchange. |
| `id_to_token` | Enumeration: 1..`tokenCount` → token address, for UIs that want to list markets. |

Note `token_to_exchange`, `exchange_to_token` and `id_to_token` are private storage with explicit
getters (`getExchange`, `getToken`, `getTokenWithId`). That is a deliberate ABI choice — the
getter names are part of the interface other contracts compile against.

### `initializeFactory(template)` — `contracts/uniswap_factory.vy:13`

```vyper
@public
def initializeFactory(template: address):
    assert self.exchangeTemplate == ZERO_ADDRESS
    assert template != ZERO_ADDRESS
    self.exchangeTemplate = template
```

- **Inputs:** the address of an already-deployed exchange contract to use as the code template.
- **Checks:** template not yet set (so this is one-shot, forever), and not the zero address.
- **State writes:** `exchangeTemplate`.
- **External calls:** none.
- **Returns/emits:** nothing.

There is **no access control**. Whoever calls this first wins, permanently. In practice the
deployer called it in the same transaction bundle as deployment. The `assert self.exchangeTemplate
== ZERO_ADDRESS` is the only protection, and the factory test at
`uni/v1-contracts/tests/exchange/test_factory.py:8-9` confirms a second call reverts.

This is the V1 equivalent of a constructor, needed because the factory itself was deployed with a
plain constructor-less flow. V2 sets the equivalent (`feeToSetter`) in a real constructor.

### `createExchange(token) -> address` — `contracts/uniswap_factory.vy:19`

```vyper
@public
def createExchange(token: address) -> address:
    assert token != ZERO_ADDRESS
    assert self.exchangeTemplate != ZERO_ADDRESS
    assert self.token_to_exchange[token] == ZERO_ADDRESS
    exchange: address = create_with_code_of(self.exchangeTemplate)
    Exchange(exchange).setup(token)
    self.token_to_exchange[token] = exchange
    self.exchange_to_token[exchange] = token
    token_id: uint256 = self.tokenCount + 1
    self.tokenCount = token_id
    self.id_to_token[token_id] = token
    log.NewExchange(token, exchange)
    return exchange
```

- **Inputs:** the ERC20 to create a market for.
- **Checks:** token is not zero; the factory has been initialized; **no exchange exists yet for
  this token** (one market per token, enforced forever).
- **State writes:** both index mappings, `tokenCount`, `id_to_token`.
- **External calls:** deploys a clone, then calls `setup(token)` on it.
- **Returns/emits:** the new exchange address; `NewExchange(token, exchange)`.

**Permissionless.** Anyone can list any token. There is no allowlist, no fee, no governance. That
was a genuinely radical property in 2018 and it is preserved in every later version.

#### `create_with_code_of` and why `setup` exists

`create_with_code_of(target)` is Vyper 0.1.x's clone primitive: it deploys a **minimal forwarder
proxy** that `DELEGATECALL`s into `target`. Each clone therefore has:

- its **own storage** (its own reserves, its own LP balances), and
- **shared code** (the template's), so deployment costs a few hundred bytes of gas instead of
  redeploying 496 lines of logic.

The catch is written directly in the source comment:

```vyper
# @dev This function acts as a contract constructor which is not currently supported in contracts deployed
#      using create_with_code_of(). It is called once by the factory during contract creation.
```
`contracts/uniswap_exchange.vy:29-30`

A proxy runs the template's *runtime* code, so the template's *constructor* never executes in the
clone's context. Initialization has to be a normal function, called immediately after deployment.
`setup` is that function, and it guards itself against being called twice
(`contracts/uniswap_exchange.vy:33`).

**Compared to V2's CREATE2.** V1 uses plain `CREATE`, so an exchange's address depends on the
factory's nonce and is **not predictable**. To find the HAY market you must call
`factory.getExchange(HAY)` — an on-chain SLOAD and, for off-chain callers, an RPC round trip.
V2 deploys pairs with `CREATE2` using `keccak256(token0, token1)` as the salt, which makes the
address a pure function of the two token addresses. That is what enables `UniswapV2Library.pairFor`
to compute a pair address locally with no storage read and no network call — a meaningful gas and
latency win for routers, and the reason V2 routers can chain hops cheaply.

### The three view functions

```vyper
@public
@constant
def getExchange(token: address) -> address:
    return self.token_to_exchange[token]
```
`contracts/uniswap_factory.vy:35-36` — token → exchange. Returns zero if the token has no market.
This is the lookup used inside the exchange itself for token→token routing
(`contracts/uniswap_exchange.vy:293`).

```vyper
def getToken(exchange: address) -> address:      # contracts/uniswap_factory.vy:40-41
def getTokenWithId(token_id: uint256) -> address # contracts/uniswap_factory.vy:45-46
```

`getToken` is the reverse lookup — its real use is letting a third-party contract verify that some
address really is a factory-blessed exchange before trusting it. `getTokenWithId` exists purely for
enumeration (ids run 1..`tokenCount`; id 0 is never assigned, since `token_id` starts at
`tokenCount + 1` on `contracts/uniswap_factory.vy:27`).

**Missing:** there is no `getExchangeWithId`, so enumerating markets takes two calls per entry
(`getTokenWithId` then `getExchange`). V2's factory exposes `allPairs(uint)` directly.

---

## 2. `uniswap_exchange.vy` — every function

496 lines, and it is simultaneously a pool, a router and an ERC20. All paths below are relative to
`uni/v1-contracts/`.

### 2.1 Storage, events, and `setup`

```vyper
name: public(bytes32)                             # Uniswap V1
symbol: public(bytes32)                           # UNI-V1
decimals: public(uint256)                         # 18
totalSupply: public(uint256)                      # total number of UNI in existence
balances: uint256[address]                        # UNI balance of an address
allowances: (uint256[address])[address]           # UNI allowance of one address on another
token: address(ERC20)                             # address of the ERC20 token traded on this contract
factory: Factory                                  # interface for the factory that created this contract
```
`contracts/uniswap_exchange.vy:20-27`

Two things to notice immediately.

**There are no reserve variables.** No `reserve0`, no `reserve1`. Every single function that needs
to know the pool's state reads it live:

- ETH reserve = `self.balance` (the contract's actual ether balance)
- token reserve = `self.token.balanceOf(self)` (an external `STATICCALL` on every swap)

This is the most consequential storage decision in V1 and it drives four separate limitations
covered later: no price oracle is possible (§4), donated tokens silently become reserves (§5),
every swap pays for an external call, and fee-on-transfer tokens are handled by accident rather
than by design. V2 caches `reserve0`/`reserve1` in a single packed storage slot precisely to fix
this, and adds `sync()`/`skim()` to reconcile the cache with reality.

**`name` and `symbol` are `bytes32`, not `string`.** Vyper 0.1.x had poor dynamic-string support,
so they are stored as raw padded bytes and set as hex literals in `setup`. Every V1 LP token in
existence is called "Uniswap V1" / "UNI-V1" — they are indistinguishable in a wallet. V2 gives each
pair the same problem (`Uniswap V2`/`UNI-V2`) but at least uses real `string` types.

**Events** (`contracts/uniswap_exchange.vy:13-18`):

| Event | Emitted by | Meaning |
|---|---|---|
| `TokenPurchase(buyer, eth_sold, tokens_bought)` | ETH→token swaps | someone bought the token |
| `EthPurchase(buyer, tokens_sold, eth_bought)` | token→ETH swaps *and the first leg of token→token* | someone sold the token |
| `AddLiquidity(provider, eth_amount, token_amount)` | `addLiquidity` | deposit |
| `RemoveLiquidity(provider, eth_amount, token_amount)` | `removeLiquidity` | withdrawal |
| `Transfer(_from, _to, _value)` | LP token moves, incl. mint/burn | standard ERC20 |
| `Approval(_owner, _spender, _value)` | `approve` | standard ERC20 |

Every field of the first four events is `indexed`. Vyper 0.1.x allowed more than three indexed
args in its own event encoding, which means these logs have **no data section at all** — everything
is a topic. Off-chain indexers must decode from topics, and you cannot filter on, say,
`eth_sold > X` cheaply. V2 uses a conventional 3-topic layout with the amounts in data.

Note there is **no `Swap` event**: an ETH→token trade emits `TokenPurchase`, a token→ETH trade emits
`EthPurchase`, and a token→token trade emits `EthPurchase` on the first exchange and
`TokenPurchase` on the second. Reconstructing a token→token trade from logs requires stitching two
events from two contracts.

#### `setup(token_addr)` — `contracts/uniswap_exchange.vy:32`

```vyper
@public
def setup(token_addr: address):
    assert (self.factory == ZERO_ADDRESS and self.token == ZERO_ADDRESS) and token_addr != ZERO_ADDRESS
    self.factory = msg.sender
    self.token = token_addr
    self.name = 0x556e697377617020563100000000000000000000000000000000000000000000
    self.symbol = 0x554e492d56310000000000000000000000000000000000000000000000000000
    self.decimals = 18
```

- **Inputs:** the ERC20 this exchange will trade.
- **Checks:** both `factory` and `token` are still zero (one-shot), and the token is not zero.
- **State writes:** `factory` (= `msg.sender`, i.e. whoever deployed the clone), `token`, `name`,
  `symbol`, `decimals`.
- **External calls:** none.
- **Returns/emits:** nothing.

Those two hex literals decode to `Uniswap V1` and `UNI-V1` (right-padded with zero bytes), which
`uni/v1-contracts/tests/exchange/test_factory.py:27-28` asserts against.

**`setup` is `@public` and has no caller restriction.** Anyone may call it on a fresh clone — the
only protection is the one-shot assert. Since the factory calls `setup` in the same transaction as
`create_with_code_of` (`contracts/uniswap_factory.vy:23-24`), there is no window to front-run it.
But it does mean you can deploy your *own* clone of the template outside the factory and set it up
yourself. Such an exchange would work mechanically, but note the guard on
`contracts/uniswap_exchange.vy:66` (discussed next) which stops it from ever bootstrapping
liquidity via `addLiquidity`, and `getToken` on the real factory would return zero for it. The
factory test at `test_factory.py:25` confirms `setup` reverts on an already-initialized exchange.

### 2.2 The LP token, built in

The exchange **is** its own LP token — `contracts/uniswap_exchange.vy:469-496`. There is no
separate contract, no `UniswapV1ERC20.vy`. Supply changes only ever happen inside `addLiquidity`
and `removeLiquidity`; there is no external `mint` or `burn`.

```vyper
@public
@constant
def balanceOf(_owner : address) -> uint256:      # :469
    return self.balances[_owner]

@public
def transfer(_to : address, _value : uint256) -> bool:    # :473
    self.balances[msg.sender] -= _value
    self.balances[_to] += _value
    log.Transfer(msg.sender, _to, _value)
    return True

@public
def transferFrom(_from : address, _to : address, _value : uint256) -> bool:  # :480
    self.balances[_from] -= _value
    self.balances[_to] += _value
    self.allowances[_from][msg.sender] -= _value
    log.Transfer(_from, _to, _value)
    return True

@public
def approve(_spender : address, _value : uint256) -> bool:   # :488
    self.allowances[msg.sender][_spender] = _value
    log.Approval(msg.sender, _spender, _value)
    return True

@public
@constant
def allowance(_owner : address, _spender : address) -> uint256:  # :495
    return self.allowances[_owner][_spender]
```

Read those carefully, because what is *absent* is the interesting part:

- **No explicit balance check** in `transfer`. Vyper 0.1.x reverts on `uint256` underflow, so
  `self.balances[msg.sender] -= _value` is the check. It works, but the revert carries no reason
  string. The test at `tests/exchange/test_liquidity_pool.py:54` relies on exactly this.
- **No explicit allowance check** in `transferFrom` either — same underflow trick on
  `self.allowances[_from][msg.sender]`.
- **No zero-address guard.** You can `transfer` LP tokens to `ZERO_ADDRESS`. Because `totalSupply`
  is untouched by `transfer`, doing so **permanently locks the underlying ETH and tokens** — they
  stay in the pool, redeemable by nobody, which silently increases the redemption value of every
  remaining LP token. It is a donation with extra steps, and it is a real footgun.
- **The classic `approve` race** is present: `approve` sets rather than adjusts, so the
  change-allowance front-running issue applies. No `increaseAllowance`/`decreaseAllowance`.
- **No `permit`.** Every LP interaction needs a separate approval transaction. V2 added EIP-2612
  `permit` to the pair token specifically so the router could do gasless-approval removals.
- `transferFrom` does not special-case `_from == msg.sender`, so self-transfers still burn
  allowance.

Contrast with V2, where the LP token lives in `UniswapV2ERC20.sol` as a proper inherited contract
with `permit`. Splitting it out is mostly hygiene, but it also let V2 reuse the token logic and
keep `UniswapV2Pair.sol` focused on pool mechanics.

### 2.3 `addLiquidity` — `contracts/uniswap_exchange.vy:48`

Signature: `addLiquidity(min_liquidity, max_tokens, deadline) -> uint256`, `@payable`.

Two completely different code paths depending on whether the pool already exists.

```vyper
@public
@payable
def addLiquidity(min_liquidity: uint256, max_tokens: uint256, deadline: timestamp) -> uint256:
    assert deadline > block.timestamp and (max_tokens > 0 and msg.value > 0)
    total_liquidity: uint256 = self.totalSupply
    if total_liquidity > 0:
        assert min_liquidity > 0
        eth_reserve: uint256(wei) = self.balance - msg.value
        token_reserve: uint256 = self.token.balanceOf(self)
        token_amount: uint256 = msg.value * token_reserve / eth_reserve + 1
        liquidity_minted: uint256 = msg.value * total_liquidity / eth_reserve
        assert max_tokens >= token_amount and liquidity_minted >= min_liquidity
        self.balances[msg.sender] += liquidity_minted
        self.totalSupply = total_liquidity + liquidity_minted
        assert self.token.transferFrom(msg.sender, self, token_amount)
        log.AddLiquidity(msg.sender, msg.value, token_amount)
        log.Transfer(ZERO_ADDRESS, msg.sender, liquidity_minted)
        return liquidity_minted
```
`contracts/uniswap_exchange.vy:48-63`

**Shared checks (line 49):** `deadline > block.timestamp` — note this is **strict**, unlike the
swap functions which use `>=` (e.g. `contracts/uniswap_exchange.vy:128`). Passing
`deadline = block.timestamp` succeeds on a swap and reverts on `addLiquidity`. Also `max_tokens > 0`
and `msg.value > 0`, so you can never add one-sided liquidity.

**Subsequent-provider branch (`total_liquidity > 0`):**

- **`eth_reserve = self.balance - msg.value`** — this is the crux of reading balances live.
  By the time the function body runs, `msg.value` has *already* been added to `self.balance`, so
  the pre-trade reserve must be recovered by subtraction. This pattern repeats in every payable
  function in the contract.
- **`token_amount = msg.value * token_reserve / eth_reserve + 1`** — you must deposit at the
  current ratio. The **`+ 1` rounds up**, charging the depositor at most 1 wei of token more than
  the exact ratio. That extra wei stays in the pool, so rounding always favours existing LPs. The
  test asserts this precisely: after a deposit the pool holds
  `HAY_RESERVE + HAY_ADDED + 1` (`tests/exchange/test_liquidity_pool.py:52`).
- **`liquidity_minted = msg.value * total_liquidity / eth_reserve`** — floor division, rounding
  **down**, again favouring existing LPs. LP tokens are minted in proportion to the ETH you add,
  not the tokens.
- **Guards:** `max_tokens >= token_amount` (your slippage protection on the token side — the ratio
  can move between signing and mining) and `liquidity_minted >= min_liquidity`. The `assert
  min_liquidity > 0` on line 52 forces you to actually set the guard; passing 0 reverts
  (`tests/exchange/test_liquidity_pool.py:41`).
- **Ordering:** LP balances and `totalSupply` are written **before** `transferFrom` pulls the
  tokens (lines 58-60). See §5 for why that ordering matters with callback tokens.

**First-provider branch:**

```vyper
    else:
        assert (self.factory != ZERO_ADDRESS and self.token != ZERO_ADDRESS) and msg.value >= 1000000000
        assert self.factory.getExchange(self.token) == self
        token_amount: uint256 = max_tokens
        initial_liquidity: uint256 = as_unitless_number(self.balance)
        self.totalSupply = initial_liquidity
        self.balances[msg.sender] = initial_liquidity
        assert self.token.transferFrom(msg.sender, self, token_amount)
        log.AddLiquidity(msg.sender, msg.value, token_amount)
        log.Transfer(ZERO_ADDRESS, msg.sender, initial_liquidity)
        return initial_liquidity
```
`contracts/uniswap_exchange.vy:64-74`

- **`min_liquidity` is ignored here** — the doc comment on line 41 says so explicitly. There is no
  prior ratio to slip against.
- **`token_amount = max_tokens` exactly.** The first provider deposits the full amount they
  authorised. Whatever ratio of ETH to tokens they choose *becomes the price*. Deposit 5 ETH and
  10 HAY and the market opens at 2 HAY/ETH. There is no oracle check, no sanity check. Setting a
  bad initial price just means arbitrageurs immediately take the difference.
- **`initial_liquidity = as_unitless_number(self.balance)`** — LP supply is set to the contract's
  **entire ETH balance in wei**, not to `msg.value`. Normally identical. But ETH can be forced into
  a contract via `SELFDESTRUCT` without executing code, so a griefer can pre-fund the exchange and
  inflate the first provider's LP supply relative to the ETH they actually contributed. The tokens
  side is unaffected. It is a curiosity rather than a serious attack (the griefer donates their own
  ETH to the first LP), but it is the same *class* of bug as V2's first-depositor share inflation,
  which V2 addresses by burning `MINIMUM_LIQUIDITY = 1000` LP tokens to the zero address.
- **`msg.value >= 1000000000`** (1 gwei) — a dust floor, so the pool cannot open with a
  pathologically small reserve where integer rounding dominates.
- **`assert self.factory.getExchange(self.token) == self`** (line 66) — the exchange refuses to
  bootstrap unless the factory's registry points back at it. This is what neutralises a
  self-deployed clone made outside the factory: it can be `setup`, but it can never receive its
  first liquidity. Nice, cheap defence.
- Note `self.totalSupply` and `self.balances[msg.sender]` are **assigned**, not incremented — safe
  only because this branch requires `total_liquidity == 0`.

### 2.4 `removeLiquidity` — `contracts/uniswap_exchange.vy:83`

```vyper
@public
def removeLiquidity(amount: uint256, min_eth: uint256(wei), min_tokens: uint256, deadline: timestamp) -> (uint256(wei), uint256):
    assert (amount > 0 and deadline > block.timestamp) and (min_eth > 0 and min_tokens > 0)
    total_liquidity: uint256 = self.totalSupply
    assert total_liquidity > 0
    token_reserve: uint256 = self.token.balanceOf(self)
    eth_amount: uint256(wei) = amount * self.balance / total_liquidity
    token_amount: uint256 = amount * token_reserve / total_liquidity
    assert eth_amount >= min_eth and token_amount >= min_tokens
    self.balances[msg.sender] -= amount
    self.totalSupply = total_liquidity - amount
    send(msg.sender, eth_amount)
    assert self.token.transfer(msg.sender, token_amount)
    log.RemoveLiquidity(msg.sender, eth_amount, token_amount)
    log.Transfer(msg.sender, ZERO_ADDRESS, amount)
    return eth_amount, token_amount
```
`contracts/uniswap_exchange.vy:83-97`

- **Inputs:** LP tokens to burn, minimum ETH and minimum tokens to accept, deadline.
- **Checks:** all four of `amount > 0`, `deadline > block.timestamp` (strict again),
  `min_eth > 0`, `min_tokens > 0`; supply is non-zero; and the two slippage guards.
- **State writes:** decrements the caller's LP balance (underflow reverts if they lack the
  balance — `tests/exchange/test_liquidity_pool.py:65`) and `totalSupply`.
- **External calls:** `balanceOf`, then `send` (ETH), then `token.transfer`.
- **Returns/emits:** `(eth_amount, token_amount)`; `RemoveLiquidity` and a burn `Transfer`.

**Pure pro-rata, no fee.** Withdrawal is `amount / total_liquidity` of *whatever the pool currently
holds*. Since fees accumulated into those balances, this is exactly how LPs get paid.

**Both divisions floor**, so you withdraw slightly less than your mathematical share and the dust
stays with remaining LPs. Consistent with the rest of the contract: rounding always favours the pool.

**Effects precede interactions** for the LP accounting (lines 91-92 before 93-94), which is the
right order. But note ETH goes out before the token transfer, and see §5 on `send`'s gas stipend.

**Requiring `min_eth > 0` and `min_tokens > 0`** has an odd side effect: if a pool somehow ends up
with a zero balance on one side, `removeLiquidity` can never succeed, because the corresponding
`assert *_amount >= min_*` can never be satisfied with a positive minimum. In practice `x·y = k`
prevents a side from hitting zero through trading.

### 2.5 Pricing: `getInputPrice` and `getOutputPrice`

Both are `@private @constant`. They are pure arithmetic — no storage, no reserves passed implicitly.
Every swap in the contract bottoms out in one of these two.

#### `getInputPrice` — exact input — `contracts/uniswap_exchange.vy:106`

```vyper
@private
@constant
def getInputPrice(input_amount: uint256, input_reserve: uint256, output_reserve: uint256) -> uint256:
    assert input_reserve > 0 and output_reserve > 0
    input_amount_with_fee: uint256 = input_amount * 997
    numerator: uint256 = input_amount_with_fee * output_reserve
    denominator: uint256 = (input_reserve * 1000) + input_amount_with_fee
    return numerator / denominator
```

**Derivation.** Let `x` = `input_reserve`, `y` = `output_reserve`, `Δx` = `input_amount`, and
`γ = 997/1000`. Only the post-fee input counts toward the invariant:

```
(x + γΔx)(y − Δy) = x·y
  y − Δy = x·y / (x + γΔx)
     Δy  = y − x·y/(x + γΔx)
     Δy  = (γΔx · y) / (x + γΔx)
```

Multiply numerator and denominator by 1000 to stay in integers:

```
     Δy  = (997·Δx · y) / (1000·x + 997·Δx)
```

which is the code, line for line. **Floor division** truncates the output, so the trader receives
slightly less than exact and the remainder stays in the pool.

The `assert` on line 107 is the only thing preventing division by zero (`denominator` would be 0 if
both `input_reserve` and `input_amount` were 0) and prevents quoting against an empty pool.

**Overflow.** `numerator = input_amount * 997 * output_reserve` is a raw `uint256` product with no
512-bit intermediate. With 18-decimal tokens it overflows if `Δx · y` exceeds roughly `2^256/997`,
i.e. around `1.16e74`. Real reserves are nowhere near that, and Vyper reverts on overflow rather
than wrapping, so the failure mode is a revert rather than a wrong price. V3 needed `FullMath`'s
512-bit `mulDiv` because its Q64.96 fixed-point numbers genuinely approach those magnitudes.

#### `getOutputPrice` — exact output — `contracts/uniswap_exchange.vy:120`

```vyper
@private
@constant
def getOutputPrice(output_amount: uint256, input_reserve: uint256, output_reserve: uint256) -> uint256:
    assert input_reserve > 0 and output_reserve > 0
    numerator: uint256 = input_reserve * output_amount * 1000
    denominator: uint256 = (output_reserve - output_amount) * 997
    return numerator / denominator + 1
```

**Derivation.** Same invariant, solved the other way. Given desired `Δy`, find required `Δx`:

```
(x + γΔx)(y − Δy) = x·y
      x + γΔx = x·y / (y − Δy)
          γΔx = x·y/(y − Δy) − x  =  x·Δy / (y − Δy)
           Δx = x·Δy / (γ(y − Δy))
           Δx = (1000·x·Δy) / (997·(y − Δy))
```

**The `+ 1` is a ceiling.** Floor division would let the trader pay fractionally less than required,
which would shrink `k` and steal from LPs. `floor(a/b) + 1` rounds up in every case except exact
division, where it overcharges by exactly 1 wei. Uniswap chose the cheap, always-safe version over
a correct ceiling (`(a + b - 1)/b`). Consistent with everything else: **when in doubt, favour the
pool.**

**The subtraction is a safety check in disguise.** If `output_amount >= output_reserve`, then
`output_reserve - output_amount` underflows and Vyper reverts. So "you cannot buy the entire
reserve" is enforced for free by the type system, with no explicit assert.

#### Sanity check against the test suite

`tests/conftest.py:127-141` reimplements both functions in Python as the `swap_input` and
`swap_output` fixtures, and the tests assert the contract matches. Reproducing those with
`ETH_RESERVE = 5e18`, `HAY_RESERVE = 10e18`:

| Call | Result | Matches constant |
|---|---|---|
| `getInputPrice(1e18, 5e18, 10e18)` | 1662497915624478906 | `HAY_BOUGHT`, `constants.py:14` |
| `getInputPrice(2e18, 10e18, 5e18)` | 831248957812239453 | `ETH_BOUGHT`, `constants.py:20` |
| `getOutputPrice(1e18, 5e18, 10e18)` | 557227237267357629 wei | — |

### 2.6 The swap matrix — the naming scheme

V1 exposes **16 public swap functions** plus the fallback. They look overwhelming until you see
that the names are a three-axis grid, and that all 16 are thin wrappers over just **6 private
functions**.

**Axis 1 — what you are trading (4 values):**

| Prefix | Meaning |
|---|---|
| `ethToToken` | sell ETH, buy this exchange's token |
| `tokenToEth` | sell this exchange's token, buy ETH |
| `tokenToToken` | sell this token, buy another token — target found **via the factory** |
| `tokenToExchange` | same, but you pass the **target exchange address directly** |

**Axis 2 — who receives the output (2 values):**

| Infix | Meaning |
|---|---|
| `Swap` | output goes to `msg.sender` |
| `Transfer` | output goes to a `recipient` argument |

**Axis 3 — which side is exact (2 values):**

| Suffix | Meaning | You specify | Guard argument |
|---|---|---|---|
| `Input` | exact input, variable output | how much you sell | `min_*_bought` (revert if you get less) |
| `Output` | exact output, variable input | how much you buy | `max_*_sold` (revert if it costs more) |

4 × 2 × 2 = 16. In modern terms `Input` is `swapExactTokensForTokens` and `Output` is
`swapTokensForExactTokens`; V2 kept both semantics but moved them to the periphery router.

#### The complete matrix

| Function | Line | Delegates to | Recipient | Exactness |
|---|---|---|---|---|
| `ethToTokenSwapInput` | 151 | `ethToTokenInput` | sender | exact in |
| `ethToTokenTransferInput` | 162 | `ethToTokenInput` | arg | exact in |
| `ethToTokenSwapOutput` | 186 | `ethToTokenOutput` | sender | exact out |
| `ethToTokenTransferOutput` | 197 | `ethToTokenOutput` | arg | exact out |
| `tokenToEthSwapInput` | 221 | `tokenToEthInput` | sender | exact in |
| `tokenToEthTransferInput` | 232 | `tokenToEthInput` | arg | exact in |
| `tokenToEthSwapOutput` | 255 | `tokenToEthOutput` | sender | exact out |
| `tokenToEthTransferOutput` | 266 | `tokenToEthOutput` | arg | exact out |
| `tokenToTokenSwapInput` | 292 | `tokenToTokenInput` | sender | exact in |
| `tokenToTokenTransferInput` | 307 | `tokenToTokenInput` | arg | exact in |
| `tokenToTokenSwapOutput` | 334 | `tokenToTokenOutput` | sender | exact out |
| `tokenToTokenTransferOutput` | 349 | `tokenToTokenOutput` | arg | exact out |
| `tokenToExchangeSwapInput` | 363 | `tokenToTokenInput` | sender | exact in |
| `tokenToExchangeTransferInput` | 378 | `tokenToTokenInput` | arg | exact in |
| `tokenToExchangeSwapOutput` | 392 | `tokenToTokenOutput` | sender | exact out |
| `tokenToExchangeTransferOutput` | 407 | `tokenToTokenOutput` | arg | exact out |
| `__default__` | 141 | `ethToTokenInput` | sender | exact in |

All 16 wrappers are one or two lines. The `tokenToToken*` group does a factory lookup
(`self.factory.getExchange(token_addr)`) and then calls the same private function the
`tokenToExchange*` group calls directly. The **only** difference between the two groups is where
the target exchange address comes from.

The `Transfer` wrappers add a recipient check. Note the inconsistency:

- `ethToTokenTransferInput:163` and `ethToTokenTransferOutput:198` check
  `recipient != self and recipient != ZERO_ADDRESS`
- `tokenToEthTransferInput:233` and `tokenToEthTransferOutput:267` check both as well
- `tokenToExchangeTransferInput:379` and `tokenToExchangeTransferOutput:408` check
  **only** `recipient != self`
- `tokenToTokenTransferInput:307` and `tokenToTokenTransferOutput:349` check **neither**

For the token→token paths that is still safe, because the second leg ultimately lands in the
target exchange's `ethToTokenTransferInput`/`Output`, which does perform the zero-address check
(`contracts/uniswap_exchange.vy:163`, `:198`). The `recipient != self` check exists because sending
output back into the pool would corrupt the live-balance accounting.

### 2.7 The six private swap engines

#### `ethToTokenInput` — `contracts/uniswap_exchange.vy:127`

```vyper
@private
def ethToTokenInput(eth_sold: uint256(wei), min_tokens: uint256, deadline: timestamp, buyer: address, recipient: address) -> uint256:
    assert deadline >= block.timestamp and (eth_sold > 0 and min_tokens > 0)
    token_reserve: uint256 = self.token.balanceOf(self)
    tokens_bought: uint256 = self.getInputPrice(as_unitless_number(eth_sold), as_unitless_number(self.balance - eth_sold), token_reserve)
    assert tokens_bought >= min_tokens
    assert self.token.transfer(recipient, tokens_bought)
    log.TokenPurchase(buyer, eth_sold, tokens_bought)
    return tokens_bought
```

- **Inputs:** ETH already received (`msg.value`), minimum acceptable output, deadline, and the
  `buyer`/`recipient` split so the public wrappers can set them.
- **Checks:** deadline (`>=`, non-strict), non-zero input, non-zero minimum, and the slippage
  guard `tokens_bought >= min_tokens`.
- **State writes:** **none directly.** The ETH is already in `self.balance`; the token reserve
  changes as a side effect of the outbound `transfer`. This is what "no reserve variables" buys —
  and costs.
- **External calls:** `token.balanceOf(self)`, then `token.transfer(recipient, ...)`.
- **Returns/emits:** tokens bought; `TokenPurchase(buyer, eth_sold, tokens_bought)`.

The key line is `self.balance - eth_sold`: `msg.value` is already in the balance, so the *pre-trade*
ETH reserve is recovered by subtracting it. Get this wrong and you would price the trade against a
reserve that already includes the trade.

#### `__default__` — `contracts/uniswap_exchange.vy:141`

```vyper
@public
@payable
def __default__():
    self.ethToTokenInput(msg.value, 1, block.timestamp, msg.sender, msg.sender)
```

Plain-sending ETH to a V1 exchange **executes a market buy**. The doc comment above it is blunt:
"User cannot specify minimum output or deadline."

- `min_tokens = 1` — effectively **no slippage protection at all**. You will accept any non-zero
  amount of token. Sending a large amount of ETH this way is an open invitation to a sandwich.
- `deadline = block.timestamp` — this passes because line 128 uses `>=` rather than `>`. That
  asymmetry with `addLiquidity`'s strict `>` exists precisely so the fallback can work.
- It reverts on a zero-value send, since line 128 requires `eth_sold > 0`
  (`tests/exchange/test_eth_to_token.py:17` asserts this).
- It returns nothing, so a contract calling it cannot learn how many tokens it got except by
  checking its balance.

Convenient for a 2018 wallet with no dapp support. A liability today. V2 has no such fallback;
`UniswapV2Pair` cannot even receive plain ETH, since it deals in WETH.

#### `ethToTokenOutput` — `contracts/uniswap_exchange.vy:167`

```vyper
@private
def ethToTokenOutput(tokens_bought: uint256, max_eth: uint256(wei), deadline: timestamp, buyer: address, recipient: address) -> uint256(wei):
    assert deadline >= block.timestamp and (tokens_bought > 0 and max_eth > 0)
    token_reserve: uint256 = self.token.balanceOf(self)
    eth_sold: uint256 = self.getOutputPrice(tokens_bought, as_unitless_number(self.balance - max_eth), token_reserve)
    # Throws if eth_sold > max_eth
    eth_refund: uint256(wei) = max_eth - as_wei_value(eth_sold, 'wei')
    if eth_refund > 0:
        send(buyer, eth_refund)
    assert self.token.transfer(recipient, tokens_bought)
    log.TokenPurchase(buyer, as_wei_value(eth_sold, 'wei'), tokens_bought)
    return as_wei_value(eth_sold, 'wei')
```

For exact-output ETH→token you must **overpay up front** (`max_eth` = `msg.value`) and get change
back. The pre-trade reserve is `self.balance - max_eth`, since the whole `max_eth` arrived.

**The slippage check is the underflow, and the comment says so.** There is no
`assert eth_sold <= max_eth`. Instead, `max_eth - eth_sold` underflows and reverts when the price
moved against you. Elegant, and free.

**The refund goes to `buyer`, not `recipient`.** Correct — the person who paid gets the change,
while the goods go to the recipient. This matters in the token→token flow below.

#### `tokenToEthInput` — `contracts/uniswap_exchange.vy:202`

```vyper
@private
def tokenToEthInput(tokens_sold: uint256, min_eth: uint256(wei), deadline: timestamp, buyer: address, recipient: address) -> uint256(wei):
    assert deadline >= block.timestamp and (tokens_sold > 0 and min_eth > 0)
    token_reserve: uint256 = self.token.balanceOf(self)
    eth_bought: uint256 = self.getInputPrice(tokens_sold, token_reserve, as_unitless_number(self.balance))
    wei_bought: uint256(wei) = as_wei_value(eth_bought, 'wei')
    assert wei_bought >= min_eth
    send(recipient, wei_bought)
    assert self.token.transferFrom(buyer, self, tokens_sold)
    log.EthPurchase(buyer, tokens_sold, wei_bought)
    return wei_bought
```

No ETH arrives with the call, so the reserve is plain `self.balance` — **no subtraction**. The
input reserve is the token side and the output reserve is ETH, the mirror of `ethToTokenInput`.

**Ordering: ETH leaves before the tokens arrive** (line 208 before line 209). The pricing already
used the correct pre-trade reserves, so the arithmetic is right, but for a moment the pool has paid
out without being paid. Two things make this safe in practice: `send` forwards only the 2300 gas
stipend (see §5), and `transferFrom` reverts the whole transaction if the buyer cannot pay.

#### `tokenToEthOutput` — `contracts/uniswap_exchange.vy:237`

```vyper
@private
def tokenToEthOutput(eth_bought: uint256(wei), max_tokens: uint256, deadline: timestamp, buyer: address, recipient: address) -> uint256:
    assert deadline >= block.timestamp and eth_bought > 0
    token_reserve: uint256 = self.token.balanceOf(self)
    tokens_sold: uint256 = self.getOutputPrice(as_unitless_number(eth_bought), token_reserve, as_unitless_number(self.balance))
    # tokens sold is always > 0
    assert max_tokens >= tokens_sold
    send(recipient, eth_bought)
    assert self.token.transferFrom(buyer, self, tokens_sold)
    log.EthPurchase(buyer, tokens_sold, eth_bought)
    return tokens_sold
```

Here the slippage guard **is** explicit (`max_tokens >= tokens_sold`) because nothing was prepaid,
so there is no subtraction to underflow. The comment on line 241 notes `tokens_sold` is always
positive thanks to the `+1` in `getOutputPrice`. No refund logic is needed: the exact ETH requested
is sent and exactly the computed token amount is pulled.

#### `tokenToTokenInput` — `contracts/uniswap_exchange.vy:271`

This is where two pools get chained.

```vyper
@private
def tokenToTokenInput(tokens_sold: uint256, min_tokens_bought: uint256, min_eth_bought: uint256(wei), deadline: timestamp, buyer: address, recipient: address, exchange_addr: address) -> uint256:
    assert (deadline >= block.timestamp and tokens_sold > 0) and (min_tokens_bought > 0 and min_eth_bought > 0)
    assert exchange_addr != self and exchange_addr != ZERO_ADDRESS
    token_reserve: uint256 = self.token.balanceOf(self)
    eth_bought: uint256 = self.getInputPrice(tokens_sold, token_reserve, as_unitless_number(self.balance))
    wei_bought: uint256(wei) = as_wei_value(eth_bought, 'wei')
    assert wei_bought >= min_eth_bought
    assert self.token.transferFrom(buyer, self, tokens_sold)
    tokens_bought: uint256 = Exchange(exchange_addr).ethToTokenTransferInput(min_tokens_bought, deadline, recipient, value=wei_bought)
    log.EthPurchase(buyer, tokens_sold, wei_bought)
    return tokens_bought
```

- **Two slippage guards**, one per leg: `min_eth_bought` protects the intermediate ETH amount and
  `min_tokens_bought` is forwarded to the second exchange, which enforces it at
  `contracts/uniswap_exchange.vy:131`.
- **`assert exchange_addr != self and exchange_addr != ZERO_ADDRESS`** (line 273) is doing a lot of
  work. When called through `tokenToTokenSwapInput`, `exchange_addr` came from
  `factory.getExchange(token_addr)`. If you pass the *same* token, that returns this very exchange
  and the check catches it. If you pass a token with no market, it returns zero and the check
  catches that too. The test suite exercises exactly these three cases at
  `tests/exchange/test_token_to_token.py:36-41`.
- **Ordering here is correct**: tokens are pulled in (line 278) *before* the ETH is forwarded
  (line 279).
- **The second leg is a real external call carrying value.** `ethToTokenTransferInput` on the other
  exchange sees `msg.sender` = this exchange and `msg.value` = `wei_bought`, so it prices against
  its own reserves and sends its token straight to `recipient`. The user's tokens never touch this
  contract twice.
- **Logging is split across contracts**: this exchange emits `EthPurchase`; the other emits
  `TokenPurchase`.

#### `tokenToTokenOutput` — `contracts/uniswap_exchange.vy:312`

```vyper
@private
def tokenToTokenOutput(tokens_bought: uint256, max_tokens_sold: uint256, max_eth_sold: uint256(wei), deadline: timestamp, buyer: address, recipient: address, exchange_addr: address) -> uint256:
    assert deadline >= block.timestamp and (tokens_bought > 0 and max_eth_sold > 0)
    assert exchange_addr != self and exchange_addr != ZERO_ADDRESS
    eth_bought: uint256(wei) = Exchange(exchange_addr).getEthToTokenOutputPrice(tokens_bought)
    token_reserve: uint256 = self.token.balanceOf(self)
    tokens_sold: uint256 = self.getOutputPrice(as_unitless_number(eth_bought), token_reserve, as_unitless_number(self.balance))
    # tokens sold is always > 0
    assert max_tokens_sold >= tokens_sold and max_eth_sold >= eth_bought
    assert self.token.transferFrom(buyer, self, tokens_sold)
    eth_sold: uint256(wei) = Exchange(exchange_addr).ethToTokenTransferOutput(tokens_bought, deadline, recipient, value=eth_bought)
    log.EthPurchase(buyer, tokens_sold, eth_bought)
    return tokens_sold
```

Exact-output across two pools has to be solved **backwards**:

1. Ask the *destination* exchange what ETH it needs to deliver `tokens_bought` — a cross-contract
   view call to `getEthToTokenOutputPrice` (line 315).
2. Ask *this* pool what token input produces that much ETH (line 317).
3. Guard both legs (line 319), pull the tokens, then execute the second leg with exactly
   `eth_bought` attached.

**A subtle and load-bearing detail.** In step 3 the destination's `ethToTokenOutput` recomputes the
price and refunds `max_eth - eth_sold` to its `buyer` — and its `buyer` is **this exchange**, not
the user. Because both calls read the same state inside the same transaction, the quote from step 1
equals the charge in step 3 exactly, so the refund is always zero. If it ever were non-zero, `send`
would forward only 2300 gas into this contract's `__default__`, which tries to run a whole swap,
run out of gas, and revert the transaction. The design silently depends on quote-equals-charge
holding within one transaction.

### 2.8 The four `tokenToExchange*` variants

```vyper
# @dev Allows trades through contracts that were not deployed from the same factory.
@public
def tokenToExchangeSwapInput(tokens_sold: uint256, min_tokens_bought: uint256, min_eth_bought: uint256(wei), deadline: timestamp, exchange_addr: address) -> uint256:
    return self.tokenToTokenInput(tokens_sold, min_tokens_bought, min_eth_bought, deadline, msg.sender, msg.sender, exchange_addr)
```
`contracts/uniswap_exchange.vy:353-364`

These skip the factory lookup and let the caller name any target address
(lines 363, 378, 392, 407). The stated purpose is interoperability with exchanges from other
factories — for instance a future Uniswap deployment, or a compatible fork.

The trade-off is explicit: **the target is untrusted**. It only has to implement
`ethToTokenTransferInput` / `ethToTokenTransferOutput` / `getEthToTokenOutputPrice`. A malicious
target could return an arbitrary "tokens bought" number, or in the `Output` case quote a low
`getEthToTokenOutputPrice` and then behave differently. The user's protection is entirely their own
`min_tokens_bought` / `max_tokens_sold` arguments, plus the fact that this exchange's own leg is
priced honestly and its `transferFrom` pulls only `tokens_sold`. So the worst case is bounded by
the guards the caller supplies — which is exactly why passing 0 for those guards is rejected
(line 272).

### 2.9 Price getters and metadata

Four `@public @constant` quoting functions, used by UIs and by other contracts:

| Function | Line | Computes |
|---|---|---|
| `getEthToTokenInputPrice(eth_sold)` | 416 | `getInputPrice(eth_sold, self.balance, token_reserve)` |
| `getEthToTokenOutputPrice(tokens_bought)` | 426 | `getOutputPrice(tokens_bought, self.balance, token_reserve)` |
| `getTokenToEthInputPrice(tokens_sold)` | 437 | `getInputPrice(tokens_sold, token_reserve, self.balance)` |
| `getTokenToEthOutputPrice(eth_bought)` | 448 | `getOutputPrice(eth_bought, token_reserve, self.balance)` |

All four assert their argument is non-zero, and all four use **plain `self.balance`** with no
subtraction — correct, because no value is attached to a view call. Compare with
`ethToTokenInput:130`, which must subtract. Mixing these up is a classic AMM bug.

`getEthToTokenOutputPrice` is the one that gets called cross-contract, by `tokenToTokenOutput:315`.

```vyper
@public
@constant
def tokenAddress() -> address:                     # :456
    return self.token

@public
@constant
def factoryAddress() -> address(Factory):          # :462
    return self.factory
```

These exist so a counterparty can verify which market it is talking to, and so the
`exchange → factory → getToken` round trip can confirm an exchange is genuine.

**Crucially, these are the only price interfaces, and they are all instantaneous spot prices read
from live balances.** There is no accumulator, no historical data, no snapshot. Any contract using
a V1 exchange as a price feed is reading a number that a flash loan can move arbitrarily within a
single transaction. That is the single most important limitation of V1 and it is what
`price0CumulativeLast`/`price1CumulativeLast` in V2 exist to fix.

### 2.10 How token→token actually flows

```
   tokenToTokenSwapInput(2 HAY, min_den, min_eth, deadline, DEN)
   called on the HAY exchange
        |
        | 1. factory.getExchange(DEN)  ──────────────►  Factory  (contracts/uniswap_exchange.vy:293)
        |    returns DEN exchange address
        v
  +-------------------------+
  |     HAY exchange        |
  |  tokenToTokenInput      |  contracts/uniswap_exchange.vy:271
  |                         |
  |  2. price leg 1:        |
  |     getInputPrice(      |
  |       2 HAY,            |
  |       HAY reserve,      |
  |       ETH reserve)      |
  |     -> 0.8312... ETH    |
  |  3. assert >= min_eth   |
  |  4. transferFrom(user)  |──► HAY token: user's 2 HAY -> HAY exchange
  |     HAY reserve +2      |
  |  5. call with value ────┼───────────────┐
  +-------------------------+               │  msg.value = 0.8312 ETH
        emits EthPurchase                   │  msg.sender = HAY exchange
                                            v
                              +-----------------------------+
                              |       DEN exchange          |
                              | ethToTokenTransferInput     |  :162
                              |   -> ethToTokenInput        |  :127
                              |                             |
                              | 6. reserve = balance - value|
                              | 7. getInputPrice(0.8312 ETH,|
                              |      ETH reserve,           |
                              |      DEN reserve)           |
                              |    -> 2.8436... DEN         |
                              | 8. assert >= min_tokens     |
                              | 9. DEN.transfer(recipient) ─┼──► user receives DEN
                              +-----------------------------+
                                    emits TokenPurchase

   Net: user paid 2 HAY, received 2.8436 DEN. ETH existed only between steps 5 and 9.
   Fees paid: 0.3% in the HAY pool, then 0.3% again in the DEN pool.
```

The `tokenToExchange*` variants are the identical diagram with step 1 removed and the DEN exchange
address supplied by the caller.

---

## 3. End-to-end traces

All three traces use the repository's own fixture values, so you can run them against the test
suite and get identical numbers.

### 3.1 The first liquidity provider creates the market

The fixture at `tests/conftest.py:103-113` does exactly this. Account `a0` holds 100,000 HAY and
wants to open a HAY market at 2 HAY per ETH.

```
Step 0   factory.createExchange(HAY)                    contracts/uniswap_factory.vy:19
         ├─ assert HAY != 0, template set, no existing market
         ├─ create_with_code_of(template)      -> new clone address E
         ├─ E.setup(HAY)                                contracts/uniswap_exchange.vy:32
         │    factory = msg.sender (the factory)
         │    token   = HAY
         │    name/symbol/decimals set
         ├─ token_to_exchange[HAY] = E ; exchange_to_token[E] = HAY
         ├─ tokenCount 0 -> 1 ; id_to_token[1] = HAY
         └─ emit NewExchange(HAY, E)

Step 1   HAY.approve(E, 10e18)                          (on the HAY token, not the exchange)

Step 2   E.addLiquidity(0, 10e18, DEADLINE) with msg.value = 5e18
         contracts/uniswap_exchange.vy:48
         ├─ assert deadline > now, max_tokens > 0, msg.value > 0     :49
         ├─ total_liquidity = totalSupply = 0  -> ELSE branch        :64
         ├─ assert factory != 0, token != 0, msg.value >= 1 gwei     :65
         ├─ assert factory.getExchange(HAY) == self  ────► Factory   :66
         ├─ token_amount      = max_tokens = 10e18                   :67
         ├─ initial_liquidity = self.balance = 5e18 wei              :68
         ├─ totalSupply            = 5e18                            :69
         ├─ balances[a0]           = 5e18                            :70
         ├─ HAY.transferFrom(a0, E, 10e18) ────► HAY token           :71
         ├─ emit AddLiquidity(a0, 5e18, 10e18)                       :72
         └─ emit Transfer(0x0, a0, 5e18)                             :73
```

Final state, which `tests/exchange/test_liquidity_pool.py:34-37` asserts:

| Quantity | Value |
|---|---|
| `E.balance` (ETH reserve) | 5 ETH |
| `HAY.balanceOf(E)` (token reserve) | 10 HAY |
| `E.totalSupply()` | 5e18 |
| `E.balanceOf(a0)` | 5e18 |
| Opening price | 2 HAY per ETH |

Note the LP supply equals the **wei of ETH deposited**. That is an arbitrary but convenient
convention — it makes LP tokens roughly "ETH-denominated" at launch.

### 3.2 `ethToTokenSwapInput` — buying HAY with 1 ETH

Buyer `a1` calls `E.ethToTokenSwapInput(min_tokens = 1, DEADLINE)` with `msg.value = 1e18`.

```
E.ethToTokenSwapInput(1, DEADLINE) {value: 1e18}        contracts/uniswap_exchange.vy:151
  └─ ethToTokenInput(msg.value=1e18, min_tokens=1, deadline, buyer=a1, recipient=a1)   :127
       ├─ assert deadline >= now, eth_sold > 0, min_tokens > 0                          :128
       ├─ token_reserve = HAY.balanceOf(E)          ────► HAY token  = 10e18            :129
       ├─ eth_reserve   = self.balance - eth_sold   = 6e18 - 1e18   = 5e18              :130
       ├─ tokens_bought = getInputPrice(1e18, 5e18, 10e18)                              :106
       │      input_amount_with_fee = 1e18 * 997          = 9.97e20
       │      numerator             = 9.97e20 * 10e18     = 9.97e39
       │      denominator           = 5e18*1000 + 9.97e20 = 5.997e21
       │      return 9.97e39 / 5.997e21                   = 1662497915624478906
       ├─ assert 1662497915624478906 >= 1                    OK                         :131
       ├─ HAY.transfer(a1, 1662497915624478906)     ────► HAY token                     :132
       ├─ emit TokenPurchase(a1, 1e18, 1662497915624478906)                             :133
       └─ return 1662497915624478906
```

| | ETH reserve | HAY reserve | k (×10^36) |
|---|---|---|---|
| before | 5.000000000000000000 | 10.000000000000000000 | 50.000000000000000000 |
| after | 6.000000000000000000 | 8.337502084375521094 | 50.025012506253126564 |

- The buyer received **1.662497915624478906 HAY**, matching `HAY_BOUGHT` in `constants.py:14`
  and asserted in `tests/exchange/test_eth_to_token.py`.
- `k` rose by 0.05%. Nothing was transferred to a fee account; the growth simply sits in the
  reserves, raising what every LP token redeems for.
- Effective price 0.6015 ETH/HAY versus 0.5 spot beforehand and 0.7196 spot afterwards. On a pool
  this small, a 1 ETH trade is enormous — that is slippage, not fee.
- **Storage writes in the exchange: none.** The reserves changed only because ETH arrived and
  tokens left.

Sending 1 ETH to `E` with no calldata produces the identical result via `__default__`
(`contracts/uniswap_exchange.vy:141`) — but with `min_tokens = 1`, so with no protection against a
sandwich attacker moving the price first.

### 3.3 `tokenToTokenSwapInput` — 2 HAY to DEN across two pools

Both pools start at 5 ETH; HAY pool holds 10 HAY, DEN pool holds 20 DEN
(`tests/constants.py:7-9`). Buyer `a1` holds 2 HAY and has approved the HAY exchange.

```
HAY_E.tokenToTokenSwapInput(2e18, min_den=1, min_eth=1, DEADLINE, DEN)   :292
  ├─ exchange_addr = factory.getExchange(DEN) ──► Factory = DEN_E        :293
  └─ tokenToTokenInput(2e18, 1, 1, deadline, buyer=a1, recipient=a1, DEN_E)  :271
       ├─ assert deadline, tokens_sold>0, min_tokens_bought>0, min_eth_bought>0   :272
       ├─ assert DEN_E != self and DEN_E != 0                                     :273
       ├─ token_reserve = HAY.balanceOf(HAY_E) = 10e18                            :274
       ├─ eth_bought = getInputPrice(2e18, 10e18, 5e18)                           :275
       │      = (2e18*997 * 5e18) / (10e18*1000 + 2e18*997)
       │      = 9.97e39 / 1.1994e22 = 831248957812239453           (0.8312 ETH)
       ├─ assert 831248957812239453 >= 1                                          :277
       ├─ HAY.transferFrom(a1, HAY_E, 2e18)   ────► HAY token                     :278
       │      HAY_E now holds 12 HAY
       └─ DEN_E.ethToTokenTransferInput(1, deadline, a1) {value: 831248957812239453}  :279
            └─ ethToTokenInput(eth_sold=0.8312e18, 1, deadline, buyer=HAY_E, recipient=a1)  :127
                 ├─ token_reserve = DEN.balanceOf(DEN_E) = 20e18                  :129
                 ├─ eth_reserve = balance - eth_sold = 5.8312e18 - 0.8312e18 = 5e18  :130
                 ├─ tokens_bought = getInputPrice(831248957812239453, 5e18, 20e18)
                 │      = 2843678215834080602                      (2.8436 DEN)
                 ├─ assert >= 1                                                    :131
                 ├─ DEN.transfer(a1, 2843678215834080602) ────► DEN token          :132
                 └─ emit TokenPurchase(HAY_E, 0.8312e18, 2.8436e18)                :133
       ├─ emit EthPurchase(a1, 2e18, 831248957812239453)                           :280
       └─ return 2843678215834080602
```

Final balances, exactly as asserted in `tests/exchange/test_token_to_token.py:45-53`:

| Contract | ETH | Token |
|---|---|---|
| HAY exchange | 5 − 0.831248957812239453 = 4.168751042187760547 | 12 HAY |
| DEN exchange | 5 + 0.831248957812239453 = 5.831248957812239453 | 20 − 2.843678215834080602 = 17.156321784165919398 DEN |
| Buyer `a1` | unchanged (`INITIAL_ETH`) | 0 HAY, 2.843678215834080602 DEN |

`2843678215834080602` is `DEN_BOUGHT` at `tests/constants.py:24`. The buyer's **ETH balance is
completely unchanged** — the intermediate 0.8312 ETH moved directly from one exchange to the other
as `msg.value`, never touching the user's account. That is the whole trick of hub-and-spoke
routing.

Two fees were paid. A single hypothetical HAY/DEN pool would have charged one.

---

## 4. What V1 lacked, and how V2 answered

Read this table next to [`uni/UNISWAP-DEEP-DIVE.md`](./UNISWAP-DEEP-DIVE.md), which covers the V2
side in full.

| Dimension | Uniswap V1 | Uniswap V2 | Why it changed |
|---|---|---|---|
| **Pairs** | Always TOKEN/ETH. `createExchange(token)` takes one argument. | Arbitrary ERC20/ERC20. `createPair(tokenA, tokenB)`. | Token→token cost two fees and two lots of slippage (§3.3), and every LP was forced into 50% ETH exposure. Direct pairs like DAI/USDC make no sense routed through ETH. |
| **Reserves** | Not stored. Read live from `self.balance` and `token.balanceOf(self)` on every call. | Cached in `reserve0`/`reserve1`, packed with `blockTimestampLast` into one slot. | Live reads cost an external call per swap and make a price oracle impossible. Caching enables both, at the cost of needing `sync()`/`skim()` to handle donations. |
| **Price oracle** | None. Only instantaneous spot getters (§2.9), manipulable within one transaction. | `price0CumulativeLast`/`price1CumulativeLast` accumulate `price × seconds`, letting anyone compute a TWAP over any window. | Spot AMM prices are trivially flash-loan manipulable. The TWAP made Uniswap usable as an oracle by other protocols, which V1 never safely was. |
| **Flash swaps** | Impossible. Output is transferred only after input has been received. | `swap()` transfers output **first**, optionally calls `uniswapV2Call` on the recipient, then checks the k-invariant. | Enables arbitrage and liquidations with no capital, and collateral swaps in one transaction. |
| **LP token** | Baked into the exchange contract (`contracts/uniswap_exchange.vy:469-496`), no `permit`, no zero-address guard. | Separate `UniswapV2ERC20` base with EIP-2612 `permit`. | Separation of concerns, and `permit` removes the extra approval transaction when removing liquidity through the router. |
| **First deposit** | `initial_liquidity = self.balance` (§2.3); force-fed ETH inflates it. | Mints `sqrt(amount0 × amount1)` and permanently burns `MINIMUM_LIQUIDITY = 1000` to address zero. | The geometric mean is symmetric in both tokens (V1's ETH-only rule cannot be), and the burned floor stops share-price inflation attacks. |
| **Deployment** | `create_with_code_of` (delegatecall proxy) + plain `CREATE`, so addresses are unpredictable and `setup` replaces the constructor. | `CREATE2` with `keccak256(token0, token1)` as salt. | Pair addresses become a pure function of the token pair, so `UniswapV2Library.pairFor` computes them locally — no storage read, no RPC call. Essential for cheap multi-hop routing. |
| **Protocol fee** | None. All 0.3% to LPs, no switch, no governance. | `feeTo`/`feeToSetter`; when enabled, 1/6 of fee growth is minted to `feeTo` via `_mintFee` and the `kLast` bookkeeping. | Gives the protocol an optional revenue lever without changing the trader-facing 0.3%. |
| **Routing** | Built into the pool itself: 16 swap entry points plus factory lookups, all on-chain. | Pool is minimal (`mint`/`burn`/`swap`/`skim`/`sync`); routing lives in the periphery `UniswapV2Router02`. | Keeping the core tiny and immutable makes it auditable and lets routing logic be replaced without migrating liquidity. |
| **Reentrancy** | No lock anywhere. Safety relies on `send`'s 2300-gas stipend and on ordering. | Explicit `lock` modifier on `mint`/`burn`/`swap`. | Callback tokens (ERC777) and flash swaps make an explicit guard mandatory. |
| **Language** | Vyper 0.1.x. | Solidity 0.5.16. | Vyper 0.1.x was pre-1.0 and moving fast; V2's flash swaps and packed storage needed control Vyper did not then offer. Ironically Curve went the other way — see the Curve chapter. |
| **Slippage defaults** | `__default__` swaps with `min_tokens = 1` (§2.7). | No fallback; the router always requires explicit minimums and a deadline. | A convenience that was an unbounded sandwich risk. |
| **Gas** | Two external calls (`balanceOf` plus the transfer) per swap, plus proxy `DELEGATECALL` overhead. | One packed SLOAD for reserves; pair address computed, not looked up. | Straightforwardly cheaper per swap. |

### On reentrancy in V1

V1 has **no `lock` modifier**. It gets away with it for two reasons, one deliberate and one lucky.

**The deliberate one:** Vyper's `send(to, value)` forwards only the 2300-gas stipend, exactly like
Solidity's `address.transfer`. Every ETH payout in V1 uses `send` —
`removeLiquidity:93`, `ethToTokenOutput:174`, `tokenToEthInput:208`, `tokenToEthOutput:243`. A
recipient therefore cannot re-enter through the ETH leg; 2300 gas is not enough to do anything.
The cost of that choice is that **contracts with non-trivial fallbacks cannot receive ETH from a V1
exchange at all**, which is why the token→token refund path discussed in §2.7 would revert if it
ever fired.

**The lucky one:** the ERC20 legs are not protected by anything. A token with transfer hooks
(ERC777, or any token with a callback) *can* re-enter. Consider `tokenToEthInput:202`: ETH is sent
at line 208 before `transferFrom` at line 209 — but `send` blocks that path. Consider
`addLiquidity:58-60`: LP balances and `totalSupply` are written before `transferFrom` pulls tokens.
A malicious token could re-enter during that `transferFrom` and observe a `totalSupply` that has
already grown while the token reserve has not yet — meaning a nested `addLiquidity` would compute
`token_amount` against a stale, lower `token_reserve`, and a nested `removeLiquidity` would
redeem against a token balance that is still missing the incoming deposit. V1 predates widespread
ERC777 deployment, and permissionless listing means such a token could always be paired. Treat this
as a genuine hazard of the design rather than a proven exploit path.

---

## 5. Security notes

**1. The fallback is a blind market order.** `__default__` (`contracts/uniswap_exchange.vy:141`)
passes `min_tokens = 1`. Any plain ETH transfer to a V1 exchange executes a swap that will accept
literally one wei of token. A searcher watching the mempool can sandwich it for nearly the whole
value. Never send ETH to a V1 exchange without calldata.

**2. Rounding always favours the pool, deliberately.** Collect the instances:

| Location | Rounding | Effect |
|---|---|---|
| `getInputPrice:111` | floor on output | trader gets marginally less |
| `getOutputPrice:124` | `floor + 1` on input | trader pays up to 1 wei more |
| `addLiquidity:55` | `+ 1` on `token_amount` | depositor gives 1 wei extra |
| `addLiquidity:56` | floor on `liquidity_minted` | depositor gets marginally fewer LP tokens |
| `removeLiquidity:88-89` | floor on both outputs | withdrawer gets marginally less |

Every one of these leaves dust in the pool. That is correct: the alternative direction would let
repeated tiny operations drain LPs. The `+1` in `getOutputPrice` is not a true ceiling — it
overcharges by exactly 1 wei when the division is exact — but erring toward the pool is safe.

**3. `getOutputPrice` reverts instead of allowing reserve drain.** `(output_reserve -
output_amount)` on line 123 underflows if you try to buy the entire reserve, and Vyper reverts.
There is no explicit check; the type system is the check.

**4. Reentrancy via callback tokens.** No `lock` modifier exists. ETH payouts are protected by
`send`'s 2300-gas stipend, but ERC20 transfer hooks are not protected by anything. See the
discussion at the end of §4. `addLiquidity:58-60` is the clearest example of state being written
before an external call.

**5. Fee-on-transfer and rebasing tokens.** Because reserves are read live via
`token.balanceOf(self)`, a fee-on-transfer token does not corrupt accounting the way it would with
cached reserves — the next read simply sees the true balance. But it does break the *promise* of
each swap: `ethToTokenInput:132` transfers `tokens_bought` and the recipient receives less, while
`tokenToEthInput:209` pulls `tokens_sold` and the pool receives less than it priced for. The
`assert self.token.transfer(...)` pattern also assumes the token **returns a bool**; tokens that
return nothing (the classic USDT problem) will fail to decode and revert. V1 has no `SafeERC20`
equivalent, so such tokens simply cannot be traded.

**6. Donations silently become reserves.** Sending tokens or force-feeding ETH (via `SELFDESTRUCT`)
to an exchange increases its reserves with no LP tokens minted, gifting value to existing LPs and
moving the price. There is no `skim()` to recover them. In `addLiquidity`'s first-provider branch
(`:68`), force-fed ETH also inflates `initial_liquidity` above `msg.value`.

**7. The first provider sets the price unchecked.** `token_amount = max_tokens` and whatever ratio
results *is* the market (`:67-68`). A wrong ratio is immediately arbitraged, so the loss falls on
the first LP. The only protection is the 1 gwei dust floor on line 65.

**8. Spot prices must never be used as an oracle.** All four getters (§2.9) read live balances.
Within a single transaction, a flash loan can move them arbitrarily and move them back. Any
protocol that read `getTokenToEthInputPrice` for pricing was exploitable. This is *the* reason V2
introduced cumulative price accumulators.

**9. Deadline semantics are inconsistent.** `addLiquidity:49` and `removeLiquidity:84` use strict
`deadline > block.timestamp`; every swap path uses `deadline >= block.timestamp`
(`:128`, `:168`, `:203`, `:238`, `:272`, `:313`). Harmless, but the asymmetry is what lets
`__default__` pass `block.timestamp`.

**10. `tokenToExchange*` trusts an arbitrary address.** Lines 363, 378, 392, 407 let the caller name
any contract implementing three functions. Your `min_tokens_bought` / `max_tokens_sold` arguments
are your only protection — which is why line 272 forbids passing zero for them.

**11. LP tokens sent to the zero address are lost, not burned.** `transfer:473` has no
zero-address guard and does not touch `totalSupply`, so the underlying becomes permanently
unredeemable.

**12. The token→token refund path is fragile.** As traced in §2.7, `tokenToTokenOutput` relies on
the destination's quote equalling its charge within the same transaction. A non-zero refund would
be `send` into this contract's `__default__` with 2300 gas and revert the whole trade.

---

## 6. Exercises: trace these yourself

1. **Follow the `+1`.** Open `contracts/uniswap_exchange.vy:55` and `:124`. Both add 1 to a
   division result but for different reasons. Write down, for each, who loses the wei and why that
   direction is the safe one. Then check `tests/exchange/test_liquidity_pool.py:52` and explain why
   the expected token balance is `HAY_RESERVE + HAY_ADDED + 1` rather than `+ 0`.

2. **Find every `self.balance` and decide whether it needs a subtraction.** Grep for it, then for
   each hit at `contracts/uniswap_exchange.vy:53, 68, 88, 130, 170, 205, 240, 275, 317, 419, 429,
   440, 451` decide whether that call site is `@payable`. Confirm that exactly the payable ones
   subtract, and work out what breaks if `:419` subtracted or `:130` did not.

3. **Walk exact-output ETH→token.** Start at `ethToTokenSwapOutput:186` and follow into
   `ethToTokenOutput:167`. There is no `assert eth_sold <= max_eth` anywhere. Find the line that
   enforces it anyway, and explain in one sentence how. Then work out who receives the refund and
   why it is `buyer` rather than `recipient`.

4. **Trace both legs of an exact-output token→token trade.** Begin at `tokenToTokenSwapOutput:334`,
   into `tokenToTokenOutput:312`. Note the cross-contract call on line 315 and the one on line 321.
   Then open `ethToTokenOutput:167` and determine what `buyer` is during that second call. Explain
   why `eth_refund` is always zero, and what would happen if it were not.

5. **Prove that the three token→token failure cases share one check.** Open
   `tests/exchange/test_token_to_token.py:36-41`. Three different bad arguments are expected to
   revert. Find the single line in `contracts/uniswap_exchange.vy` that catches all three, and
   explain how `factory.getExchange` turns each bad input into something that line rejects.

6. **Read `tests/exchange/test_token_to_token.py` and predict the assertions before running it.**
   Given `ETH_RESERVE = 5e18`, `HAY_RESERVE = 10e18`, `DEN_RESERVE = 20e18` and `HAY_SOLD = 2e18`,
   compute by hand: the intermediate ETH, the DEN received, and the four post-trade balances.
   Then check yourself against `constants.py:20` and `constants.py:24` and against the assertions
   at lines 45-53. Now do the same for `test_swap_output` starting at line 84 and explain why the
   exact-output path needs *two* maximum guards rather than one.

7. **Compare a hypothetical direct pair.** Using the V1 formula, compute what 2 HAY would buy in a
   *single* HAY/DEN pool holding 10 HAY and 20 DEN, and compare it to the 2.843678215834080602 DEN
   the two-hop route actually delivered in §3.3. Attribute the difference to the second fee versus
   the second slippage. This number is the concrete motivation for V2's arbitrary pairs.

8. **Then read the next chapter.** Open [`uni/UNISWAP-DEEP-DIVE.md`](./UNISWAP-DEEP-DIVE.md) and
   find `UniswapV2Pair.swap`. Identify the two lines that make flash swaps possible and explain why
   the equivalent cannot exist anywhere in `uniswap_exchange.vy` — specifically, which V1 ordering
   decision forecloses it.
