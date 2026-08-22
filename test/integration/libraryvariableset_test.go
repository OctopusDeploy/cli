package integration_test

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/OctopusDeploy/cli/test/integration"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// the space already holds variable sets, so every assertion here is scoped to
// fixtures named with the run id rather than to whole-command output
type lvsFixture struct {
	Environment  *environments.Environment
	Set          *variables.LibraryVariableSet
	ScriptModule *variables.LibraryVariableSet
	Empty        *variables.LibraryVariableSet
}

func createLibraryVariableSetFixture(t *testing.T, runId uuid.UUID) *lvsFixture {
	apiClient, err := integration.GetApiClient(space1ID)
	testutil.RequireSuccess(t, err)

	// our own environment, so a resolved scope name is one we control
	env, err := apiClient.Environments.Add(environments.NewEnvironment(fmt.Sprintf("lvsenv-%s", runId)))
	testutil.RequireSuccess(t, err)
	t.Cleanup(func() { assert.Nil(t, apiClient.Environments.DeleteByID(env.GetID())) })

	set := variables.NewLibraryVariableSet(fmt.Sprintf("lvs-%s", runId))
	set.Description = "set under test"
	set, err = apiClient.LibraryVariableSets.Add(set)
	testutil.RequireSuccess(t, err)
	t.Cleanup(func() { assert.Nil(t, apiClient.LibraryVariableSets.DeleteByID(set.GetID())) })

	// script modules share the libraryvariablesets endpoint; list must skip it
	// and view must not resolve it
	module := variables.NewLibraryVariableSet(fmt.Sprintf("lvsmodule-%s", runId))
	module.ContentType = "ScriptModule"
	module, err = apiClient.LibraryVariableSets.Add(module)
	testutil.RequireSuccess(t, err)
	t.Cleanup(func() { assert.Nil(t, apiClient.LibraryVariableSets.DeleteByID(module.GetID())) })

	empty, err := apiClient.LibraryVariableSets.Add(variables.NewLibraryVariableSet(fmt.Sprintf("lvsempty-%s", runId)))
	testutil.RequireSuccess(t, err)
	t.Cleanup(func() { assert.Nil(t, apiClient.LibraryVariableSets.DeleteByID(empty.GetID())) })

	unscoped := variables.NewVariable("Slack.Url")
	unscoped.Value = "https://default"

	scopedToEnv := variables.NewVariable("Slack.Url")
	scopedToEnv.Value = "https://prod"
	scopedToEnv.Scope = variables.VariableScope{Environments: []string{env.GetID()}}

	scopedToEnvAndRole := variables.NewVariable("Slack.Url")
	scopedToEnvAndRole.Value = "https://dev"
	scopedToEnvAndRole.Scope = variables.VariableScope{Environments: []string{env.GetID()}, Roles: []string{"web-server"}}

	sensitive := variables.NewVariable("Slack.Token")
	sensitive.Value = "s3cret"
	sensitive.Type = "Sensitive"
	sensitive.IsSensitive = true

	prompted := variables.NewVariable("Ask.Me")
	prompted.Prompt = &variables.VariablePromptOptions{Label: "Give me a value", IsRequired: true}

	variableSet, err := apiClient.Variables.GetAll(set.GetID())
	testutil.RequireSuccess(t, err)
	variableSet.Variables = []*variables.Variable{unscoped, scopedToEnv, scopedToEnvAndRole, sensitive, prompted}
	_, err = apiClient.Variables.Update(set.GetID(), variableSet)
	testutil.RequireSuccess(t, err)

	return &lvsFixture{Environment: env, Set: set, ScriptModule: module, Empty: empty}
}

var tableColumnSeparator = regexp.MustCompile(`\s{2,}`)

