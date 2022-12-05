package create

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/machinescommon"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workerpools"
	"github.com/spf13/cobra"
)

const (
	FlagName        = "name"
	FlagDescription = "description"
)

type CreateFlags struct {
	Name        *flag.Flag[string]
	Description *flag.Flag[string]
	*machinescommon.WebFlags
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:        flag.New[string](FlagName, false),
		Description: flag.New[string](FlagDescription, false),
		WebFlags:    machinescommon.NewWebFlags(),
	}
}

type CreateOptions struct {
	*CreateFlags
	*cmd.Dependencies
}

func NewCreateOptions(flags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:  flags,
		Dependencies: dependencies,
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a static worker pool",
		Long:  "Create a static worker pool in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %s worker-pool static create
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "A short, memorable, unique name for this worker pool.")
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

	workerPool := workerpools.NewStaticWorkerPool(opts.Name.Value)

	if opts.Description.Value != "" {
		workerPool.SetDescription(opts.Description.Value)
	}

	createdPool, err := opts.Client.WorkerPools.Add(workerPool)
	if err != nil {
		return err
	}

	fmt.Fprintf(opts.Out, "Successfully created worker pool '%s'\n", createdPool.GetName())
	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.Description)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	machinescommon.DoWebForWorkerPools(createdPool, opts.Dependencies, opts.WebFlags)

	return nil
}

func PromptMissing(opts *CreateOptions) error {
	err := question.AskName(opts.Ask, "", "Static Worker Pool", &opts.Name.Value)
	if err != nil {
		return err
	}

	err = question.AskDescription(opts.Ask, "", "Static Worker Pool", &opts.Description.Value)
	if err != nil {
		return err
	}

	return nil
}
