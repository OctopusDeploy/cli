package install

import (
	"context"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	sharedTarget "github.com/OctopusDeploy/cli/pkg/cmd/target/shared"
	sharedWorker "github.com/OctopusDeploy/cli/pkg/cmd/worker/shared"
	octoK8s "github.com/OctopusDeploy/cli/pkg/kubernetes"
	agentK8s "github.com/OctopusDeploy/cli/pkg/kubernetes/agent"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/OctopusDeploy/cli/pkg/question/selectors"
)

// PromptMissing guards every prompt on its flag, so supplying a flag suppresses
// the matching question and the generated automation command reproduces the run.
func PromptMissing(ctx context.Context, opts *InstallOptions) error {
	// Recorded before the defaults fill the rest in, so a supplied flag
	// suppresses its prompt rather than merely seeding it.
	suppliedPollingAddress := opts.ServerCommsAddress.Value != ""

	if err := promptForEula(opts); err != nil {
		return err
	}

	if err := question.AskName(opts.Ask, "", string(opts.Mode), &opts.Name.Value); err != nil {
		return err
	}

	if err := opts.resolveNames(); err != nil {
		return err
	}
	opts.applyDefaults()
	if err := opts.validateMachinePolicy(); err != nil {
		return err
	}
	// A role named by flag still has to be read, so the review can say what the
	// script pods are getting.
	if err := opts.resolveScriptPodRoles(ctx); err != nil {
		return err
	}
	opts.reportFindings()

	if err := promptForRegistration(opts); err != nil {
		return err
	}

	if !suppliedPollingAddress {
		if err := promptForPollingAddress(opts); err != nil {
			return err
		}
	}

	if err := promptForStorage(opts); err != nil {
		return err
	}
	opts.warnAboutAccessMode()

	return promptForScriptPodPermissions(opts)
}

// promptForRegistration asks only what the agent registers itself with, which
// is the one part that differs between a deployment target and a worker.
func promptForRegistration(opts *InstallOptions) error {
	if opts.isWorker() {
		return promptForWorkerPools(opts)
	}

	if err := sharedTarget.PromptForEnvironments(opts.CreateTargetEnvironmentOptions, opts.CreateTargetEnvironmentFlags); err != nil {
		return err
	}
	if err := promptForTargetTags(opts); err != nil {
		return err
	}
	// Tenanted deployments are deliberately not asked about. Octopus's own agent
	// wizard does not either: the docs point people at the target's settings
	// afterwards, and --tenanted-mode covers the scripted case.
	return promptForDefaultNamespace(opts)
}

// promptForWorkerPools stops with something to act on rather than an empty
// list. A space can easily have only dynamic pools, which Octopus runs on its
// own machines and a worker cannot join.
func promptForWorkerPools(opts *InstallOptions) error {
	if len(opts.WorkerPools.Value) > 0 {
		return opts.validateWorkerPools()
	}

	pools, err := opts.GetAllWorkerPoolsCallback()
	if err != nil {
		return err
	}
	if len(pools) == 0 {
		return fmt.Errorf("no worker pool in space %s can hold a worker. %s",
			opts.spaceName(), staticPoolAdvice())
	}

	return sharedWorker.PromptForWorkerPools(opts.WorkerPoolOptions, opts.WorkerPoolFlags)
}

// promptForTargetTags asks once for target tags, rather than once per tag set
// as the deployment target commands do. Which set a tag belongs to is Octopus's
// business; somebody installing an agent is choosing what the agent is for.
//
// A tag that is not in the list can be typed in, because Octopus creates a
// target tag as soon as a target registers with it.
func promptForTargetTags(opts *InstallOptions) error {
	if len(opts.Roles.Value) > 0 || len(opts.Tags.Value) > 0 {
		return nil
	}

	available, err := opts.knownTargetTags()
	if err != nil {
		return err
	}

	chosen, err := question.MultiSelectWithAddMap(opts.Ask,
		"Which target tags should this deployment target have?\n", available, true, "tag")
	if err != nil {
		return err
	}

	opts.Roles.Value = chosen
	return nil
}

// promptForEula comes first so that declining costs one question rather than a
// page of them. The chart will not install without it.
func promptForEula(opts *InstallOptions) error {
	if opts.AcceptEula.Value {
		return nil
	}

	fmt.Fprintf(opts.Out, "\nThe Octopus %s is covered by the Octopus Customer Agreement.\n  %s\n",
		opts.installedThing(), output.Blue(eulaURL))

	if err := opts.Ask(&survey.Confirm{
		Message: "Do you accept it?",
		Default: true,
	}, &opts.AcceptEula.Value); err != nil {
		return err
	}
	if !opts.AcceptEula.Value {
		return errEulaDeclined
	}
	return nil
}

func promptForDefaultNamespace(opts *InstallOptions) error {
	if opts.DefaultNamespace.Value != "" {
		return nil
	}

	return opts.Ask(&survey.Input{
		Message: "Default namespace for deployments (optional)",
		Help: "Used only when neither the step nor the manifest names a namespace. " +
			"Leave it blank to make every step say where it deploys to.",
	}, &opts.DefaultNamespace.Value)
}

func promptForPollingAddress(opts *InstallOptions) error {
	// Derived from the URL the CLI is logged in to, which is nearly always
	// right - but the port is configurable, and a proxy that terminates TLS
	// breaks the agent, so confirm it.
	return opts.Ask(&survey.Input{
		Message: "Octopus Server polling address",
		Default: opts.ServerCommsAddress.Value,
		Help: fmt.Sprintf("The agent polls Octopus over TCP, on port %d by default, separately from the REST API on 443. "+
			"Octopus Cloud serves this on its own hostname over 443. The connection has to reach Octopus intact - SSL offloading does not work.",
			octoK8s.DefaultPollingPort),
	}, &opts.ServerCommsAddress.Value, survey.WithValidator(survey.Required))
}

