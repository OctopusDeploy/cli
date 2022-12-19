package set

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
)

const (
	FlagName= "name"
	FlagValue= "value"
	FlagScope="scope"
	FlagType="type"
	FlagDescription="description"

	TypeText = "text"
	TypeSensitive="sensitive"
)

type SetFlags struct {
	Name *flag.Flag[string]
	Description *flag.Flag[string]
	Value *flag.Flag[string]

	Type *flag.Flag[string]
	Scopes *flag.Flag[[]string]
}

type SetOptions struct {
	*SetFlags
	*cmd.Dependencies
}

func NewSetFlags() *SetFlags{
	return &SetFlags{
		Name:   flag.New[string](FlagName, false),
		Value:  flag.New[string](FlagValue, false),
		Description: flag.New[string](FlagDescription, false),
		Type:flag.New[string](FlagType, false),
		Scopes: flag.New[[]string](FlagScope, false),
	}
}

func NewSetOptions(flags *SetFlags, dependencies *cmd.Dependencies) *SetOptions {
	return &SetOptions{
		SetFlags:     flags,
		Dependencies: dependencies,
	}
}

func NewSetCmd(f factory.Factory) *cobra.Command {
	setFlags := NewSetFlags()
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set project variables",
		Long:  "Set project variables in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project variable set
			$ %[1]s project variable set --name varname --value "abc"
			$ %[1]s project variable set --name varname --value "passwordABC" --type sensitive
			$ %[1]s project variable set --name varname --value "abc" --scope environment='test'
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewSetOptions(setFlags, cmd.NewDependencies(f,c))
			if opts.Type.Value == TypeSensitive {
				opts.Value.Secure = true
			}

			return setRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&setFlags.Name.Value, setFlags.Name.Name, "n", "", "The name of the variable")
	flags.StringVarP(&setFlags.Type.Value, setFlags.Type.Name, "t", "text", "The type of variable. Valid values are 'text', 'sensitive'. Default is 'text'")
	flags.StringVar(&setFlags.Value.Value, setFlags.Value.Name, "", "The value to set on the variable")
	flags.StringSliceVar(&setFlags.Scopes.Value, setFlags.Scopes.Name, []string{}, "Assign scopes to the variable. Valid scopes are: environment, role, target, process, step, channel. Multiple scopes can be supplied.")

	return cmd
}

func setRun(opts *SetOptions) error {
	if !opts.NoPrompt {
		err := PromptMissing(opts)
		if err != nil {
			return err
		}
	}

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.Value, opts.Description, opts.Type, opts.Scopes)
		fmt.Fprintf(opts.Out, "%s\n", autoCmd)
	}

	return nil
}

func PromptMissing(opts *SetOptions) error {
	if opts.Name.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Name",
			Help:    fmt.Sprintf("A name for this variable."),
		}, &opts.Name.Value, survey.WithValidator(survey.ComposeValidators(
			survey.MaxLength(200),
			survey.MinLength(1),
			survey.Required,
		))); err != nil {
			return err
		}
	}
	return nil
}