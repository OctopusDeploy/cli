package list

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/spf13/cobra"
	"strings"
)

func NewCmdList(client apiclient.ClientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List environments in an instance of Octopus Deploy",
		Long:  "List environments in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s environment list"
		`), constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := client.Get(false)
			if err != nil {
				return err
			}

			environments, err := client.Environments.GetAll()
			if err != nil {
				return err
			}

			outputFormat, _ := cmd.Flags().GetString("outputFormat")
			switch strings.ToLower(outputFormat) {
			// TODO make a proper thing that encodes JSON for us so we don't have to write this boilerplate a zillion times
			case "json":
				type IdName struct {
					Id   string `json:"Id"`
					Name string `json:"Name"`
				}
				outputJson := []IdName{}
				for _, e := range environments {
					outputJson = append(outputJson, IdName{Id: e.GetID(), Name: e.Name})
				}

				data, _ := json.MarshalIndent(outputJson, "", "  ")
				fmt.Println(string(data))
			case "":
				for _, e := range environments {
					fmt.Printf("%s\t%s\t%s\n", e.GetID(), e.Name, e.Description)
				}
			default:
				return errors.New(fmt.Sprintf("Unsupported outputFormat %s. Valid values are 'json' or an empty string", outputFormat))
			}

			return nil
		},
	}

	return cmd
}
