package create

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/project/convert"
	projectShared "github.com/OctopusDeploy/cli/pkg/cmd/project/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
)

const (
	FlagGroup        = "group"
	FlagName         = "name"
	FlagDescription  = "description"
	FlagLifecycle    = "lifecycle"
	FlagConfigAsCode = "process-vcs"
	FlagTag          = "tag"
)

type CreateFlags struct {
	Group        *flag.Flag[string]
	Name         *flag.Flag[string]
	Description  *flag.Flag[string]
	Lifecycle    *flag.Flag[string]
	ConfigAsCode *flag.Flag[bool]
	Tag          *flag.Flag[[]string]

	ProjectConvertFlags *convert.ConvertFlags
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Group:               flag.New[string](FlagGroup, false),
		Name:                flag.New[string](FlagName, false),
		Description:         flag.New[string](FlagDescription, false),
		Lifecycle:           flag.New[string](FlagLifecycle, false),
		ConfigAsCode:        flag.New[bool](FlagConfigAsCode, false),
		Tag:                 flag.New[[]string](FlagTag, false),
		ProjectConvertFlags: convert.NewConvertFlags(),
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a project",
		Long:  "Create a project in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project create
			$ %[1]s project create --process-vcs
			$ %[1]s project create --name 'Deploy web app' --lifecycle 'Default Lifecycle' --group 'Default Project Group'
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "Name of the project")
	flags.StringVarP(&createFlags.Description.Value, createFlags.Description.Name, "d", "", "Description of the project")
	flags.StringVarP(&createFlags.Group.Value, createFlags.Group.Name, "g", "", "Project group of the project")
	flags.StringVarP(&createFlags.Lifecycle.Value, createFlags.Lifecycle.Name, "l", "", "Lifecycle of the project")
	flags.BoolVar(&createFlags.ConfigAsCode.Value, createFlags.ConfigAsCode.Name, false, "Use Config As Code for the project")
	flags.StringArrayVarP(&createFlags.Tag.Value, createFlags.Tag.Name, "t", []string{}, "Tag to apply to project, must use canonical name: <tag_set>/<tag_name>")
	convert.RegisterCacFlags(flags, createFlags.ProjectConvertFlags)
	flags.SortFlags = false

	return cmd
}

func createRun(opts *CreateOptions) error {
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
		if opts.ConfigAsCode.Value {
			opts.ConvertOptions.Project.Value = opts.Name.Value
			optsArray = append(optsArray, opts.ConvertOptions)
		}
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

func PromptMissing(opts *CreateOptions) ([]cmd.Dependable, error) {
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

	tagSets, err := opts.GetAllTagsCallback()
	if err != nil {
		return nil, err
	}

	tags, err := selectors.Tags(opts.Ask, []string{}, opts.Tag.Value, tagSets)
	if err != nil {
		return nil, err
	}
	opts.Tag.Value = tags

	configAsCodeOpts, err := PromptForConfigAsCode(opts)
	opts.ConvertOptions.Project.Value = opts.Name.Value
	if err != nil {
		return nil, err
	}
	if configAsCodeOpts != nil {
		nestedOpts = append(nestedOpts, configAsCodeOpts)
	}

	return nestedOpts, nil
}

func PromptForConfigAsCode(opts *CreateOptions) (cmd.Dependable, error) {
	if !opts.ConfigAsCode.Value {
		opts.Ask(&survey.Confirm{
			Message: "Would you like to use Config as Code?",
			Default: false,
		}, &opts.ConfigAsCode.Value)
	}

	if opts.ConfigAsCode.Value {
		return opts.ConvertProjectCallback()
	}

	return nil, nil
}
