package main

import (
	"os"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/root"
)

func main() {
	client := apiclient.New()
	cmd := root.NewCmdRoot(client)
	if err := cmd.Execute(); err != nil {
		cmd.PrintErr(err)
		os.Exit(1)
	}
}
