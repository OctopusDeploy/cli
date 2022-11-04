package view

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"io"
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
		Short: "View a project in an instance of Octopus Deploy",
		Long:  "View a project in an instance of Octopus Deploy",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s project view 'Deploy Web App'
			$ %s project view Projects-9000
			$ %s project view deploy-web-app
		`), constants.ExecutableName, constants.ExecutableName, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient()
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
	project, err := opts.Client.Projects.GetByIdentifier(opts.idOrName)
	if err != nil {
		return err
	}

	fmt.Fprintf(opts.out, "%s %s\n", output.Bold(project.Name), output.Dimf("(%s)", project.Slug))

	cacBranch := "Not version controlled"
	if project.IsVersionControlled {
		cacBranch = project.PersistenceSettings.(projects.GitPersistenceSettings).DefaultBranch()
	}
	fmt.Fprintf(opts.out, "Version control branch: %s\n", output.Cyan(cacBranch))
	if project.Description == "" {
		fmt.Fprintln(opts.out, output.Dim(constants.NoDescription))
	} else {
		fmt.Fprintln(opts.out, output.Dim(project.Description))
	}

	url := opts.Host + project.Links["Web"]

	// footer
	fmt.Fprintf(opts.out, "View this project in Octopus Deploy: %s\n", output.Blue(url))

	if opts.flags.Web.Value {
		browser.OpenURL(url)
	}

	return nil
}
