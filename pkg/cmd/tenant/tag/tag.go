package tag

import (
	"fmt"
	"io"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/spf13/cobra"
)

const (
	FlagTag    = "tag"
	FlagTenant = "tenant"
)

type TagFlags struct {
	Tag    *flag.Flag[[]string]
	Tenant *flag.Flag[string]
}

func NewTagFlags() *TagFlags {
	return &TagFlags{
		Tag:    flag.New[[]string](FlagTag, false),
		Tenant: flag.New[string](FlagTenant, false),
	}
}

func NewCmdTag(f factory.Factory) *cobra.Command {
	createFlags := NewTagFlags()

	cmd := &cobra.Command{
		Use:     "tag",
		Short:   "Override tags for a tenant",
		Long:    "Override tags for a tenant in Octopus Deploy",
		Example: heredoc.Docf("$ %s tenant tag Tenant-1", constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewTagOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringArrayVarP(&createFlags.Tag.Value, createFlags.Tag.Name, "t", []string{}, "Tag to apply to tenant, must use canonical name: <tag_set>/<tag_name>")
	flags.StringVar(&createFlags.Tenant.Value, createFlags.Tenant.Name, "", "Name or ID of the tenant you wish to update")

	return cmd
}

func createRun(opts *TagOptions) error {
	var optsArray []cmd.Dependable
	var err error
	if !opts.NoPrompt {
		optsArray, err = PromptMissing(opts)
		if err != nil {
			return err
		}
	} else {
		// Validate tags when running with --no-prompt
		if len(opts.Tag.Value) > 0 {
			tagSets, err := opts.GetAllTagsCallback()
			if err != nil {
				return err
			}
			if err := selectors.ValidateTags(opts.Tag.Value, tagSets); err != nil {
				return err
			}
		}
		optsArray = append(optsArray, opts)
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

	return nil
}

func PromptMissing(opts *TagOptions) ([]cmd.Dependable, error) {
	nestedOpts := []cmd.Dependable{}

	tenant, err := AskTenants(opts.Ask, opts.Out, opts.Tenant.Value, opts.GetTenantsCallback, opts.GetTenantCallback)
	if err != nil {
		return nil, err
	}
	opts.tenant = tenant
	opts.Tenant.Value = tenant.Name

	tagSets, err := opts.GetAllTagsCallback()
	if err != nil {
		return nil, err
	}

	tags, err := selectors.Tags(opts.Ask, opts.tenant.TenantTags, opts.Tag.Value, tagSets)
	if err != nil {
		return nil, err
	}
	opts.Tag.Value = tags

	nestedOpts = append(nestedOpts, opts)
	return nestedOpts, nil
}

func AskTenants(ask question.Asker, out io.Writer, value string, getTenantsCallback GetTenantsCallback, getTenantCallback GetTenantCallback) (*tenants.Tenant, error) {
	if value != "" {
		tenant, err := getTenantCallback(value)
		if err != nil {
			return nil, err
		}
		return tenant, nil
	}

	// Check if there's only one tenant
	tns, err := getTenantsCallback()
	if err != nil {
		return nil, err
	}
	if len(tns) == 1 {
		fmt.Fprintf(out, "Selecting only available tenant '%s'.\n", output.Cyan(tns[0].Name))
		return tns[0], nil
	}

	tenant, err := selectors.Select(ask, "Select the Tenant you would like to update", getTenantsCallback, func(item *tenants.Tenant) string {
		return item.Name
	})
	if err != nil {
		return nil, err
	}

	return tenant, nil
}

