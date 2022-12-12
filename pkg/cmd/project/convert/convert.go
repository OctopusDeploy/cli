package convert

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/credentials"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"net/url"
	"strings"
)

const (
	FlagProject = "project"

	FlagGitUrl                  = "git-url"
	FlagGitBranch               = "git-branch"
	FlagGitLibraryCredentials   = "git-credentials"
	FlagGitUsername             = "git-username"
	FlagGitPassword             = "git-password"
	FlagGitCredentialStorage    = "git-credential-store"
	FlagGitInitialCommitMessage = "git-initial-commit"
	FlagGitBasePath             = "git-base-path"
	FlagDefaultBranchProtected  = "git-protected-default-branch"
	FlagInitialCommitBranch     = "git-initial-commit-branch"
	FlagBranchProtectionPattern = "git-protected-branch-pattern"

	DefaultGitCommitMessage = "Initial commit of deployment process"
	DefaultBasePath         = ".octopus/"
	DefaultBranch           = "main"

	GitStorageProject = "project"
	GitStorageLibrary = "library"
)

type ConvertProjectGroupCallback func(project *projects.Project) (cmd.Dependable, error)
type GetAllGitCredentialsCallback func() ([]*credentials.Resource, error)

type ConvertFlags struct {
	Project                    *flag.Flag[string]
	GitUrl                     *flag.Flag[string]
	GitBranch                  *flag.Flag[string]
	GitCredentials             *flag.Flag[string]
	GitUsername                *flag.Flag[string]
	GitPassword                *flag.Flag[string]
	GitStorage                 *flag.Flag[string]
	GitInitialCommitMessage    *flag.Flag[string]
	GitDefaultBranchProtected  *flag.Flag[bool]
	GitInitialCommitBranch     *flag.Flag[string]
	GitBasePath                *flag.Flag[string]
	GitProtectedBranchPatterns *flag.Flag[[]string]
}

func NewConvertFlags() *ConvertFlags {
	return &ConvertFlags{
		Project:                    flag.New[string](FlagProject, false),
		GitStorage:                 flag.New[string](FlagGitCredentialStorage, false),
		GitUrl:                     flag.New[string](FlagGitUrl, false),
		GitBranch:                  flag.New[string](FlagGitBranch, false),
		GitInitialCommitMessage:    flag.New[string](FlagGitInitialCommitMessage, false),
		GitCredentials:             flag.New[string](FlagGitLibraryCredentials, false),
		GitDefaultBranchProtected:  flag.New[bool](FlagDefaultBranchProtected, false),
		GitProtectedBranchPatterns: flag.New[[]string](FlagBranchProtectionPattern, false),
		GitInitialCommitBranch:     flag.New[string](FlagInitialCommitBranch, false),
		GitUsername:                flag.New[string](FlagGitUsername, false),
		GitPassword:                flag.New[string](FlagGitPassword, true),
		GitBasePath:                flag.New[string](FlagGitBasePath, false),
	}
}

type ConvertOptions struct {
	*ConvertFlags
	*cmd.Dependencies
	GetAllGitCredentialsCallback
	GetAllProjectsCallback shared.GetAllProjectsCallback
	GetProjectCallback     shared.GetProjectCallback
}

func NewConvertOptions(flags *ConvertFlags, dependencies *cmd.Dependencies) *ConvertOptions {
	return &ConvertOptions{
		ConvertFlags: flags,
		Dependencies: dependencies,
		GetAllGitCredentialsCallback: func() ([]*credentials.Resource, error) {
			return createGetAllGitCredentialsCallback(*dependencies.Client)
		},
		GetProjectCallback: func(identifier string) (*projects.Project, error) {
			return shared.GetProject(*dependencies.Client, identifier)
		},
		GetAllProjectsCallback: func() ([]*projects.Project, error) { return shared.GetAllProjects(*dependencies.Client) },
	}
}

func NewCmdConvert(f factory.Factory) *cobra.Command {
	convertProjectFlags := NewConvertFlags()
	cmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert a project to use Config As Code",
		Long:  "Convert a project to use Config As Code in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project convert
			$ %[1]s project convert --project "Deploy web site" --git-url https://github.com/orgname/reponame"
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewConvertOptions(convertProjectFlags, cmd.NewDependencies(f, c))
			return convertRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&convertProjectFlags.Project.Value, convertProjectFlags.Project.Name, "p", "", "Name, ID or Slug of the project to convert")
	RegisterCacFlags(flags, convertProjectFlags)

	return cmd
}

