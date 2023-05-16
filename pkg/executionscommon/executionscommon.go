package executionscommon

// Contains code that is common between things in the executions API in the server,
// specifically `release deploy` and `runbook run`

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	cliErrors "github.com/OctopusDeploy/cli/pkg/errors"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
	"github.com/OctopusDeploy/cli/pkg/util"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/ztrue/tracerr"
	"sort"
	"strconv"
	"strings"
	"time"
)

func findTenantsAndTags(octopus *octopusApiClient.Client, projectID string, environmentIDs []string) ([]string, []string, error) {
	var validTenants []string
	var validTags []string // these are 'Canonical' values i.e. "Regions/us-east", NOT TagSets-1/Tags-1

	page, err := octopus.Tenants.Get(tenants.TenantsQuery{ProjectID: projectID})
	if err != nil {
		return nil, nil, tracerr.Wrap(err)
	}
	for page != nil {
		tenantsForMyEnvironments := make([]*tenants.Tenant, 0)
		for _, envID := range environmentIDs {
			tenantsForMyEnvironment := util.SliceFilter(page.Items, func(t *tenants.Tenant) bool {
				if envIdsForProject, ok := t.ProjectEnvironments[projectID]; ok {
					return util.SliceContains(envIdsForProject, envID)
				}
				return false
			})
			tenantsForMyEnvironments = append(tenantsForMyEnvironments, tenantsForMyEnvironment...)
		}
		for _, tenant := range tenantsForMyEnvironments {
			for _, tag := range tenant.TenantTags {
				if !util.SliceContains(validTags, tag) {
					validTags = append(validTags, tag)
				}
			}
			validTenants = append(validTenants, tenant.Name)
		}

		page, err = page.GetNextPage(octopus.Tenants.GetClient())
		if err != nil {
			return nil, nil, tracerr.Wrap(err)
		}
	}

	return validTenants, validTags, nil
}

func AskTenantsAndTags(asker question.Asker, octopus *octopusApiClient.Client, projectID string, env []*environments.Environment, required bool) ([]string, []string, error) {
	// (presumably though we can check if the project itself is linked to any tenants and only ask then)?
	// there is a ListTenants(projectID) api that we can use. /api/tenants?projectID=
	envIDs := util.SliceTransform(env, func(e *environments.Environment) string {
		return e.ID
	})
	foundTenants, foundTags, err := findTenantsAndTags(octopus, projectID, envIDs)
	if err != nil {
		return nil, nil, tracerr.Wrap(err)
	}

	// sort because otherwise they may appear in weird order
	sort.Strings(foundTenants)
	sort.Strings(foundTags)

	// Note: merging the list sets us up for a scenario where a tenant name could hypothetically collide with
	// a tag name; we wouldn't handle that -- in practice this is so unlikely to happen that we can ignore it
	combinedList := append(foundTenants, foundTags...)

	var selection []string
	if required {
		err = asker(&survey.MultiSelect{
			Message: "Select tenants and/or tags used to determine deployment targets",
			Options: combinedList,
		}, &selection, survey.WithValidator(survey.Required))
		if err != nil {
			return nil, nil, tracerr.Wrap(err)
		}
	} else {
		err = asker(&survey.MultiSelect{
			Message: "Select tenants and/or tags used to determine deployment targets",
			Options: combinedList,
		}, &selection)
		if err != nil {
			return nil, nil, tracerr.Wrap(err)
		}
	}

	tenantsLookup := make(map[string]bool, len(foundTenants))
	for _, t := range foundTenants {
		tenantsLookup[t] = true
	}
	tagsLookup := make(map[string]bool, len(foundTags))
	for _, t := range foundTags {
		tagsLookup[t] = true
	}

	var selectedTenants []string
	var selectedTags []string

	for _, s := range selection {
		if tenantsLookup[s] {
			selectedTenants = append(selectedTenants, s)
		} else if tagsLookup[s] {
			selectedTags = append(selectedTags, s)
		}
	}

	return selectedTenants, selectedTags, nil
}

