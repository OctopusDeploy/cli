package deploy_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	surveyCore "github.com/AlecAivazis/survey/v2/core"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/release/deploy"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/executor"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"net/url"
	"testing"
	"time"
)

var serverUrl, _ = url.Parse("http://server")

const placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

var rootResource = testutil.NewRootResource()

var now = time.Date(2022, time.September, 8, 13, 25, 2, 0, time.FixedZone("Malaysia", 8*3600)) // UTC+8
var ctxWithFakeNow = context.WithValue(context.TODO(), constants.ContextKeyTimeNow, now)

func TestDeployCreate_AskQuestions(t *testing.T) {
	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	defaultChannel := fixtures.NewChannel(spaceID, "Channels-1", "Fire Project Default Channel", fireProjectID)
	altChannel := fixtures.NewChannel(spaceID, "Channels-97", "Fire Project Alt Channel", fireProjectID)

	fireProject := fixtures.NewProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "deploymentprocess-"+fireProjectID)

	fireProjectTenanted := fixtures.NewProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "deploymentprocess-"+fireProjectID)
	fireProjectTenanted.TenantedDeploymentMode = core.TenantedDeploymentModeTenanted

	fireProjectMaybeTenanted := fixtures.NewProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "deploymentprocess-"+fireProjectID)
	fireProjectMaybeTenanted.TenantedDeploymentMode = core.TenantedDeploymentModeTenantedOrUntenanted

	depProcessSnapshot := fixtures.NewDeploymentProcessForProject(spaceID, fireProjectID)
	depProcessSnapshot.ID = fmt.Sprintf("%s-s-0-2ZFWS", depProcessSnapshot.ID)
	depProcessSnapshot.Steps = []*deployments.DeploymentStep{
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

	release20 := fixtures.NewRelease(spaceID, "Releases-200", "2.0", fireProjectID, altChannel.ID)
	release20.ProjectDeploymentProcessSnapshotID = depProcessSnapshot.ID
	release20.ProjectVariableSetSnapshotID = variableSnapshotWithPromptedVariables.ID

	release19 := fixtures.NewRelease(spaceID, "Releases-193", "1.9", fireProjectID, altChannel.ID)
	release19.ProjectDeploymentProcessSnapshotID = depProcessSnapshot.ID
	release19.ProjectVariableSetSnapshotID = variableSnapshotNoVars.ID

	devEnvironment := fixtures.NewEnvironment(spaceID, "Environments-12", "dev")
	scratchEnvironment := fixtures.NewEnvironment(spaceID, "Environments-82", "scratch")
	prodEnvironment := fixtures.NewEnvironment(spaceID, "Environments-13", "production")

	cokeTenant := fixtures.NewTenant(spaceID, "Tenants-29", "Coke", "Regions/us-east", "Importance/High")
	cokeTenant.ProjectEnvironments = map[string][]string{
		fireProjectID: {devEnvironment.ID, prodEnvironment.ID},
	}
	pepsiTenant := fixtures.NewTenant(spaceID, "Tenants-37", "Pepsi", "Regions/us-east", "Importance/Low")
	pepsiTenant.ProjectEnvironments = map[string][]string{
		fireProjectID: {scratchEnvironment.ID},
	}

	// helper for advanced tests that want to skip past the first half of the questions
	doStandardApiResponses := func(options *executor.TaskOptionsDeployRelease, api *testutil.MockHttpServer, release *releases.Release, vars *variables.VariableSet) {
		if options.ReleaseVersion != release.Version {
			panic("you must set `options.ReleaseVersion` to match the supplied `release.Version`")
		}
		api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName="+url.QueryEscape(options.ProjectName)).
			RespondWith(resources.Resources[*projects.Project]{
				Items: []*projects.Project{fireProject},
			})

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/releases/"+release.Version).RespondWith(release)

		api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+vars.ID).RespondWith(&vars)
	}

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer)
	}{
		{"default process asking for standard things (non-tenanted, no advanced options)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsDeployRelease{}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return deploy.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{fireProject})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select project",
				Options: []string{"Fire Project"},
			}).AnswerWith("Fire Project")

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/channels").RespondWith(resources.Resources[*channels.Channel]{
				Items: []*channels.Channel{defaultChannel, altChannel},
			})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select channel",
				Options: []string{defaultChannel.Name, altChannel.Name},
			}).AnswerWith("Fire Project Alt Channel")

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/channels/"+altChannel.ID+"/releases").RespondWith(resources.Resources[*releases.Release]{
				Items: []*releases.Release{release20, release19},
			})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select a release to deploy",
				Options: []string{release20.Version, release19.Version},
			}).AnswerWith(release19.Version)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/releases/"+release19.ID+"/progression").RespondWith(&releases.LifecycleProgression{
				Phases: []*releases.LifecycleProgressionPhase{
					{Name: "Dev", Progress: releases.PhaseProgressCurrent, AutomaticDeploymentTargets: []string{scratchEnvironment.ID}, OptionalDeploymentTargets: []string{devEnvironment.ID}},
					{Name: "Prod", Progress: releases.PhaseProgressPending, OptionalDeploymentTargets: []string{prodEnvironment.ID}}, // should scope this out due to pending
				},
				NextDeployments: []string{devEnvironment.ID},
			})

			// now it needs to lookup the environment names
			api.ExpectRequest(t, "GET", fmt.Sprintf("/api/Spaces-1/environments?ids=%s%%2C%s", scratchEnvironment.ID, devEnvironment.ID)).RespondWith(resources.Resources[*environments.Environment]{
				Items: []*environments.Environment{scratchEnvironment, devEnvironment},
			})

			// Note: scratch comes first but default should be dev, due to NextDeployments
			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Select environment(s)",
				Options: []string{scratchEnvironment.Name, devEnvironment.Name},
				Default: []string{devEnvironment.Name},
			}).AnswerWith([]surveyCore.OptionAnswer{
				{Value: devEnvironment.Name, Index: 0},
			})

			// now it's going to go looking for prompted variables; we don't have any prompted variables here so it skips
			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshotNoVars.ID).RespondWith(&variableSnapshotNoVars)

			q := qa.ExpectQuestion(t, &survey.Select{
				Message: "Change additional options?",
				Options: []string{"Proceed to deploy", "Change"},
			})
			assert.Regexp(t, "Additional Options", stdout.String()) // actual options tested in PrintAdvancedSummary
			_ = q.AnswerWith("Proceed to deploy")

			err := <-errReceiver
			assert.Nil(t, err)

			// check that the question-asking process has filled out the things we told it to
			assert.Equal(t, &executor.TaskOptionsDeployRelease{
				ProjectName:       "Fire Project",
				ReleaseVersion:    "1.9",
				Environments:      []string{"dev"},
				GuidedFailureMode: "",
				Variables:         make(map[string]string, 0),
				ReleaseID:         release19.ID,
			}, options)
		}},

		{"default process picking up standard things from cmdline (non-tenanted, no advanced options)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsDeployRelease{
				ProjectName:    "fire project",
				ReleaseVersion: "1.9",
				Environments:   []string{"dev"},
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return deploy.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=fire+project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/releases/"+release19.Version).RespondWith(release19)

			// doesn't lookup the progression or env names because it already has them

			// now it's going to go looking for prompted variables; we don't have any prompted variables here so it skips
			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshotNoVars.ID).RespondWith(&variableSnapshotNoVars)

			assert.Equal(t, heredoc.Doc(`
				Project Fire Project
				Release 1.9
				Environments dev
			`), stdout.String())
			stdout.Reset()

			q := qa.ExpectQuestion(t, &survey.Select{
				Message: "Change additional options?",
				Options: []string{"Proceed to deploy", "Change"},
			})
			assert.Regexp(t, "Additional Options", stdout.String()) // actual options tested in PrintAdvancedSummary
			_ = q.AnswerWith("Proceed to deploy")

			err := <-errReceiver
			assert.Nil(t, err)

			// check that the question-asking process has filled out the things we told it to
			assert.Equal(t, &executor.TaskOptionsDeployRelease{
				ProjectName:       "Fire Project",
				ReleaseVersion:    "1.9",
				Environments:      []string{"dev"},
				GuidedFailureMode: "",
				Variables:         make(map[string]string, 0),
				ReleaseID:         release19.ID,
			}, options)
		}},

		{"prompted variable", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			// we don't need to fully test prompted variables; AskPromptedVariables already has all its own tests, we just
			// need to very it's wired up properly
			options := &executor.TaskOptionsDeployRelease{
				ProjectName:    "fire project",
				ReleaseVersion: "2.0",
				Environments:   []string{"dev"},
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return deploy.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=fire+project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/releases/"+release20.Version).RespondWith(release20)

			// now it's going to go looking for prompted variables; we don't have any prompted variables here so it skips
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
				Release 2.0
				Environments dev
			`), stdout.String())
			stdout.Reset()

			q = qa.ExpectQuestion(t, &survey.Select{
				Message: "Change additional options?",
				Options: []string{"Proceed to deploy", "Change"},
			})
			assert.Regexp(t, "Additional Options", stdout.String()) // actual options tested in PrintAdvancedSummary
			_ = q.AnswerWith("Proceed to deploy")

			err := <-errReceiver
			assert.Nil(t, err)

			// check that the question-asking process has filled out the things we told it to
			assert.Equal(t, &executor.TaskOptionsDeployRelease{
				ProjectName:       "Fire Project",
				ReleaseVersion:    "2.0",
				Environments:      []string{"dev"},
				GuidedFailureMode: "",
				Variables:         map[string]string{"Approver": "John"},
				ReleaseID:         release20.ID,
			}, options)
		}},

		{"tenants and tags in a definitely tenanted project", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsDeployRelease{
				ProjectName:    "fire project",
				ReleaseVersion: "1.9",
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return deploy.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=fire+project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProjectTenanted},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/releases/"+release19.Version).RespondWith(release19)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/releases/"+release19.ID+"/progression").RespondWith(&releases.LifecycleProgression{
				Phases: []*releases.LifecycleProgressionPhase{
					{Name: "Dev", Progress: releases.PhaseProgressCurrent, AutomaticDeploymentTargets: []string{scratchEnvironment.ID}, OptionalDeploymentTargets: []string{devEnvironment.ID}},
					{Name: "Prod", Progress: releases.PhaseProgressPending, OptionalDeploymentTargets: []string{prodEnvironment.ID}}, // should scope this out due to pending
				},
				NextDeployments: []string{devEnvironment.ID},
			})

			// now it needs to lookup the environment names
			api.ExpectRequest(t, "GET", fmt.Sprintf("/api/Spaces-1/environments?ids=%s%%2C%s", scratchEnvironment.ID, devEnvironment.ID)).RespondWith(resources.Resources[*environments.Environment]{
				Items: []*environments.Environment{scratchEnvironment, devEnvironment},
			})

			// Note: scratch comes first but default should be dev, due to NextDeployments
			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select environment to deploy to",
				Options: []string{scratchEnvironment.Name, devEnvironment.Name},
				Default: devEnvironment.Name,
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

			// now it's going to go looking for prompted variables; we don't have any prompted variables here so it skips
			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshotNoVars.ID).RespondWith(&variableSnapshotNoVars)

			assert.Equal(t, heredoc.Doc(`
				Project Fire Project
				Release 1.9
			`), stdout.String())
			stdout.Reset()

			q = qa.ExpectQuestion(t, &survey.Select{
				Message: "Change additional options?",
				Options: []string{"Proceed to deploy", "Change"},
			})
			assert.Regexp(t, "Additional Options", stdout.String()) // actual options tested in PrintAdvancedSummary
			_ = q.AnswerWith("Proceed to deploy")

			err := <-errReceiver
			assert.Nil(t, err)

			// check that the question-asking process has filled out the things we told it to
			assert.Equal(t, &executor.TaskOptionsDeployRelease{
				ProjectName:       "Fire Project",
				ReleaseVersion:    "1.9",
				Environments:      []string{"dev"},
				Tenants:           []string{"Coke"},
				TenantTags:        []string{"Regions/us-east"},
				GuidedFailureMode: "",
				Variables:         make(map[string]string, 0),
				ReleaseID:         release19.ID,
			}, options)
		}},

		{"tenants and tags in a maybe tenanted project (choosing tenanted)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsDeployRelease{
				ProjectName:    "fire project",
				ReleaseVersion: "1.9",
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return deploy.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=fire+project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProjectMaybeTenanted},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/releases/"+release19.Version).RespondWith(release19)

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select Tenanted or Untenanted deployment",
				Options: []string{"Tenanted", "Untenanted"},
			}).AnswerWith("Tenanted")

			// find environments via progression
			api.ExpectRequest(t, "GET", "/api/Spaces-1/releases/"+release19.ID+"/progression").RespondWith(&releases.LifecycleProgression{
				Phases: []*releases.LifecycleProgressionPhase{
					{Name: "Dev", Progress: releases.PhaseProgressCurrent, AutomaticDeploymentTargets: []string{scratchEnvironment.ID}, OptionalDeploymentTargets: []string{devEnvironment.ID}},
					{Name: "Prod", Progress: releases.PhaseProgressPending, OptionalDeploymentTargets: []string{prodEnvironment.ID}}, // should scope this out due to pending
				},
				NextDeployments: []string{devEnvironment.ID},
			})
			api.ExpectRequest(t, "GET", fmt.Sprintf("/api/Spaces-1/environments?ids=%s%%2C%s", scratchEnvironment.ID, devEnvironment.ID)).RespondWith(resources.Resources[*environments.Environment]{
				Items: []*environments.Environment{scratchEnvironment, devEnvironment},
			})

			// Note: scratch comes first but default should be dev, due to NextDeployments
			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select environment to deploy to",
				Options: []string{scratchEnvironment.Name, devEnvironment.Name},
				Default: devEnvironment.Name,
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

			// now it's going to go looking for prompted variables; we don't have any prompted variables here so it skips
			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshotNoVars.ID).RespondWith(&variableSnapshotNoVars)

			assert.Equal(t, heredoc.Doc(`
				Project Fire Project
				Release 1.9
			`), stdout.String())
			stdout.Reset()

			q = qa.ExpectQuestion(t, &survey.Select{
				Message: "Change additional options?",
				Options: []string{"Proceed to deploy", "Change"},
			})
			assert.Regexp(t, "Additional Options", stdout.String()) // actual options tested in PrintAdvancedSummary
			_ = q.AnswerWith("Proceed to deploy")

			err := <-errReceiver
			assert.Nil(t, err)

			// check that the question-asking process has filled out the things we told it to
			assert.Equal(t, &executor.TaskOptionsDeployRelease{
				ProjectName:       "Fire Project",
				ReleaseVersion:    "1.9",
				Environments:      []string{"dev"},
				Tenants:           []string{"Coke"},
				TenantTags:        []string{"Regions/us-east"},
				GuidedFailureMode: "",
				Variables:         make(map[string]string, 0),
				ReleaseID:         release19.ID,
			}, options)
		}},

		{"tenants and tags in a maybe tenanted project (choosing untenanted)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsDeployRelease{
				ProjectName:    "fire project",
				ReleaseVersion: "1.9",
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return deploy.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=fire+project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProjectMaybeTenanted},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/releases/"+release19.Version).RespondWith(release19)

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select Tenanted or Untenanted deployment",
				Options: []string{"Tenanted", "Untenanted"},
			}).AnswerWith("Untenanted")

			// find environments via progression
			api.ExpectRequest(t, "GET", "/api/Spaces-1/releases/"+release19.ID+"/progression").RespondWith(&releases.LifecycleProgression{
				Phases: []*releases.LifecycleProgressionPhase{
					{Name: "Dev", Progress: releases.PhaseProgressCurrent, AutomaticDeploymentTargets: []string{scratchEnvironment.ID}, OptionalDeploymentTargets: []string{devEnvironment.ID}},
					{Name: "Prod", Progress: releases.PhaseProgressPending, OptionalDeploymentTargets: []string{prodEnvironment.ID}}, // should scope this out due to pending
				},
				NextDeployments: []string{devEnvironment.ID},
			})
			api.ExpectRequest(t, "GET", fmt.Sprintf("/api/Spaces-1/environments?ids=%s%%2C%s", scratchEnvironment.ID, devEnvironment.ID)).RespondWith(resources.Resources[*environments.Environment]{
				Items: []*environments.Environment{scratchEnvironment, devEnvironment},
			})

			// Note: scratch comes first but default should be dev, due to NextDeployments
			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Select environment(s)",
				Options: []string{scratchEnvironment.Name, devEnvironment.Name},
				Default: []string{devEnvironment.Name},
			}).AnswerWith([]surveyCore.OptionAnswer{
				{Value: devEnvironment.Name, Index: 0},
			})

			// now it's going to go looking for prompted variables; we don't have any prompted variables here so it skips
			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshotNoVars.ID).RespondWith(&variableSnapshotNoVars)
			assert.Equal(t, heredoc.Doc(`
				Project Fire Project
				Release 1.9
			`), stdout.String())
			stdout.Reset()

			q := qa.ExpectQuestion(t, &survey.Select{
				Message: "Change additional options?",
				Options: []string{"Proceed to deploy", "Change"},
			})
			assert.Regexp(t, "Additional Options", stdout.String()) // actual options tested in PrintAdvancedSummary
			_ = q.AnswerWith("Proceed to deploy")

			err := <-errReceiver
			assert.Nil(t, err)

			// check that the question-asking process has filled out the things we told it to
			assert.Equal(t, &executor.TaskOptionsDeployRelease{
				ProjectName:       "Fire Project",
				ReleaseVersion:    "1.9",
				Environments:      []string{"dev"},
				GuidedFailureMode: "",
				Variables:         make(map[string]string, 0),
				ReleaseID:         release19.ID,
			}, options)
		}},

		{"advanced options", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsDeployRelease{ProjectName: "fire project", ReleaseVersion: "1.9", Environments: []string{"dev", "scratch"}}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return deploy.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			doStandardApiResponses(options, api, release19, variableSnapshotNoVars)
			stdout.Reset()

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Change additional options?",
				Options: []string{"Proceed to deploy", "Change"},
			}).AnswerWith("Change")
			stdout.Reset()

			plus20hours := now.Add(20 * time.Hour)
			_ = qa.ExpectQuestion(t, &surveyext.DatePicker{
				Message: "Scheduled start time",
				Default: now,
				Help:    "Enter the date and time that this deployment should start",
				Min:     now,
			}).AnswerWith(plus20hours)

			plus20hours5mins := plus20hours.Add(5 * time.Minute)
			_ = qa.ExpectQuestion(t, &surveyext.DatePicker{
				Message: "Scheduled expiry time",
				Default: plus20hours5mins,
				Help:    "At the start time, the deployment will be queued. If it does not begin before 'expiry' time, it will be cancelled. Minimum of 5 minutes after start time",
				Min:     plus20hours5mins,
			}).AnswerWith(plus20hours5mins)

			// it's going to load the deployment process to ask about excluded steps
			api.ExpectRequest(t, "GET", "/api/Spaces-1/deploymentprocesses/"+depProcessSnapshot.ID).RespondWith(depProcessSnapshot)

			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Select steps to skip (optional)",
				Options: []string{"Install", "Cleanup"},
			}).AnswerWith([]string{"Cleanup"})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Guided Failure Mode?",
				Options: []string{"Use default setting from the target environment", "Use guided failure mode", "Do not use guided failure mode"},
			}).AnswerWith("Do not use guided failure mode")

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Package download",
				Options: []string{"Use cached packages (if available)", "Re-download packages from feed"},
			}).AnswerWith("Re-download packages from feed")

			// because environments were specified on the commandline, we didn't look them up earlier, but we
			// must do it now in order to determine the list of deployment targets
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments/all").RespondWith([]*environments.Environment{
				devEnvironment, scratchEnvironment, prodEnvironment,
			})

			api.ExpectRequest(t, "GET", fmt.Sprintf("/api/Spaces-1/releases/%s/deployments/preview/%s?includeDisabledSteps=true", release19.ID, devEnvironment.ID)).RespondWith(&deployments.DeploymentPreview{
				StepsToExecute: []*deployments.DeploymentTemplateStep{
					{},
					{MachineNames: []string{"vm-1", "vm-2"}},
					{MachineNames: []string{"vm-4"}},
				},
			})
			api.ExpectRequest(t, "GET", fmt.Sprintf("/api/Spaces-1/releases/%s/deployments/preview/%s?includeDisabledSteps=true", release19.ID, scratchEnvironment.ID)).RespondWith(&deployments.DeploymentPreview{
				StepsToExecute: []*deployments.DeploymentTemplateStep{
					{MachineNames: []string{"vm-2"}}, // deliberate double up
					{MachineNames: []string{"vm-2", "vm-5"}},
				},
			})

			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Restrict to specific deployment targets (optional)",
				Options: []string{"vm-1", "vm-2", "vm-4", "vm-5"},
			}).AnswerWith([]string{"vm-1", "vm-2"})

			err := <-errReceiver
			assert.Nil(t, err)

			// check that the question-asking process has filled out the things we told it to
			assert.Equal(t, &executor.TaskOptionsDeployRelease{
				ProjectName:          "Fire Project",
				ReleaseVersion:       "1.9",
				Environments:         []string{"dev", "scratch"},
				GuidedFailureMode:    "false",
				ForcePackageDownload: true,
				Variables:            make(map[string]string, 0),
				ExcludedSteps:        []string{"Cleanup"},
				DeploymentTargets:    []string{"vm-1", "vm-2"},
				ReleaseID:            release19.ID,
				ScheduledStartTime:   "2022-09-09T09:25:02+08:00", // Important to note it's in ISO8601 which .NET on the server can parse with DateTimeOffset.Parse
				ScheduledExpiryTime:  "2022-09-09T09:30:02+08:00", // Important to note it's in ISO8601 which .NET on the server can parse with DateTimeOffset.Parse
			}, options)
		}},

		{"advanced options doesn't need to lookup environments if the Q&A process already asked for them", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsDeployRelease{
				ProjectName:                      "fire project",
				ReleaseVersion:                   "1.9",
				ExcludedSteps:                    []string{"Cleanup"},
				GuidedFailureMode:                "false",
				ForcePackageDownloadWasSpecified: true,
				ScheduledStartTime:               "some-sort-of-garbage(passthru to server)",
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return deploy.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName="+url.QueryEscape(options.ProjectName)).
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/releases/"+release19.Version).RespondWith(release19)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/releases/"+release19.ID+"/progression").RespondWith(&releases.LifecycleProgression{
				Phases: []*releases.LifecycleProgressionPhase{
					{Name: "Dev", Progress: releases.PhaseProgressCurrent, OptionalDeploymentTargets: []string{devEnvironment.ID, prodEnvironment.ID}},
				},
				NextDeployments: []string{devEnvironment.ID},
			})

			// now it needs to lookup the environment names
			api.ExpectRequest(t, "GET", fmt.Sprintf("/api/Spaces-1/environments?ids=%s%%2C%s", devEnvironment.ID, prodEnvironment.ID)).RespondWith(resources.Resources[*environments.Environment]{
				Items: []*environments.Environment{devEnvironment, prodEnvironment},
			})

			// Note: scratch comes first but default should be dev, due to NextDeployments
			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Select environment(s)",
				Options: []string{devEnvironment.Name, prodEnvironment.Name},
				Default: []string{devEnvironment.Name},
			}).AnswerWith([]surveyCore.OptionAnswer{
				{Value: devEnvironment.Name, Index: 0},
			})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshotNoVars.ID).RespondWith(&variableSnapshotNoVars)

			stdout.Reset()

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Change additional options?",
				Options: []string{"Proceed to deploy", "Change"},
			}).AnswerWith("Change")
			stdout.Reset()

			// steps, guidedFailure and forcePackageDownload already on cmdline, so we go straight to targets

			// NOTE there is NO CALL to environments.all here, because we already have info loaded for the selected environment (devEnvironment)

			api.ExpectRequest(t, "GET", fmt.Sprintf("/api/Spaces-1/releases/%s/deployments/preview/%s?includeDisabledSteps=true", release19.ID, devEnvironment.ID)).RespondWith(&deployments.DeploymentPreview{
				StepsToExecute: []*deployments.DeploymentTemplateStep{
					{},
					{MachineNames: []string{"vm-1", "vm-2"}},
					{MachineNames: []string{"vm-4"}},
				},
			})
			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Restrict to specific deployment targets (optional)",
				Options: []string{"vm-1", "vm-2", "vm-4"},
			}).AnswerWith([]string{"vm-1"})

			err := <-errReceiver
			assert.Nil(t, err)

			// check that the question-asking process has filled out the things we told it to
			assert.Equal(t, &executor.TaskOptionsDeployRelease{
				ProjectName:                      "Fire Project",
				ReleaseVersion:                   "1.9",
				Environments:                     []string{"dev"},
				GuidedFailureMode:                "false",
				ForcePackageDownload:             false,
				ForcePackageDownloadWasSpecified: true,
				Variables:                        make(map[string]string, 0),
				ExcludedSteps:                    []string{"Cleanup"},
				DeploymentTargets:                []string{"vm-1"},
				ReleaseID:                        release19.ID,
				ScheduledStartTime:               "some-sort-of-garbage(passthru to server)",
			}, options)

		}},

		{"advanced options pickup from command line; doesn't ask if all opts are supplied", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsDeployRelease{
				ProjectName:                      "fire project",
				ReleaseVersion:                   "1.9",
				Environments:                     []string{"dev"},
				ExcludedSteps:                    []string{"Cleanup"},
				GuidedFailureMode:                "false",
				ForcePackageDownload:             true,
				ForcePackageDownloadWasSpecified: true, // need this as well
				ExcludeTargets:                   []string{"vm-99"},
				ScheduledStartTime:               "some-sort-of-garbage(passthru to server)",
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return deploy.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			doStandardApiResponses(options, api, release19, variableSnapshotNoVars)
			stdout.Reset()

			err := <-errReceiver
			assert.Nil(t, err)

			// check that the question-asking process has filled out the things we told it to
			assert.Equal(t, &executor.TaskOptionsDeployRelease{
				ProjectName:                      "Fire Project",
				ReleaseVersion:                   "1.9",
				Environments:                     []string{"dev"},
				GuidedFailureMode:                "false",
				ForcePackageDownload:             true,
				ForcePackageDownloadWasSpecified: true,
				Variables:                        make(map[string]string, 0),
				ExcludedSteps:                    []string{"Cleanup"},
				ExcludeTargets:                   []string{"vm-99"},
				ReleaseID:                        release19.ID,
				ScheduledStartTime:               "some-sort-of-garbage(passthru to server)",
			}, options)
		}},

		{"advanced options pickup from command line; explicit default values", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsDeployRelease{
				ProjectName:                      "fire project",
				ReleaseVersion:                   "1.9",
				Environments:                     []string{"dev"},
				ExcludedSteps:                    []string{"Cleanup"},
				GuidedFailureMode:                "default",
				ForcePackageDownload:             false,
				ForcePackageDownloadWasSpecified: true,
				ExcludeTargets:                   []string{"vm-99"}, // just to skip the question
				ScheduledStartTime:               now.String(),
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return deploy.AskQuestions(octopus, stdout, qa.AsAsker(), space1, options, now)
			})

			doStandardApiResponses(options, api, release19, variableSnapshotNoVars)
			stdout.Reset()

			err := <-errReceiver
			assert.Nil(t, err)

			// check that the question-asking process has filled out the things we told it to
			assert.Equal(t, &executor.TaskOptionsDeployRelease{
				ProjectName:                      "Fire Project",
				ReleaseVersion:                   "1.9",
				Environments:                     []string{"dev"},
				GuidedFailureMode:                "default",
				ForcePackageDownload:             false,
				ForcePackageDownloadWasSpecified: true,
				Variables:                        make(map[string]string, 0),
				ExcludedSteps:                    []string{"Cleanup"},
				ExcludeTargets:                   []string{"vm-99"},
				ReleaseID:                        release19.ID,
				ScheduledStartTime:               "2022-09-08 13:25:02 +0800 Malaysia",
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

// These tests ensure that given the right input, we call the server's API appropriately
// they all run in automation mode where survey is disabled; they'd error if they tried to ask questions
func TestDeployCreate_AutomationMode(t *testing.T) {
	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	defaultChannel := fixtures.NewChannel(spaceID, "Channels-1", "Fire Project Default Channel", fireProjectID)

	fireProject := fixtures.NewProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "deploymentprocess-"+fireProjectID)
	//
	//
	release10 := fixtures.NewRelease(spaceID, "Releases-200", "2.0", fireProjectID, defaultChannel.ID)
	////release20.ProjectDeploymentProcessSnapshotID = depProcessSnapshot.ID
	//release20.ProjectVariableSetSnapshotID = variableSnapshotWithPromptedVariables.ID
	//
	//devEnvironment := fixtures.NewEnvironment(spaceID, "Environments-12", "dev")

	// TEST STARTS HERE
	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"release deploy requires a project name", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "deploy"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "project must be specified")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"release deploy requires a release version", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "deploy", "--project", "Fire Project"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "release version must be specified")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"release deploy requires at least one environment", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "deploy", "--project", "Fire Project", "--version", "1.9"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "environment(s) must be specified")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"release deploy specifying project, version, env only (bare minimum) assuming untenanted", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "deploy", "--project", fireProject.Name, "--version", "1.0", "--environment", "dev"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			// Note: because we didn't specify --tenant or --tenant-tag, automation-mode code is going to assume untenanted
			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/deployments/create/untenanted/v1")
			requestBody, err := testutil.ReadJson[deployments.CreateDeploymentUntenantedCommandV1](req.Request.Body)
			assert.Nil(t, err)

			assert.Equal(t, deployments.CreateDeploymentUntenantedCommandV1{
				ReleaseVersion:   "1.0",
				EnvironmentNames: []string{"dev"},
				CreateExecutionAbstractCommandV1: deployments.CreateExecutionAbstractCommandV1{
					SpaceID:         "Spaces-1",
					ProjectIDOrName: fireProject.Name,
				},
			}, requestBody)

			req.RespondWith(&deployments.CreateDeploymentResponseV1{
				DeploymentServerTasks: []*deployments.DeploymentServerTask{
					{DeploymentID: "Deployments-203", ServerTaskID: "ServerTasks-29394"},
					{DeploymentID: "Deployments-204", ServerTaskID: "ServerTasks-55312"},
				},
			})

			// now it's going to try and look up the project/version to generate the web URL
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=Fire+Project").RespondWith(resources.Resources[*projects.Project]{
				Items: []*projects.Project{fireProject},
			})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/releases/1.0").RespondWith(release10)

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Docf(`
				Successfully started 2 deployment(s)
				
				View this release on Octopus Deploy: http://server/app#/Spaces-1/releases/%s
				`, release10.ID), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"release deploy specifying project, version, env only (bare minimum) assuming untenanted; basic output format", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "deploy", "--project", fireProject.Name, "--version", "1.0", "--environment", "dev", "--output-format", constants.OutputFormatBasic})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			// Note: because we didn't specify --tenant or --tenant-tag, automation-mode code is going to assume untenanted
			api.ExpectRequest(t, "POST", "/api/Spaces-1/deployments/create/untenanted/v1").RespondWith(&deployments.CreateDeploymentResponseV1{
				DeploymentServerTasks: []*deployments.DeploymentServerTask{
					{DeploymentID: "Deployments-203", ServerTaskID: "ServerTasks-29394"},
					{DeploymentID: "Deployments-204", ServerTaskID: "ServerTasks-55312"},
				},
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				ServerTasks-29394
				ServerTasks-55312
				`), stdOut.String())

			assert.Equal(t, "", stdErr.String())
		}},

		{"release deploy specifying project, version, env only (bare minimum) assuming untenanted; json output format", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "deploy", "--project", fireProject.Name, "--version", "1.0", "--environment", "dev", "--output-format", constants.OutputFormatJson})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			// Note: because we didn't specify --tenant or --tenant-tag, automation-mode code is going to assume untenanted
			serverTasks := []*deployments.DeploymentServerTask{
				{DeploymentID: "Deployments-203", ServerTaskID: "ServerTasks-29394"},
				{DeploymentID: "Deployments-204", ServerTaskID: "ServerTasks-55312"},
			}
			api.ExpectRequest(t, "POST", "/api/Spaces-1/deployments/create/untenanted/v1").RespondWith(&deployments.CreateDeploymentResponseV1{
				DeploymentServerTasks: serverTasks,
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			var response []*deployments.DeploymentServerTask
			err = json.Unmarshal(stdOut.Bytes(), &response)
			assert.Nil(t, err)

			assert.Equal(t, serverTasks, response)
			assert.Equal(t, "", stdErr.String())
		}},

		{"release deploy specifying project, version, env only (bare minimum) assuming tenanted", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "deploy", "--project", fireProject.Name, "--version", "1.0", "--environment", "dev", "--tenant", "Coke", "--tenant", "Pepsi"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/deployments/create/tenanted/v1")
			requestBody, err := testutil.ReadJson[deployments.CreateDeploymentTenantedCommandV1](req.Request.Body)
			assert.Nil(t, err)

			assert.Equal(t, deployments.CreateDeploymentTenantedCommandV1{
				ReleaseVersion:  "1.0",
				EnvironmentName: "dev",
				Tenants:         []string{"Coke", "Pepsi"},
				CreateExecutionAbstractCommandV1: deployments.CreateExecutionAbstractCommandV1{
					SpaceID:         "Spaces-1",
					ProjectIDOrName: fireProject.Name,
				},
			}, requestBody)

			req.RespondWith(&deployments.CreateDeploymentResponseV1{
				DeploymentServerTasks: []*deployments.DeploymentServerTask{
					{DeploymentID: "Deployments-203", ServerTaskID: "ServerTasks-29394"},
				},
			})

			// now it's going to try and look up the project/version to generate the web URL
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=Fire+Project").RespondWith(resources.Resources[*projects.Project]{
				Items: []*projects.Project{fireProject},
			})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/releases/1.0").RespondWith(release10)

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Docf(`
				Successfully started 1 deployment(s)
				
				View this release on Octopus Deploy: http://server/app#/Spaces-1/releases/%s
				`, release10.ID), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"release deploy specifying project, version, env only (bare minimum) assuming tenanted via tags", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "deploy", "--project", fireProject.Name, "--version", "1.0", "--environment", "dev", "--tenant-tag", "Regions/us-west", "--tenant-tag", "Importance/High"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/deployments/create/tenanted/v1")
			requestBody, err := testutil.ReadJson[deployments.CreateDeploymentTenantedCommandV1](req.Request.Body)
			assert.Nil(t, err)

			assert.Equal(t, deployments.CreateDeploymentTenantedCommandV1{
				ReleaseVersion:  "1.0",
				EnvironmentName: "dev",
				TenantTags:      []string{"Regions/us-west", "Importance/High"},
				CreateExecutionAbstractCommandV1: deployments.CreateExecutionAbstractCommandV1{
					SpaceID:         "Spaces-1",
					ProjectIDOrName: fireProject.Name,
				},
			}, requestBody)

			req.RespondWith(&deployments.CreateDeploymentResponseV1{
				DeploymentServerTasks: []*deployments.DeploymentServerTask{
					{DeploymentID: "Deployments-203", ServerTaskID: "ServerTasks-29394"},
				},
			})

			// now it's going to try and look up the project/version to generate the web URL
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=Fire+Project").RespondWith(resources.Resources[*projects.Project]{
				Items: []*projects.Project{fireProject},
			})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/releases/1.0").RespondWith(release10)

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Docf(`
				Successfully started 1 deployment(s)
				
				View this release on Octopus Deploy: http://server/app#/Spaces-1/releases/%s
				`, release10.ID), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"release deploy specifying all the args; untentanted", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{
					"release", "deploy",
					"--project", fireProject.Name,
					"--version", "1.0",
					"--environment", "dev", "--environment", "test",
					"--deploy-at", "2022-09-10 13:32:03 +10:00",
					"--deploy-at-expiry", "2022-09-10 13:37:03 +10:00",
					"--skip", "Install",
					"--skip", "Cleanup",
					"--guided-failure", "true",
					"--force-package-download",
					"--update-variables",
					"--target", "firstMachine", "--target", "secondMachine",
					"--exclude-target", "thirdMachine",
					"--variable", "Approver:John", "--variable", "Signoff:Jane",
					"--output-format", "basic", // not neccessary, just means we don't need the follow up HTTP requests at the end to print the web link
				})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			// Note: because we didn't specify --tenant or --tenant-tag, automation-mode code is going to assume untenanted
			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/deployments/create/untenanted/v1")
			requestBody, err := testutil.ReadJson[deployments.CreateDeploymentUntenantedCommandV1](req.Request.Body)
			assert.Nil(t, err)

			trueVal := true
			assert.Equal(t, deployments.CreateDeploymentUntenantedCommandV1{
				ReleaseVersion:           "1.0",
				EnvironmentNames:         []string{"dev", "test"},
				ForcePackageRedeployment: true,
				UpdateVariableSnapshot:   true,
				CreateExecutionAbstractCommandV1: deployments.CreateExecutionAbstractCommandV1{
					SpaceID:              "Spaces-1",
					ProjectIDOrName:      fireProject.Name,
					ForcePackageDownload: true,
					SpecificMachineNames: []string{"firstMachine", "secondMachine"},
					ExcludedMachineNames: []string{"thirdMachine"},
					SkipStepNames:        []string{"Install", "Cleanup"},
					UseGuidedFailure:     &trueVal,
					RunAt:                "2022-09-10 13:32:03 +10:00",
					NoRunAfter:           "2022-09-10 13:37:03 +10:00",
					Variables: map[string]string{
						"Approver": "John",
						"Signoff":  "Jane",
					},
				},
			}, requestBody)

			req.RespondWith(&deployments.CreateDeploymentResponseV1{
				DeploymentServerTasks: []*deployments.DeploymentServerTask{
					{DeploymentID: "Deployments-203", ServerTaskID: "ServerTasks-29394"},
				},
			})

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, "ServerTasks-29394\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"release deploy specifying all the args; tentanted", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{
					"release", "deploy",
					"--project", fireProject.Name,
					"--version", "1.0",
					"--environment", "dev",
					"--deploy-at", "2022-09-10 13:32:03 +10:00",
					"--deploy-at-expiry", "2022-09-10 13:37:03 +10:00",
					"--skip", "Install",
					"--skip", "Cleanup",
					"--tenant", "Coke", "--tenant", "Pepsi",
					"--tenant-tag", "Region/us-east",
					"--guided-failure", "true",
					"--force-package-download",
					"--update-variables",
					"--target", "firstMachine", "--target", "secondMachine",
					"--exclude-target", "thirdMachine",
					"--variable", "Approver:John", "--variable", "Signoff:Jane",
					"--output-format", "basic", // not neccessary, just means we don't need the follow up HTTP requests at the end to print the web link
				})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/deployments/create/tenanted/v1")
			requestBody, err := testutil.ReadJson[deployments.CreateDeploymentTenantedCommandV1](req.Request.Body)
			assert.Nil(t, err)

			trueVal := true
			assert.Equal(t, deployments.CreateDeploymentTenantedCommandV1{
				ReleaseVersion:           "1.0",
				EnvironmentName:          "dev",
				ForcePackageRedeployment: true,
				UpdateVariableSnapshot:   true,
				Tenants:                  []string{"Coke", "Pepsi"},
				TenantTags:               []string{"Region/us-east"},
				CreateExecutionAbstractCommandV1: deployments.CreateExecutionAbstractCommandV1{
					SpaceID:              "Spaces-1",
					ProjectIDOrName:      fireProject.Name,
					ForcePackageDownload: true,
					SpecificMachineNames: []string{"firstMachine", "secondMachine"},
					ExcludedMachineNames: []string{"thirdMachine"},
					SkipStepNames:        []string{"Install", "Cleanup"},
					UseGuidedFailure:     &trueVal,
					RunAt:                "2022-09-10 13:32:03 +10:00",
					NoRunAfter:           "2022-09-10 13:37:03 +10:00",
					Variables: map[string]string{
						"Approver": "John",
						"Signoff":  "Jane",
					},
				},
			}, requestBody)

			req.RespondWith(&deployments.CreateDeploymentResponseV1{
				DeploymentServerTasks: []*deployments.DeploymentServerTask{
					{DeploymentID: "Deployments-203", ServerTaskID: "ServerTasks-29394"},
				},
			})

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, "ServerTasks-29394\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			api := testutil.NewMockHttpServer()

			rootCmd := cmdRoot.NewCmdRoot(testutil.NewMockFactoryWithSpace(api, space1), nil, nil)
			rootCmd.SetContext(ctxWithFakeNow)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)

			test.run(t, api, rootCmd, stdout, stderr)
		})
	}
}

