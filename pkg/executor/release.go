package executor

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/dashboard"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/rogpeppe/go-internal/semver"
)

// ----- Create Release --------------------------------------

type TaskResultCreateRelease struct {
	Version string
}

// the command processor is responsible for accepting related entity names from the end user
// and looking them up for their ID's; we should only deal with strong references at this level

type TaskOptionsCreateRelease struct {
	ProjectName             string            // Required
	DefaultPackageVersion   string            // Optional
	GitCommit               string            // Optional
	GitReference            string            // Required for version controlled projects
	Version                 string            // optional
	ChannelName             string            // optional
	ReleaseNotes            string            // optional
	IgnoreIfAlreadyExists   bool              // optional
	IgnoreChannelRules      bool              // optional
	PackageVersionOverrides []string          // optional
	GitResourceRefs         []string          //optional
	CustomFields            map[string]string // optional
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

	if len(params.GitResourceRefs) > 0 {
		createReleaseParams.GitResources = params.GitResourceRefs
	}

	createReleaseParams.GitCommit = params.GitCommit
	createReleaseParams.GitRef = params.GitReference

	createReleaseParams.ReleaseVersion = params.Version
	createReleaseParams.ChannelIDOrName = params.ChannelName

	createReleaseParams.ReleaseNotes = params.ReleaseNotes

	createReleaseParams.IgnoreIfAlreadyExists = params.IgnoreIfAlreadyExists
	createReleaseParams.IgnoreChannelRules = params.IgnoreChannelRules

	if len(params.CustomFields) > 0 {
		createReleaseParams.CustomFields = params.CustomFields
	}

	createReleaseResponse, err := releases.CreateReleaseV1(octopus, createReleaseParams)
	if err != nil {
		return err
	}

	params.Response = createReleaseResponse
	return nil
}

// ----- Deploy Release --------------------------------------

type BaseTaskOptionsDeployRelease struct {
	ScheduledStartTime             string
	ScheduledExpiryTime            string
	Tenants                        []string
	TenantTags                     []string
	ExcludedSteps                  []string
	GuidedFailureMode              string // ["", "true", "false", "default"]. Note default and "" are the same, the only difference is whether interactive mode prompts you
	ForcePackageDownload           bool
	DeploymentTargets              []string
	ExcludeTargets                 []string
	Variables                      map[string]string
	UpdateVariables                bool
	DeploymentFreezeNames          []string
	DeploymentFreezeOverrideReason string
}

type TaskOptionsDeployRelease struct {
	BaseTaskOptionsDeployRelease
	ProjectName    string   // required
	ReleaseVersion string   // the release to deploy
	Environments   []string // multiple for untenanted deployment, only one entry for tenanted deployment

	// extra behaviour commands

	// true if the value was specified on the command line (because ForcePackageDownload is bool, we can't distinguish 'false' from 'missing')
	ForcePackageDownloadWasSpecified bool

	// so the automation command can mask sensitive variable output
	SensitiveVariableNames []string

	// printing a link to the release (to check deployment status) requires the release ID, not version.
	// the interactive process looks this up, so we can cache it here to avoid a second lookup when generating
	// the link for the browser. It isn't necessary though
	ReleaseID string

	// After we send the request to the server, the response is stored here
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
	if params.ReleaseVersion == "" {
		return errors.New("release version must be specified")
	}
	if len(params.Environments) == 0 {
		return errors.New("environment(s) must be specified")
	}

	// common properties
	abstractCmd := deployments.CreateExecutionAbstractCommandV1{
		SpaceID:                        space.ID,
		ProjectIDOrName:                params.ProjectName,
		ForcePackageDownload:           params.ForcePackageDownload,
		SpecificMachineNames:           params.DeploymentTargets,
		ExcludedMachineNames:           params.ExcludeTargets,
		SkipStepNames:                  params.ExcludedSteps,
		RunAt:                          params.ScheduledStartTime,
		NoRunAfter:                     params.ScheduledExpiryTime,
		Variables:                      params.Variables,
		DeploymentFreezeNames:          params.DeploymentFreezeNames,
		DeploymentFreezeOverrideReason: params.DeploymentFreezeOverrideReason,
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
		if len(params.Environments) > 1 {
			return fmt.Errorf("tenanted deployments can only specify one environment")
		}
		tenantedCommand := deployments.NewCreateDeploymentTenantedCommandV1(space.ID, params.ProjectName)
		tenantedCommand.ReleaseVersion = params.ReleaseVersion
		tenantedCommand.EnvironmentName = params.Environments[0]
		tenantedCommand.Tenants = params.Tenants
		tenantedCommand.TenantTags = params.TenantTags
		tenantedCommand.ForcePackageRedeployment = params.ForcePackageDownload
		tenantedCommand.UpdateVariableSnapshot = params.UpdateVariables
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
		untenantedCommand.UpdateVariableSnapshot = params.UpdateVariables
		untenantedCommand.CreateExecutionAbstractCommandV1 = abstractCmd

		createDeploymentResponse, err := deployments.CreateDeploymentUntenantedV1(octopus, untenantedCommand)
		if err != nil {
			return err
		}
		params.Response = createDeploymentResponse

	}

	return nil
}

