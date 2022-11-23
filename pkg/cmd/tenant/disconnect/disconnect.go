package disconnect

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/spf13/cobra"
)

const (
	FlagTenant  = "tenant"
	FlagProject = "project"
)

type DisconnectFlags struct {
	Tenant  *flag.Flag[string]
	Project *flag.Flag[string]
	*question.ConfirmFlags
}

type DisconnectOptions struct {
	*cmd.Dependencies
	*DisconnectFlags
	GetAllTenantsCallback  shared.GetAllTenantsCallback
	GetTenantCallback      shared.GetTenantCallback
	GetAllProjectsCallback shared.GetAllProjectsCallback
	GetProjectCallback     shared.GetProjectCallback
}

func NewDisconnectFlags() *DisconnectFlags {
	return &DisconnectFlags{
		Tenant:       flag.New[string](FlagTenant, false),
		Project:      flag.New[string](FlagProject, false),
		ConfirmFlags: question.NewConfirmFlags(),
	}
}

func NewDisconnectOptions(disconnectFlags *DisconnectFlags, dependencies *cmd.Dependencies) *DisconnectOptions {
	return &DisconnectOptions{
		Dependencies:           dependencies,
		DisconnectFlags:        disconnectFlags,
		GetAllTenantsCallback:  func() ([]*tenants.Tenant, error) { return shared.GetAllTenants(*dependencies.Client) },
		GetAllProjectsCallback: func() ([]*projects.Project, error) { return shared.GetAllProjects(*dependencies.Client) },
		GetProjectCallback: func(identifier string) (*projects.Project, error) {
			return shared.GetProject(*dependencies.Client, identifier)
		},
		GetTenantCallback: func(identifier string) (*tenants.Tenant, error) {
			return shared.GetTenant(*dependencies.Client, identifier)
		},
	}
}

func NewCmdDisconnect(f factory.Factory) *cobra.Command {
	disconnectFlags := NewDisconnectFlags()

	cmd := &cobra.Command{
		Use:   "disconnect",
		Short: "Disconnect a tenant from a project",
		Long:  "Disconnect a tenant from a project in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s tenant disconnect
			$ %[1]s tenant disconnect --tenant "Test Tenant" --project "Deploy web site" --confirm
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewDisconnectOptions(disconnectFlags, cmd.NewDependencies(f, c))
			return DisconnectRun(opts)
		},
	}

	ConfigureFlags(cmd, disconnectFlags)
	return cmd
}

func ConfigureFlags(c *cobra.Command, disconnectFlags *DisconnectFlags) {
	flags := c.Flags()
	flags.StringVarP(&disconnectFlags.Tenant.Value, disconnectFlags.Tenant.Name, "t", "", "Name or Id of the tenant")
	flags.StringVarP(&disconnectFlags.Project.Value, disconnectFlags.Project.Name, "p", "", "Name, ID or Slug of the project to connect to the tenant")
	flags.BoolVarP(&disconnectFlags.Confirm.Value, disconnectFlags.Confirm.Name, "y", false, "Don't ask for confirmation disconnecting the project.")
}

func DisconnectRun(opts *DisconnectOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	if !opts.Confirm.Value {
		return errors.New("Cannot disconnect without confirmation")
	}

	tenant, err := opts.GetTenantCallback(opts.Tenant.Value)
	if err != nil {
		return err
	}

	project, err := opts.GetProjectCallback(opts.Project.Value)
	if err != nil {
		return err
	}

	if len(tenant.ProjectEnvironments) == 0 {
		return errors.New("Tenant is not connected to any projects")
	}

	if _, ok := tenant.ProjectEnvironments[project.GetID()]; !ok {
		return errors.New(fmt.Sprintf("Tenant is not currently connected to the '%s' project", project.GetName()))
	}

	delete(tenant.ProjectEnvironments, project.GetID())
	tenant, err = opts.Client.Tenants.Update(tenant)
	if err != nil {
		return err
	}

	fmt.Fprintf(opts.Out, "Successfully disconnected '%s' from '%s'.\n", tenant.Name, project.GetName())
	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Tenant, opts.Project, opts.Confirm)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}
	return nil
}

func PromptMissing(opts *DisconnectOptions) error {
	var selectedTenant *tenants.Tenant
	var err error
	if opts.Tenant.Value == "" {
		tenant, err := selectors.Select(opts.Ask, "You have not specified a Tenant. Please select one:", opts.GetAllTenantsCallback, func(tenant *tenants.Tenant) string {
			return tenant.Name
		})
		if err != nil {
			return err
		}

		opts.Tenant.Value = tenant.Name
		selectedTenant = tenant
	} else {
		selectedTenant, err = opts.GetTenantCallback(opts.Tenant.Value)
		if err != nil {
			return err
		}
	}

	selectedProject, err := PromptForProject(opts, selectedTenant)
	if err != nil {
		return err
	}

	if !opts.Confirm.Value {
		opts.Ask(&survey.Confirm{
			Message: fmt.Sprintf("Are you sure you wish to disconnect '%s' from '%s'?", selectedTenant.Name, selectedProject.GetName()),
			Default: false,
		}, &opts.Confirm.Value)
	}

	return nil
}

func PromptForProject(opts *DisconnectOptions, selectedTenant *tenants.Tenant) (*projects.Project, error) {
	var selectedProject *projects.Project
	var err error
	if opts.Project.Value == "" {
		switch len(selectedTenant.ProjectEnvironments) {
		case 0:
			return nil, errors.New("Not currently connected to any projects")
		case 1:
			var projectId string
			for i := range selectedTenant.ProjectEnvironments {
				projectId = i
			}
			selectedProject, err = opts.GetProjectCallback(projectId)
			opts.Project.Value = selectedProject.GetName()
			if err != nil {
				return nil, err
			}
		default:
			currentlyConnectedProjects, err := getCurrentlyConnectedProjects(selectedTenant, opts.GetProjectCallback)
			if err != nil {
				return nil, err
			}
			project, err := projectSelector("You have not specified a Project. Please select one:", func() ([]*projects.Project, error) { return currentlyConnectedProjects, nil }, opts.Ask)
			if err != nil {
				return nil, nil
			}
			opts.Project.Value = project.GetName()
			selectedProject = project
		}

	} else {
		selectedProject, err = opts.GetProjectCallback(opts.Project.Value)
		if err != nil {
			return nil, err
		}
	}
	return selectedProject, nil
}

func getCurrentlyConnectedProjects(tenant *tenants.Tenant, getProjectCallback shared.GetProjectCallback) ([]*projects.Project, error) {
	var projects []*projects.Project
	for projectId := range tenant.ProjectEnvironments {
		project, err := getProjectCallback(projectId)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	sort.SliceStable(projects, func(i int, j int) bool {
		res := strings.Compare(projects[i].Name, projects[j].Name)
		return res == -1
	})
	return projects, nil
}

func projectSelector(questionText string, getAllProjectsCallback shared.GetAllProjectsCallback, ask question.Asker) (*projects.Project, error) {
	existingProjects, err := getAllProjectsCallback()
	if err != nil {
		return nil, err
	}

	return question.SelectMap(ask, questionText, existingProjects, func(p *projects.Project) string { return p.GetName() })
}
