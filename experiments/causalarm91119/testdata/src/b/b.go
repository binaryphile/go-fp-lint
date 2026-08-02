// Package b is Case 7 — an ISOLATED package with no fluentfp Map calls at all
// (a same-named Map on an unrelated local type, plus ordinary code). The
// analyzer must emit NO diagnostics here. Kept in its own package (not folded
// into package a) so "no spurious firing in a clean package" is tested in
// isolation, not masked by package a's positives.
package b

type Box struct{ v int }

func (b Box) Map(fn func(int) int) Box { return Box{fn(b.v)} }

func Use() int {
	b := Box{1}.Map(func(x int) int { return x + 1 })
	return b.v
}
