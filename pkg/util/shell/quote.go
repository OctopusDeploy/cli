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
	// = is harmless in the middle of a word but a word which starts with one is subject
	// to zsh's =cmd expansion, so `=foo` aborts the whole command with "foo not found".
	if !strings.HasPrefix(value, "=") && isBare(value, posixSafeChars) {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// powerShellQuoteEscaper doubles every character PowerShell's tokenizer accepts as a
// single quote. As well as the ascii one those are the unicode "smart" variants, which
// close a single quoted string just like ' does; a value carrying a curly apostrophe
// from a web UI (Bob’s Project) would otherwise be a parse error. Doubling the same
// character is what escapes it, so the value still comes back byte for byte.
var powerShellQuoteEscaper = strings.NewReplacer(
	`'`, `''`,
	"‘", "‘‘", // left single quotation mark
	"’", "’’", // right single quotation mark
	"‚", "‚‚", // single low-9 quotation mark
	"‛", "‛‛", // single high-reversed-9 quotation mark
)

// quotePowerShell quotes for PowerShell. Single quoted strings are literal, and a single
// quote is escaped by doubling it.
func quotePowerShell(value string) string {
	if isBare(value, powerShellSafeChars) {
		return value
	}
	return "'" + powerShellQuoteEscaper.Replace(value) + "'"
}

// quoteCmd quotes for cmd.exe, which has to survive two passes: cmd's own parsing, and
// then the argv parsing the CLI does as it starts up.
//   - a literal double quote is `\"` for argv; the surrounding quotes are closed around it
//     and the quote is caret escaped so cmd's own quoting stays balanced
//   - backslashes only matter to argv where they run into a double quote, in which case
//     the whole run has to be doubled
//   - % is expanded even inside double quotes, so it is emitted outside them as ^%
//
// Three things can't be fixed here:
//   - a newline can't be represented in cmd at all
//   - ! is expanded when delayed expansion has been switched on
//   - % only survives at the interactive prompt. The caret doesn't really escape it,
//     because percent expansion is an earlier parsing phase than caret processing; what
//     saves us is that the prompt leaves an unmatched % and an undefined %var% alone.
//     A batch file doesn't: it drops an unmatched % and deletes an undefined %var%
//     outright, so `100% Done` and `%PATH%` both come out mangled there. The batch
//     escape is %%, which in turn doesn't collapse at the prompt, so no single encoding
//     works for both and the interactive one is the one worth having.
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
