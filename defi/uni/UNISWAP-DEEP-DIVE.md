# Uniswap Deep Dive

A code-first walkthrough of Uniswap V2, V3 and V4, written against the exact sources cloned in this folder:

| Folder | What it is | Language |
|---|---|---|
| `v2-core/` | Factory + Pair (the AMM itself) | Solidity 0.5.16 |
| `v2-periphery/` | Router02 + Library (the user-facing wrapper) | Solidity 0.6.6 |
| `v3-core/` | Factory + Pool + math libraries (concentrated liquidity) | Solidity 0.7.6 |
| `v3-periphery/` | NonfungiblePositionManager, SwapRouter, Quoter, OracleLibrary | Solidity 0.7.6 |
| `v4-core/` | Singleton PoolManager + Pool library + Hooks | Solidity 0.8.26 |

Every `path:line` below was checked with `grep -n` against these files. Open them side by side as you read.

---

## 0. Mental model of an AMM

**The problem.** An order book needs someone to post a bid and someone to post an ask. On-chain that is expensive and slow. An Automated Market Maker replaces the order book with a *pool* of two tokens and a *formula* that decides the price for any trade size, deterministically, with no counterparty to find.

**Constant product.** Uniswap V2 keeps reserves `x` (token0) and `y` (token1) and enforces

```
x * y = k        (k must never decrease across a swap)
```

If you put `dx` in, the pool gives out whatever `dy` keeps the product constant:

```
(x + dx) * (y - dy) = k   =>   dy = y - k/(x+dx) = y*dx / (x + dx)
```

**Why this gives a price.** The marginal price is the derivative: `-dy/dx = y/x`. So the *spot price* of token0 in units of token1 is just the reserve ratio `y/x`. Buying token0 (`dx < 0`) removes it from the pool, which raises `y/x`, which makes the next unit more expensive. That is the price impact ("slippage") and it is what keeps the pool near the market: if the pool is cheaper than Binance, an arbitrageur buys until the ratio matches.

**Worked example (V2 with 0.3% fee).**

```
reserves: x = 10 ETH, y = 20,000 USDC        spot = 2,000 USDC/ETH, k = 200,000
trader sends dx = 1 ETH.
fee: only 99.7% of input counts   -> effective dx = 0.997
dy = y * 0.997 / (x + 0.997) = 20,000 * 0.997 / 10.997 = 1,813.2 USDC
new reserves: 11 ETH, 18,186.8 USDC  -> new spot = 1,653.3 USDC/ETH
effective price paid: 1,813.2 USDC/ETH (vs 2,000 spot) = ~9.3% price impact
```

The fee is not sent anywhere. The full 1 ETH stays in the pool but only 0.997 ETH was "used" in the formula, so `k` grew from 200,000 to `11 * 18,186.8 = 200,055`. LPs own `k` growth pro-rata.

**LP tokens.** When you deposit both tokens you get an ERC-20 "share" whose supply is proportional to `sqrt(x*y)`. On withdrawal you get `share/totalSupply` of *whatever the pool holds now*, fees included.

**Impermanent loss.** The pool always rebalances against you. Deposit 1 ETH + 2,000 USDC at $2,000 (`k = 2,000`). ETH goes to $4,000 elsewhere; arbs trade the pool until its spot is $4,000, i.e. `x = sqrt(k/P) = 0.707 ETH`, `y = sqrt(k*P) = 2,828 USDC`. Position value = `0.707*4000 + 2828 = $5,657`. Just holding would be `$6,000`. The 5.7% gap is impermanent loss; fees must out-earn it.

**What V3 and V4 change.** V3 lets an LP put all its capital between two prices instead of spreading it from 0 to infinity (concentrated liquidity), which multiplies fee income per dollar but turns positions into non-fungible ranges. V4 keeps V3's math but moves every pool into a single contract, replaces token transfers with net "deltas" settled at the end, and adds hook contracts that can run custom logic on every action.

---

## 1. Uniswap V2

### 1.1 Contract map

```
                      user / EOA
                          |
                          v
   +---------------------------------------------+
   |  UniswapV2Router02   (v2-periphery)         |   stateless helper; computes amounts,
   |    uses UniswapV2Library (pure math)        |   moves tokens, calls pair.mint/burn/swap
   +---------------------------------------------+
        |  createPair (if missing)          |  mint / burn / swap
        v                                  v
   +-----------------------+     +---------------------------------+
   | UniswapV2Factory      |---->| UniswapV2Pair  (one per token   |
   |  getPair[a][b]        |CREATE2| pair) is-a UniswapV2ERC20     |
   |  feeTo / feeToSetter  |     |  reserve0, reserve1, kLast, ... |
   +-----------------------+     +---------------------------------+
                                        |  optional flash-swap callback
                                        v
                                  IUniswapV2Callee(to).uniswapV2Call(...)
```

- `UniswapV2Factory` (`v2-core/contracts/UniswapV2Factory.sol`): deploys pairs, stores `getPair`, and owns the protocol-fee switch `feeTo`.
- `UniswapV2Pair` (`v2-core/contracts/UniswapV2Pair.sol`): the AMM. Holds reserves, mints/burns LP tokens, executes swaps, records the TWAP oracle.
- `UniswapV2ERC20` (`v2-core/contracts/UniswapV2ERC20.sol`): the LP token, with EIP-2612 `permit` at `UniswapV2ERC20.sol:81`.
- `UniswapV2Router02` (`v2-periphery/contracts/UniswapV2Router02.sol`): what wallets actually call. It has *no* privileged role; the pair is the source of truth.
- `UniswapV2Library` (`v2-periphery/contracts/libraries/UniswapV2Library.sol`): pure math (`quote`, `getAmountOut`, `getAmountIn`) and CREATE2 address derivation (`pairFor`).

The core/periphery split is deliberate: the pair is minimal and "low-level; should be called from a contract which performs important safety checks" (comment at `UniswapV2Pair.sol:109`). Slippage checks, deadlines, and token pulling live in the router.

### 1.2 `Factory.createPair` and deterministic addresses

`v2-core/contracts/UniswapV2Factory.sol:23-38`

1. **Inputs**: `tokenA`, `tokenB`.
2. **Checks**: not identical (`:24`), sort so `token0 < token1` (`:25`), non-zero (`:26`), not already created (`:27`).
3. **State writes**: `getPair` in both directions (`:34-35`), `allPairs.push` (`:36`).
4. **External calls**: `create2` with `salt = keccak256(token0, token1)` (`:29-32`), then `pair.initialize(token0, token1)` (`:33`) which only the factory may call (`UniswapV2Pair.sol:66-70`).
5. **Emits** `PairCreated`.

Because the salt is only the sorted token addresses and the creation code is fixed, the pair address is a pure function of `(factory, token0, token1)`. The periphery exploits this in `UniswapV2Library.pairFor` (`UniswapV2Library.sol:18-26`):

```solidity
pair = address(uint(keccak256(abi.encodePacked(
        hex'ff', factory,
        keccak256(abi.encodePacked(token0, token1)),
        hex'96e8ac4277198ff8b6f785478aa9a39f403cb768dd02cbee326c3e7da348845f' // init code hash
))));
```

No external call is needed to find a pair, which saves gas on every hop of a multi-hop swap. (If you fork V2 and change the Pair bytecode, you must update this hash, a classic fork bug.)

### 1.3 Pair storage and the `lock` modifier

`v2-core/contracts/UniswapV2Pair.sol:15-36`

```solidity
uint public constant MINIMUM_LIQUIDITY = 10**3;
address public factory; address public token0; address public token1;
uint112 private reserve0;           // uses single storage slot
uint112 private reserve1;           // uses single storage slot
uint32  private blockTimestampLast; // uses single storage slot
uint public price0CumulativeLast;
uint public price1CumulativeLast;
uint public kLast; // reserve0 * reserve1, as of immediately after the most recent liquidity event
uint private unlocked = 1;
modifier lock() { require(unlocked == 1, 'UniswapV2: LOCKED'); unlocked = 0; _; unlocked = 1; }
```

- `reserve0/reserve1/blockTimestampLast` are `112+112+32 = 256` bits: one SLOAD via `getReserves()` (`:38-42`). Reserves are the pool's *accounting* view of its balances; actual `balanceOf` can drift above them (donations, fee-on-transfer quirks) and `skim`/`sync` reconcile.
- `kLast` is only used for the protocol fee (1.5).
- `lock` is a plain mutex. It matters because `swap` makes an untrusted external call (flash-swap callback) and `mint`/`burn` read `balanceOf` mid-function; reentering would let an attacker mint against inflated balances.

### 1.4 `mint(address to)`: adding liquidity

`v2-core/contracts/UniswapV2Pair.sol:110-131`

The pair never pulls tokens. The caller (router) transfers both tokens **to the pair first**, then calls `mint`. The pair infers how much arrived by diffing balance vs reserve:

```solidity
(uint112 _reserve0, uint112 _reserve1,) = getReserves();
uint balance0 = IERC20(token0).balanceOf(address(this));
uint balance1 = IERC20(token1).balanceOf(address(this));
uint amount0 = balance0.sub(_reserve0);
uint amount1 = balance1.sub(_reserve1);
```

1. **Inputs**: `to` (LP-token recipient). Amounts are implicit.
2. **Checks**: `liquidity > 0` (`:125`).
3. **State writes**: `_mintFee` may mint to `feeTo` (`:117`); `_mint(to, liquidity)` (`:126`); `_update` writes reserves + oracle (`:128`); `kLast` if fee on (`:129`).
4. **External calls**: two `balanceOf` reads, `factory.feeTo()` inside `_mintFee`.
5. **Returns** `liquidity`, emits `Mint`.

**First deposit** (`:119-121`):

```solidity
liquidity = Math.sqrt(amount0.mul(amount1)).sub(MINIMUM_LIQUIDITY);
_mint(address(0), MINIMUM_LIQUIDITY); // permanently lock the first MINIMUM_LIQUIDITY tokens
```

`sqrt(x*y)` makes the LP token value independent of the initial ratio (depositing 1 ETH + 4,000 USDC gives the same share count as 2 ETH + 2,000 USDC: both are `sqrt(4000)`). Burning 1,000 wei of shares to `address(0)` prevents the "inflation attack": without it, the first LP could mint 1 wei of share, donate a huge balance, and make each share so expensive that later depositors round down to 0 shares.

**Subsequent deposits** (`:123`):

```solidity
liquidity = Math.min(amount0.mul(_totalSupply) / _reserve0, amount1.mul(_totalSupply) / _reserve1);
```

Taking the `min` means an unbalanced deposit is penalised: the excess side is donated to existing LPs. That is why the router computes the optimal ratio first (1.9).

### 1.5 `_mintFee`: the protocol fee switch

