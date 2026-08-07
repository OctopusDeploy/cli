package shared

import (
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
)

// ResolveChannel finds a channel belonging to project, by ID or name. A direct
// GetByID is tried first; if the caller supplied a name, or the ID belongs to a
// channel in a different project, we fall back to the project-scoped lookup so we
// never return a channel from a project the caller didn't ask for.
func ResolveChannel(octopus *client.Client, project *projects.Project, idOrName string) (*channels.Channel, error) {
	if channel, err := octopus.Channels.GetByID(idOrName); err == nil && channel != nil && channel.ProjectID == project.GetID() {
		return channel, nil
	}
	return selectors.FindChannel(octopus, project, idOrName)
}
