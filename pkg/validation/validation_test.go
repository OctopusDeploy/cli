package validation

import (
	"testing"

	"github.com/google/uuid"
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

func TestIsUUID(t *testing.T) {
	isUUIDValidator := IsUuid()
	assert.NotNil(t, isUUIDValidator)

	testStrings := []string{"foo", "bar", "quxx"}
	for _, v := range testStrings {
		err := isUUIDValidator(v)
		assert.Error(t, err)
	}

	testUUID, err := uuid.NewUUID()
	assert.NoError(t, err)
	assert.NotNil(t, testUUID)

	testUUIDString := testUUID.String()
	err = isUUIDValidator(testUUIDString)
	assert.NoError(t, err)
}