`v2-core/contracts/UniswapV2Pair.sol:89-107`

Swap fees accrue into `k`. Instead of skimming on every swap (expensive), the pair measures how much `sqrt(k)` grew since the last liquidity event and mints LP tokens to `feeTo` worth 1/6 of that growth:

```solidity
uint rootK = Math.sqrt(uint(_reserve0).mul(_reserve1));
uint rootKLast = Math.sqrt(_kLast);
if (rootK > rootKLast) {
    uint numerator = totalSupply.mul(rootK.sub(rootKLast));
    uint denominator = rootK.mul(5).add(rootKLast);
    uint liquidity = numerator / denominator;
    if (liquidity > 0) _mint(feeTo, liquidity);
}
```

Derivation: we want to mint `s_m` shares so that the protocol owns fraction `φ = 1/6` of the growth. With `S` = totalSupply, the condition `s_m / (S + s_m) = φ * (√k − √k_last) / √k` solves to `s_m = S * (√k − √k_last) / ((1/φ − 1)√k + √k_last) = S(√k − √k_last) / (5√k + √k_last)`. That is exactly the code. If `feeTo` is unset the pair just zeroes `kLast` (`:104-106`). On mainnet the switch has been off for V2; the mechanism exists so governance can turn it on without a migration.

### 1.6 `burn(address to)`: removing liquidity

`v2-core/contracts/UniswapV2Pair.sol:134-156`

Again the caller transfers LP tokens **to the pair** first; `burn` reads `balanceOf[address(this)]` (`:140`).

```solidity
amount0 = liquidity.mul(balance0) / _totalSupply; // using balances ensures pro-rata distribution
amount1 = liquidity.mul(balance1) / _totalSupply;
```

It divides by *balances*, not reserves, so any tokens sitting above reserves (donations, rounding dust) are shared out too. Then: `_burn`, two `_safeTransfer`s, re-read balances, `_update`, `kLast`. `_mintFee` runs first (`:142`) so the protocol gets its cut before the share price is computed.

### 1.7 `swap(...)`: optimistic transfer, flash swaps, the k-check

`v2-core/contracts/UniswapV2Pair.sol:159-187`

This is the most important function in V2. Read the order of operations carefully:

```solidity
function swap(uint amount0Out, uint amount1Out, address to, bytes calldata data) external lock {
    require(amount0Out > 0 || amount1Out > 0, ...);                       // :160
    (uint112 _reserve0, uint112 _reserve1,) = getReserves();
    require(amount0Out < _reserve0 && amount1Out < _reserve1, ...);      // :162
    ...
    if (amount0Out > 0) _safeTransfer(_token0, to, amount0Out); // optimistically transfer tokens  :170
    if (amount1Out > 0) _safeTransfer(_token1, to, amount1Out);
    if (data.length > 0) IUniswapV2Callee(to).uniswapV2Call(msg.sender, amount0Out, amount1Out, data); // :172
    balance0 = IERC20(_token0).balanceOf(address(this));
    balance1 = IERC20(_token1).balanceOf(address(this));
    ...
    uint amount0In = balance0 > _reserve0 - amount0Out ? balance0 - (_reserve0 - amount0Out) : 0;  // :176
    uint amount1In = balance1 > _reserve1 - amount1Out ? balance1 - (_reserve1 - amount1Out) : 0;
    require(amount0In > 0 || amount1In > 0, ...);
    uint balance0Adjusted = balance0.mul(1000).sub(amount0In.mul(3));                                 // :180
    uint balance1Adjusted = balance1.mul(1000).sub(amount1In.mul(3));
    require(balance0Adjusted.mul(balance1Adjusted) >= uint(_reserve0).mul(_reserve1).mul(1000**2), 'UniswapV2: K'); // :182
    _update(balance0, balance1, _reserve0, _reserve1);
    emit Swap(...);
}
```

1. **Inputs**: how much of each token you want *out*, the recipient, and optional callback `data`. The function does not take an input amount at all.
2. **Checks**: some output requested; output below reserves; `to` is not a token; after everything, the k-invariant.
3. **Order**: it *sends the output first* (`:170-171`), then optionally calls `to` (`:172`), then measures what came in by reading balances (`:173-177`). The input is whatever the balance rose by relative to `reserve − amountOut`.
4. **The fee lives in the k-check**, not in a transfer. `balanceAdjusted = balance*1000 − amountIn*3` subtracts 0.3% of the input before checking `x'·y' ≥ x·y`. Scaling by 1000 avoids fractions. Any extra tokens you send beyond what the formula needs simply stay in the pool as LP profit.
5. **Flash swap**: if `data.length > 0`, the recipient gets the output tokens *before paying*. Inside `uniswapV2Call` it can do anything (arbitrage, liquidate elsewhere) as long as, by the time the callback returns, the pool's balances satisfy the k-check. You may repay in either token. `lock` prevents the callback from reentering `swap`/`mint`/`burn`.

Because `swap` does not know or care who sent the input, the router's pattern is: `transferFrom(user -> pair)`, then `pair.swap(out, ..., to)`.

### 1.8 `_update`: reserves and the TWAP oracle

`v2-core/contracts/UniswapV2Pair.sol:73-86`

```solidity
uint32 blockTimestamp = uint32(block.timestamp % 2**32);
uint32 timeElapsed = blockTimestamp - blockTimestampLast; // overflow is desired
if (timeElapsed > 0 && _reserve0 != 0 && _reserve1 != 0) {
    price0CumulativeLast += uint(UQ112x112.encode(_reserve1).uqdiv(_reserve0)) * timeElapsed;
    price1CumulativeLast += uint(UQ112x112.encode(_reserve0).uqdiv(_reserve1)) * timeElapsed;
}
reserve0 = uint112(balance0); reserve1 = uint112(balance1); blockTimestampLast = blockTimestamp;
```

- Prices are `UQ112x112` fixed-point (`libraries/UQ112x112.sol:8-19`): a 112-bit integer part and 112-bit fraction, so `encode(a).uqdiv(b) = (a << 112) / b`.
- The accumulator adds `price_at_start_of_block * secondsElapsed` on the **first** call in each block, using the *previous* reserves. That means the price recorded for a block is the one set by the last trade of the *previous* block, which an attacker cannot change inside the block they control. Manipulating a TWAP therefore requires holding a bad price across block boundaries, i.e. leaving money on the table for arbs.
- `uint32` timestamps and `uint256` accumulators are allowed to overflow. Consumers only ever compute *differences*, and `uint32` subtraction wraps correctly. `v2-periphery/contracts/libraries/UniswapV2OracleLibrary.sol:16-34` shows how a consumer reads it: `TWAP = (cum_now − cum_then) / (t_now − t_then)`, with the "counterfactual" trick of extending the accumulator to now if no trade happened this block.
- The `uint112` cap is why `_update` reverts with `OVERFLOW` if a balance exceeds `2^112 − 1` (`:74`).

### 1.9 `skim` and `sync`

`UniswapV2Pair.sol:190-200`. `skim(to)` sends `balance − reserve` of each token to `to`: a way to recover tokens someone accidentally transferred, and, importantly, a way to bring balances back under the `uint112` cap so the pair does not brick. `sync()` does the opposite, forcing reserves up to balances; useful after a rebasing token changes the pair's balance.

### 1.10 Router02: what users actually call

`v2-periphery/contracts/UniswapV2Router02.sol`. Every function has `ensure(deadline)` (`:18-21`) so a transaction stuck in the mempool cannot execute at a stale price. `receive()` (`:28-30`) only accepts ETH from WETH (the `withdraw` path).

**`_addLiquidity`** (`:33-60`) picks amounts that match the pool's current ratio so no value is lost to `min()` in `mint`:

```solidity
if (IUniswapV2Factory(factory).getPair(tokenA, tokenB) == address(0)) IUniswapV2Factory(factory).createPair(tokenA, tokenB);
(uint reserveA, uint reserveB) = UniswapV2Library.getReserves(factory, tokenA, tokenB);
if (reserveA == 0 && reserveB == 0) { (amountA, amountB) = (amountADesired, amountBDesired); }
else {
    uint amountBOptimal = UniswapV2Library.quote(amountADesired, reserveA, reserveB);   // amountA * reserveB / reserveA
    if (amountBOptimal <= amountBDesired) { require(amountBOptimal >= amountBMin); (amountA, amountB) = (amountADesired, amountBOptimal); }
    else { uint amountAOptimal = quote(amountBDesired, reserveB, reserveA); require(amountAOptimal >= amountAMin); (amountA, amountB) = (amountAOptimal, amountBDesired); }
}
```

`addLiquidity` (`:61-76`) then `safeTransferFrom`s both tokens *to the pair* and calls `pair.mint(to)`. `addLiquidityETH` (`:77-100`) wraps `msg.value` into WETH, transfers it, and refunds dust.

**`removeLiquidity`** (`:103-119`): `pair.transferFrom(msg.sender, pair, liquidity)` then `pair.burn(to)`, then re-orders `(amount0, amount1)` into `(amountA, amountB)` and checks the mins. `removeLiquidityETH` (`:120-140`) burns to the router, unwraps WETH, forwards ETH. The `...WithPermit` variants (`:141`, `:156`) call `pair.permit` first so removal is one transaction.

**Amount math** (`UniswapV2Library.sol:43-59`):

```solidity
// getAmountOut: dy = (dx*997 * y) / (x*1000 + dx*997)
uint amountInWithFee = amountIn.mul(997);
amountOut = amountInWithFee.mul(reserveOut) / (reserveIn.mul(1000).add(amountInWithFee));
// getAmountIn:  dx = (x * dy * 1000) / ((y − dy) * 997) + 1     (+1 rounds against the trader)
amountIn = (reserveIn.mul(amountOut).mul(1000) / reserveOut.sub(amountOut).mul(997)).add(1);
```

`getAmountsOut` (`:62-70`) chains them forward along a `path`; `getAmountsIn` (`:73-81`) chains backward.

**`_swap`** (`:212-223`) is the multi-hop engine:

```solidity
for (uint i; i < path.length - 1; i++) {
    (address input, address output) = (path[i], path[i + 1]);
    (address token0,) = UniswapV2Library.sortTokens(input, output);
    uint amountOut = amounts[i + 1];
    (uint amount0Out, uint amount1Out) = input == token0 ? (uint(0), amountOut) : (amountOut, uint(0));
    address to = i < path.length - 2 ? UniswapV2Library.pairFor(factory, output, path[i + 2]) : _to;
    IUniswapV2Pair(UniswapV2Library.pairFor(factory, input, output)).swap(amount0Out, amount1Out, to, new bytes(0));
}
```

The trick: each hop's output is sent **directly to the next pair** (`to = pairFor(output, path[i+2])`). The router never holds tokens mid-route, and since `swap` measures its input from balance changes, the next pair sees the tokens as already delivered.

