package view

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/buildinformation/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/buildinformation"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

const (
	FlagWeb = "web"
)

type ViewFlags struct {
	Web *flag.Flag[bool]
}

func NewViewFlags() *ViewFlags {
	return &ViewFlags{
		Web: flag.New[bool](FlagWeb, false),
	}
}

type ViewOptions struct {
	*cmd.Dependencies
	*ViewFlags
	idOrName string
}

func NewViewOptions(viewFlags *ViewFlags, dependencies *cmd.Dependencies) *ViewOptions {
	return &ViewOptions{
		Dependencies: dependencies,
		ViewFlags:    viewFlags,
	}
}

func NewCmdView(f factory.Factory) *cobra.Command {
	viewFlags := NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<id>}",
		Short: "View a build information",
		Long:  "View a build information in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s build-information view BuildInformation-1
			$ %[1]s build-info view BuildInformation-1
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewViewOptions(viewFlags, cmd.NewDependencies(f, c))

			if len(args) == 0 {
				return fmt.Errorf("build information identifier is required")
			}
			opts.idOrName = args[0]

			return viewRun(opts, c)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&viewFlags.Web.Value, viewFlags.Web.Name, "w", false, "Open in web browser")

	return cmd
}

func viewRun(opts *ViewOptions, cmd *cobra.Command) error {
	buildInfo, err := buildinformation.GetById(opts.Client, opts.Client.GetSpaceID(), opts.idOrName)
	if err != nil {
		return err
	}

	return output.PrintResource(buildInfo, cmd, output.Mappers[*buildinformation.BuildInformation]{
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
			}

			if len(b.WorkItems) > 0 {
				s.WriteString(output.Bold("\nWork items\n"))
				if b.IssueTrackerName != "" {
					s.WriteString(output.Dim(fmt.Sprintf("Issue tracker: %s\n", b.IssueTrackerName)))
				}
				for _, workItem := range b.WorkItems {
					s.WriteString(fmt.Sprintf("%s %s\n", output.Dim(workItem.ID), workItem.Description))
				}
			}

			link := fmt.Sprintf("%s/app#/%s/library/buildinformation/%s", opts.Host, opts.Client.GetSpaceID(), b.GetID())
			s.WriteString(fmt.Sprintf("\nView this build information in Octopus Deploy: %s\n", output.Blue(link)))

			if opts.Web.Value {
				browser.OpenURL(link)
			}

			return s.String()
		},
	})
}
