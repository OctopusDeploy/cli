package root

import (
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	accountCmd "github.com/OctopusDeploy/cli/pkg/cmd/account"
	environmentCmd "github.com/OctopusDeploy/cli/pkg/cmd/environment"
	spaceCmd "github.com/OctopusDeploy/cli/pkg/cmd/space"
	"github.com/spf13/cobra"
)

// NewCmdRoot returns the base command when called without any subcommands
func NewCmdRoot(client apiclient.ClientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "octopus",
		Short: "Octopus Deploy CLI",
		Long:  `Work seamlessly with Octopus Deploy from the command line.`,
	}

	cmd.PersistentFlags().BoolP("help", "h", false, "Show help for command")
	cmd.PersistentFlags().StringP("space", "s", "", "Set Space")
	cmd.PersistentFlags().StringP("outputFormat", "f", "", "Output Format (supported values: json)")

	// infrastructure commands
	cmd.AddCommand(accountCmd.NewCmdAccount(client))
	cmd.AddCommand(environmentCmd.NewCmdEnvironment(client))

	// configuration commands
	cmd.AddCommand(spaceCmd.NewCmdSpace(client))

	return cmd
}
