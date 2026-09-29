package authdomain

func principalDepartmentIDs(principal Principal) []int64 {
	seen := map[int64]struct{}{}
	var ids []int64
	add := func(id int64) {
		if id <= 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for _, id := range principal.DepartmentIDs {
		add(id)
	}
	if principal.DepartmentID != nil {
		add(*principal.DepartmentID)
	}
	if principal.User != nil {
		for _, id := range userDepartmentIDs(principal.User) {
			add(id)
		}
	}
	return ids
}

func principalTeamID(principal Principal) *int64 {
	if principal.TeamID != nil && *principal.TeamID > 0 {
		return principal.TeamID
	}
	if principal.User == nil || !principal.User.TeamID.Valid || principal.User.TeamID.Int64 <= 0 {
		return nil
	}
	return &principal.User.TeamID.Int64
}

func sameValidID(a, b *int64) bool {
	return a != nil && b != nil && *a > 0 && *a == *b
}

func SameValidID(a, b *int64) bool {
	return sameValidID(a, b)
}
