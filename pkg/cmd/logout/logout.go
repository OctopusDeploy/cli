package logout

import (
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
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
	configProvider.Set("Url", "")
	configProvider.Set("ApiKey", "")
	configProvider.Set("AccessToken", "")

	cmd.Printf("Logout successful")

	return nil
}
