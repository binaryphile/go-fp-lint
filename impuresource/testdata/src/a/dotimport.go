package a

import (
	. "os"
)

// dotImportedCall: a dot-imported call resolves via a bare *ast.Ident, not
// a *ast.SelectorExpr — proves object resolution is stable across
// dot-imports too, same canonical "os.Getenv" message (fixture #4).
func dotImportedCall() {
	Getenv("X") // want "direct call to os.Getenv"
}
