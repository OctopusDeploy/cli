package deploy

import (
	"errors"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"
)

const (
	// TODO progress?
	// TODO waitForDeployment?
	// TODO deploymentTimeout?
	// TODO cancelOnTimeout?
	// TODO deploymentCheckSleepCycle?
	// TODO force?
	// TODO variable(s)?
	// TODO updateVariables?

	FlagProject = "project"
	FlagChannel = "channel"

	FlagReleaseVersion           = "version"
	FlagAliasReleaseNumberLegacy = "releaseNumber"

	FlagEnvironment         = "environment" // can be specified multiple times; but only once if tenanted
	FlagAliasDeployToLegacy = "deployTo"    // TODO should "deployTo" be primary or "environment"?

	FlagTenant               = "tenant"     // can be specified multiple times
	FlagTenantTag            = "tenant-tag" // can be specified multiple times
	FlagAliasTenantTagLegacy = "tenantTag"

	FlagWhen          = "when"
	FlagAliasDeployAt = "deployAt" // TODO should "deployAt" be primary or "when"?

	FlagNoDeployAfter = "noDeployAfter"

	FlagExcludedSteps = "excludeStep" // can be specified multiple times
	FlagAliasSkip     = "skip"        // TODO which should be primary?

	FlagGuidedFailureMode            = "guided-failure"
	FlagAliasGuidedFailureModeLegacy = "guidedFailure"

	FlagForcePackageDownload            = "force-package-download"
	FlagAliasForcePackageDownloadLegacy = "forcePackageDownload"

	FlagDeploymentTargets           = "deploymentTargets"
	FlagAliasSpecificMachinesLegacy = "specificMachines"
	FlagAliasExcludeMachinesLegacy  = "excludeMachines"
)

type DeployFlags struct {
	Project              *flag.Flag[string]
	Channel              *flag.Flag[string]
	ReleaseVersion       *flag.Flag[string]   // the release to deploy
	Environment          *flag.Flag[string]   // singular for tenanted deployment
	Environments         *flag.Flag[[]string] // multiple for untenanted deployment
	Tenants              *flag.Flag[[]string]
	TenantTags           *flag.Flag[[]string]
	When                 *flag.Flag[string]
	ExcludedSteps        *flag.Flag[[]string]
	GuidedFailureMode    *flag.Flag[string] // tri-state: true, false, or "use default". Can we model it with an optional bool?
	ForcePackageDownload *flag.Flag[bool]
	DeploymentTargets    *flag.Flag[[]string]
	// TODO what about deployment targets per tenant? How do you specify that on the cmdline? Look at octo
}

