package executor

import (
	"errors"
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/factory"
)

// task type definitions
type TaskType string

const (
	TaskTypeCreateAccount = TaskType("CreateAccount")
	TaskTypeCreateRelease = TaskType("CreateRelease")
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
// On the first failure, the error will be returned.
// TODO some kind of progress/results callback? A Goroutine with channels?
func ProcessTasks(f factory.Factory, tasks []*Task) error {
	for _, task := range tasks {
		switch task.Type {
		case TaskTypeCreateAccount:
			if err := accountCreate(f, task.Options); err != nil {
				return err
			}
		case TaskTypeCreateRelease:
			if err := releaseCreate(f, task.Options); err != nil {
				return err
			}
		default:
			return errors.New(fmt.Sprintf("Unhandled task CommandType %s", task.Type))
		}
	}
	return nil
}
