package question

import "github.com/AlecAivazis/survey/v2"

func MultiSelect[T any](message string, items []T, getKey func(item T) string, selected *[]T) error {
	optionMap, options := makeItemMapAndOptions(items, getKey)
	var selectedKeys []string
	if err := AskOne(&survey.MultiSelect{
		Message: message,
		Options: options,
	}, &selectedKeys); err != nil {
		return err
	}
	for _, keyName := range selectedKeys {
		*selected = append(*selected, optionMap[keyName])
	}
	return nil
}

func Select[T any](message string, items []T, getKey func(item T) string) (T, error) {
	optionMap, options := makeItemMapAndOptions(items, getKey)
	var selectedValue T
	var selectedKey string
	if err := AskOne(&survey.Select{
		Message: message,
		Options: options,
	}, &selectedKey); err != nil {
		return selectedValue, err
	}
	selectedValue = optionMap[selectedKey]
	return selectedValue, nil
}

func makeItemMapAndOptions[T any](items []T, getKey func(item T) string) (map[string]T, []string) {
	optionMap := make(map[string]T, len(items))
	options := make([]string, 0, len(items))
	for _, item := range items {
		key := getKey(item)
		optionMap[key] = item
		options = append(options, key)
	}
	return optionMap, options
}
