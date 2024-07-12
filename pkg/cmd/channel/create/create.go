package create

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/lifecycles"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"
)

const (
	FlagProject     = "project"
	FlagName        = "name"
	FlagDescription = "description"
	FlagDefault     = "default"
	FlagLifecycle   = "lifecycle"
)

type CreateFlags struct {
	Project     *flag.Flag[string]
	Name        *flag.Flag[string]
	Description *flag.Flag[string]
	Default     *flag.Flag[bool]
	Lifecycle   *flag.Flag[string]
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Project:     flag.New[string](FlagProject, false),
		Name:        flag.New[string](FlagName, false),
		Description: flag.New[string](FlagDescription, false),
		Default:     flag.New[bool](FlagDefault, false),
		Lifecycle:   flag.New[string](FlagLifecycle, false),
	}
}

type CreateOptions struct {
	*CreateFlags
	*cmd.Dependencies
}

func NewCreateOptions(createFlags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:  createFlags,
		Dependencies: dependencies,
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a channel",
		Long:  "Create a channel in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s channel create
			$ %[1]s channel create --name "The Channel" --project "The Project" --lifecycle "Default Lifecycle" --default
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "Name of the channel")
	flags.StringVarP(&createFlags.Project.Value, createFlags.Project.Name, "p", "", "Project to create channel in")
	flags.StringVarP(&createFlags.Description.Value, createFlags.Description.Name, "d", "", "Description of the channel")
	flags.StringVarP(&createFlags.Lifecycle.Value, createFlags.Lifecycle.Name, "l", "", "The lifecycle to use for the channel")
	flags.BoolVar(&createFlags.Default.Value, createFlags.Default.Name, false, "Set this channel as default")

	return cmd
}

func createRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		err := PromptMissing(opts)
		if err != nil {
			return err
		}
	}

	project, err := opts.Client.Projects.GetByIdentifier(opts.Project.Value)
	if err != nil {
		return err
	}

	lifecycle, err := opts.Client.Lifecycles.GetByIDOrName(opts.Lifecycle.Value)
	if err != nil {
		return err
	}

	channel := channels.NewChannel(opts.Name.Value, project.GetID())
	channel.Description = opts.Description.Value
	channel.IsDefault = opts.Default.Value
	channel.LifecycleID = lifecycle.GetID()

	createChannel, err := channels.Add(opts.Client, channel)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.Out, "\nSuccessfully created channel '%s', (%s).\n", createChannel.Name, createChannel.GetID())
	if err != nil {
		return err
	}

	link := output.Bluef("%s/app#/%s/projects/%s/deployments/channels/edit/%s", opts.Host, opts.Space.GetID(), project.Slug, createChannel.GetID())
	fmt.Fprintf(opts.Out, "View this channel on Octopus Deploy: %s\n", link)

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.Project, opts.Description, opts.Default, opts.Lifecycle)
		fmt.Fprintf(opts.Out, "%s\n", autoCmd)
	}

	return nil
}

func PromptMissing(opts *CreateOptions) error {
	err := question.AskName(opts.Ask, "", "channel", &opts.Name.Value)
	if err != nil {
		return err
	}

	var selectedProject *projects.Project
	if opts.Project.Value == "" {
		selectedProject, err = selectors.Project("Select the project in which the channel will be created", opts.Client, opts.Ask)
		if err != nil {
			return err
		}
	} else {
		selectedProject, err = selectors.FindProject(opts.Client, opts.Project.Value)
		if err != nil {
			return err
		}
	}
	opts.Project.Value = selectedProject.Name

	err = question.AskDescription(opts.Ask, "", "channel", &opts.Description.Value)
	if err != nil {
		return err
	}

	var selectedLifecycle *lifecycles.Lifecycle
	if opts.Lifecycle.Value == "" {
		var shouldInheritLifecycleFromProject bool
		err := opts.Ask(&survey.Select{
			Message: "Inherit lifecycle from project?",
			Help:    "Select 'No' to select lifecycle to use for channel",
			Options: []string{"Yes", "No"},
		}, &shouldInheritLifecycleFromProject)
		if err != nil {
			return err
		}
		if shouldInheritLifecycleFromProject {
			selectedLifecycle, err = selectors.FindLifecycle(opts.Client, selectedProject.LifecycleID)
			if err != nil {
				return err
			}
		} else {
			selectedLifecycle, err = selectors.Lifecycle("Select the lifecycle to use for the channel", opts.Client, opts.Ask)
			if err != nil {
				return err
			}
		}
	} else {
		selectedLifecycle, err = selectors.FindLifecycle(opts.Client, opts.Lifecycle.Value)
		if err != nil {
			return nil
		}
	}
	opts.Lifecycle.Value = selectedLifecycle.Name

	_, err = promptBool(opts, &opts.Default.Value, false, "Set channel as default", "If default is enabled, this will set this channel as the default channel for the project")
	if err != nil {
		return err
	}

	return nil
}

func promptBool(opts *CreateOptions, value *bool, defaultValue bool, message string, help string) (bool, error) {
	if *value != defaultValue {
		return *value, nil
	}
	err := opts.Ask(&survey.Confirm{
		Message: message,
		Help:    help,
		Default: defaultValue,
	}, value)
	return *value, err
}
