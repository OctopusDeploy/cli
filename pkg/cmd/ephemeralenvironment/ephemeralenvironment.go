package ephemeralenvironment

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/create"
	cmdDeprovisionEnvironment "github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/deprovision-environment"
	cmdDeprovisionProject "github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/deprovision-project"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdEphemeralEnvironment(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ephemeral-environment <command>",
		Short: "Manage ephemeral environments",
		Long:  "Manage ephemeral environments in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s ephemeral-environment create --name "MyEphemeralEnvironment" --project "MyProject"
		`, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsInfrastructure: "true",
		},
	}

	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdDeprovisionEnvironment.NewCmdDeprovisionEnvironment(f))
	cmd.AddCommand(cmdDeprovisionProject.NewCmdDeprovisionProject(f))

	return cmd
}
