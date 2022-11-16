package worker

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdWorker(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker <command>",
		Short: "Manage workers",
		Long:  `Work with Octopus Deploy workers.`,
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s worker list
			$ %s tenant ls
		`), constants.ExecutableName, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	return cmd
}
