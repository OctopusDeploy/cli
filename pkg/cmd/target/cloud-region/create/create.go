package create

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/executionscommon"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
)

const (
	FlagName = "name"
)

type CreateFlags struct {
	Name *flag.Flag[string]
	*shared.CreateTargetEnvironmentFlags
	*shared.CreateTargetRoleFlags
	*shared.WorkerPoolFlags
	*shared.CreateTargetTenantFlags
	*machinescommon.WebFlags
}

type CreateOptions struct {
	*CreateFlags
	*shared.CreateTargetEnvironmentOptions
	*shared.CreateTargetRoleOptions
	*shared.WorkerPoolOptions
	*shared.CreateTargetTenantOptions
	*cmd.Dependencies
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:                         flag.New[string](FlagName, false),
		WorkerPoolFlags:              shared.NewWorkerPoolFlags(),
		CreateTargetEnvironmentFlags: shared.NewCreateTargetEnvironmentFlags(),
		CreateTargetRoleFlags:        shared.NewCreateTargetRoleFlags(),
		CreateTargetTenantFlags:      shared.NewCreateTargetTenantFlags(),
		WebFlags:                     machinescommon.NewWebFlags(),
	}
}

func NewCreateOptions(createFlags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:                    createFlags,
		Dependencies:                   dependencies,
		WorkerPoolOptions:              shared.NewWorkerPoolOptionsForCreateTarget(dependencies),
		CreateTargetEnvironmentOptions: shared.NewCreateTargetEnvironmentOptions(dependencies),
		CreateTargetRoleOptions:        shared.NewCreateTargetRoleOptions(dependencies),
		CreateTargetTenantOptions:      shared.NewCreateTargetTenantOptions(dependencies),
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a cloud region deployment target",
		Long:  "Create a cloud region deployment target in Octopus Deploy",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s deployment-target cloud-region create
		`), constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "A short, memorable, unique name for this Cloud Region.")
	shared.RegisterCreateTargetEnvironmentFlags(cmd, createFlags.CreateTargetEnvironmentFlags)
	shared.RegisterCreateTargetRoleFlags(cmd, createFlags.CreateTargetRoleFlags)
	shared.RegisterCreateTargetWorkerPoolFlags(cmd, createFlags.WorkerPoolFlags)
	shared.RegisterCreateTargetTenantFlags(cmd, createFlags.CreateTargetTenantFlags)
	machinescommon.RegisterWebFlag(cmd, createFlags.WebFlags)

	return cmd
}

func createRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	envs, err := executionscommon.FindEnvironments(opts.Client, opts.Environments.Value)
	if err != nil {
		return err
	}
	environmentIds := util.SliceTransform(envs, func(e *environments.Environment) string { return e.ID })

	endpoint := machines.NewCloudRegionEndpoint()
	if opts.WorkerPool.Value != "" {
		workerPoolId, err := shared.FindWorkerPoolId(opts.GetAllWorkerPoolsCallback, opts.WorkerPool.Value)
		if err != nil {
			return err
		}
		endpoint.DefaultWorkerPoolID = workerPoolId
	}

	target := machines.NewDeploymentTarget(opts.Name.Value, endpoint, environmentIds, shared.DistinctRoles(opts.Roles.Value))
	err = shared.ConfigureTenant(target, opts.CreateTargetTenantFlags, opts.CreateTargetTenantOptions)
	if err != nil {
		return err
	}

	createdTarget, err := opts.Client.Machines.Add(target)
	if err != nil {
		return err
	}
	fmt.Fprintf(opts.Out, "Successfully created cloud region '%s'.\n", target.Name)
	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.WorkerPool, opts.Environments, opts.Roles, opts.TenantedDeploymentMode, opts.Tenants, opts.TenantTags)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	machinescommon.DoWebForTargets(createdTarget, opts.Dependencies, opts.WebFlags, "cloud region")

	return nil
}

func PromptMissing(opts *CreateOptions) error {
	err := question.AskName(opts.Ask, "", "Cloud Region", &opts.Name.Value)
	if err != nil {
		return err
	}

	err = shared.PromptForEnvironments(opts.CreateTargetEnvironmentOptions, opts.CreateTargetEnvironmentFlags)
	if err != nil {
		return err
	}

	err = shared.PromptForRoles(opts.CreateTargetRoleOptions, opts.CreateTargetRoleFlags)
	if err != nil {
		return err
	}

	err = shared.PromptForWorkerPool(opts.WorkerPoolOptions, opts.WorkerPoolFlags)
	if err != nil {
		return err
	}

	err = shared.PromptForTenant(opts.CreateTargetTenantOptions, opts.CreateTargetTenantFlags)
	if err != nil {
		return err
	}

	return nil
}
