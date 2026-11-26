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

package openshift

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1api "github.com/openshift/api/config/v1"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/install"
)

func TestFetchingParameters(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test OpenShift provider")
}

var _ = Describe("Extract elements from OpenShift", func() {
	var options Options

	BeforeEach(func() {
		options = Options{Options: &install.Options{}}
	})

	It("test parse values", func() {
		const (
			podCidr     = "10.128.0.0/14"
			serviceCidr = "172.30.0.0/16"
		)

		networkConfig := &configv1api.Network{
			Status: configv1api.NetworkStatus{
				ClusterNetwork: []configv1api.ClusterNetworkEntry{{CIDR: podCidr}},
				ServiceNetwork: []string{serviceCidr},
			},
		}

		Expect(options.parseNetworkConfig(networkConfig)).To(Succeed())
		Expect(options.PodCIDRs).To(Equal([]string{podCidr}))
		Expect(options.ServiceCIDR).To(Equal(serviceCidr))
	})

	It("should reject multiple cluster networks", func() {
		networkConfig := &configv1api.Network{
			Status: configv1api.NetworkStatus{
				ClusterNetwork: []configv1api.ClusterNetworkEntry{{CIDR: "10.128.0.0/14"}, {CIDR: "10.132.0.0/14"}},
				ServiceNetwork: []string{"172.30.0.0/16"},
			},
		}

		Expect(options.parseNetworkConfig(networkConfig)).To(HaveOccurred())
	})
})

var _ = Describe("Render the chart for OpenShift", func() {
	It("should label the gateway SCC as a gateway resource", func() {
		chart, err := loader.LoadDir(filepath.Join("..", "..", "..", "..", "deployments", "liqo"))
		Expect(err).ToNot(HaveOccurred())

		values := New(&install.Options{}).Values()
		values["tag"] = "v1.2.0"
		release := chartutil.ReleaseOptions{Name: install.LiqoReleaseName, Namespace: consts.DefaultLiqoNamespace}
		renderValues, err := chartutil.ToRenderValues(chart, values, release, nil)
		Expect(err).ToNot(HaveOccurred())

		manifests, err := engine.Render(chart, renderValues)
		Expect(err).ToNot(HaveOccurred())

		var scc metav1.PartialObjectMetadata
		Expect(yaml.Unmarshal([]byte(manifests["liqo/templates/liqo-gateway-scc.yaml"]), &scc)).To(Succeed())

		Expect(scc.Labels).To(HaveKeyWithValue(consts.K8sAppNameKey, "gateway"))
		Expect(scc.Labels).To(HaveKeyWithValue(consts.K8sAppInstanceKey, "liqo-gateway"))
		Expect(scc.Labels).To(HaveKeyWithValue(consts.K8sAppComponentKey, "networking"))
	})
})
