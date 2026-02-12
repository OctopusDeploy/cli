package run

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/cmd/runbook/shared"
	"github.com/OctopusDeploy/cli/pkg/packages"
	"golang.org/x/exp/maps"
	"io"
	"math"
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
	"github.com/OctopusDeploy/cli/pkg/gitresources"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/cobra"
)

const (
	FlagProject = "project"

	FlagRunbookName        = "name"
	FlagAliasRunbookLegacy = "runbook"

	FlagRunbookTag = "runbook-tag" // can be specified multiple times

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
	RunbookTags          *flag.Flag[[]string]
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
		RunbookTags:          flag.New[[]string](FlagRunbookTag, false),
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
			$ %[1]s runbook run --project MyProject --runbook "Rebuild DB indexes"
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
	flags.StringArrayVarP(&runFlags.RunbookTags.Value, runFlags.RunbookTags.Name, "", nil, "Run all runbooks matching this tag (can be specified multiple times). Format is 'Tag Set Name/Tag Name'. Mutually exclusive with --name.")
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
	if flags.RunbookName.Value != "" && len(flags.RunbookTags.Value) > 0 {
		return errors.New("--name and --runbook-tag are mutually exclusive. Please specify either a runbook name or runbook tags, not both")
	}

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

	project, err := selectProject(octopus, f, flags.Project.Value)

	if err != nil {
		return err
	}

	flags.Project.Value = project.Name

	if f.IsPromptEnabled() && flags.RunbookName.Value == "" && len(flags.RunbookTags.Value) == 0 {
		var runBySelection string
		err = f.Ask(&survey.Select{
			Message: "How do you want to select which runbook(s) to run?",
			Options: []string{"By name", "By tag"},
		}, &runBySelection)
		if err != nil {
			return err
		}

		if runBySelection == "By tag" {
			if shared.AreRunbooksInGit(project) {
				if flags.GitRef.Value == "" {
					gitRef, err := selectors.GitReference("Select the Git Reference to run for", octopus, f.Ask, project)
					if err != nil {
						return err
					}
					flags.GitRef.Value = gitRef.CanonicalName
				}
				tags, err := selectGitRunbookTags(octopus, f.Ask, f.GetCurrentSpace(), project, flags.GitRef.Value)
				if err != nil {
					return err
				}
				flags.RunbookTags.Value = tags
			} else {
				tags, err := selectRunbookTags(octopus, f.Ask, f.GetCurrentSpace(), project)
				if err != nil {
					return err
				}
				flags.RunbookTags.Value = tags
			}
		}
	}

	if len(flags.RunbookTags.Value) > 0 {
		if shared.AreRunbooksInGit(project) {
			return runRunbooksByTag(cmd, f, flags, octopus, project, parsedVariables, outputFormat, true)
		} else {
			return runRunbooksByTag(cmd, f, flags, octopus, project, parsedVariables, outputFormat, false)
		}
	}

	if shared.AreRunbooksInGit(project) {
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
		GitReference:            flags.GitRef.Value,
		DefaultPackageVersion:   flags.PackageVersion.Value,
		PackageVersionOverrides: flags.PackageVersionSpec.Value,
		GitResourceRefs:         flags.GitResourceRefsSpec.Value,
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

func selectProject(octopus *octopusApiClient.Client, f factory.Factory, projectName string) (*projects.Project, error) {
	if projectName == "" {
		if f.IsPromptEnabled() {
			selectedProject, err := selectors.Project("Select project", octopus, f.Ask)
			if err != nil {
				return nil, err
			}
			return selectedProject, nil
		} else {
			// Project name not provided and not asking questions so error out
			return nil, errors.New("project must be specified")
		}
	} else { // project name is already provided, fetch the object because it's needed for further questions
		selectedProject, err := selectors.FindProject(octopus, projectName)
		if err != nil {
			return nil, err
		}

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

	variables, sensitiveVars, err := askRunbookPreviewVariables(
		octopus,
		asker,
		space,
		project,
		selectedRunbook,
		selectedEnvironments,
		options.Variables,
		false,
		"",
		selectedSnapshot.ID,
	)
	if err != nil {
		return err
	}
	options.Variables = variables
	options.SensitiveVariableNames = sensitiveVars

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
		gitRef, err := selectors.GitReference("Select the Git Reference to run for", octopus, asker, project)
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

	runbookSnapshotTemplate, err := runbooks.GetGitRunbookSnapshotTemplate(octopus, space.ID, project.ID, selectedRunbook.ID, options.GitReference)
	if err != nil {
		return err
	}

	packageVersionBaseline, err := packages.BuildPackageVersionBaseline(octopus, util.SliceTransform(runbookSnapshotTemplate.Packages, func(pkg *releases.ReleaseTemplatePackage) releases.ReleaseTemplatePackage { return *pkg }), nil)
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

	variables, sensitiveVars, err := askRunbookPreviewVariables(
		octopus,
		asker,
		space,
		project,
		selectedRunbook,
		selectedEnvironments,
		options.Variables,
		true,
		options.GitReference,
		"",
	)
	if err != nil {
		return err
	}
	options.Variables = variables
	options.SensitiveVariableNames = sensitiveVars

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
			runbookProcess, err := runbooks.GetGitRunbookProcess(octopus, space.ID, project.ID, selectedRunbook.ID, options.GitReference)
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

func askRunbookPreviewVariables(
	octopus *octopusApiClient.Client,
	asker question.Asker,
	space *spaces.Space,
	project *projects.Project,
	runbook *runbooks.Runbook,
	selectedEnvironments []*environments.Environment,
	variablesFromCmd map[string]string,
	isGitBased bool,
	gitRef string,
	snapshotID string,
) (map[string]string, []string, error) {
	// Get previews for each environment to determine required variables
	var previews []*runbooks.RunPreview
	for _, environment := range selectedEnvironments {
		var preview *runbooks.RunPreview
		var err error

		if isGitBased {
			preview, err = runbooks.GetGitRunbookRunPreview(octopus, space.ID, project.ID, runbook.ID, gitRef, environment.ID, true)
		} else {
			preview, err = runbooks.GetRunbookSnapshotRunPreview(octopus, space.ID, snapshotID, environment.ID, true)
		}
		if err != nil {
			return nil, nil, err
		}
		previews = append(previews, preview)
	}

	// Build a map of variable names to their values and controls
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

	// Process variables from command line and prompts
	result := make(map[string]string)
	lcaseVarsFromCmd := make(map[string]string, len(variablesFromCmd))
	for k, v := range variablesFromCmd {
		lcaseVarsFromCmd[strings.ToLower(k)] = v
	}

	keys := maps.Keys(flattenedControls)
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] > keys[j]
	})

	// Track sensitive variables
	sensitiveVars := make([]string, 0)

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
				promptMessage = fmt.Sprintf("%s (%s)", promptMessage, control.Description)
			}

			responseString, err := executionscommon.AskVariableSpecificPrompt(asker, promptMessage, control.Type, defaultValue, control.Required, isSensitive, control.DisplaySettings)
			if err != nil {
				return nil, nil, err
			}
			result[control.Name] = responseString
		}

		// Track sensitive variables from the preview
		if control.DisplaySettings.ControlType == "Sensitive" {
			sensitiveVars = append(sensitiveVars, control.Name)
		}
	}

	return result, sensitiveVars, nil
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
	envs, err := runbooks.ListEnvironmentsForGitRunbook(octopus, space.ID, project.ID, runbook.ID, gitRef)
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
	foundRunbooks, err := runbooks.ListGitRunbooks(octopus, space.ID, project.ID, gitRef, "", math.MaxInt32)
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
	result, err := runbooks.GetGitRunbookByName(octopus, spaceID, projectID, gitRef, runbookName)
	if result == nil && err == nil {
		return nil, fmt.Errorf("no runbook found with Name of %s", runbookName)
	}
	return result, err
}

