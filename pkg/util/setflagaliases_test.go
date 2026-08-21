package util_test

import (
	"strings"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func Test_SetFlagAliases(t *testing.T) {
	setup := func(value *[]string) *pflag.FlagSet {
		flags := pflag.NewFlagSet("flagset", pflag.ContinueOnError)
		flags.SetOutput(&strings.Builder{})
		flags.StringSliceVar(value, "deployment-target", nil, "usage")
		util.SetFlagAliases(flags, map[string][]string{
			"deployment-target": {"target", "specificMachines"},
		})
		return flags
	}

	t.Run("an alias sets the primary flag directly, no copy step", func(t *testing.T) {
		var targets []string
		flags := setup(&targets)
		assert.Nil(t, flags.Parse([]string{"--specificMachines", "ABC"}))
		assert.Equal(t, []string{"ABC"}, targets)
	})

	// this is the case util.ApplyFlagAliases cannot handle: it stringifies the alias and hand-splits
	// on commas, which tears a quoted element in half
	t.Run("an alias keeps an element that contains a quoted comma", func(t *testing.T) {
		var targets []string
		flags := setup(&targets)
		assert.Nil(t, flags.Parse([]string{"--specificMachines", `"Web, Prod",Other`}))
		assert.Equal(t, []string{"Web, Prod", "Other"}, targets)
	})

	// and this is why FlagOrAliasChanged is not needed: the alias marks the one real flag as Changed
	t.Run("an alias marks the primary flag as Changed", func(t *testing.T) {
		var targets []string
		flags := setup(&targets)
		assert.Nil(t, flags.Parse([]string{"--specificMachines", "ABC"}))
		assert.True(t, flags.Lookup("deployment-target").Changed)
	})

	t.Run("an alias set to an empty value is Changed with no values", func(t *testing.T) {
		var targets []string
		flags := setup(&targets)
		assert.Nil(t, flags.Parse([]string{"--specificMachines", ""}))
		assert.Empty(t, targets)
		assert.True(t, flags.Lookup("deployment-target").Changed)
	})

	t.Run("nothing specified leaves the primary flag unchanged", func(t *testing.T) {
		var targets []string
		flags := setup(&targets)
		assert.Nil(t, flags.Parse([]string{}))
		assert.False(t, flags.Lookup("deployment-target").Changed)
	})

	t.Run("primary and alias accumulate into the same flag", func(t *testing.T) {
		var targets []string
		flags := setup(&targets)
		assert.Nil(t, flags.Parse([]string{"--deployment-target", "ABC", "--target", "XYZ"}))
		assert.Equal(t, []string{"ABC", "XYZ"}, targets)
	})

	t.Run("a name that is neither primary nor alias is still an unknown flag", func(t *testing.T) {
		var targets []string
		flags := setup(&targets)
		assert.EqualError(t, flags.Parse([]string{"--specificMachine", "ABC"}), "unknown flag: --specificMachine")
	})

	// guards the mistake that led to the "SetNormalizeFunc doesn't work" conclusion in root.go:
	// normalizing onto a primary while the alias is also registered panics inside pflag with
	// "flag redefined". Fail with an explanation instead.
	t.Run("panics if an alias is also registered as a flag", func(t *testing.T) {
		flags := pflag.NewFlagSet("flagset", pflag.ContinueOnError)
		flags.StringSlice("deployment-target", nil, "usage")
		flags.StringSlice("specificMachines", nil, "usage")
		assert.PanicsWithValue(t,
			"bug! SetFlagAliases alias specificMachines is already registered as a flag; aliases must not be registered",
			func() {
				util.SetFlagAliases(flags, map[string][]string{"deployment-target": {"specificMachines"}})
			})
	})

	t.Run("panics if the primary flag does not exist", func(t *testing.T) {
		flags := pflag.NewFlagSet("flagset", pflag.ContinueOnError)
		assert.PanicsWithValue(t,
			"bug! SetFlagAliases couldn't find primary flag deployment-target in collection",
			func() {
				util.SetFlagAliases(flags, map[string][]string{"deployment-target": {"specificMachines"}})
			})
	})

	t.Run("works for a non-slice flag too", func(t *testing.T) {
		flags := pflag.NewFlagSet("flagset", pflag.ContinueOnError)
		var version string
		var force bool
		flags.StringVar(&version, "version", "", "usage")
		flags.BoolVar(&force, "force-package-download", false, "usage")
		util.SetFlagAliases(flags, map[string][]string{
			"version":                {"releaseNumber"},
			"force-package-download": {"forcePackageDownload"},
		})
		assert.Nil(t, flags.Parse([]string{"--releaseNumber", "1.0", "--forcePackageDownload"}))
		assert.Equal(t, "1.0", version)
		assert.True(t, force)
	})
}

func Test_TrimSliceFlagValues(t *testing.T) {
	setup := func(args ...string) (*pflag.FlagSet, []string) {
		flags := pflag.NewFlagSet("flagset", pflag.ContinueOnError)
		var values []string
		flags.StringSliceVar(&values, "deployment-target", nil, "usage")
		util.SetFlagAliases(flags, map[string][]string{"deployment-target": {"specificMachines"}})
		if err := flags.Parse(args); err != nil {
			t.Fatal(err)
		}
		return flags, values
	}

	t.Run("trims whitespace around comma separators", func(t *testing.T) {
		flags, values := setup("--deployment-target", "ABC, XYZ")
		result, err := util.TrimSliceFlagValues(flags, "deployment-target", values)
		assert.Nil(t, err)
		assert.Equal(t, []string{"ABC", "XYZ"}, result)
	})

	t.Run("drops a trailing comma rather than erroring", func(t *testing.T) {
		flags, values := setup("--deployment-target", "ABC,")
		result, err := util.TrimSliceFlagValues(flags, "deployment-target", values)
		assert.Nil(t, err)
		assert.Equal(t, []string{"ABC"}, result)
	})

	t.Run("errors when the flag was specified but resolved to nothing", func(t *testing.T) {
		flags, values := setup("--deployment-target", "")
		_, err := util.TrimSliceFlagValues(flags, "deployment-target", values)
		assert.EqualError(t, err, "--deployment-target was specified but resolved to no names")
	})

	t.Run("errors when only whitespace was specified", func(t *testing.T) {
		flags, values := setup("--deployment-target", "  ,  ")
		_, err := util.TrimSliceFlagValues(flags, "deployment-target", values)
		assert.EqualError(t, err, "--deployment-target was specified but resolved to no names")
	})

	t.Run("errors when the empty value arrived through an alias", func(t *testing.T) {
		flags, values := setup("--specificMachines", "")
		_, err := util.TrimSliceFlagValues(flags, "deployment-target", values)
		assert.EqualError(t, err, "--deployment-target was specified but resolved to no names")
	})

	t.Run("no error when the flag was never specified", func(t *testing.T) {
		flags, values := setup()
		result, err := util.TrimSliceFlagValues(flags, "deployment-target", values)
		assert.Nil(t, err)
		assert.Nil(t, result)
	})
}
