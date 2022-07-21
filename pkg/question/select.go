package question

import "github.com/AlecAivazis/survey/v2"

func MultiSelect[T any](message string, items []T, getKey func(item T) string, selected *[]T) error {
	optionMap := make(map[string]T, len(items))
	options := make([]string, 0, len(items))
	for _, item := range items {
		key := getKey(item)
		optionMap[key] = item
		options = append(options, key)
	}
	var selectedKeys []string
	if err := survey.AskOne(&survey.MultiSelect{
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