func NewDeployFlags() *DeployFlags {
	return &DeployFlags{
		Project:              flag.New[string](FlagProject, false),
		Channel:              flag.New[string](FlagChannel, false),
		ReleaseVersion:       flag.New[string](FlagReleaseVersion, false),
		Environment:          flag.New[string](FlagEnvironment, false),
		Environments:         flag.New[[]string](FlagEnvironments, false),
		Tenants:              flag.New[[]string](FlagTenants, false),
		TenantTags:           flag.New[[]string](FlagTenantTags, false),
		When:                 flag.New[string](FlagWhen, false),
		ExcludedSteps:        flag.New[[]string](FlagExcludedSteps, false),
		GuidedFailureMode:    flag.New[string](FlagGuidedFailureMode, false),
		ForcePackageDownload: flag.New[bool](FlagForcePackageDownload, false),
		DeploymentTargets:    flag.New[[]string](FlagDeploymentTargets, false),
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
	flags.StringVarP(&deployFlags.Project.Value, deployFlags.Project.Name, "p", "", "Name or ID of the project to deploy the release within")
	flags.StringVarP(&deployFlags.Channel.Value, deployFlags.Channel.Name, "", "", "TODO")
	flags.StringVarP(&deployFlags.ReleaseVersion.Value, deployFlags.ReleaseVersion.Name, "", "", "TODO")
	flags.StringVarP(&deployFlags.Environment.Value, deployFlags.Environment.Name, "", "", "TODO")
	flags.StringSliceVarP(&deployFlags.Environments.Value, deployFlags.Environments.Name, "", nil, "TODO")
	flags.StringSliceVarP(&deployFlags.Tenants.Value, deployFlags.Tenants.Name, "", nil, "TODO")
	flags.StringSliceVarP(&deployFlags.TenantTags.Value, deployFlags.TenantTags.Name, "", nil, "TODO")
	flags.StringVarP(&deployFlags.When.Value, deployFlags.When.Name, "", "", "TODO")
	flags.StringSliceVarP(&deployFlags.ExcludedSteps.Value, deployFlags.ExcludedSteps.Name, "", nil, "TODO")
	flags.StringVarP(&deployFlags.GuidedFailureMode.Value, deployFlags.GuidedFailureMode.Name, "", "", "TODO")
	flags.BoolVarP(&deployFlags.ForcePackageDownload.Value, deployFlags.ForcePackageDownload.Name, "", false, "TODO")
	flags.StringSliceVarP(&deployFlags.DeploymentTargets.Value, deployFlags.DeploymentTargets.Name, "", nil, "TODO")

	flags.SortFlags = false

	// flags aliases for compat with old .NET CLI
	flagAliases := make(map[string][]string, 10)
	util.AddFlagAliasesString(flags, FlagGitRef, flagAliases, FlagAliasGitRefRef, FlagAliasGitRefLegacy)
	util.AddFlagAliasesString(flags, FlagGitCommit, flagAliases, FlagAliasGitCommitLegacy)
	util.AddFlagAliasesString(flags, FlagPackageVersion, flagAliases, FlagAliasDefaultPackageVersion, FlagAliasPackageVersionLegacy, FlagAliasDefaultPackageVersionLegacy)
	util.AddFlagAliasesString(flags, FlagReleaseNotes, flagAliases, FlagAliasReleaseNotesLegacy)
	util.AddFlagAliasesString(flags, FlagReleaseNotesFile, flagAliases, FlagAliasReleaseNotesFileLegacy, FlagAliasReleaseNoteFileLegacy)
	util.AddFlagAliasesString(flags, FlagVersion, flagAliases, FlagAliasReleaseNumberLegacy)
	util.AddFlagAliasesBool(flags, FlagIgnoreExisting, flagAliases, FlagAliasIgnoreExistingLegacy)
	util.AddFlagAliasesBool(flags, FlagIgnoreChannelRules, flagAliases, FlagAliasIgnoreChannelRulesLegacy)

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
	projectNameOrID := flags.Project.Value

	octopus, err := f.GetSpacedClient()
	if err != nil {
		return err
	}
	spinner := f.Spinner()

	var selectedProject *projects.Project
	if f.IsPromptEnabled() { // this would be AskQuestions if it were bigger
		if projectNameOrID == "" {
			selectedProject, err = util.SelectProject("Select the project to list releases for", octopus, f.Ask, spinner)
			if err != nil {
				return err
			}
		} else { // project name is already provided, fetch the object because it's needed for further questions
			selectedProject, err = util.FindProject(octopus, spinner, projectNameOrID)
			if err != nil {
				return err
			}
			cmd.Printf("Project %s\n", output.Cyan(selectedProject.Name))
		}
	} else { // we don't have the executions API backing us and allowing NameOrID; we need to do the lookup ourselves
		if projectNameOrID == "" {
			return errors.New("project must be specified")
		}
		selectedProject, err = util.FindProject(octopus, factory.NoSpinner, projectNameOrID)
		if err != nil {
			return err
		}
	}

	// select channel

	// select release

	// If tentanted:
	//   select (singular) environment
	//   select tenants and/or tags
	// else:
	//   select environments

	// when? (timed deployment)

	// select steps to exclude

	// do we want guided failure mode?

	// force package re-download?

	// If tenanted:
	//   foreach tenant:
	//     select deployment target(s)
	// else
	//   select deployment target(s)

	// DONE

	return nil
}
