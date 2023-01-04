package selectors

import (
	"fmt"

	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/accounts"
)

type SelectOption[T any] struct {
	Value   T
	Display string
}

type Nameable interface {
	GetName() string
}

func ByName[T Nameable](ask question.Asker, list []T, message string) (T, error) {
	var selectedItem T
	selectedItem, err := question.SelectMap(ask, message, list, func(item T) string {
		return item.GetName()
	})
	if err != nil {
		return selectedItem, err
	}
	return selectedItem, nil
}

func Account(ask question.Asker, list []accounts.IAccount, message string) (accounts.IAccount, error) {
	var selectedItem accounts.IAccount
	selectedItem, err := question.SelectMap(ask, message, list, func(item accounts.IAccount) string {
		return fmt.Sprintf("%s %s", item.GetName(), output.Dimf("(%s)", item.GetID()))
	})
	if err != nil {
		return selectedItem, err
	}
	return selectedItem, nil
}

func SelectOptions[T any](ask question.Asker, questionText string, itemsCallback func() []*SelectOption[T]) (*SelectOption[T], error) {
	items := itemsCallback()
	callback := func() ([]*SelectOption[T], error) {
		return items, nil
	}
	return Select(ask, questionText, callback, func(option *SelectOption[T]) string { return option.Display })
}

func Select[T any](ask question.Asker, questionText string, itemsCallback func() ([]T, error), getKey func(item T) string) (T, error) {
	items, err := itemsCallback()
	if err != nil {
		var item T
		return item, err
	}
	if len(items) == 1 {
		return items[0], nil
	}

	return question.SelectMap(ask, questionText, items, getKey)
}

// SelectOrNew is the same as Select but show a create new option at the bottom of the list
// When create new is selected the returned bool will be true
func SelectOrNew[T any](ask question.Asker, questionText string, itemsCallback func() ([]T, error), getKey func(item T) string) (T, bool, error) {
	items, err := itemsCallback()
	if err != nil {
		var item T
		return item, false, err
	}
	return question.SelectMapWithNew(ask, questionText, items, getKey)
}
