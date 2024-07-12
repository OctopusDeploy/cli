package delete

import (
	"fmt"
	"slices"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/packages"
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
		Use:     "delete {<id>}",
		Short:   "Delete a package",
		Long:    "Delete a package in Octopus Deploy",
		Aliases: []string{"del", "rm", "remove"},
		Example: heredoc.Docf(`
			$ %[1]s package delete Packages-1
			$ %[1]s package rm Packages-1
			$ %[1]s package del --package-id ThePackage --version 1.0.0
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
	flags.StringVarP(&deleteFlags.PackageId.Value, deleteFlags.PackageId.Name, "p", "", "The Package ID of the package to delete")
	flags.StringVarP(&deleteFlags.Version.Value, deleteFlags.Version.Name, "v", "", "The version of the package to delete")
	question.RegisterConfirmDeletionFlag(cmd, &confirmFlags.Confirm.Value, "package")

	return cmd
}

func deleteRun(opts *DeleteOptions) error {
	if !opts.NoPrompt {
		if err := PromptMissing(opts); err != nil {
			return err
		}
	}

	if opts.ID == "" {
		return fmt.Errorf("package identifier is required but was not provided")
	}

	packageToDelete, err := packages.GetByID(opts.Client, opts.Client.GetSpaceID(), opts.ID)
	if err != nil {
		return err
	}

	if opts.ConfirmFlags.Confirm.Value {
		return delete(opts.Client, packageToDelete)
	} else {
		return question.DeleteWithConfirmation(opts.Ask, "package", packageToDelete.PackageID+" "+packageToDelete.Version, packageToDelete.GetID(), func() error {
			return delete(opts.Client, packageToDelete)
		})
	}
}

func PromptMissing(opts *DeleteOptions) error {
	if opts.ID == "" {
		packageToDelete, err := selectPackage(opts)
		if err != nil {
			return err
		}

		packageVersionToDelete, err := selectVersion(opts, packageToDelete.PackageID)
		if err != nil {
			return err
		}

		opts.ID = packageVersionToDelete.GetID()
	}

	return nil
}

func selectPackage(opts *DeleteOptions) (*packages.Package, error) {
	allExistingPackages, err := packages.GetAll(opts.Client, opts.Client.GetSpaceID())
	if err != nil {
		return nil, err
	}

	if opts.DeleteFlags.PackageId.Value != "" {
		idx := slices.IndexFunc(allExistingPackages, func(p *packages.Package) bool { return p.PackageID == opts.DeleteFlags.PackageId.Value })
		if idx == -1 {
			return nil, fmt.Errorf("unable to find a package matching the specifed ID: '%s'", opts.DeleteFlags.PackageId.Value)
		}
		return allExistingPackages[idx], nil
	} else {
		return question.SelectMap(opts.Ask, "Select the package you wish to delete:", allExistingPackages, func(item *packages.Package) string {
			return item.PackageID
		})
	}
}

func selectVersion(opts *DeleteOptions, packageID string) (*packages.Package, error) {
	packageVersions, err := packages.Get(opts.Client, opts.Client.GetSpaceID(), packages.PackagesQuery{
		NuGetPackageID: packageID,
	})
	if err != nil {
		return nil, err
	}
	allPackageVersions, err := packageVersions.GetAllPages(opts.Client.Sling())
	if err != nil {
		return nil, err
	}

	var packageVersionToDelete *packages.Package
	if opts.DeleteFlags.Version.Value != "" {
		idx := slices.IndexFunc(allPackageVersions, func(p *packages.Package) bool { return p.Version == opts.DeleteFlags.Version.Value })
		if idx == -1 {
			return nil, fmt.Errorf("unable to find a version matching the specified version: '%s", opts.DeleteFlags.Version.Value)
		}
		packageVersionToDelete = allPackageVersions[idx]
	} else {
		packageVersionToDelete, err = question.SelectMap(opts.Ask, "Select the version you wish to delete:", allPackageVersions, func(item *packages.Package) string { return item.Version })
		if err != nil {
			return nil, err
		}
	}

	return packageVersionToDelete, nil
}

func delete(client *client.Client, packageToDelete *packages.Package) error {
	return packages.DeleteByID(client, client.GetSpaceID(), packageToDelete.GetID())
}
