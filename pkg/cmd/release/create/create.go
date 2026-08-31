package create

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/OctopusDeploy/cli/pkg/apiclient"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/release/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/dryrun"
	cliErrors "github.com/OctopusDeploy/cli/pkg/errors"
	"github.com/OctopusDeploy/cli/pkg/executor"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/gitresources"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/packages"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/feeds"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/cobra"
)

const (
	FlagProject            = "project"
	FlagChannel            = "channel"
	FlagPackageVersionSpec = "package"
	FlagGitResourceRefSpec = "git-resource"
	FlagCustomField        = "custom-field"

	FlagVersion                  = "version"
	FlagAliasReleaseNumberLegacy = "releaseNumber" // alias for FlagVersion

	FlagPackageVersion                   = "package-version"
	FlagAliasDefaultPackageVersion       = "default-package-version" // alias for FlagPackageVersion
	FlagAliasPackageVersionLegacy        = "packageVersion"          // alias for FlagPackageVersion
	FlagAliasDefaultPackageVersionLegacy = "defaultPackageVersion"   // alias for FlagPackageVersion

	FlagReleaseNotes            = "release-notes"
	FlagAliasReleaseNotesLegacy = "releaseNotes"

	FlagReleaseNotesFile            = "release-notes-file"
	FlagAliasReleaseNotesFileLegacy = "releaseNotesFile"
	FlagAliasReleaseNoteFileLegacy  = "releaseNoteFile"

	FlagGitRef            = "git-ref"
	FlagAliasGitRefLegacy = "gitRef"
	FlagAliasGitRefRef    = "ref" // alias for FlagGitRef

	FlagGitCommit            = "git-commit"
	FlagAliasGitCommitLegacy = "gitCommit"

	FlagIgnoreExisting            = "ignore-existing"
	FlagAliasIgnoreExistingLegacy = "ignoreExisting"

	FlagIgnoreChannelRules            = "ignore-channel-rules"
	FlagAliasIgnoreChannelRulesLegacy = "ignoreChannelRules"

	// The .NET CLI and the server support --package-prerelease which lets you default all your package versions to
	// latest available <prerelease> e.g. latest available with -beta suffix.
	// This feature is deliberately not supported in the new CLI; it is old (predating Channels), quirky,
	// and far better served by creating a proper channel with an equivalent prerelease tag regex
	//
	//FlagPackagePrerelease            = "package-prerelease"
	//FlagAliasPackagePrereleaseLegacy = "packagePrerelease"
)

var packageOverrideLoopHelpText = heredoc.Doc(`
bold(PACKAGE SELECTION)
 This screen presents the list of packages used by your project, and the steps
 which reference them. 
 If an item is dimmed (gray text) this indicates that the attribute is duplicated.
 For example if you reference the same package in two steps, the second will be dimmed. 

bold(COMMANDS)
 Any any point, you can enter one of the following:
 - green(?) to access this help screen
 - green(y) to accept the list of packages and proceed with creating the release
 - green(u) to undo the last edit you made to package versions
 - green(r) to reset all package version edits
 - A package override string.

bold(PACKAGE OVERRIDE STRINGS)
 Package override strings must have 2 or 3 components, separated by a :
 The last component must always be a version number.
 
 When specifying 2 components, the first component is either a Package ID or a Step Name.
 You can also specify a * which will match all packages
 Examples:
   bold(octopustools:9.1)   dim(# sets package 'octopustools' in all steps to v 9.1)
   bold(Push Package:3.0)   dim(# sets all packages in the 'Push Package' step to v 3.0)
   bold(*:5.1)              dim(# sets all packages in all steps to v 5.1)

 The 3-component syntax is for advanced use cases where you reference the same package twice
 in a single step, and need to distinguish between the two.
 The syntax is bold(packageIDorStepName:packageReferenceName:version)
 Please refer to the octopus server documentation for more information regarding package reference names. 

dim(---------------------------------------------------------------------)
`) // note this expects to have prettifyHelp run over it

