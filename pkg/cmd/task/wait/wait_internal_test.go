package wait

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveTaskIDs_UsesArgumentsAndDoesNotReadStdin(t *testing.T) {
	pipeRead := false
	readFromPipe := func() []string {
		pipeRead = true
		return []string{"ServerTasks-99"}
	}

	result := resolveTaskIDs([]string{"ServerTasks-1", "ServerTasks-2"}, readFromPipe)

	assert.Equal(t, []string{"ServerTasks-1", "ServerTasks-2"}, result)
	// reading an open pipe blocks until the writer closes it, which hangs
	// callers that shell out while holding stdin open
	assert.False(t, pipeRead, "stdin must not be read when task IDs were supplied")
}

func TestResolveTaskIDs_FallsBackToStdinWhenNoArguments(t *testing.T) {
	readFromPipe := func() []string { return []string{"ServerTasks-1", "ServerTasks-2"} }

	result := resolveTaskIDs([]string{}, readFromPipe)

	assert.Equal(t, []string{"ServerTasks-1", "ServerTasks-2"}, result)
}

func TestResolveTaskIDs_NoArgumentsAndNothingPiped(t *testing.T) {
	result := resolveTaskIDs([]string{}, func() []string { return nil })

	assert.Empty(t, result)
}
