package shared

import (
	"strings"

	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tagsets"
	"github.com/spf13/cobra"
)

const (
	FlagRole = "role"
	FlagTag  = "tag"
)

type GetAllRolesCallback func() ([]string, error)

type CreateTargetRoleFlags struct {
	Roles *flag.Flag[[]string]
	Tags  *flag.Flag[[]string]
}

type CreateTargetRoleOptions struct {
	*cmd.Dependencies
	GetAllRolesCallback
}

func NewCreateTargetRoleOptions(dependencies *cmd.Dependencies) *CreateTargetRoleOptions {
	return &CreateTargetRoleOptions{
		Dependencies: dependencies,

		GetAllRolesCallback: func() ([]string, error) {
			return getAllMachineRoles(*dependencies.Client)
		},
	}
}

func NewCreateTargetRoleFlags() *CreateTargetRoleFlags {
	return &CreateTargetRoleFlags{
		Roles: flag.New[[]string](FlagRole, false),
		Tags:  flag.New[[]string](FlagTag, false),
	}
}

func RegisterCreateTargetRoleFlags(cmd *cobra.Command, commonFlags *CreateTargetRoleFlags) {
	cmd.Flags().StringSliceVar(&commonFlags.Roles.Value, FlagRole, []string{}, "Choose at least one role that this deployment target will provide (use --tag for tag sets with validation).")
	cmd.Flags().StringSliceVar(&commonFlags.Tags.Value, FlagTag, []string{}, "Target tags in canonical format (TagSetName/TagName).")
}

func PromptForRoles(opts *CreateTargetRoleOptions, flags *CreateTargetRoleFlags) error {
	if util.Empty(flags.Roles.Value) && util.Empty(flags.Tags.Value) {
		tagSets, err := getTargetTagSets(opts.Client)
		if err != nil {
			return err
		}

		if len(tagSets) > 0 {
			tags, err := selectors.Tags(opts.Ask, []string{}, []string{}, tagSets)
			if err != nil {
				return err
			}
			flags.Tags.Value = tags
		} else {
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
	}
	return nil
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

func getTargetTagSets(client *client.Client) ([]*tagsets.TagSet, error) {
	if client == nil {
		return []*tagsets.TagSet{}, nil
	}

	query := tagsets.TagSetsQuery{
		Scopes: []string{string(tagsets.TagSetScopeTarget)},
	}
	result, err := tagsets.Get(client, client.GetSpaceID(), query)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// ValidateTags validates tags in canonical format (TagSetName/TagName) against target scoped tag sets
// Returns plain tag names to send to the API
func ValidateTags(client *client.Client, tags []string) ([]string, error) {
	if len(tags) == 0 {
		return []string{}, nil
	}

	tagSets, err := getTargetTagSets(client)
	if err != nil {
		return nil, err
	}

	if err := selectors.ValidateTags(tags, tagSets); err != nil {
		return nil, err
	}

	plainNames := make([]string, 0, len(tags))
	for _, tag := range tags {
		parts := strings.SplitN(tag, "/", 2)
		plainNames = append(plainNames, parts[1])
	}

	return plainNames, nil
}

func CombineRolesAndTags(client *client.Client, roles []string, tags []string) ([]string, error) {
	combined := make([]string, 0, len(roles)+len(tags))
	combined = append(combined, roles...)

	validatedTags, err := ValidateTags(client, tags)
	if err != nil {
		return nil, err
	}
	combined = append(combined, validatedTags...)

	return combined, nil
}
