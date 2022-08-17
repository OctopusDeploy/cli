package create_test

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/release/create"
	"github.com/OctopusDeploy/cli/pkg/executor"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/constants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/feeds"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/packages"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"net/url"
	"testing"
)

var serverUrl, _ = url.Parse("http://server")

const placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

var spinner = &testutil.FakeSpinner{}

var rootResource = testutil.NewRootResource()

func TestReleaseCreate_AskQuestions_RegularProject(t *testing.T) {

	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"

	depProcess := fixtures.NewDeploymentProcessForProject(spaceID, fireProjectID)

	defaultChannel := fixtures.NewChannel(spaceID, "Channels-1", "Fire Project Default Channel", fireProjectID)
	altChannel := fixtures.NewChannel(spaceID, "Channels-97", "Fire Project Alt Channel", fireProjectID)

	fireProject := fixtures.NewProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", depProcess.ID)

	t.Run("standard process asking for everything (no package versions)", func(t *testing.T) {
		api, qa := testutil.NewMockServerAndAsker()

		options := &executor.TaskOptionsCreateRelease{}

		errReceiver := testutil.GoBegin(func() error {
			defer testutil.Close(api, qa)
			// NewClient makes network calls so we have to run it in the goroutine
			octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
			return create.AskQuestions(octopus, qa.AsAsker(), spinner, options)
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{fireProject})

		qa.ExpectQuestion(t, &survey.Select{
			Message: "Select the project in which the release will be created",
			Options: []string{"Fire Project"},
		}).AnswerWith("Fire Project")

		api.ExpectRequest(t, "GET", "/api/Spaces-1/deploymentprocesses/deploymentprocess-"+fireProjectID).RespondWith(depProcess)

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/channels").RespondWith(resources.Resources[*channels.Channel]{
			Items: []*channels.Channel{defaultChannel, altChannel},
		})

		qa.ExpectQuestion(t, &survey.Select{
			Message: "Select the channel in which the release will be created",
			Options: []string{defaultChannel.Name, altChannel.Name},
		}).AnswerWith("Fire Project Alt Channel")

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/deploymentprocesses/template?channel=Channels-97").
			RespondWith(&deployments.DeploymentProcessTemplate{NextVersionIncrement: "27.9.3"})

		qa.ExpectQuestion(t, &survey.Input{
			Message: "Release Version",
			Default: "27.9.3",
		}).AnswerWith("27.9.999")

		err := <-errReceiver
		assert.Nil(t, err)

		// check that the question-asking process has filled out the things we told it to
		assert.Equal(t, "Fire Project", options.ProjectName)
		assert.Equal(t, "Fire Project Alt Channel", options.ChannelName)
		assert.Equal(t, "27.9.999", options.Version)
	})

	t.Run("asking for nothing in interactive mode (testing case insensitivity)", func(t *testing.T) {
		api, qa := testutil.NewMockServerAndAsker()

		options := &executor.TaskOptionsCreateRelease{
			ProjectName: "fire project",
			ChannelName: "fire project default channel",
			Version:     "9.8.4-prerelease",
		}

		errReceiver := testutil.GoBegin(func() error {
			defer testutil.Close(api, qa)
			octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
			return create.AskQuestions(octopus, qa.AsAsker(), spinner, options)
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?clonedFromProjectId=&partialName=fire+project").
			RespondWith(resources.Resources[*projects.Project]{
				Items: []*projects.Project{fireProject},
			})

		api.ExpectRequest(t, "GET", "/api/Spaces-1/deploymentprocesses/deploymentprocess-"+fireProjectID).RespondWith(depProcess)

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/channels").
			RespondWith(resources.Resources[*channels.Channel]{
				Items: []*channels.Channel{defaultChannel},
			})

		// always loads the deployment process template to check for packages
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/deploymentprocesses/template?channel=Channels-1").
			RespondWith(&deployments.DeploymentProcessTemplate{NextVersionIncrement: "27.9.3"})

		err := <-errReceiver
		assert.Nil(t, err)

		// check that the question-asking process has filled out the things we told it to
		assert.Equal(t, "Fire Project", options.ProjectName)
		assert.Equal(t, "Fire Project Default Channel", options.ChannelName)
		assert.Equal(t, "9.8.4-prerelease", options.Version)
	})
}

func TestReleaseCreate_AskQuestions_VersionControlledProject(t *testing.T) {
	const spaceID = "Spaces-1"

	projectID := "Projects-87"
	depProcessDevelopBranch := fixtures.NewDeploymentProcessForVersionControlledProject(spaceID, projectID, "develop")

	depSettings := fixtures.NewDeploymentSettingsForProject(spaceID, projectID, &projects.VersioningStrategy{
		Template: "#{Octopus.Version.LastMajor}.#{Octopus.Version.LastMinor}.#{Octopus.Version.NextPatch}", // bog standard
	})
	depTemplate := &deployments.DeploymentProcessTemplate{NextVersionIncrement: "27.9.3"}

	project := fixtures.NewVersionControlledProject(spaceID, projectID, "CaC Project", "Lifecycles-1", "ProjectGroups-1", depProcessDevelopBranch.ID)

	defaultChannel := fixtures.NewChannel(spaceID, "Channels-34", "CaC Project Default Channel", projectID)
	altChannel := fixtures.NewChannel(spaceID, "Channels-97", "CaC Project Alt Channel", projectID)

	t.Run("standard process asking for everything (no package versions); specific git commit not set", func(t *testing.T) {
		api, qa := testutil.NewMockServerAndAsker()

		options := &executor.TaskOptionsCreateRelease{}

		errReceiver := testutil.GoBegin(func() error {
			defer testutil.Close(api, qa)
			octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
			return create.AskQuestions(octopus, qa.AsAsker(), spinner, options)
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{project})

		qa.ExpectQuestion(t, &survey.Select{
			Message: "Select the project in which the release will be created",
			Options: []string{project.Name},
		}).AnswerWith(project.Name)

		// CLI will load all the branches and tags
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/git/branches").RespondWith(resources.Resources[*projects.GitReference]{
			PagedResults: resources.PagedResults{ItemType: "GitBranch"},
			Items: []*projects.GitReference{
				projects.NewGitBranchReference("main", "refs/heads/main"),
				projects.NewGitBranchReference("develop", "refs/heads/develop"),
			}})

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/git/tags").RespondWith(resources.Resources[*projects.GitReference]{
			PagedResults: resources.PagedResults{ItemType: "GitTag"},
			Items: []*projects.GitReference{
				projects.NewGitTagReference("v2", "refs/tags/v2"),
				projects.NewGitTagReference("v1", "refs/tags/v1"),
			}})

		qa.ExpectQuestion(t, &survey.Select{
			Message: "Select the Git Reference to use",
			Options: []string{"main (Branch)", "develop (Branch)", "v2 (Tag)", "v1 (Tag)"},
		}).AnswerWith("develop (Branch)")

		// can't specify a git commit hash in interactive mode

		// Once the CLI has picked up the git ref it then loads the deployment process which will be based on the git ref link
		// NOTE: we are only using the git short name here, not the full name due to the golang url parsing bug which
		// incorrectly turns %2f into a literal / in the URL
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/develop/deploymentprocesses").RespondWith(depProcessDevelopBranch)

		// next phase; channel selection

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/channels").RespondWith(resources.Resources[*channels.Channel]{
			Items: []*channels.Channel{defaultChannel, altChannel},
		})
		qa.ExpectQuestion(t, &survey.Select{
			Message: "Select the channel in which the release will be created",
			Options: []string{defaultChannel.Name, altChannel.Name},
		}).AnswerWith(altChannel.Name)

		// always loads dep process template
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/develop/deploymentprocesses/template?channel="+altChannel.ID).RespondWith(depTemplate)

		// our project inline versioning strategy was nil, so the code needs to load the deployment settings to find out
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/develop/deploymentsettings").RespondWith(depSettings)

		qa.ExpectQuestion(t, &survey.Input{
			Message: "Release Version",
			Default: "27.9.3", // from the dep template
		}).AnswerWith("27.9.999")

		err := <-errReceiver
		assert.Nil(t, err)

		// check that the question-asking process has filled out the things we told it to
		assert.Equal(t, project.Name, options.ProjectName)
		assert.Equal(t, "CaC Project Alt Channel", options.ChannelName)
		assert.Equal(t, "27.9.999", options.Version)
		assert.Equal(t, "develop", options.GitReference) // not fully qualified but I guess we could hold that
		assert.Equal(t, "", options.GitCommit)
	})

	t.Run("standard process asking for everything (no package versions); specific git commit set which is passed to the server", func(t *testing.T) {
		api, qa := testutil.NewMockServerAndAsker()

		options := &executor.TaskOptionsCreateRelease{}
		options.GitCommit = "45c508a"

		errReceiver := testutil.GoBegin(func() error {
			defer testutil.Close(api, qa)
			octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
			return create.AskQuestions(octopus, qa.AsAsker(), spinner, options)
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{project})

		qa.ExpectQuestion(t, &survey.Select{
			Message: "Select the project in which the release will be created",
			Options: []string{project.Name},
		}).AnswerWith(project.Name)

		// CLI will load all the branches and tags
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/git/branches").RespondWith(resources.Resources[*projects.GitReference]{
			PagedResults: resources.PagedResults{ItemType: "GitBranch"},
			Items: []*projects.GitReference{
				projects.NewGitBranchReference("main", "refs/heads/main"),
				projects.NewGitBranchReference("develop", "refs/heads/develop"),
			}})

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/git/tags").RespondWith(resources.Resources[*projects.GitReference]{
			PagedResults: resources.PagedResults{ItemType: "GitTag"},
			Items: []*projects.GitReference{
				projects.NewGitTagReference("v2", "refs/tags/v2"),
				projects.NewGitTagReference("v1", "refs/tags/v1"),
			}})

		// NOTE we still ask for git ref even though commit is specified, this is so the server
		// can give us nice audit logs capturing the INTENT of the release (a commit may exist in more than one branch)
		qa.ExpectQuestion(t, &survey.Select{
			Message: "Select the Git Reference to use",
			Options: []string{"main (Branch)", "develop (Branch)", "v2 (Tag)", "v1 (Tag)"},
		}).AnswerWith("v2 (Tag)")

		// Deployment Processes/Templates under CaC always contain the same ID (deploymentprocess-Projects-423) but
		// the URL can change to be git-commit specific, e.g. api/Spaces-1/projects/Projects-423/cfdd4bd/deploymentprocesses or api/Spaces-1/projects/Projects-423/main/deploymentprocesses
		// this means we don't have to change our project.DeploymentProcessID when we're fiddling with this.
		depProcessSpecificCommit := fixtures.NewDeploymentProcessForVersionControlledProject(spaceID, projectID, "45c508a")

		// it uses the git commit hash regardless of which branch we picked
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/45c508a/deploymentprocesses").RespondWith(depProcessSpecificCommit)

		// next phase; channel selection

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/channels").RespondWith(resources.Resources[*channels.Channel]{
			Items: []*channels.Channel{defaultChannel, altChannel},
		})
		qa.ExpectQuestion(t, &survey.Select{
			Message: "Select the channel in which the release will be created",
			Options: []string{defaultChannel.Name, altChannel.Name},
		}).AnswerWith(altChannel.Name)

		// always loads dep process template
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/45c508a/deploymentprocesses/template?channel="+altChannel.ID).RespondWith(depTemplate)

		// our project inline versioning strategy was nil, so the code needs to load the deployment settings to find out
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/45c508a/deploymentsettings").RespondWith(depSettings)

		qa.ExpectQuestion(t, &survey.Input{
			Message: "Release Version",
			Default: "27.9.3", // from the dep template
		}).AnswerWith("27.9.654")

		err := <-errReceiver
		assert.Nil(t, err)

		// check that the question-asking process has filled out the things we told it to
		assert.Equal(t, project.Name, options.ProjectName)
		assert.Equal(t, "CaC Project Alt Channel", options.ChannelName)
		assert.Equal(t, "27.9.654", options.Version)
		assert.Equal(t, "v2", options.GitReference) // not fully qualified but I guess we could hold that
		assert.Equal(t, "45c508a", options.GitCommit)
	})

	t.Run("standard process asking for everything (no package versions); doesn't ask for git ref if already specified", func(t *testing.T) {
		api, qa := testutil.NewMockServerAndAsker()

		options := &executor.TaskOptionsCreateRelease{}
		options.GitReference = "develop"

		errReceiver := testutil.GoBegin(func() error {
			defer testutil.Close(api, qa)
			octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
			return create.AskQuestions(octopus, qa.AsAsker(), spinner, options)
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{project})

		qa.ExpectQuestion(t, &survey.Select{
			Message: "Select the project in which the release will be created",
			Options: []string{project.Name},
		}).AnswerWith(project.Name)

		// it uses the git commit hash regardless of which branch we picked
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/develop/deploymentprocesses").RespondWith(depProcessDevelopBranch)

		// next phase; channel selection

		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/channels").RespondWith(resources.Resources[*channels.Channel]{
			Items: []*channels.Channel{defaultChannel, altChannel},
		})
		qa.ExpectQuestion(t, &survey.Select{
			Message: "Select the channel in which the release will be created",
			Options: []string{defaultChannel.Name, altChannel.Name},
		}).AnswerWith(altChannel.Name)

		// always loads dep process template
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/develop/deploymentprocesses/template?channel="+altChannel.ID).RespondWith(depTemplate)

		// our project inline versioning strategy was nil, so the code needs to load the deployment settings to find out
		api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/develop/deploymentsettings").RespondWith(depSettings)

		qa.ExpectQuestion(t, &survey.Input{
			Message: "Release Version",
			Default: "27.9.3", // from the dep template
		}).AnswerWith("27.9.654")

		err := <-errReceiver
		assert.Nil(t, err)

		// check that the question-asking process has filled out the things we told it to
		assert.Equal(t, project.Name, options.ProjectName)
		assert.Equal(t, "CaC Project Alt Channel", options.ChannelName)
		assert.Equal(t, "27.9.654", options.Version)
		assert.Equal(t, "develop", options.GitReference)
		assert.Equal(t, "", options.GitCommit)
	})
}

