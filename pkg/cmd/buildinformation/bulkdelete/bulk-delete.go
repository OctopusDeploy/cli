package bulkdelete

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/buildinformation"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/spf13/cobra"
)

type DeleteOptions struct {
	*cmd.Dependencies
	*question.ConfirmFlags
	IDs []string
}

func NewDeleteOptions(ids []string, dependencies *cmd.Dependencies, confirmFlags *question.ConfirmFlags) *DeleteOptions {
	return &DeleteOptions{
		Dependencies: dependencies,
		ConfirmFlags: confirmFlags,
		IDs:          ids,
	}
}

func NewCmdBulkDelete(f factory.Factory) *cobra.Command {
	confirmFlags := question.NewConfirmFlags()
	cmd := &cobra.Command{
		Use:   "bulk-delete",
		Short: "Bulk delete build information",
		Long:  "Bulk delete build information in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s build-information bulk-delete BuildInformation-1 BuildInformation-2
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("build information ids is required but was not provided")
			}

			opts := NewDeleteOptions(args, cmd.NewDependencies(f, c), confirmFlags)

			return deleteRun(opts)
		},
	}

	question.RegisterConfirmDeletionFlag(cmd, &confirmFlags.Confirm.Value, "build information")

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	if opts.ConfirmFlags.Confirm.Value {
		return delete(opts.Client, opts.IDs)
	} else {
		return deleteWithConfirmation(opts.Ask, opts.IDs, func() error {
			return delete(opts.Client, opts.IDs)
		})
	}
}

func delete(client *client.Client, idsToDelete []string) error {
	return buildinformation.DeleteByIDs(client, client.GetSpaceID(), idsToDelete)
}

func deleteWithConfirmation(ask question.Asker, ids []string, doDelete func() error) error {
	var enteredValue string
	if err := ask(&survey.Input{
		Message: fmt.Sprintf(
			`You are about to delete the following build information %s. This action cannot be reversed. To confirm, type 'delete':`,
			strings.Join(ids, ", ")),
	}, &enteredValue); err != nil {
		return err
	}

	if enteredValue != "delete" {
		return fmt.Errorf("input value %s does match expected value 'delete'", enteredValue)
	}

	if err := doDelete(); err != nil {
		return err
	}

	fmt.Printf("%s The %s were deleted successfully.\n", output.Red("✔"), strings.Join(ids, ", "))
	return nil
}
