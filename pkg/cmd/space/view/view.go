package view

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/cobra"
)

type ViewOptions struct {
	Host        string
	IO          io.Writer
	SelectorArg string
	Space       *spaces.Space
}

func NewCmdView(f apiclient.ClientFactory) *cobra.Command {
	opts := &ViewOptions{}

	cmd := &cobra.Command{
		Use:   "view",
		Short: "View a space in an instance of Octopus Deploy",
		Long:  "View a space in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s space view"
		`), constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.Get(true)
			if err != nil {
				return err
			}

			// TODO: validate arguments

			space, err := client.Spaces.GetByIDOrName(args[0])
			if err != nil {
				return err
			}

			opts.Host = os.Getenv("OCTOPUS_HOST")
			opts.Space = space
			opts.IO = cmd.OutOrStdout()

			return viewRun(opts)
		},
	}

	return cmd
}

func viewRun(opts *ViewOptions) error {
	return printHumanSpacePreview(opts)
}

func printHumanSpacePreview(opts *ViewOptions) error {
	out := opts.IO
	space := opts.Space

	// header
	fmt.Fprintf(out, "%s (%s)\n", output.Bold(space.Name), space.GetID())

	// metadata
	if len(space.Description) == 0 {
		fmt.Fprintln(out, output.Dim("No description provided"))

	} else {
		fmt.Fprintln(out, output.Dim(space.Description))
	}

	// task processing
	taskQueue := output.Green("Running")
	if space.TaskQueueStopped {
		taskQueue = output.Yellow("Stopped")
	}
	fmt.Fprintf(out, "Task Processing Status: %s\n", taskQueue)

	// BUG: the hypermedia link, "Web" is not represented correctly in Octopus REST API
	link := strings.Split(space.Links["Web"], "/app#/")
	webLink := "/app#/configuration/" + link[1]
	spaceURL := opts.Host + webLink

	// footer
	fmt.Fprintf(out, output.Dim("View this space in Octopus Deploy: %s\n"), spaceURL)

	return nil
}
