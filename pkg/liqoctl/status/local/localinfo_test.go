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
	"strings"

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
		namespace   = "liqo"
	)

	var (
		clientBuilder     fake.ClientBuilder
		lic               *LocalInfoChecker
		ctx               context.Context
		text              string
		options           status.Options
		baseObjects       []client.Object
		argsClusterLabels []string
	)

	BeforeEach(func() {
		ctx = context.Background()
		clientBuilder = *fake.NewClientBuilder().WithScheme(scheme.Scheme)
		for k, v := range testutil.ClusterLabels {
			argsClusterLabels = append(argsClusterLabels, fmt.Sprintf("%s=%s", k, v))
		}
		baseObjects = []client.Object{
			testutil.FakeClusterIDConfigMap(namespace, clusterID, clusterName),
			testutil.FakeIPAM(namespace),
		}
		options = status.Options{Factory: factory.NewForLocal()}
		options.Printer = output.NewFakePrinter(GinkgoWriter)
		options.KubeClient = k8sfake.NewSimpleClientset()
		_, err := options.KubeClient.CoreV1().Nodes().Create(ctx, &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "fake-node",
				Labels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
				},
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: testutil.APIAddress},
				},
			},
		}, metav1.CreateOptions{})
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
	}

	DescribeTable("Collecting and Formatting LocalInfoChecker", func(args TestArgs) {
		objects := append([]client.Object{}, baseObjects...)
		if args.clusterLabels {
			objects = append(objects, forgeControllerManager(namespace, argsClusterLabels))
		} else {
			objects = append(objects, forgeControllerManager(namespace, nil))
		}
		if args.apiServerOverride {
			objects = append(objects, forgeLiqoAuth(namespace, testutil.OverrideAPIAddress))
		} else {
			objects = append(objects, forgeLiqoAuth(namespace, ""))
		}

		clientBuilder.WithObjects(objects...)
		options.CRClient = clientBuilder.Build()
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
		if args.apiServerOverride {
			Expect(text).To(ContainSubstring(
				pterm.Sprintf("Address: %s", fmt.Sprintf("https://%v", testutil.OverrideAPIAddress)),
			))
		} else {
			Expect(text).To(ContainSubstring(
				pterm.Sprintf("Address: %s", fmt.Sprintf("https://%v:6443", testutil.APIAddress)),
			))
		}
	},
		Entry("Standard case", TestArgs{false, false}),
		Entry("Cluster Labels", TestArgs{true, false}),
		Entry("API Server Override", TestArgs{false, true}),
		Entry("Cluster Labels and API Server Override", TestArgs{true, true}),
	)
})

func forgeLiqoAuth(namespace, addressOverride string) *appv1.Deployment {
	containerArgs := []string{}
	if addressOverride != "" {
		containerArgs = append(containerArgs, "--advertise-api-server-address="+addressOverride)
	}
	return &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "liqo-auth",
			Namespace: namespace,
			Labels: map[string]string{
				liqoconsts.K8sAppNameKey: liqoconsts.AuthAppName,
			},
		},
		Spec: appv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Args: containerArgs},
					},
				},
			},
		},
	}
}

func forgeControllerManager(namespace string, argsClusterLabels []string) *appv1.Deployment {
	containerArgs := []string{}
	if len(argsClusterLabels) != 0 {
		containerArgs = append(containerArgs, "--cluster-labels="+strings.Join(argsClusterLabels, ","))
	}
	return &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "liqo-controller-manager",
			Namespace: namespace,
			Labels: map[string]string{
				liqoconsts.K8sAppNameKey:      "controller-manager",
				liqoconsts.K8sAppComponentKey: "controller-manager",
			},
		},
		Spec: appv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Args: containerArgs},
					},
				},
			},
		},
	}
}
