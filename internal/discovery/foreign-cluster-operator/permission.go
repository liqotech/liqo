// Copyright 2019-2021 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		if r.ownerReferencesPermissionEnforcement {
			if err = r.namespaceManager.UnbindOutgoingClusterWideRole(ctx, remoteClusterID); err != nil {
				klog.Error(err)
				return err
			}
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
		if r.ownerReferencesPermissionEnforcement {
			if _, err = r.namespaceManager.BindOutgoingClusterWideRole(ctx, remoteClusterID); err != nil {
				klog.Error(err)
				return err
			}
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
		if r.ownerReferencesPermissionEnforcement {
			if err = r.namespaceManager.UnbindOutgoingClusterWideRole(ctx, remoteClusterID); err != nil {
				klog.Error(err)
				return err
			}
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
		if r.ownerReferencesPermissionEnforcement {
			if _, err = r.namespaceManager.BindOutgoingClusterWideRole(ctx, remoteClusterID); err != nil {
				klog.Error(err)
				return err
			}
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