// this happens outside the scope of the normal AskQuestions flow so warrants its own integration-style test
func TestDeployCreate_GenerationOfAutomationCommand_MasksSensitiveVariables(t *testing.T) {
	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	defaultChannel := fixtures.NewChannel(spaceID, "Channels-1", "Fire Project Default Channel", fireProjectID)

	fireProject := fixtures.NewProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "deploymentprocess-"+fireProjectID)

	variableSnapshotWithPromptedVariables := fixtures.NewVariableSetForProject(spaceID, fireProjectID)
	variableSnapshotWithPromptedVariables.ID = fmt.Sprintf("%s-s-0-9BZ22", variableSnapshotWithPromptedVariables.ID)
	variableSnapshotWithPromptedVariables.Variables = []*variables.Variable{
		{
			Name: "Boring Variable",
			Prompt: &variables.VariablePromptOptions{
				IsRequired: true,
			},
			Type: "String",
		},
		{
			Name: "Nuclear Launch Codes",
			Prompt: &variables.VariablePromptOptions{
				IsRequired: true,
			},
			Type: "Sensitive",
		},
		{
			Name: "Secret Password",
			Prompt: &variables.VariablePromptOptions{
				IsRequired: true,
			},
			IsSensitive: true, // old way
			Type:        "String",
		},
	}

	release20 := fixtures.NewRelease(spaceID, "Releases-200", "2.0", fireProjectID, defaultChannel.ID)
	//release20.ProjectDeploymentProcessSnapshotID = depProcessSnapshot.ID
	release20.ProjectVariableSetSnapshotID = variableSnapshotWithPromptedVariables.ID
	//
	//devEnvironment := fixtures.NewEnvironment(spaceID, "Environments-12", "dev")

	// TEST STARTS HERE

	api, qa := testutil.NewMockServerAndAsker()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	askProvider := question.NewAskProvider(qa.AsAsker())

	rootCmd := cmdRoot.NewCmdRoot(testutil.NewMockFactoryWithSpaceAndPrompt(api, space1, askProvider), nil, askProvider)
	rootCmd.SetContext(ctxWithFakeNow)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	// we don't need to fully test prompted variables; AskPromptedVariables already has all its own tests, we just
	// need to very it's wired up properly
	receiver := testutil.GoBegin2(func() (*cobra.Command, error) {
		defer testutil.Close(api, qa)
		rootCmd.SetArgs([]string{"release", "deploy", "--project", "fire project", "--version", "2.0", "--environment", "dev"})
		return rootCmd.ExecuteC()
	})

	api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

	api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=fire+project").
		RespondWith(resources.Resources[*projects.Project]{
			Items: []*projects.Project{fireProject},
		})

	api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/releases/"+release20.Version).RespondWith(release20)

	// now it's going to go looking for prompted variables; we don't have any prompted variables here so it skips
	api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshotWithPromptedVariables.ID).RespondWith(&variableSnapshotWithPromptedVariables)

	_ = qa.ExpectQuestion(t, &survey.Input{
		Message: "Boring Variable",
	}).AnswerWith("BORING")

	_ = qa.ExpectQuestion(t, &survey.Password{
		Message: "Nuclear Launch Codes",
	}).AnswerWith("9001")

	_ = qa.ExpectQuestion(t, &survey.Password{
		Message: "Secret Password",
	}).AnswerWith("donkey")

	q := qa.ExpectQuestion(t, &survey.Select{
		Message: "Change additional options?",
		Options: []string{"Proceed to deploy", "Change"},
	})
	_ = q.AnswerWith("Proceed to deploy")

	req := api.ExpectRequest(t, "POST", "/api/Spaces-1/deployments/create/untenanted/v1")

	// check that it sent the server the right request body
	requestBody, err := testutil.ReadJson[deployments.CreateDeploymentUntenantedCommandV1](req.Request.Body)
	assert.Nil(t, err)

	assert.Equal(t, deployments.CreateDeploymentUntenantedCommandV1{
		ReleaseVersion:   "2.0",
		EnvironmentNames: []string{"dev"},
		CreateExecutionAbstractCommandV1: deployments.CreateExecutionAbstractCommandV1{
			SpaceID:         "Spaces-1",
			ProjectIDOrName: fireProject.Name,
			Variables: map[string]string{
				"Boring Variable":      "BORING",
				"Nuclear Launch Codes": "9001",
				"Secret Password":      "donkey",
			},
		},
	}, requestBody)

	req.RespondWith(&deployments.CreateDeploymentResponseV1{
		DeploymentServerTasks: []*deployments.DeploymentServerTask{
			{DeploymentID: "Deployments-1", ServerTaskID: "Tasks-100"},
			{DeploymentID: "Deployments-2", ServerTaskID: "Tasks-101"},
		},
	})

	_, err = testutil.ReceivePair(receiver)
	assert.Nil(t, err)

	assert.Equal(t, heredoc.Doc(`
		Project Fire Project
		Release 2.0
		Environments dev
		Additional Options:
		  Deploy Time: Now
		  Skipped Steps: None
		  Guided Failure Mode: Use default setting from the target environment
		  Package Download: Use cached packages (if available)
		  Deployment Targets: All included
		
		Automation Command: octopus release deploy --project 'Fire Project' --version '2.0' --environment 'dev' --variable 'Boring Variable:BORING' --variable 'Nuclear Launch Codes:*****' --variable 'Secret Password:*****' --no-prompt
		Warning: Command includes some sensitive variable values which have been replaced with placeholders.
		Successfully started 2 deployment(s)

		View this release on Octopus Deploy: http://server/app#/Spaces-1/releases/Releases-200
		`), stdout.String())
	assert.Equal(t, "", stderr.String())
}

