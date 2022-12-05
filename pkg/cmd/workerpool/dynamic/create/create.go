package create

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"github.com/spf13/cobra"
)

const (
	FlagName        = "name"
	FlagDescription = "description"
	FlagWorkerType  = "worker-type"
)

type GetDynamicWorkerPoolTypes func() ([]*workerpools.DynamicWorkerPoolType, error)

type CreateFlags struct {
	Name        *flag.Flag[string]
	Type        *flag.Flag[string]
	Description *flag.Flag[string]
	*machinescommon.WebFlags
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:        flag.New[string](FlagName, false),
		Type:        flag.New[string](FlagWorkerType, false),
		Description: flag.New[string](FlagDescription, false),
		WebFlags:    machinescommon.NewWebFlags(),
	}
}

type CreateOptions struct {
	*CreateFlags
	*cmd.Dependencies
	GetDynamicWorkerPoolTypes
}

func NewCreateOptions(flags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:  flags,
		Dependencies: dependencies,
		GetDynamicWorkerPoolTypes: func() ([]*workerpools.DynamicWorkerPoolType, error) {
			return getDynamicWorkerPoolTypes(dependencies.Client)
		},
	}
}

func getDynamicWorkerPoolTypes(client *client.Client) ([]*workerpools.DynamicWorkerPoolType, error) {
	return client.WorkerPools.GetDynamicWorkerTypes()
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a dynamic worker pool",
		Long:  "Create a dynamic worker pool in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %s worker-pool dynamic create
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "A short, memorable, unique name for this worker pool.")
	flags.StringVar(&createFlags.Type.Value, createFlags.Type.Name, "", "The worker type to use for all leased workers.")
	flags.StringVar(&createFlags.Description.Value, createFlags.Description.Name, "", "Description of the worker pool.")

	machinescommon.RegisterWebFlag(cmd, createFlags.WebFlags)

	return cmd
}

func createRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	workerPool := workerpools.NewDynamicWorkerPool(opts.Name.Value, opts.Type.Value)

	if opts.Description.Value != "" {
		workerPool.SetDescription(opts.Description.Value)
	}

	createdPool, err := opts.Client.WorkerPools.Add(workerPool)
	if err != nil {
		return err
	}

	fmt.Fprintf(opts.Out, "Successfully created worker pool '%s'\n", createdPool.GetName())
	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.Description, opts.Type)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	machinescommon.DoWebForWorkerPools(createdPool, opts.Dependencies, opts.WebFlags)

	return nil
}

func PromptMissing(opts *CreateOptions) error {
	err := question.AskName(opts.Ask, "", "Dynamic Worker Pool", &opts.Name.Value)
	if err != nil {
		return err
	}

	err = question.AskDescription(opts.Ask, "", "Dynamic Worker Pool", &opts.Description.Value)
	if err != nil {
		return err
	}

	if opts.Type.Value == "" {
		selectedOption, err := selectors.Select(opts.Ask, "Select the worker type to use", opts.GetDynamicWorkerPoolTypes, func(p *workerpools.DynamicWorkerPoolType) string {
			return fmt.Sprintf("%s (%s)", p.Description, p.Type)
		})
		if err != nil {
			return err
		}

		opts.Type.Value = selectedOption.ID
	}

	return nil
}
