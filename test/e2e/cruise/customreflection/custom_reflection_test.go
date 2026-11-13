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

package customreflection

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/test/e2e/testutils/config"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 2
	// testName is the name of this E2E test.
	testName = "CUSTOM_REFLECTION"

	widgetName        = "test-widget"
	widgetSkippedName = "test-widget-skipped"
	specValue         = "hello-from-consumer"
	updatedSpecValue  = "updated-from-consumer"
	statusPhase       = "Ready"

	gadgetName = "test-gadget"

	clusterWidgetName        = "test-cluster-widget"
	clusterWidgetSkippedName = "test-cluster-widget-skipped"

	foreignAnnotationKey   = "test.liqo.io/foreign-annotation"
	foreignAnnotationValue = "added-remotely"
	foreignLabelKey        = "test.liqo.io/foreign-label"
	foreignLabelValue      = "added-remotely"
	localAnnotationKey     = "test.liqo.io/local-annotation"
	localAnnotationValue   = "added-locally"

	// The not-reflected metadata, matching the offloading.reflection.skip values
	// configured by the e2e pipeline installer.
	notReflectedLabelKey      = "test.liqo.io/not-reflected-label"
	notReflectedAnnotationKey = "test.liqo.io/not-reflected-annotation"
	notReflectedValue         = "must-not-be-reflected"
)

var (
	// The widget GVR is configured with statusSync=true, hence the remote status is
	// synchronized back to the local object.
	widgetGVR = schema.GroupVersionResource{Group: "example.io", Version: "v1", Resource: "widgets"}
	widgetGVK = schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Widget"}

	// The gadget GVR is configured with statusSync=false, hence only the spec is
	// reflected and the remote status is not synchronized back to the local object.
	gadgetGVR = schema.GroupVersionResource{Group: "other.io", Version: "v1", Resource: "gadgets"}
	gadgetGVK = schema.GroupVersionKind{Group: "other.io", Version: "v1", Kind: "Gadget"}

	// The clusterwidget GVR is cluster-scoped and configured with statusSync=true,
	// to e2e test the reflection of cluster-scoped custom resources: a single
	// reflector instance per remote cluster runs for the whole peering lifetime,
	// independently of the offloaded namespaces.
	clusterWidgetGVR = schema.GroupVersionResource{Group: "example.io", Version: "v1", Resource: "clusterwidgets"}
	clusterWidgetGVK = schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "ClusterWidget"}

	ctx           = context.Background()
	testContext   = tester.GetTester(ctx)
	interval      = config.Interval
	timeout       = config.Timeout
	shortTimeout  = config.TimeoutConsistently
	namespaceName = util.GetNameNamespaceTest(testName)
	indexCons     = 0
	consumer      = testContext.Clusters[indexCons]
	providers     = tester.GetProviders(testContext.Clusters)

	consumerDyn dynamic.Interface
	providerDyn = make(map[liqov1beta1.ClusterID]dynamic.Interface)
)

func TestE2E(t *testing.T) {
	util.CheckIfTestIsSkipped(t, clustersRequired, testName)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Custom Resource Reflection Suite")
}

func ignoreNotFound(err error) error {
	if kerrors.IsNotFound(err) {
		return nil
	}
	return err
}

func newCR(gvk schema.GroupVersionKind, name string, allowReflection bool) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName(name)
	obj.SetNamespace(namespaceName)
	obj.SetLabels(map[string]string{notReflectedLabelKey: notReflectedValue})
	annotations := map[string]string{notReflectedAnnotationKey: notReflectedValue}
	if allowReflection {
		annotations[consts.AllowReflectionAnnotationKey] = "true"
	}
	obj.SetAnnotations(annotations)
	Expect(unstructured.SetNestedField(obj.Object, specValue, "spec", "value")).To(Succeed())
	return obj
}

