package run

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
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
	"github.com/OctopusDeploy/cli/pkg/gitresources"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/packages"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
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
	options := &executor.TaskOptionsRunbookRun{
		Snapshot: flags.Snapshot.Value,
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

func convertToValueSlice(slice []*releases.ReleaseTemplatePackage) []releases.ReleaseTemplatePackage {
	values := make([]releases.ReleaseTemplatePackage, len(slice))
	for i, v := range slice {
		values[i] = *v
	}
	return values
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

	PrintAdvancedSummary(stdout, &options.TaskOptionsRunbookRunBase)

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

	packageVersionBaseline, err := packages.BuildPackageVersionBaseline(octopus, convertToValueSlice(runbookSnapshotTemplate.Packages), nil)
	if err != nil {
		return err
	}

	if len(packageVersionBaseline) > 0 { // if we have packages, run the package flow
		_, packageVersionOverrides, err := packages.AskPackageOverrideLoop(
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

	gitResourcesBaseline := gitresources.BuildGitResourcesBaseline(runbookSnapshotTemplate.GitResources)

	if len(gitResourcesBaseline) > 0 {
		overriddenGitResources, err := gitresources.AskGitResourceOverrideLoop(
			gitResourcesBaseline,
			options.GitResourceRefs,
			asker,
			stdout)

		if err != nil {
			return err
		}

		if len(overriddenGitResources) > 0 {
			options.GitResourceRefs = make([]string, 0, len(overriddenGitResources))
			for _, ov := range overriddenGitResources {
				options.GitResourceRefs = append(options.GitResourceRefs, ov.ToGitResourceGitRefString())
			}
		}
	}

	PrintAdvancedSummary(stdout, &options.TaskOptionsRunbookRunBase)

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

func PrintAdvancedSummary(stdout io.Writer, options *executor.TaskOptionsRunbookRunBase) {
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
