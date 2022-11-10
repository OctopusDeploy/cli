package create_test

import (
	"testing"

	tenantCreate "github.com/OctopusDeploy/cli/pkg/cmd/tenant/create"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tagsets"
)

func TestTenantCreate_Tags(t *testing.T) {
	tags := []string{
		"foo/bar",
		"foo/car",
		"bin/bop",
	}
	pa := []*testutil.PA{
		testutil.NewMultiSelectPrompt("Tags", "", tags, []string{"foo/bar"}),
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	getAllTagsCallback := func() ([]*tagsets.TagSet, error) {
		fooTagSet := tagsets.NewTagSet("foo")
		fooTagSet.Tags = []*tagsets.Tag{
			tagsets.NewTag("bar", "#000000"),
			tagsets.NewTag("car", "#0000FF"),
		}
		fooTagSet.Tags[0].CanonicalTagName = "foo/bar"
		fooTagSet.Tags[1].CanonicalTagName = "foo/car"
		binTagSet := tagsets.NewTagSet("bin")
		binTagSet.Tags = []*tagsets.Tag{
			tagsets.NewTag("bop", "#FF0000"),
		}
		binTagSet.Tags[0].CanonicalTagName = "bin/bop"
		return []*tagsets.TagSet{
			fooTagSet,
			binTagSet,
		}, nil
	}
	tenantCreate.AskTags(asker, []string{}, getAllTagsCallback)
	checkRemainingPrompts()
}
