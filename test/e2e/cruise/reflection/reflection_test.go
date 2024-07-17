// Copyright 2019-2024 The Liqo Authors
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

package reflection

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	dto "github.com/prometheus/client_model/go"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1alpha1 "github.com/liqotech/liqo/apis/core/v1alpha1"
	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	"github.com/liqotech/liqo/pkg/utils/getters"
	podutils "github.com/liqotech/liqo/pkg/utils/pod"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/test/e2e/testutils/config"
	httputils "github.com/liqotech/liqo/test/e2e/testutils/http"
	metricsutils "github.com/liqotech/liqo/test/e2e/testutils/metrics"
	"github.com/liqotech/liqo/test/e2e/testutils/portforward"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 2
	// testName is the name of this E2E test.
	testName = "REFLECTION"

	metricReflectionCounter = "liqo_virtual_kubelet_reflection_item_counter"
	targePortMetrics        = 5872
)

func TestE2E(t *testing.T) {
	util.CheckIfTestIsSkipped(t, clustersRequired, testName)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

type objectRetriever func() client.Object

var (
	ctx           = context.Background()
	testContext   = tester.GetTester(ctx)
	interval      = config.Interval
	timeout       = config.Timeout
	namespaceName = util.GetNameNamespaceTest(testName)
	indexCons     = 0 // the first should always be a consumer
	consumer      = testContext.Clusters[indexCons]
	providers     = tester.GetProviders(testContext.Clusters)

	ensureResourcesDeletion = func(getObj objectRetriever, consumer tester.ClusterContext, providers ...tester.ClusterContext) {
		// Delete the object on the consumer cluster
		Expect(util.DeleteResource(ctx, consumer.ControllerClient, getObj())).To(Succeed())

		// Check that the resource is effectively deleted on the consumer
		Eventually(func() error {
			res := getObj()
			return util.GetResource(ctx, consumer.ControllerClient, res)
		}, timeout, interval).Should(BeNotFound())

		// Check that the resource is effectively deleted on the providers
		for _, provider := range providers {
			Eventually(func() error {
				res := getObj()
				return util.GetResource(ctx, provider.ControllerClient, res)
			}, timeout, interval).Should(BeNotFound())
		}
	}

	// retrieveMetrics port-forwards a pod to the local machine and retrieves the metrics.
	retrieveMetrics = func(ctx context.Context, podName, podNamespace string, localPort int) map[string]*dto.MetricFamily {
		// Portforward virtualkubelet pod to retrieve metrics
		ppf := portforward.NewPodPortForwarderOptions(consumer.Config, consumer.ControllerClient,
			podName, podNamespace, localPort, targePortMetrics)

		// Managing termination signal from the terminal.
		// The stopCh gets closed to gracefully handle its termination.
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(signals)

		returnCtx, returnCtxCancel := context.WithCancel(ctx)
		defer returnCtxCancel()

		go func() {
			select {
			case <-signals:
			case <-returnCtx.Done():
			}
			if ppf.StopCh != nil {
				close(ppf.StopCh)
			}
		}()

		go func() {
			err := ppf.PortForwardPod(ctx)
			Expect(err).ToNot(HaveOccurred())
		}()

		// We need to wait some seconds to allow the port-forward to be ready.
		time.Sleep(2 * time.Second)

		// Curl metrics from port forwarded addess.
		metrics := curlMetrics("localhost", localPort)

		// Parse the metrics.
		metricFamilies, err := metricsutils.ParseMetrics(metrics)
		Expect(err).ToNot(HaveOccurred())

		return metricFamilies
	}

	// Curl metrics from the port-forwarded address.
	curlMetrics = func(url string, port int) string {
		// Get the metrics from the virtual-kubelet pod
		resp, body, err := httputils.NewHTTPClient(timeout).Curl(ctx, fmt.Sprintf("http://%s:%d/metrics", url, port))
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(body).ToNot(Or(BeNil(), (BeEmpty())))
		return string(body)
	}

	// Retrieve the counter of reflections for the given resource.
	// This metrics is used to check that no infinite or excessive reconciliations are happening
	// (likely caused by race conditions), meaning so the value should be reasonably low.
	retrieveMetricReflectionCounter = func(metricFamilies map[string]*dto.MetricFamily,
		clusterID, namespace, nodeName, resource string) float64 {
		counter, err := metricsutils.RetrieveCounter(metricFamilies, metricReflectionCounter, map[string]string{
			"cluster_id":         clusterID,
			"namespace":          namespace,
			"node_name":          nodeName,
			"reflector_resource": resource,
		})
		Expect(err).ToNot(HaveOccurred())
		return counter
	}

	getVirtualKubeletPod = func(providerClusterID liqov1alpha1.ClusterID) *corev1.Pod {
		// Get virtualnode
		vNodes, err := getters.ListVirtualNodesByClusterID(ctx, consumer.ControllerClient, providerClusterID)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(vNodes)).To(Equal(1))

		// Get virtual-kubelet pod
		vkPods, err := getters.ListVirtualKubeletPodsFromVirtualNode(ctx, consumer.ControllerClient, &vNodes[0])
		Expect(err).ToNot(HaveOccurred())
		Expect(len(vkPods.Items)).To(Equal(1))

		return &vkPods.Items[0]
	}
)

