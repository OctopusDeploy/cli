package environment

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	cmdDelete "github.com/OctopusDeploy/cli/pkg/cmd/environment/delete"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/environment/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdEnvironment(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "environment <command>",
		Short: "Manage environments",
		Long:  "Work with Octopus Deploy environments.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s environment list
			$ %s environment ls
		`), constants.ExecutableName, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsInfrastructure: "true",
		},
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdDelete.NewCmdDelete(f))
	return cmd
}
