package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/viper"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
)

// ListTools returns every tool the Octopus Server's MCP endpoint exposes
func ListTools(ctx context.Context, f factory.Factory, requester apiclient.Requester, log io.Writer) ([]*mcp.Tool, error) {
	session, err := Connect(ctx, f, requester, log)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	return listTools(ctx, session)
}

// FindTool returns the named tool, or an error if the server doesn't expose it
func FindTool(ctx context.Context, f factory.Factory, requester apiclient.Requester, log io.Writer, name string) (*mcp.Tool, error) {
	session, err := Connect(ctx, f, requester, log)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	return findTool(ctx, session, name)
}

// CallTool invokes a tool with the given arguments.
//
// Most tools operate within a space and take its ID as a parameter. If the caller leaves that out, we fill it in
// from the CLI's current space (--space, OCTOPUS_SPACE or the config file), prompting for one if needed and possible.
func CallTool(ctx context.Context, f factory.Factory, requester apiclient.Requester, log io.Writer, name string, arguments map[string]any) (*mcp.CallToolResult, error) {
	session, err := Connect(ctx, f, requester, log)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	tool, err := findTool(ctx, session, name)
	if err != nil {
		return nil, err
	}
	if err := fillSpace(f, requester, tool, arguments); err != nil {
		return nil, err
	}

	return session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
}

func listTools(ctx context.Context, session *mcp.ClientSession) ([]*mcp.Tool, error) {
	var tools []*mcp.Tool
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

func findTool(ctx context.Context, session *mcp.ClientSession, name string) (*mcp.Tool, error) {
	tools, err := listTools(ctx, session)
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

// spaceParameters are the parameters tools take the current space's ID in. Some tools take it twice, under both names.
var spaceParameters = []string{"SpaceId", "SpaceIdOrName"}

// fillSpace sets any space parameters the tool takes but the caller left out to the current space's ID.
// Required ones are resolved like any other space-scoped command, prompting if no space is set. Optional ones are only
// filled if the user has chosen a space, so we don't narrow a tool that would otherwise work across spaces.
func fillSpace(f factory.Factory, requester apiclient.Requester, tool *mcp.Tool, arguments map[string]any) error {
	parameters, err := Parameters(tool)
	if err != nil {
		return err
	}

	var missing []string
	anyRequired := false
	for _, parameter := range parameters {
		if _, supplied := arguments[parameter.Name]; supplied || !slices.Contains(spaceParameters, parameter.Name) {
			continue
		}
		missing = append(missing, parameter.Name)
		anyRequired = anyRequired || parameter.Required
	}
	if len(missing) == 0 || (!anyRequired && viper.GetString(constants.ConfigSpace) == "") {
		return nil
	}

	if _, err := f.GetSpacedClient(requester); err != nil {
		return fmt.Errorf("the MCP tool '%s' needs a space: %w", tool.Name, err)
	}
	space := f.GetCurrentSpace()
	if space == nil {
		return fmt.Errorf("the MCP tool '%s' needs a space, and the current space could not be determined", tool.Name)
	}
	for _, name := range missing {
		arguments[name] = space.GetID()
	}
	return nil
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
