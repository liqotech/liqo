package foreigncluster

import (
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
)

// HasToBeRemoved indicates if a ForeignCluster CR has to be removed.
func HasToBeRemoved(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	isIncomingDiscovery := GetDiscoveryType(foreignCluster) == discovery.IncomingPeeringDiscovery
	hasPeering := IsIncomingEnabled(foreignCluster) || IsOutgoingEnabled(foreignCluster)
	return isIncomingDiscovery && !hasPeering
}
