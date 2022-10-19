package selectors

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/stretchr/testify/assert"
	"testing"
)

type Item struct {
	id   string
	name string
}

func TestSelectForSingleItem(t *testing.T) {
	itemsCallback := func() ([]*Item, error) {
		return []*Item{
			{
				id:   "1",
				name: "name",
			},
		}, nil
	}

	selectedItem, err := Select(nil, "question", itemsCallback, func(item *Item) string { return item.name })
	assert.Nil(t, err)
	assert.Equal(t, selectedItem.id, "1")
}

func TestSelectForMultipleItem(t *testing.T) {
	items := []*Item{
		{
			id:   "1",
			name: "name",
		},
		{
			id:   "2",
			name: "name 2",
		},
	}
	itemsCallback := func() ([]*Item, error) {
		return items, nil
	}
	pa := []*testutil.PA{
		{
			Prompt: &survey.Select{
				Message: "question",
				Options: []string{items[0].name, items[1].name},
			},
			Answer: "name 2",
		},
	}
	mockAsker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	selectedItem, err := Select(mockAsker, "question", itemsCallback, func(item *Item) string { return item.name })
	checkRemainingPrompts()
	assert.Nil(t, err)
	assert.Equal(t, selectedItem.id, "2")
}
