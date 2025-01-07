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

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/info"
	"github.com/liqotech/liqo/pkg/liqoctl/info/common"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("AuthChecker tests", func() {
	remoteClusterID := "fake-remote"
	localClusterID := "fake-local"
	expectedResourceList := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("2"),
		corev1.ResourceMemory: resource.MustParse("2Gi"),
	}

	var (
		clientBuilder fake.ClientBuilder
		ac            *AuthChecker
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

	Describe("Testing the AuthChecker", func() {
		Context("Collecting and retrieving the data", func() {
			It("should collect the data and return the right result", func() {
				identityRes := testutil.FakeIdentity(liqov1beta1.ClusterID(remoteClusterID), authv1beta1.ControlPlaneIdentityType)
				resourceSlices := []client.Object{
					testutil.FakeResourceSlice("rs01", liqov1beta1.ClusterID(localClusterID), liqov1beta1.ClusterID(remoteClusterID),
						authv1beta1.ResourceSliceConditionAccepted, expectedResourceList),
					testutil.FakeResourceSlice("rs02", liqov1beta1.ClusterID(localClusterID), liqov1beta1.ClusterID(remoteClusterID),
						authv1beta1.ResourceSliceConditionDenied, expectedResourceList),
				}
				foreignclusterRes := testutil.FakeForeignCluster(liqov1beta1.ClusterID(remoteClusterID), &liqov1beta1.Modules{
					Networking:     liqov1beta1.Module{Enabled: true},
					Authentication: liqov1beta1.Module{Enabled: true},
					Offloading:     liqov1beta1.Module{Enabled: true},
				})
				foreignclusterRes.Status.Role = liqov1beta1.ProviderRole

				// Set up the fake clients
				resources := []client.Object{identityRes}
				resources = append(resources, resourceSlices...)
				clientBuilder.WithObjects(resources...)
				options.CRClient = clientBuilder.Build()

				options.ClustersInfo = map[liqov1beta1.ClusterID]*liqov1beta1.ForeignCluster{
					liqov1beta1.ClusterID(remoteClusterID): foreignclusterRes,
				}

				By("Collecting the data")
				ac = &AuthChecker{}
				ac.Collect(ctx, options)

				By("Verifying that no errors have been raised")
				Expect(ac.GetCollectionErrors()).To(BeEmpty(), "Unexpected collection errors detected")

				By("Checking the correctness of the data in the struct")
				rawData, err := ac.GetDataByClusterID(liqov1beta1.ClusterID(remoteClusterID))
				Expect(err).NotTo(HaveOccurred(), "An error occurred while getting the data")

				data := rawData.(Auth)
				Expect(data.Status).To(Equal(common.ModuleHealthy), "Unexpected status")

				Expect(data.APIServerAddr).To(Equal(identityRes.Spec.AuthParams.APIServer), "Unexpected API server address")

				// Check resource slices
				Expect(len(data.ResourceSlices)).To(Equal(len(resourceSlices)), "Unexpected number of resource slices")
				Expect(data.ResourceSlices[0].Accepted).To(BeTrue(), "First resource slice is expected to be accepted")
				Expect(data.ResourceSlices[1].Accepted).To(BeFalse(), "Second resource slice is expected to be rejected")
				for _, slice := range data.ResourceSlices {
					Expect(slice.Resources).To(Equal(expectedResourceList))
				}
			})

			It("tests ResourceSlice collection of data", func() {
				ac = &AuthChecker{}

				By("Checking that ResourceSlice status is corrently retrieved when one of the conditions is Denied")
				rs := testutil.FakeResourceSlice("rs01", liqov1beta1.ClusterID(localClusterID), liqov1beta1.ClusterID(remoteClusterID),
					authv1beta1.ResourceSliceConditionAccepted, expectedResourceList)

				// Add error to resource slice
				expectedAlert := "This is an error"
				rs.Status.Conditions = append(rs.Status.Conditions, authv1beta1.ResourceSliceCondition{
					Type:    authv1beta1.ResourceSliceConditionTypeAuthentication,
					Status:  authv1beta1.ResourceSliceConditionDenied,
					Message: expectedAlert,
				})

				authStatus := Auth{}
				ac.collectResourceSlices([]authv1beta1.ResourceSlice{*rs}, &authStatus)

				Expect(len(authStatus.ResourceSlices)).To(Equal(1), "One single resource slice expected")
				Expect(authStatus.ResourceSlices[0].Accepted).To(BeFalse(), "One condition denied, expected resource slice to be denied")
				Expect(authStatus.ResourceSlices[0].Action).To(Equal(ConsumingAction),
					"Unexpected ResourceSlice action: originated by the same cluster")
				Expect(authStatus.ResourceSlices[0].Alerts).To(ContainElements(expectedAlert), "Alert not found in ResourceSlice alerts")

				By("Checking that the checker detects a replicated ResourceSlice")
				rs = testutil.FakeResourceSlice("rs01", liqov1beta1.ClusterID(localClusterID), liqov1beta1.ClusterID(remoteClusterID),
					authv1beta1.ResourceSliceConditionAccepted, expectedResourceList)
				rs.ObjectMeta.Labels[consts.ReplicationStatusLabel] = "TRUE"

				authStatus = Auth{}
				ac.collectResourceSlices([]authv1beta1.ResourceSlice{*rs}, &authStatus)
				Expect(len(authStatus.ResourceSlices)).To(Equal(1), "One single resource slice expected")
				Expect(authStatus.ResourceSlices[0].Action).To(Equal(ProvidingAction),
					"Unexpected action: ResourceSlice has replication status label set to true")
			})
		})

		DescribeTable("FormatForClusterID function test", func(testCase Auth) {
			ac = &AuthChecker{}
			ac.data = map[liqov1beta1.ClusterID]Auth{
				liqov1beta1.ClusterID(remoteClusterID): testCase,
			}

			text := ac.FormatForClusterID(liqov1beta1.ClusterID(remoteClusterID), options)
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

				if testCase.APIServerAddr != "" {
					Expect(text).To(ContainSubstring(pterm.Sprintf("API server: %s", testCase.APIServerAddr)), "Unexpected API server")
				}

				// This test expects that all the ResourceSlice names starts with "rs-"
				if len(testCase.ResourceSlices) > 0 {
					outSections := strings.Split(text, "Resource slices")
					Expect(len(outSections)).To(Equal(2), "Invalid output: expected a section with name 'Resource slices'")
					rsSection := strings.TrimSpace(outSections[1])
					rsNamesRegex := regexp.MustCompile(`rs-\d+\s+`)
					rsSplittedSections := rsNamesRegex.Split(rsSection, -1)[1:]

					Expect(len(rsSplittedSections)).To(Equal(len(testCase.ResourceSlices)),
						"The number of ResourseSlices does not match the one shown in output")

					for i, rs := range testCase.ResourceSlices {
						sectionText := rsSplittedSections[i]

						// Check RS action
						Expect(sectionText).To(ContainSubstring(pterm.Sprintf("Action: %s", rs.Action)),
							fmt.Sprintf("Unexpected action in RS section %q", rs.Name))

						// Check that the status of the ResourceSlice is correctly shown
						statusCondition := ContainSubstring("Resource slice not accepted")
						if rs.Accepted {
							statusCondition = ContainSubstring("Resource slice accepted")
						}
						Expect(sectionText).To(statusCondition, fmt.Sprintf("Unexpected status in RS section %q", rs.Name))

						// Check that all the resources are correctly shown
						for resName, resValue := range rs.Resources {
							resString := fmt.Sprintf("%s: %s", resName, &resValue)
							Expect(sectionText).To(ContainSubstring(resString),
								fmt.Sprintf("Unexpected resources shown in RS section %q", rs.Name))
						}
					}
				}
			}
		},
			Entry("Disabled module", Auth{Status: common.ModuleDisabled}),
			Entry("Healthy module", Auth{
				Status:        common.ModuleHealthy,
				APIServerAddr: "http://192.166.0.5",
				ResourceSlices: []ResourceSliceStatus{
					{
						Name:      "rs-01",
						Action:    ConsumingAction,
						Accepted:  false,
						Alerts:    []string{"rs01 error"},
						Resources: expectedResourceList,
					},
					{
						Name:      "rs-02",
						Action:    ProvidingAction,
						Accepted:  true,
						Resources: expectedResourceList,
					},
				},
			}),
			Entry("Unhealthy module", Auth{
				Status: common.ModuleUnhealthy,
				Alerts: []string{"This is an error"},
			}),
		)

	})
})
