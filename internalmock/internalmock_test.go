package internalmock_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/binaryphile/go-fp-lint/internalmock"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, internalmock.Analyzer, "a")
}
