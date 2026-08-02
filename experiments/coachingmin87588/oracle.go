// Package coachingmin87588 is the frozen functional eligibility oracle for
// the jeeves #87588 coaching-minimum preregistration. It holds the vehicle's
// User type and the frozen table-driven SummarizeActiveUsers functional
// contract, scored via Score against an arbitrary candidate implementation
// (mirrors causalarm91119.Score's shape — see that package's oracle_test.go
// for the discrimination-test precedent this package's own oracle_test.go
// follows).
//
// DELEGATE CONTRACT (frozen): a scored delegate MUST deliver a Go package
// named coachingmin87588 at module path example.com/delegate/coachingmin87588
// exporting `func SummarizeActiveUsers(users []User) string` plus this
// package's frozen User type — wired via score.sh's generated ephemeral
// scoring module (go.mod replace), same mechanism as causalarm91119's
// wire-and-score.sh.
package coachingmin87588

import "fmt"

// User is the frozen candidate-contract type.
type User struct {
	Name   string
	Email  string
	Active bool
}

// SummarizeFunc matches the frozen candidate contract signature.
type SummarizeFunc func(users []User) string

type testCase struct {
	name  string
	users []User
	want  string
}

// Cases is the frozen table-driven functional oracle (vehicle plan
// §"Functional eligibility oracle"): empty input, all-active, all-inactive,
// mixed-order-preserved, name/email exact-format.
var Cases = []testCase{
	{
		name:  "empty input",
		users: nil,
		want:  "",
	},
	{
		name: "all active",
		users: []User{
			{Name: "Ann", Email: "ann@x.test", Active: true},
			{Name: "Bob", Email: "bob@x.test", Active: true},
		},
		want: "Ann <ann@x.test>, Bob <bob@x.test>",
	},
	{
		name: "all inactive",
		users: []User{
			{Name: "Ann", Email: "ann@x.test", Active: false},
			{Name: "Bob", Email: "bob@x.test", Active: false},
		},
		want: "",
	},
	{
		name: "mixed order preserved",
		users: []User{
			{Name: "Cid", Email: "cid@x.test", Active: false},
			{Name: "Dee", Email: "dee@x.test", Active: true},
			{Name: "Eve", Email: "eve@x.test", Active: false},
			{Name: "Fay", Email: "fay@x.test", Active: true},
		},
		want: "Dee <dee@x.test>, Fay <fay@x.test>",
	},
	{
		name: "name/email formatting exact-match",
		users: []User{
			{Name: "Gil Vance", Email: "gil.vance@x.test", Active: true},
		},
		want: "Gil Vance <gil.vance@x.test>",
	},
}

// Score runs fn against the frozen Cases and reports pass/fail plus
// mismatch messages. This is the mechanical scoring adapter score.sh's
// generated module-wiring test calls.
func Score(fn SummarizeFunc) (pass bool, mismatches []string) {
	for _, c := range Cases {
		got := fn(c.users)
		if got != c.want {
			mismatches = append(mismatches, fmt.Sprintf("%s: got %q want %q", c.name, got, c.want))
		}
	}
	return len(mismatches) == 0, mismatches
}
