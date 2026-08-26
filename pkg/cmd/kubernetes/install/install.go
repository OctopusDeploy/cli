package install

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	gatewayInstall "github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/gateway/install"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/spf13/cobra"
)

// component delegates to its own subcommand, so the wizard stays a router and
// every component remains independently scriptable.
type component struct {
	display string
	cmdPath string
	install func(f factory.Factory, dependencies *cmd.Dependencies, cmdPath string) error
}

func components() []component {
	return []component{
		{
			display: "Argo CD gateway - connect an Argo CD instance to Octopus",
			cmdPath: constants.ExecutableName + " kubernetes gateway install",
			install: func(f factory.Factory, dependencies *cmd.Dependencies, cmdPath string) error {
				return gatewayInstall.Run(f, cmd.NewDependenciesFromExisting(dependencies, cmdPath))
			},
		},
	}
}

func NewCmdInstall(f factory.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install an Octopus component into a Kubernetes cluster",
		Long: heredoc.Doc(`
			Install an Octopus component into a Kubernetes cluster.

			Asks which component to install and then runs its installer. Each component also has
			its own command, which is what to use in a script.
		`),
		Example: heredoc.Docf("$ %s kubernetes install", constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			dependencies := cmd.NewDependencies(f, c)
			available := components()

			if dependencies.NoPrompt {
				return noPromptError(available)
			}

			selected, err := question.SelectMap(dependencies.Ask, "What would you like to install?", available,
				func(item component) string { return item.display })
			if err != nil {
				return err
			}

			return selected.install(f, dependencies, selected.cmdPath)
		},
	}
}

func noPromptError(available []component) error {
	paths := make([]string, 0, len(available))
	for _, c := range available {
		paths = append(paths, "  "+c.cmdPath)
	}
	return fmt.Errorf("%s cannot ask which component to install while prompting is disabled. Run one of these instead:\n%s",
		constants.ExecutableName+" kubernetes install", strings.Join(paths, "\n"))
}
