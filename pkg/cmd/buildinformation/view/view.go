package view

import (
	"fmt"
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
			return shared.BuildInfoAsJson{
				Id:        b.GetID(),
				PackageId: b.PackageID,
				Version:   b.Version,
			}
		},
		Table: output.TableDefinition[*buildinformation.BuildInformation]{
			Header: []string{"PACKAGE ID", "VERSION", "ID"},
			Row: func(b *buildinformation.BuildInformation) []string {
				return []string{output.Bold(b.PackageID), b.Version, output.Dim(b.GetID())}
			},
		},
		Basic: func(b *buildinformation.BuildInformation) string {
			var s strings.Builder

			s.WriteString(fmt.Sprintf("%s %s %s\n", output.Bold(b.PackageID), b.Version, output.Dimf("(%s)", b.GetID())))

			if len(b.WorkItems) > 0 {

			}

			if len(b.Commits) > 0 {

			}

			link := fmt.Sprintf("%s/app#/%s/library/buildinformation/%s", opts.Host, opts.Client.GetSpaceID(), b.GetID())
			s.WriteString(fmt.Sprintf("View this build information in Octopus Deploy: %s", output.Blue(link)))

			if opts.Web.Value {
				browser.OpenURL(link)
			}

			return s.String()
		},
	})
}
