package snapshot_variables

import (
	"errors"
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/runbook/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/spf13/cobra"
)

const (
	FlagProject  = "project"
	FlagRunbook  = "runbook"
	FlagSnapshot = "snapshot"
)

type SnapshotVariablesFlags struct {
	Project  *flag.Flag[string]
	Runbook  *flag.Flag[string]
	Snapshot *flag.Flag[string]
}

func NewSnapshotVariablesFlags() *SnapshotVariablesFlags {
	return &SnapshotVariablesFlags{
		Project:  flag.New[string](FlagProject, false),
		Runbook:  flag.New[string](FlagRunbook, false),
		Snapshot: flag.New[string](FlagSnapshot, false),
	}
}

type SnapshotVariablesOptions struct {
	*SnapshotVariablesFlags
	*shared.RunbooksOptions
	GetAllProjectsCallback shared.GetAllProjectsCallback
	*cmd.Dependencies
}

func NewSnapshotVariablesOptions(snapshotVariablesFlags *SnapshotVariablesFlags, dependencies *cmd.Dependencies) *SnapshotVariablesOptions {
	return &SnapshotVariablesOptions{
		SnapshotVariablesFlags: snapshotVariablesFlags,
		RunbooksOptions:        shared.NewGetRunbooksOptions(dependencies),
		GetAllProjectsCallback: func() ([]*projects.Project, error) { return shared.GetAllProjects(dependencies.Client) },
		Dependencies:           dependencies,
	}
}

func NewCmdSnapshotVariables(f factory.Factory) *cobra.Command {
	snapshotVariablesFlags := NewSnapshotVariablesFlags()
	cmd := &cobra.Command{
		Use:   "snapshot-variables",
		Short: "Update the variable snapshot for a runbook snapshot",
		Long:  "Update the variable snapshot for a runbook snapshot in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s runbook snapshot snapshot-variables --project MyProject --runbook "Rebuild DB Indexes"
			$ %[1]s runbook snapshot snapshot-variables --project MyProject --runbook "Rebuild DB Indexes" --snapshot "Snapshot 40C9ENM"
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewSnapshotVariablesOptions(snapshotVariablesFlags, cmd.NewDependencies(f, c))
			return snapshotVariablesRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&snapshotVariablesFlags.Project.Value, snapshotVariablesFlags.Project.Name, "p", "", "Name or ID of the project where the runbook is")
	flags.StringVarP(&snapshotVariablesFlags.Runbook.Value, snapshotVariablesFlags.Runbook.Name, "r", "", "Name or ID of the runbook")
	flags.StringVar(&snapshotVariablesFlags.Snapshot.Value, snapshotVariablesFlags.Snapshot.Name, "", "Name or ID of the snapshot to update variables for (defaults to the published snapshot)")

	return cmd
}

func snapshotVariablesRun(opts *SnapshotVariablesOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	if opts.Project.Value == "" {
		return errors.New("project must be specified")
	}
	if opts.Runbook.Value == "" {
		return errors.New("runbook must be specified")
	}

	project, err := selectors.FindProject(opts.Client, opts.Project.Value)
	if err != nil {
		return err
	}
	if project == nil {
		return errors.New("unable to find project")
	}

	if shared.AreRunbooksInGit(project) {
		return errors.New("updating variable snapshots is not supported for runbooks stored in Git")
	}

	runbook, err := selectors.FindRunbook(opts.Client, project, opts.Runbook.Value)
	if err != nil {
		return err
	}
	if runbook == nil {
		return errors.New("unable to find runbook")
	}

	snapshotID, snapshotName, defaultedToPublished, err := resolveSnapshot(opts, runbook)
	if err != nil {
		return err
	}

	if defaultedToPublished {
		fmt.Fprintf(opts.Out, "Updating variables for published snapshot '%s' (%s)\n", snapshotName, output.Dim(snapshotID))
	}

	if _, err := runbooks.UpdateSnapshotVariables(opts.Client, opts.Space.GetID(), snapshotID); err != nil {
		return err
	}

	fmt.Fprintf(opts.Out, "Successfully updated variable snapshot '%s' (%s) for runbook '%s'\n", snapshotName, output.Dim(snapshotID), runbook.Name)
	link := output.Bluef("%s/app#/%s/projects/%s/operations/runbooks/%s/snapshots/%s", opts.Host, opts.Space.GetID(), project.GetID(), runbook.GetID(), snapshotID)
	fmt.Fprintf(opts.Out, "View this snapshot on Octopus Deploy: %s\n", link)

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.GetSpaceNameOrEmpty(), opts.Project, opts.Runbook, opts.Snapshot)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	return nil
}

func resolveSnapshot(opts *SnapshotVariablesOptions, runbook *runbooks.Runbook) (id string, name string, defaultedToPublished bool, err error) {
	if opts.Snapshot.Value != "" {
		snapshot, err := runbooks.GetSnapshot(opts.Client, opts.Space.GetID(), runbook.ProjectID, opts.Snapshot.Value)
		if err != nil {
			return "", "", false, err
		}
		if snapshot == nil {
			return "", "", false, errors.New("unable to find snapshot")
		}
		return snapshot.GetID(), snapshot.Name, false, nil
	}

	if runbook.PublishedRunbookSnapshotID == "" {
		return "", "", false, errors.New("runbook has no published snapshot; specify a snapshot with --snapshot")
	}

	snapshot, err := runbooks.GetSnapshot(opts.Client, opts.Space.GetID(), runbook.ProjectID, runbook.PublishedRunbookSnapshotID)
	if err != nil {
		return "", "", false, err
	}
	if snapshot == nil {
		return "", "", false, fmt.Errorf("unable to find published snapshot '%s'", runbook.PublishedRunbookSnapshotID)
	}
	return snapshot.GetID(), snapshot.Name, true, nil
}

func PromptMissing(opts *SnapshotVariablesOptions) error {
	project, err := getProject(opts)
	if err != nil {
		return err
	}
	opts.Project.Value = project.GetName()

	if shared.AreRunbooksInGit(project) {
		return errors.New("updating variable snapshots is not supported for runbooks stored in Git")
	}

	selectedRunbook, err := getRunbook(opts, project)
	if err != nil {
		return err
	}
	opts.Runbook.Value = selectedRunbook.Name

	return nil
}

func getProject(opts *SnapshotVariablesOptions) (*projects.Project, error) {
	var project *projects.Project
	var err error
	if opts.Project.Value == "" {
		project, err = selectors.Select(opts.Ask, "Select the project containing the runbook:", opts.GetAllProjectsCallback, func(p *projects.Project) string { return p.GetName() })
	} else {
		project, err = opts.GetProjectCallback(opts.Project.Value)
	}

	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, errors.New("unable to find project")
	}

	return project, nil
}

func getRunbook(opts *SnapshotVariablesOptions, project *projects.Project) (*runbooks.Runbook, error) {
	var runbook *runbooks.Runbook
	var err error
	if opts.Runbook.Value == "" {
		runbook, err = selectors.Select(opts.Ask, "Select the runbook:", func() ([]*runbooks.Runbook, error) { return opts.GetDbRunbooksCallback(project.GetID()) }, func(r *runbooks.Runbook) string { return r.Name })
	} else {
		runbook, err = opts.GetDbRunbookCallback(project.GetID(), opts.Runbook.Value)
	}

	if err != nil {
		return nil, err
	}
	if runbook == nil {
		return nil, errors.New("unable to find runbook")
	}

	return runbook, nil
}
