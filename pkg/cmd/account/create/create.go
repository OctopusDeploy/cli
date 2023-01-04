package create

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	awsCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/aws/create"
	azureCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/azure/create"
	gcpCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/gcp/create"
	sshCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/ssh/create"
	tokenCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/token/create"
	usernameCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/username/create"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

const (
	AwsAccount              = "AWS Account"
	AzureAccount            = "Azure Account"
	GcpAccount              = "Google Cloud Account"
	SshAccount              = "SSH Key Pair"
	UsernamePasswordAccount = "Username/Password"
	TokenAccount            = "Token"
)

func NewCmdCreate(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create an account",
		Long:    "Create an account in Octopus Deploy",
		Example: heredoc.Docf("$ %s account create", constants.ExecutableName),
		Aliases: []string{"new"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return createRun(f, cmd)
		},
	}

	return cmd
}

func createRun(f factory.Factory, c *cobra.Command) error {
	dependencies := cmd.NewDependencies(f, c)

	accountTypes := []string{
		AwsAccount,
		AzureAccount,
		GcpAccount,
		SshAccount,
		UsernamePasswordAccount,
		TokenAccount,
	}

	var accountType string
	err := f.Ask(&survey.Select{
		Help:    "The type of account being created.",
		Message: "Account Type",
		Options: accountTypes,
	}, &accountType)
	if err != nil {
		return err
	}

	switch accountType {
	case AwsAccount:
		opts := awsCreate.NewCreateOptions(awsCreate.NewCreateFlags(), cmd.NewDependenciesFromExisting(dependencies, fmt.Sprintf("%s account aws create", constants.ExecutableName)))
		if err := awsCreate.CreateRun(opts); err != nil {
			return err
		}
	case AzureAccount:
		opts := azureCreate.NewCreateOptions(azureCreate.NewCreateFlags(), cmd.NewDependenciesFromExisting(dependencies, fmt.Sprintf("%s account azure create", constants.ExecutableName)))
		if err := azureCreate.CreateRun(opts); err != nil {
			return err
		}
	case GcpAccount:
		opts := gcpCreate.NewCreateOptions(gcpCreate.NewCreateFlags(), cmd.NewDependenciesFromExisting(dependencies, fmt.Sprintf("%s account gcp create", constants.ExecutableName)))
		if err := gcpCreate.CreateRun(opts); err != nil {
			return err
		}
	case SshAccount:
		opts := sshCreate.NewCreateOptions(sshCreate.NewCreateFlags(), cmd.NewDependenciesFromExisting(dependencies, fmt.Sprintf("%s account ssh create", constants.ExecutableName)))
		if err := sshCreate.CreateRun(opts); err != nil {
			return err
		}
	case TokenAccount:
		opts := tokenCreate.NewCreateOptions(tokenCreate.NewCreateFlags(), cmd.NewDependenciesFromExisting(dependencies, fmt.Sprintf("%s account token create", constants.ExecutableName)))
		if err := tokenCreate.CreateRun(opts); err != nil {
			return err
		}
	case UsernamePasswordAccount:
		opts := usernameCreate.NewCreateOptions(usernameCreate.NewCreateFlags(), cmd.NewDependenciesFromExisting(dependencies, fmt.Sprintf("%s account username create", constants.ExecutableName)))
		if err := usernameCreate.CreateRun(opts); err != nil {
			return err
		}
	}

	return nil
}
