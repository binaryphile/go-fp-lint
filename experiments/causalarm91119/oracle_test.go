package causalarm91119_test

import (
	"fmt"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	cx "github.com/binaryphile/go-fp-lint/experiments/causalarm91119"
)

// recorder implements analysistest.Testing (just Errorf) and CAPTURES the
// formatted messages, so we can observe whether an analyzer passes or fails the
// fixtures WITHOUT failing this test process — and, for the broken reference,
// assert WHY it failed (attributable to over-flagging, not an unrelated
// mismatch).
type recorder struct{ msgs []string }

func (r *recorder) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	r.msgs = append(r.msgs, msg)
}

func (r *recorder) countContaining(sub string) int {
	n := 0
	for _, m := range r.msgs {
		if strings.Contains(m, sub) {
			n++
		}
	}
	return n
}

// TestOracle_CorrectPasses is the positive control: the known-correct
// type-aware reference must satisfy every fixture want-marker exactly, across
// BOTH the positive/negative package (a) and the isolated clean package (b).
func TestOracle_CorrectPasses(t *testing.T) {
	rc := &recorder{}
	analysistest.Run(rc, analysistest.TestData(), cx.CorrectAnalyzer, "a", "b")
	if len(rc.msgs) != 0 {
		joined := strings.Join(rc.msgs, "\n")
		t.Fatalf("oracle FAILED its known-correct reference: %d analysistest error(s) — the oracle or fixtures are wrong (integrity gate, prereg §5):\n%s",
			len(rc.msgs), joined)
	}
}

// TestOracle_BrokenFails is the discrimination requirement (memory
// 6b8b00f61e52). Because the correct reference passes the SAME fixtures with
// zero errors (proven above), the fixtures are well-formed; the broken
// reference emits the same message on the true positives, so its ONLY source of
// error is the FALSE POSITIVES it raises on the negative-control cases. We
// therefore require not merely "some error" but at least one "unexpected
// diagnostic" — analysistest's phrasing for an extra (over-flagged) diagnostic —
// so the failure is attributable to the name-vs-type blind spot, not an
// unrelated mismatch.
func TestOracle_BrokenFails(t *testing.T) {
	rc := &recorder{}
	analysistest.Run(rc, analysistest.TestData(), cx.BrokenAnalyzer, "a", "b")
	if len(rc.msgs) == 0 {
		t.Fatalf("oracle did NOT discriminate: the name-only reference passed the fixtures — the negative-control cases are missing or the oracle is too shallow")
	}
	if rc.countContaining("unexpected diagnostic") == 0 {
		joined := strings.Join(rc.msgs, "\n")
		t.Fatalf("oracle discriminated for the WRONG reason: broken reference failed with no 'unexpected diagnostic' (over-flag) error — the negative controls are not exercising the false-positive path:\n%s",
			joined)
	}
}
