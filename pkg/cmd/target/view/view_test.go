package view

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTargetTypeDisplayName(t *testing.T) {
	assert.Equal(t, "Listening Tentacle", getTargetTypeDisplayName("TentaclePassive"))
	assert.Equal(t, "Step Package", getTargetTypeDisplayName("StepPackage"))
	assert.Equal(t, "SomeFutureTargetType", getTargetTypeDisplayName("SomeFutureTargetType"))
	assert.Equal(t, "Unknown", getTargetTypeDisplayName(""))
}
