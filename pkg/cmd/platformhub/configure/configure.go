package configure

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/credentials"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/githubconnections"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/platformhubgithubconnections"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/platformhubversioncontrolsettings"
	"github.com/spf13/cobra"
)

const (
	FlagGithubConnection = "github-connection"
	FlagGithubRepository = "github-repository"
	FlagGitUrl           = "git-url"
	FlagGitUsername      = "git-username"
	FlagGitPassword      = "git-password"
	FlagGitBasePath      = "git-base-path"
	FlagGitBranch        = "git-branch"

	DefaultBasePath = ".octopus/"
	DefaultBranch   = "main"

	connectionTypeGitHub     = "GitHub Connection"
	connectionTypeCredential = "Git Credential"
)

// ErrNoGitHubConnections is returned from PromptMissing when the user chose to use a GitHub
// connection but the Platform Hub has none configured. Callers should treat it as a clean stop,
// not a failure.
var ErrNoGitHubConnections = errors.New("no Platform Hub GitHub connections are configured")

type GetSettingsCallback func() (*platformhubversioncontrolsettings.Resource, error)
type GetGitHubAppSettingsCallback func() (*githubconnections.Settings, error)
type GetAllConnectionsCallback func() ([]*githubconnections.Connection, error)
type GetRepositoriesCallback func(connectionID string) ([]*githubconnections.Repository, error)

type ConfigureFlags struct {
	GithubConnection *flag.Flag[string]
	GithubRepository *flag.Flag[string]
	GitUrl           *flag.Flag[string]
	GitUsername      *flag.Flag[string]
	GitPassword      *flag.Flag[string]
	GitBasePath      *flag.Flag[string]
	GitBranch        *flag.Flag[string]
}

func NewConfigureFlags() *ConfigureFlags {
	return &ConfigureFlags{
		GithubConnection: flag.New[string](FlagGithubConnection, false),
		GithubRepository: flag.New[string](FlagGithubRepository, false),
		GitUrl:           flag.New[string](FlagGitUrl, false),
		GitUsername:      flag.New[string](FlagGitUsername, false),
		GitPassword:      flag.New[string](FlagGitPassword, true),
		GitBasePath:      flag.New[string](FlagGitBasePath, false),
		GitBranch:        flag.New[string](FlagGitBranch, false),
	}
}

type ConfigureOptions struct {
	*ConfigureFlags
	*cmd.Dependencies
	GetSettingsCallback
	GetGitHubAppSettingsCallback
	GetAllConnectionsCallback
	GetRepositoriesCallback
}

func NewConfigureOptions(flags *ConfigureFlags, dependencies *cmd.Dependencies) *ConfigureOptions {
	return &ConfigureOptions{
		ConfigureFlags: flags,
		Dependencies:   dependencies,
		GetSettingsCallback: func() (*platformhubversioncontrolsettings.Resource, error) {
			return platformhubversioncontrolsettings.Get(dependencies.Client)
		},
		GetGitHubAppSettingsCallback: func() (*githubconnections.Settings, error) {
			return githubconnections.GetSettings(dependencies.Client)
		},
		GetAllConnectionsCallback: func() ([]*githubconnections.Connection, error) {
			return getAllConnections(dependencies.Client)
		},
		GetRepositoriesCallback: func(connectionID string) ([]*githubconnections.Repository, error) {
			return platformhubgithubconnections.GetRepositories(dependencies.Client, connectionID)
		},
	}
}

func getAllConnections(client *client.Client) ([]*githubconnections.Connection, error) {
	all, err := platformhubgithubconnections.List(client, 0, math.MaxInt32)
	if err != nil {
		return nil, err
	}
	return all.Connections, nil
}

