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

func MapCollectionWithLookup[T any, TResult any](
	cache map[string]string,                    // cache for keys (typically this will store a mapping of ID->Name)
	collection []T,                             // input (e.g. list of Releases)
	keySelector func(T) string,                 // fetches the key (e.g given a Release, returns the ChannelID)
	mapper func(T, string) TResult,             // fetches the value to lookup (e.g given a Channel and the name, does the mapping to return the output struct)
	runLookup func([]string) ([]string, error), // callback to go fetch values for the keys (given a list of Channel IDs, it should return the list of associated Channel Names)
) ([]TResult, error) {
	// this works in two passes. First it walks the collection and sees if there is anything it needs to look up.
	// (it doesn't produce any results or do any mapping, just key selection).
	// then, if necessary, it looks up the keys and populates the cache
	// for the second pass we walk the collection and assign keys to cached values
	var keysToLookup []string = nil
	for _, item := range collection {
		key := keySelector(item) // returns channel.ID
		_, ok := cache[key]
		if !ok { // we haven't seen this value in the cache, we need to go and look something up
			if keysToLookup == nil {
				keysToLookup = []string{key}
			} else {
				if !util.SliceContains(keysToLookup, key) {
					keysToLookup = append(keysToLookup, key)
				} // else we've already seen this key
			}
		}
	}

	// if we don't know the names of some things, go and look them up, then restart the mapping process.
	// we do a second pass over the whole array, which isn't perfectly efficient, but it's simpler, and
	// we are dealing with small numbers here (page size of 100 or less) so perf will be fine
	if keysToLookup != nil {
		lookedUpValues, err := runLookup(keysToLookup)
		if err != nil {
			return nil, err
		}
		for idx, value := range lookedUpValues {
			cache[keysToLookup[idx]] = value
		}
	}

	results := []TResult{}
	for _, item := range collection {
		key := keySelector(item)
		value, ok := cache[key]
		if ok {
			results = append(results, mapper(item, value))
		} else {
			// this shouldn't happen as we should have looked up the channel name already; substitute the Zero value
			// in lieu of crashing because it's not that important
			results = append(results, *new(TResult))
		}
	}
	return results, nil
}

