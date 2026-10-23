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

package gatewayapistatusctrl

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("ListenerSet status aggregation", func() {
	var (
		listenerSet gwv1.ListenerSet
		allowed     bool
		shadows     []offloadingv1beta1.ShadowListenerSetStatus
	)

	shadow := func(cluster string, programmed metav1.ConditionStatus, listeners ...gwv1.ListenerEntryStatus) offloadingv1beta1.ShadowListenerSetStatus {
		return offloadingv1beta1.ShadowListenerSetStatus{Spec: offloadingv1beta1.ShadowListenerSetStatusSpec{
			ListenerSetName: "app", ClusterID: cluster, Listeners: listeners,
			Conditions: []metav1.Condition{
				condition(string(gwv1.ListenerSetConditionAccepted), metav1.ConditionTrue, "Accepted", ""),
				condition(string(gwv1.ListenerSetConditionProgrammed), programmed, "Reason"+string(programmed), "msg-"+cluster),
			},
		}}
	}
	listener := func(attached int32, conflicted metav1.ConditionStatus) gwv1.ListenerEntryStatus {
		return gwv1.ListenerEntryStatus{Name: "https", AttachedRoutes: attached,
			SupportedKinds: []gwv1.RouteGroupKind{{Group: ptr.To[gwv1.Group](gwv1.GroupName), Kind: "HTTPRoute"}},
			Conditions: []metav1.Condition{
				condition(string(gwv1.ListenerEntryConditionAccepted), metav1.ConditionTrue, "Accepted", ""),
				condition(string(gwv1.ListenerEntryConditionConflicted), conflicted, "HostnameConflict", "hostname already used"),
			}}
	}

	BeforeEach(func() {
		allowed = true
		listenerSet = gwv1.ListenerSet{
			ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: namespace, Generation: 3},
			Spec: gwv1.ListenerSetSpec{ParentRef: gwv1.ParentGatewayReference{Name: "web"}, Listeners: []gwv1.ListenerEntry{
				{Name: "https", Port: 443, Protocol: gwv1.HTTPSProtocolType}, {Name: "other", Port: 443, Protocol: gwv1.HTTPSProtocolType},
			}},
		}
		shadows = []offloadingv1beta1.ShadowListenerSetStatus{
			shadow("cluster-b", metav1.ConditionFalse, listener(1, metav1.ConditionTrue)),
			shadow("cluster-a", metav1.ConditionTrue, listener(2, metav1.ConditionFalse)),
		}
	})

	JustBeforeEach(func() { AggregateListenerSetStatus(&listenerSet, allowed, shadows) })

	It("should be programmed if programmed in at least one cluster, reporting the clusters where it is not", func() {
		programmed := meta.FindStatusCondition(listenerSet.Status.Conditions, string(gwv1.ListenerSetConditionProgrammed))
		Expect(programmed.Status).To(Equal(metav1.ConditionTrue))
		Expect(programmed.ObservedGeneration).To(BeEquivalentTo(3))
		Expect(programmed.Message).To(Equal(`Condition satisfied in cluster(s) cluster-a; not satisfied in cluster "cluster-b": msg-cluster-b`))
	})
	It("should aggregate the status of the listeners", func() {
		Expect(listenerSet.Status.Listeners).To(HaveLen(2))
		https := listenerSet.Status.Listeners[0]
		Expect(https.Name).To(BeEquivalentTo("https"))
		Expect(https.AttachedRoutes).To(BeEquivalentTo(3))
		Expect(https.SupportedKinds).To(HaveLen(1))
		Expect(meta.IsStatusConditionTrue(https.Conditions, string(gwv1.ListenerEntryConditionAccepted))).To(BeTrue())
	})
	It("should report the listeners conflicting in at least one cluster", func() {
		conflicted := meta.FindStatusCondition(listenerSet.Status.Listeners[0].Conditions, string(gwv1.ListenerEntryConditionConflicted))
		Expect(conflicted.Status).To(Equal(metav1.ConditionTrue))
		Expect(conflicted.Reason).To(Equal("HostnameConflict"))
		Expect(conflicted.Message).To(ContainSubstring("cluster-b"))
	})
	It("should report the listeners not reported by any cluster as pending", func() {
		other := listenerSet.Status.Listeners[1]
		Expect(other.Name).To(BeEquivalentTo("other"))
		Expect(meta.FindStatusCondition(other.Conditions, string(gwv1.ListenerEntryConditionProgrammed)).Status).To(Equal(metav1.ConditionUnknown))
	})

	When("no remote cluster reports the status yet", func() {
		BeforeEach(func() { shadows = nil })

		It("should be pending", func() {
			accepted := meta.FindStatusCondition(listenerSet.Status.Conditions, string(gwv1.ListenerSetConditionAccepted))
			Expect(accepted.Status).To(Equal(metav1.ConditionUnknown))
			Expect(accepted.Message).To(Equal("Not yet reflected to any remote cluster"))
		})
	})

	When("the Gateway does not allow its attachment", func() {
		BeforeEach(func() { allowed = false })

		It("should not be accepted", func() {
			accepted := meta.FindStatusCondition(listenerSet.Status.Conditions, string(gwv1.ListenerSetConditionAccepted))
			Expect(accepted.Status).To(Equal(metav1.ConditionFalse))
			Expect(accepted.Reason).To(Equal(string(gwv1.ListenerSetReasonNotAllowed)))
			Expect(listenerSet.Status.Listeners).To(BeEmpty())
		})
	})
})

