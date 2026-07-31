// Package option is a minimal stub of fluentfp's option package for analysistest
// fixtures — enough for mapfusion's form-B table entry option.Map to resolve.
package option

type Option[T any] struct{ v T }

// standalone map function (form B), data-first
func Map[T, R any](o Option[T], fn func(T) R) Option[R] { return Option[R]{} }
