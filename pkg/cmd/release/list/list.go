package release

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/services"
	"github.com/spf13/cobra"
)

func NewCmdList(client factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List releases in an instance of Octopus Deploy",
		Long:  "List releases in an instance of Octopus Deploy.",
		Example: heredoc.Doc(`
			$ octopus release list"
		`),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			octopusClient, err := client.GetSpacedClient()
			if err != nil {
				return err
			}

			type ReleaseViewModel struct {
				Channel   string
				ChannelID string `json:",omitempty"`
				Project   string
				ProjectID string `json:",omitempty"`
				Version   string
			}

			// page-fetching loop. TODO sync with dom
			var allOutput []ReleaseViewModel

			caches := util.MapCollectionCacheContainer{}

			releasesPage, err := octopusClient.Releases.Get(releases.ReleasesQuery{}) // get all; server's default page size
			for releasesPage != nil {
				if err != nil {
					return err
				}

				pageOutput, err := util.MapCollectionWithLookups(
					&caches,
					releasesPage.Items,
					func(item *releases.Release) []string { // set of keys to lookup
						return []string{item.ChannelID, item.ProjectID}
					},
					func(item *releases.Release, lookup []string) ReleaseViewModel { // result producer
						return ReleaseViewModel{
							ChannelID: item.ChannelID,
							Channel:   lookup[0],
							ProjectID: item.ProjectID,
							Project:   lookup[1],
							Version:   item.Version}
					},
					// lookup for channel names
					func(keys []string) ([]string, error) {
						// Take(len) is important here just in case we have more than 30 channelsToLookup (server's default page size is 30 and we'd have to deal with pagination)
						lookupResult, err := octopusClient.Channels.Get(channels.Query{IDs: keys, Take: len(keys)})
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
					// lookup for project names
					func(keys []string) ([]string, error) {
						lookupResult, err := octopusClient.Projects.Get(projects.ProjectsQuery{IDs: keys, Take: len(keys)})
						if err != nil {
							return nil, err
						}
						return util.ExtractValuesMatchingKeys(
							lookupResult.Items,
							keys,
							func(x *projects.Project) string { return x.ID },
							func(x *projects.Project) string { return x.Name },
						), nil
					})

				if err != nil {
					return err
				}

				allOutput = append(allOutput, pageOutput...)

				// TODO replace with proper API client page fetching when that becomes available
				if releasesPage.Links.PageNext != "" {
					nextPage := releases.Releases{}
					resp, err := services.ApiGet(octopusClient.Releases.GetClient(), &nextPage, releasesPage.Links.PageNext)
					if err != nil {
						return err
					}

					releasesPage = resp.(*releases.Releases)
				} else {
					releasesPage = nil // break the loop
				}
			}

			return output.PrintArray(allOutput, cmd, output.Mappers[ReleaseViewModel]{
				Json: func(item ReleaseViewModel) any {
					return item
				},
				Table: output.TableDefinition[ReleaseViewModel]{
					Header: []string{"VERSION", "PROJECT", "CHANNEL"},
					Row: func(item ReleaseViewModel) []string {
						return []string{output.Bold(item.Version), item.Project, item.Channel}
					}},
				Basic: func(item ReleaseViewModel) string {
					return item.Version
				},
			})
		},
	}

	return cmd
}
