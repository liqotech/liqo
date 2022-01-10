// Copyright 2019-2022 The Liqo Authors
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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/utils/trace"

	fakeipam "github.com/liqotech/liqo/pkg/liqonet/ipam/fake"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/exposition"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ = Describe("EndpointSlice Reflection Tests", func() {
	Describe("the NewEndpointSliceReflector function", func() {
		It("should not return a nil reflector", func() {
			Expect(exposition.NewEndpointSliceReflector(nil, 1)).ToNot(BeNil())
		})
	})

	Describe("endpointslice handling", func() {
		const EndpointSliceName = "name"

		var (
			reflector manager.NamespacedReflector
			ipam      *fakeipam.IPAMClient

			local, remote discoveryv1beta1.EndpointSlice
			err           error
		)

		GetEndpointSlice := func(namespace string) *discoveryv1beta1.EndpointSlice {
			epslice, errepslice := client.DiscoveryV1beta1().EndpointSlices(namespace).Get(ctx, EndpointSliceName, metav1.GetOptions{})
			Expect(errepslice).ToNot(HaveOccurred())
			return epslice
		}

		CreateEndpointSlice := func(epslice *discoveryv1beta1.EndpointSlice) *discoveryv1beta1.EndpointSlice {
			epslice, errepslice := client.DiscoveryV1beta1().EndpointSlices(epslice.GetNamespace()).Create(ctx, epslice, metav1.CreateOptions{})
			Expect(errepslice).ToNot(HaveOccurred())
			return epslice
		}

		BeforeEach(func() {
			local = discoveryv1beta1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Name: EndpointSliceName, Namespace: LocalNamespace}}
			remote = discoveryv1beta1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Name: EndpointSliceName, Namespace: RemoteNamespace}}
		})

		AfterEach(func() {
			Expect(client.DiscoveryV1beta1().EndpointSlices(LocalNamespace).Delete(ctx, EndpointSliceName, metav1.DeleteOptions{})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
			Expect(client.DiscoveryV1beta1().EndpointSlices(RemoteNamespace).Delete(ctx, EndpointSliceName, metav1.DeleteOptions{})).To(
				Or(BeNil(), WithTransform(kerrors.IsNotFound, BeTrue())))
		})

		JustBeforeEach(func() {
			ipam = fakeipam.NewIPAMClient("192.168.200.0/24", "192.168.201.0/24", true)
			factory := informers.NewSharedInformerFactory(client, 10*time.Hour)
			reflector = exposition.NewNamespacedEndpointSliceReflector(ipam)(options.NewNamespaced().
				WithLocal(LocalNamespace, client, factory).
				WithRemote(RemoteNamespace, client, factory).
				WithHandlerFactory(FakeEventHandler))

			factory.Start(ctx.Done())
			factory.WaitForCacheSync(ctx.Done())
		})

		Context("object reflection", func() {
			JustBeforeEach(func() {
				err = reflector.Handle(trace.ContextWithTrace(ctx, trace.New("EndpointSlice")), EndpointSliceName)
			})

			When("the local object does not exist", func() {
				WhenBody := func(createRemote bool) func() {
					return func() {
						BeforeEach(func() {
							if createRemote {
								remote.SetLabels(labels.Merge(forge.ReflectionLabels(), forge.EndpointSliceLabels()))
								remote.AddressType = discoveryv1beta1.AddressTypeIPv4
								CreateEndpointSlice(&remote)
							}
						})

						It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
						It("the remote object should not be created", func() {
							_, err = client.DiscoveryV1beta1().EndpointSlices(RemoteNamespace).Get(ctx, EndpointSliceName, metav1.GetOptions{})
							Expect(err).To(BeNotFound())
						})
					}
				}

				When("the remote object does not exist", WhenBody(false))
				When("the remote object does exist", WhenBody(true))
			})

			When("the local object does exist", func() {
				BeforeEach(func() {
					local.SetLabels(map[string]string{"foo": "bar"})
					local.SetAnnotations(map[string]string{"bar": "baz"})
					local.AddressType = discoveryv1beta1.AddressTypeIPv4
					local.Endpoints = []discoveryv1beta1.Endpoint{{Addresses: []string{"192.168.0.25", "192.168.0.43"}}}
					CreateEndpointSlice(&local)
				})

				When("the remote object does not exist", func() {
					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("the metadata should have been correctly replicated to the remote object", func() {
						remoteAfter := GetEndpointSlice(RemoteNamespace)
						Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
						Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
						Expect(remoteAfter.Labels).To(HaveKeyWithValue(discoveryv1beta1.LabelManagedBy, forge.EndpointSliceManagedBy))
						Expect(remoteAfter.Labels).To(HaveKeyWithValue("foo", "bar"))
						Expect(remoteAfter.Annotations).To(HaveKeyWithValue("bar", "baz"))
					})
					It("the spec should have been correctly replicated to the remote object", func() {
						remoteAfter := GetEndpointSlice(RemoteNamespace)
						// Here, we assert only a few fields, as already tested in the forge package.
						Expect(remoteAfter.AddressType).To(Equal(discoveryv1beta1.AddressTypeIPv4))
						Expect(remoteAfter.Endpoints).To(HaveLen(1))
						Expect(remoteAfter.Endpoints[0].Addresses).To(ConsistOf("192.168.200.25", "192.168.200.43"))
					})
				})

				When("the remote object already exists", func() {
					BeforeEach(func() {
						remote.SetLabels(labels.Merge(forge.ReflectionLabels(), forge.EndpointSliceLabels()))
						remote.SetAnnotations(map[string]string{"bar": "previous", "existing": "existing"})
						remote.AddressType = discoveryv1beta1.AddressTypeIPv4
						CreateEndpointSlice(&remote)
					})

					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("the metadata should have been correctly replicated to the remote object", func() {
						remoteAfter := GetEndpointSlice(RemoteNamespace)
						Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
						Expect(remoteAfter.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
						Expect(remoteAfter.Labels).To(HaveKeyWithValue(discoveryv1beta1.LabelManagedBy, forge.EndpointSliceManagedBy))
						Expect(remoteAfter.Labels).To(HaveKeyWithValue("foo", "bar"))
						Expect(remoteAfter.Annotations).To(HaveKeyWithValue("bar", "baz"))
						Expect(remoteAfter.Annotations).To(HaveKeyWithValue("existing", "existing"))
					})
					It("the spec should have been correctly replicated to the remote object", func() {
						remoteAfter := GetEndpointSlice(RemoteNamespace)
						// Here, we assert only a few fields, as already tested in the forge package.
						Expect(remoteAfter.AddressType).To(Equal(discoveryv1beta1.AddressTypeIPv4))
						Expect(remoteAfter.Endpoints).To(HaveLen(1))
						Expect(remoteAfter.Endpoints[0].Addresses).To(ConsistOf("192.168.200.25", "192.168.200.43"))
					})
				})

				When("the remote object already exists, but is not managed by the reflection", func() {
					var remoteBefore *discoveryv1beta1.EndpointSlice

					BeforeEach(func() {
						remote.AddressType = discoveryv1beta1.AddressTypeIPv4
						remoteBefore = CreateEndpointSlice(&remote)
					})

					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("the remote object should be unmodified", func() {
						remoteAfter := GetEndpointSlice(RemoteNamespace)
						Expect(remoteAfter).To(Equal(remoteBefore))
					})
				})
			})
		})

		Context("address translation", func() {
			var (
				input, output []string
				err           error
			)

			BeforeEach(func() { input = []string{"192.168.0.25", "192.168.0.43"} })

			When("translating a set of IP addresses", func() {
				JustBeforeEach(func() {
					output, err = reflector.(*exposition.NamespacedEndpointSliceReflector).
						MapEndpointIPs(ctx, EndpointSliceName, input)
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should return the correct translations", func() { Expect(output).To(ConsistOf("192.168.200.25", "192.168.200.43")) })

				When("translating again the same set of IP addresses", func() {
					JustBeforeEach(func() {
						output, err = reflector.(*exposition.NamespacedEndpointSliceReflector).
							MapEndpointIPs(ctx, EndpointSliceName, input)
					})

					// The IPAMClient is configured to return an error if the same translation is requested twice.
					It("should succeed (i.e., use the cached values)", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should return the same translations", func() { Expect(output).To(ConsistOf("192.168.200.25", "192.168.200.43")) })
				})

				When("releasing the same set of IP addresses", func() {
					JustBeforeEach(func() {
						err = reflector.(*exposition.NamespacedEndpointSliceReflector).UnmapEndpointIPs(ctx, EndpointSliceName)
					})

					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
					It("should have released the translations", func() {
						Expect(ipam.IsEndpointTranslated("192.168.0.25")).To(BeFalse())
						Expect(ipam.IsEndpointTranslated("192.168.0.43")).To(BeFalse())
					})
				})

				When("releasing a different set of IP addresses", func() {
					JustBeforeEach(func() {
						err = reflector.(*exposition.NamespacedEndpointSliceReflector).UnmapEndpointIPs(ctx, "whatever")
					})
					It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				})
			})
		})
	})
})
