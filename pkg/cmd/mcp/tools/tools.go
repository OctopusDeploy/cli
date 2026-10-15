package tools

import (
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/mcpclient"
	"github.com/OctopusDeploy/cli/pkg/output"
)

const maxDescriptionWidth = 100

func NewCmdTools(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "List MCP tools",
		Long:  "List the tools exposed by the Octopus Server's MCP endpoint",
		Example: heredoc.Docf(`
			%[1]s mcp tools
			%[1]s mcp tools -f json
		`, constants.ExecutableName),
		Aliases: []string{"list", "ls"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			tools, err := mcpclient.ListTools(cmd.Context(), f, apiclient.NewRequester(cmd), cmd.ErrOrStderr())
			if err != nil {
				return err
			}

			return output.PrintArray(tools, cmd, output.Mappers[*mcp.Tool]{
				Json: func(item *mcp.Tool) any {
					return item
				},
				Table: output.TableDefinition[*mcp.Tool]{
					Header: []string{"NAME", "READ-ONLY", "DESCRIPTION"},
					Row: func(item *mcp.Tool) []string {
						return []string{output.Bold(item.Name), mcpclient.FormatReadOnly(item), output.Truncate(maxDescriptionWidth, Summary(item.Description))}
					},
				},
				Basic: func(item *mcp.Tool) string {
					return item.Name
				},
			})
		},
	}

	return cmd
}

// Summary returns the first sentence of a tool description, on a single line
func Summary(description string) string {
	description = strings.Join(strings.Fields(description), " ")
	if i := strings.Index(description, ". "); i >= 0 {
		return description[:i+1]
	}
	return description
}
