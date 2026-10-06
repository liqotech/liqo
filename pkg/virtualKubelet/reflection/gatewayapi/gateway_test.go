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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"k8s.io/utils/trace"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwclient "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gwclientfake "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/fake"
	gwinformers "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoclient "github.com/liqotech/liqo/pkg/client/clientset/versioned"
	liqoinformers "github.com/liqotech/liqo/pkg/client/informers/externalversions"
	"github.com/liqotech/liqo/pkg/consts"
	gwutils "github.com/liqotech/liqo/pkg/utils/gatewayapi"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/gatewayapi"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ = Describe("Gateway reflection", func() {
	const GatewayName = "web"

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

	gateway := func(namespace, class string, reflected bool) *gwv1.Gateway {
		gw := &gwv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{Name: GatewayName, Namespace: namespace},
			Spec: gwv1.GatewaySpec{
				GatewayClassName: gwv1.ObjectName(class),
				Listeners:        []gwv1.Listener{{Name: "http", Port: 80, Protocol: gwv1.HTTPProtocolType}},
			},
		}
		if reflected {
			gw.SetLabels(forge.ReflectionLabels())
		}
		return gw
	}

	route := &gwv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "route", Namespace: LocalNamespace},
		Spec: gwv1.HTTPRouteSpec{CommonRouteSpec: gwv1.CommonRouteSpec{ParentRefs: []gwv1.ParentReference{
			{Name: GatewayName, SectionName: ptr.To[gwv1.SectionName]("http")},
		}}},
	}

	create := func(objects ...runtime.Object) {
		for _, obj := range objects {
			var errCreate error
			switch o := obj.DeepCopyObject().(type) {
			case *gwv1.Gateway:
				_, errCreate = client.GatewayV1().Gateways(o.Namespace).Create(ctx, o, metav1.CreateOptions{})
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

	getRemoteGateway := func() (*gwv1.Gateway, error) {
		return client.GatewayV1().Gateways(RemoteNamespace).Get(ctx, GatewayName, metav1.GetOptions{})
	}

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		client = gwclient.NewForConfigOrDie(restConfig)
		liqoClient = liqoclient.NewForConfigOrDie(restConfig)
		events = make(chan *corev1.Event, 10)
		cfg = gatewayapi.Config{
			Support: gatewayapi.SupportFull, GatewaysAvailable: true,
			SharedGatewayEnabled: true,
			VirtualGatewayClass:  "liqo", RemoteGatewayClass: "envoy",
		}
	})

	AfterEach(func() {
		for _, namespace := range []string{LocalNamespace, RemoteNamespace} {
			Expect(client.GatewayV1().Gateways(namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(Succeed())
			Expect(client.GatewayV1().HTTPRoutes(namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(Succeed())
		}
		shadows := liqoClient.OffloadingV1beta1()
		Expect(shadows.ShadowGatewayStatuses(LocalNamespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(Succeed())
		Expect(shadows.ShadowRouteStatuses(LocalNamespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(Succeed())
		cancel()
	})

	When("the local Gateway belongs to the virtual class", func() {
		JustBeforeEach(func() {
			create(gateway(LocalNamespace, "liqo", false))
			setup(gatewayapi.NewNamespacedGatewayReflector)
			handle(GatewayName)
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should create the remote Gateway, with the GatewayClass offered by the remote cluster", func() {
			remote, errGet := getRemoteGateway()
			Expect(errGet).ToNot(HaveOccurred())
			Expect(forge.IsReflected(remote)).To(BeTrue())
			Expect(remote.Spec.GatewayClassName).To(BeEquivalentTo("envoy"))
		})
		It("should generate a successful event", func() {
			var event *corev1.Event
			Eventually(events).Should(Receive(&event))
			Expect(event.Reason).To(Equal(forge.EventSuccessfulReflection))
			Expect(event.InvolvedObject.Kind).To(Equal("Gateway"))
		})
	})

	When("the local Gateway belongs to another class, and a previously reflected remote Gateway exists", func() {
		JustBeforeEach(func() {
			create(gateway(LocalNamespace, "envoy", false), gateway(RemoteNamespace, "envoy", true))
			setup(gatewayapi.NewNamespacedGatewayReflector)
			handle(GatewayName)
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should delete the remote Gateway", func() {
			_, errGet := getRemoteGateway()
			Expect(errGet).To(BeNotFound())
		})
		It("should not generate any event", func() { Consistently(events).ShouldNot(Receive()) })
	})

	When("the local Gateway belongs to another class, and a remote Gateway not managed by the reflection exists", func() {
		JustBeforeEach(func() {
			create(gateway(LocalNamespace, "envoy", false), gateway(RemoteNamespace, "envoy", false))
			setup(gatewayapi.NewNamespacedGatewayReflector)
			handle(GatewayName)
		})

		It("should not touch the remote Gateway", func() {
			_, errGet := getRemoteGateway()
			Expect(errGet).ToNot(HaveOccurred())
		})
		It("should not generate any event", func() { Consistently(events).ShouldNot(Receive()) })
	})

	When("the local Gateway belongs to the virtual class, and a remote Gateway not managed by the reflection exists", func() {
		JustBeforeEach(func() {
			create(gateway(LocalNamespace, "liqo", false), gateway(RemoteNamespace, "envoy", false))
			setup(gatewayapi.NewNamespacedGatewayReflector)
			handle(GatewayName)
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should not touch the remote Gateway", func() {
			remote, errGet := getRemoteGateway()
			Expect(errGet).ToNot(HaveOccurred())
			Expect(forge.IsReflected(remote)).To(BeFalse())
		})
		It("should report the failure through the ShadowGatewayStatus", func() {
			shadow, errShadow := liqoClient.OffloadingV1beta1().ShadowGatewayStatuses(LocalNamespace).
				Get(ctx, gatewayapi.ShadowName("", GatewayName), metav1.GetOptions{})
			Expect(errShadow).ToNot(HaveOccurred())
			Expect(shadow.Spec.ClusterID).To(Equal(RemoteClusterID))
			Expect(shadow.Spec.Addresses).To(BeEmpty())
			failed := And(HaveField("Status", metav1.ConditionFalse), HaveField("Reason", forge.ConditionReasonReflectionFailed),
				HaveField("Message", ContainSubstring("not managed by Liqo, already exists")))
			Expect(shadow.Spec.Conditions).To(ConsistOf(
				And(HaveField("Type", string(gwv1.GatewayConditionAccepted)), failed),
				And(HaveField("Type", string(gwv1.GatewayConditionProgrammed)), failed),
			))
		})
	})

	When("the remote Gateway reports its status", func() {
		getShadow := func() (*offloadingv1beta1.ShadowGatewayStatus, error) {
			return liqoClient.OffloadingV1beta1().ShadowGatewayStatuses(LocalNamespace).Get(ctx, gatewayapi.ShadowName("", GatewayName), metav1.GetOptions{})
		}

		JustBeforeEach(func() {
			create(gateway(LocalNamespace, "liqo", false))
			setup(gatewayapi.NewNamespacedGatewayReflector)
			handle(GatewayName)
			Expect(err).ToNot(HaveOccurred())

			// Simulate the remote controller setting the status of the reflected Gateway.
			remote, errGet := getRemoteGateway()
			Expect(errGet).ToNot(HaveOccurred())
			remote.Status.Addresses = []gwv1.GatewayStatusAddress{{Type: ptr.To(gwv1.IPAddressType), Value: "10.0.0.1"}}
			remote.Status.Conditions = []metav1.Condition{{Type: string(gwv1.GatewayConditionProgrammed), Status: metav1.ConditionTrue,
				Reason: string(gwv1.GatewayReasonProgrammed), LastTransitionTime: metav1.Now()}}
			_, errUpdate := client.GatewayV1().Gateways(RemoteNamespace).UpdateStatus(ctx, remote, metav1.UpdateOptions{})
			Expect(errUpdate).ToNot(HaveOccurred())

			// Wait for the remote informer to observe the status, and handle the event.
			Eventually(func() error {
				handle(GatewayName)
				if err != nil {
					return err
				}
				shadow, errShadow := getShadow()
				if errShadow != nil {
					return errShadow
				}
				if len(shadow.Spec.Addresses) == 0 {
					return fmt.Errorf("status not yet reflected")
				}
				return nil
			}).Should(Succeed())
		})

		It("should report the remote status through the ShadowGatewayStatus", func() {
			shadow, errShadow := getShadow()
			Expect(errShadow).ToNot(HaveOccurred())
			Expect(shadow.Spec.GatewayName).To(Equal(GatewayName))
			Expect(shadow.Spec.ClusterID).To(Equal(RemoteClusterID))
			Expect(shadow.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, RemoteClusterID))
			Expect(shadow.OwnerReferences).To(ConsistOf(HaveField("Name", LiqoNodeName)))
			Expect(shadow.Spec.Addresses).To(ConsistOf(HaveField("Value", "10.0.0.1")))
			Expect(shadow.Spec.Conditions).To(ConsistOf(HaveField("Type", string(gwv1.GatewayConditionProgrammed))))
		})

		When("the local Gateway is deleted", func() {
			JustBeforeEach(func() {
				Expect(client.GatewayV1().Gateways(LocalNamespace).Delete(ctx, GatewayName, metav1.DeleteOptions{})).To(Succeed())
				// Handle the object until the local informer observes the deletion.
				Eventually(func() error {
					handle(GatewayName)
					Expect(err).ToNot(HaveOccurred())
					_, errShadow := getShadow()
					return errShadow
				}).Should(BeNotFound())
			})

			It("should delete the remote Gateway", func() {
				_, errGet := getRemoteGateway()
				Expect(errGet).To(BeNotFound())
			})
		})

		When("the reflection of the namespace is stopped", func() {
			JustBeforeEach(func() {
				// The shadow resources to be deleted are retrieved from the cache, which might not yet include the latest ones.
				Eventually(func() error {
					Expect(reflector.Cleanup(ctx, LocalNamespace, RemoteNamespace)).To(Succeed())
					_, errShadow := getShadow()
					return errShadow
				}).Should(BeNotFound())
			})

			It("should delete the ShadowGatewayStatus", func() {
				_, errShadow := getShadow()
				Expect(errShadow).To(BeNotFound())
			})
		})
	})

	When("the local Gateway is mapped to the shared Gateway", func() {
		var remoteRoutes []runtime.Object

		getShadow := func() (*offloadingv1beta1.ShadowGatewayStatus, error) {
			return liqoClient.OffloadingV1beta1().ShadowGatewayStatuses(LocalNamespace).Get(ctx, gatewayapi.ShadowName("", GatewayName), metav1.GetOptions{})
		}

		// remoteRoute returns a route reflected in the remote namespace, attached to the shared Gateway (as resolved by the
		// remote cluster, which replaced the placeholder), and annotated with its addresses.
		remoteRoute := func(name, addresses string) *gwv1.HTTPRoute {
			return &gwv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: RemoteNamespace, Labels: forge.ReflectionLabels(),
					Annotations: map[string]string{consts.SharedGatewayAddressesAnnotation: addresses, consts.SharedGatewayAnnotation: "infra/public"}},
				Spec: gwv1.HTTPRouteSpec{CommonRouteSpec: gwv1.CommonRouteSpec{ParentRefs: []gwv1.ParentReference{
					{Name: "public", Namespace: ptr.To[gwv1.Namespace]("infra")},
				}}},
			}
		}

		BeforeEach(func() {
			cfg.ReflectedRoutes = []schema.GroupResource{gwutils.HTTPRoutesGVR.GroupResource()}
			remoteRoutes = []runtime.Object{remoteRoute("route", `[{"type":"IPAddress","value":"10.0.0.1"}]`)}
		})

		JustBeforeEach(func() {
			local := gateway(LocalNamespace, "liqo", false)
			local.SetAnnotations(map[string]string{consts.RemoteGatewayModeAnnotation: consts.RemoteGatewayModeShared})
			// A remote Gateway previously reflected, before the local one was mapped to the shared Gateway.
			create(append([]runtime.Object{local, gateway(RemoteNamespace, "envoy", true)}, remoteRoutes...)...)
			setup(gatewayapi.NewNamespacedGatewayReflector)
			handle(GatewayName)
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should delete the remote Gateway previously reflected", func() {
			_, errGet := getRemoteGateway()
			Expect(errGet).To(BeNotFound())
		})
		It("should report the addresses of the shared Gateway through the ShadowGatewayStatus", func() {
			shadow, errShadow := getShadow()
			Expect(errShadow).ToNot(HaveOccurred())
			Expect(shadow.Spec.Addresses).To(ConsistOf(gwv1.GatewayStatusAddress{Type: ptr.To(gwv1.IPAddressType), Value: "10.0.0.1"}))
			Expect(shadow.Spec.Conditions).To(ContainElement(And(
				HaveField("Type", string(gwv1.GatewayConditionProgrammed)),
				HaveField("Status", metav1.ConditionTrue),
				HaveField("Message", ContainSubstring(`shared Gateway "infra/public"`)),
			)))
			Expect(shadow.Spec.Listeners).To(ConsistOf(HaveField("Name", BeEquivalentTo("http"))))
		})
		It("should generate the corresponding event", func() {
			var event *corev1.Event
			Eventually(events).Should(Receive(&event))
			Expect(event.Reason).To(Equal(forge.EventMappedToSharedGateway))
		})

		When("the remote routes report the shared Gateway, but not its addresses", func() {
			BeforeEach(func() {
				pending := remoteRoute("route", "")
				delete(pending.Annotations, consts.SharedGatewayAddressesAnnotation)
				remoteRoutes = []runtime.Object{pending}
			})

			It("should report the shared Gateway as not yet programmed", func() {
				shadow, errShadow := getShadow()
				Expect(errShadow).ToNot(HaveOccurred())
				Expect(shadow.Spec.Addresses).To(BeEmpty())
				Expect(shadow.Spec.Conditions).To(ContainElement(And(
					HaveField("Type", string(gwv1.GatewayConditionProgrammed)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Message", ContainSubstring(`shared Gateway "infra/public"`)),
				)))
			})
		})

		When("no remote route reports the addresses of the shared Gateway", func() {
			BeforeEach(func() { remoteRoutes = nil })

			It("should report the Gateway as not yet programmed", func() {
				shadow, errShadow := getShadow()
				Expect(errShadow).ToNot(HaveOccurred())
				Expect(shadow.Spec.Addresses).To(BeEmpty())
				Expect(shadow.Spec.Conditions).To(ContainElement(And(
					HaveField("Type", string(gwv1.GatewayConditionProgrammed)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(gwv1.GatewayReasonPending)),
				)))
			})
		})

		When("the remote routes report invalid addresses, or are not attached to the shared Gateway", func() {
			BeforeEach(func() {
				detached := remoteRoute("detached", `[{"value":"10.0.0.2"}]`)
				detached.Spec.ParentRefs[0].Name = "other"
				remoteRoutes = []runtime.Object{remoteRoute("invalid", "invalid"), detached, remoteRoute("valid", `[{"value":"10.0.0.3"}]`)}
			})

			It("should ignore them", func() {
				shadow, errShadow := getShadow()
				Expect(errShadow).ToNot(HaveOccurred())
				Expect(shadow.Spec.Addresses).To(ConsistOf(HaveField("Value", "10.0.0.3")))
			})
		})
	})

	When("a local route is attached to a Gateway", func() {
		var class string

		JustBeforeEach(func() {
			create(gateway(LocalNamespace, class, false), route)
			setup(gatewayapi.NewNamespacedHTTPRouteReflector)
			handle("route")
		})

		getRemoteRoute := func() *gwv1.HTTPRoute {
			remote, errGet := client.GatewayV1().HTTPRoutes(RemoteNamespace).Get(ctx, "route", metav1.GetOptions{})
			Expect(errGet).ToNot(HaveOccurred())
			return remote
		}

		When("the Gateway belongs to the virtual class", func() {
			BeforeEach(func() { class = "liqo" })

			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should attach the remote route to the remote copy of the Gateway", func() {
				Expect(getRemoteRoute().Spec.ParentRefs).To(ConsistOf(
					HaveField("Name", BeEquivalentTo(GatewayName)),
				))
				Expect(getRemoteRoute().Spec.ParentRefs[0].SectionName).To(PointTo(BeEquivalentTo("http")))
			})
		})

		When("the Gateway belongs to the virtual class, but the remote cluster does not offer any GatewayClass", func() {
			BeforeEach(func() { class, cfg.RemoteGatewayClass = "liqo", "" })

			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should attach the remote route to the shared Gateway, as the Gateway is mapped to it", func() {
				parents := getRemoteRoute().Spec.ParentRefs
				Expect(parents).To(HaveLen(1))
				Expect(forge.IsSharedGatewayPlaceholder(&parents[0])).To(BeTrue())
			})
		})

		When("the Gateway belongs to another class", func() {
			BeforeEach(func() { class = "envoy" })

			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should attach the remote route to the shared Gateway", func() {
				parents := getRemoteRoute().Spec.ParentRefs
				Expect(parents).To(HaveLen(1))
				Expect(forge.IsSharedGatewayPlaceholder(&parents[0])).To(BeTrue())
			})
		})
	})
})

