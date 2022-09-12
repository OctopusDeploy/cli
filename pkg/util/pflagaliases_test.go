package util_test

import (
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_FlagAliases_string(t *testing.T) {
	setup := func(value *string) (*pflag.FlagSet, map[string][]string) {
		flags := pflag.NewFlagSet("flagset", pflag.ContinueOnError)
		flags.StringVar(value, "some-flag", "", "usage")
		aliases := map[string][]string{}
		util.AddFlagAliasesString(flags, "some-flag", aliases, "someFlag", "sf")
		return flags, aliases
	}

	t.Run("basic", func(t *testing.T) {
		var someFlagValue string
		flags, aliases := setup(&someFlagValue)
		assert.Nil(t, flags.Parse([]string{"--someFlag", "someValue"}))

		// sanity check, the value should have been parsed into the hidden "someFlag" flag, but not copied into the primary "some-flag" yet
		assert.Equal(t, "", someFlagValue)

		util.ApplyFlagAliases(flags, aliases)
		assert.Equal(t, "someValue", someFlagValue)
	})

	t.Run("alt", func(t *testing.T) {
		var someFlagValue string
		flags, aliases := setup(&someFlagValue)
		assert.Nil(t, flags.Parse([]string{"--sf", "someValue"}))

		assert.Equal(t, "", someFlagValue)

		util.ApplyFlagAliases(flags, aliases)
		assert.Equal(t, "someValue", someFlagValue)
	})
}

func Test_FlagAliases_bool(t *testing.T) {
	setup := func(value *bool) (*pflag.FlagSet, map[string][]string) {
		flags := pflag.NewFlagSet("flagset", pflag.ContinueOnError)
		flags.BoolVar(value, "some-flag", false, "usage")
		aliases := map[string][]string{}
		util.AddFlagAliasesBool(flags, "some-flag", aliases, "someFlag", "sf")
		return flags, aliases
	}

	t.Run("basic", func(t *testing.T) {
		var someFlagValue bool
		flags, aliases := setup(&someFlagValue)
		assert.Nil(t, flags.Parse([]string{"--someFlag", "true"}))

		assert.Equal(t, false, someFlagValue)

		util.ApplyFlagAliases(flags, aliases)
		assert.Equal(t, true, someFlagValue)
	})

	t.Run("alt", func(t *testing.T) {
		var someFlagValue bool
		flags, aliases := setup(&someFlagValue)
		assert.Nil(t, flags.Parse([]string{"--sf", "true"}))

		assert.Equal(t, false, someFlagValue)

		util.ApplyFlagAliases(flags, aliases)
		assert.Equal(t, true, someFlagValue)
	})

	t.Run("no-opt", func(t *testing.T) {
		var someFlagValue bool
		flags, aliases := setup(&someFlagValue)
		assert.Nil(t, flags.Parse([]string{"--someFlag"}))

		assert.Equal(t, false, someFlagValue)

		util.ApplyFlagAliases(flags, aliases)
		assert.Equal(t, true, someFlagValue)
	})
}

func Test_FlagAliases_slice(t *testing.T) {
	setup := func(value *[]string) (*pflag.FlagSet, map[string][]string) {
		flags := pflag.NewFlagSet("flagset", pflag.ContinueOnError)
		flags.StringSliceVar(value, "some-flag", nil, "usage")
		aliases := map[string][]string{}
		util.AddFlagAliasesStringSlice(flags, "some-flag", aliases, "someFlag", "sf")
		return flags, aliases
	}

	t.Run("basic", func(t *testing.T) {
		var someFlagValue []string
		flags, aliases := setup(&someFlagValue)
		assert.Nil(t, flags.Parse([]string{"--someFlag", "someValue"}))

		// sanity check, the value should have been parsed into the hidden "someFlag" flag, but not copied into the primary "some-flag" yet
		assert.Nil(t, someFlagValue)

		util.ApplyFlagAliases(flags, aliases)
		assert.Equal(t, []string{"someValue"}, someFlagValue)
	})

	t.Run("alt", func(t *testing.T) {
		var someFlagValue []string
		flags, aliases := setup(&someFlagValue)
		assert.Nil(t, flags.Parse([]string{"--sf", "someValue"}))

		assert.Nil(t, someFlagValue)

		util.ApplyFlagAliases(flags, aliases)
		assert.Equal(t, []string{"someValue"}, someFlagValue)
	})

	t.Run("multiple", func(t *testing.T) {
		var someFlagValue []string
		flags, aliases := setup(&someFlagValue)
		assert.Nil(t, flags.Parse([]string{"--someFlag", "someValue", "--someFlag", "secondValue", "--someFlag", "thirdValue"}))

		// sanity check, the value should have been parsed into the hidden "someFlag" flag, but not copied into the primary "some-flag" yet
		assert.Nil(t, someFlagValue)

		util.ApplyFlagAliases(flags, aliases)
		assert.Equal(t, []string{"someValue", "secondValue", "thirdValue"}, someFlagValue)
	})

	t.Run("mixed", func(t *testing.T) {
		var someFlagValue []string
		flags, aliases := setup(&someFlagValue)
		assert.Nil(t, flags.Parse([]string{"--some-flag", "initialValue", "--someFlag", "alias1value", "--sf", "alias2value"}))

		// sanity check, the value should have been parsed into the hidden "someFlag" flag, but not copied into the primary "some-flag" yet
		assert.Equal(t, []string{"initialValue"}, someFlagValue)

		util.ApplyFlagAliases(flags, aliases)
		assert.Equal(t, []string{"initialValue", "alias1value", "alias2value"}, someFlagValue)
	})
}
