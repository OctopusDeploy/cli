package create_test

import (
	"bytes"
	"errors"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/release/create"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/executor"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/constants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/credentials"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/feeds"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/packages"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/url"
	"os"
	"testing"
)

var serverUrl, _ = url.Parse("http://server")

const placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
const packageOverrideQuestion = "Package override string (y to accept, u to undo, ? for help):"

var spinner = &testutil.FakeSpinner{}

var rootResource = testutil.NewRootResource()

func TestReleaseCreate_AskQuestions_RegularProject(t *testing.T) {
	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"

	depProcess := fixtures.NewDeploymentProcessForProject(spaceID, fireProjectID)

	defaultChannel := fixtures.NewChannel(spaceID, "Channels-1", "Fire Project Default Channel", fireProjectID)
	altChannel := fixtures.NewChannel(spaceID, "Channels-97", "Fire Project Alt Channel", fireProjectID)

	fireProject := fixtures.NewProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", depProcess.ID)

	// this horrible pattern is how you implement "beforeEach" in go testing
	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer)
	}{
		{"standard process asking for everything including release notes; no packages, release version from template", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsCreateRelease{}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				// NewClient makes network calls so we have to run it in the goroutine
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return create.AskQuestions(octopus, stdout, qa.AsAsker(), options)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{fireProject})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the project in which the release will be created",
				Options: []string{"Fire Project"},
			}).AnswerWith("Fire Project")

			api.ExpectRequest(t, "GET", "/api/Spaces-1/deploymentprocesses/deploymentprocess-"+fireProjectID).RespondWith(depProcess)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/channels").RespondWith(resources.Resources[*channels.Channel]{
				Items: []*channels.Channel{defaultChannel, altChannel},
			})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the channel in which the release will be created",
				Options: []string{defaultChannel.Name, altChannel.Name},
			}).AnswerWith("Fire Project Alt Channel")

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/deploymentprocesses/template?channel=Channels-97").
				RespondWith(&deployments.DeploymentProcessTemplate{NextVersionIncrement: "27.9.3"})

			// here is where the package version prompt goes; except we have no packages so it skips it

			_ = qa.ExpectQuestion(t, &survey.Input{
				Message: "Release Version",
				Default: "27.9.3",
			}).AnswerWith("27.9.999")

			_ = qa.ExpectQuestion(t, &surveyext.OctoEditor{
				Editor: &survey.Editor{
					Message:  "Release Notes",
					Help:     "You may optionally add notes to the release using Markdown.",
					FileName: "*.md",
				},
				Optional: true,
			}).AnswerWith("## some release notes")

			err := <-errReceiver
			assert.Nil(t, err)

			// check that the question-asking process has filled out the things we told it to
			assert.Equal(t, "Fire Project", options.ProjectName)
			assert.Equal(t, "Fire Project Alt Channel", options.ChannelName)
			assert.Equal(t, "27.9.999", options.Version)
			assert.Equal(t, "## some release notes", options.ReleaseNotes)
		}},

		{"asking for nothing in interactive mode; no packages, release version specified", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsCreateRelease{
				ProjectName:  "fire project",
				ChannelName:  "fire project default channel",
				Version:      "9.8.4-prerelease",
				ReleaseNotes: "already have release notes",
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return create.AskQuestions(octopus, stdout, qa.AsAsker(), options)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/fire project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=fire+project").
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
			assert.Equal(t, "already have release notes", options.ReleaseNotes)
		}},

		{"asking for release version based on template; packages exist", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsCreateRelease{
				ProjectName:  "fire project",
				ChannelName:  "fire project default channel",
				ReleaseNotes: "-",
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return create.AskQuestions(octopus, stdout, qa.AsAsker(), options)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/fire project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=fire+project").
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
				RespondWith(&deployments.DeploymentProcessTemplate{
					Packages: []releases.ReleaseTemplatePackage{
						{
							ActionName:           "Install",
							FeedID:               "feeds-builtin",
							PackageID:            "pterm",
							PackageReferenceName: "pterm-on-install",
							IsResolvable:         true,
						},
					},
					NextVersionIncrement: "27.9.33",
				})

			// we have some packages so it'll go looking for the feed
			api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds?ids=feeds-builtin&take=1").RespondWith(&feeds.Feeds{Items: []feeds.IFeed{
				&feeds.FeedResource{Name: "Builtin", FeedType: feeds.FeedTypeBuiltIn, Resource: resources.Resource{
					ID: "feeds-builtin",
					Links: map[string]string{
						constants.LinkSearchPackageVersionsTemplate: "/api/Spaces-1/feeds/feeds-builtin/packages/versions{?packageId,take,skip,includePreRelease,versionRange,preReleaseTag,filter,includeReleaseNotes}",
					}}},
			}})

			// then it will look for versions
			api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds/feeds-builtin/packages/versions?packageId=pterm&take=1").RespondWith(&resources.Resources[*packages.PackageVersion]{
				Items: []*packages.PackageVersion{{PackageID: "pterm", Version: "0.12.51"}},
			})

			_ = qa.ExpectQuestion(t, &survey.Input{
				Message: packageOverrideQuestion,
				Default: "",
			}).AnswerWith("y") // just accept all the packages; package loop is tested elsewhere

			_ = qa.ExpectQuestion(t, &survey.Input{
				Message: "Release Version",
				Default: "27.9.33",
			}).AnswerWith("30.0")

			err := <-errReceiver
			assert.Nil(t, err)

			// check that the question-asking process has filled out the things we told it to
			assert.Equal(t, "Fire Project", options.ProjectName)
			assert.Equal(t, "Fire Project Default Channel", options.ChannelName)
			assert.Equal(t, "30.0", options.Version)
			assert.Equal(t, heredoc.Doc(`
				Project Fire Project
				Channel Fire Project Default Channel
				PACKAGE  VERSION  STEP NAME/PACKAGE REFERENCE
				pterm    0.12.51  Install/pterm-on-install
				`), stdout.String())
		}},

		{"asking for release version based on donor package; package exists and dictates the release version - add metadata", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsCreateRelease{
				ProjectName:  "fire project",
				ChannelName:  "fire project default channel",
				ReleaseNotes: "-",
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return create.AskQuestions(octopus, stdout, qa.AsAsker(), options)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			var fireProject2 = *fireProject // clone the struct value
			fireProject2.VersioningStrategy = &projects.VersioningStrategy{
				DonorPackage: &packages.DeploymentActionPackage{
					DeploymentAction: "Verify",
					PackageReference: "nuget-on-verify",
				},
			}

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/fire project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=fire+project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{&fireProject2},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/deploymentprocesses/deploymentprocess-"+fireProjectID).RespondWith(depProcess)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/channels").
				RespondWith(resources.Resources[*channels.Channel]{
					Items: []*channels.Channel{defaultChannel},
				})

			// always loads the deployment process template to check for packages
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/deploymentprocesses/template?channel=Channels-1").
				RespondWith(&deployments.DeploymentProcessTemplate{
					Packages: []releases.ReleaseTemplatePackage{
						{
							ActionName:           "Install",
							FeedID:               "feeds-builtin",
							PackageID:            "pterm",
							PackageReferenceName: "pterm-on-install",
							IsResolvable:         true,
						},
						{
							ActionName:           "Verify",
							FeedID:               "feeds-builtin",
							PackageID:            "NuGet.CommandLine",
							PackageReferenceName: "nuget-on-verify",
							IsResolvable:         true,
						},
						{
							ActionName:           "Verify",
							FeedID:               "feeds-builtin",
							PackageID:            "pterm",
							PackageReferenceName: "pterm-on-verify",
							IsResolvable:         true,
						},
					},
					NextVersionIncrement: "27.9.33",
				})

			// we have some packages so it'll go looking for the feed
			api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds?ids=feeds-builtin&take=1").RespondWith(&feeds.Feeds{Items: []feeds.IFeed{
				&feeds.FeedResource{Name: "Builtin", FeedType: feeds.FeedTypeBuiltIn, Resource: resources.Resource{
					ID: "feeds-builtin",
					Links: map[string]string{
						constants.LinkSearchPackageVersionsTemplate: "/api/Spaces-1/feeds/feeds-builtin/packages/versions{?packageId,take,skip,includePreRelease,versionRange,preReleaseTag,filter,includeReleaseNotes}",
					}}},
			}})

			// then it will look for versions
			api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds/feeds-builtin/packages/versions?packageId=pterm&take=1").RespondWith(&resources.Resources[*packages.PackageVersion]{
				Items: []*packages.PackageVersion{{PackageID: "pterm", Version: "0.12.51"}}, // extra package to prove it's not just picking the first one
			})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds/feeds-builtin/packages/versions?packageId=NuGet.CommandLine&take=1").RespondWith(&resources.Resources[*packages.PackageVersion]{
				Items: []*packages.PackageVersion{{PackageID: "NuGet.CommandLine", Version: "6.2.1"}}, // the proper package
			})

			q := qa.ExpectQuestion(t, &survey.Input{
				Message: packageOverrideQuestion,
				Default: "",
			})
			assert.Equal(t, heredoc.Doc(`
				Project Fire Project
				Channel Fire Project Default Channel
				PACKAGE            VERSION  STEP NAME/PACKAGE REFERENCE
				pterm              0.12.51  Install/pterm-on-install
				pterm              0.12.51  Verify/pterm-on-verify
				NuGet.CommandLine  6.2.1    Verify/nuget-on-verify
				`), stdout.String())
			_ = q.AnswerWith("y") // just accept all the packages; package loop is tested elsewhere

			_ = qa.ExpectQuestion(t, &survey.Input{
				Message: "Release version 6.2.1 (from included package NuGet.CommandLine). Add metadata? (optional):",
				Default: "", // observing this value is the whole point of this test
			}).AnswerWith("bonanza")

			err := <-errReceiver
			assert.Nil(t, err)

			// check that the question-asking process has filled out the things we told it to
			assert.Equal(t, "Fire Project", options.ProjectName)
			assert.Equal(t, "Fire Project Default Channel", options.ChannelName)
			assert.Equal(t, "6.2.1+bonanza", options.Version)
		}},

		{"asking for release version based on donor package; package exists and dictates the release version - don't add metadata", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsCreateRelease{
				ProjectName:  "fire project",
				ChannelName:  "fire project default channel",
				ReleaseNotes: "-",
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return create.AskQuestions(octopus, stdout, qa.AsAsker(), options)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			var fireProject2 = *fireProject // clone the struct value
			fireProject2.VersioningStrategy = &projects.VersioningStrategy{
				DonorPackage: &packages.DeploymentActionPackage{
					DeploymentAction: "Verify",
					PackageReference: "nuget-on-verify",
				},
			}

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/fire project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=fire+project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{&fireProject2},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/deploymentprocesses/deploymentprocess-"+fireProjectID).RespondWith(depProcess)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/channels").
				RespondWith(resources.Resources[*channels.Channel]{
					Items: []*channels.Channel{defaultChannel},
				})

			// always loads the deployment process template to check for packages
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProjectID+"/deploymentprocesses/template?channel=Channels-1").
				RespondWith(&deployments.DeploymentProcessTemplate{
					Packages: []releases.ReleaseTemplatePackage{
						{
							ActionName:           "Verify",
							FeedID:               "feeds-builtin",
							PackageID:            "NuGet.CommandLine",
							PackageReferenceName: "nuget-on-verify",
							IsResolvable:         true,
						},
					},
				})

			// we have some packages so it'll go looking for the feed
			api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds?ids=feeds-builtin&take=1").RespondWith(&feeds.Feeds{Items: []feeds.IFeed{
				&feeds.FeedResource{Name: "Builtin", FeedType: feeds.FeedTypeBuiltIn, Resource: resources.Resource{
					ID: "feeds-builtin",
					Links: map[string]string{
						constants.LinkSearchPackageVersionsTemplate: "/api/Spaces-1/feeds/feeds-builtin/packages/versions{?packageId,take,skip,includePreRelease,versionRange,preReleaseTag,filter,includeReleaseNotes}",
					}}},
			}})

			// then it will look for versions
			api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds/feeds-builtin/packages/versions?packageId=NuGet.CommandLine&take=1").RespondWith(&resources.Resources[*packages.PackageVersion]{
				Items: []*packages.PackageVersion{{PackageID: "NuGet.CommandLine", Version: "6.2.1"}}, // the proper package
			})

			_ = qa.ExpectQuestion(t, &survey.Input{
				Message: packageOverrideQuestion,
				Default: "",
			}).AnswerWith("y") // just accept all the packages; package loop is tested elsewhere

			_ = qa.ExpectQuestion(t, &survey.Input{
				Message: "Release version 6.2.1 (from included package NuGet.CommandLine). Add metadata? (optional):",
				Default: "",
			}).AnswerWith("")

			err := <-errReceiver
			assert.Nil(t, err)

			// check that the question-asking process has filled out the things we told it to
			assert.Equal(t, "Fire Project", options.ProjectName)
			assert.Equal(t, "Fire Project Default Channel", options.ChannelName)
			assert.Equal(t, "6.2.1", options.Version)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api, qa := testutil.NewMockServerAndAsker()
			test.run(t, api, qa, new(bytes.Buffer))
		})
	}
}

