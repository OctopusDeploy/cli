package convert_test

import (
	"github.com/OctopusDeploy/cli/pkg/cmd"
	projectConvert "github.com/OctopusDeploy/cli/pkg/cmd/project/convert"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPromptForConfigAsCode_UsingCacWithProjectStorage(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select where to store the Git credentials", "", []string{"Library", "Project"}, "Project"),
		testutil.NewInputPrompt("Git username", "The Git username.", "user1"),
		testutil.NewPasswordPrompt("Git password", "The Git password.", "password"),
		testutil.NewInputPrompt("Git URL", "The URL of the Git repository to store configuration.", "https://github.com/blah.git"),
		testutil.NewInputPromptWithDefault("Git repository base path", "The path in the repository where Config As Code settings are stored. Default value is '.octopus/'.", ".octopus/", "./octopus/project"),
		testutil.NewInputPromptWithDefault("Git branch", "The default branch to use. Default value is 'main'.", "main", "main"),
		testutil.NewConfirmPromptWithDefault("Is the 'main' branch protected?", "If the default branch is protected, you may not have permission to push to it.", false, false),
		testutil.NewInputPrompt("Enter a protected branch pattern (enter blank to end)", "This setting only applies within Octopus and will not affect your protected branches in Git. Use wildcard syntax to specify the range of branches to include. Multiple patterns can be supplied", "test"),
		testutil.NewInputPrompt("Enter a protected branch pattern (enter blank to end)", "This setting only applies within Octopus and will not affect your protected branches in Git. Use wildcard syntax to specify the range of branches to include. Multiple patterns can be supplied", ""),
		testutil.NewInputPromptWithDefault("Initial Git commit message", "The commit message used in initializing. Default value is 'Initial commit of deployment process'.", "Initial commit of deployment process", "init message"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	flags := projectConvert.NewConvertFlags()
	opts := projectConvert.NewConvertOptions(flags, &cmd.Dependencies{Ask: asker})
	dependableCmd, err := projectConvert.PromptForConfigAsCode(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.NotNil(t, dependableCmd)
	assert.Equal(t, "project", opts.GitStorage.Value)
	assert.Equal(t, "https://github.com/blah.git", opts.GitUrl.Value)
	assert.Equal(t, "./octopus/project", opts.GitBasePath.Value)
	assert.Equal(t, "main", opts.GitBranch.Value)
	assert.False(t, opts.GitDefaultBranchProtected.Value)
	assert.Equal(t, "init message", opts.GitInitialCommitMessage.Value)
	assert.Equal(t, []string{"test"}, opts.GitProtectedBranchPatterns.Value)
	assert.Equal(t, "user1", opts.GitUsername.Value)
	assert.Equal(t, "password", opts.GitPassword.Value)
}

func TestPromptForConfigAsCode_UsingCacWithLibraryStorage(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select where to store the Git credentials", "", []string{"Library", "Project"}, "Library"),
		testutil.NewSelectPrompt("Select which Git credentials to use", "", []string{"Git Creds 1", "Git Creds 2"}, "Git Creds 2"),
		testutil.NewInputPrompt("Git URL", "The URL of the Git repository to store configuration.", "https://github.com/blah.git"),
		testutil.NewInputPromptWithDefault("Git repository base path", "The path in the repository where Config As Code settings are stored. Default value is '.octopus/'.", ".octopus/", "./octopus/project"),
		testutil.NewInputPromptWithDefault("Git branch", "The default branch to use. Default value is 'main'.", "main", "main"),
		testutil.NewConfirmPromptWithDefault("Is the 'main' branch protected?", "If the default branch is protected, you may not have permission to push to it.", false, false),
		testutil.NewInputPrompt("Enter a protected branch pattern (enter blank to end)", "This setting only applies within Octopus and will not affect your protected branches in Git. Use wildcard syntax to specify the range of branches to include. Multiple patterns can be supplied", ""),
		testutil.NewInputPromptWithDefault("Initial Git commit message", "The commit message used in initializing. Default value is 'Initial commit of deployment process'.", "Initial commit of deployment process", "init message"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	gitCredsCallbackWasCalled := false
	getGitCredentials := func() ([]*credentials.Resource, error) {
		gitCredsCallbackWasCalled = true
		creds := credentials.NewResource("Git Creds 1", credentials.NewReference("gitcreds-1"))
		creds.ID = "gitcreds-1"
		creds2 := credentials.NewResource("Git Creds 2", credentials.NewReference("gitcreds-2"))
		creds2.ID = "gitcreds-2"
		return []*credentials.Resource{creds, creds2}, nil
	}
	flags := projectConvert.NewConvertFlags()
	opts := projectConvert.NewConvertOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetAllGitCredentialsCallback = getGitCredentials

	dependableCmd, err := projectConvert.PromptForConfigAsCode(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.NotNil(t, dependableCmd)
	assert.Equal(t, "library", opts.GitStorage.Value)
	assert.Equal(t, "https://github.com/blah.git", opts.GitUrl.Value)
	assert.Equal(t, "./octopus/project", opts.GitBasePath.Value)
	assert.Equal(t, "main", opts.GitBranch.Value)
	assert.Equal(t, "init message", opts.GitInitialCommitMessage.Value)
	assert.Equal(t, "Git Creds 2", opts.GitCredentials.Value)
	assert.True(t, gitCredsCallbackWasCalled)
	assert.Empty(t, opts.GitUsername.Value)
	assert.Empty(t, opts.GitPassword.Value)
}

func TestPromptForConfigAsCode_UsingCacWithBranchProtection(t *testing.T) {
	pa := []*testutil.PA{
		testutil.NewSelectPrompt("Select where to store the Git credentials", "", []string{"Library", "Project"}, "Library"),
		testutil.NewSelectPrompt("Select which Git credentials to use", "", []string{"Git Creds 1", "Git Creds 2"}, "Git Creds 2"),
		testutil.NewInputPrompt("Git URL", "The URL of the Git repository to store configuration.", "https://github.com/blah.git"),
		testutil.NewInputPromptWithDefault("Git repository base path", "The path in the repository where Config As Code settings are stored. Default value is '.octopus/'.", ".octopus/", "./octopus/project"),
		testutil.NewInputPromptWithDefault("Git branch", "The default branch to use. Default value is 'main'.", "main", "main"),
		testutil.NewConfirmPromptWithDefault("Is the 'main' branch protected?", "If the default branch is protected, you may not have permission to push to it.", true, false),
		testutil.NewInputPrompt("Initial commit branch name", "The branch where the Config As Code settings will be initially committed", "initial-branch"),
		testutil.NewInputPrompt("Enter a protected branch pattern (enter blank to end)", "This setting only applies within Octopus and will not affect your protected branches in Git. Use wildcard syntax to specify the range of branches to include. Multiple patterns can be supplied", "test"),
		testutil.NewInputPrompt("Enter a protected branch pattern (enter blank to end)", "This setting only applies within Octopus and will not affect your protected branches in Git. Use wildcard syntax to specify the range of branches to include. Multiple patterns can be supplied", ""),
		testutil.NewInputPromptWithDefault("Initial Git commit message", "The commit message used in initializing. Default value is 'Initial commit of deployment process'.", "Initial commit of deployment process", "init message"),
	}

	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	gitCredsCallbackWasCalled := false
	getGitCredentials := func() ([]*credentials.Resource, error) {
		gitCredsCallbackWasCalled = true
		creds := credentials.NewResource("Git Creds 1", credentials.NewReference("gitcreds-1"))
		creds.ID = "gitcreds-1"
		creds2 := credentials.NewResource("Git Creds 2", credentials.NewReference("gitcreds-2"))
		creds2.ID = "gitcreds-2"
		return []*credentials.Resource{creds, creds2}, nil
	}
	flags := projectConvert.NewConvertFlags()

	opts := projectConvert.NewConvertOptions(flags, &cmd.Dependencies{Ask: asker})
	opts.GetAllGitCredentialsCallback = getGitCredentials
	dependableCmd, err := projectConvert.PromptForConfigAsCode(opts)
	checkRemainingPrompts()
	assert.NoError(t, err)
	assert.NotNil(t, dependableCmd)
	assert.Equal(t, "library", opts.GitStorage.Value)
	assert.Equal(t, "https://github.com/blah.git", opts.GitUrl.Value)
	assert.Equal(t, "./octopus/project", opts.GitBasePath.Value)
	assert.Equal(t, "main", opts.GitBranch.Value)
	assert.Equal(t, "init message", opts.GitInitialCommitMessage.Value)
	assert.Equal(t, "Git Creds 2", opts.GitCredentials.Value)
	assert.True(t, opts.GitDefaultBranchProtected.Value)
	assert.Equal(t, "initial-branch", opts.GitInitialCommitBranch.Value)

	assert.True(t, gitCredsCallbackWasCalled)
	assert.Empty(t, opts.GitUsername.Value)
	assert.Empty(t, opts.GitPassword.Value)
}
