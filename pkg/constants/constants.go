package constants

const (
	ExecutableName = "octopus"
)

// flags for command line switches
const (
	FlagHelp               = "help"
	FlagSpace              = "space"
	FlagOutputFormat       = "output-format"
	FlagOutputFormatLegacy = "outputFormat"
	FlagNoPrompt           = "no-prompt"
)

// flags for storing things in the go context
const (
	ContextKeyTimeNow = "time.now"
)

// values for output formats
const (
	OutputFormatJson  = "json"
	OutputFormatBasic = "basic"
	OutputFormatTable = "table" // TODO I'd like to rename this to just "standard" or "default"; discuss with team
)

// IsProgrammaticOutputFormat tells you if it is acceptable for your command to
// print miscellaneous output to stdout, such as progress messages.
// If your command is capable of printing such things, you should check the output format
// first, lest you print a progress message into the middle of a JSON document by accident.
func IsProgrammaticOutputFormat(outputFormat string) bool { // TODO consider whether we should move this into the Factory
	switch outputFormat {
	case OutputFormatJson, OutputFormatBasic:
		return true
	default:
		return false
	}
}
