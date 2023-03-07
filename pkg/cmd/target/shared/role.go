package shared

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/spf13/cobra"
	"github.com/ztrue/tracerr"
)

const (
	FlagRole = "role"
)

type GetAllRolesCallback func() ([]string, error)

type CreateTargetRoleFlags struct {
	Roles *flag.Flag[[]string]
}

type CreateTargetRoleOptions struct {
	*cmd.Dependencies
	GetAllRolesCallback
}

func NewCreateTargetRoleOptions(dependencies *cmd.Dependencies) *CreateTargetRoleOptions {
	return &CreateTargetRoleOptions{
		Dependencies: dependencies,

		GetAllRolesCallback: func() ([]string, error) {
			return getAllMachineRoles(*dependencies.Client)
		},
	}
}

func NewCreateTargetRoleFlags() *CreateTargetRoleFlags {
	return &CreateTargetRoleFlags{
		Roles: flag.New[[]string](FlagRole, false),
	}
}

func RegisterCreateTargetRoleFlags(cmd *cobra.Command, commonFlags *CreateTargetRoleFlags) {
	cmd.Flags().StringSliceVar(&commonFlags.Roles.Value, FlagRole, []string{}, "Choose at least one role that this deployment target will provide.")
}

func PromptForRoles(opts *CreateTargetRoleOptions, flags *CreateTargetRoleFlags) error {

	if util.Empty(flags.Roles.Value) {
		availableRoles, err := opts.GetAllRolesCallback()
		if err != nil {
			return tracerr.Wrap(err)
		}
		roles, err := question.MultiSelectWithAddMap(opts.Ask, "Choose at least one role for the deployment target.\n", availableRoles, true)

		if err != nil {
			return tracerr.Wrap(err)
		}
		flags.Roles.Value = roles
	}
	return nil
}

func getAllMachineRoles(client client.Client) ([]string, error) {
	res, err := client.MachineRoles.GetAll()
	if err != nil {
		return nil, tracerr.Wrap(err)
	}

	var roles []string
	for _, r := range res {
		roles = append(roles, *r)
	}
	return roles, nil
}
