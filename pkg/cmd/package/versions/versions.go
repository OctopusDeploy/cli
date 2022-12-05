package versions

import (
	"errors"
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"math"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/feeds"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/packages"
	"github.com/spf13/cobra"
)

const (
	FlagPackage = "package"
	FlagLimit   = "limit"
	FlagFilter  = "filter"
)

type VersionsFlags struct {
	Limit   *flag.Flag[int32]
	Filter  *flag.Flag[string]
	Package *flag.Flag[string]
}

func NewVersionsFlags() *VersionsFlags {
	return &VersionsFlags{
		Limit:   flag.New[int32](FlagLimit, false),
		Filter:  flag.New[string](FlagFilter, false),
		Package: flag.New[string](FlagPackage, false),
	}
}

// The 'versions' command lists all the available versions of a given package. Only the builtin octopus server feed repository is supported.
func NewCmdVersions(f factory.Factory) *cobra.Command {
	versionsFlags := NewVersionsFlags()

	cmd := &cobra.Command{
		Use:   "versions",
		Short: "List versions of a package",
		Long:  "List versions of a package.",
		Example: heredoc.Docf(`
			$ %[1]s package versions --package SomePackage
			$ %[1]s package versions SomePackage --filter beta --limit 5
			$ %[1]s package show SomePackage -n 2
		`, constants.ExecutableName),
		Aliases: []string{"show"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && versionsFlags.Package.Value == "" {
				versionsFlags.Package.Value = args[0]
			}

			return versionsRun(cmd, f, versionsFlags)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&versionsFlags.Package.Value, versionsFlags.Package.Name, "", "", "package ID to show versions for. required.")
	flags.StringVarP(&versionsFlags.Filter.Value, versionsFlags.Filter.Name, "q", "", "filter packages to match only ones that contain the given string")
	flags.Int32VarP(&versionsFlags.Limit.Value, versionsFlags.Limit.Name, "n", 0, "limit the maximum number of results that will be returned")
	flags.SortFlags = false
	return cmd
}

type PackageVersionViewModel struct {
	Version   string
	Published time.Time
	Size      int64 // size in bytes
}

func versionsRun(cmd *cobra.Command, f factory.Factory, flags *VersionsFlags) error {
	packageId := flags.Package.Value
	limit := flags.Limit.Value
	filter := flags.Filter.Value

	if packageId == "" {
		return errors.New("package must be specified")
	}

	octopus, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	// the underlying API has skip, take and paginated results, and will return 30 packages by default.
	// this kind of behaviour isn't going to be the expected default for a CLI, so we instead default to
	// returning "everything" if a limit is unspecified
	if limit <= 0 {
		limit = math.MaxInt32
	}

	// only the builtin server feed can be queried, but the "builtin" feed is different depending on the space you're in
	feedLookupResults, err := octopus.Feeds.Get(feeds.FeedsQuery{FeedType: string(feeds.FeedTypeBuiltIn), Take: 1})
	if err != nil {
		return err
	}
	if len(feedLookupResults.Items) != 1 {
		return fmt.Errorf("cannot locate builtin package feed for space %s", f.GetCurrentSpace().ID)
	}

	page, err := feeds.SearchPackageVersions(octopus, f.GetCurrentSpace().ID, feedLookupResults.Items[0].GetID(), packageId, filter, int(limit))
	if err != nil {
		return err
	}

	return output.PrintArray(page.Items, cmd, output.Mappers[*packages.PackageVersion]{
		Json: func(item *packages.PackageVersion) any {
			return PackageVersionViewModel{
				Version:   item.Version,
				Published: item.Published,
				Size:      item.SizeBytes,
			}
		},
		Table: output.TableDefinition[*packages.PackageVersion]{
			Header: []string{"VERSION", "PUBLISHED", "SIZE"},
			Row: func(item *packages.PackageVersion) []string {
				return []string{item.Version, item.Published.Format("2006-01-02 15:04:05"), humanReadableBytes(item.SizeBytes)} // TODO timezone?
			}},
		Basic: func(item *packages.PackageVersion) string {
			return item.Version
		},
	})
}

// there are about a zillion golang packages for formatting bytes as human-readable values, the top search result of which is
// https://pkg.go.dev/github.com/dustin/go-humanize. However, this package is large and does too much, there's no need to use it
// when we can use this trivial tutorial one instead: https://programming.guide/go/formatting-byte-size-to-human-readable-format.html
func humanReadableBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
