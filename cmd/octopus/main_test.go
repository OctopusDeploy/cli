package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandDoesNotRequireClient_CompletionCommandsDoNotRequireClient(t *testing.T) {
	cases := [][]string{
		{"completion"},
		{"completion", "powershell"},
		{"completion", "bash"},
		{"__complete", "completion", "powershell", ""},
		{"__completeNoDesc", "release", "list", ""},
	}

	for _, args := range cases {
		assert.True(t, commandDoesNotRequireClient(args), "expected %v to not require a client", args)
	}
}

func TestCommandDoesNotRequireClient_ServerCommandsRequireClient(t *testing.T) {
	cases := [][]string{
		{"release", "list"},
		{"deployment", "create"},
		{"package", "list"},
	}

	for _, args := range cases {
		assert.False(t, commandDoesNotRequireClient(args), "expected %v to require a client", args)
	}
}
