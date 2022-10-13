package create

import (
	"fmt"
	"net/url"

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

type CreateOptions struct {
	*CreateFlags
	*cmd.Dependencies
	GetAllGroupsCallback       GetAllGroupsCallback
	CreateProjectGroupCallback CreateProjectGroupCallback
}

func NewCreateOptions(createFlags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:                createFlags,
		Dependencies:               dependencies,
		GetAllGroupsCallback:       func() ([]*projectgroups.ProjectGroup, error) { return getAllGroups(*dependencies.Client) },
		CreateProjectGroupCallback: func() (string, cmd.Dependable, error) { return createProjectGroupCallback(dependencies) },
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

func (co *CreateOptions) Commit() error {
	lifecycle, err := co.Client.Lifecycles.GetByIDOrName(co.Lifecycle.Value)
	if err != nil {
		return err
	}

	projectGroup, err := co.Client.ProjectGroups.GetByIDOrName(co.Group.Value)
	if err != nil {
		return err
	}

	project := projects.NewProject(co.Name.Value, lifecycle.ID, projectGroup.ID)
	project.Description = co.Description.Value

	createdProject, err := co.Client.Projects.Add(project)
	if err != nil {
		return err
	}

	if co.ConfigAsCode.Value {
		var vcs *projects.VersionControlSettings
		if co.GitStorage.Value == "project" {
			vcs, err = co.buildProjectGitVersionControlSettings()
			if err != nil {
				return err
			}

		} else {
			vcs, err = co.buildLibraryGitVersionControlSettings()
			if err != nil {
				return err
			}
		}

		_, err = co.Client.Projects.ConvertToVcs(createdProject, getInitialCommitMessage(co), vcs)
	}

	_, err = fmt.Fprintf(co.Out, "\nSuccessfully created project %s (%s), with lifecycle %s in project group %s.\n", createdProject.Name, createdProject.Slug, co.Lifecycle.Value, co.Group.Value)
	if err != nil {
		return err
	}

	link := output.Bluef("%s/app#/%s/projects/%s", co.Host, co.Space.GetID(), createdProject.GetID())
	fmt.Fprintf(co.Out, "View this project on Octopus Deploy: %s\n", link)

	return nil
}

func (co *CreateOptions) buildLibraryGitVersionControlSettings() (*projects.VersionControlSettings, error) {
	panic("library git credentials not currently supported")
}

func (co *CreateOptions) buildProjectGitVersionControlSettings() (*projects.VersionControlSettings, error) {
	credentials := projects.NewUsernamePasswordGitCredential(co.GitUsername.Value, core.NewSensitiveValue(co.GitPassword.Value))
	url, err := url.Parse(co.GitUrl.Value)
	if err != nil {
		return nil, err
	}
	vcs := projects.NewVersionControlSettings(getBasePath(co), credentials, getGitBranch(co), GitPersistenceType, url)
	return vcs, nil
}

func (co *CreateOptions) GenerateAutomationCmd() {
	if !co.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(co.CmdPath, co.Name, co.Description, co.Group, co.Lifecycle)
		fmt.Fprintf(co.Out, "%s\n", autoCmd)
	}
}
