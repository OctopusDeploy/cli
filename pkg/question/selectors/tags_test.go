package selectors

import (
	"testing"

	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tagsets"
	"github.com/stretchr/testify/assert"
)

func createTestTagSets() []*tagsets.TagSet {
	// Single-select tag set
	regionTagSet := tagsets.NewTagSet("Region")
	regionTagSet.Type = tagsets.TagSetTypeSingleSelect
	regionTagSet.Tags = []*tagsets.Tag{
		tagsets.NewTag("US East", "#000000"),
		tagsets.NewTag("US West", "#0000FF"),
	}
	regionTagSet.Tags[0].CanonicalTagName = "region/us-east"
	regionTagSet.Tags[1].CanonicalTagName = "region/us-west"

	// Multi-select tag set
	envTypeTagSet := tagsets.NewTagSet("Environment Type")
	envTypeTagSet.Type = tagsets.TagSetTypeMultiSelect
	envTypeTagSet.Tags = []*tagsets.Tag{
		tagsets.NewTag("Production", "#FF0000"),
		tagsets.NewTag("Staging", "#00FF00"),
	}
	envTypeTagSet.Tags[0].CanonicalTagName = "envtype/production"
	envTypeTagSet.Tags[1].CanonicalTagName = "envtype/staging"

	return []*tagsets.TagSet{regionTagSet, envTypeTagSet}
}

func TestValidateTags_ValidSingleSelectTag(t *testing.T) {
	tagSets := createTestTagSets()
	tags := []string{"region/us-east"}

	err := ValidateTags(tags, tagSets)

	assert.Nil(t, err)
}

func TestValidateTags_ErrorOnMultipleSingleSelectTags(t *testing.T) {
	tagSets := createTestTagSets()
	tags := []string{"region/us-east", "region/us-west"}

	err := ValidateTags(tags, tagSets)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "only one tag can be specified for single-select tag set 'Region'")
}

func TestValidateTags_ValidMultiSelectTags(t *testing.T) {
	tagSets := createTestTagSets()
	tags := []string{"envtype/production", "envtype/staging"}

	err := ValidateTags(tags, tagSets)

	assert.Nil(t, err)
}

func TestValidateTags_MixedTags(t *testing.T) {
	tagSets := createTestTagSets()
	tags := []string{"region/us-east", "envtype/production", "envtype/staging"}

	err := ValidateTags(tags, tagSets)

	assert.Nil(t, err)
}

func TestTags_ReturnsProvidedTagsWhenValid(t *testing.T) {
	tagSets := createTestTagSets()
	providedTags := []string{"region/us-east", "envtype/production"}

	result, err := Tags(nil, []string{}, providedTags, tagSets)

	assert.Nil(t, err)
	assert.Equal(t, providedTags, result)
}

func TestTags_ReturnsErrorWhenProvidedTagsInvalid(t *testing.T) {
	tagSets := createTestTagSets()
	providedTags := []string{"region/us-east", "region/us-west"}

	result, err := Tags(nil, []string{}, providedTags, tagSets)

	assert.NotNil(t, err)
	assert.Nil(t, result)
}

func TestValidateTags_FreeTextValid(t *testing.T) {
	freeTextTagSet := tagsets.NewTagSet("Customer")
	freeTextTagSet.Type = tagsets.TagSetTypeFreeText

	tagSets := []*tagsets.TagSet{freeTextTagSet}
	tags := []string{"customer/company a"}

	err := ValidateTags(tags, tagSets)

	assert.Nil(t, err)
}

func TestValidateTags_FreeTextMultipleError(t *testing.T) {
	freeTextTagSet := tagsets.NewTagSet("Customer")
	freeTextTagSet.Type = tagsets.TagSetTypeFreeText

	tagSets := []*tagsets.TagSet{freeTextTagSet}
	tags := []string{"customer/company a", "customer/company b"}

	err := ValidateTags(tags, tagSets)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "only one tag can be specified for free text tag set 'Customer'")
}

func TestValidateTags_FreeTextEmptyValueError(t *testing.T) {
	freeTextTagSet := tagsets.NewTagSet("Customer")
	freeTextTagSet.Type = tagsets.TagSetTypeFreeText

	tagSets := []*tagsets.TagSet{freeTextTagSet}
	tags := []string{"customer/"}

	err := ValidateTags(tags, tagSets)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "must have a value after the prefix")
}

func TestTags_FreeTextReturnsProvidedTags(t *testing.T) {
	freeTextTagSet := tagsets.NewTagSet("Customer")
	freeTextTagSet.Type = tagsets.TagSetTypeFreeText

	tagSets := []*tagsets.TagSet{freeTextTagSet}
	providedTags := []string{"customer/company a"}

	result, err := Tags(nil, []string{}, providedTags, tagSets)

	assert.Nil(t, err)
	assert.Equal(t, providedTags, result)
}

func TestTags_FreeTextPrompt(t *testing.T) {
	freeTextTagSet := tagsets.NewTagSet("Customer")
	freeTextTagSet.Type = tagsets.TagSetTypeFreeText

	tagSets := []*tagsets.TagSet{freeTextTagSet}

	pa := []*testutil.PA{
		testutil.NewInputPrompt("Customer (Free Text)", "Enter a free text value for Customer tag set", "company a"),
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	result, err := Tags(asker, []string{}, []string{}, tagSets)

	checkRemainingPrompts()
	assert.Nil(t, err)
	assert.Equal(t, []string{"Customer/company a"}, result)
}

func TestTags_FreeTextPromptWithCurrentValue(t *testing.T) {
	freeTextTagSet := tagsets.NewTagSet("Customer")
	freeTextTagSet.Type = tagsets.TagSetTypeFreeText

	tagSets := []*tagsets.TagSet{freeTextTagSet}
	currentTags := []string{"customer/old-value"}

	pa := []*testutil.PA{
		testutil.NewInputPrompt("Customer (Free Text) [current: old-value]", "Current value: old-value. Enter new value or leave empty to remove", "new-value"),
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	result, err := Tags(asker, currentTags, []string{}, tagSets)

	checkRemainingPrompts()
	assert.Nil(t, err)
	assert.Equal(t, []string{"Customer/new-value"}, result)
}

func TestTags_FreeTextPromptClearsExistingValue(t *testing.T) {
	freeTextTagSet := tagsets.NewTagSet("Customer")
	freeTextTagSet.Type = tagsets.TagSetTypeFreeText

	tagSets := []*tagsets.TagSet{freeTextTagSet}
	currentTags := []string{"customer/old-value"}

	pa := []*testutil.PA{
		testutil.NewInputPrompt("Customer (Free Text) [current: old-value]", "Current value: old-value. Enter new value or leave empty to remove", ""),
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	result, err := Tags(asker, currentTags, []string{}, tagSets)

	checkRemainingPrompts()
	assert.Nil(t, err)
	assert.Empty(t, result)
}

func TestValidateTags_ErrorOnTagNotBelongingToAvailableTagSet(t *testing.T) {
	tagSets := createTestTagSets()
	tags := []string{"department/engineering"}

	err := ValidateTags(tags, tagSets)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "does not belong to any tag set available for this resource")
}

func TestValidateTags_ErrorOnTagNotExistingInTagSet(t *testing.T) {
	tagSets := createTestTagSets()
	tags := []string{"region/eu-west"}

	err := ValidateTags(tags, tagSets)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "does not exist in tag set")
}
