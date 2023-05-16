package upload

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/ztrue/tracerr"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/newclient"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/packages"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/cobra"
)

const (
	FlagPackage = "package"

	FlagOverwriteMode      = "overwrite-mode"
	FlagAliasOverwrite     = "overwrite"
	FlagAliasOverwriteMode = "overwritemode" // I keep forgetting the hyphen

	// replace-existing deprected in the .NET CLI so not brought across
	FlagUseDeltaCompression = "use-delta-compression" // this is not yet supported, but will be in future when we implement OctoDiff in go

	FlagContinueOnError = "continue-on-error"
)

type UploadFlags struct {
	Package             *flag.Flag[[]string]
	OverwriteMode       *flag.Flag[string]
	UseDeltaCompression *flag.Flag[bool]
	ContinueOnError     *flag.Flag[bool]
}

func NewUploadFlags() *UploadFlags {
	return &UploadFlags{
		Package:             flag.New[[]string](FlagPackage, false),
		OverwriteMode:       flag.New[string](FlagOverwriteMode, false),
		UseDeltaCompression: flag.New[bool](FlagUseDeltaCompression, false),
		ContinueOnError:     flag.New[bool](FlagContinueOnError, false),
	}
}

func NewCmdUpload(f factory.Factory) *cobra.Command {
	uploadFlags := NewUploadFlags()
	cmd := &cobra.Command{
		Use:     "upload",
		Short:   "upload one or more packages to Octopus Deploy",
		Long:    "upload one or more packages to Octopus Deploy. Glob patterns are supported.",
		Aliases: []string{"push"},
		Example: heredoc.Docf(`
			$ %[1]s package upload --package SomePackage.1.0.0.zip
			$ %[1]s package upload SomePackage.1.0.0.tar.gz --overwrite-mode overwrite
			$ %[1]s package push SomePackage.1.0.0.zip	
			$ %[1]s package upload bin/**/*.zip --continue-on-error
			$ %[1]s package upload PkgA.1.0.0.zip PkgB.2.0.0.tar.gz PkgC.1.0.0.nupkg
		`, constants.ExecutableName),
		Annotations: map[string]string{annotations.IsCore: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// any bare args are assumed to be packages to upload
			for _, arg := range args {
				uploadFlags.Package.Value = append(uploadFlags.Package.Value, arg)
			}
			return uploadRun(cmd, f, uploadFlags)
		},
	}

	flags := cmd.Flags()
	flags.StringSliceVarP(&uploadFlags.Package.Value, uploadFlags.Package.Name, "p", nil, "Package to upload, may be specified multiple times. Any arguments without flags will be treated as packages")
	flags.StringVarP(&uploadFlags.OverwriteMode.Value, uploadFlags.OverwriteMode.Name, "", "", "Action when a package already exists. Valid values are 'fail', 'overwrite', 'ignore'. Default is 'fail'")
	flags.BoolVarP(&uploadFlags.ContinueOnError.Value, uploadFlags.ContinueOnError.Name, "", false, "When uploading multiple packages, controls whether the CLI continues after a failed upload. Default is to abort.")
	flags.SortFlags = false

	flagAliases := make(map[string][]string, 1)
	util.AddFlagAliasesString(flags, FlagOverwriteMode, flagAliases, FlagAliasOverwrite, FlagAliasOverwriteMode)

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		util.ApplyFlagAliases(cmd.Flags(), flagAliases)
		return nil
	}
	return cmd
}

type uploadSucceededViewModel struct {
	PackagePath string `json:"package,omitempty"`
}
type uploadFailedViewModel struct {
	PackagePath string `json:"package,omitempty"`
	Error       string `json:"error,omitempty"`
}
type uploadViewModel struct {
	Succeeded []uploadSucceededViewModel `json:"succeeded,omitempty"`
	Failed    []uploadFailedViewModel    `json:"failed,omitempty"`
}

