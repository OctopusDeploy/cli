package create

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/worker/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
)

const (
	FlagName = "name"
)

type CreateFlags struct {
	Name *flag.Flag[string]
	*machinescommon.CreateTargetProxyFlags
	*machinescommon.CreateTargetMachinePolicyFlags
	*shared.WorkerPoolFlags
	*machinescommon.WebFlags
	*machinescommon.SshCommonFlags
}

type CreateOptions struct {
	*CreateFlags
	*machinescommon.SshCommonOptions
	*machinescommon.CreateTargetProxyOptions
	*machinescommon.CreateTargetMachinePolicyOptions
	*shared.WorkerPoolOptions
	*cmd.Dependencies
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:                           flag.New[string](FlagName, false),
		SshCommonFlags:                 machinescommon.NewSshCommonFlags(),
		CreateTargetProxyFlags:         machinescommon.NewCreateTargetProxyFlags(),
		CreateTargetMachinePolicyFlags: machinescommon.NewCreateTargetMachinePolicyFlags(),
		WorkerPoolFlags:                shared.NewWorkerPoolFlags(),
		WebFlags:                       machinescommon.NewWebFlags(),
	}
}

func NewCreateOptions(createFlags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:                      createFlags,
		Dependencies:                     dependencies,
		CreateTargetProxyOptions:         machinescommon.NewCreateTargetProxyOptions(dependencies),
		CreateTargetMachinePolicyOptions: machinescommon.NewCreateTargetMachinePolicyOptions(dependencies),
		WorkerPoolOptions:                shared.NewWorkerPoolOptions(dependencies),
		SshCommonOptions:                 machinescommon.NewSshCommonOpts(dependencies),
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a SSH worker",
		Long:    "Create a SSH worker in Octopus Deploy",
		Example: heredoc.Docf("$ %s worker ssh create", constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "A short, memorable, unique name for this worker.")
	machinescommon.RegisterSshCommonFlags(cmd, createFlags.SshCommonFlags, "worker")
	machinescommon.RegisterCreateTargetProxyFlags(cmd, createFlags.CreateTargetProxyFlags)
	machinescommon.RegisterCreateTargetMachinePolicyFlags(cmd, createFlags.CreateTargetMachinePolicyFlags)
	shared.RegisterCreateWorkerWorkerPoolFlags(cmd, createFlags.WorkerPoolFlags)
	machinescommon.RegisterWebFlag(cmd, createFlags.WebFlags)

	return cmd
}

func createRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	account, err := machinescommon.GetSshAccount(opts.SshCommonOptions, opts.SshCommonFlags)
	if err != nil {
		return err
	}

	port := opts.Port.Value
	if port == 0 {
		port = machinescommon.DefaultPort
	}
	endpoint := machines.NewSSHEndpoint(opts.HostName.Value, port, opts.Fingerprint.Value)
	endpoint.AccountID = account.GetID()

	if opts.Runtime.Value == machinescommon.SelfContainedCalamari {
		endpoint.DotNetCorePlatform = opts.Platform.Value
	}

	if opts.Proxy.Value != "" {
		proxy, err := machinescommon.FindProxy(opts.CreateTargetProxyOptions, opts.CreateTargetProxyFlags)
		if err != nil {
			return err
		}
		endpoint.ProxyID = proxy.GetID()
	}

	workerPoolIds, err := shared.FindWorkerPoolIds(opts.WorkerPoolOptions, opts.WorkerPoolFlags)
	if err != nil {
		return err
	}

	worker := machines.NewWorker(opts.Name.Value, endpoint)
	worker.WorkerPoolIDs = workerPoolIds
	machinePolicy, err := machinescommon.FindMachinePolicy(opts.GetAllMachinePoliciesCallback, opts.MachinePolicy.Value)
	if err != nil {
		return err
	}
	worker.MachinePolicyID = machinePolicy.GetID()

	createdWorker, err := opts.Client.Workers.Add(worker)
	if err != nil {
		return err
	}

	fmt.Fprintf(opts.Out, "Successfully created SSH worker '%s'.\n", createdWorker.Name)
	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.HostName, opts.Port, opts.Fingerprint, opts.Runtime, opts.Platform, opts.WorkerPools, opts.Account, opts.Proxy, opts.MachinePolicy)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	machinescommon.DoWebForWorkers(createdWorker, opts.Dependencies, opts.WebFlags, "ssh")

	return nil
}

func PromptMissing(opts *CreateOptions) error {
	err := question.AskName(opts.Ask, "", "SSH", &opts.Name.Value)
	if err != nil {
		return err
	}

	err = shared.PromptForWorkerPools(opts.WorkerPoolOptions, opts.WorkerPoolFlags)
	if err != nil {
		return err
	}

	err = machinescommon.PromptForMachinePolicy(opts.CreateTargetMachinePolicyOptions, opts.CreateTargetMachinePolicyFlags)
	if err != nil {
		return err
	}

	err = machinescommon.PromptForSshAccount(opts.SshCommonOptions, opts.SshCommonFlags)
	if err != nil {
		return err
	}

	err = machinescommon.PromptForSshEndpoint(opts.SshCommonOptions, opts.SshCommonFlags, "worker")
	if err != nil {
		return err
	}

	err = machinescommon.PromptForProxy(opts.CreateTargetProxyOptions, opts.CreateTargetProxyFlags)
	if err != nil {
		return err
	}

	err = machinescommon.PromptForDotNetConfig(opts.SshCommonOptions, opts.SshCommonFlags, "worker")
	if err != nil {
		return err
	}

	return nil
}
