// Package mapfusion detects double-map fusion: two adjacent fluentfp map
// operations that should fuse into a single pass with a composed function
// (the rule "Don't chain when a single pass suffices" in go-development-guide.md
// / functional-programming-unified-guide.md). Chaining two maps allocates an
// intermediate collection and reads worse than one pass. See docs/design.md §v13
// for the detection rule, the checked API inventory, and the deliberate
// exclusions (FlatMap, method expressions).
package mapfusion

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const fluentfpRoot = "github.com/binaryphile/fluentfp"

const message = `double map — fuse into one pass with a composed function (see go-development-guide.md "Don't chain when a single pass suffices")`

// mapMethods is the set of fluentfp map-family METHOD names (form A). Each was
// verified in the #66830 3a API inventory (docs/design.md §v13) to denote only a
// pointwise map (one mapper-fn arg, mappable result) across every fluentfp type
// that defines it (Mapper/Option/Either/Result/Seq/Stream/Entries); no fluentfp
// method with one of these names is a non-map.
var mapMethods = map[string]bool{
	"Transform": true,
	"ToAny":     true, "ToBool": true, "ToByte": true, "ToError": true,
	"ToFloat32": true, "ToFloat64": true,
	"ToInt": true, "ToInt32": true, "ToInt64": true,
	"ToRune": true, "ToString": true,
}

// standaloneMap is the explicit allowlist of fluentfp standalone map FUNCTIONS
// (form B), keyed by resolved "<pkgPath>.<Name>" → the index of the data-source
// argument. Verified data-first (index 0) and unique-per-package in the 3a
// inventory. A different or future top-level Map does NOT match unless added
// here — this is what keeps form B honestly bounded (grade r2 F1/F2).
var standaloneMap = map[string]int{
	fluentfpRoot + "/slice.Map":  0,
	fluentfpRoot + "/kv.Map":     0,
	fluentfpRoot + "/option.Map": 0,
	fluentfpRoot + "/stream.Map": 0,
	fluentfpRoot + "/either.Map": 0,
}

// Analyzer flags a fluentfp map operation whose data source is itself a fluentfp
// map operation — a fusable double map.
var Analyzer = &analysis.Analyzer{
	Name: "mapfusion",
	Doc:  "reports adjacent fluentfp maps that should fuse into one pass with a composed function (see go-development-guide.md)",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			source, ok := isMapOp(pass, call)
			if !ok {
				return true
			}
			if _, ok := isMapOp(pass, source); ok {
				// Report at the outer map's name (the second map that should
				// fuse). isMapOp already confirmed call.Fun is a *SelectorExpr.
				sel := call.Fun.(*ast.SelectorExpr)
				pass.Report(analysis.Diagnostic{Pos: sel.Sel.Pos(), Message: message})
			}
			return true
		})
	}
	return nil, nil
}

// isMapOp reports whether expr is a fluentfp map operation and, if so, its
// data-source sub-expression (already paren-stripped). Two forms:
//
//   - (A) fluent map METHOD `recv.M(fn)` — a genuine method value call
//     (types.MethodVal, which excludes the method-expression form `T.M(recv,fn)`)
//     whose method resolves to a fluentfp func named in mapMethods. Source = recv.
//   - (B) standalone map FUNCTION `pkg.Map(data, fn)` — a no-receiver func whose
//     resolved "<pkgPath>.<Name>" is in standaloneMap. Source = the arg at the
//     table's recorded index.
func isMapOp(pass *analysis.Pass, expr ast.Expr) (ast.Expr, bool) {
	call, ok := unparen(expr).(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok || fn.Pkg() == nil || !isFluentfpPkg(fn.Pkg().Path()) {
		return nil, false
	}

	// Form A: genuine method VALUE call (not a method expression, not a field).
	if selxn, ok := pass.TypesInfo.Selections[sel]; ok {
		if selxn.Kind() == types.MethodVal && mapMethods[fn.Name()] {
			return unparen(sel.X), true
		}
		return nil, false
	}

	// Form B: package-qualified standalone map function via the explicit table.
	if idx, ok := standaloneMap[fn.Pkg().Path()+"."+fn.Name()]; ok && len(call.Args) > idx {
		return unparen(call.Args[idx]), true
	}
	return nil, false
}

// isFluentfpPkg matches the fluentfp module root or any of its subpackages by
// path segment — not a bare substring (grade r1 F5), so a suffixed clone path
// does not resolve as fluentfp.
func isFluentfpPkg(path string) bool {
	return path == fluentfpRoot || strings.HasPrefix(path, fluentfpRoot+"/")
}

// unparen strips any number of enclosing parentheses (grade r1 F1).
func unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}
