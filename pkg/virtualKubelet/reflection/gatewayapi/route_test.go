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
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
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
	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/gatewayapi"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ = Describe("Route reflection", func() {
	const RouteName = "route"

	var (
		ctx    context.Context
		cancel context.CancelFunc

		localClient, remoteClient gwclient.Interface
		localLiqoClient           liqoclient.Interface
		remoteKubeClient          *k8sfake.Clientset
		remoteAllowed             bool
		remoteApplyError          error
		cfg                       gatewayapi.Config
		reflector                 manager.NamespacedReflector
		events                    chan *corev1.Event
		err                       error
	)

	fakeHandler := func(options.Keyer, ...options.EventFilter) cache.ResourceEventHandler {
		return cache.ResourceEventHandlerFuncs{}
	}

	mapper := func(local string) (string, bool) {
		if local == LocalNamespace {
			return RemoteNamespace, true
		}
		return "", false
	}

	localRoute := func(backendNamespace string, annotations map[string]string) *gwv1.HTTPRoute {
		backend := gwv1.HTTPBackendRef{BackendRef: gwv1.BackendRef{BackendObjectReference: gwv1.BackendObjectReference{
			Name: "svc", Port: ptr.To[gwv1.PortNumber](80)}}}
		if backendNamespace != "" {
			backend.Namespace = ptr.To(gwv1.Namespace(backendNamespace))
		}
		return &gwv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{Name: RouteName, Namespace: LocalNamespace, Annotations: annotations},
			Spec: gwv1.HTTPRouteSpec{
				CommonRouteSpec: gwv1.CommonRouteSpec{ParentRefs: []gwv1.ParentReference{{Name: "edge", Namespace: ptr.To[gwv1.Namespace]("infra")}}},
				Rules:           []gwv1.HTTPRouteRule{{BackendRefs: []gwv1.HTTPBackendRef{backend}}},
			},
		}
	}

	remoteRoute := func(reflected bool) *gwv1.HTTPRoute {
		route := &gwv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: RouteName, Namespace: RemoteNamespace, UID: "remote-uid"}}
		if reflected {
			route.SetLabels(forge.ReflectionLabels())
		}
		return route
	}

	getRemote := func() (*gwv1.HTTPRoute, error) {
		return remoteClient.GatewayV1().HTTPRoutes(RemoteNamespace).Get(ctx, RouteName, metav1.GetOptions{})
	}

	getShadow := func() (*offloadingv1beta1.ShadowRouteStatus, error) {
		return localLiqoClient.OffloadingV1beta1().ShadowRouteStatuses(LocalNamespace).
			Get(ctx, gatewayapi.ShadowName("httproute", RouteName), metav1.GetOptions{})
	}

	// expectFailureReported checks that the ShadowRouteStatus reports the failure for the parent of the local route.
	expectFailureReported := func(message string) {
		shadow, errShadow := getShadow()
		Expect(errShadow).ToNot(HaveOccurred())
		Expect(shadow.Spec.ClusterID).To(Equal(RemoteClusterID))
		Expect(shadow.Spec.Parents).To(HaveLen(1))
		parent := shadow.Spec.Parents[0]
		Expect(parent.ParentRef.Name).To(BeEquivalentTo("edge"))
		Expect(parent.ParentRef.Namespace).To(PointTo(BeEquivalentTo("infra")))
		Expect(parent.ControllerName).To(BeEquivalentTo(consts.GatewayControllerName))
		Expect(parent.Conditions).To(ConsistOf(And(
			HaveField("Type", string(gwv1.RouteConditionAccepted)),
			HaveField("Status", metav1.ConditionFalse),
			HaveField("Reason", forge.ConditionReasonReflectionFailed),
			HaveField("Message", ContainSubstring(message)),
		)))
	}

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		cfg = gatewayapi.Config{
			SharedGateway: &types.NamespacedName{Namespace: "infra", Name: "public"},
			Support:       gatewayapi.SupportFull,
		}
		events = make(chan *corev1.Event, 10)
		remoteAllowed = true
		remoteApplyError = nil
		localLiqoClient = liqoclient.NewForConfigOrDie(restConfig)
		localClient = gwclientfake.NewClientset()
		remoteClient = gwclientfake.NewClientset()
	})
	AfterEach(func() {
		Expect(localLiqoClient.OffloadingV1beta1().ShadowRouteStatuses(LocalNamespace).
			DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(Succeed())
		cancel()
	})

	// setup creates the given objects and starts the reflector.
	setup := func(local, remote []runtime.Object) {
		localClient = gwclientfake.NewClientset(local...)
		fakeRemoteClient := gwclientfake.NewClientset(remote...)
		if applyError := remoteApplyError; applyError != nil {
			// The apply patches are rejected by the remote cluster (e.g., due to a different validation).
			fakeRemoteClient.PrependReactor("patch", "httproutes", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, applyError
			})
		}
		remoteClient = fakeRemoteClient

		// The SelfSubjectAccessReviews are answered depending on the remoteAllowed flag.
		allowed := remoteAllowed
		remoteKubeClient = k8sfake.NewClientset()
		remoteKubeClient.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
			review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
			review.Status.Allowed = allowed
			return true, review, nil
		})

		broadcaster := record.NewBroadcaster()
		DeferCleanup(broadcaster.Shutdown)
		// Capture the channel by value, to prevent late events of previous tests from being delivered to the current one.
		ch := events
		broadcaster.StartEventWatcher(func(event *corev1.Event) { ch <- event })

		localFactory := gwinformers.NewSharedInformerFactory(localClient, 0)
		remoteFactory := gwinformers.NewSharedInformerFactoryWithOptions(remoteClient, 0, gwinformers.WithNamespace(RemoteNamespace))

		// The shadow resources are stored in the test API server, as the fake clients do not support their apply patches.
		localLiqoClient = liqoclient.NewForConfigOrDie(restConfig)
		localLiqoFactory := liqoinformers.NewSharedInformerFactoryWithOptions(localLiqoClient, 0, liqoinformers.WithNamespace(LocalNamespace))

		forgingOpts := forge.NewEmptyForgingOpts()
		opts := options.NewNamespaced().
			WithLocal(LocalNamespace, kubernetes.NewForConfigOrDie(restConfig), nil).WithRemote(RemoteNamespace, remoteKubeClient, nil).
			WithLiqoLocal(localLiqoClient, localLiqoFactory).
			WithGatewayLocal(localClient, localFactory).
			WithNamespaceMapper(mapper).
			WithHandlerFactory(fakeHandler).
			WithEventBroadcaster(broadcaster).
			WithReflectionType(offloadingv1beta1.DenyList).
			WithForgingOpts(&forgingOpts)
		if cfg.Support == gatewayapi.SupportFull {
			opts.WithGatewayRemote(remoteClient, remoteFactory)
		}
		reflector = gatewayapi.NewNamespacedHTTPRouteReflector(&cfg)(opts)

		localFactory.Start(ctx.Done())
		remoteFactory.Start(ctx.Done())
		localLiqoFactory.Start(ctx.Done())
		localFactory.WaitForCacheSync(ctx.Done())
		remoteFactory.WaitForCacheSync(ctx.Done())
		localLiqoFactory.WaitForCacheSync(ctx.Done())
	}

	handle := func() {
		err = reflector.Handle(trace.ContextWithTrace(ctx, trace.New("HTTPRoute")), RouteName)
	}

	Describe("the NewHTTPRouteReflector function", func() {
		It("should not return a nil reflector", func() {
			config := offloadingv1beta1.ReflectorConfig{NumWorkers: 1, Type: offloadingv1beta1.DenyList}
			Expect(gatewayapi.NewHTTPRouteReflector(&config, &cfg)).ToNot(BeNil())
			Expect(gatewayapi.NewGRPCRouteReflector(&config, &cfg)).ToNot(BeNil())
		})
	})

	When("the local route exists and can be reflected", func() {
		JustBeforeEach(func() {
			setup([]runtime.Object{localRoute("", nil)}, nil)
			handle()
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should create the remote route, attached to the shared gateway", func() {
			remote, errGet := getRemote()
			Expect(errGet).ToNot(HaveOccurred())
			Expect(forge.IsReflected(remote)).To(BeTrue())
			Expect(remote.Spec.ParentRefs).To(ConsistOf(gwv1.ParentReference{Name: "public", Namespace: ptr.To[gwv1.Namespace]("infra")}))
			Expect(remote.Spec.Rules[0].BackendRefs).To(HaveLen(1))
		})
		It("should generate a successful event", func() {
			var event *corev1.Event
			Eventually(events).Should(Receive(&event))
			Expect(event.Type).To(Equal(corev1.EventTypeNormal))
			Expect(event.Reason).To(Equal(forge.EventSuccessfulReflection))
			Expect(event.InvolvedObject.Kind).To(Equal("HTTPRoute"))
			Expect(event.Source.Host).To(Equal(LiqoNodeName))
		})
	})

	When("the local route has references which cannot be translated", func() {
		JustBeforeEach(func() {
			setup([]runtime.Object{localRoute("not-offloaded", nil)}, nil)
			handle()
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should create the remote route without the dropped references", func() {
			remote, errGet := getRemote()
			Expect(errGet).ToNot(HaveOccurred())
			Expect(remote.Spec.Rules[0].BackendRefs).To(BeEmpty())
		})
		It("should generate a warning event identifying the remote cluster", func() {
			var event *corev1.Event
			Eventually(events).Should(Receive(&event))
			Expect(event.Type).To(Equal(corev1.EventTypeWarning))
			Expect(event.Reason).To(Equal(forge.EventPartialReflection))
			Expect(event.Message).To(ContainSubstring(RemoteClusterID))
			Expect(event.Message).To(ContainSubstring(LiqoNodeName))
			Expect(event.Message).To(ContainSubstring("backendRef Service not-offloaded/svc dropped"))
		})
	})

	When("the local route cannot be reflected, and a previously reflected remote route exists", func() {
		BeforeEach(func() { cfg.SharedGateway = nil })
		JustBeforeEach(func() {
			setup([]runtime.Object{localRoute("", nil)}, []runtime.Object{remoteRoute(true)})
			handle()
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should delete the remote route", func() {
			_, errGet := getRemote()
			Expect(errGet).To(BeNotFound())
		})
		It("should generate a warning event with the reason", func() {
			var event *corev1.Event
			Eventually(events).Should(Receive(&event))
			Expect(event.Type).To(Equal(corev1.EventTypeWarning))
			Expect(event.Reason).To(Equal(forge.EventFailedReflection))
			Expect(event.Message).To(ContainSubstring(RemoteClusterID))
			Expect(event.Message).To(ContainSubstring("no shared Gateway offered by the remote cluster"))
		})
		It("should not report any status, as the route would not be attached to any parent", func() {
			_, errShadow := getShadow()
			Expect(errShadow).To(BeNotFound())
		})
	})

	When("the local route cannot be reflected without altering its semantic", func() {
		JustBeforeEach(func() {
			route := localRoute("", nil)
			route.Spec.Rules[0].Filters = []gwv1.HTTPRouteFilter{{Type: gwv1.HTTPRouteFilterExtensionRef,
				ExtensionRef: &gwv1.LocalObjectReference{Group: "example.com", Kind: "Auth", Name: "auth"}}}
			setup([]runtime.Object{route}, []runtime.Object{remoteRoute(true)})
			handle()
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should delete the remote route", func() {
			_, errGet := getRemote()
			Expect(errGet).To(BeNotFound())
		})
		It("should report the failure in the status", func() { expectFailureReported("not reflected: ") })
	})

	When("the local route does not exist, and a reflected remote route exists", func() {
		JustBeforeEach(func() {
			setup(nil, []runtime.Object{remoteRoute(true)})
			handle()
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should delete the remote route", func() {
			_, errGet := getRemote()
			Expect(errGet).To(BeNotFound())
		})
	})

	When("the remote route exists, but it is not managed by the reflection", func() {
		JustBeforeEach(func() {
			setup([]runtime.Object{localRoute("", nil)}, []runtime.Object{remoteRoute(false)})
			handle()
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should not mutate the remote route", func() {
			remote, errGet := getRemote()
			Expect(errGet).ToNot(HaveOccurred())
			Expect(remote.Spec.ParentRefs).To(BeEmpty())
		})
		It("should generate a warning event", func() {
			var event *corev1.Event
			Eventually(events).Should(Receive(&event))
			Expect(event.Reason).To(Equal(forge.EventFailedReflection))
			Expect(event.Message).To(ContainSubstring("remote object already exists"))
		})
		It("should report the failure in the status", func() { expectFailureReported("not managed by Liqo, already exists") })
	})

	When("the remote cluster rejects the route", func() {
		BeforeEach(func() {
			remoteApplyError = kerrors.NewInvalid(schema.GroupKind{Group: gwv1.GroupName, Kind: "HTTPRoute"}, RouteName, nil)
		})
		JustBeforeEach(func() {
			setup([]runtime.Object{localRoute("", nil)}, nil)
			handle()
		})

		It("should fail", func() { Expect(err).To(HaveOccurred()) })
		It("should generate a warning event identifying the remote cluster", func() {
			var event *corev1.Event
			Eventually(events).Should(Receive(&event))
			Expect(event.Reason).To(Equal(forge.EventFailedReflection))
			Expect(event.Message).To(ContainSubstring(RemoteClusterID))
		})
		It("should report the failure in the status, with the error returned by the remote cluster", func() {
			expectFailureReported("reflection failed: HTTPRoute.gateway.networking.k8s.io \"route\" is invalid")
		})

		When("the reflection keeps failing", func() {
			var first metav1.Time

			JustBeforeEach(func() {
				shadow, errShadow := getShadow()
				Expect(errShadow).ToNot(HaveOccurred())
				first = shadow.Spec.Parents[0].Conditions[0].LastTransitionTime

				// Ensure a different transition time would be generated, and the informer observed the shadow resource.
				time.Sleep(1100 * time.Millisecond)
				handle()
			})

			It("should preserve the last transition time", func() {
				shadow, errShadow := getShadow()
				Expect(errShadow).ToNot(HaveOccurred())
				Expect(shadow.Spec.Parents[0].Conditions[0].LastTransitionTime).To(Equal(first))
			})
		})

		When("the remote cluster is not reachable", func() {
			BeforeEach(func() { remoteApplyError = errors.New("connection refused") })

			It("should report the transient error", func() { expectFailureReported("reflection failed: connection refused") })
		})
	})

	When("the local route is marked to be skipped", func() {
		JustBeforeEach(func() {
			setup([]runtime.Object{localRoute("", map[string]string{consts.SkipReflectionAnnotationKey: "true"})}, nil)
			handle()
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should not create the remote route", func() {
			_, errGet := getRemote()
			Expect(errGet).To(BeNotFound())
		})
	})

	When("the virtual kubelet is not allowed to operate on the resource in the remote namespace", func() {
		BeforeEach(func() { remoteAllowed = false })
		JustBeforeEach(func() {
			setup([]runtime.Object{localRoute("", nil)}, nil)
			handle()
		})

		It("should check the permissions in the remote namespace", func() {
			Expect(remoteKubeClient.Actions()).ToNot(BeEmpty())
			review := remoteKubeClient.Actions()[0].(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
			Expect(review.Spec.ResourceAttributes.Namespace).To(Equal(RemoteNamespace))
			Expect(review.Spec.ResourceAttributes.Group).To(Equal(gwv1.GroupName))
			Expect(review.Spec.ResourceAttributes.Resource).To(Equal("httproutes"))
		})
		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should not interact with the remote cluster", func() {
			Expect(remoteClient.(*gwclientfake.Clientset).Actions()).To(BeEmpty())
		})
		It("should generate a warning event suggesting the cause", func() {
			var event *corev1.Event
			Eventually(events).Should(Receive(&event))
			Expect(event.Reason).To(Equal(forge.EventFailedReflection))
			Expect(event.Message).To(ContainSubstring(RemoteClusterID))
			Expect(event.Message).To(ContainSubstring("not allowed to list and watch resource httproutes.gateway.networking.k8s.io"))
			Expect(event.Message).To(ContainSubstring("Liqo version installed in the remote cluster"))
		})
	})

	When("the resource is not available in the remote cluster (degraded mode)", func() {
		BeforeEach(func() { cfg.Support = gatewayapi.SupportDegraded })

		When("the local route exists", func() {
			JustBeforeEach(func() {
				setup([]runtime.Object{localRoute("", nil)}, nil)
				handle()
			})

			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should not interact with the remote cluster", func() {
				Expect(remoteClient.(*gwclientfake.Clientset).Actions()).To(BeEmpty())
			})
			It("should generate a warning event identifying the remote cluster and the missing resource", func() {
				var event *corev1.Event
				Eventually(events).Should(Receive(&event))
				Expect(event.Type).To(Equal(corev1.EventTypeWarning))
				Expect(event.Reason).To(Equal(forge.EventFailedReflection))
				Expect(event.Message).To(ContainSubstring(RemoteClusterID))
				Expect(event.Message).To(ContainSubstring(LiqoNodeName))
				Expect(event.Message).To(ContainSubstring("httproutes.gateway.networking.k8s.io not available in the remote cluster"))
				Expect(event.Source.Host).To(Equal(LiqoNodeName))
			})
			It("should list only the local routes", func() {
				Expect(reflector.List()).To(HaveLen(1))
			})
			It("should report the failure in the status", func() {
				expectFailureReported("httproutes.gateway.networking.k8s.io not available in the remote cluster")
			})
		})

		When("the local route is marked to be skipped", func() {
			JustBeforeEach(func() {
				setup([]runtime.Object{localRoute("", map[string]string{consts.SkipReflectionAnnotationKey: "true"})}, nil)
				handle()
			})

			It("should not generate any event", func() { Consistently(events).ShouldNot(Receive()) })
			It("should not report any status", func() {
				_, errShadow := getShadow()
				Expect(errShadow).To(BeNotFound())
			})
		})
	})

	Describe("the List function", func() {
		JustBeforeEach(func() { setup([]runtime.Object{localRoute("", nil)}, []runtime.Object{remoteRoute(true)}) })

		It("should return the routes in both clusters", func() {
			Expect(reflector.List()).To(ConsistOf(
				types.NamespacedName{Namespace: LocalNamespace, Name: RouteName},
				types.NamespacedName{Namespace: RemoteNamespace, Name: RouteName},
			))
		})
	})
})
