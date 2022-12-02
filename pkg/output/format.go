package output

import "strings"

func FormatAsList(items []string) string {
	return strings.Join(items, ", ")
}
