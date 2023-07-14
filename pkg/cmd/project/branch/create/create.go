package create

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	sharedBranches "github.com/OctopusDeploy/cli/pkg/cmd/project/branch/shared"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"
)

const (
	FlagProject    = "project"
	FlagName       = "name"
	FlagBaseBranch = "base-branch"
)

type CreateFlags struct {
	Project    *flag.Flag[string]
	Name       *flag.Flag[string]
	BaseBranch *flag.Flag[string]
}

type CreateOptions struct {
	*CreateFlags
	*cmd.Dependencies
	shared.GetProjectCallback
	shared.GetAllProjectsCallback
	*sharedBranches.ProjectBranchCallbacks
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Project:    flag.New[string](FlagProject, false),
		Name:       flag.New[string](FlagName, false),
		BaseBranch: flag.New[string](FlagBaseBranch, false),
	}
}

func NewCreateOptions(flags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:  flags,
		Dependencies: dependencies,
		GetProjectCallback: func(identifier string) (*projects.Project, error) {
			return shared.GetProject(dependencies.Client, identifier)
		},
		GetAllProjectsCallback: func() ([]*projects.Project, error) { return shared.GetAllProjects(dependencies.Client) },
		ProjectBranchCallbacks: sharedBranches.NewProjectBranchCallbacks(dependencies),
	}
}

func NewCreateCmd(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a Git branch for a project",
		Long:    "Create a Git branch for a project in Octopus Deploy",
		Aliases: []string{"add"},
		Example: heredoc.Docf(`
			$ %[1]s project branch create
			$ %[1]s project branch create --project "Deploy Website" --name branch-nane --base-branch refs/heads/main
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))

			return CreateRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Project.Value, createFlags.Project.Name, "p", "", "The project")
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "The name of the branch")
	flags.StringVarP(&createFlags.BaseBranch.Value, createFlags.BaseBranch.Name, "", "", "The git-ref branch to create a new branch from")

	return cmd
}

func CreateRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		err := PromptMissing(opts)
		if err != nil {
			return err
		}
	}

	project, err := opts.GetProjectCallback(opts.Project.Value)
	if err != nil {
		return err
	}

	newBranch, err := opts.Client.ProjectBranches.Add(opts.Space.GetID(), project.GetID(), opts.BaseBranch.Value, opts.Name.Value)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.Out, "Successfully created branch '%s' (%s) in project '%s'\n", opts.Name.Value, newBranch.CanonicalName, project.GetName())

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Project, opts.Name, opts.BaseBranch)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	return nil
}

func PromptMissing(opts *CreateOptions) error {
	var project *projects.Project
	var err error
	if opts.Project.Value == "" {
		project, err = projectSelector("You have not specified a Project. Please select one:", opts.GetAllProjectsCallback, opts.Ask)
		if err != nil {
			return nil
		}
		opts.Project.Value = project.GetName()
	} else {
		project, err = opts.GetProjectCallback(opts.Project.Value)
		if err != nil {
			return err
		}
	}

	if opts.Name.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Name",
			Help:    fmt.Sprintf("A name for the new branch."),
		}, &opts.Name.Value, survey.WithValidator(survey.ComposeValidators(
			survey.MaxLength(200),
			survey.MinLength(1),
			survey.Required,
		))); err != nil {
			return err
		}
	}

	if opts.BaseBranch.Value == "" {
		branch, err := branchSelector("You have not specified a base branch. Please select one:", project.GetID(), opts)
		if err != nil {
			return err
		}
		opts.BaseBranch.Value = branch.CanonicalName
	}

	return nil
}

func projectSelector(questionText string, getAllProjectsCallback shared.GetAllProjectsCallback, ask question.Asker) (*projects.Project, error) {
	existingProjects, err := getAllProjectsCallback()
	if err != nil {
		return nil, err
	}

	versionControlledProjects := util.SliceFilter(existingProjects, func(p *projects.Project) bool { return p.IsVersionControlled })
	return question.SelectMap(ask, questionText, versionControlledProjects, func(p *projects.Project) string { return p.GetName() })
}

func branchSelector(questionText string, projectId string, opts *CreateOptions) (*projects.GitReference, error) {
	branches, err := opts.GetAllBranchesCallback(projectId)
	if err != nil {
		return nil, err
	}

	return question.SelectMap(opts.Ask, questionText, branches, func(b *projects.GitReference) string { return b.CanonicalName })
}
