// Package filterloop detects for-loop filter shapes that fluentfp's
// slice.From(xs).KeepIf(predicate) expresses more directly, per
// guides/fluentfp-guide.md. See docs/design.md for the detection rules
// and known limitations.
package filterloop

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// Analyzer flags for-range loops whose body is exactly one guard `if`
// that appends the range value to an accumulator declared outside the
// loop — the classic filter shape fluentfp's KeepIf replaces.
var Analyzer = &analysis.Analyzer{
	Name: "filterloop",
	Doc:  "reports for-loop filter shapes better expressed as slice.From(xs).KeepIf(predicate)",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			rangeStmt, ok := n.(*ast.RangeStmt)
			if !ok {
				return true
			}
			if diag, matched := matchFilterLoop(rangeStmt); matched {
				pass.Report(diag)
			}
			return true
		})
	}
	return nil, nil
}

// matchFilterLoop reports whether `stmt` is a filter-shaped for-range
// loop: a single guard `if` (no else) whose body is exactly one
// statement appending the range value (or key) to an accumulator
// variable declared outside the loop. Returns the diagnostic to report
// and true on a match.
func matchFilterLoop(stmt *ast.RangeStmt) (analysis.Diagnostic, bool) {
	if len(stmt.Body.List) != 1 {
		return analysis.Diagnostic{}, false
	}

	ifStmt, ok := stmt.Body.List[0].(*ast.IfStmt)
	if !ok || ifStmt.Else != nil || ifStmt.Init != nil {
		return analysis.Diagnostic{}, false
	}
	if len(ifStmt.Body.List) != 1 {
		return analysis.Diagnostic{}, false
	}

	assign, ok := ifStmt.Body.List[0].(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return analysis.Diagnostic{}, false
	}

	accIdent, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return analysis.Diagnostic{}, false
	}

	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok || !isAppendOf(call, accIdent) {
		return analysis.Diagnostic{}, false
	}

	return analysis.Diagnostic{
		Pos:     stmt.Pos(),
		Message: "for-loop filter shape — use slice.From(xs).KeepIf(predicate) instead (see fluentfp-guide.md)",
	}, true
}

// isAppendOf reports whether `call` is `append(acc, ...)` for the given
// accumulator identifier `acc`.
func isAppendOf(call *ast.CallExpr, acc *ast.Ident) bool {
	fn, ok := call.Fun.(*ast.Ident)
	if !ok || fn.Name != "append" || len(call.Args) == 0 {
		return false
	}
	firstArg, ok := call.Args[0].(*ast.Ident)
	return ok && firstArg.Name == acc.Name
}
