package list

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmdList(_ factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List values from config file",
		Long:  "List values from config file.",
		Example: heredoc.Docf(`
			$ %s config list"
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(cmd)
		},
	}

	return cmd
}

func listRun(cmd *cobra.Command) error {
	configFile := viper.New()
	configFile.SetConfigFile(viper.ConfigFileUsed())
	configFile.ReadInConfig()

	if configFile.IsSet(constants.ConfigApiKey) {
		configFile.Set(constants.ConfigApiKey, "***")
	}

	if configFile.IsSet(constants.ConfigAccessToken) {
		configFile.Set(constants.ConfigAccessToken, "***")
	}

	type ConfigData struct {
		ApiKey       string `json:"apikey"`
		Editor       string `json:"editor"`
		Host         string `json:"host"`
		NoPrompt     string `json:"noprompt"`
		OutputFormat string `json:"outputformat"`
		Space        string `json:"space"`
	}

	outputFormat, _ := cmd.Flags().GetString(constants.FlagOutputFormat)
	if outputFormat == "" {
		outputFormat = viper.GetString(constants.ConfigOutputFormat)
	}

	switch strings.ToLower(outputFormat) {
	case constants.OutputFormatJson:
		configData := &ConfigData{}
		for _, key := range configFile.AllKeys() {
			switch strings.ToLower(key) {
			case strings.ToLower(constants.ConfigApiKey):
				configData.ApiKey = configFile.GetString(key)
			case strings.ToLower(constants.ConfigEditor):
				configData.Editor = configFile.GetString(key)
			case strings.ToLower(constants.ConfigUrl):
				configData.Host = configFile.GetString(key)
			case strings.ToLower(constants.ConfigNoPrompt):
				configData.NoPrompt = configFile.GetString(key)
			case strings.ToLower(constants.ConfigSpace):
				configData.Space = configFile.GetString(key)
			case strings.ToLower(constants.ConfigOutputFormat):
				configData.OutputFormat = configFile.GetString(key)
			default:
				return fmt.Errorf("the key '%s' is not a supported config option", key)
			}
		}
		data, _ := json.MarshalIndent(configData, "", "  ")
		cmd.Println(string(data))
	case constants.OutputFormatBasic:
		for _, key := range configFile.AllKeys() {
			cmd.Println(configFile.GetString(key))
		}
	case constants.OutputFormatTable:
		t := output.NewTable(cmd.OutOrStdout())
		t.AddRow(output.Bold("KEY"), output.Bold("VALUE"))
		for _, key := range configFile.AllKeys() {
			t.AddRow(key, configFile.GetString(key))
		}
		t.Print()
	}

	return nil
}
