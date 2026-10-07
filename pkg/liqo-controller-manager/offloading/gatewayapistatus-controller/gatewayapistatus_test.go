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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

const namespace = "shop"

func condition(conditionType string, status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{Type: conditionType, Status: status, Reason: reason, Message: message}
}

func newClient(objects ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	Expect(offloadingv1beta1.AddToScheme(scheme)).To(Succeed())
	Expect(gwv1.Install(scheme)).To(Succeed())

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).
		WithStatusSubresource(&gwv1.GatewayClass{}, &gwv1.Gateway{}, &gwv1.HTTPRoute{}, &gwv1.GRPCRoute{}).
		WithIndex(&offloadingv1beta1.ShadowGatewayStatus{}, gatewayNameField, gatewayNameIndexer).
		WithIndex(&offloadingv1beta1.ShadowRouteStatus{}, routeNameField, routeNameIndexer).
		Build()
}

var _ = Describe("Gateway status aggregation", func() {
	var (
		gateway gwv1.Gateway
		shadows []offloadingv1beta1.ShadowGatewayStatus
	)

	shadow := func(cluster string, programmed metav1.ConditionStatus, address string, attached int32) offloadingv1beta1.ShadowGatewayStatus {
		return offloadingv1beta1.ShadowGatewayStatus{
			ObjectMeta: metav1.ObjectMeta{Name: "web-" + cluster, Namespace: namespace},
			Spec: offloadingv1beta1.ShadowGatewayStatusSpec{
				GatewayName: "web", ClusterID: cluster,
				Addresses:  []gwv1.GatewayStatusAddress{{Type: ptr.To(gwv1.IPAddressType), Value: address}},
				Conditions: []metav1.Condition{condition(string(gwv1.GatewayConditionProgrammed), programmed, "Reason", "message of "+cluster)},
				Listeners: []gwv1.ListenerStatus{{
					Name: "http", AttachedRoutes: attached,
					SupportedKinds: []gwv1.RouteGroupKind{{Kind: "HTTPRoute"}},
					Conditions:     []metav1.Condition{condition(string(gwv1.ListenerConditionProgrammed), programmed, "Reason", "")},
				}},
			},
		}
	}

	BeforeEach(func() {
		gateway = gwv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: namespace, Generation: 3},
			Spec:       gwv1.GatewaySpec{GatewayClassName: "liqo", Listeners: []gwv1.Listener{{Name: "http", Port: 80, Protocol: gwv1.HTTPProtocolType}}},
		}
		shadows = nil
	})

	JustBeforeEach(func() { AggregateGatewayStatus(&gateway, shadows) })

	When("the Gateway is not yet reflected to any cluster", func() {
		It("should be accepted, but not programmed", func() {
			Expect(meta.IsStatusConditionTrue(gateway.Status.Conditions, string(gwv1.GatewayConditionAccepted))).To(BeTrue())
			programmed := meta.FindStatusCondition(gateway.Status.Conditions, string(gwv1.GatewayConditionProgrammed))
			Expect(programmed.Status).To(Equal(metav1.ConditionFalse))
			Expect(programmed.Reason).To(Equal(string(gwv1.GatewayReasonPending)))
			Expect(programmed.ObservedGeneration).To(BeEquivalentTo(3))
		})
		It("should report no addresses", func() { Expect(gateway.Status.Addresses).To(BeEmpty()) })
	})

	When("the Gateway is reflected to multiple clusters", func() {
		BeforeEach(func() {
			shadows = []offloadingv1beta1.ShadowGatewayStatus{
				shadow("cluster-b", metav1.ConditionFalse, "10.0.0.2", 1),
				shadow("cluster-a", metav1.ConditionTrue, "10.0.0.1", 2),
			}
		})

		It("should report the union of the addresses", func() {
			Expect(gateway.Status.Addresses).To(ConsistOf(HaveField("Value", "10.0.0.1"), HaveField("Value", "10.0.0.2")))
		})
		It("should be programmed, reporting the clusters where it is not", func() {
			programmed := meta.FindStatusCondition(gateway.Status.Conditions, string(gwv1.GatewayConditionProgrammed))
			Expect(programmed.Status).To(Equal(metav1.ConditionTrue))
			Expect(programmed.Message).To(ContainSubstring("Programmed in cluster(s) cluster-a"))
			Expect(programmed.Message).To(ContainSubstring(`cluster "cluster-b": message of cluster-b`))
		})
		It("should aggregate the status of the listeners", func() {
			Expect(gateway.Status.Listeners).To(HaveLen(1))
			Expect(gateway.Status.Listeners[0].AttachedRoutes).To(BeEquivalentTo(3))
			Expect(gateway.Status.Listeners[0].SupportedKinds).To(ConsistOf(gwv1.RouteGroupKind{Kind: "HTTPRoute"}))
			Expect(meta.IsStatusConditionTrue(gateway.Status.Listeners[0].Conditions, string(gwv1.ListenerConditionProgrammed))).To(BeTrue())
		})
	})
})

