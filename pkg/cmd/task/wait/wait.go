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
	processedLogs      map[string]bool
	lastActivityStatus map[string]string // tracks last known status for each activity
	headerPrinted      bool
}

func NewLogState() *LogState {
	return &LogState{
		processedLogs:      make(map[string]bool),
		lastActivityStatus: make(map[string]string),
		headerPrinted:      false,
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
		fmt.Fprintf(out, "%s: %s\n", t.Description, t.State)
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
				if t.IsCompleted != nil && *t.IsCompleted {
					if t.FinishedSuccessfully != nil && !*t.FinishedSuccessfully {
						failedTaskIDs = append(failedTaskIDs, t.ID)
					}
					fmt.Fprintf(out, "%s: %s\n", t.Description, t.State)
					pendingTaskIDs = removeTaskID(pendingTaskIDs, t.ID)
				} else if showProgress {
					details, err := getTaskDetailsCallback(t.ID)
					if err != nil {
						continue // Skip progress display if we can't get details
					}

					if len(details.ActivityLogs) > 0 {
						if !logState.headerPrinted {
							fmt.Fprintf(out, "%s\n", t.Description)
							logState.headerPrinted = true
						}

						for _, activity := range details.ActivityLogs {
							printActivityElement(out, activity, 0, logState)
						}
					}
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
	indentStr := strings.Repeat(" ", indent*9)

	// Check if status has changed
	if !logState.hasProcessed(activity, nil) {
		if activity.Status != "" {
			fmt.Fprintf(out, "%s%s: %s\n", indentStr, activity.Status, activity.Name)
		}
		logState.markProcessed(activity, nil)
	}

	// Print child activities
	for _, child := range activity.Children {
		printActivityElement(out, child, indent+1, logState)
	}

	// Print only new log elements
	for _, logElement := range activity.LogElements {
		if !logState.hasProcessed(activity, logElement) {
			fmt.Fprintf(out, "%s  %-8s %s\n", indentStr, logElement.Category, logElement.MessageText)
			logState.markProcessed(activity, logElement)
		}
	}
}
