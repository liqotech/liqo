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

package provider

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

var _ = Describe("discoverGVR", func() {
	var gvr = schema.GroupVersionResource{Group: "example.io", Version: "v1", Resource: "widgets"}

	// newDiscoveryClient returns a fake clientset whose discovery serves the given API
	// resources for the group version of the GVR under test.
	newDiscoveryClient := func(resources ...metav1.APIResource) kubernetes.Interface {
		client := k8sfake.NewClientset()
		client.PrependReactor("get", "resource", func(_ clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, nil
		})
		client.Discovery().(*fakediscovery.FakeDiscovery).Resources = []*metav1.APIResourceList{{
			GroupVersion: gvr.GroupVersion().String(),
			APIResources: resources,
		}}
		return client
	}

	When("the resource is namespaced and exposes the status subresource", func() {
		It("should report it as available, namespaced, with status support", func() {
			client := newDiscoveryClient(
				metav1.APIResource{Name: "widgets", Namespaced: true},
				metav1.APIResource{Name: "widgets/status", Namespaced: true},
			)
			available, namespaced, statusSupported, err := discoverGVR(client, gvr)
			Expect(err).ToNot(HaveOccurred())
			Expect(available).To(BeTrue())
			Expect(namespaced).To(BeTrue())
			Expect(statusSupported).To(BeTrue())
		})
	})

	When("the resource is cluster-scoped without the status subresource", func() {
		It("should report it as available, cluster-scoped, without status support", func() {
			client := newDiscoveryClient(
				metav1.APIResource{Name: "widgets", Namespaced: false},
			)
			available, namespaced, statusSupported, err := discoverGVR(client, gvr)
			Expect(err).ToNot(HaveOccurred())
			Expect(available).To(BeTrue())
			Expect(namespaced).To(BeFalse())
			Expect(statusSupported).To(BeFalse())
		})
	})

	When("the group version is not served", func() {
		It("should report it as not available, without errors", func() {
			client := newDiscoveryClient()
			available, namespaced, statusSupported, err := discoverGVR(client, gvr)
			Expect(err).ToNot(HaveOccurred())
			Expect(available).To(BeFalse())
			Expect(namespaced).To(BeFalse())
			Expect(statusSupported).To(BeFalse())
		})
	})

	When("the discovery returns a not found error", func() {
		It("should report it as not available, without errors", func() {
			client := k8sfake.NewClientset()
			client.PrependReactor("get", "resource", func(_ clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, kerrors.NewNotFound(schema.GroupResource{Group: "discovery", Resource: "resources"}, gvr.GroupVersion().String())
			})
			available, namespaced, statusSupported, err := discoverGVR(client, gvr)
			Expect(err).ToNot(HaveOccurred())
			Expect(available).To(BeFalse())
			Expect(namespaced).To(BeFalse())
			Expect(statusSupported).To(BeFalse())
		})
	})

	When("the discovery returns an unexpected error", func() {
		It("should return the error", func() {
			client := k8sfake.NewClientset()
			client.PrependReactor("get", "resource", func(_ clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("discovery unavailable")
			})
			_, _, _, err := discoverGVR(client, gvr)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("discovery unavailable"))
		})
	})
})
