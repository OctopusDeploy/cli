package shared

import (
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
)

// InheritedLifecycle is shown for a channel with no lifecycle of its own, which
// therefore inherits the project's lifecycle (e.g. the default channel).
const InheritedLifecycle = "Inherited from project"

// ResolveChannel finds the channel a command should operate on within project:
// looking it up when the caller named one, prompting for it in interactive mode
// when they didn't, and failing in automation mode where there is nobody to ask.
//
// A named channel is resolved by a direct GetByID first; if the caller supplied a
// name, or the ID belongs to a channel in a different project, we fall back to the
// project-scoped lookup so we never return a channel from a project the caller
// didn't ask for.
func ResolveChannel(octopus *client.Client, ask question.Asker, promptEnabled bool, questionText string, project *projects.Project, idOrName string) (*channels.Channel, error) {
	if idOrName == "" {
		if !promptEnabled {
			return nil, errors.New("channel name or ID must be specified")
		}
		return promptForChannel(octopus, ask, questionText, project)
	}

	if channel, err := octopus.Channels.GetByID(idOrName); err == nil && channel != nil && channel.ProjectID == project.GetID() {
		return channel, nil
	}
	return selectors.FindChannel(octopus, project, idOrName)
}

func promptForChannel(octopus *client.Client, ask question.Asker, questionText string, project *projects.Project) (*channels.Channel, error) {
	existing, err := octopus.Projects.GetChannels(project)
	if err != nil {
		return nil, err
	}
	if len(existing) == 0 {
		return nil, fmt.Errorf("project %s has no channels", project.Name)
	}

	var chosenName string
	if err := ask(&survey.Select{
		Message: questionText,
		Options: channelNames(existing),
	}, &chosenName); err != nil {
		return nil, err
	}

	for _, c := range existing {
		if c.Name == chosenName {
			return c, nil
		}
	}
	return nil, fmt.Errorf("no channel found with name of %s", chosenName)
}

func channelNames(cs []*channels.Channel) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Name)
	}
	return out
}