var _ = Describe("Cluster-wide fallback", func() {
	var (
		ctx    context.Context
		cancel context.CancelFunc

		fallback manager.FallbackReflector
		keyers   []options.Keyer
	)

	mapper := func(local string) (string, bool) {
		if local == LocalNamespace {
			return RemoteNamespace, true
		}
		return "", false
	}

	route := func(namespace, name string, parents ...gwv1.ParentReference) *gwv1.HTTPRoute {
		return &gwv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       gwv1.HTTPRouteSpec{CommonRouteSpec: gwv1.CommonRouteSpec{ParentRefs: parents}},
		}
	}

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		keyers = nil

		localClient := gwclientfake.NewClientset(
			route(LocalNamespace, "attached", gwv1.ParentReference{Name: "edge", Namespace: ptr.To[gwv1.Namespace]("infra")}),
			route(LocalNamespace, "other", gwv1.ParentReference{Name: "other", Namespace: ptr.To[gwv1.Namespace]("infra")}),
			route("not-offloaded", "attached", gwv1.ParentReference{Name: "edge", Namespace: ptr.To[gwv1.Namespace]("infra")}),
		)
		factory := gwinformers.NewSharedInformerFactory(localClient, 0)

		// The handler factory records the keyers, in the order they are registered.
		handlerFactory := func(keyer options.Keyer, _ ...options.EventFilter) cache.ResourceEventHandler {
			keyers = append(keyers, keyer)
			return cache.ResourceEventHandlerFuncs{}
		}

		cfg := gatewayapi.Config{GatewaysAvailable: true}
		fallback = gatewayapi.NewClusterFallback(gatewayapi.HTTPRouteReflectorName, &cfg)(
			options.New(nil, nil).WithHandlerFactory(handlerFactory).WithGatewayLocal(factory).WithNamespaceMapper(mapper))

		factory.Start(ctx.Done())
		factory.WaitForCacheSync(ctx.Done())
	})
	AfterEach(func() { cancel() })

	It("should register the handlers for the routes and the Gateways", func() { Expect(keyers).To(HaveLen(2)) })
	It("should always be ready", func() { Expect(fallback.Ready()).To(BeTrue()) })
	It("should silently ignore the objects in namespaces not reflected", func() {
		Expect(fallback.Handle(ctx, types.NamespacedName{Namespace: "not-offloaded", Name: "attached"})).To(Succeed())
	})

	It("should return the keys of the local objects in a given namespace", func() {
		Expect(fallback.Keys(LocalNamespace, RemoteNamespace)).To(ConsistOf(
			types.NamespacedName{Namespace: LocalNamespace, Name: "attached"},
			types.NamespacedName{Namespace: LocalNamespace, Name: "other"},
		))
	})

	It("should enqueue only the routes in the namespaces reflected", func() {
		Expect(keyers[0](&metav1.ObjectMeta{Namespace: LocalNamespace, Name: "attached"})).To(
			ConsistOf(types.NamespacedName{Namespace: LocalNamespace, Name: "attached"}))
		Expect(keyers[0](&metav1.ObjectMeta{Namespace: "not-offloaded", Name: "attached"})).To(BeEmpty())
	})

	It("should enqueue the routes attached to a Gateway when it changes, in the namespaces reflected", func() {
		Expect(keyers[1](&metav1.ObjectMeta{Namespace: "infra", Name: "edge"})).To(
			ConsistOf(types.NamespacedName{Namespace: LocalNamespace, Name: "attached"}))
		Expect(keyers[1](&metav1.ObjectMeta{Namespace: "infra", Name: "unknown"})).To(BeEmpty())
	})
})