func RegisterCacFlags(flags *pflag.FlagSet, convertFlags *ConvertFlags) {
	flags.StringVar(&convertFlags.GitUrl.Value, convertFlags.GitUrl.Name, "", "Url of the Git repository for storing project configuration")
	flags.StringVar(&convertFlags.GitBranch.Value, convertFlags.GitBranch.Name, "", fmt.Sprintf("The default branch to use for Config As Code. Default is '%s'.", DefaultBranch))
	flags.StringVar(&convertFlags.GitCredentials.Value, convertFlags.GitCredentials.Name, "", "The Id or name of the Git credentials stored in Octopus")
	flags.StringVar(&convertFlags.GitUsername.Value, convertFlags.GitUsername.Name, "", "The username to authenticate with Git")
	flags.StringVar(&convertFlags.GitPassword.Value, convertFlags.GitPassword.Name, "", "The password to authenticate with Git")
	flags.StringVar(&convertFlags.GitStorage.Value, convertFlags.GitStorage.Name, "", "The location to store the supplied Git credentials. Options are library or project. Default is library")
	flags.StringVar(&convertFlags.GitInitialCommitMessage.Value, convertFlags.GitInitialCommitMessage.Name, "", "The initial commit message for configuring Config As Code.")
	flags.StringVar(&convertFlags.GitBasePath.Value, convertFlags.GitBasePath.Name, "", fmt.Sprintf("The directory where Octopus should store the project files in the repository. Default is '%s'", DefaultBasePath))
	flags.BoolVar(&convertFlags.GitDefaultBranchProtected.Value, convertFlags.GitDefaultBranchProtected.Name, false, "Protect the default branch from having Config As Code settings committed directly.")
	flags.StringVar(&convertFlags.GitInitialCommitBranch.Value, convertFlags.GitInitialCommitBranch.Name, "", fmt.Sprintf("The branch to initially commit Config As Code settings. Only required if '%s' is supplied.", convertFlags.GitDefaultBranchProtected.Name))
	flags.StringSliceVar(&convertFlags.GitProtectedBranchPatterns.Value, convertFlags.GitProtectedBranchPatterns.Name, []string{}, "Git branches which are protected from having Config As Code settings committed directly")
}

func convertRun(opts *ConvertOptions) error {
	if !opts.NoPrompt {
		err := PromptMissing(opts)
		if err != nil {
			return err
		}
	}

	err := opts.Commit()
	if err != nil {
		return err
	}

	if !opts.NoPrompt {
		fmt.Fprintln(opts.Out, "\nAutomation Commands:")
		opts.GenerateAutomationCmd()
	}

	return nil
}

func PromptMissing(opts *ConvertOptions) error {
	if opts.Project.Value == "" {
		allProjects, err := opts.GetAllProjectsCallback()
		if err != nil {
			return err
		}
		project, err := question.SelectMap(opts.Ask, "You have not specified a project. Please select one:", allProjects, func(p *projects.Project) string { return p.GetName() })
		if err != nil {
			return nil
		}
		opts.Project.Value = project.GetName()
	}

	_, err := PromptForConfigAsCode(opts)
	if err != nil {
		return err
	}

	return nil
}

func (co *ConvertOptions) Commit() error {
	gitPersistenceSettings, err := co.buildGitPersistenceSettings()
	if err != nil {
		return err
	}

	project, err := co.GetProjectCallback(co.Project.Value)
	if err != nil {
		return err
	}
	_, err = co.Client.Projects.ConvertToVcs(project, co.GitInitialCommitMessage.Value, co.GitInitialCommitBranch.Value, gitPersistenceSettings)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(co.Out, "Successfully configured Config as Code on '%s'\n", project.GetName())
	if err != nil {
		return err
	}

	return nil
}

func (co *ConvertOptions) GenerateAutomationCmd() {
	if !co.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(co.CmdPath, co.Project, co.GitStorage, co.GitBasePath, co.GitUrl, co.GitBranch, co.GitInitialCommitMessage, co.GitCredentials, co.GitUsername, co.GitPassword, co.GitInitialCommitBranch, co.GitDefaultBranchProtected, co.GitProtectedBranchPatterns)
		fmt.Fprintf(co.Out, "%s\n", autoCmd)
	}
}

