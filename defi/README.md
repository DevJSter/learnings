# DeFi Deep Dives — Uniswap, Curve, Aave, LI.FI

Source-first study notes on four protocols that between them cover the three big
primitives of on-chain finance: **exchange** (Uniswap, Curve), **credit** (Aave)
and **routing across chains** (LI.FI).

Every protocol's real source is cloned into this folder with its `.git` removed,
so the tree is one flat, greppable corpus. Every claim in the notes cites a
`file:line` that was verified against these exact files with `grep -n`. Open the
code next to the prose — the notes are written to be read that way, not instead
of it.

---

## The documents

There are two layers. **Deep dives** teach the ideas: they pick the important
code, derive the math, and trace end-to-end user actions. **Complete
references** are exhaustive: every contract, every function, every parameter,
every revert string, every event, every storage slot. Read a deep dive to
understand a protocol; read its reference to have seen all of it.

### Deep dives — read these first

| Doc | Covers |
|---|---|
| [`uni/UNISWAP-V1-DEEP-DIVE.md`](uni/UNISWAP-V1-DEEP-DIVE.md) | Uniswap V1 (2018): the original Vyper ETH-paired AMM |
| [`uni/UNISWAP-DEEP-DIVE.md`](uni/UNISWAP-DEEP-DIVE.md) | Uniswap V2, V3, V4: constant product, concentrated liquidity, singleton + hooks |
| [`curve/CURVE-DEEP-DIVE.md`](curve/CURVE-DEEP-DIVE.md) | Curve StableSwap (classic + NG) and the veCRV / gauge / emissions stack |
| [`aave/AAVE-V1-V2-DEEP-DIVE.md`](aave/AAVE-V1-V2-DEEP-DIVE.md) | Aave V1 and V2: LendingPoolCore, rebasing aTokens, debt tokenization |
| [`aave/AAVE-DEEP-DIVE.md`](aave/AAVE-DEEP-DIVE.md) | Aave V3.6: Pool + logic libraries, index math, liquidations, eMode, flash loans |
| [`aave/AAVE-V4-DEEP-DIVE.md`](aave/AAVE-V4-DEEP-DIVE.md) | Aave V4: the Hub & Spoke redesign, shares accounting, risk premiums |
| [`lifi/LIFI-DEEP-DIVE.md`](lifi/LIFI-DEEP-DIVE.md) | LI.FI: EIP-2535 Diamond, bridge facets, Executor/Receivers, DEX aggregator |

### Complete references — every contract, every function

| Doc | Covers |
|---|---|
| [`uni/V2-COMPLETE-REFERENCE.md`](uni/V2-COMPLETE-REFERENCE.md) | All of v2-core and v2-periphery, including the `examples/` contracts |
| [`uni/V3-CORE-COMPLETE-REFERENCE.md`](uni/V3-CORE-COMPLETE-REFERENCE.md) | Pool, Factory, Deployer and all 15 math libraries, derivations included |
| [`uni/V3-PERIPHERY-COMPLETE-REFERENCE.md`](uni/V3-PERIPHERY-COMPLETE-REFERENCE.md) | Position manager, router, quoters, lenses, on-chain SVG pipeline |
| [`uni/V4-COMPLETE-REFERENCE.md`](uni/V4-COMPLETE-REFERENCE.md) | PoolManager, Pool, Hooks, every type and transient-storage library |
| [`curve/CLASSIC-POOLS-COMPLETE-REFERENCE.md`](curve/CLASSIC-POOLS-COMPLETE-REFERENCE.md) | Every classic pool template and all ~35 deployed pool families, plus zaps |
| [`curve/STABLESWAP-NG-COMPLETE-REFERENCE.md`](curve/STABLESWAP-NG-COMPLETE-REFERENCE.md) | Every NG contract: pool, metapool, views, math, factory, zap, gauge |
| [`curve/DAO-COMPLETE-REFERENCE.md`](curve/DAO-COMPLETE-REFERENCE.md) | CRV, veCRV, GaugeController, every gauge version, Minter, fees, vesting |
| [`aave/V3-PROTOCOL-COMPLETE-REFERENCE.md`](aave/V3-PROTOCOL-COMPLETE-REFERENCE.md) | v3.6 core: Pool, every logic library, tokenization, config bitmaps, math |
| [`aave/V3-PERIPHERY-COMPLETE-REFERENCE.md`](aave/V3-PERIPHERY-COMPLETE-REFERENCE.md) | v3.6 rewards, oracle, rate strategy, stata-token, config engine, deployments |
| [`aave/V4-COMPLETE-REFERENCE.md`](aave/V4-COMPLETE-REFERENCE.md) | v4 Hub, Spoke, liquidation, position managers, access control, config engine |
| [`lifi/FACETS-COMPLETE-REFERENCE.md`](lifi/FACETS-COMPLETE-REFERENCE.md) | All ~43 facets: every bridge integration and every infrastructure facet |
| [`lifi/LIBRARIES-PERIPHERY-COMPLETE-REFERENCE.md`](lifi/LIBRARIES-PERIPHERY-COMPLETE-REFERENCE.md) | Diamond internals, LibSwap/LibAsset/allowlists, Executor, receivers, aggregator |

