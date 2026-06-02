package delete

import (
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
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
	Client   *client.Client
	Ask      question.Asker
	Out      *cobra.Command
	NoPrompt bool
	IdOrName string
	*DeleteFlags
}

func NewCmdDelete(f factory.Factory) *cobra.Command {
	deleteFlags := NewDeleteFlags()
	cmd := &cobra.Command{
		Use:     "delete {<name> | <id> | <slug>}",
		Short:   "Delete a channel",
		Long:    "Delete a channel in Octopus Deploy",
		Aliases: []string{"del", "rm", "remove"},
		Example: heredoc.Docf(`
			%[1]s channel delete "Hotfix" --project myProject
			%[1]s channel rm Channels-123 --project myProject -y
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			if deleteFlags.Project.Value == "" {
				return errors.New("--project is required")
			}

			c, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}

			idOrName := ""
			if len(args) > 0 {
				idOrName = args[0]
			}

			opts := &DeleteOptions{
				Client:      c,
				Ask:         f.Ask,
				Out:         cmd,
				NoPrompt:    !f.IsPromptEnabled(),
				IdOrName:    idOrName,
				DeleteFlags: deleteFlags,
			}

			return deleteRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&deleteFlags.Project.Value, deleteFlags.Project.Name, "p", "", "Name or ID of the project the channel belongs to")
	question.RegisterConfirmDeletionFlag(cmd, &deleteFlags.Confirm.Value, "channel")

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	project, err := selectors.FindProject(opts.Client, opts.Project.Value)
	if err != nil {
		return err
	}

	if !opts.NoPrompt {
		if err := promptMissing(opts, project); err != nil {
			return err
		}
	}

	if opts.IdOrName == "" {
		return errors.New("channel name or ID must be specified")
	}

	itemToDelete, err := resolveChannel(opts.Client, project, opts.IdOrName)
	if err != nil {
		return err
	}

	// In interactive mode, warn for version-controlled (CaC) projects since deleting a
	// channel can break OCL deployments referencing it.
	if !opts.NoPrompt && project.IsVersionControlled {
		opts.Out.Printf("%s This project is version-controlled (Config-as-Code). Deleting this channel can break OCL deployments that reference it.\n",
			output.Yellow("Warning:"))
	}

	doDelete := func() error {
		return opts.Client.Channels.DeleteByID(itemToDelete.GetID())
	}

	if opts.Confirm.Value {
		return doDelete()
	}
	return question.DeleteWithConfirmation(opts.Ask, "channel", itemToDelete.Name, itemToDelete.GetID(), doDelete)
}

func promptMissing(opts *DeleteOptions, project *projects.Project) error {
	if opts.IdOrName != "" {
		return nil
	}
	existing, err := opts.Client.Projects.GetChannels(project)
	if err != nil {
		return err
	}
	if len(existing) == 0 {
		return fmt.Errorf("project %s has no channels", project.Name)
	}
	var chosenName string
	if err := opts.Ask(&survey.Select{
		Message: "Select the channel you wish to delete:",
		Options: channelNames(existing),
	}, &chosenName); err != nil {
		return err
	}
	for _, c := range existing {
		if c.Name == chosenName {
			opts.IdOrName = c.GetID()
			break
		}
	}
	return nil
}

func channelNames(cs []*channels.Channel) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Name)
	}
	return out
}

func resolveChannel(c *client.Client, project *projects.Project, idOrName string) (*channels.Channel, error) {
	if ch, err := c.Channels.GetByID(idOrName); err == nil && ch != nil {
		// Verify the channel actually belongs to the named project so we don't accidentally
		// delete a channel from another project that happens to share an ID prefix.
		if ch.ProjectID == project.GetID() {
			return ch, nil
		}
	}
	return selectors.FindChannel(c, project, idOrName)
}
