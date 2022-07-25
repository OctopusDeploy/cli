package executor

type Task struct {
	CommandType string
	Inputs      map[string]any
}

func NewTask(commandType string, inputs map[string]any) Task {
	return Task{
		CommandType: commandType,
		Inputs:      inputs,
	}
}
