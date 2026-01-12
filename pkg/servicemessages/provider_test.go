package servicemessages

import (
	"bytes"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/spf13/viper"
)

func TestServiceMessage(t *testing.T) {
	tests := []struct {
		name            string
		servicemessages bool
		teamCityEnv     bool
		messsageName    string
		key             string
		value           any
		stdout          *bytes.Buffer
		stderr          *bytes.Buffer
		want            string
		wantErr         string
	}{
		{"service message flag is not enabled", false, false, "testMessage", "key1", "value1", &bytes.Buffer{}, &bytes.Buffer{}, "", ""},
		{"service message enabled with teamcity envvar and map value", true, true, "testMessage", "key1", map[string]string{"key": "value"}, &bytes.Buffer{}, &bytes.Buffer{}, "##teamcity[testMessage key=value]\n", ""},
		{"service message enabled without teamcity envvar", true, false, "testMessage", "key1", "value1", &bytes.Buffer{}, &bytes.Buffer{}, "", "service messages are only supported in TeamCity builds"},
		{"service message enabled with teamcity envvar and string value", true, true, "testMessage", "key1", "value", &bytes.Buffer{}, &bytes.Buffer{}, "##teamcity[testMessage value]\n", ""},
		{"service message enabled with teamcity envvar and unsupported value", true, true, "testMessage", "key1", []string{"dsdsd"}, &bytes.Buffer{}, &bytes.Buffer{}, "", "Unsupported service message value type"},
	}

	for _, tt := range tests {
		setupArgs(t, constants.FlagEnableServiceMessages, tt.servicemessages)
		setupEnvVar(t, "TEAMCITY_VERSION", "2021.1", tt.teamCityEnv)
		t.Run(tt.name, func(t *testing.T) {
			NewProvider(NewOutputPrinter(tt.stdout, tt.stderr)).ServiceMessage(tt.messsageName, tt.value)
			if tt.want != "" {
				got := tt.stdout.String()
				if got != tt.want {
					t.Errorf("Expected output:\n%s\nGot:\n%s", tt.want, got)
				}
			}
			if tt.wantErr != "" {
				e := tt.stderr.String()
				if e != tt.wantErr {
					t.Errorf("Expected error output:\n%s\nGot:\n%s", tt.wantErr, e)
				}
			}
		})
	}
}

func setupArgs(t *testing.T, key string, value bool) {
	viper.Reset()
	viper.Set(constants.FlagEnableServiceMessages, value)
}

func setupEnvVar(t *testing.T, key, value string, set bool) {
	if set {
		t.Setenv(key, value)
	} else {
		t.Setenv(key, "")
	}
}
