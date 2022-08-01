package create_test

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/release/create"
	"github.com/OctopusDeploy/cli/pkg/executor"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/url"
	"testing"
)

var serverUrl, _ = url.Parse("http://server")

const placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

func TestCreate_AskQuestions(t *testing.T) {
	const fireProjectID = "Projects-22"

	depProcess := deployments.NewDeploymentProcess(fireProjectID)
	depProcess.ID = "deploymentprocess-" + fireProjectID
	depProcess.Links = map[string]string{
		"Template": "/api/Spaces-1/projects/" + fireProjectID + "/deploymentprocesses/template",
	}

	defaultChannel := channels.NewChannel("Fire Project Default Channel", fireProjectID)

	fireProject := projects.NewProject("Fire Project", "Lifecycles-1", "ProjectGroups-1")
	fireProject.ID = fireProjectID
	fireProject.PersistenceSettings = projects.NewDatabasePersistenceSettings()
	fireProject.DeploymentProcessID = depProcess.ID
	fireProject.Links = map[string]string{
		"Channels":          "/api/Spaces-1/projects/" + fireProjectID + "/channels{/id}{?skip,take,partialName}",
		"DeploymentProcess": "/api/Spaces-1/projects/" + fireProjectID + "/deploymentprocesses",
	}

	t.Run("standard process asking for everything (no package versions)", func(t *testing.T) {
		api := testutil.NewFakeApiResponder()
		testutil.EnqueueRootResponder(api)
		octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")

		api.EnqueueResponder("GET", "/api/Spaces-1/projects/all", func(r *http.Request) (any, error) {
			return []*projects.Project{fireProject}, nil
		})

		api.EnqueueResponder("GET", "/api/Spaces-1/projects/"+fireProjectID+"/channels", func(r *http.Request) (any, error) {
			return resources.Resources[*channels.Channel]{
				Items: []*channels.Channel{defaultChannel},
			}, nil
		})

		api.EnqueueResponder("GET", "/api/Spaces-1/deploymentprocesses/deploymentprocess-"+fireProjectID, func(r *http.Request) (any, error) {
			return depProcess, nil
		})

		api.EnqueueResponder("GET", "/api/Spaces-1/projects/"+fireProjectID+"/deploymentprocesses/template", func(r *http.Request) (any, error) {
			return &deployments.DeploymentProcessTemplate{NextVersionIncrement: "27.9.3"}, nil // TODO
		})

		// mock survey
		asker, unasked := testutil.NewAskMocker(t, []testutil.QA{
			{
				Prompt: &survey.Select{
					Message: "Select the project in which the release will be created",
					Options: []string{"Fire Project"},
				},
				Answer: "Fire Project",
			},
			{
				Prompt: &survey.Select{
					Message: "Select the channel in which the release will be created",
					Options: []string{"Fire Project Default Channel"},
				},
				Answer: "Fire Project Default Channel",
			},
			{
				Prompt: &survey.Input{
					Message: "Version",
					Default: "27.9.3",
				},
				Answer: "27.9.3",
			},
		})
		defer unasked()

		options := &executor.TaskOptionsCreateRelease{}

		err := create.AskQuestions(octopus, asker, options)
		assert.Nil(t, err)

		// check that the question-asking process has filled out the things we told it to
		assert.Equal(t, "Fire Project", options.ProjectName)
		assert.Equal(t, "Fire Project Default Channel", options.ChannelName)
		assert.Equal(t, "27.9.3", options.Version)

		assert.Equal(t, 0, api.RemainingQueueLength())
	})

	t.Run("asking for nothing in interactive mode (testing case insensitivity)", func(t *testing.T) {
		api := testutil.NewFakeApiResponder()
		testutil.EnqueueRootResponder(api)
		octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")

		api.EnqueueResponder("GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=fire+project", func(r *http.Request) (any, error) {
			return resources.Resources[*projects.Project]{
				Items: []*projects.Project{fireProject},
			}, nil
		})

		api.EnqueueResponder("GET", "/api/Spaces-1/projects/"+fireProjectID+"/channels", func(r *http.Request) (any, error) {
			return resources.Resources[*channels.Channel]{
				Items: []*channels.Channel{defaultChannel},
			}, nil
		})

		// mock survey
		asker, _ := testutil.NewAskMocker(t, []testutil.QA{})

		options := &executor.TaskOptionsCreateRelease{
			ProjectName: "fire project",
			ChannelName: "fire project default channel",
			Version:     "9.8.4-prerelease",
		}

		err := create.AskQuestions(octopus, asker, options)
		assert.Nil(t, err)

		// check that the question-asking process has filled out the things we told it to
		assert.Equal(t, "Fire Project", options.ProjectName)
		assert.Equal(t, "Fire Project Default Channel", options.ChannelName)
		assert.Equal(t, "9.8.4-prerelease", options.Version)

		assert.Equal(t, 0, api.RemainingQueueLength())
	})

}
