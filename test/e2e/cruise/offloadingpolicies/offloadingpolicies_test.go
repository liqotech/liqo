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

package offloadingpolicies

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/test/e2e/testutils/config"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 2
	// testName is the name of this E2E test.
	testName = "OFFLOADING_POLICIES"
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
)

var _ = BeforeSuite(func() {
	// create it before the test to avoid a k8s race condition that is preventing using it for a while after the creation
	runtimeClass := &nodev1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "liqo",
		},
		Handler: "liqo",
		Scheduling: &nodev1.Scheduling{
			NodeSelector: map[string]string{
				consts.TypeLabel: consts.TypeNode,
			},
			Tolerations: []corev1.Toleration{
				{
					Key:      consts.VirtualNodeTolerationKey,
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoExecute,
				},
			},
		},
	}
	Expect(client.IgnoreAlreadyExists(testContext.Clusters[0].ControllerClient.Create(ctx, runtimeClass))).To(Succeed())
})

var _ = Describe("Liqo E2E", func() {
	Context("Offloading Policies", func() {

		var (
			deploymentName = "nginx"

			createLocalDeployment = func(options ...util.DeploymentOption) {
				options = append(options, util.LocalDeploymentOption())
				Expect(util.EnforceDeployment(ctx,
					testContext.Clusters[0].ControllerClient,
					namespaceName,
					deploymentName,
					options...,
				)).To(Succeed())
			}
			createRemoteDeployment = func(options ...util.DeploymentOption) {
				options = append(options, util.RemoteDeploymentOption())
				Expect(util.EnforceDeployment(ctx,
					testContext.Clusters[0].ControllerClient,
					namespaceName,
					deploymentName,
					options...,
				)).To(Succeed())
			}
			deleteDeployment = func() {
				Expect(util.EnsureDeploymentDeletion(ctx, testContext.Clusters[0].ControllerClient, namespaceName, deploymentName)).To(Succeed())
				Eventually(func() error {
					var d appsv1.Deployment
					return testContext.Clusters[0].ControllerClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: deploymentName}, &d)
				}, timeout, interval).ShouldNot(Succeed())
			}

			deploymentRunning = func() {
				Eventually(func() appsv1.DeploymentStatus {
					var d appsv1.Deployment
					if err := testContext.Clusters[0].ControllerClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: deploymentName}, &d); err != nil {
						return appsv1.DeploymentStatus{}
					}
					return d.Status
				}, timeout, interval).Should(MatchFields(IgnoreExtras, Fields{
					"Replicas":      BeNumerically("==", 1),
					"ReadyReplicas": BeNumerically("==", 1),
				}))
			}
			deploymentPending = func() {
				Consistently(func() appsv1.DeploymentStatus {
					var d appsv1.Deployment
					if err := testContext.Clusters[0].ControllerClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: deploymentName}, &d); err != nil {
						return appsv1.DeploymentStatus{}
					}
					return d.Status
				}, consistentlyTimeout, interval).Should(MatchFields(IgnoreExtras, Fields{
					"ReadyReplicas": BeNumerically("==", 0),
				}))
			}
		)

		BeforeEach(func() {
			Expect(testContext.Clusters[0].Role).To(Equal(liqov1beta1.ConsumerRole))

			// ensure the namespace is created
			Expect(util.Second(util.EnforceNamespace(ctx, testContext.Clusters[0].NativeClient,
				testContext.Clusters[0].Cluster, namespaceName))).To(Succeed())
		})

		AfterEach(func() {
			// ensure unoffload the namespace
			_ = util.UnoffloadNamespace(testContext.Clusters[0].KubeconfigPath, namespaceName)

			deleteDeployment()

			Eventually(func() error {
				_, err := util.GetNamespaceOffloading(ctx, testContext.Clusters[0].ControllerClient, namespaceName)
				return err
			}, timeout, interval).Should(BeNotFound())
		})

		When("the offloading policy is set to Local", func() {

			BeforeEach(func() {
				Eventually(util.OffloadNamespace(testContext.Clusters[0].KubeconfigPath,
					namespaceName, "--pod-offloading-strategy=Local"), timeout, interval).Should(Succeed())

				// wait for the namespace to be offloaded, this avoids race conditions
				time.Sleep(2 * time.Second)
			})

			It("should not schedule the remote deployment", func() {
				createRemoteDeployment()
				deploymentPending()
			})

			It("should schedule the local deployment", func() {
				createLocalDeployment()
				deploymentRunning()
			})

			It("should schedule the remote deployment with runtime class", func() {
				createRemoteDeployment(util.RuntimeClassOption("liqo"))
				deploymentRunning()
			})

			It("should not schedule the local deployment with runtime class", func() {
				createLocalDeployment(util.RuntimeClassOption("liqo"))
				deploymentPending()
			})
		})

		When("the offloading policy is set to Remote", func() {

			BeforeEach(func() {
				Eventually(util.OffloadNamespace(testContext.Clusters[0].KubeconfigPath,
					namespaceName, "--pod-offloading-strategy=Remote"), timeout, interval).Should(Succeed())

				// wait for the namespace to be offloaded, this avoids race conditions
				time.Sleep(2 * time.Second)
			})

			It("should schedule the remote deployment", func() {
				createRemoteDeployment()
				deploymentRunning()
			})

			It("should not schedule the local deployment", func() {
				createLocalDeployment()
				deploymentPending()
			})

			It("should schedule the remote deployment with runtime class", func() {
				createRemoteDeployment(util.RuntimeClassOption("liqo"))
				deploymentRunning()
			})

			It("should not schedule the local deployment with runtime class", func() {
				createLocalDeployment(util.RuntimeClassOption("liqo"))
				deploymentPending()
			})

		})

		When("the offloading policy is set to LocalAndRemote", func() {

			BeforeEach(func() {
				Eventually(util.OffloadNamespace(testContext.Clusters[0].KubeconfigPath,
					namespaceName, "--pod-offloading-strategy=LocalAndRemote"), timeout, interval).Should(Succeed())

				// wait for the namespace to be offloaded, this avoids race conditions
				time.Sleep(2 * time.Second)
			})

			It("should schedule the remote deployment", func() {
				createRemoteDeployment()
				deploymentRunning()
			})

			It("should schedule the local deployment", func() {
				createLocalDeployment()
				deploymentRunning()
			})

			It("should schedule the remote deployment with runtime class", func() {
				createRemoteDeployment(util.RuntimeClassOption("liqo"))
				deploymentRunning()
			})

			It("should not schedule the local deployment with runtime class", func() {
				createLocalDeployment(util.RuntimeClassOption("liqo"))
				deploymentPending()
			})

		})
	})
})

var _ = AfterSuite(func() {
	for i := range testContext.Clusters {
		Eventually(func() error {
			return util.EnsureNamespaceDeletion(ctx, testContext.Clusters[i].NativeClient, namespaceName)
		}, timeout, interval).Should(Succeed())
	}

	Expect(testContext.Clusters[0].ControllerClient.Delete(ctx, &nodev1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "liqo",
		},
	})).To(Succeed())

	Eventually(func() error {
		var rc nodev1.RuntimeClass
		return testContext.Clusters[0].ControllerClient.Get(ctx, client.ObjectKey{Name: "liqo"}, &rc)
	}(), timeout, interval).Should(BeNotFound())
})
