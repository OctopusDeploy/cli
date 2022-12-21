package create

import (
	"errors"
	"github.com/MakeNowJust/heredoc/v2"
	pack "github.com/OctopusDeploy/cli/pkg/cmd/package/support"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
	"time"
)

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := pack.NewPackageCreateFlags()

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create zip",
		Long:  "Create zip package",
		Example: heredoc.Docf(`
			$ %[1]s project zip create --id SomePackage --version 1.0.0
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := pack.NewPackageCreateOptions(f, createFlags, cmd)
			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&createFlags.Id.Value, createFlags.Id.Name, "", "The ID of the package")
	flags.StringVarP(&createFlags.Version.Value, createFlags.Version.Name, "v", "", "The version of the package; must be a valid SemVer; defaults to a timestamp-based version")
	flags.StringVar(&createFlags.BasePath.Value, createFlags.BasePath.Name, ".", "Root folder containing the contents to zip")
	flags.StringVar(&createFlags.OutFolder.Value, createFlags.OutFolder.Name, ".", "Folder into which the zip file will be written")
	flags.StringSliceVar(&createFlags.Include.Value, createFlags.Include.Name, []string{"**"}, "Add a file pattern to include, relative to the base path e.g. /bin/*.dll")
	flags.BoolVar(&createFlags.Verbose.Value, createFlags.Verbose.Name, false, "Verbose output")
	flags.BoolVar(&createFlags.Overwrite.Value, createFlags.Overwrite.Name, false, "Allow an existing package file of the same ID/version to be overwritten")
	flags.SortFlags = false

	return cmd
}

func createRun(opts *pack.PackageCreateOptions) error {
	//if !opts.NoPrompt {
	//	if err := PromptMissing(opts); err != nil {
	//		return err
	//	}
	//}

	if opts.Id.Value == "" {
		return errors.New("must supply a package ID")
	}

	if opts.Version.Value == "" {
		opts.Version.Value = pack.BuildTimestampSemVer(time.Now())
	}

	pack.VerboseOut(opts.Verbose.Value, "Packing \"%s\" version \"%s\"...\n", opts.Id.Value, opts.Version.Value)

	outFilePath := pack.BuildOutFileName("zip", opts.Id.Value, opts.Version.Value)
	return pack.BuildPackage(opts, outFilePath)
}
