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
| chain line-layout (one-op-per-line / inline) | all three | **A** | formatter — #66031 |
| method-expression (`func(x T) R { return x.M() }` → `T.M`) | fluentfp / fp-unified | **B** | codemod, name-free — #66032 |
| paren-depth + uniform-commas | fluentfp / go-dev | **C→B** | detector **Shipped** (`nestedcall`, #65783); `change_me` fix deferred **#66034** |
| double-map fusion → composed pass | fluentfp / fp-unified | **C→B** | detector task **#66830** (split out of #65783 at plan time — distinct violation condition, not a paren-depth/uniform-commas variant) + #66034 |
| map-loop → `Transform`/`ToXxx`/`Map` | fluentfp / go-dev | C | detector **Shipped** (`mapshape`, #65781) |
| inline lambda → named function (residual, non-method-expr) | fluentfp / go-dev | C | **#65782** |
| pointer receiver where value receiver works | go-dev | C | **#65784** (partial overlap `go vet copylocks`) |
| internal mock detection (design smell) | go-dev | C | **#65785** |
| slice/map field mutation without `Clone()` | go-dev | C | **#65786** (aliasing — undecidable; scope tight) |
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
existing linter covers.

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
