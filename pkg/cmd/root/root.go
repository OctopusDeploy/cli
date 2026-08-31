package root

import (
	"errors"
	"fmt"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	accountCmd "github.com/OctopusDeploy/cli/pkg/cmd/account"
	apiCmd "github.com/OctopusDeploy/cli/pkg/cmd/api"
	buildInfoCmd "github.com/OctopusDeploy/cli/pkg/cmd/buildinformation"
	channelCmd "github.com/OctopusDeploy/cli/pkg/cmd/channel"
	configCmd "github.com/OctopusDeploy/cli/pkg/cmd/config"
	environmentCmd "github.com/OctopusDeploy/cli/pkg/cmd/environment"
	ephemeralEnvironmentCmd "github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment"
	loginCmd "github.com/OctopusDeploy/cli/pkg/cmd/login"
	logoutCmd "github.com/OctopusDeploy/cli/pkg/cmd/logout"
	packageCmd "github.com/OctopusDeploy/cli/pkg/cmd/package"
	projectCmd "github.com/OctopusDeploy/cli/pkg/cmd/project"
	projectGroupCmd "github.com/OctopusDeploy/cli/pkg/cmd/projectgroup"
	releaseCmd "github.com/OctopusDeploy/cli/pkg/cmd/release"
	runbookCmd "github.com/OctopusDeploy/cli/pkg/cmd/runbook"
	spaceCmd "github.com/OctopusDeploy/cli/pkg/cmd/space"
	deploymentTargetCmd "github.com/OctopusDeploy/cli/pkg/cmd/target"
	taskCmd "github.com/OctopusDeploy/cli/pkg/cmd/task"
	tenantCmd "github.com/OctopusDeploy/cli/pkg/cmd/tenant"
	userCmd "github.com/OctopusDeploy/cli/pkg/cmd/user"
	"github.com/OctopusDeploy/cli/pkg/cmd/version"
	workerCmd "github.com/OctopusDeploy/cli/pkg/cmd/worker"
	workerPoolCmd "github.com/OctopusDeploy/cli/pkg/cmd/workerpool"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

	flags := cmd.Flags()
	var versionParameter bool
	flags.BoolVarP(&versionParameter, "version", "v", false, "Prints version information")
	versionCommand := version.NewCmdVersion(f)

	// ----- Child Commands -----

	cmd.AddCommand(versionCommand)

	// infrastructure
	cmd.AddCommand(accountCmd.NewCmdAccount(f))
	cmd.AddCommand(environmentCmd.NewCmdEnvironment(f))
	cmd.AddCommand(ephemeralEnvironmentCmd.NewCmdEphemeralEnvironment(f))
	cmd.AddCommand(packageCmd.NewCmdPackage(f))
	cmd.AddCommand(buildInfoCmd.NewCmdBuildInformation(f))
	cmd.AddCommand(deploymentTargetCmd.NewCmdDeploymentTarget(f))
	cmd.AddCommand(workerCmd.NewCmdWorker(f))
	cmd.AddCommand(workerPoolCmd.NewCmdWorkerPool(f))

	// core
	cmd.AddCommand(projectGroupCmd.NewCmdProjectGroup(f))
	cmd.AddCommand(projectCmd.NewCmdProject(f))
	cmd.AddCommand(channelCmd.NewCmdChannel(f))
	cmd.AddCommand(tenantCmd.NewCmdTenant(f))
	cmd.AddCommand(taskCmd.NewCmdTask(f))

	// configuration
	cmd.AddCommand(configCmd.NewCmdConfig(f))
	cmd.AddCommand(spaceCmd.NewCmdSpace(f))
	cmd.AddCommand(loginCmd.NewCmdLogin(f))
	cmd.AddCommand(logoutCmd.NewCmdLogout(f))

	cmd.AddCommand(userCmd.NewCmdUser(f))
	cmd.AddCommand(releaseCmd.NewCmdRelease(f))
	cmd.AddCommand(runbookCmd.NewCmdRunbook(f))

	cmd.AddCommand(apiCmd.NewCmdAPI(f))

	// ----- Configuration -----

	// commands are expected to print their own errors to avoid double-ups
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	cmdPFlags := cmd.PersistentFlags()

	cmdPFlags.BoolP(constants.FlagHelp, "h", false, "Show help for a command")
	cmd.SetHelpFunc(rootHelpFunc)
	cmdPFlags.StringP(constants.FlagSpace, "s", "", "Specify the space for operations")

	// remember if you read FlagOutputFormat you also need to check FlagOutputFormatLegacy
	cmdPFlags.StringP(constants.FlagOutputFormat, "f", constants.OutputFormatTable, `Specify the output format for a command ("json", "table", or "basic")`)

	cmdPFlags.BoolP(constants.FlagNoPrompt, "", false, "Disable prompting in interactive mode")

	// Enable service messages flag is hidden as it's intended for internal CI/CD use only
	cmdPFlags.BoolP(constants.FlagEnableServiceMessages, "", false, "Enable service messages for integration with Octopus CI/CD")
	cmdPFlags.MarkHidden(constants.FlagEnableServiceMessages)
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
	_ = viper.BindPFlag(constants.FlagEnableServiceMessages, cmdPFlags.Lookup(constants.FlagEnableServiceMessages))
	// if we attempt to check the flags before Execute is called, cobra hasn't parsed anything yet,
	// so we'll get bad values. PersistentPreRunE is a convenient callback for setting up our
	// environment after parsing but before execution.
	cmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
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

		noPrompt := viper.GetBool(constants.ConfigNoPrompt)
		if noPrompt {
			askProvider.DisableInteractive()
		}

		// resolve the output format once, here, rather than leaving each command to work it
		// out for itself; commands (and output.PrintResource / output.PrintArray) then just
		// read the flag and can trust what they get.
		configuredFormat := ""
		if viper.InConfig(strings.ToLower(constants.ConfigOutputFormat)) {
			configuredFormat = viper.GetString(constants.ConfigOutputFormat)
		}
		outputFormat, warning, err := resolveOutputFormat(cmdPFlags, noPrompt, configuredFormat)
		if warning != "" {
			cmd.PrintErrln(warning)
		}
		if err != nil {
			return usage.NewUsageError(err.Error(), cmd)
		}
		// write through Value so the flag isn't marked as Changed; commands such as `task wait`
		// read Changed() to mean "the user explicitly asked for a format"
		_ = cmdPFlags.Lookup(constants.FlagOutputFormat).Value.Set(outputFormat)

		if spaceNameOrId := viper.GetString(constants.ConfigSpace); spaceNameOrId != "" {
			clientFactory.SetSpaceNameOrId(spaceNameOrId)
		}
		return nil
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if versionParameter {
			return versionCommand.RunE(cmd, args)
		} else {
			return cmd.Help()
		}
	}

	return cmd
}

