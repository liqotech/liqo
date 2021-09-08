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

package storage

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/storage/test"
	"github.com/liqotech/liqo/pkg/virtualKubelet/utils"
)

var _ = Describe("CacheManager", func() {
	var (
		homeClient, foreignClient *fake.Clientset
	)

	BeforeEach(func() {
		homeClient = fake.NewSimpleClientset()
		foreignClient = fake.NewSimpleClientset()
	})

	Describe("cache manager workflow", func() {
		Context("cache manager malformed", func() {
			var (
				manager1   *Manager
				err1, err2 error
			)
			BeforeEach(func() {
				manager1 = &Manager{}
			})

			It("checking AddHomeNamespace failure", func() {
				err1 = manager1.AddHomeNamespace(test.HomeNamespace)
				Expect(err1).To(HaveOccurred())
				err2 = manager1.AddForeignNamespace(test.ForeignNamespace)
				Expect(err2).To(HaveOccurred())
			})
		})

		Context("cache manager correctly formed", func() {
			const informerResyncPeriod = 1 * time.Minute
			var (
				manager *Manager
				err     error
			)

			BeforeEach(func() {
				manager = NewManager(homeClient, foreignClient, informerResyncPeriod)
			})

			Context("cache Manager check", func() {
				It("all the manager fields must be allocated", func() {
					Expect(manager).NotTo(BeNil())
					Expect(manager.homeInformers).NotTo(BeNil())
					Expect(manager.foreignInformers).NotTo(BeNil())
					Expect(manager.homeInformers.apiInformers).NotTo(BeNil())
					Expect(manager.homeInformers.apiInformers).NotTo(BeNil())
					Expect(manager.foreignInformers.informerFactories).NotTo(BeNil())
					Expect(manager.foreignInformers.informerFactories).NotTo(BeNil())
				})
			})

			Context("With correct Namespace addiction", func() {
				var (
					stop = make(chan struct{})
				)

				BeforeEach(func() {
					err = manager.AddHomeNamespace(test.HomeNamespace)
					Expect(err).NotTo(HaveOccurred())
					err = manager.AddForeignNamespace(test.ForeignNamespace)
					Expect(err).NotTo(HaveOccurred())
				})

				It("check ApiCaches existence", func() {
					Expect(manager.homeInformers.Namespace(test.HomeNamespace)).NotTo(BeNil())
					Expect(manager.foreignInformers.Namespace(test.ForeignNamespace)).NotTo(BeNil())
				})

				Context("with active namespace mapping", func() {
					var (
						homeHandlers    = &cache.ResourceEventHandlerFuncs{}
						foreignHandlers = &cache.ResourceEventHandlerFuncs{}
					)

					BeforeEach(func() {
						By("start informers")
						err = manager.StartHomeNamespace(test.HomeNamespace, stop)
						Expect(err).NotTo(HaveOccurred())
						err = manager.StartForeignNamespace(test.ForeignNamespace, stop)
						Expect(err).NotTo(HaveOccurred())

						manager.homeInformers.informerFactories[test.HomeNamespace].WaitForCacheSync(stop)
						manager.foreignInformers.informerFactories[test.ForeignNamespace].WaitForCacheSync(stop)
					})

					Context("getter functions", func() {
						BeforeEach(func() {
							By("create pods")
							_ = manager.homeInformers.apiInformers[test.HomeNamespace].caches[apimgmt.Pods].GetIndexer().Add(test.Pods[utils.Keyer(test.HomeNamespace, test.Pod1)])
							_ = manager.homeInformers.apiInformers[test.HomeNamespace].caches[apimgmt.Pods].GetIndexer().Add(test.Pods[utils.Keyer(test.HomeNamespace, test.Pod2)])
							_ = manager.foreignInformers.apiInformers[test.ForeignNamespace].caches[apimgmt.Pods].GetIndexer().Add(test.Pods[utils.Keyer(test.ForeignNamespace, test.Pod1)])
							_ = manager.foreignInformers.apiInformers[test.ForeignNamespace].caches[apimgmt.Pods].GetIndexer().Add(test.Pods[utils.Keyer(test.ForeignNamespace, test.Pod2)])
						})

						It("get Objects", func() {
							By("home pod")
							obj, err := manager.GetHomeNamespacedObject(apimgmt.Pods, test.HomeNamespace, test.Pod1)
							Expect(err).NotTo(HaveOccurred())
							Expect(obj).To(Equal(test.Pods[utils.Keyer(test.HomeNamespace, test.Pod1)]))

							By("foreign pod")
							obj, err = manager.GetForeignNamespacedObject(apimgmt.Pods, test.ForeignNamespace, test.Pod1)
							Expect(err).NotTo(HaveOccurred())
							Expect(obj).To(Equal(test.Pods[utils.Keyer(test.ForeignNamespace, test.Pod1)]))
						})

						It("List Objects", func() {
							By("home pods")
							objs, err := manager.ListHomeNamespacedObject(apimgmt.Pods, test.HomeNamespace)
							Expect(err).NotTo(HaveOccurred())
							Expect(len(objs)).To(Equal(2))

							By("foreign pod")
							objs, err = manager.ListForeignNamespacedObject(apimgmt.Pods, test.ForeignNamespace)
							Expect(err).NotTo(HaveOccurred())
							Expect(len(objs)).To(Equal(2))
						})

						It("resync list objects", func() {
							By("home pods")
							objs, err := manager.ListHomeNamespacedObject(apimgmt.Pods, test.HomeNamespace)
							Expect(err).NotTo(HaveOccurred())
							Expect(len(objs)).To(Equal(2))

							By("foreign pod")
							objs, err = manager.ListForeignNamespacedObject(apimgmt.Pods, test.ForeignNamespace)
							Expect(err).NotTo(HaveOccurred())
							Expect(len(objs)).To(Equal(2))
						})
					})

					Context("Handlers setting", func() {
						It("set handlers", func() {
							By("home pods")
							err = manager.AddHomeEventHandlers(apimgmt.Pods, test.HomeNamespace, homeHandlers)
							Expect(err).NotTo(HaveOccurred())

							By("foreign pod")
							err = manager.AddForeignEventHandlers(apimgmt.Pods, test.ForeignNamespace, foreignHandlers)
							Expect(err).NotTo(HaveOccurred())
						})
					})
				})
			})
			Context("with incorrect namespace addiction", func() {
				It("get Objects", func() {
					By("home pod")
					_, err = manager.GetHomeNamespacedObject(apimgmt.Pods, test.HomeNamespace, test.Pod1)
					Expect(err).To(HaveOccurred())

					By("foreign pod")
					_, err = manager.GetForeignNamespacedObject(apimgmt.Pods, test.ForeignNamespace, test.Pod1)
					Expect(err).To(HaveOccurred())
				})

				It("List Objects", func() {
					By("home pods")
					_, err := manager.ListHomeNamespacedObject(apimgmt.Pods, test.HomeNamespace)
					Expect(err).To(HaveOccurred())

					By("foreign pod")
					_, err = manager.ListForeignNamespacedObject(apimgmt.Pods, test.ForeignNamespace)
					Expect(err).To(HaveOccurred())
				})

				It("resync list objects", func() {
					By("home pods")
					_, err := manager.ListHomeNamespacedObject(apimgmt.Pods, test.HomeNamespace)
					Expect(err).To(HaveOccurred())

					By("foreign pod")
					_, err = manager.ListForeignNamespacedObject(apimgmt.Pods, test.ForeignNamespace)
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})