- `swapExactTokensForTokens` (`:224-237`): compute `amounts` forward, check the last `>= amountOutMin`, pull `amounts[0]` from the user into the first pair, `_swap`.
- `swapTokensForExactTokens` (`:238-251`): compute `amounts` backward, check `amounts[0] <= amountInMax`, same execution.
- ETH variants (`:252-317`) wrap on the way in (`IWETH.deposit`, transfer to first pair) or unwrap on the way out (`_swap` to the router, `IWETH.withdraw`, `safeTransferETH`).
- **Fee-on-transfer variants** (`:321-400`): tokens that burn a % on transfer break the precomputed `amounts` (less arrives at the pair than `getAmountOut` assumed, so the k-check fails). `_swapSupportingFeeOnTransferTokens` instead reads the pair's actual balance minus reserve at each hop (`:331`) and computes the output from that; the min-out check is done on the recipient's real balance delta (`:349-354`).

### 1.11 End-to-end trace: `swapExactTokensForTokens(100 USDC -> WETH)`

Assume `path = [USDC, WETH]`, USDC address < WETH address so USDC is `token0`.

```
EOA
 |  1. USDC.approve(router, 100e6)                          (separate tx, once)
 |  2. router.swapExactTokensForTokens(100e6, minOut, [USDC, WETH], EOA, deadline)
 v
Router02.swapExactTokensForTokens                              Router02.sol:224
 |-- ensure(deadline)                                          :18
 |-- amounts = Library.getAmountsOut(factory, 100e6, path)     Library.sol:62
 |     |-- getReserves(factory, USDC, WETH)                    :29   -> pairFor (pure CREATE2 math, :18) -> pair.getReserves() [1 SLOAD]
 |     '-- amounts[1] = getAmountOut(100e6, rUSDC, rWETH)      :43   (997/1000 fee)
 |-- require(amounts[1] >= minOut)                             :232
 |-- USDC.transferFrom(EOA, pair, 100e6)                       :233   <- tokens land in the PAIR, not the router
 '-- _swap(amounts, path, EOA)                                 :212
       '-- pair.swap(0, amounts[1], EOA, "")                    Pair.sol:159   (USDC is token0, so amount0Out = 0)
             |-- lock: unlocked 1 -> 0                          :31
             |-- getReserves()                                  :161
             |-- WETH.transfer(EOA, amounts[1])   (optimistic)  :171
             |-- data.length == 0 -> no callback                :172
             |-- balance0 = USDC.balanceOf(pair)  (= r0 + 100e6):173
             |-- balance1 = WETH.balanceOf(pair)  (= r1 - out)  :174
             |-- amount0In = balance0 - (r0 - 0) = 100e6        :176
             |-- k-check with 0.3% of amount0In removed         :180-182
             |-- _update(balance0, balance1, r0, r1)            :73
             |     |-- if first call this block: price cumulatives += oldPrice * dt
             |     '-- SSTORE reserve0/reserve1/blockTimestampLast (one slot), emit Sync
             |-- emit Swap(router, 100e6, 0, 0, out, EOA)
             '-- lock: unlocked 0 -> 1
```

Storage written: the pair's packed reserves slot, `price0CumulativeLast`/`price1CumulativeLast` (first trade of the block only), the two ERC-20 balance mappings in USDC and WETH, and the `unlocked` flag (twice). The router writes nothing.

---

## 2. Uniswap V3 (concentrated liquidity)

### 2.1 Why ticks and sqrt prices

In V2 an LP's capital is spread over prices from 0 to infinity; at any moment most of it is idle. V3 lets an LP say "my capital is active only while the price is between `Pa` and `Pb`". Inside that range the position behaves like a V2 pool with *virtual* reserves that are much larger than the real ones. Outside it, the position is 100% one token and earns nothing.

**Price space is discretised into ticks:**

```
price(i) = 1.0001^i           (each tick is 1 basis point apart)
tick(P)  = floor( log_1.0001(P) )
```

So `P = 2000` is tick `76012` (`ln 2000 / ln 1.0001 = 76012.8`), and `P = 1/2000` is tick `-76012`. `TickMath.MIN_TICK = -887272`, `MAX_TICK = 887272` (`v3-core/contracts/libraries/TickMath.sol:9-11`), giving prices from `2^-128` to `2^128`. Pools only allow positions on ticks that are multiples of `tickSpacing` (10, 60 or 200 depending on the fee tier, `UniswapV3Factory.sol:26-30`).

**The state variables are `sqrtPriceX96` and `liquidity L`, not reserves.** Define `√P = sqrt(token1/token0)` stored as Q64.96 fixed point (`sqrtPriceX96 = √P * 2^96`, 160 bits). For an amount of liquidity `L` active between `√Pa` and `√Pb`:

```
Δx (token0) = L * (1/√Pa − 1/√Pb) = L * (√Pb − √Pa) / (√Pa * √Pb)
Δy (token1) = L * (√Pb − √Pa)
```

These come from V2's `x*y = L²` and `P = y/x`, which give `x = L/√P`, `y = L√P`. Two reasons to store `√P`: (1) both deltas above are *linear* in `√P`, so a swap step is one multiplication/division instead of a square root, and (2) `L` stays constant while price moves inside a tick range, so the swap loop only has to update one number per tick crossed.

**Worked example.** Token0 = ETH, token1 = USDC, `P = 2000` so `√P = 44.72`. LP picks range `[1500, 2500]`, `√Pa = 38.73`, `√Pb = 50.0`, and wants to deposit 1 ETH.

```
L  = Δx * √P * √Pb / (√Pb − √P) = 1 * 44.72 * 50 / (50 − 44.72) = 423.5      (LiquidityAmounts.getLiquidityForAmount0)
Δy = L * (√P − √Pa)            = 423.5 * (44.72 − 38.73)       = 2,537 USDC (getAmount1Delta over [√Pa, √P])
```

So the position needs 1 ETH + 2,537 USDC. Compare with V2 full range: 1 ETH + 2,000 USDC gives `L = sqrt(2000) = 44.7`. The V3 position has ~9.5x the liquidity, hence ~9.5x the fee share for trades inside `[1500, 2500]`.

A trader now sells 0.1 ETH (zeroForOne, price falls), ignoring the fee:

```
√P' = L*√P / (L + Δx*√P) = 423.5*44.72 / (423.5 + 0.1*44.72) = 44.253       (getNextSqrtPriceFromAmount0RoundingUp)
Δy  = L * (√P − √P')     = 423.5 * (44.72 − 44.253) = 197.8 USDC out
new price = 44.253² = 1958.3
```

### 2.2 Contract map

```
                              user
                               |
        +----------------------+-----------------------+
        v                      v                       v
 NonfungiblePositionManager  SwapRouter            Quoter / QuoterV2      (v3-periphery)
   mint/increase/decrease/    exactInput{,Single}    revert-to-return-data
   collect/burn (ERC-721)     exactOutput{,Single}   simulation
        |  pool.mint/burn/collect   |  pool.swap             |  pool.swap (then revert)
        |  <- uniswapV3MintCallback |  <- uniswapV3SwapCallback
        v                           v                        v
 +-------------------------------------------------------------------+
 |  UniswapV3Pool  (one per token0/token1/fee)          (v3-core)     |
 |   slot0, liquidity, feeGrowthGlobal, ticks, tickBitmap, positions, |
 |   observations                                                     |
 |   libraries: Tick, TickBitmap, Position, Oracle, SwapMath,         |
 |              SqrtPriceMath, TickMath, FullMath, LiquidityMath      |
 +-------------------------------------------------------------------+
        ^ CREATE2 via UniswapV3PoolDeployer.deploy
 UniswapV3Factory.createPool(tokenA, tokenB, fee)
```

Periphery base contracts worth knowing: `LiquidityManagement` (holds the mint callback), `PeripheryPayments` (`pay()` chooses between WETH wrap, self-transfer, or `transferFrom`), `PoolAddress` (CREATE2 derivation), `CallbackValidation` (asserts `msg.sender` is the real pool), `Path` (packed `token|fee|token|fee|token` route encoding), `Multicall`, `SelfPermit`.

**Factory and deployer.** `UniswapV3Factory.createPool` (`v3-core/contracts/UniswapV3Factory.sol:35-51`) sorts tokens, looks up `tickSpacing` for the fee (`:43`), and calls `deploy` (`UniswapV3PoolDeployer.sol:27-37`):

```solidity
parameters = Parameters({factory: factory, token0: token0, token1: token1, fee: fee, tickSpacing: tickSpacing});
pool = address(new UniswapV3Pool{salt: keccak256(abi.encode(token0, token1, fee))}());
delete parameters;
```

The pool's constructor reads its config *back from the deployer* (`UniswapV3Pool.sol:117-123`: `IUniswapV3PoolDeployer(msg.sender).parameters()`) instead of taking constructor arguments. Constructor args are part of the init code, and the init code must be constant for the CREATE2 address to be predictable by `PoolAddress.computeAddress` (`v3-periphery/contracts/libraries/PoolAddress.sol:33-47`) using the fixed `POOL_INIT_CODE_HASH` (`:6`).

### 2.3 Pool storage

`v3-core/contracts/UniswapV3Pool.sol:42-99`

| Field | Type | Meaning |
|---|---|---|
| `factory, token0, token1, fee, tickSpacing, maxLiquidityPerTick` | immutables | fee is in hundredths of a bip (3000 = 0.30%) |
| `slot0.sqrtPriceX96` | uint160 | current `√P * 2^96` |
| `slot0.tick` | int24 | current tick (= `floor(log_1.0001 P)`, may lag `sqrtPriceX96` by one after a leftward cross) |
| `slot0.observationIndex / Cardinality / CardinalityNext` | uint16 x3 | oracle ring-buffer bookkeeping |
| `slot0.feeProtocol` | uint8 | two 4-bit fields: protocol share of fees is `1/feeProtocol` per direction |
| `slot0.unlocked` | bool | reentrancy lock (`lock` modifier, `:104-109`) |
| `feeGrowthGlobal0X128 / 1X128` | uint256 | all-time fees per unit of liquidity, Q128 fixed point |
| `protocolFees` | 2 x uint128 | fees owed to the protocol, not yet collected |
| `liquidity` | uint128 | liquidity active at the *current* price |
| `ticks[int24]` | `Tick.Info` | per initialized tick: `liquidityGross`, `liquidityNet`, `feeGrowthOutside0/1`, oracle "outside" snapshots, `initialized` (`Tick.sol:17-37`) |
| `tickBitmap[int16]` | uint256 | 1 bit per (tick / tickSpacing): is it initialized |
| `positions[bytes32]` | `Position.Info` | keyed by `keccak256(owner, tickLower, tickUpper)`: `liquidity`, `feeGrowthInside0/1Last`, `tokensOwed0/1` (`Position.sol:13-22`) |
| `observations[65535]` | `Oracle.Observation` | TWAP ring buffer |

