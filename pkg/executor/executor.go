package executor

import (
	"fmt"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
)

// task type definitions
type TaskType string

const (
	TaskTypeCreateAccount = TaskType("CreateAccount")
	TaskTypeCreateRelease = TaskType("CreateRelease")
	TaskTypeDeployRelease = TaskType("DeployRelease")
	TaskTypeRunbookRun    = TaskType("RunbookRun")
)

type Task struct {
	// which type of task this is.
	Type TaskType

	// task-specific payload (usually a struct containing the data required for this task)
	// rememmber pass this as a pointer
	Options any
}

func NewTask(taskType TaskType, options any) *Task {
	return &Task{
		Type:    taskType,
		Options: options,
	}
}

// ProcessTasks iterates over the list of tasks and attempts to run them all.
// If everything goes well, a nil error will be returned.
// On the first failure, the error will be returned and the process will halt.
// TODO some kind of progress/results callback? A Goroutine with channels?
func ProcessTasks(octopus *client.Client, space *spaces.Space, tasks []*Task) error {
	for _, task := range tasks {
		switch task.Type {
		case TaskTypeCreateAccount:
			if err := accountCreate(octopus, space, task.Options); err != nil {
				return err
			}
		case TaskTypeCreateRelease:
			if err := releaseCreate(octopus, space, task.Options); err != nil {
				return err
			}
		case TaskTypeDeployRelease:
			if err := releaseDeploy(octopus, space, task.Options); err != nil {
				return err
			}
		case TaskTypeRunbookRun:
			if err := runbookRun(octopus, space, task.Options); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unhandled task CommandType %s", task.Type)
		}
	}
	return nil
}
