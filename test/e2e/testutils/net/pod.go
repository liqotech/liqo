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

package net

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

// TesterOpts contains to handle a connectivity tester pod.
type TesterOpts struct {
	Cluster   liqov1beta1.ClusterID
	PodName   string
	Offloaded bool
}

// EnsureNetTesterPods creates the NetTest pods and waits for them to be ready.
func EnsureNetTesterPods(ctx context.Context, config *tester.ClusterContext, cluster1, cluster2 *TesterOpts) error {
	ns, err := util.EnforceNamespace(ctx, config.NativeClient, cluster1.Cluster, TestNamespaceName)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		klog.Error(err)
		return err
	}

	if err := util.OffloadNamespace(config.KubeconfigPath, TestNamespaceName); err != nil {
		return err
	}

	// TODO: remove it, check if the problem is related to namespace offloading initialization time.
	// This is a temporary patch for https://github.com/liqotech/liqo/issues/924
	time.Sleep(2 * time.Second)

	cluster2Pod := forgeTesterPod(image, ns.Name, cluster2)
	_, err = config.NativeClient.CoreV1().Pods(ns.Name).Create(ctx, cluster2Pod, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		klog.Error(err)
		return err
	}
	cluster1Pod := forgeTesterPod(image, ns.Name, cluster1)
	_, err = config.NativeClient.CoreV1().Pods(ns.Name).Create(ctx, cluster1Pod, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		klog.Error(err)
		return err
	}
	return nil
}

// CheckTesterPods retrieves the netTest pods and returns true if all the pods are up and ready.
func CheckTesterPods(ctx context.Context,
	homeClient, cluster1Client, cluster2Client kubernetes.Interface,
	homeCluster liqov1beta1.ClusterID, cluster1, cluster2 *TesterOpts) bool {
	// Note that UniqueName depends on the cluster name, so this may break if the remote cluster uses a different name
	// than the one we pass as homeCluster
	reflectedNamespace := TestNamespaceName + "-" + foreignclusterutils.UniqueName(homeCluster)
	if !util.IsPodUp(ctx, homeClient, TestNamespaceName, cluster1.PodName, util.PodLocal) ||
		!util.IsPodUp(ctx, homeClient, TestNamespaceName, cluster2.PodName, util.PodLocal) {
		return false
	}
	if cluster1.Offloaded {
		if !util.IsPodUp(ctx, cluster1Client, reflectedNamespace, cluster1.PodName, util.PodRemote) {
			return false
		}
	}
	if cluster2.Offloaded {
		if !util.IsPodUp(ctx, cluster2Client, reflectedNamespace, cluster2.PodName, util.PodRemote) {
			return false
		}
	}
	return true
}

// GetTesterName returns the names for the connectivity tester pods.
func GetTesterName(clusterID1, clusterID2 liqov1beta1.ClusterID) (cluster1PodName, cluster2PodName string) {
	return fmt.Sprintf("%v-%v-%v", podTesterLocalCl, clusterID1, clusterID2),
		fmt.Sprintf("%v-%v-%v", podTesterRemoteCl, clusterID1, clusterID2)
}

// forgeTesterPod deploys the Remote pod of the test.
func forgeTesterPod(image, namespace string, opts *TesterOpts) *v1.Pod {
	var nodeSelector map[string]string
	NodeAffinityOperator := v1.NodeSelectorOpNotIn
	if opts.Offloaded {
		NodeAffinityOperator = v1.NodeSelectorOpIn
		nodeSelector = map[string]string{
			liqoconsts.RemoteClusterID: string(opts.Cluster),
		}
	}

	pod1 := v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.PodName,
			Namespace: namespace,
			Labels:    map[string]string{"app": opts.PodName},
		},
		Spec: v1.PodSpec{
			NodeSelector: nodeSelector,
			Containers: []v1.Container{
				{
					Name:            "tester",
					Image:           image,
					Resources:       util.ResourceRequirements(),
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
							Key:      liqoconsts.TypeLabel,
							Operator: NodeAffinityOperator,
							Values:   []string{liqoconsts.TypeNode},
						}},
						MatchFields: nil,
					}}},
				},
			},
		},
	}
	return &pod1
}
