package machinescommon

import (
	"fmt"

	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
)

const FlagDisabled = "disabled"

type CreateTargetDisabledFlags struct {
	Disabled *flag.Flag[bool]
}

func NewCreateTargetDisabledFlags() *CreateTargetDisabledFlags {
	return &CreateTargetDisabledFlags{
		Disabled: flag.New[bool](FlagDisabled, false),
	}
}

func RegisterCreateTargetDisabledFlags(cmd *cobra.Command, disabledFlags *CreateTargetDisabledFlags, description string) {
	cmd.Flags().BoolVar(&disabledFlags.Disabled.Value, disabledFlags.Disabled.Name, false, fmt.Sprintf("Create the %s in a disabled state.", description))
}
