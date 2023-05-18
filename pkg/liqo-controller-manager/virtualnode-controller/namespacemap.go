// Copyright 2019-2023 The Liqo Authors
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

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// ensureNamespaceMapPresence creates a new NamespaceMap associated with that virtual-node if it is not already present.
func (r *VirtualNodeReconciler) ensureNamespaceMapPresence(ctx context.Context, vn *virtualkubeletv1alpha1.VirtualNode) error {
	l := map[string]string{
		liqoconst.RemoteClusterID:             vn.Spec.ClusterIdentity.ClusterID,
		liqoconst.ReplicationRequestedLabel:   strconv.FormatBool(true),
		liqoconst.ReplicationDestinationLabel: vn.Spec.ClusterIdentity.ClusterID,
	}
	nm, err := getters.GetNamespaceMapByLabel(ctx, r.Client, vn.Namespace, labels.SelectorFromSet(l))
	if err != nil && !kerrors.IsNotFound(err) {
		klog.Errorf("%s -> unable to retrieve NamespaceMap %q", err, nm.Name)
		return err
	} else if err == nil {
		klog.Infof("NamespaceMap %q already exists in namespace %s", nm.Name, nm.Namespace)
		return nil
	}

	nm = &virtualkubeletv1alpha1.NamespaceMap{ObjectMeta: metav1.ObjectMeta{
		Name: foreignclusterutils.UniqueName(vn.Spec.ClusterIdentity), Namespace: vn.Namespace}}

	result, err := ctrlutils.CreateOrUpdate(ctx, r.Client, nm, func() error {
		nm.Labels = labels.Merge(nm.Labels, l)

		return ctrlutils.SetControllerReference(vn, nm, r.Scheme)
	})

	if err != nil {
		klog.Errorf("Failed to enforce NamespaceMap %q: %v", klog.KObj(nm), err)
		return err
	}

	klog.V(utils.FromResult(result)).Infof("NamespaceMap %q successfully enforced (with %v operation)", klog.KObj(nm), result)
	return nil
}

// removeAssociatedNamespaceMaps forces the deletion of virtual-node's NamespaceMaps before deleting it.
func (r *VirtualNodeReconciler) ensureNamespaceMapAbsence(ctx context.Context, vn *virtualkubeletv1alpha1.VirtualNode) error {
	// The deletion timestamp is automatically set on the NamespaceMaps associated with the virtual-node,
	// it's only necessary to wait until the NamespaceMaps are deleted.
	virtualNodesList, err := getters.ListVirtualNodesByLabels(ctx, r.Client, labels.Everything())
	if err != nil {
		klog.Errorf("%s -> Unable to List VirtualNodes", err)
		return err
	}
	if len(virtualNodesList.Items) != 1 {
		return nil
	}

	namespaceMapList := &virtualkubeletv1alpha1.NamespaceMapList{}
	virtualNodeRemoteClusterID := vn.Spec.ClusterIdentity.ClusterID
	if err := r.List(ctx, namespaceMapList, client.InNamespace(vn.Namespace),
		client.MatchingLabels{liqoconst.ReplicationDestinationLabel: virtualNodeRemoteClusterID}); err != nil {
		klog.Errorf("%s -> Unable to List NamespaceMaps of virtual node %q", err, vn.Name)
		return err
	}

	for i := range namespaceMapList.Items {
		if namespaceMapList.Items[i].GetDeletionTimestamp().IsZero() {
			if err := r.Delete(ctx, &namespaceMapList.Items[i]); err != nil {
				klog.Errorf("%s -> unable to delete the NamespaceMap %q", err, namespaceMapList.Items[i].Name)
			}
		}
	}

	return nil
}
