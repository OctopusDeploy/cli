package list

import (
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	sharedVariable "github.com/OctopusDeploy/cli/pkg/question/shared/variables"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
)

const (
	FlagFilter = "filter"
)

type ListFlags struct {
	Filter *flag.Flag[string]
}

func NewListFlags() *ListFlags {
	return &ListFlags{
		Filter: flag.New[string](FlagFilter, false),
	}
}

type LibraryVariableSetViewModel struct {
	ID            string
	Name          string
	Description   string
	VariableSetID string
}

func NewCmdList(f factory.Factory) *cobra.Command {
	listFlags := NewListFlags()
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List library variable sets",
		Long:  "List library variable sets in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s library-variable-set list
			%[1]s library-variable-set ls --filter Slack
			%[1]s library-variable-set ls -q Slack -f json
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(cmd, f, listFlags)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&listFlags.Filter.Value, listFlags.Filter.Name, "q", "", "Filter library variable sets to match only ones with a name containing the given string")
	return cmd
}

func listRun(cmd *cobra.Command, f factory.Factory, flags *ListFlags) error {
	octopus, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	// script modules share the libraryvariablesets endpoint; this command only deals
	// with variable sets, and GetAllLibraryVariableSets filters them out for us.
	allSets, err := sharedVariable.GetAllLibraryVariableSets(octopus)
	if err != nil {
		return err
	}

	filter := strings.ToLower(flags.Filter.Value)
	viewModels := make([]LibraryVariableSetViewModel, 0, len(allSets))
	for _, s := range allSets {
		if filter != "" && !strings.Contains(strings.ToLower(s.Name), filter) {
			continue
		}
		viewModels = append(viewModels, LibraryVariableSetViewModel{
			ID:            s.GetID(),
			Name:          s.Name,
			Description:   s.Description,
			VariableSetID: s.VariableSetID,
		})
	}

	sort.SliceStable(viewModels, func(i, j int) bool {
		return strings.ToLower(viewModels[i].Name) < strings.ToLower(viewModels[j].Name)
	})

	return output.PrintArray(viewModels, cmd, output.Mappers[LibraryVariableSetViewModel]{
		Json: func(item LibraryVariableSetViewModel) any {
			return item
		},
		Table: output.TableDefinition[LibraryVariableSetViewModel]{
			Header: []string{"NAME", "DESCRIPTION", "ID"},
			Row: func(item LibraryVariableSetViewModel) []string {
				return []string{output.Bold(item.Name), item.Description, output.Dim(item.ID)}
			},
		},
		Basic: func(item LibraryVariableSetViewModel) string {
			return item.Name
		},
	})
}
