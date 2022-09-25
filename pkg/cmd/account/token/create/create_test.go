package create_test

import (
	"bytes"
	"net/url"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/account/token/create"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/stretchr/testify/assert"
)

var serverUrl, _ = url.Parse("http://server")

const placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

var rootResource = testutil.NewRootResource()

func TestTokenAccountCreatePropmtMissing(t *testing.T) {
	const spaceID = "Space-1"
	const envID = "Env-1"
	_ = fixtures.NewSpace(spaceID, "testspace")
	env := fixtures.NewEnvironment(spaceID, envID, "testenv")
	api, qa := testutil.NewMockServerAndAsker()
	out := &bytes.Buffer{}

	opts := &create.CreateOptions{
		CreateFlags: create.NewCreateFlags(),
	}

	errReceiver := testutil.GoBegin(func() error {
		defer testutil.Close(api, qa)
		octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
		opts.Ask = qa.AsAsker()
		opts.Octopus = octopus
		opts.Writer = out
		return create.PromptMissing(opts)
	})

	api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

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

	_ = qa.ExpectQuestion(t, &survey.Password{
		Message: "Token",
		Help:    "The password to use to when authenticating against the remote host.",
	}).AnswerWith("token123")

	api.ExpectRequest(t, "GET", "/api/Spaces-1/environments").RespondWith(resources.Resources[*environments.Environment]{
		Items: []*environments.Environment{env}},
	)

	_ = qa.ExpectQuestion(t, &survey.MultiSelect{
		Message: "Choose the environments that are allowed to use this account.\nIf nothing is selected, the account can be used for deployments to any environment.",
		Options: []string{"testenv"},
	}).AnswerWith([]string{"testenv"})

	err := <-errReceiver
	assert.Nil(t, err)

	assert.Equal(t, []string{envID}, opts.Environments.Value)
	assert.Equal(t, "test 123", opts.Description.Value)
	assert.Equal(t, "TestAccount", opts.Name.Value)
	assert.Equal(t, "token123", opts.Token.Value)
}

func TestTokenAccountCreateNoPrompt(t *testing.T) {
	const spaceID = "Space-1"
	const envID = "Env-1"
	_ = fixtures.NewSpace(spaceID, "testspace")
	_ = fixtures.NewEnvironment(spaceID, envID, "testenv")
	api, qa := testutil.NewMockServerAndAsker()
	out := &bytes.Buffer{}

	opts := &create.CreateOptions{
		CreateFlags: create.NewCreateFlags(),
	}
	opts.Space = spaceID
	opts.Name.Value = "testaccount"
	opts.Token.Value = "token123"

	errReceiver := testutil.GoBegin(func() error {
		defer testutil.Close(api, qa)
		octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
		opts.Ask = qa.AsAsker()
		opts.Octopus = octopus
		opts.Writer = out
		opts.NoPrompt = true
		return create.CreateRun(opts)
	})

	testAccount, err := accounts.NewTokenAccount(
		opts.Name.Value,
		core.NewSensitiveValue(opts.Token.Value),
	)
	assert.Nil(t, err)
	testAccount.ID = "Account-1"
	testAccount.Slug = "testaccount"
	testAccount.SpaceID = spaceID

	api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
	api.ExpectRequest(t, "POST", "/api/Spaces-1/accounts").RespondWithStatus(201, "", testAccount)

	err = <-errReceiver
	assert.Nil(t, err)
	res := out.String()
	assert.Equal(t, heredoc.Docf(`
		Successfully created Token account %s %s.

		View this account on Octopus Deploy: %s
	`,
		testAccount.Name,
		output.Dimf("(%s)", testAccount.Slug),
		output.Bluef("%s/app#/%s/infrastructure/accounts/%s", "", opts.Space, testAccount.ID),
	), res)
}
