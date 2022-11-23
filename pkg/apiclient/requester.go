package apiclient

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/spf13/cobra"
	"runtime/debug"
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
	version := "0.0.0"
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			if dep.Path == "github.com/OctopusDeploy/cli" {
				if dep.Version != "" {
					version = dep.Version
				}
			}
		}
	}

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
