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

package dra_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	resourcev1 "k8s.io/api/resource/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/trace"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/dra"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ = Describe("NamespacedResourceClaimReflector", func() {
	const (
		claimName       = "claim-1"
		deviceClassName = "gpu-class"
	)

	var (
		reflector manager.NamespacedReflector
		err       error
	)

	createLocalClaim := func(c *resourcev1.ResourceClaim) *resourcev1.ResourceClaim {
		c.Namespace = LocalNamespace
		out, e := localClient.ResourceV1().ResourceClaims(LocalNamespace).Create(ctx, c, metav1.CreateOptions{})
		Expect(e).ToNot(HaveOccurred())
		return out
	}
	createRemoteClaim := func(c *resourcev1.ResourceClaim) *resourcev1.ResourceClaim {
		c.Namespace = RemoteNamespace
		out, e := remoteClient.ResourceV1().ResourceClaims(RemoteNamespace).Create(ctx, c, metav1.CreateOptions{})
		Expect(e).ToNot(HaveOccurred())
		return out
	}
	createLocalDeviceClass := func(name string) {
		_, e := localClient.ResourceV1().DeviceClasses().Create(ctx, &resourcev1.DeviceClass{
			ObjectMeta: metav1.ObjectMeta{Name: name},
		}, metav1.CreateOptions{})
		Expect(e).ToNot(HaveOccurred())
	}
	createRemoteDeviceClass := func(name string) *resourcev1.DeviceClass {
		out, e := remoteClient.ResourceV1().DeviceClasses().Create(ctx, &resourcev1.DeviceClass{
			ObjectMeta: metav1.ObjectMeta{Name: name},
		}, metav1.CreateOptions{})
		Expect(e).ToNot(HaveOccurred())
		return out
	}

	mkBasicLocalClaim := func() *resourcev1.ResourceClaim {
		return &resourcev1.ResourceClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: claimName,
				Labels: map[string]string{
					fooVal:                            barVal,
					testutil.FakeNotReflectedLabelKey: trueVal,
				},
			},
			Spec: resourcev1.ResourceClaimSpec{
				Devices: resourcev1.DeviceClaim{
					Requests: []resourcev1.DeviceRequest{{
						Name:    fakeClaimName,
						Exactly: &resourcev1.ExactDeviceRequest{DeviceClassName: deviceClassName, Count: 1},
					}},
				},
			},
		}
	}

	JustBeforeEach(func() {
		localFactory := informers.NewSharedInformerFactory(localClient, 10*time.Hour)
		remoteFactory := informers.NewSharedInformerFactory(remoteClient, 10*time.Hour)
		cfg := offloadingv1beta1.ReflectorConfig{
			NumWorkers: 0,
			Type:       offloadingv1beta1.CustomLiqo,
		}
		rfl := dra.NewResourceClaimReflector(&cfg).(*dra.ResourceClaimReflector)
		rfl.Start(ctx, options.New(localClient, localFactory.Core().V1().Pods()))
		reflector = rfl.NewNamespaced(options.NewNamespaced().
			WithLocal(LocalNamespace, localClient, localFactory).
			WithRemote(RemoteNamespace, remoteClient, remoteFactory).
			WithHandlerFactory(FakeEventHandler).
			WithEventBroadcaster(record.NewBroadcaster()).
			WithReflectionType(offloadingv1beta1.CustomLiqo).
			WithForgingOpts(testutil.FakeForgingOpts()))

		localFactory.Start(ctx.Done())
		remoteFactory.Start(ctx.Done())
		localFactory.WaitForCacheSync(ctx.Done())
		remoteFactory.WaitForCacheSync(ctx.Done())

		err = reflector.Handle(trace.ContextWithTrace(ctx, trace.New("ResourceClaim")), claimName)
	})

	Describe("Handle", func() {
		Context("when both local and remote claims are missing", func() {
			It("should be a no-op and return nil", func() {
				Expect(err).ToNot(HaveOccurred())
				_, gerr := remoteClient.ResourceV1().ResourceClaims(RemoteNamespace).Get(ctx, claimName, metav1.GetOptions{})
				Expect(kerrors.IsNotFound(gerr)).To(BeTrue())
			})
		})

		Context("when the local exists and the remote does not", func() {
			Context("with a referenced DeviceClass that exists locally and is missing remotely", func() {
				BeforeEach(func() {
					createLocalDeviceClass(deviceClassName)
					createLocalClaim(mkBasicLocalClaim())
				})

				It("should reflect the DeviceClass to remote with reflection labels and create the remote claim", func() {
					Expect(err).ToNot(HaveOccurred())

					dc, gerr := remoteClient.ResourceV1().DeviceClasses().Get(ctx, deviceClassName, metav1.GetOptions{})
					Expect(gerr).ToNot(HaveOccurred())
					Expect(forge.IsReflected(dc)).To(BeTrue())

					rc, gerr := remoteClient.ResourceV1().ResourceClaims(RemoteNamespace).Get(ctx, claimName, metav1.GetOptions{})
					Expect(gerr).ToNot(HaveOccurred())
					Expect(rc.Labels).To(HaveKeyWithValue(fooVal, barVal))
					Expect(rc.Labels).ToNot(HaveKey(testutil.FakeNotReflectedLabelKey))
					Expect(rc.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
					Expect(rc.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
				})
			})

			Context("with a referenced DeviceClass that is missing locally", func() {
				BeforeEach(func() { createLocalClaim(mkBasicLocalClaim()) })

				It("should skip the claim (no remote claim created) and not return an error", func() {
					// Implementation translates the local-not-found to a soft skip + warning.
					Expect(err).ToNot(HaveOccurred())
					_, gerr := remoteClient.ResourceV1().ResourceClaims(RemoteNamespace).Get(ctx, claimName, metav1.GetOptions{})
					Expect(kerrors.IsNotFound(gerr)).To(BeTrue())
				})
			})

			Context("with the referenced DeviceClass already present remotely", func() {
				var initialDCResourceVersion string

				BeforeEach(func() {
					createLocalDeviceClass(deviceClassName)
					existing := createRemoteDeviceClass(deviceClassName)
					initialDCResourceVersion = existing.ResourceVersion
					createLocalClaim(mkBasicLocalClaim())
				})

				It("should not re-create the DeviceClass remotely (resourceVersion preserved)", func() {
					Expect(err).ToNot(HaveOccurred())
					dc, gerr := remoteClient.ResourceV1().DeviceClasses().Get(ctx, deviceClassName, metav1.GetOptions{})
					Expect(gerr).ToNot(HaveOccurred())
					Expect(dc.ResourceVersion).To(Equal(initialDCResourceVersion))
				})
			})

			Context("with multiple DeviceClasses across Exactly and FirstAvailable", func() {
				const dc2 = "fpga-class"

				BeforeEach(func() {
					createLocalDeviceClass(deviceClassName)
					createLocalDeviceClass(dc2)
					l := mkBasicLocalClaim()
					l.Spec.Devices.Requests = append(l.Spec.Devices.Requests, resourcev1.DeviceRequest{
						Name: "req-2",
						FirstAvailable: []resourcev1.DeviceSubRequest{
							{Name: "s1", DeviceClassName: dc2},
							{Name: "s2", DeviceClassName: deviceClassName}, // duplicate
						},
					})
					createLocalClaim(l)
				})

				It("should reflect each unique DeviceClass exactly once on the remote", func() {
					Expect(err).ToNot(HaveOccurred())
					_, gerr := remoteClient.ResourceV1().DeviceClasses().Get(ctx, deviceClassName, metav1.GetOptions{})
					Expect(gerr).ToNot(HaveOccurred())
					_, gerr = remoteClient.ResourceV1().DeviceClasses().Get(ctx, dc2, metav1.GetOptions{})
					Expect(gerr).ToNot(HaveOccurred())
				})
			})
		})

		Context("when the local exists and the remote already exists and is reflected", func() {
			Context("with the same labels and annotations as desired", func() {
				var rvBefore string

				BeforeEach(func() {
					createLocalDeviceClass(deviceClassName)
					createRemoteDeviceClass(deviceClassName)
					local := createLocalClaim(mkBasicLocalClaim())

					desired := forge.RemoteResourceClaim(local, RemoteNamespace,
						testutil.FakeForgingOpts().LabelsNotReflected,
						testutil.FakeForgingOpts().AnnotationsNotReflected)
					created := createRemoteClaim(desired)
					rvBefore = created.ResourceVersion
				})

				It("should be a no-op (resourceVersion unchanged)", func() {
					Expect(err).ToNot(HaveOccurred())
					rc, gerr := remoteClient.ResourceV1().ResourceClaims(RemoteNamespace).Get(ctx, claimName, metav1.GetOptions{})
					Expect(gerr).ToNot(HaveOccurred())
					Expect(rc.ResourceVersion).To(Equal(rvBefore))
				})
			})

			Context("with stale labels", func() {
				BeforeEach(func() {
					createLocalDeviceClass(deviceClassName)
					createRemoteDeviceClass(deviceClassName)
					createLocalClaim(mkBasicLocalClaim())

					stale := &resourcev1.ResourceClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: claimName,
							Labels: map[string]string{
								forge.LiqoOriginClusterIDKey:      LocalClusterID,
								forge.LiqoDestinationClusterIDKey: RemoteClusterID,
								"old":                             "label",
							},
						},
						Spec: resourcev1.ResourceClaimSpec{
							Devices: resourcev1.DeviceClaim{
								Requests: []resourcev1.DeviceRequest{{
									Name:    fakeClaimName,
									Exactly: &resourcev1.ExactDeviceRequest{DeviceClassName: deviceClassName, Count: 1},
								}},
							},
						},
					}
					createRemoteClaim(stale)
				})

				It("should update labels on the remote claim", func() {
					Expect(err).ToNot(HaveOccurred())
					rc, gerr := remoteClient.ResourceV1().ResourceClaims(RemoteNamespace).Get(ctx, claimName, metav1.GetOptions{})
					Expect(gerr).ToNot(HaveOccurred())
					Expect(rc.Labels).To(HaveKeyWithValue(fooVal, barVal))
					Expect(rc.Labels).ToNot(HaveKey("old"))
				})
			})
		})

		Context("when a non-reflected remote claim exists", func() {
			var rvBefore string

			BeforeEach(func() {
				createLocalDeviceClass(deviceClassName)
				createLocalClaim(mkBasicLocalClaim())
				existing := &resourcev1.ResourceClaim{
					ObjectMeta: metav1.ObjectMeta{Name: claimName},
					Spec: resourcev1.ResourceClaimSpec{
						Devices: resourcev1.DeviceClaim{
							Requests: []resourcev1.DeviceRequest{{
								Name:    fakeClaimName,
								Exactly: &resourcev1.ExactDeviceRequest{DeviceClassName: deviceClassName, Count: 1},
							}},
						},
					},
				}
				created := createRemoteClaim(existing)
				rvBefore = created.ResourceVersion
			})

			It("should skip without modifying the remote claim", func() {
				Expect(err).ToNot(HaveOccurred())
				rc, gerr := remoteClient.ResourceV1().ResourceClaims(RemoteNamespace).Get(ctx, claimName, metav1.GetOptions{})
				Expect(gerr).ToNot(HaveOccurred())
				Expect(rc.ResourceVersion).To(Equal(rvBefore))
			})
		})

		Context("when the local is missing and the remote exists", func() {
			Context("and the remote is reflected by Liqo", func() {
				BeforeEach(func() {
					createRemoteClaim(&resourcev1.ResourceClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name:   claimName,
							Labels: forge.ReflectionLabels(),
						},
						Spec: resourcev1.ResourceClaimSpec{
							Devices: resourcev1.DeviceClaim{
								Requests: []resourcev1.DeviceRequest{{
									Name:    fakeClaimName,
									Exactly: &resourcev1.ExactDeviceRequest{DeviceClassName: deviceClassName, Count: 1},
								}},
							},
						},
					})
				})

				It("should delete the remote claim", func() {
					Expect(err).ToNot(HaveOccurred())
					_, gerr := remoteClient.ResourceV1().ResourceClaims(RemoteNamespace).Get(ctx, claimName, metav1.GetOptions{})
					Expect(kerrors.IsNotFound(gerr)).To(BeTrue())
				})
			})

			Context("and the remote is NOT reflected by Liqo", func() {
				var rvBefore string

				BeforeEach(func() {
					created := createRemoteClaim(&resourcev1.ResourceClaim{
						ObjectMeta: metav1.ObjectMeta{Name: claimName},
						Spec: resourcev1.ResourceClaimSpec{
							Devices: resourcev1.DeviceClaim{
								Requests: []resourcev1.DeviceRequest{{
									Name:    fakeClaimName,
									Exactly: &resourcev1.ExactDeviceRequest{DeviceClassName: deviceClassName, Count: 1},
								}},
							},
						},
					})
					rvBefore = created.ResourceVersion
				})

				It("should leave the remote claim untouched", func() {
					Expect(err).ToNot(HaveOccurred())
					rc, gerr := remoteClient.ResourceV1().ResourceClaims(RemoteNamespace).Get(ctx, claimName, metav1.GetOptions{})
					Expect(gerr).ToNot(HaveOccurred())
					Expect(rc.ResourceVersion).To(Equal(rvBefore))
				})
			})
		})
	})
})