Everything in `Slot0` fits in one 256-bit word so a swap starts with a single SLOAD (`:605`).

### 2.4 `initialize`, `mint`, `_modifyPosition`, `_updatePosition`

**`initialize(sqrtPriceX96)`** (`UniswapV3Pool.sol:271-289`): can be called once (`require(slot0.sqrtPriceX96 == 0)`), derives the tick, writes observation `[0]`, sets `unlocked = true`. Not guarded by `lock` because the lock is not yet set. Anyone can call it; the caller picks the initial price, so the first LP should check it.

**`mint(recipient, tickLower, tickUpper, amount, data)`** (`:457-487`)

1. **Inputs**: owner of the position, tick range, `amount` of *liquidity* `L` (not tokens), callback data.
2. **Checks**: `amount > 0`; tick sanity inside `_modifyPosition`; after the callback, balances rose by at least `amount0`/`amount1` (`:483-484`, errors `M0`/`M1`).
3. **State writes**: everything in `_modifyPosition`.
4. **External calls**: `balance0()/balance1()` (staticcalls), then `IUniswapV3MintCallback(msg.sender).uniswapV3MintCallback(amount0, amount1, data)` (`:482`).
5. **Returns** `(amount0, amount1)` owed, emits `Mint`.

The pool computes how many tokens the liquidity is worth, then asks `msg.sender` to *pay* via a callback, then verifies by balance diff. There is no `approve` to the pool; the router's callback (`v3-periphery/contracts/base/LiquidityManagement.sol:25-35`) does `transferFrom(payer, pool)` after checking `msg.sender` is the genuine pool via `CallbackValidation.verifyCallback` (`libraries/CallbackValidation.sol:28-35`). Any contract can integrate the pool directly by implementing the callback.

**`_modifyPosition(params)`** (`:306-372`) is shared by `mint` and `burn`:

```solidity
position = _updatePosition(params.owner, params.tickLower, params.tickUpper, params.liquidityDelta, _slot0.tick);
if (params.liquidityDelta != 0) {
    if (_slot0.tick < params.tickLower) {
        amount0 = SqrtPriceMath.getAmount0Delta(sqrtRatio(tickLower), sqrtRatio(tickUpper), liquidityDelta);   // all token0
    } else if (_slot0.tick < params.tickUpper) {
        (slot0.observationIndex, slot0.observationCardinality) = observations.write(...);                        // oracle checkpoint
        amount0 = SqrtPriceMath.getAmount0Delta(_slot0.sqrtPriceX96, sqrtRatio(tickUpper), liquidityDelta);      // token0 above price
        amount1 = SqrtPriceMath.getAmount1Delta(sqrtRatio(tickLower), _slot0.sqrtPriceX96, liquidityDelta);      // token1 below price
        liquidity = LiquidityMath.addDelta(liquidityBefore, params.liquidityDelta);                              // only in-range positions change active L
    } else {
        amount1 = SqrtPriceMath.getAmount1Delta(sqrtRatio(tickLower), sqrtRatio(tickUpper), liquidityDelta);   // all token1
    }
}
```

The three branches are the geometry from 2.1. Below the range you only owe token0 (the pool would need it as price rises through your range); above it only token1; inside it, both, split at the current price. Only the "inside" case touches the global `liquidity` and writes an oracle observation, because only in-range liquidity affects swaps. The signed helpers at `SqrtPriceMath.sol:201-226` round *up* when adding liquidity and *down* when removing, always in the pool's favour.

**`_updatePosition(...)`** (`:379-453`) does the bookkeeping:

1. Fetch the position slot (`Position.get`, keccak of owner+ticks, `Position.sol:30-37`).
2. If `liquidityDelta != 0`: get the current oracle accumulators (`observeSingle(..., 0, ...)`, `:396-404`), then `ticks.update` for the lower tick with `upper = false` and the upper tick with `upper = true` (`:406-429`). `Tick.update` (`Tick.sol:110-150`) adds `liquidityDelta` to `liquidityGross`, adds it to `liquidityNet` for the lower tick and *subtracts* it for the upper tick (`:147-149`), checks `maxLiquidityPerTick`, and, if the tick was just initialised and is at or below the current tick, snapshots the global fee growth as `feeGrowthOutside` (`:132-142`). It returns `flipped` (initialised <-> uninitialised).
3. If flipped, toggle the bit in `tickBitmap.flipTick` (`TickBitmap.sol:23-32`).
4. Compute `feeGrowthInside` for the range (`Tick.getFeeGrowthInside`, next section) and call `position.update` (`Position.sol:44-87`), which credits fees to `tokensOwed`.
5. If liquidity was removed and a tick flipped to zero, `ticks.clear` it to get the gas refund (`:445-452`).

### 2.5 Fee accounting: growth per unit of liquidity

Paying each LP on every swap is impossible. Instead, every swap adds `feeAmount * 2^128 / liquidity` to `feeGrowthGlobal{0,1}X128` (`UniswapV3Pool.sol:689-690`). "Growth" is *fees per unit of liquidity*, in Q128 to keep precision. An LP with `L` units who has been in range while the growth went from `g_then` to `g_now` is owed `L * (g_now − g_then) / 2^128`. That is exactly `Position.update` (`Position.sol:61-76`):

```solidity
uint128 tokensOwed0 = uint128(FullMath.mulDiv(feeGrowthInside0X128 - _self.feeGrowthInside0LastX128, _self.liquidity, FixedPoint128.Q128));
```

But a position only earns while the price is *inside* its range. So each position needs `feeGrowthInside(tickLower, tickUpper)`, and every tick stores `feeGrowthOutside`: the growth that happened on the *other* side of the tick relative to the current price. `Tick.getFeeGrowthInside` (`Tick.sol:60-95`):

```solidity
if (tickCurrent >= tickLower) feeGrowthBelow = lower.feeGrowthOutside;  else feeGrowthBelow = feeGrowthGlobal - lower.feeGrowthOutside;
if (tickCurrent <  tickUpper) feeGrowthAbove = upper.feeGrowthOutside;  else feeGrowthAbove = feeGrowthGlobal - upper.feeGrowthOutside;
feeGrowthInside = feeGrowthGlobal - feeGrowthBelow - feeGrowthAbove;
```

The subtraction trick: `feeGrowthOutside` is never "true" fees below/above the tick in absolute terms. It is initialised to `feeGrowthGlobal` if the tick is at or below the current price, or 0 otherwise (`Tick.sol:132-140`), and every time the price crosses the tick it is flipped with `feeGrowthOutside = feeGrowthGlobal − feeGrowthOutside` (`Tick.cross`, `Tick.sol:178-179`). Because positions only ever look at *differences* of `feeGrowthInside` between two moments, the arbitrary constant cancels out, and `uint256` underflow during these subtractions is intentional and harmless. This is why V3 fee math "just works" with two SSTOREs per crossed tick regardless of how many positions share it.

### 2.6 `swap`: the loop

`v3-core/contracts/UniswapV3Pool.sol:596-788`

1. **Inputs**: `recipient`, `zeroForOne` (true = sell token0, price goes *down*), `amountSpecified` (positive = exact input, negative = exact output), `sqrtPriceLimitX96`, callback `data`.
2. **Checks**: `amountSpecified != 0`; `unlocked`; the limit is on the correct side of the current price and inside `[MIN_SQRT_RATIO, MAX_SQRT_RATIO]` (`:608-613`, error `SPL`). Then it manually locks (`:615`) because `swap` returns from the middle and the modifier would not fit "stack too deep".
3. **Per-step state** (`SwapState`, `:561-576`): `amountSpecifiedRemaining`, `amountCalculated`, `sqrtPriceX96`, `tick`, `feeGrowthGlobalX128` (input side only), `protocolFee`, `liquidity`. `StepComputations` (`:578-593`) holds one iteration's numbers.

The loop (`:641-730`):

```solidity
while (state.amountSpecifiedRemaining != 0 && state.sqrtPriceX96 != sqrtPriceLimitX96) {
    step.sqrtPriceStartX96 = state.sqrtPriceX96;
    (step.tickNext, step.initialized) = tickBitmap.nextInitializedTickWithinOneWord(state.tick, tickSpacing, zeroForOne);   // :646
    ... clamp to MIN_TICK/MAX_TICK ...
    step.sqrtPriceNextX96 = TickMath.getSqrtRatioAtTick(step.tickNext);
    (state.sqrtPriceX96, step.amountIn, step.amountOut, step.feeAmount) = SwapMath.computeSwapStep(               // :663
        state.sqrtPriceX96,
        (zeroForOne ? step.sqrtPriceNextX96 < sqrtPriceLimitX96 : step.sqrtPriceNextX96 > sqrtPriceLimitX96) ? sqrtPriceLimitX96 : step.sqrtPriceNextX96,
        state.liquidity, state.amountSpecifiedRemaining, fee);
    if (exactInput) { state.amountSpecifiedRemaining -= (step.amountIn + step.feeAmount); state.amountCalculated -= step.amountOut; }
    else            { state.amountSpecifiedRemaining += step.amountOut;                 state.amountCalculated += (step.amountIn + step.feeAmount); }
    if (cache.feeProtocol > 0) { uint256 delta = step.feeAmount / cache.feeProtocol; step.feeAmount -= delta; state.protocolFee += delta; }  // :682
    if (state.liquidity > 0) state.feeGrowthGlobalX128 += FullMath.mulDiv(step.feeAmount, FixedPoint128.Q128, state.liquidity);        // :690
    if (state.sqrtPriceX96 == step.sqrtPriceNextX96) {                                                                               // :693
        if (step.initialized) {
            int128 liquidityNet = ticks.cross(step.tickNext, ...);                                                                  // :710
            if (zeroForOne) liquidityNet = -liquidityNet;
            state.liquidity = LiquidityMath.addDelta(state.liquidity, liquidityNet);
        }
        state.tick = zeroForOne ? step.tickNext - 1 : step.tickNext;                                                                 // :725
    } else if (state.sqrtPriceX96 != step.sqrtPriceStartX96) {
        state.tick = TickMath.getTickAtSqrtRatio(state.sqrtPriceX96);
    }
}
```

**Finding the next tick.** `TickBitmap.nextInitializedTickWithinOneWord` (`TickBitmap.sol:42-77`) compresses the tick by `tickSpacing`, splits it into a 16-bit word index and 8-bit bit index (`position`, `:14-17`), masks the word to the bits at/below (`lte`) or above, and uses `mostSignificantBit`/`leastSignificantBit` to find the nearest set bit. If no bit is set in the word, it returns the word's edge with `initialized = false`, and the loop simply takes another iteration in the next word. Each iteration touches at most one 256-bit word.

