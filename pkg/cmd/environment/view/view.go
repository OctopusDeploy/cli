package view

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/environment/helper"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

const (
	FlagWeb = "web"
)

type ViewFlags struct {
	Web *flag.Flag[bool]
}

func NewViewFlags() *ViewFlags {
	return &ViewFlags{
		Web: flag.New[bool](FlagWeb, false),
	}
}

type ViewOptions struct {
	Client   *client.Client
	Host     string
	idOrName string
	flags    *ViewFlags
	Command  *cobra.Command
}

func NewCmdView(f factory.Factory) *cobra.Command {
	viewFlags := NewViewFlags()

	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id>}",
		Short: "View an environment",
		Long:  "View an environment in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s environment view 'Production'
			$ %[1]s environment view Environments-102
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GetSystemClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}

			opts := &ViewOptions{
				client,
				f.GetCurrentHost(),
				args[0],
				viewFlags,
				cmd,
			}

			return viewRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&viewFlags.Web.Value, viewFlags.Web.Name, "w", false, "Open in web browser")

	return cmd
}

func viewRun(opts *ViewOptions) error {
	environment, err := helper.GetByIDOrName(opts.Client.Environments, opts.idOrName)
	if err != nil {
		return err
	}

	return output.PrintResource(environment, opts.Command, output.Mappers[*environments.Environment]{
		Json: func(env *environments.Environment) any {
			return EnvironmentAsJson{
				Id:                         env.GetID(),
				Slug:                       env.Slug,
				Name:                       env.Name,
				Description:                env.Description,
				UseGuidedFailure:           env.UseGuidedFailure,
				AllowDynamicInfrastructure: env.AllowDynamicInfrastructure,
				WebUrl:                     generateWebUrl(opts.Host, env),
			}
		},
		Table: output.TableDefinition[*environments.Environment]{
			Header: []string{"NAME", "SLUG", "DESCRIPTION", "GUIDED FAILURE", "DYNAMIC INFRASTRUCTURE", "WEB URL"},
			Row: func(env *environments.Environment) []string {
				description := env.Description
				if description == "" {
					description = constants.NoDescription
				}

				return []string{
					output.Bold(env.Name),
					env.Slug,
					description,
					getBoolToString(env.UseGuidedFailure, "Enabled", "Disabled"),
					getBoolToString(env.UseGuidedFailure, "Allowed", "Disallowed"),
					output.Blue(generateWebUrl(opts.Host, env)),
				}
			},
		},
		Basic: func(env *environments.Environment) string {
			var result strings.Builder

			// header
			result.WriteString(fmt.Sprintf("%s %s\n", output.Bold(env.Name), output.Dimf("(%s)", env.GetID())))

			// metadata
			if len(env.Description) == 0 {
				result.WriteString(fmt.Sprintf("%s\n", output.Dim(constants.NoDescription)))
			} else {
				result.WriteString(fmt.Sprintf("%s\n", output.Dim(env.Description)))
			}

			url := generateWebUrl(opts.Host, env)
			result.WriteString(fmt.Sprintf("View this project in Octopus Deploy: %s\n", output.Blue(url)))

			if opts.flags.Web.Value {
				browser.OpenURL(url)
			}

			return result.String()
		},
	})
}

type EnvironmentAsJson struct {
	Id                         string `json:"Id"`
	Slug                       string `json:"Slug"`
	Name                       string `json:"Name"`
	Description                string `json:"Description"`
	UseGuidedFailure           bool   `json:"UseGuidedFailure"`
	AllowDynamicInfrastructure bool   `json:"AllowDynamicInfrastructure"`
	WebUrl                     string `json:"WebUrl"`
}

func generateWebUrl(host string, env *environments.Environment) string {
	return util.GenerateWebURL(host, env.SpaceID, fmt.Sprintf("infrastructure/environments/%s", env.GetID()))
}

func getBoolToString(value bool, trueString string, falseString string) string {
	if value {
		return trueString
	} else {
		return falseString
	}
}
