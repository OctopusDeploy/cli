package list

import (
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/channel/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/lookups"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
)

const (
	FlagProject = "project"
	FlagFilter  = "filter"
)

type ListFlags struct {
	Project *flag.Flag[string]
	Filter  *flag.Flag[string]
}

func NewListFlags() *ListFlags {
	return &ListFlags{
		Project: flag.New[string](FlagProject, false),
		Filter:  flag.New[string](FlagFilter, false),
	}
}

type ChannelViewModel struct {
	ID            string
	Name          string
	Description   string
	LifecycleID   string
	LifecycleName string
	IsDefault     bool
	Type          string
}

func NewCmdList(f factory.Factory) *cobra.Command {
	listFlags := NewListFlags()
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List channels",
		Long:  "List channels for a project in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s channel list myProject
			%[1]s channel ls "Other Project"
			%[1]s channel list --project myProject
			%[1]s channel list --project myProject --filter "Hotfix"
			%[1]s channel ls -p myProject -q Hotfix
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && listFlags.Project.Value == "" {
				listFlags.Project.Value = args[0]
			}

			return listRun(cmd, f, listFlags)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&listFlags.Project.Value, listFlags.Project.Name, "p", "", "Name or ID of the project to list channels for")
	flags.StringVarP(&listFlags.Filter.Value, listFlags.Filter.Name, "q", "", "Filter channels to match only ones with a name containing the given string")
	return cmd
}

func listRun(cmd *cobra.Command, f factory.Factory, flags *ListFlags) error {
	outputFormat, err := cmd.Flags().GetString(constants.FlagOutputFormat)
	if err != nil { // should never happen, but fallback if it does
		outputFormat = constants.OutputFormatTable
	}

	projectNameOrID := flags.Project.Value

	octopus, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	selectedProject, err := selectors.ResolveProject(octopus, f.Ask, f.IsPromptEnabled(),
		"Select the project to list channels for", projectNameOrID)
	if err != nil {
		return err
	}
	if f.IsPromptEnabled() && projectNameOrID != "" && !constants.IsProgrammaticOutputFormat(outputFormat) {
		cmd.Printf("Project %s\n", output.Cyan(selectedProject.Name))
	}

	// Projects.GetChannels handles paging internally and returns the project-scoped list.
	// Server-side name filtering on the project-scoped endpoint isn't exposed by the
	// SDK helper, so we filter client-side (mirrors pkg/question/selectors/channels.go).
	allChannels, err := octopus.Projects.GetChannels(selectedProject)
	if err != nil {
		return err
	}

	// best-effort, as channel view is: listing still works without access to lifecycles
	lifecycleMap := lookups.GetLifecycleMap(octopus)

	filter := strings.ToLower(flags.Filter.Value)
	viewModels := make([]ChannelViewModel, 0, len(allChannels))
	for _, c := range allChannels {
		if filter != "" && !strings.Contains(strings.ToLower(c.Name), filter) {
			continue
		}
		viewModels = append(viewModels, ChannelViewModel{
			ID:            c.ID,
			Name:          c.Name,
			Description:   c.Description,
			LifecycleID:   c.LifecycleID,
			LifecycleName: lifecycleMap[c.LifecycleID],
			IsDefault:     c.IsDefault,
			Type:          string(c.Type),
		})
	}

	return output.PrintArray(viewModels, cmd, output.Mappers[ChannelViewModel]{
		Json: func(item ChannelViewModel) any {
			return item
		},
		Table: output.TableDefinition[ChannelViewModel]{
			Header: []string{"NAME", "TYPE", "DEFAULT", "LIFECYCLE"},
			Row: func(item ChannelViewModel) []string {
				def := ""
				if item.IsDefault {
					def = "*"
				}
				// a channel with no lifecycle of its own inherits the project's
				lifecycleDisplay := item.LifecycleName
				if item.LifecycleID == "" {
					lifecycleDisplay = shared.InheritedLifecycle
				} else if item.LifecycleName == "" { // the lifecycle couldn't be resolved
					lifecycleDisplay = item.LifecycleID
				}
				return []string{item.Name, item.Type, def, lifecycleDisplay}
			},
		},
		Basic: func(item ChannelViewModel) string {
			return item.Name
		},
	})
}
