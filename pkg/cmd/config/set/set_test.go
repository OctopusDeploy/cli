package set

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// isolateConfig points the config path at a temporary directory so the test writes
// there rather than to the config file of whoever is running it, and sets the global
// viper up so IsValidKey knows about the keys.
func isolateConfig(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("HOME", dir)    // unix
	t.Setenv("AppData", dir) // windows

	// IsValidKey deliberately reaches out to the global viper, so it has to be set up
	// against the temporary directory rather than the real config file
	viper.Reset()
	require.NoError(t, config.Setup(viper.GetViper()))
	t.Cleanup(viper.Reset)

	path, err := config.EnsureConfigPath()
	require.NoError(t, err)
	return filepath.Join(path, "cli_config.json")
}

func readConfig(t *testing.T, file string) map[string]any {
	t.Helper()

	contents, err := os.ReadFile(file)
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(contents, &settings))
	return settings
}

func TestSetRun_Shell(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr string
		want    string
	}{
		{name: "a shell name is stored", value: "powershell", want: "powershell"},
		{name: "an alias is stored as typed", value: "pwsh", want: "pwsh"},
		// empty is the documented default; it has to be accepted so there is a way back
		// to auto detection without hand editing the config file
		{name: "empty clears the setting", value: "", want: ""},
		{name: "a misspelling is rejected", value: "powershel", wantErr: "the provided value powershel is not a valid shell, please use one of bash, powershell, cmd"},
		{name: "an unsupported shell is rejected", value: "fish", wantErr: "the provided value fish is not a valid shell, please use one of bash, powershell, cmd"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := isolateConfig(t)

			err := setRun(false, nil, "Shell", test.value)

			if test.wantErr != "" {
				assert.EqualError(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, readConfig(t, file)["shell"])
		})
	}
}

// the switch in setRun grew a Shell case; this covers the two branches either side of
// it so that reshaping it can't quietly drop them
func TestSetRun_OtherKeys(t *testing.T) {
	t.Run("NoPrompt parses a bool", func(t *testing.T) {
		file := isolateConfig(t)

		require.NoError(t, setRun(false, nil, "NoPrompt", "true"))
		assert.Equal(t, true, readConfig(t, file)["noprompt"])
	})

	t.Run("NoPrompt rejects a non bool", func(t *testing.T) {
		isolateConfig(t)

		err := setRun(false, nil, "NoPrompt", "yes please")
		assert.EqualError(t, err, "the provided value yes please is not valid for NoPrompt, please use true of false")
	})

	t.Run("any other key is stored as given", func(t *testing.T) {
		file := isolateConfig(t)

		require.NoError(t, setRun(false, nil, "Space", "Default"))
		assert.Equal(t, "Default", readConfig(t, file)["space"])
	})

	t.Run("an unknown key is rejected", func(t *testing.T) {
		isolateConfig(t)

		err := setRun(false, nil, "NotAKey", "x")
		assert.EqualError(t, err, "the key 'NotAKey' is not a valid")
	})
}
