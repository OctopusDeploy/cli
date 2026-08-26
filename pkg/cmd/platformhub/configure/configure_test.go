package configure_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/platformhub/configure"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/githubconnections"
	"github.com/stretchr/testify/assert"
)

func newConnection(id, accountLogin, accountType string, status githubconnections.ConnectionStatus) *githubconnections.Connection {
	return &githubconnections.Connection{
		ID:     id,
		Status: status,
		Installation: &githubconnections.Installation{
			AccountLogin: accountLogin,
			AccountType:  accountType,
		},
	}
}

func newOptions(t *testing.T, pa []*testutil.PA) (*configure.ConfigureOptions, testutil.CheckRemaining) {
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)
	flags := configure.NewConfigureFlags()
	opts := &configure.ConfigureOptions{
		ConfigureFlags: flags,
		Dependencies:   &cmd.Dependencies{Ask: asker, Out: &bytes.Buffer{}, Host: "https://octopus.example.com"},
	}
	return opts, checkRemainingPrompts
}

func TestPromptMissing_GitHubConnection_MultipleConnectionsAndRepos(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select a Git connection type", "", []string{"GitHub Connection", "Git Credential"}, "GitHub Connection"),
		testutil.NewSelectPrompt("Select a GitHub connection", "", []string{"octo-org (Organization) - Connected", "octo-user (User) - Connected"}, "octo-org (Organization) - Connected"),
		testutil.NewSelectPrompt("Select a repository", "", []string{"repo-one", "repo-two"}, "repo-two"),
		testutil.NewInputPromptWithDefault("Git repository base path", "The path in the repository where Platform Hub settings are stored. Default value is '.octopus/'.", ".octopus/", ".octopus/"),
	}

	opts, checkRemainingPrompts := newOptions(t, pa)

	opts.GetGitHubAppSettingsCallback = func() (*githubconnections.Settings, error) {
		return &githubconnections.Settings{CanUseGitHubApp: true}, nil
	}
	orgConnection := newConnection("Connections-1", "octo-org", "Organization", githubconnections.ConnectionStatusConnected)
	userConnection := newConnection("Connections-2", "octo-user", "User", githubconnections.ConnectionStatusConnected)
	opts.GetAllConnectionsCallback = func() ([]*githubconnections.Connection, error) {
		return []*githubconnections.Connection{orgConnection, userConnection}, nil
	}
	opts.GetRepositoriesCallback = func(connectionID string) ([]*githubconnections.Repository, error) {
		assert.Equal(t, "Connections-1", connectionID)
		return []*githubconnections.Repository{
			{RepositoryName: "repo-one", GitURL: "https://github.com/octo-org/repo-one.git", DefaultBranch: "main"},
			{RepositoryName: "repo-two", GitURL: "https://github.com/octo-org/repo-two.git", DefaultBranch: "develop"},
		}, nil
	}

	err := configure.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "Connections-1", opts.GithubConnection.Value)
	assert.Equal(t, "repo-two", opts.GithubRepository.Value)
	// Url and branch are inferred from the selected repository, never prompted for separately.
	assert.Equal(t, "https://github.com/octo-org/repo-two.git", opts.GitUrl.Value)
	assert.Equal(t, "develop", opts.GitBranch.Value)
	assert.Equal(t, ".octopus/", opts.GitBasePath.Value)
}

