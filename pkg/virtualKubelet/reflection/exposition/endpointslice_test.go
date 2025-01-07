// Copyright 2019-2025 The Liqo Authors
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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"k8s.io/utils/trace"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/cmd/virtual-kubelet/root"
	liqoclient "github.com/liqotech/liqo/pkg/client/clientset/versioned"
	liqoclientfake "github.com/liqotech/liqo/pkg/client/clientset/versioned/fake"
	liqoinformers "github.com/liqotech/liqo/pkg/client/informers/externalversions"
	"github.com/liqotech/liqo/pkg/consts"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/exposition"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/resources"
)

const (
	localPodCIDR string = "192.168.0.0/16"
)

var _ = Describe("EndpointSlice Reflection Tests", func() {
	Describe("the NewEndpointSliceReflector function", func() {
		It("should not return a nil reflector", func() {
			reflectorConfig := offloadingv1beta1.ReflectorConfig{
				NumWorkers: 1,
				Type:       root.DefaultReflectorsTypes[resources.EndpointSlice],
			}
			Expect(exposition.NewEndpointSliceReflector(localPodCIDR, &reflectorConfig)).ToNot(BeNil())
		})
	})

	Describe("endpointslice handling", func() {
		const EndpointSliceName = "name"
		const ServiceName = "service"

		var (
			err            error
			reflector      manager.NamespacedReflector
			reflectionType offloadingv1beta1.ReflectionType
			liqoClient     liqoclient.Interface

			local  discoveryv1.EndpointSlice
			remote offloadingv1beta1.ShadowEndpointSlice
		)

		CreateEndpointSlice := func(epslice *discoveryv1.EndpointSlice) *discoveryv1.EndpointSlice {
			epslice, errepslice := client.DiscoveryV1().EndpointSlices(epslice.GetNamespace()).Create(ctx, epslice, metav1.CreateOptions{})
			Expect(errepslice).ToNot(HaveOccurred())
			return epslice
		}

		GetShadowEndpointSlice := func(namespace string) *offloadingv1beta1.ShadowEndpointSlice {
			epslice, errepslice := liqoClient.OffloadingV1beta1().ShadowEndpointSlices(namespace).Get(ctx, EndpointSliceName, metav1.GetOptions{})
			Expect(errepslice).ToNot(HaveOccurred())
			return epslice
		}

		CreateShadowEndpointSlice := func(epslice *offloadingv1beta1.ShadowEndpointSlice) *offloadingv1beta1.ShadowEndpointSlice {
			epslice, errepslice := liqoClient.OffloadingV1beta1().ShadowEndpointSlices(epslice.GetNamespace()).
				Create(ctx, epslice, metav1.CreateOptions{})
			Expect(errepslice).ToNot(HaveOccurred())
			return epslice
		}

		CreateService := func(svc *corev1.Service) *corev1.Service {
			svc, errsvc := client.CoreV1().Services(svc.GetNamespace()).Create(ctx, svc, metav1.CreateOptions{})
			Expect(errsvc).ToNot(HaveOccurred())
			return svc
		}

		CreateIP := func(name, namespace, ip, remappedIP string) *ipamv1alpha1.IP {
			ipamIP := &ipamv1alpha1.IP{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: ipamv1alpha1.IPSpec{IP: networkingv1beta1.IP(ip)}}
			ipamIP, err = liqoClient.IpamV1alpha1().IPs(namespace).Create(ctx, ipamIP, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
			ipamIP.Status = ipamv1alpha1.IPStatus{
				IP: networkingv1beta1.IP(remappedIP),
			}
			ipamIP, err = liqoClient.IpamV1alpha1().IPs(namespace).UpdateStatus(ctx, ipamIP, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
			return ipamIP
		}

		WhenBodyRemoteShouldNotExist := func(createRemote bool) func() {
			return func() {
				BeforeEach(func() {
					if createRemote {
						remote.SetLabels(labels.Merge(forge.ReflectionLabels(), forge.EndpointSliceLabels()))
						remote.Spec.Template.AddressType = discoveryv1.AddressTypeIPv4
						CreateShadowEndpointSlice(&remote)
					}
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the remote object should not be present", func() {
					_, err = liqoClient.OffloadingV1beta1().ShadowEndpointSlices(RemoteNamespace).Get(ctx, EndpointSliceName, metav1.GetOptions{})
					Expect(err).To(BeNotFound())
				})
			}
		}

		BeforeEach(func() {
			liqoClient = liqoclientfake.NewSimpleClientset()
			local = discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Name: EndpointSliceName, Namespace: LocalNamespace}}
			remote = offloadingv1beta1.ShadowEndpointSlice{ObjectMeta: metav1.ObjectMeta{Name: EndpointSliceName, Namespace: RemoteNamespace}}
			reflectionType = root.DefaultReflectorsTypes[resources.Service] // reflection type inherited from the service reflector
		})

		AfterEach(func() {
			Expect(client.DiscoveryV1().EndpointSlices(LocalNamespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
			Expect(liqoClient.OffloadingV1beta1().ShadowEndpointSlices(RemoteNamespace).
				DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
			Expect(client.CoreV1().Services(LocalNamespace).Delete(ctx, ServiceName, metav1.DeleteOptions{})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
		})

		JustBeforeEach(func() {
			factory := informers.NewSharedInformerFactory(client, 10*time.Hour)
			liqoFactory := liqoinformers.NewSharedInformerFactory(liqoClient, 10*time.Hour)
			reflector = exposition.NewNamespacedEndpointSliceReflector(localPodCIDR)(options.NewNamespaced().
				WithLocal(LocalNamespace, client, factory).
				WithLiqoLocal(liqoClient, liqoFactory).
				WithRemote(RemoteNamespace, client, factory).
				WithLiqoRemote(liqoClient, liqoFactory).
				WithHandlerFactory(FakeEventHandler).
				WithEventBroadcaster(record.NewBroadcaster()).
				WithReflectionType(reflectionType).
				WithForgingOpts(FakeForgingOpts()))

			factory.Start(ctx.Done())
			liqoFactory.Start(ctx.Done())
			factory.WaitForCacheSync(ctx.Done())
			liqoFactory.WaitForCacheSync(ctx.Done())
		})

		Context("object reflection", func() {
			JustBeforeEach(func() {
				err = reflector.Handle(trace.ContextWithTrace(ctx, trace.New("EndpointSlice")), EndpointSliceName)
			})

			When("the local object does not exist", func() {
				When("the remote object does not exist", WhenBodyRemoteShouldNotExist(false))
				When("the remote object does exist", WhenBodyRemoteShouldNotExist(true))
			})

			When("the local object does exist", func() {
				BeforeEach(func() {
					local.SetLabels(map[string]string{"foo": "bar", FakeNotReflectedLabelKey: "true"})
					local.SetAnnotations(map[string]string{"bar": "baz", FakeNotReflectedAnnotKey: "true"})
					local.AddressType = discoveryv1.AddressTypeIPv4
					local.Endpoints = []discoveryv1.Endpoint{{
						NodeName:  ptr.To(LocalClusterNodeName),
						Addresses: []string{"10.168.0.25", "10.168.0.43"},
					}}
					CreateEndpointSlice(&local)
					CreateIP("ip1", LocalNamespace, "10.168.0.25", "192.168.200.25")
					CreateIP("ip2", LocalNamespace, "10.168.0.43", "192.168.200.43")
				})

				When("the remote object does not exist", func() {
					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("the metadata should have been correctly replicated to the remote object", func() {
						remoteAfter := GetShadowEndpointSlice(RemoteNamespace)
						Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
						Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
						Expect(remoteAfter.Labels).To(HaveKeyWithValue(discoveryv1.LabelManagedBy, forge.EndpointSliceManagedBy))
						Expect(remoteAfter.Labels).To(HaveKeyWithValue("foo", "bar"))
						Expect(remoteAfter.Labels).ToNot(HaveKey(FakeNotReflectedLabelKey))
						Expect(remoteAfter.Annotations).To(HaveKeyWithValue("bar", "baz"))
						Expect(remoteAfter.Annotations).ToNot(HaveKey(FakeNotReflectedAnnotKey))
					})
					It("the spec should have been correctly replicated to the remote object", func() {
						remoteAfter := GetShadowEndpointSlice(RemoteNamespace)
						// Here, we assert only a few fields, as already tested in the forge package.
						Expect(remoteAfter.Spec.Template.AddressType).To(Equal(discoveryv1.AddressTypeIPv4))
						Expect(remoteAfter.Spec.Template.Endpoints).To(HaveLen(1))
						Expect(remoteAfter.Spec.Template.Endpoints[0].Addresses).To(ConsistOf("192.168.200.25", "192.168.200.43"))
					})
				})

				When("the remote object already exists", func() {
					BeforeEach(func() {
						remote.SetLabels(labels.Merge(forge.ReflectionLabels(), forge.EndpointSliceLabels()))
						remote.SetLabels(labels.Merge(remote.GetLabels(), map[string]string{"foo": "previous", "existing": "existing"}))
						remote.SetLabels(labels.Merge(remote.GetLabels(), map[string]string{FakeNotReflectedLabelKey: "true"}))
						remote.SetAnnotations(map[string]string{"bar": "previous", "existing": "existing", FakeNotReflectedAnnotKey: "true"})
						remote.Spec.Template.AddressType = discoveryv1.AddressTypeIPv4
						CreateShadowEndpointSlice(&remote)
					})

					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("the metadata should have been correctly replicated to the remote object", func() {
						remoteAfter := GetShadowEndpointSlice(RemoteNamespace)
						Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
						Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
						Expect(remoteAfter.Labels).To(HaveKeyWithValue(discoveryv1.LabelManagedBy, forge.EndpointSliceManagedBy))
						Expect(remoteAfter.Labels).To(HaveKeyWithValue("foo", "bar"))
						Expect(remoteAfter.Labels).ToNot(HaveKey("existing"))
						Expect(remoteAfter.Labels).ToNot(HaveKey(FakeNotReflectedLabelKey))
						Expect(remoteAfter.Annotations).To(HaveKeyWithValue("bar", "baz"))
						Expect(remoteAfter.Annotations).ToNot(HaveKey("existing"))
						Expect(remoteAfter.Annotations).ToNot(HaveKey(FakeNotReflectedAnnotKey))
					})
					It("the spec should have been correctly replicated to the remote object", func() {
						remoteAfter := GetShadowEndpointSlice(RemoteNamespace)
						// Here, we assert only a few fields, as already tested in the forge package.
						Expect(remoteAfter.Spec.Template.AddressType).To(Equal(discoveryv1.AddressTypeIPv4))
						Expect(remoteAfter.Spec.Template.Endpoints).To(HaveLen(1))
						Expect(remoteAfter.Spec.Template.Endpoints[0].Addresses).To(ConsistOf("192.168.200.25", "192.168.200.43"))
					})
				})

				When("the remote object already exists, but is not managed by the reflection", func() {
					var remoteBefore *offloadingv1beta1.ShadowEndpointSlice

					BeforeEach(func() {
						remote.Spec.Template.AddressType = discoveryv1.AddressTypeIPv4
						remoteBefore = CreateShadowEndpointSlice(&remote)
					})

					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("the remote object should be unmodified", func() {
						remoteAfter := GetShadowEndpointSlice(RemoteNamespace)
						Expect(remoteAfter).To(Equal(remoteBefore))
					})
				})
			})

			When("the local object does exist, but has the skip annotation", func() {
				BeforeEach(func() {
					local.SetAnnotations(map[string]string{consts.SkipReflectionAnnotationKey: "whatever"})
					local.AddressType = discoveryv1.AddressTypeIPv4
					CreateEndpointSlice(&local)
				})

				When("the remote object does not exist", WhenBodyRemoteShouldNotExist(false))
				When("the remote object does exist", WhenBodyRemoteShouldNotExist(true))
			})

			When("the local object does exist, but the associated service has the skip annotation", func() {
				BeforeEach(func() {
					local.Labels = map[string]string{discoveryv1.LabelServiceName: ServiceName}
					local.AddressType = discoveryv1.AddressTypeIPv4
					CreateEndpointSlice(&local)

					CreateService(&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name: ServiceName, Namespace: LocalNamespace,
							Annotations: map[string]string{consts.SkipReflectionAnnotationKey: "whatever"},
						},
						Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)}}},
					})
				})

				When("the remote object does not exist", WhenBodyRemoteShouldNotExist(false))
				When("the remote object does exist", WhenBodyRemoteShouldNotExist(true))
			})

			When("the reflection type is AllowList", func() {
				BeforeEach(func() {
					reflectionType = offloadingv1beta1.AllowList
				})

				When("the local object does exist, and the associated service has the allow annotation", func() {
					BeforeEach(func() {
						local.Labels = map[string]string{discoveryv1.LabelServiceName: ServiceName}
						local.AddressType = discoveryv1.AddressTypeIPv4
						CreateEndpointSlice(&local)

						CreateService(&corev1.Service{
							ObjectMeta: metav1.ObjectMeta{
								Name: ServiceName, Namespace: LocalNamespace,
								Annotations: map[string]string{consts.AllowReflectionAnnotationKey: "whatever"},
							},
							Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)}}},
						})
					})

					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("the remote object should be present", func() {
						remoteAfter := GetShadowEndpointSlice(RemoteNamespace)
						Expect(remoteAfter).ToNot(BeNil())
					})
				})

				When("the local object does exist, but does not have the allow annotation", func() {
					BeforeEach(func() {
						local.AddressType = discoveryv1.AddressTypeIPv4
						CreateEndpointSlice(&local)
					})

					When("the remote object does not exist", WhenBodyRemoteShouldNotExist(false))
					When("the remote object does exist", WhenBodyRemoteShouldNotExist(true))
				})

				When("the local object does exist, and does have the allow annotation", func() {
					BeforeEach(func() {
						local.SetAnnotations(map[string]string{consts.AllowReflectionAnnotationKey: "whatever"})
						local.AddressType = discoveryv1.AddressTypeIPv4
						CreateEndpointSlice(&local)
					})

					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("the remote object should be present", func() {
						remoteAfter := GetShadowEndpointSlice(RemoteNamespace)
						Expect(remoteAfter).ToNot(BeNil())
					})
				})
			})

			When("the reflection is forced with the allow or skip annotation", func() {
				When("the reflection is deny, but the object has the allow annotation", func() {
					BeforeEach(func() {
						reflectionType = offloadingv1beta1.DenyList
						local.SetAnnotations(map[string]string{consts.AllowReflectionAnnotationKey: "whatever"})
						local.AddressType = discoveryv1.AddressTypeIPv4
						CreateEndpointSlice(&local)
					})

					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("the remote object should be present", func() {
						remoteAfter := GetShadowEndpointSlice(RemoteNamespace)
						Expect(remoteAfter).ToNot(BeNil())
					})
				})

				When("the reflection is allow, but the object has the skip annotation", func() {
					BeforeEach(func() {
						reflectionType = offloadingv1beta1.AllowList
						local.SetAnnotations(map[string]string{consts.SkipReflectionAnnotationKey: "whatever"})
						local.AddressType = discoveryv1.AddressTypeIPv4
						CreateEndpointSlice(&local)
					})

					When("the remote object does not exist", WhenBodyRemoteShouldNotExist(false))
					When("the remote object does exist", WhenBodyRemoteShouldNotExist(true))
				})
			})
		})

		Context("address translation", func() {
			var (
				input, output []string
				err           error
			)

			BeforeEach(func() {
				input = []string{"10.168.0.25", "10.168.0.43"}

				CreateIP("ip1", LocalNamespace, "10.168.0.25", "192.168.200.25")
				CreateIP("ip2", LocalNamespace, "10.168.0.43", "192.168.200.43")
			})

			When("translating a set of IP addresses", func() {
				JustBeforeEach(func() {
					output, err = reflector.(*exposition.NamespacedEndpointSliceReflector).
						MapEndpointIPs(EndpointSliceName, input)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should return the correct translations", func() { Expect(output).To(ConsistOf("192.168.200.25", "192.168.200.43")) })

				When("translating again the same set of IP addresses", func() {
					JustBeforeEach(func() {
						output, err = reflector.(*exposition.NamespacedEndpointSliceReflector).
							MapEndpointIPs(EndpointSliceName, input)
					})

					// The IPAMClient is configured to return an error if the same translation is requested twice.
					It("should succeed (i.e., use the cached values)", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should return the same translations", func() { Expect(output).To(ConsistOf("192.168.200.25", "192.168.200.43")) })
				})
			})
		})

		Context("retrieving the remote endpointslices associated with a service", func() {
			var (
				service corev1.Service
				keys    []types.NamespacedName
			)

			BeforeEach(func() {
				service = corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: ServiceName, Namespace: LocalNamespace},
					Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)}}},
				}
				CreateService(&service)

				creator := func(suffix, service string) {
					CreateEndpointSlice(&discoveryv1.EndpointSlice{
						ObjectMeta: metav1.ObjectMeta{
							Name: EndpointSliceName + "-" + suffix, Namespace: LocalNamespace,
							Labels: map[string]string{discoveryv1.LabelServiceName: service},
						},
						AddressType: discoveryv1.AddressTypeIPv4,
					})
				}

				creator("first", ServiceName)
				creator("second", "other")
				creator("third", ServiceName)
			})

			JustBeforeEach(func() {
				keys = reflector.(*exposition.NamespacedEndpointSliceReflector).ServiceToEndpointSlicesKeyer(&service)
			})

			It("should return the expected keys", func() {
				Expect(keys).To(ConsistOf(
					types.NamespacedName{Namespace: LocalNamespace, Name: EndpointSliceName + "-first"},
					types.NamespacedName{Namespace: LocalNamespace, Name: EndpointSliceName + "-third"},
				))
			})
		})
	})
})
