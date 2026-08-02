// Package fluentmap (reference: correct) re-exports the oracle's type-aware
// reference analyzer as the frozen delegate contract, for the integrity replay
// and the per-slot scorer-control. NOT a delegate output.
package fluentmap

import (
	"golang.org/x/tools/go/analysis"
	oracle "github.com/binaryphile/go-fp-lint/experiments/causalarm91119"
)

var Analyzer *analysis.Analyzer = oracle.CorrectAnalyzer
