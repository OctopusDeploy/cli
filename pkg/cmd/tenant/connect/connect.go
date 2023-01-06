package connect

import (
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/executionscommon"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/spf13/cobra"
)

const (
	FlagTenant                  = "tenant"
	FlagProject                 = "project"
	FlagEnvironment             = "environment"
	FlagEnableTenantDeployments = "enable-tenant-deployments"
	FlagAliasEnvironment        = "env"
)

type ConnectFlags struct {
	Tenant                  *flag.Flag[string]
	Project                 *flag.Flag[string]
	Environments            *flag.Flag[[]string]
	EnableTenantDeployments *flag.Flag[bool]
}

func NewConnectFlags() *ConnectFlags {
	return &ConnectFlags{
		Tenant:                  flag.New[string](FlagTenant, false),
		Project:                 flag.New[string](FlagProject, false),
		Environments:            flag.New[[]string](FlagEnvironment, false),
		EnableTenantDeployments: flag.New[bool](FlagEnableTenantDeployments, false),
	}
}

func NewConnectOptions(connectFlags *ConnectFlags, dependencies *cmd.Dependencies) *ConnectOptions {
	return &ConnectOptions{
		Dependencies:           dependencies,
		ConnectFlags:           connectFlags,
		GetAllTenantsCallback:  func() ([]*tenants.Tenant, error) { return shared.GetAllTenants(*dependencies.Client) },
		GetAllProjectsCallback: func() ([]*projects.Project, error) { return shared.GetAllProjects(*dependencies.Client) },
		GetProjectCallback: func(identifier string) (*projects.Project, error) {
			return shared.GetProject(*dependencies.Client, identifier)
		},
		GetProjectProgressionCallback: func(project *projects.Project) (*projects.Progression, error) {
			return getProjectProgression(*dependencies.Client, project)
		},
	}
}

func getProjectProgression(client client.Client, project *projects.Project) (*projects.Progression, error) {
	res, err := client.Projects.GetProgression(project)
	if err != nil {
		return nil, err
	}

	return res, nil
}

type ConnectOptions struct {
	*cmd.Dependencies
	*ConnectFlags
	GetAllTenantsCallback         shared.GetAllTenantsCallback
	GetAllProjectsCallback        shared.GetAllProjectsCallback
	GetProjectCallback            shared.GetProjectCallback
	GetProjectProgressionCallback shared.GetProjectProgression
}

func NewCmdConnect(f factory.Factory) *cobra.Command {
	connectFlags := NewConnectFlags()
	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect a tenant to a project",
		Long:  "Connect a tenant to a project in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s tenant connect
			$ %[1]s tenant connect --tenant "Bobs Wood Shop" --project "Deploy web site" --environment "Production"
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewConnectOptions(connectFlags, cmd.NewDependencies(f, c))

			return ConnectRun(opts)
		},
	}

	ConfigureFlags(cmd, connectFlags)
	return cmd
}

func ConfigureFlags(cmd *cobra.Command, connectFlags *ConnectFlags) {
	flags := cmd.Flags()
	flags.StringVarP(&connectFlags.Tenant.Value, connectFlags.Tenant.Name, "t", "", "Name or Id of the tenant")
	flags.StringVarP(&connectFlags.Project.Value, connectFlags.Project.Name, "p", "", "Name, ID or Slug of the project to connect to the tenant")
	flags.StringSliceVarP(&connectFlags.Environments.Value, connectFlags.Environments.Name, "e", nil, "The environments to connect to the tenant (can be specified multiple times)")
	flags.StringSliceVar(&connectFlags.Environments.Value, FlagAliasEnvironment, nil, "The environments to connect to the tenant (can be specified multiple times)")
	flags.BoolVar(&connectFlags.EnableTenantDeployments.Value, connectFlags.EnableTenantDeployments.Name, false, "Update the project to support tenanted deployments, if required")
}

