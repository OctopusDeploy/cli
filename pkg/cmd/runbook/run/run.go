package run

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
)

const (
	FlagProject = "project"

	FlagRunbookName        = "name"
	FlagAliasRunbookLegacy = "runbook"

	FlagSnapshot = "snapshot"

	FlagEnvironment         = "environment" // can be specified multiple times; but only once if tenanted
	FlagAliasEnv            = "env"
	FlagAliasDeployToLegacy = "deployTo" // alias for environment

	FlagTenant = "tenant" // can be specified multiple times

	FlagTenantTag            = "tenant-tag" // can be specified multiple times
	FlagAliasTag             = "tag"
	FlagAliasTenantTagLegacy = "tenantTag"

	FlagDeployAt            = "deploy-at" // if this is less than 1 min in the future, go now
	FlagAliasWhen           = "when"      // alias for deploy-at
	FlagAliasDeployAtLegacy = "deployAt"

	FlagDeployAtExpiry           = "deploy-at-expiry"
	FlagDeployAtExpire           = "deploy-at-expire"
	FlagAliasNoDeployAfterLegacy = "noDeployAfter"

	FlagSkip = "skip" // can be specified multiple times

	FlagGuidedFailure                = "guided-failure"
	FlagAliasGuidedFailureMode       = "guided-failure-mode"
	FlagAliasGuidedFailureModeLegacy = "guidedFailure"

	FlagForcePackageDownload            = "force-package-download"
	FlagAliasForcePackageDownloadLegacy = "forcePackageDownload"

	FlagDeploymentTarget      = "deployment-target"
	FlagAliasTarget           = "target"           // alias for deployment-target
	FlagAliasSpecificMachines = "specificMachines" // octo wants a comma separated list. We prefer specifying --target multiple times, but CSV also works because pflag does it for free

	FlagExcludeDeploymentTarget = "exclude-deployment-target"
	FlagAliasExcludeTarget      = "exclude-target"
	FlagAliasExcludeMachines    = "excludeMachines" // octo wants a comma separated list. We prefer specifying --exclude-target multiple times, but CSV also works because pflag does it for free

	FlagVariable = "variable"
)

// executions API stops here.

// DEPLOYMENT TRACKING (Server Tasks): - this might be a separate `octopus task follow ID1, ID2, ID3`
// DESIGN CHOICE: We are not going to show servertask progress in the CLI.

type RunFlags struct {
	Project              *flag.Flag[string]
	RunbookName          *flag.Flag[string] // the runbook to run
	Environments         *flag.Flag[[]string]
	Tenants              *flag.Flag[[]string]
	TenantTags           *flag.Flag[[]string]
	RunAt                *flag.Flag[string]
	MaxQueueTime         *flag.Flag[string]
	Variables            *flag.Flag[[]string]
	Snapshot             *flag.Flag[string]
	ExcludedSteps        *flag.Flag[[]string]
	GuidedFailureMode    *flag.Flag[string] // tri-state: true, false, or "use default". Can we model it with an optional bool?
	ForcePackageDownload *flag.Flag[bool]
	DeploymentTargets    *flag.Flag[[]string]
	ExcludeTargets       *flag.Flag[[]string]
}

func NewRunFlags() *RunFlags {
	return &RunFlags{
		Project:              flag.New[string](FlagProject, false),
		RunbookName:          flag.New[string](FlagRunbookName, false),
		Environments:         flag.New[[]string](FlagEnvironment, false),
		Tenants:              flag.New[[]string](FlagTenant, false),
		TenantTags:           flag.New[[]string](FlagTenantTag, false),
		MaxQueueTime:         flag.New[string](FlagDeployAtExpiry, false),
		RunAt:                flag.New[string](FlagDeployAt, false),
		Variables:            flag.New[[]string](FlagVariable, false),
		Snapshot:             flag.New[string](FlagSnapshot, false),
		ExcludedSteps:        flag.New[[]string](FlagSkip, false),
		GuidedFailureMode:    flag.New[string](FlagGuidedFailure, false),
		ForcePackageDownload: flag.New[bool](FlagForcePackageDownload, false),
		DeploymentTargets:    flag.New[[]string](FlagDeploymentTarget, false),
		ExcludeTargets:       flag.New[[]string](FlagExcludeDeploymentTarget, false),
	}
}

