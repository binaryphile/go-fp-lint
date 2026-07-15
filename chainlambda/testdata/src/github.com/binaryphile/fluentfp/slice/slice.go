// Package slice is a minimal stub of fluentfp's slice package for analysistest
// fixtures — enough surface (Mapper[T] + a few higher-order chain methods) for
// chainlambda to resolve the receiver's defining package as fluentfp.
package slice

type Mapper[T any] []T

func From[T any](ts []T) Mapper[T] { return Mapper[T](ts) }

func (m Mapper[T]) KeepIf(fn func(T) bool) Mapper[T] { return m }

func (m Mapper[T]) RemoveIf(fn func(T) bool) Mapper[T] { return m }

func (m Mapper[T]) ToString(fn func(T) string) []string { return nil }
