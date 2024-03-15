package create

import (
	"errors"
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/executionscommon"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/feeds"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/packages"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/spf13/cobra"
	"os"
)

const (
	FlagProject            = "project"
	FlagRunbook            = "runbook"
	FlagName               = "name"
	FlagPackageVersionSpec = "package"
	FlagPackageVersion     = "package-version"
	FlagSnapshotNotes      = "snapshot-notes"
	FlagSnapshotNotesFile  = "snapshot-notes-file"
)

type CreateFlags struct {
	Runbook            *flag.Flag[string]
	Project            *flag.Flag[string]
	Name               *flag.Flag[string]
	PackageVersion     *flag.Flag[string]
	SnapshotNotes      *flag.Flag[string]
	SnapshotNotesFile  *flag.Flag[string]
	PackageVersionSpec *flag.Flag[[]string]
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Project:            flag.New[string](FlagProject, false),
		Runbook:            flag.New[string](FlagRunbook, false),
		Name:               flag.New[string](FlagName, false),
		SnapshotNotes:      flag.New[string](FlagSnapshotNotes, false),
		SnapshotNotesFile:  flag.New[string](FlagSnapshotNotesFile, false),
		PackageVersionSpec: flag.New[[]string](FlagPackageVersionSpec, false),
		PackageVersion:     flag.New[string](FlagPackageVersion, false),
	}
}

type CreateOptions struct {
	*CreateFlags
	*cmd.Dependencies
}

