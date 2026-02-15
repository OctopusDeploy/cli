package promote

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	cmdDeploy "github.com/OctopusDeploy/cli/pkg/cmd/release/deploy"
	"github.com/OctopusDeploy/cli/pkg/executor"
	"github.com/OctopusDeploy/cli/pkg/util/featuretoggle"

	"github.com/OctopusDeploy/cli/pkg/apiclient"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	cliErrors "github.com/OctopusDeploy/cli/pkg/errors"
	"github.com/OctopusDeploy/cli/pkg/executionscommon"

	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/dashboard"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	env "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	proj "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
)

const (
	FlagSourceEnvironment      = "source-env"
	FlagAliasSourceEnvToLegacy = "from"

	FlagLatestSuccessful            = "latest-successful"
	FlagAliasLatestSuccessfulLegacy = "latestSuccessful"
)

// PromoteFlags embeds DeployFlags to reuse all common flags and adds promote-specific flags
type PromoteFlags struct {
	*cmdDeploy.DeployFlags
	SourceEnvironment *flag.Flag[string]
	LatestSuccessful  *flag.Flag[bool]
}

func NewPromoteFlags() *PromoteFlags {
	return &PromoteFlags{
		DeployFlags:       cmdDeploy.NewDeployFlags(),
		SourceEnvironment: flag.New[string](FlagSourceEnvironment, false),
		LatestSuccessful:  flag.New[bool](FlagLatestSuccessful, false),
	}
}

