package deploy

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	cliErrors "github.com/OctopusDeploy/cli/pkg/errors"
	"github.com/OctopusDeploy/cli/pkg/executor"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/spf13/cobra"
	"io"
)

// octopus release deploy <stuff>
// octopus release deploy-tenanted <stuff>

const (

	// NO
	// TODO force?

	// YES; Prompted Variable Only: read about prompted variables (user has to input them during deployment)
	// TODO variable(s)?
	// TODO updateVariables?

	FlagProject = "project"
	FlagChannel = "channel"

	FlagReleaseVersion           = "version"
	FlagAliasReleaseNumberLegacy = "releaseNumber"

	FlagEnvironment         = "environment" // can be specified multiple times; but only once if tenanted
	FlagAliasEnv            = "env"
	FlagAliasDeployToLegacy = "deployTo" // alias for environment

	FlagTenant = "tenant" // can be specified multiple times

	FlagTenantTag            = "tenant-tag" // can be specified multiple times
	FlagAliasTag             = "tag"
	FlagAliasTenantTagLegacy = "tenantTag"

	FlagDeployAt            = "deploy-at"
	FlagAliasWhen           = "when" // alias for deploy-at
	FlagAliasDeployAtLegacy = "deployAt"

	// Don't ask this question in interactive mode; leave it for advanced. TODO review later with team
	FlagMaxQueueTime             = "max-queue-time" // should we have this as --no-deploy-after instead?
	FlagAliasNoDeployAfterLegacy = "noDeployAfter"  // max queue time

	FlagSkip = "skip" // can be specified multiple times

	FlagGuidedFailureMode            = "guided-failure"
	FlagAliasGuidedFailureModeLegacy = "guidedFailure"

	FlagForcePackageDownload            = "force-package-download"
	FlagAliasForcePackageDownloadLegacy = "forcePackageDownload"

	FlagDeploymentTargets           = "targets" // specific machines
	FlagAliasSpecificMachinesLegacy = "specificMachines"

	FlagExcludeDeploymentTargets   = "exclude-targets"
	FlagAliasExcludeMachinesLegacy = "excludeMachines"

	FlagVariable = "variable"

	FlagUpdateVariables            = "update-variables"
	FlagAliasUpdateVariablesLegacy = "updateVariables"
)

// executions API stops here.

// DEPLOYMENT TRACKING (Server Tasks): - this might be a separate `octopus task follow ID1, ID2, ID3`

// DESIGN CHOICE: We are not going to show servertask progress in the CLI. We will need to optionally wait for deployments to complete though

// TODO progress? // OUT?

// TODO deploymentTimeout? (default to 10m)
// TODO cancelOnTimeout? (default to true)

// TODO deploymentCheckSleepCycle?
// TODO waitForDeployment?

type DeployFlags struct {
	Project              *flag.Flag[string]
	Channel              *flag.Flag[string]
	ReleaseVersion       *flag.Flag[string]   // the release to deploy
	Environments         *flag.Flag[[]string] // multiple for untenanted deployment
	Tenants              *flag.Flag[[]string]
	TenantTags           *flag.Flag[[]string]
	DeployAt             *flag.Flag[string]
	MaxQueueTime         *flag.Flag[string]
	Variables            *flag.Flag[[]string]
	UpdateVariables      *flag.Flag[bool]
	ExcludedSteps        *flag.Flag[[]string]
	GuidedFailureMode    *flag.Flag[string] // tri-state: true, false, or "use default". Can we model it with an optional bool?
	ForcePackageDownload *flag.Flag[bool]
	DeploymentTargets    *flag.Flag[[]string]
	ExcludeTargets       *flag.Flag[[]string]
	// TODO what about deployment targets per tenant? How do you specify that on the cmdline? Look at octo
}

func NewDeployFlags() *DeployFlags {
	return &DeployFlags{
		Project:              flag.New[string](FlagProject, false),
		Channel:              flag.New[string](FlagChannel, false),
		ReleaseVersion:       flag.New[string](FlagReleaseVersion, false),
		Environments:         flag.New[[]string](FlagEnvironment, false),
		Tenants:              flag.New[[]string](FlagTenant, false),
		TenantTags:           flag.New[[]string](FlagTenantTag, false),
		MaxQueueTime:         flag.New[string](FlagMaxQueueTime, false),
		DeployAt:             flag.New[string](FlagDeployAt, false),
		Variables:            flag.New[[]string](FlagVariable, false),
		UpdateVariables:      flag.New[bool](FlagUpdateVariables, false),
		ExcludedSteps:        flag.New[[]string](FlagSkip, false),
		GuidedFailureMode:    flag.New[string](FlagGuidedFailureMode, false),
		ForcePackageDownload: flag.New[bool](FlagForcePackageDownload, false),
		DeploymentTargets:    flag.New[[]string](FlagDeploymentTargets, false),
		ExcludeTargets:       flag.New[[]string](FlagExcludeDeploymentTargets, false),
	}
}

