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

package util

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

const (
	// controlPlaneTaintKey is the key of the taint applied to control-plane nodes.
	controlPlaneTaintKey = "node-role.kubernetes.io/control-plane"
)

// IsNodeControlPlane checks if the node has the control-plane taint.
func IsNodeControlPlane(taints []corev1.Taint) bool {
	for _, taint := range taints {
		if taint.Key == controlPlaneTaintKey {
			return true
		}
	}
	return false
}

// GetNodes returns the list of nodes of the cluster matching the given labels.
func GetNodes(ctx context.Context, client kubernetes.Interface,
	clusterID liqov1beta1.ClusterID, labelSelector string) (*corev1.NodeList, error) {
	remoteNodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		klog.Errorf("%s -> an error occurred while listing nodes: %s", clusterID, err)
		return nil, err
	}
	return remoteNodes, nil
}

// GetWorkerNodes returns the list of worker nodes of the cluster.
func GetWorkerNodes(ctx context.Context, client kubernetes.Interface, clusterID, labelSelector string) (*corev1.NodeList, error) {
	remoteNodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		klog.Errorf("%s -> an error occurred while listing nodes: %s", clusterID, err)
		return nil, err
	}
	var remoteNodeWorkers corev1.NodeList
	for i := range remoteNodes.Items {
		if !IsNodeControlPlane(remoteNodes.Items[i].Spec.Taints) {
			remoteNodeWorkers.Items = append(remoteNodeWorkers.Items, remoteNodes.Items[i])
		}
	}
	return remoteNodes, nil
}

// CheckVirtualNodes checks if the Liqo virtual nodes of cluster C.
func CheckVirtualNodes(ctx context.Context, homeClusterClient kubernetes.Interface, clusterNumber int) (ready bool) {
	var nodeLabel = make(map[string]string)
	nodeLabel[consts.TypeLabel] = consts.TypeNode
	virtualNodes, err := homeClusterClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(nodeLabel).String(),
	})
	if err != nil {
		klog.Error(err)
		return false
	}

	if len(virtualNodes.Items) != clusterNumber-1 {
		klog.Infof("Virtual nodes aren't yet ready: %d nodes exist, %d expected", len(virtualNodes.Items), clusterNumber-1)
		return false
	}

	for index := range virtualNodes.Items {
		for _, condition := range virtualNodes.Items[index].Status.Conditions {
			if condition.Type == corev1.NodeReady {
				if condition.Status == corev1.ConditionFalse {
					klog.Infof("Virtual nodes aren't yet ready: node %d has %s=%s",
						index, condition.Type, condition.Status)
					return false
				}
			} else {
				if condition.Status == corev1.ConditionTrue {
					klog.Infof("Virtual nodes aren't yet ready: node %d has %s=%s",
						index, condition.Type, condition.Status)
					return false
				}
			}
		}
	}
	return true
}
