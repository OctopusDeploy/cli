package update_variables

import (
	"fmt"
	"io"
	"net/http"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/release/progression/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"
)

const (
	FlagProject                  = "project"
	FlagVersion                  = "version"
	FlagAliasReleaseNumberLegacy = "releaseNumber"
)

type UpdateVariablesFlags struct {
	Project *flag.Flag[string]
	Version *flag.Flag[string]
}

func NewUpdateVariablesFlags() *UpdateVariablesFlags {
	return &UpdateVariablesFlags{
		Project: flag.New[string](FlagProject, false),
		Version: flag.New[string](FlagVersion, false),
	}
}

type UpdateVariablesOptions struct {
	*UpdateVariablesFlags
	*cmd.Dependencies
}

func NewUpdateVariablesOptions(flags *UpdateVariablesFlags, dependencies *cmd.Dependencies) *UpdateVariablesOptions {
	return &UpdateVariablesOptions{
		UpdateVariablesFlags: flags,
		Dependencies:         dependencies,
	}
}

func NewCmdUpdateVariables(f factory.Factory) *cobra.Command {
	updateVariablesFlags := NewUpdateVariablesFlags()

	cmd := &cobra.Command{
		Use:   "update-variables",
		Short: "Update the variable snapshot for a release",
		Long:  "Update the variable snapshot for a release in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s release update-variables --project MyProject --version 1.2.3
			$ %[1]s release update-variables -p MyProject -v 1.2.3
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewUpdateVariablesOptions(updateVariablesFlags, cmd.NewDependencies(f, c))
			return updateVariablesRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&updateVariablesFlags.Project.Value, updateVariablesFlags.Project.Name, "p", "", "Name or ID of the project")
	flags.StringVarP(&updateVariablesFlags.Version.Value, updateVariablesFlags.Version.Name, "v", "", "Release version/number")

	flags.SortFlags = false

	flagAliases := make(map[string][]string, 1)
	util.AddFlagAliasesString(flags, FlagVersion, flagAliases, FlagAliasReleaseNumberLegacy)

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		util.ApplyFlagAliases(cmd.Flags(), flagAliases)
		return nil
	}

	return cmd
}

func updateVariablesRun(opts *UpdateVariablesOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	releaseID, err := shared.GetReleaseID(opts.Client, opts.Client.GetSpaceID(), opts.Project.Value, opts.Version.Value)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/api/%s/releases/%s/snapshot-variables", opts.Client.GetSpaceID(), releaseID)
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
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update variable snapshot (HTTP %d): %s", resp.StatusCode, string(body))
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

func PromptMissing(opts *UpdateVariablesOptions) error {
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
