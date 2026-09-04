package shell_test

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/util/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuote(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		posix      string
		powerShell string
		cmd        string
	}{
		{"plain word", "Dev", "Dev", "Dev", "Dev"},
		{"version number", "0.0.3", "0.0.3", "0.0.3", "0.0.3"},
		{"tenant tag", "Regions/us-east", "Regions/us-east", "Regions/us-east", "Regions/us-east"},
		{"variable assignment", "Name:Value", "Name:Value", "Name:Value", "Name:Value"},
		{"embedded equals", "a=b", "a=b", "a=b", "a=b"},
		{"leading equals", "=foo", `'=foo'`, "=foo", "=foo"},
		{"empty", "", `''`, `''`, `""`},
		{"space", "Soft Drinks", `'Soft Drinks'`, `'Soft Drinks'`, `"Soft Drinks"`},
		{"single quote", "it's", `'it'\''s'`, `'it''s'`, `"it's"`},
		{"single quote and space", "it's here", `'it'\''s here'`, `'it''s here'`, `"it's here"`},
		{"double quote", `say "hi"`, `'say "hi"'`, `'say "hi"'`, `"say "\^""hi"\^"""`},
		{"backtick", "a`b", "'a`b'", "'a`b'", "\"a`b\""},
		{"dollar variable", "$HOME", `'$HOME'`, `'$HOME'`, `"$HOME"`},
		{"percent variable", "%PATH%", `%PATH%`, `'%PATH%'`, `""^%"PATH"^%""`},
		{"newline", "line1\nline2", "'line1\nline2'", "'line1\nline2'", "\"line1\nline2\""},
		{"comma", "a,b", "a,b", `'a,b'`, `"a,b"`},
		{"tilde", "~/tmp", `'~/tmp'`, `'~/tmp'`, `"~/tmp"`},
		{"masked secret", "*****", `'*****'`, `'*****'`, `"*****"`},
		{"ampersand", "A & B", `'A & B'`, `'A & B'`, `"A & B"`},
		{"windows path", `C:\Program Files\Octopus\`, `'C:\Program Files\Octopus\'`, `'C:\Program Files\Octopus\'`, `"C:\Program Files\Octopus\\"`},
		{"backslash then quote", `a\"b`, `'a\"b'`, `'a\"b'`, `"a\\"\^""b"`},
		{"non ascii", "Café", `'Café'`, `'Café'`, `"Café"`},
		{"curly apostrophe", "Bob’s Project", `'Bob’s Project'`, `'Bob’’s Project'`, `"Bob’s Project"`},
		{"other smart single quotes", "a‘b‚c‛d", `'a‘b‚c‛d'`, `'a‘‘b‚‚c‛‛d'`, `"a‘b‚c‛d"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.posix, shell.Quote(shell.Bash, test.value), "bash")
			assert.Equal(t, test.powerShell, shell.Quote(shell.PowerShell, test.value), "powershell")
			assert.Equal(t, test.cmd, shell.Quote(shell.Cmd, test.value), "cmd")
		})
	}
}

// roundTripValues are fed through the real quoting and then back out again by the
// round trip tests below.
var roundTripValues = []string{
	"Dev",
	"0.0.3",
	"Regions/us-east",
	"Soft Drinks",
	"",
	"it's",
	"it's here",
	`say "hi"`,
	"a`b",
	"$HOME",
	"%PATH%",
	"a,b",
	"a=b",
	"=foo",
	"~/tmp",
	"*****",
	"A & B",
	"a|b",
	"a>b<c",
	"(a)",
	"a!b",
	"a^b",
	`C:\Program Files\Octopus\`,
	`a\"b`,
	`\\server\share\`,
	"Café",
	"Bob’s Project",
	"trailing space ",
	"#comment",
	"a;b",
	"a*b?c[d]",
	"{a}",
}

// TestQuoteCmd_RoundTrip pushes each quoted value through cmd.exe's parsing rules and
// then through the argv parsing the CLI does on startup, and checks it comes out
// unchanged. Newlines are left out because a newline ends the line in cmd, which no
// amount of quoting can work around.
func TestQuoteCmd_RoundTrip(t *testing.T) {
	for _, value := range roundTripValues {
		t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
			line := "octopus.exe release deploy --project " + shell.Quote(shell.Cmd, value)

			afterCmd, err := simulateCmd(line)
			require.NoError(t, err)

			args := parseArgv(afterCmd)
			assert.Equal(t, []string{"octopus.exe", "release", "deploy", "--project", value}, args)
		})
	}
}

// lookShell finds a shell to round trip through. Locally a missing shell just skips the
// test, but on CI it fails: these tests are the only thing that checks the generated
// quoting against a real parser, and a silent skip there means the coverage quietly
// disappears the day the runner image or the workflow changes.
func lookShell(t *testing.T, name string) string {
	t.Helper()

	path, err := exec.LookPath(name)
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("%s is not installed; it is needed to round trip the generated quoting", name)
		}
		t.Skipf("%s is not available", name)
	}
	return path
}

// TestQuotePosix_RoundTrip runs the quoted values through a real /bin/sh.
func TestQuotePosix_RoundTrip(t *testing.T) {
	sh := lookShell(t, "sh")

	for _, value := range append(roundTripValues, "line1\nline2") {
		t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
			script := "printf '%s' " + shell.Quote(shell.Bash, value)
			out, err := exec.Command(sh, "-c", script).Output()
			require.NoError(t, err)
			assert.Equal(t, value, string(out))
		})
	}
}

// TestQuoteZsh_RoundTrip runs the quoted values through a real zsh, which the bash
// quoting also covers. zsh expands a leading = and a leading ~ where the other posix
// shells don't, so it is the stricter test of the two.
func TestQuoteZsh_RoundTrip(t *testing.T) {
	zsh := lookShell(t, "zsh")

	for _, value := range append(roundTripValues, "line1\nline2") {
		t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
			script := "printf '%s' " + shell.Quote(shell.Bash, value)
			out, err := exec.Command(zsh, "--no-rcs", "-c", script).Output()
			require.NoError(t, err)
			assert.Equal(t, value, string(out))
		})
	}
}

// TestQuotePowerShell_RoundTrip runs the quoted values through pwsh when it happens to
// be installed; it isn't on CI, so this usually skips.
func TestQuotePowerShell_RoundTrip(t *testing.T) {
	pwsh := lookShell(t, "pwsh")

	for _, value := range append(roundTripValues, "line1\nline2") {
		t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
			script := "Write-Host -NoNewline " + shell.Quote(shell.PowerShell, value)
			out, err := exec.Command(pwsh, "-NoProfile", "-Command", script).Output()
			require.NoError(t, err)
			assert.Equal(t, value, string(out))
		})
	}
}

// simulateCmd applies cmd.exe's own processing to a command line and returns what cmd
// would hand to the program. It models the interactive prompt, which is where a copied
// command gets pasted; percent expansion in a batch file follows different rules and a
// value containing % doesn't survive there, as quoteCmd's comment explains.
// Outside double quotes a caret escapes the next character;
// quotes themselves are passed through for the program to deal with. An unescaped % or
// metacharacter outside quotes is reported as an error rather than simulated, because
// either one means the generated command is broken.
func simulateCmd(line string) (string, error) {
	var sb strings.Builder
	inQuote := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '"':
			inQuote = !inQuote
			sb.WriteByte(c)
		case c == '^' && !inQuote:
			i++
			if i >= len(line) {
				return "", fmt.Errorf("trailing caret in %q", line)
			}
			sb.WriteByte(line[i])
		case c == '%':
			return "", fmt.Errorf("unescaped %% in %q; cmd expands it even inside quotes", line)
		case !inQuote && strings.ContainsRune(`&|<>()^`, rune(c)):
			return "", fmt.Errorf("unquoted metacharacter %q in %q", c, line)
		default:
			sb.WriteByte(c)
		}
	}
	if inQuote {
		return "", fmt.Errorf("unbalanced quotes in %q", line)
	}
	return sb.String(), nil
}

// parseArgv splits a windows command line into arguments the same way the go runtime
// does when it populates os.Args. The rules are documented at
// http://daviddeley.com/autohotkey/parameters/parameters.htm#WINARGV, including the
// "prior to 2008" handling of a doubled quote inside a quoted run.
func parseArgv(line string) []string {
	var args []string
	var current []byte
	var backslashes int
	started := false
	inQuote := false

	appendBackslashes := func(n int) {
		for ; n > 0; n-- {
			current = append(current, '\\')
		}
	}

	for i := 0; i < len(line); i++ {
		c := line[i]
		switch c {
		case '\\':
			backslashes++
			continue
		case '"':
			appendBackslashes(backslashes / 2)
			if backslashes%2 == 0 {
				if inQuote && i+1 < len(line) && line[i+1] == '"' {
					current = append(current, '"')
					i++
				}
				inQuote = !inQuote
			} else {
				current = append(current, '"')
			}
			backslashes = 0
			started = true
			continue
		case ' ', '\t':
			if !inQuote {
				appendBackslashes(backslashes)
				backslashes = 0
				if started {
					args = append(args, string(current))
					current = nil
					started = false
				}
				continue
			}
		}
		appendBackslashes(backslashes)
		backslashes = 0
		current = append(current, c)
		started = true
	}

	appendBackslashes(backslashes)
	if started {
		args = append(args, string(current))
	}
	return args
}
