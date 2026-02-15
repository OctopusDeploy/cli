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
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/ephemeralenvironments"
	"golang.org/x/exp/maps"

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
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
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

	isTenanted, err := determineIsTenanted(selectedProject, asker)
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
		proceed := promptMissingPackages(octopus, stdout, asker, selectedRelease)
		if !proceed {
			return errors.New("aborting deployment creation as requested")
		}
	}

	err = validateDeployment(isTenanted, options.Environments)
	if err != nil {
		return err
	}
	// machine selection later on needs to refer back to the environments.
	// NOTE: this is allowed to remain nil; environments will get looked up later on if needed
	var deploymentEnvironmentIDs []string
	switch selectedChannel.Type {
	case channels.ChannelTypeLifecycle:
		deploymentEnvironmentIDs, err = selectDeploymentEnvironmentsForLifecycleChannel(octopus, stdout, asker, options, selectedRelease, isTenanted)
		if err != nil {
			return err
		}
	case channels.ChannelTypeEphemeral:
		deploymentEnvironmentIDs, err = selectDeploymentEnvironmentsForEphemeralChannel(octopus, stdout, asker, options, selectedRelease)
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
			deploymentEnvironmentIDs, err = findEphemeralEnvironmentIDs(octopus, space, options.Environments)

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

	options.Variables, err = askDeploymentPreviewVariables(octopus, options.Variables, asker, space.ID, selectedRelease.ID, deploymentPreviewRequests)
	if err != nil {
		return err
	}
	// provide list of sensitive variables to the output phase so it doesn't have to go to the server for the variableSet a second time
	if variableSet.Variables != nil {
		sv := util.SliceFilter(variableSet.Variables, func(v *variables.Variable) bool { return v.IsSensitive || v.Type == "Sensitive" })
		options.SensitiveVariableNames = util.SliceTransform(sv, func(v *variables.Variable) string { return v.Name })
	}

	PrintAdvancedSummary(stdout, options)

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
			options.DeploymentTargets, err = askDeploymentTargets(octopus, asker, space.ID, selectedRelease.ID, deploymentEnvironmentIDs)
			if err != nil {
				return err
			}
		}
	}
	// DONE
	return nil
}

func findEphemeralEnvironmentIDs(octopus *octopusApiClient.Client, space *spaces.Space, environments []string) ([]string, error) {
	allEphemeralEnvironments, err := ephemeralenvironments.GetAll(octopus, space.ID)
	if err != nil {
		return nil, err
	}

	if allEphemeralEnvironments == nil || allEphemeralEnvironments.TotalResults == 0 {
		return nil, errors.New("no ephemeral environments exist to deploy to")
	}

	var selectedEnvironments []string
	if len(environments) == 0 {
		return nil, nil
	}

	envMap := make(map[string]*ephemeralenvironments.EphemeralEnvironment, len(allEphemeralEnvironments.Items)*2)
	for _, ephemeralEnv := range allEphemeralEnvironments.Items {
		envMap[strings.ToLower(ephemeralEnv.ID)] = ephemeralEnv
		envMap[strings.ToLower(ephemeralEnv.Name)] = ephemeralEnv
	}

	for _, envIdentifier := range environments {
		ephemeralEnv, found := envMap[strings.ToLower(envIdentifier)]
		if !found {
			return nil, fmt.Errorf("environment '%s' not found in ephemeral environments", envIdentifier)
		}
		selectedEnvironments = append(selectedEnvironments, ephemeralEnv.ID)
	}

	return selectedEnvironments, nil
}

