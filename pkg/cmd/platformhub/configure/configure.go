package configure

import (
	"errors"
	"fmt"
	"math"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
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
	FlagGitConnection = "git-connection"
	FlagGitUrl        = "git-url"
	FlagGitUsername   = "git-username"
	FlagGitPassword   = "git-password"
	FlagGitBasePath   = "git-base-path"
	FlagGitBranch     = "git-branch"

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
	GitConnection *flag.Flag[string]
	GitUrl        *flag.Flag[string]
	GitUsername   *flag.Flag[string]
	GitPassword   *flag.Flag[string]
	GitBasePath   *flag.Flag[string]
	GitBranch     *flag.Flag[string]
}

func NewConfigureFlags() *ConfigureFlags {
	return &ConfigureFlags{
		GitConnection: flag.New[string](FlagGitConnection, false),
		GitUrl:        flag.New[string](FlagGitUrl, false),
		GitUsername:   flag.New[string](FlagGitUsername, false),
		GitPassword:   flag.New[string](FlagGitPassword, true),
		GitBasePath:   flag.New[string](FlagGitBasePath, false),
		GitBranch:     flag.New[string](FlagGitBranch, false),
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
			%[1]s platform-hub configure --git-connection Connections-1 --git-url https://github.com/org/repo.git --git-branch main
			%[1]s platform-hub configure --git-username user --git-password pass --git-url https://github.com/org/repo.git
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewConfigureOptions(configureFlags, cmd.NewSystemDependencies(f, c))
			return configureRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&configureFlags.GitConnection.Value, configureFlags.GitConnection.Name, "", "The Id of the Platform Hub GitHub connection to use")
	flags.StringVar(&configureFlags.GitUrl.Value, configureFlags.GitUrl.Name, "", "Url of the Git repository for storing Platform Hub configuration")
	flags.StringVar(&configureFlags.GitUsername.Value, configureFlags.GitUsername.Name, "", "The username to authenticate with Git")
	flags.StringVar(&configureFlags.GitPassword.Value, configureFlags.GitPassword.Name, "", "The password to authenticate with Git")
	flags.StringVar(&configureFlags.GitBasePath.Value, configureFlags.GitBasePath.Name, "", fmt.Sprintf("The directory where Octopus should store Platform Hub files in the repository. Default is '%s'", DefaultBasePath))
	flags.StringVar(&configureFlags.GitBranch.Value, configureFlags.GitBranch.Name, "", fmt.Sprintf("The default branch to use. Default is '%s'.", DefaultBranch))

	return cmd
}

func configureRun(opts *ConfigureOptions) error {
	current, err := opts.GetSettingsCallback()
	if err != nil {
		return err
	}

	if !opts.NoPrompt {
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

func PromptMissing(opts *ConfigureOptions) error {
	var selectedRepo *githubconnections.Repository
	useGitHub := opts.GitConnection.Value != ""

	if !useGitHub && opts.GitUsername.Value == "" && opts.GitPassword.Value == "" {
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
		repo, err := promptGitHubConnection(opts)
		if err != nil {
			return err
		}
		selectedRepo = repo
	} else if opts.GitConnection.Value == "" {
		if err := promptGitCredential(opts); err != nil {
			return err
		}
	}

	if opts.GitUrl.Value == "" {
		defaultUrl := ""
		if selectedRepo != nil {
			defaultUrl = selectedRepo.GitURL
		}
		if err := opts.Ask(&survey.Input{
			Message: "Git URL",
			Help:    "The URL of the Git repository to store Platform Hub configuration.",
			Default: defaultUrl,
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
			Help:    fmt.Sprintf("The path in the repository where Platform Hub settings are stored. Default value is '%s'.", DefaultBasePath),
			Default: DefaultBasePath,
		}, &opts.GitBasePath.Value, survey.WithValidator(survey.MaxLength(200))); err != nil {
			return err
		}
	}

	if opts.GitBranch.Value == "" {
		defaultBranch := DefaultBranch
		if selectedRepo != nil && selectedRepo.DefaultBranch != "" {
			defaultBranch = selectedRepo.DefaultBranch
		}
		if err := opts.Ask(&survey.Input{
			Message: "Git branch",
			Help:    fmt.Sprintf("The default branch to use. Default value is '%s'.", defaultBranch),
			Default: defaultBranch,
		}, &opts.GitBranch.Value, survey.WithValidator(survey.MaxLength(200))); err != nil {
			return err
		}
	}

	return nil
}

func promptGitHubConnection(opts *ConfigureOptions) (*githubconnections.Repository, error) {
	if opts.GitConnection.Value != "" {
		return nil, nil
	}

	connections, err := opts.GetAllConnectionsCallback()
	if err != nil {
		return nil, err
	}

	if len(connections) == 0 {
		fmt.Fprintf(opts.Out, "No Platform Hub GitHub connections found. Configure one at %s/app#/platform-hub/git-connections\n", opts.Host)
		return nil, ErrNoGitHubConnections
	}

	connection, err := selectors.Select(opts.Ask, "Select a GitHub connection", func() ([]*githubconnections.Connection, error) { return connections, nil }, describeConnection)
	if err != nil {
		return nil, err
	}
	opts.GitConnection.Value = connection.ID

	repositories, err := opts.GetRepositoriesCallback(connection.ID)
	if err != nil {
		return nil, err
	}
	if len(repositories) == 0 {
		return nil, fmt.Errorf("the selected GitHub connection has no accessible repositories")
	}

	return selectors.Select(opts.Ask, "Select a repository", func() ([]*githubconnections.Repository, error) { return repositories, nil }, func(r *githubconnections.Repository) string {
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
	if co.GitConnection.Value != "" {
		return credentials.NewGitHubApp(co.GitConnection.Value), nil
	}

	if co.GitUsername.Value == "" || co.GitPassword.Value == "" {
		return nil, errors.New("either --git-connection or both --git-username and --git-password must be specified")
	}

	return credentials.NewUsernamePassword(co.GitUsername.Value, core.NewSensitiveValue(co.GitPassword.Value)), nil
}

func (co *ConfigureOptions) GenerateAutomationCmd() {
	autoCmd := flag.GenerateAutomationCmd(co.CmdPath, "", co.GitConnection, co.GitUrl, co.GitBranch, co.GitBasePath, co.GitUsername, co.GitPassword)
	fmt.Fprintf(co.Out, "%s\n", autoCmd)
}