func NewCmdConfigure(f factory.Factory) *cobra.Command {
	configureFlags := NewConfigureFlags()
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Configure Platform Hub version control",
		Long:  "Configure Platform Hub version control settings in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s platform-hub configure
			%[1]s platform-hub configure --github-connection octo-org --github-repository my-repo --git-base-path .octopus/
			%[1]s platform-hub configure --git-username user --git-password pass --git-url https://github.com/org/repo.git --git-branch main
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewConfigureOptions(configureFlags, cmd.NewSystemDependencies(f, c))
			return configureRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&configureFlags.GithubConnection.Value, configureFlags.GithubConnection.Name, "", "The Id or account name of the Platform Hub GitHub connection to use")
	flags.StringVar(&configureFlags.GithubRepository.Value, configureFlags.GithubRepository.Name, "", "The name of the repository to use")
	flags.StringVar(&configureFlags.GitUrl.Value, configureFlags.GitUrl.Name, "", "Url of the Git repository for storing Platform Hub configuration")
	flags.StringVar(&configureFlags.GitUsername.Value, configureFlags.GitUsername.Name, "", "The username to authenticate with Git")
	flags.StringVar(&configureFlags.GitPassword.Value, configureFlags.GitPassword.Name, "", "The password to authenticate with Git")
	flags.StringVar(&configureFlags.GitBasePath.Value, configureFlags.GitBasePath.Name, "", fmt.Sprintf("The directory where Octopus should store Platform Hub files in the repository. Default is '%s'", DefaultBasePath))
	flags.StringVar(&configureFlags.GitBranch.Value, configureFlags.GitBranch.Name, "", fmt.Sprintf("The default branch to use. Default is '%s'", DefaultBranch))

	return cmd
}

func configureRun(opts *ConfigureOptions) error {
	if err := ValidateFlags(opts); err != nil {
		return err
	}

	current, err := opts.GetSettingsCallback()
	if err != nil {
		return err
	}

	if opts.NoPrompt {
		if usingGitHubFlags(opts) {
			if err := resolveGitHubTarget(opts); err != nil {
				return err
			}
		}
	} else {
		if isConfigured(current) {
			fmt.Fprintln(opts.Out, "Platform Hub version control is currently configured to use:")
			fmt.Fprintf(opts.Out, "  Url: %s\n", current.URL)
			fmt.Fprintf(opts.Out, "  Branch: %s\n", current.DefaultBranch)
			fmt.Fprintf(opts.Out, "  Base path: %s\n", current.BasePath)

			var confirmed bool
			if err := opts.Ask(&survey.Confirm{
				Message: "Reconfiguring will repoint Platform Hub at a different Git repository. Continue?",
				Default: false,
			}, &confirmed); err != nil {
				return err
			}
			if !confirmed {
				return nil
			}
		}

		if err := PromptMissing(opts); err != nil {
			if errors.Is(err, ErrNoGitHubConnections) {
				return nil
			}
			return err
		}
	}

	if err := opts.Commit(); err != nil {
		return err
	}

	if !opts.NoPrompt {
		opts.GenerateAutomationCmd()
	}

	return nil
}

func isConfigured(settings *platformhubversioncontrolsettings.Resource) bool {
	return settings != nil && settings.URL != ""
}

func usingGitHubFlags(opts *ConfigureOptions) bool {
	return opts.GithubConnection.Value != "" || usingGitHubRepositoryFlag(opts)
}

func usingGitHubRepositoryFlag(opts *ConfigureOptions) bool {
	return opts.GithubRepository.Value != ""
}

func usingGitCredentialFlags(opts *ConfigureOptions) bool {
	return opts.GitUsername.Value != "" || opts.GitPassword.Value != ""
}

// ValidateFlags enforces that the GitHub-connection and Git-credential flag groups are mutually
// exclusive, and that --git-url/--git-branch (which only make sense for the credential path) are
// not combined with --github-connection/--github-repository (which infer them from the
// selected repository).
func ValidateFlags(opts *ConfigureOptions) error {
	if usingGitHubFlags(opts) && usingGitCredentialFlags(opts) {
		return fmt.Errorf("GitHub connections do not accept Git credentials")
	}

	if usingGitHubRepositoryFlag(opts) && (opts.GitUrl.Value != "" || opts.GitBranch.Value != "") {
		return fmt.Errorf("--%s and --%s cannot be specified together with --%s", FlagGitUrl, FlagGitBranch, FlagGithubRepository)
	}

	return nil
}