// These tests ensure that given the right input, we call the server's API appropriately
// they all run in automation mode where survey is disabled; they'd error if they tried to ask questions
func TestReleaseCreate_AutomationMode(t *testing.T) {
	fakeRepoUrl, _ := url.Parse("https://gitserver/repo")

	const cacProjectID = "Projects-92"

	space1 := fixtures.NewSpace("Spaces-1", "Default Space")

	depProcess := fixtures.NewDeploymentProcessForProject(space1.ID, cacProjectID)

	cacProject := fixtures.NewProject(space1.ID, cacProjectID, "CaC Project", "Lifecycles-1", "ProjectGroups-1", depProcess.ID)
	cacProject.PersistenceSettings = projects.NewGitPersistenceSettings(
		".octopus",
		projects.NewAnonymousGitCredential(),
		"main",
		fakeRepoUrl,
	)

	t.Run("release creation requires a project name", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		root, stdOut, stdErr := fixtures.NewCobraRootCommand(testutil.NewMockFactoryWithSpace(api, space1))

		cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
			defer api.Close()
			root.SetArgs([]string{"release", "create"})
			return root.ExecuteC()
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

		_, err := testutil.ReceivePair(cmdReceiver)
		assert.EqualError(t, err, "project must be specified")

		assert.Equal(t, "", stdOut.String())
		// At first glance it may appear a bit weird that stdErr doesn't contain the error message here.
		// This is fine though, the main program entrypoint prints any errors that bubble up to it.
		assert.Equal(t, "", stdErr.String())
	})

	t.Run("release creation specifying project only (bare minimum)", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		root, stdOut, stdErr := fixtures.NewCobraRootCommand(testutil.NewMockFactoryWithSpace(api, space1))

		cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
			defer api.Close()
			root.SetArgs([]string{"release", "create", "--project", cacProject.Name})
			return root.ExecuteC()
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

		req := api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/create/v1")

		// check that it sent the server the right request body
		requestBody, err := testutil.ReadJson[releases.CreateReleaseV1](req.Request.Body)
		assert.Nil(t, err)

		assert.Equal(t, releases.CreateReleaseV1{
			SpaceIDOrName:   "Spaces-1",
			ProjectIDOrName: cacProject.Name,
		}, requestBody)

		req.RespondWith(&releases.CreateReleaseResponseV1{
			ReleaseID:      "Releases-999", // new release
			ReleaseVersion: "1.2.3",
		})

		// after it creates the release it's going to go back to the server and lookup the release by its ID
		// so it can tell the user what channel got selected
		releaseInfo := releases.NewRelease("Channels-32", cacProject.ID, "1.2.3")
		api.ExpectRequest(t, "GET", "/api/Spaces-1/releases/Releases-999").RespondWith(releaseInfo)

		// and now it wants to lookup the channel name too
		channelInfo := fixtures.NewChannel(space1.ID, "Channels-32", "Alpha channel", cacProject.ID)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Channels-32").RespondWith(channelInfo)

		_, err = testutil.ReceivePair(cmdReceiver)
		assert.Nil(t, err)

		assert.Equal(t, "Successfully created release version 1.2.3 (Releases-999) using channel Alpha channel (Channels-32)\n", stdOut.String())
		assert.Equal(t, "", stdErr.String())
	})

	t.Run("release creation specifying gitcommit and gitref", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		root, stdOut, stdErr := fixtures.NewCobraRootCommand(testutil.NewMockFactoryWithSpace(api, space1))

		cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
			defer api.Close()
			root.SetArgs([]string{"release", "create", "--project", cacProject.Name, "--git-ref", "refs/heads/main", "--git-commit", "6ef5e8c83cdcd4933bbeaeb458dc99902ad831ca"})
			return root.ExecuteC()
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

		req := api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/create/v1")

		// check that it sent the server the right request body
		requestBody, err := testutil.ReadJson[releases.CreateReleaseV1](req.Request.Body)
		assert.Nil(t, err)

		assert.Equal(t, releases.CreateReleaseV1{
			SpaceIDOrName:   "Spaces-1",
			ProjectIDOrName: cacProject.Name,
			GitCommit:       "6ef5e8c83cdcd4933bbeaeb458dc99902ad831ca",
			GitRef:          "refs/heads/main",
		}, requestBody)

		req.RespondWith(&releases.CreateReleaseResponseV1{
			ReleaseID:      "Releases-999", // new release
			ReleaseVersion: "1.2.3",
		})

		releaseInfo := releases.NewRelease("Channels-32", cacProject.ID, "1.2.3")
		api.ExpectRequest(t, "GET", "/api/Spaces-1/releases/Releases-999").RespondWith(releaseInfo)

		channelInfo := fixtures.NewChannel(space1.ID, "Channels-32", "Alpha channel", cacProject.ID)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Channels-32").RespondWith(channelInfo)

		_, err = testutil.ReceivePair(cmdReceiver)
		assert.Nil(t, err)

		assert.Equal(t, "Successfully created release version 1.2.3 (Releases-999) using channel Alpha channel (Channels-32)\n", stdOut.String())
		assert.Equal(t, "", stdErr.String())
	})

	t.Run("release creation with all the flags", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		root, stdOut, stdErr := fixtures.NewCobraRootCommand(testutil.NewMockFactoryWithSpace(api, space1))

		cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
			defer api.Close()
			root.SetArgs([]string{"release", "create",
				"--project", cacProject.Name,
				"--package-version", "5.6.7-beta",
				"--git-ref", "refs/heads/main",
				"--git-commit", "6ef5e8c83cdcd4933bbeaeb458dc99902ad831ca",
				"--version", "1.0.2",
				"--channel", "BetaChannel",
				"--release-notes", "Here are some **release notes**.",
			})
			return root.ExecuteC()
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

		req := api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/create/v1")

		// check that it sent the server the right request body
		requestBody, err := testutil.ReadJson[releases.CreateReleaseV1](req.Request.Body)
		assert.Nil(t, err)

		assert.Equal(t, releases.CreateReleaseV1{
			SpaceIDOrName:         "Spaces-1",
			ProjectIDOrName:       cacProject.Name,
			PackageVersion:        "5.6.7-beta",
			GitCommit:             "6ef5e8c83cdcd4933bbeaeb458dc99902ad831ca",
			GitRef:                "refs/heads/main",
			ReleaseVersion:        "1.0.2",
			ChannelIDOrName:       "BetaChannel",
			ReleaseNotes:          "Here are some **release notes**.",
			IgnoreIfAlreadyExists: false,
			IgnoreChannelRules:    false,
			PackagePrerelease:     "",
		}, requestBody)

		// this isn't realistic, we asked for version 1.0.2 and channel Beta, but it proves that
		// if the server changes its mind and uses a different channel, the CLI will show that.
		req.RespondWith(&releases.CreateReleaseResponseV1{
			ReleaseID:      "Releases-999", // new release
			ReleaseVersion: "1.0.5",
		})

		// If we GET on the endpoint and it shows us a different ReleaseVersion than the CreateReleaseResponseV1
		// does, CreateReleaseResponseV1 wins
		releaseInfo := releases.NewRelease("Channels-32", cacProject.ID, "1.2.3")
		api.ExpectRequest(t, "GET", "/api/Spaces-1/releases/Releases-999").RespondWith(releaseInfo)

		channelInfo := fixtures.NewChannel(space1.ID, "Channels-32", "Alpha channel", cacProject.ID)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Channels-32").RespondWith(channelInfo)

		_, err = testutil.ReceivePair(cmdReceiver)
		assert.Nil(t, err)

		assert.Equal(t, "Successfully created release version 1.0.5 (Releases-999) using channel Alpha channel (Channels-32)\n", stdOut.String())
		assert.Equal(t, "", stdErr.String())
	})

	t.Run("release creation with all the flags (short flags where available)", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		root, stdOut, stdErr := fixtures.NewCobraRootCommand(testutil.NewMockFactoryWithSpace(api, space1))

		cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
			defer api.Close()
			root.SetArgs([]string{"release", "create",
				"-p", cacProject.Name,
				"--package-version", "5.6.7-beta",
				"-r", "refs/heads/main",
				"--git-commit", "6ef5e8c83cdcd4933bbeaeb458dc99902ad831ca",
				"-v", "1.0.2",
				"-c", "BetaChannel",
				"-n", "Here are some **release notes**.",
			})
			return root.ExecuteC()
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

		req := api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/create/v1")

		// check that it sent the server the right request body
		requestBody, err := testutil.ReadJson[releases.CreateReleaseV1](req.Request.Body)
		assert.Nil(t, err)

		assert.Equal(t, releases.CreateReleaseV1{
			SpaceIDOrName:         "Spaces-1",
			ProjectIDOrName:       cacProject.Name,
			PackageVersion:        "5.6.7-beta",
			GitCommit:             "6ef5e8c83cdcd4933bbeaeb458dc99902ad831ca",
			GitRef:                "refs/heads/main",
			ReleaseVersion:        "1.0.2",
			ChannelIDOrName:       "BetaChannel",
			ReleaseNotes:          "Here are some **release notes**.",
			IgnoreIfAlreadyExists: false,
			IgnoreChannelRules:    false,
			PackagePrerelease:     "",
		}, requestBody)

		// this isn't realistic, we asked for version 1.0.2 and channel Beta, but it proves that
		// if the server changes its mind and uses a different channel, the CLI will show that.
		req.RespondWith(&releases.CreateReleaseResponseV1{
			ReleaseID:      "Releases-999", // new release
			ReleaseVersion: "1.0.5",
		})

		// If we GET on the endpoint and it shows us a different ReleaseVersion than the CreateReleaseResponseV1
		// does, CreateReleaseResponseV1 wins
		releaseInfo := releases.NewRelease("Channels-32", cacProject.ID, "1.2.3")
		api.ExpectRequest(t, "GET", "/api/Spaces-1/releases/Releases-999").RespondWith(releaseInfo)

		channelInfo := fixtures.NewChannel(space1.ID, "Channels-32", "Alpha channel", cacProject.ID)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Channels-32").RespondWith(channelInfo)

		_, err = testutil.ReceivePair(cmdReceiver)
		assert.Nil(t, err)

		assert.Equal(t, "Successfully created release version 1.0.5 (Releases-999) using channel Alpha channel (Channels-32)\n", stdOut.String())
		assert.Equal(t, "", stdErr.String())
	})
}

