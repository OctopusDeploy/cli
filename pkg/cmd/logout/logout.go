package logout

import (
	"github.com/OctopusDeploy/cli/pkg/cmd/config/set"
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
			return logoutRun(cmd)
		},
		Annotations: map[string]string{
			annotations.IsConfiguration: "true",
		},
	}

	return cmd
}

func logoutRun(cmd *cobra.Command) error {
	set.SetConfig("Url", "")
	set.SetConfig("ApiKey", "")
	set.SetConfig("AccessToken", "")

	return nil
}
