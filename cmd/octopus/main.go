package main

import (
	_ "embed"
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/util"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2/terminal"
	version "github.com/OctopusDeploy/cli"
	"github.com/briandowns/spinner"
	"github.com/spf13/viper"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/config"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/usage"

	"github.com/joho/godotenv"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/root"
)

func main() {
	// if there is a missing or invalid .env file anywhere, we don't care, just ignore it
	_ = godotenv.Load()

	viper := viper.GetViper()

	if err := config.Setup(viper); err != nil {
		fmt.Println(err)
		os.Exit(3)
	}
	arg := os.Args[1:]

	// initialize our wrapper around survey, which is also used as a flag for whether
	// we are in interactive mode or automation mode
	askProvider := question.NewAskProvider(survey.AskOne)
	_, ci := os.LookupEnv("CI")
	// TODO move this to some other function and have it look for GITHUB_ACTIONS etc as we learn more about it
	if ci {
		askProvider.DisableInteractive()
	}

	buildVersion := strings.TrimSpace(version.Version)

	clientFactory, err := apiclient.NewClientFactoryFromConfig(askProvider)
	if err != nil {
		// a small subset of commands can function even if the app doesn't have valid configuration information
		if commandDoesNotRequireClient(arg) {
			clientFactory = apiclient.NewStubClientFactory()
		} else {
			// can't possibly work
			fmt.Println(err)
			os.Exit(3)
		}
	}

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithColor("cyan"))

	c := config.New(viper)

	f := factory.New(clientFactory, askProvider, s, buildVersion, c)

	cmd := root.NewCmdRoot(f, clientFactory, askProvider)

	// if we don't do this then cmd.Print will get sent to stderr
	cmd.SetOut(terminal.NewAnsiStdout(os.Stdout))
	cmd.SetErr(terminal.NewAnsiStderr(os.Stderr))

	if err := cmd.Execute(); err != nil {
		cmd.PrintErr(err)
		cmd.Println()

		if usageError, ok := err.(*usage.UsageError); ok {
			// if the code returns a UsageError, print the usage information
			cmd.Println(usageError.Command().UsageString())
		}

		os.Exit(1)
	}
}

func commandDoesNotRequireClient(args []string) bool {
	cmdToRun := args[0]
	return cmdToRun == "" || cmdToRun == "config" || cmdToRun == "version" || cmdToRun == "help" || cmdToRun == "login" || cmdToRun == "logout" || (cmdToRun == "package" && util.SliceContains(args, "create"))
}
