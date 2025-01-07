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

package setup

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
)

// CreateNamespace creates a namespace.
func CreateNamespace(ctx context.Context, cl *client.Client) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: NamespaceName,
		},
	}
	if err := cl.Consumer.Create(ctx, ns); err != nil && ctrlclient.IgnoreAlreadyExists(err) != nil {
		return err
	}
	return nil
}

// OffloadNamespace offloads the namespace.
func OffloadNamespace(ctx context.Context, cl *client.Client) error {
	nsoff := offloadingv1beta1.NamespaceOffloading{
		ObjectMeta: metav1.ObjectMeta{
			Name:      consts.DefaultNamespaceOffloadingName,
			Namespace: NamespaceName,
		},
		Spec: offloadingv1beta1.NamespaceOffloadingSpec{
			NamespaceMappingStrategy: offloadingv1beta1.EnforceSameNameMappingStrategyType,
			PodOffloadingStrategy:    offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType,
			ClusterSelector: corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{},
			},
		},
	}
	if err := cl.Consumer.Create(ctx, &nsoff); err != nil && ctrlclient.IgnoreAlreadyExists(err) != nil {
		return err
	}
	// TODO: remove this sleep when the offloading race condition is fixed
	time.Sleep(10 * time.Second)
	return nil
}

// RemoveNamespace removes the namespace.
func RemoveNamespace(ctx context.Context, cl *client.Client) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: NamespaceName,
		},
	}
	if err := cl.Consumer.Delete(ctx, ns); err != nil {
		return err
	}
	timeout, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	if err := wait.PollUntilContextCancel(timeout, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		if err := cl.Consumer.Get(ctx, ctrlclient.ObjectKeyFromObject(ns), ns); err != nil {
			return ctrlclient.IgnoreNotFound(err) == nil, nil
		}
		return false, nil
	}); err != nil {
		return err
	}
	return nil
}
