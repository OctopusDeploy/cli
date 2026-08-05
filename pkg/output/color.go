package output

import (
	"fmt"
	"os"
	"regexp"

	"github.com/mgutz/ansi"
	"golang.org/x/term"
)

var (
	IsColorEnabled = isColorEnabled()
	magenta        = ansi.ColorFunc("magenta")
	cyan           = ansi.ColorFunc("cyan")
	red            = ansi.ColorFunc("red")
	yellow         = ansi.ColorFunc("yellow")
	blue           = ansi.ColorFunc("blue")
	green          = ansi.ColorFunc("green")
	bold           = ansi.ColorFunc("default+b")
	dim            = ansi.ColorFunc("default+d")
)

// isColorEnabled decides whether ANSI colour codes should be emitted, following
// the widely adopted no-color.org and bixense.com/clicolors conventions:
//
//   - NO_COLOR set to anything non-empty disables colour outright.
//   - CLICOLOR_FORCE or FORCE_COLOR turns colour on even when stdout is not a
//     terminal. CI systems such as GitHub Actions and GitLab CI render ANSI
//     codes but do not attach a TTY, so terminal detection alone can never
//     enable colour there. Setting either to "0" is the opposite instruction and
//     turns colour off even when stdout is a terminal.
//   - CLICOLOR set to "0" disables colour.
//   - Otherwise colour is used only when stdout is a terminal.
func isColorEnabled() bool {
	return isColorEnabledFor(term.IsTerminal(int(os.Stdout.Fd())))
}

// isColorEnabledFor is isColorEnabled with terminal detection supplied by the
// caller, so the decision table can be tested in both directions.
func isColorEnabledFor(isTerminal bool) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// An explicitly set force variable is an instruction in both directions, so it
	// overrides terminal detection whichever way it points.
	for _, name := range []string{"CLICOLOR_FORCE", "FORCE_COLOR"} {
		if value, isSet := os.LookupEnv(name); isSet && value != "" {
			return value != "0"
		}
	}

	if os.Getenv("CLICOLOR") == "0" {
		return false
	}

	return isTerminal
}

func Blue(s string) string {
	if !IsColorEnabled {
		return s
	}
	return blue(s)
}

func Bluef(s string, args ...interface{}) string {
	return Blue(fmt.Sprintf(s, args...))
}

func Magenta(s string) string {
	if !IsColorEnabled {
		return s
	}
	return magenta(s)
}

func Magentaf(s string, args ...interface{}) string {
	return Magenta(fmt.Sprintf(s, args...))
}

func Cyan(s string) string {
	if !IsColorEnabled {
		return s
	}
	return cyan(s)
}

func Cyanf(s string, args ...interface{}) string {
	return Cyan(fmt.Sprintf(s, args...))
}

func Red(s string) string {
	if !IsColorEnabled {
		return s
	}
	return red(s)
}

func Redf(s string, args ...interface{}) string {
	return Red(fmt.Sprintf(s, args...))
}

func Yellow(s string) string {
	if !IsColorEnabled {
		return s
	}
	return yellow(s)
}

func Yellowf(s string, args ...interface{}) string {
	return Yellow(fmt.Sprintf(s, args...))
}

func Green(s string) string {
	if !IsColorEnabled {
		return s
	}
	return green(s)
}

func Greenf(s string, args ...interface{}) string {
	return Green(fmt.Sprintf(s, args...))
}

func Bold(s string) string {
	if !IsColorEnabled {
		return s
	}
	return bold(s)
}

func Boldf(s string, args ...interface{}) string {
	return Bold(fmt.Sprintf(s, args...))
}

func Dim(s string) string {
	if !IsColorEnabled {
		return s
	}
	return dim(s)
}

func Dimf(s string, args ...interface{}) string {
	return Dim(fmt.Sprintf(s, args...))
}

// FormatDoc is designed to take a large block of heredoc text and replace formatting elements within it.
// Like a really cheap basic version of Markdown
func FormatDoc(str string) string {
	str = regexp.MustCompile("bold\\((.*?)\\)").ReplaceAllString(str, Bold("$1"))
	str = regexp.MustCompile("green\\((.*?)\\)").ReplaceAllString(str, Green("$1"))
	str = regexp.MustCompile("yellow\\((.*?)\\)").ReplaceAllString(str, Yellow("$1"))
	str = regexp.MustCompile("blue\\((.*?)\\)").ReplaceAllString(str, Blue("$1"))
	str = regexp.MustCompile("cyan\\((.*?)\\)").ReplaceAllString(str, Cyan("$1"))
	str = regexp.MustCompile("magenta\\((.*?)\\)").ReplaceAllString(str, Magenta("$1"))
	str = regexp.MustCompile("red\\((.*?)\\)").ReplaceAllString(str, Red("$1"))
	str = regexp.MustCompile("dim\\((.*?)\\)").ReplaceAllString(str, Dim("$1"))
	return str
}
