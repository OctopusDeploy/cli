package enable

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"
)

type EnableOptions struct {
	*cmd.Dependencies
	IdOrName string
}

func NewEnableOptions(args []string, dependencies *cmd.Dependencies) *EnableOptions {
	return &EnableOptions{
		Dependencies: dependencies,
		IdOrName:     args[0],
	}
}

func NewCmdEnable(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "enable",
		Short:   "Enable a project",
		Long:    "Enable a project in Octopus Deploy",
		Example: heredoc.Docf("$ %[1]s project enable", constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = append(args, "")
			}

			opts := NewEnableOptions(args, cmd.NewDependencies(f, c))
			return disableRun(opts)
		},
	}

	return cmd
}

func disableRun(opts *EnableOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	if opts.IdOrName == "" {
		return fmt.Errorf("project identifier is required but was not provided")
	}

	projectToEnable, err := projects.GetByIdentifier(opts.Client, opts.Client.GetSpaceID(), opts.IdOrName)
	if err != nil {
		return err
	}

	projectToEnable.IsDisabled = false
	_, err = projects.Update(opts.Client, projectToEnable)
	if err != nil {
		return err
	}

	return nil
}

func PromptMissing(opts *EnableOptions) error {
	if opts.IdOrName == "" {
		existingProjects, err := opts.Client.Projects.GetAll()
		if err != nil {
			return err
		}
		selectedProject, err := selectors.ByName(opts.Ask, existingProjects, "Select the project you wish to enable:")
		if err != nil {
			return err
		}
		opts.IdOrName = selectedProject.GetID()
	}

	return nil
}
