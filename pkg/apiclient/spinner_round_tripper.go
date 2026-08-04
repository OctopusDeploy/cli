package apiclient

import (
	"net/http"
	"time"

	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/briandowns/spinner"
)

type SpinnerRoundTripper struct {
	Next    http.RoundTripper
	Spinner *spinner.Spinner
	Ask     question.AskProvider
}

func NewSpinnerRoundTripper(ask question.AskProvider) *SpinnerRoundTripper {
	return &SpinnerRoundTripper{
		Next:    http.DefaultTransport,
		Spinner: spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithColor("cyan")),
		Ask:     ask,
	}
}

// shouldSpin is answered per request rather than when the client is built. The
// client factory runs before cobra parses flags, so --no-prompt has not been
// applied yet at construction time; deciding there leaves the spinner running
// for commands that explicitly asked not to be interactive. The spinner redraws
// by erasing the current terminal line, which corrupts the display of anything
// else sharing that terminal.
func (c *SpinnerRoundTripper) shouldSpin() bool {
	return c.Ask != nil && c.Ask.IsInteractive()
}

func (c *SpinnerRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if !c.shouldSpin() {
		return c.Next.RoundTrip(r)
	}

	c.Spinner.Start()
	defer c.Spinner.Stop()
	return c.Next.RoundTrip(r)
}
