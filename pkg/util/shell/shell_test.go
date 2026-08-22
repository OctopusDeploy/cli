package shell_test

import (
	"runtime"
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
		name              string
		goos              string
		env               map[string]string
		expected          shell.Shell
		usesParentProcess bool
	}{
		{"unix with SHELL", "linux", map[string]string{"SHELL": "/bin/zsh"}, shell.Bash, false},
		{"unix with unknown SHELL", "linux", map[string]string{"SHELL": "/usr/bin/fish"}, shell.Bash, false},
		{"unix without SHELL", "darwin", nil, shell.Bash, false},
		{"unix with pwsh as SHELL", "linux", map[string]string{"SHELL": "/usr/local/bin/pwsh"}, shell.PowerShell, false},
		{"windows falls back to cmd", "windows", nil, shell.Cmd, true},
		{"windows ignores PSModulePath", "windows", map[string]string{"PSModulePath": `C:\Program Files\WindowsPowerShell\Modules`}, shell.Cmd, true},
		{"override wins on unix", "linux", map[string]string{"SHELL": "/bin/bash", "OCTOPUS_SHELL": "cmd"}, shell.Cmd, false},
		{"override wins on windows", "windows", map[string]string{"OCTOPUS_SHELL": "pwsh"}, shell.PowerShell, false},
		{"invalid override is ignored", "linux", map[string]string{"SHELL": "/bin/bash", "OCTOPUS_SHELL": "fish"}, shell.Bash, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.usesParentProcess && runtime.GOOS == "windows" {
				t.Skip("the real parent process is inspected when actually running on windows")
			}
			getenv := func(key string) string { return test.env[key] }
			assert.Equal(t, test.expected, shell.Detect(test.goos, getenv))
		})
	}
}
