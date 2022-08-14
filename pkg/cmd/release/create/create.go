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
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/feeds"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
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
	FlagPackageVersionOverride = "package-override" // package-version-override? This one should allow multiple occurrences
)

type PackageVersions struct {
	Description string
	Last        string
	Latest      string
	PackageID   string
	Versions    []string
}

func NewPackageVersions() PackageVersions {
	return PackageVersions{
		Latest:   "Unknown",
		Last:     "Unknown",
		Versions: []string{},
	}
}

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
	cmd.Flags().StringSliceP(FlagPackageVersionOverride, "o", []string{}, "Version Override for a specific package.\nFormat as {step}:{package}:{version}\nYou may specify this multiple times")

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
		options.PackageVersion = value
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

	octopus, err := f.GetSpacedClient()
	if err != nil {
		return err
	}

	if f.IsPromptEnabled() {
		err = AskQuestions(octopus, f.Ask, f.Spinner(), options)
		if err != nil {
			return err
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

		// TODO AutomaticallyDeployedEnvironments. Discuss with Team
	}

	return nil
}

func AskQuestions(octopus *octopusApiClient.Client, asker question.Asker, spinner factory.Spinner, options *executor.TaskOptionsCreateRelease) error {
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
		_, _ = fmt.Printf("Project %s\n", output.Cyan(selectedProject.Name))
	}
	options.ProjectName = selectedProject.Name

	// we always need the deployment process, so we can prompt for package version overrides (or know that there aren't any packages, so it doesn't matter)

	var gitReferenceKey string
	if selectedProject.PersistenceSettings.GetType() == "VersionControlled" {
		// if there is no git reference specified, ask the server for the list and prompt.
		// we leave GitCommit alone in interactive mode; we don't prompt, but if it was specified on the
		// commandline we just pass it through untouched.

		//var gitRef *projects.GitReference
		if options.GitReference == "" { // we need a git ref; ask for one
			gitRef, err := selectGitReference(octopus, asker, spinner, selectedProject)
			if err != nil {
				return err
			}
			options.GitReference = gitRef.CanonicalName
		} else {
			// we need to go lookup the git reference
			_, _ = fmt.Printf("Git Reference %s\n", output.Cyan(options.GitReference))
		}

		// we could go and query the server and validate if the commit exists, is this worthwhile?
		if options.GitCommit != "" {
			_, _ = fmt.Printf("Git Commit %s\n", output.Cyan(options.GitCommit))
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
		_, _ = fmt.Printf("Channel %s\n", output.Cyan(selectedChannel.Name))
	}
	options.ChannelName = selectedChannel.Name
	if err != nil {
		return err
	}

	// TODO package version prompting goes here BEFORE specification of the release version

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
		if versioningStrategy.DonorPackageStepID != nil {
			// TODO go lookup the package based on the step ID, then select it's version for the release version
		} else if versioningStrategy.Template != "" {
			spinner.Start()
			dpt, err := octopus.DeploymentProcesses.GetTemplate(deploymentProcess, selectedChannel.ID, "")
			spinner.Stop()
			if err != nil {
				return err
			}
			defaultNextVersion = dpt.NextVersionIncrement
		}

		version, err := askVersion(asker, defaultNextVersion)
		if err != nil {
			return err
		}
		options.Version = version
	} else {
		_, _ = fmt.Printf("Version %s\n", output.Cyan(options.Version))
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
		// TODO is there any possible scenario where p.Channel might not be included in existingChannel?
		// we should be able to collapse this to a simple "return p.Name"
		for _, v := range existingChannels {
			if p.Name == v.Name {
				return v.Name
			}
		}

		return ""
	})
}

