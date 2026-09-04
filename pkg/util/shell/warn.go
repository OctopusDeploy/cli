package shell

import (
	"fmt"
	"strings"
)

// unsupported lists, per shell, the characters a quoted value can't reliably carry, and
// what goes wrong. Only cmd has any: posix shells and PowerShell can quote anything.
var unsupported = map[Shell][]struct {
	char   string
	name   string
	effect string
}{
	Cmd: {
		{"%", "%", "is expanded before any escaping is applied, so it only survives at the interactive prompt; a .bat or .cmd script drops an unmatched % and replaces %var%"},
		{"!", "!", "is expanded when delayed expansion is switched on, which strips it and anything it encloses"},
		{"\n", "a line break", "ends the command in cmd, and nothing can quote it"},
	},
}

// PasteWarning returns a warning for the values that can't make it through the shell
// intact, or "" when they all can. The generated command is meant to be copied and
// pasted, and quoting alone can't tell the user that what they're about to paste is
// going to be silently mangled.
func PasteWarning(sh Shell, values ...string) string {
	problems := unsupported[sh]
	if len(problems) == 0 {
		return ""
	}

	var found []string
	seen := map[string]bool{}
	for _, p := range problems {
		for _, v := range values {
			if strings.Contains(v, p.char) && !seen[p.name] {
				seen[p.name] = true
				found = append(found, fmt.Sprintf("%s %s", p.name, p.effect))
			}
		}
	}
	if len(found) == 0 {
		return ""
	}

	return fmt.Sprintf("Warning: this command can't be pasted into a script as it is: %s.", strings.Join(found, "; and "))
}
