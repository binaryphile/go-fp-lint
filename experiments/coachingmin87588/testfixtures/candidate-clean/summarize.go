package coachingmin87588

import (
	"strings"

	"github.com/binaryphile/fluentfp/slice"
)

type User struct {
	Name   string
	Email  string
	Active bool
}

func SummarizeActiveUsers(users []User) string {
	active := slice.From(users).KeepIf(func(u User) bool { return u.Active })
	named := active.Map(func(u User) User { return u })
	parts := make([]string, 0, len(named))
	for _, u := range named {
		parts = append(parts, u.Name+" <"+u.Email+">")
	}
	return strings.Join(parts, ", ")
}
