package integration

import "testing"

type CleanupHelper interface {
	// enqueue a callback to be run at cleanup time. Cleanups can't return errors as this would leave
	// the system under test in an in
	AddFailable(action func() error)
	Add(action func())
	Run(t *testing.T)
}

type cleanupHelper struct {
	actions []func() error
}

func NewCleanupHelper() CleanupHelper {
	c := &cleanupHelper{}

	return c
}

func (c *cleanupHelper) AddFailable(action func() error) {
	c.actions = append(c.actions, action)
}

func (c *cleanupHelper) Add(action func()) {
	c.actions = append(c.actions, func() error { action(); return nil })
}

func (c *cleanupHelper) Run(t *testing.T) {
	// LIFO ordering.
	for i := len(c.actions) - 1; i >= 0; i-- {
		action := c.actions[i]
		err := action()
		if err != nil {
			t.Fatalf("Abort! Error during cleanup %v", err)
		}
	}
}
