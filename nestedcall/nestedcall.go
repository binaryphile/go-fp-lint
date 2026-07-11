// Package nestedcall detects two related fluentfp call-nesting readability
// violations from fluentfp-guide.md / go-development-guide.md: paren-depth
// (don't open more than two parens without closing) and uniform-commas
// (only one nesting level may have multiple arguments). See docs/design.md
// for the detection rules and known limitations.
package nestedcall

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

const (
	parenDepthMessage    = "call nesting depth exceeds 2 — extract to an intermediate named variable (see fluentfp-guide.md paren depth rule)"
	uniformCommasMessage = "commas at multiple nesting levels — extract the inner call to a named variable (see fluentfp-guide.md uniform commas rule)"
)

// Analyzer flags call-expression nesting shapes that violate the paren-depth
// or uniform-commas rules.
var Analyzer = &analysis.Analyzer{
	Name: "nestedcall",
	Doc:  "reports call nesting that violates the paren-depth or uniform-commas rules (see fluentfp-guide.md)",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		calls := collectCallExprs(file)
		nestedAsArg := nestedAsArgSet(calls)

		for _, call := range calls {
			if !nestedAsArg[call] && chainDepth(call) > 2 {
				pass.Report(analysis.Diagnostic{Pos: call.Pos(), Message: parenDepthMessage})
			}
			if hasUniformCommaViolation(call) {
				pass.Report(analysis.Diagnostic{Pos: call.Pos(), Message: uniformCommasMessage})
			}
		}
	}
	return nil, nil
}

// collectCallExprs returns every *ast.CallExpr in file, in AST-walk order.
func collectCallExprs(file *ast.File) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			calls = append(calls, call)
		}
		return true
	})
	return calls
}

// nestedAsArgSet marks every CallExpr that appears literally inside another
// CallExpr's Args slice. A CallExpr in a receiver/Fun position (method-chain
// or func-returning-func shapes) is deliberately NOT marked — only Args
// nesting counts toward paren-depth, matching the guide's own "OK" example
// of a method chain (results.Sort(...).Take(n)) staying within the limit.
func nestedAsArgSet(calls []*ast.CallExpr) map[*ast.CallExpr]bool {
	nested := make(map[*ast.CallExpr]bool)
	for _, call := range calls {
		for _, arg := range call.Args {
			if argCall, ok := arg.(*ast.CallExpr); ok {
				nested[argCall] = true
			}
		}
	}
	return nested
}

// chainDepth returns the maximum number of simultaneously-open call frames
// rooted at call — 1 for the call itself, plus the deepest nested call
// found among its arguments. Siblings are evaluated independently (max, not
// sum): two 2-deep chains passed as separate arguments to the same call
// still peak at depth 3, not 5, since only one chain is ever open at a time
// when reading left to right.
func chainDepth(call *ast.CallExpr) int {
	depth := 0
	for _, arg := range call.Args {
		if argCall, ok := arg.(*ast.CallExpr); ok {
			if d := chainDepth(argCall); d > depth {
				depth = d
			}
		}
	}
	return depth + 1
}

// hasUniformCommaViolation reports whether call has more than one argument
// AND at least one of those arguments is itself a call with more than one
// argument — commas at two nesting levels. Evaluated per adjacent
// parent/child pair (an intentional v1 scope choice; see docs/design.md).
func hasUniformCommaViolation(call *ast.CallExpr) bool {
	if len(call.Args) <= 1 {
		return false
	}
	for _, arg := range call.Args {
		if argCall, ok := arg.(*ast.CallExpr); ok && len(argCall.Args) > 1 {
			return true
		}
	}
	return false
}
