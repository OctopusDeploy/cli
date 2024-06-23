package upload

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/OctopusDeploy/cli/pkg/apiclient"

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

	FlagUseDeltaCompression = "use-delta-compression"
	FlagAliasDelta          = "delta" // for less typing

	// "replace-existing" flag deprecated in the .NET CLI so not brought across

	FlagContinueOnError = "continue-on-error"
)

type UploadFlags struct {
	Package       *flag.Flag[[]string]
	OverwriteMode *flag.Flag[string]

	// Note: string here because cobra doesn't handle bool(default=true) well.
	// If this were a bool flag, and the user entered --use-delta-compression false, it would come out as true
	UseDeltaCompression *flag.Flag[string]
	ContinueOnError     *flag.Flag[bool]
}

func NewUploadFlags() *UploadFlags {
	return &UploadFlags{
		Package:             flag.New[[]string](FlagPackage, false),
		OverwriteMode:       flag.New[string](FlagOverwriteMode, false),
		UseDeltaCompression: flag.New[string](FlagUseDeltaCompression, false),
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
			$ %[1]s package upload --package SomePackage.2.0.0.zip --use-delta-compression false
			$ %[1]s package upload SomePackage.2.0.0.zip --delta false
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
	flags.StringArrayVarP(&uploadFlags.Package.Value, uploadFlags.Package.Name, "p", nil, "Package to upload, may be specified multiple times. Any arguments without flags will be treated as packages")
	flags.StringVarP(&uploadFlags.OverwriteMode.Value, uploadFlags.OverwriteMode.Name, "", "", "Action when a package already exists. Valid values are 'fail', 'overwrite', 'ignore'. Default is 'fail'")
	flags.BoolVarP(&uploadFlags.ContinueOnError.Value, uploadFlags.ContinueOnError.Name, "", false, "When uploading multiple packages, controls whether the CLI continues after a failed upload. Default is to abort.")
	flags.StringVarP(&uploadFlags.UseDeltaCompression.Value, uploadFlags.UseDeltaCompression.Name, "", "true", "If true, will attempt to use delta compression when uploading. Valid values are true or false. Defaults to true.")
	flags.SortFlags = false

	flagAliases := make(map[string][]string, 1)
	util.AddFlagAliasesString(flags, FlagOverwriteMode, flagAliases, FlagAliasOverwrite, FlagAliasOverwriteMode)
	util.AddFlagAliasesString(flags, FlagUseDeltaCompression, flagAliases, FlagAliasDelta)

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

	useDeltaCompressionStr := flags.UseDeltaCompression.Value
	useDeltaCompression, err := strconv.ParseBool(useDeltaCompressionStr)
	if err != nil {
		useDeltaCompression = true
	}

	var jsonResult uploadViewModel
	didErrorsOccur := false

	// with globs it's easy to specify the same thing twice by accident, so keep track of what's been uploaded as we go
	seenPackages := make(map[string]bool)
	doUpload := func(path string) error {
		if !seenPackages[path] {
			uploadStartTime := time.Now()
			uploadResult, err := uploadFileAtPath(octopus, space, path, resolvedOverwriteMode, useDeltaCompression, cmd)
			uploadDuration := time.Since(uploadStartTime)

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
					return err
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
					if uploadResult.CreatedNewFile {
						cmd.Printf("Uploaded package %s\n", path)
					} else {
						cmd.Printf("Ignored existing package %s\n", path)
					}

					if uploadResult.UploadMethod == packages.UploadMethodDelta && uploadResult.UploadInfo != nil {
						deltaInfo := uploadResult.UploadInfo
						deltaRatio := 0.0
						if deltaInfo.FileSize > 0 {
							deltaRatio = float64(deltaInfo.DeltaSize) / float64(deltaInfo.FileSize) * 100
						}

						switch deltaInfo.DeltaBehaviour {
						case packages.DeltaBehaviourNoPreviousFile:
							cmd.Printf("    Full upload for package %s. No previous versions available\n"+
								"    Timing: Signature %v, Upload %v\n",
								path, roundDuration(deltaInfo.RequestSignatureDuration), roundDuration(deltaInfo.UploadDuration))
						case packages.DeltaBehaviourNotEfficient:
							cmd.Printf("    Full upload for package %s. Delta size was %.1f%% of full file (too large)\n"+
								"    Timing: Signature %v, Build Delta %v, Upload %v\n",
								path, deltaRatio,
								roundDuration(deltaInfo.RequestSignatureDuration), roundDuration(deltaInfo.BuildDeltaDuration), roundDuration(deltaInfo.UploadDuration))
						case packages.DeltaBehaviourUploadedDeltaFile:
							cmd.Printf("    Delta upload for package %s.\n"+
								"    Delta size was %.1f%% of full file, saving %d bytes\n"+
								"    Timing: Signature %v, Build Delta %v, Upload %v\n",
								path, deltaRatio, deltaInfo.FileSize-deltaInfo.DeltaSize,
								roundDuration(deltaInfo.RequestSignatureDuration), roundDuration(deltaInfo.BuildDeltaDuration), roundDuration(deltaInfo.UploadDuration))
						default:
							break // a future unknown DeltaBehaviour will result in printing nothing, deliberately
						}
					} else { // delta disabled
						cmd.Printf("    Timing: Upload %v\n", roundDuration(uploadDuration))

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
				return err
			}
		} else if err != nil { // invalid glob pattern
			return err
		} else { // glob matched at least 1 thing
			for _, globMatch := range globMatches {
				err = doUpload(globMatch)
				if err != nil {
					return err
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

// derived from https://stackoverflow.com/a/58415564/234 by https://stackoverflow.com/users/1705598/icza
func roundDuration(d time.Duration) time.Duration {
	divTo2dp := time.Duration(100)
	switch {
	case d > time.Second:
		d = d.Round(time.Second / divTo2dp)
	case d > time.Millisecond:
		d = d.Round(time.Millisecond / divTo2dp)
	case d > time.Microsecond:
		d = d.Round(time.Microsecond / divTo2dp)
	}
	return d
}

func uploadFileAtPath(octopus newclient.Client, space *spaces.Space, path string, overwriteMode packages.OverwriteMode, useDeltaCompression bool, cmd *cobra.Command) (*packages.PackageUploadResponseV2, error) {
	opener := func(name string) (io.ReadSeekCloser, error) { return os.Open(name) }
	if cmd.Context() != nil { // allow context to override the definition of 'os.Open' for testing
		if f, ok := cmd.Context().Value(constants.ContextKeyOsOpen).(func(string) (io.ReadSeekCloser, error)); ok {
			opener = f
		}
	}

	fileReader, err := opener(path)
	if err != nil {
		return nil, err
	}

	// Note: the PackageUploadResponse has a lot of information in it, but we've chosen not to do anything
	// with it in the CLI at this time.
	result, err := packages.UploadV2(octopus, space.ID, filepath.Base(path), fileReader, overwriteMode, useDeltaCompression)
	_ = fileReader.Close()
	return result, err
}
