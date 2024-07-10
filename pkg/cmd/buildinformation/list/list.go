package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/buildinformation"
	"github.com/spf13/cobra"
)

const (
	FlagLatest    = "latest"
	FlagFilter    = "filter"
	FlagPackageId = "package-id"
)

type ListFlags struct {
	Latest    *flag.Flag[bool]
	Filter    *flag.Flag[string]
	PackageId *flag.Flag[string]
}

func NewListFlags() *ListFlags {
	return &ListFlags{
		Latest:    flag.New[bool](FlagLatest, false),
		Filter:    flag.New[string](FlagFilter, false),
		PackageId: flag.New[string](FlagPackageId, false),
	}
}

func NewCmdList(f factory.Factory) *cobra.Command {
	listFlags := NewListFlags()

	cmd := &cobra.Command{
		Use:   "List",
		Short: "List build information",
		Long:  "List build information in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s build-information list
			$ %[1]s build-information ls
			$ %[1]s build-info list
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(cmd, f, listFlags)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&listFlags.Latest.Value, listFlags.Latest.Name, "", false, "only return the latest build information")
	flags.StringVarP(&listFlags.Filter.Value, listFlags.Filter.Name, "q", "", "filter build information by version")
	flags.StringVarP(&listFlags.PackageId.Value, listFlags.PackageId.Name, "p", "", "filter build information by package id")

	return cmd
}

type BuildInfoAsJson struct {
	Id        string `json:"Id"`
	PackageId string `json:"PackageId"`
	Version   string `json:"Version"`
}

func listRun(cmd *cobra.Command, f factory.Factory, listFlags *ListFlags) error {
	octopus, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	query := buildinformation.BuildInformationQuery{
		Latest:    listFlags.Latest.Value,
		Filter:    listFlags.Filter.Value,
		PackageID: listFlags.PackageId.Value,
	}

	buildInfoResources, err := buildinformation.Get(octopus, f.GetCurrentSpace().ID, query)
	if err != nil {
		return err
	}

	allBuildInfo, err := buildInfoResources.GetAllPages(octopus.Sling())
	if err != nil {
		return err
	}

	return output.PrintArray(allBuildInfo, cmd, output.Mappers[*buildinformation.BuildInformation]{
		Json: func(b *buildinformation.BuildInformation) any {
			return BuildInfoAsJson{
				Id:        b.GetID(),
				PackageId: b.PackageID,
				Version:   b.Version,
			}
		},
		Table: output.TableDefinition[*buildinformation.BuildInformation]{
			Header: []string{"PACKAGE ID", "VERSION"},
			Row: func(b *buildinformation.BuildInformation) []string {
				return []string{output.Bold(b.PackageID), b.Version}
			},
		},
		Basic: func(b *buildinformation.BuildInformation) string {
			return b.PackageID
		},
	})
}
