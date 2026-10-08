//go:build !windows

package shell_test

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/util/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// TestQuotePowerShell_RoundTrip checks the quoted values survive PowerShell's own
// parser. pwsh is on the ubuntu runner image, so this does run on CI; it is a local
// machine without pwsh installed where it skips.
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

// TestQuotePowerShell_RoundTripToNativeCommand goes a step further than the test above:
// Write-Host is a cmdlet, so it only proves PowerShell parsed the value into one string.
// The CLI is a native executable, and handing an argument to one of those is a separate
// step with its own escaping. Running printf, which echoes its argument back verbatim,
// covers that step too. It is the step Windows PowerShell 5.1 gets wrong for a trailing
// backslash or an embedded quote, as quotePowerShell describes; 5.1 is Windows only and
// can't be exercised here, so this guards the PowerShell 7 behaviour we can reach.
func TestQuotePowerShell_RoundTripToNativeCommand(t *testing.T) {
	pwsh := lookShell(t, "pwsh")
	printf := lookShell(t, "printf")

	for _, value := range append(roundTripValues, "line1\nline2") {
		t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
			script := "& '" + printf + "' '%s' " + shell.Quote(shell.PowerShell, value)
			out, err := exec.Command(pwsh, "-NoProfile", "-Command", script).Output()
			require.NoError(t, err)
			assert.Equal(t, value, string(out))
		})
	}
}
