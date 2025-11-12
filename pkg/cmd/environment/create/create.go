package create

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tagsets"
	"github.com/spf13/cobra"
)

const (
	FlagName                  = "name"
	FlagDescription           = "description"
	FlagUseGuidedFailure      = "use-guided-failure"
	FlagDynamicInfrastructure = "allow-dynamic-infrastructure"
	FlagTag                   = "tag"
)

type CreateFlags struct {
	Name                  *flag.Flag[string]
	Description           *flag.Flag[string]
	GuidedFailureMode     *flag.Flag[bool]
	DynamicInfrastructure *flag.Flag[bool]
	Tag                   *flag.Flag[[]string]
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:                  flag.New[string](FlagName, false),
		Description:           flag.New[string](FlagDescription, false),
		GuidedFailureMode:     flag.New[bool](FlagUseGuidedFailure, false),
		DynamicInfrastructure: flag.New[bool](FlagDynamicInfrastructure, false),
		Tag:                   flag.New[[]string](FlagTag, false),
	}
}

type GetAllTagSetsCallback func() ([]*tagsets.TagSet, error)

type CreateOptions struct {
	*CreateFlags
	*cmd.Dependencies
	GetAllTagsCallback GetAllTagSetsCallback
}

func NewCreateOptions(createFlags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:        createFlags,
		Dependencies:       dependencies,
		GetAllTagsCallback: getAllTagSetsCallback(dependencies.Client),
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create an environment",
		Long:    "Create a environment in Octopus Deploy",
		Example: heredoc.Docf("$ %s environment create", constants.ExecutableName),
		Aliases: []string{"new"},
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "Name of the environment")
	flags.StringVarP(&createFlags.Description.Value, createFlags.Description.Name, "d", "", "Description of the environment")
	flags.BoolVar(&createFlags.GuidedFailureMode.Value, createFlags.GuidedFailureMode.Name, false, "Use guided failure mode by default")
	flags.BoolVar(&createFlags.DynamicInfrastructure.Value, createFlags.DynamicInfrastructure.Name, false, "Allow dynamic infrastructure")
	flags.StringArrayVarP(&createFlags.Tag.Value, createFlags.Tag.Name, "t", []string{}, "Tag to apply to environment, must use canonical name: <tag_set>/<tag_name>")

	return cmd
}

func createRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		err := PromptMissing(opts)
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
	}

	env := environments.NewEnvironment(opts.Name.Value)
	env.Description = opts.Description.Value
	env.AllowDynamicInfrastructure = opts.DynamicInfrastructure.Value
	env.UseGuidedFailure = opts.GuidedFailureMode.Value
	env.EnvironmentTags = opts.Tag.Value

	createEnv, err := opts.Client.Environments.Add(env)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(opts.Out, "\nSuccessfully created environment '%s' (%s).\n", createEnv.Name, createEnv.ID)
	if err != nil {
		return err
	}

	link := output.Bluef("%s/app#/%s/infrastructure/environments/%s", opts.Host, opts.Space.GetID(), createEnv.GetID())
	fmt.Fprintf(opts.Out, "View this environment on Octopus Deploy: %s\n", link)

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.Description, opts.GuidedFailureMode, opts.DynamicInfrastructure, opts.Tag)
		fmt.Fprintf(opts.Out, "%s\n", autoCmd)
	}

	return nil
}

func PromptMissing(opts *CreateOptions) error {
	err := question.AskName(opts.Ask, "", "environment", &opts.Name.Value)
	if err != nil {
		return err
	}

	err = question.AskDescription(opts.Ask, "", "environment", &opts.Description.Value)
	if err != nil {
		return err
	}

	_, err = promptBool(opts, &opts.GuidedFailureMode.Value, false, "Use guided failure", "If guided failure is enabled for an environment, Octopus Deploy will prompt for user intervention if a deployment fails in the environment.")
	_, err = promptBool(opts, &opts.DynamicInfrastructure.Value, false, "Allow dynamic infrastructure", "If dynamic infrastructure is enabled for an environment, deployments to this environment are allowed to create infrastructure, such as targets and accounts.")

	tagSets, err := opts.GetAllTagsCallback()
	if err != nil {
		return err
	}

	tags, err := selectors.Tags(opts.Ask, []string{}, opts.Tag.Value, tagSets)
	if err != nil {
		return err
	}
	opts.Tag.Value = tags

	return nil
}

func promptBool(opts *CreateOptions, value *bool, defaultValue bool, message string, help string) (bool, error) {
	if *value != defaultValue {
		return *value, nil
	}
	err := opts.Ask(&survey.Confirm{
		Message: message,
		Help:    help,
		Default: defaultValue,
	}, value)
	return *value, err
}

func getAllTagSetsCallback(client *client.Client) GetAllTagSetsCallback {
	return func() ([]*tagsets.TagSet, error) {
		query := tagsets.TagSetsQuery{
			Scopes: []string{string(tagsets.TagSetScopeEnvironment)},
		}
		result, err := tagsets.Get(client, client.GetSpaceID(), query)
		if err != nil {
			return nil, err
		}
		return result.Items, nil
	}
}
