// Package slice is the FROZEN vehicle stub for jeeves #87588's coaching-
// minimum experiment (mirrors contractcheck/testdata's analysistest stub,
// same KeepIf/Map/ToString surface). score.sh's generated scoring module
// `replace`s the real github.com/binaryphile/fluentfp module to this stub
// for every candidate build (structural check, transform-primary, and
// functional scoring) -- freezing the vehicle's chain-method contract
// independent of upstream fluentfp API drift (real fluentfp v0.114 has no
// plain .Map() method; Go generics don't allow one on a fixed-arity method
// set -- this stub exists so the vehicle's frozen contract has one anyway).
package slice

type Mapper[T any] []T

func From[T any](ts []T) Mapper[T] { return Mapper[T](ts) }

func (m Mapper[T]) KeepIf(fn func(T) bool) Mapper[T] {
	var out Mapper[T]
	for _, t := range m {
		if fn(t) {
			out = append(out, t)
		}
	}
	return out
}

func (m Mapper[T]) Map(fn func(T) T) Mapper[T] {
	out := make(Mapper[T], len(m))
	for i, t := range m {
		out[i] = fn(t)
	}
	return out
}

func (m Mapper[T]) RemoveIf(fn func(T) bool) Mapper[T] {
	return m.KeepIf(func(t T) bool { return !fn(t) })
}

func (m Mapper[T]) ToString(fn func(T) string) []string {
	out := make([]string, len(m))
	for i, t := range m {
		out[i] = fn(t)
	}
	return out
}
