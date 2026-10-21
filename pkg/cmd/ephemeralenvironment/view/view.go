package view

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/ephemeralenvironments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/util"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
)

const (
	FlagName    = "name"
	FlagProject = "project"
)

type ViewFlags struct {
	Name    *flag.Flag[string]
	Project *flag.Flag[string]
}

func NewViewFlags() *ViewFlags {
	return &ViewFlags{
		Name:    flag.New[string](FlagName, false),
		Project: flag.New[string](FlagProject, false),
	}
}

type ViewOptions struct {
	*ViewFlags
	*cmd.Dependencies
	Command                       *cobra.Command
	GetConfiguredProjectsCallback func() ([]*projects.Project, error)
	GetAllEphemeralEnvironments   func() ([]*ephemeralenvironments.EphemeralEnvironment, error)
}

func NewViewOptions(viewFlags *ViewFlags, dependencies *cmd.Dependencies, command *cobra.Command) *ViewOptions {
	return &ViewOptions{
		ViewFlags:    viewFlags,
		Dependencies: dependencies,
		Command:      command,
		GetConfiguredProjectsCallback: func() ([]*projects.Project, error) {
			return util.GetConfiguredProjects(dependencies.Client)
		},
		GetAllEphemeralEnvironments: func() ([]*ephemeralenvironments.EphemeralEnvironment, error) {
			allEnvironments, err := ephemeralenvironments.GetAll(dependencies.Client, dependencies.Space.ID)
			if err != nil {
				return nil, err
			}

			if len(allEnvironments.Items) == 0 {
				return nil, fmt.Errorf("no ephemeral environments found")
			}

			return allEnvironments.Items, nil
		},
	}
}

// ViewResult is what the view command outputs: an ephemeral environment and its provisioning status for a project.
type ViewResult struct {
	Id          string `json:"Id"`
	Name        string `json:"Name"`
	ProjectId   string `json:"ProjectId"`
	ProjectName string `json:"ProjectName"`
	Status      string `json:"Status"`
}

func NewCmdView(f factory.Factory) *cobra.Command {
	viewFlags := NewViewFlags()

	command := &cobra.Command{
		Use:   "view",
		Short: "View an ephemeral environment",
		Long:  "View the provisioning status of an ephemeral environment for a project in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s ephemeral-environment view --name PR-1234 --project "MyProject"
			%[1]s ephemeral-environment view --name PR-1234 --project "MyProject" --output-format json
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			deps, err := cmd.NewDependencies(f, c)
			if err != nil {
				return err
			}
			opts := NewViewOptions(viewFlags, deps, c)

			return ViewRun(opts)
		},
	}

	flags := command.Flags()
	flags.StringVarP(&viewFlags.Name.Value, viewFlags.Name.Name, "n", "", "Name of the environment")
	flags.StringVarP(&viewFlags.Project.Value, viewFlags.Project.Name, "p", "", "Name of the project")

	return command
}

func ViewRun(opts *ViewOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	if opts.Name.Value == "" {
		return fmt.Errorf("environment name is required")
	}

	if opts.Project.Value == "" {
		return fmt.Errorf("project name is required")
	}

	project, err := projects.GetByName(opts.Client, opts.Space.ID, opts.Project.Value)
	if err != nil {
		return fmt.Errorf("failed to find project '%s': %w", opts.Project.Value, err)
	}

	environment, err := util.GetByName(opts.Client, opts.Name.Value, opts.Space.ID)
	if err != nil {
		return err
	}

	status, err := ephemeralenvironments.GetStatus(opts.Client, opts.Space.ID, environment.ID, project.GetID())
	if err != nil {
		return fmt.Errorf("failed to get the status of ephemeral environment '%s' for project '%s': %w", environment.Name, project.GetName(), err)
	}

	result := &ViewResult{
		Id:          environment.ID,
		Name:        environment.Name,
		ProjectId:   project.GetID(),
		ProjectName: project.GetName(),
		Status:      status.Status,
	}

	err = output.PrintResource(result, opts.Command, output.Mappers[*ViewResult]{
		Json: func(r *ViewResult) any { return r },
		Table: output.TableDefinition[*ViewResult]{
			Header: []string{"NAME", "ID", "PROJECT", "STATUS"},
			Row: func(r *ViewResult) []string {
				return []string{output.Bold(r.Name), r.Id, r.ProjectName, r.Status}
			},
		},
		Basic: func(r *ViewResult) string {
			return fmt.Sprintf("%s (%s) for project %s: %s", r.Name, r.Id, r.ProjectName, r.Status)
		},
	})
	if err != nil {
		return err
	}

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.GetSpaceNameOrEmpty(), opts.Name, opts.Project)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	return nil
}

func PromptMissing(opts *ViewOptions) error {
	if opts.Project.Value == "" {
		fmt.Fprintf(opts.Out, "  Choose from projects configured with an ephemeral environment channel.\n")
		project, err := selectors.Select(opts.Ask, "Select a project:", opts.GetConfiguredProjectsCallback, func(project *projects.Project) string { return project.GetName() })
		if err != nil {
			return err
		}
		opts.Project.Value = project.GetName()
	}

	if opts.Name.Value == "" {
		environment, err := selectors.Select(opts.Ask, "Please select the name of the environment you wish to view:", opts.GetAllEphemeralEnvironments, func(environment *ephemeralenvironments.EphemeralEnvironment) string { return environment.Name })
		if err != nil {
			return err
		}
		opts.Name.Value = environment.Name
	}

	return nil
}
