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
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/pointer"

	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

var _ = Describe("ConfigMaps Forging", func() {
	Describe("the RemoteConfigMap function", func() {
		var (
			input       *corev1.ConfigMap
			output      *corev1apply.ConfigMapApplyConfiguration
			forgingOpts *forge.ForgingOpts
		)

		BeforeEach(func() {
			input = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "name", Namespace: "original",
					Labels:      map[string]string{"foo": "bar", testutil.FakeNotReflectedLabelKey: "true"},
					Annotations: map[string]string{"bar": "baz", testutil.FakeNotReflectedAnnotKey: "true"},
				},
				Data:       map[string]string{"data-key": "data value"},
				BinaryData: map[string][]byte{"binary-data-key": []byte("ABC")},
				Immutable:  pointer.Bool(true),
			}

			forgingOpts = testutil.FakeForgingOpts()
		})

		JustBeforeEach(func() { output = forge.RemoteConfigMap(input, "reflected", forgingOpts) })

		It("should correctly set the name and namespace", func() {
			Expect(output.Name).To(PointTo(Equal("name")))
			Expect(output.Namespace).To(PointTo(Equal("reflected")))
		})

		It("should correctly set the labels", func() {
			Expect(output.Labels).To(HaveKeyWithValue("foo", "bar"))
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(LocalClusterID)))
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, string(RemoteClusterID)))
			Expect(output.Labels).ToNot(HaveKey(testutil.FakeNotReflectedLabelKey))
		})

		It("should correctly set the annotations", func() {
			Expect(output.Annotations).To(HaveKeyWithValue("bar", "baz"))
			Expect(output.Annotations).ToNot(HaveKey(testutil.FakeNotReflectedAnnotKey))
		})

		It("should correctly set the data", func() {
			Expect(output.Data).NotTo(BeNil())
			Expect(output.Data).To(HaveKeyWithValue("data-key", "data value"))
		})

		It("should correctly set the binary data", func() {
			Expect(output.BinaryData).NotTo(BeNil())
			Expect(output.BinaryData).To(HaveKeyWithValue("binary-data-key", []byte("ABC")))
		})

		It("should correctly set the immutable field", func() {
			Expect(output.Immutable).NotTo(BeNil())
			Expect(output.Immutable).To(PointTo(BeTrue()))
		})

		When("it is the root CA configmap", func() {
			BeforeEach(func() { input.SetName(forge.RootCAConfigMapName) })

			It("should append a suffix to the name", func() {
				Expect(output.Name).To(PointTo(HaveSuffix(".local")))
			})
		})
	})
})
