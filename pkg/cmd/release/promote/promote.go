package promote

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/release/deploy"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/executionscommon"
	"github.com/OctopusDeploy/cli/pkg/executor"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
)

const (
	FlagProject       = "project"
	FlagFrom          = "from"
	FlagTo            = "to"
	FlagAliasDeployTo = "deployTo"

	FlagUpdateVariables            = "update-variables"
	FlagAliasLegacyUpdateVariables = "updateVariables"

	FlagLatestSuccessful            = "latest-successful"
	FlagAliasLegacyLatestSuccessful = "latestSuccessful"
)

type PromoteFlags struct {
	deploy.CommonDeployFlags
	Project          *flag.Flag[string]
	From             *flag.Flag[string]
	To               *flag.Flag[string]
	LatestSuccessful *flag.Flag[bool]
}

func NewPromoteFlags() *PromoteFlags {
	return &PromoteFlags{
		Project:          flag.New[string](FlagProject, false),
		From:             flag.New[string](FlagFrom, false),
		To:               flag.New[string](FlagTo, false),
		LatestSuccessful: flag.New[bool](FlagLatestSuccessful, false),
	}
}

func NewCmdPromote(f factory.Factory) *cobra.Command {
	promoteFlags := NewPromoteFlags()

	cmd := &cobra.Command{
		Use:   "promote",
		Short: "Promote a release",
		Long:  "Promote a release to a specific environment",
		Example: heredoc.Docf(`
			%[1]s release promote --project MyProject --from "Development" --to "Staging"\
			%[1]s release promote -p MyProject --from "Staging" --to "Production" --update-variables --latest-successful\`,
			constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error { return createRun(cmd, f, promoteFlags) },
	}

	flags := cmd.Flags()
	flags.StringVarP(&promoteFlags.Project.Value, promoteFlags.Project.Name, "p", "", "Name or ID of the project")
	flags.StringVarP(&promoteFlags.From.Value, promoteFlags.From.Name, "", "", "Name or ID of the environment to get the current deployment from")
	flags.StringVarP(&promoteFlags.To.Value, promoteFlags.To.Name, "", "", "Name or ID of the environment to deploy to")
	flags.BoolVarP(&promoteFlags.LatestSuccessful.Value, promoteFlags.LatestSuccessful.Name, "", false, "Use the latest successful release to promote")

	// aliases for compatability with the old .NET CLI
	flagAliases := make(map[string][]string, 10)
	util.AddFlagAliasesString(flags, FlagTo, flagAliases, FlagAliasDeployTo)
	util.AddFlagAliasesBool(flags, FlagLatestSuccessful, flagAliases, FlagAliasLegacyLatestSuccessful)

	deploy.AddCommonFlagsAndAliases(flags, &promoteFlags.CommonDeployFlags, flagAliases)

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		util.ApplyFlagAliases(cmd.Flags(), flagAliases)
		return nil
	}

	return cmd
}

func createRun(cmd *cobra.Command, f factory.Factory, flags *PromoteFlags) error {
	outputFormat, err := cmd.Flags().GetString(constants.FlagOutputFormat)
	if err != nil { // should never happen, but fallback if it does
		outputFormat = constants.OutputFormatTable
	}

	octopus, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	parsedVariables, err := executionscommon.ParseVariableStringArray(flags.Variables.Value)
	if err != nil {
		return err
	}

	options := &executor.TaskOptionsPromoteRelease{
		ProjectName:      flags.Project.Value,
		FromEnvironment:  flags.From.Value,
		ToEnvironment:    flags.To.Value,
		LatestSuccessful: flags.LatestSuccessful.Value,
		BaseTaskOptionsDeployRelease: executor.BaseTaskOptionsDeployRelease{
			Tenants:                        flags.CommonDeployFlags.Tenants.Value,
			TenantTags:                     flags.CommonDeployFlags.TenantTags.Value,
			ScheduledStartTime:             flags.DeployAt.Value,
			ScheduledExpiryTime:            flags.CommonDeployFlags.MaxQueueTime.Value,
			ExcludedSteps:                  flags.CommonDeployFlags.ExcludedSteps.Value,
			GuidedFailureMode:              flags.CommonDeployFlags.GuidedFailureMode.Value,
			ForcePackageDownload:           flags.CommonDeployFlags.ForcePackageDownload.Value,
			DeploymentTargets:              flags.CommonDeployFlags.DeploymentTargets.Value,
			ExcludeTargets:                 flags.CommonDeployFlags.ExcludeTargets.Value,
			DeploymentFreezeNames:          flags.CommonDeployFlags.DeploymentFreezeNames.Value,
			DeploymentFreezeOverrideReason: flags.CommonDeployFlags.DeploymentFreezeOverrideReason.Value,
			Variables:                      parsedVariables,
			UpdateVariables:                flags.CommonDeployFlags.UpdateVariables.Value,
		},
	}

	err = executor.ProcessTasks(octopus, f.GetCurrentSpace(), []*executor.Task{
		executor.NewTask(executor.TaskTypePromoteRelease, options),
	})
	if err != nil {
		return err
	}

	deploy.OutputCreateDeploymentResponseV1(cmd, f, options.Response, options.ReleaseID, outputFormat)

	return nil
}
