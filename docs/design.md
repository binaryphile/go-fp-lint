# go-fp-lint: Design

Originating task: jeeves #62380 — "Build a Go linter enforcing fluentfp +
FP + go-dev conventions (parallel of shellcheck-convention-plugin for
bash)".

## Design questions resolved this cycle

**golangci-lint plugin vs. standalone `go/analysis` binary.** Chose
standalone. `golangci-lint`'s custom-analyzer story requires either their
module-plugin API (Go plugin build mode — real version-coupling risk
between golangci-lint's build and the plugin's) or contributing directly
upstream. A standalone `golang.org/x/tools/go/analysis`-based
`multichecker` is simpler, has no version-coupling to a host linter's
release cadence, and is directly usable two ways: run standalone, or drop
into `go vet -vettool=<binary>` for any existing `go vet`-based workflow.
This mirrors the shellcheck-convention-plugin precedent structurally — a
narrow, purpose-built plugin loaded by `--plugin-dir`, not a shellcheck
fork — even though the underlying mechanism differs (Haskell dylib vs. Go
analysis.Analyzer).

**Prior art check.** `~/projects/fluentfp/analysis.md` and `comparison.md`
are conceptual/motivational documents (why fluentfp reduces complexity),
not linter tooling. No existing golangci-lint or go/analysis usage exists
anywhere in this ecosystem (`grep -rl golangci-lint ~/projects/*/go.*` —
zero hits) — this is genuinely greenfield.

**Scope for this cycle.** The originating task lists ~10 distinct checks
across 4 guide areas (fluentfp chain-shape, go-dev value-semantics,
FP calculations-vs-actions, option.Basic/option.Option drift). Attempting
all of them in one cycle risks shipping several half-verified checks
instead of one solid one. Shipped exactly **one** analyzer this cycle,
fully tested and verified against real code; the rest are tracked as
explicit follow-up tasks (see §Roster below), per "working partial over
broken full."

## v1: `filterloop`

Detects the classic fluentfp-guide filter-loop shape:

```go
var active []User
for _, u := range users {
    if u.Active {
        active = append(active, u)
    }
}
```

flagged as: *"for-loop filter shape — use `slice.From(xs).KeepIf(predicate)`
instead"*.

**Detection rule (guard-if shape)**: a `for range` loop whose body is
exactly one `if` statement (no `else`, no `init`), whose body is exactly
one assignment `acc = append(acc, ...)` where `acc` is the same identifier
on both sides.

### v1.1: continue-guard shape (jeeves #65780)

The early-`continue` equivalent of the guard-if shape is now also
detected — same diagnostic, same KeepIf rewrite:

```go
var active []User
for _, u := range users {
    if !u.Active {
        continue
    }
    active = append(active, u)
}
```

**Detection rule (continue-guard shape)**: a `for range` loop whose body
is exactly **two** statements — first an `if` (no `else`, no `init`) whose
body is exactly one **unlabeled** `continue`, second an assignment
`acc = append(acc, ...)` with `acc` the same identifier on both sides. The
two shapes share the same `acc = append(acc, ...)` tail check
(`appendAccIdent`); `matchFilterLoop` reports a match when either
`ifGuardFilter` or `continueGuardFilter` holds.

**Deliberately NOT flagged for the continue-guard shape** (verified via
`testdata/src/a/a.go` fixtures):

- **Side effect before the continue** (`if !cond { println(...); continue }`)
  — the guard `if` body is then two statements; removing it in a KeepIf
  rewrite would silently drop the side effect. Parallel to the
  multi-statement guard-if negative.
- **Labeled continue** (`continue outer`) — targets an enclosing loop, so
  the loop is not a simple filter of its own range.
- **Continue followed by a non-append** (e.g. `count += 1` reduction) — not
  a slice-accumulator filter; a Fold/count, not a KeepIf.

**Deliberately NOT flagged** (verified via `testdata/src/a/a.go` fixtures):

- `if`/`else` filter-partition shapes (splits into two accumulators — not
  a simple `KeepIf`, would need a different fluentfp idiom or two passes)
- Multi-statement `if` bodies (e.g., a filter step plus a side-effecting
  log line) — conflating removal of the log statement with a KeepIf
  rewrite is a larger, riskier transform than this tool should suggest
  silently
- Map shapes (`for _, u := range users { names = append(names, u.Name) }`
  — no `if` guard at all) — this is `slice.From(xs).ToString(...)`, a
  different fluentfp method; a separate analyzer, not this one

**Resolved limitation (was a false-negative)**: filter shapes using an
early `continue` instead of a guarding `if` are now detected as of
jeeves #65780 — see §v1.1 above.