func TestReleaseCreate_AskQuestions_VersionControlledProject(t *testing.T) {
	const spaceID = "Spaces-1"

	projectID := "Projects-87"
	depProcessDevelopBranch := fixtures.NewDeploymentProcessForVersionControlledProject(spaceID, projectID, "refs%2Fheads%2Fdevelop")

	depSettings := fixtures.NewDeploymentSettingsForProject(spaceID, projectID, &projects.VersioningStrategy{
		Template: "#{Octopus.Version.LastMajor}.#{Octopus.Version.LastMinor}.#{Octopus.Version.NextPatch}", // bog standard
	})
	depTemplate := &deployments.DeploymentProcessTemplate{NextVersionIncrement: "27.9.3"}

	project := fixtures.NewVersionControlledProject(spaceID, projectID, "CaC Project", "Lifecycles-1", "ProjectGroups-1", depProcessDevelopBranch.ID)

	defaultChannel := fixtures.NewChannel(spaceID, "Channels-34", "CaC Project Default Channel", projectID)
	altChannel := fixtures.NewChannel(spaceID, "Channels-97", "CaC Project Alt Channel", projectID)

	// this horrible pattern is how you implement "beforeEach" in go testing
	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer)
	}{
		{"standard process asking for everything; no packages, release version from template, specific git commit not set", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsCreateRelease{}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return create.AskQuestions(octopus, stdout, qa.AsAsker(), options)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{project})

			_ = qa.ExpectQuestion(t, &survey.Select{
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

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the Git Reference to use",
				Options: []string{"main (Branch)", "develop (Branch)", "v2 (Tag)", "v1 (Tag)"},
			}).AnswerWith("develop (Branch)")

			// can't specify a git commit hash in interactive mode

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/refs%2Fheads%2Fdevelop/deploymentprocesses").RespondWith(depProcessDevelopBranch)

			// next phase; channel selection

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/channels").RespondWith(resources.Resources[*channels.Channel]{
				Items: []*channels.Channel{defaultChannel, altChannel},
			})
			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the channel in which the release will be created",
				Options: []string{defaultChannel.Name, altChannel.Name},
			}).AnswerWith(altChannel.Name)

			// always loads dep process template
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/refs%2Fheads%2Fdevelop/deploymentprocesses/template?channel="+altChannel.ID).RespondWith(depTemplate)

			// our project inline versioning strategy was nil, so the code needs to load the deployment settings to find out
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/refs%2Fheads%2Fdevelop/deploymentsettings").RespondWith(depSettings)

			_ = qa.ExpectQuestion(t, &survey.Input{
				Message: "Release Version",
				Default: "27.9.3", // from the dep template
			}).AnswerWith("27.9.999")

			_ = qa.ExpectQuestion(t, &surveyext.OctoEditor{
				Editor: &survey.Editor{
					Message:  "Release Notes",
					Help:     "You may optionally add notes to the release using Markdown.",
					FileName: "*.md",
				},
				Optional: true,
			}).AnswerWith("## some release notes")

			err := <-errReceiver
			assert.Nil(t, err)

			// check that the question-asking process has filled out the things we told it to
			assert.Equal(t, project.Name, options.ProjectName)
			assert.Equal(t, "CaC Project Alt Channel", options.ChannelName)
			assert.Equal(t, "27.9.999", options.Version)
			assert.Equal(t, "refs/heads/develop", options.GitReference) // not fully qualified but I guess we could hold that
			assert.Equal(t, "", options.GitCommit)
			assert.Equal(t, "## some release notes", options.ReleaseNotes)
		}},

		{"standard process asking for everything; no packages, release version from template, specific git commit set which is passed to the server", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsCreateRelease{
				ReleaseNotes: "already tested release notes",
			}
			options.GitCommit = "45c508a"

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return create.AskQuestions(octopus, stdout, qa.AsAsker(), options)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{project})

			_ = qa.ExpectQuestion(t, &survey.Select{
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
			_ = qa.ExpectQuestion(t, &survey.Select{
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
			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the channel in which the release will be created",
				Options: []string{defaultChannel.Name, altChannel.Name},
			}).AnswerWith(altChannel.Name)

			// always loads dep process template
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/45c508a/deploymentprocesses/template?channel="+altChannel.ID).RespondWith(depTemplate)

			// our project inline versioning strategy was nil, so the code needs to load the deployment settings to find out
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/45c508a/deploymentsettings").RespondWith(depSettings)

			_ = qa.ExpectQuestion(t, &survey.Input{
				Message: "Release Version",
				Default: "27.9.3", // from the dep template
			}).AnswerWith("27.9.654")

			err := <-errReceiver
			assert.Nil(t, err)

			// check that the question-asking process has filled out the things we told it to
			assert.Equal(t, project.Name, options.ProjectName)
			assert.Equal(t, "CaC Project Alt Channel", options.ChannelName)
			assert.Equal(t, "27.9.654", options.Version)
			assert.Equal(t, "refs/tags/v2", options.GitReference) // not fully qualified but I guess we could hold that
			assert.Equal(t, "45c508a", options.GitCommit)
		}},

		{"standard process asking for everything; no packages, release version from template, doesn't ask for git ref if already specified", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			options := &executor.TaskOptionsCreateRelease{
				GitReference: "develop", // specifying a short name here not a fully qualified refs/heads/develop
				ReleaseNotes: "already tested release notes",
			}

			errReceiver := testutil.GoBegin(func() error {
				defer testutil.Close(api, qa)
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return create.AskQuestions(octopus, stdout, qa.AsAsker(), options)
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{project})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the project in which the release will be created",
				Options: []string{project.Name},
			}).AnswerWith(project.Name)

			// it uses the git commit hash regardless of which branch we picked
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/develop/deploymentprocesses").RespondWith(depProcessDevelopBranch)

			// next phase; channel selection

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/channels").RespondWith(resources.Resources[*channels.Channel]{
				Items: []*channels.Channel{defaultChannel, altChannel},
			})
			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the channel in which the release will be created",
				Options: []string{defaultChannel.Name, altChannel.Name},
			}).AnswerWith(altChannel.Name)

			// always loads dep process template
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/refs%2Fheads%2Fdevelop/deploymentprocesses/template?channel="+altChannel.ID).RespondWith(depTemplate)

			// our project inline versioning strategy was nil, so the code needs to load the deployment settings to find out
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+projectID+"/develop/deploymentsettings").RespondWith(depSettings)

			_ = qa.ExpectQuestion(t, &survey.Input{
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
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api, qa := testutil.NewMockServerAndAsker()
			test.run(t, api, qa, &bytes.Buffer{})
		})
	}
}

