package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotEquals(t *testing.T) {
	testStrings := []string{"foo", "bar", "quxx"}
	errorMessage := "this is an error"
	notEqualsValidator := NotEquals(testStrings, errorMessage)

	assert.NotNil(t, notEqualsValidator)

	for _, v := range testStrings {
		err := notEqualsValidator(v)
		assert.Error(t, err)
		assert.Equal(t, err.Error(), errorMessage)
	}

	test := "xyzzy"
	err := notEqualsValidator(test)
	assert.NoError(t, err)
}