func NewCmdPromote(f factory.Factory) *cobra.Command {
	promoteFlags := NewPromoteFlags()
	cmd := &cobra.Command{
		Use:   "promote",
		Short: "Promote release",
		Long:  "Promote release in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s release promote  # fully interactive
			$ %[1]s release promote -p MyProject --version 1.0 --source-env Dev -e Staging --environment Production --skip InstallStep --variable VarName:VarValue
			$ %[1]s release promote -p MyProject --version 1.0 --source-env Dev -e Staging --environment Production --force-package-download --guided-failure true --latest-successful
			$ %[1]s release promote -p MyProject --version 1.0 --source-env Dev -e Staging --environment Production --force-package-download --guided-failure true --latest-successful --update-variables 
			$ %[1]s release promote -p MyProject --version 1.0 --source-env Dev -e Staging --environment Production --force-package-download --guided-failure true --latest-successful --update-variables 
		`, constants.ExecutableName),

		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && promoteFlags.Project.Value == "" {
				promoteFlags.Project.Value = args[0]
			}

			return promoteRun(cmd, f, promoteFlags)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&promoteFlags.Project.Value, promoteFlags.Project.Name, "p", "", "Name or ID of the project to promote the release from")
	flags.StringArrayVarP(&promoteFlags.Environments.Value, promoteFlags.Environments.Name, "e", nil, "Promote to this environment (can be specified multiple times)")
	flags.StringArrayVarP(&promoteFlags.Tenants.Value, promoteFlags.Tenants.Name, "", nil, "Promote to this tenant (can be specified multiple times)")
	flags.StringArrayVarP(&promoteFlags.TenantTags.Value, promoteFlags.TenantTags.Name, "", nil, "Promote to tenants matching this tag (can be specified multiple times). Format is 'Tag Set Name/Tag Name', such as 'Regions/South'.")
	flags.StringVarP(&promoteFlags.DeployAt.Value, promoteFlags.DeployAt.Name, "", "", "Deploy at a later time. Deploy now if omitted. TODO date formats and timezones!")
	flags.StringVarP(&promoteFlags.MaxQueueTime.Value, promoteFlags.MaxQueueTime.Name, "", "", "Cancel the deployment if it hasn't started within this time period.")
	flags.StringArrayVarP(&promoteFlags.Variables.Value, promoteFlags.Variables.Name, "v", nil, "Set the value for a prompted variable in the format Label:Value")
	flags.BoolVarP(&promoteFlags.UpdateVariables.Value, promoteFlags.UpdateVariables.Name, "", false, "Overwrite the variable snapshot for the release by re-importing the variables from the project.")
	flags.StringArrayVarP(&promoteFlags.ExcludedSteps.Value, promoteFlags.ExcludedSteps.Name, "", nil, "Exclude specific steps from the deployment")
	flags.StringVarP(&promoteFlags.GuidedFailureMode.Value, promoteFlags.GuidedFailureMode.Name, "", "", "Enable Guided failure mode (true/false/default)")
	flags.BoolVarP(&promoteFlags.ForcePackageDownload.Value, promoteFlags.ForcePackageDownload.Name, "", false, "Force re-download of packages")
	flags.StringArrayVarP(&promoteFlags.DeploymentTargets.Value, promoteFlags.DeploymentTargets.Name, "", nil, "Deploy to this target (can be specified multiple times)")
	flags.StringArrayVarP(&promoteFlags.ExcludeTargets.Value, promoteFlags.ExcludeTargets.Name, "", nil, "Deploy to targets except for this (can be specified multiple times)")
	flags.StringArrayVarP(&promoteFlags.DeploymentFreezeNames.Value, promoteFlags.DeploymentFreezeNames.Name, "", nil, "Override this deployment freeze (can be specified multiple times)")
	flags.StringVarP(&promoteFlags.DeploymentFreezeOverrideReason.Value, promoteFlags.DeploymentFreezeOverrideReason.Name, "", "", "Reason for overriding a deployment freeze")

	// Promote-specific flags
	flags.StringVar(&promoteFlags.SourceEnvironment.Value, promoteFlags.SourceEnvironment.Name, "", "Source environment to promote from")
	flags.BoolVarP(&promoteFlags.LatestSuccessful.Value, promoteFlags.LatestSuccessful.Name, "", false, "Use the latest successful release to promote.")

	flags.SortFlags = false

	// flags aliases for compat with old .NET CLI - reuse deploy's aliases
	flagAliases := make(map[string][]string, 10)
	util.AddFlagAliasesString(flags, FlagSourceEnvironment, flagAliases, FlagAliasSourceEnvToLegacy)
	util.AddFlagAliasesStringSlice(flags, cmdDeploy.FlagEnvironment, flagAliases, cmdDeploy.FlagAliasDeployToLegacy, cmdDeploy.FlagAliasEnv)
	util.AddFlagAliasesStringSlice(flags, cmdDeploy.FlagTenantTag, flagAliases, cmdDeploy.FlagAliasTag, cmdDeploy.FlagAliasTenantTagLegacy)
	util.AddFlagAliasesString(flags, cmdDeploy.FlagDeployAt, flagAliases, cmdDeploy.FlagAliasWhen, cmdDeploy.FlagAliasDeployAtLegacy)
	util.AddFlagAliasesString(flags, cmdDeploy.FlagDeployAtExpiry, flagAliases, cmdDeploy.FlagDeployAtExpire, cmdDeploy.FlagAliasNoDeployAfterLegacy)
	util.AddFlagAliasesString(flags, cmdDeploy.FlagUpdateVariables, flagAliases, cmdDeploy.FlagAliasUpdateVariablesLegacy)
	util.AddFlagAliasesBool(flags, FlagLatestSuccessful, flagAliases, FlagAliasLatestSuccessfulLegacy)
	util.AddFlagAliasesString(flags, cmdDeploy.FlagGuidedFailure, flagAliases, cmdDeploy.FlagAliasGuidedFailureMode, cmdDeploy.FlagAliasGuidedFailureModeLegacy)
	util.AddFlagAliasesBool(flags, cmdDeploy.FlagForcePackageDownload, flagAliases, cmdDeploy.FlagAliasForcePackageDownloadLegacy)
	util.AddFlagAliasesStringSlice(flags, cmdDeploy.FlagDeploymentTarget, flagAliases, cmdDeploy.FlagAliasTarget, cmdDeploy.FlagAliasSpecificMachines)
	util.AddFlagAliasesStringSlice(flags, cmdDeploy.FlagExcludeDeploymentTarget, flagAliases, cmdDeploy.FlagAliasExcludeTarget, cmdDeploy.FlagAliasExcludeMachines)
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		util.ApplyFlagAliases(cmd.Flags(), flagAliases)
		return nil
	}
	return cmd
}

func promoteRun(cmd *cobra.Command, f factory.Factory, flags *PromoteFlags) error {
	outputFormat, err := cmd.Flags().GetString(constants.FlagOutputFormat)
	if err != nil { // should never happen, but fallback if it does
		outputFormat = constants.OutputFormatTable
	}

	octopus, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	parsedVariables, err := executionscommon.ParseVariableStringArray(flags.Variables.Value)
	if err != nil {
		return err
	}

	options := &executor.TaskOptionsPromoteRelease{
		TaskOptionsDeployRelease: executor.TaskOptionsDeployRelease{
			ProjectName:                    flags.Project.Value,
			ReleaseVersion:                 flags.ReleaseVersion.Value,
			Environments:                   flags.Environments.Value,
			Tenants:                        flags.Tenants.Value,
			TenantTags:                     flags.TenantTags.Value,
			ScheduledStartTime:             flags.DeployAt.Value,
			ScheduledExpiryTime:            flags.MaxQueueTime.Value,
			ExcludedSteps:                  flags.ExcludedSteps.Value,
			GuidedFailureMode:              flags.GuidedFailureMode.Value,
			ForcePackageDownload:           flags.ForcePackageDownload.Value,
			DeploymentTargets:              flags.DeploymentTargets.Value,
			ExcludeTargets:                 flags.ExcludeTargets.Value,
			DeploymentFreezeNames:          flags.DeploymentFreezeNames.Value,
			DeploymentFreezeOverrideReason: flags.DeploymentFreezeOverrideReason.Value,
			Variables:                      parsedVariables,
			UpdateVariables:                flags.UpdateVariables.Value,
		},
		SourceEnvironment: flags.SourceEnvironment.Value,
		LatestSuccessful:  flags.LatestSuccessful.Value,
	}

	// special case for FlagForcePackageDownload bool so we can tell if it was set on the cmdline or missing
	if cmd.Flags().Lookup(cmdDeploy.FlagForcePackageDownload).Changed {
		options.ForcePackageDownloadWasSpecified = true
	}

	if f.IsPromptEnabled() {
		now := time.Now
		if cmd.Context() != nil { // allow context to override the definition of 'now' for testing
			if n, ok := cmd.Context().Value(constants.ContextKeyTimeNow).(func() time.Time); ok {
				now = n
			}
		}

		err = AskQuestions(octopus, cmd.OutOrStdout(), f.Ask, f.GetCurrentSpace(), options, now)
		if err != nil {
			return err
		}

		if !constants.IsProgrammaticOutputFormat(outputFormat) {
			// the Q&A process will have modified options;backfill into flags for generation of the automation cmd
			resolvedFlags := NewPromoteFlags()
			resolvedFlags.Project.Value = options.ProjectName
			resolvedFlags.ReleaseVersion.Value = options.ReleaseVersion
			resolvedFlags.SourceEnvironment.Value = options.SourceEnvironment
			resolvedFlags.Environments.Value = options.Environments
			resolvedFlags.Tenants.Value = options.Tenants
			resolvedFlags.TenantTags.Value = options.TenantTags
			resolvedFlags.DeployAt.Value = options.ScheduledStartTime
			resolvedFlags.MaxQueueTime.Value = options.ScheduledExpiryTime
			resolvedFlags.ExcludedSteps.Value = options.ExcludedSteps
			resolvedFlags.GuidedFailureMode.Value = options.GuidedFailureMode
			resolvedFlags.DeploymentTargets.Value = options.DeploymentTargets
			resolvedFlags.ExcludeTargets.Value = options.ExcludeTargets
			resolvedFlags.DeploymentFreezeNames.Value = options.DeploymentFreezeNames
			resolvedFlags.DeploymentFreezeOverrideReason.Value = options.DeploymentFreezeOverrideReason

			didMaskSensitiveVariable := false
			automationVariables := make(map[string]string, len(options.Variables))
			for variableName, variableValue := range options.Variables {
				if util.SliceContainsAny(options.SensitiveVariableNames, func(x string) bool { return strings.EqualFold(x, variableName) }) {
					didMaskSensitiveVariable = true
					automationVariables[variableName] = "*****"
				} else {
					automationVariables[variableName] = variableValue
				}
			}
			resolvedFlags.Variables.Value = executionscommon.ToVariableStringArray(automationVariables)

			// we're deliberately adding --no-prompt to the generated cmdline so ForcePackageDownload=false will be missing,
			// but that's fine
			resolvedFlags.ForcePackageDownload.Value = options.ForcePackageDownload
			resolvedFlags.UpdateVariables.Value = options.UpdateVariables
			resolvedFlags.LatestSuccessful.Value = options.LatestSuccessful

			autoCmd := flag.GenerateAutomationCmd(constants.ExecutableName+" release promote",
				resolvedFlags.Project,
				resolvedFlags.ReleaseVersion,
				resolvedFlags.SourceEnvironment,
				resolvedFlags.Environments, // Use Environments from embedded DeployFlags
				resolvedFlags.Tenants,
				resolvedFlags.TenantTags,
				resolvedFlags.DeployAt,
				resolvedFlags.MaxQueueTime,
				resolvedFlags.ExcludedSteps,
				resolvedFlags.GuidedFailureMode,
				resolvedFlags.ForcePackageDownload,
				resolvedFlags.DeploymentTargets,
				resolvedFlags.ExcludeTargets,
				resolvedFlags.Variables,
				resolvedFlags.UpdateVariables,
				resolvedFlags.LatestSuccessful,
				resolvedFlags.DeploymentFreezeNames,
				resolvedFlags.DeploymentFreezeOverrideReason,
			)
			cmd.Printf("\nAutomation Command: %s\n", autoCmd)

			if didMaskSensitiveVariable {
				cmd.Printf("%s\n", output.Yellow("Warning: Command includes some sensitive variable values which have been replaced with placeholders."))
			}
		}
	} else {
		release, err := getPromotionReleaseVersion(octopus,
			f.GetCurrentSpace(),
			options.ProjectName,
			options.SourceEnvironment,
			options.LatestSuccessful)
		if err != nil {
			return err
		}
		options.ReleaseVersion = release.Version
		options.ReleaseID = release.ID
	}

	// the executor will raise errors if any required options are missing
	err = executor.ProcessTasks(octopus, f.GetCurrentSpace(), []*executor.Task{
		executor.NewTask(executor.TaskTypePromoteRelease, options),
	})
	if err != nil {
		return err
	}

	if options.Response != nil {
		switch outputFormat {
		case constants.OutputFormatBasic:
			for _, task := range options.Response.DeploymentServerTasks {
				cmd.Printf("%s\n", task.ServerTaskID)
			}

		case constants.OutputFormatJson:
			data, err := json.Marshal(options.Response.DeploymentServerTasks)
			if err != nil { // shouldn't happen but fallback in case
				cmd.PrintErrln(err)
			} else {
				_, _ = cmd.OutOrStdout().Write(data)
				cmd.Println()
			}
		default: // table
			cmd.Printf("Successfully started %d deployment(s)\n", len(options.Response.DeploymentServerTasks))
		}

		// output web URL all the time, so long as output format is not JSON or basic
		if err == nil && !constants.IsProgrammaticOutputFormat(outputFormat) {
			releaseID := options.ReleaseID
			if releaseID == "" {
				// we may already have the release ID from AskQuestions. If not, we need to go and look up the release ID to link to it
				// which needs the project ID. Errors here are ignorable; it's not the end of the world if we can't print the web link
				prj, err := selectors.FindProject(octopus, options.ProjectName)
				if err == nil {
					rel, err := releases.GetReleaseInProject(octopus, f.GetCurrentSpace().ID, prj.ID, options.ReleaseVersion)
					if err == nil {
						releaseID = rel.ID
					}
				}
			}

			if releaseID != "" {
				link := output.Bluef("%s/app#/%s/releases/%s", f.GetCurrentHost(), f.GetCurrentSpace().ID, releaseID)
				cmd.Printf("\nView this release on Octopus Deploy: %s\n", link)
			}
		}
	}

	return nil
}

func AskQuestions(octopus *octopusApiClient.Client, stdout io.Writer, asker question.Asker, space *spaces.Space, options *executor.TaskOptionsPromoteRelease, now func() time.Time) error {
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
		selectedProject, err = selectors.Project("Select project", octopus, asker)
		if err != nil {
			return err
		}
	} else { // project name is already provided, fetch the object because it's needed for further questions
		selectedProject, err = selectors.FindProject(octopus, options.ProjectName)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "Project %s\n", output.Cyan(selectedProject.Name))
	}
	options.ProjectName = selectedProject.Name

	isTenanted, err := cmdDeploy.DetermineIsTenanted(selectedProject, asker)
	if err != nil {
		return err
	}

	// For promote, we need source environment first to find the release
	if options.SourceEnvironment == "" {
		selectedSourceEnvironment, err := selectors.EnvironmentSelect(asker, func() ([]*environments.Environment, error) {
			return selectors.GetAllEnvironments(octopus)
		}, "Select source environment")
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "Source Environment %s\n", output.Cyan(selectedSourceEnvironment.Name))
		options.SourceEnvironment = selectedSourceEnvironment.Name
	}

	// Find release from source environment
	var selectedRelease *releases.Release
	var selectedChannel *channels.Channel

	release, err := getPromotionReleaseVersion(octopus, space, selectedProject.Name, options.SourceEnvironment, options.LatestSuccessful)
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "Release %s\n", output.Cyan(release.Version))
	selectedRelease = release
	selectedChannel, err = channels.GetByID(octopus, space.ID, selectedRelease.ChannelID)
	if err != nil {
		return err
	}
	options.ReleaseVersion = release.Version
	options.ReleaseID = release.ID

	indicateMissingPackagesForReleaseFeatureToggleValue, err := featuretoggle.IsToggleEnabled(octopus, "indicate-missing-packages-for-release")
	if indicateMissingPackagesForReleaseFeatureToggleValue {
		proceed := cmdDeploy.PromptMissingPackages(octopus, stdout, asker, selectedRelease)
		if !proceed {
			return errors.New("aborting deployment creation as requested")
		}
	}

	err = cmdDeploy.ValidateDeployment(isTenanted, options.Environments)
	if err != nil {
		return err
	}
	// machine selection later on needs to refer back to the environments.
	// NOTE: this is allowed to remain nil; environments will get looked up later on if needed
	var deploymentEnvironmentIDs []string
	switch selectedChannel.Type {
	case channels.ChannelTypeLifecycle:
		deploymentEnvironmentIDs, err = cmdDeploy.SelectDeploymentEnvironmentsForLifecycleChannel(octopus, stdout, asker, &options.TaskOptionsDeployRelease, selectedRelease, isTenanted)
		if err != nil {
			return err
		}
	case channels.ChannelTypeEphemeral:
		deploymentEnvironmentIDs, err = cmdDeploy.SelectDeploymentEnvironmentsForEphemeralChannel(octopus, stdout, asker, &options.TaskOptionsDeployRelease, selectedRelease)
		if err != nil {
			return err
		}
	default:
		return errors.New("invalid channel type: " + string(selectedChannel.Type))
	}

	variableSet, err := variables.GetVariableSet(octopus, space.ID, selectedRelease.ProjectVariableSetSnapshotID)
	if err != nil {
		return err
	}

	if len(deploymentEnvironmentIDs) == 0 { // if the Q&A process earlier hasn't loaded environments already, we need to load them now
		switch selectedChannel.Type {
		case channels.ChannelTypeLifecycle:
			selectedEnvironments, err := executionscommon.FindEnvironments(octopus, options.Environments)
			if err != nil {
				return err
			}

			deploymentEnvironmentIDs = util.SliceTransform(selectedEnvironments, func(env *environments.Environment) string { return env.ID })
		case channels.ChannelTypeEphemeral:
			deploymentEnvironmentIDs, err = cmdDeploy.FindEphemeralEnvironmentIDs(octopus, space, options.Environments)

			if err != nil {
				return err
			}
		}
	}

	var deploymentPreviewRequests []deployments.DeploymentPreviewRequest
	for _, environmentId := range deploymentEnvironmentIDs {
		preview := deployments.DeploymentPreviewRequest{
			EnvironmentId: environmentId,
			// We ignore the TenantId here as we're just using the deployments previews for prompted variables.
			// Tenant variables do not support prompted variables
			TenantId: "",
		}
		deploymentPreviewRequests = append(deploymentPreviewRequests, preview)
	}

	options.Variables, err = cmdDeploy.AskDeploymentPreviewVariables(octopus, options.Variables, asker, space.ID, selectedRelease.ID, deploymentPreviewRequests)
	if err != nil {
		return err
	}
	// provide list of sensitive variables to the output phase so it doesn't have to go to the server for the variableSet a second time
	if variableSet.Variables != nil {
		sv := util.SliceFilter(variableSet.Variables, func(v *variables.Variable) bool { return v.IsSensitive || v.Type == "Sensitive" })
		options.SensitiveVariableNames = util.SliceTransform(sv, func(v *variables.Variable) string { return v.Name })
	}

	cmdDeploy.PrintAdvancedSummary(stdout, &options.TaskOptionsDeployRelease)

	isDeployAtSpecified := options.ScheduledStartTime != ""
	isExcludedStepsSpecified := len(options.ExcludedSteps) > 0
	isGuidedFailureModeSpecified := options.GuidedFailureMode != ""
	isForcePackageDownloadSpecified := options.ForcePackageDownloadWasSpecified
	isDeploymentTargetsSpecified := len(options.DeploymentTargets) > 0 || len(options.ExcludeTargets) > 0

	allAdvancedOptionsSpecified := isDeployAtSpecified && isExcludedStepsSpecified && isGuidedFailureModeSpecified && isForcePackageDownloadSpecified && isDeploymentTargetsSpecified

	shouldAskAdvancedQuestions := false
	if !allAdvancedOptionsSpecified {
		var changeOptionsAnswer string
		err = asker(&survey.Select{
			Message: "Change additional options?",
			Options: []string{"Proceed to deploy", "Change"},
		}, &changeOptionsAnswer)
		if err != nil {
			return err
		}
		if changeOptionsAnswer == "Change" {
			shouldAskAdvancedQuestions = true
		} else {
			shouldAskAdvancedQuestions = false
		}
	}

	if shouldAskAdvancedQuestions {
		if !isDeployAtSpecified {
			referenceNow := now()
			maxSchedStartTime := referenceNow.Add(30 * 24 * time.Hour) // octopus server won't let you schedule things more than 30d in the future

			var answer surveyext.DatePickerAnswer
			err = asker(&surveyext.DatePicker{
				Message:         "Scheduled start time",
				Help:            "Enter the date and time that this deployment should start. A value less than 1 minute in the future means 'now'",
				Default:         referenceNow,
				Min:             referenceNow,
				Max:             maxSchedStartTime,
				OverrideNow:     referenceNow,
				AnswerFormatter: executionscommon.ScheduledStartTimeAnswerFormatter,
			}, &answer)
			if err != nil {
				return err
			}
			scheduledStartTime := answer.Time
			// if they enter a time within 1 minute, assume 'now', else we need to pick it up.
			// note: the server has some code in it which attempts to detect past
			if scheduledStartTime.After(referenceNow.Add(1 * time.Minute)) {
				options.ScheduledStartTime = scheduledStartTime.Format(time.RFC3339)

				// only ask for an expiry if they didn't pick "now"
				startPlusFiveMin := scheduledStartTime.Add(5 * time.Minute)
				err = asker(&surveyext.DatePicker{
					Message:     "Scheduled expiry time",
					Help:        "At the start time, the deployment will be queued. If it does not begin before 'expiry' time, it will be cancelled. Minimum of 5 minutes after start time",
					Default:     startPlusFiveMin,
					Min:         startPlusFiveMin,
					Max:         maxSchedStartTime.Add(24 * time.Hour), // the octopus server doesn't enforce any upper bound for schedule expiry, so we make a minor judgement call and pick 1d extra here.
					OverrideNow: referenceNow,
				}, &answer)
				if err != nil {
					return err
				}
				options.ScheduledExpiryTime = answer.Time.Format(time.RFC3339)
			}
		}

		if !isExcludedStepsSpecified {
			// select steps to exclude
			deploymentProcess, err := deployments.GetDeploymentProcessByID(octopus, space.ID, selectedRelease.ProjectDeploymentProcessSnapshotID)
			if err != nil {
				return err
			}
			options.ExcludedSteps, err = executionscommon.AskExcludedSteps(asker, deploymentProcess.Steps)
			if err != nil {
				return err
			}
		}

		if !isGuidedFailureModeSpecified { // if they deliberately specified false, don't ask them
			options.GuidedFailureMode, err = executionscommon.AskGuidedFailureMode(asker)
			if err != nil {
				return err
			}
		}

		if !isForcePackageDownloadSpecified { // if they deliberately specified false, don't ask them
			options.ForcePackageDownload, err = executionscommon.AskPackageDownload(asker)
			if err != nil {
				return err
			}
		}

		if !isDeploymentTargetsSpecified {
			if len(deploymentEnvironmentIDs) == 0 { // if the Q&A process earlier hasn't loaded environments already, we need to load them now
				selectedEnvironments, err := executionscommon.FindEnvironments(octopus, options.Environments)
				if err != nil {
					return err
				}
				deploymentEnvironmentIDs = util.SliceTransform(selectedEnvironments, func(env *environments.Environment) string { return env.ID })
			}
			options.DeploymentTargets, err = cmdDeploy.AskDeploymentTargets(octopus, asker, space.ID, selectedRelease.ID, deploymentEnvironmentIDs)
			if err != nil {
				return err
			}
		}
	}
	// DONE
	return nil
}

func getPromotionReleaseVersion(octopus *octopusApiClient.Client,
	space *spaces.Space,
	projectName string,
	sourceEnvironmentName string,
	latestSuccessful bool) (*releases.Release, error) {

	projectResource, err := proj.GetByName(octopus, space.Resource.ID, projectName)
	if err != nil {
		return nil, err
	}
	environmentResource, err := env.Get(octopus, space.Resource.ID, env.EnvironmentsQuery{
		Name: sourceEnvironmentName,
	})
	if err != nil {
		return nil, err
	}
	dashboardItem, err := dashboard.GetDynamicDashboardItem(octopus, space.Resource.ID, dashboard.DashboardDynamicQuery{
		Environments:    []string{environmentResource.Items[0].ID},
		Projects:        []string{projectResource.ID},
		IncludePrevious: latestSuccessful,
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(dashboardItem.Items, func(i, j int) bool {
		return dashboardItem.Items[i].ReleaseVersion < dashboardItem.Items[j].ReleaseVersion
	})
	if len(dashboardItem.Items) == 0 {
		return nil, errors.New("no release found in dashboard")
	}
	version := dashboardItem.Items[0].ReleaseVersion
	if latestSuccessful {
		for _, item := range dashboardItem.Items {
			if item.State == "Success" {
				version = item.ReleaseVersion
				break
			}
		}
	}
	releaseResource, err := releases.GetReleaseInProject(octopus, space.Resource.ID, projectResource.ID, version)
	if err != nil {
		return nil, err
	}
	return releaseResource, nil
}
