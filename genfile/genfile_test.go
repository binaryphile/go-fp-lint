package genfile_test

import (
	"go/ast"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/binaryphile/go-fp-lint/genfile"
)

// declReporter is a toy analyzer that reports every function declaration.
// Standing in for a real go-fp-lint analyzer -- SkipGenerated's own
// contract (suppress diagnostics from files carrying the canonical
// generated-code marker) is independent of what a wrapped analyzer
// actually looks for, so a minimal fixture analyzer is the right
// integration-test boundary (Khorikov: test the wrapper's observable
// behavior, don't couple this suite to any one real analyzer's rules).
var declReporter = &analysis.Analyzer{
	Name: "declreporter",
	Doc:  "reports every function declaration (test fixture only)",
	Run:  runDeclReporter,
}

func runDeclReporter(pass *analysis.Pass) (interface{}, error) {
	for _, f := range pass.Files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			pass.Reportf(fn.Pos(), "found decl")
		}
	}
	return nil, nil
}

// TestSkipGenerated wraps declReporter with genfile.SkipGenerated and
// runs it against a fixture package with one hand-authored file
// (diagnostic expected) and one file carrying the canonical
// "// Code generated ... DO NOT EDIT." marker (diagnostic suppressed).
// The `// want` comments in testdata/src/a encode the expectation:
// present in plain.go, absent from gen.go.
func TestSkipGenerated(t *testing.T) {
	testdata := analysistest.TestData()
	wrapped := genfile.SkipGenerated(declReporter)
	analysistest.Run(t, testdata, wrapped, "a")
}
