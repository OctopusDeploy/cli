package main

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/usage"

	"github.com/joho/godotenv"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/root"
)

func main() {
	// if there is a missing or invalid .env file anywhere, we don't care, just ignore it
	_ = godotenv.Load()

	clientFactory, err := apiclient.NewFromEnvironment()
	if err != nil {
		fmt.Println(err)
		os.Exit(3)
	}

	client := factory.New(clientFactory, survey.AskOne)

	cmd := root.NewCmdRoot(client)
	// commands are expected to print their own errors to avoid double-ups
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

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
