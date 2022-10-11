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

type NameOrID interface {
	GetName() string
	GetID() string
}

func ByNameOrID[T NameOrID](ask question.Asker, list []T, message string) (T, error) {
	var selectedItem T
	selectedItem, err := question.SelectMap(ask, message, list, func(item T) string {
		return fmt.Sprintf("%s %s", item.GetName(), output.Dimf("(%s)", item.GetID()))
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

func Select[T any](ask *question.Asker, questionText string, items []*T, getKey func(item *T) string) (*T, error) {
	if len(items) == 1 {
		return items[0], nil
	}

	return question.SelectMap(*ask, questionText, items, getKey)
}
