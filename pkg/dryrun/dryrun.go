// Package dryrun provides the shared pieces of the --dry-run flag: declaring it,
// detecting it, the banners a dry run prints, and the guard that keeps it honest.
//
// The flag is declared per command rather than persistently on the root command.
// A persistent flag would be accepted everywhere, including by the commands which
// have not implemented it, and silently mutating Octopus while the caller believes
// the run was a rehearsal is worse than having no flag at all. Declaring it locally
// means `--dry-run` on an unsupported command fails with "unknown flag".
//
// GuardRoundTripper is the second half of that guarantee. Once a dry run is under
// way, any request that would change server state is refused before it is sent, so
// a partially implemented dry run fails loudly instead of quietly mutating.
package dryrun

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const FlagDescription = "Show what would happen, without making any changes in Octopus"

// AddFlag declares --dry-run on a command that genuinely supports it.
func AddFlag(flags *pflag.FlagSet, value *bool) {
	flags.BoolVar(value, constants.FlagDryRun, false, FlagDescription)
}

// IsEnabled reports whether the command being executed declared --dry-run and it was set.
func IsEnabled(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	enabled, err := cmd.Flags().GetBool(constants.FlagDryRun)
	if err != nil { // the command doesn't declare the flag
		return false
	}
	return enabled
}

// Header opens a dry run's output.
func Header(cmd *cobra.Command) {
	cmd.Printf("%s no changes will be made in Octopus.\n\n", output.Yellow("DRY RUN:"))
}

// Footer closes a dry run's output, restating that nothing happened.
func Footer(cmd *cobra.Command, summary string) {
	cmd.Printf("\n%s %s\n", output.Yellow("DRY RUN:"), summary)
}

// BlockedError is returned when a dry run attempts a request that would change server state.
type BlockedError struct {
	Method string
	URL    string
}

func (e *BlockedError) Error() string {
	return fmt.Sprintf("dry run blocked a %s request to %s; this command does not fully support --dry-run, please raise an issue", e.Method, e.URL)
}

// GuardRoundTripper refuses to send anything other than a read-only request.
type GuardRoundTripper struct {
	Next http.RoundTripper
}

func NewGuardRoundTripper(next http.RoundTripper) *GuardRoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return &GuardRoundTripper{Next: next}
}

func (g *GuardRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if !isReadOnly(r.Method) {
		url := ""
		if r.URL != nil {
			url = r.URL.Path
		}
		return nil, &BlockedError{Method: strings.ToUpper(r.Method), URL: url}
	}
	return g.Next.RoundTrip(r)
}

func isReadOnly(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}