Each reference ends with an ABI/selector table, a storage-layout table, an events
reference, a revert-string decoder, and a use-case index that maps "I want to do
X" to the exact function and its full internal call chain.

## The source tree

```
uni/     v1-contracts/  v2-core/  v2-periphery/  v3-core/  v3-periphery/  v4-core/
curve/   curve-contract/  stableswap-ng/  curve-dao-contracts/
aave/    v1-aave-protocol/  v2-protocol/  v3-core-original/  aave-v3-origin/ (v3.6)  v4-aave/
lifi/    contracts/
```

`aave/aave-v3-origin` is the current v3.6 release; `aave/v3-core-original` is the
original v3.0 for diffing. `aave/v4-aave` is the public V4 code, audited through
2026 (see its `audits/`).

---

## A suggested reading order

If you read these front to back you will build each idea on the previous one.

1. **`uni/UNISWAP-V1-DEEP-DIVE.md`** — the smallest complete AMM that ever
   shipped. A few hundred lines of Vyper. Learn `x*y=k`, LP shares and the
   exact-in/exact-out distinction with nothing else in the way.
2. **`uni/UNISWAP-DEEP-DIVE.md` §0–1** — V2. The same math, done properly:
   arbitrary pairs, CREATE2 addresses, flash swaps, TWAP accumulators, and the
   "optimistic transfer then check `k`" pattern that half of DeFi copies.
3. **`curve/CURVE-DEEP-DIVE.md` §0–1** — a *different* invariant for the same
   job. This is where you learn that an AMM is just a curve, and that choosing
   the curve is the whole product. Newton's method appears because the curve has
   no closed form.
4. **`uni/UNISWAP-DEEP-DIVE.md` §2** — V3. Ticks, `√P`, and fee-growth
   accounting. The hardest math in the set, and the pattern most often copied
   badly.
5. **`aave/AAVE-V1-V2-DEEP-DIVE.md`** then **`aave/AAVE-DEEP-DIVE.md`** —
   lending. Index-based interest, scaled balances, health factors, liquidations.
   Reading v1 and v2 first makes v3's design choices legible as answers to
   specific failures.
6. **`curve/CURVE-DEEP-DIVE.md` §3** — veCRV, gauges, emissions. The canonical
   "staking" design: vote-escrow, boosts, and a weekly emission split by vote.
7. **`uni/UNISWAP-DEEP-DIVE.md` §3** and **`aave/AAVE-V4-DEEP-DIVE.md`** — the
   two current-generation redesigns. Both move from "one contract per market" to
   "one shared core plus pluggable modules". Read them together; the parallel is
   not a coincidence.
8. **`lifi/LIFI-DEEP-DIVE.md`** — the integration layer. Everything above is a
   callee here. This is also where you see what production infrastructure code
   actually looks like: allowlists, upgrade governance, emergency pauses.

---

