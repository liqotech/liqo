package foreigncluster

import (
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// AllowIncomingPeering returns the value set in the ForeignCluster spec if it has been set,
// it returns the value set in the ClusterConfig if it is automatic.
func AllowIncomingPeering(foreignCluster *discoveryv1alpha1.ForeignCluster, defaultEnableIncomingPeering bool) bool {
	switch foreignCluster.Spec.IncomingPeeringEnabled {
	case discoveryv1alpha1.PeeringEnabledYes:
		return true
	case discoveryv1alpha1.PeeringEnabledNo:
		return false
	case discoveryv1alpha1.PeeringEnabledAuto:
		return defaultEnableIncomingPeering
	default:
		klog.Warningf("invalid value for incomingPeeringEnabled field: %v", foreignCluster.Spec.IncomingPeeringEnabled)
		return false
	}
}
