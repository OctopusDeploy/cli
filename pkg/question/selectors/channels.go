package selectors

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"io"
	"strings"
)

func Channel(octopus *octopusApiClient.Client, ask question.Asker, io io.Writer, questionText string, project *projects.Project) (*channels.Channel, error) {
	existingChannels, err := octopus.Projects.GetChannels(project)
	if len(existingChannels) == 1 {
		fmt.Fprintf(io, "Selecting only available channel '%s'.\n", output.Cyan(existingChannels[0].Name))
		return existingChannels[0], nil
	}
	if err != nil {
		return nil, err
	}

	return question.SelectMap(ask, questionText, existingChannels, func(p *channels.Channel) string {
		return p.Name
	})
}

// FindChannel looks a channel up within a project by either its ID or its name. An ID match
// wins over a name match, so it stays consistent with how projects and tenants resolve.
func FindChannel(octopus *octopusApiClient.Client, project *projects.Project, channelIdentifier string) (*channels.Channel, error) {
	foundChannels, err := octopus.Projects.GetChannels(project) // TODO change this to channel partial name search on server; will require go client update
	if err != nil {
		return nil, err
	}
	for _, c := range foundChannels {
		if strings.EqualFold(c.ID, channelIdentifier) {
			return c, nil
		}
	}
	for _, c := range foundChannels { // server doesn't support channel search by exact name so we must emulate it
		if strings.EqualFold(c.Name, channelIdentifier) {
			return c, nil
		}
	}
	return nil, fmt.Errorf("cannot find a channel in project '%s' with the ID or name of '%s'", project.GetName(), channelIdentifier)
}
