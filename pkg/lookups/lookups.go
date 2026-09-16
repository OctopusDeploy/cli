// Package lookups resolves resource IDs to human readable names for display.
// It lives outside pkg/cmd so any command group can use it without importing
// another command's shared package.
package lookups

import (
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
)

// GetLifecycleIdToNameMap resolves lifecycle IDs to names for display.
// If the name cannot be resolved, the caller should fall back to the ID.
func GetLifecycleIdToNameMap(octopus *client.Client) map[string]string {
	lifecycleIdToNameMap := make(map[string]string)
	allLifecycles, err := octopus.Lifecycles.GetAll()
	if err != nil {
		return lifecycleIdToNameMap
	}
	for _, l := range allLifecycles {
		lifecycleIdToNameMap[l.GetID()] = l.Name
	}
	return lifecycleIdToNameMap
}

// GetProjectGroupIdToNameMap resolves project group IDs to names for display.
// If the name cannot be resolved, the caller should fall back to the ID.
func GetProjectGroupIdToNameMap(octopus *client.Client) map[string]string {
	projectGroupIdToNameMap := make(map[string]string)
	allProjectGroups, err := octopus.ProjectGroups.GetAll()
	if err != nil {
		return projectGroupIdToNameMap
	}
	for _, pg := range allProjectGroups {
		projectGroupIdToNameMap[pg.GetID()] = pg.Name
	}
	return projectGroupIdToNameMap
}

// GetLifecycleName resolves a single lifecycle name given its ID.
// An empty string is returned when the name cannot be resolved.
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

// GetProjectGroupName resolves a single project group name given its ID.
// An empty string is returned when the name cannot be resolved.
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
