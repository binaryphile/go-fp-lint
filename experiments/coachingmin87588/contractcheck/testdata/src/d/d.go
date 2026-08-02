// Package d is the contract-violation fixture: the only KeepIf/Map call is
// inside a function literal that is DEFINED but never INVOKED -- a live
// statement (the assignment) does not make the closure's own body
// CFG-reachable.
package d

import "github.com/binaryphile/fluentfp/slice"

type User struct {
	Name   string
	Active bool
}

func SummarizeActiveUsers(users []User) string { // want "SummarizeActiveUsers must use the project's fluentfp/slice chain methods"
	uncalled := func() string {
		active := slice.From(users).KeepIf(func(u User) bool { return u.Active })
		named := active.Map(func(u User) User { return u })
		return named[0].Name
	}
	_ = uncalled
	return ""
}
