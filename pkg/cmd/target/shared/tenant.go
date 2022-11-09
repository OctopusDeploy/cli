package shared

import (
	"errors"
	"fmt"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	sharedTenants "github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/machines"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tagsets"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/spf13/cobra"
	"sort"
	"strings"
)

const (
	FlagTenantedDeployment = "tenanted-mode"
	FlagProxy              = "proxy"
	FlagTenant             = "tenant"
	FlagTenantTag          = "tenant-tag"

	Untenanted           = "Untenanted"
	Tenanted             = "Tenanted"
	TenantedOrUntenanted = "TenantedOrUntenanted"
)

type GetAllTagsCallback func() ([]*tagsets.Tag, error)

type CreateTargetTenantFlags struct {
	TenantedDeploymentMode *flag.Flag[string]
	Tenants                *flag.Flag[[]string]
	TenantTags             *flag.Flag[[]string]
}

type CreateTargetTenantOptions struct {
	*cmd.Dependencies

	GetAllTagsCallback
	sharedTenants.GetAllTenantsCallback
}

func NewCreateTargetTenantFlags() *CreateTargetTenantFlags {
	return &CreateTargetTenantFlags{
		TenantedDeploymentMode: flag.New[string](FlagTenantedDeployment, false),
		Tenants:                flag.New[[]string](FlagTenant, false),
		TenantTags:             flag.New[[]string](FlagTenantTag, false),
	}
}

func NewCreateTargetTenantOptions(dependencies *cmd.Dependencies) *CreateTargetTenantOptions {
	return &CreateTargetTenantOptions{
		Dependencies: dependencies,
		GetAllTenantsCallback: func() ([]*tenants.Tenant, error) {
			return sharedTenants.GetAllTenants(*dependencies.Client)
		},
		GetAllTagsCallback: func() ([]*tagsets.Tag, error) {
			return getAllTags(*dependencies.Client)
		},
	}
}

func RegisterCreateTargetTenantFlags(cmd *cobra.Command, flags *CreateTargetTenantFlags) {
	cmd.Flags().StringVar(&flags.TenantedDeploymentMode.Value, FlagTenantedDeployment, "", "\nChoose the kind of deployments where this deployment target should be included. Default is 'untenanted'")
	cmd.Flags().StringSliceVar(&flags.Tenants.Value, FlagTenant, []string{}, "Associate the deployment target with tenants")
	cmd.Flags().StringSliceVar(&flags.TenantTags.Value, FlagTenantTag, []string{}, "Associate the deployment target with tenant tags, should be in the format 'tag set name/tag name'")
}

func PromptForTenant(opts *CreateTargetTenantOptions, flags *CreateTargetTenantFlags) error {
	if flags.TenantedDeploymentMode.Value == "" {
		selectedOption, err := selectors.SelectOptions(opts.Ask, "Choose the kind of deployments where this deployment target should be included", getTenantDeploymentOptions)
		if err != nil {
			return err
		}
		flags.TenantedDeploymentMode.Value = selectedOption.Value
	}

	if isTenantedTarget(flags) && (util.Empty(flags.Tenants.Value) || util.Empty(flags.TenantTags.Value)) {
		allTags, err := opts.GetAllTagsCallback()
		if err != nil {
			return err
		}
		allTenants, err := opts.GetAllTenantsCallback()
		if err != nil {
			return err
		}

		tenantsLookup := make(map[string]bool, len(allTenants))
		tagsLookup := make(map[string]bool, len(allTags))

		var combinedList []string
		for _, tenant := range allTenants {
			combinedList = append(combinedList, tenant.Name)
			tenantsLookup[tenant.Name] = true
		}
		sort.Strings(combinedList)
		canonicalTags := []string{}
		for _, tag := range allTags {
			canonicalTags = append(canonicalTags, tag.CanonicalTagName)
			tagsLookup[tag.CanonicalTagName] = true
		}
		sort.Strings(canonicalTags)
		combinedList = append(combinedList, canonicalTags...)

		var selectedTenantsAndTags []string
		switch flags.TenantedDeploymentMode.Value {
		case "Tenanted":
			selectedTenantsAndTags, err = question.MultiSelectMap(
				opts.Ask,
				"Select tenants this deployment target should be associated with",
				combinedList,
				func(item string) string { return item }, true)
		case "TenantedOrUntenanted":
			selectedTenantsAndTags, err = question.MultiSelectMap(
				opts.Ask,
				"Select tenants this deployment target should be associated with",
				combinedList,
				func(item string) string { return item }, false)
		}

		if err != nil {
			return err
		}

		for _, selection := range selectedTenantsAndTags {
			if tenantsLookup[selection] {
				flags.Tenants.Value = append(flags.Tenants.Value, selection)
			} else if tagsLookup[selection] {
				flags.TenantTags.Value = append(flags.TenantTags.Value, selection)
			} else {
				return errors.New(fmt.Sprintf("unknown selection %s", selection))
			}
		}

	}
	return nil
}

func isTenantedTarget(flags *CreateTargetTenantFlags) bool {
	return flags.TenantedDeploymentMode.Value == Tenanted || flags.TenantedDeploymentMode.Value == TenantedOrUntenanted
}

func getTenantDeploymentOptions() []*selectors.SelectOption[string] {
	return []*selectors.SelectOption[string]{
		{Display: "Exclude from tenanted deployments (default)", Value: Untenanted},
		{Display: "Include only in tenanted deployments", Value: Tenanted},
		{Display: "Include in both tenanted and untenanted deployments", Value: TenantedOrUntenanted},
	}
}

func getAllTags(client client.Client) ([]*tagsets.Tag, error) {
	tagSets, err := client.TagSets.GetAll()
	if err != nil {
		return nil, err
	}

	tags := []*tagsets.Tag{}
	for _, tagSet := range tagSets {
		tags = append(tags, tagSet.Tags...)
	}

	return tags, nil
}

func ConfigureTenant(target *machines.DeploymentTarget, flags *CreateTargetTenantFlags, opts *CreateTargetTenantOptions) error {
	target.TenantedDeploymentMode = core.TenantedDeploymentMode(flags.TenantedDeploymentMode.Value)
	target.TenantTags = flags.TenantTags.Value

	allTenants, err := opts.GetAllTenantsCallback()
	if err != nil {
		return err
	}

	nameLookup := make(map[string]*tenants.Tenant, len(allTenants))
	idLookup := make(map[string]*tenants.Tenant, len(allTenants))

	for _, tenant := range allTenants {
		nameLookup[strings.ToLower(tenant.Name)] = tenant
		idLookup[strings.ToLower(tenant.GetID())] = tenant
	}

	for _, tenantNameOrId := range flags.Tenants.Value {
		nameOrId := strings.ToLower(tenantNameOrId)
		t := nameLookup[nameOrId]
		if t != nil {
			target.TenantIDs = append(target.TenantIDs, t.GetID())
		} else {
			t = idLookup[nameOrId]
			if t != nil {
				target.TenantIDs = append(target.TenantIDs, t.GetID())
			} else {
				return fmt.Errorf("Cannot find tenant '%s'", tenantNameOrId)
			}
		}
	}
	return nil

}
