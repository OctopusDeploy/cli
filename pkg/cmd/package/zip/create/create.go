package create

import (
	"errors"
	"fmt"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	pack "github.com/OctopusDeploy/cli/pkg/cmd/package/support"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
)

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := pack.NewPackageCreateFlags()

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create zip",
		Long:  "Create zip package",
		Example: heredoc.Docf(`
			$ %[1]s package zip create --id SomePackage --version 1.0.0
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := pack.NewPackageCreateOptions(f, createFlags, cmd)
			return createRun(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&createFlags.Id.Value, createFlags.Id.Name, "", "The ID of the package.")
	flags.StringVarP(&createFlags.Version.Value, createFlags.Version.Name, "v", "", "The version of the package, must be a valid SemVer.")
	flags.StringVar(&createFlags.BasePath.Value, createFlags.BasePath.Name, "", "Root folder containing the contents to zip.")
	flags.StringVar(&createFlags.OutFolder.Value, createFlags.OutFolder.Name, "", "Folder into which the zip file will be written.")
	flags.StringSliceVar(&createFlags.Include.Value, createFlags.Include.Name, []string{}, "Add a file pattern to include, relative to the base path e.g. /bin/*.dll; defaults to \"**\".")
	flags.BoolVar(&createFlags.Verbose.Value, createFlags.Verbose.Name, false, "Verbose output.")
	flags.BoolVar(&createFlags.Overwrite.Value, createFlags.Overwrite.Name, false, "Allow an existing package file of the same ID/version to be overwritten.")
	flags.SortFlags = false

	return cmd
}

func createRun(cmd *cobra.Command, opts *pack.PackageCreateOptions) error {
	outputFormat, err := cmd.Flags().GetString(constants.FlagOutputFormat)
	if err != nil { // should never happen, but fallback if it does
		outputFormat = constants.OutputFormatTable
	}

	if !opts.NoPrompt {
		if err := pack.PackageCreatePromptMissing(opts); err != nil {
			return err
		}
	}

	if opts.Id.Value == "" {
		return errors.New("must supply a package ID")
	}
	applyDefaultsToUnspecifiedOptions(opts)

	pack.VerboseOut(opts.Writer, opts.Verbose.Value, "Packing \"%s\" version \"%s\"...\n", opts.Id.Value, opts.Version.Value)

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(
			opts.CmdPath,
			opts.Id,
			opts.Version,
			opts.BasePath,
			opts.OutFolder,
			opts.Include,
			opts.Verbose,
			opts.Overwrite,
		)
		fmt.Fprintf(opts.Writer, "\nAutomation Command: %s\n", autoCmd)
	}

	outFilePath := pack.BuildOutFileName("zip", opts.Id.Value, opts.Version.Value)

	zip, err := pack.BuildPackage(opts, outFilePath)
	if zip != nil {
		switch outputFormat {
		case constants.OutputFormatBasic:
			cmd.Printf("%s\n", zip.Name())
		case constants.OutputFormatJson:
			cmd.Printf(`{"Path":"%s"}`, zip.Name())
			cmd.Println()
		default: // table
			cmd.Printf("Successfully created package %s\n", zip.Name())
		}
	}
	return err
}

func applyDefaultsToUnspecifiedOptions(opts *pack.PackageCreateOptions) {
	if opts.Version.Value == "" {
		opts.Version.Value = pack.BuildTimestampSemVer(time.Now())
	}

	if opts.BasePath.Value == "" {
		opts.BasePath.Value = "."
	}

	if opts.OutFolder.Value == "" {
		opts.OutFolder.Value = "."
	}

	if len(opts.Include.Value) == 0 {
		opts.Include.Value = append(opts.Include.Value, "**")
	}
}
