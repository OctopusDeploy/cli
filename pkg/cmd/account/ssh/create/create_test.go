package create_test

import (
	"bytes"
	"encoding/base64"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"net/url"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/account/ssh/create"
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

func TestGCPAccountCreatePromptMissing(t *testing.T) {
	const spaceID = "Space-1"
	const envID = "Env-1"
	_ = fixtures.NewSpace(spaceID, "testspace")
	env := fixtures.NewEnvironment(spaceID, envID, "testenv")
	api, qa := testutil.NewMockServerAndAsker()
	out := &bytes.Buffer{}

	opts := &create.CreateOptions{
		CreateFlags:  create.NewCreateFlags(),
		Dependencies: &cmd.Dependencies{},
		GetAllEnvironmentsCallback: func() ([]*environments.Environment, error) {
			return []*environments.Environment{env}, nil
		},
	}

	opts.KeyFileData = []byte{1, 1}

	errReceiver := testutil.GoBegin(func() error {
		defer testutil.Close(api, qa)
		octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
		opts.Ask = qa.AsAsker()
		opts.Client = octopus
		opts.Out = out
		return create.PromptMissing(opts)
	})

	api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
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
		Message: "Username",
		Help:    "The username to use when authenticating against the remote host.",
	}).AnswerWith("username123")

	_ = qa.ExpectQuestion(t, &survey.Password{
		Message: "Passphrase",
		Help:    "The passphrase for the private key, if required.",
	}).AnswerWith("password123")

	_ = qa.ExpectQuestion(t, &survey.MultiSelect{
		Message: "Choose the environments that are allowed to use this account.\nIf nothing is selected, the account can be used for deployments to any environment.",
		Options: []string{"testenv"},
	}).AnswerWith([]string{"testenv"})

	err := <-errReceiver
	assert.Nil(t, err)

	assert.Equal(t, []string{envID}, opts.Environments.Value)
	assert.Equal(t, "test 123", opts.Description.Value)
	assert.Equal(t, "TestAccount", opts.Name.Value)
	assert.Equal(t, "username123", opts.Username.Value)
	assert.Equal(t, "password123", opts.Passphrase.Value)
}

func TestGCPAccountCreateNoPrompt(t *testing.T) {
	const spaceID = "Space-1"
	const envID = "Env-1"
	_ = fixtures.NewSpace(spaceID, "testspace")
	_ = fixtures.NewEnvironment(spaceID, envID, "testenv")
	api, qa := testutil.NewMockServerAndAsker()
	out := &bytes.Buffer{}

	opts := &create.CreateOptions{
		CreateFlags:  create.NewCreateFlags(),
		Dependencies: &cmd.Dependencies{Space: &spaces.Space{}},
	}
	opts.Space.ID = spaceID

	opts.Name.Value = "testaccount"
	opts.KeyFileData = []byte{1, 1}
	opts.Username.Value = "username123"
	opts.Passphrase.Value = "passphrase"

	errReceiver := testutil.GoBegin(func() error {
		defer testutil.Close(api, qa)
		octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
		opts.Ask = qa.AsAsker()
		opts.Client = octopus
		opts.Out = out
		opts.NoPrompt = true
		return create.CreateRun(opts)
	})

	testAccount, err := accounts.NewSSHKeyAccount(
		opts.Name.Value,
		opts.Username.Value,
		core.NewSensitiveValue(base64.StdEncoding.EncodeToString(opts.KeyFileData)),
	)
	assert.Nil(t, err)
	testAccount.ID = "Account-1"
	testAccount.Slug = "testaccount"
	testAccount.SpaceID = spaceID
	testAccount.PrivateKeyPassphrase = core.NewSensitiveValue(opts.Passphrase.Value)

	api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
	api.ExpectRequest(t, "GET", "/api/spaces").RespondWith(rootResource)
	api.ExpectRequest(t, "POST", "/api/Spaces-1/accounts").RespondWithStatus(201, "", testAccount)

	err = <-errReceiver
	assert.Nil(t, err)
	res := out.String()
	assert.Equal(t, heredoc.Docf(`
		Successfully created SSH account %s %s.

		View this account on Octopus Deploy: %s
	`,
		testAccount.Name,
		output.Dimf("(%s)", testAccount.Slug),
		output.Bluef("%s/app#/%s/infrastructure/accounts/%s", "", opts.Space.GetID(), testAccount.ID),
	), res)
}
