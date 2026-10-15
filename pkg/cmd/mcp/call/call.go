package call

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/mcpclient"
)

const FlagParams = "params"

func NewCmdCall(f factory.Factory) *cobra.Command {
	var params string

	cmd := &cobra.Command{
		Use:   "call <tool>",
		Short: "Call an MCP tool",
		Long:  "Call a tool exposed by the Octopus Server's MCP endpoint and print its result",
		Example: heredoc.Docf(`
			%[1]s mcp call get_current_user
			%[1]s mcp call find_projects --space Default --params '{"PartialName": "web"}'
			%[1]s mcp call find_projects --params '{"SpaceId": "Spaces-2"}' -f json
			%[1]s mcp call create_release --space Default --params @release.json
			echo '{"PartialName": "web"}' | %[1]s mcp call find_projects --space Default --params -
		`, constants.ExecutableName),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			arguments, err := readParams(params, cmd.InOrStdin())
			if err != nil {
				return err
			}

			result, err := mcpclient.CallTool(cmd.Context(), f, apiclient.NewRequester(cmd), cmd.ErrOrStderr(), args[0], arguments)
			if err != nil {
				return err
			}

			if result.IsError {
				printContent(cmd.ErrOrStderr(), result.Content)
				return fmt.Errorf("the MCP tool '%s' returned an error", args[0])
			}

			outputFormat, _ := cmd.Flags().GetString(constants.FlagOutputFormat)
			if strings.EqualFold(outputFormat, constants.OutputFormatJson) {
				return printJson(cmd.OutOrStdout(), result)
			}
			printContent(cmd.OutOrStdout(), result.Content)
			return nil
		},
	}

	cmd.Flags().StringVarP(&params, FlagParams, "p", "", "The tool's arguments, as a JSON object. Use @path to read them from a file, or - to read them from stdin")
	return cmd
}

// readParams parses the tool's arguments from the --params value: inline JSON, @path to read a file, or - for stdin.
// Reading from a file or stdin saves callers from quoting JSON for their shell.
func readParams(value string, stdin io.Reader) (map[string]any, error) {
	source := "--" + FlagParams
	var raw []byte
	switch {
	case value == "-":
		b, err := io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("cannot read %s from stdin: %w", source, err)
		}
		raw, source = b, "the JSON on stdin"
	case strings.HasPrefix(value, "@"):
		path := strings.TrimPrefix(value, "@")
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("cannot read %s: %w", source, err)
		}
		raw, source = b, fmt.Sprintf("the JSON in '%s'", path)
	default:
		raw = []byte(value)
	}

	arguments := map[string]any{}
	if strings.TrimSpace(string(raw)) == "" {
		return arguments, nil
	}
	if err := json.Unmarshal(raw, &arguments); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object: %w", source, err)
	}
	// "null" unmarshals without error but leaves us with no map
	if arguments == nil {
		return nil, fmt.Errorf("%s must be a JSON object, not null", source)
	}
	return arguments, nil
}

// printJson prints the tool's structured result if it has one, otherwise its content blocks
func printJson(out io.Writer, result *mcp.CallToolResult) error {
	var value any = result.Content
	if result.StructuredContent != nil {
		value = result.StructuredContent
	} else if len(result.Content) == 1 {
		// most tools return a single text block holding JSON; emit that JSON directly rather than wrapping it
		if text, ok := result.Content[0].(*mcp.TextContent); ok && json.Valid([]byte(text.Text)) {
			value = json.RawMessage(text.Text)
		}
	}

	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(encoded))
	return err
}

func printContent(out io.Writer, content []mcp.Content) {
	for _, c := range content {
		switch c := c.(type) {
		case *mcp.TextContent:
			var pretty bytes.Buffer
			if json.Indent(&pretty, []byte(c.Text), "", "  ") == nil {
				fmt.Fprintln(out, pretty.String())
			} else {
				fmt.Fprintln(out, c.Text)
			}
		default:
			fmt.Fprintf(out, "[%T content omitted]\n", c)
		}
	}
}
