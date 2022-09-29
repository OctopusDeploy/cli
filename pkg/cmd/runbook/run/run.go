package run

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/release/deploy"
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
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
	"io"
	"math"
	"strings"
	"time"
)

const (
	FlagProject = "project"

	FlagRunbookName        = "name"
	FlagAliasRunbookLegacy = "runbook"

	FlagSnapshot = "snapshot"

	FlagEnvironment = "environment" // can be specified multiple times; but only once if tenanted
	FlagAliasEnv    = "env"

	FlagTenant = "tenant" // can be specified multiple times

	FlagTenantTag            = "tenant-tag" // can be specified multiple times
	FlagAliasTag             = "tag"
	FlagAliasTenantTagLegacy = "tenantTag"

	FlagRunAt            = "run-at" // if this is less than 1 min in the future, go now
	FlagAliasWhen        = "when"   // alias for deploy-at
	FlagAliasRunAtLegacy = "runAt"

	FlagRunAtExpiry           = "run-at-expiry"
	FlagRunAtExpire           = "run-at-expire"
	FlagAliasNoRunAfterLegacy = "noRunAfter"

	FlagSkip = "skip" // can be specified multiple times

	FlagGuidedFailure                = "guided-failure"
	FlagAliasGuidedFailureMode       = "guided-failure-mode"
	FlagAliasGuidedFailureModeLegacy = "guidedFailure"

	FlagForcePackageDownload            = "force-package-download"
	FlagAliasForcePackageDownloadLegacy = "forcePackageDownload"

	FlagRunTarget             = "run-target"
	FlagAliasTarget           = "target"           // alias for deployment-target
	FlagAliasSpecificMachines = "specificMachines" // octo wants a comma separated list. We prefer specifying --target multiple times, but CSV also works because pflag does it for free

	FlagExcludeRunTarget     = "exclude-run-target"
	FlagAliasExcludeTarget   = "exclude-target"
	FlagAliasExcludeMachines = "excludeMachines" // octo wants a comma separated list. We prefer specifying --exclude-target multiple times, but CSV also works because pflag does it for free

	FlagVariable = "variable"
)

// executions API stops here.

// DEPLOYMENT TRACKING (Server Tasks): - this might be a separate `octopus task follow ID1, ID2, ID3`
// DESIGN CHOICE: We are not going to show servertask progress in the CLI.

type RunFlags struct {
	Project              *flag.Flag[string]
	RunbookName          *flag.Flag[string] // the runbook to run
	Environments         *flag.Flag[[]string]
	Tenants              *flag.Flag[[]string]
	TenantTags           *flag.Flag[[]string]
	RunAt                *flag.Flag[string]
	MaxQueueTime         *flag.Flag[string]
	Variables            *flag.Flag[[]string]
	Snapshot             *flag.Flag[string]
	ExcludedSteps        *flag.Flag[[]string]
	GuidedFailureMode    *flag.Flag[string] // tri-state: true, false, or "use default". Can we model it with an optional bool?
	ForcePackageDownload *flag.Flag[bool]
	RunTargets           *flag.Flag[[]string]
	ExcludeTargets       *flag.Flag[[]string]
}

func NewRunFlags() *RunFlags {
	return &RunFlags{
		Project:              flag.New[string](FlagProject, false),
		RunbookName:          flag.New[string](FlagRunbookName, false),
		Environments:         flag.New[[]string](FlagEnvironment, false),
		Tenants:              flag.New[[]string](FlagTenant, false),
		TenantTags:           flag.New[[]string](FlagTenantTag, false),
		MaxQueueTime:         flag.New[string](FlagRunAtExpiry, false),
		RunAt:                flag.New[string](FlagRunAt, false),
		Variables:            flag.New[[]string](FlagVariable, false),
		Snapshot:             flag.New[string](FlagSnapshot, false),
		ExcludedSteps:        flag.New[[]string](FlagSkip, false),
		GuidedFailureMode:    flag.New[string](FlagGuidedFailure, false),
		ForcePackageDownload: flag.New[bool](FlagForcePackageDownload, false),
		RunTargets:           flag.New[[]string](FlagRunTarget, false),
		ExcludeTargets:       flag.New[[]string](FlagExcludeRunTarget, false),
	}
}

