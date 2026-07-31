# go-fp-lint

Standalone `go/analysis` checks enforcing fluentfp/FP/go-dev conventions —
the Go parallel of [shellcheck-convention-plugin](https://github.com/binaryphile/shellcheck-convention-plugin)
for bash. See `docs/design.md` for the full analyzer roster, design
decisions, and deferred scope.

## Usage

```bash
go build -o go-fp-lint ./cmd/go-fp-lint
./go-fp-lint ./...              # lint a module
go vet -vettool=$(which go-fp-lint) ./...   # or as a go vet plugin
```

## Development

```bash
nix develop      # devShell with go + gopls
go test ./...    # run analyzer tests (analysistest golden fixtures)
go vet ./...     # lint this repo's own code
```

## Status

Twelve analyzers ship today (see `docs/design.md` §vN for each; §Roster for
the full tiered plan):

- `filterloop` — for-loop filter shapes that
  `slice.From(xs).KeepIf(predicate)` expresses more directly.
- `impuresource` — direct impure-call + package-var touch inventory.
- `impurereach` — transitive reach into impure sources.
- `nestedcall` — paren-depth / uniform-comma nested-call shapes; offers a
  `change_me`-placeholder extraction `SuggestedFix` (`-fix`/`-diff`) within
  a narrow, evaluation-order-safe domain (jeeves #66034; see
  `docs/design.md` §v12).
- `mapshape` — map-loop shapes that `Transform`/`ToXxx`/`Map` express.
- `recvshape` — pointer receivers that could be value receivers
  (go-development-guide.md §3).
- `aliaswrite` — value-receiver methods that mutate a slice/map field's
  shared backing when the type has no `Clone()` method
  (go-development-guide.md §11 Slice Aliasing Trap).
- `chainlambda` — inline lambdas passed to a fluentfp chain method; prefer a
  named function or method expression (fluentfp-guide.md).
- `chainlayout` — fluentfp chain line-layout: single-op chains inline, multi-op
  one-per-line with trailing dots (fluentfp-guide.md §Chain Formatting; Tier-A
  detector, setup-constructor- or variable/return-rooted via generalized
  static-type root detection).
- `internalmock` — `Mock<X>` types whose target `<X>` is defined within the
  same module (a design smell — extract pure domain logic instead), vs. a
  real inter-system boundary mock (go-development-guide.md §6).
- `methodexpr` — Tier-B codemod: `func(x T) R { return x.M() }` passed to a
  fluentfp chain method → the method expression `T.M`
  (fluentfp-guide.md §Method Expressions); offered via `SuggestedFix`
  (`-fix`/`-diff`), name-free, value-receiver-only.
- `mapfusion` — two adjacent fluentfp maps that should fuse into one pass with a
  composed function ("Don't chain when a single pass suffices",
  go-development-guide.md); covers fluent-chain, standalone-nested `slice.Map`,
  and mixed forms (`docs/design.md` §v13).

The remaining categories from the originating task (jeeves #62380) are
tracked as follow-up tasks — see `docs/design.md` §Roster.
