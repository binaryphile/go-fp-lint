// Package f is the COMPLIANT fixture: the chain call is inside an
// IMMEDIATELY-INVOKED function literal (an IIFE) -- proves the R2 finding-2
// fix (skip uninvoked closures) does not overcorrect into rejecting a
// closure that DOES execute as part of the reachable statement (R2 finding
// 13's regression).
package f

import "github.com/binaryphile/fluentfp/slice"

type User struct {
	Name   string
	Active bool
}

func SummarizeActiveUsers(users []User) string {
	return func() string {
		active := slice.From(users).KeepIf(func(u User) bool { return u.Active })
		named := active.Map(func(u User) User { return u })
		return named[0].Name
	}()
}
