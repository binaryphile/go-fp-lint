// Package causalarm91119 is the frozen hidden oracle for the jeeves #91119
// randomized normal-brief-vs-technique-brief causal arm (preregistration:
// jeeves investigations/2026-08-01-model-routing-causal-arm-preregistration.md,
// §5 + Appendix C). It holds the reference-correct (type-aware) and
// reference-broken (name-only) fluentmap analyzers plus the discrimination test
// that validates the oracle before it scores any delegate output.
//
// NOTE FOR DELEGATE DISPATCH (#93569 §6): this directory is the oracle's ground
// truth and MUST NOT be visible to the dispatched delegates — the frozen base
// commit for delegate worktrees must exclude experiments/causalarm91119/ (or
// delegates receive only the App A/B brief in a checkout without it). Seeing
// refcorrect.go would trivially leak the answer.
package causalarm91119

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const fluentfpPath = "binaryphile/fluentfp"

const mapMessage = "fluent Map call"

// CorrectAnalyzer is the reference type-aware implementation: it flags a
// `.Map(...)` call whose method resolves (via go/types) to fluentfp's Map —
// regardless of type alias or embedding — and never flags a same-named Map on
// an unrelated type. It is the oracle's known-correct reference (the
// discrimination test requires this to PASS the fixtures).
var CorrectAnalyzer = &analysis.Analyzer{
	Name: "fluentmapcorrect",
	Doc:  "reference-correct type-aware fluentmap analyzer (jeeves #91119 causal-arm oracle)",
	Run:  runCorrect,
}

func runCorrect(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if resolvesToFluentMap(pass, sel) {
				pass.ReportRangef(call, mapMessage)
			}
			return true
		})
	}
	return nil, nil
}

// resolvesToFluentMap reports whether the selector's method resolves to a
// *types.Func named Map whose defining package path is under fluentfp. Uses
// Selections first (handles promoted/embedded methods and method values), then
// falls back to Uses. This is the type-aware technique a name-only check omits.
func resolvesToFluentMap(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	var fn *types.Func
	if seln := pass.TypesInfo.Selections[sel]; seln != nil {
		fn, _ = seln.Obj().(*types.Func)
	}
	if fn == nil {
		fn, _ = pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	}
	if fn == nil || fn.Name() != "Map" || fn.Pkg() == nil {
		return false
	}
	return strings.Contains(fn.Pkg().Path(), fluentfpPath)
}
