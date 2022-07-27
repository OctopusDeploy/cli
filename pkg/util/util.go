package util

func SliceContains[T comparable](slice []T, target T) bool {
	for _, item := range slice {
		if item == target {
			return true
		}
	}
	return false
}

func MapSlice[T any, TResult any](slice []T, mapper func(item T) TResult) []TResult {
	var results []TResult = nil
	for _, item := range slice {
		results = append(results, mapper(item))
	}
	return results
}
