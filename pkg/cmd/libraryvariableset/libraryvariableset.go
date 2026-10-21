package libraryvariableset

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/libraryvariableset/list"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/libraryvariableset/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdLibraryVariableSet(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "library-variable-set <command>",
		Aliases: []string{"library-variable-sets", "lvs"},
		Short:   "Manage library variable sets",
		Long:    "Manage library variable sets in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s library-variable-set list
			%[1]s library-variable-set view "Slack Variables"
		`, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsLibrary: "true",
		},
	}

	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdView.NewCmdView(f))

	return cmd
}