func TestReleaseCreate_AskQuestions_AskPackageOverrideLoop(t *testing.T) {

	baseline := []*create.StepPackageVersion{
		{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: "0.12"},
		{ActionName: "Install", PackageID: "NuGet.CommandLine", PackageReferenceName: "NuGet.CommandLine", Version: "6.1.2"},
		{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "0.12"},
	}

	tests := []struct {
		name string
		run  func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer)
	}{
		// this is the happy path where the CLI presents the list of server-selected packages and they just go 'yep'
		{"no-op test", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin3(func() ([]*create.StepPackageVersion, []*create.PackageVersionOverride, error) {
				return create.AskPackageOverrideLoop(baseline, "", make([]string, 0), qa.AsAsker(), stdout)
			})

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("y")

			versions, overrides, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Equal(t, baseline, versions)
			assert.Equal(t, make([]*create.PackageVersionOverride, 0), overrides)
		}},

		{"override package based on package ID", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin3(func() ([]*create.StepPackageVersion, []*create.PackageVersionOverride, error) {
				return create.AskPackageOverrideLoop(baseline, "", make([]string, 0), qa.AsAsker(), stdout)
			})

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("pterm:2.5")

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("y")

			versions, overrides, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*create.StepPackageVersion{
				{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: "2.5"},
				{ActionName: "Install", PackageID: "NuGet.CommandLine", PackageReferenceName: "NuGet.CommandLine", Version: "6.1.2"},
				{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "2.5"},
			}, versions)
			assert.Equal(t, []*create.PackageVersionOverride{
				{PackageID: "pterm", Version: "2.5"},
			}, overrides)
		}},

		{"override package based on step name", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin3(func() ([]*create.StepPackageVersion, []*create.PackageVersionOverride, error) {
				return create.AskPackageOverrideLoop(baseline, "", make([]string, 0), qa.AsAsker(), stdout)
			})

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("Install:2.5")

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("y")

			versions, overrides, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*create.StepPackageVersion{
				{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: "2.5"},
				{ActionName: "Install", PackageID: "NuGet.CommandLine", PackageReferenceName: "NuGet.CommandLine", Version: "2.5"},
				{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "0.12"},
			}, versions)
			assert.Equal(t, []*create.PackageVersionOverride{
				{ActionName: "Install", Version: "2.5"},
			}, overrides)
		}},

		{"override package based on package reference", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin3(func() ([]*create.StepPackageVersion, []*create.PackageVersionOverride, error) {
				return create.AskPackageOverrideLoop(baseline, "", make([]string, 0), qa.AsAsker(), stdout)
			})

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("Install:pterm:2.5")

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("y")

			versions, overrides, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*create.StepPackageVersion{
				{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: "2.5"},
				{ActionName: "Install", PackageID: "NuGet.CommandLine", PackageReferenceName: "NuGet.CommandLine", Version: "6.1.2"},
				{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "0.12"},
			}, versions)
			assert.Equal(t, []*create.PackageVersionOverride{
				{PackageReferenceName: "pterm", ActionName: "Install", Version: "2.5"},
			}, overrides)
		}},

		{"entering the loop with --package-version picked up from the command line", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			defaultPackageVersion := "2.5"

			receiver := testutil.GoBegin3(func() ([]*create.StepPackageVersion, []*create.PackageVersionOverride, error) {
				return create.AskPackageOverrideLoop(baseline, defaultPackageVersion, make([]string, 0), qa.AsAsker(), stdout)
			})

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("y")

			versions, overrides, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*create.StepPackageVersion{
				{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: "2.5"},
				{ActionName: "Install", PackageID: "NuGet.CommandLine", PackageReferenceName: "NuGet.CommandLine", Version: "2.5"},
				{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "2.5"},
			}, versions)
			assert.Equal(t, []*create.PackageVersionOverride{
				{Version: "2.5"}, // TODO the "regenerate command line flags" code is going to re-interpret this as "--package *:2.5" rather than the input which was "--package-version 2.5". Does that matter?
			}, overrides)
		}},

		{"entering the loop with --package picked up from the command line", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			cmdlinePackages := []string{"Install:pterm:2.5", "NuGet.CommandLine:7.1"}

			receiver := testutil.GoBegin3(func() ([]*create.StepPackageVersion, []*create.PackageVersionOverride, error) {
				return create.AskPackageOverrideLoop(baseline, "", cmdlinePackages, qa.AsAsker(), stdout)
			})

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("y")

			versions, overrides, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*create.StepPackageVersion{
				{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: "2.5"},
				{ActionName: "Install", PackageID: "NuGet.CommandLine", PackageReferenceName: "NuGet.CommandLine", Version: "7.1"},
				{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "0.12"},
			}, versions)
			assert.Equal(t, []*create.PackageVersionOverride{
				{PackageReferenceName: "pterm", ActionName: "Install", Version: "2.5"},
				{PackageID: "NuGet.CommandLine", Version: "7.1"},
			}, overrides)
		}},

		{"entering the loop with --package-version and --package(s) picked up from the command line", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			defaultPackageVersion := "9.9"
			cmdlinePackages := []string{"Install:pterm:2.5", "NuGet.CommandLine:7.1"}

			receiver := testutil.GoBegin3(func() ([]*create.StepPackageVersion, []*create.PackageVersionOverride, error) {
				return create.AskPackageOverrideLoop(baseline, defaultPackageVersion, cmdlinePackages, qa.AsAsker(), stdout)
			})

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("y")

			versions, overrides, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*create.StepPackageVersion{
				{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: "2.5"},
				{ActionName: "Install", PackageID: "NuGet.CommandLine", PackageReferenceName: "NuGet.CommandLine", Version: "7.1"},
				{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "9.9"},
			}, versions)
			assert.Equal(t, []*create.PackageVersionOverride{
				{Version: "9.9"}, // TODO the "regenerate command line flags" code is going to re-interpret this as "--package *:9.9" rather than the input which was "--package-version 9.9". Does that matter?
				{PackageReferenceName: "pterm", ActionName: "Install", Version: "2.5"},
				{PackageID: "NuGet.CommandLine", Version: "7.1"},
			}, overrides)
		}},

		{"blank answer retries the question", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin3(func() ([]*create.StepPackageVersion, []*create.PackageVersionOverride, error) {
				return create.AskPackageOverrideLoop(baseline, "", make([]string, 0), qa.AsAsker(), stdout)
			})

			validationErr := qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("")
			assert.Nil(t, validationErr)

			validationErr = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("")
			assert.Nil(t, validationErr)

			validationErr = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("y")
			assert.Nil(t, validationErr)

			versions, overrides, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Equal(t, baseline, versions)
			assert.Equal(t, make([]*create.PackageVersionOverride, 0), overrides)
		}},

		{"can't specify garbage; question loop retries", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin3(func() ([]*create.StepPackageVersion, []*create.PackageVersionOverride, error) {
				return create.AskPackageOverrideLoop(baseline, "", make([]string, 0), qa.AsAsker(), stdout)
			})

			q := qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion})

			validationErr := q.AnswerWith("fish") // not enough components
			assert.EqualError(t, validationErr, "package version specification \"fish\" does not use expected format")

			validationErr = q.AnswerWith("z:z:z:z") // too many components
			assert.EqualError(t, validationErr, "package version specification \"z:z:z:z\" does not use expected format")

			validationErr = q.AnswerWith("2.5") // can't just have a version with no :
			assert.EqualError(t, validationErr, "package version specification \"2.5\" does not use expected format")

			validationErr = q.AnswerWith("*:x2.5") // not valid semver(ish)
			assert.EqualError(t, validationErr, "version component \"x2.5\" is not a valid version")

			validationErr = q.AnswerWith("*:2.5") // answer properly this time
			assert.Nil(t, validationErr)

			// it'll ask again; y to confirm
			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("y") // confirm packages

			versions, overrides, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*create.StepPackageVersion{
				{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: "2.5"},
				{ActionName: "Install", PackageID: "NuGet.CommandLine", PackageReferenceName: "NuGet.CommandLine", Version: "2.5"},
				{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "2.5"},
			}, versions)
			assert.Equal(t, []*create.PackageVersionOverride{
				{Version: "2.5"},
			}, overrides)
		}},

		{"can't specify packages or steps that aren't there due to validator; question loop retries", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin3(func() ([]*create.StepPackageVersion, []*create.PackageVersionOverride, error) {
				return create.AskPackageOverrideLoop(baseline, "", make([]string, 0), qa.AsAsker(), stdout)
			})

			q := qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion})

			validationErr := q.AnswerWith("banana:2.5")
			assert.EqualError(t, validationErr, "could not resolve step name or package matching banana")

			validationErr = q.AnswerWith(":2.5") // ok answer properly this time, set everything to 2.5
			assert.Nil(t, validationErr)

			// it'll ask again; y to confirm
			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("y") // confirm packages

			versions, overrides, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*create.StepPackageVersion{
				{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: "2.5"},
				{ActionName: "Install", PackageID: "NuGet.CommandLine", PackageReferenceName: "NuGet.CommandLine", Version: "2.5"},
				{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "2.5"},
			}, versions)
			assert.Equal(t, []*create.PackageVersionOverride{
				{Version: "2.5"},
			}, overrides)
		}},

		{"question loop doesn't retry if it gets a hard error", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin3(func() ([]*create.StepPackageVersion, []*create.PackageVersionOverride, error) {
				return create.AskPackageOverrideLoop(baseline, "", make([]string, 0), qa.AsAsker(), stdout)
			})

			qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWithError(errors.New("hard fail"))

			versions, overrides, err := testutil.ReceiveTriple(receiver)
			assert.Equal(t, errors.New("hard fail"), err)
			assert.Nil(t, versions)
			assert.Nil(t, overrides)
		}},

		{"multiple overrides with undo", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin3(func() ([]*create.StepPackageVersion, []*create.PackageVersionOverride, error) {
				return create.AskPackageOverrideLoop(baseline, "", make([]string, 0), qa.AsAsker(), stdout)
			})

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("NuGet.CommandLine:7.1")

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("pterm:35")

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("u") // undo pterm:35

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("Install:pterm:2.5")

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("y")

			versions, overrides, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*create.StepPackageVersion{
				{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: "2.5"},
				{ActionName: "Install", PackageID: "NuGet.CommandLine", PackageReferenceName: "NuGet.CommandLine", Version: "7.1"},
				{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "0.12"}, // this would have been hit by pterm:35 but we undid it
			}, versions)
			assert.Equal(t, []*create.PackageVersionOverride{
				{PackageID: "NuGet.CommandLine", Version: "7.1"},
				{PackageReferenceName: "pterm", ActionName: "Install", Version: "2.5"},
			}, overrides)
		}},

		{"multiple overrides with reset", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			receiver := testutil.GoBegin3(func() ([]*create.StepPackageVersion, []*create.PackageVersionOverride, error) {
				return create.AskPackageOverrideLoop(baseline, "", make([]string, 0), qa.AsAsker(), stdout)
			})

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("NuGet.CommandLine:7.1")

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("pterm:35")

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("r") // undo pterm:35 and NuGet:CommandLine:7.1

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("Install:pterm:2.5")

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("y")

			versions, overrides, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*create.StepPackageVersion{
				{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: "2.5"},
				{ActionName: "Install", PackageID: "NuGet.CommandLine", PackageReferenceName: "NuGet.CommandLine", Version: "6.1.2"},
				{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "0.12"}, // this would have been hit by pterm:35 but we undid it
			}, versions)
			assert.Equal(t, []*create.PackageVersionOverride{
				{PackageReferenceName: "pterm", ActionName: "Install", Version: "2.5"},
			}, overrides)
		}},

		// this is the happy path where the CLI presents the list of server-selected packages and they just go 'yep'
		{"if we enter the loop with any unresolved packages, force version selection for them before entering the main loop", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			baselineSomeUnresolved := []*create.StepPackageVersion{
				{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: ""},                         // unresolved
				{ActionName: "Install", PackageID: "NuGet.CommandLine", PackageReferenceName: "NuGet.CommandLine", Version: ""}, // unresolved
				{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "0.12"},
			}

			receiver := testutil.GoBegin3(func() ([]*create.StepPackageVersion, []*create.PackageVersionOverride, error) {
				return create.AskPackageOverrideLoop(baselineSomeUnresolved, "", make([]string, 0), qa.AsAsker(), stdout)
			})

			q := qa.ExpectQuestion(t, &survey.Input{Message: "Unable to find a version for \"pterm\". Specify a version:"})
			assert.Equal(t, heredoc.Doc(`
				PACKAGE            VERSION  STEP NAME/PACKAGE REFERENCE
				pterm              unknown  Install/pterm
				NuGet.CommandLine  unknown  Install/NuGet.CommandLine
				pterm              0.12     Verify/pterm
			`), stdout.String())
			stdout.Reset()
			_ = q.AnswerWith("75")

			q = qa.ExpectQuestion(t, &survey.Input{Message: "Unable to find a version for \"NuGet.CommandLine\". Specify a version:"})
			assert.Equal(t, heredoc.Doc(`
				PACKAGE            VERSION  STEP NAME/PACKAGE REFERENCE
				pterm              75       Install/pterm
				NuGet.CommandLine  unknown  Install/NuGet.CommandLine
				pterm              0.12     Verify/pterm
			`), stdout.String())
			stdout.Reset()
			_ = q.AnswerWith("1.0.0")

			q = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion})
			assert.Equal(t, heredoc.Doc(`
				PACKAGE            VERSION  STEP NAME/PACKAGE REFERENCE
				pterm              75       Install/pterm
				NuGet.CommandLine  1.0.0    Install/NuGet.CommandLine
				pterm              0.12     Verify/pterm
			`), stdout.String())
			stdout.Reset()
			_ = q.AnswerWith("y")

			versions, overrides, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*create.StepPackageVersion{
				{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: "75"},
				{ActionName: "Install", PackageID: "NuGet.CommandLine", PackageReferenceName: "NuGet.CommandLine", Version: "1.0.0"},
				{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "0.12"},
			}, versions)
			assert.Equal(t, []*create.PackageVersionOverride{
				{PackageReferenceName: "pterm", ActionName: "Install", Version: "75"}, // fully qualify packagereference+actionname to be sure
				{PackageReferenceName: "NuGet.CommandLine", ActionName: "Install", Version: "1.0.0"},
			}, overrides)
		}},

		{"if we enter the loop with any unresolved packages, forced version selection doesn't accept bad input", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			baselineSomeUnresolved := []*create.StepPackageVersion{
				{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: ""}, // unresolved
				{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "0.12"},
			}

			receiver := testutil.GoBegin3(func() ([]*create.StepPackageVersion, []*create.PackageVersionOverride, error) {
				return create.AskPackageOverrideLoop(baselineSomeUnresolved, "", make([]string, 0), qa.AsAsker(), stdout)
			})

			q := qa.ExpectQuestion(t, &survey.Input{Message: "Unable to find a version for \"pterm\". Specify a version:"})
			assert.Equal(t, heredoc.Doc(`
				PACKAGE  VERSION  STEP NAME/PACKAGE REFERENCE
				pterm    unknown  Install/pterm
				pterm    0.12     Verify/pterm
			`), stdout.String())
			stdout.Reset()
			validationErr := q.AnswerWith("z75")
			assert.EqualError(t, validationErr, "\"z75\" is not a valid version")

			validationErr = q.AnswerWith("")
			assert.EqualError(t, validationErr, "\"\" is not a valid version")

			validationErr = q.AnswerWith("dog")
			assert.EqualError(t, validationErr, "\"dog\" is not a valid version")

			validationErr = q.AnswerWith("25.0")
			assert.Nil(t, validationErr)

			_ = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion}).AnswerWith("y")

			versions, overrides, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*create.StepPackageVersion{
				{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: "25.0"},
				{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "0.12"},
			}, versions)
			assert.Equal(t, []*create.PackageVersionOverride{
				{PackageReferenceName: "pterm", ActionName: "Install", Version: "25.0"}, // fully qualify packagereference+actionname to be sure
			}, overrides)
		}},

		{"if we enter the loop with any unresolved packages, pick up --package-version before assuming they're unresolved", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			baselineSomeUnresolved := []*create.StepPackageVersion{
				{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: ""},                         // unresolved
				{ActionName: "Install", PackageID: "NuGet.CommandLine", PackageReferenceName: "NuGet.CommandLine", Version: ""}, // unresolved
				{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "0.12"},
			}

			receiver := testutil.GoBegin3(func() ([]*create.StepPackageVersion, []*create.PackageVersionOverride, error) {
				return create.AskPackageOverrideLoop(baselineSomeUnresolved, "12.7.5", make([]string, 0), qa.AsAsker(), stdout)
			})

			q := qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion})
			assert.Equal(t, heredoc.Doc(`
				PACKAGE            VERSION  STEP NAME/PACKAGE REFERENCE
				pterm              12.7.5   Install/pterm
				pterm              12.7.5   Verify/pterm
				NuGet.CommandLine  12.7.5   Install/NuGet.CommandLine
			`), stdout.String())
			stdout.Reset()
			_ = q.AnswerWith("y")

			versions, overrides, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*create.StepPackageVersion{
				{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: "12.7.5"},
				{ActionName: "Install", PackageID: "NuGet.CommandLine", PackageReferenceName: "NuGet.CommandLine", Version: "12.7.5"},
				{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "12.7.5"},
			}, versions)
			assert.Equal(t, []*create.PackageVersionOverride{
				{Version: "12.7.5"}, // the --package-version input produces this as the first 'override'
				// and that's all we did there
			}, overrides)
		}},

		{"if we enter the loop with any unresolved packages, pick up --package before assuming they're unresolved", func(t *testing.T, qa *testutil.AskMocker, stdout *bytes.Buffer) {
			baselineSomeUnresolved := []*create.StepPackageVersion{
				{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: ""},                         // unresolved
				{ActionName: "Install", PackageID: "NuGet.CommandLine", PackageReferenceName: "NuGet.CommandLine", Version: ""}, // unresolved
				{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "0.12"},
			}

			receiver := testutil.GoBegin3(func() ([]*create.StepPackageVersion, []*create.PackageVersionOverride, error) {
				return create.AskPackageOverrideLoop(baselineSomeUnresolved, "", []string{"Install:pterm:12.9.2"}, qa.AsAsker(), stdout)
			})

			q := qa.ExpectQuestion(t, &survey.Input{Message: "Unable to find a version for \"NuGet.CommandLine\". Specify a version:"})
			assert.Equal(t, heredoc.Doc(`
				PACKAGE            VERSION  STEP NAME/PACKAGE REFERENCE
				pterm              12.9.2   Install/pterm
				NuGet.CommandLine  unknown  Install/NuGet.CommandLine
				pterm              0.12     Verify/pterm
			`), stdout.String())
			stdout.Reset()
			_ = q.AnswerWith("75")

			q = qa.ExpectQuestion(t, &survey.Input{Message: packageOverrideQuestion})
			assert.Equal(t, heredoc.Doc(`
				PACKAGE            VERSION  STEP NAME/PACKAGE REFERENCE
				pterm              12.9.2   Install/pterm
				NuGet.CommandLine  75       Install/NuGet.CommandLine
				pterm              0.12     Verify/pterm
			`), stdout.String())
			stdout.Reset()
			_ = q.AnswerWith("y")

			versions, overrides, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []*create.StepPackageVersion{
				{ActionName: "Install", PackageID: "pterm", PackageReferenceName: "pterm", Version: "12.9.2"},
				{ActionName: "Install", PackageID: "NuGet.CommandLine", PackageReferenceName: "NuGet.CommandLine", Version: "75"},
				{ActionName: "Verify", PackageID: "pterm", PackageReferenceName: "pterm", Version: "0.12"},
			}, versions)
			assert.Equal(t, []*create.PackageVersionOverride{
				{PackageReferenceName: "pterm", ActionName: "Install", Version: "12.9.2"},         // input commandline switch produces this output
				{PackageReferenceName: "NuGet.CommandLine", ActionName: "Install", Version: "75"}, // our first question produces this
			}, overrides)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			qa := testutil.NewAskMocker()
			test.run(t, qa, &bytes.Buffer{})
		})
	}
}

