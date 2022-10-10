package create

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
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
	Out      io.Writer
	Client   *client.Client
	Host     string
	Space    *spaces.Space
	NoPrompt bool
	Ask      question.Asker
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
		Ask:         f.Ask,
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
			opts.Out = cmd.OutOrStdout()
			opts.Client = client
			opts.Host = f.GetCurrentHost()
			opts.NoPrompt = !f.IsPromptEnabled()
			opts.Space = f.GetCurrentSpace()

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
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	lifecycle, err := opts.Client.Lifecycles.GetByIDOrName(opts.CreateFlags.Lifecycle.Value)
	if err != nil {
		return err
	}

	projectGroup, err := opts.Client.ProjectGroups.GetByIDOrName(opts.CreateFlags.Group.Value)
	if err != nil {
		return err
	}

	project := projects.NewProject(opts.CreateFlags.Name.Value, lifecycle.ID, projectGroup.ID)
	project.Description = opts.CreateFlags.Description.Value

	createdProject, err := opts.Client.Projects.Add(project)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.Out, "Successfully created project %s (%s), with lifecycle %s in project group %s.\n", createdProject.Name, createdProject.Slug, opts.Lifecycle.Value, opts.Group.Value)
	if err != nil {
		return err
	}

	link := output.Bluef("%s/app#/%s/projects/%s", opts.Host, opts.Space.GetID(), createdProject.GetID())
	fmt.Fprintf(opts.Out, "\nView this project on Octopus Deploy: %s\n", link)

	return nil
}

func PromptMissing(opts *CreateOptions) error {
	if opts.Name.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Name",
			Help:    "A short, memorable, unique name for this project.",
		}, &opts.Name.Value, survey.WithValidator(survey.ComposeValidators(
			survey.MaxLength(200),
			survey.MinLength(1),
			survey.Required,
		))); err != nil {
			return err
		}
	}

	if opts.Lifecycle.Value == "" {
		lc, err := selectors.Lifecycle("You have not specified a Lifecycle for this project. Please select one:", opts.Client, opts.Ask)
		if err != nil {
			return err
		}
		opts.Lifecycle.Value = lc.Name
	}

	if opts.Group.Value == "" {
		g, err := selectors.ProjectGroup("You have not specified a Project group for this project. Please select one:", opts.Client, opts.Ask)
		if err != nil {
			return err
		}
		opts.Group.Value = g.Name
	}

	return nil
}
