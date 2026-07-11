// Command go-fp-lint mechanically enforces fluentfp/FP/go-dev conventions,
// parallel to shellcheck-convention-plugin for bash. Standalone
// go/analysis multichecker — usable directly or as `go vet -vettool=`.
// See docs/design.md for the analyzer roster and roadmap.
package main

import (
	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/binaryphile/go-fp-lint/filterloop"
	"github.com/binaryphile/go-fp-lint/impurereach"
	"github.com/binaryphile/go-fp-lint/impuresource"
	"github.com/binaryphile/go-fp-lint/mapshape"
	"github.com/binaryphile/go-fp-lint/nestedcall"
	"github.com/binaryphile/go-fp-lint/recvshape"
)

// multichecker (not singlechecker) even with one analyzer today — future
// analyzers (docs/design.md roster) just add to this list.
func main() {
	multichecker.Main(filterloop.Analyzer, impuresource.Analyzer, impurereach.Analyzer, nestedcall.Analyzer, mapshape.Analyzer, recvshape.Analyzer)
}
