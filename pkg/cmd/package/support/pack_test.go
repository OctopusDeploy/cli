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

func setupForExclude(t *testing.T) string {
	dir := filepath.ToSlash(t.TempDir())
	for _, relativePath := range []string{
		"test.txt",
		"app.config",
		"sub/nested.config",
		"sub/keep.dll",
		"AppData/data.bin",
		"AppData/deep/more.bin",
		"conf/settings.local.env",
	} {
		fullPath := filepath.Join(dir, relativePath)
		assert.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
		assert.NoError(t, os.WriteFile(fullPath, []byte("x"), 0644))
	}

	return dir
}

func TestGetDistinctPatternMatches_NoExcludeKeepsEverything(t *testing.T) {
	basePath := setupForExclude(t)

	result, err := getDistinctPatternMatches(basePath, []string{"**"}, nil)

	assert.NoError(t, err)
	assert.Contains(t, result, "app.config")
	assert.Contains(t, result, "sub/nested.config")
	assert.Contains(t, result, "AppData/data.bin")
	assert.Contains(t, result, "conf/settings.local.env")
}

// The three scenarios called out in the original request for --exclude.
func TestGetDistinctPatternMatches_ExcludeByExtension(t *testing.T) {
	basePath := setupForExclude(t)

	result, err := getDistinctPatternMatches(basePath, []string{"**"}, []string{"**/*.config"})

	assert.NoError(t, err)
	// matched at the root as well as in a subdirectory
	assert.NotContains(t, result, "app.config")
	assert.NotContains(t, result, "sub/nested.config")
	assert.Contains(t, result, "sub/keep.dll")
	assert.Contains(t, result, "test.txt")
}

func TestGetDistinctPatternMatches_ExcludeDirectoryAndContents(t *testing.T) {
	basePath := setupForExclude(t)

	result, err := getDistinctPatternMatches(basePath, []string{"**"}, []string{"AppData/**"})

	assert.NoError(t, err)
	// the directory entries go too, otherwise they are archived as empty folders
	assert.NotContains(t, result, "AppData")
	assert.NotContains(t, result, "AppData/deep")
	assert.NotContains(t, result, "AppData/data.bin")
	assert.NotContains(t, result, "AppData/deep/more.bin")
	assert.Contains(t, result, "test.txt")
}

func TestGetDistinctPatternMatches_ExcludeCompoundExtension(t *testing.T) {
	basePath := setupForExclude(t)

	result, err := getDistinctPatternMatches(basePath, []string{"**"}, []string{"**/*.local.env"})

	assert.NoError(t, err)
	assert.NotContains(t, result, "conf/settings.local.env")
	assert.Contains(t, result, "conf")
	assert.Contains(t, result, "test.txt")
}

func TestGetDistinctPatternMatches_MultipleExcludePatterns(t *testing.T) {
	basePath := setupForExclude(t)

	result, err := getDistinctPatternMatches(basePath, []string{"**"}, []string{"**/*.config", "AppData/**"})

	assert.NoError(t, err)
	assert.NotContains(t, result, "app.config")
	assert.NotContains(t, result, "sub/nested.config")
	assert.NotContains(t, result, "AppData/data.bin")
	assert.Contains(t, result, "sub/keep.dll")
	assert.Contains(t, result, "conf/settings.local.env")
}

// --exclude narrows whatever --include selected, rather than widening it.
func TestGetDistinctPatternMatches_ExcludeAppliesAfterInclude(t *testing.T) {
	basePath := setupForExclude(t)

	result, err := getDistinctPatternMatches(basePath, []string{"sub/**"}, []string{"**/*.config"})

	assert.NoError(t, err)
	assert.Contains(t, result, "sub/keep.dll")
	assert.NotContains(t, result, "sub/nested.config")
	assert.NotContains(t, result, "test.txt")
}

func TestBuildPackage_VerboseOutput(t *testing.T) {
	basePath := setupForArchive(t)
	if runtime.GOOS == "windows" {
		defer t.Cleanup(func() {
			cleanUpTemp(basePath)
		})
	}

	outFolder := filepath.Join(basePath, "out")
	err := os.MkdirAll(outFolder, 0755)
	assert.NoError(t, err)

	var buf bytes.Buffer
	opts := &PackageCreateOptions{
		PackageCreateFlags: &PackageCreateFlags{
			Id:        &flag.Flag[string]{Value: "TestPackage"},
			Version:   &flag.Flag[string]{Value: "1.2.3"},
			BasePath:  &flag.Flag[string]{Value: basePath},
			OutFolder: &flag.Flag[string]{Value: outFolder},
			Include:   &flag.Flag[[]string]{Value: []string{"**"}},
			Exclude:   &flag.Flag[[]string]{Value: []string{}},
			Verbose:   &flag.Flag[bool]{Value: true},
			Overwrite: &flag.Flag[bool]{Value: true},
		},
		Writer: &buf,
	}
	_, err = BuildPackage(opts, "TestPackage.1.2.3.zip")

	expectedOutput := "Saving \"TestPackage.1.2.3.zip\" to \"" + outFolder + "\"...\n" +
		"Adding files from \"" + filepath.ToSlash(basePath) + "\" matching pattern/s \"**\"\n" +
		"Added file: test.txt\n"

	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, buf.String())
}
