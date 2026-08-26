package view

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/credentials"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/platformhubversioncontrolsettings"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

const (
	FlagWeb = "web"

	webPath = "/app#/platform-hub/version-control-settings"
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
	Client  *client.Client
	Host    string
	Command *cobra.Command
	flags   *ViewFlags
}

func NewCmdView(f factory.Factory) *cobra.Command {
	viewFlags := NewViewFlags()
	cmd := &cobra.Command{
		Use:   "view",
		Short: "View Platform Hub version control settings",
		Long:  "View the Platform Hub version control settings in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s platform-hub view
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			client, err := f.GetSystemClient(apiclient.NewRequester(c))
			if err != nil {
				return err
			}

			opts := &ViewOptions{
				Client:  client,
				Host:    f.GetCurrentHost(),
				Command: c,
				flags:   viewFlags,
			}

			return viewRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&viewFlags.Web.Value, viewFlags.Web.Name, "w", false, "Open in web browser")

	return cmd
}

func viewRun(opts *ViewOptions) error {
	settings, err := platformhubversioncontrolsettings.Get(opts.Client)
	if err != nil {
		return err
	}

	if settings.URL == "" {
		fmt.Fprintln(opts.Command.OutOrStdout(), "Platform Hub version control is not configured. Run `octopus platform-hub configure` to configure it.")
		return nil
	}

	webUrl := opts.Host + webPath

	return output.PrintResource(settings, opts.Command, output.Mappers[*platformhubversioncontrolsettings.Resource]{
		Json: func(s *platformhubversioncontrolsettings.Resource) any {
			return SettingsAsJson{
				Url:            s.URL,
				CredentialType: credentialTypeDisplay(s.Credentials),
				DefaultBranch:  s.DefaultBranch,
				BasePath:       s.BasePath,
				WebUrl:         webUrl,
			}
		},
		Table: output.TableDefinition[*platformhubversioncontrolsettings.Resource]{
			Header: []string{"URL", "CREDENTIAL", "BRANCH", "BASE PATH", "WEB URL"},
			Row: func(s *platformhubversioncontrolsettings.Resource) []string {
				return []string{
					output.Bold(s.URL),
					credentialTypeDisplay(s.Credentials),
					s.DefaultBranch,
					s.BasePath,
					output.Blue(webUrl),
				}
			},
		},
		Basic: func(s *platformhubversioncontrolsettings.Resource) string {
			return formatForBasic(opts, s, webUrl)
		},
	})
}

type SettingsAsJson struct {
	Url            string `json:"Url"`
	CredentialType string `json:"CredentialType"`
	DefaultBranch  string `json:"DefaultBranch"`
	BasePath       string `json:"BasePath"`
	WebUrl         string `json:"WebUrl"`
}

func credentialTypeDisplay(c credentials.GitCredential) string {
	if c == nil {
		return "None"
	}
	switch c.Type() {
	case credentials.GitCredentialTypeAnonymous:
		return "Anonymous"
	case credentials.GitCredentialTypeUsernamePassword:
		return "Username/Password"
	case credentials.GitCredentialTypeGitHubApp:
		return "GitHub Connection"
	default:
		return string(c.Type())
	}
}

func formatForBasic(opts *ViewOptions, settings *platformhubversioncontrolsettings.Resource, webUrl string) string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("Url: %s\n", output.Bold(settings.URL)))
	result.WriteString(fmt.Sprintf("Credential: %s\n", credentialTypeDisplay(settings.Credentials)))
	result.WriteString(fmt.Sprintf("Branch: %s\n", settings.DefaultBranch))
	result.WriteString(fmt.Sprintf("Base path: %s\n", settings.BasePath))

	result.WriteString(fmt.Sprintf("\nView Platform Hub version control settings in Octopus Deploy: %s\n", output.Blue(webUrl)))

	if opts.flags.Web.Value {
		_ = browser.OpenURL(webUrl)
	}

	return result.String()
}
