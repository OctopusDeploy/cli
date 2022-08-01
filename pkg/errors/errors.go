package errors

import "fmt"

// OsEnvironmentError is raised when the CLI cannot launch because a required environment variable is not set
type OsEnvironmentError struct {
	EnvironmentVariable string
}

func (e *OsEnvironmentError) Error() string {
	return fmt.Sprintf("%s environment variable is missing or blank", e.EnvironmentVariable)
}

// PromptDisabledError is a fallback error if code attempts to prompt the user when prompting is disabled.
// If you see it, it represents a bug because Commands should check IsPromptEnabled before attempting to prompt
type PromptDisabledError struct {
}

func (e *PromptDisabledError) Error() string {
	return fmt.Sprintf("prompt disabled")
}