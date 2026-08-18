package shared

import (
	"errors"
	"io"

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
func ResolveChannel(octopus *client.Client, ask question.Asker, out io.Writer, promptEnabled bool, questionText string, project *projects.Project, idOrName string) (*channels.Channel, error) {
	if idOrName == "" {
		if !promptEnabled {
			return nil, errors.New("channel name or ID must be specified")
		}
		return selectors.Channel(octopus, ask, out, questionText, project)
	}

	if channel, err := octopus.Channels.GetByID(idOrName); err == nil && channel != nil && channel.ProjectID == project.GetID() {
		return channel, nil
	}
	return selectors.FindChannel(octopus, project, idOrName)
}
