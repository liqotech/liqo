// Copyright 2019-2025 The Liqo Authors
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
	"fmt"
	"net"
	"net/netip"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	grpcutils "github.com/liqotech/liqo/pkg/utils/grpc"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("IPAM integration tests", func() {
	const (
		testNamespace = "test"

		grpcAddress = "0.0.0.0"
		grpcPort    = consts.IpamPort
	)

	var (
		ctx               context.Context
		fakeClientBuilder *fake.ClientBuilder
		err               error

		ipamServer *LiqoIPAM
		serverOpts = &ServerOptions{
			Pools:           consts.PrivateAddressSpace,
			Port:            grpcPort,
			SyncInterval:    time.Duration(0), // we disable sync routine as already tested in sync_test.go
			SyncGracePeriod: time.Duration(0), // same as above
			GraphvizEnabled: false,
		}
		server        *grpc.Server
		lis           net.Listener
		listenAddress = fmt.Sprintf("%s:%d", grpcAddress, grpcPort)
		errServe      error

		ipamClient IPAMClient
		conn       *grpc.ClientConn

		addPreAllocated = func(nw *ipamv1alpha1.Network, preAllocated uint32) *ipamv1alpha1.Network {
			nw.Spec.PreAllocated = preAllocated
			return nw
		}
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Build kubernetes (fake) client
		fakeClientBuilder = fake.NewClientBuilder().WithScheme(scheme.Scheme)
		cl := fakeClientBuilder.WithObjects(
			testutil.FakeNetworkPodCIDR(),
			testutil.FakeNetworkServiceCIDR(),
			testutil.FakeNetworkExternalCIDR(),
			testutil.FakeNetworkInternalCIDR(),
		).Build()

		// Start grpc ipam server
		ipamServer, err = New(ctx, cl, serverOpts)
		Expect(err).ToNot(HaveOccurred())
		Expect(ipamServer).ToNot(BeNil())
		Expect(ipamServer.IpamCore).ToNot(BeNil())

		server = grpc.NewServer()
		Expect(server).ToNot(BeNil())
		grpc_health_v1.RegisterHealthServer(server, ipamServer.HealthServer) // register health service
		RegisterIPAMServer(server, ipamServer)                               // register IPAM service

		lis, err = net.Listen("tcp", listenAddress)
		Expect(err).ToNot(HaveOccurred())
		Expect(lis).ToNot(BeNil())
		go func() {
			errServe = fmt.Errorf("server failed when serving") // set error to check if server fails
			errServe = server.Serve(lis)
		}()

		// Start gprc ipam client
		conn, err = grpc.NewClient(listenAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
		Expect(err).ToNot(HaveOccurred())
		Expect(conn).ToNot(BeNil())
		Expect(grpcutils.WaitForConnectionReady(ctx, conn, 10*time.Second)).To(Succeed())
		ipamClient = NewIPAMClient(conn)
	})

	AfterEach(func() {
		Expect(conn.Close()).To(Succeed()) // close grpc client
		server.GracefulStop()              // graceful stop grpc server
		time.Sleep(10 * time.Millisecond)  // wait a bit for the gprc server to gracefully stop
		Expect(errServe).ToNot(HaveOccurred())
	})

	Describe("Preinstalled networks", func() {
		It("should have reserved preinstalled networks", func() {
			Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix(testutil.PodCIDR))).To(BeFalse())
			Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix(testutil.ServiceCIDR))).To(BeFalse())
			Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix(testutil.ExternalCIDR))).To(BeFalse())
			Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix(testutil.InternalCIDR))).To(BeFalse())
		})
	})

	Describe("Acquiring networks", func() {
		When("acquiring a network not occupied", func() {
			When("remapping is allowed", func() {
				It("should acquire the network without remapping", func() {
					res, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
						Cidr:      "10.20.0.0/16",
						Immutable: false,
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(res).ToNot(BeNil())
					Expect(res.Cidr).To(Equal("10.20.0.0/16"))
					Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix("10.20.0.0/16"))).To(BeFalse())
				})
			})

			When("remapping is not allowed", func() {
				It("should acquire the network without remapping", func() {
					res, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
						Cidr:      "10.20.0.0/16",
						Immutable: true,
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(res).ToNot(BeNil())
					Expect(res.Cidr).To(Equal("10.20.0.0/16"))
					Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix("10.20.0.0/16"))).To(BeFalse())
				})
			})
		})

		When("acquiring a network already occupied", func() {
			BeforeEach(func() {
				res, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
					Cidr:      "10.20.0.0/16",
					Immutable: true,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				Expect(res.Cidr).To(Equal("10.20.0.0/16"))
				Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix("10.20.0.0/16"))).To(BeFalse())
			})

			When("remapping is allowed", func() {
				It("should acquire the network and get a remapping", func() {
					res, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
						Cidr:      "10.20.0.0/16",
						Immutable: false,
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(res).ToNot(BeNil())
					Expect(res.Cidr).ToNot(Equal("10.20.0.0/16"))
					Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix(res.Cidr))).To(BeFalse())
				})

				It("should acquire a network that contains it and get a remapping", func() {
					res, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
						Cidr:      "10.20.0.0/17",
						Immutable: false,
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(res).ToNot(BeNil())
					Expect(res.Cidr).ToNot(Equal("10.20.0.0/17"))
					Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix(res.Cidr))).To(BeFalse())
				})

				It("should acquire a network that contains it and get a remapping", func() {
					res, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
						Cidr:      "10.20.0.0/15",
						Immutable: false,
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(res).ToNot(BeNil())
					Expect(res.Cidr).ToNot(Equal("10.20.0.0/15"))
					Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix(res.Cidr))).To(BeFalse())
				})
			})

			When("remapping is not allowed", func() {
				It("should not acquire the network and get an error", func() {
					_, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
						Cidr:      "10.20.0.0/16",
						Immutable: true,
					})
					Expect(err).To(HaveOccurred())
				})

				It("should not acquire a network that is contained in and get an error", func() {
					_, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
						Cidr:      "10.20.0.0/17",
						Immutable: true,
					})
					Expect(err).To(HaveOccurred())
				})

				It("should not acquire a network that contains it and get an error", func() {
					_, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
						Cidr:      "10.20.0.0/15",
						Immutable: true,
					})
					Expect(err).To(HaveOccurred())
				})
			})

		})

		When("acquiring a network out of the pools", func() {
			It("should not acquire the network and get an error", func() {
				_, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
					Cidr:      "50.0.0.0/24",
					Immutable: false,
				})
				Expect(err).To(HaveOccurred())
			})
		})

		When("acquiring a network bigger than a pool", func() {
			It("should not acquire the network and get an error", func() {
				_, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
					Cidr:      "192.168.0.0/15",
					Immutable: false,
				})
				Expect(err).To(HaveOccurred())
			})
		})

		When("acquiring a network equal to a pool", func() {
			It("should acquire the network", func() {
				res, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
					Cidr:      "192.168.0.0/16",
					Immutable: true,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				Expect(res.Cidr).To(Equal("192.168.0.0/16"))
				Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix("192.168.0.0/16"))).To(BeFalse())
			})
		})

		When("acquiring a network with preallocated IPs", func() {
			It("should acquire the network and preallocate the IPs", func() {
				res, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
					Cidr:         "10.20.0.0/16",
					Immutable:    true,
					PreAllocated: 2,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				Expect(res.Cidr).To(Equal("10.20.0.0/16"))
				Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix(res.Cidr))).To(BeFalse())
				available, err := ipamServer.ipIsAvailable(netip.MustParseAddr("10.20.0.0"), netip.MustParsePrefix("10.20.0.0/16"))
				Expect(err).ToNot(HaveOccurred())
				Expect(available).To(BeFalse())
				available, err = ipamServer.ipIsAvailable(netip.MustParseAddr("10.20.0.1"), netip.MustParsePrefix("10.20.0.0/16"))
				Expect(err).ToNot(HaveOccurred())
				Expect(available).To(BeFalse())
				available, err = ipamServer.ipIsAvailable(netip.MustParseAddr("10.20.0.2"), netip.MustParsePrefix("10.20.0.0/16"))
				Expect(err).ToNot(HaveOccurred())
				Expect(available).To(BeTrue())
			})

			It("should not acquire the network if the preallocated IPs are more than the network size", func() {
				_, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
					Cidr:         "192.168.1.0/31",
					Immutable:    true,
					PreAllocated: 3,
				})
				Expect(err).To(HaveOccurred())
				// if at least one preAllocated IP was not acquired, the entire network and the IPs allocated should be released (atomicity)
				Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix("192.168.1.0/31"))).To(BeTrue())
				available, err := ipamServer.ipIsAvailable(netip.MustParseAddr("192.168.1.0"), netip.MustParsePrefix("192.168.1.0/31"))
				Expect(err).ToNot(HaveOccurred())
				Expect(available).To(BeTrue())
				available, err = ipamServer.ipIsAvailable(netip.MustParseAddr("192.168.1.1"), netip.MustParsePrefix("192.168.1.0/31"))
				Expect(err).ToNot(HaveOccurred())
				Expect(available).To(BeTrue())
			})
		})

		When("acquiring an invalid network", func() {
			It("should not acquire the network and get an error", func() {
				_, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
					Cidr:      "10.0.0.256/16",
					Immutable: false,
				})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Releasing networks", func() {
		When("releasing an allocated network", func() {
			BeforeEach(func() {
				_, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
					Cidr:      "10.20.0.0/16",
					Immutable: true,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix("10.20.0.0/16"))).To(BeFalse())
			})

			It("should release the network", func() {
				_, err := ipamClient.NetworkRelease(ctx, &NetworkReleaseRequest{
					Cidr: "10.20.0.0/16",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix("10.20.0.0/16"))).To(BeTrue())
			})
		})

		When("releasing an unallocated network", func() {
			It("should do nothing and succeed without errors for idempotency", func() {
				_, err := ipamClient.NetworkRelease(ctx, &NetworkReleaseRequest{
					Cidr: "10.20.0.0/16",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix("10.20.0.0/16"))).To(BeTrue())
			})
		})

		When("releasing a network with preallocated IPs", func() {
			BeforeEach(func() {
				_, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
					Cidr:         "10.20.0.0/16",
					Immutable:    true,
					PreAllocated: 2,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix("10.20.0.0/16"))).To(BeFalse())
				available, err := ipamServer.ipIsAvailable(netip.MustParseAddr("10.20.0.0"), netip.MustParsePrefix("10.20.0.0/16"))
				Expect(err).ToNot(HaveOccurred())
				Expect(available).To(BeFalse())
				available, err = ipamServer.ipIsAvailable(netip.MustParseAddr("10.20.0.1"), netip.MustParsePrefix("10.20.0.0/16"))
				Expect(err).ToNot(HaveOccurred())
				Expect(available).To(BeFalse())
			})

			It("should release the network and the preallocated IPs", func() {
				_, err := ipamClient.NetworkRelease(ctx, &NetworkReleaseRequest{
					Cidr: "10.20.0.0/16",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix("10.20.0.0/16"))).To(BeTrue())
				available, err := ipamServer.ipIsAvailable(netip.MustParseAddr("10.20.0.0"), netip.MustParsePrefix("10.20.0.0/16"))
				Expect(err).ToNot(HaveOccurred())
				Expect(available).To(BeTrue())
				available, err = ipamServer.ipIsAvailable(netip.MustParseAddr("10.20.0.1"), netip.MustParsePrefix("10.20.0.0/16"))
				Expect(err).ToNot(HaveOccurred())
				Expect(available).To(BeTrue())
			})
		})

		When("releasing an invalid network", func() {
			It("should get an error", func() {
				_, err := ipamClient.NetworkRelease(ctx, &NetworkReleaseRequest{
					Cidr: "10.0.0.256/16",
				})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Checking for networks availability", func() {
		// normal
		When("checking for an available network", func() {
			It("should return true", func() {
				res, err := ipamClient.NetworkIsAvailable(ctx, &NetworkAvailableRequest{
					Cidr: "10.20.0.0/16",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				Expect(res.Available).To(BeTrue())
			})
		})

		When("checking for an occupied network", func() {
			BeforeEach(func() {
				_, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
					Cidr:      "10.20.0.0/16",
					Immutable: true,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix("10.20.0.0/16"))).To(BeFalse())
			})

			When("checking for the same network", func() {
				It("should return false", func() {
					res, err := ipamClient.NetworkIsAvailable(ctx, &NetworkAvailableRequest{
						Cidr: "10.20.0.0/16",
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(res).ToNot(BeNil())
					Expect(res.Available).To(BeFalse())
				})
			})

			When("checking for a network that is contained in", func() {
				It("should return false", func() {
					res, err := ipamClient.NetworkIsAvailable(ctx, &NetworkAvailableRequest{
						Cidr: "10.20.0.0/17",
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(res).ToNot(BeNil())
					Expect(res.Available).To(BeFalse())
				})
			})

			When("checking for a network that contains it", func() {
				It("should return false", func() {
					res, err := ipamClient.NetworkIsAvailable(ctx, &NetworkAvailableRequest{
						Cidr: "10.20.0.0/15",
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(res).ToNot(BeNil())
					Expect(res.Available).To(BeFalse())
				})
			})
		})

		When("checking for an out of pool network", func() {
			It("should get an error", func() {
				_, err := ipamClient.NetworkIsAvailable(ctx, &NetworkAvailableRequest{
					Cidr: "50.0.0.0/24",
				})
				Expect(err).To(HaveOccurred())
			})
		})

		When("checking for a network bigger than a pool", func() {
			It("should get an error", func() {
				_, err := ipamClient.NetworkIsAvailable(ctx, &NetworkAvailableRequest{
					Cidr: "192.168.0.0/15",
				})
				Expect(err).To(HaveOccurred())
			})
		})

		When("checking for an invalid network", func() {
			It("should get an error", func() {
				_, err := ipamClient.NetworkIsAvailable(ctx, &NetworkAvailableRequest{
					Cidr: "10.0.0.256/16",
				})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Acquiring IPs", func() {
		When("acquiring IPs from an allocated network", func() {
			var firstIP, secondIP string

			BeforeEach(func() {
				// Allocate network of size 4, with 2 preallocated IPs
				_, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
					Cidr:         "10.20.0.0/30",
					Immutable:    true,
					PreAllocated: 2,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix("10.20.0.0/30"))).To(BeFalse())

				// First IP
				res, err := ipamClient.IPAcquire(ctx, &IPAcquireRequest{
					Cidr: "10.20.0.0/30",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				firstIP = res.Ip

				// Second IP
				res, err = ipamClient.IPAcquire(ctx, &IPAcquireRequest{
					Cidr: "10.20.0.0/30",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				secondIP = res.Ip
			})

			It("the preAllocated IPs should be allocated", func() {
				Expect(ipamServer.ipIsAvailable(netip.MustParseAddr("10.20.0.0"), netip.MustParsePrefix("10.20.0.0/30"))).To(BeFalse())
				Expect(ipamServer.ipIsAvailable(netip.MustParseAddr("10.20.0.1"), netip.MustParsePrefix("10.20.0.0/30"))).To(BeFalse())
			})

			It("should have acquired IPs", func() {
				Expect(firstIP).ToNot(Equal(secondIP))
				Expect(ipamServer.ipIsAvailable(netip.MustParseAddr(firstIP), netip.MustParsePrefix("10.20.0.0/30"))).To(BeFalse())
				Expect(ipamServer.ipIsAvailable(netip.MustParseAddr(secondIP), netip.MustParsePrefix("10.20.0.0/30"))).To(BeFalse())
				Expect(firstIP).ToNot(BeEmpty())
				Expect(secondIP).ToNot(BeEmpty())
				Expect(firstIP).ToNot(Or(Equal("10.20.0.0"), Equal("10.20.0.1")))  // allocated IPs should differ from preAllocated ones
				Expect(secondIP).ToNot(Or(Equal("10.20.0.0"), Equal("10.20.0.1"))) // allocated IPs should differ from preAllocated ones

			})

			It("should not acquire an IP if network is full", func() {
				_, err := ipamClient.IPAcquire(ctx, &IPAcquireRequest{
					Cidr: "10.20.0.0/30",
				})
				Expect(err).To(HaveOccurred())
			})
		})

		When("acquiring a free IP from a network contained in an allocated one", func() {
			BeforeEach(func() {
				_, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
					Cidr:      "10.20.0.0/16",
					Immutable: true,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix("10.20.0.0/16"))).To(BeFalse())
			})

			It("should not acquire the IP and get an error", func() {
				_, err := ipamClient.IPAcquire(ctx, &IPAcquireRequest{
					Cidr: "10.20.0.0/24",
				})
				Expect(err).To(HaveOccurred())
			})
		})

		When("acquiring an IP from a network not allocated", func() {
			It("should get an error", func() {
				_, err := ipamClient.IPAcquire(ctx, &IPAcquireRequest{
					Cidr: "10.20.0.0/16",
				})
				Expect(err).To(HaveOccurred())
			})
		})

		When("acquiring an IP from a network out of the pools", func() {
			It("should get an error", func() {
				_, err := ipamClient.IPAcquire(ctx, &IPAcquireRequest{
					Cidr: "50.0.0.0/24",
				})
				Expect(err).To(HaveOccurred())
			})
		})

		When("acquiring an IP from a network bigger than a pool", func() {
			It("should get an error", func() {
				_, err := ipamClient.IPAcquire(ctx, &IPAcquireRequest{
					Cidr: "192.168.0.0/15",
				})
				Expect(err).To(HaveOccurred())
			})
		})

		When("acquiring an IP from an invalid network", func() {
			It("should get an error", func() {
				_, err := ipamClient.IPAcquire(ctx, &IPAcquireRequest{
					Cidr: "10.0.0.256/16",
				})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Releasing IPs", func() {
		When("releasing an allocated IP", func() {
			var ip string

			BeforeEach(func() {
				_, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
					Cidr:         "10.20.0.0/16",
					Immutable:    true,
					PreAllocated: 1,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix("10.20.0.0/16"))).To(BeFalse())
				Expect(ipamServer.ipIsAvailable(netip.MustParseAddr("10.20.0.0"), netip.MustParsePrefix("10.20.0.0/16"))).To(BeFalse()) // preAllocated

				res, err := ipamClient.IPAcquire(ctx, &IPAcquireRequest{
					Cidr: "10.20.0.0/16",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(res).ToNot(BeNil())
				ip = res.Ip
				Expect(ip).ToNot(Equal("10.20.0.0")) // allocated IP should differ from preAllocated one
			})

			It("should release the allocated IP and guarantee idempotency", func() {
				// Release the IP
				_, err := ipamClient.IPRelease(ctx, &IPReleaseRequest{
					Cidr: "10.20.0.0/16",
					Ip:   ip,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(ipamServer.ipIsAvailable(netip.MustParseAddr(ip), netip.MustParsePrefix("10.20.0.0/16"))).To(BeTrue())

				// Release the IP again. It should not return an error and the IP should still be available (idempotency)
				_, err = ipamClient.IPRelease(ctx, &IPReleaseRequest{
					Cidr: "10.20.0.0/16",
					Ip:   ip,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(ipamServer.ipIsAvailable(netip.MustParseAddr(ip), netip.MustParsePrefix("10.20.0.0/16"))).To(BeTrue())
				// should not interfere with preAllocated IP
				Expect(ipamServer.ipIsAvailable(netip.MustParseAddr("10.20.0.0"), netip.MustParsePrefix("10.20.0.0/16"))).To(BeFalse())
			})

			It("should not release an IP that is preallocated", func() {
				_, err := ipamClient.IPRelease(ctx, &IPReleaseRequest{
					Cidr: "10.20.0.0/16",
					Ip:   "10.20.0.0",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(ipamServer.ipIsAvailable(netip.MustParseAddr("10.20.0.0"), netip.MustParsePrefix("10.20.0.0/16"))).To(BeTrue())

				// Run sync.
				// Add the network with the preallocated IP to the cluster (i.e., inject to the client),
				// so that the sync routine does not delete it.
				cl := fakeClientBuilder.WithObjects(
					addPreAllocated(testutil.FakeNetwork("net1", testNamespace, "10.20.0.0/16", nil), 1),
				).Build()
				ipamServer.Client = cl
				Expect(ipamServer.syncNetworks(ctx)).To(Succeed())
				Expect(ipamServer.syncIPs(ctx)).To(Succeed())

				// The preallocated IP should now be allocated again
				Expect(ipamServer.ipIsAvailable(netip.MustParseAddr("10.20.0.0"), netip.MustParsePrefix("10.20.0.0/16"))).To(BeFalse())
			})
		})

		When("releasing an IP from an unallocated network", func() {
			It("should do nothing and succeed without errors for idempotency", func() {
				_, err := ipamClient.IPRelease(ctx, &IPReleaseRequest{
					Cidr: "10.20.0.0/16",
					Ip:   "10.20.0.0",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(ipamServer.ipIsAvailable(netip.MustParseAddr("10.20.0.0"), netip.MustParsePrefix("10.20.0.0/16"))).To(BeTrue())
			})
		})

		When("releasing an IP outside of the pools", func() {
			It("should get an error", func() {
				_, err := ipamClient.IPRelease(ctx, &IPReleaseRequest{
					Cidr: "50.0.0.0/16",
					Ip:   "50.0.0.0",
				})
				Expect(err).To(HaveOccurred())
			})
		})

		When("input is valid", func() {
			BeforeEach(func() {
				_, err := ipamClient.NetworkAcquire(ctx, &NetworkAcquireRequest{
					Cidr:      "10.20.0.0/16",
					Immutable: true,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(ipamServer.networkIsAvailable(netip.MustParsePrefix("10.20.0.0/16"))).To(BeFalse())
			})

			When("releasing an invalid ip", func() {
				It("should get an error", func() {
					_, err := ipamClient.IPRelease(ctx, &IPReleaseRequest{
						Cidr: "10.20.0.0/16",
						Ip:   "10.0.0.256",
					})
					Expect(err).To(HaveOccurred())
				})
			})

			When("releasing an ip from an invalid network", func() {
				It("should get an error", func() {
					_, err := ipamClient.IPRelease(ctx, &IPReleaseRequest{
						Cidr: "10.0.0.256/16",
						Ip:   "10.0.0.0",
					})
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})
