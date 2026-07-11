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
