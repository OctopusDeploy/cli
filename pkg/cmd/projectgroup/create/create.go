package create

import (
	"fmt"
	"io"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/cobra"
)

const (
	FlagName        = "name"
	FlagDescription = "description"
)

type CreateFlags struct {
	Name        *flag.Flag[string]
	Description *flag.Flag[string]
}

type CreateOptions struct {
	*CreateFlags
	Out               io.Writer
	Client            *client.Client
	Host              string
	Space             *spaces.Space
	NoPrompt          bool
	Ask               question.Asker
	CmdPath           string
	ShowMessagePrefix bool
}

func (co *CreateOptions) Commit() error {
	projectGroup := projectgroups.NewProjectGroup(co.Name.Value)
	projectGroup.Description = co.Description.Value

	createdGroupProject, err := co.Client.ProjectGroups.Add(projectGroup)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(co.Out, "\nSuccessfully created project group %s.\n", createdGroupProject.Name)
	if err != nil {
		return err
	}
	link := output.Bluef("%s/app#/%s/projectGroups/%s", co.Host, co.Space.GetID(), createdGroupProject.GetID())
	fmt.Fprintf(co.Out, "View this project group on Octopus Deploy: %s\n", link)
	return nil
}

func (co *CreateOptions) GenerateAutomationCmd() {
	if !co.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(co.CmdPath, co.Name, co.Description)
		fmt.Fprintf(co.Out, "%s\n", autoCmd)
	}
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:        flag.New[string](FlagName, false),
		Description: flag.New[string](FlagDescription, false),
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	opts := &CreateOptions{
		Ask:         f.Ask,
		CreateFlags: NewCreateFlags(),
	}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a new project group in Octopus Deploy",
		Long:  "Creates a new project group in Octopus Deploy.",
		Example: heredoc.Doc(`
			$ octopus project-group create
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient()
			if err != nil {
				return err
			}
			opts.CmdPath = cmd.CommandPath()
			opts.Out = cmd.OutOrStdout()
			opts.Client = client
			opts.Host = f.GetCurrentHost()
			opts.NoPrompt = !f.IsPromptEnabled()
			opts.Space = f.GetCurrentSpace()

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Name.Value, opts.CreateFlags.Name.Name, "n", "", "Name of the project group")
	flags.StringVarP(&opts.Description.Value, opts.CreateFlags.Description.Name, "d", "", "Description of the project group")
	flags.SortFlags = false

	return cmd
}

func createRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	if err := opts.Commit(); err != nil {
		return err
	}
	if !opts.NoPrompt {
		fmt.Fprint(opts.Out, "Automation Command: ")
		opts.GenerateAutomationCmd()
	}

	return nil
}

func PromptMissing(opts *CreateOptions) error {
	messagePrefix := ""
	if opts.ShowMessagePrefix {
		messagePrefix = "Project Group "
	}

	if opts.Name.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: messagePrefix + "Name",
			Help:    "A short, memorable, unique name for this project group.",
		}, &opts.Name.Value, survey.WithValidator(survey.ComposeValidators(
			survey.MaxLength(200),
			survey.MinLength(1),
			survey.Required,
		))); err != nil {
			return err
		}
	}

	if opts.Description.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: messagePrefix + "Description",
			Help:    "A short, memorable, unique name for this project.",
		}, &opts.Description.Value); err != nil {
			return err
		}
	}

	return nil
}
