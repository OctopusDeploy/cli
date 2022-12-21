package support

import (
	"archive/zip"
	"errors"
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/spf13/cobra"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	FlagId        = "id"
	FlagVersion   = "version"
	FlagBasePath  = "base-path"
	FlagOutFolder = "out-folder"
	FlagInclude   = "include"
	FlagVerbose   = "verbose"
	FlagOverwrite = "overwrite"
)

type PackageCreateFlags struct {
	Id        *flag.Flag[string]
	Version   *flag.Flag[string]
	BasePath  *flag.Flag[string]
	OutFolder *flag.Flag[string]
	Include   *flag.Flag[[]string]
	Verbose   *flag.Flag[bool]
	Overwrite *flag.Flag[bool]
}

type PackageCreateOptions struct {
	*PackageCreateFlags
	Writer   io.Writer
	Ask      question.Asker
	NoPrompt bool
	CmdPath  string
}

func NewPackageCreateFlags() *PackageCreateFlags {
	return &PackageCreateFlags{
		Id:        flag.New[string](FlagId, false),
		Version:   flag.New[string](FlagVersion, false),
		BasePath:  flag.New[string](FlagBasePath, false),
		OutFolder: flag.New[string](FlagOutFolder, false),
		Include:   flag.New[[]string](FlagInclude, false),
		Verbose:   flag.New[bool](FlagVerbose, false),
		Overwrite: flag.New[bool](FlagOverwrite, false),
	}
}

func NewPackageCreateOptions(f factory.Factory, flags *PackageCreateFlags, cmd *cobra.Command) *PackageCreateOptions {
	return &PackageCreateOptions{
		PackageCreateFlags: flags,
		Writer:             cmd.OutOrStdout(),
		Ask:                f.Ask,
		NoPrompt:           !f.IsPromptEnabled(),
		CmdPath:            cmd.CommandPath(),
	}
}

func VerboseOut(isVerbose bool, messageTemplate string, messageArgs ...any) {
	if isVerbose {
		fmt.Printf(messageTemplate, messageArgs...)
	}
}

func BuildTimestampSemVer(dateTime time.Time) string {
	endSegment := dateTime.Hour()*10000 + dateTime.Minute()*100 + dateTime.Second()
	return fmt.Sprintf("%d.%d.%d.%d", dateTime.Year(), dateTime.Month(), dateTime.Day(), endSegment)
}

func BuildOutFileName(packageType string, id, version string) string {
	return fmt.Sprintf("%s.%s.%s", id, version, packageType)
}

func BuildPackage(opts *PackageCreateOptions, outFileName string) error {
	outFilePath := filepath.Join(opts.OutFolder.Value, outFileName)
	outPath, err := filepath.Abs(opts.OutFolder.Value)
	if err != nil {
		return err
	}

	_, err = os.Stat(outFilePath)
	if !opts.Overwrite.Value && err == nil { // if not overwrite and package archive already exists
		return errors.New(fmt.Sprintf("package with name '%s' already exists ...aborting", outFileName))
	}

	VerboseOut(opts.Verbose.Value, "Saving \"%s\" to \"%s\"...\nAdding files from \"%s\" matching pattern/s \"%s\"\n", outPath, outFileName, outPath, strings.Join(opts.Include.Value, ", "))

	filePaths, err := getUniqueMatchingFilePaths(opts.BasePath.Value, opts.Include.Value)
	if err != nil {
		return err
	}

	if len(filePaths) == 0 {
		return errors.New("no files identified to package")
	}

	zipFile, zipCreateErr := os.Create(outFilePath)
	if zipCreateErr != nil {
		return zipCreateErr
	}
	defer zipFile.Close()

	writer := zip.NewWriter(zipFile)
	defer writer.Close()

	for _, path := range filePaths {
		if path == outFileName || path == "." {
			continue
		}

		fullPath := filepath.Join(opts.BasePath.Value, path)
		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			return err
		}

		header.Method = zip.Deflate
		header.Name = path
		if fileInfo.IsDir() {
			header.Name += "/"
		}

		headerWriter, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}

		if fileInfo.IsDir() {
			continue
		}

		f, err := os.Open(fullPath)
		if err != nil {
			return err
		}

		_, err = io.Copy(headerWriter, f)
		if err == nil {
			VerboseOut(opts.Verbose.Value, "Added file: %s\n", path)
		} else {
			return err
		}

		err = f.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func getUniqueMatchingFilePaths(basePath string, patterns []string) ([]string, error) {
	fileSys := os.DirFS(filepath.Clean(basePath))
	var filePaths []string

	for _, pattern := range patterns {
		paths, err := doublestar.Glob(fileSys, filepath.ToSlash(pattern))
		if err != nil {
			return nil, err
		}
		filePaths = append(filePaths, paths...)
	}

	return util.SliceDistinct(filePaths), nil
}
