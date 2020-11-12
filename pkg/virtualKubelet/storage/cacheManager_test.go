package storage

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/utils"
	. "github.com/liqotech/liqo/test/unit/virtualKubelet/storage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
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
				err1 = manager1.AddHomeNamespace(HomeNamespace)
				Expect(err1).To(HaveOccurred())
				err2 = manager1.AddForeignNamespace(ForeignNamespace)
				Expect(err2).To(HaveOccurred())
			})
		})

		Context("cache manager correctly formed", func() {
			var (
				manager *Manager
				err     error
			)

			BeforeEach(func() {
				manager = NewManager(homeClient, foreignClient)
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
					err = manager.AddHomeNamespace(HomeNamespace)
					Expect(err).NotTo(HaveOccurred())
					err = manager.AddForeignNamespace(ForeignNamespace)
					Expect(err).NotTo(HaveOccurred())
				})

				It("check ApiCaches existence", func() {
					Expect(manager.homeInformers.Namespace(HomeNamespace)).NotTo(BeNil())
					Expect(manager.foreignInformers.Namespace(ForeignNamespace)).NotTo(BeNil())
				})

				Context("with active namespace mapping", func() {
					var (
						homeHandlers    = &cache.ResourceEventHandlerFuncs{}
						foreignHandlers = &cache.ResourceEventHandlerFuncs{}
					)

					BeforeEach(func() {
						By("start informers")
						err = manager.StartHomeNamespace(HomeNamespace, stop)
						Expect(err).NotTo(HaveOccurred())
						err = manager.StartForeignNamespace(ForeignNamespace, stop)
						Expect(err).NotTo(HaveOccurred())

						manager.homeInformers.informerFactories[HomeNamespace].WaitForCacheSync(stop)
						manager.foreignInformers.informerFactories[ForeignNamespace].WaitForCacheSync(stop)
					})

					Context("getter functions", func() {
						BeforeEach(func() {
							By("create pods")
							_ = manager.homeInformers.apiInformers[HomeNamespace].caches[apimgmt.Pods].GetIndexer().Add(Pods[utils.Keyer(HomeNamespace, Pod1)])
							_ = manager.homeInformers.apiInformers[HomeNamespace].caches[apimgmt.Pods].GetIndexer().Add(Pods[utils.Keyer(HomeNamespace, Pod2)])
							_ = manager.foreignInformers.apiInformers[ForeignNamespace].caches[apimgmt.Pods].GetIndexer().Add(Pods[utils.Keyer(ForeignNamespace, Pod1)])
							_ = manager.foreignInformers.apiInformers[ForeignNamespace].caches[apimgmt.Pods].GetIndexer().Add(Pods[utils.Keyer(ForeignNamespace, Pod2)])
						})

						It("get Objects", func() {
							By("home pod")
							obj, err := manager.GetHomeNamespacedObject(apimgmt.Pods, HomeNamespace, Pod1)
							Expect(err).NotTo(HaveOccurred())
							Expect(obj).To(Equal(Pods[utils.Keyer(HomeNamespace, Pod1)]))

							By("foreign pod")
							obj, err = manager.GetForeignNamespacedObject(apimgmt.Pods, ForeignNamespace, Pod1)
							Expect(err).NotTo(HaveOccurred())
							Expect(obj).To(Equal(Pods[utils.Keyer(ForeignNamespace, Pod1)]))
						})

						It("List Objects", func() {
							By("home pods")
							objs, err := manager.ListHomeNamespacedObject(apimgmt.Pods, HomeNamespace)
							Expect(err).NotTo(HaveOccurred())
							Expect(len(objs)).To(Equal(2))

							By("foreign pod")
							objs, err = manager.ListForeignNamespacedObject(apimgmt.Pods, ForeignNamespace)
							Expect(err).NotTo(HaveOccurred())
							Expect(len(objs)).To(Equal(2))
						})

						It("resync list objects", func() {
							By("home pods")
							objs, err := manager.ResyncListHomeNamespacedObject(apimgmt.Pods, HomeNamespace)
							Expect(err).NotTo(HaveOccurred())
							Expect(len(objs)).To(Equal(2))

							By("foreign pod")
							objs, err = manager.ResyncListForeignNamespacedObject(apimgmt.Pods, ForeignNamespace)
							Expect(err).NotTo(HaveOccurred())
							Expect(len(objs)).To(Equal(2))
						})
					})

					Context("Handlers setting", func() {
						It("set handlers", func() {
							By("home pods")
							err = manager.AddHomeEventHandlers(apimgmt.Pods, HomeNamespace, homeHandlers)
							Expect(err).NotTo(HaveOccurred())

							By("foreign pod")
							err = manager.AddForeignEventHandlers(apimgmt.Pods, ForeignNamespace, foreignHandlers)
							Expect(err).NotTo(HaveOccurred())
						})
					})
				})
			})
			Context("with incorrect namespace addiction", func() {
				It("get Objects", func() {
					By("home pod")
					_, err = manager.GetHomeNamespacedObject(apimgmt.Pods, HomeNamespace, Pod1)
					Expect(err).To(HaveOccurred())

					By("foreign pod")
					_, err = manager.GetForeignNamespacedObject(apimgmt.Pods, ForeignNamespace, Pod1)
					Expect(err).To(HaveOccurred())
				})

				It("List Objects", func() {
					By("home pods")
					_, err := manager.ListHomeNamespacedObject(apimgmt.Pods, HomeNamespace)
					Expect(err).To(HaveOccurred())

					By("foreign pod")
					_, err = manager.ListForeignNamespacedObject(apimgmt.Pods, ForeignNamespace)
					Expect(err).To(HaveOccurred())
				})

				It("resync list objects", func() {
					By("home pods")
					_, err := manager.ResyncListHomeNamespacedObject(apimgmt.Pods, HomeNamespace)
					Expect(err).To(HaveOccurred())

					By("foreign pod")
					_, err = manager.ResyncListForeignNamespacedObject(apimgmt.Pods, ForeignNamespace)
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})
