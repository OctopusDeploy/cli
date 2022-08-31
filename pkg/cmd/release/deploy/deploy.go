package deploy

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	cliErrors "github.com/OctopusDeploy/cli/pkg/errors"
	"github.com/OctopusDeploy/cli/pkg/executor"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/cobra"
	"io"
	"strings"
)

// octopus release deploy <stuff>
// octopus release deploy-tenanted <stuff>

const (

	// NO
	// TODO force?

	// YES; Prompted Variable Only: read about prompted variables (user has to input them during deployment)
	// TODO variable(s)?
	// TODO updateVariables?

	FlagProject = "project"

	FlagReleaseVersion           = "version"
	FlagAliasReleaseNumberLegacy = "releaseNumber"

	FlagEnvironment         = "environment" // can be specified multiple times; but only once if tenanted
	FlagAliasEnv            = "env"
	FlagAliasDeployToLegacy = "deployTo" // alias for environment

	FlagTenant = "tenant" // can be specified multiple times

	FlagTenantTag            = "tenant-tag" // can be specified multiple times
	FlagAliasTag             = "tag"
	FlagAliasTenantTagLegacy = "tenantTag"

	FlagDeployAt            = "deploy-at"
	FlagAliasWhen           = "when" // alias for deploy-at
	FlagAliasDeployAtLegacy = "deployAt"

	// Don't ask this question in interactive mode; leave it for advanced. TODO review later with team
	FlagMaxQueueTime             = "max-queue-time" // should we have this as --no-deploy-after instead?
	FlagAliasNoDeployAfterLegacy = "noDeployAfter"  // max queue time

	FlagSkip = "skip" // can be specified multiple times

	FlagGuidedFailure                = "guided-failure"
	FlagAliasGuidedFailureMode       = "guided-failure-mode"
	FlagAliasGuidedFailureModeLegacy = "guidedFailure"

	FlagForcePackageDownload            = "force-package-download"
	FlagAliasForcePackageDownloadLegacy = "forcePackageDownload"

	FlagDeploymentTargets           = "targets" // specific machines
	FlagAliasSpecificMachinesLegacy = "specificMachines"

	FlagExcludeDeploymentTargets   = "exclude-targets"
	FlagAliasExcludeMachinesLegacy = "excludeMachines"

	FlagVariable = "variable"

	FlagUpdateVariables            = "update-variables"
	FlagAliasUpdateVariablesLegacy = "updateVariables"
)

// executions API stops here.

// DEPLOYMENT TRACKING (Server Tasks): - this might be a separate `octopus task follow ID1, ID2, ID3`

// DESIGN CHOICE: We are not going to show servertask progress in the CLI. We will need to optionally wait for deployments to complete though

// TODO progress? // OUT?

// TODO deploymentTimeout? (default to 10m)
// TODO cancelOnTimeout? (default to true)

// TODO deploymentCheckSleepCycle?
// TODO waitForDeployment?

type DeployFlags struct {
	Project              *flag.Flag[string]
	ReleaseVersion       *flag.Flag[string]   // the release to deploy
	Environments         *flag.Flag[[]string] // multiple for untenanted deployment
	Tenants              *flag.Flag[[]string]
	TenantTags           *flag.Flag[[]string]
	DeployAt             *flag.Flag[string]
	MaxQueueTime         *flag.Flag[string]
	Variables            *flag.Flag[[]string]
	UpdateVariables      *flag.Flag[bool]
	ExcludedSteps        *flag.Flag[[]string]
	GuidedFailureMode    *flag.Flag[string] // tri-state: true, false, or "use default". Can we model it with an optional bool?
	ForcePackageDownload *flag.Flag[bool]
	DeploymentTargets    *flag.Flag[[]string]
	ExcludeTargets       *flag.Flag[[]string]
	// TODO what about deployment targets per tenant? How do you specify that on the cmdline? Look at octo
}

func NewDeployFlags() *DeployFlags {
	return &DeployFlags{
		Project:              flag.New[string](FlagProject, false),
		ReleaseVersion:       flag.New[string](FlagReleaseVersion, false),
		Environments:         flag.New[[]string](FlagEnvironment, false),
		Tenants:              flag.New[[]string](FlagTenant, false),
		TenantTags:           flag.New[[]string](FlagTenantTag, false),
		MaxQueueTime:         flag.New[string](FlagMaxQueueTime, false),
		DeployAt:             flag.New[string](FlagDeployAt, false),
		Variables:            flag.New[[]string](FlagVariable, false),
		UpdateVariables:      flag.New[bool](FlagUpdateVariables, false),
		ExcludedSteps:        flag.New[[]string](FlagSkip, false),
		GuidedFailureMode:    flag.New[string](FlagGuidedFailure, false),
		ForcePackageDownload: flag.New[bool](FlagForcePackageDownload, false),
		DeploymentTargets:    flag.New[[]string](FlagDeploymentTargets, false),
		ExcludeTargets:       flag.New[[]string](FlagExcludeDeploymentTargets, false),
	}
}

