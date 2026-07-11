package a

type User struct {
	Active bool
	Name   string
}

func filterActive(users []User) []User {
	var active []User
	for _, u := range users { // want "for-loop filter shape"
		if u.Active {
			active = append(active, u)
		}
	}
	return active
}

func filterActiveContinue(users []User) []User {
	var active []User
	for _, u := range users { // want "for-loop filter shape"
		if !u.Active {
			continue
		}
		active = append(active, u)
	}
	return active
}

func filterActiveWithElse(users []User) []User {
	var active, inactive []User
	for _, u := range users {
		if u.Active {
			active = append(active, u)
		} else {
			inactive = append(inactive, u)
		}
	}
	return active
}

func multiStatementBody(users []User) []User {
	var active []User
	for _, u := range users {
		if u.Active {
			active = append(active, u)
			println("added")
		}
	}
	return active
}

// continueWithSideEffect: the guard `if` body has a statement before the
// continue, so the removal-to-KeepIf rewrite would drop that side effect —
// not flagged (parallel to the multi-statement guard-if negative).
func continueWithSideEffect(users []User) []User {
	var active []User
	for _, u := range users {
		if !u.Active {
			println("skipping")
			continue
		}
		active = append(active, u)
	}
	return active
}

// labeledContinue: the continue targets an outer loop, so the inner loop
// is not a simple filter of its own range — not flagged.
func labeledContinue(groups [][]User) []User {
	var active []User
outer:
	for _, g := range groups {
		for _, u := range g {
			if !u.Active {
				continue outer
			}
			active = append(active, u)
		}
	}
	return active
}

// continueThenReduce: the statement after the continue is a reduction, not
// an append into a slice accumulator — not a KeepIf shape, not flagged.
func continueThenReduce(users []User) int {
	count := 0
	for _, u := range users {
		if !u.Active {
			continue
		}
		count += 1
	}
	return count
}

func sumNotFilter(nums []int) int {
	total := 0
	for _, n := range nums {
		total += n
	}
	return total
}

func mapNotFilter(users []User) []string {
	var names []string
	for _, u := range users {
		names = append(names, u.Name)
	}
	return names
}
