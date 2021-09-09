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
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// removeAssociatedNamespaceMaps forces the deletion of virtual-node's NamespaceMaps before deleting it.
func (r *VirtualNodeReconciler) removeAssociatedNamespaceMaps(ctx context.Context, n *corev1.Node) error {
	klog.Infof("The virtual virtualNode '%s' is requested to be deleted", n.GetName())

	// The deletion timestamp is automatically set on the NamespaceMaps associated with the virtual-node,
	// it's only necessary to wait until the NamespaceMaps are deleted.
	namespaceMapList := &mapsv1alpha1.NamespaceMapList{}
	virtualNodeClusterID := n.Annotations[liqoconst.RemoteClusterID]
	if err := r.List(ctx, namespaceMapList,
		client.InNamespace(r.getLocalTenantNamespaceName(virtualNodeClusterID)),
		client.MatchingLabels{liqoconst.RemoteClusterID: virtualNodeClusterID}); err != nil {
		klog.Errorf("%s --> Unable to List NamespaceMaps of virtual virtualNode '%s'", err, n.GetName())
		return err
	}

	if len(namespaceMapList.Items) == 0 {
		delete(r.LocalTenantNamespacesNames, virtualNodeClusterID)
		return r.removeVirtualNodeFinalizer(ctx, n)
	}

	for i := range namespaceMapList.Items {
		if namespaceMapList.Items[i].GetDeletionTimestamp().IsZero() {
			if err := r.Delete(ctx, &namespaceMapList.Items[i]); err != nil {
				klog.Errorf("%s -> unable to delete the NamespaceMap '%s'", err, namespaceMapList.Items[i].Name)
			}
		}
	}

	err := fmt.Errorf("waiting for deletion of NamespaceMaps associated with the virtual-node '%s'", n.Name)
	klog.Error(err)
	return err
}
