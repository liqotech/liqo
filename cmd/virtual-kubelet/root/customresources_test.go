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

package root

import (
	"testing"

	. "github.com/onsi/gomega"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
)

func TestParseCustomResources(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		got, err := parseCustomResources(nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(got).To(BeNil())
	})

	t.Run("gvr only uses defaults", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		got, err := parseCustomResources([]string{"example.io/v1/widgets"})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(got).To(HaveLen(1))
		g.Expect(got[0].Group).To(Equal("example.io"))
		g.Expect(got[0].Version).To(Equal("v1"))
		g.Expect(got[0].Resource).To(Equal("widgets"))
		g.Expect(got[0].NumWorkers).To(Equal(defaultCustomResourceWorkers))
		g.Expect(got[0].Type).To(Equal(offloadingv1beta1.AllowList))
	})

	t.Run("full form", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		got, err := parseCustomResources([]string{"other.io/v1alpha1/gadgets,5,DenyList"})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(got).To(HaveLen(1))
		g.Expect(got[0].NumWorkers).To(Equal(uint(5)))
		g.Expect(got[0].Type).To(Equal(offloadingv1beta1.DenyList))
	})

	t.Run("invalid gvr", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		_, err := parseCustomResources([]string{"invalid"})
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("empty version or resource", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		_, err := parseCustomResources([]string{"example.io//widgets"})
		g.Expect(err).To(HaveOccurred())
		_, err = parseCustomResources([]string{"example.io/v1/"})
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("invalid type", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		_, err := parseCustomResources([]string{"example.io/v1/widgets,1,Nope"})
		g.Expect(err).To(HaveOccurred())
	})
}
