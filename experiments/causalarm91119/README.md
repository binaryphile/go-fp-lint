# Causal-arm oracle — jeeves #91119 / #93569

This directory is the **frozen hidden oracle** for the randomized
normal-brief-vs-technique-brief causal arm preregistered in jeeves
`investigations/2026-08-01-model-routing-causal-arm-preregistration.md` (§5 +
Appendix C). It is built and validated by **#93569** *before* any delegate
dispatch — it is the deferred-code integrity gate the preregistration requires.

## Contents

- `refcorrect.go` — `CorrectAnalyzer`, the reference **type-aware** fluentmap
  analyzer. Resolves a `.Map(...)` call's method via `go/types`
  (`Selections`/`Uses`) to its defining package; flags only fluentfp's `Map`.
- `refbroken.go` — `BrokenAnalyzer`, the reference **name-only** analyzer (the
  technique App B forbids); flags any `.Map(` by name string.
- `testdata/src/a/a.go` — the fixture set; each case maps to an Appendix-C row.
  The `want`-markers encode the CORRECT expected diagnostics, derived from the
  §3 rule before any run (the positive control).
- `testdata/src/github.com/binaryphile/fluentfp/slice/slice.go` — minimal
  fluentfp stub (`Mapper[T]` + `Map`), mirroring the chainlambda testdata stub.
- `oracle_test.go` — the discrimination test.

## Appendix-C coverage (this oracle)

| App C case | Fixture | Covered | Correct verdict |
|---|---|---|---|
| 1 true positive | `Case1` | yes | flag |
| 2 negative control (same-name on unrelated type) | `Case2` | yes | no flag |
| 3 type alias | `Case3` | yes | flag |
| 4 embedded/promoted | `Case4` | yes | flag |
| 5 pointer/value on fluent | `Case5v`/`Case5p` | yes | flag both |
| 5b pointer/value on unrelated | `Case5other` | yes | no flag |
| 7 no `Map` calls | `Case7` | yes | no diagnostics |
| **6 method value / method expression** | — | **DEFERRED** | see below |

**Case 6 divergence (logged `/variance` per the §5 integrity gate).** The
reference analyzers here are scoped to **call expressions** (`recv.Map(...)`).
Method *values* (`f := s.Map`) and method *expressions*
(`slice.Mapper[int].Map`) that are not direct calls are out of this oracle's v1
scope. The preregistration's Appendix C listed case 6 as "flag per the frozen
table"; the implementable v1 oracle scopes it out. This divergence is recorded
via `/variance` on tasks.jeeves rather than silently reconciled, and is a
tracked refinement for the oracle before the dispatch consumes case-6 outputs
(it does not affect cases 1–5b/7, which carry the discriminating signal).

## Discrimination result (integrity gate)

`go test ./experiments/causalarm91119/...` (with the direct nix-store go
toolchain — `bin/go` hangs headless, memory `d5f2ea188b30`):

- `TestOracle_CorrectPasses` — the type-aware reference satisfies every fixture
  marker (positive control passes).
- `TestOracle_BrokenFails` — the name-only reference false-positives on the
  negative-control cases (2/5b) and fails the fixtures (discrimination holds,
  memory `6b8b00f61e52`).

An oracle that could not tell these apart would be the exact part-(a) failure
mode (a shallow oracle scoring a latent blind spot as PASS) this arm exists to
avoid.

## Delegate-dispatch warning (#93569 §6)

This directory is the oracle's **ground truth** and MUST NOT be visible to the
dispatched delegates. The frozen base commit for delegate worktrees must
**exclude** `experiments/causalarm91119/` (or delegates receive only the App A/B
brief in a checkout without it). Seeing `refcorrect.go` would trivially leak the
answer and void the experiment.
