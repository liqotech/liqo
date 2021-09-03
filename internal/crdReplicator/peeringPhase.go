package crdreplicator

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/consts"
)

// checkResourcesOnPeeringPhaseChange checks if some of the replicated resources
// need to start replication of this specific ForeignCluster on this phase change.
// If some of the replicated resources need to start replication, it lists all the
// instances that are already present in the local cluster and calls the
// AddHandler on them.
func (c *Controller) checkResourcesOnPeeringPhaseChange(ctx context.Context,
	remoteClusterID string, currentPhase, oldPhase consts.PeeringPhase) {
	for i := range c.RegisteredResources {
		res := &c.RegisteredResources[i]
		if !isReplicationEnabled(oldPhase, res) && isReplicationEnabled(currentPhase, res) {
			// this change has triggered the replication on this resource
			klog.Infof("%v -> phase from %v to %v triggers replication of resource %v",
				remoteClusterID, oldPhase, currentPhase, res.GroupVersionResource)
			if err := c.startResourceReplicationHandler(ctx, remoteClusterID, res); err != nil {
				klog.Error(err)
				continue
			}
		}
	}
}

// startResourceReplicationHandler lists all the instances already that are already present
// in the local cluster and calls the AddHandler on them.
func (c *Controller) startResourceReplicationHandler(ctx context.Context,
	remoteClusterID string, res *Resource) error {
	localNamespace, err := c.clusterIDToLocalNamespace(remoteClusterID)
	if err != nil {
		klog.Error(err)
		return err
	}

	listOptions := metav1.ListOptions{}
	SetLabelsForLocalResources(&listOptions)

	// this change has triggered the replication on this resource
	resources, err := c.LocalDynClient.Resource(res.GroupVersionResource).
		Namespace(localNamespace).List(ctx, listOptions)
	if err != nil {
		klog.Error(err)
		return err
	}
	c.addListHandler(resources, res.GroupVersionResource)
	return nil
}

// addListHandler calls the AddHandler on a list of resources.
func (c *Controller) addListHandler(list *unstructured.UnstructuredList, gvr schema.GroupVersionResource) {
	for i := range list.Items {
		item := &list.Items[i]
		klog.V(4).Infof("replicating %v %v/%v", item.GetKind(), item.GetNamespace(), item.GetName())
		c.AddedHandler(item, gvr)
	}
}

// getPeeringPhase returns the peering phase for a cluster given its clusterID.
func (c *Controller) getPeeringPhase(clusterID string) consts.PeeringPhase {
	c.peeringPhasesMutex.RLock()
	defer c.peeringPhasesMutex.RUnlock()
	if c.peeringPhases == nil {
		return consts.PeeringPhaseNone
	}
	if phase, ok := c.peeringPhases[clusterID]; ok {
		return phase
	}
	return consts.PeeringPhaseNone
}

// setPeeringPhase sets the peering phase for a given clusterID.
func (c *Controller) setPeeringPhase(clusterID string, phase consts.PeeringPhase) {
	c.peeringPhasesMutex.Lock()
	defer c.peeringPhasesMutex.Unlock()
	if c.peeringPhases == nil {
		c.peeringPhases = map[string]consts.PeeringPhase{}
	}
	c.peeringPhases[clusterID] = phase
}

// isReplicationEnabled indicates if the replication has to be enabled for a given peeringPhase
// and a given CRD.
func isReplicationEnabled(peeringPhase consts.PeeringPhase, resource *Resource) bool {
	switch resource.PeeringPhase {
	case consts.PeeringPhaseNone:
		return false
	case consts.PeeringPhaseAuthenticated:
		return peeringPhase != consts.PeeringPhaseNone
	case consts.PeeringPhaseBidirectional:
		return peeringPhase == consts.PeeringPhaseBidirectional
	case consts.PeeringPhaseIncoming:
		return peeringPhase == consts.PeeringPhaseBidirectional || peeringPhase == consts.PeeringPhaseIncoming
	case consts.PeeringPhaseOutgoing:
		return peeringPhase == consts.PeeringPhaseBidirectional || peeringPhase == consts.PeeringPhaseOutgoing
	case consts.PeeringPhaseEstablished:
		bidirectional := peeringPhase == consts.PeeringPhaseBidirectional
		incoming := peeringPhase == consts.PeeringPhaseIncoming
		outgoing := peeringPhase == consts.PeeringPhaseOutgoing
		return bidirectional || incoming || outgoing
	default:
		klog.Warning("Unknown peering phase %v", resource.PeeringPhase)
		return false
	}
}
