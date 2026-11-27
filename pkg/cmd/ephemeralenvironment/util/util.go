package util

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/ephemeralenvironments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"

	"github.com/OctopusDeploy/cli/pkg/cmd/runbook/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/output"
)

// GetConfiguredProjects returns the projects that have an ephemeral environment channel.
func GetConfiguredProjects(client *client.Client) ([]*projects.Project, error) {
	allProjects, err := shared.GetAllProjects(client)
	if err != nil {
		return nil, err
	}

	var filteredProjects []*projects.Project

	for _, project := range allProjects {
		projectChannels, err := client.Projects.GetChannels(project)
		if err != nil {
			return nil, fmt.Errorf("failed to get channels for project '%s': %w", project.GetName(), err)
		}

		if slices.ContainsFunc(projectChannels, func(channel *channels.Channel) bool {
			return channel.Type == channels.ChannelTypeEphemeral
		}) {
			filteredProjects = append(filteredProjects, project)
		}
	}

	if len(filteredProjects) == 0 {
		return nil, fmt.Errorf("no configured projects - configure a project with an ephemeral environment channel first")
	}

	return filteredProjects, nil
}

func GetByName(client *client.Client, name string, spaceID string) (*ephemeralenvironments.EphemeralEnvironment, error) {
	environments, err := ephemeralenvironments.GetByPartialName(client, spaceID, name)
	if err != nil {
		return nil, err
	}

	if environments.TotalResults == 0 {
		return nil, fmt.Errorf("no ephemeral environment found with the name '%s'", name)
	} else {
		var exactMatch *ephemeralenvironments.EphemeralEnvironment
		toLowerName := strings.ToLower(name)

		for _, environment := range environments.Items {
			if strings.ToLower(environment.Name) == toLowerName {
				exactMatch = environment
				break
			}
		}

		if exactMatch != nil {
			return exactMatch, nil
		}

		return nil, fmt.Errorf("could not find an exact match of an ephemeral environment with the name '%s'. Please specify a more specific name", name)
	}
}

func OutputDeprovisionResult(message string, command *cobra.Command, deprovisioningRuns []ephemeralenvironments.DeprovisioningRunbookRun) {
	outputFormat, err := command.Flags().GetString(constants.FlagOutputFormat)
	if err != nil {
		outputFormat = constants.OutputFormatTable
	}

	if deprovisioningRuns == nil {
		deprovisioningRuns = []ephemeralenvironments.DeprovisioningRunbookRun{}
	}

	switch outputFormat {
	case constants.OutputFormatBasic:
		command.Print(message)

		if len(deprovisioningRuns) == 0 {
			command.Println("Environment deprovisioned without running a runbook.")
		} else {
			for _, run := range deprovisioningRuns {
				command.Printf("Runbook Run ID: %s \nServer Task ID %s \n", run.RunbookRunID, run.TaskId)
			}
		}
	case constants.OutputFormatJson:
		data, err := json.Marshal(deprovisioningRuns)
		if err != nil {
			command.PrintErrln(err)
		} else {
			_, _ = command.OutOrStdout().Write(data)
			command.Println()
		}
	default:
		command.Print(message)

		if len(deprovisioningRuns) == 0 {
			command.Println("Environment deprovisioned without running a runbook.")
		} else {
			t := output.NewTable(command.OutOrStdout())
			t.AddRow(output.Bold("Runbook Run ID"), output.Bold("Server Task ID"))
			for _, run := range deprovisioningRuns {
				t.AddRow(run.RunbookRunID, run.TaskId)
			}
			err := t.Print()
			if err != nil {
				command.PrintErrln(err)
			}
		}
	}
}
