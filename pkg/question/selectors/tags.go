package selectors

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tagsets"
)

// ValidateTags validates that the provided tags follow tag set rules.
// Returns an error if validation fails.
func ValidateTags(tags []string, tagSets []*tagsets.TagSet) error {
	for _, tagSet := range tagSets {
		if tagSet.Type == tagsets.TagSetTypeSingleSelect {
			// Validate that single-select tag sets only have one tag specified
			count := 0
			var matchedTags []string
			for _, tag := range tags {
				for _, t := range tagSet.Tags {
					if strings.EqualFold(t.CanonicalTagName, tag) {
						count++
						matchedTags = append(matchedTags, tag)
					}
				}
			}
			if count > 1 {
				return fmt.Errorf("only one tag can be specified for single-select tag set '%s', but found: %v", tagSet.Name, matchedTags)
			}
		} else if tagSet.Type == tagsets.TagSetTypeFreeText {
			// For FreeText, validate that only one tag is specified per tag set
			tagSetPrefix := strings.ToLower(tagSet.Name) + "/"
			count := 0
			var matchedTags []string
			for _, tag := range tags {
				if strings.HasPrefix(strings.ToLower(tag), tagSetPrefix) {
					// Validate that the tag has content after the prefix
					if len(tag) <= len(tagSetPrefix) {
						return fmt.Errorf("free text tag '%s' for tag set '%s' must have a value after the prefix", tag, tagSet.Name)
					}
					count++
					matchedTags = append(matchedTags, tag)
				}
			}
			if count > 1 {
				return fmt.Errorf("only one tag can be specified for free text tag set '%s', but found: %v", tagSet.Name, matchedTags)
			}
		}
	}
	return nil
}

// Tags prompts the user to select tags from the provided tag sets.
// For single-select tag sets, a "(None)" option is provided to make selection optional.
// For multi-select tag sets, users can select zero or more tags.
// For free-text tag sets, users can enter custom text which will be prefixed with the tag set name.
//
// Returns the selected canonical tag names.
func Tags(ask question.Asker, currentTags []string, newTags []string, tagSets []*tagsets.TagSet) ([]string, error) {
	// If tags were provided via command line, validate and use those
	if len(newTags) > 0 {
		if err := ValidateTags(newTags, tagSets); err != nil {
			return nil, err
		}
		return newTags, nil
	}

	var selectedTags []string

	// Prompt for each tag set
	for _, tagSet := range tagSets {
		// Find current value(s) for this tag set
		var currentValue string
		var currentValues []string
		tagSetPrefix := strings.ToLower(tagSet.Name) + "/"
		for _, v := range currentTags {
			// For FreeText, match by prefix; for others, match against tag list
			if tagSet.Type == tagsets.TagSetTypeFreeText {
				if strings.HasPrefix(strings.ToLower(v), tagSetPrefix) {
					currentValue = v
					currentValues = append(currentValues, v)
				}
			} else {
				for _, tag := range tagSet.Tags {
					if strings.EqualFold(tag.CanonicalTagName, v) {
						currentValue = v
						currentValues = append(currentValues, v)
					}
				}
			}
		}

		if tagSet.Type == tagsets.TagSetTypeFreeText {
			// Free text input
			var input string
			message := tagSet.Name + " (Free Text)"
			help := fmt.Sprintf("Enter a free text value for %s tag set", tagSet.Name)

			// If there's a current value, indicate it in the message and help
			if currentValue != "" && strings.HasPrefix(strings.ToLower(currentValue), tagSetPrefix) {
				currentText := currentValue[len(tagSetPrefix):]
				message = fmt.Sprintf("%s [current: %s]", message, currentText)
				help = fmt.Sprintf("Current value: %s. Enter new value or leave empty to remove", currentText)
			}

			inputPrompt := &survey.Input{
				Message: message,
				Help:    help,
			}

			err := ask(inputPrompt, &input)
			if err != nil {
				return nil, err
			}
			// Only add if not empty - empty input removes the tag
			if strings.TrimSpace(input) != "" {
				canonicalTag := tagSet.Name + "/" + strings.TrimSpace(input)
				selectedTags = append(selectedTags, canonicalTag)
			}
		} else if tagSet.Type == tagsets.TagSetTypeSingleSelect {
			// Skip empty tag sets for single/multi select
			if len(tagSet.Tags) == 0 {
				continue
			}

			var tagOptions []string
			for _, tag := range tagSet.Tags {
				tagOptions = append(tagOptions, tag.CanonicalTagName)
			}
			// Single select
			optionsWithNone := append([]string{"(None)"}, tagOptions...)
			var selected string
			selectPrompt := &survey.Select{
				Options: optionsWithNone,
				Message: tagSet.Name + " (Single Select)",
			}
			if currentValue != "" {
				selectPrompt.Default = currentValue
			} else {
				selectPrompt.Default = "(None)"
			}
			err := ask(selectPrompt, &selected)
			if err != nil {
				return nil, err
			}
			// Only add if not "None"
			if selected != "" && selected != "(None)" {
				selectedTags = append(selectedTags, selected)
			}
		} else {
			// Multi select
			// Skip empty tag sets for multi select
			if len(tagSet.Tags) == 0 {
				continue
			}

			var tagOptions []string
			for _, tag := range tagSet.Tags {
				tagOptions = append(tagOptions, tag.CanonicalTagName)
			}

			var selected []string
			defaultValues := currentValues
			if defaultValues == nil {
				defaultValues = []string{}
			}
			err := ask(&survey.MultiSelect{
				Options: tagOptions,
				Message: tagSet.Name,
				Default: defaultValues,
			}, &selected)
			if err != nil {
				return nil, err
			}
			selectedTags = append(selectedTags, selected...)
		}
	}

	return selectedTags, nil
}
