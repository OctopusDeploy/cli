package util

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

// TrimSpaceAndDropEmpty trims surrounding whitespace from each value and drops any value
// left empty, preserving order and duplicates. Returns nil if input is nil or empty, or if all
// entries are empty or contain only whitespace.
//
// It deliberately does NOT split on commas. pflag's stringSlice type has already done the
// splitting, honouring CSV quoting so that a value which legitimately contains a comma can
// be passed as "Web, Prod". Splitting again here would break exactly those values. This only
// restores the trim and drop-blanks semantics the legacy octo CLI had:
//
//	v.Split(new[] { ',' }, StringSplitOptions.RemoveEmptyEntries).Select(m => m.Trim())
func TrimSpaceAndDropEmpty(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// TrimSliceFlagValues tidies up the values of a stringSlice flag with TrimSpaceAndDropEmpty, and
// rejects the case where the flag was given on the command line but nothing survived.
//
// That case has to be an error rather than being treated as "flag not specified", because for a
// flag that narrows what a command acts on, the two mean opposite things. pflag parses
// --deployment-target "" to an empty slice, so `--deployment-target "$TARGETS"` with $TARGETS unset
// would otherwise read as no target restriction at all and deploy to every target in the
// environment; the same shape turns --tenant "" into an untenanted deployment. Erroring keeps the
// old stringArray behaviour of failing rather than widening, but fails locally with a clear message
// instead of sending a blank name to the server.
func TrimSliceFlagValues(flags *pflag.FlagSet, name string, values []string) ([]string, error) {
	trimmed := TrimSpaceAndDropEmpty(values)
	if len(trimmed) == 0 {
		if f := flags.Lookup(name); f != nil && f.Changed {
			return nil, fmt.Errorf("--%s was specified but resolved to no names", name)
		}
	}
	return trimmed, nil
}

// QuoteForCSV CSV-quotes a single value if it needs it: if it contains a comma or a double quote,
// it is wrapped in double quotes with any interior double quote doubled, matching the CSV quoting
// pflag's stringSlice type reads (and writes) for its elements. A value needing neither is
// returned unchanged.
//
// This exists so a value round-trips through GenerateAutomationCmd, which shell-single-quotes each
// element but does not itself apply CSV quoting - without this, an element containing a comma or a
// quote would print as an automation command that parses back into the wrong values (or fails to
// parse at all).
func QuoteForCSV(value string) string {
	if !strings.ContainsAny(value, `,"`) {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
