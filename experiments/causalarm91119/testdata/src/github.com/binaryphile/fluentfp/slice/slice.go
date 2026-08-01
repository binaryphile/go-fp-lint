// Package slice is a minimal stub of fluentfp's slice package for the #91119
// causal-arm oracle fixtures — enough surface (Mapper[T] + a Map method) for the
// reference analyzers to resolve a receiver's Map method to its fluentfp
// defining package via go/types. Mirrors the chainlambda testdata stub.
package slice

type Mapper[T any] []T

func From[T any](ts []T) Mapper[T] { return Mapper[T](ts) }

// Map is the fluentfp method under test: a same-named Map on an unrelated type
// (see testdata a.go's Other) must NOT be confused with this one.
func (m Mapper[T]) Map(fn func(T) T) Mapper[T] { return m }
