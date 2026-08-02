// Package slice is a minimal stub of fluentfp's slice package for analysistest
// fixtures — enough surface (Mapper[T] + a few higher-order chain methods) for
// chainlambda to resolve the receiver's defining package as fluentfp.
package slice

type Mapper[T any] []T

func From[T any](ts []T) Mapper[T] { return Mapper[T](ts) }

func (m Mapper[T]) KeepIf(fn func(T) bool) Mapper[T] { return m }

func (m Mapper[T]) Map(fn func(T) T) Mapper[T] { return m }

func (m Mapper[T]) RemoveIf(fn func(T) bool) Mapper[T] { return m }

func (m Mapper[T]) ToString(fn func(T) string) []string { return nil }

// Entries is a SECOND fluentfp-rooted type carrying its own KeepIf/Map
// methods, same as real fluentfp's Entries type. contractcheck's isolated
// fixture package e/ uses this to prove the contract-checker rejects a
// same-named method on the WRONG fluentfp type, not just a non-fluentfp
// type (package c/'s coverage).
type Entries[K comparable, V any] map[K]V

func (e Entries[K, V]) KeepIf(fn func(K, V) bool) Entries[K, V] { return e }

func (e Entries[K, V]) Map(fn func(K, V) (K, V)) Entries[K, V] { return e }
