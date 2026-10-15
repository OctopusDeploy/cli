package list

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/ephemeralenvironments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
)

const (
	FlagProject = "project"

	// StatusUnknown is shown when the status of an environment could not be read for the project.
	StatusUnknown = "Unknown"
)

type ListFlags struct {
	Project *flag.Flag[string]
}

func NewListFlags() *ListFlags {
	return &ListFlags{
		Project: flag.New[string](FlagProject, false),
	}
}

type ListOptions struct {
	*ListFlags
	*cmd.Dependencies
	Command *cobra.Command
}

func NewListOptions(listFlags *ListFlags, dependencies *cmd.Dependencies, command *cobra.Command) *ListOptions {
	return &ListOptions{
		ListFlags:    listFlags,
		Dependencies: dependencies,
		Command:      command,
	}
}

// EphemeralEnvironmentViewModel is a listed ephemeral environment. Status is only set when a project is given,
// because the provisioning status of an ephemeral environment is per project.
type EphemeralEnvironmentViewModel struct {
	Id     string `json:"Id"`
	Name   string `json:"Name"`
	Status string `json:"Status,omitempty"`
}

func NewCmdList(f factory.Factory) *cobra.Command {
	listFlags := NewListFlags()

	command := &cobra.Command{
		Use:   "list",
		Short: "List ephemeral environments",
		Long:  "List ephemeral environments in Octopus Deploy. Pass a project to include the provisioning status of each environment for that project.",
		Example: heredoc.Docf(`
			%[1]s ephemeral-environment list
			%[1]s ephemeral-environment ls
			%[1]s ephemeral-environment list --project "MyProject"
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewListOptions(listFlags, cmd.NewDependencies(f, c), c)

			return ListRun(opts)
		},
	}

	flags := command.Flags()
	flags.StringVarP(&listFlags.Project.Value, listFlags.Project.Name, "p", "", "Name of the project to show the provisioning status for")

	return command
}

func ListRun(opts *ListOptions) error {
	var project *projects.Project

	if opts.Project.Value != "" {
		var err error
		project, err = projects.GetByName(opts.Client, opts.Space.ID, opts.Project.Value)
		if err != nil {
			return fmt.Errorf("failed to find project '%s': %w", opts.Project.Value, err)
		}
	}

	allEnvironments, err := ephemeralenvironments.GetAll(opts.Client, opts.Space.ID)
	if err != nil {
		return err
	}

	viewModels := make([]EphemeralEnvironmentViewModel, 0, len(allEnvironments.Items))
	for _, environment := range allEnvironments.Items {
		viewModel := EphemeralEnvironmentViewModel{Id: environment.ID, Name: environment.Name}

		if project != nil {
			viewModel.Status = getStatus(opts, environment, project)
		}

		viewModels = append(viewModels, viewModel)
	}

	header := []string{"NAME", "ID"}
	if project != nil {
		header = append(header, "STATUS")
	}

	return output.PrintArray(viewModels, opts.Command, output.Mappers[EphemeralEnvironmentViewModel]{
		Json: func(item EphemeralEnvironmentViewModel) any {
			return item
		},
		Table: output.TableDefinition[EphemeralEnvironmentViewModel]{
			Header: header,
			Row: func(item EphemeralEnvironmentViewModel) []string {
				row := []string{output.Bold(item.Name), item.Id}
				if project != nil {
					row = append(row, item.Status)
				}
				return row
			},
		},
		Basic: func(item EphemeralEnvironmentViewModel) string {
			return item.Name
		},
	})
}

// getStatus reads the provisioning status of an environment for a project. A failure for one environment
// should not stop the whole listing, so it is reported on stderr and shown as unknown.
func getStatus(opts *ListOptions, environment *ephemeralenvironments.EphemeralEnvironment, project *projects.Project) string {
	status, err := ephemeralenvironments.GetStatus(opts.Client, opts.Space.ID, environment.ID, project.GetID())
	if err != nil {
		opts.Command.PrintErrf("Could not get the status of ephemeral environment '%s' for project '%s': %v\n", environment.Name, project.GetName(), err)
		return StatusUnknown
	}

	return status.Status
}
