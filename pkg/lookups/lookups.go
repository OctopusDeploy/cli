// Package lookups resolves resource IDs to human readable names for display.
// It lives outside pkg/cmd so any command group can use it without importing
// another command's shared package.
package lookups

import (
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
)

// GetLifecycleMap resolves lifecycle IDs to names for display. Best-effort: a
// failed lookup yields an empty map and callers fall back to the ID.
func GetLifecycleMap(octopus *client.Client) map[string]string {
	lifecycleMap := make(map[string]string)
	allLifecycles, err := octopus.Lifecycles.GetAll()
	if err != nil {
		return lifecycleMap
	}
	for _, l := range allLifecycles {
		lifecycleMap[l.GetID()] = l.Name
	}
	return lifecycleMap
}

// GetProjectGroupMap resolves project group IDs to names for display. Best-effort,
// as GetLifecycleMap is.
func GetProjectGroupMap(octopus *client.Client) map[string]string {
	projectGroupMap := make(map[string]string)
	allProjectGroups, err := octopus.ProjectGroups.GetAll()
	if err != nil {
		return projectGroupMap
	}
	for _, pg := range allProjectGroups {
		projectGroupMap[pg.GetID()] = pg.Name
	}
	return projectGroupMap
}

// GetLifecycleName resolves a single lifecycle ID, which is cheaper than a whole
// map when only one resource is being displayed. Empty when it can't be resolved.
func GetLifecycleName(octopus *client.Client, lifecycleID string) string {
	if lifecycleID == "" {
		return ""
	}
	lifecycle, err := octopus.Lifecycles.GetByID(lifecycleID)
	if err != nil {
		return ""
	}
	return lifecycle.Name
}

// GetProjectGroupName resolves a single project group ID, as GetLifecycleName does.
func GetProjectGroupName(octopus *client.Client, projectGroupID string) string {
	if projectGroupID == "" {
		return ""
	}
	projectGroup, err := octopus.ProjectGroups.GetByID(projectGroupID)
	if err != nil {
		return ""
	}
	return projectGroup.Name
}

// DisplayName prefers the resolved name, falling back to the ID so there is always
// something to show.
func DisplayName(id string, name string) string {
	if name == "" {
		return id
	}
	return name
}
