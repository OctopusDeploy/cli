package root

import (
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/spf13/cobra"
)

// NewCmdRoot returns the base command when called without any subcommands
func NewCmdRoot(client *apiclient.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "octopus",
		Short: "Octopus Deploy CLI",
		Long:  `Work seamlessly with Octopus Deploy from the command line.`,
	}
	cmd.PersistentFlags().BoolP("help", "h", false, "Show help for command")
	cmd.PersistentFlags().StringP("space", "s", "", "Set Space")
	return cmd
}