type CreateFlags struct {
	Project             *flag.Flag[string]
	Channel             *flag.Flag[string]
	GitRef              *flag.Flag[string]
	GitCommit           *flag.Flag[string]
	PackageVersion      *flag.Flag[string]
	ReleaseNotes        *flag.Flag[string]
	ReleaseNotesFile    *flag.Flag[string]
	Version             *flag.Flag[string]
	IgnoreExisting      *flag.Flag[bool]
	IgnoreChannelRules  *flag.Flag[bool]
	PackageVersionSpec  *flag.Flag[[]string]
	GitResourceRefsSpec *flag.Flag[[]string]
	CustomFields        *flag.Flag[[]string]
	DryRun              *flag.Flag[bool]
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Project:             flag.New[string](FlagProject, false),
		Channel:             flag.New[string](FlagChannel, false),
		GitRef:              flag.New[string](FlagGitRef, false),
		GitCommit:           flag.New[string](FlagGitCommit, false),
		PackageVersion:      flag.New[string](FlagPackageVersion, false),
		ReleaseNotes:        flag.New[string](FlagReleaseNotes, false),
		ReleaseNotesFile:    flag.New[string](FlagReleaseNotesFile, false),
		Version:             flag.New[string](FlagVersion, false),
		IgnoreExisting:      flag.New[bool](FlagIgnoreExisting, false),
		IgnoreChannelRules:  flag.New[bool](FlagIgnoreChannelRules, false),
		PackageVersionSpec:  flag.New[[]string](FlagPackageVersionSpec, false),
		GitResourceRefsSpec: flag.New[[]string](FlagGitResourceRefSpec, false),
		CustomFields:        flag.New[[]string](FlagCustomField, false),
		DryRun:              flag.New[bool](constants.FlagDryRun, false),
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a release",
		Long:  "Create a release in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s release create --project MyProject --channel Beta --version 1.2.3
			%[1]s release create -p MyProject -c Beta -v 1.2.3
			%[1]s release create -p MyProject -c default --package "utils:1.2.3" --package "utils:InstallOnly:5.6.7"
			%[1]s release create -p MyProject --package "com.example\:my-artifact:1.0"
			%[1]s release create -p MyProject -c Beta --no-prompt
			%[1]s release create -p MyProject -c Beta --dry-run
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error { return createRun(cmd, f, createFlags) },
	}

	// project is required in automation mode, other options are not. Nothing is required in interactive mode because we prompt for everything
	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Project.Value, createFlags.Project.Name, "p", "", "Name or ID of the project to create the release in")
	flags.StringVarP(&createFlags.Channel.Value, createFlags.Channel.Name, "c", "", "Name or ID of the channel to use")
	flags.StringVarP(&createFlags.GitRef.Value, createFlags.GitRef.Name, "r", "", "Git Reference e.g. refs/heads/main. Only relevant for config-as-code projects")
	flags.StringVarP(&createFlags.GitCommit.Value, createFlags.GitCommit.Name, "", "", "Git Commit Hash; Specify this in addition to Git Reference if you want to reference a commit other than the latest for that branch/tag.")
	flags.StringVarP(&createFlags.PackageVersion.Value, createFlags.PackageVersion.Name, "", "", "Default version to use for all Packages")
	flags.StringVar(&createFlags.ReleaseNotes.Value, createFlags.ReleaseNotes.Name, "", "Release notes to attach")
	flags.StringVarP(&createFlags.ReleaseNotesFile.Value, createFlags.ReleaseNotesFile.Name, "", "", "Release notes to attach (from file)")
	flags.StringVarP(&createFlags.Version.Value, createFlags.Version.Name, "v", "", "Override the Release Version")
	flags.BoolVarP(&createFlags.IgnoreExisting.Value, createFlags.IgnoreExisting.Name, "x", false, "If a release with the same version exists, do nothing instead of failing.")
	flags.BoolVarP(&createFlags.IgnoreChannelRules.Value, createFlags.IgnoreChannelRules.Name, "", false, "Allow creation of a release where channel rules would otherwise prevent it.")
	flags.StringArrayVarP(&createFlags.PackageVersionSpec.Value, createFlags.PackageVersionSpec.Name, "", []string{}, "Version specification for a specific package. You may specify this multiple times.\nFormat as {package}:{version}, {step}:{version} or {package-ref-name}:{packageOrStep}:{version}\nIf the package ID or step name contains a colon, slash, or equals sign (such as Maven coordinates like com.example:my-artifact), escape that character with a backslash:\n    --package \"com.example\\:my-artifact:1.0\"\nThis escape syntax requires Octopus CLI 2.21.2 or later and Octopus Server 2025.4.10680 or later.")
	flags.StringArrayVarP(&createFlags.GitResourceRefsSpec.Value, createFlags.GitResourceRefsSpec.Name, "", []string{}, "Git reference for a specific Git resource.\nFormat as {step}:{git-ref}, {step}:{git-resource-name}:{git-ref}\nYou may specify this multiple times")
	flags.StringArrayVarP(&createFlags.CustomFields.Value, createFlags.CustomFields.Name, "", []string{}, "Custom field value to set on the release.\nFormat as {name}:{value}. You may specify multiple times")
	dryrun.AddFlag(flags, &createFlags.DryRun.Value)

	// we want the help text to display in the above order, rather than alphabetical
	flags.SortFlags = false

	// flags aliases for compat with old .NET CLI
	flagAliases := make(map[string][]string, 10)
	util.AddFlagAliasesString(flags, FlagGitRef, flagAliases, FlagAliasGitRefRef, FlagAliasGitRefLegacy)
	util.AddFlagAliasesString(flags, FlagGitCommit, flagAliases, FlagAliasGitCommitLegacy)
	util.AddFlagAliasesString(flags, FlagPackageVersion, flagAliases, FlagAliasDefaultPackageVersion, FlagAliasPackageVersionLegacy, FlagAliasDefaultPackageVersionLegacy)
	util.AddFlagAliasesString(flags, FlagReleaseNotes, flagAliases, FlagAliasReleaseNotesLegacy)
	util.AddFlagAliasesString(flags, FlagReleaseNotesFile, flagAliases, FlagAliasReleaseNotesFileLegacy, FlagAliasReleaseNoteFileLegacy)
	util.AddFlagAliasesString(flags, FlagVersion, flagAliases, FlagAliasReleaseNumberLegacy)
	util.AddFlagAliasesBool(flags, FlagIgnoreExisting, flagAliases, FlagAliasIgnoreExistingLegacy)
	util.AddFlagAliasesBool(flags, FlagIgnoreChannelRules, flagAliases, FlagAliasIgnoreChannelRulesLegacy)

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		util.ApplyFlagAliases(cmd.Flags(), flagAliases)
		return nil
	}

	return cmd
}

