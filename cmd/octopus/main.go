package main

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
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

	f := factory.New(clientFactory, askProvider)

	cmd := root.NewCmdRoot(f, clientFactory, askProvider)
	// if we don't do this then cmd.Print will get sent to stderr
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

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
