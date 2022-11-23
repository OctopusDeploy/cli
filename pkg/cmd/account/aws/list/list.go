package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/spf13/cobra"
)

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List AWS accounts",
		Long:    "List AWS accounts in Octopus Deploy",
		Example: heredoc.Docf("$ %s account aws list", constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}
			return listAwsAccounts(client, cmd)
		},
	}

	return cmd
}

func listAwsAccounts(client *client.Client, cmd *cobra.Command) error {
	accountResources, err := client.Accounts.Get(accounts.AccountsQuery{
		AccountType: accounts.AccountTypeAmazonWebServicesAccount,
	})
	if err != nil {
		return err
	}
	items, err := accountResources.GetAllPages(client.Accounts.GetClient())
	if err != nil {
		return err
	}

	output.PrintArray(items, cmd, output.Mappers[accounts.IAccount]{
		Json: func(item accounts.IAccount) any {
			acc := item.(*accounts.AmazonWebServicesAccount)
			return &struct {
				Id        string
				Slug      string
				Name      string
				AccessKey string
			}{
				Id:        acc.GetID(),
				Slug:      acc.GetSlug(),
				Name:      acc.GetName(),
				AccessKey: acc.AccessKey,
			}
		},
		Table: output.TableDefinition[accounts.IAccount]{
			Header: []string{"NAME", "SLUG", "ACCESS KEY"},
			Row: func(item accounts.IAccount) []string {
				acc := item.(*accounts.AmazonWebServicesAccount)
				return []string{output.Bold(acc.GetName()), acc.GetSlug(), acc.AccessKey}
			}},
		Basic: func(item accounts.IAccount) string {
			return item.GetName()
		},
	})
	return nil
}
