package executor

const CreateAccountTaskType = "createAccount"

type Task struct {
	// which type of task this is. Use the XyzTaskType set of constant strings
	Type string

	// task-specific payload (usually a struct containing the data required for this task)
	Options any
}

func NewTask(taskType string, options any) Task {
	return Task{
		Type:    taskType,
		Options: options,
	}
}
