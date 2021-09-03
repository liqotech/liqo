package crdreplicator

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

// Resource contains a list of resources identified by their GVR.
type Resource struct {
	// GroupVersionResource contains the GVR of the resource to replicate.
	GroupVersionResource schema.GroupVersionResource
	// PeeringPhase contains the peering phase when this resource should be replicated.
	PeeringPhase consts.PeeringPhase
	// Ownership indicates the ownership over this resource.
	Ownership consts.OwnershipType
}

// GetResourcesToReplicate returns the list of resources to be replicated through the CRD replicator.
func GetResourcesToReplicate() []Resource {
	return []Resource{
		{
			GroupVersionResource: discoveryv1alpha1.ResourceRequestGroupVersionResource,
			PeeringPhase:         consts.PeeringPhaseAuthenticated,
			Ownership:            consts.OwnershipShared,
		},
		{
			GroupVersionResource: sharingv1alpha1.ResourceOfferGroupVersionResource,
			PeeringPhase:         consts.PeeringPhaseIncoming,
			Ownership:            consts.OwnershipShared,
		},
		{
			GroupVersionResource: netv1alpha1.NetworkConfigGroupVersionResource,
			PeeringPhase:         consts.PeeringPhaseEstablished,
			Ownership:            consts.OwnershipShared,
		},
	}
}