func findChannel(octopus *octopusApiClient.Client, spinner factory.Spinner, project *projects.Project, channelName string) (*channels.Channel, error) {
	spinner.Start()
	foundChannels, err := octopus.Projects.GetChannels(project) // TODO server-side filtering support in future
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

func selectPackageOverrides(octopus *octopusApiClient.Client, ask question.Asker, project *projects.Project, channel *channels.Channel, releaseID string) (string, error) {
	deploymentProcess, err := octopus.DeploymentProcesses.Get(project, "")
	if err != nil {
		return "", err
	}

	template, err := octopus.DeploymentProcesses.GetTemplate(deploymentProcess, channel.ID, releaseID)
	if err != nil {
		return "", err
	}

	feedsToQuery := make([]string, len(template.Packages))
	for _, v := range template.Packages {
		feedsToQuery = append(feedsToQuery, v.FeedID)
	}

	existingFeeds, err := octopus.Feeds.Get(feeds.FeedsQuery{IDs: feedsToQuery})
	if err != nil {
		return "", err
	}

	packageVersions := []PackageVersions{}

	stepPackages := []string{}
	stepPackages = append(stepPackages, output.Greenf("Done"))
	packageVersion := NewPackageVersions()

	for _, v := range template.Packages {
		for _, existingFeed := range existingFeeds.Items {
			if v.FeedID == existingFeed.GetID() {
				packageDescriptions, err := octopus.Feeds.SearchPackages(existingFeed, feeds.SearchPackagesQuery{
					Term: v.PackageID,
				})
				if err != nil {
					return "", err
				}

				packageVersion.Description = v.ActionName
				packageVersion.PackageID = v.PackageID
				packageVersion.Last = v.VersionSelectedLastRelease

				// TODO: iterate collection of package descriptions
				packageVersions, err := octopus.Feeds.SearchPackageVersions(packageDescriptions.Items[0], feeds.SearchPackageVersionsQuery{
					FeedID:    v.FeedID,
					PackageID: v.PackageID,
				})
				if err != nil {
					return "", err
				}

				for _, v := range packageVersions.Items {
					packageVersion.Versions = append(packageVersion.Versions, v.Version)
				}

				// TODO: iterate collection of package descriptions
				packageVersion.Latest = packageDescriptions.Items[0].LatestVersion
			}
		}
		// get other versions
		packageListing := fmt.Sprintf("%s (%s) - %s", packageVersion.PackageID, packageVersion.Description, packageVersion.Latest)
		stepPackages = append(stepPackages, packageListing)
		packageVersions = append(packageVersions, packageVersion)
	}
	stepPackages = append(stepPackages, "NuGet.CommandLine (Push Octopus.DotNet.Cli to NuGet style feed) - 1.2.3")
	stepPackages = append(stepPackages, "Octopus.DotNet.Cli (Push Octopus.DotNet.Cli to NuGet style feed) - 1.2.3")
	stepPackages = append(stepPackages, "Quux (do something) - 3.2.2")
	stepPackages = append(stepPackages, "Bar (do something) - 1.0.0")
	stepPackages = append(stepPackages, "Bar (do something) - 1.2.3")
	stepPackages = append(stepPackages, "Bar (do something)")
	stepPackages = append(stepPackages, "Bar (do something)")
	stepPackages = append(stepPackages, "Bar (do something)")

	packageVersion.Versions = append(packageVersion.Versions, "4.4.1 (Latest)")
	packageVersion.Versions = append(packageVersion.Versions, "3.4.1 (Last)")
	packageVersion.Versions = append(packageVersion.Versions, "3.4.0")
	packageVersion.Versions = append(packageVersion.Versions, "3.3.0")
	packageVersion.Versions = append(packageVersion.Versions, "3.2.0")
	packageVersion.Versions = append(packageVersion.Versions, "1.0.0")

	for {
		var selectedStepName string
		if err := ask(&survey.Select{
			Help:    "asdadsd",
			Message: "Select a step package to update its version to be used in the release",
			Options: stepPackages,
		}, &selectedStepName); err != nil {
			return "", err
		}

		if selectedStepName == output.Greenf("Done") {
			break
		}

		var selectedVersion string
		if err := ask(&survey.Select{
			Message: "Select a version of the package to be used",
			Options: packageVersion.Versions,
		}, &selectedVersion); err != nil {
			return "", err
		}
	}

	return "", nil
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

	// TODO talk within the team about what question wording to use here. It'd be nice to guide users as to why they need a git ref
	return question.SelectMap(ask, "Select the Git Reference to use", allRefs, func(g *projects.GitReference) string {
		return fmt.Sprintf("%s %s", g.Name, output.Dimf("(%s)", g.Type.Description()))
	})
}
