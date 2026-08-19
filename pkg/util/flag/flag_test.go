package flag_test

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/cli/pkg/util/shell"
	"github.com/stretchr/testify/assert"
)

func TestGenerateAutomationCmdForShell(t *testing.T) {
	project := flag.New[string]("project", false)
	project.Value = "Soft Drinks"
	version := flag.New[string]("version", false)
	version.Value = "0.0.3"
	environments := flag.New[[]string]("environment", false)
	environments.Value = []string{"Dev", "Test Environment"}
	tenantTag := flag.New[string]("tenant-tag", false)
	tenantTag.Value = "Regions/us-east"
	force := flag.New[bool]("force-package-download", false)
	force.Value = true
	timeout := flag.New[int]("timeout", false)
	timeout.Value = 30
	password := flag.New[string]("password", true)
	password.Value = "hunter2"
	empty := flag.New[string]("description", false)

	flags := []flag.Generatable{project, version, environments, tenantTag, force, timeout, password, empty}

	tests := []struct {
		shell    shell.Shell
		expected string
	}{
		{
			shell.Bash,
			`octopus release deploy --space 'Default Space' --project 'Soft Drinks' --version 0.0.3 --environment Dev --environment 'Test Environment' --tenant-tag Regions/us-east --force-package-download --timeout 30 --password '***' --no-prompt`,
		},
		{
			shell.PowerShell,
			`octopus release deploy --space 'Default Space' --project 'Soft Drinks' --version 0.0.3 --environment Dev --environment 'Test Environment' --tenant-tag Regions/us-east --force-package-download --timeout 30 --password '***' --no-prompt`,
		},
		{
			shell.Cmd,
			`octopus release deploy --space "Default Space" --project "Soft Drinks" --version 0.0.3 --environment Dev --environment "Test Environment" --tenant-tag Regions/us-east --force-package-download --timeout 30 --password "***" --no-prompt`,
		},
	}

	for _, test := range tests {
		t.Run(string(test.shell), func(t *testing.T) {
			actual := flag.GenerateAutomationCmdForShell(test.shell, "octopus release deploy", "Default Space", flags...)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestGenerateAutomationCmdForShell_NoSpace(t *testing.T) {
	name := flag.New[string]("name", false)
	name.Value = "Dev"

	actual := flag.GenerateAutomationCmdForShell(shell.Bash, "octopus environment create", "", name)
	assert.Equal(t, "octopus environment create --name Dev --no-prompt", actual)
}
