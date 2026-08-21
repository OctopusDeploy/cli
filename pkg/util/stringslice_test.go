package util_test

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestTrimSpaceAndDropEmpty(t *testing.T) {
	t.Run("trims surrounding whitespace", func(t *testing.T) {
		assert.Equal(t, []string{"ABC", "XYZ"}, util.TrimSpaceAndDropEmpty([]string{"ABC", " XYZ"}))
	})

	t.Run("drops empty entries", func(t *testing.T) {
		assert.Equal(t, []string{"ABC"}, util.TrimSpaceAndDropEmpty([]string{"ABC", ""}))
	})

	t.Run("drops whitespace-only entries", func(t *testing.T) {
		assert.Equal(t, []string{"ABC"}, util.TrimSpaceAndDropEmpty([]string{"ABC", "   "}))
	})

	t.Run("preserves internal whitespace", func(t *testing.T) {
		assert.Equal(t, []string{"first Machine", "second Machine"},
			util.TrimSpaceAndDropEmpty([]string{"first Machine", " second Machine"}))
	})

	t.Run("does not split on commas", func(t *testing.T) {
		// pflag's stringSlice has already split, honouring quotes. Splitting again here
		// would destroy values that legitimately contain a comma.
		assert.Equal(t, []string{"Web, Prod"}, util.TrimSpaceAndDropEmpty([]string{"Web, Prod"}))
	})

	t.Run("preserves order and duplicates", func(t *testing.T) {
		assert.Equal(t, []string{"b", "a", "b"}, util.TrimSpaceAndDropEmpty([]string{" b", "a ", "b"}))
	})

	t.Run("nil in, nil out", func(t *testing.T) {
		assert.Nil(t, util.TrimSpaceAndDropEmpty(nil))
	})

	t.Run("all-empty input returns nil", func(t *testing.T) {
		assert.Nil(t, util.TrimSpaceAndDropEmpty([]string{"", "  "}))
	})

	t.Run("empty slice in, nil out", func(t *testing.T) {
		assert.Nil(t, util.TrimSpaceAndDropEmpty([]string{}))
	})
}

func TestQuoteForCSV(t *testing.T) {
	t.Run("element with no comma or quote is returned unchanged", func(t *testing.T) {
		assert.Equal(t, "Web", util.QuoteForCSV("Web"))
	})

	t.Run("element containing a comma is wrapped in double quotes", func(t *testing.T) {
		assert.Equal(t, `"Web, Prod"`, util.QuoteForCSV("Web, Prod"))
	})

	t.Run("element containing a double quote is wrapped, and the interior quote is doubled", func(t *testing.T) {
		assert.Equal(t, `"He said ""hi"""`, util.QuoteForCSV(`He said "hi"`))
	})

	t.Run("element containing both a comma and a quote", func(t *testing.T) {
		assert.Equal(t, `"He said, ""hi"" there"`, util.QuoteForCSV(`He said, "hi" there`))
	})

	t.Run("empty string is returned unchanged", func(t *testing.T) {
		assert.Equal(t, "", util.QuoteForCSV(""))
	})
}
