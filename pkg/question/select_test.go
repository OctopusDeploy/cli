package question_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/test/testutil"
)

func TestMultiSelectMap_NoItems(t *testing.T) {
	pa := []*testutil.PA{}
	mockAsker, _ := testutil.NewMockAsker(t, pa)
	selectedItem, err := question.MultiSelectMap(mockAsker, "question", []*string{}, func(item *string) string { return *item }, false)
	assert.Nil(t, selectedItem)
	assert.Error(t, err)
}

func TestSelectMap_NoItems(t *testing.T) {
	pa := []*testutil.PA{}
	mockAsker, _ := testutil.NewMockAsker(t, pa)
	selectedItem, err := question.SelectMap(mockAsker, "question", []*string{}, func(item *string) string { return *item })
	assert.Nil(t, selectedItem)
	assert.Error(t, err)
}
