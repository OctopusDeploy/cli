package get

import (
	"fmt"
	"io"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/config"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmdGet(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [key]",
		Short: "Gets the value of config key for Octopus CLI",
		Long:  "Gets the value of config key for Octopus CLI.",
		RunE: func(cmd *cobra.Command, args []string) error {
			key := ""
			if len(args) > 0 {
				key = args[0]
			}
			return getRun(f.IsPromptEnabled(), f.Ask, key, cmd.OutOrStdout())

		},
	}
	return cmd
}

func getRun(isPromptEnabled bool, ask question.Asker, key string, out io.Writer) error {
	if key != "" {
		if !config.IsValidKey(key) {
			return fmt.Errorf("the key '%s' is not a valid", key)
		}
	}
	value := ""
	configFile := viper.New()
	configFile.SetConfigFile(viper.ConfigFileUsed())
	configFile.ReadInConfig()
	if isPromptEnabled && key == "" {
		k, err := promptMissing(ask)
		if err != nil {
			return err
		}
		key = k
	}
	value = configFile.GetString(key)
	if value == "" && !configFile.InConfig(key) {
		return fmt.Errorf("unable to get value for key: %s", key)
	}

	// a proxy url can carry a password, and this output routinely ends up in a
	// terminal recording or a support ticket. 'config list' redacts it the same way
	if strings.EqualFold(key, constants.ConfigProxyUrl) {
		value = apiclient.RedactProxyUrl(value)
	}

	fmt.Fprintln(out, value)
	return nil
}

func promptMissing(ask question.Asker) (string, error) {
	keys := []string{
		constants.ConfigApiKey,
		constants.ConfigSpace,
		constants.ConfigNoPrompt,
		constants.ConfigUrl,
		constants.ConfigOutputFormat,
		constants.ConfigShowOctopus,
		constants.ConfigEditor,
		constants.ConfigProxyUrl,
	}

	var selectKey string
	if err := ask(&survey.Select{
		Options: keys,
		Message: "What key you would you would like to see the value of?",
	}, &selectKey); err != nil {
		return "", err
	}
	return selectKey, nil
}
