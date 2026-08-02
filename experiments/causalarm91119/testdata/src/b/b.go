// Package b is Case 7 — a TRUE no-op package: ordinary code with NO Map call of
// any kind (fluent or otherwise). Both reference analyzers must emit NO
// diagnostics here — including the name-only reference, which has nothing named
// Map to over-flag. (v3 fix, R2 New-1: the prior version defined a Box.Map that
// the name-only reference DID flag, making App C row 7's "name-only agrees"
// false. Removed so the table matches the code.) The unrelated-Map negative
// controls live in package a (Case2/4b/5b).
package b

func Double(xs []int) []int {
	out := make([]int, 0, len(xs))
	for _, x := range xs {
		out = append(out, x*2)
	}
	return out
}
