package executor

import (
	"errors"
	"fmt"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"strconv"
	"strings"
)

// ----- Create Release --------------------------------------

type TaskResultCreateRelease struct {
	Version string
}

// the command processor is responsible for accepting related entity names from the end user
// and looking them up for their ID's; we should only deal with strong references at this level

type TaskOptionsCreateRelease struct {
	ProjectName             string   // Required
	DefaultPackageVersion   string   // Optional
	GitCommit               string   // Optional
	GitReference            string   // Required for version controlled projects
	Version                 string   // optional
	ChannelName             string   // optional
	ReleaseNotes            string   // optional
	IgnoreIfAlreadyExists   bool     // optional
	IgnoreChannelRules      bool     // optional
	PackageVersionOverrides []string // optional
	// if the task succeeds, the resulting output will be stored here
	Response *releases.CreateReleaseResponseV1
}

func releaseCreate(octopus *client.Client, space *spaces.Space, input any) error {
	params, ok := input.(*TaskOptionsCreateRelease)
	if !ok {
		return errors.New("invalid input type; expecting TaskOptionsCreateRelease")
	}

	if space == nil {
		return errors.New("space must be specified")
	}

	// we have the provided project name; go look it up
	if params.ProjectName == "" {
		return errors.New("project must be specified")
	}

	createReleaseParams := releases.NewCreateReleaseCommandV1(space.ID, params.ProjectName)

	createReleaseParams.PackageVersion = params.DefaultPackageVersion

	if len(params.PackageVersionOverrides) > 0 {
		createReleaseParams.Packages = params.PackageVersionOverrides
	}

	createReleaseParams.GitCommit = params.GitCommit
	createReleaseParams.GitRef = params.GitReference

	createReleaseParams.ReleaseVersion = params.Version
	createReleaseParams.ChannelIDOrName = params.ChannelName

	createReleaseParams.ReleaseNotes = params.ReleaseNotes

	createReleaseParams.IgnoreIfAlreadyExists = params.IgnoreIfAlreadyExists
	createReleaseParams.IgnoreChannelRules = params.IgnoreChannelRules

	createReleaseResponse, err := releases.CreateReleaseV1(octopus, createReleaseParams)
	if err != nil {
		return err
	}

	params.Response = createReleaseResponse
	return nil
}

// ----- Deploy Release --------------------------------------

type TaskOptionsDeployRelease struct {
	ProjectName          string   // required
	ReleaseVersion       string   // the release to deploy
	Environments         []string // multiple for untenanted deployment, only one entry for tenanted deployment
	Tenants              []string
	TenantTags           []string
	DeployAt             string
	MaxQueueTime         string
	ExcludedSteps        []string
	GuidedFailureMode    string // ["", "true", "false", "default"]. Note default and "" are the same, the only difference is whether interactive mode prompts you
	ForcePackageDownload bool
	DeploymentTargets    []string
	ExcludeTargets       []string

	// extra behaviour commands

	// true if the value was specified on the command line (because ForcePackageDownload is bool, we can't distinguish 'false' from 'missing')
	ForcePackageDownloadWasSpecified bool

	Response *deployments.CreateDeploymentResponseV1
}

func releaseDeploy(octopus *client.Client, space *spaces.Space, input any) error {
	params, ok := input.(*TaskOptionsDeployRelease)
	if !ok {
		return errors.New("invalid input type; expecting TaskOptionsDeployRelease")
	}
	if space == nil {
		return errors.New("space must be specified")
	}

	// we have the provided project name; go look it up
	if params.ProjectName == "" {
		return errors.New("project must be specified")
	}

	// common properties
	abstractCmd := deployments.CreateExecutionAbstractCommandV1{
		SpaceID:              space.ID,
		ProjectIDOrName:      params.ProjectName,
		ForcePackageDownload: params.ForcePackageDownload,
		SpecificMachineNames: params.DeploymentTargets,
		ExcludedMachineNames: params.ExcludeTargets,
		SkipStepNames:        params.ExcludedSteps,
		RunAt:                "",
		NoRunAfter:           "",
		Variables:            nil,
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

	// If either tenants or tenantTags are specified then it must be a tenanted deployment.
	// Otherwise it must be untenanted.
	// If the server has a tenanted deployment and both TenantNames+Tags are empty, the request fails,
	// which makes this a safe thing to build our logic on.
	isTenanted := len(params.Tenants) > 0 || len(params.TenantTags) > 0

	if isTenanted {
		tenantedCommand := deployments.NewCreateDeploymentTenantedCommandV1(space.ID, params.ProjectName)
		tenantedCommand.ReleaseVersion = params.ReleaseVersion
		tenantedCommand.EnvironmentName = params.Environments[0]
		tenantedCommand.Tenants = params.Tenants
		tenantedCommand.TenantTags = params.TenantTags
		tenantedCommand.ForcePackageRedeployment = params.ForcePackageDownload
		// tenantedCommand.UpdateVariableSnapshot = params.UpdateVariableSnapshot
		tenantedCommand.CreateExecutionAbstractCommandV1 = abstractCmd

		createDeploymentResponse, err := deployments.CreateDeploymentTenantedV1(octopus, tenantedCommand)
		if err != nil {
			return err
		}
		params.Response = createDeploymentResponse
	} else {
		untenantedCommand := deployments.NewCreateDeploymentUntenantedCommandV1(space.ID, params.ProjectName)
		untenantedCommand.ReleaseVersion = params.ReleaseVersion
		untenantedCommand.EnvironmentNames = params.Environments
		untenantedCommand.ForcePackageRedeployment = params.ForcePackageDownload
		//untenantedCommand.UpdateVariableSnapshot = params.UpdateVariableSnapshot
		untenantedCommand.CreateExecutionAbstractCommandV1 = abstractCmd

		createDeploymentResponse, err := deployments.CreateDeploymentUntenantedV1(octopus, untenantedCommand)
		if err != nil {
			return err
		}
		params.Response = createDeploymentResponse
	}

	return nil
}
