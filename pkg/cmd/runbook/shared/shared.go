package shared

import (
	"math"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/newclient"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
)

type GetDbRunbooksCallback func(projectID string) ([]*runbooks.Runbook, error)
type GetDbRunbookCallback func(projectID string, runbookID string) (*runbooks.Runbook, error)
type GetGitRunbooksCallback func(projectID string, gitRef string) ([]*runbooks.Runbook, error)
type GetGitRunbookCallback func(projectID string, gitRef string, runbookID string) (*runbooks.Runbook, error)
type GetGitReferencesCallback func(project *projects.Project) ([]*projects.GitReference, error)
type DeleteDbRunbookCallback func(runbook *runbooks.Runbook) error
type DeleteGitRunbookCallback func(runbook *runbooks.Runbook, gitRef string) error
type GetProjectCallback func(projectIdentifier string) (*projects.Project, error)
type GetAllProjectsCallback func() ([]*projects.Project, error)

type RunbooksOptions struct {
	GetDbRunbooksCallback
	GetDbRunbookCallback
	GetGitRunbooksCallback
	GetGitRunbookCallback
	GetGitReferencesCallback
	DeleteDbRunbookCallback
	DeleteGitRunbookCallback
	GetProjectCallback
	GetAllProjectsCallback
}

func NewGetRunbooksOptions(dependencies *cmd.Dependencies) *RunbooksOptions {
	return &RunbooksOptions{
		GetDbRunbooksCallback: func(projectID string) ([]*runbooks.Runbook, error) {
			return GetAllRunbooks(dependencies.Client, projectID)
		},
		GetDbRunbookCallback: func(projectID string, runbookIdentifier string) (*runbooks.Runbook, error) {
			return GetRunbook(dependencies.Client, projectID, runbookIdentifier)
		},
		GetGitRunbookCallback: func(projectID string, gitRef string, runbookIdentifier string) (*runbooks.Runbook, error) {
			return GetGitRunbook(dependencies.Client, projectID, gitRef, runbookIdentifier)
		},
		GetGitRunbooksCallback: func(projectID string, gitRef string) ([]*runbooks.Runbook, error) {
			return GetAllGitRunbooks(dependencies.Client, projectID, gitRef)
		},
		GetGitReferencesCallback: func(project *projects.Project) ([]*projects.GitReference, error) {
			return GetAllGitReferences(dependencies.Client, project)
		},
		DeleteDbRunbookCallback: func(runbook *runbooks.Runbook) error {
			return DeleteRunbook(dependencies.Client, runbook)
		},
		DeleteGitRunbookCallback: func(runbook *runbooks.Runbook, gitRef string) error {
			return DeleteGitRunbook(dependencies.Client, runbook, gitRef)
		},
		GetProjectCallback: func(projectIdentifier string) (*projects.Project, error) {
			return GetProject(dependencies.Client, projectIdentifier)
		},
	}
}

func GetAllRunbooks(client newclient.Client, projectID string) ([]*runbooks.Runbook, error) {
	res, err := runbooks.List(client, client.GetSpaceID(), projectID, "", math.MaxInt32)
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

func GetRunbook(client newclient.Client, projectID string, runbookIdentifier string) (*runbooks.Runbook, error) {
	runbook, err := runbooks.GetByID(client, client.GetSpaceID(), runbookIdentifier)
	if err != nil {
		runbook, err = runbooks.GetByName(client, client.GetSpaceID(), projectID, runbookIdentifier)
		if err != nil {
			return nil, err
		}
	}

	return runbook, nil
}

func GetAllGitRunbooks(client newclient.Client, projectID string, gitRef string) ([]*runbooks.Runbook, error) {
	res, err := runbooks.ListGitRunbooks(client, client.GetSpaceID(), projectID, gitRef, "", math.MaxInt32)
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

func GetGitRunbook(client *client.Client, projectID string, gitRef string, runbookIdentifier string) (*runbooks.Runbook, error) {
	runbook, err := runbooks.GetGitRunbookByID(client, client.GetSpaceID(), projectID, gitRef, runbookIdentifier)
	if err != nil {
		runbook, err = runbooks.GetGitRunbookByName(client, client.GetSpaceID(), projectID, gitRef, runbookIdentifier)
		if err != nil {
			return nil, err
		}
	}

	return runbook, nil
}

func GetAllGitReferences(client *client.Client, project *projects.Project) ([]*projects.GitReference, error) {
	branches, err := client.Projects.GetGitBranches(project)
	if err != nil {
		return nil, err
	}

	tags, err := client.Projects.GetGitTags(project)

	if err != nil {
		return nil, err
	}

	allRefs := append(branches, tags...)

	defaultBranch := project.PersistenceSettings.(projects.GitPersistenceSettings).DefaultBranch()

	// if the default branch is in the list, move it to the top
	defaultBranchInRefsList := util.SliceFilter(allRefs, func(g *projects.GitReference) bool { return g.Name == defaultBranch })
	if len(defaultBranchInRefsList) > 0 {
		defaultBranchRef := defaultBranchInRefsList[0]
		allRefs = util.SliceExcept(allRefs, func(g *projects.GitReference) bool { return g.Name == defaultBranch })
		allRefs = append([]*projects.GitReference{defaultBranchRef}, allRefs...)
	}

	return allRefs, nil
}

func DeleteRunbook(client *client.Client, runbook *runbooks.Runbook) error {
	err := runbooks.DeleteByID(client, runbook.SpaceID, runbook.ID)
	if err != nil {
		return err
	}

	return nil
}

func DeleteGitRunbook(client *client.Client, runbook *runbooks.Runbook, gitRef string) error {
	err := runbooks.DeleteGitRunbook(client, runbook.SpaceID, runbook.ProjectID, gitRef, runbook.ID)
	if err != nil {
		return err
	}

	return nil
}

func GetAllProjects(client *client.Client) ([]*projects.Project, error) {
	res, err := client.Projects.GetAll()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func GetProject(octopus *client.Client, projectIdentifier string) (*projects.Project, error) {
	project, err := octopus.Projects.GetByIdentifier(projectIdentifier)
	if err != nil {
		return nil, err
	}

	return project, nil
}

func AreRunbooksInGit(project *projects.Project) bool {
	inGit := false

	if project.PersistenceSettings.Type() == projects.PersistenceSettingsTypeVersionControlled {
		inGit = project.PersistenceSettings.(projects.GitPersistenceSettings).RunbooksAreInGit()
	}

	return inGit
}