**Computing one step.** `SwapMath.computeSwapStep` (`SwapMath.sol:21-97`) answers: "with liquidity `L`, starting at `√P`, not going past `√P_target`, how far can `amountRemaining` push the price?"

- Exact input (`:40-52`): take the fee off the remaining amount (`amountRemainingLessFee = amount * (1e6 − fee) / 1e6`), compute the input needed to reach the target (`getAmount0Delta`/`getAmount1Delta`, rounded up). If we have enough, the price goes exactly to the target; otherwise `getNextSqrtPriceFromInput` computes where we stop.
- Exact output (`:53-65`): symmetric using `getNextSqrtPriceFromOutput`.
- Then recompute `amountIn`/`amountOut` for the actual price move (`:70-84`), cap output to what was asked (`:87-89`), and set the fee: if we did *not* reach the target in an exact-input swap, the fee is simply "whatever is left over" (`:91-93`), otherwise `amountIn * fee / (1e6 − fee)` rounded up (`:95`).

**Crossing a tick.** When the step ends exactly at `sqrtPriceNextX96` and that tick is initialised, `Tick.cross` (`Tick.sol:168-184`) flips the "outside" accumulators and returns `liquidityNet`. Moving right (price up) through a lower tick adds the position's liquidity; through an upper tick removes it. Moving left the signs invert (`:720`). `state.tick` is set to `tickNext − 1` when moving left (`:725`) because the price is now *at* the boundary, and by V3's convention a price exactly at tick `i` while moving down belongs to tick `i − 1`. The first time any tick is crossed the current oracle accumulators are computed once and cached (`:698-708`).

**After the loop** (`:733-769`): write `slot0` (price, tick, and a new oracle observation if the tick changed, `:733-752`); write `liquidity` if changed (`:755`); write `feeGrowthGlobal` for the input token and bump `protocolFees` (`:759-765`); compute the signed `(amount0, amount1)` for the caller (`:767-769`), where positive means "pool is owed", negative means "pool pays".

**Settlement** (`:772-784`):

```solidity
if (zeroForOne) {
    if (amount1 < 0) TransferHelper.safeTransfer(token1, recipient, uint256(-amount1));   // pay output FIRST
    uint256 balance0Before = balance0();
    IUniswapV3SwapCallback(msg.sender).uniswapV3SwapCallback(amount0, amount1, data);   // ask for input
    require(balance0Before.add(uint256(amount0)) <= balance0(), 'IIA');                // verify
} else { ... mirror ... }
```

Like V2, the output goes out optimistically, the caller is asked to pay through a callback, and the pool verifies by balance. Unlike V2 there is no `k` check: the amounts were computed exactly, so the pool just checks it received `amount0`. The caller can use the output tokens inside the callback to source the input (a flash swap). `sqrtPriceLimitX96` is both a slippage guard and a "partial fill" tool: the swap stops at the limit and returns smaller amounts rather than reverting (which is why `SwapRouter.exactOutputInternal` re-checks `amountOutReceived == amountOut` when no limit was passed, `SwapRouter.sol:199`).

### 2.7 `burn` and `collect`

`burn(tickLower, tickUpper, amount)` (`UniswapV3Pool.sol:517-543`) calls `_modifyPosition` with a *negative* delta, which returns negative amounts. It **does not transfer anything**; it adds the freed principal to `position.tokensOwed0/1` (`:535-540`). `burn(..., 0)` is a legal "poke" that just refreshes fees owed (allowed by `Position.update` only if the position has liquidity, `Position.sol:53-55`).

`collect(recipient, tickLower, tickUpper, amount0Requested, amount1Requested)` (`:490-513`) transfers `min(requested, tokensOwed)` of each token and decrements `tokensOwed`. Splitting burn and collect means fees and principal share one withdrawal path, and `collect` never has to touch ticks or liquidity.

### 2.8 `flash`

`flash(recipient, amount0, amount1, data)` (`:791-834`): requires `liquidity > 0`, computes fees `amount * fee / 1e6` rounded up (`:800-801`), transfers both amounts, calls `uniswapV3FlashCallback(fee0, fee1, data)` (`:808`), then requires balances to have grown by at least the fees (`:813-814`). Whatever was actually paid (`paid0`, `paid1`) is split between `protocolFees` and `feeGrowthGlobal` (`:820-831`), so flash loan fees go to LPs exactly like swap fees. Note it is under `lock`, so you cannot swap in the same pool inside the callback.

### 2.9 Oracle

`v3-core/contracts/libraries/Oracle.sol`. Each `Observation` (`:12-21`) stores `blockTimestamp`, `tickCumulative` (`Σ tick * dt`, an int56) and `secondsPerLiquidityCumulativeX128` (`Σ dt / liquidity`, Q128). `transform` (`:30-45`) advances an observation by `delta` seconds at the current tick and liquidity. `write` (`:78-101`) is called from `swap` and in-range `_modifyPosition`; it no-ops if already written this block (`:90`), otherwise writes the next ring slot and grows cardinality if `cardinalityNext` was raised. `grow` (`:108-120`) is what `increaseObservationCardinalityNext` (`UniswapV3Pool.sol:255-267`) calls: it pre-writes `blockTimestamp = 1` into new slots so that later swaps pay a warm SSTORE instead of a cold one. Anyone can pay to lengthen a pool's history.

`observe(secondsAgos[])` (`UniswapV3Pool.sol:236-252` -> `Oracle.observe:300-324` -> `observeSingle:245-287`): for `secondsAgo == 0` it extends the latest observation to now (`:254-258`); otherwise it finds the surrounding observations (`getSurroundingObservations`, `:198-230`, binary search over the ring at `:153`) and linearly interpolates (`:271-286`).

Consumers use tick cumulatives, so the TWAP is a *geometric* mean price. `OracleLibrary.consult` (`v3-periphery/contracts/libraries/OracleLibrary.sol:16-41`):

```solidity
secondsAgos = [secondsAgo, 0];
(tickCumulatives, secondsPerLiquidityCumulativeX128s) = IUniswapV3Pool(pool).observe(secondsAgos);
arithmeticMeanTick = int24((tickCumulatives[1] - tickCumulatives[0]) / secondsAgo);   // rounded toward −inf
harmonicMeanLiquidity = uint128(secondsAgo * 2^160 / (secondsPerLiquidityDelta << 32));
```

The harmonic-mean liquidity lets a consumer reject a TWAP that was cheap to manipulate because liquidity was thin.

### 2.10 Periphery

**NonfungiblePositionManager** (`v3-periphery/contracts/NonfungiblePositionManager.sol`). The pool keys positions by `(owner, tickLower, tickUpper)`, so if two users used the same range through one contract they would collide. The NPM solves this by being the *sole owner* of every position it manages, and keeping its own `_positions[tokenId]` (`:34-52`, `:61`) with a per-token copy of `liquidity`, `feeGrowthInsideLast` and `tokensOwed`. Pools are interned as `uint80 poolId` (`cachePoolKey`, `:119-125`) to shrink the struct.

- `mint(params)` (`:128-182`): `addLiquidity` (`base/LiquidityManagement.sol:51-89`) computes `L` from the desired token amounts (`LiquidityAmounts.getLiquidityForAmounts`, `:71-77`), calls `pool.mint(address(this), ...)` with `MintCallbackData{poolKey, payer: msg.sender}`, and enforces `amount0Min/amount1Min` (`:88`). Back in `mint` it `_mint`s the ERC-721, reads the pool's `feeGrowthInsideLast` for the aggregated position (`:158-159`), and stores the per-token snapshot.
- `increaseLiquidity` (`:198-254`): same `addLiquidity`, then credits the token its share of fee growth since its own last snapshot (`:234-247`), updates the snapshot, adds liquidity.
- `decreaseLiquidity` (`:257-306`): `pool.burn` (`:273`), slippage check, then `tokensOwed += principal + fees accrued since snapshot` (`:281-298`).
- `collect` (`:309-374`): if the token still has liquidity, `pool.burn(..., 0)` as a poke (`:330`) to refresh the aggregate, add the token's fee share, then `pool.collect(recipient, ...)` for `min(tokensOwed, amountMax)` (`:361-367`).
- `burn(tokenId)` (`:377-382`) only when liquidity and owed amounts are 0.

**SwapRouter** (`v3-periphery/contracts/SwapRouter.sol`). Routes are packed bytes `tokenIn(20) | fee(3) | tokenOut(20) | fee(3) | ...` decoded by `Path` (`libraries/Path.sol:42-68`).

- `exactInputSingle` (`:115-129`) -> `exactInputInternal` (`:87-112`): derive `zeroForOne = tokenIn < tokenOut`, call `pool.swap(recipient, zeroForOne, +amountIn, limit or MIN/MAX±1, abi.encode(SwapCallbackData{path, payer}))`, return the negative delta of the output token.
- `exactInput` (`:132-166`) loops: hop 1 pays from `msg.sender` and sends output to the router; every subsequent hop has `payer = address(this)` (`:157`) and the callback pays from the router's own balance (`PeripheryPayments.pay`, `base/PeripheryPayments.sol:52-69`, branch `payer == address(this)`).
- `uniswapV3SwapCallback` (`:57-84`): verify `msg.sender` is the pool for the first segment of `path`; if exact input, `pay(tokenIn, payer, pool, amountToPay)`; if exact output and the path has more pools, *recursively* start the next exact-output swap from inside the callback (`:75-77`), so multi-hop exact output unwinds as nested callbacks, with the final input amount smuggled out through the storage variable `amountInCached` (`:38`, `:79`, `:240`).

**Quoter** (`v3-periphery/contracts/lens/Quoter.sol`). There is no view function for "how much would I get". `quoteExactInputSingle` (`:81-103`) performs a real `pool.swap` inside `try`, and its callback (`:38-66`) does not pay: it `revert`s with the received amount as the 32-byte revert data (`:52-56`). `parseRevertReason` (`:69-78`) decodes it. The swap's state changes are rolled back by the revert, so the caller gets an accurate simulation from an `eth_call`. (QuoterV2 also returns the post-swap price, ticks crossed and gas estimate.)

### 2.11 End-to-end traces

**(a) LP mints an in-range position via the NPM**

