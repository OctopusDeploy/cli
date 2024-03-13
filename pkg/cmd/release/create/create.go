package create

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/apiclient"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/release/list"
	"github.com/OctopusDeploy/cli/pkg/constants"
	cliErrors "github.com/OctopusDeploy/cli/pkg/errors"
	"github.com/OctopusDeploy/cli/pkg/executionscommon"
	"github.com/OctopusDeploy/cli/pkg/executor"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
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
	"github.com/spf13/cobra"
)

const (
	FlagProject            = "project"
	FlagChannel            = "channel"
	FlagPackageVersionSpec = "package"

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
	Project            *flag.Flag[string]
	Channel            *flag.Flag[string]
	GitRef             *flag.Flag[string]
	GitCommit          *flag.Flag[string]
	PackageVersion     *flag.Flag[string]
	ReleaseNotes       *flag.Flag[string]
	ReleaseNotesFile   *flag.Flag[string]
	Version            *flag.Flag[string]
	IgnoreExisting     *flag.Flag[bool]
	IgnoreChannelRules *flag.Flag[bool]
	PackageVersionSpec *flag.Flag[[]string]
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Project:            flag.New[string](FlagProject, false),
		Channel:            flag.New[string](FlagChannel, false),
		GitRef:             flag.New[string](FlagGitRef, false),
		GitCommit:          flag.New[string](FlagGitCommit, false),
		PackageVersion:     flag.New[string](FlagPackageVersion, false),
		ReleaseNotes:       flag.New[string](FlagReleaseNotes, false),
		ReleaseNotesFile:   flag.New[string](FlagReleaseNotesFile, false),
		Version:            flag.New[string](FlagVersion, false),
		IgnoreExisting:     flag.New[bool](FlagIgnoreExisting, false),
		IgnoreChannelRules: flag.New[bool](FlagIgnoreChannelRules, false),
		PackageVersionSpec: flag.New[[]string](FlagPackageVersionSpec, false),
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a release",
		Long:  "Create a release in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s release create --project MyProject --channel Beta --version 1.2.3
			$ %[1]s release create -p MyProject -c Beta -v 1.2.3
			$ %[1]s release create -p MyProject -c default --package "utils:1.2.3" --package "utils:InstallOnly:5.6.7"
			$ %[1]s release create -p MyProject -c Beta --no-prompt
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
	flags.StringArrayVarP(&createFlags.PackageVersionSpec.Value, createFlags.PackageVersionSpec.Name, "", []string{}, "Version specification a specific packages.\nFormat as {package}:{version}, {step}:{version} or {package-ref-name}:{packageOrStep}:{version}\nYou may specify this multiple times")

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
			resolvedFlags.Channel.Value = options.ChannelName
			resolvedFlags.GitRef.Value = options.GitReference
			resolvedFlags.GitCommit.Value = options.GitCommit
			resolvedFlags.Version.Value = options.Version
			resolvedFlags.ReleaseNotes.Value = options.ReleaseNotes
			resolvedFlags.IgnoreExisting.Value = options.IgnoreIfAlreadyExists
			resolvedFlags.IgnoreChannelRules.Value = options.IgnoreChannelRules

			autoCmd := flag.GenerateAutomationCmd(constants.ExecutableName+" release create",
				resolvedFlags.Project,
				resolvedFlags.GitCommit,
				resolvedFlags.GitRef,
				resolvedFlags.Channel,
				resolvedFlags.ReleaseNotes,
				resolvedFlags.IgnoreExisting,
				resolvedFlags.IgnoreChannelRules,
				resolvedFlags.PackageVersionSpec,
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
		}
	}

	// the executor will raise errors if any required options are missing
	err = executor.ProcessTasks(octopus, f.GetCurrentSpace(), []*executor.Task{
		executor.NewTask(executor.TaskTypeCreateRelease, options),
	})
	if err != nil {
		return err
	}

	if options.Response != nil {
		printReleaseVersion := func(releaseVersion string, channel *channels.Channel) {
			switch outputFormat {
			case constants.OutputFormatBasic:
				cmd.Printf("%s\n", releaseVersion)
			case constants.OutputFormatJson:
				v := &list.ReleaseViewModel{Version: releaseVersion}
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
			}
		}

		// the API response doesn't tell us what channel it selected, so we need to go look that up to tell the end user
		newlyCreatedRelease, lookupErr := octopus.Releases.GetByID(options.Response.ReleaseID)
		if lookupErr != nil {
			cmd.PrintErrf("Warning: cannot fetch release details: %v\n", lookupErr)
			printReleaseVersion(options.Response.ReleaseVersion, nil)
		} else {
			releaseChan, lookupErr := octopus.Channels.GetByID(newlyCreatedRelease.ChannelID)
			if lookupErr != nil {
				cmd.PrintErrf("Warning: cannot fetch release channel details: %v\n", lookupErr)
				printReleaseVersion(options.Response.ReleaseVersion, nil)
			} else {
				printReleaseVersion(options.Response.ReleaseVersion, releaseChan)
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

// BuildPackageVersionBaseline loads the deployment process template from the server, and for each step+package therein,
// finds the latest available version satisfying the channel version rules. Result is the list of step+package+versions
// to use as a baseline. The package version override process takes this as an input and layers on top of it
func BuildPackageVersionBaseline(octopus *octopusApiClient.Client, deploymentProcessTemplate *deployments.DeploymentProcessTemplate, channel *channels.Channel) ([]*executionscommon.StepPackageVersion, error) {
	result := make([]*executionscommon.StepPackageVersion, 0, len(deploymentProcessTemplate.Packages))

	// step 1: pass over all the packages in the deployment process, group them
	// by their feed, then subgroup by packageId

	// map(key: FeedID, value: list of references using the package so we can trace back to steps)
	feedsToQuery := make(map[string][]releases.ReleaseTemplatePackage)
	for _, pkg := range deploymentProcessTemplate.Packages {

		// If a package is not considered resolvable by the server, don't attempt to query it's feed or lookup
		// any potential versions for it; we can't succeed in that because variable templates won't get expanded
		// until deployment time
		if !pkg.IsResolvable {
			result = append(result, &executionscommon.StepPackageVersion{
				PackageID:            pkg.PackageID,
				ActionName:           pkg.ActionName,
				PackageReferenceName: pkg.PackageReferenceName,
				Version:              "",
			})
			continue
		}
		if feedPackages, seenFeedBefore := feedsToQuery[pkg.FeedID]; !seenFeedBefore {
			feedsToQuery[pkg.FeedID] = []releases.ReleaseTemplatePackage{pkg}
		} else {
			// seen both the feed and package, but not against this particular step
			feedsToQuery[pkg.FeedID] = append(feedPackages, pkg)
		}
	}

	if len(feedsToQuery) == 0 {
		return make([]*executionscommon.StepPackageVersion, 0), nil
	}

	// step 2: load the feed resources, so we can get SearchPackageVersionsTemplate
	feedIds := make([]string, 0, len(feedsToQuery))
	for k := range feedsToQuery {
		feedIds = append(feedIds, k)
	}
	sort.Strings(feedIds) // we need to sort them otherwise the order is indeterminate. Server doesn't care but our unit tests fail
	foundFeeds, err := octopus.Feeds.Get(feeds.FeedsQuery{IDs: feedIds, Take: len(feedIds)})
	if err != nil {
		return nil, err
	}

	// step 3: for each package within a feed, ask the server to select the best package version for it, applying the channel rules
	for _, feed := range foundFeeds.Items {
		packageRefsInFeed, ok := feedsToQuery[feed.GetID()]
		if !ok {
			return nil, errors.New("internal consistency error; feed ID not found in feedsToQuery") // should never happen
		}

		cache := make(map[feeds.SearchPackageVersionsQuery]string) // cache value is the package version

		for _, packageRef := range packageRefsInFeed {
			query := feeds.SearchPackageVersionsQuery{
				PackageID: packageRef.PackageID,
				Take:      1,
			}
			// look in the channel rules for a version filter for this step+package
		rulesLoop:
			for _, rule := range channel.Rules {
				for _, ap := range rule.ActionPackages {
					if ap.PackageReference == packageRef.PackageReferenceName && ap.DeploymentAction == packageRef.ActionName {
						// this rule applies to our step/packageref combo
						query.PreReleaseTag = rule.Tag
						query.VersionRange = rule.VersionRange
						// the octopus server won't let the same package be targeted by more than one rule, so
						// once we've found the first matching rule for our step+package, we can stop looping
						break rulesLoop
					}
				}
			}

			if cachedVersion, ok := cache[query]; ok {
				result = append(result, &executionscommon.StepPackageVersion{
					PackageID:            packageRef.PackageID,
					ActionName:           packageRef.ActionName,
					PackageReferenceName: packageRef.PackageReferenceName,
					Version:              cachedVersion,
				})
			} else { // uncached; ask the server
				versions, err := octopus.Feeds.SearchFeedPackageVersions(feed, query)
				if err != nil {
					return nil, err
				}

				switch len(versions.Items) {
				case 0: // no package found; cache the response
					cache[query] = ""
					result = append(result, &executionscommon.StepPackageVersion{
						PackageID:            packageRef.PackageID,
						ActionName:           packageRef.ActionName,
						PackageReferenceName: packageRef.PackageReferenceName,
						Version:              "",
					})

				case 1:
					cache[query] = versions.Items[0].Version
					result = append(result, &executionscommon.StepPackageVersion{
						PackageID:            packageRef.PackageID,
						ActionName:           packageRef.ActionName,
						PackageReferenceName: packageRef.PackageReferenceName,
						Version:              versions.Items[0].Version,
					})

				default:
					return nil, errors.New("internal error; more than one package returned when only 1 specified")
				}
			}
		}
	}
	return result, nil
}

type PackageVersionOverride struct {
	ActionName           string // optional, but one or both of ActionName or PackageID must be supplied
	PackageID            string // optional, but one or both of ActionName or PackageID must be supplied
	PackageReferenceName string // optional; use for advanced situations where the same package is referenced multiple times by a single step
	Version              string // required
}

// ToPackageOverrideString converts the struct back into a string which the server can parse e.g. StepName:Version.
// This is the inverse of ParsePackageOverrideString
func (p *PackageVersionOverride) ToPackageOverrideString() string {
	components := make([]string, 0, 3)

	// stepNameOrPackageID always comes first if we have it
	if p.PackageID != "" {
		components = append(components, p.PackageID)
	} else if p.ActionName != "" { // can't have both PackageID and ActionName; PackageID wins
		components = append(components, p.ActionName)
	}

	// followed by package reference name if we have it
	if p.PackageReferenceName != "" {
		if len(components) == 0 { // if we have an explicit packagereference but no packageId or action, we need to express it with *:ref:version
			components = append(components, "*")
		}
		components = append(components, p.PackageReferenceName)
	}

	if len(components) == 0 { // the server can't deal with just a number by itself; if we want to override everything we must pass *:Version
		components = append(components, "*")
	}
	components = append(components, p.Version)

	return strings.Join(components, ":")
}

// splitPackageOverrideString splits the input string into components based on delimiter characters.
// we want to pick up empty entries here; so "::5" and ":pterm:5" should both return THREE components, rather than one or two
// and we want to allow for multiple different delimeters.
// neither the builtin golang strings.Split or strings.FieldsFunc support this. Logic borrowed from strings.FieldsFunc with heavy modifications
func splitPackageOverrideString(s string) []string {
	// pass 1: collect spans; golang strings.FieldsFunc says it's much more efficient this way
	type span struct {
		start int
		end   int
	}
	spans := make([]span, 0, 3)

	// Find the field start and end indices.
	start := 0 // we always start the first span at the beginning of the string
	for idx, ch := range s {
		if ch == ':' || ch == '/' || ch == '=' {
			if start >= 0 { // we found a delimiter and we are already in a span; end the span and start a new one
				spans = append(spans, span{start, idx})
				start = idx + 1
			} else { // we found a delimiter and we are not in a span; start a new span
				if start < 0 {
					start = idx
				}
			}
		}
	}

	// Last field might end at EOF.
	if start >= 0 {
		spans = append(spans, span{start, len(s)})
	}

	// pass 2: create strings from recorded field indices.
	a := make([]string, len(spans))
	for i, span := range spans {
		a[i] = s[span.start:span.end]
	}
	return a
}

// AmbiguousPackageVersionOverride tells us that we want to set the version of some package to `Version`
// but it's not clear whether ActionNameOrPackageID refers to an ActionName or PackageID at this point
type AmbiguousPackageVersionOverride struct {
	ActionNameOrPackageID string
	PackageReferenceName  string
	Version               string
}

// taken from here https://github.com/OctopusDeploy/Versioning/blob/main/source/Octopus.Versioning/Octopus/OctopusVersionParser.cs#L29
// but simplified, and removed the support for optional whitespace around version numbers (OctopusVersion would allow "1 . 2 . 3" whereas we won't
// otherwise this is very lenient
var validVersionRegex, _ = regexp.Compile("(?i)" + `^\s*(v|V)?\d+(\.\d+)?(\.\d+)?(\.\d+)?[.\-_\\]?([a-z0-9]*?)([.\-_\\]([a-z0-9.\-_\\]*?)?)?(\+([a-z0-9_\-.\\+]*?))?$`)

func isValidVersion(version string) bool {
	return validVersionRegex.MatchString(version)
}

// ParsePackageOverrideString parses a package version override string into a structure.
// Logic should align with PackageVersionResolver in the Octopus Server and .NET CLI
// In cases where things are ambiguous, we look in steps for matching values to see if something is a PackageID or a StepName
func ParsePackageOverrideString(packageOverride string) (*AmbiguousPackageVersionOverride, error) {
	if packageOverride == "" {
		return nil, errors.New("empty package version specification")
	}

	components := splitPackageOverrideString(packageOverride)
	packageReferenceName, stepNameOrPackageID, version := "", "", ""

	switch len(components) {
	case 2:
		// if there are two components it is (StepName|PackageID):Version
		stepNameOrPackageID, version = strings.TrimSpace(components[0]), strings.TrimSpace(components[1])
	case 3:
		// if there are three components it is (StepName|PackageID):PackageReferenceName:Version
		stepNameOrPackageID, packageReferenceName, version = strings.TrimSpace(components[0]), strings.TrimSpace(components[1]), strings.TrimSpace(components[2])
	default:
		return nil, fmt.Errorf("package version specification \"%s\" does not use expected format", packageOverride)
	}

	// must always specify a version; must specify either packageID, stepName or both
	if version == "" {
		return nil, fmt.Errorf("package version specification \"%s\" does not use expected format", packageOverride)
	}
	if !isValidVersion(version) {
		return nil, fmt.Errorf("version component \"%s\" is not a valid version", version)
	}

	// compensate for wildcards
	if packageReferenceName == "*" {
		packageReferenceName = ""
	}
	if stepNameOrPackageID == "*" {
		stepNameOrPackageID = ""
	}

	return &AmbiguousPackageVersionOverride{
		ActionNameOrPackageID: stepNameOrPackageID,
		PackageReferenceName:  packageReferenceName,
		Version:               version,
	}, nil
}

func ResolvePackageOverride(override *AmbiguousPackageVersionOverride, steps []*executionscommon.StepPackageVersion) (*PackageVersionOverride, error) {
	// shortcut for wildcard matches; these match everything so we don't need to do any work
	if override.PackageReferenceName == "" && override.ActionNameOrPackageID == "" {
		return &PackageVersionOverride{
			ActionName:           "",
			PackageID:            "",
			PackageReferenceName: "",
			Version:              override.Version,
		}, nil
	}

	actionNameOrPackageID := override.ActionNameOrPackageID

	// it could be either a stepname or a package ID; match against the list of packages to try and guess.
	// logic matching the server:
	//  - exact match on stepName + refName
	//  - then exact match on packageId + refName
	//  - then match on * + refName
	//  - then match on stepName + *
	//  - then match on packageID + *
	type match struct {
		priority             int
		actionName           string // if set we matched on actionName, else we didn't
		packageID            string // if set we matched on packageID, else we didn't
		packageReferenceName string // if set we matched on packageReferenceName, else we didn't
	}

	matches := make([]match, 0, 2) // common case is likely to be 2; if we have a packageID then we may match both exactly and partially on the ID depending on referenceName
	for _, p := range steps {
		if p.ActionName != "" && p.ActionName == actionNameOrPackageID {
			if p.PackageReferenceName == override.PackageReferenceName {
				matches = append(matches, match{priority: 100, actionName: p.ActionName, packageReferenceName: p.PackageReferenceName})
			} else {
				matches = append(matches, match{priority: 50, actionName: p.ActionName})
			}
		} else if p.PackageID != "" && p.PackageID == actionNameOrPackageID {
			if p.PackageReferenceName == override.PackageReferenceName {
				matches = append(matches, match{priority: 90, packageID: p.PackageID, packageReferenceName: p.PackageReferenceName})
			} else {
				matches = append(matches, match{priority: 40, packageID: p.PackageID})
			}
		} else if p.PackageReferenceName != "" && p.PackageReferenceName == override.PackageReferenceName {
			matches = append(matches, match{priority: 80, packageReferenceName: p.PackageReferenceName})
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("could not resolve step name or package matching %s", actionNameOrPackageID)
	}
	sort.SliceStable(matches, func(i, j int) bool { // want a stable sort so if there's more than one possible match we pick the first one
		return matches[i].priority > matches[j].priority
	})

	return &PackageVersionOverride{
		ActionName:           matches[0].actionName,
		PackageID:            matches[0].packageID,
		PackageReferenceName: matches[0].packageReferenceName,
		Version:              override.Version,
	}, nil
}

func ApplyPackageOverrides(packages []*executionscommon.StepPackageVersion, overrides []*PackageVersionOverride) []*executionscommon.StepPackageVersion {
	for _, o := range overrides {
		packages = applyPackageOverride(packages, o)
	}
	return packages
}

func applyPackageOverride(packages []*executionscommon.StepPackageVersion, override *PackageVersionOverride) []*executionscommon.StepPackageVersion {
	if override.Version == "" {
		return packages // not specifying a version is technically an error, but we'll just no-op it for safety; should have been filtered out by ParsePackageOverrideString before we get here
	}

	var matcher func(pkg *executionscommon.StepPackageVersion) bool = nil

	switch {
	case override.PackageID == "" && override.ActionName == "": // match everything
		matcher = func(pkg *executionscommon.StepPackageVersion) bool {
			return true
		}
	case override.PackageID != "" && override.ActionName == "": // match on package ID only
		matcher = func(pkg *executionscommon.StepPackageVersion) bool {
			return pkg.PackageID == override.PackageID
		}
	case override.PackageID == "" && override.ActionName != "": // match on step only
		matcher = func(pkg *executionscommon.StepPackageVersion) bool {
			return pkg.ActionName == override.ActionName
		}
	case override.PackageID != "" && override.ActionName != "": // match on both; shouldn't be possible but let's ensure it works anyway
		matcher = func(pkg *executionscommon.StepPackageVersion) bool {
			return pkg.PackageID == override.PackageID && pkg.ActionName == override.ActionName
		}
	}

	if override.PackageReferenceName != "" { // must also match package reference name
		if matcher == nil {
			matcher = func(pkg *executionscommon.StepPackageVersion) bool {
				return pkg.PackageReferenceName == override.PackageReferenceName
			}
		} else {
			prevMatcher := matcher
			matcher = func(pkg *executionscommon.StepPackageVersion) bool {
				return pkg.PackageReferenceName == override.PackageReferenceName && prevMatcher(pkg)
			}
		}
	}

	if matcher == nil {
		return packages // we can't possibly match against anything; no-op. Should have been filtered out by ParsePackageOverrideString
	}

	result := make([]*executionscommon.StepPackageVersion, len(packages))
	for i, p := range packages {
		if matcher(p) {
			result[i] = &executionscommon.StepPackageVersion{
				PackageID:            p.PackageID,
				ActionName:           p.ActionName,
				PackageReferenceName: p.PackageReferenceName,
				Version:              override.Version, // Important bit
			}
		} else {
			result[i] = p
		}
	}
	return result
}

// Note this always uses the Table Printer, it pays no respect to outputformat=json, because it's only part of the interactive flow
func printPackageVersions(ioWriter io.Writer, packages []*executionscommon.StepPackageVersion) error {
	// step 1: consolidate multiple rows
	consolidated := make([]*executionscommon.StepPackageVersion, 0, len(packages))
	for _, pkg := range packages {

		// the common case is that packageReferenceName will be equal to PackageID.
		// however, in advanced cases it may not be, so we need to do extra work to show the packageReferenceName too.
		// we suffix it onto the step name, following the web UI
		qualifiedPkgActionName := pkg.ActionName
		if pkg.PackageID != pkg.PackageReferenceName {
			//qualifiedPkgActionName = fmt.Sprintf("%s%s", qualifiedPkgActionName, output.Yellowf("/%s", pkg.PackageReferenceName))
			qualifiedPkgActionName = fmt.Sprintf("%s/%s", qualifiedPkgActionName, pkg.PackageReferenceName)
		} else {
			qualifiedPkgActionName = fmt.Sprintf("%s%s", qualifiedPkgActionName, output.Dimf("/%s", pkg.PackageReferenceName))
		}

		// find existing entry and insert row below it
		updatedExisting := false
		for index, entry := range consolidated {
			if entry.PackageID == pkg.PackageID && entry.Version == pkg.Version {
				consolidated = append(consolidated[:index+2], consolidated[index+1:]...)
				consolidated[index+1] = &executionscommon.StepPackageVersion{
					PackageID:  output.Dim(pkg.PackageID),
					Version:    output.Dim(pkg.Version),
					ActionName: qualifiedPkgActionName,
				}
				updatedExisting = true
				break
			}
		}
		if !updatedExisting {
			consolidated = append(consolidated, &executionscommon.StepPackageVersion{
				PackageID:  pkg.PackageID,
				Version:    pkg.Version,
				ActionName: qualifiedPkgActionName,
			})
		}
	}

	// step 2: print them
	t := output.NewTable(ioWriter)
	t.AddRow(
		output.Bold("PACKAGE"),
		output.Bold("VERSION"),
		output.Bold("STEP NAME/PACKAGE REFERENCE"),
	)
	//t.AddRow(
	//	"-------",
	//	"-------",
	//	"---------------------------",
	//)

	for _, pkg := range consolidated {
		version := pkg.Version
		if version == "" {
			version = output.Yellow("unknown") // can't determine version for this package
		}
		t.AddRow(
			pkg.PackageID,
			version,
			pkg.ActionName,
		)
	}

	return t.Print()
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

	var err error
	var selectedProject *projects.Project
	if options.ProjectName == "" {
		selectedProject, err = selectors.Project("Select the project in which the release will be created", octopus, asker)
		if err != nil {
			return err
		}
	} else { // project name is already provided, fetch the object because it's needed for further questions
		selectedProject, err = selectors.FindProject(octopus, options.ProjectName)
		if err != nil {
			return err
		}
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
	if err != nil {
		return err
	}

	// immediately load the deployment process template
	// we need the deployment process template in order to get the steps, so we can lookup the stepID
	deploymentProcessTemplate, err := octopus.DeploymentProcesses.GetTemplate(deploymentProcess, selectedChannel.ID, "")
	// don't stop the spinner, BuildPackageVersionBaseline does more networking
	if err != nil {
		return err
	}

	packageVersionBaseline, err := BuildPackageVersionBaseline(octopus, deploymentProcessTemplate, selectedChannel)
	if err != nil {
		return err
	}

	var overriddenPackageVersions []*executionscommon.StepPackageVersion
	if len(packageVersionBaseline) > 0 { // if we have packages, run the package flow
		opv, packageVersionOverrides, err := AskPackageOverrideLoop(
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

	if options.Version == "" {
		// After loading the deployment process and channel, the logic forks here:
		// If the project's VersioningStrategy has a Template then we need to look in the deploymentprocesstemplate for the next release version
		// If the project's VersioningStrategy has a DonorPackageStepId then we need to follow the package trail to determine the release version
		// - but we must allow the user to override package versions first.
		// If the project's VersioningStrategy is null, it means this is a Config-as-code project and we need to
		// additionally load the deployment settings because the API doesn't inline the strategy in the main project resource for some reason
		var versioningStrategy *projects.VersioningStrategy
		if selectedProject.VersioningStrategy != nil {
			versioningStrategy = selectedProject.VersioningStrategy
		} else {
			deploymentSettings, err := octopus.Deployments.GetDeploymentSettings(selectedProject, gitReferenceKey)
			if err != nil {
				return err
			}
			versioningStrategy = deploymentSettings.VersioningStrategy
		}
		if versioningStrategy == nil { // not sure if this should ever happen, but best to be defensive
			return cliErrors.NewInvalidResponseError(fmt.Sprintf("cannot determine versioning strategy for project %s", selectedProject.Name))
		}

		if versioningStrategy.DonorPackageStepID != nil || versioningStrategy.DonorPackage != nil {
			// we've already done the package version work so we can just ask the donor package which version it has selected
			var donorPackage *executionscommon.StepPackageVersion
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
	return nil
}

func AskPackageOverrideLoop(
	packageVersionBaseline []*executionscommon.StepPackageVersion,
	defaultPackageVersion string, // the --package-version command line flag
	initialPackageOverrideFlags []string, // the --package command line flag (multiple occurrences)
	asker question.Asker,
	stdout io.Writer) ([]*executionscommon.StepPackageVersion, []*PackageVersionOverride, error) {
	packageVersionOverrides := make([]*PackageVersionOverride, 0)

	// pickup any partial package specifications that may have arrived on the commandline
	if defaultPackageVersion != "" {
		// blind apply to everything
		packageVersionOverrides = append(packageVersionOverrides, &PackageVersionOverride{Version: defaultPackageVersion})
	}

	for _, s := range initialPackageOverrideFlags {
		ambOverride, err := ParsePackageOverrideString(s)
		if err != nil {
			continue // silently ignore anything that wasn't parseable (should we emit a warning?)
		}
		resolvedOverride, err := ResolvePackageOverride(ambOverride, packageVersionBaseline)
		if err != nil {
			continue // silently ignore anything that wasn't parseable (should we emit a warning?)
		}
		packageVersionOverrides = append(packageVersionOverrides, resolvedOverride)
	}

	overriddenPackageVersions := ApplyPackageOverrides(packageVersionBaseline, packageVersionOverrides)

outerLoop:
	for {
		err := printPackageVersions(stdout, overriddenPackageVersions)
		if err != nil {
			return nil, nil, err
		}

		// While there are any unresolved package versions, force those
		for _, pkgVersionEntry := range overriddenPackageVersions {
			if strings.TrimSpace(pkgVersionEntry.Version) == "" {

				var answer = ""
				for strings.TrimSpace(answer) == "" { // if they enter a blank line just ask again repeatedly
					err = asker(&survey.Input{
						Message: output.Yellowf("Unable to find a version for \"%s\". Specify a version:", pkgVersionEntry.PackageID),
					}, &answer, survey.WithValidator(func(ans interface{}) error {
						str, ok := ans.(string)
						if !ok {
							return errors.New("internal error; answer was not a string")
						}
						if !isValidVersion(str) {
							return fmt.Errorf("\"%s\" is not a valid version", str)
						}
						return nil
					}))

					if err != nil {
						return nil, nil, err
					}
				}

				override := &PackageVersionOverride{Version: answer, ActionName: pkgVersionEntry.ActionName, PackageReferenceName: pkgVersionEntry.PackageReferenceName}
				if override != nil {
					packageVersionOverrides = append(packageVersionOverrides, override)
					overriddenPackageVersions = ApplyPackageOverrides(packageVersionBaseline, packageVersionOverrides)
				}
				continue outerLoop
			}
		}

		// After all packages have versions attached, we can let people freely tweak things until they're happy

		// side-channel return value from the validator
		var resolvedOverride *PackageVersionOverride = nil
		var answer = ""
		err = asker(&survey.Input{
			Message: "Package override string (y to accept, u to undo, ? for help):",
		}, &answer, survey.WithValidator(func(ans interface{}) error {
			str, ok := ans.(string)
			if !ok {
				return errors.New("internal error; answer was not a string")
			}

			switch str {
			// valid response for continuing the loop; don't attempt to validate these
			case "y", "u", "r", "?", "":
				return nil
			}

			ambOverride, err := ParsePackageOverrideString(str)
			if err != nil {
				return err
			}
			resolvedOverride, err = ResolvePackageOverride(ambOverride, packageVersionBaseline)
			if err != nil {
				return err
			}

			return nil // good!
		}))

		// if validators return an error, survey retries itself; the errors don't end up at this level.
		if err != nil {
			return nil, nil, err
		}

		switch answer {
		case "y": // YES these are the packages they want
			break outerLoop
		case "?": // help text
			_, _ = fmt.Fprintf(stdout, output.FormatDoc(packageOverrideLoopHelpText))
		case "u": // undo!
			if len(packageVersionOverrides) > 0 {
				packageVersionOverrides = packageVersionOverrides[:len(packageVersionOverrides)-1]
				// always reset to the baseline and apply everything in order, there's less room for logic errors
				overriddenPackageVersions = ApplyPackageOverrides(packageVersionBaseline, packageVersionOverrides)
			}
		case "r": // reset! All the way back to the calculated versions, discarding even the stuff that came in from the cmdline
			if len(packageVersionOverrides) > 0 {
				packageVersionOverrides = make([]*PackageVersionOverride, 0)
				overriddenPackageVersions = ApplyPackageOverrides(packageVersionBaseline, packageVersionOverrides)
			}
		default:
			if resolvedOverride != nil {
				packageVersionOverrides = append(packageVersionOverrides, resolvedOverride)
				// always reset to the baseline and apply everything in order, there's less room for logic errors
				overriddenPackageVersions = ApplyPackageOverrides(packageVersionBaseline, packageVersionOverrides)
			}
		}
		// loop around and let them put in more input
	}
	return overriddenPackageVersions, packageVersionOverrides, nil
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
