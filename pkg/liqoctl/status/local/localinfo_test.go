// Copyright 2019-2023 The Liqo Authors
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

package statuslocal

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/status"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("LocalInfo", func() {
	const (
		clusterID   = "fake"
		clusterName = "fake"
	)

	var (
		clientBuilder     fake.ClientBuilder
		lic               *LocalInfoChecker
		ctx               context.Context
		text              string
		options           status.Options
		argsClusterLabels []string
		baseObjects       = []client.Object{
			testutil.FakeNode(),
			testutil.FakeClusterIDConfigMap(liqoconsts.DefaultLiqoNamespace, clusterID, clusterName),
			testutil.FakeIPAM(liqoconsts.DefaultLiqoNamespace),
		}
	)

	BeforeEach(func() {
		ctx = context.Background()
		clientBuilder = *fake.NewClientBuilder().WithScheme(scheme.Scheme)
		for k, v := range testutil.ClusterLabels {
			argsClusterLabels = append(argsClusterLabels, fmt.Sprintf("%s=%s", k, v))
		}

		options = status.Options{Factory: factory.NewForLocal()}
		options.Printer = output.NewFakePrinter(GinkgoWriter)
		options.KubeClient = k8sfake.NewSimpleClientset()
		_, err := options.KubeClient.CoreV1().Nodes().Create(ctx, baseObjects[0].(*corev1.Node), metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
	})

	Context("Creating a new LocalInfoChecker", func() {
		JustBeforeEach(func() {
			lic = NewLocalInfoChecker(&options)
		})
		It("should return a valid LocalInfoChecker", func() {
			Expect(lic.localInfoSection).To(Equal(output.NewRootSection()))
		})
	})

	type TestArgs struct {
		clusterLabels, apiServerOverride bool
		endpointServiceType              corev1.ServiceType
	}

	DescribeTable("Collecting and Formatting LocalInfoChecker", func(args TestArgs) {
		objects := append([]client.Object{}, baseObjects...)
		if args.clusterLabels {
			objects = append(objects, testutil.FakeControllerManagerDeployment(argsClusterLabels))
		} else {
			objects = append(objects, testutil.FakeControllerManagerDeployment(nil))
		}
		if args.apiServerOverride {
			objects = append(objects, testutil.FakeLiqoAuthDeployment(testutil.OverrideAPIAddress))
		} else {
			objects = append(objects, testutil.FakeLiqoAuthDeployment(""))
		}
		objects = append(objects,
			testutil.FakeLiqoAuthService(args.endpointServiceType),
			testutil.FakeLiqoGatewayService(args.endpointServiceType),
		)

		clientBuilder.WithObjects(objects...)
		options.CRClient = clientBuilder.Build()
		options.LiqoNamespace = liqoconsts.DefaultLiqoNamespace
		lic = NewLocalInfoChecker(&options)
		lic.Collect(ctx)

		text = lic.Format()
		text = pterm.RemoveColorFromString(text)
		text = testutil.SqueezeWhitespaces(text)

		Expect(lic.HasSucceeded()).To(BeTrue())
		Expect(text).To(ContainSubstring(
			pterm.Sprintf("Cluster ID: %s", clusterID),
		))
		Expect(text).To(ContainSubstring(
			pterm.Sprintf("Cluster Name: %s", clusterName),
		))
		if args.clusterLabels {
			for _, v := range testutil.ClusterLabels {
				Expect(text).To(ContainSubstring(v))
			}
		}
		Expect(text).To(ContainSubstring(
			pterm.Sprintf("Pod CIDR: %s", testutil.PodCIDR),
		))
		Expect(text).To(ContainSubstring(
			pterm.Sprintf("Service CIDR: %s", testutil.ServiceCIDR),
		))
		Expect(text).To(ContainSubstring(
			pterm.Sprintf("External CIDR: %s", testutil.ExternalCIDR),
		))
		for _, v := range testutil.ReservedSubnets {
			Expect(text).To(ContainSubstring(v))
		}

		Expect(text).To(ContainSubstring(
			pterm.Sprintf("VPN Gateway: udp://%s:%d", testutil.EndpointIP, testutil.VPNGatewayPort),
		))
		Expect(text).To(ContainSubstring(
			pterm.Sprintf("Authentication: https://%s:%d", testutil.EndpointIP, testutil.AuthenticationPort),
		))
		if args.apiServerOverride {
			Expect(text).To(ContainSubstring(
				pterm.Sprintf("Kubernetes API Server: %s", fmt.Sprintf("https://%v", testutil.OverrideAPIAddress)),
			))
		} else {
			Expect(text).To(ContainSubstring(
				pterm.Sprintf("Kubernetes API Server: %s", fmt.Sprintf("https://%v:6443", testutil.EndpointIP)),
			))
		}

	},
		Entry("Standard case with NodePort",
			TestArgs{false, false, corev1.ServiceTypeNodePort}),
		Entry("Standard case with LoadBalancer",
			TestArgs{false, false, corev1.ServiceTypeLoadBalancer}),
		Entry("Cluster Labels with NodePort",
			TestArgs{true, false, corev1.ServiceTypeNodePort}),
		Entry("Cluster Labels with LoadBalancer",
			TestArgs{true, false, corev1.ServiceTypeLoadBalancer}),
		Entry("API Server Override with NodePort",
			TestArgs{false, true, corev1.ServiceTypeNodePort}),
		Entry("API Server Override with LoadBalancer",
			TestArgs{false, true, corev1.ServiceTypeLoadBalancer}),
		Entry("Cluster Labels and API Server Override with NodePort",
			TestArgs{true, true, corev1.ServiceTypeNodePort}),
		Entry("Cluster Labels and API Server Override with LoadBalancer",
			TestArgs{true, true, corev1.ServiceTypeLoadBalancer}),
	)
})
