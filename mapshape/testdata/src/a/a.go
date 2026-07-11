package a

import "errors"

type User struct {
	Name    string
	ID      int
	Int32ID int32
	Active  bool
}

func (u User) Clone() User { return u }

func validate(u User) error {
	if u.ID == 0 {
		return errBad
	}
	return nil
}

var errBad = errors.New("bad")

type Summary struct {
	Name string
}

func toSummary(u User) Summary { return Summary{Name: u.Name} }

// Marker is a locally-declared empty interface — distinct from the builtin
// `any`, used to confirm the ToAny classification doesn't over-match on
// every empty interface.
type Marker interface{}

func toMarker(u User) Marker { return u }

var users []User

// sameTypeTransform: EXPR (u.Clone()) has the same type as the range value
// (User) — same-type mapping, suggest Transform.
func sameTypeTransform() []User {
	var clones []User
	for _, u := range users { // want "map-loop shape"
		clones = append(clones, u.Clone())
	}
	return clones
}

// stringTarget: EXPR (u.Name) is a string — known-alias, suggest ToString.
func stringTarget() []string {
	var names []string
	for _, u := range users { // want "map-loop shape"
		names = append(names, u.Name)
	}
	return names
}

// intTarget: EXPR (u.ID) is a plain int — known-alias, suggest ToInt.
func intTarget() []int {
	var ids []int
	for _, u := range users { // want "map-loop shape"
		ids = append(ids, u.ID)
	}
	return ids
}

// int32Target: EXPR (u.Int32ID) is int32, distinct from the plain-int case
// above — discrimination fixture confirming the classifier doesn't
// conflate int and int32 (adjacent branches of the same Basic-kind switch).
func int32Target() []int32 {
	var codes []int32
	for _, u := range users { // want "map-loop shape"
		codes = append(codes, u.Int32ID)
	}
	return codes
}

// errorTarget: EXPR (validate(u)) is the error interface — known-alias,
// suggest ToError.
func errorTarget() []error {
	var errs []error
	for _, u := range users { // want "map-loop shape"
		errs = append(errs, validate(u))
	}
	return errs
}

// anyTarget: EXPR (any(u)) is the builtin empty interface `any` — known-alias,
// suggest ToAny.
func anyTarget() []any {
	var things []any
	for _, u := range users { // want "map-loop shape"
		things = append(things, any(u))
	}
	return things
}

// markerTarget: EXPR (toMarker(u)) is a LOCALLY-DECLARED empty interface,
// not the builtin `any` — discrimination fixture confirming the ToAny
// classification doesn't over-match on every empty interface; must fall
// through to the arbitrary-R slice.Map branch instead.
func markerTarget() []Marker {
	var markers []Marker
	for _, u := range users { // want "map-loop shape"
		markers = append(markers, toMarker(u))
	}
	return markers
}

// structTarget: EXPR (toSummary(u)) is an arbitrary struct type — no
// known-alias match, suggest the standalone slice.Map.
func structTarget() []Summary {
	var summaries []Summary
	for _, u := range users { // want "map-loop shape"
		summaries = append(summaries, toSummary(u))
	}
	return summaries
}

// identityCopyLoop: EXPR is the bare range-value identifier itself — a
// plain copy, nothing to transform, deliberately NOT flagged.
func identityCopyLoop() []User {
	var copies []User
	for _, u := range users {
		copies = append(copies, u)
	}
	return copies
}

// guardIfShape: filterloop's guard-if shape — body's one statement is an
// `if`, not an assign, so mapshape must NOT double-fire.
func guardIfShape() []User {
	var active []User
	for _, u := range users {
		if u.Active {
			active = append(active, u)
		}
	}
	return active
}

// continueGuardShape: filterloop's continue-guard shape — body is two
// statements, so mapshape must NOT double-fire.
func continueGuardShape() []User {
	var active []User
	for _, u := range users {
		if !u.Active {
			continue
		}
		active = append(active, u)
	}
	return active
}

// multiStatementBody: body is two statements (neither an if-guard), not
// the single-statement map-loop shape — not flagged.
func multiStatementBody() []string {
	var names []string
	for _, u := range users {
		name := u.Name
		names = append(names, name)
	}
	return names
}
