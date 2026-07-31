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

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/forge"
)

var _ = Describe("The MutateResourceSlice function", func() {
	const (
		remoteClusterID = liqov1beta1.ClusterID("cool-firefly")
		existingClass   = authv1beta1.ResourceSliceClass("custom-class")
		requestedClass  = authv1beta1.ResourceSliceClass("other-class")
	)

	var (
		resourceSlice *authv1beta1.ResourceSlice
		opts          *forge.ResourceSliceOptions
		err           error
	)

	JustBeforeEach(func() {
		err = forge.MutateResourceSlice(resourceSlice, remoteClusterID, opts, true)
	})

	When("no class is requested, and the ResourceSlice already has one", func() {
		BeforeEach(func() {
			resourceSlice = &authv1beta1.ResourceSlice{Spec: authv1beta1.ResourceSliceSpec{Class: existingClass}}
			opts = &forge.ResourceSliceOptions{Class: authv1beta1.ResourceSliceClassUnknown}
		})

		It("should preserve the existing class, as it is immutable", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(resourceSlice.Spec.Class).To(BeEquivalentTo(existingClass))
		})
	})

	When("a class is requested", func() {
		BeforeEach(func() {
			resourceSlice = &authv1beta1.ResourceSlice{Spec: authv1beta1.ResourceSliceSpec{Class: existingClass}}
			opts = &forge.ResourceSliceOptions{Class: requestedClass}
		})

		It("should set the requested class, letting the API server reject the change", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(resourceSlice.Spec.Class).To(BeEquivalentTo(requestedClass))
		})
	})

	When("no class is requested, and the ResourceSlice does not have one", func() {
		BeforeEach(func() {
			resourceSlice = &authv1beta1.ResourceSlice{}
			opts = &forge.ResourceSliceOptions{Class: authv1beta1.ResourceSliceClassUnknown}
		})

		It("should leave the class empty, for the provider to resolve it", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(resourceSlice.Spec.Class).To(BeEquivalentTo(authv1beta1.ResourceSliceClassUnknown))
		})
	})
})
