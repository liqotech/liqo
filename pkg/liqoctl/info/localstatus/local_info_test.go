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
	"fmt"
	"strings"

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
	liqoctlutils "github.com/liqotech/liqo/pkg/liqoctl/utils"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("InstallationChecker tests", func() {
	const (
		clusterID = "fake"
	)

	var (
		clientBuilder     fake.ClientBuilder
		ic                *localstatus.InstallationChecker
		ctx               context.Context
		options           info.Options
		argsClusterLabels []string
		baseObjects       = []client.Object{
			testutil.FakeNode(),
			testutil.FakeClusterIDConfigMap(liqoconsts.DefaultLiqoNamespace, clusterID),
		}
	)

	BeforeEach(func() {
		ctx = context.Background()
		clientBuilder = *fake.NewClientBuilder().WithScheme(scheme.Scheme)
		for k, v := range testutil.ClusterLabels {
			argsClusterLabels = append(argsClusterLabels, fmt.Sprintf("%s=%s", k, v))
		}

		options = info.Options{Factory: factory.NewForLocal()}
		options.Printer = output.NewFakePrinter(GinkgoWriter)
		options.KubeClient = k8sfake.NewSimpleClientset()
	})

	Describe("Testing the InstallationChecker", func() {
		Context("Collecting and retrieving the data", func() {
			It("should collect the data and return the right result", func() {
				objects := append([]client.Object{}, baseObjects...)

				fakectrlman := testutil.FakeControllerManagerDeployment(argsClusterLabels, true)
				objects = append(objects, fakectrlman)

				// Set up the fake clients
				clientBuilder.WithObjects(objects...)
				options.CRClient = clientBuilder.Build()
				options.LiqoNamespace = liqoconsts.DefaultLiqoNamespace

				// The configmap is retrieved with the native client, so mock the kubeclient
				options.KubeClient = k8sfake.NewSimpleClientset(baseObjects[1])

				By("Collecting the data")
				ic = &localstatus.InstallationChecker{}
				ic.Collect(ctx, options)

				By("Verifying that no errors have been raised")
				Expect(ic.GetCollectionErrors()).To(BeEmpty())

				By("Checking the correctness of the data in the struct")
				data := ic.GetData().(localstatus.Installation)

				Expect(string(data.ClusterID)).To(Equal(clusterID))
				Expect(data.Version).To(Equal(testutil.FakeLiqoVersion))
				Expect(data.APIServerAddr).To(Equal(fmt.Sprintf("https://%v:6443", testutil.EndpointIP)))

				// Check if the cluster labels are the expected ones
				labels, _ := liqoctlutils.ParseArgsMultipleValues(strings.Join(argsClusterLabels, ","), ",")
				Expect(data.Labels).To(Equal(labels))

				By("Checking the formatted output")
				text := ic.Format(options)
				text = pterm.RemoveColorFromString(text)
				text = testutil.SqueezeWhitespaces(text)

				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Cluster ID: %s", clusterID),
				))
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Version: %s", testutil.FakeLiqoVersion),
				))

				for _, v := range testutil.ClusterLabels {
					Expect(text).To(ContainSubstring(v))
				}
			})
		})
	})
})
