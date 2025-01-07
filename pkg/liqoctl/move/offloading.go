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

package move

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
)

func offloadLiqoStorageNamespace(ctx context.Context, cl client.Client, originNode, targetNode *corev1.Node) error {
	namespaceOffloading := &offloadingv1beta1.NamespaceOffloading{
		ObjectMeta: metav1.ObjectMeta{
			Name:      liqoconst.DefaultNamespaceOffloadingName,
			Namespace: liqoStorageNamespace,
		},
		Spec: offloadingv1beta1.NamespaceOffloadingSpec{
			NamespaceMappingStrategy: offloadingv1beta1.DefaultNameMappingStrategyType,
			PodOffloadingStrategy:    offloadingv1beta1.LocalPodOffloadingStrategyType,
			ClusterSelector: corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/hostname",
								Operator: corev1.NodeSelectorOpIn,
								Values:   getRemoteNodeNames(originNode, targetNode),
							},
						},
					},
				},
			},
		},
	}

	if err := cl.Create(ctx, namespaceOffloading); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func repatriateLiqoStorageNamespace(ctx context.Context, cl client.Client) error {
	namespaceOffloading := &offloadingv1beta1.NamespaceOffloading{
		ObjectMeta: metav1.ObjectMeta{
			Name:      liqoconst.DefaultNamespaceOffloadingName,
			Namespace: liqoStorageNamespace,
		},
	}

	return client.IgnoreNotFound(cl.Delete(ctx, namespaceOffloading))
}

func getRemoteNodeNames(nodes ...*corev1.Node) []string {
	var remoteNodes []string
	for _, node := range nodes {
		if utils.IsVirtualNode(node) {
			remoteNodes = append(remoteNodes, node.Name)
		}
	}
	return remoteNodes
}

func getRemoteStorageNamespaceName(ctx context.Context, cl client.Client, backoff *wait.Backoff) (string, error) {
	var nsOffloading offloadingv1beta1.NamespaceOffloading

	if backoff == nil {
		backoff = &wait.Backoff{
			Steps:    20,
			Duration: 500 * time.Millisecond,
			Factor:   1.5,
		}
	}

	err := retry.OnError(*backoff, func(e error) bool {
		return true
	}, func() error {
		if err := cl.Get(ctx, client.ObjectKey{
			Name:      liqoconst.DefaultNamespaceOffloadingName,
			Namespace: liqoStorageNamespace}, &nsOffloading); err != nil {
			return err
		}

		if nsOffloading.Status.OffloadingPhase != offloadingv1beta1.ReadyOffloadingPhaseType {
			return fmt.Errorf("namespace offloading is not ready")
		}
		if nsOffloading.Status.RemoteNamespaceName == "" {
			return fmt.Errorf("remote namespace name is not set")
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	return nsOffloading.Status.RemoteNamespaceName, nil
}
