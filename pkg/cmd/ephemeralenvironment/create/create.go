package create

import (
	"fmt"
	"slices"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/runbook/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/ephemeralenvironments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"
)

const (
	FlagName    = "name"
	FlagProject = "project"
)

type CreateFlags struct {
	Name    *flag.Flag[string]
	Project *flag.Flag[string]
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:    flag.New[string](FlagName, false),
		Project: flag.New[string](FlagProject, false),
	}
}

type CreateOptions struct {
	*CreateFlags
	*cmd.Dependencies
	GetConfiguredProjectsCallback func() ([]*projects.Project, error)
}

func NewCreateOptions(createFlags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:  createFlags,
		Dependencies: dependencies,
		GetConfiguredProjectsCallback: func() ([]*projects.Project, error) {
			return getConfiguredProjects(dependencies)
		},
	}
}

func getConfiguredProjects(dependencies *cmd.Dependencies) ([]*projects.Project, error) {
	allProjects, err := shared.GetAllProjects(dependencies.Client)
	if err != nil {
		return nil, err
	}

	var filteredProjects []*projects.Project

	for _, project := range allProjects {
		projectChannels, err := dependencies.Client.Projects.GetChannels(project)
		if err != nil {
			return nil, fmt.Errorf("failed to get channels for project '%s': %w", project.GetName(), err)
		}

		if slices.ContainsFunc(projectChannels, func(channel *channels.Channel) bool {
			return channel.Type == "EphemeralEnvironment"
		}) {
			filteredProjects = append(filteredProjects, project)
		}
	}

	return filteredProjects, nil
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create an ephemeral environment",
		Long:    "Create an ephemeral environment in Octopus Deploy",
		Example: heredoc.Docf("$ %s ephemeral-environment create", constants.ExecutableName),
		Aliases: []string{"new"},
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "Name of the environment")
	flags.StringVarP(&createFlags.Project.Value, createFlags.Project.Name, "p", "", "Name of the project")

	return cmd
}

func createRun(opts *CreateOptions) error {

	if !opts.NoPrompt {
		err := PromptMissing(opts)
		if err != nil {
			return err
		}
	}

	projectResource, err := projects.GetByName(opts.Client, opts.Space.ID, opts.Project.Value)
	if err != nil {
		return fmt.Errorf("failed to find project '%s': %w", opts.Project.Value, err)
	}
	projectId := projectResource.GetID()

	createEnv, err := ephemeralenvironments.Add(opts.Client, opts.Space.ID, projectId, opts.Name.Value)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.Out, "\nSuccessfully created ephemeral environment '%s' with id '%s'.\n", opts.Name.Value, createEnv.Id)
	if err != nil {
		return err
	}

	link := output.Bluef("%s/app#/%s/projects/%s/ephemeral-environments", opts.Host, opts.Space.GetID(), projectId)
	fmt.Fprintf(opts.Out, "View this ephemeral environment for project `%s` on Octopus Deploy: %s\n", opts.Project.Value, link)

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.Project)
		fmt.Fprintf(opts.Out, "%s\n", autoCmd)
	}

	return nil
}

func PromptMissing(opts *CreateOptions) error {

	err := question.AskName(opts.Ask, "", "ephemeral environment", &opts.Name.Value)
	if err != nil {
		return err
	}

	if opts.Project.Value == "" {
		project, err := selectors.Select(opts.Ask, "Select an ephemeral environments configured project to associate with the environment:", opts.GetConfiguredProjectsCallback, func(project *projects.Project) string { return project.GetName() })
		if err != nil {
			return err
		}
		opts.Project.Value = project.GetName()
	}

	return nil
}
