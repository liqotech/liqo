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

package net

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

// EnsureNetTesterPods creates the NetTest pods and waits for them to be ready.
func EnsureNetTesterPods(ctx context.Context, homeClient kubernetes.Interface, homeID, remoteID, localPodName, remotePodName string) error {
	ns, err := util.EnforceNamespace(ctx, homeClient, homeID, TestNamespaceName)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		klog.Error(err)
		return err
	}

	// TODO: remove it, check if the problem is related to namespace offloading initialization time.
	// This is a temporary patch for https://github.com/liqotech/liqo/issues/924
	time.Sleep(2 * time.Second)

	podRemote := forgeTesterPod(image, remotePodName, ns.Name, remoteID, true)
	_, err = homeClient.CoreV1().Pods(ns.Name).Create(ctx, podRemote, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		klog.Error(err)
		return err
	}
	podLocal := forgeTesterPod(image, localPodName, ns.Name, homeID, false)
	_, err = homeClient.CoreV1().Pods(ns.Name).Create(ctx, podLocal, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		klog.Error(err)
		return err
	}
	return nil
}

// CheckTesterPods retrieves the netTest pods and returns true if all the pods are up and ready.
func CheckTesterPods(ctx context.Context, homeClient, foreignClient kubernetes.Interface, homeClusterID, localPodName, remotePodName string) bool {
	reflectedNamespace := TestNamespaceName + "-" + homeClusterID
	return util.IsPodUp(ctx, homeClient, TestNamespaceName, localPodName, true) &&
		util.IsPodUp(ctx, homeClient, TestNamespaceName, remotePodName, true) &&
		util.IsPodUp(ctx, foreignClient, reflectedNamespace, remotePodName, false)
}

// GetTesterName returns the names for the local and the remote connectivity tester pods.
func GetTesterName(homeClusterID, remoteClusterID string) (remotePodName, localPodName string) {
	return fmt.Sprintf("%v-%v-%v", podTesterLocalCl, homeClusterID[:10], remoteClusterID[:10]),
		fmt.Sprintf("%v-%v-%v", podTesterRemoteCl, homeClusterID[:10], remoteClusterID[:10])
}

// forgeTesterPod deploys the Remote pod of the test.
func forgeTesterPod(image, podName, namespace, clusterID string, isOffloaded bool) *v1.Pod {
	var nodeSelector map[string]string
	NodeAffinityOperator := v1.NodeSelectorOpNotIn
	if isOffloaded {
		NodeAffinityOperator = v1.NodeSelectorOpIn
		nodeSelector = map[string]string{
			"kubernetes.io/hostname": strings.Join([]string{"liqo", clusterID}, "-"),
		}
	}

	pod1 := v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels:    map[string]string{"app": podName},
		},
		Spec: v1.PodSpec{
			NodeSelector: nodeSelector,
			Containers: []v1.Container{
				{
					Name:            "tester",
					Image:           image,
					Resources:       v1.ResourceRequirements{},
					ImagePullPolicy: "IfNotPresent",
					Ports: []v1.ContainerPort{{
						ContainerPort: 80,
					}},
				},
			},
			Affinity: &v1.Affinity{
				NodeAffinity: &v1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{NodeSelectorTerms: []v1.NodeSelectorTerm{{
						MatchExpressions: []v1.NodeSelectorRequirement{{
							Key:      liqoconst.TypeLabel,
							Operator: NodeAffinityOperator,
							Values:   []string{liqoconst.TypeNode},
						}},
						MatchFields: nil,
					}}},
				},
			},
		},
		Status: v1.PodStatus{},
	}
	return &pod1
}
