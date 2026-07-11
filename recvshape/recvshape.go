// Package recvshape detects methods with an unnecessary pointer receiver —
// go-development-guide.md §3 Value Semantics: default to value receivers;
// pointer receivers are for lock-containing types, interface-satisfaction
// consistency, or methods that actually mutate the receiver's own fields.
// See docs/design.md §v5 for the detection rules, the exclusion algorithms
// (ported from go vet's copylocks lockPath), and known limitations.
package recvshape

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

const message = "%s has a pointer receiver on %s but never mutates %s's fields — consider a value receiver (see go-development-guide.md §3 Value Semantics)"

// Analyzer flags pointer-receiver methods that never mutate their receiver's
// own fields and have no lock-containing or interface-satisfaction reason to
// stay pointer receivers.
var Analyzer = &analysis.Analyzer{
	Name: "recvshape",
	Doc:  "reports pointer receivers that could be value receivers (see go-development-guide.md §3 Value Semantics)",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	ifaces := collectInterfaces(pass)

	byType := map[*types.Named][]*ast.FuncDecl{}
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || len(fd.Recv.List) == 0 {
				continue
			}
			star, ok := fd.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue // value receiver — not our concern
			}
			named, ok := pass.TypesInfo.TypeOf(star.X).(*types.Named)
			if !ok {
				continue
			}
			byType[named] = append(byType[named], fd)
		}
	}

	for named, decls := range byType {
		if named.TypeParams() != nil && named.TypeParams().Len() > 0 {
			continue // v1 scope: generic receiver types skipped entirely
		}
		if typeHasLock(named) {
			continue
		}
		if typeSatisfiesInterface(named, ifaces) {
			continue
		}

		anyMutates := false
		for _, fd := range decls {
			if mutatesReceiver(pass, fd) {
				anyMutates = true
				break
			}
		}
		if anyMutates {
			continue // type-level consistency exemption
		}

		for _, fd := range decls {
			pass.ReportRangef(fd.Recv.List[0], message, fd.Name.Name, named.Obj().Name(), named.Obj().Name())
		}
	}

	return nil, nil
}

// collectInterfaces gathers every non-empty interface type lexically
// referenced anywhere in the package's own files (var decls, params,
// fields, type-asserts, etc.) via the type-checker's expression-type
// records. The empty interface (interface{}/any) is excluded — every type
// trivially implements it, so including it would disable the analyzer.
func collectInterfaces(pass *analysis.Pass) []*types.Interface {
	seen := map[types.Type]bool{}
	var out []*types.Interface
	for _, tv := range pass.TypesInfo.Types {
		t := tv.Type
		if t == nil || seen[t] {
			continue
		}
		iface, ok := t.Underlying().(*types.Interface)
		if !ok || iface.NumMethods() == 0 {
			continue
		}
		seen[t] = true
		out = append(out, iface)
	}
	return out
}

// typeSatisfiesInterface reports whether *named implements any of ifaces —
// the interface-satisfaction-consistency exclusion (decision 2).
func typeSatisfiesInterface(named *types.Named, ifaces []*types.Interface) bool {
	ptr := types.NewPointer(named)
	for _, iface := range ifaces {
		if types.Implements(ptr, iface) {
			return true
		}
	}
	return false
}

// typeHasLock reports whether named contains a lock (directly or through
// struct/array recursion) — the lock exclusion (decision 3), a port of go
// vet copylocks' unexported lockPath algorithm. Deliberately does NOT
// recurse through pointer fields: copying a struct that holds a *pointer*
// to a lock-containing type does not duplicate the lock, only the pointer
// — this matches upstream copylock exactly, not a v1 scope-narrowing.
func typeHasLock(named *types.Named) bool {
	return lockPath(named, nil)
}

func lockPath(typ types.Type, seen map[types.Type]bool) bool {
	if typ == nil || seen[typ] {
		return false
	}
	if seen == nil {
		seen = map[types.Type]bool{}
	}
	seen[typ] = true

	for {
		atyp, ok := typ.Underlying().(*types.Array)
		if !ok {
			break
		}
		typ = atyp.Elem()
	}

	styp, ok := typ.Underlying().(*types.Struct)
	if !ok {
		return false
	}

	if types.Implements(types.NewPointer(typ), lockerType) && !types.Implements(typ, lockerType) {
		return true
	}

	for i := 0; i < styp.NumFields(); i++ {
		if lockPath(styp.Field(i).Type(), seen) {
			return true
		}
	}
	return false
}

var lockerType *types.Interface

func init() {
	nullary := types.NewSignatureType(nil, nil, nil, nil, nil, false) // func()
	methods := []*types.Func{
		types.NewFunc(token.NoPos, nil, "Lock", nullary),
		types.NewFunc(token.NoPos, nil, "Unlock", nullary),
	}
	lockerType = types.NewInterface(methods, nil).Complete()
}

// mutatesReceiver reports whether fd's body plausibly mutates its own
// receiver's fields (decision: conservative — favors false-negative over
// false-positive per the repo's tight-scope precedent). A blank/unnamed
// receiver has no identifier to trace mutation through and always returns
// false (still subject to the type-level exclusions above).
func mutatesReceiver(pass *analysis.Pass, fd *ast.FuncDecl) bool {
	names := fd.Recv.List[0].Names
	if len(names) == 0 || names[0].Name == "_" {
		return false
	}
	recvObj := pass.TypesInfo.Defs[names[0]]
	if recvObj == nil || fd.Body == nil {
		return false
	}

	isReceiverSelector := func(e ast.Expr) bool {
		sel, ok := e.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		ident, ok := sel.X.(*ast.Ident)
		return ok && pass.TypesInfo.Uses[ident] == recvObj
	}
	isReceiverDeref := func(e ast.Expr) bool {
		star, ok := e.(*ast.StarExpr)
		if !ok {
			return false
		}
		ident, ok := star.X.(*ast.Ident)
		return ok && pass.TypesInfo.Uses[ident] == recvObj
	}

	mutates := false
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if mutates {
			return false
		}
		switch node := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				if isReceiverSelector(lhs) || isReceiverDeref(lhs) {
					mutates = true
					return false
				}
			}
		case *ast.IncDecStmt:
			if isReceiverSelector(node.X) {
				mutates = true
				return false
			}
		case *ast.UnaryExpr:
			if node.Op == token.AND && isReceiverSelector(node.X) {
				mutates = true
				return false
			}
		}
		return true
	})
	return mutates
}