```
EOA: token0.approve(NPM), token1.approve(NPM)
EOA -> NPM.mint({token0, token1, fee, tickLower, tickUpper, amount0Desired, amount1Desired, mins, recipient, deadline})   NPM.sol:128
  |-- LiquidityManagement.addLiquidity                                            LiquidityManagement.sol:51
  |     |-- pool = PoolAddress.computeAddress(factory, key)                      PoolAddress.sol:33   (pure)
  |     |-- (sqrtPriceX96,...) = pool.slot0()                                    1 SLOAD
  |     |-- liquidity = LiquidityAmounts.getLiquidityForAmounts(...)            min over both token constraints
  |     '-- pool.mint(NPM, tickLower, tickUpper, liquidity, encode{poolKey, payer: EOA})   UniswapV3Pool.sol:457
  |           |-- lock                                                            :104
  |           |-- _modifyPosition                                                 :306
  |           |     |-- _updatePosition                                           :379
  |           |     |     |-- observations.observeSingle(now)                     Oracle.sol:245
  |           |     |     |-- ticks.update(lower, upper=false)  -> may flip       Tick.sol:110
  |           |     |     |-- ticks.update(upper, upper=true)   -> may flip
  |           |     |     |-- tickBitmap.flipTick x0..2                           TickBitmap.sol:23
  |           |     |     |-- ticks.getFeeGrowthInside                            Tick.sol:60
  |           |     |     '-- position.update(+L, inside0, inside1)               Position.sol:44
  |           |     |-- (tick inside range) observations.write(...)               Oracle.sol:78
  |           |     |-- amount0 = getAmount0Delta(√P, √Pb, +L)  rounded up        SqrtPriceMath.sol:201
  |           |     |-- amount1 = getAmount1Delta(√Pa, √P, +L)  rounded up        SqrtPriceMath.sol:217
  |           |     '-- liquidity += L                                            :361
  |           |-- balance0(), balance1()
  |           |-- NPM.uniswapV3MintCallback(amount0, amount1, data)              LiquidityManagement.sol:25
  |           |     |-- CallbackValidation.verifyCallback  (msg.sender == computed pool)
  |           |     |-- pay(token0, EOA, pool, amount0)  -> token0.transferFrom(EOA, pool)
  |           |     '-- pay(token1, EOA, pool, amount1)  -> token1.transferFrom(EOA, pool)
  |           |-- require(balance grew by amount0 / amount1)  'M0' / 'M1'         :483
  |           '-- emit Mint; unlock
  |-- require(amount0 >= amount0Min && amount1 >= amount1Min)                    LiquidityManagement.sol:88
  |-- _mint(recipient, tokenId = _nextId++)                                       NPM.sol:156
  |-- pool.positions(keccak(NPM, tl, tu)) -> feeGrowthInsideLast snapshot         :158-159
  '-- _positions[tokenId] = {...}; emit IncreaseLiquidity                         :168-181
```

Storage written in the pool: `ticks[tickLower]`, `ticks[tickUpper]`, up to two `tickBitmap` words, `positions[key]`, `liquidity`, `slot0` (observation index) and one `observations[i]`.

**(b) Exact-input swap that crosses one initialised tick**

Setup: price at tick 76012, one position `A` covers `[75000, 76020]` with `L_A`, another `B` covers `[76020, 77000]` with `L_B`. Trader sells token1 for token0 (`zeroForOne = false`, price rises).

```
EOA -> SwapRouter.exactInputSingle({tokenIn: token1, tokenOut: token0, fee, amountIn, amountOutMinimum, sqrtPriceLimitX96: 0})   SwapRouter.sol:115
  '-- exactInputInternal -> pool.swap(EOA, false, +amountIn, MAX_SQRT_RATIO-1, encode{path, payer: EOA})                        :87
        UniswapV3Pool.swap                                                                                                        UniswapV3Pool.sol:596
        |-- slot0 SLOAD, checks, slot0.unlocked = false
        |-- state = {remaining: amountIn, price: √P(76012), tick: 76012, liquidity: L_A, feeGrowth: feeGrowthGlobal1}
        |-- ITERATION 1
        |     |-- nextInitializedTickWithinOneWord(76012, spacing, lte=false) -> 76020, initialized = true                         TickBitmap.sol:42
        |     |-- target = √P(76020) (limit is further away)
        |     |-- computeSwapStep(√P, √P(76020), L_A, remaining, fee)                                                             SwapMath.sol:21
        |     |     amountIn needed to reach 76020 with L_A = L_A * (√P(76020) − √P) ... suppose remaining is bigger, so
        |     |     price -> exactly √P(76020); amountIn/amountOut/feeAmount for that segment
        |     |-- remaining -= amountIn + fee; amountCalculated -= amountOut
        |     |-- feeGrowthGlobal1 += fee * 2^128 / L_A                                                                            :690
        |     |-- price == sqrtPriceNext and initialized  -> cross tick 76020                                                     :693
        |     |     |-- (first cross) cache oracle accumulators                                                                    :698
        |     |     |-- Tick.cross(76020): feeGrowthOutside0/1 = global − outside; returns liquidityNet                           Tick.sol:168
        |     |     |     tick 76020 is A's upper (−L_A) and B's lower (+L_B), so liquidityNet = L_B − L_A
        |     |     '-- state.liquidity = L_A + (L_B − L_A) = L_B
        |     '-- state.tick = 76020                                                                                              :725
        |-- ITERATION 2
        |     |-- next tick -> 77000 (or word edge), target = √P(77000)
        |     |-- computeSwapStep(√P(76020), √P(77000), L_B, remaining, fee): remaining runs out before 77000
        |     |     price -> getNextSqrtPriceFromInput(...)   (SqrtPriceMath.sol:106)
        |     |-- remaining = 0; fee = leftover; feeGrowthGlobal1 += fee * 2^128 / L_B
        |     '-- price != next -> state.tick = getTickAtSqrtRatio(price)                                                          :728
        |-- loop ends (remaining == 0)
        |-- tick changed -> observations.write(...); slot0 = {price, tick, obsIndex, obsCardinality}                              :733-748
        |-- liquidity = L_B                                                                                                       :755
        |-- feeGrowthGlobal1X128 = state.feeGrowthGlobalX128; protocolFees.token1 += ...                                          :763-764
        |-- (amount0, amount1) = (amountCalculated (negative), amountIn)                                                          :767
        |-- token0.transfer(EOA, −amount0)                                                                                        :779
        |-- balance1Before; SwapRouter.uniswapV3SwapCallback(amount0, amount1, data)                                              :782
        |     '-- verifyCallback; pay(token1, EOA, pool, amount1) -> token1.transferFrom(EOA, pool)                               SwapRouter.sol:57
        |-- require(balance1 grew by amount1) 'IIA'                                                                               :783
        '-- emit Swap; slot0.unlocked = true
  '-- require(amountOut >= amountOutMinimum)                                                                                      SwapRouter.sol:128
```

Position `A` stopped earning at the cross; position `B` started. Neither position's storage was touched. They will each learn their share the next time `_updatePosition` runs for them, via `feeGrowthInside`.

---

## 3. Uniswap V4

### 3.1 The architecture shift

V3 deploys one contract per pool and moves ERC-20s on every action. V4 (`v4-core/src/PoolManager.sol`) keeps **all pools in one contract**: `mapping(PoolId id => Pool.State) internal _pools` (`:93`). A pool is identified by a `PoolKey` (`types/PoolKey.sol:11-22`: `currency0`, `currency1`, `fee`, `tickSpacing`, `hooks`) and its id is `keccak256(abi.encode(key))` (`types/PoolId.sol:11-16`). Keys are *not* stored; the caller must pass the full key every time. `Currency` is an address wrapper where `address(0)` means native ETH (`types/Currency.sol:7`, `:38`), so ETH pools need no WETH.

The V3 pool contract became a *library*, `libraries/Pool.sol`, operating on `Pool.State` (`:83-91`): `slot0` (packed into a single `bytes32`, `types/Slot0.sol:8-9`: `sqrtPriceX96 | tick | protocolFee | lpFee`), `feeGrowthGlobal0/1`, `liquidity`, `ticks`, `tickBitmap`, `positions`. Note what is gone: no oracle (`observations`), no per-tick oracle snapshots, no `tokensOwed` in positions (`libraries/Position.sol:19-25`), no `protocolFees` per pool. Those moved to hooks or to the manager.

**`unlock` and flash accounting.** Nothing except `initialize` can be called directly. A caller must first call `unlock(data)` (`PoolManager.sol:104-114`):

```solidity
function unlock(bytes calldata data) external override returns (bytes memory result) {
    if (Lock.isUnlocked()) AlreadyUnlocked.selector.revertWith();
    Lock.unlock();
    // the caller does everything in this callback, including paying what they owe via calls to settle
    result = IUnlockCallback(msg.sender).unlockCallback(data);
    if (NonzeroDeltaCount.read() != 0) CurrencyNotSettled.selector.revertWith();
    Lock.lock();
}
```

Inside `unlockCallback` the caller can do any number of `swap` / `modifyLiquidity` / `donate` / `take` / `settle` / `mint` / `burn` calls (all guarded by `onlyWhenUnlocked`, `:96-99`). None of them move tokens by default. Instead each action *accounts a delta* per `(address, currency)` in **transient storage** (`tstore`/`tload`, EIP-1153, cleared at the end of the transaction):

- `CurrencyDelta` (`libraries/CurrencyDelta.sol:10-41`): slot `keccak256(target, currency)`; `applyDelta` returns previous and next values.
- `NonzeroDeltaCount` (`libraries/NonzeroDeltaCount.sol`): a transient counter of how many `(address, currency)` pairs currently have a non-zero delta. `_accountDelta` (`PoolManager.sol:368-378`) increments it when a delta goes from 0 to non-zero and decrements on the way back.
- `Lock` (`libraries/Lock.sol`): the transient unlocked flag.

Sign convention: **positive delta = the manager owes you; negative = you owe the manager.** The transaction is valid only if every delta is back to zero when `unlockCallback` returns (`:112`). This is why swap outputs, LP withdrawals and hook payments can all be netted within one transaction, and why multi-hop swaps cost one token transfer in and one out regardless of hop count.

**How you settle deltas** (all `PoolManager.sol`):

| Function | Line | Delta effect | Tokens move? |
|---|---|---|---|
| `sync(currency)` | `:279-288` | none; records `balanceOf(manager)` in transient `CurrencyReserves` (`libraries/CurrencyReserves.sol:27-32`) | no |
| `settle()` / `settleFor(to)` | `:300-307` -> `_settle:349-365` | `+ (balanceNow − reservesSynced)` for ERC-20, or `+ msg.value` for native | you must have *already transferred* the ERC-20 to the manager between `sync` and `settle` |
| `take(currency, to, amount)` | `:291-297` | `− amount` to `msg.sender` | manager transfers `amount` to `to` |
| `mint(to, id, amount)` | `:322-329` | `− amount` to `msg.sender` | mints ERC-6909 claim tokens to `to` instead of transferring |
| `burn(from, id, amount)` | `:332-336` | `+ amount` to `msg.sender` | burns claim tokens (`ERC6909Claims._burnFrom`, `ERC6909Claims.sol:13-22`, needs allowance/operator) |
| `clear(currency, amount)` | `:310-319` | zero out an exact *positive* delta | forfeit dust rather than pay gas to take it |

