package create

import (
	"fmt"
	"io"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	projectGroupCreate "github.com/OctopusDeploy/cli/pkg/cmd/projectgroup/create"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/spf13/cobra"
	"net/url"
)

const (
	FlagGroup                   = "group"
	FlagName                    = "name"
	FlagDescription             = "description"
	FlagLifecycle               = "lifecycle"
	FlagConfigAsCode            = "cac"
	FlagGitUrl                  = "git-url"
	FlagGitBranch               = "git-branch"
	FlagGitCredential           = "git-credentials"
	FlagGitCredentialName       = "git-credentials-name"
	FlagGitUsername             = "git-username"
	FlagGitPassword             = "git-password"
	FlagGitAnonymous            = "git-anonymous"
	FlagGitCredentialStorage    = "git-cred-store"
	FlagGitInitialCommitMessage = "git-initial-commit"
	FlagGitBasePath             = "git-base-path"

	InitialGitCommitMessage = "Initial commit of deployment process"
	DefaultBasePath         = ".octopus/"
	DefaultBranch           = "main"
	GitPersistenceType      = "VersionControlled"
)

type CreateFlags struct {
	Group        *flag.Flag[string]
	Name         *flag.Flag[string]
	Description  *flag.Flag[string]
	Lifecycle    *flag.Flag[string]
	ConfigAsCode *flag.Flag[bool]
	GitUrl       *flag.Flag[string]
	GitBranch    *flag.Flag[string]

	//GitCredentials *flag.Flag[string]
	GitUsername *flag.Flag[string]
	GitPassword *flag.Flag[string]
	//GitAnonymous   *flag.Flag[bool]
	GitStorage              *flag.Flag[string]
	GitInitialCommitMessage *flag.Flag[string]
	GitBasePath             *flag.Flag[string]
}

type CreateOptions struct {
	*CreateFlags
	Out      io.Writer
	Client   *client.Client
	Host     string
	Space    *spaces.Space
	NoPrompt bool
	Ask      question.Asker
	CmdPath  string
}

func (co *CreateOptions) Commit() error {
	lifecycle, err := co.Client.Lifecycles.GetByIDOrName(co.Lifecycle.Value)
	if err != nil {
		return err
	}

	projectGroup, err := co.Client.ProjectGroups.GetByIDOrName(co.Group.Value)
	if err != nil {
		return err
	}

	project := projects.NewProject(co.Name.Value, lifecycle.ID, projectGroup.ID)
	project.Description = co.Description.Value

	createdProject, err := co.Client.Projects.Add(project)
	if err != nil {
		return err
	}

	if co.ConfigAsCode.Value {
		if co.GitStorage.Value == "project" {
			credentials := projects.NewUsernamePasswordGitCredential(co.GitUsername.Value, core.NewSensitiveValue(co.GitPassword.Value))
			url, err := url.Parse(co.GitUrl.Value)
			vcs := projects.NewVersionControlSettings(getBasePath(co), credentials, getGitBranch(co), GitPersistenceType, url)
			_, err = co.Client.Projects.ConvertToVcs(createdProject, getInitialCommitMessage(co), vcs)
			if err != nil {
				return err
			}
		} else {
			// TODO: create-git-credentials
		}
	}

	_, err = fmt.Fprintf(co.Out, "\nSuccessfully created project %s (%s), with lifecycle %s in project group %s.\n", createdProject.Name, createdProject.Slug, co.Lifecycle.Value, co.Group.Value)
	if err != nil {
		return err
	}

	link := output.Bluef("%s/app#/%s/projects/%s", co.Host, co.Space.GetID(), createdProject.GetID())
	fmt.Fprintf(co.Out, "View this project on Octopus Deploy: %s\n", link)

	return nil
}

func (co *CreateOptions) GenerateAutomationCmd() {
	if !co.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(co.CmdPath, co.Name, co.Description, co.Group, co.Lifecycle)
		fmt.Fprintf(co.Out, "%s\n", autoCmd)
	}
}

