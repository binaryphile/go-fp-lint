package chainlayout_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/binaryphile/go-fp-lint/chainlayout"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, chainlayout.Analyzer, "a")
}