func ConnectRun(opts *ConnectOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	tenant, err := opts.Client.Tenants.GetByIdentifier(opts.Tenant.Value)
	if err != nil {
		return err
	}

	project, err := opts.Client.Projects.GetByIdentifier(opts.Project.Value)
	if err != nil {
		return err
	}

	if !supportsTenantedDeployments(project) {
		if opts.EnableTenantDeployments.Value == false {
			return errors.New(getFailureMessageForUntenantedProject(project))
		}
		project.TenantedDeploymentMode = core.TenantedDeploymentModeTenantedOrUntenanted
		project, err = opts.Client.Projects.Update(project)
	}

	environments, err := executionscommon.FindEnvironments(opts.Client, opts.Environments.Value)
	if err != nil {
		return err
	}

	var environmentIds []string
	for _, e := range environments {
		environmentIds = append(environmentIds, e.GetID())
	}

	tenant.ProjectEnvironments[project.GetID()] = environmentIds
	tenant, err = opts.Client.Tenants.Update(tenant)
	if err != nil {
		return err
	}

	fmt.Fprintf(opts.Out, "Successfully connected '%s' to '%s'.\n", tenant.Name, project.GetName())
	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Tenant, opts.Project, opts.Environments, opts.EnableTenantDeployments)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}
	return nil
}

func PromptMissing(opts *ConnectOptions) error {
	if opts.Tenant.Value == "" {
		tenant, err := selectors.Select(opts.Ask, "You have not specified a Tenant. Please select one:", opts.GetAllTenantsCallback, func(tenant *tenants.Tenant) string {
			return tenant.Name
		})
		if err != nil {
			return err
		}

		opts.Tenant.Value = tenant.Name
	}

	if opts.Project.Value == "" {
		project, err := projectSelector("You have not specified a Project. Please select one:", opts.GetAllProjectsCallback, opts.Ask)
		if err != nil {
			return err
		}
		opts.Project.Value = project.GetName()
	}

	err := PromptForEnablingTenantedDeployments(opts, opts.GetProjectCallback)
	if err != nil {
		return err
	}

	if opts.Environments.Value == nil || len(opts.Environments.Value) == 0 {
		project, err := opts.GetProjectCallback(opts.Project.Value)
		if err != nil {
			return err
		}
		var progression *projects.Progression
		progression, err = opts.GetProjectProgressionCallback(project)
		if len(progression.Environments) == 1 {
			opts.Environments.Value = append(opts.Environments.Value, progression.Environments[0].Name)
		} else {
			var selectedEnvironments []*resources.ReferenceDataItem
			selectedEnvironments, err = question.MultiSelectMap(opts.Ask, "You have not specified any environments. Please select at least one:", progression.Environments, func(item *resources.ReferenceDataItem) string { return item.Name }, true)
			for _, e := range selectedEnvironments {
				opts.Environments.Value = append(opts.Environments.Value, e.Name)
			}
		}
	}

	return nil
}

func PromptForEnablingTenantedDeployments(opts *ConnectOptions, getProjectCallback shared.GetProjectCallback) error {
	if !opts.EnableTenantDeployments.Value {
		project, err := getProjectCallback(opts.Project.Value)
		if err != nil {
			return err
		}
		if !supportsTenantedDeployments(project) {
			opts.Ask(&survey.Confirm{
				Message: fmt.Sprintf("Do you want to enable tenanted deployments for '%s'?", project.GetName()),
				Default: false,
			}, &opts.EnableTenantDeployments.Value)

			if !opts.EnableTenantDeployments.Value {
				return errors.New(getFailureMessageForUntenantedProject(project))
			}
		}
	}

	return nil
}

func getFailureMessageForUntenantedProject(project *projects.Project) string {
	return fmt.Sprintf("Cannot connect tenant to '%s' as it does not support tenanted deployments.", project.GetName())
}

func projectSelector(questionText string, getAllProjectsCallback shared.GetAllProjectsCallback, ask question.Asker) (*projects.Project, error) {
	existingProjects, err := getAllProjectsCallback()
	if err != nil {
		return nil, err
	}

	return question.SelectMap(ask, questionText, existingProjects, getProjectDisplay())
}

func getProjectDisplay() func(p *projects.Project) string {
	return func(p *projects.Project) string {
		if supportsTenantedDeployments(p) {
			return p.GetName()

		}

		return output.Dim(fmt.Sprintf("%s (Tenanted deployments not currently supported)", p.Name))
	}
}

func supportsTenantedDeployments(project *projects.Project) bool {
	return project.TenantedDeploymentMode != core.TenantedDeploymentModeUntenanted
}
