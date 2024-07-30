package progression

import (
	"github.com/MakeNowJust/heredoc/v2"
	cmdAllow "github.com/OctopusDeploy/cli/pkg/cmd/release/progression/allow"
	cmdPrevent "github.com/OctopusDeploy/cli/pkg/cmd/release/progression/prevent"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdProgression(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "progression <command>",
		Short: "Manage progression of a release",
		Long:  "Manage progression of a release in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s release progression prevent
			$ %[1]s release progression allow
		`, constants.ExecutableName),
	}

	cmd.AddCommand(cmdAllow.NewCmdAllow(f))
	cmd.AddCommand(cmdPrevent.NewCmdPrevent(f))

	return cmd
}
