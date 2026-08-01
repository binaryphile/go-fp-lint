package causalarm91119

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

// BrokenAnalyzer is the reference name-only implementation — the FORBIDDEN
// technique (App B forbids it): it flags any `.Map(...)` call by matching the
// selector name string, with no type resolution. It therefore false-positives
// on the negative-control cases (Other.Map, case 2 and 5b). The discrimination
// test requires this to FAIL the fixtures; if it passed, the oracle would be
// too shallow to tell a correct analyzer from a broken one (the exact part-(a)
// failure mode this arm exists to avoid).
var BrokenAnalyzer = &analysis.Analyzer{
	Name: "fluentmapbroken",
	Doc:  "reference-broken name-only fluentmap analyzer (jeeves #91119 causal-arm oracle discrimination)",
	Run:  runBroken,
}

func runBroken(pass *analysis.Pass) (interface{}, error) {
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
			if sel.Sel.Name == "Map" { // name-only string match — no go/types
				pass.ReportRangef(call, mapMessage)
			}
			return true
		})
	}
	return nil, nil
}
