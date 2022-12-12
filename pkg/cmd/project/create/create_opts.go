package create

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	projectConvert "github.com/OctopusDeploy/cli/pkg/cmd/project/convert"
	projectGroupCreate "github.com/OctopusDeploy/cli/pkg/cmd/projectgroup/create"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
)

type CreateProjectGroupCallback func() (string, cmd.Dependable, error)
type ConvertProjectToConfigAsCodeCallback func() (cmd.Dependable, error)

type GetAllGroupsCallback func() ([]*projectgroups.ProjectGroup, error)

type CreateOptions struct {
	*CreateFlags
	*cmd.Dependencies
	*projectConvert.ConvertOptions
	GetAllGroupsCallback       GetAllGroupsCallback
	CreateProjectGroupCallback CreateProjectGroupCallback
	ConvertProjectCallback     ConvertProjectToConfigAsCodeCallback
}

func NewCreateOptions(createFlags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	convertOptions := projectConvert.NewConvertOptions(createFlags.ProjectConvertFlags, dependencies)
	return &CreateOptions{
		CreateFlags:  createFlags,
		Dependencies: dependencies,

		ConvertOptions:             convertOptions,
		GetAllGroupsCallback:       func() ([]*projectgroups.ProjectGroup, error) { return getAllGroups(*dependencies.Client) },
		CreateProjectGroupCallback: func() (string, cmd.Dependable, error) { return createProjectGroupCallback(dependencies) },
		ConvertProjectCallback:     func() (cmd.Dependable, error) { return convertProjectCallback(convertOptions) },
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

func convertProjectCallback(opts *projectConvert.ConvertOptions) (cmd.Dependable, error) {
	flags := opts.ConvertFlags
	deps := cmd.NewDependenciesFromExisting(opts.Dependencies, "octopus project convert")
	convertOpts := projectConvert.NewConvertOptions(flags, deps)
	return projectConvert.PromptForConfigAsCode(convertOpts)
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

	_, err = fmt.Fprintf(co.Out, "\nSuccessfully created project '%s' (%s), with lifecycle '%s' in project group '%s'.\n", createdProject.Name, createdProject.Slug, co.Lifecycle.Value, co.Group.Value)
	if err != nil {
		return err
	}

	link := output.Bluef("%s/app#/%s/projects/%s", co.Host, co.Space.GetID(), createdProject.GetID())
	fmt.Fprintf(co.Out, "View this project on Octopus Deploy: %s\n", link)

	return nil
}

func (co *CreateOptions) GenerateAutomationCmd() {
	if !co.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(co.CmdPath, co.Name, co.Description, co.Group, co.Lifecycle, co.ConfigAsCode)
		fmt.Fprintf(co.Out, "%s\n", autoCmd)
	}
}
