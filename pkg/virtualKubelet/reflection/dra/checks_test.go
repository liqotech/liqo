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
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/dra"
)

const (
	fakeResourceK8sIOV1 = "resource.k8s.io/v1"
	fakeDeviceClasses   = "deviceclasses"
)

var _ = Describe("DRA support detection", func() {

	// allDRAResources returns a fully-populated APIResourceList for resource.k8s.io/v1.
	allDRAResources := func() *metav1.APIResourceList {
		return &metav1.APIResourceList{
			GroupVersion: fakeResourceK8sIOV1,
			APIResources: []metav1.APIResource{
				{Name: "resourceslices", Namespaced: false},
				{Name: "resourceclaims", Namespaced: true},
				{Name: fakeDeviceClasses, Namespaced: false},
			},
		}
	}

	withResources := func(resources ...*metav1.APIResourceList) *fake.Clientset {
		fakeClient := fake.NewClientset()
		fakeClient.Discovery().(*discoveryfake.FakeDiscovery).Resources = resources
		return fakeClient
	}

	Describe("isDRAAPISupported", func() {
		It("should return true when all three DRA resources are exposed", func() {
			c := withResources(allDRAResources())
			ok, err := dra.IsDRAAPISupported(c)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		DescribeTable("should return false when any of the three required resources is missing",
			func(missing string) {
				rl := allDRAResources()
				kept := rl.APIResources[:0]
				for _, r := range rl.APIResources {
					if r.Name != missing {
						kept = append(kept, r)
					}
				}
				rl.APIResources = kept
				c := withResources(rl)

				ok, err := dra.IsDRAAPISupported(c)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeFalse())
			},
			Entry("missing resourceslices", "resourceslices"),
			Entry("missing resourceclaims", "resourceclaims"),
			Entry("missing deviceclasses", "deviceclasses"),
		)

		It("should return false when the resource.k8s.io/v1 group is not exposed at all", func() {
			c := withResources()
			ok, err := dra.IsDRAAPISupported(c)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("should return false when APIResources is empty for the group", func() {
			c := withResources(&metav1.APIResourceList{
				GroupVersion: fakeResourceK8sIOV1,
				APIResources: []metav1.APIResource{},
			})
			ok, err := dra.IsDRAAPISupported(c)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("should propagate an unexpected discovery error", func() {
			c := fake.NewClientset()
			err := errors.New("boom")
			c.Discovery().(*discoveryfake.FakeDiscovery).PrependReactor("get", "resource",
				func(_ clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, err
				},
			)
			ok, err := dra.IsDRAAPISupported(c)
			Expect(err).To(MatchError(err))
			Expect(ok).To(BeFalse())
		})
	})

	Describe("IsDRASupportedOnBothClusters", func() {
		It("should return true when both clusters expose DRA", func() {
			local := withResources(allDRAResources())
			remote := withResources(allDRAResources())
			ok, err := dra.IsDRASupportedOnBothClusters(local, remote)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should return false when local supports DRA but remote does not", func() {
			local := withResources(allDRAResources())
			remote := withResources()
			ok, err := dra.IsDRASupportedOnBothClusters(local, remote)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("should return false when remote supports DRA but local does not", func() {
			local := withResources()
			remote := withResources(allDRAResources())
			ok, err := dra.IsDRASupportedOnBothClusters(local, remote)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})
})
