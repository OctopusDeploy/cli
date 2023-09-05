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
	ContextKeyTimeNow = "time.now" // func() time.Time
	ContextKeyOsOpen  = "os.open"  // func(string) (io.ReadCloser, error)
)

// values for output formats
const (
	OutputFormatJson  = "json"
	OutputFormatBasic = "basic"
	OutputFormatTable = "table" // TODO I'd like to rename this to just "standard" or "default"; discuss with team
)

// keys for key/value store config file
const (
	ConfigUrl         = "Url"
	ConfigApiKey      = "ApiKey"
	ConfigAccessToken = "AccessToken"
	ConfigSpace       = "Space"
	ConfigNoPrompt    = "NoPrompt"
	// ConfigProxyUrl     = "ProxyUrl"
	ConfigEditor       = "Editor"
	ConfigShowOctopus  = "ShowOctopus"
	ConfigOutputFormat = "OutputFormat"
)

const (
	EnvOctopusUrl         = "OCTOPUS_URL"
	EnvOctopusApiKey      = "OCTOPUS_API_KEY"
	EnvOctopusAccessToken = "OCTOPUS_ACCESS_TOKEN"
	EnvOctopusSpace       = "OCTOPUS_SPACE"
	EnvEditor             = "EDITOR"
	EnvVisual             = "VISUAL"
	EnvCI                 = "CI"
)

const (
	NoDescription = "No description provided"
)

const OctopusLogo = `            &#BGGGGGGB#&
         &GP%%%%%%%%%%%%%G&
       &G%%%%%%%%%%%%%%%%%%P&
      &%%%%%%%%%%%%%%%%%%%%%%#
      %%%%%%%%%%%%%%%%%%%%%%%%
     #%%%%%%%%%%%%%%%%%%%%%%%%&
     #%%%%%%%%%%%%%%%%%%%%%%%%&
      P%%%%%%%%%%%%%%%%%%%%%%#
      &%%%%%%%%%%%%%%%%%%%%%G
       B%%%%%%%%%%%%%%%%%%%%#
       P%%%%%%%%%%%%%%%%%%%%P
     &P%%%%%%%%%%%%%%%%%%%%%%P#
   #G%%%%P%%%%%%GP%%%%%%PP%%%%%PG&
&G%%%PB&#P%%%%G   G%%%%P  &P%%P#&&
 &&     G%%%%#    &%%%%G    #P%%#
       &%%%G&      G%%%B       &&
        &#          BPB`

const (
	PromptCreateNew = "<Create New>"
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
