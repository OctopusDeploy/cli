package surveyext

import (
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

const maxTerminalRetries = 3

// AskOne recovers from key sequences survey cannot parse. Survey aborts the
// prompt for any escape sequence outside the handful it understands, which on
// its own ends the whole command and discards every answer given so far.
func AskOne(p survey.Prompt, response any, opts ...survey.AskOpt) error {
	opts = append([]survey.AskOpt{withTranslatedStdio()}, opts...)

	var err error
	for attempt := 0; attempt <= maxTerminalRetries; attempt++ {
		err = survey.AskOne(p, response, opts...)
		if !isUnparsedKeyError(err) {
			return err
		}

		fmt.Fprintf(os.Stderr, "\nThat key isn't supported here. Please try again.\n")
	}
	return err
}

func withTranslatedStdio() survey.AskOpt {
	return survey.WithStdio(NewTranslatedStdin(os.Stdin), os.Stdout, os.Stderr)
}

// isUnparsedKeyError matches on the message because survey builds this with
// fmt.Errorf and exports no type for it.
func isUnparsedKeyError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unexpected escape sequence from terminal")
}