func selectDeploymentEnvironmentsForEphemeralChannel(octopus *octopusApiClient.Client, stdout io.Writer, asker question.Asker, options *executor.TaskOptionsPromoteRelease, selectedRelease *releases.Release) ([]string, error) {
	var deploymentEnvironmentIds []string
	var selectedEnvironments []*ephemeralenvironments.EphemeralEnvironment

	if len(options.Environments) == 0 {
		allEphemeralEnvironments, err := ephemeralenvironments.GetAll(octopus, selectedRelease.SpaceID)
		if err != nil {
			return nil, err
		}
		if allEphemeralEnvironments == nil || allEphemeralEnvironments.TotalResults == 0 {
			return nil, errors.New("no ephemeral environments exist to deploy to")
		}

		deploymentEnvironmentTemplate, err := releases.GetReleaseDeploymentTemplate(octopus, selectedRelease.SpaceID, selectedRelease.ID)
		if err != nil {
			return nil, err
		}

		allowedEnvironmentIds := map[string]bool{}
		for _, p := range deploymentEnvironmentTemplate.PromoteTo {
			allowedEnvironmentIds[p.ID] = true
		}

		var availableEnvironments []*ephemeralenvironments.EphemeralEnvironment
		for _, env := range allEphemeralEnvironments.Items {
			if _, ok := allowedEnvironmentIds[env.ID]; ok {
				availableEnvironments = append(availableEnvironments, env)
			}
		}

		if len(availableEnvironments) > 0 {
			selectedEnvironments, err = selectEphemeralDeploymentEnvironments(asker, availableEnvironments)
			if err != nil {
				return nil, err
			}
			deploymentEnvironmentIds = util.SliceTransform(selectedEnvironments, func(env *ephemeralenvironments.EphemeralEnvironment) string { return env.ID })
			options.Environments = util.SliceTransform(selectedEnvironments, func(env *ephemeralenvironments.EphemeralEnvironment) string { return env.Name })
		}
	} else {
		_, _ = fmt.Fprintf(stdout, "Environments %s\n", output.Cyan(strings.Join(options.Environments, ",")))
	}

	return deploymentEnvironmentIds, nil
}

func selectDeploymentEnvironmentsForLifecycleChannel(octopus *octopusApiClient.Client, stdout io.Writer, asker question.Asker, options *executor.TaskOptionsPromoteRelease, selectedRelease *releases.Release, isTenanted bool) ([]string, error) {
	var deploymentEnvironmentIds []string
	var selectedEnvironments []*environments.Environment
	var err error

	if isTenanted {
		var selectedEnvironment *environments.Environment
		if len(options.Environments) == 0 {
			deployableEnvironmentIDs, nextEnvironmentID, err := FindDeployableEnvironmentIDs(octopus, selectedRelease)
			if err != nil {
				return nil, err
			}
			selectedEnvironment, err = selectDeploymentEnvironment(asker, octopus, deployableEnvironmentIDs, nextEnvironmentID)
			if err != nil {
				return nil, err
			}
			options.Environments = []string{selectedEnvironment.Name} // executions api allows env names, so let's use these instead so they look nice in generated automationcmd
		} else {
			selectedEnvironment, err = selectors.FindEnvironment(octopus, options.Environments[0])
			if err != nil {
				return nil, err
			}
			_, _ = fmt.Fprintf(stdout, "Environment %s\n", output.Cyan(selectedEnvironment.Name))
		}
		selectedEnvironments = []*environments.Environment{selectedEnvironment}
		deploymentEnvironmentIds = util.SliceTransform(selectedEnvironments, func(env *environments.Environment) string { return env.ID })

		// ask for tenants and/or tags unless some were specified on the command line
		if len(options.Tenants) == 0 && len(options.TenantTags) == 0 {
			options.Tenants, options.TenantTags, err = executionscommon.AskTenantsAndTags(asker, octopus, selectedRelease.ProjectID, selectedEnvironments, true)
			if len(options.Tenants) == 0 && len(options.TenantTags) == 0 {
				return nil, errors.New("no tenants or tags available; cannot deploy")
			}
			if err != nil {
				return nil, err
			}
		} else {
			if len(options.Tenants) > 0 {
				_, _ = fmt.Fprintf(stdout, "Tenants %s\n", output.Cyan(strings.Join(options.Tenants, ",")))
			}
			if len(options.TenantTags) > 0 {
				_, _ = fmt.Fprintf(stdout, "Tenant Tags %s\n", output.Cyan(strings.Join(options.TenantTags, ",")))
			}
		}
	} else {
		if len(options.Environments) == 0 {
			deployableEnvironmentIDs, nextEnvironmentID, err := FindDeployableEnvironmentIDs(octopus, selectedRelease)
			if err != nil {
				return nil, err
			}
			selectedEnvironments, err = selectDeploymentEnvironments(asker, octopus, deployableEnvironmentIDs, nextEnvironmentID)
			if err != nil {
				return nil, err
			}
			deploymentEnvironmentIds = util.SliceTransform(selectedEnvironments, func(env *environments.Environment) string { return env.ID })
			options.Environments = util.SliceTransform(selectedEnvironments, func(env *environments.Environment) string { return env.Name })
		} else {
			if len(options.Environments) > 0 {
				_, _ = fmt.Fprintf(stdout, "Environments %s\n", output.Cyan(strings.Join(options.Environments, ",")))
			}
		}
	}
	return deploymentEnvironmentIds, nil
}

