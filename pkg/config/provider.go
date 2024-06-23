package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type IConfigProvider interface {
	Get(key string) string
	Set(key string, value string) error
}

type FileConfigProvider struct {
	viper *viper.Viper
	IConfigProvider
}

func New(viper *viper.Viper) IConfigProvider {
	return &FileConfigProvider{
		viper: viper,
	}
}

func (accessToken *FileConfigProvider) Get(key string) string {
	return viper.GetString(key)
}

func (accessToken *FileConfigProvider) Set(key string, value string) error {
	// have to make new viper so it only contains file value, no ENVs or Flags
	configPath, err := EnsureConfigPath()
	if err != nil {
		return err
	}

	localViper := viper.New()
	SetupConfigFile(localViper, configPath)

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
	if key != "" && !IsValidKey(key) {
		return fmt.Errorf("the key '%s' is not a valid", key)
	}
	key = strings.ToLower(key)
	localViper.Set(key, value)
	if err := localViper.WriteConfig(); err != nil {
		return err
	}
	return nil
}
