package deploy_test

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	surveyCore "github.com/AlecAivazis/survey/v2/core"
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
			}).AnswerWith([]surveyCore.OptionAnswer{
				{Value: devEnvironment.Name, Index: 0},
			})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/deploymentprocesses/"+depProcessSnapshot.ID).RespondWith(depProcessSnapshot)

			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Select steps to skip (if any)",
				Options: []string{"Install", "Cleanup"},
			}).AnswerWith(make([]surveyCore.OptionAnswer, 0))

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
		{name: "foo:bar,baz:qux", input: map[string]string{"foo": "bar", "baz": "qux"}, expect: []string{"foo:bar", "baz:qux"}},

		{name: "foo:bar:more=stuff", input: map[string]string{"foo": "bar:more=stuff"}, expect: []string{"foo:bar:more=stuff"}},

		{name: "strips empty keys", input: map[string]string{"": "bar"}, expect: []string{}},
		{name: "strips empty values", input: map[string]string{"foo": ""}, expect: []string{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := deploy.ToVariableStringArray(test.input)
			assert.Equal(t, test.expect, result)
		})
	}
}
