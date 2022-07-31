package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"time"

	"github.com/AlecAivazis/survey/v2"
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

	clientFactory, err := apiclient.NewClientFactoryFromEnvironment()
	if err != nil {
		fmt.Println(err)
		os.Exit(3)
	}

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithColor("cyan"))

	f := factory.New(clientFactory, survey.AskOne, s)

	cmd := root.NewCmdRoot(f)
	// commands are expected to print their own errors to avoid double-ups
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	// if we attempt to check the flags before Execute is called, cobra hasn't parsed anything yet,
	// so we'll get bad values. PersistentPreRun is a convenient callback for setting up our
	// environment after parsing but before execution.
	cmd.PersistentPreRun = func(_ *cobra.Command, args []string) {
		if noPrompt, err := cmd.PersistentFlags().GetBool(root.FlagNoPrompt); err == nil && noPrompt {
			f.SetPromptDisabled()
		}

		if spaceNameOrId, err := cmd.PersistentFlags().GetString(root.FlagSpace); err == nil && spaceNameOrId != "" {
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
