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

package virtualnodectrl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// createNamespaceMap creates a new NamespaceMap with OwnerReference.
func (r *VirtualNodeReconciler) createNamespaceMap(ctx context.Context, n *corev1.Node) error {
	virtualNodeClusterID := n.Annotations[liqoconst.RemoteClusterID]
	nm := &mapsv1alpha1.NamespaceMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", virtualNodeClusterID),
			Namespace:    r.getLocalTenantNamespaceName(virtualNodeClusterID),
			Labels: map[string]string{
				liqoconst.RemoteClusterID: virtualNodeClusterID,
			},
		},
	}

	if err := ctrlutils.SetControllerReference(n, nm, r.Scheme); err != nil {
		return err
	}

	if err := r.Create(ctx, nm); err != nil {
		klog.Errorf("%s --> Problems in NamespaceMap creation for the virtual node '%s'", err, n.GetName())
		return err
	}
	klog.Infof("Create the NamespaceMap '%s' for the virtual node '%s'", nm.GetName(), n.GetName())
	return nil
}

// ensureNamespaceMapPresence creates a new NamespaceMap associated with that virtual-node if it is not already present.
func (r *VirtualNodeReconciler) ensureNamespaceMapPresence(ctx context.Context, n *corev1.Node) error {
	// Only when the NamespaceMap is created for the first time it is necessary to check the presence of the local
	// Tenant namespace's name.
	if err := r.checkLocalTenantNamespaceNamePresence(ctx, n.Annotations[liqoconst.RemoteClusterID]); err != nil {
		return err
	}
	nms := &mapsv1alpha1.NamespaceMapList{}
	if err := r.List(ctx, nms, client.InNamespace(r.getLocalTenantNamespaceName(n.Annotations[liqoconst.RemoteClusterID])),
		client.MatchingLabels{liqoconst.RemoteClusterID: n.Annotations[liqoconst.RemoteClusterID]}); err != nil {
		klog.Errorf("%s --> Unable to List NamespaceMaps of the virtual-node '%s'", err, n.GetName())
		return err
	}

	if len(nms.Items) == 0 {
		return r.createNamespaceMap(ctx, n)
	}

	return nil
}

// checkLocalTenantNamespaceNamePresence checks if the local tenant namespace's name for the cluster with
// `remoteClusterID` clusterID is already present in the map r.LocalTenantNamespacesNames.
func (r *VirtualNodeReconciler) checkLocalTenantNamespaceNamePresence(ctx context.Context, remoteClusterID string) error {
	if r.LocalTenantNamespacesNames == nil {
		r.LocalTenantNamespacesNames = map[string]string{}
	}

	if _, ok := r.LocalTenantNamespacesNames[remoteClusterID]; !ok {
		fc, err := foreignclusterutils.GetForeignClusterByID(ctx, r.Client, remoteClusterID)
		if err != nil {
			return err
		}

		if fc.Status.TenantNamespace.Local == "" {
			err = fmt.Errorf("there is no tenant namespace associated with the peering with the remote cluster '%s'",
				remoteClusterID)
			klog.Error(err)
			return err
		}

		r.LocalTenantNamespacesNames[remoteClusterID] = fc.Status.TenantNamespace.Local
		klog.Infof("The Tenant namespace '%s' associated with the peering with the remote cluster '%s' is added to the Map",
			fc.Status.TenantNamespace.Local, remoteClusterID)
	}
	return nil
}

// getLocalTenantNamespaceName provides the name of the local tenant namespace, given the remoteClusterID
// associated with a peering.
func (r *VirtualNodeReconciler) getLocalTenantNamespaceName(remoteClusterID string) string {
	return r.LocalTenantNamespacesNames[remoteClusterID]
}
