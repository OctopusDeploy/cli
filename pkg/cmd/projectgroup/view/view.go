package view

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"io"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

const (
	FlagWeb = "web"
)

type ViewFlags struct {
	Web *flag.Flag[bool]
}

func NewViewFlags() *ViewFlags {
	return &ViewFlags{
		Web: flag.New[bool](FlagWeb, false),
	}
}

type ViewOptions struct {
	Client   *client.Client
	Host     string
	out      io.Writer
	idOrName string
	flags    *ViewFlags
}

func NewCmdView(f factory.Factory) *cobra.Command {
	viewFlags := NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id> | <slug>}",
		Short: "View a project group",
		Long:  "View a project group in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project-group view 'Default Project Group'
			$ %[1]s project-group view ProjectGroups-9000
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}

			opts := &ViewOptions{
				client,
				f.GetCurrentHost(),
				cmd.OutOrStdout(),
				args[0],
				viewFlags,
			}

			return viewRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&viewFlags.Web.Value, viewFlags.Web.Name, "w", false, "Open in web browser")

	return cmd
}

func viewRun(opts *ViewOptions) error {
	projectGroup, err := opts.Client.ProjectGroups.GetByIDOrName(opts.idOrName)
	if err != nil {
		return err
	}

	fmt.Fprintf(opts.out, "%s %s\n", output.Bold(projectGroup.GetName()), output.Dimf("(%s)", projectGroup.GetID()))

	if projectGroup.Description == "" {
		fmt.Fprintln(opts.out, output.Dim(constants.NoDescription))
	} else {
		fmt.Fprintln(opts.out, output.Dim(projectGroup.Description))
	}

	projects, err := opts.Client.ProjectGroups.GetProjects(projectGroup)
	if err != nil {
		return err
	}
	fmt.Fprintf(opts.out, output.Cyan("\nProjects:\n"))
	for _, project := range projects {
		fmt.Fprintf(opts.out, "%s (%s)\n", output.Bold(project.GetName()), project.Slug)
	}

	url := fmt.Sprintf("%s/app#/%s/projects?projectGroupId=%s", opts.Host, projectGroup.SpaceID, projectGroup.GetID())

	// footer
	fmt.Fprintf(opts.out, "\nView this project group in Octopus Deploy: %s\n", output.Blue(url))

	if opts.flags.Web.Value {
		browser.OpenURL(url)
	}

	return nil
}
