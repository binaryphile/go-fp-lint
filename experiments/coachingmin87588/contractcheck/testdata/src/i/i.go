// Package i is the COMPLIANT fixture: the chain call is inside a
// PARENTHESIZED immediately-invoked closure, `(func(){ ... })()` rather
// than the bare `func(){ ... })()` form package f already covers. Proves
// unwrapFuncLit's ast.ParenExpr handling recognizes this equally-valid IIFE
// spelling.
package i

import "github.com/binaryphile/fluentfp/slice"

type User struct {
	Name   string
	Active bool
}

func SummarizeActiveUsers(users []User) string {
	return (func() string {
		active := slice.From(users).KeepIf(func(u User) bool { return u.Active })
		named := active.Map(func(u User) User { return u })
		return named[0].Name
	})()
}
