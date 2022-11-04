package selectors

import (
	"github.com/OctopusDeploy/cli/pkg/question"
)

type GetAllRolesCallback func() ([]*string, error)

func RoleMultiSelect(ask question.Asker, getAllRoles GetAllRolesCallback, message string, required bool) ([]*string, error) {
	allRoles, err := getAllRoles()
	if err != nil {
		return nil, err
	}

	// TODO: allow creating new entries

	return question.MultiSelectMap(ask, message, allRoles, func(item *string) string { return *item }, required)
}
