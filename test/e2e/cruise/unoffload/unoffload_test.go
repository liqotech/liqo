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

package unoffload

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
	sliceutils "github.com/liqotech/liqo/pkg/utils/slice"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/test/e2e/testutils/config"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 2
	// testName is the name of this E2E test.
	testName = "UNOFFLOAD"
)

func TestE2E(t *testing.T) {
	util.CheckIfTestIsSkipped(t, clustersRequired, testName)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

var (
	ctx           = context.Background()
	testContext   = tester.GetTester(ctx)
	interval      = config.Interval
	timeout       = config.Timeout
	namespaceName = util.GetNameNamespaceTest(testName)

	deploymentName = "nginx"

	ensureResources = func() {
		// enforce deployment
		Expect(util.EnforceDeployment(ctx,
			testContext.Clusters[0].ControllerClient,
			namespaceName,
			deploymentName,
			util.RemoteDeploymentOption(),
		)).To(Succeed())

		// enforce service
		Expect(util.EnforceService(ctx,
			testContext.Clusters[0].ControllerClient,
			namespaceName,
			deploymentName,
		)).To(Succeed())

		// enforce config map
		cm := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-configmap",
				Namespace: namespaceName,
			},
		}
		Expect(client.IgnoreAlreadyExists(testContext.Clusters[0].ControllerClient.Create(ctx, &cm))).To(Succeed())
	}

	checkApp = func(cluster *tester.ClusterContext, homeIndex int) {
		// get deployment pod
		var pod *corev1.Pod
		Eventually(func() corev1.PodStatus {
			pods, err := util.GetPodsFromDeployment(ctx, cluster.ControllerClient, namespaceName, deploymentName)
			if err != nil {
				return corev1.PodStatus{}
			}

			switch len(pods) {
			case 0:
				return corev1.PodStatus{}
			case 1:
				pod = &pods[0]
				return pod.Status
			default:
				Fail(fmt.Sprintf("found more than one pod for deployment %s", deploymentName))
				return corev1.PodStatus{}
			}
		}, timeout, interval).Should(MatchFields(IgnoreExtras, Fields{
			"Phase": Equal(corev1.PodRunning),
		}))

		// check the pod in the remote cluster
		podNode := pod.Spec.NodeName
		var node corev1.Node
		Expect(cluster.ControllerClient.Get(ctx, client.ObjectKey{Name: podNode}, &node)).To(Succeed())

		clusterID, ok := node.Labels[consts.RemoteClusterID]
		Expect(ok).To(BeTrue())
		Expect(clusterID).ToNot(BeEmpty())

		var testerIndex = -1
		for i := range testContext.Clusters {
			if i == homeIndex {
				continue
			}

			remoteClusterID, err := utils.GetClusterID(ctx, testContext.Clusters[i].NativeClient, "liqo")
			Expect(err).ToNot(HaveOccurred())
			if string(remoteClusterID) == clusterID {
				testerIndex = i
				break
			}
		}
		Expect(testerIndex).ToNot(Equal(-1))

		// check the pod
		Eventually(func() []corev1.PodStatus {
			var podList corev1.PodList
			_ = testContext.Clusters[testerIndex].ControllerClient.List(ctx, &podList, client.InNamespace(namespaceName))
			return sliceutils.Map(podList.Items, func(p corev1.Pod) corev1.PodStatus {
				return p.Status
			})
		}, timeout, interval).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
			"Phase": Equal(corev1.PodRunning),
		})))

		// check the service
		for i := range testContext.Clusters {
			if testContext.Clusters[i].Role == liqov1beta1.ConsumerRole {
				continue
			}

			Expect(testContext.Clusters[i].ControllerClient.Get(ctx,
				client.ObjectKey{Name: deploymentName, Namespace: namespaceName}, &corev1.Service{})).To(Succeed())
		}

		// check the config map
		for i := range testContext.Clusters {
			if testContext.Clusters[i].Role == liqov1beta1.ConsumerRole {
				continue
			}

			Expect(testContext.Clusters[i].ControllerClient.Get(ctx,
				client.ObjectKey{Name: "test-configmap", Namespace: namespaceName}, &corev1.ConfigMap{})).To(Succeed())
		}
	}
)

var _ = Describe("Liqo E2E", func() {
	Context("Unoffload", func() {

		BeforeEach(func() {
			// ensure the namespace is created
			for i := range testContext.Clusters {
				if testContext.Clusters[i].Role == liqov1beta1.ConsumerRole {
					// ensure the namespace is created
					Expect(util.Second(util.EnforceNamespace(ctx, testContext.Clusters[i].NativeClient,
						testContext.Clusters[i].Cluster, namespaceName))).To(Succeed())

					_ = util.OffloadNamespace(testContext.Clusters[i].KubeconfigPath, namespaceName,
						"--namespace-mapping-strategy=EnforceSameName")

					// wait for the namespace to be offloaded, this avoids race conditions
					time.Sleep(2 * time.Second)

					ensureResources()
					checkApp(&testContext.Clusters[i], i)

					// only for the first consumer
					break
				}
			}
		})

		When("the namespace is unoffloaded", func() {

			BeforeEach(func() {
				Expect(util.UnoffloadNamespace(testContext.Clusters[0].KubeconfigPath, namespaceName)).To(Succeed())
			})

			It("should not found resources in the remote cluster", func() {
				// check the pod
				for i := range testContext.Clusters {
					if testContext.Clusters[i].Role == liqov1beta1.ConsumerRole {
						continue
					}

					Eventually(func() []corev1.Pod {
						var podList corev1.PodList
						_ = testContext.Clusters[i].ControllerClient.List(ctx, &podList, client.InNamespace(namespaceName))
						return podList.Items
					}, timeout, interval).Should(BeEmpty())
				}

				// check the service
				for i := range testContext.Clusters {
					if testContext.Clusters[i].Role == liqov1beta1.ConsumerRole {
						continue
					}

					Eventually(func() error {
						return testContext.Clusters[i].ControllerClient.Get(ctx,
							client.ObjectKey{Name: deploymentName, Namespace: namespaceName}, &corev1.Service{})
					}, timeout, interval).Should(BeNotFound())
				}

				// check the config map
				for i := range testContext.Clusters {
					if testContext.Clusters[i].Role == liqov1beta1.ConsumerRole {
						continue
					}

					Eventually(func() error {
						return testContext.Clusters[i].ControllerClient.Get(ctx,
							client.ObjectKey{Name: "test-configmap", Namespace: namespaceName}, &corev1.ConfigMap{})
					}, timeout, interval).Should(BeNotFound())
				}

				// check the namespace is deleted
				for i := range testContext.Clusters {
					if testContext.Clusters[i].Role == liqov1beta1.ConsumerRole {
						continue
					}

					Eventually(func() error {
						return testContext.Clusters[i].ControllerClient.Get(ctx, client.ObjectKey{Name: namespaceName}, &corev1.Namespace{})
					}, timeout, interval).Should(BeNotFound())
				}
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
})
