package run

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/OctopusDeploy/cli/pkg/apiclient"

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
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/feeds"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
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
	FlagAliasWhen        = "when"   // alias for run-at
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
	FlagAliasTarget           = "target"           // alias for run-target
	FlagAliasSpecificMachines = "specificMachines" // octo wants a comma separated list. We prefer specifying --target multiple times, but CSV also works because pflag does it for free

	FlagExcludeRunTarget     = "exclude-run-target"
	FlagAliasExcludeTarget   = "exclude-target"
	FlagAliasExcludeMachines = "excludeMachines" // octo wants a comma separated list. We prefer specifying --exclude-target multiple times, but CSV also works because pflag does it for free

	FlagVariable = "variable"

	FlagGitRef             = "git-ref"
	FlagPackageVersion     = "package-version"
	FlagPackageVersionSpec = "package"
	FlagGitResourceRefSpec = "git-resource"
)

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
	GitRef               *flag.Flag[string]
	PackageVersion       *flag.Flag[string]
	PackageVersionSpec   *flag.Flag[[]string]
	GitResourceRefsSpec  *flag.Flag[[]string]
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
		GitRef:               flag.New[string](FlagGitRef, false),
		PackageVersion:       flag.New[string](FlagPackageVersion, false),
		PackageVersionSpec:   flag.New[[]string](FlagPackageVersionSpec, false),
		GitResourceRefsSpec:  flag.New[[]string](FlagGitResourceRefSpec, false),
	}
}

