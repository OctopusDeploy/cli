package main

import (
	"fmt"
	"os"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/spf13/cobra"

	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/briandowns/spinner"

	"github.com/joho/godotenv"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/root"
)

func main() {
	// if there is a missing or invalid .env file anywhere, we don't care, just ignore it
	_ = godotenv.Load()

	// initialize our wrapper around survey, which is also used as a flag for whether
	// we are in interactive mode or automation mode
	askProvider := question.NewAskProvider(survey.AskOne)
	_, ci := os.LookupEnv("CI")
	// TODO move this to some other function and have it look for GITHUB_ACTIONS etc as we learn more about it
	if ci {
		askProvider.DisableInteractive()
	}

	clientFactory, err := apiclient.NewClientFactoryFromEnvironment(askProvider)
	if err != nil {
		fmt.Println(err)
		os.Exit(3)
	}

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithColor("cyan"))

	f := factory.New(clientFactory, askProvider, s)

	cmd := root.NewCmdRoot(f)
	// commands are expected to print their own errors to avoid double-ups
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	// if we don't do this then cmd.Print will get sent to stderr
	cmd.SetOut(os.Stdout)

	// if we attempt to check the flags before Execute is called, cobra hasn't parsed anything yet,
	// so we'll get bad values. PersistentPreRun is a convenient callback for setting up our
	// environment after parsing but before execution.
	cmd.PersistentPreRun = func(_ *cobra.Command, args []string) {
		if noPrompt, err := cmd.PersistentFlags().GetBool(constants.FlagNoPrompt); err == nil && noPrompt {
			askProvider.DisableInteractive()
		}

		if spaceNameOrId, err := cmd.PersistentFlags().GetString(constants.FlagSpace); err == nil && spaceNameOrId != "" {
			clientFactory.SetSpaceNameOrId(spaceNameOrId)
		}
	}

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
