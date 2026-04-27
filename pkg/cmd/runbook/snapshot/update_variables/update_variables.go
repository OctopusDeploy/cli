package update_variables

import (
	"errors"
	"fmt"
	"io"
	"net/http"

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

type UpdateVariablesFlags struct {
	Project  *flag.Flag[string]
	Runbook  *flag.Flag[string]
	Snapshot *flag.Flag[string]
}

func NewUpdateVariablesFlags() *UpdateVariablesFlags {
	return &UpdateVariablesFlags{
		Project:  flag.New[string](FlagProject, false),
		Runbook:  flag.New[string](FlagRunbook, false),
		Snapshot: flag.New[string](FlagSnapshot, false),
	}
}

type UpdateVariablesOptions struct {
	*UpdateVariablesFlags
	*shared.RunbooksOptions
	GetAllProjectsCallback shared.GetAllProjectsCallback
	*cmd.Dependencies
}

func NewUpdateVariablesOptions(updateVariablesFlags *UpdateVariablesFlags, dependencies *cmd.Dependencies) *UpdateVariablesOptions {
	return &UpdateVariablesOptions{
		UpdateVariablesFlags:   updateVariablesFlags,
		RunbooksOptions:        shared.NewGetRunbooksOptions(dependencies),
		GetAllProjectsCallback: func() ([]*projects.Project, error) { return shared.GetAllProjects(dependencies.Client) },
		Dependencies:           dependencies,
	}
}

func NewCmdUpdateVariables(f factory.Factory) *cobra.Command {
	updateVariablesFlags := NewUpdateVariablesFlags()
	cmd := &cobra.Command{
		Use:   "update-variables",
		Short: "Update the variable snapshot for a runbook snapshot",
		Long:  "Update the variable snapshot for a runbook snapshot in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s runbook snapshot update-variables --project MyProject --runbook "Rebuild DB Indexes"
			$ %[1]s runbook snapshot update-variables --project MyProject --runbook "Rebuild DB Indexes" --snapshot "Snapshot 40C9ENM"
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewUpdateVariablesOptions(updateVariablesFlags, cmd.NewDependencies(f, c))
			return updateVariablesRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&updateVariablesFlags.Project.Value, updateVariablesFlags.Project.Name, "p", "", "Name or ID of the project where the runbook is")
	flags.StringVarP(&updateVariablesFlags.Runbook.Value, updateVariablesFlags.Runbook.Name, "r", "", "Name or ID of the runbook")
	flags.StringVar(&updateVariablesFlags.Snapshot.Value, updateVariablesFlags.Snapshot.Name, "", "Name or ID of the snapshot to update variables for (defaults to the published snapshot)")

	return cmd
}

func updateVariablesRun(opts *UpdateVariablesOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
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

	snapshotID, snapshotName, err := resolveSnapshot(opts, runbook)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/api/%s/runbookSnapshots/%s/snapshot-variables", opts.Space.GetID(), snapshotID)
	req, err := http.NewRequest(http.MethodPost, path, nil)
	if err != nil {
		return err
	}

	resp, err := opts.Client.HttpSession().DoRawRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(resp.Body)
 		if readErr != nil {
 			return fmt.Errorf("failed to update variable snapshot (HTTP %d) and failed to read response body: %w", resp.StatusCode, readErr)
 		}
		return fmt.Errorf("failed to update variable snapshot (HTTP %d): %s", resp.StatusCode, string(body))
	}

	fmt.Fprintf(opts.Out, "Successfully updated variable snapshot for '%s'\n", snapshotName)
	link := output.Bluef("%s/app#/%s/projects/%s/operations/runbooks/%s/snapshots/%s", opts.Host, opts.Space.GetID(), project.GetID(), runbook.GetID(), snapshotID)
	fmt.Fprintf(opts.Out, "View this snapshot on Octopus Deploy: %s\n", link)

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.GetSpaceNameOrEmpty(), opts.Project, opts.Runbook, opts.Snapshot)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	return nil
}

func resolveSnapshot(opts *UpdateVariablesOptions, runbook *runbooks.Runbook) (id string, name string, err error) {
	if opts.Snapshot.Value != "" {
		snapshot, err := runbooks.GetSnapshot(opts.Client, opts.Space.GetID(), runbook.ProjectID, opts.Snapshot.Value)
		if err != nil {
			return "", "", err
		}
		if snapshot == nil {
			return "", "", errors.New("unable to find snapshot")
		}
		return snapshot.GetID(), snapshot.Name, nil
	}

	if runbook.PublishedRunbookSnapshotID == "" {
		return "", "", errors.New("runbook has no published snapshot; specify a snapshot with --snapshot")
	}

	snapshot, err := runbooks.GetSnapshot(opts.Client, opts.Space.GetID(), runbook.ProjectID, runbook.PublishedRunbookSnapshotID)
	if err != nil {
		return "", "", err
	}
	if snapshot == nil {
		return "", "", fmt.Errorf("unable to find published snapshot '%s'", runbook.PublishedRunbookSnapshotID)
	}
	return snapshot.GetID(), snapshot.Name, nil
}

func PromptMissing(opts *UpdateVariablesOptions) error {
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

func getProject(opts *UpdateVariablesOptions) (*projects.Project, error) {
	var project *projects.Project
	var err error
	if opts.Project.Value == "" {
		project, err = selectors.Select(opts.Ask, "Select the project containing the runbook:", opts.GetAllProjectsCallback, func(p *projects.Project) string { return p.GetName() })
	} else {
		project, err = opts.GetProjectCallback(opts.Project.Value)
	}

	if project == nil {
		return nil, errors.New("unable to find project")
	}

	return project, err
}

func getRunbook(opts *UpdateVariablesOptions, project *projects.Project) (*runbooks.Runbook, error) {
	var runbook *runbooks.Runbook
	var err error
	if opts.Runbook.Value == "" {
		runbook, err = selectors.Select(opts.Ask, "Select the runbook:", func() ([]*runbooks.Runbook, error) { return opts.GetDbRunbooksCallback(project.GetID()) }, func(r *runbooks.Runbook) string { return r.Name })
	} else {
		runbook, err = opts.GetDbRunbookCallback(project.GetID(), opts.Runbook.Value)
	}

	if runbook == nil {
		return nil, errors.New("unable to find runbook")
	}

	return runbook, err
}
