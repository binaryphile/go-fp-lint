package a

import "github.com/binaryphile/fluentfp/slice"

func namedPred(x int) bool     { return x > 0 }
func namedConv(x int) string   { return "" }
func namedPredS(s string) bool { return s != "" }

// ---- POSITIVES: layout disagrees with the op-count-selected form ----

// Two operations written inline — should be one-per-line.
func Pos2opInline(xs []int) int {
	return slice.From(xs).KeepIf(namedPred).Len() // want "fluentfp chain with 2 operations should be one per line"
}

// Three operations inline.
func Pos3opInline(xs []int) []string {
	return slice.From(xs).KeepIf(namedPred).RemoveIf(namedPred).ToString(namedConv) // want "fluentfp chain with 3 operations should be one per line"
}

// Two operations, partially collapsed: setup+op1 share a line, op2 below.
func PosPartial(xs []int) int {
	return slice.From(xs).KeepIf(namedPred). // want "fluentfp chain with 2 operations should be one per line"
							Len()
}

// Single operation split across lines — should be inline.
func PosSingleSplit(xs []int) []string {
	return slice.From(xs). // want "single-operation fluentfp chain should be inline"
				ToString(namedConv)
}

// Generic setup (slice.Map[int, string] — an IndexListExpr callee) rooted, two
// operations inline. Without calleeSelector's index unwrap this would silently
// escape (R1 F5).
func PosGenericInline(xs []int) int {
	return slice.Map[int, string](xs).KeepIf(namedPredS).Len() // want "fluentfp chain with 2 operations should be one per line"
}

// Parenthesized generic setup callee ((slice.Map[...]) ) rooted, two operations
// inline — calleeSelector must strip the ParenExpr wrapper (R2 F4).
func PosParenGeneric(xs []int) int {
	return (slice.Map[int, string])(xs).KeepIf(namedPredS).Len() // want "fluentfp chain with 2 operations should be one per line"
}

// ---- NEGATIVES (silent) ----

// Canonical single-operation inline.
func Neg1opInline(xs []int) []string { return slice.From(xs).ToString(namedConv) }

// Canonical two-operation one-per-line with trailing dots.
func Neg2opCorrect(xs []int) int {
	return slice.From(xs).
		KeepIf(namedPred).
		Len()
}

// Single-operation inline whose argument is a multi-line lambda. The method-name
// line metric (not the call's End) must NOT treat this as a split (R1 F2 guard).
func NegMultilineLambda(xs []int) slice.Mapper[int] {
	return slice.From(xs).KeepIf(func(x int) bool {
		return x > 0
	})
}

// Generic setup (slice.From[int] — an IndexExpr callee) rooted, correctly
// formatted one-per-line: recognized (unwrap fires) but conformant → silent.
func NegGenericCorrect(xs []int) int {
	return slice.From[int](xs).
		KeepIf(namedPred).
		Len()
}

// Non-fluentfp chain of the same shape — not our concern.
type Other struct{}

func (o Other) A() Other { return o }
func (o Other) B() int   { return 0 }

func NegNonFluent() int { return Other{}.A().B() }

// Variable-rooted fluentfp chain — out of the v1 claim (setup-constructor-rooted
// only); the executable form of the documented limitation (#71302).
func NegVarRooted(xs []int) int {
	m := slice.From(xs)
	return m.KeepIf(namedPred).Len()
}

// Bare setup call, zero counted operations.
func NegBareSetup(xs []int) { _ = slice.From(xs) }
