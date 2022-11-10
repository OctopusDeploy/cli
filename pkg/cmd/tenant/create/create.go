package create

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/spf13/cobra"
)

const (
	FlagName        = "name"
	FlagDescription = "description"
	FlagTag         = "tag"
)

type CreateFlags struct {
	Name        *flag.Flag[string]
	Description *flag.Flag[string]
	Tag         *flag.Flag[[]string]
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:        flag.New[string](FlagName, false),
		Description: flag.New[string](FlagDescription, false),
		Tag:         flag.New[[]string](FlagTag, false),
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a new tenant in Octopus Deploy",
		Long:  "Creates a new tenant in Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s tenant create
		`), constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "Name of the tenant")
	flags.StringVarP(&createFlags.Description.Value, createFlags.Description.Name, "d", "", "Description of the tenant")
	flags.StringArrayVarP(&createFlags.Tag.Value, createFlags.Tag.Name, "t", []string{}, "Tag to apply to tenant, must use canonical name: <tag_set>/<tag_name>")

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

func PromptMissing(opts *CreateOptions) ([]cmd.Dependable, error) {
	nestedOpts := []cmd.Dependable{}

	question.AskName(opts.Ask, "", "tenant", &opts.Name.Value)

	if err := opts.Ask(&survey.Input{
		Message: "Description",
	}, &opts.Description.Value); err != nil {
		return nestedOpts, err
	}

	tags, err := AskTags(opts.Ask, opts.Tag.Value, opts.GetAllTagsCallback)
	if err != nil {
		return nil, err
	}
	opts.Tag.Value = tags

	nestedOpts = append(nestedOpts, opts)
	return nestedOpts, nil
}

func AskTags(ask question.Asker, value []string, getAllTagSetsCallback GetAllTagSetsCallback) ([]string, error) {
	if len(value) > 0 {
		return value, nil
	}
	tagSets, err := getAllTagSetsCallback()
	if err != nil {
		return nil, err
	}

	canonicalTagName := []string{}
	for _, tagSet := range tagSets {
		for _, tag := range tagSet.Tags {
			canonicalTagName = append(canonicalTagName, tag.CanonicalTagName)
		}
	}
	tags := []string{}
	err = ask(&survey.MultiSelect{
		Options: canonicalTagName,
		Message: "Tags",
	}, &tags)
	if err != nil {
		return nil, err
	}
	return tags, nil
}
