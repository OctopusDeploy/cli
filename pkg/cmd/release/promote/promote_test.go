package promote_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/dashboard"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/OctopusDeploy/cli/pkg/cmd/release/promote"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
)

var rootResource = testutil.NewRootResource()

var now = func() time.Time {
	return time.Date(2022, time.September, 8, 13, 25, 2, 0, time.FixedZone("Malaysia", 8*3600)) // UTC+8
}

var ctxWithFakeNow = context.WithValue(context.TODO(), constants.ContextKeyTimeNow, now)

func TestPromoteRelease_Success(t *testing.T) {
	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"

	space1 := fixtures.NewSpace(spaceID, "Default Space")
	fireProject := fixtures.NewProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "deploymentprocess-"+fireProjectID)

	devEnvironment := fixtures.NewEnvironment(spaceID, "Environments-12", "Development")
	stagingEnvironment := fixtures.NewEnvironment(spaceID, "Environments-13", "Staging")

	release19 := fixtures.NewRelease(spaceID, "Releases-193", "1.9", fireProjectID, "Channels-1")
	release20 := fixtures.NewRelease(spaceID, "Releases-194", "2.0", fireProjectID, "Channels-1")
	release21 := fixtures.NewRelease(spaceID, "Releases-195", "2.1", fireProjectID, "Channels-1")

	dashboardItem := &dashboard.DashboardItem{
		ProjectID:      fireProjectID,
		EnvironmentID:  devEnvironment.ID,
		ReleaseVersion: release19.Version,
		ReleaseID:      release19.ID,
		State:          "Success",
	}

	release20DashboardItem := &dashboard.DashboardItem{
		ProjectID:      fireProjectID,
		EnvironmentID:  devEnvironment.ID,
		ReleaseVersion: release20.Version,
		ReleaseID:      release20.ID,
		State:          "Failed",
	}

	release21DashboardItem := &dashboard.DashboardItem{
		ProjectID:      fireProjectID,
		EnvironmentID:  devEnvironment.ID,
		ReleaseVersion: release21.Version,
		ReleaseID:      release21.ID,
		State:          "Executing",
	}

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"promote release from dev to staging", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "promote", "--project", "Fire Project", "--from", "Development", "--to", "Staging"})
				return rootCmd, rootCmd.Execute()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/"+spaceID).RespondWith(rootResource)

			// Lookup project by name
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", struct{ ErrorMessage string }{ErrorMessage: "Project not found"})
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects?partialName=Fire%20Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			// Lookup from environment by ID
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=&skip=0&ids=Development&take=0").
				RespondWith(resources.Resources[*environments.Environment]{
					Items: []*environments.Environment{},
				})

			// Lookup from environment by name
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=Development&skip=0&take=0").
				RespondWith(resources.Resources[*environments.Environment]{
					Items: []*environments.Environment{devEnvironment},
				})

			// Get dashboard for project and environment
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/dashboard/dynamic?environments=Environments-12&projects=Projects-22").RespondWith(&dashboard.Dashboard{
				Items: []*dashboard.DashboardItem{dashboardItem},
			})

			// Get the release
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/releases/"+release19.Version).
				RespondWith(release19)

			// Deploy the release to staging
			req := api.ExpectRequest(t, "POST", "/api/"+spaceID+"/deployments/create/untenanted/v1")
			requestBody, err := testutil.ReadJson[deployments.CreateDeploymentUntenantedCommandV1](req.Request.Body)
			assert.Nil(t, err)

			assert.Equal(t, deployments.CreateDeploymentUntenantedCommandV1{
				ReleaseVersion:   release19.Version,
				EnvironmentNames: []string{stagingEnvironment.Name},
				CreateExecutionAbstractCommandV1: deployments.CreateExecutionAbstractCommandV1{
					SpaceID:         spaceID,
					ProjectIDOrName: fireProject.ID,
				},
			}, requestBody)

			req.RespondWith(&deployments.CreateDeploymentResponseV1{
				DeploymentServerTasks: []*deployments.DeploymentServerTask{
					{
						ServerTaskID: "ServerTasks-1",
						DeploymentID: "Deployments-1",
					},
				},
			})

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
		}},

		{"promote most recent release from dev to staging", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "promote", "--project", "Fire Project", "--from", "Development", "--to", "Staging"})
				return rootCmd, rootCmd.Execute()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/"+spaceID).RespondWith(rootResource)

			// Lookup project by name
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", struct{ ErrorMessage string }{ErrorMessage: "Project not found"})
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects?partialName=Fire%20Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			// Lookup from environment by ID
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=&skip=0&ids=Development&take=0").
				RespondWith(resources.Resources[*environments.Environment]{
					Items: []*environments.Environment{},
				})

			// Lookup from environment by name
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=Development&skip=0&take=0").
				RespondWith(resources.Resources[*environments.Environment]{
					Items: []*environments.Environment{devEnvironment},
				})

			// Get dashboard for project and environment
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/dashboard/dynamic?environments=Environments-12&projects=Projects-22").RespondWith(&dashboard.Dashboard{
				Items: []*dashboard.DashboardItem{release20DashboardItem, dashboardItem, release21DashboardItem},
			})

			// Get the release
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/releases/"+release21.Version).
				RespondWith(release21)

			// Deploy the release to staging
			req := api.ExpectRequest(t, "POST", "/api/"+spaceID+"/deployments/create/untenanted/v1")
			requestBody, err := testutil.ReadJson[deployments.CreateDeploymentUntenantedCommandV1](req.Request.Body)
			assert.Nil(t, err)

			assert.Equal(t, deployments.CreateDeploymentUntenantedCommandV1{
				ReleaseVersion:   release21.Version,
				EnvironmentNames: []string{stagingEnvironment.Name},
				CreateExecutionAbstractCommandV1: deployments.CreateExecutionAbstractCommandV1{
					SpaceID:         spaceID,
					ProjectIDOrName: fireProject.ID,
				},
			}, requestBody)

			req.RespondWith(&deployments.CreateDeploymentResponseV1{
				DeploymentServerTasks: []*deployments.DeploymentServerTask{
					{
						ServerTaskID: "ServerTasks-1",
						DeploymentID: "Deployments-1",
					},
				},
			})

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
		}},

		{"promote latest successful release from dev to staging", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "promote", "--project", "Fire Project", "--from", "Development", "--to", "Staging", "--latest-successful"})
				return rootCmd, rootCmd.Execute()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/"+spaceID).RespondWith(rootResource)

			// Lookup project by name
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", struct{ ErrorMessage string }{ErrorMessage: "Project not found"})
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects?partialName=Fire%20Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			// Lookup from environment by ID
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=&skip=0&ids=Development&take=0").
				RespondWith(resources.Resources[*environments.Environment]{
					Items: []*environments.Environment{},
				})

			// Lookup from environment by name
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=Development&skip=0&take=0").
				RespondWith(resources.Resources[*environments.Environment]{
					Items: []*environments.Environment{devEnvironment},
				})

			// Get dashboard for project and environment with IncludePrevious=true
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/dashboard/dynamic?environments=Environments-12&includePrevious=true&projects=Projects-22").RespondWith(&dashboard.Dashboard{
				Items: []*dashboard.DashboardItem{release20DashboardItem, dashboardItem},
			})

			// Get the release
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/releases/"+release19.Version).
				RespondWith(release19)

			// Deploy the release to staging
			api.ExpectRequest(t, "POST", "/api/"+spaceID+"/deployments/create/untenanted/v1").
				RespondWith(&deployments.CreateDeploymentResponseV1{
					DeploymentServerTasks: []*deployments.DeploymentServerTask{
						{
							ServerTaskID: "ServerTasks-1",
							DeploymentID: "Deployments-1",
						},
					},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
		}},

		{"promote release with update variables", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "promote", "--project", "Fire Project", "--from", "Development", "--to", "Staging", "--update-variables"})
				return rootCmd, rootCmd.Execute()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/"+spaceID).RespondWith(rootResource)

			// Lookup project by name
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", struct{ ErrorMessage string }{ErrorMessage: "Project not found"})
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects?partialName=Fire%20Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			// Lookup from environment by ID
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=&skip=0&ids=Development&take=0").
				RespondWith(resources.Resources[*environments.Environment]{
					Items: []*environments.Environment{},
				})

			// Lookup from environment by name
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=Development&skip=0&take=0").
				RespondWith(resources.Resources[*environments.Environment]{
					Items: []*environments.Environment{devEnvironment},
				})

			// Get dashboard for project and environment
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/dashboard/dynamic?environments=Environments-12&projects=Projects-22").RespondWith(&dashboard.Dashboard{
				Items: []*dashboard.DashboardItem{dashboardItem},
			})

			// Get the release
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/releases/"+release19.Version).
				RespondWith(release19)

			// Deploy the release to staging with update variables
			api.ExpectRequest(t, "POST", "/api/"+spaceID+"/deployments/create/untenanted/v1").
				RespondWith(&deployments.CreateDeploymentResponseV1{
					DeploymentServerTasks: []*deployments.DeploymentServerTask{
						{
							ServerTaskID: "ServerTasks-1",
							DeploymentID: "Deployments-1",
						},
					},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
		}},

		{"promote release with tenanted deployment", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "promote", "--project", "Fire Project", "--from", "Development", "--to", "Staging", "--tenant", "Acme Corp"})
				return rootCmd, rootCmd.Execute()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/"+spaceID).RespondWith(rootResource)

			// Lookup project by name
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", struct{ ErrorMessage string }{ErrorMessage: "Project not found"})
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects?partialName=Fire%20Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			// Lookup from environment by ID
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=&skip=0&ids=Development&take=0").
				RespondWith(resources.Resources[*environments.Environment]{
					Items: []*environments.Environment{},
				})

			// Lookup from environment by name
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=Development&skip=0&take=0").
				RespondWith(resources.Resources[*environments.Environment]{
					Items: []*environments.Environment{devEnvironment},
				})

			// Get dashboard for project and environment
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/dashboard/dynamic?environments=Environments-12&projects=Projects-22").RespondWith(&dashboard.Dashboard{
				Items: []*dashboard.DashboardItem{dashboardItem},
			})

			// Get the release
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/releases/"+release19.Version).
				RespondWith(release19)

			// Deploy the release to staging with tenants
			api.ExpectRequest(t, "POST", "/api/"+spaceID+"/deployments/create/tenanted/v1").
				RespondWith(&deployments.CreateDeploymentResponseV1{
					DeploymentServerTasks: []*deployments.DeploymentServerTask{
						{
							ServerTaskID: "ServerTasks-1",
							DeploymentID: "Deployments-1",
						},
					},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
		}},

		{"promote release using legacy alias deployTo", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "promote", "--project", "Fire Project", "--from", "Development", "--deployTo", "Staging"})
				return rootCmd, rootCmd.Execute()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/"+spaceID).RespondWith(rootResource)

			// Lookup project by name
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", struct{ ErrorMessage string }{ErrorMessage: "Project not found"})
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects?partialName=Fire%20Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			// Lookup from environment by ID
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=&skip=0&ids=Development&take=0").
				RespondWith(resources.Resources[*environments.Environment]{
					Items: []*environments.Environment{},
				})

			// Lookup from environment by name
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=Development&skip=0&take=0").
				RespondWith(resources.Resources[*environments.Environment]{
					Items: []*environments.Environment{devEnvironment},
				})

			// Get dashboard for project and environment
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/dashboard/dynamic?environments=Environments-12&projects=Projects-22").RespondWith(&dashboard.Dashboard{
				Items: []*dashboard.DashboardItem{dashboardItem},
			})

			// Get the release
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/releases/"+release19.Version).
				RespondWith(release19)

			// Deploy the release to staging
			api.ExpectRequest(t, "POST", "/api/"+spaceID+"/deployments/create/untenanted/v1").
				RespondWith(&deployments.CreateDeploymentResponseV1{
					DeploymentServerTasks: []*deployments.DeploymentServerTask{
						{
							ServerTaskID: "ServerTasks-1",
							DeploymentID: "Deployments-1",
						},
					},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
		}},

		{"promote release using legacy alias latestSuccessful", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "promote", "--project", "Fire Project", "--from", "Development", "--to", "Staging", "--latestSuccessful"})
				return rootCmd, rootCmd.Execute()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/"+spaceID).RespondWith(rootResource)

			// Lookup project by name
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", struct{ ErrorMessage string }{ErrorMessage: "Project not found"})
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects?partialName=Fire%20Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			// Lookup from environment by ID
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=&skip=0&ids=Development&take=0").
				RespondWith(resources.Resources[*environments.Environment]{
					Items: []*environments.Environment{},
				})

			// Lookup from environment by name
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=Development&skip=0&take=0").
				RespondWith(resources.Resources[*environments.Environment]{
					Items: []*environments.Environment{devEnvironment},
				})

			// Get dashboard for project and environment with IncludePrevious=true
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/dashboard/dynamic?environments=Environments-12&includePrevious=true&projects=Projects-22").RespondWith(&dashboard.Dashboard{
				Items: []*dashboard.DashboardItem{dashboardItem},
			})

			// Get the release
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/releases/"+release19.Version).
				RespondWith(release19)

			// Deploy the release to staging
			api.ExpectRequest(t, "POST", "/api/"+spaceID+"/deployments/create/untenanted/v1").
				RespondWith(&deployments.CreateDeploymentResponseV1{
					DeploymentServerTasks: []*deployments.DeploymentServerTask{
						{
							ServerTaskID: "ServerTasks-1",
							DeploymentID: "Deployments-1",
						},
					},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
		}},

		{"promote release using legacy alias updateVariables", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"release", "promote", "--project", "Fire Project", "--from", "Development", "--to", "Staging", "--updateVariables"})
				return rootCmd, rootCmd.Execute()
			})

			api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/"+spaceID).RespondWith(rootResource)

			// Lookup project by name
			api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", struct{ ErrorMessage string }{ErrorMessage: "Project not found"})
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects?partialName=Fire%20Project").
				RespondWith(resources.Resources[*projects.Project]{
					Items: []*projects.Project{fireProject},
				})

			// Lookup from environment by ID
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=&skip=0&ids=Development&take=0").
				RespondWith(resources.Resources[*environments.Environment]{
					Items: []*environments.Environment{},
				})

			// Lookup from environment by name
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=Development&skip=0&take=0").
				RespondWith(resources.Resources[*environments.Environment]{
					Items: []*environments.Environment{devEnvironment},
				})

			// Get dashboard for project and environment
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/dashboard/dynamic?environments=Environments-12&projects=Projects-22").RespondWith(&dashboard.Dashboard{
				Items: []*dashboard.DashboardItem{dashboardItem},
			})

			// Get the release
			api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/"+fireProjectID+"/releases/"+release19.Version).
				RespondWith(release19)

			// Deploy the release to staging with update variables
			api.ExpectRequest(t, "POST", "/api/"+spaceID+"/deployments/create/untenanted/v1").
				RespondWith(&deployments.CreateDeploymentResponseV1{
					DeploymentServerTasks: []*deployments.DeploymentServerTask{
						{
							ServerTaskID: "ServerTasks-1",
							DeploymentID: "Deployments-1",
						},
					},
				})

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout := new(bytes.Buffer)
			stderr := new(bytes.Buffer)
			api := testutil.NewMockHttpServer()

			rootCmd := cmdRoot.NewCmdRoot(testutil.NewMockFactoryWithSpace(api, space1), nil, nil)
			rootCmd.SetContext(ctxWithFakeNow)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)

			test.run(t, api, rootCmd, stdout, stderr)
		})
	}
}