var _ = BeforeSuite(func() {
	Expect(consumer.Role).To(Equal(liqov1alpha1.ConsumerRole))

	// ensure the namespace is created
	Expect(util.Second(util.EnforceNamespace(ctx, consumer.NativeClient,
		consumer.Cluster, namespaceName))).To(Succeed())

	Expect(util.OffloadNamespace(consumer.KubeconfigPath, namespaceName,
		"--namespace-mapping-strategy", string(offv1alpha1.EnforceSameNameMappingStrategyType),
		"--pod-offloading-strategy", string(offv1alpha1.LocalAndRemotePodOffloadingStrategyType),
	)).To(Succeed())
	// wait for the namespace to be offloaded, this avoids race conditions
	time.Sleep(2 * time.Second)
})

var _ = Describe("Liqo E2E", func() {
	Context("Reflection of resources to remote provider cluster", func() {

		When("Offloading pods to remote provider clusters", func() {
			var (
				podNamePrefix    = "pod-test"
				getRemotePodName = func(clusterID liqov1alpha1.ClusterID) string {
					return fmt.Sprintf("%s-%s", podNamePrefix, clusterID)
				}
				getPod = func(clusterID liqov1alpha1.ClusterID) *corev1.Pod {
					return &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      getRemotePodName(clusterID),
							Namespace: namespaceName,
						},
					}
				}
			)

			BeforeEach(func() {
				// On the consumer, create a remote pod for each provider clusters
				for _, provider := range providers {
					Expect(util.EnforcePod(ctx,
						consumer.ControllerClient,
						namespaceName,
						getRemotePodName(provider.Cluster),
						util.RemotePodOption(true, ptr.To(string(provider.Cluster))),
					)).To(Succeed())
				}
			})

			AfterEach(func() {
				// On the consumer, delete the remote pods
				for _, provider := range providers {
					ensureResourcesDeletion(func() client.Object {
						return getPod(provider.Cluster)
					}, consumer, provider)
				}
			})

			It("Remote pods should be scheduled on the provider clusters and ready", func() {
				for _, provider := range providers {
					Eventually(func() error {
						pod := getPod(provider.Cluster)
						if err := util.GetResource(ctx, provider.ControllerClient, pod); err != nil {
							return err
						}

						// check for the pod to be ready
						ready, _ := podutils.IsPodReady(pod)
						if !ready {
							return fmt.Errorf("pod %s/%s is not ready", pod.GetNamespace(), pod.GetName())
						}
						return nil
					}, timeout, interval).Should(Succeed())

					// Get metrics from virtual-kubelet pod, using local portforwarding.
					localPort := targePortMetrics // we use the same port for all providers
					vkPod := getVirtualKubeletPod(provider.Cluster)
					metrics := retrieveMetrics(ctx, vkPod.Name, vkPod.Namespace, localPort)

					// Check the reflection counter for the resources
					counter := retrieveMetricReflectionCounter(metrics,
						string(provider.Cluster), namespaceName, string(provider.Cluster), "Pod")
					Expect(counter).To(BeNumerically("<", 100))
				}
			})
		})

		When("Creating a service on the consumer cluster", func() {
			var (
				serviceName = "svc-test"
				getService  = func() client.Object {
					return &corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      serviceName,
							Namespace: namespaceName,
						},
					}
				}
				createService = func(name string) {
					Expect(util.EnforceService(ctx, consumer.ControllerClient, namespaceName, name, util.WithNodePort())).To(Succeed())
				}
			)

			BeforeEach(func() {
				createService(serviceName)
			})

			AfterEach(func() {
				ensureResourcesDeletion(getService, consumer, providers...)
			})

			It("Service and EndpointSlice should be replicated on all provider clusters", func() {
				for _, provider := range providers {
					Eventually(func() error {
						svc := getService()
						return util.GetResource(ctx, provider.ControllerClient, svc)
					}, timeout, interval).Should(Succeed())

					// Check that the custom Liqo endpointslice is reflected on the provider cluster
					epsLabelSelector := map[string]string{
						discoveryv1.LabelServiceName:      serviceName,
						discoveryv1.LabelManagedBy:        forge.EndpointSliceManagedBy,
						forge.LiqoOriginClusterIDKey:      string(consumer.Cluster),
						forge.LiqoDestinationClusterIDKey: string(provider.Cluster),
					}
					Eventually(func() error {
						var epslices discoveryv1.EndpointSliceList
						if err := provider.ControllerClient.List(ctx, &epslices,
							client.InNamespace(namespaceName),
							client.MatchingLabels(epsLabelSelector)); err != nil {
							return err
						}
						if len(epslices.Items) != 1 {
							return fmt.Errorf("expected 1 endpointslice, got %d", len(epslices.Items))
						}
						return nil
					}, timeout, interval).Should(Succeed())

					// Get metrics from virtual-kubelet pod, using local portforwarding.
					localPort := targePortMetrics // we use the same port for all providers
					vkPod := getVirtualKubeletPod(provider.Cluster)
					metrics := retrieveMetrics(ctx, vkPod.Name, vkPod.Namespace, localPort)

					// Check the reflection counter for the resources
					counter := retrieveMetricReflectionCounter(metrics,
						string(provider.Cluster), namespaceName, string(provider.Cluster), "Service")
					Expect(counter).To(BeNumerically("<", 100))
					counter = retrieveMetricReflectionCounter(metrics,
						string(provider.Cluster), namespaceName, string(provider.Cluster), "EndpointSlice")
					Expect(counter).To(BeNumerically("<", 100))
				}
			})
		})

		When("Creating an ingress on the consumer cluster", func() {
			var (
				ingressName = "ingress-test"
				getIngress  = func() client.Object {
					return &netv1.Ingress{
						ObjectMeta: metav1.ObjectMeta{
							Name:      ingressName,
							Namespace: namespaceName,
						},
					}
				}
				createIngress = func(name string) {
					Expect(util.EnforceIngress(ctx, consumer.ControllerClient, namespaceName, name)).To(Succeed())
				}
			)

			BeforeEach(func() {
				createIngress(ingressName)
			})

			AfterEach(func() {
				ensureResourcesDeletion(getIngress, consumer, providers...)
			})

			It("Ingress should be replicated on all provider clusters", func() {
				for _, provider := range providers {
					Eventually(func() error {
						ingress := getIngress()
						return util.GetResource(ctx, provider.ControllerClient, ingress)
					}, timeout, interval).Should(Succeed())

					// Get metrics from virtual-kubelet pod, using local portforwarding.
					localPort := targePortMetrics // we use the same port for all providers
					vkPod := getVirtualKubeletPod(provider.Cluster)
					metrics := retrieveMetrics(ctx, vkPod.Name, vkPod.Namespace, localPort)

					// Check the reflection counter for Ithe resource
					counter := retrieveMetricReflectionCounter(metrics,
						string(provider.Cluster), namespaceName, string(provider.Cluster), "Ingress")
					Expect(counter).To(BeNumerically("<", 100))
				}
			})
		})

		When("Creating a configmap on the consumer cluster", func() {
			var (
				configMapName = "cm-test"
				getConfigMap  = func() client.Object {
					return &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      configMapName,
							Namespace: namespaceName,
						},
					}
				}
				createConfigMap = func(name string) {
					Expect(util.EnforceConfigMap(ctx, consumer.ControllerClient, namespaceName, name)).To(Succeed())
				}
			)

			BeforeEach(func() {
				createConfigMap(configMapName)
			})

			AfterEach(func() {
				ensureResourcesDeletion(getConfigMap, consumer, providers...)
			})

			It("ConfigMap should be replicated on all provider clusters", func() {
				for _, provider := range providers {
					Eventually(func() error {
						cm := getConfigMap()
						return util.GetResource(ctx, provider.ControllerClient, cm)
					}, timeout, interval).Should(Succeed())

					// Get metrics from virtual-kubelet pod, using local portforwarding.
					localPort := targePortMetrics // we use the same port for all providers
					vkPod := getVirtualKubeletPod(provider.Cluster)
					metrics := retrieveMetrics(ctx, vkPod.Name, vkPod.Namespace, localPort)

					// Check the reflection counter for Ithe resource
					counter := retrieveMetricReflectionCounter(metrics,
						string(provider.Cluster), namespaceName, string(provider.Cluster), "ConfigMap")
					Expect(counter).To(BeNumerically("<", 100))
				}
			})
		})

		When("Creating a secret on the consumer cluster", func() {
			var (
				secretName = "secret-test"
				getSecret  = func() client.Object {
					return &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      secretName,
							Namespace: namespaceName,
						},
					}
				}
				createSecret = func(name string) {
					Expect(util.EnforceSecret(ctx, consumer.ControllerClient, namespaceName, name)).To(Succeed())
				}
			)

			BeforeEach(func() {
				createSecret(secretName)
			})

			AfterEach(func() {
				ensureResourcesDeletion(getSecret, consumer, providers...)
			})

			It("Secret should be replicated on all provider clusters", func() {
				for _, provider := range providers {
					Eventually(func() error {
						secret := getSecret()
						return util.GetResource(ctx, provider.ControllerClient, secret)
					}, timeout, interval).Should(Succeed())

					// Get metrics from virtual-kubelet pod, using local portforwarding.
					localPort := targePortMetrics // we use the same port for all providers
					vkPod := getVirtualKubeletPod(provider.Cluster)
					metrics := retrieveMetrics(ctx, vkPod.Name, vkPod.Namespace, localPort)

					// Check the reflection counter for Ithe resource
					counter := retrieveMetricReflectionCounter(metrics,
						string(provider.Cluster), namespaceName, string(provider.Cluster), "Secret")
					Expect(counter).To(BeNumerically("<", 100))
				}
			})
		})

		When("Creating an event on the provider cluster", func() {
			var (
				eventNamePrefix    = "event-test"
				getRemoteEventName = func(clusterID liqov1alpha1.ClusterID) string {
					return fmt.Sprintf("%s-%s", eventNamePrefix, clusterID)
				}
				getEvent = func(clusterID liqov1alpha1.ClusterID) client.Object {
					return &corev1.Event{
						ObjectMeta: metav1.ObjectMeta{
							Name:      getRemoteEventName(clusterID),
							Namespace: namespaceName,
						},
					}
				}
			)

			BeforeEach(func() {
				// Create a resource on the consumer cluster to use as involvedObject for the events
				Expect(util.EnforceSecret(ctx, consumer.ControllerClient, namespaceName, "secret-test-event")).To(Succeed())

				// On each providers, create a remote event on the consumer cluster.
				for _, provider := range providers {
					Expect(util.EnforceEvent(ctx,
						provider.ControllerClient,
						namespaceName,
						getRemoteEventName(provider.Cluster),
						&corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Namespace:  namespaceName,
							Name:       "secret-test-event",
						},
					)).To(Succeed())

					// Get metrics from virtual-kubelet pod, using local portforwarding.
					localPort := targePortMetrics // we use the same port for all providers
					vkPod := getVirtualKubeletPod(provider.Cluster)
					metrics := retrieveMetrics(ctx, vkPod.Name, vkPod.Namespace, localPort)

					// Check the reflection counter for Ithe resource
					counter := retrieveMetricReflectionCounter(metrics,
						string(provider.Cluster), namespaceName, string(provider.Cluster), "Event")
					Expect(counter).To(BeNumerically("<", 1000))
				}
			})

			AfterEach(func() {
				// On the providers, delete the events
				for _, provider := range providers {
					ensureResourcesDeletion(func() client.Object {
						return getEvent(provider.Cluster)
					}, provider, consumer)
				}
			})

			It("Each event on the providers should be replicated on the consumer cluster", func() {
				for _, provider := range providers {
					Eventually(func() error {
						event := getEvent(provider.Cluster)
						return util.GetResource(ctx, consumer.ControllerClient, event)
					}, timeout, interval).Should(Succeed())
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
