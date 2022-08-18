package executor

import (
	"errors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
)

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

	// CreateReleaseV1 looks like this:
	//SpaceIDOrName         string   `json:"spaceIdOrName"`
	//ProjectIDOrName       string   `json:"projectName"`
	//PackageVersion        string   `json:"packageVersion,omitempty"`
	//GitCommit             string   `json:"gitCommit,omitempty"`
	//GitRef                string   `json:"gitRef,omitempty"`
	//ReleaseVersion        string   `json:"releaseVersion,omitempty"`
	//ChannelIDOrName       string   `json:"channelName,omitempty"`
	//Packages              []string `json:"packages,omitempty"`
	//ReleaseNotes          string   `json:"releaseNotes,omitempty"`
	//IgnoreIfAlreadyExists bool     `json:"ignoreIfAlreadyExists,omitempty"`
	//IgnoreChannelRules    bool     `json:"ignoreChannelRules,omitempty"`
	//PackagePrerelease     string   `json:"packagePrerelease,omitempty"`
	createReleaseParams := releases.NewCreateReleaseV1(space.ID, params.ProjectName)

	if params.DefaultPackageVersion != "" {
		createReleaseParams.PackageVersion = params.DefaultPackageVersion
	}

	if len(params.PackageVersionOverrides) > 0 {
		createReleaseParams.Packages = params.PackageVersionOverrides
	}

	if params.GitCommit != "" {
		createReleaseParams.GitCommit = params.GitCommit
	}
	if params.GitReference != "" {
		createReleaseParams.GitRef = params.GitReference
	}

	if params.Version != "" {
		createReleaseParams.ReleaseVersion = params.Version
	}
	if params.ChannelName != "" {
		createReleaseParams.ChannelIDOrName = params.ChannelName
	}

	if params.ReleaseNotes != "" {
		createReleaseParams.ReleaseNotes = params.ReleaseNotes
	}

	createReleaseParams.IgnoreIfAlreadyExists = params.IgnoreIfAlreadyExists
	createReleaseParams.IgnoreChannelRules = params.IgnoreChannelRules

	// TODO what is PackagePrerelease and do we need it?
	// createReleaseParams.PackagePrerelease = params.PackagePrerelease

	createReleaseResponse, err := octopus.Releases.CreateV1(createReleaseParams)
	if err != nil {
		return err
	}

	params.Response = createReleaseResponse
	return nil
}
