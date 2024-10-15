package prevent

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
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
	FlagReason                   = "reason"
)

type PreventFlags struct {
	Project *flag.Flag[string]
	Version *flag.Flag[string]
	Reason  *flag.Flag[string]
}

type PreventOptions struct {
	ReleaseID string
	*PreventFlags
	*cmd.Dependencies
}

func NewPreventFlags() *PreventFlags {
	return &PreventFlags{
		Project: flag.New[string](FlagProject, false),
		Version: flag.New[string](FlagVersion, false),
		Reason:  flag.New[string](FlagReason, false),
	}
}

func NewPreventOptions(flags *PreventFlags, dependencies *cmd.Dependencies) *PreventOptions {
	return &PreventOptions{
		PreventFlags: flags,
		Dependencies: dependencies,
	}
}

func NewCmdPrevent(f factory.Factory) *cobra.Command {
	preventFlags := NewPreventFlags()

	cmd := &cobra.Command{
		Use:   "prevent",
		Short: "Prevents a release from progression to the next phase",
		Long:  "Prevents a release from progression to the next phase in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s release progression prevent --project MyProject --version 1.2.3 --reason "It's broken"
			$ %[1]s release progression prevent -p MyProject -v 1.2.3 -r "It's broken"
			$ %[1]s release progression prevent -p MyProject -v 1.2.3 -r "It's broken" --no-prompt
		`, constants.ExecutableName),
		Aliases: []string{"prevent-releaseprogression"},
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewPreventOptions(preventFlags, cmd.NewDependencies(f, c))
			return createReleaseDefectRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&preventFlags.Project.Value, preventFlags.Project.Name, "p", "", "Name or ID of the project.")
	flags.StringVarP(&preventFlags.Version.Value, preventFlags.Version.Name, "v", "", "Release version/number.")
	flags.StringVarP(&preventFlags.Reason.Value, preventFlags.Reason.Name, "r", "", "Reason to prevent this release from progressing to the next phase.")

	flags.SortFlags = false

	flagAliases := make(map[string][]string, 1)
	util.AddFlagAliasesString(flags, FlagVersion, flagAliases, FlagAliasReleaseNumberLegacy)

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		util.ApplyFlagAliases(cmd.Flags(), flagAliases)
		return nil
	}

	return cmd
}

func createReleaseDefectRun(opts *PreventOptions) error {
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

	existingDefects, err := defects.GetAll(opts.Client, opts.Client.GetSpaceID(), opts.ReleaseID)
	if err != nil {
		return err
	}

	isReleasePreventedToProgressAlready := util.SliceContainsAny(existingDefects, func(item *defects.Defect) bool {
		return item.Status == defects.DefectStatusUnresolved
	})

	if isReleasePreventedToProgressAlready {
		_, _ = fmt.Fprintf(opts.Out, "Release with version/release number '%s' (%s) is already prevented from progressing to the next phase.\n", opts.Version.Value, output.Dim(opts.ReleaseID))
		return nil
	}

	if opts.Reason.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Reason",
			Help:    "Reason to prevent this release from progressing to the next phase.",
		}, &opts.Reason.Value, survey.WithValidator(survey.ComposeValidators(
			survey.MinLength(1),
			survey.Required,
		))); err != nil {
			return err
		}
	}

	createReleaseDefectCommand, err := defects.NewCreateReleaseDefectCommand(opts.ReleaseID, opts.Reason.Value)
	if err != nil {
		return err
	}

	_, err = defects.Create(opts.Client, opts.Client.GetSpaceID(), createReleaseDefectCommand)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.Out, "Successfully prevented progression for release %s (%s) in project %s", opts.Version.Value, output.Dim(opts.ReleaseID), opts.Project.Value)
	if err != nil {
		return err
	}

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Project, opts.Version, opts.Reason)
		_, _ = fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	return nil
}

func PromptMissing(opts *PreventOptions) error {
	var err error
	var selectedProject *projects.Project
	if opts.Project.Value == "" {
		selectedProject, err = selectors.Project("Selected the project in which the release to be blocked exists", opts.Client, opts.Ask)
		if err != nil {
			return err
		}
		opts.Project.Value = selectedProject.GetName()
	}

	var selectedRelease *releases.Release
	if opts.Version.Value == "" {
		selectedRelease, err = shared.SelectRelease(opts.Client, selectedProject, opts.Ask, "Prevent")
		if err != nil {
			return err
		}
		opts.Version.Value = selectedRelease.Version
	}

	return nil
}
