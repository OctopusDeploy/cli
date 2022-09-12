package deploy

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	cliErrors "github.com/OctopusDeploy/cli/pkg/errors"
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
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
	"io"
	"sort"
	"strconv"
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

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
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

	parsedVariables, err := ParseVariableStringArray(flags.Variables.Value)
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
			resolvedFlags.Variables.Value = ToVariableStringArray(automationVariables)

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

// scheduledStartTimeAnswerFormatter is passed to the DatePicker so that if the user selects a time within the next
// one minute after 'now', it will show the answer as the string "Now" rather than the actual datetime string
func scheduledStartTimeAnswerFormatter(datePicker *surveyext.DatePicker, t time.Time) string {
	if t.Before(datePicker.Now().Add(1 * time.Minute)) {
		return "Now"
	} else {
		return t.String()
	}
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
			deployableEnvironmentIDs, nextEnvironmentID, err := findDeployableEnvironmentIDs(octopus, selectedRelease)
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
			_, _ = fmt.Fprintf(stdout, "Environment %s\n", output.Cyan(selectedEnvironment.Name))
		}
		selectedEnvironments = []*environments.Environment{selectedEnvironment}

		// ask for tenants and/or tags unless some were specified on the command line
		if len(options.Tenants) == 0 && len(options.TenantTags) == 0 {
			options.Tenants, options.TenantTags, err = AskTenantsAndTags(asker, octopus, selectedRelease, selectedEnvironment)
			if len(options.Tenants) == 0 && len(options.TenantTags) == 0 {
				return errors.New("no tenants or tags available; cannot deploy")
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
			deployableEnvironmentIDs, nextEnvironmentID, err := findDeployableEnvironmentIDs(octopus, selectedRelease)
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
	options.Variables, err = AskVariables(asker, variableSet, options.Variables)
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
				AnswerFormatter: scheduledStartTimeAnswerFormatter,
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
			options.ExcludedSteps, err = askExcludedSteps(asker, deploymentProcess.Steps)
			if err != nil {
				return err
			}
		}

		if !isGuidedFailureModeSpecified { // if they deliberately specified false, don't ask them
			options.GuidedFailureMode, err = askGuidedFailureMode(asker)
			if err != nil {
				return err
			}
		}

		if !isForcePackageDownloadSpecified { // if they deliberately specified false, don't ask them
			options.ForcePackageDownload, err = askPackageDownload(asker)
			if err != nil {
				return err
			}
		}

		if !isDeploymentTargetsSpecified {
			// What the web portal does:
			// If tenanted:
			//   foreach tenant:
			//     select deployment target(s)
			// else
			//   select deployment target(s)
			//
			// however the executions API doesn't support deployment targets per-tenant, so in all cases
			// we can only do:
			//   select deployment target(s)
			if len(selectedEnvironments) == 0 { // if the Q&A process earlier hasn't loaded environments already, we need to load them now
				selectedEnvironments, err = findEnvironments(octopus, options.Environments)
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

// findDeployableEnvironmentIDs returns an array of environment IDs that we can deploy to,
// the preferred 'next' environment, and an error
func findDeployableEnvironmentIDs(octopus *octopusApiClient.Client, release *releases.Release) ([]string, string, error) {
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

// given an array of environment names, maps these all to actual objects by querying the server
func findEnvironments(client *octopusApiClient.Client, environmentNames []string) ([]*environments.Environment, error) {
	if len(environmentNames) == 0 {
		return nil, nil
	}
	// there's no "bulk lookup" API, so we either need to do a foreach loop to find each environment individually, or load the entire server's worth of environments
	// it's probably going to be cheaper to just list out all the environments and match them client side, so we'll do that for simplicity's sake
	allEnvs, err := client.Environments.GetAll()
	if err != nil {
		return nil, err
	}

	lookup := make(map[string]*environments.Environment, len(allEnvs))
	for _, env := range allEnvs {
		lookup[strings.ToLower(env.Name)] = env
	}

	var result []*environments.Environment
	for _, name := range environmentNames {
		env := lookup[strings.ToLower(name)]
		if env != nil {
			result = append(result, env)
		} else {
			return nil, fmt.Errorf("cannot find environment %s", name)
		}
	}
	return result, nil
}

func lookupGuidedFailureModeString(value string) string {
	switch value {
	case "", "default":
		return "Use default setting from the target environment"
	case "true", "True":
		return "Use guided failure mode"
	case "false", "False":
		return "Do not use guided failure mode"
	default:
		return fmt.Sprintf("Unknown %s", value)
	}
}

func lookupPackageDownloadString(value bool) string {
	if value {
		return "Use cached packages (if available)"
	} else {
		return "Re-download packages from feed"
	}
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

	gfmStr := lookupGuidedFailureModeString(options.GuidedFailureMode)

	pkgDownloadStr := lookupPackageDownloadString(!options.ForcePackageDownload)

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

func findTenantsAndTags(octopus *octopusApiClient.Client, projectID string, environmentID string) ([]string, []string, error) {
	var validTenants []string
	var validTags []string // these are 'Canonical' values i.e. "Regions/us-east", NOT TagSets-1/Tags-1

	page, err := octopus.Tenants.Get(tenants.TenantsQuery{ProjectID: projectID})
	if err != nil {
		return nil, nil, err
	}
	for page != nil {
		tenantsForMyEnvironment := util.SliceFilter(page.Items, func(t *tenants.Tenant) bool {
			if envIdsForProject, ok := t.ProjectEnvironments[projectID]; ok {
				return util.SliceContains(envIdsForProject, environmentID)
			}
			return false
		})
		for _, tenant := range tenantsForMyEnvironment {
			for _, tag := range tenant.TenantTags {
				if !util.SliceContains(validTags, tag) {
					validTags = append(validTags, tag)
				}
			}
			validTenants = append(validTenants, tenant.Name)
		}

		page, err = page.GetNextPage(octopus.Tenants.GetClient())
		if err != nil {
			return nil, nil, err
		}
	}

	return validTenants, validTags, nil
}

func AskTenantsAndTags(asker question.Asker, octopus *octopusApiClient.Client, release *releases.Release, env *environments.Environment) ([]string, []string, error) {
	// (presumably though we can check if the project itself is linked to any tenants and only ask then)?
	// there is a ListTenants(projectID) api that we can use. /api/tenants?projectID=
	foundTenants, foundTags, err := findTenantsAndTags(octopus, release.ProjectID, env.ID)
	if err != nil {
		return nil, nil, err
	}

	// sort because otherwise they may appear in weird order
	sort.Strings(foundTenants)
	sort.Strings(foundTags)

	// Note: merging the list sets us up for a scenario where a tenant name could hypothetically collide with
	// a tag name; we wouldn't handle that -- in practice this is so unlikely to happen that we can ignore it
	combinedList := append(foundTenants, foundTags...)

	var selection []string
	err = asker(&survey.MultiSelect{
		Message: "Select tenants and/or tags used to determine deployment targets",
		Options: combinedList,
	}, &selection, survey.WithValidator(survey.Required))
	if err != nil {
		return nil, nil, err
	}

	tenantsLookup := make(map[string]bool, len(foundTenants))
	for _, t := range foundTenants {
		tenantsLookup[t] = true
	}
	tagsLookup := make(map[string]bool, len(foundTags))
	for _, t := range foundTags {
		tagsLookup[t] = true
	}

	var selectedTenants []string
	var selectedTags []string

	for _, s := range selection {
		if tenantsLookup[s] {
			selectedTenants = append(selectedTenants, s)
		} else if tagsLookup[s] {
			selectedTags = append(selectedTags, s)
		}
	}

	return selectedTenants, selectedTags, nil
}

func askExcludedSteps(asker question.Asker, steps []*deployments.DeploymentStep) ([]string, error) {
	stepsToExclude, err := question.MultiSelectMap(asker, "Steps to skip (If none selected, run all steps)", steps, func(s *deployments.DeploymentStep) string {
		return s.Name
	}, false)
	if err != nil {
		return nil, err
	}
	return util.SliceTransform(stepsToExclude, func(s *deployments.DeploymentStep) string {
		return s.Name // server expects us to send a list of step names
	}), nil
}

func askPackageDownload(asker question.Asker) (bool, error) {
	result, err := question.SelectMap(asker, "Package download", []bool{true, false}, lookupPackageDownloadString)
	// our question is phrased such that "Use cached packages" (the do-nothing option) is true,
	// but we want to set the --force-package-download flag, so we need to invert the response
	return !result, err
}

func askGuidedFailureMode(asker question.Asker) (string, error) {
	modes := []string{
		"", "true", "false", // maps to a nullable bool in C#
	}
	return question.SelectMap(asker, "Guided Failure Mode", modes, lookupGuidedFailureModeString)
}

// AskVariables returns the map of ALL variables to send to the server, whether they were prompted for, or came from the command line.
// variablesFromCmd is copied into the result, you don't need to merge them yourselves.
// Return values: 0: Variables to send to the server, 1: List of sensitive variable names for masking automation command, 2: error
func AskVariables(asker question.Asker, variableSet *variables.VariableSet, variablesFromCmd map[string]string) (map[string]string, error) {
	if asker == nil {
		return nil, cliErrors.NewArgumentNullOrEmptyError("asker")
	}
	if variableSet == nil {
		return nil, cliErrors.NewArgumentNullOrEmptyError("variableSet")
	}

	// variablesFromCmd is pure user input and may not have correct casing.
	lcaseVarsFromCmd := make(map[string]string, len(variablesFromCmd))
	for k, v := range variablesFromCmd {
		lcaseVarsFromCmd[strings.ToLower(k)] = v
	}

	result := make(map[string]string)
	if len(variableSet.Variables) > 0 { // nothing to be done here, move along
		for _, v := range variableSet.Variables {
			valueFromCmd, foundValueOnCommandLine := lcaseVarsFromCmd[strings.ToLower(v.Name)]
			if foundValueOnCommandLine {
				// implicitly fixes up variable casing
				result[v.Name] = valueFromCmd
			}

			if v.Prompt != nil && !foundValueOnCommandLine { // this is a prompted variable, ask for input (unless we already have it)
				// NOTE: there is a v.Prompt.Label which is shown in the web portal,
				// but we explicitly don't use it here because it can lead to confusion.
				// e.g.
				// A variable "CrmTicketNumber" exists with Label "CRM Ticket Number"
				// If we were to use the label then the prompt would ask for "CRM Ticket Number" but the command line
				// invocation would say "CrmTicketNumber:<value>" and it wouldn't be clear to and end user whether
				// "CrmTicketNumber" or "CRM Ticket Number" was the right thing.
				promptMessage := v.Name

				if v.Prompt.Description != "" {
					promptMessage = fmt.Sprintf("%s (%s)", promptMessage, v.Prompt.Description) // we'd like to dim the description, but survey overrides this, so we can't
				}

				if v.Type == "String" || v.Type == "Sensitive" {
					responseString, err := askVariableSpecificPrompt(asker, promptMessage, v.Type, v.Value, v.Prompt.IsRequired, v.IsSensitive, v.Prompt.DisplaySettings)
					if err != nil {
						return nil, err
					}
					result[v.Name] = responseString
				}
				// else it's a complex variable type with the prompt flag, which (at time of writing) is currently broken
				// and a decision on how to fix it had not yet been made. Ignore it.
				// BUG: https://github.com/OctopusDeploy/Issues/issues/7699
			}
		}
	}
	return result, nil
}

func askVariableSpecificPrompt(asker question.Asker, message string, variableType string, defaultValue string, isRequired bool, isSensitive bool, displaySettings *variables.DisplaySettings) (string, error) {
	var askOpt survey.AskOpt = func(options *survey.AskOptions) error {
		if isRequired {
			options.Validators = append(options.Validators, survey.Required)
		}
		return nil
	}

	// work out what kind of prompt to use
	var controlType variables.ControlType
	if displaySettings != nil && displaySettings.ControlType != "" {
		controlType = displaySettings.ControlType
	} else { // infer the control type based on other flags
		// The shape of the data model allows for the possibility of a sensitive multi-line or sensitive combo-box
		// variable. However, the web portal doesn't implement any of these, the only sensitive thing it supports
		// is single-line text, so we can simplify our logic here.
		if variableType == "Sensitive" || isSensitive {
			// From comment in server:
			// variable.IsSensitive is Kept for backwards compatibility. New way is to use variable.Type=VariableType.Sensitive
			controlType = variables.ControlTypeSensitive
		} else {
			controlType = variables.ControlTypeSingleLineText
		}
	}

	switch controlType {
	case variables.ControlTypeSingleLineText, "": // if control type is not explicitly set it means single line text.
		var response string
		err := asker(&survey.Input{
			Message: message,
			Default: defaultValue,
		}, &response, askOpt)
		return response, err

	case variables.ControlTypeSensitive:
		var response string
		err := asker(&survey.Password{
			Message: message,
		}, &response, askOpt)
		return response, err

	case variables.ControlTypeMultiLineText: // not clear if the server ever does this
		var response string
		err := asker(&surveyext.OctoEditor{
			Editor: &survey.Editor{
				Message:  "message",
				FileName: "*.txt",
			},
			Optional: !isRequired}, &response)
		return response, err

	case variables.ControlTypeSelect:
		if displaySettings == nil {
			return "", cliErrors.NewArgumentNullOrEmptyError("displaySettings") // select needs actual display settings
		}
		reverseLookup := make(map[string]string, len(displaySettings.SelectOptions))
		optionStrings := make([]string, 0, len(displaySettings.SelectOptions))
		displayNameForDefaultValue := ""
		for _, v := range displaySettings.SelectOptions {
			if v.Value == defaultValue {
				displayNameForDefaultValue = v.DisplayName
			}
			optionStrings = append(optionStrings, v.DisplayName)
			reverseLookup[v.DisplayName] = v.Value
		}
		var response string
		err := asker(&survey.Select{
			Message: message,
			Default: displayNameForDefaultValue,
			Options: optionStrings,
		}, &response, askOpt)
		if err != nil {
			return "", err
		}
		return reverseLookup[response], nil

	case variables.ControlTypeCheckbox:
		// if the server didn't specifically set a default value of True then default to No
		defTrueFalse := "False"
		if b, err := strconv.ParseBool(defaultValue); err == nil && b {
			defTrueFalse = "True"
		}
		var response string
		err := asker(&survey.Select{
			Message: message,
			Default: defTrueFalse,
			Options: []string{"True", "False"}, // Yes/No would read more nicely, but doesn't fit well with cmdline which expects True/False
		}, &response, askOpt)
		return response, err

	default:
		return "", fmt.Errorf("unhandled control type %s", displaySettings.ControlType)
	}
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

func ParseVariableStringArray(variables []string) (map[string]string, error) {
	result := make(map[string]string, len(variables))
	for _, v := range variables {
		components := splitVariableString(v, 2)
		if len(components) != 2 || components[0] == "" || components[1] == "" {
			return nil, fmt.Errorf("could not parse variable definition '%s'", v)
		}
		result[strings.TrimSpace(components[0])] = strings.TrimSpace(components[1])
	}
	return result, nil
}

func ToVariableStringArray(variables map[string]string) []string {
	result := make([]string, 0, len(variables))
	for k, v := range variables {
		if k == "" || v == "" {
			continue
		}
		result = append(result, fmt.Sprintf("%s:%s", k, v))
	}
	sort.Strings(result) // sort for reliable test output
	return result
}

// splitVariableString is a derivative of splitPackageOverrideString in release create.
// it is required because the builtin go strings.SplitN can't handle more than one delimeter character.
// otherwise it works the same, but caps the number of splits at 'n'
func splitVariableString(s string, n int) []string {
	// pass 1: collect spans; golang strings.FieldsFunc says it's much more efficient this way
	type span struct {
		start int
		end   int
	}
	spans := make([]span, 0, n)

	// Find the field start and end indices.
	start := 0 // we always start the first span at the beginning of the string
	for idx, ch := range s {
		if ch == ':' || ch == '=' {
			if start >= 0 { // we found a delimiter and we are already in a span; end the span and start a new one
				if len(spans) == n-1 { // we're about to append the last span, break so the 'last field' code consumes the rest of the string
					break
				} else {
					spans = append(spans, span{start, idx})
					start = idx + 1
				}
			} else { // we found a delimiter and we are not in a span; start a new span
				if start < 0 {
					start = idx
				}
			}
		}
	}

	// Last field might end at EOF.
	if start >= 0 {
		spans = append(spans, span{start, len(s)})
	}

	// pass 2: create strings from recorded field indices.
	a := make([]string, len(spans))
	for i, span := range spans {
		a[i] = s[span.start:span.end]
	}
	return a
}
