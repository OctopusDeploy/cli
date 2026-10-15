package call

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadParams_Inline(t *testing.T) {
	arguments, err := readParams(`{"PartialName": "web", "Take": 5}`, strings.NewReader(""))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"PartialName": "web", "Take": float64(5)}, arguments)
}

func TestReadParams_EmptyIsNoArguments(t *testing.T) {
	for _, value := range []string{"", "  "} {
		arguments, err := readParams(value, strings.NewReader(""))
		require.NoError(t, err)
		assert.Equal(t, map[string]any{}, arguments)
	}
}

func TestReadParams_FromStdin(t *testing.T) {
	// a value with a single quote is the kind of thing that's awkward to pass inline
	arguments, err := readParams("-", strings.NewReader(`{"ReleaseNotes": "it's fixed"}`+"\n"))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"ReleaseNotes": "it's fixed"}, arguments)
}

func TestReadParams_FromEmptyStdin(t *testing.T) {
	arguments, err := readParams("-", strings.NewReader(""))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{}, arguments)
}

func TestReadParams_FromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "params.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"ProjectName": "Web", "Variables": {"Foo": "bar"}}`), 0o600))

	arguments, err := readParams("@"+path, strings.NewReader(""))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"ProjectName": "Web", "Variables": map[string]any{"Foo": "bar"}}, arguments)
}

func TestReadParams_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")

	_, err := readParams("@"+path, strings.NewReader(""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read --params")
	assert.Contains(t, err.Error(), "missing.json")
}

func TestReadParams_RejectsNonObjects(t *testing.T) {
	path := filepath.Join(t.TempDir(), "params.json")
	require.NoError(t, os.WriteFile(path, []byte(`[1, 2]`), 0o600))

	tests := []struct {
		name    string
		value   string
		stdin   string
		message string
	}{
		{"inline array", `["a"]`, "", "--params must be a JSON object"},
		{"inline invalid", `{PartialName: web}`, "", "--params must be a JSON object"},
		{"inline null", `null`, "", "--params must be a JSON object, not null"},
		{"stdin invalid", "-", "not json", "the JSON on stdin must be a JSON object"},
		{"file array", "@" + path, "", "the JSON in '" + path + "' must be a JSON object"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := readParams(test.value, strings.NewReader(test.stdin))
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.message)
		})
	}
}
