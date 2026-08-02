package coachingmin87588

import "github.com/binaryphile/fluentfp/slice"

type User struct {
	Name   string
	Email  string
	Active bool
}

func SummarizeActiveUsers(users []User) string {
	active := slice.From(users).KeepIf(func(u User) bool { return u.Active })
	_ = active
	return "wrong-always"
}