// resolveOutputFormat works out the output format a command should use, in precedence order:
// an explicit --output-format (or legacy --outputFormat) flag, then the OutputFormat config file
// setting, then basic when prompting is disabled, and finally table.
//
// Note the flag carries a non-empty default, so "did the caller ask for a format?" has to be
// answered with Changed() rather than by testing the value for emptiness. configuredFormat is
// the OutputFormat config file setting, or empty if the config file doesn't set one.
//
// An unusable value returns an error, except when it came from the config file, which we can
// only warn about; see below.
func resolveOutputFormat(flags *pflag.FlagSet, noPrompt bool, configuredFormat string) (string, string, error) {
	// the legacy flag is copied onto the new one by value, which doesn't mark it as Changed
	explicit := flags.Changed(constants.FlagOutputFormat) || flags.Changed(constants.FlagOutputFormatLegacy)
	outputFormat, _ := flags.GetString(constants.FlagOutputFormat)

	// this runs for every command, so failing hard on a bad config file value would lock the
	// user out of the whole CLI - `octopus config set OutputFormat table` included. Warn and
	// carry on down the precedence chain instead, so the config is still fixable.
	warning := ""
	if configuredFormat != "" && !constants.IsValidOutputFormat(strings.TrimSpace(configuredFormat)) {
		warning = fmt.Sprintf("Ignoring the %s config setting: %s",
			constants.ConfigOutputFormat, constants.UnsupportedOutputFormatMessage(configuredFormat))
		configuredFormat = ""
	}

	switch {
	case explicit: // take the flag as given
	case configuredFormat != "":
		outputFormat = configuredFormat
	case noPrompt:
		outputFormat = constants.OutputFormatBasic
	default:
		outputFormat = constants.OutputFormatTable
	}

	outputFormat = strings.ToLower(strings.TrimSpace(outputFormat))
	if !constants.IsValidOutputFormat(outputFormat) {
		return "", warning, errors.New(constants.UnsupportedOutputFormatMessage(outputFormat))
	}
	return outputFormat, warning, nil
}
