package root

import (
	accountCmd "github.com/OctopusDeploy/cli/pkg/cmd/account"
	environmentCmd "github.com/OctopusDeploy/cli/pkg/cmd/environment"
	spaceCmd "github.com/OctopusDeploy/cli/pkg/cmd/space"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

// NewCmdRoot returns the base command when called without any subcommands
func NewCmdRoot(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "octopus",
		Short: "Octopus Deploy CLI",
		Long:  `Work seamlessly with Octopus Deploy from the command line.`,
	}

	cmd.PersistentFlags().BoolP("help", "h", false, "Show help for command")
	cmd.PersistentFlags().StringP("space", "s", "", "Set Space")
	cmd.PersistentFlags().StringP("outputFormat", "f", "", "Output Format (Valid values are 'json', 'table', 'basic'. Defaults to table)")

	// infrastructure commands
	cmd.AddCommand(accountCmd.NewCmdAccount(f))
	cmd.AddCommand(environmentCmd.NewCmdEnvironment(f))

	// configuration commands
	cmd.AddCommand(spaceCmd.NewCmdSpace(f))

	return cmd
}
