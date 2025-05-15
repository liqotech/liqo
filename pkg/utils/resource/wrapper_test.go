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

package resource

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("CreateOrUpdate Wrapper", func() {
	var (
		ctx context.Context
		cl  client.Client
		obj client.Object = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "testname",
				Namespace:   "testns",
				Labels:      map[string]string{"existing": "label"},
				Annotations: map[string]string{"existing": "annotation"},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: func() *int32 { i := int32(1); return &i }(),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "testapp"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "testapp"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "testcontainer",
								Image: "testimage",
							},
						},
					},
				},
			},
		}
	)

	BeforeEach(func() {
		ctx = context.TODO()
		cl = fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	})

	AfterEach(func() {
		SetGlobalLabels(nil)
		SetGlobalAnnotations(nil)
	})

	It("should inject global labels and annotations with variable replacement", func() {
		SetGlobalLabels(map[string]string{
			"app":           "liqo:${namespace}-${name}:${kind}-${group}",
			"env":           "test",
			"namespace-var": "${namespace}",
		})
		SetGlobalAnnotations(map[string]string{
			"anno": "liqo:${namespace}-${name}:${kind}-${group}",
		})

		mutateFn := func() error {
			// Simulate mutation logic
			return nil
		}

		newObj := obj.DeepCopyObject().(client.Object)

		_, err := CreateOrUpdate(ctx, cl, newObj, mutateFn)
		Expect(err).ToNot(HaveOccurred())

		groupKind, _, _ := cl.Scheme().ObjectKinds(newObj)
		expectedReplacedString := fmt.Sprintf("liqo:%s-%s:%s-%s",
			newObj.GetNamespace(),
			newObj.GetName(),
			groupKind[0].Kind,
			groupKind[0].Group,
		)

		labels := newObj.GetLabels()
		// Check that existing labels are preserved
		for k, v := range obj.GetLabels() {
			Expect(labels).To(HaveKeyWithValue(k, v))
		}
		Expect(labels).To(HaveKeyWithValue("app", expectedReplacedString))
		Expect(labels).To(HaveKeyWithValue("env", "test"))
		Expect(labels).To(HaveKeyWithValue("namespace-var", newObj.GetNamespace()))

		annotations := newObj.GetAnnotations()
		Expect(annotations).To(HaveKeyWithValue("anno", expectedReplacedString))
	})

	It("should not panic if no global labels/annotations are set", func() {
		mutateFn := func() error { return nil }
		_, err := CreateOrUpdate(ctx, cl, obj, mutateFn)
		Expect(err).ToNot(HaveOccurred())
	})
})
