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

package namespaceoffloadingctrl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	k8shelper "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

func (r *NamespaceOffloadingReconciler) enforceClusterSelector(ctx context.Context, noff *offv1alpha1.NamespaceOffloading,
	clusterIDMap map[string]*mapsv1alpha1.NamespaceMap) error {
	virtualNodes := &corev1.NodeList{}
	if err := r.List(ctx, virtualNodes,
		client.MatchingLabels{liqoconst.TypeLabel: liqoconst.TypeNode}); err != nil {
		klog.Error(err, " --> Unable to List all virtual nodes")
		return err
	}

	// If here there are no virtual nodes is an error because it means that in the cluster there are NamespaceMap
	// but not their associated virtual nodes
	if len(virtualNodes.Items) != len(clusterIDMap) {
		err := fmt.Errorf(" No VirtualNodes at the moment in the cluster")
		klog.Error(err)
		return err
	}

	errorCondition := false
	for i := range virtualNodes.Items {
		match, err := k8shelper.MatchNodeSelectorTerms(&virtualNodes.Items[i], &noff.Spec.ClusterSelector)
		if err != nil {
			klog.Infof("%s -> Unable to offload the namespace '%s', there is an error in ClusterSelectorField",
				err, noff.Namespace)
			patch := noff.DeepCopy()
			if noff.Annotations == nil {
				noff.Annotations = map[string]string{}
			}
			noff.Annotations[liqoconst.SchedulingLiqoLabel] = fmt.Sprintf("Invalid Cluster Selector : %s", err)
			if err = r.Patch(ctx, noff, client.MergeFrom(patch)); err != nil {
				klog.Errorf("%s -> unable to add the liqo scheduling annotation to the NamespaceOffloading in the namespace '%s'",
					err, noff.Namespace)
				return err
			}
			klog.Infof("The liqo scheduling annotation is correctly added to the NamespaceOffloading in the namespace '%s'",
				noff.Namespace)
			break
		}
		if match {
			if err = addDesiredMapping(ctx, r.Client, noff.Namespace, noff.Status.RemoteNamespaceName,
				clusterIDMap[virtualNodes.Items[i].Labels[liqoconst.RemoteClusterID]]); err != nil {
				errorCondition = true
				continue
			}
			delete(clusterIDMap, virtualNodes.Items[i].Labels[liqoconst.RemoteClusterID])
		}
	}
	if errorCondition {
		err := fmt.Errorf("some desiredMappings has not been added")
		klog.Error(err)
		return err
	}
	return nil
}

func (r *NamespaceOffloadingReconciler) getClusterIDMap(ctx context.Context) (map[string]*mapsv1alpha1.NamespaceMap, error) {
	// Build the selector to consider only local NamespaceMaps.
	metals := reflection.LocalResourcesLabelSelector()
	selector, err := metav1.LabelSelectorAsSelector(&metals)
	utilruntime.Must(err)

	nms := &mapsv1alpha1.NamespaceMapList{}
	if err := r.List(ctx, nms, client.MatchingLabelsSelector{Selector: selector}); err != nil {
		klog.Error(err, " --> Unable to List NamespaceMaps")
		return nil, err
	}

	clusterIDMap := make(map[string]*mapsv1alpha1.NamespaceMap)
	if len(nms.Items) == 0 {
		klog.Info("No NamespaceMaps at the moment in the cluster")
		return clusterIDMap, nil
	}

	for i := range nms.Items {
		clusterIDMap[nms.Items[i].Labels[liqoconst.RemoteClusterID]] = &nms.Items[i]
	}
	return clusterIDMap, nil
}
