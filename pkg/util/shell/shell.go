package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/spf13/viper"
)

// Shell identifies the command line interpreter that generated automation commands
// are quoted for. Each shell has its own quoting and escaping rules.
type Shell string

const (
	// Bash covers the POSIX shell family; sh, bash, zsh and friends all quote the same way.
	Bash       Shell = "bash"
	PowerShell Shell = "powershell"
	Cmd        Shell = "cmd"
)

// Names lists the values accepted by Parse, for help text and error messages.
var Names = []string{string(Bash), string(PowerShell), string(Cmd)}

// Parse converts a shell name, such as the value of a config setting or the name of
// an executable, into a Shell.
func Parse(name string) (Shell, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.TrimSuffix(name, ".exe")
	switch name {
	case "sh", "bash", "zsh", "ksh", "dash", "ash":
		return Bash, true
	case "powershell", "pwsh":
		return PowerShell, true
	case "cmd", "command":
		return Cmd, true
	}
	return "", false
}

// Validate returns an error if name isn't a shell we can generate commands for.
func Validate(name string) error {
	if _, ok := Parse(name); !ok {
		return fmt.Errorf("the provided value %s is not a valid shell, please use one of %s", name, strings.Join(Names, ", "))
	}
	return nil
}

// Current returns the shell to generate automation commands for; the explicitly
// configured shell if there is one, otherwise the detected host shell.
func Current() Shell {
	if s, ok := Parse(viper.GetString(constants.ConfigShell)); ok {
		return s
	}
	return Detect(runtime.GOOS, os.Getenv)
}

// Detect works out which shell the CLI is being run from. goos is a runtime.GOOS value
// and getenv looks up environment variables; both are parameters so this can be tested.
func Detect(goos string, getenv func(string) string) Shell {
	// checked here as well as through viper so the override still works if the
	// config system hasn't been set up, such as in tests
	if s, ok := Parse(getenv(constants.EnvOctopusShell)); ok {
		return s
	}

	if goos == "windows" {
		// the parent process is the only reliable signal on windows; PSModulePath is
		// a machine wide variable that cmd.exe inherits too, so it tells us nothing.
		// note this always misses when cross-compiled elsewhere, which is why it can't
		// be the only check.
		if s, ok := Parse(parentProcessName()); ok {
			return s
		}
		// cmd is the guess with the better failure mode, though neither is safe. Single
		// quoted output is useless in cmd for any value that needed quoting, whereas cmd
		// output is mostly readable in PowerShell: a plain quoted value like
		// "Soft Drinks" works in both. It only diverges for a value carrying a double
		// quote, where the ^ escapes are meaningless to PowerShell, or a trailing
		// backslash, which cmd doubles for argv and PowerShell leaves alone. Setting
		// Shell in config is the fix when detection can't see the parent process.
		return Cmd
	}

	if sh := getenv("SHELL"); sh != "" {
		if s, ok := Parse(filepath.Base(sh)); ok {
			return s
		}
	}
	return Bash
}
