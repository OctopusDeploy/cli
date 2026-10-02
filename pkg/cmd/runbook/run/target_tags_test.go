package run

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/stretchr/testify/assert"
)

// newTagSetStep builds a preview step offering a single tag set, which is the shape
// that the canonical tag names are collected out of.
func newTagSetStep(tagSetName string, tagNames ...string) *deployments.DeploymentTemplateStep {
	tags := make([]*deployments.TargetTagPreview, 0, len(tagNames))
	for _, name := range tagNames {
		tags = append(tags, &deployments.TargetTagPreview{TagName: name})
	}
	return &deployments.DeploymentTemplateStep{
		AvailableTagSets: []*deployments.TagSetPreview{
			{TagSetName: tagSetName, AvailableTags: tags},
		},
	}
}

func TestRunbookRun_AskTargetTags(t *testing.T) {
	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"
	const fireRunbookID = "Runbooks-33"
	const fireSnapshotID = "RunbookSnapshots-44"

	serverUrl, _ := url.Parse("http://server")
	const placeholderApiKey = "API-XXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
	rootResource := testutil.NewRootResource()

	devEnvironment := fixtures.NewEnvironment(spaceID, "Environments-12", "dev")
	scratchEnvironment := fixtures.NewEnvironment(spaceID, "Environments-13", "scratch")

	snapshotPreviewPath := func(environmentID string) string {
		return fmt.Sprintf("/api/%s/runbookSnapshots/%s/runbookRuns/preview/%s?includeDisabledSteps=true", spaceID, fireSnapshotID, environmentID)
	}
	gitPreviewPath := func(gitRef string, environmentID string) string {
		return fmt.Sprintf("/api/%s/projects/%s/%s/runbooks/%s/runbookRuns/preview/%s?includeDisabledSteps=true", spaceID, fireProjectID, gitRef, fireRunbookID, environmentID)
	}

	doStandardApiResponses := func(api *testutil.MockHttpServer) {
		api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
		api.ExpectRequest(t, "GET", "/api/spaces").RespondWith(rootResource)
	}

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker)
	}{
		{"target tags with specific tags selected", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker) {
			receiver := testutil.GoBegin3(func() ([]string, []string, error) {
				defer testutil.Close(api, qa)
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return askRunbookTargetTags(octopus, qa.AsAsker(), spaceID, fireSnapshotID, []*environments.Environment{devEnvironment})
			})

			doStandardApiResponses(api)

			api.ExpectRequest(t, "GET", snapshotPreviewPath(devEnvironment.ID)).RespondWith(&runbooks.RunPreview{
				StepsToExecute: []*deployments.DeploymentTemplateStep{
					newTagSetStep("Role", "WebServer", "Database", "Legacy"),
					newTagSetStep("Environment", "Production", "Staging"),
				},
			})

			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Specific target tags to include (If none selected, include all)",
				Options: []string{"Environment/Production", "Environment/Staging", "Role/Database", "Role/Legacy", "Role/WebServer"},
			}).AnswerWith([]string{"Role/WebServer", "Environment/Production"})

			specific, excluded, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []string{"Role/WebServer", "Environment/Production"}, specific)
			assert.Nil(t, excluded)
		}},

		{"target tags with excluded tags selected", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker) {
			receiver := testutil.GoBegin3(func() ([]string, []string, error) {
				defer testutil.Close(api, qa)
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return askRunbookTargetTags(octopus, qa.AsAsker(), spaceID, fireSnapshotID, []*environments.Environment{devEnvironment})
			})

			doStandardApiResponses(api)

			api.ExpectRequest(t, "GET", snapshotPreviewPath(devEnvironment.ID)).RespondWith(&runbooks.RunPreview{
				StepsToExecute: []*deployments.DeploymentTemplateStep{
					newTagSetStep("Role", "WebServer", "Database", "Legacy"),
					newTagSetStep("Environment", "Production", "Staging"),
				},
			})

			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Specific target tags to include (If none selected, include all)",
				Options: []string{"Environment/Production", "Environment/Staging", "Role/Database", "Role/Legacy", "Role/WebServer"},
			}).AnswerWith([]string{}) // Selecting no specific tags to allow testing excluded tags

			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Target tags to exclude (If none selected, exclude none)",
				Options: []string{"Environment/Production", "Environment/Staging", "Role/Database", "Role/Legacy", "Role/WebServer"},
			}).AnswerWith([]string{"Role/Legacy"})

			specific, excluded, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Nil(t, specific)
			assert.Equal(t, []string{"Role/Legacy"}, excluded)
		}},

		{"target tags are deduped and sorted across multiple environments", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker) {
			receiver := testutil.GoBegin3(func() ([]string, []string, error) {
				defer testutil.Close(api, qa)
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return askRunbookTargetTags(octopus, qa.AsAsker(), spaceID, fireSnapshotID, []*environments.Environment{devEnvironment, scratchEnvironment})
			})

			doStandardApiResponses(api)

			api.ExpectRequest(t, "GET", snapshotPreviewPath(devEnvironment.ID)).RespondWith(&runbooks.RunPreview{
				StepsToExecute: []*deployments.DeploymentTemplateStep{
					newTagSetStep("Role", "WebServer"),
					newTagSetStep("Environment", "Staging"),
				},
			})
			api.ExpectRequest(t, "GET", snapshotPreviewPath(scratchEnvironment.ID)).RespondWith(&runbooks.RunPreview{
				StepsToExecute: []*deployments.DeploymentTemplateStep{
					newTagSetStep("Role", "WebServer"), // deliberate double up
					newTagSetStep("Role", "Database"),
				},
			})

			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Specific target tags to include (If none selected, include all)",
				Options: []string{"Environment/Staging", "Role/Database", "Role/WebServer"},
			}).AnswerWith([]string{"Role/Database"})

			specific, excluded, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Equal(t, []string{"Role/Database"}, specific)
			assert.Nil(t, excluded)
		}},

		{"target tag questions are skipped when the previews offer no tags", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker) {
			receiver := testutil.GoBegin3(func() ([]string, []string, error) {
				defer testutil.Close(api, qa)
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return askRunbookTargetTags(octopus, qa.AsAsker(), spaceID, fireSnapshotID, []*environments.Environment{devEnvironment})
			})

			doStandardApiResponses(api)

			api.ExpectRequest(t, "GET", snapshotPreviewPath(devEnvironment.ID)).RespondWith(&runbooks.RunPreview{
				StepsToExecute: []*deployments.DeploymentTemplateStep{{}},
			})

			specific, excluded, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Nil(t, specific)
			assert.Nil(t, excluded)
		}},

		{"git runbook target tags with excluded tags selected", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker) {
			const gitRef = "main"

			receiver := testutil.GoBegin3(func() ([]string, []string, error) {
				defer testutil.Close(api, qa)
				octopus, _ := octopusApiClient.NewClient(testutil.NewMockHttpClientWithTransport(api), serverUrl, placeholderApiKey, "")
				return askGitRunbookTargetTags(octopus, qa.AsAsker(), spaceID, fireProjectID, fireRunbookID, gitRef, []*environments.Environment{devEnvironment})
			})

			doStandardApiResponses(api)

			api.ExpectRequest(t, "GET", gitPreviewPath(gitRef, devEnvironment.ID)).RespondWith(&runbooks.RunPreview{
				StepsToExecute: []*deployments.DeploymentTemplateStep{
					newTagSetStep("Role", "WebServer", "Legacy"),
				},
			})

			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Specific target tags to include (If none selected, include all)",
				Options: []string{"Role/Legacy", "Role/WebServer"},
			}).AnswerWith([]string{})

			_ = qa.ExpectQuestion(t, &survey.MultiSelect{
				Message: "Target tags to exclude (If none selected, exclude none)",
				Options: []string{"Role/Legacy", "Role/WebServer"},
			}).AnswerWith([]string{"Role/Legacy"})

			specific, excluded, err := testutil.ReceiveTriple(receiver)
			assert.Nil(t, err)
			assert.Nil(t, specific)
			assert.Equal(t, []string{"Role/Legacy"}, excluded)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api, qa := testutil.NewMockServerAndAsker()
			test.run(t, api, qa)
		})
	}
}
