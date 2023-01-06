package question

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/util"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
)

func MultiSelectMap[T any](ask Asker, message string, items []T, getKey func(item T) string, required bool) ([]T, error) {
	if util.Empty(items) {
		return nil, fmt.Errorf("%s - no options available", message)
	}
	optionMap, options := MakeItemMapAndOptions(items, getKey)

	askOpts := func(options *survey.AskOptions) error { return nil }
	if required {
		askOpts = survey.WithValidator(survey.Required)
	}

	var selectedKeys []string
	if err := ask(&survey.MultiSelect{Message: message, Options: options}, &selectedKeys, askOpts); err != nil {
		return nil, err
	}
	selected := make([]T, 0)
	for _, keyName := range selectedKeys {
		selected = append(selected, optionMap[keyName])
	}
	// it's valid to have zero selected options in a multi-select
	return selected, nil
}

func MultiSelectWithAddMap(ask Asker, message string, items []string, required bool) ([]string, error) {
	askOpts := func(options *survey.AskOptions) error { return nil }
	if required {
		askOpts = survey.WithValidator(survey.Required)
	}

	var selectedKeys []string
	if err := ask(&surveyext.MultiSelectWithAdd{Message: message, Options: items}, &selectedKeys, askOpts); err != nil {
		return nil, err
	}
	return selectedKeys, nil
}

func SelectMap[T any](ask Asker, message string, items []T, getKey func(item T) string) (T, error) {
	if util.Empty(items) {
		return *new(T), fmt.Errorf("%s - no options available", message)
	}
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

func SelectMapWithNew[T any](ask Asker, message string, items []T, getKey func(item T) string) (T, bool, error) {
	optionMap, options := MakeItemMapAndOptions(items, getKey)
	var selectedValue T
	var selectedKey string
	if err := ask(&surveyext.Select{
		Message: message,
		Options: options,
	}, &selectedKey); err != nil {
		return selectedValue, false, err
	}
	if selectedKey == constants.PromptCreateNew {
		return *new(T), true, nil
	}
	selectedValue, ok := optionMap[selectedKey]
	if !ok { // without this explict check SelectMap can return nil, nil which people don't expect
		return *new(T), false, fmt.Errorf("SelectMap did not get valid answer (selectedKey=%s)", selectedKey)
	}
	return selectedValue, false, nil
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