func uploadRun(cmd *cobra.Command, f factory.Factory, flags *UploadFlags) error {
	outputFormat, err := cmd.Flags().GetString(constants.FlagOutputFormat)
	if err != nil { // should never happen, but fallback if it does
		outputFormat = constants.OutputFormatTable
	}

	// package upload doesn't have interactive mode, so we don't care about the question.asker
	octopus, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}
	// core infrastructure will ask the user for a space interactively, should it need to
	space := f.GetCurrentSpace()
	if space == nil {
		return errors.New("package upload must run with a configured space")
	}

	continueOnError := flags.ContinueOnError.Value

	overwriteMode := flags.OverwriteMode.Value
	resolvedOverwriteMode := packages.OverwriteMode("")
	switch strings.ToLower(overwriteMode) {
	case "fail", "failifexists", "": // include aliases from old CLI, default (empty string) = fail
		resolvedOverwriteMode = packages.OverwriteModeFailIfExists
	case "ignore", "ignoreifexists":
		resolvedOverwriteMode = packages.OverwriteModeIgnoreIfExists
	case "overwrite", "overwriteexisting", "replace":
		resolvedOverwriteMode = packages.OverwriteModeOverwriteExisting
	default:
		return fmt.Errorf("invalid value '%s' for --overwrite-mode. Valid values are 'fail', 'ignore', 'overwrite'", overwriteMode)
	}

	var jsonResult uploadViewModel
	didErrorsOccur := false

	// with globs it's easy to specify the same thing twice by accident, so keep track of what's been uploaded as we go
	seenPackages := make(map[string]bool)
	doUpload := func(path string) error {
		if !seenPackages[path] {
			created, err := uploadFileAtPath(octopus, space, path, resolvedOverwriteMode, cmd)
			seenPackages[path] = true // whether a given package succeeds or fails, we still don't want to process it twice

			if err != nil {
				didErrorsOccur = true // for process exit code
				switch outputFormat {
				case constants.OutputFormatJson:
					jsonResult.Failed = append(jsonResult.Failed, uploadFailedViewModel{
						PackagePath: path,
						Error:       err.Error(),
					})
				case constants.OutputFormatBasic:
					cmd.PrintErrf("Failed %s\n", path)
				default:
					cmd.PrintErrf("Failed to upload package %s - %v\n", path, err)
				}

				if !continueOnError {
					return tracerr.Wrap(err)
				} // else keep going to the next file.

				// side-effect: If there is a single failed upload, and you specify --continue-on-error, then
				// no error will be returned to the outer shell, and the process will exit with a *success* code.
				// This is intended behaviour, not a bug
			} else {
				switch outputFormat {
				case constants.OutputFormatJson:
					jsonResult.Succeeded = append(jsonResult.Succeeded, uploadSucceededViewModel{
						PackagePath: path,
					})
				case constants.OutputFormatBasic:
					cmd.Printf("%s\n", path)
				default:
					if created {
						cmd.Printf("Uploaded package %s\n", path)
					} else {
						cmd.Printf("Ignored existing package %s\n", path)
					}
				}
			}
		}
		return nil
	}

	for _, pkgString := range flags.Package.Value {
		globMatches, err := filepath.Glob(pkgString)
		// nil, nil means this wasn't a valid glob pattern; assume it's just a filepath
		if err == nil && globMatches == nil {
			err = doUpload(pkgString)
			if err != nil {
				return tracerr.Wrap(err)
			}
		} else if err != nil { // invalid glob pattern
			return tracerr.Wrap(err)
		} else { // glob matched at least 1 thing
			for _, globMatch := range globMatches {
				err = doUpload(globMatch)
				if err != nil {
					return tracerr.Wrap(err)
				}
			}
		}
	}
	if len(seenPackages) == 0 {
		return errors.New("at least one package must be specified")
	}
	if outputFormat == constants.OutputFormatJson {
		bytes, _ := json.Marshal(jsonResult)
		_, _ = cmd.OutOrStdout().Write(bytes)
	}
	if didErrorsOccur {
		// return a generic error to avoid repetition of a previous error, which should have already been printed to stderr
		return errors.New("one or more packages failed to upload")
	}
	return nil
}

func uploadFileAtPath(octopus newclient.Client, space *spaces.Space, path string, overwriteMode packages.OverwriteMode, cmd *cobra.Command) (bool, error) {
	opener := func(name string) (io.ReadCloser, error) { return os.Open(name) }
	if cmd.Context() != nil { // allow context to override the definition of 'os.Open' for testing
		if f, ok := cmd.Context().Value(constants.ContextKeyOsOpen).(func(string) (io.ReadCloser, error)); ok {
			opener = f
		}
	}

	fileReader, err := opener(path)
	if err != nil {
		return false, tracerr.Wrap(err)
	}

	// Note: the PackageUploadResponse has a lot of information in it, but we've chosen not to do anything
	// with it in the CLI at this time.
	_, created, err := packages.Upload(octopus, space.ID, filepath.Base(path), fileReader, overwriteMode)
	_ = fileReader.Close()
	return created, tracerr.Wrap(err)
}