func validateDeployment(isTenanted bool, environments []string) error {
	if isTenanted && len(environments) > 1 {
		return fmt.Errorf("tenanted deployments can only specify one environment")
	}

	return nil
}

func askDeploymentTargets(octopus *octopusApiClient.Client, asker question.Asker, spaceID string, releaseID string, deploymentEnvironmentIDs []string) ([]string, error) {
	var results []string

	// this is what the portal does. Can we do it better? I don't know
	for _, envID := range deploymentEnvironmentIDs {
		preview, err := deployments.GetReleaseDeploymentPreview(octopus, spaceID, releaseID, envID, true)
		if err != nil {
			return nil, err
		}
		for _, step := range preview.StepsToExecute {
			for _, m := range step.MachineNames {
				if !util.SliceContains(results, m) {
					results = append(results, m)
				}
			}
		}
	}

	// if there are no machines, then either
	// a) everything is server based
	// b) machines will be provisioned dynamically
	// c) or the deployment will fail.
	// In all of the above cases, we can't do anything about it so the correct course of action is just skip the question
	if len(results) > 0 {
		var selectedDeploymentTargetNames []string
		err := asker(&survey.MultiSelect{
			Message: "Deployment targets (If none selected, deploy to all)",
			Options: results,
		}, &selectedDeploymentTargetNames)
		if err != nil {
			return nil, err
		}

		return selectedDeploymentTargetNames, nil
	}
	return nil, nil
}

func askDeploymentPreviewVariables(octopus *octopusApiClient.Client, variablesFromCmd map[string]string, asker question.Asker, spaceID string, releaseID string, deploymentPreviewsReqests []deployments.DeploymentPreviewRequest) (map[string]string, error) {
	previews, err := deployments.GetReleaseDeploymentPreviews(octopus, spaceID, releaseID, deploymentPreviewsReqests, true)
	if err != nil {
		return nil, err
	}

	flattenedValues := make(map[string]string)
	flattenedControls := make(map[string]*deployments.Control)
	for _, preview := range previews {
		for _, element := range preview.Form.Elements {
			flattenedControls[element.Name] = element.Control
		}
		for key, value := range preview.Form.Values {
			flattenedValues[key] = value
		}
	}

	result := make(map[string]string)
	lcaseVarsFromCmd := make(map[string]string, len(variablesFromCmd))
	for k, v := range variablesFromCmd {
		lcaseVarsFromCmd[strings.ToLower(k)] = v
	}

	keys := maps.Keys(flattenedControls)
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] > keys[j]
	})

	for _, key := range keys {
		control := flattenedControls[key]
		valueFromCmd, foundValueOnCommandLine := lcaseVarsFromCmd[strings.ToLower(control.Name)]
		if foundValueOnCommandLine {
			// implicitly fixes up variable casing
			result[control.Name] = valueFromCmd
		}
		if control.Required == true && !foundValueOnCommandLine {

			defaultValue := flattenedValues[key]
			isSensitive := control.DisplaySettings.ControlType == "Sensitive"
			promptMessage := control.Name

			if control.Description != "" {
				promptMessage = fmt.Sprintf("%s (%s)", promptMessage, control.Description) // we'd like to dim the description, but survey overrides this, so we can't
			}

			responseString, err := executionscommon.AskVariableSpecificPrompt(asker, promptMessage, control.Type, defaultValue, control.Required, isSensitive, control.DisplaySettings)
			if err != nil {
				return nil, err
			}
			result[control.Name] = responseString
		}
	}

	return result, nil
}

func promptMissingPackages(octopus *octopusApiClient.Client, stdout io.Writer, asker question.Asker, release *releases.Release) bool {
	missingPackages, err := releases.GetMissingPackages(octopus, release)
	if err != nil {
		// We don't want to prevent deployments from going through because of this check
		_, _ = fmt.Fprintf(stdout, "Unable to determine if there are missing packages for this release - %v\n", err)
		return true
	}

	if len(missingPackages) == 0 {
		return true
	}

	_, _ = fmt.Fprintf(stdout, "Warning: The following packages are missing from the built-in feed for this release:\n")
	for _, p := range missingPackages {
		_, _ = fmt.Fprintf(stdout, " - %s (Version: %s)\n", p.ID, p.Version)
	}
	_, _ = fmt.Fprintln(stdout, "\nThis might cause the deployment to fail.")

	prompt := &survey.Confirm{
		Message: "Do you want to continue?",
		Default: false,
	}

	var answer bool
	if err := asker(prompt, &answer); err != nil {
		return answer
	}

	return answer
}