func filterRunbooksByTags(allRunbooks []*runbooks.Runbook, tags []string) []*runbooks.Runbook {
	var matchingRunbooks []*runbooks.Runbook
	for _, runbook := range allRunbooks {
		for _, tag := range tags {
			if util.SliceContains(runbook.RunbookTags, tag) {
				matchingRunbooks = append(matchingRunbooks, runbook)
				break
			}
		}
	}
	return matchingRunbooks
}

func selectRunbookTags(octopus *octopusApiClient.Client, asker question.Asker, space *spaces.Space, project *projects.Project) ([]string, error) {
	allRunbooks, err := shared.GetAllRunbooks(octopus, project.ID)
	if err != nil {
		return nil, err
	}

	tagMap := make(map[string]bool) // deduplicate tags across all runbooks
	for _, runbook := range allRunbooks {
		for _, tag := range runbook.RunbookTags {
			tagMap[tag] = true
		}
	}

	if len(tagMap) == 0 {
		return nil, fmt.Errorf("no runbooks with tags found in project %s", project.Name)
	}

	availableTags := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		availableTags = append(availableTags, tag)
	}
	sort.Strings(availableTags)

	var selectedTags []string
	err = asker(&survey.MultiSelect{
		Message: "Select runbook tags (space to select, enter to confirm):",
		Options: availableTags,
	}, &selectedTags)
	if err != nil {
		return nil, err
	}

	if len(selectedTags) == 0 {
		return nil, fmt.Errorf("at least one tag must be selected")
	}

	return selectedTags, nil
}

