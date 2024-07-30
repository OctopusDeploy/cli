package allow

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/release/progression/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/defects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/spf13/cobra"
)

const (
	FlagProject                  = "project"
	FlagVersion                  = "version"
	FlagAliasReleaseNumberLegacy = "releaseNumber" // alias for FlagVersion
)

type AllowFlags struct {
	Project *flag.Flag[string]
	Version *flag.Flag[string]
}

type AllowOptions struct {
	ReleaseID string
	*AllowFlags
	*cmd.Dependencies
}

func NewAllowFlags() *AllowFlags {
	return &AllowFlags{
		Project: flag.New[string](FlagProject, false),
		Version: flag.New[string](FlagVersion, false),
	}
}

func NewAllowOptions(flags *AllowFlags, dependencies *cmd.Dependencies) *AllowOptions {
	return &AllowOptions{
		AllowFlags:   flags,
		Dependencies: dependencies,
	}
}

func NewCmdAllow(f factory.Factory) *cobra.Command {
	allowFlags := NewAllowFlags()

	cmd := &cobra.Command{
		Use:   "allow",
		Short: "Allows a release to progress to the next phase.",
		Long:  "Allows a release to progress to the next phase in Octopus Deploy.",
		Example: heredoc.Docf(`
			$ %[1]s release progression allow --project MyProject --version 1.2.3
			$ %[1]s release progression allow -p MyProject -v 1.2.3
			$ %[1]s release progression allow -p MyProject -v 1.2.3 --no-prompt
		`, constants.ExecutableName),
		Aliases: []string{"allow-releaseprogression"},
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewAllowOptions(allowFlags, cmd.NewDependencies(f, c))
			return resolveReleaseDefectRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&allowFlags.Project.Value, allowFlags.Project.Name, "p", "", "Name or ID of the project.")
	flags.StringVarP(&allowFlags.Version.Value, allowFlags.Version.Name, "v", "", "Release version/number.")

	flags.SortFlags = false

	flagAliases := make(map[string][]string, 1)
	util.AddFlagAliasesString(flags, FlagVersion, flagAliases, FlagAliasReleaseNumberLegacy)

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		util.ApplyFlagAliases(cmd.Flags(), flagAliases)
		return nil
	}

	return cmd
}

func resolveReleaseDefectRun(opts *AllowOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	var err error
	opts.ReleaseID, err = shared.GetReleaseID(opts.Client, opts.Client.GetSpaceID(), opts.Project.Value, opts.Version.Value)
	if err != nil {
		return err
	}

	resolveDefectCommand, err := defects.NewResolveReleaseDefectCommand(opts.ReleaseID)
	if err != nil {
		return err
	}

	existingDefects, err := defects.GetAll(opts.Client, opts.Client.GetSpaceID(), opts.ReleaseID)
	if err != nil {
		return err
	}

	isReleaseAllowedToProgressAlready := util.SliceContainsAll(existingDefects, func(item *defects.Defect) bool {
		return item.Status == defects.DefectStatusResolved
	})
	if isReleaseAllowedToProgressAlready {
		_, _ = fmt.Fprintf(opts.Out, "Release with version/release number '%s' (%s) is already allowed to progress to the next phase.\n", opts.Version.Value, output.Dim(opts.ReleaseID))
		return nil
	}

	_, err = defects.Resolve(opts.Client, opts.Client.GetSpaceID(), resolveDefectCommand)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.Out, "Successfully allowed progression for release %s (%s) in project %s\n", opts.Version.Value, output.Dim(opts.ReleaseID), opts.Project.Value)
	if err != nil {
		return err
	}

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Project, opts.Version)
		_, _ = fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	return nil
}

func PromptMissing(opts *AllowOptions) error {
	var err error
	var selectedProject *projects.Project
	if opts.Project.Value == "" {
		selectedProject, err = selectors.Project("Select the project in which the blocked release exists", opts.Client, opts.Ask)
		if err != nil {
			return err
		}
	}
	opts.Project.Value = selectedProject.GetName()

	var selectedRelease *releases.Release
	if opts.Version.Value == "" {
		selectedRelease, err = shared.SelectRelease(opts.Client, selectedProject, opts.Ask, "Allow")
		if err != nil {
			return err
		}
	}
	opts.Version.Value = selectedRelease.Version

	return nil
}
