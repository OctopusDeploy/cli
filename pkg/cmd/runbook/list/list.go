package list

import (
	"errors"
	"github.com/OctopusDeploy/cli/pkg/cmd/runbook/shared"
	"math"

	"github.com/OctopusDeploy/cli/pkg/apiclient"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/runbooks"
	"github.com/spf13/cobra"
)

const (
	FlagProject = "project"
	FlagLimit   = "limit"
	FlagFilter  = "filter"
	FlagGitRef  = "git-ref"
)

type ListFlags struct {
	Project *flag.Flag[string]
	Limit   *flag.Flag[int32]
	Filter  *flag.Flag[string]
	GitRef  *flag.Flag[string]
}

func NewListFlags() *ListFlags {
	return &ListFlags{
		Project: flag.New[string](FlagProject, false),
		Limit:   flag.New[int32](FlagLimit, false),
		Filter:  flag.New[string](FlagFilter, false),
		GitRef:  flag.New[string](FlagGitRef, false),
	}
}

func NewCmdList(f factory.Factory) *cobra.Command {
	listFlags := NewListFlags()

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List runbooks",
		Long:  "List runbooks in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s runbook list SomeProject
			$ %[1]s runbook list --project SomeProject --limit 50 --filter SomeKeyword
			$ %[1]s runbook ls -p SomeProject -n 30 -q SomeKeyword
		`, constants.ExecutableName),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && listFlags.Project.Value == "" {
				listFlags.Project.Value = args[0]
			}

			return listRun(cmd, f, listFlags)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&listFlags.Project.Value, listFlags.Project.Name, "p", "", "Name or ID of the project to list runbooks for")
	flags.Int32Var(&listFlags.Limit.Value, listFlags.Limit.Name, 0, "limit the maximum number of results that will be returned")
	flags.StringVarP(&listFlags.Filter.Value, listFlags.Filter.Name, "q", "", "filter runbooks to match only ones with a name containing the given string")
	flags.StringVarP(&listFlags.GitRef.Value, listFlags.GitRef.Name, "", "", "Git reference to list runbooks for e.g. refs/heads/main. Only relevant for config-as-code projects where runbooks are stored in Git.")
	return cmd
}

type RunbookViewModel struct {
	ID          string
	Name        string
	Description string
}

func listRun(cmd *cobra.Command, f factory.Factory, flags *ListFlags) error {
	outputFormat, err := cmd.Flags().GetString(constants.FlagOutputFormat)
	if err != nil { // should never happen, but fallback if it does
		outputFormat = constants.OutputFormatTable
	}

	limit := flags.Limit.Value
	filter := flags.Filter.Value
	projectNameOrID := flags.Project.Value
	gitReference := flags.GitRef.Value

	octopus, err := f.GetSpacedClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	var selectedProject *projects.Project
	if f.IsPromptEnabled() { // this would be AskQuestions if it were bigger
		if projectNameOrID == "" {
			selectedProject, err = selectors.Project("Select the project to list runbooks for", octopus, f.Ask)
			if err != nil {
				return err
			}
		} else { // project name is already provided, fetch the object because it's needed for further questions
			selectedProject, err = selectors.FindProject(octopus, projectNameOrID)
			if err != nil {
				return err
			}
			if !constants.IsProgrammaticOutputFormat(outputFormat) {
				cmd.Printf("Project %s\n", output.Cyan(selectedProject.Name))
			}
		}
	} else { // we don't have the executions API backing us and allowing NameOrID; we need to do the lookup ourselves
		if projectNameOrID == "" {
			return errors.New("project must be specified")
		}
		selectedProject, err = selectors.FindProject(octopus, projectNameOrID)
		if err != nil {
			return err
		}
	}

	var foundRunbooks *resources.Resources[*runbooks.Runbook]
	if limit <= 0 {
		limit = math.MaxInt32
	}

	if shared.AreRunbooksInGit(selectedProject) {
		if f.IsPromptEnabled() {
			if gitReference == "" { // we need a git ref; ask for one
				gitRef, err := selectors.GitReference("Select the Git reference to list runbooks for", octopus, f.Ask, selectedProject)
				if err != nil {
					return err
				}
				gitReference = gitRef.CanonicalName // e.g /refs/heads/main
			} else {
				if !constants.IsProgrammaticOutputFormat(outputFormat) {
					cmd.Printf("Git reference %s\n", output.Cyan(gitReference))
				}
			}
		} else {
			if gitReference == "" {
				return errors.New("git reference must be specified")
			}
		}

		foundRunbooks, err = runbooks.ListGitRunbooks(octopus, f.GetCurrentSpace().ID, selectedProject.ID, gitReference, filter, int(limit))

		if err != nil {
			return err
		}
	} else {
		foundRunbooks, err = runbooks.List(octopus, f.GetCurrentSpace().ID, selectedProject.ID, filter, int(limit))

		if err != nil {
			return err
		}
	}

	return output.PrintArray(foundRunbooks.Items, cmd, output.Mappers[*runbooks.Runbook]{
		Json: func(item *runbooks.Runbook) any {
			return RunbookViewModel{
				ID:          item.ID,
				Name:        item.Name,
				Description: item.Description,
			}
		},
		Table: output.TableDefinition[*runbooks.Runbook]{
			Header: []string{"NAME", "DESCRIPTION"},
			Row: func(item *runbooks.Runbook) []string {
				return []string{item.Name, item.Description}
			}},
		Basic: func(item *runbooks.Runbook) string {
			return item.Name
		},
	})
}
