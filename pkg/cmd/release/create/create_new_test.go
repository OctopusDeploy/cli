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
	"testing"
)

//var serverUrl, _ = url.Parse("http://server")
//
//const placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

func TestCreate_AskQuestions_NewStyle(t *testing.T) {
	root := testutil.NewRootResource()

	const fireProjectID = "Projects-22"

	depProcess := deployments.NewDeploymentProcess(fireProjectID)
	depProcess.ID = "deploymentprocess-" + fireProjectID
	depProcess.Links = map[string]string{
		"Template": "/api/Spaces-1/projects/" + fireProjectID + "/deploymentprocesses/template",
	}

	defaultChannel := channels.NewChannel("Fire Project Default Channel", fireProjectID)
	altChannel := channels.NewChannel("Fire Project Alt Channel", fireProjectID)

	fireProject := projects.NewProject("Fire Project", "Lifecycles-1", "ProjectGroups-1")
	fireProject.ID = fireProjectID
	fireProject.PersistenceSettings = projects.NewDatabasePersistenceSettings()
	fireProject.DeploymentProcessID = depProcess.ID
	fireProject.Links = map[string]string{
		"Channels":          "/api/Spaces-1/projects/" + fireProjectID + "/channels{/id}{?skip,take,partialName}",
		"DeploymentProcess": "/api/Spaces-1/projects/" + fireProjectID + "/deploymentprocesses",
	}

	t.Run("standard process asking for everything (no package versions)", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		qa := testutil.NewAskMocker2()

		options := &executor.TaskOptionsCreateRelease{}

		p := testutil.GoBeginFunc1(func() error {
			// NewClient makes network calls so we have to run it in the goroutine
			octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
			return create.AskQuestions(octopus, qa.AsAsker(), options)
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{fireProject})

		qa.ExpectQuestion(t, &survey.Select{
			Message: "Select the project in which the release will be created",
			Options: []string{"Fire Project"},
		}).AnswerWith("Fire Project")

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/channels").RespondWith(resources.Resources[*channels.Channel]{
			Items: []*channels.Channel{defaultChannel, altChannel},
		})

		qa.ExpectQuestion(t, &survey.Select{
			Message: "Select the channel in which the release will be created",
			Options: []string{defaultChannel.Name, altChannel.Name},
		}).AnswerWith("Fire Project Alt Channel")

		api.ExpectRequest(t, "GET", "/api/Spaces-1/deploymentprocesses/deploymentprocess-"+fireProjectID).RespondWith(depProcess)

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/deploymentprocesses/template").
			RespondWith(&deployments.DeploymentProcessTemplate{NextVersionIncrement: "27.9.3"})

		qa.ExpectQuestion(t, &survey.Input{
			Message: "Version",
			Default: "27.9.3",
		}).AnswerWith("27.9.999")

		err := p.Wait()
		assert.Nil(t, err)

		// check that the question-asking process has filled out the things we told it to
		assert.Equal(t, "Fire Project", options.ProjectName)
		assert.Equal(t, "Fire Project Alt Channel", options.ChannelName)
		assert.Equal(t, "27.9.999", options.Version)

		assert.Equal(t, 0, api.GetPendingMessageCount()+qa.GetPendingMessageCount())
	})

	t.Run("asking for nothing in interactive mode (testing case insensitivity)", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		qa := testutil.NewAskMocker2()

		options := &executor.TaskOptionsCreateRelease{
			ProjectName: "fire project",
			ChannelName: "fire project default channel",
			Version:     "9.8.4-prerelease",
		}

		p := testutil.GoBeginFunc1(func() error {
			// NewClient makes network calls so we have to run it in the goroutine
			octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
			return create.AskQuestions(octopus, qa.AsAsker(), options)
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=fire+project").
			RespondWith(resources.Resources[*projects.Project]{
				Items: []*projects.Project{fireProject},
			})

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/channels").
			RespondWith(resources.Resources[*channels.Channel]{
				Items: []*channels.Channel{defaultChannel},
			})

		err := p.Wait()
		assert.Nil(t, err)

		// check that the question-asking process has filled out the things we told it to
		assert.Equal(t, "Fire Project", options.ProjectName)
		assert.Equal(t, "Fire Project Default Channel", options.ChannelName)
		assert.Equal(t, "9.8.4-prerelease", options.Version)

		assert.Equal(t, 0, api.GetPendingMessageCount()+qa.GetPendingMessageCount())
	})

}
