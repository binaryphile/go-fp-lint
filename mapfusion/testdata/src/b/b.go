// Package b exercises mapfusion's dot-import path: a standalone fluentfp map
// used as a bare identifier (callee is *ast.Ident, not a SelectorExpr).
package b

import (
	. "github.com/binaryphile/fluentfp/slice"
)

func toI(x int) int    { return x }
func toS(x int) string { return "" }

// Dot-imported standalone Map nested in itself — bare-ident callee.
func Pos(xs []int) Mapper[string] {
	return Map(Map(xs, toI), toS) // want `double map`
}

// Single dot-imported map — nothing to fuse.
func Neg(xs []int) Mapper[int] { return Map(xs, toI) }