// ----- Promote Release --------------------------------------

type TaskOptionsPromoteRelease struct {
	BaseTaskOptionsDeployRelease
	ProjectName      string
	FromEnvironment  string
	ToEnvironment    string
	LatestSuccessful bool

	// After we send the request to the server, the response is stored here
	Response  *deployments.CreateDeploymentResponseV1
	ReleaseID string
}

func releasePromote(octopus *client.Client, space *spaces.Space, input any) error {
	params, ok := input.(*TaskOptionsPromoteRelease)
	if !ok {
		return errors.New("invalid input type; expecting TaskOptionsPromoteRelease")
	}
	if space == nil {
		return errors.New("space must be specified")
	}

	// we have the provided project name; go look it up
	if params.ProjectName == "" {
		return errors.New("project must be specified")
	}
	if params.FromEnvironment == "" {
		return errors.New("from environment must be specified")
	}
	if params.ToEnvironment == "" {
		return errors.New("to environment must be specified")
	}

	proj, err := projects.GetByName(octopus, space.ID, params.ProjectName)
	if err != nil {
		return errors.New("project not found")
	}

	envs, err := environments.Get(octopus, space.ID, environments.EnvironmentQuery{IDs: []string{params.FromEnvironment}})
	if err != nil {
		return errors.New("environment not found")
	}
	//
	if len(envs.Items) == 0 {
		envs, err = environments.Get(octopus, space.ID, environments.EnvironmentQuery{Name: params.FromEnvironment})
		if err != nil || len(envs.Items) == 0 {
			return errors.New("environment not found")
		}
	}
	//just grab the first environment
	sourceEnv := envs.Items[0]

	dashboardQuery := dashboard.DashboardDynamicQuery{
		Environments: []string{sourceEnv.ID},
		Projects:     []string{proj.ID},
		// If latest successful, this corresponds to IncludePrevious
		IncludePrevious: params.LatestSuccessful,
	}

	dynamicDashboard, err := octopus.Dashboards.GetDynamicDashboard(dashboardQuery)
	if err != nil {
		return err
	}

	//filter the dashboard items based on project & environment
	//this is probably unnecessary, but it mimic's the existing logic
	var dashboardItems []*dashboard.DashboardItem
	for _, item := range dynamicDashboard.Items {
		if item.EnvironmentID == sourceEnv.ID && item.ProjectID == proj.ID {
			dashboardItems = append(dashboardItems, item)
		}
	}

	//we need to order the dashboard items by the release so we get the latest release first
	slices.SortFunc(dashboardItems, func(a, b *dashboard.DashboardItem) int {
		aVersion := a.ReleaseVersion
		if !strings.HasPrefix(aVersion, "v") {
			aVersion = "v" + aVersion
		}
		bVersion := b.ReleaseVersion
		if !strings.HasPrefix(bVersion, "v") {
			bVersion = "v" + bVersion
		}

		//we multiply by -1 to order by descending
		return semver.Compare(aVersion, bVersion) * -1
	})

	var dashboardItem *dashboard.DashboardItem
	deploymentType := "deployment"

	if params.LatestSuccessful {
		deploymentType = "successful deployment"
		for _, item := range dashboardItems {
			if item.State == "Success" {
				dashboardItem = item
			}
		}
	} else if len(dashboardItems) > 0 {
		dashboardItem = dashboardItems[0]
	}

	if dashboardItem == nil {
		return errors.New(fmt.Sprintf("Could not find the latest %[1]s of the project for this environment. Please check that a %[1]s for this project/environment exists on the dashboard.", deploymentType))
	}

	rel, err := releases.GetReleaseInProject(octopus, space.ID, proj.ID, dashboardItem.ReleaseVersion)
	if err != nil {
		return errors.New("release not found")
	}

	return releaseDeploy(octopus, space, TaskOptionsDeployRelease{
		BaseTaskOptionsDeployRelease: params.BaseTaskOptionsDeployRelease,
		ProjectName:                  proj.ID,
		ReleaseVersion:               rel.Version,
		Environments:                 []string{params.ToEnvironment},
	})
}
