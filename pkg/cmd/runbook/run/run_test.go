package run_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	surveyCore "github.com/AlecAivazis/survey/v2/core"
	"github.com/OctopusDeploy/cli/pkg/cmd/runbook/run"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/executor"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/stretchr/testify/assert"
	"net/url"
	"testing"
	"time"
)

var serverUrl, _ = url.Parse("http://server")

const placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

var rootResource = testutil.NewRootResource()

var now = func() time.Time {
	return time.Date(2022, time.September, 8, 13, 25, 2, 0, time.FixedZone("Malaysia", 8*3600)) // UTC+8
}
var ctxWithFakeNow = context.WithValue(context.TODO(), constants.ContextKeyTimeNow, now)

func TestRunbookRun_AskQuestions(t *testing.T) {
	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"
	space1 := fixtures.NewSpace(spaceID, "Default Space")

	fireProject := fixtures.NewProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "deploymentprocess-"+fireProjectID)

	// note: we don't need to test variableset stuff here because it's all the same code as deploy release, and tested as part of that.
	// however we do need at least one "sanity check" test to make sure we've plumbed the two bits of code into eachother properly
	variableSnapshotNoVars := fixtures.NewVariableSetForProject(spaceID, fireProjectID)
	variableSnapshotNoVars.ID = fmt.Sprintf("%s-s-0-2ZFWS", variableSnapshotNoVars.ID)

	//variableSnapshotWithPromptedVariables := fixtures.NewVariableSetForProject(spaceID, fireProjectID)
	//variableSnapshotWithPromptedVariables.ID = fmt.Sprintf("%s-s-0-9BZ22", variableSnapshotWithPromptedVariables.ID)
	//variableSnapshotWithPromptedVariables.Variables = []*variables.Variable{
	//	{
	//		Name: "Approver",
	//		Prompt: &variables.VariablePromptOptions{
	//			Description: "Who approved this deployment?",
	//			IsRequired:  true,
	//		},
	//		Type:  "String",
	//		Value: "",
	//	},
	//}

	provisionDbRunbook := fixtures.NewRunbook(spaceID, fireProjectID, "Runbooks-66", "Provision Database")
	provisionDbSnapshot1 := fixtures.NewRunbookSnapshot(spaceID, provisionDbRunbook.ID, "RunbookSnapshots-6601", "Snapshot FWKMLUX")
	provisionDbSnapshot1.FrozenProjectVariableSetID = variableSnapshotNoVars.ID
	provisionDbRunbook.PublishedRunbookSnapshotID = provisionDbSnapshot1.ID

	destroyDbRunbook := fixtures.NewRunbook(spaceID, fireProjectID, "Runbooks-201", "Destroy Database")
	// PublishedRunbookSnapshotID deliberately null here

	devEnvironment := fixtures.NewEnvironment(spaceID, "Environments-12", "dev")
	prodEnvironment := fixtures.NewEnvironment(spaceID, "Environments-13", "production")

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer)
	}{
		{"default process asking for standard things (non-tenanted, no advanced options)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return run.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{fireProject})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select project",
				Options: []string{"Fire Project"},
			}).AnswerWith("Fire Project")

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/runbooks?take=2147483647").RespondWith(resources.Resources[*runbooks.Runbook]{
				Items: []*runbooks.Runbook{provisionDbRunbook, destroyDbRunbook},
			})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select a runbook to run",
				Options: []string{provisionDbRunbook.Name, destroyDbRunbook.Name},
			}).AnswerWith(provisionDbRunbook.Name)

			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/runbooks/"+provisionDbRunbook.ID+"/environments").RespondWith([]*environments.Environment{
				devEnvironment, prodEnvironment,
			})

			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Select one or more environments",
				Options: []string{devEnvironment.Name, prodEnvironment.Name},
			}).AnswerWith([]surveyCore.OptionAnswer{
				{Value: devEnvironment.Name, Index: 0},
			})

			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/runbookSnapshots/"+provisionDbRunbook.PublishedRunbookSnapshotID).RespondWith(provisionDbSnapshot1)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshotNoVars.ID).RespondWith(&variableSnapshotNoVars)

			q := qa.ExpectQuestion(t, &survey.Select{
				Message: "Change additional options?",
				Options: []string{"Proceed to run", "Change"},
			})
			assert.Regexp(t, "Additional Options", stdout.String()) // actual options tested in PrintAdvancedSummary
			_ = q.AnswerWith("Proceed to run")

			err := <-errReceiver
			assert.Nil(t, err)

			// check that the question-asking process has filled out the things we told it to
			assert.Equal(t, &executor.TaskOptionsRunbookRun{
				ProjectName:       "Fire Project",
				RunbookName:       "Provision Database",
				Environments:      []string{"dev"},
				GuidedFailureMode: "",
				Variables:         make(map[string]string, 0),
			}, options)
		}},

		// TOOD test for a runbook with zero published snapshots:
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api, qa := testutil.NewMockServerAndAsker()
			test.run(t, api, qa, new(bytes.Buffer))
		})
	}
}
