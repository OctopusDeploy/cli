package shared

import (
	"errors"
	"fmt"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
)

type SetDisabledStateOptions struct {
	*cmd.Dependencies
	*GetTargetsOptions
	IdOrName string
	// Disabled is the state the deployment target should end up in.
	Disabled bool
	// Target is the deployment target chosen at the prompt. When set it is used directly, saving a
	// round trip back to the server for something we already have.
	Target *machines.DeploymentTarget
}

func NewSetDisabledStateOptions(args []string, dependencies *cmd.Dependencies, disabled bool) *SetDisabledStateOptions {
	idOrName := ""
	if len(args) > 0 {
		idOrName = args[0]
	}

	// Only targets that aren't already in the requested state are worth offering. The machines
	// endpoint can filter server-side for the enable case (the query field is omitempty, so only
	// isDisabled=true can be expressed); the disable case is filtered client-side below.
	query := machines.MachinesQuery{IsDisabled: !disabled}

	return &SetDisabledStateOptions{
		Dependencies:      dependencies,
		GetTargetsOptions: NewGetTargetsOptions(dependencies, query),
		IdOrName:          idOrName,
		Disabled:          disabled,
	}
}

// SetDisabledState enables or disables a deployment target, prompting for the target when no
// name or ID was supplied.
func SetDisabledState(opts *SetDisabledStateOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissingTarget(opts); err != nil {
			return err
		}
	}

	target := opts.Target
	if target == nil {
		if opts.IdOrName == "" {
			return errors.New("deployment target identifier is required but was not provided")
		}

		var err error
		if target, err = opts.Client.Machines.GetByIdentifier(opts.IdOrName); err != nil {
			return err
		}
	}

	state := disabledStateDescription(opts.Disabled)
	if target.IsDisabled == opts.Disabled {
		_, _ = fmt.Fprintf(opts.Out, "Deployment target '%s' %s is already %s.\n", target.Name, output.Dimf("(%s)", target.GetID()), state)
		return nil
	}

	target.IsDisabled = opts.Disabled
	if _, err := machines.Update(opts.Client, target); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(opts.Out, "Successfully %s deployment target '%s' %s.\n", state, target.Name, output.Dimf("(%s)", target.GetID()))
	return nil
}

func PromptMissingTarget(opts *SetDisabledStateOptions) error {
	if opts.IdOrName != "" {
		return nil
	}

	targets, err := opts.GetTargetsCallback()
	if err != nil {
		return err
	}

	// The server-side filter isn't guaranteed (older servers, and the disable case can't express
	// it), so drop anything already in the requested state here as well.
	candidates := make([]*machines.DeploymentTarget, 0, len(targets))
	for _, target := range targets {
		if target.IsDisabled != opts.Disabled {
			candidates = append(candidates, target)
		}
	}

	if len(candidates) == 0 {
		return fmt.Errorf("no deployment targets to %s were found", actionDescription(opts.Disabled))
	}

	// deliberately not selectors.Select: that auto-selects when there is exactly one target, which
	// would mutate the target without the user ever being asked. Enable/disable always asks.
	selectedTarget, err := question.SelectMap(
		opts.Ask,
		fmt.Sprintf("Select the deployment target you wish to %s:", actionDescription(opts.Disabled)),
		candidates,
		func(target *machines.DeploymentTarget) string { return target.Name })
	if err != nil {
		return err
	}

	opts.Target = selectedTarget
	opts.IdOrName = selectedTarget.GetID()
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
