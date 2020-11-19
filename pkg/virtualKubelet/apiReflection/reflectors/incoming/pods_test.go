package incoming_test

import (
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/incoming"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping/test"
	storageTest "github.com/liqotech/liqo/pkg/virtualKubelet/storage/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Pods", func() {

	var (
		cacheManager          *storageTest.MockManager
		namespaceNattingTable *test.MockNamespaceMapper
		genericReflector      *reflectors.GenericAPIReflector
		reflector             *incoming.PodsIncomingReflector
	)

	BeforeEach(func() {
		cacheManager = &storageTest.MockManager{
			HomeCache:    map[string]map[apimgmt.ApiType]map[string]metav1.Object{},
			ForeignCache: map[string]map[apimgmt.ApiType]map[string]metav1.Object{},
		}
		namespaceNattingTable = &test.MockNamespaceMapper{Cache: map[string]string{}}
		genericReflector = &reflectors.GenericAPIReflector{
			NamespaceNatting: namespaceNattingTable,
			CacheManager:     cacheManager,
		}
		reflector = &incoming.PodsIncomingReflector{APIReflector: genericReflector}
		reflector.APIReflector = genericReflector

		reflector.SetSpecializedPreProcessingHandlers()
		forge.InitForger(namespaceNattingTable)
	})

	Describe("pre routines", func() {
		When("empty caches", func() {
			Context("pre add", func() {
				type addTestcase struct {
					input          *corev1.Pod
					expectedOutput types.GomegaMatcher
				}

				DescribeTable("pre add test cases",
					func(c addTestcase) {
						ret := reflector.PreProcessAdd(c.input)
						Expect(ret).To(c.expectedOutput)
					},

					Entry("pod without labels", addTestcase{
						input:          &corev1.Pod{},
						expectedOutput: BeNil(),
					}),

					Entry("pod without the incoming label", addTestcase{
						input: &corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{}},
						},
						expectedOutput: BeNil(),
					}),

					Entry("pod with the incoming label", addTestcase{
						input: &corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "testPod",
								Namespace: "foreignNamespace",
								Labels: map[string]string{
									virtualKubelet.ReflectedpodKey: "wrongPod",
								},
							},
						},
						expectedOutput: BeNil(),
					}),
				)
			})
		})

		When("not empty caches", func() {
			Context("pre add", func() {
				var (
					homePod, foreignPod *corev1.Pod
				)

				BeforeEach(func() {
					foreignPod = &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "foreignPod",
							Namespace: "homeNamespace-natted",
							Labels: map[string]string{
								virtualKubelet.ReflectedpodKey: "homePod",
							},
						},
						Status: corev1.PodStatus{
							Phase:   corev1.PodRunning,
							Message: "testing",
						},
					}

					homePod = &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "homePod",
							Namespace: "homeNamespace",
						},
					}
					_, _ = namespaceNattingTable.NatNamespace("homeNamespace", true)
					_ = cacheManager.AddHomeNamespace("homeNamespace")
					_ = cacheManager.AddForeignNamespace("homeNamespace-natted")

					cacheManager.AddHomeEntry("homeNamespace", apimgmt.Pods, homePod)
				})

				It("correct foreign pod added", func() {
					ret := reflector.PreProcessAdd(foreignPod).(*corev1.Pod)
					Expect(ret.Name).To(Equal(homePod.Name))
					Expect(ret.Namespace).To(Equal(homePod.Namespace))
					Expect(ret.Status).To(Equal(foreignPod.Status))
				})

				It("correct foreign pod updated", func() {
					ret := reflector.PreProcessUpdate(foreignPod, nil).(*corev1.Pod)
					Expect(ret.Name).To(Equal(homePod.Name))
					Expect(ret.Namespace).To(Equal(homePod.Namespace))
					Expect(ret.Status).To(Equal(foreignPod.Status))
				})
			})
		})
	})
})
