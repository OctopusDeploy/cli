package util

func SliceOverlap(a, b []string) bool {
	set := make(map[string]struct{}, len(a))
	for _, item := range a {
		set[item] = struct{}{}
	}

	for _, item := range b {
		if _, exists := set[item]; exists {
			return true
		}
	}
	return false
}
