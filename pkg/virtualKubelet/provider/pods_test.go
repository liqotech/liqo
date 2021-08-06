package provider

import (
	"bytes"
	"context"
	"flag"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog/v2"

	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	test2 "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/controller/test"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesmapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesmapping/test"
	test3 "github.com/liqotech/liqo/pkg/virtualKubelet/storage/test"
)

var _ = Describe("Pods", func() {
	var (
		provider              *LiqoProvider
		namespaceMapper       namespacesmapping.MapperController
		namespaceNattingTable *test.MockNamespaceMapper
		foreignClient         kubernetes.Interface
	)

	BeforeEach(func() {
		foreignClient = fake.NewSimpleClientset()
		namespaceNattingTable = &test.MockNamespaceMapper{Cache: map[string]string{}}
		namespaceNattingTable.Cache["homeNamespace"] = "homeNamespace-natted"
		namespaceMapper = test.NewMockNamespaceMapperController(namespaceNattingTable)
		mockManager := &test3.MockManager{
			HomeCache:    map[string]map[apimgmt.ApiType]map[string]metav1.Object{},
			ForeignCache: map[string]map[apimgmt.ApiType]map[string]metav1.Object{},
		}
		provider = &LiqoProvider{
			namespaceMapper: namespaceMapper,
			foreignClient:   foreignClient,
			apiController:   &test2.MockController{Manager: mockManager},
		}
	})

	Context("with legit input pod", func() {
		var (
			pod *corev1.Pod
		)

		When("writing functions", func() {

			BeforeEach(func() {
				pod = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testObject",
						Namespace: "homeNamespace",
					},
				}
				forge.InitForger(namespaceMapper)
			})

			/*
				TODO: We need to change the clients for allowing this test to pass

				It("create pod", func() {
					err := provider.CreatePod(context.TODO(), pod)
					Expect(err).NotTo(HaveOccurred())
					rs, err := foreignClient.AppsV1().ReplicaSets("homeNamespace-natted").Get(context.TODO(), "testObject", metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					Expect(rs.Name).To(Equal(pod.Name))
					Expect(rs.Namespace).To(Equal("homeNamespace-natted"))
				})
			*/

			It("update pod", func() {
				err := provider.UpdatePod(context.TODO(), pod)
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("delete pod", func() {
				var (
					replicaset *appsv1.ReplicaSet
					buffer     *bytes.Buffer
					flags      *flag.FlagSet
				)

				BeforeEach(func() {
					replicaset = &appsv1.ReplicaSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "testObject",
							Namespace: "homeNamespace-natted",
						},
					}
					buffer = &bytes.Buffer{}
					flags = &flag.FlagSet{}
					klog.InitFlags(flags)
					_ = flags.Set("logtostderr", "false")
					_ = flags.Set("v", "5")
					klog.SetOutput(buffer)
				})

				It("with corresponding replicaset existing", func() {
					_, _ = foreignClient.AppsV1().ReplicaSets("homeNamespace-natted").Create(context.TODO(), replicaset, metav1.CreateOptions{})
					err := provider.DeletePod(context.TODO(), pod)
					Expect(err).NotTo(HaveOccurred())
				})

				It("without corresponding replicaset existing", func() {
					err := provider.DeletePod(context.TODO(), pod)
					Expect(err).NotTo(HaveOccurred())
					klog.Flush()
					Expect(strings.Contains(buffer.String(), "replicaset homeNamespace-natted/testObject not deleted because not existing")).To(BeTrue())
				})
			})
		})
	})

	Context("with nil input pod", func() {

		It("create pod", func() {
			err := provider.CreatePod(context.TODO(), nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("update pod", func() {
			err := provider.UpdatePod(context.TODO(), nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("delete pod", func() {
			err := provider.DeletePod(context.TODO(), nil)
			Expect(err).To(HaveOccurred())
		})
	})
})