var _ = Describe("Status ownership", func() {
	// The Liqo controller manager runs in both the consumer and the provider clusters: the status of the Gateway API resources
	// not managed by Liqo (e.g., the ones reflected in the provider cluster, and served by its Gateway controller) shall never
	// be modified, as it happened for the ingresses of other classes.
	var (
		ctx     context.Context
		cl      client.Client
		objects []client.Object
	)

	envoyStatus := []metav1.Condition{condition("Accepted", metav1.ConditionTrue, "Accepted", "by envoy"),
		condition("Programmed", metav1.ConditionTrue, "Programmed", "by envoy")}
	envoyParent := func(name string) gwv1.RouteParentStatus {
		return gwv1.RouteParentStatus{ParentRef: gwv1.ParentReference{Name: gwv1.ObjectName(name), Namespace: ptr.To[gwv1.Namespace]("infra")},
			ControllerName: "gateway.envoyproxy.io/gatewayclass-controller", Conditions: envoyStatus[:1]}
	}

	BeforeEach(func() {
		ctx = context.Background()
		objects = []client.Object{
			&gwv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "liqo"}, Spec: gwv1.GatewayClassSpec{ControllerName: consts.GatewayControllerName}},
			&gwv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "eg"},
				Spec:   gwv1.GatewayClassSpec{ControllerName: "gateway.envoyproxy.io/gatewayclass-controller"},
				Status: gwv1.GatewayClassStatus{Conditions: envoyStatus[:1]}},
			&gwv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "public", Namespace: "infra"}, Spec: gwv1.GatewaySpec{GatewayClassName: "eg"},
				Status: gwv1.GatewayStatus{Conditions: envoyStatus, AttachedListenerSets: ptr.To[int32](4),
					Addresses: []gwv1.GatewayStatusAddress{{Value: "1.2.3.4"}}}},
			&gwv1.ListenerSet{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: namespace},
				Spec:   gwv1.ListenerSetSpec{ParentRef: gwv1.ParentGatewayReference{Name: "public", Namespace: ptr.To[gwv1.Namespace]("infra")}},
				Status: gwv1.ListenerSetStatus{Conditions: envoyStatus}},
			&gwv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: "route", Namespace: namespace},
				Status: gwv1.HTTPRouteStatus{RouteStatus: gwv1.RouteStatus{Parents: []gwv1.RouteParentStatus{envoyParent("public"), envoyParent("other")}}}},
		}
	})

	JustBeforeEach(func() { cl = newClient(objects...) })

	// unchanged asserts that the given object, after the reconciliation, is identical to the original one (including the resource version).
	unchanged := func(key types.NamespacedName, obj client.Object, reconcile func() error) {
		Expect(cl.Get(ctx, key, obj)).To(Succeed())
		before := obj.DeepCopyObject().(client.Object)
		Expect(reconcile()).To(Succeed())
		Expect(cl.Get(ctx, key, obj)).To(Succeed())
		Expect(obj).To(Equal(before))
	}
	request := func(namespace, name string) ctrl.Request {
		return ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}}
	}

	It("should not modify the GatewayClasses of other controllers", func() {
		// The predicate filters them out, while the reconciler would accept any class it is invoked for.
		Expect(isVirtualGatewayClass(&gwv1.GatewayClass{Spec: gwv1.GatewayClassSpec{ControllerName: "gateway.envoyproxy.io/gatewayclass-controller"}})).
			To(BeFalse())
	})

	It("should not modify the Gateways of other classes", func() {
		unchanged(types.NamespacedName{Namespace: "infra", Name: "public"}, &gwv1.Gateway{}, func() error {
			_, err := (&GatewayReconciler{Client: cl, ListenerSetsAvailable: true}).Reconcile(ctx, request("infra", "public"))
			return err
		})
	})

	It("should not modify the ListenerSets attached to Gateways of other classes", func() {
		unchanged(types.NamespacedName{Namespace: namespace, Name: "app"}, &gwv1.ListenerSet{}, func() error {
			_, err := (&ListenerSetReconciler{Client: cl}).Reconcile(ctx, request(namespace, "app"))
			return err
		})
	})

	It("should not modify the routes served by other controllers only", func() {
		unchanged(types.NamespacedName{Namespace: namespace, Name: "route"}, &gwv1.HTTPRoute{}, func() error {
			_, err := (&RouteReconciler{Client: cl, Kind: offloadingv1beta1.HTTPRouteKind}).Reconcile(ctx, request(namespace, "route"))
			return err
		})
	})

	When("the route is attached to both a Gateway served by Liqo and Gateways served by other controllers", func() {
		BeforeEach(func() {
			route := objects[len(objects)-1].(*gwv1.HTTPRoute)
			// The entry managed by Liqo is in between the ones of the other controller.
			route.Status.Parents = []gwv1.RouteParentStatus{envoyParent("public"),
				{ParentRef: gwv1.ParentReference{Name: "web"}, ControllerName: consts.GatewayControllerName,
					Conditions: []metav1.Condition{condition("Accepted", metav1.ConditionTrue, "Accepted", "Condition satisfied in cluster(s) cluster-a")}},
				envoyParent("other")}
			objects = append(objects, &offloadingv1beta1.ShadowRouteStatus{
				ObjectMeta: metav1.ObjectMeta{Name: "httproute-route-cluster-a", Namespace: namespace},
				Spec: offloadingv1beta1.ShadowRouteStatusSpec{Kind: offloadingv1beta1.HTTPRouteKind, RouteName: "route", ClusterID: "cluster-a",
					Parents: []gwv1.RouteParentStatus{{ParentRef: gwv1.ParentReference{Name: "web"}, ControllerName: "example.com/remote",
						Conditions: []metav1.Condition{condition("Accepted", metav1.ConditionTrue, "Accepted", "")}}}},
			})
		})

		It("should preserve the order of the entries, hence not update the route if the aggregated status is unchanged", func() {
			var route gwv1.HTTPRoute
			key := types.NamespacedName{Namespace: namespace, Name: "route"}
			// The first reconciliation sets the last transition times, the following ones shall not modify the route.
			_, err := (&RouteReconciler{Client: cl, Kind: offloadingv1beta1.HTTPRouteKind}).Reconcile(ctx, request(namespace, "route"))
			Expect(err).ToNot(HaveOccurred())
			Expect(cl.Get(ctx, key, &route)).To(Succeed())
			Expect(route.Status.Parents).To(HaveLen(3))
			Expect(route.Status.Parents[0]).To(Equal(envoyParent("public")))
			Expect(route.Status.Parents[1].ControllerName).To(BeEquivalentTo(consts.GatewayControllerName))
			Expect(route.Status.Parents[2]).To(Equal(envoyParent("other")))

			unchanged(key, &gwv1.HTTPRoute{}, func() error {
				_, err := (&RouteReconciler{Client: cl, Kind: offloadingv1beta1.HTTPRouteKind}).Reconcile(ctx, request(namespace, "route"))
				return err
			})
		})
	})

	When("the ListenerSet is attached to a Gateway of the virtual class", func() {
		BeforeEach(func() {
			objects = append(objects,
				&gwv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: namespace},
					Spec: gwv1.GatewaySpec{GatewayClassName: "liqo",
						AllowedListeners: &gwv1.AllowedListeners{Namespaces: &gwv1.ListenerNamespaces{From: ptr.To(gwv1.NamespacesFromSame)}}}},
				&gwv1.ListenerSet{ObjectMeta: metav1.ObjectMeta{Name: "tls", Namespace: namespace},
					Spec: gwv1.ListenerSetSpec{ParentRef: gwv1.ParentGatewayReference{Name: "web"},
						Listeners: []gwv1.ListenerEntry{{Name: "https", Port: 443, Protocol: gwv1.HTTPSProtocolType}}}},
				&offloadingv1beta1.ShadowListenerSetStatus{ObjectMeta: metav1.ObjectMeta{Name: "tls-cluster-a", Namespace: namespace},
					Spec: offloadingv1beta1.ShadowListenerSetStatusSpec{ListenerSetName: "tls", ClusterID: "cluster-a", Conditions: envoyStatus}},
			)
		})

		It("should aggregate its status, and count it among the ListenerSets attached to the Gateway", func() {
			_, err := (&ListenerSetReconciler{Client: cl}).Reconcile(ctx, request(namespace, "tls"))
			Expect(err).ToNot(HaveOccurred())
			var listenerSet gwv1.ListenerSet
			Expect(cl.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "tls"}, &listenerSet)).To(Succeed())
			Expect(meta.IsStatusConditionTrue(listenerSet.Status.Conditions, string(gwv1.ListenerSetConditionAccepted))).To(BeTrue())

			_, err = (&GatewayReconciler{Client: cl, ListenerSetsAvailable: true}).Reconcile(ctx, request(namespace, "web"))
			Expect(err).ToNot(HaveOccurred())
			var gateway gwv1.Gateway
			Expect(cl.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "web"}, &gateway)).To(Succeed())
			Expect(gateway.Status.AttachedListenerSets).To(PointTo(BeEquivalentTo(1)))
		})

		It("should delete the orphan shadows once the ListenerSet is deleted", func() {
			Expect(cl.Delete(ctx, &gwv1.ListenerSet{ObjectMeta: metav1.ObjectMeta{Name: "tls", Namespace: namespace}})).To(Succeed())
			_, err := (&ListenerSetReconciler{Client: cl}).Reconcile(ctx, request(namespace, "tls"))
			Expect(err).ToNot(HaveOccurred())
			var shadows offloadingv1beta1.ShadowListenerSetStatusList
			Expect(cl.List(ctx, &shadows)).To(Succeed())
			Expect(shadows.Items).To(BeEmpty())
		})
	})
})
