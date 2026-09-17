package config_test

import (
	"testing"

	"github.com/OctopusDeploy/cli/pkg/config"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestSetup_BindsTheProxyEnvironmentVariable(t *testing.T) {
	t.Setenv(constants.EnvOctopusProxy, "http://envproxy:3128")

	v := viper.New()
	assert.NoError(t, config.Setup(v))

	assert.Equal(t, "http://envproxy:3128", v.GetString(constants.ConfigProxyUrl))
}

func TestSetup_DefaultsTheProxyToEmpty(t *testing.T) {
	v := viper.New()
	assert.NoError(t, config.Setup(v))

	assert.Contains(t, v.AllKeys(), "proxyurl", "the proxy url must be a settable config key")
}

func TestSetup_DefaultsToVerifyingTheServerCertificate(t *testing.T) {
	v := viper.New()
	assert.NoError(t, config.Setup(v))

	assert.False(t, v.GetBool(constants.ConfigIgnoreSslErrors))
	assert.Contains(t, v.AllKeys(), "ignoresslerrors", "ignoring ssl errors must be a settable config key")
}

func TestSetup_BindsTheIgnoreSslErrorsEnvironmentVariable(t *testing.T) {
	t.Setenv(constants.EnvOctopusIgnoreSslErrors, "true")

	v := viper.New()
	assert.NoError(t, config.Setup(v))

	assert.True(t, v.GetBool(constants.ConfigIgnoreSslErrors))
}
