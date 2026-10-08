package shell

import (
	"fmt"
	"strings"
)

// limitation is one shape of value a shell can't carry, and what it does to it.
type limitation struct {
	name   string
	effect string
	match  func(value string) bool
}

// limits describes what a shell does to the values it can't carry. leadIn introduces
// them, because the two shells fail differently enough to need different framing: cmd
// only breaks down once the command is in a script, while 5.1 mangles the value wherever
// it is pasted.
type limits struct {
	leadIn      string
	limitations []limitation
}

func contains(substr string) func(string) bool {
	return func(value string) bool { return strings.Contains(value, substr) }
}

// unsupported lists, per shell, the values a quoted command can't reliably carry. The
// posix shells have none: single quotes make anything literal, and the shell hands the
// argument to the program without a second round of parsing to get wrong.
//
// PowerShell's entries are not about its quoting, which is sound, but about what Windows
// PowerShell 5.1 does afterwards, when it rebuilds the command line for a native command
// without escaping it; quotePowerShell has the detail. They are warned about for every
// PowerShell because a generated command is text the user carries elsewhere, so the
// shell it eventually runs in isn't necessarily the one it was generated in, and 5.1 is
// still the one that opens from a Windows Start menu. The wording says which PowerShell
// is affected so that a reader on 7 can see it doesn't apply to them.
var unsupported = map[Shell]limits{
	Cmd: {
		leadIn: "this command can't be pasted into a script as it is",
		limitations: []limitation{
			{"%", "is expanded before any escaping is applied, so it only survives at the interactive prompt; a .bat or .cmd script drops an unmatched % and replaces %var%", contains("%")},
			{"!", "is expanded when delayed expansion is switched on, which strips it and anything it encloses", contains("!")},
			{"a line break", "ends the command in cmd, and nothing can quote it", contains("\n")},
		},
	},
	PowerShell: {
		leadIn: "Windows PowerShell 5.1 corrupts this command as it hands the arguments to the CLI, though PowerShell 7 runs it correctly",
		limitations: []limitation{
			{"a trailing backslash", "escapes the closing quote 5.1 puts around the value, so the value arrives with a quote stuck on the end", func(value string) bool { return strings.HasSuffix(value, `\`) }},
			{"a double quote", "is passed on unescaped, so 5.1 loses it from the value", contains(`"`)},
			{"an empty value", "is dropped from the command line altogether, so the flag before it takes whatever follows as its value", func(value string) bool { return value == "" }},
		},
	},
}

// PasteWarning returns a warning for the values that can't make it through the shell
// intact, or "" when they all can. The generated command is meant to be copied and
// pasted, and quoting alone can't tell the user that what they're about to paste is
// going to be silently mangled.
func PasteWarning(sh Shell, values ...string) string {
	problems := unsupported[sh]
	if len(problems.limitations) == 0 {
		return ""
	}

	var found []string
	seen := map[string]bool{}
	for _, p := range problems.limitations {
		for _, v := range values {
			if p.match(v) && !seen[p.name] {
				seen[p.name] = true
				found = append(found, fmt.Sprintf("%s %s", p.name, p.effect))
			}
		}
	}
	if len(found) == 0 {
		return ""
	}

	return fmt.Sprintf("Warning: %s: %s.", problems.leadIn, strings.Join(found, "; and "))
}
