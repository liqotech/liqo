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

var _ = Describe("IPs tests", func() {
	const (
		testNamespace = "test"
	)

	var (
		ctx               context.Context
		fakeClientBuilder *fake.ClientBuilder
		err               error

		ipamServer *LiqoIPAM
		ipamCore   *ipamcore.Ipam

		emptyIPStatus = func(ip *ipamv1alpha1.IP) *ipamv1alpha1.IP {
			ip.Status = ipamv1alpha1.IPStatus{}
			return ip
		}

		addDeletionTimestamp = func(ip *ipamv1alpha1.IP) *ipamv1alpha1.IP {
			ip.SetDeletionTimestamp(ptr.To(metav1.NewTime(time.Now())))
			ip.SetFinalizers([]string{"test-finalizer"}) // fake client requires at least one finalizer if deletion timestamp is set
			return ip
		}
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeClientBuilder = fake.NewClientBuilder().WithScheme(scheme.Scheme)
		ipamCore, err = ipamcore.NewIpam([]netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
		Expect(err).ToNot(HaveOccurred())
	})

	Context("List ips on cluster", func() {
		BeforeEach(func() {
			// Add in-cluster ips
			client := fakeClientBuilder.WithObjects(
				testutil.FakeIP("ip1", testNamespace, "10.1.0.0", "10.1.0.0/16", nil, nil, false),
				testutil.FakeIP("ip2", testNamespace, "10.1.0.1", "10.1.0.0/16", nil, nil, false),
				testutil.FakeIP("ip3", testNamespace, "10.2.0.0", "10.2.0.0/16", nil, nil, false),

				// IP with no status
				emptyIPStatus(testutil.FakeIP("ip4", testNamespace, "10.3.0.0", "10.3.0.0/16", nil, nil, false)),

				// IP with deletion timestamp
				addDeletionTimestamp(testutil.FakeIP("ip5", testNamespace, "10.3.0.1", "10.3.0.0/16", nil, nil, false)),
			).Build()

			ipamServer = &LiqoIPAM{
				Client:   client,
				IpamCore: ipamCore,
				opts: &ServerOptions{
					GraphvizEnabled: false,
				},
			}
		})

		It("should correctly list ips on cluster", func() {
			ips, err := ipamServer.listIPsOnCluster(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(ips).To(HaveLen(3))

			Expect(ips).To(HaveKeyWithValue(netip.MustParseAddr("10.1.0.0"), netip.MustParsePrefix("10.1.0.0/16")))
			Expect(ips).To(HaveKeyWithValue(netip.MustParseAddr("10.1.0.1"), netip.MustParsePrefix("10.1.0.0/16")))
			Expect(ips).To(HaveKeyWithValue(netip.MustParseAddr("10.2.0.0"), netip.MustParsePrefix("10.2.0.0/16")))
			Expect(ips).ToNot(HaveKey(netip.MustParseAddr("10.3.0.0")))
			Expect(ips).ToNot(HaveKey(netip.MustParseAddr("10.3.0.1")))
		})
	})

})
