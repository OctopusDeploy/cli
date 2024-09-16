package create

import (
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/deployments"
	"golang.org/x/exp/slices"
	"io"
	"strings"
)

var gitResourceGitRefLoopHelpText = heredoc.Doc(`
bold(GIT RESOURCE SELECTION)
 This screen presents the list of Git resources used by your project, and the steps
 which reference them. 

bold(COMMANDS)
 Any any point, you can enter one of the following:
 - green(?) to access this help screen
 - green(y) to accept the list of Git resources and proceed with creating the release
 - green(u) to undo the last edit you made to Git resource Git refs
 - green(r) to reset all Git resource Git ref edits
 - A Git resource Git ref override string.

bold(GIT RESOURCE OVERRIDE STRINGS)
 Git resource override strings must have 2 or 3 components, separated by a : or =
 The first component is always the step name. The last component is always the target Git ref.
 When specifying 2 components, it is assumed that this is for the primary git resource.
 When specifying 3 components, the second component is the name of the Git resource.
 You can specify a * for the Git ref which will default to the step-defined default branch.

 Examples:
   bold(Run Script:refs/heads/my-branch)   dim(# sets primary Git resource in the 'Run Script' step to use the 'refs/heads/my-branch' ref)
   bold(Deploy helm:TemplateValues-1:refs/tags/1.3.2)   dim(# sets the 'TemplateValues-1' Git resource in the 'Deploy helm' step to use the 'refs/tags/1.3.2' ref)
   bold(Run Script:*)              dim(# sets primary Git resource in the 'Run Script' step to use the step-defined default branch)

dim(---------------------------------------------------------------------)
`) // note this expects to have prettifyHelp run over it

func BuildGitResourcesBaseline(deploymentProcessTemplate *deployments.DeploymentProcessTemplate) []*GitResourceGitRef {
	result := make([]*GitResourceGitRef, 0, len(deploymentProcessTemplate.GitResources))

	for _, gitResource := range deploymentProcessTemplate.GitResources {
		result = append(result, &GitResourceGitRef{
			ActionName:      gitResource.ActionName,
			GitRef:          gitResource.DefaultBranch,
			GitResourceName: gitResource.Name,
		})
	}

	return result
}

type GitResourceGitRef struct {
	ActionName      string
	GitRef          string
	GitResourceName string
}

const GitResourceOverrideQuestion = "Git resource reference override string (y to accept, u to undo, r to reset, ? for help):"

func AskGitResourceOverrideLoop(
	gitResourcesBaseline []*GitResourceGitRef,
	initialGitResourceOverrideFlags []string, // the --git-resource command line flag (multiple occurrences)
	asker question.Asker,
	stdout io.Writer) ([]*GitResourceGitRef, error) {

	overriddenGitResourceGitRefs := make([]*GitResourceGitRef, 0)

	// Parse the command line flags into overrides
	for _, s := range initialGitResourceOverrideFlags {
		//take the string from the command line and parse it into a GitResourceGitRef
		parsedOverride, err := ParseGitResourceGitRefString(s)
		if err != nil {
			continue
		}

		resolvedOverride, err := ResolveGitResourceOverride(parsedOverride, gitResourcesBaseline)
		if err != nil {
			continue
		}

		overriddenGitResourceGitRefs = append(overriddenGitResourceGitRefs, resolvedOverride)
	}

	//now merge the parsed overrides with the baseline resources, applying any overrides that match
	gitResourceGitRefsWithOverrides := ApplyGitResourceOverrides(gitResourcesBaseline, overriddenGitResourceGitRefs)

outerLoop:
	for {
		err := printGitResourceGitRefs(stdout, gitResourceGitRefsWithOverrides)
		if err != nil {
			return nil, err
		}

		var resolvedOverride *GitResourceGitRef = nil
		var answer = ""
		err = asker(&survey.Input{
			Message: GitResourceOverrideQuestion,
		}, &answer, survey.WithValidator(func(ans interface{}) error {
			str, ok := ans.(string)
			if !ok {
				return errors.New("internal error; answer was not a string")
			}

			switch str {
			// valid response for continuing the loop; don't attempt to validate these
			case "y", "u", "r", "?", "":
				return nil
			}

			parsedOverride, err := ParseGitResourceGitRefString(str)
			if err != nil {
				return err
			}

			resolvedOverride, err = ResolveGitResourceOverride(parsedOverride, gitResourcesBaseline)
			if err != nil {
				return err
			}

			return nil //all good!
		}))

		// if validators return an error, survey retries itself; the errors don't end up at this level.
		if err != nil {
			return nil, err
		}

		switch answer {
		case "y": // YES these are the git resources they want
			break outerLoop
		case "?": // help text
			_, _ = fmt.Fprintf(stdout, output.FormatDoc(gitResourceGitRefLoopHelpText))
		case "u": // undo!
			if len(overriddenGitResourceGitRefs) > 0 {
				//strip the last override from the list
				overriddenGitResourceGitRefs = overriddenGitResourceGitRefs[:len(overriddenGitResourceGitRefs)-1]
				gitResourceGitRefsWithOverrides = ApplyGitResourceOverrides(gitResourcesBaseline, overriddenGitResourceGitRefs)
			} else {
				_ = fmt.Sprint(stdout, "nothing to undo")
			}

		case "r": // reset! All the way back to the calculated versions, discarding even the stuff that came in from the cmdline
			overriddenGitResourceGitRefs = make([]*GitResourceGitRef, 0)
			gitResourceGitRefsWithOverrides = ApplyGitResourceOverrides(gitResourcesBaseline, overriddenGitResourceGitRefs) //applying any empty array gives us the original list
		default:
			if resolvedOverride != nil {
				overriddenGitResourceGitRefs = append(overriddenGitResourceGitRefs, resolvedOverride)
				gitResourceGitRefsWithOverrides = ApplyGitResourceOverrides(gitResourcesBaseline, overriddenGitResourceGitRefs)
			}
		}
		// loop around and let them put in more input
	}
	return overriddenGitResourceGitRefs, nil
}

