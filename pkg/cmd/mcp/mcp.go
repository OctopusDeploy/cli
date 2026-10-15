package mcp

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	cmdCall "github.com/OctopusDeploy/cli/pkg/cmd/mcp/call"
	cmdTools "github.com/OctopusDeploy/cli/pkg/cmd/mcp/tools"
	cmdView "github.com/OctopusDeploy/cli/pkg/cmd/mcp/view"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
)

func NewCmdMcp(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp <command>",
		Short: "Use the Octopus Server's MCP tools",
		Long: heredoc.Doc(`
			List and call the tools exposed by the Octopus Server's remote MCP (Model Context Protocol) endpoint.

			The MCP endpoint only accepts AI agent API keys. Unless one is supplied via the OCTOPUS_MCP_API_KEY
			environment variable, the CLI creates an AI agent API key for the logged-in user on first use and
			saves it to the CLI config.
		`),
		Example: heredoc.Docf(`
			%[1]s mcp tools
			%[1]s mcp view find_projects
			%[1]s mcp call find_projects --params '{"SpaceId": "Spaces-1"}'
		`, constants.ExecutableName),
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	cmd.AddCommand(cmdTools.NewCmdTools(f))
	cmd.AddCommand(cmdView.NewCmdView(f))
	cmd.AddCommand(cmdCall.NewCmdCall(f))
	return cmd
}
