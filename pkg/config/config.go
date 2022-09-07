package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/spf13/viper"
)

const configName = "cli_config"
const defaultConfigFileType = "json"
const appData = "AppData"

// var configPath string

func Setup() {
	viper.SetDefault(constants.ConfigHost, "")
	viper.SetDefault(constants.ConfigApiKey, "")
	viper.SetDefault(constants.ConfigSpace, "")
	viper.SetDefault(constants.ConfigNoPrompt, false)
	//	viper.SetDefault(constants.ConfigProxyUrl, "")
	//	viper.SetDefault(constants.ConfigShowOctopus, true)
	viper.SetDefault(constants.ConfigOutputFormat, "table")

	viper.SetConfigName(configName)
	viper.SetConfigType(defaultConfigFileType)

	if runtime.GOOS == "windows" {
		viper.SetDefault(constants.ConfigEditor, "notepad")
	} else { // unix
		viper.SetDefault(constants.ConfigEditor, "nano")
	}
	// used to set the config path in viper
	_, _ = getConfigPath()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Do nothing, config file will be created on `config set` cmd
			// This is to avoid issues with CI tools that may not have access
			// to the file system
		} else {
			// Config file was found but something is wrong
			fmt.Println("Error reading config file: %w", err)
		}
	}
}

func SetupEnv() {
	viper.BindEnv(constants.ConfigApiKey, constants.EnvOctopusApiKey)
	viper.BindEnv(constants.ConfigHost, constants.EnvOctopusHost)
	viper.BindEnv(constants.ConfigSpace, constants.EnvOctopusSpace)
	// Envs will take precedence in the specified order
	viper.BindEnv(constants.ConfigEditor, constants.EnvVisual, constants.EnvEditor)
	viper.BindEnv(constants.ConfigNoPrompt, constants.EnvCI)
}

// CreateNewConfig will create a config file and read it if it does not
// yet exist
func CreateNewConfig() error {
	if err := writeNewConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileAlreadyExistsError); ok {
			return nil
		}
		return fmt.Errorf("error writing config file: %w", err)
	}
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return fmt.Errorf("error could not find config file after create: %w", err)
		} else {
			// Config file was found but something is wrong
			return fmt.Errorf("error reading config file after create: %w", err)
		}
	}
	return nil
}

func getConfigPath() (string, error) {
	if runtime.GOOS == "windows" {
		if appdataPath := os.Getenv(appData); appdataPath != "" {
			configPath := filepath.Join(appdataPath, "octopus")
			viper.AddConfigPath(configPath)
			return configPath, nil
		} else {
			return "", fmt.Errorf("error could not find path to appdata")
		}
	}
	// is unix
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("error could not find user home directory: %w", err)
	}
	configPath := filepath.Join(home, ".config", "octopus")
	viper.AddConfigPath(configPath)
	return configPath, nil
}

func writeNewConfig() error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(configPath, os.ModePerm); err != nil {
		return err
	}
	return viper.SafeWriteConfigAs(filepath.Join(configPath, fmt.Sprintf("%s.%s", configName, defaultConfigFileType)))
}

func ValidateKey(key string) bool {
	key = strings.TrimSpace(key)
	key = strings.ToLower(key)
	return util.SliceContains(viper.AllKeys(), key)
}
