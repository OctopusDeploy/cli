package create

import (
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/runbook/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/gitresources"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/packages"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/feeds"
	clientGit "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/gitdependencies"
	clientPackages "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/packages"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/spf13/cobra"
	"os"
)

const (
	FlagProject            = "project"
	FlagRunbook            = "runbook"
	FlagName               = "name"
	FlagPublish            = "publish"
	FlagSnapshotNotes      = "snapshot-notes"
	FlagSnapshotNotesFile  = "snapshot-notes-file"
	FlagPackageVersionSpec = "package"
	FlagPackageVersion     = "package-version"
	FlagGitResourceRefSpec = "git-resource"
)

type CreateFlags struct {
	Runbook             *flag.Flag[string]
	Project             *flag.Flag[string]
	Name                *flag.Flag[string]
	Publish             *flag.Flag[bool]
	SnapshotNotes       *flag.Flag[string]
	SnapshotNotesFile   *flag.Flag[string]
	PackageVersion      *flag.Flag[string]
	PackageVersionSpec  *flag.Flag[[]string]
	GitResourceRefsSpec *flag.Flag[[]string]
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Project:             flag.New[string](FlagProject, false),
		Runbook:             flag.New[string](FlagRunbook, false),
		Name:                flag.New[string](FlagName, false),
		Publish:             flag.New[bool](FlagPublish, false),
		SnapshotNotes:       flag.New[string](FlagSnapshotNotes, false),
		SnapshotNotesFile:   flag.New[string](FlagSnapshotNotesFile, false),
		PackageVersion:      flag.New[string](FlagPackageVersion, false),
		PackageVersionSpec:  flag.New[[]string](FlagPackageVersionSpec, false),
		GitResourceRefsSpec: flag.New[[]string](FlagGitResourceRefSpec, false),
	}
}

type CreateOptions struct {
	*CreateFlags
	PackageVersionOverrides []string
	*shared.RunbooksOptions
	GetAllProjectsCallback shared.GetAllProjectsCallback
	*cmd.Dependencies
}

