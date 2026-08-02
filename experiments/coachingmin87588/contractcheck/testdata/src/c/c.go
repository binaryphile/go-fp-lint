// Package c is the contract-violation fixture: the chain call resolves to a
// type that merely LOOKS like fluentfp's Mapper (same method names) but
// isn't -- the receiver's package must be fluentfp, not just the shape.
package c

import "fmt"

type User struct {
	Name   string
	Active bool
}

type notFluentMapper[T any] []T

func (m notFluentMapper[T]) KeepIf(fn func(T) bool) notFluentMapper[T] { return m }
func (m notFluentMapper[T]) Map(fn func(T) T) notFluentMapper[T]       { return m }

func SummarizeActiveUsers(users []User) string { // want "SummarizeActiveUsers must use the project's fluentfp/slice chain methods"
	active := notFluentMapper[User](users).KeepIf(func(u User) bool { return u.Active })
	named := active.Map(func(u User) User { return u })
	return fmt.Sprint(named)
}
