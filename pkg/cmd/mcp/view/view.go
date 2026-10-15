package view

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/mcpclient"
	"github.com/OctopusDeploy/cli/pkg/output"
)

func NewCmdView(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <tool>",
		Short: "View an MCP tool",
		Long:  "View the description and parameters of a tool exposed by the Octopus Server's MCP endpoint",
		Example: heredoc.Docf(`
			%[1]s mcp view find_projects
			%[1]s mcp view find_projects -f json
		`, constants.ExecutableName),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tool, err := mcpclient.FindTool(cmd.Context(), f, apiclient.NewRequester(cmd), cmd.ErrOrStderr(), args[0])
			if err != nil {
				return err
			}

			outputFormat, _ := cmd.Flags().GetString(constants.FlagOutputFormat)
			if strings.EqualFold(outputFormat, constants.OutputFormatJson) {
				encoded, err := json.MarshalIndent(tool, "", "  ")
				if err != nil {
					return err
				}
				cmd.Println(string(encoded))
				return nil
			}

			parameters, err := mcpclient.Parameters(tool)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s %s\n\n", output.Bold(tool.Name), output.Dimf("(read-only: %s)", mcpclient.FormatReadOnly(tool)))
			fmt.Fprintf(out, "%s\n\n", strings.TrimSpace(tool.Description))

			if len(parameters) == 0 {
				fmt.Fprintln(out, "This tool takes no parameters.")
				return nil
			}

			t := output.NewTable(out)
			t.AddRow(output.Bold("PARAMETER"), output.Bold("TYPE"), output.Bold("REQUIRED"), output.Bold("DESCRIPTION"))
			for _, p := range parameters {
				required := ""
				if p.Required {
					required = "yes"
				}
				t.AddRow(p.Name, p.Type, required, strings.Join(strings.Fields(p.Description), " "))
			}
			return t.Print()
		},
	}

	return cmd
}