// These tests ensure that given the right input, we call the server's API appropriately
// they all run in automation mode where survey is disabled; they'd error if they tried to ask questions
func TestReleaseCreate_AutomationMode(t *testing.T) {
	fakeRepoUrl, _ := url.Parse("https://gitserver/repo")

	const cacProjectID = "Projects-92"

	space1 := fixtures.NewSpace("Spaces-1", "Default Space")

	depProcess := fixtures.NewDeploymentProcessForProject(space1.ID, cacProjectID)

	protectedBranchNamePatterns := []string{}
	cacProject := fixtures.NewProject(space1.ID, cacProjectID, "CaC Project", "Lifecycles-1", "ProjectGroups-1", depProcess.ID)
	cacProject.PersistenceSettings = projects.NewGitPersistenceSettings(
		".octopus",
		credentials.NewAnonymous(),
		"main",
		false,
		protectedBranchNamePatterns,
		fakeRepoUrl,
	)

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"release creation requires a project name", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "create"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "project must be specified")

			assert.Equal(t, "", stdOut.String())
			// At first glance it may appear a bit weird that stdErr doesn't contain the error message here.
			// This is fine though, the main program entrypoint prints any errors that bubble up to it.
			assert.Equal(t, "", stdErr.String())
		}},

		{"release creation specifying project only (bare minimum)", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "create", "--project", cacProject.Name})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/create/v1")

			// check that it sent the server the right request body
			requestBody, err := testutil.ReadJson[releases.CreateReleaseCommandV1](req.Request.Body)
			assert.Nil(t, err)

			assert.Equal(t, releases.CreateReleaseCommandV1{
				SpaceID:         "Spaces-1",
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

			assert.Equal(t, heredoc.Doc(`
				Successfully created release version 1.2.3 using channel Alpha channel
				
				View this release on Octopus Deploy: http://server/app#/Spaces-1/releases/Releases-999
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"release creation specifying project only (bare minimum)", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "create", "--project", cacProject.Name})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/create/v1")

			// check that it sent the server the right request body
			requestBody, err := testutil.ReadJson[releases.CreateReleaseCommandV1](req.Request.Body)
			assert.Nil(t, err)

			assert.Equal(t, releases.CreateReleaseCommandV1{
				SpaceID:         "Spaces-1",
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

			assert.Equal(t, heredoc.Doc(`
				Successfully created release version 1.2.3 using channel Alpha channel
				
				View this release on Octopus Deploy: http://server/app#/Spaces-1/releases/Releases-999
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"release creation outputformat basic", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "create", "--project", cacProject.Name, "--output-format", "basic"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			// don't need to validate the json received by the server, we've done that already
			api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/create/v1").RespondWith(&releases.CreateReleaseResponseV1{
				ReleaseID:      "Releases-999",
				ReleaseVersion: "1.2.3",
			})

			// after it creates the release it's going to go back to the server and lookup the release by its ID
			// so it can tell the user what channel got selected
			api.ExpectRequest(t, "GET", "/api/Spaces-1/releases/Releases-999").RespondWith(releases.NewRelease("Channels-32", cacProject.ID, "1.2.3"))

			// and now it wants to lookup the channel name too
			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Channels-32").
				RespondWith(fixtures.NewChannel(space1.ID, "Channels-32", "Alpha channel", cacProject.ID))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, "1.2.3\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"release creation outputformat json", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "create", "--project", cacProject.Name, "--output-format", "json"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			// don't need to validate the json received by the server, we've done that already
			api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/create/v1").RespondWith(&releases.CreateReleaseResponseV1{
				ReleaseID:      "Releases-999",
				ReleaseVersion: "1.2.3",
			})

			// after it creates the release it's going to go back to the server and lookup the release by its ID
			// so it can tell the user what channel got selected
			api.ExpectRequest(t, "GET", "/api/Spaces-1/releases/Releases-999").RespondWith(releases.NewRelease("Channels-32", cacProject.ID, "1.2.3"))

			// and now it wants to lookup the channel name too
			api.ExpectRequest(t, "GET", "/api/Spaces-1/channels/Channels-32").
				RespondWith(fixtures.NewChannel(space1.ID, "Channels-32", "Alpha channel", cacProject.ID))

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, "{\"Channel\":\"Alpha channel\",\"Version\":\"1.2.3\"}\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"release creation specifying gitcommit and gitref", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "create", "--project", cacProject.Name, "--git-ref", "refs/heads/main", "--git-commit", "6ef5e8c83cdcd4933bbeaeb458dc99902ad831ca"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/create/v1")

			// check that it sent the server the right request body
			requestBody, err := testutil.ReadJson[releases.CreateReleaseCommandV1](req.Request.Body)
			assert.Nil(t, err)

			assert.Equal(t, releases.CreateReleaseCommandV1{
				SpaceID:         "Spaces-1",
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

			assert.Equal(t, heredoc.Doc(`
				Successfully created release version 1.2.3 using channel Alpha channel
				
				View this release on Octopus Deploy: http://server/app#/Spaces-1/releases/Releases-999
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"release creation specifying package default version + overrides", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "create", "--project", cacProject.Name, "--package-version", "1.2", "--package", "NuGet.CommandLine:6.12", "--package", "pterm:0.12.5", "--package", "pterm-on-deploy:pterm:0.12.7"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/create/v1")

			// check that it sent the server the right request body
			requestBody, err := testutil.ReadJson[releases.CreateReleaseCommandV1](req.Request.Body)
			assert.Nil(t, err)

			assert.Equal(t, releases.CreateReleaseCommandV1{
				SpaceID:         "Spaces-1",
				ProjectIDOrName: cacProject.Name,
				PackageVersion:  "1.2",
				Packages: []string{
					"NuGet.CommandLine:6.12",
					"pterm:0.12.5",
					"pterm-on-deploy:pterm:0.12.7",
				},
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

			assert.Equal(t, heredoc.Doc(`
				Successfully created release version 1.2.3 using channel Alpha channel
				
				View this release on Octopus Deploy: http://server/app#/Spaces-1/releases/Releases-999
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"can't specify release-notes and release-notes-file at the same time", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			// doesn't even do any API stuff, we get kicked out immediately
			rootCmd.SetArgs([]string{"release", "create",
				"--project", cacProject.Name,
				"--release-notes", "Here are some **release notes**.",
				"--release-notes-file", "foo.md",
			})
			_, err := rootCmd.ExecuteC()
			assert.EqualError(t, err, "cannot specify both --release-notes and --release-notes-file at the same time")
		}},

		{"release creation with all the flags", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "create",
					"--project", cacProject.Name,
					"--package-version", "5.6.7-beta",
					"--package", "pterm:2.5",
					"--package", "NuGet.CommandLine:5.4.1",
					"--git-ref", "refs/heads/main",
					"--git-commit", "6ef5e8c83cdcd4933bbeaeb458dc99902ad831ca",
					"--version", "1.0.2",
					"--channel", "BetaChannel",
					"--release-notes", "Here are some **release notes**.",
					"--ignore-channel-rules",
					"--ignore-existing",
				})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/create/v1")

			// check that it sent the server the right request body
			requestBody, err := testutil.ReadJson[releases.CreateReleaseCommandV1](req.Request.Body)
			assert.Nil(t, err)

			assert.Equal(t, releases.CreateReleaseCommandV1{
				SpaceID:               "Spaces-1",
				ProjectIDOrName:       cacProject.Name,
				PackageVersion:        "5.6.7-beta",
				GitCommit:             "6ef5e8c83cdcd4933bbeaeb458dc99902ad831ca",
				GitRef:                "refs/heads/main",
				ReleaseVersion:        "1.0.2",
				ChannelIDOrName:       "BetaChannel",
				ReleaseNotes:          "Here are some **release notes**.",
				IgnoreIfAlreadyExists: true,
				IgnoreChannelRules:    true,
				PackagePrerelease:     "", // not supported in the new CLI
				Packages:              []string{"pterm:2.5", "NuGet.CommandLine:5.4.1"},
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

			assert.Equal(t, heredoc.Doc(`
				Successfully created release version 1.0.5 using channel Alpha channel
				
				View this release on Octopus Deploy: http://server/app#/Spaces-1/releases/Releases-999
				`), stdOut.String())

			assert.Equal(t, "", stdErr.String())
		}},

		{"release creation with all the flags (legacy aliases)", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "create",
					"--project=" + cacProject.Name,
					"--packageVersion=5.6.7-beta",
					"--package=pterm:2.5",
					"--package=NuGet.CommandLine:5.4.1",
					"--gitRef=refs/heads/main",
					"--gitCommit=6ef5e8c83cdcd4933bbeaeb458dc99902ad831ca",
					"--releaseNumber=1.0.2",
					"--channel=BetaChannel",
					"--releaseNotes=Here are some **release notes**.",
					"--ignoreChannelRules",
					"--ignoreExisting",
				})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/create/v1")

			// check that it sent the server the right request body
			requestBody, err := testutil.ReadJson[releases.CreateReleaseCommandV1](req.Request.Body)
			assert.Nil(t, err)

			assert.Equal(t, releases.CreateReleaseCommandV1{
				SpaceID:               "Spaces-1",
				ProjectIDOrName:       cacProject.Name,
				PackageVersion:        "5.6.7-beta",
				GitCommit:             "6ef5e8c83cdcd4933bbeaeb458dc99902ad831ca",
				GitRef:                "refs/heads/main",
				ReleaseVersion:        "1.0.2",
				ChannelIDOrName:       "BetaChannel",
				ReleaseNotes:          "Here are some **release notes**.",
				IgnoreIfAlreadyExists: true,
				IgnoreChannelRules:    true,
				PackagePrerelease:     "", // not supported in the new CLI
				Packages:              []string{"pterm:2.5", "NuGet.CommandLine:5.4.1"},
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

			assert.Equal(t, heredoc.Doc(`
				Successfully created release version 1.0.5 using channel Alpha channel
				
				View this release on Octopus Deploy: http://server/app#/Spaces-1/releases/Releases-999
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"release creation with all the flags (short flags where available)", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "create",
					"-p", cacProject.Name,
					"--package-version", "5.6.7-beta",
					"-r", "refs/heads/main",
					"--git-commit", "6ef5e8c83cdcd4933bbeaeb458dc99902ad831ca",
					"--version", "1.0.2", // no short form for version; or it wouldn't align with release deploy where -v is variable
					"-c", "BetaChannel",
					"-n", "Here are some **release notes**.",
				})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/create/v1")

			// check that it sent the server the right request body
			requestBody, err := testutil.ReadJson[releases.CreateReleaseCommandV1](req.Request.Body)
			assert.Nil(t, err)

			assert.Equal(t, releases.CreateReleaseCommandV1{
				SpaceID:               "Spaces-1",
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

			assert.Equal(t, heredoc.Doc(`
				Successfully created release version 1.0.5 using channel Alpha channel
				
				View this release on Octopus Deploy: http://server/app#/Spaces-1/releases/Releases-999
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"release-notes-file pickup", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			file, err := os.CreateTemp("", "*.md")
			require.Nil(t, err)
			require.NotNil(t, file)
			_, err = file.WriteString("release notes **in a file**")
			require.Nil(t, err)
			err = file.Close()
			require.Nil(t, err)

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "create",
					"--project", cacProject.Name,
					"--channel", "BetaChannel",
					"--release-notes-file", file.Name(),
				})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/releases/create/v1")

			// check that it sent the server the right request body
			requestBody, err := testutil.ReadJson[releases.CreateReleaseCommandV1](req.Request.Body)
			assert.Nil(t, err)

			assert.Equal(t, releases.CreateReleaseCommandV1{
				SpaceID:         "Spaces-1",
				ProjectIDOrName: cacProject.Name,
				ChannelIDOrName: "BetaChannel",
				ReleaseNotes:    "release notes **in a file**",
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

			assert.Equal(t, heredoc.Doc(`
				Successfully created release version 1.0.5 using channel Alpha channel
				
				View this release on Octopus Deploy: http://server/app#/Spaces-1/releases/Releases-999
				`), stdOut.String())

			assert.Equal(t, "", stdErr.String())
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			api := testutil.NewMockHttpServer()

			rootCmd := cmdRoot.NewCmdRoot(testutil.NewMockFactoryWithSpace(api, space1), nil, nil)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)

			test.run(t, api, rootCmd, stdout, stderr)
		})
	}
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
					IsResolvable:         true,
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
				PackageID:            "pterm",
				ActionName:           "Install",
				Version:              "0.12.51",
				PackageReferenceName: "pterm-on-install",
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
					IsResolvable:         true,
				},
				{
					ActionName:           "Install",
					FeedID:               externalFeedID,
					PackageID:            "NuGet.CommandLine",
					PackageReferenceName: "nuget-on-install",
					IsResolvable:         true,
				},
				{
					ActionName:           "Verify",
					FeedID:               builtinFeedID,
					PackageID:            "pterm",
					PackageReferenceName: "pterm-on-verify",
					IsResolvable:         true,
				},
				{
					ActionName:           "Cleanup",
					FeedID:               externalFeedID,
					PackageID:            "NuGet.CommandLine",
					PackageReferenceName: "nuget-on-cleanup",
					IsResolvable:         true,
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
				PackageID:            "pterm",
				ActionName:           "Install",
				Version:              "0.12.51",
				PackageReferenceName: "pterm-on-install",
			},
			{
				PackageID:            "pterm",
				ActionName:           "Verify",
				Version:              "0.12.51",
				PackageReferenceName: "pterm-on-verify",
			},
			{
				PackageID:            "NuGet.CommandLine",
				ActionName:           "Install",
				Version:              "6.1.2",
				PackageReferenceName: "nuget-on-install",
			},
			{
				PackageID:            "NuGet.CommandLine",
				ActionName:           "Cleanup",
				Version:              "6.1.2",
				PackageReferenceName: "nuget-on-cleanup",
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
					IsResolvable:         true,
				},
				{
					ActionName:           "Install",
					FeedID:               externalFeedID,
					PackageID:            "NuGet.CommandLine",
					PackageReferenceName: "nuget-on-install",
					IsResolvable:         true,
				},
				{
					ActionName:           "Verify",
					FeedID:               builtinFeedID,
					PackageID:            "pterm",
					PackageReferenceName: "pterm-on-verify",
					IsResolvable:         true,
				},
				{
					ActionName:           "Cleanup",
					FeedID:               externalFeedID,
					PackageID:            "NuGet.CommandLine", // channel rule is going to say that this one should have a different version
					PackageReferenceName: "nuget-on-cleanup",
					IsResolvable:         true,
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
				PackageID:            "pterm",
				ActionName:           "Install",
				Version:              "0.12.51",
				PackageReferenceName: "pterm-on-install",
			},
			{
				PackageID:            "pterm",
				ActionName:           "Verify",
				Version:              "0.12.51",
				PackageReferenceName: "pterm-on-verify",
			},
			{
				PackageID:            "NuGet.CommandLine",
				ActionName:           "Install",
				Version:              "6.1.2",
				PackageReferenceName: "nuget-on-install",
			},
			{
				PackageID:            "NuGet.CommandLine",
				ActionName:           "Cleanup",
				Version:              "5.4.1-prerelease",
				PackageReferenceName: "nuget-on-cleanup",
			},
		}, packageVersions)
	})

	t.Run("still returns a value if the server returns zero available packages", func(t *testing.T) {
		// note: channel rules can affect which packages the server returns so there might be zero,
		// but either way the server returns zero items, which is the code path we are testing, so channel rules aren't relevant here
		api := testutil.NewMockHttpServer()
		processTemplate := &deployments.DeploymentProcessTemplate{
			Packages: []releases.ReleaseTemplatePackage{
				{ActionName: "Install", FeedID: builtinFeedID, PackageID: "pterm", PackageReferenceName: "pterm-on-install", IsResolvable: true},
				{ActionName: "Install", FeedID: builtinFeedID, PackageID: "NuGet.CommandLine", PackageReferenceName: "nuget-on-install", IsResolvable: true},
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
			Items: []*packages.PackageVersion{}, // empty!
		})
		api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds/feeds-builtin/packages/versions?packageId=NuGet.CommandLine&take=1").RespondWith(&resources.Resources[*packages.PackageVersion]{
			Items: []*packages.PackageVersion{}, // empty!
		})

		packageVersions, err := testutil.ReceivePair(receiver)
		assert.Nil(t, err)
		assert.Equal(t, []*create.StepPackageVersion{
			{
				PackageID:            "pterm",
				ActionName:           "Install",
				Version:              "", // no version found
				PackageReferenceName: "pterm-on-install",
			},
			{
				PackageID:            "NuGet.CommandLine",
				ActionName:           "Install",
				Version:              "", // no version found
				PackageReferenceName: "nuget-on-install",
			},
		}, packageVersions)
	})

	t.Run("fails if the server returns zero available packages; dynamic packages, including where the Feed ID is templated", func(t *testing.T) {
		api := testutil.NewMockHttpServer()
		processTemplate := &deployments.DeploymentProcessTemplate{
			Packages: []releases.ReleaseTemplatePackage{
				// IsResolvable controls whether the CLI will attempt to lookup versions for it. Anything with IsResolvable=false just gets dumped straight into the output table
				{ActionName: "Install", FeedID: builtinFeedID, PackageID: "pterm-#{Octopus.Environment.Id}", PackageReferenceName: "pterm-on-install-with-id", StepName: "Install", IsResolvable: true},
				{ActionName: "Install", FeedID: builtinFeedID, PackageID: "pterm-#{Octopus.Environment.Name}", PackageReferenceName: "pterm-on-install-with-name", StepName: "Install", IsResolvable: false},
				{ActionName: "Install", FeedID: "#{FeedID}", PackageID: "NuGet.CommandLine-#{Octopus.Project.Name}", PackageReferenceName: "nuget-on-install", StepName: "Install", IsResolvable: false},
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

		// now it will search for the package versions (the first one said resolvable:true so we ask the server, even though there's a variable template)
		api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds/feeds-builtin/packages/versions?packageId=pterm-%23%7BOctopus.Environment.Id%7D&take=1").RespondWith(&resources.Resources[*packages.PackageVersion]{
			Items: []*packages.PackageVersion{}, // empty!
		})
		// the other two packages have IsResolvable:false so we don't even try to look for them.

		packageVersions, err := testutil.ReceivePair(receiver)
		assert.Nil(t, err)
		assert.Equal(t, []*create.StepPackageVersion{
			{
				PackageID:            "pterm-#{Octopus.Environment.Name}",
				ActionName:           "Install",
				Version:              "", // no version found
				PackageReferenceName: "pterm-on-install-with-name",
			},
			{
				PackageID:            "NuGet.CommandLine-#{Octopus.Project.Name}",
				ActionName:           "Install",
				Version:              "", // no version found
				PackageReferenceName: "nuget-on-install",
			},
			{
				PackageID:            "pterm-#{Octopus.Environment.Id}",
				ActionName:           "Install",
				Version:              "", // no version found
				PackageReferenceName: "pterm-on-install-with-id",
			},
		}, packageVersions)
	})

}

