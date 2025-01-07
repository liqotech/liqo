// Copyright 2019-2025 The Liqo Authors
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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// ensureNamespaceMapPresence creates a new NamespaceMap associated with that virtual-node if it is not already present.
func (r *VirtualNodeReconciler) ensureNamespaceMapPresence(ctx context.Context, vn *offloadingv1beta1.VirtualNode) error {
	l := map[string]string{
		liqoconst.RemoteClusterID:             string(vn.Spec.ClusterID),
		liqoconst.ReplicationRequestedLabel:   strconv.FormatBool(true),
		liqoconst.ReplicationDestinationLabel: string(vn.Spec.ClusterID),
	}
	nm, err := getters.GetNamespaceMapByLabel(ctx, r.Client, corev1.NamespaceAll, labels.SelectorFromSet(l))
	if err != nil && !kerrors.IsNotFound(err) {
		klog.Errorf("%s -> unable to retrieve NamespaceMap %q", err, nm.Name)
		return err
	} else if err == nil {
		klog.V(4).Infof("NamespaceMap %q already exists", nm.Name)
		return nil
	}

	nmNamespace, err := r.namespaceManager.GetNamespace(ctx, vn.Spec.ClusterID)
	if err != nil {
		klog.Errorf("%s -> unable to retrieve tenant namespace for cluster %q", err, vn.Spec.ClusterID)
		return err
	}

	nm = &offloadingv1beta1.NamespaceMap{ObjectMeta: metav1.ObjectMeta{
		Name:      foreignclusterutils.UniqueName(vn.Spec.ClusterID),
		Namespace: nmNamespace.Name,
	}}

	result, err := ctrlutils.CreateOrUpdate(ctx, r.Client, nm, func() error {
		nm.Labels = labels.Merge(nm.Labels, l)
		return nil
	})

	if err != nil {
		klog.Errorf("Failed to enforce NamespaceMap %q: %v", klog.KObj(nm), err)
		return err
	}

	klog.V(utils.FromResult(result)).Infof("NamespaceMap %q successfully enforced (with %v operation)", klog.KObj(nm), result)
	return nil
}

// removeAssociatedNamespaceMaps forces the deletion of virtual-node's NamespaceMaps before deleting it.
func (r *VirtualNodeReconciler) ensureNamespaceMapAbsence(ctx context.Context, vn *offloadingv1beta1.VirtualNode) error {
	namespaceMapList := &offloadingv1beta1.NamespaceMapList{}
	virtualNodeRemoteClusterID := vn.Spec.ClusterID
	if err := r.List(ctx, namespaceMapList, client.InNamespace(corev1.NamespaceAll),
		client.MatchingLabels{liqoconst.ReplicationDestinationLabel: string(virtualNodeRemoteClusterID)}); err != nil {
		klog.Errorf("unable to List NamespaceMaps of virtual node %q: %s", client.ObjectKeyFromObject(vn), err)
		return err
	}

	for i := range namespaceMapList.Items {
		nm := &namespaceMapList.Items[i]

		// Retrieve all the VirtualNodes associated with the NamespaceMap.
		virtualNodes, err := getters.ListVirtualNodesByClusterID(ctx, r.Client, virtualNodeRemoteClusterID)
		if err != nil {
			klog.Errorf("%s -> unable to retrieve VirtualNodes for clusterID %q", err, virtualNodeRemoteClusterID)
			return err
		}

		// Delete the NamespaceMap only if there is only one VirtualNode remaining, which will be deleted in this routine.
		if len(virtualNodes) == 1 && nm.GetDeletionTimestamp().IsZero() {
			if err := client.IgnoreNotFound(r.Delete(ctx, nm)); err != nil {
				klog.Errorf("%s -> unable to delete the NamespaceMap %q", err, nm.Name)
				return err
			}
			klog.Infof("Ensured NamespaceMap %q absence", nm.Name)
		}
	}

	return nil
}
