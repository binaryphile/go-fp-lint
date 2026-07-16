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

## Enforcement tiers and the guide-division model

The roster below was first conceived as a flat list of linter checks.
Investigation jeeves #65931 (2026-07-11; era memories `e6372253f9f8`,
`4e1d8c9ad4a8`, grade-absorption corrections `75a0de9ced0d`) reframed it: each
convention belongs to one of three enforcement tiers, decided by properties of
its **fix**, not its detection (detection is always mechanical AST-matching).

**The fault line.** A rewrite can be always-on and automatic only if it is
deterministic, semantics-preserving, and requires **no semantic planning beyond
local syntax**. Synthesizing a binding name is the most common disqualifier
(gofmt never manufactures identifiers; `gofmt -r` and `eg` only substitute
existing syntax; `SuggestedFix` is raw `TextEdit`s with no scope model), but not
the only one — evaluation-order and aliasing hazards also require planning
beyond local syntax.

The three tiers:

- **Tier A — Format.** Deterministic, name-free, semantics-preserving, safe to
  apply on every save. Vehicle: a formatter pass (gofmt-class).
- **Tier B — Codemod.** Mechanical detection, but the fix synthesizes a name or
  needs contextual planning. Vehicle: an offered / `-fix` codemod, not
  always-on. Where the fix needs a name, the codemod hoists to a `change_me_N`
  placeholder and an *optional* downstream naming pass (LLM or human) fills it.
- **Tier C — Lint.** Detectable, but the fix needs a judgment the tool can't
  make, the rule carries a stated exception, or the property is statically
  undecidable. Vehicle: a diagnostic (`impuresource` §v2 is the first shipped
  Tier-C check).

**The guide-division model (and its limits).** Because a formatter/codemod
owning a rule removes that rule's three standing costs — guide context (guides
are force-read every session), authoring effort, and rework loops — the tier
partition doubles as a **guide-reduction map**: a rule's mechanical
*specification* can leave the guide once a tool owns it. Two constraints keep
this honest:

