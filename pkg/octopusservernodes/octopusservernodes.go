// Package octopusservernodes reads the Octopus Server cluster topology, which
// is how an installer learns it is talking to a High Availability cluster: a
// polling agent connects to every node, and each node needs its own address.
package octopusservernodes

import (
	"fmt"
	"time"

	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/newclient"
)

const path = "/api/octopusservernodes/summary"

type Node struct {
	Name string `json:"Name"`
	// MaxConcurrentTasks is zero for a node that only serves the web UI, which
	// a polling agent never needs to reach.
	MaxConcurrentTasks int  `json:"MaxConcurrentTasks"`
	IsOffline          bool `json:"IsOffline"`
	// LastSeen is nil when the node has never reported in.
	LastSeen *time.Time `json:"LastSeen"`
}

// decommissionedAfter matches the Octopus portal: an offline node not seen for
// this long is treated as removed rather than merely restarting.
const decommissionedAfter = 5 * 24 * time.Hour

// TaskNodes returns the nodes an agent would poll: the ones that run tasks and
// have not been gone long enough to call decommissioned.
func TaskNodes(client newclient.Client) ([]Node, error) {
	response, err := newclient.Get[struct {
		Nodes []Node `json:"Nodes"`
	}](client.HttpSession(), path)
	if err != nil {
		return nil, fmt.Errorf("could not read the Octopus Server's nodes: %w", err)
	}

	nodes := make([]Node, 0, len(response.Nodes))
	for _, node := range response.Nodes {
		if node.runsTasks() && !node.decommissioned() {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func (n Node) runsTasks() bool {
	return n.MaxConcurrentTasks != 0
}

func (n Node) decommissioned() bool {
	if !n.IsOffline {
		return false
	}
	return n.LastSeen == nil || time.Since(*n.LastSeen) > decommissionedAfter
}
