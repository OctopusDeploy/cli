package shared

import (
	"errors"
	"fmt"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
)

type SetDisabledStateOptions struct {
	*cmd.Dependencies
	*GetTargetsOptions
	IdOrName string
}

func NewSetDisabledStateOptions(args []string, dependencies *cmd.Dependencies) *SetDisabledStateOptions {
	idOrName := ""
	if len(args) > 0 {
		idOrName = args[0]
	}

	return &SetDisabledStateOptions{
		Dependencies:      dependencies,
		GetTargetsOptions: NewGetTargetsOptionsForAllTargets(dependencies),
		IdOrName:          idOrName,
	}
}

// SetDisabledState enables or disables a deployment target, prompting for the target when no
// name or ID was supplied.
func SetDisabledState(opts *SetDisabledStateOptions, isDisabled bool) error {
	if !opts.NoPrompt {
		if err := PromptMissingTarget(opts, isDisabled); err != nil {
			return err
		}
	}

	if opts.IdOrName == "" {
		return errors.New("deployment target identifier is required but was not provided")
	}

	target, err := opts.Client.Machines.GetByIdentifier(opts.IdOrName)
	if err != nil {
		return err
	}

	state := disabledStateDescription(isDisabled)
	if target.IsDisabled == isDisabled {
		_, _ = fmt.Fprintf(opts.Out, "Deployment target '%s' %s is already %s.\n", target.Name, output.Dimf("(%s)", target.GetID()), state)
		return nil
	}

	target.IsDisabled = isDisabled
	if _, err = machines.Update(opts.Client, target); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(opts.Out, "Successfully %s deployment target '%s' %s.\n", state, target.Name, output.Dimf("(%s)", target.GetID()))
	return nil
}

func PromptMissingTarget(opts *SetDisabledStateOptions, isDisabled bool) error {
	if opts.IdOrName != "" {
		return nil
	}

	selectedTarget, err := selectors.Select(
		opts.Ask,
		fmt.Sprintf("Select the deployment target you wish to %s:", actionDescription(isDisabled)),
		opts.GetTargetsCallback,
		func(target *machines.DeploymentTarget) string { return target.Name })
	if err != nil {
		return err
	}

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
