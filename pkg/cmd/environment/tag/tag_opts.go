package tag

import (
	"fmt"
	"strings"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tagsets"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
)

type GetAllTagSetsCallback func() ([]*tagsets.TagSet, error)
type GetEnvironmentCallback func(environmentIdentifier string) (*environments.Environment, error)
type GetEnvironmentsCallback func() ([]*environments.Environment, error)

type TagOptions struct {
	*TagFlags
	*cmd.Dependencies
	GetAllTagsCallback      GetAllTagSetsCallback
	GetEnvironmentCallback  GetEnvironmentCallback
	GetEnvironmentsCallback GetEnvironmentsCallback
	environment             *environments.Environment
}

func NewTagOptions(tagFlags *TagFlags, dependencies *cmd.Dependencies) *TagOptions {
	return &TagOptions{
		TagFlags:                tagFlags,
		Dependencies:            dependencies,
		GetAllTagsCallback:      getAllTagSetsCallback(dependencies.Client),
		GetEnvironmentCallback:  getEnvironmentCallback(dependencies.Client),
		GetEnvironmentsCallback: getEnvironmentsCallback(dependencies.Client),
		environment:             nil,
	}
}

func (to *TagOptions) Commit() error {
	if to.environment == nil {
		environment, err := to.GetEnvironmentCallback(to.Environment.Value)
		if err != nil {
			return err
		}
		to.environment = environment
	}

	to.environment.EnvironmentTags = to.Tag.Value

	updatedEnvironment, err := to.Client.Environments.Update(to.environment)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(to.Out, "\nSuccessfully updated environment %s (%s).\n", updatedEnvironment.Name, updatedEnvironment.ID)
	if err != nil {
		return err
	}
	return nil
}

func (to *TagOptions) GenerateAutomationCmd() {
	if !to.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(to.CmdPath, to.Environment, to.Tag)
		fmt.Fprintf(to.Out, "%s\n", autoCmd)
	}
}

func getEnvironmentCallback(client *client.Client) GetEnvironmentCallback {
	return func(environmentIdentifier string) (*environments.Environment, error) {
		// Try to get by ID first
		environment, _ := environments.GetByID(client, client.GetSpaceID(), environmentIdentifier)
		if environment != nil {
			return environment, nil
		}

		// Fall back to lookup by name
		allEnvironments, err := environments.Get(client, client.GetSpaceID(), environments.EnvironmentsQuery{
			PartialName: environmentIdentifier,
		})
		if err != nil {
			return nil, err
		}

		for _, env := range allEnvironments.Items {
			if strings.EqualFold(env.Name, environmentIdentifier) {
				return env, nil
			}
		}

		return nil, fmt.Errorf("environment '%s' not found", environmentIdentifier)
	}
}

func getEnvironmentsCallback(client *client.Client) GetEnvironmentsCallback {
	return func() ([]*environments.Environment, error) {
		allEnvironments, err := environments.GetAll(client, client.GetSpaceID())
		if err != nil {
			return nil, err
		}
		return allEnvironments, nil
	}
}

func getAllTagSetsCallback(client *client.Client) GetAllTagSetsCallback {
	return func() ([]*tagsets.TagSet, error) {
		query := tagsets.TagSetsQuery{
			Scopes: []string{string(tagsets.TagSetScopeEnvironment)},
		}
		result, err := tagsets.Get(client, client.GetSpaceID(), query)
		if err != nil {
			return nil, err
		}
		return result.Items, nil
	}
}
