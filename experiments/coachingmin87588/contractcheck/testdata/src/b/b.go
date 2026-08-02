// Package b is the contract-violation fixture: the only KeepIf/Map mentions
// are in a comment and an unreachable branch -- no CFG-reachable chain call.
package b

type User struct {
	Name   string
	Active bool
}

// SummarizeActiveUsers should use slice.From(users).KeepIf(...).Map(...)
// but the mention above is a comment, and the dead branch below is
// unreachable, so neither counts as a real contract-compliant call.
func SummarizeActiveUsers(users []User) string { // want "SummarizeActiveUsers must use the project's fluentfp/slice chain methods"
	if false {
		_ = deadKeepIfMap(users)
	}
	result := ""
	for _, u := range users {
		if u.Active {
			result += u.Name
		}
	}
	return result
}

func deadKeepIfMap(users []User) []User { return users }