func ParseGitResourceGitRefString(gitResourceRef string) (*GitResourceGitRef, error) {
	if strings.TrimSpace(gitResourceRef) == "" {
		return nil, errors.New("empty git resource git ref specification")
	}

	components := splitString(gitResourceRef, []int32{':', '='})
	actionName, gitResourceName, gitRef := "", "", ""

	// We support 2 formats of git resource
	// {StepName}:{GitRef} - This targets the primary git resource for a step only
	// {StepName}:{GitResourceName}:{GitRef} - This targets a secondary git resource

	actionName = strings.TrimSpace(components[0])
	switch len(components) {
	case 2:
		gitRef = strings.TrimSpace(components[1])
	case 3:
		gitResourceName = strings.TrimSpace(components[1])
		gitRef = strings.TrimSpace(components[2])
	default:
		return nil, fmt.Errorf("git resource git ref specification \"%s\" does not use an expected format", gitResourceRef)
	}

	if actionName == "" {
		return nil, fmt.Errorf("git resource git ref specification \"%s\" cannot have an empty step name", gitResourceRef)
	}

	if gitRef == "" {
		return nil, fmt.Errorf("git resource git ref specification \"%s\" cannot have an empty git ref", gitResourceRef)
	}

	return &GitResourceGitRef{
		ActionName:      actionName,
		GitRef:          gitRef,
		GitResourceName: gitResourceName,
	}, nil
}

func ResolveGitResourceOverride(override *GitResourceGitRef, baseline []*GitResourceGitRef) (*GitResourceGitRef, error) {
	index := slices.IndexFunc(baseline, func(grgr *GitResourceGitRef) bool {
		return grgr.Equals(override)
	})

	if index < 0 {
		return nil, fmt.Errorf("could not resolve step name \"%s\" or git resource name \"%s\"", override.ActionName, override.GitResourceName)
	}

	//If the user override matches a git resource in the "baseline"
	var gitRef string
	if override.GitRef == "*" {
		gitRef = baseline[index].GitRef
	} else {
		gitRef = override.GitRef
	}

	return &GitResourceGitRef{
		ActionName:      override.ActionName,
		GitRef:          gitRef,
		GitResourceName: override.GitResourceName,
	}, nil
}

func ApplyGitResourceOverrides(gitResourcesBaseline []*GitResourceGitRef, overrides []*GitResourceGitRef) []*GitResourceGitRef {
	result := make([]*GitResourceGitRef, 0, len(gitResourcesBaseline))

	for _, gitResourceGitRef := range gitResourcesBaseline {

		//Try and find any override
		overrideIdx := slices.IndexFunc(overrides, func(override *GitResourceGitRef) bool {
			return override.Equals(gitResourceGitRef)
		})

		if overrideIdx >= 0 {
			var gitRef string
			override := overrides[overrideIdx]
			if override.GitRef == "*" {
				gitRef = gitResourceGitRef.GitRef
			} else {
				gitRef = override.GitRef
			}
			result = append(result, &GitResourceGitRef{
				ActionName:      gitResourceGitRef.ActionName,
				GitRef:          gitRef,
				GitResourceName: gitResourceGitRef.GitResourceName,
			})
		} else {
			//no override, just return a copy of the original
			result = append(result, &GitResourceGitRef{
				ActionName:      gitResourceGitRef.ActionName,
				GitRef:          gitResourceGitRef.GitRef,
				GitResourceName: gitResourceGitRef.GitResourceName,
			})
		}
	}

	return result
}

// Note this always uses the Table Printer, it pays no respect to outputformat=json, because it's only part of the interactive flow
func printGitResourceGitRefs(ioWriter io.Writer, gitResourceGitRefs []*GitResourceGitRef) error {
	t := output.NewTable(ioWriter)
	t.AddRow(
		output.Bold("STEP NAME"),
		output.Bold("GIT RESOURCE"),
		output.Bold("GIT REF"),
	)

	for _, grgr := range gitResourceGitRefs {
		var name = grgr.GitResourceName
		if name == "" {
			name = "<primary>"
		}

		t.AddRow(
			grgr.ActionName,
			name,
			grgr.GitRef,
		)
	}

	return t.Print()
}

func (gr *GitResourceGitRef) ToGitResourceGitRefString() string {
	components := make([]string, 0, 3)

	components = append(components, gr.ActionName)

	if gr.GitResourceName != "" {
		components = append(components, gr.GitResourceName)
	}

	components = append(components, gr.GitRef)

	return strings.Join(components, ":")
}

func (gr *GitResourceGitRef) Equals(other *GitResourceGitRef) bool {
	return gr.ActionName == other.ActionName && gr.GitResourceName == other.GitResourceName
}