func NewCmdRun(f factory.Factory) *cobra.Command {
	runFlags := NewRunFlags()
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run runbooks in Octopus Deploy",
		Long:  "Run runbooks in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s runbook run  # fully interactive
			$ %[1]s runbook run --project MyProject ... TODO
		`, constants.ExecutableName),
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
	flags.StringArrayVarP(&runFlags.Environments.Value, runFlags.Environments.Name, "e", nil, "Run in this environment (can be specified multiple times)")
	flags.StringArrayVarP(&runFlags.Tenants.Value, runFlags.Tenants.Name, "", nil, "Run for this tenant (can be specified multiple times)")
	flags.StringArrayVarP(&runFlags.TenantTags.Value, runFlags.TenantTags.Name, "", nil, "Run for tenants matching this tag (can be specified multiple times). Format is 'Tag Set Name/Tag Name', such as 'Regions/South'.")
	flags.StringVarP(&runFlags.RunAt.Value, runFlags.RunAt.Name, "", "", "Run at a later time. Run now if omitted. TODO date formats and timezones!")
	flags.StringVarP(&runFlags.MaxQueueTime.Value, runFlags.MaxQueueTime.Name, "", "", "Cancel a scheduled run if it hasn't started within this time period.")
	flags.StringArrayVarP(&runFlags.Variables.Value, runFlags.Variables.Name, "v", nil, "Set the value for a prompted variable in the format Label:Value")
	flags.StringVarP(&runFlags.Snapshot.Value, runFlags.Snapshot.Name, "", "", "Name or ID of the snapshot to run. If not supplied, the command will attempt to use the published snapshot.")
	flags.StringArrayVarP(&runFlags.ExcludedSteps.Value, runFlags.ExcludedSteps.Name, "", nil, "Exclude specific steps from the runbook")
	flags.StringVarP(&runFlags.GuidedFailureMode.Value, runFlags.GuidedFailureMode.Name, "", "", "Enable Guided failure mode (true/false/default)")
	flags.BoolVarP(&runFlags.ForcePackageDownload.Value, runFlags.ForcePackageDownload.Name, "", false, "Force re-download of packages")
	flags.StringArrayVarP(&runFlags.RunTargets.Value, runFlags.RunTargets.Name, "", nil, "Run on this target (can be specified multiple times)")
	flags.StringArrayVarP(&runFlags.ExcludeTargets.Value, runFlags.ExcludeTargets.Name, "", nil, "Run on targets except for this (can be specified multiple times)")
	flags.StringVarP(&runFlags.GitRef.Value, runFlags.GitRef.Name, "", "", "Git Reference e.g. refs/heads/main. Only relevant for config-as-code projects where runbooks are stored in Git.")
	flags.StringVarP(&runFlags.PackageVersion.Value, runFlags.PackageVersion.Name, "", "", "Default version to use for all packages. Only relevant for config-as-code projects where runbooks are stored in Git.")
	flags.StringArrayVarP(&runFlags.PackageVersionSpec.Value, runFlags.PackageVersionSpec.Name, "", nil, "Version specification for a specific package.\nFormat as {package}:{version}, {step}:{version} or {package-ref-name}:{packageOrStep}:{version}\nYou may specify this multiple times.\nOnly relevant for config-as-code projects where runbooks are stored in Git.")
	flags.StringArrayVarP(&runFlags.GitResourceRefsSpec.Value, runFlags.GitResourceRefsSpec.Name, "", nil, "Git reference for a specific Git resource.\nFormat as {step}:{git-ref}, {step}:{git-resource-name}:{git-ref}\nYou may specify this multiple times.\nOnly relevant for config-as-code projects where runbooks are stored in Git.")

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

	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
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

	octopus, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	parsedVariables, err := executionscommon.ParseVariableStringArray(flags.Variables.Value)
	if err != nil {
		return err
	}

	project, err := selectProject(octopus, cmd.OutOrStdout(), f.Ask, flags.Project.Value)

	if err != nil {
		return err
	}

	flags.Project.Value = project.Name
	runbooksAreInGit := false

	if project.PersistenceSettings.Type() == projects.PersistenceSettingsTypeVersionControlled {
		runbooksAreInGit = project.PersistenceSettings.(projects.GitPersistenceSettings).RunbooksAreInGit()
	}

	if runbooksAreInGit {
		return runGitRunbook(cmd, f, flags, octopus, project, parsedVariables, outputFormat)
	} else {
		return runDbRunbook(cmd, f, flags, octopus, project, parsedVariables, outputFormat)
	}
}

func runDbRunbook(cmd *cobra.Command, f factory.Factory, flags *RunFlags, octopus *octopusApiClient.Client, project *projects.Project, parsedVariables map[string]string, outputFormat string) error {
	options := &executor.TaskOptionsRunbookRun{
		ProjectName:          project.Name,
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

		err := AskDbRunbookRunQuestions(octopus, cmd.OutOrStdout(), f.Ask, f.GetCurrentSpace(), project, options, now)
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
			resolvedFlags.Variables.Value = executionscommon.ToVariableStringArray(automationVariables)

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
	err := executor.ProcessTasks(octopus, f.GetCurrentSpace(), []*executor.Task{
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
	}

	return nil
}

func runGitRunbook(cmd *cobra.Command, f factory.Factory, flags *RunFlags, octopus *octopusApiClient.Client, project *projects.Project, parsedVariables map[string]string, outputFormat string) error {
	commonOptions := &executor.TaskOptionsRunbookRunBase{
		ProjectName:          project.Name,
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
	}
	options := &executor.TaskOptionsGitRunbookRun{
		GitReference: flags.GitRef.Value,
	}

	options.TaskOptionsRunbookRunBase = *commonOptions

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

		err := AskGitRunbookRunQuestions(octopus, cmd.OutOrStdout(), f.Ask, f.GetCurrentSpace(), project, options, now)
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
			resolvedFlags.GitRef.Value = options.GitReference
			resolvedFlags.PackageVersion.Value = options.DefaultPackageVersion
			resolvedFlags.PackageVersionSpec.Value = options.PackageVersionOverrides
			resolvedFlags.GitResourceRefsSpec.Value = options.GitResourceRefs

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

			autoCmd := flag.GenerateAutomationCmd(constants.ExecutableName+" runbook run",
				resolvedFlags.Project,
				resolvedFlags.GitRef,
				resolvedFlags.RunbookName,
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
				resolvedFlags.PackageVersion,
				resolvedFlags.PackageVersionSpec,
				resolvedFlags.GitResourceRefsSpec,
			)
			cmd.Printf("\nAutomation Command: %s\n", autoCmd)

			if didMaskSensitiveVariable {
				cmd.Printf("%s\n", output.Yellow("Warning: Command includes some sensitive variable values which have been replaced with placeholders."))
			}
		}
	}

	// the executor will raise errors if any required options are missing
	err := executor.ProcessTasks(octopus, f.GetCurrentSpace(), []*executor.Task{
		executor.NewTask(executor.TaskTypeGitRunbookRun, options),
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
	}

	return nil
}

func selectProject(octopus *octopusApiClient.Client, stdout io.Writer, asker question.Asker, projectName string) (*projects.Project, error) {
	if projectName == "" {
		selectedProject, err := selectors.Project("Select project", octopus, asker)
		if err != nil {
			return nil, err
		}
		return selectedProject, nil
	} else { // project name is already provided, fetch the object because it's needed for further questions
		selectedProject, err := selectors.FindProject(octopus, projectName)
		if err != nil {
			return nil, err
		}
		_, _ = fmt.Fprintf(stdout, "Project %s\n", output.Cyan(selectedProject.Name))
		return selectedProject, nil
	}
}

func AskDbRunbookRunQuestions(octopus *octopusApiClient.Client, stdout io.Writer, asker question.Asker, space *spaces.Space, project *projects.Project, options *executor.TaskOptionsRunbookRun, now func() time.Time) error {
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

	// select the runbook

	var selectedRunbook *runbooks.Runbook
	if options.RunbookName == "" {
		selectedRunbook, err = selectRunbook(octopus, asker, "Select a runbook to run", space, project)
		if err != nil {
			return err
		}
	} else {
		selectedRunbook, err = findRunbook(octopus, space.ID, project.ID, options.RunbookName)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "Runbook %s\n", output.Cyan(selectedRunbook.Name))
	}
	options.RunbookName = selectedRunbook.Name
	if err != nil {
		return err
	}

	// machine selection later on needs to refer back to the environments.
	var selectedEnvironments []*environments.Environment
	if len(options.Environments) == 0 {
		selectedEnvironments, err = selectRunEnvironments(asker, octopus, space, project, selectedRunbook)
		if err != nil {
			return err
		}
		options.Environments = util.SliceTransform(selectedEnvironments, func(env *environments.Environment) string { return env.Name })
	} else {
		_, _ = fmt.Fprintf(stdout, "Environments %s\n", output.Cyan(strings.Join(options.Environments, ",")))
	}

	// ask for tenants and/or tags unless some were specified on the command line
	if len(options.Tenants) == 0 && len(options.TenantTags) == 0 {
		tenantedDeploymentMode := false
		if project.TenantedDeploymentMode == core.TenantedDeploymentModeTenanted {
			tenantedDeploymentMode = true
		}
		options.Tenants, options.TenantTags, _ = executionscommon.AskTenantsAndTags(asker, octopus, selectedRunbook.ProjectID, selectedEnvironments, tenantedDeploymentMode)
	} else {
		if len(options.Tenants) > 0 {
			_, _ = fmt.Fprintf(stdout, "Tenants %s\n", output.Cyan(strings.Join(options.Tenants, ",")))
		}
		if len(options.TenantTags) > 0 {
			_, _ = fmt.Fprintf(stdout, "Tenant Tags %s\n", output.Cyan(strings.Join(options.TenantTags, ",")))
		}
	}

	// The runbook snapshot contains the variables, we must ask about snapshots in the mainline query path
	// because otherwise we won't know if there are any required prompted variables.
	// NOTE: Using a non-default snapshot is advanced/niche behaviour, you can only opt into this by specifying
	// --snapshot on the command line, there's deliberately no interactive flow for it.
	var selectedSnapshot *runbooks.RunbookSnapshot
	if options.Snapshot != "" {
		selectedSnapshot, err = findRunbookSnapshot(octopus, space.ID, project.ID, options.Snapshot)
		if err != nil {
			return err
		}
	} else {
		selectedSnapshot, err = findRunbookPublishedSnapshot(octopus, space, project, selectedRunbook)
		if err != nil {
			return err
		}
	}

	variableSet, err := variables.GetVariableSet(octopus, space.ID, selectedSnapshot.FrozenProjectVariableSetID)
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

	isRunAtSpecified := options.ScheduledStartTime != ""
	isExcludedStepsSpecified := len(options.ExcludedSteps) > 0
	isGuidedFailureModeSpecified := options.GuidedFailureMode != ""
	isForcePackageDownloadSpecified := options.ForcePackageDownloadWasSpecified
	isRunTargetsSpecified := len(options.RunTargets) > 0 || len(options.ExcludeTargets) > 0

	allAdvancedOptionsSpecified := isRunAtSpecified && isExcludedStepsSpecified && isGuidedFailureModeSpecified && isForcePackageDownloadSpecified && isRunTargetsSpecified

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
		if !isRunAtSpecified {
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
					Help:        "At the start time, the run will be queued. If it does not begin before 'expiry' time, it will be cancelled. Minimum of 5 minutes after start time",
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
			runbookProcess, err := runbooks.GetProcess(octopus, space.ID, project.ID, selectedSnapshot.FrozenRunbookProcessID)
			if err != nil {
				return err
			}
			options.ExcludedSteps, err = executionscommon.AskExcludedSteps(asker, runbookProcess.Steps)
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

		if !isRunTargetsSpecified {
			if len(selectedEnvironments) == 0 { // if the Q&A process earlier hasn't loaded environments already, we need to load them now
				selectedEnvironments, err = executionscommon.FindEnvironments(octopus, options.Environments)
				if err != nil {
					return err
				}
			}

			options.RunTargets, err = askRunbookTargets(octopus, asker, space.ID, selectedSnapshot.ID, selectedEnvironments)
			if err != nil {
				return err
			}
		}
	}
	// DONE
	return nil
}

func AskGitRunbookRunQuestions(octopus *octopusApiClient.Client, stdout io.Writer, asker question.Asker, space *spaces.Space, project *projects.Project, options *executor.TaskOptionsGitRunbookRun, now func() time.Time) error {
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

	if options.GitReference == "" { // we need a git ref; ask for one
		gitRef, err := selectGitReference(octopus, asker, project)
		if err != nil {
			return err
		}
		options.GitReference = gitRef.CanonicalName // e.g /refs/heads/main
	} else {
		// we need to go lookup the git reference
		_, _ = fmt.Fprintf(stdout, "Git Reference %s\n", output.Cyan(options.GitReference))
	}

	// select the runbook

	var selectedRunbook *runbooks.Runbook
	if options.RunbookName == "" {
		selectedRunbook, err = selectGitRunbook(octopus, asker, "Select a runbook to run", space, project, options.GitReference)
		if err != nil {
			return err
		}
	} else {
		selectedRunbook, err = findGitRunbook(octopus, space.ID, project.ID, options.RunbookName, options.GitReference)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "Runbook %s\n", output.Cyan(selectedRunbook.Name))
	}
	options.RunbookName = selectedRunbook.Name
	if err != nil {
		return err
	}

	// machine selection later on needs to refer back to the environments.
	var selectedEnvironments []*environments.Environment
	if len(options.Environments) == 0 {
		selectedEnvironments, err = selectGitRunEnvironments(asker, octopus, space, project, selectedRunbook, options.GitReference)
		if err != nil {
			return err
		}
		options.Environments = util.SliceTransform(selectedEnvironments, func(env *environments.Environment) string { return env.Name })
	} else {
		_, _ = fmt.Fprintf(stdout, "Environments %s\n", output.Cyan(strings.Join(options.Environments, ",")))
	}

	// ask for tenants and/or tags unless some were specified on the command line
	if len(options.Tenants) == 0 && len(options.TenantTags) == 0 {
		tenantedDeploymentMode := false
		if project.TenantedDeploymentMode == core.TenantedDeploymentModeTenanted {
			tenantedDeploymentMode = true
		}
		options.Tenants, options.TenantTags, _ = executionscommon.AskTenantsAndTags(asker, octopus, selectedRunbook.ProjectID, selectedEnvironments, tenantedDeploymentMode)
	} else {
		if len(options.Tenants) > 0 {
			_, _ = fmt.Fprintf(stdout, "Tenants %s\n", output.Cyan(strings.Join(options.Tenants, ",")))
		}
		if len(options.TenantTags) > 0 {
			_, _ = fmt.Fprintf(stdout, "Tenant Tags %s\n", output.Cyan(strings.Join(options.TenantTags, ",")))
		}
	}

	// TODO: Packages, Git resources
	runbookSnapshotTemplate, err := runbooks.GetGitRunbookSnapshotTemplate(octopus, space.ID, project.ID, selectedRunbook.ID, options.GitReference)
	if err != nil {
		return err
	}

	packageVersionBaseline, err := BuildPackageVersionBaseline(octopus, runbookSnapshotTemplate)
	if err != nil {
		return err
	}

	if len(packageVersionBaseline) > 0 { // if we have packages, run the package flow
		packageVersionOverrides, err := AskPackageOverrideLoop(
			packageVersionBaseline,
			options.DefaultPackageVersion,
			options.PackageVersionOverrides,
			asker,
			stdout)

		if err != nil {
			return err
		}

		if len(packageVersionOverrides) > 0 {
			options.PackageVersionOverrides = make([]string, 0, len(packageVersionOverrides))
			for _, ov := range packageVersionOverrides {
				options.PackageVersionOverrides = append(options.PackageVersionOverrides, ov.ToPackageOverrideString())
			}
		}
	}

	// gitResourcesBaseline := BuildGitResourcesBaseline(deploymentProcessTemplate)

	// if len(gitResourcesBaseline) > 0 {
	// 	overriddenGitResources, err := AskGitResourceOverrideLoop(
	// 		gitResourcesBaseline,
	// 		options.GitResourceRefs,
	// 		asker,
	// 		stdout)

	// 	if err != nil {
	// 		return err
	// 	}

	// 	if len(overriddenGitResources) > 0 {
	// 		options.GitResourceRefs = make([]string, 0, len(overriddenGitResources))
	// 		for _, ov := range overriddenGitResources {
	// 			options.GitResourceRefs = append(options.GitResourceRefs, ov.ToGitResourceGitRefString())
	// 		}
	// 	}
	// }

	PrintAdvancedSummaryForBase(stdout, &options.TaskOptionsRunbookRunBase)

	isRunAtSpecified := options.ScheduledStartTime != ""
	isExcludedStepsSpecified := len(options.ExcludedSteps) > 0
	isGuidedFailureModeSpecified := options.GuidedFailureMode != ""
	isForcePackageDownloadSpecified := options.ForcePackageDownloadWasSpecified
	isRunTargetsSpecified := len(options.RunTargets) > 0 || len(options.ExcludeTargets) > 0

	allAdvancedOptionsSpecified := isRunAtSpecified && isExcludedStepsSpecified && isGuidedFailureModeSpecified && isForcePackageDownloadSpecified && isRunTargetsSpecified

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
		if !isRunAtSpecified {
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
					Help:        "At the start time, the run will be queued. If it does not begin before 'expiry' time, it will be cancelled. Minimum of 5 minutes after start time",
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
			runbookProcess, err := runbooks.GetProcessGit(octopus, space.ID, project.ID, selectedRunbook.ID, options.GitReference)
			if err != nil {
				return err
			}
			options.ExcludedSteps, err = executionscommon.AskExcludedSteps(asker, runbookProcess.Steps)
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

		if !isRunTargetsSpecified {
			if len(selectedEnvironments) == 0 { // if the Q&A process earlier hasn't loaded environments already, we need to load them now
				selectedEnvironments, err = executionscommon.FindEnvironments(octopus, options.Environments)
				if err != nil {
					return err
				}
			}

			options.RunTargets, err = askGitRunbookTargets(octopus, asker, space.ID, project.ID, selectedRunbook.ID, options.GitReference, selectedEnvironments)
			if err != nil {
				return err
			}
		}
	}
	// DONE
	return nil
}

// ToPackageOverrideString converts the struct back into a string which the server can parse e.g. StepName:Version.
// This is the inverse of ParsePackageOverrideString
func (p *PackageVersionOverride) ToPackageOverrideString() string {
	components := make([]string, 0, 3)

	// stepNameOrPackageID always comes first if we have it
	if p.PackageID != "" {
		components = append(components, p.PackageID)
	} else if p.ActionName != "" { // can't have both PackageID and ActionName; PackageID wins
		components = append(components, p.ActionName)
	}

	// followed by package reference name if we have it
	if p.PackageReferenceName != "" {
		if len(components) == 0 { // if we have an explicit packagereference but no packageId or action, we need to express it with *:ref:version
			components = append(components, "*")
		}
		components = append(components, p.PackageReferenceName)
	}

	if len(components) == 0 { // the server can't deal with just a number by itself; if we want to override everything we must pass *:Version
		components = append(components, "*")
	}
	components = append(components, p.Version)

	return strings.Join(components, ":")
}

type StepPackageVersion struct {
	// these 3 fields are the main ones for showing the user
	PackageID  string
	ActionName string // "StepName is an obsolete alias for ActionName, they always contain the same value"
	Version    string // note this may be an empty string, indicating that no version could be found for this package yet

	// used to locate the deployment process VersioningStrategy Donor Package
	PackageReferenceName string
}

// BuildPackageVersionBaseline loads the deployment process template from the server, and for each step+package therein,
// finds the latest available version satisfying the channel version rules. Result is the list of step+package+versions
// to use as a baseline. The package version override process takes this as an input and layers on top of it
func BuildPackageVersionBaseline(octopus *octopusApiClient.Client, runbookProcessTemplate *runbooks.RunbookSnapshotTemplate) ([]*StepPackageVersion, error) {
	result := make([]*StepPackageVersion, 0, len(runbookProcessTemplate.Packages))

	// step 1: pass over all the packages in the deployment process, group them
	// by their feed, then subgroup by packageId

	// map(key: FeedID, value: list of references using the package so we can trace back to steps)
	feedsToQuery := make(map[string][]releases.ReleaseTemplatePackage)
	for _, pkg := range runbookProcessTemplate.Packages {

		if pkg.FixedVersion != "" {
			// If a package has a fixed version it shouldn't be displayed or overridable at all
			continue
		}

		// If a package is not considered resolvable by the server, don't attempt to query it's feed or lookup
		// any potential versions for it; we can't succeed in that because variable templates won't get expanded
		// until deployment time
		if !pkg.IsResolvable {
			result = append(result, &StepPackageVersion{
				PackageID:            pkg.PackageID,
				ActionName:           pkg.ActionName,
				PackageReferenceName: pkg.PackageReferenceName,
				Version:              "",
			})
			continue
		}
		if feedPackages, seenFeedBefore := feedsToQuery[pkg.FeedID]; !seenFeedBefore {
			feedsToQuery[pkg.FeedID] = []releases.ReleaseTemplatePackage{*pkg}
		} else {
			// seen both the feed and package, but not against this particular step
			feedsToQuery[pkg.FeedID] = append(feedPackages, *pkg)
		}
	}

	if len(feedsToQuery) == 0 {
		return make([]*StepPackageVersion, 0), nil
	}

	// step 2: load the feed resources, so we can get SearchPackageVersionsTemplate
	feedIds := make([]string, 0, len(feedsToQuery))
	for k := range feedsToQuery {
		feedIds = append(feedIds, k)
	}
	sort.Strings(feedIds) // we need to sort them otherwise the order is indeterminate. Server doesn't care but our unit tests fail
	foundFeeds, err := octopus.Feeds.Get(feeds.FeedsQuery{IDs: feedIds, Take: len(feedIds)})
	if err != nil {
		return nil, err
	}

	// step 3: for each package within a feed, ask the server to select the best package version for it, applying the channel rules
	for _, feed := range foundFeeds.Items {
		packageRefsInFeed, ok := feedsToQuery[feed.GetID()]
		if !ok {
			return nil, errors.New("internal consistency error; feed ID not found in feedsToQuery") // should never happen
		}

		cache := make(map[feeds.SearchPackageVersionsQuery]string) // cache value is the package version

		for _, packageRef := range packageRefsInFeed {
			query := feeds.SearchPackageVersionsQuery{
				PackageID: packageRef.PackageID,
				Take:      1,
			}

			if cachedVersion, ok := cache[query]; ok {
				result = append(result, &StepPackageVersion{
					PackageID:            packageRef.PackageID,
					ActionName:           packageRef.ActionName,
					PackageReferenceName: packageRef.PackageReferenceName,
					Version:              cachedVersion,
				})
			} else { // uncached; ask the server
				versions, err := octopus.Feeds.SearchFeedPackageVersions(feed, query)
				if err != nil {
					return nil, err
				}

				switch len(versions.Items) {
				case 0: // no package found; cache the response
					cache[query] = ""
					result = append(result, &StepPackageVersion{
						PackageID:            packageRef.PackageID,
						ActionName:           packageRef.ActionName,
						PackageReferenceName: packageRef.PackageReferenceName,
						Version:              "",
					})

				case 1:
					cache[query] = versions.Items[0].Version
					result = append(result, &StepPackageVersion{
						PackageID:            packageRef.PackageID,
						ActionName:           packageRef.ActionName,
						PackageReferenceName: packageRef.PackageReferenceName,
						Version:              versions.Items[0].Version,
					})

				default:
					return nil, errors.New("internal error; more than one package returned when only 1 specified")
				}
			}
		}
	}
	return result, nil
}

type PackageVersionOverride struct {
	ActionName           string // optional, but one or both of ActionName or PackageID must be supplied
	PackageID            string // optional, but one or both of ActionName or PackageID must be supplied
	PackageReferenceName string // optional; use for advanced situations where the same package is referenced multiple times by a single step
	Version              string // required
}

// AmbiguousPackageVersionOverride tells us that we want to set the version of some package to `Version`
// but it's not clear whether ActionNameOrPackageID refers to an ActionName or PackageID at this point
type AmbiguousPackageVersionOverride struct {
	ActionNameOrPackageID string
	PackageReferenceName  string
	Version               string
}

// ParsePackageOverrideString parses a package version override string into a structure.
// Logic should align with PackageVersionResolver in the Octopus Server and .NET CLI
// In cases where things are ambiguous, we look in steps for matching values to see if something is a PackageID or a StepName
func ParsePackageOverrideString(packageOverride string) (*AmbiguousPackageVersionOverride, error) {
	if packageOverride == "" {
		return nil, errors.New("empty package version specification")
	}

	components := splitPackageOverrideString(packageOverride)
	packageReferenceName, stepNameOrPackageID, version := "", "", ""

	switch len(components) {
	case 2:
		// if there are two components it is (StepName|PackageID):Version
		stepNameOrPackageID, version = strings.TrimSpace(components[0]), strings.TrimSpace(components[1])
	case 3:
		// if there are three components it is (StepName|PackageID):PackageReferenceName:Version
		stepNameOrPackageID, packageReferenceName, version = strings.TrimSpace(components[0]), strings.TrimSpace(components[1]), strings.TrimSpace(components[2])
	default:
		return nil, fmt.Errorf("package version specification \"%s\" does not use expected format", packageOverride)
	}

	// must always specify a version; must specify either packageID, stepName or both
	if version == "" {
		return nil, fmt.Errorf("package version specification \"%s\" does not use expected format", packageOverride)
	}
	if !isValidVersion(version) {
		return nil, fmt.Errorf("version component \"%s\" is not a valid version", version)
	}

	// compensate for wildcards
	if packageReferenceName == "*" {
		packageReferenceName = ""
	}
	if stepNameOrPackageID == "*" {
		stepNameOrPackageID = ""
	}

	return &AmbiguousPackageVersionOverride{
		ActionNameOrPackageID: stepNameOrPackageID,
		PackageReferenceName:  packageReferenceName,
		Version:               version,
	}, nil
}

// splitString splits the input string into components based on delimiter characters.
// we want to pick up empty entries here; so "::5" and ":pterm:5" should both return THREE components, rather than one or two
// and we want to allow for multiple different delimeters.
// neither the builtin golang strings.Split or strings.FieldsFunc support this. Logic borrowed from strings.FieldsFunc with heavy modifications
func splitString(s string, delimiters []rune) []string {
	// pass 1: collect spans; golang strings.FieldsFunc says it's much more efficient this way
	type span struct {
		start int
		end   int
	}
	spans := make([]span, 0, 3)

	// Find the field start and end indices.
	start := 0 // we always start the first span at the beginning of the string
	for idx, ch := range s {
		if slices.Contains(delimiters, ch) {
			if start >= 0 { // we found a delimiter and we are already in a span; end the span and start a new one
				spans = append(spans, span{start, idx})
				start = idx + 1
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

// taken from here https://github.com/OctopusDeploy/Versioning/blob/main/source/Octopus.Versioning/Octopus/OctopusVersionParser.cs#L29
// but simplified, and removed the support for optional whitespace around version numbers (OctopusVersion would allow "1 . 2 . 3" whereas we won't
// otherwise this is very lenient
var validVersionRegex, _ = regexp.Compile("(?i)" + `^\s*(v|V)?\d+(\.\d+)?(\.\d+)?(\.\d+)?[.\-_\\]?([a-z0-9]*?)([.\-_\\]([a-z0-9.\-_\\]*?)?)?(\+([a-z0-9_\-.\\+]*?))?$`)

func isValidVersion(version string) bool {
	return validVersionRegex.MatchString(version)
}

// splitPackageOverrideString splits the input string into components based on delimiter characters.
// we want to pick up empty entries here; so "::5" and ":pterm:5" should both return THREE components, rather than one or two
// and we want to allow for multiple different delimeters.
// neither the builtin golang strings.Split or strings.FieldsFunc support this. Logic borrowed from strings.FieldsFunc with heavy modifications
func splitPackageOverrideString(s string) []string {
	return splitString(s, []int32{':', '/', '='})
}

// Note this always uses the Table Printer, it pays no respect to outputformat=json, because it's only part of the interactive flow
func printPackageVersions(ioWriter io.Writer, packages []*StepPackageVersion) error {
	// step 1: consolidate multiple rows
	consolidated := make([]*StepPackageVersion, 0, len(packages))
	for _, pkg := range packages {

		// the common case is that packageReferenceName will be equal to PackageID.
		// however, in advanced cases it may not be, so we need to do extra work to show the packageReferenceName too.
		// we suffix it onto the step name, following the web UI
		qualifiedPkgActionName := pkg.ActionName
		if pkg.PackageID != pkg.PackageReferenceName {
			//qualifiedPkgActionName = fmt.Sprintf("%s%s", qualifiedPkgActionName, output.Yellowf("/%s", pkg.PackageReferenceName))
			qualifiedPkgActionName = fmt.Sprintf("%s/%s", qualifiedPkgActionName, pkg.PackageReferenceName)
		} else {
			qualifiedPkgActionName = fmt.Sprintf("%s%s", qualifiedPkgActionName, output.Dimf("/%s", pkg.PackageReferenceName))
		}

		// find existing entry and insert row below it
		updatedExisting := false
		for index, entry := range consolidated {
			if entry.PackageID == pkg.PackageID && entry.Version == pkg.Version {
				consolidated = append(consolidated[:index+2], consolidated[index+1:]...)
				consolidated[index+1] = &StepPackageVersion{
					PackageID:  output.Dim(pkg.PackageID),
					Version:    output.Dim(pkg.Version),
					ActionName: qualifiedPkgActionName,
				}
				updatedExisting = true
				break
			}
		}
		if !updatedExisting {
			consolidated = append(consolidated, &StepPackageVersion{
				PackageID:  pkg.PackageID,
				Version:    pkg.Version,
				ActionName: qualifiedPkgActionName,
			})
		}
	}

	// step 2: print them
	t := output.NewTable(ioWriter)
	t.AddRow(
		output.Bold("PACKAGE"),
		output.Bold("VERSION"),
		output.Bold("STEP NAME/PACKAGE REFERENCE"),
	)
	//t.AddRow(
	//	"-------",
	//	"-------",
	//	"---------------------------",
	//)

	for _, pkg := range consolidated {
		version := pkg.Version
		if version == "" {
			version = output.Yellow("unknown") // can't determine version for this package
		}
		t.AddRow(
			pkg.PackageID,
			version,
			pkg.ActionName,
		)
	}

	return t.Print()
}

func AskPackageOverrideLoop(
	packageVersionBaseline []*StepPackageVersion,
	defaultPackageVersion string, // the --package-version command line flag
	initialPackageOverrideFlags []string, // the --package command line flag (multiple occurrences)
	asker question.Asker,
	stdout io.Writer) ([]*PackageVersionOverride, error) {
	packageVersionOverrides := make([]*PackageVersionOverride, 0)

	// pickup any partial package specifications that may have arrived on the commandline
	if defaultPackageVersion != "" {
		// blind apply to everything
		packageVersionOverrides = append(packageVersionOverrides, &PackageVersionOverride{Version: defaultPackageVersion})
	}

	for _, s := range initialPackageOverrideFlags {
		ambOverride, err := ParsePackageOverrideString(s)
		if err != nil {
			continue // silently ignore anything that wasn't parseable (should we emit a warning?)
		}
		resolvedOverride, err := ResolvePackageOverride(ambOverride, packageVersionBaseline)
		if err != nil {
			continue // silently ignore anything that wasn't parseable (should we emit a warning?)
		}
		packageVersionOverrides = append(packageVersionOverrides, resolvedOverride)
	}

	overriddenPackageVersions := ApplyPackageOverrides(packageVersionBaseline, packageVersionOverrides)

outerLoop:
	for {
		err := printPackageVersions(stdout, overriddenPackageVersions)
		if err != nil {
			return nil, err
		}

		// While there are any unresolved package versions, force those
		for _, pkgVersionEntry := range overriddenPackageVersions {
			if strings.TrimSpace(pkgVersionEntry.Version) == "" {

				var answer = ""
				for strings.TrimSpace(answer) == "" { // if they enter a blank line just ask again repeatedly
					err = asker(&survey.Input{
						Message: output.Yellowf("Unable to find a version for \"%s\". Specify a version:", pkgVersionEntry.PackageID),
					}, &answer, survey.WithValidator(func(ans interface{}) error {
						str, ok := ans.(string)
						if !ok {
							return errors.New("internal error; answer was not a string")
						}
						if !isValidVersion(str) {
							return fmt.Errorf("\"%s\" is not a valid version", str)
						}
						return nil
					}))

					if err != nil {
						return nil, err
					}
				}

				override := &PackageVersionOverride{Version: answer, ActionName: pkgVersionEntry.ActionName, PackageReferenceName: pkgVersionEntry.PackageReferenceName}
				if override != nil {
					packageVersionOverrides = append(packageVersionOverrides, override)
					overriddenPackageVersions = ApplyPackageOverrides(packageVersionBaseline, packageVersionOverrides)
				}
				continue outerLoop
			}
		}

		// After all packages have versions attached, we can let people freely tweak things until they're happy

		// side-channel return value from the validator
		var resolvedOverride *PackageVersionOverride = nil
		var answer = ""
		err = asker(&survey.Input{
			Message: "Package override string (y to accept, u to undo, ? for help):",
		}, &answer, survey.WithValidator(func(ans interface{}) error {
			str, ok := ans.(string)
			if !ok {
				return errors.New("internal error; answer was not a string")
			}

			switch str {
			// valid response for continuing the loop; don't attempt to validate these
			case "y", "u", "r", "?", "":
				return nil
			}

			ambOverride, err := ParsePackageOverrideString(str)
			if err != nil {
				return err
			}
			resolvedOverride, err = ResolvePackageOverride(ambOverride, packageVersionBaseline)
			if err != nil {
				return err
			}

			return nil // good!
		}))

		// if validators return an error, survey retries itself; the errors don't end up at this level.
		if err != nil {
			return nil, err
		}

		switch answer {
		case "y": // YES these are the packages they want
			break outerLoop
		case "?": // help text
			_, _ = fmt.Fprintf(stdout, output.FormatDoc(packageOverrideLoopHelpText))
		case "u": // undo!
			if len(packageVersionOverrides) > 0 {
				packageVersionOverrides = packageVersionOverrides[:len(packageVersionOverrides)-1]
				// always reset to the baseline and apply everything in order, there's less room for logic errors
				overriddenPackageVersions = ApplyPackageOverrides(packageVersionBaseline, packageVersionOverrides)
			}
		case "r": // reset! All the way back to the calculated versions, discarding even the stuff that came in from the cmdline
			if len(packageVersionOverrides) > 0 {
				packageVersionOverrides = make([]*PackageVersionOverride, 0)
				overriddenPackageVersions = ApplyPackageOverrides(packageVersionBaseline, packageVersionOverrides)
			}
		default:
			if resolvedOverride != nil {
				packageVersionOverrides = append(packageVersionOverrides, resolvedOverride)
				// always reset to the baseline and apply everything in order, there's less room for logic errors
				overriddenPackageVersions = ApplyPackageOverrides(packageVersionBaseline, packageVersionOverrides)
			}
		}
		// loop around and let them put in more input
	}
	return packageVersionOverrides, nil
}

var packageOverrideLoopHelpText = heredoc.Doc(`
bold(PACKAGE SELECTION)
 This screen presents the list of packages used by your project, and the steps
 which reference them. 
 If an item is dimmed (gray text) this indicates that the attribute is duplicated.
 For example if you reference the same package in two steps, the second will be dimmed. 

bold(COMMANDS)
 Any any point, you can enter one of the following:
 - green(?) to access this help screen
 - green(y) to accept the list of packages and proceed with creating the release
 - green(u) to undo the last edit you made to package versions
 - green(r) to reset all package version edits
 - A package override string.

bold(PACKAGE OVERRIDE STRINGS)
 Package override strings must have 2 or 3 components, separated by a :
 The last component must always be a version number.
 
 When specifying 2 components, the first component is either a Package ID or a Step Name.
 You can also specify a * which will match all packages
 Examples:
   bold(octopustools:9.1)   dim(# sets package 'octopustools' in all steps to v 9.1)
   bold(Push Package:3.0)   dim(# sets all packages in the 'Push Package' step to v 3.0)
   bold(*:5.1)              dim(# sets all packages in all steps to v 5.1)

 The 3-component syntax is for advanced use cases where you reference the same package twice
 in a single step, and need to distinguish between the two.
 The syntax is bold(packageIDorStepName:packageReferenceName:version)
 Please refer to the octopus server documentation for more information regarding package reference names. 

dim(---------------------------------------------------------------------)
`) // note this expects to have prettifyHelp run over it

func ResolvePackageOverride(override *AmbiguousPackageVersionOverride, steps []*StepPackageVersion) (*PackageVersionOverride, error) {
	// shortcut for wildcard matches; these match everything so we don't need to do any work
	if override.PackageReferenceName == "" && override.ActionNameOrPackageID == "" {
		return &PackageVersionOverride{
			ActionName:           "",
			PackageID:            "",
			PackageReferenceName: "",
			Version:              override.Version,
		}, nil
	}

	actionNameOrPackageID := override.ActionNameOrPackageID

	// it could be either a stepname or a package ID; match against the list of packages to try and guess.
	// logic matching the server:
	//  - exact match on stepName + refName
	//  - then exact match on packageId + refName
	//  - then match on * + refName
	//  - then match on stepName + *
	//  - then match on packageID + *
	type match struct {
		priority             int
		actionName           string // if set we matched on actionName, else we didn't
		packageID            string // if set we matched on packageID, else we didn't
		packageReferenceName string // if set we matched on packageReferenceName, else we didn't
	}

	matches := make([]match, 0, 2) // common case is likely to be 2; if we have a packageID then we may match both exactly and partially on the ID depending on referenceName
	for _, p := range steps {
		if p.ActionName != "" && p.ActionName == actionNameOrPackageID {
			if p.PackageReferenceName == override.PackageReferenceName {
				matches = append(matches, match{priority: 100, actionName: p.ActionName, packageReferenceName: p.PackageReferenceName})
			} else {
				matches = append(matches, match{priority: 50, actionName: p.ActionName})
			}
		} else if p.PackageID != "" && p.PackageID == actionNameOrPackageID {
			if p.PackageReferenceName == override.PackageReferenceName {
				matches = append(matches, match{priority: 90, packageID: p.PackageID, packageReferenceName: p.PackageReferenceName})
			} else {
				matches = append(matches, match{priority: 40, packageID: p.PackageID})
			}
		} else if p.PackageReferenceName != "" && p.PackageReferenceName == override.PackageReferenceName {
			matches = append(matches, match{priority: 80, packageReferenceName: p.PackageReferenceName})
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("could not resolve step name or package matching %s", actionNameOrPackageID)
	}
	sort.SliceStable(matches, func(i, j int) bool { // want a stable sort so if there's more than one possible match we pick the first one
		return matches[i].priority > matches[j].priority
	})

	return &PackageVersionOverride{
		ActionName:           matches[0].actionName,
		PackageID:            matches[0].packageID,
		PackageReferenceName: matches[0].packageReferenceName,
		Version:              override.Version,
	}, nil
}

func ApplyPackageOverrides(packages []*StepPackageVersion, overrides []*PackageVersionOverride) []*StepPackageVersion {
	for _, o := range overrides {
		packages = applyPackageOverride(packages, o)
	}
	return packages
}

func applyPackageOverride(packages []*StepPackageVersion, override *PackageVersionOverride) []*StepPackageVersion {
	if override.Version == "" {
		return packages // not specifying a version is technically an error, but we'll just no-op it for safety; should have been filtered out by ParsePackageOverrideString before we get here
	}

	var matcher func(pkg *StepPackageVersion) bool = nil

	switch {
	case override.PackageID == "" && override.ActionName == "": // match everything
		matcher = func(pkg *StepPackageVersion) bool {
			return true
		}
	case override.PackageID != "" && override.ActionName == "": // match on package ID only
		matcher = func(pkg *StepPackageVersion) bool {
			return pkg.PackageID == override.PackageID
		}
	case override.PackageID == "" && override.ActionName != "": // match on step only
		matcher = func(pkg *StepPackageVersion) bool {
			return pkg.ActionName == override.ActionName
		}
	case override.PackageID != "" && override.ActionName != "": // match on both; shouldn't be possible but let's ensure it works anyway
		matcher = func(pkg *StepPackageVersion) bool {
			return pkg.PackageID == override.PackageID && pkg.ActionName == override.ActionName
		}
	}

	if override.PackageReferenceName != "" { // must also match package reference name
		if matcher == nil {
			matcher = func(pkg *StepPackageVersion) bool {
				return pkg.PackageReferenceName == override.PackageReferenceName
			}
		} else {
			prevMatcher := matcher
			matcher = func(pkg *StepPackageVersion) bool {
				return pkg.PackageReferenceName == override.PackageReferenceName && prevMatcher(pkg)
			}
		}
	}

	if matcher == nil {
		return packages // we can't possibly match against anything; no-op. Should have been filtered out by ParsePackageOverrideString
	}

	result := make([]*StepPackageVersion, len(packages))
	for i, p := range packages {
		if matcher(p) {
			result[i] = &StepPackageVersion{
				PackageID:            p.PackageID,
				ActionName:           p.ActionName,
				PackageReferenceName: p.PackageReferenceName,
				Version:              override.Version, // Important bit
			}
		} else {
			result[i] = p
		}
	}
	return result
}

func selectGitReference(octopus *octopusApiClient.Client, ask question.Asker, project *projects.Project) (*projects.GitReference, error) {
	branches, err := octopus.Projects.GetGitBranches(project)
	if err != nil {
		return nil, err
	}

	tags, err := octopus.Projects.GetGitTags(project)

	if err != nil {
		return nil, err
	}

	allRefs := append(branches, tags...)

	return question.SelectMap(ask, "Select the Git Reference to use", allRefs, func(g *projects.GitReference) string {
		return fmt.Sprintf("%s %s", g.Name, output.Dimf("(%s)", g.Type.Description()))
	})
}

func askRunbookTargets(octopus *octopusApiClient.Client, asker question.Asker, spaceID string, runbookSnapshotID string, selectedEnvironments []*environments.Environment) ([]string, error) {
	var results []string

	for _, env := range selectedEnvironments {
		preview, err := runbooks.GetRunbookSnapshotRunPreview(octopus, spaceID, runbookSnapshotID, env.ID, true)
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
			Message: "Run targets (If none selected, run on all)",
			Options: results,
		}, &selectedDeploymentTargetNames)
		if err != nil {
			return nil, err
		}

		return selectedDeploymentTargetNames, nil
	}
	return nil, nil
}

func askGitRunbookTargets(octopus *octopusApiClient.Client, asker question.Asker, spaceID string, projectID string, runbookID string, gitRef string, selectedEnvironments []*environments.Environment) ([]string, error) {
	var results []string

	for _, env := range selectedEnvironments {
		preview, err := runbooks.GetGitRunbookRunPreview(octopus, spaceID, projectID, runbookID, gitRef, env.ID, true)
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
			Message: "Run targets (If none selected, run on all)",
			Options: results,
		}, &selectedDeploymentTargetNames)
		if err != nil {
			return nil, err
		}

		return selectedDeploymentTargetNames, nil
	}
	return nil, nil
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

// selectRunEnvironments selects multiple environments for use in an untenanted run
func selectGitRunEnvironments(ask question.Asker, octopus *octopusApiClient.Client, space *spaces.Space, project *projects.Project, runbook *runbooks.Runbook, gitRef string) ([]*environments.Environment, error) {
	envs, err := runbooks.ListEnvironmentsGit(octopus, space.ID, project.ID, runbook.ID, gitRef)
	if err != nil {
		return nil, err
	}

	return question.MultiSelectMap(ask, "Select one or more environments", envs, func(p *environments.Environment) string {
		return p.Name
	}, true)
}

func PrintAdvancedSummary(stdout io.Writer, options *executor.TaskOptionsRunbookRun) {
	runAtStr := "Now"
	if options.ScheduledStartTime != "" {
		runAtStr = options.ScheduledStartTime // we assume the server is going to understand this
	}
	skipStepsStr := "None"
	if len(options.ExcludedSteps) > 0 {
		skipStepsStr = strings.Join(options.ExcludedSteps, ",")
	}

	gfmStr := executionscommon.LookupGuidedFailureModeString(options.GuidedFailureMode)

	pkgDownloadStr := executionscommon.LookupPackageDownloadString(!options.ForcePackageDownload)

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
		  Run Targets: cyan(%s)
	`)), runAtStr, skipStepsStr, gfmStr, pkgDownloadStr, runTargetsStr)
}

