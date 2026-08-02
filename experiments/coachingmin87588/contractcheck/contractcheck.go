// Package contractcheck flags a SummarizeActiveUsers function that does not
// reach a fluentfp/slice.Mapper chain call (KeepIf or Map) in CFG-reachable
// code. This is jeeves #87588's structural contract check: a candidate that
// routes around the fluentfp chain library, or only mentions it in a
// comment or unreachable branch, is a contract violation distinct from a
// functional failure. Reuses go-fp-lint's own package-prefix + type-name
// receiver-resolution pattern (mapfusion.isFluentfpPkg) rather than a bare
// grep, and CFG.Live to exclude dead code (golang.org/x/tools/go/cfg).
package contractcheck

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/cfg"
)

const fluentfpRoot = "github.com/binaryphile/fluentfp"

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
// defined in the fluentfp module. (C)
func reachesFluentChainCall(pass *analysis.Pass, body *ast.BlockStmt) bool {
	g := cfg.New(body, func(*ast.CallExpr) bool { return true })

	for _, block := range g.Blocks {
		if !block.Live {
			continue
		}
		for _, node := range block.Nodes {
			found := false
			ast.Inspect(node, func(n ast.Node) bool {
				if found {
					return false
				}
				if isFluentChainCall(pass, n) {
					found = true
					return false
				}
				return true
			})
			if found {
				return true
			}
		}
	}
	return false
}

// isFluentChainCall reports whether `n` is a call to KeepIf or Map whose
// receiver's defining package is fluentfp. (C)
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
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	return ok && fn.Pkg() != nil && isFluentfpPkg(fn.Pkg().Path())
}

// isFluentfpPkg reports whether `path` is the fluentfp module or a package
// beneath it (mirrors mapfusion.isFluentfpPkg — the same check go-fp-lint's
// own shipped analyzers already use to resolve a receiver as fluentfp,
// robust to slice.Mapper's real-module aliasing to internal/base). (C)
func isFluentfpPkg(path string) bool {
	return path == fluentfpRoot || len(path) > len(fluentfpRoot) && path[:len(fluentfpRoot)+1] == fluentfpRoot+"/"
}
