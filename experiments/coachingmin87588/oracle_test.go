package coachingmin87588_test

import (
	"strings"
	"testing"

	cm "github.com/binaryphile/go-fp-lint/experiments/coachingmin87588"
)

// TestOracle_CorrectPasses is the positive control: the known-correct
// reference must PASS every frozen case, scored via the same Score adapter
// score.sh's generated module-wiring test calls.
func TestOracle_CorrectPasses(t *testing.T) {
	pass, mismatches := cm.Score(cm.RefCorrect)
	if !pass {
		t.Fatalf("oracle FAILED its known-correct reference (%d mismatch(es)) — the oracle or fixtures are wrong (integrity gate):\n%s",
			len(mismatches), strings.Join(mismatches, "\n"))
	}
}

// TestOracle_BrokenFails is the discrimination requirement: the known-broken
// (order-reversing) reference must FAIL, and specifically on the
// "mixed order preserved" case.
func TestOracle_BrokenFails(t *testing.T) {
	pass, mismatches := cm.Score(cm.RefBroken)
	if pass {
		t.Fatal("oracle did NOT discriminate: the order-reversing reference passed every case")
	}
	found := false
	for _, m := range mismatches {
		if strings.HasPrefix(m, "mixed order preserved:") {
			found = true
		}
	}
	if !found {
		t.Fatalf("oracle discriminated for the WRONG reason: broken reference failed with no 'mixed order preserved' mismatch:\n%s",
			strings.Join(mismatches, "\n"))
	}
}
