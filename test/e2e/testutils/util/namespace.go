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
	"fmt"

	"github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/test/e2e/testconsts"
)

// EnforceNamespace creates and returns a namespace. If it already exists, it just returns the namespace.
func EnforceNamespace(ctx context.Context, cl kubernetes.Interface, cluster discoveryv1alpha1.ClusterIdentity,
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

// EnsureNamespaceDeletion wrap the deletion of a namespace.
func EnsureNamespaceDeletion(ctx context.Context, cl kubernetes.Interface, labelSelector map[string]string) error {
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

// OffloadNamespace offloads a namespace using liqoctl.
func OffloadNamespace(kubeconfig, namespace string, args ...string) error {
	return ExecLiqoctl(kubeconfig, append([]string{"offload", "namespace", namespace}, args...), ginkgo.GinkgoWriter)
}

// UnoffloadNamespace unoffloads a namespace using liqoctl.
func UnoffloadNamespace(kubeconfig, namespace string) error {
	return ExecLiqoctl(kubeconfig, []string{"unoffload", "namespace", namespace}, ginkgo.GinkgoWriter)
}