func PrintAdvancedSummaryForBase(stdout io.Writer, options *executor.TaskOptionsRunbookRunBase) {
	runAtStr := "Now"
	if options.ScheduledStartTime != "" {
		runAtStr = options.ScheduledStartTime // we assume the server is going to understand this
	}
	skipStepsStr := "None"
	if len(options.ExcludedSteps) > 0 {
		skipStepsStr = strings.Join(options.ExcludedSteps, ",")
	}

	gfmStr := executionscommon.LookupGuidedFailureModeString(options.GuidedFailureMode)

	pkgDownloadStr := executionscommon.LookupPackageDownloadString(!options.ForcePackageDownload)

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
		  Run Targets: cyan(%s)
	`)), runAtStr, skipStepsStr, gfmStr, pkgDownloadStr, runTargetsStr)
}

func selectRunbook(octopus *octopusApiClient.Client, ask question.Asker, questionText string, space *spaces.Space, project *projects.Project) (*runbooks.Runbook, error) {
	foundRunbooks, err := runbooks.List(octopus, space.ID, project.ID, "", math.MaxInt32)
	if err != nil {
		return nil, err
	}

	if len(foundRunbooks.Items) == 0 {
		return nil, fmt.Errorf("no runbooks found for selected project: %s", project.Name)
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

// findRunbookSnapshot wraps the API client, such that we are always guaranteed to get a result, or error. The "successfully can't find matching name" case doesn't exist
func findRunbookSnapshot(octopus *octopusApiClient.Client, spaceID string, projectID string, snapshotIDorName string) (*runbooks.RunbookSnapshot, error) {
	result, err := runbooks.GetSnapshot(octopus, spaceID, projectID, snapshotIDorName)
	if result == nil && err == nil {
		return nil, fmt.Errorf("no snapshot found with ID or Name of %s", snapshotIDorName)
	}
	return result, err
}

// findRunbookPublishedSnapshot finds the published snapshot ID. If it cannot be found, an error is returned, you'll never get nil, nil
func findRunbookPublishedSnapshot(octopus *octopusApiClient.Client, space *spaces.Space, project *projects.Project, runbook *runbooks.Runbook) (*runbooks.RunbookSnapshot, error) {
	if runbook.PublishedRunbookSnapshotID == "" {
		return nil, fmt.Errorf("cannot run runbook %s, it has no published snapshot", runbook.Name)
	}
	return findRunbookSnapshot(octopus, space.ID, project.ID, runbook.PublishedRunbookSnapshotID)
}

func selectGitRunbook(octopus *octopusApiClient.Client, ask question.Asker, questionText string, space *spaces.Space, project *projects.Project, gitRef string) (*runbooks.Runbook, error) {
	foundRunbooks, err := runbooks.ListGit(octopus, space.ID, project.ID, gitRef, "", math.MaxInt32)
	if err != nil {
		return nil, err
	}

	if len(foundRunbooks.Items) == 0 {
		return nil, fmt.Errorf("no runbooks found for selected project: %s", project.Name)
	}

	return question.SelectMap(ask, questionText, foundRunbooks.Items, func(p *runbooks.Runbook) string {
		return p.Name
	})
}

// findRunbook wraps the API client, such that we are always guaranteed to get a result, or error. The "successfully can't find matching name" case doesn't exist
func findGitRunbook(octopus *octopusApiClient.Client, spaceID string, projectID string, runbookName string, gitRef string) (*runbooks.Runbook, error) {
	result, err := runbooks.GetByNameGit(octopus, spaceID, projectID, gitRef, runbookName)
	if result == nil && err == nil {
		return nil, fmt.Errorf("no runbook found with Name of %s", runbookName)
	}
	return result, err
}
