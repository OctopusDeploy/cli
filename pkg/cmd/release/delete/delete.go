package delete

import (
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/releases"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
)

const (
	FlagProject = "project"
	FlagVersion = "version"
)

type Flags struct {
	Project *flag.Flag[string]
	Version *flag.Flag[[]string]
}

func NewFlags() *Flags {
	return &Flags{
		Project: flag.New[string](FlagProject, false),
		Version: flag.New[[]string](FlagVersion, false),
	}
}

func NewCmdDelete(f factory.Factory) *cobra.Command {
	cmdFlags := NewFlags()
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a release in Octopus Deploy",
		Long:  "Delete a release in Octopus Deploy.",
		Example: heredoc.Doc(`
			$ octopus release delete myProject 2.0
			$ octopus release delete --project myProject --version 2.0
			$ octopus release rm "Other Project" -v 2.0
		`),
		Aliases: []string{"del", "rm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return deleteRun(cmd, f, cmdFlags, args)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&cmdFlags.Project.Value, cmdFlags.Project.Name, "p", "", "Name or ID of the project to delete releases in")
	flags.StringSliceVarP(&cmdFlags.Version.Value, cmdFlags.Version.Name, "v", make([]string, 0), "Release version to delete, can be specified multiple times")
	return cmd
}

func deleteRun(cmd *cobra.Command, f factory.Factory, flags *Flags, args []string) error {
	// command line arg interpretation depends on which flags are present.
	// e.g. `release delete -p MyProject -v 2.0` means we don't need to look at args at
	// e.g. `release delete -p MyProject 2.0` means args[0] is the version
	// e.g. `release delete MyProject -v 2.0` means args[0] is the project
	// e.g. `release delete MyProject 2.0` means args[0] is the project and args[1] is the version
	projectNameOrID := flags.Project.Value
	versionsToDelete := flags.Version.Value

	possibleVersionArgs := args
	if projectNameOrID == "" && len(args) > 0 {
		projectNameOrID = args[0]
		possibleVersionArgs = args[1:] // we've consumed this arg, take it out of consideration for a version
	}
	for _, a := range possibleVersionArgs {
		versionsToDelete = append(versionsToDelete, a)
	}

	// now off we go
	octopus, err := f.GetSpacedClient()
	if err != nil {
		return err
	}
	spinner := f.Spinner()

	var selectedProject *projects.Project
	var releasesToDelete []*releases.Release

	if f.IsPromptEnabled() { // this would be AskQuestions if it were bigger
		if projectNameOrID == "" {
			selectedProject, err = selectors.Project("Select the project to delete a release in", octopus, f.Ask, spinner)
			if err != nil {
				return err
			}
		} else { // project name is already provided, fetch the object because it's needed for further questions
			selectedProject, err = selectors.FindProject(octopus, spinner, projectNameOrID)
			if err != nil {
				return err
			}
			cmd.Printf("Project %s\n", output.Cyan(selectedProject.Name))
		}

		if len(versionsToDelete) == 0 {
			releasesToDelete, err = selectReleases(octopus, selectedProject, f.Ask, spinner)
			if err != nil {
				return err
			}
		} else {
			releasesToDelete, err = findReleases(octopus, spinner, selectedProject, versionsToDelete)
			if err != nil {
				return err
			}
		}

		if len(releasesToDelete) == 0 {
			return nil // no work to do, just exit
		}

		// prompt for confirmation
		cmd.Printf("You are about to delete the following releases:\n")
		for _, r := range releasesToDelete {
			cmd.Printf("%s\n", r.Version)
		}

		var isConfirmed bool
		if err = f.Ask(&survey.Confirm{
			Message: fmt.Sprintf("Confirm delete of %d release(s)", len(releasesToDelete)),
			Default: false,
		}, &isConfirmed); err != nil {
			return err
		}
		if !isConfirmed {
			return nil // nothing to be done here
		}

	} else { // we don't have the executions API backing us and allowing NameOrID; we need to do the lookups ourselves
		// validation
		if projectNameOrID == "" {
			return errors.New("project must be specified")
		}
		if len(versionsToDelete) == 0 {
			return errors.New("at least one release version must be specified")
		}

		selectedProject, err = selectors.FindProject(octopus, factory.NoSpinner, projectNameOrID)
		if err != nil {
			return err
		}
		releasesToDelete, err = findReleases(octopus, factory.NoSpinner, selectedProject, versionsToDelete)
		if err != nil {
			return err
		}
	}

	if len(releasesToDelete) == 0 {
		// no work to do, just exit
		return nil
	}

	spinner.Start()
	var releaseDeleteErrors = &multierror.Error{}
	for _, r := range releasesToDelete {
		err = octopus.Releases.DeleteByID(r.ID)
		if err != nil {
			wrappedErr := fmt.Errorf("failed to delete release %s: %s", r.Version, err)
			cmd.PrintErr(fmt.Sprintf("%s\n", wrappedErr.Error()))
			releaseDeleteErrors = multierror.Append(releaseDeleteErrors, wrappedErr)
		}
	}
	spinner.Stop()

	failedCount := releaseDeleteErrors.Len()
	actuallyDeletedCount := len(releasesToDelete) - failedCount

	if failedCount == 0 { // all good
		cmd.Printf("Successfully deleted %d releases\n", actuallyDeletedCount)
	} else if actuallyDeletedCount == 0 { // all bad
		cmd.Printf("Failed to delete %d releases\n", failedCount)
	} else { // partial
		cmd.Printf("Deleted %d releases. %d releases failed\n", actuallyDeletedCount, failedCount)
	}
	return releaseDeleteErrors.ErrorOrNil()
}

func selectReleases(octopus *octopusApiClient.Client, project *projects.Project, ask question.Asker, spinner factory.Spinner) ([]*releases.Release, error) {
	spinner.Start()
	existingReleases, err := octopus.Projects.GetReleases(project) // gets all of them, no paging
	spinner.Stop()
	if err != nil {
		return nil, err
	}

	return question.MultiSelectMap(ask, "Select Releases to delete", existingReleases, func(p *releases.Release) string {
		return p.Version
	})
}

func findReleases(octopus *octopusApiClient.Client, spinner factory.Spinner, project *projects.Project, versionStrings []string) ([]*releases.Release, error) {
	spinner.Start()
	existingReleases, err := octopus.Projects.GetReleases(project) // gets all of them, no paging
	spinner.Stop()
	if err != nil {
		return nil, err
	}

	versionStringLookup := make(map[string]bool, len(versionStrings))
	for _, s := range versionStrings {
		versionStringLookup[s] = true
	}

	return util.SliceFilter(existingReleases, func(p *releases.Release) bool {
		_, exists := versionStringLookup[p.Version]
		return exists
	}), nil
}
