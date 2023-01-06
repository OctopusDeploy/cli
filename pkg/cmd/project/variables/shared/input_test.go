package shared_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd/project/variables/shared"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPromptScopes(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewMultiSelectPrompt("Environment scope", "", []string{"test"}, []string{"test"}),
		testutil.NewMultiSelectPrompt("Process scope", "", []string{"Run, book, run"}, []string{"Run, book, run"}),
		testutil.NewMultiSelectPrompt("Channel scope", "", []string{"Default channel"}, []string{"Default channel"}),
		testutil.NewMultiSelectPrompt("Target scope", "", []string{"Deployment target"}, []string{"Deployment target"}),
		testutil.NewMultiSelectPrompt("Role scope", "", []string{"Role 1"}, []string{"Role 1"}),
		testutil.NewMultiSelectPrompt("Tag scope", "", []string{"tag set/tag 1"}, []string{"tag set/tag 1"}),
		testutil.NewMultiSelectPrompt("Step scope", "", []string{"Step name"}, []string{"Step name"}),
	}

	variable := variables.NewVariable("")
	variable.ID = "123abc"
	variableSet := &variables.VariableSet{
		OwnerID: "Projects-1",
		ScopeValues: &variables.VariableScopeValues{
			Environments: []*resources.ReferenceDataItem{{ID: "Environments-1", Name: "test"}},
			Actions:      []*resources.ReferenceDataItem{{ID: "actionId", Name: "Step name"}},
			Channels:     []*resources.ReferenceDataItem{{ID: "Channels-1", Name: "Default channel"}},
			Machines:     []*resources.ReferenceDataItem{{ID: "Machines-1", Name: "Deployment target"}},
			TenantTags:   []*resources.ReferenceDataItem{{ID: "tag set/tag 1", Name: "tag 1"}},
			Roles:        []*resources.ReferenceDataItem{{ID: "Role 1", Name: "Role 1"}},
			Processes:    []*resources.ProcessReferenceDataItem{{ID: "Runbooks-1", Name: "Run, book, run"}},
		},
		SpaceID:   "Spaces-1",
		Variables: []*variables.Variable{variable},
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	flags := shared.NewScopeFlags()
	err := shared.PromptScopes(asker, variableSet, flags, false)

	assert.NoError(t, err)
	checkRemainingPrompts()
}

func TestPromptScopes_Prompted(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewMultiSelectPrompt("Environment scope", "", []string{"test"}, []string{"test"}),
		testutil.NewMultiSelectPrompt("Process scope", "", []string{"Run, book, run"}, []string{"Run, book, run"}),
	}

	variable := variables.NewVariable("")
	variable.ID = "123abc"
	variableSet := &variables.VariableSet{
		OwnerID: "Projects-1",
		ScopeValues: &variables.VariableScopeValues{
			Environments: []*resources.ReferenceDataItem{{ID: "Environments-1", Name: "test"}},
			Actions:      []*resources.ReferenceDataItem{{ID: "actionId", Name: "Step name"}},
			Channels:     []*resources.ReferenceDataItem{{ID: "Channels-1", Name: "Default channel"}},
			Machines:     []*resources.ReferenceDataItem{{ID: "Machines-1", Name: "Deployment target"}},
			TenantTags:   []*resources.ReferenceDataItem{{ID: "TenantTags-1", Name: "tag set/tag 1"}},
			Roles:        []*resources.ReferenceDataItem{{ID: "Role 1", Name: "Role 1"}},
			Processes:    []*resources.ProcessReferenceDataItem{{ID: "Runbooks-1", Name: "Run, book, run"}},
		},
		SpaceID:   "Spaces-1",
		Variables: []*variables.Variable{variable},
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	flags := shared.NewScopeFlags()
	err := shared.PromptScopes(asker, variableSet, flags, true)

	assert.NoError(t, err)
	checkRemainingPrompts()
}

func TestPromptScope_NoItems(t *testing.T) {
	pa := []*testutil.PA{}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	selectedValue, err := shared.PromptScope(asker, "Test", nil, nil)

	assert.NoError(t, err)
	checkRemainingPrompts()
	assert.Nil(t, selectedValue)
}

func TestPromptScope_HasItems(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewMultiSelectPrompt("Test scope", "", []string{"test", "not test"}, []string{"test", "not test"}),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	selectedValue, err := shared.PromptScope(asker, "Test", []*resources.ReferenceDataItem{
		{ID: "item-1", Name: "test"},
		{ID: "item-2", Name: "not test"},
	}, nil)

	assert.NoError(t, err)
	checkRemainingPrompts()
	assert.Equal(t, []string{"test", "not test"}, selectedValue)
}
