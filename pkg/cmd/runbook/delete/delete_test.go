package delete_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/runbook/delete"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/stretchr/testify/assert"
)

func TestDeleteDbRunbook_AllFlagsProvided_DeletesRunbook(t *testing.T) {
	pa := []*testutil.PA{}

	const spaceID = "Spaces-1"
	const projectID = "Projects-1"
	const runbookID = "Runbooks-1"

	project := fixtures.NewProject(spaceID, projectID, "Test", "Lifecycles-1", "ProjectGroups-1", "DeploymentProcesses-1")
	runbook := fixtures.NewRunbook(spaceID, projectID, runbookID, "Test")

	asker, _ := testutil.NewMockAsker(t, pa)
	flags := delete.NewDeleteFlags()
	flags.Project.Value = project.Name
	flags.Runbook.Value = runbook.Name
	flags.SkipConfirmation = true

	opts := delete.NewDeleteOptions(&cmd.Dependencies{Ask: asker}, flags)
	opts.GetProjectCallback = func(projectIdentifier string) (*projects.Project, error) {
		if projectIdentifier == project.Name {
			return project, nil
		}

		return nil, errors.New("Project not found")
	}

	opts.GetDbRunbookCallback = func(projectID string, runbookID string) (*runbooks.Runbook, error) {
		if projectID == project.ID && runbookID == runbook.Name {
			return runbook, nil
		}

		return nil, errors.New("Runbook not found")
	}

	opts.DeleteDbRunbookCallback = func(runbookToDelete *runbooks.Runbook) error {
		if runbookToDelete.ID != runbook.ID {
			return errors.New("Runbook not found")
		}

		return nil
	}

	err := delete.DeleteRun(opts)
	assert.NoError(t, err)
}

func TestDeleteDbRunbook_NoFlagsProvided_AsksForRequiredFlags_DeletesRunbook(t *testing.T) {

	const spaceID = "Spaces-1"

	project1 := fixtures.NewProject(spaceID, "Projects-1", "Test1", "Lifecycles-1", "ProjectGroups-1", "DeploymentProcesses-1")
	project2 := fixtures.NewProject(spaceID, "Projects-2", "Test2", "Lifecycles-1", "ProjectGroups-1", "DeploymentProcesses-2")
	runbook1 := fixtures.NewRunbook(spaceID, project1.ID, "Runbooks-1", "Test1")
	runbook2 := fixtures.NewRunbook(spaceID, project1.ID, "Runbooks-2", "Test2")

	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select the project containing the runbook you wish to delete:", "", []string{project1.Name, project2.Name}, project1.Name),
		testutil.NewSelectPrompt("Select the runbook you wish to delete:", "", []string{runbook1.Name, runbook2.Name}, runbook1.Name),
		testutil.NewInputPrompt(fmt.Sprintf(`You are about to delete the runbook "%s" (%s). This action cannot be reversed. To confirm, type the runbook name:`, runbook1.Name, runbook1.ID), "", runbook1.Name),
	}

	asker, _ := testutil.NewMockAsker(t, pa)
	flags := delete.NewDeleteFlags()

	opts := delete.NewDeleteOptions(&cmd.Dependencies{Ask: asker}, flags)
	opts.GetAllProjectsCallback = func() ([]*projects.Project, error) {
		return []*projects.Project{project1, project2}, nil
	}

	opts.GetDbRunbooksCallback = func(projectID string) ([]*runbooks.Runbook, error) {
		if projectID == project1.ID {
			return []*runbooks.Runbook{runbook1, runbook2}, nil
		}

		return nil, errors.New("Runbooks not found")
	}

	opts.DeleteDbRunbookCallback = func(runbookToDelete *runbooks.Runbook) error {
		if runbookToDelete.ID != runbook1.ID {
			return errors.New("Runbook not found")
		}

		return nil
	}

	err := delete.DeleteRun(opts)
	assert.NoError(t, err)
}

