package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/users"
	"github.com/spf13/cobra"
)

type UserAsJson struct {
	Id          string `json:"Id"`
	Name        string `json:"Name"`
	UserName    string `json:"UserName"`
	Description string `json:"Description"`
}

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
		Long:  "List users in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s user list
			$ %[1]s user ls
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(cmd, f)
		},
	}

	return cmd
}

func listRun(cmd *cobra.Command, f factory.Factory) error {
	client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	allUsers, err := client.Users.GetAll()
	if err != nil {
		return err
	}

	return output.PrintArray(allUsers, cmd, output.Mappers[*users.User]{
		Json: func(t *users.User) any {
			return UserAsJson{
				Id:       t.GetID(),
				Name:     t.DisplayName,
				UserName: t.Username,
				// TODO other fields like isService, etc
			}
		},
		Table: output.TableDefinition[*users.User]{
			// TODO other fields like isService, etc
			Header: []string{"USERNAME", "NAME", "ID"},
			Row: func(t *users.User) []string {
				return []string{output.Bold(t.Username), t.DisplayName, t.GetID()}
			},
		},
		Basic: func(t *users.User) string {
			return t.Username
		},
	})
}