// tableRows splits table output into rows of trimmed cells. Variable IDs are
// GUIDs, so tests match on cells rather than on whole lines.
func tableRows(stdOut string) [][]string {
	rows := [][]string{}
	for _, line := range strings.Split(strings.TrimRight(stdOut, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		cells := tableColumnSeparator.Split(strings.TrimRight(line, " "), -1)
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		rows = append(rows, cells)
	}
	return rows
}

type lvsListItem struct {
	Id            string
	Name          string
	Description   string
	VariableSetId string
}

type lvsViewScope struct {
	Environment []string
	Role        []string
}

type lvsViewValue struct {
	Id           string
	Value        string
	IsSensitive  bool
	Type         string
	IsScoped     bool
	Scope        *lvsViewScope
	ScopeSummary string
	Prompt       *variables.VariablePromptOptions
}

type lvsViewGroup struct {
	Name   string
	Values []lvsViewValue
}

type lvsView struct {
	Id            string
	Name          string
	Description   string
	SpaceId       string
	VariableSetId string
	TemplateCount int
	Variables     []lvsViewGroup
	WebUrl        string
}

func TestLibraryVariableSet(t *testing.T) {
	runId := uuid.New()
	fx := createLibraryVariableSetFixture(t, runId)

	t.Run("list", func(t *testing.T) { testLibraryVariableSetList(t, runId, fx) })
	t.Run("view", func(t *testing.T) { testLibraryVariableSetView(t, fx) })
}

func testLibraryVariableSetList(t *testing.T, runId uuid.UUID, fx *lvsFixture) {
	t.Run("--filter finds the set and excludes the script module", func(t *testing.T) {
		stdOut, stdErr, err := integration.RunCli("Default", "library-variable-set", "list", "--filter", runId.String(), "--output-format=basic")
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}
		// lvsempty-<id> and lvs-<id> both match the run id; the script module does not appear
		assert.Equal(t, []string{fx.Set.Name, fx.Empty.Name}, strings.Fields(strings.TrimSpace(stdOut)))
		assert.NotContains(t, stdOut, fx.ScriptModule.Name)
	})

	t.Run("--output-format=table", func(t *testing.T) {
		stdOut, stdErr, err := integration.RunCli("Default", "library-variable-set", "list", "--filter", fx.Set.Name, "--output-format=table")
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}
		rows := tableRows(stdOut)
		require.Len(t, rows, 2)
		assert.Equal(t, []string{"NAME", "DESCRIPTION", "ID"}, rows[0])
		assert.Equal(t, []string{fx.Set.Name, "set under test", fx.Set.GetID()}, rows[1])
	})

	t.Run("--output-format=json uses the API's field casing", func(t *testing.T) {
		stdOut, stdErr, err := integration.RunCliRawOutput("Default", "library-variable-set", "list", "--filter", fx.Set.Name, "--output-format=json")
		if !testutil.AssertSuccess(t, err, string(stdOut), string(stdErr)) {
			return
		}
		var results []lvsListItem
		require.Nil(t, json.Unmarshal(stdOut, &results))
		assert.Equal(t, []lvsListItem{{
			Id:            fx.Set.GetID(),
			Name:          fx.Set.Name,
			Description:   "set under test",
			VariableSetId: fx.Set.VariableSetID,
		}}, results)

		// Id/VariableSetId, not the Go field names, matching every other list command
		assert.Contains(t, string(stdOut), `"Id"`)
		assert.Contains(t, string(stdOut), `"VariableSetId"`)
	})
}

