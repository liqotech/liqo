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
	widgetSpecValue   = "hello-from-consumer"
	widgetStatusPhase = "Ready"
)

var (
	gvr = schema.GroupVersionResource{Group: "example.io", Version: "v1", Resource: "widgets"}
	gvk = schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Widget"}

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

func newWidget(name string, allowReflection bool, value string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName(name)
	obj.SetNamespace(namespaceName)
	if allowReflection {
		obj.SetAnnotations(map[string]string{
			consts.AllowReflectionAnnotationKey: "true",
		})
	}
	Expect(unstructured.SetNestedField(obj.Object, value, "spec", "value")).To(Succeed())
	return obj
}

func getWidget(dyn dynamic.Interface, name string) (*unstructured.Unstructured, error) {
	return dyn.Resource(gvr).Namespace(namespaceName).Get(ctx, name, metav1.GetOptions{})
}

func createWidget(dyn dynamic.Interface, obj *unstructured.Unstructured) *unstructured.Unstructured {
	created, err := dyn.Resource(gvr).Namespace(namespaceName).Create(ctx, obj, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	return created
}

func ensureWidgetDeleted(name string) {
	err := consumerDyn.Resource(gvr).Namespace(namespaceName).Delete(ctx, name, metav1.DeleteOptions{})
	Expect(ignoreNotFound(err)).To(Succeed())

	Eventually(func() error {
		_, err := getWidget(consumerDyn, name)
		return err
	}, timeout, interval).Should(BeNotFound())

	for _, provider := range providers {
		Eventually(func() error {
			_, err := getWidget(providerDyn[provider.Cluster], name)
			return err
		}, timeout, interval).Should(BeNotFound())
	}
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
				createWidget(consumerDyn, newWidget(widgetName, true, widgetSpecValue))
			})

			AfterEach(func() {
				ensureWidgetDeleted(widgetName)
			})

			It("Should reflect the local spec to the remote provider clusters", func() {
				for _, provider := range providers {
					Eventually(func(g Gomega) {
						remote, err := getWidget(providerDyn[provider.Cluster], widgetName)
						g.Expect(err).ToNot(HaveOccurred())
						g.Expect(remote.GetLabels()).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(consumer.Cluster)))
						g.Expect(remote.GetLabels()).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, string(provider.Cluster)))

						value, found, err := unstructured.NestedString(remote.Object, "spec", "value")
						g.Expect(err).ToNot(HaveOccurred())
						g.Expect(found).To(BeTrue())
						g.Expect(value).To(Equal(widgetSpecValue))
					}, timeout, interval).Should(Succeed())
				}
			})

			It("Should sync the remote status back to the local Widget", func() {
				provider := providers[0]
				Eventually(func() error {
					_, err := getWidget(providerDyn[provider.Cluster], widgetName)
					return err
				}, timeout, interval).Should(Succeed())

				remote, err := getWidget(providerDyn[provider.Cluster], widgetName)
				Expect(err).ToNot(HaveOccurred())
				Expect(unstructured.SetNestedField(remote.Object, widgetStatusPhase, "status", "phase")).To(Succeed())
				_, err = providerDyn[provider.Cluster].Resource(gvr).Namespace(namespaceName).UpdateStatus(ctx, remote, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())

				Eventually(func(g Gomega) {
					local, err := getWidget(consumerDyn, widgetName)
					g.Expect(err).ToNot(HaveOccurred())
					phase, found, err := unstructured.NestedString(local.Object, "status", "phase")
					g.Expect(err).ToNot(HaveOccurred())
					g.Expect(found).To(BeTrue())
					g.Expect(phase).To(Equal(widgetStatusPhase))
				}, timeout, interval).Should(Succeed())
			})

			It("Should delete the remote twin when the local Widget is deleted", func() {
				for _, provider := range providers {
					Eventually(func() error {
						_, err := getWidget(providerDyn[provider.Cluster], widgetName)
						return err
					}, timeout, interval).Should(Succeed())
				}

				ensureWidgetDeleted(widgetName)
			})
		})

		When("Creating a Widget without the allow-reflection annotation", func() {
			BeforeEach(func() {
				createWidget(consumerDyn, newWidget(widgetSkippedName, false, widgetSpecValue))
			})

			AfterEach(func() {
				err := consumerDyn.Resource(gvr).Namespace(namespaceName).Delete(ctx, widgetSkippedName, metav1.DeleteOptions{})
				Expect(ignoreNotFound(err)).To(Succeed())
				Eventually(func() error {
					_, err := getWidget(consumerDyn, widgetSkippedName)
					return err
				}, timeout, interval).Should(BeNotFound())
			})

			It("Should not reflect the Widget to remote provider clusters", func() {
				for _, provider := range providers {
					Consistently(func() error {
						_, err := getWidget(providerDyn[provider.Cluster], widgetSkippedName)
						return err
					}, shortTimeout, interval).Should(BeNotFound())
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
