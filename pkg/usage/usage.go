package usage

import (
	"fmt"
	"github.com/spf13/cobra"
)

// UsageError indicates the caller has not invoked the CLI properly
// and the root error handler should print the usage/help
type UsageError struct {
	s string

	// this needs to carry the command, otherwise the root cmd doesn't know how to print usage
	cmd *cobra.Command
}

func NewUsageError(s string, cmd *cobra.Command) *UsageError {
	return &UsageError{s: s, cmd: cmd}
}

func (e *UsageError) Error() string {
	return e.s
}

func (e *UsageError) Command() *cobra.Command {
	return e.cmd
}

// Argument validation helper which emits a UsageError rather than a plain string error
func ExactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return NewUsageError(
				fmt.Sprintf("accepts %d arg(s), received %d", n, len(args)),
				cmd)
		}
		return nil
	}
}
