package crdreplicator

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// checkResourcesOnPeeringPhaseChange checks if some of the replicated resources
// need to start replication of this specific ForeignCluster on this phase change.
// If some of the replicated resources need to start replication, it lists all the
// instances already that are already present in the local cluster and calls the
// AddHandler on them.
func (c *Controller) checkResourcesOnPeeringPhaseChange(ctx context.Context,
	remoteClusterID string, currentPhase, oldPhase consts.PeeringPhase) {
	for i := range c.RegisteredResources {
		res := &c.RegisteredResources[i]
		if !isReplicationEnabled(oldPhase, res) && isReplicationEnabled(currentPhase, res) {
			// this change has triggered the replication on this resource
			klog.Infof("phase from %v to %v triggers replication on resource %v",
				oldPhase, currentPhase, res.GroupVersionResource)
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
	remoteClusterID string, res *configv1alpha1.Resource) error {
	localNamespace, err := c.clusterIDToLocalNamespace(remoteClusterID)
	if err != nil {
		klog.Error(err)
		return err
	}

	listOptions := metav1.ListOptions{}
	SetLabelsForLocalResources(&listOptions)

	// this change has triggered the replication on this resource
	resources, err := c.LocalDynClient.Resource(convertGVR(res.GroupVersionResource)).
		Namespace(localNamespace).List(ctx, listOptions)
	if err != nil {
		klog.Error(err)
		return err
	}
	c.addListHandler(resources, convertGVR(res.GroupVersionResource))
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

// getPeeringPhase returns the peering phase for a fiver ForignCluster CR.
func getPeeringPhase(fc *discoveryv1alpha1.ForeignCluster) consts.PeeringPhase {
	incoming := foreigncluster.IsIncomingEnabled(fc)
	outgoing := foreigncluster.IsOutgoingEnabled(fc)
	if incoming && outgoing {
		return consts.PeeringPhaseBidirectional
	}
	if incoming {
		return consts.PeeringPhaseIncoming
	}
	if outgoing {
		return consts.PeeringPhaseOutgoing
	}
	return consts.PeeringPhaseNone
}

// isReplicationEnabled indicates if the replication has to be enabled for a given peeringPhase
// and a given CRD.
func isReplicationEnabled(peeringPhase consts.PeeringPhase, resource *configv1alpha1.Resource) bool {
	switch resource.PeeringPhase {
	case consts.PeeringPhaseNone:
		return false
	case consts.PeeringPhaseAll:
		return true
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
		klog.Info("unknown peeringPhase %v", resource.PeeringPhase)
		return false
	}
}
