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

package remotemetrics

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Context("Mapper", func() {

	var mapper Mapper

	BeforeEach(func() {
		mapper = NewNamespaceMapper(MappedNamespace{
			Namespace:    "namespace",
			OriginalName: "original_namespace",
		}, MappedNamespace{
			Namespace:    "namespace2",
			OriginalName: "namespace2",
		})
	})

	It("should map namespace name", func() {
		res := mapper.Map("metric1{namespace=\"namespace\",pod=\"pod1\"} 1 1000000000")
		Expect(res).To(Equal("metric1{namespace=\"original_namespace\",pod=\"pod1\"} 1 1000000000"))
	})

	It("should map namespace name if the name is equal", func() {
		res := mapper.Map("metric1{namespace=\"namespace2\",pod=\"pod1\"} 1 1000000000")
		Expect(res).To(Equal("metric1{namespace=\"namespace2\",pod=\"pod1\"} 1 1000000000"))
	})

	It("should not map namespace name if not in list", func() {
		res := mapper.Map("metric1{namespace=\"namespace3\",pod=\"pod1\"} 1 1000000000")
		Expect(res).To(Equal("metric1{namespace=\"namespace3\",pod=\"pod1\"} 1 1000000000"))
	})

})
