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

package tenantnamespace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	capsulev1beta1 "github.com/clastix/capsule/api/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

func TestTenantNamespace(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TenantNamespace Suite")
}

var _ = Describe("TenantNamespace", func() {

	var (
		cluster     testutil.Cluster
		homeCluster discoveryv1alpha1.ClusterIdentity

		namespaceManager Manager
	)

	BeforeSuite(func() {
		homeCluster = discoveryv1alpha1.ClusterIdentity{
			ClusterID:   "home-cluster-id",
			ClusterName: "home-cluster-name",
		}

		var err error
		cluster, _, err = testutil.NewTestCluster([]string{filepath.Join("..", "..", "deployments", "liqo", "crds")})
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}

		namespaceManager = NewTenantNamespaceManager(cluster.GetClient())
	})

	AfterSuite(func() {
		err := cluster.GetEnv().Stop()
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}
	})

	It("Should correctly create the namespace", func() {
		By("Creating the namespace once")
		ns, err := namespaceManager.CreateNamespace(homeCluster)
		Expect(err).To(BeNil())
		Expect(ns).NotTo(BeNil())
		Expect(strings.HasPrefix(ns.Name, "liqo-tenant-")).To(BeTrue())
		Expect(ns.Labels).NotTo(BeNil())

		_, ok := ns.Labels[discovery.TenantNamespaceLabel]
		Expect(ok).To(BeTrue())

		By("Checking the namespace can be correctly retrieved")
		Eventually(func() (*v1.Namespace, error) { return namespaceManager.GetNamespace(homeCluster) }).Should(Equal(ns))

		By("Creating the namespace once more and checking it is still the original one")
		ns2, err := namespaceManager.CreateNamespace(homeCluster)
		Expect(err).To(BeNil())
		Expect(ns2).To(Equal(ns))
	})

	It("Get Namespace", func() {
		ns, err := namespaceManager.GetNamespace(homeCluster)
		Expect(err).To(BeNil())
		Expect(ns).NotTo(BeNil())
		Expect(strings.HasPrefix(ns.Name, "liqo-tenant-")).To(BeTrue())
		Expect(ns.Labels).NotTo(BeNil())

		_, ok := ns.Labels[discovery.TenantNamespaceLabel]
		Expect(ok).To(BeTrue())

		ns, err = namespaceManager.GetNamespace(discoveryv1alpha1.ClusterIdentity{
			ClusterID:   "unknown-cluster-id",
			ClusterName: "unknown-cluster-name",
		})
		Expect(err).NotTo(BeNil())
		Expect(ns).To(BeNil())
	})

	Context("Permission Management", func() {

		var client kubernetes.Interface
		var namespace *v1.Namespace
		var clusterRoles []*rbacv1.ClusterRole
		var cnt = 0

		BeforeEach(func() {
			cnt++
			clusterPrefix := fmt.Sprintf("test-permission-%v", cnt)
			homeCluster = discoveryv1alpha1.ClusterIdentity{
				ClusterID:   clusterPrefix + "-id",
				ClusterName: clusterPrefix + "-name",
			}
			client = cluster.GetClient()

			cr := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "role1",
				},
			}
			cr, err := client.RbacV1().ClusterRoles().Create(context.TODO(), cr, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			Expect(cr).NotTo(BeNil())
			clusterRoles = append(clusterRoles, cr)

			cr = &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "role2",
				},
			}
			cr, err = client.RbacV1().ClusterRoles().Create(context.TODO(), cr, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			Expect(cr).NotTo(BeNil())
			clusterRoles = append(clusterRoles, cr)

			namespace, err = namespaceManager.CreateNamespace(homeCluster)
			Expect(err).To(BeNil())
			Expect(namespace).NotTo(BeNil())

			// Let wait for the namespace to be cached, to prevent race conditions.
			Eventually(func() (*v1.Namespace, error) { return namespaceManager.GetNamespace(homeCluster) }).Should(Equal(namespace))
		})

		AfterEach(func() {
			for _, cr := range clusterRoles {
				err := client.RbacV1().ClusterRoles().Delete(context.TODO(), cr.Name, metav1.DeleteOptions{})
				Expect(err).To(BeNil())
			}
			clusterRoles = []*rbacv1.ClusterRole{}

			err := client.CoreV1().Namespaces().Delete(context.TODO(), namespace.Name, metav1.DeleteOptions{})
			Expect(err).To(BeNil())
		})

		Context("Single ClusterRole", func() {

			var rb []*rbacv1.RoleBinding
			var crb *rbacv1.ClusterRoleBinding

			It("Bind ClusterRole", func() {
				var err error
				rb, err = namespaceManager.BindClusterRoles(homeCluster, clusterRoles[0])
				Expect(err).To(BeNil())
				Expect(rb).NotTo(BeNil())
				Expect(len(rb)).To(Equal(1))
				checkRoleBinding(rb[0], namespace.Name, homeCluster, clusterRoles[0].Name)

				err = namespaceManager.UnbindClusterRoles(homeCluster, clusterRoles[0].Name)
				Expect(err).To(BeNil())

				_, err = client.RbacV1().RoleBindings(namespace.Name).Get(context.TODO(), rb[0].Name, metav1.GetOptions{})
				Expect(err).NotTo(BeNil())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())
			})

			It("Bind Multiple ClusterRoles", func() {
				var err error
				rb, err = namespaceManager.BindClusterRoles(homeCluster, clusterRoles...)
				Expect(err).To(BeNil())
				Expect(rb).NotTo(BeNil())
				Expect(len(rb)).To(Equal(len(clusterRoles)))

				checkRoleBinding(rb[0], namespace.Name, homeCluster, clusterRoles[0].Name)
				checkRoleBinding(rb[1], namespace.Name, homeCluster, clusterRoles[1].Name)

				err = namespaceManager.UnbindClusterRoles(homeCluster, clusterRoles[0].Name)
				Expect(err).To(BeNil())

				_, err = client.RbacV1().RoleBindings(namespace.Name).Get(context.TODO(), rb[0].Name, metav1.GetOptions{})
				Expect(err).NotTo(BeNil())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())

				_, err = client.RbacV1().RoleBindings(namespace.Name).Get(context.TODO(), rb[1].Name, metav1.GetOptions{})
				Expect(err).To(BeNil())
				Expect(kerrors.IsNotFound(err)).NotTo(BeTrue())

				err = namespaceManager.UnbindClusterRoles(homeCluster, clusterRoles[1].Name)
				Expect(err).To(BeNil())
			})

			It("Bind Multiple Times", func() {
				var err error
				rb, err = namespaceManager.BindClusterRoles(homeCluster, clusterRoles[0])
				Expect(err).To(BeNil())
				Expect(rb).NotTo(BeNil())
				Expect(len(rb)).To(BeNumerically("==", 1))

				// bind twice the same cluster role
				rb, err = namespaceManager.BindClusterRoles(homeCluster, clusterRoles[0])
				Expect(err).To(BeNil())
				Expect(rb).NotTo(BeNil())
				Expect(len(rb)).To(BeNumerically("==", 1))

				checkRoleBinding(rb[0], namespace.Name, homeCluster, clusterRoles[0].Name)

				rbs, err := client.RbacV1().RoleBindings(namespace.Name).List(context.TODO(), metav1.ListOptions{})
				Expect(err).To(BeNil())
				Expect(len(rbs.Items)).To(BeNumerically("==", 1))

				err = namespaceManager.UnbindClusterRoles(homeCluster, clusterRoles[0].Name)
				Expect(err).To(BeNil())

				_, err = client.RbacV1().RoleBindings(namespace.Name).Get(context.TODO(), rb[0].Name, metav1.GetOptions{})
				Expect(err).NotTo(BeNil())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())
			})

			It("cluster wide outgoing permission management", func() {
				var err error
				ctx := context.Background()

				var checkLabels = func(labels map[string]string) {
					cID, ok := labels[discovery.ClusterIDLabel]
					Expect(ok).To(BeTrue())
					Expect(cID).To(Equal(homeCluster.ClusterID))
				}

				clusterRoleName := fmt.Sprintf("%v-%v", clusterRolePrefix, homeCluster.ClusterID)

				By("Creating the binding")

				crb, err = namespaceManager.BindIncomingClusterWideRole(ctx, homeCluster)
				Expect(err).NotTo(HaveOccurred())
				Expect(crb).NotTo(BeNil())

				checkLabels(crb.GetLabels())

				Expect(len(crb.Subjects)).To(Equal(1))
				Expect(crb.Subjects[0].Kind).To(Equal(rbacv1.UserKind))
				Expect(crb.Subjects[0].Name).To(Equal(homeCluster.ClusterID))
				Expect(crb.RoleRef.Kind).To(Equal("ClusterRole"))
				Expect(crb.RoleRef.Name).To(Equal(clusterRoleName))

				cr, err := client.RbacV1().ClusterRoles().Get(ctx, clusterRoleName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(cr).NotTo(BeNil())

				checkLabels(cr.GetLabels())

				Expect(len(cr.Rules)).To(BeNumerically("==", 1))
				Expect(cr.Rules[0].APIGroups).To(ContainElement(capsulev1beta1.GroupVersion.Group))
				Expect(cr.Rules[0].ResourceNames).To(ContainElement(homeCluster.ClusterName))
				Expect(cr.Rules[0].Resources).To(ContainElement("tenants/finalizers"))
				Expect(cr.Rules[0].Verbs).To(ContainElements("get", "patch", "update"))

				By("Creating the binding twice")

				crb, err = namespaceManager.BindIncomingClusterWideRole(ctx, homeCluster)
				Expect(err).NotTo(HaveOccurred())

				By("Deleting the binding")

				Expect(namespaceManager.UnbindIncomingClusterWideRole(ctx, homeCluster)).To(Succeed())

				_, err = client.RbacV1().ClusterRoles().Get(ctx, clusterRoleName, metav1.GetOptions{})
				Expect(err).To(HaveOccurred())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())

				_, err = client.RbacV1().ClusterRoleBindings().Get(ctx, clusterRoleName, metav1.GetOptions{})
				Expect(err).To(HaveOccurred())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())

				By("Deleting the binding twice")

				Expect(namespaceManager.UnbindIncomingClusterWideRole(ctx, homeCluster)).To(Succeed())
			})

		})

	})

})

func checkRoleBinding(rb *rbacv1.RoleBinding, namespace string, homeCluster discoveryv1alpha1.ClusterIdentity,
	clusterRoleName string) {
	Expect(rb.Namespace).To(Equal(namespace))
	Expect(len(rb.Subjects)).To(Equal(1))
	Expect(rb.Subjects[0].Kind).To(Equal(rbacv1.UserKind))
	Expect(rb.Subjects[0].Name).To(Equal(homeCluster.ClusterID))
	Expect(rb.RoleRef.Kind).To(Equal("ClusterRole"))
	Expect(rb.RoleRef.Name).To(Equal(clusterRoleName))
}
