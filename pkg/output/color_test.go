package output

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsColorEnabled(t *testing.T) {
	tests := []struct {
		name       string
		noColor    string
		clicolor   string
		forceColor string
		cliColor   string
		// expected result when stdout is, and is not, a terminal
		expectOnTerminal    bool
		expectNotOnTerminal bool
	}{
		{
			name:             "no environment variables set: terminal detection decides",
			expectOnTerminal: true,
		},
		{
			name:                "FORCE_COLOR forces colour on",
			forceColor:          "1",
			expectOnTerminal:    true,
			expectNotOnTerminal: true,
		},
		{
			name:                "CLICOLOR_FORCE forces colour on",
			clicolor:            "1",
			expectOnTerminal:    true,
			expectNotOnTerminal: true,
		},
		{
			name:       "FORCE_COLOR of 0 forces colour off",
			forceColor: "0",
		},
		{
			name:     "CLICOLOR_FORCE of 0 forces colour off",
			clicolor: "0",
		},
		{
			name:     "CLICOLOR of 0 disables colour",
			cliColor: "0",
		},
		{
			name:             "CLICOLOR of 1 leaves terminal detection to decide",
			cliColor:         "1",
			expectOnTerminal: true,
		},
		{
			name:       "NO_COLOR wins over FORCE_COLOR",
			noColor:    "1",
			forceColor: "1",
		},
		{
			name:     "NO_COLOR wins over CLICOLOR_FORCE",
			noColor:  "1",
			clicolor: "1",
		},
		{
			name:                "CLICOLOR_FORCE wins over FORCE_COLOR",
			clicolor:            "1",
			forceColor:          "0",
			expectOnTerminal:    true,
			expectNotOnTerminal: true,
		},
		{
			name:                "FORCE_COLOR wins over CLICOLOR",
			forceColor:          "1",
			cliColor:            "0",
			expectOnTerminal:    true,
			expectNotOnTerminal: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", test.noColor)
			t.Setenv("CLICOLOR_FORCE", test.clicolor)
			t.Setenv("FORCE_COLOR", test.forceColor)
			t.Setenv("CLICOLOR", test.cliColor)

			assert.Equal(t, test.expectOnTerminal, isColorEnabledFor(true), "on a terminal")
			assert.Equal(t, test.expectNotOnTerminal, isColorEnabledFor(false), "not on a terminal")
		})
	}
}

// Every exported helper must honour IsColorEnabled, otherwise NO_COLOR only
// partially takes effect.
func TestColorHelpersHonourIsColorEnabled(t *testing.T) {
	helpers := map[string]struct {
		plain     func(string) string
		formatted func(string, ...interface{}) string
	}{
		"Blue":    {Blue, Bluef},
		"Magenta": {Magenta, Magentaf},
		"Cyan":    {Cyan, Cyanf},
		"Red":     {Red, Redf},
		"Yellow":  {Yellow, Yellowf},
		"Green":   {Green, Greenf},
		"Bold":    {Bold, Boldf},
		"Dim":     {Dim, Dimf},
	}

	original := IsColorEnabled
	t.Cleanup(func() { IsColorEnabled = original })

	for name, helper := range helpers {
		t.Run(name, func(t *testing.T) {
			IsColorEnabled = false
			assert.Equal(t, "text", helper.plain("text"))
			assert.Equal(t, "text", helper.formatted("%s", "text"))

			IsColorEnabled = true
			assert.NotEqual(t, "text", helper.plain("text"))
			assert.NotEqual(t, "text", helper.formatted("%s", "text"))
		})
	}
}

// GitHub Actions and other log viewers reset SGR state at every line break, so
// a multi-line string needs the escape repeated on each line rather than once
// around the whole block.
func TestMultiLineTextIsColouredPerLine(t *testing.T) {
	original := IsColorEnabled
	t.Cleanup(func() { IsColorEnabled = original })

	IsColorEnabled = true
	lines := strings.Split(Cyan("one\ntwo\nthree"), "\n")

	assert.Len(t, lines, 3)
	for _, line := range lines {
		assert.True(t, strings.HasPrefix(line, "\x1b["), "line should open with an escape: %q", line)
		assert.True(t, strings.HasSuffix(line, "\x1b[0m"), "line should close with a reset: %q", line)
	}
}

// A blank line has nothing to colour, so escapes around it would be the only
// thing on the line.
func TestBlankLinesAreNotColoured(t *testing.T) {
	original := IsColorEnabled
	t.Cleanup(func() { IsColorEnabled = original })

	IsColorEnabled = true
	lines := strings.Split(Cyan("one\n\ntwo\n"), "\n")

	assert.Len(t, lines, 4)
	assert.Equal(t, "", lines[1], "interior blank line should be untouched")
	assert.Equal(t, "", lines[3], "trailing newline should not gain escapes")
}

func TestMultiLineTextIsUnchangedWhenColourDisabled(t *testing.T) {
	original := IsColorEnabled
	t.Cleanup(func() { IsColorEnabled = original })

	IsColorEnabled = false
	assert.Equal(t, "one\ntwo\nthree", Cyan("one\ntwo\nthree"))
}

func TestFormatDocHonoursIsColorEnabled(t *testing.T) {
	original := IsColorEnabled
	t.Cleanup(func() { IsColorEnabled = original })

	doc := "bold(a) green(b) yellow(c) blue(d) cyan(e) magenta(f) red(g) dim(h)"

	IsColorEnabled = false
	assert.Equal(t, "a b c d e f g h", FormatDoc(doc))

	IsColorEnabled = true
	assert.NotEqual(t, "a b c d e f g h", FormatDoc(doc))
}