func createRun(cmd *cobra.Command, f factory.Factory, flags *CreateFlags) error {
	outputFormat, err := cmd.Flags().GetString(constants.FlagOutputFormat)
	if err != nil { // should never happen, but fallback if it does
		outputFormat = constants.OutputFormatTable
	}

	if flags.ReleaseNotes.Value != "" && flags.ReleaseNotesFile.Value != "" {
		return errors.New("cannot specify both --release-notes and --release-notes-file at the same time")
	}

	// ignore errors when fetching flags
	options := &executor.TaskOptionsCreateRelease{
		ProjectName:             flags.Project.Value,
		DefaultPackageVersion:   flags.PackageVersion.Value,
		PackageVersionOverrides: flags.PackageVersionSpec.Value,
		ChannelName:             flags.Channel.Value,
		GitReference:            flags.GitRef.Value,
		GitCommit:               flags.GitCommit.Value,
		Version:                 flags.Version.Value,
		ReleaseNotes:            flags.ReleaseNotes.Value,
		IgnoreIfAlreadyExists:   flags.IgnoreExisting.Value,
		IgnoreChannelRules:      flags.IgnoreChannelRules.Value,
		GitResourceRefs:         flags.GitResourceRefsSpec.Value,
	}

	if len(flags.CustomFields.Value) > 0 {
		customFields := make(map[string]string)
		for _, raw := range flags.CustomFields.Value {
			// expect first ':' to split name and value; allow value to contain additional ':' characters
			parts := strings.SplitN(raw, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid custom-field value '%s'; expected format name:value", raw)
			}
			name := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if name == "" {
				return fmt.Errorf("invalid custom-field value '%s'; field name cannot be empty", raw)
			}
			customFields[name] = value
		}
		options.CustomFields = customFields
	}

	if flags.ReleaseNotesFile.Value != "" {
		fileContents, err := os.ReadFile(flags.ReleaseNotesFile.Value)
		if err != nil {
			return err
		}
		options.ReleaseNotes = string(fileContents)
	}

	octopus, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	// only populated in automation mode; the interactive Q&A resolves everything as it goes
	var resolvedProject *projects.Project

	if f.IsPromptEnabled() {
		err = AskQuestions(octopus, cmd.OutOrStdout(), f.Ask, options)
		if err != nil {
			return err
		}

		if !constants.IsProgrammaticOutputFormat(outputFormat) {
			// the Q&A process will have modified options;backfill into flags for generation of the automation cmd
			resolvedFlags := NewCreateFlags()
			// deliberately don't include resolvedFlags.PackageVersion in the automation command; it gets converted into PackageVersionSpec
			resolvedFlags.Project.Value = options.ProjectName
			resolvedFlags.PackageVersionSpec.Value = options.PackageVersionOverrides
			resolvedFlags.GitResourceRefsSpec.Value = options.GitResourceRefs
			resolvedFlags.Channel.Value = options.ChannelName
			resolvedFlags.GitRef.Value = options.GitReference
			resolvedFlags.GitCommit.Value = options.GitCommit
			resolvedFlags.Version.Value = options.Version
			resolvedFlags.ReleaseNotes.Value = options.ReleaseNotes
			resolvedFlags.IgnoreExisting.Value = options.IgnoreIfAlreadyExists
			resolvedFlags.IgnoreChannelRules.Value = options.IgnoreChannelRules
			if len(options.CustomFields) > 0 {
				for k, v := range options.CustomFields {
					resolvedFlags.CustomFields.Value = append(resolvedFlags.CustomFields.Value, fmt.Sprintf("%s: %s", k, v))
				}
			}

			spaceName := ""
			if s := f.GetCurrentSpace(); s != nil {
				spaceName = s.GetName()
			}
			autoCmd := flag.GenerateAutomationCmd(constants.ExecutableName+" release create", spaceName,
				resolvedFlags.Project,
				resolvedFlags.GitCommit,
				resolvedFlags.GitRef,
				resolvedFlags.Channel,
				resolvedFlags.ReleaseNotes,
				resolvedFlags.IgnoreExisting,
				resolvedFlags.IgnoreChannelRules,
				resolvedFlags.PackageVersionSpec,
				resolvedFlags.GitResourceRefsSpec,
				resolvedFlags.CustomFields,
				resolvedFlags.Version,
			)
			cmd.Printf("\nAutomation Command: %s\n", autoCmd)
		}
	} else {
		if options.ProjectName != "" {
			project, err := selectors.FindProject(octopus, options.ProjectName)
			if err != nil {
				return err
			}
			options.ProjectName = project.GetName()
			resolvedProject = project
		}
	}

	if flags.DryRun.Value {
		preview, err := buildReleasePreview(octopus, f.GetCurrentSpace(), options, resolvedProject)
		if err != nil {
			return err
		}
		return printReleasePreview(cmd, preview, outputFormat)
	}

	// the executor will raise errors if any required options are missing
	err = executor.ProcessTasks(octopus, f.GetCurrentSpace(), []*executor.Task{
		executor.NewTask(executor.TaskTypeCreateRelease, options),
	})
	if err != nil {
		return err
	}

	if options.Response != nil {
		printReleaseVersion := func(releaseVersion string, assembled time.Time, releaseNotes string, channel *channels.Channel) {
			switch outputFormat {
			case constants.OutputFormatBasic:
				cmd.Printf("%s\n", releaseVersion)
			case constants.OutputFormatJson:
				v := &list.ReleaseViewModel{
					ID:           options.Response.ReleaseID,
					Version:      releaseVersion,
					Assembled:    assembled,
					ReleaseNotes: releaseNotes,
				}
				if channel != nil {
					v.Channel = channel.Name
				}
				data, err := json.Marshal(v)
				if err != nil { // shouldn't happen but fallback in case
					cmd.PrintErrln(err)
				} else {
					_, _ = cmd.OutOrStdout().Write(data)
					cmd.Println()
				}
			default: // table
				if channel != nil {
					cmd.Printf("Successfully created release version %s using channel %s\n", releaseVersion, channel.Name)
				} else {
					cmd.Printf("Successfully created release version %s\n", releaseVersion)
				}
				f.GetServiceMessageProvider().ServiceMessage("setParameter", map[string]string{"name": "octo.releaseNumber", "value": releaseVersion})
			}
		}

		// the API response doesn't tell us what channel it selected, so we need to go look that up to tell the end user
		newlyCreatedRelease, lookupErr := octopus.Releases.GetByID(options.Response.ReleaseID)
		if lookupErr != nil {
			cmd.PrintErrf("Warning: cannot fetch release details: %v\n", lookupErr)
			printReleaseVersion(options.Response.ReleaseVersion, newlyCreatedRelease.Assembled, newlyCreatedRelease.ReleaseNotes, nil)
		} else {
			releaseChan, lookupErr := octopus.Channels.GetByID(newlyCreatedRelease.ChannelID)
			if lookupErr != nil {
				cmd.PrintErrf("Warning: cannot fetch release channel details: %v\n", lookupErr)
				printReleaseVersion(options.Response.ReleaseVersion, newlyCreatedRelease.Assembled, newlyCreatedRelease.ReleaseNotes, nil)
			} else {
				printReleaseVersion(options.Response.ReleaseVersion, newlyCreatedRelease.Assembled, newlyCreatedRelease.ReleaseNotes, releaseChan)
			}
		}

		// output web URL all the time, so long as output format is not JSON or basic
		if err == nil && !constants.IsProgrammaticOutputFormat(outputFormat) {
			link := output.Bluef("%s/app#/%s/releases/%s", f.GetCurrentHost(), f.GetCurrentSpace().ID, options.Response.ReleaseID)
			cmd.Printf("\nView this release on Octopus Deploy: %s\n", link)
		}

		// response also returns AutomaticallyDeployedEnvironments, which was a failed feature; we should ignore it.
	} else {
		cmd.Printf("Error: did not receive valid response from server, cannot output release details")
	}

	return nil
}

