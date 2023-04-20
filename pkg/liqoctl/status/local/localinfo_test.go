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
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/status"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

type TestArgsNet struct {
	InternalNetworkEnabled, apiServerOverride bool
	endpointServiceType                       corev1.ServiceType
}

type TestArgs struct {
	clusterLabels bool
	net           TestArgsNet
}

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

	DescribeTable("Collecting and Formatting LocalInfoChecker", func(args TestArgs) {
		objects := append([]client.Object{}, baseObjects...)

		var fakectrlman *appv1.Deployment
		if args.clusterLabels {
			fakectrlman = testutil.FakeControllerManagerDeployment(argsClusterLabels, args.net.InternalNetworkEnabled)
		} else {
			fakectrlman = testutil.FakeControllerManagerDeployment(nil, args.net.InternalNetworkEnabled)
		}
		objects = append(objects, fakectrlman)

		if args.net.apiServerOverride {
			objects = append(objects, testutil.FakeLiqoAuthDeployment(testutil.OverrideAPIAddress))
		} else {
			objects = append(objects, testutil.FakeLiqoAuthDeployment(""))
		}
		objects = append(objects,
			testutil.FakeLiqoAuthService(args.net.endpointServiceType),
		)
		if args.net.InternalNetworkEnabled {
			objects = append(objects,
				testutil.FakeLiqoGatewayService(args.net.endpointServiceType),
				testutil.FakeIPAM(liqoconsts.DefaultLiqoNamespace))
		}

		clientBuilder.WithObjects(objects...)
		options.InternalNetworkEnabled = args.net.InternalNetworkEnabled
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
			pterm.Sprintf("Cluster name: %s", clusterName),
		))
		if args.clusterLabels {
			for _, v := range testutil.ClusterLabels {
				Expect(text).To(ContainSubstring(v))
			}
		}
		if args.net.InternalNetworkEnabled {
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
				pterm.Sprintf("Network gateway: udp://%s:%d", testutil.EndpointIP, testutil.VPNGatewayPort),
			))
		} else {
			Expect(text).To(ContainSubstring(pterm.Sprintf("Status: %s", discoveryv1alpha1.PeeringConditionStatusExternal)))
		}
		Expect(text).To(ContainSubstring(
			pterm.Sprintf("Authentication: https://%s:%d", testutil.EndpointIP, testutil.AuthenticationPort),
		))
		if args.net.apiServerOverride {
			Expect(text).To(ContainSubstring(
				pterm.Sprintf("Kubernetes API server: %s", fmt.Sprintf("https://%v", testutil.OverrideAPIAddress)),
			))
		} else {
			Expect(text).To(ContainSubstring(
				pterm.Sprintf("Kubernetes API server: %s", fmt.Sprintf("https://%v:6443", testutil.EndpointIP)),
			))
		}

	},
		Entry("Standard case with NodePort",
			TestArgs{false, TestArgsNet{
				InternalNetworkEnabled: true,
				apiServerOverride:      false,
				endpointServiceType:    corev1.ServiceTypeNodePort,
			}}),
		Entry("Standard case with LoadBalancer",
			TestArgs{false, TestArgsNet{
				InternalNetworkEnabled: true,
				apiServerOverride:      false,
				endpointServiceType:    corev1.ServiceTypeLoadBalancer,
			}}),
		Entry("Cluster Labels with NodePort",
			TestArgs{true, TestArgsNet{
				InternalNetworkEnabled: true,
				apiServerOverride:      false,
				endpointServiceType:    corev1.ServiceTypeNodePort,
			}}),

		Entry("Cluster Labels with LoadBalancer",
			TestArgs{true, TestArgsNet{
				InternalNetworkEnabled: true,
				apiServerOverride:      false,
				endpointServiceType:    corev1.ServiceTypeLoadBalancer,
			}}),
		Entry("API Server Override with NodePort",
			TestArgs{false, TestArgsNet{
				InternalNetworkEnabled: true,
				apiServerOverride:      true,
				endpointServiceType:    corev1.ServiceTypeNodePort,
			}}),
		Entry("API Server Override with LoadBalancer",
			TestArgs{false, TestArgsNet{
				InternalNetworkEnabled: true,
				apiServerOverride:      true,
				endpointServiceType:    corev1.ServiceTypeLoadBalancer,
			}}),
		Entry("Cluster Labels and API Server Override with NodePort",
			TestArgs{true, TestArgsNet{
				InternalNetworkEnabled: true,
				apiServerOverride:      true,
				endpointServiceType:    corev1.ServiceTypeNodePort,
			}}),
		Entry("Cluster Labels and API Server Override with LoadBalancer",
			TestArgs{true, TestArgsNet{
				InternalNetworkEnabled: true,
				apiServerOverride:      true,
				endpointServiceType:    corev1.ServiceTypeLoadBalancer,
			}}),
		Entry("Standard case with NodePort (Internal network Disabled)",
			TestArgs{false, TestArgsNet{
				InternalNetworkEnabled: false,
				apiServerOverride:      false,
				endpointServiceType:    corev1.ServiceTypeNodePort,
			}}),
		Entry("Standard case with LoadBalancer (Internal network Disabled)",
			TestArgs{false, TestArgsNet{
				InternalNetworkEnabled: false,
				apiServerOverride:      false,
				endpointServiceType:    corev1.ServiceTypeLoadBalancer,
			}}),
		Entry("Cluster Labels with NodePort (Internal network Disabled)",
			TestArgs{true, TestArgsNet{
				InternalNetworkEnabled: false,
				apiServerOverride:      false,
				endpointServiceType:    corev1.ServiceTypeNodePort,
			}}),

		Entry("Cluster Labels with LoadBalancer (Internal network Disabled)",
			TestArgs{true, TestArgsNet{
				InternalNetworkEnabled: false,
				apiServerOverride:      false,
				endpointServiceType:    corev1.ServiceTypeLoadBalancer,
			}}),
		Entry("API Server Override with NodePort (Internal network Disabled)",
			TestArgs{false, TestArgsNet{
				InternalNetworkEnabled: false,
				apiServerOverride:      true,
				endpointServiceType:    corev1.ServiceTypeNodePort,
			}}),
		Entry("API Server Override with LoadBalancer (Internal network Disabled)",
			TestArgs{false, TestArgsNet{
				InternalNetworkEnabled: false,
				apiServerOverride:      true,
				endpointServiceType:    corev1.ServiceTypeLoadBalancer,
			}}),
		Entry("Cluster Labels and API Server Override with NodePort (Internal network Disabled)",
			TestArgs{true, TestArgsNet{
				InternalNetworkEnabled: false,
				apiServerOverride:      true,
				endpointServiceType:    corev1.ServiceTypeNodePort,
			}}),
		Entry("Cluster Labels and API Server Override with LoadBalancer (Internal network Disabled)",
			TestArgs{true, TestArgsNet{
				InternalNetworkEnabled: false,
				apiServerOverride:      true,
				endpointServiceType:    corev1.ServiceTypeLoadBalancer,
			}}),
	)
})
