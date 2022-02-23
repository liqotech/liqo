// Copyright 2019-2022 The Liqo Authors
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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// ensureNamespaceMapPresence creates a new NamespaceMap associated with that virtual-node if it is not already present.
func (r *VirtualNodeReconciler) ensureNamespaceMapPresence(ctx context.Context, fc *discoveryv1alpha1.ForeignCluster, n *corev1.Node) error {
	nm := mapsv1alpha1.NamespaceMap{ObjectMeta: metav1.ObjectMeta{
		Name: foreignclusterutils.UniqueName(&fc.Spec.ClusterIdentity), Namespace: fc.Status.TenantNamespace.Local}}

	result, err := ctrlutils.CreateOrUpdate(ctx, r.Client, &nm, func() error {
		nm.Labels = labels.Merge(nm.Labels, map[string]string{
			liqoconst.RemoteClusterID:             fc.Spec.ClusterIdentity.ClusterID,
			liqoconst.ReplicationRequestedLabel:   strconv.FormatBool(true),
			liqoconst.ReplicationDestinationLabel: fc.Spec.ClusterIdentity.ClusterID,
		})

		return ctrlutils.SetControllerReference(n, &nm, r.Scheme)
	})

	if err != nil {
		klog.Errorf("Failed to enforce NamespaceMap %q: %v", klog.KObj(&nm), err)
		return err
	}

	klog.V(utils.FromResult(result)).Infof("NamespaceMap %q successfully enforced (with %v operation)", klog.KObj(&nm), result)
	return nil
}

// removeAssociatedNamespaceMaps forces the deletion of virtual-node's NamespaceMaps before deleting it.
func (r *VirtualNodeReconciler) ensureNamespaceMapAbsence(ctx context.Context, fc *discoveryv1alpha1.ForeignCluster, n *corev1.Node) error {
	// The deletion timestamp is automatically set on the NamespaceMaps associated with the virtual-node,
	// it's only necessary to wait until the NamespaceMaps are deleted.
	namespaceMapList := &mapsv1alpha1.NamespaceMapList{}
	virtualNodeClusterID := n.Labels[liqoconst.RemoteClusterID]
	if err := r.List(ctx, namespaceMapList, client.InNamespace(fc.Status.TenantNamespace.Local),
		client.MatchingLabels{liqoconst.ReplicationDestinationLabel: virtualNodeClusterID}); err != nil {
		klog.Errorf("%s -> Unable to List NamespaceMaps of virtual node %q", err, n.GetName())
		return err
	}

	if len(namespaceMapList.Items) == 0 {
		return r.removeVirtualNodeFinalizer(ctx, n)
	}

	for i := range namespaceMapList.Items {
		if namespaceMapList.Items[i].GetDeletionTimestamp().IsZero() {
			if err := r.Delete(ctx, &namespaceMapList.Items[i]); err != nil {
				klog.Errorf("%s -> unable to delete the NamespaceMap %q", err, namespaceMapList.Items[i].Name)
			}
		}
	}

	err := fmt.Errorf("waiting for deletion of NamespaceMaps associated with virtual node %q", n.Name)
	klog.Info(err)
	return err
}