func TestPromoteRelease_Errors(t *testing.T) {
	const spaceID = "Spaces-1"
	const fireProjectID = "Projects-22"

	space1 := fixtures.NewSpace(spaceID, "Default Space")
	fireProject := fixtures.NewProject(spaceID, fireProjectID, "Fire Project", "Lifecycles-1", "ProjectGroups-1", "deploymentprocess-"+fireProjectID)

	devEnvironment := fixtures.NewEnvironment(spaceID, "Environments-12", "Development")

	tests := []struct {
		name          string
		args          []string
		expectedError string
		setupApi      func(api *testutil.MockHttpServer)
	}{
		{
			name: "project not found",
			args: []string{"release", "promote", "--project", "NonExistent Project", "--from", "Development", "--to", "Staging"},
			setupApi: func(api *testutil.MockHttpServer) {
				api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
				api.ExpectRequest(t, "GET", "/api/"+spaceID).RespondWith(rootResource)
				api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects/NonExistent Project").RespondWithStatus(404, "NotFound", struct{ ErrorMessage string }{ErrorMessage: "Project not found"})
				api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects?partialName=NonExistent%20Project").
					RespondWith(resources.Resources[*projects.Project]{
						Items: []*projects.Project{},
					})
			},
			expectedError: "project not found",
		},
		{
			name: "from environment not found",
			args: []string{"release", "promote", "--project", "Fire Project", "--from", "NonExistent", "--to", "Staging"},
			setupApi: func(api *testutil.MockHttpServer) {
				api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
				api.ExpectRequest(t, "GET", "/api/"+spaceID).RespondWith(rootResource)
				api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", struct{ ErrorMessage string }{ErrorMessage: "Project not found"})
				api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects?partialName=Fire%20Project").
					RespondWith(resources.Resources[*projects.Project]{
						Items: []*projects.Project{fireProject},
					})
				api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=&skip=0&ids=NonExistent&take=0").
					RespondWith(resources.Resources[*environments.Environment]{
						Items: []*environments.Environment{},
					})
				api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=NonExistent&skip=0&take=0").
					RespondWith(resources.Resources[*environments.Environment]{
						Items: []*environments.Environment{},
					})
			},
			expectedError: "environment not found",
		},
		{
			name: "no deployment found in from environment",
			args: []string{"release", "promote", "--project", "Fire Project", "--from", "Development", "--to", "Staging"},
			setupApi: func(api *testutil.MockHttpServer) {
				api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
				api.ExpectRequest(t, "GET", "/api/"+spaceID).RespondWith(rootResource)
				api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", struct{ ErrorMessage string }{ErrorMessage: "Project not found"})
				api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects?partialName=Fire%20Project").
					RespondWith(resources.Resources[*projects.Project]{
						Items: []*projects.Project{fireProject},
					})
				api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=&skip=0&ids=Development&take=0").
					RespondWith(resources.Resources[*environments.Environment]{
						Items: []*environments.Environment{},
					})
				api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=Development&skip=0&take=0").
					RespondWith(resources.Resources[*environments.Environment]{
						Items: []*environments.Environment{devEnvironment},
					})
				api.ExpectRequest(t, "GET", "/api/"+spaceID+"/dashboard/dynamic?environments=Environments-12&projects=Projects-22").RespondWith(&dashboard.Dashboard{
					Items: []*dashboard.DashboardItem{},
				})
			},
			expectedError: "Could not find the latest deployment of the project for this environment",
		},
		{
			name: "no successful deployment found when using --latest-successful",
			args: []string{"release", "promote", "--project", "Fire Project", "--from", "Development", "--to", "Staging", "--latest-successful"},
			setupApi: func(api *testutil.MockHttpServer) {
				api.ExpectRequest(t, "GET", "/api/").RespondWith(rootResource)
				api.ExpectRequest(t, "GET", "/api/"+spaceID).RespondWith(rootResource)
				api.ExpectRequest(t, "GET", "/api/Spaces-1/projects/Fire Project").RespondWithStatus(404, "NotFound", struct{ ErrorMessage string }{ErrorMessage: "Project not found"})
				api.ExpectRequest(t, "GET", "/api/"+spaceID+"/projects?partialName=Fire%20Project").
					RespondWith(resources.Resources[*projects.Project]{
						Items: []*projects.Project{fireProject},
					})
				api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=&skip=0&ids=Development&take=0").
					RespondWith(resources.Resources[*environments.Environment]{
						Items: []*environments.Environment{},
					})
				api.ExpectRequest(t, "GET", "/api/"+spaceID+"/environments/v2?name=Development&skip=0&take=0").
					RespondWith(resources.Resources[*environments.Environment]{
						Items: []*environments.Environment{devEnvironment},
					})
				api.ExpectRequest(t, "GET", "/api/"+spaceID+"/dashboard/dynamic?environments=Environments-12&includePrevious=true&projects=Projects-22").RespondWith(&dashboard.Dashboard{
					Items: []*dashboard.DashboardItem{
						{
							ProjectID:      fireProjectID,
							EnvironmentID:  devEnvironment.ID,
							ReleaseVersion: "1.9",
							ReleaseID:      "Releases-193",
							State:          "Failed", // Not successful
						},
					},
				})
			},
			expectedError: "Could not find the latest successful deployment of the project for this environment",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout := new(bytes.Buffer)
			stderr := new(bytes.Buffer)
			api := testutil.NewMockHttpServer()

			rootCmd := cmdRoot.NewCmdRoot(testutil.NewMockFactoryWithSpace(api, space1), nil, nil)
			rootCmd.SetContext(ctxWithFakeNow)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)

			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs(test.args)
				return rootCmd, rootCmd.Execute()
			})

			if test.setupApi != nil {
				test.setupApi(api)
			}

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), test.expectedError)
		})
	}
}

