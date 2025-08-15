package machinescommon

import (
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util"
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
	url := util.GenerateWebURL(dependencies.Host, dependencies.Space.GetID(), fmt.Sprintf("infrastructure/machines/%s/settings", target.GetID()))
	doWeb(url, description, dependencies.Out, flags)
}

func DoWebForWorkers(worker *machines.Worker, dependencies *cmd.Dependencies, flags *WebFlags, description string) {
	url := util.GenerateWebURL(dependencies.Host, dependencies.Space.GetID(), fmt.Sprintf("infrastructure/workers/%s/settings", worker.GetID()))
	doWeb(url, description, dependencies.Out, flags)
}

func DoWebForWorkerPools(workerPool workerpools.IWorkerPool, dependencies *cmd.Dependencies, flags *WebFlags) {
	url := util.GenerateWebURL(dependencies.Host, dependencies.Space.GetID(), fmt.Sprintf("infrastructure/workerpools/%s", workerPool.GetID()))
	doWeb(url, "Worker Pool", dependencies.Out, flags)
}

func doWeb(url string, description string, out io.Writer, flags *WebFlags) {
	link := output.Bluef(url)
	fmt.Fprintf(out, "View this %s on Octopus Deploy: %s\n", description, link)
	if flags.Web.Value {
		browser.OpenURL(url)
	}
}
