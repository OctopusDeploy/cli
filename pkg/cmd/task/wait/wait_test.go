package wait_test

import (
	"bytes"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	taskWaitCreate "github.com/OctopusDeploy/cli/pkg/cmd/task/wait"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tasks"
	"github.com/stretchr/testify/assert"
)

var serverUrl, _ = url.Parse("https://serverurl")
var spinner = &testutil.FakeSpinner{}
var rootResource = testutil.NewRootResource()

func TestWait(t *testing.T) {
	out := bytes.Buffer{}
	defaultTaskIDs := []string{
		"TaskID1",
		"TaskID2",
	}

	taskList := []*tasks.Task{
		tasks.NewTask(),
		tasks.NewTask(),
	}

	// bool vars as bool constants can't be used as pointers for IsCompleted
	boolFalse := false
	boolTrue := true

	taskList[0].ID = defaultTaskIDs[0]
	taskList[0].IsCompleted = &boolFalse
	taskList[0].FinishedSuccessfully = &boolFalse
	taskList[0].Description = "Deploy Bar 1 release 0.0.2 to Foo"
	taskList[0].State = "Executing"

	taskList[1].ID = defaultTaskIDs[1]
	taskList[1].IsCompleted = &boolTrue
	taskList[1].FinishedSuccessfully = &boolTrue
	taskList[1].Description = "Deploy Bar 2 release 0.0.2 to Foo"
	taskList[1].State = "Success"

	timesCalled := 0

	getServerTaskCallback := func(taskIDs []string) ([]*tasks.Task, error) {
		timesCalled += 1
		switch timesCalled {
		case 1:
			assert.Len(t, taskIDs, 2)
			assert.Equal(t, defaultTaskIDs[0], taskIDs[0])
			assert.Equal(t, defaultTaskIDs[1], taskIDs[1])
			return taskList, nil
		case 2:
			assert.Len(t, taskIDs, 1)
			assert.Equal(t, defaultTaskIDs[0], taskIDs[0])
			taskList[0].IsCompleted = &boolTrue
			taskList[0].FinishedSuccessfully = &boolTrue
			taskList[0].State = "Success"
			taskList = taskList[:len(taskList)-1]
			return taskList, nil
		}
		return nil, fmt.Errorf("getServerTaskCallback was called more then the expected amount of times")
	}

	opts := &taskWaitCreate.WaitOptions{
		Dependencies: &cmd.Dependencies{
			Out: &out,
		},
		TaskIDs:                defaultTaskIDs,
		GetServerTasksCallback: getServerTaskCallback,
		GetTaskDetailsCallback: nil,
		Timeout:                taskWaitCreate.DefaultTimeout,
		PollInterval:           1,
		ShowProgress:           false,
	}

	err := taskWaitCreate.WaitRun(opts)
	assert.NoError(t, err)
	assert.Equal(t, 2, timesCalled)
	expectedOutput := heredoc.Doc(`
  TaskID1: Deploy Bar 1 release 0.0.2 to Foo: Executing
  TaskID2: Deploy Bar 2 release 0.0.2 to Foo: Success
  TaskID1: Deploy Bar 1 release 0.0.2 to Foo: Success
  `)
	assert.Equal(t, expectedOutput, out.String())
}

func TestWait_FailedTask(t *testing.T) {
	out := bytes.Buffer{}
	defaultTaskIDs := []string{
		"TaskID1",
	}

	taskList := []*tasks.Task{
		tasks.NewTask(),
	}

	// bool vars as bool constants can't be used as pointers for IsCompleted
	boolFalse := false
	boolTrue := true

	taskList[0].ID = defaultTaskIDs[0]
	taskList[0].IsCompleted = &boolTrue
	taskList[0].FinishedSuccessfully = &boolFalse
	taskList[0].Description = "Deploy Bar 1 release 0.0.2 to Foo"
	taskList[0].State = "Failed"

	getServerTaskCallback := func(taskIDs []string) ([]*tasks.Task, error) {
		return taskList, nil
	}

	opts := &taskWaitCreate.WaitOptions{
		Dependencies: &cmd.Dependencies{
			Out: &out,
		},
		TaskIDs:                defaultTaskIDs,
		GetServerTasksCallback: getServerTaskCallback,
		GetTaskDetailsCallback: nil,
		Timeout:                taskWaitCreate.DefaultTimeout,
		PollInterval:           1,
		ShowProgress:           false,
	}

	err := taskWaitCreate.WaitRun(opts)
	assert.EqualError(t, err, "one or more deployment tasks failed: TaskID1")
	expectedOutput := heredoc.Doc(`
  TaskID1: Deploy Bar 1 release 0.0.2 to Foo: Failed
  `)
	assert.Equal(t, expectedOutput, out.String())
}

