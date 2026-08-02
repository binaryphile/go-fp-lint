package contractcheck_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/binaryphile/go-fp-lint/experiments/coachingmin87588/contractcheck"
)

// TestAnalyzer runs the three testdata packages: a (compliant, reachable
// KeepIf/Map on fluentfp), b (comment + dead-code mention only), c
// (chain-shaped but non-fluentfp type). analysistest resolves each
// package's `// want` annotations (or absence) against the reported
// diagnostics.
func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, contractcheck.Analyzer, "a", "b", "c")
}
