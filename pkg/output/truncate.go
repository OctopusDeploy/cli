package output

import (
	"github.com/muesli/reflow/ansi"
	"github.com/muesli/reflow/truncate"
)

const (
	ellipsis            = "..."
	minWidthForEllipsis = len(ellipsis) + 2
)

func Truncate(maxWidth int, s string) string {
	w := ansi.PrintableRuneWidth(s)
	if w <= maxWidth {
		return s
	}

	tail := ""
	if maxWidth >= minWidthForEllipsis {
		tail = ellipsis
	}

	ts := truncate.StringWithTail(s, uint(maxWidth), tail)
	if ansi.PrintableRuneWidth(ts) < maxWidth {
		ts += " "
	}

	return ts
}
