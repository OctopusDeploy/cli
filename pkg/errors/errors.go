package errors

import "fmt"

type OsEnvironmentError struct {
	EnvironmentVariable string
}

func (e *OsEnvironmentError) Error() string {
	return fmt.Sprintf("%s environment variable is missing or blank", e.EnvironmentVariable)
}
