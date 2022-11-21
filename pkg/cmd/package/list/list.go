package list

import (
	"math"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/packages"
	"github.com/spf13/cobra"
)

const (
	FlagLimit  = "limit"
	FlagFilter = "filter"
)

type ListFlags struct {
	Limit  *flag.Flag[int32]
	Filter *flag.Flag[string]
}

func NewListFlags() *ListFlags {
	return &ListFlags{
		Limit:  flag.New[int32](FlagLimit, false),
		Filter: flag.New[string](FlagFilter, false),
	}
}

func NewCmdList(f factory.Factory) *cobra.Command {
	listFlags := NewListFlags()

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List packages",
		Long:  "List packages in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s package list
			$ %[1]s package list --limit 50 --filter SomePackage
			$ %[1]s package ls -n 30 -q SomePackage
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(cmd, f, listFlags)
		},
	}

	flags := cmd.Flags()
	flags.Int32VarP(&listFlags.Limit.Value, listFlags.Limit.Name, "n", 0, "limit the maximum number of results that will be returned")
	flags.StringVarP(&listFlags.Filter.Value, listFlags.Filter.Name, "q", "", "filter packages to match only ones that contain the given string")
	return cmd
}

type PackageViewModel struct {
	ID          string
	Version     string
	Description string
}

func listRun(cmd *cobra.Command, f factory.Factory, flags *ListFlags) error {
	limit := flags.Limit.Value
	filter := flags.Filter.Value

	octopus, err := f.GetSpacedClient()
	if err != nil {
		return err
	}

	// the underlying API has skip, take and paginated results, and will return 30 packages by default.
	// this kind of behaviour isn't going to be the expected default for a CLI, so we instead default to
	// returning "everything" if a limit is unspecified
	if limit <= 0 {
		limit = math.MaxInt32
	}

	page, err := packages.List(octopus, f.GetCurrentSpace().ID, filter, int(limit))
	if err != nil {
		return err
	}

	return output.PrintArray(page.Items, cmd, output.Mappers[*packages.Package]{
		Json: func(item *packages.Package) any {
			return PackageViewModel{
				ID:          item.PackageID,
				Version:     item.Version,
				Description: item.Description,
			}
		},
		Table: output.TableDefinition[*packages.Package]{
			Header: []string{"ID", "HIGHEST VERSION", "DESCRIPTION"},
			Row: func(item *packages.Package) []string {
				return []string{item.PackageID, item.Version, item.Description}
			}},
		Basic: func(item *packages.Package) string {
			return item.PackageID
		},
	})
}
