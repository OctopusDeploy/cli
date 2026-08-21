package shell

import "strings"

// Characters which carry no special meaning to the shell and so never need quoting.
// Letters and digits are always safe and aren't repeated here.
const (
	posixSafeChars      = `@%+=:,./-_`
	powerShellSafeChars = `+=:./\-_`
	cmdSafeChars        = `+=:./\-_`
)

// Quote renders value so that sh passes it to the CLI as a single argument, unchanged.
// Values which don't need quoting are returned as they are.
func Quote(sh Shell, value string) string {
	switch sh {
	case PowerShell:
		return quotePowerShell(value)
	case Cmd:
		return quoteCmd(value)
	default:
		return quotePosix(value)
	}
}

func isBare(value string, safeChars string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune(safeChars, r):
		default:
			return false
		}
	}
	return true
}

// quotePosix quotes for sh, bash, zsh and friends. Everything inside single quotes is
// literal, so only the single quote itself needs handling; it is closed, escaped with a
// backslash, and reopened.
func quotePosix(value string) string {
	if isBare(value, posixSafeChars) {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// quotePowerShell quotes for PowerShell. Single quoted strings are literal, and a single
// quote is escaped by doubling it.
func quotePowerShell(value string) string {
	if isBare(value, powerShellSafeChars) {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

// quoteCmd quotes for cmd.exe, which has to survive two passes: cmd's own parsing, and
// then the argv parsing the CLI does as it starts up.
//   - a literal double quote is `\"` for argv; the surrounding quotes are closed around it
//     and the quote is caret escaped so cmd's own quoting stays balanced
//   - backslashes only matter to argv where they run into a double quote, in which case
//     the whole run has to be doubled
//   - % is expanded even inside double quotes and can't be escaped there, so it is emitted
//     outside them as ^%
//
// Two things can't be fixed here: a newline can't be represented in cmd at all, and ! is
// expanded when delayed expansion has been switched on.
func quoteCmd(value string) string {
	if isBare(value, cmdSafeChars) {
		return value
	}

	var sb strings.Builder
	sb.WriteByte('"')
	backslashes := 0
	for i := 0; i < len(value); i++ {
		switch c := value[i]; c {
		case '\\':
			backslashes++
		case '"':
			sb.WriteString(strings.Repeat(`\`, backslashes*2))
			backslashes = 0
			sb.WriteString(`"\^""`)
		case '%':
			sb.WriteString(strings.Repeat(`\`, backslashes*2))
			backslashes = 0
			sb.WriteString(`"^%"`)
		default:
			sb.WriteString(strings.Repeat(`\`, backslashes))
			backslashes = 0
			sb.WriteByte(c)
		}
	}
	sb.WriteString(strings.Repeat(`\`, backslashes*2))
	sb.WriteByte('"')
	return sb.String()
}
