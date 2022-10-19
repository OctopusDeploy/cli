package create

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/credentials"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projectgroups"
	"github.com/spf13/cobra"
)

const (
	FlagGroup                   = "group"
	FlagName                    = "name"
	FlagDescription             = "description"
	FlagLifecycle               = "lifecycle"
	FlagConfigAsCode            = "process-vcs"
	FlagGitUrl                  = "git-url"
	FlagGitBranch               = "git-branch"
	FlagGitLibraryCredentials   = "git-credentials"
	FlagGitUsername             = "git-username"
	FlagGitPassword             = "git-password"
	FlagGitCredentialStorage    = "git-credential-store"
	FlagGitInitialCommitMessage = "git-initial-commit"
	FlagGitBasePath             = "git-base-path"

	DefaultGitCommitMessage = "Initial commit of deployment process"
	DefaultBasePath         = ".octopus/"
	DefaultBranch           = "main"
	GitPersistenceType      = "VersionControlled"

	GitStorageProject = "project"
	GitStorageLibrary = "library"
)

type CreateFlags struct {
	Group        *flag.Flag[string]
	Name         *flag.Flag[string]
	Description  *flag.Flag[string]
	Lifecycle    *flag.Flag[string]
	ConfigAsCode *flag.Flag[bool]

	GitUrl                  *flag.Flag[string]
	GitBranch               *flag.Flag[string]
	GitCredentials          *flag.Flag[string]
	GitUsername             *flag.Flag[string]
	GitPassword             *flag.Flag[string]
	GitStorage              *flag.Flag[string]
	GitInitialCommitMessage *flag.Flag[string]
	GitBasePath             *flag.Flag[string]
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Group:                   flag.New[string](FlagGroup, false),
		Name:                    flag.New[string](FlagName, false),
		Description:             flag.New[string](FlagDescription, false),
		Lifecycle:               flag.New[string](FlagLifecycle, false),
		ConfigAsCode:            flag.New[bool](FlagConfigAsCode, false),
		GitStorage:              flag.New[string](FlagGitCredentialStorage, false),
		GitUrl:                  flag.New[string](FlagGitUrl, false),
		GitBranch:               flag.New[string](FlagGitBranch, false),
		GitInitialCommitMessage: flag.New[string](FlagGitInitialCommitMessage, false),
		GitCredentials:          flag.New[string](FlagGitLibraryCredentials, false),
		GitUsername:             flag.New[string](FlagGitUsername, false),
		GitPassword:             flag.New[string](FlagGitPassword, true),
		GitBasePath:             flag.New[string](FlagGitBasePath, false),
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags()

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a new project in Octopus Deploy",
		Long:  "Creates a new project in Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s project create
			$ %s project create --process-vcs 
			$ %s project create --name 'Deploy web app' --lifecycle 'Default Lifecycle' --group 'Default Project Group'
		`), constants.ExecutableName, constants.ExecutableName, constants.ExecutableName),
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

	flags.StringVar(&createFlags.GitUrl.Value, createFlags.GitUrl.Name, "", "Url of the Git repository for storing project configuration")
	flags.StringVar(&createFlags.GitBranch.Value, createFlags.GitBranch.Name, "", fmt.Sprintf("The default branch to use for Config As Code. Default is '%s'.", DefaultBranch))
	flags.StringVar(&createFlags.GitCredentials.Value, createFlags.GitCredentials.Name, "", "The Id or name of the Git credentials stored in Octopus")
	flags.StringVar(&createFlags.GitUsername.Value, createFlags.GitUsername.Name, "", "The username to authenticate with Git")
	flags.StringVar(&createFlags.GitPassword.Value, createFlags.GitPassword.Name, "", "The password to authenticate with Git")
	flags.StringVar(&createFlags.GitStorage.Value, createFlags.GitStorage.Name, "", "The location to store the supplied Git credentials. Options are library or project. Default is library")
	flags.StringVar(&createFlags.GitInitialCommitMessage.Value, createFlags.GitInitialCommitMessage.Name, "", "The initial commit message for configuring Config As Code.")
	flags.StringVar(&createFlags.GitBasePath.Value, createFlags.GitBasePath.Name, "", fmt.Sprintf("The directory where Octopus should store the project files in the repository. Default is '%s'", DefaultBasePath))
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

	question.AskName(opts.Ask, "", "project", &opts.Name.Value)

	if opts.Lifecycle.Value == "" {
		lc, err := selectors.Lifecycle("You have not specified a Lifecycle for this project. Please select one:", opts.Client, opts.Ask)
		if err != nil {
			return nil, err
		}
		opts.Lifecycle.Value = lc.Name
	}

	value, projectGroupOpt, err := AskProjectGroups(opts.Ask, opts.Group.Value, opts.GetAllGroupsCallback, opts.CreateProjectGroupCallback)
	if err != nil {
		return nil, err
	}
	opts.Group.Value = value
	if projectGroupOpt != nil {
		nestedOpts = append(nestedOpts, projectGroupOpt)
	}

	err = PromptForConfigAsCode(opts, opts.GetAllGitCredentialsCallback)
	if err != nil {
		return nil, err
	}

	nestedOpts = append(nestedOpts, opts)
	return nestedOpts, nil
}

func AskProjectGroups(ask question.Asker, value string, getAllGroupsCallback GetAllGroupsCallback, createProjectGroupCallback CreateProjectGroupCallback) (string, cmd.Dependable, error) {
	if value != "" {
		return value, nil, nil
	}
	var shouldCreateNewProjectGroup bool
	ask(&survey.Confirm{
		Message: "Would you like to create a new Project Group?",
		Default: false,
	}, &shouldCreateNewProjectGroup)

	if shouldCreateNewProjectGroup {
		return createProjectGroupCallback()
	}

	g, err := selectors.Select(ask, "You have not specified a Project group for this project. Please select one:", getAllGroupsCallback, func(pg *projectgroups.ProjectGroup) string {
		return pg.Name
	})
	if err != nil {
		return "", nil, err
	}
	return g.Name, nil, nil

}

func PromptForConfigAsCode(opts *CreateOptions, getGitCredentialsCallback GetAllGitCredentialsCallback) error {
	if !opts.ConfigAsCode.Value {
		opts.Ask(&survey.Confirm{
			Message: "Would you like to use Config as Code?",
			Default: false,
		}, &opts.ConfigAsCode.Value)
	}

	if opts.ConfigAsCode.Value {
		if opts.GitStorage.Value == "" {
			selectedOption, err := selectors.SelectOptions[string](opts.Ask, "Select where to store the Git credentials", getGitStorageOptions)

			if err != nil {
				return err
			}
			opts.GitStorage.Value = selectedOption.Value
		}

		if opts.GitUrl.Value == "" {
			if err := opts.Ask(&survey.Input{
				Message: "Git URL",
				Help:    "The URL of the Git repository to store configuration.",
			}, &opts.GitUrl.Value, survey.WithValidator(survey.ComposeValidators(
				survey.MaxLength(200),
				survey.Required,
			))); err != nil {
				return err
			}
		}

		if opts.GitBasePath.Value == "" {
			if err := opts.Ask(&survey.Input{
				Message: "Git repository base path",
				Help:    fmt.Sprintf("The path in the repository where Config As Code settings are stored. Default value is '%s'.", DefaultBasePath),
			}, &opts.GitBasePath.Value, survey.WithValidator(survey.ComposeValidators(
				survey.MaxLength(200),
			))); err != nil {
				return err
			}
		}

		if opts.GitBranch.Value == "" {
			if err := opts.Ask(&survey.Input{
				Message: "Git branch",
				Help:    fmt.Sprintf("The default branch to use. Default value is '%s'.", DefaultBranch),
			}, &opts.GitBranch.Value, survey.WithValidator(survey.ComposeValidators(
				survey.MaxLength(200),
			))); err != nil {
				return err
			}
		}

		if opts.GitInitialCommitMessage.Value == "" {
			if err := opts.Ask(&survey.Input{
				Message: "Initial Git commit message",
				Help:    fmt.Sprintf("The commit message used in initializing. Default value is '%s'.", DefaultGitCommitMessage),
			}, &opts.GitInitialCommitMessage.Value, survey.WithValidator(survey.ComposeValidators(
				survey.MaxLength(50),
			))); err != nil {
				return err
			}
		}

		if opts.GitStorage.Value == GitStorageLibrary {
			err := promptLibraryGitCredentials(opts, getGitCredentialsCallback)
			if err != nil {
				return err
			}
		} else {
			err := promptProjectGitCredentials(opts)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func promptProjectGitCredentials(opts *CreateOptions) error {
	if opts.GitUsername.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Git username",
			Help:    "The Git username.",
		}, &opts.GitUsername.Value, survey.WithValidator(survey.ComposeValidators(
			survey.MaxLength(200),
			survey.Required,
		))); err != nil {
			return err
		}
	}

	if opts.GitPassword.Value == "" {
		if err := opts.Ask(&survey.Password{
			Message: "Git password",
			Help:    "The Git password.",
		}, &opts.GitPassword.Value, survey.WithValidator(survey.ComposeValidators(
			survey.MaxLength(200),
			survey.Required,
		))); err != nil {
			return err
		}
	}
	return nil
}

func promptLibraryGitCredentials(opts *CreateOptions, gitCredentialsCallback GetAllGitCredentialsCallback) error {
	if opts.GitCredentials.Value == "" {
		selectedOption, err := selectors.Select(opts.Ask, "Select which Git credentials to use", gitCredentialsCallback, func(resource *credentials.Resource) string { return resource.Name })

		if err != nil {
			return err
		}
		opts.GitCredentials.Value = selectedOption.GetName()
	}
	return nil
}

func getGitStorageOptions() []*selectors.SelectOption[string] {
	return []*selectors.SelectOption[string]{
		{Display: "Library", Value: GitStorageLibrary},
		{Display: "Project", Value: GitStorageProject},
	}
}
