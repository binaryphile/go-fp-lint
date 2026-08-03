// Package h is the contract-violation fixture: the chain call is inside an
// IMMEDIATELY-INVOKED closure, but AFTER an unconditional return inside
// that closure -- dead code one level deeper than the top-level case
// (package b). Proves the invoked-closure's own CFG-reachability analysis
// (IMPL-grade R3 finding 13 remainder) excludes dead code inside the
// closure, not just at the top level.
package h

import "github.com/binaryphile/fluentfp/slice"

type User struct {
	Name   string
	Active bool
}

func SummarizeActiveUsers(users []User) string { // want "SummarizeActiveUsers must use the project's fluentfp/slice chain methods"
	return func() string {
		return ""
		active := slice.From(users).KeepIf(func(u User) bool { return u.Active })
		named := active.Map(func(u User) User { return u })
		return named[0].Name
	}()
}
