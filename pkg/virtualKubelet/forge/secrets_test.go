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

package forge_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/pointer"

	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

var _ = Describe("Secrets Forging", func() {
	BeforeEach(func() { forge.Init(LocalClusterID, RemoteClusterID, LiqoNodeName, LiqoNodeIP) })

	Describe("the RemoteSecret function", func() {
		var (
			input  *corev1.Secret
			output *corev1apply.SecretApplyConfiguration
		)

		BeforeEach(func() {
			input = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "name", Namespace: "original",
					Labels:      map[string]string{"foo": "bar"},
					Annotations: map[string]string{"bar": "baz"},
				},
				Data:      map[string][]byte{"data-key": []byte("ABC")},
				Type:      corev1.SecretTypeBasicAuth,
				Immutable: pointer.Bool(true),
			}
		})

		JustBeforeEach(func() { output = forge.RemoteSecret(input, "reflected") })

		It("should correctly set the name and namespace", func() {
			Expect(output.Name).To(PointTo(Equal("name")))
			Expect(output.Namespace).To(PointTo(Equal("reflected")))
		})

		It("should correctly set the labels", func() {
			Expect(output.Labels).To(HaveKeyWithValue("foo", "bar"))
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
		})

		It("should correctly set the annotations", func() {
			Expect(output.Annotations).To(HaveKeyWithValue("bar", "baz"))
		})

		It("should correctly set the data", func() {
			Expect(output.Data).NotTo(BeNil())
			Expect(output.Data).To(HaveKeyWithValue("data-key", []byte("ABC")))
		})

		It("should correctly set the type", func() {
			Expect(output.Type).To(PointTo(Equal(corev1.SecretTypeBasicAuth)))
		})

		It("should correctly set the immutable field", func() {
			Expect(output.Immutable).NotTo(BeNil())
			Expect(output.Immutable).To(PointTo(BeTrue()))
		})

		When("it is of type ServiceAccountToken", func() {
			BeforeEach(func() {
				input.Type = corev1.SecretTypeServiceAccountToken
				input.Annotations[corev1.ServiceAccountNameKey] = "service-account"
			})

			It("should change the type to Opaque", func() {
				Expect(output.Type).To(PointTo(Equal(corev1.SecretTypeOpaque)))
			})

			It("should add a label with the ServiceAccount name", func() {
				Expect(output.Labels).To(HaveLen(4)) // Ensure existing labels are not removed
				Expect(output.Labels).To(HaveKeyWithValue(corev1.ServiceAccountNameKey, "service-account"))
			})
		})
	})
})
