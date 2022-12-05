package machinescommon

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"io"
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

func DoWebForTargets(target *machines.DeploymentTarget, dependencies *cmd.Dependencies, flags *WebFlags, description string) {
	url := fmt.Sprintf("%s/app#/%s/infrastructure/machines/%s", dependencies.Host, dependencies.Space.GetID(), target.GetID())
	doWeb(url, description, dependencies.Out, flags)
}

func DoWebForWorkers(worker *machines.Worker, dependencies *cmd.Dependencies, flags *WebFlags, description string) {
	url := fmt.Sprintf("%s/app#/%s/infrastructure/workers/%s", dependencies.Host, dependencies.Space.GetID(), worker.GetID())
	doWeb(url, description, dependencies.Out, flags)
}

func DoWebForWorkerPools(workerPool workerpools.IWorkerPool, dependencies *cmd.Dependencies, flags *WebFlags) {
	url := fmt.Sprintf("%s/app#/%s/infrastructure/workerpools/%s", dependencies.Host, dependencies.Space.GetID(), workerPool.GetID())
	doWeb(url, "Worker Pool", dependencies.Out, flags)
}

func doWeb(url string, description string, out io.Writer, flags *WebFlags) {
	link := output.Bluef(url)
	fmt.Fprintf(out, "View this %s on Octopus Deploy: %s\n", description, link)
	if flags.Web.Value {
		browser.OpenURL(url)
	}
}
