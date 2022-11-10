package tenant

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	cmdConnect "github.com/OctopusDeploy/cli/pkg/cmd/tenant/connect"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/tenant/create"
	cmdDelete "github.com/OctopusDeploy/cli/pkg/cmd/tenant/delete"
	cmdDisconnect "github.com/OctopusDeploy/cli/pkg/cmd/tenant/disconnect"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/tenant/list"
	cmdTag "github.com/OctopusDeploy/cli/pkg/cmd/tenant/tag"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/tenant/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdTenant(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenant <command>",
		Short: "Manage tenants",
		Long:  `Work with Octopus Deploy tenants.`,
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s tenant list
			$ %s tenant ls
		`), constants.ExecutableName, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(cmdConnect.NewCmdConnect(f))
	cmd.AddCommand(cmdDisconnect.NewCmdDisconnect(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdTag.NewCmdTag(f))
	cmd.AddCommand(cmdDelete.NewCmdDelete(f))
	cmd.AddCommand(cmdView.NewCmdView(f))

	return cmd
}
