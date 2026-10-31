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

package gatewayapi_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/trace"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	gwutils "github.com/liqotech/liqo/pkg/utils/gatewayapi"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/gatewayapi"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ = Describe("Unsupported routes", func() {
	const RouteName = "tcp"

	var (
		ctx    context.Context
		cancel context.CancelFunc

		route     gwutils.UnsupportedRoute
		dynClient dynamic.Interface
		reflector manager.NamespacedReflector
		events    chan *corev1.Event
		err       error
	)

	create := func(annotations map[string]string) {
		obj := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": route.GVR.GroupVersion().String(),
			"kind":       route.Kind,
			"metadata":   map[string]interface{}{"name": RouteName, "namespace": LocalNamespace},
			"spec": map[string]interface{}{
				"parentRefs": []interface{}{map[string]interface{}{"name": "web"}},
				"rules": []interface{}{map[string]interface{}{
					"backendRefs": []interface{}{map[string]interface{}{"name": "svc", "port": int64(80)}},
				}},
			},
		}}
		obj.SetAnnotations(annotations)
		_, errCreate := dynClient.Resource(route.GVR).Namespace(LocalNamespace).Create(ctx, obj, metav1.CreateOptions{})
		Expect(errCreate).ToNot(HaveOccurred())
	}

	setup := func() {
		broadcaster := record.NewBroadcaster()
		DeferCleanup(broadcaster.Shutdown)
		ch := events
		broadcaster.StartEventWatcher(func(event *corev1.Event) { ch <- event })

		factory := metadatainformer.NewSharedInformerFactory(metadata.NewForConfigOrDie(restConfig), 0)
		kubeClient := kubernetes.NewForConfigOrDie(restConfig)
		reflector = gatewayapi.NewNamespacedUnsupportedRouteReflector(route, factory)(options.NewNamespaced().
			WithLocal(LocalNamespace, kubeClient, nil).WithRemote(RemoteNamespace, kubeClient, nil).
			WithHandlerFactory(func(options.Keyer, ...options.EventFilter) cache.ResourceEventHandler {
				return cache.ResourceEventHandlerFuncs{}
			}).
			WithEventBroadcaster(broadcaster).
			WithReflectionType(offloadingv1beta1.DenyList))

		factory.Start(ctx.Done())
		factory.WaitForCacheSync(ctx.Done())
	}

	handle := func() {
		err = reflector.Handle(trace.ContextWithTrace(ctx, trace.New("Test")), RouteName)
	}

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		route = gwutils.UnsupportedRoute{
			GVR:  schema.GroupVersionResource{Group: gwv1.GroupName, Version: "v1alpha2", Resource: "tcproutes"},
			Kind: "TCPRoute",
		}
		dynClient = dynamic.NewForConfigOrDie(restConfig)
		events = make(chan *corev1.Event, 10)
	})

	AfterEach(func() {
		Expect(dynClient.Resource(route.GVR).Namespace(LocalNamespace).
			DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(Succeed())
		cancel()
	})

	Describe("the NewUnsupportedRouteReflectors function", func() {
		var (
			config     offloadingv1beta1.ReflectorConfig
			reflectors []manager.Reflector
		)

		JustBeforeEach(func() {
			reflectors = gatewayapi.NewUnsupportedRouteReflectors(kubernetes.NewForConfigOrDie(restConfig),
				metadata.NewForConfigOrDie(restConfig), []gwutils.UnsupportedRoute{route}, &config)
		})

		When("the reflection is enabled", func() {
			BeforeEach(func() { config = offloadingv1beta1.ReflectorConfig{NumWorkers: 1, Type: offloadingv1beta1.DenyList} })
			It("should return one reflector for each kind of route", func() {
				Expect(reflectors).To(ConsistOf(HaveField("String()", "TCPRoute")))
			})
		})

		When("the reflection is disabled", func() {
			BeforeEach(func() { config = offloadingv1beta1.ReflectorConfig{NumWorkers: 0} })
			It("should return no reflectors", func() { Expect(reflectors).To(BeEmpty()) })
		})
	})

	When("the local route exists", func() {
		JustBeforeEach(func() {
			create(nil)
			setup()
			handle()
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should generate a warning event identifying the remote cluster and the unsupported kind", func() {
			var event *corev1.Event
			Eventually(events).Should(Receive(&event))
			Expect(event.Type).To(Equal(corev1.EventTypeWarning))
			Expect(event.Reason).To(Equal(forge.EventFailedReflection))
			Expect(event.InvolvedObject.Kind).To(Equal("TCPRoute"))
			Expect(event.InvolvedObject.APIVersion).To(Equal("gateway.networking.k8s.io/v1alpha2"))
			Expect(event.InvolvedObject.Name).To(Equal(RouteName))
			Expect(event.InvolvedObject.UID).ToNot(BeEmpty())
			Expect(event.Message).To(ContainSubstring(RemoteClusterID))
			Expect(event.Message).To(ContainSubstring(LiqoNodeName))
			Expect(event.Message).To(ContainSubstring("TCPRoute resources are not supported by the Liqo reflection"))
			Expect(event.Source.Host).To(Equal(LiqoNodeName))
		})
		It("should list the local route", func() {
			Expect(reflector.List()).To(ConsistOf(types.NamespacedName{Namespace: LocalNamespace, Name: RouteName}))
		})
	})

	When("the local route is marked to be skipped", func() {
		JustBeforeEach(func() {
			create(map[string]string{consts.SkipReflectionAnnotationKey: "true"})
			setup()
			handle()
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should not generate any event", func() { Consistently(events).ShouldNot(Receive()) })
	})

	When("the local route does not exist", func() {
		JustBeforeEach(func() {
			setup()
			handle()
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should not generate any event", func() { Consistently(events).ShouldNot(Receive()) })
	})
})
