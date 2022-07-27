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

// ExtractValuesMatchingKeys returns a collection of values which matched a specified set of keys, in the exact order of keys.
// Given a collection of items, and a collection of keys to match within that collection
// This makes no sense, hopefully an example helps:
// - Given an array of Channels [<id:C1, name:'C1Name'>, <id:C7, name:'C7Name'> , <id:C2, name:'C2Name'>]
// - And a set of keys [C2, C7]
// - Returns the extracted values ['C2Name', 'C7Name']
//
// Note: if something can't be found, then the extracted values collection will contain a zero value in-place
func ExtractValuesMatchingKeys[T any](collection []T, keys []string, idSelector func(T) string, valueSelector func(T) string) []string {
	// the server doesn't neccessarily return items in the order matching 'keys'
	// so we have to build the association manually
	results := make([]string, len(keys))
	for idx, key := range keys {
		// find the key in the lookupResult.Items and slot it into the correct array index
		foundItem := false
		for _, item := range collection {
			if idSelector(item) == key {
				results[idx] = valueSelector(item)
				foundItem = true
				break
				// TODO we could optimise this by removing something from 'collection' once we've found it,
				// or by front-loading them into a map[string]string. Dig into that later for performance if need be
			}
		}
		if !foundItem {
			results[idx] = ""
		}
	}
	return results
}

// like MapCollectionWithLookup except it can lookup more than one attribute.
// e.g. a Release has both a Project and a Channel that we'd like to lookup the name of
func MapCollectionWithLookups[T any, TResult any](
	caches []map[string]string, // cache for keys (typically this will store a mapping of ID->[Name, Name]). YOU MUST PREALLOCATE THIS
	collection []T, // input (e.g. list of Releases)
	keySelector func(T) []string, // fetches the keys (e.g given a Release, returns the [ChannelID, ProjectID]
	mapper func(T, []string) TResult, // fetches the value to lookup (e.g given a Release and the [ChannelName,ProjectName], does the mapping to return the output struct)
	runLookups ...func([]string) ([]string, error), // callbacks to go fetch values for the keys (given a list of Channel IDs, it should return the list of associated Channel Names)
) ([]TResult, error) {
	// this works in two passes. First it walks the collection and sees if there is anything it needs to look up.
	// (it doesn't produce any results or do any mapping, just key selection).
	// then, if necessary, it looks up the keys and populates the cache
	// for the second pass we walk the collection and assign keys to cached values

	var allKeysToLookup = make([][]string, len(caches)) // preallocate the right number of nils
	for _, item := range collection {
		keys := keySelector(item)
		for i, key := range keys {
			_, ok := caches[i][key]
			if !ok { // we haven't seen this value in the cache, we need to go and look something up
				if allKeysToLookup[i] == nil {
					allKeysToLookup[i] = []string{key}
				} else {
					if !SliceContains(allKeysToLookup[i], key) {
						allKeysToLookup[i] = append(allKeysToLookup[i], key)
					} // else we've already seen this key
				}
			}
		}
	}

	// if we don't know the names of some things, go and look them up, then restart the mapping process.
	// we do a second pass over the whole array, which isn't perfectly efficient, but it's simpler, and
	// we are dealing with small numbers here (page size of 100 or less) so perf will be fine
	for lookupIdx, keysToLookup := range allKeysToLookup {
		if keysToLookup != nil {
			lookedUpValues, err := runLookups[lookupIdx](keysToLookup)
			if err != nil {
				return nil, err
			}
			for valueIdx, value := range lookedUpValues {
				caches[lookupIdx][keysToLookup[valueIdx]] = value
			}
		}
	}

	var results []TResult
	for _, item := range collection {
		keys := keySelector(item)
		values := make([]string, len(keys))
		for i, key := range keys {
			value, ok := caches[i][key]
			if ok {
				values[i] = value
			} else {
				values[i] = ""
			}
		}

		results = append(results, mapper(item, values))
	}
	return results, nil
}
