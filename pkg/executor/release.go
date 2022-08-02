package executor

import (
	"errors"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
)

// the command processor is responsible for accepting related entity names from the end user
// and looking them up for their ID's; we should only deal with strong references at this level
type TaskOptionsCreateRelease struct {
	ProjectName  string // Required
	GitReference string // Optional
	Version      string // optional
	ChannelName  string // optional
	// TODO array of package version overrides
	ReleaseNotes string // optional
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
		return errors.New("Project must be specified")
	}

	createReleaseParams := releases.NewCreateReleaseV1(currentSpace.ID, params.ProjectName)
	if params.ChannelName != "" {
		createReleaseParams.ChannelNameOrID = params.ChannelName
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

	_, err = apiClient.Releases.CreateV1(createReleaseParams)
	// TODO the response contains the output release information, we should probably return or log that

	return err
}
