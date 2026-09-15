package machinescommon

import (
	"fmt"
	"math"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/workers"
)

// Machine is the part of a deployment target or worker that enabling and disabling needs.
// Both implementations live in this package, so callers only pick a MachineKind.
type Machine interface {
	ID() string
	Name() string
	IsDisabled() bool
	SetDisabled(disabled bool)
	Save() error
}

// MachineKind adapts one machine repository - deployment targets or workers - to the
// enable/disable flow.
type MachineKind struct {
	// Noun names the machine in prompts and output, e.g. "deployment target".
	Noun string
	// GetAllInState returns the machines the server reports in the given disabled state. The
	// filter is best effort: the query field is omitempty, so only isDisabled=true can be
	// expressed, and callers filter again.
	GetAllInState func(isDisabled bool) ([]Machine, error)
	// GetByIdentifier resolves a single machine by name or ID.
	GetByIdentifier func(idOrName string) (Machine, error)
}

func NewDeploymentTargetKind(dependencies *cmd.Dependencies) *MachineKind {
	return &MachineKind{
		Noun: "deployment target",
		GetAllInState: func(isDisabled bool) ([]Machine, error) {
			res, err := dependencies.Client.Machines.Get(machines.MachinesQuery{IsDisabled: isDisabled, Skip: 0, Take: math.MaxInt32})
			if err != nil {
				return nil, err
			}

			result := make([]Machine, 0, len(res.Items))
			for _, target := range res.Items {
				result = append(result, deploymentTarget{client: dependencies.Client, target: target})
			}
			return result, nil
		},
		GetByIdentifier: func(idOrName string) (Machine, error) {
			target, err := dependencies.Client.Machines.GetByIdentifier(idOrName)
			if err != nil {
				return nil, err
			}
			return deploymentTarget{client: dependencies.Client, target: target}, nil
		},
	}
}

func NewWorkerKind(dependencies *cmd.Dependencies) *MachineKind {
	return &MachineKind{
		Noun: "worker",
		GetAllInState: func(isDisabled bool) ([]Machine, error) {
			res, err := dependencies.Client.Workers.Get(machines.WorkersQuery{IsDisabled: isDisabled, Skip: 0, Take: math.MaxInt32})
			if err != nil {
				return nil, err
			}

			result := make([]Machine, 0, len(res.Items))
			for _, w := range res.Items {
				result = append(result, worker{client: dependencies.Client, worker: w})
			}
			return result, nil
		},
		GetByIdentifier: func(idOrName string) (Machine, error) {
			w, err := dependencies.Client.Workers.GetByIdentifier(idOrName)
			if err != nil {
				return nil, err
			}
			return worker{client: dependencies.Client, worker: w}, nil
		},
	}
}

type deploymentTarget struct {
	client *client.Client
	target *machines.DeploymentTarget
}

func (m deploymentTarget) ID() string                { return m.target.GetID() }
func (m deploymentTarget) Name() string              { return m.target.Name }
func (m deploymentTarget) IsDisabled() bool          { return m.target.IsDisabled }
func (m deploymentTarget) SetDisabled(disabled bool) { m.target.IsDisabled = disabled }

func (m deploymentTarget) Save() error {
	_, err := machines.Update(m.client, m.target)
	return err
}

type worker struct {
	client *client.Client
	worker *machines.Worker
}

func (m worker) ID() string                { return m.worker.GetID() }
func (m worker) Name() string              { return m.worker.Name }
func (m worker) IsDisabled() bool          { return m.worker.IsDisabled }
func (m worker) SetDisabled(disabled bool) { m.worker.IsDisabled = disabled }

func (m worker) Save() error {
	_, err := workers.Update(m.client, m.worker)
	return err
}

type SetDisabledStateOptions struct {
	*cmd.Dependencies
	*MachineKind
	IdOrName string
	// Disabled is the state the machine should end up in.
	Disabled bool
	// Machine is the machine chosen at the prompt. When set it is used directly, saving a round
	// trip back to the server for something we already have.
	Machine Machine
}

func NewSetDisabledStateOptions(args []string, dependencies *cmd.Dependencies, kind *MachineKind, disabled bool) *SetDisabledStateOptions {
	idOrName := ""
	if len(args) > 0 {
		idOrName = args[0]
	}

	return &SetDisabledStateOptions{
		Dependencies: dependencies,
		MachineKind:  kind,
		IdOrName:     idOrName,
		Disabled:     disabled,
	}
}

// SetDisabledState enables or disables a deployment target or worker, prompting for it when no
// name or ID was supplied.
func SetDisabledState(opts *SetDisabledStateOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissingMachine(opts); err != nil {
			return err
		}
	}

	machine := opts.Machine
	if machine == nil {
		if opts.IdOrName == "" {
			return fmt.Errorf("%s identifier is required but was not provided", opts.Noun)
		}

		var err error
		if machine, err = opts.GetByIdentifier(opts.IdOrName); err != nil {
			return err
		}
	}

	state := disabledStateDescription(opts.Disabled)
	if machine.IsDisabled() == opts.Disabled {
		_, _ = fmt.Fprintf(opts.Out, "%s '%s' %s is already %s.\n", capitalise(opts.Noun), machine.Name(), output.Dimf("(%s)", machine.ID()), state)
		return nil
	}

	machine.SetDisabled(opts.Disabled)
	if err := machine.Save(); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(opts.Out, "Successfully %s %s '%s' %s.\n", state, opts.Noun, machine.Name(), output.Dimf("(%s)", machine.ID()))
	return nil
}

func PromptMissingMachine(opts *SetDisabledStateOptions) error {
	if opts.IdOrName != "" {
		return nil
	}

	// Only machines that aren't already in the requested state are worth offering.
	machineList, err := opts.GetAllInState(!opts.Disabled)
	if err != nil {
		return err
	}

	// The server-side filter isn't guaranteed (older servers, and the disable case can't express
	// it), so drop anything already in the requested state here as well.
	candidates := make([]Machine, 0, len(machineList))
	for _, machine := range machineList {
		if machine.IsDisabled() != opts.Disabled {
			candidates = append(candidates, machine)
		}
	}

	if len(candidates) == 0 {
		return fmt.Errorf("no %ss to %s were found", opts.Noun, actionDescription(opts.Disabled))
	}

	// deliberately not selectors.Select: that auto-selects when there is exactly one machine,
	// which would mutate it without the user ever being asked. Enable/disable always asks.
	selected, err := question.SelectMap(
		opts.Ask,
		fmt.Sprintf("Select the %s you wish to %s:", opts.Noun, actionDescription(opts.Disabled)),
		candidates,
		func(machine Machine) string { return machine.Name() })
	if err != nil {
		return err
	}

	opts.Machine = selected
	opts.IdOrName = selected.ID()
	return nil
}

func actionDescription(isDisabled bool) string {
	if isDisabled {
		return "disable"
	}
	return "enable"
}

func disabledStateDescription(isDisabled bool) string {
	if isDisabled {
		return "disabled"
	}
	return "enabled"
}

func capitalise(noun string) string {
	return strings.ToUpper(noun[:1]) + noun[1:]
}
