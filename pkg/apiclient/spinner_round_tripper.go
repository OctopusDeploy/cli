package apiclient

import (
	"net/http"
	"time"

	"github.com/briandowns/spinner"
)

type SpinnerRoundTripper struct {
	Next    http.RoundTripper
	Spinner *spinner.Spinner
}

func NewSpinnerRoundTripper() *SpinnerRoundTripper {
	return &SpinnerRoundTripper{
		Next:    http.DefaultTransport,
		Spinner: spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithColor("cyan")),
	}
}

func (c *SpinnerRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	c.Spinner.Start()
	defer c.Spinner.Stop()
	return c.Next.RoundTrip(r)
}
