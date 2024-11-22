package selectors

import (
	"fmt"

	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
)

func GitReference(questionText string, octopus *octopusApiClient.Client, ask question.Asker, project *projects.Project) (*projects.GitReference, error) {
	branches, err := octopus.Projects.GetGitBranches(project)
	if err != nil {
		return nil, err
	}

	tags, err := octopus.Projects.GetGitTags(project)

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

	return question.SelectMap(ask, questionText, allRefs, func(g *projects.GitReference) string {
		return fmt.Sprintf("%s %s", g.Name, output.Dimf("(%s)", g.Type.Description()))
	})
}