func TestDeployCreate_PrintAdvancedSummary(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, stdout *bytes.Buffer)
	}{
		{"default state", func(t *testing.T, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsDeployRelease{}
			deploy.PrintAdvancedSummary(stdout, options)

			assert.Equal(t, heredoc.Doc(`
			Additional Options:
			  Deploy Time: Now
			  Skipped Steps: None
			  Guided Failure Mode: Use default setting from the target environment
			  Package Download: Use cached packages (if available)
			  Deployment Targets: All included
			`), stdout.String())
		}},

		{"all the things different", func(t *testing.T, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsDeployRelease{
				ScheduledStartTime:   "2022-09-23",
				GuidedFailureMode:    "false",
				ForcePackageDownload: true,
				ExcludedSteps:        []string{"Step 1", "Step 37"},
				DeploymentTargets:    []string{"vm-1", "vm-2"},
				ExcludeTargets:       []string{"vm-3", "vm-4"},
			}
			deploy.PrintAdvancedSummary(stdout, options)

			assert.Equal(t, heredoc.Doc(`
			Additional Options:
			  Deploy Time: 2022-09-23
			  Skipped Steps: Step 1,Step 37
			  Guided Failure Mode: Do not use guided failure mode
			  Package Download: Re-download packages from feed
			  Deployment Targets: Include vm-1,vm-2; Exclude vm-3,vm-4
			`), stdout.String())
		}},

		{"variation on include deployment targets only", func(t *testing.T, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsDeployRelease{
				DeploymentTargets: []string{"vm-2"},
			}
			deploy.PrintAdvancedSummary(stdout, options)

			assert.Equal(t, heredoc.Doc(`
			Additional Options:
			  Deploy Time: Now
			  Skipped Steps: None
			  Guided Failure Mode: Use default setting from the target environment
			  Package Download: Use cached packages (if available)
			  Deployment Targets: Include vm-2
			`), stdout.String())
		}},

		{"variation on exclude deployment targets only", func(t *testing.T, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsDeployRelease{
				ExcludeTargets: []string{"vm-4"},
			}
			deploy.PrintAdvancedSummary(stdout, options)

			assert.Equal(t, heredoc.Doc(`
			Additional Options:
			  Deploy Time: Now
			  Skipped Steps: None
			  Guided Failure Mode: Use default setting from the target environment
			  Package Download: Use cached packages (if available)
			  Deployment Targets: Exclude vm-4
			`), stdout.String())
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.run(t, new(bytes.Buffer))
		})
	}
}