// resolveVersioningStrategy loads the project's versioning strategy. Config-as-code projects
// don't inline it in the project resource, so it has to come from the deployment settings.
func resolveVersioningStrategy(octopus *octopusApiClient.Client, project *projects.Project, gitReferenceKey string) (*projects.VersioningStrategy, error) {
	if project.VersioningStrategy != nil {
		return project.VersioningStrategy, nil
	}

	deploymentSettings, err := octopus.Deployments.GetDeploymentSettings(project, gitReferenceKey)
	if err != nil {
		return nil, err
	}
	if deploymentSettings.VersioningStrategy == nil { // not sure if this should ever happen, but best to be defensive
		return nil, cliErrors.NewInvalidResponseError(fmt.Sprintf("cannot determine versioning strategy for project %s", project.Name))
	}
	return deploymentSettings.VersioningStrategy, nil
}

// ReleasePreview is the release that a dry run would have created, resolved as far as the
// CLI can resolve it. An empty Channel or Version means the Octopus Server decides it.
type ReleasePreview struct {
	// always true; it marks machine-readable output as a plan rather than a result
	DryRun             bool
	Space              string
	Project            string
	Channel            string
	GitReference       string `json:",omitempty"`
	GitCommit          string `json:",omitempty"`
	Version            string
	ReleaseNotes       string                         `json:",omitempty"`
	PackageVersions    []*packages.StepPackageVersion `json:",omitempty"`
	PackageOverrides   []string                       `json:",omitempty"`
	GitResources       []string                       `json:",omitempty"`
	CustomFields       map[string]string              `json:",omitempty"`
	IgnoreExisting     bool
	IgnoreChannelRules bool
}

