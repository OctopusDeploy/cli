package deploy_test

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	surveyCore "github.com/AlecAivazis/survey/v2/core"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/release/deploy"
	"github.com/OctopusDeploy/cli/pkg/executor"
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
	"github.com/stretchr/testify/assert"
	"net/url"
	"testing"
)

var serverUrl, _ = url.Parse("http://server")

const placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
const packageOverrideQuestion = "Package override string (y to accept, u to undo, ? for help):"

var spinner = &testutil.FakeSpinner{}

var rootResource = testutil.NewRootResource()

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

	variableSnapshot := fixtures.NewVariableSetForProject(spaceID, fireProjectID)
	variableSnapshot.ID = fmt.Sprintf("%s-s-0-2ZFWS", variableSnapshot.ID)

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
	release19.ProjectVariableSetSnapshotID = variableSnapshot.ID

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
				return deploy.AskQuestions(octopus, stdout, qa.AsAsker(), spinner, space1, options)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{fireProject})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the project to deploy from",
				Options: []string{"Fire Project"},
			}).AnswerWith("Fire Project")

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/channels").RespondWith(resources.Resources[*channels.Channel]{
				Items: []*channels.Channel{defaultChannel, altChannel},
			})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the channel to deploy from",
				Options: []string{defaultChannel.Name, altChannel.Name},
			}).AnswerWith("Fire Project Alt Channel")

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/channels/"+altChannel.ID+"/releases").RespondWith(resources.Resources[*releases.Release]{
				Items: []*releases.Release{release20, release19},
			})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the release to deploy",
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
				Message: "Select environments to deploy to",
				Options: []string{scratchEnvironment.Name, devEnvironment.Name},
				Default: []string{devEnvironment.Name},
			}).AnswerWith([]surveyCore.OptionAnswer{
				{Value: devEnvironment.Name, Index: 0},
			})

			// now it's going to go looking for prompted variables; we don't have any prompted variables here so it skips
			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshot.ID).RespondWith(&variableSnapshot)

			q := qa.ExpectQuestion(t, &survey.Select{
				Message: "Do you want to change advanced options?",
				Options: []string{"Proceed to deploy", "Change advanced options"},
			})

			assert.Equal(t, heredoc.Doc(`
				Advanced Options:
				  Deploy Time: Now
				  Skipped Steps: None
				  Guided Failure Mode: Use default setting from the target environment
				  Package Download: Use cached packages (if available)
				  Deployment Targets: All included
				`), stdout.String())

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
				return deploy.AskQuestions(octopus, stdout, qa.AsAsker(), spinner, space1, options)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=fire+project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/releases/"+release19.Version).RespondWith(release19)

			// doesn't lookup the progression or env names because it already has them

			// now it's going to go looking for prompted variables; we don't have any prompted variables here so it skips
			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshot.ID).RespondWith(&variableSnapshot)

			q := qa.ExpectQuestion(t, &survey.Select{
				Message: "Do you want to change advanced options?",
				Options: []string{"Proceed to deploy", "Change advanced options"},
			})

			assert.Equal(t, heredoc.Doc(`
				Project Fire Project
				Release 1.9
				Environments dev
				Advanced Options:
				  Deploy Time: Now
				  Skipped Steps: None
				  Guided Failure Mode: Use default setting from the target environment
				  Package Download: Use cached packages (if available)
				  Deployment Targets: All included
				`), stdout.String())

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
				return deploy.AskQuestions(octopus, stdout, qa.AsAsker(), spinner, space1, options)
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

			q = qa.ExpectQuestion(t, &survey.Select{
				Message: "Do you want to change advanced options?",
				Options: []string{"Proceed to deploy", "Change advanced options"},
			})

			assert.Equal(t, heredoc.Doc(`
				Project Fire Project
				Release 2.0
				Environments dev
				Advanced Options:
				  Deploy Time: Now
				  Skipped Steps: None
				  Guided Failure Mode: Use default setting from the target environment
				  Package Download: Use cached packages (if available)
				  Deployment Targets: All included
				`), stdout.String())

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
				return deploy.AskQuestions(octopus, stdout, qa.AsAsker(), spinner, space1, options)
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
			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshot.ID).RespondWith(&variableSnapshot)

			q = qa.ExpectQuestion(t, &survey.Select{
				Message: "Do you want to change advanced options?",
				Options: []string{"Proceed to deploy", "Change advanced options"},
			})

			assert.Equal(t, heredoc.Doc(`
				Project Fire Project
				Release 1.9
				Advanced Options:
				  Deploy Time: Now
				  Skipped Steps: None
				  Guided Failure Mode: Use default setting from the target environment
				  Package Download: Use cached packages (if available)
				  Deployment Targets: All included
				`), stdout.String())

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
				return deploy.AskQuestions(octopus, stdout, qa.AsAsker(), spinner, space1, options)
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
			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshot.ID).RespondWith(&variableSnapshot)

			q = qa.ExpectQuestion(t, &survey.Select{
				Message: "Do you want to change advanced options?",
				Options: []string{"Proceed to deploy", "Change advanced options"},
			})

			assert.Equal(t, heredoc.Doc(`
				Project Fire Project
				Release 1.9
				Advanced Options:
				  Deploy Time: Now
				  Skipped Steps: None
				  Guided Failure Mode: Use default setting from the target environment
				  Package Download: Use cached packages (if available)
				  Deployment Targets: All included
				`), stdout.String())

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
				return deploy.AskQuestions(octopus, stdout, qa.AsAsker(), spinner, space1, options)
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
				Message: "Select environments to deploy to",
				Options: []string{scratchEnvironment.Name, devEnvironment.Name},
				Default: []string{devEnvironment.Name},
			}).AnswerWith([]surveyCore.OptionAnswer{
				{Value: devEnvironment.Name, Index: 0},
			})

			// now it's going to go looking for prompted variables; we don't have any prompted variables here so it skips
			api.ExpectRequest(t, "GET", "/api/Spaces-1/variables/"+variableSnapshot.ID).RespondWith(&variableSnapshot)

			q := qa.ExpectQuestion(t, &survey.Select{
				Message: "Do you want to change advanced options?",
				Options: []string{"Proceed to deploy", "Change advanced options"},
			})

			assert.Equal(t, heredoc.Doc(`
				Project Fire Project
				Release 1.9
				Advanced Options:
				  Deploy Time: Now
				  Skipped Steps: None
				  Guided Failure Mode: Use default setting from the target environment
				  Package Download: Use cached packages (if available)
				  Deployment Targets: All included
				`), stdout.String())

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
