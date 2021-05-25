package routing

import netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"

const (
	routingTableID = 18952
)

// Routing interface used to configure the routing rules for peering clusters.
type Routing interface {
	EnsureRoutesPerCluster(tep *netv1alpha1.TunnelEndpoint) (bool, error)
	RemoveRoutesPerCluster(tep *netv1alpha1.TunnelEndpoint) (bool, error)
	CleanRoutingTable() error
	CleanPolicyRules() error
}
