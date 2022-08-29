package executor

import (
	"errors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/executionsapi"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
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
	Response *executionsapi.CreateReleaseResponseV1
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

	createReleaseParams := executionsapi.NewCreateReleaseCommandV1(space.ID, params.ProjectName)

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

	createReleaseResponse, err := executionsapi.CreateReleaseV1(octopus, createReleaseParams)
	if err != nil {
		return err
	}

	params.Response = createReleaseResponse
	return nil
}

// ----- Deploy Release --------------------------------------

type TaskOptionsDeployRelease struct {
	ProjectName          string
	ChannelName          string
	ReleaseVersion       string   // the release to deploy
	Environment          string   // singular for tenanted deployment
	Environments         []string // multiple for untenanted deployment
	Tenants              []string
	TenantTags           []string
	DeployAt             string
	MaxQueueTime         string
	ExcludedSteps        []string
	GuidedFailureMode    string // tri-state: true, false, or "use default". Can we model it with an optional bool?
	ForcePackageDownload bool
	DeploymentTargets    []string
	ExcludeTargets       []string

	// TODO response output carrier
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

	return nil
}
