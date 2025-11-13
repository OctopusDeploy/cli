package tag

import (
	"fmt"
	"io"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"
)

const (
	FlagTag     = "tag"
	FlagProject = "project"
)

type TagFlags struct {
	Tag     *flag.Flag[[]string]
	Project *flag.Flag[string]
}

func NewTagFlags() *TagFlags {
	return &TagFlags{
		Tag:     flag.New[[]string](FlagTag, false),
		Project: flag.New[string](FlagProject, false),
	}
}

func NewCmdTag(f factory.Factory) *cobra.Command {
	createFlags := NewTagFlags()

	cmd := &cobra.Command{
		Use:     "tag",
		Short:   "Override tags for a project",
		Long:    "Override tags for a project in Octopus Deploy",
		Example: heredoc.Docf("$ %s project tag Project-1", constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewTagOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringArrayVarP(&createFlags.Tag.Value, createFlags.Tag.Name, "t", []string{}, "Tag to apply to project, must use canonical name: <tag_set>/<tag_name>")
	flags.StringVar(&createFlags.Project.Value, createFlags.Project.Name, "", "Name or ID of the project you wish to update")

	return cmd
}

func createRun(opts *TagOptions) error {
	var optsArray []cmd.Dependable
	var err error
	if !opts.NoPrompt {
		optsArray, err = PromptMissing(opts)
		if err != nil {
			return err
		}
	} else {
		// Validate tags when running with --no-prompt
		if len(opts.Tag.Value) > 0 {
			tagSets, err := opts.GetAllTagsCallback()
			if err != nil {
				return err
			}
			if err := selectors.ValidateTags(opts.Tag.Value, tagSets); err != nil {
				return err
			}
		}
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

func PromptMissing(opts *TagOptions) ([]cmd.Dependable, error) {
	nestedOpts := []cmd.Dependable{}

	project, err := AskProjects(opts.Ask, opts.Out, opts.Project.Value, opts.GetProjectsCallback, opts.GetProjectCallback)
	if err != nil {
		return nil, err
	}
	opts.project = project
	opts.Project.Value = project.Name

	tagSets, err := opts.GetAllTagsCallback()
	if err != nil {
		return nil, err
	}

	tags, err := selectors.Tags(opts.Ask, opts.project.ProjectTags, opts.Tag.Value, tagSets)
	if err != nil {
		return nil, err
	}
	opts.Tag.Value = tags

	nestedOpts = append(nestedOpts, opts)
	return nestedOpts, nil
}

func AskProjects(ask question.Asker, out io.Writer, value string, getProjectsCallback GetProjectsCallback, getProjectCallback GetProjectCallback) (*projects.Project, error) {
	if value != "" {
		project, err := getProjectCallback(value)
		if err != nil {
			return nil, err
		}
		return project, nil
	}

	// Check if there's only one project
	projs, err := getProjectsCallback()
	if err != nil {
		return nil, err
	}
	if len(projs) == 1 {
		fmt.Fprintf(out, "Selecting only available project '%s'.\n", output.Cyan(projs[0].Name))
		return projs[0], nil
	}

	project, err := selectors.Select(ask, "Select the Project you would like to update", getProjectsCallback, func(item *projects.Project) string {
		return item.Name
	})
	if err != nil {
		return nil, err
	}

	return project, nil
}