func NewCreateOptions(createFlags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:  createFlags,
		Dependencies: dependencies,
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a runbook snapshot",
		Long:    "Create a runbook snapshot in Octopus Deploy",
		Aliases: []string{"new", "publish"},
		Example: heredoc.Docf(`
			$ %[1]s runbook snapshot create --project MyProject --runbook "Rebuild DB Indexes" 
			$ %[1]s runbook snapshot create -p MyProject -r "Restart App" --package "azure-cli:1.2.3" --no-prompt
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))
			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Project.Value, createFlags.Project.Name, "p", "", "Name or ID of the project where the runbook is")
	flags.StringVarP(&createFlags.Runbook.Value, createFlags.Runbook.Name, "r", "", "Name or ID of the runbook to create the snapshot for")
	flags.StringVar(&createFlags.PackageVersion.Value, createFlags.PackageVersion.Name, "", "Default version to use for all Packages")
	flags.StringArrayVarP(&createFlags.PackageVersionSpec.Value, createFlags.PackageVersionSpec.Name, "", []string{}, "Version specification a specific packages.\nFormat as {package}:{version}, {step}:{version} or {package-ref-name}:{packageOrStep}:{version}\nYou may specify this multiple times")
	flags.StringVar(&createFlags.SnapshotNotes.Value, createFlags.SnapshotNotes.Name, "", "Release notes to attach")
	flags.StringVar(&createFlags.SnapshotNotesFile.Value, createFlags.SnapshotNotesFile.Name, "", "Release notes to attach (from file)")
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "Override the snapshot name")

	return cmd
}

func createRun(opts *CreateOptions) error {
	if opts.SnapshotNotes.Value != "" && opts.SnapshotNotesFile.Value != "" {
		return errors.New("cannot specify both --snapshot-notes and --snapshot-notes-file at the same time")
	}

	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	if opts.SnapshotNotesFile.Value != "" {
		fileContents, err := os.ReadFile(opts.SnapshotNotesFile.Value)
		if err != nil {
			return err
		}
		opts.SnapshotNotes.Value = string(fileContents)
	}

	project, err := selectors.FindProject(opts.Client, opts.Project.Value)
	if err != nil {
		return err
	}
	if project == nil {
		return errors.New("unable to find project")
	}

	runbook, err := selectors.FindRunbook(opts.Client, project, opts.Runbook.Value)
	if err != nil {
		return err
	}
	if runbook == nil {
		return errors.New("unable to find runbook")
	}

	runbookTemplate, err := opts.Client.Runbooks.GetRunbookSnapshotTemplate(runbook)
	if err != nil {
		return err
	}

	snapshotName := getSnapshotName(opts, runbookTemplate)
	if err != nil {
		return err
	}

	packageVersionOverrides := make([]*executionscommon.PackageVersionOverride, 0)
	packageVersionBaseline := buildPackageVersionBaseline(opts, runbookTemplate)
	for _, s := range opts.PackageVersionSpec.Value {
		ambOverride, err := executionscommon.ParsePackageOverrideString(s)
		if err != nil {
			continue // silently ignore anything that wasn't parseable (should we emit a warning?)
		}
		resolvedOverride, err := executionscommon.ResolvePackageOverride(ambOverride, packageVersionBaseline)
		if err != nil {
			continue // silently ignore anything that wasn't parseable (should we emit a warning?)
		}
		packageVersionOverrides = append(packageVersionOverrides, resolvedOverride)
	}

	selectedPackages := executionscommon.ApplyPackageOverrides(packageVersionBaseline, packageVersionOverrides)

	snapshot := runbooks.NewRunbookSnapshot(snapshotName, project.GetID(), runbook.ID)
	if opts.SnapshotNotes.Value != "" {
		snapshot.Notes = opts.SnapshotNotes.Value
	}
	snapshot.SelectedPackages = util.SliceTransform(selectedPackages, func(p *executionscommon.StepPackageVersion) *packages.SelectedPackage {
		return &packages.SelectedPackage{
			ActionName:           p.ActionName,
			PackageReferenceName: p.PackageReferenceName,
			StepName:             p.ActionName,
			Version:              p.Version,
		}
	})

	createdSnapshot, err := opts.Client.RunbookSnapshots.Publish(snapshot)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.Out, "\nSuccessfully created and published runbook snapshot '%s' (%s) for runbook '%s'\n", createdSnapshot.Name, createdSnapshot.GetID(), runbook.Name)
	if err != nil {
		return err
	}

	link := output.Bluef("%s/app#/%s/projects/%s/operations/runbooks/%s/snapshots/%s", opts.Host, opts.Space.GetID(), project.GetID(), runbook.GetID(), createdSnapshot.GetID())
	fmt.Fprintf(opts.Out, "View this project on Octopus Deploy: %s\n", link)

	return nil
}

func buildPackageVersionBaseline(opts *CreateOptions, runbookTemplate *runbooks.RunbookSnapshotTemplate) []*executionscommon.StepPackageVersion {
	feedPackageVersions := make(map[string]string)
	pkgs := make([]*executionscommon.StepPackageVersion, 0, len(runbookTemplate.Packages))
	for _, p := range runbookTemplate.Packages {
		{
			key := fmt.Sprintf("%s/%s", p.FeedID, p.PackageID)
			if _, ok := feedPackageVersions[key]; !ok {
				packageVersion, err := feeds.SearchPackageVersions(opts.Client, opts.Client.GetSpaceID(), p.FeedID, p.PackageID, "", 1)
				if err == nil && packageVersion != nil && !util.Empty(packageVersion.Items) {
					feedPackageVersions[key] = packageVersion.Items[0].Version
				}
			}
		}
	}

	for _, p := range runbookTemplate.Packages {
		pkg := &executionscommon.StepPackageVersion{
			PackageID:            p.PackageID,
			ActionName:           p.ActionName,
			PackageReferenceName: p.PackageReferenceName}
		if !p.IsResolvable {
			pkg.Version = ""
		} else {
			key := fmt.Sprintf("%s/%s", p.FeedID, p.PackageID)
			pkg.Version = feedPackageVersions[key]
		}
		pkgs = append(pkgs, pkg)
	}

	return pkgs
}

func getSnapshotName(opts *CreateOptions, template *runbooks.RunbookSnapshotTemplate) string {
	if opts.Name.Value != "" {
		return opts.Name.Value
	}

	return template.NextNameIncrement
}

func PromptMissing(opts *CreateOptions) error {
	return nil
}
