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

package k3s

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/liqotech/liqo/pkg/consts"
)

func TestFetchingParameters(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test K3S provider")
}

const (
	apiServer   = "https://example.com"
	podCIDR     = "10.0.0.0/16"
	serviceCIDR = "10.80.0.0/16"
)

var _ = Describe("Extract elements from K3S", func() {

	It("test flags", func() {

		p := NewProvider().(*k3sProvider)

		cmd := &cobra.Command{}

		GenerateFlags(cmd)
		cmd.Flags().String("cluster-name", "", "")
		cmd.Flags().Bool("generate-name", true, "")
		cmd.Flags().String("reserved-subnets", "", "")

		flags := cmd.Flags()
		Expect(flags.Set("pod-cidr", podCIDR)).To(Succeed())
		Expect(flags.Set("service-cidr", serviceCIDR)).To(Succeed())
		Expect(flags.Set("api-server", apiServer)).To(Succeed())

		Expect(p.ValidateCommandArguments(flags)).To(Succeed())

		Expect(p.podCIDR).To(Equal(podCIDR))
		Expect(p.serviceCIDR).To(Equal(serviceCIDR))
		Expect(p.apiServer).To(Equal(apiServer))

		Expect(p.ClusterLabels).ToNot(BeEmpty())
		Expect(p.ClusterLabels[consts.ProviderClusterLabel]).To(Equal(providerPrefix))

	})

	Context("test api server validation", func() {

		type apiServerValidatorTestcase struct {
			apiServerAddress string
			expectedOutput   types.GomegaMatcher
		}

		DescribeTable("api server validation table",
			func(c apiServerValidatorTestcase) {
				p := NewProvider().(*k3sProvider)
				p.apiServer = c.apiServerAddress

				err := p.validateAPIServer()
				Expect(err).To(c.expectedOutput)
			},

			Entry("valid address", apiServerValidatorTestcase{
				apiServerAddress: apiServer,
				expectedOutput:   Succeed(),
			}),

			Entry("valid without https", apiServerValidatorTestcase{
				apiServerAddress: "example.com",
				expectedOutput:   Succeed(),
			}),

			Entry("invalid localhost address", apiServerValidatorTestcase{
				apiServerAddress: "http://localhost:6443",
				expectedOutput:   HaveOccurred(),
			}),

			Entry("invalid 127.0.0.1 address", apiServerValidatorTestcase{
				apiServerAddress: "https://127.0.0.1",
				expectedOutput:   HaveOccurred(),
			}),
		)

	})

	Context("test serviceCIDR validation", func() {

		type serviceCIDRValidatorTestcase struct {
			serviceList    []runtime.Object
			expectedOutput types.GomegaMatcher
		}

		DescribeTable("api server validation table",
			func(c serviceCIDRValidatorTestcase) {
				ctx := context.TODO()

				client := fake.NewSimpleClientset(c.serviceList...)

				p := NewProvider().(*k3sProvider)
				p.serviceCIDR = serviceCIDR
				p.k8sClient = client

				err := p.validateServiceCIDR(ctx)
				Expect(err).To(c.expectedOutput)
			},

			Entry("valid service CIDR", serviceCIDRValidatorTestcase{
				serviceList: []runtime.Object{
					getSvc("svc-1", "10.80.0.1"),
					getSvc("svc-2", "10.80.0.2"),
					getSvc("svc-3", "10.80.0.3"),
				},
				expectedOutput: Succeed(),
			}),

			Entry("valid service CIDR with None clusterIPs", serviceCIDRValidatorTestcase{
				serviceList: []runtime.Object{
					getSvc("svc-1", "10.80.0.1"),
					getSvc("svc-2", "10.80.0.2"),
					getSvc("svc-3", "10.80.0.3"),
					getSvc("svc-4", "None"),
					getSvc("svc-5", ""),
				},
				expectedOutput: Succeed(),
			}),

			Entry("invalid service CIDR", serviceCIDRValidatorTestcase{
				serviceList: []runtime.Object{
					getSvc("svc-1", "10.80.0.1"),
					getSvc("svc-2", "10.80.0.2"),
					getSvc("svc-3", "None"),
					getSvc("svc-4", ""),
					getSvc("svc-5", "10.81.0.3"),
				},
				expectedOutput: HaveOccurred(),
			}),
		)

	})

	Context("test podCIDR validation", func() {

		type podCIDRValidatorTestcase struct {
			podList        []runtime.Object
			expectedOutput types.GomegaMatcher
		}

		DescribeTable("api server validation table",
			func(c podCIDRValidatorTestcase) {
				ctx := context.TODO()

				client := fake.NewSimpleClientset(c.podList...)

				p := NewProvider().(*k3sProvider)
				p.podCIDR = podCIDR
				p.k8sClient = client

				err := p.validatePodCIDR(ctx)
				Expect(err).To(c.expectedOutput)
			},

			Entry("valid pod CIDR", podCIDRValidatorTestcase{
				podList: []runtime.Object{
					getPod("pod-1", "10.0.0.1", false, false),
					getPod("pod-2", "10.0.0.2", false, false),
					getPod("pod-3", "10.0.0.3", false, false),
				},
				expectedOutput: Succeed(),
			}),

			Entry("valid pod CIDR with hostNetwork pods", podCIDRValidatorTestcase{
				podList: []runtime.Object{
					getPod("pod-1", "10.0.0.1", false, false),
					getPod("pod-2", "10.0.0.2", false, false),
					getPod("pod-3", "192.168.10.30", true, false),
					getPod("pod-4", "", false, false),
				},
				expectedOutput: Succeed(),
			}),

			Entry("invalid pod CIDR", podCIDRValidatorTestcase{
				podList: []runtime.Object{
					getPod("pod-1", "10.0.0.1", false, false),
					getPod("pod-2", "10.0.0.2", false, false),
					getPod("pod-3", "192.168.10.30", true, false),
					getPod("pod-4", "", false, false),
					getPod("pod-5", "10.1.0.1", false, false),
				},
				expectedOutput: HaveOccurred(),
			}),

			Entry("valid pod CIDR with offloaded pods", podCIDRValidatorTestcase{
				podList: []runtime.Object{
					getPod("pod-1", "10.0.0.1", false, false),
					getPod("pod-2", "10.0.0.2", false, false),
					getPod("pod-3", "192.168.10.30", true, false),
					getPod("pod-4", "", false, false),
					getPod("pod-5", "10.1.0.1", false, true),
				},
				expectedOutput: Succeed(),
			}),
		)

	})

})

func getSvc(name, clusterIP string) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1.ServiceSpec{
			ClusterIP: clusterIP,
		},
	}
}

func getPod(name, ip string, hostNetwork, offloaded bool) *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1.PodSpec{
			HostNetwork: hostNetwork,
		},
		Status: v1.PodStatus{
			PodIP: ip,
		},
	}
	if offloaded {
		pod.Labels = map[string]string{
			consts.LocalPodLabelKey: "true",
		}
	}
	return pod
}
