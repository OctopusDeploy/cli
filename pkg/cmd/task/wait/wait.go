package wait

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tasks"
	"github.com/spf13/cobra"
)

const (
	FlagTimeout      = "timeout"
	FlagProgress     = "progress"
	DefaultTimeout   = 600
	indentSize       = 4
	separator        = "─"
	sepLength        = 29
	timeFormat       = "02-01-2006 15:04:05"
	logIndentLevel   = 6
	taskHeaderIndent = "──────"
	logLineIndent    = "                  "
)

type WaitOptions struct {
	*cmd.Dependencies
	taskIDs                []string
	GetServerTasksCallback ServerTasksCallback
	GetTaskDetailsCallback TaskDetailsCallback
}

type ServerTasksCallback func([]string) ([]*tasks.Task, error)
type TaskDetailsCallback func(string) (*tasks.TaskDetailsResource, error)

type LogState struct {
	completedChildIds map[string]bool
}

func NewLogState() *LogState {
	return &LogState{
		completedChildIds: make(map[string]bool),
	}
}

func NewWaitOps(dependencies *cmd.Dependencies, taskIDs []string) *WaitOptions {
	return &WaitOptions{
		Dependencies:           dependencies,
		GetServerTasksCallback: GetServerTasksCallback(dependencies.Client),
		GetTaskDetailsCallback: GetTaskDetailsCallback(dependencies.Client),
	}
}

func NewCmdWait(f factory.Factory) *cobra.Command {
	var timeout int
	var showProgress bool
	cmd := &cobra.Command{
		Use:     "wait [TaskIDs]",
		Short:   "Wait for task(s) to finish",
		Long:    "Wait for a provided list of task(s) to finish",
		Example: heredoc.Docf("$ %s task wait", constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			taskIDs := make([]string, len(args))
			copy(taskIDs, args)

			taskIDs = append(taskIDs, util.ReadValuesFromPipe()...)

			dependencies := cmd.NewDependencies(f, c)
			opts := NewWaitOps(dependencies, taskIDs)

			return WaitRun(opts.Out, taskIDs, opts.GetServerTasksCallback, opts.GetTaskDetailsCallback, timeout, showProgress)
		},
	}

	flags := cmd.Flags()
	flags.IntVar(&timeout, FlagTimeout, DefaultTimeout, "Duration to wait (in seconds) before stopping execution")
	flags.BoolVar(&showProgress, FlagProgress, false, "Show detailed progress of the tasks")

	return cmd
}

func WaitRun(out io.Writer, taskIDs []string, getServerTasksCallback ServerTasksCallback, getTaskDetailsCallback TaskDetailsCallback, timeout int, showProgress bool) error {
	if len(taskIDs) == 0 {
		return fmt.Errorf("no server task IDs provided, at least one is required")
	}

	if showProgress && len(taskIDs) > 1 {
		return fmt.Errorf("--progress flag is only supported when waiting for a single task")
	}

	tasks, err := getServerTasksCallback(taskIDs)
	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		return fmt.Errorf("no server tasks found")
	}

	pendingTaskIDs := make([]string, 0)
	failedTaskIDs := make([]string, 0)
	for _, t := range tasks {
		if t.IsCompleted == nil || !*t.IsCompleted {
			pendingTaskIDs = append(pendingTaskIDs, t.ID)
		}
		if (t.IsCompleted != nil && *t.IsCompleted) && (t.FinishedSuccessfully != nil && !*t.FinishedSuccessfully) {
			failedTaskIDs = append(failedTaskIDs, t.ID)
		}

		printTaskInfo(out, t)
	}

	if len(pendingTaskIDs) == 0 {
		if len(failedTaskIDs) != 0 {
			return fmt.Errorf("One or more deployment tasks failed: %s", strings.Join(failedTaskIDs, ", "))
		}
		return nil
	}

	gotError := make(chan error, 1)
	done := make(chan bool, 1)
	logState := NewLogState()

	go func() {
		for len(pendingTaskIDs) != 0 {
			time.Sleep(5 * time.Second)
			tasks, err = getServerTasksCallback(pendingTaskIDs)
			if err != nil {
				gotError <- err
				return
			}
			for _, t := range tasks {
				if showProgress {
					details, err := getTaskDetailsCallback(t.ID)
					if err != nil {
						continue // Skip progress display if we can't get details
					}

					if len(details.ActivityLogs) > 0 {
						// Process all activities
						for _, activity := range details.ActivityLogs {
							printActivityElement(out, activity, 0, logState)
						}
					}
				}

				if t.IsCompleted != nil && *t.IsCompleted {
					if t.FinishedSuccessfully != nil && !*t.FinishedSuccessfully {
						failedTaskIDs = append(failedTaskIDs, t.ID)
					}
					printTaskInfo(out, t)
					pendingTaskIDs = removeTaskID(pendingTaskIDs, t.ID)
				}
			}
		}
		if len(failedTaskIDs) != 0 {
			gotError <- fmt.Errorf("One or more deployment tasks failed: %s", strings.Join(failedTaskIDs, ", "))
			return
		}
		done <- true
	}()

	select {
	case <-done:
		return nil
	case err := <-gotError:
		return err
	case <-time.After(time.Duration(timeout) * time.Second):
		return fmt.Errorf("timeout while waiting for pending tasks")
	}
}

