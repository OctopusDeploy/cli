package delete

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/buildinformation"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/spf13/cobra"
)

const (
	FlagPackageId = "package-id"
	FlagVersion   = "version"
)

type DeleteFlags struct {
	PackageId *flag.Flag[string]
	Version   *flag.Flag[string]
}

func NewDeleteFlags() *DeleteFlags {
	return &DeleteFlags{
		PackageId: flag.New[string](FlagPackageId, false),
		Version:   flag.New[string](FlagVersion, false),
	}
}

type DeleteOptions struct {
	*cmd.Dependencies
	*DeleteFlags
	*question.ConfirmFlags
	ID string
}

func NewDeleteOptions(id string, deleteFlags *DeleteFlags, confirmFlags *question.ConfirmFlags, dependencies *cmd.Dependencies) *DeleteOptions {
	return &DeleteOptions{
		Dependencies: dependencies,
		DeleteFlags:  deleteFlags,
		ConfirmFlags: confirmFlags,
		ID:           id,
	}
}

func NewCmdDelete(f factory.Factory) *cobra.Command {
	deleteFlags := NewDeleteFlags()
	confirmFlags := question.NewConfirmFlags()
	cmd := &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete a build information",
		Long:    "Delete a build information in Octopus Deploy",
		Aliases: []string{"del", "rm", "remove"},
		Example: heredoc.Docf(`
			$ %[1]s build-information delete BuildInformation-1
			$ %[1]s build-info rm BuildInformation-1
			$ %[1]s build-info del --package-id ThePackage --version 1.2.3
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = append(args, "")
			}

			opts := NewDeleteOptions(args[0], deleteFlags, confirmFlags, cmd.NewDependencies(f, c))

			return deleteRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&deleteFlags.PackageId.Value, deleteFlags.PackageId.Name, "p", "", "The Package ID of the build information to delete")
	flags.StringVarP(&deleteFlags.Version.Value, deleteFlags.Version.Name, "v", "", "The version of the build information to delete")
	question.RegisterConfirmDeletionFlag(cmd, &confirmFlags.Confirm.Value, "build information")

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	if opts.DeleteFlags.PackageId.Value != "" {
		if opts.DeleteFlags.Version.Value == "" {
			return fmt.Errorf("--version must be specified when --package-id is used")
		}

		buildInfo, err := buildinformation.Get(
			opts.Client,
			opts.Client.GetSpaceID(),
			buildinformation.BuildInformationQuery{
				PackageID: opts.DeleteFlags.PackageId.Value,
				Filter:    opts.DeleteFlags.Version.Value,
			})
		if err != nil {
			return err
		}

		if len(buildInfo.Items) == 1 {
			opts.ID = buildInfo.Items[0].GetID()
		}
	}

	if opts.ID == "" {
		return fmt.Errorf("build information identifier is required but was not provided")
	}

	itemToDelete, err := buildinformation.GetById(opts.Client, opts.Client.GetSpaceID(), opts.ID)
	if err != nil {
		return err
	}

	if opts.ConfirmFlags.Confirm.Value {
		return delete(opts.Client, itemToDelete)
	} else {
		return question.DeleteWithConfirmation(opts.Ask, "build information", itemToDelete.PackageID+" "+itemToDelete.Version, itemToDelete.ID, func() error {
			return delete(opts.Client, itemToDelete)
		})
	}
}

func delete(client *client.Client, itemToDelete *buildinformation.BuildInformation) error {
	return buildinformation.DeleteByID(client, client.GetSpaceID(), itemToDelete.GetID())
}
