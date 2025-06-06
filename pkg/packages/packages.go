package packages

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/feeds"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
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

type StepPackageVersion struct {
	// these 3 fields are the main ones for showing the user
	PackageID  string
	ActionName string // "StepName is an obsolete alias for ActionName, they always contain the same value"
	Version    string // note this may be an empty string, indicating that no version could be found for this package yet

	// used to locate the deployment process VersioningStrategy Donor Package
	PackageReferenceName string
}

// BuildPackageVersionBaseline takes in a set of template packages from the server, and for each step+package therein,
// finds the latest available version. Additional parameters for the feed query can be supplied using the setAdditionalFeedQueryParameters callback.
// Result is the list of step+package+versions to use as a baseline.
// The package version override process takes this as an input and layers on top of it
func BuildPackageVersionBaseline(octopus *octopusApiClient.Client, packages []releases.ReleaseTemplatePackage,
	setAdditionalFeedQueryParameters func(releases.ReleaseTemplatePackage, feeds.SearchPackageVersionsQuery) (feeds.SearchPackageVersionsQuery, error)) ([]*StepPackageVersion, error) {
	result := make([]*StepPackageVersion, 0, len(packages))

	// step 1: pass over all the packages in the deployment process, group them
	// by their feed, then subgroup by packageId

	// map(key: FeedID, value: list of references using the package so we can trace back to steps)
	feedsToQuery := make(map[string][]releases.ReleaseTemplatePackage)
	for _, pkg := range packages {

		if pkg.FixedVersion != "" {
			// If a package has a fixed version it shouldn't be displayed or overridable at all
			continue
		}

		// If a package is not considered resolvable by the server, don't attempt to query it's feed or lookup
		// any potential versions for it; we can't succeed in that because variable templates won't get expanded
		// until deployment time
		if !pkg.IsResolvable {
			result = append(result, &StepPackageVersion{
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

			if setAdditionalFeedQueryParameters != nil {
				query, err = setAdditionalFeedQueryParameters(packageRef, query)

				if err != nil {
					return nil, err
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
				case 0: // no package found; cache the response
					cache[query] = ""
					result = append(result, &StepPackageVersion{
						PackageID:            packageRef.PackageID,
						ActionName:           packageRef.ActionName,
						PackageReferenceName: packageRef.PackageReferenceName,
						Version:              "",
					})

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
	return util.SplitString(s, []int32{':', '/', '='})
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

func ResolvePackageOverride(override *AmbiguousPackageVersionOverride, steps []*StepPackageVersion) (*PackageVersionOverride, error) {
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
				consolidated[index+1] = &StepPackageVersion{
					PackageID:  output.Dim(pkg.PackageID),
					Version:    output.Dim(pkg.Version),
					ActionName: qualifiedPkgActionName,
				}
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

func AskPackageOverrideLoop(
	packageVersionBaseline []*StepPackageVersion,
	defaultPackageVersion string, // the --package-version command line flag
	initialPackageOverrideFlags []string, // the --package command line flag (multiple occurrences)
	asker question.Asker,
	stdout io.Writer) ([]*StepPackageVersion, []*PackageVersionOverride, error) {
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
