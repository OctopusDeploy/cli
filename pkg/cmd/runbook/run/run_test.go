package run_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	surveyCore "github.com/AlecAivazis/survey/v2/core"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/runbook/run"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/executor"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
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

	// Unlike deployments, runbooks each have their own tenanted setting, they don't care about the project-level tenanted setting

	// note: we don't need to test variableset stuff here because it's all the same code as deploy release, and tested as part of that.
	// however we do need at least one "sanity check" test to make sure we've plumbed the two bits of code into eachother properly
	variableSnapshotNoVars := fixtures.NewVariableSetForProject(spaceID, fireProjectID)
	variableSnapshotNoVars.ID = fmt.Sprintf("%s-s-0-2ZFWS", variableSnapshotNoVars.ID)

	variableSnapshotWithPromptedVariables := fixtures.NewVariableSetForProject(spaceID, fireProjectID)
	variableSnapshotWithPromptedVariables.ID = fmt.Sprintf("%s-s-0-9BZ22", variableSnapshotWithPromptedVariables.ID)
	variableSnapshotWithPromptedVariables.Variables = []*variables.Variable{
		{
			Name: "Approver",
			Prompt: &variables.VariablePromptOptions{
				Description: "Who approved this deployment?",
				IsRequired:  true,
			},
			Type:  "String",
			Value: "",
		},
	}

	provisionDbRunbook := fixtures.NewRunbook(spaceID, fireProjectID, "Runbooks-66", "Provision Database")
	provisionDbRunbookSnapshot := fixtures.NewRunbookSnapshot(spaceID, provisionDbRunbook.ID, "RunbookSnapshots-6601", "Snapshot FWKMLUX")
	provisionDbRunbook.PublishedRunbookSnapshotID = provisionDbRunbookSnapshot.ID

	provisionDbRunbookSnapshot.FrozenProjectVariableSetID = variableSnapshotNoVars.ID

	runProcessSnapshot := fixtures.NewRunbookProcessForRunbook(spaceID, fireProjectID, provisionDbRunbook.ID)
	runProcessSnapshot.ID = fmt.Sprintf("%s-s-2-62VMF", runProcessSnapshot.ID)
	provisionDbRunbookSnapshot.FrozenRunbookProcessID = runProcessSnapshot.ID

	runProcessSnapshot.Steps = []*deployments.DeploymentStep{
		{
			Name:       "Install",
			Properties: map[string]core.PropertyValue{"Octopus.Action.TargetRoles": core.NewPropertyValue("deploy", false)},
			Actions: []*deployments.DeploymentAction{
				{ActionType: "Octopus.Script", Name: "Run a script"}, // technically scriptbody and other things are required but we don't touch them so it's fine
			},
		},
		{
			Name:       "Cleanup",
			Properties: map[string]core.PropertyValue{"Octopus.Action.TargetRoles": core.NewPropertyValue("deploy", false)},
			Actions: []*deployments.DeploymentAction{
				{ActionType: "Octopus.Script", Name: "Run a script"},
			},
		},
	}

	destroyDbRunbook := fixtures.NewRunbook(spaceID, fireProjectID, "Runbooks-201", "Destroy Database")
	// PublishedRunbookSnapshotID deliberately null here

	devEnvironment := fixtures.NewEnvironment(spaceID, "Environments-12", "dev")
	prodEnvironment := fixtures.NewEnvironment(spaceID, "Environments-13", "production")
	scratchEnvironment := fixtures.NewEnvironment(spaceID, "Environments-14", "scratch")

	cokeTenant := fixtures.NewTenant(spaceID, "Tenants-29", "Coke", "Regions/us-east", "Importance/High")
	cokeTenant.ProjectEnvironments = map[string][]string{
		fireProjectID: {devEnvironment.ID, prodEnvironment.ID},
	}
	pepsiTenant := fixtures.NewTenant(spaceID, "Tenants-37", "Pepsi", "Regions/us-east", "Importance/Low")
	pepsiTenant.ProjectEnvironments = map[string][]string{
		fireProjectID: {prodEnvironment.ID},
	}

	// helper for advanced tests that want to skip past the first half of the questions
	doStandardApiResponses := func(options *executor.TaskOptionsRunbookRun, api *testutil.MockHttpServer, runbook *runbooks.Runbook, snapshot *runbooks.RunbookSnapshot, vars *variables.VariableSet) {
		if options.RunbookName != runbook.Name {
			panic("you must set `options.RunbookName` to match the supplied `runbook.Name`")
		}
		api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName="+url.QueryEscape(options.ProjectName)).
			RespondWith(resources.Resources[*projects.Project]{
				Items: []*projects.Project{fireProject},
			})

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/runbooks?partialName=Provision%20Database").RespondWith(resources.Resources[*runbooks.Runbook]{
			Items: []*runbooks.Runbook{runbook},
		})

		api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/runbookSnapshots/"+provisionDbRunbook.PublishedRunbookSnapshotID).RespondWith(snapshot)

		api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+vars.ID).RespondWith(&vars)
	}

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

			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/runbookSnapshots/"+provisionDbRunbook.PublishedRunbookSnapshotID).RespondWith(provisionDbRunbookSnapshot)

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

		{"default process picking up standard things from cmdline (non-tenanted, no advanced options)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{
				ProjectName:  "fire project",
				RunbookName:  "provision database",
				Environments: []string{"dev"},
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return run.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=fire+project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/runbooks?partialName=provision%20database").RespondWith(resources.Resources[*runbooks.Runbook]{
				Items: []*runbooks.Runbook{provisionDbRunbook},
			})

			// doesn't lookup the env names because it already has them

			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/runbookSnapshots/"+provisionDbRunbook.PublishedRunbookSnapshotID).RespondWith(provisionDbRunbookSnapshot)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshotNoVars.ID).RespondWith(&variableSnapshotNoVars)

			assert.Equal(t, heredoc.Doc(`
				Project Fire Project
				Runbook Provision Database
				Environments dev
			`), stdout.String())
			stdout.Reset()

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

		{"can't run a runbook with no published snapshots", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{
				ProjectName:  "fire project",
				RunbookName:  "provision database",
				Environments: []string{"dev"},
			}

			provisionDbRunbookNoSnapshots := fixtures.NewRunbook(spaceID, fireProjectID, "Runbooks-66", "Provision Database")

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return run.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=fire+project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/runbooks?partialName=provision%20database").RespondWith(resources.Resources[*runbooks.Runbook]{
				Items: []*runbooks.Runbook{provisionDbRunbookNoSnapshots},
			})

			assert.Equal(t, heredoc.Doc(`
				Project Fire Project
			`), stdout.String())
			stdout.Reset()

			err := <-errReceiver
			assert.EqualError(t, err, "cannot run runbook Provision Database, it has no published snapshot")
		}},

		{"prompted variable", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{
				ProjectName:  "fire project",
				RunbookName:  "provision database",
				Environments: []string{"dev"},
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return run.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=fire+project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/runbooks?partialName=provision%20database").RespondWith(resources.Resources[*runbooks.Runbook]{
				Items: []*runbooks.Runbook{provisionDbRunbook},
			})

			// doesn't lookup the env names because it already has them

			provisionDbSnapshot2 := new(runbooks.RunbookSnapshot)
			*provisionDbSnapshot2 = *provisionDbRunbookSnapshot
			provisionDbSnapshot2.FrozenProjectVariableSetID = variableSnapshotWithPromptedVariables.ID

			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/runbookSnapshots/"+provisionDbRunbook.PublishedRunbookSnapshotID).RespondWith(provisionDbSnapshot2)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshotWithPromptedVariables.ID).RespondWith(&variableSnapshotWithPromptedVariables)

			q := qa.ExpectQuestion(t, &survey.Input{
				Message: "Approver (Who approved this deployment?)",
			})
			validationErr := q.AnswerWith("")
			assert.EqualError(t, validationErr, "Value is required")

			validationErr = q.AnswerWith("John")
			assert.Nil(t, validationErr)

			assert.Equal(t, heredoc.Doc(`
				Project Fire Project
				Runbook Provision Database
				Environments dev
			`), stdout.String())
			stdout.Reset()

			q = qa.ExpectQuestion(t, &survey.Select{
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
				Variables:         map[string]string{"Approver": "John"},
			}, options)
		}},

		{"tenants and tags for a tenanted runbook", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{
				ProjectName: "fire project",
				RunbookName: "provision database",
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return run.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=fire+project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			provisionDbRunbookTenanted := *provisionDbRunbook
			provisionDbRunbookTenanted.MultiTenancyMode = core.TenantedDeploymentModeTenanted

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/runbooks?partialName=provision%20database").RespondWith(resources.Resources[*runbooks.Runbook]{
				Items: []*runbooks.Runbook{&provisionDbRunbookTenanted},
			})

			// now we prompt for environment. Single select on a tenanted project
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/runbooks/"+provisionDbRunbook.ID+"/environments").RespondWith([]*environments.Environment{
				devEnvironment, prodEnvironment,
			})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select an environment",
				Options: []string{devEnvironment.Name, prodEnvironment.Name},
			}).AnswerWith(devEnvironment.Name)

			// now we look for tenants for this project
			api.ExpectRequest(t, "GET", "/api/Spaces-1/tenants?projectId="+fireProjectID).RespondWith(resources.Resources[*tenants.Tenant]{
				Items: []*tenants.Tenant{cokeTenant, pepsiTenant},
			})

			q := qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Select tenants and/or tags used to determine deployment targets",
				Options: []string{"Coke", "Importance/High", "Regions/us-east"},
			})

			validationErr := q.AnswerWith([]surveyCore.OptionAnswer{})
			assert.EqualError(t, validationErr, "Value is required")

			validationErr = q.AnswerWith([]surveyCore.OptionAnswer{
				{Value: "Coke", Index: 0},
				{Value: "Regions/us-east", Index: 2},
			})
			assert.Nil(t, validationErr)

			// done with tenants, back to the main flow
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/runbookSnapshots/"+provisionDbRunbook.PublishedRunbookSnapshotID).RespondWith(provisionDbRunbookSnapshot)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshotNoVars.ID).RespondWith(&variableSnapshotNoVars)

			assert.Equal(t, heredoc.Doc(`
				Project Fire Project
				Runbook Provision Database
			`), stdout.String())
			stdout.Reset()

			q = qa.ExpectQuestion(t, &survey.Select{
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
				Tenants:           []string{"Coke"},
				TenantTags:        []string{"Regions/us-east"},
				GuidedFailureMode: "",
				Variables:         make(map[string]string, 0),
			}, options)
		}},

		{"tenants and tags in a maybe tenanted runbook (choosing tenanted)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{
				ProjectName: "fire project",
				RunbookName: "provision database",
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return run.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=fire+project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			provisionDbRunbookMaybeTenanted := *provisionDbRunbook
			provisionDbRunbookMaybeTenanted.MultiTenancyMode = core.TenantedDeploymentModeTenantedOrUntenanted

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/runbooks?partialName=provision%20database").RespondWith(resources.Resources[*runbooks.Runbook]{
				Items: []*runbooks.Runbook{&provisionDbRunbookMaybeTenanted},
			})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select Tenanted or Untenanted run",
				Options: []string{"Tenanted", "Untenanted"},
			}).AnswerWith("Tenanted")

			// now we prompt for environment. Single select on a tenanted project
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/runbooks/"+provisionDbRunbook.ID+"/environments").RespondWith([]*environments.Environment{
				devEnvironment, prodEnvironment,
			})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select an environment",
				Options: []string{devEnvironment.Name, prodEnvironment.Name},
			}).AnswerWith(devEnvironment.Name)

			// now we look for tenants for this project
			api.ExpectRequest(t, "GET", "/api/Spaces-1/tenants?projectId="+fireProjectID).RespondWith(resources.Resources[*tenants.Tenant]{
				Items: []*tenants.Tenant{cokeTenant, pepsiTenant},
			})

			q := qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Select tenants and/or tags used to determine deployment targets",
				Options: []string{"Coke", "Importance/High", "Regions/us-east"},
			})

			validationErr := q.AnswerWith([]surveyCore.OptionAnswer{})
			assert.EqualError(t, validationErr, "Value is required")

			validationErr = q.AnswerWith([]surveyCore.OptionAnswer{
				{Value: "Coke", Index: 0},
				{Value: "Regions/us-east", Index: 2},
			})
			assert.Nil(t, validationErr)

			// done with tenants, back to the main flow
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/runbookSnapshots/"+provisionDbRunbook.PublishedRunbookSnapshotID).RespondWith(provisionDbRunbookSnapshot)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshotNoVars.ID).RespondWith(&variableSnapshotNoVars)

			assert.Equal(t, heredoc.Doc(`
				Project Fire Project
				Runbook Provision Database
			`), stdout.String())
			stdout.Reset()

			q = qa.ExpectQuestion(t, &survey.Select{
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
				Tenants:           []string{"Coke"},
				TenantTags:        []string{"Regions/us-east"},
				GuidedFailureMode: "",
				Variables:         make(map[string]string, 0),
			}, options)
		}},

		{"tenants and tags in a maybe tenanted runbook (choosing untenanted)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{
				ProjectName: "fire project",
				RunbookName: "provision database",
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return run.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=fire+project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			provisionDbRunbookMaybeTenanted := *provisionDbRunbook
			provisionDbRunbookMaybeTenanted.MultiTenancyMode = core.TenantedDeploymentModeTenantedOrUntenanted

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/runbooks?partialName=provision%20database").RespondWith(resources.Resources[*runbooks.Runbook]{
				Items: []*runbooks.Runbook{&provisionDbRunbookMaybeTenanted},
			})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select Tenanted or Untenanted run",
				Options: []string{"Tenanted", "Untenanted"},
			}).AnswerWith("Untenanted")

			// now we prompt for environment. Single select on a tenanted project
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/runbooks/"+provisionDbRunbook.ID+"/environments").RespondWith([]*environments.Environment{
				devEnvironment, prodEnvironment,
			})

			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Select one or more environments",
				Options: []string{devEnvironment.Name, prodEnvironment.Name},
			}).AnswerWith([]surveyCore.OptionAnswer{
				{Value: devEnvironment.Name, Index: 0},
			})

			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/runbookSnapshots/"+provisionDbRunbook.PublishedRunbookSnapshotID).RespondWith(provisionDbRunbookSnapshot)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshotNoVars.ID).RespondWith(&variableSnapshotNoVars)

			assert.Equal(t, heredoc.Doc(`
				Project Fire Project
				Runbook Provision Database
			`), stdout.String())
			stdout.Reset()

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

		{"advanced options", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{ProjectName: "fire project", RunbookName: "Provision Database", Environments: []string{"dev"}}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return run.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			doStandardApiResponses(options, api, provisionDbRunbook, provisionDbRunbookSnapshot, variableSnapshotNoVars)
			stdout.Reset()

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Change additional options?",
				Options: []string{"Proceed to run", "Change"},
			}).AnswerWith("Change")
			stdout.Reset()

			refNow := now()
			plus20hours := refNow.Add(20 * time.Hour)
			q := qa.ReceiveQuestion() // can't use ExpectQuestion; the DatePicker struct contains a func which is not comparable with anything
			datePicker := q.Question.(*surveyext.DatePicker)
			datePicker.AnswerFormatter = nil // now we can compare the struct

			assert.Equal(t, &surveyext.DatePicker{
				Message:     "Scheduled start time",
				Help:        "Enter the date and time that this runbook should run. A value less than 1 minute in the future means 'now'",
				Default:     refNow,
				Min:         refNow,
				Max:         refNow.Add(30 * 24 * time.Hour),
				OverrideNow: refNow,
			}, datePicker)
			_ = q.AnswerWith(plus20hours)

			plus20hours5mins := plus20hours.Add(5 * time.Minute)
			_ = qa.ExpectQuestion(t, &surveyext.DatePicker{
				Message:     "Scheduled expiry time",
				Default:     plus20hours5mins,
				Help:        "At the start time, the run will be queued. If it does not begin before 'expiry' time, it will be cancelled. Minimum of 5 minutes after start time",
				Min:         plus20hours5mins,
				Max:         refNow.Add(31 * 24 * time.Hour),
				OverrideNow: refNow,
			}).AnswerWith(plus20hours5mins)

			// it's going to load the runbook process to ask about excluded steps
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/runbookProcesses/"+runProcessSnapshot.ID).RespondWith(runProcessSnapshot)

			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Steps to skip (If none selected, run all steps)",
				Options: []string{"Install", "Cleanup"},
			}).AnswerWith([]string{"Cleanup"})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Guided Failure Mode",
				Options: []string{"Use default setting from the target environment", "Use guided failure mode", "Do not use guided failure mode"},
			}).AnswerWith("Do not use guided failure mode")

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Package download",
				Options: []string{"Use cached packages (if available)", "Re-download packages from feed"},
			}).AnswerWith("Re-download packages from feed")

			// because environments were specified on the commandline, we didn't look them up earlier, but we
			// must do it now in order to determine the list of deployment targets
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/all").RespondWith([]*environments.Environment{
				scratchEnvironment, devEnvironment, prodEnvironment,
			})

			api.ExpectRequest(t, "GET", fmt.Sprintf("/api/Spaces-1/runbookSnapshots/%s/runbookRuns/preview/%s?includeDisabledSteps=true", provisionDbRunbookSnapshot.ID, devEnvironment.ID)).RespondWith(&runbooks.RunPreview{
				StepsToExecute: []*runbooks.DeploymentTemplateStep{
					{},
					{MachineNames: []string{"vm-1", "vm-2"}},
					{MachineNames: []string{"vm-4"}},
				},
			})

			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Run targets (If none selected, run on all)",
				Options: []string{"vm-1", "vm-2", "vm-4"},
			}).AnswerWith([]string{"vm-1", "vm-2"})

			err := <-errReceiver
			assert.Nil(t, err)

			assert.Equal(t, &executor.TaskOptionsRunbookRun{
				ProjectName:          "Fire Project",
				RunbookName:          "Provision Database",
				Environments:         []string{"dev"},
				Variables:            make(map[string]string, 0),
				ScheduledStartTime:   "2022-09-09T09:25:02+08:00",
				ScheduledExpiryTime:  "2022-09-09T09:30:02+08:00",
				ExcludedSteps:        []string{"Cleanup"},
				GuidedFailureMode:    "false",
				ForcePackageDownload: true,
				RunTargets:           []string{"vm-1", "vm-2"},
			}, options)
		}},

		{"advanced options doesn't need to lookup environments if the Q&A process already asked for them", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{
				ProjectName:                      "fire project",
				RunbookName:                      "Provision Database",
				ExcludedSteps:                    []string{"Cleanup"},
				GuidedFailureMode:                "false",
				ForcePackageDownloadWasSpecified: true,
				ScheduledStartTime:               "some-sort-of-garbage(passthru to server)",
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return run.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName="+url.QueryEscape(options.ProjectName)).
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/runbooks?partialName=Provision%20Database").RespondWith(resources.Resources[*runbooks.Runbook]{
				Items: []*runbooks.Runbook{provisionDbRunbook},
			})

			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/runbooks/"+provisionDbRunbook.ID+"/environments").RespondWith([]*environments.Environment{
				devEnvironment, prodEnvironment,
			})

			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Select one or more environments",
				Options: []string{devEnvironment.Name, prodEnvironment.Name},
			}).AnswerWith([]surveyCore.OptionAnswer{
				{Value: devEnvironment.Name, Index: 0},
			})

			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/runbookSnapshots/"+provisionDbRunbook.PublishedRunbookSnapshotID).RespondWith(provisionDbRunbookSnapshot)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshotNoVars.ID).RespondWith(&variableSnapshotNoVars)

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Change additional options?",
				Options: []string{"Proceed to run", "Change"},
			}).AnswerWith("Change")
			stdout.Reset()
			// steps, guidedFailure and forcePackageDownload already on cmdline, so we go straight to targets

			// NOTE there is NO CALL to environments.all here, because we already have info loaded for the selected environment (devEnvironment)

			api.ExpectRequest(t, "GET", fmt.Sprintf("/api/Spaces-1/runbookSnapshots/%s/runbookRuns/preview/%s?includeDisabledSteps=true", provisionDbRunbookSnapshot.ID, devEnvironment.ID)).RespondWith(&runbooks.RunPreview{
				StepsToExecute: []*runbooks.DeploymentTemplateStep{
					{},
					{MachineNames: []string{"vm-1", "vm-2"}},
					{MachineNames: []string{"vm-4"}},
				},
			})

			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Run targets (If none selected, run on all)",
				Options: []string{"vm-1", "vm-2", "vm-4"},
			}).AnswerWith([]string{"vm-1"})

			err := <-errReceiver
			assert.Nil(t, err)

			assert.Equal(t, &executor.TaskOptionsRunbookRun{
				ProjectName:                      "Fire Project",
				RunbookName:                      "Provision Database",
				Environments:                     []string{"dev"},
				Variables:                        make(map[string]string, 0),
				ScheduledStartTime:               "some-sort-of-garbage(passthru to server)",
				ExcludedSteps:                    []string{"Cleanup"},
				GuidedFailureMode:                "false",
				ForcePackageDownload:             false,
				ForcePackageDownloadWasSpecified: true,
				RunTargets:                       []string{"vm-1"},
			}, options)
		}},

		{"advanced options pickup from command line; doesn't ask if all opts are supplied", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{
				ProjectName:                      "fire project",
				RunbookName:                      "Provision Database",
				Environments:                     []string{"dev"},
				ExcludedSteps:                    []string{"Cleanup"},
				GuidedFailureMode:                "false",
				ForcePackageDownloadWasSpecified: true,
				ExcludeTargets:                   []string{"vm-99"},
				ScheduledStartTime:               "some-sort-of-garbage(passthru to server)",
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return run.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			doStandardApiResponses(options, api, provisionDbRunbook, provisionDbRunbookSnapshot, variableSnapshotNoVars)
			stdout.Reset()

			err := <-errReceiver
			assert.Nil(t, err)

			assert.Equal(t, &executor.TaskOptionsRunbookRun{
				ProjectName:                      "Fire Project",
				RunbookName:                      "Provision Database",
				Environments:                     []string{"dev"},
				Variables:                        make(map[string]string, 0),
				ScheduledStartTime:               "some-sort-of-garbage(passthru to server)",
				ExcludedSteps:                    []string{"Cleanup"},
				GuidedFailureMode:                "false",
				ForcePackageDownload:             false,
				ForcePackageDownloadWasSpecified: true,
				ExcludeTargets:                   []string{"vm-99"},
			}, options)
		}},

		{"scheduled start time; interactive start times less than 1 minute in future are interpreted as 'now'", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{
				ProjectName:                      "fire project",
				RunbookName:                      "Provision Database",
				Environments:                     []string{"dev"},
				ExcludedSteps:                    []string{"Cleanup"},
				ExcludeTargets:                   []string{"vm-99"},
				GuidedFailureMode:                "false",
				ForcePackageDownloadWasSpecified: true,
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return run.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			doStandardApiResponses(options, api, provisionDbRunbook, provisionDbRunbookSnapshot, variableSnapshotNoVars)
			stdout.Reset()

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Change additional options?",
				Options: []string{"Proceed to run", "Change"},
			}).AnswerWith("Change")
			stdout.Reset()

			refNow := now()
			plus59s := refNow.Add(59 * time.Second)
			q := qa.ReceiveQuestion() // can't use ExpectQuestion; the DatePicker struct contains a func which is not comparable with anything
			datePicker := q.Question.(*surveyext.DatePicker)
			datePicker.AnswerFormatter = nil // now we can compare the struct

			assert.Equal(t, &surveyext.DatePicker{
				Message:     "Scheduled start time",
				Default:     refNow,
				Help:        "Enter the date and time that this runbook should run. A value less than 1 minute in the future means 'now'",
				Min:         refNow,
				Max:         refNow.Add(30 * 24 * time.Hour),
				OverrideNow: refNow,
			}, datePicker)
			_ = q.AnswerWith(plus59s)
			// note it doesn't ask for a scheduled end time

			err := <-errReceiver
			assert.Nil(t, err)

			assert.Equal(t, &executor.TaskOptionsRunbookRun{
				ProjectName:                      "Fire Project",
				RunbookName:                      "Provision Database",
				Environments:                     []string{"dev"},
				Variables:                        make(map[string]string, 0),
				ExcludedSteps:                    []string{"Cleanup"},
				ExcludeTargets:                   []string{"vm-99"},
				GuidedFailureMode:                "false",
				ForcePackageDownloadWasSpecified: true,
				// no scheduled start time explicitly
			}, options)
		}},

		{"scheduled start time; interactive start times greater than 1 minute in future are interpreted as scheduled", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{
				ProjectName:                      "fire project",
				RunbookName:                      "Provision Database",
				Environments:                     []string{"dev"},
				ExcludedSteps:                    []string{"Cleanup"},
				ExcludeTargets:                   []string{"vm-99"},
				GuidedFailureMode:                "false",
				ForcePackageDownloadWasSpecified: true,
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return run.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			doStandardApiResponses(options, api, provisionDbRunbook, provisionDbRunbookSnapshot, variableSnapshotNoVars)
			stdout.Reset()

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Change additional options?",
				Options: []string{"Proceed to run", "Change"},
			}).AnswerWith("Change")
			stdout.Reset()

			refNow := now()
			plus61s := refNow.Add(61 * time.Second)

			q := qa.ReceiveQuestion() // can't use ExpectQuestion; the DatePicker struct contains a func which is not comparable with anything
			datePicker := q.Question.(*surveyext.DatePicker)
			datePicker.AnswerFormatter = nil // now we can compare the struct

			assert.Equal(t, &surveyext.DatePicker{
				Message:     "Scheduled start time",
				Default:     refNow,
				Help:        "Enter the date and time that this runbook should run. A value less than 1 minute in the future means 'now'",
				Min:         refNow,
				Max:         refNow.Add(30 * 24 * time.Hour),
				OverrideNow: refNow,
			}, datePicker)
			_ = q.AnswerWith(plus61s)

			plus61s5min := plus61s.Add(5 * time.Minute)
			_ = qa.ExpectQuestion(t, &surveyext.DatePicker{
				Message:     "Scheduled expiry time",
				Default:     plus61s5min,
				Help:        "At the start time, the run will be queued. If it does not begin before 'expiry' time, it will be cancelled. Minimum of 5 minutes after start time",
				Min:         plus61s5min,
				Max:         refNow.Add(31 * 24 * time.Hour),
				OverrideNow: refNow,
			}).AnswerWith(plus61s5min)

			err := <-errReceiver
			assert.Nil(t, err)

			assert.Equal(t, &executor.TaskOptionsRunbookRun{
				ProjectName:                      "Fire Project",
				RunbookName:                      "Provision Database",
				Environments:                     []string{"dev"},
				Variables:                        make(map[string]string, 0),
				ExcludedSteps:                    []string{"Cleanup"},
				ExcludeTargets:                   []string{"vm-99"},
				GuidedFailureMode:                "false",
				ForcePackageDownloadWasSpecified: true,
				// no scheduled start time explicitly
			}, options)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api, qa := testutil.NewMockServerAndAsker()
			test.run(t, api, qa, new(bytes.Buffer))
		})
	}
}

