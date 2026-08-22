//go:build !windows

package shell

// parentProcessName is only used to tell cmd and PowerShell apart, so there is
// nothing to look up on unix.
func parentProcessName() string {
	return ""
}
