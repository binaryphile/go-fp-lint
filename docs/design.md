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
| hidden actions in ostensibly-pure functions | functional-programming-unified-guide.md | Deferred — explicitly flagged in the originating task as possibly "too semantic for static analysis"; needs its own design pass before a task is even worth filing |
| `option.Basic`/`option.Option` API drift check | fluentfp API vs go-development-guide.md | Deferred — no task filed yet |

Each deferred item needs its own `evtctl task` filed against `tasks.jeeves`
before pickup (not done in this cycle — see the closeout for this arc).

## Integration points (documented, not wired up this cycle)

Per the originating task: pre-commit hook, `/c` skill invocation, tandem
gate-bash (`<linter> ./... || exit 1` on Standard-tier cycles touching
`*.go`), author-time IDE/LSP integration. None of these are wired up
yet — v1 ships the binary + one analyzer only. Wiring these up is
follow-up scope, likely per-integration-point tasks rather than one
umbrella task (each has a different owner/repo: pre-commit hooks live
per-repo, `/c` skill lives in `~/.claude/skills/`, gate-bash is a
per-cycle plan-file convention).

## Verification performed this cycle

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
