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

package exposition_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/informers"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"k8s.io/utils/trace"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/cmd/virtual-kubelet/root"
	liqoclientfake "github.com/liqotech/liqo/pkg/client/clientset/versioned/fake"
	liqoinformers "github.com/liqotech/liqo/pkg/client/informers/externalversions"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/directconnection"
	"github.com/liqotech/liqo/pkg/utils/indexer"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/exposition"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/resources"
)

const (
	rvFourthClusterID       = "fourth-cluster-id"
	rvFourthClusterNodeName = "fourth-cluster-node"
	rvEPS                   = "rv-eps"
	rvSvc                   = "rv-svc"
	rvCompanion             = rvEPS + forge.IndirectEndpointSliceSuffix
)

// Reflection of the EndpointSlices of a Service with direct connections: the direct/indirect
// ShadowEndpointSlice pair, observed across several Handle invocations.
var _ = Describe("Direct-connections reflection", func() {
	var (
		liqoClient  *liqoclientfake.Clientset
		factory     informers.SharedInformerFactory
		liqoFactory liqoinformers.SharedInformerFactory
		ner         *exposition.NamespacedEndpointSliceReflector
		keyers      []options.Keyer
		eventsMu    sync.Mutex
		events      []corev1.Event

		thirdNode  = ptr.To(ThirdClusterNodeName)
		fourthNode = ptr.To(rvFourthClusterNodeName)
		localNode  = ptr.To(LocalClusterNodeName)
		targetNode = ptr.To(LiqoNodeName) // hosted on the provider this VK reflects to
	)

	ep := func(node *string, addr, pod string) discoveryv1.Endpoint {
		e := discoveryv1.Endpoint{NodeName: node, Addresses: []string{addr}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)}}
		if pod != "" {
			e.TargetRef = &corev1.ObjectReference{Kind: "Pod", Name: pod, Namespace: LocalNamespace}
		}
		return e
	}

	createServiceNamed := func(name string, annotated bool) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: LocalNamespace},
			Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)}}},
		}
		if annotated {
			svc.Annotations = map[string]string{consts.UseDirectConnectionAnnotationKey: "true"}
		}
		_, err := client.CoreV1().Services(LocalNamespace).Create(ctx, svc, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
	}

	createLocalFull := func(name, svc string, annotations map[string]string, endpoints ...discoveryv1.Endpoint) {
		eps := &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: LocalNamespace,
				Labels: map[string]string{discoveryv1.LabelServiceName: svc}, Annotations: annotations},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints:   endpoints,
		}
		_, err := client.DiscoveryV1().EndpointSlices(LocalNamespace).Create(ctx, eps, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
	}
	createLocal := func(endpoints ...discoveryv1.Endpoint) { createLocalFull(rvEPS, rvSvc, nil, endpoints...) }

	createIP := func(name, ip, remapped string) {
		obj := &ipamv1alpha1.IP{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: ipamv1alpha1.IPSpec{IP: networkingv1beta1.IP(ip)}}
		obj, err := liqoClient.IpamV1alpha1().IPs(LocalNamespace).Create(ctx, obj, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
		obj.Status = ipamv1alpha1.IPStatus{IP: networkingv1beta1.IP(remapped)}
		_, err = liqoClient.IpamV1alpha1().IPs(LocalNamespace).UpdateStatus(ctx, obj, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())
	}

	startReflector := func() {
		factory = informers.NewSharedInformerFactory(client, 10*time.Hour)
		liqoFactory = liqoinformers.NewSharedInformerFactory(liqoClient, 10*time.Hour)
		broadcaster := record.NewBroadcaster()
		broadcaster.StartEventWatcher(func(e *corev1.Event) {
			eventsMu.Lock()
			defer eventsMu.Unlock()
			events = append(events, *e)
		})
		keyers = nil
		r := exposition.NewNamespacedEndpointSliceReflector(localPodCIDRs)(options.NewNamespaced().
			WithLocal(LocalNamespace, client, factory).
			WithLiqoLocal(liqoClient, liqoFactory).
			WithRemote(RemoteNamespace, client, factory).
			WithLiqoRemote(liqoClient, liqoFactory).
			WithHandlerFactory(func(k options.Keyer, _ ...options.EventFilter) cache.ResourceEventHandler {
				keyers = append(keyers, k)
				return cache.ResourceEventHandlerFuncs{}
			}).
			WithEventBroadcaster(broadcaster).
			WithReflectionType(root.DefaultReflectorsTypes[resources.Service]).
			WithForgingOpts(FakeForgingOpts()))
		ner = r.(*exposition.NamespacedEndpointSliceReflector)
		factory.Start(ctx.Done())
		liqoFactory.Start(ctx.Done())
		factory.WaitForCacheSync(ctx.Done())
		liqoFactory.WaitForCacheSync(ctx.Done())
	}

	handle := func(name string) error {
		return ner.Handle(trace.ContextWithTrace(ctx, trace.New("EndpointSlice")), name)
	}

	getRemote := func(name string) (*offloadingv1beta1.ShadowEndpointSlice, error) {
		return liqoClient.OffloadingV1beta1().ShadowEndpointSlices(RemoteNamespace).Get(ctx, name, metav1.GetOptions{})
	}
	mustRemote := func(name string) *offloadingv1beta1.ShadowEndpointSlice {
		obj, err := getRemote(name)
		Expect(err).ToNot(HaveOccurred())
		return obj
	}

	// waitRemoteSynced waits for the informer cache to mirror the API state of the given remote objects,
	// so that the next Handle sees the result of the previous one (as it would in production).
	waitRemoteSynced := func(names ...string) {
		lister := liqoFactory.Offloading().V1beta1().ShadowEndpointSlices().Lister().ShadowEndpointSlices(RemoteNamespace)
		for _, n := range names {
			Eventually(func() bool {
				apiObj, apiErr := getRemote(n)
				cached, cacheErr := lister.Get(n)
				if kerrors.IsNotFound(apiErr) {
					return kerrors.IsNotFound(cacheErr)
				}
				return cacheErr == nil && reflect.DeepEqual(apiObj.Spec, cached.Spec) &&
					reflect.DeepEqual(apiObj.Annotations, cached.Annotations) && reflect.DeepEqual(apiObj.Labels, cached.Labels)
			}).Should(BeTrue(), "remote %q not synced", n)
		}
	}
	waitLocalSynced := func(name string) {
		lister := factory.Discovery().V1().EndpointSlices().Lister().EndpointSlices(LocalNamespace)
		Eventually(func() bool {
			apiObj, apiErr := client.DiscoveryV1().EndpointSlices(LocalNamespace).Get(ctx, name, metav1.GetOptions{})
			cached, cacheErr := lister.Get(name)
			if kerrors.IsNotFound(apiErr) {
				return kerrors.IsNotFound(cacheErr)
			}
			return cacheErr == nil && apiObj.ResourceVersion == cached.ResourceVersion
		}).Should(BeTrue(), "local %q not synced", name)
	}
	waitServiceSynced := func(name string) {
		lister := factory.Core().V1().Services().Lister().Services(LocalNamespace)
		Eventually(func() bool {
			apiObj, apiErr := client.CoreV1().Services(LocalNamespace).Get(ctx, name, metav1.GetOptions{})
			cached, cacheErr := lister.Get(name)
			return apiErr == nil && cacheErr == nil && apiObj.ResourceVersion == cached.ResourceVersion
		}).Should(BeTrue(), "service %q not synced", name)
	}

	dataOf := func(obj *offloadingv1beta1.ShadowEndpointSlice) map[string][]string {
		raw, ok := obj.Annotations[consts.DirectConnectionDataAnnotationKey]
		if !ok {
			return nil
		}
		var d directconnection.ClusterAddresses
		Expect(d.FromJSON([]byte(raw))).To(Succeed())
		return d.Clusters
	}
	addressesOf := func(obj *offloadingv1beta1.ShadowEndpointSlice) []string {
		var out []string
		for i := range obj.Spec.Template.Endpoints {
			out = append(out, obj.Spec.Template.Endpoints[i].Addresses...)
		}
		return out
	}
	eventsSnapshot := func() []corev1.Event {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		return append([]corev1.Event(nil), events...)
	}

	BeforeEach(func() {
		liqoClient = liqoclientfake.NewSimpleClientset() //nolint:staticcheck // NewClientset is not generated for the Liqo clientset
		eventsMu.Lock()
		events = nil
		eventsMu.Unlock()
		// A second remote provider, besides ThirdClusterID: a virtual node of another cluster.
		_, err := client.CoreV1().Nodes().Get(ctx, rvFourthClusterNodeName, metav1.GetOptions{})
		if kerrors.IsNotFound(err) {
			_, err = client.CoreV1().Nodes().Create(ctx, FakeNodeWithNameAndLabels(rvFourthClusterNodeName, map[string]string{
				consts.TypeLabel: consts.TypeNode, consts.RemoteClusterID: rvFourthClusterID,
			}), metav1.CreateOptions{})
		}
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		Expect(client.DiscoveryV1().EndpointSlices(LocalNamespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(Succeed())
		for _, s := range []string{rvSvc, "rv-a", "rv-b"} {
			Expect(client.CoreV1().Services(LocalNamespace).Delete(ctx, s, metav1.DeleteOptions{})).To(
				Or(Succeed(), WithTransform(kerrors.IsNotFound, BeTrue())))
		}
	})

	Context("a Service with backends on two other providers plus a consumer-hosted endpoint", func() {
		BeforeEach(func() {
			createServiceNamed(rvSvc, true)
			createLocal(
				ep(thirdNode, "10.10.0.5", "pod-a"),
				ep(thirdNode, "10.10.0.6", "pod-b"),
				ep(fourthNode, "10.20.0.7", "pod-c"),
				ep(localNode, "192.168.0.30", "pod-local"), // consumer-hosted, inside the consumer pod CIDR
				ep(targetNode, "10.30.0.9", "pod-target"),  // natively present on the target provider
			)
			createIP("ip-a", "10.10.0.5", "10.70.0.5")
			createIP("ip-b", "10.10.0.6", "10.70.0.6")
			createIP("ip-c", "10.20.0.7", "10.70.0.7")
			startReflector()
			Expect(handle(rvEPS)).To(Succeed())
		})

		It("direct slice: untranslated data per cluster; companion: exactly its own translated addresses", func() {
			direct, companion := mustRemote(rvEPS), mustRemote(rvCompanion)

			Expect(dataOf(direct)).To(Equal(map[string][]string{
				ThirdClusterID: {"10.10.0.5", "10.10.0.6"}, rvFourthClusterID: {"10.20.0.7"}}))
			Expect(dataOf(companion)).To(Equal(map[string][]string{
				ThirdClusterID: {"10.70.0.5", "10.70.0.6"}, rvFourthClusterID: {"10.70.0.7"}}))

			Expect(addressesOf(direct)).To(ConsistOf("10.10.0.5", "10.10.0.6", "10.20.0.7", "192.168.0.30"))
			Expect(addressesOf(companion)).To(ConsistOf("10.70.0.5", "10.70.0.6", "10.70.0.7"))
		})

		It("every companion address is attributed to the right cluster, and the #3329 field index agrees", func() {
			direct, companion := mustRemote(rvEPS), mustRemote(rvCompanion)
			var cd directconnection.ClusterAddresses
			cd.Clusters = dataOf(companion)
			idx := cd.BuildIndex()
			want := map[string]string{"10.70.0.5": ThirdClusterID, "10.70.0.6": ThirdClusterID, "10.70.0.7": rvFourthClusterID}
			for _, a := range addressesOf(companion) {
				c, found := idx.LookupClusterID(a)
				Expect(found).To(BeTrue(), a)
				Expect(c).To(Equal(want[a]), a)
			}
			Expect(indexer.ExtractDirectConnectionClusterIDs(direct)).To(ConsistOf(ThirdClusterID, rvFourthClusterID))
			Expect(indexer.ExtractDirectConnectionClusterIDs(companion)).To(ConsistOf(indexer.ExtractDirectConnectionClusterIDs(direct)))
		})
	})

	Context("the IPAM has not translated one cross-provider address yet", func() {
		BeforeEach(func() {
			createServiceNamed(rvSvc, true)
			createIP("ip-a", "10.10.0.5", "10.70.0.5")
		})

		It("reflects the direct slice, fails only the companion (Warning event), and converges on a later Handle", func() {
			createLocal(ep(thirdNode, "10.10.0.5", "pod-a"), ep(fourthNode, "10.20.0.7", "pod-c"), ep(localNode, "192.168.0.30", "pod-local"))
			startReflector()

			err := handle(rvEPS)
			Expect(err).To(MatchError(ContainSubstring("10.20.0.7")))
			direct := mustRemote(rvEPS)
			Expect(addressesOf(direct)).To(ConsistOf("10.10.0.5", "10.20.0.7", "192.168.0.30"))
			Expect(dataOf(direct)).To(HaveKeyWithValue(rvFourthClusterID, ConsistOf("10.20.0.7")))
			_, err = getRemote(rvCompanion)
			Expect(err).To(BeNotFound())
			Eventually(eventsSnapshot).Should(ContainElement(SatisfyAll(
				HaveField("Type", corev1.EventTypeWarning),
				HaveField("Reason", forge.EventFailedReflection),
				HaveField("Message", ContainSubstring("10.20.0.7")))))

			// Nothing watches IP resources: only the local EPS, remote shadow and Service handlers exist,
			// so convergence relies on the rate-limited requeue of the failed key.
			Expect(keyers).To(HaveLen(3))

			createIP("ip-c", "10.20.0.7", "10.70.0.7")
			Eventually(func() (string, error) { return ner.MapEndpointIPFromIPResource("10.20.0.7") }).Should(Equal("10.70.0.7"))
			waitRemoteSynced(rvEPS, rvCompanion)
			Expect(handle(rvEPS)).To(Succeed())
			companion := mustRemote(rvCompanion)
			Expect(addressesOf(companion)).To(ConsistOf("10.70.0.5", "10.70.0.7"))
			Expect(dataOf(companion)).To(Equal(map[string][]string{ThirdClusterID: {"10.70.0.5"}, rvFourthClusterID: {"10.70.0.7"}}))
		})

		It("keeps the existing companion, and reports the error, until the IPAM converges", func() {
			createLocal(ep(thirdNode, "10.10.0.5", "pod-a"), ep(localNode, "192.168.0.30", "pod-local"))
			startReflector()
			Expect(handle(rvEPS)).To(Succeed())

			local, err := client.DiscoveryV1().EndpointSlices(LocalNamespace).Get(ctx, rvEPS, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			local.Endpoints = append(local.Endpoints, ep(fourthNode, "10.20.0.7", "pod-c"))
			_, err = client.DiscoveryV1().EndpointSlices(LocalNamespace).Update(ctx, local, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
			waitLocalSynced(rvEPS)
			waitRemoteSynced(rvEPS, rvCompanion)

			Expect(handle(rvEPS)).To(MatchError(ContainSubstring("10.20.0.7")))
			Expect(addressesOf(mustRemote(rvEPS))).To(ContainElement("10.20.0.7"))
			companion := mustRemote(rvCompanion)
			Expect(addressesOf(companion)).To(ConsistOf("10.70.0.5"))
			Expect(dataOf(companion)).To(Equal(map[string][]string{ThirdClusterID: {"10.70.0.5"}}))
		})
	})

	Context("the per-EndpointSlice translations cache", func() {
		BeforeEach(func() {
			createServiceNamed(rvSvc, true)
			createLocal(ep(thirdNode, "10.10.0.5", "pod-a"))
			createIP("ip-a", "10.10.0.5", "10.70.0.5")
			startReflector()
			Expect(handle(rvEPS)).To(Succeed())
			Expect(addressesOf(mustRemote(rvCompanion))).To(ConsistOf("10.70.0.5"))
		})

		It("is forgotten with the slice, so that a re-created slice observes the current IPAM mapping", func() {
			Expect(client.DiscoveryV1().EndpointSlices(LocalNamespace).Delete(ctx, rvEPS, metav1.DeleteOptions{})).To(Succeed())
			waitLocalSynced(rvEPS)
			Expect(handle(rvEPS)).To(Succeed())
			_, err := getRemote(rvEPS)
			Expect(err).To(BeNotFound())
			_, err = getRemote(rvCompanion)
			Expect(err).To(BeNotFound())

			// The mapping changes meanwhile (e.g. the IP resource is re-created for a new pod reusing the address).
			ip, err := liqoClient.IpamV1alpha1().IPs(LocalNamespace).Get(ctx, "ip-a", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			ip.Status.IP = "10.70.0.99"
			_, err = liqoClient.IpamV1alpha1().IPs(LocalNamespace).UpdateStatus(ctx, ip, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() (string, error) { return ner.MapEndpointIPFromIPResource("10.10.0.5") }).Should(Equal("10.70.0.99"))

			createLocal(ep(thirdNode, "10.10.0.5", "pod-a"))
			waitLocalSynced(rvEPS)
			waitRemoteSynced(rvEPS, rvCompanion)
			Expect(handle(rvEPS)).To(Succeed())
			companion := mustRemote(rvCompanion)
			Expect(addressesOf(companion)).To(ConsistOf("10.70.0.99"))
			Expect(dataOf(companion)).To(Equal(map[string][]string{ThirdClusterID: {"10.70.0.99"}}))
		})
	})

	Context("the annotations of the slice are close to the size limit", func() {
		BeforeEach(func() {
			createServiceNamed(rvSvc, true)
			var endpoints []discoveryv1.Endpoint
			var originals []string
			for i := 1; i <= 20; i++ {
				orig := fmt.Sprintf("10.1.0.%d", i)
				originals = append(originals, orig)
				endpoints = append(endpoints, ep(thirdNode, orig, fmt.Sprintf("pod-%d", i)))
				createIP(fmt.Sprintf("ip-%d", i), orig, fmt.Sprintf("100.100.100.%d", 100+i)) // longer translated form
			}
			directJSON, err := (&directconnection.ClusterAddresses{Clusters: map[string][]string{ThirdClusterID: originals}}).ToJSON()
			Expect(err).ToNot(HaveOccurred())
			// Largest padding the reflector accepts for the direct slice (its check is ">= 256KiB").
			padding := apivalidation.TotalAnnotationSizeLimitB - len(consts.DirectConnectionDataAnnotationKey) - len(directJSON) - len("padding") - 1
			createLocalFull(rvEPS, rvSvc, map[string]string{"padding": strings.Repeat("x", padding)}, endpoints...)
			startReflector()
		})

		It("refuses the reflection when the companion's data exceeds the limit, even if the direct slice's would not", func() {
			// The translated addresses of the companion are longer than the original ones of the direct slice.
			Expect(handle(rvEPS)).To(MatchError(ContainSubstring("annotations exceed maximum size")))
			_, err := getRemote(rvCompanion)
			Expect(err).To(BeNotFound())
		})
	})

	Context("endpoints the reflector cannot attribute to a provider", func() {
		BeforeEach(func() {
			createLocal(
				ep(localNode, "172.18.0.3", "pod-hostnet"), // consumer-hosted, hostNetwork: outside the consumer pod CIDR
				ep(nil, "203.0.113.10", ""),                // external endpoint (selectorless Service)
				ep(thirdNode, "10.10.0.9", ""),             // on another provider, but without targetRef
				ep(thirdNode, "10.10.0.5", "pod-a"),        // control: attributable cross-provider endpoint
			)
			// A converged consumer: IP resources exist for every address (user-created for the
			// external/hostNetwork ones, as documented in external-ip-remapping.md).
			createIP("ip-hostnet", "172.18.0.3", "10.70.0.33")
			createIP("ip-external", "203.0.113.10", "10.70.0.44")
			createIP("ip-notarget", "10.10.0.9", "10.70.0.9")
			createIP("ip-a", "10.10.0.5", "10.70.0.5")
		})

		It("without the annotation: every address is IPAM-translated onto the consumer path", func() {
			createServiceNamed(rvSvc, false)
			startReflector()
			Expect(handle(rvEPS)).To(Succeed())
			plain := mustRemote(rvEPS)
			Expect(addressesOf(plain)).To(ConsistOf("10.70.0.33", "10.70.0.44", "10.70.0.9", "10.70.0.5"))
			_, err := getRemote(rvCompanion)
			Expect(err).To(BeNotFound())
		})

		It("with the annotation: they are translated exactly the same, only the attributed endpoint travels untranslated", func() {
			createServiceNamed(rvSvc, true)
			startReflector()
			Expect(handle(rvEPS)).To(Succeed())
			direct := mustRemote(rvEPS)
			Expect(addressesOf(direct)).To(ConsistOf("10.70.0.33", "10.70.0.44", "10.70.0.9", "10.10.0.5"))
			Expect(dataOf(direct)).To(Equal(map[string][]string{ThirdClusterID: {"10.10.0.5"}}))
			Expect(addressesOf(mustRemote(rvCompanion))).To(ConsistOf("10.70.0.5"))
		})
	})

	Context("the companion lifecycle", func() {
		BeforeEach(func() {
			createServiceNamed(rvSvc, true)
			createLocal(ep(thirdNode, "10.10.0.5", "pod-a"), ep(localNode, "192.168.0.30", "pod-local"))
			createIP("ip-a", "10.10.0.5", "10.70.0.5")
			startReflector()
			Expect(handle(rvEPS)).To(Succeed())
			mustRemote(rvCompanion)
			waitRemoteSynced(rvEPS, rvCompanion)
		})

		It("is deleted when the annotation is removed from the Service (direct slice reverts to a plain one)", func() {
			svc, err := client.CoreV1().Services(LocalNamespace).Get(ctx, rvSvc, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			delete(svc.Annotations, consts.UseDirectConnectionAnnotationKey)
			_, err = client.CoreV1().Services(LocalNamespace).Update(ctx, svc, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
			waitServiceSynced(rvSvc)

			Expect(handle(rvEPS)).To(Succeed())
			_, err = getRemote(rvCompanion)
			Expect(err).To(BeNotFound())
			direct := mustRemote(rvEPS)
			Expect(direct.Annotations).ToNot(HaveKey(consts.DirectConnectionDataAnnotationKey))
			Expect(addressesOf(direct)).To(ConsistOf("10.70.0.5", "192.168.0.30"))
		})

		It("is deleted when no cross-provider endpoint is left", func() {
			local, err := client.DiscoveryV1().EndpointSlices(LocalNamespace).Get(ctx, rvEPS, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			local.Endpoints = local.Endpoints[1:]
			_, err = client.DiscoveryV1().EndpointSlices(LocalNamespace).Update(ctx, local, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
			waitLocalSynced(rvEPS)

			Expect(handle(rvEPS)).To(Succeed())
			_, err = getRemote(rvCompanion)
			Expect(err).To(BeNotFound())
			Expect(mustRemote(rvEPS).Annotations).ToNot(HaveKey(consts.DirectConnectionDataAnnotationKey))
		})

		It("is deleted together with the direct slice when the local slice goes away", func() {
			Expect(client.DiscoveryV1().EndpointSlices(LocalNamespace).Delete(ctx, rvEPS, metav1.DeleteOptions{})).To(Succeed())
			waitLocalSynced(rvEPS)
			Expect(handle(rvEPS)).To(Succeed())
			_, err := getRemote(rvEPS)
			Expect(err).To(BeNotFound())
			_, err = getRemote(rvCompanion)
			Expect(err).To(BeNotFound())
		})

		It("companion events are keyed to the parent, and List() omits companions", func() {
			Expect(keyers).To(HaveLen(3))
			remoteKeyer := keyers[1]
			Expect(remoteKeyer(mustRemote(rvCompanion))).To(ConsistOf(types.NamespacedName{Namespace: LocalNamespace, Name: rvEPS}))
			Expect(remoteKeyer(mustRemote(rvEPS))).To(ConsistOf(types.NamespacedName{Namespace: LocalNamespace, Name: rvEPS}))

			items, err := ner.List()
			Expect(err).ToNot(HaveOccurred())
			Expect(items).To(ContainElement(types.NamespacedName{Namespace: RemoteNamespace, Name: rvEPS}))
			Expect(items).ToNot(ContainElement(types.NamespacedName{Namespace: RemoteNamespace, Name: rvCompanion}))
		})
	})

	// The companion name is derived from the parent name only, so a local EndpointSlice that is itself
	// named "<other>-indirect" shares its name: its reflection must never be taken for the companion.
	Context("a local EndpointSlice named after another one plus the companion suffix", func() {
		BeforeEach(func() {
			createServiceNamed("rv-b", false)
			createLocalFull("rv-x-indirect", "rv-b", nil, ep(localNode, "192.168.0.32", "pod-y"))
		})

		When("the other slice has no direct connections", func() {
			BeforeEach(func() {
				createServiceNamed("rv-a", false)
				createLocalFull("rv-x", "rv-a", nil, ep(localNode, "192.168.0.31", "pod-x"))
				startReflector()
				Expect(handle("rv-x-indirect")).To(Succeed())
				waitRemoteSynced("rv-x", "rv-x-indirect")
			})

			It("reconciling the other slice does not delete its reflection", func() {
				Expect(handle("rv-x")).To(Succeed())
				Expect(addressesOf(mustRemote("rv-x-indirect"))).To(ConsistOf("192.168.0.32"))
			})
		})

		When("the other slice has direct connections", func() {
			BeforeEach(func() {
				createServiceNamed("rv-a", true)
				createLocalFull("rv-x", "rv-a", nil, ep(thirdNode, "10.10.0.5", "pod-x"))
				createIP("ip-a", "10.10.0.5", "10.70.0.5")
				startReflector()
				Expect(handle("rv-x-indirect")).To(Succeed())
				waitRemoteSynced("rv-x-indirect")
			})

			It("reconciling the other slice does not overwrite its reflection with a companion", func() {
				Expect(handle("rv-x")).To(Succeed())
				reflection := mustRemote("rv-x-indirect")
				Expect(addressesOf(reflection)).To(ConsistOf("192.168.0.32"))
				Expect(reflection.Labels).ToNot(HaveKey(forge.IndirectEndpointSliceLabelKey))
			})
		})
	})

	Context("the companion cannot be written", func() {
		BeforeEach(func() {
			createServiceNamed(rvSvc, true)
			createLocal(ep(thirdNode, "10.10.0.5", "pod-a"))
			createIP("ip-a", "10.10.0.5", "10.70.0.5")
			liqoClient.PrependReactor("create", "shadowendpointslices", func(action k8stesting.Action) (bool, runtime.Object, error) {
				if action.(k8stesting.CreateAction).GetObject().(metav1.Object).GetName() == rvCompanion {
					return true, nil, errors.New("companion write refused")
				}
				return false, nil, nil
			})
			startReflector()
		})

		It("reports the failure with a Warning event, as for the direct slice", func() {
			Expect(handle(rvEPS)).To(MatchError(ContainSubstring("companion write refused")))
			Eventually(eventsSnapshot).Should(ContainElement(SatisfyAll(
				HaveField("Type", corev1.EventTypeWarning),
				HaveField("Reason", forge.EventFailedReflection),
				HaveField("Message", ContainSubstring("companion write refused")))))
		})
	})
})
