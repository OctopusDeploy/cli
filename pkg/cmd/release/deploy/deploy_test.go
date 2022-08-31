package deploy_test

import (
	"bytes"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
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

	release20 := fixtures.NewRelease(spaceID, "Releases-200", "2.0", fireProjectID, altChannel.ID)
	// doesn't have any dep process snapshots, wat
	release19 := fixtures.NewRelease(spaceID, "Releases-193", "1.9", fireProjectID, altChannel.ID)
	release19.ProjectDeploymentProcessSnapshotID = depProcessSnapshot.ID
	//release09 := fixtures.NewRelease(spaceID, "Releases-87", "0.9", fireProjectID, defaultChannel.ID)

	devEnvironment := fixtures.NewEnvironment(spaceID, "Environments-12", "dev")
	prodEnvironment := fixtures.NewEnvironment(spaceID, "Environments-13", "production")

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer)
	}{
		{"standard process asking for everything (explicitly non-tenanted)", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
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

			// TODO this isn't right; we should only be listing the environments that this project can deploy to based on... TODO???
			api.ExpectRequest(t, "GET", "/api/Spaces-1/environments").RespondWith(resources.Resources[*environments.Environment]{
				Items: []*environments.Environment{devEnvironment, prodEnvironment},
			})

			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Select environments to deploy to",
				Options: []string{devEnvironment.Name, prodEnvironment.Name},
			}).AnswerWith(devEnvironment.Name)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/deploymentprocesses/"+depProcessSnapshot.ID).RespondWith(depProcessSnapshot)

			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Select steps to skip (if any)",
				Options: []string{"Install", "Cleanup"},
			}).AnswerWith(make([]string, 0))

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Guided Failure Mode?",
				Options: []string{"Use default setting from the target environment", "Use guided failure mode", "Do not use guided failure mode"},
			}).AnswerWith("Do not use guided failure mode")

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Force package re-download?",
				Options: []string{"No", "Yes"},
			}).AnswerWith("No")

			err := <-errReceiver
			assert.Nil(t, err)

			// check that the question-asking process has filled out the things we told it to
			assert.Equal(t, "Fire Project", options.ProjectName)
			assert.Equal(t, "1.9", options.ReleaseVersion)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api, qa := testutil.NewMockServerAndAsker()
			test.run(t, api, qa, new(bytes.Buffer))
		})
	}
}
