package list

import (
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/spf13/cobra"
)

const (
	FlagProject = "project"
)

type ListFlags struct {
	Project *flag.Flag[string]
}

func NewListFlags() *ListFlags {
	return &ListFlags{
		Project: flag.New[string](FlagProject, false),
	}
}

type ReleaseViewModel struct {
	ID           string
	ReleaseNotes string
	Assembled    time.Time
	Channel      string
	Version      string
}

func NewCmdList(f factory.Factory) *cobra.Command {
	listFlags := NewListFlags()
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List releases",
		Long:  "List releases in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s release list myProject
			%[1]s release ls "Other Project"
			%[1]s release list --project myProject
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
	flags.StringVarP(&listFlags.Project.Value, listFlags.Project.Name, "p", "", "Name or ID of the project to list releases for")
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
		"Select the project to list releases for", projectNameOrID)
	if err != nil {
		return err
	}
	// echo the project when it came from the command line, so there is always a line
	// showing what was selected
	if f.IsPromptEnabled() && projectNameOrID != "" && !constants.IsProgrammaticOutputFormat(outputFormat) {
		cmd.Printf("Project %s\n", output.Cyan(selectedProject.Name))
	}

	foundReleases, err := octopus.Projects.GetReleases(selectedProject) // does paging internally
	if err != nil {
		return err
	}

	caches := util.MapCollectionCacheContainer{}
	allReleases, err := util.MapCollectionWithLookups(
		&caches,
		foundReleases,
		func(item *releases.Release) []string { // set of keys to lookup
			return []string{item.ChannelID}
		},
		func(item *releases.Release, lookup []string) ReleaseViewModel { // result producer
			item.Links = nil
			return ReleaseViewModel{
				ID:        item.ID,
				Assembled: item.Assembled,
				Channel:   lookup[0],
				Version:   item.Version,
			}
		},
		// lookup for channel names
		func(keys []string) ([]string, error) {
			// Take(len) is important here just in case we have more than 30 channelsToLookup (server's default page size is 30 and we'd have to deal with pagination)
			lookupResult, err := octopus.Channels.Get(channels.Query{IDs: keys, Take: len(keys)})
			if err != nil {
				return nil, err
			}
			return util.ExtractValuesMatchingKeys(
				lookupResult.Items,
				keys,
				func(x *channels.Channel) string { return x.ID },
				func(x *channels.Channel) string { return x.Name },
			), nil
		},
	)
	if err != nil {
		return err
	}

	return output.PrintArray(allReleases, cmd, output.Mappers[ReleaseViewModel]{
		Json: func(item ReleaseViewModel) any {
			return item
		},
		Table: output.TableDefinition[ReleaseViewModel]{
			Header: []string{"VERSION", "CHANNEL", "CREATED"},
			Row: func(item ReleaseViewModel) []string {
				return []string{item.Version, item.Channel, item.Assembled.Format(time.RFC1123Z)}
			}},
		Basic: func(item ReleaseViewModel) string {
			return item.Version
		},
	})
}