func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Group:        flag.New[string](FlagGroup, false),
		Name:         flag.New[string](FlagName, false),
		Description:  flag.New[string](FlagDescription, false),
		Lifecycle:    flag.New[string](FlagLifecycle, false),
		ConfigAsCode: flag.New[bool](FlagConfigAsCode, false),
		GitUrl:       flag.New[string](FlagGitUrl, false),
		GitBranch:    flag.New[string](FlagGitBranch, false),
		//GitCredentials: flag.New[string](FlagGitCredential, false),
		GitUsername: flag.New[string](FlagGitUsername, false),
		GitPassword: flag.New[string](FlagGitPassword, true),
		//GitAnonymous:   flag.New[bool](FlagGitAnonymous, false),
		GitStorage:              flag.New[string](FlagGitCredentialStorage, false),
		GitInitialCommitMessage: flag.New[string](FlagGitInitialCommitMessage, false),
		GitBasePath:             flag.New[string](FlagGitBasePath, false),
	}
}

func NewCmdCreate(f factory.Factory) *cobra.Command {
	opts := &CreateOptions{
		Ask:         f.Ask,
		CreateFlags: NewCreateFlags(),
	}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a new project in Octopus Deploy",
		Long:  "Creates a new project in Octopus Deploy.",
		Example: heredoc.Doc(`
			$ octopus project create .... fill this in later
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSpacedClient()
			if err != nil {
				return err
			}
			opts.CmdPath = cmd.CommandPath()
			opts.Out = cmd.OutOrStdout()
			opts.Client = client
			opts.Host = f.GetCurrentHost()
			opts.NoPrompt = !f.IsPromptEnabled()
			opts.Space = f.GetCurrentSpace()

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Name.Value, opts.Name.Name, "n", "", "Name of the project")
	flags.StringVarP(&opts.Description.Value, opts.Description.Name, "d", "", "Description of the project")
	flags.StringVarP(&opts.Group.Value, opts.Group.Name, "g", "", "Project group of the project")
	flags.StringVarP(&opts.Lifecycle.Value, opts.Lifecycle.Name, "l", "", "Lifecycle of the project")
	flags.BoolVar(&opts.ConfigAsCode.Value, opts.ConfigAsCode.Name, false, "Use Config As Code for the project")

	flags.StringVarP(&opts.GitUrl.Value, opts.GitUrl.Name, "u", "", "Url of the Git repository for storing project configuration")
	flags.StringVarP(&opts.GitBranch.Value, opts.GitBranch.Name, "b", "", "The default branch to use for Config As Code, default is main.")
	//flags.StringVarP(&opts.GitCredentials.Value, opts..GitCredentials.Name, "c", "", "The Id or name of the Git credentials stored in Octopus")
	flags.StringVar(&opts.GitUsername.Value, opts.GitUsername.Name, "", "The username to authenticate with Git")
	flags.StringVar(&opts.GitPassword.Value, opts.GitPassword.Name, "", "The password to authenticate with Git")
	//flags.BoolVarP(&opts.GitAnonymous.Value, opts.GitAnonymous.Name, "", false, "Use anonymous authentication for accessing the Git repo.")
	flags.StringVar(&opts.GitStorage.Value, opts.GitStorage.Name, "", "The location to store the supplied Git credentials. Options are library or project. Default is library")
	flags.StringVar(&opts.GitInitialCommitMessage.Value, opts.GitInitialCommitMessage.Name, InitialGitCommitMessage, "The initial commit message for configuring Config As Code.")
	flags.SortFlags = false

	return cmd
}

func createRun(opts *CreateOptions) error {
	if !opts.NoPrompt {
		optsArray, err := PromptMissing(opts)
		if err != nil {
			return err
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
	}

	return nil
}

func PromptMissing(opts *CreateOptions) ([]cmd.NestedOpts, error) {
	nestedOpts := []cmd.NestedOpts{}

	if opts.Name.Value == "" {
		if err := opts.Ask(&survey.Input{
			Message: "Name",
			Help:    "A short, memorable, unique name for this project.",
		}, &opts.Name.Value, survey.WithValidator(survey.ComposeValidators(
			survey.MaxLength(200),
			survey.MinLength(1),
			survey.Required,
		))); err != nil {
			return nil, err
		}
	}

	if opts.Lifecycle.Value == "" {
		lc, err := selectors.Lifecycle("You have not specified a Lifecycle for this project. Please select one:", opts.Client, opts.Ask)
		if err != nil {
			return nil, err
		}
		opts.Lifecycle.Value = lc.Name
	}

	if opts.Group.Value == "" {
		var shouldCreateNewProjectGroup bool
		opts.Ask(&survey.Confirm{
			Message: "Would you like to create a new Project Group?",
			Default: false,
		}, &shouldCreateNewProjectGroup)

		if shouldCreateNewProjectGroup {
			optValues := projectGroupCreate.NewCreateFlags()
			projectGroupCreateOpts := projectGroupCreate.CreateOptions{
				Host:              opts.Host,
				Ask:               opts.Ask,
				Out:               opts.Out,
				CreateFlags:       optValues,
				Client:            opts.Client,
				Space:             opts.Space,
				NoPrompt:          opts.NoPrompt,
				CmdPath:           "octopus project-group create",
				ShowMessagePrefix: true,
			}
			projectGroupCreate.PromptMissing(&projectGroupCreateOpts)
			opts.Group.Value = projectGroupCreateOpts.Name.Value
			nestedOpts = append(nestedOpts, &projectGroupCreateOpts)
		} else {
			g, err := selectors.ProjectGroup("You have not specified a Project group for this project. Please select one:", opts.Client, opts.Ask)
			if err != nil {
				return nil, err
			}
			opts.Group.Value = g.Name
		}

	}

	err := PromptForConfigAsCode(opts)
	if err != nil {
		return nil, err
	}

	nestedOpts = append(nestedOpts, opts)
	return nestedOpts, nil
}

func PromptForConfigAsCode(opts *CreateOptions) error {
	if opts.ConfigAsCode.Value {
		if opts.GitStorage.Value == "" {
			s, err := selectors.GitCredentialStorage("Select where to store the Git credentials", opts.Ask)
			if err != nil {
				return err
			}
			opts.GitStorage.Value = s
		}

		if opts.GitInitialCommitMessage.Value == "" {
			if err := opts.Ask(&survey.Input{
				Message: "Initial Git commit message",
				Help:    "The commit message used in initializing. Default value: '" + InitialGitCommitMessage + "'",
			}, &opts.GitInitialCommitMessage.Value, survey.WithValidator(survey.ComposeValidators(
				survey.MaxLength(50),
			))); err != nil {
				return err
			}
		}

		if opts.GitBasePath.Value == "" {
			if err := opts.Ask(&survey.Input{
				Message: "Git repository base path",
				Help:    "The path in the repository where Config As Code settings are stored. Default value: '" + DefaultBasePath + "'",
			}, &opts.GitBasePath.Value, survey.WithValidator(survey.ComposeValidators(
				survey.MaxLength(200),
			))); err != nil {
				return err
			}
		}

		if opts.GitUrl.Value == "" {
			if err := opts.Ask(&survey.Input{
				Message: "Git Url",
				Help:    "The Url of the Git repository to store configuration.",
			}, &opts.GitUrl.Value, survey.WithValidator(survey.ComposeValidators(
				survey.MaxLength(200),
				survey.MinLength(1),
				survey.Required,
			))); err != nil {
				return err
			}
		}

		if opts.GitBranch.Value == "" {
			if err := opts.Ask(&survey.Input{
				Message: "Git Branch",
				Help:    "The default branch to use. Default value: '" + DefaultBranch + "'",
			}, &opts.GitBranch.Value, survey.WithValidator(survey.ComposeValidators(
				survey.MaxLength(200),
			))); err != nil {
				return err
			}
		}

		if opts.GitUsername.Value == "" {
			if err := opts.Ask(&survey.Input{
				Message: "Git Username",
				Help:    "The Git username.",
			}, &opts.GitUsername.Value, survey.WithValidator(survey.ComposeValidators(
				survey.MaxLength(200),
				survey.MinLength(1),
				survey.Required,
			))); err != nil {
				return err
			}
		}

		if opts.GitPassword.Value == "" {
			if err := opts.Ask(&survey.Password{
				Message: "Git Password",
				Help:    "The Git password.",
			}, &opts.GitPassword.Value, survey.WithValidator(survey.ComposeValidators(
				survey.MaxLength(200),
				survey.MinLength(1),
				survey.Required,
			))); err != nil {
				return err
			}
		}
	}
	return nil
}

func getInitialCommitMessage(opts *CreateOptions) string {
	if opts.GitInitialCommitMessage.Value == "" {
		return InitialGitCommitMessage
	}

	return opts.GitInitialCommitMessage.Value
}

func getBasePath(opts *CreateOptions) string {
	if opts.GitBasePath.Value == "" {
		return DefaultBasePath
	}

	return opts.GitBasePath.Value
}

func getGitBranch(opts *CreateOptions) string {
	if opts.GitBranch.Value == "" {
		return "main"
	}

	return opts.GitBranch.Value
}
