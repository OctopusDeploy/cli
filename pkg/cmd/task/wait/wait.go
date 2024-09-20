package wait

import (
	"fmt"
	"io"
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
	FlagTimeout = "timeout"

	DefaultTimeout = 600 // 600 seconds : 10 minutes
)

type WaitOptions struct {
	*cmd.Dependencies
	taskIDs                []string
	GetServerTasksCallback ServerTasksCallback
}

func NewWaitOps(dependencies *cmd.Dependencies, taskIDs []string) *WaitOptions {
	return &WaitOptions{
		Dependencies:           dependencies,
		GetServerTasksCallback: GetServerTasksCallback(dependencies.Client),
	}
}

type ServerTasksCallback func([]string) ([]*tasks.Task, error)

func NewCmdWait(f factory.Factory) *cobra.Command {
	var timeout int
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

			return WaitRun(opts.Out, taskIDs, opts.GetServerTasksCallback, timeout)
		},
	}

	flags := cmd.Flags()
	flags.IntVar(&timeout, FlagTimeout, DefaultTimeout, "Duration to wait (in seconds) before stopping execution")

	return cmd
}

func WaitRun(out io.Writer, taskIDs []string, getServerTasksCallback ServerTasksCallback, timeout int) error {
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
