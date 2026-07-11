// Package mapshape detects the classic fluentfp map-loop shape: a for-range
// loop with no if-guard whose body directly transforms and appends each
// element. See docs/design.md for the detection rules, the target-type
// classification, and known limitations.
package mapshape

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// Analyzer flags map-loop shapes better expressed via slice.From(xs).Transform,
// slice.From(xs).ToXxx, or the standalone slice.Map, depending on target type.
var Analyzer = &analysis.Analyzer{
	Name: "mapshape",
	Doc:  "reports map-loop shapes better expressed via fluentfp Transform/ToXxx/Map (see fluentfp-guide.md)",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			rangeStmt, ok := n.(*ast.RangeStmt)
			if !ok {
				return true
			}
			if diag, matched := matchMapLoop(pass, rangeStmt); matched {
				pass.Report(diag)
			}
			return true
		})
	}
	return nil, nil
}

// matchMapLoop reports whether `stmt` is a map-loop shape and, when so, the
// diagnostic to report. A map-loop is a for-range loop whose body is
// exactly one `acc = append(acc, EXPR)` statement, where EXPR is not the
// bare range value itself (a plain copy, nothing to transform). This shape
// structurally excludes filterloop's guard-if (body's one statement is an
// `if`, not an assign) and continue-guard (body is two statements).
func matchMapLoop(pass *analysis.Pass, stmt *ast.RangeStmt) (analysis.Diagnostic, bool) {
	if len(stmt.Body.List) != 1 {
		return analysis.Diagnostic{}, false
	}
	expr, ok := mapLoopAppendExpr(stmt)
	if !ok {
		return analysis.Diagnostic{}, false
	}
	rangeValue, ok := stmt.Value.(*ast.Ident)
	if !ok {
		return analysis.Diagnostic{}, false
	}
	if exprIdent, ok := expr.(*ast.Ident); ok && exprIdent.Name == rangeValue.Name {
		return analysis.Diagnostic{}, false // identity copy-loop, nothing to transform
	}
	message := classifyTarget(pass, rangeValue, expr)
	return analysis.Diagnostic{
		Pos:     stmt.Pos(),
		Message: message,
	}, true
}

// mapLoopAppendExpr returns the appended expression when `stmt`'s body is
// exactly one assignment `acc = append(acc, EXPR)` with `acc` the same
// identifier on both sides and exactly one non-spread argument.
func mapLoopAppendExpr(stmt *ast.RangeStmt) (ast.Expr, bool) {
	assign, ok := stmt.Body.List[0].(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, false
	}
	accIdent, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return nil, false
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok || call.Ellipsis.IsValid() || len(call.Args) != 2 {
		return nil, false
	}
	fn, ok := call.Fun.(*ast.Ident)
	if !ok || fn.Name != "append" {
		return nil, false
	}
	firstArg, ok := call.Args[0].(*ast.Ident)
	if !ok || firstArg.Name != accIdent.Name {
		return nil, false
	}
	return call.Args[1], true
}

// classifyTarget returns the diagnostic message for EXPR's target type
// relative to rangeValue's type: same-type maps to Transform, one of the
// ~10 fluentfp typed-alias targets maps to the matching ToXxx, anything
// else maps to the standalone slice.Map.
func classifyTarget(pass *analysis.Pass, rangeValue *ast.Ident, expr ast.Expr) string {
	sourceType := pass.TypesInfo.TypeOf(rangeValue)
	targetType := pass.TypesInfo.TypeOf(expr)

	if sourceType != nil && targetType != nil && types.Identical(sourceType, targetType) {
		return mapLoopMessage("slice.From(xs).Transform(fn)")
	}
	if method, ok := toXxxMethod(targetType); ok {
		return mapLoopMessage("slice.From(xs)." + method + "(fn)")
	}
	return mapLoopMessage("slice.Map(xs, fn)")
}

// toXxxMethod returns the fluentfp Mapper[T] typed-mapping method name for
// targetType, when targetType is one of the ~10 known-alias target types
// (error and any checked by exact identity against go/types' universe
// scope, to avoid misclassifying a user-declared named-error type or a
// locally-declared empty interface as the builtin error/any).
//
// Known limitation: rune and int32 are the identical Go type (rune is a
// builtin alias for int32) — go/types cannot distinguish which spelling
// the source used. This always resolves to ToInt32, never ToRune; an
// inherent ambiguity in fluentfp's own API surface, not a bug to fix here.
func toXxxMethod(targetType types.Type) (string, bool) {
	if targetType == nil {
		return "", false
	}
	if types.Identical(targetType, types.Universe.Lookup("error").Type()) {
		return "ToError", true
	}
	if types.Identical(targetType, types.Universe.Lookup("any").Type()) {
		return "ToAny", true
	}
	basic, ok := targetType.Underlying().(*types.Basic)
	if !ok {
		return "", false
	}
	switch basic.Kind() {
	case types.Bool:
		return "ToBool", true
	case types.Uint8: // byte is an alias for uint8
		return "ToByte", true
	case types.Float32:
		return "ToFloat32", true
	case types.Float64:
		return "ToFloat64", true
	case types.Int:
		return "ToInt", true
	case types.Int32: // also covers rune (alias for int32); see doc comment
		return "ToInt32", true
	case types.Int64:
		return "ToInt64", true
	case types.String:
		return "ToString", true
	default:
		return "", false
	}
}

func mapLoopMessage(replacement string) string {
	return "map-loop shape — use " + replacement + " instead (see fluentfp-guide.md)"
}
