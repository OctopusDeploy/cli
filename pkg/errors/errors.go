package errors

import "fmt"

// OsEnvironmentError is raised when the CLI cannot launch because a required environment variable is not set
type OsEnvironmentError struct{ EnvironmentVariable string }

func (e *OsEnvironmentError) Error() string {
	return fmt.Sprintf("%s environment variable is missing or blank", e.EnvironmentVariable)
}

// PromptDisabledError is a fallback error if code attempts to prompt the user when prompting is disabled.
// If you see it, it represents a bug because Commands should check IsInteractive before attempting to prompt
type PromptDisabledError struct{}

func (e *PromptDisabledError) Error() string {
	return "prompt disabled"
}

// ArgumentNullOrEmptyError is a utility error indicating that a required parameter was
// null or blank. This is not a recoverable error; if you observe one this indicates a bug in the code.
type ArgumentNullOrEmptyError struct{ ArgumentName string }

func (e *ArgumentNullOrEmptyError) Error() string {
	return fmt.Sprintf("argument %s was nil or empty", e.ArgumentName)
}
func NewArgumentNullOrEmptyError(argumentName string) *ArgumentNullOrEmptyError {
	return &ArgumentNullOrEmptyError{ArgumentName: argumentName}
}

// InvalidResponseError is a utility error that means the CLI couldn't deal with a response from the server.
// this may represent a bug (missing code path) in the CLI, a bug in the server (wrong response), or a change in server behaviour over time.
type InvalidResponseError struct{ Message string }

func (e *InvalidResponseError) Error() string { return e.Message }
func NewInvalidResponseError(message string) *InvalidResponseError {
	return &InvalidResponseError{Message: message}
}
