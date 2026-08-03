// Package g is the contract-violation fixture: the chain call resolves to
// fluentfp/slice/evil.Mapper -- a type sharing slice.Mapper's exact name
// and method names, but defined in a fluentfp SUBPACKAGE outside the
// vehicle's permitted set. Closes IMPL-grade R2 finding 1 (the R1 fix
// checked type name but not the exact package).
package g

import "github.com/binaryphile/fluentfp/slice/evil"

type User struct {
	Name   string
	Active bool
}

func SummarizeActiveUsers(users []User) string { // want "SummarizeActiveUsers must use the project's fluentfp/slice chain methods"
	active := evil.From(users).KeepIf(func(u User) bool { return u.Active })
	named := active.Map(func(u User) User { return u })
	return named[0].Name
}