// promptForStorage asks only where the volume comes from. The access mode
// follows from the class, so it is worked out rather than asked about.
func promptForStorage(opts *InstallOptions) error {
	if opts.StorageClass.Value == "" && len(opts.StorageClasses) > 0 {
		if err := promptForStorageClass(opts); err != nil {
			return err
		}
	}

	opts.deriveAccessMode()
	return nil
}

func promptForStorageClass(opts *InstallOptions) error {
	const clusterDefault = "Use the cluster's default storage class"

	options := []*selectors.SelectOption[string]{{Display: clusterDefault, Value: ""}}
	for _, class := range opts.StorageClasses {
		options = append(options, &selectors.SelectOption[string]{Display: class.Display(), Value: class.Name})
	}

	selected, err := selectors.SelectOptions(opts.Ask, "Which storage class should the agent use?",
		func() []*selectors.SelectOption[string] { return options })
	if err != nil {
		return err
	}
	opts.StorageClass.Value = selected.Value
	return nil
}

// promptForScriptPodPermissions is only worth asking where the permissions
// controller can act on the answer. Without it, this is the only thing standing
// between a deployment and the cluster, and taking it away stops deployments.
func promptForScriptPodPermissions(opts *InstallOptions) error {
	if !opts.PermissionsController || opts.RestrictScriptPods.Value || len(opts.ScriptPodRoles.Value) > 0 {
		return nil
	}

	const (
		nothing  = "Nothing"
		anything = "Anything"
		copyRole = "Copy an existing role"
	)

	answer := ""
	if err := opts.Ask(&survey.Select{
		Message: "What permissions should be used by workloads out of any WSA scope?",
		Options: []string{nothing, anything, copyRole},
		Default: nothing,
		Help: "A workload that no WorkloadServiceAccount matches falls back to these. Granting nothing means such a " +
			"workload fails rather than running with more access than it should have; anything is the chart's own " +
			"default of the whole cluster.",
	}, &answer); err != nil {
		return err
	}

	switch answer {
	case nothing:
		opts.RestrictScriptPods.Value = true
		return nil
	case copyRole:
		return promptForScriptPodRoles(opts)
	default:
		return nil
	}
}

// promptForScriptPodRoles copies rules out of existing roles rather than
// binding to them, because that is what the chart takes. Saying so matters: the
// copy does not follow the roles afterwards.
func promptForScriptPodRoles(opts *InstallOptions) error {
	roles, err := opts.Cluster.Roles(context.Background())
	if err != nil {
		return err
	}
	if len(roles) == 0 {
		fmt.Fprintf(opts.Out, "  %s This cluster has no roles of its own to copy, so script pods keep the chart's default.\n",
			output.Yellow("!"))
		return nil
	}

	selected, err := question.MultiSelectMap(opts.Ask, "Which roles should they be given the rules of?", roles,
		func(role octoK8s.Role) string { return role.Display() }, true)
	if err != nil {
		return err
	}

	rules := octoK8s.MergePolicyRules(selected)
	if len(rules) == 0 {
		fmt.Fprintf(opts.Out, "  %s Those roles grant nothing, so script pods keep the chart's default.\n",
			output.Yellow("!"))
		return nil
	}

	references := make([]string, 0, len(selected))
	for _, role := range selected {
		references = append(references, role.Reference())
	}

	opts.ScriptPodRoles.Value = references
	opts.ScriptPodRules = rules
	fmt.Fprintf(opts.Out, "  %s\n", output.Dimf(
		"The %d %s are copied in now. Later changes to %s are not picked up.",
		len(rules), octoK8s.Pluralise("rule", "rules", len(rules)), strings.Join(references, ", ")))
	return nil
}

// reportFindings says what was already in the cluster and in Octopus, because
// both change what this install does: a name that is already in use upgrades an
// agent rather than adding one.
func (opts *InstallOptions) reportFindings() {
	opts.reportUnsupportedNodes()

	if existing, found := opts.existingRelease(); found {
		if existing.Mode != agentK8s.ModeUnknown && existing.Mode != opts.Mode {
			fmt.Fprintf(opts.Out, "%s Release %s in namespace %s is already a %s. Installing here would replace it; "+
				"give this one a different name.\n",
				output.Yellow("!"), output.Cyan(existing.Release.Name), output.Cyan(existing.Release.Namespace), existing.Mode)
		} else {
			fmt.Fprintf(opts.Out, "%s Upgrading the existing release %s in namespace %s (%s %s).\n",
				output.Dim("-"), output.Cyan(existing.Release.Name), output.Cyan(existing.Release.Namespace),
				existing.Release.Chart, existing.Release.Version)
		}
	}

	if opts.RegisteredCallback != nil {
		switch taken, err := opts.alreadyRegistered(); {
		case err != nil:
			fmt.Fprintf(opts.Out, "%s Could not check whether Octopus already has a %s named %s: %v\n",
				output.Yellow("!"), opts.Mode, output.Cyan(opts.Name.Value), err)
		case taken:
			fmt.Fprintf(opts.Out, "%s Octopus already has a %s named %s. The agent registers by name, so it will take that one over.\n",
				output.Yellow("!"), opts.Mode, output.Cyan(opts.Name.Value))
		}
	}

	if opts.PermissionsController {
		fmt.Fprintf(opts.Out, "%s The Octopus permissions controller is running in this cluster, so each deployment can be\n"+
			"  granted its own permissions by a WorkloadServiceAccount rather than sharing the agent's.\n", output.Dim("-"))
	}
}

func ReportFindingsForTest(opts *InstallOptions) {
	opts.reportFindings()
}
