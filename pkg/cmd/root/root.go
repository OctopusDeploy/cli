package root

import (
	accountCmd "github.com/OctopusDeploy/cli/pkg/cmd/account"
	environmentCmd "github.com/OctopusDeploy/cli/pkg/cmd/environment"
	releaseCmd "github.com/OctopusDeploy/cli/pkg/cmd/release"
	spaceCmd "github.com/OctopusDeploy/cli/pkg/cmd/space"
	"github.com/OctopusDeploy/cli/pkg/constants"
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

	cmdPFlags := cmd.PersistentFlags()

	cmdPFlags.BoolP(constants.FlagHelp, "h", false, "Show help for command")
	cmdPFlags.StringP(constants.FlagSpace, "s", "", "Set Space")

	// remember if you read FlagOutputFormat you also need to check FlagOutputFormatLegacy
	cmdPFlags.StringP(constants.FlagOutputFormat, "f", "", "Output Format (Valid values are 'json', 'table', 'basic'. Defaults to table)")

	cmdPFlags.BoolP(constants.FlagNoPrompt, "", false, "disable prompting in interactive mode")

	// Legacy flags brought across from the .NET CLI.
	// Consumers of these flags will have to explicitly check for them as well as the new
	// flags. The pflag documentation says you can use SetNormalizeFunc to translate/alias flag
	// names, however this doesn't actually work; It normalizes both the old and new flag
	// names to the same thing at configuration time, then panics due to duplicate flag declarations.
	cmdPFlags.StringP(constants.FlagOutputFormatLegacy, "", "", "Output Format")
	_ = cmdPFlags.MarkHidden(constants.FlagOutputFormatLegacy)

	// we want to allow outputFormat as well as output-format, but don't advertise it.
	// must add this AFTER setting the normalize func or it strips out the flag

	// infrastructure commands
	cmd.AddCommand(accountCmd.NewCmdAccount(f))
	cmd.AddCommand(environmentCmd.NewCmdEnvironment(f))

	// configuration commands
	cmd.AddCommand(spaceCmd.NewCmdSpace(f))

	cmd.AddCommand(releaseCmd.NewCmdRelease(f))

	return cmd
}
