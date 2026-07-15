// Package chainlambda detects inline function literals passed directly as an
// argument to a fluentfp chain method (KeepIf, RemoveIf, ToString, …) —
// fluentfp-guide.md prefers a named function or method expression, which reads
// better in a chain than an inline lambda. Residual to the method-expression
// codemod (#66032); this is the syntactic detector (jeeves #65782).
//
// A call is flagged when the called method's DEFINING package is fluentfp
// (resolved via go/types, not a method-name guess) and any argument is a
// function literal. Named functions and method expressions are the fix and are
// never flagged; lambdas passed to non-fluentfp methods are out of scope.
package chainlambda

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const fluentfpPath = "binaryphile/fluentfp"

const message = "inline lambda passed to fluentfp chain method %s — use a named function or method expression (see fluentfp-guide.md)"

// Analyzer flags inline function literals passed to a fluentfp chain method.
var Analyzer = &analysis.Analyzer{
	Name: "chainlambda",
	Doc:  "reports inline lambdas passed to fluentfp chain methods; prefer a named function or method expression (see fluentfp-guide.md)",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
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
			if !isFluentfpMethod(pass, sel.Sel) {
				return true
			}
			for _, arg := range call.Args {
				if _, ok := arg.(*ast.FuncLit); ok {
					pass.ReportRangef(arg, message, sel.Sel.Name)
				}
			}
			return true
		})
	}
	return nil, nil
}

// isFluentfpMethod reports whether the method identifier resolves to a *types.Func
// whose defining package path is under fluentfp.
func isFluentfpMethod(pass *analysis.Pass, methodIdent *ast.Ident) bool {
	fn, ok := pass.TypesInfo.Uses[methodIdent].(*types.Func)
	if !ok || fn.Pkg() == nil {
		return false
	}
	return strings.Contains(fn.Pkg().Path(), fluentfpPath)
}
