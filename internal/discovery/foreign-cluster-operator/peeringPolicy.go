package foreignclusteroperator

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

type desiredPeeringPhase string

const (
	desiredPeeringPhasePeering   desiredPeeringPhase = "Peering"
	desiredPeeringPhaseUnpeering desiredPeeringPhase = "Unpeering"
)

// getDesiredOutgoingPeeringState returns the desired state for the outgoing peering basing on the ForeignCluster resource.
func (r *ForeignClusterReconciler) getDesiredOutgoingPeeringState(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) desiredPeeringPhase {
	outgoingPeeringEnabled, err := r.isOutgoingPeeringEnabled(ctx, foreignCluster)
	if err != nil {
		klog.Error(err)
		return desiredPeeringPhaseUnpeering
	}

	remoteNamespace := foreignCluster.Status.TenantNamespace.Remote
	if remoteNamespace != "" && outgoingPeeringEnabled {
		return desiredPeeringPhasePeering
	}
	return desiredPeeringPhaseUnpeering
}

func (r *ForeignClusterReconciler) isOutgoingPeeringEnabled(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) (bool, error) {
	switch foreignCluster.Spec.OutgoingPeeringEnabled {
	case discoveryv1alpha1.PeeringEnabledNo:
		return false, nil
	case discoveryv1alpha1.PeeringEnabledYes:
		return true, nil
	case discoveryv1alpha1.PeeringEnabledAuto:
		if !r.ConfigProvider.GetConfig().AutoJoin {
			return false, nil
		}

		discoveryType := foreignclusterutils.GetDiscoveryType(foreignCluster)
		switch discoveryType {
		case discovery.LanDiscovery:
			return true, nil
		case discovery.WanDiscovery:
			searchDomain, err := r.getSearchDomain(ctx, foreignCluster)
			if err != nil {
				klog.Error(err)
				return false, err
			}
			return searchDomain.Spec.AutoJoin, nil
		case discovery.ManualDiscovery, discovery.IncomingPeeringDiscovery:
			return false, nil
		}
	}

	return false, nil
}

func (r *ForeignClusterReconciler) getSearchDomain(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) (*discoveryv1alpha1.SearchDomain, error) {
	for i := range foreignCluster.OwnerReferences {
		own := &foreignCluster.OwnerReferences[i]
		if own.Kind == "SearchDomain" {
			var searchDomain discoveryv1alpha1.SearchDomain
			if err := r.Client.Get(ctx, types.NamespacedName{
				Name: own.Name,
			}, &searchDomain); err != nil {
				klog.Error(err)
				return nil, err
			}
		}
	}

	return nil, apierrors.NewNotFound(discoveryv1alpha1.SearchDomainGroupResource, "")
}
