package testutil

import (
	"errors"
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/factory"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"net/url"
)

type FakeSpinner struct{}

func (f *FakeSpinner) Start() {}
func (f *FakeSpinner) Stop()  {}

func NewMockFactory(api *MockHttpServer) *MockFactory {
	if api == nil {
		panic("api MockHttpServer can't be nil")
	}
	return &MockFactory{
		api:        api,
		RawSpinner: &FakeSpinner{},
	}
}

func NewMockFactoryWithSpace(api *MockHttpServer, space *spaces.Space) *MockFactory {
	result := NewMockFactory(api)
	result.CurrentSpace = space
	return result
}

type MockFactory struct {
	api           *MockHttpServer          // must not be nil
	ApiClient     *octopusApiClient.Client // nil; lazily created like with the real factory
	CurrentSpace  *spaces.Space
	RawSpinner    factory.Spinner
	PromptEnabled bool
	Asker         func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error
}

// refactor this later if there's ever a need for unit tests to vary the server url or API key (why would there be?)
const serverUrl = "http://server"
const placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

func (f *MockFactory) GetSystemClient() (*octopusApiClient.Client, error) {
	serverUrl, _ := url.Parse(serverUrl)

	if f.ApiClient == nil {
		octopus, err := octopusApiClient.NewClient(NewMockHttpClientWithTransport(f.api), serverUrl, placeholderApiKey, "")
		if err != nil {
			return nil, err
		}
		f.ApiClient = octopus
	}
	return f.ApiClient, nil
}
func (f *MockFactory) GetSpacedClient() (*octopusApiClient.Client, error) {
	if f.CurrentSpace == nil {
		return nil, errors.New("can't get space-scoped client from MockFactory while CurrentSpace is nil")
	}
	return f.GetSystemClient() // not meaningful in unit tests
}
func (f *MockFactory) GetCurrentSpace() *spaces.Space {
	return f.CurrentSpace
}
func (f *MockFactory) GetCurrentHost() string {
	return serverUrl
}
func (f *MockFactory) Spinner() factory.Spinner {
	return f.RawSpinner
}
func (f *MockFactory) IsPromptEnabled() bool {
	return f.PromptEnabled
}
func (f *MockFactory) Ask(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
	return f.Asker(p, response, opts...)
}
