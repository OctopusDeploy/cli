package selectors

import (
	"errors"
	"github.com/OctopusDeploy/cli/pkg/question"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"math"
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

func FindRunbook(client *octopusApiClient.Client, runbookIdentifier string, projectID string) (*runbooks.Runbook, error) {
	runbook, err := runbooks.GetByID(client, client.GetSpaceID(), runbookIdentifier)
	if err != nil {
		runbook, err = runbooks.GetByName(client, client.GetSpaceID(), projectID, runbookIdentifier)
		if err != nil {
			return nil, err
		}
	}

	return runbook, nil
}
