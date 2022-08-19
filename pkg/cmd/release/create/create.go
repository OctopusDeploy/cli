package create

import (
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	cliErrors "github.com/OctopusDeploy/cli/pkg/errors"
	"github.com/OctopusDeploy/cli/pkg/executor"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/feeds"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"io"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

const (
	FlagProject                = "project"
	FlagPackageVersion         = "package-version" // would default-package-version? be a better name?
	FlagReleaseNotes           = "release-notes"   // should we also add release-notes-file?
	FlagChannel                = "channel"
	FlagVersion                = "version"
	FlagGitRef                 = "git-ref"
	FlagGitCommit              = "git-commit"
	FlagIgnoreExisting         = "ignore-existing"
	FlagIgnoreChannelRules     = "ignore-channel-rules"
	FlagPackagePrerelease      = "prerelease-packages"
	FlagPackageVersionOverride = "package"
)

func NewCmdCreate(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a release in an instance of Octopus Deploy",
		Long:  "Creates a release in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s release create --project MyProject --channel Beta -v "1.2.3"

			$ %s release create -p MyProject -c default -o "installstep:utils:1.2.3" -o "installstep:helpers:5.6.7"
		`), constants.ExecutableName, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error { return createRun(cmd, f) },
	}

	// project is required in automation mode, other options are not. Nothing is required in interactive mode because we prompt for everything
	cmd.Flags().StringP(FlagProject, "p", "", "Name or ID of the project to create the release in")
	cmd.Flags().StringP(FlagChannel, "c", "", "Name or ID of the channel to use")
	cmd.Flags().StringP(FlagGitRef, "r", "", "Git Reference e.g. refs/heads/main. Only relevant for config-as-code projects")
	cmd.Flags().StringP(FlagGitCommit, "", "", "Git Commit Hash; Specify this in addition to Git Reference if you want to reference a commit other than the latest for that branch/tag.")
	cmd.Flags().StringP(FlagPackageVersion, "", "", "Default version to use for all Packages")
	cmd.Flags().StringP(FlagReleaseNotes, "n", "", "Release notes to attach")
	cmd.Flags().StringP(FlagVersion, "v", "", "Version Override")
	cmd.Flags().BoolP(FlagIgnoreExisting, "x", false, "If a release with the same version exists, do nothing rather than failing.")
	cmd.Flags().BoolP(FlagIgnoreChannelRules, "", false, "Force creation of a release where channel rules would otherwise prevent it.")
	cmd.Flags().BoolP(FlagPackagePrerelease, "", false, "Allow selection of prerelease packages.") // TODO does this make sense? The server is going to follow channel rules anyway isn't it?
	// stringSlice also allows comma-separated things
	cmd.Flags().StringSliceP(FlagPackageVersionOverride, "", []string{}, "Version Override for a specific package.\nFormat as {step}:{package}:{version}\nYou may specify this multiple times")

	// we want the help text to display in the above order, rather than alphabetical
	cmd.Flags().SortFlags = false

	return cmd
}

func createRun(cmd *cobra.Command, f factory.Factory) error {
	project, err := cmd.Flags().GetString(FlagProject)
	if err != nil {
		return err
	}

	options := &executor.TaskOptionsCreateRelease{
		ProjectName: project,
	}
	// ignore errors when fetching flags
	if value, _ := cmd.Flags().GetString(FlagPackageVersion); value != "" {
		options.DefaultPackageVersion = value
	}

	if value, _ := cmd.Flags().GetStringSlice(FlagPackageVersionOverride); value != nil {
		options.PackageVersionOverrides = value
	}

	if value, _ := cmd.Flags().GetString(FlagChannel); value != "" {
		options.ChannelName = value
	}

	if value, _ := cmd.Flags().GetString(FlagGitRef); value != "" {
		options.GitReference = value
	}
	if value, _ := cmd.Flags().GetString(FlagGitCommit); value != "" {
		options.GitCommit = value
	}

	if value, _ := cmd.Flags().GetString(FlagVersion); value != "" {
		options.Version = value
	}

	if value, _ := cmd.Flags().GetString(FlagReleaseNotes); value != "" {
		options.ReleaseNotes = value
	}

	if value, _ := cmd.Flags().GetBool(FlagIgnoreExisting); value {
		options.IgnoreIfAlreadyExists = value
	}
	if value, _ := cmd.Flags().GetBool(FlagIgnoreChannelRules); value {
		options.IgnoreChannelRules = value
	}

	octopus, err := f.GetSpacedClient()
	if err != nil {
		return err
	}

	if f.IsPromptEnabled() {
		err = AskQuestions(octopus, cmd.OutOrStdout(), f.Ask, f.Spinner(), options)
		if err != nil {
			return err
		}

		cmd.Printf("invocation: release create %s\n", ToCmdFlags(options))
	}

	// the executor will raise errors if any required options are missing
	err = executor.ProcessTasks(octopus, f.GetCurrentSpace(), []*executor.Task{
		executor.NewTask(executor.TaskTypeCreateRelease, options),
	})
	if err != nil {
		return err
	}

	if options.Response != nil {
		// the API response doesn't tell us what channel it selected, so we need to go look that up to tell the end user
		// TODO unit test for the error cases
		newlyCreatedRelease, lookupErr := octopus.Releases.GetByID(options.Response.ReleaseID)
		if lookupErr != nil { // ignorable error
			cmd.Printf("Successfully created release version %s %s\n",
				options.Response.ReleaseVersion,
				output.Dimf("(%s)", options.Response.ReleaseID))

			cmd.PrintErrf("Warning: cannot fetch release details: %v\n", lookupErr)
		} else {
			releaseChan, lookupErr := octopus.Channels.GetByID(newlyCreatedRelease.ChannelID)
			if lookupErr != nil { // ignorable error
				cmd.Printf("Successfully created release version %s %s using channel %s\n",
					options.Response.ReleaseVersion,
					output.Dimf("(%s)", options.Response.ReleaseID),
					output.Dimf("(%s)", releaseChan.ID))

				cmd.PrintErrf("Warning: cannot fetch release channel details: %v\n", lookupErr)
			} else {
				cmd.Printf("Successfully created release version %s %s using channel %s %s\n",
					options.Response.ReleaseVersion,
					output.Dimf("(%s)", options.Response.ReleaseID),
					releaseChan.Name,
					output.Dimf("(%s)", releaseChan.ID))
			}
		}

		// response also returns AutomaticallyDeployedEnvironments, which was a failed feature; we should ignore it.
	}

	return nil
}

func quoteStringIfRequired(str string) string {
	for _, c := range str {
		if c == ' ' || c == '\t' { // TODO golang probably has a proper 'IsWhitespace'; look for that
			return fmt.Sprintf("\"%s\"", str)
		}
	}
	return str
}

// ToCmdFlags generates the command line switches that you'd need to type in to make this work in automation mode.
// TODO sync this with whatever dom has done; this is a big one-off hack
func ToCmdFlags(t *executor.TaskOptionsCreateRelease) string {
	components := make([]string, 0, 20)

	appendComponent := func(flag string, value string) {
		if value != "" {
			components = append(components, flag)
			components = append(components, quoteStringIfRequired(value))
		}
	}

	appendComponent("-p", t.ProjectName)
	appendComponent("--"+FlagGitCommit, t.GitCommit)
	appendComponent("-r", t.GitReference)
	appendComponent("-c", t.ChannelName)
	appendComponent("--"+FlagReleaseNotes, t.ReleaseNotes)
	if t.IgnoreIfAlreadyExists {
		components = append(components, "--"+FlagIgnoreExisting)
	}
	if t.IgnoreChannelRules {
		components = append(components, "--"+FlagIgnoreChannelRules)
	}
	for _, ov := range t.PackageVersionOverrides {
		components = append(components, "--package")
		components = append(components, quoteStringIfRequired(ov))
	}

	// version always goes at the end so if people copy/paste the commandline it's easy to tweak
	appendComponent("-v", t.Version)
	return strings.Join(components, " ")
}

type StepPackageVersion struct {
	// these 3 fields are the main ones for showing the user
	PackageID  string
	ActionName string // "StepName is an obsolete alias for ActionName, they always contain the same value"
	Version    string

	// used to locate the deployment process VersioningStrategy Donor Package
	PackageReferenceName string
}

// buildPackageVersionBaseline loads the deployment process template from the server, and for each step+package therein,
// finds the latest available version satisfying the channel version rules. Result is the list of step+package+versions
// to use as a baseline. The package version override process takes this as an input and layers on top of it
func BuildPackageVersionBaseline(octopus *octopusApiClient.Client, deploymentProcessTemplate *deployments.DeploymentProcessTemplate, channel *channels.Channel) ([]*StepPackageVersion, error) {
	result := make([]*StepPackageVersion, 0, len(deploymentProcessTemplate.Packages))

	// step 1: pass over all the packages in the deployment process, group them
	// by their feed, then subgroup by packageId

	// map(key: FeedID, value: list of references using the package so we can trace back to steps)
	feedsToQuery := make(map[string][]releases.ReleaseTemplatePackage)
	for _, pkg := range deploymentProcessTemplate.Packages {
		if feedPackages, seenFeedBefore := feedsToQuery[pkg.FeedID]; !seenFeedBefore {
			feedsToQuery[pkg.FeedID] = []releases.ReleaseTemplatePackage{pkg}
		} else {
			// seen both the feed and package, but not against this particular step
			feedsToQuery[pkg.FeedID] = append(feedPackages, pkg)
		}
	}

	if len(feedsToQuery) == 0 {
		return make([]*StepPackageVersion, 0), nil
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
				result = append(result, &StepPackageVersion{
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
				case 0:
					// TODO add some unit tests for this
					channelRulesHelp := ""
					if query.PreReleaseTag != "" {
						channelRulesHelp = fmt.Sprintf("%s. pre-release tag matching %s. ", channelRulesHelp, query.PreReleaseTag)
					}
					if query.VersionRange != "" {
						channelRulesHelp = fmt.Sprintf("%s. version range matching %s. ", channelRulesHelp, query.VersionRange)
					}
					return nil, fmt.Errorf("no package version found for %s. %s please check that the package exists in your package feed", packageRef.PackageID, channelRulesHelp)
					// if channel rules are in-play tweak the message to say "on package matching rules xyz

				case 1:
					cache[query] = versions.Items[0].Version
					result = append(result, &StepPackageVersion{
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
	if p.PackageReferenceName != "" {
		components = append(components, p.PackageReferenceName)
	}
	if p.PackageID != "" {
		components = append(components, p.PackageID)
	} else if p.ActionName != "" { // can't have both PackageID and ActionName; PackageID wins
		components = append(components, p.ActionName)
	} else if len(components) == 1 { // if we have an explicit packagereference but no packageId or action, we need to express it with ref:*:version
		components = append(components, "*")
	}

	if len(components) == 0 { // the server can't deal with just a number by itself; if we want to override everything we must pass *:Version
		components = append(components, "*")
	}
	components = append(components, p.Version)

	return strings.Join(components, ":")
}

// splitPackageOverrideString splits the input string into components based on delimiter characters.
// we want to pick up empty entries here; so "::5" nad ":pterm:5" should both return THREE components, rather than one or two
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
	case 1:
		// the server doesn't support this, but we do interactively; override the version for all packages
		version = strings.TrimSpace(components[0])
	case 2:
		// if there are two components it is (StepName|PackageID):Version
		stepNameOrPackageID, version = strings.TrimSpace(components[0]), strings.TrimSpace(components[1])
	case 3:
		// if there are three components it is PackageReferenceName:(StepName|PackageID):Version
		packageReferenceName, stepNameOrPackageID, version = strings.TrimSpace(components[0]), strings.TrimSpace(components[1]), strings.TrimSpace(components[2])
	default:
		return nil, fmt.Errorf("package version specification %s does not use expected format", packageOverride)
	}

	// must always specify a version; must specify either packageID, stepName or both
	if version == "" {
		return nil, fmt.Errorf("package version specification %s does not use expected format", packageOverride)
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

func ResolvePackageOverride(override *AmbiguousPackageVersionOverride, steps []*StepPackageVersion) (*PackageVersionOverride, error) {
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
		return nil, fmt.Errorf("could not resolve find step name or package ID matching %s", actionNameOrPackageID)
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

func ApplyPackageOverrides(packages []*StepPackageVersion, overrides []*PackageVersionOverride) []*StepPackageVersion {
	for _, o := range overrides {
		packages = applyPackageOverride(packages, o)
	}
	return packages
}

func applyPackageOverride(packages []*StepPackageVersion, override *PackageVersionOverride) []*StepPackageVersion {
	if override.Version == "" {
		return packages // not specifying a version is technically an error, but we'll just no-op it for safety; should have been filtered out by ParsePackageOverrideString before we get here
	}

	var matcher func(pkg *StepPackageVersion) bool = nil

	switch {
	case override.PackageID == "" && override.ActionName == "": // match everything
		matcher = func(pkg *StepPackageVersion) bool {
			return true
		}
	case override.PackageID != "" && override.ActionName == "": // match on package ID only
		matcher = func(pkg *StepPackageVersion) bool {
			return pkg.PackageID == override.PackageID
		}
	case override.PackageID == "" && override.ActionName != "": // match on step only
		matcher = func(pkg *StepPackageVersion) bool {
			return pkg.ActionName == override.ActionName
		}
	case override.PackageID != "" && override.ActionName != "": // match on both; shouldn't be possible but let's ensure it works anyway
		matcher = func(pkg *StepPackageVersion) bool {
			return pkg.PackageID == override.PackageID && pkg.ActionName == override.ActionName
		}
	}

	if override.PackageReferenceName != "" { // must also match package reference name
		if matcher == nil {
			matcher = func(pkg *StepPackageVersion) bool {
				return pkg.PackageReferenceName == override.PackageReferenceName
			}
		} else {
			prevMatcher := matcher
			matcher = func(pkg *StepPackageVersion) bool {
				return pkg.PackageReferenceName == override.PackageReferenceName && prevMatcher(pkg)
			}
		}
	}

	if matcher == nil {
		return packages // we can't possibly match against anything; no-op. Should have been filtered out by ParsePackageOverrideString
	}

	result := make([]*StepPackageVersion, len(packages))
	for i, p := range packages {
		if matcher(p) {
			result[i] = &StepPackageVersion{
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
func printPackageVersions(ioWriter io.Writer, packages []*StepPackageVersion) error {
	// step 1: consolidate multiple rows
	consolidated := make([]*StepPackageVersion, 0, len(packages))
	for _, pkg := range packages {

		// the common case is that packageReferenceName will be equal to PackageID.
		// however, in advanced cases it may not be, so we need to do extra work to show the packageReferenceName too.
		// we suffix it onto the step name, following the web UI
		qualifiedPkgActionName := pkg.ActionName
		if pkg.PackageID != pkg.PackageReferenceName {
			qualifiedPkgActionName = fmt.Sprintf("%s/%s", qualifiedPkgActionName, pkg.PackageReferenceName)
		}

		// find existing entry and update it if possible
		updatedExisting := false
		for _, entry := range consolidated {
			if entry.PackageID == pkg.PackageID && entry.Version == pkg.Version {
				entry.ActionName = fmt.Sprintf("%s, %s", entry.ActionName, qualifiedPkgActionName)
				updatedExisting = true
				break
			}
		}
		if !updatedExisting {
			consolidated = append(consolidated, &StepPackageVersion{
				PackageID:  pkg.PackageID,
				Version:    pkg.Version,
				ActionName: qualifiedPkgActionName,
			})
		}
	}

	// step 2: print them
	t := output.NewTable(ioWriter)
	t.AddRow(output.Dim("PACKAGE"), output.Dim("VERSION"), output.Dim("STEPS"))

	for _, pkg := range consolidated {
		t.AddRow(
			pkg.PackageID,
			pkg.Version,
			pkg.ActionName,
		)
	}

	return t.Print()
}

func AskQuestions(octopus *octopusApiClient.Client, stdout io.Writer, asker question.Asker, spinner factory.Spinner, options *executor.TaskOptionsCreateRelease) error {
	if octopus == nil {
		return errors.New("api client is required")
	}
	if asker == nil {
		return errors.New("asker is required")
	}
	if options == nil {
		return errors.New("options is required")
	}
	// Note: we don't get here at all if no-prompt is enabled, so we know we are free to ask questions

	// Note on output: survey prints things; if the option is specified already from the command line,
	// we should emulate that so there is always a line where you can see what the item was when specified on the command line,
	// however if we support a "quiet mode" then we shouldn't emit those

	var err error
	var selectedProject *projects.Project
	if options.ProjectName == "" {
		selectedProject, err = selectProject(octopus, asker, spinner)
		if err != nil {
			return err
		}
	} else { // project name is already provided, fetch the object because it's needed for further questions
		selectedProject, err = findProject(octopus, spinner, options.ProjectName)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "Project %s\n", output.Cyan(selectedProject.Name))
	}
	options.ProjectName = selectedProject.Name

	// we always need the deployment process, so we can prompt for package version overrides (or know that there aren't any packages, so it doesn't matter)

	var gitReferenceKey string
	if selectedProject.PersistenceSettings.GetType() == "VersionControlled" {
		// if there is no git reference specified, ask the server for the list and prompt.
		// we leave GitCommit alone in interactive mode; we don't prompt, but if it was specified on the
		// commandline we just pass it through untouched.

		if options.GitReference == "" { // we need a git ref; ask for one
			gitRef, err := selectGitReference(octopus, asker, spinner, selectedProject)
			if err != nil {
				return err
			}
			options.GitReference = gitRef.Name // Hold the short name, not the canonical name due to golang url parsing bug replacing %2f with /
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
	spinner.Start()
	deploymentProcess, err := octopus.DeploymentProcesses.Get(selectedProject, gitReferenceKey)
	spinner.Stop()
	if err != nil {
		return err
	}

	var selectedChannel *channels.Channel
	if options.ChannelName == "" {
		selectedChannel, err = selectChannel(octopus, asker, spinner, selectedProject)
		if err != nil {
			return err
		}
	} else {
		selectedChannel, err = findChannel(octopus, spinner, selectedProject, options.ChannelName)
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
	spinner.Start()
	deploymentProcessTemplate, err := octopus.DeploymentProcesses.GetTemplate(deploymentProcess, selectedChannel.ID, "")
	// don't stop the spinner, we may have more networking
	if err != nil {
		spinner.Stop()
		return err
	}

	packageVersionBaseline, err := BuildPackageVersionBaseline(octopus, deploymentProcessTemplate, selectedChannel)
	var overriddenPackageVersions []*StepPackageVersion

	if len(packageVersionBaseline) > 0 { // if we have packages, run the package flow
		packageVersionOverrides := make([]*PackageVersionOverride, 0)

		// pickup any partial package specifications that may have arrived on the commandline
		for _, s := range options.PackageVersionOverrides {
			ambOverride, err := ParsePackageOverrideString(s)
			if err != nil {
				continue // silently ignore anything that wasn't parseable (TODO should we emit a warning?)
			}
			resolvedOverride, err := ResolvePackageOverride(ambOverride, packageVersionBaseline)
			if err != nil {
				continue // silently ignore anything that wasn't parseable (TODO should we emit a warning?)
			}
			packageVersionOverrides = append(packageVersionOverrides, resolvedOverride)
		}

		overriddenPackageVersions = ApplyPackageOverrides(packageVersionBaseline, packageVersionOverrides)

		spinner.Stop()
		if err != nil {
			return err
		}

		for {
			err = printPackageVersions(stdout, overriddenPackageVersions)
			if err != nil {
				return err
			}

			var resolvedOverride *PackageVersionOverride = nil
			var answer = ""

			err := asker(&survey.Input{
				Message: "Enter package override string, or 'y' to accept package versions", // TODO nicer string when we do a usability pass.
			}, &answer, survey.WithValidator(func(ans interface{}) error {
				str, ok := ans.(string)
				if !ok {
					return errors.New("internal error; answer was not a string")
				}

				if str == "y" || str == "" { // valid response for continuing the loop; don't attempt to parse this
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

			if err != nil {
				return err // TODO probably handle this and loop again
			}
			if answer == "y" { // YES these are the packages they want
				break
			}

			if resolvedOverride != nil {
				packageVersionOverrides = append(packageVersionOverrides, resolvedOverride)
				// always reset to the baseline and apply everything in order, there's less room for logic errors
				overriddenPackageVersions = ApplyPackageOverrides(packageVersionBaseline, packageVersionOverrides)
			}
			// else the user most likely typed an empty string, loop around
		}

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
			spinner.Start()
			deploymentSettings, err := octopus.Deployments.GetDeploymentSettings(selectedProject, gitReferenceKey)
			spinner.Stop()
			if err != nil {
				return err
			}
			versioningStrategy = deploymentSettings.VersioningStrategy
		}
		if versioningStrategy == nil { // not sure if this should ever happen, but best to be defensive
			return cliErrors.NewInvalidResponseError(fmt.Sprintf("cannot determine versioning strategy for project %s", selectedProject.Name))
		}

		defaultNextVersion := ""
		if versioningStrategy.DonorPackageStepID != nil || versioningStrategy.DonorPackage != nil {
			// we've already done the package version work so we can just ask the donor package which version it has selected
			var donorPackage *StepPackageVersion
			for _, pkg := range overriddenPackageVersions {
				if pkg.PackageReferenceName == versioningStrategy.DonorPackage.PackageReference && pkg.ActionName == versioningStrategy.DonorPackage.DeploymentAction {
					donorPackage = pkg
					break
				}
			}
			if donorPackage == nil {
				// should this just be a warning rather than a hard fail? we could still just ask the user if they'd
				// like to type in a version or leave it blank? On the other hand, it shouldn't fail anyway :shrug:
				spinner.Stop()
				return fmt.Errorf("internal error: can't find donor package in deployment process template - version controlled configuration file in an invalid state")
			}

			defaultNextVersion = donorPackage.Version
			spinner.Stop()
		} else if versioningStrategy.Template != "" {
			// we already loaded the deployment process template when we were looking for packages
			defaultNextVersion = deploymentProcessTemplate.NextVersionIncrement
		}

		version, err := askVersion(asker, defaultNextVersion)
		if err != nil {
			return err
		}
		options.Version = version
	} else {
		_, _ = fmt.Fprintf(stdout, "Version %s\n", output.Cyan(options.Version))
	}
	return nil
}

func askVersion(ask question.Asker, defaultVersion string) (string, error) {
	var version string
	if err := ask(&survey.Input{
		Default: defaultVersion,
		Message: "Release Version",
	}, &version); err != nil {
		return "", err
	}

	return version, nil
}

func selectChannel(octopus *octopusApiClient.Client, ask question.Asker, spinner factory.Spinner, project *projects.Project) (*channels.Channel, error) {
	spinner.Start()
	existingChannels, err := octopus.Projects.GetChannels(project)
	spinner.Stop()
	if err != nil {
		return nil, err
	}

	return question.SelectMap(ask, "Select the channel in which the release will be created", existingChannels, func(p *channels.Channel) string {
		return p.Name
	})
}

func findChannel(octopus *octopusApiClient.Client, spinner factory.Spinner, project *projects.Project, channelName string) (*channels.Channel, error) {
	spinner.Start()
	foundChannels, err := octopus.Projects.GetChannels(project) // TODO change this to channel partial name search on server; will require go client update
	spinner.Stop()
	if err != nil {
		return nil, err
	}
	for _, c := range foundChannels { // server doesn't support channel search by exact name so we must emulate it
		if strings.EqualFold(c.Name, channelName) {
			return c, nil
		}
	}
	return nil, fmt.Errorf("no channel found with name of %s", channelName)
}

func findProject(octopus *octopusApiClient.Client, spinner factory.Spinner, projectName string) (*projects.Project, error) {
	// projectsQuery has "Name" but it's just an alias in the server for PartialName; we need to filter client side
	spinner.Start()
	projectsPage, err := octopus.Projects.Get(projects.ProjectsQuery{PartialName: projectName})
	if err != nil {
		spinner.Stop()
		return nil, err
	}
	for projectsPage != nil && len(projectsPage.Items) > 0 {
		for _, c := range projectsPage.Items { // server doesn't support channel search by exact name so we must emulate it
			if strings.EqualFold(c.Name, projectName) {
				spinner.Stop()
				return c, nil
			}
		}
		projectsPage, err = projectsPage.GetNextPage(octopus.Projects.GetClient())
		if err != nil {
			spinner.Stop()
			return nil, err
		} // if there are no more pages, then GetNextPage will return nil, which breaks us out of the loop
	}

	spinner.Stop()
	return nil, fmt.Errorf("no project found with name of %s", projectName)
}

func selectProject(octopus *octopusApiClient.Client, ask question.Asker, spinner factory.Spinner) (*projects.Project, error) {
	spinner.Start()
	existingProjects, err := octopus.Projects.GetAll()
	spinner.Stop()
	if err != nil {
		return nil, err
	}

	return question.SelectMap(ask, "Select the project in which the release will be created", existingProjects, func(p *projects.Project) string {
		return p.Name
	})
}

func selectGitReference(octopus *octopusApiClient.Client, ask question.Asker, spinner factory.Spinner, project *projects.Project) (*projects.GitReference, error) {
	spinner.Start()
	branches, err := octopus.Projects.GetGitBranches(project)
	if err != nil {
		spinner.Stop()
		return nil, err
	}

	tags, err := octopus.Projects.GetGitTags(project)
	spinner.Stop()

	if err != nil {
		return nil, err
	}

	allRefs := append(branches, tags...)

	return question.SelectMap(ask, "Select the Git Reference to use", allRefs, func(g *projects.GitReference) string {
		return fmt.Sprintf("%s %s", g.Name, output.Dimf("(%s)", g.Type.Description()))
	})
}