func NewCmdDeploy(f factory.Factory) *cobra.Command {
	deployFlags := NewDeployFlags()
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy releases in Octopus Deploy",
		Long:  "Deploy releases in Octopus Deploy.",
		Example: heredoc.Doc(`
			$ octopus release deploy: TODO
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && deployFlags.Project.Value == "" {
				deployFlags.Project.Value = args[0]
			}

			return deployRun(cmd, f, deployFlags)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&deployFlags.Project.Value, deployFlags.Project.Name, "p", "", "Name or ID of the project to deploy the release from")
	flags.StringVarP(&deployFlags.Channel.Value, deployFlags.Channel.Name, "c", "", "Name or ID of the project to deploy the release from")
	flags.StringVarP(&deployFlags.ReleaseVersion.Value, deployFlags.ReleaseVersion.Name, "v", "", "Release version to deploy")
	flags.StringSliceVarP(&deployFlags.Environments.Value, deployFlags.Environments.Name, "e", nil, "Deploy to this environment (can be specified multiple times)")
	flags.StringSliceVarP(&deployFlags.Tenants.Value, deployFlags.Tenants.Name, "", nil, "Deploy to this tenant (can be specified multiple times)")
	flags.StringSliceVarP(&deployFlags.TenantTags.Value, deployFlags.TenantTags.Name, "", nil, "Deploy to tenants matching this tag (can be specified multiple times)")
	flags.StringVarP(&deployFlags.DeployAt.Value, deployFlags.DeployAt.Name, "", "", "Deploy at a later time. Deploy now if omitted. TODO date formats and timezones!")
	flags.StringVarP(&deployFlags.MaxQueueTime.Value, deployFlags.MaxQueueTime.Name, "", "", "Cancel the deployment if it hasn't started within this time period.")
	flags.StringSliceVarP(&deployFlags.Variables.Value, deployFlags.Variables.Name, "r", nil, "Set the value for a prompted variable in the format Label:Value")
	flags.BoolVarP(&deployFlags.UpdateVariables.Value, deployFlags.UpdateVariables.Name, "", false, "Overwrite the release variable snapshot by re-importing variables from the project.")
	flags.StringSliceVarP(&deployFlags.ExcludedSteps.Value, deployFlags.ExcludedSteps.Name, "", nil, "Exclude specific steps from the deployment")
	flags.StringVarP(&deployFlags.GuidedFailureMode.Value, deployFlags.GuidedFailureMode.Name, "", "", "Enable Guided failure mode (yes/no/default)")
	flags.BoolVarP(&deployFlags.ForcePackageDownload.Value, deployFlags.ForcePackageDownload.Name, "", false, "Force re-download of packages")
	flags.StringSliceVarP(&deployFlags.DeploymentTargets.Value, deployFlags.DeploymentTargets.Name, "", nil, "Deploy to this target (can be specified multiple times)")
	flags.StringSliceVarP(&deployFlags.ExcludeTargets.Value, deployFlags.ExcludeTargets.Name, "", nil, "Deploy to targets except for this (can be specified multiple times)")

	flags.SortFlags = false

	// flags aliases for compat with old .NET CLI
	flagAliases := make(map[string][]string, 10)
	util.AddFlagAliasesString(flags, FlagReleaseVersion, flagAliases, FlagAliasReleaseNumberLegacy)
	util.AddFlagAliasesString(flags, FlagEnvironment, flagAliases, FlagAliasDeployToLegacy, FlagAliasEnv)
	util.AddFlagAliasesString(flags, FlagTenantTag, flagAliases, FlagAliasTag, FlagAliasTenantTagLegacy)
	util.AddFlagAliasesString(flags, FlagDeployAt, flagAliases, FlagAliasWhen, FlagAliasDeployAtLegacy)
	util.AddFlagAliasesString(flags, FlagMaxQueueTime, flagAliases, FlagAliasNoDeployAfterLegacy)
	util.AddFlagAliasesString(flags, FlagUpdateVariables, flagAliases, FlagAliasUpdateVariablesLegacy)
	util.AddFlagAliasesString(flags, FlagGuidedFailureMode, flagAliases, FlagAliasGuidedFailureModeLegacy)
	util.AddFlagAliasesBool(flags, FlagForcePackageDownload, flagAliases, FlagAliasForcePackageDownloadLegacy)
	util.AddFlagAliasesBool(flags, FlagDeploymentTargets, flagAliases, FlagAliasSpecificMachinesLegacy)
	util.AddFlagAliasesBool(flags, FlagExcludeDeploymentTargets, flagAliases, FlagAliasExcludeMachinesLegacy)

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		// map alias values
		for k, v := range flagAliases {
			for _, aliasName := range v {
				f := cmd.Flags().Lookup(aliasName)
				r := f.Value.String() // boolean flags get stringified here but it's fast enough and a one-shot so meh
				if r != f.DefValue {
					_ = cmd.Flags().Lookup(k).Value.Set(r)
				}
			}
		}
		return nil
	}
	return cmd
}

func deployRun(cmd *cobra.Command, f factory.Factory, flags *DeployFlags) error {
	outputFormat, err := cmd.Flags().GetString(constants.FlagOutputFormat)
	if err != nil { // should never happen, but fallback if it does
		outputFormat = constants.OutputFormatTable
	}

	octopus, err := f.GetSpacedClient()
	if err != nil {
		return err
	}

	options := &executor.TaskOptionsDeployRelease{
		ProjectName: flags.Project.Value,
	}

	if f.IsPromptEnabled() {
		err = AskQuestions(octopus, cmd.OutOrStdout(), f.Ask, f.Spinner(), options)
		if err != nil {
			return err
		}

		if !constants.IsProgrammaticOutputFormat(outputFormat) {
			// the Q&A process will have modified options;backfill into flags for generation of the automation cmd
			resolvedFlags := NewDeployFlags()
			resolvedFlags.Project.Value = options.ProjectName
			resolvedFlags.Channel.Value = options.ChannelName
			resolvedFlags.ReleaseVersion.Value = options.ReleaseVersion
			resolvedFlags.Environments.Value = options.Environments
			resolvedFlags.Tenants.Value = options.Tenants
			resolvedFlags.TenantTags.Value = options.TenantTags
			resolvedFlags.DeployAt.Value = options.DeployAt
			resolvedFlags.MaxQueueTime.Value = options.MaxQueueTime
			resolvedFlags.ExcludedSteps.Value = options.ExcludedSteps
			resolvedFlags.GuidedFailureMode.Value = options.GuidedFailureMode
			resolvedFlags.ForcePackageDownload.Value = options.ForcePackageDownload
			resolvedFlags.DeploymentTargets.Value = options.DeploymentTargets
			resolvedFlags.ExcludeTargets.Value = options.ExcludeTargets

			autoCmd := flag.GenerateAutomationCmd(constants.ExecutableName+" release deploy",
				resolvedFlags.Project,
				resolvedFlags.Channel,
				resolvedFlags.ReleaseVersion,
				resolvedFlags.Environments,
				resolvedFlags.Tenants,
				resolvedFlags.TenantTags,
				resolvedFlags.DeployAt,
				resolvedFlags.MaxQueueTime,
				resolvedFlags.ExcludedSteps,
				resolvedFlags.GuidedFailureMode,
				resolvedFlags.ForcePackageDownload,
				resolvedFlags.DeploymentTargets,
				resolvedFlags.ExcludeTargets,
			)
			cmd.Printf("\nAutomation Command: %s\n", autoCmd)
		}
	}

	// the executor will raise errors if any required options are missing
	err = executor.ProcessTasks(octopus, f.GetCurrentSpace(), []*executor.Task{
		executor.NewTask(executor.TaskTypeCreateRelease, options),
	})
	if err != nil {
		return err
	}

	return nil
}

func AskQuestions(octopus *octopusApiClient.Client, stdout io.Writer, asker question.Asker, spinner factory.Spinner, options *executor.TaskOptionsDeployRelease) error {
	if octopus == nil {
		return cliErrors.NewArgumentNullOrEmptyError("octopus")
	}
	if asker == nil {
		return cliErrors.NewArgumentNullOrEmptyError("asker")
	}
	if options == nil {
		return cliErrors.NewArgumentNullOrEmptyError("options")
	}
	// Note: we don't get here at all if no-prompt is enabled, so we know we are free to ask questions

	// Note on output: survey prints things; if the option is specified already from the command line,
	// we should emulate that so there is always a line where you can see what the item was when specified on the command line,
	// however if we support a "quiet mode" then we shouldn't emit those

	var err error

	// select project
	var selectedProject *projects.Project
	if options.ProjectName == "" {
		selectedProject, err = util.SelectProject("Select the project to deploy from", octopus, asker, spinner)
		if err != nil {
			return err
		}
	} else { // project name is already provided, fetch the object because it's needed for further questions
		selectedProject, err = util.FindProject(octopus, spinner, options.ProjectName)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "Project %s\n", output.Cyan(selectedProject.Name))
	}

	// QUESTION: Presumably channel here is used to narrow down the search for release versions?
	// if --version is specified on the command line, then we should be able to skip the question?

	// select channel
	var selectedChannel *channels.Channel
	if options.ChannelName == "" {
		selectedChannel, err = util.SelectChannel(octopus, asker, spinner, "Select the channel to deploy from", selectedProject)
		if err != nil {
			return err
		}
	} else {
		selectedChannel, err = util.FindChannel(octopus, spinner, selectedProject, options.ChannelName)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "Channel %s\n", output.Cyan(selectedChannel.Name))
	}
	options.ChannelName = selectedChannel.Name
	if err != nil {
		return err
	}

	// select release

	var selectedRelease *releases.Release
	if options.ReleaseVersion == "" {
		selectedRelease, err = selectRelease(octopus, asker, spinner, "Select the release to deploy", selectedProject, selectedChannel)
		if err != nil {
			return err
		}
	} else {
		selectedRelease, err = findRelease(octopus, spinner, selectedProject, selectedChannel)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "Release %s\n", output.Cyan(selectedRelease.Version))
	}
	options.ReleaseVersion = selectedRelease.Version
	if err != nil {
		return err
	}

	// NOTE: Tenant can be disabled or forced. In these cases we know what to do.

	// The middle case is "allowed, but not forced", in which case we don't know ahead of time what to do WRT tenants,
	// so we'd need to ask the user (presumably though we can check if the project itself is linked to any tenants and only ask then)?
	// there is a ListTenants(projectID) api that we can use. /api/tenants?projectID=

	// 		If tentanted:
	// 		  select (singular) environment
	// 		    select tenants and/or tags (this is just a way of finding which tenants we are going to deploy to)
	// 		else:
	// 		  select environments

	// 		UX problem: How do we find tenants via their tags?

	// when? (timed deployment)

	// select steps to exclude

	// do we want guided failure mode?

	// force package re-download?

	// If tenanted:
	//   foreach tenant:
	//     select deployment target(s)
	// else
	//   select deployment target(s)

	// DONE
	return nil
}

func selectRelease(octopus *octopusApiClient.Client, ask question.Asker, spinner factory.Spinner, questionText string, project *projects.Project, channel *channels.Channel) (*releases.Release, error) {
	foundReleases := make([]*releases.Release, 0)
	spinner.Start()
	recv, _ := projects.GetReleasesForChannel(octopus, project, channel)
	for {
		pageOrError := <-recv
		if pageOrError.Response != nil && len(pageOrError.Response.Items) != 0 {
			foundReleases = append(foundReleases, pageOrError.Response.Items...)
		} else if pageOrError.Error != nil {
			spinner.Stop()
			return nil, pageOrError.Error
		} else { // both nil means channel closed
			break
		}
	}
	spinner.Stop()

	return question.SelectMap(ask, questionText, foundReleases, func(p *releases.Release) string {
		return p.Version
	})
}

func findRelease(octopus *octopusApiClient.Client, spinner factory.Spinner, project *projects.Project, channel *channels.Channel) (*releases.Release, error) {
	spinner.Start()
	//foundChannels, err := octopus.Projects.GetChannels(project) // TODO change this to channel partial name search on server; will require go client update
	spinner.Stop()
	//if err != nil {
	//	return nil, err
	//}
	//for _, c := range foundChannels { // server doesn't support channel search by exact name so we must emulate it
	//	if strings.EqualFold(c.Name, channelName) {
	//		return c, nil
	//	}
	//}
	//return nil, fmt.Errorf("no channel found with name of %s", channelName)
	return nil, nil
}