func GetServerTasksCallback(octopus *client.Client) ServerTasksCallback {
	return func(taskIDs []string) ([]*tasks.Task, error) {
		query := tasks.TasksQuery{
			IDs: taskIDs,
		}

		resourceTasks, err := octopus.Tasks.Get(query)
		if err != nil {
			return nil, err
		}

		tasks, err := resourceTasks.GetAllPages(octopus.Sling())
		if err != nil {
			return nil, err
		}

		return tasks, nil
	}
}

func GetTaskDetailsCallback(octopus *client.Client) TaskDetailsCallback {
	return func(taskID string) (*tasks.TaskDetailsResource, error) {
		return tasks.GetDetails(octopus, octopus.GetSpaceID(), taskID)
	}
}

func removeTaskID(taskIDs []string, taskID string) []string {
	for i, p := range taskIDs {
		if p == taskID {
			taskIDs[i] = taskIDs[len(taskIDs)-1]
			taskIDs = taskIDs[:len(taskIDs)-1]
			break
		}
	}
	return taskIDs
}

func getIndentation(level int) string {
	return strings.Repeat(" ", level*indentSize)
}

func formatSeparatorLine(indent string) string {
	return indent + strings.Repeat(separator, sepLength)
}

func formatTaskHeader(taskID string, description string, status string, startTime *time.Time, endTime *time.Time, duration time.Duration) string {
	if startTime == nil || endTime == nil {
		return fmt.Sprintf("%s: %s: %s", taskID, description, status)
	}

	return fmt.Sprintf("\n%s %s %s\n   Name: %s\n   Status: %s\n   Started: %s\n   Ended: %s\n   Duration: %s\n",
		taskHeaderIndent,
		taskID,
		taskHeaderIndent,
		description,
		status,
		startTime.Format(timeFormat),
		endTime.Format(timeFormat),
		duration)
}

func formatLogLine(timeStr, category, message string) string {
	return fmt.Sprintf("%s%-19s      %-8s %s", logLineIndent, timeStr, category, message)
}

func formatRetryMessage(message string) string {
	return fmt.Sprintf("%s%s", logLineIndent, output.Yellow(fmt.Sprintf("------ %s ------", message)))
}

func printActivityElement(out io.Writer, activity *tasks.ActivityElement, indent int, logState *LogState) {
	// Process children activities (these are the steps)
	for _, child := range activity.Children {
		// Print logs for any status except Pending and Running
		if child.Status != "Pending" && child.Status != "Running" && !logState.completedChildIds[child.ID] {
			line := fmt.Sprintf("         %s: %s", child.Status, child.Name)

			var timeInfo string
			if child.Started != nil && child.Ended != nil {
				startTime := child.Started.Format(timeFormat)
				endTime := child.Ended.Format(timeFormat)
				duration := child.Ended.Sub(*child.Started).Round(time.Second)
				indentStr := getIndentation(logIndentLevel)
				sep := formatSeparatorLine(indentStr)
				timeInfo = fmt.Sprintf("\n%s\n%sStarted:   %s\n%sEnded:     %s\n%sDuration:  %s\n%s",
					sep,
					indentStr, startTime,
					indentStr, endTime,
					indentStr, duration,
					sep)
			}

			switch child.Status {
			case "Success":
				line = output.Green(line)
			case "Failed":
				line = output.Red(line)
			case "Skipped":
				line = output.Yellow(line)
			case "SuccessWithWarning":
				line = output.Yellow(line)
			case "Canceled":
				line = output.Yellow(line)
			}

			if timeInfo != "" {
				line = line + timeInfo
			}
			fmt.Fprintln(out, line)

			for _, stepChild := range child.Children {
				if stepChild.Status != "Pending" && stepChild.Status != "Running" {
					var lastWasRetry bool
					for _, logElement := range stepChild.LogElements {
						message := logElement.MessageText
						timeStr := logElement.OccurredAt.Format(timeFormat)
						category := logElement.Category

						if strings.Contains(message, "Retry (attempt") {
							fmt.Fprintln(out, formatRetryMessage(message))
							lastWasRetry = true
						} else if lastWasRetry && strings.Contains(message, "Starting") {
							lastWasRetry = false
						}

						logLine := formatLogLine(timeStr, category, message)
						switch strings.ToLower(category) {
						case "warning":
							logLine = output.Yellow(logLine)
						case "error", "fatal":
							logLine = output.Red(logLine)
						}

						fmt.Fprintln(out, logLine)
					}
				}
			}

			logState.completedChildIds[child.ID] = true
		}
	}
}

func formatTaskStatus(state string) string {
	switch state {
	case "Failed", "TimedOut":
		return output.Red(state)
	case "Success":
		return output.Green(state)
	case "Queued", "Executing", "Cancelling", "Canceled":
		return output.Yellow(state)
	default:
		return state
	}
}

func printTaskInfo(out io.Writer, t *tasks.Task) {
	status := formatTaskStatus(t.State)
	if t.StartTime != nil && t.CompletedTime != nil {
		duration := t.CompletedTime.Sub(*t.StartTime).Round(time.Second)
		timeInfo := formatTaskHeader(t.ID, t.Description, status, t.StartTime, t.CompletedTime, duration)
		fmt.Fprintln(out, timeInfo)
	} else {
		fmt.Fprintln(out, formatTaskHeader(t.ID, t.Description, status, nil, nil, time.Duration(0)))
	}
}
