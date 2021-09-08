// Copyright 2019-2021 The Liqo Authors
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

package incoming_test

import (
	"bytes"
	"errors"
	"flag"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/virtualKubelet"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/incoming"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesmapping/test"
	storageTest "github.com/liqotech/liqo/pkg/virtualKubelet/storage/test"
)

var _ = Describe("Pod incoming reflector", func() {

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
		reflector = &incoming.PodsIncomingReflector{
			APIReflector:  genericReflector,
			HomePodGetter: incoming.GetHomePodFunc,
		}

		reflector.SetSpecializedPreProcessingHandlers()
		forge.InitForger(namespaceNattingTable)
	})

	AfterEach(func() {
		namespaceNattingTable.Clear()
		cacheManager.Clear()
	})

	Describe("PreProcessingHandlers", func() {
		Context("PreDelete", func() {
			var (
				foreignPod          interface{}
				podGot              interface{}
				homePodGetterCalled int
				homePodGetterError  error

				buffer *bytes.Buffer
				flags  *flag.FlagSet
			)

			When("it's not possible to get a home pod for the foreign pod", func() {
				BeforeEach(func() {
					// setup logger
					buffer = &bytes.Buffer{}
					flags = &flag.FlagSet{}
					klog.InitFlags(flags)
					_ = flags.Set("logtostderr", "false")
					_ = flags.Set("v", "5")
					klog.SetOutput(buffer)

					// setup getHomePod mock to return an error
					homePodGetterCalled = 0
					reflector.HomePodGetter = func(reflector ri.APIReflector, foreignPod *corev1.Pod) (*corev1.Pod, error) {
						homePodGetterCalled++
						homePodGetterError = errors.New("home pod not found")
						return nil, homePodGetterError
					}

					// trigger unit under test
					foreignPod = &corev1.Pod{}
					podGot, _ = reflector.PreDelete(foreignPod)

					klog.Flush()
				})

				It("should call GetHomePod once", func() {
					Expect(homePodGetterCalled).To(Equal(1))
				})

				It("should return nil", func() {
					Expect(podGot).To(BeNil())
				})

				It("should log an error", func() {
					Expect(buffer.String()).To(ContainSubstring("cannot get home pod for foreign pod"))
					Expect(buffer.String()).To(ContainSubstring(homePodGetterError.Error()))
				})

			})

			When("it's possible to get a home pod for the foreign pod", func() {
				var (
					foreignPod *corev1.Pod
				)

				BeforeEach(func() {
					// setup getHomePod mock to return a valid pod object
					homePodGetterCalled = 0
					reflector.HomePodGetter = func(reflector ri.APIReflector, foreignPod *corev1.Pod) (*corev1.Pod, error) {
						homePodGetterCalled++
						return &corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name: "homePodName",
							},
							Status: corev1.PodStatus{
								ContainerStatuses: []corev1.ContainerStatus{
									{
										State: corev1.ContainerState{
											Running: &corev1.ContainerStateRunning{},
										},
									},
								},
							},
						}, nil
					}

					// configure foreign pod and put it in blacklist
					foreignPod = &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "homePodName-foreign",
							Namespace: "preDeleteNamespace-natted",
						},
					}
					foreignPodKey := reflector.Keyer(foreignPod.Namespace, foreignPod.Name)
					reflectors.Blacklist[apimgmt.Pods][foreignPodKey] = struct{}{}

					// trigger unit under test
					podGot, _ = reflector.PreDelete(foreignPod)
				})

				It("should call GetHomePod once", func() {
					Expect(homePodGetterCalled).To(Equal(1))
				})

				It("should return the deleted home pod", func() {
					Expect(podGot).NotTo(BeNil())
					homePod := podGot.(*corev1.Pod)
					Expect(homePod.Name).To(Equal("homePodName"))
				})

				It("should remove the foreign pod from the black list", func() {
					foreignPodKey := reflector.Keyer(foreignPod.Namespace, foreignPod.Name)
					podsBlackList := reflectors.Blacklist[apimgmt.Pods]
					Expect(podsBlackList).NotTo(ContainElement(foreignPodKey))
				})

				It("should set each container statuses to terminated", func() {
					homePod := podGot.(*corev1.Pod)
					containerStatuses := homePod.Status.ContainerStatuses
					Expect(containerStatuses).To(HaveLen(1))

					for _, status := range containerStatuses {
						Expect(status.State.Terminated).NotTo(BeNil())
					}
				})
			})
		})

		When("caches are empty", func() {
			Context("PreAdd", func() {
				type addTestcase struct {
					input          *corev1.Pod
					expectedOutput types.GomegaMatcher
				}

				DescribeTable("PreAdd test cases",
					func(c addTestcase) {
						ret, _ := reflector.PreProcessAdd(c.input)
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

		When("caches are not empty", func() {
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
					namespaceNattingTable.NewNamespace("homeNamespace")
					_ = cacheManager.AddHomeNamespace("homeNamespace")
					_ = cacheManager.AddForeignNamespace("homeNamespace-natted")

					cacheManager.AddHomeEntry("homeNamespace", apimgmt.Pods, homePod)
				})

				It("correct foreign pod added", func() {
					ret, _ := reflector.PreProcessAdd(foreignPod)
					Expect(ret).NotTo(BeNil())
					Expect(ret.(*corev1.Pod).Name).To(Equal(homePod.Name))
					Expect(ret.(*corev1.Pod).Namespace).To(Equal(homePod.Namespace))
					Expect(ret.(*corev1.Pod).Status).To(Equal(foreignPod.Status))
				})

				It("correct foreign pod updated", func() {
					ret, _ := reflector.PreProcessUpdate(foreignPod, nil)
					Expect(ret).NotTo(BeNil())
					Expect(ret.(*corev1.Pod).Name).To(Equal(homePod.Name))
					Expect(ret.(*corev1.Pod).Namespace).To(Equal(homePod.Namespace))
					Expect(ret.(*corev1.Pod).Status).To(Equal(foreignPod.Status))
				})
			})
		})

		Describe("GetHomePodFunc", func() {
			When("a home pod doesn't have labels", func() {
				var (
					foreignPod *corev1.Pod
					err        error
				)

				BeforeEach(func() {
					foreignPod = &corev1.Pod{}
					_, err = incoming.GetHomePodFunc(reflector, foreignPod)
				})

				It("should error", func() {
					Expect(err).NotTo(BeNil())
					Expect(err.Error()).To(Equal("foreign pod labels not found"))
				})
			})

			When("a home pod doesn't have the label requested for foreign to home pod translation", func() {
				var (
					foreignPod *corev1.Pod
					err        error
				)

				BeforeEach(func() {
					foreignPod = &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "liqo",
							},
						},
					}

					_, err = incoming.GetHomePodFunc(reflector, foreignPod)
				})

				It("should error", func() {
					Expect(err).NotTo(BeNil())
					expectedErrorMessage := fmt.Sprintf("foreign pod label with key: %s, not found", virtualKubelet.ReflectedpodKey)
					Expect(err.Error()).To(Equal(expectedErrorMessage))
				})
			})

			When("cannot denat namespace", func() {
				var (
					foreignPod *corev1.Pod
					err        error
				)

				BeforeEach(func() {
					foreignPod = &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								virtualKubelet.ReflectedpodKey: "testHomePod",
							},
						},
					}

					_, err = incoming.GetHomePodFunc(reflector, foreignPod)
				})

				It("should error", func() {
					Expect(err).NotTo(BeNil())
					Expect(err.Error()).To(ContainSubstring("cannot get home pod namespace"))
				})
			})

			When("cannot get home pod from cache", func() {
				var (
					foreignPod *corev1.Pod
					err        error
				)

				BeforeEach(func() {
					namespaceNattingTable.NewNamespace("testNamespace")
					nattedNamespace, testSetupError := namespaceNattingTable.NatNamespace("testNamespace")
					if testSetupError != nil {
						Fail("failed to setup test: cannot nat namespace using fake namespace natter")
					}

					foreignPod = &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								virtualKubelet.ReflectedpodKey: "testHomePod",
							},
							Namespace: nattedNamespace,
						},
					}

					_, err = incoming.GetHomePodFunc(reflector, foreignPod)
				})

				It("should error", func() {
					Expect(err).NotTo(BeNil())
					Expect(err.Error()).To(ContainSubstring("cannot get home pod from cache manager"))
				})
			})

			When("cache misbehaving - returns non-pod object", func() {
				var (
					foreignPod   *corev1.Pod
					nonPodObject interface{}
					err          error
				)

				BeforeEach(func() {
					homeNamespace := "cacheMisbehaving"
					namespaceNattingTable.NewNamespace(homeNamespace)
					nattedNamespace, testSetupError := namespaceNattingTable.NatNamespace(homeNamespace)
					if testSetupError != nil {
						Fail("failed to setup test: cannot nat namespace using fake namespace natter")
					}

					// setup cache manager
					if testSetupError = cacheManager.AddHomeNamespace(homeNamespace); testSetupError != nil {
						Fail("failed to setup test: cannot setup fake cache manager")
					}

					if testSetupError = cacheManager.AddForeignNamespace(nattedNamespace); testSetupError != nil {
						Fail("failed to setup test: cannot setup fake cache manager")
					}

					nonPodObject = &corev1.Service{}
					nonPodObj, ok := nonPodObject.(metav1.Object)
					if !ok {
						Fail("failed to setup test: cannot complete fake cacheManager setup")
					}
					homePodName := "testCacheMisbehavingHomePod"
					nonPodObj.SetName(homePodName)

					cacheManager.AddHomeEntry(homeNamespace, apimgmt.Pods, nonPodObj)

					foreignPod = &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								virtualKubelet.ReflectedpodKey: homePodName,
							},
							Namespace: nattedNamespace,
						},
					}

					_, err = incoming.GetHomePodFunc(reflector, foreignPod)
				})

				It("should error", func() {
					Expect(err).NotTo(BeNil())
					Expect(err.Error()).To(ContainSubstring("could not execute type conversion: GetHomeNamespacedObject expected to return a Pod object"))
				})
			})

			When("it's possible to retrieve home pod for foreign pod", func() {
				var (
					foreignPod *corev1.Pod
					homePod    interface{}
					err        error
				)

				BeforeEach(func() {
					// setup namespace natter
					homeNamespace := "GetHomePodNamespace"
					namespaceNattingTable.NewNamespace(homeNamespace)
					nattedNamespace, testSetupError := namespaceNattingTable.NatNamespace(homeNamespace)
					if testSetupError != nil {
						Fail("failed to setup test: cannot nat namespace using fake namespace natter")
					}

					// setup cache manager
					if testSetupError = cacheManager.AddHomeNamespace(homeNamespace); testSetupError != nil {
						Fail("failed to setup test: cannot setup fake cache manager")
					}

					if testSetupError = cacheManager.AddForeignNamespace(nattedNamespace); testSetupError != nil {
						Fail("failed to setup test: cannot setup fake cache manager")
					}

					homePod = &corev1.Pod{}
					homePodObj, ok := homePod.(metav1.Object)
					if !ok {
						Fail("failed to setup test: cannot complete fake cacheManager setup")
					}
					homePodName := "testHomePod"
					homePodObj.SetName(homePodName)
					cacheManager.AddHomeEntry(homeNamespace, apimgmt.Pods, homePodObj)

					// configure foreign pod
					foreignPod = &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								virtualKubelet.ReflectedpodKey: homePodName,
							},
							Namespace: nattedNamespace,
						},
					}

					// trigger unit under test
					homePod, err = incoming.GetHomePodFunc(reflector, foreignPod)
				})

				It("should return home pod", func() {
					Expect(err).To(BeNil())
					Expect(homePod).NotTo(BeNil())
					Expect(homePod).To(BeAssignableToTypeOf(&corev1.Pod{}))
				})
			})
		})
	})
})
