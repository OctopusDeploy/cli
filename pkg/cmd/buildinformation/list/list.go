package list

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/buildinformation/shared"
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
			$ %[1]s build-info ls --package-id ThePackage
			$ %[1]s build-info ls --package-id ThePackage --filter 1.2.3
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
			buildInfo := shared.BuildInfoAsJson{
				Id:               b.GetID(),
				PackageId:        b.PackageID,
				Version:          b.Version,
				Branch:           b.Branch,
				BuildEnvironment: b.BuildEnvironment,
				VcsCommitNumber:  b.VcsCommitNumber,
				VcsType:          b.VcsType,
				VcsRoot:          b.VcsRoot,
			}

			if len(b.Commits) > 0 {
				for _, c := range b.Commits {
					buildInfo.Commits = append(buildInfo.Commits, &shared.CommitAsJson{Id: c.ID, Comment: c.Comment})
				}
			}

			if len(b.WorkItems) > 0 {
				for _, w := range b.WorkItems {
					buildInfo.WorkItems = append(buildInfo.WorkItems, &shared.WorkItemAsJson{Id: w.ID, Source: w.Source, Description: w.Description})
				}
			}

			return buildInfo
		},
		Table: output.TableDefinition[*buildinformation.BuildInformation]{
			Header: []string{
				"PACKAGE ID",
				"VERSION",
				"ENVIRONMENT",
				"BUILD NO",
				"BRANCH",
				"COMMITS",
				"WORKITEMS"},
			Row: func(b *buildinformation.BuildInformation) []string {
				return []string{
					output.Bold(b.PackageID),
					b.Version,
					b.BuildEnvironment,
					b.BuildNumber,
					b.Branch,
					strconv.Itoa(len(b.Commits)),
					strconv.Itoa(len(b.WorkItems))}
			},
		},
		Basic: func(b *buildinformation.BuildInformation) string {
			var s strings.Builder

			s.WriteString(fmt.Sprintf("%s %s %s\n", output.Bold(b.PackageID), b.Version, output.Dimf("(%s)", b.GetID())))

			s.WriteString(output.Bold("\nBuild details\n"))
			s.WriteString(output.Dim(fmt.Sprintf("Environment: %s\n", b.BuildEnvironment)))
			s.WriteString(output.Dim(fmt.Sprintf("Branch: %s\n", b.Branch)))
			s.WriteString(output.Dim(fmt.Sprintf("URL: %s\n", output.Blue(b.BuildURL))))

			s.WriteString(output.Bold("\nVCS Details\n"))
			s.WriteString(output.Dim(fmt.Sprintf("Type: %s\n", b.VcsType)))
			s.WriteString(output.Dim(fmt.Sprintf("URL: %s\n", output.Blue(b.VcsRoot))))

			s.WriteString(output.Bold("\nCommit details\n"))
			s.WriteString(output.Dim(fmt.Sprintf("Hash: %s\n", b.VcsCommitNumber)))
			s.WriteString(output.Dim(fmt.Sprintf("URL: %s\n", output.Blue(b.VcsCommitURL))))
			if len(b.Commits) > 0 {
				for _, commit := range b.Commits {
					s.WriteString(fmt.Sprintf("%s %s\n", output.Dim(commit.ID[0:8]), commit.Comment))
				}
			} else {
				s.WriteString("No commits included\n")
			}

			s.WriteString(output.Bold("\nWork items\n"))
			if len(b.WorkItems) > 0 {
				if b.IssueTrackerName != "" {
					s.WriteString(output.Dim(fmt.Sprintf("Issue tracker: %s\n", b.IssueTrackerName)))
				}
				for _, workItem := range b.WorkItems {
					s.WriteString(fmt.Sprintf("%s %s\n", output.Dim(workItem.ID), workItem.Description))
				}
			} else {
				s.WriteString("No work items included\n")
			}

			return s.String()
		},
	})
}
