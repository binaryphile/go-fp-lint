package causalarm91119

import (
	"fmt"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
)

// DelegateContract (frozen, prereg §3): a scored delegate MUST deliver a Go
// package that exports `var Analyzer *analysis.Analyzer` implementing the
// fluentmap vehicle. The #93569 dispatch scores each delegate mechanically by
// importing that package and calling Score() with its Analyzer — no per-delegate
// bespoke harnessing, no human adjudication.
const DelegateContract = "delegate package MUST export: var Analyzer *analysis.Analyzer"

// scoreRecorder captures analysistest failures so an analyzer can be scored
// WITHOUT failing a test process (analysistest.Testing is just Errorf).
type scoreRecorder struct{ msgs []string }

func (r *scoreRecorder) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	r.msgs = append(r.msgs, msg)
}

// Score runs an ARBITRARY analyzer against the frozen App-C fixture set
// (packages "a" and "b" under testdataDir) and returns whether it PASSES —
// exactly the expected diagnostics, none missing and none extra — plus any
// mismatch messages. This is the oracle's mechanical scoring adapter: the
// #93569 dispatch calls Score(dir, delegatePkg.Analyzer) per delegate. Pass
// analysistest.TestData() for testdataDir from a test in this package, or the
// absolute path to this package's testdata/ from an external harness.
func Score(testdataDir string, a *analysis.Analyzer) (pass bool, mismatches []string) {
	rc := &scoreRecorder{}
	analysistest.Run(rc, testdataDir, a, "a", "b")
	return len(rc.msgs) == 0, rc.msgs
}
