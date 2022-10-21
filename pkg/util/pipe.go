package util

import (
	"bufio"
	"os"
)

func IsCalledFromPipe() bool {
	fileInfo, _ := os.Stdin.Stat()
	return fileInfo != nil && fileInfo.Mode()&os.ModeCharDevice == 0
}

// ReadValuesFromPipe will return an array of strings from the pipe
// input separated by new line.
func ReadValuesFromPipe() []string {
	items := []string{}
	if IsCalledFromPipe() {
		scanner := bufio.NewScanner(bufio.NewReader(os.Stdin))
		for scanner.Scan() {
			items = append(items, scanner.Text())
		}
	}
	return items
}
