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

package firewall

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/fabric"
	firewallpkg "github.com/liqotech/liqo/pkg/firewall"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/remapping"
)

// ---------- helpers (Phase 2) ----------

var _ = Describe("attachResourceName helper", func() {
	It("returns <fwcfg>-<entity> when short enough", func() {
		Expect(attachResourceName("fw", "node-a")).To(Equal("fw-node-a"))
	})

	It("preserves the entity suffix when the joined name exceeds 253 chars", func() {
		longFw := strings.Repeat("a", 260)
		entity := "node-suffix"
		got := attachResourceName(longFw, entity)
		Expect(len(got)).To(BeNumerically("<=", 253))
		Expect(got).To(HaveSuffix("-" + entity))
	})

	It("returns names that match (i.e. is deterministic)", func() {
		a := attachResourceName("fw", "n")
		b := attachResourceName("fw", "n")
		Expect(a).To(Equal(b))
	})
})

var _ = Describe("isOwnedBy helper", func() {
	It("returns true when an ownerReference UID matches", func() {
		obj := &networkingv1beta1.FirewallConfigurationAttach{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{UID: types.UID("other")},
					{UID: types.UID("match")},
				},
			},
		}
		Expect(isOwnedBy(obj, types.UID("match"))).To(BeTrue())
	})

	It("returns false when no ownerReference UID matches", func() {
		obj := &networkingv1beta1.FirewallConfigurationAttach{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{{UID: types.UID("other")}},
			},
		}
		Expect(isOwnedBy(obj, types.UID("match"))).To(BeFalse())
	})

	It("returns false when there are no ownerReferences", func() {
		obj := &networkingv1beta1.FirewallConfigurationAttach{}
		Expect(isOwnedBy(obj, types.UID("anything"))).To(BeFalse())
	})
})

// ---------- AttachCreatorReconciler (Phase 3) ----------

const fwcfgNamespace = "liqo-tenant"

func newFakeReconciler(objs ...client.Object) *AttachCreatorReconciler {
	cb := fake.NewClientBuilder().WithScheme(scheme.Scheme)
	if len(objs) > 0 {
		cb = cb.WithObjects(objs...)
	}
	return NewAttachCreatorReconciler(cb.Build(), scheme.Scheme)
}

func newFwcfg(name string, labels map[string]string) *networkingv1beta1.FirewallConfiguration {
	return &networkingv1beta1.FirewallConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: fwcfgNamespace,
			UID:       types.UID("fwcfg-uid-" + name),
			Labels:    labels,
		},
	}
}

func req(name string) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: fwcfgNamespace}}
}

func listAttaches(ctx context.Context, c client.Client) []networkingv1beta1.FirewallConfigurationAttach {
	var l networkingv1beta1.FirewallConfigurationAttachList
	Expect(c.List(ctx, &l, client.InNamespace(fwcfgNamespace))).To(Succeed())
	return l.Items
}