var _ = Describe("Route status aggregation", func() {
	var (
		current  []gwv1.RouteParentStatus
		shadows  []offloadingv1beta1.ShadowRouteStatus
		result   []gwv1.RouteParentStatus
		degraded []string
	)

	edge := gwv1.ParentReference{Name: "edge", Namespace: ptr.To[gwv1.Namespace]("infra")}
	web := gwv1.ParentReference{Name: "web", SectionName: ptr.To[gwv1.SectionName]("http")}
	api := gwv1.ParentReference{Name: "api"}

	shadow := func(cluster string, parents ...gwv1.RouteParentStatus) offloadingv1beta1.ShadowRouteStatus {
		return offloadingv1beta1.ShadowRouteStatus{
			ObjectMeta: metav1.ObjectMeta{Name: "httproute-route-" + cluster, Namespace: namespace},
			Spec: offloadingv1beta1.ShadowRouteStatusSpec{
				Kind: offloadingv1beta1.HTTPRouteKind, RouteName: "route", ClusterID: cluster, Parents: parents},
		}
	}

	parent := func(ref gwv1.ParentReference, accepted metav1.ConditionStatus, message string) gwv1.RouteParentStatus {
		return gwv1.RouteParentStatus{ParentRef: ref, ControllerName: "example.com/remote", Conditions: []metav1.Condition{
			condition(string(gwv1.RouteConditionAccepted), accepted, "Reason"+string(accepted), message),
			condition(string(gwv1.RouteConditionResolvedRefs), metav1.ConditionTrue, "ResolvedRefs", ""),
		}}
	}

	BeforeEach(func() {
		// The entry of the local controller is preserved, while the stale one of Liqo is replaced.
		current = []gwv1.RouteParentStatus{
			{ParentRef: edge, ControllerName: "example.com/local", Conditions: []metav1.Condition{
				condition(string(gwv1.RouteConditionAccepted), metav1.ConditionTrue, "Accepted", "")}},
			{ParentRef: gwv1.ParentReference{Name: "stale"}, ControllerName: consts.GatewayControllerName},
		}
		shadows = []offloadingv1beta1.ShadowRouteStatus{
			shadow("cluster-a", parent(edge, metav1.ConditionTrue, ""), parent(web, metav1.ConditionTrue, ""),
				parent(api, metav1.ConditionUnknown, "Pending")),
			shadow("cluster-b", parent(edge, metav1.ConditionFalse, "NotAllowedByListeners"), parent(api, metav1.ConditionFalse, "NoMatchingParent")),
		}
	})

	JustBeforeEach(func() { result, degraded = AggregateRouteParents(current, shadows, namespace, 7) })

	It("should preserve the entries of the other controllers", func() {
		Expect(result).To(ContainElement(current[0]))
	})
	It("should add one entry managed by Liqo for each parent, replacing the stale ones", func() {
		liqo := []gwv1.ParentReference{}
		for i := range result {
			if result[i].ControllerName == gwv1.GatewayController(consts.GatewayControllerName) {
				liqo = append(liqo, result[i].ParentRef)
			}
		}
		Expect(liqo).To(ConsistOf(edge, web, api))
	})
	It("should aggregate the conditions reported by the remote clusters", func() {
		for i := range result {
			if result[i].ControllerName != gwv1.GatewayController(consts.GatewayControllerName) {
				continue
			}
			accepted := meta.FindStatusCondition(result[i].Conditions, string(gwv1.RouteConditionAccepted))
			Expect(accepted.ObservedGeneration).To(BeEquivalentTo(7))
			Expect(accepted.LastTransitionTime.IsZero()).To(BeFalse())
			switch result[i].ParentRef.Name {
			case "edge":
				// Accepted in at least one cluster, reporting the clusters where it is not.
				Expect(accepted.Status).To(Equal(metav1.ConditionTrue))
				Expect(accepted.Reason).To(Equal("ReasonTrue"))
				Expect(accepted.Message).To(Equal(
					`Condition satisfied in cluster(s) cluster-a; not satisfied in cluster "cluster-b": NotAllowedByListeners`))
			case "web":
				Expect(accepted.Status).To(Equal(metav1.ConditionTrue))
				Expect(accepted.Message).To(Equal("Condition satisfied in cluster(s) cluster-a"))
			case "api":
				// Not accepted in any cluster: False takes precedence over Unknown.
				Expect(accepted.Status).To(Equal(metav1.ConditionFalse))
				Expect(accepted.Reason).To(Equal("ReasonFalse"))
				Expect(accepted.Message).To(Equal(`cluster "cluster-a": Pending; cluster "cluster-b": NoMatchingParent`))
			}
			Expect(meta.IsStatusConditionTrue(result[i].Conditions, string(gwv1.RouteConditionResolvedRefs))).To(BeTrue())
		}
	})

	It("should return the parents accepted only by part of the remote clusters", func() {
		Expect(degraded).To(ConsistOf(`parent "edge" not accepted in cluster "cluster-b": NotAllowedByListeners`))
	})

	When("a condition is no longer reported by any remote cluster", func() {
		var accepted metav1.Condition

		BeforeEach(func() {
			accepted = condition(string(gwv1.RouteConditionAccepted), metav1.ConditionTrue, "Accepted", "")
			accepted.LastTransitionTime = metav1.NewTime(metav1.Now().Add(-time.Hour))
			current = append(current, gwv1.RouteParentStatus{ParentRef: web, ControllerName: consts.GatewayControllerName,
				Conditions: []metav1.Condition{accepted,
					condition("BackendsAvailable", metav1.ConditionFalse, "NoEndpoints", `cluster "cluster-a": no ready endpoints`)}})
		})

		It("should remove it, while preserving the last transition time of the others", func() {
			for i := range result {
				if result[i].ControllerName != gwv1.GatewayController(consts.GatewayControllerName) || result[i].ParentRef.Name != "web" {
					continue
				}
				Expect(meta.FindStatusCondition(result[i].Conditions, "BackendsAvailable")).To(BeNil())
				Expect(meta.FindStatusCondition(result[i].Conditions, string(gwv1.RouteConditionAccepted)).LastTransitionTime).
					To(Equal(accepted.LastTransitionTime))
			}
		})
	})
})

