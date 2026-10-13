// Command argvecho prints the arguments it was given as JSON, so a test can see exactly
// what a program receives after a shell and the Go runtime's argv parsing have both had
// a go at the command line. It lives under testdata so it isn't part of the build; the
// round trip tests compile it on demand.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// marker prefixes the output because the round trip feeds commands to an interactive
// cmd.exe, whose stdout also carries the banner and the prompt.
const marker = "ARGVECHO:"

func main() {
	encoded, err := json.Marshal(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(marker + string(encoded))
}
