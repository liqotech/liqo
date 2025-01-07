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

package resourceenforcement

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/test/e2e/testutils/config"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 2
	// testName is the name of this E2E test.
	testName = "RESOURCE_ENFORCEMENT"
)

func TestE2E(t *testing.T) {
	util.CheckIfTestIsSkipped(t, clustersRequired, testName)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

var (
	ctx                 = context.Background()
	testContext         = tester.GetTester(ctx)
	interval            = config.Interval
	timeout             = config.Timeout
	consistentlyTimeout = config.TimeoutConsistently
	namespaceName       = util.GetNameNamespaceTest(testName)

	sliceName       = "resourceenforcement"
	tenantNamespace = ""

	wakeUpResourceSlice = func() {
		var slices authv1beta1.ResourceSliceList
		Expect(testContext.Clusters[0].ControllerClient.List(ctx, &slices)).To(Succeed())

		for i := range slices.Items {
			slice := &slices.Items[i]
			if slice.Labels == nil {
				slice.Labels = make(map[string]string)
			}
			slice.Labels["update"] = fmt.Sprintf("%d", time.Now().UnixMilli())
			Expect(testContext.Clusters[0].ControllerClient.Update(ctx, slice)).To(Succeed())
		}
	}

	activateTenants = func() {
		for i := range testContext.Clusters {
			if testContext.Clusters[i].Role == liqov1beta1.ProviderRole {
				Expect(util.ActivateTenants(ctx, testContext.Clusters[i].ControllerClient)).To(Succeed())
			}
		}

		time.Sleep(2 * time.Second)
	}
	cordonTenants = func() {
		for i := range testContext.Clusters {
			if testContext.Clusters[i].Role == liqov1beta1.ProviderRole {
				Expect(util.CordonTenants(ctx, testContext.Clusters[i].ControllerClient)).To(Succeed())
			}
		}

		time.Sleep(2 * time.Second)
	}
	drainTenants = func() {
		for i := range testContext.Clusters {
			if testContext.Clusters[i].Role == liqov1beta1.ProviderRole {
				Expect(util.DrainTenants(ctx, testContext.Clusters[i].ControllerClient)).To(Succeed())
			}
		}

		time.Sleep(2 * time.Second)
	}
)

var _ = Describe("Liqo E2E", func() {
	Context("Resource Enforcement", func() {

		var (
			deploymentName = "nginx"

			createDeployment = func() {
				Expect(util.EnforceDeployment(ctx,
					testContext.Clusters[0].ControllerClient,
					namespaceName,
					deploymentName,
					util.RemoteDeploymentOption(),
				)).To(Succeed())
			}
			deleteDeployment = func() {
				Expect(util.EnsureDeploymentDeletion(ctx, testContext.Clusters[0].ControllerClient, namespaceName, deploymentName)).To(Succeed())
				Eventually(func() error {
					var d appsv1.Deployment
					return testContext.Clusters[0].ControllerClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: deploymentName}, &d)
				}, timeout, interval).ShouldNot(Succeed())
			}
		)

		BeforeEach(func() {
			activateTenants()

			// ensure the namespace is created
			for i := range testContext.Clusters {
				if testContext.Clusters[i].Role == liqov1beta1.ConsumerRole {
					// ensure the namespace is created
					Expect(util.Second(util.EnforceNamespace(ctx, testContext.Clusters[i].NativeClient,
						testContext.Clusters[i].Cluster, namespaceName))).To(Succeed())

					Eventually(util.OffloadNamespace(testContext.Clusters[i].KubeconfigPath, namespaceName),
						timeout, interval).Should(Succeed())

					// wait for the namespace to be offloaded, this avoids race conditions
					time.Sleep(2 * time.Second)
				}
			}
		})

		When("The Tenant is active", func() {

			BeforeEach(func() {
				activateTenants()
			})

			It("Should offload the pods to remote clusters", func() {
				// create deployment
				createDeployment()

				// wait for the deployment to be offloaded
				Eventually(func() int32 {
					var d appsv1.Deployment
					if err := testContext.Clusters[0].ControllerClient.Get(ctx,
						client.ObjectKey{Namespace: namespaceName, Name: deploymentName}, &d); err != nil {
						return 0
					}

					return d.Status.AvailableReplicas
				}, timeout, interval).Should(BeNumerically("==", 1))

				pods, err := util.GetPodsFromDeployment(ctx, testContext.Clusters[0].ControllerClient, namespaceName, deploymentName)
				Expect(err).ToNot(HaveOccurred())
				Expect(pods).To(HaveLen(1))
				Expect(pods[0].Spec.NodeName).ToNot(BeEmpty())
				Expect(pods[0].Status.Phase).To(Equal(corev1.PodRunning))

				// check the pod is running on the remote cluster
				nodeName := pods[0].Spec.NodeName
				var node corev1.Node
				Expect(testContext.Clusters[0].ControllerClient.Get(ctx, client.ObjectKey{Name: nodeName}, &node)).To(Succeed())
				Expect(node.Labels).To(HaveKey(consts.TypeLabel))

				// clean up the deployment
				deleteDeployment()
			})

			It("Should drain the pods when the Tenant is drained", func() {
				// create deployment
				createDeployment()

				// wait for the deployment to be offloaded
				Eventually(func() int32 {
					var d appsv1.Deployment
					if err := testContext.Clusters[0].ControllerClient.Get(ctx,
						client.ObjectKey{Namespace: namespaceName, Name: deploymentName}, &d); err != nil {
						return 0
					}

					return d.Status.AvailableReplicas
				}, timeout, interval).Should(BeNumerically("==", 1))

				// drain the tenant
				drainTenants()

				defer func() {
					time.Sleep(2 * time.Second)

					// wake up the resource slice to avoid next tests to fail
					wakeUpResourceSlice()
				}()

				// wait for the deployment to be drained
				Eventually(func() int32 {
					var d appsv1.Deployment
					if err := testContext.Clusters[0].ControllerClient.Get(ctx,
						client.ObjectKey{Namespace: namespaceName, Name: deploymentName}, &d); err != nil {
						return 0
					}

					return d.Status.AvailableReplicas
				}, timeout, interval).Should(BeNumerically("==", 0))

				pods, err := util.GetPodsFromDeployment(ctx, testContext.Clusters[0].ControllerClient, namespaceName, deploymentName)
				Expect(err).ToNot(HaveOccurred())
				Expect(pods).To(HaveLen(2))
				Expect(pods).To(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Status": MatchFields(IgnoreExtras, Fields{
						"Reason": Equal(string(forge.PodOffloadingBackOffReason)),
						"Phase":  Equal(corev1.PodPending),
					}),
				})))
				Expect(pods).To(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Status": MatchFields(IgnoreExtras, Fields{
						"Reason": Equal(string(forge.PodOffloadingAbortedReason)),
						"Phase":  Equal(corev1.PodFailed),
					}),
				})))

				// clean up the deployment
				deleteDeployment()
			})

		})

		When("The Tenant is cordoned", func() {

			BeforeEach(func() {
				cordonTenants()
			})

			It("Should not offload the pods to remote clusters", func() {
				// create deployment
				createDeployment()

				// no pods should be available
				Consistently(func() int32 {
					var d appsv1.Deployment
					if err := testContext.Clusters[0].ControllerClient.Get(ctx,
						client.ObjectKey{Namespace: namespaceName, Name: deploymentName}, &d); err != nil {
						return 0
					}

					return d.Status.AvailableReplicas
				}, consistentlyTimeout, interval).Should(BeNumerically("==", 0))

				pods, err := util.GetPodsFromDeployment(ctx, testContext.Clusters[0].ControllerClient, namespaceName, deploymentName)
				Expect(err).ToNot(HaveOccurred())
				Expect(pods).To(HaveLen(1))
				Expect(pods[0].Spec.NodeName).ToNot(BeEmpty())
				Expect(pods[0].Status.Phase).To(Equal(corev1.PodPending))

				// check the pod is schedulated on the remote cluster
				nodeName := pods[0].Spec.NodeName
				var node corev1.Node
				Expect(testContext.Clusters[0].ControllerClient.Get(ctx, client.ObjectKey{Name: nodeName}, &node)).To(Succeed())
				Expect(node.Labels).To(HaveKey(consts.TypeLabel))

				// clean up the deployment
				deleteDeployment()
			})

			It("Should not accept new resources", func() {
				Expect(util.ExecLiqoctl(
					testContext.Clusters[0].KubeconfigPath,
					[]string{"create", "resourceslice", sliceName, "--remote-cluster-id", string(testContext.Clusters[1].Cluster)},
					GinkgoWriter,
				)).To(Succeed())

				var slices authv1beta1.ResourceSliceList
				Expect(testContext.Clusters[0].ControllerClient.List(ctx, &slices)).To(Succeed())

				var slice *authv1beta1.ResourceSlice
				for i := range slices.Items {
					if slices.Items[i].Name == sliceName {
						slice = &slices.Items[i]
						break
					}
				}
				Expect(slice).ToNot(BeNil())
				Expect(slice.Namespace).ToNot(BeEmpty())
				tenantNamespace = slice.Namespace

				Consistently(func() authv1beta1.ResourceSliceConditionStatus {
					var s authv1beta1.ResourceSlice
					if err := testContext.Clusters[0].ControllerClient.Get(ctx,
						client.ObjectKey{Name: sliceName, Namespace: tenantNamespace}, &s); err != nil {
						return authv1beta1.ResourceSliceConditionAccepted
					}

					for i := range s.Status.Conditions {
						if s.Status.Conditions[i].Type == authv1beta1.ResourceSliceConditionTypeResources {
							return s.Status.Conditions[i].Status
						}
					}
					return authv1beta1.ResourceSliceConditionDenied
				}, consistentlyTimeout, interval).Should(Or(Equal(authv1beta1.ResourceSliceConditionDenied), Equal("")))

				Expect(testContext.Clusters[0].ControllerClient.Delete(ctx, slice)).To(Succeed())
			})

		})

	})
})

var _ = AfterSuite(func() {
	activateTenants()

	if tenantNamespace != "" {
		Expect(client.IgnoreNotFound(testContext.Clusters[0].ControllerClient.Delete(ctx, &authv1beta1.ResourceSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sliceName,
				Namespace: tenantNamespace,
			},
		}))).To(Succeed())
	}

	for i := range testContext.Clusters {
		Eventually(func() error {
			return util.EnsureNamespaceDeletion(ctx, testContext.Clusters[i].NativeClient, namespaceName)
		}, timeout, interval).Should(Succeed())
	}

	// wake up the resource slice to avoid next tests to fail
	wakeUpResourceSlice()
})
