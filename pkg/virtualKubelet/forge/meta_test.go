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

package forge_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/pointer"

	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

var _ = Describe("Meta forging", func() {
	Describe("Reflection labels", func() {

		Describe("the ReflectionLabels function", func() {
			It("should set exactly two labels", func() { Expect(forge.ReflectionLabels()).To(HaveLen(2)) })
			It("should set the origin cluster label", func() {
				Expect(forge.ReflectionLabels()).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(LocalClusterID)))
			})
			It("should set the destination cluster label", func() {
				Expect(forge.ReflectionLabels()).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, string(RemoteClusterID)))
			})
		})

		DescribeTableFactory := func(checker func(labels.Set) bool) func() {
			return func() {
				DescribeTable("checking whether there is a match",
					func(labels map[string]string, matches bool) {
						Expect(checker(labels)).To(BeIdenticalTo(matches))
					},
					Entry("when no label is specified", nil, false),
					Entry("when different labels are specified", map[string]string{"foo": "bar"}, false),
					Entry("when only one label is specified", map[string]string{forge.LiqoOriginClusterIDKey: string(LocalClusterID)}, false),
					Entry("when only the other label is specified", map[string]string{forge.LiqoDestinationClusterIDKey: string(RemoteClusterID)}, false),
					Entry("when both labels are specified, with incorrect values", map[string]string{
						forge.LiqoOriginClusterIDKey:      "foo",
						forge.LiqoDestinationClusterIDKey: "bar",
					}, false),
					Entry("when both labels are specified, with the correct values", map[string]string{
						forge.LiqoOriginClusterIDKey:      string(LocalClusterID),
						forge.LiqoDestinationClusterIDKey: string(RemoteClusterID),
					}, true),
				)
			}
		}

		Describe("the ReflectedLabelSelector function", DescribeTableFactory(func(labels labels.Set) bool {
			return forge.ReflectedLabelSelector().Matches(labels)
		}))
		Describe("the IsReflected function", DescribeTableFactory(func(labels labels.Set) bool {
			return forge.IsReflected(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Labels: labels}})
		}))
	})

	Describe("the RemoteObjectMeta function", func() {
		var local, remote, original, output metav1.ObjectMeta

		BeforeEach(func() {
			local = metav1.ObjectMeta{
				Name: "local-name", Namespace: "local-namespace",
				Labels:      map[string]string{"foo": "bar"},
				Annotations: map[string]string{"bar": "baz"},
			}
			remote = metav1.ObjectMeta{
				Name: "remote-name", Namespace: "remote-namespace", UID: "remote-uid",
				Labels:      map[string]string{"foo": "existing", "bar": "baz"},
				Annotations: map[string]string{"bar": "existing", "baz": "foo"},
			}
		})

		JustBeforeEach(func() {
			original = *local.DeepCopy()
			output = forge.RemoteObjectMeta(&local, &remote)
		})

		It("should correctly preserve the name and namespace, and accessory fields", func() {
			Expect(output.Name).To(Equal("remote-name"))
			Expect(output.Namespace).To(Equal("remote-namespace"))
			Expect(output.UID).To(BeEquivalentTo("remote-uid"))
		})

		It("should correctly set the labels", func() {
			// Check whether the reflection labels are present
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(LocalClusterID)))
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, string(RemoteClusterID)))
			// Check whether the local labels are present (higher precedence if a remote label matches)
			Expect(output.Labels).To(HaveKeyWithValue("foo", "bar"))
			// Check whether the remote labels are present
			Expect(output.Labels).NotTo(HaveKeyWithValue("bar", "baz"))
		})

		It("should correctly set the annotations", func() {
			// Check whether the local annotations are present (higher precedence if a remote annotation matches)
			Expect(output.Annotations).To(HaveKeyWithValue("bar", "baz"))
			// Check whether the remote annotations are present
			Expect(output.Annotations).NotTo(HaveKeyWithValue("baz", "foo"))
		})

		It("should not mutate the local object", func() { Expect(local).To(Equal(original)) })
	})

	Describe("the RemoteObjectReference function", func() {
		var (
			input  *corev1.ObjectReference
			output *corev1apply.ObjectReferenceApplyConfiguration
		)

		JustBeforeEach(func() { output = forge.RemoteObjectReference(input) })

		When("the ObjectReference is correctly initialized", func() {
			BeforeEach(func() {
				input = &corev1.ObjectReference{
					APIVersion: "foo.bar", Kind: "Kind",
					Namespace: "namespace", Name: "name",
					UID: "uid", ResourceVersion: "99999",
					FieldPath: "path.to.field",
				}
			})

			It("should correctly replicate all the fields", func() {
				Expect(output.APIVersion).To(PointTo(Equal("foo.bar")))
				Expect(output.Kind).To(PointTo(Equal("RemoteKind")))
				Expect(output.Namespace).To(PointTo(Equal("namespace")))
				Expect(output.Name).To(PointTo(Equal("name")))
				Expect(output.UID).To(PointTo(BeEquivalentTo("uid")))
				Expect(output.ResourceVersion).To(PointTo(Equal("99999")))
				Expect(output.FieldPath).To(PointTo(Equal("path.to.field")))
			})
		})

		When("the ObjectReference is nil", func() {
			BeforeEach(func() { input = nil })
			It("should return a nil output", func() { Expect(output).To(BeNil()) })
		})
	})

	Describe("the RemoteIngressResource function", func() {
		var (
			input  corev1.TypedLocalObjectReference
			output *corev1apply.TypedLocalObjectReferenceApplyConfiguration
		)

		BeforeEach(func() {
			input = corev1.TypedLocalObjectReference{
				APIGroup: pointer.String("example-group"),
				Kind:     "example-kind",
				Name:     "example-name",
			}
		})

		JustBeforeEach(func() { output = forge.RemoteTypedLocalObjectReference(&input) })

		It("should correctly replicate the resource fields", func() {
			Expect(output.APIGroup).To(PointTo(Equal("example-group")))
			Expect(output.Kind).To(PointTo(Equal("example-kind")))
			Expect(output.Name).To(PointTo(Equal("example-name")))
		})
	})
})