func TestReleaseCreate_ToPackageOverrideString(t *testing.T) {
	tests := []struct {
		name   string
		input  *create.PackageVersionOverride
		expect string
	}{
		{name: "ver-only", input: &create.PackageVersionOverride{Version: "0.12"}, expect: "*:0.12"},
		{name: "action-ver", input: &create.PackageVersionOverride{ActionName: "Install", Version: "0.12"}, expect: "Install:0.12"},
		{name: "action-ver-2", input: &create.PackageVersionOverride{ActionName: "Verify", Version: "6.1.2-beta"}, expect: "Verify:6.1.2-beta"},
		{name: "pkg-ver", input: &create.PackageVersionOverride{PackageID: "pterm", Version: "0.12"}, expect: "pterm:0.12"},
		{name: "pkg-ver-2", input: &create.PackageVersionOverride{PackageID: "NuGet.CommandLine", Version: "6.1.2-beta"}, expect: "NuGet.CommandLine:6.1.2-beta"},
		{name: "pkg-action-ver", input: &create.PackageVersionOverride{PackageID: "pterm", ActionName: "Install", Version: "0.12"}, expect: "pterm:0.12"}, // this isn't valid, but if it did happen it should pick packageID
		{name: "pkg-ref-ver", input: &create.PackageVersionOverride{PackageReferenceName: "pterm-on-install", PackageID: "pterm", Version: "6.1.2"}, expect: "pterm:pterm-on-install:6.1.2"},
		{name: "action-ref-ver", input: &create.PackageVersionOverride{PackageReferenceName: "pterm-on-install", ActionName: "Install", Version: "6.1.2"}, expect: "Install:pterm-on-install:6.1.2"},
		{name: "star-ref-ver", input: &create.PackageVersionOverride{PackageReferenceName: "pterm-on-install", Version: "6.1.2"}, expect: "*:pterm-on-install:6.1.2"},
		{name: "pkg-action-ref-ver", input: &create.PackageVersionOverride{PackageReferenceName: "pterm", PackageID: "pterm", ActionName: "Install", Version: "1.2.3"}, expect: "pterm:pterm:1.2.3"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.input.ToPackageOverrideString()
			assert.Equal(t, test.expect, result)
		})
	}
}