func testLibraryVariableSetView(t *testing.T, fx *lvsFixture) {
	t.Run("--output-format=table groups values under one name", func(t *testing.T) {
		stdOut, stdErr, err := integration.RunCli("Default", "library-variable-set", "view", fx.Set.Name, "--output-format=table")
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}
		rows := tableRows(stdOut)
		require.Len(t, rows, 6)
		assert.Equal(t, []string{"NAME", "VALUE", "SCOPE", "ID"}, rows[0])

		// prompted variable has no value; sensitive one is masked
		assert.Equal(t, "Ask.Me", rows[1][0])
		assert.Equal(t, []string{"Slack.Token", "***", "(unscoped)"}, rows[2][:3])

		// the three Slack.Url values share one name cell: unscoped first, then
		// scoped ones ordered by scope summary
		assert.Equal(t, []string{"Slack.Url", "https://default", "(unscoped)"}, rows[3][:3])
		assert.Equal(t, []string{"", "https://prod", fmt.Sprintf("Environment: %s", fx.Environment.Name)}, rows[4][:3])
		assert.Equal(t, []string{"", "https://dev", fmt.Sprintf("Environment: %s; Role: web-server", fx.Environment.Name)}, rows[5][:3])
	})

	t.Run("--output-format=json resolves scope IDs to names", func(t *testing.T) {
		stdOut, stdErr, err := integration.RunCliRawOutput("Default", "library-variable-set", "view", fx.Set.Name, "--output-format=json")
		if !testutil.AssertSuccess(t, err, string(stdOut), string(stdErr)) {
			return
		}
		var result lvsView
		require.Nil(t, json.Unmarshal(stdOut, &result))

		assert.Equal(t, fx.Set.GetID(), result.Id)
		assert.Equal(t, fx.Set.Name, result.Name)
		assert.Equal(t, "set under test", result.Description)
		assert.Equal(t, space1ID, result.SpaceId)
		assert.Equal(t, fx.Set.VariableSetID, result.VariableSetId)
		assert.Equal(t, 0, result.TemplateCount)
		assert.Contains(t, result.WebUrl, fmt.Sprintf("library/variablesets/%s", fx.Set.GetID()))

		require.Len(t, result.Variables, 3)
		assert.Equal(t, []string{"Ask.Me", "Slack.Token", "Slack.Url"},
			[]string{result.Variables[0].Name, result.Variables[1].Name, result.Variables[2].Name})

		prompted := result.Variables[0]
		require.Len(t, prompted.Values, 1)
		require.NotNil(t, prompted.Values[0].Prompt)
		assert.Equal(t, "Give me a value", prompted.Values[0].Prompt.Label)

		// the server never returns a sensitive value, so json carries the flag not the mask
		sensitive := result.Variables[1]
		require.Len(t, sensitive.Values, 1)
		assert.True(t, sensitive.Values[0].IsSensitive)
		assert.Equal(t, "", sensitive.Values[0].Value)
		assert.Equal(t, "Sensitive", sensitive.Values[0].Type)

		urls := result.Variables[2]
		require.Len(t, urls.Values, 3)
		for _, v := range urls.Values {
			assert.NotEmpty(t, v.Id)
		}

		assert.False(t, urls.Values[0].IsScoped)
		assert.Nil(t, urls.Values[0].Scope)
		assert.Equal(t, "(unscoped)", urls.Values[0].ScopeSummary)

		assert.True(t, urls.Values[1].IsScoped)
		require.NotNil(t, urls.Values[1].Scope)
		assert.Equal(t, []string{fx.Environment.Name}, urls.Values[1].Scope.Environment)
		assert.Nil(t, urls.Values[1].Scope.Role)

		require.NotNil(t, urls.Values[2].Scope)
		assert.Equal(t, []string{fx.Environment.Name}, urls.Values[2].Scope.Environment)
		assert.Equal(t, []string{"web-server"}, urls.Values[2].Scope.Role)
	})

	t.Run("resolves by ID as well as by name", func(t *testing.T) {
		stdOut, stdErr, err := integration.RunCliRawOutput("Default", "library-variable-set", "view", fx.Set.GetID(), "--output-format=json")
		if !testutil.AssertSuccess(t, err, string(stdOut), string(stdErr)) {
			return
		}
		var result lvsView
		require.Nil(t, json.Unmarshal(stdOut, &result))
		assert.Equal(t, fx.Set.Name, result.Name)
	})

	t.Run("--filter narrows to matching variables", func(t *testing.T) {
		stdOut, stdErr, err := integration.RunCliRawOutput("Default", "library-variable-set", "view", fx.Set.Name, "--filter", "Token", "--output-format=json")
		if !testutil.AssertSuccess(t, err, string(stdOut), string(stdErr)) {
			return
		}
		var result lvsView
		require.Nil(t, json.Unmarshal(stdOut, &result))
		require.Len(t, result.Variables, 1)
		assert.Equal(t, "Slack.Token", result.Variables[0].Name)
	})

	t.Run("--filter matching nothing still emits an array", func(t *testing.T) {
		stdOut, stdErr, err := integration.RunCliRawOutput("Default", "library-variable-set", "view", fx.Set.Name, "--filter", "no-such-variable", "--output-format=json")
		if !testutil.AssertSuccess(t, err, string(stdOut), string(stdErr)) {
			return
		}
		assert.Contains(t, string(stdOut), `"Variables": []`)
	})

	t.Run("a set with no variables", func(t *testing.T) {
		stdOut, stdErr, err := integration.RunCli("Default", "library-variable-set", "view", fx.Empty.Name, "--output-format=basic")
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}
		assert.Contains(t, stdOut, fx.Empty.Name)
		assert.Contains(t, stdOut, "No variables")
	})

	t.Run("--output-format=basic", func(t *testing.T) {
		stdOut, stdErr, err := integration.RunCli("Default", "library-variable-set", "view", fx.Set.Name, "--output-format=basic")
		if !testutil.AssertSuccess(t, err, stdOut, stdErr) {
			return
		}
		assert.Contains(t, stdOut, fmt.Sprintf("%s (%s)", fx.Set.Name, fx.Set.GetID()))
		assert.Contains(t, stdOut, "(unscoped) = https://default")
		assert.Contains(t, stdOut, fmt.Sprintf("Environment: %s = https://prod", fx.Environment.Name))
		assert.Contains(t, stdOut, "(unscoped) = ***")
		assert.Contains(t, stdOut, "prompted at deployment time")
	})

	t.Run("errors", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			args     []string
			expected string
		}{
			{"script module by name", []string{fx.ScriptModule.Name}, fmt.Sprintf("cannot find library variable set '%s'", fx.ScriptModule.Name)},
			{"script module by ID", []string{fx.ScriptModule.GetID()}, fmt.Sprintf("cannot find library variable set '%s'", fx.ScriptModule.GetID())},
			{"unknown name", []string{"no-such-set"}, "cannot find library variable set 'no-such-set'"},
			{"no identifier without prompting", []string{}, "library variable set must be specified"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				args := append([]string{"library-variable-set", "view"}, tc.args...)
				stdOut, stdErr, err := integration.RunCli("Default", args...)
				assert.Error(t, err, stdOut)
				assert.Contains(t, stdOut+stdErr, tc.expected)
			})
		}
	})
}