func TestPromoteFlags_NewPromoteFlags(t *testing.T) {
	flags := promote.NewPromoteFlags()

	assert.NotNil(t, flags)
	assert.NotNil(t, flags.Project)
	assert.NotNil(t, flags.From)
	assert.NotNil(t, flags.To)
	assert.NotNil(t, flags.LatestSuccessful)

	assert.Equal(t, promote.FlagProject, flags.Project.Name)
	assert.Equal(t, promote.FlagFrom, flags.From.Name)
	assert.Equal(t, promote.FlagTo, flags.To.Name)
	assert.Equal(t, promote.FlagLatestSuccessful, flags.LatestSuccessful.Name)
}

func TestPromoteCommand_Documentation(t *testing.T) {
	const spaceID = "Spaces-1"
	space1 := fixtures.NewSpace(spaceID, "Default Space")

	api := testutil.NewMockHttpServer()
	defer api.Close()

	rootCmd := cmdRoot.NewCmdRoot(testutil.NewMockFactoryWithSpace(api, space1), nil, nil)

	cmd, _, err := rootCmd.Find([]string{"release", "promote"})
	assert.Nil(t, err)
	assert.NotNil(t, cmd)

	assert.Equal(t, "promote", cmd.Use)
	assert.Equal(t, "Promote a release", cmd.Short)
	assert.Equal(t, "Promote a release to a specific environment", cmd.Long)

	// Verify examples are present
	assert.Contains(t, cmd.Example, "release promote --project MyProject --from \"Development\" --to \"Staging\"")
	assert.Contains(t, cmd.Example, "release promote -p MyProject --from \"Staging\" --to \"Production\" --latest-successful")
}

func TestPromoteCommand_FlagAliases(t *testing.T) {
	const spaceID = "Spaces-1"
	space1 := fixtures.NewSpace(spaceID, "Default Space")

	api := testutil.NewMockHttpServer()
	defer api.Close()

	rootCmd := cmdRoot.NewCmdRoot(testutil.NewMockFactoryWithSpace(api, space1), nil, nil)

	cmd, _, err := rootCmd.Find([]string{"release", "promote"})
	assert.Nil(t, err)
	assert.NotNil(t, cmd)

	// Verify that the command has the expected flags
	assert.NotNil(t, cmd.Flags().Lookup("project"))
	assert.NotNil(t, cmd.Flags().Lookup("from"))
	assert.NotNil(t, cmd.Flags().Lookup("to"))
	assert.NotNil(t, cmd.Flags().Lookup("latest-successful"))

	// Verify legacy aliases exist
	assert.NotNil(t, cmd.Flags().Lookup("deployTo"))
	assert.NotNil(t, cmd.Flags().Lookup("latestSuccessful"))

	// Verify short flag for project
	projectFlag := cmd.Flags().Lookup("project")
	assert.Equal(t, "p", projectFlag.Shorthand)
}
