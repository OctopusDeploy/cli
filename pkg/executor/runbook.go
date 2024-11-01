package executor

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
)

// ----- Create Release --------------------------------------

type TaskResultRunbookRun struct {
	Version string
}

// the command processor is responsible for accepting related entity names from the end user
// and looking them up for their ID's; we should only deal with strong references at this level

type TaskOptionsRunbookRunBase struct {
	ProjectName          string // required
	RunbookName          string // the name of the runbook to run
	Environments         []string
	Tenants              []string
	TenantTags           []string
	ScheduledStartTime   string
	ScheduledExpiryTime  string
	ExcludedSteps        []string
	GuidedFailureMode    string // ["", "true", "false", "default"]. Note default and "" are the same, the only difference is whether interactive mode prompts you
	ForcePackageDownload bool
	RunTargets           []string
	ExcludeTargets       []string
	Variables            map[string]string

	// extra behaviour commands

	// true if the value was specified on the command line (because ForcePackageDownload is bool, we can't distinguish 'false' from 'missing')
	ForcePackageDownloadWasSpecified bool

	// so the automation command can mask sensitive variable output
	SensitiveVariableNames []string
}

type TaskOptionsRunbookRun struct {
	Snapshot string

	// if the task succeeds, the resulting output will be stored here
	Response *runbooks.RunbookRunResponseV1
	TaskOptionsRunbookRunBase
}

func runbookRun(octopus *client.Client, space *spaces.Space, input any) error {
	params, ok := input.(*TaskOptionsRunbookRun)
	if !ok {
		return errors.New("invalid input type; expecting TaskOptionsRunbookRun")
	}
	if space == nil {
		return errors.New("space must be specified")
	}

	// we have the provided project name; go look it up
	if params.ProjectName == "" {
		return errors.New("project must be specified")
	}
	if params.RunbookName == "" {
		return errors.New("runbook name must be specified")
	}
	if len(params.Environments) == 0 {
		return errors.New("environment(s) must be specified")
	}

	// common properties
	abstractCmd := deployments.CreateExecutionAbstractCommandV1{
		SpaceID:              space.ID,
		ProjectIDOrName:      params.ProjectName,
		ForcePackageDownload: params.ForcePackageDownload,
		SpecificMachineNames: params.RunTargets,
		ExcludedMachineNames: params.ExcludeTargets,
		SkipStepNames:        params.ExcludedSteps,
		RunAt:                params.ScheduledStartTime,
		NoRunAfter:           params.ScheduledExpiryTime,
		Variables:            params.Variables,
	}

	b, err := strconv.ParseBool(params.GuidedFailureMode)
	if err == nil {
		abstractCmd.UseGuidedFailure = &b
	} else {
		// else they must have specified nothing, or perhaps "default". Sanity check it's not garbage
		if params.GuidedFailureMode != "" && !strings.EqualFold("default", params.GuidedFailureMode) {
			return fmt.Errorf("'%s' is not a valid value for guided failure mode", params.GuidedFailureMode)
		}
	}
	runCommand := runbooks.NewRunbookRunCommandV1(space.ID, params.ProjectName)
	runCommand.RunbookName = params.RunbookName
	runCommand.EnvironmentNames = params.Environments
	runCommand.Tenants = params.Tenants
	runCommand.TenantTags = params.TenantTags
	runCommand.Snapshot = params.Snapshot

	runCommand.CreateExecutionAbstractCommandV1 = abstractCmd

	runResponse, err := runbooks.RunbookRunV1(octopus, runCommand)
	if err != nil {
		return err
	}
	params.Response = runResponse
	return nil
}

type TaskOptionsGitRunbookRun struct {
	GitReference            string // required
	DefaultPackageVersion   string
	PackageVersionOverrides []string
	GitResourceRefs         []string

	// if the task succeeds, the resulting output will be stored here
	Response *runbooks.GitRunbookRunResponseV1
	TaskOptionsRunbookRunBase
}

func gitRunbookRun(octopus *client.Client, space *spaces.Space, input any) error {
	params, ok := input.(*TaskOptionsGitRunbookRun)
	if !ok {
		return errors.New("invalid input type; expecting TaskOptionsGitRunbookRun")
	}
	if space == nil {
		return errors.New("space must be specified")
	}

	// we have the provided project name; go look it up
	if params.ProjectName == "" {
		return errors.New("project must be specified")
	}
	if params.RunbookName == "" {
		return errors.New("runbook name must be specified")
	}
	if len(params.Environments) == 0 {
		return errors.New("environment(s) must be specified")
	}

	if params.GitReference == "" {
		return errors.New("git reference must be specified")
	}

	// common properties
	abstractCmd := deployments.CreateExecutionAbstractCommandV1{
		SpaceID:              space.ID,
		ProjectIDOrName:      params.ProjectName,
		ForcePackageDownload: params.ForcePackageDownload,
		SpecificMachineNames: params.RunTargets,
		ExcludedMachineNames: params.ExcludeTargets,
		SkipStepNames:        params.ExcludedSteps,
		RunAt:                params.ScheduledStartTime,
		NoRunAfter:           params.ScheduledExpiryTime,
		Variables:            params.Variables,
	}

	b, err := strconv.ParseBool(params.GuidedFailureMode)
	if err == nil {
		abstractCmd.UseGuidedFailure = &b
	} else {
		// else they must have specified nothing, or perhaps "default". Sanity check it's not garbage
		if params.GuidedFailureMode != "" && !strings.EqualFold("default", params.GuidedFailureMode) {
			return fmt.Errorf("'%s' is not a valid value for guided failure mode", params.GuidedFailureMode)
		}
	}
	runCommand := runbooks.NewGitRunbookRunCommandV1(space.ID, params.ProjectName)
	runCommand.RunbookName = params.RunbookName
	runCommand.EnvironmentNames = params.Environments
	runCommand.Tenants = params.Tenants
	runCommand.TenantTags = params.TenantTags
	runCommand.GitRef = params.GitReference

	runCommand.PackageVersion = params.DefaultPackageVersion

	if len(params.PackageVersionOverrides) > 0 {
		runCommand.Packages = params.PackageVersionOverrides
	}

	if len(params.GitResourceRefs) > 0 {
		runCommand.GitResources = params.GitResourceRefs
	}

	runCommand.CreateExecutionAbstractCommandV1 = abstractCmd

	runResponse, err := runbooks.GitRunbookRunV1(octopus, runCommand)
	if err != nil {
		return err
	}
	params.Response = runResponse
	return nil
}
