package a

// Case 3b — RENAMED IMPORT (distinct from the type-alias case): the fluentfp
// slice package imported under a different local name still resolves to the same
// receiver type and MUST flag. A name-only check would agree here; the point is
// that the type-aware reference must NOT be fooled into MISSING it (a false
// negative) by the non-canonical import name.
import fp "github.com/binaryphile/fluentfp/slice"

func Case3b(xs []int) fp.Mapper[int] {
	return fp.From(xs).Map(inc) // want `fluent Map call: github\.com/binaryphile/fluentfp/slice\.Mapper`
}
