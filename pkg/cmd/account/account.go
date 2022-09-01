package account

import (
	"fmt"

	cmdAWS "github.com/OctopusDeploy/cli/pkg/cmd/account/aws"
	cmdAzure "github.com/OctopusDeploy/cli/pkg/cmd/account/azure"
	cmdCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/create"
	cmdDelete "github.com/OctopusDeploy/cli/pkg/cmd/account/delete"
	cmdGCP "github.com/OctopusDeploy/cli/pkg/cmd/account/gcp"
	cmdList "github.com/OctopusDeploy/cli/pkg/cmd/account/list"
	cmdSSH "github.com/OctopusDeploy/cli/pkg/cmd/account/ssh"
	cmdToken "github.com/OctopusDeploy/cli/pkg/cmd/account/token"
	cmdUsr "github.com/OctopusDeploy/cli/pkg/cmd/account/username"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdAccount(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "account <command>",
		Short:   "Manage accounts",
		Long:    `Work with Octopus Deploy accounts.`,
		Example: fmt.Sprintf("$ %s account list", constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsInfrastructure: "true",
		},
	}

	cmd.AddCommand(cmdDelete.NewCmdDelete(f))
	cmd.AddCommand(cmdCreate.NewCmdCreate(f))
	cmd.AddCommand(cmdList.NewCmdList(f))
	cmd.AddCommand(cmdAWS.NewCmdAws(f))
	cmd.AddCommand(cmdAzure.NewCmdAzure(f))
	cmd.AddCommand(cmdGCP.NewCmdGcp(f))
	cmd.AddCommand(cmdSSH.NewCmdSsh(f))
	cmd.AddCommand(cmdUsr.NewCmdUsername(f))
	cmd.AddCommand(cmdToken.NewCmdToken(f))
	return cmd
}
