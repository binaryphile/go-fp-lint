package contractcheck_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/binaryphile/go-fp-lint/experiments/coachingmin87588/contractcheck"
)

// TestAnalyzer runs the seven testdata packages: a (compliant, reachable
// KeepIf/Map on fluentfp), b (comment + dead-code mention only), c
// (chain-shaped but non-fluentfp type), d (chain call inside an uninvoked
// function literal), e (chain-shaped but a DIFFERENT fluentfp-rooted type,
// slice.Entries, not slice.Mapper), f (compliant: chain call inside an
// IMMEDIATELY-INVOKED closure), g (chain-shaped but a fluentfp SUBPACKAGE
// type outside the permitted set, slice/evil.Mapper). analysistest
// resolves each package's `// want` annotations (or absence) against the
// reported diagnostics.
func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, contractcheck.Analyzer, "a", "b", "c", "d", "e", "f", "g")
}
