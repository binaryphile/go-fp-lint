// Package chainlayout enforces fluentfp chain line-layout (fluentfp-guide.md
// §Chain Formatting): a single-operation chain stays inline; a chain of two or
// more operations puts each operation on its own line with a trailing dot, the
// setup constructor alone on the first line. It is the Tier-A (Format) member of
// the go-fp-lint roster (docs/design.md §"Tier-A spec: chain line-layout").
//
// A fluent chain is a value produced by a fluentfp CHAIN ROOT followed by
// chained method calls. Only the chained methods count; the root is a
// non-counted bookend and the terminal ToX/Len call counts. Identity is
// resolved through go/types, not method-name guessing.
//
// A chain root is either an inline, qualifying fluentfp setup constructor
// (slice.From, slice.Map[R], option.Of, …), OR — as of v2 (jeeves #71302) — any
// other expression whose static type is itself a named fluentfp type:
// variable-rooted (m := slice.From(xs); m.A().B()) and function-return-rooted
// (getM().A().B(), where getM's OWN return type is fluentfp even though getM
// itself is not defined in the fluentfp module). The root's line is the
// method-name identifier's line when the root is itself a selector call
// (setup-constructor- or qualified-return-rooted), or the root expression's own
// end position otherwise (variable-rooted, or a bare-identifier-called return
// root) — see rootLine.
//
// v2 remaining exclusion: dot-imported (import . ".../slice") chains — import
// spelling is load-bearing despite the types-resolved identity. Detector only;
// an always-on rewriting SuggestedFix is a later layer.
package chainlayout

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const fluentfpPath = "github.com/binaryphile/fluentfp"

const (
	msgMultiOp  = "fluentfp chain with %d operations should be one per line with trailing dots (see fluentfp-guide.md §Chain Formatting)"
	msgSingleOp = "single-operation fluentfp chain should be inline (see fluentfp-guide.md §Chain Formatting)"
)

// Analyzer flags fluentfp chains whose line layout disagrees with the form their
// counted-operation count selects (one op → inline; two+ → one per line).
var Analyzer = &analysis.Analyzer{
	Name: "chainlayout",
	Doc:  "reports fluentfp chains whose line layout violates one-op-inline / multi-op-one-per-line (see fluentfp-guide.md §Chain Formatting)",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	handled := map[*ast.CallExpr]bool{}
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || handled[call] {
				return true
			}
			ops, root, spine, ok := walkChain(pass, call)
			if !ok {
				return true
			}
			for _, c := range spine {
				handled[c] = true // inner spine nodes: don't re-process as their own chain
			}
			reportChain(pass, root, ops, call)
			return true // keep descending: independent chains may live in argument subtrees
		})
	}
	return nil, nil
}

// walkChain classifies outer as the outermost call of a fluentfp chain. It
// returns the counted method-call ops (outermost-first), the chain root (an
// inline setup-constructor call, or — v2, jeeves #71302 — any other expression
// whose static type is itself fluentfp), and every CallExpr on the receiver
// spine (for handled-marking). ok is false when outer is not the outermost call
// of such a chain (not fluentfp, or dot-imported — the remaining v2 exclusion).
func walkChain(pass *analysis.Pass, outer *ast.CallExpr) (ops []*ast.CallExpr, root ast.Expr, spine []*ast.CallExpr, ok bool) {
	cur := outer
	for {
		sel := calleeSelector(cur)
		if sel == nil {
			return nil, nil, nil, false
		}
		fn, ok := fluentfpFunc(pass, sel.Sel)
		if !ok {
			return nil, nil, nil, false
		}
		spine = append(spine, cur)
		if !hasReceiver(fn) {
			// Package func: qualifies as a root bookend only if it constructs a
			// fluentfp chain value (guards against rooting at an unrelated
			// helper).
			if !returnsFluentfpChain(fn) {
				return nil, nil, nil, false
			}
			return ops, cur, spine, true
		}
		ops = append(ops, cur)
		if spineContinues(pass, sel.X) {
			cur = sel.X.(*ast.CallExpr)
			continue
		}
		// sel.X does not continue the fluentfp call spine — accept it as the
		// generalized chain root (variable-rooted, or return-rooted via a
		// function whose OWN return type is fluentfp even though the function
		// itself isn't defined in the fluentfp module; jeeves #71302) when its
		// static type is itself fluentfp; otherwise this isn't a fluentfp chain
		// at all.
		if !exprIsFluentfpTyped(pass, sel.X) {
			return nil, nil, nil, false
		}
		return ops, sel.X, spine, true
	}
}

