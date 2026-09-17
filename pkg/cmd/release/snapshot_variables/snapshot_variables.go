package snapshot_variables

import (
	"errors"
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/release/progression/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/spf13/cobra"
)

const (
	FlagProject = "project"
	FlagVersion = "version"
)

type SnapshotVariablesFlags struct {
	Project *flag.Flag[string]
	Version *flag.Flag[string]
}

func NewSnapshotVariablesFlags() *SnapshotVariablesFlags {
	return &SnapshotVariablesFlags{
		Project: flag.New[string](FlagProject, false),
		Version: flag.New[string](FlagVersion, false),
	}
}

type SnapshotVariablesOptions struct {
	*SnapshotVariablesFlags
	*cmd.Dependencies
}

func NewSnapshotVariablesOptions(flags *SnapshotVariablesFlags, dependencies *cmd.Dependencies) *SnapshotVariablesOptions {
	return &SnapshotVariablesOptions{
		SnapshotVariablesFlags: flags,
		Dependencies:           dependencies,
	}
}

func NewCmdSnapshotVariables(f factory.Factory) *cobra.Command {
	snapshotVariablesFlags := NewSnapshotVariablesFlags()

	cmd := &cobra.Command{
		Use:   "snapshot-variables",
		Short: "Update the variable snapshot for a release",
		Long:  "Update the variable snapshot for a release in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s release snapshot-variables --project MyProject --version 1.2.3
			$ %[1]s release snapshot-variables -p MyProject -v 1.2.3
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewSnapshotVariablesOptions(snapshotVariablesFlags, cmd.NewDependencies(f, c))
			return snapshotVariablesRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&snapshotVariablesFlags.Project.Value, snapshotVariablesFlags.Project.Name, "p", "", "Name or ID of the project")
	flags.StringVarP(&snapshotVariablesFlags.Version.Value, snapshotVariablesFlags.Version.Name, "v", "", "Release version/number")

	flags.SortFlags = false

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
	if opts.Version.Value == "" {
		return errors.New("version must be specified")
	}

	releaseID, err := shared.GetReleaseID(opts.Client, opts.Client.GetSpaceID(), opts.Project.Value, opts.Version.Value)
	if err != nil {
		return err
	}

	if _, err := releases.UpdateSnapshotVariables(opts.Client, opts.Client.GetSpaceID(), releaseID); err != nil {
		return err
	}

	fmt.Fprintf(opts.Out, "Successfully updated variable snapshot for release '%s' (%s)\n", opts.Version.Value, output.Dim(releaseID))
	link := output.Bluef("%s/app#/%s/releases/%s", opts.Host, opts.Space.GetID(), releaseID)
	fmt.Fprintf(opts.Out, "View this release on Octopus Deploy: %s\n", link)

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.GetSpaceNameOrEmpty(), opts.Project, opts.Version)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	return nil
}

func PromptMissing(opts *SnapshotVariablesOptions) error {
	var selectedProject *projects.Project
	var err error

	if opts.Project.Value == "" {
		selectedProject, err = selectors.Project("Select the project containing the release", opts.Client, opts.Ask)
		if err != nil {
			return err
		}
		opts.Project.Value = selectedProject.GetName()
	} else {
		selectedProject, err = selectors.FindProject(opts.Client, opts.Project.Value)
		if err != nil {
			return err
		}
		if selectedProject == nil {
			return fmt.Errorf("unable to find project '%s'", opts.Project.Value)
		}
	}

	if opts.Version.Value == "" {
		selectedRelease, err := shared.SelectRelease(opts.Client, selectedProject, opts.Ask, "Update Variables for")
		if err != nil {
			return err
		}
		opts.Version.Value = selectedRelease.Version
	}

	return nil
}