## Cross-cutting concepts, and where each is explained

These ideas recur across all four protocols. When one clicks, it clicks
everywhere.

**Shares vs. assets (the accounting backbone).** Never store a user's balance as
an amount; store a *share* and multiply by a global index at read time. Uniswap
LP tokens (`UNISWAP-DEEP-DIVE.md` §1.4), Curve's `D`-based LP token and
`get_virtual_price` (§1.6), Aave's `scaledBalance × liquidityIndex`
(`AAVE-DEEP-DIVE.md` §2.4), Aave V4's share price. Same trick, four dialects.
The attack that follows it everywhere is share-price manipulation by donation:
compare Uniswap's `MINIMUM_LIQUIDITY` burn, Curve's use of `D` rather than
`balanceOf` as numeraire, and Aave's `virtualUnderlyingBalance`.

**Growth-per-unit accumulators.** To pay N users a stream without touching N
storage slots, track a global "growth per unit of stake" and store each user's
snapshot of it; the difference is what they are owed. Uniswap V2's price
cumulative, V3's `feeGrowthInside` (§2.5), Curve's gauge `integrate_inv_supply`
(§3.4), Aave's `RewardsDistributor` index (§5). If you understand one you
understand all four, including why they all overflow on purpose.

**Callbacks and the "check after" pattern.** Uniswap V2 `swap` transfers first
and verifies `k` afterwards, enabling flash swaps for free (§1.7). V3 makes it
explicit with `uniswapV3SwapCallback` (§2.6). V4 generalises it to net deltas
across a whole `unlock` (§3.1). Aave's flash loan is the same shape
(`AAVE-DEEP-DIVE.md` §3.9). LI.FI's whole design is calling untrusted contracts
and checking balances afterwards (`LIFI-DEEP-DIVE.md` §2). The invariant is
always: *hand out control, then prove nothing was stolen.*

**Rounding is a security property.** Every one of these codebases rounds in the
protocol's favour, deliberately, in a specific direction per operation. Uniswap
V3's `roundUp` flags, Curve's `+1`s in `get_y`, Aave's `TokenMath`. Reversing a
single one is a slow drain.

**Spot price is not an oracle.** Uniswap V2/V3 cumulative accumulators, Curve's
EMA `price_oracle`, Aave's Chainlink dependency. And the inverse failure:
Curve's read-only reentrancy, where a *view* function lies mid-transaction
(`CURVE-DEEP-DIVE.md` §5.1). That one is worth reading even if you never touch
Curve.

**Singleton + modules is the current answer to fragmentation.** Uniswap V4's
PoolManager with hooks, Aave V4's Hub with Spokes, LI.FI's Diamond with facets.
Three different problems, one architectural move: shared state in an immutable
core, behaviour in swappable modules, and a hard question about who is allowed
to add a module.

---

## How the notes are structured

Each document follows the same shape, so you can navigate any of them once you
know one:

- **§0** — the mental model in plain English, with a worked numeric example, before any code.
- **§1..n** — contract map, then storage layout, then every external function traced as: inputs → checks → state writes → external calls → returns/events.
- **End-to-end traces** — a real user action followed hop by hop with real numbers.
- **Comparison table** — versions against each other, or against the other protocols.
- **Security notes** — the classic bugs, each tied to the line of code that prevents (or caused) it.
- **Exercises** — concrete "open this file at this line and work it out" tasks. These are the part that actually teaches; do them.

---

## Verifying as you read

The line citations are checkable and you should check them, because the code
moves:

```bash
sed -n '176,182p' uni/v2-core/contracts/UniswapV2Pair.sol      # the k-check
grep -n 'function executeLiquidationCall' aave/aave-v3-origin/src/contracts/protocol/libraries/logic/LiquidationLogic.sol
grep -n 'def get_D' curve/curve-contract/contracts/pools/3pool/StableSwap3Pool.vy
```

Uniswap V4, Aave V3.6 and Aave V4 are Foundry projects; `forge build` and
`forge test` in those directories will run the real test suites, which are
themselves the best documentation of intended behaviour.
