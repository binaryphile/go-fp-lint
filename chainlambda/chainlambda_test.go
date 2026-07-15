package chainlambda_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/binaryphile/go-fp-lint/chainlambda"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, chainlambda.Analyzer, "a")
}
