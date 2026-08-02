// Package a is the compliant fixture: SummarizeActiveUsers reaches a
// KeepIf/Map call on the fluentfp slice.Mapper type in reachable code.
package a

import (
	"fmt"

	"github.com/binaryphile/fluentfp/slice"
)

type User struct {
	Name   string
	Active bool
}

func SummarizeActiveUsers(users []User) string {
	active := slice.From(users).KeepIf(func(u User) bool { return u.Active })
	named := active.Map(func(u User) User { return u })
	return fmt.Sprint(named)
}