- **Specification ≠ rationale.** A guide also teaches. Shedding a rule's
  mechanical "how" does not license deleting its conceptual "why" — the
  rationale stays (condensed) for onboarding. The guide shrinks *modestly*, not
  to zero. (grade #65931 R1 X1.)
- **Shrink lags ship.** A rule leaves the guide only *after* a tool enforces it
  (else the convention rots un-enforced), and the tool must run automatically in
  the workflow (pre-commit / gate-bash / on-save) for the savings to be real.

**The payoff is back-loaded.** Tier A is small — essentially one whitespace
family (chain line-layout). The larger guide-shrink lives in Tier B, whose
primary value is **one-time bulk migration** of existing code, not steady-state
authoring: in steady state, a Tier-C diagnostic plus a human-named `gopls`
extract may beat a codemod whose LLM-generated names still need review (the cost
is relocated, not eliminated). The optional LLM naming pass is gated on a
demonstrated migration need, not an assumed component. (grade #65931 R1
P1/P2/P7.)

**Codemod transformation contract (Tier B, name-synthesizing fixes).**
Extraction-to-intermediate is applied only where provably safe: statement
position (assignment RHS / `return` / expr-statement) **minus** disqualifiers —
inside `defer`/`go` argument construction, free-variable capture that changes
evaluation timing, a subexpression not provably evaluated exactly once, or an
aliasing-sensitive receiver. Everywhere else the codemod flags rather than
rewrites. Statement position is a hazard-reducing heuristic, **not** a proof of
semantic preservation. (grade #65931 R1 P3, blocking.)

**Detection is the shared foundation.** A flag, a `SuggestedFix`, and a codemod
all need the identical detector, so a check's detector is a no-regrets build
regardless of which tier ultimately delivers it. This is why the paren-depth /
uniform-commas detector (#65783) proceeds as a Tier-C diagnostic, with the
Tier-B `change_me` fix layered on later.

**Purity checking is Tier C, and normativity is deferred.** `impuresource`
(§v2) ships as an *informational* action-inventory — it reports "function
directly calls `os.Getenv`," not "this Calculation is impure," because no
purity-declaration convention exists in the corpus. A normative upgrade needs a
declared-purity marker; the naive `//fp:calc` comment-marker was found unsound
in this higher-order library (grade #66086 R1, D+; era `bab8e36b72eb`) — the
viable form is a bounded effect-system (define the Calculation purity boundary;
a first-order subset; conditionally-pure combinator effect signatures), tracked
as the deferred design **#66155**, gating enforcement **#66086**.

**Generality (open, N=1).** The format/lint *split* is well-precedented — bash's
`shfmt` + shellcheck-convention-plugin is the same division, and is why
go-fp-lint was modeled on shellcheck. The specific three-tier
format/codemod/naming architecture is, so far, an N=1 fluentfp hypothesis.
(grade #65931 R1 P6.)

## Roster (tiered)

Tier per the model above. "C→B" = ships now as a Tier-C detector (the shared
foundation); a Tier-B codemod fix may be layered on later.

| Check | Guide(s) | Tier | Status / task |
|---|---|---|---|
| `filterloop` — filter shape → `KeepIf` | fluentfp | C | **Shipped v1** |
| continue-guard filter shape | fluentfp | C | **Shipped v1.1** (#65780) |
| `impuresource` — direct impure-call + package-var touch | fp-unified | C | **Shipped v2** (#65900; §v2). Informational inventory; normative upgrade deferred (#66086 ← #66155). Transitive: **Shipped v2.1** (#65901; §v2.1) |
| chain line-layout (one-op-per-line / inline) | all three | **A** | detector **Shipped v8** (`chainlayout`, #66031; setup-constructor-rooted, types-resolved — see §"Tier-A spec: chain line-layout"). Rewriting `SuggestedFix` + guide-shrink deferred (arc #71278→#71279→#71280); var/return-rooted #71302 |
| method-expression (`func(x T) R { return x.M() }` → `T.M`) | fluentfp / fp-unified | **B** | codemod, name-free — #66032 |
| paren-depth + uniform-commas | fluentfp / go-dev | **C→B** | detector **Shipped** (`nestedcall`, #65783); `change_me` fix deferred **#66034** |
| double-map fusion → composed pass | fluentfp / fp-unified | **C→B** | detector task **#66830** (split out of #65783 at plan time — distinct violation condition, not a paren-depth/uniform-commas variant) + #66034 |
| map-loop → `Transform`/`ToXxx`/`Map` | fluentfp / go-dev | C | detector **Shipped** (`mapshape`, #65781) |
| inline lambda → named function (residual, non-method-expr) | fluentfp / go-dev | C | **Shipped v7** (`chainlambda`, #65782; type-resolved fluentfp receiver, see §v7) |
| pointer receiver where value receiver works | go-dev | C | **Shipped v5** (#65784; overlap with `go vet copylocks` resolved via ported `lockPath`, see §v5) |
| internal mock detection (design smell) | go-dev | C | **#65785** |
| slice/map field mutation without `Clone()` | go-dev | C | **Shipped v6** (`aliaswrite`, #65786; aliasing undecidable — tight conservative scope, see §v6) |
| `option.Basic` / `option.Option` API drift | fluentfp / go-dev | C (rename → B) | no analyzer task yet |

Deferred / optional (tracked): `change_me` extraction substrate **#66034**, LLM
naming pass **#66036** (gated on a demonstrated migration need), guide-shrink
umbrella **#66033** (gated per-tier on the owning tool shipping), effect-lite
purity design **#66155** (gates enforcement #66086), this design.md write
**#66161**.

**Overlap discipline.** Several conventions surveyed in #65931 are already
enforced by existing tooling (`copylocks`, `errorlint`, `copyloopvar`,
`thelper`, staticcheck `ST10xx`, `go test` example validation). go-fp-lint does
**not** reimplement these — it scopes to the fluentfp/FP-specific rules no
existing linter covers. **Exception, not a violation**: `recvshape` (§v5)
ports `copylocks`' unexported `lockPath` *exclusion* algorithm to stay
non-contradictory with it — this reuses `copylocks`' own lock-detection
logic as a supporting check, it does not reimplement `copylocks`' flagging
behavior (value-receiver-on-lock-type), which remains solely `copylocks`' job.

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

## Tier-A spec: chain line-layout (`chainlayout`, #66031)

**Shipped v8** (jeeves #66031). This section specifies the rule the `chainlayout`
pass enforces. The authoritative WHAT lives in `fluentfp-guide.md §"Chain
Formatting"`; per the guide-division doctrine above (**shrink lags ship**) the
rule stays in the guide until the pass runs automatically in-workflow — that
shrink is the tail of the sequenced arc (jeeves #71278 install → #71279 wire →
#71280 shrink), NOT this cycle.

**v1 enforceable claim (scope).** chainlayout enforces layout **only for chains
rooted at an inline, qualifying fluentfp setup constructor**. Variable-rooted
(`m := slice.From(xs); m.A().B()`), function-return-rooted (`getM().A().B()`), and
dot-imported (`import . ".../slice"`) chains are **out of the v1 claim** (tracked
jeeves #71302) — import spelling is load-bearing despite the types-resolved
identity. Detector only; an always-on rewriting `SuggestedFix` is a compatible
later layer, not shipped here.

**What the rule governs.** A *fluent chain* is a value produced by a fluentfp
setup constructor (`slice.From(...)`, `slice.Map[R](...)`, `option.Of(...)`, and
siblings) followed by one or more chained method calls on the resulting fluent
value. The rule constrains only the *line layout* of such a chain — it is
deterministic, name-free, and semantics-preserving (Tier A).

**Counted operations.** Only the chained method calls count. The setup
constructor (`slice.From`, `slice.Map[R]`, `option.Of`, …) is a non-counted
bookend — it establishes the fluent value but is not itself a chained operation.
Every method call *after* setup counts, **including a terminal `ToX` / `Len` /
etc. call** — the terminal call is NOT exempt (in the single-operation example
below, the one counted op *is* the terminal `.ToString(...)`).

**Two layout forms:**

- **One counted operation → inline.** The whole chain on a single line:

  ```go
  names := slice.From(users).ToString(User.Name)
  ```

- **Two or more counted operations → one per line, trailing dot.** The setup
  call keeps the first `.` as a trailing dot; each counted method call sits on
  its own line, indented one level deeper than the statement, and every line
  ends in the trailing `.` except the terminal call:

  ```go
  count := slice.From(tickets).
      KeepIf(completedAfterCutoff).
      Len()
  ```

  Indentation is gofmt's (tabs); the rule fixes chain *structure* (line breaks +
  trailing-dot placement), not column counts — the result is run through gofmt.

**A violation** is any fluent chain whose actual line layout does not match the
form its counted-operation count selects: a two-plus-op chain written inline (or
split with leading rather than trailing dots), or a single-op chain split across
lines.

**Realization (mirrors the sibling passes).** A `go/analysis` detector in
`chainlayout/`, mirroring `filterloop/`, `mapshape/`, `recvshape/`: one
`chainlayout.go` + `testdata/src/...` fixtures carrying `// want` comments + a
thin `chainlayout_test.go` driving `analysistest.Run`. It *reports* violating
chains (diagnostic). Key decisions (earned over the R1–R3 adversarial grade):

- **Chain identity is types-resolved.** `walkChain` walks the right-nested
  `CallExpr` receiver spine; `calleeSelector` unwraps `*ast.ParenExpr` /
  `*ast.IndexExpr` / `*ast.IndexListExpr` so a **generic instantiated** setup
  constructor (`slice.Map[R](xs)`) is not silently skipped (R1 F5). A method
  (`Signature().Recv()!=nil`) is a counted op; a package func is the setup
  bookend **only if it has exactly one result whose type — after
  `types.Unalias` + pointer-strip — is a `*types.Named` defined under fluentfp**
  (R1 F6 / R2 F2), nil-guarding `Obj().Pkg()` against universe types. Package
  membership is the org-qualified `isFluentfpPath` predicate (segment-exact,
  shared with the method-identity check).
- **Layout metric = the source line of each method-name identifier**
  (`Fset.Position(sel.Sel.Pos()).Line`), NOT the call's `End()` — so a single-op
  chain whose argument is a multi-line lambda is not a false split. One op →
  `setupLine == opLine`; ≥2 ops → `[setupLine, opLines...]` strictly increasing.

An always-on rewriting `SuggestedFix` (gofmt-class formatter behavior) is a
compatible later layer, not shipped here. The detector yields an independently
re-runnable oracle: `go test ./chainlayout/...`.

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
| `*ast.RangeStmt`, `Tok == token.ASSIGN`, ident is `Key` or `Value` | write |
| anything else (Rhs, call arg, selector/index base, condition, ...) | read (default) |

**Found during 3b `/i` self-review**: the assign-form range clause
(`for globalCount = range xs`, reusing an existing package var rather than
`for globalCount := range xs` declaring a new local one) writes to the var
on every iteration, but its immediate parent is an `*ast.RangeStmt`, not an
`*ast.AssignStmt` — the original classifier had no case for it and
silently mis-classified it as a read. Added the `*ast.RangeStmt` row above
plus a positive fixture (assign-form) and an adjacent negative
(declare-form, which shadows via `Defs` and never reaches the classifier
at all) to lock in the fix. A multi-value-assignment fixture
(`globalCount, err = 5, nil`) was also added as regression coverage for
already-correct behavior (`identInList` checks list membership, not
position).

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

## v2.1: `impurereach` (jeeves #65901)

Fourth analyzer, new package `impurereach/`. Closes the gap #65900 punted:
"function-value indirection, callbacks, interface-mediated calls ... natural
territory for #65901's callgraph machinery" (§Feasibility resolution above).
Normand: "actions are infectious" — a function that doesn't itself directly
call an impure func, but *transitively* reaches one through local calls, is
flagged; a separate, honestly-worded diagnostic covers calls that can't be
resolved statically at all, rather than silently under-reporting or
over-claiming impurity through them.

**Scope boundaries (1b decisions, all recommended options)**:

- **Intra-package only.** `buildssa.Analyzer` builds a *per-package*
  `*ssa.Program` — imported packages are unbuilt stubs (no `Blocks`), so
  `go/callgraph/static.CallGraph(prog)` structurally can't produce edges
  through another package's function bodies. Cross-package propagation
  needs `analysis.Fact` export/import — deferred, tracked as **#66810**.
- **Dynamic dispatch gets a separate honest diagnostic**, not silence and
  not conservative "assume impure." `go/ssa.CallCommon.StaticCallee()`
  resolves direct calls and immediately-invoked closures (`*ssa.MakeClosure`
  case); everything else (stored/passed closures, interface `invoke`-mode
  calls) returns `nil` and gets `"call via function value — purity cannot
  be determined"` or `"interface-dispatched call — purity cannot be
  determined"`, independent of the transitive-seed set.
- **Seed set is #65900's direct-impure-call allowlist only** (not also
  package-var writes) — reused via export (`impuresource.ImpureFuncs`,
  renamed from the unexported `impureFuncs`) rather than duplicated,
  avoiding allowlist drift between the two analyzers.

Value-derived-var taint tracking (`x := os.Getenv("X"); foo(x)`) is a
materially different (dataflow/taint, not call-graph) mechanism, explicitly
NOT attempted — deferred, tracked as **#66811**.

**Algorithm**: `static.CallGraph(ssaInfo.Pkg.Prog)` builds the local call
graph; seeds are nodes whose `Func.Object()` matches the allowlist (same
`Recv() == nil` guard as `matchImpureCall`). A multi-source BFS from all
seeds walks reverse edges (`Node.In`), tracking hop depth — cycles/recursion
are handled for free by the visited-once BFS property, no separate SCC pass
needed. **Depth ≥ 2 only**: depth-1 nodes are *direct* callers of the seed,
already reported by `impuresource`'s direct-call diagnostic — reporting them
again here would be duplicate and inaccurately worded ("transitively" when
it's actually direct). Reports once per named source function at
`Func.Pos()`; anonymous closures are never report *targets* (no clean
name/position) but still propagate reachability correctly through the
graph — an IIFE that directly calls an impure func makes its *enclosing
named function* transitively impure (2-hop through the
`MakeClosure`+immediate-`Call` static edge).

**Found during 3b `/i` self-review**: `go/callgraph/static`'s `methodsOf()`
synthesizes compiler-generated pointer-receiver wrapper methods (`(*T).M`
calling through to the real value-receiver `T.M`) for every package-level
non-interface type. These wrappers create a real static-call edge that was
producing a spurious extra hop — a value-receiver method that directly
calls an impure func (a depth-1 direct caller, correctly excluded) was
*also* being reported via its own auto-generated wrapper's depth-2 edge,
under the wrapper's shared source position (`ssa.Function.Synthetic`
wrappers share position with the function they wrap). Fixed by excluding
`Func.Synthetic != ""` from report targets.

**Absorbed from IMPL-stage `/grade` (finding 1, GAP)**: the initial fix
was justified only against the one pointer-wrapper category it was found
against, while the code excludes *every* synthetic function — a broader
mechanism than the demonstrated failure. Confirmed this is the correct
general policy **for every `Synthetic != ""` category present in the
inspected `x/tools/go/ssa` version** (pointer wrappers: `"from type
information"`; bound-method-value closures: `"bound method wrapper for
..."`; generic instantiations: `"instance of ..."`/`"instantiation
wrapper of ..."`) — none of these is code a user wrote, so none should
ever be a report target regardless of which category produced it. This is
a policy validated against the categories `go/ssa` generates today, not a
timeless semantic guarantee — a future `x/tools` release adding a new
synthetic category would inherit the same exclusion by construction
(anything with `Synthetic != ""`), but that's an inference from the
policy's shape, not something re-verified here. Added fixture #12
(`funcMethodValue`, a bound
method value `f := h.Leaf; f()`) as a second, structurally different
synthetic category, empirically confirming the general policy: the real
named caller is still correctly flagged (reachability propagates
*through* the synthetic bound-thunk hop), while the synthetic thunk
itself is correctly excluded from reporting.

**Absorbed from IMPL-stage `/grade` (finding 4, WEAKENS)**: the
"intra-package only" framing could read as the call graph itself being
scoped to one package by construction. Sharper mechanism: `static.
CallGraph`'s traversal loop does walk every package in
`prog.AllPackages()`, imports included — the boundary is enforced by
imported packages never being `Build()`'d (`buildssa` only builds the
analyzed package; imports are created via `CreatePackage(p, nil, nil,
true)`, so they have no `Blocks` and contribute zero `Out` edges), not by
the graph excluding them outright. Fixture #8 (cross-package) is the
empirical check for this; the mechanism is now also documented as a code
comment at the `static.CallGraph` call site.

**Absorbed from IMPL-stage `/grade` (finding 5, WEAKENS)**: "not a
purity question" for builtin calls slightly overclaimed exhaustiveness.
The `*ssa.Builtin` exclusion is verified empirically for `println`
(fixture #11), not proven to cover every builtin's SSA lowering shape. A
missed shape would at worst be a false-negative "indeterminate" report on
an already-out-of-scope, non-purity-relevant category — low severity,
now stated honestly in the code comment rather than implied as exhaustive.

**Considered, not changed (finding 6, WEAKENS — message wording)**: the
grader found `"actions are infectious"` project-specific flavor text that
adds less value than the lead clause. Left as-is: the wording mirrors
`impuresource`'s already-shipped precedent (`"direct call to X — actions
are not calculations (see ...)"`), and changing only this analyzer's
phrasing would create an inconsistency between two sibling diagnostics a
user will see side-by-side in the same tool run.

**Documented subtlety, not a bug**: a trivial non-branching closure
binding (`f := time.Now; f()`, no address-of, no reassignment) can resolve
`StaticCallee()` directly via SSA register-lifting — the indeterminate-call
diagnostic's true boundary is narrower than "any variable holding a func
value": only genuinely-unresolvable bindings (function parameters,
struct/slice/map storage, branch-merged values) hit it. This is inherent to
`StaticCallee()`'s definition (see `go/ssa`'s `CallCommon.StaticCallee`),
not a gap in this analyzer.

**Inherited limitations, not worked around**: generic package-level
functions are a stated limitation of `go/callgraph/static` itself (its own
doc comment excludes parameterized methods from `methodsOf`; plain generic
functions have no special handling either) — out of this analyzer's scope
to compensate for.

## v3: `nestedcall` (jeeves #65783)

Third analyzer, third package: `nestedcall/`. Detects two related
call-nesting readability violations from fluentfp-guide.md /
go-development-guide.md (duplicated verbatim in both — the rule is
general-purpose Go guidance, not fluentfp-specific, so the analyzer has no
import-gating, matching `filterloop`'s precedent of firing on shape alone):

- **Paren-depth**: don't open more than two parens without closing (chain
  depth via nested call-as-argument > 2).
- **Uniform-commas**: only one nesting level may have multiple
  (comma-separated) arguments.

One package, two diagnostics, shared `*ast.CallExpr` traversal (mirrors how
`filterloop` shares `appendAccIdent` across its two shapes) — this shape was
an explicit operator choice over two separate analyzer packages.

**Algorithm** (pure syntax, no type info): a pre-pass marks every CallExpr
that appears literally inside another CallExpr's `Args` slice (NOT its
`Fun`/receiver position — this is what correctly excludes method chains
like `results.Sort(...).Take(n)`, and by extension func-returning-func
shapes like `f()(x)`, from paren-depth counting). Paren-depth is then
evaluated only at "root" calls (not marked nested-as-arg), via
`depth(call) = 1 + max(depth(argCall) for CallExpr args, else 0)` — max, not
sum, across sibling arguments, since only one nested chain is ever
simultaneously open when reading left to right. Uniform-commas is evaluated
independently at every CallExpr (root or nested): violated when a call has
>1 arg AND at least one of its args is itself a CallExpr with >1 arg.

**Deliberate scope narrowing — adjacent-pair, not whole-chain
(`/grade r1` finding 2, jeeves #65783)**: the guide's prose ("only one level
may have multiple arguments") reads as a whole-chain invariant, but all
guide examples only exercise immediate parent/child pairs. This v1 ships
the **adjacent-pair** interpretation: `f(g(h(a, b)), c)` (where `f` and `h`
both have multiple args but their direct parent/child pairs don't) is NOT
flagged. This is an intentional v1 choice, not an accidental narrowing; a
whole-chain variant is a candidate follow-up if real-world false negatives
surface.

**Scope correction — double-map fusion split out**: `docs/design.md`'s
roster previously listed "double-map fusion" as in-scope for #65783's
detector; the originating task description and the prior session's
`/pickup` handoff both scoped #65783 to paren-depth + uniform-commas only.
Split to task **#66830** — confirmed at `/grade r1` finding 6 that the two
checks have genuinely distinct violation conditions (same-aggregate-op
composition vs. nesting-depth/comma-count metrics) and don't share
meaningful implementation beyond the generic CallExpr walk.

## v4: `mapshape` (jeeves #65781)

Fourth analyzer, fourth package: `mapshape/`. Detects the map-loop shape —
a for-range loop with no `if`-guard whose body is exactly one
`acc = append(acc, EXPR)`, where `EXPR` transforms the range value
(distinct from `filterloop`'s guard-if/continue-guard shapes, which this
detector structurally cannot match, and from a plain copy-loop where `EXPR`
is the bare range identifier — deliberately not flagged, nothing to
transform).

**Target-type classification (`pass.TypesInfo`)**: `T` = the range value's
type, `R` = `EXPR`'s type.

1. `types.Identical(T, R)` → same-type mapping → `slice.From(xs).Transform(fn)`.
2. `R` matches one of the ~10 fluentfp typed-alias targets (`bool`, `byte`,
   `error`, `float32`, `float64`, `int`, `int32`, `int64`, `string`, `any`)
   → the matching `slice.From(xs).ToXxx(fn)`. `error`/`any` are checked by
   exact identity against `types.Universe.Lookup("error"/"any").Type()`
   (NOT a structural "has one method"/"has zero methods" check) — this
   correctly excludes a user-declared named error-like type or a
   locally-declared empty interface from being misclassified as the
   builtin `error`/`any` (verified via the `Marker` — a local empty
   interface — testdata fixture, which correctly falls through to case 3
   instead of `ToAny`).
3. Otherwise (arbitrary struct/pointer/slice/etc.) → the standalone
   `slice.Map(xs, fn)` — a bare function call, NOT a `slice.From(xs).`
   chain continuation (`Map` isn't a `Mapper[T]` method).

**Known limitation, not solved**: `rune` and `int32` are the literal same
Go type (`rune` is a builtin alias for `int32`) — `go/types` cannot
distinguish which spelling the source used. This detector always resolves
to `ToInt32`, never `ToRune`. An inherent ambiguity in fluentfp's own API
surface, not a bug in this detector.

**Guide drift found this cycle (evidence for #65868, not fixed here)**:
the originating task description, and `go-development-guide.md` line 1334,
both describe a `Convert` method and a `slice.MapTo[R,T]`/`MapperTo[R,T]`
mechanism. Neither exists in the current fluentfp source, verified
directly against `~/projects/fluentfp/internal/base/mapper.go` and
`~/projects/fluentfp/slice/map.go`:

- The same-type method is **`Transform`** (`mapper.go:28`), not `Convert`
  — `Convert` doesn't exist. Operator confirmed live: "we use transform
  not convert any more."
- The arbitrary-target mechanism is the **standalone function**
  `slice.Map[T, R any](ts []T, fn func(T) R) Mapper[R]` (`slice/map.go:8`)
  — `MapTo`/`MapperTo` do not exist anywhere in `slice/` or
  `internal/base/` (zero grep hits).
- The ~10 known-alias `ToXxx` methods ARE confirmed present exactly as
  documented (`mapper.go` lines 383–483).

This detector's diagnostics name the **verified-correct** method
(`Transform`/`ToXxx`/`Map`), not the guide's stale wording. #65868 ("guide
fluentfp-API drift... systematic pass verifying every fluentfp module
table") already scopes a `slice` module audit — confirmed its scope
covers this finding; this section is the concrete evidence for whoever
picks that audit up, filed via `/grade r1` finding 6 discussion (jeeves
tasks.jeeves interaction 66952).

## v5: `recvshape` (jeeves #65784)

Fifth analyzer, fifth package: `recvshape/`. Detects pointer-receiver
methods that could be value receivers per go-development-guide.md §3 Value
Semantics — default to value receivers; pointer receivers are for
lock-containing types, interface-satisfaction consistency, or methods that
actually mutate the receiver's own fields.

**Partial overlap with `go vet copylocks` — resolved, not duplicated.**
`copylocks` (`golang.org/x/tools/go/analysis/passes/copylock`) flags the
*opposite* direction: a value-receiver method (or any value copy) on a type
containing a lock, via its unexported `lockPath` algorithm (does `*T`
implement `sync.Locker` while `T` does not, recursed through struct
fields/arrays; pointer and interface fields are deliberately NOT recursed
into — "safe to copy"). `recvshape` flags the reverse: an unnecessary
pointer receiver. Without an independent lock exclusion, it would
contradict `copylocks` — recommending a value receiver on a type where
pointer semantics are actually required. Since `lockPath` is
unexported/internal to `x/tools`, `recvshape` ports the algorithm
(`typeHasLock`/`lockPath` in `recvshape.go`) rather than importing it.

**Detection algorithm.** For each named struct type `T` with at least one
pointer-receiver method:

1. **Lock exclusion** (`typeHasLock`) — ported `lockPath`: recurse through
   struct fields (unwrapping arrays), checking
   `types.Implements(*fieldType, sync.Locker) && !types.Implements(fieldType, sync.Locker)`.
   If any field (or the type itself) qualifies, skip every method on `T`.
   Faithfully mirrors upstream, including NOT recursing through pointer
   fields — a `*innerLock` field is safe to copy (only the pointer is
   duplicated, not the lock), so a type reaching a lock only through a
   pointer field is correctly left un-excluded and its non-mutating
   pointer-receiver methods are legitimately flaggable (see
   `PtrLockHolder` fixture).
2. **Interface exclusion** (`typeSatisfiesInterface`) — skip `T` entirely
   if `*T` implements any *non-empty* interface type lexically referenced
   anywhere in the package's own files (`pass.TypesInfo.Types`, filtered to
   `Underlying().(*types.Interface)` with `NumMethods() > 0` — the empty
   interface is excluded since every type trivially implements it, which
   would otherwise disable the analyzer entirely).
3. **Mutation detection** (`mutatesReceiver`) — AST scan for: assignment
   whose LHS is a selector rooted at the receiver (matched via
   `pass.TypesInfo.Uses[ident] == recvObj`, object identity, not name-string
   matching); `IncDecStmt` on such a selector; whole-value pointer-deref
   assignment (`*t = ...`); or `&t.field` (address-of any receiver field)
   anywhere in the body — conservative, favors false-negative over
   false-positive. A blank/unnamed receiver has no identifier to trace
   mutation through, so it's always eligible to flag (subject to the
   type-level exclusions above).
4. **Type-level consistency exemption** — if ANY pointer-receiver method on
   `T` mutates, skip flagging every method on `T`, not just the mutator.
   Real code mixing receivers per-method on the same type is the
   anti-pattern the guide steers away from in the *other* direction; the
   checker doesn't fight consistency choices.
5. **Generics** — receiver types with non-empty `TypeParams()` are skipped
   entirely (v1 scope; see Known limitations).

**Known limitations (accepted v1 scope, not solved):**

- **Cross-package-only interface conformance.** A type satisfying
  `io.Writer` etc. with no lexical mention of that interface anywhere in
  its own package is not detected — same undecidability class as general
  interface satisfaction.
- **Call-site-only interface polymorphism.** `pass.TypesInfo.Types` only
  records the static type of expressions actually *written* in the
  package. A bare call site like `sort.Sort(t)` never writes
  `sort.Interface` as a type expression anywhere — only `t`'s own concrete
  type is recorded at that call — so it is NOT caught by the interface
  exclusion. Concretely: a type satisfying `fmt.Stringer` purely through
  `%v`/`%s` formatting call sites, with no local `fmt.Stringer`-typed
  var/param/assert, will be incorrectly flagged. Resolving this would
  require resolving callee signatures at every call expression — ruled out
  as materially larger scope than the lexical-mention scan.
- **Helper-call-mediated mutation not traced**, including: mutation
  through a field's own method (`t.buf.Write(...)`, `t.list.PushBack(...)`);
  slice/map element mutation (`t.items[0] = x`, `delete(t.m, k)`); and
  passing a receiver field by reference to another function that mutates
  it there. Undecidable in general at this Tier-C's AST-only scope;
  direct-field-mutation-only matches the guide's literal per-method
  question ("does *this* method need to modify the struct's own fields").
- **Ported-not-imported `lockPath` requires periodic re-audit.** Because
  `lockPath` is copied rather than imported (unexported/internal to
  `x/tools`), a future upstream bug fix or semantic extension to the real
  `copylock` analyzer will silently NOT propagate here. Re-compare
  `recvshape.go`'s `lockPath` against the vendored `x/tools` version's
  `copylock.go` whenever the `golang.org/x/tools` dependency is bumped.
- **Generic receiver types** (`T.TypeParams() != nil`) are skipped
  entirely rather than porting `lockPath`'s upstream `*types.TypeParam`
  branch (`typeparams.StructuralTerms`) — deferred, not silently dropped.

**Cross-vendor `/grade r1` (A-, APPROVE)** confirmed the design is sound
and identified the limitations above as the accepted scope boundary rather
than oversights; all findings were absorbed into the plan before
implementation (jeeves tasks.jeeves interactions 67060–67062).

## v6: `aliaswrite` (jeeves #65786)

Sixth analyzer, sixth package: `aliaswrite/`. Detects the **Slice Aliasing
Trap** (go-development-guide.md §11): value-copying a struct copies slice/map
headers but shares the backing array/map, so mutating that backing through a
value-receiver method silently corrupts every other copy.

**Detection** — a method is flagged when ALL hold:

1. **Value receiver**, named (not `_`) — a pointer receiver shares the pointer
   deliberately, so mutation is intended, not the trap; a blank receiver has no
   identifier to trace.
2. **Receiver type has no `Clone()` method** (checked over the pointer method
   set, so a value- or pointer-receiver `Clone` both count). Presence of
   `Clone()` is the task heuristic that the aliasing is handled — exempt.
3. The body **mutates a slice/map field's shared backing** in one of three
   shapes: index-assignment `r.f[i] = x`; map `delete(r.f, k)`; or
   **reslice-append** `r.f = append(r.f[lo:hi], …)` (the guide's flagship
   dangerous pattern — first `append` arg is a `SliceExpr` over the field).

**Deliberately NOT flagged** (guide §11 "when Clone NOT required"): plain
append-only `r.f = append(r.f, x)` (first arg is the whole field, no reslice —
grows without corrupting a shared prefix); read-only access; scalar-field
assignment `r.f = x` (mutates only the local copy); mutation of a local slice.

**Undecidability + scope (roster note).** Whole-program aliasing is undecidable;
`aliaswrite` is a syntactic, conservative, single-method detector that
**favors false-negatives over false-positives** (the repo precedent — see
`recvshape`). It does not do escape/reachability analysis, cross-method flow,
or interprocedural tracing. Generic receiver types are skipped (v1 scope,
matching `recvshape`). The `Clone()`-presence exemption is a heuristic: a type
can have `Clone()` yet a method still mutate without calling it — accepted as
the tight-scope boundary, not an oversight.

**Structure** mirrors `recvshape/`: `aliaswrite.go` (receiver iteration +
`sliceOrMapField`/`isResliceAppend`/`isBuiltin` helpers) + `testdata/src/a/a.go`
(`// want` fixtures) + `aliaswrite_test.go` (`analysistest.Run`). Wired into
`cmd/go-fp-lint`'s `multichecker.Main`.

## v7: `chainlambda` (jeeves #65782)

Seventh analyzer, seventh package: `chainlambda/`. Detects an inline function
literal passed directly as an argument to a **fluentfp chain method**
(`KeepIf`, `RemoveIf`, `ToString`, …) — fluentfp-guide.md prefers a named
function or method expression, which reads better in a chain. Residual to the
method-expression codemod (#66032); this is the syntactic detector half.

**Detection** — a `CallExpr` whose `Fun` is a `SelectorExpr` (method call) is
flagged when: (1) the method identifier resolves via `pass.TypesInfo.Uses` to a
`*types.Func` whose **defining package path contains `binaryphile/fluentfp`**
(type-resolved, NOT a method-name guess — so a same-named method on a
non-fluentfp type is not a false positive); and (2) any argument is an
`*ast.FuncLit`. Each offending `FuncLit` arg is reported at its own position.

**NOT flagged**: named function / method-expression arguments (the fix); inline
lambdas passed to non-fluentfp methods; lambdas that are not chain-method
arguments (plain assignment, etc.).

**Testdata note.** Type-resolution needs the receiver's package to actually be
fluentfp, so `testdata/src/github.com/binaryphile/fluentfp/slice/slice.go`
stubs a minimal `Mapper[T]` + a few higher-order methods; `analysistest`'s
GOPATH-mode testdata resolves the nested import path. This is the pattern for
any analyzer keyed on a specific external package's identity.

**Structure** mirrors the other analyzers: `chainlambda.go` +
`testdata/src/a/a.go` (`// want` fixtures) + `chainlambda_test.go`. Wired into
`multichecker.Main` (8th analyzer).

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

- `go test ./impuresource/...` — analysistest golden-fixture suite, 13
  fixture functions asserting 13 `// want`-tagged diagnostics (4
  call-detection shapes covering regular/aliased/dot-imports; 7 var-touch
  classifications covering all four verbs plus the selector-chain-boundary
  double-read, the range-assign write, and the multi-assign write) and 7
  true negatives with no diagnostic expected (method-shadow, name-shadow,
  local-scope, cross-package, allowlist-miss, const-vs-var,
  range-declare-shadow) — all correct.
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

## Verification performed (v2.1 cycle — `impurereach`, jeeves #65901)

TDD red/green (khorikov-unit-testing-guide.md §9, required for new
behavioral surface): `impurereach_test.go` + `testdata/src/a/a.go` (12
fixtures) and `testdata/src/b/b.go` were written and confirmed failing
(`impurereach` package didn't exist yet) BEFORE `impurereach.go` was
implemented.

- `go test ./impurereach/...` — 12 golden fixtures covering: a 2-hop and a
  3-hop chain; self-recursion combined with a direct call (no diagnostic,
  no infinite loop); mutual recursion reaching an impure call (cycle
  safety + correct depth counting); an IIFE (anonymous closure propagates
  reachability to its enclosing named function); a function-value
  parameter (indeterminate diagnostic, no propagation to the caller); an
  interface-mediated call (interface-specific indeterminate wording, no
  propagation); the cross-package boundary (unflagged — structurally
  can't be otherwise); an allowlist-miss deep in a chain (unflagged); a
  method as an intermediate hop (participates like a plain function); a
  builtin call (`println`, confirmed empirically to lower to a
  `*ssa.Builtin` call — not flagged as indeterminate); and a bound method
  value (added post-IMPL-grade, fixture #12 — a second, structurally
  different synthetic-wrapper category, empirically confirming the
  general `Synthetic != ""` exclusion policy). One real bug found and
  fixed during the first run (synthetic pointer-wrapper methods causing
  double-reporting — see §v2.1 above); all fixtures green after the fix,
  and after the post-grade fixture addition.
- `go test ./...` — full repo suite green (`filterloop`, `impuresource`,
  `nestedcall` all unaffected — confirms the `impuresource.ImpureFuncs`
  export rename caused no regression).
- `go build ./...` / `go vet ./...` — zero findings.
- Built binary smoke-tested against a real 3-function chain
  (`inner`→`middle`→`outer`, `inner` directly calling `os.Getenv`) outside
  the testdata tree — `inner` got `impuresource`'s direct-call diagnostic,
  `middle` and `outer` both got `impurereach`'s transitive diagnostic, all
  three analyzers (`filterloop`, `impuresource`, `impurereach`) and the
  concurrently-shipped `nestedcall` registered and running together
  correctly in the multichecker.

## Verification performed (v3 cycle — `nestedcall`, jeeves #65783)

TDD red/green (khorikov-unit-testing-guide.md, required for new behavioral
surface): `nestedcall_test.go` + `testdata/src/a/a.go` (14 fixtures) were
written against a stub no-op `Analyzer` and confirmed failing (5 missing-
diagnostic failures, exactly matching the 3 `// want`-tagged positive
fixtures) BEFORE `nestedcall.go`'s real logic was implemented.

- `go test ./nestedcall/...` — all 14 golden fixtures pass on the first
  real-logic implementation attempt: 3 positives (paren-depth,
  uniform-commas, both-at-once) plus 11 negatives, including the 5 AST edge
  cases added per `/grade r1` finding 7 (nested zero-arg calls,
  func-returning-func, variadic spread, parenthesized callee, mixed
  sibling-depth confirming max-not-sum aggregation).
- `go build ./...` / `go vet ./...` / `go test ./...` — scoped to
  `nestedcall`, `cmd/go-fp-lint`, `filterloop`, `impuresource` (excluding a
  concurrently-in-progress sibling package in the shared tree); all green.
- Built binary smoke-tested against `filterloop/filterloop.go` and
  `impuresource/impuresource.go` — zero `nestedcall` false positives.
- Broader false-positive corpus check (per `/grade r1` finding 4): built
  binary run via `go vet -vettool=` against `~/projects/era`'s Go module (a
  large, unrelated, real-world codebase) — both diagnostics fired at
  multiple genuine nested-call sites (e.g. `codesearch.go`, `era.go`,
  `storereporter.go`), with no crashes or obviously-wrong matches observed
  on manual spot-check, giving reasonable confidence the detector doesn't
  produce pathological noise on ordinary Go code outside this repo.
- One round of cross-vendor `/grade` (R1: A-, APPROVE) — findings absorbed:
  documented the adjacent-pair (not whole-chain) uniform-commas scope
  choice explicitly in Design, added 5 AST edge-case fixtures, and
  broadened the false-positive verification beyond the two in-tree files.

## Verification performed (v4 cycle — `mapshape`, jeeves #65781)

TDD red/green: `mapshape_test.go` + `testdata/src/a/a.go` (12 fixtures)
were written against a stub no-op `Analyzer` and confirmed failing (8
missing-diagnostic failures, exactly matching the 8 positive `// want`
lines) BEFORE `mapshape.go`'s real logic was implemented.

- `go test ./mapshape/...` — all 12 golden fixtures pass on the first
  real-logic implementation attempt: 8 positives (Transform, ToString,
  ToInt, ToInt32, ToError, ToAny, arbitrary-struct/`slice.Map`, and the
  `Marker`-empty-interface case correctly falling through to `slice.Map`
  rather than `ToAny`) plus 4 negatives (identity copy-loop, `filterloop`'s
  guard-if and continue-guard shapes correctly not double-firing,
  multi-statement body).
- Discrimination fixtures added per `/grade r1` finding 5: `int` vs
  `int32` targets (adjacent `*types.Basic` kind-switch branches) and a
  locally-declared empty interface vs the builtin `any` (adjacent
  identity-check branches) — both confirm the classifier selects the
  correct branch, not merely that some diagnostic fires at the right line.
- `go build` / `go vet` / `go test` — scoped to `mapshape`,
  `cmd/go-fp-lint`, `filterloop`, `impuresource`, `nestedcall`; all green.
- One round of cross-vendor `/grade` (R1: A-, APPROVE) — findings absorbed:
  specified the exact `error`/`any` detection mechanism (universe-identity
  check, not structural) per finding 2, added the two discrimination
  fixtures per finding 5, and confirmed #65868's scope covers this cycle's
  guide-drift finding per finding 6 (see §v4 above). Process note: this
  grade round ran after Implementation Gate bash had already published
  contract/plan events (user invoked `/grade` mid-execution rather than
  before `ExitPlanMode`, per normal 1d.5-then-gate ordering) — findings
  were absorbed directly into the implementation rather than back into the
  already-published plan file; logged as `/variance 65781` on
  `tasks.jeeves`, not a scope or contract change.

## Verification performed (v5 cycle — `recvshape`, jeeves #65784)

TDD red/green (REQUIRED for new behavioral surface per khorikov-unit-testing-guide.md):
`recvshape_test.go` + `testdata/src/a/a.go` (17 type declarations,
including the `Stringer` interface and `innerLock` helper type; 3 positive
`// want`-tagged fixtures) were written against a stub no-op `Analyzer` and
confirmed failing with exactly 3 missing-diagnostic failures — `NoMutate`,
`BlankRecv`, `PtrLockHolder` — BEFORE `recvshape.go`'s real detection logic
(lock exclusion, interface exclusion, mutation detection, type-level
exemption) was implemented.

- `go test ./recvshape/...` — the 3 `// want`-tagged positives pass, and
  every other method/type in the fixture file implicitly asserts a
  negative (`analysistest` fails on any unexpected diagnostic), on the
  first real-logic implementation attempt. Negative coverage includes:
  legitimate
  mutation (direct assign, `IncDecStmt`, whole-value pointer-deref);
  type-level mixed-receiver exemption; the conservative `&t.field`
  address-of heuristic; direct/nested/embedded/array-of-structs lock
  exclusion; in-package interface-satisfaction exclusion; a composition
  fixture with two simultaneous exclusion conditions (lock + interface) on
  one type, verifying no double-flag or crash; and a value-receiver-only
  type the analyzer never examines.
- **`PtrLockHolder` fixture correction during implementation**: the
  original plan's fixture-matrix description treated "lock reached via an
  embedded pointer to a lock-containing struct" as an expected-negative
  exclusion case. Investigating upstream `copylock.go` while implementing
  `lockPath` showed this is backwards — pointers are explicitly "safe to
  copy" in copylock's own semantics (copying a struct with a `*Mutex`
  pointer field duplicates the pointer, not the lock), so the ported
  algorithm correctly does NOT exclude this case, and the fixture was
  written as a POSITIVE (flagged) case instead — a closer match to
  upstream than the plan anticipated, not a deviation from it. Documented
  in §v5 above and in the fixture's own comment.
- `go build ./...` / `go vet ./...` / `go test ./...` — scoped to the full
  repo (all 6 analyzer packages + `cmd/go-fp-lint`); all green.
- Built binary smoke-tested via `go vet -vettool=` against
  `impuresource/impuresource.go` and `nestedcall/nestedcall.go` — zero
  `recvshape` diagnostics (other analyzers' pre-existing diagnostics
  present and unaffected), no crashes.
- Broader false-positive corpus check: built binary run via
  `go vet -vettool=` against `~/projects/era`'s Go module (30 packages,
  large unrelated real-world codebase; other analyzers fired dozens of
  genuine diagnostics across the run, confirming the tool actually
  traversed the packages) — **zero `recvshape` diagnostics**. Honest
  characterization: this corpus run functioned as a crash/false-positive
  check (passed — no crashes, no noise) rather than a true-positive
  plausibility check, since it produced no hits to spot-check at all;
  plausible explanation is that era's types mostly either mutate
  legitimately or fall under the type-level consistency exemption, but
  this wasn't independently confirmed.
- **Non-contradiction with `go vet copylocks`** (the cycle's core
  correctness property, per `/grade r1` finding 1): ran stdlib `go vet`
  (includes `copylocks`) against `recvshape/testdata/src/a/` — zero
  diagnostics (the fixture file's lock-containing types are only ever
  touched through pointer-receiver methods, so `copylocks` has nothing to
  flag). Ran the built `recvshape` binary against the same file — flagged
  exactly the 3 expected positives, none of which are lock-containing
  types. Additionally, this non-contradiction is **structural, not just
  empirical**: `recvshape` only ever examines pointer-receiver methods
  (`byType` collection filters on `*ast.StarExpr` receivers), so it can
  never touch the value-receiver methods `copylocks` polices — the two
  analyzers operate on disjoint method sets by construction.
- One round of cross-vendor `/grade` (R1: A-, APPROVE) — findings
  absorbed: documented the `lockPath`-port maintenance/re-audit
  obligation, the field-method/slice-map/pass-by-reference mutation
  false-positive gap, a concrete `fmt.Stringer` call-site-only example for
  the interface-exclusion gap, and added the composition + lock-adversarial-matrix
  fixtures — all reflected in §v5's Known limitations above (jeeves
  tasks.jeeves interactions 67060–67062).

## Verification performed (v6 cycle — `aliaswrite`, jeeves #65786)

TDD red/green (REQUIRED for new behavioral surface per khorikov-unit-testing-guide.md):
`aliaswrite_test.go` + `testdata/src/a/a.go` were authored first — 4 positive
`// want`-tagged fixtures (slice index-assign, map index-assign, map `delete`,
reslice-append) and 7 implicit negatives (pointer receiver, `Clone`-bearing
type, append-only, read-only, scalar field, local slice, plus the whole-file
`analysistest` no-unexpected-diagnostic guarantee).

- `go test ./aliaswrite/` — all 4 positives flagged and every negative silent
  on the first real-logic implementation. Negative coverage is the design's
  scope boundary made executable: `PointerRecv` (pointer receiver skipped),
  `HasClone` (Clone-presence exemption), `AppendOnly` (append-only is not a
  reslice → guide §11 "Clone NOT required"), `ReadOnly` (no mutation),
  `ScalarField` (non-slice/map field), `LocalSlice` (mutation of a local, not a
  receiver field).
- `go build ./cmd/go-fp-lint` — the analyzer is wired into `multichecker.Main`
  as the 7th analyzer; builds clean.
- `go test ./...` / `go vet ./aliaswrite/` — full repo (7 analyzer packages +
  `cmd`) green; vet clean.
- **Non-contradiction with `recvshape`/`copylocks`**: `aliaswrite` examines only
  **value-receiver** methods (skips `*ast.StarExpr` receivers), while `recvshape`
  examines only pointer-receiver methods — disjoint method sets by construction,
  no double-flag. `copylocks` polices lock-copying, an orthogonal concern.

## Verification performed (v7 cycle — `chainlambda`, jeeves #65782)

TDD red/green (REQUIRED per khorikov-unit-testing-guide.md): authored first were
`chainlambda_test.go`, `testdata/src/a/a.go` (3 positive `// want` fixtures:
lambda→`KeepIf`, `ToString`, `RemoveIf`), and a stub fluentfp package
(`testdata/src/github.com/binaryphile/fluentfp/slice/slice.go`).

- `go test ./chainlambda/` — 3 positives flagged, 4 negatives silent
  (named-func `KeepIf`/`ToString`, inline lambda on a non-fluentfp method, a
  non-argument lambda) on the first real-logic run. The stub-fluentfp testdata
  package resolved under `analysistest`'s GOPATH mode without extra setup.
- `go build ./cmd/go-fp-lint` — wired as the 8th analyzer; builds clean.
- `go test ./...` (8 analyzer packages + `cmd`) / `go vet ./chainlambda/` — green.
- **Type-resolution, not name-guessing**: the fluentfp check is on the resolved
  method's `Pkg().Path()`, so `Other.Do(func...)` (a same-shaped call on a
  non-fluentfp type) is correctly NOT flagged — verified by the `Neg3` fixture.

## Verification performed (v8 cycle — `chainlayout`, jeeves #66031)

TDD red/green (REQUIRED per khorikov-unit-testing-guide.md): authored first were
`chainlayout_test.go`, `testdata/src/a/a.go`, and the stub fluentfp `slice`
package (extending chainlambda's with `Len`, `Map[T,R]`). First run was RED (no
non-test Go files); GREEN on the first real-logic implementation.

- `go test ./chainlayout/` — **6 positives flagged, 7 negatives silent** on the
  first real-logic run. Positives: 2-op inline, 3-op inline, 2-op partially
  collapsed, single-op split, generic-setup (`slice.Map[int,string]`,
  `IndexListExpr`) inline, parenthesized generic setup (`(slice.Map[...])(xs)`).
  Negatives (the scope boundary made executable): single-op inline, 2-op correct
  trailing-dot, **single-op inline with a multi-line lambda arg** (the method-name
  line metric, not the call `End()`, prevents the false split), generic-setup
  (`slice.From[int]`, `IndexExpr`) correctly formatted, non-fluentfp chain,
  **variable-rooted chain** (v1-skip, #71302), bare setup call (0 ops).
- `go build ./cmd/go-fp-lint` — wired as the **9th** analyzer; builds clean.
- `go test ./...` (9 analyzer packages + `cmd`) / `go vet ./chainlayout/` — green;
  fixtures gofmt-clean (gofmt preserves chain line-breaks, so layout violations
  survive formatting — the realistic case).
- **Cross-vendor adversarial grade (R1 B → R2 B+ → R3 B+, plateau-on-novelty
  exit).** R1 caught a load-bearing false-negative: a generic instantiated setup
  (`slice.Map[R]`) has an `*ast.IndexExpr`/`IndexListExpr` callee, not a bare
  `*ast.SelectorExpr` — `calleeSelector` now unwraps it (proven by the
  `PosGenericInline`/`PosParenGeneric` fixtures). R2/R3 hardened the
  setup-result-type recognizer (single-result, `types.Unalias`+pointer-strip,
  nil-guarded universe types, org-qualified path predicate).
- **No double-flag with `chainlambda`**: orthogonal concerns (lambda-as-argument
  vs chain line-layout); running the 9-analyzer binary on the repo's own
  (non-fluentfp) code emits zero `chainlayout` diagnostics.
