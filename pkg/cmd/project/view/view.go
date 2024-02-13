package view

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/apiclient"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
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
	Client   *client.Client
	Host     string
	idOrName string
	flags    *ViewFlags
	cmd      *cobra.Command
}

func NewCmdView(f factory.Factory) *cobra.Command {
	viewFlags := NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id> | <slug>}",
		Short: "View a project",
		Long:  "View a project in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project view 'Deploy Web App'
			$ %[1]s project view Projects-9000
			$ %[1]s project view deploy-web-app
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}

			opts := &ViewOptions{
				client,
				f.GetCurrentHost(),
				args[0],
				viewFlags,
				cmd,
			}

			return viewRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&viewFlags.Web.Value, viewFlags.Web.Name, "w", false, "Open in web browser")

	return cmd
}

func viewRun(opts *ViewOptions) error {
	project, err := opts.Client.Projects.GetByIdentifier(opts.idOrName)
	if err != nil {
		return err
	}

	outputFormat, err := opts.cmd.Flags().GetString(constants.FlagOutputFormat)

	if err != nil { // should never happen, but fallback if it does
		outputFormat = constants.OutputFormatTable
	}
	out := opts.cmd.OutOrStdout()

	projectVcsBranch := "N/A"
	if project.IsVersionControlled {
		projectVcsBranch = project.PersistenceSettings.(projects.GitPersistenceSettings).DefaultBranch()
	}

	projectDescription := project.Description
	if projectDescription == "" {
		projectDescription = constants.NoDescription
	}

	url := opts.Host + project.Links["Web"]

	type ViewData struct {
		Name                string `json:"name"`
		Slug                string `json:"slug"`
		Description         string `json:"description"`
		IsVersionControlled bool   `json:"isversioncontrolled"`
		Branch              string `json:"branch"`
		Url                 string `json:"url"`
	}

	switch strings.ToLower(outputFormat) {
	case constants.OutputFormatBasic:
		fmt.Fprintf(out, "Name: %s %s\n", output.Bold(project.Name), output.Dimf("(%s)", project.Slug))
		fmt.Fprintf(out, "Description: %s\n", output.Dim(projectDescription))
		fmt.Fprintf(out, "Is version controlled: %s\n", output.Cyanf("%t", project.IsVersionControlled))
		fmt.Fprintf(out, "Branch: %s\n", output.Cyan(projectVcsBranch))
		fmt.Fprintf(out, "View this project in Octopus Deploy: %s\n", output.Blue(url))
	case constants.OutputFormatTable:
		t := output.NewTable(out)
		t.AddRow(output.Bold("KEY"), output.Bold("VALUE"))
		t.AddRow("Name", project.Name)
		t.AddRow("Slug", project.Slug)
		t.AddRow("Description", project.Description)
		t.AddRow("IsVersionControlled", fmt.Sprintf("%t", project.IsVersionControlled))
		t.AddRow("Branch", projectVcsBranch)
		t.AddRow("Url", fmt.Sprintf("%s", output.Blue(url)))
		t.Print()
	case constants.OutputFormatJson:
		viewData := &ViewData{}
		viewData.Name = project.Name
		viewData.Slug = project.Slug
		viewData.Description = project.Description
		viewData.IsVersionControlled = project.IsVersionControlled
		viewData.Branch = projectVcsBranch
		viewData.Url = url
		data, _ := json.MarshalIndent(viewData, "", "  ")
		opts.cmd.Println(string(data))
	}

	if opts.flags.Web.Value {
		browser.OpenURL(url)
	}

	return nil
}
