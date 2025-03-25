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
	"github.com/OctopusDeploy/cli/pkg/util"
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
	PackageID *flag.Flag[string]
	Version   *flag.Flag[[]string]
}

func NewDeleteFlags() *DeleteFlags {
	return &DeleteFlags{
		PackageID: flag.New[string](FlagPackageId, false),
		Version:   flag.New[[]string](FlagVersion, false),
	}
}

type DeleteOptions struct {
	*cmd.Dependencies
	*question.ConfirmFlags
	*DeleteFlags
}

func NewDeleteOptions(deleteFlags *DeleteFlags, dependencies *cmd.Dependencies, confirmFlags *question.ConfirmFlags) *DeleteOptions {
	return &DeleteOptions{
		Dependencies: dependencies,
		ConfirmFlags: confirmFlags,
		DeleteFlags:  deleteFlags,
	}
}

func NewCmdBulkDelete(f factory.Factory) *cobra.Command {
	deleteFlags := NewDeleteFlags()
	confirmFlags := question.NewConfirmFlags()
	cmd := &cobra.Command{
		Use:   "bulk-delete",
		Short: "Bulk delete build information",
		Long:  "Bulk delete build information in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s build-information bulk-delete
			$ %[1]s build-info --package-id ThePackage
			$ %[1]s build-info --package-id ThePackage --version 1.0.0 --version 1.0.1
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewDeleteOptions(deleteFlags, cmd.NewDependencies(f, c), confirmFlags)

			return deleteRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&deleteFlags.PackageID.Value, deleteFlags.PackageID.Name, "", "", "the package id of the build information.")
	flags.StringArrayVarP(&deleteFlags.Version.Value, deleteFlags.Version.Name, "", nil, "the version of the build information, may be specified multiple times.")

	question.RegisterConfirmDeletionFlag(cmd, &confirmFlags.Confirm.Value, "build information")

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	var ids []string

	if !opts.NoPrompt {
		err := PromptMissing(opts)
		if err != nil {
			return err
		}
	}

	selectedBuildInfoVersions, err := selectVersions(opts)
	if err != nil {
		return nil
	}

	ids = util.SliceTransform(selectedBuildInfoVersions, func(item *buildinformation.BuildInformation) string { return item.GetID() })

	if opts.ConfirmFlags.Confirm.Value {
		return delete(opts.Client, ids)
	} else {
		return deleteWithConfirmation(opts.Ask, ids, func() error {
			return delete(opts.Client, ids)
		})
	}
}

func delete(client *client.Client, idsToDelete []string) error {
	return buildinformation.DeleteByIDs(client, client.GetSpaceID(), idsToDelete)
}

func PromptMissing(opts *DeleteOptions) error {
	if opts.PackageID.Value == "" {
		existingBuildInfo, err := buildinformation.Get(opts.Client, opts.Client.GetSpaceID(), buildinformation.BuildInformationQuery{
			Latest: true,
		})
		if err != nil {
			return err
		}

		selectedItem, err := question.SelectMap(opts.Ask, "Select the Package ID you wish to delete:", existingBuildInfo.Items, func(item *buildinformation.BuildInformation) string {
			return item.PackageID
		})
		if err != nil {
			return nil
		}

		opts.PackageID.Value = selectedItem.PackageID
	}

	return nil
}

func selectVersions(opts *DeleteOptions) ([]*buildinformation.BuildInformation, error) {
	if len(opts.Version.Value) > 0 {
		return selectVersionsBasedOnUserInput(opts)
	}

	shouldSelectVersions := false
	var selectVersionsAnswer string
	err := opts.Ask(&survey.Select{
		Message: "Select versions to delete?",
		Help:    "Select 'No' to delete every version of build information for a package id",
		Options: []string{"Yes", "No"},
	}, &selectVersionsAnswer)
	if err != nil {
		return nil, err
	}
	if selectVersionsAnswer == "Yes" {
		shouldSelectVersions = true
	} else {
		shouldSelectVersions = false
	}

	if shouldSelectVersions {
		versionsMap, versions, err := getVersionMap(opts)
		if err != nil {
			return nil, err
		}
		var selectedKeys []string
		err = opts.Ask(&survey.MultiSelect{
			Message: "Select version(s)",
			Options: versions,
		}, &selectedKeys, survey.WithValidator(survey.Required))

		if err != nil {
			return nil, err
		}

		var selectedBuildInformationVersions []*buildinformation.BuildInformation
		for _, v := range selectedKeys {
			if value, ok := versionsMap[v]; ok {
				selectedBuildInformationVersions = append(selectedBuildInformationVersions, value)
			}
		}

		return selectedBuildInformationVersions, nil
	}

	return selectAllVersionsForPackage(opts)
}

func getVersionMap(opts *DeleteOptions) (map[string]*buildinformation.BuildInformation, []string, error) {
	buildInfoForPackageId, err := buildinformation.Get(opts.Client, opts.Client.GetSpaceID(), buildinformation.BuildInformationQuery{
		PackageID: opts.PackageID.Value,
	})
	if err != nil {
		return nil, nil, err
	}

	allBuildInfoForPackageId, err := buildInfoForPackageId.GetAllPages(opts.Client.Sling())
	if err != nil {
		return nil, nil, err
	}

	optionMap, options := question.MakeItemMapAndOptions(allBuildInfoForPackageId, func(item *buildinformation.BuildInformation) string { return item.Version })
	return optionMap, options, nil
}

func selectVersionsBasedOnUserInput(opts *DeleteOptions) ([]*buildinformation.BuildInformation, error) {
	var selectedBuildInformationVersions []*buildinformation.BuildInformation
	versionMap, _, err := getVersionMap(opts)
	if err != nil {
		return nil, err
	}

	for _, v := range opts.Version.Value {
		if value, ok := versionMap[v]; ok {
			selectedBuildInformationVersions = append(selectedBuildInformationVersions, value)
		}
	}
	return selectedBuildInformationVersions, nil
}

func selectAllVersionsForPackage(opts *DeleteOptions) ([]*buildinformation.BuildInformation, error) {
	buildInfoForPackageId, err := buildinformation.Get(opts.Client, opts.Client.GetSpaceID(), buildinformation.BuildInformationQuery{
		PackageID: opts.PackageID.Value,
	})
	if err != nil {
		return nil, err
	}

	allBuildInformationVersions, err := buildInfoForPackageId.GetAllPages(opts.Client.Sling())
	if err != nil {
		return nil, err
	}

	return allBuildInformationVersions, nil
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

	fmt.Printf("%s The %s were deleted successfully.\n", output.Red(""), strings.Join(ids, ", "))
	return nil
}
