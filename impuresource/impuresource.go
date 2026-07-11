// Package impuresource detects direct calls to an allowlisted set of
// impure stdlib functions and touches (read/write/address-of/
// compound-assign) of the analyzed package's own package-scope vars, per
// functional-programming-unified-guide.md's Actions/Calculations/Data
// taxonomy. See docs/design.md for the detection rules, allowlist, and
// known limitations — this is a direct-impurity-source detector / action
// inventory, not a hidden-action or purity-contract-violation detector.
package impuresource

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// Analyzer reports direct calls to an allowlisted impure-func set and
// touches of the analyzed package's own package-scope vars.
var Analyzer = &analysis.Analyzer{
	Name: "impuresource",
	Doc:  "reports direct calls to allowlisted impure funcs and package-scope-var touches (see functional-programming-unified-guide.md)",
	Run:  run,
}

// impureFuncs is the v1 allowlist, keyed by import path then func name.
// Intentionally incomplete — extending it is a code edit, not a runtime
// flag (see docs/design.md).
var impureFuncs = map[string]map[string]bool{
	"time": {"Now": true},
	"os":   {"Getenv": true},
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return false
			}
			if diag, ok := matchImpureCall(pass, n); ok {
				pass.Report(diag)
			}
			if ident, ok := n.(*ast.Ident); ok {
				if diag, ok := matchPackageVarTouch(pass, ident, stack); ok {
					pass.Report(diag)
				}
			}
			stack = append(stack, n)
			return true
		})
	}
	return nil, nil
}

// matchImpureCall reports whether n is a call whose callee resolves to a
// *types.Func matching the allowlist. Resolves via pass.TypesInfo.Uses,
// stable across import aliases and dot-imports; requires Recv() == nil to
// exclude methods. Only matches calls whose callee resolves to a
// *types.Func — a function-valued variable (e.g. f := time.Now; f())
// resolves to a *types.Var instead and is out of scope by construction.
func matchImpureCall(pass *analysis.Pass, n ast.Node) (analysis.Diagnostic, bool) {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return analysis.Diagnostic{}, false
	}
	var ident *ast.Ident
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		ident = fn
	case *ast.SelectorExpr:
		ident = fn.Sel
	default:
		return analysis.Diagnostic{}, false
	}
	obj, ok := pass.TypesInfo.Uses[ident].(*types.Func)
	if !ok || obj.Pkg() == nil {
		return analysis.Diagnostic{}, false
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok || sig.Recv() != nil {
		return analysis.Diagnostic{}, false
	}
	if !impureFuncs[obj.Pkg().Path()][obj.Name()] {
		return analysis.Diagnostic{}, false
	}
	return analysis.Diagnostic{
		Pos:     call.Pos(),
		Message: "direct call to " + obj.Pkg().Name() + "." + obj.Name() + " — actions are not calculations (see functional-programming-unified-guide.md)",
	}, true
}

// matchPackageVarTouch reports whether ident denotes a package-scope var of
// the analyzed package (own package only — imported packages' exported
// vars are a stated v1 non-goal), classifying the touch by ident's
// immediate ancestor in stack.
func matchPackageVarTouch(pass *analysis.Pass, ident *ast.Ident, stack []ast.Node) (analysis.Diagnostic, bool) {
	obj, ok := pass.TypesInfo.Uses[ident].(*types.Var)
	if !ok || obj.Pkg() != pass.Pkg || obj.Parent() != pass.Pkg.Scope() {
		return analysis.Diagnostic{}, false
	}
	verb := classifyUse(ident, stack)
	return analysis.Diagnostic{
		Pos:     ident.Pos(),
		Message: verb + " package-scope var " + obj.Name() + " (see functional-programming-unified-guide.md)",
	}, true
}

// classifyUse classifies ident's use by its immediate ancestor: address-of
// (&ident), compound-assign (ident++, ident += x), write (ident = x, or the
// assign-form range clause `for ident = range xs` reusing an existing var),
// or read (the default). Only inspects the immediate parent; deeper
// selector-chain writes (e.g. ident.Field = x) fall through to read — see
// docs/design.md's selector-boundary limitation.
func classifyUse(ident *ast.Ident, stack []ast.Node) string {
	if len(stack) == 0 {
		return "read of"
	}
	switch parent := stack[len(stack)-1].(type) {
	case *ast.UnaryExpr:
		if parent.Op == token.AND && parent.X == ident {
			return "address-of"
		}
	case *ast.IncDecStmt:
		if parent.X == ident {
			return "compound-assign of"
		}
	case *ast.AssignStmt:
		if identInList(ident, parent.Lhs) {
			if isCompoundAssignTok(parent.Tok) {
				return "compound-assign of"
			}
			if parent.Tok == token.ASSIGN {
				return "write to"
			}
		}
	case *ast.RangeStmt:
		if parent.Tok == token.ASSIGN && (parent.Key == ident || parent.Value == ident) {
			return "write to"
		}
	}
	return "read of"
}

func identInList(ident *ast.Ident, list []ast.Expr) bool {
	for _, e := range list {
		if e == ident {
			return true
		}
	}
	return false
}

func isCompoundAssignTok(tok token.Token) bool {
	switch tok {
	case token.ADD_ASSIGN, token.SUB_ASSIGN, token.MUL_ASSIGN, token.QUO_ASSIGN,
		token.REM_ASSIGN, token.AND_ASSIGN, token.OR_ASSIGN, token.XOR_ASSIGN,
		token.SHL_ASSIGN, token.SHR_ASSIGN, token.AND_NOT_ASSIGN:
		return true
	default:
		return false
	}
}
