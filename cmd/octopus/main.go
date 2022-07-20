package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/root"
)

func main() {
	// if there is a missing or invalid .env file anywhere, we don't care, just ignore it
	_ = godotenv.Load()

	client, err := apiclient.NewFromEnvironment()
	if err != nil {
		fmt.Println(err)
		os.Exit(3)
	}

	cmd := root.NewCmdRoot(client)
	if err := cmd.Execute(); err != nil {
		cmd.PrintErr(err)
		os.Exit(1)
	}
}
