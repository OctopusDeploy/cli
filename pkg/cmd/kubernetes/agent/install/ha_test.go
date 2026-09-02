package install_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/OctopusDeploy/cli/pkg/cmd/kubernetes/agent/install"
	agentK8s "github.com/OctopusDeploy/cli/pkg/kubernetes/agent"
	"github.com/OctopusDeploy/cli/pkg/octopusservernodes"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const selfHostedHost = "https://octopus.internal"

func haNodes() []octopusservernodes.Node {
	return []octopusservernodes.Node{
		{Name: "OCTOPUS-01", MaxConcurrentTasks: 5},
		{Name: "OCTOPUS-02", MaxConcurrentTasks: 5},
	}
}

// A High Availability node has no derivable polling address: the agent polls
// every node, each on its own address, and only the person who set the cluster
// up knows what those are.
func TestResolveWithoutPrompting_HANeedsAnAddressPerNode(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedTargetFlags()
	flags.ServerCommsAddresses.Value = nil
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)
	opts.Host = selfHostedHost
	opts.ServerNodesCallback = func() ([]octopusservernodes.Node, error) { return haNodes(), nil }

	err := opts.ResolveWithoutPrompting()
	assert.ErrorContains(t, err, "High Availability")
	assert.ErrorContains(t, err, "OCTOPUS-01, OCTOPUS-02")
	assert.ErrorContains(t, err, install.FlagServerCommsAddress)
}

// Addresses given explicitly are taken at face value, however many nodes there
// are: the person installing may deliberately point an agent at a subset.
func TestResolveWithoutPrompting_HAAcceptsTheAddressesGiven(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedTargetFlags()
	flags.ServerCommsAddresses.Value = []string{"https://node-1.internal:10943", "https://node-2.internal:10943"}
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)
	opts.Host = selfHostedHost
	opts.ServerNodesCallback = func() ([]octopusservernodes.Node, error) { return haNodes(), nil }

	require.NoError(t, opts.ResolveWithoutPrompting())

	values, err := opts.BuildValues()
	require.NoError(t, err)
	assert.Equal(t, []string{"https://node-1.internal:10943", "https://node-2.internal:10943"},
		values["agent"].(map[string]any)["serverCommsAddresses"])
}

func TestPromptMissing_HAAsksForEveryNodesAddress(t *testing.T) {
	nodeHelp := "The agent polls each node over TCP, on port 10943 by default, separately from the REST API on 443. " +
		"The connection has to reach that node intact - SSL offloading does not work."

	pa := []*testutil.PA{
		testutil.NewConfirmPromptWithDefault("Do you accept it?", "", true, true),
		testutil.NewInputPrompt("Name", "A short, memorable, unique name for this deployment target.", "Production"),
		testutil.NewMultiSelectPrompt("Choose at least one environment for the deployment target.\n", "",
			[]string{"Development", "Production"}, []string{"Production"}),
		testutil.NewMultiSelectWithAddPrompt("Which target tags should this deployment target have?\n", "",
			[]string{"k8s", "web"}, []string{"k8s"}, "tag"),
		testutil.NewInputPrompt("Default namespace for deployments (optional)",
			"Used only when neither the step nor the manifest names a namespace. "+
				"Leave it blank to make every step say where it deploys to.", ""),
		testutil.NewInputPrompt("Polling address for node OCTOPUS-01", nodeHelp, "https://node-1.internal:10943"),
		testutil.NewInputPrompt("Polling address for node OCTOPUS-02", nodeHelp, "https://node-2.internal:10943"),
		testutil.NewSelectPrompt("Which storage class should the agent use?", "",
			[]string{
				"Use the cluster's default storage class",
				"standard (cluster default, kubernetes.io/gce-pd)",
				"filestore (filestore.csi.storage.gke.io)",
			}, "Use the cluster's default storage class"),
	}
	asker, checkRemainingPrompts := testutil.NewMockAsker(t, pa)

	flags := install.NewInstallFlags()
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)
	opts.Host = selfHostedHost
	opts.ServerNodesCallback = func() ([]octopusservernodes.Node, error) { return haNodes(), nil }

	require.NoError(t, install.PromptMissing(context.Background(), opts))
	checkRemainingPrompts()

	assert.Equal(t, []string{"https://node-1.internal:10943", "https://node-2.internal:10943"},
		flags.ServerCommsAddresses.Value)
}

// Octopus Cloud serves every polling connection on one shared address, so its
// nodes are its own business and never worth a question.
func TestResolveWithoutPrompting_CloudNeverReadsTheTopology(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedTargetFlags()
	flags.ServerCommsAddresses.Value = nil
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)
	opts.ServerNodesCallback = func() ([]octopusservernodes.Node, error) {
		t.Fatal("Octopus Cloud topology should not be read")
		return nil, nil
	}

	require.NoError(t, opts.ResolveWithoutPrompting())
	assert.Equal(t, []string{pollingAddress}, flags.ServerCommsAddresses.Value)
}

// The topology read is a convenience: a credential that cannot read it can
// still name the polling addresses itself, so the install carries on with the
// derived one.
func TestResolveWithoutPrompting_UnreadableTopologyFallsBackToOneAddress(t *testing.T) {
	asker, _ := testutil.NewMockAsker(t, []*testutil.PA{})

	flags := allSuppliedTargetFlags()
	flags.ServerCommsAddresses.Value = nil
	opts := newOptions(t, flags, agentK8s.ModeDeploymentTarget, asker)
	opts.Host = selfHostedHost
	opts.ServerNodesCallback = func() ([]octopusservernodes.Node, error) { return nil, assert.AnError }

	require.NoError(t, opts.ResolveWithoutPrompting())
	assert.Equal(t, []string{"https://octopus.internal:10943"}, flags.ServerCommsAddresses.Value)
	assert.Contains(t, opts.Out.(*bytes.Buffer).String(), "Could not read the Octopus Server's nodes")
}