func selectGitRunbookTags(octopus *octopusApiClient.Client, asker question.Asker, space *spaces.Space, project *projects.Project, gitRef string) ([]string, error) {
	allRunbooks, err := shared.GetAllGitRunbooks(octopus, project.ID, gitRef)
	if err != nil {
		return nil, err
	}

	tagMap := make(map[string]bool) // deduplicate tags across all runbooks
	for _, runbook := range allRunbooks {
		for _, tag := range runbook.RunbookTags {
			tagMap[tag] = true
		}
	}

	if len(tagMap) == 0 {
		return nil, fmt.Errorf("no runbooks with tags found in project %s", project.Name)
	}

	availableTags := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		availableTags = append(availableTags, tag)
	}
	sort.Strings(availableTags)

	var selectedTags []string
	err = asker(&survey.MultiSelect{
		Message: "Select runbook tags (space to select, enter to confirm):",
		Options: availableTags,
	}, &selectedTags)
	if err != nil {
		return nil, err
	}

	if len(selectedTags) == 0 {
		return nil, fmt.Errorf("at least one tag must be selected")
	}

	return selectedTags, nil
}

type runbookTaskResult struct {
	runbookName          string
	environments         []string
	runbookRunServerTasks []*runbooks.RunbookRunServerTask
	err                  error
}

func processRunbookTasks(octopus *octopusApiClient.Client, space *spaces.Space, tasks []*executor.Task) []runbookTaskResult {
	results := make([]runbookTaskResult, len(tasks))

	for i, task := range tasks {
		var runbookName string
		var environments []string
		var serverTasks []*runbooks.RunbookRunServerTask
		var err error

		switch task.Type {
		case executor.TaskTypeRunbookRun:
			params, ok := task.Options.(*executor.TaskOptionsRunbookRun)
			if ok {
				runbookName = params.RunbookName
				environments = params.Environments
				err = executor.ProcessTasks(octopus, space, []*executor.Task{task})
				if params.Response != nil {
					serverTasks = params.Response.RunbookRunServerTasks
				}
			} else {
				err = fmt.Errorf("invalid task options type for RunbookRun")
			}
		case executor.TaskTypeGitRunbookRun:
			params, ok := task.Options.(*executor.TaskOptionsGitRunbookRun)
			if ok {
				runbookName = params.RunbookName
				environments = params.Environments
				err = executor.ProcessTasks(octopus, space, []*executor.Task{task})
				if params.Response != nil {
					serverTasks = params.Response.RunbookRunServerTasks
				}
			} else {
				err = fmt.Errorf("invalid task options type for GitRunbookRun")
			}
		default:
			err = fmt.Errorf("unhandled task type %s", task.Type)
		}

		results[i] = runbookTaskResult{
			runbookName:          runbookName,
			environments:         environments,
			runbookRunServerTasks: serverTasks,
			err:                  err,
		}
	}

	return results
}

