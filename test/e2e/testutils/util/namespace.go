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
	"fmt"

	"github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/test/e2e/testconsts"
)

// EnforceNamespace creates and returns a namespace. If it already exists, it just returns the namespace.
func EnforceNamespace(ctx context.Context, cl kubernetes.Interface, cluster liqov1beta1.ClusterID,
	name string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{testconsts.LiqoTestingLabelKey: testconsts.LiqoTestingLabelValue},
		},
	}
	ns, err := cl.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if kerrors.IsAlreadyExists(err) {
		ns, err = cl.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("%s -> an error occurred while creating namespace %s : %s", cluster, name, err)
			return nil, err
		}
	} else if err != nil {
		klog.Errorf("%s -> an error occurred while creating namespace %s : %s", cluster, name, err)
		return nil, err
	}
	return ns, nil
}

// EnsureNamespaceDeletion deletes the namespace with the given name.
func EnsureNamespaceDeletion(ctx context.Context, cl kubernetes.Interface, name string) error {
	err := cl.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if kerrors.IsNotFound(err) {
		return nil
	}
	return fmt.Errorf("still deleting namespace %s", name)
}

// EnsureNamespacesDeletionWithSelector wrap the deletion of namespaces matching the given labelSelector.
func EnsureNamespacesDeletionWithSelector(ctx context.Context, cl kubernetes.Interface, labelSelector map[string]string) error {
	namespaceList, err := cl.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labelSelector).String(),
	})
	if err != nil {
		return err
	}
	if len(namespaceList.Items) == 0 {
		return nil
	}
	for i := range namespaceList.Items {
		_ = cl.CoreV1().Namespaces().Delete(ctx, namespaceList.Items[i].Name, metav1.DeleteOptions{})
	}
	return fmt.Errorf("still deleting namespaces")
}

// CreateNamespaceOffloading creates a new NamespaceOffloading resource, with the given parameters.
func CreateNamespaceOffloading(ctx context.Context, cl client.Client, namespace string,
	nms offloadingv1beta1.NamespaceMappingStrategyType, pof offloadingv1beta1.PodOffloadingStrategyType) error {
	nsoff := &offloadingv1beta1.NamespaceOffloading{
		ObjectMeta: metav1.ObjectMeta{Name: consts.DefaultNamespaceOffloadingName, Namespace: namespace},
		Spec: offloadingv1beta1.NamespaceOffloadingSpec{
			NamespaceMappingStrategy: nms, PodOffloadingStrategy: pof,
			ClusterSelector: corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{}}},
	}

	return cl.Create(ctx, nsoff)
}

// GetNamespaceOffloading returns the NamespaceOffloading resource for the given namespace.
func GetNamespaceOffloading(ctx context.Context, cl client.Client, namespace string) (*offloadingv1beta1.NamespaceOffloading, error) {
	nsoff := &offloadingv1beta1.NamespaceOffloading{}
	err := cl.Get(ctx, client.ObjectKey{Name: consts.DefaultNamespaceOffloadingName, Namespace: namespace}, nsoff)
	return nsoff, err
}

// OffloadNamespace offloads a namespace using liqoctl.
func OffloadNamespace(kubeconfig, namespace string, args ...string) error {
	return ExecLiqoctl(kubeconfig, append([]string{"offload", "namespace", namespace}, args...), ginkgo.GinkgoWriter)
}

// UnoffloadNamespace unoffloads a namespace using liqoctl.
func UnoffloadNamespace(kubeconfig, namespace string) error {
	return ExecLiqoctl(kubeconfig, []string{"unoffload", "namespace", "--skip-confirm", namespace}, ginkgo.GinkgoWriter)
}
