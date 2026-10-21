package delete

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/ephemeralenvironments"
	"github.com/spf13/cobra"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/util"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
)

const (
	FlagName = "name"
)

type DeleteFlags struct {
	Name *flag.Flag[string]
	*question.ConfirmFlags
}

func NewDeleteFlags() *DeleteFlags {
	return &DeleteFlags{
		Name:         flag.New[string](FlagName, false),
		ConfirmFlags: question.NewConfirmFlags(),
	}
}

type DeleteOptions struct {
	*DeleteFlags
	*cmd.Dependencies
	GetAllEphemeralEnvironments func() ([]*ephemeralenvironments.EphemeralEnvironment, error)
}

func NewDeleteOptions(deleteFlags *DeleteFlags, dependencies *cmd.Dependencies) *DeleteOptions {
	return &DeleteOptions{
		DeleteFlags:  deleteFlags,
		Dependencies: dependencies,
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

func NewCmdDelete(f factory.Factory) *cobra.Command {
	deleteFlags := NewDeleteFlags()

	command := &cobra.Command{
		Use:     "delete",
		Short:   "Delete an ephemeral environment",
		Long:    "Delete an ephemeral environment in Octopus Deploy. This removes the environment without running its deprovisioning runbook.",
		Aliases: []string{"del", "rm", "remove"},
		Example: heredoc.Docf(`
			%[1]s ephemeral-environment delete --name PR-1234
			%[1]s ephemeral-environment delete --name PR-1234 --confirm
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			deps, err := cmd.NewDependencies(f, c)
			if err != nil {
				return err
			}
			opts := NewDeleteOptions(deleteFlags, deps)

			return DeleteRun(opts)
		},
	}

	flags := command.Flags()
	flags.StringVarP(&deleteFlags.Name.Value, deleteFlags.Name.Name, "n", "", "Name of the environment")
	question.RegisterConfirmDeletionFlag(command, &deleteFlags.Confirm.Value, "ephemeral environment")

	return command
}

func DeleteRun(opts *DeleteOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	if opts.Name.Value == "" {
		return fmt.Errorf("environment name is required")
	}

	environment, err := util.GetByName(opts.Client, opts.Name.Value, opts.Space.ID)
	if err != nil {
		return err
	}

	doDelete := func() error {
		return ephemeralenvironments.Delete(opts.Client, opts.Space.ID, environment.ID)
	}

	if opts.Confirm.Value {
		if err := doDelete(); err != nil {
			return err
		}
		_, err = fmt.Fprintf(opts.Out, "Deleted ephemeral environment '%s' with id '%s'.\n", environment.Name, environment.ID)
		return err
	}

	return question.DeleteWithConfirmation(opts.Ask, "ephemeral environment", environment.Name, environment.ID, doDelete)
}

func PromptMissing(opts *DeleteOptions) error {
	if opts.Name.Value != "" {
		return nil
	}

	environment, err := selectors.Select(opts.Ask, "Please select the name of the environment you wish to delete:", opts.GetAllEphemeralEnvironments, func(environment *ephemeralenvironments.EphemeralEnvironment) string { return environment.Name })
	if err != nil {
		return err
	}

	opts.Name.Value = environment.Name

	return nil
}
