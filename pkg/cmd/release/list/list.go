package list

import (
	"errors"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
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
	Channel string
	Version string
}

func NewCmdList(f factory.Factory) *cobra.Command {
	listFlags := NewListFlags()
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List releases in Octopus Deploy",
		Long:  "List releases in Octopus Deploy.",
		Example: heredoc.Doc(`
			$ octopus release list myProject
			$ octopus release ls "Other Project"
			$ octopus release list --project myProject
		`),
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

	octopus, err := f.GetSpacedClient()
	if err != nil {
		return err
	}
	spinner := f.Spinner()

	var selectedProject *projects.Project
	if f.IsPromptEnabled() { // this would be AskQuestions if it were bigger
		if projectNameOrID == "" {
			selectedProject, err = selectors.Project("Select the project to list releases for", octopus, f.Ask, spinner)
			if err != nil {
				return err
			}
		} else { // project name is already provided, fetch the object because it's needed for further questions
			selectedProject, err = selectors.FindProject(octopus, spinner, projectNameOrID)
			if err != nil {
				return err
			}
			if !constants.IsProgrammaticOutputFormat(outputFormat) {
				cmd.Printf("Project %s\n", output.Cyan(selectedProject.Name))
			}
		}
	} else { // we don't have the executions API backing us and allowing NameOrID; we need to do the lookup ourselves
		if projectNameOrID == "" {
			return errors.New("project must be specified")
		}
		selectedProject, err = selectors.FindProject(octopus, factory.NoSpinner, projectNameOrID)
		if err != nil {
			return err
		}
	}

	spinner.Start()

	foundReleases, err := octopus.Projects.GetReleases(selectedProject) // does paging internally
	if err != nil {
		spinner.Stop()
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
			return ReleaseViewModel{
				Channel: lookup[0],
				Version: item.Version}
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
	spinner.Stop()
	if err != nil {
		return err
	}

	return output.PrintArray(allReleases, cmd, output.Mappers[ReleaseViewModel]{
		Json: func(item ReleaseViewModel) any {
			return item
		},
		Table: output.TableDefinition[ReleaseViewModel]{
			Header: []string{"VERSION", "CHANNEL"},
			Row: func(item ReleaseViewModel) []string {
				return []string{item.Version, item.Channel}
			}},
		Basic: func(item ReleaseViewModel) string {
			return item.Version
		},
	})
}
