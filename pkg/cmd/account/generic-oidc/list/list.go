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
		Short:   "List Generic OpenID Connect accounts",
		Long:    "List Generic OpenID Connect accounts in Octopus Deploy",
		Example: heredoc.Docf("$ %s account generic-oidc list", constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}
			return listGenericOidcAccounts(client, cmd)
		},
	}

	return cmd
}

func listGenericOidcAccounts(client *client.Client, cmd *cobra.Command) error {
	accountResources, err := client.Accounts.Get(accounts.AccountsQuery{
		AccountType: accounts.AccountTypeGenericOIDCAccount,
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
			acc := item.(*accounts.GenericOIDCAccount)
			return &struct {
				Id                   string
				Name                 string
				Slug                 string
				AccountType          string
				ExecutionSubjectKeys []string
				Audience             string
			}{
				Id:                   acc.GetID(),
				Name:                 acc.GetName(),
				Slug:                 acc.GetSlug(),
				AccountType:          string(acc.AccountType),
				ExecutionSubjectKeys: acc.DeploymentSubjectKeys,
				Audience:             acc.Audience,
			}
		},
		Table: output.TableDefinition[accounts.IAccount]{
			Header: []string{"NAME", "SLUG", "AUDIENCE"},
			Row: func(item accounts.IAccount) []string {
				acc := item.(*accounts.GenericOIDCAccount)
				return []string{
					output.Bold(acc.GetName()),
					acc.GetSlug(),
					acc.Audience}
			}},
		Basic: func(item accounts.IAccount) string {
			return item.GetName()
		},
	})
	return nil
}