func runRunbooksByTag(cmd *cobra.Command, f factory.Factory, flags *RunFlags, octopus *octopusApiClient.Client, project *projects.Project, parsedVariables map[string]string, outputFormat string, isGit bool) error {
	var allRunbooks []*runbooks.Runbook
	var err error

	if isGit {
		if flags.GitRef.Value == "" {
			return errors.New("--git-ref is required when running runbooks by tag in a git-based project")
		}
		allRunbooks, err = shared.GetAllGitRunbooks(octopus, project.ID, flags.GitRef.Value)
	} else {
		allRunbooks, err = shared.GetAllRunbooks(octopus, project.ID)
	}

	if err != nil {
		return err
	}

	matchingRunbooks := filterRunbooksByTags(allRunbooks, flags.RunbookTags.Value)

	if len(matchingRunbooks) == 0 {
		return fmt.Errorf("no runbooks found matching tags: %s", strings.Join(flags.RunbookTags.Value, ", "))
	}

	if !constants.IsProgrammaticOutputFormat(outputFormat) {
		cmd.Printf("Found %d runbook(s) matching tags:\n", len(matchingRunbooks))
		for _, rb := range matchingRunbooks {
			cmd.Printf("  - %s\n", rb.Name)
		}
		cmd.Println()
	}

	var selectedEnvironments []*environments.Environment
	if f.IsPromptEnabled() {
		if len(flags.Environments.Value) == 0 {
			if isGit {
				selectedEnvironments, err = selectGitRunEnvironments(f.Ask, octopus, f.GetCurrentSpace(), project, matchingRunbooks[0], flags.GitRef.Value)
			} else {
				selectedEnvironments, err = selectRunEnvironments(f.Ask, octopus, f.GetCurrentSpace(), project, matchingRunbooks[0])
			}
			if err != nil {
				return err
			}
			flags.Environments.Value = util.SliceTransform(selectedEnvironments, func(env *environments.Environment) string { return env.Name })
		}

		if len(flags.Tenants.Value) == 0 && len(flags.TenantTags.Value) == 0 {
			tenantedDeploymentMode := false
			if project.TenantedDeploymentMode == core.TenantedDeploymentModeTenanted {
				tenantedDeploymentMode = true
			}
			flags.Tenants.Value, flags.TenantTags.Value, _ = executionscommon.AskTenantsAndTags(f.Ask, octopus, matchingRunbooks[0].ProjectID, selectedEnvironments, tenantedDeploymentMode)
		}
	}

	if len(flags.Environments.Value) == 0 {
		return errors.New("environment(s) must be specified")
	}

	// Check if any runbooks have prompted variables - block execution if found
	if len(parsedVariables) == 0 {
		hasPromptedVars := false
		var runbookWithPrompts string
		for _, runbook := range matchingRunbooks {
			var preview *runbooks.RunPreview
			if isGit {
				// Get preview for first environment to check for prompted variables
				if len(flags.Environments.Value) > 0 {
					envs, err := executionscommon.FindEnvironments(octopus, flags.Environments.Value[:1])
					if err == nil && len(envs) > 0 {
						preview, _ = runbooks.GetGitRunbookRunPreview(octopus, f.GetCurrentSpace().ID, project.ID, runbook.ID, flags.GitRef.Value, envs[0].ID, true)
					}
				}
			} else {
				// For DB runbooks, we need the published snapshot
				if runbook.PublishedRunbookSnapshotID != "" {
					if len(flags.Environments.Value) > 0 {
						envs, err := executionscommon.FindEnvironments(octopus, flags.Environments.Value[:1])
						if err == nil && len(envs) > 0 {
							preview, _ = runbooks.GetRunbookSnapshotRunPreview(octopus, f.GetCurrentSpace().ID, runbook.PublishedRunbookSnapshotID, envs[0].ID, true)
						}
					}
				}
			}
			if preview != nil && len(preview.Form.Elements) > 0 {
				for _, element := range preview.Form.Elements {
					if element.Control.Required {
						hasPromptedVars = true
						runbookWithPrompts = runbook.Name
						break
					}
				}
			}
			if hasPromptedVars {
				break
			}
		}

		if hasPromptedVars {
			return fmt.Errorf("cannot run multiple runbooks by tag when prompted variables are present. Runbook '%s' has required prompted variables. Please run runbooks individually by name, or specify all required variables via --variable flag", runbookWithPrompts)
		}
	}

	// Ask for advanced options that apply to all runbooks
	if f.IsPromptEnabled() {
		now := time.Now
		if cmd.Context() != nil {
			if n, ok := cmd.Context().Value(constants.ContextKeyTimeNow).(func() time.Time); ok {
				now = n
			}
		}

		isRunAtSpecified := flags.RunAt.Value != ""
		isExcludedStepsSpecified := len(flags.ExcludedSteps.Value) > 0
		isGuidedFailureModeSpecified := flags.GuidedFailureMode.Value != ""
		isForcePackageDownloadSpecified := cmd.Flags().Lookup(FlagForcePackageDownload).Changed
		isRunTargetsSpecified := len(flags.RunTargets.Value) > 0 || len(flags.ExcludeTargets.Value) > 0

		allAdvancedOptionsSpecified := isRunAtSpecified && isExcludedStepsSpecified && isGuidedFailureModeSpecified && isForcePackageDownloadSpecified && isRunTargetsSpecified

		shouldAskAdvancedQuestions := false
		if !allAdvancedOptionsSpecified {
			var changeOptionsAnswer string
			err = f.Ask(&survey.Select{
				Message: "Change additional options? (will apply to all matching runbooks)",
				Options: []string{"Proceed to run", "Change"},
			}, &changeOptionsAnswer)
			if err != nil {
				return err
			}
			shouldAskAdvancedQuestions = changeOptionsAnswer == "Change"
		}

		if shouldAskAdvancedQuestions {
			if !isRunAtSpecified {
				referenceNow := now()
				maxSchedStartTime := referenceNow.Add(30 * 24 * time.Hour)

				var answer surveyext.DatePickerAnswer
				err = f.Ask(&surveyext.DatePicker{
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
				if scheduledStartTime.After(referenceNow.Add(1 * time.Minute)) {
					flags.RunAt.Value = scheduledStartTime.Format(time.RFC3339)

					startPlusFiveMin := scheduledStartTime.Add(5 * time.Minute)
					err = f.Ask(&surveyext.DatePicker{
						Message:     "Scheduled expiry time",
						Help:        "At the start time, the run will be queued. If it does not begin before 'expiry' time, it will be cancelled. Minimum of 5 minutes after start time",
						Default:     startPlusFiveMin,
						Min:         startPlusFiveMin,
						Max:         maxSchedStartTime.Add(24 * time.Hour),
						OverrideNow: referenceNow,
					}, &answer)
					if err != nil {
						return err
					}
					flags.MaxQueueTime.Value = answer.Time.Format(time.RFC3339)
				}
			}

			if !isGuidedFailureModeSpecified {
				flags.GuidedFailureMode.Value, err = executionscommon.AskGuidedFailureMode(f.Ask)
				if err != nil {
					return err
				}
			}

			if !isForcePackageDownloadSpecified {
				flags.ForcePackageDownload.Value, err = executionscommon.AskPackageDownload(f.Ask)
				if err != nil {
					return err
				}
			}

			// Note: We skip ExcludedSteps and RunTargets for multi-runbook runs as they may differ per runbook
			// Users can specify these via command line flags if needed
		}
	}

	if f.IsPromptEnabled() && !constants.IsProgrammaticOutputFormat(outputFormat) {
		resolvedFlags := NewRunFlags()
		resolvedFlags.Project.Value = flags.Project.Value
		resolvedFlags.RunbookTags.Value = flags.RunbookTags.Value
		resolvedFlags.Environments.Value = flags.Environments.Value
		resolvedFlags.Tenants.Value = flags.Tenants.Value
		resolvedFlags.TenantTags.Value = flags.TenantTags.Value

		if isGit {
			resolvedFlags.GitRef.Value = flags.GitRef.Value
			autoCmd := flag.GenerateAutomationCmd(constants.ExecutableName+" runbook run",
				resolvedFlags.Project,
				resolvedFlags.RunbookTags,
				resolvedFlags.GitRef,
				resolvedFlags.Environments,
				resolvedFlags.Tenants,
				resolvedFlags.TenantTags,
			)
			cmd.Printf("\nAutomation Command: %s\n", autoCmd)
		} else {
			autoCmd := flag.GenerateAutomationCmd(constants.ExecutableName+" runbook run",
				resolvedFlags.Project,
				resolvedFlags.RunbookTags,
				resolvedFlags.Environments,
				resolvedFlags.Tenants,
				resolvedFlags.TenantTags,
			)
			cmd.Printf("\nAutomation Command: %s\n", autoCmd)
		}
	}

	tasks := make([]*executor.Task, 0, len(matchingRunbooks))
	for _, runbook := range matchingRunbooks {
		commonOptions := &executor.TaskOptionsRunbookRunBase{
			ProjectName:          project.Name,
			RunbookName:          runbook.Name,
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

		if isGit {
			gitOptions := &executor.TaskOptionsGitRunbookRun{
				GitReference:            flags.GitRef.Value,
				DefaultPackageVersion:   flags.PackageVersion.Value,
				PackageVersionOverrides: flags.PackageVersionSpec.Value,
				GitResourceRefs:         flags.GitResourceRefsSpec.Value,
			}
			gitOptions.TaskOptionsRunbookRunBase = *commonOptions
			tasks = append(tasks, executor.NewTask(executor.TaskTypeGitRunbookRun, gitOptions))
		} else {
			dbOptions := &executor.TaskOptionsRunbookRun{
				Snapshot: flags.Snapshot.Value,
			}
			dbOptions.TaskOptionsRunbookRunBase = *commonOptions
			if cmd.Flags().Lookup(FlagForcePackageDownload).Changed {
				dbOptions.ForcePackageDownloadWasSpecified = true
			}
			tasks = append(tasks, executor.NewTask(executor.TaskTypeRunbookRun, dbOptions))
		}
	}

	results := processRunbookTasks(octopus, f.GetCurrentSpace(), tasks)

	type runbookRunResult struct {
		RunbookName string `json:"runbookName"`
		Environment string `json:"environment"`
		Status      string `json:"status"`
		TaskID      string `json:"taskId"`
	}

	var flatResults []runbookRunResult
	successCount := 0
	failCount := 0

	for _, result := range results {
		if result.err != nil {
			failCount++
			for _, env := range result.environments {
				flatResults = append(flatResults, runbookRunResult{
					RunbookName: result.runbookName,
					Environment: env,
					Status:      fmt.Sprintf("Failed: %v", result.err),
					TaskID:      "",
				})
			}
		} else {
			for i, task := range result.runbookRunServerTasks {
				successCount++
				env := "Unknown"
				if i < len(result.environments) {
					env = result.environments[i]
				} else if len(result.environments) > 0 {
					env = result.environments[0]
				}
				flatResults = append(flatResults, runbookRunResult{
					RunbookName: result.runbookName,
					Environment: env,
					Status:      "Started",
					TaskID:      task.ServerTaskID,
				})
			}
		}
	}

	switch outputFormat {
	case constants.OutputFormatBasic:
		for _, result := range flatResults {
			if result.Status == "Started" {
				cmd.Printf("%s\n", result.TaskID)
			}
		}
	case constants.OutputFormatJson:
		data, err := json.Marshal(flatResults)
		if err != nil {
			cmd.PrintErrln(err)
		} else {
			_, _ = cmd.OutOrStdout().Write(data)
			cmd.Println()
		}
	default:
		cmd.Println()
		t := output.NewTable(cmd.OutOrStdout())
		t.AddRow(output.Bold("RUNBOOK"), output.Bold("ENVIRONMENT"), output.Bold("STATUS"), output.Bold("TASK ID"))
		for _, result := range flatResults {
			statusDisplay := result.Status
			if result.Status == "Started" {
				statusDisplay = output.Cyan(result.Status)
			} else {
				statusDisplay = output.Red(result.Status)
			}
			t.AddRow(result.RunbookName, result.Environment, statusDisplay, result.TaskID)
		}
		t.Print()
		cmd.Println()
		cmd.Printf("Successfully started: %d, Failed: %d\n", successCount, failCount)
	}

	if failCount > 0 {
		return fmt.Errorf("%d runbook run(s) failed to start", failCount)
	}

	return nil
}
