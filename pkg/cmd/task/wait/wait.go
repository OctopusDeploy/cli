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
	FlagTimeout  = "timeout"
	FlagProgress = "progress"

	DefaultTimeout = 600 // 600 seconds : 10 minutes
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
	processedLogs       map[string]bool
	lastActivityStatus  map[string]string
	completedChildIds   map[string]bool
	mainActivityPrinted bool
}

func NewLogState() *LogState {
	return &LogState{
		processedLogs:       make(map[string]bool),
		lastActivityStatus:  make(map[string]string),
		completedChildIds:   make(map[string]bool),
		mainActivityPrinted: false,
	}
}

func (s *LogState) hasProcessed(activity *tasks.ActivityElement, logElement *tasks.ActivityLogElement) bool {
	if logElement == nil {
		key := fmt.Sprintf("activity-%s-%s-%s", activity.ID, activity.Name, activity.Status)
		lastStatus, exists := s.lastActivityStatus[activity.ID]
		if !exists || lastStatus != activity.Status {
			s.lastActivityStatus[activity.ID] = activity.Status
			return false
		}
		return s.processedLogs[key]
	}

	key := fmt.Sprintf("log-%s-%s-%s", activity.ID, logElement.Category, logElement.MessageText)
	return s.processedLogs[key]
}

func (s *LogState) markProcessed(activity *tasks.ActivityElement, logElement *tasks.ActivityLogElement) {
	if logElement == nil {
		key := fmt.Sprintf("activity-%s-%s-%s", activity.ID, activity.Name, activity.Status)
		s.processedLogs[key] = true
		return
	}
	key := fmt.Sprintf("log-%s-%s-%s", activity.ID, logElement.Category, logElement.MessageText)
	s.processedLogs[key] = true
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
		
		status := fmt.Sprintf("%s: %s", t.Description, t.State)
		if t.State == "Failed" {
			status = output.Red(status)
		}
		fmt.Fprintln(out, status)
	}

	if len(pendingTaskIDs) == 0 {
		if len(failedTaskIDs) != 0 {
			return fmt.Errorf("One or more deployment tasks failed.")
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
							if !logState.mainActivityPrinted {
								fmt.Fprintf(out, "Running: %s\n", activity.Name)
								logState.mainActivityPrinted = true
							}
							printActivityElement(out, activity, 0, logState)
						}
					}
				}

				if t.IsCompleted != nil && *t.IsCompleted {
					if t.FinishedSuccessfully != nil && !*t.FinishedSuccessfully {
						failedTaskIDs = append(failedTaskIDs, t.ID)
					}
					fmt.Fprintf(out, "%s: %s\n", t.Description, t.State)
					pendingTaskIDs = removeTaskID(pendingTaskIDs, t.ID)
				}
			}
		}
		if len(failedTaskIDs) != 0 {
			gotError <- fmt.Errorf("One or more deployment tasks failed.")
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
		return octopus.Tasks.GetDetails(taskID)
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

func printActivityElement(out io.Writer, activity *tasks.ActivityElement, indent int, logState *LogState) {
	// Process children activities (these are the steps)
	for _, child := range activity.Children {
		// Print logs for any status except Pending and Running
		if child.Status != "Pending" && child.Status != "Running" && !logState.completedChildIds[child.ID] {
			line := fmt.Sprintf("         %s: %s", child.Status, child.Name)
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
			fmt.Fprintln(out, line)

			// Each step has child activities (like "Octopus Server") that contain the actual logs
			for _, stepChild := range child.Children {
				if stepChild.Status != "Pending" && stepChild.Status != "Running" {
					// Print all log elements for this step's child
					var lastWasRetry bool
					for _, logElement := range stepChild.LogElements {
						// Extract just the message without the timestamp
						message := logElement.MessageText
						if idx := strings.LastIndex(message, " at "); idx != -1 {
							// Check if the remaining part matches a date pattern
							remainder := message[idx+4:] // Skip " at "
							if _, err := time.Parse("02/01/2006 15:04:05", strings.TrimSpace(remainder)); err == nil {
								message = strings.TrimSpace(message[:idx])
							}
						}
						
						timeStr := logElement.OccurredAt.Format("02-01-2006 15:04:05")
						category := logElement.Category
						
						// Add visual separator before retry attempts
						if strings.Contains(message, "Retry (attempt") {
							fmt.Fprintln(out, "                  "+output.Yellow("------ Retrying Previous Step ------"))
							lastWasRetry = true
						} else if lastWasRetry && strings.Contains(message, "Starting") {
							lastWasRetry = false
						}
						
						// Color the category and message based on severity
						logLine := fmt.Sprintf("                  %-19s      %-8s %s", timeStr, category, message)
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

			// Mark this child as completed
			logState.completedChildIds[child.ID] = true
		}
	}
}
