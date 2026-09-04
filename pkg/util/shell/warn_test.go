package shell_test

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/util/shell"
	"github.com/stretchr/testify/assert"
)

func TestPasteWarning(t *testing.T) {
	tests := []struct {
		name     string
		shell    shell.Shell
		values   []string
		contains []string
	}{
		{"cmd is fine with an ordinary value", shell.Cmd, []string{"Soft Drinks", "0.0.3"}, nil},
		{"cmd warns about a percent", shell.Cmd, []string{"100% Cotton"}, []string{"%"}},
		{"cmd warns about an environment variable", shell.Cmd, []string{"%PATH%"}, []string{"%"}},
		{"cmd warns about a bang", shell.Cmd, []string{"Ship it!"}, []string{"!"}},
		{"cmd warns about a line break", shell.Cmd, []string{"line1\nline2"}, []string{"a line break"}},
		{"cmd reports every problem it finds", shell.Cmd, []string{"100%", "Ship it!"}, []string{"%", "!"}},
		{"cmd only reports each problem once", shell.Cmd, []string{"100%", "50%"}, []string{"%"}},
		{"cmd checks every value", shell.Cmd, []string{"fine", "100% Cotton"}, []string{"%"}},
		// posix shells and PowerShell can quote anything, so there is never a warning
		{"bash is always fine", shell.Bash, []string{"100% Cotton", "Ship it!", "line1\nline2"}, nil},
		{"powershell is always fine", shell.PowerShell, []string{"100% Cotton", "Ship it!", "line1\nline2"}, nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := shell.PasteWarning(test.shell, test.values...)

			if test.contains == nil {
				assert.Equal(t, "", actual)
				return
			}
			assert.Contains(t, actual, "Warning:")
			for _, want := range test.contains {
				assert.Contains(t, actual, want)
			}
		})
	}
}

func TestPasteWarning_NoValues(t *testing.T) {
	assert.Equal(t, "", shell.PasteWarning(shell.Cmd))
}