func PromptMissing(opts *ConfigureOptions) error {
	useGitHub := usingGitHubFlags(opts)

	if !useGitHub && !usingGitCredentialFlags(opts) {
		settings, err := opts.GetGitHubAppSettingsCallback()
		if err != nil {
			return err
		}

		if settings != nil && settings.CanUseGitHubApp {
			selectedOption, err := selectors.SelectOptions(opts.Ask, "Select a Git connection type", getConnectionTypeOptions)
			if err != nil {
				return err
			}
			useGitHub = selectedOption.Value == connectionTypeGitHub
		}
	}

	if useGitHub {
		if err := resolveGitHubTarget(opts); err != nil {
			return err
		}
	} else {
		if err := promptGitCredential(opts); err != nil {
			return err
		}
	}

	if opts.GitUrl.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Git URL",
			Help:    "The URL of the Git repository to store Platform Hub configuration.",
		}, &opts.GitUrl.Value, survey.WithValidator(survey.ComposeValidators(
			survey.MaxLength(200),
			survey.Required,
		))); err != nil {
			return err
		}
	}

	if opts.GitBranch.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Git branch",
			Help:    fmt.Sprintf("The default branch to use. Default value is '%s'.", DefaultBranch),
			Default: DefaultBranch,
		}, &opts.GitBranch.Value, survey.WithValidator(survey.MaxLength(200))); err != nil {
			return err
		}
	}

	if opts.GitBasePath.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Git repository base path",
			Help:    fmt.Sprintf("The path in the repository where Platform Hub settings are stored. Default value is '%s'.", DefaultBasePath),
			Default: DefaultBasePath,
		}, &opts.GitBasePath.Value, survey.WithValidator(survey.MaxLength(200))); err != nil {
			return err
		}
	}

	return nil
}

// resolveGitHubTarget resolves a GitHub connection - from a flag where given, otherwise via a
// required selection - normalizing it to its Id. Resolves a repository and infers the Git URL and default branch directly
// from it unless --git-url is already provided. Used for both interactive prompting and automation mode as the connections
// need to be loaded to ensure the specified connection and repository exist.
func resolveGitHubTarget(opts *ConfigureOptions) error {
	connections, err := opts.GetAllConnectionsCallback()
	if err != nil {
		return err
	}

	if len(connections) == 0 {
		fmt.Fprintf(opts.Out, "No Platform Hub GitHub connections found. Configure one at %s/app#/platform-hub/git-connections\n", opts.Host)
		return ErrNoGitHubConnections
	}

	connection, err := resolveOrSelectConnection(opts, connections)
	if err != nil {
		return err
	}
	opts.GithubConnection.Value = connection.ID

	if opts.GitUrl.Value != "" {
		return nil
	}

	repositories, err := opts.GetRepositoriesCallback(connection.ID)
	if err != nil {
		return err
	}
	if len(repositories) == 0 {
		return fmt.Errorf("the selected GitHub connection has no accessible repositories")
	}

	repository, err := resolveOrSelectRepository(opts, repositories)
	if err != nil {
		return err
	}

	opts.GithubRepository.Value = repository.RepositoryName
	opts.GitUrl.Value = repository.GitURL
	opts.GitBranch.Value = repository.DefaultBranch

	return nil
}

// resolveOrSelectConnection matches --github-connection against connection Id first, then
// against the connection's GitHub account login, since a connection has no name of its own.
// There's no easy server-side filter for the account login, so it checks the full in-memory list.
func resolveOrSelectConnection(opts *ConfigureOptions, connections []*githubconnections.Connection) (*githubconnections.Connection, error) {
	if opts.GithubConnection.Value != "" {
		for _, c := range connections {
			if strings.EqualFold(c.ID, opts.GithubConnection.Value) {
				return c, nil
			}
			if c.Installation != nil && strings.EqualFold(c.Installation.AccountLogin, opts.GithubConnection.Value) {
				return c, nil
			}
		}
		return nil, fmt.Errorf("could not find a Platform Hub GitHub connection matching '%s'", opts.GithubConnection.Value)
	}

	return question.SelectMap(opts.Ask, "Select a GitHub connection", connections, describeConnection)
}