// spineContinues reports whether expr is itself a CallExpr on the fluentfp
// spine (its callee resolves to a fluentfp-defined func) — i.e. whether
// walkChain should keep descending into expr rather than treat expr as the
// chain root.
func spineContinues(pass *analysis.Pass, expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel := calleeSelector(call)
	if sel == nil {
		return false
	}
	_, ok = fluentfpFunc(pass, sel.Sel)
	return ok
}

// calleeSelector returns call's callee selector, unwrapping parentheses and the
// generic-instantiation index nodes ((slice.Map[R])(xs) → slice.Map). It returns
// nil when the callee is not ultimately a selector (e.g. a dot-imported ident).
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

// fluentfpFunc resolves ident to the *types.Func it uses and reports whether that
// func is defined under the fluentfp module.
func fluentfpFunc(pass *analysis.Pass, ident *ast.Ident) (*types.Func, bool) {
	fn, ok := pass.TypesInfo.Uses[ident].(*types.Func)
	if !ok || fn.Pkg() == nil || !isFluentfpPath(fn.Pkg().Path()) {
		return nil, false
	}
	return fn, true
}

// hasReceiver reports whether fn is a method (has a receiver) rather than a
// package-level function.
func hasReceiver(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	return ok && sig.Recv() != nil
}

// returnsFluentfpChain reports whether fn has exactly one result whose type is
// itself a named type defined under fluentfp.
func returnsFluentfpChain(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Results().Len() != 1 {
		return false
	}
	return namedFluentfpType(sig.Results().At(0).Type()) != nil
}

// exprIsFluentfpTyped reports whether expr's static type is itself a named type
// defined under fluentfp — the generalized chain-root test that subsumes
// setup-constructor rooting (jeeves #71302: variable- and function-return-rooted
// chains).
func exprIsFluentfpTyped(pass *analysis.Pass, expr ast.Expr) bool {
	t := pass.TypesInfo.TypeOf(expr)
	if t == nil {
		return false
	}
	return namedFluentfpType(t) != nil
}

// namedFluentfpType unwraps alias and pointer and returns t's underlying named
// type when that type is defined under the fluentfp module; nil otherwise.
func namedFluentfpType(t types.Type) *types.Named {
	t = types.Unalias(t)
	if ptr, ok := t.(*types.Pointer); ok {
		t = types.Unalias(ptr.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok {
		return nil
	}
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil || !isFluentfpPath(obj.Pkg().Path()) {
		return nil
	}
	return named
}

// isFluentfpPath reports whether importPath is the fluentfp module or a package
// within it (org-qualified, so look-alikes like "notfluentfp" do not match).
func isFluentfpPath(importPath string) bool {
	return importPath == fluentfpPath || strings.HasPrefix(importPath, fluentfpPath+"/")
}

// rootLine returns the source line used as root's layout-metric position: the
// method-name identifier's line when root is itself a selector call
// (setup-constructor-rooted, or return-rooted via a qualified call), or the
// root expression's own end position otherwise (variable-rooted, or
// return-rooted via a bare-identifier call) — jeeves #71302.
func rootLine(pass *analysis.Pass, root ast.Expr) int {
	if call, ok := root.(*ast.CallExpr); ok {
		if sel := calleeSelector(call); sel != nil {
			return pass.Fset.Position(sel.Sel.Pos()).Line
		}
	}
	return pass.Fset.Position(root.End()).Line
}

// reportChain emits a diagnostic when the chain's line layout disagrees with the
// form its counted-operation count selects. The layout metric is the source line
// of each method-name identifier (not the call's End), so a multi-line argument
// (e.g. an inline lambda) never triggers a false split/collapse.
func reportChain(pass *analysis.Pass, root ast.Expr, ops []*ast.CallExpr, outer *ast.CallExpr) {
	count := len(ops)
	if count == 0 {
		return // bare setup call — nothing to enforce
	}
	line := func(c *ast.CallExpr) int {
		return pass.Fset.Position(calleeSelector(c).Sel.Pos()).Line
	}
	rootLn := rootLine(pass, root)
	opLines := make([]int, count) // source order (ops were collected outermost-first)
	for i, c := range ops {
		opLines[count-1-i] = line(c)
	}
	if count == 1 {
		if rootLn != opLines[0] {
			pass.ReportRangef(outer, msgSingleOp)
		}
		return
	}
	prev := rootLn
	for _, l := range opLines {
		if l <= prev {
			pass.ReportRangef(outer, msgMultiOp, count)
			return
		}
		prev = l
	}
}