The `sync` -> transfer -> `settle` dance exists because the manager cannot `transferFrom` without an approval to the manager, which V4 avoids. `settle` measures what arrived by comparing balances against the snapshot; the synced currency is stored transiently (`CurrencyReserves.sol:11-13`) so two settles cannot double-count. Native ETH skips the snapshot and uses `msg.value` (`:353-354`).

**ERC-6909 claims** (`ERC6909.sol`): a multi-token standard (`balanceOf[owner][id]`, `id = uint160(currency)`). Frequent traders and routers can keep balances *inside* the manager as claims and skip ERC-20 transfers entirely.

**Reading state.** Since all pools share one contract, there are no per-pool getters. `Extsload` (`Extsload.sol:10`) and `Exttload` (`Exttload.sol:10`) expose raw `sload`/`tload` of arbitrary slots; `StateLibrary` (`libraries/StateLibrary.sol:40-63` for `getSlot0`) knows the layout (`_pools` is slot 6, `:11`, and the offsets of each field inside `Pool.State`, `:14-28`) and decodes it.

### 3.2 Hooks

A pool's `hooks` address is a contract implementing `IHooks` (`interfaces/IHooks.sol`). The manager calls it at up to ten points: `before/afterInitialize`, `before/afterAddLiquidity`, `before/afterRemoveLiquidity`, `before/afterSwap`, `before/afterDonate`. Every hook function must return its own selector (`Hooks.callHook`, `libraries/Hooks.sol:131-155`, check at `:152`), and failures are wrapped in `HookCallFailed` with the original revert bubbled (`:137`).

**Permissions live in the address.** `Hooks.sol:27-47` defines 14 flag bits; the manager checks `uint160(address(hook)) & FLAG != 0` (`hasPermission`, `:337-339`). A hook that wants `beforeSwap` must be deployed (via CREATE2 salt mining) to an address whose bit 7 is set. This makes the permission check a pure bit test with no storage read, and makes the hook's capabilities visible from its address. `validateHookPermissions` (`:83-103`) is meant for hook constructors; `isValidHookAddress` (`:109-127`) is enforced at `initialize` (`PoolManager.sol:126`): a return-delta flag requires the matching action flag, and a non-zero hook address must have at least one flag or a dynamic fee.

**Which calls happen where** (`Hooks.sol`):

- `beforeInitialize` / `afterInitialize` (`:178-192`) around `Pool.initialize`.
- `beforeModifyLiquidity` (`:195-206`) picks `beforeAddLiquidity` when `liquidityDelta > 0`, else `beforeRemoveLiquidity`. `afterModifyLiquidity` (`:209-245`) likewise, and if the hook has the `RETURNS_DELTA` flag it may return a `BalanceDelta` that is *charged to the caller* (`callerDelta = callerDelta − hookDelta`, `:230`/`:242`) and credited to the hook (`PoolManager.sol:181`).
- `beforeSwap` (`:248-282`) may return three things: the selector, a `BeforeSwapDelta`, and an `lpFeeOverride`. The `BeforeSwapDelta` (`types/BeforeSwapDelta.sol`) packs `(specifiedDelta, unspecifiedDelta)`; the *specified* part is added to `amountToSwap` (`:275`) so a hook can take a cut of, or fully replace, the swap (a hook that consumes the whole specified amount makes the pool swap 0 and can fill the trade from its own inventory: the "custom curve" pattern). It may not flip exact-in to exact-out (`:276-278`).
- `afterSwap` (`:285-315`) may return an `int128` delta in the *unspecified* currency; total hook deltas are subtracted from the caller's `swapDelta` and accounted to the hook.
- `beforeDonate` / `afterDonate` (`:318-335`).
- `noSelfCall` (`:171-175`) and the `msg.sender == address(self)` early returns skip hooks when the hook itself is the caller, so a hook can re-enter the manager without infinite recursion.

**Fees.** `PoolKey.fee` is either a static LP fee in pips (max `1_000_000` = 100%, `LPFeeLibrary.sol:25`) or the sentinel `DYNAMIC_FEE_FLAG = 0x800000` (`:15`). Dynamic pools start at 0 (`getInitialLPFee`, `:51-56`) and the hook sets the fee via `updateDynamicLPFee` (`PoolManager.sol:339-346`, hook-only) or per-swap by returning a fee with `OVERRIDE_FEE_FLAG = 0x400000` (`:19`) from `beforeSwap` (`Pool.sol:303-305`). Protocol fees (`ProtocolFees.sol`) are set per pool by the `protocolFeeController` (`:35-41`), capped at 0.1% per direction (`ProtocolFeeLibrary.sol:8`), stored as two 12-bit fields in `slot0`, and accrued per currency in `protocolFeesAccrued` (`:21`). The combined swap fee is `protocol + lp − protocol*lp/1e6` (`ProtocolFeeLibrary.calculateSwapFee`, `:38-46`): protocol first, LP on the remainder.

### 3.3 What moved where in `Pool.sol`

| V3 `UniswapV3Pool` | V4 | Notes |
|---|---|---|
| `initialize` (`:271`) | `Pool.initialize` (`Pool.sol:100-107`) via `PoolManager.initialize` (`:117-142`) | also validates hook address, tick spacing bounds (`TickMath.sol:26-28`), stores the lp fee in slot0 |
| `mint` + `burn` + `_modifyPosition` + `_updatePosition` | `Pool.modifyLiquidity` (`Pool.sol:146-238`) via `PoolManager.modifyLiquidity` (`:145-184`) | one signed `liquidityDelta`; returns `(principalDelta, feesAccrued)`; positions also keyed by a `salt` (`Position.sol:48-67`) so one owner can hold many positions in the same range |
| `collect` / `tokensOwed` | gone | `Position.update` (`Position.sol:76-102`) *returns* fees owed and they are immediately accounted as a positive delta to the caller (`PoolManager.sol:171`). Any `modifyLiquidity(0)` poke collects. |
| `swap` (`:596`) | `Pool.swap` (`Pool.sol:279-463`) via `PoolManager.swap` (`:187-227`) | same loop; `amountSpecified < 0` means exact **input** (`types/PoolOperation.sol:22-23`, `SwapMath.sol:61`), the opposite of V3's sign; `lpFeeOverride` from the hook; protocol fee taken from `amountIn + feeAmount` (`Pool.sol:391-393`); returns a packed `BalanceDelta` (`types/BalanceDelta.sol:14-18`, two int128s in one int256) |
| `flash` | gone | flash accounting makes it unnecessary: `take` in the callback, `settle` before returning |
| oracle | gone | implement as a hook |
| `Tick.update/cross` | `Pool.updateTick` (`:520-558`, packs `liquidityGross`+`liquidityNet` with a single `sstore`), `Pool.crossTick` (`:602-612`) | no oracle fields |
| `Tick.getFeeGrowthInside` | `Pool.getFeeGrowthInside` (`:488-511`) | same subtraction trick |
| — | `Pool.donate` (`:466-480`) via `PoolManager.donate` (`:256-276`) | pay fees directly to in-range LPs by bumping `feeGrowthGlobal` |

`SwapMath.getSqrtPriceTarget` (`v4-core/src/libraries/SwapMath.sol:20-37`) is the branch-free version of V3's ternary at `UniswapV3Pool.sol:665-667`.

### 3.4 End-to-end trace: one swap through a router

`Router` is any contract implementing `IUnlockCallback`. Trader sells `amountIn` of `currency0` for `currency1` on a pool with a `beforeSwap`+`afterSwap` hook.

```
EOA: currency0.approve(Router)
EOA -> Router.swap(key, zeroForOne=true, amountIn, minOut)
  '-- PoolManager.unlock(abi.encode(...))                                                     PoolManager.sol:104
        |-- Lock.unlock()   (tstore)                                                          Lock.sol:10
        '-- Router.unlockCallback(data)                                                       IUnlockCallback.sol:9
              |-- PoolManager.swap(key, {zeroForOne: true, amountSpecified: −amountIn, sqrtPriceLimitX96}, hookData)   :187
              |     |-- onlyWhenUnlocked; checkPoolInitialized
              |     |-- Hooks.beforeSwap(key, params, hookData)                                Hooks.sol:248
              |     |     |-- address bit 7 set -> callHook(abi.encodeCall(IHooks.beforeSwap, ...))   :131
              |     |     '-- parse (selector, BeforeSwapDelta, lpFeeOverride); amountToSwap += specifiedDelta
              |     |-- _swap -> Pool.swap(state, {tickSpacing, zeroForOne, amountToSwap, limit, lpFeeOverride})   Pool.sol:279
              |     |     |-- fee = override ? override : slot0.lpFee; swapFee = calculateSwapFee(protocolFee, lpFee)   :302-308
              |     |     |-- while loop over ticks exactly as V3 (nextInitializedTick, computeSwapStep, crossTick)     :344-437
              |     |     |-- self.slot0 = ...; self.liquidity; feeGrowthGlobal0                                        :439-449
              |     |     '-- return swapDelta = (−amountIn, +amountOut) packed, amountToProtocol, swapFee, result        :451-462
              |     |-- _updateProtocolFees(currency0, amountToProtocol); emit Swap                                     :238-250
              |     |-- Hooks.afterSwap(key, params, swapDelta, hookData, beforeSwapDelta)                             Hooks.sol:285
              |     |     '-- callHookWithReturnDelta(...) -> hookDeltaUnspecified; swapDelta −= hookDelta
              |     |-- hookDelta != 0 -> _accountPoolBalanceDelta(key, hookDelta, hookAddress)                        :224
              |     '-- _accountPoolBalanceDelta(key, swapDelta, Router)                                               :226
              |           |-- _accountDelta(currency0, −amountIn, Router)  -> tstore; NonzeroDeltaCount++              :368
              |           '-- _accountDelta(currency1, +amountOut, Router) -> tstore; NonzeroDeltaCount++
              |-- require(amountOut >= minOut)                                                  (router's own check)
              |-- PoolManager.take(currency1, EOA, amountOut)                                   :291
              |     |-- _accountDelta(currency1, −amountOut, Router) -> delta 0; NonzeroDeltaCount−−
              |     '-- currency1.transfer(EOA, amountOut)                                      Currency.sol:40
              |-- PoolManager.sync(currency0)                                                   :279  (tstore reserves snapshot)
              |-- currency0.transferFrom(EOA, PoolManager, amountIn)                            (router pulls from user)
              '-- PoolManager.settle()                                                          :300 -> _settle:349
                    |-- paid = balanceNow − reservesSynced = amountIn; resetCurrency
                    '-- _accountDelta(currency0, +amountIn, Router) -> delta 0; NonzeroDeltaCount−−
        |-- require(NonzeroDeltaCount.read() == 0)   'CurrencyNotSettled'                       :112
        '-- Lock.lock()
```

