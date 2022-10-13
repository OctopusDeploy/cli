package selectors

import (
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/lifecycles"
)

func Lifecycle(questionText string, octopus *client.Client, ask question.Asker) (*lifecycles.Lifecycle, error) {
	existingLifecycles, err := octopus.Lifecycles.GetAll()
	if err != nil {
		return nil, err
	}

	return Select(ask, questionText, existingLifecycles, func(lc *lifecycles.Lifecycle) string {
		return lc.Name
	})
}
