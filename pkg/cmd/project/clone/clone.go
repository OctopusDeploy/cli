package clone

import (
	"fmt"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	projectShared "github.com/OctopusDeploy/cli/pkg/cmd/project/shared"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"
)

const (
	FlagGroup       = "group"
	FlagName        = "name"
	FlagDescription = "description"
	FlagSource      = "source"
	FlagLifecycle   = "lifecycle"
)

type CloneFlags struct {
	Group       *flag.Flag[string]
	Name        *flag.Flag[string]
	Description *flag.Flag[string]
	Source      *flag.Flag[string]
	Lifecycle   *flag.Flag[string]
}

func NewCloneFlags() *CloneFlags {
	return &CloneFlags{
		Group:       flag.New[string](FlagGroup, false),
		Name:        flag.New[string](FlagName, false),
		Description: flag.New[string](FlagDescription, false),
		Source:      flag.New[string](FlagSource, false),
		Lifecycle:   flag.New[string](FlagLifecycle, false),
	}
}

type CloneOptions struct {
	*CloneFlags
	*cmd.Dependencies
	GetAllGroupsCallback       projectShared.GetAllGroupsCallback
	CreateProjectGroupCallback projectShared.CreateProjectGroupCallback
	shared.GetAllProjectsCallback
}

func NewCloneOptions(createFlags *CloneFlags, dependencies *cmd.Dependencies) *CloneOptions {
	return &CloneOptions{
		CloneFlags:                 createFlags,
		Dependencies:               dependencies,
		GetAllGroupsCallback:       func() ([]*projectgroups.ProjectGroup, error) { return projectShared.GetAllGroups(*dependencies.Client) },
		GetAllProjectsCallback:     func() ([]*projects.Project, error) { return shared.GetAllProjects(dependencies.Client) },
		CreateProjectGroupCallback: func() (string, cmd.Dependable, error) { return projectShared.CreateProjectGroup(dependencies) },
	}
}

func NewCmdClone(f factory.Factory) *cobra.Command {
	createFlags := NewCloneFlags()

	cmd := &cobra.Command{
		Use:   "clone",
		Short: "Clone a project",
		Long:  "Clone a project in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project clone
			$ %[1]s project clone --name 'New Project' --source 'Old Project'
			$ %[1]s project clone --name 'Deploy web app 2' --source 'Deploy web app' --lifecycle 'Test Only Lifecycle' --group 'Web App Project Group'
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewCloneOptions(createFlags, cmd.NewDependencies(f, c))

			return cloneRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "Name of the project")
	flags.StringVarP(&createFlags.Description.Value, createFlags.Description.Name, "d", "", "Description of the project")
	flags.StringVarP(&createFlags.Source.Value, createFlags.Source.Name, "", "", "Name of the source project")
	flags.StringVarP(&createFlags.Group.Value, createFlags.Group.Name, "g", "", "Project group of the project")
	flags.StringVarP(&createFlags.Lifecycle.Value, createFlags.Lifecycle.Name, "l", "", "Lifecycle of the project")
	flags.SortFlags = false

	return cmd
}

func cloneRun(opts *CloneOptions) error {
	var optsArray []cmd.Dependable
	var err error
	if !opts.NoPrompt {
		optsArray, err = PromptMissing(opts)
		if err != nil {
			return err
		}
	} else {
		optsArray = append(optsArray, opts)
	}

	for _, o := range optsArray {
		if err := o.Commit(); err != nil {
			return err
		}
	}

	if !opts.NoPrompt {
		fmt.Fprintln(opts.Out, "\nAutomation Commands:")
		for _, o := range optsArray {
			o.GenerateAutomationCmd()
		}
	}

	return nil
}

func PromptMissing(opts *CloneOptions) ([]cmd.Dependable, error) {
	nestedOpts := []cmd.Dependable{}

	question.AskName(opts.Ask, "", "project", &opts.Name.Value)

	if opts.Lifecycle.Value == "" {
		lc, err := selectors.Lifecycle("You have not specified a Lifecycle for this project. Please select one:", opts.Client, opts.Ask)
		if err != nil {
			return nil, err
		}
		opts.Lifecycle.Value = lc.Name
	}

	value, projectGroupOpt, err := projectShared.AskProjectGroups(opts.Ask, opts.Group.Value, opts.GetAllGroupsCallback, opts.CreateProjectGroupCallback)
	if err != nil {
		return nil, err
	}
	opts.Group.Value = value
	if projectGroupOpt != nil {
		nestedOpts = append(nestedOpts, projectGroupOpt)
	}

	nestedOpts = append(nestedOpts, opts)

	return nestedOpts, nil
}

func (co *CloneOptions) Commit() error {
	sourceProject, err := co.Client.Projects.GetByIdentifier(co.Source.Value)
	if err != nil {
		return err
	}

	var lifecycleID string
	if co.Lifecycle.Value != "" {
		lifecycle, err := co.Client.Lifecycles.GetByIDOrName(co.Lifecycle.Value)
		if err != nil {
			return err
		}
		lifecycleID = lifecycle.GetID()
	} else {
		lifecycleID = sourceProject.LifecycleID
	}

	var projectGroupID string
	if co.Group.Value != "" {
		projectGroup, err := co.Client.ProjectGroups.GetByIDOrName(co.Group.Value)
		if err != nil {
			return err
		}
		projectGroupID = projectGroup.GetID()
	} else {
		projectGroupID = sourceProject.ProjectGroupID
	}

	clonedProject, err := co.Client.Projects.Clone(sourceProject, projects.ProjectCloneRequest{Name: co.Name.Value, Description: co.Description.Value, ProjectGroupID: projectGroupID, LifecycleID: lifecycleID})
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(co.Out, "\nSuccessfully cloned project '%s' (%s), with lifecycle '%s' in project group '%s'.\n", clonedProject.Name, clonedProject.Slug, lifecycleID, projectGroupID)
	if err != nil {
		return err
	}

	link := output.Bluef("%s/app#/%s/projects/%s", co.Host, co.Space.GetID(), clonedProject.GetID())
	fmt.Fprintf(co.Out, "View this project on Octopus Deploy: %s\n", link)

	return nil
}

func (co *CloneOptions) GenerateAutomationCmd() {
	if !co.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(co.CmdPath, co.Name, co.Source, co.Group, co.Lifecycle)
		fmt.Fprintf(co.Out, "%s\n", autoCmd)
	}
}

func getAllGroups(client client.Client) ([]*projectgroups.ProjectGroup, error) {
	res, err := client.ProjectGroups.GetAll()
	if err != nil {
		return nil, err
	}
	return res, nil
}