// buildReleasePreview describes the release that would be created, without creating it.
// In interactive mode the Q&A has already resolved everything, so resolvedProject is nil
// and the options are the answer. In automation mode only the project has been resolved,
// so we go back to the server (read-only) for the channel, packages and version.
func buildReleasePreview(octopus *octopusApiClient.Client, space *spaces.Space, options *executor.TaskOptionsCreateRelease, resolvedProject *projects.Project) (*ReleasePreview, error) {
	if options.ProjectName == "" {
		return nil, errors.New("project must be specified")
	}

	preview := &ReleasePreview{
		DryRun:             true,
		Project:            options.ProjectName,
		Channel:            options.ChannelName,
		GitReference:       options.GitReference,
		GitCommit:          options.GitCommit,
		Version:            options.Version,
		ReleaseNotes:       options.ReleaseNotes,
		PackageOverrides:   options.PackageVersionOverrides,
		GitResources:       options.GitResourceRefs,
		CustomFields:       options.CustomFields,
		IgnoreExisting:     options.IgnoreIfAlreadyExists,
		IgnoreChannelRules: options.IgnoreChannelRules,
	}
	if space != nil {
		preview.Space = space.GetName()
	}

	if resolvedProject == nil {
		return preview, nil
	}

	// without an explicit channel the server picks one by applying the channel rules, and
	// both the package versions and the release version follow from that choice; guessing
	// which channel it would pick risks showing a plan that doesn't match what happens
	if options.ChannelName == "" {
		return preview, nil
	}

	channel, err := selectors.FindChannel(octopus, resolvedProject, options.ChannelName)
	if err != nil {
		return nil, err
	}
	preview.Channel = channel.Name

	gitReferenceKey := ""
	if resolvedProject.PersistenceSettings != nil && resolvedProject.PersistenceSettings.Type() == projects.PersistenceSettingsTypeVersionControlled {
		gitReferenceKey = options.GitReference
		if options.GitCommit != "" { // prefer a specific git commit if one was specified
			gitReferenceKey = options.GitCommit
		}
		if gitReferenceKey == "" {
			// DeploymentProcesses.Get falls back to the default branch for us, but
			// GetDeploymentSettings doesn't: for a config-as-code project its link is
			// git-templated, and expanding it without a gitRef produces a bad path. Pin the
			// ref here so the process template and the deployment settings agree, too.
			if gitSettings, ok := resolvedProject.PersistenceSettings.(projects.GitPersistenceSettings); ok {
				gitReferenceKey = gitSettings.DefaultBranch()
			}
		}
	}

	deploymentProcess, err := octopus.DeploymentProcesses.Get(resolvedProject, gitReferenceKey)
	if err != nil {
		return nil, err
	}

	deploymentProcessTemplate, err := octopus.DeploymentProcesses.GetTemplate(deploymentProcess, channel.ID, "")
	if err != nil {
		return nil, err
	}

	baseline, err := BuildPackageVersionBaselineForChannel(octopus, deploymentProcessTemplate, channel)
	if err != nil {
		return nil, err
	}
	overrides := packages.BuildPackageVersionOverrides(baseline, options.DefaultPackageVersion, options.PackageVersionOverrides)
	preview.PackageVersions = packages.ApplyPackageOverrides(baseline, overrides)

	if preview.Version == "" {
		preview.Version, err = determineReleaseVersion(octopus, resolvedProject, gitReferenceKey, deploymentProcessTemplate, preview.PackageVersions)
		if err != nil {
			return nil, err
		}
	}

	return preview, nil
}

// determineReleaseVersion works out the version the server would assign, following the same
// rules as the interactive prompt but without asking anything. An empty string means the
// version can't be determined up front.
func determineReleaseVersion(octopus *octopusApiClient.Client, project *projects.Project, gitReferenceKey string, deploymentProcessTemplate *deployments.DeploymentProcessTemplate, packageVersions []*packages.StepPackageVersion) (string, error) {
	versioningStrategy, err := resolveVersioningStrategy(octopus, project, gitReferenceKey)
	if err != nil {
		return "", err
	}

	if donor := versioningStrategy.DonorPackage; donor != nil {
		for _, pkg := range packageVersions {
			if pkg.PackageReferenceName == donor.PackageReference && pkg.ActionName == donor.DeploymentAction {
				return pkg.Version, nil
			}
		}
		return "", nil
	}
	if versioningStrategy.DonorPackageStepID != nil { // a donor step with no package reference; nothing to read a version from
		return "", nil
	}

	if versioningStrategy.Template != "" {
		return deploymentProcessTemplate.NextVersionIncrement, nil
	}

	return "", nil
}

