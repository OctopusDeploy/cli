package view

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"io"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/usage"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/cobra"
)

type ViewOptions struct {
	Client   *client.Client
	Host     string
	IO       io.Writer
	Selector string
	Space    *spaces.Space
}

func NewCmdView(f factory.Factory) *cobra.Command {
	opts := &ViewOptions{}

	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id>}",
		Short: "View a space",
		Long:  "View a space in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s space view Spaces-9000
			$ %[1]s space view Integrations
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSystemClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}

			opts.Client = client
			opts.Host = f.GetCurrentHost()
			opts.IO = cmd.OutOrStdout()
			opts.Selector = args[0]

			return viewRun(opts)
		},
	}

	return cmd
}

func viewRun(opts *ViewOptions) error {
	space, err := opts.Client.Spaces.GetByIDOrName(opts.Selector)
	if err != nil {
		return err
	}

	return printHumanSpacePreview(opts.Host, space, opts.IO)
}

func printHumanSpacePreview(host string, space *spaces.Space, out io.Writer) error {
	// header
	fmt.Fprintf(out, "%s %s\n", output.Bold(space.Name), output.Dimf("(%s)", space.GetID()))

	// metadata
	if len(space.Description) == 0 {
		fmt.Fprintln(out, output.Dim(constants.NoDescription))

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
	spaceURL := host + webLink

	// footer
	fmt.Fprintf(out, "View this space in Octopus Deploy: %s\n", output.Blue(spaceURL))

	return nil
}
