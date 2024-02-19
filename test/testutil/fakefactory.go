package testutil

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/config"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
)

type FakeSpinner struct{}

func (f *FakeSpinner) Start() {}
func (f *FakeSpinner) Stop()  {}

type FakeConfigProvider struct {
	config map[string]string
}

func (f *FakeConfigProvider) Get(key string) string {
	return f.config[key]
}

func (f *FakeConfigProvider) Set(key string, value string) error {
	if f.config == nil {
		f.config = make(map[string]string)
	}
	f.config[key] = value
	return nil
}

func NewMockFactory(api *MockHttpServer) *MockFactory {
	if api == nil {
		panic("api MockHttpServer can't be nil")
	}
	return &MockFactory{
		api:            api,
		RawSpinner:     &FakeSpinner{},
		ConfigProvider: &FakeConfigProvider{},
	}
}

func NewMockFactoryWithSpace(api *MockHttpServer, space *spaces.Space) *MockFactory {
	return NewMockFactoryWithSpaceAndPrompt(api, space, nil)
}

func NewMockFactoryWithSpaceAndPrompt(api *MockHttpServer, space *spaces.Space, askProvider question.AskProvider) *MockFactory {
	result := NewMockFactory(api)
	result.CurrentSpace = space
	result.AskProvider = askProvider
	return result
}

type MockFactory struct {
	api               *MockHttpServer          // must not be nil
	SystemClient      *octopusApiClient.Client // nil; lazily created like with the real factory
	SpaceScopedClient *octopusApiClient.Client // nil; lazily created like with the real factory
	CurrentSpace      *spaces.Space
	RawSpinner        factory.Spinner
	AskProvider       question.AskProvider
	ConfigProvider    config.IConfigProvider
}

// refactor this later if there's ever a need for unit tests to vary the server url or API key (why would there be?)
const serverUrl = "http://server"
const placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

func (f *MockFactory) GetSystemClient(_ apiclient.Requester) (*octopusApiClient.Client, error) {
	serverUrl, _ := url.Parse(serverUrl)

	if f.SystemClient == nil {
		octopus, err := octopusApiClient.NewClient(NewMockHttpClientWithTransport(f.api), serverUrl, placeholderApiKey, "")
		if err != nil {
			return nil, err
		}
		f.SystemClient = octopus
	}
	return f.SystemClient, nil
}
func (f *MockFactory) GetSpacedClient(_ apiclient.Requester) (*octopusApiClient.Client, error) {
	if f.CurrentSpace == nil {
		return nil, errors.New("can't get space-scoped client from MockFactory while CurrentSpace is nil")
	}
	serverUrl, _ := url.Parse(serverUrl)
	if f.SpaceScopedClient == nil {
		octopus, err := octopusApiClient.NewClient(NewMockHttpClientWithTransport(f.api), serverUrl, placeholderApiKey, f.CurrentSpace.ID)
		if err != nil {
			return nil, err
		}
		f.SpaceScopedClient = octopus
	}
	return f.SpaceScopedClient, nil
}
func (f *MockFactory) GetCurrentSpace() *spaces.Space {
	return f.CurrentSpace
}
func (f *MockFactory) GetCurrentHost() string {
	return serverUrl
}
func (f *MockFactory) GetHttpClient() (*http.Client, error) {
	return NewMockHttpClientWithTransport(f.api), nil
}
func (f *MockFactory) Spinner() factory.Spinner {
	return f.RawSpinner
}
func (f *MockFactory) BuildVersion() string {
	return "0.0.0-test"
}
func (f *MockFactory) IsPromptEnabled() bool {
	if f.AskProvider == nil {
		return false
	}
	return f.AskProvider.IsInteractive()
}
func (f *MockFactory) Ask(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
	if f.AskProvider == nil {
		return errors.New("method Ask called on fake factory when provider was nil")
	}
	return f.AskProvider.Ask(p, response, opts...)
}
func (f *MockFactory) GetConfigProvider() (config.IConfigProvider, error) {
	return f.ConfigProvider, nil
}
