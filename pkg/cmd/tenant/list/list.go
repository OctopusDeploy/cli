package list

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tenants in Octopus Deploy",
		Long:  "List tenants in Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s tenant list
			$ %s tenant ls
		`), constants.ExecutableName, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE:

	}
}

func listRun(cmd *cobra.Command, f factory.Factory)