If the hook took a positive delta for itself in `afterSwap`, the hook's `(currency, delta)` is also non-zero at this point, and the *hook* must have settled it (e.g. by `take`-ing its cut, or by `mint`-ing itself claims) inside its own callback, or the whole transaction reverts. Persistent storage written: `_pools[id].slot0`, `liquidity`, `feeGrowthGlobal0X128`, any crossed `ticks`, `protocolFeesAccrued[currency0]`, and the two ERC-20 balance mappings. Everything else was transient.

---

## 4. V2 vs V3 vs V4

| | V2 | V3 | V4 |
|---|---|---|---|
| Liquidity model | full range, `x*y=k` | concentrated per-position ranges on a tick grid; `√P` and `L` are the state | same math as V3 |
| Position representation | fungible ERC-20 LP token per pair | `(owner, tickLower, tickUpper)` in the pool; NFT wrapper in periphery | `(owner, tickLower, tickUpper, salt)` in the manager; NFT wrapper in periphery |
| Fee tiers | fixed 0.30% | 0.05 / 0.30 / 1.00% (+ governance-added), fixed per pool | any static fee up to 100%, or dynamic fee set by a hook per swap |
| Fee accounting | fees stay in reserves, `k` grows, LP token appreciates | `feeGrowthGlobal` / `feeGrowthOutside` / `feeGrowthInside`, `tokensOwed`, `collect` | same growth math; fees returned as deltas on every `modifyLiquidity` |
| Protocol fee | 1/6 of `√k` growth minted as LP tokens, switch off by default | up to 1/4..1/10 of swap fees per token, per pool | up to 0.1% of input per direction, per pool, accrued per currency |
| Oracle | cumulative price (arithmetic TWAP), 1 slot | `observations` ring buffer of tick cumulatives (geometric TWAP) + seconds-per-liquidity | none in core; write one as a hook |
| Custody | each pair holds its own tokens | each pool holds its own tokens | one `PoolManager` holds everything; ERC-6909 claims optional |
| Settlement | balance diff + `k` check after optimistic transfer | callback per action + balance check | transient deltas per `(address, currency)` must net to zero per `unlock` |
| Native ETH | no (WETH) | no (WETH) | yes (`Currency(address(0))`) |
| Extensibility | none | none | hooks with 14 permission flags, return deltas, custom curves |
| Gas (rough) | cheapest swap; two SSTOREs | more expensive: tick walking, oracle write | cheaper than V3 for multi-hop and for pool creation (no contract deploy); hooks add cost |
| Reentrancy guard | `unlocked` storage flag | `slot0.unlocked` | transient `Lock` + delta accounting |
| Solidity | 0.5.16 / 0.6.6 | 0.7.6 | 0.8.26 (transient storage, custom errors, user-defined value types) |

---

## 5. Security notes and classic bugs

- **Callbacks are untrusted code.** V2 `swap` (`:172`), V3 `mint/swap/flash` (`:482`, `:776`, `:808`), and every V4 hook and `unlockCallback` hand control to an external contract mid-function. The pools defend themselves with the `lock` and by verifying balances *after* the callback; integrators must defend themselves by verifying `msg.sender` is the real pool (`CallbackValidation.verifyCallback`). A contract that implements `uniswapV3SwapCallback` without that check can be drained by anyone calling it directly.
- **Spot price is not an oracle.** `reserve1/reserve0` (V2) or `slot0.sqrtPriceX96` (V3) can be moved arbitrarily within one transaction with a flash loan. Use the cumulative accumulators over a window (`UniswapV2OracleLibrary`, `OracleLibrary.consult`) and, in V3, check `harmonicMeanLiquidity` and how far back `observations` go. Even a TWAP can be moved by a multi-block attacker if liquidity is thin.
- **Rounding direction.** V3 rounds every amount the pool *receives* up and every amount it *pays* down (`SqrtPriceMath.sol:201-226`, `SwapMath.sol` `roundUp` flags). `getAmountIn` in V2 adds `+1` (`UniswapV2Library.sol:58`). Reversing any of these gives a "wei-by-wei" drain. If you write an integration that computes amounts yourself, copy the rounding.
- **Fee-on-transfer and rebasing tokens.** V2's `getAmountsOut` path fails the k-check with deflationary tokens; use the `SupportingFeeOnTransferTokens` variants. V3's `require(balanceBefore + amount <= balanceAfter)` makes fee-on-transfer tokens revert every mint and swap, so V3 simply does not support them. Rebasing tokens leave V2 balances out of sync (`sync`/`skim`) and in V3 the extra balance is unreachable by LPs.
- **First-depositor / inflation attack.** V2 burns `MINIMUM_LIQUIDITY` (`UniswapV2Pair.sol:121`). Vaults that copy the "shares = amount * supply / balance" pattern without an equivalent are vulnerable to a donate-then-round-down attack.
- **`initialize` is permissionless.** In V3 and V4 anyone can initialise a pool at any price. A malicious first price is instantly arbitraged, so the *first LP* is who gets hurt; front-ends check the price before minting.
- **`uint112` overflow in V2.** `_update` reverts if a balance exceeds `2^112 − 1` (`UniswapV2Pair.sol:74`). Someone can brick a pair by donating; `skim` is the recovery.
- **Tick spacing and `maxLiquidityPerTick`.** `enableFeeAmount` caps `tickSpacing < 16384` (`UniswapV3Factory.sol:67`) because `nextInitializedTickWithinOneWord` multiplies compressed ticks back by spacing and would overflow `int24`. `Tick.update` enforces `maxLiquidityPerTick` (`Tick.sol:128`) so `liquidityGross` cannot overflow `uint128` when summed across ticks.
- **`sqrtPriceLimitX96` and sandwiches.** Passing `MIN/MAX_SQRT_RATIO ± 1` (the router default) means "any price"; the only protection is `amountOutMinimum`. A tight limit gives a partial fill instead of a revert, which some integrations mishandle (`SwapRouter.sol:199` guards the exact-output case).
- **V3 tick/price lag.** After a leftward cross `slot0.tick` is `tickNext − 1` while `sqrtPriceX96` sits exactly on `tickNext` (`UniswapV3Pool.sol:725`, and the V4 comment at `Pool.sol:409-412`). Code that recomputes the tick from the price and compares will see a mismatch of one; `donate` in V4 must check both.
- **V4 hooks are trusted by the pool's LPs and traders.** A hook can revert every swap (censorship), take arbitrary deltas (`RETURNS_DELTA` flags), set a 100% dynamic fee, or be upgradeable. The hook address is part of the `PoolKey`, so a "USDC/ETH 0.05%" pool with a bad hook is a *different pool*; always verify the full key. The manager only guarantees that whatever the hook takes is charged to the swapper and that all deltas net out.
- **V4 `settle` ordering.** Calling `settle` without a preceding `sync` for an ERC-20 attributes the wrong reserve snapshot; `collectProtocolFees` refuses to run while a currency is synced (`ProtocolFees.sol:49-52`) precisely to stop balance-based accounting from being confused. Native ETH `settle` needs `msg.value`, and a non-zero `msg.value` with an ERC-20 synced reverts (`PoolManager.sol:356`).
- **`feeGrowthGlobal` can be inflated by `donate`** in a V4 pool with a single position (`Pool.sol:80-82`); do not use it as a metric across pools.

---

## 6. Exercises to trace yourself

1. **V2 fee proof.** Open `v2-core/contracts/UniswapV2Pair.sol:176-182`. With `reserve0 = 1000`, `reserve1 = 1000`, `amount0Out = 0`, `amount1Out = 90`, work out by hand the minimum `balance0` that passes the k-check, then confirm it equals `getAmountIn(90, 1000, 1000)` from `UniswapV2Library.sol:53-59`.
2. **V2 protocol fee.** Set `feeTo`, do two swaps, then call `mint`. Follow `_mintFee` at `UniswapV2Pair.sol:89-107` and verify that the shares minted to `feeTo` are worth exactly 1/6 of the `√k` growth by computing the pool value before and after.
3. **V3 tick bitmap.** With `tickSpacing = 60` and initialised ticks at `-120`, `0`, `600`, hand-trace `TickBitmap.nextInitializedTickWithinOneWord(tick = 30, 60, lte = true)` and `(tick = 30, 60, lte = false)` at `TickBitmap.sol:42-77`, including the `compressed` rounding at `:48-49`. Then do `tick = -1` and see why the extra `compressed--` exists.
4. **V3 fee growth invariance.** Take a position `[Pa, Pb]` and a price path that starts below `Pa`, goes above `Pb`, then comes back inside. Track `feeGrowthOutside` of both ticks through `Tick.update` (`Tick.sol:132-140`) and each `Tick.cross` (`:178-179`), then compute `getFeeGrowthInside` (`:60-95`) before and after and convince yourself the difference equals only the fees earned while inside the range.
5. **V3 swap step.** Start from `UniswapV3Pool.sol:663-671` and step into `SwapMath.computeSwapStep` (`SwapMath.sol:21-97`) for an exact-*output* swap (`amountRemaining < 0`) that hits the price target. Identify which of `amountIn`/`amountOut` is recomputed at `:70-84` and why `feeAmount` uses `1e6 − feePips` as the denominator.
6. **V3 exact-output multi-hop.** Trace `SwapRouter.exactOutput` (`SwapRouter.sol:224-243`) for a 2-pool path. Draw the call stack at the moment the innermost `pay` runs and explain how `amountInCached` (`:38`, `:79`, `:240`) carries the answer out through the nested callbacks.
7. **V4 delta netting.** Write on paper the sequence of `_accountDelta` calls (`PoolManager.sol:368-378`) and the value of `NonzeroDeltaCount` for a router that does two swaps A->B then B->C inside one `unlockCallback` and only `settle`s A and `take`s C. Explain why no B transfer ever happens.
8. **V4 hook delta.** A `beforeSwap` hook returns `BeforeSwapDelta(specified = +amountIn, unspecified = −X)` on an exact-input swap. Follow `Hooks.beforeSwap` (`Hooks.sol:248-282`) and `Hooks.afterSwap` (`:285-315`), then `PoolManager.swap` (`:187-227`), and work out (a) what amount `Pool.swap` receives, (b) what the trader's final delta is, (c) what the hook must do before `unlock` returns.
9. **V4 address mining.** Using `Hooks.sol:27-47`, compute the lowest 14 bits an address needs to get `beforeSwap`, `afterSwap` and `beforeSwapReturnDelta`, and check `isValidHookAddress` (`:109-127`) would accept it with a static fee and with `DYNAMIC_FEE_FLAG`.