func printReleasePreview(cmd *cobra.Command, preview *ReleasePreview, outputFormat string) error {
	if outputFormat == constants.OutputFormatJson {
		data, err := json.Marshal(preview)
		if err != nil {
			return err
		}
		_, _ = cmd.OutOrStdout().Write(data)
		cmd.Println()
		return nil
	}

	dryrun.Header(cmd)
	cmd.Printf("Would create a release with:\n")

	byServer := output.Dim("(determined by the Octopus Server)")
	rows := []*output.DataRow{
		output.NewDataRow("Space", preview.Space),
		output.NewDataRow("Project", preview.Project),
		output.NewDataRow("Channel", orDefault(preview.Channel, byServer)),
		output.NewDataRow("Version", orDefault(preview.Version, byServer)),
	}
	if preview.GitReference != "" {
		rows = append(rows, output.NewDataRow("Git Reference", preview.GitReference))
	}
	if preview.GitCommit != "" {
		rows = append(rows, output.NewDataRow("Git Commit", preview.GitCommit))
	}
	rows = append(rows, output.NewDataRow("Release Notes", orDefault(preview.ReleaseNotes, output.Dim("(none)"))))
	for _, ref := range preview.GitResources {
		rows = append(rows, output.NewDataRow("Git Resource", ref))
	}
	for name, value := range preview.CustomFields {
		rows = append(rows, output.NewDataRow("Custom Field", fmt.Sprintf("%s: %s", name, value)))
	}
	if preview.IgnoreExisting {
		rows = append(rows, output.NewDataRow("Ignore Existing", "true"))
	}
	if preview.IgnoreChannelRules {
		rows = append(rows, output.NewDataRow("Ignore Channel Rules", "true"))
	}
	output.PrintRows(rows, cmd.OutOrStdout())

	if len(preview.PackageVersions) > 0 {
		cmd.Printf("\nPackages:\n")
		t := output.NewTable(cmd.OutOrStdout())
		t.AddRow(output.Bold("PACKAGE"), output.Bold("VERSION"), output.Bold("STEP NAME/PACKAGE REFERENCE"))
		for _, pkg := range preview.PackageVersions {
			t.AddRow(pkg.PackageID, orDefault(pkg.Version, output.Yellow("unknown")), fmt.Sprintf("%s/%s", pkg.ActionName, pkg.PackageReferenceName))
		}
		if err := t.Print(); err != nil {
			return err
		}
	} else if len(preview.PackageOverrides) > 0 {
		cmd.Printf("\nPackage overrides:\n")
		for _, ov := range preview.PackageOverrides {
			cmd.Printf("  %s\n", ov)
		}
	}

	dryrun.Footer(cmd, "no release was created.")
	return nil
}

