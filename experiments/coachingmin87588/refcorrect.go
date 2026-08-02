package coachingmin87588

import "strings"

// RefCorrect is the oracle's known-correct reference implementation of the
// frozen SummarizeActiveUsers contract (vehicle plan: filter to Active, map
// to "<Name> <<Email>>", join with ", " in input order). Used only by
// oracle_test.go's discrimination test — the functional oracle scores
// BEHAVIOR, independent of implementation strategy; the separate structural
// contract check (contractcheck/) is what enforces the fluentfp chain-method
// requirement.
func RefCorrect(users []User) string {
	var parts []string
	for _, u := range users {
		if !u.Active {
			continue
		}
		parts = append(parts, u.Name+" <"+u.Email+">")
	}
	return strings.Join(parts, ", ")
}

// RefBroken is the oracle's known-broken reference: it reverses candidate
// order, which the "mixed order preserved" case must catch.
func RefBroken(users []User) string {
	var parts []string
	for i := len(users) - 1; i >= 0; i-- {
		u := users[i]
		if !u.Active {
			continue
		}
		parts = append(parts, u.Name+" <"+u.Email+">")
	}
	return strings.Join(parts, ", ")
}
