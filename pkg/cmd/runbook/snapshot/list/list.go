package list

import (
	"errors"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/spf13/cobra"
	"math"
	"time"
)

const (
	FlagProject = "project"
	FlagRunbook = "runbook"
	FlagLimit   = "limit"
)

type ListFlags struct {
	Project *flag.Flag[string]
	Runbook *flag.Flag[string]
	Limit   *flag.Flag[int32]
}

func NewListFlags() *ListFlags {
	return &ListFlags{
		Project: flag.New[string](FlagProject, false),
		Runbook: flag.New[string](FlagRunbook, false),
		Limit:   flag.New[int32](FlagLimit, false),
	}
}

type SnapshotsAsJson struct {
	Id        string     `json:"Id"`
	Name      string     `json:"Name"`
	Assembled *time.Time `json:"Assembled"`
	Published bool       `json:"Published"`
}

func NewCmdList(f factory.Factory) *cobra.Command {
	listFlags := NewListFlags()
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List runbook snapshots",
		Long:  "List runbook snapshots in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s runbook snapshot list --project "Deploy Web App" --runbook "Run maintenance"
			$ %[1]s runbook snapshot ls
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(cmd, f, listFlags)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&listFlags.Project.Value, listFlags.Project.Name, "p", "", "Name or ID of the project to list runbook snapshots for")
	flags.StringVarP(&listFlags.Runbook.Value, listFlags.Runbook.Name, "r", "", "Name or ID of the runbook to list snapshots for")
	flags.Int32Var(&listFlags.Limit.Value, listFlags.Limit.Name, math.MaxInt32, "Limit the maximum number of results that will be returned")

	return cmd
}

func listRun(cmd *cobra.Command, f factory.Factory, flags *ListFlags) error {
	client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	outputFormat, err := cmd.Flags().GetString(constants.FlagOutputFormat)
	if err != nil { // should never happen, but fallback if it does
		outputFormat = constants.OutputFormatTable
	}

	projectNameOrID := flags.Project.Value
	runbookNameOrID := flags.Runbook.Value

	var selectedProject *projects.Project
	var selectedRunbook *runbooks.Runbook
	if f.IsPromptEnabled() { // this would be AskQuestions if it were bigger
		if projectNameOrID == "" {
			selectedProject, err = selectors.Project("Select the project to list runbook snapshots for", client, f.Ask)
			if err != nil {
				return err
			}
		} else { // project name is already provided, fetch the object because it's needed for further questions
			selectedProject, err = selectors.FindProject(client, projectNameOrID)
			if err != nil {
				return err
			}
			if !constants.IsProgrammaticOutputFormat(outputFormat) {
				cmd.Printf("Project %s\n", output.Cyan(selectedProject.Name))
			}
		}

		if runbookNameOrID == "" {
			selectedRunbook, err = selectors.Runbook("Select the runbook to list snapshots for", client, f.Ask, selectedProject.GetID())
			if err != nil {
				return err
			}
		} else { // project name is already provided, fetch the object because it's needed for further questions
			selectedRunbook, err = selectors.FindRunbook(client, selectedProject, runbookNameOrID)
			if err != nil {
				return err
			}
			if !constants.IsProgrammaticOutputFormat(outputFormat) {
				cmd.Printf("Runbook %s\n", output.Cyan(selectedRunbook.Name))
			}
		}
	} else { // we don't have the executions API backing us and allowing NameOrID; we need to do the lookup ourselves
		if projectNameOrID == "" {
			return errors.New("project must be specified")
		}
		selectedProject, err = selectors.FindProject(client, projectNameOrID)
		if err != nil {
			return err
		}

		if runbookNameOrID == "" {
			return errors.New("runbook must be specified")
		}
		selectedRunbook, err = selectors.FindRunbook(client, selectedProject, runbookNameOrID)
		if err != nil {
			return err
		}
	}

	allSnapshots, err := runbooks.ListSnapshots(client, client.GetSpaceID(), selectedProject.GetID(), selectedRunbook.GetID(), int(flags.Limit.Value))
	if err != nil {
		return err
	}

	return output.PrintArray(allSnapshots.Items, cmd, output.Mappers[*runbooks.RunbookSnapshot]{
		Json: func(s *runbooks.RunbookSnapshot) any {
			return SnapshotsAsJson{
				Id:        s.GetID(),
				Name:      s.Name,
				Assembled: s.Assembled,
				Published: s.GetID() == selectedRunbook.PublishedRunbookSnapshotID,
			}
		},
		Table: output.TableDefinition[*runbooks.RunbookSnapshot]{
			Header: []string{"ID", "NAME", "PUBLISHED", "ASSEMBLED"},
			Row: func(s *runbooks.RunbookSnapshot) []string {
				published := ""
				if selectedRunbook.PublishedRunbookSnapshotID == s.GetID() {
					published = "Yes"
				}
				return []string{s.GetID(), output.Bold(s.Name), output.Green(published), s.Assembled.Format(time.RFC1123Z)}
			},
		},
		Basic: func(s *runbooks.RunbookSnapshot) string {
			return s.Name
		},
	})
}