func TestDeleteGitRunbook_AllFlagsProvided_DeletesRunbook(t *testing.T) {
	pa := []*testutil.PA{}

	const spaceID = "Spaces-1"
	const projectID = "Projects-1"
	const runbookID = "Runbooks-1"

	project := fixtures.NewVersionControlledProject(spaceID, projectID, "Test", "Lifecycles-1", "ProjectGroups-1", "DeploymentProcesses-1")
	project.PersistenceSettings.(projects.GitPersistenceSettings).SetRunbooksAreInGit()

	runbook := fixtures.NewRunbook(spaceID, projectID, runbookID, "Test")

	asker, _ := testutil.NewMockAsker(t, pa)
	flags := delete.NewDeleteFlags()
	flags.Project.Value = project.Name
	flags.Runbook.Value = runbook.Name
	flags.GitRef.Value = "main"
	flags.SkipConfirmation = true

	opts := delete.NewDeleteOptions(&cmd.Dependencies{Ask: asker}, flags)
	opts.GetProjectCallback = func(projectIdentifier string) (*projects.Project, error) {
		if projectIdentifier == project.Name {
			return project, nil
		}

		return nil, errors.New("Project not found")
	}

	opts.GetGitRunbookCallback = func(projectID string, gitRef string, runbookID string) (*runbooks.Runbook, error) {
		if projectID == project.ID && gitRef == "main" && runbookID == runbook.Name {
			return runbook, nil
		}

		return nil, errors.New("Runbook not found")
	}

	opts.DeleteGitRunbookCallback = func(runbookToDelete *runbooks.Runbook, gitRef string) error {

		if runbookToDelete.ID != runbook.ID || gitRef != "main" {
			return errors.New("Runbook not found")
		}

		return nil
	}

	err := delete.DeleteRun(opts)
	assert.NoError(t, err)
}

func TestDeleteGitRunbook_NoFlagsProvided_AsksForRequiredFlags_DeletesRunbook(t *testing.T) {

	const spaceID = "Spaces-1"

	project1 := fixtures.NewVersionControlledProject(spaceID, "Projects-1", "Test1", "Lifecycles-1", "ProjectGroups-1", "DeploymentProcesses-1")
	project1.PersistenceSettings.(projects.GitPersistenceSettings).SetDefaultBranch("main") // This is the default from above, but let's be explicit
	project1.PersistenceSettings.(projects.GitPersistenceSettings).SetRunbooksAreInGit()
	project2 := fixtures.NewVersionControlledProject(spaceID, "Projects-2", "Test2", "Lifecycles-1", "ProjectGroups-1", "DeploymentProcesses-2")
	project2.PersistenceSettings.(projects.GitPersistenceSettings).SetRunbooksAreInGit()
	project2.PersistenceSettings.(projects.GitPersistenceSettings).SetDefaultBranch("main") // This is the default from above, but let's be explicit

	runbook1 := fixtures.NewRunbook(spaceID, project1.ID, "Runbooks-1", "Test1")
	runbook2 := fixtures.NewRunbook(spaceID, project1.ID, "Runbooks-2", "Test2")

	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select the project containing the runbook you wish to delete:", "", []string{project1.Name, project2.Name}, project1.Name),
		testutil.NewSelectPrompt("Select the Git reference to delete runbook for:", "", []string{"main (Branch)", "develop (Branch)", "v1 (Tag)"}, "main (Branch)"), // The default branch should be first
		testutil.NewSelectPrompt("Select the runbook you wish to delete:", "", []string{runbook1.Name, runbook2.Name}, runbook1.Name),
		testutil.NewInputPrompt(fmt.Sprintf(`You are about to delete the runbook "%s" (%s). This action cannot be reversed. To confirm, type the runbook name:`, runbook1.Name, runbook1.ID), "", runbook1.Name),
	}

	asker, _ := testutil.NewMockAsker(t, pa)
	flags := delete.NewDeleteFlags()

	opts := delete.NewDeleteOptions(&cmd.Dependencies{Ask: asker}, flags)
	opts.GetAllProjectsCallback = func() ([]*projects.Project, error) {
		return []*projects.Project{project1, project2}, nil
	}

	opts.GetGitReferencesCallback = func(project *projects.Project) ([]*projects.GitReference, error) {
		return []*projects.GitReference{
			{
				Type:          projects.GitRefTypeBranch,
				Name:          "develop",
				CanonicalName: "refs/heads/develop",
			},
			{
				Type:          projects.GitRefTypeBranch,
				Name:          "main",
				CanonicalName: "refs/heads/main",
			},
			{
				Type:          projects.GitRefTypeTag,
				Name:          "v1",
				CanonicalName: "refs/tags/v1",
			}}, nil
	}

	opts.GetGitRunbooksCallback = func(projectID string, gitRef string) ([]*runbooks.Runbook, error) {
		if projectID == project1.ID && gitRef == "refs/heads/main" {
			return []*runbooks.Runbook{runbook1, runbook2}, nil
		}

		return nil, errors.New("Runbooks not found")
	}

	opts.DeleteGitRunbookCallback = func(runbookToDelete *runbooks.Runbook, gitRef string) error {
		if runbookToDelete.ID != runbook1.ID || gitRef != "refs/heads/main" {
			return errors.New("Runbook not found")
		}

		return nil
	}

	err := delete.DeleteRun(opts)
	assert.NoError(t, err)
}
