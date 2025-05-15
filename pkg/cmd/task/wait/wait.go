package wait

import (
	"fmt"
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
	FlagTimeout         = "timeout"
	FlagPollInterval    = "poll-interval"
	FlagCancelOnTimeout = "cancel-on-timeout"
	FlagProgress        = "progress"
	DefaultTimeout      = 600
	DefaultPollInterval = 10
)

type WaitOptions struct {
	*cmd.Dependencies
	TaskIDs                   []string
	GetServerTasksCallback    ServerTasksCallback
	GetTaskDetailsCallback    TaskDetailsCallback
	CancelServerTasksCallback ServerTasksCallback
	Timeout                   int
	PollInterval              int
	CancelOnTimeout           bool
	ShowProgress              bool
}

type ServerTasksCallback func([]string) ([]*tasks.Task, error)
type TaskDetailsCallback func(string) (*tasks.TaskDetailsResource, error)

func NewWaitOps(dependencies *cmd.Dependencies, taskIDs []string, timeout int, pollInterval int, cancelOnTimeout bool, showProgress bool) *WaitOptions {
	return &WaitOptions{
		Dependencies:              dependencies,
		TaskIDs:                   taskIDs,
		GetServerTasksCallback:    getServerTasksCallback(dependencies.Client),
		GetTaskDetailsCallback:    getTaskDetailsCallback(dependencies.Client),
		CancelServerTasksCallback: cancelServerTasksCallback(dependencies.Client),
		Timeout:                   timeout,
		PollInterval:              pollInterval,
		CancelOnTimeout:           cancelOnTimeout,
		ShowProgress:              showProgress,
	}
}

func NewCmdWait(f factory.Factory) *cobra.Command {
	var timeout int
	var pollInterval int
	var cancelOnTimeout bool
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
			opts := NewWaitOps(dependencies, taskIDs, timeout, pollInterval, cancelOnTimeout, showProgress)

			return WaitRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.IntVar(&timeout, FlagTimeout, DefaultTimeout, "Time in seconds to wait for the tasks to complete")
	flags.IntVar(&pollInterval, FlagPollInterval, DefaultPollInterval, "Polling interval in seconds to check task status during wait")
	flags.BoolVar(&cancelOnTimeout, FlagCancelOnTimeout, false, "Cancel the tasks if the wait timeout is reached")
	flags.BoolVar(&showProgress, FlagProgress, false, "Show detailed progress of the tasks")

	return cmd
}

func WaitRun(opts *WaitOptions) error {
	if len(opts.TaskIDs) == 0 {
		return fmt.Errorf("no server task IDs provided, at least one is required")
	}

	if opts.ShowProgress && len(opts.TaskIDs) > 1 {
		return fmt.Errorf("--progress flag is only supported when waiting for a single task")
	}

	serverTasks, err := opts.GetServerTasksCallback(opts.TaskIDs)
	if err != nil {
		return err
	}

	if len(serverTasks) == 0 {
		return fmt.Errorf("no server tasks found")
	}

	pendingTaskIDs := make([]string, 0)
	failedTaskIDs := make([]string, 0)
	formatter := NewTaskOutputFormatter(opts.Out)

	for _, t := range serverTasks {
		if t.IsCompleted == nil || !*t.IsCompleted {
			pendingTaskIDs = append(pendingTaskIDs, t.ID)
		}
		if (t.IsCompleted != nil && *t.IsCompleted) && (t.FinishedSuccessfully != nil && !*t.FinishedSuccessfully) {
			failedTaskIDs = append(failedTaskIDs, t.ID)
		}

		formatter.PrintTaskInfo(t)
	}

	if len(pendingTaskIDs) == 0 {
		if len(failedTaskIDs) != 0 {
			return fmt.Errorf("one or more deployment tasks failed: %s", strings.Join(failedTaskIDs, ", "))
		}
		return nil
	}

	gotError := make(chan error, 1)
	done := make(chan bool, 1)
	completedChildIds := make(map[string]bool)

	go func() {
		for len(pendingTaskIDs) != 0 {
			time.Sleep(time.Duration(opts.PollInterval) * time.Second)
			serverTasks, err = opts.GetServerTasksCallback(pendingTaskIDs)
			if err != nil {
				gotError <- err
				return
			}
			for _, t := range serverTasks {
				if opts.ShowProgress {
					details, err := opts.GetTaskDetailsCallback(t.ID)
					if err != nil {
						continue // Skip progress display if we can't get details
					}

					if len(details.ActivityLogs) > 0 {
						// Process all activities
						for _, activity := range details.ActivityLogs {
							formatter.PrintActivityElement(activity, 0, completedChildIds)
						}
					}
				}

				if t.IsCompleted != nil && *t.IsCompleted {
					if t.FinishedSuccessfully != nil && !*t.FinishedSuccessfully {
						failedTaskIDs = append(failedTaskIDs, t.ID)
					}
					formatter.PrintTaskInfo(t)
					pendingTaskIDs = removeTaskID(pendingTaskIDs, t.ID)
				}
			}
		}
		if len(failedTaskIDs) != 0 {
			gotError <- fmt.Errorf("one or more deployment tasks failed: %s", strings.Join(failedTaskIDs, ", "))
			return
		}
		done <- true
	}()

	select {
	case <-done:
		return nil
	case err := <-gotError:
		return err
	case <-time.After(time.Duration(opts.Timeout) * time.Second):
		if opts.CancelOnTimeout {
			fmt.Fprintf(opts.Dependencies.Out, "Cancelling remaining tasks: %s\n", strings.Join(pendingTaskIDs, ", "))
			_, err := opts.CancelServerTasksCallback(pendingTaskIDs)
			if err != nil {
				return err
			}
		}
		return fmt.Errorf("timeout while waiting for pending tasks")
	}
}

func getServerTasksCallback(octopus *client.Client) ServerTasksCallback {
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

func getTaskDetailsCallback(octopus *client.Client) TaskDetailsCallback {
	return func(taskID string) (*tasks.TaskDetailsResource, error) {
		return tasks.GetDetails(octopus, octopus.GetSpaceID(), taskID)
	}
}

func cancelServerTasksCallback(octopus *client.Client) ServerTasksCallback {
	return func(taskIDs []string) ([]*tasks.Task, error) {
		serverTasks := make([]*tasks.Task, len(taskIDs))
		for _, taskID := range taskIDs {
			serverTask, err := tasks.Cancel(octopus, octopus.GetSpaceID(), taskID)
			if err != nil {
				return nil, err
			}
			serverTasks = append(serverTasks, serverTask)
		}
		return serverTasks, nil
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
