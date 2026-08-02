// Package option is a second fluentfp-namespace stub for the #91119 oracle: a
// DIFFERENT fluentfp type (Option) that also defines a Map method. It exists to
// exercise the contract's "MUST NOT flag another fluentfp type" requirement
// (R2/R4 New): an analyzer that checks only a broad `path contains
// binaryphile/fluentfp` condition would FALSELY flag option.Option.Map — the
// receiver-type-exact reference must not.
package option

type Option[T any] struct {
	v  T
	ok bool
}

func Some[T any](v T) Option[T] { return Option[T]{v: v, ok: true} }

// Map is a same-named method on a DIFFERENT fluentfp type — must NOT be flagged
// by the fluentmap vehicle (which targets slice.Mapper specifically).
func (o Option[T]) Map(fn func(T) T) Option[T] {
	if o.ok {
		o.v = fn(o.v)
	}
	return o
}
