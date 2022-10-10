package deploy

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	cliErrors "github.com/OctopusDeploy/cli/pkg/errors"
	"github.com/OctopusDeploy/cli/pkg/executionscommon"
	"github.com/OctopusDeploy/cli/pkg/executor"
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
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
	"io"
	"strings"
	"time"
)

const (
	FlagProject = "project"

	FlagReleaseVersion           = "version"
	FlagAliasReleaseNumberLegacy = "releaseNumber"

	FlagEnvironment         = "environment" // can be specified multiple times; but only once if tenanted
	FlagAliasEnv            = "env"
	FlagAliasDeployToLegacy = "deployTo" // alias for environment

	FlagTenant = "tenant" // can be specified multiple times

	FlagTenantTag            = "tenant-tag" // can be specified multiple times
	FlagAliasTag             = "tag"
	FlagAliasTenantTagLegacy = "tenantTag"

	FlagDeployAt            = "deploy-at" // if this is less than 1 min in the future, go now
	FlagAliasWhen           = "when"      // alias for deploy-at
	FlagAliasDeployAtLegacy = "deployAt"

	FlagDeployAtExpiry           = "deploy-at-expiry"
	FlagDeployAtExpire           = "deploy-at-expire"
	FlagAliasNoDeployAfterLegacy = "noDeployAfter"

	FlagSkip = "skip" // can be specified multiple times

	FlagGuidedFailure                = "guided-failure"
	FlagAliasGuidedFailureMode       = "guided-failure-mode"
	FlagAliasGuidedFailureModeLegacy = "guidedFailure"

	FlagForcePackageDownload            = "force-package-download"
	FlagAliasForcePackageDownloadLegacy = "forcePackageDownload"

	FlagDeploymentTarget      = "deployment-target"
	FlagAliasTarget           = "target"           // alias for deployment-target
	FlagAliasSpecificMachines = "specificMachines" // octo wants a comma separated list. We prefer specifying --target multiple times, but CSV also works because pflag does it for free

	FlagExcludeDeploymentTarget = "exclude-deployment-target"
	FlagAliasExcludeTarget      = "exclude-target"
	FlagAliasExcludeMachines    = "excludeMachines" // octo wants a comma separated list. We prefer specifying --exclude-target multiple times, but CSV also works because pflag does it for free

	FlagVariable = "variable"

	FlagUpdateVariables            = "update-variables"
	FlagAliasUpdateVariablesLegacy = "updateVariables"
)

// executions API stops here.

// DEPLOYMENT TRACKING (Server Tasks): - this might be a separate `octopus task follow ID1, ID2, ID3`
// DESIGN CHOICE: We are not going to show servertask progress in the CLI.

type DeployFlags struct {
	Project              *flag.Flag[string]
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
}

func NewDeployFlags() *DeployFlags {
	return &DeployFlags{
		Project:              flag.New[string](FlagProject, false),
		ReleaseVersion:       flag.New[string](FlagReleaseVersion, false),
		Environments:         flag.New[[]string](FlagEnvironment, false),
		Tenants:              flag.New[[]string](FlagTenant, false),
		TenantTags:           flag.New[[]string](FlagTenantTag, false),
		MaxQueueTime:         flag.New[string](FlagDeployAtExpiry, false),
		DeployAt:             flag.New[string](FlagDeployAt, false),
		Variables:            flag.New[[]string](FlagVariable, false),
		UpdateVariables:      flag.New[bool](FlagUpdateVariables, false),
		ExcludedSteps:        flag.New[[]string](FlagSkip, false),
		GuidedFailureMode:    flag.New[string](FlagGuidedFailure, false),
		ForcePackageDownload: flag.New[bool](FlagForcePackageDownload, false),
		DeploymentTargets:    flag.New[[]string](FlagDeploymentTarget, false),
		ExcludeTargets:       flag.New[[]string](FlagExcludeDeploymentTarget, false),
	}
}

