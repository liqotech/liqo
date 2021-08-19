package discovery

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/grandcat/zeroconf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	"github.com/liqotech/liqo/apis/config/v1alpha1"
	v1alpha12 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/clusterid/test"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

func TestDiscovery(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Discovery Suite")
}

var _ = Describe("Discovery", func() {

	// --- AuthData ---

	Describe("AuthData", func() {

		var (
			entry zeroconf.ServiceEntry
		)

		BeforeEach(func() {
			entry = zeroconf.ServiceEntry{
				HostName: "test.example.com",
				Port:     53,
				TTL:      45,
				AddrIPv4: []net.IP{
					{8, 8, 8, 8},
				},
			}
		})

		Context("Decode", func() {
			type decodeTestcase struct {
				input          *zeroconf.ServiceEntry
				expectedOutput types.GomegaMatcher
			}

			DescribeTable("DNS entry table",
				func(c decodeTestcase) {
					var data AuthData
					err := data.decode(c.input, 1*time.Second)
					Expect(err).To(c.expectedOutput)
				},

				Entry("valid entry", decodeTestcase{
					input:          &entry,
					expectedOutput: BeNil(),
				}),

				Entry("not reachable ip", decodeTestcase{
					input: &zeroconf.ServiceEntry{
						HostName: "test.example.com",
						Port:     443,
						TTL:      45,
						AddrIPv4: []net.IP{
							{1, 2, 3, 4},
						},
					},
					expectedOutput: HaveOccurred(),
				}),

				Entry("not reachable port", decodeTestcase{
					input: &zeroconf.ServiceEntry{
						HostName: "test.example.com",
						Port:     4433,
						TTL:      45,
						AddrIPv4: []net.IP{
							{8, 8, 8, 8},
						},
					},
					expectedOutput: HaveOccurred(),
				}),

				Entry("loopback address", decodeTestcase{
					input: &zeroconf.ServiceEntry{
						HostName: "test.example.com",
						Port:     22,
						TTL:      45,
						AddrIPv4: []net.IP{
							{127, 0, 0, 1},
						},
					},
					expectedOutput: HaveOccurred(),
				}),
			)

			It("check that the data are correct", func() {
				By("Decode AuthData")
				var data AuthData
				err := data.decode(&entry, 1*time.Second)
				Expect(err).To(BeNil())

				targetData := AuthData{
					address: "8.8.8.8",
					port:    53,
					ttl:     45,
				}
				Expect(data).To(Equal(targetData))
			})
		})

		Context("IsComplete", func() {
			type isCompleteTestcase struct {
				input          AuthData
				expectedOutput types.GomegaMatcher
			}

			DescribeTable("AuthData table",
				func(c isCompleteTestcase) {
					isComplete := c.input.IsComplete()
					Expect(isComplete).To(c.expectedOutput)
				},

				Entry("valid entry", isCompleteTestcase{
					input: AuthData{
						address: "1.2.3.4",
						port:    443,
						ttl:     10,
					},
					expectedOutput: BeTrue(),
				}),

				Entry("invalid entry 1", isCompleteTestcase{
					input: AuthData{
						address: "",
						port:    443,
						ttl:     10,
					},
					expectedOutput: BeFalse(),
				}),

				Entry("invalid entry 2", isCompleteTestcase{
					input: AuthData{
						address: "1.2.3.4",
						port:    0,
						ttl:     10,
					},
					expectedOutput: BeFalse(),
				}),
			)
		})

		It("GetUrl", func() {
			By("Get Url")

			var data AuthData
			err := data.decode(&entry, 1*time.Second)
			Expect(err).To(BeNil())

			url := data.getURL()
			Expect(url).To(Equal("https://8.8.8.8:53"))
		})

	})

	// --- DiscoveryCache ---

	Describe("DiscoveryCache", func() {

		var (
			cache discoveryCache
		)

		BeforeEach(func() {
			cache = discoveryCache{
				discoveredServices: map[string]discoveryData{},
			}
		})

		It("add", func() {
			data := NewAuthData("1.2.3.4", 1234, 30)

			cache.add("test", data)
			Expect(len(cache.discoveredServices)).To(Equal(1))

			dataR, err := cache.get("test")
			Expect(err).To(BeNil())
			Expect(dataR.AuthData).To(Equal(data))

			data2 := *data
			data2.address = "2.3.4.5"

			cache.add("test", &data2)
			Expect(len(cache.discoveredServices)).To(Equal(1))

			dataR, err = cache.get("test")
			Expect(err).To(BeNil())
			Expect(*dataR.AuthData).To(Equal(data2))
		})

		It("get", func() {
			data, err := cache.get("test")
			Expect(err).To(HaveOccurred())
			Expect(data).To(BeNil())
		})

		It("isComplete", func() {
			data := NewAuthData("1.2.3.4", 1234, 30)

			cache.add("test", data)
			Expect(len(cache.discoveredServices)).To(Equal(1))

			isComplete := cache.isComplete("test")
			Expect(isComplete).To(BeTrue())
		})

		It("delete", func() {
			data := NewAuthData("1.2.3.4", 1234, 30)

			cache.add("test", data)
			Expect(len(cache.discoveredServices)).To(Equal(1))

			cache.delete("test")
			Expect(len(cache.discoveredServices)).To(Equal(0))
		})

	})

	// --- DiscoveryCtrl ---

	Describe("DiscoveryCtrl", func() {

		var (
			discoveryCtrl Controller
		)

		BeforeEach(func() {
			cID := &test.ClusterIDMock{}
			_ = cID.SetupClusterID("default")

			discoveryCtrl = Controller{
				Namespace: "default",
				Config: &v1alpha1.DiscoveryConfig{
					AuthService:         "_liqo_auth._tcp",
					ClusterName:         "Name",
					AutoJoin:            true,
					Domain:              "local.",
					EnableAdvertisement: false,
					EnableDiscovery:     false,
					Name:                "MyLiqo",
					Port:                6443,
					Service:             "_liqo_api._tcp",
					TTL:                 90,
				},
				LocalClusterID: cID,
				stopMDNS:       make(chan bool, 1),
				stopMDNSClient: make(chan bool, 1),
			}
		})

		Context("Config", func() {
			type configTestcase struct {
				initialConfig        *v1alpha1.DiscoveryConfig
				changedConfig        v1alpha1.DiscoveryConfig
				expectedOutputServer types.GomegaMatcher
				expectedOutputClient types.GomegaMatcher
			}

			DescribeTable("DiscoveryConfig table",
				func(c configTestcase) {
					discoveryCtrl.Config = c.initialConfig

					hasItemsServer := make(chan bool, 1)
					hasItemsClient := make(chan bool, 1)

					go func() {
						select {
						case <-discoveryCtrl.stopMDNS:
							hasItemsServer <- true
						case <-time.NewTimer(time.Second * 1).C:
							hasItemsServer <- false
						}
					}()

					go func() {
						select {
						case <-discoveryCtrl.stopMDNSClient:
							hasItemsClient <- true
						case <-time.NewTimer(time.Second * 1).C:
							hasItemsClient <- false
						}
					}()

					discoveryCtrl.handleConfiguration(&c.changedConfig)

					Expect(<-hasItemsServer).To(c.expectedOutputServer)
					Expect(<-hasItemsClient).To(c.expectedOutputClient)

					close(hasItemsClient)
					close(hasItemsServer)
				},

				Entry("no change", configTestcase{
					initialConfig: &v1alpha1.DiscoveryConfig{
						ClusterName:         "Name",
						AutoJoin:            true,
						Domain:              "local.",
						EnableAdvertisement: false,
						EnableDiscovery:     false,
						Name:                "MyLiqo",
						Port:                6443,
						Service:             "_liqo_api._tcp",
						TTL:                 90,
					},
					changedConfig: v1alpha1.DiscoveryConfig{
						ClusterName:         "Name",
						AutoJoin:            true,
						Domain:              "local.",
						EnableAdvertisement: false,
						EnableDiscovery:     false,
						Name:                "MyLiqo",
						Port:                6443,
						Service:             "_liqo_api._tcp",
						TTL:                 90,
					},
					expectedOutputServer: BeFalse(),
					expectedOutputClient: BeFalse(),
				}),

				Entry("reload server", configTestcase{
					initialConfig: &v1alpha1.DiscoveryConfig{
						ClusterName:         "Name",
						AutoJoin:            true,
						Domain:              "local.",
						EnableAdvertisement: false,
						EnableDiscovery:     false,
						Name:                "MyLiqo",
						Port:                6443,
						Service:             "_liqo_api._tcp",
						TTL:                 90,
					},
					changedConfig: v1alpha1.DiscoveryConfig{
						ClusterName:         "Name",
						AutoJoin:            true,
						Domain:              "local.",
						EnableAdvertisement: false,
						EnableDiscovery:     false,
						Name:                "MyLiqo",
						Port:                443,
						Service:             "_liqo_api._tcp",
						TTL:                 90,
					},
					expectedOutputServer: BeTrue(),
					expectedOutputClient: BeFalse(),
				}),

				Entry("reload client", configTestcase{
					initialConfig: &v1alpha1.DiscoveryConfig{
						ClusterName:         "Name",
						AutoJoin:            true,
						Domain:              "local.",
						EnableAdvertisement: false,
						EnableDiscovery:     false,
						Name:                "MyLiqo",
						Port:                6443,
						Service:             "_liqo_api._tcp",
						TTL:                 90,
					},
					changedConfig: v1alpha1.DiscoveryConfig{
						ClusterName:         "Name",
						AutoJoin:            true,
						Domain:              "local.",
						EnableAdvertisement: false,
						EnableDiscovery:     false,
						Name:                "MyLiqo",
						Port:                6443,
						Service:             "_test._liqo_api._tcp",
						TTL:                 90,
					},
					expectedOutputServer: BeTrue(),
					expectedOutputClient: BeTrue(),
				}),
			)
		})

		Describe("ForeignCluster management", func() {

			var (
				cluster testutil.Cluster
				cID     test.ClusterIDMock
			)

			BeforeSuite(func() {
				cID = test.ClusterIDMock{}
				_ = cID.SetupClusterID("default")
			})

			BeforeEach(func() {
				var err error
				cluster, _, err = testutil.NewTestCluster([]string{filepath.Join("..", "..", "deployments", "liqo", "crds")})
				if err != nil {
					By(err.Error())
					os.Exit(1)
				}

				discoveryCtrl.crdClient = cluster.GetClient()
			})

			AfterEach(func() {
				err := cluster.GetEnv().Stop()
				if err != nil {
					By(err.Error())
					os.Exit(1)
				}
			})

			type updateForeignTestcase struct {
				data            discoveryData
				expectedLength  types.GomegaMatcher
				expectedPeering types.GomegaMatcher
				expectedSdLabel types.GomegaMatcher
			}

			Context("UpdateForeignLAN", func() {

				DescribeTable("UpdateForeign table",
					func(c updateForeignTestcase) {
						discoveryCtrl.updateForeignLAN(&c.data)
						obj, err := discoveryCtrl.crdClient.Resource("foreignclusters").List(&metav1.ListOptions{})
						Expect(err).To(BeNil())
						Expect(obj).NotTo(BeNil())

						fcs, ok := obj.(*v1alpha12.ForeignClusterList)
						Expect(ok).To(BeTrue())
						Expect(len(fcs.Items)).To(c.expectedLength)

						if len(fcs.Items) > 0 {
							fc := fcs.Items[0]
							Expect(fc.GetAnnotations()[discovery.LastUpdateAnnotation]).NotTo(BeEmpty())
							Expect(fc.Spec.OutgoingPeeringEnabled).To(c.expectedPeering)
							Expect(fc.GetAnnotations()[discovery.SearchDomainLabel]).To(c.expectedSdLabel)
						}
					},

					Entry("local cluster", updateForeignTestcase{
						data: discoveryData{
							AuthData: NewAuthData("1.2.3.4", 1234, 30),
							ClusterInfo: &auth.ClusterInfo{
								ClusterID:   "local-cluster",
								ClusterName: "ClusterTest1",
							},
						},
						expectedLength:  Equal(0),
						expectedPeering: Equal(v1alpha12.PeeringEnabledAuto),
						expectedSdLabel: BeEmpty(),
					}),

					Entry("foreign cluster (untrusted)", updateForeignTestcase{
						data: discoveryData{
							AuthData: NewAuthData("1.2.3.4", 1234, 30),
							ClusterInfo: &auth.ClusterInfo{
								ClusterID:   "foreign-cluster",
								ClusterName: "ClusterTest2",
							},
						},
						expectedLength:  Equal(1),
						expectedPeering: Equal(v1alpha12.PeeringEnabledAuto),
						expectedSdLabel: BeEmpty(),
					}),

					Entry("foreign cluster (trusted)", updateForeignTestcase{
						data: discoveryData{
							AuthData: NewAuthData("1.2.3.4", 1234, 30),
							ClusterInfo: &auth.ClusterInfo{
								ClusterID:   "foreign-cluster",
								ClusterName: "ClusterTest2",
							},
						},
						expectedLength:  Equal(1),
						expectedPeering: Equal(v1alpha12.PeeringEnabledAuto),
						expectedSdLabel: BeEmpty(),
					}),
				)
			})

			Context("Update existing", func() {

				var (
					updateTime string
				)

				BeforeEach(func() {
					discoveryCtrl.updateForeignLAN(&discoveryData{
						AuthData: NewAuthData("1.2.3.4", 1234, 30),
						ClusterInfo: &auth.ClusterInfo{
							ClusterID:   "foreign-cluster",
							ClusterName: "ClusterTest2",
						},
					})

					obj, _ := discoveryCtrl.crdClient.Resource("foreignclusters").List(&metav1.ListOptions{})
					fcs, _ := obj.(*v1alpha12.ForeignClusterList)
					fc := fcs.Items[0]

					updateTime = fc.GetAnnotations()[discovery.LastUpdateAnnotation]

					// I need to wait that at least a second is passed
					time.Sleep(time.Second * 1)
				})

				DescribeTable("UpdateForeign table",
					func(c updateForeignTestcase) {
						discoveryCtrl.updateForeignLAN(&c.data)
						obj, err := discoveryCtrl.crdClient.Resource("foreignclusters").List(&metav1.ListOptions{})
						Expect(err).To(BeNil())
						Expect(obj).NotTo(BeNil())

						fcs, ok := obj.(*v1alpha12.ForeignClusterList)
						Expect(ok).To(BeTrue())
						Expect(len(fcs.Items)).To(c.expectedLength)

						if len(fcs.Items) > 0 {
							fc := fcs.Items[0]
							Expect(fc.GetAnnotations()[discovery.LastUpdateAnnotation]).NotTo(BeEmpty())
							Expect(fc.GetAnnotations()[discovery.LastUpdateAnnotation]).NotTo(Equal(updateTime))
							Expect(fc.Spec.OutgoingPeeringEnabled).To(c.expectedPeering)
							Expect(fc.GetAnnotations()[discovery.SearchDomainLabel]).To(c.expectedSdLabel)
						}
					},

					Entry("no update", updateForeignTestcase{
						data: discoveryData{
							AuthData: NewAuthData("1.2.3.4", 1234, 30),
							ClusterInfo: &auth.ClusterInfo{
								ClusterID:   "foreign-cluster",
								ClusterName: "ClusterTest2",
							},
						},
						expectedLength:  Equal(1),
						expectedPeering: Equal(v1alpha12.PeeringEnabledAuto),
						expectedSdLabel: BeEmpty(),
					}),

					Entry("update", updateForeignTestcase{
						data: discoveryData{
							AuthData: NewAuthData("1.2.3.4", 1234, 30),
							ClusterInfo: &auth.ClusterInfo{
								ClusterID:   "foreign-cluster",
								ClusterName: "ClusterTest2",
							},
						},
						expectedLength:  Equal(1),
						expectedPeering: Equal(v1alpha12.PeeringEnabledAuto),
						expectedSdLabel: BeEmpty(),
					}),
				)

			})

			Context("Update existing (Discovery Priority)", func() {

				BeforeEach(func() {
					fc := &v1alpha12.ForeignCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foreign-cluster",
							Labels: map[string]string{
								discovery.DiscoveryTypeLabel: string(discovery.IncomingPeeringDiscovery),
								discovery.ClusterIDLabel:     "foreign-cluster",
							},
						},
						Spec: v1alpha12.ForeignClusterSpec{
							ClusterIdentity: v1alpha12.ClusterIdentity{
								ClusterID:   "foreign-cluster",
								ClusterName: "ClusterTest2",
							},
							ForeignAuthURL:         "https://example.com",
							OutgoingPeeringEnabled: v1alpha12.PeeringEnabledAuto,
							IncomingPeeringEnabled: v1alpha12.PeeringEnabledAuto,
							InsecureSkipTLSVerify:  pointer.BoolPtr(true),
						},
					}

					_, err := discoveryCtrl.crdClient.Resource("foreignclusters").Create(fc, &metav1.CreateOptions{})
					if err != nil {
						klog.Error(err)
						os.Exit(1)
					}
				})

				DescribeTable("UpdateForeign table",
					func(c updateForeignTestcase) {
						discoveryCtrl.updateForeignLAN(&c.data)
						obj, err := discoveryCtrl.crdClient.Resource("foreignclusters").List(&metav1.ListOptions{})
						Expect(err).To(BeNil())
						Expect(obj).NotTo(BeNil())

						fcs, ok := obj.(*v1alpha12.ForeignClusterList)
						Expect(ok).To(BeTrue())
						Expect(len(fcs.Items)).To(c.expectedLength)

						if len(fcs.Items) > 0 {
							fc := fcs.Items[0]
							Expect(fc.GetAnnotations()[discovery.LastUpdateAnnotation]).NotTo(BeEmpty())
							Expect(fc.Spec.OutgoingPeeringEnabled).To(c.expectedPeering)
							Expect(fc.GetAnnotations()[discovery.SearchDomainLabel]).To(c.expectedSdLabel)
							Expect(foreignclusterutils.GetDiscoveryType(&fc)).To(Equal(discovery.LanDiscovery))
						}
					},

					Entry("update discovery type", updateForeignTestcase{
						data: discoveryData{
							AuthData: NewAuthData("1.2.3.4", 1234, 30),
							ClusterInfo: &auth.ClusterInfo{
								ClusterID:   "foreign-cluster",
								ClusterName: "ClusterTest2",
							},
						},
						expectedLength:  Equal(1),
						expectedPeering: Equal(v1alpha12.PeeringEnabledAuto),
						expectedSdLabel: BeEmpty(),
					}),
				)

			})

			Context("GarbageCollector", func() {

				type garbageCollectorTestcase struct {
					fc             v1alpha12.ForeignCluster
					expectedLength types.GomegaMatcher
				}

				DescribeTable("GarbageCollector table",
					func(c garbageCollectorTestcase) {
						_, err := discoveryCtrl.crdClient.Resource("foreignclusters").Create(&c.fc, &metav1.CreateOptions{})
						Expect(err).To(BeNil())

						err = discoveryCtrl.collectGarbage()
						Expect(err).To(BeNil())

						obj, err := discoveryCtrl.crdClient.Resource("foreignclusters").List(&metav1.ListOptions{})
						Expect(err).To(BeNil())
						Expect(obj).NotTo(BeNil())

						fcs, ok := obj.(*v1alpha12.ForeignClusterList)
						Expect(ok).To(BeTrue())
						Expect(len(fcs.Items)).To(c.expectedLength)
					},

					Entry("no garbage", garbageCollectorTestcase{
						fc: v1alpha12.ForeignCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name: "foreign-cluster",
								Labels: map[string]string{
									discovery.DiscoveryTypeLabel: string(discovery.LanDiscovery),
									discovery.ClusterIDLabel:     "foreign-cluster",
								},
								Annotations: map[string]string{
									discovery.LastUpdateAnnotation: strconv.Itoa(int(time.Now().Unix())),
								},
							},
							Spec: v1alpha12.ForeignClusterSpec{
								ClusterIdentity: v1alpha12.ClusterIdentity{
									ClusterID:   "foreign-cluster",
									ClusterName: "ClusterTest2",
								},
								OutgoingPeeringEnabled: v1alpha12.PeeringEnabledAuto,
								IncomingPeeringEnabled: v1alpha12.PeeringEnabledAuto,
								ForeignAuthURL:         "https://example.com",
								InsecureSkipTLSVerify:  pointer.BoolPtr(true),
								TTL:                    300,
							},
						},

						expectedLength: Equal(1),
					}),

					Entry("garbage", garbageCollectorTestcase{
						fc: v1alpha12.ForeignCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name: "foreign-cluster",
								Labels: map[string]string{
									discovery.DiscoveryTypeLabel: string(discovery.LanDiscovery),
									discovery.ClusterIDLabel:     "foreign-cluster",
								},
								Annotations: map[string]string{
									discovery.LastUpdateAnnotation: strconv.Itoa(int(time.Now().Unix()) - 600),
								},
							},
							Spec: v1alpha12.ForeignClusterSpec{
								ClusterIdentity: v1alpha12.ClusterIdentity{
									ClusterID:   "foreign-cluster",
									ClusterName: "ClusterTest2",
								},
								OutgoingPeeringEnabled: v1alpha12.PeeringEnabledAuto,
								IncomingPeeringEnabled: v1alpha12.PeeringEnabledAuto,
								ForeignAuthURL:         "https://example.com",
								InsecureSkipTLSVerify:  pointer.BoolPtr(true),
								TTL:                    300,
							},
						},

						expectedLength: Equal(0),
					}),

					Entry("no garbage (Manual Discovery)", garbageCollectorTestcase{
						fc: v1alpha12.ForeignCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name: "foreign-cluster",
								Labels: map[string]string{
									discovery.DiscoveryTypeLabel: string(discovery.ManualDiscovery),
									discovery.ClusterIDLabel:     "foreign-cluster",
								},
								Annotations: map[string]string{
									discovery.LastUpdateAnnotation: strconv.Itoa(int(time.Now().Unix()) - 600),
								},
							},
							Spec: v1alpha12.ForeignClusterSpec{
								ClusterIdentity: v1alpha12.ClusterIdentity{
									ClusterID:   "foreign-cluster",
									ClusterName: "ClusterTest2",
								},
								OutgoingPeeringEnabled: v1alpha12.PeeringEnabledAuto,
								IncomingPeeringEnabled: v1alpha12.PeeringEnabledAuto,
								ForeignAuthURL:         "https://example.com",
								InsecureSkipTLSVerify:  pointer.BoolPtr(true),
								TTL:                    300,
							},
						},

						expectedLength: Equal(1),
					}),
				)
			})

			Context("mDNS", func() {

				BeforeEach(func() {
					discoveryCtrl.Config.EnableDiscovery = true
					discoveryCtrl.Config.EnableAdvertisement = true

					_ = discoveryCtrl.LocalClusterID.SetupClusterID("default")

					// create the auth service
					_, err := cluster.GetClient().Client().CoreV1().Services("default").Create(context.TODO(), &v1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name: liqoconst.AuthServiceName,
						},
						Spec: v1.ServiceSpec{
							Type: v1.ServiceTypeNodePort,
							Ports: []v1.ServicePort{
								{
									Port:       1234,
									TargetPort: intstr.FromInt(1234),
									NodePort:   31234,
								},
							},
						},
					}, metav1.CreateOptions{})
					if err != nil {
						klog.Error(err)
						os.Exit(1)
					}
				})

				AfterEach(func() {
					discoveryCtrl.Config.EnableDiscovery = false
					discoveryCtrl.Config.EnableAdvertisement = false
				})

				It("register/resolve", func() {
					// register
					registerExit := make(chan bool, 1)
					go func() {
						discoveryCtrl.register()
						registerExit <- true
					}()

					// resolve
					ctx, cancel := context.WithCancel(context.TODO())
					defer cancel()

					resultChan := make(chan discoverableData)

					resolveExit := make(chan bool, 1)
					go func() {
						discoveryCtrl.resolve(ctx, discoveryCtrl.Config.AuthService, discoveryCtrl.Config.Domain, resultChan)
						resolveExit <- true
					}()

					var data discoverableData = nil
					select {
					case data = <-resultChan:
						break
					case <-time.NewTimer(time.Second * 5).C:
						break
					}

					Expect(data).NotTo(BeNil())
					Expect(len(registerExit)).To(Equal(0))
					Expect(len(resolveExit)).To(Equal(0))

					// shutdown
					discoveryCtrl.Config.EnableDiscovery = false
					discoveryCtrl.Config.EnableAdvertisement = false

					close(discoveryCtrl.stopMDNSClient)
					close(discoveryCtrl.stopMDNS)
					cancel()

					Eventually(<-registerExit, 5).Should(BeTrue())
					Eventually(<-resolveExit, 5).Should(BeTrue())
				})

			})

		})

	})

})