func NewCmdRun(f factory.Factory) *cobra.Command {
	runFlags := NewRunFlags()
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run runbooks in Octopus Deploy",
		Long:  "Run runbooks in Octopus Deploy.",
		Example: heredoc.Doc(`
			$ octopus runbook run  # fully interactive
			$ octopus runbook run --project MyProject ... TODO
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && runFlags.Project.Value == "" {
				runFlags.Project.Value = args[0]
			}

			return runbookRun(cmd, f, runFlags)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&runFlags.Project.Value, runFlags.Project.Name, "p", "", "Name or ID of the project to run the runbook from")
	flags.StringVarP(&runFlags.RunbookName.Value, runFlags.RunbookName.Name, "", "", "Name of the runbook to run")
	flags.StringSliceVarP(&runFlags.Environments.Value, runFlags.Environments.Name, "e", nil, "Run in this environment (can be specified multiple times)")
	flags.StringSliceVarP(&runFlags.Tenants.Value, runFlags.Tenants.Name, "", nil, "Run for this tenant (can be specified multiple times)")
	flags.StringSliceVarP(&runFlags.TenantTags.Value, runFlags.TenantTags.Name, "", nil, "Run for tenants matching this tag (can be specified multiple times)")
	flags.StringVarP(&runFlags.RunAt.Value, runFlags.RunAt.Name, "", "", "Run at a later time. Run now if omitted. TODO date formats and timezones!")
	flags.StringVarP(&runFlags.MaxQueueTime.Value, runFlags.MaxQueueTime.Name, "", "", "Cancel a scheduled run if it hasn't started within this time period.")
	flags.StringSliceVarP(&runFlags.Variables.Value, runFlags.Variables.Name, "v", nil, "Set the value for a prompted variable in the format Label:Value")
	flags.StringVarP(&runFlags.Snapshot.Value, runFlags.Snapshot.Name, "", "", "Name or ID of the snapshot to run. If not supplied, the command will attempt to use the published snapshot.")
	flags.StringSliceVarP(&runFlags.ExcludedSteps.Value, runFlags.ExcludedSteps.Name, "", nil, "Exclude specific steps from the runbook")
	flags.StringVarP(&runFlags.GuidedFailureMode.Value, runFlags.GuidedFailureMode.Name, "", "", "Enable Guided failure mode (true/false/default)")
	flags.BoolVarP(&runFlags.ForcePackageDownload.Value, runFlags.ForcePackageDownload.Name, "", false, "Force re-download of packages")
	flags.StringSliceVarP(&runFlags.DeploymentTargets.Value, runFlags.DeploymentTargets.Name, "", nil, "Run on this target (can be specified multiple times)")
	flags.StringSliceVarP(&runFlags.ExcludeTargets.Value, runFlags.ExcludeTargets.Name, "", nil, "Run on targets except for this (can be specified multiple times)")

	flags.SortFlags = false

	// flags aliases for compat with old .NET CLI
	flagAliases := make(map[string][]string, 10)
	util.AddFlagAliasesString(flags, FlagRunbookName, flagAliases, FlagAliasRunbookLegacy)
	util.AddFlagAliasesStringSlice(flags, FlagEnvironment, flagAliases, FlagAliasDeployToLegacy, FlagAliasEnv)
	util.AddFlagAliasesStringSlice(flags, FlagTenantTag, flagAliases, FlagAliasTag, FlagAliasTenantTagLegacy)
	util.AddFlagAliasesString(flags, FlagDeployAt, flagAliases, FlagAliasWhen, FlagAliasDeployAtLegacy)
	util.AddFlagAliasesString(flags, FlagDeployAtExpiry, flagAliases, FlagDeployAtExpire, FlagAliasNoDeployAfterLegacy)
	util.AddFlagAliasesString(flags, FlagGuidedFailure, flagAliases, FlagAliasGuidedFailureMode, FlagAliasGuidedFailureModeLegacy)
	util.AddFlagAliasesBool(flags, FlagForcePackageDownload, flagAliases, FlagAliasForcePackageDownloadLegacy)
	util.AddFlagAliasesStringSlice(flags, FlagDeploymentTarget, flagAliases, FlagAliasTarget, FlagAliasSpecificMachines)
	util.AddFlagAliasesStringSlice(flags, FlagExcludeDeploymentTarget, flagAliases, FlagAliasExcludeTarget, FlagAliasExcludeMachines)

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		util.ApplyFlagAliases(cmd.Flags(), flagAliases)
		return nil
	}
	return cmd
}

func runbookRun(cmd *cobra.Command, f factory.Factory, flags *RunFlags) error {

	return nil
}
