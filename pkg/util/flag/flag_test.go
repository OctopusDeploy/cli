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

func TestGenerateAutomationCmdForShell_WarnsWhenCmdCannotCarryAValue(t *testing.T) {
	project := flag.New[string]("project", false)
	project.Value = "100% Cotton"

	actual := flag.GenerateAutomationCmdForShell(shell.Cmd, "octopus release deploy", "", project)

	assert.Contains(t, actual, `--project "100"^%" Cotton"`)
	assert.Contains(t, actual, "\nWarning: this command can't be pasted into a script as it is:")
	assert.Contains(t, actual, "%")
}

// the same value is fine in a posix shell, so nothing is appended
func TestGenerateAutomationCmdForShell_DoesNotWarnForPosix(t *testing.T) {
	project := flag.New[string]("project", false)
	project.Value = "100% Cotton"

	actual := flag.GenerateAutomationCmdForShell(shell.Bash, "octopus release deploy", "", project)

	assert.Equal(t, `octopus release deploy --project '100% Cotton' --no-prompt`, actual)
}

// the space is quoted like any other value, so it has to be checked too
func TestGenerateAutomationCmdForShell_WarnsAboutTheSpaceName(t *testing.T) {
	name := flag.New[string]("name", false)
	name.Value = "Dev"

	actual := flag.GenerateAutomationCmdForShell(shell.Cmd, "octopus environment create", "100% Cotton", name)

	assert.Contains(t, actual, "Warning: this command can't be pasted into a script as it is:")
}
