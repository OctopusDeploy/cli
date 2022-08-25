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

const (
	OutputFormatJson  = "json"
	OutputFormatBasic = "basic"
	OutputFormatTable = "table"
)

// IsProgrammaticOutputFormat tells you if it is acceptable for your command to
// print miscellaneous output to stdout, such as progress messages.
// If your command is capable of printing such things, you should check the output format
// first, lest you print a progress message into the middle of a JSON document by accident.
func IsProgrammaticOutputFormat(outputFormat string) bool {
	switch outputFormat {
	case OutputFormatJson, OutputFormatBasic:
		return true
	default:
		return false
	}
}
