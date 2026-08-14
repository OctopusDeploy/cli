package delete

import (
	"errors"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/channel/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
)

const (
	FlagProject = "project"
)

type DeleteFlags struct {
	Project *flag.Flag[string]
	*question.ConfirmFlags
}

func NewDeleteFlags() *DeleteFlags {
	return &DeleteFlags{
		Project:      flag.New[string](FlagProject, false),
		ConfirmFlags: question.NewConfirmFlags(),
	}
}

type DeleteOptions struct {
	*DeleteFlags
	*cmd.Dependencies
	IdOrName string
}

func NewDeleteOptions(deleteFlags *DeleteFlags, dependencies *cmd.Dependencies, idOrName string) *DeleteOptions {
	return &DeleteOptions{
		DeleteFlags:  deleteFlags,
		Dependencies: dependencies,
		IdOrName:     idOrName,
	}
}

func NewCmdDelete(f factory.Factory) *cobra.Command {
	deleteFlags := NewDeleteFlags()
	command := &cobra.Command{
		Use:     "delete {<name> | <id>}",
		Short:   "Delete a channel",
		Long:    "Delete a channel in Octopus Deploy",
		Aliases: []string{"del", "rm", "remove"},
		Example: heredoc.Docf(`
			%[1]s channel delete "Hotfix" --project myProject
			%[1]s channel rm Channels-123 --project myProject -y
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			idOrName := ""
			if len(args) > 0 {
				idOrName = args[0]
			}

			opts := NewDeleteOptions(deleteFlags, cmd.NewDependencies(f, c), idOrName)

			return deleteRun(opts)
		},
	}

	flags := command.Flags()
	flags.StringVarP(&deleteFlags.Project.Value, deleteFlags.Project.Name, "p", "", "Name or ID of the project the channel belongs to")
	question.RegisterConfirmDeletionFlag(command, &deleteFlags.Confirm.Value, "channel")

	return command
}

func deleteRun(opts *DeleteOptions) error {
	// in automation mode we validate the flags up front, before making any API calls
	if opts.NoPrompt {
		if opts.Project.Value == "" {
			return errors.New("project must be specified")
		}
		if opts.IdOrName == "" {
			return errors.New("channel name or ID must be specified")
		}
	}

	project, err := selectors.ResolveProject(opts.Client, opts.Ask, !opts.NoPrompt,
		"Select the project containing the channel you wish to delete", opts.Project.Value)
	if err != nil {
		return err
	}

	itemToDelete, err := shared.ResolveChannel(opts.Client, opts.Ask, !opts.NoPrompt,
		"Select the channel you wish to delete:", project, opts.IdOrName)
	if err != nil {
		return err
	}

	doDelete := func() error {
		return opts.Client.Channels.DeleteByID(itemToDelete.GetID())
	}

	if opts.Confirm.Value {
		return doDelete()
	}
	return question.DeleteWithConfirmation(opts.Ask, "channel", itemToDelete.Name, itemToDelete.GetID(), doDelete)
}
