// Copyright 2019-2024 The Liqo Authors
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

package uninstaller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	discoveryV1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	"github.com/liqotech/liqo/pkg/utils/getters"
	peeringconditionsutils "github.com/liqotech/liqo/pkg/utils/peeringConditions"
)

// AnnotateControllerManagerDeployment annotates the controller-manager deployment with the uninstaller label.
func AnnotateControllerManagerDeployment(ctx context.Context, client dynamic.Interface, liqoNamespace string) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		deploy, err := getters.GetControllerManagerDeploymentWithDynamicClient(ctx, client, liqoNamespace)
		if err != nil {
			return err
		}

		// Add an annotation to the liqo-controller-manager to signal the uninstalling process.
		deploy.SetAnnotations(labels.Merge(deploy.GetAnnotations(),
			map[string]string{consts.UninstallingAnnotationKey: consts.UninstallingAnnotationValue}))

		unstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deploy)
		if err != nil {
			return err
		}

		_, err = client.Resource(appsv1.SchemeGroupVersion.WithResource("deployments")).Namespace(liqoNamespace).Update(
			ctx, &unstructured.Unstructured{Object: unstr}, metav1.UpdateOptions{},
		)
		return err
	})
}

// getForeignList retrieve the list of available ForeignCluster and return it as a ForeignClusterList object.
func getForeignList(client dynamic.Interface) (*discoveryV1alpha1.ForeignClusterList, error) {
	r1 := client.Resource(discoveryV1alpha1.ForeignClusterGroupVersionResource)
	t, err := r1.Namespace("").List(context.TODO(), metav1.ListOptions{TypeMeta: metav1.TypeMeta{}})
	if err != nil {
		return nil, err
	}
	klog.V(5).Info("Getting ForeignClusters list")
	var foreign *discoveryV1alpha1.ForeignClusterList
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(t.UnstructuredContent(), &foreign); err != nil {
		return nil, err
	}
	return foreign, nil
}

// checkPeeringsStatus verifies if the cluster has any active peerings with foreign clusters.
func checkPeeringsStatus(foreign *discoveryV1alpha1.ForeignClusterList) bool {
	var returnValue = true
	for i := range foreign.Items {
		item := &foreign.Items[i]
		if foreigncluster.IsIncomingJoined(item) || foreigncluster.IsOutgoingJoined(item) {
			incomingStatus := peeringconditionsutils.GetStatus(item, discoveryV1alpha1.IncomingPeeringCondition)
			outgoingStatus := peeringconditionsutils.GetStatus(item, discoveryV1alpha1.OutgoingPeeringCondition)
			klog.Infof("Cluster %s still has a valid peering: (Incoming: %s, Outgoing: %s",
				item.Name, incomingStatus, outgoingStatus)
			returnValue = false
		}
	}
	return returnValue
}

// generateLabelString converts labelSelector to string.
func generateLabelString(labelSelector metav1.LabelSelector) (string, error) {
	labelMap, err := metav1.LabelSelectorAsMap(&labelSelector)
	if err != nil {
		return "", err
	}
	return labels.SelectorFromSet(labelMap).String(), nil
}
