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

package peer_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pterm/pterm"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/info"
	"github.com/liqotech/liqo/pkg/liqoctl/info/peer"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("InfoChecker tests", func() {
	var (
		ic      *peer.InfoChecker
		ctx     context.Context
		options info.Options
	)

	BeforeEach(func() {
		ctx = context.Background()

		options = info.Options{Factory: factory.NewForLocal()}
		options.Printer = output.NewFakePrinter(GinkgoWriter)
	})

	Describe("Testing the InfoChecker", func() {
		Context("Collecting and retrieving the data", func() {
			It("should collect the data and return the right result", func() {
				expectedClusterRole := liqov1beta1.ConsumerAndProviderRole
				expectedClusterID := liqov1beta1.ClusterID("cl01")

				fakeForeignCluster := testutil.FakeForeignCluster(expectedClusterID, &liqov1beta1.Modules{
					Networking:     liqov1beta1.Module{},
					Authentication: liqov1beta1.Module{},
					Offloading:     liqov1beta1.Module{},
				})
				fakeForeignCluster.Status.Role = expectedClusterRole
				options.ClustersInfo = map[liqov1beta1.ClusterID]*liqov1beta1.ForeignCluster{
					expectedClusterID: fakeForeignCluster,
				}

				By("Collecting the data")
				ic = &peer.InfoChecker{}
				ic.Collect(ctx, options)

				By("Verifying that no errors have been raised")
				Expect(ic.GetCollectionErrors()).To(BeEmpty(), "Unexpected collection errors detected")

				By("Checking the correctness of the data in the struct")
				rawData, err := ic.GetDataByClusterID(expectedClusterID)
				Expect(err).NotTo(HaveOccurred(), "An error occurred while getting the data")

				data := rawData.(peer.Info)

				Expect(data.ClusterID).To(Equal(expectedClusterID))
				Expect(data.Role).To(Equal(expectedClusterRole))

				By("Checking the formatted output")
				text := ic.FormatForClusterID(expectedClusterID, options)
				text = pterm.RemoveColorFromString(text)
				text = testutil.SqueezeWhitespaces(text)

				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Cluster ID: %s", expectedClusterID),
				))
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Role: %s", expectedClusterRole),
				))
			})
		})
	})
})
