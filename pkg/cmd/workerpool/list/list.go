package list

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/workerpool/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"github.com/spf13/cobra"
)

type ListOptions struct {
	*shared.GetWorkerPoolsOptions
	*cobra.Command
	*cmd.Dependencies
}

func NewListOptions(dependencies *cmd.Dependencies, command *cobra.Command) *ListOptions {
	return &ListOptions{
		GetWorkerPoolsOptions: shared.NewGetWorkerPoolsOptions(dependencies),
		Command:               command,
		Dependencies:          dependencies,
	}
}

func NewCmdList(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List worker pools",
		Long:    "List worker pools in Octopus Deploy",
		Aliases: []string{"ls"},
		Example: heredoc.Docf("$ %s worker-pool list", constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			return ListRun(NewListOptions(cmd.NewDependencies(f, c), c))
		},
	}

	return cmd
}

func ListRun(opts *ListOptions) error {
	allPools, err := opts.GetWorkerPoolsCallback()
	if err != nil {
		return err
	}

	type TargetAsJson struct {
		Id   string `json:"Id"`
		Name string `json:"Name"`
		Type string `json:"Type"`
		Slug string `json:"Slug"`
	}

	return output.PrintArray(allPools, opts.Command, output.Mappers[*workerpools.WorkerPoolListResult]{
		Json: func(item *workerpools.WorkerPoolListResult) any {

			return TargetAsJson{
				Id:   item.ID,
				Name: item.Name,
				Type: string(item.WorkerPoolType),
				Slug: item.Slug,
			}
		},
		Table: output.TableDefinition[*workerpools.WorkerPoolListResult]{
			Header: []string{"NAME", "TYPE", "SLUG"},
			Row: func(item *workerpools.WorkerPoolListResult) []string {
				return []string{output.Bold(item.Name), string(item.WorkerPoolType), output.Dim(item.Slug)}
			},
		},
		Basic: func(item *workerpools.WorkerPoolListResult) string {
			return item.Name
		},
	})
}
