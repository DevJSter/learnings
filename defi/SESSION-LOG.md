# Session log — how these documents were produced

A record of what was run, what each agent produced, what is finished, and what
is still owed. Written so the work can be picked up later without re-deriving
the plan.

## What was set up

Thirteen protocol repositories were shallow-cloned and their `.git` directories
removed, giving one flat greppable corpus:

```
uni/     v1-contracts  v2-core  v2-periphery  v3-core  v3-periphery  v4-core
curve/   curve-contract  stableswap-ng  curve-dao-contracts
aave/    v1-aave-protocol  v2-protocol  v3-core-original  aave-v3-origin (v3.6)  v4-aave
lifi/    contracts
```

Aave v4 turned out to be public at `aave/aave-v4`, with audits dated through
2026, so it is covered from real source rather than from the announcement.

## How the work was split

Twenty agent runs in total, each given one file to own and told not to touch any
other. Two layers were commissioned: **deep dives** that teach the ideas and
derive the math, and **complete references** that walk every contract and every
function. Running them in parallel meant the whole set landed in roughly the
time one document takes, rather than in sequence over several hours.

One early wave of six agents was lost to a session rate limit before any of them
wrote output. Three were resumed from their own transcripts the next day and
finished; the other three had already written their files.

## Finished and verified

| Document | Lines | Notes |
|---|---|---|
| `uni/UNISWAP-V1-DEEP-DIVE.md` | 1,327 | All 44 functions across both contracts, confirmed by script |
| `uni/UNISWAP-DEEP-DIVE.md` | 874 | v2, v3, v4 |
| `curve/CURVE-DEEP-DIVE.md` | 988 | StableSwap classic and NG, plus the veCRV flywheel |
| `aave/AAVE-V1-V2-DEEP-DIVE.md` | 960 | |
| `aave/AAVE-DEEP-DIVE.md` | 773 | v3.6 |
| `aave/AAVE-V4-DEEP-DIVE.md` | 1,255 | Hub and Spoke, from source |
| `lifi/LIFI-DEEP-DIVE.md` | 1,204 | |
| `uni/V2-COMPLETE-REFERENCE.md` | 2,447 | All 35 files |
| `uni/V3-CORE-COMPLETE-REFERENCE.md` | 2,689 | All 62 files |
| `uni/V4-COMPLETE-REFERENCE.md` | 2,638 | All 84 files |
| `lifi/FACETS-COMPLETE-REFERENCE.md` | 2,239 | All 42 facets |

## Stopped mid-write

The run was halted to conserve budget. These files are substantial and readable,
but each is missing its closing sections and none got a final citation sweep.
Treat their line citations as unverified.

| Document | Lines | Reached | Still owed |
|---|---|---|---|
| `curve/STABLESWAP-NG-COMPLETE-REFERENCE.md` | 3,125 | §19 of 19 | Was applying its own correction pass; content essentially complete |
| `aave/V3-PROTOCOL-COMPLETE-REFERENCE.md` | 3,136 | §18 | Configuration contracts, tables, use cases |
| `lifi/LIBRARIES-PERIPHERY-COMPLETE-REFERENCE.md` | 2,660 | §19 | Cross-cutting tables, use cases |
| `aave/V3-PERIPHERY-COMPLETE-REFERENCE.md` | 2,570 | §12 | Deployments walkthrough, tables, use cases |
| `aave/V4-COMPLETE-REFERENCE.md` | 2,413 | §17 | Use cases, v3 to v4 migration table |
| `uni/V3-PERIPHERY-COMPLETE-REFERENCE.md` | 2,313 | §8 | Tables, revert decoder, use cases |
| `curve/DAO-COMPLETE-REFERENCE.md` | 1,590 | §6 | Proxies, vesting, streamers, tables, flywheel diagram |
| `aave/V1-V2-COMPLETE-REFERENCE.md` | 1,492 | §1.16 | All of Part II (Aave v2) and the v1 to v2 migration table |

## Not started

`curve/CLASSIC-POOLS-COMPLETE-REFERENCE.md` — the classic pool templates and the
roughly 35 deployed pool families. That agent was still surveying and diffing
pools against their templates when the run was stopped, so no file exists.
`curve/CURVE-DEEP-DIVE.md` §1 already covers the 3pool in depth, which is the
canonical instance of the base template.

## Things that were verified rather than assumed

Worth keeping, because these are the claims most likely to be wrong in notes
written from memory:

- The `v3-core` tree was compiled with solc 0.7.6 and its pool creation code
  hashes to the value hardcoded as `POOL_INIT_CODE_HASH` in the periphery. The
  cloned source is the deployed protocol.
- Uniswap v4 function selectors were recomputed with `cast sig`. The repo's own
  `signatures/` JSON is wrong for `swap`, having hashed the literal word
  `PoolKey` instead of the expanded tuple.
- `UniswapV2Router01.getAmountIn` calls `UniswapV2Library.getAmountOut`. This is
  a real upstream bug, fixed in Router02. Only the public view helper was
  affected, since swaps call `getAmountsIn` directly.
- Aave v4 has no `calculateCompoundedInterest` anywhere in `src/`, and no
  e-mode. Supply uses ERC-4626 shares with a virtual offset while debt uses a ray
  index, so there is no `liquidityIndex`.
- Uniswap v1 moves ETH with `send` at exactly four sites, which is what makes
  the missing reentrancy lock survivable.

## If you pick this up again

The eight stopped documents each need their remaining sections plus a citation
sweep. The pattern that worked: give one agent one file, tell it to enumerate
every source file first, verify every `path:line` with `grep -n` before writing,
and append in chunks rather than one heredoc. The heredoc limit was the single
most common failure mode.