func NewCmdDeploy(f factory.Factory) *cobra.Command {
	deployFlags := NewDeployFlags()
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy releases in Octopus Deploy",
		Long:  "Deploy releases in Octopus Deploy.",
		Example: heredoc.Doc(`
			$ octopus release deploy  # fully interactive
			$ octopus release deploy --project MyProject --version 1.0 --environment Dev
			$ octopus release deploy --project MyProject --version 1.0 --tenant-tag Regions/East --tenant-tag Regions/South
			$ octopus release deploy -p MyProject --version 1.0 -e Dev --skip InstallStep --variable VarName:VarValue
			$ octopus release deploy -p MyProject --version 1.0 -e Dev --force-package-download --guided-failure true -f basic
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
	flags.StringVarP(&deployFlags.ReleaseVersion.Value, deployFlags.ReleaseVersion.Name, "", "", "Release version to deploy")
	flags.StringSliceVarP(&deployFlags.Environments.Value, deployFlags.Environments.Name, "e", nil, "Deploy to this environment (can be specified multiple times)")
	flags.StringSliceVarP(&deployFlags.Tenants.Value, deployFlags.Tenants.Name, "", nil, "Deploy to this tenant (can be specified multiple times)")
	flags.StringSliceVarP(&deployFlags.TenantTags.Value, deployFlags.TenantTags.Name, "", nil, "Deploy to tenants matching this tag (can be specified multiple times)")
	flags.StringVarP(&deployFlags.DeployAt.Value, deployFlags.DeployAt.Name, "", "", "Deploy at a later time. Deploy now if omitted. TODO date formats and timezones!")
	flags.StringVarP(&deployFlags.MaxQueueTime.Value, deployFlags.MaxQueueTime.Name, "", "", "Cancel the deployment if it hasn't started within this time period.")
	flags.StringSliceVarP(&deployFlags.Variables.Value, deployFlags.Variables.Name, "v", nil, "Set the value for a prompted variable in the format Label:Value")
	flags.BoolVarP(&deployFlags.UpdateVariables.Value, deployFlags.UpdateVariables.Name, "", false, "Overwrite the release variable snapshot by re-importing variables from the project.")
	flags.StringSliceVarP(&deployFlags.ExcludedSteps.Value, deployFlags.ExcludedSteps.Name, "", nil, "Exclude specific steps from the deployment")
	flags.StringVarP(&deployFlags.GuidedFailureMode.Value, deployFlags.GuidedFailureMode.Name, "", "", "Enable Guided failure mode (true/false/default)")
	flags.BoolVarP(&deployFlags.ForcePackageDownload.Value, deployFlags.ForcePackageDownload.Name, "", false, "Force re-download of packages")
	flags.StringSliceVarP(&deployFlags.DeploymentTargets.Value, deployFlags.DeploymentTargets.Name, "", nil, "Deploy to this target (can be specified multiple times)")
	flags.StringSliceVarP(&deployFlags.ExcludeTargets.Value, deployFlags.ExcludeTargets.Name, "", nil, "Deploy to targets except for this (can be specified multiple times)")

	flags.SortFlags = false

	// flags aliases for compat with old .NET CLI
	flagAliases := make(map[string][]string, 10)
	util.AddFlagAliasesString(flags, FlagReleaseVersion, flagAliases, FlagAliasReleaseNumberLegacy)
	util.AddFlagAliasesStringSlice(flags, FlagEnvironment, flagAliases, FlagAliasDeployToLegacy, FlagAliasEnv)
	util.AddFlagAliasesStringSlice(flags, FlagTenantTag, flagAliases, FlagAliasTag, FlagAliasTenantTagLegacy)
	util.AddFlagAliasesString(flags, FlagDeployAt, flagAliases, FlagAliasWhen, FlagAliasDeployAtLegacy)
	util.AddFlagAliasesString(flags, FlagDeployAtExpiry, flagAliases, FlagDeployAtExpire, FlagAliasNoDeployAfterLegacy)
	util.AddFlagAliasesString(flags, FlagUpdateVariables, flagAliases, FlagAliasUpdateVariablesLegacy)
	util.AddFlagAliasesString(flags, FlagGuidedFailure, flagAliases, FlagAliasGuidedFailureMode, FlagAliasGuidedFailureModeLegacy)
	util.AddFlagAliasesBool(flags, FlagForcePackageDownload, flagAliases, FlagAliasForcePackageDownloadLegacy)
	util.AddFlagAliasesStringSlice(flags, FlagDeploymentTarget, flagAliases, FlagAliasTarget, FlagAliasSpecificMachines)
	util.AddFlagAliasesStringSlice(flags, FlagExcludeDeploymentTarget, flagAliases, FlagAliasExcludeTarget, FlagAliasExcludeMachines)

	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		util.ApplyFlagAliases(cmd.Flags(), flagAliases)
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

	parsedVariables, err := executionscommon.ParseVariableStringArray(flags.Variables.Value)
	if err != nil {
		return err
	}

	options := &executor.TaskOptionsDeployRelease{
		ProjectName:          flags.Project.Value,
		ReleaseVersion:       flags.ReleaseVersion.Value,
		Environments:         flags.Environments.Value,
		Tenants:              flags.Tenants.Value,
		TenantTags:           flags.TenantTags.Value,
		ScheduledStartTime:   flags.DeployAt.Value,
		ScheduledExpiryTime:  flags.MaxQueueTime.Value,
		ExcludedSteps:        flags.ExcludedSteps.Value,
		GuidedFailureMode:    flags.GuidedFailureMode.Value,
		ForcePackageDownload: flags.ForcePackageDownload.Value,
		DeploymentTargets:    flags.DeploymentTargets.Value,
		ExcludeTargets:       flags.ExcludeTargets.Value,
		Variables:            parsedVariables,
		UpdateVariables:      flags.UpdateVariables.Value,
	}

	// special case for FlagForcePackageDownload bool so we can tell if it was set on the cmdline or missing
	if cmd.Flags().Lookup(FlagForcePackageDownload).Changed {
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
			resolvedFlags := NewDeployFlags()
			resolvedFlags.Project.Value = options.ProjectName
			resolvedFlags.ReleaseVersion.Value = options.ReleaseVersion
			resolvedFlags.Environments.Value = options.Environments
			resolvedFlags.Tenants.Value = options.Tenants
			resolvedFlags.TenantTags.Value = options.TenantTags
			resolvedFlags.DeployAt.Value = options.ScheduledStartTime
			resolvedFlags.MaxQueueTime.Value = options.ScheduledExpiryTime
			resolvedFlags.ExcludedSteps.Value = options.ExcludedSteps
			resolvedFlags.GuidedFailureMode.Value = options.GuidedFailureMode
			resolvedFlags.DeploymentTargets.Value = options.DeploymentTargets
			resolvedFlags.ExcludeTargets.Value = options.ExcludeTargets

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

			autoCmd := flag.GenerateAutomationCmd(constants.ExecutableName+" release deploy",
				resolvedFlags.Project,
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
				resolvedFlags.Variables,
			)
			cmd.Printf("\nAutomation Command: %s\n", autoCmd)

			if didMaskSensitiveVariable {
				cmd.Printf("%s\n", output.Yellow("Warning: Command includes some sensitive variable values which have been replaced with placeholders."))
			}
		}
	}

	// the executor will raise errors if any required options are missing
	err = executor.ProcessTasks(octopus, f.GetCurrentSpace(), []*executor.Task{
		executor.NewTask(executor.TaskTypeDeployRelease, options),
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

func AskQuestions(octopus *octopusApiClient.Client, stdout io.Writer, asker question.Asker, space *spaces.Space, options *executor.TaskOptionsDeployRelease, now func() time.Time) error {
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

	// select release

	var selectedRelease *releases.Release
	if options.ReleaseVersion == "" {
		// first we want to ask them to pick a channel just to narrow down the search space for releases (not sent to server)
		selectedChannel, err := selectors.Channel(octopus, asker, "Select channel", selectedProject)
		if err != nil {
			return err
		}
		selectedRelease, err = selectRelease(octopus, asker, "Select a release to deploy", space, selectedProject, selectedChannel)
		if err != nil {
			return err
		}
	} else {
		selectedRelease, err = releases.GetReleaseInProject(octopus, space.ID, selectedProject.ID, options.ReleaseVersion)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "Release %s\n", output.Cyan(selectedRelease.Version))
	}
	options.ReleaseVersion = selectedRelease.Version
	options.ReleaseID = selectedRelease.ID
	if err != nil {
		return err
	}

	isTenanted, err := determineIsTenanted(selectedProject, asker)
	if err != nil {
		return err
	}

	// machine selection later on needs to refer back to the environments.
	// NOTE: this is allowed to remain nil; environments will get looked up later on if needed
	var selectedEnvironments []*environments.Environment
	if isTenanted {
		var selectedEnvironment *environments.Environment
		if len(options.Environments) == 0 {
			deployableEnvironmentIDs, nextEnvironmentID, err := FindDeployableEnvironmentIDs(octopus, selectedRelease)
			if err != nil {
				return err
			}
			selectedEnvironment, err = selectDeploymentEnvironment(asker, octopus, deployableEnvironmentIDs, nextEnvironmentID)
			if err != nil {
				return err
			}
			options.Environments = []string{selectedEnvironment.Name} // executions api allows env names, so let's use these instead so they look nice in generated automationcmd
		} else {
			selectedEnvironment, err = selectors.FindEnvironment(octopus, options.Environments[0])
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(stdout, "Environment %s\n", output.Cyan(selectedEnvironment.Name))
		}
		selectedEnvironments = []*environments.Environment{selectedEnvironment}

		// ask for tenants and/or tags unless some were specified on the command line
		if len(options.Tenants) == 0 && len(options.TenantTags) == 0 {
			options.Tenants, options.TenantTags, err = executionscommon.AskTenantsAndTags(asker, octopus, selectedRelease.ProjectID, selectedEnvironments, true)
			if len(options.Tenants) == 0 && len(options.TenantTags) == 0 {
				return errors.New("no tenants or tags available; cannot deploy")
			}
			if err != nil {
				return err
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
				return err
			}
			selectedEnvironments, err = selectDeploymentEnvironments(asker, octopus, deployableEnvironmentIDs, nextEnvironmentID)
			if err != nil {
				return err
			}
			options.Environments = util.SliceTransform(selectedEnvironments, func(env *environments.Environment) string { return env.Name })
		} else {
			if len(options.Environments) > 0 {
				_, _ = fmt.Fprintf(stdout, "Environments %s\n", output.Cyan(strings.Join(options.Environments, ",")))
			}
		}
	}

	variableSet, err := variables.GetVariableSet(octopus, space.ID, selectedRelease.ProjectVariableSetSnapshotID)
	if err != nil {
		return err
	}
	options.Variables, err = executionscommon.AskVariables(asker, variableSet, options.Variables)
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
			deploymentProcess, err := deployments.GetDeploymentProcess(octopus, space.ID, selectedRelease.ProjectDeploymentProcessSnapshotID)
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
			if len(selectedEnvironments) == 0 { // if the Q&A process earlier hasn't loaded environments already, we need to load them now
				selectedEnvironments, err = executionscommon.FindEnvironments(octopus, options.Environments)
				if err != nil {
					return err
				}
			}
			options.DeploymentTargets, err = askDeploymentTargets(octopus, asker, space.ID, selectedRelease.ID, selectedEnvironments)
			if err != nil {
				return err
			}
		}
	}
	// DONE
	return nil
}

func askDeploymentTargets(octopus *octopusApiClient.Client, asker question.Asker, spaceID string, releaseID string, selectedEnvironments []*environments.Environment) ([]string, error) {
	var results []string

	// this is what the portal does. Can we do it better? I don't know
	for _, env := range selectedEnvironments {
		preview, err := deployments.GetReleaseDeploymentPreview(octopus, spaceID, releaseID, env.ID, true)
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
		Message: "Select environment",
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

func selectDeploymentEnvironments(asker question.Asker, octopus *octopusApiClient.Client, deployableEnvironmentIDs []string, nextDeployEnvironmentID string) ([]*environments.Environment, error) {
	allEnvs, nextDeployEnvironmentName, err := loadEnvironmentsForDeploy(octopus, deployableEnvironmentIDs, nextDeployEnvironmentID)
	if err != nil {
		return nil, err
	}

	optionMap, options := question.MakeItemMapAndOptions(allEnvs, func(e *environments.Environment) string { return e.Name })
	var selectedKeys []string
	err = asker(&survey.MultiSelect{
		Message: "Select environment(s)",
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

func PrintAdvancedSummary(stdout io.Writer, options *executor.TaskOptionsDeployRelease) {
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

func selectRelease(octopus *octopusApiClient.Client, ask question.Asker, questionText string, space *spaces.Space, project *projects.Project, channel *channels.Channel) (*releases.Release, error) {
	foundReleases, err := releases.GetReleasesInProjectChannel(octopus, space.ID, project.ID, channel.ID)
	if err != nil {
		return nil, err
	}

	return question.SelectMap(ask, questionText, foundReleases, func(p *releases.Release) string {
		return p.Version
	})
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
