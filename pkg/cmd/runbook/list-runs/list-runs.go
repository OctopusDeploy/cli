package list_runs

import (
	"errors"
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd/runbook/run"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tasks"
	"github.com/spf13/cobra"
	"math"
	"time"
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
		Runbook: flag.New[string](FlagRunbook, false),
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
	flags.SortFlags = false

	flagAliases := make(map[string][]string, 1)
	util.AddFlagAliasesString(flags, FlagRunbook, flagAliases, FlagAliasRunbookName)
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		util.ApplyFlagAliases(cmd.Flags(), flagAliases)
		return nil
	}
	return cmd
}

type RunbookRunViewModel struct {
	ID            string  `json:"Id,omitempty"`
	TaskID        string  `json:"TaskId,omitempty"`
	State         string  `json:"State,omitempty"`
	QueueTime     string  `json:"QueueTime,omitempty"`
	StartTime     string  `json:"StartTime,omitempty"`
	CompletedTime string  `json:"CompletedTime,omitempty"`
	Duration      float64 `json:"Duration,omitempty"`
}

func listRunsRun(cmd *cobra.Command, f factory.Factory, flags *ListRunsFlags) error {
	outputFormat, err := cmd.Flags().GetString(constants.FlagOutputFormat)
	if err != nil { // should never happen, but fallback if it does
		outputFormat = constants.OutputFormatTable
	}

	limit := flags.Limit.Value
	projectNameOrID := flags.Project.Value
	runbookName := flags.Runbook.Value

	octopus, err := f.GetSpacedClient()
	if err != nil {
		return err
	}
	space := f.GetCurrentSpace()

	var selectedProject *projects.Project
	var selectedRunbook *runbooks.Runbook
	if f.IsPromptEnabled() { // this would be AskQuestions if it were bigger
		selectedProject, err = selectors.SelectOrFindProject(projectNameOrID, "Select the project to list runbooks for", octopus, f.Ask, cmd.OutOrStdout(), outputFormat)
		if err != nil {
			return err
		}

		if runbookName == "" {
			selectedRunbook, err = run.SelectRunbook(octopus, f.Ask, "Select runbook to list runs for", space, selectedProject)
			if err != nil {
				return err
			}
		} else {
			selectedRunbook, err = run.FindRunbook(octopus, space.ID, selectedProject.ID, runbookName)
			if err != nil {
				return err
			}
			if !constants.IsProgrammaticOutputFormat(outputFormat) {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Runbook %s\n", output.Cyan(selectedRunbook.Name))
			}
		}

	} else { // we don't have the executions API backing us and allowing NameOrID; we need to do the lookup ourselves
		if projectNameOrID == "" {
			return errors.New("project must be specified")
		}
		selectedProject, err = selectors.FindProject(octopus, projectNameOrID)
		if err != nil {
			return err
		}

		if runbookName == "" {
			return errors.New("runbook must be specified")
		}
		selectedRunbook, err = run.FindRunbook(octopus, space.ID, selectedProject.ID, runbookName)
		if err != nil {
			return err
		}
	}

	if limit <= 0 {
		limit = math.MaxInt32
	}
	tasksQuery := tasks.TasksQuery{Runbook: selectedRunbook.ID, Project: selectedProject.ID, Spaces: []string{space.ID}, Take: int(limit)}
	runTasks, err := octopus.Tasks.Get(tasksQuery)

	return output.PrintArray(runTasks.Items, cmd, output.Mappers[*tasks.Task]{
		Json: func(item *tasks.Task) any {
			startTime, endTime, queueTime := "", "", ""
			duration := float64(0)
			if item.StartTime != nil && !item.StartTime.IsZero() {
				startTime = item.StartTime.Format(time.RFC3339)
			}
			if item.CompletedTime != nil && !item.CompletedTime.IsZero() {
				endTime = item.CompletedTime.Format(time.RFC3339)
			}
			if item.QueueTime != nil && !item.QueueTime.IsZero() {
				queueTime = item.QueueTime.Format(time.RFC3339)
			}

			if startTime != "" && endTime != "" {
				d := item.CompletedTime.Sub(*item.StartTime)
				duration = d.Seconds()
			}

			runId := ""
			if r, ok := item.Arguments["RunbookRunId"].(string); ok {
				runId = r
			}

			return RunbookRunViewModel{ID: runId, TaskID: item.ID, State: item.State, QueueTime: queueTime, StartTime: startTime, CompletedTime: endTime, Duration: duration}
		},
		Table: output.TableDefinition[*tasks.Task]{
			Header: []string{"STATE", "QUEUE TIME", "START TIME", "END TIME", "DURATION"},
			Row: func(item *tasks.Task) []string {
				const timeFormat = "2 Jan 06 15:04:05"

				startTime, endTime, queueTime, duration := "", "", "", ""
				if item.StartTime != nil && !item.StartTime.IsZero() {
					startTime = item.StartTime.Local().Format(timeFormat)
				}
				if item.CompletedTime != nil && !item.CompletedTime.IsZero() {
					endTime = item.CompletedTime.Local().Format(timeFormat)
				}
				if item.QueueTime != nil && !item.QueueTime.IsZero() {
					queueTime = item.QueueTime.Local().Format(timeFormat)
				}
				if startTime != "" && endTime != "" {
					d := item.CompletedTime.Sub(*item.StartTime)
					duration = d.String()
				}

				return []string{
					item.State,
					queueTime,
					startTime,
					endTime,
					duration}
			}},
		Basic: func(item *tasks.Task) string {
			return item.Name
		},
	})
}
