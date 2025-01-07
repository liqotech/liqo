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

package postuninstall

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/getters"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

func Test_Unjoin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

var _ = Describe("Liqo E2E", func() {
	var (
		ctx                   = context.Background()
		testContext           = tester.GetTesterUninstall(ctx)
		interval              = 3 * time.Second
		timeout               = 5 * time.Minute
		managedByLiqoSelector = labels.SelectorFromSet(labels.Set{
			liqoconst.K8sAppManagedByKey: liqoconst.LiqoAppLabelValue,
		})
	)

	Describe("Assert that Liqo is correctly uninstalled", func() {
		Context("Test Liqo uninstall", func() {
			var uninstalledTableEntries []TableEntry
			for index := range testContext.Clusters {
				uninstalledTableEntries = append(uninstalledTableEntries,
					Entry(fmt.Sprintf("Check Liqo is correctly uninstalled on cluster %v", index+1),
						testContext.Clusters[index], testContext.Namespace, "liqo-storage"))
			}

			DescribeTable("Liqo Uninstall Check", util.DescribeTableArgs(
				func(homeCluster tester.ClusterContext, liqoNamespace, storageNamespace string) {
					// Check resources on liqo, liqo-storage and liqo-tenant namespaces.
					namespaces := []string{liqoNamespace, storageNamespace}
					tenantNsList, err := homeCluster.NativeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
						LabelSelector: labels.SelectorFromSet(labels.Set{
							liqoconst.TenantNamespaceLabel: "true",
						}).String(),
					})
					Expect(err).ToNot(HaveOccurred())
					for _, ns := range tenantNsList.Items {
						namespaces = append(namespaces, ns.Name)
					}

					// Check that there are no pods remaining in liqo related namespaces.
					Eventually(NoPods(ctx, homeCluster.NativeClient, namespaces),
						timeout, interval).Should(Succeed())

					// Check that there are no roles and roleBindings remaining in liqo related namespaces.
					Eventually(NoRoles(ctx, homeCluster.ControllerClient, namespaces, labels.Everything()),
						timeout, interval).Should(Succeed())
					Eventually(NoRoleBindings(ctx, homeCluster.ControllerClient, namespaces, labels.Everything()),
						timeout, interval).Should(Succeed())

					// Check that there are no roles, roleBindings, clusterRoles and clusterRoleBindings that are managed by Liqo
					Eventually(NoRoles(ctx, homeCluster.ControllerClient, []string{corev1.NamespaceAll}, managedByLiqoSelector),
						timeout, interval).Should(Succeed())
					Eventually(NoRoleBindings(ctx, homeCluster.ControllerClient, []string{corev1.NamespaceAll}, managedByLiqoSelector),
						timeout, interval).Should(Succeed())
					Eventually(NoClusterRoles(ctx, homeCluster.ControllerClient, managedByLiqoSelector),
						timeout, interval).Should(Succeed())
					Eventually(NoClusterRoleBindings(ctx, homeCluster.ControllerClient, managedByLiqoSelector),
						timeout, interval).Should(Succeed())

					// Check that there are no Liqo nodes remaining in liqo related namespaces.
					Eventually(NoLiqoNodes(ctx, homeCluster.NativeClient),
						timeout, interval).Should(Succeed())

				},
				uninstalledTableEntries...,
			)...)
		})
	})
})

func NoPods(ctx context.Context, clientset *kubernetes.Clientset, namespaces []string) error {
	for _, namespace := range namespaces {
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Error(err)
			return err
		}
		if len(pods.Items) > 0 {
			return fmt.Errorf("There are still running pods in namespace %s", namespace)
		}
	}
	return nil
}

func NoLiqoNodes(ctx context.Context, clientset *kubernetes.Clientset) error {
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%v=%v", liqoconst.TypeLabel, liqoconst.TypeNode),
	})
	if err != nil {
		klog.Error(err)
		return err
	}
	if len(nodes.Items) > 0 {
		return fmt.Errorf("There are still virtual nodes in the cluster")
	}
	return nil
}

func NoRoles(ctx context.Context, cl client.Client, namespaces []string, lSelector labels.Selector) error {
	for _, namespace := range namespaces {
		roles, err := getters.ListRolesByLabel(ctx, cl, namespace, lSelector)
		if err != nil {
			return err
		}
		if len(roles) > 0 {
			return fmt.Errorf("There are still roles in namespace %s matching the selector", namespace)
		}
	}
	return nil
}

func NoRoleBindings(ctx context.Context, cl client.Client, namespaces []string, lSelector labels.Selector) error {
	for _, namespace := range namespaces {
		roleBindings, err := getters.ListRoleBindingsByLabel(ctx, cl, namespace, lSelector)
		if err != nil {
			return err
		}
		if len(roleBindings) > 0 {
			return fmt.Errorf("There are still rolebindings in namespace %s matching the selector", namespace)
		}
	}
	return nil
}

func NoClusterRoles(ctx context.Context, cl client.Client, lSelector labels.Selector) error {
	clusterRoles, err := getters.ListClusterRolesByLabel(ctx, cl, lSelector)
	if err != nil {
		return err
	}
	if len(clusterRoles) > 0 {
		return fmt.Errorf("There are still clusterroles in the cluster matching the selector")
	}
	return nil
}

func NoClusterRoleBindings(ctx context.Context, cl client.Client, lSelector labels.Selector) error {
	clusterRoleBindings, err := getters.ListClusterRoleBindingsByLabel(ctx, cl, lSelector)
	if err != nil {
		return err
	}
	if len(clusterRoleBindings) > 0 {
		return fmt.Errorf("There are still clusterrolebindings in the cluster matching the selector")
	}
	return nil
}
