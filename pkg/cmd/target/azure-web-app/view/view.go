package view

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
	"strings"
)

func NewCmdView(f factory.Factory) *cobra.Command {
	flags := shared.NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id>}",
		Short: "View an Azure Web App deployment target in an instance of Octopus Deploy",
		Long:  "View an Azure Web App deployment target in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s deployment-target azure-web-app view 'Shop Api'
			$ %s deployment-target azure-web-app view Machines-100
		`), constants.ExecutableName, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := shared.NewViewOptions(flags, cmd.NewDependencies(f, c), args)
			return viewRun(opts)
		},
	}

	shared.RegisterWebFlag(cmd, flags.WebFlags)

	return cmd
}

func viewRun(opts *shared.ViewOptions) error {
	var target, err = opts.Client.Machines.GetByIdentifier(opts.IdOrName)
	if err != nil {
		return err
	}
	err = shared.ViewRun(opts, target)
	if err != nil {
		return err
	}

	endpoint := target.Endpoint.(*machines.AzureWebAppEndpoint)
	account, err := opts.Client.Accounts.GetByID(endpoint.AccountID)
	if err != nil {
		return err
	}
	fmt.Fprintf(opts.Out, "Account: %s\n", account.GetName())
	fmt.Fprintf(opts.Out, "Web App: %s\n", getWebAppDisplay(endpoint))

	fmt.Fprintf(opts.Out, "\n")
	shared.DoWeb(target, opts.Dependencies, opts.WebFlags, "Azure Web App")
	return nil
}

func getWebAppDisplay(endpoint *machines.AzureWebAppEndpoint) string {
	builder := &strings.Builder{}
	builder.WriteString(endpoint.WebAppName)
	if endpoint.WebAppSlotName != "" {
		builder.WriteString(fmt.Sprintf("/%s", endpoint.WebAppSlotName))
	}

	return builder.String()
}
