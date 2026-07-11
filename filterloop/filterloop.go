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
// loop and, when so, the diagnostic to report. Two shapes qualify, both
// expressible as slice.From(xs).KeepIf(predicate):
//
//   - guard-if:       for _, u := range xs { if cond { acc = append(acc, u) } }
//   - continue-guard: for _, u := range xs { if !cond { continue }; acc = append(acc, u) }
func matchFilterLoop(stmt *ast.RangeStmt) (analysis.Diagnostic, bool) {
	if !ifGuardFilter(stmt) && !continueGuardFilter(stmt) {
		return analysis.Diagnostic{}, false
	}
	return analysis.Diagnostic{
		Pos:     stmt.Pos(),
		Message: "for-loop filter shape — use slice.From(xs).KeepIf(predicate) instead (see fluentfp-guide.md)",
	}, true
}

// ifGuardFilter reports whether `stmt`'s body is exactly one guard `if`
// (no else, no init) whose body is exactly one `acc = append(acc, ...)`
// assignment — the classic filter shape.
func ifGuardFilter(stmt *ast.RangeStmt) bool {
	if len(stmt.Body.List) != 1 {
		return false
	}
	ifStmt, ok := stmt.Body.List[0].(*ast.IfStmt)
	if !ok || ifStmt.Else != nil || ifStmt.Init != nil {
		return false
	}
	if len(ifStmt.Body.List) != 1 {
		return false
	}
	_, ok = appendAccIdent(ifStmt.Body.List[0])
	return ok
}

// continueGuardFilter reports whether `stmt`'s body is exactly two
// statements: an `if <cond> { continue }` guard (unlabeled continue, no
// else, no init) followed by a single `acc = append(acc, ...)`
// assignment — the early-continue equivalent of the classic filter shape.
func continueGuardFilter(stmt *ast.RangeStmt) bool {
	if len(stmt.Body.List) != 2 {
		return false
	}
	ifStmt, ok := stmt.Body.List[0].(*ast.IfStmt)
	if !ok || ifStmt.Else != nil || ifStmt.Init != nil {
		return false
	}
	if len(ifStmt.Body.List) != 1 {
		return false
	}
	branch, ok := ifStmt.Body.List[0].(*ast.BranchStmt)
	if !ok || branch.Tok != token.CONTINUE || branch.Label != nil {
		return false
	}
	_, ok = appendAccIdent(stmt.Body.List[1])
	return ok
}

// appendAccIdent returns the accumulator identifier when `stmt` is an
// assignment `acc = append(acc, ...)` with the same identifier `acc` on
// both sides, and true. Both filter shapes share this tail.
func appendAccIdent(stmt ast.Stmt) (*ast.Ident, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, false
	}
	accIdent, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return nil, false
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok || !isAppendOf(call, accIdent) {
		return nil, false
	}
	return accIdent, true
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
