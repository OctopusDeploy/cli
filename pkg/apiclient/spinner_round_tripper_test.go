package apiclient_test

import (
	"net/http"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/stretchr/testify/assert"
)

type fakeAskProvider struct{ interactive bool }

func (f *fakeAskProvider) IsInteractive() bool { return f.interactive }
func (f *fakeAskProvider) DisableInteractive() { f.interactive = false }
func (f *fakeAskProvider) Ask(survey.Prompt, interface{}, ...survey.AskOpt) error {
	return nil
}

type recordingTransport struct{ called bool }

func (t *recordingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.called = true
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func TestSpinnerRoundTripper_SpinsWhenInteractive(t *testing.T) {
	assert.True(t, apiclient.NewSpinnerRoundTripper(&fakeAskProvider{interactive: true}).ShouldSpin())
}

// The spinner redraws by erasing the current terminal line, which corrupts
// anything else sharing that terminal, so a command that asked not to be
// interactive must not produce one.
func TestSpinnerRoundTripper_DoesNotSpinWhenNotInteractive(t *testing.T) {
	assert.False(t, apiclient.NewSpinnerRoundTripper(&fakeAskProvider{interactive: false}).ShouldSpin())
}

func TestSpinnerRoundTripper_DoesNotSpinWithoutAnAskProvider(t *testing.T) {
	assert.False(t, apiclient.NewSpinnerRoundTripper(nil).ShouldSpin())
}

// The client is built before cobra parses --no-prompt, so the answer has to be
// read at request time rather than captured when the round-tripper is created.
func TestSpinnerRoundTripper_HonoursInteractivityChangedAfterConstruction(t *testing.T) {
	ask := &fakeAskProvider{interactive: true}
	roundTripper := apiclient.NewSpinnerRoundTripper(ask)
	assert.True(t, roundTripper.ShouldSpin())

	ask.DisableInteractive()

	assert.False(t, roundTripper.ShouldSpin(), "interactivity must be re-read per request")
}

func TestSpinnerRoundTripper_PassesTheRequestOn(t *testing.T) {
	for _, interactive := range []bool{true, false} {
		roundTripper := apiclient.NewSpinnerRoundTripper(&fakeAskProvider{interactive: interactive})
		next := &recordingTransport{}
		roundTripper.Next = next

		request, err := http.NewRequest(http.MethodGet, "http://example.invalid/api", nil)
		assert.NoError(t, err)

		response, err := roundTripper.RoundTrip(request)

		assert.NoError(t, err)
		assert.True(t, next.called)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	}
}
