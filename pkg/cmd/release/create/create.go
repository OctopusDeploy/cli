package release

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/executor"
	"io"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

const (
	flagProject      = "project"
	flagReleaseNotes = "release-notes"
	flagChannel      = "channel"
	flagVersion      = "version"
)

func NewCmdCreate(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a release in an instance of Octopus Deploy",
		Long:  "Creates a release in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s release create"
		`), constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := cmd.Flags().GetString(flagProject)
			if err != nil {
				return err
			}

			t := &executor.TaskOptionsCreateRelease{
				ProjectName: project,
			}
			if rn, err := cmd.Flags().GetString(flagReleaseNotes); err != nil && rn != "" {
				t.ReleaseNotes = rn
			}
			if ch, err := cmd.Flags().GetString(flagChannel); err != nil && ch != "" {
				t.ChannelName = ch
			}
			if v, err := cmd.Flags().GetString(flagVersion); err != nil && v != "" {
				t.Version = v
			}

			return createRun(f, cmd.OutOrStdout(), t)
		},
	}

	cmd.Flags().StringP(flagProject, "p", "", "Name or ID of the project to create the release in. Required")
	_ = cmd.MarkFlagRequired(flagProject)

	cmd.Flags().StringP(flagReleaseNotes, "n", "", "Release notes to attach")
	cmd.Flags().StringP(flagChannel, "c", "", "Channel to use")
	cmd.Flags().StringP(flagVersion, "v", "", "Version Override")

	return cmd
}

func createRun(f factory.Factory, w io.Writer, options *executor.TaskOptionsCreateRelease) error {
	// TODO go through the UI flow and prompt for any values that have not already been specified from flags
	// At this point our options should be fully populated

	return executor.ProcessTasks(f, []*executor.Task{executor.NewTask(executor.TaskTypeCreateRelease, options)})
}
