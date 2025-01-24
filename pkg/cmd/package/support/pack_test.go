package support

import (
	"bytes"
	"errors"
	flag "github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestVerboseOut_WithVerboseEnabled(t *testing.T) {
	result := testutil.CaptureConsoleOutput(func() {
		VerboseOut(os.Stdout, true, "This %s a %s... %d", "is", "test", 123)
	})
	assert.Equal(t, "This is a test... 123", result)
}

func TestVerboseOut_WithVerboseDisabled(t *testing.T) {
	result := testutil.CaptureConsoleOutput(func() {
		VerboseOut(os.Stdout, false, "This %s a %s... %d", "is", "test", 123)
	})
	assert.Equal(t, "", result)
}

func TestBuildTimestampSemVer(t *testing.T) {
	knownTime := time.Date(2000, time.January, 1, 1, 1, 1, 0, time.UTC)
	assert.Equal(t, "2000.1.1.10101", BuildTimestampSemVer(knownTime))
}

func TestBuildOutFileName(t *testing.T) {
	result := BuildOutFileName("zip", "SomePackage", "1.0.1")
	assert.Equal(t, "SomePackage.1.0.1.zip", result)
}

func TestPanicImmediately(t *testing.T) {
	basePath := setupForArchive(t)
	if runtime.GOOS == "windows" { // See line 63
		defer t.Cleanup(func() {
			cleanUpTemp(basePath)
		})
	}

	newPath := filepath.Join(basePath, "test.txt")
	_, err := os.Stat(newPath)
	assert.Nil(t, err)
}

func setupForArchive(t *testing.T) string {
	dir := filepath.ToSlash(t.TempDir())
	_, err := os.Create(dir + "/test.txt")
	if err != nil {
		panic(err)
	}

	return dir
}

// TODO Test and potentially remove manual clean-up when go version >= 1.20.0
// cleanUpTemp is a temporary solution for windows to https://github.com/golang/go/issues/51442.
func cleanUpTemp(tempDir string) {
	err := errors.New("init not nil")
	for err != nil {
		time.Sleep(time.Millisecond * 10)
		err = os.RemoveAll(tempDir)
	}
}

func TestBuildPackage_VerboseOutput(t *testing.T) {
	// Setup test directory and file
	basePath := setupForArchive(t)
	if runtime.GOOS == "windows" {
		defer t.Cleanup(func() {
			cleanUpTemp(basePath)
		})
	}

	outFolder := filepath.Join(basePath, "out")
	err := os.MkdirAll(outFolder, 0755)
	assert.NoError(t, err)

	// Create test options
	var buf bytes.Buffer
	opts := &PackageCreateOptions{
		PackageCreateFlags: &PackageCreateFlags{
			Id:        &flag.Flag[string]{Value: "TestPackage"},
			Version:   &flag.Flag[string]{Value: "1.2.3"},
			BasePath:  &flag.Flag[string]{Value: basePath},
			OutFolder: &flag.Flag[string]{Value: outFolder},
			Include:   &flag.Flag[[]string]{Value: []string{"**"}},
			Verbose:   &flag.Flag[bool]{Value: true},
			Overwrite: &flag.Flag[bool]{Value: true},
		},
		Writer: &buf,
	}

	// Execute package build
	_, err = BuildPackage(opts, "TestPackage.1.2.3.zip")
	assert.NoError(t, err)

	// Verify output format
	expectedOutput := "Saving \"TestPackage.1.2.3.zip\" to \"" + outFolder + "\"...\n" +
		"Adding files from \"" + filepath.ToSlash(basePath) + "\" matching pattern/s \"**\"\n" +
		"Added file: test.txt\n"
	assert.Equal(t, expectedOutput, buf.String())
}
