package create

import (
	"fmt"
	"io"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	awsCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/aws/create"
	azureCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/azure/create"
	gcpCreate "github.com/OctopusDeploy/cli/pkg/cmd/account/gcp/create"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdCreate(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates an account in an instance of Octopus Deploy",
		Long:  "Creates an account in an instance of Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s account create"
		`), constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return createRun(f, cmd.OutOrStdout())
		},
	}

	return cmd
}

func createRun(f factory.Factory, w io.Writer) error {
	client, err := f.GetSpacedClient()
	if err != nil {
		return err
	}

	accountTypes := []string{
		"AWS Account",
		"Azure Account",
		"Google Cloud Account",
		"SSH Key Pair",
		"Username/Password",
		"Token",
	}

	var accountType string
	err = f.Ask(&survey.Select{
		Help:    "The type of account being created.",
		Message: "Account Type",
		Options: accountTypes,
	}, &accountType)
	if err != nil {
		return err
	}

	switch accountType {
	case "AWS Account":
		opts := &awsCreate.CreateOptions{
			Writer:  w,
			Octopus: client,
			Spinner: f.Spinner(),
			Ask:     f.Ask,
		}
		if err := awsCreate.CreateRun(opts); err != nil {
			return err
		}
	case "Azure Account":
		opts := &azureCreate.CreateOptions{
			Writer:  w,
			Octopus: client,
			Spinner: f.Spinner(),
			Ask:     f.Ask,
		}
		if err := azureCreate.CreateRun(opts); err != nil {
			return err
		}
	case "Google Cloud Account":
		opts := &gcpCreate.CreateOptions{
			Writer:  w,
			Octopus: client,
			Spinner: f.Spinner(),
			Ask:     f.Ask,
		}
		if err := gcpCreate.CreateRun(opts); err != nil {
			return err
		}
	}

	return nil
}