// FindDeployableEnvironmentIDs returns an array of environment IDs that we can deploy to,
// the preferred 'next' environment, and an error
func FindDeployableEnvironmentIDs(octopus *octopusApiClient.Client, release *releases.Release) ([]string, string, error) {
	var result []string
	// to determine the list of viable environments we need to hit /api/projects/{ID}/progression.
	releaseProgression, err := octopus.Deployments.GetProgression(release)
	if err != nil {
		return nil, "", err
	}
	for _, phase := range releaseProgression.Phases {
		if phase.Progress == releases.PhaseProgressPending {
			continue // we can't deploy to this phase yet
		}
		for _, id := range phase.AutomaticDeploymentTargets {
			if !util.SliceContains(result, id) {
				result = append(result, id)
			}
		}
		for _, id := range phase.OptionalDeploymentTargets {
			if !util.SliceContains(result, id) {
				result = append(result, id)
			}
		}
	}
	nextDeployEnvID := ""
	if len(releaseProgression.NextDeployments) > 0 {
		nextDeployEnvID = releaseProgression.NextDeployments[0]
	}

	return result, nextDeployEnvID, nil
}

func loadEnvironmentsForDeploy(octopus *octopusApiClient.Client, deployableEnvironmentIDs []string, nextDeployEnvironmentID string) ([]*environments.Environment, string, error) {
	envResources, err := octopus.Environments.Get(environments.EnvironmentsQuery{IDs: deployableEnvironmentIDs})
	if err != nil {
		return nil, "", err
	}
	allEnvs, err := envResources.GetAllPages(octopus.Environments.GetClient())
	if err != nil {
		return nil, "", err
	}

	// match the next deploy environment
	nextDeployEnvironmentName := ""
	for _, e := range allEnvs {
		if e.ID == nextDeployEnvironmentID {
			nextDeployEnvironmentName = e.Name
			break
		}
	}
	return allEnvs, nextDeployEnvironmentName, nil
}

func selectDeploymentEnvironment(asker question.Asker, octopus *octopusApiClient.Client, deployableEnvironmentIDs []string, nextDeployEnvironmentID string) (*environments.Environment, error) {
	allEnvs, nextDeployEnvironmentName, err := loadEnvironmentsForDeploy(octopus, deployableEnvironmentIDs, nextDeployEnvironmentID)
	if err != nil {
		return nil, err
	}

	optionMap, options := question.MakeItemMapAndOptions(allEnvs, func(e *environments.Environment) string { return e.Name })
	var selectedKey string
	err = asker(&survey.Select{
		Message: "Select target environment",
		Options: options,
		Default: nextDeployEnvironmentName,
	}, &selectedKey)
	if err != nil {
		return nil, err
	}
	selectedValue, ok := optionMap[selectedKey]
	if !ok {
		return nil, fmt.Errorf("selectDeploymentEnvironment did not get valid answer (selectedKey=%s)", selectedKey)
	}
	return selectedValue, nil
}

func selectEphemeralDeploymentEnvironments(asker question.Asker, deployableEnvironments []*ephemeralenvironments.EphemeralEnvironment) ([]*ephemeralenvironments.EphemeralEnvironment, error) {
	var err error
	optionMap, options := question.MakeItemMapAndOptions(deployableEnvironments, func(e *ephemeralenvironments.EphemeralEnvironment) string { return e.Name })
	var selectedKeys []string
	err = asker(&survey.MultiSelect{
		Message: "Select target environment(s)",
		Options: options,
		Default: nil,
	}, &selectedKeys, survey.WithValidator(survey.Required))

	if err != nil {
		return nil, err
	}
	var selectedValues []*ephemeralenvironments.EphemeralEnvironment
	for _, k := range selectedKeys {
		if value, ok := optionMap[k]; ok {
			selectedValues = append(selectedValues, value)
		} // if we were to somehow get invalid answers, ignore them
	}
	return selectedValues, nil
}

