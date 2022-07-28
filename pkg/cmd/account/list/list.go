package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
	"github.com/spf13/cobra"
)

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List accounts in an instance of Octopus Deploy",
		Long:  "List accounts in an instance of Octopus Deploy.",
		Example: heredoc.Doc(`
			$ octopus account list"
		`),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient()
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
				Name string
				Type string
			}

			accountTypeMap := map[string]string{
				"AmazonWebServicesAccount": "AWS Account",
				"AzureServicePrincipal":    "Azure Subscription",
				"GoogleCloudAccount":       "Google Cloud Account",
				"SshKeyPair":               "SSH Key Pair",
				"UsernamePassword":         "Username/Password",
				"Token":                    "Token",
			}

			return output.PrintArray(items, cmd, output.Mappers[accounts.IAccount]{
				Json: func(item accounts.IAccount) any {
					return AccountJson{Id: item.GetID(), Name: item.GetName(), Type: string(item.GetAccountType())}
				},
				Table: output.TableDefinition[accounts.IAccount]{
					Header: []string{"NAME", "TYPE"},
					Row: func(item accounts.IAccount) []string {
						return []string{output.Bold(item.GetName()), accountTypeMap[string(item.GetAccountType())]}
					}},
				Basic: func(item accounts.IAccount) string {
					return item.GetName()
				},
			})
		},
	}

	return cmd
}
