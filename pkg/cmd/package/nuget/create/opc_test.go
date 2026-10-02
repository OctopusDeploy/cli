package create

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func partsByName(t *testing.T, id, version string, authors []string, description string, packaged []string) map[string]string {
	t.Helper()

	entries, err := buildOpcParts(id, version, authors, description, packaged)
	assert.NoError(t, err)

	byName := map[string]string{}
	for _, entry := range entries {
		byName[entry.Name] = string(entry.Content)
	}

	return byName
}

// Without these three parts a .nupkg is a plain zip, and feeds such as
// Cloudsmith and Artifactory refuse to index it.
func TestBuildOpcParts_ProducesTheRequiredParts(t *testing.T) {
	parts := partsByName(t, "Acme.Widget", "1.2.3", []string{"Acme"}, "A widget", []string{"lib/thing.dll"})

	assert.Len(t, parts, 3)
	assert.Contains(t, parts, "[Content_Types].xml")
	assert.Contains(t, parts, "_rels/.rels")

	var corePropertiesPart string
	for name := range parts {
		if strings.HasPrefix(name, "package/services/metadata/core-properties/") {
			corePropertiesPart = name
		}
	}
	assert.NotEmpty(t, corePropertiesPart, "expected a core properties part")
	assert.True(t, strings.HasSuffix(corePropertiesPart, ".psmdcp"))
}

func TestBuildOpcParts_PartsAreWellFormedXml(t *testing.T) {
	parts := partsByName(t, "Acme.Widget", "1.2.3", []string{"Acme"}, "A widget", []string{"lib/thing.dll"})

	for name, content := range parts {
		assert.True(t, strings.HasPrefix(content, xml.Header), "%s should start with an XML declaration", name)

		var discard any
		assert.NoError(t, xml.Unmarshal([]byte(content), &discard), "%s should be well formed", name)
	}
}

func TestBuildOpcParts_RelationshipsPointAtTheManifestAndCoreProperties(t *testing.T) {
	parts := partsByName(t, "Acme.Widget", "1.2.3", []string{"Acme"}, "A widget", []string{"lib/thing.dll"})
	rels := parts["_rels/.rels"]

	assert.Contains(t, rels, `Target="/Acme.Widget.nuspec"`)
	assert.Contains(t, rels, manifestRelationshipType)
	assert.Contains(t, rels, corePropertiesRelType)
	assert.Contains(t, rels, `Target="/package/services/metadata/core-properties/`)
}

func TestBuildOpcParts_CorePropertiesCarryTheMetadata(t *testing.T) {
	parts := partsByName(t, "Acme.Widget", "1.2.3", []string{"Alice", "Bob"}, "A widget", []string{"lib/thing.dll"})

	var coreProperties string
	for name, content := range parts {
		if strings.HasSuffix(name, ".psmdcp") {
			coreProperties = content
		}
	}

	assert.Contains(t, coreProperties, "<dc:creator>Alice, Bob</dc:creator>")
	assert.Contains(t, coreProperties, "<dc:description>A widget</dc:description>")
	assert.Contains(t, coreProperties, "<dc:identifier>Acme.Widget</dc:identifier>")
	assert.Contains(t, coreProperties, "<version>1.2.3</version>")
}

func TestBuildOpcParts_IsDeterministic(t *testing.T) {
	first := partsByName(t, "Acme.Widget", "1.2.3", []string{"Acme"}, "A widget", []string{"lib/thing.dll"})
	second := partsByName(t, "Acme.Widget", "1.2.3", []string{"Acme"}, "A widget", []string{"lib/thing.dll"})

	assert.Equal(t, first, second, "packing the same inputs twice should produce identical parts")
}

func TestBuildContentTypes_DeclaresEachExtensionOnce(t *testing.T) {
	types := buildContentTypes([]string{"lib/a.dll", "lib/b.dll", "readme.txt", "Acme.nuspec", "_rels/.rels"})

	extensions := map[string]string{}
	for _, entry := range types.Defaults {
		assert.NotContains(t, extensions, entry.Extension, "extension %s declared twice", entry.Extension)
		extensions[entry.Extension] = entry.ContentType
	}

	assert.Equal(t, "application/octet", extensions["dll"])
	assert.Equal(t, "application/octet", extensions["txt"])
	assert.Equal(t, "application/vnd.openxmlformats-package.relationships+xml", extensions["rels"])
	assert.Empty(t, types.Override)
}

// A Default only covers parts that have an extension, so anything without one
// needs an Override or the container is not fully described.
func TestBuildContentTypes_OverridesPartsWithoutAnExtension(t *testing.T) {
	types := buildContentTypes([]string{"tools/run", "lib/a.dll"})

	assert.Len(t, types.Override, 1)
	assert.Equal(t, "/tools/run", types.Override[0].PartName)
	assert.Equal(t, "application/octet", types.Override[0].ContentType)
}

func TestBuildContentTypes_IgnoresDirectoryAndCurrentPathEntries(t *testing.T) {
	types := buildContentTypes([]string{".", "lib/", "lib/a.dll"})

	assert.Empty(t, types.Override)
	assert.Len(t, types.Defaults, 1)
	assert.Equal(t, "dll", types.Defaults[0].Extension)
}

func TestBuildContentTypes_ExtensionMatchingIsCaseInsensitive(t *testing.T) {
	types := buildContentTypes([]string{"lib/a.DLL", "lib/b.dll"})

	assert.Len(t, types.Defaults, 1)
	assert.Equal(t, "dll", types.Defaults[0].Extension)
}
