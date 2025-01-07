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
//

package localstatus_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pterm/pterm"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/info"
	"github.com/liqotech/liqo/pkg/liqoctl/info/localstatus"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("NetworkChecker tests", func() {

	var (
		clientBuilder fake.ClientBuilder
		nc            *localstatus.NetworkChecker
		ctx           context.Context
		options       info.Options
		baseObjects   = []client.Object{
			testutil.FakeNetworkPodCIDR(),
			testutil.FakeNetworkServiceCIDR(),
			testutil.FakeNetworkInternalCIDR(),
			testutil.FakeNetworkExternalCIDR(),
		}
	)

	BeforeEach(func() {
		ctx = context.Background()
		clientBuilder = *fake.NewClientBuilder().WithScheme(scheme.Scheme)

		options = info.Options{Factory: factory.NewForLocal()}
		options.Printer = output.NewFakePrinter(GinkgoWriter)
		options.KubeClient = k8sfake.NewSimpleClientset()
	})

	Describe("Testing the NetworkChecker", func() {
		Context("Collecting and retrieving the data", func() {
			It("should collect the data and return the right result", func() {

				// Set up the fake clients
				clientBuilder.WithObjects(baseObjects...)
				options.CRClient = clientBuilder.Build()
				options.LiqoNamespace = liqoconsts.DefaultLiqoNamespace

				By("Collecting the data")
				nc = &localstatus.NetworkChecker{}
				nc.Collect(ctx, options)

				By("Verifying that no errors have been raised")
				Expect(nc.GetCollectionErrors()).To(BeEmpty())

				By("Checking the correctness of the data in the struct")
				data := nc.GetData().(localstatus.Network)

				Expect(data.PodCIDR).To(Equal(testutil.PodCIDR))
				Expect(data.ServiceCIDR).To(Equal(testutil.ServiceCIDR))
				Expect(data.ExternalCIDR).To(Equal(testutil.ExternalCIDR))
				Expect(data.InternalCIDR).To(Equal(testutil.InternalCIDR))

				By("Checking the formatted output")
				text := nc.Format(options)
				text = pterm.RemoveColorFromString(text)
				text = testutil.SqueezeWhitespaces(text)

				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Pod CIDR: %s", testutil.PodCIDR),
				))
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Service CIDR: %s", testutil.ServiceCIDR),
				))
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("External CIDR: %s", testutil.ExternalCIDR),
				))
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Internal CIDR: %s", testutil.InternalCIDR),
				))
			})
		})
	})
})
