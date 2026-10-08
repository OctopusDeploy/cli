package list

import (
	"fmt"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/ephemeralenvironment/util"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/tasks"
	"github.com/spf13/cobra"
)

const (
	FlagProject     = "project"
	FlagEnvironment = "environment"
	FlagState       = "state"
	FlagLimit       = "limit"

	DefaultLimit = 30
)

// taskStates are the states the server reports for a task, keyed by their lowercase
// form so that --state is case insensitive.
var taskStates = map[string]string{
	"pending":    "Pending",
	"queued":     "Queued",
	"executing":  "Executing",
	"cancelling": "Cancelling",
	"success":    "Success",
	"failed":     "Failed",
	"canceled":   "Canceled",
	"timedout":   "TimedOut",
}

type ListFlags struct {
	Project     *flag.Flag[string]
	Environment *flag.Flag[string]
	State       *flag.Flag[[]string]
	Limit       *flag.Flag[int]
}

func NewListFlags() *ListFlags {
	return &ListFlags{
		Project:     flag.New[string](FlagProject, false),
		Environment: flag.New[string](FlagEnvironment, false),
		State:       flag.New[[]string](FlagState, false),
		Limit:       flag.New[int](FlagLimit, false),
	}
}

type TaskViewModel struct {
	ID            string
	Description   string
	State         string
	StartTime     *time.Time
	CompletedTime *time.Time
	Duration      string
}

func NewCmdList(f factory.Factory) *cobra.Command {
	listFlags := NewListFlags()
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		Long:  "List tasks in Octopus Deploy, optionally filtered by project, environment and state",
		Example: heredoc.Docf(`
			%[1]s task list
			%[1]s task ls --project "Deploy Web App"
			%[1]s task list --project "Deploy Web App" --environment Production --state Queued --state Executing
			%[1]s task list --environment 123-pr --state Queued,Executing,Cancelling --output-format basic
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRun(cmd, f, listFlags)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&listFlags.Project.Value, listFlags.Project.Name, "p", "", "Name or ID of the project to list tasks for")
	flags.StringVarP(&listFlags.Environment.Value, listFlags.Environment.Name, "e", "", "Name or ID of the environment to list tasks for, including ephemeral environments")
	flags.StringSliceVar(&listFlags.State.Value, listFlags.State.Name, nil, "Only list tasks in this state (Pending, Queued, Executing, Cancelling, Success, Failed, Canceled, TimedOut). Can be repeated or comma separated")
	flags.IntVar(&listFlags.Limit.Value, listFlags.Limit.Name, DefaultLimit, "Maximum number of tasks to list, or 0 to list all matching tasks")
	return cmd
}

func listRun(cmd *cobra.Command, f factory.Factory, flags *ListFlags) error {
	if flags.Limit.Value < 0 {
		return fmt.Errorf("--%s must be zero or greater", FlagLimit)
	}

	states, err := normaliseStates(flags.State.Value)
	if err != nil {
		return err
	}

	octopus, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	query := tasks.TasksQuery{
		States: states,
		Take:   flags.Limit.Value,
	}

	if flags.Project.Value != "" {
		project, err := selectors.FindProject(octopus, flags.Project.Value)
		if err != nil {
			return err
		}
		query.Project = project.GetID()
	}

	if flags.Environment.Value != "" {
		environmentID, err := resolveEnvironmentID(octopus, flags.Environment.Value)
		if err != nil {
			return err
		}
		query.Environment = environmentID
	}

	foundTasks, err := octopus.Tasks.Get(query)
	if err != nil {
		return err
	}

	allTasks := foundTasks.Items
	if flags.Limit.Value == 0 {
		// no limit was asked for, so follow the paging links to collect every match
		allTasks, err = foundTasks.GetAllPages(octopus.Sling())
		if err != nil {
			return err
		}
	}

	viewModels := make([]TaskViewModel, 0, len(allTasks))
	for _, t := range allTasks {
		viewModels = append(viewModels, toViewModel(t))
	}

	return output.PrintArray(viewModels, cmd, output.Mappers[TaskViewModel]{
		Json: func(item TaskViewModel) any {
			return item
		},
		Table: output.TableDefinition[TaskViewModel]{
			Header: []string{"ID", "DESCRIPTION", "STATE", "STARTED", "COMPLETED", "DURATION"},
			Row: func(item TaskViewModel) []string {
				var startTime, completedTime string
				if item.StartTime != nil {
					startTime = item.StartTime.Format(time.RFC1123Z)
				}
				if item.CompletedTime != nil {
					completedTime = item.CompletedTime.Format(time.RFC1123Z)
				}
				return []string{item.ID, item.Description, colourState(item.State), startTime, completedTime, item.Duration}
			},
		},
		Basic: func(item TaskViewModel) string {
			return item.ID
		},
	})
}

// normaliseStates validates the requested states and returns them in the casing the
// server expects. Returns nil when no states were requested.
func normaliseStates(requested []string) ([]string, error) {
	if len(requested) == 0 {
		return nil, nil
	}

	states := make([]string, 0, len(requested))
	for _, s := range requested {
		state, ok := taskStates[strings.ToLower(strings.TrimSpace(s))]
		if !ok {
			return nil, fmt.Errorf("unknown task state '%s'", s)
		}
		states = append(states, state)
	}
	return states, nil
}

// resolveEnvironmentID turns a name or ID into the environment ID that the tasks API
// filters on. Ephemeral environments are not returned by the classic environments
// endpoint, so they are looked up separately when no regular environment matches.
func resolveEnvironmentID(octopus *client.Client, nameOrID string) (string, error) {
	if strings.HasPrefix(nameOrID, "Environments-") {
		return nameOrID, nil
	}

	environment, err := selectors.FindEnvironment(octopus, nameOrID)
	if err == nil {
		return environment.GetID(), nil
	}

	ephemeralEnvironment, ephemeralErr := util.GetByName(octopus, nameOrID, octopus.GetSpaceID())
	if ephemeralErr == nil {
		return ephemeralEnvironment.ID, nil
	}

	return "", fmt.Errorf("no environment or ephemeral environment found with name of %s", nameOrID)
}

func toViewModel(t *tasks.Task) TaskViewModel {
	var duration string
	if t.StartTime != nil && t.CompletedTime != nil {
		duration = t.CompletedTime.Sub(*t.StartTime).Round(time.Second).String()
	}
	return TaskViewModel{
		ID:            t.ID,
		Description:   t.Description,
		State:         t.State,
		StartTime:     t.StartTime,
		CompletedTime: t.CompletedTime,
		Duration:      duration,
	}
}

func colourState(state string) string {
	switch state {
	case "Failed", "TimedOut":
		return output.Red(state)
	case "Success":
		return output.Green(state)
	case "Queued", "Executing", "Cancelling", "Canceled":
		return output.Yellow(state)
	}
	return state
}
