package tenantnamespace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/liqotech/liqo/pkg/discovery"
	testUtils2 "github.com/liqotech/liqo/pkg/utils/testUtils"
)

func TestTenantNamespace(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TenantNamespace Suite")
}

var _ = Describe("TenantNamespace", func() {

	var (
		cluster   testUtils2.Cluster
		clusterID string

		namespaceManager Manager
	)

	BeforeSuite(func() {
		clusterID = "test-creation"

		var err error
		cluster, _, err = testUtils2.NewTestCluster([]string{filepath.Join("..", "..", "deployments", "liqo", "crds")})
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}

		namespaceManager = NewTenantNamespaceManager(cluster.GetClient().Client())
	})

	AfterSuite(func() {
		err := cluster.GetEnv().Stop()
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}
	})

	It("Create Namespace", func() {
		ns, err := namespaceManager.CreateNamespace(clusterID)
		Expect(err).To(BeNil())
		Expect(ns).NotTo(BeNil())
		Expect(strings.HasPrefix(ns.Name, "liqo-tenant-")).To(BeTrue())
		Expect(ns.Labels).NotTo(BeNil())

		_, ok := ns.Labels[discovery.TenantNamespaceLabel]
		Expect(ok).To(BeTrue())
	})

	It("Create Namespace Twice", func() {
		ns, err := namespaceManager.CreateNamespace(clusterID)
		Expect(err).To(BeNil())
		Expect(ns).NotTo(BeNil())
		Expect(strings.HasPrefix(ns.Name, "liqo-tenant-")).To(BeTrue())
		Expect(ns.Labels).NotTo(BeNil())

		_, ok := ns.Labels[discovery.TenantNamespaceLabel]
		Expect(ok).To(BeTrue())
	})

	It("Get Namespace", func() {
		ns, err := namespaceManager.GetNamespace(clusterID)
		Expect(err).To(BeNil())
		Expect(ns).NotTo(BeNil())
		Expect(strings.HasPrefix(ns.Name, "liqo-tenant-")).To(BeTrue())
		Expect(ns.Labels).NotTo(BeNil())

		_, ok := ns.Labels[discovery.TenantNamespaceLabel]
		Expect(ok).To(BeTrue())

		ns, err = namespaceManager.GetNamespace("unknownId")
		Expect(err).NotTo(BeNil())
		Expect(ns).To(BeNil())
	})

	Context("Permission Management", func() {

		var client kubernetes.Interface
		var namespace *v1.Namespace
		var clusterRoles []*rbacv1.ClusterRole
		var cnt int = 0

		BeforeEach(func() {
			cnt += 1
			clusterID = fmt.Sprintf("test-permission-%v", cnt)
			client = cluster.GetClient().Client()

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

			namespace, err = namespaceManager.CreateNamespace(clusterID)
			Expect(err).To(BeNil())
			Expect(namespace).NotTo(BeNil())
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

			It("Bind ClusterRole", func() {
				var err error
				rb, err = namespaceManager.BindClusterRoles(clusterID, clusterRoles[0])
				Expect(err).To(BeNil())
				Expect(rb).NotTo(BeNil())
				Expect(len(rb)).To(Equal(1))
				checkRoleBinding(rb[0], namespace.Name, clusterID, clusterRoles[0].Name)

				err = namespaceManager.UnbindClusterRoles(clusterID, clusterRoles[0].Name)
				Expect(err).To(BeNil())

				_, err = client.RbacV1().RoleBindings(namespace.Name).Get(context.TODO(), rb[0].Name, metav1.GetOptions{})
				Expect(err).NotTo(BeNil())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())
			})

			It("Bind Multiple ClusterRoles", func() {
				var err error
				rb, err = namespaceManager.BindClusterRoles(clusterID, clusterRoles...)
				Expect(err).To(BeNil())
				Expect(rb).NotTo(BeNil())
				Expect(len(rb)).To(Equal(len(clusterRoles)))

				checkRoleBinding(rb[0], namespace.Name, clusterID, clusterRoles[0].Name)
				checkRoleBinding(rb[1], namespace.Name, clusterID, clusterRoles[1].Name)

				err = namespaceManager.UnbindClusterRoles(clusterID, clusterRoles[0].Name)
				Expect(err).To(BeNil())

				_, err = client.RbacV1().RoleBindings(namespace.Name).Get(context.TODO(), rb[0].Name, metav1.GetOptions{})
				Expect(err).NotTo(BeNil())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())

				_, err = client.RbacV1().RoleBindings(namespace.Name).Get(context.TODO(), rb[1].Name, metav1.GetOptions{})
				Expect(err).To(BeNil())
				Expect(kerrors.IsNotFound(err)).NotTo(BeTrue())

				err = namespaceManager.UnbindClusterRoles(clusterID, clusterRoles[1].Name)
				Expect(err).To(BeNil())
			})

			It("Bind Multiple Times", func() {
				var err error
				rb, err = namespaceManager.BindClusterRoles(clusterID, clusterRoles[0])
				Expect(err).To(BeNil())
				Expect(rb).NotTo(BeNil())
				Expect(len(rb)).To(BeNumerically("==", 1))

				// bind twice the same cluster role
				rb, err = namespaceManager.BindClusterRoles(clusterID, clusterRoles[0])
				Expect(err).To(BeNil())
				Expect(rb).NotTo(BeNil())
				Expect(len(rb)).To(BeNumerically("==", 1))

				checkRoleBinding(rb[0], namespace.Name, clusterID, clusterRoles[0].Name)

				rbs, err := client.RbacV1().RoleBindings(namespace.Name).List(context.TODO(), metav1.ListOptions{})
				Expect(err).To(BeNil())
				Expect(len(rbs.Items)).To(BeNumerically("==", 1))

				err = namespaceManager.UnbindClusterRoles(clusterID, clusterRoles[0].Name)
				Expect(err).To(BeNil())

				_, err = client.RbacV1().RoleBindings(namespace.Name).Get(context.TODO(), rb[0].Name, metav1.GetOptions{})
				Expect(err).NotTo(BeNil())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())
			})

		})

	})

})

func checkRoleBinding(rb *rbacv1.RoleBinding, namespace string, clusterID string, clusterRoleName string) {
	Expect(rb.Namespace).To(Equal(namespace))
	Expect(len(rb.Subjects)).To(Equal(1))
	Expect(rb.Subjects[0].Kind).To(Equal(rbacv1.UserKind))
	Expect(rb.Subjects[0].Name).To(Equal(clusterID))
	Expect(rb.RoleRef.Kind).To(Equal("ClusterRole"))
	Expect(rb.RoleRef.Name).To(Equal(clusterRoleName))
}
