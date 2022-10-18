package connect

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/executionscommon"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/spf13/cobra"
)

const (
	FlagTenant           = "tenant"
	FlagProject          = "project"
	FlagEnvironment      = "environment"
	FlagAliasEnvironment = "env"
)

type ConnectFlags struct {
	Tenant       *flag.Flag[string]
	Project      *flag.Flag[string]
	Environments *flag.Flag[[]string]
}

func NewConnectFlags() *ConnectFlags {
	return &ConnectFlags{
		Tenant:       flag.New[string](FlagTenant, false),
		Project:      flag.New[string](FlagProject, false),
		Environments: flag.New[[]string](FlagEnvironment, false),
	}
}

type ConnectOptions struct {
	Client   *client.Client
	Ask      question.Asker
	NoPrompt bool
	*ConnectFlags
}

func NewCmdConnect(f factory.Factory) *cobra.Command {
	connectFlags := NewConnectFlags()
	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect a tenant to a project in Octopus Deploy",
		Long:  "Connect a tenant to a project in Octopus Deploy",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s tenant connect
			$ %s tenant connect --project "Deploy web site" --environment "Production"
`), constants.ExecutableName, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient()
			if err != nil {
				return err
			}

			opts := &ConnectOptions{
				Client:       client,
				Ask:          f.Ask,
				NoPrompt:     !f.IsPromptEnabled(),
				ConnectFlags: connectFlags,
			}

			return connectRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&connectFlags.Tenant.Value, connectFlags.Tenant.Name, "t", "", "Name or Id of the tenant")
	flags.StringVarP(&connectFlags.Project.Value, connectFlags.Project.Name, "p", "", "Name, ID or Slug of the project to connect to the tenant")
	flags.StringSliceVarP(&connectFlags.Environments.Value, connectFlags.Environments.Name, "e", nil, "The environments to connect to the tenant (can be specified multiple times)")
	return cmd
}

func connectRun(opts *ConnectOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	tenant, err := opts.Client.Tenants.GetByIdOrName(opts.Tenant.Value)
	if err != nil {
		return err
	}

	project, err := opts.Client.Projects.GetByIdOrName(opts.Project.Value)
	if err != nil {
		return err
	}
	if project.TenantedDeploymentMode == core.TenantedDeploymentModeUntenanted {
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

	return nil
}

func PromptMissing(opts *ConnectOptions) error {
	if opts.Tenant.Value == "" {

	}

	var selectedProject *projects.Project
	var err error
	if opts.Project.Value == "" {
		selectedProject, err = selectors.Project("Select the project to connect", opts.Client, opts.Ask)
		if err != nil {
			return nil
		}
		opts.Project.Value = selectedProject.GetName()
	}

	if opts.Environments.Value == nil || len(opts.Environments.Value) == 0 {
		var progression *projects.Progression
		progression, err = opts.Client.Projects.GetProgression(selectedProject)
		if len(progression.Environments) == 1 {
			opts.Environments.Value = append(opts.Environments.Value, progression.Environments[0].Name)
		} else {
			var selectedEnvironments []*resources.ReferenceDataItem
			selectedEnvironments, err = question.MultiSelectMap(opts.Ask, "Select the environments to connect", progression.Environments, func(item *resources.ReferenceDataItem) string { return item.Name }, true)
			for _, e := range selectedEnvironments {
				opts.Environments.Value = append(opts.Environments.Value, e.Name)
			}
		}
	}

	return nil
}
