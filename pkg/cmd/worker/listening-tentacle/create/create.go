package create

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	targetShared "github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	workerShared "github.com/OctopusDeploy/cli/pkg/cmd/worker/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/spf13/cobra"
	"net/url"
)

const (
	FlagName       = "name"
	FlagThumbprint = "thumbprint"
	FlagUrl        = "url"
)

type CreateFlags struct {
	Name       *flag.Flag[string]
	Thumbprint *flag.Flag[string]
	URL        *flag.Flag[string]
	*targetShared.CreateTargetProxyFlags
	*targetShared.CreateTargetMachinePolicyFlags
	*workerShared.WorkerPoolFlags
	*targetShared.WebFlags
}

type CreateOptions struct {
	*CreateFlags
	*targetShared.CreateTargetProxyOptions
	*targetShared.CreateTargetMachinePolicyOptions
	*workerShared.WorkerPoolOptions
	*cmd.Dependencies
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:                           flag.New[string](FlagName, false),
		Thumbprint:                     flag.New[string](FlagThumbprint, true),
		URL:                            flag.New[string](FlagUrl, false),
		CreateTargetProxyFlags:         targetShared.NewCreateTargetProxyFlags(),
		CreateTargetMachinePolicyFlags: targetShared.NewCreateTargetMachinePolicyFlags(),
		WorkerPoolFlags:                workerShared.NewWorkerPoolFlags(),
		WebFlags:                       targetShared.NewWebFlags(),
	}
}

func NewCreateOptions(createFlags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:                      createFlags,
		Dependencies:                     dependencies,
		CreateTargetProxyOptions:         targetShared.NewCreateTargetProxyOptions(dependencies),
		CreateTargetMachinePolicyOptions: targetShared.NewCreateTargetMachinePolicyOptions(dependencies),
		WorkerPoolOptions:                workerShared.NewWorkerPoolOptionsForCreateWorker(dependencies),
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a listening tentacle worker",
		Long:  "Create a listening tentacle worker in Octopus Deploy",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s worker listening-tentacle create
		`), constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "A short, memorable, unique name for this Listening Tentacle worker.")
	flags.StringVar(&createFlags.Thumbprint.Value, createFlags.Thumbprint.Name, "", "The X509 certificate thumbprint that securely identifies the Tentacle.")
	flags.StringVar(&createFlags.URL.Value, createFlags.URL.Name, "", "The network address at which the Tentacle can be reached.")
	targetShared.RegisterCreateTargetProxyFlags(cmd, createFlags.CreateTargetProxyFlags)
	targetShared.RegisterCreateTargetMachinePolicyFlags(cmd, createFlags.CreateTargetMachinePolicyFlags)
	workerShared.RegisterCreateWorkerWorkerPoolFlags(cmd, createFlags.WorkerPoolFlags)
	targetShared.RegisterWebFlag(cmd, createFlags.WebFlags)

	return cmd
}

func createRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	url, err := url.Parse(opts.URL.Value)
	if err != nil {
		return err
	}

	endpoint := machines.NewListeningTentacleEndpoint(url, opts.Thumbprint.Value)
	if opts.Proxy.Value != "" {
		proxy, err := targetShared.FindProxy(opts.CreateTargetProxyOptions, opts.CreateTargetProxyFlags)
		if err != nil {
			return err
		}
		endpoint.ProxyID = proxy.GetID()
	}

	workerPoolIds, err := workerShared.FindWorkerPoolIds(opts.WorkerPoolOptions, opts.WorkerPoolFlags)
	if err != nil {
		return err
	}

	worker := machines.NewWorker(opts.Name.Value, endpoint)
	worker.WorkerPoolIDs = workerPoolIds
	machinePolicy, err := targetShared.FindMachinePolicy(opts.GetAllMachinePoliciesCallback, opts.MachinePolicy.Value)
	if err != nil {
		return err
	}
	worker.MachinePolicyID = machinePolicy.GetID()

	createdWorker, err := opts.Client.Workers.Add(worker)
	if err != nil {
		return err
	}

	fmt.Fprintf(opts.Out, "Successfully created listening tentcle worker '%s'.\n", worker.Name)
	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.URL, opts.Thumbprint, opts.Proxy, opts.MachinePolicy, opts.WorkerPools)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	targetShared.DoWebForWorkers(createdWorker, opts.Dependencies, opts.WebFlags, "listening tentacle worker")

	return nil
}

func PromptMissing(opts *CreateOptions) error {
	err := question.AskName(opts.Ask, "", "Listening Tentacle", &opts.Name.Value)
	if err != nil {
		return err
	}

	err = workerShared.PromptForWorkerPools(opts.WorkerPoolOptions, opts.WorkerPoolFlags)
	if err != nil {
		return err
	}

	err = targetShared.PromptForMachinePolicy(opts.CreateTargetMachinePolicyOptions, opts.CreateTargetMachinePolicyFlags)
	if err != nil {
		return err
	}

	if opts.Thumbprint.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Thumbprint",
			Help:    "The X509 certificate thumbprint that securely identifies the Tentacle.",
		}, &opts.Thumbprint.Value, survey.WithValidator(survey.ComposeValidators(
			survey.MinLength(40),
			survey.MaxLength(40),
		))); err != nil {
			return err
		}
	}

	if opts.URL.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "URL",
			Help:    "The network address at which the Tentacle can be reached.",
		}, &opts.URL.Value, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	err = targetShared.PromptForProxy(opts.CreateTargetProxyOptions, opts.CreateTargetProxyFlags)
	if err != nil {
		return err
	}

	return nil
}
