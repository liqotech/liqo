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

package liqodeploymentctrl

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	k8shelper "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	nodeselectorutils "github.com/liqotech/liqo/pkg/utils/nodeSelector"
)

const (
	errorAnnotationKey   = "selector-error"
	errorAnnotationValue = "Invalid Selector in this resource or in the associated NamespaceOffloading"
	labelSeparator       = "&&"
	keyValueSeparator    = "="
)

//
func addErrorAnnotation(cl client.Client, ctx context.Context, ldp *offv1alpha1.LiqoDeployment, e error) {
	klog.Errorf("%s -> There is an error in the Selector specified or in the LiqoDeployment '%s' or "+
		"in the NamespaceOffloading resource inside the namespace '%s'.", e, ldp.Name, ldp.Namespace)
	patch := ldp.DeepCopy()
	if ldp.Annotations == nil {
		ldp.Annotations = map[string]string{}
	}
	ldp.Annotations[errorAnnotationKey] = fmt.Sprintf("%s: %s", errorAnnotationValue, e)
	if err := cl.Patch(ctx, ldp, client.MergeFrom(patch)); err != nil {
		klog.Errorf("%s -> unable to add the error annotation to the LiqoDeployment '%s'", err, ldp.Name)
		return
	}
	klog.Infof("The error annotation is correctly added to the LiqoDeployment '%s'", ldp.Name)
}

//
func removeErrorAnnotation(cl client.Client, ctx context.Context, ldp *offv1alpha1.LiqoDeployment) error {
	patch := ldp.DeepCopy()
	delete(ldp.Annotations, errorAnnotationKey)
	if err := cl.Patch(ctx, ldp, client.MergeFrom(patch)); err != nil {
		klog.Errorf("%s -> unable to remove the error annotation from the LiqoDeployment '%s'",
			err, ldp.Name)
		return err
	}
	klog.Infof("The error annotation is correctly removed from the LiqoDeployment '%s'", ldp.Name)
	return nil
}

//
func (r *LiqoDeploymentReconciler) checkCompatibleVirtualNodes(ctx context.Context, ns *corev1.NodeSelector,
	ldp *offv1alpha1.LiqoDeployment) error {
	// Clean the SelectedCluster map
	r.SelectedClusters = map[string]struct{}{}
	orderedGenerationLabels := ldp.Spec.GroupByLabels
	sort.Strings(orderedGenerationLabels)

	virtualNodes := &corev1.NodeList{}
	if err := r.List(ctx, virtualNodes, client.MatchingLabels{liqoconst.TypeLabel: liqoconst.TypeNode}); err != nil {
		klog.Errorf(" %s --> Unable to List all virtual nodes", err)
		return err
	}

	// If there are no virtual nodes in the cluster, the SelectedCluster map will be empty.
	// When the map is empty there should not be any replicated deployment.
	if len(virtualNodes.Items) == 0 {
		klog.Info("No virtual nodes in the cluster.")
		return nil
	}

	for i := range virtualNodes.Items {
		match, err := k8shelper.MatchNodeSelectorTerms(&virtualNodes.Items[i], ns)
		if err != nil {
			addErrorAnnotation(r.Client, ctx, ldp, err)
			return err
		}
		if match {
			labelsNumber := 0
			mapKey := ""

			for _, labelKey := range orderedGenerationLabels {
				if value, ok := virtualNodes.Items[i].Labels[labelKey]; ok {
					labelsNumber++
					mapKey = fmt.Sprintf("%s%s%s%s%s", mapKey, labelSeparator, labelKey, keyValueSeparator, value)
				}
			}

			if labelsNumber == len(orderedGenerationLabels) {
				r.SelectedClusters[mapKey] = struct{}{}
			}
		}
	}
	if _, ok := (ldp.Annotations)[errorAnnotationKey]; ok {
		return removeErrorAnnotation(r.Client, ctx, ldp)
	}
	return nil
}

// getClusterFilter merges the NodeSelector specified in the LiqoDeployment with the one specified in the NamespaceOffloading.
// If the user does not specify a NodeSelector in the LiqoDelpoyment resource, the resulting Selector will be equal to
// the NamespaceOffloading one.
func getClusterFilter(noff *offv1alpha1.NamespaceOffloading, ldp *offv1alpha1.LiqoDeployment) corev1.NodeSelector {
	mergedNodeSelector := corev1.NodeSelector{}
	if ldp.Spec.ClusterFilter.Size() == 0 {
		mergedNodeSelector = noff.Spec.ClusterSelector
	} else {
		mergedNodeSelector = nodeselectorutils.GetMergedNodeSelector(&ldp.Spec.ClusterFilter, &noff.Spec.ClusterSelector)
	}
	return mergedNodeSelector
}
