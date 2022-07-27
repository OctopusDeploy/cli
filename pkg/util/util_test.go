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
		// need to preallocate two caches
		caches := []map[string]string{
			{}, {},
		}

		results, err := util.MapCollectionWithLookups(
			caches,
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
				return util.MapSlice(departmentIds, func(deptId string) string {
					return departments[deptId].Name
				}), nil
			},
			// lookup for countries
			func(countryIds []string) ([]string, error) {
				return util.MapSlice(countryIds, func(cid string) string {
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

	t.Run("just one lookup", func(t *testing.T) {
		// just one cache needed
		caches := []map[string]string{
			{},
		}

		results, err := util.MapCollectionWithLookups(
			caches,
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
				return util.MapSlice(departmentIds, func(deptId string) string {
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
		var caches []map[string]string = nil

		results, err := util.MapCollectionWithLookups(
			caches,
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

	t.Run("doesn't use lookup if values are already cached", func(t *testing.T) {
		// preload the cache with not-quite-right data to check that the function fetches it from the cache rather than lookup
		caches := []map[string]string{
			{"S": "zzSales", "M": "zzMarketing"},
		}

		results, err := util.MapCollectionWithLookups(
			caches,
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

	t.Run("returns error if the first lookup fails", func(t *testing.T) {
		// need to preallocate two caches
		caches := []map[string]string{
			{}, {},
		}

		results, err := util.MapCollectionWithLookups(
			caches,
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
				return util.MapSlice(countryIds, func(cid string) string {
					return countries[cid].Name
				}), nil
			},
		)

		assert.EqualError(t, err, "Can't lookup departments")
		assert.Nil(t, results)
	})

	t.Run("returns error if the second lookup fails", func(t *testing.T) {
		// need to preallocate two caches
		caches := []map[string]string{
			{}, {},
		}

		results, err := util.MapCollectionWithLookups(
			caches,
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
				return util.MapSlice(departmentIds, func(deptId string) string {
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