func AskExcludedSteps(asker question.Asker, steps []*deployments.DeploymentStep) ([]string, error) {
	stepsToExclude, err := question.MultiSelectMap(asker, "Steps to skip (If none selected, run all steps)", steps, func(s *deployments.DeploymentStep) string {
		return s.Name
	}, false)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	return util.SliceTransform(stepsToExclude, func(s *deployments.DeploymentStep) string {
		return s.Name // server expects us to send a list of step names
	}), nil
}

func AskPackageDownload(asker question.Asker) (bool, error) {
	result, err := question.SelectMap(asker, "Package download", []bool{true, false}, LookupPackageDownloadString)
	// our question is phrased such that "Use cached packages" (the do-nothing option) is true,
	// but we want to set the --force-package-download flag, so we need to invert the response
	return !result, tracerr.Wrap(err)
}

func AskGuidedFailureMode(asker question.Asker) (string, error) {
	modes := []string{
		"", "true", "false", // maps to a nullable bool in C#
	}
	return question.SelectMap(asker, "Guided Failure Mode", modes, LookupGuidedFailureModeString)
}

// AskVariables returns the map of ALL variables to send to the server, whether they were prompted for, or came from the command line.
// variablesFromCmd is copied into the result, you don't need to merge them yourselves.
// Return values: 0: Variables to send to the server, 1: List of sensitive variable names for masking automation command, 2: error
func AskVariables(asker question.Asker, variableSet *variables.VariableSet, variablesFromCmd map[string]string) (map[string]string, error) {
	if asker == nil {
		return nil, cliErrors.NewArgumentNullOrEmptyError("asker")
	}
	if variableSet == nil {
		return nil, cliErrors.NewArgumentNullOrEmptyError("variableSet")
	}

	// variablesFromCmd is pure user input and may not have correct casing.
	lcaseVarsFromCmd := make(map[string]string, len(variablesFromCmd))
	for k, v := range variablesFromCmd {
		lcaseVarsFromCmd[strings.ToLower(k)] = v
	}

	result := make(map[string]string)
	if len(variableSet.Variables) > 0 { // nothing to be done here, move along
		for _, v := range variableSet.Variables {
			valueFromCmd, foundValueOnCommandLine := lcaseVarsFromCmd[strings.ToLower(v.Name)]
			if foundValueOnCommandLine {
				// implicitly fixes up variable casing
				result[v.Name] = valueFromCmd
			}

			if v.Prompt != nil && !foundValueOnCommandLine { // this is a prompted variable, ask for input (unless we already have it)
				// NOTE: there is a v.Prompt.Label which is shown in the web portal,
				// but we explicitly don't use it here because it can lead to confusion.
				// e.g.
				// A variable "CrmTicketNumber" exists with Label "CRM Ticket Number"
				// If we were to use the label then the prompt would ask for "CRM Ticket Number" but the command line
				// invocation would say "CrmTicketNumber:<value>" and it wouldn't be clear to and end user whether
				// "CrmTicketNumber" or "CRM Ticket Number" was the right thing.
				promptMessage := v.Name

				if v.Prompt.Description != "" {
					promptMessage = fmt.Sprintf("%s (%s)", promptMessage, v.Prompt.Description) // we'd like to dim the description, but survey overrides this, so we can't
				}

				if v.Type == "String" || v.Type == "Sensitive" {
					responseString, err := askVariableSpecificPrompt(asker, promptMessage, v.Type, v.Value, v.Prompt.IsRequired, v.IsSensitive, v.Prompt.DisplaySettings)
					if err != nil {
						return nil, tracerr.Wrap(err)
					}
					result[v.Name] = responseString
				}
				// else it's a complex variable type with the prompt flag, which (at time of writing) is currently broken
				// and a decision on how to fix it had not yet been made. Ignore it.
				// BUG: https://github.com/OctopusDeploy/Issues/issues/7699
			}
		}
	}
	return result, nil
}

