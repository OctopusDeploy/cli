package deprovision_environment

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/util"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/ephemeralenvironments"
	"github.com/spf13/cobra"
)

const (
	FlagName = "name"
)

type DeprovisionEnvironmentFlags struct {
	Name *flag.Flag[string]
}

func NewDeprovisionEnvironmentFlags() *DeprovisionEnvironmentFlags {
	return &DeprovisionEnvironmentFlags{
		Name: flag.New[string](FlagName, false),
	}
}

type DeprovisionEnvironmentOptions struct {
	*DeprovisionEnvironmentFlags
	*cmd.Dependencies
	Command                     *cobra.Command
	GetAllEphemeralEnvironments func() ([]*ephemeralenvironments.EphemeralEnvironment, error)
}

func NewDeprovisionEnvironmentOptions(deprovisionEnvironmentFlags *DeprovisionEnvironmentFlags, dependencies *cmd.Dependencies, command *cobra.Command) *DeprovisionEnvironmentOptions {
	return &DeprovisionEnvironmentOptions{
		DeprovisionEnvironmentFlags: deprovisionEnvironmentFlags,
		Dependencies:                dependencies,
		Command:                     command,
		GetAllEphemeralEnvironments: func() ([]*ephemeralenvironments.EphemeralEnvironment, error) {
			return getAllEphemeralEnvironments(dependencies)
		},
	}
}

func getAllEphemeralEnvironments(dependencies *cmd.Dependencies) ([]*ephemeralenvironments.EphemeralEnvironment, error) {
	allEphemeralEnvironments, err := ephemeralenvironments.GetAll(dependencies.Client, dependencies.Client.GetSpaceID())

	if err != nil {
		return nil, err
	}

	return allEphemeralEnvironments.Items, nil
}

func NewCmdDeprovisionEnvironment(factory factory.Factory) *cobra.Command {
	deprovisionEnvironmentFlags := NewDeprovisionEnvironmentFlags()

	command := &cobra.Command{
		Use:     "deprovision-environment",
		Short:   "Deprovision an environment",
		Long:    "Deprovision an environment",
		Example: heredoc.Docf("$ %s ephemeral-environment deprovision-environment --name PR-1234", constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			deprovisionEnvironmentOptions := NewDeprovisionEnvironmentOptions(deprovisionEnvironmentFlags, cmd.NewDependencies(factory, c), c)

			return DeprovisionEnvironmentRun(deprovisionEnvironmentOptions, c)
		},
	}

	flags := command.Flags()
	flags.StringVarP(&deprovisionEnvironmentFlags.Name.Value, FlagName, "n", "", "Name of the environment")

	return command
}

func DeprovisionEnvironmentRun(deprovisionEnvironmentOptions *DeprovisionEnvironmentOptions, command *cobra.Command) error {
	var err error

	if !deprovisionEnvironmentOptions.NoPrompt {
		err = PromptMissing(deprovisionEnvironmentOptions)
		if err != nil {
			return err
		}
	}

	if deprovisionEnvironmentOptions.Name.Value == "" {
		return fmt.Errorf("environment name is required")
	}

	var environment *ephemeralenvironments.EphemeralEnvironment

	environment, err = util.GetByName(deprovisionEnvironmentOptions.Client, deprovisionEnvironmentOptions.Name.Value, deprovisionEnvironmentOptions.Space.ID)
	if err != nil {
		return err
	}

	var response *ephemeralenvironments.DeprovisionEphemeralEnvironmentResponse

	response, err = deprovisionEnvironment(deprovisionEnvironmentOptions, environment)
	if err != nil {
		return err
	}

	util.OutPutDeprovisionResult(deprovisionEnvironmentOptions.Name.Value, command, response.DeprovisioningRuns)

	return nil
}

func deprovisionEnvironment(deprovisionEnvironmentOptions *DeprovisionEnvironmentOptions, environment *ephemeralenvironments.EphemeralEnvironment) (*ephemeralenvironments.DeprovisionEphemeralEnvironmentResponse, error) {
	response, err := ephemeralenvironments.Deprovision(deprovisionEnvironmentOptions.Client, deprovisionEnvironmentOptions.Space.ID, environment.ID)

	if err != nil {
		return nil, err
	}

	return response, nil
}

func PromptMissing(options *DeprovisionEnvironmentOptions) error {
	if options.Name.Value != "" {
		return nil
	}

	environment, err := selectors.Select(options.Ask, "Please select the name of the environment you wish to deprovision", options.GetAllEphemeralEnvironments, func(environment *ephemeralenvironments.EphemeralEnvironment) string { return environment.Name })

	if err != nil {
		return err
	}

	options.Name.Value = environment.Name

	return nil
}
