package create

import (
	"encoding/xml"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	pack "github.com/OctopusDeploy/cli/pkg/cmd/package/support"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nuspec is the subset of the manifest these tests care about, used to assert
// against parsed XML rather than raw substrings.
type nuspec struct {
	Metadata struct {
		Id           string `xml:"id"`
		Version      string `xml:"version"`
		Title        string `xml:"title"`
		Description  string `xml:"description"`
		Authors      string `xml:"authors"`
		ReleaseNotes string `xml:"releaseNotes"`
	} `xml:"metadata"`
}

func newTestOptions(t *testing.T, basePath string) *NuPkgCreateOptions {
	t.Helper()

	opts := &NuPkgCreateOptions{
		NuPkgCreateFlags:     NewNuPkgCreateFlags(),
		PackageCreateOptions: &pack.PackageCreateOptions{PackageCreateFlags: pack.NewPackageCreateFlags()},
	}
	opts.Id.Value = "Acme.Web"
	opts.Version.Value = "1.2.3"
	opts.BasePath.Value = basePath
	return opts
}

func generateAndParse(t *testing.T, opts *NuPkgCreateOptions) (nuspec, string) {
	t.Helper()

	path, err := GenerateNuSpec(opts)
	require.NoError(t, err)

	contents, err := os.ReadFile(path)
	require.NoError(t, err)

	var parsed nuspec
	require.NoError(t, xml.Unmarshal(contents, &parsed), "generated nuspec should be well-formed XML")

	return parsed, string(contents)
}

// The manifest is what makes a .nupkg a NuGet package rather than a zip, and
// the OPC relationships part points at it by name whether or not it exists.
func TestGeneratesNuSpecWithNoMetadataSupplied(t *testing.T) {
	opts := newTestOptions(t, t.TempDir())
	require.NoError(t, applyDefaultsToUnspecifiedPackageOptions(opts))

	parsed, _ := generateAndParse(t, opts)

	assert.Equal(t, "Acme.Web", parsed.Metadata.Id)
	assert.Equal(t, "1.2.3", parsed.Metadata.Version)
	assert.NotEmpty(t, parsed.Metadata.Description, "description is required by the nuspec schema")
	assert.NotEmpty(t, parsed.Metadata.Authors, "authors is required by the nuspec schema")
}

func TestDescriptionDefaultsWhenNotSupplied(t *testing.T) {
	opts := newTestOptions(t, t.TempDir())
	require.NoError(t, applyDefaultsToUnspecifiedPackageOptions(opts))

	assert.Equal(t, DefaultDescription, opts.Description.Value)
}

func TestSuppliedDescriptionIsKept(t *testing.T) {
	opts := newTestOptions(t, t.TempDir())
	opts.Description.Value = "Something specific"
	require.NoError(t, applyDefaultsToUnspecifiedPackageOptions(opts))

	assert.Equal(t, "Something specific", opts.Description.Value)
}

func TestAuthorDefaultsToCurrentUser(t *testing.T) {
	current, err := user.Current()
	require.NoError(t, err)

	opts := newTestOptions(t, t.TempDir())
	require.NoError(t, applyDefaultsToUnspecifiedPackageOptions(opts))

	assert.Equal(t, []string{current.Username}, opts.Author.Value)
}

func TestSuppliedAuthorsAreKept(t *testing.T) {
	opts := newTestOptions(t, t.TempDir())
	opts.Author.Value = []string{"Ada", "Grace"}
	require.NoError(t, applyDefaultsToUnspecifiedPackageOptions(opts))

	parsed, _ := generateAndParse(t, opts)

	assert.Equal(t, []string{"Ada", "Grace"}, opts.Author.Value)
	assert.Equal(t, "Ada,Grace", parsed.Metadata.Authors)
}

// User lookup fails on some minimal container images. The command should still
// produce a schema-valid manifest rather than failing over metadata nobody asked
// for.
func TestDefaultAuthorFallsBackWhenUserIsUnknown(t *testing.T) {
	original := currentUser
	t.Cleanup(func() { currentUser = original })
	currentUser = func() (*user.User, error) { return nil, errors.New("no user") }

	assert.Equal(t, "Acme.Web", defaultAuthor("Acme.Web"))
}

func TestDefaultAuthorFallsBackWhenUsernameIsBlank(t *testing.T) {
	original := currentUser
	t.Cleanup(func() { currentUser = original })
	currentUser = func() (*user.User, error) { return &user.User{Username: "  "}, nil }

	assert.Equal(t, "Acme.Web", defaultAuthor("Acme.Web"))
}

func TestPackageIsStillUsableWhenUserLookupFails(t *testing.T) {
	original := currentUser
	t.Cleanup(func() { currentUser = original })
	currentUser = func() (*user.User, error) { return nil, errors.New("no user") }

	opts := newTestOptions(t, t.TempDir())
	require.NoError(t, applyDefaultsToUnspecifiedPackageOptions(opts))

	parsed, _ := generateAndParse(t, opts)

	assert.Equal(t, "Acme.Web", parsed.Metadata.Authors)
}

// A hand-written manifest is the user's own file. Generating over the top would
// discard their metadata, and the cleanup step would then delete it outright.
func TestExistingNuSpecIsDetected(t *testing.T) {
	basePath := t.TempDir()
	existing := filepath.Join(basePath, "Acme.Web.nuspec")
	require.NoError(t, os.WriteFile(existing, []byte("<package><metadata><id>Acme.Web</id></metadata></package>"), 0644))

	supplied, err := hasSuppliedNuSpec(basePath, "Acme.Web.nuspec")

	require.NoError(t, err)
	assert.True(t, supplied)
}

func TestMissingNuSpecIsNotMistakenForOne(t *testing.T) {
	supplied, err := hasSuppliedNuSpec(t.TempDir(), "Acme.Web.nuspec")

	require.NoError(t, err)
	assert.False(t, supplied)
}

func TestTitleIsOmittedWhenNotSupplied(t *testing.T) {
	opts := newTestOptions(t, t.TempDir())
	require.NoError(t, applyDefaultsToUnspecifiedPackageOptions(opts))

	_, raw := generateAndParse(t, opts)

	assert.NotContains(t, raw, "<title>")
}

func TestReleaseNotesAreIncludedWhenSupplied(t *testing.T) {
	opts := newTestOptions(t, t.TempDir())
	opts.ReleaseNotes.Value = "Fixed a thing"
	require.NoError(t, applyDefaultsToUnspecifiedPackageOptions(opts))

	parsed, _ := generateAndParse(t, opts)

	assert.Equal(t, "Fixed a thing", parsed.Metadata.ReleaseNotes)
}

func TestReleaseNotesAndReleaseNotesFileAreMutuallyExclusive(t *testing.T) {
	opts := newTestOptions(t, t.TempDir())
	opts.ReleaseNotes.Value = "Fixed a thing"
	opts.ReleaseNotesFile.Value = "notes.txt"

	_, err := GenerateNuSpec(opts)

	assert.ErrorContains(t, err, "cannot specify both")
}

// Descriptions and release notes are free text. An unescaped ampersand or angle
// bracket is enough to make the whole manifest unparseable, which matters more
// now that one is written for every package.
func TestMetadataIsXmlEscaped(t *testing.T) {
	opts := newTestOptions(t, t.TempDir())
	opts.Description.Value = `Fish & chips <b>bold</b>`
	opts.Title.Value = `A "quoted" title`
	opts.ReleaseNotes.Value = `1 < 2 && 3 > 2`
	opts.Author.Value = []string{"Ada & Grace"}

	parsed, raw := generateAndParse(t, opts)

	assert.NotContains(t, raw, "Fish & chips", "raw ampersand should have been escaped")
	assert.Contains(t, raw, "&amp;")

	// Round-tripping is the real assertion: what went in is what comes back out.
	assert.Equal(t, `Fish & chips <b>bold</b>`, parsed.Metadata.Description)
	assert.Equal(t, `A "quoted" title`, parsed.Metadata.Title)
	assert.Equal(t, `1 < 2 && 3 > 2`, parsed.Metadata.ReleaseNotes)
	assert.Equal(t, "Ada & Grace", parsed.Metadata.Authors)
}

func TestNuSpecIsWrittenIntoTheBasePath(t *testing.T) {
	basePath := t.TempDir()
	opts := newTestOptions(t, basePath)
	require.NoError(t, applyDefaultsToUnspecifiedPackageOptions(opts))

	path, err := GenerateNuSpec(opts)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(basePath, "Acme.Web.nuspec"), path)
	assert.True(t, strings.HasSuffix(path, ".nuspec"))
}
