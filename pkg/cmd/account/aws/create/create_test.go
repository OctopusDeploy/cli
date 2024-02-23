package create_test

import (
	"bytes"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"net/url"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/account/aws/create"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/stretchr/testify/assert"
)

var serverUrl, _ = url.Parse("http://server")

const placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

var rootResource = testutil.NewRootResource()

func TestAWSAccountCreatePromptMissing(t *testing.T) {
	const spaceID = "Space-1"
	const envID = "Env-1"
	space := fixtures.NewSpace(spaceID, "testspace")
	env := fixtures.NewEnvironment(spaceID, envID, "testenv")
	api, qa := testutil.NewMockServerAndAsker()
	out := &bytes.Buffer{}

	opts := &create.CreateOptions{
		CreateFlags:  create.NewCreateFlags(),
		Dependencies: &cmd.Dependencies{Space: space},
		GetAllEnvironmentsCallback: func() ([]*environments.Environment, error) {
			return []*environments.Environment{env}, nil
		},
	}

	errReceiver := testutil.GoBegin(func() error {
		defer testutil.Close(api, qa)
		octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
		opts.Ask = qa.AsAsker()
		opts.Client = octopus
		opts.Out = out
		return create.PromptMissing(opts)
	})

	api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/spaces").RespondWith(rootResource)

	_ = qa.ExpectQuestion(t, &survey.Input{
		Message: "Name",
		Help:    "A short, memorable, unique name for this account.",
	}).AnswerWith("TestAccount")

	_ = qa.ExpectQuestion(t, &surveyext.OctoEditor{
		Editor: &survey.Editor{
			Message:  "Description",
			Help:     "A summary explaining the use of the account to other users.",
			FileName: "*.md",
		},
		Optional: true,
	}).AnswerWith("test 123")

	_ = qa.ExpectQuestion(t, &survey.Input{
		Message: "Access Key",
		Help:    "The AWS access key to use when authenticating against Amazon Web Services.",
	}).AnswerWith("testaccesskey123")

	_ = qa.ExpectQuestion(t, &survey.Password{
		Message: "Secret Key",
		Help:    "The AWS secret key to use when authenticating against Amazon Web Services.",
	}).AnswerWith("testpassword124")

	_ = qa.ExpectQuestion(t, &survey.MultiSelect{
		Message: "Choose the environments that are allowed to use this account.\nIf nothing is selected, the account can be used for deployments to any environment.",
		Options: []string{"testenv"},
	}).AnswerWith([]string{"testenv"})

	err := <-errReceiver
	assert.Nil(t, err)

	assert.Equal(t, []string{envID}, opts.Environments.Value)
	assert.Equal(t, "testpassword124", opts.SecretKey.Value)
	assert.Equal(t, "testaccesskey123", opts.AccessKey.Value)
	assert.Equal(t, "test 123", opts.Description.Value)
	assert.Equal(t, "TestAccount", opts.Name.Value)
}

func TestAWSAccountCreateNoPrompt(t *testing.T) {
	const spaceID = "Space-1"
	const envID = "Env-1"
	space := fixtures.NewSpace(spaceID, "testspace")
	_ = fixtures.NewEnvironment(spaceID, envID, "testenv")
	api, qa := testutil.NewMockServerAndAsker()
	out := &bytes.Buffer{}

	opts := &create.CreateOptions{
		CreateFlags:  create.NewCreateFlags(),
		Dependencies: &cmd.Dependencies{Space: space},
	}
	opts.Name.Value = "testaccount"
	opts.AccessKey.Value = "testaccesskey123"
	opts.SecretKey.Value = "testsecretkey123"

	errReceiver := testutil.GoBegin(func() error {
		defer testutil.Close(api, qa)
		octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
		opts.Ask = qa.AsAsker()
		opts.Client = octopus
		opts.Out = out
		opts.NoPrompt = true
		return create.CreateRun(opts)
	})

	testAccount, err := accounts.NewAmazonWebServicesAccount(
		opts.Name.Value,
		opts.AccessKey.Value,
		core.NewSensitiveValue(opts.SecretKey.Value),
	)
	assert.Nil(t, err)
	testAccount.ID = "Account-1"
	testAccount.Slug = "testaccount"
	testAccount.SpaceID = spaceID

	api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/spaces").RespondWith(rootResource)
	api.ExpectRequest(t, "POST", "/api/Spaces-1/accounts").RespondWithStatus(201, "", testAccount)

	err = <-errReceiver
	assert.Nil(t, err)
	res := out.String()
	assert.Equal(t, heredoc.Docf(`
		Successfully created AWS account %s %s.

		View this account on Octopus Deploy: %s
	`,
		testAccount.Name,
		output.Dimf("(%s)", testAccount.Slug),
		output.Bluef("%s/app#/%s/infrastructure/accounts/%s", "", opts.Space.GetID(), testAccount.ID),
	), res)
}
