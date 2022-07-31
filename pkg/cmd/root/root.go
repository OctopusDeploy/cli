package root

import (
	accountCmd "github.com/OctopusDeploy/cli/pkg/cmd/account"
	environmentCmd "github.com/OctopusDeploy/cli/pkg/cmd/environment"
	releaseCmd "github.com/OctopusDeploy/cli/pkg/cmd/release"
	spaceCmd "github.com/OctopusDeploy/cli/pkg/cmd/space"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	FlagHelp               = "help"
	FlagSpace              = "space"
	FlagOutputFormat       = "output-format"
	flagOutputFormatLegacy = "outputFormat"
	FlagNoPrompt           = "no-prompt"
)

// NewCmdRoot returns the base command when called without any subcommands
func NewCmdRoot(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "octopus",
		Short: "Octopus Deploy CLI",
		Long:  `Work seamlessly with Octopus Deploy from the command line.`,
	}

	cmd.PersistentFlags().BoolP(FlagHelp, "h", false, "Show help for command")
	cmd.PersistentFlags().StringP(FlagSpace, "s", "", "Set Space")
	cmd.PersistentFlags().StringP(FlagOutputFormat, "f", "", "Output Format (Valid values are 'json', 'table', 'basic'. Defaults to table)")
	cmd.PersistentFlags().BoolP(FlagNoPrompt, "", false, "disable prompting in interactive mode")

	// translate flags inherited from .NET CLI
	cmd.PersistentFlags().SetNormalizeFunc(func(f *pflag.FlagSet, name string) pflag.NormalizedName {
		switch name {
		case flagOutputFormatLegacy:
			name = FlagOutputFormat
			break
		}
		return pflag.NormalizedName(name)
	})

	// infrastructure commands
	cmd.AddCommand(accountCmd.NewCmdAccount(f))
	cmd.AddCommand(environmentCmd.NewCmdEnvironment(f))

	// configuration commands
	cmd.AddCommand(spaceCmd.NewCmdSpace(f))

	cmd.AddCommand(releaseCmd.NewCmdRelease(f))

	return cmd
}
