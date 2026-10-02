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

package gatewayapi

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/test/e2e/testutils/config"
	"github.com/liqotech/liqo/test/e2e/testutils/portforward"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 2
	// testName is the name of this E2E test.
	testName = "GATEWAYAPI"

	// virtualClass is the GatewayClass whose Gateways are reflected, created by Liqo in the consumer cluster.
	virtualClass = "liqo"
	// providerClass is the GatewayClass announced by the provider clusters (see test/e2e/manifests/gatewayapi/provider.yaml).
	providerClass = "eg"
	// sharedGatewayNamespace and sharedGatewayName identify the shared Gateway announced by the provider clusters.
	sharedGatewayNamespace = "infra"
	sharedGatewayName      = "public"

	gatewayName = "web"
	backendName = "backend"

	// rejectLabel marks the routes rejected by the admission policy configured in the first provider cluster.
	rejectLabel = "e2e.liqo.io/reject"
	// policyName is the name of the admission policy rejecting the routes in the first provider cluster.
	policyName = "liqo-e2e-reject-httproutes"
)

func TestE2E(t *testing.T) {
	util.CheckIfTestIsSkipped(t, clustersRequired, testName)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

var (
	ctx         = context.Background()
	testContext = tester.GetTester(ctx)
	interval    = config.Interval
	timeout     = config.Timeout
	// dataPlaneTimeout accounts for the time required by Envoy Gateway to deploy and program the proxies.
	dataPlaneTimeout = 5 * time.Minute

	namespaceName = util.GetNameNamespaceTest(testName)
	// localNamespaceName is a namespace not offloaded, hosting a Gateway of a class different from the virtual one.
	localNamespaceName = namespaceName + "-local"
	indexCons          = 0 // the first should always be a consumer
	consumer           = testContext.Clusters[indexCons]
	providers          = tester.GetProviders(testContext.Clusters)
)

// liqoParentStatus returns the status of the given route with respect to its parents, as reported by Liqo.
func liqoParentStatus(route *gwv1.HTTPRoute) *gwv1.RouteParentStatus {
	for i := range route.Status.Parents {
		if route.Status.Parents[i].ControllerName == consts.GatewayControllerName {
			return &route.Status.Parents[i]
		}
	}
	return nil
}

// acceptedCondition returns the Accepted condition reported by Liqo for the given local route.
func acceptedCondition(name string) (*metav1.Condition, error) {
	var route gwv1.HTTPRoute
	if err := consumer.ControllerClient.Get(ctx, types.NamespacedName{Namespace: namespaceName, Name: name}, &route); err != nil {
		return nil, err
	}
	parent := liqoParentStatus(&route)
	if parent == nil {
		return nil, fmt.Errorf("no status reported by Liqo for route %q", name)
	}
	condition := meta.FindStatusCondition(parent.Conditions, string(gwv1.RouteConditionAccepted))
	if condition == nil {
		return nil, fmt.Errorf("no Accepted condition reported by Liqo for route %q", name)
	}
	return condition, nil
}

// remoteRoute retrieves the route reflected in the given provider cluster.
func remoteRoute(provider *tester.ClusterContext, name string) (*gwv1.HTTPRoute, error) {
	var route gwv1.HTTPRoute
	err := provider.ControllerClient.Get(ctx, types.NamespacedName{Namespace: namespaceName, Name: name}, &route)
	return &route, err
}

// failedReflectionEvents returns the messages of the FailedReflection events concerning the given local object.
func failedReflectionEvents(kind, name string) []string {
	var events corev1.EventList
	Expect(consumer.ControllerClient.List(ctx, &events, client.InNamespace(namespaceName))).To(Succeed())

	var messages []string
	for i := range events.Items {
		event := &events.Items[i]
		if event.InvolvedObject.Kind == kind && event.InvolvedObject.Name == name && event.Reason == forge.EventFailedReflection {
			messages = append(messages, event.Message)
		}
	}
	return messages
}

// requestThroughGateway sends an HTTP request for the given host to the Envoy proxy serving the given Gateway
// in the given provider cluster, through a port-forward, and returns the resulting status code.
func requestThroughGateway(provider *tester.ClusterContext, gatewayNamespace, gateway, host string) (int, error) {
	pod, targetPort, err := util.EnvoyProxyTarget(ctx, provider.ControllerClient, gatewayNamespace, gateway, 80)
	if err != nil {
		return 0, err
	}

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	localPort := listener.Addr().(*net.TCPAddr).Port
	Expect(listener.Close()).To(Succeed())

	pf := portforward.NewPodPortForwarderOptions(provider.Config, provider.ControllerClient,
		pod, util.EnvoyGatewayNamespace, localPort, targetPort)
	defer close(pf.StopCh)

	errCh := make(chan error, 1)
	go func() { errCh <- pf.PortForwardPod(ctx) }()

	select {
	case <-pf.ReadyCh:
	case err := <-errCh:
		return 0, fmt.Errorf("port-forward to pod %q failed: %w", pod, err)
	case <-time.After(30 * time.Second):
		return 0, fmt.Errorf("port-forward to pod %q not ready", pod)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/", localPort), http.NoBody)
	if err != nil {
		return 0, err
	}
	request.Host = host

	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return 0, err
	}
	Expect(response.Body.Close()).To(Succeed())
	return response.StatusCode, nil
}

// createRejectionPolicy creates an admission policy, in the given provider cluster, rejecting the routes with the reject label
// in the test namespace, to simulate a provider cluster rejecting a route (e.g., due to a stricter Gateway API validation).
func createRejectionPolicy(provider *tester.ClusterContext) {
	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: policyName},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			FailurePolicy: ptr.To(admissionregistrationv1.Fail),
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
						Rule: admissionregistrationv1.Rule{
							APIGroups: []string{gwv1.GroupName}, APIVersions: []string{"*"}, Resources: []string{"httproutes"},
						},
					},
				}},
			},
			Validations: []admissionregistrationv1.Validation{{
				Expression: fmt.Sprintf("!has(object.metadata.labels) || !('%s' in object.metadata.labels)", rejectLabel),
				Message:    "route rejected by the e2e admission policy",
			}},
		},
	}
	binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: policyName},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName:        policyName,
			ValidationActions: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
			MatchResources: &admissionregistrationv1.MatchResources{NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{corev1.LabelMetadataName: namespaceName},
			}},
		},
	}
	Expect(client.IgnoreAlreadyExists(provider.ControllerClient.Create(ctx, policy))).To(Succeed())
	Expect(client.IgnoreAlreadyExists(provider.ControllerClient.Create(ctx, binding))).To(Succeed())

	// Wait for the policy to be enforced, by checking that a labeled route is rejected (dry-run).
	Eventually(func() error {
		route := &gwv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "policy-check", Namespace: namespaceName, Labels: map[string]string{rejectLabel: "true"}},
			Spec:       gwv1.HTTPRouteSpec{CommonRouteSpec: gwv1.CommonRouteSpec{ParentRefs: []gwv1.ParentReference{{Name: "check"}}}},
		}
		if err := provider.ControllerClient.Create(ctx, route, client.DryRunAll); err == nil {
			return fmt.Errorf("admission policy not yet enforced")
		}
		return nil
	}, timeout, interval).Should(Succeed())
}

