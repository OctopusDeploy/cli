package create

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/cobra"
	"io"
)

const (
	FlagGroup       = "group"
	FlagName        = "name"
	FlagDescription = "description"
	FlagLifecycle   = "lifecycle"
)

type CreateFlags struct {
	Group       *flag.Flag[string]
	Name        *flag.Flag[string]
	Description *flag.Flag[string]
	Lifecycle   *flag.Flag[string]
}

type CreateOptions struct {
	*CreateFlags
	out    io.Writer
	client *client.Client
	host   string
	space  *spaces.Space
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Group:       flag.New[string](FlagGroup, false),
		Name:        flag.New[string](FlagName, false),
		Description: flag.New[string](FlagDescription, false),
		Lifecycle:   flag.New[string](FlagLifecycle, false),
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	opts := &CreateOptions{
		CreateFlags: NewCreateFlags(),
	}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a new project in Octopus Deploy",
		Long:  "Creates a new project in Octopus Deploy.",
		Example: heredoc.Doc(`
			$ octopus project create .... fill this in later
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient()
			if err != nil {
				return err
			}
			opts.out = cmd.OutOrStdout()
			opts.client = client
			opts.host = f.GetCurrentHost()
			opts.space = f.GetCurrentSpace()

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.CreateFlags.Name.Value, opts.CreateFlags.Name.Name, "n", "", "Name of the project")
	flags.StringVarP(&opts.CreateFlags.Description.Value, opts.CreateFlags.Description.Name, "d", "", "Description of the project")
	flags.StringVarP(&opts.CreateFlags.Group.Value, opts.CreateFlags.Group.Name, "g", "", "Project group of the project")
	flags.StringVarP(&opts.CreateFlags.Lifecycle.Value, opts.CreateFlags.Lifecycle.Name, "l", "", "Lifecycle of the project")
	flags.SortFlags = false

	return cmd
}

func createRun(opts *CreateOptions) error {

	fmt.Println(opts.CreateFlags)
	fmt.Println(opts.CreateFlags.Lifecycle)
	fmt.Println(opts.CreateFlags.Lifecycle.Value)
	lifecycle, err := opts.client.Lifecycles.GetByIDOrName(opts.CreateFlags.Lifecycle.Value)
	if err != nil {
		return err
	}

	projectGroup, err := opts.client.ProjectGroups.GetByIDOrName(opts.CreateFlags.Group.Value)
	if err != nil {
		return err
	}

	project := projects.NewProject(opts.CreateFlags.Name.Value, lifecycle.ID, projectGroup.ID)
	project.Description = opts.CreateFlags.Description.Value

	createdProject, err := opts.client.Projects.Add(project)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.out, "Successfully created project %s %s.\n", createdProject.Name, createdProject.Slug)
	if err != nil {
		return err
	}

	link := output.Bluef("%s/app#/%s/projects/%s", opts.host, opts.space.GetID(), createdProject.GetID())
	fmt.Fprintf(opts.out, "\nView this account on Octopus Deploy: %s\n", link)

	return nil

}
