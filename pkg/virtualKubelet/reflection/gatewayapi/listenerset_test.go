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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"k8s.io/utils/trace"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwclient "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gwinformers "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoclient "github.com/liqotech/liqo/pkg/client/clientset/versioned"
	liqoinformers "github.com/liqotech/liqo/pkg/client/informers/externalversions"
	"github.com/liqotech/liqo/pkg/consts"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/gatewayapi"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ = Describe("ListenerSet reflection", func() {
	const ListenerSetName = "app"

	var (
		ctx    context.Context
		cancel context.CancelFunc

		cfg        gatewayapi.Config
		client     gwclient.Interface
		liqoClient liqoclient.Interface
		reflector  manager.NamespacedReflector
		events     chan *corev1.Event
		err        error
	)

	mapper := func(local string) (string, bool) {
		if local == LocalNamespace {
			return RemoteNamespace, true
		}
		return "", false
	}

	gateway := func(name, class string, from *gwv1.FromNamespaces, annotations map[string]string) *gwv1.Gateway {
		gw := &gwv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: LocalNamespace, Annotations: annotations},
			Spec: gwv1.GatewaySpec{
				GatewayClassName: gwv1.ObjectName(class),
				Listeners:        []gwv1.Listener{{Name: "http", Port: 80, Protocol: gwv1.HTTPProtocolType}},
			},
		}
		if from != nil {
			gw.Spec.AllowedListeners = &gwv1.AllowedListeners{Namespaces: &gwv1.ListenerNamespaces{From: from}}
		}
		return gw
	}
	shared := map[string]string{consts.RemoteGatewayModeAnnotation: consts.RemoteGatewayModeShared}

	listenerSet := func(namespace, parent string, reflected bool) *gwv1.ListenerSet {
		ls := &gwv1.ListenerSet{
			ObjectMeta: metav1.ObjectMeta{Name: ListenerSetName, Namespace: namespace,
				Annotations: map[string]string{"cert-manager.io/cluster-issuer": "letsencrypt"}},
			Spec: gwv1.ListenerSetSpec{
				ParentRef: gwv1.ParentGatewayReference{Name: gwv1.ObjectName(parent)},
				Listeners: []gwv1.ListenerEntry{{
					Name: "https", Hostname: ptr.To[gwv1.Hostname]("app.example.com"), Port: 443, Protocol: gwv1.HTTPSProtocolType,
					TLS: &gwv1.ListenerTLSConfig{Mode: ptr.To(gwv1.TLSModeTerminate),
						CertificateRefs: []gwv1.SecretObjectReference{{Name: "app-tls"}}},
				}},
			},
		}
		if reflected {
			ls.SetLabels(forge.ReflectionLabels())
		}
		return ls
	}

	route := &gwv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "route", Namespace: LocalNamespace},
		Spec: gwv1.HTTPRouteSpec{CommonRouteSpec: gwv1.CommonRouteSpec{ParentRefs: []gwv1.ParentReference{
			{Name: ListenerSetName, Kind: ptr.To[gwv1.Kind](forge.ListenerSetKind), SectionName: ptr.To[gwv1.SectionName]("https")},
		}}},
	}

	create := func(objects ...runtime.Object) {
		for _, obj := range objects {
			var errCreate error
			switch o := obj.DeepCopyObject().(type) {
			case *gwv1.Gateway:
				_, errCreate = client.GatewayV1().Gateways(o.Namespace).Create(ctx, o, metav1.CreateOptions{})
			case *gwv1.ListenerSet:
				_, errCreate = client.GatewayV1().ListenerSets(o.Namespace).Create(ctx, o, metav1.CreateOptions{})
			case *gwv1.HTTPRoute:
				_, errCreate = client.GatewayV1().HTTPRoutes(o.Namespace).Create(ctx, o, metav1.CreateOptions{})
			}
			Expect(errCreate).ToNot(HaveOccurred())
		}
	}

	setup := func(newReflector func(*gatewayapi.Config) func(*options.NamespacedOpts) manager.NamespacedReflector) {
		broadcaster := record.NewBroadcaster()
		DeferCleanup(broadcaster.Shutdown)
		ch := events
		broadcaster.StartEventWatcher(func(event *corev1.Event) { ch <- event })

		// The local and remote namespaces are hosted by the same API server.
		localFactory := gwinformers.NewSharedInformerFactory(client, 0)
		remoteFactory := gwinformers.NewSharedInformerFactoryWithOptions(client, 0, gwinformers.WithNamespace(RemoteNamespace))
		liqoFactory := liqoinformers.NewSharedInformerFactoryWithOptions(liqoClient, 0, liqoinformers.WithNamespace(LocalNamespace))
		kubeClient := kubernetes.NewForConfigOrDie(restConfig)

		forgingOpts := forge.NewEmptyForgingOpts()
		reflector = newReflector(&cfg)(options.NewNamespaced().
			WithLocal(LocalNamespace, kubeClient, nil).WithRemote(RemoteNamespace, kubeClient, nil).
			WithLiqoLocal(liqoClient, liqoFactory).
			WithGatewayLocal(client, localFactory).WithGatewayRemote(client, remoteFactory).
			WithNamespaceMapper(mapper).
			WithHandlerFactory(func(options.Keyer, ...options.EventFilter) cache.ResourceEventHandler {
				return cache.ResourceEventHandlerFuncs{}
			}).
			WithEventBroadcaster(broadcaster).
			WithReflectionType(offloadingv1beta1.DenyList).
			WithForgingOpts(&forgingOpts))

		localFactory.Start(ctx.Done())
		remoteFactory.Start(ctx.Done())
		liqoFactory.Start(ctx.Done())
		localFactory.WaitForCacheSync(ctx.Done())
		remoteFactory.WaitForCacheSync(ctx.Done())
		liqoFactory.WaitForCacheSync(ctx.Done())
	}

	handle := func(name string) {
		err = reflector.Handle(trace.ContextWithTrace(ctx, trace.New("Test")), name)
	}

	getRemote := func() (*gwv1.ListenerSet, error) {
		return client.GatewayV1().ListenerSets(RemoteNamespace).Get(ctx, ListenerSetName, metav1.GetOptions{})
	}
	getShadow := func() (*offloadingv1beta1.ShadowListenerSetStatus, error) {
		return liqoClient.OffloadingV1beta1().ShadowListenerSetStatuses(LocalNamespace).
			Get(ctx, gatewayapi.ShadowName("", ListenerSetName), metav1.GetOptions{})
	}

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		client = gwclient.NewForConfigOrDie(restConfig)
		liqoClient = liqoclient.NewForConfigOrDie(restConfig)
		events = make(chan *corev1.Event, 10)
		cfg = gatewayapi.Config{
			Support: gatewayapi.SupportFull, GatewaysAvailable: true, ListenerSetsReflected: true,
			SharedGatewayEnabled: true,
			VirtualGatewayClass:  "liqo", RemoteGatewayClass: "envoy",
		}
	})

	AfterEach(func() {
		for _, namespace := range []string{LocalNamespace, RemoteNamespace} {
			Expect(client.GatewayV1().Gateways(namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(Succeed())
			Expect(client.GatewayV1().ListenerSets(namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(Succeed())
			Expect(client.GatewayV1().HTTPRoutes(namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(Succeed())
		}
		shadows := liqoClient.OffloadingV1beta1()
		Expect(shadows.ShadowListenerSetStatuses(LocalNamespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(Succeed())
		Expect(shadows.ShadowRouteStatuses(LocalNamespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(Succeed())
		cancel()
	})

	When("the ListenerSet is attached to a Gateway mapped to the shared one", func() {
		JustBeforeEach(func() {
			create(gateway("web", "liqo", ptr.To(gwv1.NamespacesFromSame), shared), listenerSet(LocalNamespace, "web", false))
			setup(gatewayapi.NewNamespacedListenerSetReflector)
			handle(ListenerSetName)
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should create the remote ListenerSet, attached to the placeholder of the shared Gateway", func() {
			remote, errGet := getRemote()
			Expect(errGet).ToNot(HaveOccurred())
			Expect(forge.IsReflected(remote)).To(BeTrue())
			Expect(forge.IsSharedGatewayPlaceholderParent(&remote.Spec.ParentRef)).To(BeTrue())
			Expect(remote.Annotations).To(HaveKeyWithValue("cert-manager.io/cluster-issuer", "letsencrypt"))
			Expect(remote.Spec.Listeners).To(ConsistOf(HaveField("Hostname", PointTo(BeEquivalentTo("app.example.com")))))
		})
		It("should generate a successful event", func() {
			var event *corev1.Event
			Eventually(events).Should(Receive(&event))
			Expect(event.Reason).To(Equal(forge.EventSuccessfulReflection))
			Expect(event.InvolvedObject.Kind).To(Equal("ListenerSet"))
		})

		When("the remote controller reports the status of the ListenerSet", func() {
			JustBeforeEach(func() {
				Expect(err).ToNot(HaveOccurred())
				remote, errGet := getRemote()
				Expect(errGet).ToNot(HaveOccurred())
				remote.Status.Conditions = []metav1.Condition{{Type: string(gwv1.ListenerSetConditionAccepted), Status: metav1.ConditionTrue,
					Reason: string(gwv1.ListenerSetReasonAccepted), LastTransitionTime: metav1.Now()}}
				remote.Status.Listeners = []gwv1.ListenerEntryStatus{{Name: "https", AttachedRoutes: 1, Conditions: []metav1.Condition{{
					Type: string(gwv1.ListenerEntryConditionConflicted), Status: metav1.ConditionTrue,
					Reason: string(gwv1.ListenerEntryReasonHostnameConflict), LastTransitionTime: metav1.Now()}}}}
				_, errUpdate := client.GatewayV1().ListenerSets(RemoteNamespace).UpdateStatus(ctx, remote, metav1.UpdateOptions{})
				Expect(errUpdate).ToNot(HaveOccurred())

				// Wait for the remote informer to observe the status, and handle the event.
				Eventually(func() error {
					handle(ListenerSetName)
					if err != nil {
						return err
					}
					shadow, errShadow := getShadow()
					if errShadow != nil {
						return errShadow
					}
					if len(shadow.Spec.Listeners) == 0 {
						return fmt.Errorf("status not yet reflected")
					}
					return nil
				}).Should(Succeed())
			})

			It("should report the remote status through the ShadowListenerSetStatus", func() {
				shadow, errShadow := getShadow()
				Expect(errShadow).ToNot(HaveOccurred())
				Expect(shadow.Spec.ListenerSetName).To(Equal(ListenerSetName))
				Expect(shadow.Spec.ClusterID).To(Equal(RemoteClusterID))
				Expect(shadow.OwnerReferences).To(ConsistOf(HaveField("Name", LiqoNodeName)))
				Expect(shadow.Spec.Conditions).To(ConsistOf(HaveField("Type", string(gwv1.ListenerSetConditionAccepted))))
				Expect(shadow.Spec.Listeners).To(ConsistOf(HaveField("Conditions",
					ConsistOf(HaveField("Reason", string(gwv1.ListenerEntryReasonHostnameConflict))))))
			})
		})
	})

	When("the ListenerSet is attached to a reflected Gateway", func() {
		JustBeforeEach(func() {
			create(gateway("web", "liqo", ptr.To(gwv1.NamespacesFromSame), nil), listenerSet(LocalNamespace, "web", false))
			setup(gatewayapi.NewNamespacedListenerSetReflector)
			handle(ListenerSetName)
		})

		It("should attach the remote ListenerSet to the remote copy of the Gateway", func() {
			Expect(err).ToNot(HaveOccurred())
			remote, errGet := getRemote()
			Expect(errGet).ToNot(HaveOccurred())
			Expect(remote.Spec.ParentRef.Name).To(BeEquivalentTo("web"))
			Expect(remote.Spec.ParentRef.Namespace).To(BeNil())
		})
	})

	When("the Gateway does not allow the attachment of the ListenerSet, and a previously reflected remote ListenerSet exists", func() {
		JustBeforeEach(func() {
			create(gateway("web", "liqo", nil, shared), listenerSet(LocalNamespace, "web", false), listenerSet(RemoteNamespace, "web", true))
			setup(gatewayapi.NewNamespacedListenerSetReflector)
			handle(ListenerSetName)
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should delete the remote ListenerSet", func() {
			_, errGet := getRemote()
			Expect(errGet).To(BeNotFound())
		})
		It("should not report any status, as it is set by the controller manager", func() {
			_, errShadow := getShadow()
			Expect(errShadow).To(BeNotFound())
		})
	})

	When("the ListenerSet is attached to a Gateway of another class", func() {
		JustBeforeEach(func() {
			create(gateway("envoy", "envoy", ptr.To(gwv1.NamespacesFromAll), nil), listenerSet(LocalNamespace, "envoy", false))
			setup(gatewayapi.NewNamespacedListenerSetReflector)
			handle(ListenerSetName)
		})

		It("should not be reflected, nor generate events", func() {
			Expect(err).ToNot(HaveOccurred())
			_, errGet := getRemote()
			Expect(errGet).To(BeNotFound())
			Consistently(events).ShouldNot(Receive())
		})
	})

	When("a route is attached to a ListenerSet", func() {
		JustBeforeEach(func() {
			create(gateway("web", "liqo", ptr.To(gwv1.NamespacesFromSame), shared), listenerSet(LocalNamespace, "web", false), route)
			setup(gatewayapi.NewNamespacedHTTPRouteReflector)
			handle(route.Name)
		})

		It("should attach the remote route to the remote copy of the ListenerSet", func() {
			Expect(err).ToNot(HaveOccurred())
			remote, errGet := client.GatewayV1().HTTPRoutes(RemoteNamespace).Get(ctx, route.Name, metav1.GetOptions{})
			Expect(errGet).ToNot(HaveOccurred())
			// The group is defaulted by the API server.
			Expect(remote.Spec.ParentRefs).To(ConsistOf(gwv1.ParentReference{Name: ListenerSetName,
				Group: ptr.To[gwv1.Group](gwv1.GroupName), Kind: ptr.To[gwv1.Kind](forge.ListenerSetKind),
				SectionName: ptr.To[gwv1.SectionName]("https")}))
		})

		When("the ListenerSets are not available in the remote cluster", func() {
			BeforeEach(func() { cfg.ListenerSetsReflected = false })

			It("should not reflect the route, as it would not be attached to any parent", func() {
				Expect(err).ToNot(HaveOccurred())
				_, errGet := client.GatewayV1().HTTPRoutes(RemoteNamespace).Get(ctx, route.Name, metav1.GetOptions{})
				Expect(errGet).To(BeNotFound())
				var event *corev1.Event
				Eventually(events).Should(Receive(&event))
				Expect(event.Reason).To(Equal(forge.EventFailedReflection))
				Expect(event.Message).To(ContainSubstring("the ListenerSet is not reflected"))
			})
		})
	})
})
