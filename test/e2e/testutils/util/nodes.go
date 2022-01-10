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

package util

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/consts"
)

// GetNodes returns the list of nodes of the cluster matching the given labels.
func GetNodes(ctx context.Context, client kubernetes.Interface, clusterID, labelSelector string) (*v1.NodeList, error) {
	remoteNodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		klog.Errorf("%s -> an error occurred while listing nodes: %s", clusterID, err)
		return nil, err
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
			if condition.Type == v1.NodeReady {
				if condition.Status == v1.ConditionFalse {
					klog.Infof("Virtual nodes aren't yet ready: node %d has %s=%s",
						index, condition.Type, condition.Status)
					return false
				}
			} else {
				if condition.Status == v1.ConditionTrue {
					klog.Infof("Virtual nodes aren't yet ready: node %d has %s=%s",
						index, condition.Type, condition.Status)
					return false
				}
			}
		}
	}
	return true
}
