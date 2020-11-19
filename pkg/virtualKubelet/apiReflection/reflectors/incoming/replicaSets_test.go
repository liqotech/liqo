package incoming_test

import (
	"bytes"
	"context"
	"flag"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/incoming"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping/test"
	storageTest "github.com/liqotech/liqo/pkg/virtualKubelet/storage/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog"
	"strings"
)

var _ = Describe("ReplicationControllers", func() {
	var (
		cacheManager          *storageTest.MockManager
		namespaceNattingTable *test.MockNamespaceMapper
		genericReflector      *reflectors.GenericAPIReflector
		reflector             *incoming.ReplicaSetsIncomingReflector
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
		reflector = &incoming.ReplicaSetsIncomingReflector{APIReflector: genericReflector}
		reflector.APIReflector = genericReflector

		reflector.SetSpecializedPreProcessingHandlers()
	})

	Describe("pre routines", func() {
		Context("pre add", func() {
			type addTestcase struct {
				input *appsv1.ReplicaSet
			}

			DescribeTable("pre add test cases",
				func(c addTestcase) {
					ret := reflector.PreProcessAdd(c.input)
					Expect(ret).To(BeNil())
				},
				Entry("with empty replicaset", addTestcase{
					input: &appsv1.ReplicaSet{},
				}),
			)
		})

		Context("pre update", func() {
			type updateTestcase struct {
				newInput, oldInput *appsv1.ReplicaSet
			}

			DescribeTable("pre update test cases",
				func(c updateTestcase) {
					ret := reflector.PreProcessUpdate(c.newInput, c.oldInput)
					Expect(ret).To(BeNil())
				},
				Entry("empty replicasets", updateTestcase{
					newInput: &appsv1.ReplicaSet{},
					oldInput: &appsv1.ReplicaSet{},
				}),
			)
		})

		Context("pre delete", func() {
			type deleteTestcase struct {
				input    *appsv1.ReplicaSet
				expected *corev1.Pod
			}

			BeforeEach(func() {
				_, err := namespaceNattingTable.NatNamespace("homeNamespace", true)
				Expect(err).NotTo(HaveOccurred())
				_ = cacheManager.AddHomeNamespace("homeNamespace")
				_ = cacheManager.AddForeignNamespace("homeNamespace-natted")
			})

			DescribeTable("pre delete test cases",
				func(c deleteTestcase) {
					cacheManager.AddHomeEntry("homeNamespace", apimgmt.Pods, c.expected)
					ret := reflector.PreProcessDelete(c.input)
					Expect(ret).To(Equal(c.expected))
				},

				Entry("", deleteTestcase{
					input: &appsv1.ReplicaSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "replicaset1",
							Namespace: "homeNamespace-natted",
							Labels: map[string]string{
								virtualKubelet.ReflectedpodKey: "homePodName",
							},
						},
					},
					expected: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "homePodName",
							Namespace: "homeNamespace",
						},
					},
				}),
			)
		})
	})

	Describe("handle event", func() {
		type handleEventTestCase struct {
			input          interface{}
			expectedOutput string
		}

		var (
			homeClient kubernetes.Interface
			buffer     *bytes.Buffer
			pod        *corev1.Pod
			flags      *flag.FlagSet
		)

		BeforeEach(func() {
			homeClient = fake.NewSimpleClientset()
			genericReflector.HomeClient = homeClient

			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: "homeNamespace",
				}}

			buffer = &bytes.Buffer{}
			flags = &flag.FlagSet{}
			klog.InitFlags(flags)
			_ = flags.Set("logtostderr", "false")
			_ = flags.Set("v", "4")
			klog.SetOutput(buffer)
		})

		Context("with correct object creation", func() {
			DescribeTable("handle event test cases",
				func(c handleEventTestCase) {
					_, err := homeClient.CoreV1().Pods("homeNamespace").Create(context.TODO(), pod, metav1.CreateOptions{})
					Expect(err).NotTo(HaveOccurred())
					reflector.HandleEvent(c.input)
					klog.Flush()
					Expect(strings.Contains(buffer.String(), c.expectedOutput)).To(BeTrue())
				},
				Entry("correct delete event", handleEventTestCase{
					input: watch.Event{
						Type: watch.Deleted,
						Object: &corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pod1",
								Namespace: "homeNamespace",
							}},
					},
					expectedOutput: "INCOMING REFLECTION: home pod homeNamespace/pod1 correctly deleted",
				}),
				Entry("correct add event", handleEventTestCase{
					input: watch.Event{
						Type: watch.Added,
						Object: &corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pod1",
								Namespace: "homeNamespace",
							}},
					},
					expectedOutput: "INCOMING REFLECTION: event ADDED for object homeNamespace/pod1 ignored",
				}),
				Entry("correct update event", handleEventTestCase{
					input: watch.Event{
						Type: watch.Modified,
						Object: &corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pod1",
								Namespace: "homeNamespace",
							}},
					},
					expectedOutput: "INCOMING REFLECTION: event MODIFIED for object homeNamespace/pod1 ignored",
				}),
				Entry("wrong object", handleEventTestCase{
					expectedOutput: "cannot cast object to event",
				}),
			)
		})

		Context("without object creation", func() {
			var (
				event watch.Event
			)

			BeforeEach(func() {
				event = watch.Event{
					Type:   watch.Deleted,
					Object: pod,
				}
			})

			It("failing delete", func() {
				reflector.HandleEvent(event)
				klog.Flush()
				Expect(strings.Contains(buffer.String(), "INCOMING REFLECTION: error while deleting home pod homeNamespace/pod1")).To(BeTrue())
			})
		})
	})
})
