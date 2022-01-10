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

package discovery

import (
	"context"
	"net"
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
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

func TestDiscovery(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Discovery Suite")
}

var _ = BeforeSuite(func() {
	utilruntime.Must(discoveryv1alpha1.AddToScheme(scheme.Scheme))
})

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
			ctx           context.Context
			discoveryCtrl Controller
		)

		BeforeEach(func() {
			ctx = context.Background()

			clusterIdentity := discoveryv1alpha1.ClusterIdentity{
				ClusterID:   "local-cluster-id",
				ClusterName: "local-cluster-name",
			}

			client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			discoveryCtrl = Controller{
				Client:           client,
				namespacedClient: client,
				LocalCluster:     clusterIdentity,
				namespace:        "default",
				mdnsConfig: MDNSConfig{
					Service:             "_liqo_auth._tcp",
					Domain:              "local.",
					EnableAdvertisement: false,
					EnableDiscovery:     false,
					TTL:                 90 * time.Second,
				},
			}
		})

		Describe("ForeignCluster management", func() {

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

						var fcs discoveryv1alpha1.ForeignClusterList
						Eventually(func() int {
							Expect(discoveryCtrl.List(ctx, &fcs)).To(Succeed())
							return len(fcs.Items)
						}).Should(c.expectedLength)

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
								ClusterID:   "local-cluster-id",
								ClusterName: "ClusterTest1",
							},
						},
						expectedLength:  Equal(0),
						expectedPeering: Equal(discoveryv1alpha1.PeeringEnabledAuto),
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
						expectedPeering: Equal(discoveryv1alpha1.PeeringEnabledAuto),
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
						expectedPeering: Equal(discoveryv1alpha1.PeeringEnabledAuto),
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

					var fcs discoveryv1alpha1.ForeignClusterList
					Expect(discoveryCtrl.List(ctx, &fcs))
					Expect(fcs.Items).To(HaveLen(1))

					fc := fcs.Items[0]

					updateTime = fc.GetAnnotations()[discovery.LastUpdateAnnotation]

					// I need to wait that at least a second is passed
					time.Sleep(time.Second * 1)
				})

				DescribeTable("UpdateForeign table",
					func(c updateForeignTestcase) {
						discoveryCtrl.updateForeignLAN(&c.data)

						var fcs discoveryv1alpha1.ForeignClusterList
						Expect(discoveryCtrl.List(ctx, &fcs))
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
						expectedPeering: Equal(discoveryv1alpha1.PeeringEnabledAuto),
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
						expectedPeering: Equal(discoveryv1alpha1.PeeringEnabledAuto),
						expectedSdLabel: BeEmpty(),
					}),
				)

			})

			Context("Update existing (Discovery Priority)", func() {

				BeforeEach(func() {
					fc := discoveryv1alpha1.ForeignCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foreign-cluster",
							Labels: map[string]string{
								discovery.DiscoveryTypeLabel: string(discovery.IncomingPeeringDiscovery),
								discovery.ClusterIDLabel:     "foreign-cluster",
							},
						},
						Spec: discoveryv1alpha1.ForeignClusterSpec{
							ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
								ClusterID:   "foreign-cluster",
								ClusterName: "ClusterTest2",
							},
							ForeignAuthURL:         "https://example.com",
							OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
							IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
							InsecureSkipTLSVerify:  pointer.BoolPtr(true),
						},
					}

					Expect(discoveryCtrl.Create(ctx, &fc)).To(Succeed())
				})

				DescribeTable("UpdateForeign table",
					func(c updateForeignTestcase) {
						discoveryCtrl.updateForeignLAN(&c.data)

						var fcs discoveryv1alpha1.ForeignClusterList
						Expect(discoveryCtrl.List(ctx, &fcs))
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
						expectedPeering: Equal(discoveryv1alpha1.PeeringEnabledAuto),
						expectedSdLabel: BeEmpty(),
					}),
				)

			})

			Context("GarbageCollector", func() {

				type garbageCollectorTestcase struct {
					fc             discoveryv1alpha1.ForeignCluster
					expectedLength types.GomegaMatcher
				}

				DescribeTable("GarbageCollector table",
					func(c garbageCollectorTestcase) {
						Expect(discoveryCtrl.Create(ctx, &c.fc)).To(Succeed())

						Expect(discoveryCtrl.collectGarbage(ctx)).To(Succeed())

						var fcs discoveryv1alpha1.ForeignClusterList
						Expect(discoveryCtrl.List(ctx, &fcs))
						Expect(len(fcs.Items)).To(c.expectedLength)
					},

					Entry("no garbage", garbageCollectorTestcase{
						fc: discoveryv1alpha1.ForeignCluster{
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
							Spec: discoveryv1alpha1.ForeignClusterSpec{
								ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
									ClusterID:   "foreign-cluster",
									ClusterName: "ClusterTest2",
								},
								OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
								IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
								ForeignAuthURL:         "https://example.com",
								InsecureSkipTLSVerify:  pointer.BoolPtr(true),
								TTL:                    300,
							},
						},

						expectedLength: Equal(1),
					}),

					Entry("garbage", garbageCollectorTestcase{
						fc: discoveryv1alpha1.ForeignCluster{
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
							Spec: discoveryv1alpha1.ForeignClusterSpec{
								ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
									ClusterID:   "foreign-cluster",
									ClusterName: "ClusterTest2",
								},
								OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
								IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
								ForeignAuthURL:         "https://example.com",
								InsecureSkipTLSVerify:  pointer.BoolPtr(true),
								TTL:                    300,
							},
						},

						expectedLength: Equal(0),
					}),

					Entry("no garbage (Manual Discovery)", garbageCollectorTestcase{
						fc: discoveryv1alpha1.ForeignCluster{
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
							Spec: discoveryv1alpha1.ForeignClusterSpec{
								ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
									ClusterID:   "foreign-cluster",
									ClusterName: "ClusterTest2",
								},
								OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
								IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
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
					discoveryCtrl.mdnsConfig.EnableDiscovery = true
					discoveryCtrl.mdnsConfig.EnableAdvertisement = true

					// create the auth service
					Expect(discoveryCtrl.Create(ctx, &v1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      liqoconst.AuthServiceName,
							Namespace: discoveryCtrl.namespace,
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
					})).To(Succeed())
				})

				AfterEach(func() {
					discoveryCtrl.mdnsConfig.EnableDiscovery = false
					discoveryCtrl.mdnsConfig.EnableAdvertisement = false
				})

				It("register/resolve", func() {
					// register
					registerExit := make(chan bool, 1)
					ctx1, cancel1 := context.WithCancel(ctx)
					defer cancel1()
					go func() {
						discoveryCtrl.register(ctx1)
						registerExit <- true
					}()

					// resolve
					ctx2, cancel2 := context.WithCancel(ctx)
					defer cancel2()

					resultChan := make(chan discoverableData)

					resolveExit := make(chan bool, 1)
					go func() {
						discoveryCtrl.resolve(ctx2, discoveryCtrl.mdnsConfig.Service, discoveryCtrl.mdnsConfig.Domain, resultChan)
						resolveExit <- true
					}()

					var data discoverableData = nil
					select {
					case data = <-resultChan:
						break
					case <-time.After(time.Second * 5):
						break
					}

					Expect(data).NotTo(BeNil())
					Expect(len(registerExit)).To(Equal(0))
					Expect(len(resolveExit)).To(Equal(0))

					cancel1()
					cancel2()

					Eventually(<-registerExit, 5).Should(BeTrue())
					Eventually(<-resolveExit, 5).Should(BeTrue())
				})

			})

		})

	})

})
