package util

import "github.com/spf13/pflag"

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
					_ = primaryFlag.Value.Set(aliasValueString[1 : len(aliasValueString)-1])
				} else {
					_ = primaryFlag.Value.Set(aliasValueString)
				}
			}
		}
	}
}
