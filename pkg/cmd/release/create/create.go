package create

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
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
	flagProject                = "project"
	flagPackageVersion         = "package-version" // would default-package-version? be a better name?
	flagReleaseNotes           = "release-notes"   // should we also add release-notes-file?
	flagChannel                = "channel"
	flagVersion                = "version"
	flagGitRef                 = "git-ref"
	flagGitCommit              = "git-commit"
	flagIgnoreExisting         = "ignore-existing"
	flagIgnoreChannelRules     = "ignore-channel-rules"
	flagPackagePrerelease      = "prerelease-packages"
	flagPackageVersionOverride = "package-override" // package-version-override? This one should allow multiple occurrences
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
	cmd.Flags().StringP(flagProject, "p", "", "Name or ID of the project to create the release in")
	cmd.Flags().StringP(flagChannel, "c", "", "Name or ID of the channel to use")
	cmd.Flags().StringP(flagGitRef, "r", "", "Git Reference e.g. refs/head/main. Only relevant for config-as-code projects")
	cmd.Flags().StringP(flagGitCommit, "", "", "Git Commit Hash; Use as an alternative to Git Reference for advanced cases.")
	cmd.Flags().StringP(flagPackageVersion, "", "", "Default version to use for all Packages")
	cmd.Flags().StringP(flagReleaseNotes, "n", "", "Release notes to attach")
	cmd.Flags().StringP(flagVersion, "v", "", "Version Override")
	cmd.Flags().BoolP(flagIgnoreExisting, "x", false, "If a release with the same version exists, do nothing rather than failing.")
	cmd.Flags().BoolP(flagIgnoreChannelRules, "", false, "Force creation of a release where channel rules would otherwise prevent it.")
	cmd.Flags().BoolP(flagPackagePrerelease, "", false, "Allow selection of prerelease packages.") // TODO does this make sense? The server is going to follow channel rules anyway isn't it?
	// stringSlice also allows comma-separated things
	cmd.Flags().StringSliceP(flagPackageVersionOverride, "o", []string{}, "Version Override for a specific package.\nFormat as {step}:{package}:{version}\nYou may specify this multiple times")

	// we want the help text to display in the above order, rather than alphabetical
	cmd.Flags().SortFlags = false

	return cmd
}

