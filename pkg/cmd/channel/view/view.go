package view

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/usage"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/channels"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

const (
	FlagProject = "project"
	FlagWeb     = "web"
)

type ViewFlags struct {
	Project *flag.Flag[string]
	Web     *flag.Flag[bool]
}

func NewViewFlags() *ViewFlags {
	return &ViewFlags{
		Project: flag.New[string](FlagProject, false),
		Web:     flag.New[bool](FlagWeb, false),
	}
}

type ViewOptions struct {
	Client   *client.Client
	Host     string
	out      io.Writer
	idOrName string
	flags    *ViewFlags
	Command  *cobra.Command
}

func NewCmdView(f factory.Factory) *cobra.Command {
	viewFlags := NewViewFlags()
	cmd := &cobra.Command{
		Args:  usage.ExactArgs(1),
		Use:   "view {<name> | <id> | <slug>}",
		Short: "View a channel",
		Long:  "View a channel in Octopus Deploy",
		Example: heredoc.Docf(`
			%[1]s channel view "Hotfix" --project myProject
			%[1]s channel view Channels-123 --project myProject
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			if viewFlags.Project.Value == "" {
				return errors.New("--project is required")
			}

			c, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
			if err != nil {
				return err
			}

			opts := &ViewOptions{
				Client:   c,
				Host:     f.GetCurrentHost(),
				out:      cmd.OutOrStdout(),
				idOrName: args[0],
				flags:    viewFlags,
				Command:  cmd,
			}

			return viewRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&viewFlags.Project.Value, viewFlags.Project.Name, "p", "", "Name or ID of the project the channel belongs to")
	flags.BoolVarP(&viewFlags.Web.Value, viewFlags.Web.Name, "w", false, "Open in web browser")

	return cmd
}

func viewRun(opts *ViewOptions) error {
	project, err := selectors.FindProject(opts.Client, opts.flags.Project.Value)
	if err != nil {
		return err
	}

	// Try direct GetByID first; if that fails (the user gave a name or slug), fall back to
	// the project-scoped FindChannel which iterates Projects.GetChannels.
	var channel *channels.Channel
	channel, err = opts.Client.Channels.GetByID(opts.idOrName)
	if err != nil || channel == nil {
		channel, err = selectors.FindChannel(opts.Client, project, opts.idOrName)
		if err != nil {
			return err
		}
	}

	// Resolve lifecycle name for display (best-effort — fall back to ID on error).
	lifecycleName := channel.LifecycleID
	if channel.LifecycleID != "" {
		if lc, lcErr := opts.Client.Lifecycles.GetByIDOrName(channel.LifecycleID); lcErr == nil && lc != nil {
			lifecycleName = lc.Name
		}
	}

	webPath := fmt.Sprintf("projects/%s/deployments/channels/edit/%s", project.Slug, channel.ID)

	return output.PrintResource(channel, opts.Command, output.Mappers[*channels.Channel]{
		Json: func(c *channels.Channel) any {
			return ChannelAsJson{
				Id:                                      c.ID,
				Name:                                    c.Name,
				Description:                             c.Description,
				ProjectId:                               c.ProjectID,
				LifecycleId:                             c.LifecycleID,
				LifecycleName:                           lifecycleName,
				IsDefault:                               c.IsDefault,
				Type:                                    string(c.Type),
				TenantTags:                              c.TenantTags,
				EphemeralEnvironmentNameTemplate:        c.EphemeralEnvironmentNameTemplate,
				ParentEnvironmentId:                     c.ParentEnvironmentID,
				AutomaticEphemeralEnvironmentDeployments: c.AutomaticEphemeralEnvironmentDeployments,
				RuleCount:                               len(c.Rules),
				GitReferenceRuleCount:                   len(c.GitReferenceRules),
				GitResourceRuleCount:                    len(c.GitResourceRules),
				CustomFieldDefinitionCount:              len(c.CustomFieldDefinitions),
				WebUrl:                                  util.GenerateWebURL(opts.Host, c.SpaceID, webPath),
			}
		},
		Table: output.TableDefinition[*channels.Channel]{
			Header: []string{"NAME", "TYPE", "DEFAULT", "LIFECYCLE", "DESCRIPTION", "WEB URL"},
			Row: func(c *channels.Channel) []string {
				description := c.Description
				if description == "" {
					description = constants.NoDescription
				}
				def := ""
				if c.IsDefault {
					def = "yes"
				}
				return []string{
					output.Bold(c.Name),
					string(c.Type),
					def,
					lifecycleName,
					description,
					output.Blue(util.GenerateWebURL(opts.Host, c.SpaceID, webPath)),
				}
			},
		},
		Basic: func(c *channels.Channel) string {
			return formatChannelForBasic(opts, c, lifecycleName, webPath)
		},
	})
}

type ChannelAsJson struct {
	Id                                       string   `json:"Id"`
	Name                                     string   `json:"Name"`
	Description                              string   `json:"Description"`
	ProjectId                                string   `json:"ProjectId"`
	LifecycleId                              string   `json:"LifecycleId"`
	LifecycleName                            string   `json:"LifecycleName,omitempty"`
	IsDefault                                bool     `json:"IsDefault"`
	Type                                     string   `json:"Type"`
	TenantTags                               []string `json:"TenantTags,omitempty"`
	EphemeralEnvironmentNameTemplate         string   `json:"EphemeralEnvironmentNameTemplate,omitempty"`
	ParentEnvironmentId                      string   `json:"ParentEnvironmentId,omitempty"`
	AutomaticEphemeralEnvironmentDeployments bool     `json:"AutomaticEphemeralEnvironmentDeployments,omitempty"`
	RuleCount                                int      `json:"RuleCount"`
	GitReferenceRuleCount                    int      `json:"GitReferenceRuleCount"`
	GitResourceRuleCount                     int      `json:"GitResourceRuleCount"`
	CustomFieldDefinitionCount               int      `json:"CustomFieldDefinitionCount"`
	WebUrl                                   string   `json:"WebUrl"`
}

func formatChannelForBasic(opts *ViewOptions, c *channels.Channel, lifecycleName, webPath string) string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("%s %s\n", output.Bold(c.Name), output.Dimf("(%s)", c.ID)))

	if c.IsDefault {
		result.WriteString(fmt.Sprintf("%s\n", output.Cyan("Default channel")))
	}

	result.WriteString(fmt.Sprintf("Type: %s\n", string(c.Type)))
	result.WriteString(fmt.Sprintf("Lifecycle: %s %s\n", lifecycleName, output.Dimf("(%s)", c.LifecycleID)))

	if c.Description == "" {
		result.WriteString(fmt.Sprintln(output.Dim(constants.NoDescription)))
	} else {
		result.WriteString(fmt.Sprintln(output.Dim(c.Description)))
	}

	if len(c.TenantTags) > 0 {
		result.WriteString(fmt.Sprintf("Tenant tags: %s\n", output.Cyan(output.FormatAsList(c.TenantTags))))
	}

	if string(c.Type) == "EphemeralEnvironment" {
		if c.ParentEnvironmentID != "" {
			result.WriteString(fmt.Sprintf("Parent environment: %s\n", c.ParentEnvironmentID))
		}
		if c.EphemeralEnvironmentNameTemplate != "" {
			result.WriteString(fmt.Sprintf("Ephemeral environment name template: %s\n", c.EphemeralEnvironmentNameTemplate))
		}
		result.WriteString(fmt.Sprintf("Automatic deployments: %t\n", c.AutomaticEphemeralEnvironmentDeployments))
	}

	if len(c.Rules) > 0 {
		result.WriteString(fmt.Sprintf("Version rules: %d\n", len(c.Rules)))
	}
	if len(c.GitReferenceRules) > 0 {
		result.WriteString(fmt.Sprintf("Git reference rules: %d\n", len(c.GitReferenceRules)))
	}
	if len(c.GitResourceRules) > 0 {
		result.WriteString(fmt.Sprintf("Git resource rules: %d\n", len(c.GitResourceRules)))
	}
	if len(c.CustomFieldDefinitions) > 0 {
		result.WriteString(fmt.Sprintf("Custom field definitions: %d\n", len(c.CustomFieldDefinitions)))
	}

	url := util.GenerateWebURL(opts.Host, c.SpaceID, webPath)
	result.WriteString(fmt.Sprintf("\nView this channel in Octopus Deploy: %s\n", output.Blue(url)))

	if opts.flags.Web.Value {
		_ = browser.OpenURL(url)
	}

	return result.String()
}
