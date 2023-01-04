package shared

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/spf13/cobra"
)

const (
	FlagEnvironment = "environment"
)

type CreateTargetEnvironmentFlags struct {
	Environments *flag.Flag[[]string]
}

type CreateTargetEnvironmentOptions struct {
	*cmd.Dependencies
	selectors.GetAllEnvironmentsCallback
}

func NewCreateTargetEnvironmentOptions(dependencies *cmd.Dependencies) *CreateTargetEnvironmentOptions {
	return &CreateTargetEnvironmentOptions{
		Dependencies: dependencies,
		GetAllEnvironmentsCallback: func() ([]*environments.Environment, error) {
			return selectors.GetAllEnvironments(dependencies.Client)
		},
	}
}

func NewCreateTargetEnvironmentFlags() *CreateTargetEnvironmentFlags {
	return &CreateTargetEnvironmentFlags{
		Environments: flag.New[[]string](FlagEnvironment, false),
	}
}

func RegisterCreateTargetEnvironmentFlags(cmd *cobra.Command, flags *CreateTargetEnvironmentFlags) {
	cmd.Flags().StringSliceVar(&flags.Environments.Value, FlagEnvironment, []string{}, "Choose at least one environment for the deployment target.")
}

func PromptForEnvironments(opts *CreateTargetEnvironmentOptions, flags *CreateTargetEnvironmentFlags) error {
	if util.Empty(flags.Environments.Value) {
		envs, err := selectors.EnvironmentsMultiSelect(opts.Ask, opts.GetAllEnvironmentsCallback,
			"Choose at least one environment for the deployment target.\n", true)
		if err != nil {
			return err
		}
		flags.Environments.Value = util.SliceTransform(envs, func(e *environments.Environment) string { return e.Name })
	}

	return nil
}