func TestReleaseCreate_ParsePackageOverrideString(t *testing.T) {
	tests := []struct {
		input     string
		expect    *create.AmbiguousPackageVersionOverride
		expectErr error
	}{
		{input: ":5", expect: &create.AmbiguousPackageVersionOverride{ActionNameOrPackageID: "", Version: "5"}},
		{input: "::5", expect: &create.AmbiguousPackageVersionOverride{ActionNameOrPackageID: "", Version: "5"}},
		{input: "*:5", expect: &create.AmbiguousPackageVersionOverride{ActionNameOrPackageID: "", Version: "5"}},
		{input: "*:*:5", expect: &create.AmbiguousPackageVersionOverride{ActionNameOrPackageID: "", Version: "5"}},
		{input: ":*:5", expect: &create.AmbiguousPackageVersionOverride{ActionNameOrPackageID: "", Version: "5"}},
		{input: "NuGet:NuGet:0.1", expect: &create.AmbiguousPackageVersionOverride{PackageReferenceName: "NuGet", ActionNameOrPackageID: "NuGet", Version: "0.1"}},
		{input: "NuGet:nuget-on-install:0.1", expect: &create.AmbiguousPackageVersionOverride{PackageReferenceName: "nuget-on-install", ActionNameOrPackageID: "NuGet", Version: "0.1"}},
		{input: "Install:nuget-on-install:0.1", expect: &create.AmbiguousPackageVersionOverride{PackageReferenceName: "nuget-on-install", ActionNameOrPackageID: "Install", Version: "0.1"}},
		{input: "pterm:9.7-pre-xyz", expect: &create.AmbiguousPackageVersionOverride{ActionNameOrPackageID: "pterm", Version: "9.7-pre-xyz"}},
		{input: "pterm:55", expect: &create.AmbiguousPackageVersionOverride{ActionNameOrPackageID: "pterm", Version: "55"}},
		{input: "pterm::55", expect: &create.AmbiguousPackageVersionOverride{ActionNameOrPackageID: "pterm", Version: "55"}},
		{input: ":Push Package:55", expect: &create.AmbiguousPackageVersionOverride{PackageReferenceName: "Push Package", ActionNameOrPackageID: "", Version: "55"}},
		{input: "*:Push Package:55", expect: &create.AmbiguousPackageVersionOverride{PackageReferenceName: "Push Package", ActionNameOrPackageID: "", Version: "55"}},

		{input: "pterm/Push Package=9.7-pre-xyz", expect: &create.AmbiguousPackageVersionOverride{PackageReferenceName: "Push Package", ActionNameOrPackageID: "pterm", Version: "9.7-pre-xyz"}},
		{input: "pterm=Push Package/9.7-pre-xyz", expect: &create.AmbiguousPackageVersionOverride{PackageReferenceName: "Push Package", ActionNameOrPackageID: "pterm", Version: "9.7-pre-xyz"}},

		{input: "", expectErr: errors.New("empty package version specification")},

		// bare identifiers aren't valid
		{input: "5", expectErr: errors.New("package version specification \"5\" does not use expected format")},
		{input: "fish", expectErr: errors.New("package version specification \"fish\" does not use expected format")},
		{input: "Install:pterm:nuget:5", expectErr: errors.New("package version specification \"Install:pterm:nuget:5\" does not use expected format")},

		// versions must be version-ish
		{input: ":x5", expectErr: errors.New("version component \"x5\" is not a valid version")},
		{input: "NuGet:NuGet:dog", expectErr: errors.New("version component \"dog\" is not a valid version")},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result, err := create.ParsePackageOverrideString(test.input)
			assert.Equal(t, test.expectErr, err)
			assert.Equal(t, test.expect, result)
		})
	}
}

