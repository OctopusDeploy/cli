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

func SetupConfigFile(v *viper.Viper, configPath string) {
	v.SetConfigName(configName)
	v.SetConfigType(defaultConfigFileType)
	v.AddConfigPath(configPath)
}

func setDefaults(v *viper.Viper) {
	v.SetDefault(constants.ConfigHost, "")
	v.SetDefault(constants.ConfigApiKey, "")
	v.SetDefault(constants.ConfigSpace, "")
	v.SetDefault(constants.ConfigNoPrompt, false)
	//	v.SetDefault(constants.ConfigProxyUrl, "")
	//	v.SetDefault(constants.ConfigShowOctopus, true)
	v.SetDefault(constants.ConfigOutputFormat, "table")

	if runtime.GOOS == "windows" {
		v.SetDefault(constants.ConfigEditor, "notepad")
	} else { // unix
		v.SetDefault(constants.ConfigEditor, "nano")
	}
}

func Setup() error {
	// we use the global static viper through all the CLI, EXCEPT when writing config files, which is done in pkg/cmd/set, not here
	setDefaults(viper.GetViper())

	// bind environment variables
	if err := viper.BindEnv(constants.ConfigApiKey, constants.EnvOctopusApiKey); err != nil {
		return err
	}
	if err := viper.BindEnv(constants.ConfigHost, constants.EnvOctopusHost); err != nil {
		return err
	}
	if err := viper.BindEnv(constants.ConfigSpace, constants.EnvOctopusSpace); err != nil {
		return err
	}
	// Envs will take precedence in the specified order
	if err := viper.BindEnv(constants.ConfigEditor, constants.EnvVisual, constants.EnvEditor); err != nil {
		return err
	}
	if err := viper.BindEnv(constants.ConfigNoPrompt, constants.EnvCI); err != nil {
		return err
	}

	// read the config file
	configPath, err := getConfigPath()
	if err == nil {
		SetupConfigFile(viper.GetViper(), configPath)

		if err := viper.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				// Do nothing, config file will be created on `config set` cmd
				// This is to avoid issues with CI tools that may not have access
				// to the file system
			} else {
				// Config file was found but something is wrong
				// we can recover and run with no config
				fmt.Println("Error reading config file: %w", err)
			}
		}
	}
	// if we can't get the configPath, then everything will just be defaulted
	return nil
}

// EnsureConfigPath works out the config path, then creates the directory to make sure that it exists
func EnsureConfigPath() (string, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return "", err
	}
	err = os.MkdirAll(configPath, os.ModePerm)
	if err != nil {
		return "", err
	}
	return configPath, nil
}

// getConfigPath works out the directory where the config file should be saved and returns it.
// does not modify the global viper
func getConfigPath() (string, error) {
	if runtime.GOOS == "windows" {
		if appdataPath := os.Getenv(appData); appdataPath != "" {
			configPath := filepath.Join(appdataPath, "octopus")
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
	return configPath, nil
}

func ValidateKey(key string) bool {
	key = strings.TrimSpace(key)
	key = strings.ToLower(key)
	return util.SliceContains(viper.AllKeys(), key)
}
