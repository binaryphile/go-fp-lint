// Package evil is a HOSTILE fixture package: a type also named "Mapper",
// with the same KeepIf/Map method names, but defined in a fluentfp
// SUBPACKAGE that is not the vehicle's permitted set
// (fluentfp/slice, fluentfp/internal/base). Proves the exact-package check
// (contractcheck.permittedMapperPkgs) rejects a same-named type merely
// because it lives somewhere under the fluentfp module root.
package evil

type Mapper[T any] []T

func From[T any](ts []T) Mapper[T] { return Mapper[T](ts) }

func (m Mapper[T]) KeepIf(fn func(T) bool) Mapper[T] { return m }

func (m Mapper[T]) Map(fn func(T) T) Mapper[T] { return m }
