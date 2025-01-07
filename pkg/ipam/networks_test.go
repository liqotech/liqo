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
	"net/netip"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	ipamcore "github.com/liqotech/liqo/pkg/ipam/core"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("Networks tests", func() {
	const (
		testNamespace = "test"
	)

	var (
		ctx               context.Context
		fakeClientBuilder *fake.ClientBuilder
		err               error

		ipamServer *LiqoIPAM
		ipamCore   *ipamcore.Ipam

		addPreAllocated = func(nw *ipamv1alpha1.Network, preAllocated uint32) *ipamv1alpha1.Network {
			nw.Spec.PreAllocated = preAllocated
			return nw
		}

		emptyNetworkStatus = func(nw *ipamv1alpha1.Network) *ipamv1alpha1.Network {
			nw.Status = ipamv1alpha1.NetworkStatus{}
			return nw
		}

		addDeletionTimestamp = func(nw *ipamv1alpha1.Network) *ipamv1alpha1.Network {
			nw.SetDeletionTimestamp(ptr.To(metav1.NewTime(time.Now())))
			nw.SetFinalizers([]string{"test-finalizer"}) // fake client requires at least one finalizer if deletion timestamp is set
			return nw
		}
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeClientBuilder = fake.NewClientBuilder().WithScheme(scheme.Scheme)
		ipamCore, err = ipamcore.NewIpam([]netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
		Expect(err).ToNot(HaveOccurred())
	})

	Context("List networks on cluster", func() {
		BeforeEach(func() {
			// Add in-cluster networks
			client := fakeClientBuilder.WithObjects(
				testutil.FakeNetwork("net1", testNamespace, "10.1.0.0/16", nil),
				testutil.FakeNetwork("net2", testNamespace, "10.2.0.0/16", nil),
				testutil.FakeNetwork("net3", testNamespace, "10.3.0.0/16", nil),

				// Network with preAllocated fields
				addPreAllocated(testutil.FakeNetwork("net4", testNamespace, "10.4.0.0/16", nil), 10),

				// Network with no status
				emptyNetworkStatus(testutil.FakeNetwork("net5", testNamespace, "10.5.0.0/16)", nil)),

				// Network in deletion
				addDeletionTimestamp(testutil.FakeNetwork("net6", testNamespace, "10.6.0.0/16", nil)),
			).Build()

			ipamServer = &LiqoIPAM{
				Client:   client,
				IpamCore: ipamCore,
				opts: &ServerOptions{
					GraphvizEnabled: false,
				},
			}
		})

		It("should correctly list networks on cluster", func() {
			nets, err := ipamServer.listNetworksOnCluster(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(nets).To(HaveLen(4))

			Expect(nets).To(HaveKey(netip.MustParsePrefix("10.1.0.0/16")))
			Expect(nets).To(HaveKey(netip.MustParsePrefix("10.2.0.0/16")))
			Expect(nets).To(HaveKey(netip.MustParsePrefix("10.3.0.0/16")))
			Expect(nets).To(HaveKeyWithValue(netip.MustParsePrefix("10.4.0.0/16"), prefixDetails{10})) // network with preAllocated field
			Expect(nets).ToNot(HaveKey(netip.MustParsePrefix(("10.5.0.0/16"))))                        // network with no status
			Expect(nets).ToNot(HaveKey(netip.MustParsePrefix(("10.6.0.0/16"))))                        // network in deletion
		})
	})

})
