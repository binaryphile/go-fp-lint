// Package slice is a minimal stub of fluentfp's slice package for analysistest
// fixtures — enough surface (Mapper[T], From, the map-family methods, a couple
// non-map methods, and the standalone Map) for mapfusion to resolve defining
// packages and for the fixture chains to type-check. Map-family methods return
// Mapper[X] so chains like .ToInt(f).ToString(g) compile.
package slice

type Mapper[T any] []T

func From[T any](ts []T) Mapper[T] { return Mapper[T](ts) }

// map-family methods (form A)
func (m Mapper[T]) Transform(fn func(T) T) Mapper[T]          { return m }
func (m Mapper[T]) ToInt(fn func(T) int) Mapper[int]          { return nil }
func (m Mapper[T]) ToString(fn func(T) string) Mapper[string] { return nil }
func (m Mapper[T]) ToBool(fn func(T) bool) Mapper[bool]       { return nil }

// non-map methods (must NOT be treated as maps)
func (m Mapper[T]) KeepIf(fn func(T) bool) Mapper[T]   { return m }
func (m Mapper[T]) RemoveIf(fn func(T) bool) Mapper[T] { return m }
func (m Mapper[T]) FlatMap(fn func(T) []T) Mapper[T]   { return m }

// standalone map function (form B), data-first
func Map[T, R any](ts []T, fn func(T) R) Mapper[R] {
	out := make(Mapper[R], len(ts))
	for i, t := range ts {
		out[i] = fn(t)
	}
	return out
}
