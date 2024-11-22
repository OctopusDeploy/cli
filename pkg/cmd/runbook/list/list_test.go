package list_test

import (
	"bytes"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

// if this were bigger we'd split out the AskQuestions into a seperate test group, but because there's only one we don't worry about it
func TestRunbookList(t *testing.T) {
	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	fireProject := fixtures.NewProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "")

	rbInventory := fixtures.NewRunbook(spaceID, fireProjectID, "Runbooks-1", "Inventory")
	rbInventory.Description = "Adds the machine to inventory"

	rbApplyPatches := fixtures.NewRunbook(spaceID, fireProjectID, "Runbooks-2", "Apply Patches")
	rbApplyPatches.Description = "Runs windows update"

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"runbook list requires a project name in automation mode", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "project must be specified")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"runbook list prompts for project name in interactive mode", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{fireProject})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the project to list runbooks for",
				Options: []string{fireProject.Name},
			}).AnswerWith(fireProject.Name)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/runbooks?take=2147483647").
				RespondWith(resources.Resources[*runbooks.Runbook]{
					Items: []*runbooks.Runbook{rbInventory, rbApplyPatches},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME           DESCRIPTION
				Inventory      Adds the machine to inventory
				Apply Patches  Runs windows update
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"runbook list picks up project from args in automation mode and prints list", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list", fireProject.Name, "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/runbooks?take=2147483647").
				RespondWith(resources.Resources[*runbooks.Runbook]{
					Items: []*runbooks.Runbook{rbInventory, rbApplyPatches},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME           DESCRIPTION
				Inventory      Adds the machine to inventory
				Apply Patches  Runs windows update
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},
		{"runbook list picks up project from flag in automation mode and prints list", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list", "--project", fireProject.Name, "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/runbooks?take=2147483647").
				RespondWith(resources.Resources[*runbooks.Runbook]{
					Items: []*runbooks.Runbook{rbInventory, rbApplyPatches},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME           DESCRIPTION
				Inventory      Adds the machine to inventory
				Apply Patches  Runs windows update
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},
		{"runbook list picks up project from short flag in automation mode and prints list", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list", "-p", fireProject.Name, "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/runbooks?take=2147483647").
				RespondWith(resources.Resources[*runbooks.Runbook]{
					Items: []*runbooks.Runbook{rbInventory, rbApplyPatches},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME           DESCRIPTION
				Inventory      Adds the machine to inventory
				Apply Patches  Runs windows update
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},
		{"runbook list limit and filter", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list", "-p", fireProject.Name, "--limit", "1", "--filter", "Apply", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/runbooks?take=1&partialName=Apply").
				RespondWith(resources.Resources[*runbooks.Runbook]{
					Items: []*runbooks.Runbook{rbApplyPatches},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME           DESCRIPTION
				Apply Patches  Runs windows update
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},
		{"outputFormat json", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list", "-p", fireProject.Name, "--output-format", "json", "--no-prompt"})
				return rootCmd.ExecuteC()
			})
			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/runbooks?take=2147483647").
				RespondWith(resources.Resources[*runbooks.Runbook]{
					Items: []*runbooks.Runbook{rbInventory, rbApplyPatches},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			type x struct {
				ID          string
				Name        string
				Description string
			}
			parsedStdout, err := testutil.ParseJsonStrict[[]x](stdOut)
			assert.Nil(t, err)

			assert.Equal(t, []x{{
				ID:          rbInventory.ID,
				Name:        rbInventory.Name,
				Description: rbInventory.Description,
			}, {
				ID:          rbApplyPatches.ID,
				Name:        rbApplyPatches.Name,
				Description: rbApplyPatches.Description,
			}}, parsedStdout)
			assert.Equal(t, "", stdErr.String())
		}},

		{"outputFormat basic", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list", "-p", fireProject.Name, "--output-format", "basic", "--no-prompt"})
				return rootCmd.ExecuteC()
			})
			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/runbooks?take=2147483647").
				RespondWith(resources.Resources[*runbooks.Runbook]{
					Items: []*runbooks.Runbook{rbInventory, rbApplyPatches},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				Inventory
				Apply Patches
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"outputFormat basic in interactive mode doesn't print 'helpful' extra information", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list", "-p", fireProject.Name, "--output-format", "basic"})
				return rootCmd.ExecuteC()
			})
			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/runbooks?take=2147483647").
				RespondWith(resources.Resources[*runbooks.Runbook]{
					Items: []*runbooks.Runbook{rbInventory, rbApplyPatches},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				Inventory
				Apply Patches
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			api, qa := testutil.NewMockServerAndAsker()
			askProvider := question.NewAskProvider(qa.AsAsker())
			fac := testutil.NewMockFactoryWithSpaceAndPrompt(api, space1, askProvider)
			rootCmd := cmdRoot.NewCmdRoot(fac, nil, askProvider)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)
			test.run(t, api, qa, rootCmd, stdout, stderr)
		})
	}

}

func TestGitRunbookList(t *testing.T) {
	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"

	space1 := fixtures.NewSpace(spaceID, "Default Space")

	fireProject := fixtures.NewVersionControlledProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "")

	fireProject.PersistenceSettings.(projects.GitPersistenceSettings).SetRunbooksAreInGit()

	rbInventory := fixtures.NewRunbook(spaceID, fireProjectID, "Runbooks-1", "Inventory")
	rbInventory.Description = "Adds the machine to inventory"

	rbApplyPatches := fixtures.NewRunbook(spaceID, fireProjectID, "Runbooks-2", "Apply Patches")
	rbApplyPatches.Description = "Runs windows update"

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"runbook list requires a project name in automation mode", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list", "--git-ref", "refs/heads/main", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "project must be specified")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"runbook list requires a Git reference in automation mode", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list", fireProject.Name, "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "git reference must be specified")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"runbook list prompts for project name and Git reference in interactive mode", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/all").RespondWith([]*projects.Project{fireProject})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the project to list runbooks for",
				Options: []string{fireProject.Name},
			}).AnswerWith(fireProject.Name)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProject.ID+"/git/branches").RespondWith(resources.Resources[*projects.GitReference]{
				PagedResults: resources.PagedResults{ItemType: "GitBranch"},
				Items: []*projects.GitReference{
					projects.NewGitBranchReference("main", "refs/heads/main"),
					projects.NewGitBranchReference("develop", "refs/heads/develop"),
				}})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/"+fireProject.ID+"/git/tags").RespondWith(resources.Resources[*projects.GitReference]{
				PagedResults: resources.PagedResults{ItemType: "GitTag"},
				Items: []*projects.GitReference{
					projects.NewGitTagReference("v2", "refs/tags/v2"),
					projects.NewGitTagReference("v1", "refs/tags/v1"),
				}})

			_ = qa.ExpectQuestion(t, &survey.Select{
				Message: "Select the Git reference to list runbooks for",
				Options: []string{"main (Branch)", "develop (Branch)", "v2 (Tag)", "v1 (Tag)"},
			}).AnswerWith("main (Branch)")

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/refs%2Fheads%2Fmain/runbooks?take=2147483647").
				RespondWith(resources.Resources[*runbooks.Runbook]{
					Items: []*runbooks.Runbook{rbInventory, rbApplyPatches},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME           DESCRIPTION
				Inventory      Adds the machine to inventory
				Apply Patches  Runs windows update
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"runbook list picks up project from args in automation mode and prints list", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list", fireProject.Name, "--git-ref", "refs/heads/main", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/refs%2Fheads%2Fmain/runbooks?take=2147483647").
				RespondWith(resources.Resources[*runbooks.Runbook]{
					Items: []*runbooks.Runbook{rbInventory, rbApplyPatches},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME           DESCRIPTION
				Inventory      Adds the machine to inventory
				Apply Patches  Runs windows update
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},
		{"runbook list picks up project from flag in automation mode and prints list", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list", "--project", fireProject.Name, "--git-ref", "refs/heads/main", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/refs%2Fheads%2Fmain/runbooks?take=2147483647").
				RespondWith(resources.Resources[*runbooks.Runbook]{
					Items: []*runbooks.Runbook{rbInventory, rbApplyPatches},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME           DESCRIPTION
				Inventory      Adds the machine to inventory
				Apply Patches  Runs windows update
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},
		{"runbook list picks up project from short flag in automation mode and prints list", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list", "-p", fireProject.Name, "--git-ref", "refs/heads/main", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/refs%2Fheads%2Fmain/runbooks?take=2147483647").
				RespondWith(resources.Resources[*runbooks.Runbook]{
					Items: []*runbooks.Runbook{rbInventory, rbApplyPatches},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME           DESCRIPTION
				Inventory      Adds the machine to inventory
				Apply Patches  Runs windows update
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"runbook list limit and filter", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list", "-p", fireProject.Name, "--git-ref", "refs/heads/main", "--limit", "1", "--filter", "Apply", "--no-prompt", "-f", "table"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/refs%2Fheads%2Fmain/runbooks?take=1&partialName=Apply").
				RespondWith(resources.Resources[*runbooks.Runbook]{
					Items: []*runbooks.Runbook{rbApplyPatches},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				NAME           DESCRIPTION
				Apply Patches  Runs windows update
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"outputFormat json", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list", "-p", fireProject.Name, "--git-ref", "refs/heads/main", "--output-format", "json", "--no-prompt"})
				return rootCmd.ExecuteC()
			})
			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/refs%2Fheads%2Fmain/runbooks?take=2147483647").
				RespondWith(resources.Resources[*runbooks.Runbook]{
					Items: []*runbooks.Runbook{rbInventory, rbApplyPatches},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			type x struct {
				ID          string
				Name        string
				Description string
			}
			parsedStdout, err := testutil.ParseJsonStrict[[]x](stdOut)
			assert.Nil(t, err)

			assert.Equal(t, []x{{
				ID:          rbInventory.ID,
				Name:        rbInventory.Name,
				Description: rbInventory.Description,
			}, {
				ID:          rbApplyPatches.ID,
				Name:        rbApplyPatches.Name,
				Description: rbApplyPatches.Description,
			}}, parsedStdout)
			assert.Equal(t, "", stdErr.String())
		}},

		{"outputFormat basic", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list", "-p", fireProject.Name, "--git-ref", "refs/heads/main", "--output-format", "basic", "--no-prompt"})
				return rootCmd.ExecuteC()
			})
			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/refs%2Fheads%2Fmain/runbooks?take=2147483647").
				RespondWith(resources.Resources[*runbooks.Runbook]{
					Items: []*runbooks.Runbook{rbInventory, rbApplyPatches},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				Inventory
				Apply Patches
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"outputFormat basic in interactive mode doesn't print 'helpful' extra information", func(t *testing.T, api *testutil.MockHttpServer, qa *testutil.AskMocker, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"runbook", "list", "-p", fireProject.Name, "--git-ref", "refs/heads/main", "--output-format", "basic"})
				return rootCmd.ExecuteC()
			})
			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", nil)
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects?partialName=Fire+Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Projects-22/refs%2Fheads%2Fmain/runbooks?take=2147483647").
				RespondWith(resources.Resources[*runbooks.Runbook]{
					Items: []*runbooks.Runbook{rbInventory, rbApplyPatches},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			assert.Equal(t, heredoc.Doc(`
				Inventory
				Apply Patches
				`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			api, qa := testutil.NewMockServerAndAsker()
			askProvider := question.NewAskProvider(qa.AsAsker())
			fac := testutil.NewMockFactoryWithSpaceAndPrompt(api, space1, askProvider)
			rootCmd := cmdRoot.NewCmdRoot(fac, nil, askProvider)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)
			test.run(t, api, qa, rootCmd, stdout, stderr)
		})
	}

}
