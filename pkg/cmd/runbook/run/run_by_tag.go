package run

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/runbook/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/executionscommon"
	"github.com/OctopusDeploy/cli/pkg/executor"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/cobra"
)

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
	}, &selectedTags, survey.WithValidator(survey.Required))
	if err != nil {
		return nil, err
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
	}, &selectedTags, survey.WithValidator(survey.Required))
	if err != nil {
		return nil, err
	}

	return selectedTags, nil
}

type runbookTaskResult struct {
	runbookName           string
	environments          []string
	runbookRunServerTasks []*runbooks.RunbookRunServerTask
	err                   error
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
			runbookName:           runbookName,
			environments:          environments,
			runbookRunServerTasks: serverTasks,
			err:                   err,
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
			cmd.Println(output.Red("X Cannot run multiple runbooks by tag when prompted variables are present."))
			cmd.Printf("Runbook '%s' requires prompted variables.\n\n", runbookWithPrompts)
			cmd.Println("To proceed, you can:")
			cmd.Println("  • Run runbooks individually by name")
			cmd.Println("  • Specify all required variables using --variable flags")
			return fmt.Errorf("prompted variables required")
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

		shouldAskAdvancedQuestions, err := shouldAskAdvancedOptions(f.Ask, "Change additional options? (will apply to all matching runbooks)", allAdvancedOptionsSpecified)
		if err != nil {
			return err
		}

		if shouldAskAdvancedQuestions {
			// Ask common advanced options (schedule, guided failure, package download)
			err = askCommonAdvancedOptions(
				f.Ask,
				now,
				&flags.RunAt.Value,
				&flags.MaxQueueTime.Value,
				&flags.GuidedFailureMode.Value,
				&flags.ForcePackageDownload.Value,
				isRunAtSpecified,
				isGuidedFailureModeSpecified,
				isForcePackageDownloadSpecified,
			)
			if err != nil {
				return err
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
