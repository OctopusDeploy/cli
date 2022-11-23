package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/spf13/cobra"
)

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List accounts",
		Long:    "List accounts in Octopus Deploy",
		Example: heredoc.Docf("$ %s account list", constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}

			accountResoures, err := client.Accounts.Get()
			if err != nil {
				return err
			}

			items, err := accountResoures.GetAllPages(client.Accounts.GetClient())
			if err != nil {
				return err
			}

			type AccountJson struct {
				Id   string
				Slug string
				Name string
				Type string
			}

			accountTypeMap := map[accounts.AccountType]string{
				accounts.AccountTypeAmazonWebServicesAccount:   "AWS Account",
				accounts.AccountTypeAzureSubscription:          "Azure Subscription",
				accounts.AccountTypeAzureServicePrincipal:      "Azure Service Principal",
				accounts.AccountTypeGoogleCloudPlatformAccount: "Google Cloud Account",
				accounts.AccountTypeSSHKeyPair:                 "SSH Key Pair",
				accounts.AccountTypeUsernamePassword:           "Username/Password",
				accounts.AccountTypeToken:                      "Token",
			}

			return output.PrintArray(items, cmd, output.Mappers[accounts.IAccount]{
				Json: func(item accounts.IAccount) any {
					return AccountJson{Id: item.GetID(), Slug: item.GetSlug(), Name: item.GetName(), Type: string(item.GetAccountType())}
				},
				Table: output.TableDefinition[accounts.IAccount]{
					Header: []string{"NAME", "TYPE", "SLUG"},
					Row: func(item accounts.IAccount) []string {
						return []string{item.GetName(), accountTypeMap[item.GetAccountType()], item.GetSlug()}
					}},
				Basic: func(item accounts.IAccount) string {
					return item.GetName()
				},
			})
		},
	}

	return cmd
}