func getCR(dyn dynamic.Interface, gvr schema.GroupVersionResource, name string) (*unstructured.Unstructured, error) {
	return dyn.Resource(gvr).Namespace(namespaceName).Get(ctx, name, metav1.GetOptions{})
}

func createCR(dyn dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) *unstructured.Unstructured {
	created, err := dyn.Resource(gvr).Namespace(namespaceName).Create(ctx, obj, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	return created
}

func updateCR(dyn dynamic.Interface, gvr schema.GroupVersionResource, name string,
	mutate func(*unstructured.Unstructured), subresources ...string) {
	obj, err := getCR(dyn, gvr, name)
	Expect(err).ToNot(HaveOccurred())
	mutate(obj)
	_, err = dyn.Resource(gvr).Namespace(namespaceName).Update(ctx, obj, metav1.UpdateOptions{}, subresources...)
	Expect(err).ToNot(HaveOccurred())
}

func ensureCRDeleted(gvr schema.GroupVersionResource, name string) {
	err := consumerDyn.Resource(gvr).Namespace(namespaceName).Delete(ctx, name, metav1.DeleteOptions{})
	Expect(ignoreNotFound(err)).To(Succeed())

	Eventually(func() error {
		_, err := getCR(consumerDyn, gvr, name)
		return err
	}, timeout, interval).Should(BeNotFound())

	for _, provider := range providers {
		Eventually(func() error {
			_, err := getCR(providerDyn[provider.Cluster], gvr, name)
			return err
		}, timeout, interval).Should(BeNotFound())
	}
}

func newClusterScopedCR(gvk schema.GroupVersionKind, name string, allowReflection bool) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName(name)
	obj.SetLabels(map[string]string{notReflectedLabelKey: notReflectedValue})
	annotations := map[string]string{notReflectedAnnotationKey: notReflectedValue}
	if allowReflection {
		annotations[consts.AllowReflectionAnnotationKey] = "true"
	}
	obj.SetAnnotations(annotations)
	Expect(unstructured.SetNestedField(obj.Object, specValue, "spec", "value")).To(Succeed())
	return obj
}

func getClusterScopedCR(dyn dynamic.Interface, gvr schema.GroupVersionResource, name string) (*unstructured.Unstructured, error) {
	return dyn.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
}

