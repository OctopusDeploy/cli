// Package create handles creating new environments in Octopus Deploy
package create

import (
	"fmt"

	// "github.com/AlecAivazis/survey/v2"  // Interactive prompts for user input
	"github.com/MakeNowJust/heredoc/v2" // Multi-line string formatting
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/output"                                                // Output formatting utilities
	"github.com/OctopusDeploy/cli/pkg/question"                                              // Common question prompts
	"github.com/OctopusDeploy/cli/pkg/question/selectors"                                    // Project selectors
	"github.com/OctopusDeploy/cli/pkg/util/flag"                                             // Flag management utilities
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/environments/v2/ephemeralenvironments" // Octopus Deploy environments API
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/projects"                              // Octopus Deploy projects API
	"github.com/spf13/cobra"                                                                 // Command-line interface framework

	"github.com/OctopusDeploy/cli/pkg/cmd/runbook/shared" // For GetAllProjects function
)

// Constants defining the command-line flag names
const (
	FlagName    = "name"
	FlagProject = "project"
)

// CreateFlags holds all the command-line flags for the create command
type CreateFlags struct {
	Name    *flag.Flag[string] // Environment name
	Project *flag.Flag[string] // Project name
}

// NewCreateFlags creates and initializes a new CreateFlags struct with default values
func NewCreateFlags() *CreateFlags {
	return &CreateFlags{
		Name:    flag.New[string](FlagName, false),    // Create string flag for name
		Project: flag.New[string](FlagProject, false), // Create string flag for project
	}
}

// CreateOptions combines the command flags with common dependencies like API client and output
type CreateOptions struct {
	*CreateFlags                                               // Embeds all the command flags
	*cmd.Dependencies                                          // Embeds common dependencies (API client, output writer, etc.)
	GetAllProjectsCallback func() ([]*projects.Project, error) // Callback to retrieve all projects
}

// NewCreateOptions creates a new CreateOptions struct with the provided flags and dependencies
func NewCreateOptions(createFlags *CreateFlags, dependencies *cmd.Dependencies) *CreateOptions {
	return &CreateOptions{
		CreateFlags:            createFlags,  // Store the command flags
		Dependencies:           dependencies, // Store the common dependencies
		GetAllProjectsCallback: func() ([]*projects.Project, error) { return shared.GetAllProjects(dependencies.Client) },
	}
}

// NewCmdCreate creates the cobra command for creating environments
func NewCmdCreate(f factory.Factory) *cobra.Command {
	createFlags := NewCreateFlags() // Initialize the command flags

	// Create the cobra command with its configuration
	cmd := &cobra.Command{
		Use:     "create",                                                                   // Command name
		Short:   "Create an ephemeral environment",                                          // Short description
		Long:    "Create an ephemeral environment in Octopus Deploy",                        // Long description
		Example: heredoc.Docf("$ %s ephemeralenvironment create", constants.ExecutableName), // Usage example
		Aliases: []string{"new"},                                                            // Alternative command names
		RunE: func(c *cobra.Command, _ []string) error { // Function to run when command is executed
			opts := NewCreateOptions(createFlags, cmd.NewDependencies(f, c)) // Create options with flags and dependencies

			return createRun(opts) // Execute the main create logic
		},
	}

	// Set up command-line flags
	flags := cmd.Flags()
	flags.StringVarP(&createFlags.Name.Value, createFlags.Name.Name, "n", "", "Name of the environment")   // -n, --name flag
	flags.StringVarP(&createFlags.Project.Value, createFlags.Project.Name, "p", "", "Name of the project") // -p, --project flag

	return cmd // Return the configured command
}

// createRun contains the main logic for creating an environment
func createRun(opts *CreateOptions) error {
	// If prompting is enabled, ask user for any missing values
	if !opts.NoPrompt {
		err := PromptMissing(opts)
		if err != nil {
			return err
		}
	}

	// Get the project by name to retrieve its ID
	projectResource, err := projects.GetByName(opts.Client, opts.Space.ID, opts.Project.Value)
	if err != nil {
		return fmt.Errorf("failed to find project '%s': %w", opts.Project.Value, err)
	}
	// Update the project value to use the ID instead of name for the API call
	projectId := projectResource.GetID()

	// Create a new environment command and send to Octopus deploy
	createEnv, err := ephemeralenvironments.Add(opts.Client, opts.Space.ID, projectId, opts.Name.Value)
	if err != nil {
		return err
	}

	// Print success message with environment name and ID
	_, err = fmt.Fprintf(opts.Out, "\nSuccessfully created ephemeral environment '%s` with id '%s'.\n", opts.Name.Value, createEnv.Id)
	if err != nil {
		return err
	}

	// Generate and display a clickable link to view the environment in Octopus Deploy web UI
	link := output.Bluef("%s/app#/%s/projects/%s/ephemeral-environments", opts.Host, opts.Space.GetID(), projectId)              // cc check link works after you fix project id/name issue
	fmt.Fprintf(opts.Out, "View this ephemeral environments for project `%s` on Octopus Deploy: %s\n", opts.Project.Value, link) // cc fix text to show project name

	// If prompting is enabled, show the equivalent automation command for future use
	if !opts.NoPrompt {
		autoCmd := flag.GenerateAutomationCmd(opts.CmdPath, opts.Name, opts.Project)
		fmt.Fprintf(opts.Out, "%s\n", autoCmd)
	}

	return nil // Success - no error
}

// PromptMissing prompts the user for any missing required values
func PromptMissing(opts *CreateOptions) error {
	// Ask for environment name if not provided
	err := question.AskName(opts.Ask, "", "ephemeral environment", &opts.Name.Value)
	if err != nil {
		return err
	}

	// Ask for project name if not provided
	if opts.Project.Value == "" {
		project, err := selectors.Select(opts.Ask, "Select the project to associate the ephemeral environment with:", opts.GetAllProjectsCallback, func(project *projects.Project) string { return project.GetName() })
		if err != nil {
			return err
		}
		opts.Project.Value = project.GetName()
	}

	return nil
}
