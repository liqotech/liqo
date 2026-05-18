// Copyright 2019-2026 The Liqo Authors
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

package install

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
)

const (
	apiServer   = "https://example.com"
	podCIDR     = "10.0.0.0/16"
	serviceCIDR = "10.80.0.0/16"
)

var _ = Describe("Validation", func() {
	var (
		options Options
		ctx     context.Context
	)

	BeforeEach(func() {
		options = Options{
			CommonOptions: &CommonOptions{
				Factory: &factory.Factory{},
			},
		}
		ctx = context.Background()
	})

	Context("API server endpoint", func() {
		type apiServerValidatorTestcase struct {
			apiServerAddress string
			expectedOutput   types.GomegaMatcher
		}

		DescribeTable("API server validation table",
			func(c apiServerValidatorTestcase) {
				options.APIServer = c.apiServerAddress
				options.DisableAPIServerSanityChecks = true

				err := options.validateAPIServer()
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

			Entry("invalid 0.0.0.0 address", apiServerValidatorTestcase{
				apiServerAddress: "https://0.0.0.0",
				expectedOutput:   HaveOccurred(),
			}),
		)

		type checkEndpointTestcase struct {
			endpoint string
			config   *rest.Config
			expected types.GomegaMatcher
		}

		DescribeTable("API server consistency table",
			func(c checkEndpointTestcase) {
				options.RESTConfig = c.config
				options.APIServer = c.endpoint
				Expect(options.validateAPIServerConsistency()).To(c.expected)
			},

			Entry("protocol and no port, equal", checkEndpointTestcase{
				endpoint: "https://example.com",
				config: &rest.Config{
					Host: "https://example.com",
				},
				expected: Succeed(),
			}),

			Entry("protocol and no port, not equal", checkEndpointTestcase{
				endpoint: "https://example.com",
				config: &rest.Config{
					Host: "https://example2.com",
				},
				expected: Not(Succeed()),
			}),

			Entry("protocol and port, equal", checkEndpointTestcase{
				endpoint: "https://example.com:1234",
				config: &rest.Config{
					Host: "https://example.com:1234",
				},
				expected: Succeed(),
			}),

			Entry("protocol and port, not equal", checkEndpointTestcase{
				endpoint: "https://example.com:1234",
				config: &rest.Config{
					Host: "https://example2.com:1234",
				},
				expected: Not(Succeed()),
			}),

			Entry("protocol and port, not equal (different port)", checkEndpointTestcase{
				endpoint: "https://example.com:1234",
				config: &rest.Config{
					Host: "https://example.com:123",
				},
				expected: Not(Succeed()),
			}),

			Entry("protocol and port, not equal (different port)", checkEndpointTestcase{
				endpoint: "https://example.com:1234",
				config: &rest.Config{
					Host: "https://example.com",
				},
				expected: Not(Succeed()),
			}),

			Entry("protocol and port, equal", checkEndpointTestcase{
				endpoint: "https://example.com:443",
				config: &rest.Config{
					Host: "https://example.com",
				},
				expected: Succeed(),
			}),

			Entry("no protocol and port, equal", checkEndpointTestcase{
				endpoint: "example.com:443",
				config: &rest.Config{
					Host: "example.com",
				},
				expected: Succeed(),
			}),

			Entry("no protocol and port, equal", checkEndpointTestcase{
				endpoint: "example.com:443",
				config: &rest.Config{
					Host: "https://example.com",
				},
				expected: Succeed(),
			}),

			Entry("no protocol and port, equal", checkEndpointTestcase{
				endpoint: "example.com",
				config: &rest.Config{
					Host: "https://example.com:443",
				},
				expected: Succeed(),
			}),
		)

	})

	Context("Service CIDR", func() {

		type serviceCIDRValidatorTestcase struct {
			serviceList    []runtime.Object
			expectedOutput types.GomegaMatcher
		}

		DescribeTable("Service CIDR validation table",
			func(c serviceCIDRValidatorTestcase) {
				options.ServiceCIDR = serviceCIDR
				options.KubeClient = fake.NewClientset(c.serviceList...)

				err := options.validateServiceCIDR(ctx)
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

	Context("Pod CIDR", func() {

		type podCIDRValidatorTestcase struct {
			podCIDRs       []string
			serviceCIDR    string
			externalCIDR   string
			reservedCIDRs  []string
			podList        []runtime.Object
			expectedOutput types.GomegaMatcher
		}

		DescribeTable("Pod CIDR validation table",
			func(c podCIDRValidatorTestcase) {
				options.PodCIDRs = c.podCIDRs
				if len(options.PodCIDRs) == 0 {
					options.PodCIDRs = []string{podCIDR}
				}
				options.ServiceCIDR = c.serviceCIDR
				options.ExternalCIDR = c.externalCIDR
				options.ReservedSubnets = c.reservedCIDRs
				options.KubeClient = fake.NewClientset(c.podList...)

				err := options.validatePodCIDRs(ctx)
				Expect(err).To(c.expectedOutput)
			},

			Entry("valid pod CIDR", podCIDRValidatorTestcase{
				podCIDRs:    []string{podCIDR},
				serviceCIDR: serviceCIDR,
				podList: []runtime.Object{
					getPod("pod-1", "10.0.0.1", false, false),
					getPod("pod-2", "10.0.0.2", false, false),
					getPod("pod-3", "10.0.0.3", false, false),
				},
				expectedOutput: Succeed(),
			}),

			Entry("valid pod CIDR with hostNetwork pods", podCIDRValidatorTestcase{
				podCIDRs:    []string{podCIDR},
				serviceCIDR: serviceCIDR,
				podList: []runtime.Object{
					getPod("pod-1", "10.0.0.1", false, false),
					getPod("pod-2", "10.0.0.2", false, false),
					getPod("pod-3", "192.168.10.30", true, false),
					getPod("pod-4", "", false, false),
				},
				expectedOutput: Succeed(),
			}),

			Entry("invalid pod CIDR", podCIDRValidatorTestcase{
				podCIDRs:    []string{podCIDR},
				serviceCIDR: serviceCIDR,
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
				podCIDRs:    []string{podCIDR},
				serviceCIDR: serviceCIDR,
				podList: []runtime.Object{
					getPod("pod-1", "10.0.0.1", false, false),
					getPod("pod-2", "10.0.0.2", false, false),
					getPod("pod-3", "192.168.10.30", true, false),
					getPod("pod-4", "", false, false),
					getPod("pod-5", "10.1.0.1", false, true),
				},
				expectedOutput: Succeed(),
			}),

			Entry("valid multiple pod CIDRs", podCIDRValidatorTestcase{
				podCIDRs:    []string{"10.0.0.0/16", "10.1.0.0/16"},
				serviceCIDR: serviceCIDR,
				podList: []runtime.Object{
					getPod("pod-1", "10.0.0.1", false, false),
					getPod("pod-2", "10.1.0.2", false, false),
				},
				expectedOutput: Succeed(),
			}),

			Entry("reject pod CIDR overlapping service CIDR", podCIDRValidatorTestcase{
				podCIDRs:       []string{"10.0.0.0/16"},
				serviceCIDR:    "10.0.1.0/24",
				expectedOutput: HaveOccurred(),
			}),

			Entry("reject pod CIDR overlapping external CIDR", podCIDRValidatorTestcase{
				podCIDRs:       []string{"10.0.0.0/16"},
				externalCIDR:   "10.0.2.0/24",
				expectedOutput: HaveOccurred(),
			}),
		)

		It("should reject overlapping pod CIDRs", func() {
			options.PodCIDRs = []string{"10.0.0.0/16", "10.0.1.0/24"}
			options.KubeClient = fake.NewClientset()

			Expect(options.validatePodCIDRs(ctx)).To(HaveOccurred())
		})
	})
})

func getSvc(name, clusterIP string) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1.ServiceSpec{ClusterIP: clusterIP},
	}
}

func getPod(name, ip string, hostNetwork, offloaded bool) *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1.PodSpec{HostNetwork: hostNetwork},
		Status:     v1.PodStatus{PodIP: ip},
	}

	if offloaded {
		pod.Labels = map[string]string{consts.LocalPodLabelKey: "true"}
	}
	return pod
}