func TestReleaseCreate_ResolvePackageOverride(t *testing.T) {
	// this is the packageId:5.0 syntax
	t.Run("match on package ID", func(t *testing.T) { // this is probably the most common thing people will do
		nugetPackage := &create.AmbiguousPackageVersionOverride{ActionNameOrPackageID: "NuGet", Version: "5.0"}

		steps := []*create.StepPackageVersion{ // baseline
			{PackageID: "NuGet", ActionName: "Install", Version: "0.1", PackageReferenceName: "NuGet"},
		}

		r, err := create.ResolvePackageOverride(nugetPackage, steps)
		assert.Nil(t, err)
		assert.Equal(t, &create.PackageVersionOverride{PackageID: "NuGet", ActionName: "", Version: "5.0", PackageReferenceName: ""}, r)
	})

	// this is the stepName:5.0 syntax
	t.Run("match on step name", func(t *testing.T) { // this is probably the most common thing people will do
		nugetPackage := &create.AmbiguousPackageVersionOverride{ActionNameOrPackageID: "Install", Version: "5.0"}

		steps := []*create.StepPackageVersion{ // baseline
			{PackageID: "NuGet", ActionName: "Install", Version: "0.1", PackageReferenceName: "NuGet"},
		}

		r, err := create.ResolvePackageOverride(nugetPackage, steps)
		assert.Nil(t, err)
		assert.Equal(t, &create.PackageVersionOverride{PackageID: "", ActionName: "Install", Version: "5.0", PackageReferenceName: ""}, r)
	})

	// this is the packageRef:*:5.0 syntax
	t.Run("match on packageRef", func(t *testing.T) {
		nugetPackage := &create.AmbiguousPackageVersionOverride{PackageReferenceName: "NuGet-B", Version: "5.0"}

		steps := []*create.StepPackageVersion{ // baseline
			{PackageID: "NuGet", ActionName: "Install", Version: "0.1", PackageReferenceName: "NuGet-A"},
			{PackageID: "NuGet", ActionName: "Verify", Version: "0.1", PackageReferenceName: "NuGet-B"},
		}

		r, err := create.ResolvePackageOverride(nugetPackage, steps)
		assert.Nil(t, err)
		assert.Equal(t, &create.PackageVersionOverride{PackageID: "", ActionName: "", Version: "5.0", PackageReferenceName: "NuGet-B"}, r)
	})

	// this is the *:5.0 or :5.0 or just 5.0 syntax
	t.Run("match on just version", func(t *testing.T) { // this is probably the most common thing people will do
		nugetPackage := &create.AmbiguousPackageVersionOverride{Version: "5.0"}

		steps := []*create.StepPackageVersion{ // baseline
			{PackageID: "NuGet", ActionName: "Install", Version: "0.1", PackageReferenceName: "NuGet"},
		}

		r, err := create.ResolvePackageOverride(nugetPackage, steps)
		assert.Nil(t, err)
		assert.Equal(t, &create.PackageVersionOverride{PackageID: "", ActionName: "", Version: "5.0", PackageReferenceName: ""}, r)
	})

	t.Run("match on just version doesn't even need any packages to look at", func(t *testing.T) { // this is probably the most common thing people will do
		nugetPackage := &create.AmbiguousPackageVersionOverride{Version: "5.0"}

		steps := make([]*create.StepPackageVersion, 0) // baseline

		r, err := create.ResolvePackageOverride(nugetPackage, steps)
		assert.Nil(t, err)
		assert.Equal(t, &create.PackageVersionOverride{PackageID: "", ActionName: "", Version: "5.0", PackageReferenceName: ""}, r)
	})

	t.Run("match on action+packageRef before packageID", func(t *testing.T) { // this is probably the most common thing people will do
		nugetPackage := &create.AmbiguousPackageVersionOverride{ActionNameOrPackageID: "Verify", PackageReferenceName: "NuGet", Version: "5.0"}

		steps := []*create.StepPackageVersion{ // baseline
			{PackageID: "NuGet", ActionName: "Install", Version: "0.1", PackageReferenceName: "NuGet"},
			{PackageID: "NuGet", ActionName: "Verify", Version: "0.1", PackageReferenceName: "NuGet"},
		}

		r, err := create.ResolvePackageOverride(nugetPackage, steps)
		assert.Nil(t, err)
		assert.Equal(t, &create.PackageVersionOverride{PackageID: "", ActionName: "Verify", Version: "5.0", PackageReferenceName: "NuGet"}, r)
	})

	t.Run("match on packageID+packageRef picks the first one where they are the same", func(t *testing.T) {
		nugetPackage := &create.AmbiguousPackageVersionOverride{ActionNameOrPackageID: "NuGet", PackageReferenceName: "NuGet", Version: "5.0"}

		steps := []*create.StepPackageVersion{ // baseline
			{PackageID: "NuGet", ActionName: "Install", Version: "0.1", PackageReferenceName: "NuGet"},
			{PackageID: "NuGet", ActionName: "Verify", Version: "0.1", PackageReferenceName: "NuGet"},
		}

		r, err := create.ResolvePackageOverride(nugetPackage, steps)
		assert.Nil(t, err)
		assert.Equal(t, &create.PackageVersionOverride{PackageID: "NuGet", ActionName: "", Version: "5.0", PackageReferenceName: "NuGet"}, r)
	})

	t.Run("match on packageID+packageRef picks the correct one where they are different", func(t *testing.T) {
		nugetPackage := &create.AmbiguousPackageVersionOverride{ActionNameOrPackageID: "NuGet", PackageReferenceName: "NuGet-B", Version: "5.0"}

		steps := []*create.StepPackageVersion{ // baseline
			{PackageID: "NuGet", ActionName: "Install", Version: "0.1", PackageReferenceName: "NuGet-A"},
			{PackageID: "NuGet", ActionName: "Verify", Version: "0.1", PackageReferenceName: "NuGet-B"},
		}

		r, err := create.ResolvePackageOverride(nugetPackage, steps)
		assert.Nil(t, err)
		assert.Equal(t, &create.PackageVersionOverride{PackageID: "NuGet", ActionName: "", Version: "5.0", PackageReferenceName: "NuGet-B"}, r)
	})

	t.Run("match on packageRef wins over match on ActionName", func(t *testing.T) {
		// we shouldn't get in this situation, but just in case we do :shrug:
		nugetPackage := &create.AmbiguousPackageVersionOverride{PackageReferenceName: "NuGet-B", ActionNameOrPackageID: "Cheese", Version: "5.0"}

		steps := []*create.StepPackageVersion{ // baseline
			{PackageID: "NuGet", ActionName: "Install", Version: "0.1", PackageReferenceName: "NuGet-A"},
			{PackageID: "NuGet", ActionName: "Verify", Version: "0.1", PackageReferenceName: "NuGet-B"},
			{PackageID: "OtherPackage", ActionName: "Cheese", Version: "0.1", PackageReferenceName: "Cheese"},
		}

		r, err := create.ResolvePackageOverride(nugetPackage, steps)
		assert.Nil(t, err)
		assert.Equal(t, &create.PackageVersionOverride{PackageID: "", ActionName: "", Version: "5.0", PackageReferenceName: "NuGet-B"}, r)
	})

	t.Run("match on packageRef wins over match on PackageID", func(t *testing.T) {
		// we shouldn't get in this situation, but just in case we do :shrug:
		nugetPackage := &create.AmbiguousPackageVersionOverride{PackageReferenceName: "NuGet-B", ActionNameOrPackageID: "Cheese", Version: "5.0"}

		steps := []*create.StepPackageVersion{ // baseline
			{PackageID: "NuGet", ActionName: "Install", Version: "0.1", PackageReferenceName: "NuGet-A"},
			{PackageID: "NuGet", ActionName: "Verify", Version: "0.1", PackageReferenceName: "NuGet-B"},
			{PackageID: "Cheese", ActionName: "OtherAction", Version: "0.1", PackageReferenceName: "OtherAction"},
		}

		r, err := create.ResolvePackageOverride(nugetPackage, steps)
		assert.Nil(t, err)
		assert.Equal(t, &create.PackageVersionOverride{PackageID: "", ActionName: "", Version: "5.0", PackageReferenceName: "NuGet-B"}, r)
	})
}