func askVariableSpecificPrompt(asker question.Asker, message string, variableType string, defaultValue string, isRequired bool, isSensitive bool, displaySettings *variables.DisplaySettings) (string, error) {
	var askOpt survey.AskOpt = func(options *survey.AskOptions) error {
		if isRequired {
			options.Validators = append(options.Validators, survey.Required)
		}
		return nil
	}

	// work out what kind of prompt to use
	var controlType variables.ControlType
	if displaySettings != nil && displaySettings.ControlType != "" {
		controlType = displaySettings.ControlType
	} else { // infer the control type based on other flags
		// The shape of the data model allows for the possibility of a sensitive multi-line or sensitive combo-box
		// variable. However, the web portal doesn't implement any of these, the only sensitive thing it supports
		// is single-line text, so we can simplify our logic here.
		if variableType == "Sensitive" || isSensitive {
			// From comment in server:
			// variable.IsSensitive is Kept for backwards compatibility. New way is to use variable.Type=VariableType.Sensitive
			controlType = variables.ControlTypeSensitive
		} else {
			controlType = variables.ControlTypeSingleLineText
		}
	}

	switch controlType {
	case variables.ControlTypeSingleLineText, "": // if control type is not explicitly set it means single line text.
		var response string
		err := asker(&survey.Input{
			Message: message,
			Default: defaultValue,
		}, &response, askOpt)
		return response, tracerr.Wrap(err)

	case variables.ControlTypeSensitive:
		var response string
		err := asker(&survey.Password{
			Message: message,
		}, &response, askOpt)
		return response, tracerr.Wrap(err)

	case variables.ControlTypeMultiLineText: // not clear if the server ever does this
		var response string
		err := asker(&surveyext.OctoEditor{
			Editor: &survey.Editor{
				Message:  "message",
				FileName: "*.txt",
			},
			Optional: !isRequired}, &response)
		return response, tracerr.Wrap(err)

	case variables.ControlTypeSelect:
		if displaySettings == nil {
			return "", cliErrors.NewArgumentNullOrEmptyError("displaySettings") // select needs actual display settings
		}
		reverseLookup := make(map[string]string, len(displaySettings.SelectOptions))
		optionStrings := make([]string, 0, len(displaySettings.SelectOptions))
		displayNameForDefaultValue := ""
		for _, v := range displaySettings.SelectOptions {
			if v.Value == defaultValue {
				displayNameForDefaultValue = v.DisplayName
			}
			optionStrings = append(optionStrings, v.DisplayName)
			reverseLookup[v.DisplayName] = v.Value
		}
		var response string
		err := asker(&survey.Select{
			Message: message,
			Default: displayNameForDefaultValue,
			Options: optionStrings,
		}, &response, askOpt)
		if err != nil {
			return "", tracerr.Wrap(err)
		}
		return reverseLookup[response], nil

	case variables.ControlTypeCheckbox:
		// if the server didn't specifically set a default value of True then default to No
		defTrueFalse := "False"
		if b, err := strconv.ParseBool(defaultValue); err == nil && b {
			defTrueFalse = "True"
		}
		var response string
		err := asker(&survey.Select{
			Message: message,
			Default: defTrueFalse,
			Options: []string{"True", "False"}, // Yes/No would read more nicely, but doesn't fit well with cmdline which expects True/False
		}, &response, askOpt)
		return response, tracerr.Wrap(err)

	default:
		return "", fmt.Errorf("unhandled control type %s", displaySettings.ControlType)
	}
}

func ParseVariableStringArray(variables []string) (map[string]string, error) {
	result := make(map[string]string, len(variables))
	for _, v := range variables {
		components := splitVariableString(v, 2)
		if len(components) != 2 || components[0] == "" || components[1] == "" {
			return nil, fmt.Errorf("could not parse variable definition '%s'", v)
		}
		result[strings.TrimSpace(components[0])] = strings.TrimSpace(components[1])
	}
	return result, nil
}

