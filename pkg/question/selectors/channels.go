package selectors

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"strings"
)

func Channel(octopus *octopusApiClient.Client, ask question.Asker, spinner factory.Spinner, questionText string, project *projects.Project) (*channels.Channel, error) {
	spinner.Start()
	existingChannels, err := octopus.Projects.GetChannels(project)
	spinner.Stop()
	if err != nil {
		return nil, err
	}

	return question.SelectMap(ask, questionText, existingChannels, func(p *channels.Channel) string {
		return p.Name
	})
}

func FindChannel(octopus *octopusApiClient.Client, spinner factory.Spinner, project *projects.Project, channelName string) (*channels.Channel, error) {
	spinner.Start()
	foundChannels, err := octopus.Projects.GetChannels(project) // TODO change this to channel partial name search on server; will require go client update
	spinner.Stop()
	if err != nil {
		return nil, err
	}
	for _, c := range foundChannels { // server doesn't support channel search by exact name so we must emulate it
		if strings.EqualFold(c.Name, channelName) {
			return c, nil
		}
	}
	return nil, fmt.Errorf("no channel found with name of %s", channelName)
}
