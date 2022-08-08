package create_test

import (
	"encoding/json"
	"github.com/OctopusDeploy/cli/pkg/executor"
	"github.com/OctopusDeploy/cli/test/testutil"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/stretchr/testify/assert"
	"io"
	"net/url"
	"testing"
)

// See "Config As Code projects aren't covered by integration tests" in the release creation integration tests for explanation

func readJson[T any](body io.ReadCloser) (T, error) {
	buf := make([]byte, 4096)

	bytesRead, err := body.Read(buf)
	if err != nil {
		return *new(T), err
	}

	var unmarshalled T
	err = json.Unmarshal(buf[:bytesRead], &unmarshalled)
	if err != nil {
		return *new(T), err
	}

	return unmarshalled, nil
}

func TestReleaseCreate_AutomationModeCaCProject(t *testing.T) {
	fakeRepoUrl, _ := url.Parse("https://gitserver/repo")

	root := testutil.NewRootResource()

	const cacProjectID = "Projects-92"

	space1 := spaces.NewSpace("Default Space")
	space1.ID = "Spaces-1"

	depProcess := deployments.NewDeploymentProcess(cacProjectID)
	depProcess.ID = "deploymentprocess-" + cacProjectID
	depProcess.Links = map[string]string{
		"Template": "/api/Spaces-1/projects/" + cacProjectID + "/deploymentprocesses/template",
	}
	//
	//defaultChannel := channels.NewChannel("CaC Project Default Channel", cacProjectID)
	//altChannel := channels.NewChannel("CaC Project Alt Channel", cacProjectID)

	cacProject := projects.NewProject("CaC Project", "Lifecycles-1", "ProjectGroups-1")
	cacProject.ID = cacProjectID
	cacProject.PersistenceSettings = projects.NewGitPersistenceSettings(
		".octopus",
		projects.NewAnonymousGitCredential(),
		"main",
		fakeRepoUrl,
	)
	cacProject.DeploymentProcessID = depProcess.ID
	cacProject.Links = map[string]string{
		"Channels":          "/api/Spaces-1/projects/" + cacProjectID + "/channels{/id}{?skip,take,partialName}",
		"DeploymentProcess": "/api/Spaces-1/projects/" + cacProjectID + "/deploymentprocesses",
	}

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

		receivedReq, err := readJson[releases.CreateReleaseV1](reader)
		assert.Nil(t, err)

		assert.Equal(t, releases.CreateReleaseV1{
			SpaceIDOrName:   "Spaces-1",
			ProjectIDOrName: cacProject.Name,
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

	t.Run("release creation specifies gitcommit and gitref", func(t *testing.T) {
		taskOptions := &executor.TaskOptionsCreateRelease{
			ProjectName:  cacProject.Name,
			GitReference: "/refs/heads/main",
			GitCommit:    "cfdd4bdf6f66569f141dc41189f0f975d4aa1110",
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

		receivedReq, err := readJson[releases.CreateReleaseV1](reader)
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
