package view

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/libraryvariableset/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	FlagFilter = "filter"
	FlagWeb    = "web"
)

type ViewFlags struct {
	Filter *flag.Flag[string]
	Web    *flag.Flag[bool]
}

func NewViewFlags() *ViewFlags {
	return &ViewFlags{
		Filter: flag.New[string](FlagFilter, false),
		Web:    flag.New[bool](FlagWeb, false),
	}
}

type ViewOptions struct {
	Client        *client.Client
	Host          string
	Ask           question.Asker
	PromptEnabled bool
	idOrName      string
	flags         *ViewFlags
	Command       *cobra.Command
}

func NewCmdView(f factory.Factory) *cobra.Command {
	viewFlags := NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.MaximumNArgs(1),
		Use:   "view [<name> | <id>]",
		Short: "View a library variable set and its variables",
		Long: heredoc.Doc(`
			View a library variable set in Octopus Deploy, along with its variables.

			Values stored under the same variable name are grouped together, so a
			variable with several scoped values reads as one entry rather than several.
		`),
		Example: heredoc.Docf(`
			%[1]s library-variable-set view "Slack Variables"
			%[1]s library-variable-set view LibraryVariableSets-1
			%[1]s library-variable-set view "Slack Variables" --filter Url
			%[1]s library-variable-set view "Slack Variables" -f json
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}

			idOrName := ""
			if len(args) > 0 {
				idOrName = args[0]
			}

			opts := &ViewOptions{
				Client:        c,
				Host:          f.GetCurrentHost(),
				Ask:           f.Ask,
				PromptEnabled: f.IsPromptEnabled(),
				idOrName:      idOrName,
				flags:         viewFlags,
				Command:       cmd,
			}

			return viewRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&viewFlags.Filter.Value, viewFlags.Filter.Name, "q", "", "Show only variables with a name containing the given string")
	flags.BoolVarP(&viewFlags.Web.Value, viewFlags.Web.Name, "w", false, "Open in web browser")

	return cmd
}

type LibraryVariableSetAsJson struct {
	Id            string                  `json:"Id"`
	Name          string                  `json:"Name"`
	Description   string                  `json:"Description"`
	SpaceId       string                  `json:"SpaceId"`
	VariableSetId string                  `json:"VariableSetId"`
	TemplateCount int                     `json:"TemplateCount"`
	Variables     []*shared.VariableGroup `json:"Variables"`
	WebUrl        string                  `json:"WebUrl"`
}

func viewRun(opts *ViewOptions) error {
	set, err := shared.ResolveLibraryVariableSet(opts.Client, opts.Ask, opts.PromptEnabled,
		"Select the library variable set you wish to view:", opts.idOrName)
	if err != nil {
		return err
	}

	// the set itself and the variables it owns live on two different endpoints;
	// stitching them together here is the point of this command
	variableSet, err := opts.Client.Variables.GetAll(set.GetID())
	if err != nil {
		return err
	}

	groups := shared.GroupVariables(&variableSet)
	if filter := strings.ToLower(opts.flags.Filter.Value); filter != "" {
		groups = util.SliceFilter(groups, func(g *shared.VariableGroup) bool {
			return strings.Contains(strings.ToLower(g.Name), filter)
		})
		if groups == nil {
			// SliceFilter returns nil on no match; keep Variables as [] in json
			groups = []*shared.VariableGroup{}
		}
	}

	webUrl := util.GenerateWebURL(opts.Host, set.SpaceID, fmt.Sprintf("library/variablesets/%s", set.GetID()))

	outputFormat, _ := opts.Command.Flags().GetString(constants.FlagOutputFormat)
	if outputFormat == "" {
		outputFormat = viper.GetString(constants.ConfigOutputFormat)
	}

	// output.PrintResource/PrintArray both render a single shape; a library variable
	// set needs the set's own details plus a row per stored value, so the formats are
	// dispatched here (as pkg/cmd/config/list does).
	switch strings.ToLower(outputFormat) {
	case constants.OutputFormatJson:
		data, _ := json.MarshalIndent(LibraryVariableSetAsJson{
			Id:            set.GetID(),
			Name:          set.Name,
			Description:   set.Description,
			SpaceId:       set.SpaceID,
			VariableSetId: set.VariableSetID,
			TemplateCount: len(set.Templates),
			Variables:     groups,
			WebUrl:        webUrl,
		}, "", "  ")
		opts.Command.Println(string(data))
	case constants.OutputFormatBasic:
		opts.Command.Print(formatForBasic(set.Name, set.GetID(), set.Description, groups, webUrl))
	case constants.OutputFormatTable, "":
		printTable(opts, groups)
	default:
		return usage.NewUsageError(
			fmt.Sprintf("unsupported output format %s. Valid values are 'json', 'table', 'basic'. Defaults to table", outputFormat),
			opts.Command)
	}

	if opts.flags.Web.Value {
		_ = browser.OpenURL(webUrl)
	}

	return nil
}

// printTable shows one row per stored value, repeating the variable name only on the
// first row of each group so that a variable with many scopes reads as one block.
func printTable(opts *ViewOptions, groups []*shared.VariableGroup) {
	t := output.NewTable(opts.Command.OutOrStdout())
	t.AddRow(output.Bold("NAME"), output.Bold("VALUE"), output.Bold("SCOPE"), output.Bold("ID"))
	for _, group := range groups {
		for i, value := range group.Values {
			name := ""
			if i == 0 {
				name = output.Bold(group.Name)
			}
			t.AddRow(name, value.DisplayValue(), value.ScopeSummary, output.Dim(value.Id))
		}
	}
	t.Print()
}

func formatForBasic(name string, id string, description string, groups []*shared.VariableGroup, webUrl string) string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("%s %s\n", output.Bold(name), output.Dimf("(%s)", id)))
	if description == "" {
		result.WriteString(fmt.Sprintln(output.Dim(constants.NoDescription)))
	} else {
		result.WriteString(fmt.Sprintln(output.Dim(description)))
	}

	if len(groups) == 0 {
		result.WriteString("\nNo variables\n")
		return result.String()
	}

	for _, group := range groups {
		result.WriteString(fmt.Sprintf("\n%s\n", output.Bold(group.Name)))
		for _, value := range group.Values {
			result.WriteString(fmt.Sprintf("  %s = %s\n", value.ScopeSummary, value.DisplayValue()))
			if value.Prompt != nil {
				result.WriteString(fmt.Sprintf("    %s\n", output.Dim("prompted at deployment time")))
			}
		}
	}

	result.WriteString(fmt.Sprintf("\nView this library variable set in Octopus Deploy: %s\n", output.Blue(webUrl)))

	return result.String()
}