func TestPromptMissing_GitHubConnection_SingleConnectionAndRepoStillRequireSelection(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select a Git connection type", "", []string{"GitHub Connection", "Git Credential"}, "GitHub Connection"),
		// a single connection or repository should still require selection instead of auto-picking it
		testutil.NewSelectPrompt("Select a GitHub connection", "", []string{"octo-org (Organization) - Connected"}, "octo-org (Organization) - Connected"),
		testutil.NewSelectPrompt("Select a repository", "", []string{"repo-one"}, "repo-one"),
		testutil.NewInputPromptWithDefault("Git repository base path", "The path in the repository where Platform Hub settings are stored. Default value is '.octopus/'.", ".octopus/", ".octopus/"),
	}

	opts, checkRemainingPrompts := newOptions(t, pa)

	opts.GetGitHubAppSettingsCallback = func() (*githubconnections.Settings, error) {
		return &githubconnections.Settings{CanUseGitHubApp: true}, nil
	}
	opts.GetAllConnectionsCallback = func() ([]*githubconnections.Connection, error) {
		return []*githubconnections.Connection{newConnection("Connections-1", "octo-org", "Organization", githubconnections.ConnectionStatusConnected)}, nil
	}
	opts.GetRepositoriesCallback = func(connectionID string) ([]*githubconnections.Repository, error) {
		return []*githubconnections.Repository{
			{RepositoryName: "repo-one", GitURL: "https://github.com/octo-org/repo-one.git", DefaultBranch: "main"},
		}, nil
	}

	err := configure.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "Connections-1", opts.GithubConnection.Value)
	assert.Equal(t, "repo-one", opts.GithubRepository.Value)
	assert.Equal(t, "https://github.com/octo-org/repo-one.git", opts.GitUrl.Value)
	assert.Equal(t, "main", opts.GitBranch.Value)
}

func TestPromptMissing_GitHubConnection_NoneConfiguredStopsPromptingWithoutErrors(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select a Git connection type", "", []string{"GitHub Connection", "Git Credential"}, "GitHub Connection"),
	}

	opts, checkRemainingPrompts := newOptions(t, pa)

	opts.GetGitHubAppSettingsCallback = func() (*githubconnections.Settings, error) {
		return &githubconnections.Settings{CanUseGitHubApp: true}, nil
	}
	opts.GetAllConnectionsCallback = func() ([]*githubconnections.Connection, error) {
		return nil, nil
	}

	err := configure.PromptMissing(opts)
	checkRemainingPrompts()
	assert.True(t, errors.Is(err, configure.ErrNoGitHubConnections))
}

func TestPromptMissing_GitCredential_WhenSelected(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select a Git connection type", "", []string{"GitHub Connection", "Git Credential"}, "Git Credential"),
		testutil.NewInputPrompt("Git username", "The Git username.", "user1"),
		testutil.NewPasswordPrompt("Git password", "The Git password.", "password1"),
		testutil.NewInputPrompt("Git URL", "The URL of the Git repository to store Platform Hub configuration.", "https://github.com/blah.git"),
		testutil.NewInputPromptWithDefault("Git branch", "The default branch to use. Default value is 'main'.", "main", "main"),
		testutil.NewInputPromptWithDefault("Git repository base path", "The path in the repository where Platform Hub settings are stored. Default value is '.octopus/'.", ".octopus/", ".octopus/"),
	}

	opts, checkRemainingPrompts := newOptions(t, pa)
	opts.GetGitHubAppSettingsCallback = func() (*githubconnections.Settings, error) {
		return &githubconnections.Settings{CanUseGitHubApp: true}, nil
	}

	err := configure.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "user1", opts.GitUsername.Value)
	assert.Equal(t, "password1", opts.GitPassword.Value)
	assert.Equal(t, "https://github.com/blah.git", opts.GitUrl.Value)
	assert.Equal(t, "main", opts.GitBranch.Value)
}

func TestPromptMissing_CanUseGitHubAppFalse_SkipsConnectionTypePrompt(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewInputPrompt("Git username", "The Git username.", "user1"),
		testutil.NewPasswordPrompt("Git password", "The Git password.", "password1"),
		testutil.NewInputPrompt("Git URL", "The URL of the Git repository to store Platform Hub configuration.", "https://github.com/blah.git"),
		testutil.NewInputPromptWithDefault("Git branch", "The default branch to use. Default value is 'main'.", "main", "main"),
		testutil.NewInputPromptWithDefault("Git repository base path", "The path in the repository where Platform Hub settings are stored. Default value is '.octopus/'.", ".octopus/", ".octopus/"),
	}

	opts, checkRemainingPrompts := newOptions(t, pa)
	opts.GetGitHubAppSettingsCallback = func() (*githubconnections.Settings, error) {
		return &githubconnections.Settings{CanUseGitHubApp: false}, nil
	}

	err := configure.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "user1", opts.GitUsername.Value)
}

