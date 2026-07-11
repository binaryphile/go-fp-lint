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

v1 ships exactly one analyzer (`filterloop` — detects for-loop filter
shapes that `slice.From(xs).KeepIf(predicate)` expresses more directly).
The remaining categories from the originating task (jeeves #62380) are
tracked as follow-up tasks — see `docs/design.md` §Roster.