// like MapCollectionWithLookup except it can lookup more than one attribute.
// e.g. a Release has both a Project and a Channel that we'd like to lookup the name of
func MapCollectionWithLookups[T any, TResult any](
	caches []map[string]string,                    // cache for keys (typically this will store a mapping of ID->[Name, Name]). YOU MUST PREALLOCATE THIS
	collection []T,                                // input (e.g. list of Releases)
	keySelector func(T) []string,                  // fetches the keys (e.g given a Release, returns the [ChannelID, ProjectID]
	mapper func(T, []string) TResult,              // fetches the value to lookup (e.g given a Release and the [ChannelName,ProjectName], does the mapping to return the output struct)
	runLookups []func([]string) ([]string, error), // callbacks to go fetch values for the keys (given a list of Channel IDs, it should return the list of associated Channel Names)
) ([]TResult, error) {
	// this works in two passes. First it walks the collection and sees if there is anything it needs to look up.
	// (it doesn't produce any results or do any mapping, just key selection).
	// then, if necessary, it looks up the keys and populates the cache
	// for the second pass we walk the collection and assign keys to cached values

	var allKeysToLookup = make([][]string, len(caches)) // preallocate the right number of nils
	for _, item := range collection {
		keys := keySelector(item)
		// we can't use an array of strings as a map key; build a composite key.
		for i, key := range keys {
			_, ok := caches[i][key]
			if !ok { // we haven't seen this value in the cache, we need to go and look something up
				if allKeysToLookup[i] == nil {
					allKeysToLookup[i] = []string{key}
				} else {
					if !util.SliceContains(allKeysToLookup[i], key) {
						allKeysToLookup[i] = append(allKeysToLookup[i], key)
					} // else we've already seen this key
				}
			}
		}
	}

	// if we don't know the names of some things, go and look them up, then restart the mapping process.
	// we do a second pass over the whole array, which isn't perfectly efficient, but it's simpler, and
	// we are dealing with small numbers here (page size of 100 or less) so perf will be fine
	for lookupIdx, keysToLookup := range allKeysToLookup {
		if keysToLookup != nil {
			lookedUpValues, err := runLookups[lookupIdx](keysToLookup)
			if err != nil {
				return nil, err
			}
			for valueIdx, value := range lookedUpValues {
				caches[lookupIdx][keysToLookup[valueIdx]] = value
			}
		}
	}

	var results []TResult
	for _, item := range collection {
		keys := keySelector(item)
		values := make([]string, len(keys))
		for i, key := range keys {
			value, ok := caches[i][key]
			if ok {
				values[i] = value
			} else {
				values[i] = ""
			}
		}

		results = append(results, mapper(item, values))
	}
	return results, nil
}

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

			caches := []map[string]string{
				{}, // first cache for Channel Names
				{}, // second cache for Project Names
			}

			type ReleaseOutput struct {
				Channel string
				Project string
				Version string
			}

			// page-fetching loop. TODO sync with dom
			releaseOutput := []ReleaseOutput{}

			releasesPage, err := octopusClient.Releases.Get(releases.ReleasesQuery{}) // get all; server's default page size
			for releasesPage != nil {
				if err != nil {
					return err
				}

				pageOutput, err := MapCollectionWithLookups(
					caches,
					releasesPage.Items,
					func(item *releases.Release) []string {
						return []string{item.ChannelID, item.ProjectID}
					},
					func(item *releases.Release, cachedValues []string) ReleaseOutput {
						return ReleaseOutput{Channel: cachedValues[0], Project: cachedValues[1], Version: item.Version}
					},
					[]func(keys []string) ([]string, error){
						// lookup for channel names
						func(keys []string) ([]string, error) {
							lookupResult, err := octopusClient.Channels.Get(channels.Query{
								IDs:  keys,
								Take: len(keys), // important here just in case we have more than 30 channelsToLookup (server's default page size is 30 and we'd have to deal with pagination)
							})
							if err != nil {
								return nil, err
							}

							// the server doesn't neccessarily return items in the order matching 'keys'
							// so we have to build the association manually
							results := make([]string, len(keys))
							for idx, key := range keys {
								// find the key in the lookupResult.Items and slot it into the correct array index
								for _, item := range lookupResult.Items {
									if item.ID == key {
										results[idx] = item.Name
										// TODO we could optimise this by removing something from lookupResult.Items once we've found it,
										// or by front-loading them into a map[string]string. Dig into that later for performance if need be
									}
								}
							}
							return results, nil
						},
						// lookup for project names
						func(keys []string) ([]string, error) {
							lookupResult, err := octopusClient.Projects.Get(projects.ProjectsQuery{
								IDs:  keys,
								Take: len(keys),
							})
							if err != nil {
								return nil, err
							}

							results := make([]string, len(keys))
							for idx, key := range keys {
								// find the key in the lookupResult.Items and slot it into the correct array index
								for _, item := range lookupResult.Items {
									if item.ID == key {
										results[idx] = item.Name
									}
								}
							}
							return results, nil
						},
					},
				)

				releaseOutput = append(releaseOutput, pageOutput...)

				if err != nil {
					return err
				}

				// TODO extend the API Client to add FetchNextPage() so we can make this cleaner
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

			return output.PrintArray(releaseOutput, cmd, output.Mappers[ReleaseOutput]{
				Json: func(item ReleaseOutput) any {
					return item
				},
				Table: output.TableDefinition[ReleaseOutput]{
					Header: []string{"PROJECT", "CHANNEL", "VERSION"},
					Row: func(item ReleaseOutput) []string {
						return []string{output.Bold(item.Project), item.Channel, item.Version}
					}},
				Basic: func(item ReleaseOutput) string {
					return item.Version
				},
			})
		},
	}

	return cmd
}
