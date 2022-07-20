package output

import (
	"fmt"
	"os"

	"github.com/mgutz/ansi"
	"golang.org/x/term"
)

var (
	isColorEnabled = os.Getenv("NO_COLOR") == "" && term.IsTerminal(int(os.Stdout.Fd()))
	magenta        = ansi.ColorFunc("magenta")
	cyan           = ansi.ColorFunc("cyan")
	red            = ansi.ColorFunc("red")
	yellow         = ansi.ColorFunc("yellow")
	blue           = ansi.ColorFunc("blue")
	green          = ansi.ColorFunc("green")
	bold           = ansi.ColorFunc("default+b")
)

func Blue(s string) string {
	if !isColorEnabled {
		return s
	}
	return blue(s)
}

func Bluef(s string, args ...interface{}) string {
	return Blue(fmt.Sprintf(s, args...))
}

func Magenta(s string) string {
	if !isColorEnabled {
		return s
	}
	return magenta(s)
}

func Magentaf(s string, args ...interface{}) string {
	return Magenta(fmt.Sprintf(s, args...))
}

func Cyan(s string) string {
	if !isColorEnabled {
		return s
	}
	return cyan(s)
}

func Cyanf(s string, args ...interface{}) string {
	return Cyan(fmt.Sprintf(s, args...))
}

func Red(s string) string {
	if !isColorEnabled {
		return s
	}
	return red(s)
}

func Redf(s string, args ...interface{}) string {
	return Red(fmt.Sprintf(s, args...))
}

func Yellow(s string) string {
	if !isColorEnabled {
		return s
	}
	return yellow(s)
}

func Yellowf(s string, args ...interface{}) string {
	return Yellow(fmt.Sprintf(s, args...))
}

func Green(s string) string {
	if !isColorEnabled {
		return s
	}
	return green(s)
}

func Greenf(s string, args ...interface{}) string {
	return Green(fmt.Sprintf(s, args...))
}

func Bold(s string) string {
	if !isColorEnabled {
		return s
	}
	return bold(s)
}

func Boldf(s string, args ...interface{}) string {
	return Bold(fmt.Sprintf(s, args...))
}
