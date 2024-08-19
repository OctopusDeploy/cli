package root

import (
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	accountCmd "github.com/OctopusDeploy/cli/pkg/cmd/account"
	buildInfoCmd "github.com/OctopusDeploy/cli/pkg/cmd/buildinformation"
	channelCmd "github.com/OctopusDeploy/cli/pkg/cmd/channel"
	configCmd "github.com/OctopusDeploy/cli/pkg/cmd/config"
	environmentCmd "github.com/OctopusDeploy/cli/pkg/cmd/environment"
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

	flags := cmd.Flags()
	var versionParameter bool
	flags.BoolVarP(&versionParameter, "version", "v", false, "Prints version information")
	versionCommand := version.NewCmdVersion(f)

	// ----- Child Commands -----

	cmd.AddCommand(versionCommand)

	// infrastructure
	cmd.AddCommand(accountCmd.NewCmdAccount(f))
	cmd.AddCommand(environmentCmd.NewCmdEnvironment(f))
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
			if v, _ := cmdPFlags.GetString(constants.FlagOutputFormat); v == "" {
				cmdPFlags.Set(constants.FlagOutputFormat, constants.OutputFormatBasic)
			}
		}

		if spaceNameOrId := viper.GetString(constants.ConfigSpace); spaceNameOrId != "" {
			clientFactory.SetSpaceNameOrId(spaceNameOrId)
		}
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if versionParameter {
			versionCommand.RunE(cmd, args)
		}

		return nil
	}

	return cmd
}
