package list

import (
	"errors"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"
)

const (
	FlagProject     = "project"
	FlagPartialName = "partial-name"
)

type ListFlags struct {
	Project     *flag.Flag[string]
	PartialName *flag.Flag[string]
}

func NewListFlags() *ListFlags {
	return &ListFlags{
		Project:     flag.New[string](FlagProject, false),
		PartialName: flag.New[string](FlagPartialName, false),
	}
}

type ChannelViewModel struct {
	ID          string
	Name        string
	Description string
	LifecycleID string
	IsDefault   bool
	Type        string
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
			%[1]s channel list --project myProject --partial-name "Hotfix"
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
	flags.StringVar(&listFlags.PartialName.Value, listFlags.PartialName.Name, "", "Filter channels by partial name match (case-insensitive)")
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

	var selectedProject *projects.Project
	if f.IsPromptEnabled() {
		if projectNameOrID == "" {
			selectedProject, err = selectors.Project("Select the project to list channels for", octopus, f.Ask)
			if err != nil {
				return err
			}
		} else {
			selectedProject, err = selectors.FindProject(octopus, projectNameOrID)
			if err != nil {
				return err
			}
			if !constants.IsProgrammaticOutputFormat(outputFormat) {
				cmd.Printf("Project %s\n", output.Cyan(selectedProject.Name))
			}
		}
	} else {
		if projectNameOrID == "" {
			return errors.New("project must be specified")
		}
		selectedProject, err = selectors.FindProject(octopus, projectNameOrID)
		if err != nil {
			return err
		}
	}

	// Projects.GetChannels handles paging internally and returns the project-scoped list.
	// Server-side partialName filtering on the project-scoped endpoint isn't exposed by the
	// SDK helper, so we filter client-side (mirrors pkg/question/selectors/channels.go).
	allChannels, err := octopus.Projects.GetChannels(selectedProject)
	if err != nil {
		return err
	}

	partial := strings.ToLower(flags.PartialName.Value)
	viewModels := make([]ChannelViewModel, 0, len(allChannels))
	for _, c := range allChannels {
		if partial != "" && !strings.Contains(strings.ToLower(c.Name), partial) {
			continue
		}
		viewModels = append(viewModels, ChannelViewModel{
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			LifecycleID: c.LifecycleID,
			IsDefault:   c.IsDefault,
			Type:        string(c.Type),
		})
	}

	return output.PrintArray(viewModels, cmd, output.Mappers[ChannelViewModel]{
		Json: func(item ChannelViewModel) any {
			return item
		},
		Table: output.TableDefinition[ChannelViewModel]{
			Header: []string{"NAME", "TYPE", "DEFAULT", "LIFECYCLE ID"},
			Row: func(item ChannelViewModel) []string {
				def := ""
				if item.IsDefault {
					def = "*"
				}
				return []string{item.Name, item.Type, def, item.LifecycleID}
			},
		},
		Basic: func(item ChannelViewModel) string {
			return item.Name
		},
	})
}

