package versions_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/feeds"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/packages"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

var rootResource = testutil.NewRootResource()

func TestPackageVersions(t *testing.T) {
	const spaceID = "Spaces-1"
	space1 := fixtures.NewSpace(spaceID, "Default Space")

	builtInFeed, _ := feeds.NewBuiltInFeed("builtin")
	builtInFeed.ID = "Feeds-109"

	refTime := time.Date(2022, 9, 29, 11, 26, 0, 0, time.UTC)

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"package versions requires a package ID", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "versions"})
				return rootCmd.ExecuteC()
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "package must be specified")

			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{name: "lists all package versions", run: func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "versions", "--package", "pterm"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds?feedType=BuiltIn&take=1").RespondWith(&resources.Resources[feeds.IFeed]{
				Items: []feeds.IFeed{
					builtInFeed,
				},
			})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds/Feeds-109/packages/versions?packageId=pterm&take=2147483647").RespondWith(&resources.Resources[*packages.PackageVersion]{
				Items: []*packages.PackageVersion{
					{Version: "1.1", SizeBytes: 67890, Published: refTime.Add(5 * time.Hour)},
					{Version: "1.0.7", SizeBytes: 55231, Published: refTime.Add(2 * time.Hour)},
					{Version: "0.99-beta", SizeBytes: 53001, Published: refTime},
				},
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			// Note these tests print times in UTC all the time because our refTime is UTC. The server specifies DateTimeOffset, we believe go will
			// automagically convert these to local time and print them nicely in the real world
			assert.Equal(t, heredoc.Doc(`
			VERSION    PUBLISHED            SIZE
			1.1        2022-09-29 16:26:00  66.3 KiB
			1.0.7      2022-09-29 13:26:00  53.9 KiB
			0.99-beta  2022-09-29 11:26:00  51.8 KiB
			`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"pass through filter and limit", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "versions", "--package", "pterm", "--filter", "beta", "--limit", "1"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds?feedType=BuiltIn&take=1").RespondWith(&resources.Resources[feeds.IFeed]{
				Items: []feeds.IFeed{
					builtInFeed,
				},
			})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds/Feeds-109/packages/versions?packageId=pterm&take=1&filter=beta").RespondWith(&resources.Resources[*packages.PackageVersion]{
				Items: []*packages.PackageVersion{
					{Version: "0.99-beta", SizeBytes: 53001, Published: refTime},
				},
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, heredoc.Doc(`
			VERSION    PUBLISHED            SIZE
			0.99-beta  2022-09-29 11:26:00  51.8 KiB
			`), stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"outputformat json", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "versions", "--package", "pterm", "--output-format", "json"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds?feedType=BuiltIn&take=1").RespondWith(&resources.Resources[feeds.IFeed]{
				Items: []feeds.IFeed{
					builtInFeed,
				},
			})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds/Feeds-109/packages/versions?packageId=pterm&take=2147483647").RespondWith(&resources.Resources[*packages.PackageVersion]{
				Items: []*packages.PackageVersion{
					{Version: "1.1", SizeBytes: 67890, Published: refTime.Add(5 * time.Hour)},
					{Version: "1.0.7", SizeBytes: 55231, Published: refTime.Add(2 * time.Hour)},
					{Version: "0.99-beta", SizeBytes: 53001, Published: refTime},
				},
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)

			type x struct {
				Version   string
				Published time.Time
				Size      int64 // size in bytes
			}
			parsedStdout, err := testutil.ParseJsonStrict[[]x](stdOut)
			assert.Nil(t, err)

			assert.Equal(t, []x{
				{Version: "1.1", Size: 67890, Published: refTime.Add(5 * time.Hour)},
				{Version: "1.0.7", Size: 55231, Published: refTime.Add(2 * time.Hour)},
				{Version: "0.99-beta", Size: 53001, Published: refTime},
			}, parsedStdout)

			assert.Equal(t, "", stdErr.String())
		}},

		{"outputformat basic", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "versions", "--package", "pterm", "--output-format", "basic"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds?feedType=BuiltIn&take=1").RespondWith(&resources.Resources[feeds.IFeed]{
				Items: []feeds.IFeed{
					builtInFeed,
				},
			})
			api.ExpectRequest(t, "GET", "/api/Spaces-1/feeds/Feeds-109/packages/versions?packageId=pterm&take=2147483647").RespondWith(&resources.Resources[*packages.PackageVersion]{
				Items: []*packages.PackageVersion{
					{Version: "1.1", SizeBytes: 67890, Published: refTime.Add(5 * time.Hour)},
					{Version: "1.0.7", SizeBytes: 55231, Published: refTime.Add(2 * time.Hour)},
					{Version: "0.99-beta", SizeBytes: 53001, Published: refTime},
				},
			})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			assert.Equal(t, heredoc.Doc(`
			1.1
			1.0.7
			0.99-beta
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
