package shared

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
)

const (
	FlagMachinePolicy = "machine-policy"
)

type GetAllMachinePoliciesCallback func() ([]*machines.MachinePolicy, error)

type CreateTargetMachinePolicyFlags struct {
	MachinePolicy *flag.Flag[string]
}

type CreateTargetMachinePolicyOptions struct {
	*cmd.Dependencies
	GetAllMachinePoliciesCallback
}

func NewCreateTargetMachinePolicyOptions(dependencies *cmd.Dependencies) *CreateTargetMachinePolicyOptions {
	return &CreateTargetMachinePolicyOptions{
		Dependencies: dependencies,
		GetAllMachinePoliciesCallback: func() ([]*machines.MachinePolicy, error) {
			return getAllMachinePolicies(*dependencies.Client)
		},
	}
}

func NewCreateTargetMachinePolicyFlags() *CreateTargetMachinePolicyFlags {
	return &CreateTargetMachinePolicyFlags{
		MachinePolicy: flag.New[string](FlagMachinePolicy, false),
	}
}

func RegisterCreateTargetMachinePolicyFlags(cmd *cobra.Command, machinePolicyFlags *CreateTargetMachinePolicyFlags) {
	cmd.Flags().StringVar(&machinePolicyFlags.MachinePolicy.Value, machinePolicyFlags.MachinePolicy.Name, "", "The machine policy for ")
}

func PromptForMachinePolicy(opts *CreateTargetMachinePolicyOptions, flags *CreateTargetMachinePolicyFlags) error {
	if flags.MachinePolicy.Value == "" {
		selectedOption, err := selectors.Select(opts.Ask, "Select the machine policy to use", opts.GetAllMachinePoliciesCallback, func(p *machines.MachinePolicy) string { return p.Name })
		if err != nil {
			return err
		}

		flags.MachinePolicy.Value = selectedOption.Name
	}

	return nil
}

func getAllMachinePolicies(client client.Client) ([]*machines.MachinePolicy, error) {
	res, err := client.MachinePolicies.GetAll()
	if err != nil {
		return nil, err
	}

	return res, nil
}
