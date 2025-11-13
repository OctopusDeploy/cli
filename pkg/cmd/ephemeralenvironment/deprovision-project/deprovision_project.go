package deprovision_project

import (
	"fmt"
	"slices"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/util"
	"github.com/OctopusDeploy/cli/pkg/cmd/runbook/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
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

type DeprovisionProjectFlags struct {
	Name    *flag.Flag[string]
	Project *flag.Flag[string]
}

func NewDeprovisionProjectFlags() *DeprovisionProjectFlags {
	return &DeprovisionProjectFlags{
		Name:    flag.New[string](FlagName, false),
		Project: flag.New[string](FlagProject, false),
	}
}

type DeprovisionProjectOptions struct {
	*DeprovisionProjectFlags
	*cmd.Dependencies
	Cmd                           *cobra.Command
	GetConfiguredProjectsCallback func() ([]*projects.Project, error)
	GetAllEphemeralEnvironments   func() ([]*ephemeralenvironments.EphemeralEnvironment, error)
}

func NewDeprovisionProjectOptions(deprovisionFlags *DeprovisionProjectFlags, dependencies *cmd.Dependencies, command *cobra.Command) *DeprovisionProjectOptions {
	return &DeprovisionProjectOptions{
		DeprovisionProjectFlags: deprovisionFlags,
		Dependencies:            dependencies,
		Cmd:                     command,
		GetConfiguredProjectsCallback: func() ([]*projects.Project, error) {
			return getConfiguredProjects(dependencies)
		},
		GetAllEphemeralEnvironments: func() ([]*ephemeralenvironments.EphemeralEnvironment, error) {
			return getEphemeralEnvironments(dependencies)
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
			return channel.Type == channels.ChannelTypeEphemeral
		}) {
			filteredProjects = append(filteredProjects, project)
		}
	}

	if len(filteredProjects) == 0 {
		return nil, fmt.Errorf("no configured projects - configure a project with an ephemeral environment channel before creating an ephemeral environment")
	}

	return filteredProjects, nil
}

func getEphemeralEnvironments(dependencies *cmd.Dependencies) ([]*ephemeralenvironments.EphemeralEnvironment, error) {
	allEnvironments, err := ephemeralenvironments.GetAll(dependencies.Client, dependencies.Space.ID)
	if err != nil {
		return nil, err
	}

	if len(allEnvironments.Items) == 0 {
		return nil, fmt.Errorf("no ephemeral environments found")
	}

	return allEnvironments.Items, nil
}

func NewCmdDeprovisionProject(f factory.Factory) *cobra.Command {
	createFlags := NewDeprovisionProjectFlags()

	cmd := &cobra.Command{
		Use:     "deprovision-project",
		Short:   "Deprovision an ephemeral environment for a project",
		Long:    "Deprovision an ephemeral environment in Octopus Deploy for a specific project",
		Example: heredoc.Docf("$ %s ephemeral-environment deprovision-project", constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewDeprovisionProjectOptions(createFlags, cmd.NewDependencies(f, c), c)

			return DeprovisionEphemeralEnvironmentProject(c, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "Name of the environment")
	flags.StringVarP(&createFlags.Project.Value, createFlags.Project.Name, "p", "", "Name of the project")

	return cmd
}

func DeprovisionEphemeralEnvironmentProject(cmd *cobra.Command, opts *DeprovisionProjectOptions) error {
	if !opts.NoPrompt {
		err := PromptMissing(opts)
		if err != nil {
			return err
		}
	}

	if opts.Name.Value == "" {
		return fmt.Errorf("environment name is required")
	}

	if opts.Project.Value == "" {
		return fmt.Errorf("project name is required")
	}

	projectResource, err := projects.GetByName(opts.Client, opts.Space.ID, opts.Project.Value)
	if err != nil {
		return fmt.Errorf("failed to find project '%s': %w", opts.Project.Value, err)
	}

	environmentResource, err := util.GetByName(opts.Client, opts.Name.Value, opts.Space.ID)
	if err != nil {
		return fmt.Errorf("failed to retrieve ephemeral environments: %w", err)
	}

	if environmentResource == nil {
		return fmt.Errorf("no ephemeral environment found with name '%s'", opts.Name.Value)
	}

	environmentId := environmentResource.ID

	projectId := projectResource.GetID()

	deprovisionedEnv, err := ephemeralenvironments.DeprovisionForProject(opts.Client, opts.Space.ID, environmentId, projectId)
	if err != nil {
		return err
	}

	runs := []ephemeralenvironments.DeprovisioningRunbookRun{}

	if deprovisionedEnv.DeprovisioningRun.RunbookRunID != "" {
		runs = append(runs, deprovisionedEnv.DeprovisioningRun)
	}

	message := fmt.Sprintf("Deprovisioning ephemeral environment '%s' with id '%s' for project '%s'...\n", opts.Name.Value, environmentId, opts.Project.Value)

	util.OutputDeprovisionResult(message, cmd, runs)

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.Project)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	return nil
}

func PromptMissing(opts *DeprovisionProjectOptions) error {
	if opts.Project.Value == "" {
		opts.Cmd.Printf("  Choose from projects configured with an ephemeral environment channel.\n")
		project, err := selectors.Select(opts.Ask, "Select a project:", opts.GetConfiguredProjectsCallback, func(project *projects.Project) string { return project.GetName() })
		if err != nil {
			return err
		}
		opts.Project.Value = project.GetName()
	}

	if opts.Name.Value == "" {
		environment, err := selectors.Select(opts.Ask, "Please select the name of the environment you wish to deprovision:", opts.GetAllEphemeralEnvironments, func(env *ephemeralenvironments.EphemeralEnvironment) string { return env.Name })
		if err != nil {
			return err
		}
		opts.Name.Value = environment.Name
	}
	return nil
}
