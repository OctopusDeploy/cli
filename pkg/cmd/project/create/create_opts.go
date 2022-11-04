package create

import (
	"fmt"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/credentials"
	"net/url"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	projectGroupCreate "github.com/OctopusDeploy/cli/pkg/cmd/projectgroup/create"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
)

type CreateProjectGroupCallback func() (string, cmd.Dependable, error)

type GetAllGroupsCallback func() ([]*projectgroups.ProjectGroup, error)

type GetAllGitCredentialsCallback func() ([]*credentials.Resource, error)

type CreateOptions struct {
	*CreateFlags
	*cmd.Dependencies
	GetAllGroupsCallback         GetAllGroupsCallback
	CreateProjectGroupCallback   CreateProjectGroupCallback
	GetAllGitCredentialsCallback GetAllGitCredentialsCallback
}

func NewCreateOptions(createFlags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:                createFlags,
		Dependencies:               dependencies,
		GetAllGroupsCallback:       func() ([]*projectgroups.ProjectGroup, error) { return getAllGroups(*dependencies.Client) },
		CreateProjectGroupCallback: func() (string, cmd.Dependable, error) { return createProjectGroupCallback(dependencies) },
		GetAllGitCredentialsCallback: func() ([]*credentials.Resource, error) {
			return createGetAllGitCredentialsCallback(*dependencies.Client)
		},
	}
}

func getAllGroups(client client.Client) ([]*projectgroups.ProjectGroup, error) {
	res, err := client.ProjectGroups.GetAll()
	if err != nil {
		return nil, err
	}
	return res, nil
}

func createProjectGroupCallback(dependencies *cmd.Dependencies) (string, cmd.Dependable, error) {
	optValues := projectGroupCreate.NewCreateFlags()
	projectGroupOpts := cmd.NewDependenciesFromExisting(dependencies, "octopus project-group create")

	projectGroupCreateOpts := projectGroupCreate.NewCreateOptions(optValues, projectGroupOpts)
	projectGroupCreate.PromptMissing(projectGroupCreateOpts)
	returnValue := projectGroupCreateOpts.Name.Value
	return returnValue, projectGroupCreateOpts, nil
}

func createGetAllGitCredentialsCallback(client client.Client) ([]*credentials.Resource, error) {
	res, err := client.GitCredentials.Get(credentials.Query{})
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

func (co *CreateOptions) Commit() error {
	lifecycle, err := co.Client.Lifecycles.GetByIDOrName(co.Lifecycle.Value)
	if err != nil {
		return err
	}

	projectGroup, err := co.Client.ProjectGroups.GetByIDOrName(co.Group.Value)
	if err != nil {
		return err
	}

	project := projects.NewProject(co.Name.Value, lifecycle.GetID(), projectGroup.GetID())
	project.Description = co.Description.Value

	createdProject, err := co.Client.Projects.Add(project)
	if err != nil {
		return err
	}

	if co.ConfigAsCode.Value {
		gitPersistenceSettings, err := co.buildGitPersistenceSettings()
		if err != nil {
			return err
		}

		_, err = co.Client.Projects.ConvertToVcs(createdProject, getInitialCommitMessage(co), gitPersistenceSettings)
	}

	_, err = fmt.Fprintf(co.Out, "\nSuccessfully created project %s (%s), with lifecycle %s in project group %s.\n", createdProject.Name, createdProject.Slug, co.Lifecycle.Value, co.Group.Value)
	if err != nil {
		return err
	}

	link := output.Bluef("%s/app#/%s/projects/%s", co.Host, co.Space.GetID(), createdProject.GetID())
	fmt.Fprintf(co.Out, "View this project on Octopus Deploy: %s\n", link)

	return nil
}

func (co *CreateOptions) buildGitPersistenceSettings() (projects.GitPersistenceSettings, error) {
	var credentials credentials.GitCredential
	var err error
	if strings.EqualFold(co.GitStorage.Value, GitStorageLibrary) {
		credentials, err = co.buildLibraryGitVersionControlSettings()
		if err != nil {
			return nil, err
		}
	} else {
		credentials, err = co.buildProjectGitVersionControlSettings()
		if err != nil {
			return nil, err
		}
	}
	url, err := url.Parse(co.GitUrl.Value)
	if err != nil {
		return nil, err
	}

	vcs := projects.NewGitPersistenceSettings(getBasePath(co), credentials, getGitBranch(co), []string{}, url)
	return vcs, nil
}

func (co *CreateOptions) buildLibraryGitVersionControlSettings() (credentials.GitCredential, error) {
	creds, err := co.Client.GitCredentials.GetByIDOrName(co.GitCredentials.Value)
	if err != nil {
		return nil, err
	}

	credentials := credentials.NewReference(creds.GetID())
	return credentials, nil
}

func (co *CreateOptions) buildProjectGitVersionControlSettings() (credentials.GitCredential, error) {
	credentials := credentials.NewUsernamePassword(co.GitUsername.Value, core.NewSensitiveValue(co.GitPassword.Value))
	return credentials, nil
}

func getGitBranch(opts *CreateOptions) string {
	if opts.GitBranch.Value == "" {
		return "main"
	}

	return opts.GitBranch.Value
}

func getBasePath(opts *CreateOptions) string {
	if opts.GitBasePath.Value == "" {
		return DefaultBasePath
	}

	return opts.GitBasePath.Value
}

func getInitialCommitMessage(opts *CreateOptions) string {
	if opts.GitInitialCommitMessage.Value == "" {
		return DefaultGitCommitMessage
	}

	return opts.GitInitialCommitMessage.Value
}

func (co *CreateOptions) GenerateAutomationCmd() {
	if !co.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(co.CmdPath, co.Name, co.Description, co.Group, co.Lifecycle, co.ConfigAsCode, co.GitStorage, co.GitBasePath, co.GitUrl, co.GitBranch, co.GitInitialCommitMessage, co.GitCredentials, co.GitUsername, co.GitPassword)
		fmt.Fprintf(co.Out, "%s\n", autoCmd)
	}
}