func TestPromptMissing_WithGithubConnectionFlagSet_SkipsConnectionPromptButStillPromptsRepository(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select a repository", "", []string{"repo-one", "repo-two"}, "repo-two"),
		testutil.NewInputPromptWithDefault("Git repository base path", "The path in the repository where Platform Hub settings are stored. Default value is '.octopus/'.", ".octopus/", ".octopus/"),
	}

	opts, checkRemainingPrompts := newOptions(t, pa)
	opts.GithubConnection.Value = "Connections-1"
	opts.GetAllConnectionsCallback = func() ([]*githubconnections.Connection, error) {
		return []*githubconnections.Connection{newConnection("Connections-1", "octo-org", "Organization", githubconnections.ConnectionStatusConnected)}, nil
	}
	opts.GetRepositoriesCallback = func(connectionID string) ([]*githubconnections.Repository, error) {
		assert.Equal(t, "Connections-1", connectionID)
		return []*githubconnections.Repository{
			{RepositoryName: "repo-one", GitURL: "https://github.com/octo-org/repo-one.git", DefaultBranch: "main"},
			{RepositoryName: "repo-two", GitURL: "https://github.com/octo-org/repo-two.git", DefaultBranch: "develop"},
		}, nil
	}

	err := configure.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "repo-two", opts.GithubRepository.Value)
	assert.Equal(t, "https://github.com/octo-org/repo-two.git", opts.GitUrl.Value)
	assert.Equal(t, "develop", opts.GitBranch.Value)
}

func TestPromptMissing_WithGithubConnectionAndGitUrlSet_SkipsRepositorySelection(t *testing.T) {
	pa := []*testutil.PA{
		// no "Select a repository" prompt: the explicit --git-url opts out of repository selection
		testutil.NewInputPromptWithDefault("Git branch", "The default branch to use. Default value is 'main'.", "main", "develop"),
		testutil.NewInputPromptWithDefault("Git repository base path", "The path in the repository where Platform Hub settings are stored. Default value is '.octopus/'.", ".octopus/", ".octopus/"),
	}

	opts, checkRemainingPrompts := newOptions(t, pa)
	opts.GithubConnection.Value = "octo-org"
	opts.GitUrl.Value = "https://github.com/octo-org/repo.git"
	opts.GetAllConnectionsCallback = func() ([]*githubconnections.Connection, error) {
		return []*githubconnections.Connection{newConnection("Connections-1", "octo-org", "Organization", githubconnections.ConnectionStatusConnected)}, nil
	}

	err := configure.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "Connections-1", opts.GithubConnection.Value)
	assert.Equal(t, "", opts.GithubRepository.Value)
	assert.Equal(t, "https://github.com/octo-org/repo.git", opts.GitUrl.Value)
	assert.Equal(t, "develop", opts.GitBranch.Value)
}

func TestPromptMissing_GithubConnectionFlag_MatchesByAccountLoginName(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewInputPromptWithDefault("Git repository base path", "The path in the repository where Platform Hub settings are stored. Default value is '.octopus/'.", ".octopus/", ".octopus/"),
	}

	opts, checkRemainingPrompts := newOptions(t, pa)
	opts.GithubConnection.Value = "octo-org"
	opts.GithubRepository.Value = "repo-one"
	opts.GetAllConnectionsCallback = func() ([]*githubconnections.Connection, error) {
		return []*githubconnections.Connection{
			newConnection("Connections-1", "octo-org", "Organization", githubconnections.ConnectionStatusConnected),
			newConnection("Connections-2", "octo-user", "User", githubconnections.ConnectionStatusConnected),
		}, nil
	}
	opts.GetRepositoriesCallback = func(connectionID string) ([]*githubconnections.Repository, error) {
		assert.Equal(t, "Connections-1", connectionID)
		return []*githubconnections.Repository{
			{RepositoryName: "repo-one", GitURL: "https://github.com/octo-org/repo-one.git", DefaultBranch: "main"},
		}, nil
	}

	err := configure.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	// the account-login name is normalized to the connection Id once resolved
	assert.Equal(t, "Connections-1", opts.GithubConnection.Value)
	assert.Equal(t, "https://github.com/octo-org/repo-one.git", opts.GitUrl.Value)
}

