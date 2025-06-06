package util

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
)

// SliceContains returns true if it finds an item in the slice that is equal to the target
func SliceContains[T comparable](slice []T, target T) bool {
	for _, item := range slice {
		if item == target {
			return true
		}
	}
	return false
}

// SliceContainsSlice returns true all items in the target are equal to items in the slice
func SliceContainsSlice[T comparable](slice []T, target []T) bool {
	var containsAllElementsOfTarget = true
	for _, targetElement := range target {
		if SliceContains(slice, targetElement) {
			containsAllElementsOfTarget = true
		} else {
			return false
		}
	}

	return containsAllElementsOfTarget
}

// SliceEquals returns true if the slices have the same string contents, regardless of order
func SliceEquals(slice []string, target []string) bool {
	sort.Strings(slice)
	sort.Strings(target)

	if slices.Equal(slice, target) {
		return true
	}
	return false
}

// SliceTransform takes an input collection, applies the transform function to each row, and returns the output.
// Known as 'map' in most other languages or 'Select' in C# Linq.
func SliceTransform[T any, TResult any](slice []T, transform func(item T) TResult) []TResult {
	var results []TResult = nil
	for _, item := range slice {
		results = append(results, transform(item))
	}
	return results
}

// SliceFilter takes an input collection and returns elements where `predicate` returns true
// Known as 'filter' in most other languages or 'Select' in C# Linq.
func SliceFilter[T any](slice []T, predicate func(item T) bool) []T {
	var results []T = nil
	for _, item := range slice {
		if predicate(item) {
			results = append(results, item)
		}
	}
	return results
}

func SliceExcept[T any](slice []T, predicate func(item T) bool) []T {
	return SliceFilter(slice, func(item T) bool { return !predicate(item) })
}

// SliceContainsAny returns true if it finds an item in the slice where `predicate` returns true
func SliceContainsAny[T comparable](slice []T, predicate func(item T) bool) bool {
	for _, item := range slice {
		if predicate(item) {
			return true
		}
	}
	return false
}

// SliceContainsAll returns true if it finds that all items in the slice for which `predicate` returns true
func SliceContainsAll[T comparable](slice []T, predicate func(item T) bool) bool {
	for _, item := range slice {
		if !predicate(item) {
			return false
		}
	}
	return true
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

type MapCollectionCacheContainer struct {
	Caches []map[string]string
}

// MapCollectionWithLookups does a 'map' operation over the collection, transforming each input row
// into an output row according to `mapper`, however it also provides the ability to collect ID's of related
// items as it iterates the collection, and call out to lambdas to look those values up.
// See the unit tests for examples which should clarify the use-cases for this.
func MapCollectionWithLookups[T any, TResult any](
	cacheContainer *MapCollectionCacheContainer, // cache for keys (typically this will store a mapping of ID->[Name, Name]).
	collection []T, // input (e.g. list of Releases)
	keySelector func(T) []string, // fetches the keys (e.g given a Release, returns the [ChannelID, ProjectID]
	mapper func(T, []string) TResult, // fetches the value to lookup (e.g given a Release and the [ChannelName,ProjectName], does the mapping to return the output struct)
	runLookups ...func([]string) ([]string, error), // callbacks to go fetch values for the keys (given a list of Channel IDs, it should return the list of associated Channel Names)
) ([]TResult, error) {
	// if the caller didn't specify an external cache, create an internal one.
	// it'll work, but we lose the ability to cache across multiple lookups
	// (e.g. when fetching more than one page of results from the server)
	if cacheContainer == nil {
		cacheContainer = &MapCollectionCacheContainer{}
	}

	if len(cacheContainer.Caches) < len(runLookups) {
		// caches aren't allocated, we need to do that
		cacheContainer.Caches = nil
		for i := 0; i < len(runLookups); i++ {
			cacheContainer.Caches = append(cacheContainer.Caches, map[string]string{})
		}
	}

	caches := cacheContainer.Caches

	// first pass: walk the collection and see if there's anything we need to look up.
	// if we detect a situation where all the lookups are already satisfied by the cache,
	// then we may not need to do any lookups at all.
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

	// if we have things we need to look up, go and look them up and insert them into the cache.
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

	// Now we do a second pass over the array in order to produce the output (incorporating the looked-up values from cache)
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

func Empty[T any](items []T) bool {
	return items == nil || len(items) == 0
}

func Any[T any](items []T) bool {
	return !Empty(items)
}

func SliceDistinct[T comparable](slice []T) []T {
	inResult := make(map[T]bool)
	var result []T
	for _, str := range slice {
		if _, ok := inResult[str]; !ok {
			inResult[str] = true
			result = append(result, str)
		}
	}
	return result
}

func RemoveIndex[T any](s []T, index int) []T {
	if index < 0 || index >= len(s) {
		return s
	}

	return append(s[:index], s[index+1:]...)
}

// HumanReadableBytes converts a byte count into a human-readable string, e.g. 2.5 MiB
// there are about a zillion golang packages for formatting bytes as human-readable values, the top search result of which is
// https://pkg.go.dev/github.com/dustin/go-humanize. However, this package is large and does too much, there's no need to use it
// when we can use this trivial tutorial one instead: https://programming.guide/go/formatting-byte-size-to-human-readable-format.html
func HumanReadableBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// SplitString splits the input string into components based on delimiter characters.
// we want to pick up empty entries here; so "::5" and ":pterm:5" should both return THREE components, rather than one or two
// and we want to allow for multiple different delimeters.
// neither the builtin golang strings.Split or strings.FieldsFunc support this. Logic borrowed from strings.FieldsFunc with heavy modifications
func SplitString(s string, delimiters []rune) []string {
	// pass 1: collect spans; golang strings.FieldsFunc says it's much more efficient this way
	type span struct {
		start int
		end   int
	}
	spans := make([]span, 0, 3)

	// Find the field start and end indices.
	start := 0 // we always start the first span at the beginning of the string
	for idx, ch := range s {
		if slices.Contains(delimiters, ch) {
			if start >= 0 { // we found a delimiter and we are already in a span; end the span and start a new one
				spans = append(spans, span{start, idx})
				start = idx + 1
			} else { // we found a delimiter and we are not in a span; start a new span
				if start < 0 {
					start = idx
				}
			}
		}
	}

	// Last field might end at EOF.
	if start >= 0 {
		spans = append(spans, span{start, len(s)})
	}

	// pass 2: create strings from recorded field indices.
	a := make([]string, len(spans))
	for i, span := range spans {
		a[i] = s[span.start:span.end]
	}
	return a
}

// helpful for debugging
func PrintJSON(obj interface{}) {
	bytes, _ := json.MarshalIndent(obj, "\t", "\t")
	fmt.Println(string(bytes))
}
