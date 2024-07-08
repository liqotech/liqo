// Copyright 2019-2024 The Liqo Authors
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

package ipam

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/nftables"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	ipamerrors "github.com/liqotech/liqo/pkg/ipam/errors"
	ipamutils "github.com/liqotech/liqo/pkg/ipam/utils"
)

const (
	clusterID1           = "cluster1"
	clusterID2           = "cluster2"
	clusterID3           = "cluster3"
	remotePodCIDR        = "10.50.0.0/16"
	remoteExternalCIDR   = "10.60.0.0/16"
	homePodCIDR          = "10.0.0.0/24"
	homeExternalCIDR     = "10.1.0.0/24"
	localEndpointIP      = "10.0.0.20"
	localNATPodCIDR      = "10.0.1.0/24"
	localNATExternalCIDR = "192.168.30.0/24"
	externalEndpointIP   = "10.0.50.6"
	endpointIP           = "20.0.0.1"
	invalidValue         = "invalid value"
	namespace            = "test-namespace"
)

var (
	ipam      *IPAM
	dynClient *fake.FakeDynamicClient

	ctx = context.Background()
)

func fillNetworkPool(pool string, ipam *IPAM) error {

	// Get halves mask length
	mask := ipamutils.GetMask(pool)
	mask++

	// Get first half CIDR
	halfCidr := ipamutils.SetMask(pool, mask)

	err := ipam.AcquireReservedSubnet(halfCidr)
	if err != nil {
		return err
	}

	// Get second half CIDR
	halfCidr = ipamutils.Next(halfCidr)
	err = ipam.AcquireReservedSubnet(halfCidr)

	return err
}

func setDynClient() error {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "ipam.liqo.io",
		Version: "v1alpha1",
		Kind:    "ipamstorages",
	}, &ipamv1alpha1.IpamStorage{})

	var m = make(map[schema.GroupVersionResource]string)

	m[schema.GroupVersionResource{
		Group:    "ipam.liqo.io",
		Version:  "v1alpha1",
		Resource: "ipamstorages",
	}] = "ipamstoragesList"

	dynClient = fake.NewSimpleDynamicClientWithCustomListKinds(scheme, m)
	return nil
}

