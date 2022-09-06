package question

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
)

func MultiSelectMap[T any](ask Asker, message string, items []T, getKey func(item T) string, minItems int) ([]T, error) {
	optionMap, options := MakeItemMapAndOptions(items, getKey)

	var selectedKeys []string
	if err := ask(&survey.MultiSelect{
		Message: message,
		Options: options,
	}, &selectedKeys, survey.WithValidator(survey.MinItems(minItems))); err != nil {
		return nil, err
	}
	selected := make([]T, 0)
	for _, keyName := range selectedKeys {
		selected = append(selected, optionMap[keyName])
	}
	// it's valid to have zero selected options in a multi-select
	return selected, nil
}

func SelectMap[T any](ask Asker, message string, items []T, getKey func(item T) string) (T, error) {
	optionMap, options := MakeItemMapAndOptions(items, getKey)
	var selectedValue T
	var selectedKey string
	if err := ask(&survey.Select{
		Message: message,
		Options: options,
	}, &selectedKey); err != nil {
		return selectedValue, err
	}
	selectedValue, ok := optionMap[selectedKey]
	if !ok { // without this explict check SelectMap can return nil, nil which people don't expect
		return *new(T), fmt.Errorf("SelectMap did not get valid answer (selectedKey=%s)", selectedKey)
	}
	return selectedValue, nil
}

func MakeItemMapAndOptions[T any](items []T, getKey func(item T) string) (map[string]T, []string) {
	optionMap := make(map[string]T, len(items))
	options := make([]string, 0, len(items))
	for _, item := range items {
		key := getKey(item)
		optionMap[key] = item
		options = append(options, key)
	}
	return optionMap, options
}
