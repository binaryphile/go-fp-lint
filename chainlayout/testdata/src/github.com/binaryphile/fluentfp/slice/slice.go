// Package slice is a minimal stub of fluentfp's slice package for analysistest
// fixtures — enough surface (Mapper[T] + setup constructors + chain methods) for
// chainlayout to resolve chain identities and setup-result types as fluentfp.
package slice

type Mapper[T any] []T

// From and Map are setup constructors (package funcs returning a Mapper). Map is
// generic in two params so slice.Map[int, string](xs) is an *ast.IndexListExpr
// callee; From called with an explicit type arg (slice.From[int](xs)) is an
// *ast.IndexExpr callee — both exercise chainlayout's calleeSelector unwrap.
func From[T any](ts []T) Mapper[T] { return Mapper[T](ts) }

func Map[T, R any](ts []T) Mapper[R] { return nil }

func (m Mapper[T]) KeepIf(fn func(T) bool) Mapper[T] { return m }

func (m Mapper[T]) RemoveIf(fn func(T) bool) Mapper[T] { return m }

func (m Mapper[T]) ToString(fn func(T) string) []string { return nil }

func (m Mapper[T]) Len() int { return len(m) }
