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
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tagsets"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tenants"
	"github.com/spf13/cobra"
	"sort"
)

const (
	FlagRole               = "role"
	FlagEnvironment        = "environment"
	FlagTenantedDeployment = "tenanted-mode"
	FlagProxy              = "proxy"
	FlagTenant             = "tenant"
	FlagTenantTag          = "tenant-tag"

	Untenanted           = "Untenanted"
	Tenanted             = "Tenanted"
	TenantedOrUntenanted = "TenantedOrUntenanted"
)

type GetAllTagsCallback func() ([]*tagsets.Tag, error)
type GetAllRolesCallback func() ([]string, error)

type CreateTargetCommonFlags struct {
	Roles                  *flag.Flag[[]string]
	Environments           *flag.Flag[[]string]
	TenantedDeploymentMode *flag.Flag[string]
	Tenants                *flag.Flag[[]string]
	TenantTags             *flag.Flag[[]string]
}

type CreateTargetCommonOptions struct {
	*cmd.Dependencies

	GetAllTagsCallback
	GetAllRolesCallback
	selectors.GetAllEnvironmentsCallback
	sharedTenants.GetAllTenantsCallback
}

func NewCreateTargetCommonOptions(dependencies *cmd.Dependencies) *CreateTargetCommonOptions {
	return &CreateTargetCommonOptions{
		Dependencies: dependencies,
		GetAllTenantsCallback: func() ([]*tenants.Tenant, error) {
			return sharedTenants.GetAllTenants(*dependencies.Client)
		},
		GetAllTagsCallback: func() ([]*tagsets.Tag, error) {
			return getAllTags(*dependencies.Client)
		},
		GetAllRolesCallback: func() ([]string, error) {
			return getAllMachineRoles(*dependencies.Client)
		},
		GetAllEnvironmentsCallback: func() ([]*environments.Environment, error) {
			return selectors.GetAllEnvironments(*dependencies.Client)
		},
	}
}

func NewCreateTargetCommonFlags() *CreateTargetCommonFlags {
	return &CreateTargetCommonFlags{
		Roles:                  flag.New[[]string](FlagRole, false),
		Environments:           flag.New[[]string](FlagEnvironment, false),
		TenantedDeploymentMode: flag.New[string](FlagTenantedDeployment, false),
		Tenants:                flag.New[[]string](FlagTenant, false),
		TenantTags:             flag.New[[]string](FlagTenantTag, false),
	}
}

func RegisterCreateTargetCommonFlags(cmd *cobra.Command, commonFlags *CreateTargetCommonFlags) {
	cmd.Flags().StringSliceVar(&commonFlags.Roles.Value, FlagRole, []string{}, "Choose at least one role that this deployment target will provide.")
	cmd.Flags().StringSliceVar(&commonFlags.Environments.Value, FlagEnvironment, []string{}, "Choose at least one environment for the deployment target.")
	cmd.Flags().StringVar(&commonFlags.TenantedDeploymentMode.Value, FlagTenantedDeployment, "", "\nChoose the kind of deployments where this deployment target should be included. Default is 'untenanted'")
	cmd.Flags().StringSliceVar(&commonFlags.Tenants.Value, FlagTenant, []string{}, "Associate the deployment target with tenants")
	cmd.Flags().StringSliceVar(&commonFlags.TenantTags.Value, FlagTenantTag, []string{}, "Associate the deployment target with tenant tags, should be in the format 'tag set name/tag name'")
}

func PromptRolesAndEnvironments(opts *CreateTargetCommonOptions, flags *CreateTargetCommonFlags) error {
	if util.Empty(flags.Environments.Value) {
		envs, err := selectors.EnvironmentsMultiSelect(opts.Ask, opts.GetAllEnvironmentsCallback,
			"Choose at least one environment for the deployment target.\n", true)
		if err != nil {
			return err
		}
		flags.Environments.Value = util.SliceTransform(envs, func(e *environments.Environment) string { return e.Name })
	}

	if util.Empty(flags.Roles.Value) {
		availableRoles, err := opts.GetAllRolesCallback()
		if err != nil {
			return err
		}
		roles, err := question.MultiSelectWithAddMap(opts.Ask, "Choose at least one role for the deployment target.\n", availableRoles, true)

		if err != nil {
			return err
		}
		flags.Roles.Value = roles
	}
	return nil
}

func PromptForTenant(opts *CreateTargetCommonOptions, flags *CreateTargetCommonFlags) error {
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

func isTenantedTarget(flags *CreateTargetCommonFlags) bool {
	return flags.TenantedDeploymentMode.Value == Tenanted || flags.TenantedDeploymentMode.Value == TenantedOrUntenanted
}

func getTenantDeploymentOptions() []*selectors.SelectOption[string] {
	return []*selectors.SelectOption[string]{
		{Display: "Exclude from tenanted deployments (default)", Value: Untenanted},
		{Display: "Include only in tenanted deployments", Value: Tenanted},
		{Display: "Include in both tenanted and untenanted deployments", Value: TenantedOrUntenanted},
	}
}

func getAllMachineRoles(client client.Client) ([]string, error) {
	res, err := client.MachineRoles.GetAll()
	if err != nil {
		return nil, err
	}

	var roles []string
	for _, r := range res {
		roles = append(roles, *r)
	}
	return roles, nil
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

func DistinctRoles(roles []string) []string {
	rolesMap := make(map[string]bool)
	result := []string{}
	for _, r := range roles {
		if _, ok := rolesMap[r]; !ok {
			rolesMap[r] = true
			result = append(result, r)
		}
	}

	return result
}
