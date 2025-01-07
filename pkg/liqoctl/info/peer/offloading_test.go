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

package peer

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/info"
	"github.com/liqotech/liqo/pkg/liqoctl/info/common"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("OffloadingChecker tests", func() {
	clusterID := "fake-clusterid"
	expectedResourceList := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("2"),
		corev1.ResourceMemory: resource.MustParse("2Gi"),
	}

	var (
		clientBuilder fake.ClientBuilder
		oc            *OffloadingChecker
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

	Describe("Testing the OffloadingChecker", func() {
		Context("Collecting and retrieving the data", func() {
			It("should collect the data and return the right result", func() {
				virtualNodes := []client.Object{
					testutil.FakeVirtualNode("vn01", liqov1beta1.ClusterID(clusterID),
						offloadingv1beta1.RunningConditionStatusType, expectedResourceList),
					testutil.FakeVirtualNode("vn02", liqov1beta1.ClusterID(clusterID),
						offloadingv1beta1.CreatingConditionStatusType, expectedResourceList),
				}

				foreignclusterRes := testutil.FakeForeignCluster(liqov1beta1.ClusterID(clusterID), &liqov1beta1.Modules{
					Networking:     liqov1beta1.Module{Enabled: true},
					Authentication: liqov1beta1.Module{Enabled: true},
					Offloading:     liqov1beta1.Module{Enabled: true},
				})
				foreignclusterRes.Status.Role = liqov1beta1.ProviderRole

				clientBuilder.WithObjects(virtualNodes...)
				options.CRClient = clientBuilder.Build()

				options.ClustersInfo = map[liqov1beta1.ClusterID]*liqov1beta1.ForeignCluster{
					liqov1beta1.ClusterID(clusterID): foreignclusterRes,
				}

				By("Collecting the data")
				oc = &OffloadingChecker{}
				oc.Collect(ctx, options)

				By("Verifying that no errors have been raised")
				Expect(oc.GetCollectionErrors()).To(BeEmpty(), "Unexpected collection errors detected")

				By("Checking the correctness of the data in the struct")
				rawData, err := oc.GetDataByClusterID(liqov1beta1.ClusterID(clusterID))
				Expect(err).NotTo(HaveOccurred(), "An error occurred while getting the data")

				data := rawData.(Offloading)
				Expect(data.Status).To(Equal(common.ModuleHealthy), "Unexpected status")

				// Check the correctness of VirtualNodes data
				Expect(len(data.VirtualNodes)).To(Equal(len(virtualNodes)), "Unexpected number of VirtualNodes")
				Expect(data.VirtualNodes[0].Status).To(Equal(common.ModuleHealthy), "First VirtualNode is expected to be ready")
				Expect(data.VirtualNodes[1].Status).To(Equal(common.ModuleUnhealthy), "Second VirtualNode is expected to be NOT ready")

				for i, vnStatus := range data.VirtualNodes {
					virtualNode := virtualNodes[i].(*offloadingv1beta1.VirtualNode)
					Expect(vnStatus.ResourceSlice).To(Equal(virtualNode.Name), "Unexpected VirtualNode ResourceSlice name")
					Expect(vnStatus.Secret).To(Equal(fmt.Sprintf("%s-secret", virtualNode.Name)), "Unexpected VirtualNode secret name")
					Expect(vnStatus.Resources).To(Equal(expectedResourceList), "Unexpected resources")
				}
			})

			It("tests ResourceSlice collection of data", func() {
				oc = &OffloadingChecker{}

				By("Checking that ResourceSlice status is corrently retrieved when one of the conditions is Denied")
				vn := testutil.FakeVirtualNode("vn01", liqov1beta1.ClusterID(clusterID),
					offloadingv1beta1.RunningConditionStatusType, expectedResourceList)

				// Add error to virtual node
				vn.Status.Conditions = append(vn.Status.Conditions, offloadingv1beta1.VirtualNodeCondition{
					Status: offloadingv1beta1.CreatingConditionStatusType,
				})
				offloadingStatus := Offloading{}
				oc.collectVirtualNodes([]offloadingv1beta1.VirtualNode{*vn}, &offloadingStatus)

				Expect(len(offloadingStatus.VirtualNodes)).To(Equal(1), "One single virtual node expected")
				Expect(offloadingStatus.VirtualNodes[0].Status).To(Equal(common.ModuleUnhealthy),
					"One condition unhealthy, expected virtual node to be unhealthy")
			})
		})

		DescribeTable("FormatForClusterID function test", func(testCase Offloading) {
			oc = &OffloadingChecker{}
			oc.data = map[liqov1beta1.ClusterID]Offloading{
				liqov1beta1.ClusterID(clusterID): testCase,
			}

			text := oc.FormatForClusterID(liqov1beta1.ClusterID(clusterID), options)
			text = pterm.RemoveColorFromString(text)
			text = strings.TrimSpace(testutil.SqueezeWhitespaces(text))

			if testCase.Status == common.ModuleDisabled {
				Expect(text).To(Equal("Status: Disabled"))
			} else {
				Expect(text).To(ContainSubstring(
					pterm.Sprintf("Status: %s", testCase.Status),
				), "Unexpected status shown")

				// Check that alerts are shown when list is not empty
				for _, alert := range testCase.Alerts {
					Expect(text).To(ContainSubstring(alert), "Alert not shown")
				}

				// This test expects that all the ResourceSlice names starts with "rs-"
				if len(testCase.VirtualNodes) > 0 {
					outSections := strings.Split(text, "Virtual nodes")
					Expect(len(outSections)).To(Equal(2), "Invalid output: expected a section with name 'Resource slices'")
					rsSection := strings.TrimSpace(outSections[1])
					rsNamesRegex := regexp.MustCompile(`vn-\d+\s+`)
					rsSplittedSections := rsNamesRegex.Split(rsSection, -1)[1:]

					Expect(len(rsSplittedSections)).To(Equal(len(testCase.VirtualNodes)),
						"The number of ResourseSlices does not match the one shown in output")

					for i, vn := range testCase.VirtualNodes {
						sectionText := rsSplittedSections[i]

						// Check RS action
						Expect(sectionText).To(ContainSubstring(pterm.Sprintf("Status: %s", vn.Status)),
							fmt.Sprintf("Unexpected health in VirtualNode section %q", vn.Name))

						Expect(sectionText).To(ContainSubstring(pterm.Sprintf("Secret: %s", vn.Secret)),
							fmt.Sprintf("Unexpected secret in VirtualNode section %q", vn.Name))

						Expect(sectionText).To(ContainSubstring(pterm.Sprintf("Resource slice: %s", vn.ResourceSlice)),
							fmt.Sprintf("Unexpected ResourceSlice in VirtualNode section %q", vn.Name))

						// Check that all the resources are correctly shown
						for resName, resValue := range vn.Resources {
							resString := fmt.Sprintf("%s: %s", resName, &resValue)
							Expect(sectionText).To(ContainSubstring(resString),
								fmt.Sprintf("Unexpected resources shown in RS section %q", vn.Name))
						}
					}
				}
			}
		},
			Entry("Disabled module", Offloading{Status: common.ModuleDisabled}),
			Entry("Healthy module", Offloading{
				Status: common.ModuleHealthy,
				VirtualNodes: []VirtualNodeStatus{
					{
						Name:          "vn-01",
						Status:        common.ModuleHealthy,
						Secret:        "secret",
						ResourceSlice: "slice",
						Resources:     expectedResourceList,
					},
					{
						Name:          "vn-02",
						Status:        common.ModuleUnhealthy,
						Secret:        "secret",
						ResourceSlice: "slice",
						Resources:     expectedResourceList,
					},
				},
			}),
			Entry("Unhealthy module", Offloading{
				Status: common.ModuleUnhealthy,
				Alerts: []string{"This is an error"},
			}),
		)

	})
})
