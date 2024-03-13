package selectors

import (
	"errors"
	"github.com/OctopusDeploy/cli/pkg/question"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"math"
)

func Runbook(questionText string, octopus *octopusApiClient.Client, ask question.Asker, projectID string) (*runbooks.Runbook, error) {
	existingRunbooks, err := runbooks.List(octopus, octopus.GetSpaceID(), projectID, "", math.MaxInt32)
	if len(existingRunbooks.Items) == 0 {
		return nil, errors.New("no runbooks found")
	}

	if len(existingRunbooks.Items) == 1 {
		return existingRunbooks.Items[0], nil
	}
	if err != nil {
		return nil, err
	}
	return question.SelectMap(ask, questionText, existingRunbooks.Items, func(p *runbooks.Runbook) string {

		return p.Name
	})
}

func FindRunbook(octopus *octopusApiClient.Client, projectID string, runbookIdentifier string) (*runbooks.Runbook, error) {
	runbook, err := runbooks.GetByName(octopus, octopus.GetSpaceID(), projectID, runbookIdentifier)
	if err != nil {
		runbook, err = runbooks.GetByID(octopus, octopus.GetSpaceID(), runbookIdentifier)
		if err != nil {
			return nil, err
		}
	}

	return runbook, nil
}
