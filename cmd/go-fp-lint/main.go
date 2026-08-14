// Command go-fp-lint mechanically enforces fluentfp/FP/go-dev conventions,
// parallel to shellcheck-convention-plugin for bash. Standalone
// go/analysis multichecker — usable directly or as `go vet -vettool=`.
// See docs/design.md for the analyzer roster and roadmap.
package main

import (
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/binaryphile/go-fp-lint/aliaswrite"
	"github.com/binaryphile/go-fp-lint/chainlambda"
	"github.com/binaryphile/go-fp-lint/chainlayout"
	"github.com/binaryphile/go-fp-lint/filterloop"
	"github.com/binaryphile/go-fp-lint/genfile"
	"github.com/binaryphile/go-fp-lint/impurereach"
	"github.com/binaryphile/go-fp-lint/impuresource"
	"github.com/binaryphile/go-fp-lint/internalmock"
	"github.com/binaryphile/go-fp-lint/mapfusion"
	"github.com/binaryphile/go-fp-lint/mapshape"
	"github.com/binaryphile/go-fp-lint/methodexpr"
	"github.com/binaryphile/go-fp-lint/nestedcall"
	"github.com/binaryphile/go-fp-lint/recvshape"
)

// multichecker (not singlechecker) even with one analyzer today — future
// analyzers (docs/design.md roster) just add to this list.
//
// Every analyzer is wrapped with genfile.SkipGenerated (#97273): a
// go.dev/s/generatedcode-marked file (e.g. templ-compiler output) never
// produces a diagnostic, matching golangci-lint's own convention -- the
// author has no hand-editing control over regenerated code.
func main() {
	analyzers := []*analysis.Analyzer{filterloop.Analyzer, impuresource.Analyzer, impurereach.Analyzer, nestedcall.Analyzer, mapshape.Analyzer, recvshape.Analyzer, aliaswrite.Analyzer, chainlambda.Analyzer, chainlayout.Analyzer, internalmock.Analyzer, methodexpr.Analyzer, mapfusion.Analyzer}
	for i, a := range analyzers {
		analyzers[i] = genfile.SkipGenerated(a)
	}
	multichecker.Main(analyzers...)
}