// resolveOrSelectRepository matches --github-repository against a repository name.
func resolveOrSelectRepository(opts *ConfigureOptions, repositories []*githubconnections.Repository) (*githubconnections.Repository, error) {
	if opts.GithubRepository.Value != "" {
		for _, r := range repositories {
			if strings.EqualFold(r.RepositoryName, opts.GithubRepository.Value) {
				return r, nil
			}
		}
		return nil, fmt.Errorf("could not find a repository matching '%s' for the selected GitHub connection", opts.GithubRepository.Value)
	}

	return question.SelectMap(opts.Ask, "Select a repository", repositories, func(r *githubconnections.Repository) string {
		return r.RepositoryName
	})
}

func promptGitCredential(opts *ConfigureOptions) error {
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

func getConnectionTypeOptions() []*selectors.SelectOption[string] {
	return []*selectors.SelectOption[string]{
		{Display: connectionTypeGitHub, Value: connectionTypeGitHub},
		{Display: connectionTypeCredential, Value: connectionTypeCredential},
	}
}

func describeConnection(c *githubconnections.Connection) string {
	login := "unknown"
	accountType := ""
	if c.Installation != nil {
		login = c.Installation.AccountLogin
		accountType = c.Installation.AccountType
	}
	if accountType != "" {
		return fmt.Sprintf("%s (%s) - %s", login, accountType, c.Status)
	}
	return fmt.Sprintf("%s - %s", login, c.Status)
}

func (co *ConfigureOptions) Commit() error {
	co.GitBasePath.Value = check(co.GitBasePath.Value, DefaultBasePath)
	co.GitBranch.Value = check(co.GitBranch.Value, DefaultBranch)

	creds, err := co.buildCredentials()
	if err != nil {
		return err
	}

	resource := platformhubversioncontrolsettings.NewResource(co.GitUrl.Value, creds, co.GitBranch.Value, co.GitBasePath.Value)

	if _, err := platformhubversioncontrolsettings.Update(co.Client, resource); err != nil {
		return err
	}

	_, err = fmt.Fprintln(co.Out, "Successfully configured Platform Hub version control.")
	return err
}

func check(value string, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func (co *ConfigureOptions) buildCredentials() (credentials.GitCredential, error) {
	if co.GithubConnection.Value != "" {
		return credentials.NewGitHubApp(co.GithubConnection.Value), nil
	}

	if co.GitUsername.Value == "" || co.GitPassword.Value == "" {
		return nil, fmt.Errorf("either --%s or both --%s and --%s must be specified", FlagGithubConnection, FlagGitUsername, FlagGitPassword)
	}

	return credentials.NewUsernamePassword(co.GitUsername.Value, core.NewSensitiveValue(co.GitPassword.Value)), nil
}

func (co *ConfigureOptions) GenerateAutomationCmd() {
	var autoCmd string
	switch {
	case co.GithubRepository.Value != "":
		autoCmd = flag.GenerateAutomationCmd(co.CmdPath, "", co.GithubConnection, co.GithubRepository, co.GitBasePath)
	case co.GithubConnection.Value != "":
		// the --github-connection + --git-url shortcut: no repository was selected
		autoCmd = flag.GenerateAutomationCmd(co.CmdPath, "", co.GithubConnection, co.GitUrl, co.GitBranch, co.GitBasePath)
	default:
		autoCmd = flag.GenerateAutomationCmd(co.CmdPath, "", co.GitUrl, co.GitBranch, co.GitBasePath, co.GitUsername, co.GitPassword)
	}
	fmt.Fprintf(co.Out, "%s\n", autoCmd)
}