func ToVariableStringArray(variables map[string]string) []string {
	result := make([]string, 0, len(variables))
	for k, v := range variables {
		if k == "" || v == "" {
			continue
		}
		result = append(result, fmt.Sprintf("%s:%s", k, v))
	}
	sort.Strings(result) // sort for reliable test output
	return result
}

func LookupGuidedFailureModeString(value string) string {
	switch value {
	case "", "default":
		return "Use default setting from the target environment"
	case "true", "True":
		return "Use guided failure mode"
	case "false", "False":
		return "Do not use guided failure mode"
	default:
		return fmt.Sprintf("Unknown %s", value)
	}
}

func LookupPackageDownloadString(value bool) string {
	if value {
		return "Use cached packages (if available)"
	} else {
		return "Re-download packages from feed"
	}
}

// splitVariableString is a derivative of splitPackageOverrideString in release create.
// it is required because the builtin go strings.SplitN can't handle more than one delimeter character.
// otherwise it works the same, but caps the number of splits at 'n'
func splitVariableString(s string, n int) []string {
	// pass 1: collect spans; golang strings.FieldsFunc says it's much more efficient this way
	type span struct {
		start int
		end   int
	}
	spans := make([]span, 0, n)

	// Find the field start and end indices.
	start := 0 // we always start the first span at the beginning of the string
	for idx, ch := range s {
		if ch == ':' || ch == '=' {
			if start >= 0 { // we found a delimiter and we are already in a span; end the span and start a new one
				if len(spans) == n-1 { // we're about to append the last span, break so the 'last field' code consumes the rest of the string
					break
				} else {
					spans = append(spans, span{start, idx})
					start = idx + 1
				}
			} else { // we found a delimiter and we are not in a span; start a new span
				if start < 0 {
					start = idx
				}
			}
		}
	}

	// Last field might end at EOF.
	if start >= 0 {
		spans = append(spans, span{start, len(s)})
	}

	// pass 2: create strings from recorded field indices.
	a := make([]string, len(spans))
	for i, span := range spans {
		a[i] = s[span.start:span.end]
	}
	return a
}

// ScheduledStartTimeAnswerFormatter is passed to the DatePicker so that if the user selects a time within the next
// one minute after 'now', it will show the answer as the string "Now" rather than the actual datetime string
func ScheduledStartTimeAnswerFormatter(datePicker *surveyext.DatePicker, t time.Time) string {
	if t.Before(datePicker.Now().Add(1 * time.Minute)) {
		return "Now"
	} else {
		return t.String()
	}
}

// given an array of environment names, maps these all to actual objects by querying the server
func FindEnvironments(client *octopusApiClient.Client, environmentNamesOrIds []string) ([]*environments.Environment, error) {
	if len(environmentNamesOrIds) == 0 {
		return nil, nil
	}
	// there's no "bulk lookup" API, so we either need to do a foreach loop to find each environment individually, or load the entire server's worth of environments
	// it's probably going to be cheaper to just list out all the environments and match them client side, so we'll do that for simplicity's sake
	allEnvs, err := client.Environments.GetAll()
	if err != nil {
		return nil, tracerr.Wrap(err)
	}

	nameLookup := make(map[string]*environments.Environment, len(allEnvs))
	idLookup := make(map[string]*environments.Environment, len(allEnvs))

	for _, env := range allEnvs {
		nameLookup[strings.ToLower(env.GetName())] = env
		idLookup[strings.ToLower(env.GetID())] = env
	}

	var result []*environments.Environment
	for _, n := range environmentNamesOrIds {
		nameOrId := strings.ToLower(n)
		env := nameLookup[nameOrId]
		if env != nil {
			result = append(result, env)
		} else {
			env = idLookup[nameOrId]
			if env != nil {
				result = append(result, env)
			} else {
				return nil, tracerr.Wrap(fmt.Errorf("cannot find environment %s", nameOrId))
			}
		}
	}
	return result, nil
}
