//go:build windows

package shell

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParentProcessName is a smoke test for the Toolhelp32 walk. Detect's own tests use
// a stub, so without this nothing ever executes the real thing, and a snapshot that
// silently returns nothing looks exactly like a shell we don't recognise: both fall
// through to the cmd default, so the bug would never show up as a failure.
//
// The name itself can't be asserted, because it is whatever launched the test binary
// (go.exe under `go test`, the debugger under an IDE). That it found a name at all is
// the part that matters.
func TestParentProcessName(t *testing.T) {
	name := parentProcessName()

	assert.NotEmpty(t, name, "the parent process should be found")
	assert.True(t, strings.HasSuffix(strings.ToLower(name), ".exe"), "expected an executable name, got %q", name)
}
