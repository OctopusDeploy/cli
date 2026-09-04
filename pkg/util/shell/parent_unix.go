//go:build !windows

package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// parentProcessName returns the file name of the executable that launched us, or ""
// when it can't be worked out. $SHELL names the login shell, not the shell the command
// was actually typed into, so this is what tells us the user is sitting in pwsh on a
// machine whose login shell is something else.
func parentProcessName() string {
	ppid := os.Getppid()
	if ppid <= 1 {
		return ""
	}

	if runtime.GOOS == "linux" {
		// /proc/<pid>/comm is truncated to 15 characters, which is fine for every name
		// Parse recognises
		if b, err := os.ReadFile("/proc/" + strconv.Itoa(ppid) + "/comm"); err == nil {
			return strings.TrimSpace(string(b))
		}
		return ""
	}

	// darwin and the bsds have no /proc, so ask ps. -o comm= prints the executable with
	// no header, as a full path on darwin, so take the base of it
	out, err := exec.Command("ps", "-o", "comm=", "-p", strconv.Itoa(ppid)).Output()
	if err != nil {
		return ""
	}
	return filepath.Base(strings.TrimSpace(string(out)))
}
