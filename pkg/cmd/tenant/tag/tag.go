package tag

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
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
		Use:   "tag",
		Short: "Tag a tenant in Octopus Deploy",
		Long:  "Tag a tenant in Octopus Deploy.",
		Example: fmt.Sprintf(heredoc.Doc(`
			$ %s tenant tag Tenant-1
		`), constants.ExecutableName),
		RunE: func(c *cobra.Command, _ []string) error {
			opts := NewTagOptions(createFlags, cmd.NewDependencies(f, c))

			return createRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringArrayVarP(&createFlags.Tag.Value, createFlags.Tag.Name, "t", []string{}, "Tag to apply to tenant, must use canonical name: <tag_set>/<tag_name>")
	flags.StringVar(&createFlags.Tenant.Value, createFlags.Tenant.Name, "", "Name or ID you wish to update")

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

	tenant, err := AskTenants(opts.Ask, opts.Tenant.Value, opts.GetTenantsCallback, opts.GetTenantCallback)
	if err != nil {
		return nil, err
	}
	opts.tenant = tenant
	opts.Tenant.Value = tenant.Name

	tags, err := AskTags(opts.Ask, opts.tenant.TenantTags, opts.Tag.Value, opts.GetAllTagsCallback)
	if err != nil {
		return nil, err
	}
	opts.Tag.Value = tags

	nestedOpts = append(nestedOpts, opts)
	return nestedOpts, nil
}

func AskTenants(ask question.Asker, value string, getTenantsCallback GetTenantsCallback, getTenantCallback GetTenantCallback) (*tenants.Tenant, error) {
	if value != "" {
		tenant, err := getTenantCallback(value)
		if err != nil {
			return nil, err
		}
		return tenant, nil
	}

	tenant, err := selectors.Select(ask, "Select the Tenant you would like to update", getTenantsCallback, func(item *tenants.Tenant) string {
		return item.Name
	})
	if err != nil {
		return nil, err
	}

	return tenant, nil
}

func AskTags(ask question.Asker, value []string, newValue []string, getAllTagSetsCallback GetAllTagSetsCallback) ([]string, error) {
	if len(newValue) > 0 {
		return newValue, nil
	}
	tagSets, err := getAllTagSetsCallback()
	if err != nil {
		return nil, err
	}

	canonicalTagName := []string{}
	for _, tagSet := range tagSets {
		for _, tag := range tagSet.Tags {
			canonicalTagName = append(canonicalTagName, tag.CanonicalTagName)
		}
	}
	tags := []string{}
	err = ask(&survey.MultiSelect{
		Options: canonicalTagName,
		Message: "Tags",
		Default: value,
	}, &tags)
	if err != nil {
		return nil, err
	}
	return tags, nil
}
