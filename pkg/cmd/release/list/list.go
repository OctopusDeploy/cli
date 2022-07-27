package release

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util"
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
			client, err := client.GetSpacedClient()
			if err != nil {
				return err
			}

			channelIdMap := map[string]string{}

			type ReleaseOutput struct {
				Channel string
				Version string
			}

			// page-fetching loop. TODO sync with dom
			releasesPage, err := client.Releases.Get(releases.ReleasesQuery{Take: 1})
			for releasesPage != nil {
				if err != nil {
					return err
				}

				// map input into ReleaseOutput, looking up channel IDs along the way
				// lookup channel IDs
				releaseOutput := []ReleaseOutput{}
				var channelsToLookup []string = nil
				for _, release := range releasesPage.Items {
					channelName, ok := channelIdMap[release.ChannelID]
					if ok {
						releaseOutput = append(releaseOutput, ReleaseOutput{Channel: channelName, Version: release.Version})
					} else {
						if channelsToLookup == nil {
							channelsToLookup = []string{release.ChannelID}
						} else {
							if !util.SliceContains(channelsToLookup, release.ChannelID) {
								channelsToLookup = append(channelsToLookup, release.ChannelID)
							} // else we've already seen this.
						}
					}
				}

				if channelsToLookup != nil {
					// if we don't know the names of some channels, go and look them up, then restart the mapping process.
				}

				if releasesPage.Links.PageNext != "" {
					nextPage := releases.Releases{}
					resp, err := services.ApiGet(client.Releases.GetClient(), &nextPage, releasesPage.Links.PageNext)
					if err != nil {
						return err
					}

					releasesPage = resp.(*releases.Releases)
				}
			}

			return output.PrintArray(releasesPage.Items, cmd, output.Mappers[*releases.Release]{
				Json: func(item *releases.Release) any {

					return ReleaseJson{Channel: item.ChannelID, Version: item.Version}
				},
				Table: output.TableDefinition[*releases.Release]{
					Header: []string{"CHANNEL", "VERSION"},
					Row: func(item *releases.Release) []string {
						return []string{output.Bold(item.ChannelID), item.Version}
					}},
				Basic: func(item *releases.Release) string {
					return item.Version
				},
			})
		},
	}

	return cmd
}