func TestPromptMissing_GithubConnectionFlag_NoMatchReturnsError(t *testing.T) {
	opts, checkRemainingPrompts := newOptions(t, nil)
	opts.GithubConnection.Value = "does-not-exist"
	opts.GetAllConnectionsCallback = func() ([]*githubconnections.Connection, error) {
		return []*githubconnections.Connection{newConnection("Connections-1", "octo-org", "Organization", githubconnections.ConnectionStatusConnected)}, nil
	}

	err := configure.PromptMissing(opts)
	checkRemainingPrompts()
	assert.EqualError(t, err, "could not find a Platform Hub GitHub connection matching 'does-not-exist'")
}

func TestValidateFlags_GitHubAndCredentialFlagsTogetherIsRejected(t *testing.T) {
	flags := configure.NewConfigureFlags()
	flags.GithubConnection.Value = "octo-org"
	flags.GitUsername.Value = "user1"
	opts := &configure.ConfigureOptions{ConfigureFlags: flags}

	err := configure.ValidateFlags(opts)
	assert.EqualError(t, err, "GitHub connections do not accept Git credentials")
}

func TestValidateFlags_GitHubRepositoryAndCredentialPasswordTogetherIsRejected(t *testing.T) {
	flags := configure.NewConfigureFlags()
	flags.GithubRepository.Value = "my-repo"
	flags.GitPassword.Value = "password1"
	opts := &configure.ConfigureOptions{ConfigureFlags: flags}

	err := configure.ValidateFlags(opts)
	assert.EqualError(t, err, "GitHub connections do not accept Git credentials")
}

func TestValidateFlags_GitBranchWithGitHubRepositoryIsRejected(t *testing.T) {
	flags := configure.NewConfigureFlags()
	flags.GithubRepository.Value = "my-repo"
	flags.GitBranch.Value = "main"
	opts := &configure.ConfigureOptions{ConfigureFlags: flags}

	err := configure.ValidateFlags(opts)
	assert.EqualError(t, err, "--git-url and --git-branch cannot be specified together with --github-repository")
}

func TestValidateFlags_GitHubConnectionWithGitUrlIsAllowed(t *testing.T) {
	// no --github-repository, so the user is opting out of repository selection and providing
	// the Git URL directly
	flags := configure.NewConfigureFlags()
	flags.GithubConnection.Value = "octo-org"
	flags.GitUrl.Value = "https://github.com/octo-org/repo.git"
	flags.GitBranch.Value = "main"
	opts := &configure.ConfigureOptions{ConfigureFlags: flags}

	assert.NoError(t, configure.ValidateFlags(opts))
}

func TestValidateFlags_GitCredentialUrlAndBranch_Allowed(t *testing.T) {
	flags := configure.NewConfigureFlags()
	flags.GitUsername.Value = "user1"
	flags.GitPassword.Value = "password1"
	flags.GitUrl.Value = "https://github.com/octo-org/repo.git"
	flags.GitBranch.Value = "main"
	opts := &configure.ConfigureOptions{ConfigureFlags: flags}

	assert.NoError(t, configure.ValidateFlags(opts))
}

func TestValidateFlags_GitHubConnectionWithRepository_Allowed(t *testing.T) {
	flags := configure.NewConfigureFlags()
	flags.GithubConnection.Value = "octo-org"
	flags.GithubRepository.Value = "my-repo"
	opts := &configure.ConfigureOptions{ConfigureFlags: flags}

	assert.NoError(t, configure.ValidateFlags(opts))
}

func TestPromptMissing_GithubRepositoryFlag_NoMatchReturnsError(t *testing.T) {
	opts, checkRemainingPrompts := newOptions(t, nil)
	opts.GithubConnection.Value = "Connections-1"
	opts.GithubRepository.Value = "does-not-exist"
	opts.GetAllConnectionsCallback = func() ([]*githubconnections.Connection, error) {
		return []*githubconnections.Connection{newConnection("Connections-1", "octo-org", "Organization", githubconnections.ConnectionStatusConnected)}, nil
	}
	opts.GetRepositoriesCallback = func(connectionID string) ([]*githubconnections.Repository, error) {
		return []*githubconnections.Repository{{RepositoryName: "repo-one"}}, nil
	}

	err := configure.PromptMissing(opts)
	checkRemainingPrompts()
	assert.EqualError(t, err, "could not find a repository matching 'does-not-exist' for the selected GitHub connection")
}