var _ = Describe("AttachCreatorReconciler", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Context("when the FirewallConfiguration is not found", func() {
		It("returns an empty result without error", func() {
			r := newFakeReconciler()
			res, err := r.Reconcile(ctx, req("missing"))
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(ctrl.Result{}))
		})
	})

	Context("when the FirewallConfiguration is being deleted", func() {
		It("does not create any attach (GC handles cascade)", func() {
			fwcfg := newFwcfg("fw-del", fabric.ForgeFirewallTargetLabels())
			now := metav1.Now()
			fwcfg.DeletionTimestamp = &now
			fwcfg.Finalizers = []string{"keep"} // required for the fake client to accept a deletion-timestamp object
			r := newFakeReconciler(fwcfg)

			res, err := r.Reconcile(ctx, req("fw-del"))
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(ctrl.Result{}))
			Expect(listAttaches(ctx, r.Client)).To(BeEmpty())
		})
	})

	Context("when the FirewallConfiguration has an unknown category", func() {
		It("returns successfully and creates no attaches", func() {
			fwcfg := newFwcfg("fw-unknown", map[string]string{
				firewallpkg.FirewallCategoryTargetKey: "bogus",
			})
			r := newFakeReconciler(fwcfg)

			res, err := r.Reconcile(ctx, req("fw-unknown"))
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(ctrl.Result{}))
			Expect(listAttaches(ctx, r.Client)).To(BeEmpty())
		})
	})

	Context("fabric / all-nodes", func() {
		It("creates one attach per InternalNode with the expected labels, owner and spec", func() {
			fwcfg := newFwcfg("fw-fabric", fabric.ForgeFirewallTargetLabels())
			nodes := []client.Object{
				&networkingv1beta1.InternalNode{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}},
				&networkingv1beta1.InternalNode{ObjectMeta: metav1.ObjectMeta{Name: "node-b"}},
				&networkingv1beta1.InternalNode{ObjectMeta: metav1.ObjectMeta{Name: "node-c"}},
			}
			r := newFakeReconciler(append(nodes, fwcfg)...)

			_, err := r.Reconcile(ctx, req("fw-fabric"))
			Expect(err).ToNot(HaveOccurred())

			items := listAttaches(ctx, r.Client)
			Expect(items).To(HaveLen(3))
			seen := map[string]struct{}{}
			for i := range items {
				a := items[i]
				seen[a.Name] = struct{}{}
				Expect(a.Namespace).To(Equal(fwcfgNamespace))
				Expect(a.Spec.FirewallConfigurationRef.Name).To(Equal("fw-fabric"))
				// Owner ref points to the FWCfg.
				Expect(a.OwnerReferences).To(HaveLen(1))
				Expect(a.OwnerReferences[0].UID).To(Equal(fwcfg.UID))
				// Labels are derived from the node name suffix of the attach.
				nodeName := strings.TrimPrefix(a.Name, "fw-fabric-")
				Expect(a.Labels).To(Equal(fabric.ForgeFirewallAttachTargetLabels(nodeName)))
			}
			Expect(seen).To(HaveKey("fw-fabric-node-a"))
			Expect(seen).To(HaveKey("fw-fabric-node-b"))
			Expect(seen).To(HaveKey("fw-fabric-node-c"))
		})

		It("is idempotent: a second reconcile produces no changes", func() {
			fwcfg := newFwcfg("fw-fabric", fabric.ForgeFirewallTargetLabels())
			node := &networkingv1beta1.InternalNode{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}}
			r := newFakeReconciler(fwcfg, node)

			_, err := r.Reconcile(ctx, req("fw-fabric"))
			Expect(err).ToNot(HaveOccurred())
			before := listAttaches(ctx, r.Client)

			_, err = r.Reconcile(ctx, req("fw-fabric"))
			Expect(err).ToNot(HaveOccurred())
			after := listAttaches(ctx, r.Client)

			Expect(after).To(HaveLen(len(before)))
			Expect(after[0].UID).To(Equal(before[0].UID))
		})
	})

	Context("fabric / single-node", func() {
		It("creates exactly one attach for the targeted node, ignoring other InternalNodes", func() {
			fwcfg := newFwcfg("fw-single", fabric.ForgeFirewallTargetLabelsSingleNode("node-target"))
			objs := []client.Object{
				fwcfg,
				&networkingv1beta1.InternalNode{ObjectMeta: metav1.ObjectMeta{Name: "node-target"}},
				&networkingv1beta1.InternalNode{ObjectMeta: metav1.ObjectMeta{Name: "node-other"}},
			}
			r := newFakeReconciler(objs...)

			_, err := r.Reconcile(ctx, req("fw-single"))
			Expect(err).ToNot(HaveOccurred())

			items := listAttaches(ctx, r.Client)
			Expect(items).To(HaveLen(1))
			Expect(items[0].Name).To(Equal("fw-single-node-target"))
			Expect(items[0].Labels).To(Equal(fabric.ForgeFirewallAttachTargetLabelsSingleNode("node-target")))
		})
	})

	Context("fabric / ip-mapping", func() {
		It("creates one attach per InternalNode with remapping-fabric labels", func() {
			fwcfg := newFwcfg("fw-mapfab", map[string]string{
				firewallpkg.FirewallCategoryTargetKey:    fabric.FirewallCategoryTargetValue,
				firewallpkg.FirewallSubCategoryTargetKey: remapping.FirewallSubCategoryTargetValueIPMapping,
			})
			r := newFakeReconciler(
				fwcfg,
				&networkingv1beta1.InternalNode{ObjectMeta: metav1.ObjectMeta{Name: "n1"}},
				&networkingv1beta1.InternalNode{ObjectMeta: metav1.ObjectMeta{Name: "n2"}},
			)

			_, err := r.Reconcile(ctx, req("fw-mapfab"))
			Expect(err).ToNot(HaveOccurred())

			items := listAttaches(ctx, r.Client)
			Expect(items).To(HaveLen(2))
			for i := range items {
				nodeName := strings.TrimPrefix(items[i].Name, "fw-mapfab-")
				Expect(items[i].Labels).To(Equal(remapping.ForgeFirewallAttachTargetLabelsIPMappingFabric(nodeName)))
			}
		})
	})

	Context("gateway / all-gateways", func() {
		It("creates one attach per GatewayServer and per GatewayClient across all namespaces", func() {
			fwcfg := newFwcfg("fw-allgw", gateway.ForgeFirewallAllGatewaysTargetLabels())
			r := newFakeReconciler(
				fwcfg,
				&networkingv1beta1.GatewayServer{ObjectMeta: metav1.ObjectMeta{Name: "gws-1", Namespace: "tenant-1"}},
				&networkingv1beta1.GatewayServer{ObjectMeta: metav1.ObjectMeta{Name: "gws-2", Namespace: "tenant-2"}},
				&networkingv1beta1.GatewayClient{ObjectMeta: metav1.ObjectMeta{Name: "gwc-1", Namespace: "tenant-3"}},
			)

			_, err := r.Reconcile(ctx, req("fw-allgw"))
			Expect(err).ToNot(HaveOccurred())

			items := listAttaches(ctx, r.Client)
			Expect(items).To(HaveLen(3))
			byName := map[string]networkingv1beta1.FirewallConfigurationAttach{}
			for i := range items {
				byName[items[i].Name] = items[i]
			}
			Expect(byName).To(HaveKey("fw-allgw-gws-1"))
			Expect(byName).To(HaveKey("fw-allgw-gws-2"))
			Expect(byName).To(HaveKey("fw-allgw-gwc-1"))
			Expect(byName["fw-allgw-gws-1"].Labels).To(Equal(gateway.ForgeFirewallAttachAllGatewaysTargetLabels("gws-1")))
			Expect(byName["fw-allgw-gwc-1"].Labels).To(Equal(gateway.ForgeFirewallAttachAllGatewaysTargetLabels("gwc-1")))
		})
	})

	Context("gateway / fabric (internal)", func() {
		It("creates one attach per gateway with internal labels", func() {
			fwcfg := newFwcfg("fw-gwfab", gateway.ForgeFirewallInternalTargetLabels())
			r := newFakeReconciler(
				fwcfg,
				&networkingv1beta1.GatewayServer{ObjectMeta: metav1.ObjectMeta{Name: "gw-s", Namespace: "tenant-1"}},
			)

			_, err := r.Reconcile(ctx, req("fw-gwfab"))
			Expect(err).ToNot(HaveOccurred())

			items := listAttaches(ctx, r.Client)
			Expect(items).To(HaveLen(1))
			Expect(items[0].Labels).To(Equal(gateway.ForgeFirewallAttachInternalTargetLabels("gw-s")))
		})
	})

	Context("gateway / ip-mapping", func() {
		It("creates one attach per gateway across servers and clients", func() {
			fwcfg := newFwcfg("fw-gwmap", map[string]string{
				firewallpkg.FirewallCategoryTargetKey:    gateway.FirewallCategoryGwTargetValue,
				firewallpkg.FirewallSubCategoryTargetKey: remapping.FirewallSubCategoryTargetValueIPMapping,
			})
			r := newFakeReconciler(
				fwcfg,
				&networkingv1beta1.GatewayServer{ObjectMeta: metav1.ObjectMeta{Name: "srv-a", Namespace: "tenant-1"}},
				&networkingv1beta1.GatewayClient{ObjectMeta: metav1.ObjectMeta{Name: "cli-a", Namespace: "tenant-2"}},
			)

			_, err := r.Reconcile(ctx, req("fw-gwmap"))
			Expect(err).ToNot(HaveOccurred())

			items := listAttaches(ctx, r.Client)
			Expect(items).To(HaveLen(2))
			for i := range items {
				gwName := strings.TrimPrefix(items[i].Name, "fw-gwmap-")
				Expect(items[i].Labels).To(Equal(remapping.ForgeFirewallAttachTargetLabelsIPMappingGw(gwName)))
			}
		})
	})

	Context("gateway / no-subcategory (single remote cluster)", func() {
		It("creates one attach for the matching GatewayServer when only the server exists", func() {
			fwcfg := newFwcfg("fw-rid", map[string]string{
				firewallpkg.FirewallCategoryTargetKey: gateway.FirewallCategoryGwTargetValue,
				firewallpkg.FirewallUniqueTargetKey:   "remote-cluster-1",
			})
			gws := &networkingv1beta1.GatewayServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gw-srv",
					Namespace: "tenant-1",
					Labels:    map[string]string{consts.RemoteClusterID: "remote-cluster-1"},
				},
			}
			r := newFakeReconciler(fwcfg, gws)

			_, err := r.Reconcile(ctx, req("fw-rid"))
			Expect(err).ToNot(HaveOccurred())

			items := listAttaches(ctx, r.Client)
			Expect(items).To(HaveLen(1))
			Expect(items[0].Name).To(Equal("fw-rid-gw-srv"))
			Expect(items[0].Labels).To(Equal(remapping.ForgeFirewallAttachTargetLabels("remote-cluster-1", "gw-srv")))
		})

		It("creates no attach when no gateway matches the remote cluster ID", func() {
			fwcfg := newFwcfg("fw-rid-missing", map[string]string{
				firewallpkg.FirewallCategoryTargetKey: gateway.FirewallCategoryGwTargetValue,
				firewallpkg.FirewallUniqueTargetKey:   "no-such-cluster",
			})
			r := newFakeReconciler(fwcfg)

			_, err := r.Reconcile(ctx, req("fw-rid-missing"))
			Expect(err).ToNot(HaveOccurred())
			Expect(listAttaches(ctx, r.Client)).To(BeEmpty())
		})
	})

	Context("stale attach garbage-collection", func() {
		It("deletes an owned attach whose name is no longer expected, and strips any leftover finalizer first", func() {
			fwcfg := newFwcfg("fw-fabric", fabric.ForgeFirewallTargetLabels())
			node := &networkingv1beta1.InternalNode{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}}
			// Pre-existing stale attach owned by the FWCfg but for a node that no longer exists.
			stale := &networkingv1beta1.FirewallConfigurationAttach{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fw-fabric-node-gone",
					Namespace: fwcfgNamespace,
					Finalizers: []string{
						firewallpkg.FirewallConfigurationAttachControllerFinalizer,
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: networkingv1beta1.GroupVersion.String(),
							Kind:       "FirewallConfiguration",
							Name:       fwcfg.Name,
							UID:        fwcfg.UID,
						},
					},
				},
			}
			r := newFakeReconciler(fwcfg, node, stale)

			_, err := r.Reconcile(ctx, req("fw-fabric"))
			Expect(err).ToNot(HaveOccurred())

			items := listAttaches(ctx, r.Client)
			// node-gone attach must have been removed; node-a attach must have been created.
			Expect(items).To(HaveLen(1))
			Expect(items[0].Name).To(Equal("fw-fabric-node-a"))
		})

		It("does NOT delete an attach that is not owned by the FirewallConfiguration", func() {
			fwcfg := newFwcfg("fw-fabric", fabric.ForgeFirewallTargetLabels())
			node := &networkingv1beta1.InternalNode{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}}
			foreign := &networkingv1beta1.FirewallConfigurationAttach{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fw-other-thing",
					Namespace: fwcfgNamespace,
					// no ownerReferences pointing to fwcfg
				},
			}
			r := newFakeReconciler(fwcfg, node, foreign)

			_, err := r.Reconcile(ctx, req("fw-fabric"))
			Expect(err).ToNot(HaveOccurred())

			items := listAttaches(ctx, r.Client)
			names := map[string]struct{}{}
			for i := range items {
				names[items[i].Name] = struct{}{}
			}
			Expect(names).To(HaveKey("fw-fabric-node-a"))
			Expect(names).To(HaveKey("fw-other-thing"))
		})
	})

	Describe("enqueueFirewallConfigurationsByCategory", func() {
		It("returns only requests for FirewallConfigurations with the requested category", func() {
			fwFabric := newFwcfg("fw-fab", fabric.ForgeFirewallTargetLabels())
			fwGw := newFwcfg("fw-gw", gateway.ForgeFirewallAllGatewaysTargetLabels())
			r := newFakeReconciler(fwFabric, fwGw)

			fab := r.enqueueFirewallConfigurationsByCategory(context.Background(), fabric.FirewallCategoryTargetValue)
			Expect(fab).To(HaveLen(1))
			Expect(fab[0].Name).To(Equal("fw-fab"))

			gw := r.enqueueFirewallConfigurationsByCategory(context.Background(), gateway.FirewallCategoryGwTargetValue)
			Expect(gw).To(HaveLen(1))
			Expect(gw[0].Name).To(Equal("fw-gw"))
		})
	})
})