var _ = Describe("Ipam", func() {
	BeforeEach(func() {
		ipam = NewIPAM()
		err := setDynClient()
		Expect(err).To(BeNil())
		n, err := rand.Int(rand.Reader, big.NewInt(10000))
		Expect(err).To(BeNil())
		err = ipam.Init(Pools, dynClient, namespace)
		Expect(err).To(BeNil())
		err = ipam.Serve(2000 + int(n.Int64()))
		Expect(err).To(BeNil())
	})
	AfterEach(func() {
		ipam.Terminate()
	})

	Describe("AcquireReservedSubnet", func() {
		Context("When the reserved network equals a network pool", func() {
			It("Should successfully reserve the subnet", func() {
				// Reserve network
				err := ipam.AcquireReservedSubnet("10.0.0.0/8")
				Expect(err).To(BeNil())
				res, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: "10.0.2.0/24",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Cidr).ToNot(HavePrefix("10."))
			})
		})
		Context("When the reserved network belongs to a pool", func() {
			It("Should not be possible to acquire the same network for a cluster", func() {
				err := ipam.AcquireReservedSubnet("10.244.0.0/24")
				Expect(err).To(BeNil())
				res, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: "10.244.0.0/24",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Cidr).ToNot(Equal("10.244.0.0/24"))
			})
			It("Should not be possible to acquire a larger network that contains it for a cluster", func() {
				err := ipam.AcquireReservedSubnet("10.0.0.0/24")
				Expect(err).To(BeNil())
				res, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: "10.0.0.0/16",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Cidr).ToNot(Equal("10.0.0.0/16"))
			})
			It("Should not be possible to acquire a smaller network contained by it for a cluster", func() {
				err := ipam.AcquireReservedSubnet("10.0.2.0/24")
				Expect(err).To(BeNil())
				res, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: "10.0.2.0/25",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Cidr).ToNot(Equal("10.0.2.0/25"))
			})
		})
	})

	Describe("FreeReservedSubnet", func() {
		Context("Freeing a network that has been reserved previously", func() {
			It("Should successfully free the subnet", func() {
				err := ipam.AcquireReservedSubnet("10.0.1.0/24")
				Expect(err).To(BeNil())
				err = ipam.FreeReservedSubnet("10.0.1.0/24")
				Expect(err).To(BeNil())
				err = ipam.AcquireReservedSubnet("10.0.1.0/24")
				Expect(err).To(BeNil())
			})
		})
		Context("Freeing a network that does not exists", func() {
			It("Should return no errors", func() {
				err := ipam.FreeReservedSubnet("10.0.1.0/24")
				Expect(err).To(BeNil())
			})
		})
		Context("Freeing a reserved subnet equal to a network pool", func() {
			It("Should make available the network pool", func() {
				err := ipam.AcquireReservedSubnet("10.0.0.0/8")
				Expect(err).To(BeNil())
				err = ipam.FreeReservedSubnet("10.0.0.0/8")
				Expect(err).To(BeNil())
				res, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: "10.0.0.0/16",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Cidr).To(Equal("10.0.0.0/16"))
			})
		})
	})
	Describe("Restating manager manager idempotency", func() {
		It("ipam should retrieve configuration by resource", func() {
			// Assign networks
			res, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
				Cidr: "10.0.1.0/24",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Cidr).To(Equal("10.0.1.0/24"))

			// Simulate re-scheduling
			ipam.Terminate()
			ipam = NewIPAM()
			n, err := rand.Int(rand.Reader, big.NewInt(2000))
			Expect(err).To(BeNil())
			err = ipam.Init(Pools, dynClient, namespace)
			Expect(err).To(BeNil())
			err = ipam.Serve(2000 + int(n.Int64()))
			Expect(err).To(BeNil())

			// Ask for the same network again
			res, err = ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
				Cidr: "10.0.1.0/24",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Cidr).ToNot(Equal("10.0.1.0/24"))
		})
	})
	Describe("AddNetworkPool", func() {
		Context("Trying to add a default network pool", func() {
			It("Should generate an error", func() {
				err := ipam.AddNetworkPool("10.0.0.0/8")
				Expect(err).ToNot(BeNil())
			})
		})
		Context("Trying to add twice the same network pool", func() {
			It("Should generate an error", func() {
				err := ipam.AddNetworkPool("11.0.0.0/8")
				Expect(err).To(BeNil())
				err = ipam.AddNetworkPool("11.0.0.0/8")
				Expect(err).ToNot(BeNil())
			})
		})
		Context("After adding a new network pool", func() {
			It("Should be possible to use that pool for cluster networks", func() {
				// Reserve default network pools
				for _, network := range Pools {
					err := fillNetworkPool(network, ipam)
					Expect(err).To(BeNil())
				}

				// Add new network pool
				err := ipam.AddNetworkPool("11.0.0.0/8")
				Expect(err).To(BeNil())

				// Reserve a given network
				err = ipam.AcquireReservedSubnet("12.0.0.0/24")
				Expect(err).To(BeNil())

				// Reserve a given network
				err = ipam.AcquireReservedSubnet("12.0.1.0/24")
				Expect(err).To(BeNil())

				res, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: "12.0.0.0/24",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Cidr).To(HavePrefix("11"))
				Expect(res.Cidr).To(HaveSuffix("/24"))
			})
		})
		Context("Trying to add a network pool that overlaps with a reserved network", func() {
			It("Should generate an error", func() {
				err := ipam.AcquireReservedSubnet("11.0.0.0/8")
				Expect(err).To(BeNil())
				err = ipam.AddNetworkPool("11.0.0.0/16")
				Expect(err).ToNot(BeNil())
			})
		})
	})
	Describe("RemoveNetworkPool", func() {
		Context("Remove a network pool that does not exist", func() {
			It("Should return an error", func() {
				err := ipam.RemoveNetworkPool("11.0.0.0/8")
				Expect(err).ToNot(BeNil())
			})
		})
		Context("Remove a network pool that exists", func() {
			It("Should successfully remove the network pool", func() {
				// Reserve default network pools
				for _, network := range Pools {
					err := ipam.AcquireReservedSubnet(network)
					Expect(err).To(BeNil())
				}

				// Add new network pool
				err := ipam.AddNetworkPool("11.0.0.0/8")
				Expect(err).To(BeNil())

				// Remove network pool
				err = ipam.RemoveNetworkPool("11.0.0.0/8")
				Expect(err).To(BeNil())

				// Reserve a given network
				err = ipam.AcquireReservedSubnet("12.0.0.0/24")
				Expect(err).To(BeNil())

				// Reserve a given network
				err = ipam.AcquireReservedSubnet("12.0.1.0/24")
				Expect(err).To(BeNil())

				_, err = ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: "12.0.0.0/24",
				})
				Expect(err).To(HaveOccurred())
			})
		})
		Context("Remove a network pool that is a default one", func() {
			It("Should generate an error", func() {
				err := ipam.RemoveNetworkPool(Pools[0])
				Expect(err).ToNot(BeNil())
			})
		})
	})

	Describe("GetExternalCIDR", func() {
		Context("Invoking it twice", func() {
			It("should return no errors", func() {
				e, err := ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())
				Expect(e).To(HaveSuffix("/24"))
				_, err = ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())
			})
		})
		Context("Using a valid mask length", func() {
			It("should return no errors", func() {
				e, err := ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())
				Expect(e).To(HaveSuffix("/24"))
			})
		})
		Context("Using an invalid mask length", func() {
			It("should return an error", func() {
				_, err := ipam.GetExternalCIDR(33)
				Expect(err).ToNot(BeNil())
			})
		})
		Context("Call after SetPodCIDR", func() {
			It("should return no errors", func() {
				err := ipam.SetPodCIDR(homePodCIDR)
				Expect(err).To(BeNil())
				externalCIDR, err := ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())
				Expect(externalCIDR).To(Equal("10.0.1.0/24"))
			})
		})
		Context("Call before SetPodCIDR", func() {
			It("should produce an error in SetPodCIDR", func() {
				externalCIDR, err := ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())
				Expect(externalCIDR).To(Equal(homePodCIDR))
				// ExternalCIDR has been assigned "10.0.0.0/24", so the network
				// is not available anymore.
				err = ipam.SetPodCIDR(homePodCIDR)
				Expect(err).ToNot(BeNil())
			})
		})
	})

	Describe("SetPodCIDR", func() {
		Context("Invoking func for the first time", func() {
			It("should return no errors", func() {
				err := ipam.SetPodCIDR(homePodCIDR)
				Expect(err).To(BeNil())
			})
		})
		Context("Later invocation with the same PodCIDR", func() {
			It("should return no errors", func() {
				err := ipam.SetPodCIDR(homePodCIDR)
				Expect(err).To(BeNil())
				err = ipam.SetPodCIDR(homePodCIDR)
				Expect(err).To(BeNil())
			})
		})
		Context("Later invocation with a different PodCIDR", func() {
			It("should return no errors", func() {
				err := ipam.SetPodCIDR(homePodCIDR)
				Expect(err).To(BeNil())
				err = ipam.SetPodCIDR("18.0.0.0/24")
				Expect(err).ToNot(BeNil())
			})
		})
		Context("Using a reserved network", func() {
			It("should return an error", func() {
				err := ipam.AcquireReservedSubnet("10.0.1.0/24")
				Expect(err).To(BeNil())
				err = ipam.SetPodCIDR("10.0.1.0/24")
				Expect(err).ToNot(BeNil())
			})
		})
	})
	Describe("SetServiceCIDR", func() {
		Context("Invoking func for the first time", func() {
			It("should return no errors", func() {
				err := ipam.SetServiceCIDR("10.0.0.0/24")
				Expect(err).To(BeNil())
			})
		})
		Context("Later invocation with the same ServiceCIDR", func() {
			It("should return no errors", func() {
				err := ipam.SetServiceCIDR("10.0.0.0/24")
				Expect(err).To(BeNil())
				err = ipam.SetServiceCIDR("10.0.0.0/24")
				Expect(err).To(BeNil())
			})
		})
		Context("Later invocation with a different ServiceCIDR", func() {
			It("should return no errors", func() {
				err := ipam.SetServiceCIDR("10.0.0.0/24")
				Expect(err).To(BeNil())
				err = ipam.SetServiceCIDR("10.0.1.0/24")
				Expect(err).ToNot(BeNil())
			})
		})
		Context("Using a reserved network", func() {
			It("should return an error", func() {
				err := ipam.AcquireReservedSubnet("10.0.1.0/24")
				Expect(err).To(BeNil())
				err = ipam.SetServiceCIDR("10.0.1.0/24")
				Expect(err).ToNot(BeNil())
			})
		})
	})
	Describe("MapEndpointIP", func() {
		Context("If the endpoint IP does not belong to local PodCIDR", func() {
			It("should map the endpoint IP to a new IP belonging to local ExternalCIDR", func() {
				// Set PodCIDR
				err := ipam.SetPodCIDR(homePodCIDR)
				Expect(err).To(BeNil())

				// Get ExternalCIDR
				externalCIDRResp, err := ipam.GetOrSetExternalCIDR(ctx, &GetOrSetExtCIDRRequest{
					DesiredExtCIDR: homeExternalCIDR,
				})
				Expect(err).To(BeNil())
				externalCIDR := externalCIDRResp.GetRemappedExtCIDR()
				Expect(externalCIDR).To(HaveSuffix("/24"))

				// Assign networks to cluster
				res, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: remotePodCIDR,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Cidr).To(HaveSuffix("/16"))

				res2, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: remoteExternalCIDR,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res2.Cidr).To(HaveSuffix("/16"))

				subRes, err := ipam.SetSubnetsPerCluster(ctx, &SetSubnetsPerClusterRequest{
					RemappedPodCIDR:      res.Cidr,
					RemappedExternalCIDR: res2.Cidr,
					ClusterID:            clusterID1,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(subRes).ToNot(BeNil())

				response, err := ipam.MapEndpointIP(ctx, &MapRequest{
					ClusterID: clusterID1,
					Ip:        endpointIP,
				})
				Expect(err).To(BeNil())
				slicedPrefix := strings.SplitN(externalCIDR, ".", 4)
				slicedPrefix = slicedPrefix[:len(slicedPrefix)-1]
				Expect(response.GetIp()).To(HavePrefix(strings.Join(slicedPrefix, ".")))
			})
			It("should return the same IP if more remote clusters ask for the same endpoint", func() {
				// Set PodCIDR
				err := ipam.SetPodCIDR(homePodCIDR)
				Expect(err).To(BeNil())

				// Get ExternalCIDR
				externalCIDRResp, err := ipam.GetOrSetExternalCIDR(ctx, &GetOrSetExtCIDRRequest{
					DesiredExtCIDR: homeExternalCIDR,
				})
				Expect(err).To(BeNil())
				externalCIDR := externalCIDRResp.GetRemappedExtCIDR()
				Expect(externalCIDR).To(HaveSuffix("/24"))

				// Assign networks to cluster
				res, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: remotePodCIDR,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Cidr).To(HaveSuffix("/16"))

				res2, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: remoteExternalCIDR,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res2.Cidr).To(HaveSuffix("/16"))

				subRes, err := ipam.SetSubnetsPerCluster(ctx, &SetSubnetsPerClusterRequest{
					RemappedPodCIDR:      res.Cidr,
					RemappedExternalCIDR: res2.Cidr,
					ClusterID:            clusterID1,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(subRes).ToNot(BeNil())

				resB, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: remotePodCIDR,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(resB.Cidr).To(HaveSuffix("/16"))

				res2B, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: remoteExternalCIDR,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res2B.Cidr).To(HaveSuffix("/16"))

				subResB, err := ipam.SetSubnetsPerCluster(ctx, &SetSubnetsPerClusterRequest{
					RemappedPodCIDR:      resB.Cidr,
					RemappedExternalCIDR: res2B.Cidr,
					ClusterID:            clusterID2,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(subResB).ToNot(BeNil())

				response, err := ipam.MapEndpointIP(ctx, &MapRequest{
					ClusterID: clusterID1,
					Ip:        endpointIP,
				})
				Expect(err).To(BeNil())
				slicedPrefix := strings.SplitN(externalCIDR, ".", 4)
				slicedPrefix = slicedPrefix[:len(slicedPrefix)-1]
				Expect(response.GetIp()).To(HavePrefix(strings.Join(slicedPrefix, ".")))

				responseB, err := ipam.MapEndpointIP(ctx, &MapRequest{
					ClusterID: "cluster2",
					Ip:        endpointIP,
				})
				Expect(err).To(BeNil())
				slicedPrefix = strings.SplitN(externalCIDR, ".", 4)
				slicedPrefix = slicedPrefix[:len(slicedPrefix)-1]
				Expect(responseB.GetIp()).To(HavePrefix(strings.Join(slicedPrefix, ".")))

				Expect(response.GetIp()).To(Equal(responseB.GetIp()))
			})
			Context("and the ExternalCIDR has not any more available IPs", func() {
				It("should return an error", func() {
					var response *MapResponse
					var err error
					// Set PodCIDR
					err = ipam.SetPodCIDR(homePodCIDR)
					Expect(err).To(BeNil())

					// Get ExternalCIDR
					externalCIDR, err := ipam.GetExternalCIDR(24)
					Expect(err).To(BeNil())
					Expect(externalCIDR).To(HaveSuffix("/24"))
					slicedPrefix := strings.SplitN(externalCIDR, ".", 4)
					slicedPrefix = slicedPrefix[:len(slicedPrefix)-1]

					// Assign networks to cluster
					res, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
						Cidr: remotePodCIDR,
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(res.Cidr).To(HaveSuffix("/16"))

					res2, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
						Cidr: remoteExternalCIDR,
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(res2.Cidr).To(HaveSuffix("/16"))

					subRes, err := ipam.SetSubnetsPerCluster(ctx, &SetSubnetsPerClusterRequest{
						RemappedPodCIDR:      res.Cidr,
						RemappedExternalCIDR: res2.Cidr,
						ClusterID:            clusterID1,
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(subRes).ToNot(BeNil())

					// Fill up ExternalCIDR
					for i := 0; i < 254; i++ {
						response, err = ipam.MapEndpointIP(ctx, &MapRequest{
							ClusterID: clusterID1,
							Ip:        fmt.Sprintf("20.0.0.%d", i),
						})
						Expect(err).To(BeNil())
						Expect(response.GetIp()).To(HavePrefix(strings.Join(slicedPrefix, ".")))
					}

					_, err = ipam.MapEndpointIP(ctx, &MapRequest{
						ClusterID: clusterID1,
						Ip:        "3.100.0.9",
					})
					Expect(err).ToNot(BeNil())
				})
			})
			Context("Passing invalid parameters", func() {
				It("Empty clusterID", func() {
					_, err := ipam.MapEndpointIP(ctx, &MapRequest{
						ClusterID: "",
						Ip:        localEndpointIP,
					})
					Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("%s must be %s", consts.ClusterIDLabelName, ipamerrors.StringNotEmpty)))
				})
				It("Non-existing clusterID", func() {
					_, err := ipam.MapEndpointIP(ctx, &MapRequest{
						ClusterID: clusterID3,
						Ip:        localEndpointIP,
					})
					Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("%s has not a network configuration", clusterID3)))
				})
				It("Invalid IP", func() {
					_, err := ipam.MapEndpointIP(ctx, &MapRequest{
						ClusterID: clusterID3,
						Ip:        "10.9.9",
					})
					Expect(err.Error()).To(ContainSubstring("Endpoint IP must be a valid IP"))
				})
			})
			Context("If the local PodCIDR is not set", func() {
				It("should return an error", func() {
					// Get ExternalCIDR
					externalCIDR, err := ipam.GetExternalCIDR(24)
					Expect(err).To(BeNil())
					Expect(externalCIDR).To(HaveSuffix("/24"))

					// Assign networks to cluster
					res, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
						Cidr: remotePodCIDR,
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(res.Cidr).To(HaveSuffix("/16"))

					res2, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
						Cidr: remoteExternalCIDR,
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(res2.Cidr).To(HaveSuffix("/16"))

					subRes, err := ipam.SetSubnetsPerCluster(ctx, &SetSubnetsPerClusterRequest{
						RemappedPodCIDR:      res.Cidr,
						RemappedExternalCIDR: res2.Cidr,
						ClusterID:            clusterID1,
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(subRes).ToNot(BeNil())

					_, err = ipam.MapEndpointIP(ctx, &MapRequest{
						ClusterID: clusterID1,
						Ip:        "30.0.4.9",
					})
					Expect(err.Error()).To(ContainSubstring("cannot get cluster PodCIDR"))
				})
			})
			Context("If the remote cluster has not a network configuration", func() {
				It("should return an error", func() {
					// Set PodCIDR
					err := ipam.SetPodCIDR(homePodCIDR)
					Expect(err).To(BeNil())

					_, err = ipam.MapEndpointIP(ctx, &MapRequest{
						ClusterID: clusterID1,
						Ip:        "10.0.0.9",
					})
					Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("cluster %s has not a network configuration", clusterID1)))
				})
			})
		})
	})

	Describe("GetHomePodIP", func() {
		Context("Pass function an invalid IP address", func() {
			It("should return WrongParameter error", func() {
				_, err := ipam.GetHomePodIP(ctx,
					&GetHomePodIPRequest{
						Ip:        invalidValue,
						ClusterID: clusterID1,
					})
				err = errors.Unwrap(err)
				Expect(err).To(MatchError(fmt.Sprintf("%s must be %s", invalidValue, ipamerrors.ValidIP)))
			})
		})
		Context("Pass function an empty cluster ID", func() {
			It("should return WrongParameter error", func() {
				_, err := ipam.GetHomePodIP(ctx,
					&GetHomePodIPRequest{
						Ip:        invalidValue,
						ClusterID: "",
					})
				err = errors.Unwrap(err)
				Expect(err).To(MatchError(fmt.Sprintf("%s must be %s", consts.ClusterIDLabelName, ipamerrors.StringNotEmpty)))
			})
		})
		Context("Invoking func without subnets init", func() {
			It("should return WrongParameter error", func() {
				_, err := ipam.GetHomePodIP(ctx,
					&GetHomePodIPRequest{
						Ip:        "10.0.0.1",
						ClusterID: clusterID1,
					})
				err = errors.Unwrap(err)
				Expect(err).To(MatchError(fmt.Sprintf("cluster %s subnets are not set", clusterID1)))
			})
		})
		Context(`When the remote Pod CIDR has not been remapped by home cluster
			and the call refers to a remote Pod`, func() {
			It("should return the same IP", func() {
				ip, _, err := nftables.NetFirstAndLastIP(remotePodCIDR)
				Expect(err).To(BeNil())

				// Assign networks to cluster
				res, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: remotePodCIDR,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Cidr).To(HaveSuffix("/16"))

				res2, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: remoteExternalCIDR,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res2.Cidr).To(HaveSuffix("/16"))

				subRes, err := ipam.SetSubnetsPerCluster(ctx, &SetSubnetsPerClusterRequest{
					RemappedPodCIDR:      res.Cidr,
					RemappedExternalCIDR: res2.Cidr,
					ClusterID:            clusterID1,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(subRes).ToNot(BeNil())

				// Home cluster has not remapped remote PodCIDR
				Expect(res.Cidr).To(Equal(remotePodCIDR))

				response, err := ipam.GetHomePodIP(ctx,
					&GetHomePodIPRequest{
						Ip:        ip.String(),
						ClusterID: clusterID1,
					})
				Expect(err).To(BeNil())
				Expect(response.GetHomeIP()).To(Equal(ip.String()))
			})
		})
		Context(`When the remote Pod CIDR has been remapped by home cluster
			and the call refers to a remote Pod`, func() {
			It("should return the remapped IP", func() {
				// Original Pod IP
				ip, _, err := nftables.NetFirstAndLastIP(remotePodCIDR)
				Expect(err).To(BeNil())

				// Reserve original PodCIDR so that home cluster will remap it
				err = ipam.AcquireReservedSubnet(remotePodCIDR)
				Expect(err).To(BeNil())

				// Assign networks to cluster
				res, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: remotePodCIDR,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Cidr).To(HaveSuffix("/16"))

				res2, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: remoteExternalCIDR,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res2.Cidr).To(HaveSuffix("/16"))

				subRes, err := ipam.SetSubnetsPerCluster(ctx, &SetSubnetsPerClusterRequest{
					RemappedPodCIDR:      res.Cidr,
					RemappedExternalCIDR: res2.Cidr,
					ClusterID:            clusterID1,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(subRes).ToNot(BeNil())

				// Home cluster has remapped remote PodCIDR
				Expect(res.Cidr).ToNot(Equal(remotePodCIDR))

				response, err := ipam.GetHomePodIP(ctx,
					&GetHomePodIPRequest{
						Ip:        ip.String(),
						ClusterID: clusterID1,
					})
				Expect(err).To(BeNil())

				// IP should be mapped to remoteNATPodCIDR
				remappedIP, err := ipamutils.MapIPToNetwork(res.Cidr, ip.String())
				Expect(err).To(BeNil())
				Expect(response.GetHomeIP()).To(Equal(remappedIP))
			})
		})
	})

	Describe("UnmapEndpointIP", func() {
		Context("Passing invalid parameters", func() {
			It("Empty clusterID", func() {
				_, err := ipam.MapEndpointIP(ctx, &MapRequest{
					ClusterID: "",
					Ip:        localEndpointIP,
				})
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("%s must be %s", consts.ClusterIDLabelName, ipamerrors.StringNotEmpty)))
			})
			It("Non-existing clusterID", func() {
				_, err := ipam.MapEndpointIP(ctx, &MapRequest{
					ClusterID: clusterID3,
					Ip:        localEndpointIP,
				})
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("%s has not a network configuration", clusterID3)))
			})
			It("Invalid IP", func() {
				_, err := ipam.MapEndpointIP(ctx, &MapRequest{
					ClusterID: clusterID3,
					Ip:        "10.9.9",
				})
				Expect(err.Error()).To(ContainSubstring("Endpoint IP must be a valid IP"))
			})
		})
		Context("If there are no more clusters using an endpointIP", func() {
			It("should free the relative IP", func() {
				endpointIP := endpointIP
				// Set PodCIDR
				err := ipam.SetPodCIDR(homePodCIDR)
				Expect(err).To(BeNil())

				// Get ExternalCIDR
				externalCIDR, err := ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())
				Expect(externalCIDR).To(HaveSuffix("/24"))
				slicedPrefix := strings.SplitN(externalCIDR, ".", 4)
				slicedPrefix = slicedPrefix[:len(slicedPrefix)-1]

				// Assign networks to cluster
				res, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: remotePodCIDR,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Cidr).To(HaveSuffix("/16"))

				res2, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: remoteExternalCIDR,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res2.Cidr).To(HaveSuffix("/16"))

				subRes, err := ipam.SetSubnetsPerCluster(ctx, &SetSubnetsPerClusterRequest{
					RemappedPodCIDR:      res.Cidr,
					RemappedExternalCIDR: res2.Cidr,
					ClusterID:            clusterID1,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(subRes).ToNot(BeNil())

				resB, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: remotePodCIDR,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(resB.Cidr).To(HaveSuffix("/16"))

				res2B, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: remoteExternalCIDR,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res2B.Cidr).To(HaveSuffix("/16"))

				subResB, err := ipam.SetSubnetsPerCluster(ctx, &SetSubnetsPerClusterRequest{
					RemappedPodCIDR:      resB.Cidr,
					RemappedExternalCIDR: res2B.Cidr,
					ClusterID:            clusterID2,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(subResB).ToNot(BeNil())

				// Reflection in cluster1
				response, err := ipam.MapEndpointIP(ctx, &MapRequest{
					ClusterID: clusterID1,
					Ip:        endpointIP,
				})
				Expect(err).To(BeNil())
				Expect(response.GetIp()).To(HavePrefix(strings.Join(slicedPrefix, ".")))

				// Reflection in cluster2
				_, err = ipam.MapEndpointIP(ctx, &MapRequest{
					ClusterID: clusterID2,
					Ip:        endpointIP,
				})
				Expect(err).To(BeNil())

				// Terminate reflection in cluster1
				_, err = ipam.UnmapEndpointIP(ctx, &UnmapRequest{
					ClusterID: clusterID1,
					Ip:        endpointIP,
				})
				Expect(err).To(BeNil())

				// Terminate reflection in cluster2
				_, err = ipam.UnmapEndpointIP(ctx, &UnmapRequest{
					ClusterID: clusterID2,
					Ip:        endpointIP,
				})
				Expect(err).To(BeNil())

				// Get Ipam configuration
				ipamConfig, err := getIpamStorageResource()
				Expect(err).To(BeNil())

				// Check if IP is freed
				Expect(ipamConfig.Spec.EndpointMappings).To(HaveLen(0))
			})
		})
		Context("If there are other clusters using an endpointIP", func() {
			It("should not free the relative IP", func() {
				// Set PodCIDR
				err := ipam.SetPodCIDR(homePodCIDR)
				Expect(err).To(BeNil())

				// Get ExternalCIDR
				externalCIDR, err := ipam.GetExternalCIDR(24)
				Expect(err).To(BeNil())
				Expect(externalCIDR).To(HaveSuffix("/24"))
				slicedPrefix := strings.SplitN(externalCIDR, ".", 4)
				slicedPrefix = slicedPrefix[:len(slicedPrefix)-1]

				// Assign networks to cluster
				res, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: remotePodCIDR,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res.Cidr).To(HaveSuffix("/16"))

				res2, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: remoteExternalCIDR,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res2.Cidr).To(HaveSuffix("/16"))

				subRes, err := ipam.SetSubnetsPerCluster(ctx, &SetSubnetsPerClusterRequest{
					RemappedPodCIDR:      res.Cidr,
					RemappedExternalCIDR: res2.Cidr,
					ClusterID:            clusterID1,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(subRes).ToNot(BeNil())

				resB, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: remotePodCIDR,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(resB.Cidr).To(HaveSuffix("/16"))

				res2B, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
					Cidr: remoteExternalCIDR,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res2B.Cidr).To(HaveSuffix("/16"))

				subResB, err := ipam.SetSubnetsPerCluster(ctx, &SetSubnetsPerClusterRequest{
					RemappedPodCIDR:      resB.Cidr,
					RemappedExternalCIDR: res2B.Cidr,
					ClusterID:            clusterID2,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(subResB).ToNot(BeNil())

				// Reflection in cluster1
				response, err := ipam.MapEndpointIP(ctx, &MapRequest{
					ClusterID: clusterID1,
					Ip:        endpointIP,
				})
				Expect(err).To(BeNil())
				Expect(response.GetIp()).To(HavePrefix(strings.Join(slicedPrefix, ".")))
				ip := response.GetIp()

				// Reflection in cluster2
				_, err = ipam.MapEndpointIP(ctx, &MapRequest{
					ClusterID: clusterID2,
					Ip:        endpointIP,
				})
				Expect(err).To(BeNil())

				// Terminate reflection in cluster2
				_, err = ipam.UnmapEndpointIP(ctx, &UnmapRequest{
					ClusterID: clusterID2,
					Ip:        endpointIP,
				})
				Expect(err).To(BeNil())

				// Get Ipam configuration
				ipamConfig, err := getIpamStorageResource()
				Expect(err).To(BeNil())

				// Check if IP is not freed
				Expect(ipamConfig.Spec.EndpointMappings).To(HaveLen(1))
				Expect(ipamConfig.Spec.EndpointMappings[endpointIP].ExternalCIDROriginalIP).To(Equal(ip))
			})
		})
	})

	Describe("SetReservedSubnets", func() {
		var (
			toBeReservedSubnets1          []string
			toBeReservedSubnets2          []string
			toBeReservedSubnetsIncorrect1 []string
			toBeReservedOverlapping1      []string
			toBeReservedOverlapping2      []string
			toBeReservedOverlapping3      []string
			toBeReservedOverlapping4      []string
			toBeReservedOverlapping5      []string
			toBeReservedOverlapping6      []string
		)

		BeforeEach(func() {
			var (
				serviceCidr  = "10.210.0.0/16"
				podCidr      = "10.220.0.0/16"
				externalCidr string
			)
			Expect(ipam.SetPodCIDR(podCidr)).To(Succeed())
			Expect(ipam.SetServiceCIDR(serviceCidr)).To(Succeed())
			externalCidr, err := ipam.GetExternalCIDR(24)
			Expect(err).To(BeNil())
			Expect(externalCidr).To(HaveSuffix("/24"))

			// Assign networks to cluster
			res, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
				Cidr: "10.0.0.0/16",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(res.Cidr).To(HaveSuffix("/16"))

			res2, err := ipam.MapNetworkCIDR(ctx, &MapCIDRRequest{
				Cidr: "10.1.0.0/16",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(res2.Cidr).To(HaveSuffix("/16"))

			subRes, err := ipam.SetSubnetsPerCluster(ctx, &SetSubnetsPerClusterRequest{
				RemappedPodCIDR:      res.Cidr,
				RemappedExternalCIDR: res2.Cidr,
				ClusterID:            clusterID1,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(subRes).ToNot(BeNil())

			toBeReservedSubnets1 = []string{"192.168.0.0/16", "100.200.0.0/16", "10.200.250.0/24"}
			toBeReservedSubnets2 = []string{"192.168.1.0/24", "100.200.0.0/16", "172.16.34.0/24"}
			toBeReservedSubnetsIncorrect1 = []string{"192.168.0.0/16", "100.200.0/16", "10.200.250.0/24"}
			toBeReservedOverlapping1 = []string{"192.168.1.0/24", "192.168.0.0/16", "172.16.34.0/24"}
			toBeReservedOverlapping2 = []string{"192.168.1.0/24", podCidr, "172.16.34.0/24"}
			toBeReservedOverlapping3 = []string{"192.168.1.0/24", serviceCidr, "172.16.34.0/24"}
			toBeReservedOverlapping4 = []string{"192.168.1.0/24", externalCidr, "172.16.34.0/24"}
			toBeReservedOverlapping5 = []string{res.Cidr, "100.200.0/16", "172.16.34.0/24"}
			toBeReservedOverlapping6 = []string{res2.Cidr, "100.200.0/16", "172.16.34.0/24"}
		})

		Context("Reserving subnets", func() {
			When("Reserving subnets for the first time", func() {
				It("should reserve all the the subnets and return nil", func() {
					Expect(ipam.SetReservedSubnets(toBeReservedSubnets1)).To(Succeed())
					Expect(ipam.ipamStorage.getReservedSubnets()).To(ContainElements(toBeReservedSubnets1))
					checkForPrefixes(toBeReservedSubnets1)
				})
			})

			When("Reserving subnets multiple times", func() {
				It("should return nil", func() {
					// Reserving the first time.
					Expect(ipam.SetReservedSubnets(toBeReservedSubnets1)).To(Succeed())
					Expect(ipam.ipamStorage.getReservedSubnets()).To(ContainElements(toBeReservedSubnets1))
					// Reserving the second time.
					Expect(ipam.SetReservedSubnets(toBeReservedSubnets1)).To(Succeed())
					Expect(ipam.ipamStorage.getReservedSubnets()).To(ContainElements(toBeReservedSubnets1))
					checkForPrefixes(toBeReservedSubnets1)
				})
			})

			When("Reserving a list of subnets with incorrect ones", func() {
				It("should reserve all the correct ones that comes before the incorrect one", func() {
					Expect(ipam.SetReservedSubnets(toBeReservedSubnetsIncorrect1)).To(HaveOccurred())
					Expect(ipam.ipamStorage.getReservedSubnets()).To(HaveLen(1))
					Expect(ipam.ipamStorage.getReservedSubnets()).To(ContainElement(toBeReservedSubnetsIncorrect1[0]))
				})
			})

			When("A subnet has been added to the reserved list but non effectively acquired", func() {
				It("should acquire the reserved subnet", func() {
					Expect(ipam.SetReservedSubnets(toBeReservedSubnets1)).To(Succeed())
					Expect(ipam.ipamStorage.getReservedSubnets()).To(ContainElements(toBeReservedSubnets1))
					// Remove the prefix from.
					_, err := ipam.ipamStorage.DeletePrefix(context.TODO(), *ipam.ipam.PrefixFrom(context.TODO(), toBeReservedSubnets1[1]))
					Expect(err).To(BeNil())
					Expect(ipam.SetReservedSubnets(toBeReservedSubnets1)).To(Succeed())
					Expect(ipam.ipam.PrefixFrom(context.TODO(), toBeReservedSubnets1[1]).Cidr).To(Equal(toBeReservedSubnets1[1]))
				})
			})

			When("Subnets to be reserved overlaps with each other", func() {
				It("should fail while reserving the subnet that overlaps", func() {
					Expect(ipam.SetReservedSubnets(toBeReservedOverlapping1)).NotTo(Succeed())
				})
			})

			When("Subnets to be reserved overlaps with pod CIDR ", func() {
				It("should fail while reserving the subnet that overlaps", func() {
					Expect(ipam.SetReservedSubnets(toBeReservedOverlapping2)).NotTo(Succeed())
				})
			})

			When("Subnets to be reserved overlaps with service CIDR ", func() {
				It("should fail while reserving the subnet that overlaps", func() {
					Expect(ipam.SetReservedSubnets(toBeReservedOverlapping3)).NotTo(Succeed())
				})
			})

			When("Subnets to be reserved overlaps with external CIDR ", func() {
				It("should fail while reserving the subnet that overlaps", func() {
					Expect(ipam.SetReservedSubnets(toBeReservedOverlapping4)).NotTo(Succeed())
				})
			})

			When("Subnets to be reserved overlaps with pod CIDR of a remote cluster ", func() {
				It("should fail while reserving the subnet that overlaps", func() {
					Expect(ipam.SetReservedSubnets(toBeReservedOverlapping5)).NotTo(Succeed())
				})
			})

			When("Subnets to be reserved overlaps with external CIDR of a remote cluster ", func() {
				It("should fail while reserving the subnet that overlaps", func() {
					Expect(ipam.SetReservedSubnets(toBeReservedOverlapping6)).NotTo(Succeed())
				})
			})
		})

		Context("Making available subnets previously reserved", func() {
			JustBeforeEach(func() {
				// Reserve the subnets.
				Expect(ipam.SetReservedSubnets(toBeReservedSubnets1)).To(Succeed())
				Expect(ipam.ipamStorage.getReservedSubnets()).To(ContainElements(toBeReservedSubnets1))
				checkForPrefixes(toBeReservedSubnets1)
			})
			When("reserved subnets are no more needed", func() {
				It("should remove all the previously reserved networks", func() {
					Expect(ipam.SetReservedSubnets(nil)).To(BeNil())
					Expect(ipam.ipamStorage.getReservedSubnets()).Should(HaveLen(0))
				})
			})

			When("new subnets are reserved and existing ones are freed", func() {
				It("should return nil and update the reserved subnets", func() {
					Expect(ipam.SetReservedSubnets(toBeReservedSubnets2)).To(Succeed())
					Expect(ipam.ipamStorage.getReservedSubnets()).To(ContainElements(toBeReservedSubnets2))
					checkForPrefixes(toBeReservedSubnets2)
				})
			})
		})
	})

	Describe("BelongsToPodCIDR", func() {
		BeforeEach(func() {
			Expect(ipam.ipamStorage.updatePodCIDR("10.244.0.0/16")).To(Succeed())
		})
		Context("Calling it on an IP in the pod CIDR", func() {
			It("should return true", func() {
				response, err := ipam.BelongsToPodCIDR(ctx, &BelongsRequest{Ip: "10.244.0.1"})
				Expect(err).ToNot(HaveOccurred())
				Expect(response.GetBelongs()).To(BeTrue())
			})
		})
		Context("Calling it on an IP not in the pod CIDR", func() {
			It("should return false", func() {
				response, err := ipam.BelongsToPodCIDR(ctx, &BelongsRequest{Ip: "1.2.3.4"})
				Expect(err).ToNot(HaveOccurred())
				Expect(response.GetBelongs()).To(BeFalse())
			})
		})
		Context("Calling it on an invalid IP", func() {
			It("should return an error", func() {
				_, err := ipam.BelongsToPodCIDR(ctx, &BelongsRequest{Ip: "10.9.9"})
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

func checkForPrefixes(subnets []string) {
	for _, s := range subnets {
		prefix, err := ipam.ipamStorage.ReadPrefix(context.TODO(), s)
		Expect(err).ToNot(HaveOccurred())
		Expect(prefix.Cidr).To(Equal(s))
	}
}

func getIpamStorageResource() (*ipamv1alpha1.IpamStorage, error) {
	ipamConfig := &ipamv1alpha1.IpamStorage{}
	list, err := dynClient.Resource(ipamv1alpha1.IpamStorageGroupVersionResource).Namespace(namespace).List(
		ctx,
		v1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s",
				consts.IpamStorageResourceLabelKey,
				consts.IpamStorageResourceLabelValue),
		},
	)
	if err != nil {
		return nil, err
	}
	if len(list.Items) == 0 {
		return nil, k8serrors.NewNotFound(ipamv1alpha1.IpamStorageGroupResource, "")
	}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(list.Items[0].Object, ipamConfig)
	if err != nil {
		return nil, err
	}
	return ipamConfig, nil
}
