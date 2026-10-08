//go:build windows

package shell_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/util/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const argvEchoMarker = "ARGVECHO:"

// buildArgvEcho compiles the helper that reports the arguments it received, and returns
// the path to it.
//
// OCTOPUS_TEST_ARGVECHO points at an already built copy instead. These tests are the
// ones most worth running on a Windows box that isn't CI, and such a box often has no Go
// toolchain on it; cross compiling the test binary and the helper from the developer's
// own machine is then the only way to run them there at all.
func buildArgvEcho(t *testing.T) string {
	t.Helper()

	if prebuilt := os.Getenv("OCTOPUS_TEST_ARGVECHO"); prebuilt != "" {
		return prebuilt
	}

	exe := filepath.Join(t.TempDir(), "argvecho.exe")
	build := exec.Command("go", "build", "-o", exe, "./testdata/argvecho")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "building argvecho: %s", out)
	return exe
}

// argvEchoResult picks the helper's line out of the shell's output and decodes it.
func argvEchoResult(t *testing.T, output string) []string {
	t.Helper()

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, argvEchoMarker) {
			continue
		}
		var args []string
		require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, argvEchoMarker)), &args))
		return args
	}
	t.Fatalf("the command produced no output from argvecho; the shell got:\n%s", output)
	return nil
}

// runAtCmdPrompt types line at a real interactive cmd.exe and returns everything it
// printed.
//
// The line goes in on stdin rather than through `cmd /c` on purpose. The generated
// command exists to be pasted at a prompt, and the prompt is not the same parser as
// `/c`, which has its own rule for stripping the outer quotes; % in particular behaves
// differently again in a batch file, which is why quoteCmd only claims the prompt.
// Feeding stdin is the only one of the three that is actually the case we ship.
func runAtCmdPrompt(t *testing.T, line string) string {
	t.Helper()

	cmd := exec.Command("cmd.exe")
	// Go builds a child's command line by quoting each argument, which would rewrite the
	// very escaping under test. CmdLine hands cmd.exe the bytes we mean.
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd.exe`}
	cmd.Stdin = strings.NewReader("@echo off\r\n" + line + "\r\nexit\r\n")

	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "running cmd.exe: %s", out)
	return string(out)
}

// isASCII reports whether the harness can deliver value to cmd at all.
//
// Everything above reaches cmd as bytes on a redirected stdin, and cmd decodes those
// with an OEM codepage that has no é in it; chcp doesn't help, because it sets the
// console codepage and there is no console here. A real prompt never has the problem,
// since Windows hands a console its input as UTF-16 and builds the child's command line
// in UTF-16 too. So this is a limit of feeding cmd through a pipe, not of the quoting,
// and the non-ascii values are covered by the simulated round trip and by both
// PowerShell round trips, which pass their arguments as UTF-16 and carry them intact.
func isASCII(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] > 127 {
			return false
		}
	}
	return true
}

// TestQuoteCmd_RoundTripRealCmd is TestQuoteCmd_RoundTrip against the real thing. The
// simulator that test uses encodes cmd's documented rules, which is worth having because
// it runs everywhere, but it can only ever confirm that the quoting agrees with our
// reading of the documentation. This one pastes the generated command at a real cmd.exe
// prompt and checks the value arrives byte for byte.
//
// Newlines are left out for the same reason as in the simulated test: a newline ends the
// line in cmd and no quoting can carry it. PasteWarning is what covers that case.
func TestQuoteCmd_RoundTripRealCmd(t *testing.T) {
	exe := buildArgvEcho(t)

	for _, value := range roundTripValues {
		t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
			if !isASCII(value) {
				t.Skip("the harness can't deliver this value, see isASCII")
			}

			line := shell.Quote(shell.Cmd, exe) + " release deploy --project " + shell.Quote(shell.Cmd, value)

			args := argvEchoResult(t, runAtCmdPrompt(t, line))
			assert.Equal(t, []string{"release", "deploy", "--project", value}, args)
		})
	}
}

// TestQuotePowerShell_RoundTripToNativeCommandWindows is the Windows counterpart of
// TestQuotePowerShell_RoundTripToNativeCommand. Both PowerShells on the box are tried:
// pwsh 7 should carry every value, while Windows PowerShell 5.1 rebuilds a native
// command's arguments without escaping them and so corrupts three shapes of value no
// matter how the literal is written. Those three are skipped rather than asserted as
// broken, because the day 5.1 stops mangling them is not a day this test should fail.
func TestQuotePowerShell_RoundTripToNativeCommandWindows(t *testing.T) {
	exe := buildArgvEcho(t)

	for _, name := range []string{"pwsh", "powershell"} {
		t.Run(name, func(t *testing.T) {
			ps := lookShell(t, name)

			for _, value := range roundTripValues {
				t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
					if name == "powershell" && (strings.Contains(value, `"`) || strings.HasSuffix(value, `\`) || value == "") {
						t.Skip("Windows PowerShell 5.1 corrupts this shape of value on its way to a native command; see quotePowerShell")
					}

					script := "& " + shell.Quote(shell.PowerShell, exe) + " release deploy --project " + shell.Quote(shell.PowerShell, value)
					out, err := exec.Command(ps, "-NoProfile", "-Command", script).CombinedOutput()
					require.NoError(t, err, "running %s: %s", name, out)

					args := argvEchoResult(t, string(out))
					assert.Equal(t, []string{"release", "deploy", "--project", value}, args)
				})
			}
		})
	}
}