func NewCreateOptions(createFlags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:            createFlags,
		RunbooksOptions:        shared.NewGetRunbooksOptions(dependencies),
		GetAllProjectsCallback: func() ([]*projects.Project, error) { return shared.GetAllProjects(dependencies.Client) },
		Dependencies:           dependencies,
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a runbook snapshot",
		Long:    "Create a runbook snapshot in Octopus Deploy",
		Aliases: []string{"new"},
		Example: heredoc.Docf(`
			$ %[1]s runbook snapshot create --project MyProject --runbook "Rebuild DB Indexes"
			$ %[1]s runbook snapshot create --project MyProject --runbook "Rebuild DB Indexes" --name "My cool snapshot"
			$ %[1]s runbook snapshot create -p MyProject -r "Restart App" --package "azure-cli:1.2.3" --no-prompt
			$ %[1]s runbook snapshot create -p MyProject -r "Restart App" --git-resource "Script step from Git:refs/heads/dev-branch" --publish --no-prompt
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))
			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Project.Value, createFlags.Project.Name, "p", "", "Name or ID of the project where the runbook is")
	flags.StringVarP(&createFlags.Runbook.Value, createFlags.Runbook.Name, "r", "", "Name or ID of the runbook to create the snapshot for")
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "Override the snapshot name")
	flags.StringVarP(&createFlags.PackageVersion.Value, createFlags.PackageVersion.Name, "", "", "Default version to use for all packages. Only relevant for config-as-code projects where runbooks are stored in Git.")
	flags.StringArrayVarP(&createFlags.PackageVersionSpec.Value, createFlags.PackageVersionSpec.Name, "", []string{}, "Version specification a specific packages.\nFormat as {package}:{version}, {step}:{version} or {package-ref-name}:{packageOrStep}:{version}\nYou may specify this multiple times")
	flags.StringVar(&createFlags.SnapshotNotes.Value, createFlags.SnapshotNotes.Name, "", "Release notes to attach")
	flags.StringVar(&createFlags.SnapshotNotesFile.Value, createFlags.SnapshotNotesFile.Name, "", "Release notes to attach (from file)")
	flags.BoolVar(&createFlags.Publish.Value, createFlags.Publish.Name, false, "Publish the snapshot immediately")
	flags.StringArrayVarP(&createFlags.GitResourceRefsSpec.Value, createFlags.GitResourceRefsSpec.Name, "", nil, "Git reference for a specific Git resource.\nFormat as {step}:{git-ref}, {step}:{git-resource-name}:{git-ref}\nYou may specify this multiple times.\nOnly relevant for config-as-code projects where runbooks are stored in Git.")

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

	runbooksInGit := areRunbooksInGit(project)
	if runbooksInGit {
		return errors.New("creating independent Runbook snapshots is not supported for Runbooks stored in Git")
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

	packageVersionOverrides := make([]*packages.PackageVersionOverride, 0)
	packageVersionBaseline := buildPackageVersionBaseline(opts, runbookTemplate)
	for _, s := range opts.PackageVersionSpec.Value {
		ambOverride, err := packages.ParsePackageOverrideString(s)
		if err != nil {
			continue // silently ignore anything that wasn't parseable (should we emit a warning?)
		}
		resolvedOverride, err := packages.ResolvePackageOverride(ambOverride, packageVersionBaseline)
		if err != nil {
			continue // silently ignore anything that wasn't parseable (should we emit a warning?)
		}
		packageVersionOverrides = append(packageVersionOverrides, resolvedOverride)
	}

	selectedPackages := packages.ApplyPackageOverrides(packageVersionBaseline, packageVersionOverrides)

	snapshot := runbooks.NewRunbookSnapshot(snapshotName, project.GetID(), runbook.ID)
	if opts.SnapshotNotes.Value != "" {
		snapshot.Notes = opts.SnapshotNotes.Value
	}
	snapshot.SelectedPackages = util.SliceTransform(selectedPackages, func(p *packages.StepPackageVersion) *clientPackages.SelectedPackage {
		return &clientPackages.SelectedPackage{
			ActionName:           p.ActionName,
			StepName:             p.ActionName,
			PackageReferenceName: p.PackageReferenceName,
			Version:              p.Version,
		}
	})

	gitRefOverrides := make([]*gitresources.GitResourceGitRef, 0)
	gitResourceBaseline := gitresources.BuildGitResourcesBaseline(runbookTemplate.GitResources)
	for _, s := range opts.GitResourceRefsSpec.Value {
		ambOverride, err := gitresources.ParseGitResourceGitRefString(s)
		if err != nil {
			continue // silently ignore anything that wasn't parseable (should we emit a warning?)
		}
		resolvedOverride, err := gitresources.ResolveGitResourceOverride(ambOverride, gitResourceBaseline)
		if err != nil {
			continue // silently ignore anything that wasn't parseable (should we emit a warning?)
		}
		gitRefOverrides = append(gitRefOverrides, resolvedOverride)
	}

	selectedGitRefs := gitresources.ApplyGitResourceOverrides(gitResourceBaseline, gitRefOverrides)
	snapshot.SelectedGitResources = util.SliceTransform(selectedGitRefs, func(g *gitresources.GitResourceGitRef) *clientGit.SelectedGitResources {
		return &clientGit.SelectedGitResources{
			ActionName: g.ActionName,
			GitReference: &clientGit.GitReference{
				GitRef: g.GitRef,
			},
			GitResourceReferenceName: g.GitResourceName,
		}
	})

	if opts.Publish.Value {
		snapshot, err = opts.Client.RunbookSnapshots.Publish(snapshot)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(opts.Out, "\nSuccessfully created and published runbook snapshot '%s' (%s) for runbook '%s'\n", snapshot.Name, snapshot.GetID(), runbook.Name)
	} else {
		snapshot, err = opts.Client.RunbookSnapshots.Add(snapshot)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(opts.Out, "\nSuccessfully created runbook snapshot '%s' (%s) for runbook '%s'\n", snapshot.Name, snapshot.GetID(), runbook.Name)
	}

	link := output.Bluef("%s/app#/%s/projects/%s/operations/runbooks/%s/snapshots/%s", opts.Host, opts.Space.GetID(), project.GetID(), runbook.GetID(), snapshot.GetID())
	fmt.Fprintf(opts.Out, "View this snapshot on Octopus Deploy: %s\n", link)

	return nil
}

func buildPackageVersionBaseline(opts *CreateOptions, runbookTemplate *runbooks.RunbookSnapshotTemplate) []*packages.StepPackageVersion {
	feedPackageVersions := make(map[string]string)
	pkgs := make([]*packages.StepPackageVersion, 0, len(runbookTemplate.Packages))
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
		pkg := &packages.StepPackageVersion{
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
	project, err := getProject(opts)
	if err != nil {
		return err
	}
	opts.Project.Value = project.GetName()

	runbooksInGit := areRunbooksInGit(project)
	if runbooksInGit {
		return errors.New("creating independent Runbook snapshots is not supported for Runbooks stored in Git")
	}

	selectedRunbook, err := getRunbook(opts, project)
	if err != nil {
		return err
	}
	opts.Runbook.Value = selectedRunbook.Name

	template, err := opts.Client.Runbooks.GetRunbookSnapshotTemplate(selectedRunbook)

	if err != nil {
		return err
	}

	if opts.Name.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Snapshot name",
			Help:    "A short, memorable, name for this snapshot.",
			Default: template.NextNameIncrement,
		}, &opts.Name.Value); err != nil {
			return err
		}
	}

	packageVersionBaseline, err := packages.BuildPackageVersionBaseline(opts.Client, util.SliceTransform(template.Packages, func(pkg *releases.ReleaseTemplatePackage) releases.ReleaseTemplatePackage { return *pkg }), nil)
	if err != nil {
		return err
	}

	if len(packageVersionBaseline) > 0 { // if we have packages, run the package flow
		_, packageVersionOverrides, err := packages.AskPackageOverrideLoop(
			packageVersionBaseline,
			opts.PackageVersion.Value,
			opts.PackageVersionOverrides,
			opts.Ask,
			opts.Out)

		if err != nil {
			return err
		}

		if len(packageVersionOverrides) > 0 {
			opts.PackageVersionOverrides = make([]string, 0, len(packageVersionOverrides))
			for _, ov := range packageVersionOverrides {
				opts.PackageVersionOverrides = append(opts.PackageVersionOverrides, ov.ToPackageOverrideString())
			}
		}
	}

	gitResourcesBaseline := gitresources.BuildGitResourcesBaseline(template.GitResources)
	if len(gitResourcesBaseline) > 0 {
		overriddenGitResources, err := gitresources.AskGitResourceOverrideLoop(
			gitResourcesBaseline,
			opts.GitResourceRefsSpec.Value,
			opts.Ask,
			opts.Out)

		if err != nil {
			return err
		}

		if len(overriddenGitResources) > 0 {
			opts.GitResourceRefsSpec.Value = make([]string, 0, len(overriddenGitResources))
			for _, ov := range overriddenGitResources {
				opts.GitResourceRefsSpec.Value = append(opts.GitResourceRefsSpec.Value, ov.ToGitResourceGitRefString())
			}
		}
	}

	if !opts.Publish.Value {
		if err = opts.Ask(&survey.Confirm{
			Message: "Would you like to publish this snapshot immediately?",
			Default: false,
		}, &opts.Publish.Value); err != nil {
			return err
		}
	}

	return nil
}

func getProject(opts *CreateOptions) (*projects.Project, error) {
	var project *projects.Project
	var err error
	if opts.Project.Value == "" {
		project, err = selectors.Select(opts.Ask, "Select the project containing the runbook you wish to snapshot:", opts.GetAllProjectsCallback, func(project *projects.Project) string { return project.GetName() })
	} else {
		project, err = opts.GetProjectCallback(opts.Project.Value)
	}

	if project == nil {
		return nil, errors.New("unable to find project")
	}

	return project, err
}

func getRunbook(opts *CreateOptions, project *projects.Project) (*runbooks.Runbook, error) {
	var runbook *runbooks.Runbook
	var err error
	if opts.Runbook.Value == "" {
		runbook, err = selectors.Select(opts.Ask, "Select the runbook you wish to to snapshot:", func() ([]*runbooks.Runbook, error) { return opts.GetDbRunbooksCallback(project.GetID()) }, func(runbook *runbooks.Runbook) string { return runbook.Name })
	} else {
		runbook, err = opts.GetDbRunbookCallback(project.GetID(), opts.Runbook.Value)
	}

	if runbook == nil {
		return nil, errors.New("unable to find runbook")
	}

	return runbook, err
}

func areRunbooksInGit(project *projects.Project) bool {
	inGit := false

	if project.PersistenceSettings.Type() == projects.PersistenceSettingsTypeVersionControlled {
		inGit = project.PersistenceSettings.(projects.GitPersistenceSettings).RunbooksAreInGit()
	}

	return inGit
}
