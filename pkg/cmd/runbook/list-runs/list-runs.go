package list_runs

import (
	"errors"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"
)

const (
	FlagProject          = "project"
	FlagRunbook          = "runbook"
	FlagAliasRunbookName = "name"
	FlagLimit            = "limit"
	// TODO Filter.
	// Runbook runs in the web portal have lots of different filtering options.
	// Need to discuss with the team as to which are relevant for the CLI
	// - task state [Canceled,Canceling,Completed,Executing,Failed,Incomplete,Queued,Running,Success,TimedOut,Unsuccessful]
	// - awaiting manual intervention
	// - has warnings or errors
	// - task type (seems to be related to server tasks, not runbooks?)
	// - environment
	// - tenant
	// - date [Queue Time, Start Time, Completed Time] and all have from/to ranges
)

type ListRunsFlags struct {
	Project *flag.Flag[string]
	Runbook *flag.Flag[string]
	Limit   *flag.Flag[int32]
	//Filter  *flag.Flag[string]
}

func NewListRunsFlags() *ListRunsFlags {
	return &ListRunsFlags{
		Project: flag.New[string](FlagProject, false),
		Runbook: flag.New[string](FlagProject, false),
		Limit:   flag.New[int32](FlagLimit, false),
		//Filter:  flag.New[string](FlagFilter, false),
	}
}

func NewCmdListRuns(f factory.Factory) *cobra.Command {
	listRunsFlags := NewListRunsFlags()

	cmd := &cobra.Command{
		Use:   "list-runs",
		Short: "List runbooks in Octopus Deploy",
		Long:  "List runbooks in Octopus Deploy.",
		Example: heredoc.Doc(`
			$ octopus runbook list-runs SomeProject SomeRunbook
			$ octopus runbook list-runs --project SomeProject --runbook SomeRunbook --limit 50
			$ octopus runbook runs -p SomeProject -b SomeRunbook -n 30
		`),
		Aliases: []string{"runs", "listruns", "ls-runs", "lsruns"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && listRunsFlags.Project.Value == "" {
				listRunsFlags.Project.Value = args[0]
				args = args[1:]
			}
			if len(args) > 0 && listRunsFlags.Runbook.Value == "" {
				listRunsFlags.Runbook.Value = args[0]
				args = args[1:]
			}

			return listRunsRun(cmd, f, listRunsFlags)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&listRunsFlags.Project.Value, listRunsFlags.Project.Name, "p", "", "Name or ID of the project to list runbooks for")
	flags.StringVarP(&listRunsFlags.Runbook.Value, listRunsFlags.Runbook.Name, "b", "", "Name or ID of the runbook to list runs for")
	flags.Int32VarP(&listRunsFlags.Limit.Value, listRunsFlags.Limit.Name, "n", 0, "limit the maximum number of results that will be returned")
	return cmd
}

type RunbookRunViewModel struct {
	ID            string
	SnapshotName  string
	Name          string
	QueueTime     string
	StartTime     string
	CompletedTime string
	Duration      string
}

func listRunsRun(cmd *cobra.Command, f factory.Factory, flags *ListRunsFlags) error {
	outputFormat, err := cmd.Flags().GetString(constants.FlagOutputFormat)
	if err != nil { // should never happen, but fallback if it does
		outputFormat = constants.OutputFormatTable
	}

	limit := flags.Limit.Value
	projectNameOrID := flags.Project.Value

	octopus, err := f.GetSpacedClient()
	if err != nil {
		return err
	}

	var selectedProject *projects.Project
	if f.IsPromptEnabled() { // this would be AskQuestions if it were bigger
		selectedProject, err = selectors.SelectOrFindProject(projectNameOrID, "Select the project to list runbooks for", octopus, f.Ask, cmd.OutOrStdout(), outputFormat)
	} else { // we don't have the executions API backing us and allowing NameOrID; we need to do the lookup ourselves
		if projectNameOrID == "" {
			return errors.New("project must be specified")
		}
		selectedProject, err = selectors.FindProject(octopus, projectNameOrID)
		if err != nil {
			return err
		}
	}
	_, _, _ = selectedProject, limit, outputFormat
	//
	//if limit <= 0 {
	//	limit = math.MaxInt32
	//}
	//foundRunbooks, err := runbooks.List(octopus, f.GetCurrentSpace().ID, selectedProject.ID, filter, int(limit))
	//
	//return output.PrintArray(foundRunbooks.Items, cmd, output.Mappers[*runbooks.Runbook]{
	//	Json: func(item *runbooks.Runbook) any {
	//		return RunbookRunViewModel{
	//			ID:          item.ID,
	//			Name:        item.Name,
	//			Description: item.Description,
	//		}
	//	},
	//	Table: output.TableDefinition[*runbooks.Runbook]{
	//		Header: []string{"NAME", "DESCRIPTION"},
	//		Row: func(item *runbooks.Runbook) []string {
	//			return []string{item.Name, item.Description}
	//		}},
	//	Basic: func(item *runbooks.Runbook) string {
	//		return item.Name
	//	},
	//})
	return nil
}
