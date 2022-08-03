package executor

import (
	"errors"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
)

type TaskResultCreateRelease struct {
	Version string
}

// the command processor is responsible for accepting related entity names from the end user
// and looking them up for their ID's; we should only deal with strong references at this level
type TaskOptionsCreateRelease struct {
	ProjectName  string // Required
	GitReference string // Optional
	Version      string // optional
	ChannelName  string // optional
	// TODO array of package version overrides
	ReleaseNotes string // optional

	// if the task succeeds, the resulting output will be stored here
	Response *releases.CreateReleaseResponseV1
}

func releaseCreate(f factory.Factory, input any) error {
	params, ok := input.(*TaskOptionsCreateRelease)
	if !ok {
		return errors.New("invalid input type; expecting TaskOptionsCreateRelease")
	}

	apiClient, err := f.GetSpacedClient()
	if err != nil {
		return err
	}

	// we know which space to use as the client is already bound to it
	currentSpace := f.GetCurrentSpace()

	// we have the provided project name; go look it up
	if params.ProjectName == "" {
		return errors.New("project must be specified")
	}

	createReleaseParams := releases.NewCreateReleaseV1(currentSpace.ID, params.ProjectName)
	if params.ChannelName != "" {
		createReleaseParams.ChannelIDOrName = params.ChannelName
	}
	if params.GitReference != "" {
		createReleaseParams.GitRef = params.GitReference
	}
	if params.Version != "" {
		createReleaseParams.ReleaseVersion = params.Version
	}
	// TODO all the other flags

	if params.ReleaseNotes != "" {
		createReleaseParams.ReleaseNotes = params.ReleaseNotes
	}

	createReleaseResponse, err := apiClient.Releases.CreateV1(createReleaseParams)
	if err != nil {
		return err
	}

	params.Response = createReleaseResponse
	return nil
}
