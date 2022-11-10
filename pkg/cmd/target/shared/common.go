package shared

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

const (
	FlagWeb = "web"
)

type WebFlags struct {
	Web *flag.Flag[bool]
}

func NewWebFlags() *WebFlags {
	return &WebFlags{
		Web: flag.New[bool](FlagWeb, false),
	}
}

func RegisterWebFlag(cmd *cobra.Command, flags *WebFlags) {
	cmd.Flags().BoolVarP(&flags.Web.Value, flags.Web.Name, "w", false, "Open in web browser")
}

func DoWeb(target *machines.DeploymentTarget, dependencies *cmd.Dependencies, flags *WebFlags, description string) {
	url := fmt.Sprintf("%s/app#/%s/infrastructure/machines/%s", dependencies.Host, dependencies.Space.GetID(), target.GetID())
	link := output.Bluef(url)
	fmt.Fprintf(dependencies.Out, "View this %s on Octopus Deploy: %s\n", description, link)
	if flags.Web.Value {
		browser.OpenURL(url)
	}
}
