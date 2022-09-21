package root

import (
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	accountCmd "github.com/OctopusDeploy/cli/pkg/cmd/account"
	configCmd "github.com/OctopusDeploy/cli/pkg/cmd/config"
	environmentCmd "github.com/OctopusDeploy/cli/pkg/cmd/environment"
	releaseCmd "github.com/OctopusDeploy/cli/pkg/cmd/release"
	spaceCmd "github.com/OctopusDeploy/cli/pkg/cmd/space"
	"github.com/OctopusDeploy/cli/pkg/cmd/version"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewCmdRoot returns the base command when called without any subcommands
// note we explicitly pass in clientFactory and askProvider here because we configure them,
// which the Factory wrapper deliberately doesn't allow us to do
func NewCmdRoot(f factory.Factory, clientFactory apiclient.ClientFactory, askProvider question.AskProvider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "octopus <command>",
		Short: "Octopus Deploy CLI",
		Long:  `Work seamlessly with Octopus Deploy from the command line.`,
	}

	// ----- Child Commands -----

	cmd.AddCommand(version.NewCmdVersion(f))

	// infrastructure
	cmd.AddCommand(accountCmd.NewCmdAccount(f))
	cmd.AddCommand(environmentCmd.NewCmdEnvironment(f))

	// configuration
	cmd.AddCommand(configCmd.NewCmdConfig(f))
	cmd.AddCommand(spaceCmd.NewCmdSpace(f))
	cmd.AddCommand(releaseCmd.NewCmdRelease(f))

	// ----- Configuration -----

	// commands are expected to print their own errors to avoid double-ups
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	cmdPFlags := cmd.PersistentFlags()

	cmdPFlags.BoolP(constants.FlagHelp, "h", false, "Show help for command")
	cmd.SetHelpFunc(rootHelpFunc)
	cmdPFlags.StringP(constants.FlagSpace, "s", "", "Set Space")

	// remember if you read FlagOutputFormat you also need to check FlagOutputFormatLegacy
	cmdPFlags.StringP(constants.FlagOutputFormat, "f", "", "Output Format (Valid values are 'json', 'table', 'basic'. Defaults to table)")

	cmdPFlags.BoolP(constants.FlagNoPrompt, "", false, "disable prompting in interactive mode")

	// Legacy flags brought across from the .NET CLI.
	// Consumers of these flags will have to explicitly check for them as well as the new
	// flags. The pflag documentation says you can use SetNormalizeFunc to translate/alias flag
	// names, however this doesn't actually work; It normalizes both the old and new flag
	// names to the same thing at configuration time, then panics due to duplicate flag declarations.

	cmdPFlags.String(constants.FlagOutputFormatLegacy, "", "")
	_ = cmdPFlags.MarkHidden(constants.FlagOutputFormatLegacy)

	flagAliases := map[string][]string{constants.FlagOutputFormat: {constants.FlagOutputFormatLegacy}}

	_ = viper.BindPFlag(constants.ConfigNoPrompt, cmdPFlags.Lookup(constants.FlagNoPrompt))
	_ = viper.BindPFlag(constants.ConfigSpace, cmdPFlags.Lookup(constants.FlagSpace))
	// if we attempt to check the flags before Execute is called, cobra hasn't parsed anything yet,
	// so we'll get bad values. PersistentPreRun is a convenient callback for setting up our
	// environment after parsing but before execution.
	cmd.PersistentPreRun = func(_ *cobra.Command, _ []string) {
		// map flag alias values
		for k, v := range flagAliases {
			for _, aliasName := range v {
				f := cmdPFlags.Lookup(aliasName)
				r := f.Value.String() // boolean flags get stringified here but it's fast enough and a one-shot so meh
				if r != f.DefValue {
					_ = cmdPFlags.Lookup(k).Value.Set(r)
				}
			}
		}

		if noPrompt := viper.GetBool(constants.ConfigNoPrompt); noPrompt {
			askProvider.DisableInteractive()
		}

		if spaceNameOrId := viper.GetString(constants.ConfigSpace); spaceNameOrId != "" {
			clientFactory.SetSpaceNameOrId(spaceNameOrId)
		}
	}

	return cmd
}
