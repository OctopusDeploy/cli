package disable

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

type DisableOptions struct {
	*cmd.Dependencies
	IdOrName string
}

func NewDisableOptions(args []string, dependencies *cmd.Dependencies) *DisableOptions {
	return &DisableOptions{
		Dependencies: dependencies,
		IdOrName:     args[0],
	}
}

func NewCmdDisable(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "disable",
		Short:   "Disable a project",
		Long:    "Disable a project in Octopus Deploy",
		Example: heredoc.Docf("$ %[1]s project disable", constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = append(args, "")
			}

			opts := NewDisableOptions(args, cmd.NewDependencies(f, c))
			return disableRun(opts)
		},
	}

	return cmd
}

func disableRun(opts *DisableOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	if opts.IdOrName == "" {
		return fmt.Errorf("project identifier is required but was not provided")
	}

	projectToDisable, err := projects.GetByIdentifier(opts.Client, opts.Client.GetSpaceID(), opts.IdOrName)
	if err != nil {
		return nil
	}

	projectToDisable.IsDisabled = true
	_, err = projects.Update(opts.Client, projectToDisable)
	if err != nil {
		return err
	}

	return nil
}

func PromptMissing(opts *DisableOptions) error {
	if opts.IdOrName == "" {
		existingProjects, err := opts.Client.Projects.GetAll()
		if err != nil {
			return err
		}
		itemToDelete, err := selectors.ByName(opts.Ask, existingProjects, "Select the project you wish to disable:")
		if err != nil {
			return err
		}
		opts.IdOrName = itemToDelete.GetID()
	}

	return nil
}