func PromptForConfigAsCode(opts *ConvertOptions) (cmd.Dependable, error) {
	if opts.GitStorage.Value == "" {
		selectedOption, err := selectors.SelectOptions(opts.Ask, "Select where to store the Git credentials", getGitStorageOptions)

		if err != nil {
			return nil, err
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
			return nil, err
		}
	}

	if opts.GitBasePath.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Git repository base path",
			Help:    fmt.Sprintf("The path in the repository where Config As Code settings are stored. Default value is '%s'.", DefaultBasePath),
			Default: DefaultBasePath,
		}, &opts.GitBasePath.Value, survey.WithValidator(survey.ComposeValidators(
			survey.MaxLength(200),
		))); err != nil {
			return nil, err
		}
	}

	if opts.GitBranch.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Git branch",
			Help:    fmt.Sprintf("The default branch to use. Default value is '%s'.", DefaultBranch),
			Default: DefaultBranch,
		}, &opts.GitBranch.Value, survey.WithValidator(survey.ComposeValidators(
			survey.MaxLength(200),
		))); err != nil {
			return nil, err
		}
	}

	if !opts.GitDefaultBranchProtected.Value {
		opts.Ask(&survey.Confirm{
			Message: fmt.Sprintf("Is the '%s' branch protected?", opts.GitBranch.Value),
			Help:    "If the default branch is protected, you may not have permission to push to it.",
			Default: false,
		}, &opts.GitDefaultBranchProtected.Value)
	}

	if opts.GitDefaultBranchProtected.Value && opts.GitInitialCommitBranch.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Initial commit branch name",
			Help:    "The branch where the Config As Code settings will be initially committed",
		}, &opts.GitInitialCommitBranch.Value, survey.WithValidator(survey.ComposeValidators(
			survey.Required,
			survey.MaxLength(200),
		))); err != nil {
			return nil, err
		}
	}

	if util.Empty(opts.GitProtectedBranchPatterns.Value) {
		for {
			var pattern string
			if err := opts.Ask(&survey.Input{
				Message: "Enter a protected branch pattern (enter blank to end)",
				Help:    "This setting only applies within Octopus and will not affect your protected branches in Git. Use wildcard syntax to specify the range of branches to include. Multiple patterns can be supplied",
			}, &pattern, survey.WithValidator(survey.MaxLength(200))); err != nil {
				return nil, err
			}

			if pattern == "" {
				break
			}
			opts.GitProtectedBranchPatterns.Value = append(opts.GitProtectedBranchPatterns.Value, pattern)
		}
	}

	if opts.GitInitialCommitMessage.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Initial Git commit message",
			Help:    fmt.Sprintf("The commit message used in initializing. Default value is '%s'.", DefaultGitCommitMessage),
			Default: DefaultGitCommitMessage,
		}, &opts.GitInitialCommitMessage.Value, survey.WithValidator(survey.ComposeValidators(
			survey.MaxLength(50),
		))); err != nil {
			return nil, err
		}
	}

	if opts.GitStorage.Value == GitStorageLibrary {
		err := promptLibraryGitCredentials(opts, opts.GetAllGitCredentialsCallback)
		if err != nil {
			return nil, err
		}
	} else {
		err := promptProjectGitCredentials(opts)
		if err != nil {
			return nil, err
		}
	}

	return opts, nil
}

func promptProjectGitCredentials(opts *ConvertOptions) error {
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

func promptLibraryGitCredentials(opts *ConvertOptions, gitCredentialsCallback GetAllGitCredentialsCallback) error {
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

func createGetAllGitCredentialsCallback(client client.Client) ([]*credentials.Resource, error) {
	res, err := client.GitCredentials.Get(credentials.Query{})
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

func (co *ConvertOptions) buildGitPersistenceSettings() (projects.GitPersistenceSettings, error) {
	var credentials credentials.GitCredential
	var err error
	if strings.EqualFold(co.GitStorage.Value, GitStorageLibrary) {
		credentials, err = co.buildLibraryGitVersionControlSettings()
		if err != nil {
			return nil, err
		}
	} else {
		credentials, err = co.buildProjectGitVersionControlSettings()
		if err != nil {
			return nil, err
		}
	}
	url, err := url.Parse(co.GitUrl.Value)
	if err != nil {
		return nil, err
	}

	vcs := projects.NewGitPersistenceSettings(co.GitBasePath.Value, credentials, co.GitBranch.Value, co.GitDefaultBranchProtected.Value, co.GitProtectedBranchPatterns.Value, url)
	return vcs, nil
}

func (co *ConvertOptions) buildLibraryGitVersionControlSettings() (credentials.GitCredential, error) {
	creds, err := co.Client.GitCredentials.GetByIDOrName(co.GitCredentials.Value)
	if err != nil {
		return nil, err
	}

	credentials := credentials.NewReference(creds.GetID())
	return credentials, nil
}

func (co *ConvertOptions) buildProjectGitVersionControlSettings() (credentials.GitCredential, error) {
	credentials := credentials.NewUsernamePassword(co.GitUsername.Value, core.NewSensitiveValue(co.GitPassword.Value))
	return credentials, nil
}
