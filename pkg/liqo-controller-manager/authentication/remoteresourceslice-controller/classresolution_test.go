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

package remoteresourceslicecontroller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/tools/record"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
)

var _ = Describe("ResourceSlice class resolution", func() {
	const (
		providerClass = authv1beta1.ResourceSliceClass("provider-class")
		customClass   = authv1beta1.ResourceSliceClass("custom-class")
	)

	var (
		reconciler    *RemoteResourceSliceReconciler
		resourceSlice *authv1beta1.ResourceSlice
	)

	// newReconciler returns a reconciler configured with the given provider-side default class.
	newReconciler := func(defaultClass authv1beta1.ResourceSliceClass) *RemoteResourceSliceReconciler {
		return &RemoteResourceSliceReconciler{
			eventRecorder: record.NewFakeRecorder(10),
			sliceStatusOptions: &SliceStatusOptions{
				DefaultResourceSliceClass: defaultClass,
			},
			reconciledClasses: []authv1beta1.ResourceSliceClass{
				authv1beta1.ResourceSliceClassDefault,
				authv1beta1.ResourceSliceClassUnknown,
			},
		}
	}

	newResourceSlice := func(spec, status authv1beta1.ResourceSliceClass) *authv1beta1.ResourceSlice {
		return &authv1beta1.ResourceSlice{
			Spec:   authv1beta1.ResourceSliceSpec{Class: spec},
			Status: authv1beta1.ResourceSliceStatus{Class: status},
		}
	}

	// handled marks the resources of the ResourceSlice as already processed by a class controller.
	handled := func(rs *authv1beta1.ResourceSlice) *authv1beta1.ResourceSlice {
		rs.Status.Conditions = append(rs.Status.Conditions, authv1beta1.ResourceSliceCondition{
			Type:   authv1beta1.ResourceSliceConditionTypeResources,
			Status: authv1beta1.ResourceSliceConditionAccepted,
		})
		return rs
	}

	Describe("The resolveClass function", func() {
		JustBeforeEach(func() { reconciler.resolveClass(resourceSlice) })

		When("no default class is configured on the provider", func() {
			BeforeEach(func() {
				reconciler = newReconciler(authv1beta1.ResourceSliceClassUnknown)
				resourceSlice = newResourceSlice(authv1beta1.ResourceSliceClassUnknown, authv1beta1.ResourceSliceClassUnknown)
			})

			It("should leave the class unresolved, preserving the previous behavior", func() {
				Expect(resourceSlice.Status.Class).To(BeEquivalentTo(authv1beta1.ResourceSliceClassUnknown))
				Expect(resourceSlice.EffectiveClass()).To(BeEquivalentTo(authv1beta1.ResourceSliceClassUnknown))
			})
		})

		When("a default class is configured, and the consumer did not request any", func() {
			BeforeEach(func() {
				reconciler = newReconciler(providerClass)
				resourceSlice = newResourceSlice(authv1beta1.ResourceSliceClassUnknown, authv1beta1.ResourceSliceClassUnknown)
			})

			It("should record the configured class in the status, leaving the spec untouched", func() {
				Expect(resourceSlice.Status.Class).To(BeEquivalentTo(providerClass))
				Expect(resourceSlice.Spec.Class).To(BeEquivalentTo(authv1beta1.ResourceSliceClassUnknown))
				Expect(resourceSlice.EffectiveClass()).To(BeEquivalentTo(providerClass))
			})
		})

		When("the class has already been resolved", func() {
			BeforeEach(func() {
				reconciler = newReconciler(providerClass)
				resourceSlice = newResourceSlice(authv1beta1.ResourceSliceClassUnknown, customClass)
			})

			It("should never reconsider it, so that changing the default only affects new ResourceSlices", func() {
				Expect(resourceSlice.Status.Class).To(BeEquivalentTo(customClass))
			})
		})

		When("the consumer requested an explicit class", func() {
			BeforeEach(func() {
				reconciler = newReconciler(providerClass)
				resourceSlice = newResourceSlice(customClass, authv1beta1.ResourceSliceClassUnknown)
			})

			It("should honor it, without resolving anything", func() {
				Expect(resourceSlice.Status.Class).To(BeEquivalentTo(authv1beta1.ResourceSliceClassUnknown))
				Expect(resourceSlice.EffectiveClass()).To(BeEquivalentTo(customClass))
			})
		})

		When("the resources have already been handled, before any default was configured", func() {
			BeforeEach(func() {
				reconciler = newReconciler(providerClass)
				resourceSlice = handled(newResourceSlice(authv1beta1.ResourceSliceClassUnknown, authv1beta1.ResourceSliceClassUnknown))
			})

			It("should leave it to the controller already responsible for it", func() {
				Expect(resourceSlice.Status.Class).To(BeEquivalentTo(authv1beta1.ResourceSliceClassUnknown))
				Expect(isInResourceClasses(resourceSlice, reconciler.reconciledClasses...)).To(BeTrue())
			})
		})
	})

	Describe("The isInResourceClasses function", func() {
		var claimed bool

		JustBeforeEach(func() {
			reconciler.resolveClass(resourceSlice)
			claimed = isInResourceClasses(resourceSlice, reconciler.reconciledClasses...)
		})

		When("the empty class is remapped to a custom one", func() {
			BeforeEach(func() {
				reconciler = newReconciler(providerClass)
				resourceSlice = newResourceSlice(authv1beta1.ResourceSliceClassUnknown, authv1beta1.ResourceSliceClassUnknown)
			})

			It("should leave the ResourceSlice to the controller of the resolved class", func() {
				Expect(claimed).To(BeFalse())
			})
		})

		When("no default class is configured", func() {
			BeforeEach(func() {
				reconciler = newReconciler(authv1beta1.ResourceSliceClassUnknown)
				resourceSlice = newResourceSlice(authv1beta1.ResourceSliceClassUnknown, authv1beta1.ResourceSliceClassUnknown)
			})

			It("should keep handling the empty class with the built-in one", func() {
				Expect(claimed).To(BeTrue())
			})
		})

		When("the empty class is remapped to the built-in default one", func() {
			BeforeEach(func() {
				reconciler = newReconciler(authv1beta1.ResourceSliceClassDefault)
				resourceSlice = newResourceSlice(authv1beta1.ResourceSliceClassUnknown, authv1beta1.ResourceSliceClassUnknown)
			})

			It("should keep handling it with the built-in one", func() {
				Expect(claimed).To(BeTrue())
			})
		})
	})
})
