package create

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"path"
	"sort"
	"strings"

	pack "github.com/OctopusDeploy/cli/pkg/cmd/package/support"
	"github.com/OctopusDeploy/cli/pkg/constants"
)

// A .nupkg is not a plain zip. It is an Open Packaging Conventions container,
// and feeds such as Cloudsmith, Artifactory and nuget.org reject one that is
// missing the OPC parts below:
//
//	[Content_Types].xml                                       declares a content type per extension
//	_rels/.rels                                               points at the manifest and core properties
//	package/services/metadata/core-properties/<hash>.psmdcp   the core properties themselves
//
// The nuspec and the packaged files alone are not enough, even though most
// tooling can still read them.
const (
	contentTypesPart  = "[Content_Types].xml"
	relationshipsPart = "_rels/.rels"
	corePropertiesDir = "package/services/metadata/core-properties"

	contentTypesNamespace    = "http://schemas.openxmlformats.org/package/2006/content-types"
	relationshipsNamespace   = "http://schemas.openxmlformats.org/package/2006/relationships"
	corePropertiesNamespace  = "http://schemas.openxmlformats.org/package/2006/metadata/core-properties"
	manifestRelationshipType = "http://schemas.microsoft.com/packaging/2010/07/manifest"
	corePropertiesRelType    = "http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties"

	// what NuGet itself writes for parts it has no more specific type for
	defaultContentType = "application/octet"
)

type contentTypes struct {
	XMLName  xml.Name              `xml:"Types"`
	Xmlns    string                `xml:"xmlns,attr"`
	Defaults []contentTypeDefault  `xml:"Default"`
	Override []contentTypeOverride `xml:"Override"`
}

type contentTypeDefault struct {
	Extension   string `xml:"Extension,attr"`
	ContentType string `xml:"ContentType,attr"`
}

type contentTypeOverride struct {
	PartName    string `xml:"PartName,attr"`
	ContentType string `xml:"ContentType,attr"`
}

type relationships struct {
	XMLName       xml.Name       `xml:"Relationships"`
	Xmlns         string         `xml:"xmlns,attr"`
	Relationships []relationship `xml:"Relationship"`
}

type relationship struct {
	Type   string `xml:"Type,attr"`
	Target string `xml:"Target,attr"`
	Id     string `xml:"Id,attr"`
}

type coreProperties struct {
	XMLName        xml.Name `xml:"coreProperties"`
	Xmlns          string   `xml:"xmlns,attr"`
	XmlnsDc        string   `xml:"xmlns:dc,attr"`
	XmlnsDcterms   string   `xml:"xmlns:dcterms,attr"`
	XmlnsXsi       string   `xml:"xmlns:xsi,attr"`
	Creator        string   `xml:"dc:creator,omitempty"`
	Description    string   `xml:"dc:description,omitempty"`
	Identifier     string   `xml:"dc:identifier"`
	Version        string   `xml:"version"`
	Keywords       string   `xml:"keywords,omitempty"`
	LastModifiedBy string   `xml:"lastModifiedBy"`
}

// buildOpcParts produces the OPC parts for a package whose archive will contain
// packagedPaths plus a nuspec named after the package id.
func buildOpcParts(id string, version string, authors []string, description string, packagedPaths []string) ([]pack.ArchiveEntry, error) {
	nuspecPart := id + ".nuspec"
	corePropertiesPart := fmt.Sprintf("%s/%s.psmdcp", corePropertiesDir, deterministicHex(id+version, 16))

	// every part in the finished archive needs a declared content type
	allParts := append([]string{}, packagedPaths...)
	allParts = append(allParts, nuspecPart, relationshipsPart, corePropertiesPart)

	contentTypesXml, err := marshalPart(buildContentTypes(allParts))
	if err != nil {
		return nil, err
	}

	relationshipsXml, err := marshalPart(relationships{
		Xmlns: relationshipsNamespace,
		Relationships: []relationship{
			{
				Type:   manifestRelationshipType,
				Target: "/" + nuspecPart,
				Id:     "R" + deterministicHex("manifest"+id+version, 8),
			},
			{
				Type:   corePropertiesRelType,
				Target: "/" + corePropertiesPart,
				Id:     "R" + deterministicHex("coreproperties"+id+version, 8),
			},
		},
	})
	if err != nil {
		return nil, err
	}

	creator := strings.Join(authors, ", ")
	corePropertiesXml, err := marshalPart(coreProperties{
		Xmlns:          corePropertiesNamespace,
		XmlnsDc:        "http://purl.org/dc/elements/1.1/",
		XmlnsDcterms:   "http://purl.org/dc/terms/",
		XmlnsXsi:       "http://www.w3.org/2001/XMLSchema-instance",
		Creator:        creator,
		Description:    description,
		Identifier:     id,
		Version:        version,
		LastModifiedBy: constants.ExecutableName,
	})
	if err != nil {
		return nil, err
	}

	return []pack.ArchiveEntry{
		{Name: contentTypesPart, Content: contentTypesXml},
		{Name: relationshipsPart, Content: relationshipsXml},
		{Name: corePropertiesPart, Content: corePropertiesXml},
	}, nil
}

// buildContentTypes declares one Default per distinct extension. Parts without
// an extension cannot be covered by a Default, so they get an Override each.
func buildContentTypes(parts []string) contentTypes {
	types := contentTypes{Xmlns: contentTypesNamespace}

	seenExtensions := map[string]bool{}
	var extensions []string
	var overrides []contentTypeOverride

	for _, part := range parts {
		// a trailing slash marks a directory, which is not a part at all
		if part == "" || part == "." || strings.HasSuffix(part, "/") {
			continue
		}

		extension := strings.TrimPrefix(strings.ToLower(path.Ext(part)), ".")
		if extension == "" {
			overrides = append(overrides, contentTypeOverride{
				PartName:    "/" + part,
				ContentType: defaultContentType,
			})
			continue
		}

		if !seenExtensions[extension] {
			seenExtensions[extension] = true
			extensions = append(extensions, extension)
		}
	}

	sort.Strings(extensions)
	for _, extension := range extensions {
		types.Defaults = append(types.Defaults, contentTypeDefault{
			Extension:   extension,
			ContentType: contentTypeFor(extension),
		})
	}

	sort.Slice(overrides, func(i, j int) bool { return overrides[i].PartName < overrides[j].PartName })
	types.Override = overrides

	return types
}

func contentTypeFor(extension string) string {
	switch extension {
	case "rels":
		return "application/vnd.openxmlformats-package.relationships+xml"
	case "psmdcp":
		return "application/vnd.openxmlformats-package.core-properties+xml"
	default:
		return defaultContentType
	}
}

func marshalPart(part any) ([]byte, error) {
	body, err := xml.Marshal(part)
	if err != nil {
		return nil, err
	}

	return append([]byte(xml.Header), body...), nil
}

// deterministicHex keeps the generated part and relationship names stable for a
// given package, so repacking the same inputs produces the same archive.
func deterministicHex(seed string, bytes int) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:bytes])
}
