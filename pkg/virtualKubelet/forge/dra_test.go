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

package forge_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

const (
	barVal            = "bar"
	bazVal            = "baz"
	fooVal            = "foo"
	trueVal           = "true"
	fakeMutatedValue  = "mutated"
	fakeGPUClassValue = "gpu-class"
	fakeGPUValue      = "gpu"
	fakeFPGAValue     = "fpga"
)

var _ = Describe("DRA forging", func() {

	Describe("the LocalResourceSlice function", func() {
		var (
			remote *resourcev1.ResourceSlice
			node   *corev1.Node
			output *resourcev1.ResourceSlice
			opts   *forge.ForgingOpts
		)

		BeforeEach(func() {
			remote = &resourcev1.ResourceSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name: "slice",
					Labels: map[string]string{
						fooVal:                            barVal,
						testutil.FakeNotReflectedLabelKey: trueVal,
					},
					Annotations: map[string]string{
						barVal:                            bazVal,
						testutil.FakeNotReflectedAnnotKey: trueVal,
					},
				},
				Spec: resourcev1.ResourceSliceSpec{
					Driver:   "test-driver",
					NodeName: ptr.To("remote-node"),
					Pool: resourcev1.ResourcePool{
						Name:               "pool-a",
						Generation:         3,
						ResourceSliceCount: 1,
					},
					Devices: []resourcev1.Device{{Name: "dev-1"}},
				},
			}
			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: LiqoNodeName,
					UID:  types.UID("node-uid-123"),
				},
			}
			opts = testutil.FakeForgingOpts()
		})

		JustBeforeEach(func() {
			output = forge.LocalResourceSlice(remote, node, opts.LabelsNotReflected, opts.AnnotationsNotReflected)
		})

		It("should preserve the slice name", func() {
			Expect(output.Name).To(Equal("slice"))
		})

		It("should merge reflection labels with the originals", func() {
			Expect(output.Labels).To(HaveKeyWithValue(fooVal, barVal))
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(LocalClusterID)))
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, string(RemoteClusterID)))
		})

		It("should filter out the not-reflected label/annotation keys", func() {
			Expect(output.Labels).ToNot(HaveKey(testutil.FakeNotReflectedLabelKey))
			Expect(output.Annotations).To(HaveKeyWithValue(barVal, bazVal))
			Expect(output.Annotations).ToNot(HaveKey(testutil.FakeNotReflectedAnnotKey))
		})

		It("should set a single OwnerReference pointing at the local node", func() {
			Expect(output.OwnerReferences).To(HaveLen(1))
			ref := output.OwnerReferences[0]
			Expect(ref.APIVersion).To(Equal("v1"))
			Expect(ref.Kind).To(Equal("Node"))
			Expect(ref.Name).To(Equal(node.Name))
			Expect(ref.UID).To(Equal(node.UID))
			Expect(ref.Controller).To(PointTo(BeTrue()))
			Expect(ref.BlockOwnerDeletion).To(PointTo(BeFalse()))
		})

		It("should deep-copy the spec (mutating remote does not affect the output)", func() {
			Expect(output.Spec.Driver).To(Equal("test-driver"))
			remote.Spec.Driver = fakeMutatedValue
			remote.Spec.Devices[0].Name = "mutated-dev"
			Expect(output.Spec.Driver).To(Equal("test-driver"))
			Expect(output.Spec.Devices[0].Name).To(Equal("dev-1"))
		})

		When("the remote has nil labels and annotations", func() {
			BeforeEach(func() {
				remote.Labels = nil
				remote.Annotations = nil
			})
			It("should still produce reflection labels without panicking", func() {
				Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(LocalClusterID)))
				Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, string(RemoteClusterID)))
			})
		})

		When("the remote labels conflict with reflection-managed keys", func() {
			BeforeEach(func() {
				remote.Labels[forge.LiqoOriginClusterIDKey] = "some-other-cluster"
			})
			It("should let reflection labels win the merge", func() {
				Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(LocalClusterID)))
			})
		})
	})

	Describe("the RemoteResourceClaim function", func() {
		const (
			localNamespace  = "local-namespace"
			remoteNamespace = "remote-namespace"
		)

		var (
			local  *resourcev1.ResourceClaim
			output *resourcev1.ResourceClaim
			opts   *forge.ForgingOpts
		)

		BeforeEach(func() {
			local = &resourcev1.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "claim",
					Namespace: localNamespace,
					Labels: map[string]string{
						fooVal:                            barVal,
						testutil.FakeNotReflectedLabelKey: trueVal,
					},
					Annotations: map[string]string{
						barVal:                            bazVal,
						testutil.FakeNotReflectedAnnotKey: trueVal,
					},
				},
				Spec: resourcev1.ResourceClaimSpec{
					Devices: resourcev1.DeviceClaim{
						Requests: []resourcev1.DeviceRequest{{
							Name: "req-1",
							Exactly: &resourcev1.ExactDeviceRequest{
								DeviceClassName: fakeGPUClassValue,
								Count:           1,
							},
						}},
					},
				},
				Status: resourcev1.ResourceClaimStatus{
					Allocation: &resourcev1.AllocationResult{
						Devices: resourcev1.DeviceAllocationResult{
							Results: []resourcev1.DeviceRequestAllocationResult{{Driver: "x"}},
						},
					},
				},
			}
			opts = testutil.FakeForgingOpts()
		})

		JustBeforeEach(func() {
			output = forge.RemoteResourceClaim(local, remoteNamespace, opts.LabelsNotReflected, opts.AnnotationsNotReflected)
		})

		It("should preserve the name and remap the namespace", func() {
			Expect(output.Name).To(Equal("claim"))
			Expect(output.Namespace).To(Equal(remoteNamespace))
		})

		It("should merge reflection labels and filter not-reflected keys", func() {
			Expect(output.Labels).To(HaveKeyWithValue(fooVal, barVal))
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(LocalClusterID)))
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, string(RemoteClusterID)))
			Expect(output.Labels).ToNot(HaveKey(testutil.FakeNotReflectedLabelKey))
			Expect(output.Annotations).To(HaveKeyWithValue(barVal, bazVal))
			Expect(output.Annotations).ToNot(HaveKey(testutil.FakeNotReflectedAnnotKey))
		})

		It("should deep-copy the spec", func() {
			Expect(output.Spec.Devices.Requests[0].Exactly.DeviceClassName).To(Equal(fakeGPUClassValue))
			local.Spec.Devices.Requests[0].Exactly.DeviceClassName = fakeMutatedValue
			Expect(output.Spec.Devices.Requests[0].Exactly.DeviceClassName).To(Equal(fakeGPUClassValue))
		})

		It("should not propagate status (allocation is owned by remote)", func() {
			Expect(output.Status).To(Equal(resourcev1.ResourceClaimStatus{}))
		})

		When("the local has nil labels and annotations", func() {
			BeforeEach(func() {
				local.Labels = nil
				local.Annotations = nil
			})
			It("should still produce reflection labels without panicking", func() {
				Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(LocalClusterID)))
			})
		})
	})

	Describe("the RemoteDeviceClass function", func() {
		var (
			local  *resourcev1.DeviceClass
			output *resourcev1.DeviceClass
			opts   *forge.ForgingOpts
		)

		BeforeEach(func() {
			local = &resourcev1.DeviceClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: fakeGPUClassValue,
					Labels: map[string]string{
						fooVal:                            barVal,
						testutil.FakeNotReflectedLabelKey: trueVal,
					},
					Annotations: map[string]string{
						barVal:                            bazVal,
						testutil.FakeNotReflectedAnnotKey: trueVal,
					},
				},
				Spec: resourcev1.DeviceClassSpec{
					Selectors: []resourcev1.DeviceSelector{
						{CEL: &resourcev1.CELDeviceSelector{Expression: "device.attributes.foo == \"bar\""}},
					},
				},
			}
			opts = testutil.FakeForgingOpts()
		})

		JustBeforeEach(func() {
			output = forge.RemoteDeviceClass(local, opts.LabelsNotReflected, opts.AnnotationsNotReflected)
		})

		It("should preserve the cluster-scoped name", func() {
			Expect(output.Name).To(Equal(fakeGPUClassValue))
			Expect(output.Namespace).To(BeEmpty())
		})

		It("should merge reflection labels and filter not-reflected keys", func() {
			Expect(output.Labels).To(HaveKeyWithValue(fooVal, barVal))
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(LocalClusterID)))
			Expect(output.Labels).ToNot(HaveKey(testutil.FakeNotReflectedLabelKey))
			Expect(output.Annotations).To(HaveKeyWithValue(barVal, bazVal))
			Expect(output.Annotations).ToNot(HaveKey(testutil.FakeNotReflectedAnnotKey))
		})

		It("should deep-copy the spec", func() {
			Expect(output.Spec.Selectors).To(HaveLen(1))
			local.Spec.Selectors[0].CEL.Expression = fakeMutatedValue
			Expect(output.Spec.Selectors[0].CEL.Expression).To(Equal("device.attributes.foo == \"bar\""))
		})
	})

	Describe("the ReferencedDeviceClasses function", func() {
		newClaimWithRequests := func(reqs ...resourcev1.DeviceRequest) *resourcev1.ResourceClaim {
			return &resourcev1.ResourceClaim{Spec: resourcev1.ResourceClaimSpec{
				Devices: resourcev1.DeviceClaim{Requests: reqs},
			}}
		}

		It("should return the single Exactly DeviceClassName", func() {
			out := forge.ReferencedDeviceClasses(newClaimWithRequests(
				resourcev1.DeviceRequest{Name: "exactDevice", Exactly: &resourcev1.ExactDeviceRequest{DeviceClassName: fakeGPUValue}},
			))
			Expect(out).To(ConsistOf(fakeGPUValue))
		})

		It("should deduplicate Exactly entries that reference the same class", func() {
			out := forge.ReferencedDeviceClasses(newClaimWithRequests(
				resourcev1.DeviceRequest{Name: "ded1", Exactly: &resourcev1.ExactDeviceRequest{DeviceClassName: fakeGPUValue}},
				resourcev1.DeviceRequest{Name: "ded2", Exactly: &resourcev1.ExactDeviceRequest{DeviceClassName: fakeGPUValue}},
				resourcev1.DeviceRequest{Name: "ded3", Exactly: &resourcev1.ExactDeviceRequest{DeviceClassName: fakeFPGAValue}},
			))
			Expect(out).To(ConsistOf(fakeGPUValue, fakeFPGAValue))
		})

		It("should extract names from FirstAvailable subrequests", func() {
			out := forge.ReferencedDeviceClasses(newClaimWithRequests(
				resourcev1.DeviceRequest{
					Name: "firstAvailable1",
					FirstAvailable: []resourcev1.DeviceSubRequest{
						{Name: "firstAvailableSub1", DeviceClassName: fakeGPUValue},
						{Name: "firstAvailableSub2", DeviceClassName: fakeFPGAValue},
					},
				},
			))
			Expect(out).To(ConsistOf(fakeGPUValue, fakeFPGAValue))
		})

		It("should deduplicate across Exactly and FirstAvailable", func() {
			out := forge.ReferencedDeviceClasses(newClaimWithRequests(
				resourcev1.DeviceRequest{Name: "exactDevice1", Exactly: &resourcev1.ExactDeviceRequest{DeviceClassName: fakeGPUValue}},
				resourcev1.DeviceRequest{
					Name: "exactDevice2",
					FirstAvailable: []resourcev1.DeviceSubRequest{
						{Name: "exactDevice2Sub1", DeviceClassName: fakeGPUValue},
						{Name: "exactDevice2Sub2", DeviceClassName: fakeFPGAValue},
					},
				},
			))
			Expect(out).To(ConsistOf(fakeGPUValue, fakeFPGAValue))
		})

		It("should return an empty slice for an empty Requests list", func() {
			out := forge.ReferencedDeviceClasses(newClaimWithRequests())
			Expect(out).To(BeEmpty())
		})

		It("should skip empty-string DeviceClassName entries", func() {
			out := forge.ReferencedDeviceClasses(newClaimWithRequests(
				resourcev1.DeviceRequest{Name: "emptyClassName", Exactly: &resourcev1.ExactDeviceRequest{DeviceClassName: ""}},
				resourcev1.DeviceRequest{
					Name: "emptyClassName2",
					FirstAvailable: []resourcev1.DeviceSubRequest{
						{Name: "emptyClassNameSub1", DeviceClassName: ""},
						{Name: "emptyClassNameSub2", DeviceClassName: fakeFPGAValue},
					},
				},
			))
			Expect(out).To(ConsistOf(fakeFPGAValue))
		})
	})
})
