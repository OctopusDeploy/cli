package apiclient

import (
	"fmt"
	version "github.com/OctopusDeploy/cli"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/spf13/cobra"
	"strings"
)

type Requester interface {
	GetRequester() string
}

type RequesterContext struct {
	cmd *cobra.Command
}

type FakeRequesterContext struct {
}

func NewRequester(c *cobra.Command) *RequesterContext {
	return &RequesterContext{
		cmd: c,
	}
}

func (r *FakeRequesterContext) GetRequester() string { return "octopus/0.0.0" }

func (r *RequesterContext) GetRequester() string {
	version := strings.TrimSpace(version.Version)

	if r.cmd == nil {
		if version == "" {
			return constants.ExecutableName
		}
		return fmt.Sprintf("%s/%s", constants.ExecutableName, version)
	}

	commands := []string{r.cmd.Name()}
	var rootCmd string
	parentCmd := r.cmd.Parent()
	for parentCmd != nil {
		name := parentCmd.Name()
		if name == constants.ExecutableName && version != "" {
			rootCmd = fmt.Sprintf("%s/%s", name, version)
		} else {
			commands = append([]string{name}, commands...)
		}
		parentCmd = parentCmd.Parent()
	}
	return fmt.Sprintf("%s (%s)", rootCmd, strings.Join(commands, ";"))
}
