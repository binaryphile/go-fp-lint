package causalarm91119_test

import (
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	cx "github.com/binaryphile/go-fp-lint/experiments/causalarm91119"
)

// These tests exercise the SAME mechanical scoring adapter (cx.Score) the
// #93569 dispatch uses to score arbitrary delegate analyzers — so "scoring is
// mechanical" is demonstrated, not asserted (R2 F7/New-3). The two reference
// analyzers are the discrimination pair (memory 6b8b00f61e52).

// TestOracle_CorrectPasses is the positive control: the known-correct
// type-aware reference must PASS the frozen fixtures (exact-message markers,
// packages a + b), scored via the adapter.
func TestOracle_CorrectPasses(t *testing.T) {
	pass, mismatches := cx.Score(analysistest.TestData(), cx.CorrectAnalyzer)
	if !pass {
		joined := strings.Join(mismatches, "\n")
		t.Fatalf("oracle FAILED its known-correct reference (%d mismatch(es)) — the oracle or fixtures are wrong (integrity gate, prereg §5):\n%s",
			len(mismatches), joined)
	}
}

// TestOracle_BrokenFails is the discrimination requirement: the known-broken
// name-only reference must FAIL — and specifically via an "unexpected
// diagnostic" (over-flag on the negative controls), so the failure is
// attributable to the name-vs-type blind spot, not an unrelated mismatch (F6).
// Because the correct reference passes the same fixtures with zero mismatches,
// the fixtures are well-formed and the broken reference's only error source is
// its false positives.
func TestOracle_BrokenFails(t *testing.T) {
	pass, mismatches := cx.Score(analysistest.TestData(), cx.BrokenAnalyzer)
	if pass {
		t.Fatalf("oracle did NOT discriminate: the name-only reference passed the fixtures — negative controls missing or oracle too shallow")
	}
	overFlag := 0
	for _, m := range mismatches {
		if strings.Contains(m, "unexpected diagnostic") {
			overFlag++
		}
	}
	if overFlag == 0 {
		joined := strings.Join(mismatches, "\n")
		t.Fatalf("oracle discriminated for the WRONG reason: broken reference failed with no 'unexpected diagnostic' (over-flag) error:\n%s",
			joined)
	}
}
