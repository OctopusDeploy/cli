package create_test

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/release/create"
	"github.com/OctopusDeploy/cli/pkg/executor"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/stretchr/testify/assert"
	"net/url"
	"testing"
)

var serverUrl, _ = url.Parse("http://server")
var fakeRepoUrl, _ = url.Parse("https://gitserver/repo")

const placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

func TestReleaseCreate_AskQuestions(t *testing.T) {
	root := testutil.NewRootResource()

	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"
	const cacProjectID = "Projects-92"

	depProcess := fixtures.NewDeploymentProcessForProject(spaceID, fireProjectID)

	defaultChannel := channels.NewChannel("Fire Project Default Channel", fireProjectID)
	altChannel := channels.NewChannel("Fire Project Alt Channel", fireProjectID)

	fireProject := fixtures.NewProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", depProcess.ID)

	cacProject := fixtures.NewProject(spaceID, cacProjectID, "CaC Project", "Lifecycles-1", "ProjectGroups-1", depProcess.ID)
	cacProject.PersistenceSettings = projects.NewGitPersistenceSettings(
		".octopus",
		projects.NewAnonymousGitCredential(),
		"main",
		fakeRepoUrl,
	)

	t.Run("standard process asking for everything (no package versions) or CaC", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		qa := testutil.NewAskMocker()

		options := &executor.TaskOptionsCreateRelease{}

		errReceiver := testutil.GoBegin(func() error {
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

		err := <-errReceiver
		assert.Nil(t, err)

		// check that the question-asking process has filled out the things we told it to
		assert.Equal(t, "Fire Project", options.ProjectName)
		assert.Equal(t, "Fire Project Alt Channel", options.ChannelName)
		assert.Equal(t, "27.9.999", options.Version)
	})

	t.Run("asking for nothing in interactive mode (testing case insensitivity)", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		qa := testutil.NewAskMocker()

		options := &executor.TaskOptionsCreateRelease{
			ProjectName: "fire project",
			ChannelName: "fire project default channel",
			Version:     "9.8.4-prerelease",
		}

		errReceiver := testutil.GoBegin(func() error {
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

		err := <-errReceiver
		assert.Nil(t, err)

		// check that the question-asking process has filled out the things we told it to
		assert.Equal(t, "Fire Project", options.ProjectName)
		assert.Equal(t, "Fire Project Default Channel", options.ChannelName)
		assert.Equal(t, "9.8.4-prerelease", options.Version)
	})

	t.Run("standard process asking for everything (no package versions) in CaC project", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		qa := testutil.NewAskMocker()

		options := &executor.TaskOptionsCreateRelease{}

		errReceiver := testutil.GoBegin(func() error {
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

		err := <-errReceiver
		assert.Nil(t, err)

		// check that the question-asking process has filled out the things we told it to
		assert.Equal(t, "Fire Project", options.ProjectName)
		assert.Equal(t, "Fire Project Alt Channel", options.ChannelName)
		assert.Equal(t, "27.9.999", options.Version)
	})
}

func TestReleaseCreate_AutomationModeCaCProject(t *testing.T) {
	fakeRepoUrl, _ := url.Parse("https://gitserver/repo")

	root := testutil.NewRootResource()

	const cacProjectID = "Projects-92"

	space1 := fixtures.NewSpace("Spaces-1", "Default Space")

	depProcess := fixtures.NewDeploymentProcessForProject(space1.ID, cacProjectID)

	cacProject := fixtures.NewProject(space1.ID, cacProjectID, "CaC Project", "Lifecycles-1", "ProjectGroups-1", depProcess.ID)
	cacProject.PersistenceSettings = projects.NewGitPersistenceSettings(
		".octopus",
		projects.NewAnonymousGitCredential(),
		"main",
		fakeRepoUrl,
	)

	api := testutil.NewMockHttpServer()

	t.Run("release creation sanity check", func(t *testing.T) {
		taskOptions := &executor.TaskOptionsCreateRelease{
			ProjectName: cacProject.Name,
		}

		errReceiver := testutil.GoBegin(func() error {
			// NewClient makes network calls so we have to run it in the goroutine
			octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
			return executor.ProcessTasks(octopus, space1, []*executor.Task{
				executor.NewTask(executor.TaskTypeCreateRelease, taskOptions),
			})
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		req := api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/create/v1")

		reader, err := req.Request.GetBody()
		assert.Nil(t, err)

		requestBody, err := testutil.ReadJson[releases.CreateReleaseV1](reader)
		assert.Nil(t, err)

		assert.Equal(t, releases.CreateReleaseV1{
			SpaceIDOrName:   "Spaces-1",
			ProjectIDOrName: cacProject.Name,
		}, requestBody)

		req.RespondWith(&releases.CreateReleaseResponseV1{
			ReleaseID:      "Releases-999", // new release
			ReleaseVersion: "Blah",
		})

		err = <-errReceiver
		assert.Nil(t, err)

		assert.Equal(t, &releases.CreateReleaseResponseV1{
			ReleaseID:                         "Releases-999",
			ReleaseVersion:                    "Blah",
			AutomaticallyDeployedEnvironments: "",
		}, taskOptions.Response)
	})

	t.Run("release creation specifies gitcommit and gitref", func(t *testing.T) {
		taskOptions := &executor.TaskOptionsCreateRelease{
			ProjectName:  cacProject.Name,
			GitReference: "/refs/heads/main",
			GitCommit:    "cfdd4bdf6f66569f141dc41189f0f975d4aa1110",
		}

		errReceiver := testutil.GoBegin(func() error {
			octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
			return executor.ProcessTasks(octopus, space1, []*executor.Task{
				executor.NewTask(executor.TaskTypeCreateRelease, taskOptions),
			})
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		req := api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/create/v1")

		reader, err := req.Request.GetBody()
		assert.Nil(t, err)

		receivedReq, err := testutil.ReadJson[releases.CreateReleaseV1](reader)
		assert.Nil(t, err)

		assert.Equal(t, releases.CreateReleaseV1{
			SpaceIDOrName:   "Spaces-1",
			ProjectIDOrName: cacProject.Name,
			GitRef:          "/refs/heads/main",
			GitCommit:       "cfdd4bdf6f66569f141dc41189f0f975d4aa1110",
		}, receivedReq)

		req.RespondWith(&releases.CreateReleaseResponseV1{
			ReleaseID:      "Releases-999", // new release
			ReleaseVersion: "Blah",
		})

		err = <-errReceiver
		assert.Nil(t, err)

		assert.Equal(t, &releases.CreateReleaseResponseV1{
			ReleaseID:                         "Releases-999",
			ReleaseVersion:                    "Blah",
			AutomaticallyDeployedEnvironments: "",
		}, taskOptions.Response)
	})
}
