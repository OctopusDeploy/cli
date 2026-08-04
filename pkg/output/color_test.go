package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Note: under `go test` stdout is not a terminal, so the terminal-detection
// fallback always resolves to false. That makes these cases deterministic.
func TestIsColorEnabled(t *testing.T) {
	tests := []struct {
		name         string
		noColor      string
		clicolor     string
		forceColor   string
		expectResult bool
	}{
		{name: "no environment variables set, not a terminal", expectResult: false},
		{name: "FORCE_COLOR forces colour on", forceColor: "1", expectResult: true},
		{name: "CLICOLOR_FORCE forces colour on", clicolor: "1", expectResult: true},
		{name: "FORCE_COLOR of 0 does not force colour on", forceColor: "0", expectResult: false},
		{name: "CLICOLOR_FORCE of 0 does not force colour on", clicolor: "0", expectResult: false},
		{name: "NO_COLOR wins over FORCE_COLOR", noColor: "1", forceColor: "1", expectResult: false},
		{name: "NO_COLOR wins over CLICOLOR_FORCE", noColor: "1", clicolor: "1", expectResult: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", test.noColor)
			t.Setenv("CLICOLOR_FORCE", test.clicolor)
			t.Setenv("FORCE_COLOR", test.forceColor)

			assert.Equal(t, test.expectResult, isColorEnabled())
		})
	}
}