// this is technically internal to AskQuestions, but the complexity is high enough it's better to extract it out and
// test it individually
func TestReleaseCreate_BuildPackageVersionBaseline(t *testing.T) {

	spaceID := "Spaces-1"
	builtinFeedID := "feeds-builtin"
	externalFeedID := "Feeds-1001"

	t.Run("builds empty list for no packages", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		processTemplate := &deployments.DeploymentProcessTemplate{
			Packages: nil,
			Resource: resources.Resource{},
		}

		channel := fixtures.NewChannel(spaceID, "Channels-1", "Default", "Projects-1")

		receiver := testutil.GoBegin2(func() ([]*create.StepPackageVersion, error) {
			defer api.Close()
			octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
			return create.BuildPackageVersionBaseline(octopus, processTemplate, channel)
		})

		// octopusApiClient.NewClient fetches the root resource but otherwise BuildPackageVersionBaseline does nothing
		api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

		packageVersions, err := testutil.ReceivePair(receiver)
		assert.Nil(t, err)
		assert.Equal(t, []*create.StepPackageVersion{}, packageVersions)
	})

	t.Run("builds list for single package/step", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		processTemplate := &deployments.DeploymentProcessTemplate{
			Packages: []releases.ReleaseTemplatePackage{
				{
					ActionName:           "Install",
					FeedID:               builtinFeedID,
					PackageID:            "pterm",
					PackageReferenceName: "pterm-on-install",
					StepName:             "Install",
				},
			},
			Resource: resources.Resource{},
		}

		channel := fixtures.NewChannel(spaceID, "Channels-1", "Default", "Projects-1")

		receiver := testutil.GoBegin2(func() ([]*create.StepPackageVersion, error) {
			defer api.Close()
			octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
			return create.BuildPackageVersionBaseline(octopus, processTemplate, channel)
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

		// it needs to load the feeds to find the links
		api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds?ids=feeds-builtin&take=1").RespondWith(&feeds.Feeds{Items: []feeds.IFeed{
			&feeds.FeedResource{Name: "Builtin", FeedType: feeds.FeedTypeBuiltIn, Resource: resources.Resource{
				ID: builtinFeedID,
				Links: map[string]string{
					constants.LinkSearchPackageVersionsTemplate: "/api/Spaces-1/feeds/feeds-builtin/packages/versions{?packageId,take,skip,includePreRelease,versionRange,preReleaseTag,filter,includeReleaseNotes}",
				}}},
		}})

		// now it will search for the package versions
		api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds/feeds-builtin/packages/versions?packageId=pterm&take=1").RespondWith(&resources.Resources[*packages.PackageVersion]{
			Items: []*packages.PackageVersion{
				{PackageID: "pterm", Version: "0.12.51"},
			},
		})

		packageVersions, err := testutil.ReceivePair(receiver)
		assert.Nil(t, err)
		assert.Equal(t, []*create.StepPackageVersion{
			{
				PackageID: "pterm",
				StepName:  "Install",
				Version:   "0.12.51",
				FeedID:    "feeds-builtin",
			},
		}, packageVersions)
	})

	t.Run("builds list for multiple package/steps with some overlapping packages; no duplicate requests sent to server", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		processTemplate := &deployments.DeploymentProcessTemplate{
			Packages: []releases.ReleaseTemplatePackage{
				{
					ActionName:           "Install",
					FeedID:               builtinFeedID,
					PackageID:            "pterm",
					PackageReferenceName: "pterm-on-install",
					StepName:             "Install",
				},
				{
					ActionName:           "Install",
					FeedID:               externalFeedID,
					PackageID:            "NuGet.CommandLine",
					PackageReferenceName: "nuget-on-install",
					StepName:             "Install",
				},
				{
					ActionName:           "Verify",
					FeedID:               builtinFeedID,
					PackageID:            "pterm",
					PackageReferenceName: "pterm-on-verify",
					StepName:             "Verify",
				},
				{
					ActionName:           "Cleanup",
					FeedID:               externalFeedID,
					PackageID:            "NuGet.CommandLine",
					PackageReferenceName: "nuget-on-cleanup",
					StepName:             "Cleanup",
				},
			},
			Resource: resources.Resource{},
		}

		channel := fixtures.NewChannel(spaceID, "Channels-1", "Default", "Projects-1")

		receiver := testutil.GoBegin2(func() ([]*create.StepPackageVersion, error) {
			defer api.Close()
			octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
			return create.BuildPackageVersionBaseline(octopus, processTemplate, channel)
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

		// it needs to load the feeds to find the links
		api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds?ids=Feeds-1001&ids=feeds-builtin&take=2").RespondWith(&feeds.Feeds{Items: []feeds.IFeed{
			&feeds.FeedResource{Name: "Builtin", FeedType: feeds.FeedTypeBuiltIn, Resource: resources.Resource{
				ID: builtinFeedID,
				Links: map[string]string{
					constants.LinkSearchPackageVersionsTemplate: "/api/Spaces-1/feeds/feeds-builtin/packages/versions{?packageId,take,skip,includePreRelease,versionRange,preReleaseTag,filter,includeReleaseNotes}",
				}}},
			&feeds.FeedResource{Name: "External Nuget", FeedType: feeds.FeedTypeNuGet, Resource: resources.Resource{
				ID: externalFeedID,
				Links: map[string]string{
					constants.LinkSearchPackageVersionsTemplate: "/api/Spaces-1/feeds/Feeds-1001/packages/versions{?packageId,take,skip,includePreRelease,versionRange,preReleaseTag,filter,includeReleaseNotes}",
				}}},
		}})

		// now it will search for the package versions
		api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds/feeds-builtin/packages/versions?packageId=pterm&take=1").RespondWith(&resources.Resources[*packages.PackageVersion]{
			Items: []*packages.PackageVersion{
				{PackageID: "pterm", Version: "0.12.51"},
			},
		})
		// even though two steps use pterm, they're the same so we don't need to ask the server twice
		api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds/Feeds-1001/packages/versions?packageId=NuGet.CommandLine&take=1").RespondWith(&resources.Resources[*packages.PackageVersion]{
			Items: []*packages.PackageVersion{
				{PackageID: "NuGet.CommandLine", Version: "6.1.2"},
			},
		})
		// even though two steps use nuget, they're the same so we don't need to ask the server twice

		packageVersions, err := testutil.ReceivePair(receiver)
		assert.Nil(t, err)
		assert.Equal(t, []*create.StepPackageVersion{
			{
				PackageID: "pterm",
				StepName:  "Install",
				Version:   "0.12.51",
				FeedID:    builtinFeedID,
			},
			{
				PackageID: "pterm",
				StepName:  "Verify",
				Version:   "0.12.51",
				FeedID:    builtinFeedID,
			},
			{
				PackageID: "NuGet.CommandLine",
				StepName:  "Install",
				Version:   "6.1.2",
				FeedID:    externalFeedID,
			},
			{
				PackageID: "NuGet.CommandLine",
				StepName:  "Cleanup",
				Version:   "6.1.2",
				FeedID:    externalFeedID,
			},
		}, packageVersions)
	})

	t.Run("builds list for multiple package/steps with some overlapping packages where channel rules call for differing versions", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		processTemplate := &deployments.DeploymentProcessTemplate{
			Packages: []releases.ReleaseTemplatePackage{
				{
					ActionName:           "Install",
					FeedID:               builtinFeedID,
					PackageID:            "pterm",
					PackageReferenceName: "pterm-on-install",
					StepName:             "Install",
				},
				{
					ActionName:           "Install",
					FeedID:               externalFeedID,
					PackageID:            "NuGet.CommandLine",
					PackageReferenceName: "nuget-on-install",
					StepName:             "Install",
				},
				{
					ActionName:           "Verify",
					FeedID:               builtinFeedID,
					PackageID:            "pterm",
					PackageReferenceName: "pterm-on-verify",
					StepName:             "Verify",
				},
				{
					ActionName:           "Cleanup",
					FeedID:               externalFeedID,
					PackageID:            "NuGet.CommandLine", // channel rule is going to say that this one should have a different version
					PackageReferenceName: "nuget-on-cleanup",
					StepName:             "Cleanup",
				},
			},
			Resource: resources.Resource{},
		}

		channel := fixtures.NewChannel(spaceID, "Channels-1", "Default", "Projects-1")
		channel.Rules = []channels.ChannelRule{
			{
				Tag:          "^pre$",
				VersionRange: "[5.0,6.0)",
				ActionPackages: []packages.DeploymentActionPackage{
					{DeploymentAction: "Install", PackageReference: "pterm-on-NOSUCHSTEP"}, // this should be ignored as PackageReference doesn't match
					{DeploymentAction: "Cleanup", PackageReference: "nuget-on-cleanup"},    // this should match
				},
			},

			{
				Tag:          "^$",
				VersionRange: "[9.9]",
				ActionPackages: []packages.DeploymentActionPackage{
					{DeploymentAction: "InstallXYZ", PackageReference: "pterm-on-install"}, // this should be ignored as DeploymentAction doesn't match
				},
			},
		}

		receiver := testutil.GoBegin2(func() ([]*create.StepPackageVersion, error) {
			defer api.Close()
			octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
			return create.BuildPackageVersionBaseline(octopus, processTemplate, channel)
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

		// it needs to load the feeds to find the links
		api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds?ids=Feeds-1001&ids=feeds-builtin&take=2").RespondWith(&feeds.Feeds{Items: []feeds.IFeed{
			&feeds.FeedResource{Name: "Builtin", FeedType: feeds.FeedTypeBuiltIn, Resource: resources.Resource{
				ID: builtinFeedID,
				Links: map[string]string{
					constants.LinkSearchPackageVersionsTemplate: "/api/Spaces-1/feeds/feeds-builtin/packages/versions{?packageId,take,skip,includePreRelease,versionRange,preReleaseTag,filter,includeReleaseNotes}",
				}}},
			&feeds.FeedResource{Name: "External Nuget", FeedType: feeds.FeedTypeNuGet, Resource: resources.Resource{
				ID: externalFeedID,
				Links: map[string]string{
					constants.LinkSearchPackageVersionsTemplate: "/api/Spaces-1/feeds/Feeds-1001/packages/versions{?packageId,take,skip,includePreRelease,versionRange,preReleaseTag,filter,includeReleaseNotes}",
				}}},
		}})

		// now it will search for the package versions
		api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds/feeds-builtin/packages/versions?packageId=pterm&take=1").RespondWith(&resources.Resources[*packages.PackageVersion]{
			Items: []*packages.PackageVersion{
				{PackageID: "pterm", Version: "0.12.51"},
			},
		})
		// even though two steps use pterm, they're the same so we don't need to ask the server twice
		api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds/Feeds-1001/packages/versions?packageId=NuGet.CommandLine&take=1").RespondWith(&resources.Resources[*packages.PackageVersion]{
			Items: []*packages.PackageVersion{
				{PackageID: "NuGet.CommandLine", Version: "6.1.2"},
			},
		})
		// second request asks for different filters due to channel rules
		api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds/Feeds-1001/packages/versions?packageId=NuGet.CommandLine&preReleaseTag=%5Epre%24&take=1&versionRange=%5B5.0%2C6.0%29").RespondWith(&resources.Resources[*packages.PackageVersion]{
			Items: []*packages.PackageVersion{
				{PackageID: "NuGet.CommandLine", Version: "5.4.1-prerelease"},
			},
		})

		packageVersions, err := testutil.ReceivePair(receiver)
		assert.Nil(t, err)
		assert.Equal(t, []*create.StepPackageVersion{
			{
				PackageID: "pterm",
				StepName:  "Install",
				Version:   "0.12.51",
				FeedID:    builtinFeedID,
			},
			{
				PackageID: "pterm",
				StepName:  "Verify",
				Version:   "0.12.51",
				FeedID:    builtinFeedID,
			},
			{
				PackageID: "NuGet.CommandLine",
				StepName:  "Install",
				Version:   "6.1.2",
				FeedID:    externalFeedID,
			},
			{
				PackageID: "NuGet.CommandLine",
				StepName:  "Cleanup",
				Version:   "5.4.1-prerelease",
				FeedID:    externalFeedID,
			},
		}, packageVersions)
	})

}