func TestRunbookRun_PrintAdvancedSummary(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, stdout *bytes.Buffer)
	}{
		{"default state", func(t *testing.T, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{}
			run.PrintAdvancedSummary(stdout, options)

			assert.Equal(t, heredoc.Doc(`
			Additional Options:
			  Run At: Now
			  Skipped Steps: None
			  Guided Failure Mode: Use default setting from the target environment
			  Package Download: Use cached packages (if available)
			  Run Targets: All included
			`), stdout.String())
		}},

		{"all the things different", func(t *testing.T, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{
				ScheduledStartTime:   "2022-09-23",
				GuidedFailureMode:    "false",
				ForcePackageDownload: true,
				ExcludedSteps:        []string{"Step 1", "Step 37"},
				RunTargets:           []string{"vm-1", "vm-2"},
				ExcludeTargets:       []string{"vm-3", "vm-4"},
			}
			run.PrintAdvancedSummary(stdout, options)

			assert.Equal(t, heredoc.Doc(`
			Additional Options:
			  Run At: 2022-09-23
			  Skipped Steps: Step 1,Step 37
			  Guided Failure Mode: Do not use guided failure mode
			  Package Download: Re-download packages from feed
			  Run Targets: Include vm-1,vm-2; Exclude vm-3,vm-4
			`), stdout.String())
		}},

		{"variation on include deployment targets only", func(t *testing.T, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{
				RunTargets: []string{"vm-2"},
			}
			run.PrintAdvancedSummary(stdout, options)

			assert.Equal(t, heredoc.Doc(`
			Additional Options:
			  Run At: Now
			  Skipped Steps: None
			  Guided Failure Mode: Use default setting from the target environment
			  Package Download: Use cached packages (if available)
			  Run Targets: Include vm-2
			`), stdout.String())
		}},

		{"variation on exclude deployment targets only", func(t *testing.T, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsRunbookRun{
				ExcludeTargets: []string{"vm-4"},
			}
			run.PrintAdvancedSummary(stdout, options)

			assert.Equal(t, heredoc.Doc(`
			Additional Options:
			  Run At: Now
			  Skipped Steps: None
			  Guided Failure Mode: Use default setting from the target environment
			  Package Download: Use cached packages (if available)
			  Run Targets: Exclude vm-4
			`), stdout.String())
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.run(t, new(bytes.Buffer))
		})
	}
}
