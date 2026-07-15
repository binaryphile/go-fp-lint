// Package aliaswrite detects value-receiver methods that mutate a slice or map
// field's shared backing storage when the receiver type has no Clone() method —
// go-development-guide.md §11 Slice Aliasing Trap: value-copying a struct copies
// slice/map headers but NOT the underlying array/map, so two copies share the
// same backing. Mutating that backing through one copy silently corrupts the
// other. A Clone() method on the type signals the aliasing is handled, so its
// presence exempts the type (task heuristic, jeeves #65786).
//
// Detection is conservative (favors false-negative over false-positive, per the
// repo precedent — see recvshape). Three mutation shapes are flagged:
//   - index-assignment into a slice/map field:  r.f[i] = x
//   - delete on a map field:                     delete(r.f, k)
//   - reslice-append back to a slice field:      r.f = append(r.f[lo:hi], ...)
//
// Plain append-only (r.f = append(r.f, x) — no reslice), read-only access, and
// scalar-field assignment are NOT flagged: guide §11 says append-only does not
// require Clone(), and mutating a scalar field or a local only touches the copy.
package aliaswrite

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

const message = "%s mutates slice/map field %s through a value receiver on %s, which has no Clone() method — value copies share backing storage (go-development-guide.md §11 Slice Aliasing Trap)"

// Analyzer flags value-receiver methods that mutate a slice/map field's shared
// backing when the receiver type has no Clone() method.
var Analyzer = &analysis.Analyzer{
	Name: "aliaswrite",
	Doc:  "reports value-receiver methods that mutate a slice/map field's shared backing when the type has no Clone() method (see go-development-guide.md §11 Slice Aliasing Trap)",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || len(fd.Recv.List) == 0 || fd.Body == nil {
				continue
			}
			if _, isPtr := fd.Recv.List[0].Type.(*ast.StarExpr); isPtr {
				continue // pointer receiver — mutation is intended, not the aliasing trap
			}
			names := fd.Recv.List[0].Names
			if len(names) == 0 || names[0].Name == "_" {
				continue // blank/unnamed receiver — no identifier to trace
			}
			recvObj := pass.TypesInfo.Defs[names[0]]
			if recvObj == nil {
				continue
			}
			named, ok := recvObj.Type().(*types.Named)
			if !ok {
				continue
			}
			if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
				continue // v1 scope: generic receiver types skipped
			}
			if hasCloneMethod(named) {
				continue // aliasing presumed handled
			}
			reportFieldMutations(pass, fd, recvObj, named)
		}
	}
	return nil, nil
}

// hasCloneMethod reports whether named (value or pointer method set) has a
// method literally called "Clone".
func hasCloneMethod(named *types.Named) bool {
	ms := types.NewMethodSet(types.NewPointer(named))
	for i := 0; i < ms.Len(); i++ {
		if ms.At(i).Obj().Name() == "Clone" {
			return true
		}
	}
	return false
}

// reportFieldMutations walks fd's body and reports each slice/map field
// mutation shape (index-assign, reslice-append, delete) through the receiver.
func reportFieldMutations(pass *analysis.Pass, fd *ast.FuncDecl, recvObj types.Object, named *types.Named) {
	report := func(n ast.Node, field string) {
		pass.ReportRangef(n, message, fd.Name.Name, field, named.Obj().Name())
	}

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				// index-assign: r.f[i] = x
				if idx, ok := lhs.(*ast.IndexExpr); ok {
					if field, ok := sliceOrMapField(pass, recvObj, idx.X); ok {
						report(node, field)
						continue
					}
				}
				// reslice-append: r.f = append(r.f[lo:hi], ...)
				if field, ok := sliceOrMapField(pass, recvObj, lhs); ok && isResliceAppend(pass, recvObj, node.Rhs) {
					report(node, field)
				}
			}
		case *ast.CallExpr:
			// delete(r.f, k)
			if isBuiltin(pass, node.Fun, "delete") && len(node.Args) >= 1 {
				if field, ok := sliceOrMapField(pass, recvObj, node.Args[0]); ok {
					report(node, field)
				}
			}
		}
		return true
	})
}

// sliceOrMapField reports the field name and true when e is `<recv>.<field>`
// and the field's type underlying is a slice or map.
func sliceOrMapField(pass *analysis.Pass, recvObj types.Object, e ast.Expr) (string, bool) {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok || pass.TypesInfo.Uses[ident] != recvObj {
		return "", false
	}
	t := pass.TypesInfo.TypeOf(sel)
	if t == nil {
		return "", false
	}
	switch t.Underlying().(type) {
	case *types.Slice, *types.Map:
		return sel.Sel.Name, true
	}
	return "", false
}

// isResliceAppend reports whether rhs is a single `append(X, ...)` call whose
// first argument is a reslice (`r.f[lo:hi]`) of a receiver slice/map field —
// the guide's dangerous pattern. Plain `append(r.f, x)` (first arg is the whole
// field, not a slice expression) is append-only and returns false.
func isResliceAppend(pass *analysis.Pass, recvObj types.Object, rhs []ast.Expr) bool {
	if len(rhs) != 1 {
		return false
	}
	call, ok := rhs[0].(*ast.CallExpr)
	if !ok || !isBuiltin(pass, call.Fun, "append") || len(call.Args) == 0 {
		return false
	}
	slice, ok := call.Args[0].(*ast.SliceExpr)
	if !ok {
		return false
	}
	_, ok = sliceOrMapField(pass, recvObj, slice.X)
	return ok
}

// isBuiltin reports whether fun is the builtin named name (not a shadowing
// local of the same spelling).
func isBuiltin(pass *analysis.Pass, fun ast.Expr, name string) bool {
	ident, ok := fun.(*ast.Ident)
	if !ok || ident.Name != name {
		return false
	}
	_, ok = pass.TypesInfo.Uses[ident].(*types.Builtin)
	return ok
}
