package cmd

import (
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"io"

	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/cobra"
)

type Dependable interface {
	Commit() error
	GenerateAutomationCmd()
}

type Dependencies struct {
	Out               io.Writer
	Client            *client.Client
	Host              string
	Space             *spaces.Space
	NoPrompt          bool
	Ask               question.Asker
	CmdPath           string
	ShowMessagePrefix bool
}

func NewDependencies(f factory.Factory, cmd *cobra.Command) *Dependencies {
	client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		panic(err)
	}

	return newDependencies(f, cmd, client)
}

func NewSystemDependencies(f factory.Factory, cmd *cobra.Command) *Dependencies {
	client, err := f.GetSystemClient(apiclient.NewRequester(cmd))
	if err != nil {
		panic(err)
	}
	return newDependencies(f, cmd, client)
}

func newDependencies(f factory.Factory, cmd *cobra.Command, client *client.Client) *Dependencies {
	return &Dependencies{
		Ask:      f.Ask,
		CmdPath:  cmd.CommandPath(),
		Out:      cmd.OutOrStdout(),
		Client:   client,
		Host:     f.GetCurrentHost(),
		NoPrompt: !f.IsPromptEnabled(),
		Space:    f.GetCurrentSpace(),
	}
}

func NewDependenciesFromExisting(opts *Dependencies, cmdPath string) *Dependencies {
	return &Dependencies{
		Ask:               opts.Ask,
		CmdPath:           cmdPath,
		Out:               opts.Out,
		Client:            opts.Client,
		Host:              opts.Host,
		NoPrompt:          opts.NoPrompt,
		Space:             opts.Space,
		ShowMessagePrefix: true,
	}
}