func deleteRejectionPolicy(provider *tester.ClusterContext) {
	Expect(client.IgnoreNotFound(provider.ControllerClient.Delete(ctx,
		&admissionregistrationv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: policyName}}))).To(Succeed())
	Expect(client.IgnoreNotFound(provider.ControllerClient.Delete(ctx,
		&admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: policyName}}))).To(Succeed())
}

var _ = BeforeSuite(func() {
	Expect(consumer.Role).To(Equal(liqov1beta1.ConsumerRole))
	Expect(providers).ToNot(BeEmpty())

	Expect(util.Second(util.EnforceNamespace(ctx, consumer.NativeClient, consumer.Cluster, namespaceName))).To(Succeed())
	Expect(util.OffloadNamespace(consumer.KubeconfigPath, namespaceName,
		"--namespace-mapping-strategy", string(offloadingv1beta1.EnforceSameNameMappingStrategyType),
		"--pod-offloading-strategy", string(offloadingv1beta1.RemotePodOffloadingStrategyType),
	)).To(Succeed())
	Expect(util.Second(util.EnforceNamespace(ctx, consumer.NativeClient, consumer.Cluster, localNamespaceName))).To(Succeed())

	// Wait for the namespace to be created in all provider clusters, as required by the admission policy.
	for i := range providers {
		provider := &providers[i]
		Eventually(func() error {
			return util.GetResource(ctx, provider.ControllerClient, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}})
		}, timeout, interval).Should(Succeed())
	}
	createRejectionPolicy(&providers[0])

	// One backend pod for each provider cluster, so that each Envoy proxy has a local endpoint.
	Expect(util.EnforceService(ctx, consumer.ControllerClient, namespaceName, backendName)).To(Succeed())
	for i := range providers {
		provider := &providers[i]
		Expect(util.EnforcePod(ctx, consumer.ControllerClient, namespaceName, fmt.Sprintf("%s-%s", backendName, provider.Cluster),
			util.RemotePodOption(true, ptr.To(string(provider.Cluster))),
			func(pod *corev1.Pod) { pod.SetLabels(map[string]string{"app": backendName}) },
		)).To(Succeed())
	}

	// A Gateway of the virtual class, reflected to the provider clusters, and one of another class, which is not reflected.
	Expect(util.EnforceGateway(ctx, consumer.ControllerClient, namespaceName, gatewayName, virtualClass)).To(Succeed())
	Expect(util.EnforceGateway(ctx, consumer.ControllerClient, localNamespaceName, "edge", "e2e-local")).To(Succeed())
})

