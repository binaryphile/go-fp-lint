// Package internalmock detects hand-rolled mock types (named MockX) whose
// correlated target type X is defined within the SAME module as the mock
// itself — go-development-guide.md §6 Mocks = Design Smell: mocking an
// intra-system collaborator means the design should be fixed (extract pure
// domain logic) rather than papered over with a mock. Mocking a type from an
// EXTERNAL module (a real system boundary — email gateway, payment API,
// third-party client) is fine and is not flagged.
//
// Detection (task jeeves #65785, follow-up to filterloop v1 #62380):
//  1. Find a struct type named "Mock<X>" (PascalCase suffix).
//  2. Look up a type literally named "<X>" — first in the mock's own
//     package, then in each imported package — via pass.TypesInfo /
//     import-path inspection, not text matching.
//  3. Compare the module root (derived from the import path) of the mock's
//     own package against the target type's package. Same root -> smell.
//
// Module root heuristic: an import path whose first slash-segment contains
// a "." (e.g. "github.com/org/repo/pkg") is treated as VCS-hosted; the root
// is its first three segments (the repo). A path with no dot in its first
// segment (stdlib, or a synthetic single-segment test package) is its own
// root in full — this is conservative: it never claims two undotted paths
// are the same module unless they are identical.
//
// Conservative by design (favors false-negative over false-positive, per
// repo precedent — see aliaswrite/recvshape): if the correlated target type
// can't be found anywhere reachable from the pass, nothing is reported.
//
// v1 scope: correlation is by NAME only ("Mock<X>" <-> "<X>"), not by
// verifying the mock's method set actually implements <X>. A same-module
// type coincidentally sharing the stripped name would false-positive; this
// is accepted for v1 (the naming convention this rule targets is itself the
// repo's mocking convention — see filterloop v1 #62380's follow-up scope).
package internalmock

import (
	"go/ast"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const message = "%s mocks %s, which is defined within this module (%s) — go-development-guide.md §6: intra-system mocks are a design smell, extract pure domain logic instead of mocking the collaborator"

var mockNameRe = regexp.MustCompile(`^Mock([A-Z][A-Za-z0-9]*)$`)

// Analyzer flags Mock<X> struct types whose correlated <X> type is defined
// within the same module as the mock (see go-development-guide.md §6).
var Analyzer = &analysis.Analyzer{
	Name: "internalmock",
	Doc:  "reports Mock<X> types whose target <X> is defined in the same module (see go-development-guide.md §6 Mocks = Design Smell)",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok.String() != "type" {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, isStruct := ts.Type.(*ast.StructType); !isStruct {
					continue
				}
				m := mockNameRe.FindStringSubmatch(ts.Name.Name)
				if m == nil {
					continue
				}
				checkMock(pass, ts, m[1])
			}
		}
	}
	return nil, nil
}

// checkMock looks up targetName (the "<X>" in "Mock<X>") and reports if it
// resolves to a type defined within the mock's own module.
func checkMock(pass *analysis.Pass, ts *ast.TypeSpec, targetName string) {
	target := lookupType(pass, targetName)
	if target == nil {
		return // no correlated type found anywhere reachable — conservative no-op
	}
	mockRoot := moduleRoot(pass.Pkg.Path())
	targetRoot := moduleRoot(target.Pkg().Path())
	if mockRoot != targetRoot {
		return // different module — a real system boundary, not the smell
	}
	pass.ReportRangef(ts, message, ts.Name.Name, targetName, mockRoot)
}

// lookupType finds a package-level type object named name, checking the
// mock's own package first, then every directly imported package.
func lookupType(pass *analysis.Pass, name string) types.Object {
	if obj := pass.Pkg.Scope().Lookup(name); obj != nil {
		if _, ok := obj.(*types.TypeName); ok {
			return obj
		}
	}
	for _, imp := range pass.Pkg.Imports() {
		if obj := imp.Scope().Lookup(name); obj != nil {
			if _, ok := obj.(*types.TypeName); ok {
				return obj
			}
		}
	}
	return nil
}

// moduleRoot derives a comparable "module identity" from an import path.
// VCS-hosted paths (first segment contains a dot) collapse to their first
// three segments (host/org/repo); everything else (stdlib, synthetic
// single-segment test packages) is its own root in full.
func moduleRoot(pkgPath string) string {
	segs := strings.Split(pkgPath, "/")
	if !strings.Contains(segs[0], ".") {
		return pkgPath
	}
	if len(segs) <= 3 {
		return pkgPath
	}
	return strings.Join(segs[:3], "/")
}