func NewCmdRun(f factory.Factory) *cobra.Command {
	runFlags := NewRunFlags()
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run runbooks in Octopus Deploy",
		Long:  "Run runbooks in Octopus Deploy.",
		Example: heredoc.Doc(`
			$ octopus runbook run  # fully interactive
			$ octopus runbook run --project MyProject ... TODO
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && runFlags.Project.Value == "" {
				runFlags.Project.Value = args[0]
			}

			return runbookRun(cmd, f, runFlags)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&runFlags.Project.Value, runFlags.Project.Name, "p", "", "Name or ID of the project to run the runbook from")
	flags.StringVarP(&runFlags.RunbookName.Value, runFlags.RunbookName.Name, "n", "", "Name of the runbook to run")
	flags.StringSliceVarP(&runFlags.Environments.Value, runFlags.Environments.Name, "e", nil, "Run in this environment (can be specified multiple times)")
	flags.StringSliceVarP(&runFlags.Tenants.Value, runFlags.Tenants.Name, "", nil, "Run for this tenant (can be specified multiple times)")
	flags.StringSliceVarP(&runFlags.TenantTags.Value, runFlags.TenantTags.Name, "", nil, "Run for tenants matching this tag (can be specified multiple times)")
	flags.StringVarP(&runFlags.RunAt.Value, runFlags.RunAt.Name, "", "", "Run at a later time. Run now if omitted. TODO date formats and timezones!")
	flags.StringVarP(&runFlags.MaxQueueTime.Value, runFlags.MaxQueueTime.Name, "", "", "Cancel a scheduled run if it hasn't started within this time period.")
	flags.StringSliceVarP(&runFlags.Variables.Value, runFlags.Variables.Name, "v", nil, "Set the value for a prompted variable in the format Label:Value")
	flags.StringVarP(&runFlags.Snapshot.Value, runFlags.Snapshot.Name, "", "", "Name or ID of the snapshot to run. If not supplied, the command will attempt to use the published snapshot.")
	flags.StringSliceVarP(&runFlags.ExcludedSteps.Value, runFlags.ExcludedSteps.Name, "", nil, "Exclude specific steps from the runbook")
	flags.StringVarP(&runFlags.GuidedFailureMode.Value, runFlags.GuidedFailureMode.Name, "", "", "Enable Guided failure mode (true/false/default)")
	flags.BoolVarP(&runFlags.ForcePackageDownload.Value, runFlags.ForcePackageDownload.Name, "", false, "Force re-download of packages")
	flags.StringSliceVarP(&runFlags.RunTargets.Value, runFlags.RunTargets.Name, "", nil, "Run on this target (can be specified multiple times)")
	flags.StringSliceVarP(&runFlags.ExcludeTargets.Value, runFlags.ExcludeTargets.Name, "", nil, "Run on targets except for this (can be specified multiple times)")

	flags.SortFlags = false

	// flags aliases for compat with old .NET CLI
	flagAliases := make(map[string][]string, 10)
	util.AddFlagAliasesString(flags, FlagRunbookName, flagAliases, FlagAliasRunbookLegacy)
	util.AddFlagAliasesStringSlice(flags, FlagEnvironment, flagAliases, FlagAliasEnv)
	util.AddFlagAliasesStringSlice(flags, FlagTenantTag, flagAliases, FlagAliasTag, FlagAliasTenantTagLegacy)
	util.AddFlagAliasesString(flags, FlagRunAt, flagAliases, FlagAliasWhen, FlagAliasRunAtLegacy)
	util.AddFlagAliasesString(flags, FlagRunAtExpiry, flagAliases, FlagRunAtExpire, FlagAliasNoRunAfterLegacy)
	util.AddFlagAliasesString(flags, FlagGuidedFailure, flagAliases, FlagAliasGuidedFailureMode, FlagAliasGuidedFailureModeLegacy)
	util.AddFlagAliasesBool(flags, FlagForcePackageDownload, flagAliases, FlagAliasForcePackageDownloadLegacy)
	util.AddFlagAliasesStringSlice(flags, FlagRunTarget, flagAliases, FlagAliasTarget, FlagAliasSpecificMachines)
	util.AddFlagAliasesStringSlice(flags, FlagExcludeRunTarget, flagAliases, FlagAliasExcludeTarget, FlagAliasExcludeMachines)

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		util.ApplyFlagAliases(cmd.Flags(), flagAliases)
		return nil
	}
	return cmd
}

func runbookRun(cmd *cobra.Command, f factory.Factory, flags *RunFlags) error {
	outputFormat, err := cmd.Flags().GetString(constants.FlagOutputFormat)
	if err != nil { // should never happen, but fallback if it does
		outputFormat = constants.OutputFormatTable
	}

	octopus, err := f.GetSpacedClient()
	if err != nil {
		return err
	}

	parsedVariables, err := deploy.ParseVariableStringArray(flags.Variables.Value)
	if err != nil {
		return err
	}

	options := &executor.TaskOptionsRunbookRun{
		ProjectName:          flags.Project.Value,
		RunbookName:          flags.RunbookName.Value,
		Environments:         flags.Environments.Value,
		Tenants:              flags.Tenants.Value,
		TenantTags:           flags.TenantTags.Value,
		ScheduledStartTime:   flags.RunAt.Value,
		ScheduledExpiryTime:  flags.MaxQueueTime.Value,
		ExcludedSteps:        flags.ExcludedSteps.Value,
		GuidedFailureMode:    flags.GuidedFailureMode.Value,
		ForcePackageDownload: flags.ForcePackageDownload.Value,
		RunTargets:           flags.RunTargets.Value,
		ExcludeTargets:       flags.ExcludeTargets.Value,
		Variables:            parsedVariables,
		Snapshot:             flags.Snapshot.Value,
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
			resolvedFlags := NewRunFlags()
			resolvedFlags.Project.Value = options.ProjectName
			resolvedFlags.RunbookName.Value = options.RunbookName
			resolvedFlags.Environments.Value = options.Environments
			resolvedFlags.Tenants.Value = options.Tenants
			resolvedFlags.TenantTags.Value = options.TenantTags
			resolvedFlags.RunAt.Value = options.ScheduledStartTime
			resolvedFlags.MaxQueueTime.Value = options.ScheduledExpiryTime
			resolvedFlags.ExcludedSteps.Value = options.ExcludedSteps
			resolvedFlags.GuidedFailureMode.Value = options.GuidedFailureMode
			resolvedFlags.RunTargets.Value = options.RunTargets
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
			resolvedFlags.Variables.Value = deploy.ToVariableStringArray(automationVariables)

			// we're deliberately adding --no-prompt to the generated cmdline so ForcePackageDownload=false will be missing,
			// but that's fine
			resolvedFlags.ForcePackageDownload.Value = options.ForcePackageDownload

			autoCmd := flag.GenerateAutomationCmd(constants.ExecutableName+" runbook run",
				resolvedFlags.Project,
				resolvedFlags.RunbookName,
				resolvedFlags.Snapshot,
				resolvedFlags.Environments,
				resolvedFlags.Tenants,
				resolvedFlags.TenantTags,
				resolvedFlags.RunAt,
				resolvedFlags.MaxQueueTime,
				resolvedFlags.ExcludedSteps,
				resolvedFlags.GuidedFailureMode,
				resolvedFlags.ForcePackageDownload,
				resolvedFlags.RunTargets,
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
		executor.NewTask(executor.TaskTypeRunbookRun, options),
	})
	if err != nil {
		return err
	}

	if options.Response != nil {
		switch outputFormat {
		case constants.OutputFormatBasic:
			for _, task := range options.Response.RunbookRunServerTasks {
				cmd.Printf("%s\n", task.ServerTaskID)
			}

		case constants.OutputFormatJson:
			data, err := json.Marshal(options.Response.RunbookRunServerTasks)
			if err != nil { // shouldn't happen but fallback in case
				cmd.PrintErrln(err)
			} else {
				_, _ = cmd.OutOrStdout().Write(data)
				cmd.Println()
			}
		default: // table
			cmd.Printf("Successfully started %d runbook run(s)\n", len(options.Response.RunbookRunServerTasks))
		}

		// output web URL all the time, so long as output format is not JSON or basic
		//if err == nil && !constants.IsProgrammaticOutputFormat(outputFormat) {
		//	releaseID := options.ReleaseID
		//	if releaseID == "" {
		//		// we may already have the release ID from AskQuestions. If not, we need to go and look up the release ID to link to it
		//		// which needs the project ID. Errors here are ignorable; it's not the end of the world if we can't print the web link
		//		prj, err := selectors.FindProject(octopus, options.ProjectName)
		//		if err == nil {
		//			rel, err := releases.GetReleaseInProject(octopus, f.GetCurrentSpace().ID, prj.ID, options.ReleaseVersion)
		//			if err == nil {
		//				releaseID = rel.ID
		//			}
		//		}
		//	}
		//
		//	if releaseID != "" {
		//		link := output.Bluef("%s/app#/%s/releases/%s", f.GetCurrentHost(), f.GetCurrentSpace().ID, releaseID)
		//		cmd.Printf("\nView this release on Octopus Deploy: %s\n", link)
		//	}
		//}
	}

	return nil
}

func AskQuestions(octopus *octopusApiClient.Client, stdout io.Writer, asker question.Asker, space *spaces.Space, options *executor.TaskOptionsRunbookRun, now func() time.Time) error {
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

	// select the runbook

	var selectedRunbook *runbooks.Runbook
	if options.RunbookName == "" {
		selectedRunbook, err = selectRunbook(octopus, asker, "Select a runbook to run", space, selectedProject)
		if err != nil {
			return err
		}
	} else {
		selectedRunbook, err = findRunbook(octopus, space.ID, selectedProject.ID, options.RunbookName)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "Runbook %s\n", output.Cyan(selectedRunbook.Name))
	}
	options.RunbookName = selectedRunbook.Name
	if err != nil {
		return err
	}

	isTenanted, err := determineIsTenanted(selectedRunbook, asker)
	if err != nil {
		return err
	}

	// machine selection later on needs to refer back to the environments.
	// NOTE: this is allowed to remain nil; environments will get looked up later on if needed
	var selectedEnvironments []*environments.Environment
	// tenanted runs only allow one environment, untenanted allows multiple environments
	if isTenanted {
		var selectedEnvironment *environments.Environment
		if len(options.Environments) == 0 {
			selectedEnvironment, err = selectRunEnvironment(asker, octopus, space, selectedProject, selectedRunbook)
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
			options.Tenants, options.TenantTags, err = deploy.AskTenantsAndTags(asker, octopus, selectedRunbook.ProjectID, selectedEnvironment)
			if len(options.Tenants) == 0 && len(options.TenantTags) == 0 {
				return errors.New("no tenants or tags available; cannot run")
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
			selectedEnvironments, err = selectRunEnvironments(asker, octopus, space, selectedProject, selectedRunbook)
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

	// TODO this whole snapshot bit is slightly wrong. Talk to Shannon and figure out how we are supposed to deal with it

	// The runbook snapshot contains the variables, we must ask about snapshots in the mainline query path
	// because otherwise we won't know if there are any required prompted variables
	var selectedSnapshot *runbooks.RunbookSnapshot
	if options.Snapshot == "" {
		selectedSnapshot, err = selectRunbookSnapshot(octopus, asker, "Select a snapshot", space, selectedProject, selectedRunbook)
		if err != nil {
			return err
		}
	} else {
		selectedSnapshot, err = findRunbookSnapshot(octopus, space.ID, selectedProject.ID, options.Snapshot)
		if err != nil {
			return err
		}
	}
	// TODO if they selected 'published snapshot', then don't back-propagate into the generated automation command
	options.Snapshot = selectedSnapshot.Name

	variableSet, err := variables.GetVariableSet(octopus, space.ID, selectedSnapshot.FrozenProjectVariableSetID)
	if err != nil {
		return err
	}
	options.Variables, err = deploy.AskVariables(asker, variableSet, options.Variables)
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
	isDeploymentTargetsSpecified := len(options.RunTargets) > 0 || len(options.ExcludeTargets) > 0

	allAdvancedOptionsSpecified := isDeployAtSpecified && isExcludedStepsSpecified && isGuidedFailureModeSpecified && isForcePackageDownloadSpecified && isDeploymentTargetsSpecified

	shouldAskAdvancedQuestions := false
	if !allAdvancedOptionsSpecified {
		var changeOptionsAnswer string
		err = asker(&survey.Select{
			Message: "Change additional options?",
			Options: []string{"Proceed to run", "Change"},
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
				Help:            "Enter the date and time that this runbook should run. A value less than 1 minute in the future means 'now'",
				Default:         referenceNow,
				Min:             referenceNow,
				Max:             maxSchedStartTime,
				OverrideNow:     referenceNow,
				AnswerFormatter: deploy.ScheduledStartTimeAnswerFormatter,
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
			runbookProcess, err := runbooks.GetProcess(octopus, space.ID, selectedProject.ID, selectedSnapshot.FrozenRunbookProcessID)
			if err != nil {
				return err
			}
			options.ExcludedSteps, err = deploy.AskExcludedSteps(asker, runbookProcess.Steps)
			if err != nil {
				return err
			}
		}

		if !isGuidedFailureModeSpecified { // if they deliberately specified false, don't ask them
			options.GuidedFailureMode, err = deploy.AskGuidedFailureMode(asker)
			if err != nil {
				return err
			}
		}

		if !isForcePackageDownloadSpecified { // if they deliberately specified false, don't ask them
			options.ForcePackageDownload, err = deploy.AskPackageDownload(asker)
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
				selectedEnvironments, err = deploy.FindEnvironments(octopus, options.Environments)
				if err != nil {
					return err
				}
			}
			// TODO figure out how to ask for runbook target machines
			//options.RunTargets, err = askDeploymentTargets(octopus, asker, space.ID, selectedRunbook.ID, selectedEnvironments)
			//if err != nil {
			//	return err
			//}
		}
	}
	// DONE
	return nil
}

// selectRunEnvironment selects a single environment for use in a tenanted run
func selectRunEnvironment(ask question.Asker, octopus *octopusApiClient.Client, space *spaces.Space, project *projects.Project, runbook *runbooks.Runbook) (*environments.Environment, error) {
	envs, err := runbooks.ListEnvironments(octopus, space.ID, project.ID, runbook.ID)
	if err != nil {
		return nil, err
	}

	return question.SelectMap(ask, "Select an environment", envs, func(p *environments.Environment) string {
		return p.Name
	})
}

// selectRunEnvironments selects multiple environments for use in an untenanted run
func selectRunEnvironments(ask question.Asker, octopus *octopusApiClient.Client, space *spaces.Space, project *projects.Project, runbook *runbooks.Runbook) ([]*environments.Environment, error) {
	envs, err := runbooks.ListEnvironments(octopus, space.ID, project.ID, runbook.ID)
	if err != nil {
		return nil, err
	}

	return question.MultiSelectMap(ask, "Select one or more environments", envs, func(p *environments.Environment) string {
		return p.Name
	}, true)
}

func PrintAdvancedSummary(stdout io.Writer, options *executor.TaskOptionsRunbookRun) {
	deployAtStr := "Now"
	if options.ScheduledStartTime != "" {
		deployAtStr = options.ScheduledStartTime // we assume the server is going to understand this
	}
	skipStepsStr := "None"
	if len(options.ExcludedSteps) > 0 {
		skipStepsStr = strings.Join(options.ExcludedSteps, ",")
	}

	gfmStr := deploy.LookupGuidedFailureModeString(options.GuidedFailureMode)

	pkgDownloadStr := deploy.LookupPackageDownloadString(!options.ForcePackageDownload)

	runTargetsStr := "All included"
	if len(options.RunTargets) != 0 || len(options.ExcludeTargets) != 0 {
		sb := strings.Builder{}
		if len(options.RunTargets) > 0 {
			sb.WriteString("Include ")
			for idx, name := range options.RunTargets {
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
		runTargetsStr = sb.String()
	}

	_, _ = fmt.Fprintf(stdout, output.FormatDoc(heredoc.Doc(`
		bold(Additional Options):
		  Run At: cyan(%s)
		  Skipped Steps: cyan(%s)
		  Guided Failure Mode: cyan(%s)
		  Package Download: cyan(%s)
		  Targets: cyan(%s)
	`)), deployAtStr, skipStepsStr, gfmStr, pkgDownloadStr, runTargetsStr)
}

// This mirrors determineIsTenanted in deploy release, but looks at a runbook, rather than a project
func determineIsTenanted(runbook *runbooks.Runbook, ask question.Asker) (bool, error) {
	switch runbook.MultiTenancyMode {
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
		return false, fmt.Errorf("unhandled tenanted deployment mode %s", runbook.MultiTenancyMode)
	}
}

func selectRunbook(octopus *octopusApiClient.Client, ask question.Asker, questionText string, space *spaces.Space, project *projects.Project) (*runbooks.Runbook, error) {
	foundRunbooks, err := runbooks.List(octopus, space.ID, project.ID, "", math.MaxInt32)
	if err != nil {
		return nil, err
	}

	return question.SelectMap(ask, questionText, foundRunbooks.Items, func(p *runbooks.Runbook) string {
		return p.Name
	})
}

// findRunbook wraps the API client, such that we are always guaranteed to get a result, or error. The "successfully can't find matching name" case doesn't exist
func findRunbook(octopus *octopusApiClient.Client, spaceID string, projectID string, runbookName string) (*runbooks.Runbook, error) {
	result, err := runbooks.GetByName(octopus, spaceID, projectID, runbookName)
	if result == nil && err == nil {
		return nil, fmt.Errorf("no runbook found with Name of %s", runbookName)
	}
	return result, err
}

func selectRunbookSnapshot(octopus *octopusApiClient.Client, ask question.Asker, questionText string, space *spaces.Space, project *projects.Project, runbook *runbooks.Runbook) (*runbooks.RunbookSnapshot, error) {
	foundSnapshots, err := runbooks.ListSnapshots(octopus, space.ID, project.ID, runbook.ID, math.MaxInt32)
	if err != nil {
		return nil, err
	}

	return question.SelectMap(ask, questionText, foundSnapshots.Items, func(p *runbooks.RunbookSnapshot) string {
		return p.Name
	})
}

// findRunbookSnapshot wraps the API client, such that we are always guaranteed to get a result, or error. The "successfully can't find matching name" case doesn't exist
func findRunbookSnapshot(octopus *octopusApiClient.Client, spaceID string, projectID string, snapshotIDorName string) (*runbooks.RunbookSnapshot, error) {
	result, err := runbooks.GetSnapshot(octopus, spaceID, projectID, snapshotIDorName)
	if result == nil && err == nil {
		return nil, fmt.Errorf("no snapshot found with ID or Name of %s", snapshotIDorName)
	}
	return result, err
}
