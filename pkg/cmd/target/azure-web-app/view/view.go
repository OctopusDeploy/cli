package view

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/output"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
)

func NewCmdView(f factory.Factory) *cobra.Command {
	flags := shared.NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id>}",
		Short: "View an Azure Web App deployment target",
		Long:  "View an Azure Web App deployment target in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s deployment-target azure-web-app view 'Shop Api'
			$ %[1]s deployment-target azure-web-app view Machines-100
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := shared.NewViewOptions(flags, cmd.NewDependencies(f, c), args)
			return ViewRun(opts)
		},
	}

	machinescommon.RegisterWebFlag(cmd, flags.WebFlags)

	return cmd
}

func ViewRun(opts *shared.ViewOptions) error {
	return shared.ViewRun(opts, contributeEndpoint, "Azure Web App")
}

func contributeEndpoint(opts *shared.ViewOptions, targetEndpoint machines.IEndpoint) ([]*output.DataRow, error) {
	data := []*output.DataRow{}
	endpoint := targetEndpoint.(*machines.AzureWebAppEndpoint)
	accountRows, err := shared.ContributeAccount(opts, endpoint.AccountID)
	if err != nil {
		return nil, err
	}

	data = append(data, accountRows...)
	data = append(data, output.NewDataRow("Web App", getWebAppDisplay(endpoint)))
	return data, nil
}

func getWebAppDisplay(endpoint *machines.AzureWebAppEndpoint) string {
	builder := &strings.Builder{}
	builder.WriteString(endpoint.WebAppName)
	if endpoint.WebAppSlotName != "" {
		builder.WriteString(fmt.Sprintf("/%s", endpoint.WebAppSlotName))
	}

	return builder.String()
}
