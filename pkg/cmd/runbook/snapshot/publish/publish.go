package publish

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
	"math"
	"time"
)

const (
	FlagProject  = "project"
	FlagRunbook  = "runbook"
	FlagSnapshot = "snapshot"
)

type PublishFlags struct {
	Runbook  *flag.Flag[string]
	Project  *flag.Flag[string]
	Snapshot *flag.Flag[string]
}

func NewPublishFlags() *PublishFlags {
	return &PublishFlags{
		Runbook:  flag.New[string](FlagRunbook, false),
		Project:  flag.New[string](FlagProject, false),
		Snapshot: flag.New[string](FlagSnapshot, false),
	}
}

type PublishOptions struct {
	*PublishFlags
	*shared.RunbooksOptions
	GetAllProjectsCallback shared.GetAllProjectsCallback
	*cmd.Dependencies
}

func NewPublishOptions(publishFlags *PublishFlags, dependencies *cmd.Dependencies) *PublishOptions {
	return &PublishOptions{
		PublishFlags:           publishFlags,
		RunbooksOptions:        shared.NewGetRunbooksOptions(dependencies),
		GetAllProjectsCallback: func() ([]*projects.Project, error) { return shared.GetAllProjects(dependencies.Client) },
		Dependencies:           dependencies,
	}
}

func NewCmdPublish(f factory.Factory) *cobra.Command {
	publishFlags := NewPublishFlags()
	cmd := &cobra.Command{
		Use:     "publish",
		Short:   "Publish a runbook snapshot",
		Long:    "Publish a runbook snapshot in Octopus Deploy",
		Aliases: []string{"new", "publish"},
		Example: heredoc.Docf(`
			$ %[1]s runbook snapshot publish --project MyProject --runbook "Rebuild DB Indexes" --snapshot "Snapshot 40C9ENM"
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewPublishOptions(publishFlags, cmd.NewDependencies(f, c))
			return publishRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&publishFlags.Project.Value, publishFlags.Project.Name, "p", "", "Name or ID of the project where the runbook is")
	flags.StringVarP(&publishFlags.Runbook.Value, publishFlags.Runbook.Name, "r", "", "Name or ID of the runbook to publish an existing snapshot")
	flags.StringVarP(&publishFlags.Snapshot.Value, publishFlags.Snapshot.Name, "", "", "Name or ID of the snapshot to publish")

	return cmd
}

func publishRun(opts *PublishOptions) error {
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

	runbooksInGit := shared.AreRunbooksInGit(project)
	if runbooksInGit {
		return errors.New("independent Runbook snapshots is not supported for Runbooks stored in Git")
	}

	runbook, err := selectors.FindRunbook(opts.Client, project, opts.Runbook.Value)

	if err != nil {
		return err
	}
	if runbook == nil {
		return errors.New("unable to find runbook")
	}

	snapshot, err := findSnapshot(opts, runbook)
	if err != nil {
		return err
	}

	runbook.PublishedRunbookSnapshotID = snapshot.ID
	runbook, err = runbooks.Update(opts.Client, runbook)
	if err != nil {
		return err
	}

	fmt.Fprintf(opts.Out, "Runbook snapshot %s has been published\n", output.Greenf("%s", snapshot.Name))
	link := output.Bluef("%s/app#/%s/projects/%s/operations/runbooks/%s/snapshots/%s", opts.Host, opts.Space.GetID(), project.GetID(), runbook.GetID(), snapshot.GetID())
	fmt.Fprintf(opts.Out, "View this snapshot on Octopus Deploy: %s\n", link)

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Project, opts.Runbook, opts.Snapshot)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}
	return nil
}

func PromptMissing(opts *PublishOptions) error {
	project, err := getProject(opts)
	if err != nil {
		return err
	}
	opts.Project.Value = project.GetName()

	runbooksInGit := shared.AreRunbooksInGit(project)
	if runbooksInGit {
		return errors.New("independent Runbook snapshots is not supported for Runbooks stored in Git")
	}

	selectedRunbook, err := getRunbook(opts, project)
	if err != nil {
		return err
	}
	opts.Runbook.Value = selectedRunbook.Name

	selectedSnapshot, err := getSnapshot(opts, selectedRunbook)
	if err != nil {
		return err
	}
	opts.Snapshot.Value = selectedSnapshot.Name

	return nil
}

func getSnapshot(opts *PublishOptions, runbook *runbooks.Runbook) (*runbooks.RunbookSnapshot, error) {
	if opts.Snapshot.Value != "" {
		snapshot, err := runbooks.GetSnapshot(opts.Client, opts.Space.GetID(), runbook.ProjectID, opts.Snapshot.Value)
		if err != nil {
			return nil, err
		}
		if snapshot == nil {
			return nil, errors.New("unable to find snapshot")
		}

		return snapshot, nil
	}

	snapshot, err := selectors.Select(opts.Ask, "Select the snapshot to publish:", func() ([]*runbooks.RunbookSnapshot, error) {
		allSnapshots, err := runbooks.ListSnapshots(opts.Client, opts.Space.GetID(), runbook.ProjectID, runbook.GetID(), math.MaxInt16)
		if err != nil {
			return nil, err
		}

		var availableSnapshots = make([]*runbooks.RunbookSnapshot, 0)
		for _, s := range allSnapshots.Items {
			if s.GetID() != runbook.PublishedRunbookSnapshotID {
				availableSnapshots = append(availableSnapshots, s)
			}
		}
		return availableSnapshots, nil
	}, func(s *runbooks.RunbookSnapshot) string {
		return fmt.Sprintf("%s (Assembled: %s)", s.Name, s.Assembled.Format(time.RFC1123Z))
	})

	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func getProject(opts *PublishOptions) (*projects.Project, error) {
	var project *projects.Project
	var err error
	if opts.Project.Value == "" {
		project, err = selectors.Select(opts.Ask, "Select the project containing the runbook you wish update the snapshot:", opts.GetAllProjectsCallback, func(project *projects.Project) string { return project.GetName() })
	} else {
		project, err = opts.GetProjectCallback(opts.Project.Value)
	}

	if project == nil {
		return nil, errors.New("unable to find project")
	}

	return project, err
}

func getRunbook(opts *PublishOptions, project *projects.Project) (*runbooks.Runbook, error) {
	var runbook *runbooks.Runbook
	var err error
	if opts.Runbook.Value == "" {
		runbook, err = selectors.Select(opts.Ask, "Select the runbook you wish to you wish to update the snapshot:", func() ([]*runbooks.Runbook, error) { return opts.GetDbRunbooksCallback(project.GetID()) }, func(runbook *runbooks.Runbook) string { return runbook.Name })
	} else {
		runbook, err = opts.GetDbRunbookCallback(project.GetID(), opts.Runbook.Value)
	}

	if runbook == nil {
		return nil, errors.New("unable to find runbook")
	}

	return runbook, err
}

func findSnapshot(opts *PublishOptions, runbook *runbooks.Runbook) (*runbooks.RunbookSnapshot, error) {
	snapshot, err := runbooks.GetSnapshot(opts.Client, opts.Space.GetID(), runbook.ProjectID, opts.Snapshot.Value)

	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, errors.New("unable to find snapshot")
	}

	return snapshot, nil
}
