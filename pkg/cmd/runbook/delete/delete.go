package delete

import (
	"errors"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/runbook/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/spf13/cobra"
)

const resourceDescription = "runbook"

const (
	FlagProject = "project"
	FlagRunbook = "runbook"
)

type DeleteFlags struct {
	Project          *flag.Flag[string]
	Runbook          *flag.Flag[string]
	SkipConfirmation bool
}

func NewDeleteFlags() *DeleteFlags {
	return &DeleteFlags{
		Project: flag.New[string](FlagProject, false),
		Runbook: flag.New[string](FlagRunbook, false),
	}
}

type DeleteOptions struct {
	*DeleteFlags
	*cmd.Dependencies
	*shared.RunbooksOptions
	GetAllProjectsCallback shared.GetAllProjectsCallback
}

func NewDeleteOptions(dependencies *cmd.Dependencies, flags *DeleteFlags) *DeleteOptions {
	return &DeleteOptions{
		DeleteFlags:            flags,
		Dependencies:           dependencies,
		RunbooksOptions:        shared.NewGetRunbooksOptions(dependencies),
		GetAllProjectsCallback: func() ([]*projects.Project, error) { return shared.GetAllProjects(dependencies.Client) },
	}
}

func NewCmdDelete(f factory.Factory) *cobra.Command {
	deleteFlags := NewDeleteFlags()
	cmd := &cobra.Command{
		Use:     "delete {<name> | <id>}",
		Short:   "Delete a runbook",
		Long:    "Delete a runbook in Octopus Deploy",
		Aliases: []string{"del", "rm", "remove"},
		Example: heredoc.Docf(`
			$ %[1]s runbook delete
			$ %[1]s runbook rm
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			deps := cmd.NewDependencies(f, c)

			opts := NewDeleteOptions(deps, deleteFlags)
			if deleteFlags.Runbook.Value == "" && len(args) > 0 {
				deleteFlags.Runbook.Value = args[0]
			}

			return deleteRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&deleteFlags.Project.Value, deleteFlags.Project.Name, "p", "", "Name or ID of the project to delete a runbook from")
	flags.StringVarP(&deleteFlags.Runbook.Value, deleteFlags.Runbook.Name, "r", "", "Name or ID of the runbook to delete")
	question.RegisterConfirmDeletionFlag(cmd, &deleteFlags.SkipConfirmation, resourceDescription)

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	project, err := getProject(opts)
	if err != nil {
		return err
	}

	runbook, err := getRunbook(opts, project)
	if err != nil {
		return err
	}

	if opts.SkipConfirmation {
		return delete(opts.Client, runbook)
	} else {
		return question.DeleteWithConfirmation(opts.Ask, resourceDescription, runbook.Name, runbook.GetID(), func() error {
			return delete(opts.Client, runbook)
		})
	}
}

func getRunbook(opts *DeleteOptions, project *projects.Project) (*runbooks.Runbook, error) {
	var runbook *runbooks.Runbook
	var err error
	if opts.Runbook.Value == "" {
		runbook, err = selectors.Select(opts.Ask, "Select the runbook you wish to delete:", func() ([]*runbooks.Runbook, error) { return opts.GetRunbooksCallback(project.GetID()) }, func(runbook *runbooks.Runbook) string { return runbook.Name })
	} else {
		runbook, err = selectors.FindRunbook(opts.Client, project, opts.Runbook.Value)
	}

	if runbook == nil {
		return nil, errors.New("unable to find runbook")
	}

	return runbook, err
}

func getProject(opts *DeleteOptions) (*projects.Project, error) {
	var project *projects.Project
	var err error
	if opts.Project.Value == "" {
		project, err = selectors.Select(opts.Ask, "Select the project containing the runbook you wish to delete:", opts.GetAllProjectsCallback, func(project *projects.Project) string { return project.GetName() })
	} else {
		project, err = selectors.FindProject(opts.Client, opts.Project.Value)
	}

	if project == nil {
		return nil, errors.New("unable to find project")
	}

	return project, err
}

func delete(client *client.Client, itemToDelete *runbooks.Runbook) error {
	return client.Runbooks.DeleteByID(itemToDelete.GetID())
}