func TestDeployCreate_AskVariables(t *testing.T) {
	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"

	makeVariables := func(variables ...*variables.Variable) *variables.VariableSet {
		vars := fixtures.NewVariableSetForProject(spaceID, fireProjectID)
		vars.ID = fmt.Sprintf("%s-s-0-2ZFWS", vars.ID)
		vars.Variables = variables
		return vars
	}

	tests := []struct {
		name string
		run  func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer)
	}{
		{"doesn't do anything if there are no variables", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			output, err := deploy.AskVariables(qa.AsAsker(), makeVariables(), make(map[string]string, 0))
			assert.Nil(t, err)
			assert.Equal(t, make(map[string]string, 0), output)
		}},

		{"variablesFromCmd are filtered and normalized against the server", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			serverVars := makeVariables(variables.NewVariable("Foo"))
			cmdlineVars := map[string]string{"foO": "bar", "doesntexist": "value"}

			output, err := deploy.AskVariables(qa.AsAsker(), serverVars, cmdlineVars)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"Foo": "bar"}, output)
		}},

		{"prompts for a single line text", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("SomeText")
			v1.Prompt = &variables.VariablePromptOptions{
				Description:     "Enter some text",
				DisplaySettings: nil,
				IsRequired:      false,
				Label:           "ignored",
			}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return deploy.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			_ = qa.ExpectQuestion(t, &survey.Input{
				Message: "SomeText (Enter some text)",
				Default: "",
			}).AnswerWith("Some Value")

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"SomeText": "Some Value"}, output)
		}},

		{"single line text with default value", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("SomeText")
			v1.Value = "Some Default Value"
			v1.Prompt = &variables.VariablePromptOptions{
				Description:     "Enter some text",
				DisplaySettings: nil,
				IsRequired:      false,
				Label:           "ignored",
			}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return deploy.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			_ = qa.ExpectQuestion(t, &survey.Input{
				Message: "SomeText (Enter some text)",
				Default: "Some Default Value",
			}).AnswerWith("Some Default Value")

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"SomeText": "Some Default Value"}, output)
		}},

		{"prompts for a single line text with explicit display settings and required", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("ReqText")
			v1.Prompt = &variables.VariablePromptOptions{
				Description:     "Enter required text",
				DisplaySettings: variables.NewDisplaySettings(variables.ControlTypeSingleLineText, nil),
				IsRequired:      true,
			}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return deploy.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			q := qa.ExpectQuestion(t, &survey.Input{Message: "ReqText (Enter required text)", Default: ""})
			validationErr := q.AnswerWith("")
			assert.EqualError(t, validationErr, "Value is required")

			validationErr = q.AnswerWith("A value")
			assert.Nil(t, validationErr)

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"ReqText": "A value"}, output)
		}},

		{"prompts for a sensitive value (isSensitive on variable declaration)", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("SomeText")
			v1.IsSensitive = true
			v1.Prompt = &variables.VariablePromptOptions{Description: "Enter secret text"}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return deploy.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			_ = qa.ExpectQuestion(t, &survey.Password{
				Message: "SomeText (Enter secret text)",
			}).AnswerWith("secretsquirrel")

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"SomeText": "secretsquirrel"}, output)
		}},

		{"prompts for a sensitive value (controlType=sensitive)", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("SomeText")
			v1.Prompt = &variables.VariablePromptOptions{
				Description:     "Enter secret text",
				DisplaySettings: variables.NewDisplaySettings(variables.ControlTypeSensitive, nil),
			}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return deploy.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			_ = qa.ExpectQuestion(t, &survey.Password{
				Message: "SomeText (Enter secret text)",
			}).AnswerWith("secretsquirrel")

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"SomeText": "secretsquirrel"}, output)
		}},

		{"prompts for a sensitive value (variable.type=sensitive)", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("SomeText")
			v1.Type = "Sensitive"
			v1.Prompt = &variables.VariablePromptOptions{
				Description: "Enter secret text",
			}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return deploy.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			_ = qa.ExpectQuestion(t, &survey.Password{
				Message: "SomeText (Enter secret text)",
			}).AnswerWith("secretsquirrel")

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"SomeText": "secretsquirrel"}, output)
		}},

		// REFER: https://github.com/OctopusDeploy/Issues/issues/7699
		{"does not prompt for complex variable types", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			// It is possible in octopus to set the 'Prompt' flag on complex variable types such as accounts and certificates.
			// This results in the web portal prompting for user input, but this doesn't work as expected.
			// As at 5 Sep 2022, the FNM team hadn't decided how they were going to fix it. Interim measure is to
			// have the CLI ignore the prompt flag on these kinds of variables and revisit later once FNM have resolved the bug
			v1 := variables.NewVariable("Certificate")
			v1.Type = "Certificate"
			v1.Prompt = &variables.VariablePromptOptions{
				Description:     "Codesigning certificate?",
				DisplaySettings: variables.NewDisplaySettings(variables.ControlTypeCertificate, nil),
			}

			v2 := variables.NewVariable("AWS Account")
			v2.Type = "AzureAccount"
			v2.Prompt = &variables.VariablePromptOptions{
				Description:     "AZ Account?",
				DisplaySettings: &variables.DisplaySettings{},
			}
			vars := makeVariables(v1, v2)

			output, err := deploy.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))

			assert.Nil(t, err)
			assert.Equal(t, make(map[string]string, 0), output)

		}},

		{"prompts for a combo box value", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("Division")
			v1.Type = "String"
			v1.Prompt = &variables.VariablePromptOptions{
				Description: "Which part of the company do you work in?",
				DisplaySettings: variables.NewDisplaySettings(variables.ControlTypeSelect, []*variables.SelectOption{
					{Value: "rnd", DisplayName: "R&D"},
					{Value: "finance", DisplayName: "Finance"},
					{Value: "hr", DisplayName: "Human Resources"},
					{Value: "other", DisplayName: "Other"},
				}),
			}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return deploy.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Division (Which part of the company do you work in?)",
				Options: []string{"R&D", "Finance", "Human Resources", "Other"},
				Default: "",
			}).AnswerWith("Human Resources")

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"Division": "hr"}, output)
		}},

		{"combo box with default value", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("Division")
			v1.Type = "String"
			v1.Value = "rnd" // looks this up!
			v1.Prompt = &variables.VariablePromptOptions{
				Description: "Which part of the company do you work in?",
				DisplaySettings: variables.NewDisplaySettings(variables.ControlTypeSelect, []*variables.SelectOption{
					{Value: "rnd", DisplayName: "R&D"},
					{Value: "finance", DisplayName: "Finance"},
					{Value: "hr", DisplayName: "Human Resources"},
					{Value: "other", DisplayName: "Other"},
				}),
			}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return deploy.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Division (Which part of the company do you work in?)",
				Options: []string{"R&D", "Finance", "Human Resources", "Other"},
				Default: "R&D",
			}).AnswerWith("Other")

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"Division": "other"}, output)
		}},

		{"checkbox", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("IsApproved")
			v1.Prompt = &variables.VariablePromptOptions{
				Description:     "Is this approved?",
				DisplaySettings: variables.NewDisplaySettings(variables.ControlTypeCheckbox, nil),
				IsRequired:      false,
			}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return deploy.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "IsApproved (Is this approved?)",
				Default: "False", // checkbox defaults itself to false if not specified
				Options: []string{"True", "False"},
			}).AnswerWith("True")

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"IsApproved": "True"}, output)
		}},

		{"checkbox with default value true", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			v1 := variables.NewVariable("IsApproved")
			v1.Value = "True"
			v1.Prompt = &variables.VariablePromptOptions{
				Description:     "Is this approved?",
				DisplaySettings: variables.NewDisplaySettings(variables.ControlTypeCheckbox, nil),
				IsRequired:      false,
			}

			vars := makeVariables(v1)
			receiver := testutil.GoBegin2(func() (map[string]string, error) {
				return deploy.AskVariables(qa.AsAsker(), vars, make(map[string]string, 0))
			})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "IsApproved (Is this approved?)",
				Default: "True",
				Options: []string{"True", "False"},
			}).AnswerWith("True")

			output, err := testutil.ReceivePair(receiver)
			assert.Nil(t, err)
			assert.Equal(t, map[string]string{"IsApproved": "True"}, output)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			qa := testutil.NewAskMocker()
			test.run(t, qa, new(bytes.Buffer))
		})
	}
}