func NewCmdDeploy(f factory.Factory) *cobra.Command {
	deployFlags := NewDeployFlags()
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy releases in Octopus Deploy",
		Long:  "Deploy releases in Octopus Deploy.",
		Example: heredoc.Doc(`
			$ octopus release deploy: TODO
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && deployFlags.Project.Value == "" {
				deployFlags.Project.Value = args[0]
			}

			return deployRun(cmd, f, deployFlags)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&deployFlags.Project.Value, deployFlags.Project.Name, "p", "", "Name or ID of the project to deploy the release from")
	flags.StringVarP(&deployFlags.ReleaseVersion.Value, deployFlags.ReleaseVersion.Name, "", "", "Release version to deploy")
	flags.StringSliceVarP(&deployFlags.Environments.Value, deployFlags.Environments.Name, "e", nil, "Deploy to this environment (can be specified multiple times)")
	flags.StringSliceVarP(&deployFlags.Tenants.Value, deployFlags.Tenants.Name, "", nil, "Deploy to this tenant (can be specified multiple times)")
	flags.StringSliceVarP(&deployFlags.TenantTags.Value, deployFlags.TenantTags.Name, "", nil, "Deploy to tenants matching this tag (can be specified multiple times)")
	flags.StringVarP(&deployFlags.DeployAt.Value, deployFlags.DeployAt.Name, "", "", "Deploy at a later time. Deploy now if omitted. TODO date formats and timezones!")
	flags.StringVarP(&deployFlags.MaxQueueTime.Value, deployFlags.MaxQueueTime.Name, "", "", "Cancel the deployment if it hasn't started within this time period.")
	flags.StringSliceVarP(&deployFlags.Variables.Value, deployFlags.Variables.Name, "v", nil, "Set the value for a prompted variable in the format Label:Value")
	flags.BoolVarP(&deployFlags.UpdateVariables.Value, deployFlags.UpdateVariables.Name, "", false, "Overwrite the release variable snapshot by re-importing variables from the project.")
	flags.StringSliceVarP(&deployFlags.ExcludedSteps.Value, deployFlags.ExcludedSteps.Name, "", nil, "Exclude specific steps from the deployment")
	flags.StringVarP(&deployFlags.GuidedFailureMode.Value, deployFlags.GuidedFailureMode.Name, "", "", "Enable Guided failure mode (yes/no/default)")
	flags.BoolVarP(&deployFlags.ForcePackageDownload.Value, deployFlags.ForcePackageDownload.Name, "", false, "Force re-download of packages")
	flags.StringSliceVarP(&deployFlags.DeploymentTargets.Value, deployFlags.DeploymentTargets.Name, "", nil, "Deploy to this target (can be specified multiple times)")
	flags.StringSliceVarP(&deployFlags.ExcludeTargets.Value, deployFlags.ExcludeTargets.Name, "", nil, "Deploy to targets except for this (can be specified multiple times)")

	flags.SortFlags = false

	// flags aliases for compat with old .NET CLI
	flagAliases := make(map[string][]string, 10)
	util.AddFlagAliasesString(flags, FlagReleaseVersion, flagAliases, FlagAliasReleaseNumberLegacy)
	util.AddFlagAliasesString(flags, FlagEnvironment, flagAliases, FlagAliasDeployToLegacy, FlagAliasEnv)
	util.AddFlagAliasesString(flags, FlagTenantTag, flagAliases, FlagAliasTag, FlagAliasTenantTagLegacy)
	util.AddFlagAliasesString(flags, FlagDeployAt, flagAliases, FlagAliasWhen, FlagAliasDeployAtLegacy)
	util.AddFlagAliasesString(flags, FlagMaxQueueTime, flagAliases, FlagAliasNoDeployAfterLegacy)
	util.AddFlagAliasesString(flags, FlagUpdateVariables, flagAliases, FlagAliasUpdateVariablesLegacy)
	util.AddFlagAliasesString(flags, FlagGuidedFailure, flagAliases, FlagAliasGuidedFailureMode, FlagAliasGuidedFailureModeLegacy)
	util.AddFlagAliasesBool(flags, FlagForcePackageDownload, flagAliases, FlagAliasForcePackageDownloadLegacy)
	util.AddFlagAliasesBool(flags, FlagDeploymentTargets, flagAliases, FlagAliasSpecificMachinesLegacy)
	util.AddFlagAliasesBool(flags, FlagExcludeDeploymentTargets, flagAliases, FlagAliasExcludeMachinesLegacy)

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		// map alias values
		for k, v := range flagAliases {
			for _, aliasName := range v {
				f := cmd.Flags().Lookup(aliasName)
				r := f.Value.String() // boolean flags get stringified here but it's fast enough and a one-shot so meh
				if r != f.DefValue {
					_ = cmd.Flags().Lookup(k).Value.Set(r)
				}
			}
		}
		return nil
	}
	return cmd
}

func deployRun(cmd *cobra.Command, f factory.Factory, flags *DeployFlags) error {
	outputFormat, err := cmd.Flags().GetString(constants.FlagOutputFormat)
	if err != nil { // should never happen, but fallback if it does
		outputFormat = constants.OutputFormatTable
	}

	octopus, err := f.GetSpacedClient()
	if err != nil {
		return err
	}

	parsedVariables, err := ParseVariableStringArray(flags.Variables.Value)
	if err != nil {
		return err
	}

	options := &executor.TaskOptionsDeployRelease{
		ProjectName:          flags.Project.Value,
		ReleaseVersion:       flags.ReleaseVersion.Value,
		Environments:         flags.Environments.Value,
		Tenants:              flags.Tenants.Value,
		TenantTags:           flags.TenantTags.Value,
		DeployAt:             flags.DeployAt.Value,
		MaxQueueTime:         flags.MaxQueueTime.Value,
		ExcludedSteps:        flags.ExcludedSteps.Value,
		GuidedFailureMode:    flags.GuidedFailureMode.Value,
		ForcePackageDownload: flags.ForcePackageDownload.Value,
		DeploymentTargets:    flags.DeploymentTargets.Value,
		ExcludeTargets:       flags.ExcludeTargets.Value,
		Variables:            parsedVariables,
	}

	// special case for FlagForcePackageDownload bool so we can tell if it was set on the cmdline or missing
	if cmd.Flags().Lookup(FlagForcePackageDownload).Changed {
		options.ForcePackageDownloadWasSpecified = true
	}

	if f.IsPromptEnabled() {
		err = AskQuestions(octopus, cmd.OutOrStdout(), f.Ask, f.Spinner(), f.GetCurrentSpace(), options)
		if err != nil {
			return err
		}

		if !constants.IsProgrammaticOutputFormat(outputFormat) {
			// the Q&A process will have modified options;backfill into flags for generation of the automation cmd
			resolvedFlags := NewDeployFlags()
			resolvedFlags.Project.Value = options.ProjectName
			resolvedFlags.ReleaseVersion.Value = options.ReleaseVersion
			resolvedFlags.Environments.Value = options.Environments
			resolvedFlags.Tenants.Value = options.Tenants
			resolvedFlags.TenantTags.Value = options.TenantTags
			resolvedFlags.DeployAt.Value = options.DeployAt
			resolvedFlags.MaxQueueTime.Value = options.MaxQueueTime
			resolvedFlags.ExcludedSteps.Value = options.ExcludedSteps
			resolvedFlags.GuidedFailureMode.Value = options.GuidedFailureMode
			resolvedFlags.DeploymentTargets.Value = options.DeploymentTargets
			resolvedFlags.ExcludeTargets.Value = options.ExcludeTargets
			resolvedFlags.Variables.Value = ToVariableStringArray(options.Variables)

			// we're deliberately adding --no-prompt to the generated cmdline so ForcePackageDownload=false will be missing,
			// but that's fine
			resolvedFlags.ForcePackageDownload.Value = options.ForcePackageDownload

			autoCmd := flag.GenerateAutomationCmd(constants.ExecutableName+" release deploy",
				resolvedFlags.Project,
				resolvedFlags.ReleaseVersion,
				resolvedFlags.Environments,
				resolvedFlags.Tenants,
				resolvedFlags.TenantTags,
				resolvedFlags.DeployAt,
				resolvedFlags.MaxQueueTime,
				resolvedFlags.ExcludedSteps,
				resolvedFlags.GuidedFailureMode,
				resolvedFlags.ForcePackageDownload,
				resolvedFlags.DeploymentTargets,
				resolvedFlags.ExcludeTargets,
				resolvedFlags.Variables,
			)
			cmd.Printf("\nAutomation Command: %s\n", autoCmd)
		}
	}

	// the executor will raise errors if any required options are missing
	err = executor.ProcessTasks(octopus, f.GetCurrentSpace(), []*executor.Task{
		executor.NewTask(executor.TaskTypeDeployRelease, options),
	})
	if err != nil {
		return err
	}

	return nil
}

func AskQuestions(octopus *octopusApiClient.Client, stdout io.Writer, asker question.Asker, spinner factory.Spinner, space *spaces.Space, options *executor.TaskOptionsDeployRelease) error {
	if octopus == nil {
		return cliErrors.NewArgumentNullOrEmptyError("octopus")
	}
	if asker == nil {
		return cliErrors.NewArgumentNullOrEmptyError("asker")
	}
	if options == nil {
		return cliErrors.NewArgumentNullOrEmptyError("options")
	}
	// Note: we don't get here at all if no-prompt is enabled, so we know we are free to ask questions

	// Note on output: survey prints things; if the option is specified already from the command line,
	// we should emulate that so there is always a line where you can see what the item was when specified on the command line,
	// however if we support a "quiet mode" then we shouldn't emit those

	var err error

	// select project
	var selectedProject *projects.Project
	if options.ProjectName == "" {
		selectedProject, err = selectors.Project("Select the project to deploy from", octopus, asker, spinner)
		if err != nil {
			return err
		}
	} else { // project name is already provided, fetch the object because it's needed for further questions
		selectedProject, err = selectors.FindProject(octopus, spinner, options.ProjectName)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "Project %s\n", output.Cyan(selectedProject.Name))
	}
	options.ProjectName = selectedProject.Name

	// select release

	var selectedRelease *releases.Release
	if options.ReleaseVersion == "" {
		// first we want to ask them to pick a channel just to narrow down the search space for releases (not sent to server)
		selectedChannel, err := selectors.Channel(octopus, asker, spinner, "Select the channel to deploy from", selectedProject)
		if err != nil {
			return err
		}
		selectedRelease, err = selectRelease(octopus, asker, spinner, "Select the release to deploy", space, selectedProject, selectedChannel)
		if err != nil {
			return err
		}
	} else {
		selectedRelease, err = findRelease(octopus, spinner, space, selectedProject, options.ReleaseVersion)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "Release %s\n", output.Cyan(selectedRelease.Version))
	}
	options.ReleaseVersion = selectedRelease.Version
	if err != nil {
		return err
	}

	_, err = AskPromptedVariables(octopus, spinner, space, selectedRelease.ProjectVariableSetSnapshotID, options.Variables)
	if err != nil {
		return err
	}

	isTenanted, err := determineIsTenanted(selectedProject, asker)
	if err != nil {
		return err
	}

	// 		If tentanted:
	// 		  select (singular) environment
	// 		    select tenants and/or tags (this is just a way of finding which tenants we are going to deploy to)
	// 		else:
	// 		  select environments

	// TODO do we need to hit the releases/{id}/progression endpoint to filter environments here?

	if isTenanted {
		if len(options.Environments) == 0 {
			env, err := selectors.EnvironmentSelect(asker, octopus, spinner, "Select environment to deploy to")
			if err != nil {
				return err
			}
			options.Environments = []string{env.Name} // executions api allows env names, so let's use these instead so they look nice in generated automationcmd
		}

		// TODO select TagsAndTenants is going to require it's own function
		// 		UX problem: How do we find tenants via their tags?
	} else {
		if len(options.Environments) == 0 {
			envs, err := selectors.EnvironmentsMultiSelect(asker, octopus, spinner, "Select environments to deploy to", 1)
			if err != nil {
				return err
			}
			options.Environments = util.SliceTransform(envs, func(env *environments.Environment) string { return env.Name })
		}
	}

	// when? (timed deployment)

	// select steps to exclude
	deploymentProcess, err := deployments.GetDeploymentProcess(octopus, space.ID, selectedRelease.ProjectDeploymentProcessSnapshotID)
	if err != nil {
		return err
	}

	if len(options.ExcludedSteps) == 0 {
		stepsToExclude, err := question.MultiSelectMap(asker, "Select steps to skip (if any)", deploymentProcess.Steps, func(s *deployments.DeploymentStep) string {
			return s.Name
		}, 0)
		if err != nil {
			return err
		}
		options.ExcludedSteps = util.SliceTransform(stepsToExclude, func(s *deployments.DeploymentStep) string {
			return s.ID // this is a GUID, what should we actually be sending to the server?
		})
	}

	// do we want guided failure mode?
	if options.GuidedFailureMode == "" { // if they deliberately specified false, don't ask them
		modes := []core.GuidedFailureMode{
			"", "true", "false",
		}
		gfm, err := question.SelectMap(asker, "Guided Failure Mode?", modes, func(g core.GuidedFailureMode) string {
			switch g {
			case "":
				return "Use default setting from the target environment"
			case "true":
				return "Use guided failure mode"
			case "false":
				return "Do not use guided failure mode"
			default:
				return fmt.Sprintf("Unhandled %s", g)
			}
		})
		if err != nil {
			return err
		}
		options.GuidedFailureMode = string(gfm)
	}

	// force package re-download?
	if !options.ForcePackageDownloadWasSpecified { // if they deliberately specified false, don't ask them
		forcePackageDownload, err := question.SelectMap(asker, "Force package re-download?", []bool{false, true}, func(b bool) string {
			if b {
				return "Yes" // should be the default; they probably want to not force
			} else {
				return "No"
			}
		})
		if err != nil {
			return err
		}
		options.ForcePackageDownload = forcePackageDownload
	}

	// If tenanted:
	//   foreach tenant:
	//     select deployment target(s)
	// else
	//   select deployment target(s)

	// DONE
	return nil
}

func AskPromptedVariables(octopus *octopusApiClient.Client, spinner factory.Spinner, space *spaces.Space, id string, variableFromCmd map[string]string) (map[string]string, error) {
	//
	//variableSet, err := variables.GetVariableSet(octopus, space.ID, selectedRelease.ProjectVariableSetSnapshotID)
	//if err != nil {
	//	return err
	//}
	//
	//if len(variableSet.Variables) > 0 { // nothing to be done here, move along
	//	for _, v := range variableSet.Variables {
	//		if v.Prompt != nil { // this is a prompted variable, ask for input
	//			asker(&survey.Input{})
	//		}
	//	}
	//}
	return nil, nil
}

//
//type RequestProcessor struct {
//	GetReleasesInProjectChannel func(client newclient.Client, projectID string, channelID string) ([]*releases.Release, error)
//	// 300 more functions for every kind of GetXyz
//}
//
//func NewRealRequestProcessor() *RequestProcessor {
//	return &RequestProcessor{
//		GetReleasesInProjectChannel: releases.GetReleasesInProjectChannel,
//	}
//}
//
//func NewFakeRequestProcessor() *RequestProcessor {
//	return &RequestProcessor{
//		GetReleasesInProjectChannel: func(client newclient.Client, projectID string, channelID string) ([]*releases.Release, error) {
//			// It's a mock!
//			return nil, nil
//		},
//	}
//}

func selectRelease(octopus *octopusApiClient.Client, ask question.Asker, spinner factory.Spinner, questionText string, space *spaces.Space, project *projects.Project, channel *channels.Channel) (*releases.Release, error) {
	spinner.Start()
	foundReleases, err := releases.GetReleasesInProjectChannel(octopus, space.ID, project.ID, channel.ID)
	spinner.Stop()
	if err != nil {
		return nil, err
	}

	return question.SelectMap(ask, questionText, foundReleases, func(p *releases.Release) string {
		return p.Version
	})
}

func findRelease(octopus *octopusApiClient.Client, spinner factory.Spinner, space *spaces.Space, project *projects.Project, releaseVersion string) (*releases.Release, error) {
	spinner.Start()
	foundRelease, err := releases.GetReleaseInProject(octopus, space.ID, project.ID, releaseVersion)
	spinner.Stop()
	return foundRelease, err
}

// determineIsTenanted returns true if we are going to do a tenanted deployment, false if untenanted
// NOTE: Tenant can be disabled or forced. In these cases we know what to do.
// The middle case is "allowed, but not forced", in which case we don't know ahead of time what to do WRT tenants,
// so we'd need to ask the user (presumably though we can check if the project itself is linked to any tenants and only ask then)?
// there is a ListTenants(projectID) api that we can use. /api/tenants?projectID=
func determineIsTenanted(project *projects.Project, ask question.Asker) (bool, error) {
	switch project.TenantedDeploymentMode {
	case core.TenantedDeploymentModeUntenanted:
		return false, nil
	case core.TenantedDeploymentModeTenanted:
		return true, nil
	case core.TenantedDeploymentModeTenantedOrUntenanted:
		return question.SelectMap(ask, "Select Tenanted or Untenanted deployment", []bool{true, false}, func(b bool) string {
			if b {
				return "Tenanted" // should be the default; they probably want tenanted
			} else {
				return "Untenanted"
			}
		})

	default: // should not get here
		return false, fmt.Errorf("unhandled tenanted deployment mode %s", project.TenantedDeploymentMode)
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
		result = append(result, fmt.Sprintf("%s:%s", k, v)) // TODO what about variables that have a : in their name?
	}
	return result
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