func createClusterScopedCR(dyn dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) *unstructured.Unstructured {
	created, err := dyn.Resource(gvr).Create(ctx, obj, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	return created
}

func updateClusterScopedCR(dyn dynamic.Interface, gvr schema.GroupVersionResource, name string,
	mutate func(*unstructured.Unstructured), subresources ...string) {
	obj, err := getClusterScopedCR(dyn, gvr, name)
	Expect(err).ToNot(HaveOccurred())
	mutate(obj)
	_, err = dyn.Resource(gvr).Update(ctx, obj, metav1.UpdateOptions{}, subresources...)
	Expect(err).ToNot(HaveOccurred())
}

func ensureClusterScopedCRDeleted(gvr schema.GroupVersionResource, name string) {
	err := consumerDyn.Resource(gvr).Delete(ctx, name, metav1.DeleteOptions{})
	Expect(ignoreNotFound(err)).To(Succeed())

	Eventually(func() error {
		_, err := getClusterScopedCR(consumerDyn, gvr, name)
		return err
	}, timeout, interval).Should(BeNotFound())

	for _, provider := range providers {
		Eventually(func() error {
			_, err := getClusterScopedCR(providerDyn[provider.Cluster], gvr, name)
			return err
		}, timeout, interval).Should(BeNotFound())
	}
}

// multiProviderStatusStability implements the multi-provider status synchronization scenario,
// shared by the namespaced and the cluster-scoped custom resource reflection tests. The
// create, ensureDeleted, get and update functions abstract the resource scope.
func multiProviderStatusStability(name string, createAnnotatedCR, ensureCRDeleted func(),
	getCRByScope func(dyn dynamic.Interface, resourceName string) (*unstructured.Unstructured, error),
	updateCRByScope func(dyn dynamic.Interface, resourceName string, mutate func(*unstructured.Unstructured), subresources ...string)) {
	BeforeEach(func() {
		if len(providers) < 2 {
			Skip("this scenario requires multiple providers")
		}
		createAnnotatedCR()
	})

	AfterEach(func() { ensureCRDeleted() })

	It("Should sync the status from the provider running the controller and keep it stable", func() {
		By("waiting for the remote twins to be created on all providers")
		for _, provider := range providers {
			Eventually(func() error {
				_, err := getCRByScope(providerDyn[provider.Cluster], name)
				return err
			}, timeout, interval).Should(Succeed())
		}

		By("setting the status on a single provider's twin, simulating the controller running on one side only")
		updateCRByScope(providerDyn[providers[0].Cluster], name, func(obj *unstructured.Unstructured) {
			Expect(unstructured.SetNestedField(obj.Object, statusPhase, "status", "phase")).To(Succeed())
		}, "status")

		By("waiting for the status to be synced back to the local object")
		Eventually(func(g Gomega) {
			local, err := getCRByScope(consumerDyn, name)
			g.Expect(err).ToNot(HaveOccurred())
			phase, found, err := unstructured.NestedString(local.Object, "status", "phase")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(phase).To(Equal(statusPhase))
		}, timeout, interval).Should(Succeed())

		By("checking that the status remains stable, as the reflectors of the other providers must not overwrite it")
		Consistently(func(g Gomega) {
			local, err := getCRByScope(consumerDyn, name)
			g.Expect(err).ToNot(HaveOccurred())
			phase, found, err := unstructured.NestedString(local.Object, "status", "phase")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(phase).To(Equal(statusPhase))
		}, shortTimeout, interval).Should(Succeed())
	})
}

var _ = BeforeSuite(func() {
	Expect(consumer.Role).To(Equal(liqov1beta1.ConsumerRole))

	var err error
	consumerDyn, err = dynamic.NewForConfig(consumer.Config)
	Expect(err).ToNot(HaveOccurred())
	for i := range providers {
		provider := &providers[i]
		dyn, err := dynamic.NewForConfig(provider.Config)
		Expect(err).ToNot(HaveOccurred())
		providerDyn[provider.Cluster] = dyn
	}

	Expect(util.Second(util.EnforceNamespace(ctx, consumer.NativeClient,
		consumer.Cluster, namespaceName))).To(Succeed())

	Expect(util.OffloadNamespace(consumer.KubeconfigPath, namespaceName,
		"--namespace-mapping-strategy", string(offloadingv1beta1.EnforceSameNameMappingStrategyType),
		"--pod-offloading-strategy", string(offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType),
	)).To(Succeed())
	// Wait for the namespace to be offloaded; avoids race conditions with Virtual Kubelet startup.
	time.Sleep(2 * time.Second)
})

var _ = Describe("Liqo E2E", func() {
	Context("Custom resource reflection", func() {

		When("Creating an annotated Widget on the consumer", func() {
			BeforeEach(func() {
				createCR(consumerDyn, widgetGVR, newCR(widgetGVK, widgetName, true))
			})

			AfterEach(func() {
				ensureCRDeleted(widgetGVR, widgetName)
			})

			It("Should reflect the local spec to the remote provider clusters", func() {
				for _, provider := range providers {
					Eventually(func(g Gomega) {
						remote, err := getCR(providerDyn[provider.Cluster], widgetGVR, widgetName)
						g.Expect(err).ToNot(HaveOccurred())
						g.Expect(remote.GetLabels()).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(consumer.Cluster)))
						g.Expect(remote.GetLabels()).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, string(provider.Cluster)))
						g.Expect(remote.GetLabels()).ToNot(HaveKey(notReflectedLabelKey))
						g.Expect(remote.GetAnnotations()).ToNot(HaveKey(notReflectedAnnotationKey))

						value, found, err := unstructured.NestedString(remote.Object, "spec", "value")
						g.Expect(err).ToNot(HaveOccurred())
						g.Expect(found).To(BeTrue())
						g.Expect(value).To(Equal(specValue))
					}, timeout, interval).Should(Succeed())
				}
			})

			It("Should sync the remote status back to the local Widget", func() {
				Expect(providers).ToNot(BeEmpty())
				provider := providers[0]
				Eventually(func() error {
					_, err := getCR(providerDyn[provider.Cluster], widgetGVR, widgetName)
					return err
				}, timeout, interval).Should(Succeed())

				updateCR(providerDyn[provider.Cluster], widgetGVR, widgetName, func(obj *unstructured.Unstructured) {
					Expect(unstructured.SetNestedField(obj.Object, statusPhase, "status", "phase")).To(Succeed())
				}, "status")

				Eventually(func(g Gomega) {
					local, err := getCR(consumerDyn, widgetGVR, widgetName)
					g.Expect(err).ToNot(HaveOccurred())
					phase, found, err := unstructured.NestedString(local.Object, "status", "phase")
					g.Expect(err).ToNot(HaveOccurred())
					g.Expect(found).To(BeTrue())
					g.Expect(phase).To(Equal(statusPhase))
				}, timeout, interval).Should(Succeed())
			})

			It("Should delete the remote twin when the local Widget is deleted", func() {
				for _, provider := range providers {
					Eventually(func() error {
						_, err := getCR(providerDyn[provider.Cluster], widgetGVR, widgetName)
						return err
					}, timeout, interval).Should(Succeed())
				}

				ensureCRDeleted(widgetGVR, widgetName)
			})

			It("Should preserve annotations and labels added on the remote twin by other controllers", func() {
				By("waiting for the remote twin to be created")
				provider := providers[0]
				Eventually(func() error {
					_, err := getCR(providerDyn[provider.Cluster], widgetGVR, widgetName)
					return err
				}, timeout, interval).Should(Succeed())

				By("adding a foreign annotation and label on the remote twin, simulating another controller")
				updateCR(providerDyn[provider.Cluster], widgetGVR, widgetName, func(obj *unstructured.Unstructured) {
					annotations := obj.GetAnnotations()
					annotations[foreignAnnotationKey] = foreignAnnotationValue
					obj.SetAnnotations(annotations)
					labels := obj.GetLabels()
					labels[foreignLabelKey] = foreignLabelValue
					obj.SetLabels(labels)
				})

				By("modifying the local spec, forcing the reflector to reconcile the remote twin")
				updateCR(consumerDyn, widgetGVR, widgetName, func(obj *unstructured.Unstructured) {
					Expect(unstructured.SetNestedField(obj.Object, updatedSpecValue, "spec", "value")).To(Succeed())
				})

				By("checking that the remote twin is updated while preserving the remote-added metadata")
				Eventually(func(g Gomega) {
					remote, err := getCR(providerDyn[provider.Cluster], widgetGVR, widgetName)
					g.Expect(err).ToNot(HaveOccurred())

					value, found, err := unstructured.NestedString(remote.Object, "spec", "value")
					g.Expect(err).ToNot(HaveOccurred())
					g.Expect(found).To(BeTrue())
					g.Expect(value).To(Equal(updatedSpecValue))

					g.Expect(remote.GetAnnotations()).To(HaveKeyWithValue(foreignAnnotationKey, foreignAnnotationValue))
					g.Expect(remote.GetLabels()).To(HaveKeyWithValue(foreignLabelKey, foreignLabelValue))
				}, timeout, interval).Should(Succeed())
			})

			It("Should reflect annotation additions and removals from the local object", func() {
				By("waiting for the remote twin to be created")
				provider := providers[0]
				Eventually(func() error {
					_, err := getCR(providerDyn[provider.Cluster], widgetGVR, widgetName)
					return err
				}, timeout, interval).Should(Succeed())

				By("adding an annotation to the local object")
				updateCR(consumerDyn, widgetGVR, widgetName, func(obj *unstructured.Unstructured) {
					annotations := obj.GetAnnotations()
					annotations[localAnnotationKey] = localAnnotationValue
					obj.SetAnnotations(annotations)
				})

				By("waiting for the annotation to be reflected on the remote twin")
				Eventually(func(g Gomega) {
					remote, err := getCR(providerDyn[provider.Cluster], widgetGVR, widgetName)
					g.Expect(err).ToNot(HaveOccurred())
					g.Expect(remote.GetAnnotations()).To(HaveKeyWithValue(localAnnotationKey, localAnnotationValue))
				}, timeout, interval).Should(Succeed())

				By("removing the annotation from the local object")
				updateCR(consumerDyn, widgetGVR, widgetName, func(obj *unstructured.Unstructured) {
					annotations := obj.GetAnnotations()
					delete(annotations, localAnnotationKey)
					obj.SetAnnotations(annotations)
				})

				By("checking that the annotation is removed from the remote twin")
				Eventually(func(g Gomega) {
					remote, err := getCR(providerDyn[provider.Cluster], widgetGVR, widgetName)
					g.Expect(err).ToNot(HaveOccurred())
					g.Expect(remote.GetAnnotations()).ToNot(HaveKey(localAnnotationKey))
				}, timeout, interval).Should(Succeed())
			})
		})

		When("Creating a Widget without the allow-reflection annotation", func() {
			BeforeEach(func() {
				createCR(consumerDyn, widgetGVR, newCR(widgetGVK, widgetSkippedName, false))
			})

			AfterEach(func() {
				err := consumerDyn.Resource(widgetGVR).Namespace(namespaceName).Delete(ctx, widgetSkippedName, metav1.DeleteOptions{})
				Expect(ignoreNotFound(err)).To(Succeed())
				Eventually(func() error {
					_, err := getCR(consumerDyn, widgetGVR, widgetSkippedName)
					return err
				}, timeout, interval).Should(BeNotFound())
			})

			It("Should not reflect the Widget to remote provider clusters", func() {
				for _, provider := range providers {
					Consistently(func() error {
						_, err := getCR(providerDyn[provider.Cluster], widgetGVR, widgetSkippedName)
						return err
					}, shortTimeout, interval).Should(BeNotFound())
				}
			})
		})

		When("the namespace is offloaded to multiple providers", func() {
			multiProviderStatusStability(widgetName,
				func() { createCR(consumerDyn, widgetGVR, newCR(widgetGVK, widgetName, true)) },
				func() { ensureCRDeleted(widgetGVR, widgetName) },
				func(dyn dynamic.Interface, name string) (*unstructured.Unstructured, error) {
					return getCR(dyn, widgetGVR, name)
				},
				func(dyn dynamic.Interface, name string, mutate func(*unstructured.Unstructured), subresources ...string) {
					updateCR(dyn, widgetGVR, name, mutate, subresources...)
				})
		})

		When("the statusSync option is disabled for a custom resource", func() {
			BeforeEach(func() {
				createCR(consumerDyn, gadgetGVR, newCR(gadgetGVK, gadgetName, true))
			})

			AfterEach(func() {
				ensureCRDeleted(gadgetGVR, gadgetName)
			})

			It("Should reflect the spec without syncing the status", func() {
				By("waiting for the remote twins to be created on all providers, with the local spec")
				for _, provider := range providers {
					Eventually(func(g Gomega) {
						remote, err := getCR(providerDyn[provider.Cluster], gadgetGVR, gadgetName)
						g.Expect(err).ToNot(HaveOccurred())

						value, found, err := unstructured.NestedString(remote.Object, "spec", "value")
						g.Expect(err).ToNot(HaveOccurred())
						g.Expect(found).To(BeTrue())
						g.Expect(value).To(Equal(specValue))
					}, timeout, interval).Should(Succeed())
				}

				By("setting the status on the remote twins")
				for _, provider := range providers {
					updateCR(providerDyn[provider.Cluster], gadgetGVR, gadgetName, func(obj *unstructured.Unstructured) {
						Expect(unstructured.SetNestedField(obj.Object, statusPhase, "status", "phase")).To(Succeed())
					}, "status")
				}

				By("checking that the status is not synced back to the local Gadget")
				Consistently(func(g Gomega) {
					local, err := getCR(consumerDyn, gadgetGVR, gadgetName)
					g.Expect(err).ToNot(HaveOccurred())
					_, found, err := unstructured.NestedString(local.Object, "status", "phase")
					g.Expect(err).ToNot(HaveOccurred())
					g.Expect(found).To(BeFalse())
				}, shortTimeout, interval).Should(Succeed())
			})
		})

		Context("Cluster-scoped custom resource reflection", func() {
			When("Creating an annotated ClusterWidget on the consumer", func() {
				BeforeEach(func() {
					createClusterScopedCR(consumerDyn, clusterWidgetGVR, newClusterScopedCR(clusterWidgetGVK, clusterWidgetName, true))
				})

				AfterEach(func() {
					ensureClusterScopedCRDeleted(clusterWidgetGVR, clusterWidgetName)
				})

				It("Should reflect the local spec to the remote provider clusters", func() {
					for _, provider := range providers {
						Eventually(func(g Gomega) {
							remote, err := getClusterScopedCR(providerDyn[provider.Cluster], clusterWidgetGVR, clusterWidgetName)
							g.Expect(err).ToNot(HaveOccurred())
							g.Expect(remote.GetNamespace()).To(BeEmpty())
							g.Expect(remote.GetLabels()).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(consumer.Cluster)))
							g.Expect(remote.GetLabels()).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, string(provider.Cluster)))
							g.Expect(remote.GetLabels()).ToNot(HaveKey(notReflectedLabelKey))
							g.Expect(remote.GetAnnotations()).ToNot(HaveKey(notReflectedAnnotationKey))

							value, found, err := unstructured.NestedString(remote.Object, "spec", "value")
							g.Expect(err).ToNot(HaveOccurred())
							g.Expect(found).To(BeTrue())
							g.Expect(value).To(Equal(specValue))
						}, timeout, interval).Should(Succeed())
					}
				})

				It("Should sync the remote status back to the local ClusterWidget", func() {
					Expect(providers).ToNot(BeEmpty())
					provider := providers[0]
					Eventually(func() error {
						_, err := getClusterScopedCR(providerDyn[provider.Cluster], clusterWidgetGVR, clusterWidgetName)
						return err
					}, timeout, interval).Should(Succeed())

					updateClusterScopedCR(providerDyn[provider.Cluster], clusterWidgetGVR, clusterWidgetName, func(obj *unstructured.Unstructured) {
						Expect(unstructured.SetNestedField(obj.Object, statusPhase, "status", "phase")).To(Succeed())
					}, "status")

					Eventually(func(g Gomega) {
						local, err := getClusterScopedCR(consumerDyn, clusterWidgetGVR, clusterWidgetName)
						g.Expect(err).ToNot(HaveOccurred())
						phase, found, err := unstructured.NestedString(local.Object, "status", "phase")
						g.Expect(err).ToNot(HaveOccurred())
						g.Expect(found).To(BeTrue())
						g.Expect(phase).To(Equal(statusPhase))
					}, timeout, interval).Should(Succeed())
				})

				It("Should delete the remote twin when the local ClusterWidget is deleted", func() {
					for _, provider := range providers {
						Eventually(func() error {
							_, err := getClusterScopedCR(providerDyn[provider.Cluster], clusterWidgetGVR, clusterWidgetName)
							return err
						}, timeout, interval).Should(Succeed())
					}

					ensureClusterScopedCRDeleted(clusterWidgetGVR, clusterWidgetName)
				})

				It("Should preserve annotations and labels added on the remote twin by other controllers", func() {
					By("waiting for the remote twin to be created")
					provider := providers[0]
					Eventually(func() error {
						_, err := getClusterScopedCR(providerDyn[provider.Cluster], clusterWidgetGVR, clusterWidgetName)
						return err
					}, timeout, interval).Should(Succeed())

					By("adding a foreign annotation and label on the remote twin, simulating another controller")
					updateClusterScopedCR(providerDyn[provider.Cluster], clusterWidgetGVR, clusterWidgetName, func(obj *unstructured.Unstructured) {
						annotations := obj.GetAnnotations()
						annotations[foreignAnnotationKey] = foreignAnnotationValue
						obj.SetAnnotations(annotations)
						labels := obj.GetLabels()
						labels[foreignLabelKey] = foreignLabelValue
						obj.SetLabels(labels)
					})

					By("modifying the local spec, forcing the reflector to reconcile the remote twin")
					updateClusterScopedCR(consumerDyn, clusterWidgetGVR, clusterWidgetName, func(obj *unstructured.Unstructured) {
						Expect(unstructured.SetNestedField(obj.Object, updatedSpecValue, "spec", "value")).To(Succeed())
					})

					By("checking that the remote twin is updated while preserving the remote-added metadata")
					Eventually(func(g Gomega) {
						remote, err := getClusterScopedCR(providerDyn[provider.Cluster], clusterWidgetGVR, clusterWidgetName)
						g.Expect(err).ToNot(HaveOccurred())

						value, found, err := unstructured.NestedString(remote.Object, "spec", "value")
						g.Expect(err).ToNot(HaveOccurred())
						g.Expect(found).To(BeTrue())
						g.Expect(value).To(Equal(updatedSpecValue))

						g.Expect(remote.GetAnnotations()).To(HaveKeyWithValue(foreignAnnotationKey, foreignAnnotationValue))
						g.Expect(remote.GetLabels()).To(HaveKeyWithValue(foreignLabelKey, foreignLabelValue))
					}, timeout, interval).Should(Succeed())
				})
			})

			When("Creating a ClusterWidget without the allow-reflection annotation", func() {
				BeforeEach(func() {
					createClusterScopedCR(consumerDyn, clusterWidgetGVR, newClusterScopedCR(clusterWidgetGVK, clusterWidgetSkippedName, false))
				})

				AfterEach(func() {
					err := consumerDyn.Resource(clusterWidgetGVR).Delete(ctx, clusterWidgetSkippedName, metav1.DeleteOptions{})
					Expect(ignoreNotFound(err)).To(Succeed())
					Eventually(func() error {
						_, err := getClusterScopedCR(consumerDyn, clusterWidgetGVR, clusterWidgetSkippedName)
						return err
					}, timeout, interval).Should(BeNotFound())
				})

				It("Should not reflect the ClusterWidget to remote provider clusters", func() {
					for _, provider := range providers {
						Consistently(func() error {
							_, err := getClusterScopedCR(providerDyn[provider.Cluster], clusterWidgetGVR, clusterWidgetSkippedName)
							return err
						}, shortTimeout, interval).Should(BeNotFound())
					}
				})
			})

			When("the consumer is peered with multiple providers", func() {
				multiProviderStatusStability(clusterWidgetName,
					func() {
						createClusterScopedCR(consumerDyn, clusterWidgetGVR, newClusterScopedCR(clusterWidgetGVK, clusterWidgetName, true))
					},
					func() { ensureClusterScopedCRDeleted(clusterWidgetGVR, clusterWidgetName) },
					func(dyn dynamic.Interface, name string) (*unstructured.Unstructured, error) {
						return getClusterScopedCR(dyn, clusterWidgetGVR, name)
					},
					func(dyn dynamic.Interface, name string, mutate func(*unstructured.Unstructured), subresources ...string) {
						updateClusterScopedCR(dyn, clusterWidgetGVR, name, mutate, subresources...)
					})
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
