// Package methodexpr is the Tier-B (Codemod) flagship rule from the go-fp-lint
// roster (docs/design.md §Roster): rewrite a single-parameter, single-statement
// passthrough lambda passed to a fluentfp chain method —
// func(x T) R { return x.M() } — to the method expression T.M
// (fluentfp-guide.md §"Method Expressions (preferred)"). Genuinely name-free
// (T and M are both already-existing identifiers) and semantics-preserving
// modulo the receiver-aliasing edge case documented below, so it is offered as
// an analysis.SuggestedFix rather than imposed (jeeves #66032, era e6372253f9f8).
//
// The detector doubles as a Tier-C diagnostic on its own — chainlambda already
// flags "inline lambda passed to a fluentfp chain method" generically; this
// package additionally flags the specific rewritable subset with an actionable
// fix. The two analyzers may both report the same lambda (chainlambda's
// broader rule and this package's narrower, fix-bearing one are not mutually
// exclusive) — accepted overlap, not a double-count of the same violation
// class; a reader acting on either message ends up at the same correct code.
//
// Safety scope: only VALUE-receiver methods qualify (fluentfp-guide.md's own
// "Critical: Use value receivers for read-only methods" — a pointer-receiver
// M would require the method expression (*T).M, whose signature func(*T) R
// does not match the original func(T) R, so it is never a drop-in
// replacement and is left undetected, not merely unfixed). The rewrite is
// also skipped unless x.M()'s result type is IDENTICAL (not just assignable)
// to the lambda's declared return type — Go function values are invariant in
// their result type, so a merely-assignable (e.g. via interface satisfaction)
// result would silently change the fluentfp chain method's inferred call
// signature if substituted.
package methodexpr

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const fluentfpPath = "github.com/binaryphile/fluentfp"

const message = "%s can be a method expression: replace with %s.%s"

// Analyzer flags a single-param, single-statement passthrough lambda passed to
// a fluentfp chain method that can be rewritten to a method expression.
var Analyzer = &analysis.Analyzer{
	Name: "methodexpr",
	Doc:  "reports fluentfp chain-method lambdas rewritable to a method expression T.M (see fluentfp-guide.md §Method Expressions)",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isFluentfpChainMethodCall(pass, call) {
				return true
			}
			for _, arg := range call.Args {
				lit, ok := arg.(*ast.FuncLit)
				if !ok {
					continue
				}
				if paramType, methodName, ok := rewritableMethodExpr(pass, lit); ok {
					report(pass, lit, paramType, methodName)
				}
			}
			return true
		})
	}
	return nil, nil
}

// isFluentfpChainMethodCall reports whether call's callee resolves to a
// fluentfp-defined method (has a receiver) — the shape a passthrough-lambda
// argument would need to be a chain-method callback.
func isFluentfpChainMethodCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel := calleeSelector(call)
	if sel == nil {
		return false
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok || fn.Pkg() == nil || !isFluentfpPath(fn.Pkg().Path()) {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	return ok && sig.Recv() != nil
}

// calleeSelector returns call's callee selector, unwrapping parentheses and
// generic-instantiation index nodes. It returns nil when the callee is not
// ultimately a selector.
func calleeSelector(call *ast.CallExpr) *ast.SelectorExpr {
	e := call.Fun
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		case *ast.SelectorExpr:
			return x
		default:
			return nil
		}
	}
}

// rewritableMethodExpr reports whether lit is exactly
// func(x T) R { return x.M() } for some value-receiver method M on T with no
// arguments, where M's result type is IDENTICAL to R. On success it returns
// the parameter's type expression (for rendering "T") and M's name.
func rewritableMethodExpr(pass *analysis.Pass, lit *ast.FuncLit) (paramType ast.Expr, methodName string, ok bool) {
	params := lit.Type.Params.List
	if len(params) != 1 || len(params[0].Names) != 1 {
		return nil, "", false
	}
	param := params[0].Names[0]
	if len(lit.Body.List) != 1 {
		return nil, "", false
	}
	ret, ok := lit.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return nil, "", false
	}
	call, ok := ret.Results[0].(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return nil, "", false // extra args would need to be threaded through T.M's params — not a plain passthrough
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, "", false
	}
	recvIdent, ok := sel.X.(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[recvIdent] != pass.TypesInfo.Defs[param] {
		return nil, "", false // the call's receiver must be exactly the lambda's own sole parameter
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok {
		return nil, "", false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return nil, "", false
	}
	if _, isPtr := sig.Recv().Type().(*types.Pointer); isPtr {
		return nil, "", false // pointer receiver: (*T).M has type func(*T) R, never a drop-in for func(T) R
	}
	if sig.Params().Len() != 0 {
		return nil, "", false
	}
	declaredResults := lit.Type.Results
	if declaredResults == nil || len(declaredResults.List) != 1 || sig.Results().Len() != 1 {
		return nil, "", false
	}
	if !types.Identical(sig.Results().At(0).Type(), pass.TypesInfo.TypeOf(declaredResults.List[0].Type)) {
		return nil, "", false // assignable-but-not-identical result types would change the substituted func value's signature
	}
	return params[0].Type, sel.Sel.Name, true
}

// report emits the diagnostic with a SuggestedFix replacing lit with the
// method expression T.M.
func report(pass *analysis.Pass, lit *ast.FuncLit, paramType ast.Expr, methodName string) {
	typeName := types.ExprString(paramType)
	replacement := fmt.Sprintf("%s.%s", typeName, methodName)
	pass.Report(analysis.Diagnostic{
		Pos:     lit.Pos(),
		End:     lit.End(),
		Message: fmt.Sprintf(message, "inline lambda", typeName, methodName),
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: fmt.Sprintf("replace with %s", replacement),
			TextEdits: []analysis.TextEdit{{
				Pos:     lit.Pos(),
				End:     lit.End(),
				NewText: []byte(replacement),
			}},
		}},
	})
}

// isFluentfpPath reports whether importPath is the fluentfp module or a
// package within it.
func isFluentfpPath(importPath string) bool {
	return importPath == fluentfpPath || strings.HasPrefix(importPath, fluentfpPath+"/")
}
