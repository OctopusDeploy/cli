package tag

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/spf13/cobra"
)

const (
	FlagTag         = "tag"
	FlagEnvironment = "environment"
)

type TagFlags struct {
	Tag         *flag.Flag[[]string]
	Environment *flag.Flag[string]
}

func NewTagFlags() *TagFlags {
	return &TagFlags{
		Tag:         flag.New[[]string](FlagTag, false),
		Environment: flag.New[string](FlagEnvironment, false),
	}
}

func NewCmdTag(f factory.Factory) *cobra.Command {
	createFlags := NewTagFlags()

	cmd := &cobra.Command{
		Use:     "tag",
		Short:   "Override tags for an environment",
		Long:    "Override tags for an environment in Octopus Deploy",
		Example: heredoc.Docf("$ %s environment tag Environment-1", constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewTagOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringArrayVarP(&createFlags.Tag.Value, createFlags.Tag.Name, "t", []string{}, "Tag to apply to environment, must use canonical name: <tag_set>/<tag_name>")
	flags.StringVar(&createFlags.Environment.Value, createFlags.Environment.Name, "", "Name or ID of the environment you wish to update")

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

	environment, err := AskEnvironments(opts.Ask, opts.Environment.Value, opts.GetEnvironmentsCallback, opts.GetEnvironmentCallback)
	if err != nil {
		return nil, err
	}
	opts.environment = environment
	opts.Environment.Value = environment.Name

	tagSets, err := opts.GetAllTagsCallback()
	if err != nil {
		return nil, err
	}

	tags, err := selectors.Tags(opts.Ask, opts.environment.EnvironmentTags, opts.Tag.Value, tagSets)
	if err != nil {
		return nil, err
	}
	opts.Tag.Value = tags

	nestedOpts = append(nestedOpts, opts)
	return nestedOpts, nil
}

func AskEnvironments(ask question.Asker, value string, getEnvironmentsCallback GetEnvironmentsCallback, getEnvironmentCallback GetEnvironmentCallback) (*environments.Environment, error) {
	if value != "" {
		environment, err := getEnvironmentCallback(value)
		if err != nil {
			return nil, err
		}
		return environment, nil
	}

	environment, err := selectors.Select(ask, "Select the Environment you would like to update", getEnvironmentsCallback, func(item *environments.Environment) string {
		return item.Name
	})
	if err != nil {
		return nil, err
	}

	return environment, nil
}

