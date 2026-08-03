// Package contractcheck flags a SummarizeActiveUsers function that does not
// reach a fluentfp/slice.Mapper chain call (KeepIf or Map) in CFG-reachable
// code. This is jeeves #87588's structural contract check: a candidate that
// routes around the fluentfp chain library, or only mentions it in a
// comment or unreachable branch, is a contract violation distinct from a
// functional failure. Reuses go-fp-lint's own type-name + exact-package
// receiver-resolution pattern (mirrors mapfusion/#91119's reference
// analyzers) rather than a bare grep, and CFG.Live to exclude dead code
// (golang.org/x/tools/go/cfg).
package contractcheck

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/cfg"
)

// permittedMapperPkgs is the exact, closed set of packages a genuine
// fluentfp/slice.Mapper may be defined in -- this vehicle's stub defines it
// directly in fluentfp/slice; real fluentfp aliases it to
// fluentfp/internal/base (still checked for robustness, though this
// vehicle never exercises that path -- see fixtures/fluentfp-stub/). A
// broad fluentfp/* PREFIX match (the R1 shape) let a hypothetical
// fluentfp/slice/evil.Mapper also satisfy the contract (IMPL-grade R2
// finding 1) -- closed by enumerating the exact legitimate packages
// instead of prefix-matching the module root.
var permittedMapperPkgs = map[string]bool{
	"github.com/binaryphile/fluentfp/slice":         true,
	"github.com/binaryphile/fluentfp/internal/base": true,
}

var Analyzer = &analysis.Analyzer{
	Name: "contractcheck",
	Doc:  "checks that SummarizeActiveUsers reaches a fluentfp/slice.Mapper chain call in CFG-reachable code",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "SummarizeActiveUsers" || fn.Body == nil {
				continue
			}
			if !reachesFluentChainCall(pass, fn.Body) {
				pass.Reportf(fn.Pos(), "SummarizeActiveUsers must use the project's fluentfp/slice chain methods (KeepIf/Map) in reachable code — contract violation")
			}
		}
	}
	return nil, nil
}

// reachesFluentChainCall reports whether `body` contains at least one
// CFG-reachable call to KeepIf or Map on a receiver whose static type is
// exactly fluentfp's slice.Mapper. (C)
func reachesFluentChainCall(pass *analysis.Pass, body *ast.BlockStmt) bool {
	g := cfg.New(body, func(*ast.CallExpr) bool { return true })

	for _, block := range g.Blocks {
		if !block.Live {
			continue
		}
		for _, node := range block.Nodes {
			if nodeReachesFluentChainCall(pass, node) {
				return true
			}
		}
	}
	return false
}

// nodeReachesFluentChainCall walks `n` for a qualifying chain call, treating
// a function literal's body as reachable ONLY when the literal is
// immediately invoked (its enclosing node is a call expression with the
// literal as the callee) -- so `func(){ ... }()` is inspected like any
// other reachable code, but a merely-DEFINED, uninvoked closure (`var _ =
// func(){ ... }`) is not credited (IMPL-grade R2 finding 2's fix, without
// R2 finding 13's overcorrection that also rejected invoked closures).
// Closures assigned to a variable and invoked at a LATER, separate call
// site are a documented residual scope boundary -- this is a structural,
// not data-flow, check, matching #91119's own precedent of naming call-
// expression-only forms as a coherent boundary rather than a gap.
func nodeReachesFluentChainCall(pass *analysis.Pass, n ast.Node) bool {
	found := false
	var visit func(ast.Node) bool
	visit = func(n ast.Node) bool {
		if found {
			return false
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if isFluentChainCall(pass, call) {
				found = true
				return false
			}
			if lit, ok := call.Fun.(*ast.FuncLit); ok {
				// Immediately-invoked: walk the closure's body now: it
				// executes as part of this reachable call.
				ast.Inspect(lit.Body, visit)
				if found {
					return false
				}
			}
		}
		if _, isFuncLit := n.(*ast.FuncLit); isFuncLit {
			// A bare (non-immediately-invoked) closure: already handled
			// above if it was an IIFE; otherwise its body is not
			// reachable via this statement alone.
			return false
		}
		return true
	}
	ast.Inspect(n, visit)
	return found
}

// isFluentChainCall reports whether `n` is a call to KeepIf or Map whose
// receiver's static type resolves exactly to fluentfp's slice.Mapper --
// both the type NAME ("Mapper") and the defining package (fluentfpRoot or a
// package beneath it, robust to slice.Mapper's real-module aliasing to
// internal/base). Resolves via Selections first (handles promoted/embedded
// methods and aliases, matching go-fp-lint's own reference-analyzer
// pattern), falling back to Uses. (C)
func isFluentChainCall(pass *analysis.Pass, n ast.Node) bool {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if sel.Sel.Name != "KeepIf" && sel.Sel.Name != "Map" {
		return false
	}
	fn := resolveMethod(pass, sel)
	return isFluentSliceMapper(fn)
}

// resolveMethod resolves a selector to the *types.Func it calls, using
// Selections first (handles promoted/embedded methods and aliases) then
// Uses.
func resolveMethod(pass *analysis.Pass, sel *ast.SelectorExpr) *types.Func {
	if seln := pass.TypesInfo.Selections[sel]; seln != nil {
		if fn, ok := seln.Obj().(*types.Func); ok {
			return fn
		}
	}
	fn, _ := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	return fn
}

// isFluentSliceMapper reports whether fn is a method whose RECEIVER's
// static type is exactly named "Mapper" and defined in the fluentfp
// module -- both the type name AND the package check, closing the gap
// where any fluentfp-rooted type's same-named method would otherwise
// satisfy the contract.
func isFluentSliceMapper(fn *types.Func) bool {
	if fn == nil {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	named := namedOf(sig.Recv().Type())
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Name() == "Mapper" && permittedMapperPkgs[named.Obj().Pkg().Path()]
}

// namedOf strips a leading pointer and returns the *types.Named, or nil.
func namedOf(t types.Type) *types.Named {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	named, _ := t.(*types.Named)
	return named
}
