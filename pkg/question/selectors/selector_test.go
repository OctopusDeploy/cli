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
	items := []*Item{
		{
			id:   "1",
			name: "name",
		}}
	selectedItem, err := Select[Item](nil, "question", items, func(item *Item) string { return item.name })
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
		}}
	mocker := testutil.NewAskMocker()
	mocker.ExpectQuestion(t, &survey.Select{
		Message: "You have not specified a Lifecycle for this project. Please select one:",
		Options: []string{items[0].name, items[1].name}}).AnswerWith("name 2")
	asker := mocker.AsAsker()
	selectedItem, err := Select[Item](asker, "question", items, func(item *Item) string { return item.name })
	assert.Nil(t, err)
	assert.Equal(t, selectedItem.id, "1")
}
