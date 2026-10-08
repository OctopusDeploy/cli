package shell_test

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/util/shell"
	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		expected shell.Shell
		ok       bool
	}{
		{"bash", shell.Bash, true},
		{"BASH", shell.Bash, true},
		{" zsh ", shell.Bash, true},
		{"sh", shell.Bash, true},
		{"ksh", shell.Bash, true},
		{"dash", shell.Bash, true},
		{"powershell", shell.PowerShell, true},
		{"powershell.exe", shell.PowerShell, true},
		{"pwsh", shell.PowerShell, true},
		{"pwsh.exe", shell.PowerShell, true},
		{"cmd", shell.Cmd, true},
		{"cmd.exe", shell.Cmd, true},
		{"", "", false},
		{"fish", "", false},
		{"nushell", "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, ok := shell.Parse(test.name)
			assert.Equal(t, test.ok, ok)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestValidate(t *testing.T) {
	assert.NoError(t, shell.Validate("cmd"))
	assert.EqualError(t, shell.Validate("fish"), "the provided value fish is not a valid shell, please use one of bash, powershell, cmd")
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		env      map[string]string
		parent   string
		expected shell.Shell
	}{
		{"unix with SHELL", "linux", map[string]string{"SHELL": "/bin/zsh"}, "", shell.Bash},
		{"unix with unknown SHELL", "linux", map[string]string{"SHELL": "/usr/bin/fish"}, "", shell.Bash},
		{"unix without SHELL", "darwin", nil, "", shell.Bash},
		{"unix with pwsh as SHELL", "linux", map[string]string{"SHELL": "/usr/local/bin/pwsh"}, "", shell.PowerShell},
		{"windows falls back to cmd", "windows", nil, "", shell.Cmd},
		{"windows ignores PSModulePath", "windows", map[string]string{"PSModulePath": `C:\Program Files\WindowsPowerShell\Modules`}, "", shell.Cmd},
		{"windows reads the parent process", "windows", nil, "powershell.exe", shell.PowerShell},
		{"override wins on unix", "linux", map[string]string{"SHELL": "/bin/bash", "OCTOPUS_SHELL": "cmd"}, "", shell.Cmd},
		{"override wins on windows", "windows", map[string]string{"OCTOPUS_SHELL": "pwsh"}, "", shell.PowerShell},
		{"invalid override is ignored", "linux", map[string]string{"SHELL": "/bin/bash", "OCTOPUS_SHELL": "fish"}, "", shell.Bash},
		// $SHELL names the login shell, so it keeps saying bash while the user is sat in
		// pwsh; the parent process is the one that knows
		{"unix in pwsh started from bash", "linux", map[string]string{"SHELL": "/bin/bash"}, "pwsh", shell.PowerShell},
		{"unix parent beats SHELL", "darwin", map[string]string{"SHELL": "/bin/bash"}, "zsh", shell.Bash},
		{"unix falls back to SHELL for an unknown parent", "linux", map[string]string{"SHELL": "/usr/local/bin/pwsh"}, "make", shell.PowerShell},
		{"unix override beats the parent process", "linux", map[string]string{"OCTOPUS_SHELL": "cmd"}, "pwsh", shell.Cmd},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			getenv := func(key string) string { return test.env[key] }
			parent := func() string { return test.parent }
			assert.Equal(t, test.expected, shell.Detect(test.goos, getenv, parent))
		})
	}
}
