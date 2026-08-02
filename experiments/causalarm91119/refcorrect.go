// Package causalarm91119 is the frozen hidden oracle for the jeeves #91119
// randomized normal-brief-vs-technique-brief causal arm (preregistration:
// jeeves investigations/2026-08-01-model-routing-causal-arm-preregistration.md,
// §5 + Appendix C). It holds the reference-correct (type-aware) and
// reference-broken (name-only) fluentmap analyzers plus the discrimination test
// that validates the oracle before it scores any delegate output.
//
// CONTRACT (prereg §3, frozen): flag a CALL expression recv.Map(fn) whose
// RECEIVER's static type is the fluentfp slice type (github.com/binaryphile/
// fluentfp/slice.Mapper) — resolved via go/types, including through type
// aliases, renamed imports, and embedding/promotion. Never flag a same-named
// Map on any other type, and never flag a Map method belonging to a different
// fluentfp type. The diagnostic message is exactly
// "fluent Map call: <recv-type-path>.<recv-type-name>". Method values and
// method expressions (non-call forms) are OUT of the vehicle's scope (§3 targets
// call expressions); this is a coherent contract boundary, not a gap.
//
// NOTE FOR DELEGATE DISPATCH (#93569 §6): this directory is the oracle's ground
// truth and MUST NOT be visible to the dispatched delegates. Delegates work in a
// fresh SHALLOW CLONE at a PRE-ORACLE commit (NOT a git worktree — a worktree
// shares the .git object store, leaving this directory reachable via git show),
// and the oracle is mounted independently at scoring time. Verify the base
// predates the oracle by the oracle file being ABSENT there
// (`! git cat-file -e <base>:experiments/causalarm91119/refcorrect.go`) — a
// stronger check than `merge-base --is-ancestor`, which is also true for the
// oracle commit itself.
package causalarm91119

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

const (
	fluentSlicePkg  = "github.com/binaryphile/fluentfp/slice"
	fluentSliceType = "Mapper"
	mapMethod       = "Map"
)

// CorrectAnalyzer is the reference type-aware implementation. Its FLAGGING
// decision resolves the called method to a *types.Func and checks that the
// method's RECEIVER type is fluentfp's slice.Mapper (name + exact package path)
// — not merely that the method is named Map, and not merely that it lives
// somewhere under the fluentfp namespace. This is the oracle's known-correct
// reference (the discrimination test requires it to PASS the fixtures).
var CorrectAnalyzer = &analysis.Analyzer{
	Name: "fluentmapcorrect",
	Doc:  "reference-correct type-aware fluentmap analyzer (jeeves #91119 causal-arm oracle)",
	Run:  runCorrect,
}

func runCorrect(pass *analysis.Pass) (interface{}, error) {
	forEachMapCall(pass, func(call *ast.CallExpr, sel *ast.SelectorExpr, fn *types.Func) {
		if isFluentSliceMap(fn) {
			pass.ReportRangef(call, "fluent Map call: %s", recvTypeString(fn))
		}
	})
	return nil, nil
}

// forEachMapCall visits every call expression of the form recv.Map(...) and
// invokes visit with the resolved *types.Func (or nil if unresolved). Shared by
// both reference analyzers so they differ ONLY in the flagging decision.
func forEachMapCall(pass *analysis.Pass, visit func(*ast.CallExpr, *ast.SelectorExpr, *types.Func)) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != mapMethod {
				return true
			}
			fn := resolveMethod(pass, sel)
			visit(call, sel, fn)
			return true
		})
	}
}

// resolveMethod resolves a selector to the *types.Func it calls, using
// Selections first (handles promoted/embedded methods and aliases) then Uses.
func resolveMethod(pass *analysis.Pass, sel *ast.SelectorExpr) *types.Func {
	if seln := pass.TypesInfo.Selections[sel]; seln != nil {
		if fn, ok := seln.Obj().(*types.Func); ok {
			return fn
		}
	}
	fn, _ := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	return fn
}

// isFluentSliceMap reports whether fn is the Map method whose RECEIVER type is
// fluentfp's slice.Mapper. Checking the receiver type (not the method's own
// defining package) is what distinguishes slice.Mapper.Map from a hypothetical
// same-named Map on a different fluentfp type.
func isFluentSliceMap(fn *types.Func) bool {
	if fn == nil || fn.Name() != mapMethod {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	named := namedOf(sig.Recv().Type())
	if named == nil {
		return false
	}
	obj := named.Obj()
	return obj != nil && obj.Name() == fluentSliceType && obj.Pkg() != nil && obj.Pkg().Path() == fluentSlicePkg
}

// recvTypeString renders the resolved receiver type as "<pkgpath>.<name>",
// stable across type arguments, for the diagnostic message.
func recvTypeString(fn *types.Func) string {
	if fn == nil {
		return "?"
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return "?"
	}
	named := namedOf(sig.Recv().Type())
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return "?"
	}
	return named.Obj().Pkg().Path() + "." + named.Obj().Name()
}

// namedOf strips a leading pointer and returns the *types.Named, or nil.
func namedOf(t types.Type) *types.Named {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	named, _ := t.(*types.Named)
	return named
}
