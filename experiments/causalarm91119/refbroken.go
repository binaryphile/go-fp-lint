package causalarm91119

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// BrokenAnalyzer is the reference name-only implementation — the FORBIDDEN
// technique (App B forbids it). Its FLAGGING DECISION is purely the selector
// name string ("Map"), with NO receiver-type check. To keep the discrimination
// signal cleanly attributable, it emits the SAME message as CorrectAnalyzer on
// the calls both would flag (computing the receiver type only for the message,
// not the decision) — so on the true-positive fixtures it matches the `// want`
// markers exactly, and its ONLY divergence is the FALSE POSITIVES it raises on
// the negative-control cases (Other.Map, cases 2 and 5b). The discrimination
// test therefore fails the broken reference specifically on "unexpected
// diagnostic" over-flagging — the exact part-(a) blind spot this arm exists to
// catch — rather than on some unrelated mismatch.
var BrokenAnalyzer = &analysis.Analyzer{
	Name: "fluentmapbroken",
	Doc:  "reference-broken name-only fluentmap analyzer (jeeves #91119 causal-arm oracle discrimination)",
	Run:  runBroken,
}

func runBroken(pass *analysis.Pass) (interface{}, error) {
	forEachMapCall(pass, func(call *ast.CallExpr, sel *ast.SelectorExpr, fn *types.Func) {
		// name-only decision: sel.Sel.Name == "Map" already guaranteed by
		// forEachMapCall, so flag unconditionally — no receiver-type check.
		pass.ReportRangef(call, "fluent Map call: %s", recvTypeString(fn))
	})
	return nil, nil
}