func createRun(cmd *cobra.Command, f factory.Factory) error {
	project, err := cmd.Flags().GetString(flagProject)
	if err != nil {
		return err
	}

	options := &executor.TaskOptionsCreateRelease{
		ProjectName: project,
	}
	// ignore errors when fetching flags
	if value, _ := cmd.Flags().GetString(flagPackageVersion); value != "" {
		options.PackageVersion = value
	}
	if value, _ := cmd.Flags().GetString(flagChannel); value != "" {
		options.ChannelName = value
	}

	if value, _ := cmd.Flags().GetString(flagGitRef); value != "" {
		options.GitReference = value
	}
	if value, _ := cmd.Flags().GetString(flagGitCommit); value != "" {
		options.GitCommit = value
	}

	if value, _ := cmd.Flags().GetString(flagVersion); value != "" {
		options.Version = value
	}

	if value, _ := cmd.Flags().GetString(flagReleaseNotes); value != "" {
		options.ReleaseNotes = value
	}

	if value, _ := cmd.Flags().GetBool(flagIgnoreExisting); value {
		options.IgnoreIfAlreadyExists = value
	}

	octopus, err := f.GetSpacedClient()
	if err != nil {
		return err
	}

	if f.IsPromptEnabled() {
		err = AskQuestions(octopus, f.Ask, options)
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

func AskQuestions(octopus *octopusApiClient.Client, asker question.Asker, options *executor.TaskOptionsCreateRelease) error {
	// Note on output: survey prints things; if the option is specified already from the command line,
	// we should emulate that so there is always a line where you can see what the item was when specified on the command line,
	// however if we support a "quiet mode" then we shouldn't emit those

	var err error
	var selectedProject *projects.Project
	if options.ProjectName == "" {
		selectedProject, err = selectProject(octopus, asker)
		if err != nil {
			return err
		}
	} else { // project name is already provided, fetch the object because it's needed for further questions
		selectedProject, err = findProject(octopus, options.ProjectName)
		if err != nil {
			return err
		}
		_, _ = fmt.Printf("Project %s\n", output.Cyan(selectedProject.Name))
	}
	options.ProjectName = selectedProject.Name

	if selectedProject.PersistenceSettings.GetType() == "VersionControlled" && options.GitReference != "" || options.GitCommit != "" {
		// it's a config-as-code project, we need to ask for Git Ref or Commit

	}

	var selectedChannel *channels.Channel
	if options.ChannelName == "" {
		selectedChannel, err = selectChannel(octopus, asker, selectedProject)
		if err != nil {
			return err
		}
	} else {
		selectedChannel, err = findChannel(octopus, selectedProject, options.ChannelName)
		if err != nil {
			return err
		}
		_, _ = fmt.Printf("Channel %s\n", output.Cyan(selectedChannel.Name))
	}
	options.ChannelName = selectedChannel.Name

	if err != nil {
		return err
	}

	if options.Version == "" {
		version, err := askVersion(octopus, asker, selectedProject, selectedChannel)
		if err != nil {
			return err
		}
		options.Version = version
	} else {
		_, _ = fmt.Printf("Version %s\n", output.Cyan(options.Version))
	}
	return nil
}

func askVersion(octopus *octopusApiClient.Client, ask question.Asker, project *projects.Project, channel *channels.Channel) (string, error) {
	deploymentProcess, err := octopus.DeploymentProcesses.Get(project, "")
	if err != nil {
		return "", err
	}

	template, err := octopus.DeploymentProcesses.GetTemplate(deploymentProcess, channel.ID, "")
	if err != nil {
		return "", err
	}

	var version string
	if err := ask(&survey.Input{
		Default: template.NextVersionIncrement,
		Message: "Version",
	}, &version); err != nil {
		return "", err
	}

	return version, nil
}

func selectChannel(octopus *octopusApiClient.Client, ask question.Asker, project *projects.Project) (*channels.Channel, error) {
	existingChannels, err := octopus.Projects.GetChannels(project)
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

func findChannel(octopus *octopusApiClient.Client, project *projects.Project, channelName string) (*channels.Channel, error) {
	foundChannels, err := octopus.Projects.GetChannels(project) // TODO server-side filtering support in future
	if err != nil {
		return nil, err
	}
	for _, c := range foundChannels { // server doesn't support channel search by exact name so we must emulate it
		if strings.EqualFold(c.Name, channelName) {
			return c, nil
		}
	}
	// TODO should we prompt here instead?
	return nil, fmt.Errorf("no channel found with name of %s", channelName)
}

func findProject(octopus *octopusApiClient.Client, projectName string) (*projects.Project, error) {
	// projectsQuery has "Name" but it's just an alias in the server for PartialName; we need to filter client side
	projectsPage, err := octopus.Projects.Get(projects.ProjectsQuery{PartialName: projectName})
	if err != nil {
		return nil, err
	}
	for projectsPage != nil && len(projectsPage.Items) > 0 {
		for _, c := range projectsPage.Items { // server doesn't support channel search by exact name so we must emulate it
			if strings.EqualFold(c.Name, projectName) {
				return c, nil
			}
		}
		projectsPage, err = projectsPage.GetNextPage(octopus.Projects.GetClient())
		if err != nil {
			return nil, err
		} // if there are no more pages, then GetNextPage will return nil, which breaks us out of the loop
	}

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

func selectProject(octopus *octopusApiClient.Client, ask question.Asker) (*projects.Project, error) {

	existingProjects, err := octopus.Projects.GetAll()
	if err != nil {
		return nil, err
	}

	return question.SelectMap(ask, "Select the project in which the release will be created", existingProjects, func(p *projects.Project) string {
		for _, v := range existingProjects {
			if p.Name == v.Name {
				return v.Name
			}
		}

		return ""
	})
}
