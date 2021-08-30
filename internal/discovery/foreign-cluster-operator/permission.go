package foreignclusteroperator

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

func (r *ForeignClusterReconciler) ensurePermission(ctx context.Context, foreignCluster *discoveryv1alpha1.ForeignCluster) (err error) {
	remoteClusterID := foreignCluster.Spec.ClusterIdentity.ClusterID
	peeringPhase := foreignclusterutils.GetPeeringPhase(foreignCluster)

	if _, err = r.namespaceManager.BindClusterRoles(remoteClusterID, r.peeringPermission.Basic...); err != nil {
		klog.Error(err)
		return err
	}

	switch peeringPhase {
	case consts.PeeringPhaseNone, consts.PeeringPhaseAuthenticated:
		if err = r.namespaceManager.UnbindClusterRoles(remoteClusterID,
			clusterRolesToNames(r.peeringPermission.Outgoing)...); err != nil {
			klog.Error(err)
			return err
		}
		if err = r.namespaceManager.UnbindClusterRoles(remoteClusterID,
			clusterRolesToNames(r.peeringPermission.Incoming)...); err != nil {
			klog.Error(err)
			return err
		}
	case consts.PeeringPhaseOutgoing:
		if _, err = r.namespaceManager.BindClusterRoles(remoteClusterID,
			r.peeringPermission.Outgoing...); err != nil {
			klog.Error(err)
			return err
		}
		if err = r.namespaceManager.UnbindClusterRoles(remoteClusterID,
			clusterRolesToNames(r.peeringPermission.Incoming)...); err != nil {
			klog.Error(err)
			return err
		}
	case consts.PeeringPhaseIncoming:
		if err = r.namespaceManager.UnbindClusterRoles(remoteClusterID,
			clusterRolesToNames(r.peeringPermission.Outgoing)...); err != nil {
			klog.Error(err)
			return err
		}
		if _, err = r.namespaceManager.BindClusterRoles(remoteClusterID,
			r.peeringPermission.Incoming...); err != nil {
			klog.Error(err)
			return err
		}
	case consts.PeeringPhaseBidirectional:
		if _, err = r.namespaceManager.BindClusterRoles(remoteClusterID,
			r.peeringPermission.Outgoing...); err != nil {
			klog.Error(err)
			return err
		}
		if _, err = r.namespaceManager.BindClusterRoles(remoteClusterID,
			r.peeringPermission.Incoming...); err != nil {
			klog.Error(err)
			return err
		}
	default:
		err = fmt.Errorf("invalid PeeringPhase %v", peeringPhase)
		klog.Error(err)
		return err
	}
	return nil
}

func clusterRolesToNames(clusterRoles []*rbacv1.ClusterRole) []string {
	res := make([]string, len(clusterRoles))
	for i := range clusterRoles {
		res[i] = clusterRoles[i].Name
	}
	return res
}
