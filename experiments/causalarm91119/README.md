# Causal-arm oracle — jeeves #91119 / #93569

This directory is the **frozen hidden oracle** for the randomized
normal-brief-vs-technique-brief causal arm preregistered in jeeves
`investigations/2026-08-01-model-routing-causal-arm-preregistration.md` (§5 +
Appendix C). It is built and validated by **#93569** *before* any delegate
dispatch — it is the deferred-code integrity gate the preregistration requires.

**v2 (post cross-vendor IMPL grade #93725, D → SEND BACK).** The v1 oracle was
contract-divergent (checked the method's defining package, not the receiver
type; dropped the `: <recv-type>` message suffix; loose substring markers hid
both; fixtures missed the embedded-unrelated and renamed-import negatives). This
version fixes those (findings F4/F5/F6).

## Contents

- `refcorrect.go` — `CorrectAnalyzer`, the reference **type-aware** analyzer.
  Resolves the called method to a `*types.Func`, then checks its **RECEIVER
  type** is fluentfp's `slice.Mapper` (name + exact package path
  `github.com/binaryphile/fluentfp/slice`) — via the method signature's
  receiver, so type aliases, renamed imports, and embedding/promotion all
  resolve correctly, and a same-named `Map` on any other type (or a different
  fluentfp type) is not flagged. Emits exactly `fluent Map call: <recv-path>.<name>`.
- `refbroken.go` — `BrokenAnalyzer`, the reference **name-only** analyzer (the
  technique App B forbids). Decides purely on the selector name `Map`; emits the
  same message on shared flags, so its ONLY divergence is false positives on the
  negative controls.
- `testdata/src/a/a.go` + `a2.go` — the fixture set; each case maps to an
  Appendix-C row. The want-markers are the exact expected diagnostics (message
  incl. receiver type), derived from the rule before any run (positive control).
- `testdata/src/b/b.go` — the isolated no-op package (Case 7).
- `testdata/src/github.com/binaryphile/fluentfp/slice/slice.go` — fluentfp stub
  (`Mapper[T]` + `Map`), matching real fluentfp's path + slice type.
- `oracle_test.go` — the discrimination test (capturing recorder).

## Appendix-C coverage (this oracle)

| App C case | Fixture | Covered | Correct verdict |
|---|---|---|---|
| 1 true positive | `a.Case1` | yes | flag |
| 2 negative control (same-name on unrelated type) | `a.Case2` | yes | no flag |
| 3 type alias | `a.Case3` | yes | flag |
| 3b renamed import | `a.Case3b` (a2.go) | yes | flag (no false negative) |
| 4 embedded/promoted on fluent type | `a.Case4` | yes | flag |
| 4b embedded/promoted on unrelated type | `a.Case4b` | yes | no flag |
| 5 pointer/value on fluent | `a.Case5v`/`a.Case5p` | yes | flag both |
| 5b pointer/value on unrelated | `a.Case5other` | yes | no flag |
| 7 isolated no-`Map`-on-fluent package | `b` | yes | no diagnostics |
| **6 method value / method expression** | — | **OUT OF SCOPE** | see below |

**Case 6 — coherent contract boundary (resolves the v1 §3-vs-App-C
contradiction).** §3 defines the vehicle over **call expressions**
(`recv.Map(...)`). Method *values* (`f := s.Map`) and method *expressions*
(`slice.Mapper[int].Map`) are non-call forms and are **out of the vehicle's
scope by contract** — not a silent gap and not a deferral. The v1 divergence
(App C had listed case 6 as "flag per the frozen table" while §3 scoped to
calls) is reconciled: both §3 and App C now agree the target is call
expressions. Recorded via `/variance` on tasks.jeeves.

## Discrimination result (integrity gate)

`go test ./experiments/causalarm91119/...` (direct nix-store go — `bin/go` hangs
headless, memory `d5f2ea188b30`):

- `TestOracle_CorrectPasses` — the type-aware reference satisfies every fixture
  marker exactly, across the positive/negative package `a` AND the clean package
  `b` (positive control passes; zero analysistest errors).
- `TestOracle_BrokenFails` — the name-only reference emits the same messages on
  the true positives but false-positives on the negative controls (2/4b/5b);
  the test requires at least one **"unexpected diagnostic"** error, so the
  failure is attributable specifically to the name-vs-type over-flag — not an
  unrelated mismatch (finding F6).

Because the correct reference passes the same fixtures with zero errors, the
fixtures are well-formed and the broken reference's failure is exactly the
false-positive path — the part-(a) blind spot this arm exists to catch.

## Delegate-dispatch warning (#93569 §6) — pre-oracle base, not "exclude dir"

This directory is the oracle's **ground truth** and MUST NOT be visible to the
dispatched delegates. "Exclude this directory from a post-oracle checkout" is
**insufficient** — the files persist in git history and any remote. The frozen
base commit for delegate worktrees must be a **PRE-ORACLE commit** (an ancestor
of the commit that introduced this directory), with the oracle mounted
independently at scoring time. Verify with
`git merge-base --is-ancestor <delegate-base> <oracle-commit>` before dispatch.
