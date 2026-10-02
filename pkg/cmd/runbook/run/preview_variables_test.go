package run

import (
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/core"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/stretchr/testify/assert"
)

func noPromptAsker(t *testing.T) question.Asker {
	t.Helper()
	return func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		t.Fatalf("unexpected prompt: %T %#v", p, p)
		return nil
	}
}

func cannedAsker(answer interface{}) question.Asker {
	return func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		return core.WriteAnswer(response, "", answer)
	}
}

// Regression test for issue #582: command-line --variable values that don't
// match a runbook form preview control were being silently dropped in
// interactive mode, causing the run to fall back to default values.
func TestResolveRunbookPreviewVariables_PreservesCmdVarWithoutMatchingControl(t *testing.T) {
	controls := map[string]*deployments.Control{}
	values := map[string]string{}
	cmdVars := map[string]string{
		"Approver": "John",
		"Signoff":  "Jane",
	}

	result, sensitive, err := resolveRunbookPreviewVariables(noPromptAsker(t), controls, values, cmdVars)

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"Approver": "John", "Signoff": "Jane"}, result)
	assert.Empty(t, sensitive)
}

func TestResolveRunbookPreviewVariables_CanonicalisesCasingForMatchedControl(t *testing.T) {
	approver := deployments.NewControl("VariableValue", "Approver", "", "", false, &resources.DisplaySettings{})
	controls := map[string]*deployments.Control{"elem-1": approver}
	values := map[string]string{"elem-1": ""}
	cmdVars := map[string]string{"APPROVER": "John"}

	result, _, err := resolveRunbookPreviewVariables(noPromptAsker(t), controls, values, cmdVars)

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"Approver": "John"}, result)
}

func TestResolveRunbookPreviewVariables_DoesNotPromptWhenCmdVarSuppliedForRequiredControl(t *testing.T) {
	approver := deployments.NewControl("VariableValue", "Approver", "", "", true, &resources.DisplaySettings{})
	controls := map[string]*deployments.Control{"elem-1": approver}
	values := map[string]string{"elem-1": ""}
	cmdVars := map[string]string{"Approver": "John"}

	result, _, err := resolveRunbookPreviewVariables(noPromptAsker(t), controls, values, cmdVars)

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"Approver": "John"}, result)
}

func TestResolveRunbookPreviewVariables_TracksSensitiveControl(t *testing.T) {
	token := deployments.NewControl("VariableValue", "Token", "", "", true, resources.NewDisplaySettings(resources.ControlTypeSensitive, nil))
	controls := map[string]*deployments.Control{"elem-1": token}
	values := map[string]string{"elem-1": ""}
	cmdVars := map[string]string{"Token": "secret"}

	result, sensitive, err := resolveRunbookPreviewVariables(noPromptAsker(t), controls, values, cmdVars)

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"Token": "secret"}, result)
	assert.Equal(t, []string{"Token"}, sensitive)
}

func TestResolveRunbookPreviewVariables_PromptsForRequiredControlWithoutCmdVar(t *testing.T) {
	approver := deployments.NewControl("VariableValue", "Approver", "", "", true, &resources.DisplaySettings{})
	controls := map[string]*deployments.Control{"elem-1": approver}
	values := map[string]string{"elem-1": ""}
	cmdVars := map[string]string{}

	result, _, err := resolveRunbookPreviewVariables(cannedAsker("John"), controls, values, cmdVars)

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"Approver": "John"}, result)
}

func TestResolveRunbookPreviewVariables_PreservesUnmatchedAndCanonicalisesMatched(t *testing.T) {
	approver := deployments.NewControl("VariableValue", "Approver", "", "", false, &resources.DisplaySettings{})
	controls := map[string]*deployments.Control{"elem-1": approver}
	values := map[string]string{"elem-1": ""}
	cmdVars := map[string]string{
		"ApprOVER": "John",
		"extra":    "passthrough",
	}

	result, _, err := resolveRunbookPreviewVariables(noPromptAsker(t), controls, values, cmdVars)

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"Approver": "John", "extra": "passthrough"}, result)
}
