package validation

import (
	"fmt"
	"reflect"

	"github.com/AlecAivazis/survey/v2"
	uuid "github.com/google/uuid"
)

// NotEquals requires that the string does not equal any of the specified values
func NotEquals(stringsToCheck []string, errorMessage string) survey.Validator {
	// return a validator to perform the check
	return func(val interface{}) error {
		if str, ok := val.(string); ok {
			for _, v := range stringsToCheck {
				if str == v {
					return fmt.Errorf("%s", errorMessage)
				}
			}
		} else {
			// otherwise we cannot convert the value into a string and cannot perform check
			return fmt.Errorf("cannot check value on response of type %v", reflect.TypeOf(val).Name())
		}

		// the input is fine
		return nil
	}
}

// IsUuid requires that the string is a valid UUID
func IsUuid(val interface{}) error {
	if str, ok := val.(string); ok {
		if _, err := uuid.Parse(str); err != nil {
			return fmt.Errorf("not a valid UUID")
		}
	} else {
		// otherwise we cannot convert the value into a string and cannot perform check
		return fmt.Errorf("cannot check value on response of type %v", reflect.TypeOf(val).Name())
	}

	// the input is fine
	return nil
}