var _ = Describe("Reconcilers", func() {
	var (
		ctx context.Context
		cl  client.Client
	)

	BeforeEach(func() { ctx = context.Background() })

	Describe("the GatewayClass reconciler", func() {
		It("should accept the virtual GatewayClasses", func() {
			cl = newClient(&gwv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "liqo", Generation: 2},
				Spec: gwv1.GatewayClassSpec{ControllerName: consts.GatewayControllerName}})
			_, err := (&GatewayClassReconciler{Client: cl}).Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "liqo"}})
			Expect(err).ToNot(HaveOccurred())

			var class gwv1.GatewayClass
			Expect(cl.Get(ctx, types.NamespacedName{Name: "liqo"}, &class)).To(Succeed())
			accepted := meta.FindStatusCondition(class.Status.Conditions, string(gwv1.GatewayClassConditionStatusAccepted))
			Expect(accepted).ToNot(BeNil())
			Expect(accepted.Status).To(Equal(metav1.ConditionTrue))
			Expect(accepted.ObservedGeneration).To(BeEquivalentTo(2))
		})
	})

	Describe("the Gateway reconciler", func() {
		var (
			class   string
			objects []client.Object
		)

		req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "web"}}
		shadowObj := &offloadingv1beta1.ShadowGatewayStatus{
			ObjectMeta: metav1.ObjectMeta{Name: "web-cluster-a", Namespace: namespace},
			Spec: offloadingv1beta1.ShadowGatewayStatusSpec{GatewayName: "web", ClusterID: "cluster-a",
				Addresses: []gwv1.GatewayStatusAddress{{Value: "10.0.0.1"}}},
		}

		BeforeEach(func() {
			class = "liqo"
			objects = []client.Object{
				&gwv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "liqo"}, Spec: gwv1.GatewayClassSpec{ControllerName: consts.GatewayControllerName}},
				&gwv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "envoy"}, Spec: gwv1.GatewayClassSpec{ControllerName: "example.com/envoy"}},
				shadowObj.DeepCopy(),
			}
		})

		JustBeforeEach(func() {
			cl = newClient(objects...)
			_, err := (&GatewayReconciler{Client: cl}).Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())
		})

		When("the Gateway belongs to the virtual class", func() {
			BeforeEach(func() {
				objects = append(objects, &gwv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: namespace},
					Spec: gwv1.GatewaySpec{GatewayClassName: gwv1.ObjectName(class)}})
			})

			It("should aggregate the status", func() {
				var gateway gwv1.Gateway
				Expect(cl.Get(ctx, req.NamespacedName, &gateway)).To(Succeed())
				Expect(gateway.Status.Addresses).To(ConsistOf(HaveField("Value", "10.0.0.1")))
				Expect(meta.IsStatusConditionTrue(gateway.Status.Conditions, string(gwv1.GatewayConditionAccepted))).To(BeTrue())
			})

			When("it belongs to another class", func() {
				BeforeEach(func() { objects[len(objects)-1].(*gwv1.Gateway).Spec.GatewayClassName = "envoy" })

				It("should not modify the status", func() {
					var gateway gwv1.Gateway
					Expect(cl.Get(ctx, req.NamespacedName, &gateway)).To(Succeed())
					Expect(gateway.Status.Conditions).To(BeEmpty())
				})
			})
		})

		When("the Gateway does not exist", func() {
			It("should delete the orphan shadows", func() {
				var shadows offloadingv1beta1.ShadowGatewayStatusList
				Expect(cl.List(ctx, &shadows)).To(Succeed())
				Expect(shadows.Items).To(BeEmpty())
			})
		})
	})

	Describe("the route reconciler", func() {
		var objects []client.Object

		req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "route"}}
		shadowObj := func(kind offloadingv1beta1.RouteKind) *offloadingv1beta1.ShadowRouteStatus {
			return &offloadingv1beta1.ShadowRouteStatus{
				ObjectMeta: metav1.ObjectMeta{Name: string(kind) + "-route-cluster-a", Namespace: namespace},
				Spec: offloadingv1beta1.ShadowRouteStatusSpec{Kind: kind, RouteName: "route", ClusterID: "cluster-a",
					Parents: []gwv1.RouteParentStatus{{ParentRef: gwv1.ParentReference{Name: "web"}, ControllerName: "example.com/remote",
						Conditions: []metav1.Condition{condition(string(gwv1.RouteConditionAccepted), metav1.ConditionTrue, "Accepted", "")}}}},
			}
		}

		BeforeEach(func() {
			objects = []client.Object{shadowObj(offloadingv1beta1.HTTPRouteKind), shadowObj(offloadingv1beta1.GRPCRouteKind)}
		})

		JustBeforeEach(func() {
			cl = newClient(objects...)
			_, err := (&RouteReconciler{Client: cl, Kind: offloadingv1beta1.HTTPRouteKind}).Reconcile(ctx, req)
			Expect(err).ToNot(HaveOccurred())
		})

		When("the route exists", func() {
			BeforeEach(func() {
				objects = append(objects, &gwv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: "route", Namespace: namespace}})
			})

			It("should aggregate the status", func() {
				var route gwv1.HTTPRoute
				Expect(cl.Get(ctx, req.NamespacedName, &route)).To(Succeed())
				Expect(route.Status.Parents).To(ConsistOf(And(
					HaveField("ControllerName", gwv1.GatewayController(consts.GatewayControllerName)),
					HaveField("ParentRef.Name", gwv1.ObjectName("web")),
				)))
			})
		})

		When("the route does not exist", func() {
			It("should delete only the orphan shadows of the same kind", func() {
				var shadows offloadingv1beta1.ShadowRouteStatusList
				Expect(cl.List(ctx, &shadows)).To(Succeed())
				Expect(shadows.Items).To(ConsistOf(HaveField("Spec.Kind", offloadingv1beta1.GRPCRouteKind)))
			})
		})
	})
})
