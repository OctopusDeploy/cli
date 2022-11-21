package version

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdVersion(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "version",
		Hidden:  true,
		Example: heredoc.Docf("$ %s version", constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println(f.BuildVersion())
			return nil
		},
	}

	return cmd
}
