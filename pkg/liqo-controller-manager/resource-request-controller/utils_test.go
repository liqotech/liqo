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

package resourcerequestoperator

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

var _ = Describe("Utility functions", func() {

	Describe("The ensureControllerReference function", func() {
		var (
			r ResourceRequestReconciler

			foreignCluster  discoveryv1alpha1.ForeignCluster
			resourceRequest discoveryv1alpha1.ResourceRequest

			update bool
			err    error
		)

		BeforeEach(func() {
			r = ResourceRequestReconciler{Scheme: scheme.Scheme}
			foreignCluster = discoveryv1alpha1.ForeignCluster{
				TypeMeta: metav1.TypeMeta{
					APIVersion: discoveryv1alpha1.GroupVersion.String(),
					Kind:       "ForeignCluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "foreign-cluster",
					UID:  "8a402261-9cf4-402e-89e8-4d743fb315fb",
				},
			}
			resourceRequest = discoveryv1alpha1.ResourceRequest{
				TypeMeta: metav1.TypeMeta{
					APIVersion: discoveryv1alpha1.GroupVersion.String(),
					Kind:       "ResourceRequest",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "resource-request",
					Namespace: "foo",
				},
			}
		})

		JustBeforeEach(func() {
			update, err = r.ensureControllerReference(&foreignCluster, &resourceRequest)
		})

		When("The resource request has not a controller reference", func() {
			It("Should not return an error", func() { Expect(err).NotTo(HaveOccurred()) })
			It("Should return that an update is needed", func() { Expect(update).To(BeTrue()) })
			It("Should set the correct controller reference", func() {
				Expect(metav1.GetControllerOf(&resourceRequest).Kind).To(Equal(foreignCluster.Kind))
				Expect(metav1.GetControllerOf(&resourceRequest).APIVersion).To(Equal(foreignCluster.APIVersion))
				Expect(metav1.GetControllerOf(&resourceRequest).Name).To(Equal(foreignCluster.GetName()))
				Expect(metav1.GetControllerOf(&resourceRequest).UID).To(Equal(foreignCluster.GetUID()))
			})
		})

		When("The resource request has already a controller reference", func() {
			var controller discoveryv1alpha1.ForeignCluster

			BeforeEach(func() {
				controller = discoveryv1alpha1.ForeignCluster{
					TypeMeta: foreignCluster.TypeMeta,
					ObjectMeta: metav1.ObjectMeta{
						Name: "foreign-cluster-controller",
						UID:  "1d392296-3798-47d2-8e8d-73dc3f65ffc8",
					},
				}

				Expect(controllerutil.SetControllerReference(&controller, &resourceRequest, r.Scheme)).To(Succeed())
			})

			It("Should not return an error", func() { Expect(err).NotTo(HaveOccurred()) })
			It("Should return that an update is not needed", func() { Expect(update).To(BeFalse()) })
			It("Should not modify the controller reference", func() {
				Expect(metav1.GetControllerOf(&resourceRequest).Kind).To(Equal(controller.Kind))
				Expect(metav1.GetControllerOf(&resourceRequest).APIVersion).To(Equal(controller.APIVersion))
				Expect(metav1.GetControllerOf(&resourceRequest).Name).To(Equal(controller.GetName()))
				Expect(metav1.GetControllerOf(&resourceRequest).UID).To(Equal(controller.GetUID()))
			})
		})
	})
})
