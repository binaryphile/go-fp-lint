package a

import "github.com/binaryphile/fluentfp/slice"

func namedPred(x int) bool  { return x > 0 }
func namedConv(x int) string { return "" }

// --- positives: inline lambda passed directly to a fluentfp chain method ---

func Pos1(xs []int) slice.Mapper[int] {
	return slice.From(xs).KeepIf(func(x int) bool { return x > 0 }) // want "inline lambda passed to fluentfp chain method KeepIf"
}

func Pos2(xs []int) []string {
	return slice.From(xs).ToString(func(x int) string { return "" }) // want "inline lambda passed to fluentfp chain method ToString"
}

func Pos3(xs []int) slice.Mapper[int] {
	return slice.From(xs).RemoveIf(func(x int) bool { return x < 0 }) // want "inline lambda passed to fluentfp chain method RemoveIf"
}

// --- negatives ---

// Named function / method reference — the recommended form.
func Neg1(xs []int) slice.Mapper[int] { return slice.From(xs).KeepIf(namedPred) }
func Neg2(xs []int) []string          { return slice.From(xs).ToString(namedConv) }

// Inline lambda on a NON-fluentfp method — not our concern.
type Other struct{}

func (o Other) Do(fn func(int) bool) {}

func Neg3() { Other{}.Do(func(x int) bool { return true }) }

// A lambda that is not an argument to a fluentfp chain method (plain assignment).
func Neg4() { f := func(x int) bool { return true }; _ = f }
