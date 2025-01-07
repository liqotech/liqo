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

package concurrent

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/consts"
)

// AddActiveGatewayLabel adds the active gateway label to the pod.
func AddActiveGatewayLabel(ctx context.Context, cl client.Client, key client.ObjectKey) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
	}
	if err := cl.Get(ctx, key, pod); err != nil {
		return err
	}

	labels := pod.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[ActiveGatewayKey] = ActiveGatewayValue
	pod.SetLabels(labels)

	if err := cl.Update(ctx, pod); err != nil {
		return err
	}
	klog.Infof("Pod %s/%s is now the active gateway", pod.Namespace, pod.Name)
	return nil
}

// RemoveActiveGatewayLabel removes the active gateway label from the pod.
func RemoveActiveGatewayLabel(ctx context.Context, cl client.Client, key client.ObjectKey) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
	}
	if err := cl.Get(ctx, key, pod); err != nil {
		return err
	}

	labels := pod.GetLabels()
	if labels == nil {
		return nil
	}
	delete(labels, ActiveGatewayKey)
	pod.SetLabels(labels)

	if err := cl.Update(ctx, pod); err != nil {
		return err
	}
	klog.Infof("Pod %s/%s is no longer the active gateway", pod.Namespace, pod.Name)
	return nil
}

// ListAllGatewaysReplicas returns the list of all the gateways replicas of the same gateway.
func ListAllGatewaysReplicas(ctx context.Context, cl client.Client, namespace, gatewayName string) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := cl.List(ctx, podList, client.InNamespace(namespace), client.MatchingLabels{
		consts.K8sAppNameKey: gatewayName,
	}); err != nil {
		return nil, err
	}
	return podList.Items, nil
}
