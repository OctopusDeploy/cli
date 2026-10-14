package logout

import (
	"github.com/spf13/cobra"

	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
)

func NewCmdLogout(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Logout of Octopus",
		Long:  "Logout of your Octopus server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return logoutRun(f, cmd)
		},
		Annotations: map[string]string{
			annotations.IsConfiguration: "true",
		},
	}

	return cmd
}

func logoutRun(f factory.Factory, cmd *cobra.Command) error {
	configProvider, err := f.GetConfigProvider()
	if err != nil {
		return err
	}
	if err := configProvider.Set("Url", ""); err != nil {
		return err
	}
	if err := configProvider.Set("ApiKey", ""); err != nil {
		return err
	}
	if err := configProvider.Set("AccessToken", ""); err != nil {
		return err
	}

	cmd.Printf("Logout successful")

	return nil
}
