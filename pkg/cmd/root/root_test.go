package root

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

// newOutputFormatFlags mirrors the way NewCmdRoot registers the output format flags,
// including the non-empty default which is what makes Changed() necessary.
func newOutputFormatFlags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.StringP(constants.FlagOutputFormat, "f", constants.OutputFormatTable, "")
	flags.String(constants.FlagOutputFormatLegacy, "", "")
	return flags
}

func TestResolveOutputFormat(t *testing.T) {
	tests := []struct {
		name             string
		flag             string // --output-format, empty means not supplied
		legacyFlag       string // --outputFormat, empty means not supplied
		noPrompt         bool
		configuredFormat string
		expected         string
	}{
		{name: "defaults to table", expected: constants.OutputFormatTable},
		{name: "explicit flag is honoured", flag: "json", expected: constants.OutputFormatJson},
		{name: "explicit flag is normalised", flag: "  JSON ", expected: constants.OutputFormatJson},
		{name: "legacy flag is honoured", legacyFlag: "json", expected: constants.OutputFormatJson},
		{name: "config file setting is honoured", configuredFormat: "json", expected: constants.OutputFormatJson},
		{name: "flag beats config file", flag: "basic", configuredFormat: "json", expected: constants.OutputFormatBasic},
		{name: "legacy flag beats config file", legacyFlag: "basic", configuredFormat: "json", expected: constants.OutputFormatBasic},
		// the flag's non-empty default used to mask this, so --no-prompt never took effect
		{name: "no-prompt falls back to basic", noPrompt: true, expected: constants.OutputFormatBasic},
		{name: "flag beats no-prompt", flag: "json", noPrompt: true, expected: constants.OutputFormatJson},
		{name: "explicitly requesting table beats no-prompt", flag: "table", noPrompt: true, expected: constants.OutputFormatTable},
		{name: "config file beats no-prompt", noPrompt: true, configuredFormat: "json", expected: constants.OutputFormatJson},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			flags := newOutputFormatFlags()
			if test.flag != "" {
				assert.NoError(t, flags.Set(constants.FlagOutputFormat, test.flag))
			}
			if test.legacyFlag != "" {
				assert.NoError(t, flags.Set(constants.FlagOutputFormatLegacy, test.legacyFlag))
				// NewCmdRoot copies the legacy value across without marking the new flag as Changed
				assert.NoError(t, flags.Lookup(constants.FlagOutputFormat).Value.Set(test.legacyFlag))
			}

			actual, warning, err := resolveOutputFormat(flags, test.noPrompt, test.configuredFormat)

			assert.NoError(t, err)
			assert.Empty(t, warning)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestResolveOutputFormat_RejectsUnsupportedFormats(t *testing.T) {
	// commands that hand-roll their own format switch have no default case, so an unsupported
	// format used to print nothing at all and exit 0
	tests := []struct {
		name       string
		flag       string
		legacyFlag string
	}{
		{name: "from the flag", flag: "xml"},
		{name: "from the legacy flag", legacyFlag: "yaml"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			flags := newOutputFormatFlags()
			if test.flag != "" {
				assert.NoError(t, flags.Set(constants.FlagOutputFormat, test.flag))
			}
			if test.legacyFlag != "" {
				assert.NoError(t, flags.Set(constants.FlagOutputFormatLegacy, test.legacyFlag))
				assert.NoError(t, flags.Lookup(constants.FlagOutputFormat).Value.Set(test.legacyFlag))
			}

			_, _, err := resolveOutputFormat(flags, false, "")

			assert.ErrorContains(t, err, "unsupported output format")
		})
	}
}

// an unsupported value in the config file must not be fatal: this runs ahead of every command,
// so failing hard would lock the user out of the `config set` that would fix it
func TestResolveOutputFormat_WarnsAndFallsBackForAnUnsupportedConfigFileValue(t *testing.T) {
	flags := newOutputFormatFlags()

	actual, warning, err := resolveOutputFormat(flags, false, "csv")

	assert.NoError(t, err)
	assert.Equal(t, constants.OutputFormatTable, actual)
	assert.Contains(t, warning, "unsupported output format 'csv'")
	assert.Contains(t, warning, constants.ConfigOutputFormat)
}

func TestResolveOutputFormat_AnExplicitFlagStillWinsOverAnUnsupportedConfigFileValue(t *testing.T) {
	flags := newOutputFormatFlags()
	assert.NoError(t, flags.Set(constants.FlagOutputFormat, "json"))

	actual, warning, err := resolveOutputFormat(flags, false, "csv")

	assert.NoError(t, err)
	assert.Equal(t, constants.OutputFormatJson, actual)
	assert.NotEmpty(t, warning)
}

func TestUnsupportedOutputFormatMessage(t *testing.T) {
	assert.Equal(t,
		"unsupported output format ''. Valid values are 'json', 'table', 'basic'",
		constants.UnsupportedOutputFormatMessage(""))
}

func TestIsValidOutputFormat(t *testing.T) {
	assert.True(t, constants.IsValidOutputFormat(constants.OutputFormatJson))
	assert.True(t, constants.IsValidOutputFormat(constants.OutputFormatTable))
	assert.True(t, constants.IsValidOutputFormat(constants.OutputFormatBasic))
	assert.True(t, constants.IsValidOutputFormat("JSON"), "should be case-insensitive")
	assert.False(t, constants.IsValidOutputFormat(""))
	assert.False(t, constants.IsValidOutputFormat("xml"))
}
