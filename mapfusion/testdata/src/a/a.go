package a

import (
	"github.com/binaryphile/fluentfp/option"
	"github.com/binaryphile/fluentfp/slice"
)

func addOne(x int) int    { return x + 1 }
func idInt(x int) int     { return x }
func toStr(x int) string  { return "" }
func isPos(x int) bool    { return x > 0 }
func toSlice(x int) []int { return nil }

// ---------------- positives ----------------

// A∘A fluent chain, lambdas.
func Pos1(xs []int) slice.Mapper[int] {
	return slice.From(xs).Transform(func(x int) int { return x }).Transform(func(x int) int { return x }) // want `double map`
}

// A∘A cross-type (ToInt then ToString).
func Pos2(xs []int) slice.Mapper[string] {
	return slice.From(xs).ToInt(idInt).ToString(toStr) // want `double map`
}

// A∘A mixed method names (Transform then ToInt).
func Pos3(xs []int) slice.Mapper[int] {
	return slice.From(xs).Transform(addOne).ToInt(idInt) // want `double map`
}

// A∘A with NAMED functions — fusion is shape-based, so still flagged (r1 F2).
func Pos4(xs []int) slice.Mapper[int] {
	return slice.From(xs).Transform(addOne).Transform(addOne) // want `double map`
}

// Parenthesized receiver — unparen strips it (r1 F1).
func Pos5(xs []int) slice.Mapper[int] {
	return (slice.From(xs).Transform(addOne)).Transform(addOne) // want `double map`
}

// Standalone-nested (B∘B) — the task's literal shape.
func Pos6(xs []int) slice.Mapper[string] {
	return slice.Map(slice.Map(xs, idInt), toStr) // want `double map`
}

// Parenthesized standalone source (B∘B + paren).
func Pos7(xs []int) slice.Mapper[string] {
	return slice.Map((slice.Map(xs, idInt)), toStr) // want `double map`
}

// Mixed A∘B — fluent Transform whose receiver is a standalone slice.Map.
func Pos8(xs []int) slice.Mapper[int] {
	return slice.Map(xs, idInt).Transform(addOne) // want `double map`
}

// Mixed B∘A — standalone slice.Map whose source arg is a fluent Transform.
func Pos9(xs []int) slice.Mapper[string] {
	return slice.Map(slice.From(xs).Transform(addOne), toStr) // want `double map`
}

// Second form-B table entry (option.Map nested).
func Pos10(o option.Option[int]) option.Option[int] {
	return option.Map(option.Map(o, idInt), idInt) // want `double map`
}

// Triple map — one report per adjacent pair (2 total), on distinct lines.
func Pos11(xs []int) slice.Mapper[int] {
	return slice.From(xs).
		Transform(addOne).
		Transform(addOne). // want `double map`
		Transform(addOne)  // want `double map`
}

// Explicitly-instantiated generic standalone maps — callee is IndexListExpr,
// must still be caught (IMPL grade).
func Pos12(xs []int) slice.Mapper[string] {
	return slice.Map[int, string](slice.Map[int, int](xs, idInt), toStr) // want `double map`
}

// Parenthesized callee wrapping an instantiation — (slice.Map[…])(...).
func Pos13(xs []int) slice.Mapper[string] {
	return (slice.Map[int, string])((slice.Map[int, int])(xs, idInt), toStr) // want `double map`
}

// ---------------- negatives ----------------

// Single map (lambda + named + standalone) — nothing to fuse.
func Neg1(xs []int) slice.Mapper[int]    { return slice.From(xs).Transform(addOne) }
func Neg2(xs []int) slice.Mapper[string] { return slice.Map(xs, toStr) }

// Filter before a map — KeepIf is not a map op.
func Neg3(xs []int) slice.Mapper[int] { return slice.From(xs).KeepIf(isPos).Transform(addOne) }

// Filter BETWEEN two maps — not adjacent, genuinely cannot fuse.
func Neg4(xs []int) slice.Mapper[int] {
	return slice.From(xs).Transform(addOne).KeepIf(isPos).Transform(addOne)
}

// Two non-map methods.
func Neg5(xs []int) slice.Mapper[int] { return slice.From(xs).KeepIf(isPos).RemoveIf(isPos) }

// FlatMap is deliberately excluded (flatMap g . flatMap f != flatMap(g∘f)).
func Neg6(xs []int) slice.Mapper[int] { return slice.From(xs).FlatMap(toSlice).FlatMap(toSlice) }

// Non-fluentfp user type with same-named methods — type-resolution rejects it.
type Other[T any] struct{}

func (o Other[T]) Transform(fn func(T) T) Other[T] { return o }

func Neg7(o Other[int]) Other[int] { return o.Transform(addOne).Transform(addOne) }

// Local (non-fluentfp) Map function nested — not in the table.
func localMap(xs []int, fn func(int) int) []int { return xs }

func Neg8(xs []int) []int { return localMap(localMap(xs, idInt), idInt) }

// Method EXPRESSION double-map — form A requires MethodVal, so this is
// intentionally NOT flagged in v1 (documented, r2 F3).
func Neg9(m slice.Mapper[int]) slice.Mapper[int] {
	return slice.Mapper[int].Transform(slice.Mapper[int].Transform(m, addOne), addOne)
}
