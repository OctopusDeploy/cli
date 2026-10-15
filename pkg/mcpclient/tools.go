package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/factory"
)

// ListTools returns every tool the Octopus Server's MCP endpoint exposes
func ListTools(ctx context.Context, f factory.Factory, requester apiclient.Requester, log io.Writer) ([]*mcp.Tool, error) {
	session, err := Connect(ctx, f, requester, log)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	var tools []*mcp.Tool
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

// FindTool returns the named tool, or an error if the server doesn't expose it
func FindTool(ctx context.Context, f factory.Factory, requester apiclient.Requester, log io.Writer, name string) (*mcp.Tool, error) {
	tools, err := ListTools(ctx, f, requester, log)
	if err != nil {
		return nil, err
	}
	for _, tool := range tools {
		if tool.Name == name {
			return tool, nil
		}
	}
	return nil, fmt.Errorf("the Octopus Server has no MCP tool named '%s'; run 'octopus mcp tools' to list them", name)
}

// CallTool invokes a tool with the given arguments
func CallTool(ctx context.Context, f factory.Factory, requester apiclient.Requester, log io.Writer, name string, arguments map[string]any) (*mcp.CallToolResult, error) {
	session, err := Connect(ctx, f, requester, log)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	return session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
}

// FormatReadOnly describes a tool's read-only hint for display
func FormatReadOnly(tool *mcp.Tool) string {
	if tool.Annotations != nil && tool.Annotations.ReadOnlyHint {
		return "yes"
	}
	return "no"
}

type Parameter struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

// Parameters flattens the top level of a tool's input schema, required parameters first
func Parameters(tool *mcp.Tool) ([]Parameter, error) {
	// the client SDK leaves InputSchema as generic JSON; round-trip it into the shape we need
	raw, err := json.Marshal(tool.InputSchema)
	if err != nil {
		return nil, err
	}
	var schema struct {
		Properties map[string]struct {
			Type        any    `json:"type"`
			Description string `json:"description"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("cannot read the input schema for tool '%s': %w", tool.Name, err)
	}

	required := map[string]bool{}
	for _, name := range schema.Required {
		required[name] = true
	}

	parameters := make([]Parameter, 0, len(schema.Properties))
	for name, property := range schema.Properties {
		parameters = append(parameters, Parameter{
			Name:        name,
			Type:        formatType(property.Type),
			Required:    required[name],
			Description: property.Description,
		})
	}
	sort.Slice(parameters, func(i, j int) bool {
		if parameters[i].Required != parameters[j].Required {
			return parameters[i].Required
		}
		return parameters[i].Name < parameters[j].Name
	})
	return parameters, nil
}

// formatType renders a JSON schema "type", which may be a single type or a list such as ["string", "null"]
func formatType(t any) string {
	switch t := t.(type) {
	case string:
		return t
	case []any:
		s := ""
		for i, item := range t {
			if i > 0 {
				s += "|"
			}
			s += fmt.Sprint(item)
		}
		return s
	default:
		return ""
	}
}
