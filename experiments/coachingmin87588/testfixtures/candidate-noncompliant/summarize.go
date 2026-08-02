package coachingmin87588

type User struct {
	Name   string
	Email  string
	Active bool
}

func SummarizeActiveUsers(users []User) string {
	result := ""
	for _, u := range users {
		if u.Active {
			if result != "" {
				result += ", "
			}
			result += u.Name + " <" + u.Email + ">"
		}
	}
	return result
}
