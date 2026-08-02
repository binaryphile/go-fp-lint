// Package a is the #91119 causal-arm oracle fixture set. Each case corresponds
// to a row of Appendix C of
// investigations/2026-08-01-model-routing-causal-arm-preregistration.md (jeeves).
// The want-markers below encode the CORRECT (type-aware) expected diagnostics,
// derived from the rule BEFORE any run (the positive control): exact message
// including the resolved receiver type. A name-only analyzer diverges on the
// negative-control cases (2, 4b, 5b) by false-positiving — that divergence is
// what the discrimination test exploits.
package a

import "github.com/binaryphile/fluentfp/slice"

func inc(x int) int { return x + 1 }

// Case 1 — true positive: Map on the fluentfp slice type MUST flag.
func Case1(xs []int) slice.Mapper[int] {
	return slice.From(xs).Map(inc) // want `fluent Map call: github\.com/binaryphile/fluentfp/slice\.Mapper`
}

// Case 2 — negative control: a same-named Map on an UNRELATED type must NOT
// flag. The core trap a name-only linter fails (false positive here).
type Other struct{}

func (o Other) Map(fn func(int) int) Other { return o }

func Case2() { Other{}.Map(inc) }

// Case 3 — type alias of the fluentfp type MUST flag (resolves to the same
// fluentfp receiver type).
type Aliased = slice.Mapper[int]

func Case3(a Aliased) { a.Map(inc) } // want `fluent Map call: github\.com/binaryphile/fluentfp/slice\.Mapper`

// Case 4 — embedded/promoted Map on the fluentfp type MUST flag (the promoted
// method's receiver type is still slice.Mapper).
type Embedder struct{ slice.Mapper[int] }

func Case4(e Embedder) { e.Map(inc) } // want `fluent Map call: github\.com/binaryphile/fluentfp/slice\.Mapper`

// Case 4b — embedded/promoted Map on an UNRELATED type must NOT flag (the
// promoted method's receiver type is Other).
type EmbedOther struct{ Other }

func Case4b(e EmbedOther) { e.Map(inc) }

// Case 5 — pointer and value receiver on the fluentfp type: both MUST flag.
func Case5v(m slice.Mapper[int]) { m.Map(inc) }    // want `fluent Map call: github\.com/binaryphile/fluentfp/slice\.Mapper`
func Case5p(m slice.Mapper[int]) { (&m).Map(inc) } // want `fluent Map call: github\.com/binaryphile/fluentfp/slice\.Mapper`

// Case 5b — pointer/value on the unrelated type: neither flags.
func Case5other(o Other) {
	o.Map(inc)
	(&o).Map(inc)
}