func selectDeploymentEnvironments(asker question.Asker, octopus *octopusApiClient.Client, deployableEnvironmentIDs []string, nextDeployEnvironmentID string) ([]*environments.Environment, error) {
	allEnvs, nextDeployEnvironmentName, err := loadEnvironmentsForDeploy(octopus, deployableEnvironmentIDs, nextDeployEnvironmentID)
	if err != nil {
		return nil, err
	}

	optionMap, options := question.MakeItemMapAndOptions(allEnvs, func(e *environments.Environment) string { return e.Name })
	var selectedKeys []string
	err = asker(&survey.MultiSelect{
		Message: "Select target environment(s)",
		Options: options,
		Default: []string{nextDeployEnvironmentName},
	}, &selectedKeys, survey.WithValidator(survey.Required))

	if err != nil {
		return nil, err
	}
	var selectedValues []*environments.Environment
	for _, k := range selectedKeys {
		if value, ok := optionMap[k]; ok {
			selectedValues = append(selectedValues, value)
		} // if we were to somehow get invalid answers, ignore them
	}
	return selectedValues, nil
}

func PrintAdvancedSummary(stdout io.Writer, options *executor.TaskOptionsPromoteRelease) {
	deployAtStr := "Now"
	if options.ScheduledStartTime != "" {
		deployAtStr = options.ScheduledStartTime // we assume the server is going to understand this
	}
	skipStepsStr := "None"
	if len(options.ExcludedSteps) > 0 {
		skipStepsStr = strings.Join(options.ExcludedSteps, ",")
	}

	gfmStr := executionscommon.LookupGuidedFailureModeString(options.GuidedFailureMode)

	pkgDownloadStr := executionscommon.LookupPackageDownloadString(!options.ForcePackageDownload)

	depTargetsStr := "All included"
	if len(options.DeploymentTargets) != 0 || len(options.ExcludeTargets) != 0 {
		sb := strings.Builder{}
		if len(options.DeploymentTargets) > 0 {
			sb.WriteString("Include ")
			for idx, name := range options.DeploymentTargets {
				if idx > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(name)
			}
		}
		if len(options.ExcludeTargets) > 0 {
			if sb.Len() > 0 {
				sb.WriteString("; ")
			}

			sb.WriteString("Exclude ")
			for idx, name := range options.ExcludeTargets {
				if idx > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(name)
			}
		}
		depTargetsStr = sb.String()
	}

	_, _ = fmt.Fprintf(stdout, output.FormatDoc(heredoc.Doc(`
		bold(Additional Options):
		  Deploy Time: cyan(%s)
		  Skipped Steps: cyan(%s)
		  Guided Failure Mode: cyan(%s)
		  Package Download: cyan(%s)
		  Deployment Targets: cyan(%s)
	`)), deployAtStr, skipStepsStr, gfmStr, pkgDownloadStr, depTargetsStr)
}

// determineIsTenanted returns true if we are going to do a tenanted deployment, false if untenanted
// NOTE: Tenant can be disabled or forced. In these cases we know what to do.
// The middle case is "allowed, but not forced", in which case we don't know ahead of time what to do WRT tenants,
// so we'd need to ask the user. This is not great UX, but the intent of the 'middle ground' tenant state
// is to allow for graceful migrations of older projects, and we don't expect it to happen very often.
// We COULD do a little bit of a shortcut; if tenant is 'allowed but not required' but the project has no
// linked tenants, then it can't be tenanted, but is this worth the extra complexity? Decision: no
func determineIsTenanted(project *projects.Project, ask question.Asker) (bool, error) {
	switch project.TenantedDeploymentMode {
	case core.TenantedDeploymentModeUntenanted:
		return false, nil
	case core.TenantedDeploymentModeTenanted:
		return true, nil
	case core.TenantedDeploymentModeTenantedOrUntenanted:
		return question.SelectMap(ask, "Select Tenanted or Untenanted deployment", []bool{true, false}, func(b bool) string {
			if b {
				return "Tenanted" // should be the default; they probably want tenanted
			} else {
				return "Untenanted"
			}
		})

	default: // should not get here
		return false, fmt.Errorf("unhandled tenanted deployment mode %s", project.TenantedDeploymentMode)
	}
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
	var version string
	if latestSuccessful {
		for _, item := range dashboardItem.Items {
			if item.State == "Success" {
				version = item.ReleaseVersion
				break
			}
		}
	} else {
		version = dashboardItem.Items[0].ReleaseVersion
	}
	releaseResource, err := releases.GetReleaseInProject(octopus, space.Resource.ID, projectResource.ID, version)
	if err != nil {
		return nil, err
	}
	return releaseResource, nil
}
