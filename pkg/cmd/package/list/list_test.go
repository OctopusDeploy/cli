package list_test

import (
	"bytes"
	"github.com/MakeNowJust/heredoc/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/packages"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"testing"
)

var rootResource = testutil.NewRootResource()

func TestPackageList(t *testing.T) {
	const spaceID = "Spaces-1"
	space1 := fixtures.NewSpace(spaceID, "Default Space")

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"list all packages by default", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "list"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/packages?take=2147483647").RespondWith(&resources.Resources[*packages.Package]{
				Items: []*packages.Package{
					{PackageID: "pterm", Version: "0.12.51", Description: "some sort of package"},
					{PackageID: "NuGet.CommandLine", Version: "6.2.1"},
				},
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, heredoc.Doc(`
        	ID                 HIGHEST VERSION  DESCRIPTION
        	pterm              0.12.51          some sort of package
        	NuGet.CommandLine  6.2.1            
			`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"pass through filter and limit", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "list", "--filter", "pterm", "--limit", "1"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/packages?filter=pterm&take=1").RespondWith(&resources.Resources[*packages.Package]{
				Items: []*packages.Package{
					{PackageID: "pterm", Version: "0.12.51", Description: "some sort of package"},
				},
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, heredoc.Doc(`
        	ID     HIGHEST VERSION  DESCRIPTION
        	pterm  0.12.51          some sort of package
			`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"outputformat json", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "list", "--output-format", "json"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/packages?take=2147483647").RespondWith(&resources.Resources[*packages.Package]{
				Items: []*packages.Package{
					{PackageID: "pterm", Version: "0.12.51", Description: "some sort of package"},
					{PackageID: "NuGet.CommandLine", Version: "6.2.1"},
				},
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			type x struct {
				ID          string
				Version     string
				Description string
			}
			parsedStdout, err := testutil.ParseJsonStrict[[]x](stdOut)
			assert.Nil(t, err)

			assert.Equal(t, []x{
				{ID: "pterm", Version: "0.12.51", Description: "some sort of package"},
				{ID: "NuGet.CommandLine", Version: "6.2.1"},
			}, parsedStdout)

			assert.Equal(t, "", stdErr.String())
		}},

		{"outputformat basic", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "list", "--output-format", "basic"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/packages?take=2147483647").RespondWith(&resources.Resources[*packages.Package]{
				Items: []*packages.Package{
					{PackageID: "pterm", Version: "0.12.51", Description: "some sort of package"},
					{PackageID: "NuGet.CommandLine", Version: "6.2.1"},
				},
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, heredoc.Doc(`
			pterm
			NuGet.CommandLine
			`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			api := testutil.NewMockHttpServer()
			fac := testutil.NewMockFactoryWithSpaceAndPrompt(api, space1, nil)
			rootCmd := cmdRoot.NewCmdRoot(fac, nil, nil)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)
			test.run(t, api, rootCmd, stdout, stderr)
		})
	}
}
