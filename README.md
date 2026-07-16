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

Nine analyzers ship today (see `docs/design.md` §vN for each; §Roster for
the full tiered plan):

- `filterloop` — for-loop filter shapes that
  `slice.From(xs).KeepIf(predicate)` expresses more directly.
- `impuresource` — direct impure-call + package-var touch inventory.
- `impurereach` — transitive reach into impure sources.
- `nestedcall` — paren-depth / uniform-comma nested-call shapes.
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
  detector, setup-constructor-rooted).

The remaining categories from the originating task (jeeves #62380) are
tracked as follow-up tasks — see `docs/design.md` §Roster.