func TestParseVariableStringArray(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		expect    map[string]string
		expectErr error
	}{
		{name: "foo:bar", input: []string{"foo:bar"}, expect: map[string]string{"foo": "bar"}},
		{name: "foo:bar,baz:qux", input: []string{"foo:bar", "baz:qux"}, expect: map[string]string{"foo": "bar", "baz": "qux"}},
		{name: "foo=bar,baz=qux", input: []string{"foo=bar", "baz=qux"}, expect: map[string]string{"foo": "bar", "baz": "qux"}},

		{name: "foo:bar:more=stuff", input: []string{"foo:bar:more=stuff"}, expect: map[string]string{"foo": "bar:more=stuff"}},

		{name: "trims whitespace", input: []string{" foo : \tbar "}, expect: map[string]string{"foo": "bar"}},

		// error cases
		{name: "blank", input: []string{""}, expectErr: errors.New("could not parse variable definition ''")},
		{name: "no delimeter", input: []string{"zzz"}, expectErr: errors.New("could not parse variable definition 'zzz'")},
		{name: "missing key", input: []string{":bar"}, expectErr: errors.New("could not parse variable definition ':bar'")},
		{name: "missing val", input: []string{"foo:"}, expectErr: errors.New("could not parse variable definition 'foo:'")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := deploy.ParseVariableStringArray(test.input)
			assert.Equal(t, test.expectErr, err)
			assert.Equal(t, test.expect, result)
		})
	}
}

func TestToVariableStringArray(t *testing.T) {
	tests := []struct {
		name   string
		input  map[string]string
		expect []string
	}{
		{name: "foo:bar", input: map[string]string{"foo": "bar"}, expect: []string{"foo:bar"}},

		{name: "foo:bar:more=stuff", input: map[string]string{"foo": "bar:more=stuff"}, expect: []string{"foo:bar:more=stuff"}},

		{name: "strips empty keys", input: map[string]string{"": "bar"}, expect: []string{}},
		{name: "strips empty values", input: map[string]string{"foo": ""}, expect: []string{}},

		// these two tests in combination check that output order is deterministic
		{name: "foo:bar,baz:qux", input: map[string]string{"foo": "bar", "baz": "qux"}, expect: []string{"baz:qux", "foo:bar"}},
		{name: "baz:qux,foo:bar", input: map[string]string{"baz": "qux", "foo": "bar"}, expect: []string{"baz:qux", "foo:bar"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := deploy.ToVariableStringArray(test.input)
			assert.Equal(t, test.expect, result)
		})
	}
}