func TestReleaseCreate_ApplyPackageOverride(t *testing.T) {
	standardPackageSpec := []*create.StepPackageVersion{
		{PackageID: "pterm", ActionName: "Install", Version: "0.12", PackageReferenceName: "pterm-on-install"},
		{PackageID: "pterm", ActionName: "Push", Version: "0.12", PackageReferenceName: "pterm-on-push"},
		{PackageID: "NuGet.CommandLine", ActionName: "Install", Version: "5.4", PackageReferenceName: "nuget-on-install"},
		{PackageID: "NuGet.CommandLine", ActionName: "Push", Version: "5.4", PackageReferenceName: "nuget-on-push"},
	}

	t.Run("apply wildcard override", func(t *testing.T) {
		result := create.ApplyPackageOverrides(standardPackageSpec, []*create.PackageVersionOverride{
			{Version: "99"},
		})

		assert.Equal(t, []*create.StepPackageVersion{
			{PackageID: "pterm", ActionName: "Install", Version: "99", PackageReferenceName: "pterm-on-install"},
			{PackageID: "pterm", ActionName: "Push", Version: "99", PackageReferenceName: "pterm-on-push"},
			{PackageID: "NuGet.CommandLine", ActionName: "Install", Version: "99", PackageReferenceName: "nuget-on-install"},
			{PackageID: "NuGet.CommandLine", ActionName: "Push", Version: "99", PackageReferenceName: "nuget-on-push"},
		}, result)
	})

	t.Run("apply one override based on package ID", func(t *testing.T) {
		result := create.ApplyPackageOverrides(standardPackageSpec, []*create.PackageVersionOverride{
			{PackageID: "pterm", Version: "99"},
		})

		assert.Equal(t, []*create.StepPackageVersion{
			{PackageID: "pterm", ActionName: "Install", Version: "99", PackageReferenceName: "pterm-on-install"},
			{PackageID: "pterm", ActionName: "Push", Version: "99", PackageReferenceName: "pterm-on-push"},
			{PackageID: "NuGet.CommandLine", ActionName: "Install", Version: "5.4", PackageReferenceName: "nuget-on-install"},
			{PackageID: "NuGet.CommandLine", ActionName: "Push", Version: "5.4", PackageReferenceName: "nuget-on-push"},
		}, result)
	})

	t.Run("apply one override based on step name", func(t *testing.T) {
		result := create.ApplyPackageOverrides(standardPackageSpec, []*create.PackageVersionOverride{
			{ActionName: "Install", Version: "99"},
		})

		assert.Equal(t, []*create.StepPackageVersion{
			{PackageID: "pterm", ActionName: "Install", Version: "99", PackageReferenceName: "pterm-on-install"},
			{PackageID: "pterm", ActionName: "Push", Version: "0.12", PackageReferenceName: "pterm-on-push"},
			{PackageID: "NuGet.CommandLine", ActionName: "Install", Version: "99", PackageReferenceName: "nuget-on-install"},
			{PackageID: "NuGet.CommandLine", ActionName: "Push", Version: "5.4", PackageReferenceName: "nuget-on-push"},
		}, result)
	})

	t.Run("apply one override based on both package and step name", func(t *testing.T) {
		result := create.ApplyPackageOverrides(standardPackageSpec, []*create.PackageVersionOverride{
			{PackageID: "pterm", ActionName: "Install", Version: "99"},
		})

		assert.Equal(t, []*create.StepPackageVersion{
			{PackageID: "pterm", ActionName: "Install", Version: "99", PackageReferenceName: "pterm-on-install"},
			{PackageID: "pterm", ActionName: "Push", Version: "0.12", PackageReferenceName: "pterm-on-push"},
			{PackageID: "NuGet.CommandLine", ActionName: "Install", Version: "5.4", PackageReferenceName: "nuget-on-install"},
			{PackageID: "NuGet.CommandLine", ActionName: "Push", Version: "5.4", PackageReferenceName: "nuget-on-push"},
		}, result)
	})

	t.Run("apply multiple overrides", func(t *testing.T) {
		result := create.ApplyPackageOverrides(standardPackageSpec, []*create.PackageVersionOverride{
			{Version: "0.1"},
			{PackageID: "pterm", Version: "2"},
			{PackageID: "NuGet.CommandLine", ActionName: "Push", Version: "6"},
		})

		assert.Equal(t, []*create.StepPackageVersion{
			{PackageID: "pterm", ActionName: "Install", Version: "2", PackageReferenceName: "pterm-on-install"},
			{PackageID: "pterm", ActionName: "Push", Version: "2", PackageReferenceName: "pterm-on-push"},
			{PackageID: "NuGet.CommandLine", ActionName: "Install", Version: "0.1", PackageReferenceName: "nuget-on-install"},
			{PackageID: "NuGet.CommandLine", ActionName: "Push", Version: "6", PackageReferenceName: "nuget-on-push"},
		}, result)
	})

	t.Run("apply multiple overrides; order matters", func(t *testing.T) {
		result := create.ApplyPackageOverrides(standardPackageSpec, []*create.PackageVersionOverride{
			{PackageID: "pterm", Version: "2"},
			{PackageID: "NuGet.CommandLine", ActionName: "Push", Version: "6"},
			{Version: "0.1"}, // overwrites everything
		})

		assert.Equal(t, []*create.StepPackageVersion{
			{PackageID: "pterm", ActionName: "Install", Version: "0.1", PackageReferenceName: "pterm-on-install"},
			{PackageID: "pterm", ActionName: "Push", Version: "0.1", PackageReferenceName: "pterm-on-push"},
			{PackageID: "NuGet.CommandLine", ActionName: "Install", Version: "0.1", PackageReferenceName: "nuget-on-install"},
			{PackageID: "NuGet.CommandLine", ActionName: "Push", Version: "0.1", PackageReferenceName: "nuget-on-push"},
		}, result)
	})

	t.Run("apply single override targeting only package-ref", func(t *testing.T) {
		packageSpec := []*create.StepPackageVersion{
			{PackageID: "pterm", ActionName: "Install", PackageReferenceName: "pterm-on-install", Version: "0.12"},
			{PackageID: "pterm", ActionName: "Push", PackageReferenceName: "pterm", Version: "0.12"},
			{PackageID: "pterm", ActionName: "Verify", PackageReferenceName: "pterm", Version: "0.12"},
			{PackageID: "NuGet.CommandLine", ActionName: "Install", PackageReferenceName: "NuGet.CommandLine", Version: "5.4"},
		}

		result := create.ApplyPackageOverrides(packageSpec, []*create.PackageVersionOverride{
			{PackageReferenceName: "pterm", Version: "2000"},
		})

		assert.Equal(t, []*create.StepPackageVersion{
			{PackageID: "pterm", ActionName: "Install", PackageReferenceName: "pterm-on-install", Version: "0.12"},
			{PackageID: "pterm", ActionName: "Push", PackageReferenceName: "pterm", Version: "2000"},
			{PackageID: "pterm", ActionName: "Verify", PackageReferenceName: "pterm", Version: "2000"},
			{PackageID: "NuGet.CommandLine", ActionName: "Install", PackageReferenceName: "NuGet.CommandLine", Version: "5.4"},
		}, result)
	})

	t.Run("target both of package-ref:action where package referencename matches another package too", func(t *testing.T) {
		// real bug observed with manual testing
		packageSpec := []*create.StepPackageVersion{
			{PackageID: "pterm", ActionName: "Install", PackageReferenceName: "pterm-on-install", Version: "0.12"},
			{PackageID: "pterm", ActionName: "Push", PackageReferenceName: "pterm", Version: "0.12"},
			{PackageID: "pterm", ActionName: "Verify", PackageReferenceName: "pterm", Version: "0.12"},
			{PackageID: "NuGet.CommandLine", ActionName: "Install", PackageReferenceName: "NuGet.CommandLine", Version: "5.4"},
		}

		result := create.ApplyPackageOverrides(packageSpec, []*create.PackageVersionOverride{
			{PackageReferenceName: "pterm-on-install", ActionName: "Install", Version: "20000"},
		})

		assert.Equal(t, []*create.StepPackageVersion{
			{PackageID: "pterm", ActionName: "Install", PackageReferenceName: "pterm-on-install", Version: "20000"}, // only this one should be overridden
			{PackageID: "pterm", ActionName: "Push", PackageReferenceName: "pterm", Version: "0.12"},
			{PackageID: "pterm", ActionName: "Verify", PackageReferenceName: "pterm", Version: "0.12"},
			{PackageID: "NuGet.CommandLine", ActionName: "Install", PackageReferenceName: "NuGet.CommandLine", Version: "5.4"},
		}, result)
	})
}
