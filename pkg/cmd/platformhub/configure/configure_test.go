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
		testutil.NewInputPromptWithDefault("Git URL", "The URL of the Git repository to store Platform Hub configuration.", "https://github.com/octo-org/repo-two.git", "https://github.com/octo-org/repo-two.git"),
		testutil.NewInputPromptWithDefault("Git repository base path", "The path in the repository where Platform Hub settings are stored. Default value is '.octopus/'.", ".octopus/", ".octopus/"),
		testutil.NewInputPromptWithDefault("Git branch", "The default branch to use. Default value is 'develop'.", "develop", "develop"),
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
	assert.Equal(t, "Connections-1", opts.GitConnection.Value)
	assert.Equal(t, "https://github.com/octo-org/repo-two.git", opts.GitUrl.Value)
	assert.Equal(t, ".octopus/", opts.GitBasePath.Value)
	assert.Equal(t, "develop", opts.GitBranch.Value)
}

func TestPromptMissing_GitHubConnection_SingleConnectionSkipsSelection(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select a Git connection type", "", []string{"GitHub Connection", "Git Credential"}, "GitHub Connection"),
		// only one connection and one repository, so no select prompts are shown for either
		testutil.NewInputPromptWithDefault("Git URL", "The URL of the Git repository to store Platform Hub configuration.", "https://github.com/octo-org/repo-one.git", "https://github.com/octo-org/repo-one.git"),
		testutil.NewInputPromptWithDefault("Git repository base path", "The path in the repository where Platform Hub settings are stored. Default value is '.octopus/'.", ".octopus/", ".octopus/"),
		testutil.NewInputPromptWithDefault("Git branch", "The default branch to use. Default value is 'main'.", "main", "main"),
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
	assert.Equal(t, "Connections-1", opts.GitConnection.Value)
	assert.Equal(t, "https://github.com/octo-org/repo-one.git", opts.GitUrl.Value)
}

func TestPromptMissing_GitHubConnection_NoneConfiguredStopsCleanly(t *testing.T) {
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
		testutil.NewInputPromptWithDefault("Git URL", "The URL of the Git repository to store Platform Hub configuration.", "", "https://github.com/blah.git"),
		testutil.NewInputPromptWithDefault("Git repository base path", "The path in the repository where Platform Hub settings are stored. Default value is '.octopus/'.", ".octopus/", ".octopus/"),
		testutil.NewInputPromptWithDefault("Git branch", "The default branch to use. Default value is 'main'.", "main", "main"),
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
}

func TestPromptMissing_CanUseGitHubAppFalse_SkipsConnectionTypePrompt(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewInputPrompt("Git username", "The Git username.", "user1"),
		testutil.NewPasswordPrompt("Git password", "The Git password.", "password1"),
		testutil.NewInputPromptWithDefault("Git URL", "The URL of the Git repository to store Platform Hub configuration.", "", "https://github.com/blah.git"),
		testutil.NewInputPromptWithDefault("Git repository base path", "The path in the repository where Platform Hub settings are stored. Default value is '.octopus/'.", ".octopus/", ".octopus/"),
		testutil.NewInputPromptWithDefault("Git branch", "The default branch to use. Default value is 'main'.", "main", "main"),
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

func TestPromptMissing_GitConnectionFlagPreset_SkipsConnectionAndRepoPrompts(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewInputPromptWithDefault("Git URL", "The URL of the Git repository to store Platform Hub configuration.", "", "https://github.com/octo-org/repo.git"),
		testutil.NewInputPromptWithDefault("Git repository base path", "The path in the repository where Platform Hub settings are stored. Default value is '.octopus/'.", ".octopus/", ".octopus/"),
		testutil.NewInputPromptWithDefault("Git branch", "The default branch to use. Default value is 'main'.", "main", "main"),
	}

	opts, checkRemainingPrompts := newOptions(t, pa)
	opts.GitConnection.Value = "Connections-1"

	err := configure.PromptMissing(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.Equal(t, "https://github.com/octo-org/repo.git", opts.GitUrl.Value)
}