func TestWait_FailedPendingTask(t *testing.T) {
	out := bytes.Buffer{}
	defaultTaskIDs := []string{
		"TaskID1",
	}

	taskList := []*tasks.Task{
		tasks.NewTask(),
	}

	// bool vars as bool constants can't be used as pointers for IsCompleted
	boolFalse := false
	boolTrue := true

	taskList[0].ID = defaultTaskIDs[0]
	taskList[0].IsCompleted = &boolFalse
	taskList[0].FinishedSuccessfully = &boolFalse
	taskList[0].Description = "Deploy Bar 1 release 0.0.2 to Foo"
	taskList[0].State = "Executing"

	timesCalled := 0

	getServerTaskCallback := func(taskIDs []string) ([]*tasks.Task, error) {
		timesCalled += 1
		switch timesCalled {
		case 1:
			return taskList, nil
		case 2:
			taskList[0].IsCompleted = &boolTrue
			taskList[0].FinishedSuccessfully = &boolFalse
			taskList[0].State = "Failed"
			return taskList, nil
		}
		return nil, fmt.Errorf("getServerTaskCallback was called more then the expected amount of times")
	}

	opts := &taskWaitCreate.WaitOptions{
		Dependencies: &cmd.Dependencies{
			Out: &out,
		},
		TaskIDs:                defaultTaskIDs,
		GetServerTasksCallback: getServerTaskCallback,
		GetTaskDetailsCallback: nil,
		Timeout:                taskWaitCreate.DefaultTimeout,
		PollInterval:           1,
		ShowProgress:           false,
	}

	err := taskWaitCreate.WaitRun(opts)
	assert.EqualError(t, err, "one or more deployment tasks failed: TaskID1")
	assert.Equal(t, 2, timesCalled)
	expectedOutput := heredoc.Doc(`
  TaskID1: Deploy Bar 1 release 0.0.2 to Foo: Executing
  TaskID1: Deploy Bar 1 release 0.0.2 to Foo: Failed
  `)
	assert.Equal(t, expectedOutput, out.String())
}

func TestWait__CancelOnTimeout(t *testing.T) {
	out := bytes.Buffer{}
	defaultTaskIDs := []string{
		"TaskID1",
		"TaskID2",
	}

	taskList := []*tasks.Task{
		tasks.NewTask(),
		tasks.NewTask(),
	}

	// bool vars as bool constants can't be used as pointers for IsCompleted
	boolFalse := false
	boolTrue := true

	taskList[0].ID = defaultTaskIDs[0]
	taskList[0].IsCompleted = &boolFalse
	taskList[0].FinishedSuccessfully = &boolFalse
	taskList[0].Description = "Deploy Bar 1 release 0.0.2 to Foo"
	taskList[0].State = "Executing"

	taskList[1].ID = defaultTaskIDs[1]
	taskList[1].IsCompleted = &boolTrue
	taskList[1].FinishedSuccessfully = &boolTrue
	taskList[1].Description = "Deploy Bar 2 release 0.0.2 to Foo"
	taskList[1].State = "Success"

	getServerTaskCallback := func(taskIDs []string) ([]*tasks.Task, error) {
		time.Sleep(1 * time.Second)
		assert.Len(t, taskIDs, 2)
		assert.Equal(t, defaultTaskIDs[0], taskIDs[0])
		assert.Equal(t, defaultTaskIDs[1], taskIDs[1])
		return taskList, nil
	}

	cancelCalled := false
	cancelServerTaskCallback := func(taskIDs []string) ([]*tasks.Task, error) {
		cancelCalled = true
		return []*tasks.Task{}, nil
	}

	opts := &taskWaitCreate.WaitOptions{
		Dependencies: &cmd.Dependencies{
			Out: &out,
		},
		TaskIDs:                   defaultTaskIDs,
		GetServerTasksCallback:    getServerTaskCallback,
		GetTaskDetailsCallback:    nil,
		CancelServerTasksCallback: cancelServerTaskCallback,
		Timeout:                   1,
		PollInterval:              1,
		CancelOnTimeout:           true,
		ShowProgress:              false,
	}

	err := taskWaitCreate.WaitRun(opts)
	assert.True(t, cancelCalled)
	assert.EqualError(t, err, "timeout while waiting for pending tasks")
	expectedOutput := heredoc.Doc(`
  TaskID1: Deploy Bar 1 release 0.0.2 to Foo: Executing
  TaskID2: Deploy Bar 2 release 0.0.2 to Foo: Success
  Cancelling remaining tasks: TaskID1
  `)
	assert.Equal(t, expectedOutput, out.String())
}
