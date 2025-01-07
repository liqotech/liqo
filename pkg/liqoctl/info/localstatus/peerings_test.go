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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pterm/pterm"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/info"
	"github.com/liqotech/liqo/pkg/liqoctl/info/common"
	"github.com/liqotech/liqo/pkg/liqoctl/info/localstatus"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("PeeringChecker tests", func() {

	getFakeModuleFromStatus := func(status common.ModuleStatus) liqov1beta1.Module {
		var conditions []liqov1beta1.Condition
		if status == common.ModuleDisabled {
			return liqov1beta1.Module{
				Enabled: false,
			}
		} else if status == common.ModuleHealthy {
			conditions = append(conditions,
				liqov1beta1.Condition{
					Type:   "fake1",
					Status: liqov1beta1.ConditionStatusEstablished,
				},
				liqov1beta1.Condition{
					Type:   "fake2",
					Status: liqov1beta1.ConditionStatusReady,
				},
			)
		} else {
			conditions = append(conditions,
				liqov1beta1.Condition{
					Type:   "fake1",
					Status: liqov1beta1.ConditionStatusError,
				},
			)
		}

		return liqov1beta1.Module{
			Enabled:    true,
			Conditions: conditions,
		}
	}

	getFakeForeignClusterID := func(n int) string {
		return fmt.Sprintf("cluster-%d", n)
	}

	type ForeignClusterDescription struct {
		Authentication common.ModuleStatus
		Networking     common.ModuleStatus
		Offloading     common.ModuleStatus
	}

	type TestArgs struct {
		foreignClusterDescriptions []ForeignClusterDescription
	}

	var (
		clientBuilder fake.ClientBuilder
		pc            *localstatus.PeeringChecker
		ctx           context.Context
		options       info.Options
	)

	BeforeEach(func() {
		ctx = context.Background()
		clientBuilder = *fake.NewClientBuilder().WithScheme(scheme.Scheme)

		options = info.Options{Factory: factory.NewForLocal()}
		options.Printer = output.NewFakePrinter(GinkgoWriter)
		options.KubeClient = k8sfake.NewSimpleClientset()
	})

	Describe("Testing the PeeringChecker", func() {
		Context("Collecting and retrieving the data", func() {
			DescribeTable("should collect the data and return the right result", func(args TestArgs) {
				// Set up the fake foreign clusters
				var clustersList []client.Object

				for i, d := range args.foreignClusterDescriptions {
					clustersList = append(clustersList, testutil.FakeForeignCluster(
						liqov1beta1.ClusterID(getFakeForeignClusterID(i)),
						&liqov1beta1.Modules{
							Authentication: getFakeModuleFromStatus(d.Authentication),
							Networking:     getFakeModuleFromStatus(d.Networking),
							Offloading:     getFakeModuleFromStatus(d.Offloading),
						},
					))
				}

				clientBuilder.WithObjects(clustersList...)
				options.CRClient = clientBuilder.Build()
				options.LiqoNamespace = liqoconsts.DefaultLiqoNamespace

				By("Collecting the data")
				pc = &localstatus.PeeringChecker{}
				pc.Collect(ctx, options)

				By("Verifying that no errors have been raised")
				Expect(pc.GetCollectionErrors()).To(BeEmpty())

				By("Checking the correctness of the data in the struct")
				data := pc.GetData().(localstatus.Peerings)

				// Checking that the number of active peerings match the number of ForeignCluster resources
				Expect(len(data.Peers)).To(Equal(len(args.foreignClusterDescriptions)),
					"The number of active peerings reported is not the expected one",
				)

				for i, clusterInfo := range data.Peers {
					expectedClusterRes := &args.foreignClusterDescriptions[i]
					Expect(clusterInfo.ClusterID).To(Equal(liqov1beta1.ClusterID(getFakeForeignClusterID(i))),
						"Unexpected ClusterID",
					)
					Expect(clusterInfo.AuthenticationStatus).To(Equal(expectedClusterRes.Authentication))
					Expect(clusterInfo.NetworkingStatus).To(Equal(expectedClusterRes.Networking))
					Expect(clusterInfo.OffloadingStatus).To(Equal(expectedClusterRes.Offloading))
				}

				By("Checking the formatted output")
				text := pc.Format(options)
				text = pterm.RemoveColorFromString(text)
				text = testutil.SqueezeWhitespaces(text)
				for i, clusterInfo := range data.Peers {
					Expect(text).To(ContainSubstring(
						pterm.Sprintf(
							"%s Role: Unknown Networking status: %v Authentication status: %v Offloading status: %v",
							getFakeForeignClusterID(i),
							clusterInfo.NetworkingStatus,
							clusterInfo.AuthenticationStatus,
							clusterInfo.OffloadingStatus,
						),
					))
				}
			},
				Entry("Healthy 1 peering", TestArgs{
					foreignClusterDescriptions: []ForeignClusterDescription{
						{
							Authentication: common.ModuleHealthy,
							Networking:     common.ModuleHealthy,
							Offloading:     common.ModuleHealthy,
						},
					},
				}),
				Entry("Healthy and Unhealthy peerings", TestArgs{
					foreignClusterDescriptions: []ForeignClusterDescription{
						{
							Authentication: common.ModuleHealthy,
							Networking:     common.ModuleHealthy,
							Offloading:     common.ModuleHealthy,
						},
						{
							Authentication: common.ModuleHealthy,
							Networking:     common.ModuleUnhealthy,
							Offloading:     common.ModuleHealthy,
						},
					},
				}),
				Entry("Unhealthy peering with disable module", TestArgs{
					foreignClusterDescriptions: []ForeignClusterDescription{
						{
							Authentication: common.ModuleHealthy,
							Networking:     common.ModuleDisabled,
							Offloading:     common.ModuleUnhealthy,
						},
					},
				}),
			)
		})
	})
})