var _ = AfterSuite(func() {
	deleteRejectionPolicy(&providers[0])

	for i := range testContext.Clusters {
		Eventually(func() error {
			return util.EnsureNamespaceDeletion(ctx, testContext.Clusters[i].NativeClient, namespaceName)
		}, timeout, interval).Should(Succeed())
	}
	Eventually(func() error {
		return util.EnsureNamespaceDeletion(ctx, consumer.NativeClient, localNamespaceName)
	}, timeout, interval).Should(Succeed())
})

var _ = Describe("Liqo E2E", Ordered, func() {
	Context("Reflection of the Gateway API resources to the remote provider clusters", func() {
		It("the virtual GatewayClass should be accepted", func() {
			Eventually(func() (*metav1.Condition, error) {
				var class gwv1.GatewayClass
				if err := consumer.ControllerClient.Get(ctx, types.NamespacedName{Name: virtualClass}, &class); err != nil {
					return nil, err
				}
				return meta.FindStatusCondition(class.Status.Conditions, string(gwv1.GatewayClassConditionStatusAccepted)), nil
			}, timeout, interval).Should(PointTo(HaveField("Status", metav1.ConditionTrue)))
		})

		It("the Gateway of the virtual class should be reflected to all providers, with the announced GatewayClass", func() {
			for i := range providers {
				provider := &providers[i]
				Eventually(func() (*gwv1.Gateway, error) {
					var gateway gwv1.Gateway
					err := provider.ControllerClient.Get(ctx, types.NamespacedName{Namespace: namespaceName, Name: gatewayName}, &gateway)
					return &gateway, err
				}, timeout, interval).Should(And(
					HaveField("Spec.GatewayClassName", BeEquivalentTo(providerClass)),
					// The reflection labels identify the origin and destination clusters of the reflected object.
					HaveField("ObjectMeta.Labels", And(
						HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(consumer.Cluster)),
						HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, string(provider.Cluster)),
					)),
				), "provider %s", provider.Cluster)
			}
		})

		It("the Gateway should report the aggregated status of the remote ones", func() {
			var addresses []string
			for i := range providers {
				provider := &providers[i]
				Eventually(func() (bool, error) {
					var gateway gwv1.Gateway
					if err := provider.ControllerClient.Get(ctx, types.NamespacedName{Namespace: namespaceName, Name: gatewayName}, &gateway); err != nil {
						return false, err
					}
					if !meta.IsStatusConditionTrue(gateway.Status.Conditions, string(gwv1.GatewayConditionProgrammed)) {
						return false, nil
					}
					for _, address := range gateway.Status.Addresses {
						addresses = append(addresses, address.Value)
					}
					return true, nil
				}, dataPlaneTimeout, interval).Should(BeTrue(), "provider %s", provider.Cluster)
			}

			Eventually(func(g Gomega) {
				var gateway gwv1.Gateway
				g.Expect(consumer.ControllerClient.Get(ctx, types.NamespacedName{Namespace: namespaceName, Name: gatewayName}, &gateway)).To(Succeed())

				programmed := meta.FindStatusCondition(gateway.Status.Conditions, string(gwv1.GatewayConditionProgrammed))
				g.Expect(programmed).ToNot(BeNil())
				g.Expect(programmed.Status).To(Equal(metav1.ConditionTrue))
				for i := range providers {
					provider := &providers[i]
					g.Expect(programmed.Message).To(ContainSubstring(string(provider.Cluster)))
				}

				local := make([]string, 0, len(gateway.Status.Addresses))
				for _, address := range gateway.Status.Addresses {
					local = append(local, address.Value)
				}
				g.Expect(local).To(ContainElements(addresses))

				var shadows offloadingv1beta1.ShadowGatewayStatusList
				g.Expect(consumer.ControllerClient.List(ctx, &shadows, client.InNamespace(namespaceName))).To(Succeed())
				g.Expect(shadows.Items).To(HaveLen(len(providers)))
			}, timeout, interval).Should(Succeed())
		})

		When("a route is attached to the Gateway of the virtual class", func() {
			const routeName, host = "web", "web.e2e.liqo.io"

			BeforeAll(func() {
				Expect(util.EnforceHTTPRoute(ctx, consumer.ControllerClient, namespaceName, routeName,
					[]gwv1.ParentReference{{Name: gatewayName, SectionName: ptr.To[gwv1.SectionName](util.GatewayListenerName)}}, host, backendName)).To(Succeed())
			})

			It("should be reflected to all providers, attached to the reflected Gateway", func() {
				for i := range providers {
					provider := &providers[i]
					Eventually(func() (*gwv1.HTTPRoute, error) { return remoteRoute(provider, routeName) }, timeout, interval).
						Should(HaveField("Spec.ParentRefs", ConsistOf(And(
							HaveField("Name", BeEquivalentTo(gatewayName)),
							HaveField("SectionName", PointTo(BeEquivalentTo(util.GatewayListenerName))),
						))), "provider %s", provider.Cluster)
				}
			})

			It("should be accepted in all providers", func() {
				Eventually(func() (*metav1.Condition, error) { return acceptedCondition(routeName) }, dataPlaneTimeout, interval).
					Should(PointTo(HaveField("Status", metav1.ConditionTrue)))

				condition, err := acceptedCondition(routeName)
				Expect(err).ToNot(HaveOccurred())
				for i := range providers {
					provider := &providers[i]
					Expect(condition.Message).To(ContainSubstring(string(provider.Cluster)))
				}
			})

			It("should serve the requests in all providers", func() {
				for i := range providers {
					provider := &providers[i]
					Eventually(func() (int, error) { return requestThroughGateway(provider, namespaceName, gatewayName, host) },
						dataPlaneTimeout, interval).Should(Equal(http.StatusOK), "provider %s", provider.Cluster)
				}
			})
		})

		When("a route is attached to a Gateway of another class", func() {
			const routeName, host = "shared", "shared.e2e.liqo.io"

			BeforeAll(func() {
				Expect(util.EnforceHTTPRoute(ctx, consumer.ControllerClient, namespaceName, routeName,
					[]gwv1.ParentReference{{Name: "edge", Namespace: ptr.To(gwv1.Namespace(localNamespaceName))}}, host, backendName)).To(Succeed())
			})

			It("should be reflected to all providers, attached to the shared Gateway", func() {
				for i := range providers {
					provider := &providers[i]
					Eventually(func() (*gwv1.HTTPRoute, error) { return remoteRoute(provider, routeName) }, timeout, interval).
						Should(HaveField("Spec.ParentRefs", ConsistOf(And(
							HaveField("Name", BeEquivalentTo(sharedGatewayName)),
							HaveField("Namespace", PointTo(BeEquivalentTo(sharedGatewayNamespace))),
						))), "provider %s", provider.Cluster)
				}
			})

			It("should serve the requests through the shared Gateway in all providers", func() {
				for i := range providers {
					provider := &providers[i]
					Eventually(func() (int, error) {
						return requestThroughGateway(provider, sharedGatewayNamespace, sharedGatewayName, host)
					}, dataPlaneTimeout, interval).Should(Equal(http.StatusOK), "provider %s", provider.Cluster)
				}
			})
		})

		When("a Gateway of the virtual class is mapped to the shared Gateway", func() {
			const mappedGateway, routeName, host = "mapped", "mapped", "mapped.e2e.liqo.io"

			BeforeAll(func() {
				Expect(util.EnforceGateway(ctx, consumer.ControllerClient, namespaceName, mappedGateway, virtualClass,
					util.WithGatewayAnnotations(map[string]string{consts.RemoteGatewayModeAnnotation: consts.RemoteGatewayModeShared}))).To(Succeed())
				Expect(util.EnforceHTTPRoute(ctx, consumer.ControllerClient, namespaceName, routeName,
					[]gwv1.ParentReference{{Name: mappedGateway, SectionName: ptr.To[gwv1.SectionName](util.GatewayListenerName)}},
					host, backendName)).To(Succeed())
			})

			It("should attach the reflected route to the shared Gateway, annotated with its addresses", func() {
				for i := range providers {
					provider := &providers[i]
					Eventually(func() (*gwv1.HTTPRoute, error) { return remoteRoute(provider, routeName) }, dataPlaneTimeout, interval).
						Should(And(
							HaveField("Spec.ParentRefs", ConsistOf(And(
								HaveField("Name", BeEquivalentTo(sharedGatewayName)),
								HaveField("Namespace", PointTo(BeEquivalentTo(sharedGatewayNamespace))),
								HaveField("SectionName", BeNil()),
							))),
							HaveField("ObjectMeta.Annotations", HaveKey(consts.SharedGatewayAddressesAnnotation)),
						), "provider %s", provider.Cluster)
				}
			})

			It("should not reflect the Gateway to any provider", func() {
				for i := range providers {
					provider := &providers[i]
					Expect(util.GetResource(ctx, provider.ControllerClient,
						&gwv1.Gateway{ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: mappedGateway}})).
						To(BeNotFound(), "provider %s", provider.Cluster)
				}
			})

			It("should report the addresses of the shared Gateways of all providers", func() {
				var addresses []string
				for i := range providers {
					provider := &providers[i]
					var shared gwv1.Gateway
					Expect(provider.ControllerClient.Get(ctx, types.NamespacedName{Namespace: sharedGatewayNamespace, Name: sharedGatewayName},
						&shared)).To(Succeed())
					for _, address := range shared.Status.Addresses {
						addresses = append(addresses, address.Value)
					}
				}
				Expect(addresses).ToNot(BeEmpty())

				Eventually(func(g Gomega) {
					var gateway gwv1.Gateway
					g.Expect(consumer.ControllerClient.Get(ctx, types.NamespacedName{Namespace: namespaceName, Name: mappedGateway}, &gateway)).To(Succeed())
					g.Expect(meta.IsStatusConditionTrue(gateway.Status.Conditions, string(gwv1.GatewayConditionProgrammed))).To(BeTrue())

					local := make([]string, 0, len(gateway.Status.Addresses))
					for _, address := range gateway.Status.Addresses {
						local = append(local, address.Value)
					}
					g.Expect(local).To(ConsistOf(addresses))
				}, timeout, interval).Should(Succeed())
			})

			It("should serve the requests through the shared Gateway in all providers", func() {
				for i := range providers {
					provider := &providers[i]
					Eventually(func() (int, error) {
						return requestThroughGateway(provider, sharedGatewayNamespace, sharedGatewayName, host)
					}, dataPlaneTimeout, interval).Should(Equal(http.StatusOK), "provider %s", provider.Cluster)
				}
			})
		})

		When("a route is rejected by a single provider", func() {
			const routeName, host = "rejected", "rejected.e2e.liqo.io"

			BeforeAll(func() {
				Expect(util.EnforceHTTPRoute(ctx, consumer.ControllerClient, namespaceName, routeName,
					[]gwv1.ParentReference{{Name: gatewayName}}, host, backendName,
					util.WithRouteLabels(map[string]string{rejectLabel: "true"}))).To(Succeed())
			})

			It("should report the failure in the status, identifying the provider rejecting it", func() {
				Eventually(func() (*metav1.Condition, error) { return acceptedCondition(routeName) }, timeout, interval).
					Should(PointTo(And(
						HaveField("Status", metav1.ConditionFalse),
						HaveField("Reason", forge.ConditionReasonReflectionFailed),
						HaveField("Message", ContainSubstring(fmt.Sprintf("cluster %q: reflection failed", providers[0].Cluster))),
						HaveField("Message", ContainSubstring("route rejected by the e2e admission policy")),
					)))

				condition, err := acceptedCondition(routeName)
				Expect(err).ToNot(HaveOccurred())
				for i := range providers[1:] {
					provider := &providers[1:][i]
					Expect(condition.Message).ToNot(ContainSubstring(string(provider.Cluster)))
				}
			})

			It("should generate a warning event identifying the provider rejecting it", func() {
				Eventually(func() []string { return failedReflectionEvents("HTTPRoute", routeName) }, timeout, interval).
					Should(ContainElement(ContainSubstring(string(providers[0].Cluster))))
			})

			It("should be reflected to the other providers only", func() {
				for i := range providers[1:] {
					provider := &providers[1:][i]
					Eventually(func() error { return util.Second(remoteRoute(provider, routeName)) }, timeout, interval).
						Should(Succeed(), "provider %s", provider.Cluster)
				}
				Expect(util.Second(remoteRoute(&providers[0], routeName))).To(BeNotFound())
			})

			It("should serve the requests only in the providers where it is reflected", func() {
				for i := range providers[1:] {
					provider := &providers[1:][i]
					Eventually(func() (int, error) { return requestThroughGateway(provider, namespaceName, gatewayName, host) },
						dataPlaneTimeout, interval).Should(Equal(http.StatusOK), "provider %s", provider.Cluster)
				}
				Expect(requestThroughGateway(&providers[0], namespaceName, gatewayName, host)).To(Equal(http.StatusNotFound))
			})
		})

		When("a route cannot be reflected without altering its semantic", func() {
			const routeName, host = "protected", "protected.e2e.liqo.io"

			BeforeAll(func() {
				Expect(util.EnforceHTTPRoute(ctx, consumer.ControllerClient, namespaceName, routeName,
					[]gwv1.ParentReference{{Name: gatewayName}}, host, backendName, util.WithExtensionRefFilter())).To(Succeed())
			})

			It("should report the failure in the status, for all providers", func() {
				Eventually(func(g Gomega) {
					condition, err := acceptedCondition(routeName)
					g.Expect(err).ToNot(HaveOccurred())
					g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
					g.Expect(condition.Reason).To(Equal(forge.ConditionReasonReflectionFailed))
					for i := range providers {
						provider := &providers[i]
						g.Expect(condition.Message).To(ContainSubstring(
							fmt.Sprintf("cluster %q: not reflected: unsupported ExtensionRef filter", provider.Cluster)))
					}
				}, timeout, interval).Should(Succeed())
			})

			It("should generate a warning event for all providers", func() {
				for i := range providers {
					provider := &providers[i]
					Eventually(func() []string { return failedReflectionEvents("HTTPRoute", routeName) }, timeout, interval).
						Should(ContainElement(ContainSubstring(string(provider.Cluster))), "provider %s", provider.Cluster)
				}
			})

			It("should not be reflected to any provider", func() {
				for i := range providers {
					provider := &providers[i]
					Expect(util.Second(remoteRoute(provider, routeName))).To(BeNotFound(), "provider %s", provider.Cluster)
				}
			})
		})

		When("a route of a kind not supported by the reflection is created", func() {
			const routeName = "tls"

			BeforeAll(func() {
				Expect(util.EnforceTLSRoute(ctx, consumer.ControllerClient, namespaceName, routeName,
					[]gwv1.ParentReference{{Name: gatewayName}}, "tls.e2e.liqo.io", backendName)).To(Succeed())
			})

			It("should generate a warning event for all providers", func() {
				for i := range providers {
					provider := &providers[i]
					Eventually(func() []string { return failedReflectionEvents("TLSRoute", routeName) }, timeout, interval).
						Should(ContainElement(And(
							ContainSubstring(string(provider.Cluster)),
							ContainSubstring("TLSRoute resources are not supported by the Liqo reflection"),
						)), "provider %s", provider.Cluster)
				}
			})
		})

		When("the local resources are deleted", func() {
			BeforeAll(func() {
				Expect(util.DeleteResource(ctx, consumer.ControllerClient,
					&gwv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: "web"}})).To(Succeed())
				Expect(util.DeleteResource(ctx, consumer.ControllerClient,
					&gwv1.Gateway{ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: gatewayName}})).To(Succeed())
			})

			It("should delete the reflected resources and the corresponding proxies", func() {
				for i := range providers {
					provider := &providers[i]
					Eventually(func() error { return util.Second(remoteRoute(provider, "web")) }, timeout, interval).
						Should(BeNotFound(), "provider %s", provider.Cluster)
					Eventually(func() error {
						return util.GetResource(ctx, provider.ControllerClient,
							&gwv1.Gateway{ObjectMeta: metav1.ObjectMeta{Namespace: namespaceName, Name: gatewayName}})
					}, timeout, interval).Should(BeNotFound(), "provider %s", provider.Cluster)
					Eventually(func() error {
						_, _, err := util.EnvoyProxyTarget(ctx, provider.ControllerClient, namespaceName, gatewayName, 80)
						return err
					}, dataPlaneTimeout, interval).Should(HaveOccurred(), "provider %s", provider.Cluster)
				}
			})

			It("should delete the shadow resources reporting their status", func() {
				Eventually(func(g Gomega) {
					var gatewayShadows offloadingv1beta1.ShadowGatewayStatusList
					g.Expect(consumer.ControllerClient.List(ctx, &gatewayShadows, client.InNamespace(namespaceName))).To(Succeed())
					for i := range gatewayShadows.Items {
						g.Expect(gatewayShadows.Items[i].Spec.GatewayName).ToNot(Equal(gatewayName))
					}

					var routeShadows offloadingv1beta1.ShadowRouteStatusList
					g.Expect(consumer.ControllerClient.List(ctx, &routeShadows, client.InNamespace(namespaceName))).To(Succeed())
					for i := range routeShadows.Items {
						g.Expect(routeShadows.Items[i].Spec.RouteName).ToNot(Equal("web"))
					}
				}, timeout, interval).Should(Succeed())
			})
		})
	})
})
