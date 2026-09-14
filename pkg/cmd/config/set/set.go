package set

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/config"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmdSet(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set will write the value for given key to Octopus CLI config file",
		Long:  "Set will write the value for given key to Octopus CLI config file.",
		RunE: func(_ *cobra.Command, args []string) error {
			key := ""
			value := ""
			if len(args) > 0 {
				key = args[0]
				if len(args) > 1 {
					value = args[1]
				}
			}
			return setRun(f.IsPromptEnabled(), f.Ask, key, value)

		},
	}
	return cmd
}

func setRun(isPromptEnabled bool, ask question.Asker, key string, value string) error {
	// have to make new viper so it only contains file value, no ENVs or Flags
	configPath, err := config.EnsureConfigPath()
	if err != nil {
		return err
	}

	localViper := viper.New()
	config.SetupConfigFile(localViper, configPath)

	if err := localViper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// config file not found, we create it here and recover
			if err = localViper.SafeWriteConfig(); err != nil {
				return err
			}
		} else {
			return err // any other error is unrecoverable; abort
		}
	}
	if key != "" && !config.IsValidKey(key) {
		return fmt.Errorf("the key '%s' is not a valid", key)
	}
	if isPromptEnabled && value == "" {
		k, v, err := promptMissing(ask, key)
		if err != nil {
			return err
		}
		value = v
		key = k
	}
	key = strings.ToLower(key)
	// IgnoreSslErrors turns off certificate verification, so a value that is not
	// plainly true or false must be rejected rather than quietly stored: viper would
	// read 'yes' back as false, and a user who thought they had turned verification
	// off is better served by an error than by a setting that does nothing
	if boolKey := boolConfigKey(key); boolKey != "" {
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("the provided value %s is not valid for %s, please use true of false", value, boolKey)
		}
		localViper.Set(key, boolValue)
	} else {
		localViper.Set(key, value)
	}
	if err := localViper.WriteConfig(); err != nil {
		return err
	}
	return nil
}

// boolConfigKey returns the display name of the config key if it only accepts a
// boolean value, and an empty string otherwise.
func boolConfigKey(lowercaseKey string) string {
	for _, key := range []string{constants.ConfigNoPrompt, constants.ConfigIgnoreSslErrors} {
		if lowercaseKey == strings.ToLower(key) {
			return key
		}
	}
	return ""
}

func promptMissing(ask question.Asker, key string) (string, string, error) {
	keys := []string{
		constants.ConfigApiKey,
		constants.ConfigSpace,
		constants.ConfigNoPrompt,
		constants.ConfigUrl,
		constants.ConfigOutputFormat,
		constants.ConfigShowOctopus,
		constants.ConfigEditor,
		constants.ConfigProxyUrl,
		constants.ConfigIgnoreSslErrors,
	}

	if key == "" {
		var k string
		if err := ask(&survey.Select{
			Options: keys,
			Message: "What key you would you would like to change?",
		}, &k); err != nil {
			return "", "", err
		}
		key = k
	}

	if !config.IsValidKey(key) {
		return "", "", fmt.Errorf("unable to get value for key: %s", key)
	}

	var value string
	if err := ask(&survey.Input{
		Message: fmt.Sprintf("Enter the new value for %s", key),
	}, &value); err != nil {
		return "", "", err
	}
	value = strings.TrimSpace(value)

	return key, value, nil
}