**Remaining known limitation**: the continue-guard detection requires
`Init == nil` on the guard `if`, so a filter with an init clause
(`if v, ok := seen[u]; ok { continue }`) is not flagged. This keeps the
shape tight and parallel to the guard-if rule; extending to init-form
guards is a possible future increment, not filed as a task.

## Roster (this cycle: 1 shipped, rest deferred/tracked)

| Check | Guide | Status |
|---|---|---|
| `filterloop` — for-loop filter shape → `KeepIf` | fluentfp-guide.md | **Shipped v1** |
| continue-guarded filter shape | fluentfp-guide.md | **Shipped v1.1** (jeeves #65780) |
| map-loop shape → `Convert`/`ToString`/etc. | fluentfp-guide.md | Deferred — no task filed yet |
| inline lambda inside fluentfp chain → named function/method ref | fluentfp-guide.md | Deferred — no task filed yet |
| nested `slice.X(slice.Y(...))` paren-depth violation | fluentfp-guide.md | Deferred — no task filed yet |
| pointer receiver where value receiver would work | go-development-guide.md | Deferred — no task filed yet |
| internal mock detection (design smell) | go-development-guide.md | Deferred — no task filed yet |
| slice/map field mutation without `Clone()` | go-development-guide.md | Deferred — no task filed yet |
| hidden actions in ostensibly-pure functions | functional-programming-unified-guide.md | Feasibility resolved by split (jeeves #65787) — direct-call + package-var-touch detection **Shipped v2** (jeeves #65900, `impuresource`; see §v2 below); transitive propagation tracked as jeeves #65901 |
| `option.Basic`/`option.Option` API drift check | fluentfp API vs go-development-guide.md | Deferred — no task filed yet |

Each deferred item needs its own `evtctl task` filed against `tasks.jeeves`
before pickup (not done in this cycle — see the closeout for this arc).

### Feasibility resolution: hidden actions in ostensibly-pure functions (jeeves #65787)

**Tractable, scoped to syntactically direct references** (jeeves #65900):
`pass.TypesInfo.Uses`/`Defs` resolves an identifier or a call's callee to
its `types.Object`, which is stable across import aliases and
dot-imports. This detects two things, each narrower than it may first
sound:

- **Direct calls** written as `pkg.Func(...)` (or via an alias/dot-import)
  where `Func` matches an allowlist. Initial list: `time.Now`,
  `os.Getenv`; the list is **allowlist-defined and intentionally
  incomplete** — `time.Since`, `time.After`, `os.LookupEnv`,
  `crypto/rand`, filesystem/network calls, logging, sync/channel ops, and
  mutable globals in *dependency* packages are deliberately out of scope
  for #65900's first cut and left as follow-up list-expansion, not a
  blocker.
- **Direct package-scope-var touches**, own package only for v1 (imported
  packages' exported globals are a stated non-goal — flagging every read
  of another package's var produces noise without a scope story yet).
  Object resolution alone only proves an identifier denotes a
  package-scope `*types.Var`; distinguishing read vs write vs
  address-taken vs compound-assignment requires inspecting the AST use
  context (parent node — is the identifier the LHS of an assignment,
  operand of `&`, etc.). #65900's implementation must specify this
  classification explicitly; it does not fall out of `TypesInfo` for
  free.

**Explicitly out of scope for #65900** (a stated boundary, not an
oversight): function-value indirection (`now := time.Now; now()`),
callbacks/higher-order dispatch, interface-mediated calls, and variables
initialized from an impure call's result. These need SSA/dataflow
analysis — natural territory for #65901's callgraph machinery to extend
into later, not something #65900 attempts.

Given these limits, #65900 is a **direct-impurity-source detector /
action inventory**, not a "hidden action" or "purity violation" detector
— it reports "this function directly calls X" or "directly touches
package-var Y," and nothing stronger. It cannot establish that a call was
*hidden* or violated an intended-Calculation contract, since (below) no
such contract is declared anywhere in the corpus. Diagnostics are worded
accordingly ("direct call to os.Getenv," not "this Calculation is
impure").

**Sequenced after, not hard-blocked by, #65900** (jeeves #65901,
transitive/callgraph propagation — Normand: "actions are infectious"):
`go/analysis/passes/buildssa` + `go/callgraph` are already available
transitively via the existing `golang.org/x/tools` dependency, no new
deps needed. #65901 needs a seed set of impure functions to propagate
from; #65900's allowlist-matching is a natural, reusable seed source, so
building #65901 second avoids duplicating that logic — but #65901 does
not technically *require* #65900 to have shipped, since a minimal seed
list could be inlined directly. Filed as **Deferred (tracked)**, pure
discretionary, sequenced-after #65900.

**Not tractable under the current corpus/contracts**: "should have been
marked pure but wasn't." Grepped `go-development-guide.md`,
`fluentfp-guide.md`, and `fluentfp-conversion-guide.md` for any
naming/comment/directive convention that declares a function's intended
purity — none exists. This is not a claim that the question is
undecidable in principle — a future annotation, naming contract, or
generated manifest could supply the missing oracle — only that no such
oracle exists *today* to check against. Rejected (not deferred)
until/unless the guide corpus adopts one; introducing that convention is
a guide-authoring design decision, out of go-fp-lint's scope.

Also rejected: a `Calculate*`/`Compute*` naming-heuristic proxy — not for
being infeasible to implement (it's trivially implementable), but because
it would manufacture an unwritten policy inside the linter rather than
check a documented one. The applicable principle isn't filterloop's
"no silent transform on ambiguous shapes" (that precedent concerns
automatic rewrites, not diagnostics) — it's more directly: **do not
emit normative diagnostics derived from an undocumented intent
heuristic**, since a user disputing the finding has no contract to point
to.

## v2: `impuresource` (jeeves #65900)

Ships the direct-detection half of the Feasibility resolution above: one
combined `go/analysis.Analyzer` reporting (a) direct calls to an
allowlisted impure-func set and (b) classified touches of the analyzed
package's own package-scope vars. Both checks resolve identifiers via
`pass.TypesInfo.Uses` — stable across import aliases and dot-imports — and
stay within the "action inventory, not hidden-action detector" framing
established above: diagnostics report observed syntactic facts ("direct
call to os.Getenv," "write to package-scope var X"), never intentionality
or contract violation.

**Direct-call detection** (`matchImpureCall`): resolves `call.Fun` to a
`*types.Func`, requires `sig.Recv() == nil` to exclude methods, and looks
up `(obj.Pkg().Path(), obj.Name())` against the allowlist below. Keying by
import path (not display name) keeps matching stable across aliases and
dot-imports; the diagnostic displays the short package name
(`obj.Pkg().Name()`) for readability. Only matches calls whose callee
resolves to a `*types.Func` — a function-valued variable
(`f := time.Now; f()`) resolves to a `*types.Var` instead and is out of
scope by construction, not a missed case of "every syntactically direct
call" (confirmed via cross-vendor `/grade` R1 probe 1).

Allowlist v1 (same intentional-incompleteness posture as the Feasibility
resolution above):

| Import path | Func |
|---|---|
| `time` | `Now` |
| `os` | `Getenv` |

**Package-var-touch detection** (`matchPackageVarTouch` + `classifyUse`):
identifies package-scope vars of the analyzed package via
`obj.Pkg() == pass.Pkg && obj.Parent() == pass.Pkg.Scope()` (own package
only — an imported package's exported vars are out of scope, verified by
a cross-package negative fixture), then classifies each use by its
**immediate** AST-ancestor node (a manual stack maintained over
`ast.Inspect`'s push/pop callback):

| Immediate parent | Classification |
|---|---|
| `*ast.UnaryExpr` with `Op == token.AND` | address-of |
| `*ast.IncDecStmt` | compound-assign |
| `*ast.AssignStmt`, compound-assign token (`+=`, etc.), ident in `Lhs` | compound-assign |
| `*ast.AssignStmt`, `Tok == token.ASSIGN`, ident in `Lhs` | write |
| anything else (Rhs, call arg, selector/index base, condition, ...) | read (default) |

**Documented limitation — selector-chain boundary**: classification only
inspects the identifier's *immediate* parent, not deeper into a selector
chain. `globalConfig.Name = "x"` classifies the base identifier
`globalConfig` as **read of**, not a field-level write, because its
immediate parent is the `*ast.SelectorExpr`, not the enclosing
`*ast.AssignStmt`. This is a stated boundary (tested by a fixture, see
§Verification below), not a silent mis-label — deeper selector-chain
write-precision is a possible future increment, not filed as a task.

**Explicitly out of scope for v2** (same boundaries the Feasibility
resolution already drew): function-value indirection, callbacks,
interface-mediated calls, impure-result-derived vars (#65901's
SSA/dataflow territory); imported-package exported-var touches (own-package
only, stated non-goal); a `Calculate*`/`Compute*` naming heuristic (already
rejected above); CLI-configurable allowlist (zero-config, matches
`filterloop`'s precedent — extending the allowlist is a one-line code
edit).

**Relationship to the `//fp:calc` marker proposal (jeeves #66086)**: a
design proposal to add a machine-read purity-marker convention surfaced
mid-cycle (would upgrade this analyzer's diagnostics from an informational
action-inventory into a normative purity-contract check for marked
functions). Deliberately NOT adopted this cycle — the guide-side
convention doesn't exist yet, and a first draft of it was independently
sent back on cross-vendor grade as unsound in the higher-order fluentfp
domain (a marker on a combinator can't establish its function-value/
interface/generic callbacks are pure without an effect system). #65900
ships in its informational form; #66086 is now scoped as a further-out
"effect-lite design" cycle (define the purity boundary + first-order
subset + conditionally-pure combinator effect signatures), not a
near-term guide convention. If #66086 ever lands, note the forward
migration risk it flagged: any future rewording of these diagnostics from
"direct call to X" to a contract-violation phrasing is an observable
compatibility change for anything that scrapes diagnostic text rather than
analyzer IDs or structured (`-json`) output.

## Integration points (documented, not wired up this cycle)

Per the originating task: pre-commit hook, `/c` skill invocation, tandem
gate-bash (`<linter> ./... || exit 1` on Standard-tier cycles touching
`*.go`), author-time IDE/LSP integration. None of these are wired up
yet — v1 ships the binary + one analyzer only. Wiring these up is
follow-up scope, likely per-integration-point tasks rather than one
umbrella task (each has a different owner/repo: pre-commit hooks live
per-repo, `/c` skill lives in `~/.claude/skills/`, gate-bash is a
per-cycle plan-file convention).

## Verification performed (v1 cycle)

> v1.1 (continue-guard shape, jeeves #65780) added 3 fixtures — a positive
> continue-guard case plus 2 adjacent negatives (labeled continue,
> continue-then-reduce) and a side-effect-before-continue negative; the
> golden-fixture suite is now 9 cases (2 positive, 7 negative). Same
> `go test ./...` + `go vet ./...` + built-binary smoke-test gates, all
> green. See §v1.1.
>
> v2 (`impuresource`, jeeves #65900) shipped as a second analyzer — see
> below.

- `go test ./...` — analysistest golden-fixture suite, 6 cases (1 positive
  match, 5 negative — else-partition, multi-statement body, sum-reduction,
  map-shape, clean pass) all correct.
- `go vet ./...` — zero findings on the repo's own code.
- Built binary smoke-tested against a real filter-shape snippet — correct
  diagnostic at the correct line.
- `nix flake check` / standalone `nix develop` were NOT verified to
  complete this cycle (hit a nix evaluation timeout unrelated to this
  repo's flake.nix content — the same `nixpkgs-unstable` pin pattern used
  by `era`'s and `fluentfp`'s already-working flakes). Development and
  verification this cycle used `nix develop "path:$HOME/projects/fluentfp"`
  (already-cached nixpkgs) as a workaround. Follow-up: verify this repo's
  own `nix develop` completes cleanly once the evaluation slowdown is
  understood — may just need a first fetch to complete outside a
  time-constrained session.

## Verification performed (v2 cycle — `impuresource`, jeeves #65900)

TDD red/green (khorikov-unit-testing-guide.md §9, required for new
behavioral surface): `impuresource_test.go` + all `testdata/src/a/*.go`
and `testdata/src/b/b.go` fixtures were written and confirmed failing
(`impuresource` package didn't exist yet) BEFORE `impuresource.go` was
implemented.

- `go test ./impuresource/...` — analysistest golden-fixture suite, 16
  cases (9 positive: 4 call-detection shapes covering regular/aliased/
  dot-imports, 5 var-touch classifications covering all four verbs;
  7 negative: method-shadow, name-shadow, local-scope, cross-package,
  allowlist-miss, selector-chain-boundary, const-vs-var) all correct.
- `go test ./...` — full repo suite green, `filterloop` unaffected.
- `go vet ./...` — zero findings on the repo's own code.
- Built binary (`multichecker.Main(filterloop.Analyzer, impuresource.Analyzer)`)
  smoke-tested against a real snippet containing a filter-loop shape, a
  package-var increment, and an `os.Getenv` call — all three diagnostics
  fired at the correct lines, both analyzers running together correctly.
- Two rounds of cross-vendor `/grade` (R1: A-, APPROVE; R2: A, APPROVE) —
  findings absorbed: narrowed "direct call" wording to "resolved function
  calls" (not "every syntactically direct call"), added the const-vs-var
  negative fixture, and scoped the cross-session coordination claim to
  "discharges this session's side" rather than implying conflict
  prevention.
