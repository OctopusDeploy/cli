package util

import (
	"strings"

	"github.com/spf13/pflag"
)

func AddFlagAliasesString(flags *pflag.FlagSet, originalFlag string, aliasMap map[string][]string, aliases ...string) {
	f := flags.Lookup(originalFlag)
	if f == nil {
		panic("bug! AddFlagAliasesString couldn't find original flag in collection")
	}
	for _, alias := range aliases {
		flags.String(alias, f.DefValue, "")
		_ = flags.MarkHidden(alias)
	}
	aliasMap[originalFlag] = aliases
}

func AddFlagAliasesBool(flags *pflag.FlagSet, originalFlag string, aliasMap map[string][]string, aliases ...string) {
	f := flags.Lookup(originalFlag)
	if f == nil {
		panic("bug! AddFlagAliasesBool couldn't find original flag in collection")
	}
	for _, alias := range aliases {
		flags.Bool(alias, false, "") // this would be broken if we had any bools with default value of true, but we don't
		_ = flags.MarkHidden(alias)
	}
	aliasMap[originalFlag] = aliases
}

func AddFlagAliasesStringSlice(flags *pflag.FlagSet, originalFlag string, aliasMap map[string][]string, aliases ...string) {
	f := flags.Lookup(originalFlag)
	if f == nil {
		panic("bug! AddFlagAliasesStringSlice couldn't find original flag in collection")
	}
	for _, alias := range aliases {
		flags.StringSlice(alias, nil, "")
		_ = flags.MarkHidden(alias)
	}
	aliasMap[originalFlag] = aliases
}

func ApplyFlagAliases(flags *pflag.FlagSet, aliases map[string][]string) {
	// find values that may have been specified using a flag alias, and copy the values across to the primary flags
	for k, v := range aliases {
		primaryFlag := flags.Lookup(k)
		for _, aliasName := range v {
			aliasFlag := flags.Lookup(aliasName)
			aliasValueString := aliasFlag.Value.String() // flags get stringified here, but it's fast enough and a one-shot so meh
			if aliasValueString != aliasFlag.DefValue {
				// we have to call set because .Value holds the pointer to the bound variable;
				// if we set one Value to another we end up pointing at different storage and it doesn't work

				if aliasFlag.DefValue == "[]" && len(aliasValueString) > 2 && aliasValueString[0] == '[' {
					// this is not great. We rely on the assumption that pflag's internal Set(string) calls readAsCsv in a
					// predictable way that doesn't change. However, there is nothing in the pflag public API that would
					// allow us to do a better job, as flag values are only exposed via the `Value` interface which
					// only allows read/write of values using String.
					aliasValues := strings.Split(aliasValueString[1:len(aliasValueString)-1], ",")
					for _, aliasValue := range aliasValues {
						_ = primaryFlag.Value.Set(aliasValue)
					}
				} else {
					_ = primaryFlag.Value.Set(aliasValueString)
				}
			}
		}
	}
}

// SetFlagAliases makes each alias in aliasMap an alternate spelling of its primary flag, using
// pflag's flag-name normalization. Unlike AddFlagAliases*/ApplyFlagAliases there is no second flag
// and nothing to copy: --specificMachines X and --deployment-target X reach the same flag, so an
// alias behaves exactly as the primary does - same type, same parsing, same Changed field.
//
// That last part is why this is not just tidier. ApplyFlagAliases copies a value across with
// Value.Set(), which does not set the primary's Changed field (only FlagSet.Set and parsing do
// that), so a value that arrived through an alias is indistinguishable from one never supplied.
// With normalization there is only one flag, and its Changed field is the truth.
//
// The aliases must NOT also be registered as flags. Normalizing two registered names onto one
// primary panics inside pflag with "flag redefined" - that is what an earlier attempt hit, leading
// to the conclusion recorded in pkg/cmd/root/root.go that normalization can't be used for
// aliasing. Registering only the primary is the point, so this panics early on a name that is
// already taken rather than leaving a registered flag silently shadowed.
//
// aliasMap is keyed by primary flag name, matching the map AddFlagAliases* builds.
func SetFlagAliases(flags *pflag.FlagSet, aliasMap map[string][]string) {
	normalized := make(map[string]string, len(aliasMap)*2)
	for primary, aliases := range aliasMap {
		if flags.Lookup(primary) == nil {
			panic("bug! SetFlagAliases couldn't find primary flag " + primary + " in collection")
		}
		for _, alias := range aliases {
			if flags.Lookup(alias) != nil {
				panic("bug! SetFlagAliases alias " + alias + " is already registered as a flag; aliases must not be registered")
			}
			normalized[alias] = primary
		}
	}
	flags.SetNormalizeFunc(func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		if primary, ok := normalized[name]; ok {
			return pflag.NormalizedName(primary)
		}
		return pflag.NormalizedName(name)
	})
}
