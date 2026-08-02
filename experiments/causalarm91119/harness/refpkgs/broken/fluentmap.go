// Package fluentmap (reference: broken) re-exports the oracle's name-only
// reference analyzer, for the integrity replay (must FAIL). NOT a delegate output.
package fluentmap

import (
	"golang.org/x/tools/go/analysis"
	oracle "github.com/binaryphile/go-fp-lint/experiments/causalarm91119"
)

var Analyzer *analysis.Analyzer = oracle.BrokenAnalyzer
