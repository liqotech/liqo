// Copyright 2019-2021 The Liqo Authors
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

package install

import (
	"context"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestLiqoctlInstall(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ForeignClusterUtils")
}

var _ = Describe("liqoctl", func() {
	Context("install", func() {
		When("the user does not specify a tag", func() {
			It("returns a suitable tag", func() {
				tag, err := FindNewestRelease(context.Background())
				Expect(err).To(BeNil())
				tag = strings.ToLower(tag)
				// Do not pick "latest", because it's not guaranteed to be a release
				Expect(tag).ToNot(Equal("latest"))
				Expect(tag).ToNot(ContainSubstring("rc"))
				Expect(tag).ToNot(ContainSubstring("alpha"))
			})
		})
	})
})
