package ephemeralenvironment

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/create"
	cmdDelete "github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/delete"
	cmdDeprovisionEnvironment "github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/deprovision-environment"
	cmdDeprovisionProject "github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/deprovision-project"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/list"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
)

func NewCmdEphemeralEnvironment(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ephemeral-environment <command>",
		Short: "Manage ephemeral environments",
		Long:  "Manage ephemeral environments in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s ephemeral-environment create --name "MyEphemeralEnvironment" --project "MyProject"
			%[1]s ephemeral-environment list --project "MyProject"
			%[1]s ephemeral-environment view --name "MyEphemeralEnvironment" --project "MyProject"
			%[1]s ephemeral-environment delete --name "MyEphemeralEnvironment"
		`, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsInfrastructure: "true",
		},
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdDeprovisionEnvironment.NewCmdDeprovisionEnvironment(f))
	cmd.AddCommand(cmdDeprovisionProject.NewCmdDeprovisionProject(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdView.NewCmdView(f))
	cmd.AddCommand(cmdDelete.NewCmdDelete(f))

	return cmd
}
