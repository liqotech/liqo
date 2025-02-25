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

package tenantnamespace

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("TenantNamespace", func() {

	It("Should correctly create the namespace", func() {
		By("Creating the namespace once")
		ns, err := namespaceManager.CreateNamespace(ctx, homeCluster)
		Expect(err).To(BeNil())
		Expect(ns).NotTo(BeNil())
		Expect(strings.HasPrefix(ns.Name, "liqo-tenant-")).To(BeTrue())
		Expect(ns.Labels).NotTo(BeNil())

		_, ok := ns.Labels[consts.TenantNamespaceLabel]
		Expect(ok).To(BeTrue())

		By("Checking the namespace can be correctly retrieved")
		Eventually(func() (*v1.Namespace, error) { return namespaceManager.GetNamespace(ctx, homeCluster) }).Should(Equal(ns))

		By("Creating the namespace once more and checking it is still the original one")
		ns2, err := namespaceManager.CreateNamespace(ctx, homeCluster)
		Expect(err).To(BeNil())
		Expect(ns2).To(Equal(ns))
	})

	It("Should forge a namespace with a custom name", func() {
		By("Forging a new namespace providing the name")
		nsname := "custom-cluster-name"
		ns := namespaceManager.ForgeNamespace(homeCluster, &nsname)
		Expect(ns).NotTo(BeNil())
		Expect(ns.Name).To(Equal(nsname))
		Expect(ns.Labels).NotTo(BeNil())
	})

	It("Get Namespace", func() {
		ns, err := namespaceManager.GetNamespace(ctx, homeCluster)
		Expect(err).To(BeNil())
		Expect(ns).NotTo(BeNil())
		Expect(strings.HasPrefix(ns.Name, "liqo-tenant-")).To(BeTrue())
		Expect(ns.Labels).NotTo(BeNil())

		_, ok := ns.Labels[consts.TenantNamespaceLabel]
		Expect(ok).To(BeTrue())

		ns, err = namespaceManager.GetNamespace(ctx, "unknown-cluster-id")
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
			homeCluster = liqov1beta1.ClusterID(clusterPrefix + "-id")
			client = cluster.GetClient()

			cr := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "role1",
				},
			}
			cr, err := client.RbacV1().ClusterRoles().Create(ctx, cr, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			Expect(cr).NotTo(BeNil())
			clusterRoles = append(clusterRoles, cr)

			cr = &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "role2",
				},
			}
			cr, err = client.RbacV1().ClusterRoles().Create(ctx, cr, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			Expect(cr).NotTo(BeNil())
			clusterRoles = append(clusterRoles, cr)

			namespace, err = namespaceManager.CreateNamespace(ctx, homeCluster)
			Expect(err).To(BeNil())
			Expect(namespace).NotTo(BeNil())

			// Let wait for the namespace to be cached, to prevent race conditions.
			Eventually(func() (*v1.Namespace, error) { return namespaceManager.GetNamespace(ctx, homeCluster) }).Should(Equal(namespace))
		})

		AfterEach(func() {
			for _, cr := range clusterRoles {
				err := client.RbacV1().ClusterRoles().Delete(ctx, cr.Name, metav1.DeleteOptions{})
				Expect(err).To(BeNil())
			}
			clusterRoles = []*rbacv1.ClusterRole{}

			err := client.CoreV1().Namespaces().Delete(ctx, namespace.Name, metav1.DeleteOptions{})
			Expect(err).To(BeNil())
		})

		Context("Single ClusterRole", func() {

			var rb []*rbacv1.RoleBinding

			It("Bind ClusterRole", func() {
				var err error
				rb, err = namespaceManager.BindClusterRoles(ctx, homeCluster, nil, clusterRoles[0])
				Expect(err).To(BeNil())
				Expect(rb).NotTo(BeNil())
				Expect(len(rb)).To(Equal(1))
				checkRoleBinding(rb[0], namespace.Name, homeCluster, clusterRoles[0].Name)

				err = namespaceManager.UnbindClusterRoles(ctx, homeCluster, clusterRoles[0])
				Expect(err).To(BeNil())

				_, err = client.RbacV1().RoleBindings(namespace.Name).Get(ctx, rb[0].Name, metav1.GetOptions{})
				Expect(err).NotTo(BeNil())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())
			})

			It("Bind Multiple ClusterRoles", func() {
				var err error
				rb, err = namespaceManager.BindClusterRoles(ctx, homeCluster, nil, clusterRoles...)
				Expect(err).To(BeNil())
				Expect(rb).NotTo(BeNil())
				Expect(len(rb)).To(Equal(len(clusterRoles)))

				checkRoleBinding(rb[0], namespace.Name, homeCluster, clusterRoles[0].Name)
				checkRoleBinding(rb[1], namespace.Name, homeCluster, clusterRoles[1].Name)

				err = namespaceManager.UnbindClusterRoles(ctx, homeCluster, clusterRoles[0])
				Expect(err).To(BeNil())

				_, err = client.RbacV1().RoleBindings(namespace.Name).Get(ctx, rb[0].Name, metav1.GetOptions{})
				Expect(err).NotTo(BeNil())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())

				_, err = client.RbacV1().RoleBindings(namespace.Name).Get(ctx, rb[1].Name, metav1.GetOptions{})
				Expect(err).To(BeNil())
				Expect(kerrors.IsNotFound(err)).NotTo(BeTrue())

				err = namespaceManager.UnbindClusterRoles(ctx, homeCluster, clusterRoles[1])
				Expect(err).To(BeNil())
			})

			It("Bind Multiple Times", func() {
				var err error
				rb, err = namespaceManager.BindClusterRoles(ctx, homeCluster, nil, clusterRoles[0])
				Expect(err).To(BeNil())
				Expect(rb).NotTo(BeNil())
				Expect(len(rb)).To(BeNumerically("==", 1))

				// bind twice the same cluster role
				rb, err = namespaceManager.BindClusterRoles(ctx, homeCluster, nil, clusterRoles[0])
				Expect(err).To(BeNil())
				Expect(rb).NotTo(BeNil())
				Expect(len(rb)).To(BeNumerically("==", 1))

				checkRoleBinding(rb[0], namespace.Name, homeCluster, clusterRoles[0].Name)

				rbs, err := client.RbacV1().RoleBindings(namespace.Name).List(ctx, metav1.ListOptions{})
				Expect(err).To(BeNil())
				Expect(len(rbs.Items)).To(BeNumerically("==", 1))

				err = namespaceManager.UnbindClusterRoles(ctx, homeCluster, clusterRoles[0])
				Expect(err).To(BeNil())

				_, err = client.RbacV1().RoleBindings(namespace.Name).Get(ctx, rb[0].Name, metav1.GetOptions{})
				Expect(err).NotTo(BeNil())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())
			})

		})

	})

})

func checkRoleBinding(rb *rbacv1.RoleBinding, namespace string, homeCluster liqov1beta1.ClusterID,
	clusterRoleName string) {
	Expect(rb.Namespace).To(Equal(namespace))
	Expect(len(rb.Subjects)).To(Equal(1))
	Expect(rb.Subjects[0].Kind).To(Equal(rbacv1.UserKind))
	Expect(rb.Subjects[0].Name).To(Equal(string(homeCluster)))
	Expect(rb.RoleRef.Kind).To(Equal("ClusterRole"))
	Expect(rb.RoleRef.Name).To(Equal(clusterRoleName))
}