func orDefault(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// BuildPackageVersionBaselineForChannel loads the deployment process template from the server, and for each step+package therein,
// finds the latest available version satisfying the channel version rules. Result is the list of step+package+versions
// to use as a baseline. The package version override process takes this as an input and layers on top of it
func BuildPackageVersionBaselineForChannel(octopus *octopusApiClient.Client, deploymentProcessTemplate *deployments.DeploymentProcessTemplate, channel *channels.Channel) ([]*packages.StepPackageVersion, error) {

	result, err := packages.BuildPackageVersionBaseline(octopus, deploymentProcessTemplate.Packages, func(packageRef releases.ReleaseTemplatePackage, query feeds.SearchPackageVersionsQuery) (feeds.SearchPackageVersionsQuery, error) {
		// look in the channel rules for a version filter for this step+package

	rulesLoop:
		for _, rule := range channel.Rules {
			for _, ap := range rule.ActionPackages {
				if ap.PackageReference == packageRef.PackageReferenceName && ap.DeploymentAction == packageRef.ActionName {
					// this rule applies to our step/packageref combo.
					// VersionRange, Tag/pre-release and VersionTagRegex are always applied together,
					// regardless of strategy; VersioningStrategy only changes ordering (publish-date
					// vs SemVer), not which versions satisfy the rule. (empty fields are dropped by
					// the query's omitempty uri tags)
					query.PreReleaseTag = rule.Tag
					query.VersionRange = rule.VersionRange
					query.VersionTagRegex = rule.VersionTagRegex
					query.VersioningStrategy = rule.VersioningStrategy
					// the octopus server won't let the same package be targeted by more than one rule, so
					// once we've found the first matching rule for our step+package, we can stop looping
					break rulesLoop
				}
			}
		}

		return query, nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func AskQuestions(octopus *octopusApiClient.Client, stdout io.Writer, asker question.Asker, options *executor.TaskOptionsCreateRelease) error {
	if octopus == nil {
		return cliErrors.NewArgumentNullOrEmptyError("octopus")
	}
	if asker == nil {
		return cliErrors.NewArgumentNullOrEmptyError("asker")
	}
	if options == nil {
		return cliErrors.NewArgumentNullOrEmptyError("options")
	}
	// Note: we don't get here at all if no-prompt is enabled, so we know we are free to ask questions

	// Note on output: survey prints things; if the option is specified already from the command line,
	// we should emulate that so there is always a line where you can see what the item was when specified on the command line,
	// however if we support a "quiet mode" then we shouldn't emit those

	selectedProject, err := selectors.ResolveProject(octopus, asker, true,
		"Select the project in which the release will be created", options.ProjectName)
	if err != nil {
		return err
	}
	if options.ProjectName != "" { // project name was already provided; echo it so the choice is always visible
		_, _ = fmt.Fprintf(stdout, "Project %s\n", output.Cyan(selectedProject.Name))
	}
	options.ProjectName = selectedProject.Name

	// we always need the deployment process, so we can prompt for package version overrides (or know that there aren't any packages, so it doesn't matter)

	var gitReferenceKey string
	if selectedProject.PersistenceSettings.Type() == projects.PersistenceSettingsTypeVersionControlled {
		// if there is no git reference specified, ask the server for the list and prompt.
		// we leave GitCommit alone in interactive mode; we don't prompt, but if it was specified on the
		// commandline we just pass it through untouched.

		if options.GitReference == "" { // we need a git ref; ask for one
			gitRef, err := selectGitReference(octopus, asker, selectedProject)
			if err != nil {
				return err
			}
			options.GitReference = gitRef.CanonicalName // e.g /refs/heads/main
		} else {
			// we need to go lookup the git reference
			_, _ = fmt.Fprintf(stdout, "Git Reference %s\n", output.Cyan(options.GitReference))
		}

		// we could go and query the server and validate if the commit exists, is this worthwhile?
		if options.GitCommit != "" {
			_, _ = fmt.Fprintf(stdout, "Git Commit %s\n", output.Cyan(options.GitCommit))
		}

		if options.GitCommit != "" { // prefer a specific git commit if one was specified
			gitReferenceKey = options.GitCommit
		} else {
			gitReferenceKey = options.GitReference
		}

	} else {
		// normal projects just have one deployment process, load that instead
		gitReferenceKey = ""
	}

	// we've figured out how to load the dep process; go load it
	deploymentProcess, err := octopus.DeploymentProcesses.Get(selectedProject, gitReferenceKey)
	if err != nil {
		return err
	}

	var selectedChannel *channels.Channel
	if options.ChannelName == "" {
		selectedChannel, err = selectors.Channel(octopus, asker, stdout, "Select the channel in which the release will be created", selectedProject)
		if err != nil {
			return err
		}
	} else {
		selectedChannel, err = selectors.FindChannel(octopus, selectedProject, options.ChannelName)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "Channel %s\n", output.Cyan(selectedChannel.Name))
	}
	options.ChannelName = selectedChannel.Name

	// immediately load the deployment process template
	// we need the deployment process template in order to get the steps, so we can lookup the stepID
	deploymentProcessTemplate, err := octopus.DeploymentProcesses.GetTemplate(deploymentProcess, selectedChannel.ID, "")
	// don't stop the spinner, BuildPackageVersionBaseline does more networking
	if err != nil {
		return err
	}

	packageVersionBaseline, err := BuildPackageVersionBaselineForChannel(octopus, deploymentProcessTemplate, selectedChannel)
	if err != nil {
		return err
	}

	var overriddenPackageVersions []*packages.StepPackageVersion
	if len(packageVersionBaseline) > 0 { // if we have packages, run the package flow
		opv, packageVersionOverrides, err := packages.AskPackageOverrideLoop(
			packageVersionBaseline,
			options.DefaultPackageVersion,
			options.PackageVersionOverrides,
			asker,
			stdout)

		if err != nil {
			return err
		}
		overriddenPackageVersions = opv

		if len(packageVersionOverrides) > 0 {
			options.PackageVersionOverrides = make([]string, 0, len(packageVersionOverrides))
			for _, ov := range packageVersionOverrides {
				options.PackageVersionOverrides = append(options.PackageVersionOverrides, ov.ToPackageOverrideString())
			}
		}
	} else {
		overriddenPackageVersions = packageVersionBaseline // there aren't any, but satisfy the code below anyway
	}

	gitResourcesBaseline := gitresources.BuildGitResourcesBaseline(deploymentProcessTemplate.GitResources)

	if len(gitResourcesBaseline) > 0 {
		overriddenGitResources, err := gitresources.AskGitResourceOverrideLoop(
			gitResourcesBaseline,
			options.GitResourceRefs,
			asker,
			stdout)

		if err != nil {
			return err
		}

		if len(overriddenGitResources) > 0 {
			options.GitResourceRefs = make([]string, 0, len(overriddenGitResources))
			for _, ov := range overriddenGitResources {
				options.GitResourceRefs = append(options.GitResourceRefs, ov.ToGitResourceGitRefString())
			}
		}
	}

	if options.Version == "" {
		// After loading the deployment process and channel, the logic forks here:
		// If the project's VersioningStrategy has a Template then we need to look in the deploymentprocesstemplate for the next release version
		// If the project's VersioningStrategy has a DonorPackageStepId then we need to follow the package trail to determine the release version
		// - but we must allow the user to override package versions first.
		// If the project's VersioningStrategy is null, it means this is a Config-as-code project and we need to
		// additionally load the deployment settings because the API doesn't inline the strategy in the main project resource for some reason
		versioningStrategy, err := resolveVersioningStrategy(octopus, selectedProject, gitReferenceKey)
		if err != nil {
			return err
		}

		if versioningStrategy.DonorPackageStepID != nil || versioningStrategy.DonorPackage != nil {
			// we've already done the package version work so we can just ask the donor package which version it has selected
			var donorPackage *packages.StepPackageVersion
			for _, pkg := range overriddenPackageVersions {
				if pkg.PackageReferenceName == versioningStrategy.DonorPackage.PackageReference && pkg.ActionName == versioningStrategy.DonorPackage.DeploymentAction {
					donorPackage = pkg
					break
				}
			}
			if donorPackage == nil {
				// should this just be a warning rather than a hard fail? we could still just ask the user if they'd
				// like to type in a version or leave it blank? On the other hand, it shouldn't fail anyway :shrug:
				return fmt.Errorf("internal error: can't find donor package in deployment process template - version controlled configuration file in an invalid state")
			}

			versionMetadata, err := askVersionMetadata(asker, donorPackage.PackageID, donorPackage.Version)
			if err != nil {
				return err
			}
			if versionMetadata == "" {
				options.Version = donorPackage.Version
			} else {
				options.Version = fmt.Sprintf("%s+%s", donorPackage.Version, versionMetadata)
			}
		} else if versioningStrategy.Template != "" {
			// we already loaded the deployment process template when we were looking for packages
			options.Version, err = askVersion(asker, deploymentProcessTemplate.NextVersionIncrement)
			if err != nil {
				return err
			}
		}

	} else {
		_, _ = fmt.Fprintf(stdout, "Version %s\n", output.Cyan(options.Version))
	}

	if options.ReleaseNotes == "" {
		options.ReleaseNotes, err = askReleaseNotes(asker)
		if err != nil {
			return err
		}
	}

	if len(selectedChannel.CustomFieldDefinitions) > 0 {
		if options.CustomFields == nil { // ensure map initialised
			options.CustomFields = make(map[string]string, len(selectedChannel.CustomFieldDefinitions))
		}
		for _, customFieldDefinition := range selectedChannel.CustomFieldDefinitions {
			// skip if already provided via automation
			if _, exists := options.CustomFields[customFieldDefinition.FieldName]; exists {
				continue
			}

			customFieldValue, err := askCustomField(customFieldDefinition, asker)
			if err != nil {
				return err
			}
			options.CustomFields[customFieldDefinition.FieldName] = customFieldValue
		}
	}

	return nil
}

func askCustomField(customFieldDefinition channels.ChannelCustomFieldDefinition, asker question.Asker) (string, error) {
	msg := fmt.Sprint(customFieldDefinition.FieldName)
	helpText := customFieldDefinition.Description
	var answer string

	validator := func(val interface{}) error {
		str, _ := val.(string)
		if strings.TrimSpace(str) == "" {
			return fmt.Errorf("%s is required", customFieldDefinition.FieldName)
		}
		return nil
	}
	if err := asker(&survey.Input{Message: msg, Help: helpText}, &answer, survey.WithValidator(validator)); err != nil {
		return "", err
	}
	return answer, nil
}

func askVersion(ask question.Asker, defaultVersion string) (string, error) {
	var result string
	if err := ask(&survey.Input{
		Default: defaultVersion,
		Message: "Release Version",
	}, &result); err != nil {
		return "", err
	}
	return result, nil
}

func askVersionMetadata(ask question.Asker, packageId string, packageVersion string) (string, error) {
	var result string
	if err := ask(&survey.Input{
		Default: "",
		Message: fmt.Sprintf("Release version %s (from included package %s). Add metadata? (optional):", packageVersion, packageId),
	}, &result); err != nil {
		return "", err
	}
	return result, nil
}

func askReleaseNotes(ask question.Asker) (string, error) {
	var result string
	if err := ask(&surveyext.OctoEditor{
		Editor: &survey.Editor{
			Message:  "Release Notes",
			Help:     "You may optionally add notes to the release using Markdown.",
			FileName: "*.md",
		},
		Optional: true,
	}, &result); err != nil {
		return "", err
	}
	return result, nil
}

func selectGitReference(octopus *octopusApiClient.Client, ask question.Asker, project *projects.Project) (*projects.GitReference, error) {
	branches, err := octopus.Projects.GetGitBranches(project)
	if err != nil {
		return nil, err
	}

	tags, err := octopus.Projects.GetGitTags(project)

	if err != nil {
		return nil, err
	}

	allRefs := append(branches, tags...)

	return question.SelectMap(ask, "Select the Git Reference to use", allRefs, func(g *projects.GitReference) string {
		return fmt.Sprintf("%s %s", g.Name, output.Dimf("(%s)", g.Type.Description()))
	})
}
