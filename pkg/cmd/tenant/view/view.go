package view

import (
	"errors"
	"fmt"

	"github.com/OctopusDeploy/cli/pkg/apiclient"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

const (
	FlagTenant = "tenant"
	FlagWeb    = "web"
)

type ViewFlags struct {
	Tenant *flag.Flag[string]
	Web    *flag.Flag[bool]
}

func NewViewFlags() *ViewFlags {
	return &ViewFlags{
		Tenant: flag.New[string](FlagTenant, false),
		Web:    flag.New[bool](FlagWeb, false),
	}
}

func NewCmdView(f factory.Factory) *cobra.Command {
	viewFlags := NewViewFlags()
	cmd := &cobra.Command{
		Use:   "view {<name> | <id>}",
		Short: "View a tenant",
		Long:  "View a tenant in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s tenant view Tenants-1
			$ %[1]s tenant view 'Tenant'
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && viewFlags.Tenant.Value == "" {
				viewFlags.Tenant.Value = args[0]
			}
			return viewRun(cmd, f, viewFlags)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&viewFlags.Tenant.Value, viewFlags.Tenant.Name, "t", "", "Name or ID of the tenant to list variables for")
	flags.BoolVarP(&viewFlags.Web.Value, viewFlags.Web.Name, "w", false, "Open in web browser")

	return cmd
}

func viewRun(cmd *cobra.Command, f factory.Factory, flags *ViewFlags) error {
	client, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	tenant := flags.Tenant.Value
	if tenant == "" {
		return errors.New("tenant must be specified")
	}

	selectedTenant, err := client.Tenants.GetByIdentifier(tenant)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s %s\n", output.Bold(selectedTenant.Name), output.Dimf("(%s)", selectedTenant.ID))

	if len(selectedTenant.TenantTags) > 0 {
		fmt.Fprintf(out, "Tags: ")
	}
	for i, tag := range selectedTenant.TenantTags {
		suffix := ", "
		if i == len(selectedTenant.TenantTags)-1 {
			suffix = ""
		}
		fmt.Fprintf(out, "%s%s", tag, suffix)
	}
	if len(selectedTenant.TenantTags) > 0 {
		fmt.Fprintf(out, "\n")
	}

	if selectedTenant.Description == "" {
		fmt.Fprintln(out, output.Dim(constants.NoDescription))
	} else {
		fmt.Fprintln(out, output.Dim(selectedTenant.Description))
	}

	link := fmt.Sprintf("%s/app#/%s/tenants/%s/overview", f.GetCurrentHost(), selectedTenant.SpaceID, selectedTenant.ID)

	// footer
	fmt.Fprintf(out, "View this tenant in Octopus Deploy: %s\n", output.Blue(link))

	if flags.Web.Value {
		browser.OpenURL(link)
	}

	return nil
}
