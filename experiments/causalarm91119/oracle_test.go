package causalarm91119_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	cx "github.com/binaryphile/go-fp-lint/experiments/causalarm91119"
)

// recorder implements analysistest.Testing (just Errorf) so we can observe
// whether an analyzer passes or fails the fixtures WITHOUT failing this test
// process directly. That inversion is what lets us assert the oracle
// discriminates: the correct reference must produce zero analysistest errors,
// the broken reference must produce at least one.
type recorder struct{ errs int }

func (r *recorder) Errorf(format string, args ...interface{}) { r.errs++ }

// TestOracle_CorrectPasses is the positive control: the known-correct
// type-aware reference must satisfy every Appendix-C `// want` marker exactly.
func TestOracle_CorrectPasses(t *testing.T) {
	rc := &recorder{}
	analysistest.Run(rc, analysistest.TestData(), cx.CorrectAnalyzer, "a")
	if rc.errs != 0 {
		t.Fatalf("oracle FAILED its known-correct reference: %d analysistest error(s) — the oracle or the fixtures are wrong (integrity gate, prereg §5)", rc.errs)
	}
}

// TestOracle_BrokenFails is the discrimination requirement (memory
// 6b8b00f61e52): the known-broken name-only reference MUST NOT pass the
// fixtures. If it did, the oracle would be too shallow to catch the exact
// name-vs-type blind spot the causal arm is about.
func TestOracle_BrokenFails(t *testing.T) {
	rc := &recorder{}
	analysistest.Run(rc, analysistest.TestData(), cx.BrokenAnalyzer, "a")
	if rc.errs == 0 {
		t.Fatalf("oracle did NOT discriminate: the name-only reference passed the fixtures — the negative-control cases (2/5b) are missing or the oracle is too shallow")
	}
}
