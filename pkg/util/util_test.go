package util_test

import (
	"errors"
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExtractValuesMatchingKeys(t *testing.T) {
	type Person struct {
		ID        string
		FirstName string
		LastName  string
	}

	people := []Person{
		{ID: "1", FirstName: "John", LastName: "Smith"},
		{ID: "2", FirstName: "Jane", LastName: "Doe"},
		{ID: "3", FirstName: "Alan", LastName: "Walker"},
	}

	t.Run("happy path extracting all keys in order", func(t *testing.T) {
		firstNames := util.ExtractValuesMatchingKeys(
			people,
			[]string{"1", "2", "3"},
			func(p Person) string { return p.ID },
			func(p Person) string { return p.FirstName })

		assert.Equal(t, []string{"John", "Jane", "Alan"}, firstNames)
	})

	t.Run("extracting all keys in a different order", func(t *testing.T) {
		firstNames := util.ExtractValuesMatchingKeys(
			people,
			[]string{"3", "1", "2"},
			func(p Person) string { return p.ID },
			func(p Person) string { return p.FirstName })

		assert.Equal(t, []string{"Alan", "John", "Jane"}, firstNames)
	})

	t.Run("extracting subset of keys", func(t *testing.T) {
		lastNames := util.ExtractValuesMatchingKeys(
			people,
			[]string{"3", "2"},
			func(p Person) string { return p.ID },
			func(p Person) string { return p.LastName })

		assert.Equal(t, []string{"Walker", "Doe"}, lastNames)
	})

	t.Run("extracting missing keys returns a blank value", func(t *testing.T) {
		lastNames := util.ExtractValuesMatchingKeys(
			people,
			[]string{"3", "19", "2", "12"},
			func(p Person) string { return p.ID },
			func(p Person) string { return p.LastName })

		assert.Equal(t, []string{"Walker", "", "Doe", ""}, lastNames)
	})
}

func TestMapCollectionWithLookups(t *testing.T) {
	// model: A person works in the Sales Department in the Country of New Zealand
	type Person struct {
		ID           string
		FirstName    string
		LastName     string
		DepartmentID string
		CountryID    string
	}

	type Department struct {
		ID   string
		Name string
	}

	departments := map[string]Department{
		"S": {ID: "S", Name: "Sales"},
		"M": {ID: "M", Name: "Marketing"},
	}

	type Country struct {
		ID   string
		Name string
	}

	countries := map[string]Country{
		"NZ": {ID: "NZ", Name: "New Zealand"},
		"AU": {ID: "AU", Name: "Australia"},
	}

	people := []Person{
		{ID: "1", FirstName: "John", LastName: "Smith", DepartmentID: "S", CountryID: "NZ"},
		{ID: "2", FirstName: "Jane", LastName: "Doe", DepartmentID: "M", CountryID: "NZ"},
		{ID: "3", FirstName: "Alan", LastName: "Walker", DepartmentID: "S", CountryID: "AU"},
	}

	type PersonWithCountryDepartment struct {
		PersonID       string
		Name           string
		DepartmentName string
		CountryName    string
	}

	t.Run("typical with two lookups", func(t *testing.T) {
		caches := util.MapCollectionCacheContainer{}

		results, err := util.MapCollectionWithLookups(
			&caches,
			people,
			func(p Person) []string { return []string{p.DepartmentID, p.CountryID} },
			func(p Person, lookup []string) PersonWithCountryDepartment {
				return PersonWithCountryDepartment{
					PersonID:       p.ID,
					Name:           fmt.Sprintf("%s %s", p.FirstName, p.LastName),
					DepartmentName: lookup[0],
					CountryName:    lookup[1],
				}
			},
			// lookup for departments
			func(departmentIds []string) ([]string, error) {
				return util.SliceTransform(departmentIds, func(deptId string) string {
					return departments[deptId].Name
				}), nil
			},
			// lookup for countries
			func(countryIds []string) ([]string, error) {
				return util.SliceTransform(countryIds, func(cid string) string {
					return countries[cid].Name
				}), nil
			},
		)

		assert.Nil(t, err)
		assert.Equal(t, []PersonWithCountryDepartment{
			{PersonID: "1", Name: "John Smith", CountryName: "New Zealand", DepartmentName: "Sales"},
			{PersonID: "2", Name: "Jane Doe", CountryName: "New Zealand", DepartmentName: "Marketing"},
			{PersonID: "3", Name: "Alan Walker", CountryName: "Australia", DepartmentName: "Sales"},
		}, results)

		// should have populated the caches for the next page of lookups
		assert.Equal(t, []map[string]string{
			{"S": "Sales", "M": "Marketing"},
			{"NZ": "New Zealand", "AU": "Australia"},
		}, caches.Caches)
	})

	t.Run("just one lookup", func(t *testing.T) {
		caches := util.MapCollectionCacheContainer{}

		results, err := util.MapCollectionWithLookups(
			&caches,
			people,
			func(p Person) []string { return []string{p.DepartmentID} },
			func(p Person, lookup []string) PersonWithCountryDepartment {
				return PersonWithCountryDepartment{
					PersonID:       p.ID,
					Name:           fmt.Sprintf("%s %s", p.FirstName, p.LastName),
					DepartmentName: lookup[0],
				}
			},
			// lookup for departments
			func(departmentIds []string) ([]string, error) {
				return util.SliceTransform(departmentIds, func(deptId string) string {
					return departments[deptId].Name
				}), nil
			},
		)

		assert.Nil(t, err)
		assert.Equal(t, []PersonWithCountryDepartment{
			{PersonID: "1", Name: "John Smith", DepartmentName: "Sales"},
			{PersonID: "2", Name: "Jane Doe", DepartmentName: "Marketing"},
			{PersonID: "3", Name: "Alan Walker", DepartmentName: "Sales"},
		}, results)
	})

	t.Run("no lookups", func(t *testing.T) {
		caches := util.MapCollectionCacheContainer{}

		results, err := util.MapCollectionWithLookups(
			&caches,
			people,
			func(p Person) []string { return []string{} },
			func(p Person, lookup []string) PersonWithCountryDepartment {
				return PersonWithCountryDepartment{
					PersonID: p.ID,
					Name:     fmt.Sprintf("%s %s", p.FirstName, p.LastName),
				}
			},
		)

		assert.Nil(t, err)
		assert.Equal(t, []PersonWithCountryDepartment{
			{PersonID: "1", Name: "John Smith"},
			{PersonID: "2", Name: "Jane Doe"},
			{PersonID: "3", Name: "Alan Walker"},
		}, results)
	})

	t.Run("allocates internal cache if storage isn't provided", func(t *testing.T) {
		results, err := util.MapCollectionWithLookups(
			nil, // no cache storage provided; we lose the ability to cache across calls but it should still work
			people,
			func(p Person) []string { return []string{p.DepartmentID, p.CountryID} },
			func(p Person, lookup []string) PersonWithCountryDepartment {
				return PersonWithCountryDepartment{
					PersonID:       p.ID,
					Name:           fmt.Sprintf("%s %s", p.FirstName, p.LastName),
					DepartmentName: lookup[0],
					CountryName:    lookup[1],
				}
			},
			// lookup for departments
			func(departmentIds []string) ([]string, error) {
				return util.SliceTransform(departmentIds, func(deptId string) string {
					return departments[deptId].Name
				}), nil
			},
			// lookup for countries
			func(countryIds []string) ([]string, error) {
				return util.SliceTransform(countryIds, func(cid string) string {
					return countries[cid].Name
				}), nil
			},
		)

		assert.Nil(t, err)
		assert.Equal(t, []PersonWithCountryDepartment{
			{PersonID: "1", Name: "John Smith", CountryName: "New Zealand", DepartmentName: "Sales"},
			{PersonID: "2", Name: "Jane Doe", CountryName: "New Zealand", DepartmentName: "Marketing"},
			{PersonID: "3", Name: "Alan Walker", CountryName: "Australia", DepartmentName: "Sales"},
		}, results)
	})

	t.Run("doesn't use lookup if values are already cached", func(t *testing.T) {
		// preload the cache with not-quite-right data to check that the function fetches it from the cache rather than lookup
		caches := util.MapCollectionCacheContainer{
			Caches: []map[string]string{
				{"S": "zzSales", "M": "zzMarketing"},
			},
		}

		results, err := util.MapCollectionWithLookups(
			&caches,
			people,
			func(p Person) []string { return []string{p.DepartmentID} },
			func(p Person, lookup []string) PersonWithCountryDepartment {
				return PersonWithCountryDepartment{
					PersonID:       p.ID,
					Name:           fmt.Sprintf("%s %s", p.FirstName, p.LastName),
					DepartmentName: lookup[0],
				}
			},
			// lookup for departments
			func(departmentIds []string) ([]string, error) {
				t.Fatal("Should not get invoked")
				return nil, nil
			},
		)

		assert.Nil(t, err)
		assert.Equal(t, []PersonWithCountryDepartment{
			{PersonID: "1", Name: "John Smith", DepartmentName: "zzSales"},
			{PersonID: "2", Name: "Jane Doe", DepartmentName: "zzMarketing"},
			{PersonID: "3", Name: "Alan Walker", DepartmentName: "zzSales"},
		}, results)
	})

	t.Run("only looks up the minimum required if data is partially cached", func(t *testing.T) {
		// preload the cache with not-quite-right data to check that the function fetches it from the cache rather than lookup
		caches := util.MapCollectionCacheContainer{
			Caches: []map[string]string{
				{"S": "zzSales"}, // marketing is not cached
				{"AU": "zzAustralia", "NZ": "zzNewZealand"}, // full country data is cached
			},
		}

		results, err := util.MapCollectionWithLookups(
			&caches,
			people,
			func(p Person) []string { return []string{p.DepartmentID, p.CountryID} },
			func(p Person, lookup []string) PersonWithCountryDepartment {
				return PersonWithCountryDepartment{
					PersonID:       p.ID,
					Name:           fmt.Sprintf("%s %s", p.FirstName, p.LastName),
					DepartmentName: lookup[0],
					CountryName:    lookup[1],
				}
			},
			// lookup for departments
			func(departmentIds []string) ([]string, error) {
				assert.Equal(t, []string{"M"}, departmentIds, "only the Marketing department should be looked up, sales is already cached")

				return util.SliceTransform(departmentIds, func(deptId string) string {
					return departments[deptId].Name
				}), nil
			},
			// lookup for countries
			func(countryIds []string) ([]string, error) {
				assert.Fail(t, "Should not get here")
				return nil, nil
			},
		)

		assert.Nil(t, err)
		assert.Equal(t, []PersonWithCountryDepartment{
			{PersonID: "1", Name: "John Smith", CountryName: "zzNewZealand", DepartmentName: "zzSales"},
			{PersonID: "2", Name: "Jane Doe", CountryName: "zzNewZealand", DepartmentName: "Marketing"},
			{PersonID: "3", Name: "Alan Walker", CountryName: "zzAustralia", DepartmentName: "zzSales"},
		}, results)
	})

	t.Run("returns error if the first lookup fails", func(t *testing.T) {
		caches := util.MapCollectionCacheContainer{}

		results, err := util.MapCollectionWithLookups(
			&caches,
			people,
			func(p Person) []string { return []string{p.DepartmentID, p.CountryID} },
			func(p Person, lookup []string) PersonWithCountryDepartment {
				return PersonWithCountryDepartment{
					PersonID:       p.ID,
					Name:           fmt.Sprintf("%s %s", p.FirstName, p.LastName),
					DepartmentName: lookup[0],
					CountryName:    lookup[1],
				}
			},
			// lookup for departments
			func(departmentIds []string) ([]string, error) {
				return nil, errors.New("Can't lookup departments")
			},
			// lookup for countries
			func(countryIds []string) ([]string, error) {
				return util.SliceTransform(countryIds, func(cid string) string {
					return countries[cid].Name
				}), nil
			},
		)

		assert.EqualError(t, err, "Can't lookup departments")
		assert.Nil(t, results)
	})

	t.Run("returns error if the second lookup fails", func(t *testing.T) {
		caches := util.MapCollectionCacheContainer{}

		results, err := util.MapCollectionWithLookups(
			&caches,
			people,
			func(p Person) []string { return []string{p.DepartmentID, p.CountryID} },
			func(p Person, lookup []string) PersonWithCountryDepartment {
				return PersonWithCountryDepartment{
					PersonID:       p.ID,
					Name:           fmt.Sprintf("%s %s", p.FirstName, p.LastName),
					DepartmentName: lookup[0],
					CountryName:    lookup[1],
				}
			},
			// lookup for departments
			func(departmentIds []string) ([]string, error) {
				return util.SliceTransform(departmentIds, func(deptId string) string {
					return departments[deptId].Name
				}), nil
			},
			// lookup for countries
			func(countryIds []string) ([]string, error) {
				return nil, errors.New("Can't lookup countries")
			},
		)

		assert.EqualError(t, err, "Can't lookup countries")
		assert.Nil(t, results)
	})
}

func TestEmpty_Nil(t *testing.T) {
	var empty []string = nil
	assert.True(t, util.Empty(empty))
}

func TestEmpty_ZeroItems(t *testing.T) {
	assert.True(t, util.Empty([]string{}))
}

func TestEmpty_SomeItems(t *testing.T) {
	assert.False(t, util.Empty([]string{"value"}))
}
