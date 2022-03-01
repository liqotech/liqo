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

package links_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"

	"github.com/liqotech/liqo/pkg/liqonet/utils/links"
)

var _ = Describe("Links", func() {
	var interfaceName = "dummy-link"
	Describe("testing DeleteIfaceByName function", func() {
		Context("when network interface exists", func() {
			BeforeEach(func() {
				// Create dummy link.
				err := netlink.LinkAdd(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: interfaceName}})
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should return nil", func() {
				err := links.DeleteIFaceByName(interfaceName)
				Expect(err).Should(BeNil())
				_, err = netlink.LinkByName(interfaceName)
				Expect(err).Should(MatchError("Link not found"))
			})
		})

		Context("when network interface does not exist", func() {
			It("should return nil", func() {
				err := links.DeleteIFaceByName("not-existing")
				Expect(err).Should(BeNil())
			})
		})
	})
})
