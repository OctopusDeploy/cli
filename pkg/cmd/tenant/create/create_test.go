package create_test

import (
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tagsets"
	"github.com/stretchr/testify/assert"
)

func TestTenantCreate_Tags(t *testing.T) {
	fooTagSet := tagsets.NewTagSet("foo")
	fooTagSet.Type = tagsets.TagSetTypeMultiSelect
	fooTagSet.Tags = []*tagsets.Tag{
		tagsets.NewTag("bar", "#000000"),
		tagsets.NewTag("car", "#0000FF"),
	}
	fooTagSet.Tags[0].CanonicalTagName = "foo/bar"
	fooTagSet.Tags[1].CanonicalTagName = "foo/car"

	binTagSet := tagsets.NewTagSet("bin")
	binTagSet.Type = tagsets.TagSetTypeMultiSelect
	binTagSet.Tags = []*tagsets.Tag{
		tagsets.NewTag("bop", "#FF0000"),
	}
	binTagSet.Tags[0].CanonicalTagName = "bin/bop"

	tagSets := []*tagsets.TagSet{fooTagSet, binTagSet}

	pa := []*testutil.PA{
		{
			Prompt: &survey.MultiSelect{
				Message: "foo",
				Options: []string{"foo/bar", "foo/car"},
				Default: []string{},
			},
			Answer: []string{"foo/bar"},
		},
		{
			Prompt: &survey.MultiSelect{
				Message: "bin",
				Options: []string{"bin/bop"},
				Default: []string{},
			},
			Answer: []string{},
		},
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	result, err := selectors.Tags(asker, []string{}, []string{}, tagSets)

	checkRemainingPrompts()
	assert.Nil(t, err)
	assert.Equal(t, []string{"foo/bar"}, result)
}
