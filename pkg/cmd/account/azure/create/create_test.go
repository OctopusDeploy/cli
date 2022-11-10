package create_test

import (
	"bytes"
	"net/url"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/account/azure/create"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

var serverUrl, _ = url.Parse("http://server")

const placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

var rootResource = testutil.NewRootResource()

func TestAzureAccountCreatePromptMissing(t *testing.T) {
	const spaceID = "Space-1"
	const envID = "Env-1"
	_ = fixtures.NewSpace(spaceID, "testspace")
	env := fixtures.NewEnvironment(spaceID, envID, "testenv")
	api, qa := testutil.NewMockServerAndAsker()
	out := &bytes.Buffer{}

	opts := &create.CreateOptions{
		CreateFlags: create.NewCreateFlags(),
		GetAllEnvironmentsCallback: func() ([]*environments.Environment, error) {
			return []*environments.Environment{env}, nil
		},
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

	_ = qa.ExpectQuestion(t, &survey.Input{
		Message: "Subscription ID",
		Help:    "Your Azure subscription ID. This is a GUID in the format xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.",
	}).AnswerWith("d2486c05-0cac-4d54-a91e-654043036f31")

	_ = qa.ExpectQuestion(t, &survey.Input{
		Message: "Tenant ID",
		Help:    "Your Azure Active Directory Tenant ID. This is a GUID in the format xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.",
	}).AnswerWith("d2486c05-0cac-4d54-a91e-654043036f32")

	_ = qa.ExpectQuestion(t, &survey.Input{
		Message: "Application ID",
		Help:    "Your Azure Active Directory Tenant ID. This is a GUID in the format xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.",
	}).AnswerWith("d2486c05-0cac-4d54-a91e-654043036f33")

	_ = qa.ExpectQuestion(t, &survey.Password{
		Message: "Application Password / Key",
		Help:    "The password for the Azure Active Directory application. This value is known as Key in the Azure Portal, and Password in the API.",
	}).AnswerWith("password123")

	_ = qa.ExpectQuestion(t, &survey.Confirm{
		Message: "Configure isolated Azure Environment connection.",
		Default: false,
	}).AnswerWith(false)

	_ = qa.ExpectQuestion(t, &survey.MultiSelect{
		Message: "Choose the environments that are allowed to use this account.\nIf nothing is selected, the account can be used for deployments to any environment.",
		Options: []string{"testenv"},
	}).AnswerWith([]string{"testenv"})

	err := <-errReceiver
	assert.Nil(t, err)

	assert.Equal(t, []string{envID}, opts.Environments.Value)
	assert.Equal(t, "test 123", opts.Description.Value)
	assert.Equal(t, "TestAccount", opts.Name.Value)
	assert.Equal(t, "d2486c05-0cac-4d54-a91e-654043036f31", opts.SubscriptionID.Value)
	assert.Equal(t, "d2486c05-0cac-4d54-a91e-654043036f32", opts.TenantID.Value)
	assert.Equal(t, "d2486c05-0cac-4d54-a91e-654043036f33", opts.ApplicationID.Value)
	assert.Equal(t, "password123", opts.ApplicationPasswordKey.Value)
}

func TestAzureAccountCreateNoPrompt(t *testing.T) {
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
	opts.ApplicationPasswordKey.Value = "password123"
	opts.SubscriptionID.Value = "d2486c05-0cac-4d54-a91e-654043036f31"
	opts.TenantID.Value = "d2486c05-0cac-4d54-a91e-654043036f32"
	opts.ApplicationID.Value = "d2486c05-0cac-4d54-a91e-654043036f33"
	opts.Environments.Value = []string{envID}

	errReceiver := testutil.GoBegin(func() error {
		defer testutil.Close(api, qa)
		octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
		opts.Ask = qa.AsAsker()
		opts.Octopus = octopus
		opts.Writer = out
		opts.NoPrompt = true
		return create.CreateRun(opts)
	})

	testAccount, err := accounts.NewAzureServicePrincipalAccount(
		opts.Name.Value,
		uuid.MustParse(opts.SubscriptionID.Value),
		uuid.MustParse(opts.TenantID.Value),
		uuid.MustParse(opts.ApplicationID.Value),
		core.NewSensitiveValue(opts.ApplicationPasswordKey.Value),
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
		Successfully created Azure account %s %s.

		View this account on Octopus Deploy: %s
	`,
		testAccount.Name,
		output.Dimf("(%s)", testAccount.Slug),
		output.Bluef("%s/app#/%s/infrastructure/accounts/%s", "", opts.Space, testAccount.ID),
	), res)
}
