package exclude

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/cmd/tenant/shared"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	sharedVariable "github.com/OctopusDeploy/cli/pkg/question/shared/variables"
	"github.com/OctopusDeploy/cli/pkg/util"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/variables"
	"github.com/spf13/cobra"
)

const (
	FlagProject     = "project"
	FlagVariableSet = "variable-set"
)

type ExcludeFlags struct {
	Project      *flag.Flag[string]
	VariableSets *flag.Flag[[]string]
}

func NewExcludeVariableSetFlags() *ExcludeFlags {
	return &ExcludeFlags{
		Project:      flag.New[string](FlagProject, false),
		VariableSets: flag.New[[]string](FlagVariableSet, false),
	}
}

type ExcludeOptions struct {
	*ExcludeFlags
	*cmd.Dependencies
	shared.GetProjectCallback
	shared.GetAllProjectsCallback
	sharedVariable.GetAllLibraryVariableSetsCallback
}

func NewExcludeOptions(flags *ExcludeFlags, dependencies *cmd.Dependencies) *ExcludeOptions {
	return &ExcludeOptions{
		ExcludeFlags: flags,
		Dependencies: dependencies,
		GetProjectCallback: func(identifier string) (*projects.Project, error) {
			return shared.GetProject(dependencies.Client, identifier)
		},
		GetAllProjectsCallback: func() ([]*projects.Project, error) { return shared.GetAllProjects(dependencies.Client) },
		GetAllLibraryVariableSetsCallback: func() ([]*variables.LibraryVariableSet, error) {
			return sharedVariable.GetAllLibraryVariableSets(dependencies.Client)
		},
	}
}

func NewExcludeVariableSetCmd(f factory.Factory) *cobra.Command {
	createFlags := NewExcludeVariableSetFlags()
	cmd := &cobra.Command{
		Use:   "exclude",
		Short: "Exclude a variable set from a project",
		Long:  "Exclude a variable set from a project in Octopus Deploy",
		Example: heredoc.Docf(`
			$ %[1]s project variable exclude
			$ %[1]s project variable exclude --variable-set "Slack Variables"
		`, constants.ExecutableName),
		RunE: func(c *cobra.Command, args []string) error {
			opts := NewExcludeOptions(createFlags, cmd.NewDependencies(f, c))

			return excludeRun(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Project.Value, createFlags.Project.Name, "p", "", "The project")
	flags.StringArrayVarP(&createFlags.VariableSets.Value, createFlags.VariableSets.Name, "", []string{}, "The name of the library variable set")

	return cmd
}

func excludeRun(opts *ExcludeOptions) error {
	if !opts.NoPrompt {
		err := PromptMissing(opts)
		if err != nil {
			return err
		}
	}

	project, err := opts.GetProjectCallback(opts.Project.Value)
	if err != nil {
		return err
	}

	libraryVariableSets, err := opts.GetAllLibraryVariableSetsCallback()
	if err != nil {
		return err
	}

	projectModified := false
	for _, variableSet := range opts.VariableSets.Value {
		targetVariableSet := util.SliceFilter(libraryVariableSets, func(item *variables.LibraryVariableSet) bool { return strings.EqualFold(variableSet, item.Name) })
		if util.Empty(targetVariableSet) {
			return fmt.Errorf("cannot find library variable set '%s'", variableSet)
		}

		if len(targetVariableSet) > 1 {
			return fmt.Errorf("'%s' matched more than one library variable set", variableSet)
		}

		if !util.SliceContainsAny(project.IncludedLibraryVariableSets, func(item string) bool { return item == targetVariableSet[0].ID }) {
			fmt.Fprint(opts.Out, output.Yellowf("'%s' is not currently included, skipping\n", targetVariableSet[0].Name))
		} else {
			project.IncludedLibraryVariableSets = util.SliceFilter(project.IncludedLibraryVariableSets, func(id string) bool { return id != targetVariableSet[0].ID })
			projectModified = true
			fmt.Fprint(opts.Out, output.Cyanf("Removing '%s' library variable set\n", targetVariableSet[0].Name))
		}
	}

	if projectModified {
		_, err = opts.Client.Projects.Update(project)
		if err != nil {
			return err
		}

		fmt.Fprintf(opts.Out, "Successfully updated library variable sets\n")
	}

	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Project, opts.VariableSets)
		fmt.Fprintf(opts.Out, "\nAutomation Command: %s\n", autoCmd)
	}

	return nil
}

func PromptMissing(opts *ExcludeOptions) error {
	var project *projects.Project
	var err error
	if opts.Project.Value == "" {
		project, err = projectSelector("You have not specified a Project. Please select one:", opts.GetAllProjectsCallback, opts.Ask)
		if err != nil {
			return nil
		}
		opts.Project.Value = project.GetName()
	} else {
		project, err = opts.GetProjectCallback(opts.Project.Value)
		if err != nil {
			return err
		}
	}

	libraryVariableSets, err := opts.GetAllLibraryVariableSetsCallback()

	linkedVariableSets := util.SliceFilter(libraryVariableSets, func(item *variables.LibraryVariableSet) bool {
		return util.SliceContains(project.IncludedLibraryVariableSets, item.ID)
	})

	if util.Empty(linkedVariableSets) {
		return fmt.Errorf("no library variable sets available to exclude")
	}

	if util.Empty(opts.VariableSets.Value) {
		selectedVariableSets, err := question.MultiSelectMap(
			opts.Ask,
			"Select the Library Variable Sets to exclude in the project",
			linkedVariableSets,
			func(item *variables.LibraryVariableSet) string {
				return item.Name
			}, false)

		if err != nil {
			return err
		}

		if util.Empty(selectedVariableSets) {
			return fmt.Errorf("no library variable sets selected")
		}
		opts.VariableSets.Value = util.SliceTransform(selectedVariableSets, func(item *variables.LibraryVariableSet) string { return item.Name })
	}

	return nil
}

func projectSelector(questionText string, getAllProjectsCallback shared.GetAllProjectsCallback, ask question.Asker) (*projects.Project, error) {
	existingProjects, err := getAllProjectsCallback()
	if err != nil {
		return nil, err
	}

	return question.SelectMap(ask, questionText, existingProjects, func(p *projects.Project) string { return p.GetName() })
}
