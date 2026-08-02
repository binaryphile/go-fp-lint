// Package e is the contract-violation fixture: the chain call resolves to a
// DIFFERENT fluentfp-rooted type (slice.Entries) that happens to share the
// KeepIf/Map method names with slice.Mapper -- the exact-type-name check
// must reject this, not just a same-named method on a wholly unrelated
// package (package c's coverage).
package e

import "github.com/binaryphile/fluentfp/slice"

type User struct {
	Name   string
	Active bool
}

func SummarizeActiveUsers(users []User) string { // want "SummarizeActiveUsers must use the project's fluentfp/slice chain methods"
	byName := make(slice.Entries[string, User])
	for _, u := range users {
		byName[u.Name] = u
	}
	active := byName.KeepIf(func(k string, u User) bool { return u.Active })
	named := active.Map(func(k string, u User) (string, User) { return k, u })
	for k := range named {
		return k
	}
	return ""
}
