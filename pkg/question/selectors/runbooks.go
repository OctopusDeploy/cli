package selectors

import (
	"errors"
	"math"

	"github.com/OctopusDeploy/cli/pkg/question"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
)

func Runbook(questionText string, client *octopusApiClient.Client, ask question.Asker, projectID string) (*runbooks.Runbook, error) {
	existingRunbooks, err := runbooks.List(client, client.GetSpaceID(), projectID, "", math.MaxInt32)
	if err != nil {
		return nil, err
	}

	if len(existingRunbooks.Items) == 0 {
		return nil, errors.New("no runbooks found")
	}

	if len(existingRunbooks.Items) == 1 {
		return existingRunbooks.Items[0], nil
	}

	return question.SelectMap(ask, questionText, existingRunbooks.Items, func(r *runbooks.Runbook) string {
		return r.Name
	})
}

func FindRunbook(client *octopusApiClient.Client, project *projects.Project, runbookIdentifier string) (*runbooks.Runbook, error) {
	runbook, err := runbooks.GetByID(client, client.GetSpaceID(), runbookIdentifier)
	if err != nil {
		runbook, err = runbooks.GetByName(client, client.GetSpaceID(), project.GetID(), runbookIdentifier)
		if err != nil {
			return nil, err
		}
	}

	return runbook, nil
}

func FindGitRunbook(client *octopusApiClient.Client, project *projects.Project, runbookIdentifier string, gitRef string) (*runbooks.Runbook, error) {
	runbook, err := runbooks.GetGitRunbookByID(client, client.GetSpaceID(), project.ID, gitRef, runbookIdentifier)
	if err != nil {
		runbook, err = runbooks.GetGitRunbookByName(client, client.GetSpaceID(), project.GetID(), gitRef, runbookIdentifier)
		if err != nil {
			return nil, err
		}
	}

	return runbook, nil
}
