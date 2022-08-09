package helper

import (
	"github.com/OctopusDeploy/cli/pkg/factory"
	"strings"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
)

// ResolveNamesOrId takes in an array of names and trys to find an exact match.
// If a match is found it will return its corresponding ID. If no match is found
// it will return the name as is, in assumption it is an ID.
func ResolveEnvironmentNames(envs []string, octopus *client.Client, spinner factory.Spinner) ([]string, error) {
	spinner.Start()
	envIds := make([]string, 0, len(envs))
loop:
	for _, envName := range envs {
		matches, err := octopus.Environments.Get(environments.EnvironmentsQuery{
			Name: envName,
		})
		if err != nil {
			return nil, err
		}
		allMatches, err := matches.GetAllPages(octopus.Environments.GetClient())
		if err != nil {
			spinner.Stop()
			return nil, err
		}
		for _, match := range allMatches {
			if strings.EqualFold(envName, match.Name) {
				envIds = append(envIds, match.ID)
				continue loop
			}
		}
		envIds = append(envIds, envName)
	}
	spinner.Stop()
	return envIds, nil
}
